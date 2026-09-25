// Package studio implements the local ML Studio workspace and HTTP surface.
package studio

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/debugtarget"
	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

//go:embed ui/*
var assets embed.FS

// Workspace is one validated project opened by a Studio process.
type Workspace struct {
	root          string
	mu            sync.Mutex
	bslIndex      *BSLSymbolIndex
	bslNavigation *bslNavigationIndex
	bslHelp       *bslHelpIndex
	projectSearch *projectSearchIndex
	querySchema   *QueryDesignerSchema
	debugSession  *vm.DebugSession
	debugTargets  *debugtarget.Registry
	debugTarget   *debugtarget.Lease
	debugDatabase uuid.UUID
	saveData      SaveDataProvider
	savedNames    SavedNamesProvider
}

// Snapshot is the read-only project model rendered by the Studio shell.
type Snapshot struct {
	ProjectPath string          `json:"projectPath"`
	Manifest    project.Project `json:"manifest"`
	Tree        Node            `json:"tree"`
}

// Node represents a stable item in the metadata tree.
type Node struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Path       string     `json:"path,omitempty"`
	Fragment   string     `json:"fragment,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	Children   []Node     `json:"children,omitempty"`
	Line       int        `json:"line,omitempty"`
}

// Property is one ordered value displayed in the properties panel.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// commonKindOrder and topLevelKindOrder fix the configuration tree's shape:
// "Общие" collects the configuration-wide objects, every other kind stands
// at the top level beside it. Order is the one the specification fixes, not
// the physical directory order, and kinds not implemented yet simply leave
// a gap rather than shifting their neighbours.
var (
	commonKindOrder = []string{
		"subsystems", "common-modules", "session-parameters", "roles",
		"common-attributes", "exchange-plans", "filter-criteria",
		"event-subscriptions", "scheduled-jobs", "bots",
		"functional-options", "functional-options-parameters",
		"defined-types", "settings-storages", "common-commands",
		"command-groups", "common-forms", "common-templates",
		"common-pictures", "xdto-packages", "web-services", "http-services",
		"ws-references", "websocket-clients", "integration-services",
		"style-items", "styles", "languages",
	}
	topLevelKindOrder = []string{
		"constants", "catalogs", "documents", "document-journals",
		"enumerations", "reports", "data-processors",
		"charts-of-characteristic-types", "charts-of-accounts",
		"charts-of-calculation-types", "information-registers",
		"accumulation-registers", "accounting-registers",
		"calculation-registers", "business-processes", "tasks",
		"external-data-sources",
	}
	// Нумераторы и последовательности в верхнем уровне не стоят: обе ветви
	// живут в корне «Документов», рядом с самими документами - там же, где
	// создаются их виды.
	documentNestedKindOrder = []string{"document-numerators", "sequences"}
)

var metadataTitles = map[string]string{
	"subsystems":                     "Подсистемы",
	"common-modules":                 "Общие модули",
	"exchange-plans":                 "Планы обмена",
	"filter-criteria":                "Критерии отбора",
	"bots":                           "Боты",
	"functional-options":             "Функциональные опции",
	"functional-options-parameters":  "Параметры функциональных опций",
	"command-groups":                 "Группы команд",
	"xdto-packages":                  "XDTO-пакеты",
	"web-services":                   "Web-сервисы",
	"ws-references":                  "WS-ссылки",
	"websocket-clients":              "WebSocket-клиенты",
	"integration-services":           "Сервисы интеграции",
	"style-items":                    "Элементы стиля",
	"document-numerators":            "Нумераторы",
	"sequences":                      "Последовательности",
	"charts-of-calculation-types":    "Планы видов расчёта",
	"calculation-registers":          "Регистры расчёта",
	"business-processes":             "Бизнес-процессы",
	"tasks":                          "Задачи",
	"external-data-sources":          "Внешние источники данных",
	"session-parameters":             "Параметры сеанса",
	"roles":                          "Роли",
	"common-attributes":              "Общие реквизиты",
	"event-subscriptions":            "Подписки на события",
	"scheduled-jobs":                 "Регламентные задания",
	"defined-types":                  "Определяемые типы",
	"common-commands":                "Общие команды",
	"common-forms":                   "Общие формы",
	"common-templates":               "Общие макеты",
	"common-pictures":                "Общие картинки",
	"http-services":                  "HTTP-сервисы",
	"styles":                         "Стили",
	"languages":                      "Языки",
	"constants":                      "Константы",
	"settings-storages":              "Хранилища настроек",
	"catalogs":                       "Справочники",
	"documents":                      "Документы",
	"document-journals":              "Журналы документов",
	"enumerations":                   "Перечисления",
	"reports":                        "Отчёты",
	"data-processors":                "Обработки",
	"charts-of-characteristic-types": "Планы видов характеристик",
	"charts-of-accounts":             "Планы счетов",
	"information-registers":          "Регистры сведений",
	"accumulation-registers":         "Регистры накопления",
	"accounting-registers":           "Регистры бухгалтерии",
	"folders":                        "Каталоги Studio",
}

// Open validates and opens an ML Project without mutating its files.
func Open(root string) (*Workspace, error) {
	return openWorkspace(root, uuid.UUID{}, debugtarget.NewRegistry())
}

// OpenWithDebugTargets opens a project against a shared live-runtime registry.
func OpenWithDebugTargets(root string, targets *debugtarget.Registry) (*Workspace, error) {
	return openWorkspace(root, uuid.UUID{}, targets)
}

// OpenForDatabase opens the Studio workspace for one concrete ML database.
func OpenForDatabase(root string, databaseID uuid.UUID) (*Workspace, error) {
	return OpenForDatabaseWithDebugTargets(root, databaseID, debugtarget.NewRegistry())
}

// OpenForDatabaseWithDebugTargets opens a database-bound Studio workspace
// against a shared live-runtime registry.
func OpenForDatabaseWithDebugTargets(root string, databaseID uuid.UUID, targets *debugtarget.Registry) (*Workspace, error) {
	if databaseID.IsZero() {
		return nil, fmt.Errorf("debug database identifier is required")
	}
	return openWorkspace(root, databaseID, targets)
}

func openWorkspace(root string, databaseID uuid.UUID, targets *debugtarget.Registry) (*Workspace, error) {
	if targets == nil {
		return nil, fmt.Errorf("debug target registry is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve ML Project path: %w", err)
	}
	if _, err := project.ValidateLayout(absolute); err != nil {
		return nil, err
	}
	return &Workspace{root: filepath.Clean(absolute), debugTargets: targets, debugDatabase: databaseID}, nil
}

// Snapshot scans current project sources, including edits made outside Studio.
func (workspace *Workspace) Snapshot() (Snapshot, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	workspace.invalidateStudioIndexesLocked()
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return Snapshot{}, err
	}
	root := Node{
		ID: "project", Kind: "project", Title: manifest.Name, Path: project.ManifestFile,
		Properties: []Property{
			{Name: "Имя", Value: manifest.Name}, {Name: "Заголовок", Value: manifest.Title},
			{Name: "UUID", Value: manifest.ID.String()}, {Name: "Основной язык", Value: manifest.DefaultLanguage},
			{Name: "Формат", Value: fmt.Sprint(manifest.Format)},
		},
	}
	root.Children = append(root.Children, workspace.sessionModuleNodeLocked())
	branches, err := workspace.metadataTree(manifest.DefaultLanguage, manifest.Languages)
	if err != nil {
		return Snapshot{}, err
	}
	root.Children = append(root.Children, branches...)
	return Snapshot{ProjectPath: workspace.root, Manifest: manifest, Tree: root}, nil
}

// SaveDataProvider runs "Сохранить данные" against this Studio's database.
// The project directory is the artifact: nothing is built into a package on
// the way from what a developer edits to what the database holds.
type SaveDataProvider func(ctx context.Context, root string, consent schemadiff.MigrationConsent) (publication.SavedState, schemadiff.MigrationRecord, error)

// SavedNamesProvider reads the names of what is currently SAVED in the
// database. The project on disk cannot name an object that is being dropped -
// the developer just deleted it - and "будет удалено: c_1000…" is precisely the
// line that must be readable.
type SavedNamesProvider func(ctx context.Context) map[string]string

// SetSavedNamesProvider connects the migration dialog to the saved metadata.
func (workspace *Workspace) SetSavedNamesProvider(provider SavedNamesProvider) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	workspace.savedNames = provider
}

// SetSaveDataProvider connects this database-bound Studio to its database.
func (workspace *Workspace) SetSaveDataProvider(provider SaveDataProvider) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	workspace.saveData = provider
}

// SaveData validates open sources are saved, then migrates PostgreSQL and
// refreshes the stored metadata/BSL snapshot directly from the project
// directory under the same lock as Studio's own file saves.
func (workspace *Workspace) SaveData(ctx context.Context, consent schemadiff.MigrationConsent) (publication.SavedState, schemadiff.MigrationRecord, error) {
	workspace.mu.Lock()
	provider, root := workspace.saveData, workspace.root
	workspace.mu.Unlock()
	if provider == nil {
		return publication.SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("saving data is unavailable in this build")
	}
	return provider(ctx, root, consent)
}

// NewHandler serves the local read-only Studio shell for one workspace.
func NewHandler(workspace *Workspace) http.Handler {
	routes := http.NewServeMux()
	registerDebugRoutes(routes, workspace)
	registerRoleRoutes(routes, workspace)
	registerSessionModuleRoutes(routes, workspace)
	registerCatalogEditorRoutes(routes, workspace)
	registerProjectEditorRoutes(routes, workspace)
	routes.Handle("GET /ui/", http.FileServer(http.FS(assets)))
	routes.HandleFunc("GET /{$}", func(response http.ResponseWriter, _ *http.Request) {
		page, err := assets.ReadFile("ui/index.html")
		if err != nil {
			http.Error(response, "ML Studio UI unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(page)
	})
	routes.HandleFunc("GET /api/project", func(response http.ResponseWriter, _ *http.Request) {
		snapshot, err := workspace.Snapshot()
		if err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(response).Encode(snapshot)
	})
	routes.HandleFunc("GET /api/file", func(response http.ResponseWriter, request *http.Request) {
		file, err := workspace.ReadSource(request.URL.Query().Get("path"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		entityTag := `"` + file.Revision + `"`
		response.Header().Set("ETag", entityTag)
		if request.Header.Get("If-None-Match") == entityTag {
			response.WriteHeader(http.StatusNotModified)
			return
		}
		writeStudioJSON(response, file)
	})
	routes.HandleFunc("PUT /api/file", func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-ML-CSRF") != "1" {
			http.Error(response, "CSRF check failed", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path             string `json:"path"`
			Content          string `json:"content"`
			ExpectedRevision string `json:"expectedRevision"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		file, err := workspace.SaveSource(input.Path, input.Content, input.ExpectedRevision)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, file)
	})
	routes.HandleFunc("GET /api/form", func(response http.ResponseWriter, request *http.Request) {
		form, err := workspace.ReadManagedForm(request.URL.Query().Get("path"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, form)
	})
	routes.HandleFunc("PUT /api/form", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path             string               `json:"path"`
			ExpectedRevision string               `json:"expectedRevision"`
			Form             metadata.ManagedForm `json:"form"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		form, err := workspace.SaveManagedForm(input.Path, input.Form, input.ExpectedRevision)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, form)
	})
	routes.HandleFunc("POST /api/form/handler", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path             string `json:"path"`
			ExpectedRevision string `json:"expectedRevision"`
			Command          string `json:"command"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		commandID, err := uuid.Parse(input.Command)
		if err != nil {
			http.Error(response, "Invalid form command UUID", http.StatusBadRequest)
			return
		}
		result, err := workspace.EnsureManagedFormHandler(input.Path, input.ExpectedRevision, commandID)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("GET /api/query/schema", func(response http.ResponseWriter, _ *http.Request) {
		schema, err := workspace.QueryDesignerSchema()
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, schema)
	})
	routes.HandleFunc("POST /api/query/build", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var design QueryDesign
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, querylang.MaxSourceBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&design); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		result, err := workspace.BuildDesignedQuery(design)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("POST /api/bsl/analyze", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		relative, language, err := validateEditablePath(input.Path)
		if err != nil || language != "bsl" {
			http.Error(response, "BSL analysis requires a canonical project module path", http.StatusBadRequest)
			return
		}
		analysis, err := AnalyzeBSL(relative, input.Content)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeStudioJSON(response, analysis)
	})
	routes.HandleFunc("POST /api/bsl/complete", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path     string      `json:"path"`
			Content  string      `json:"content"`
			Position BSLPosition `json:"position"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		completion, err := workspace.CompleteBSL(input.Path, input.Content, input.Position)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeStudioJSON(response, completion)
	})
	routes.HandleFunc("POST /api/bsl/navigate", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path     string      `json:"path"`
			Content  string      `json:"content"`
			Position BSLPosition `json:"position"`
			Mode     string      `json:"mode"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		result, err := workspace.NavigateBSL(input.Path, input.Content, input.Position, input.Mode)
		if err != nil {
			writeBSLNavigationError(response, err)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("POST /api/bsl/rename", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path             string      `json:"path"`
			Position         BSLPosition `json:"position"`
			NewName          string      `json:"newName"`
			ExpectedRevision string      `json:"expectedRevision"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		result, err := workspace.RenameBSL(input.Path, input.Position, input.NewName, input.ExpectedRevision)
		if err != nil {
			writeBSLNavigationError(response, err)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("GET /api/search", func(response http.ResponseWriter, request *http.Request) {
		result, err := workspace.SearchProject(request.URL.Query().Get("query"))
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("GET /api/bsl/help", func(response http.ResponseWriter, request *http.Request) {
		result, err := workspace.SearchBSLHelp(request.URL.Query().Get("query"))
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		writeStudioJSON(response, result)
	})
	routes.HandleFunc("POST /api/bsl/help/resolve", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		var input struct {
			Path     string      `json:"path"`
			Content  string      `json:"content"`
			Position BSLPosition `json:"position"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(response, "Invalid request", http.StatusBadRequest)
			return
		}
		item, err := workspace.ResolveBSLHelp(input.Path, input.Content, input.Position)
		if err != nil {
			writeBSLNavigationError(response, err)
			return
		}
		writeStudioJSON(response, item)
	})
	routes.HandleFunc("GET /api/git/status", func(response http.ResponseWriter, request *http.Request) {
		client, err := gitclient.Open(request.Context(), workspace.root)
		if err != nil {
			writeGitError(response, err)
			return
		}
		status, err := client.Status(request.Context())
		if err != nil {
			writeGitError(response, err)
			return
		}
		writeStudioJSON(response, status)
	})
	routes.HandleFunc("GET /api/git/diff", func(response http.ResponseWriter, request *http.Request) {
		client, err := gitclient.Open(request.Context(), workspace.root)
		if err != nil {
			writeGitError(response, err)
			return
		}
		diff, err := client.Diff(request.Context(), request.URL.Query().Get("path"))
		if err != nil {
			writeGitError(response, err)
			return
		}
		writeStudioJSON(response, map[string]string{"diff": diff})
	})
	routes.HandleFunc("GET /api/git/branches", func(response http.ResponseWriter, request *http.Request) {
		client, err := gitclient.Open(request.Context(), workspace.root)
		if err != nil {
			writeGitError(response, err)
			return
		}
		branches, err := client.Branches(request.Context())
		if err != nil {
			writeGitError(response, err)
			return
		}
		writeStudioJSON(response, branches)
	})
	routes.HandleFunc("POST /api/git/commit", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Message string `json:"message"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		client, err := gitclient.Open(request.Context(), workspace.root)
		if err == nil {
			var result gitclient.Result
			result, err = client.Commit(request.Context(), input.Message)
			if err == nil {
				writeStudioJSON(response, result)
				return
			}
		}
		writeGitError(response, err)
	})
	for path, operation := range map[string]func(context.Context, *gitclient.Client) (gitclient.Result, error){
		"/api/git/pull": func(ctx context.Context, client *gitclient.Client) (gitclient.Result, error) { return client.Pull(ctx) },
		"/api/git/push": func(ctx context.Context, client *gitclient.Client) (gitclient.Result, error) { return client.Push(ctx) },
	} {
		routes.HandleFunc("POST "+path, func(response http.ResponseWriter, request *http.Request) {
			if !validateStudioMutation(response, request) {
				return
			}
			client, err := gitclient.Open(request.Context(), workspace.root)
			if err == nil {
				var result gitclient.Result
				result, err = operation(request.Context(), client)
				if err == nil {
					if path == "/api/git/pull" {
						workspace.invalidateBSLIndex()
					}
					writeStudioJSON(response, result)
					return
				}
			}
			writeGitError(response, err)
		})
	}
	routes.HandleFunc("POST /api/git/branches/switch", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name   string `json:"name"`
			Create bool   `json:"create"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		client, err := gitclient.Open(request.Context(), workspace.root)
		if err == nil {
			var result gitclient.Result
			result, err = client.SwitchBranch(request.Context(), input.Name, input.Create)
			if err == nil {
				workspace.invalidateBSLIndex()
				writeStudioJSON(response, result)
				return
			}
		}
		writeGitError(response, err)
	})
	routes.HandleFunc("POST /api/save-data", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		var input struct {
			Consent schemadiff.MigrationConsent `json:"consent"`
		}
		_ = json.NewDecoder(io.LimitReader(request.Body, 1<<10)).Decode(&input)
		saved, migration, err := workspace.SaveData(request.Context(), input.Consent)
		if err != nil {
			writeSaveDataError(response, workspace.PhysicalNames(request.Context()), err)
			return
		}
		writeStudioJSON(response, struct {
			GitCommit        string `json:"gitCommit"`
			SchemaSHA256     string `json:"schemaSha256"`
			SavedAt          string `json:"savedAt"`
			MigrationStatus  string `json:"migrationStatus"`
			DestructiveCount int    `json:"destructiveCount"`
		}{
			GitCommit: saved.GitCommit, SchemaSHA256: saved.SchemaSHA256, SavedAt: saved.SavedAt.Format("2006-01-02T15:04:05Z07:00"),
			MigrationStatus: migration.Status, DestructiveCount: migration.DestructiveCount,
		})
	})
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Frame-Options", "DENY")
		routes.ServeHTTP(response, request)
	})
}

