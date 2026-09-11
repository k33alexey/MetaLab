package mlapp

import (
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
}

func TestEmbeddedApplicationAssetsAreSafeAndCacheable(t *testing.T) {
	page := httptest.NewRecorder()
	ServePage(page)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<ml-app-shell") || !strings.Contains(page.Body.String(), `name="viewport"`) || page.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("page status=%d headers=%v body=%s", page.Code, page.Header(), page.Body.String())
	}
	for _, asset := range []struct {
		name, contentType, marker string
	}{
		{"app.js", "text/javascript", `customElements.define("ml-form"`},
		{"app.css", "text/css", "--ml-color-header"},
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
	missing := httptest.NewRecorder()
	ServeAsset(missing, httptest.NewRequest(http.MethodGet, "/", nil), "../index.html")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status=%d", missing.Code)
	}
}
