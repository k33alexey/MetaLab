package metadata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	maxProjectModules = 10_000
	maxProjectSource  = 16 << 20
)

type moduleNameDescriptor struct {
	name           string
	predefined     []string
	defaultContext syntax.ExecutionContext
}

// LoadProjectModules reads every project BSL module from disk and names each
// one canonically, ready to be persisted into a RuntimeSnapshot. This is the
// only place ML Project files are read for BSL purposes - ML Service itself
// never touches disk, it compiles from what "Сохранить данные" stores here.
func LoadProjectModules(root string, catalog *Catalog) ([]RuntimeModule, error) {
	descriptors := moduleNameDescriptors(catalog)
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
	if _, err := os.Stat(filepath.Join(root, project.SessionModuleFile)); err == nil {
		relativePaths = append(relativePaths, project.SessionModuleFile)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read session module: %w", err)
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
	modules := make([]RuntimeModule, 0, len(relativePaths))
	sourceBytes := 0
	for _, relative := range relativePaths {
		if len(modules) >= maxProjectModules {
			return nil, fmt.Errorf("project supports at most %d BSL modules", maxProjectModules)
		}
		content, err := readBoundedModuleFile(filepath.Join(root, filepath.FromSlash(relative)), maxProjectSource-sourceBytes)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", relative, err)
		}
		sourceBytes += len(content)
		descriptor := descriptors[relative]
		if relative == project.SessionModuleFile {
			descriptor = moduleNameDescriptor{name: SessionModuleName, defaultContext: syntax.ContextServer}
		}
		if descriptor.name == "" {
			descriptor.name = moduleNameFromPath(relative)
		}
		modules = append(modules, RuntimeModule{
			Name: descriptor.name, Filename: relative, Source: string(content),
			PredefinedVariables: append([]string(nil), descriptor.predefined...),
			DefaultContext:      descriptor.defaultContext,
		})
	}
	return modules, nil
}

// moduleNameDescriptors maps the path of every module the configuration knows
// to the name it is compiled and reported under.
//
// The path, because that is now the whole of a module's identity: a module is
// the object module of a catalog by lying in that catalog's folder under the
// name of that role. Nothing declares it, so nothing can disagree with it -
// which is also why the map is keyed by what LoadProjectModules walks rather
// than by anything read out of a file.
func moduleNameDescriptors(catalog *Catalog) map[string]moduleNameDescriptor {
	result := make(map[string]moduleNameDescriptor)
	add := func(kind Kind, object, role, name string, predefined ...string) {
		path, err := project.ObjectModulePath(string(kind), object, role)
		if err != nil {
			return
		}
		result[path] = moduleNameDescriptor{name: name, predefined: predefined}
	}
	for _, item := range catalog.Catalogs {
		add(CatalogKind, item.Name, project.ObjectModuleFile, "МодульОбъектаСправочника."+item.Name, "ЭтотОбъект", "ThisObject")
		add(CatalogKind, item.Name, project.ManagerModuleFile, "МодульМенеджераСправочника."+item.Name)
	}
	for _, item := range catalog.Documents {
		add(DocumentKind, item.Name, project.ObjectModuleFile, "МодульОбъектаДокумента."+item.Name, "ЭтотОбъект", "ThisObject", "Движения", "Movements")
		add(DocumentKind, item.Name, project.ManagerModuleFile, "МодульМенеджераДокумента."+item.Name)
	}
	for _, item := range catalog.InformationRegisters {
		add(InformationRegisterKind, item.Name, project.RecordSetModuleFile, "МодульНабораЗаписейРегистраСведений."+item.Name, "ЭтотОбъект", "ThisObject")
		add(InformationRegisterKind, item.Name, project.ManagerModuleFile, "МодульМенеджераРегистраСведений."+item.Name)
	}
	for _, item := range catalog.AccumulationRegisters {
		add(AccumulationRegisterKind, item.Name, project.RecordSetModuleFile, "МодульНабораЗаписейРегистраНакопления."+item.Name, "ЭтотОбъект", "ThisObject")
		add(AccumulationRegisterKind, item.Name, project.ManagerModuleFile, "МодульМенеджераРегистраНакопления."+item.Name)
	}
	for _, item := range catalog.Constants {
		add(ConstantKind, item.Name, project.ValueModuleFile, "МодульЗначенияКонстанты."+item.Name, "ЭтотОбъект", "ThisObject")
		add(ConstantKind, item.Name, project.ManagerModuleFile, "МодульМенеджераКонстанты."+item.Name)
	}
	for _, item := range catalog.CommonModules {
		if path, err := project.ModulePath(item.Module); err == nil {
			result[path] = moduleNameDescriptor{name: item.Name, defaultContext: item.DefaultContext()}
		}
	}
	// A form's module lies beside the form. Which forms an object keeps is
	// known only from the folders, so the names come from the index built
	// while reading them rather than from anything an object declared.
	for _, form := range catalog.ObjectForms() {
		path, err := project.ObjectFormModulePath(string(form.ObjectKind), form.Object, form.Name)
		if err != nil {
			continue
		}
		result[path] = moduleNameDescriptor{
			name:       FormModuleName(form.ObjectKind, form.Object, form.Name),
			predefined: []string{"ЭтаФорма", "ThisForm"},
		}
	}
	return result
}

// formModuleKindNames is what a form's module is called after, per kind of
// object. A kind missing here falls back to the kind's own folder name, which
// is readable enough and cannot be wrong.
var formModuleKindNames = map[Kind]string{
	CatalogKind: "Справочника", DocumentKind: "Документа", EnumerationKind: "Перечисления",
	InformationRegisterKind: "РегистраСведений", AccumulationRegisterKind: "РегистраНакопления",
	AccountingRegisterKind: "РегистраБухгалтерии", CalculationRegisterKind: "РегистраРасчета",
	ChartOfCharacteristicTypesKind: "ПланаВидовХарактеристик", ChartOfAccountsKind: "ПланаСчетов",
	ChartOfCalculationTypesKind: "ПланаВидовРасчета", BusinessProcessKind: "БизнесПроцесса",
	TaskKind: "Задачи", ExchangePlanKind: "ПланаОбмена", DocumentJournalKind: "ЖурналаДокументов",
	ReportKind: "Отчета", DataProcessorKind: "Обработки", FilterCriterionKind: "КритерияОтбора",
	SettingsStorageKind: "ХранилищаНастроек",
}

// FormModuleName is the name one form's module is compiled and reported
// under. It names the object and the form, because that pair is what the form
// is: two objects may well each keep a ФормаСписка.
func FormModuleName(objectKind Kind, object, form string) string {
	kind, ok := formModuleKindNames[objectKind]
	if !ok {
		kind = string(objectKind)
	}
	return "МодульФормы" + kind + "." + object + "." + form
}

// moduleNameFromPath names a module nothing in the catalog claims. A module
// keeps no identifier any more, so the fallback is built out of where the file
// lies, which is unique by construction and readable in a stack trace - where
// the previous fallback, a UUID with its dashes removed, was neither.
func moduleNameFromPath(relative string) string {
	trimmed := strings.TrimSuffix(filepath.ToSlash(relative), ".bsl")
	return "Модуль." + strings.ReplaceAll(strings.TrimPrefix(trimmed, "metadata/"), "/", ".")
}

func readBoundedModuleFile(path string, maximum int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return content, nil
}