func validateStudioMutation(response http.ResponseWriter, request *http.Request) bool {
	if request.Header.Get("X-ML-CSRF") != "1" {
		http.Error(response, "CSRF check failed", http.StatusForbidden)
		return false
	}
	return true
}

func decodeStudioMutation(response http.ResponseWriter, request *http.Request, value any) bool {
	if !validateStudioMutation(response, request) {
		return false
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeSaveDataError(response http.ResponseWriter, names map[string]string, err error) {
	// A refused migration is answered with WHAT needs confirming and HOW MUCH
	// data each part touches, not with a sentence telling the developer that
	// something somewhere is destructive.
	var needed *schemadiff.ConfirmationNeeded
	if errors.As(err, &needed) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(response).Encode(struct {
			Error        string            `json:"error"`
			ObjectLoss   bool              `json:"objectLoss"`
			ValueRewrite bool              `json:"valueRewrite"`
			Changes      []migrationDetail `json:"changes"`
		}{
			Error:      "Изменения затрагивают существующие данные — требуется явное подтверждение",
			ObjectLoss: needed.Plan.ObjectLossCount > 0, ValueRewrite: needed.Plan.ValueRewriteCount > 0,
			Changes: describeMigration(names, needed),
		})
		return
	}
	switch {
	case errors.Is(err, publication.ErrDirtyPrimary):
		http.Error(response, "Сохраните и закоммитьте изменения в Git перед сохранением данных на основной базе", http.StatusConflict)
	case errors.Is(err, gitclient.ErrGitUnavailable), errors.Is(err, gitclient.ErrNotRepository), errors.Is(err, gitclient.ErrUnsafeRepository):
		writeGitError(response, err)
	default:
		http.Error(response, err.Error(), http.StatusBadRequest)
	}
}

func writeGitError(response http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, gitclient.ErrGitUnavailable) {
		status = http.StatusServiceUnavailable
	} else if errors.Is(err, gitclient.ErrNotRepository) || errors.Is(err, gitclient.ErrUnsafeRepository) {
		status = http.StatusConflict
	}
	http.Error(response, err.Error(), status)
}

