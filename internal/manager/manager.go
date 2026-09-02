// Package manager implements the local ML Manager HTTP surface.
package manager

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/systemdb"
)

//go:embed ui/index.html
var assets embed.FS

// NewHandler creates a Manager UI that observes, but does not own, ML Service.
func NewHandler(configuration appconfig.Config) http.Handler {
	return newHandler(configuration, http.DefaultClient, nil, nil)
}

// NewHandlerWithSetup adds the local first-run administrator wizard.
func NewHandlerWithSetup(configuration appconfig.Config, setup administratorSetup) http.Handler {
	return newHandler(configuration, http.DefaultClient, setup, nil)
}

// NewHandlerWithPlatform adds PostgreSQL and first-administrator setup.
func NewHandlerWithPlatform(configuration appconfig.Config, runtime platformSetup) http.Handler {
	return newHandler(configuration, http.DefaultClient, runtime, runtime)
}

type administratorSetup interface {
	InitialSetupRequired(context.Context) (bool, error)
	CreateInitial(context.Context, string, string) (systemdb.Administrator, []string, error)
}

type platformSetup interface {
	administratorSetup
	State() platform.State
	CheckPostgreSQL(context.Context, platform.ProvisionRequest) (postgresadmin.Check, error)
	ProvisionPostgreSQL(context.Context, platform.ProvisionRequest) (postgresadmin.Check, error)
}

func newHandler(configuration appconfig.Config, client *http.Client, setup administratorSetup, postgres platformSetup) http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /{$}", func(response http.ResponseWriter, _ *http.Request) {
		page, err := assets.ReadFile("ui/index.html")
		if err != nil {
			http.Error(response, "ML Manager UI unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(page)
	})
	routes.HandleFunc("GET /api/status", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()
		status := struct {
			Running bool   `json:"running"`
			URL     string `json:"url"`
		}{URL: configuration.LocalServiceURL()}
		serviceRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, status.URL+"/api/health", nil)
		if err == nil {
			serviceResponse, requestErr := client.Do(serviceRequest)
			if requestErr == nil {
				status.Running = serviceResponse.StatusCode == http.StatusOK
				_ = serviceResponse.Body.Close()
			}
		}
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(response).Encode(status)
	})
	routes.HandleFunc("GET /api/setup", func(response http.ResponseWriter, request *http.Request) {
		state := struct {
			Available bool `json:"available"`
			Required  bool `json:"required"`
		}{Available: setup != nil}
		if setup != nil {
			var err error
			state.Required, err = setup.InitialSetupRequired(request.Context())
			if err != nil {
				http.Error(response, "Unable to read initial setup state", http.StatusInternalServerError)
				return
			}
		}
		writeJSON(response, http.StatusOK, state)
	})
	routes.HandleFunc("GET /api/postgres", func(response http.ResponseWriter, _ *http.Request) {
		if postgres == nil {
			writeJSON(response, http.StatusOK, platform.State{})
			return
		}
		writeJSON(response, http.StatusOK, postgres.State())
	})
	routes.HandleFunc("POST /api/postgres/check", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "PostgreSQL setup is unavailable", http.StatusServiceUnavailable)
			return
		}
		var input platform.ProvisionRequest
		if !decodeJSON(response, request, &input) {
			return
		}
		check, err := postgres.CheckPostgreSQL(request.Context(), input)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusOK, check)
	})
	routes.HandleFunc("POST /api/postgres/provision", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "PostgreSQL setup is unavailable", http.StatusServiceUnavailable)
			return
		}
		var input platform.ProvisionRequest
		if !decodeJSON(response, request, &input) {
			return
		}
		check, err := postgres.ProvisionPostgreSQL(request.Context(), input)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusCreated, check)
	})
	routes.HandleFunc("POST /api/setup/administrator", func(response http.ResponseWriter, request *http.Request) {
		if setup == nil {
			http.Error(response, "ML System database is not configured", http.StatusServiceUnavailable)
			return
		}
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		administrator, codes, err := setup.CreateInitial(request.Context(), input.Login, input.Password)
		switch {
		case errors.Is(err, systemdb.ErrInitialAdministratorExists):
			http.Error(response, "Initial administrator already exists", http.StatusConflict)
			return
		case errors.Is(err, systemdb.ErrInvalidLogin), errors.Is(err, auth.ErrPasswordPolicy):
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		case err != nil:
			http.Error(response, "Unable to create initial administrator", http.StatusInternalServerError)
			return
		}
		writeJSON(response, http.StatusCreated, struct {
			Login         string   `json:"login"`
			RecoveryCodes []string `json:"recoveryCodes"`
		}{Login: administrator.Login, RecoveryCodes: codes})
	})
	return routes
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) bool {
	if contentType := request.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		http.Error(response, "Invalid setup request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(response, "Invalid setup request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		http.Error(response, "Unable to encode response", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_, _ = response.Write(body.Bytes())
}
