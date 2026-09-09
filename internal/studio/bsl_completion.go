package studio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/spec"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

const maxBSLCompletionItems = 200
const maxBSLCompletionCandidates = 1000

// BSLCompletionItem is one context-aware editor suggestion.
type BSLCompletionItem struct {
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Detail     string `json:"detail,omitempty"`
	InsertText string `json:"insertText"`
}

// BSLSignatureParameter identifies one argument in a call signature.
type BSLSignatureParameter struct {
	Label string `json:"label"`
}

// BSLSignatureHelp describes the innermost call at the cursor.
type BSLSignatureHelp struct {
	Label           string                  `json:"label"`
	Parameters      []BSLSignatureParameter `json:"parameters"`
	ActiveParameter int                     `json:"activeParameter"`
}

// BSLCompletion contains suggestions, their replacement range and optional signature help.
type BSLCompletion struct {
	Items     []BSLCompletionItem `json:"items"`
	Replace   BSLRange            `json:"replace"`
	Signature *BSLSignatureHelp   `json:"signature,omitempty"`
	Truncated bool                `json:"truncated,omitempty"`
}

type bslRoutineIndex struct {
	name       string
	function   bool
	exported   bool
	parameters []string
	signature  string
	span       syntax.Span
}

type bslModuleIndex struct {
	name         string
	public       bool
	variables    []syntax.Variable
	routines     []bslRoutineIndex
	predefined   []string
	objectRU     []string
	objectEN     []string
	objectKind   string
	movementSets []bslMovementSet
}

type bslMetadataIndex struct {
	constants             []string
	enumerations          []string
	enumerationValues     map[string][]string
	definedTypes          []string
	catalogs              []string
	documents             []string
	informationRegisters  []string
	accumulationRegisters []string
}

// BSLSymbolIndex is an immutable project-wide symbol snapshot cached by Workspace.
type BSLSymbolIndex struct {
	modules    map[string]bslModuleIndex
	public     map[string]bslModuleIndex
	metadata   bslMetadataIndex
	keywordsRU []string
	keywordsEN []string
	truncated  bool
}

type moduleDescriptor struct {
	name         string
	public       bool
	predefined   []string
	objectRU     []string
	objectEN     []string
	objectKind   string
	movementSets []bslMovementSet
}

type bslMovementSet struct {
	name string
	kind string
}

func (workspace *Workspace) invalidateBSLIndex() {
	workspace.mu.Lock()
	workspace.bslIndex = nil
	workspace.mu.Unlock()
}

// CompleteBSL returns bounded suggestions for an unsaved module without executing project code.
func (workspace *Workspace) CompleteBSL(relative, source string, position BSLPosition) (BSLCompletion, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return BSLCompletion{}, fmt.Errorf("BSL completion requires a canonical project module path")
	}
	if len(source) > MaxEditableFileBytes || !utf8.ValidString(source) || strings.IndexByte(source, 0) >= 0 {
		return BSLCompletion{}, fmt.Errorf("editable source must be valid UTF-8 and at most %d bytes", MaxEditableFileBytes)
	}
	offset, err := bslOffset(source, position)
	if err != nil {
		return BSLCompletion{}, err
	}

	workspace.mu.Lock()
	if workspace.bslIndex == nil {
		workspace.bslIndex, err = workspace.buildBSLSymbolIndex()
		if err != nil {
			workspace.mu.Unlock()
			return BSLCompletion{}, err
		}
	}
	index := workspace.bslIndex
	workspace.mu.Unlock()
	return index.complete(relative, source, offset), nil
}

func (workspace *Workspace) buildBSLSymbolIndex() (*BSLSymbolIndex, error) {
	result := &BSLSymbolIndex{
		modules:  make(map[string]bslModuleIndex),
		public:   make(map[string]bslModuleIndex),
		metadata: bslMetadataIndex{enumerationValues: make(map[string][]string)},
	}
	catalog, err := spec.Load()
	if err != nil {
		return nil, err
	}
	for _, keyword := range catalog.Keywords {
		if kind, ok := syntax.KeywordKind(keyword.Russian); ok && kind.IsKeyword() {
			result.keywordsRU = append(result.keywordsRU, keyword.Russian)
		}
		if kind, ok := syntax.KeywordKind(keyword.English); ok && kind.IsKeyword() {
			result.keywordsEN = append(result.keywordsEN, keyword.English)
		}
	}
	metadataCatalog, _ := metadata.Load(workspace.root)
	descriptors := workspace.moduleDescriptors(metadataCatalog)
	for _, directory := range []string{"modules", "tests"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return nil, fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" {
				continue
			}
			if len(result.modules) >= 10_000 {
				result.truncated = true
				break
			}
			relative := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			file, err := workspace.readSource(relative)
			if err != nil {
				return nil, err
			}
			id := strings.TrimSuffix(entry.Name(), ".bsl")
			descriptor := descriptors[id]
			if descriptor.name == "" {
				descriptor.name = "Модуль" + strings.ReplaceAll(id, "-", "")
				if directory == "tests" {
					descriptor.name = "Тест" + strings.ReplaceAll(id, "-", "")
				}
			}
			module := indexBSLModule(relative, descriptor, file.Content)
			result.modules[relative] = module
			if module.public {
				result.public[strings.ToLower(module.name)] = module
			}
		}
	}
	if metadataCatalog != nil {
		result.metadata = indexBSLMetadata(metadataCatalog)
	} else {
		result.metadata = workspace.scanBSLMetadata()
	}
	return result, nil
}

