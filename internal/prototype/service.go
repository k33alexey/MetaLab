// Package prototype contains the measured end-to-end architecture scenario.
package prototype

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
)

const calculationSource = `Function Calculate(A, B) Export
Return (A + B) * 2;
EndFunction`

//go:embed ui/index.html
var assets embed.FS

// Service connects the web boundary, BSL VM and persistence boundary.
type Service struct {
	store   Store
	machine *vm.Machine
	handler http.Handler
	now     func() time.Time
}

// NewService creates the measured end-to-end prototype service.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("prototype store is nil")
	}
	program, diagnostics := compiler.CompileSource("prototype.bsl", calculationSource)
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("compile prototype BSL: %v", diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		return nil, fmt.Errorf("create prototype VM: %w", err)
	}
	service := &Service{store: store, machine: machine, now: time.Now}
	service.handler = service.routes()
	return service, nil
}

// Handler returns the complete HTTP surface of the prototype service.
func (service *Service) Handler() http.Handler { return service.handler }

func (service *Service) routes() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /{$}", service.index)
	routes.HandleFunc("GET /api/health", service.health)
	routes.HandleFunc("GET /api/prototype/stats", service.stats)
	routes.HandleFunc("POST /api/prototype/calculate", service.calculate)
	return routes
}

func (service *Service) index(response http.ResponseWriter, _ *http.Request) {
	page, err := assets.ReadFile("ui/index.html")
	if err != nil {
		http.Error(response, "prototype UI unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = response.Write(page)
}

func (service *Service) health(response http.ResponseWriter, request *http.Request) {
	if err := service.store.Ping(request.Context()); err != nil {
		writeError(response, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "database": "postgresql"})
}

func (service *Service) stats(response http.ResponseWriter, request *http.Request) {
	stats, err := service.store.Stats(request.Context())
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, stats)
}

func (service *Service) calculate(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Left  float64 `json:"left"`
		Right float64 `json:"right"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}

	left, err := bytecode.NumberFromFloat64(input.Left)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid left operand: "+err.Error())
		return
	}
	right, err := bytecode.NumberFromFloat64(input.Right)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid right operand: "+err.Error())
		return
	}
	started := time.Now()
	result, err := service.machine.Call("Calculate", left, right)
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	number, ok := result.AsNumber()
	if !ok {
		writeError(response, http.StatusInternalServerError, "prototype BSL returned a non-number")
		return
	}
	calculation, err := service.store.Save(request.Context(), Calculation{
		Left: input.Left, Right: input.Right, Result: number, CreatedAt: service.now().UTC(),
	})
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, struct {
		Calculation
		ElapsedMicroseconds int64 `json:"elapsedMicroseconds"`
	}{Calculation: calculation, ElapsedMicroseconds: time.Since(started).Microseconds()})
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON value")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}
