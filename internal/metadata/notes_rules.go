package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The kinds of note found in a loaded catalog. Each is a state an earlier
// iteration of block 2 stopped refusing; the comment beside the old refusal's
// place says why, and the description below says it for the report.
const (
	NoteUnusedBound                NoteKind = "unused-bound"
	NoteFillingNotHeld             NoteKind = "filling-not-held"
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
	NoteTextInUndeclaredLanguage   NoteKind = "text-in-undeclared-language"
	NoteTextWithoutLanguage        NoteKind = "text-without-language"
	NoteTextOfSpaces               NoteKind = "text-of-spaces"
	NoteHelpInUndeclaredLanguage   NoteKind = "help-in-undeclared-language"
	NotePlatformTypeByName         NoteKind = "platform-type-by-name"
	NotePlatformValueByName        NoteKind = "platform-value-by-name"
	NoteStandardFieldOfDocument    NoteKind = "standard-field-of-document"
	NoteLeadingDataNotDimension    NoteKind = "leading-data-not-dimension"
	NotePictureSettingsLeft        NoteKind = "picture-settings-without-picture"
	NoteChartHoldsNothing          NoteKind = "chart-value-type-holds-nothing"
	NoteFormReferenceAsWritten     NoteKind = "form-reference-as-written"
	NoteRepeatedElementName        NoteKind = "repeated-element-name"
	NotePropertyOutsideHelp        NoteKind = "property-outside-help"
	NoteUserSettingsGroupNotGroup  NoteKind = "user-settings-group-not-group"
	NoteHeldAdditionOfAnother      NoteKind = "held-addition-of-another-element"
	NoteAdditionOfNoTable          NoteKind = "addition-of-no-table"
	NoteEventByIdentifier          NoteKind = "event-by-identifier"
	NoteFormEventNotRaised         NoteKind = "form-event-not-raised"
	NoteAssociatedTableNotTable    NoteKind = "associated-table-not-table"
	NoteRepeatedInterfaceCommand   NoteKind = "repeated-interface-command"
	NoteChartStateUnexplained      NoteKind = "chart-state-unexplained"
	NoteChartValuesOrder           NoteKind = "chart-values-order"
	NoteChartTextAnyLanguage       NoteKind = "chart-text-any-language"
	NoteGanttChartStateUnexplained NoteKind = "gantt-chart-state-unexplained"
	NotePlannerStateUnexplained    NoteKind = "planner-state-unexplained"
)

