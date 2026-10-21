package webpage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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

// What a route did is answered one way on every server: the result as JSON
// with the route's status, No Content with no body at all, a failure by the
// route's refusal and nothing else; a plain refusal says its own words, or
// the failure's where it has none. Defects caught: a body written after 204,
// which a client reads as a broken answer; a result written after the
// refusal; a refusal answering with another status or losing the failure's
// words.
func TestAnswerSaysTheResultOrTheRefusal(t *testing.T) {
	failure := errors.New("database is busy")
	for _, check := range []struct {
		name   string
		status int
		result any
		err    error
		refuse func(http.ResponseWriter, error)
		code   int
		body   string
	}{
		{"a result", http.StatusCreated, map[string]int{"count": 2}, nil, Refusal(http.StatusConflict, ""), http.StatusCreated, "{\"count\":2}\n"},
		{"no content", http.StatusNoContent, map[string]int{"count": 2}, nil, Refusal(http.StatusConflict, ""), http.StatusNoContent, ""},
		{"a failure in its own words", http.StatusOK, "ignored", failure, Refusal(http.StatusConflict, ""), http.StatusConflict, "database is busy\n"},
		{"a failure in the route's words", http.StatusNoContent, nil, failure, Refusal(http.StatusBadRequest, "Unable to save"), http.StatusBadRequest, "Unable to save\n"},
	} {
		answer := httptest.NewRecorder()
		Answer(answer, check.status, check.result, check.err, check.refuse)
		if answer.Code != check.code || answer.Body.String() != check.body {
			t.Errorf("%s: status=%d body=%q, want %d %q", check.name, answer.Code, answer.Body.String(), check.code, check.body)
		}
	}
}

// TestAPolicyLoosenedIsMissing catches the check of a server's answers
// taking a header of the policy that is there for one that is right: a
// policy that lets scripts in from anywhere is as missing as none.
func TestAPolicyLoosenedIsMissing(t *testing.T) {
	header := http.Header{}
	SetPolicy(header)
	if missing := PolicyMissing(header); len(missing) != 0 {
		t.Fatalf("the policy itself lacks %v", missing)
	}
	header.Set("Content-Security-Policy", "default-src *")
	header.Del("X-Frame-Options")
	missing := PolicyMissing(header)
	slices.Sort(missing)
	if !slices.Equal(missing, []string{"Content-Security-Policy", "X-Frame-Options"}) {
		t.Fatalf("a loosened and a lost header are missing as %v", missing)
	}
}
