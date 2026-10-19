// Package manager implements the local ML Manager HTTP surface.
package manager

import (
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
	"github.com/k33alexey/MetaLab/internal/webpage"
)

//go:embed ui/index.html
var assets embed.FS

// NewHandler creates a Manager UI that observes, but does not own, ML Service.
func NewHandler(settings appconfig.Config) http.Handler {
	return newHandler(settings, http.DefaultClient, nil, nil)
}

// NewHandlerWithSetup adds the local first-run administrator wizard.
func NewHandlerWithSetup(settings appconfig.Config, setup administratorSetup) http.Handler {
	return newHandler(settings, http.DefaultClient, setup, nil)
}

// NewHandlerWithPlatform adds PostgreSQL and first-administrator setup.
func NewHandlerWithPlatform(settings appconfig.Config, runtime platformSetup) http.Handler {
	return secureManager(newHandler(settings, http.DefaultClient, runtime, runtime), runtime)
}

// NewHandlerWithPlatformAndStudio adds launching an independent ML Studio process.
func NewHandlerWithPlatformAndStudio(settings appconfig.Config, runtime platformSetup, launcher StudioLauncher) http.Handler {
	return secureManager(newHandler(settings, http.DefaultClient, runtime, runtime, launcher), runtime)
}

// StudioLauncher opens a filesystem-backed project without tying its lifetime to Manager HTTP requests.
type StudioLauncher interface {
	OpenStudio(context.Context, uuid.UUID, string) error
	CloneProject(context.Context, string, string) error
}

// BrowserOpener opens an address in the machine's default browser.
//
// Manager is a window around its own local interface, so an ordinary link
// would open inside that same window. Opening a browser, launching Studio and
// cloning a repository are one kind of thing - what only the desktop side of
// Manager can do, outside an HTTP request - so the desktop launcher carries
// all of them and this interface is asked of it rather than passed separately.
// Where it is absent, as in a Manager without a desktop, the button answers
// with the address instead of opening it.
type BrowserOpener interface {
	OpenURL(string) error
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
	ListStudioSessions(context.Context) ([]systemdb.StudioSession, error)
	TerminateStudioSession(context.Context, uuid.UUID) error
}