func writeStudioJSON(response http.ResponseWriter, value any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		http.Error(response, "Unable to encode response", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = response.Write(body.Bytes())
}

func writeSourceError(response http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrSourceChanged):
		status = http.StatusConflict
	case errors.Is(err, ErrSourceNotFound):
		status = http.StatusNotFound
	}
	http.Error(response, err.Error(), status)
}

func writeBSLNavigationError(response http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrSourceChanged), errors.Is(err, ErrBSLRenameConflict):
		status = http.StatusConflict
	case errors.Is(err, ErrSourceNotFound), errors.Is(err, ErrBSLSymbolNotFound), errors.Is(err, ErrBSLHelpNotFound):
		status = http.StatusNotFound
	}
	http.Error(response, err.Error(), status)
}

// metadataTree builds the configuration tree's top-level branches: the
// "Общие" group and the object kinds standing beside it. Sources never form
// branches of their own - a module or a form is always reached through the
// object that owns it, the way the specification fixes the tree.
func (workspace *Workspace) metadataTree(language string, languages []project.Language) ([]Node, error) {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata"))
	if err != nil {
		return nil, fmt.Errorf("read metadata directory: %w", err)
	}
	known := make(map[string]bool, len(project.MetadataKinds()))
	for _, kind := range project.MetadataKinds() {
		known[kind] = true
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !known[entry.Name()] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected metadata path %q", filepath.Join("metadata", entry.Name()))
		}
	}
	// A tree listing must stay usable even while some object's metadata
	// doesn't yet fully validate (e.g. mid-edit, or an older/minimal
	// fixture) — so a load failure here just means the per-kind data
	// groups below (Реквизиты, Измерения, ...) come back empty, not a
	// broken tree.
	// Modules form no branch of their own - each one is reached through the
	// object that owns it - but the directory is still checked here, so a
	// stray file is reported when the tree is read and not only when data is
	// saved into the database.
	if _, err := workspace.sourceFiles(filepath.Join(workspace.root, "modules"), "modules", ".bsl", "modules", language, languages); err != nil {
		return nil, err
	}
	loaded, _ := metadata.Load(workspace.root)
	built := make(map[string]Node, len(project.MetadataKinds()))
	for _, kind := range project.MetadataKinds() {
		relative := filepath.Join("metadata", kind)
		node := Node{ID: "metadata/" + kind, Kind: "metadata-group", Title: metadataTitle(kind), Path: filepath.ToSlash(relative)}
		path := filepath.Join(workspace.root, relative)
		switch {
		case kind == "languages":
			// Languages have no per-object UUID or file of their own (they're
			// a plain list inside mlproject.yaml), so this branch is
			// synthesized directly from the already in-memory list rather
			// than scanned from disk like every other metadata kind.
			// The group node itself stays inert — matching every other
			// metadata-group node (e.g. "Справочники" shows nothing until
			// you open a specific catalog) — only an individual language
			// leaf carries a Path/Fragment, opening the project editor and
			// switching straight to that one language's own properties.
			for _, item := range languages {
				node.Children = append(node.Children, Node{
					ID: "language:" + item.Code, Kind: "language", Title: item.Name,
					Path: project.ManifestFile, Fragment: "language:" + item.Code,
					Properties: []Property{{Name: "Код", Value: item.Code}, {Name: "Имя", Value: item.Name}},
				})
			}
			sortNodesByTitle(node.Children)
		default:
			if _, err := os.Stat(path); err == nil {
				var buildErr error
				if slices.Contains(project.ObjectFolderKinds(), kind) {
					node.Children, buildErr = workspace.objectFolderNodes(path, filepath.ToSlash(relative), "metadata", language, languages, objectDataGroups(loaded, kind, language, languages))
				} else if named, ok := project.NamedFolderFiles(kind); ok {
					// A kind that keeps only files still keeps them in a folder
					// named after the object: its description, and the modules
					// that run it.
					node.Children, buildErr = namedFolderNodes(path, filepath.ToSlash(relative), kind, named)
				} else {
					node.Children, buildErr = workspace.sourceFiles(path, filepath.ToSlash(relative), ".yaml", "metadata", language, languages)
				}
				if buildErr != nil {
					return nil, buildErr
				}
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("inspect metadata directory %q: %w", kind, err)
			}
		}
		// Studio-only navigation grouping belongs inside each category's own
		// list, never as its own top-level metadata category.
		if kind == "folders" {
			continue
		}
		// A common module keeps its BSL in the shared modules directory, so
		// without this leaf the code would have no place in the tree at all:
		// unlike a catalog, a common module has nothing else to open.
		if kind == "common-modules" {
			workspace.attachCommonModuleSources(&node)
		}
		node.Properties = countProperties(node.Path, len(node.Children))
		built[kind] = node
	}
	if documents, ok := built["documents"]; ok {
		var nested []Node
		for _, kind := range documentNestedKindOrder {
			if node, exists := built[kind]; exists {
				nested = append(nested, node)
			}
		}
		// Счётчик у «Документов» уже посчитан и остаётся числом документов:
		// вложенные ветви - не документы.
		documents.Children = append(nested, documents.Children...)
		built["documents"] = documents
	}
	common := Node{ID: "metadata/common", Kind: "group", Title: "Общие", Path: "metadata"}
	for _, kind := range commonKindOrder {
		if node, ok := built[kind]; ok {
			common.Children = append(common.Children, node)
		}
	}
	common.Properties = countProperties(common.Path, countDescendants(common))
	branches := []Node{common}
	for _, kind := range topLevelKindOrder {
		if node, ok := built[kind]; ok {
			branches = append(branches, node)
		}
	}
	return branches, nil
}

