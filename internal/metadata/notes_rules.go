package metadata

import (
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// The kinds of note found in a loaded catalog. Each is a state an earlier
// iteration of block 2 stopped refusing; the comment beside the old refusal's
// place says why, and the description below says it for the report.
const (
	NoteUnresolvedPath         NoteKind = "unresolved-path"
	NoteUnusedBound            NoteKind = "unused-bound"
	NoteFillingNotHeld         NoteKind = "filling-not-held"
	NoteValueOfVanishedType    NoteKind = "value-of-vanished-type"
	NoteInactiveHierarchy      NoteKind = "inactive-hierarchy"
	NoteFolderUseWithoutFolds  NoteKind = "folder-use-without-folders"
	NoteFieldOutsideIndex      NoteKind = "field-outside-full-text-index"
	NoteFieldOutsideHistory    NoteKind = "field-outside-data-history"
	NoteInactivePosting        NoteKind = "inactive-posting-settings"
	NotePictureWithoutPicture  NoteKind = "picture-without-picture"
	NoteParameterUseNoType     NoteKind = "parameter-use-without-type"
	NoteLevelOfHierarchyTable  NoteKind = "level-of-hierarchical-table"
	NoteFixedLengthOfNumber    NoteKind = "fixed-length-of-number"
	NotePredefinedCodeKept     NoteKind = "predefined-code-without-code"
	NoteIncompleteAddressing   NoteKind = "incomplete-addressing"
	NoteUnfilledCharacteristic NoteKind = "unfilled-characteristic"
	NoteChoiceSetAndLinked     NoteKind = "choice-parameter-set-and-linked"
)

func init() {
	noteKinds = append(noteKinds,
		NoteKindInfo{NoteUnresolvedPath,
			"Связь параметров выбора или связь по типу записана путём, который не ведёт ни к одному полю объекта: " +
				"по идентификатору чужого объекта или числом. Так остаётся связь реквизита, скопированного из другого объекта.",
			"Путь несётся как записан, связь ничего не отбирает."},
		NoteKindInfo{NoteUnusedBound,
			"Минимальное или максимальное значение у поля, которое не хранит одно число, или текст, который не читается числом. " +
				"Конфигуратор задаёт их только числу; у других типов это след смены типа.",
			"Граница несётся как записана и ничего не ограничивает."},
		NoteKindInfo{NoteFillingNotHeld,
			"Значение заполнения типа, которого поле не хранит, — след смены типа поля (пустая ссылка у строкового поля).",
			"Значение несётся как записано, поле при создании ничем не заполняется."},
		NoteKindInfo{NoteValueOfVanishedType,
			"Значение — ссылка на тип, которого в конфигурации больше нет: записано идентификатором типа.",
			"Значение несётся как записано; в заполнении ничего не заполняет, в параметре выбора отбирает по несуществующему значению."},
		NoteKindInfo{NoteInactiveHierarchy,
			"Настройки иерархии, которые не действуют: вид иерархии, число уровней и «группы сверху» у плоского справочника, " +
				"число уровней без ограничения, «группы сверху» у иерархии элементов. Конфигуратор пишет их на каждом объекте " +
				"значениями диалога.",
			"Настройка несётся как записана и читается, только когда включено то, от чего она зависит."},
		NoteKindInfo{NoteFolderUseWithoutFolds,
			"Использование «для группы и элемента» у объекта, у которого групп нет: след выключенных групп.",
			"Несётся как записано и действует как «для элемента»."},
		NoteKindInfo{NoteFieldOutsideIndex,
			"Поле отмечено для полнотекстового поиска, а объект в полнотекстовый индекс не входит.",
			"Отметка несётся как записана и ничего не включает, пока объект не войдёт в индекс."},
		NoteKindInfo{NoteFieldOutsideHistory,
			"Поле отмечено для истории данных, а у объекта история данных не используется.",
			"Отметка несётся как записана и ничего не включает; история данных в ML не ведётся вовсе."},
		NoteKindInfo{NoteInactivePosting,
			"Оперативное проведение и удаление движений у документа, проведение которого запрещено: " +
				"в конфигураторе эти свойства неактивны, а выгрузка пишет их всегда.",
			"Свойство несётся как записано и не действует."},
		NoteKindInfo{NotePictureWithoutPicture,
			"Команда или группа команд отображается картинкой, а картинки у неё нет.",
			"Отображение несётся как записано, рисуется текст — заголовок команды."},
		NoteKindInfo{NoteParameterUseNoType,
			"Режим использования параметра у команды без типа параметра: конфигуратор пишет режим на каждой команде.",
			"Режим несётся как записан и ничего не значит, пока у команды нет типа параметра."},
		NoteKindInfo{NoteLevelOfHierarchyTable,
			"Номер уровня у иерархической таблицы измерения куба внешнего источника, отличный от нуля: справка даёт такой " +
				"таблице уровень 0, а конфигуратор сохраняет записанный.",
			"Номер несётся как записан, приложение видит уровень 0."},
		NoteKindInfo{NoteFixedLengthOfNumber,
			"«Допустимая длина: фиксированная» у числового кода или номера — след времени, когда код был строкой. " +
				"Справка: свойство имеет смысл для строки.",
			"Настройка несётся как записана и ни на что не влияет: колонка остаётся числом."},
		NoteKindInfo{NotePredefinedCodeKept,
			"Код предопределённого элемента у объекта, длина кода которого 0: код, оставшийся с тех пор, как код был.",
			"Код несётся как записан; элемент создаётся без кода — поля для него нет."},
		NoteKindInfo{NoteIncompleteAddressing,
			"Адресация задачи заполнена наполовину: регистр без реквизитов адресации, реквизиты без регистра или без " +
				"основного реквизита, измерение без регистра. Конфигуратор такое сохраняет (mdclasses).",
			"Адресация несётся как записана и исполнителя не определяет."},
		NoteKindInfo{NoteUnfilledCharacteristic,
			"Характеристика с выбранными таблицами, но без полей ключа, объекта, вида или значения " +
				"(в выгрузке поля записаны как -1).",
			"Характеристика несётся как записана и характеристик объекту не даёт."},
		NoteKindInfo{NoteChoiceSetAndLinked,
			"Один и тот же параметр выбора задан и значением, и связью параметров выбора: два свойства прототипа, " +
				"имя в обоих.",
			"Оба несутся как записаны; какое из них применяет форма, решается вместе с исполнением выбора в формах (блок 7)."},
	)
	noteRules[NoteUnresolvedPath] = noteUnresolvedPath
	noteRules[NoteUnusedBound] = noteUnusedBound
	noteRules[NoteFillingNotHeld] = noteFillingNotHeld
	noteRules[NoteValueOfVanishedType] = noteValueOfVanishedType
	noteRules[NoteInactiveHierarchy] = noteInactiveHierarchy
	noteRules[NoteFolderUseWithoutFolds] = noteFolderUseWithoutFolds
	noteRules[NoteFieldOutsideIndex] = noteFieldOutsideIndex
	noteRules[NoteFieldOutsideHistory] = noteFieldOutsideHistory
	noteRules[NoteInactivePosting] = noteInactivePosting
	noteRules[NotePictureWithoutPicture] = notePictureWithoutPicture
	noteRules[NoteParameterUseNoType] = noteParameterUseNoType
	noteRules[NoteLevelOfHierarchyTable] = noteLevelOfHierarchyTable
	noteRules[NoteFixedLengthOfNumber] = noteFixedLengthOfNumber
	noteRules[NotePredefinedCodeKept] = notePredefinedCodeKept
	noteRules[NoteIncompleteAddressing] = noteIncompleteAddressing
	noteRules[NoteUnfilledCharacteristic] = noteUnfilledCharacteristic
	noteRules[NoteChoiceSetAndLinked] = noteChoiceSetAndLinked
}

func noteUnresolvedPath(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.choice == nil {
			return
		}
		for _, link := range holder.choice.ParameterLinks {
			if link.Source.Unresolved != "" {
				note(holder.where+" choice link "+link.Name, link.Source.Unresolved)
			}
		}
		if link := holder.choice.LinkByType; link != nil && link.Source.Unresolved != "" {
			note(holder.where+" link by type", link.Source.Unresolved)
		}
	})
}

