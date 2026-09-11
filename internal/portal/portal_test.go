package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestPortalLoginListOpenAndLogout(t *testing.T) {
	t.Parallel()
	databaseID := uuid.MustNew()
	runtime := &fakeRuntime{
		token: "opaque-token",
		session: systemdb.PortalSession{
			ID: uuid.MustNew(), UserID: uuid.MustNew(), Login: "admin", PlatformAdmin: true,
			AbsoluteExpiresAt: time.Now().Add(time.Hour),
		},
		databases: []platform.PortalDatabase{{
			ID: databaseID, Name: "Продажи", Mode: systemdb.DatabasePrimary, AllowNewSessions: true,
		}},
	}
	handler := NewHandler(runtime)
	login := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"admin","password":"top secret"}`))
	login.Header.Set("Content-Type", "application/json")
	login.RemoteAddr = "192.0.2.10:5000"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, login)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "top secret") || response.Header().Get("Set-Cookie") == "" {
		t.Fatalf("login status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookies = %+v", cookies)
	}
	portalRequest := httptest.NewRequest(http.MethodGet, "/api/portal", nil)
	portalRequest.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, portalRequest)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Продажи") {
		t.Fatalf("portal status=%d body=%s", response.Code, response.Body.String())
	}
	open := httptest.NewRequest(http.MethodPost, "/api/databases/"+databaseID.String()+"/open", nil)
	open.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, open)
	if response.Code != http.StatusForbidden {
		t.Fatalf("open without CSRF status=%d", response.Code)
	}
	open.Header.Set("X-ML-CSRF", "1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, open)
	if response.Code != http.StatusOK || runtime.opened != databaseID {
		t.Fatalf("open status=%d body=%s id=%s", response.Code, response.Body.String(), runtime.opened)
	}
	logout := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logout.AddCookie(cookies[0])
	logout.Header.Set("X-ML-CSRF", "1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, logout)
	if response.Code != http.StatusNoContent || !runtime.loggedOut {
		t.Fatalf("logout status=%d loggedOut=%v", response.Code, runtime.loggedOut)
	}
}

func TestMLAppPageBootstrapAndAssets(t *testing.T) {
	t.Parallel()
	databaseID := uuid.MustNew()
	runtime := &fakeRuntime{token: "token", session: systemdb.PortalSession{ID: uuid.MustNew(), UserID: uuid.MustNew(), Login: "admin"}}
	handler := NewHandler(runtime)
	cookie := &http.Cookie{Name: sessionCookie, Value: runtime.token}

	pageRequest := httptest.NewRequest(http.MethodGet, "/app/"+databaseID.String(), nil)
	pageRequest.AddCookie(cookie)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, pageRequest)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<ml-app-shell") {
		t.Fatalf("app page status=%d body=%s", page.Code, page.Body.String())
	}

	bootstrapRequest := httptest.NewRequest(http.MethodGet, "/api/databases/"+databaseID.String()+"/app-bootstrap", nil)
	bootstrapRequest.AddCookie(cookie)
	bootstrapResponse := httptest.NewRecorder()
	handler.ServeHTTP(bootstrapResponse, bootstrapRequest)
	var bootstrap struct {
		Database struct {
			Name string `json:"name"`
		} `json:"database"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.Unmarshal(bootstrapResponse.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if bootstrapResponse.Code != http.StatusOK || bootstrap.Database.Name != "Продажи" || bootstrap.User.Login != "admin" {
		t.Fatalf("bootstrap status=%d value=%+v body=%s", bootstrapResponse.Code, bootstrap, bootstrapResponse.Body.String())
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/ml-app/app.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "customElements.define") {
		t.Fatalf("asset status=%d body=%s", asset.Code, asset.Body.String())
	}
}

func TestPortalDoesNotDiscloseAuthenticationFailure(t *testing.T) {
	t.Parallel()
	runtime := &fakeRuntime{failure: systemdb.ErrInvalidCredentials}
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"unknown","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(runtime).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "unknown") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestLoginLimiterIsBoundedAndExpires(t *testing.T) {
	t.Parallel()
	limiter := newLoginLimiter()
	now := time.Now()
	for range 10 {
		if !limiter.Allow("client", now) {
			t.Fatal("login was limited too early")
		}
		limiter.Failed("client", now)
	}
	if limiter.Allow("client", now) {
		t.Fatal("eleventh login was not limited")
	}
	if !limiter.Allow("client", now.Add(time.Minute)) {
		t.Fatal("login limit did not expire")
	}
}

type fakeRuntime struct {
	token     string
	session   systemdb.PortalSession
	databases []platform.PortalDatabase
	opened    uuid.UUID
	loggedOut bool
	failure   error
}

func (runtime *fakeRuntime) LoginPortal(context.Context, string, string, string, string) (platform.PortalLogin, error) {
	return platform.PortalLogin{Token: runtime.token, Session: runtime.session}, runtime.failure
}
func (runtime *fakeRuntime) AuthenticatePortal(context.Context, string) (systemdb.PortalSession, error) {
	return runtime.session, runtime.failure
}
func (runtime *fakeRuntime) LogoutPortal(context.Context, string) error {
	runtime.loggedOut = true
	return runtime.failure
}
func (runtime *fakeRuntime) ChangePortalPassword(context.Context, string, string, string) error {
	return runtime.failure
}
func (runtime *fakeRuntime) LoadPortal(context.Context, string) (systemdb.PortalSession, []platform.PortalDatabase, error) {
	return runtime.session, runtime.databases, runtime.failure
}
func (runtime *fakeRuntime) OpenPortalDatabase(_ context.Context, _ string, id uuid.UUID) (systemdb.DatabaseSession, error) {
	runtime.opened = id
	if runtime.failure != nil {
		return systemdb.DatabaseSession{}, runtime.failure
	}
	return systemdb.DatabaseSession{
		ID: uuid.MustNew(), PortalSessionID: runtime.session.ID, DatabaseID: id,
		DatabaseName: "Продажи", UserID: runtime.session.UserID, Login: runtime.session.Login,
	}, nil
}
func (runtime *fakeRuntime) ResumePortalDatabase(ctx context.Context, token string, id uuid.UUID) (systemdb.DatabaseSession, error) {
	return runtime.OpenPortalDatabase(ctx, token, id)
}
func (runtime *fakeRuntime) AcknowledgeSessionMessage(context.Context, string, uuid.UUID) error {
	return runtime.failure
}
