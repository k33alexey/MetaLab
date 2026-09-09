package studio

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/spec"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const (
	maxBSLHelpItems       = 50_000
	maxBSLHelpResults     = 200
	maxBSLHelpQueryRunes  = 256
	maxBSLHelpCommentSize = 16 << 10
	maxBSLHelpTextBytes   = 64 << 20
)

var ErrBSLHelpNotFound = errors.New("BSL help topic not found")

// BSLHelpParameter documents one procedure or function parameter.
type BSLHelpParameter struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// BSLHelpItem is one local syntax-help topic. Text is plain text and safe to render without HTML.
type BSLHelpItem struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Kind        string             `json:"kind"`
	Signature   string             `json:"signature,omitempty"`
	Summary     string             `json:"summary"`
	Description string             `json:"description,omitempty"`
	Parameters  []BSLHelpParameter `json:"parameters,omitempty"`
	Returns     string             `json:"returns,omitempty"`
	Source      string             `json:"source"`
	Location    *StudioLocation    `json:"location,omitempty"`
}

// BSLHelpSearchResult contains bounded local syntax-help matches.
type BSLHelpSearchResult struct {
	Items     []BSLHelpItem `json:"items"`
	Truncated bool          `json:"truncated,omitempty"`
}

type bslHelpDocument struct {
	item       BSLHelpItem
	aliases    []string
	searchText string
}

type bslHelpIndex struct {
	documents []bslHelpDocument
	byAlias   map[string][]int
	bySymbol  map[string]int
	textBytes int
	full      bool
	truncated bool
}

type platformHelpDefinition struct {
	id          string
	name        string
	kind        string
	signature   string
	summary     string
	description string
	returns     string
	aliases     []string
	parameters  []string
}

// SearchBSLHelp searches the embedded platform reference and documented ML Project routines.
func (workspace *Workspace) SearchBSLHelp(query string) (BSLHelpSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || !utf8.ValidString(query) || utf8.RuneCountInString(query) > maxBSLHelpQueryRunes {
		return BSLHelpSearchResult{}, fmt.Errorf("BSL help query must contain 1..%d Unicode characters", maxBSLHelpQueryRunes)
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	index, err := workspace.ensureBSLHelpIndexLocked()
	if err != nil {
		return BSLHelpSearchResult{}, err
	}
	return index.search(query), nil
}

// ResolveBSLHelp returns the best local help topic for the symbol under an unsaved editor cursor.
func (workspace *Workspace) ResolveBSLHelp(relative, source string, position BSLPosition) (BSLHelpItem, error) {
	relative, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return BSLHelpItem{}, fmt.Errorf("BSL help requires a canonical project module path")
	}
	if err := validateEditorSource(source); err != nil {
		return BSLHelpItem{}, err
	}
	offset, err := bslOffset(source, position)
	if err != nil {
		return BSLHelpItem{}, err
	}
	parsed, tokens, _ := syntax.ParseWithTokens(relative, source)
	if cursorInLiteralOrComment(tokens, source, offset) {
		return BSLHelpItem{}, ErrBSLHelpNotFound
	}

	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	index, err := workspace.ensureBSLHelpIndexLocked()
	if err != nil {
		return BSLHelpItem{}, err
	}
	stored, exists := workspace.bslNavigation.modules[relative]
	if !exists {
		return BSLHelpItem{}, ErrBSLHelpNotFound
	}
	current := buildBSLSemanticModule(relative, source, stored.name, stored.public, stored.predefined)
	if target, found := resolveBSLSymbolAt(workspace.bslNavigation, current, offset); found {
		if target.path == relative && (target.kind == "function" || target.kind == "procedure") {
			if item, ok := projectRoutineHelp(current, parsed, tokens, source, target); ok {
				return item, nil
			}
		}
		if document, ok := index.bySymbol[target.key]; ok {
			return cloneBSLHelpItem(index.documents[document].item), nil
		}
		if target.kind == "metadata" || target.kind == "enum-value" || target.kind == "module" {
			return generatedSymbolHelp(target), nil
		}
		return BSLHelpItem{}, ErrBSLHelpNotFound
	}

	path := bslHelpTokenPath(tokens, offset)
	for start := 0; start < len(path); start++ {
		if item, ok := index.firstAlias(strings.Join(path[start:], ".")); ok {
			return item, nil
		}
	}
	return BSLHelpItem{}, ErrBSLHelpNotFound
}

