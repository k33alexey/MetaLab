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
	"sort"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/debugtarget"
	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/querylang"
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
	testRuntime   TestRuntimeProvider
	testRunning   bool
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
	Properties []Property `json:"properties,omitempty"`
	Children   []Node     `json:"children,omitempty"`
	Line       int        `json:"line,omitempty"`
}

// Property is one ordered value displayed in the properties panel.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var rootTitles = map[string]string{
	"metadata": "Метаданные", "modules": "Модули", "forms": "Формы",
	"reports": "Компоновка данных", "tests": "Тесты", "assets": "Ресурсы",
}

var metadataTitles = map[string]string{
	"subsystems":                     "Подсистемы",
	"common-modules":                 "Общие модули",
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
		ID: "project", Kind: "project", Title: manifest.Title, Path: project.ManifestFile,
		Properties: []Property{
			{Name: "Имя", Value: manifest.Name}, {Name: "Заголовок", Value: manifest.Title},
			{Name: "UUID", Value: manifest.ID.String()}, {Name: "Основной язык", Value: manifest.DefaultLanguage},
			{Name: "Формат", Value: fmt.Sprint(manifest.Format)},
		},
	}
	for _, directory := range project.RootDirectories() {
		var node Node
		if directory == "metadata" {
			node, err = workspace.metadataTree(manifest.DefaultLanguage, manifest.Languages)
		} else {
			node, err = workspace.sourceTree(directory, manifest.DefaultLanguage, manifest.Languages)
		}
		if err != nil {
			return Snapshot{}, err
		}
		root.Children = append(root.Children, node)
	}
	return Snapshot{ProjectPath: workspace.root, Manifest: manifest, Tree: root}, nil
}

// BuildPublicationPackage snapshots validated sources under the same lock as Studio saves.
func (workspace *Workspace) BuildPublicationPackage(ctx context.Context, destination string) (publication.Manifest, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	client, err := gitclient.Open(ctx, workspace.root)
	if err != nil {
		return publication.Manifest{}, err
	}
	status, err := client.Status(ctx)
	if err != nil {
		return publication.Manifest{}, err
	}
	if status.Revision == "" {
		return publication.Manifest{}, fmt.Errorf("ML Project must have at least one Git commit before packaging")
	}
	state := publication.SourceState{
		GitCommit: status.Revision,
		Dirty:     len(status.Entries) != 0,
	}
	manifest, err := publication.BuildFile(ctx, workspace.root, destination, state)
	if err != nil {
		return publication.Manifest{}, err
	}
	current, err := client.Status(ctx)
	if err != nil || current.Revision != state.GitCommit || (len(current.Entries) != 0) != state.Dirty {
		_ = os.Remove(destination)
		if err != nil {
			return publication.Manifest{}, err
		}
		return publication.Manifest{}, publication.ErrSourceChanged
	}
	return manifest, nil
}

