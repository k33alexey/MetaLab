package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The kinds of note found in a loaded catalog. Each is a state an earlier
// iteration of block 2 stopped refusing; the comment beside the old refusal's
// place says why, and the description below says it for the report.
const (
	NoteUnresolvedPath             NoteKind = "unresolved-path"
	NoteUnusedBound                NoteKind = "unused-bound"
	NoteFillingNotHeld             NoteKind = "filling-not-held"
	NoteValueOfVanishedType        NoteKind = "value-of-vanished-type"
	NoteInactiveHierarchy          NoteKind = "inactive-hierarchy"
	NoteFolderUseWithoutFolds      NoteKind = "folder-use-without-folders"
	NoteFieldOutsideIndex          NoteKind = "field-outside-full-text-index"
	NoteFieldOutsideHistory        NoteKind = "field-outside-data-history"
	NoteInactivePosting            NoteKind = "inactive-posting-settings"
	NotePictureWithoutPicture      NoteKind = "picture-without-picture"
	NoteParameterUseNoType         NoteKind = "parameter-use-without-type"
	NoteLevelOfHierarchyTable      NoteKind = "level-of-hierarchical-table"
	NoteFixedLengthOfNumber        NoteKind = "fixed-length-of-number"
	NotePredefinedCodeKept         NoteKind = "predefined-code-without-code"
	NoteIncompleteAddressing       NoteKind = "incomplete-addressing"
	NoteUnfilledCharacteristic     NoteKind = "unfilled-characteristic"
	NoteChoiceSetAndLinked         NoteKind = "choice-parameter-set-and-linked"
	NoteQuickChoiceUnderFormChoice NoteKind = "quick-choice-under-form-choice"
	NoteFullTextInputOutsideIndex  NoteKind = "full-text-input-outside-index"
	NoteExtDimensionsHalfSet       NoteKind = "ext-dimensions-half-set"
	NoteAutoOrderWithoutLength     NoteKind = "auto-order-without-length"
	NoteBaseDependencyHalfSet      NoteKind = "base-dependency-half-set"
	NoteLengthZeroUnchecked        NoteKind = "length-zero-unchecked"
	NoteJournalShowsNothing        NoteKind = "journal-shows-nothing"
	NoteSequenceWithoutDocuments   NoteKind = "sequence-without-documents"
	NoteRecalculationHalfSet       NoteKind = "recalculation-dimension-half-set"
	NoteRegisterWithoutFields      NoteKind = "register-without-fields"
	NoteDuplicateAggregate         NoteKind = "duplicate-aggregate"
	NoteCriterionWithoutType       NoteKind = "filter-criterion-without-type"
	NoteHTTPRootsDifferInCase      NoteKind = "http-roots-differ-in-case"
	NoteUnfilledParentWithoutTree  NoteKind = "unfilled-parent-without-hierarchy"
	NoteObjectTableWithoutKey      NoteKind = "object-table-without-key"
	NoteEmptyNameInSource          NoteKind = "empty-name-in-source"
	NoteAmpersandZero              NoteKind = "expression-ampersand-zero"
	NoteLongExternalString         NoteKind = "long-external-string"
	NoteForeignSourceReference     NoteKind = "foreign-source-reference"
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
		NoteKindInfo{NoteQuickChoiceUnderFormChoice,
			"Быстрый выбор у объекта, выбор которого только из формы: в конфигураторе флаг тогда не действует, а выгрузка его пишет.",
			"Флаг несётся как записан; быстрого выбора нет."},
		NoteKindInfo{NoteFullTextInputOutsideIndex,
			"Полнотекстовый поиск при вводе по строке у объекта, который в полнотекстовый индекс не входит.",
			"Настройка несётся как записана; ввод по строке ищет обычным способом."},
		NoteKindInfo{NoteExtDimensionsHalfSet,
			"Субконто плана счетов заданы наполовину: максимум без плана видов характеристик, план видов характеристик " +
				"при максимуме 0, признаки учёта субконто без плана видов. Конфигуратор такое сохраняет (mdclasses).",
			"Несётся как записано; без плана видов характеристик субконто у счетов нет."},
		NoteKindInfo{NoteAutoOrderWithoutLength,
			"Автопорядок по коду у плана счетов без длины порядка.",
			"Несётся как записан; порядок строится по ширине кода."},
		NoteKindInfo{NoteBaseDependencyHalfSet,
			"Зависимость от базы у плана видов расчёта без базовых планов или базовые планы без зависимости.",
			"Несётся как записано; база собирается из названных базовых планов, без них — пуста."},
		NoteKindInfo{NoteLengthZeroUnchecked,
			"Длина 0 кода, наименования или номера у вида, где её в конфигураторе не проверяли: наименование плана счетов, " +
				"плана видов расчёта и задачи, код и наименование плана обмена, номер нумератора.",
			"Поле выключено, как у справочника с длиной 0: ни колонки, ни поиска по нему."},
		NoteKindInfo{NoteJournalShowsNothing,
			"Журнал документов без регистрируемых документов или графа журнала без ссылок на реквизиты — " +
				"таким журнал бывает сразу после создания.",
			"Журнал или графа несётся как записан и ничего не показывает."},
		NoteKindInfo{NoteSequenceWithoutDocuments,
			"Последовательность без документов.",
			"Несётся как записана и ни за чем не следит."},
		NoteKindInfo{NoteRecalculationHalfSet,
			"Измерение перерасчёта без измерения регистра или без данных ведущих регистров.",
			"Несётся как записано и ничего не пересчитывает; без измерения регистра колонки у него нет."},
		NoteKindInfo{NoteRegisterWithoutFields,
			"Регистр сведений без измерений, ресурсов и реквизитов — таким он бывает сразу после создания.",
			"Несётся как записан: одна пустая запись на период."},
		NoteKindInfo{NoteDuplicateAggregate,
			"Два агрегата регистра накопления с теми же измерениями и той же периодичностью.",
			"Оба несутся как записаны; второй повторяет первый."},
		NoteKindInfo{NoteCriterionWithoutType,
			"Критерий отбора без типа при заполненном составе (sb: 190 полей).",
			"Несётся как записан и ничего не находит."},
		NoteKindInfo{NoteHTTPRootsDifferInCase,
			"Корневые URL двух HTTP-сервисов различаются только регистром букв; различает ли их прототип, не известно.",
			"Оба несутся как записаны; адреса сравниваются как написаны."},
		NoteKindInfo{NoteUnfilledParentWithoutTree,
			"Значение незаполненного родителя у таблицы измерения без иерархии.",
			"Несётся как записано и не читается: родителя без иерархии нет."},
		NoteKindInfo{NoteObjectTableWithoutKey,
			"Таблица объектных данных внешнего источника без поля ключа.",
			"Несётся как записана; ссылок на её строки нет, пока ключ не задан."},
		NoteKindInfo{NoteEmptyNameInSource,
			"Пустое имя в источнике данных у таблицы, поля, куба, таблицы измерения или ресурса: справка его требует, " +
				"конфигуратор сохраняет без него.",
			"Несётся как записано; объект ни к чему в источнике не привязан."},
		NoteKindInfo{NoteAmpersandZero,
			"«&0» в выражении функции внешнего источника: параметры считаются с 1, так что это текст языка источника " +
				"(во многих — побитовое «и»).",
			"Выражение несётся как записано, «&0» не заменяется."},
		NoteKindInfo{NoteLongExternalString,
			"Строка длиннее 1024 у поля внешнего источника; конфигуратор пишет 4294967295 для столбца без предела (mdclasses).",
			"Длина несётся как записана и читается как неограниченная."},
		NoteKindInfo{NoteForeignSourceReference,
			"Поле внешнего источника ссылается на таблицу другого источника.",
			"Ссылка несётся как записана; соединить две базы ML не может, ссылка ничего не находит."},
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
	noteRules[NoteQuickChoiceUnderFormChoice] = noteQuickChoiceUnderFormChoice
	noteRules[NoteFullTextInputOutsideIndex] = noteFullTextInputOutsideIndex
	noteRules[NoteExtDimensionsHalfSet] = noteExtDimensionsHalfSet
	noteRules[NoteAutoOrderWithoutLength] = noteAutoOrderWithoutLength
	noteRules[NoteBaseDependencyHalfSet] = noteBaseDependencyHalfSet
	noteRules[NoteLengthZeroUnchecked] = noteLengthZeroUnchecked
	noteRules[NoteJournalShowsNothing] = noteJournalShowsNothing
	noteRules[NoteSequenceWithoutDocuments] = noteSequenceWithoutDocuments
	noteRules[NoteRecalculationHalfSet] = noteRecalculationHalfSet
	noteRules[NoteRegisterWithoutFields] = noteRegisterWithoutFields
	noteRules[NoteDuplicateAggregate] = noteDuplicateAggregate
	noteRules[NoteCriterionWithoutType] = noteCriterionWithoutType
	noteRules[NoteHTTPRootsDifferInCase] = noteHTTPRootsDifferInCase
	noteRules[NoteUnfilledParentWithoutTree] = noteUnfilledParentWithoutTree
	noteRules[NoteObjectTableWithoutKey] = noteObjectTableWithoutKey
	noteRules[NoteEmptyNameInSource] = noteEmptyNameInSource
	noteRules[NoteAmpersandZero] = noteAmpersandZero
	noteRules[NoteLongExternalString] = noteLongExternalString
	noteRules[NoteForeignSourceReference] = noteForeignSourceReference
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

func noteQuickChoiceUnderFormChoice(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.choiceMode != nil && *holder.choiceMode == ChoiceFromForm && holder.quickChoice != nil && *holder.quickChoice {
			note(holder.where+" quick_choice", "true")
		}
	})
}

