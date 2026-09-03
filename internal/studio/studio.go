// Package studio implements the local ML Studio workspace and HTTP surface.
package studio

import (
	"bytes"
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

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

//go:embed ui/index.html
var assets embed.FS

// Workspace is one validated project opened by a Studio process.
type Workspace struct {
	root string
	mu   sync.Mutex
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
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve ML Project path: %w", err)
	}
	if _, err := project.ValidateLayout(absolute); err != nil {
		return nil, err
	}
	return &Workspace{root: filepath.Clean(absolute)}, nil
}

// Snapshot scans current project sources, including edits made outside Studio.
func (workspace *Workspace) Snapshot() (Snapshot, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
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
			node, err = workspace.metadataTree()
		} else {
			node, err = workspace.sourceTree(directory)
		}
		if err != nil {
			return Snapshot{}, err
		}
		root.Children = append(root.Children, node)
	}
	return Snapshot{ProjectPath: workspace.root, Manifest: manifest, Tree: root}, nil
}

// NewHandler serves the local read-only Studio shell for one workspace.
func NewHandler(workspace *Workspace) http.Handler {
	routes := http.NewServeMux()
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
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, MaxEditableFileBytes+(64<<10)))
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
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Frame-Options", "DENY")
		routes.ServeHTTP(response, request)
	})
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

func (workspace *Workspace) metadataTree() (Node, error) {
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
			node.Children, err = workspace.sourceFiles(path, filepath.ToSlash(relative), ".yaml", "metadata")
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

func (workspace *Workspace) sourceTree(directory string) (Node, error) {
	extension, kind := "", directory
	switch directory {
	case "modules", "tests":
		extension = ".bsl"
	case "forms", "reports":
		extension = ".yaml"
	case "assets":
		kind = "asset"
	}
	children, err := workspace.sourceFiles(filepath.Join(workspace.root, directory), directory, extension, kind)
	if err != nil {
		return Node{}, err
	}
	return Node{
		ID: directory, Kind: "group", Title: rootTitles[directory], Path: directory,
		Properties: countProperties(directory, len(children)), Children: children,
	}, nil
}

func (workspace *Workspace) sourceFiles(directory, relative, extension, kind string) ([]Node, error) {
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
			title, err = yamlSourceTitle(filepath.Join(directory, entry.Name()), path, title, info.Size())
			if err != nil {
				return nil, err
			}
		}
		nodes = append(nodes, Node{
			ID: id.String(), Kind: kind, Title: title, Path: path,
			Properties: []Property{{Name: "UUID", Value: id.String()}, {Name: "Путь", Value: path}, {Name: "Размер", Value: fmt.Sprintf("%d байт", info.Size())}},
		})
	}
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Path < nodes[right].Path })
	return nodes, nil
}

func yamlSourceTitle(filePath, relative, fallback string, size int64) (string, error) {
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
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if value.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "name":
			name = strings.TrimSpace(value.Value)
		case "title":
			title = strings.TrimSpace(value.Value)
		}
	}
	if title != "" {
		return title, nil
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