func init() {
	noteKinds = append(noteKinds,
		NoteKindInfo{NoteUnusedBound,
			"Минимальное или максимальное значение у поля, которое не хранит одно число, или текст, который не читается числом. " +
				"Конфигуратор задаёт их только числу; у других типов это след смены типа.",
			"Граница несётся как записана и ничего не ограничивает."},
		NoteKindInfo{NoteFillingNotHeld,
			"Значение заполнения типа, которого поле не хранит, — след смены типа поля (пустая ссылка у строкового поля).",
			"Значение несётся как записано, поле при создании ничем не заполняется."},
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
		NoteKindInfo{NoteTextInUndeclaredLanguage,
			"Перевод на язык, которого нет среди языков конфигурации (erp, acc и sb: «en» при языках ru и uk).",
			"Перевод несётся как записан и показывается, только если ни на одном языке конфигурации текста нет."},
		NoteKindInfo{NoteTextWithoutLanguage,
			"Перевод без кода языка (синоним стандартной табличной части плана счетов и ПВР: 7/6/1).",
			"Перевод несётся как записан и показывается, только если ни на одном языке конфигурации текста нет."},
		NoteKindInfo{NoteTextOfSpaces,
			"Текст из одних пробелов — подсказка из одного пробела (sb, 24 раза); похоже на нарочное подавление подсказки.",
			"Текст несётся как записан и при показе считается пустым."},
		NoteKindInfo{NoteHelpInUndeclaredLanguage,
			"Страница справки на языке, которого нет среди языков конфигурации, — или файл, названный не кодом языка, " +
				"а, например, его именем (Русский.html).",
			"Страница несётся и публикуется как записана; справка не показывается."},
		NoteKindInfo{NotePlatformTypeByName,
			"Тип, который определяет платформа и которого модель не перечисляет поимённо (energy: Отбор, ТипДиаграммы), — " +
				"у реквизита отчёта или обработки, в определяемом типе.",
			"Тип несётся по имени и не исполняется: значение такого реквизита — Неопределено."},
		NoteKindInfo{NotePlatformValueByName,
			"Значение типа, который определяет платформа и которого модель не перечисляет поимённо, — вид счёта, вид сравнения " +
				"компоновки — в списке выбора поля формы (вид счёта 6, вид сравнения 2) или в параметре выбора.",
			"Значение несётся по имени типа и значения, как записано, и не исполняется: в список выбора и в отбор не попадает."},
		NoteKindInfo{NoteStandardFieldOfDocument,
			"Графа журнала или измерение последовательности берёт стандартный реквизит документа (Дата, Номер). " +
				"Справка описывает обе ссылки как набор реквизитов документов, а стандартный реквизит объектом метаданных не является; " +
				"в выгрузках таких ссылок нет, сохраняет ли их конфигуратор — не проверено.",
			"Ссылка несётся по документу и имени реквизита. Журнал и последовательность ML пока не исполняет; " +
				"при исполнении значение берётся из стандартного реквизита, как из обычного."},
		NoteKindInfo{NoteLeadingDataNotDimension,
			"Данные ведущих регистров у измерения перерасчёта — ресурс или реквизит регистра расчёта, а не измерение. " +
				"Справка вид поля не уточняет, в выгрузках все такие ссылки — на измерения.",
			"Ссылка несётся как записана. Перерасчёт ML пока не исполняет; при исполнении изменение этого поля " +
				"требует перерасчёта наравне с измерением."},
		NoteKindInfo{NotePictureSettingsLeft,
			"Ссылка на картинку без картинки: картинку убрали, а загрузка прозрачной или точка прозрачности остались " +
				"(в выгрузке — ссылка на картинку 0). Конфигуратор такое сохраняет: в выгрузках 8 ссылок.",
			"Настройки несутся как записаны; рисуется как без картинки — команда, отображаемая картинкой, показывается текстом."},
		NoteKindInfo{NoteChartHoldsNothing,
			"Тип значения плана видов характеристик — только характеристики самого плана или планов, которые в ответ держат только его. " +
				"Конфигуратор позволяет выбрать план в его же типе значения и сохраняет это (проверено на 8.3.27); " +
				"рядом с другими типами характеристики себя ничего не добавляют, а без них тип пуст.",
			"План несётся как записан; его характеристика не принимает ни одного значения."},
		NoteKindInfo{NoteFormReferenceAsWritten,
			"Ссылка в форме, записанная кодом или числом вместо имени: номер элемента с идентификатором (\"3:409b9a53-…\"), " +
				"одинокий «0», число вместо пути к данным, два пути через «~». Прототип пишет их в команде кнопки, источнике команд, " +
				"пути к данным, пути к данным заголовка, путях связей поля и группе пользовательских настроек таблицы; на что такая ссылка указывает, не установлено " +
				"(открытый вопрос карты блоков о коде элемента формы).",
			"Ссылка несётся как записана и ни на что не указывает: кнопка без команды, панель без источника, поле и заголовок без данных, связь не действует, " +
				"настройки списка не показываются."},
		NoteKindInfo{NoteRepeatedElementName,
			"Имя повторяется у нескольких элементов одной формы. Дополнения таблицы, контекстные меню и расширенные подсказки прототип " +
				"называет сам («Addition», «ContextMenu», «ExtendedTooltip») и сохраняет повтор, если имя не изменили: в выгрузках так шесть форм. " +
				"Имя других элементов уникально.",
			"Элементы несутся как записаны; из кода по такому имени однозначно не обратиться, исполнение выберет один из них."},
		NoteKindInfo{NotePropertyOutsideHelp,
			"Свойство, которое прототип записывает, а справка 8.3.27 у этого элемента не знает: автозаполнение таблицы " +
				"(267 таблиц — настроек компоновки, таблиц и списков значений, всегда «да»). Что оно делает, не установлено.",
			"Свойство несётся как записано и не исполняется."},
		NoteKindInfo{NoteUserSettingsGroupNotGroup,
			"Группа пользовательских настроек динамического списка называет элемент, который не группа: поле ввода " +
				"(2 формы выгрузок). Платформа создаёт элементы настроек внутри группы, а в поле их создать некуда — " +
				"по всей видимости, след переименования или удаления группы.",
			"Ссылка несётся как записана; элементы пользовательских настроек списка не создаются."},
		NoteKindInfo{NoteHeldAdditionOfAnother,
			"Дополнение, которое таблица держит у себя (строка поиска, состояние просмотра, управление поиском), называет источником " +
				"не эту таблицу, а другой элемент: в одной форме sb — расширенную подсказку соседнего элемента. Прототип такое сохраняет; " +
				"что оно показывает, не установлено.",
			"Дополнение несётся как записано, с источником; таблица показывает его как своё."},
		NoteKindInfo{NoteAdditionOfNoTable,
			"Строка поиска, состояние просмотра или управление поиском стоит отдельно от таблицы и не называет никакой таблицы: " +
				"в lombard1 так одна форма, две штуки в командной панели формы. Конфигуратор такое сохраняет; по всей видимости, " +
				"след удалённой таблицы или дополнение, перенесённое из другой формы.",
			"Дополнение несётся как записано и ничего не показывает."},
		NoteKindInfo{NoteEventByIdentifier,
			"Событие формы или элемента записано не именем, а внутренним идентификатором (64 раза в выгрузках): у формы, основной " +
				"реквизит которой такого события не вызывает, — по имени обработчика это запись на сервере, чтение, загрузка настроек. " +
				"По всей видимости, след смены основного реквизита. Какое событие стоит за идентификатором, справка не называет.",
			"Событие несётся как записано; обработчик не вызывается."},
		NoteKindInfo{NoteFormEventNotRaised,
			"Событие формы записано именем, но основной реквизит формы его не вызывает: обработчик записи или чтения у формы обработки " +
				"или формы без основного реквизита (14 раз в выгрузках) — тот же след смены основного реквизита, что и событие по идентификатору.",
			"Событие несётся как записано; обработчик не вызывается."},
		NoteKindInfo{NoteAssociatedTableNotTable,
			"Команда формы называет связанной таблицей элемент, который не таблица: поле ввода, кнопку, контекстное меню (5 команд " +
				"выгрузок). Справка требует таблицу той же формы; по всей видимости, след удалённой или переименованной таблицы.",
			"Ссылка несётся как записана; текущей строки у такой «таблицы» нет, и команда её не получает."},
		NoteKindInfo{NoteRepeatedInterfaceCommand,
			"Командный интерфейс формы повторяет в одной панели строку целиком: ту же команду с той же группой, местом, видимостью и " +
				"источником параметра (59 раз в выгрузках, без счёта команд, записанных кодом). Покажет ли прототип команду дважды, не известно.",
			"Строки несутся все, в записанном порядке; командный интерфейс формы строится в блоке 18, и там решается, сколько раз её показать."},
		NoteKindInfo{NoteChartStateUnexplained,
			"Содержимое диаграммы несёт значения, которых справка не называет нигде: время перестроения, признаки инициализации областей, " +
				"случайные новые значения, транспонирование, связи многоуровневой диаграммы и другие — собственное состояние объекта у прототипа " +
				"(у всех 31 диаграммы выгрузок).",
			"Значения несутся как записаны, в отдельной части содержимого; диаграмма их не исполняет."},
		NoteKindInfo{NoteChartValuesOrder,
			"Значения диаграммы записаны одним списком на все серии и точки, и по выгрузкам не видно, идёт ли список по сериям или по точкам: " +
				"во всех девяти диаграммах с значениями они придуманы конструктором. Порядок устанавливается прогоном на платформе.",
			"Значения несутся в записанном порядке; диаграмма рисуется только после того, как порядок установлен."},
		NoteKindInfo{NoteChartTextAnyLanguage,
			"Текст диаграммы, диаграммы Ганта или планировщика записан на «языке» «#», которого нет среди языков конфигурации: имя " +
				"сводной серии, форматы подписей, шкалы времени и заголовков переносов (43 текста выгрузок: 23 у диаграмм, 15 у диаграмм " +
				"Ганта, 5 у планировщиков). Что прототип под ним понимает, справка не говорит.",
			"Текст несётся как записан, под тем же ключом."},
		NoteKindInfo{NoteGanttChartStateUnexplained,
			"Содержимое диаграммы Ганта несёт значения, которых справка не называет нигде: начало видимой части, мера шкалы без варианта " +
				"масштаба, ключи и данные корня точек и серий, текущий уровень шкалы времени, числа на месте интервалов фона и меток шкалы — " +
				"собственное состояние объекта у прототипа (у всех 15 диаграмм Ганта выгрузок).",
			"Значения несутся как записаны, в отдельных частях содержимого; диаграмма Ганта их не исполняет."},
		NoteKindInfo{NotePlannerStateUnexplained,
			"Содержимое планировщика несёт значения, которых справка не называет нигде: идентификатор элемента планировщика, текущий " +
				"уровень шкалы времени, число на месте меток шкалы — собственное состояние объекта у прототипа (у всех 3 планировщиков выгрузок " +
				"есть идентификатор элемента).",
			"Значения несутся как записаны, в отдельных частях содержимого; планировщик их не исполняет."},
	)
	noteRules[NoteUnusedBound] = noteUnusedBound
	noteRules[NoteFillingNotHeld] = noteFillingNotHeld
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
	noteRules[NoteTextInUndeclaredLanguage] = noteTextInUndeclaredLanguage
	noteRules[NoteTextWithoutLanguage] = noteTextWithoutLanguage
	noteRules[NoteTextOfSpaces] = noteTextOfSpaces
	noteRules[NoteHelpInUndeclaredLanguage] = noteHelpInUndeclaredLanguage
	noteRules[NotePlatformTypeByName] = notePlatformTypeByName
	noteRules[NotePlatformValueByName] = notePlatformValueByName
	noteRules[NoteStandardFieldOfDocument] = noteStandardFieldOfDocument
	noteRules[NoteLeadingDataNotDimension] = noteLeadingDataNotDimension
	noteRules[NotePictureSettingsLeft] = notePictureSettingsLeft
	noteRules[NoteChartHoldsNothing] = noteChartHoldsNothing
	noteRules[NoteRepeatedElementName] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteRepeatedElementName, note)
	}
	noteRules[NoteFormReferenceAsWritten] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteFormReferenceAsWritten, note)
	}
	noteRules[NotePropertyOutsideHelp] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NotePropertyOutsideHelp, note)
	}
	noteRules[NoteUserSettingsGroupNotGroup] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteUserSettingsGroupNotGroup, note)
	}
	noteRules[NoteHeldAdditionOfAnother] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteHeldAdditionOfAnother, note)
	}
	noteRules[NoteAdditionOfNoTable] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteAdditionOfNoTable, note)
	}
	noteRules[NoteEventByIdentifier] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteEventByIdentifier, note)
	}
	noteRules[NoteFormEventNotRaised] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteFormEventNotRaised, note)
	}
	noteRules[NoteAssociatedTableNotTable] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteAssociatedTableNotTable, note)
	}
	noteRules[NoteRepeatedInterfaceCommand] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteRepeatedInterfaceCommand, note)
	}
	noteRules[NoteChartStateUnexplained] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteChartStateUnexplained, note)
	}
	noteRules[NoteChartValuesOrder] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteChartValuesOrder, note)
	}
	noteRules[NoteChartTextAnyLanguage] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteChartTextAnyLanguage, note)
	}
	noteRules[NoteGanttChartStateUnexplained] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NoteGanttChartStateUnexplained, note)
	}
	noteRules[NotePlannerStateUnexplained] = func(catalog *Catalog, note func(where, written string)) {
		catalog.formNotesOf(NotePlannerStateUnexplained, note)
	}
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
	catalog.formNotesOf(NoteChoiceSetAndLinked, note)
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
			if len(column.References)+len(column.StandardReferences) == 0 {
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

func noteStandardFieldOfDocument(catalog *Catalog, note func(where, written string)) {
	for _, journal := range catalog.DocumentJournals {
		for _, column := range journal.Columns {
			for _, field := range column.StandardReferences {
				note("document-journals "+journal.Name+" columns "+column.Name+" standard_references", field.Standard)
			}
		}
	}
	for _, sequence := range catalog.Sequences {
		for _, dimension := range sequence.Dimensions {
			for _, field := range dimension.DocumentStandardAttributes {
				note("sequences "+sequence.Name+" dimensions "+dimension.Name+" document_standard_attributes", field.Standard)
			}
		}
	}
}

func noteLeadingDataNotDimension(catalog *Catalog, note func(where, written string)) {
	dimensions := map[uuid.UUID]bool{}
	for _, register := range catalog.CalculationRegisters {
		for _, dimension := range register.Dimensions {
			dimensions[dimension.ID] = true
		}
	}
	for _, register := range catalog.CalculationRegisters {
		for _, recalculation := range register.Recalculations {
			for _, dimension := range recalculation.Dimensions {
				for _, source := range dimension.LeadingData {
					if !dimensions[source] {
						note("calculation-registers "+register.Name+" recalculations "+recalculation.Name+" dimensions "+dimension.Name+" leading_data", source.String())
					}
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

func noteChartHoldsNothing(catalog *Catalog, note func(where, written string)) {
	for _, chart := range catalog.ChartsOfCharacteristicTypes {
		if len(chart.ValueType) == 0 {
			continue
		}
		if types, err := catalog.expandTypes(chart.ValueType, nil); err == nil && len(types) == 0 {
			note("charts-of-characteristic-types "+chart.Name, "value type: characteristics only")
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

func noteTextInUndeclaredLanguage(catalog *Catalog, note func(where, written string)) {
	declared := map[string]bool{}
	for _, language := range catalog.Project.Languages {
		declared[strings.ToLower(language.Code)] = true
	}
	eachLocalizedText(catalog, func(where string, text LocalizedText) {
		for _, code := range sortedTextKeys(text) {
			if code != "" && !declared[strings.ToLower(code)] {
				note(where+"."+code, text[code])
			}
		}
	})
}

func noteTextWithoutLanguage(catalog *Catalog, note func(where, written string)) {
	eachLocalizedText(catalog, func(where string, text LocalizedText) {
		if value, ok := text[""]; ok {
			note(where, value)
		}
	})
}

func noteTextOfSpaces(catalog *Catalog, note func(where, written string)) {
	eachLocalizedText(catalog, func(where string, text LocalizedText) {
		for _, code := range sortedTextKeys(text) {
			if value := text[code]; value != "" && strings.TrimSpace(value) == "" {
				note(where+"."+code, strconv.Quote(value))
			}
		}
	})
}

func noteHelpInUndeclaredLanguage(catalog *Catalog, note func(where, written string)) {
	declared := map[string]bool{}
	for _, language := range catalog.Project.Languages {
		declared[strings.ToLower(language.Code)] = true
	}
	for _, page := range catalog.helpPages {
		if !declared[strings.ToLower(page.code)] {
			note(page.where, page.code)
		}
	}
}

func notePlatformTypeByName(catalog *Catalog, note func(where, written string)) {
	eachTypeList(catalog, func(where string, types []Type) {
		for _, item := range types {
			if item.Kind == PlatformType {
				note(where, item.Name)
			}
		}
	})
}

func notePlatformValueByName(catalog *Catalog, note func(where, written string)) {
	eachNoteHolder(catalog, func(holder noteHolder) {
		if holder.choice == nil {
			return
		}
		for _, parameter := range holder.choice.Parameters {
			for _, value := range parameter.Values {
				if value.Kind == PlatformType {
					note(holder.where+" choice parameter "+parameter.Name, value.Data)
				}
			}
		}
	})
	catalog.formNotesOf(NotePlatformValueByName, note)
}

func notePictureSettingsLeft(catalog *Catalog, note func(where, written string)) {
	eachPictureReference(catalog, func(where string, picture *PictureReference) {
		if !picture.Names() {
			written := "load_transparent"
			if picture.TransparentPixel != nil {
				written = fmt.Sprintf("transparent_pixel %d,%d", picture.TransparentPixel.X, picture.TransparentPixel.Y)
			}
			note(where, written)
		}
	})
}

// eachPictureReference walks every reference to a picture in the catalog -
// beside a command, a group of commands, a subsystem, an item of a route map -
// and calls visit with the place it stands in.
func eachPictureReference(catalog *Catalog, visit func(where string, picture *PictureReference)) {
	top := reflect.ValueOf(catalog).Elem()
	for index := range top.NumField() {
		field := top.Type().Field(index)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		list := top.Field(index)
		for item := range list.Len() {
			object := list.Index(item)
			walkPictureReferences(object, kebab(field.Name)+" "+nameOf(object), visit)
		}
	}
}

func walkPictureReferences(value reflect.Value, where string, visit func(string, *PictureReference)) {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return
		}
		if picture, ok := value.Interface().(*PictureReference); ok {
			visit(where, picture)
			return
		}
		walkPictureReferences(value.Elem(), where, visit)
	case reflect.Slice:
		switch value.Type().Elem().Kind() {
		case reflect.Struct, reflect.Pointer:
			for index := range value.Len() {
				element := value.Index(index)
				walkPictureReferences(element, where+" "+nameOf(element), visit)
			}
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			if !field.IsExported() {
				continue
			}
			next := where
			if !field.Anonymous {
				next = where + " " + yamlName(field)
			}
			walkPictureReferences(value.Field(index), next, visit)
		}
	}
}

// eachTypeList walks every type description of the catalog - the types of a
// field, the parameter of a command, the value type of a chart - and calls
// visit with the place it stands in.
func eachTypeList(catalog *Catalog, visit func(where string, types []Type)) {
	top := reflect.ValueOf(catalog).Elem()
	for index := range top.NumField() {
		field := top.Type().Field(index)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		list := top.Field(index)
		for item := range list.Len() {
			object := list.Index(item)
			walkTypeLists(object, kebab(field.Name)+" "+nameOf(object), visit)
		}
	}
	// The forms are not part of the catalog; their attributes' types were
	// gathered when the forms were read.
	for _, list := range catalog.formTypeLists {
		visit(list.where, list.types)
	}
}

func walkTypeLists(value reflect.Value, where string, visit func(string, []Type)) {
	typesType := reflect.TypeFor[[]Type]()
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			walkTypeLists(value.Elem(), where, visit)
		}
	case reflect.Slice:
		if value.Type() == typesType {
			visit(where, value.Interface().([]Type))
			return
		}
		switch value.Type().Elem().Kind() {
		case reflect.Struct, reflect.Pointer:
			for index := range value.Len() {
				element := value.Index(index)
				walkTypeLists(element, where+" "+nameOf(element), visit)
			}
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			if !field.IsExported() {
				continue
			}
			next := where
			if !field.Anonymous {
				next = where + " " + yamlName(field)
			}
			walkTypeLists(value.Field(index), next, visit)
		}
	}
}

func sortedTextKeys(text LocalizedText) []string {
	keys := make([]string, 0, len(text))
	for key := range text {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// eachLocalizedText walks every text of the catalog - the configuration's own
// and every object's, at any depth - and calls visit with the place it stands
// in. Texts are maps, which the walk of the holders does not enter.
func eachLocalizedText(catalog *Catalog, visit func(where string, text LocalizedText)) {
	walkTexts(reflect.ValueOf(&catalog.Project).Elem(), "configuration", visit)
	top := reflect.ValueOf(catalog).Elem()
	for index := range top.NumField() {
		field := top.Type().Field(index)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		list := top.Field(index)
		for item := range list.Len() {
			object := list.Index(item)
			walkTexts(object, kebab(field.Name)+" "+nameOf(object), visit)
		}
	}
	// The forms are not part of the catalog; the titles of their attributes
	// were gathered when the forms were read.
	for _, text := range catalog.formTitles {
		visit(text.where, text.text)
	}
}

func walkTexts(value reflect.Value, where string, visit func(string, LocalizedText)) {
	textType := reflect.TypeFor[LocalizedText]()
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			walkTexts(value.Elem(), where, visit)
		}
	case reflect.Map:
		if value.Type() == textType {
			visit(where, value.Interface().(LocalizedText))
			return
		}
		if value.Type().Elem() == textType {
			for _, key := range value.MapKeys() {
				walkTexts(value.MapIndex(key), where+" "+fmt.Sprint(key.Interface()), visit)
			}
		}
	case reflect.Slice:
		switch value.Type().Elem().Kind() {
		case reflect.Struct, reflect.Pointer, reflect.Map:
			for index := range value.Len() {
				element := value.Index(index)
				walkTexts(element, where+" "+nameOf(element), visit)
			}
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			if !field.IsExported() {
				continue
			}
			next := where
			if !field.Anonymous {
				next = where + " " + yamlName(field)
			}
			walkTexts(value.Field(index), next, visit)
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
