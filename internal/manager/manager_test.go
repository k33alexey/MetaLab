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
	"github.com/k33alexey/MetaLab/internal/systemdb"
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
	handler := newHandler(configuration, client, nil)

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
	newHandler(configuration, client, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
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
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup)
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

	unavailable := newHandler(appconfig.Default(), http.DefaultClient, nil)
	response := httptest.NewRecorder()
	unavailable.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/setup/administrator", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", response.Code)
	}

	setup := &fakeAdministratorSetup{failure: systemdb.ErrInitialAdministratorExists}
	handler := newHandler(appconfig.Default(), http.DefaultClient, setup)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