func (workspace *Workspace) moduleDescriptors(catalog *metadata.Catalog) map[string]moduleDescriptor {
	result := make(map[string]moduleDescriptor)
	if catalog != nil {
		movements := make(map[uuid.UUID][]bslMovementSet)
		for _, item := range catalog.InformationRegisters {
			for _, recorder := range item.Recorders {
				movements[recorder] = append(movements[recorder], bslMovementSet{name: item.Name, kind: "information"})
			}
		}
		for _, item := range catalog.AccumulationRegisters {
			for _, recorder := range item.Recorders {
				movements[recorder] = append(movements[recorder], bslMovementSet{name: item.Name, kind: "accumulation"})
			}
		}
		for _, item := range catalog.Catalogs {
			addModuleDescriptor(result, item.ObjectModule, "МодульОбъектаСправочника."+item.Name, false, "ЭтотОбъект", "ThisObject")
			if item.ObjectModule != nil {
				descriptor := result[item.ObjectModule.String()]
				descriptor.objectKind = "catalog"
				descriptor.objectRU = append([]string{"Ссылка", "Код", "Наименование", "Версия", "ПометкаУдаления", "ИмяПредопределенныхДанных"}, objectFieldNames(item.Attributes, item.TableParts)...)
				descriptor.objectEN = append([]string{"Ref", "Code", "Description", "Version", "DeletionMark", "PredefinedDataName"}, objectFieldNames(item.Attributes, item.TableParts)...)
				result[item.ObjectModule.String()] = descriptor
			}
			addModuleDescriptor(result, item.ManagerModule, "МодульМенеджераСправочника."+item.Name, false)
		}
		for _, item := range catalog.Documents {
			addModuleDescriptor(result, item.ObjectModule, "МодульОбъектаДокумента."+item.Name, false, "ЭтотОбъект", "ThisObject", "Движения", "Movements")
			if item.ObjectModule != nil {
				descriptor := result[item.ObjectModule.String()]
				descriptor.objectKind = "document"
				descriptor.objectRU = append([]string{"Ссылка", "Номер", "Дата", "Проведен", "Версия", "ПометкаУдаления", "Движения"}, objectFieldNames(item.Attributes, item.TableParts)...)
				descriptor.objectEN = append([]string{"Ref", "Number", "Date", "Posted", "Version", "DeletionMark", "Movements"}, objectFieldNames(item.Attributes, item.TableParts)...)
				descriptor.movementSets = append([]bslMovementSet(nil), movements[item.ID]...)
				result[item.ObjectModule.String()] = descriptor
			}
			addModuleDescriptor(result, item.ManagerModule, "МодульМенеджераДокумента."+item.Name, false)
		}
		for _, item := range catalog.InformationRegisters {
			addModuleDescriptor(result, item.RecordSetModule, "МодульНабораЗаписейРегистраСведений."+item.Name, false, "ЭтотОбъект", "ThisObject")
			if item.RecordSetModule != nil {
				descriptor := result[item.RecordSetModule.String()]
				descriptor.objectKind = "information-set"
				descriptor.objectRU = []string{"Отбор", "Количество", "Записывать"}
				descriptor.objectEN = []string{"Filter", "Count", "Write"}
				result[item.RecordSetModule.String()] = descriptor
			}
			addModuleDescriptor(result, item.ManagerModule, "МодульМенеджераРегистраСведений."+item.Name, false)
		}
		for _, item := range catalog.AccumulationRegisters {
			addModuleDescriptor(result, item.RecordSetModule, "МодульНабораЗаписейРегистраНакопления."+item.Name, false, "ЭтотОбъект", "ThisObject")
			if item.RecordSetModule != nil {
				descriptor := result[item.RecordSetModule.String()]
				descriptor.objectKind = "accumulation-set"
				descriptor.objectRU = []string{"Отбор", "Количество", "Записывать", "БлокироватьДляИзменения"}
				descriptor.objectEN = []string{"Filter", "Count", "Write", "LockForUpdate"}
				result[item.RecordSetModule.String()] = descriptor
			}
			addModuleDescriptor(result, item.ManagerModule, "МодульМенеджераРегистраНакопления."+item.Name, false)
		}
	}
	directory := filepath.Join(workspace.root, "metadata", "common-modules")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := readBSLIndexFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}
		var value struct {
			Name   string    `yaml:"name"`
			Module uuid.UUID `yaml:"module"`
		}
		if yaml.Unmarshal(data, &value) == nil && value.Name != "" && !value.Module.IsZero() {
			result[value.Module.String()] = moduleDescriptor{name: value.Name, public: true}
		}
	}
	return result
}

func objectFieldNames(attributes []metadata.Attribute, parts []metadata.TablePart) []string {
	result := make([]string, 0, len(attributes)+len(parts))
	for _, item := range attributes {
		result = append(result, item.Name)
	}
	for _, item := range parts {
		result = append(result, item.Name)
	}
	return result
}