func (workspace *Workspace) ensureBSLHelpIndexLocked() (*bslHelpIndex, error) {
	if workspace.bslNavigation == nil {
		index, err := workspace.buildBSLNavigationIndex()
		if err != nil {
			return nil, err
		}
		workspace.bslNavigation = index
	}
	if workspace.bslHelp == nil {
		index, err := workspace.buildBSLHelpIndex(workspace.bslNavigation)
		if err != nil {
			return nil, err
		}
		workspace.bslHelp = index
	}
	return workspace.bslHelp, nil
}

func (workspace *Workspace) buildBSLHelpIndex(navigation *bslNavigationIndex) (*bslHelpIndex, error) {
	result := &bslHelpIndex{byAlias: make(map[string][]int), bySymbol: make(map[string]int), truncated: navigation.truncated}
	items, err := platformBSLHelp()
	if err != nil {
		return nil, err
	}
	for _, document := range items {
		result.add(document, "")
	}
	paths := make([]string, 0, len(navigation.modules))
	for path := range navigation.modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if result.full || len(result.documents) >= maxBSLHelpItems {
			result.truncated = true
			break
		}
		module := navigation.modules[path]
		file, readErr := workspace.readSource(path)
		if readErr != nil {
			continue
		}
		parsed, tokens, _ := syntax.ParseWithTokens(path, file.Content)
		for _, routine := range parsed.Routines {
			if result.full || len(result.documents) >= maxBSLHelpItems {
				result.truncated = true
				break
			}
			declaration, ok := findModuleDeclaration(module, routine.Name, false, true)
			if !ok {
				continue
			}
			target := declarationBSLSymbol(module, declaration)
			if editorRange(routineDeclarationSpan(tokens, routine)) != target.definition.Range {
				continue
			}
			item := projectRoutineHelpItem(module, routine, file.Content, target)
			aliases := []string{routine.Name}
			if module.public {
				aliases = append(aliases, module.name+"."+routine.Name)
			}
			result.add(bslHelpDocument{item: item, aliases: aliases}, target.key)
		}
	}
	return result, nil
}

func (index *bslHelpIndex) add(document bslHelpDocument, symbol string) {
	if document.item.ID == "" || document.item.Name == "" {
		index.truncated = true
		return
	}
	if index.full || len(index.documents) >= maxBSLHelpItems {
		index.full = true
		index.truncated = true
		return
	}
	document.aliases = append(document.aliases, document.item.Name)
	search := append([]string{
		document.item.Name, document.item.Kind, document.item.Signature, document.item.Summary, document.item.Description,
	}, document.aliases...)
	search = append(search, document.item.Returns)
	for _, parameter := range document.item.Parameters {
		search = append(search, parameter.Label, parameter.Description)
	}
	document.searchText = strings.ToLower(strings.Join(search, "\n"))
	textBytes := len(document.searchText) + len(document.item.ID) + len(document.item.Source)
	for _, alias := range document.aliases {
		textBytes += len(alias)
	}
	if index.textBytes+textBytes > maxBSLHelpTextBytes {
		index.full = true
		index.truncated = true
		return
	}
	index.textBytes += textBytes
	position := len(index.documents)
	index.documents = append(index.documents, document)
	seen := make(map[string]bool)
	for _, alias := range document.aliases {
		key := strings.ToLower(strings.TrimSpace(alias))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		index.byAlias[key] = append(index.byAlias[key], position)
	}
	if symbol != "" {
		index.bySymbol[symbol] = position
	}
}

