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
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readWSReferenceEditorLocked(relative)
	if err != nil {
		return WSReferenceEditorSource{}, err
	}
	if opened.Revision != revision {
		return WSReferenceEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Reference.ID {
		return WSReferenceEditorSource{}, fmt.Errorf("WS reference UUID cannot be changed")
	}
	if value.Name != opened.Reference.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return WSReferenceEditorSource{}, err
		}
	}
	value.LocationURL = opened.Reference.LocationURL
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return WSReferenceEditorSource{}, err
	}
	written, moveBack, err := workspace.followNameLocked(relative, opened.Reference.Name, value.Name)
	if err != nil {
		return WSReferenceEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(written, encoded.String(), revision); err != nil {
		return WSReferenceEditorSource{}, errors.Join(err, moveBack())
	}
	return workspace.readWSReferenceEditorLocked(written)
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
	// The description of the service first, put back if the description of
	// the reference is then refused.
	definition := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(request.Path)), metadata.WSReferenceDefinitionFile)
	before, err := os.ReadFile(definition)
	had := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return WSReferenceEditorSource{}, err
	}
	if err := atomicfile.Write(definition, ".ml-write-*", 0o644, atomicfile.Bytes(content)); err != nil {
		return WSReferenceEditorSource{}, err
	}
	workspace.invalidateStudioIndexesLocked()
	if _, err := workspace.saveSourceLocked(request.Path, encoded.String(), opened.Revision); err != nil {
		putBack := os.Remove(definition)
		if had {
			putBack = atomicfile.Write(definition, ".ml-write-*", 0o644, atomicfile.Bytes(before))
		}
		workspace.invalidateStudioIndexesLocked()
		return WSReferenceEditorSource{}, errors.Join(err, putBack)
	}
	return workspace.readWSReferenceEditorLocked(request.Path)
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
	routes.HandleFunc("GET "+wsReferenceEditorAddress, answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadWSReferenceEditor(request.URL.Query().Get("path"))
	}))
	routes.HandleFunc("POST "+wsImportAddress, func(response http.ResponseWriter, request *http.Request) {
		answerStudio(0, writeSourceError, func(input WSImport) (any, error) {
			return workspace.ImportWSReference(request.Context(), input)
		})(response, request)
	})
	// A reference is made by importing a description: «Добавить» of the
	// tree imports at its own address, and a reference named alone is not
	// made.
	registerObjectWindow(routes, wsReferenceEditorAddress, workspace.SaveWSReferenceEditor, func(string) (WSReferenceEditorSource, error) {
		return WSReferenceEditorSource{}, fmt.Errorf("a WS reference is made by importing a description of a service")
	}, workspace.DeleteWSReference)
}
