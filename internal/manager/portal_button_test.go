package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

// The button exists so that nobody types the address by hand, so what it must
// get right is the address.
func TestManagerOpensThePortalInTheBrowser(t *testing.T) {
	t.Parallel()
	settings := appconfig.Default()
	settings.Service.Listen = "0.0.0.0:8090"
	opener := &fakeDesktopLauncher{}
	handler := newHandler(settings, http.DefaultClient, nil, nil, opener)
	result := postPortalOpen(t, handler, http.StatusOK)
	switch {
	case !result.Opened:
		t.Fatalf("the browser was not asked to open anything: %+v", result)
	case opener.opened != "http://127.0.0.1:8090":
		// Listening on every interface is not an address anyone visits.
		t.Fatalf("the browser was sent to %q", opener.opened)
	case result.URL != opener.opened:
		t.Fatalf("the answer said %q and the browser got %q", result.URL, opener.opened)
	}
}

// Behind a reverse proxy - the default deployment - the service listens
// locally and the entrance is somewhere else entirely.
func TestThePublicAddressWinsOverTheListenAddress(t *testing.T) {
	t.Parallel()
	settings := appconfig.Default()
	settings.Service.PublicURL = "https://ml.example.test/"
	opener := &fakeDesktopLauncher{}
	handler := newHandler(settings, http.DefaultClient, nil, nil, opener)
	if result := postPortalOpen(t, handler, http.StatusOK); opener.opened != "https://ml.example.test" {
		t.Fatalf("the browser was sent to %q, the answer said %q", opener.opened, result.URL)
	}
}

// Without a desktop side there is nothing to open a browser with. The button
// still answers, and with the address, so it can be copied.
func TestWithoutADesktopThePortalButtonAnswersWithTheAddress(t *testing.T) {
	t.Parallel()
	handler := newHandler(appconfig.Default(), http.DefaultClient, nil, nil, &fakeStudioLauncher{})
	result := postPortalOpen(t, handler, http.StatusOK)
	if result.Opened || result.URL == "" {
		t.Fatalf("result = %+v", result)
	}
}

// A browser that refuses to open is said out loud rather than reported as a
// successful opening.
func TestAFailedOpeningIsReported(t *testing.T) {
	t.Parallel()
	opener := &fakeDesktopLauncher{err: fmt.Errorf("no browser here")}
	handler := newHandler(appconfig.Default(), http.DefaultClient, nil, nil, opener)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/portal/open", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

type portalOpenResult struct {
	URL    string `json:"url"`
	Opened bool   `json:"opened"`
}

func postPortalOpen(t *testing.T, handler http.Handler, want int) portalOpenResult {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/portal/open", nil))
	if response.Code != want {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result portalOpenResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("body=%s error=%v", response.Body.String(), err)
	}
	return result
}

// fakeDesktopLauncher is the desktop side: it launches Studio and opens a
// browser, because both are things only it can do.
type fakeDesktopLauncher struct {
	fakeStudioLauncher
	opened string
	err    error
}

func (launcher *fakeDesktopLauncher) OpenURL(address string) error {
	launcher.opened = address
	return launcher.err
}
