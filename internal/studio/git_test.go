package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestGitAPIStatusDiffCommitAndBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	root := createProject(t)
	runStudioGit(t, root, "init", "-b", "main")
	runStudioGit(t, root, "config", "user.name", "MetaLab Test")
	runStudioGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runStudioGit(t, root, "add", "--all")
	runStudioGit(t, root, "commit", "-m", "Initial project")
	modulePath, _ := project.ModulePath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Тест();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)

	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/git/status", nil))
	var status struct {
		Branch  string `json:"branch"`
		Entries []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if statusResponse.Code != http.StatusOK || json.Unmarshal(statusResponse.Body.Bytes(), &status) != nil || status.Branch != "main" || len(status.Entries) != 1 {
		t.Fatalf("status=%d body=%s parsed=%+v", statusResponse.Code, statusResponse.Body.String(), status)
	}

	diffResponse := httptest.NewRecorder()
	handler.ServeHTTP(diffResponse, httptest.NewRequest(http.MethodGet, "/api/git/diff?path="+modulePath, nil))
	if diffResponse.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", diffResponse.Code, diffResponse.Body.String())
	}

	commitPayload := []byte(`{"message":"Add module"}`)
	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/git/commit", bytes.NewReader(commitPayload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("commit without CSRF status = %d", denied.Code)
	}
	committed := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/git/commit", bytes.NewReader(commitPayload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(committed, request)
	if committed.Code != http.StatusOK || !strings.Contains(committed.Body.String(), "Add module") {
		t.Fatalf("commit status=%d body=%s", committed.Code, committed.Body.String())
	}

	switched := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/git/branches/switch", strings.NewReader(`{"name":"feature/forms","create":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(switched, request)
	if switched.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", switched.Code, switched.Body.String())
	}
	branches := httptest.NewRecorder()
	handler.ServeHTTP(branches, httptest.NewRequest(http.MethodGet, "/api/git/branches", nil))
	if branches.Code != http.StatusOK || !strings.Contains(branches.Body.String(), `"current":"feature/forms"`) {
		t.Fatalf("branches status=%d body=%s", branches.Code, branches.Body.String())
	}
}

func runStudioGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}
