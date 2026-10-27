package studio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A WS reference is made and changed by importing a description of a
// service (docs/block-00/ws-ssylka.md): «Добавить» asks «Введите URL
// импортируемого WSDL» - an address or a path to a file - and makes nothing
// when the description is refused; «Импорт WSDL…» asks again with the
// address before and leaves the reference as it was when refused. Its palette
// shows the address as a link nobody types into; the window viewing the
// description is an item of its own.

// WSReferenceEditorSource is a WS reference as its palette reads and writes
// it.
type WSReferenceEditorSource struct {
	objectHead
	Reference metadata.WSReferenceDefinition `json:"reference"`
}

func (source WSReferenceEditorSource) paletteOf() (metadata.WSReferenceDefinition, objectHead) {
	return source.Reference, source.objectHead
}
func (WSReferenceEditorSource) named(value metadata.WSReferenceDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source WSReferenceEditorSource) identity() (uuid.UUID, string) {
	return source.Reference.ID, source.Revision
}

// WSImport is an import of a description: of a new reference, when Path is
// empty, or of the reference described at Path, read at Revision.
type WSImport struct {
	Path     string `json:"path,omitempty"`
	Revision string `json:"revision,omitempty"`
	Address  string `json:"address"`
}

// The refusals of an import, in the prototype's words (ws-ssylka.md,
// «Добавление»; snapshots 02, 12, 14, 16). The prototype adds to the first
// the text of its internal exception; Studio names the address instead.
var (
	errWSFileMissing    = errors.New("Файл не обнаружен")
	errWSNotWSDL        = errors.New("Ошибка разбора WSDL: неверный формат WSDL описания")
	errWSNotDefinitions = errors.New("WSDL должен содержать элемент definitions")
)

// wsDefinitionRefusals are the refusals of a description by its fault.
var wsDefinitionRefusals = map[metadata.WSDefinitionFault]error{
	metadata.WSDefinitionNotXML:         errWSNotWSDL,
	metadata.WSDefinitionNotDefinitions: errWSNotDefinitions,
}

// wsReferenceEditorAddress is where the palette of a WS reference is read
// and written, wsImportAddress where a description is imported.
const (
	wsReferenceEditorAddress = "/api/ws-reference"
	wsImportAddress          = "/api/ws-reference/import"
)

// wsFetchTimeout is how long an import waits for the description at an
// address of the network.
const wsFetchTimeout = 30 * time.Second

func (workspace *Workspace) ReadWSReferenceEditor(relative string) (WSReferenceEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readWSReferenceEditorLocked(relative)
}

func (workspace *Workspace) readWSReferenceEditorLocked(relative string) (WSReferenceEditorSource, error) {
	value, head, err := readObjectDescription(workspace, metadata.WSReferenceKind, relative, metadata.DecodeWSReference)
	return WSReferenceEditorSource{objectHead: head, Reference: value}, err
}

// SaveWSReferenceEditor writes the name, the synonym and the comment of a
// reference; the address stays as the import wrote it, and a new name moves
// the folder with the description (followNameLocked).
func (workspace *Workspace) SaveWSReferenceEditor(relative string, value metadata.WSReferenceDefinition, revision string) (WSReferenceEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, value, workspace.readWSReferenceEditorLocked, func(opened metadata.WSReferenceDefinition, value *metadata.WSReferenceDefinition) error {
		value.LocationURL = opened.LocationURL
		return nil
	})
}

