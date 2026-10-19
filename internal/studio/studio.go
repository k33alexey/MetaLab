// Package studio implements the local ML Studio workspace and HTTP surface.
package studio

import (
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

	"github.com/k33alexey/MetaLab/internal/brand"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/debugtarget"
	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/k33alexey/MetaLab/internal/webpage"
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
	ProjectPath   string          `json:"projectPath"`
	Configuration project.Project `json:"configuration"`
	Tree          Node            `json:"tree"`
	// Subsystems are what the tree's filter by subsystems selects by; absent
	// while the project's descriptions do not read as a whole, and the
	// filter is not offered then.
	Subsystems *metadata.SubsystemFilter `json:"subsystems,omitempty"`
}

// Node represents a stable item in the metadata tree.
type Node struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Icon       string     `json:"icon,omitempty"`
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

// metadataTitles are the captions of the kinds, for the editors that show
// objects of many kinds side by side.
var metadataTitles = func() map[string]string {
	result := map[string]string{}
	for _, descriptor := range kinds.All() {
		result[string(descriptor.Kind)] = descriptor.Titles
	}
	return result
}()

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
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return Snapshot{}, err
	}
	// The root's synonym is localized now, so the tree reads it the way it
	// reads every other synonym: in the project's own default language.
	synonym := configuration.Title.Resolve(configuration.DefaultLanguage)
	properties := []Property{
		{Name: "Имя", Value: configuration.Name}, {Name: "Синоним", Value: synonym},
		{Name: "UUID", Value: configuration.ID.String()}, {Name: "Основной язык", Value: configuration.DefaultLanguage},
		{Name: "Формат", Value: fmt.Sprint(configuration.Format)},
	}
	if configuration.Vendor != "" {
		properties = append(properties, Property{Name: "Поставщик", Value: configuration.Vendor})
	}
	if configuration.Version != "" {
		properties = append(properties, Property{Name: "Версия", Value: configuration.Version})
	}
	root := Node{
		ID: "project", Kind: "project", Icon: configurationIcon, Title: configuration.Name, Path: project.ConfigurationFile,
		Properties: properties,
	}
	root.Children = append(root.Children, workspace.rootModuleNodesLocked()...)
	root.Children = append(root.Children, workspace.rootPictureNodesLocked()...)
	// A tree listing must stay usable even while some object's metadata
	// doesn't yet fully validate (e.g. mid-edit, or an older/minimal
	// fixture) — so a load failure here just means the per-kind data
	// groups (Реквизиты, Измерения, ...) come back empty and the filter by
	// subsystems is not offered, not a broken tree.
	loaded, _ := metadata.Load(workspace.root)
	branches, err := workspace.metadataTree(loaded, configuration.DefaultLanguage, configuration.Languages)
	if err != nil {
		return Snapshot{}, err
	}
	root.Children = append(root.Children, branches...)
	snapshot := Snapshot{ProjectPath: workspace.root, Configuration: configuration, Tree: root}
	if loaded != nil {
		filter := loaded.SubsystemFilter()
		snapshot.Subsystems = &filter
	}
	return snapshot, nil
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
	registerRootModuleRoutes(routes, workspace)
	registerCatalogEditorRoutes(routes, workspace)
	registerProjectEditorRoutes(routes, workspace)
	routes.Handle("GET /ui/", http.FileServer(http.FS(assets)))
	routes.HandleFunc("GET /ui/icons.svg", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "image/svg+xml")
		_, _ = response.Write(icons.Sprite())
	})
	// The tab of Studio shows the icon of MetaLab, as the application's tab
	// does (the owner's choice of 17.10.2026).
	routes.HandleFunc("GET /favicon.ico", func(response http.ResponseWriter, _ *http.Request) {
		icon, _ := brand.Icon("icon-192.png")
		response.Header().Set("Content-Type", "image/png")
		response.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = response.Write(icon)
	})
	routes.HandleFunc("GET /{$}", webpage.Page(assets, "ML Studio"))
	// A project that does not read is in conflict with what Studio shows,
	// not a request it could not use.
	routes.HandleFunc("GET /api/project", answerQuery(func(response http.ResponseWriter, err error) {
		http.Error(response, err.Error(), http.StatusConflict)
	}, func(*http.Request) (any, error) {
		return workspace.Snapshot()
	}))
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
		webpage.JSON(response, http.StatusOK, file)
	})
	routes.HandleFunc("PUT /api/file", answerStudio(0, writeSourceError, func(input struct {
		Path             string `json:"path"`
		Content          string `json:"content"`
		ExpectedRevision string `json:"expectedRevision"`
	}) (any, error) {
		return workspace.SaveSource(input.Path, input.Content, input.ExpectedRevision)
	}))
	routes.HandleFunc("GET /api/form", answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadManagedForm(request.URL.Query().Get("path"))
	}))
	routes.HandleFunc("PUT /api/form", answerStudio(0, writeSourceError, func(input struct {
		Path             string               `json:"path"`
		ExpectedRevision string               `json:"expectedRevision"`
		Form             metadata.ManagedForm `json:"form"`
	}) (any, error) {
		return workspace.SaveManagedForm(input.Path, input.Form, input.ExpectedRevision)
	}))
	routes.HandleFunc("POST /api/form/handler", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path             string `json:"path"`
			ExpectedRevision string `json:"expectedRevision"`
			Command          string `json:"command"`
		}
		if !decodeStudioJSON(response, request, &input, 64<<10) {
			return
		}
		commandID, err := uuid.Parse(input.Command)
		if err != nil {
			http.Error(response, "Invalid form command UUID", http.StatusBadRequest)
			return
		}
		result, err := workspace.EnsureManagedFormHandler(input.Path, input.ExpectedRevision, commandID)
		answerRoute(response, writeSourceError, result, err)
	})
	routes.HandleFunc("GET /api/query/schema", answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.QueryDesignerSchema()
	}))
	routes.HandleFunc("POST /api/query/build", answerStudio(querylang.MaxSourceBytes, writeBadRequest, func(design QueryDesign) (any, error) {
		return workspace.BuildDesignedQuery(design)
	}))
	// The text of a restriction or a template of one is highlighted by the
	// query language's own list of keywords, not by a second one in the page.
	routes.HandleFunc("POST /api/query/highlight", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Text string `json:"text"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		highlights := querylang.HighlightText(input.Text)
		if highlights == nil {
			highlights = []querylang.Highlight{}
		}
		webpage.JSON(response, http.StatusOK, struct {
			Highlights []querylang.Highlight `json:"highlights"`
		}{highlights})
	})
	routes.HandleFunc("POST /api/bsl/analyze", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		relative, language, err := validateEditablePath(input.Path)
		if err != nil || language != "bsl" {
			http.Error(response, "BSL analysis requires a canonical project module path", http.StatusBadRequest)
			return
		}
		analysis, err := AnalyzeBSL(relative, input.Content)
		answerRoute(response, writeBadRequest, analysis, err)
	})
	routes.HandleFunc("POST /api/bsl/complete", answerStudio(0, writeBadRequest, func(input struct {
		Path     string      `json:"path"`
		Content  string      `json:"content"`
		Position BSLPosition `json:"position"`
	}) (any, error) {
		return workspace.CompleteBSL(input.Path, input.Content, input.Position)
	}))
	routes.HandleFunc("POST /api/bsl/navigate", answerStudio(0, writeBSLNavigationError, func(input struct {
		Path     string      `json:"path"`
		Content  string      `json:"content"`
		Position BSLPosition `json:"position"`
		Mode     string      `json:"mode"`
	}) (any, error) {
		return workspace.NavigateBSL(input.Path, input.Content, input.Position, input.Mode)
	}))
	routes.HandleFunc("POST /api/bsl/rename", answerStudio(studioMutationCeiling, writeBSLNavigationError, func(input struct {
		Path             string      `json:"path"`
		Position         BSLPosition `json:"position"`
		NewName          string      `json:"newName"`
		ExpectedRevision string      `json:"expectedRevision"`
	}) (any, error) {
		return workspace.RenameBSL(input.Path, input.Position, input.NewName, input.ExpectedRevision)
	}))
	routes.HandleFunc("GET /api/search", answerQuery(writeBadRequest, func(request *http.Request) (any, error) {
		return workspace.SearchProject(request.URL.Query().Get("query"))
	}))
	routes.HandleFunc("GET /api/bsl/help", answerQuery(writeBadRequest, func(request *http.Request) (any, error) {
		return workspace.SearchBSLHelp(request.URL.Query().Get("query"))
	}))
	routes.HandleFunc("POST /api/bsl/help/resolve", answerStudio(0, writeBSLNavigationError, func(input struct {
		Path     string      `json:"path"`
		Content  string      `json:"content"`
		Position BSLPosition `json:"position"`
	}) (any, error) {
		return workspace.ResolveBSLHelp(input.Path, input.Content, input.Position)
	}))
	routes.HandleFunc("GET /api/git/status", workspace.gitQuery(func(request *http.Request, client *gitclient.Client) (any, error) {
		return client.Status(request.Context())
	}))
	routes.HandleFunc("GET /api/git/diff", workspace.gitQuery(func(request *http.Request, client *gitclient.Client) (any, error) {
		diff, err := client.Diff(request.Context(), request.URL.Query().Get("path"))
		return map[string]string{"diff": diff}, err
	}))
	routes.HandleFunc("GET /api/git/branches", workspace.gitQuery(func(request *http.Request, client *gitclient.Client) (any, error) {
		return client.Branches(request.Context())
	}))
	routes.HandleFunc("POST /api/git/commit", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Message string `json:"message"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		result, err := workspace.git(request, func(request *http.Request, client *gitclient.Client) (any, error) {
			return client.Commit(request.Context(), input.Message)
		})
		answerRoute(response, writeGitError, result, err)
	})
	// Pulling and switching a branch change the modules under the index of
	// BSL, so it is read again.
	routes.HandleFunc("POST /api/git/pull", answerMarked(writeGitError, func(request *http.Request) (any, error) {
		result, err := workspace.git(request, func(request *http.Request, client *gitclient.Client) (any, error) {
			return client.Pull(request.Context())
		})
		if err == nil {
			workspace.invalidateBSLIndex()
		}
		return result, err
	}))
	routes.HandleFunc("POST /api/git/push", answerMarked(writeGitError, func(request *http.Request) (any, error) {
		return workspace.git(request, func(request *http.Request, client *gitclient.Client) (any, error) {
			return client.Push(request.Context())
		})
	}))
	routes.HandleFunc("POST /api/git/branches/switch", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name   string `json:"name"`
			Create bool   `json:"create"`
		}
		if !decodeStudioMutation(response, request, &input) {
			return
		}
		result, err := workspace.git(request, func(request *http.Request, client *gitclient.Client) (any, error) {
			return client.SwitchBranch(request.Context(), input.Name, input.Create)
		})
		if err == nil {
			workspace.invalidateBSLIndex()
		}
		answerRoute(response, writeGitError, result, err)
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
		webpage.JSON(response, http.StatusOK, struct {
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
	return webpage.Secured(routes)
}

func validateStudioMutation(response http.ResponseWriter, request *http.Request) bool {
	if request.Header.Get("X-ML-CSRF") != "1" {
		http.Error(response, "CSRF check failed", http.StatusForbidden)
		return false
	}
	return true
}

// studioMutationCeiling is the body of a small request of Studio: a command,
// a name, a message.
const studioMutationCeiling = 16 << 10

func decodeStudioMutation(response http.ResponseWriter, request *http.Request, value any) bool {
	return decodeStudioJSON(response, request, value, studioMutationCeiling)
}

// answerRoute answers what a route of Studio did: the result as JSON, or the
// error the way the route answers errors. Every route answers through it.
func answerRoute(response http.ResponseWriter, refuse func(http.ResponseWriter, error), result any, err error) {
	if err != nil {
		refuse(response, err)
		return
	}
	webpage.JSON(response, http.StatusOK, result)
}

// answerStudio is a route of Studio that reads a request the one way
// (decodeStudioJSON, with the ceiling of the route), does what it asks and
// answers through answerRoute.
func answerStudio[In any](limit int64, refuse func(http.ResponseWriter, error), do func(In) (any, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input In
		if !decodeStudioJSON(response, request, &input, limit) {
			return
		}
		result, err := do(input)
		answerRoute(response, refuse, result, err)
	}
}

// answerQuery is a route of Studio that reads nothing but its address - a
// path, a query - does what it asks and answers through answerRoute.
func answerQuery(refuse func(http.ResponseWriter, error), do func(*http.Request) (any, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		result, err := do(request)
		answerRoute(response, refuse, result, err)
	}
}

// answerMarked is a route of Studio that changes something and carries no
// body: it acts only on a request with the mark of Studio's page
// (validateStudioMutation) and answers through answerRoute.
func answerMarked(refuse func(http.ResponseWriter, error), do func(*http.Request) (any, error)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		result, err := do(request)
		answerRoute(response, refuse, result, err)
	}
}