func newHandler(settings appconfig.Config, client *http.Client, setup administratorSetup, postgres platformSetup, launchers ...StudioLauncher) *http.ServeMux {
	routes := http.NewServeMux()
	var launcher StudioLauncher
	if len(launchers) > 0 {
		launcher = launchers[0]
	}
	routes.HandleFunc("GET /{$}", webpage.Page(assets, "ML Manager"))
	routes.HandleFunc("GET /api/status", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()
		status := struct {
			Running bool   `json:"running"`
			URL     string `json:"url"`
		}{URL: settings.LocalServiceURL()}
		serviceRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, status.URL+"/api/health", nil)
		if err == nil {
			serviceResponse, requestErr := client.Do(serviceRequest)
			if requestErr == nil {
				status.Running = serviceResponse.StatusCode == http.StatusOK
				_ = serviceResponse.Body.Close()
			}
		}
		webpage.JSON(response, http.StatusOK, status)
	})
	routes.HandleFunc("POST /api/portal/open", func(response http.ResponseWriter, _ *http.Request) {
		result := struct {
			URL    string `json:"url"`
			Opened bool   `json:"opened"`
		}{URL: settings.PortalURL()}
		var err error
		if browser, ok := launcher.(BrowserOpener); ok && browser != nil {
			err = browser.OpenURL(result.URL)
			result.Opened = err == nil
		}
		webpage.Answer(response, http.StatusOK, result, err, func(response http.ResponseWriter, err error) {
			http.Error(response, "Не удалось открыть браузер: "+err.Error(), http.StatusBadGateway)
		})
	})
	studio := func(route http.HandlerFunc) http.HandlerFunc {
		return whenAvailable(launcher != nil, "ML Studio launcher is unavailable", route)
	}
	operations := func(unavailable string, route http.HandlerFunc) http.HandlerFunc {
		return whenAvailable(postgres != nil, unavailable, route)
	}
	routes.HandleFunc("POST /api/studio/open", studio(func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			DatabaseID  uuid.UUID `json:"databaseId"`
			ProjectPath string    `json:"projectPath"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		input.ProjectPath = strings.TrimSpace(input.ProjectPath)
		if input.DatabaseID.IsZero() || input.ProjectPath == "" {
			http.Error(response, "Database and ML Project path are required", http.StatusBadRequest)
			return
		}
		if postgres != nil {
			security, ok := postgres.(managerSecurity)
			if !ok {
				http.Error(response, "Manager authentication unavailable", http.StatusServiceUnavailable)
				return
			}
			if err := security.AuthorizeManager(request.Context(), input.DatabaseID, "studio"); err != nil {
				managerAccessError(response, err)
				return
			}
		}
		err := launcher.OpenStudio(request.Context(), input.DatabaseID, input.ProjectPath)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("POST /api/studio/clone", studio(func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Repository  string `json:"repository"`
			Destination string `json:"destination"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		err := launcher.CloneProject(request.Context(), input.Repository, input.Destination)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("GET /api/studio/sessions", operations(platformUnavailable, listing(postgres, platformSetup.ListStudioSessions, "Unable to list ML Studio sessions")))
	routes.HandleFunc("DELETE /api/studio/sessions/{databaseId}", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
		databaseID, ok := parsePathValue(response, request, "databaseId", "database")
		if !ok {
			return
		}
		err := postgres.TerminateStudioSession(request.Context(), databaseID)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusConflict, ""))
	}))
	routes.HandleFunc("GET /api/setup", func(response http.ResponseWriter, request *http.Request) {
		state := struct {
			Available bool `json:"available"`
			Required  bool `json:"required"`
		}{Available: setup != nil}
		var err error
		if setup != nil {
			state.Required, err = setup.InitialSetupRequired(request.Context())
		}
		webpage.Answer(response, http.StatusOK, state, err, webpage.Refusal(http.StatusInternalServerError, "Unable to read initial setup state"))
	})
	routes.HandleFunc("GET /api/postgres", func(response http.ResponseWriter, _ *http.Request) {
		if postgres == nil {
			webpage.JSON(response, http.StatusOK, platform.State{})
			return
		}
		webpage.JSON(response, http.StatusOK, postgres.State())
	})
	for path, step := range map[string]struct {
		do     func(context.Context, platform.ProvisionRequest) (postgresadmin.Check, error)
		status int
	}{
		"check": {func(ctx context.Context, input platform.ProvisionRequest) (postgresadmin.Check, error) {
			return postgres.CheckPostgreSQL(ctx, input)
		}, http.StatusOK},
		"provision": {func(ctx context.Context, input platform.ProvisionRequest) (postgresadmin.Check, error) {
			return postgres.ProvisionPostgreSQL(ctx, input)
		}, http.StatusCreated},
	} {
		routes.HandleFunc("POST /api/postgres/"+path, operations("PostgreSQL setup is unavailable",
			fromBody(step.do, step.status, webpage.Refusal(http.StatusBadRequest, ""))))
	}
	routes.HandleFunc("GET /api/databases", operations(registryUnavailable, listing(postgres, platformSetup.ListDatabases, "Unable to read database registry")))
	routes.HandleFunc("POST /api/databases", operations(registryUnavailable, fromBody(func(ctx context.Context, input platform.RegisterDatabaseRequest) (systemdb.RegisteredDatabase, error) {
		return postgres.RegisterDatabase(ctx, input)
	}, http.StatusCreated, refuseRegistration)))
	routes.HandleFunc("POST /api/databases/{id}/debug", operations(registryUnavailable, func(response http.ResponseWriter, request *http.Request) {
		id, ok := parsePathUUID(response, request, "source database")
		if !ok {
			return
		}
		var input platform.CreateDebugDatabaseRequest
		if !decodeJSON(response, request, &input) {
			return
		}
		item, err := postgres.CreateDebugDatabase(request.Context(), id, input)
		webpage.Answer(response, http.StatusCreated, item, err, func(response http.ResponseWriter, err error) {
			if errors.Is(err, systemdb.ErrDatabaseNotFound) {
				http.Error(response, err.Error(), http.StatusNotFound)
				return
			}
			refuseRegistration(response, err)
		})
	}))
	routes.HandleFunc("DELETE /api/databases/{id}", operations(registryUnavailable, func(response http.ResponseWriter, request *http.Request) {
		id, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		err := postgres.UnregisterDatabase(request.Context(), id)
		webpage.Answer(response, http.StatusNoContent, nil, err, func(response http.ResponseWriter, err error) {
			switch {
			case errors.Is(err, systemdb.ErrDatabaseNotFound):
				http.Error(response, err.Error(), http.StatusNotFound)
			case errors.Is(err, systemdb.ErrDatabaseCannotUnregister), errors.Is(err, systemdb.ErrDatabaseHasDebugCopies), errors.Is(err, systemdb.ErrDatabaseHasBackups), errors.Is(err, systemdb.ErrDatabaseHasStudioSession):
				http.Error(response, err.Error(), http.StatusConflict)
			default:
				http.Error(response, "Unable to unregister database", http.StatusInternalServerError)
			}
		})
	}))
	if postgres != nil {
		for action, operation := range map[string]func(platformSetup, context.Context, uuid.UUID) (systemdb.RegisteredDatabase, error){
			"start": platformSetup.StartDatabase, "stop": platformSetup.StopDatabase, "health": platformSetup.CheckDatabaseHealth,
		} {
			routes.HandleFunc("POST /api/databases/{id}/"+action, ofDatabase(postgres, operation, http.StatusOK, webpage.Refusal(http.StatusConflict, "")))
		}
	}
	routes.HandleFunc("PUT /api/databases/{id}/session-access", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
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
		webpage.Answer(response, http.StatusOK, item, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("GET /api/sessions", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
		databaseID, ok := optionalQueryUUID(response, request, "databaseId")
		if !ok {
			return
		}
		items, err := postgres.ListDatabaseSessions(request.Context(), databaseID)
		webpage.Answer(response, http.StatusOK, items, err, webpage.Refusal(http.StatusServiceUnavailable, "Unable to list sessions"))
	}))
	routes.HandleFunc("POST /api/sessions/{id}/message", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
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
		err := postgres.SendSessionMessage(request.Context(), id, input.Message)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("DELETE /api/sessions/{id}", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
		id, ok := parsePathUUID(response, request, "session")
		if !ok {
			return
		}
		err := postgres.TerminateDatabaseSession(request.Context(), id)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("GET /api/logs", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
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
		webpage.Answer(response, http.StatusOK, items, err, webpage.Refusal(http.StatusServiceUnavailable, "Unable to list logs"))
	}))
	routes.HandleFunc("GET /api/databases/{id}/backups", operations(platformUnavailable,
		ofDatabase(postgres, platformSetup.ListDatabaseBackups, http.StatusOK, webpage.Refusal(http.StatusBadRequest, ""))))
	routes.HandleFunc("POST /api/databases/{id}/backups", operations(platformUnavailable,
		ofDatabase(postgres, platformSetup.CreateDatabaseBackup, http.StatusCreated, webpage.Refusal(http.StatusBadRequest, ""))))
	routes.HandleFunc("POST /api/databases/{id}/backups/{backupId}/restore", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
		databaseID, backupID, ok := backupPath(response, request)
		if !ok {
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
		err := postgres.RestoreDatabaseBackup(request.Context(), databaseID, backupID)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusConflict, ""))
	}))
	routes.HandleFunc("DELETE /api/databases/{id}/backups/{backupId}", operations(platformUnavailable, func(response http.ResponseWriter, request *http.Request) {
		databaseID, backupID, ok := backupPath(response, request)
		if !ok {
			return
		}
		err := postgres.DeleteDatabaseBackup(request.Context(), databaseID, backupID)
		webpage.Answer(response, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, ""))
	}))
	routes.HandleFunc("POST /api/setup/administrator", whenAvailable(setup != nil, "ML System database is not configured", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		administrator, codes, err := setup.CreateInitial(request.Context(), input.Login, input.Password)
		webpage.Answer(response, http.StatusCreated, struct {
			Login         string   `json:"login"`
			RecoveryCodes []string `json:"recoveryCodes"`
		}{Login: administrator.Login, RecoveryCodes: codes}, err, func(response http.ResponseWriter, err error) {
			switch {
			case errors.Is(err, systemdb.ErrInitialAdministratorExists):
				http.Error(response, "Initial administrator already exists", http.StatusConflict)
			case errors.Is(err, systemdb.ErrInvalidLogin), errors.Is(err, auth.ErrPasswordPolicy):
				http.Error(response, err.Error(), http.StatusBadRequest)
			default:
				http.Error(response, "Unable to create initial administrator", http.StatusInternalServerError)
			}
		})
	}))
	return routes
}

