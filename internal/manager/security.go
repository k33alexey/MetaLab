package manager

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/k33alexey/MetaLab/internal/webpage"
)

type managerSecurity interface {
	LoginManager(context.Context, string, string, string, string) (platform.PortalLogin, error)
	AuthenticateManager(context.Context, string) (systemdb.PortalSession, error)
	LogoutManager(context.Context, string) error
	ChangeManagerPassword(context.Context, string, string, string) error
	AuthorizeManager(context.Context, uuid.UUID, string) error
	AuthorizeManagerSession(context.Context, uuid.UUID) error
	ListManagerDatabases(context.Context) ([]platform.ManagerDatabase, error)
	ListManagerStudioSessions(context.Context) ([]systemdb.StudioSession, error)
	ListManagerSessions(context.Context, *uuid.UUID) ([]systemdb.DatabaseSession, error)
	ListManagerDatabaseAccess(context.Context, uuid.UUID) ([]systemdb.DatabaseAccess, error)
	AssignManagerDatabasePermissions(context.Context, uuid.UUID, string, systemdb.DatabasePermissions) error
	CreateManagerUser(context.Context, systemdb.UserCreation) (systemdb.User, error)
	ListManagerUsers(context.Context) ([]systemdb.User, error)
	UpdateManagerUser(context.Context, uuid.UUID, systemdb.UserAccessUpdate) error
}