func noteUnusedBound(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.presentation == nil {
			return
		}
		for _, item := range []struct {
			name  string
			bound *string
		}{{"min_value", holder.presentation.MinValue}, {"max_value", holder.presentation.MaxValue}} {
			if item.bound == nil {
				continue
			}
			if _, ok := NumberBound(item.bound, holder.types); !ok {
				note(holder.where+" "+item.name, *item.bound)
			}
		}
	})
}

func noteFillingNotHeld(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.filling == nil || holder.filling.Value == nil {
			return
		}
		value := *holder.filling.Value
		// Неопределено in a filling is «not set», which is what it does.
		if value.Kind == UndefinedValue || value.Kind == UnresolvedReferenceValue {
			return
		}
		if EffectiveFillingValue(&value, holder.types) == nil {
			note(holder.where+" filling", string(value.Kind)+" "+value.Data)
		}
	})
}

func noteValueOfVanishedType(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.filling != nil && holder.filling.Value != nil && holder.filling.Value.Kind == UnresolvedReferenceValue {
			note(holder.where+" filling", holder.filling.Value.Data)
		}
		if holder.choice == nil {
			return
		}
		for _, parameter := range holder.choice.Parameters {
			for _, value := range parameter.Values {
				if value.Kind == UnresolvedReferenceValue {
					note(holder.where+" choice parameter "+parameter.Name, value.Data)
				}
			}
		}
	})
}