// attachCommonModuleSources hangs each common module's BSL file under its own
// node, matching how every other object reaches its module through itself.
// The reference is read from the description file rather than from the loaded
// catalogue on purpose: the tree has to stay usable while some unrelated
// object is mid-edit and the catalogue as a whole does not yet validate.
func (workspace *Workspace) attachCommonModuleSources(node *Node) {
	for index := range node.Children {
		file, err := workspace.readSource(node.Children[index].Path)
		if err != nil {
			continue
		}
		var descriptor struct {
			Module uuid.UUID `yaml:"module"`
		}
		if yaml.Unmarshal([]byte(file.Content), &descriptor) != nil || descriptor.Module.IsZero() {
			continue
		}
		path, err := project.ModulePath(descriptor.Module)
		if err != nil {
			continue
		}
		node.Children[index].Children = append(node.Children[index].Children, Node{
			ID: descriptor.Module.String(), Kind: "modules", Title: "Модуль", Path: path,
			Properties: []Property{{Name: "UUID", Value: descriptor.Module.String()}, {Name: "Путь", Value: path}},
		})
	}
}

func (workspace *Workspace) sourceFiles(directory, relative, extension, kind, language string, languages []project.Language) ([]Node, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read project directory %q: %w", relative, err)
	}
	nodes := make([]Node, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected source path %q", filepath.ToSlash(filepath.Join(relative, entry.Name())))
		}
		ext := filepath.Ext(entry.Name())
		if extension != "" && ext != extension || extension == "" && ext == "" {
			return nil, fmt.Errorf("unexpected source file %q", filepath.ToSlash(filepath.Join(relative, entry.Name())))
		}
		id, err := uuid.Parse(strings.TrimSuffix(entry.Name(), ext))
		if err != nil {
			return nil, fmt.Errorf("source file %q must use a UUID name: %w", entry.Name(), err)
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect source file %q: %w", entry.Name(), err)
		}
		path := filepath.ToSlash(filepath.Join(relative, entry.Name()))
		title := id.String()
		if ext == ".yaml" {
			title, err = yamlSourceTitle(filepath.Join(directory, entry.Name()), path, title, info.Size(), language, languages)
			if err != nil {
				return nil, err
			}
		}
		node := Node{
			ID: id.String(), Kind: kind, Title: title, Path: path,
			Properties: []Property{{Name: "UUID", Value: id.String()}, {Name: "Путь", Value: path}, {Name: "Размер", Value: fmt.Sprintf("%d байт", info.Size())}},
		}
		nodes = append(nodes, node)
	}
	sortNodesByTitle(nodes)
	return nodes, nil
}

