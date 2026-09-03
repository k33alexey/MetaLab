// Package portal implements the shared browser entry point for ML App and ML Studio.
package portal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const sessionCookie = "ml_session"

//go:embed ui/index.html
var assets embed.FS

type runtime interface {
	LoginPortal(context.Context, string, string, string, string) (platform.PortalLogin, error)
	LogoutPortal(context.Context, string) error
	ChangePortalPassword(context.Context, string, string, string) error
	LoadPortal(context.Context, string) (systemdb.PortalSession, []platform.PortalDatabase, error)
	OpenPortalDatabase(context.Context, string, uuid.UUID) (systemdb.DatabaseSession, error)
	ResumePortalDatabase(context.Context, string, uuid.UUID) (systemdb.DatabaseSession, error)
	AcknowledgeSessionMessage(context.Context, string, uuid.UUID) error
}

// NewHandler creates the public Portal HTTP surface.
func NewHandler(platformRuntime runtime) http.Handler {
	routes := http.NewServeMux()
	loginLimits := newLoginLimiter()
	routes.HandleFunc("GET /{$}", func(response http.ResponseWriter, _ *http.Request) {
		page, err := assets.ReadFile("ui/index.html")
		if err != nil {
			http.Error(response, "ML Portal UI unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(page)
	})
	routes.HandleFunc("GET /api/health", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "database": "postgresql"})
	})
	routes.HandleFunc("POST /api/login", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		limitKey := remoteHost(request.RemoteAddr) + "\x00" + strings.ToLower(strings.TrimSpace(input.Login))
		if !loginLimits.Allow(limitKey, time.Now()) {
			response.Header().Set("Retry-After", "60")
			http.Error(response, "Too many login attempts", http.StatusTooManyRequests)
			return
		}
		login, err := platformRuntime.LoginPortal(
			request.Context(), input.Login, input.Password, remoteHost(request.RemoteAddr), request.UserAgent(),
		)
		if err != nil {
			loginLimits.Failed(limitKey, time.Now())
			http.Error(response, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		loginLimits.Succeeded(limitKey)
		setSessionCookie(response, request, login.Token, time.Until(login.Session.AbsoluteExpiresAt))
		writeJSON(response, http.StatusOK, login.Session)
	})
	routes.HandleFunc("GET /api/portal", func(response http.ResponseWriter, request *http.Request) {
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		session, databases, err := platformRuntime.LoadPortal(request.Context(), token)
		if err != nil {
			clearSessionCookie(response, request)
			http.Error(response, "Authentication required", http.StatusUnauthorized)
			return
		}
		writeJSON(response, http.StatusOK, struct {
			Session   systemdb.PortalSession    `json:"session"`
			Databases []platform.PortalDatabase `json:"databases"`
		}{Session: session, Databases: databases})
	})
	routes.HandleFunc("POST /api/logout", func(response http.ResponseWriter, request *http.Request) {
		if !requireCSRF(response, request) {
			return
		}
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		_ = platformRuntime.LogoutPortal(request.Context(), token)
		clearSessionCookie(response, request)
		response.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("POST /api/password", func(response http.ResponseWriter, request *http.Request) {
		if !requireCSRF(response, request) {
			return
		}
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		var input struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if !decodeJSON(response, request, &input) {
			return
		}
		if err := platformRuntime.ChangePortalPassword(request.Context(), token, input.CurrentPassword, input.NewPassword); err != nil {
			http.Error(response, "Unable to change password", http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("POST /api/databases/{id}/open", func(response http.ResponseWriter, request *http.Request) {
		if !requireCSRF(response, request) {
			return
		}
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.Error(response, "Invalid database identifier", http.StatusBadRequest)
			return
		}
		session, err := platformRuntime.OpenPortalDatabase(request.Context(), token, id)
		switch {
		case errors.Is(err, systemdb.ErrSessionNotFound):
			http.Error(response, "Authentication required", http.StatusUnauthorized)
		case errors.Is(err, systemdb.ErrDatabaseNotFound):
			http.Error(response, err.Error(), http.StatusNotFound)
		case errors.Is(err, systemdb.ErrDatabaseNotRunning), errors.Is(err, systemdb.ErrNewSessionsForbidden):
			http.Error(response, err.Error(), http.StatusConflict)
		case err != nil:
			http.Error(response, "Unable to open database", http.StatusServiceUnavailable)
		default:
			writeJSON(response, http.StatusOK, struct {
				Session systemdb.DatabaseSession `json:"session"`
				URL     string                   `json:"url"`
			}{Session: session, URL: "/app/" + id.String()})
		}
	})
	routes.HandleFunc("GET /app/{id}", func(response http.ResponseWriter, request *http.Request) {
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.NotFound(response, request)
			return
		}
		session, err := platformRuntime.ResumePortalDatabase(request.Context(), token, id)
		if err != nil {
			http.Error(response, "Database session unavailable", http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write([]byte("<!doctype html><meta charset=utf-8><title>ML App</title><h1>ML App</h1><p>База «" + htmlText(session.DatabaseName) + "» запущена. Прикладной интерфейс будет добавлен в следующих итерациях.</p><p><a href=\"/\">Вернуться в Portal</a></p><script>const databaseId='" + id.String() + "';async function poll(){const response=await fetch(`/api/databases/${databaseId}/session`);if(!response.ok){location.href='/';return}const session=await response.json();if(session.message){alert(session.message);await fetch(`/api/sessions/${session.id}/message/ack`,{method:'POST',headers:{'X-ML-CSRF':'1'}})}}setInterval(poll,5000);poll();</script>"))
	})
	routes.HandleFunc("GET /api/databases/{id}/session", func(response http.ResponseWriter, request *http.Request) {
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.Error(response, "Invalid database identifier", http.StatusBadRequest)
			return
		}
		session, err := platformRuntime.ResumePortalDatabase(request.Context(), token, id)
		if err != nil {
			http.Error(response, "Database session unavailable", http.StatusConflict)
			return
		}
		writeJSON(response, http.StatusOK, session)
	})
	routes.HandleFunc("POST /api/sessions/{id}/message/ack", func(response http.ResponseWriter, request *http.Request) {
		if !requireCSRF(response, request) {
			return
		}
		token, ok := requireToken(response, request)
		if !ok {
			return
		}
		id, err := uuid.Parse(request.PathValue("id"))
		if err != nil {
			http.Error(response, "Invalid session identifier", http.StatusBadRequest)
			return
		}
		if err := platformRuntime.AcknowledgeSessionMessage(request.Context(), token, id); err != nil {
			http.Error(response, "Session unavailable", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	return securityHeaders(routes)
}

type loginAttempt struct {
	windowStart time.Time
	failures    int
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{attempts: make(map[string]loginAttempt)} }

func (limiter *loginLimiter) Allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	attempt := limiter.attempts[key]
	if now.Sub(attempt.windowStart) >= time.Minute {
		delete(limiter.attempts, key)
		return true
	}
	return attempt.failures < 10
}

func (limiter *loginLimiter) Failed(key string, now time.Time) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.attempts) >= 10_000 {
		for existing, attempt := range limiter.attempts {
			if now.Sub(attempt.windowStart) >= time.Minute {
				delete(limiter.attempts, existing)
			}
		}
		if len(limiter.attempts) >= 10_000 {
			return
		}
	}
	attempt := limiter.attempts[key]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) >= time.Minute {
		attempt = loginAttempt{windowStart: now}
	}
	attempt.failures++
	limiter.attempts[key] = attempt
}

func (limiter *loginLimiter) Succeeded(key string) {
	limiter.mu.Lock()
	delete(limiter.attempts, key)
	limiter.mu.Unlock()
}

func requireToken(response http.ResponseWriter, request *http.Request) (string, bool) {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		http.Error(response, "Authentication required", http.StatusUnauthorized)
		return "", false
	}
	return cookie.Value, true
}

func requireCSRF(response http.ResponseWriter, request *http.Request) bool {
	if request.Header.Get("X-ML-CSRF") != "1" {
		http.Error(response, "CSRF check failed", http.StatusForbidden)
		return false
	}
	return true
}

func setSessionCookie(response http.ResponseWriter, request *http.Request, token string, lifetime time.Duration) {
	if lifetime < 0 {
		lifetime = 0
	}
	http.SetCookie(response, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: requestIsSecure(request),
		SameSite: http.SameSiteStrictMode, MaxAge: int(lifetime.Seconds()),
	})
}

func clearSessionCookie(response http.ResponseWriter, request *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name: sessionCookie, Path: "/", HttpOnly: true, Secure: requestIsSecure(request),
		SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

func requestIsSecure(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) bool {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(response, "Invalid request", http.StatusBadRequest)
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

func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}

func htmlText(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(response, request)
	})
}