func (workspace *Workspace) scanBSLMetadata() bslMetadataIndex {
	result := bslMetadataIndex{enumerationValues: make(map[string][]string)}
	for _, kind := range []string{"constants", "enumerations", "defined-types", "catalogs", "documents", "information-registers", "accumulation-registers"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", kind))
		if err != nil {
			continue
		}
		for index, entry := range entries {
			if index >= 100_000 || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			data, err := readBSLIndexFile(filepath.Join(workspace.root, "metadata", kind, entry.Name()))
			if err != nil {
				continue
			}
			var value struct {
				Name   string `yaml:"name"`
				Values []struct {
					Name string `yaml:"name"`
				} `yaml:"values"`
			}
			if yaml.Unmarshal(data, &value) != nil || value.Name == "" {
				continue
			}
			switch kind {
			case "constants":
				result.constants = append(result.constants, value.Name)
			case "enumerations":
				result.enumerations = append(result.enumerations, value.Name)
				for _, item := range value.Values {
					if item.Name != "" {
						key := strings.ToLower(value.Name)
						result.enumerationValues[key] = append(result.enumerationValues[key], item.Name)
					}
				}
			case "defined-types":
				result.definedTypes = append(result.definedTypes, value.Name)
			case "catalogs":
				result.catalogs = append(result.catalogs, value.Name)
			case "documents":
				result.documents = append(result.documents, value.Name)
			case "information-registers":
				result.informationRegisters = append(result.informationRegisters, value.Name)
			case "accumulation-registers":
				result.accumulationRegisters = append(result.accumulationRegisters, value.Name)
			}
		}
	}
	return result
}

func readBSLIndexFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("BSL index source exceeds 1 MiB")
	}
	return data, nil
}

func addModuleDescriptor(target map[string]moduleDescriptor, id *uuid.UUID, name string, public bool, predefined ...string) {
	if id != nil && !id.IsZero() {
		target[id.String()] = moduleDescriptor{name: name, public: public, predefined: predefined}
	}
}

func indexBSLModule(path string, descriptor moduleDescriptor, source string) bslModuleIndex {
	parsed, _ := syntax.Parse(path, source)
	return indexParsedBSLModule(path, descriptor, source, parsed)
}

func indexParsedBSLModule(path string, descriptor moduleDescriptor, source string, parsed *syntax.Module) bslModuleIndex {
	result := bslModuleIndex{
		name: descriptor.name, public: descriptor.public,
		variables: parsed.Variables, predefined: append([]string(nil), descriptor.predefined...),
		objectRU: append([]string(nil), descriptor.objectRU...), objectEN: append([]string(nil), descriptor.objectEN...),
		objectKind: descriptor.objectKind, movementSets: append([]bslMovementSet(nil), descriptor.movementSets...),
	}
	for _, routine := range parsed.Routines {
		parameters := make([]string, 0, len(routine.Parameters))
		for _, parameter := range routine.Parameters {
			label := parameter.Name
			if parameter.SourceSpan.Start.Offset >= 0 && parameter.SourceSpan.End.Offset <= len(source) && parameter.SourceSpan.End.Offset > parameter.SourceSpan.Start.Offset {
				label = strings.TrimSpace(source[parameter.SourceSpan.Start.Offset:parameter.SourceSpan.End.Offset])
			}
			parameters = append(parameters, label)
		}
		kind := "Процедура"
		if routine.Function {
			kind = "Функция"
		}
		result.routines = append(result.routines, bslRoutineIndex{
			name: routine.Name, function: routine.Function, exported: routine.Export, parameters: parameters,
			signature: kind + " " + routine.Name + "(" + strings.Join(parameters, ", ") + ")", span: routine.SourceSpan,
		})
	}
	return result
}

func indexBSLMetadata(catalog *metadata.Catalog) bslMetadataIndex {
	result := bslMetadataIndex{enumerationValues: make(map[string][]string)}
	for _, item := range catalog.Constants {
		result.constants = append(result.constants, item.Name)
	}
	for _, item := range catalog.Enumerations {
		result.enumerations = append(result.enumerations, item.Name)
		for _, value := range item.Values {
			result.enumerationValues[strings.ToLower(item.Name)] = append(result.enumerationValues[strings.ToLower(item.Name)], value.Name)
		}
	}
	for _, item := range catalog.DefinedTypes {
		result.definedTypes = append(result.definedTypes, item.Name)
	}
	for _, item := range catalog.Catalogs {
		result.catalogs = append(result.catalogs, item.Name)
	}
	for _, item := range catalog.Documents {
		result.documents = append(result.documents, item.Name)
	}
	for _, item := range catalog.InformationRegisters {
		result.informationRegisters = append(result.informationRegisters, item.Name)
	}
	for _, item := range catalog.AccumulationRegisters {
		result.accumulationRegisters = append(result.accumulationRegisters, item.Name)
	}
	return result
}

func (index *BSLSymbolIndex) complete(path, source string, offset int) BSLCompletion {
	start := identifierStart(source, offset)
	result := BSLCompletion{
		Items:   make([]BSLCompletionItem, 0, 32),
		Replace: BSLRange{Start: bslPosition(source, start), End: bslPosition(source, offset)},
	}
	parsed, tokens, _ := syntax.ParseWithTokens(path, source)
	if cursorInLiteralOrComment(tokens, source, offset) {
		return result
	}
	prefix := source[start:offset]
	qualifier, previous := bslQualifier(tokens, start)
	english := preferEnglish(prefix, qualifier, tokens)
	current := index.modules[path]
	current = indexParsedBSLModule(path, moduleDescriptor{
		name: current.name, public: current.public, predefined: current.predefined,
		objectRU: current.objectRU, objectEN: current.objectEN, objectKind: current.objectKind, movementSets: current.movementSets,
	}, source, parsed)

	add := newCompletionCollector(prefix)
	switch {
	case previous == syntax.Ampersand:
		if english {
			add.values("directive", "execution directive", "AtClient", "AtServer", "AtServerNoContext", "AtClientAtServer", "AtClientAtServerNoContext")
		} else {
			add.values("directive", "директива выполнения", "НаКлиенте", "НаСервере", "НаСервереБезКонтекста", "НаКлиентеНаСервере", "НаКлиентеНаСервереБезКонтекста")
		}
	case len(qualifier) != 0:
		index.addQualified(add, qualifier, english, current)
	case previous == syntax.New:
		if english {
			add.values("constructor", "constructor", "Array", "Structure", "Map", "ValueList", "ValueTable", "Query", "TempTablesManager", "DataLock")
		} else {
			add.values("constructor", "конструктор", "Массив", "Структура", "Соответствие", "СписокЗначений", "ТаблицаЗначений", "Запрос", "МенеджерВременныхТаблиц", "БлокировкаДанных")
		}
	default:
		index.addUnqualified(add, current, parsed, offset, english)
	}
	result.Items, result.Truncated = add.items(maxBSLCompletionItems)
	result.Truncated = result.Truncated || index.truncated
	result.Signature = index.signature(tokens, current, offset)
	return result
}