func noteInactiveHierarchy(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.hierarchy == nil {
			return
		}
		hierarchy := *holder.hierarchy
		switch {
		case !hierarchy.Enabled && (hierarchy.Kind != "" || hierarchy.FoldersOnTop || hierarchy.LimitLevels || hierarchy.LevelCount != 0):
			note(holder.where+" hierarchy", "settings of a hierarchy that is off")
		case hierarchy.Enabled && hierarchy.Kind == ItemsHierarchy && hierarchy.FoldersOnTop:
			note(holder.where+" hierarchy", "folders on top over a hierarchy of items")
		case hierarchy.Enabled && !hierarchy.LimitLevels && hierarchy.LevelCount != 0:
			note(holder.where+" hierarchy", "a level count with no limit")
		}
	})
}

func noteFolderUseWithoutFolds(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.use == nil || *holder.use != UseForFolderAndItem || holder.object.hierarchy == nil {
			return
		}
		if hierarchy := holder.object.hierarchy; !hierarchy.Enabled || hierarchy.Kind != FoldersAndItemsHierarchy {
			note(holder.where+" use", string(*holder.use))
		}
	})
}

func noteFieldOutsideIndex(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.fullTextSearch != nil && *holder.fullTextSearch == UsageUse &&
			holder.object.fullTextSearch != nil && *holder.object.fullTextSearch != FullTextSearchUse {
			note(holder.where+" full_text_search", string(*holder.fullTextSearch))
		}
	})
}

func noteFieldOutsideHistory(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.dataHistory != nil && *holder.dataHistory == UsageUse &&
			holder.object.dataHistory != nil && *holder.object.dataHistory != DataHistoryUse {
			note(holder.where+" data_history", string(*holder.dataHistory))
		}
	})
}

func noteInactivePosting(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.posting == nil || holder.posting.Allowed {
			return
		}
		if holder.posting.RealTime != "" {
			note(holder.where+" posting real_time", string(holder.posting.RealTime))
		}
		if holder.posting.RecordsDeletion != "" {
			note(holder.where+" posting records_deletion", string(holder.posting.RecordsDeletion))
		}
	})
}

