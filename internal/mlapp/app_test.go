package mlapp

import (
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestBootstrapIsValidAndBoundToSession(t *testing.T) {
	databaseID, userID := uuid.MustNew(), uuid.MustNew()
	bootstrap := NewBootstrap(databaseID, systemdb.DatabaseSession{DatabaseName: "Продажи", UserID: userID, Login: "Иванов"})
	if err := bootstrap.Validate(); err != nil {
		t.Fatal(err)
	}
	if bootstrap.Database.ID != databaseID || bootstrap.User.ID != userID || bootstrap.Database.Name != "Продажи" || bootstrap.User.Login != "Иванов" {
		t.Fatalf("bootstrap=%+v", bootstrap)
	}
	if bootstrap.Locale != "ru" || len(bootstrap.Form.Commands) == 0 || len(bootstrap.Form.Items) == 0 {
		t.Fatalf("incomplete bootstrap=%+v", bootstrap)
	}
}

func TestBootstrapRejectsInvalidComponentContract(t *testing.T) {
	bootstrap := NewBootstrap(uuid.MustNew(), systemdb.DatabaseSession{DatabaseName: "База", UserID: uuid.MustNew(), Login: "user"})
	bootstrap.Form.Items = []Element{{ID: "unsafe", Kind: "html"}}
	if err := bootstrap.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("invalid component error=%v", err)
	}
	bootstrap = NewBootstrap(uuid.MustNew(), systemdb.DatabaseSession{DatabaseName: "База", UserID: uuid.MustNew(), Login: "user"})
	bootstrap.Form.Items = []Element{{ID: "button", Kind: "button", Command: "missing"}}
	if err := bootstrap.Validate(); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("invalid button error=%v", err)
	}
	bootstrap = NewBootstrap(uuid.MustNew(), systemdb.DatabaseSession{DatabaseName: "База", UserID: uuid.MustNew(), Login: "user"})
	bootstrap.Form.Items = []Element{{ID: "same", Kind: "field"}, {ID: "same", Kind: "field"}}
	if err := bootstrap.Validate(); err == nil {
		t.Fatal("duplicate component IDs were accepted")
	}
	bootstrap = NewBootstrap(uuid.MustNew(), systemdb.DatabaseSession{DatabaseName: "База", UserID: uuid.MustNew(), Login: "user"})
	bootstrap.Navigation = append(bootstrap.Navigation, NavigationItem{ID: "object", Title: "Объект"})
	if err := bootstrap.Validate(); err == nil {
		t.Fatal("navigation item without a metadata target was accepted")
	}
}

func TestEmbeddedApplicationAssetsAreSafeAndCacheable(t *testing.T) {
	page := httptest.NewRecorder()
	ServePage(page)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<ml-app-shell") || !strings.Contains(page.Body.String(), `viewport-fit=cover`) || !strings.Contains(page.Body.String(), `rel="manifest"`) || page.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("page status=%d headers=%v body=%s", page.Code, page.Header(), page.Body.String())
	}
	for _, asset := range []struct {
		name, contentType, marker string
	}{
		{"app.js", "text/javascript", `customElements.define("ml-form"`},
		{"app.css", "text/css", "@media (pointer: coarse)"},
		{"manifest.webmanifest", "application/manifest+json", `"display": "standalone"`},
		{"icon.svg", "image/svg+xml", "<svg"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/assets/ml-app/"+asset.name, nil)
		ServeAsset(response, request, asset.name)
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), asset.contentType) || !strings.Contains(response.Body.String(), asset.marker) || response.Header().Get("ETag") == "" {
			t.Fatalf("asset %s status=%d headers=%v", asset.name, response.Code, response.Header())
		}
		if asset.name == "app.js" && strings.Contains(response.Body.String(), "innerHTML") {
			t.Fatal("ML App renderer must not inject HTML")
		}
		cached := httptest.NewRequest(http.MethodGet, "/assets/ml-app/"+asset.name, nil)
		cached.Header.Set("If-None-Match", response.Header().Get("ETag"))
		cachedResponse := httptest.NewRecorder()
		ServeAsset(cachedResponse, cached, asset.name)
		if cachedResponse.Code != http.StatusNotModified || cachedResponse.Body.Len() != 0 {
			t.Fatalf("conditional asset %s status=%d", asset.name, cachedResponse.Code)
		}
	}
	manifestResponse := httptest.NewRecorder()
	ServeAsset(manifestResponse, httptest.NewRequest(http.MethodGet, "/assets/ml-app/manifest.webmanifest", nil), "manifest.webmanifest")
	var manifest struct {
		StartURL string `json:"start_url"`
		Scope    string `json:"scope"`
		Display  string `json:"display"`
		Icons    []struct {
			Source string `json:"src"`
			Sizes  string `json:"sizes"`
		} `json:"icons"`
	}
	if err := json.NewDecoder(manifestResponse.Body).Decode(&manifest); err != nil || manifest.StartURL != "/" || manifest.Scope != "/" || manifest.Display != "standalone" || len(manifest.Icons) != 2 || manifest.Icons[0].Sizes != "192x192" || manifest.Icons[1].Sizes != "512x512" {
		t.Fatalf("manifest=%+v error=%v", manifest, err)
	}
	for _, icon := range []struct {
		name string
		size int
	}{{"icon-192.png", 192}, {"icon-512.png", 512}} {
		response := httptest.NewRecorder()
		ServeAsset(response, httptest.NewRequest(http.MethodGet, "/assets/ml-app/"+icon.name, nil), icon.name)
		configuration, err := png.DecodeConfig(response.Body)
		if err != nil || response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || configuration.Width != icon.size || configuration.Height != icon.size {
			t.Fatalf("icon %s status=%d size=%dx%d error=%v", icon.name, response.Code, configuration.Width, configuration.Height, err)
		}
	}
	for _, asset := range []struct {
		name    string
		markers []string
	}{
		{"app.css", []string{"@media (max-width: 1024px)", "@media (max-width: 720px)", "env(safe-area-inset-top)", "@media (display-mode: standalone)"}},
		{"app.js", []string{"nav-toggle", "toggleNavigation()", "handleNavigationKey(event)", "this._model.list"}},
	} {
		response := httptest.NewRecorder()
		ServeAsset(response, httptest.NewRequest(http.MethodGet, "/assets/ml-app/"+asset.name, nil), asset.name)
		for _, marker := range asset.markers {
			if !strings.Contains(response.Body.String(), marker) {
				t.Fatalf("asset %s has no adaptive marker %q", asset.name, marker)
			}
		}
	}
	missing := httptest.NewRecorder()
	ServeAsset(missing, httptest.NewRequest(http.MethodGet, "/", nil), "../index.html")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status=%d", missing.Code)
	}
}