type completionCollector struct {
	prefix   string
	seen     map[string]bool
	all      []BSLCompletionItem
	overflow bool
}

func newCompletionCollector(prefix string) *completionCollector {
	return &completionCollector{prefix: strings.ToLower(prefix), seen: make(map[string]bool)}
}

func (collector *completionCollector) add(label, kind, detail string) {
	key := strings.ToLower(label)
	if label == "" || collector.seen[key] || !strings.HasPrefix(key, collector.prefix) {
		return
	}
	collector.seen[key] = true
	if len(collector.all) >= maxBSLCompletionCandidates {
		collector.overflow = true
		return
	}
	collector.all = append(collector.all, BSLCompletionItem{Label: label, Kind: kind, Detail: detail, InsertText: label})
}

func (collector *completionCollector) values(kind, detail string, values ...string) {
	for _, value := range values {
		collector.add(value, kind, detail)
	}
}

func (collector *completionCollector) items(limit int) ([]BSLCompletionItem, bool) {
	if collector.all == nil {
		return []BSLCompletionItem{}, collector.overflow
	}
	sort.SliceStable(collector.all, func(left, right int) bool {
		return strings.ToLower(collector.all[left].Label) < strings.ToLower(collector.all[right].Label)
	})
	truncated := collector.overflow || len(collector.all) > limit
	if truncated {
		collector.all = collector.all[:limit]
	}
	return collector.all, truncated
}

func (index *BSLSymbolIndex) addUnqualified(add *completionCollector, current bslModuleIndex, parsed *syntax.Module, offset int, english bool) {
	for _, predefined := range current.predefined {
		add.add(predefined, "variable", "предопределённая переменная")
	}
	for _, variable := range parsed.Variables {
		add.add(variable.Name, "variable", "переменная модуля")
	}
	for _, routine := range current.routines {
		kind := "procedure"
		if routine.function {
			kind = "function"
		}
		add.add(routine.name, kind, routine.signature)
		if offset >= routine.span.Start.Offset && offset <= routine.span.End.Offset {
			parsedRoutine := routineAt(parsed, offset)
			if parsedRoutine != nil {
				for _, parameter := range parsedRoutine.Parameters {
					add.add(parameter.Name, "parameter", "параметр")
				}
				for _, name := range bslLocals(parsedRoutine.Body) {
					add.add(name, "variable", "локальная переменная")
				}
			}
		}
	}
	for _, module := range index.public {
		add.add(module.name, "module", "общий модуль")
	}
	if english {
		add.values("function", "global function", "ErrorDescription", "TransactionActive")
		add.values("procedure", "global procedure", "BeginTransaction", "CommitTransaction", "RollbackTransaction")
		add.values("metadata", "application metadata", "Catalogs", "Documents", "InformationRegisters", "AccumulationRegisters", "Constants", "Enums", "DefinedTypes")
		add.values("system-enum", "system enumeration", "DataLockMode", "AccumulationMovementKind", "DocumentWriteMode", "DocumentPostingMode")
		for _, keyword := range index.keywordsEN {
			add.add(keyword, "keyword", "keyword")
		}
	} else {
		add.values("function", "глобальная функция", "ОписаниеОшибки", "ТранзакцияАктивна")
		add.values("procedure", "глобальная процедура", "НачатьТранзакцию", "ЗафиксироватьТранзакцию", "ОтменитьТранзакцию")
		add.values("metadata", "метаданные приложения", "Справочники", "Документы", "РегистрыСведений", "РегистрыНакопления", "Константы", "Перечисления", "ОпределяемыеТипы")
		add.values("system-enum", "системное перечисление", "РежимБлокировкиДанных", "ВидДвиженияНакопления", "РежимЗаписиДокумента", "РежимПроведенияДокумента")
		for _, keyword := range index.keywordsRU {
			add.add(keyword, "keyword", "ключевое слово")
		}
	}
}