// answerRemoval is a route of Studio that deletes what its address names: it
// acts only on a marked request and answers No Content, or the error the way
// the route answers errors.
func answerRemoval(refuse func(http.ResponseWriter, error), remove func(*http.Request) error) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if err := remove(request); err != nil {
			refuse(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}
}

// git opens the project's repository for a request and does one thing with
// it; a project that is no repository is an error of the route like any
// other.
func (workspace *Workspace) git(request *http.Request, do func(*http.Request, *gitclient.Client) (any, error)) (any, error) {
	client, err := gitclient.Open(request.Context(), workspace.root)
	if err != nil {
		return nil, err
	}
	return do(request, client)
}

// gitQuery is a route of Studio that reads the project's repository by its
// address, answered through answerQuery with the errors of git.
func (workspace *Workspace) gitQuery(do func(*http.Request, *gitclient.Client) (any, error)) http.HandlerFunc {
	return answerQuery(writeGitError, func(request *http.Request) (any, error) { return workspace.git(request, do) })
}

// writeBadRequest answers a request whose content the route could not use.
func writeBadRequest(response http.ResponseWriter, err error) {
	http.Error(response, err.Error(), http.StatusBadRequest)
}

// decodeStudioJSONRequest reads a request of Studio with no ceiling of its
// own on the body: a role, a text of a restriction, a module or a form may be
// large.
func decodeStudioJSONRequest(response http.ResponseWriter, request *http.Request, target any) bool {
	return decodeStudioJSON(response, request, target, 0)
}

