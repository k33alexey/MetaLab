package studio

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CatalogEditorSource is the typed, revision-safe representation the
// visual catalog editor reads and writes, in place of raw YAML: the head
// of its window with what its pages show (windowHead), the catalog, and
// what the window of a catalog offers beside them.
type CatalogEditorSource struct {
	windowHead
	Catalog   metadata.CatalogDefinition `json:"catalog"`
	TypeOffer metadata.TypeOffer         `json:"typeOffer"`
	// PartPalettes are the palettes of its parts: an attribute, a table part.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
	// Choices are what the windows «Выбор объекта» of the catalog offer, by
	// the property they choose: owners - the objects of the five kinds that
	// may own it, basedOn - those of the eight that may be its basis.
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
}

func (source CatalogEditorSource) windowOf() (metadata.CatalogDefinition, windowHead) {
	return source.Catalog, source.windowHead
}

func (CatalogEditorSource) named(value metadata.CatalogDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// catalogShape is how the window names a catalog and what its pages know
// of it: its commands, the fields a right is given to, its templates.
var catalogShape = objectShape[metadata.CatalogDefinition]{
	named: CatalogEditorSource{}.named,
	commands: func(value metadata.CatalogDefinition) []string {
		names := make([]string, 0, len(value.Commands))
		for _, command := range value.Commands {
			names = append(names, command.Name)
		}
		return names
	},
	fields: func(value metadata.CatalogDefinition) []string {
		return metadata.DeclaredFieldKeys(value.Attributes, value.TableParts)
	},
	templates: func(value metadata.CatalogDefinition) []metadata.ObjectTemplate { return value.Templates },
}

// NamedPath is a part of an object kept in a file of its own: its name and
// where the file lies.
type NamedPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func validateCatalogEditorPath(relative string) error {
	return validateObjectEditorPath(relative, metadata.CatalogKind)
}

// validateObjectEditorPath accepts the description of an object of a kind
// where its kind keeps it: metadata/<kind>/<Имя>/object.yaml of a kind
// keeping a folder named after the object, metadata/<kind>/<id>.yaml of
// one keeping a file of its own (kinds.InFile).
func validateObjectEditorPath(relative string, kind metadata.Kind) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" {
		return ErrInvalidSourcePath
	}
	parts := strings.Split(canonical, "/")
	if kinds.Must(kind).Layout().Storage == kinds.InFile {
		if len(parts) != 3 || parts[0] != "metadata" || parts[1] != string(kind) {
			return ErrInvalidSourcePath
		}
		return nil
	}
	if len(parts) != 4 || parts[0] != "metadata" || parts[1] != string(kind) || parts[3] != project.ObjectMetadataFile {
		return ErrInvalidSourcePath
	}
	return nil
}

func (workspace *Workspace) ReadCatalogEditor(relative string) (CatalogEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readCatalogEditorLocked(relative)
}

func (workspace *Workspace) readCatalogEditorLocked(relative string) (CatalogEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.CatalogKind, relative, metadata.DecodeCatalog, catalogShape)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	// What the window «Редактирование типа данных» offers for an attribute
	// is a field the database stores.
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	return CatalogEditorSource{windowHead: head, Catalog: value, TypeOffer: offer,
		Choices: map[string][]ObjectChoiceBranch{
			"owners": objectChoiceTree(loaded, metadata.CatalogOwnerKinds()), "basedOn": objectChoiceTree(loaded, metadata.BasisKinds()),
		},
		PartPalettes: map[string][]kinds.PaletteTab{"attribute": kinds.PartPalette("attribute"), "tablePart": kinds.PartPalette("tablePart"),
			"template": kinds.PartPalette("template")},
	}, nil
}

// SaveCatalogEditor validates and writes an edited catalog. Reference
// existence (enumeration/catalog/document ids used by composite attribute
// types) is checked the same way raw YAML edits are today — at the next
// full project Load, not here — so this is not a regression versus the
// plain text editor it replaces.
func (workspace *Workspace) SaveCatalogEditor(relative string, value metadata.CatalogDefinition, revision string) (CatalogEditorSource, error) {
	return workspace.SaveCatalogWindow(relative, CatalogWindowValue{CatalogDefinition: value}, revision)
}