// objectFolderNodes lists one metadata kind's per-object folders (catalogs,
// documents, information/accumulation registers): each node represents one
// object, titled from its own object.yaml, with its data groups (Реквизиты,
// Табличные части, ...), Формы/Команды/Макеты, and module(s) as children —
// everything physically grouped under that one folder.
func (workspace *Workspace) objectFolderNodes(directory, relative, kind, language string, languages []project.Language, dataGroups func(id uuid.UUID, descriptionPath string) []Node) ([]Node, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read project directory %q: %w", relative, err)
	}
	nodes := make([]Node, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected source path %q", filepath.ToSlash(filepath.Join(relative, entry.Name())))
		}
		if err := project.ObjectName(entry.Name()); err != nil {
			return nil, fmt.Errorf("metadata object folder %q: %w", entry.Name(), err)
		}
		objectDirectory := filepath.Join(directory, entry.Name())
		objectRelative := filepath.ToSlash(filepath.Join(relative, entry.Name()))
		descriptionPath := filepath.ToSlash(filepath.Join(objectRelative, "object.yaml"))
		info, err := os.Stat(filepath.Join(objectDirectory, "object.yaml"))
		if err != nil {
			return nil, fmt.Errorf("inspect metadata object %q: %w", descriptionPath, err)
		}
		title, id, err := yamlSourceTitleAndID(filepath.Join(objectDirectory, "object.yaml"), descriptionPath, entry.Name(), info.Size())
		if err != nil {
			return nil, err
		}
		owned, err := objectOwnedFileNodes(objectDirectory, objectRelative)
		if err != nil {
			return nil, err
		}
		var children []Node
		if dataGroups != nil {
			children = append(children, dataGroups(id, descriptionPath)...)
		}
		children = append(children,
			Node{ID: id.String() + ":forms", Kind: "group", Title: "Формы", Children: owned.forms},
			// A command shows the module that runs it, because that is the
			// whole of what a command keeps on disk. "Макеты" still stands in
			// empty: a template's content has no editor yet.
			Node{ID: id.String() + ":commands", Kind: "group", Title: "Команды", Children: owned.commands},
			Node{ID: id.String() + ":templates", Kind: "group", Title: "Макеты"},
		)
		children = append(children, owned.modules...)
		nodes = append(nodes, Node{
			ID: id.String(), Kind: kind, Title: title, Path: descriptionPath, Children: children,
			Properties: []Property{{Name: "UUID", Value: id.String()}, {Name: "Путь", Value: descriptionPath}, {Name: "Размер", Value: fmt.Sprintf("%d байт", info.Size())}},
		})
	}
	sortNodesByTitle(nodes)
	return nodes, nil
}