func (index *BSLSymbolIndex) addQualified(add *completionCollector, qualifier []string, english bool, current bslModuleIndex) {
	if len(qualifier) == 1 {
		if module, ok := index.public[strings.ToLower(qualifier[0])]; ok {
			for _, variable := range module.variables {
				if variable.Export {
					add.add(variable.Name, "variable", module.name)
				}
			}
			for _, routine := range module.routines {
				if routine.exported {
					kind := "procedure"
					if routine.function {
						kind = "function"
					}
					add.add(routine.name, kind, routine.signature)
				}
			}
			return
		}
	}
	root := strings.ToLower(qualifier[0])
	if len(qualifier) == 1 {
		switch root {
		case "этотобъект", "thisobject":
			if english {
				add.values("property", "object property", current.objectEN...)
				add.values("method", "object method", objectMethodNames(current.objectKind, true)...)
			} else {
				add.values("property", "реквизит объекта", current.objectRU...)
				add.values("method", "метод объекта", objectMethodNames(current.objectKind, false)...)
			}
		case "движения", "movements":
			for _, movement := range current.movementSets {
				add.add(movement.name, "metadata-object", "набор движений")
			}
		case "справочники", "catalogs":
			add.values("metadata-object", "справочник", index.metadata.catalogs...)
		case "документы", "documents":
			add.values("metadata-object", "документ", index.metadata.documents...)
		case "регистрысведений", "informationregisters":
			add.values("metadata-object", "регистр сведений", index.metadata.informationRegisters...)
		case "регистрынакопления", "accumulationregisters":
			add.values("metadata-object", "регистр накопления", index.metadata.accumulationRegisters...)
		case "константы", "constants":
			add.values("metadata-object", "константа", index.metadata.constants...)
		case "перечисления", "enums":
			add.values("metadata-object", "перечисление", index.metadata.enumerations...)
		case "определяемыетипы", "definedtypes":
			add.values("metadata-object", "определяемый тип", index.metadata.definedTypes...)
		case "режимблокировкиданных", "datalockmode":
			if english {
				add.values("enum-value", "system enum value", "Exclusive", "Shared")
			} else {
				add.values("enum-value", "значение системного перечисления", "Исключительный", "Разделяемый")
			}
		case "виддвижениянакопления", "accumulationmovementkind":
			if english {
				add.values("enum-value", "system enum value", "Receipt", "Expense")
			} else {
				add.values("enum-value", "значение системного перечисления", "Приход", "Расход")
			}
		case "режимзаписидокумента", "documentwritemode":
			if english {
				add.values("enum-value", "system enum value", "Write", "Post", "UndoPosting")
			} else {
				add.values("enum-value", "значение системного перечисления", "Запись", "Проведение", "ОтменаПроведения")
			}
		case "режимпроведениядокумента", "documentpostingmode":
			if english {
				add.values("enum-value", "system enum value", "Regular", "RealTime")
			} else {
				add.values("enum-value", "значение системного перечисления", "Неоперативный", "Оперативный")
			}
		}
		return
	}
	if len(qualifier) != 2 {
		return
	}
	switch root {
	case "движения", "movements":
		movement, ok := findMovementSet(current.movementSets, qualifier[1])
		if !ok {
			return
		}
		if english {
			add.values("property", "record set property", "Filter", "Count", "Write")
			if movement.kind == "accumulation" {
				add.add("LockForUpdate", "property", "record set property")
			}
			add.values("method", "record set method", "Add", "Clear", "Get", "Delete", "Read", "Write")
		} else {
			add.values("property", "свойство набора записей", "Отбор", "Количество", "Записывать")
			if movement.kind == "accumulation" {
				add.add("БлокироватьДляИзменения", "property", "свойство набора записей")
			}
			add.values("method", "метод набора записей", "Добавить", "Очистить", "Получить", "Удалить", "Прочитать", "Записать")
		}
	case "перечисления", "enums":
		add.values("enum-value", "значение перечисления", index.metadata.enumerationValues[strings.ToLower(qualifier[1])]...)
	case "справочники", "catalogs":
		if !containsFold(index.metadata.catalogs, qualifier[1]) {
			return
		}
		if english {
			add.values("method", "catalog manager", "CreateItem", "GetObject", "FindByCode", "GetRef", "GetReference")
		} else {
			add.values("method", "менеджер справочника", "СоздатьЭлемент", "ПолучитьОбъект", "НайтиПоКоду", "ПолучитьСсылку")
		}
	case "документы", "documents":
		if !containsFold(index.metadata.documents, qualifier[1]) {
			return
		}
		if english {
			add.values("method", "document manager", "CreateDocument", "GetObject", "FindByNumber", "GetRef", "GetReference")
		} else {
			add.values("method", "менеджер документа", "СоздатьДокумент", "ПолучитьОбъект", "НайтиПоНомеру", "ПолучитьСсылку")
		}
	case "регистрысведений", "informationregisters":
		if !containsFold(index.metadata.informationRegisters, qualifier[1]) {
			return
		}
		if english {
			add.values("method", "information register manager", "CreateRecordSet", "SliceLast", "SliceFirst")
		} else {
			add.values("method", "менеджер регистра сведений", "СоздатьНаборЗаписей", "СрезПоследних", "СрезПервых")
		}
	case "регистрынакопления", "accumulationregisters":
		if !containsFold(index.metadata.accumulationRegisters, qualifier[1]) {
			return
		}
		if english {
			add.values("method", "accumulation register manager", "CreateRecordSet", "Balances", "Turnovers", "BalancesAndTurnovers")
		} else {
			add.values("method", "менеджер регистра накопления", "СоздатьНаборЗаписей", "Остатки", "Обороты", "ОстаткиИОбороты")
		}
	case "константы", "constants":
		if !containsFold(index.metadata.constants, qualifier[1]) {
			return
		}
		if english {
			add.values("method", "constant manager", "Get", "Set")
		} else {
			add.values("method", "менеджер константы", "Получить", "Установить")
		}
	}
}