// decodeStudioJSON reads the one JSON value of a request of Studio, with
// no fields beyond the target's and, when limit is not zero, no more bytes
// than it.
func decodeStudioJSON(response http.ResponseWriter, request *http.Request, value any, limit int64) bool {
	if !validateStudioMutation(response, request) {
		return false
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	body := request.Body
	if limit > 0 {
		body = http.MaxBytesReader(response, body, limit)
	}
	decoder := json.NewDecoder(body)
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
func (workspace *Workspace) metadataTree(loaded *metadata.Catalog, language string, languages []project.Language) ([]Node, error) {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata"))
	if err != nil {
		return nil, fmt.Errorf("read metadata directory: %w", err)
	}
	known := make(map[string]bool, len(project.MetadataKinds()))
	for _, kind := range project.MetadataKinds() {
		known[kind] = true
	}
	// Every such folder is named at once - see ML-STUDIO.md, a folder the
	// platform does not know blocks the work with the project until it is gone.
	var unexpected []string
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !known[entry.Name()] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			unexpected = append(unexpected, fmt.Sprintf("%q", filepath.ToSlash(filepath.Join("metadata", entry.Name()))))
		}
	}
	if len(unexpected) != 0 {
		return nil, fmt.Errorf("unexpected metadata paths: %s", strings.Join(unexpected, ", "))
	}
	// Modules form no branch of their own - each one is reached through the
	// object that owns it - but the directory is still checked here, so a
	// stray file is reported when the tree is read and not only when data is
	// saved into the database.
	if _, err := workspace.sourceFiles(filepath.Join(workspace.root, "modules"), "modules", ".bsl", "modules"); err != nil {
		return nil, err
	}
	built := make(map[string]Node, len(project.MetadataKinds()))
	for _, kind := range project.MetadataKinds() {
		relative := filepath.Join("metadata", kind)
		node := Node{ID: "metadata/" + kind, Kind: "metadata-group", Icon: kindIcon(kind), Title: metadataTitle(kind), Path: filepath.ToSlash(relative)}
		path := filepath.Join(workspace.root, relative)
		switch {
		case kind == "languages":
			// Languages have no per-object UUID or file of their own (they're
			// a plain list inside configuration.yaml), so this branch is
			// synthesized directly from the already in-memory list rather
			// than scanned from disk like every other metadata kind.
			// The group node itself stays inert — matching every other
			// metadata-group node (e.g. "Справочники" shows nothing until
			// you open a specific catalog) — only an individual language
			// leaf carries a Path/Fragment, opening the project editor and
			// switching straight to that one language's own properties.
			for _, item := range languages {
				node.Children = append(node.Children, Node{
					ID: "language:" + item.Code, Kind: "language", Icon: kindIcon(kind), Title: item.Name,
					Path: project.ConfigurationFile, Fragment: "language:" + item.Code,
					Properties: []Property{{Name: "Код", Value: item.Code}, {Name: "Имя", Value: item.Name}},
				})
			}
			sortNodesByTitle(node.Children)
		default:
			if _, err := os.Stat(path); err == nil {
				var buildErr error
				if slices.Contains(project.ObjectFolderKinds(), kind) {
					node.Children, buildErr = workspace.objectFolderNodes(path, filepath.ToSlash(relative), "metadata", kind, loaded)
				} else if named, ok := project.NamedFolderFiles(kind); ok {
					// A kind that keeps only files still keeps them in a folder
					// named after the object: its description, and the modules
					// that run it.
					node.Children, buildErr = namedFolderNodes(path, filepath.ToSlash(relative), kind, named)
					if buildErr == nil {
						buildErr = workspace.attachSubordinateObjects(node.Children, path, filepath.ToSlash(relative), kind, loaded)
					}
				} else {
					node.Children, buildErr = workspace.sourceFiles(path, filepath.ToSlash(relative), ".yaml", "metadata")
				}
				if buildErr != nil {
					return nil, buildErr
				}
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("inspect metadata directory %q: %w", kind, err)
			}
		}
		// A common module keeps its BSL in the shared modules directory, so
		// without this leaf the code would have no place in the tree at all:
		// unlike a catalog, a common module has nothing else to open.
		if kind == "common-modules" {
			workspace.attachCommonModuleSources(&node)
		}
		node.Properties = countProperties(node.Path, len(node.Children)+nestedObjects(node.Children))
		built[kind] = node
	}
	// The branches stand in the order and the places the kinds give them.
	// Нумераторы и последовательности в верхнем уровне не стоят: обе ветви
	// живут в корне «Документов», рядом с самими документами - там же, где
	// создаются их виды. Счётчик у «Документов» уже посчитан и остаётся
	// числом документов: вложенные ветви - не документы.
	common := Node{ID: "metadata/common", Kind: "group", Icon: commonIcon, Title: "Общие", Path: "metadata"}
	branches := []Node{{}}
	var nested []Node
	for _, item := range kinds.Branches() {
		node := built[string(item.Kind)]
		switch item.Layout().Branch {
		case kinds.CommonBranch:
			common.Children = append(common.Children, node)
		case kinds.DocumentsBranch:
			nested = append(nested, node)
		default:
			branches = append(branches, node)
		}
	}
	common.Properties = countProperties(common.Path, countDescendants(common))
	branches[0] = common
	for index := range branches {
		if branches[index].ID == "metadata/"+string(kinds.Document) {
			branches[index].Children = append(nested, branches[index].Children...)
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
			ID: descriptor.Module.String(), Kind: "modules", Icon: moduleIcon, Title: "Модуль", Path: path,
			Properties: []Property{{Name: "UUID", Value: descriptor.Module.String()}, {Name: "Путь", Value: path}},
		})
	}
}