func noteFullTextInputOutsideIndex(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.fullTextInput != nil && *holder.fullTextInput == FullTextOnInputUse &&
			holder.object.fullTextSearch != nil && *holder.object.fullTextSearch == FullTextSearchDontUse {
			note(holder.where+" full_text_search_on_input", string(*holder.fullTextInput))
		}
	})
}

func noteExtDimensionsHalfSet(catalog *Catalog, note func(where, written string)) {
	for _, chart := range catalog.ChartsOfAccounts {
		where := "charts-of-accounts " + chart.Name
		switch {
		case chart.ExtDimensionTypes == nil && chart.MaxExtDimensionCount > 0:
			note(where+" max_ext_dimension_count", strconv.Itoa(chart.MaxExtDimensionCount)+" without ext_dimension_types")
		case chart.ExtDimensionTypes != nil && chart.MaxExtDimensionCount == 0:
			note(where+" ext_dimension_types", "with max_ext_dimension_count 0")
		}
		if chart.ExtDimensionTypes == nil && len(chart.ExtDimensionAccountingFlags) > 0 {
			note(where+" ext_dimension_accounting_flags", "without ext_dimension_types")
		}
	}
}

func noteAutoOrderWithoutLength(catalog *Catalog, note func(where, written string)) {
	for _, chart := range catalog.ChartsOfAccounts {
		if chart.AutoOrderByCode && chart.OrderLength == 0 {
			note("charts-of-accounts "+chart.Name+" auto_order_by_code", "without order_length")
		}
	}
}