// secureManager is the only production boundary for database/Studio operations.
// newHandler remains the route implementation, not an authenticated public surface.
func secureManager(routes *http.ServeMux, backend platformSetup) http.Handler {
	registerApplicationRoleRoutes(routes, backend)
	security, available := backend.(managerSecurity)
	// Cookies are unique to this Manager process, including simultaneous windows on localhost.
	cookieName := "ml_manager_" + uuid.MustNew().String()
	tokenOf := func(request *http.Request) string {
		cookie, err := request.Cookie(cookieName)
		if err != nil {
			return ""
		}
		return cookie.Value
	}
	setCookie := func(w http.ResponseWriter, r *http.Request, token string) {
		age := 0
		if token == "" {
			age = -1
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: age})
	}
	authentication := func(route http.HandlerFunc) http.HandlerFunc {
		return whenAvailable(available, "Manager authentication unavailable", route)
	}
	var loginMu sync.Mutex
	var loginWindow time.Time
	loginAttempts := 0
	routes.HandleFunc("POST /api/manager/login", authentication(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		loginMu.Lock()
		if time.Since(loginWindow) >= time.Minute {
			loginWindow = time.Now()
			loginAttempts = 0
		}
		allowed := loginAttempts < 10
		if allowed {
			loginAttempts++
		}
		loginMu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many login attempts", http.StatusTooManyRequests)
			return
		}
		login, err := security.LoginManager(r.Context(), input.Login, input.Password, r.RemoteAddr, r.UserAgent())
		if err != nil {
			http.Error(w, "Invalid credentials or no Manager access", http.StatusUnauthorized)
			return
		}
		setCookie(w, r, login.Token)
		webpage.JSON(w, 200, login.Session)
	}))
	routes.HandleFunc("GET /api/manager/session", authentication(func(w http.ResponseWriter, r *http.Request) {
		session, err := security.AuthenticateManager(r.Context(), tokenOf(r))
		webpage.Answer(w, http.StatusOK, session, err, managerAccessError)
	}))
	routes.HandleFunc("POST /api/manager/logout", authentication(func(w http.ResponseWriter, r *http.Request) {
		err := security.LogoutManager(r.Context(), tokenOf(r))
		if err != nil && !errors.Is(err, systemdb.ErrSessionNotFound) {
			managerAccessError(w, err)
			return
		}
		setCookie(w, r, "")
		w.WriteHeader(204)
	}))
	routes.HandleFunc("POST /api/manager/password", authentication(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Current string `json:"currentPassword"`
			New     string `json:"newPassword"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		err := security.ChangeManagerPassword(r.Context(), tokenOf(r), input.Current, input.New)
		webpage.Answer(w, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, "Unable to change password"))
	}))
	routes.HandleFunc("GET /api/manager/users", func(w http.ResponseWriter, r *http.Request) {
		items, err := security.ListManagerUsers(r.Context())
		webpage.Answer(w, http.StatusOK, items, err, managerAccessError)
	})
	routes.HandleFunc("POST /api/manager/users", fromBody(func(ctx context.Context, input systemdb.UserCreation) (systemdb.User, error) {
		return security.CreateManagerUser(ctx, input)
	}, http.StatusCreated, webpage.Refusal(http.StatusBadRequest, "Unable to create user: check login and password")))
	routes.HandleFunc("PUT /api/manager/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathUUID(w, r, "user")
		if !ok {
			return
		}
		var input systemdb.UserAccessUpdate
		if !decodeJSON(w, r, &input) {
			return
		}
		err := security.UpdateManagerUser(r.Context(), id, input)
		webpage.Answer(w, http.StatusNoContent, nil, err, func(w http.ResponseWriter, err error) {
			if errors.Is(err, systemdb.ErrLastPlatformAdministrator) {
				http.Error(w, "Cannot disable or demote the last platform administrator", http.StatusConflict)
				return
			}
			managerAccessError(w, err)
		})
	})
	routes.HandleFunc("GET /api/databases/{id}/permissions", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathUUID(w, r, "database")
		if !ok {
			return
		}
		items, err := security.ListManagerDatabaseAccess(r.Context(), id)
		webpage.Answer(w, http.StatusOK, items, err, managerAccessError)
	})
	routes.HandleFunc("PUT /api/databases/{id}/permissions", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathUUID(w, r, "database")
		if !ok {
			return
		}
		var input struct {
			Login       string                       `json:"login"`
			Permissions systemdb.DatabasePermissions `json:"permissions"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		err := security.AssignManagerDatabasePermissions(r.Context(), id, input.Login, input.Permissions)
		webpage.Answer(w, http.StatusNoContent, nil, err, webpage.Refusal(http.StatusBadRequest, "Unable to assign access: check user and system roles"))
	})
	guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "localhost" && !net.ParseIP(strings.Trim(host, "[]")).IsLoopback() {
			http.Error(w, "Manager is local-only", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		webpage.SetPolicy(w.Header())
		_, pattern := routes.Handler(r)
		if pattern == "" {
			http.NotFound(w, r)
			return
		}
		switch pattern {
		case "GET /{$}", "GET /api/status", "GET /api/setup", "POST /api/manager/login", "GET /api/manager/session", "POST /api/manager/logout", "POST /api/manager/password":
			routes.ServeHTTP(w, r)
			return
		case "GET /api/postgres", "POST /api/postgres/check", "POST /api/postgres/provision", "POST /api/setup/administrator":
			if !backend.State().Configured {
				routes.ServeHTTP(w, r)
				return
			}
			required, err := backend.InitialSetupRequired(r.Context())
			if err == nil && required {
				routes.ServeHTTP(w, r)
				return
			}
		}
		if !available {
			http.Error(w, "Manager authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx := platform.WithManagerToken(r.Context(), tokenOf(r))
		session, err := security.AuthenticateManager(ctx, tokenOf(r))
		if err != nil {
			managerAccessError(w, err)
			return
		}
		if session.MustChangePassword {
			managerAccessError(w, systemdb.ErrPasswordChangeRequired)
			return
		}
		r = r.WithContext(ctx)
		// List endpoints filter on the server, before serialization.
		switch pattern {
		case "GET /api/databases":
			items, err := security.ListManagerDatabases(ctx)
			if err != nil {
				managerAccessError(w, err)
				return
			}
			webpage.JSON(w, 200, items)
			return
		case "GET /api/studio/sessions":
			items, err := security.ListManagerStudioSessions(ctx)
			if err != nil {
				managerAccessError(w, err)
				return
			}
			webpage.JSON(w, 200, items)
			return
		case "GET /api/sessions":
			id, ok := optionalQueryUUID(w, r, "databaseId")
			if !ok {
				return
			}
			items, err := security.ListManagerSessions(ctx, id)
			if err != nil {
				managerAccessError(w, err)
				return
			}
			webpage.JSON(w, 200, items)
			return
		}
		permission := "platform"
		var databaseID uuid.UUID
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case pattern == "POST /api/studio/clone", pattern == "POST /api/studio/open":
			permission = "developer" // OpenStudio also checks the database grant transactionally.
		case strings.HasPrefix(pattern, "POST /api/sessions/{id}/"), pattern == "DELETE /api/sessions/{id}":
			id, err := uuid.Parse(segments[2])
			if err == nil {
				err = security.AuthorizeManagerSession(ctx, id)
			}
			if err != nil {
				managerAccessError(w, err)
				return
			}
			routes.ServeHTTP(w, r)
			return
		case strings.Contains(pattern, "/api/databases/{id}"), pattern == "DELETE /api/studio/sessions/{databaseId}":
			index := 2
			if strings.Contains(pattern, "/studio/sessions/") {
				index = 3
			}
			var err error
			databaseID, err = uuid.Parse(segments[index])
			if err != nil {
				http.Error(w, "Invalid database identifier", 400)
				return
			}
			permission = "admin"
			if pattern == "POST /api/databases/{id}/debug" && !session.PlatformAdmin {
				permission = "studio"
			}
		case pattern == "GET /api/logs":
			if text := r.URL.Query().Get("databaseId"); text != "" {
				var err error
				databaseID, err = uuid.Parse(text)
				if err != nil {
					http.Error(w, "Invalid database identifier", 400)
					return
				}
				permission = "admin"
			}
		}
		if err := security.AuthorizeManager(ctx, databaseID, permission); err != nil {
			managerAccessError(w, err)
			return
		}
		routes.ServeHTTP(w, r)
	})
	return http.NewCrossOriginProtection().Handler(guard)
}

func managerAccessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, systemdb.ErrSessionNotFound), errors.Is(err, systemdb.ErrInvalidCredentials):
		http.Error(w, "Sign in to ML Manager", http.StatusUnauthorized)
	case errors.Is(err, systemdb.ErrPasswordChangeRequired):
		http.Error(w, "Change your password before continuing", http.StatusForbidden)
	case errors.Is(err, systemdb.ErrDatabaseAccessDenied), errors.Is(err, systemdb.ErrManagerAccessDenied), errors.Is(err, systemdb.ErrDatabaseOwnerOnly):
		http.Error(w, "Access denied", http.StatusForbidden)
	default:
		http.Error(w, "Manager operation unavailable", http.StatusServiceUnavailable)
	}
}
