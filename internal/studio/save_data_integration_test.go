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

	"github.com/k33alexey/MetaLab/internal/integrationtest"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
)

// TestSaveDataButtonEndToEndIntegration exercises the exact path a click on
// the Studio "Сохранить данные" button takes: the real HTTP handler, wired
// to a real SaveDataProvider, against a real PostgreSQL database and the
// real demo project - not just the underlying Go functions in isolation.
func TestSaveDataButtonEndToEndIntegration(t *testing.T) {
	ctx, pool, root, handler := saveDataStudio(t)

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

// saveDataStudio is Studio on a copy of the demo project, committed to Git,
// with «Сохранить данные» wired to the database under test, emptied before
// and after. It skips without PostgreSQL or Git.
func saveDataStudio(t *testing.T) (context.Context, *pgxpool.Pool, string, http.Handler) {
	t.Helper()
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS ml_core.database_state"); err != nil {
		t.Fatal(err)
	}

	root := copyDemoProjectForStudioTest(t)
	demo, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	integrationtest.ForgetSavedProject(t, pool, demo.Project.ID, demo.ObjectIDs())
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

	return ctx, pool, root, handler
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

// TestSaveDataRefusesAModuleThatDoesNotCompileIntegration holds the item of
// block 2в found by the pass 2.444 (case 2в-12, step 1): a syntax error in the
// module of a common form, shown by its own diagnostics, did not stop
// «Сохранить данные» - «Данные сохранены» -, and neither did an error in the
// module of an object; ML App then refused to compile the base it opened.
// The click through the real handler now refuses with every error, naming
// the module, the line and the column, before the base is touched (the owner,
// 22.10.2026: a refusal without asking whether to go on), and saves once the
// modules compile. Defects caught: the modules only read and not compiled
// (2.444); a refusal naming the first error alone, or a file and not the
// module; a refusal after the migration, leaving a base that does not open.
func TestSaveDataRefusesAModuleThatDoesNotCompileIntegration(t *testing.T) {
	ctx, pool, root, handler := saveDataStudio(t)
	click := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/save-data", bytes.NewReader([]byte("{}")))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-ML-CSRF", "1")
		handler.ServeHTTP(response, request)
		return response
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) {
		runStudioGit(t, root, "add", "--all")
		runStudioGit(t, root, "commit", "-m", message)
	}
	write("metadata/common-forms/Настройки/"+project.FormMetadataFile,
		"format: 1\nid: 5e7a0000-0000-4000-8000-000000000001\nname: Настройки\ntitle: {ru: Настройки}\nkind: common\n")
	write("metadata/common-forms/Настройки/"+project.FormModuleFile, "Процедура Обновить()\n\tА = ;\nКонецПроцедуры\n")
	objectModule := "metadata/documents/ПоступлениеТоваров/МодульОбъекта.bsl"
	original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(objectModule)))
	if err != nil {
		t.Fatal(err)
	}
	write(objectModule, string(original)+"\nПроцедура Проверить()\n\tБ = ;\nКонецПроцедуры\n")
	commit("Two modules that do not parse")
	refuse := func(wants ...string) {
		t.Helper()
		refused := click()
		body := refused.Body.String()
		if refused.Code != http.StatusUnprocessableEntity || !strings.HasPrefix(body, "Модули не компилируются — данные не сохранены:") {
			t.Fatalf("a module that does not compile: status=%d body=%s", refused.Code, body)
		}
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("the refusal does not say %q:\n%s", want, body)
			}
		}
		if _, found, err := publication.CurrentDatabaseState(ctx, pool); err != nil || found {
			t.Fatalf("the refused save left a state in the base: found=%v err=%v", found, err)
		}
	}
	// The compiler reads every module before it resolves any name: the
	// syntax errors of both modules come first, together.
	refuse("МодульФормыОбщейФормы.Настройки (строка 2, колонка 6): unexpected ;, expected expression [BSL2001]",
		"МодульОбъектаДокумента.ПоступлениеТоваров (строка 15, колонка 6): unexpected ;, expected expression [BSL2001]")
	write("metadata/common-forms/Настройки/"+project.FormModuleFile, "Процедура Обновить()\n\tА = 1;\nКонецПроцедуры\n")
	write(objectModule, string(original)+"\nПроцедура Проверить()\n\tНеизвестная();\nКонецПроцедуры\n")
	commit("A module calling what is not there")
	refuse("МодульОбъектаДокумента.ПоступлениеТоваров (строка 15, колонка 2): unknown routine Неизвестная [BSL3033]")

	write(objectModule, string(original))
	commit("The modules compile")
	if saved := click(); saved.Code != http.StatusOK {
		t.Fatalf("the modules compile, the save is refused: status=%d body=%s", saved.Code, saved.Body.String())
	}
}