func noteBaseDependencyHalfSet(catalog *Catalog, note func(where, written string)) {
	for _, chart := range catalog.ChartsOfCalculationTypes {
		where := "charts-of-calculation-types " + chart.Name
		switch {
		case takesABase(chart) && len(chart.BaseCharts) == 0:
			note(where+" base_dependency", string(chart.BaseDependency)+" without base_charts")
		case !takesABase(chart) && len(chart.BaseCharts) > 0:
			note(where+" base_charts", "without base_dependency")
		}
	}
}

func noteLengthZeroUnchecked(catalog *Catalog, note func(where, written string)) {
	for _, chart := range catalog.ChartsOfAccounts {
		if chart.DescriptionLength == 0 {
			note("charts-of-accounts "+chart.Name+" description_length", "0")
		}
	}
	for _, chart := range catalog.ChartsOfCalculationTypes {
		if chart.DescriptionLength == 0 {
			note("charts-of-calculation-types "+chart.Name+" description_length", "0")
		}
	}
	for _, task := range catalog.Tasks {
		if task.DescriptionLength == 0 {
			note("tasks "+task.Name+" description_length", "0")
		}
	}
	for _, numerator := range catalog.Numerators {
		if numerator.Number.Length == 0 {
			note("document-numerators "+numerator.Name+" number.length", "0")
		}
	}
	for _, plan := range catalog.ExchangePlans {
		if plan.Code.Length == 0 {
			note("exchange-plans "+plan.Name+" code.length", "0")
		}
		if plan.DescriptionLength == 0 {
			note("exchange-plans "+plan.Name+" description_length", "0")
		}
	}
}

