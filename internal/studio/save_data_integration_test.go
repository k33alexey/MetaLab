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
	workspace.SetSaveDataProvider(func(saveContext context.Context, projectRoot string, allowDestructive bool) (publication.SavedState, schemadiff.MigrationRecord, error) {
		return publication.SaveData(saveContext, pool, publication.SaveDataRequest{
			Root: projectRoot, Mode: publication.ActivationPrimary, Confirmed: true, AllowDestructive: allowDestructive,
		})
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