func (index *bslHelpIndex) firstAlias(alias string) (BSLHelpItem, bool) {
	positions := index.byAlias[strings.ToLower(alias)]
	if len(positions) == 0 {
		return BSLHelpItem{}, false
	}
	return cloneBSLHelpItem(index.documents[positions[0]].item), true
}

func (index *bslHelpIndex) search(query string) BSLHelpSearchResult {
	foldedQuery := strings.ToLower(query)
	tokens := strings.Fields(foldedQuery)
	type match struct{ position, score int }
	matches := make([]match, 0, 32)
	for position, document := range index.documents {
		matched := true
		for _, token := range tokens {
			if !strings.Contains(document.searchText, token) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		name := strings.ToLower(document.item.Name)
		score := 3
		if name == foldedQuery {
			score = 0
		} else if strings.HasPrefix(name, foldedQuery) {
			score = 1
		} else if strings.Contains(name, foldedQuery) {
			score = 2
		}
		matches = append(matches, match{position: position, score: score})
	}
	sort.SliceStable(matches, func(left, right int) bool {
		if matches[left].score != matches[right].score {
			return matches[left].score < matches[right].score
		}
		leftItem, rightItem := index.documents[matches[left].position].item, index.documents[matches[right].position].item
		if !strings.EqualFold(leftItem.Name, rightItem.Name) {
			return strings.ToLower(leftItem.Name) < strings.ToLower(rightItem.Name)
		}
		return leftItem.ID < rightItem.ID
	})
	result := BSLHelpSearchResult{Items: make([]BSLHelpItem, 0, min(len(matches), maxBSLHelpResults)), Truncated: index.truncated || len(matches) > maxBSLHelpResults}
	for _, value := range matches[:min(len(matches), maxBSLHelpResults)] {
		result.Items = append(result.Items, cloneBSLHelpItem(index.documents[value.position].item))
	}
	return result
}

func platformBSLHelp() ([]bslHelpDocument, error) {
	catalog, err := spec.Load()
	if err != nil {
		return nil, err
	}
	result := make([]bslHelpDocument, 0, len(catalog.Keywords)+len(platformHelpDefinitions))
	for _, keyword := range catalog.Keywords {
		summary := "Ключевое слово BSL."
		switch keyword.Category {
		case "declaration":
			summary = "Ключевое слово объявления BSL."
		case "control":
			summary = "Ключевое слово управления выполнением BSL."
		case "async":
			summary = "Ключевое слово асинхронного выполнения BSL."
		case "literal":
			summary = "Литерал встроенного типа BSL."
		}
		result = append(result, bslHelpDocument{item: BSLHelpItem{
			ID: "platform:keyword:" + keyword.ID, Name: keyword.Russian, Kind: "keyword",
			Signature: keyword.Russian + " / " + keyword.English, Summary: summary, Source: "Платформа ML",
		}, aliases: []string{keyword.Russian, keyword.English}})
	}
	for _, definition := range platformHelpDefinitions {
		parameters := make([]BSLHelpParameter, len(definition.parameters))
		for index, parameter := range definition.parameters {
			parameters[index] = BSLHelpParameter{Label: parameter}
		}
		result = append(result, bslHelpDocument{item: BSLHelpItem{
			ID: "platform:" + definition.id, Name: definition.name, Kind: definition.kind, Signature: definition.signature,
			Summary: definition.summary, Description: definition.description, Parameters: parameters, Returns: definition.returns, Source: "Платформа ML",
		}, aliases: definition.aliases})
	}
	return result, nil
}

var platformHelpDefinitions = []platformHelpDefinition{
	{id: "function/error-description", name: "ОписаниеОшибки", kind: "function", signature: "ОписаниеОшибки() / ErrorDescription()", summary: "Возвращает описание текущей ошибки.", returns: "Строка; вне блока Исключение — пустая строка.", aliases: []string{"ОписаниеОшибки", "ErrorDescription"}},
	{id: "function/transaction-active", name: "ТранзакцияАктивна", kind: "function", signature: "ТранзакцияАктивна() / TransactionActive()", summary: "Проверяет наличие активной транзакции текущего сеанса.", returns: "Булево.", aliases: []string{"ТранзакцияАктивна", "TransactionActive"}},
	{id: "procedure/begin-transaction", name: "НачатьТранзакцию", kind: "procedure", signature: "НачатьТранзакцию() / BeginTransaction()", summary: "Начинает транзакцию текущего сеанса.", aliases: []string{"НачатьТранзакцию", "BeginTransaction"}},
	{id: "procedure/commit-transaction", name: "ЗафиксироватьТранзакцию", kind: "procedure", signature: "ЗафиксироватьТранзакцию() / CommitTransaction()", summary: "Фиксирует активную транзакцию.", aliases: []string{"ЗафиксироватьТранзакцию", "CommitTransaction"}},
	{id: "procedure/rollback-transaction", name: "ОтменитьТранзакцию", kind: "procedure", signature: "ОтменитьТранзакцию() / RollbackTransaction()", summary: "Отменяет активную транзакцию.", aliases: []string{"ОтменитьТранзакцию", "RollbackTransaction"}},
	{id: "constructor/array", name: "Массив", kind: "constructor", signature: "Новый Массив(Размер1 = 0, ...) / New Array(Dimension1 = 0, ...)", summary: "Создаёт индексируемую коллекцию значений.", aliases: []string{"Массив", "Array"}, parameters: []string{"Размер1 = 0", "..."}, returns: "Массив."},
	{id: "constructor/structure", name: "Структура", kind: "constructor", signature: "Новый Структура(ИменаСвойств = \"\", ...) / New Structure(PropertyNames = \"\", ...)", summary: "Создаёт коллекцию именованных свойств.", aliases: []string{"Структура", "Structure"}, parameters: []string{"ИменаСвойств = \"\"", "..."}, returns: "Структура."},
	{id: "constructor/map", name: "Соответствие", kind: "constructor", signature: "Новый Соответствие() / New Map()", summary: "Создаёт коллекцию пар ключ–значение.", aliases: []string{"Соответствие", "Map"}, returns: "Соответствие."},
	{id: "constructor/value-list", name: "СписокЗначений", kind: "constructor", signature: "Новый СписокЗначений() / New ValueList()", summary: "Создаёт список значений.", aliases: []string{"СписокЗначений", "ValueList"}, returns: "СписокЗначений."},
	{id: "constructor/value-table", name: "ТаблицаЗначений", kind: "constructor", signature: "Новый ТаблицаЗначений() / New ValueTable()", summary: "Создаёт табличную коллекцию значений.", aliases: []string{"ТаблицаЗначений", "ValueTable"}, returns: "ТаблицаЗначений."},
	{id: "constructor/query", name: "Запрос", kind: "constructor", signature: "Новый Запрос(Текст = \"\") / New Query(Text = \"\")", summary: "Создаёт запрос к данным ML Project.", aliases: []string{"Запрос", "Query"}, parameters: []string{"Текст = \"\""}, returns: "Запрос."},
	{id: "constructor/temp-tables-manager", name: "МенеджерВременныхТаблиц", kind: "constructor", signature: "Новый МенеджерВременныхТаблиц() / New TempTablesManager()", summary: "Создаёт область жизни временных таблиц запросов.", description: "Таблицы уничтожаются при Закрыть() либо после освобождения последней BSL-ссылки на менеджер.", aliases: []string{"МенеджерВременныхТаблиц", "TempTablesManager"}, returns: "МенеджерВременныхТаблиц."},
	{id: "constructor/data-lock", name: "БлокировкаДанных", kind: "constructor", signature: "Новый БлокировкаДанных() / New DataLock()", summary: "Создаёт описание управляемой блокировки данных.", aliases: []string{"БлокировкаДанных", "DataLock"}, returns: "БлокировкаДанных."},
	{id: "system-enum/lock-mode", name: "РежимБлокировкиДанных", kind: "system-enum", signature: "РежимБлокировкиДанных.Исключительный | Разделяемый", summary: "Режим управляемой блокировки данных.", aliases: []string{"РежимБлокировкиДанных", "DataLockMode", "РежимБлокировкиДанных.Исключительный", "РежимБлокировкиДанных.Разделяемый", "DataLockMode.Exclusive", "DataLockMode.Shared"}},
	{id: "system-enum/movement-kind", name: "ВидДвиженияНакопления", kind: "system-enum", signature: "ВидДвиженияНакопления.Приход | Расход", summary: "Вид движения регистра накопления.", aliases: []string{"ВидДвиженияНакопления", "AccumulationMovementKind", "ВидДвиженияНакопления.Приход", "ВидДвиженияНакопления.Расход", "AccumulationMovementKind.Receipt", "AccumulationMovementKind.Expense"}},
	{id: "system-enum/document-write-mode", name: "РежимЗаписиДокумента", kind: "system-enum", signature: "РежимЗаписиДокумента.Запись | Проведение | ОтменаПроведения", summary: "Определяет операцию записи документа.", aliases: []string{"РежимЗаписиДокумента", "DocumentWriteMode", "РежимЗаписиДокумента.Запись", "РежимЗаписиДокумента.Проведение", "РежимЗаписиДокумента.ОтменаПроведения", "DocumentWriteMode.Write", "DocumentWriteMode.Post", "DocumentWriteMode.UndoPosting"}},
	{id: "system-enum/document-posting-mode", name: "РежимПроведенияДокумента", kind: "system-enum", signature: "РежимПроведенияДокумента.Неоперативный | Оперативный", summary: "Определяет режим проведения документа.", aliases: []string{"РежимПроведенияДокумента", "DocumentPostingMode", "РежимПроведенияДокумента.Неоперативный", "РежимПроведенияДокумента.Оперативный", "DocumentPostingMode.Regular", "DocumentPostingMode.RealTime"}},
	{id: "method/create-item", name: "СоздатьЭлемент", kind: "method", signature: "Справочники.<Имя>.СоздатьЭлемент() / Catalogs.<Name>.CreateItem()", summary: "Создаёт новый объект справочника в памяти.", aliases: []string{"СоздатьЭлемент", "CreateItem"}, returns: "Объект справочника."},
	{id: "method/create-document", name: "СоздатьДокумент", kind: "method", signature: "Документы.<Имя>.СоздатьДокумент() / Documents.<Name>.CreateDocument()", summary: "Создаёт новый объект документа в памяти.", aliases: []string{"СоздатьДокумент", "CreateDocument"}, returns: "Объект документа."},
	{id: "method/get-object", name: "ПолучитьОбъект", kind: "method", signature: "<Менеджер>.ПолучитьОбъект(Ссылка) / <Manager>.GetObject(Reference)", summary: "Загружает изменяемый объект по ссылке.", aliases: []string{"ПолучитьОбъект", "GetObject"}, parameters: []string{"Ссылка"}, returns: "Объект данных."},
	{id: "method/find-by-code", name: "НайтиПоКоду", kind: "method", signature: "Справочники.<Имя>.НайтиПоКоду(Код) / Catalogs.<Name>.FindByCode(Code)", summary: "Ищет элемент справочника по коду.", aliases: []string{"НайтиПоКоду", "FindByCode"}, parameters: []string{"Код"}, returns: "Ссылка или пустая ссылка."},
	{id: "method/find-by-number", name: "НайтиПоНомеру", kind: "method", signature: "Документы.<Имя>.НайтиПоНомеру(Номер, Период = Неопределено)", summary: "Ищет документ по номеру и необязательному периоду.", aliases: []string{"НайтиПоНомеру", "FindByNumber"}, parameters: []string{"Номер", "Период = Неопределено"}, returns: "Ссылка или пустая ссылка."},
	{id: "method/get-reference", name: "ПолучитьСсылку", kind: "method", signature: "<Менеджер>.ПолучитьСсылку(Идентификатор) / <Manager>.GetRef(Identifier)", summary: "Возвращает ссылку с заданным внутренним идентификатором.", aliases: []string{"ПолучитьСсылку", "GetRef", "GetReference"}, parameters: []string{"Идентификатор"}, returns: "Ссылка объекта."},
	{id: "method/create-record-set", name: "СоздатьНаборЗаписей", kind: "method", signature: "<Регистр>.СоздатьНаборЗаписей() / <Register>.CreateRecordSet()", summary: "Создаёт набор записей регистра.", aliases: []string{"СоздатьНаборЗаписей", "CreateRecordSet"}, returns: "Набор записей регистра."},
	{id: "method/slice-last", name: "СрезПоследних", kind: "method", signature: "РегистрыСведений.<Имя>.СрезПоследних(Период, Отбор = Неопределено)", summary: "Получает последние записи регистра сведений на момент времени.", aliases: []string{"СрезПоследних", "SliceLast"}, parameters: []string{"Период", "Отбор = Неопределено"}, returns: "ТаблицаЗначений."},
	{id: "method/slice-first", name: "СрезПервых", kind: "method", signature: "РегистрыСведений.<Имя>.СрезПервых(Период, Отбор = Неопределено)", summary: "Получает первые записи регистра сведений после момента времени.", aliases: []string{"СрезПервых", "SliceFirst"}, parameters: []string{"Период", "Отбор = Неопределено"}, returns: "ТаблицаЗначений."},
	{id: "method/balances", name: "Остатки", kind: "method", signature: "РегистрыНакопления.<Имя>.Остатки(Период, Отбор = Неопределено)", summary: "Получает остатки регистра накопления.", aliases: []string{"Остатки", "Balances"}, parameters: []string{"Период", "Отбор = Неопределено"}, returns: "ТаблицаЗначений."},
	{id: "method/turnovers", name: "Обороты", kind: "method", signature: "РегистрыНакопления.<Имя>.Обороты(НачалоПериода, КонецПериода, Отбор = Неопределено)", summary: "Получает обороты регистра накопления за период.", aliases: []string{"Обороты", "Turnovers"}, parameters: []string{"НачалоПериода", "КонецПериода", "Отбор = Неопределено"}, returns: "ТаблицаЗначений."},
	{id: "method/balances-and-turnovers", name: "ОстаткиИОбороты", kind: "method", signature: "РегистрыНакопления.<Имя>.ОстаткиИОбороты(НачалоПериода, КонецПериода, Отбор = Неопределено)", summary: "Получает остатки и обороты регистра накопления за период.", aliases: []string{"ОстаткиИОбороты", "BalancesAndTurnovers"}, parameters: []string{"НачалоПериода", "КонецПериода", "Отбор = Неопределено"}, returns: "ТаблицаЗначений."},
	{id: "method/get", name: "Получить", kind: "method", signature: "Константы.<Имя>.Получить() / Constants.<Name>.Get()", summary: "Получает значение константы.", aliases: []string{"Получить", "Get"}, returns: "Значение константы."},
	{id: "method/set", name: "Установить", kind: "method", signature: "Константы.<Имя>.Установить(Значение) / Constants.<Name>.Set(Value)", summary: "Устанавливает значение константы.", aliases: []string{"Установить", "Set"}, parameters: []string{"Значение"}},
	{id: "directive/at-client", name: "НаКлиенте", kind: "directive", signature: "&НаКлиенте / &AtClient", summary: "Выполняет процедуру или функцию на клиенте.", aliases: []string{"НаКлиенте", "AtClient"}},
	{id: "directive/at-server", name: "НаСервере", kind: "directive", signature: "&НаСервере / &AtServer", summary: "Выполняет процедуру или функцию на сервере с контекстом.", aliases: []string{"НаСервере", "AtServer"}},
	{id: "directive/at-server-no-context", name: "НаСервереБезКонтекста", kind: "directive", signature: "&НаСервереБезКонтекста / &AtServerNoContext", summary: "Выполняет процедуру или функцию на сервере без контекста формы.", aliases: []string{"НаСервереБезКонтекста", "AtServerNoContext"}},
	{id: "directive/at-client-server", name: "НаКлиентеНаСервере", kind: "directive", signature: "&НаКлиентеНаСервере / &AtClientAtServer", summary: "Разрешает выполнение процедуры или функции на клиенте и сервере.", aliases: []string{"НаКлиентеНаСервере", "AtClientAtServer"}},
	{id: "directive/at-client-server-no-context", name: "НаКлиентеНаСервереБезКонтекста", kind: "directive", signature: "&НаКлиентеНаСервереБезКонтекста / &AtClientAtServerNoContext", summary: "Разрешает выполнение на клиенте и сервере без серверного контекста формы.", aliases: []string{"НаКлиентеНаСервереБезКонтекста", "AtClientAtServerNoContext"}},
}

func projectRoutineHelp(module bslSemanticModule, parsed *syntax.Module, tokens []syntax.Token, source string, target bslResolvedSymbol) (BSLHelpItem, bool) {
	for _, routine := range parsed.Routines {
		declaration := routineDeclarationSpan(tokens, routine)
		if !strings.EqualFold(routine.Name, target.name) || editorRange(declaration) != target.definition.Range {
			continue
		}
		return projectRoutineHelpItem(module, routine, source, target), true
	}
	return BSLHelpItem{}, false
}

func projectRoutineHelpItem(module bslSemanticModule, routine *syntax.Routine, source string, target bslResolvedSymbol) BSLHelpItem {
	parameters := make([]BSLHelpParameter, 0, len(routine.Parameters))
	for _, parameter := range routine.Parameters {
		label := parameter.Name
		if parameter.SourceSpan.Start.Offset >= 0 && parameter.SourceSpan.End.Offset <= len(source) && parameter.SourceSpan.End.Offset > parameter.SourceSpan.Start.Offset {
			label = strings.TrimSpace(source[parameter.SourceSpan.Start.Offset:parameter.SourceSpan.End.Offset])
		}
		parameters = append(parameters, BSLHelpParameter{Label: label})
	}
	kind := "Процедура"
	if routine.Function {
		kind = "Функция"
	}
	documentation := parseRoutineDocumentation(source, routine.SourceSpan.Start.Line, parameters)
	parameters = documentation.parameters
	summary := documentation.summary
	if summary == "" {
		summary = kind + " ML Project."
	}
	location := target.definition
	return BSLHelpItem{
		ID: "project:" + target.key, Name: routine.Name, Kind: target.kind,
		Signature: kind + " " + routine.Name + "(" + strings.Join(parameterLabels(parameters), ", ") + ")",
		Summary:   summary, Description: documentation.description, Parameters: parameters, Returns: documentation.returns,
		Source: "ML Project · " + module.name, Location: &location,
	}
}

type routineDocumentation struct {
	summary, description, returns string
	parameters                    []BSLHelpParameter
}

func parseRoutineDocumentation(source string, declarationLine int, parameters []BSLHelpParameter) routineDocumentation {
	lines := splitSourceLines(source)
	line := declarationLine - 2
	for line >= 0 && (strings.HasPrefix(strings.TrimSpace(lines[line]), "&") || strings.HasPrefix(strings.TrimSpace(lines[line]), "@")) {
		line--
	}
	var reversed []string
	bytes := 0
	for ; line >= 0; line-- {
		trimmed := strings.TrimSpace(lines[line])
		if !strings.HasPrefix(trimmed, "//") {
			break
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		bytes += len(value)
		if bytes > maxBSLHelpCommentSize {
			break
		}
		reversed = append(reversed, value)
	}
	comments := make([]string, len(reversed))
	for index := range reversed {
		comments[len(reversed)-1-index] = reversed[index]
	}
	result := routineDocumentation{parameters: append([]BSLHelpParameter(nil), parameters...)}
	if len(comments) == 0 {
		return result
	}
	section := "description"
	var description, returns []string
	parameterDescriptions := make(map[string]string)
	for _, value := range comments {
		heading := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(value, ":")))
		switch heading {
		case "параметры", "parameters":
			section = "parameters"
			continue
		case "возвращаемое значение", "возвращает", "returns", "return value":
			section = "returns"
			continue
		}
		switch section {
		case "parameters":
			name, text, ok := splitDocumentationValue(value)
			if ok {
				parameterDescriptions[strings.ToLower(name)] = text
			}
		case "returns":
			returns = append(returns, value)
		default:
			description = append(description, value)
		}
	}
	for index := range result.parameters {
		name := documentationParameterName(result.parameters[index].Label)
		result.parameters[index].Description = parameterDescriptions[strings.ToLower(name)]
	}
	result.description = strings.TrimSpace(strings.Join(description, "\n"))
	result.summary = firstDocumentationParagraph(description)
	result.returns = strings.TrimSpace(strings.Join(returns, "\n"))
	return result
}

