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
	"strconv"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
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

// NewHandlerWithPlatformAndStudio adds launching an independent ML Studio process.
func NewHandlerWithPlatformAndStudio(configuration appconfig.Config, runtime platformSetup, launcher StudioLauncher) http.Handler {
	return newHandler(configuration, http.DefaultClient, runtime, runtime, launcher)
}

// StudioLauncher opens a filesystem-backed project without tying its lifetime to Manager HTTP requests.
type StudioLauncher interface {
	OpenStudio(string) error
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
	ListDatabases(context.Context) ([]systemdb.RegisteredDatabase, error)
	RegisterDatabase(context.Context, platform.RegisterDatabaseRequest) (systemdb.RegisteredDatabase, error)
	CreateDebugDatabase(context.Context, uuid.UUID, platform.CreateDebugDatabaseRequest) (systemdb.RegisteredDatabase, error)
	UnregisterDatabase(context.Context, uuid.UUID) error
	StartDatabase(context.Context, uuid.UUID) (systemdb.RegisteredDatabase, error)
	StopDatabase(context.Context, uuid.UUID) (systemdb.RegisteredDatabase, error)
	SetDatabaseSessionAccess(context.Context, uuid.UUID, bool) (systemdb.RegisteredDatabase, error)
	CheckDatabaseHealth(context.Context, uuid.UUID) (systemdb.RegisteredDatabase, error)
	ListDatabaseSessions(context.Context, *uuid.UUID) ([]systemdb.DatabaseSession, error)
	SendSessionMessage(context.Context, uuid.UUID, string) error
	TerminateDatabaseSession(context.Context, uuid.UUID) error
	ListAuditEvents(context.Context, *uuid.UUID, int) ([]systemdb.AuditEvent, error)
	CreateDatabaseBackup(context.Context, uuid.UUID) (systemdb.Backup, error)
	ListDatabaseBackups(context.Context, uuid.UUID) ([]systemdb.Backup, error)
	DeleteDatabaseBackup(context.Context, uuid.UUID, uuid.UUID) error
	RestoreDatabaseBackup(context.Context, uuid.UUID, uuid.UUID) error
}