func notePictureWithoutPicture(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.representation != nil && holder.representation.ShownAs(holder.picture) != *holder.representation {
			note(holder.where+" representation", string(*holder.representation))
		}
	})
}

func noteParameterUseNoType(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.parameterUse != nil && *holder.parameterUse != "" && holder.parameter != nil && len(*holder.parameter) == 0 {
			note(holder.where+" parameter_use", string(*holder.parameterUse))
		}
	})
}

func noteLevelOfHierarchyTable(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if table := holder.dimensionTable; table != nil && table.LevelNumber != table.EffectiveLevelNumber() {
			note(holder.where+" level_number", strconv.Itoa(table.LevelNumber))
		}
	})
}

func noteFixedLengthOfNumber(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if code := holder.code; code != nil && code.FixedLength && code.Type != StringType {
			note(holder.where+" fixed_length", string(code.Type))
		}
		if number := holder.number; number != nil && number.FixedLength && number.Type != StringType {
			note(holder.where+" fixed_length", string(number.Type))
		}
	})
}

func notePredefinedCodeKept(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.predefinedCode != nil && *holder.predefinedCode != "" && holder.object.code != nil && holder.object.code.Length == 0 {
			note(holder.where+" code", *holder.predefinedCode)
		}
	})
}

func noteIncompleteAddressing(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		task := holder.task
		if task == nil {
			return
		}
		switch {
		case task.Addressing != nil && len(task.AddressingAttributes) == 0:
			note(holder.where+" addressing", "a register without addressing attributes")
		case task.Addressing == nil && len(task.AddressingAttributes) > 0:
			note(holder.where+" addressing", "addressing attributes without a register")
		case len(task.AddressingAttributes) > 0 && task.MainAddressingAttribute == "":
			note(holder.where+" addressing", "addressing attributes without the main one")
		}
		if task.Addressing == nil {
			for _, attribute := range task.AddressingAttributes {
				if attribute.Dimension != nil {
					note(holder.where+" addressing_attributes "+attribute.Name+" dimension", attribute.Dimension.String())
				}
			}
		}
	})
}

func noteUnfilledCharacteristic(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.characteristic != nil && !holder.characteristic.Filled() {
			note(holder.where, "a characteristic with its fields unnamed")
		}
	})
}

func noteChoiceSetAndLinked(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.choice == nil {
			return
		}
		set := map[string]bool{}
		for _, parameter := range holder.choice.Parameters {
			set[strings.ToLower(parameter.Name)] = true
		}
		for _, link := range holder.choice.ParameterLinks {
			if set[strings.ToLower(link.Name)] {
				note(holder.where+" choice "+link.Name, link.Name)
			}
		}
	})
}

// noteObject is what a rule may need to know about the top-level object a
// holder stands in: the settings a field's own flag depends on.
type noteObject struct {
	hierarchy      *Hierarchy
	fullTextSearch *FullTextSearchMode
	dataHistory    *DataHistoryMode
	code           *CatalogCode
}

// noteHolder is one struct of the model that carries something a rule looks
// at - the parts of a field, a hierarchy, the posting of a document, the
// drawing of a command - with the place it stands in. A part the struct does
// not carry is nil.
type noteHolder struct {
	where          string
	object         noteObject
	types          []Type
	presentation   *FieldPresentation
	choice         *FieldChoice
	filling        *FieldFilling
	use            *AttributeUse
	fullTextSearch *UsageMode
	dataHistory    *UsageMode
	hierarchy      *Hierarchy
	posting        *DocumentPosting
	representation *CommandRepresentation
	picture        *PictureReference
	parameterUse   *CommandParameterUse
	parameter      *[]Type
	dimensionTable *ExternalDimensionTable
	code           *CatalogCode
	number         *DocumentNumber
	predefinedCode *string
	task           *TaskDefinition
	characteristic *ObjectCharacteristic
}