func (workspace *Workspace) sourceFiles(directory, relative, extension, kind string) ([]Node, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read project directory %q: %w", relative, err)
	}
	nodes := make([]Node, 0, len(entries))
	parents := make(map[string]string)
	// The folder a role, a subsystem or a group of commands keeps beside its
	// description is the loader's to check, and is no node of its own.
	folderKind := metadata.Kind(strings.TrimPrefix(relative, "metadata/"))
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if kind == "metadata" && metadata.KeepsIdentifiedFolder(folderKind) && entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			_, err := os.Stat(filepath.Join(directory, entry.Name()+extension))
			if err := metadata.ValidateIdentifiedFolder(directory, folderKind, entry.Name(), err == nil); err != nil {
				return nil, err
			}
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
			head, err := yamlSourceHead(filepath.Join(directory, entry.Name()), path, title)
			if err != nil {
				return nil, err
			}
			title = head.title
			if !head.parent.IsZero() {
				parents[id.String()] = head.parent.String()
			}
		}
		icon := moduleIcon
		if kind == "metadata" {
			icon = kindIcon(string(folderKind))
		}
		node := Node{
			ID: id.String(), Kind: kind, Icon: icon, Title: title, Path: path,
			Properties: []Property{{Name: "UUID", Value: id.String()}, {Name: "Путь", Value: path}, {Name: "Размер", Value: fmt.Sprintf("%d байт", info.Size())}},
		}
		nodes = append(nodes, node)
	}
	return nestUnderParents(nodes, parents), nil
}