func noteJournalShowsNothing(catalog *Catalog, note func(where, written string)) {
	for _, journal := range catalog.DocumentJournals {
		where := "document-journals " + journal.Name
		if len(journal.Documents) == 0 {
			note(where+" documents", "none")
		}
		for _, column := range journal.Columns {
			if len(column.References) == 0 {
				note(where+" columns "+column.Name, "no references")
			}
		}
	}
}

func noteSequenceWithoutDocuments(catalog *Catalog, note func(where, written string)) {
	for _, sequence := range catalog.Sequences {
		if len(sequence.Documents) == 0 {
			note("sequences "+sequence.Name+" documents", "none")
		}
	}
}

func noteRecalculationHalfSet(catalog *Catalog, note func(where, written string)) {
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			for _, dimension := range recalculation.Dimensions {
				where := "calculation-registers " + register.Name + " recalculations " + recalculation.Name + " dimensions " + dimension.Name
				if dimension.RegisterDimension.IsZero() {
					note(where+" register_dimension", "none")
				}
				if len(dimension.LeadingData) == 0 {
					note(where+" leading_data", "none")
				}
			}
		}
	}
}

func noteRegisterWithoutFields(catalog *Catalog, note func(where, written string)) {
	for _, register := range catalog.InformationRegisters {
		if len(register.Dimensions)+len(register.Resources)+len(register.Attributes) == 0 {
			note("information-registers "+register.Name, "no fields")
		}
	}
}

func noteDuplicateAggregate(catalog *Catalog, note func(where, written string)) {
	for _, register := range catalog.AccumulationRegisters {
		seen := map[string]int{}
		for index, aggregate := range register.Aggregates {
			folded := make([]string, 0, len(aggregate.Dimensions))
			for _, name := range aggregate.Dimensions {
				folded = append(folded, strings.ToLower(name))
			}
			key := strings.Join(sortedStrings(folded), ",") + "|" + string(aggregate.Periodicity)
			if previous, exists := seen[key]; exists {
				note(fmt.Sprintf("accumulation-registers %s aggregates[%d]", register.Name, index), fmt.Sprintf("the same as aggregates[%d]", previous))
				continue
			}
			seen[key] = index
		}
	}
}

func noteCriterionWithoutType(catalog *Catalog, note func(where, written string)) {
	for _, criterion := range catalog.FilterCriteria {
		if len(criterion.Types) == 0 {
			note("filter-criteria "+criterion.Name+" types", strconv.Itoa(len(criterion.Fields))+" fields")
		}
	}
}

func noteHTTPRootsDifferInCase(catalog *Catalog, note func(where, written string)) {
	seen := map[string]string{}
	for _, service := range catalog.HTTPServices {
		folded := strings.ToLower(service.RootURL)
		if previous, ok := seen[folded]; ok && previous != service.RootURL {
			note("http-services "+service.Name+" root_url", service.RootURL+" and "+previous)
			continue
		}
		seen[folded] = service.RootURL
	}
}

func noteUnfilledParentWithoutTree(catalog *Catalog, note func(where, written string)) {
	for _, source := range catalog.ExternalDataSources {
		for _, cube := range source.Cubes {
			for _, table := range cube.DimensionTables {
				if table.UnfilledParentValue != nil && !table.Hierarchical {
					note("external-data-sources "+source.Name+" cubes "+cube.Name+" dimension tables "+table.Name+" unfilled_parent_value", *table.UnfilledParentValue)
				}
			}
		}
	}
}

func noteObjectTableWithoutKey(catalog *Catalog, note func(where, written string)) {
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			if table.ObjectTable() && len(table.KeyFields) == 0 {
				note("external-data-sources "+source.Name+" tables "+table.Name+" key_fields", "none")
			}
		}
	}
}