func (index *BSLSymbolIndex) signature(tokens []syntax.Token, current bslModuleIndex, offset int) *BSLSignatureHelp {
	path, active := openCall(tokens, offset)
	if len(path) == 0 {
		return nil
	}
	var routine *bslRoutineIndex
	if len(path) == 1 {
		for item := range current.routines {
			if strings.EqualFold(current.routines[item].name, path[0]) {
				routine = &current.routines[item]
				break
			}
		}
	} else if len(path) == 2 {
		if module, ok := index.public[strings.ToLower(path[0])]; ok {
			for item := range module.routines {
				if module.routines[item].exported && strings.EqualFold(module.routines[item].name, path[1]) {
					routine = &module.routines[item]
					break
				}
			}
		}
	}
	if routine != nil {
		return signatureHelp(routine.signature, routine.parameters, active)
	}
	if label, parameters := objectSignature(path, current); label != "" {
		return signatureHelp(label, parameters, active)
	}
	if label, parameters := movementSetSignature(path, current); label != "" {
		return signatureHelp(label, parameters, active)
	}
	if label, parameters := builtinSignature(path); label != "" {
		return signatureHelp(label, parameters, active)
	}
	return nil
}

func findMovementSet(sets []bslMovementSet, name string) (bslMovementSet, bool) {
	for _, set := range sets {
		if strings.EqualFold(set.name, name) {
			return set, true
		}
	}
	return bslMovementSet{}, false
}

func objectMethodNames(kind string, english bool) []string {
	if kind == "information-set" || kind == "accumulation-set" {
		if english {
			return []string{"Add", "Clear", "Get", "Delete", "Read", "Write"}
		}
		return []string{"Добавить", "Очистить", "Получить", "Удалить", "Прочитать", "Записать"}
	}
	if kind == "catalog" {
		if english {
			return []string{"Write", "GetRef", "GetReference", "SetDeletionMark", "Delete"}
		}
		return []string{"Записать", "ПолучитьСсылку", "УстановитьПометкуУдаления", "Удалить"}
	}
	if kind == "document" {
		if english {
			return []string{"Write", "Post", "UndoPosting", "GetRef", "GetReference", "SetDeletionMark", "Delete"}
		}
		return []string{"Записать", "Провести", "ОтменитьПроведение", "ПолучитьСсылку", "УстановитьПометкуУдаления", "Удалить"}
	}
	return nil
}

