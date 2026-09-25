package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
)

// TestSaveDataButtonEndToEndIntegration exercises the exact path a click on
// the Studio "Сохранить данные" button takes: the real HTTP handler, wired
// to a real SaveDataProvider, against a real PostgreSQL database and the
// real demo project - not just the underlying Go functions in isolation.
func TestSaveDataButtonEndToEndIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DROP TABLE IF EXISTS ml_core.database_state")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = '10000000-0000-4000-8000-000000000001'")
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS ml_core.database_state"); err != nil {
		t.Fatal(err)
	}

	root := copyDemoProjectForStudioTest(t)
	runStudioGit(t, root, "init", "-b", "main")
	runStudioGit(t, root, "config", "user.name", "MetaLab Test")
	runStudioGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runStudioGit(t, root, "add", "--all")
	runStudioGit(t, root, "commit", "-m", "Initial project")

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	workspace.SetSaveDataProvider(func(saveContext context.Context, projectRoot string, consent schemadiff.MigrationConsent) (publication.SavedState, schemadiff.MigrationRecord, error) {
		return publication.SaveData(saveContext, pool, publication.SaveDataRequest{
			Root: projectRoot, Mode: publication.ActivationPrimary, Confirmed: true, Consent: consent,
		})
	})
	// Studio in the desktop app gets saved names from the platform; here the
	// same provider is wired straight to the database under test.
	workspace.SetSavedNamesProvider(func(namesContext context.Context) map[string]string {
		snapshot, found, err := publication.CurrentDatabaseState(namesContext, pool)
		if err != nil || !found {
			return map[string]string{}
		}
		catalog, err := snapshot.Catalog()
		if err != nil {
			return map[string]string{}
		}
		return catalog.PhysicalNames()
	})
	handler := NewHandler(workspace)

	// Same click, uncommitted change first: must be rejected with a clear message.
	if err := os.WriteFile(filepath.Join(root, "README-dirty.md"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirtyResponse := httptest.NewRecorder()
	dirtyRequest := httptest.NewRequest(http.MethodPost, "/api/save-data", bytes.NewReader([]byte("{}")))
	dirtyRequest.Header.Set("Content-Type", "application/json")
	dirtyRequest.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(dirtyResponse, dirtyRequest)
	if dirtyResponse.Code != http.StatusConflict {
		t.Fatalf("save-data on a dirty Primary project status=%d body=%s", dirtyResponse.Code, dirtyResponse.Body.String())
	}
	if err := os.Remove(filepath.Join(root, "README-dirty.md")); err != nil {
		t.Fatal(err)
	}

	// The real click: committed project, through the real HTTP handler.
	saveResponse := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPost, "/api/save-data", bytes.NewReader([]byte("{}")))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save-data status=%d body=%s", saveResponse.Code, saveResponse.Body.String())
	}
	var result struct {
		GitCommit       string `json:"gitCommit"`
		SchemaSHA256    string `json:"schemaSha256"`
		MigrationStatus string `json:"migrationStatus"`
	}
	if err := json.Unmarshal(saveResponse.Body.Bytes(), &result); err != nil || result.GitCommit == "" || result.MigrationStatus != "succeeded" {
		t.Fatalf("save-data response body=%s err=%v", saveResponse.Body.String(), err)
	}

	// The data really landed where ML App reads it from.
	snapshot, found, err := publication.CurrentDatabaseState(ctx, pool)
	if err != nil || !found || len(snapshot.Modules) != 2 || len(snapshot.Documents) != 2 {
		t.Fatalf("saved database state: found=%v snapshot=%+v err=%v", found, snapshot, err)
	}

	// Removing an attribute drops a column, which the same click must refuse
	// until it is confirmed - and the refusal has to say WHAT is being dropped,
	// in the names the developer wrote, not as t_1000… · c_1000….
	goods := filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml")
	source, err := os.ReadFile(goods)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := source[:bytes.Index(source, []byte("attributes:"))]
	if err := os.WriteFile(goods, trimmed, 0o644); err != nil {
		t.Fatal(err)
	}
	runStudioGit(t, root, "add", "--all")
	runStudioGit(t, root, "commit", "-m", "Drop the SKU attribute")

	deniedResponse := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodPost, "/api/save-data", bytes.NewReader([]byte("{}")))
	deniedRequest.Header.Set("Content-Type", "application/json")
	deniedRequest.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(deniedResponse, deniedRequest)
	if deniedResponse.Code != http.StatusConflict {
		t.Fatalf("dropping a column without consent status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
	var denied struct {
		ObjectLoss   bool `json:"objectLoss"`
		ValueRewrite bool `json:"valueRewrite"`
		Changes      []struct {
			Kind, Impact, Title string
		} `json:"changes"`
	}
	if err := json.Unmarshal(deniedResponse.Body.Bytes(), &denied); err != nil {
		t.Fatalf("confirmation details body=%s err=%v", deniedResponse.Body.String(), err)
	}
	if !denied.ObjectLoss || denied.ValueRewrite || len(denied.Changes) == 0 {
		t.Fatalf("confirmation details = %+v", denied)
	}
	named := false
	for _, change := range denied.Changes {
		if change.Impact == "object_loss" && strings.Contains(change.Title, "Справочник.Товары · Артикул") {
			named = true
		}
	}
	if !named {
		t.Fatalf("the dropped attribute is not named in the configuration's own terms: %+v", denied.Changes)
	}

	confirmedResponse := httptest.NewRecorder()
	confirmedRequest := httptest.NewRequest(http.MethodPost, "/api/save-data", bytes.NewReader([]byte(`{"consent":{"objectLoss":true}}`)))
	confirmedRequest.Header.Set("Content-Type", "application/json")
	confirmedRequest.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(confirmedResponse, confirmedRequest)
	if confirmedResponse.Code != http.StatusOK {
		t.Fatalf("confirmed save-data status=%d body=%s", confirmedResponse.Code, confirmedResponse.Body.String())
	}
}

func copyDemoProjectForStudioTest(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "project")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, relErr := filepath.Rel(source, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		input, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer input.Close()
		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o755); mkdirErr != nil {
			return mkdirErr
		}
		output, createErr := os.Create(target)
		if createErr != nil {
			return createErr
		}
		defer output.Close()
		_, copyErr := io.Copy(output, input)
		return copyErr
	}); err != nil {
		t.Fatal(err)
	}
	return destination
}

func TestSaveDataDialogUI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for save-data dialog tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/save-data-dialog.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("save-data dialog tests: %v\n%s", err, output)
	}
}