func documentationParameterName(label string) string {
	beforeDefault, _, _ := strings.Cut(label, "=")
	fields := strings.Fields(beforeDefault)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func splitDocumentationValue(value string) (string, string, bool) {
	for _, separator := range []string{" - ", " — ", " – "} {
		if position := strings.Index(value, separator); position > 0 {
			return strings.TrimSpace(value[:position]), strings.TrimSpace(value[position+len(separator):]), true
		}
	}
	return "", "", false
}

func firstDocumentationParagraph(lines []string) string {
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(result) != 0 {
				break
			}
			continue
		}
		result = append(result, strings.TrimSpace(line))
	}
	return strings.Join(result, " ")
}

func parameterLabels(parameters []BSLHelpParameter) []string {
	result := make([]string, len(parameters))
	for index := range parameters {
		result[index] = parameters[index].Label
	}
	return result
}

func generatedSymbolHelp(target bslResolvedSymbol) BSLHelpItem {
	location := target.definition
	summary := "Объект метаданных ML Project."
	if target.kind == "enum-value" {
		summary = "Значение перечисления ML Project."
	} else if target.kind == "module" {
		summary = "Общий модуль ML Project."
	}
	return BSLHelpItem{ID: "project:" + target.key, Name: target.name, Kind: target.kind, Summary: summary, Source: "ML Project", Location: &location}
}

