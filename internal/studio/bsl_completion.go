package studio

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/spec"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
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
	object       []kinds.Member
	movementSets []bslMovementSet
}

// bslManagers is the objects the code reaches through the global context
// (metadata.Catalog.ManagedObjects), by kind and by the name in lower case.
type bslManagers map[kinds.Kind]map[string]metadata.ManagedObject

func indexBSLManagers(catalog *metadata.Catalog) bslManagers {
	result := bslManagers{}
	for _, object := range catalog.ManagedObjects() {
		if result[object.Kind] == nil {
			result[object.Kind] = map[string]metadata.ManagedObject{}
		}
		result[object.Kind][strings.ToLower(object.Name)] = object
	}
	return result
}

// walk reads the words of the code before a point as a path through the
// managers of the global context: a collection there, then an object's name,
// then the collection of its manager that holds objects of another kind,
// then a name again (ВнешниеИсточникиДанных.Склад.Таблицы.Товары). It is the
// kind the path ends in and the name after its owner's of the object it
// names, or of the owner of the collection it ends with - empty for a
// collection of the global context. False is no such path; one through an
// object the configuration does not have names nothing in it.
func (managers bslManagers) walk(path []string) (descriptor kinds.Descriptor, name string, collection bool, ok bool) {
	for position, word := range path {
		switch {
		case position == 0:
			index := slices.IndexFunc(kinds.All(), func(item kinds.Descriptor) bool {
				return item.InGlobalContext() && item.Collection.Is(word)
			})
			if index < 0 {
				return kinds.Descriptor{}, "", false, false
			}
			descriptor = kinds.All()[index]
		case position%2 == 1:
			if name != "" {
				name += "."
			}
			name += word
		default:
			if descriptor, ok = descriptor.Inner(word); !ok {
				return kinds.Descriptor{}, "", false, false
			}
		}
	}
	return descriptor, name, len(path)%2 == 1, len(path) != 0
}

// object is the object the path names, with the members of its manager.
func (managers bslManagers) object(path []string) (metadata.ManagedObject, bool) {
	descriptor, name, collection, ok := managers.walk(path)
	if !ok || collection {
		return metadata.ManagedObject{}, false
	}
	object, exists := managers[descriptor.Kind][strings.ToLower(name)]
	return object, exists
}

// names adds the objects of the collection the path ends with: those of the
// global context, or those inside the object before it. The objects of a
// kind all lie as deep, so the owner's name before them is enough.
func (managers bslManagers) names(add *completionCollector, path []string) {
	descriptor, owner, collection, ok := managers.walk(path)
	if !ok || !collection {
		return
	}
	prefix := ""
	if owner != "" {
		prefix = owner + "."
	}
	for _, object := range managers[descriptor.Kind] {
		if len(object.Name) > len(prefix) && strings.EqualFold(object.Name[:len(prefix)], prefix) {
			add.add(object.Name[len(prefix):], "metadata-object", strings.ToLower(descriptor.Title))
		}
	}
}

// BSLSymbolIndex is an immutable project-wide symbol snapshot cached by Workspace.
type BSLSymbolIndex struct {
	modules    map[string]bslModuleIndex
	public     map[string]bslModuleIndex
	managers   bslManagers
	keywordsRU []string
	keywordsEN []string
}

type moduleDescriptor struct {
	name           string
	public         bool
	predefined     []string
	defaultContext syntax.ExecutionContext
	// object is what ЭтотОбъект has where the module runs around an object
	// of data (metadata.Catalog.ThisObjectMembers).
	object       []kinds.Member
	movementSets []bslMovementSet
}

// bslMovementSet is a record set a document writes into: Движения.<Имя
// регистра>, with the members of the register's record set.
type bslMovementSet struct {
	name    string
	members []kinds.Member
}

func (workspace *Workspace) invalidateBSLIndex() {
	workspace.mu.Lock()
	workspace.invalidateStudioIndexesLocked()
	workspace.mu.Unlock()
}

func (workspace *Workspace) invalidateStudioIndexesLocked() {
	workspace.bslIndex = nil
	workspace.bslNavigation = nil
	workspace.bslHelp = nil
	workspace.projectSearch = nil
	workspace.querySchema = nil
}