func newHandler(configuration appconfig.Config, client *http.Client, setup administratorSetup, postgres platformSetup, launchers ...StudioLauncher) http.Handler {
	routes := http.NewServeMux()
	var launcher StudioLauncher
	if len(launchers) > 0 {
		launcher = launchers[0]
	}
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
	routes.HandleFunc("POST /api/studio/open", func(response http.ResponseWriter, request *http.Request) {
		if launcher == nil {
			http.Error(response, "ML Studio launcher is unavailable", http.StatusServiceUnavailable)
			return
		}
		var input struct {
			ProjectPath string `json:"projectPath"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		input.ProjectPath = strings.TrimSpace(input.ProjectPath)
		if input.ProjectPath == "" {
			http.Error(response, "ML Project path is required", http.StatusBadRequest)
			return
		}
		if err := launcher.OpenStudio(input.ProjectPath); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
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
	routes.HandleFunc("GET /api/databases", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Database registry is unavailable", http.StatusServiceUnavailable)
			return
		}
		items, err := postgres.ListDatabases(request.Context())
		if err != nil {
			http.Error(response, "Unable to read database registry", http.StatusServiceUnavailable)
			return
		}
		writeJSON(response, http.StatusOK, items)
	})
	routes.HandleFunc("POST /api/databases", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Database registry is unavailable", http.StatusServiceUnavailable)
			return
		}
		var input platform.RegisterDatabaseRequest
		if !decodeJSON(response, request, &input) {
			return
		}
		item, err := postgres.RegisterDatabase(request.Context(), input)
		switch {
		case errors.Is(err, systemdb.ErrDatabaseNameExists), errors.Is(err, systemdb.ErrPhysicalDatabaseExists):
			http.Error(response, err.Error(), http.StatusConflict)
			return
		case err != nil:
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusCreated, item)
	})
	routes.HandleFunc("POST /api/databases/{id}/debug", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Database registry is unavailable", http.StatusServiceUnavailable)
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.Error(response, "Invalid source database identifier", http.StatusBadRequest)
			return
		}
		var input platform.CreateDebugDatabaseRequest
		if !decodeJSON(response, request, &input) {
			return
		}
		item, err := postgres.CreateDebugDatabase(request.Context(), id, input)
		switch {
		case errors.Is(err, systemdb.ErrDatabaseNotFound):
			http.Error(response, err.Error(), http.StatusNotFound)
		case errors.Is(err, systemdb.ErrDatabaseNameExists), errors.Is(err, systemdb.ErrPhysicalDatabaseExists):
			http.Error(response, err.Error(), http.StatusConflict)
		case err != nil:
			http.Error(response, err.Error(), http.StatusBadRequest)
		default:
			writeJSON(response, http.StatusCreated, item)
		}
	})
	routes.HandleFunc("DELETE /api/databases/{id}", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Database registry is unavailable", http.StatusServiceUnavailable)
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.Error(response, "Invalid database identifier", http.StatusBadRequest)
			return
		}
		err = postgres.UnregisterDatabase(request.Context(), id)
		switch {
		case errors.Is(err, systemdb.ErrDatabaseNotFound):
			http.Error(response, err.Error(), http.StatusNotFound)
		case errors.Is(err, systemdb.ErrDatabaseCannotUnregister), errors.Is(err, systemdb.ErrDatabaseHasDebugCopies), errors.Is(err, systemdb.ErrDatabaseHasBackups):
			http.Error(response, err.Error(), http.StatusConflict)
		case err != nil:
			http.Error(response, "Unable to unregister database", http.StatusInternalServerError)
		default:
			response.WriteHeader(http.StatusNoContent)
		}
	})
	if postgres != nil {
		for action, operation := range map[string]func(context.Context, uuid.UUID) (systemdb.RegisteredDatabase, error){
			"start": postgres.StartDatabase, "stop": postgres.StopDatabase, "health": postgres.CheckDatabaseHealth,
		} {
			action, operation := action, operation
			routes.HandleFunc("POST /api/databases/{id}/"+action, func(response http.ResponseWriter, request *http.Request) {
				id, ok := parsePathUUID(response, request, "database")
				if !ok {
					return
				}
				item, err := operation(request.Context(), id)
				if err != nil {
					http.Error(response, err.Error(), http.StatusConflict)
					return
				}
				writeJSON(response, http.StatusOK, item)
			})
		}
	}
	routes.HandleFunc("PUT /api/databases/{id}/session-access", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		id, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		var input struct {
			Allowed bool `json:"allowed"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		item, err := postgres.SetDatabaseSessionAccess(request.Context(), id, input.Allowed)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusOK, item)
	})
	routes.HandleFunc("GET /api/sessions", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		databaseID, ok := optionalQueryUUID(response, request, "databaseId")
		if !ok {
			return
		}
		items, err := postgres.ListDatabaseSessions(request.Context(), databaseID)
		if err != nil {
			http.Error(response, "Unable to list sessions", http.StatusServiceUnavailable)
			return
		}
		writeJSON(response, http.StatusOK, items)
	})
	routes.HandleFunc("POST /api/sessions/{id}/message", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		id, ok := parsePathUUID(response, request, "session")
		if !ok {
			return
		}
		var input struct {
			Message string `json:"message"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		if err := postgres.SendSessionMessage(request.Context(), id, input.Message); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("DELETE /api/sessions/{id}", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		id, ok := parsePathUUID(response, request, "session")
		if !ok {
			return
		}
		if err := postgres.TerminateDatabaseSession(request.Context(), id); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("GET /api/logs", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		databaseID, ok := optionalQueryUUID(response, request, "databaseId")
		if !ok {
			return
		}
		limit := 100
		if value := request.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 500 {
				http.Error(response, "Invalid log limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		items, err := postgres.ListAuditEvents(request.Context(), databaseID, limit)
		if err != nil {
			http.Error(response, "Unable to list logs", http.StatusServiceUnavailable)
			return
		}
		writeJSON(response, http.StatusOK, items)
	})
	routes.HandleFunc("GET /api/databases/{id}/backups", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		id, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		items, err := postgres.ListDatabaseBackups(request.Context(), id)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusOK, items)
	})
	routes.HandleFunc("POST /api/databases/{id}/backups", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		id, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		backup, err := postgres.CreateDatabaseBackup(request.Context(), id)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(response, http.StatusCreated, backup)
	})
	routes.HandleFunc("POST /api/databases/{id}/backups/{backupId}/restore", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		databaseID, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		backupID, err := uuid.Parse(request.PathValue("backupId"))
		if err != nil {
			http.Error(response, "Invalid backup identifier", http.StatusBadRequest)
			return
		}
		var input struct {
			Confirm bool `json:"confirm"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		if !input.Confirm {
			http.Error(response, "Restore confirmation is required", http.StatusBadRequest)
			return
		}
		if err := postgres.RestoreDatabaseBackup(request.Context(), databaseID, backupID); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("DELETE /api/databases/{id}/backups/{backupId}", func(response http.ResponseWriter, request *http.Request) {
		if postgres == nil {
			http.Error(response, "Platform operations are unavailable", http.StatusServiceUnavailable)
			return
		}
		databaseID, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		backupID, err := uuid.Parse(request.PathValue("backupId"))
		if err != nil {
			http.Error(response, "Invalid backup identifier", http.StatusBadRequest)
			return
		}
		if err := postgres.DeleteDatabaseBackup(request.Context(), databaseID, backupID); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
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

func parsePathUUID(response http.ResponseWriter, request *http.Request, kind string) (uuid.UUID, bool) {
	id, err := uuid.Parse(request.PathValue("id"))
	if err != nil {
		http.Error(response, "Invalid "+kind+" identifier", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

func optionalQueryUUID(response http.ResponseWriter, request *http.Request, name string) (*uuid.UUID, bool) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	id, err := uuid.Parse(value)
	if err != nil {
		http.Error(response, "Invalid "+name, http.StatusBadRequest)
		return nil, false
	}
	return &id, true
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