// eachNoteHolder walks every object of the catalog, top level and nested, and
// calls visit for every struct carrying a part some rule looks at.
//
// It walks by reflection on purpose: a new kind of object, or a new place a
// field may stand, is covered the day it is added to the model, and a rule
// cannot miss it because nobody remembered to list it here.
func eachNoteHolder(catalog *Catalog, visit func(noteHolder)) {
	top := reflect.ValueOf(catalog).Elem()
	for index := range top.NumField() {
		field := top.Type().Field(index)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		list := top.Field(index)
		for item := range list.Len() {
			object := list.Index(item)
			where := kebab(field.Name) + " " + nameOf(object)
			walkNoteHolders(object, where, objectContext(object), visit)
		}
	}
}

func objectContext(object reflect.Value) noteObject {
	var context noteObject
	if field := object.FieldByName("Hierarchy"); field.IsValid() && field.Type() == reflect.TypeFor[Hierarchy]() {
		context.hierarchy = field.Addr().Interface().(*Hierarchy)
	}
	if field := object.FieldByName("FullTextSearch"); field.IsValid() && field.Type() == reflect.TypeFor[FullTextSearchMode]() {
		context.fullTextSearch = field.Addr().Interface().(*FullTextSearchMode)
	}
	if field := object.FieldByName("DataHistory"); field.IsValid() && field.Type() == reflect.TypeFor[DataHistoryMode]() {
		context.dataHistory = field.Addr().Interface().(*DataHistoryMode)
	}
	if field := object.FieldByName("Code"); field.IsValid() && field.Type() == reflect.TypeFor[CatalogCode]() {
		context.code = field.Addr().Interface().(*CatalogCode)
	}
	return context
}

func walkNoteHolders(value reflect.Value, where string, object noteObject, visit func(noteHolder)) {
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			walkNoteHolders(value.Elem(), where, object, visit)
		}
		return
	case reflect.Slice:
		if value.Type().Elem().Kind() != reflect.Struct && value.Type().Elem().Kind() != reflect.Pointer {
			return
		}
		for index := range value.Len() {
			element := value.Index(index)
			walkNoteHolders(element, where+" "+nameOf(element), object, visit)
		}
		return
	case reflect.Struct:
	default:
		return
	}
	if !value.CanAddr() {
		copied := reflect.New(value.Type()).Elem()
		copied.Set(value)
		value = copied
	}
	if holder, ok := holderOf(value, where, object); ok {
		visit(holder)
	}
	for index := range value.NumField() {
		field := value.Type().Field(index)
		if !field.IsExported() {
			continue
		}
		child := value.Field(index)
		next := where
		if !field.Anonymous {
			next = where + " " + yamlName(field)
		}
		switch field.Type.Kind() {
		case reflect.Struct, reflect.Pointer, reflect.Slice:
			walkNoteHolders(child, next, object, visit)
		}
	}
}