// objectModuleTitles reads a module file name as what the module is, so the
// tree says "Модуль объекта" where the file says МодульОбъекта.bsl.
var objectModuleTitles = map[string]string{
	project.ObjectModuleFile:    "Модуль объекта",
	project.ManagerModuleFile:   "Модуль менеджера",
	project.RecordSetModuleFile: "Модуль набора записей",
}

// objectOwnedNodes are the tree nodes for what one object keeps in its folder,
// kept apart because each goes to a different place in the tree: forms and
// commands under groups of their own, modules as flat leaves beside them -
// which is how the prototype shows an object's own modules too.
type objectOwnedNodes struct {
	modules, forms, commands []Node
}

// objectOwnedFileNodes lists one object's own modules, managed forms and
// commands, physically stored alongside its object.yaml.
//
// A module is recognised by its name, because the name is the whole of its
// identity: МодульОбъекта.bsl is the object module of whatever object's folder
// it lies in, and nothing declares it anywhere else.
func objectOwnedFileNodes(objectDirectory, objectRelative string) (objectOwnedNodes, error) {
	var owned objectOwnedNodes
	entries, err := os.ReadDir(objectDirectory)
	if err != nil {
		return objectOwnedNodes{}, fmt.Errorf("read metadata object %q: %w", objectRelative, err)
	}
	for _, entry := range entries {
		if entry.Name() == project.ObjectMetadataFile {
			continue
		}
		unexpected := func(name string) error {
			return fmt.Errorf("unexpected source path %q", filepath.ToSlash(filepath.Join(objectRelative, name)))
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return objectOwnedNodes{}, unexpected(entry.Name())
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "forms":
				owned.forms, err = objectFormNodes(objectDirectory, objectRelative, "forms")
			case "commands":
				owned.commands, err = objectCommandNodes(objectDirectory, objectRelative)
			case "templates":
				continue
			default:
				return objectOwnedNodes{}, unexpected(entry.Name())
			}
			if err != nil {
				return objectOwnedNodes{}, err
			}
			continue
		}
		title, ok := objectModuleTitles[entry.Name()]
		if !ok {
			return objectOwnedNodes{}, unexpected(entry.Name())
		}
		path := filepath.ToSlash(filepath.Join(objectRelative, entry.Name()))
		owned.modules = append(owned.modules, Node{
			ID: path, Kind: "modules", Title: title, Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		})
	}
	sort.Slice(owned.modules, func(left, right int) bool { return owned.modules[left].Path < owned.modules[right].Path })
	sort.Slice(owned.forms, func(left, right int) bool { return owned.forms[left].Path < owned.forms[right].Path })
	sort.Slice(owned.commands, func(left, right int) bool { return owned.commands[left].Title < owned.commands[right].Title })
	return owned, nil
}

// objectFormNodes lists the managed forms of one object. A form is a folder
// named after the form, and the tree shows that name - the identifier the form
// keeps inside is what other things refer to it by, not what a developer reads.
// objectFormNodes lists the forms kept under one folder - an object's forms/,
// or the configuration's common-forms/, which hold their forms the same way.
func objectFormNodes(parentDirectory, parentRelative, forms string) ([]Node, error) {
	entries, err := os.ReadDir(filepath.Join(parentDirectory, forms))
	if err != nil {
		return nil, fmt.Errorf("read metadata forms %q: %w", parentRelative, err)
	}
	var nodes []Node
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || project.SubordinateName(entry.Name()) != nil {
			return nil, fmt.Errorf("unexpected form source %q", filepath.ToSlash(filepath.Join(parentRelative, forms, entry.Name())))
		}
		path := filepath.ToSlash(filepath.Join(parentRelative, forms, entry.Name(), project.FormMetadataFile))
		node := Node{
			ID: path, Kind: "forms", Title: entry.Name(), Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		}
		// The module hangs under its form, because that is where it lies and
		// the only thing that says it exists is that it lies there.
		modulePath := filepath.ToSlash(filepath.Join(parentRelative, forms, entry.Name(), project.FormModuleFile))
		if info, err := os.Lstat(filepath.Join(parentDirectory, forms, entry.Name(), project.FormModuleFile)); err == nil && info.Mode().IsRegular() {
			node.Children = append(node.Children, Node{
				ID: modulePath, Kind: "modules", Title: "Модуль формы", Path: modulePath,
				Properties: []Property{{Name: "Путь", Value: modulePath}},
			})
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// namedFolderFileTitles reads a file name as what the file is, so the tree
// says "Модуль значения" where the folder says МодульЗначения.bsl.
var namedFolderFileTitles = map[string]string{
	project.CommandModuleFile: "Модуль команды",
	project.FormModuleFile:    "Модуль формы",
	project.ValueModuleFile:   "Модуль значения",
	project.ManagerModuleFile: "Модуль менеджера",
}

// namedFolderNodes lists the objects of a kind that keeps only files: each in a
// folder named after it, holding its description and the modules that run it.
// The node opens the description; the modules hang beneath.
func namedFolderNodes(directory, relative, kind string, named []string) ([]Node, error) {
	description := project.ObjectMetadataFile
	if slices.Contains(named, project.FormMetadataFile) {
		description = project.FormMetadataFile
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read metadata %s %q: %w", kind, relative, err)
	}
	var nodes []Node
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || project.ObjectName(entry.Name()) != nil {
			return nil, fmt.Errorf("unexpected source path %q", filepath.ToSlash(filepath.Join(relative, entry.Name())))
		}
		path := filepath.ToSlash(filepath.Join(relative, entry.Name(), description))
		node := Node{
			ID: path, Kind: kind, Title: entry.Name(), Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		}
		for _, file := range named {
			if file == description {
				continue
			}
			info, err := os.Lstat(filepath.Join(directory, entry.Name(), file))
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			modulePath := filepath.ToSlash(filepath.Join(relative, entry.Name(), file))
			node.Children = append(node.Children, Node{
				ID: modulePath, Kind: "modules", Title: namedFolderFileTitles[file], Path: modulePath,
				Properties: []Property{{Name: "Путь", Value: modulePath}},
			})
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Title < nodes[right].Title })
	return nodes, nil
}

// objectCommandNodes lists the commands of one object. A command is a folder
// named after the command, and what it holds is the module that runs it - so
// the node carries the command's name and opens its module.
func objectCommandNodes(objectDirectory, objectRelative string) ([]Node, error) {
	entries, err := os.ReadDir(filepath.Join(objectDirectory, "commands"))
	if err != nil {
		return nil, fmt.Errorf("read metadata object commands %q: %w", objectRelative, err)
	}
	var nodes []Node
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || project.SubordinateName(entry.Name()) != nil {
			return nil, fmt.Errorf("unexpected command source %q", filepath.ToSlash(filepath.Join(objectRelative, "commands", entry.Name())))
		}
		path := filepath.ToSlash(filepath.Join(objectRelative, "commands", entry.Name(), project.CommandModuleFile))
		nodes = append(nodes, Node{
			ID: path, Kind: "commands", Title: entry.Name(), Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		})
	}
	return nodes, nil
}