// ImportWSReference takes the description at the address, checks it as the
// prototype does, and makes a reference of it - «WSСсылка1» and on - or puts
// it in place of the description of the reference the import names, with
// the address. A refusal makes nothing and changes nothing.
func (workspace *Workspace) ImportWSReference(ctx context.Context, request WSImport) (WSReferenceEditorSource, error) {
	content, location, err := fetchWSDefinition(ctx, request.Address)
	if err != nil {
		return WSReferenceEditorSource{}, err
	}
	if fault, _ := metadata.WSDefinitionFaultOf(bytes.NewReader(content)); fault != metadata.WSDefinitionFits {
		return WSReferenceEditorSource{}, wsDefinitionRefusals[fault]
	}
	content, schemas, err := fetchWSSchemas(ctx, content, location)
	if err != nil {
		return WSReferenceEditorSource{}, err
	}
	if request.Path == "" {
		return createObjectAs(workspace, metadata.WSReferenceKind, "", func(fresh *metadata.Fresh) error {
			reference, _ := fresh.Value.(metadata.WSReferenceDefinition)
			reference.LocationURL = location
			fresh.Value = reference
			directory, err := project.ObjectDirectory(string(metadata.WSReferenceKind), reference.Name)
			if err != nil {
				return err
			}
			fresh.Files = append(fresh.Files, metadata.FreshFile{Path: path.Join(directory, metadata.WSReferenceDefinitionFile), Content: content})
			for _, schema := range schemas {
				fresh.Files = append(fresh.Files, metadata.FreshFile{Path: path.Join(directory, schema.name), Content: schema.content})
			}
			return nil
		}, workspace.readWSReferenceEditorLocked)
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readWSReferenceEditorLocked(request.Path)
	if err != nil {
		return WSReferenceEditorSource{}, err
	}
	if opened.Revision != request.Revision {
		return WSReferenceEditorSource{}, ErrSourceChanged
	}
	value := opened.Reference
	value.LocationURL = location
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return WSReferenceEditorSource{}, err
	}
	// The description of the service and its schemas first - the schemas of
	// the import before taken away - all put back if the description of the
	// reference is then refused.
	directory := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(request.Path)))
	files := map[string][]byte{metadata.WSReferenceDefinitionFile: content}
	for _, schema := range schemas {
		files[schema.name] = schema.content
	}
	putBack, err := replaceWSFiles(directory, files)
	workspace.invalidateStudioIndexesLocked()
	if err != nil {
		return WSReferenceEditorSource{}, errors.Join(err, putBack())
	}
	if _, err := workspace.saveSourceLocked(request.Path, encoded.String(), opened.Revision); err != nil {
		err = errors.Join(err, putBack())
		workspace.invalidateStudioIndexesLocked()
		return WSReferenceEditorSource{}, err
	}
	return workspace.readWSReferenceEditorLocked(request.Path)
}

// wsSchema is a schema a description imports, as the reference keeps it
// beside the description: «1.xsd» and on.
type wsSchema struct {
	name    string
	content []byte
}

// wsImportedSchema finds where a description names a schema it imports or
// includes: the attribute schemaLocation, its value.
var wsImportedSchema = regexp.MustCompile(`(\sschemaLocation\s*=\s*["'])([^"']+)(["'])`)

// fetchWSSchemas takes the schemas the description imports - each
// schemaLocation of it, by the address of the description where it is
// relative - and keeps them as the prototype does (ws-ssylka.md,
// «Выгрузки»): «1.xsd», «2.xsd» in the order the description names them,
// the description naming each «file://1.xsd». A schema named twice is taken
// once. Answers the description so changed and the schemas; a schema that
// cannot be read refuses the import with its address, as a description
// does. The schemas a schema imports in turn are not taken (series 2.603,
// question В7).
func fetchWSSchemas(ctx context.Context, content []byte, location string) ([]byte, []wsSchema, error) {
	base, err := url.Parse(location)
	if err != nil {
		return nil, nil, err
	}
	var schemas []wsSchema
	numbers := map[string]string{}
	var failure error
	changed := wsImportedSchema.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := wsImportedSchema.FindSubmatch(match)
		written := string(parts[2])
		name, ok := numbers[written]
		if !ok && failure == nil {
			address := written
			if kept, found := strings.CutPrefix(written, "file://"); found && metadata.WSReferenceSchemaFile(kept) {
				address = kept
			}
			if reference, err := url.Parse(address); err == nil {
				address = base.ResolveReference(reference).String()
			}
			schema, _, err := fetchWSDefinition(ctx, address)
			if err != nil {
				failure = err
				return match
			}
			name = strconv.Itoa(len(schemas)+1) + ".xsd"
			numbers[written] = name
			schemas = append(schemas, wsSchema{name: name, content: schema})
		}
		return []byte(string(parts[1]) + "file://" + name + string(parts[3]))
	})
	if failure != nil {
		return nil, nil, failure
	}
	return changed, schemas, nil
}