// nestUnderParents puts a description that names its parent among the others
// under it, as the prototype's tree shows a subsystem under the one it lies
// in (inventory of block 00, «Подсистема», snapshot 20-vlozhennaya-sklad).
// One whose parent is not there, or whose chain of parents comes back to
// itself, stays at the top, so that the tree still shows it while the loader
// refuses the project. Every level is in the order of titles.
func nestUnderParents(nodes []Node, parents map[string]string) []Node {
	index := make(map[string]int, len(nodes))
	for position, node := range nodes {
		index[node.ID] = position
	}
	nested := func(id string) bool {
		seen := map[string]bool{id: true}
		for current := parents[id]; current != ""; current = parents[current] {
			if _, ok := index[current]; !ok || seen[current] {
				return false
			}
			seen[current] = true
		}
		return parents[id] != ""
	}
	children := make(map[string][]Node)
	var top []Node
	for _, node := range nodes {
		if nested(node.ID) {
			children[parents[node.ID]] = append(children[parents[node.ID]], node)
		} else {
			top = append(top, node)
		}
	}
	var attach func(level []Node) []Node
	attach = func(level []Node) []Node {
		for position := range level {
			if below := children[level[position].ID]; below != nil {
				level[position].Children = attach(below)
			}
		}
		sortNodesByTitle(level)
		return level
	}
	return attach(top)
}