func noteEmptyNameInSource(catalog *Catalog, note func(where, written string)) {
	for _, source := range catalog.ExternalDataSources {
		at := "external-data-sources " + source.Name
		fields := func(where string, fields []ExternalField) {
			for _, field := range fields {
				if field.NameInDataSource == "" {
					note(where+" fields "+field.Name+" name_in_data_source", "empty")
				}
			}
		}
		for _, table := range source.Tables {
			where := at + " tables " + table.Name
			if (table.TableType == "" || table.TableType == ExternalTableFromTable) && table.NameInDataSource == "" {
				note(where+" name_in_data_source", "empty")
			}
			fields(where, table.Fields)
		}
		for _, cube := range source.Cubes {
			where := at + " cubes " + cube.Name
			if cube.NameInDataSource == "" {
				note(where+" name_in_data_source", "empty")
			}
			for _, resource := range cube.Resources {
				if resource.NameInDataSource == "" {
					note(where+" resources "+resource.Name+" name_in_data_source", "empty")
				}
			}
			for _, table := range cube.DimensionTables {
				if table.NameInDataSource == "" {
					note(where+" dimension tables "+table.Name+" name_in_data_source", "empty")
				}
				fields(where+" dimension tables "+table.Name, table.Fields)
			}
		}
	}
}

// ampersandZero is &0, &00 and the like - not a parameter, since parameters
// are counted from 1 - standing where a parameter would.
var ampersandZero = regexp.MustCompile(`&0+(\D|$)`)

func noteAmpersandZero(catalog *Catalog, note func(where, written string)) {
	for _, source := range catalog.ExternalDataSources {
		for _, function := range source.Functions {
			if ampersandZero.MatchString(function.ExpressionInDataSource) {
				note("external-data-sources "+source.Name+" functions "+function.Name+" expression_in_data_source", function.ExpressionInDataSource)
			}
		}
	}
}

func noteLongExternalString(catalog *Catalog, note func(where, written string)) {
	check := func(where string, fields []ExternalField) {
		for _, field := range fields {
			for _, item := range field.Types {
				if item.Kind == StringType && item.Length > maxStringLength {
					note(where+" fields "+field.Name+" types", strconv.Itoa(item.Length))
				}
			}
		}
	}
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			check("external-data-sources "+source.Name+" tables "+table.Name, table.Fields)
		}
		for _, cube := range source.Cubes {
			for _, table := range cube.DimensionTables {
				check("external-data-sources "+source.Name+" cubes "+cube.Name+" dimension tables "+table.Name, table.Fields)
			}
		}
	}
}

func noteForeignSourceReference(catalog *Catalog, note func(where, written string)) {
	for _, source := range catalog.ExternalDataSources {
		own := map[uuid.UUID]bool{}
		for _, table := range source.Tables {
			own[table.ID] = true
		}
		for _, table := range source.Tables {
			for _, field := range table.Fields {
				for _, item := range field.Types {
					if item.Kind == ExternalTableType && item.Reference != nil && !own[*item.Reference] {
						if _, elsewhere := catalog.externalTableByID[*item.Reference]; elsewhere {
							note("external-data-sources "+source.Name+" tables "+table.Name+" fields "+field.Name, item.Reference.String())
						}
					}
				}
			}
		}
	}
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
	choiceMode     *ChoiceMode
	quickChoice    *bool
	fullTextInput  *FullTextSearchOnInput
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
	take("ChoiceMode", reflect.TypeFor[ChoiceMode](), func(v reflect.Value) { holder.choiceMode = v.Interface().(*ChoiceMode) })
	take("FullTextSearchOnInput", reflect.TypeFor[FullTextSearchOnInput](), func(v reflect.Value) { holder.fullTextInput = v.Interface().(*FullTextSearchOnInput) })
	if holder.choiceMode != nil {
		if field, ok := value.Type().FieldByName("QuickChoice"); ok && field.Type == reflect.TypeFor[bool]() && len(field.Index) == 1 {
			holder.quickChoice = value.Field(field.Index[0]).Addr().Interface().(*bool)
		}
	}
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