// objectDataGroups returns, per metadata kind, a function building that
// object's data-shape tree groups (Реквизиты, Измерения, Ресурсы, Табличные
// части, ...) — always present, even empty, so a catalog/document/register
// node has the same fixed subtree shape 1C shows regardless of content.
// loaded may be nil (metadata failed to fully validate); the resulting
// groups then simply come back empty rather than breaking the whole tree.
func objectDataGroups(loaded *metadata.Catalog, kind, language string, languages []project.Language) func(id uuid.UUID, descriptionPath string) []Node {
	switch kind {
	case "catalogs":
		return func(id uuid.UUID, descriptionPath string) []Node {
			var definition metadata.CatalogDefinition
			if loaded != nil {
				definition, _ = loaded.CatalogByID(id)
			}
			return []Node{
				attributeGroupNode(id.String()+":attributes", "Реквизиты", definition.Attributes, descriptionPath, language, languages),
				tablePartGroupNode(id.String()+":table-parts", "Табличные части", definition.TableParts, descriptionPath, language, languages),
			}
		}
	case "documents":
		return func(id uuid.UUID, descriptionPath string) []Node {
			var definition metadata.DocumentDefinition
			if loaded != nil {
				definition, _ = loaded.DocumentByID(id)
			}
			return []Node{
				attributeGroupNode(id.String()+":attributes", "Реквизиты", definition.Attributes, descriptionPath, language, languages),
				tablePartGroupNode(id.String()+":table-parts", "Табличные части", definition.TableParts, descriptionPath, language, languages),
			}
		}
	case "information-registers":
		return func(id uuid.UUID, descriptionPath string) []Node {
			var definition metadata.InformationRegisterDefinition
			if loaded != nil {
				definition, _ = loaded.InformationRegisterByID(id)
			}
			return []Node{
				attributeGroupNode(id.String()+":dimensions", "Измерения", definition.Dimensions, descriptionPath, language, languages),
				attributeGroupNode(id.String()+":resources", "Ресурсы", definition.Resources, descriptionPath, language, languages),
				attributeGroupNode(id.String()+":attributes", "Реквизиты", definition.Attributes, descriptionPath, language, languages),
			}
		}
	case "accumulation-registers":
		return func(id uuid.UUID, descriptionPath string) []Node {
			var definition metadata.AccumulationRegisterDefinition
			if loaded != nil {
				definition, _ = loaded.AccumulationRegisterByID(id)
			}
			return []Node{
				attributeGroupNode(id.String()+":dimensions", "Измерения", definition.Dimensions, descriptionPath, language, languages),
				attributeGroupNode(id.String()+":resources", "Ресурсы", definition.Resources, descriptionPath, language, languages),
				attributeGroupNode(id.String()+":attributes", "Реквизиты", definition.Attributes, descriptionPath, language, languages),
			}
		}
	default:
		return nil
	}
}

// attributeGroupNode builds one always-visible tree group (Реквизиты,
// Измерения, Ресурсы, ...) listing one object's plain attributes. Each leaf
// carries the object's own object.yaml as Path (so selecting it opens the
// matching visual editor once one exists for this kind) and a Fragment
// identifying which attribute to select there.
func attributeGroupNode(groupID, title string, attributes []metadata.Attribute, descriptionPath, language string, languages []project.Language) Node {
	children := make([]Node, 0, len(attributes))
	for _, attribute := range attributes {
		children = append(children, Node{
			ID: attribute.ID.String(), Kind: "attribute",
			Title: attribute.Name,
			Path:  descriptionPath, Fragment: "attribute:" + attribute.ID.String(),
			Properties: []Property{{Name: "UUID", Value: attribute.ID.String()}},
		})
	}
	sortNodesByTitle(children)
	return Node{ID: groupID, Kind: "group", Title: title, Path: descriptionPath, Children: children}
}