// holderOf reads the parts a rule looks at out of one struct, and says
// whether there were any.
func holderOf(value reflect.Value, where string, object noteObject) (noteHolder, bool) {
	holder := noteHolder{where: where, object: object}
	found := false
	take := func(name string, kind reflect.Type, into func(reflect.Value)) {
		field, ok := value.Type().FieldByName(name)
		if !ok || field.Type != kind || len(field.Index) != 1 {
			return
		}
		into(value.Field(field.Index[0]).Addr())
		found = true
	}
	take("Presentation", reflect.TypeFor[FieldPresentation](), func(v reflect.Value) { holder.presentation = v.Interface().(*FieldPresentation) })
	take("Choice", reflect.TypeFor[FieldChoice](), func(v reflect.Value) { holder.choice = v.Interface().(*FieldChoice) })
	take("Filling", reflect.TypeFor[FieldFilling](), func(v reflect.Value) { holder.filling = v.Interface().(*FieldFilling) })
	take("Use", reflect.TypeFor[AttributeUse](), func(v reflect.Value) { holder.use = v.Interface().(*AttributeUse) })
	take("FullTextSearch", reflect.TypeFor[UsageMode](), func(v reflect.Value) { holder.fullTextSearch = v.Interface().(*UsageMode) })
	take("DataHistory", reflect.TypeFor[UsageMode](), func(v reflect.Value) { holder.dataHistory = v.Interface().(*UsageMode) })
	take("Hierarchy", reflect.TypeFor[Hierarchy](), func(v reflect.Value) { holder.hierarchy = v.Interface().(*Hierarchy) })
	take("Posting", reflect.TypeFor[DocumentPosting](), func(v reflect.Value) { holder.posting = v.Interface().(*DocumentPosting) })
	take("Representation", reflect.TypeFor[CommandRepresentation](), func(v reflect.Value) { holder.representation = v.Interface().(*CommandRepresentation) })
	take("ParameterUse", reflect.TypeFor[CommandParameterUse](), func(v reflect.Value) { holder.parameterUse = v.Interface().(*CommandParameterUse) })
	take("Parameter", reflect.TypeFor[[]Type](), func(v reflect.Value) { holder.parameter = v.Interface().(*[]Type) })
	switch value.Type() {
	case reflect.TypeFor[ExternalDimensionTable]():
		holder.dimensionTable = value.Addr().Interface().(*ExternalDimensionTable)
		found = true
	case reflect.TypeFor[CatalogCode]():
		holder.code = value.Addr().Interface().(*CatalogCode)
		found = true
	case reflect.TypeFor[DocumentNumber]():
		holder.number = value.Addr().Interface().(*DocumentNumber)
		found = true
	case reflect.TypeFor[TaskDefinition]():
		holder.task = value.Addr().Interface().(*TaskDefinition)
		found = true
	case reflect.TypeFor[ObjectCharacteristic]():
		holder.characteristic = value.Addr().Interface().(*ObjectCharacteristic)
		found = true
	case reflect.TypeFor[PredefinedCatalogItem](), reflect.TypeFor[PredefinedCalculationType](),
		reflect.TypeFor[PredefinedAccount](), reflect.TypeFor[PredefinedCharacteristic]():
		if field := value.FieldByName("Code"); field.IsValid() && field.Kind() == reflect.String {
			holder.predefinedCode = field.Addr().Interface().(*string)
			found = true
		}
	}
	if !found {
		return noteHolder{}, false
	}
	// The types and the picture are not parts a rule fires on - they are what
	// the parts are read against.
	if field, ok := value.Type().FieldByName("Types"); ok && field.Type == reflect.TypeFor[[]Type]() && len(field.Index) == 1 {
		holder.types = value.Field(field.Index[0]).Interface().([]Type)
	}
	if field, ok := value.Type().FieldByName("Picture"); ok && field.Type == reflect.TypeFor[*PictureReference]() && len(field.Index) == 1 {
		holder.picture = value.Field(field.Index[0]).Interface().(*PictureReference)
	}
	return holder, true
}

// nameOf is the name a struct of the model goes by, or nothing.
func nameOf(value reflect.Value) string {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	if field := value.FieldByName("Name"); field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

// yamlName is the key a field is written under in the project's files, which
// is the vocabulary a developer knows the place by.
func yamlName(field reflect.StructField) string {
	if name, _, _ := strings.Cut(field.Tag.Get("yaml"), ","); name != "" && name != "-" {
		return name
	}
	return kebab(field.Name)
}

// kebab turns a Go name into the folder name of its kind: ChartsOfAccounts
// into charts-of-accounts.
func kebab(name string) string {
	var out strings.Builder
	for index, symbol := range name {
		if unicode.IsUpper(symbol) {
			if index > 0 {
				out.WriteByte('-')
			}
			symbol = unicode.ToLower(symbol)
		}
		out.WriteRune(symbol)
	}
	return out.String()
}
