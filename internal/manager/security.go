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
	var loginMu sync.Mutex
	var loginWindow time.Time
	loginAttempts := 0
	routes.HandleFunc("POST /api/manager/login", func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "Manager authentication unavailable", 503)
			return
		}
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
			http.Error(w, "Too many login attempts", 429)
			return
		}
		login, err := security.LoginManager(r.Context(), input.Login, input.Password, r.RemoteAddr, r.UserAgent())
		if err != nil {
			http.Error(w, "Invalid credentials or no Manager access", 401)
			return
		}
		setCookie(w, r, login.Token)
		writeJSON(w, 200, login.Session)
	})
	routes.HandleFunc("GET /api/manager/session", func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "Manager authentication unavailable", 503)
			return
		}
		session, err := security.AuthenticateManager(r.Context(), tokenOf(r))
		if err != nil {
			managerAccessError(w, err)
			return
		}
		writeJSON(w, 200, session)
	})
	routes.HandleFunc("POST /api/manager/logout", func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "Manager authentication unavailable", 503)
			return
		}
		err := security.LogoutManager(r.Context(), tokenOf(r))
		if err != nil && !errors.Is(err, systemdb.ErrSessionNotFound) {
			managerAccessError(w, err)
			return
		}
		setCookie(w, r, "")
		w.WriteHeader(204)
	})
	routes.HandleFunc("POST /api/manager/password", func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "Manager authentication unavailable", 503)
			return
		}
		var input struct {
			Current string `json:"currentPassword"`
			New     string `json:"newPassword"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := security.ChangeManagerPassword(r.Context(), tokenOf(r), input.Current, input.New); err != nil {
			http.Error(w, "Unable to change password", 400)
			return
		}
		w.WriteHeader(204)
	})
	routes.HandleFunc("GET /api/manager/users", func(w http.ResponseWriter, r *http.Request) {
		items, err := security.ListManagerUsers(r.Context())
		if err != nil {
			managerAccessError(w, err)
			return
		}
		writeJSON(w, 200, items)
	})
	routes.HandleFunc("POST /api/manager/users", func(w http.ResponseWriter, r *http.Request) {
		var input systemdb.UserCreation
		if !decodeJSON(w, r, &input) {
			return
		}
		user, err := security.CreateManagerUser(r.Context(), input)
		if err != nil {
			http.Error(w, "Unable to create user: check login and password", 400)
			return
		}
		writeJSON(w, 201, user)
	})
	routes.HandleFunc("PUT /api/manager/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathUUID(w, r, "user")
		if !ok {
			return
		}
		var input systemdb.UserAccessUpdate
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := security.UpdateManagerUser(r.Context(), id, input); err != nil {
			if errors.Is(err, systemdb.ErrLastPlatformAdministrator) {
				http.Error(w, "Cannot disable or demote the last platform administrator", 409)
				return
			}
			managerAccessError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	routes.HandleFunc("GET /api/databases/{id}/permissions", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathUUID(w, r, "database")
		if !ok {
			return
		}
		items, err := security.ListManagerDatabaseAccess(r.Context(), id)
		if err != nil {
			managerAccessError(w, err)
			return
		}
		writeJSON(w, 200, items)
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
		if err := security.AssignManagerDatabasePermissions(r.Context(), id, input.Login, input.Permissions); err != nil {
			http.Error(w, "Unable to assign access: check user and system roles", 400)
			return
		}
		w.WriteHeader(204)
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
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
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
			http.Error(w, "Manager authentication unavailable", 503)
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
			writeJSON(w, 200, items)
			return
		case "GET /api/studio/sessions":
			items, err := security.ListManagerStudioSessions(ctx)
			if err != nil {
				managerAccessError(w, err)
				return
			}
			writeJSON(w, 200, items)
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
			writeJSON(w, 200, items)
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
		http.Error(w, "Sign in to ML Manager", 401)
	case errors.Is(err, systemdb.ErrPasswordChangeRequired):
		http.Error(w, "Change your password before continuing", 403)
	case errors.Is(err, systemdb.ErrDatabaseAccessDenied), errors.Is(err, systemdb.ErrManagerAccessDenied), errors.Is(err, systemdb.ErrDatabaseOwnerOnly):
		http.Error(w, "Access denied", 403)
	default:
		http.Error(w, "Manager operation unavailable", 503)
	}
}