// CatalogWindowValue is what the window of a catalog writes: the catalog,
// the subsystems marked on its page «Подсистемы», the options marked on
// «Функциональные опции», the exchange plans marked on «Обмен данными»
// with their autoregistration and the objects listed «Является основанием
// для» on «Ввод на основании», and what the roles say about it on «Права» -
// none where a page is not given, which leaves those objects as they are.
type CatalogWindowValue struct {
	metadata.CatalogDefinition
	// TemplateContents are the contents loaded from files into templates
	// the constructor added in this window, by their identifiers.
	TemplateContents  []TemplateContent `json:"templateContents,omitempty"`
	Subsystems        *[]uuid.UUID      `json:"subsystems,omitempty"`
	FunctionalOptions *[]uuid.UUID      `json:"functionalOptions,omitempty"`
	ExchangePlans     *[]MemberPlace    `json:"exchangePlans,omitempty"`
	BasisFor          *[]uuid.UUID      `json:"basisFor,omitempty"`
	Roles             *[]RoleMark       `json:"roles,omitempty"`
}

// SaveCatalogWindow writes the catalog and what the pages of its window
// mark - the roles, the subsystems, the options, the exchange plans, the
// objects entered on its basis, the contents of new templates - as one
// save (saveObjectWindow).
func (workspace *Workspace) SaveCatalogWindow(relative string, window CatalogWindowValue, revision string) (CatalogEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.CatalogDefinition, windowMarks{
		Subsystems: window.Subsystems, FunctionalOptions: window.FunctionalOptions, ExchangePlans: window.ExchangePlans,
		BasisFor: window.BasisFor, Roles: window.Roles, TemplateContents: window.TemplateContents, Kind: metadata.CatalogKind,
	}, workspace.readCatalogEditorLocked, catalogShape)
}

// writeObjectWindowLocked writes what the window of an object marks on its
// pages - the subsystems, the options, the plans that list it - then the
// files it adds to its folder, then the object itself, its folder following
// a new name (before, after), as one save: a refusal of any step puts back
// what was written before it. Answers where the description lies and as
// what it was written.
func (workspace *Workspace) writeObjectWindowLocked(relative, before, after, encoded, revision string, pages []membership, files []newFile) (string, SourceFile, error) {
	restoreMembership, err := workspace.writeMembershipLocked(pages)
	if err != nil {
		return "", SourceFile{}, fmt.Errorf("write the objects listing the object: %w", err)
	}
	// The object keeps a folder named after it, so a new name moves the
	// folder - its forms and modules with it - and the description is
	// written there; a refusal puts the folder back.
	written, moveBack, err := workspace.followNameLocked(relative, before, after)
	if err != nil {
		return "", SourceFile{}, errors.Join(err, restoreMembership())
	}
	// The files go into the folder before the description declares them,
	// and away again if it is refused.
	takeFiles, err := workspace.writeNewFilesLocked(files)
	if err != nil {
		return "", SourceFile{}, errors.Join(err, moveBack(), restoreMembership())
	}
	saved, err := workspace.saveSourceLocked(written, encoded, revision)
	if err != nil {
		takeFiles()
		return "", SourceFile{}, errors.Join(err, moveBack(), restoreMembership())
	}
	return written, saved, nil
}

// followNameLocked moves the folder of the object described at relative from
// its name before to its name after, and answers where its description lies
// then and how to move it back. An object of no folder of its own, and an
// unchanged name, stay where they are.
func (workspace *Workspace) followNameLocked(relative, before, after string) (string, func() error, error) {
	stay := func() error { return nil }
	kind, name, rest, ok := project.SplitObjectPath(relative)
	if !ok || before == after || name != before {
		return relative, stay, nil
	}
	moved, err := project.ObjectMetadataPath(kind, after)
	if err != nil || len(rest) != 1 {
		return relative, stay, err
	}
	from := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	to := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(moved)))
	if err := renameFolder(from, to); err != nil {
		return relative, stay, err
	}
	workspace.invalidateStudioIndexesLocked()
	return moved, func() error {
		workspace.invalidateStudioIndexesLocked()
		return renameFolder(to, from)
	}, nil
}

// CreateCatalog makes the next catalog by the one way every object is made
// (CreateObject) and opens it as any catalog is read.
func (workspace *Workspace) CreateCatalog(name string) (CatalogEditorSource, error) {
	return createObjectAs(workspace, metadata.CatalogKind, name, nil, workspace.readCatalogEditorLocked)
}

