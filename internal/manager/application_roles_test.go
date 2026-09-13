package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type applicationRolesTestBackend struct {
	securityBackend
	project, target uuid.UUID
	last            platform.ApplicationRoleUpdate
	writes          int
	err             error
}

func (backend *applicationRolesTestBackend) AuthorizeManager(_ context.Context, id uuid.UUID, permission string) error {
	if id == backend.allowedID && permission == "admin" {
		return nil
	}
	return systemdb.ErrDatabaseAccessDenied
}
func (backend *applicationRolesTestBackend) GetManagerApplicationRoles(_ context.Context, databaseID, userID uuid.UUID) (platform.ManagerApplicationRoles, error) {
	if databaseID != backend.allowedID || userID != backend.target {
		return platform.ManagerApplicationRoles{}, systemdb.ErrDatabaseAccessDenied
	}
	return platform.ManagerApplicationRoles{ProjectID: backend.project, Available: []platform.ApplicationRoleOption{}, Assignment: systemdb.ApplicationRoleAssignment{RoleIDs: []uuid.UUID{}}}, backend.err
}
func (backend *applicationRolesTestBackend) SetManagerApplicationRoles(_ context.Context, databaseID, userID uuid.UUID, update platform.ApplicationRoleUpdate) (systemdb.ApplicationRoleAssignment, error) {
	if databaseID != backend.allowedID || userID != backend.target {
		return systemdb.ApplicationRoleAssignment{}, systemdb.ErrDatabaseAccessDenied
	}
	backend.writes++
	backend.last = update
	return systemdb.ApplicationRoleAssignment{ProjectID: &backend.project, RoleIDs: update.RoleIDs, Revision: 1}, backend.err
}

func TestManagerApplicationRoleRoutesAreScoped(t *testing.T) {
	backend := &applicationRolesTestBackend{securityBackend: securityBackend{session: systemdb.PortalSession{ID: uuid.MustNew(), UserID: uuid.MustNew(), Login: "developer"}, allowedID: uuid.MustNew()}, project: uuid.MustNew(), target: uuid.MustNew()}
	handler := NewHandlerWithPlatform(appconfig.Default(), backend)
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
	path := "/api/databases/" + backend.allowedID.String() + "/application-roles/" + backend.target.String()
	if w := request("GET", path, "", nil, ""); w.Code != 401 {
		t.Fatalf("anonymous read: %d", w.Code)
	}
	login := request("POST", "/api/manager/login", `{"login":"developer","password":"password"}`, nil, "")
	if login.Code != 200 {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if w := request("GET", path, "", cookie, ""); w.Code != 200 {
		t.Fatalf("role read: %d %s", w.Code, w.Body.String())
	}
	foreign := "/api/databases/" + uuid.MustNew().String() + "/application-roles/" + backend.target.String()
	if w := request("GET", foreign, "", cookie, ""); w.Code != 403 {
		t.Fatalf("foreign database read: %d", w.Code)
	}
	roleID := uuid.MustNew()
	payload, _ := json.Marshal(platform.ApplicationRoleUpdate{ProjectID: backend.project, RoleIDs: []uuid.UUID{roleID}})
	if w := request("PUT", path, string(payload), cookie, "https://foreign.invalid"); w.Code != 403 || backend.writes != 0 {
		t.Fatalf("cross-origin role update: %d", w.Code)
	}
	if w := request("PUT", path, `{"platformAdministrator":true}`, cookie, ""); w.Code != 400 || backend.writes != 0 {
		t.Fatalf("unknown privilege field: %d", w.Code)
	}
	if w := request("PUT", foreign, string(payload), cookie, ""); w.Code != 403 || backend.writes != 0 {
		t.Fatalf("foreign database write: %d", w.Code)
	}
	if w := request("PUT", path, string(payload), cookie, ""); w.Code != 200 || backend.writes != 1 || backend.last.RoleIDs[0] != roleID {
		t.Fatalf("valid write: %d %s", w.Code, w.Body.String())
	}
	backend.err = systemdb.ErrApplicationRolesChanged
	if w := request("PUT", path, string(payload), cookie, ""); w.Code != 409 {
		t.Fatalf("stale write status: %d", w.Code)
	}
	backend.err = nil
	large := platform.ApplicationRoleUpdate{ProjectID: backend.project, RoleIDs: make([]uuid.UUID, systemdb.MaxApplicationRoles)}
	for index := range large.RoleIDs {
		large.RoleIDs[index] = uuid.MustNew()
	}
	payload, _ = json.Marshal(large)
	if w := request("PUT", path, string(payload), cookie, ""); w.Code != 200 {
		t.Fatalf("bounded full role selection rejected: %d", w.Code)
	}
	if w := request("PUT", path, strings.Repeat(" ", 64<<10)+"{}", cookie, ""); w.Code != 400 {
		t.Fatalf("oversized request: %d", w.Code)
	}
}
