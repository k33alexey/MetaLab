package webpage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// TestSecuredAnswersUnderThePolicy catches a header of the policy lost or
// loosened: the values are written out here, not read from SetPolicy.
func TestSecuredAnswersUnderThePolicy(t *testing.T) {
	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
	}
	handler := Secured(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "refused", http.StatusForbidden)
	}))
	answer := httptest.NewRecorder()
	handler.ServeHTTP(answer, httptest.NewRequest(http.MethodGet, "/api/anything", nil))
	for name, value := range want {
		if got := answer.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if answer.Code != http.StatusForbidden {
		t.Fatalf("status = %d: the policy must not change the answer", answer.Code)
	}
}

// TestPageIsNeverCached catches a page a browser would keep from an older
// build, and a missing page answered as an empty one.
func TestPageIsNeverCached(t *testing.T) {
	files := fstest.MapFS{"ui/index.html": {Data: []byte("<!doctype html><title>ML</title>")}}
	answer := httptest.NewRecorder()
	Page(files, "ML Test")(answer, httptest.NewRequest(http.MethodGet, "/", nil))
	if answer.Code != http.StatusOK || answer.Body.String() != "<!doctype html><title>ML</title>" ||
		answer.Header().Get("Content-Type") != "text/html; charset=utf-8" || answer.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("page: status=%d headers=%v body=%q", answer.Code, answer.Header(), answer.Body.String())
	}
	missing := httptest.NewRecorder()
	Page(fstest.MapFS{}, "ML Test")(missing, httptest.NewRequest(http.MethodGet, "/", nil))
	if missing.Code != http.StatusInternalServerError || !strings.Contains(missing.Body.String(), "ML Test UI unavailable") {
		t.Fatalf("missing page: status=%d body=%q", missing.Code, missing.Body.String())
	}
}

// TestJSONIsWholeOrRefused catches data cached by the browser, a status lost,
// and a value that does not encode answered with its status and a cut body.
func TestJSONIsWholeOrRefused(t *testing.T) {
	answer := httptest.NewRecorder()
	JSON(answer, http.StatusCreated, map[string]int{"count": 2})
	if answer.Code != http.StatusCreated || answer.Body.String() != "{\"count\":2}\n" ||
		answer.Header().Get("Content-Type") != "application/json; charset=utf-8" || answer.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("json: status=%d headers=%v body=%q", answer.Code, answer.Header(), answer.Body.String())
	}
	refused := httptest.NewRecorder()
	JSON(refused, http.StatusCreated, map[string]any{"value": make(chan int)})
	if refused.Code != http.StatusInternalServerError || strings.Contains(refused.Body.String(), "{") {
		t.Fatalf("unencodable value: status=%d body=%q", refused.Code, refused.Body.String())
	}
}
