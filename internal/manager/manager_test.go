package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagerServesUIAndRunningServiceStatus(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/health" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}
	configuration := appconfig.Default()
	handler := newHandler(configuration, client, nil, nil)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d", page.Code)
	}
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var status struct {
		Running bool   `json:"running"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Running || status.URL != "http://127.0.0.1:8090" {
		t.Fatalf("status = %+v", status)
	}
}

func TestManagerReportsStoppedService(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}
	configuration := appconfig.Default()
	response := httptest.NewRecorder()
	newHandler(configuration, client, nil, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var status struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Running {
		t.Fatal("stopped service reported as running")
	}
}

func TestManagerInitialAdministratorWizard(t *testing.T) {
	t.Parallel()

	setup := &fakeAdministratorSetup{required: true}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup, nil)
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, httptest.NewRequest(http.MethodGet, "/api/setup", nil))
	if stateResponse.Code != http.StatusOK || !strings.Contains(stateResponse.Body.String(), `"required":true`) {
		t.Fatalf("state status = %d, body = %s", stateResponse.Code, stateResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/api/setup/administrator", bytes.NewBufferString(`{"login":"admin","password":"secure password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"recoveryCodes":["CODE"]`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if setup.login != "admin" || setup.password != "secure password" {
		t.Fatalf("login = %q, password = %q", setup.login, setup.password)
	}
}

func TestManagerSetupRejectsUnavailableInvalidAndRepeatedRequests(t *testing.T) {
	t.Parallel()

	unavailable := newHandler(appconfig.Default(), http.DefaultClient, nil, nil)
	response := httptest.NewRecorder()
	unavailable.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/setup/administrator", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", response.Code)
	}

	setup := &fakeAdministratorSetup{failure: systemdb.ErrInitialAdministratorExists}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/setup/administrator", strings.NewReader(`{"login":"admin","password":"secure password"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("repeated status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/setup/administrator", strings.NewReader(`{}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content-type status = %d", response.Code)
	}
}

func TestManagerPostgreSQLSetupAPI(t *testing.T) {
	t.Parallel()

	setup := &fakePlatformSetup{state: platform.State{}}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup, setup)
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, httptest.NewRequest(http.MethodGet, "/api/postgres", nil))
	if stateResponse.Code != http.StatusOK || !strings.Contains(stateResponse.Body.String(), `"configured":false`) {
		t.Fatalf("state status=%d body=%s", stateResponse.Code, stateResponse.Body.String())
	}
	payload := `{"host":"localhost","port":5432,"administratorDatabase":"postgres","administratorUser":"postgres","administratorPassword":"secret","sslMode":"disable","systemDatabase":"metalab_system","technicalUser":"metalab_service"}`
	for _, path := range []string{"/api/postgres/check", "/api/postgres/provision"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code < 200 || response.Code >= 300 || strings.Contains(response.Body.String(), "secret") {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	if setup.request.AdministratorPassword != "secret" || setup.checks != 1 || setup.provisions != 1 {
		t.Fatalf("request=%+v checks=%d provisions=%d", setup.request, setup.checks, setup.provisions)
	}
}

func TestManagerDatabaseRegistryAPI(t *testing.T) {
	t.Parallel()

	id := uuid.MustNew()
	setup := &fakePlatformSetup{databases: []systemdb.RegisteredDatabase{{ID: id, Name: "Продажи", PhysicalID: uuid.MustNew(), State: systemdb.DatabaseStopped}}}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup, setup)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/databases", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Продажи") {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	payload := `{"name":"Склад","host":"localhost","port":5432,"database":"warehouse","user":"ml","password":"do-not-return","sslMode":"disable"}`
	request := httptest.NewRequest(http.MethodPost, "/api/databases", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "do-not-return") || setup.registerRequest.Database != "warehouse" {
		t.Fatalf("register status=%d body=%s request=%+v", response.Code, response.Body.String(), setup.registerRequest)
	}
	debugPayload := `{"name":"Продажи — отладка","copyData":false,"host":"localhost","port":5432,"administratorDatabase":"postgres","administratorUser":"postgres","administratorPassword":"do-not-return","sslMode":"disable","targetDatabase":"sales_debug","technicalUser":"sales_debug_role"}`
	request = httptest.NewRequest(http.MethodPost, "/api/databases/"+id.String()+"/debug", strings.NewReader(debugPayload))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "do-not-return") || setup.debugSource != id || setup.debugRequest.TargetDatabase != "sales_debug" {
		t.Fatalf("debug status=%d body=%s source=%s request=%+v", response.Code, response.Body.String(), setup.debugSource, setup.debugRequest)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/databases/"+id.String(), nil))
	if response.Code != http.StatusNoContent || setup.unregistered != id {
		t.Fatalf("delete status=%d id=%s", response.Code, setup.unregistered)
	}
}

