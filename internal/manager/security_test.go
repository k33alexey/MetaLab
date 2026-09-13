package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type securityBackend struct {
	fakePlatformSetup
	managerSecurity
	session   systemdb.PortalSession
	allowedID uuid.UUID
	checks    []string
}

func (backend *securityBackend) LoginManager(_ context.Context, login, password, _, _ string) (platform.PortalLogin, error) {
	if login != "developer" || password != "password" {
		return platform.PortalLogin{}, systemdb.ErrInvalidCredentials
	}
	return platform.PortalLogin{Token: "manager-token", Session: backend.session}, nil
}
func (backend *securityBackend) AuthenticateManager(_ context.Context, token string) (systemdb.PortalSession, error) {
	if token != "manager-token" {
		return systemdb.PortalSession{}, systemdb.ErrSessionNotFound
	}
	return backend.session, nil
}
func (backend *securityBackend) AuthorizeManager(_ context.Context, id uuid.UUID, permission string) error {
	backend.checks = append(backend.checks, permission)
	if permission == "developer" && backend.session.MetadataAdmin {
		return nil
	}
	if permission == "studio" && id == backend.allowedID && backend.session.MetadataAdmin {
		return nil
	}
	return systemdb.ErrDatabaseAccessDenied
}
func (backend *securityBackend) ListManagerDatabases(context.Context) ([]platform.ManagerDatabase, error) {
	return []platform.ManagerDatabase{{RegisteredDatabase: systemdb.RegisteredDatabase{ID: backend.allowedID, PhysicalID: uuid.MustNew(), Name: "My debug database"}, Permissions: systemdb.DatabasePermissions{Studio: true}}}, nil
}

func TestManagerProductionBoundaryRequiresIdentityAndScope(t *testing.T) {
	backend := &securityBackend{fakePlatformSetup: fakePlatformSetup{state: platform.State{Configured: true, Connected: true}}, session: systemdb.PortalSession{ID: uuid.MustNew(), UserID: uuid.MustNew(), Login: "developer", MetadataAdmin: true, AbsoluteExpiresAt: time.Now().Add(time.Hour)}, allowedID: uuid.MustNew()}
	launcher := &fakeStudioLauncher{}
	handler := NewHandlerWithPlatformAndStudio(appconfig.Default(), backend, launcher)
	request := func(method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:12345"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/databases", "/api/postgres", "/api/studio/sessions", "/api/manager/users"} {
		if w := request("GET", path, "", nil, ""); w.Code != 401 {
			t.Fatalf("anonymous %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	login := request("POST", "/api/manager/login", `{"login":"developer","password":"password"}`, nil, "")
	if login.Code != 200 {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	if strings.Contains(login.Body.String(), "manager-token") {
		t.Fatal("raw token exposed to JS")
	}
	if w := request("GET", "/api/databases", "", cookie, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "My debug database") {
		t.Fatalf("personal list: %d %s", w.Code, w.Body.String())
	}
	for _, endpoint := range []struct{ method, path, body string }{
		{"POST", "/api/postgres/provision", `{}`}, {"POST", "/api/databases", `{}`}, {"POST", "/api/manager/users", `{}`},
		{"PUT", "/api/manager/users/" + backend.session.UserID.String(), `{"platformAdministrator":true,"enabled":true}`},
		{"POST", "/api/databases/" + backend.allowedID.String() + "/start", `{}`}, {"PUT", "/api/databases/" + backend.allowedID.String() + "/permissions", `{}`},
		{"DELETE", "/api/studio/sessions/" + backend.allowedID.String(), ""},
		{"POST", "/api/studio/open", `{"databaseId":"` + uuid.MustNew().String() + `","projectPath":"/project"}`},
	} {
		if w := request(endpoint.method, endpoint.path, endpoint.body, cookie, ""); w.Code != 403 {
			t.Fatalf("developer %s: %d %s", endpoint.path, w.Code, w.Body.String())
		}
	}
	if launcher.path != "" || backend.provisions != 0 {
		t.Fatal("denied request reached a side effect")
	}
	body := `{"databaseId":"` + backend.allowedID.String() + `","projectPath":"/project"}`
	if w := request("POST", "/api/studio/open", body, cookie, ""); w.Code != 204 {
		t.Fatalf("assigned Studio: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", "/api/studio/open", body, cookie, "https://evil.example"); w.Code != 403 {
		t.Fatalf("cross-origin request: %d", w.Code)
	}
	backend.session.MustChangePassword = true
	if w := request("GET", "/api/databases", "", cookie, ""); w.Code != 403 {
		t.Fatalf("mandatory password bypass: %d", w.Code)
	}
	wrong := *cookie
	wrong.Value = "portal-token"
	if w := request("GET", "/api/databases", "", &wrong, ""); w.Code != 401 {
		t.Fatalf("Portal token accepted: %d", w.Code)
	}
	foreignHost := httptest.NewRecorder()
	handler.ServeHTTP(foreignHost, httptest.NewRequest("GET", "http://attacker.example/", nil))
	if foreignHost.Code != 403 {
		t.Fatalf("nonlocal host accepted: %d", foreignHost.Code)
	}
}

func TestManagerFailsClosedWithoutAuthenticationBackend(t *testing.T) {
	backend := &fakePlatformSetup{state: platform.State{Configured: true, Connected: true}}
	handler := NewHandlerWithPlatform(appconfig.Default(), backend)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "http://localhost/api/databases", strings.NewReader(`{}`)))
	if w.Code != 503 {
		t.Fatalf("unprotected fallback: %d", w.Code)
	}
}