// objectFolderNodes lists one metadata kind's per-object folders (catalogs,
// documents, information/accumulation registers): each node represents one
// object, titled from its own object.yaml, with its data groups (Реквизиты,
// Табличные части, ...), Формы/Команды/Макеты, and module(s) as children —
// everything physically grouped under that one folder.
//
// kind is the kind of the objects; nodeKind is what their nodes are called in
// the tree - "metadata" for an object of the configuration, which is what
// opens its editor, the kind itself for an object inside another.
func (workspace *Workspace) objectFolderNodes(directory, relative, nodeKind, kind string, loaded *metadata.Catalog) ([]Node, error) {
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
		head, err := yamlSourceHead(filepath.Join(objectDirectory, "object.yaml"), descriptionPath, entry.Name())
		if err != nil {
			return nil, err
		}
		title, id := head.title, head.id
		owned, err := objectOwnedFileNodes(objectDirectory, objectRelative, kind, project.SubordinateCollections(kind)...)
		if err != nil {
			return nil, err
		}
		children := objectBranches(loaded, kind)(id, descriptionPath)
		children = append(children, owned.inner...)
		// A group stands where the kind has what it holds, as in the
		// prototype's tree: a sequence has none of them, a criterion of
		// filter no templates, a storage of settings no commands.
		layout := kinds.Must(kinds.Kind(kind)).Layout()
		if layout.Forms {
			children = append(children, Node{ID: id.String() + ":forms", Kind: "group", Icon: formIcon, Title: "Формы", Children: owned.forms})
		}
		// A command shows the module that runs it, because that is the whole
		// of what a command keeps on disk.
		if layout.Commands {
			children = append(children, Node{ID: id.String() + ":commands", Kind: "group", Icon: commandIcon, Title: "Команды", Children: owned.commands})
		}
		// "Макеты" still stands in empty: a template's content has no editor
		// yet.
		if layout.Templates {
			children = append(children, Node{ID: id.String() + ":templates", Kind: "group", Icon: templateIcon, Title: "Макеты"})
		}
		children = append(children, owned.modules...)
		nodes = append(nodes, Node{
			ID: id.String(), Kind: nodeKind, Icon: kindIcon(kind), Title: title, Path: descriptionPath, Children: children,
			Properties: []Property{{Name: "UUID", Value: id.String()}, {Name: "Путь", Value: descriptionPath}, {Name: "Размер", Value: fmt.Sprintf("%d байт", info.Size())}},
		})
	}
	sortNodesByTitle(nodes)
	// An object that keeps objects of its own - a cube its dimension tables -
	// shows them under it the way a source shows its tables.
	if err := workspace.attachSubordinateObjects(nodes, directory, relative, kind, loaded); err != nil {
		return nil, err
	}
	return nodes, nil
}

// objectOwnedNodes are the tree nodes for what one object keeps in its folder,
// kept apart because each goes to a different place in the tree: forms and
// commands under groups of their own, modules as flat leaves beside them -
// which is how the prototype shows an object's own modules too.
type objectOwnedNodes struct {
	modules, forms, commands []Node
	// inner are the groups of what the object describes inside itself and
	// keeps the modules of: the recalculations of a calculation register.
	inner []Node
}

// objectOwnedFileNodes lists one object's own modules, managed forms and
// commands, physically stored alongside its object.yaml.
//
// A module is recognised by its name, because the name is the whole of its
// identity: МодульОбъекта.bsl is the object module of whatever object's folder
// it lies in, and nothing declares it anywhere else.
func objectOwnedFileNodes(objectDirectory, objectRelative, kind string, collections ...string) (objectOwnedNodes, error) {
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
			name := entry.Name()
			if inner, ok := project.InnerKind(kind, name); ok {
				group, err := innerObjectNodes(objectDirectory, objectRelative, inner, name)
				if err != nil {
					return objectOwnedNodes{}, err
				}
				owned.inner = append(owned.inner, group)
				continue
			}
			if !slices.Contains(project.ObjectSubordinateDirectories(kind), name) && !slices.Contains(collections, name) {
				return objectOwnedNodes{}, unexpected(name)
			}
			switch name {
			case "forms":
				owned.forms, err = objectFormNodes(objectDirectory, objectRelative, "forms")
			case "commands":
				owned.commands, err = objectCommandNodes(objectDirectory, objectRelative)
			default:
				// Templates have no editor yet, and a collection of objects
				// inside this one is shown as objects, by
				// attachSubordinateObjects.
				continue
			}
			if err != nil {
				return objectOwnedNodes{}, err
			}
			continue
		}
		if !project.KeepsModule(kind, entry.Name()) {
			return objectOwnedNodes{}, unexpected(entry.Name())
		}
		title := moduleTitle(entry.Name())
		path := filepath.ToSlash(filepath.Join(objectRelative, entry.Name()))
		owned.modules = append(owned.modules, moduleNode(path, title))
	}
	sort.Slice(owned.modules, func(left, right int) bool { return owned.modules[left].Path < owned.modules[right].Path })
	sort.Slice(owned.forms, func(left, right int) bool { return owned.forms[left].Path < owned.forms[right].Path })
	sort.Slice(owned.commands, func(left, right int) bool { return owned.commands[left].Title < owned.commands[right].Title })
	return owned, nil
}

