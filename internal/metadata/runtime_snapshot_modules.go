package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
)

// LoadProjectModules reads every project BSL module from disk and names each
// one canonically, ready to be persisted into a RuntimeSnapshot. This is the
// only place ML Project files are read for BSL purposes - ML Service itself
// never touches disk, it compiles from what "Сохранить данные" stores here.
func LoadProjectModules(root string, catalog *Catalog) ([]RuntimeModule, error) {
	namer := catalog.ModuleNamer()
	var relativePaths []string
	entries, err := os.ReadDir(filepath.Join(root, "modules"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read BSL modules: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".bsl" {
			continue
		}
		relativePaths = append(relativePaths, filepath.ToSlash(filepath.Join("modules", entry.Name())))
	}
	for _, module := range project.RootModuleFiles() {
		if _, err := os.Stat(filepath.Join(root, module)); err == nil {
			relativePaths = append(relativePaths, module)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", module, err)
		}
	}
	objectPaths, err := project.ObjectFolderSourcePaths(root)
	if err != nil {
		return nil, err
	}
	for _, relative := range objectPaths {
		if filepath.Ext(relative) == ".bsl" {
			relativePaths = append(relativePaths, relative)
		}
	}
	// Neither the number of modules nor their size is bounded: erp has 10 962
	// modules and 425 MB of them, and a project the loader takes must publish.
	modules := make([]RuntimeModule, 0, len(relativePaths))
	for _, relative := range relativePaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", relative, err)
		}
		identity := namer.Identify(relative)
		modules = append(modules, RuntimeModule{
			Name: identity.Name, Filename: relative, Source: string(content),
			PredefinedVariables: identity.Predefined, DefaultContext: identity.DefaultContext,
		})
	}
	return modules, nil
}

// ModuleIdentity is what one module is compiled and reported as.
type ModuleIdentity struct {
	Name string
	// Predefined are the words its code finds defined without declaring
	// them, each in both languages: ЭтотОбъект, ThisObject.
	Predefined     []string
	DefaultContext syntax.ExecutionContext
	// Public is a common module: its exported routines are called by its
	// name from any other module.
	Public bool
}

// ModuleNamer names the modules of one configuration by where they lie.
//
// The path, because that is the whole of a module's identity: a module is the
// object module of a catalog by lying in that catalog's folder under the name
// of that role. Nothing declares it, so nothing can disagree with it - and the
// kinds package says of every kind which roles it has and what its modules
// find defined, so one reading of the path names a module of any kind, of a
// form, of a command and of an object inside another alike. Only a common
// module lies by its identifier and is named out of the catalog.
type ModuleNamer struct {
	common map[string]CommonModuleDefinition
}

// ModuleNamer is the namer of the catalog's modules; a nil catalog names
// every module but a common one.
func (catalog *Catalog) ModuleNamer() ModuleNamer {
	namer := ModuleNamer{common: map[string]CommonModuleDefinition{}}
	if catalog == nil {
		return namer
	}
	for _, item := range catalog.CommonModules {
		if path, err := project.ModulePath(item.Module); err == nil {
			namer.common[path] = item
		}
	}
	return namer
}

// Identify names the module lying at the path. A module the configuration
// does not claim gets a name built out of the path (FallbackModuleName).
func (namer ModuleNamer) Identify(relative string) ModuleIdentity {
	switch relative {
	case project.SessionModuleFile:
		return ModuleIdentity{Name: SessionModuleName, DefaultContext: syntax.ContextServer}
	case project.ApplicationModuleFile:
		// The application module runs on the client: it is where the
		// application starts and stops, and both happen where the person is
		// sitting.
		return ModuleIdentity{Name: ApplicationModuleName, DefaultContext: syntax.ContextClient}
	case project.ExternalConnectionModuleFile:
		// An external connection has no client at all: whatever runs there
		// runs on the server, because there is nowhere else.
		return ModuleIdentity{Name: ExternalConnectionModuleName, DefaultContext: syntax.ContextServer}
	}
	if item, ok := namer.common[relative]; ok {
		return ModuleIdentity{Name: item.Name, Predefined: moduleWords(CommonModuleKind, ""),
			DefaultContext: item.DefaultContext(), Public: true}
	}
	address, ok := project.ReadModulePath(relative)
	if !ok {
		return ModuleIdentity{Name: FallbackModuleName(relative)}
	}
	kind := Kind(address.Kind)
	result := ModuleIdentity{Name: ObjectModuleName(kind, address.Object, string(address.Module)), Predefined: moduleWords(kind, address.Module)}
	switch {
	case address.Form != "":
		result.Name = FormModuleName(kind, address.Object, address.Form)
	case address.Command != "":
		// A command is named with its object for the reason a form is: two
		// objects may well each keep a command called Печать.
		result.Name = ObjectModuleName(kind, address.Object+"."+address.Command, string(address.Module))
	}
	return result
}

// moduleWords are the words a module of an object of the kind in the role
// finds defined, each in both languages, as the compiler takes them.
func moduleWords(kind Kind, module kinds.Module) []string {
	var result []string
	for _, words := range kinds.Must(kind).ModuleWords(module) {
		result = append(result, words.Russian, words.English)
	}
	return result
}

// ObjectModuleName is the name a module of an object is compiled and reported
// under: the module's role, the kind of the object and the object -
// МодульОбъектаСправочника.Банки.
func ObjectModuleName(kind Kind, object, role string) string {
	return strings.TrimSuffix(role, ".bsl") + kinds.Must(kind).Genitive + "." + object
}

// FormModuleName is the name one form's module is compiled and reported
// under. It names the object and the form, because that pair is what the form
// is: two objects may well each keep a ФормаСписка.
func FormModuleName(objectKind Kind, object, form string) string {
	return ObjectModuleName(objectKind, object+"."+form, "МодульФормы")
}

// FallbackModuleName names a BSL file the configuration does not claim - a
// test, or a module of a role its kind does not have. A module keeps no
// identifier of its own, so the name is built out of where the file lies,
// which is unique by construction and readable in a stack trace - where the
// previous fallback, a UUID with its dashes removed, was neither.
func FallbackModuleName(relative string) string {
	trimmed := strings.TrimSuffix(filepath.ToSlash(relative), ".bsl")
	prefix := "Модуль."
	if strings.HasPrefix(trimmed, "tests/") {
		prefix = "Тест."
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "metadata/"), "tests/")
	return prefix + strings.ReplaceAll(trimmed, "/", ".")
}
