package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The settings of a list are the user's and outlive Studio (help,
// form_customizegriddlg): a workspace opened anew over the same directory
// reads what the one before it saved, and each configuration has its own.
// Defect caught: the settings lived only as long as the process, or one
// configuration's list took another's setting.
func TestListSettingsOutliveTheWorkspaceAndStayWithTheirConfiguration(t *testing.T) {
	directory := t.TempDir()
	root := createProject(t)
	first, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first.KeepUserSettingsIn(directory)
	restrictions := map[string]any{"opening": "beginning", "sort": map[string]any{"column": "operation", "descending": true}}
	if err := first.SaveListSettings("all-restrictions", restrictions); err != nil {
		t.Fatal(err)
	}
	if err := first.SaveListSettings("all-templates", map[string]any{"opening": "auto"}); err != nil {
		t.Fatal(err)
	}
	if err := first.SaveListSettings("all-templates", map[string]any{"opening": "end"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, first.configurationID.String(), listSettingsFile)); err != nil {
		t.Fatalf("the settings are not in the folder of the configuration: %v", err)
	}

	again, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	again.KeepUserSettingsIn(directory)
	lists, err := again.ListSettings()
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(lists)
	if string(got) != `{"all-restrictions":{"opening":"beginning","sort":{"column":"operation","descending":true}},"all-templates":{"opening":"end"}}` {
		t.Fatalf("settings read anew = %s", got)
	}

	other, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	other.KeepUserSettingsIn(directory)
	if lists, err := other.ListSettings(); err != nil || len(lists) != 0 {
		t.Fatalf("another configuration reads %v, %v; want nothing", lists, err)
	}
}

// A list's key is a key of the file, never a path, and a setting is an
// object. Defect caught: a key such as "../x" or an array written as a
// setting.
func TestListSettingsRefuseAKeyOfAnotherFormAndASettingNotAnObject(t *testing.T) {
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	workspace.KeepUserSettingsIn(directory)
	handler := NewHandler(workspace)
	put := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPut, "/api/settings/list", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-ML-CSRF", "1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for _, body := range []string{
		`{"key":"../x","settings":{}}`, `{"key":"","settings":{}}`, `{"key":"All","settings":{}}`,
		`{"key":"` + strings.Repeat("a", 65) + `","settings":{}}`,
		`{"key":"list","settings":null}`, `{"key":"list"}`, `{"key":"list","settings":[1]}`, `{"key":"list","settings":{},"extra":1}`,
	} {
		if response := put(body); response.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d %q, want 400", body, response.Code, response.Body.String())
		}
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("a refused setting left %d entries", len(entries))
	}
	if response := put(`{"key":"all-restrictions","settings":{"opening":"end"}}`); response.Code != http.StatusOK {
		t.Fatalf("a setting answered %d %q", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/settings/lists", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"lists":{"all-restrictions":{"opening":"end"}}}` {
		t.Fatalf("lists answered %d %q", response.Code, response.Body.String())
	}
	// A request without the mark of Studio's page writes nothing.
	unmarked := httptest.NewRequest(http.MethodPut, "/api/settings/list", strings.NewReader(`{"key":"x","settings":{}}`))
	unmarked.Header.Set("Content-Type", "application/json")
	refused := httptest.NewRecorder()
	handler.ServeHTTP(refused, unmarked)
	if refused.Code != http.StatusForbidden {
		t.Fatalf("an unmarked request answered %d", refused.Code)
	}
}

// The file is the user's and may be broken by hand: reading it says so, and
// the next setting the user makes takes its place rather than leaving
// Studio unable to keep any. Defect caught: a broken file refused every
// save for good, or read as no settings without a word.
func TestABrokenFileOfListSettingsIsSaidAndThenReplaced(t *testing.T) {
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	workspace.KeepUserSettingsIn(directory)
	folder := filepath.Join(directory, workspace.configurationID.String())
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, listSettingsFile), []byte("- not\n- a map\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ListSettings(); err == nil {
		t.Fatal("a broken file read as settings")
	}
	if err := workspace.SaveListSettings("all-templates", map[string]any{"opening": "auto"}); err != nil {
		t.Fatal(err)
	}
	lists, err := workspace.ListSettings()
	if err != nil || len(lists) != 1 {
		t.Fatalf("after a save the file reads %v, %v", lists, err)
	}
}

// A workspace not told where to keep the settings keeps them while it is
// open. Defect caught: such a workspace - the tests' - lost a setting at
// once or wrote into the user's real profile.
func TestListSettingsWithoutADirectoryLiveWithTheWorkspace(t *testing.T) {
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.SaveListSettings("all-restrictions", map[string]any{"opening": "end"}); err != nil {
		t.Fatal(err)
	}
	lists, err := workspace.ListSettings()
	if err != nil || len(lists) != 1 {
		t.Fatalf("lists = %v, %v", lists, err)
	}
}