// CompleteBSL returns bounded suggestions for an unsaved module without executing project code.
func (workspace *Workspace) CompleteBSL(relative, source string, position BSLPosition) (BSLCompletion, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return BSLCompletion{}, fmt.Errorf("BSL completion requires a canonical project module path")
	}
	if !utf8.ValidString(source) || strings.IndexByte(source, 0) >= 0 {
		return BSLCompletion{}, fmt.Errorf("editable source must be valid UTF-8")
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
		modules: make(map[string]bslModuleIndex),
		public:  make(map[string]bslModuleIndex),
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
	var relativePaths []string
	for _, directory := range []string{"modules"} {
		entries, err := os.ReadDir(filepath.Join(workspace.root, directory))
		if err != nil {
			return nil, fmt.Errorf("read BSL %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" {
				continue
			}
			relativePaths = append(relativePaths, filepath.ToSlash(filepath.Join(directory, entry.Name())))
		}
	}
	objectPaths, err := project.ObjectFolderSourcePaths(workspace.root)
	if err != nil {
		return nil, err
	}
	for _, relative := range objectPaths {
		if filepath.Ext(relative) == ".bsl" {
			relativePaths = append(relativePaths, relative)
		}
	}
	for _, relative := range relativePaths {
		file, err := workspace.readSource(relative)
		if err != nil {
			return nil, err
		}
		descriptor := descriptors.of(relative)
		module := indexBSLModule(relative, descriptor, file.Content)
		result.modules[relative] = module
		if module.public {
			result.public[strings.ToLower(module.name)] = module
		}
	}
	result.managers = indexBSLManagers(metadataCatalog)
	return result, nil
}

// bslModuleDescriptors are the modules of the project as the BSL tools see
// them: named, with their words and context, as the published program names
// them (metadata.ModuleNamer), and with the members of the object the module
// runs around (metadata.Catalog.ThisObjectMembers).
type bslModuleDescriptors struct {
	namer     metadata.ModuleNamer
	objects   map[project.ModuleAddress][]kinds.Member
	movements map[string][]bslMovementSet
}

// of is the module lying at the path.
func (descriptors bslModuleDescriptors) of(relative string) moduleDescriptor {
	identity := descriptors.namer.Identify(relative)
	result := moduleDescriptor{name: identity.Name, public: identity.Public, defaultContext: identity.DefaultContext,
		predefined: identity.Predefined}
	if address, ok := project.ReadModulePath(relative); ok {
		result.object = descriptors.objects[address]
		if address.Kind == string(kinds.Document) && address.Module == kinds.ObjectModule {
			result.movementSets = descriptors.movements[address.Object]
		}
	}
	return result
}

func (workspace *Workspace) moduleDescriptors(catalog *metadata.Catalog) bslModuleDescriptors {
	result := bslModuleDescriptors{namer: catalog.ModuleNamer(), objects: catalog.ThisObjectMembers(),
		movements: make(map[string][]bslMovementSet)}
	if catalog == nil {
		return result
	}
	// A document names its own registers, and a register's record set has
	// the members of the type its record set module runs around.
	for _, item := range catalog.Documents {
		for _, target := range catalog.DocumentMovements(item.ID) {
			if set, ok := kinds.Must(target.Kind).ThisObject(kinds.RecordSetModule); ok {
				result.movements[item.Name] = append(result.movements[item.Name], bslMovementSet{name: target.Name, members: set.Members})
			}
		}
	}
	return result
}

// parameterLabel is a parameter of a routine as its source writes it, with
// Знач and the value it takes when left out (syntax.Parameter.SourceSpan);
// its name where the source does not hold it. Completion and help sign a routine of the project alike.
func parameterLabel(parameter syntax.Parameter, source string) string {
	span := parameter.SourceSpan
	if span.Start.Offset >= 0 && span.End.Offset <= len(source) && span.End.Offset > span.Start.Offset {
		return strings.TrimSpace(source[span.Start.Offset:span.End.Offset])
	}
	return parameter.Name
}

func indexBSLModule(path string, descriptor moduleDescriptor, source string) bslModuleIndex {
	parsed, _ := syntax.Parse(path, source)
	return indexParsedBSLModule(path, descriptor, source, parsed)
}

func indexParsedBSLModule(path string, descriptor moduleDescriptor, source string, parsed *syntax.Module) bslModuleIndex {
	result := bslModuleIndex{
		name: descriptor.name, public: descriptor.public,
		variables: parsed.Variables, predefined: append([]string(nil), descriptor.predefined...),
		object: descriptor.object, movementSets: descriptor.movementSets,
	}
	for _, routine := range parsed.Routines {
		parameters := make([]string, 0, len(routine.Parameters))
		for _, parameter := range routine.Parameters {
			parameters = append(parameters, parameterLabel(parameter, source))
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
		object: current.object, movementSets: current.movementSets,
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

// members adds the properties and methods of an object in the language of
// the code, each described as the object's: "свойство объекта", "record set
// method".
func (collector *completionCollector) members(members []kinds.Member, english bool, whoseEnglish, whoseRussian string) {
	for _, member := range members {
		kind, label, detail := "property", member.Name.Russian, "свойство "+whoseRussian
		if member.Method {
			kind, detail = "method", "метод "+whoseRussian
		}
		if english {
			label, detail = member.Name.English, whoseEnglish+" "+kind
		}
		collector.add(label, kind, detail)
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
	for _, kind := range kinds.All() {
		if !kind.InGlobalContext() {
			continue
		}
		if english {
			add.add(kind.Collection.English, "metadata", "application metadata")
		} else {
			add.add(kind.Collection.Russian, "metadata", "метаданные приложения")
		}
	}
	if english {
		add.values("function", "global function", "ErrorDescription", "TransactionActive")
		add.values("procedure", "global procedure", "BeginTransaction", "CommitTransaction", "RollbackTransaction")
		add.values("system-enum", "system enumeration", "DataLockMode", "AccumulationMovementKind", "DocumentWriteMode", "DocumentPostingMode")
		for _, keyword := range index.keywordsEN {
			add.add(keyword, "keyword", "keyword")
		}
	} else {
		add.values("function", "глобальная функция", "ОписаниеОшибки", "ТранзакцияАктивна")
		add.values("procedure", "глобальная процедура", "НачатьТранзакцию", "ЗафиксироватьТранзакцию", "ОтменитьТранзакцию")
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
		switch {
		case kinds.ThisObject.Is(root):
			add.members(current.object, english, "object", "объекта")
		case kinds.RegisterRecords.Is(root):
			for _, movement := range current.movementSets {
				add.add(movement.name, "metadata-object", "набор движений")
			}
		case root == "режимблокировкиданных" || root == "datalockmode":
			if english {
				add.values("enum-value", "system enum value", "Exclusive", "Shared")
			} else {
				add.values("enum-value", "значение системного перечисления", "Исключительный", "Разделяемый")
			}
		case root == "виддвижениянакопления" || root == "accumulationmovementkind":
			if english {
				add.values("enum-value", "system enum value", "Receipt", "Expense")
			} else {
				add.values("enum-value", "значение системного перечисления", "Приход", "Расход")
			}
		case root == "режимзаписидокумента" || root == "documentwritemode":
			if english {
				add.values("enum-value", "system enum value", "Write", "Post", "UndoPosting")
			} else {
				add.values("enum-value", "значение системного перечисления", "Запись", "Проведение", "ОтменаПроведения")
			}
		case root == "режимпроведениядокумента" || root == "documentpostingmode":
			if english {
				add.values("enum-value", "system enum value", "Regular", "RealTime")
			} else {
				add.values("enum-value", "значение системного перечисления", "Неоперативный", "Оперативный")
			}
		default:
			index.managers.names(add, qualifier)
		}
		return
	}
	if len(qualifier) == 2 && kinds.RegisterRecords.Is(root) {
		if movement, ok := findMovementSet(current.movementSets, qualifier[1]); ok {
			add.members(movement.members, english, "record set", "набора записей")
		}
		return
	}
	if object, ok := index.managers.object(qualifier); ok {
		add.members(object.Members, english, "manager", "менеджера")
		return
	}
	index.managers.names(add, qualifier)
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
	if label, parameters := index.objectSignature(path, current); label != "" {
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

// objectSignature is the call of a method of ЭтотОбъект, of a record set of
// Движения or of the manager of an object, as the help gives its parameters.
func (index *BSLSymbolIndex) objectSignature(path []string, current bslModuleIndex) (string, []string) {
	var members []kinds.Member
	switch {
	case len(path) == 2 && kinds.ThisObject.Is(path[0]):
		members = current.object
	case len(path) == 3 && kinds.RegisterRecords.Is(path[0]):
		movement, ok := findMovementSet(current.movementSets, path[1])
		if !ok {
			return "", nil
		}
		members = movement.members
	default:
		object, ok := index.managers.object(path[:len(path)-1])
		if !ok {
			return "", nil
		}
		members = object.Members
	}
	for _, member := range members {
		if member.Method && member.Name.Is(path[len(path)-1]) {
			parameters := make([]string, len(member.Parameters))
			for index, parameter := range member.Parameters {
				parameters[index] = parameter.Label()
			}
			return strings.Join(path, ".") + "(" + strings.Join(parameters, ", ") + ")", parameters
		}
	}
	return "", nil
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

// builtinBSLSignatures are the functions and procedures of the global
// context and the constructors the editor signs, by name in lower case.
var builtinBSLSignatures = map[string][]string{
	"описаниеошибки": {}, "errordescription": {}, "начатьтранзакцию": {}, "begintransaction": {},
	"зафиксироватьтранзакцию": {}, "committransaction": {}, "отменитьтранзакцию": {}, "rollbacktransaction": {},
	"транзакцияактивна": {}, "transactionactive": {},
	"массив": {"Размер1 = 0", "..."}, "array": {"Dimension1 = 0", "..."},
	"структура": {"ИменаСвойств = \"\"", "..."}, "structure": {"PropertyNames = \"\"", "..."},
	"соответствие": {}, "map": {}, "списокзначений": {}, "valuelist": {}, "таблицазначений": {}, "valuetable": {},
	"запрос": {"Текст = \"\""}, "query": {"Text = \"\""}, "менеджервременныхтаблиц": {}, "temptablesmanager": {},
	"блокировкаданных": {}, "datalock": {},
}

func builtinSignature(path []string) (string, []string) {
	if len(path) != 1 {
		return "", nil
	}
	parameters, ok := builtinBSLSignatures[strings.ToLower(path[0])]
	if !ok {
		return "", nil
	}
	return path[0] + "(" + strings.Join(parameters, ", ") + ")", parameters
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
