package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
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
			ID: databaseID, Name: "Продажи", Mode: systemdb.DatabasePrimary,
			AccessLevel: systemdb.DatabaseOwner, State: systemdb.DatabaseRunning, AllowNewSessions: true,
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
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Продажи") || !strings.Contains(response.Body.String(), `"accessLevel":"owner"`) {
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

func TestPortalContainsOnlyAppEntryAndHidesInaccessibleDatabase(t *testing.T) {
	t.Parallel()
	databaseID := uuid.MustNew()
	handler := NewHandler(&fakeRuntime{
		token: "token", session: systemdb.PortalSession{ID: uuid.MustNew(), UserID: uuid.MustNew(), Login: "admin"},
		failure: systemdb.ErrDatabaseAccessDenied,
	})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), "ML Studio") || strings.Contains(page.Body.String(), "Studio/App") || strings.Contains(page.Body.String(), "data.databases.length===1") {
		t.Fatalf("portal page status=%d body=%s", page.Code, page.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/api/databases/"+databaseID.String()+"/open", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	request.Header.Set("X-ML-CSRF", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("inaccessible database status=%d body=%s", response.Code, response.Body.String())
	}
	direct := httptest.NewRequest(http.MethodGet, "/app/"+databaseID.String(), nil)
	direct.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, direct)
	if response.Code != http.StatusNotFound {
		t.Fatalf("direct inaccessible database status=%d body=%s", response.Code, response.Body.String())
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
		Navigation []struct {
			Name       string   `json:"name"`
			Kind       string   `json:"kind"`
			KindTitle  string   `json:"kindTitle"`
			Operations []string `json:"operations"`
		} `json:"navigation"`
	}
	if err := json.Unmarshal(bootstrapResponse.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if bootstrapResponse.Code != http.StatusOK || bootstrap.Database.Name != "Продажи" || bootstrap.User.Login != "admin" || len(bootstrap.Navigation) != 3 || bootstrap.Navigation[1].Name != "Товары" {
		t.Fatalf("bootstrap status=%d value=%+v body=%s", bootstrapResponse.Code, bootstrap, bootstrapResponse.Body.String())
	}
	// Глобальный поиск строится из этих полей: без вида объекта два одинаково
	// названных объекта неразличимы, а без прав он предложил бы «создать» там,
	// где создание запрещено.
	goods, document := bootstrap.Navigation[1], bootstrap.Navigation[2]
	if goods.KindTitle != "Справочник" || !slices.Contains(goods.Operations, "create") {
		t.Fatalf("catalog navigation item = %+v", goods)
	}
	if document.KindTitle != "Документ" || slices.Contains(document.Operations, "create") {
		t.Fatalf("document navigation item = %+v", document)
	}

	formRequest := httptest.NewRequest(http.MethodGet, "/api/databases/"+databaseID.String()+"/forms/catalogs/Товары/list", nil)
	formRequest.AddCookie(cookie)
	formResponse := httptest.NewRecorder()
	handler.ServeHTTP(formResponse, formRequest)
	if formResponse.Code != http.StatusOK || !strings.Contains(formResponse.Body.String(), `"title":"Товары"`) || !strings.Contains(formResponse.Body.String(), `"kind":"table"`) {
		t.Fatalf("form status=%d body=%s", formResponse.Code, formResponse.Body.String())
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/databases/"+databaseID.String()+"/lists/catalogs/Товары?limit=50&search=болт&searchField=Description&sort=Description&direction=desc&filter=Description%3DБолт", nil)
	listRequest.AddCookie(cookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"Description":"Болт"`) || runtime.listRequest.Limit != 50 || runtime.listRequest.Search != "болт" || runtime.listRequest.SearchField != "Description" || runtime.listRequest.SortField != "Description" || !runtime.listRequest.Descending || len(runtime.listRequest.Filters) != 1 {
		t.Fatalf("list status=%d request=%+v body=%s", listResponse.Code, runtime.listRequest, listResponse.Body.String())
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
	token        string
	session      systemdb.PortalSession
	databases    []platform.PortalDatabase
	opened       uuid.UUID
	loggedOut    bool
	failure      error
	loginFailure error
	listRequest  metadata.DynamicListRequest
}

func (runtime *fakeRuntime) LoginPortal(context.Context, string, string, string, string) (platform.PortalLogin, error) {
	if runtime.loginFailure != nil {
		return platform.PortalLogin{}, runtime.loginFailure
	}
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
func (runtime *fakeRuntime) LoadApplicationObjects(_ context.Context, _ string, _ uuid.UUID, preferences []string) (platform.ApplicationObjects, error) {
	language := metadata.TitleLanguage{Code: "ru", Default: "ru"}
	if len(preferences) > 0 {
		language.Code = preferences[0]
	}
	return platform.ApplicationObjects{Language: language, Objects: []platform.ApplicationObject{
		{Kind: metadata.CatalogKind, Name: "Товары", Title: "Товары", Operations: []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionCreate}},
		{Kind: metadata.DocumentKind, Name: "ПродажаТоваров", Title: "Продажа товаров", Operations: []metadata.PermissionOperation{metadata.PermissionRead}},
	}}, runtime.failure
}
func (runtime *fakeRuntime) LoadApplicationForm(_ context.Context, _ string, _ uuid.UUID, kind metadata.Kind, name string, formKind metadata.FormKind, _ []string) (platform.ApplicationForm, error) {
	return platform.ApplicationForm{Descriptor: metadata.FormDescriptor{
		Kind: formKind, ObjectKind: kind, ObjectID: uuid.MustNew(), ObjectName: name, Title: name, Generated: true,
		Fields:   []metadata.FormField{{Name: "Description", Title: "Наименование", Types: []metadata.Type{{Kind: metadata.StringType}}}},
		Commands: []metadata.FormCommand{{Name: "Create", Title: "Создать"}},
	}}, runtime.failure
}
func (runtime *fakeRuntime) LoadApplicationList(_ context.Context, _ string, _ uuid.UUID, _ metadata.Kind, _ string, request metadata.DynamicListRequest) (platform.ApplicationListPage, error) {
	runtime.listRequest = request
	return platform.ApplicationListPage{PageSize: request.Limit, Rows: []platform.ApplicationListRow{{Reference: uuid.MustNew(), Values: map[string]string{"Description": "Болт"}}}}, runtime.failure
}
func (runtime *fakeRuntime) GetApplicationObject(context.Context, string, uuid.UUID, metadata.Kind, string, string) (platform.ApplicationObjectState, error) {
	return platform.ApplicationObjectState{Reference: uuid.MustNew().String(), Fields: map[string]metadata.Value{}}, runtime.failure
}
func (runtime *fakeRuntime) SaveApplicationObject(context.Context, string, uuid.UUID, metadata.Kind, string, platform.ApplicationObjectWrite) (platform.ApplicationObjectState, error) {
	return platform.ApplicationObjectState{Reference: uuid.MustNew().String(), Fields: map[string]metadata.Value{}}, runtime.failure
}
func (runtime *fakeRuntime) PostApplicationDocument(context.Context, string, uuid.UUID, string, string) (platform.ApplicationObjectState, error) {
	return platform.ApplicationObjectState{Posted: true, Fields: map[string]metadata.Value{}}, runtime.failure
}
func (runtime *fakeRuntime) UndoApplicationDocumentPosting(context.Context, string, uuid.UUID, string, string) (platform.ApplicationObjectState, error) {
	return platform.ApplicationObjectState{Fields: map[string]metadata.Value{}}, runtime.failure
}
func (runtime *fakeRuntime) SetApplicationDeletionMark(_ context.Context, _ string, _ uuid.UUID, _ metadata.Kind, _ string, _ string, mark bool) (platform.ApplicationObjectState, error) {
	return platform.ApplicationObjectState{DeletionMark: mark, Fields: map[string]metadata.Value{}}, runtime.failure
}

// The reader's language comes from the request until accounts carry one of
// their own: the browser states preferences in order, and the order is what
// decides - a q-value is a preference, not decoration.
func TestRequestLanguagesReadsThePreferenceOrder(t *testing.T) {
	t.Parallel()
	for header, want := range map[string][]string{
		"":                                 nil,
		"   ":                              nil,
		"uk":                               {"uk"},
		"uk-UA,uk;q=0.9,ru;q=0.8,en;q=0.7": {"uk-UA", "uk", "ru", "en"},
		"ru;q=0.3, uk;q=0.9, en;q=0.6":     {"uk", "en", "ru"},
		"*":                                nil,
		"uk;q=0":                           nil,
		"uk;q=bad":                         nil,
	} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			request.Header.Set("Accept-Language", header)
		}
		got := requestLanguages(request)
		if len(got) != len(want) {
			t.Fatalf("%q -> %v, want %v", header, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("%q -> %v, want %v", header, got, want)
			}
		}
	}
}

// Отказ во втором входе — не неудачная попытка: пароль верный, и считать её
// в счёт лимита значит позволить пользователю заблокировать себе вход
// перезагрузкой страницы, которой он уже пользуется.
func TestSecondLoginIsRefusedWithItsOwnStatusAndDoesNotCountAsFailure(t *testing.T) {
	t.Parallel()
	runtime := &fakeRuntime{loginFailure: systemdb.ErrPortalSessionActive}
	handler := NewHandler(runtime)
	for range 12 {
		request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"admin","password":"secret"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("refused login status = %d, body = %s", response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "другом устройстве") {
			t.Fatalf("refusal does not say why: %s", response.Body.String())
		}
	}
}

// Живая страница обязана подавать признак жизни, иначе её собственный сеанс
// через две минуты выглядит брошенным.
func TestPortalPageKeepsItsSessionAlive(t *testing.T) {
	t.Parallel()
	page, err := assets.ReadFile("ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(page)
	for _, fragment := range []string{"startHeartbeat", "setInterval", "60000", "stopHeartbeat"} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("portal page has no heartbeat: %q is missing", fragment)
		}
	}
}
