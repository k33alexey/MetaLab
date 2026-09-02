// Package manager implements the local ML Manager HTTP surface.
package manager

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

//go:embed ui/index.html
var assets embed.FS

// NewHandler creates a Manager UI that observes, but does not own, ML Service.
func NewHandler(configuration appconfig.Config) http.Handler {
	return newHandler(configuration, http.DefaultClient)
}

func newHandler(configuration appconfig.Config, client *http.Client) http.Handler {
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
	return routes
}
