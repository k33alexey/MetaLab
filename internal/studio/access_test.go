package studio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStudioRequiresDesktopBootstrapAndCurrentLease(t *testing.T) {
	active := true
	operations, checks := 0, 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { operations++; w.WriteHeader(204) })
	handler, launch, err := protectStudioHandler(next, "127.0.0.1:23456", func(context.Context) error {
		checks++
		if !active {
			return errors.New("revoked")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:23456"+path, nil)
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
	for _, path := range []string{"/", "/api/project", "/ui/index.html", "/api/file?path=ml.yaml", "/api/role?path=metadata/roles/test.yaml", "/ui/role-editor.js"} {
		if w := request("GET", path, nil, ""); w.Code != 401 {
			t.Fatalf("anonymous %s: %d", path, w.Code)
		}
	}
	if checks != 0 || operations != 0 {
		t.Fatal("anonymous request reached protected code")
	}
	if w := request("GET", "/__ml_open/invalid", nil, ""); w.Code != 401 {
		t.Fatalf("invalid bootstrap: %d", w.Code)
	}
	opened := request("GET", launch, nil, "")
	if opened.Code != 303 || opened.Header().Get("Location") != "/" {
		t.Fatalf("bootstrap: %d %s", opened.Code, opened.Body.String())
	}
	if strings.Contains(opened.Body.String(), launch) || opened.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("bootstrap URL leaked")
	}
	cookies := opened.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || strings.Contains(launch, cookie.Value) {
		t.Fatal("unsafe cookie")
	}
	if w := request("GET", launch, nil, ""); w.Code != 401 {
		t.Fatalf("replayed bootstrap: %d", w.Code)
	}
	if w := request("GET", "/api/project", cookie, ""); w.Code != 204 {
		t.Fatalf("authenticated read: %d", w.Code)
	}
	if w := request("POST", "/api/file", cookie, "https://foreign.example"); w.Code != 403 {
		t.Fatalf("CSRF: %d", w.Code)
	}
	wrong := *cookie
	wrong.Value = "wrong-token"
	if w := request("GET", "/api/project", &wrong, ""); w.Code != 401 {
		t.Fatalf("wrong token: %d", w.Code)
	}
	host := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://evil.example/api/project", nil)
	r.AddCookie(cookie)
	handler.ServeHTTP(host, r)
	if host.Code != 403 {
		t.Fatalf("foreign host: %d", host.Code)
	}
	active = false
	if w := request("POST", "/api/file", cookie, ""); w.Code != 403 {
		t.Fatalf("revoked write: %d", w.Code)
	}
	if operations != 1 {
		t.Fatalf("unauthorized operations reached workspace: %d", operations)
	}
}

func TestStudioBootstrapFailsClosed(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized workspace operation") })
	if _, _, err := protectStudioHandler(next, "127.0.0.1:23456", nil); err == nil {
		t.Fatal("accepted missing authorization")
	}
	handler, launch, err := protectStudioHandler(next, "127.0.0.1:23456", func(context.Context) error { return errors.New("unavailable") })
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:23456"+launch, nil))
	if w.Code != 403 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("failed lease issued cookie: %d", w.Code)
	}
}