// DeleteCatalog permanently removes one catalog's whole per-object folder
// (its object.yaml plus any nested modules/forms), after confirming the
// caller still has the latest revision. Reference existence (other objects
// pointing at this catalog) is not checked here — same as SaveCatalogEditor,
// that is caught at the next full project Load, not at delete time.
func (workspace *Workspace) DeleteCatalog(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CatalogKind, relative, expectedRevision, workspace.readCatalogEditorLocked)
}

// windowSource is what a window of an object reads: the object's
// identifier and the revision it was read at.
type windowSource interface {
	identity() (uuid.UUID, string)
}

func (source CatalogEditorSource) identity() (uuid.UUID, string) {
	return source.Catalog.ID, source.Revision
}
func (source ConstantEditorSource) identity() (uuid.UUID, string) {
	return source.Constant.ID, source.Revision
}
func (source PictureEditorSource) identity() (uuid.UUID, string) {
	return source.Picture.ID, source.Revision
}
func (source TemplateEditorSource) identity() (uuid.UUID, string) {
	return source.Template.ID, source.Revision
}

// deleteObjectFolder deletes an object - the folder named after it with its
// description, modules, forms, content, or the one file of an object of a
// kind keeping no folder (kinds.InFile) - by the one order of steps of
// every deletion (deleteObjectLocked), while the caller holds the revision
// read last. read reads the object as its window does.
func deleteObjectFolder[Window windowSource](workspace *Workspace, kind metadata.Kind, relative, expectedRevision string, read func(string) (Window, error)) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := read(relative)
	if err != nil {
		return err
	}
	id, revision := opened.identity()
	if revision != expectedRevision {
		return ErrSourceChanged
	}
	return workspace.deleteObjectLocked(kind, id, relative, func() error {
		if kinds.Must(kind).Layout().Storage == kinds.InFile {
			return os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		}
		return os.RemoveAll(filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative))))
	})
}

// ownerRefusals are the prototype's words at «ОК» of the window choosing the
// owners of a catalog (docs/block-00/spravochnik.md, «Проверки»; snapshots
// vladelcy-otkaz-*): the one of a catalog with predefined items, and the one
// of an owner that is the catalog itself or closes a ring, naming the owner.
var ownerRefusals = map[metadata.OwnerFault]string{
	metadata.OwnerOfPredefined: "Справочнику, имеющему предопределенные данные, нельзя назначить владельца!",
	metadata.OwnerItself:       "Данный справочник не может быть установлен в качестве владельца! (%s)",
	metadata.OwnerInRing:       "Данный справочник не может быть установлен в качестве владельца! (%s)",
}

// CatalogOwnersRefusal answers the prototype's refusal of the owners the
// window of the catalog at relative holds - catalog, unsaved - against the
// owners of every other catalog of the project, or nothing.
func (workspace *Workspace) CatalogOwnersRefusal(relative string, catalog metadata.CatalogDefinition) (NameAnswer, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if err := validateCatalogEditorPath(relative); err != nil {
		return NameAnswer{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return NameAnswer{}, err
	}
	fault, owner := loaded.OwnersFault(catalog)
	if fault == metadata.OwnerFits {
		return NameAnswer{}, nil
	}
	if strings.Contains(ownerRefusals[fault], "%s") {
		return NameAnswer{Refusal: fmt.Sprintf(ownerRefusals[fault], owner)}, nil
	}
	return NameAnswer{Refusal: ownerRefusals[fault]}, nil
}

// catalogEditorAddress is where the window of a catalog is read, written,
// made and deleted.
const catalogEditorAddress = "/api/catalog"

func registerCatalogEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, catalogEditorAddress, workspace.ReadCatalogEditor, workspace.SaveCatalogWindow, workspace.CreateCatalog, workspace.DeleteCatalog)
	routes.HandleFunc("POST "+catalogEditorAddress+"/owners", answerStudio(0, writeSourceError, func(input struct {
		Path   string                     `json:"path"`
		Object metadata.CatalogDefinition `json:"object"`
	}) (any, error) {
		return workspace.CatalogOwnersRefusal(input.Path, input.Object)
	}))
}