// NewHandler serves the local read-only Studio shell for one workspace.
func NewHandler(workspace *Workspace) http.Handler {
	routes := http.NewServeMux()
	registerDebugRoutes(routes, workspace)
	registerTestRoutes(routes, workspace)
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
	routes.HandleFunc("POST /api/publication/package", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		temporaryDirectory, err := os.MkdirTemp("", "metalab-publication-*")
		if err != nil {
			http.Error(response, "Unable to create publication package", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(temporaryDirectory)
		destination := filepath.Join(temporaryDirectory, "publication"+publication.PackageExtension)
		manifest, err := workspace.BuildPublicationPackage(request.Context(), destination)
		if err != nil {
			writeGitError(response, err)
			return
		}
		file, err := os.Open(destination)
		if err != nil {
			http.Error(response, "Unable to open publication package", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		response.Header().Set("Content-Type", "application/vnd.metalab.package")
		response.Header().Set("Content-Disposition", `attachment; filename="publication.mlpkg"`)
		response.Header().Set("X-ML-Package-Digest", manifest.ContentSHA256)
		if _, err := io.Copy(response, file); err != nil {
			return
		}
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

func (workspace *Workspace) metadataTree(language string, languages []project.Language) (Node, error) {
	root := Node{ID: "metadata", Kind: "group", Title: rootTitles["metadata"], Path: "metadata"}
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata"))
	if err != nil {
		return Node{}, fmt.Errorf("read metadata directory: %w", err)
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
			return Node{}, fmt.Errorf("unexpected metadata path %q", filepath.Join("metadata", entry.Name()))
		}
	}
	for _, kind := range project.MetadataKinds() {
		relative := filepath.Join("metadata", kind)
		node := Node{ID: "metadata/" + kind, Kind: "metadata-group", Title: metadataTitle(kind), Path: filepath.ToSlash(relative)}
		path := filepath.Join(workspace.root, relative)
		if _, err := os.Stat(path); err == nil {
			node.Children, err = workspace.sourceFiles(path, filepath.ToSlash(relative), ".yaml", "metadata", language, languages)
			if err != nil {
				return Node{}, err
			}
		} else if !os.IsNotExist(err) {
			return Node{}, fmt.Errorf("inspect metadata directory %q: %w", kind, err)
		}
		node.Properties = countProperties(node.Path, len(node.Children))
		root.Children = append(root.Children, node)
	}
	root.Properties = countProperties(root.Path, countDescendants(root))
	return root, nil
}

func (workspace *Workspace) sourceTree(directory, language string, languages []project.Language) (Node, error) {
	extension, kind := "", directory
	switch directory {
	case "modules", "tests":
		extension = ".bsl"
	case "forms", "reports":
		extension = ".yaml"
	case "assets":
		kind = "asset"
	}
	children, err := workspace.sourceFiles(filepath.Join(workspace.root, directory), directory, extension, kind, language, languages)
	if err != nil {
		return Node{}, err
	}
	return Node{
		ID: directory, Kind: "group", Title: rootTitles[directory], Path: directory,
		Properties: countProperties(directory, len(children)), Children: children,
	}, nil
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
		if relative == "tests" {
			file, readErr := workspace.readSource(path)
			if readErr != nil {
				return nil, readErr
			}
			module, _ := syntax.Parse(path, file.Content)
			for _, routine := range module.Routines {
				if routine.Function || !routine.Export {
					continue
				}
				node.Children = append(node.Children, Node{
					ID: "test:" + id.String() + ":" + strings.ToLower(routine.Name), Kind: "test",
					Title: routine.Name, Path: path, Line: routine.SourceSpan.Start.Line,
					Properties: []Property{{Name: "Тест", Value: routine.Name}, {Name: "Строка", Value: fmt.Sprint(routine.SourceSpan.Start.Line)}},
				})
			}
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Path < nodes[right].Path })
	return nodes, nil
}

func yamlSourceTitle(filePath, relative, fallback string, size int64, language string, languages []project.Language) (string, error) {
	if size > project.MaxYAMLDocumentBytes {
		return "", fmt.Errorf("read source %q: %w", relative, project.ErrYAMLDocumentTooLarge)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open source %q: %w", relative, err)
	}
	defer file.Close()
	var document yaml.Node
	if err := yaml.NewDecoder(io.LimitReader(file, project.MaxYAMLDocumentBytes+1)).Decode(&document); err != nil {
		return "", fmt.Errorf("decode source %q: %w", relative, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fallback, nil
	}
	mapping := document.Content[0]
	name, title := "", ""
	localizedTitles := map[string]string{}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		switch key.Value {
		case "name":
			if value.Kind == yaml.ScalarNode {
				name = strings.TrimSpace(value.Value)
			}
		case "title":
			if value.Kind == yaml.ScalarNode {
				title = strings.TrimSpace(value.Value)
			} else if value.Kind == yaml.MappingNode {
				for item := 0; item+1 < len(value.Content); item += 2 {
					if value.Content[item].Kind == yaml.ScalarNode && value.Content[item+1].Kind == yaml.ScalarNode {
						localizedTitles[value.Content[item].Value] = strings.TrimSpace(value.Content[item+1].Value)
					}
				}
			}
		}
	}
	if title != "" {
		return title, nil
	}
	if title = localizedTitles[language]; title != "" {
		return title, nil
	}
	for _, configured := range languages {
		if title = localizedTitles[configured.Code]; title != "" {
			return title, nil
		}
	}
	localizedCodes := make([]string, 0, len(localizedTitles))
	for code := range localizedTitles {
		localizedCodes = append(localizedCodes, code)
	}
	sort.Strings(localizedCodes)
	for _, code := range localizedCodes {
		if title = localizedTitles[code]; title != "" {
			return title, nil
		}
	}
	if name != "" {
		return name, nil
	}
	return fallback, nil
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