// innerObjectNodes is the group of what an object describes inside itself
// and keeps only the modules of, a folder each: the recalculations of a
// calculation register, each showing its record set module.
func innerObjectNodes(objectDirectory, objectRelative, kind, folder string) (Node, error) {
	relative := filepath.ToSlash(filepath.Join(objectRelative, folder))
	group := Node{ID: relative, Kind: "group", Icon: kindIcon(kind), Title: metadataTitle(kind)}
	entries, err := os.ReadDir(filepath.Join(objectDirectory, folder))
	if err != nil {
		return Node{}, fmt.Errorf("read %s: %w", relative, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || project.SubordinateName(entry.Name()) != nil {
			return Node{}, fmt.Errorf("unexpected source path %q", relative+"/"+entry.Name())
		}
		node := Node{ID: relative + "/" + entry.Name(), Kind: kind, Icon: kindIcon(kind), Title: entry.Name()}
		files, err := os.ReadDir(filepath.Join(objectDirectory, folder, entry.Name()))
		if err != nil {
			return Node{}, fmt.Errorf("read %s: %w", node.ID, err)
		}
		for _, file := range files {
			if file.IsDir() || file.Type()&os.ModeSymlink != 0 || !project.KeepsModule(kind, file.Name()) {
				return Node{}, fmt.Errorf("unexpected source path %q", node.ID+"/"+file.Name())
			}
			path := node.ID + "/" + file.Name()
			node.Children = append(node.Children, moduleNode(path, moduleTitle(file.Name())))
		}
		group.Children = append(group.Children, node)
	}
	return group, nil
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
			ID: path, Kind: "forms", Icon: formIcon, Title: entry.Name(), Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		}
		// The module hangs under its form, because that is where it lies and
		// the only thing that says it exists is that it lies there.
		modulePath := filepath.ToSlash(filepath.Join(parentRelative, forms, entry.Name(), project.FormModuleFile))
		if info, err := os.Lstat(filepath.Join(parentDirectory, forms, entry.Name(), project.FormModuleFile)); err == nil && info.Mode().IsRegular() {
			node.Children = append(node.Children, moduleNode(modulePath, moduleTitle(project.FormModuleFile)))
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// attachSubordinateObjects hangs under each object the collections of objects
// that lie inside it - the tables of an external data source - each object
// shown the way an object of the configuration is, with its branches, forms,
// commands, templates and modules. A collection stands even when it is empty, the way
// a branch of the tree does: an empty branch in its place says there is
// nothing, a missing one says nothing at all.
func (workspace *Workspace) attachSubordinateObjects(owners []Node, directory, relative, kind string, loaded *metadata.Catalog) error {
	collections := project.SubordinateCollections(kind)
	if len(collections) == 0 {
		return nil
	}
	for index := range owners {
		// The folder's name, read off where the description lies: the title
		// of a node may be the object's synonym, and the folder is named after
		// the object's name.
		ownerName := filepath.Base(filepath.Dir(filepath.FromSlash(owners[index].Path)))
		groups := make([]Node, 0, len(collections))
		for _, collection := range collections {
			subordinateKind, _ := project.SubordinateObjectKind(kind, collection)
			group := Node{
				ID: relative + "/" + ownerName + "/" + collection, Kind: "group", Icon: kindIcon(subordinateKind), Title: metadataTitle(subordinateKind),
			}
			collectionDirectory := filepath.Join(directory, ownerName, collection)
			if _, err := os.Stat(collectionDirectory); err == nil {
				children, err := workspace.objectFolderNodes(collectionDirectory, group.ID, subordinateKind, subordinateKind, loaded)
				if err != nil {
					return err
				}
				group.Children = children
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect %s: %w", group.ID, err)
			}
			groups = append(groups, group)
		}
		// What lies inside the object comes first, as in the prototype's
		// tree: a cube shows its dimension tables above its dimensions.
		owners[index].Children = append(groups, owners[index].Children...)
	}
	return nil
}

// moduleTitle reads a module file name as what the module is, so the tree
// says "Модуль объекта" where the file says МодульОбъекта.bsl.
func moduleTitle(file string) string {
	module, _ := kinds.ModuleOfFile(file)
	return module.Title()
}

// The icons of what the tree shows beside the objects of the kinds: the
// objects themselves are drawn with the icon of their kind (kindIcon).
const (
	configurationIcon = "configuration"
	commonIcon        = "common"
	moduleIcon        = "module"
	formIcon          = "form"
	commandIcon       = "command"
	templateIcon      = "template"
	pictureIcon       = "picture"
	attributeIcon     = "attribute"
)

// kindIcon is the icon of a kind, which its descriptor names.
func kindIcon(kind string) string {
	descriptor, _ := kinds.Of(kinds.Kind(kind))
	return descriptor.Icon
}

// moduleNode is the leaf of a module file, which opens it.
func moduleNode(path, title string) Node {
	return Node{ID: path, Kind: "modules", Icon: moduleIcon, Title: title, Path: path, Properties: []Property{{Name: "Путь", Value: path}}}
}

// namedFolderNodes lists the objects of a kind that keeps only files: each in a
// folder named after it, holding its description and the modules that run it.
// The node opens the description; the modules hang beneath.
func namedFolderNodes(directory, relative, kind string, named []string) ([]Node, error) {
	description := project.DescriptionFile(kind)
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
			ID: path, Kind: kind, Icon: kindIcon(kind), Title: entry.Name(), Path: path,
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
			node.Children = append(node.Children, moduleNode(modulePath, moduleTitle(file)))
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
			ID: path, Kind: "commands", Icon: commandIcon, Title: entry.Name(), Path: path,
			Properties: []Property{{Name: "Путь", Value: path}},
		})
	}
	return nodes, nil
}