// The words a route of Manager says when what it needs is not there.
const (
	platformUnavailable = "Platform operations are unavailable"
	registryUnavailable = "Database registry is unavailable"
)

// whenAvailable is a route of what this Manager may run without - the
// platform's operations, the system database, the launcher of Studio: the
// route itself where it is there, and otherwise a refusal 503 in the route's
// own words. Whether it is there is settled when Manager starts.
func whenAvailable(available bool, unavailable string, route http.HandlerFunc) http.HandlerFunc {
	if available {
		return route
	}
	return func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, unavailable, http.StatusServiceUnavailable)
	}
}

// listing is a route of Manager that lists what the platform keeps and
// answers the list; a failure is that the platform cannot be read, said in
// the route's words. The list is named by the method of the platform, read
// only when the route is asked.
func listing[List any](postgres platformSetup, list func(platformSetup, context.Context) (List, error), failure string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		items, err := list(postgres, request.Context())
		webpage.Answer(response, http.StatusOK, items, err, webpage.Refusal(http.StatusServiceUnavailable, failure))
	}
}

// fromBody is a route of Manager that reads its request whole from the body
// (decodeJSON), does what it asks and answers what came of it with status,
// or refuses as the route does.
func fromBody[In, Result any](do func(context.Context, In) (Result, error), status int, refuse func(http.ResponseWriter, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input In
		if !decodeJSON(response, request, &input) {
			return
		}
		result, err := do(request.Context(), input)
		webpage.Answer(response, status, result, err, refuse)
	}
}