func bslHelpTokenPath(tokens []syntax.Token, offset int) []string {
	for index, token := range tokens {
		if token.Kind == syntax.EOF || offset < token.Span.Start.Offset || offset > token.Span.End.Offset {
			continue
		}
		if token.Kind.IsKeyword() {
			return []string{token.Lexeme}
		}
		if token.Kind != syntax.Identifier {
			return nil
		}
		start, end := index, index
		for start >= 2 && (tokens[start-1].Kind == syntax.Dot || tokens[start-1].Kind == syntax.DotTrailing) && tokens[start-2].Kind == syntax.Identifier {
			start -= 2
		}
		for end+2 < len(tokens) && (tokens[end+1].Kind == syntax.Dot || tokens[end+1].Kind == syntax.DotTrailing) && tokens[end+2].Kind == syntax.Identifier {
			end += 2
		}
		result := make([]string, 0, (end-start)/2+1)
		for position := start; position <= end; position += 2 {
			result = append(result, tokens[position].Value)
		}
		return result
	}
	return nil
}

func cloneBSLHelpItem(value BSLHelpItem) BSLHelpItem {
	value.Parameters = append([]BSLHelpParameter(nil), value.Parameters...)
	if value.Location != nil {
		location := *value.Location
		value.Location = &location
	}
	return value
}