// tablePartGroupNode builds the always-visible "Табличные части" group,
// nesting each table part's own attributes as informational children.
func tablePartGroupNode(groupID, title string, tableParts []metadata.TablePart, descriptionPath, language string, languages []project.Language) Node {
	children := make([]Node, 0, len(tableParts))
	for _, part := range tableParts {
		fragment := "tablepart:" + part.ID.String()
		partAttributes := make([]Node, 0, len(part.Attributes))
		for _, attribute := range part.Attributes {
			partAttributes = append(partAttributes, Node{
				ID: part.ID.String() + ":" + attribute.ID.String(), Kind: "attribute",
				Title: attribute.Name,
				Path:  descriptionPath, Fragment: fragment,
				Properties: []Property{{Name: "UUID", Value: attribute.ID.String()}},
			})
		}
		sortNodesByTitle(partAttributes)
		children = append(children, Node{
			ID: part.ID.String(), Kind: "tablepart",
			Title: part.Name,
			Path:  descriptionPath, Fragment: fragment, Children: partAttributes,
			Properties: []Property{{Name: "UUID", Value: part.ID.String()}},
		})
	}
	sortNodesByTitle(children)
	return Node{ID: groupID, Kind: "group", Title: title, Path: descriptionPath, Children: children}
}

// sortNodesByTitle orders tree nodes by their display title so that
// user-created objects (catalogs, attributes, languages, ...) always appear
// alphabetically within their group — recomputed on every tree build, not a
// persisted order, so a newly created or renamed object lands in its
// alphabetical place immediately without any separate "sort"/"move" action.
func sortNodesByTitle(nodes []Node) {
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Title < nodes[right].Title })
}

func yamlSourceTitle(filePath, relative, fallback string, size int64, language string, languages []project.Language) (string, error) {
	title, _, err := yamlSourceTitleAndID(filePath, relative, fallback, size)
	return title, err
}

// yamlSourceTitleAndID reads the two things the tree needs from an object's
// description: the name it is shown by, and the identifier everything else
// addresses it with. The folder is named by the name now, so the identifier
// has to come from inside the file.
func yamlSourceTitleAndID(filePath, relative, fallback string, size int64) (string, uuid.UUID, error) {
	if size > project.MaxYAMLDocumentBytes {
		return "", uuid.UUID{}, fmt.Errorf("read source %q: %w", relative, project.ErrYAMLDocumentTooLarge)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", uuid.UUID{}, fmt.Errorf("open source %q: %w", relative, err)
	}
	defer file.Close()
	var document yaml.Node
	if err := yaml.NewDecoder(io.LimitReader(file, project.MaxYAMLDocumentBytes+1)).Decode(&document); err != nil {
		return "", uuid.UUID{}, fmt.Errorf("decode source %q: %w", relative, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fallback, uuid.UUID{}, nil
	}
	// The tree always shows an object's Имя (Name), never its Заголовок
	// (Title) — matching 1C Configurator, where the tree is the developer's
	// technical view and the localized title is only what end users see.
	mapping := document.Content[0]
	title, id := fallback, uuid.UUID{}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if value.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "name":
			if name := strings.TrimSpace(value.Value); name != "" {
				title = name
			}
		case "id":
			if parsed, err := uuid.Parse(strings.TrimSpace(value.Value)); err == nil {
				id = parsed
			}
		}
	}
	return title, id, nil
}

func countProperties(path string, count int) []Property {
	return []Property{{Name: "Путь", Value: path}, {Name: "Объектов", Value: fmt.Sprint(count)}}
}

func countDescendants(node Node) int {
	count := 0
	for _, child := range node.Children {
		count += len(child.Children)
	}
	return count
}

func metadataTitle(kind string) string {
	if title := metadataTitles[kind]; title != "" {
		return title
	}
	words := strings.Split(strings.ReplaceAll(kind, "-", " "), " ")
	for index := range words {
		if words[index] != "" {
			words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
		}
	}
	return strings.Join(words, " ")
}

// migrationDetail is one line of the confirmation dialog: what will happen, to
// which object of the configuration, and to how many rows. The physical name is
// kept alongside the readable one - when something goes wrong afterwards, the
// PostgreSQL name is what the investigation needs.
type migrationDetail struct {
	Kind     string `json:"kind"`
	Impact   string `json:"impact"`
	Table    string `json:"table"`
	Object   string `json:"object,omitempty"`
	Title    string `json:"title"`
	Rows     int64  `json:"rows"`
	Measured bool   `json:"measured"`
	Reason   string `json:"reason,omitempty"`
}

func describeMigration(names map[string]string, needed *schemadiff.ConfirmationNeeded) []migrationDetail {
	measured := make(map[string]schemadiff.ImpactMeasurement, len(needed.Impacts))
	for _, impact := range needed.Impacts {
		measured[impact.Table+"\x00"+impact.Column] = impact
	}
	title := func(physical string) string {
		if human, ok := names[physical]; ok && human != "" {
			return human
		}
		return physical
	}
	result := make([]migrationDetail, 0, len(needed.Plan.Changes))
	for _, change := range needed.Plan.Changes {
		if change.Impact == schemadiff.ImpactNone {
			continue
		}
		detail := migrationDetail{Kind: string(change.Kind), Impact: string(change.Impact), Table: change.Table, Object: change.Object, Title: title(change.Table)}
		if change.Object != "" {
			// An attribute's name already carries its owner, so the owner is not
			// repeated: "Справочник.Товары · Цена", not the same words twice.
			object := strings.TrimPrefix(title(change.Object), detail.Title+".")
			detail.Title += " · " + object
		}
		if impact, ok := measured[change.Table+"\x00"+change.Object]; ok {
			detail.Rows, detail.Measured, detail.Reason = impact.Rows, true, impact.Reason
		}
		result = append(result, detail)
	}
	return result
}

// PhysicalNames translates this project's PostgreSQL names into configuration
// names for the migration dialog. A project that cannot be read yields no
// translations rather than failing the answer the caller is in the middle of.
func (workspace *Workspace) PhysicalNames(ctx context.Context) map[string]string {
	workspace.mu.Lock()
	root, saved := workspace.root, workspace.savedNames
	workspace.mu.Unlock()
	result := map[string]string{}
	// Saved names first, project names over them: what is being dropped is only
	// in the former, what was renamed is correct in the latter.
	if saved != nil {
		for physical, title := range saved(ctx) {
			result[physical] = title
		}
	}
	if catalog, err := metadata.Load(root); err == nil {
		for physical, title := range catalog.PhysicalNames() {
			result[physical] = title
		}
	}
	return result
}