// ofDatabase is a route of Manager that does one thing to the database its
// address names in {id} - starts it, lists or makes its backups - and answers
// what came of it with status, or refuses as the route does.
func ofDatabase[Result any](postgres platformSetup, do func(platformSetup, context.Context, uuid.UUID) (Result, error), status int, refuse func(http.ResponseWriter, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		id, ok := parsePathUUID(response, request, "database")
		if !ok {
			return
		}
		result, err := do(postgres, request.Context(), id)
		webpage.Answer(response, status, result, err, refuse)
	}
}

// refuseRegistration refuses a database that cannot be registered: a name or
// a database of the server taken already is a conflict, the rest a bad
// request.
func refuseRegistration(response http.ResponseWriter, err error) {
	if errors.Is(err, systemdb.ErrDatabaseNameExists) || errors.Is(err, systemdb.ErrPhysicalDatabaseExists) {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	http.Error(response, err.Error(), http.StatusBadRequest)
}

// parsePathUUID reads the identifier of the address's {id}, naming what it
// identifies in the refusal.
func parsePathUUID(response http.ResponseWriter, request *http.Request, kind string) (uuid.UUID, bool) {
	return parsePathValue(response, request, "id", kind)
}

// parsePathValue reads the identifier of one value of the address and
// refuses a request whose value is no identifier, naming what it identifies.
func parsePathValue(response http.ResponseWriter, request *http.Request, name, kind string) (uuid.UUID, bool) {
	id, err := uuid.Parse(request.PathValue(name))
	if err != nil {
		http.Error(response, "Invalid "+kind+" identifier", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

// backupPath reads the database and the backup an address of a backup names.
func backupPath(response http.ResponseWriter, request *http.Request) (uuid.UUID, uuid.UUID, bool) {
	databaseID, ok := parsePathUUID(response, request, "database")
	if !ok {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	backupID, ok := parsePathValue(response, request, "backupId", "backup")
	return databaseID, backupID, ok
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
	return decodeJSONLimit(response, request, destination, 8<<10)
}

func decodeJSONLimit(response http.ResponseWriter, request *http.Request, destination any, limit int64) bool {
	return decodeJSONBody(response, request, http.MaxBytesReader(response, request.Body, limit), destination)
}

// decodeJSONBody reads one JSON value from body and nothing after it. A route
// whose body carries the configuration's own composition - the roles of a
// user - reads it whole, because a bound on the request would be a bound on
// what the configuration may hold.
func decodeJSONBody(response http.ResponseWriter, request *http.Request, body io.Reader, destination any) bool {
	if contentType := request.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(body)
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