// objectBranches builds the branches of the tree under an object of the kind
// - Реквизиты, Измерения, Ресурсы, Табличные части and the rest - from what
// the kind's descriptor says an object of it holds (ObjectBranches), every
// branch standing even when empty, as in the prototype's tree. loaded may be
// nil (the project does not load as a whole): the branches then stand empty
// rather than breaking the tree.
//
// A part opens its object's description, its fragment naming the part, which
// the editor of a catalog selects; a field of a table part names the table
// part.
func objectBranches(loaded *metadata.Catalog, kind string) func(id uuid.UUID, descriptionPath string) []Node {
	return func(id uuid.UUID, descriptionPath string) []Node {
		var nodes []Node
		for _, branch := range loaded.ObjectBranches(metadata.Kind(kind), id) {
			node := Node{ID: id.String() + ":" + string(branch.Group), Kind: "group", Icon: branch.Icon, Title: branch.Title, Path: descriptionPath}
			for _, part := range branch.Parts {
				leaf := Node{ID: part.ID.String(), Kind: "attribute", Icon: branch.Icon, Title: part.Name, Path: descriptionPath,
					Fragment: "attribute:" + part.ID.String(), Properties: []Property{{Name: "UUID", Value: part.ID.String()}}}
				if branch.Group == metadata.TablePartsPart {
					leaf.Kind, leaf.Fragment = "tablepart", "tablepart:"+part.ID.String()
					for _, field := range part.Parts {
						leaf.Children = append(leaf.Children, Node{ID: part.ID.String() + ":" + field.ID.String(), Kind: "attribute", Icon: attributeIcon,
							Title: field.Name, Path: descriptionPath, Fragment: leaf.Fragment, Properties: []Property{{Name: "UUID", Value: field.ID.String()}}})
					}
					sortNodesByTitle(leaf.Children)
				}
				node.Children = append(node.Children, leaf)
			}
			sortNodesByTitle(node.Children)
			nodes = append(nodes, node)
		}
		return nodes
	}
}

// sortNodesByTitle orders tree nodes by their display title so that
// user-created objects (catalogs, attributes, languages, ...) always appear
// alphabetically within their group — recomputed on every tree build, not a
// persisted order, so a newly created or renamed object lands in its
// alphabetical place immediately without any separate "sort"/"move" action.
func sortNodesByTitle(nodes []Node) {
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Title < nodes[right].Title })
}

// sourceHead is what the tree needs from an object's description: the name
// it is shown by, the identifier everything else addresses it with, and the
// object of its own kind it stands under, if it names one.
type sourceHead struct {
	title      string
	id, parent uuid.UUID
}

// yamlSourceHead reads the head of a description. The folder is named by the
// name now, so the identifier has to come from inside the file. What does not
// read as a value is left as it is: the tree shows the object by the fallback
// and the loader names the fault.
func yamlSourceHead(filePath, relative, fallback string) (sourceHead, error) {
	head := sourceHead{title: fallback}
	file, err := os.Open(filePath)
	if err != nil {
		return head, fmt.Errorf("open source %q: %w", relative, err)
	}
	defer file.Close()
	var document yaml.Node
	if err := yaml.NewDecoder(file).Decode(&document); err != nil {
		return head, fmt.Errorf("decode source %q: %w", relative, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return head, nil
	}
	// The tree always shows an object's Имя (Name), never its Заголовок
	// (Title) — matching 1C Configurator, where the tree is the developer's
	// technical view and the localized title is only what end users see.
	mapping := document.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if value.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "name":
			if name := strings.TrimSpace(value.Value); name != "" {
				head.title = name
			}
		case "id", "parent":
			if parsed, err := uuid.Parse(strings.TrimSpace(value.Value)); err == nil {
				if key.Value == "id" {
					head.id = parsed
				} else {
					head.parent = parsed
				}
			}
		}
	}
	return head, nil
}

func countProperties(path string, count int) []Property {
	return []Property{{Name: "Путь", Value: path}, {Name: "Объектов", Value: fmt.Sprint(count)}}
}

// nestedObjects counts the objects of the configuration standing under
// others of their branch - a subsystem under the one it lies in - which are as
// many objects of the branch as those at its top. What an object holds -
// its groups, a cube of a source - is no object of the branch.
func nestedObjects(nodes []Node) int {
	count := 0
	for _, node := range nodes {
		if node.Kind != "metadata" {
			continue
		}
		for _, child := range node.Children {
			if child.Kind == "metadata" {
				count += 1 + nestedObjects([]Node{child})
			}
		}
	}
	return count
}

func countDescendants(node Node) int {
	count := 0
	for _, child := range node.Children {
		count += len(child.Children)
	}
	return count
}

// metadataTitle is the caption of a kind's branch of the tree; a kind with
// no descriptor is shown by its folder, never blank.
func metadataTitle(kind string) string {
	if descriptor, ok := kinds.Of(kinds.Kind(kind)); ok {
		return descriptor.Titles
	}
	return kind
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