func TestManagerDatabaseRegistryReportsConflicts(t *testing.T) {
	t.Parallel()

	setup := &fakePlatformSetup{registryError: systemdb.ErrPhysicalDatabaseExists}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup, setup)
	payload := `{"name":"Duplicate","host":"localhost","port":5432,"database":"app","user":"ml","password":"secret","sslMode":"disable"}`
	request := httptest.NewRequest(http.MethodPost, "/api/databases", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/databases/not-a-uuid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid identifier status=%d body=%s", response.Code, response.Body.String())
	}
}

type fakeAdministratorSetup struct {
	required bool
	login    string
	password string
	failure  error
}

func (setup *fakeAdministratorSetup) InitialSetupRequired(context.Context) (bool, error) {
	return setup.required, setup.failure
}

func (setup *fakeAdministratorSetup) CreateInitial(_ context.Context, login, password string) (systemdb.Administrator, []string, error) {
	setup.login, setup.password = login, password
	return systemdb.Administrator{Login: login}, []string{"CODE"}, setup.failure
}

type fakePlatformSetup struct {
	fakeAdministratorSetup
	state           platform.State
	request         platform.ProvisionRequest
	checks          int
	provisions      int
	databases       []systemdb.RegisteredDatabase
	registerRequest platform.RegisterDatabaseRequest
	debugRequest    platform.CreateDebugDatabaseRequest
	debugSource     uuid.UUID
	unregistered    uuid.UUID
	registryError   error
}

func (setup *fakePlatformSetup) State() platform.State { return setup.state }
func (setup *fakePlatformSetup) CheckPostgreSQL(_ context.Context, request platform.ProvisionRequest) (postgresadmin.Check, error) {
	setup.request, setup.checks = request, setup.checks+1
	return postgresadmin.Check{Version: "16.14", Encoding: "UTF8", CanCreateDB: true, CanCreateRole: true}, nil
}
func (setup *fakePlatformSetup) ProvisionPostgreSQL(_ context.Context, request platform.ProvisionRequest) (postgresadmin.Check, error) {
	setup.request, setup.provisions = request, setup.provisions+1
	setup.state = platform.State{Configured: true, Connected: true, Connection: &postgresconn.Descriptor{Host: request.Host}}
	return postgresadmin.Check{Version: "16.14", Encoding: "UTF8", CanCreateDB: true, CanCreateRole: true}, nil
}
func (setup *fakePlatformSetup) ListDatabases(context.Context) ([]systemdb.RegisteredDatabase, error) {
	return setup.databases, setup.registryError
}
func (setup *fakePlatformSetup) RegisterDatabase(_ context.Context, request platform.RegisterDatabaseRequest) (systemdb.RegisteredDatabase, error) {
	setup.registerRequest = request
	return systemdb.RegisteredDatabase{ID: uuid.MustNew(), Name: request.Name, PhysicalID: uuid.MustNew(), State: systemdb.DatabaseStopped}, setup.registryError
}
func (setup *fakePlatformSetup) CreateDebugDatabase(_ context.Context, source uuid.UUID, request platform.CreateDebugDatabaseRequest) (systemdb.RegisteredDatabase, error) {
	setup.debugSource, setup.debugRequest = source, request
	return systemdb.RegisteredDatabase{ID: uuid.MustNew(), Name: request.Name, PhysicalID: uuid.MustNew(), Mode: systemdb.DatabaseDebug, State: systemdb.DatabaseStopped}, setup.registryError
}
func (setup *fakePlatformSetup) UnregisterDatabase(_ context.Context, id uuid.UUID) error {
	setup.unregistered = id
	return setup.registryError
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