func objectSignature(path []string, current bslModuleIndex) (string, []string) {
	if len(path) != 2 || !strings.EqualFold(path[0], "ЭтотОбъект") && !strings.EqualFold(path[0], "ThisObject") {
		return "", nil
	}
	method := strings.ToLower(path[1])
	var parameters []string
	switch {
	case current.objectKind == "catalog" && (method == "записать" || method == "write" || method == "получитьссылку" || method == "getref" || method == "getreference" || method == "удалить" || method == "delete"):
		parameters = []string{}
	case current.objectKind == "catalog" && method == "установитьпометкуудаления":
		parameters = []string{"Пометка"}
	case current.objectKind == "catalog" && method == "setdeletionmark":
		parameters = []string{"Mark"}
	case current.objectKind == "document" && (method == "получитьссылку" || method == "getref" || method == "getreference" || method == "отменитьпроведение" || method == "undoposting" || method == "удалить" || method == "delete"):
		parameters = []string{}
	case current.objectKind == "document" && method == "записать":
		parameters = []string{"РежимЗаписи = РежимЗаписиДокумента.Запись", "РежимПроведения = РежимПроведенияДокумента.Неоперативный"}
	case current.objectKind == "document" && method == "write":
		parameters = []string{"WriteMode = DocumentWriteMode.Write", "PostingMode = DocumentPostingMode.Regular"}
	case current.objectKind == "document" && method == "провести":
		parameters = []string{"РежимПроведения = РежимПроведенияДокумента.Неоперативный"}
	case current.objectKind == "document" && method == "post":
		parameters = []string{"PostingMode = DocumentPostingMode.Regular"}
	case current.objectKind == "document" && method == "установитьпометкуудаления":
		parameters = []string{"Пометка"}
	case current.objectKind == "document" && method == "setdeletionmark":
		parameters = []string{"Mark"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && (method == "добавить" || method == "add" || method == "очистить" || method == "clear" || method == "прочитать" || method == "read"):
		parameters = []string{}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "получить":
		parameters = []string{"Индекс"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "get":
		parameters = []string{"Index"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "удалить":
		parameters = []string{"Запись"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "delete":
		parameters = []string{"Record"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "записать":
		parameters = []string{"Замещение = Истина"}
	case (current.objectKind == "information-set" || current.objectKind == "accumulation-set") && method == "write":
		parameters = []string{"Replace = True"}
	default:
		return "", nil
	}
	return strings.Join(path, ".") + "(" + strings.Join(parameters, ", ") + ")", parameters
}

func movementSetSignature(path []string, current bslModuleIndex) (string, []string) {
	if len(path) != 3 || !strings.EqualFold(path[0], "Движения") && !strings.EqualFold(path[0], "Movements") {
		return "", nil
	}
	if _, ok := findMovementSet(current.movementSets, path[1]); !ok {
		return "", nil
	}
	var parameters []string
	switch strings.ToLower(path[2]) {
	case "добавить", "add", "очистить", "clear", "прочитать", "read":
		parameters = []string{}
	case "получить":
		parameters = []string{"Индекс"}
	case "get":
		parameters = []string{"Index"}
	case "удалить":
		parameters = []string{"Запись"}
	case "delete":
		parameters = []string{"Record"}
	case "записать":
		parameters = []string{"Замещение = Истина"}
	case "write":
		parameters = []string{"Replace = True"}
	default:
		return "", nil
	}
	return strings.Join(path, ".") + "(" + strings.Join(parameters, ", ") + ")", parameters
}

func signatureHelp(label string, parameters []string, active int) *BSLSignatureHelp {
	result := &BSLSignatureHelp{Label: label, ActiveParameter: active, Parameters: make([]BSLSignatureParameter, len(parameters))}
	if len(parameters) == 0 {
		result.ActiveParameter = 0
	} else if result.ActiveParameter >= len(parameters) {
		result.ActiveParameter = len(parameters) - 1
	}
	for index, parameter := range parameters {
		result.Parameters[index] = BSLSignatureParameter{Label: parameter}
	}
	return result
}

var standaloneBSLSignatures = map[string]bool{
	"описаниеошибки": true, "errordescription": true, "начатьтранзакцию": true, "begintransaction": true,
	"зафиксироватьтранзакцию": true, "committransaction": true, "отменитьтранзакцию": true, "rollbacktransaction": true,
	"транзакцияактивна": true, "transactionactive": true, "массив": true, "array": true, "структура": true, "structure": true,
	"соответствие": true, "map": true, "списокзначений": true, "valuelist": true, "таблицазначений": true, "valuetable": true,
	"запрос": true, "query": true, "менеджервременныхтаблиц": true, "temptablesmanager": true, "блокировкаданных": true, "datalock": true,
}

var builtinBSLSignatures = map[string][]string{
	"описаниеошибки": {}, "errordescription": {}, "начатьтранзакцию": {}, "begintransaction": {},
	"зафиксироватьтранзакцию": {}, "committransaction": {}, "отменитьтранзакцию": {}, "rollbacktransaction": {},
	"транзакцияактивна": {}, "transactionactive": {}, "создатьэлемент": {}, "createitem": {},
	"создатьдокумент": {}, "createdocument": {}, "получитьобъект": {"Ссылка"}, "getobject": {"Reference"},
	"найтипокоду": {"Код"}, "findbycode": {"Code"}, "найтипономеру": {"Номер", "Период = Неопределено"}, "findbynumber": {"Number", "Period = Undefined"},
	"получитьссылку": {"Идентификатор"}, "getref": {"Identifier"}, "getreference": {"Identifier"},
	"создатьнаборзаписей": {}, "createrecordset": {}, "срезпоследних": {"Период", "Отбор = Неопределено"}, "slicelast": {"Period", "Filter = Undefined"},
	"срезпервых": {"Период", "Отбор = Неопределено"}, "slicefirst": {"Period", "Filter = Undefined"},
	"остатки": {"Период", "Отбор = Неопределено"}, "balances": {"Period", "Filter = Undefined"},
	"обороты": {"НачалоПериода", "КонецПериода", "Отбор = Неопределено"}, "turnovers": {"PeriodBegin", "PeriodEnd", "Filter = Undefined"},
	"остаткииобороты": {"НачалоПериода", "КонецПериода", "Отбор = Неопределено"}, "balancesandturnovers": {"PeriodBegin", "PeriodEnd", "Filter = Undefined"},
	"получить": {}, "get": {}, "установить": {"Значение"}, "set": {"Value"},
	"массив": {"Размер1 = 0", "..."}, "array": {"Dimension1 = 0", "..."},
	"структура": {"ИменаСвойств = \"\"", "..."}, "structure": {"PropertyNames = \"\"", "..."},
	"соответствие": {}, "map": {}, "списокзначений": {}, "valuelist": {}, "таблицазначений": {}, "valuetable": {},
	"запрос": {"Текст = \"\""}, "query": {"Text = \"\""}, "менеджервременныхтаблиц": {}, "temptablesmanager": {},
	"блокировкаданных": {}, "datalock": {},
}

func builtinSignature(path []string) (string, []string) {
	name := strings.ToLower(path[len(path)-1])
	if len(path) == 1 {
		if !standaloneBSLSignatures[name] {
			return "", nil
		}
	} else if len(path) != 3 || !metadataMethod(path[0], name) {
		return "", nil
	}
	parameters, ok := builtinBSLSignatures[name]
	if !ok {
		return "", nil
	}
	label := strings.Join(path, ".") + "(" + strings.Join(parameters, ", ") + ")"
	return label, parameters
}

func metadataMethod(root, method string) bool {
	var allowed []string
	switch strings.ToLower(root) {
	case "справочники", "catalogs":
		allowed = []string{"создатьэлемент", "createitem", "получитьобъект", "getobject", "найтипокоду", "findbycode", "получитьссылку", "getref", "getreference"}
	case "документы", "documents":
		allowed = []string{"создатьдокумент", "createdocument", "получитьобъект", "getobject", "найтипономеру", "findbynumber", "получитьссылку", "getref", "getreference"}
	case "регистрысведений", "informationregisters":
		allowed = []string{"создатьнаборзаписей", "createrecordset", "срезпоследних", "slicelast", "срезпервых", "slicefirst"}
	case "регистрынакопления", "accumulationregisters":
		allowed = []string{"создатьнаборзаписей", "createrecordset", "остатки", "balances", "обороты", "turnovers", "остаткииобороты", "balancesandturnovers"}
	case "константы", "constants":
		allowed = []string{"получить", "get", "установить", "set"}
	}
	for _, candidate := range allowed {
		if method == candidate {
			return true
		}
	}
	return false
}

func containsFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func openCall(tokens []syntax.Token, offset int) ([]string, int) {
	type call struct{ token, active int }
	stack := make([]call, 0, 4)
	for index, token := range tokens {
		if token.Kind == syntax.EOF || token.Span.Start.Offset >= offset {
			break
		}
		switch token.Kind {
		case syntax.LeftParen:
			stack = append(stack, call{token: index})
		case syntax.Comma:
			if len(stack) != 0 {
				stack[len(stack)-1].active++
			}
		case syntax.RightParen:
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return nil, 0
	}
	entry := stack[len(stack)-1]
	return tokenPathBefore(tokens, entry.token), entry.active
}

func tokenPathBefore(tokens []syntax.Token, index int) []string {
	if index <= 0 {
		return nil
	}
	parts := make([]string, 0, 3)
	expectName := true
	for cursor := index - 1; cursor >= 0; cursor-- {
		token := tokens[cursor]
		if expectName {
			if token.Kind != syntax.Identifier {
				break
			}
			parts = append(parts, token.Value)
			expectName = false
		} else {
			if token.Kind != syntax.Dot && token.Kind != syntax.DotTrailing {
				break
			}
			expectName = true
		}
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return parts
}

func bslQualifier(tokens []syntax.Token, prefixStart int) ([]string, syntax.Kind) {
	previous := syntax.Invalid
	index := -1
	for item, token := range tokens {
		if token.Kind == syntax.EOF || token.Span.End.Offset > prefixStart {
			break
		}
		index = item
		previous = token.Kind
	}
	if index < 0 || previous != syntax.Dot && previous != syntax.DotTrailing {
		return nil, previous
	}
	parts := tokenPathBefore(tokens, index)
	return parts, previous
}

func routineAt(module *syntax.Module, offset int) *syntax.Routine {
	for _, routine := range module.Routines {
		if offset >= routine.SourceSpan.Start.Offset && offset <= routine.SourceSpan.End.Offset {
			return routine
		}
	}
	return nil
}

func bslLocals(statements []syntax.Statement) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	var walk func([]syntax.Statement)
	add := func(name string) {
		key := strings.ToLower(name)
		if name != "" && !seen[key] {
			seen[key] = true
			result = append(result, name)
		}
	}
	walk = func(items []syntax.Statement) {
		for _, statement := range items {
			switch node := statement.(type) {
			case *syntax.VariableStatement:
				for _, variable := range node.Variables {
					add(variable.Name)
				}
			case *syntax.AssignmentStatement:
				if node.Qualifier == "" {
					add(node.Name)
				}
			case *syntax.ForStatement:
				add(node.Variable)
				walk(node.Body)
			case *syntax.ForEachStatement:
				add(node.Variable)
				walk(node.Body)
			case *syntax.IfStatement:
				for _, branch := range node.Branches {
					walk(branch.Body)
				}
				walk(node.ElseBody)
			case *syntax.WhileStatement:
				walk(node.Body)
			case *syntax.TryStatement:
				walk(node.Body)
				walk(node.ExceptBody)
			}
		}
	}
	walk(statements)
	return result
}

func bslOffset(source string, position BSLPosition) (int, error) {
	if position.Line < 1 || position.Column < 1 {
		return 0, fmt.Errorf("BSL cursor position must be one-based")
	}
	line, column := 1, 1
	for offset := 0; offset < len(source); {
		if line == position.Line && column == position.Column {
			return offset, nil
		}
		r, width := utf8.DecodeRuneInString(source[offset:])
		if r == '\r' {
			offset += width
			if offset < len(source) && source[offset] == '\n' {
				offset++
			}
			line, column = line+1, 1
			continue
		}
		offset += width
		if r == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	if line == position.Line && column == position.Column {
		return len(source), nil
	}
	return 0, fmt.Errorf("BSL cursor position is outside the source")
}

func bslPosition(source string, target int) BSLPosition {
	line, column := 1, 1
	for offset := 0; offset < target; {
		r, width := utf8.DecodeRuneInString(source[offset:])
		offset += width
		if r == '\r' {
			if offset < target && source[offset] == '\n' {
				offset++
			}
			line, column = line+1, 1
		} else if r == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return BSLPosition{Line: line, Column: column}
}

func identifierStart(source string, offset int) int {
	for offset > 0 {
		r, width := utf8.DecodeLastRuneInString(source[:offset])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		offset -= width
	}
	return offset
}

func cursorInLiteralOrComment(tokens []syntax.Token, source string, offset int) bool {
	for _, token := range tokens {
		for _, trivia := range token.LeadingTrivia {
			if trivia.Kind == syntax.LineCommentTrivia && offset > trivia.Span.Start.Offset && offset <= trivia.Span.End.Offset {
				return true
			}
		}
		if offset <= token.Span.Start.Offset || offset > token.Span.End.Offset {
			continue
		}
		switch token.Kind {
		case syntax.String, syntax.StringStart, syntax.StringPart, syntax.StringEnd, syntax.Date:
			if offset < token.Span.End.Offset || offset == 0 || source[offset-1] != '"' && source[offset-1] != '\'' {
				return true
			}
		}
	}
	return false
}

func preferEnglish(prefix string, qualifier []string, tokens []syntax.Token) bool {
	probe := prefix
	if len(qualifier) != 0 {
		probe = qualifier[0]
	}
	for _, r := range probe {
		if unicode.IsLetter(r) {
			return r <= unicode.MaxASCII
		}
	}
	english, russian := 0, 0
	for _, token := range tokens {
		if !token.Kind.IsKeyword() {
			continue
		}
		for _, r := range token.Lexeme {
			if unicode.IsLetter(r) {
				if r <= unicode.MaxASCII {
					english++
				} else {
					russian++
				}
				break
			}
		}
	}
	return english > russian
}