// replaceWSFiles writes the files of an import into the folder of a
// reference and takes away the schemas of the import before; answers how to
// put the folder back as it was.
func replaceWSFiles(directory string, files map[string][]byte) (func() error, error) {
	before := map[string][]byte{}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return func() error { return nil }, err
	}
	for _, entry := range entries {
		if entry.Name() == metadata.WSReferenceDefinitionFile || metadata.WSReferenceSchemaFile(entry.Name()) {
			content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				return func() error { return nil }, err
			}
			before[entry.Name()] = content
		}
	}
	putBack := func() error {
		var errs []error
		for name := range files {
			if _, had := before[name]; !had {
				if err := os.Remove(filepath.Join(directory, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
					errs = append(errs, err)
				}
			}
		}
		for name, content := range before {
			errs = append(errs, atomicfile.Write(filepath.Join(directory, name), ".ml-write-*", 0o644, atomicfile.Bytes(content)))
		}
		return errors.Join(errs...)
	}
	for name, content := range files {
		if err := atomicfile.Write(filepath.Join(directory, name), ".ml-write-*", 0o644, atomicfile.Bytes(content)); err != nil {
			return putBack, err
		}
	}
	for name := range before {
		if _, kept := files[name]; !kept {
			if err := os.Remove(filepath.Join(directory, name)); err != nil {
				return putBack, err
			}
		}
	}
	return putBack, nil
}

// fetchWSDefinition reads the description at an address and answers it with
// the address the reference keeps: a page of the network as it is, a file -
// by a path or a file:// address - as file:///… of its full path, as the
// prototype keeps it (snapshot ws-ssylka/03).
func fetchWSDefinition(ctx context.Context, address string) ([]byte, string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, "", errWSFileMissing
	}
	if parsed, err := url.Parse(address); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		ctx, cancel := context.WithTimeout(ctx, wsFetchTimeout)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s", errWSFileMissing, address)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s: %v", errWSFileMissing, address, err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("%w: %s: %s", errWSFileMissing, address, response.Status)
		}
		content, err := io.ReadAll(response.Body)
		return content, address, err
	}
	file := address
	if parsed, err := url.Parse(address); err == nil && parsed.Scheme == "file" {
		file = parsed.Path
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s", errWSFileMissing, address)
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s", errWSFileMissing, address)
	}
	return content, (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String(), nil
}

// DeleteWSReference removes a WS reference's folder - its description and
// the service's - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteWSReference(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.WSReferenceKind, relative, expectedRevision, workspace.readWSReferenceEditorLocked)
}

func registerWSReferenceEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST "+wsImportAddress, func(response http.ResponseWriter, request *http.Request) {
		answerStudio(0, writeSourceError, func(input WSImport) (any, error) {
			return workspace.ImportWSReference(request.Context(), input)
		})(response, request)
	})
	// A reference is made by importing a description: «Добавить» of the
	// tree imports at its own address, and a reference named alone is not
	// made.
	registerObjectEditor(routes, wsReferenceEditorAddress, workspace.ReadWSReferenceEditor, workspace.SaveWSReferenceEditor, func(string) (WSReferenceEditorSource, error) {
		return WSReferenceEditorSource{}, fmt.Errorf("a WS reference is made by importing a description of a service")
	}, workspace.DeleteWSReference)
}
