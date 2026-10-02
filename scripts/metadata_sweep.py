#!/usr/bin/env python3
"""Сверить состав нашей модели метаданных с выгрузкой конфигурации-прототипа.

Зачем. Правило закрытия блока требует сверки с `docs/materials` по области
блока целиком, а не по его диффу. Состав свойств можно сверить машинно: в
`docs/materials/platform-model/01-metadata/property-index.json` лежат свойства
по видам объектов, собранные из XML-выгрузки, и их можно сопоставить с полями
наших структур. Находить такие расхождения по одному, спотыкаясь, — худший
способ: так нашлись индексирование у ресурса, признак учёта субконто у
измерения и связи параметров выбора, не достающие до измерений.

Сверка идёт двумя проходами. Первый — свойства самих видов по
`property-index.json`. Второй — свойства **подчинённых объектов** по
`child-objects.json`: измерений, ресурсов, реквизитов, табличных частей,
команд, макетов, признаков учёта, реквизитов адресации, граф журнала, операций
веб-сервиса и шаблонов URL, — а также двух объектов второй ступени: метода
шаблона URL и параметра операции. Второй проход нужен
отдельно потому, что `property-index.json` собран по видам и свойств измерения
— ведущее, основной отбор, использование в итогах — в нём нет вовсе. Пока его
не было, четыре свойства измерения отсутствовали у нас незамеченными и нашлись
чтением справки, а не прогоном; первый же его прогон нашёл ещё четыре пробела,
среди них признак учёта плана счетов с тремя свойствами из двадцати шести.

Область сопоставления во втором проходе — **сам подчинённый вид**, а не его
владелец: команда значит одно и то же у справочника, документа и обработки, и
алиас для неё пишется один раз, а не по разу на владельца.

Чего сверка НЕ делает, и это важнее того, что делает:

- она сверяет **состав, а не поведение**. Механику — порядок шагов, кто кого
  ждёт, что происходит при отказе — выгрузка не содержит вовсе, её читают
  руками по синтакс-помощнику;
- **отсутствие свойства в выгрузке ничего не доказывает.** Выгрузка
  показывает, что используется в НЕЙ. Остаток сверки — это перечень вопросов
  к синтакс-помощнику, а не перечень ошибок;
- имена у нас свои. Совпадение имён проверяется с точностью до порядка слов и
  формы слова, а осознанные переименования перечислены в RENAMED ниже. Каждая
  строка там — утверждение «это то же самое, названное иначе», и она читается
  глазами, а не выводится.

Как запускать:

    python3 scripts/metadata_sweep.py            # остаток по всем видам
    python3 scripts/metadata_sweep.py --kind Constant

Состав модели выгружается прогоном `TestDumpMetadataComposition` из
`internal/metadata`: рефлексия по структурам лежит рядом с самими структурами,
а не повторяется здесь списком, который разойдётся с моделью на первой правке.
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
EXPORT = ROOT / "docs" / "materials" / "platform-model" / "01-metadata" / "property-index.json"
CHILDREN = ROOT / "docs" / "materials" / "platform-model" / "01-metadata" / "child-objects.json"
HELP = ROOT / "docs" / "materials" / "its" / "8.3.27.2342" / "json" / "shcntx_ru.json"

# Имена объектов метаданных в синтакс-помощнике, которые не совпадают с именами
# выгрузки. Справка называет подчинённые виды полнее: операция и параметр
# веб-сервиса, шаблон URL и метод HTTP-сервиса, графа журнала.
HELP_KIND_ALIAS = {
    "WebServiceOperation": "Operation",
    "WebServiceParameter": "Parameter",
    "HTTPServiceURLTemplate": "URLTemplate",
    "HttpServiceMethod": "Method",
    "Graph": "Column",
    "DocumentNumerator": "Numerator",
    "FunctionalOptionsParameter": "FunctionalOptionParameter",
}

# Подчинённые виды, которые в выгрузке состава лежат глубже, чем в справке:
# таблицы, поля, кубы, таблицы измерений и функции внешнего источника, а также
# измерения перерасчёта - он сам подчинён регистру расчёта.
HELP_CHILD_COLLECTIONS = {
    "Tables": "Table",
    "Fields": "Field",
    "Cubes": "Cube",
    "DimensionTables": "DimensionTable",
    "Functions": "Function",
}

# Свойство справки под нашим именем, которое сопоставитель по словам не находит:
# вложено или названо иначе. Каждая строка проверена по коду.
HELP_RENAMED = {
    ("Language", "LanguageCode"): "code",
    ("Document", "ActionsWritingOnPost"): "records_writing",     # posting.records_writing
    ("Document", "PrivilegedPostingMode"): "privileged",         # posting.privileged
    ("Document", "PrivilegedUnpostingMode"): "unpost_privileged",
    ("Template", "TemplateType"): "kind",
    ("Subsystem", "Subsystems"): "parent",                       # вложенность записана ссылкой на родителя
    ("Enum", "EnumValues"): "values",
    ("HTTPService", "URLTemplates"): "templates",
    ("Role", "Rights"): "objects",                               # права на объекты и на команды
    ("Dimension", "LeadingRegisterData"): "leading_data",        # измерение перерасчёта
    ("Table", "TableDataType"): "data_type",
    ("BusinessProcess", "Flowchart"): "route",                   # карта маршрута
    ("Table", "TransactionsIsolationLevel"): "isolation_level",
    ("Attribute", "BinaryDataStorageLocationUse"): "binary_data_storage",
    ("Attribute", "BinaryDataStorageLocationUseField"): "binary_data_storage_field",
    ("Resource", "BinaryDataStorageLocationUse"): "binary_data_storage",
    ("Resource", "BinaryDataStorageLocationUseField"): "binary_data_storage_field",
}
# То же для любого вида: табличные части у нас называются table_parts.
HELP_RENAMED_ANY = {"TabularSections": "table_parts"}

# Свойства, которые у нас не поля описания, а файлы рядом с ним: модули и тела
# объектов. Выгрузка состава их не видит, а в проекте они есть.
HELP_AS_FILES = {"ManagerModule", "ObjectModule", "RecordSetModule", "ValueManagerModule",
                 "CommandModule", "Module"}
HELP_AS_FILES_BY_KIND = {
    ("CommonPicture", "Picture"), ("Template", "Template"), ("XDTOPackage", "Package"),
    ("WSReference", "WSDefinition"), ("Style", "Style"),
}

# Решено в docs/requirements: свойство есть у прототипа, у нас его нет нарочно.
HELP_DECIDED = {
    ("FilterCriterion", "StandardAttributes"): "METADATA-OBJECTS.md: у критерия отбора стандартных реквизитов нет",
    ("WebSocketClient", "StandardAttributes"): "METADATA-OBJECTS.md: у WebSocket-клиента стандартных реквизитов нет",
}

# Объекты справки, которых у нас нет в модели нарочно, и где это записано.
HELP_DECIDED_KINDS = {
    "Bot": "METADATA-OBJECTS.md: боты не переносятся - отказ владельца",
    "IntegrationService": "METADATA-OBJECTS.md: сервисы интеграции не переносятся - отказ владельца",
    "IntegrationServiceChannel": "METADATA-OBJECTS.md: канал сервиса интеграции - вместе с ним",
    "Interface": "METADATA-OBJECTS.md: интерфейс обычного приложения не переносится",
    "Form": "форма - файл описания формы рядом с объектом, её состав сверяется конструктором форм",
}

# Свойства, которые имеют смысл только у объекта расширения конфигурации:
# принадлежность объекта расширению и заимствованный объект основной
# конфигурации. Расширения не поддерживаются - решение владельца, записано в
# METADATA-OBJECTS.md, - поэтому это не пробел у каждого вида.
HELP_EXTENSIONS = {"ExtendedConfigurationObject", "ObjectBelonging"}

# Имена наших коллекций против имён дочерних видов выгрузки. Сверка по дочерним
# объектам нужна отдельно, потому что property-index собран только по свойствам
# самих видов: свойств измерения - ведущее, основной отбор, использование в
# итогах - в нём нет вовсе. Пока сверка их не смотрела, четыре свойства измерения
# отсутствовали у нас незамеченными, и нашлись чтением справки, а не прогоном.
CHILD_COLLECTIONS = {
    "Dimensions": "Dimension",
    "Resources": "Resource",
    "Attributes": "Attribute",
    "TableParts": "TabularSection",
    "AddressingAttributes": "AddressingAttribute",
    "AccountingFlags": "AccountingFlag",
    "ExtDimensionAccountingFlags": "ExtDimensionAccountingFlag",
    "Recalculations": "Recalculation",
    "Predefined": "PredefinedItem",
    "Commands": "Command",
    "Templates": "Template",
    "Characteristics": "Characteristic",
    "Columns": "Column",
    "Values": "EnumValue",
    "Operations": "Operation",
}

# Одна и та же коллекция значит у разных видов разное: макеты у объекта - это
# макеты, а `Templates` у HTTP-сервиса - шаблоны URL. Разбирается по владельцу.
CHILD_COLLECTIONS_BY_KIND = {
    ("HTTPService", "Templates"): "URLTemplate",
}

# Внуки: выгрузка называет отношение тройкой «вид, владелец, дочерний вид», и
# два её отношения лежат на второй ступени - метод шаблона URL и параметр
# операции веб-сервиса. Ключ здесь - вид дочернего объекта-владельца.
GRANDCHILD_COLLECTIONS = {
    ("URLTemplate", "Methods"): "Method",
    ("Operation", "Parameters"): "Parameter",
    ("TabularSection", "Attributes"): "Attribute",
}

# Осознанные переименования: слово прототипа -> наше слово. Каждая строка -
# решение, а не догадка, и держится тем, что записано в docs/requirements.
RENAMED = {
    "synonym": "title",          # синоним объекта - у нас title
    "type": "types",             # тип поля у нас всегда набор типов
    "default": "main",           # DefaultForm и прочие - слоты форм
    "hierarchical": "hierarchy",
    "autonumbering": "auto",
    "xdto": "",                  # XDTOPackages -> packages
    "behavior": "",              # OnMainServerUnavalableBehavior
    "check": "",                 # CheckUnique -> unique внутри code и number
}

# Свойства, которые у нас лежат вложенными и потому не совпадают ни по словам,
# ни по уровню: имя прототипа целиком слева, наше имя листа справа.
FLAT_ALIAS = {
    "PasswordMode": "password",
    "FillValue": "value",
    "FillFromFillingValue": "from_filling_value",
}

# Переименования, которые верны только у одного вида: одно и то же слово
# прототипа значит у разных объектов разное.
RENAMED_BY_KIND = {
    # Общая форма - ManagedForm, общий тип всех форм; конфигурация - описание
    # проекта (internal/project). Оба вида сверяются с 02.10.2026: до того
    # дамп состава их не содержал, и сверка молча пропускала оба.
    ("Language", "LanguageCode"): "code",
    ("CommonForm", "FormType"): "type",
    ("CommonForm", "UsePurposes"): "purposes",
    ("Configuration", "AllowedIncomingShareRequestTypes"): "allowed_share_request_types",
    ("Configuration", "CompatibilityMode"): "compatibility_version",
    ("Configuration", "ConfigurationExtensionCompatibilityMode"): "extension_compatibility_version",
    ("Configuration", "ConfigurationInformationAddress"): "information_address",
    ("Configuration", "DefaultCollaborationSystemUsersChoiceForm"): "collaboration_system_users_choice_form",
    ("Configuration", "DefaultDataHistoryChangeHistoryForm"): "data_history_changes_form",
    ("Configuration", "DefaultDataHistoryVersionDataForm"): "data_history_version_form",
    ("Configuration", "DefaultDataHistoryVersionDifferencesForm"): "data_history_version_difference_form",
    ("Configuration", "MainClientApplicationWindowMode"): "main_window_mode",
    ("Configuration", "RequiredMobileApplicationPermissions"): "required_mobile_permissions",
    ("Configuration", "SynchronousPlatformExtensionAndAddInCallUseMode"): "synchronous_platform_extension_call_use",
    ("Configuration", "UseManagedFormInOrdinaryApplication"): "use_managed_forms_in_ordinary_application",
    ("Configuration", "UseOrdinaryFormInManagedApplication"): "use_ordinary_forms_in_managed_application",
    ("Configuration", "UsedMobileApplicationFunctionalities"): "used_mobile_functionalities",
    ("Document", "RegisterRecords"): "movements",
    ("Sequence", "RegisterRecords"): "movements",
    ("Subsystem", "Content"): "members",
    ("FilterCriterion", "Content"): "fields",
    ("CommonTemplate", "TemplateType"): "kind",
    ("EventSubscription", "Handler"): "procedure",
    ("EventSubscription", "Source"): "source",
    ("CommonCommand", "OnMainServerUnavalableBehavior"): "on_server_unavailable",
    ("WebService", "XDTOPackages"): "packages",
    ("AccountingRegister", "EnableTotalsSplitting"): "allow_totals_splitting",
    ("AccumulationRegister", "EnableTotalsSplitting"): "allow_totals_splitting",
    ("AccumulationRegister", "RegisterType"): "kind",
    ("ChartOfCalculationTypes", "BaseCalculationTypes"): "base_charts",
    ("ChartOfCalculationTypes", "DependenceOnCalculationTypes"): "base_dependency",
    # Проведение: прототип называет каждое свойство от глагола, у нас они лежат
    # внутри структуры posting. Проверено по модели поимённо - именно этих пять
    # строк не было, и остаток выглядел так, будто у документа нет
    # привилегированного проведения вовсе.
    ("Document", "PostInPrivilegedMode"): "privileged",
    ("Document", "UnpostInPrivilegedMode"): "unpost_privileged",
    ("Document", "RealTimePosting"): "real_time",
    ("Document", "RegisterRecordsDeletion"): "records_deletion",
    ("Document", "RegisterRecordsWritingOnPost"): "records_writing",
    ("BusinessProcess", "CreateTaskInPrivilegedMode"): "create_tasks_privileged",
    ("ExchangePlan", "IncludeConfigurationExtensions"): "include_extensions",
    ("ScheduledJob", "MethodName"): "procedure",
    ("InformationRegister", "EnableTotalsSliceFirst"): "slice_first",
    ("InformationRegister", "EnableTotalsSliceLast"): "slice_last",
    ("InformationRegister", "InformationRegisterPeriodicity"): "periodicity",
    ("DocumentJournal", "RegisteredDocuments"): "documents",
    ("Report", "MainDataCompositionSchema"): "main_schema",
    ("CommonCommand", "CommandParameterType"): "parameter",
    ("Catalog", "LimitLevelCount"): "limit_levels",
    ("ChartOfAccounts", "StandardTabularSections"): "standard_table_parts",
    ("ChartOfCalculationTypes", "StandardTabularSections"): "standard_table_parts",
    ("ChartOfCharacteristicTypes", "CharacteristicExtValues"): "additional_values",
    ("Task", "TaskNumberAutoPrefix"): "number_prefix",
    # Дочерние объекты: область - сам дочерний вид, не его владелец. Команда
    # значит одно и то же у справочника, документа, журнала, регистра сведений,
    # отчёта, обработки и задачи, и до этих двух строк остаток повторял её
    # семь раз подряд.
    ("Command", "CommandParameterType"): "parameter",
    ("Command", "OnMainServerUnavalableBehavior"): "on_server_unavailable",
    # Измерение последовательности: соответствие документам - это реквизиты
    # документа, из которых берётся значение, соответствие движениям - это
    # измерения регистров, где то же значение лежит.
    ("Dimension", "DocumentMap"): "document_attributes",
    ("Dimension", "RegisterRecordsMap"): "register_dimensions",
    # Реквизит адресации: измерение регистра адресации, с которым он сверяется.
    ("AddressingAttribute", "AddressingDimension"): "dimension",
    # Операция веб-сервиса: тип возвращаемого значения XDTO - это и есть наш
    # тип ответа, названный без упоминания XDTO, как и сам пакет.
    ("Operation", "XDTOReturningValueType"): "return_type",
    # Параметр операции: тип значения XDTO и направление передачи. Оба названы
    # у нас короче, потому что другого типа и другой передачи у параметра нет.
    ("Parameter", "XDTOValueType"): "type",
    ("Parameter", "TransferDirection"): "direction",
    # Метод шаблона URL: HTTP-метод - единственный метод, который у него есть.
    ("Method", "HTTPMethod"): "method",
}


# Наши поля, которых у прототипа нет: они не участвуют в сопоставлении.
OURS_ONLY = {"format", "id"}

# Осознанные расхождения: свойство у прототипа есть, у нас его нет нарочно, и
# причина записана в docs/requirements. Правило закрытия блока требует записывать
# такие решения именно для того, «чтобы следующая сверка не нашла его снова и не
# завела как ошибку», — а сверка про docs/requirements ничего не знает, поэтому
# список нужен здесь. Справа — где искать причину, чтобы строку можно было
# проверить, а не принять на веру.
#
# Прецедент: разделение данных общего реквизита было заведено пунктом карты как
# пробел, хотя в METADATA-OBJECTS.md стоял абзац «Разделения данных у общего
# реквизита нет» с причиной «одна организация — одна база». Сверка нашла, а
# читатель завёл.
ACCEPTED = {
    # Пока пусто, и это решение, а не недосмотр. По составу метаданных
    # расхождений с прототипом мы не держим: свойство, которого нет в модели,
    # исчезает при импорте, а это потеря смысла. Осознанное решение выражается
    # иначе - свойство несём, поведение не делаем, причину пишем в
    # docs/requirements. Такое свойство в модели есть, и сверка его не видит.
    #
    # Что здесь стояло и почему снято: девять свойств разделения данных общего
    # реквизита. Их отсутствие было записано как принципиальное, а оказалось
    # потерей: условное разделение живёт внутри элемента состава, и не неся его,
    # нельзя нести и состав - а один из шести общих реквизитов живой
    # конфигурации задан списком исключений из 331 объекта.
}

# Виды объектов, названные у нас иначе. Остальные, о которых скрипт сообщает в
# конце, лежат не в коллекциях каталога, а рядом: конфигурация и языки - в
# project, общие формы - в папке форм, параметр функциональной опции - у самой
# опции. Их состав сверяется руками, машинно здесь их не видно.
KIND_ALIAS = {"Enum": "Enumeration", "DefinedType": "DefinedTypeObject",
              "FunctionalOptionsParameter": "FunctionalOptionParameter"}


def tokens(name: str) -> frozenset[str]:
    """Имя как множество слов: порядок слов у нас и у прототипа разный."""
    spaced = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", name)
    return frozenset(part for part in re.split(r"[^A-Za-z0-9]+", spaced.lower()) if part)


def flat(name: str) -> str:
    return re.sub(r"[^a-z0-9]", "", name.lower())


def ordered(name: str) -> list[str]:
    """Слова имени по порядку: для группирующей подстановки порядок решает."""
    spaced = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", name)
    return [part for part in re.split(r"[^A-Za-z0-9]+", spaced.lower()) if part]


def stems_match(left: frozenset[str], right: frozenset[str]) -> bool:
    """Слова одной формы считаются одним словом: availability и available."""
    if len(left) != len(right):
        return False
    remaining = set(right)
    for word in left:
        found = None
        for candidate in remaining:
            shared = os.path.commonprefix([word, candidate])
            if candidate == word or (len(shared) >= 5 and len(shared) >= min(len(word), len(candidate)) - 3):
                found = candidate
                break
        if found is None:
            return False
        remaining.discard(found)
    return True


def matches(prototype: str, ours: dict[str, frozenset[str]], kind: str = "", top: frozenset[str] = frozenset()) -> bool:
    named = RENAMED_BY_KIND.get((kind, prototype), FLAT_ALIAS.get(prototype))
    if named is not None:
        return named in ours
    theirs = tokens(prototype)
    # Прототип называет каждый слот формы отдельным свойством - DefaultForm,
    # DefaultListForm, DefaultFolderChoiceForm; у нас слоты лежат в одной
    # структуре forms, и сверять их состав надо отдельно, а не по этим именам.
    if "form" in theirs and "forms" in ours:
        return True
    renamed = frozenset(filter(None, (RENAMED.get(word, word) for word in theirs)))
    flat_theirs = flat(prototype)
    for our_name, our_tokens in ours.items():  # noqa: PLR1702
        if theirs == our_tokens or renamed == our_tokens:
            return True
        if flat_theirs == flat(our_name):
            return True
        # Имя прототипа целиком внутри нашего или наоборот: у нас свойство
        # часто сложено в структуру, и её имя несёт лишнее слово.
        # Только одно направление: имя прототипа целиком внутри нашего. Наше
        # имя внутри их имени - подстановка опасная, и она уже врала: наше
        # `value` внутри filling схватило их `DataSeparationValue`, и принятое
        # расхождение перестало считаться расхождением. Одно общее слово не
        # делает два свойства одним свойством.
        if theirs <= our_tokens or renamed <= our_tokens:
            return True
        # Наше имя в начале их имени - и только в начале. Прототип называет
        # свойства группы «группа плюс признак»: CodeLength, CodeType,
        # NumberPeriodicity, - поэтому наше `code` стоит за всеми тремя. А
        # `types` в конце `LinkByType` не группа, а признак, и подстановка по
        # вхождению куда угодно хватала связь по типу за наш набор типов.
        # Вдобавок имя должно быть верхнеуровневым: лист группой не бывает.
        if our_name in top:
            theirs_words = [RENAMED.get(word, word) for word in ordered(prototype)]
            our_words = ordered(our_name)
            if theirs_words[: len(our_words)] == our_words:
                return True
        if stems_match(theirs, our_tokens) or stems_match(renamed, our_tokens):
            return True
    return False


# Проверки на сам сопоставитель. Каждая строка: вид объекта, имя прототипа,
# наши имена, наши имена верхнего уровня, ожидание.
#
# Зачем они есть. Инструмент, который ищет ложную тишину, сам врал ложным
# совпадением пять раз за один день — то есть молчал на настоящем пробеле, — и
# каждый раз это находилось разглядыванием вывода, а не проверкой. Все пять
# случаев ниже стоят поимённо; если сопоставитель снова начнёт хватать чужое
# имя по одному общему слову, красной станет эта таблица, а не чей-то отчёт
# через месяц.
SELF_CHECK = [
    # (вид, имя прототипа, наши имена, наши верхнеуровневые, ожидание)
    # Врал: наши имена хранились по набору слов, и два имени из одних слов -
    # управляемые формы в обычном приложении и обычные в управляемом -
    # затирали друг друга. Пропавшее имя не находилось, хотя у нас было.
    ("Configuration", "UseManagedFormInOrdinaryApplication",
     ["use_managed_forms_in_ordinary_application", "use_ordinary_forms_in_managed_application"],
     ["use_managed_forms_in_ordinary_application", "use_ordinary_forms_in_managed_application"], True),
    ("CommonCommand", "ToolTip", ["tooltip"], ["tooltip"], True),
    ("CommonPicture", "AvailabilityForAppearance", ["available_for_appearance"], ["available_for_appearance"], True),
    # Врал: одно общее слово «тип» не делает связь по типу нашим набором типов.
    ("Constant", "LinkByType", ["types"], ["types"], False),
    # Врал: наш лист `value` внутри заполнения хватал их разделение данных.
    ("CommonAttribute", "DataSeparationValue", ["filling", "value"], ["filling"], False),
    # Врал: слоты форм прототип называет по одному, у нас они в одной структуре.
    ("DataProcessor", "DefaultForm", ["forms", "main"], ["forms", "main"], True),
    ("Report", "DefaultSettingsForm", ["forms", "settings"], ["forms", "settings"], True),
    # Нужное направление: одно наше имя стоит за несколькими их свойствами.
    ("Catalog", "CodeLength", ["code"], ["code"], True),
    ("Document", "NumberPeriodicity", ["number"], ["number"], True),
    # То же направление, но от листа - запрещено.
    ("Catalog", "DescriptionLength", ["description"], [], False),
    # Точные алиасы вложенных.
    ("Constant", "PasswordMode", ["presentation", "password"], ["presentation"], True),
    ("CommonAttribute", "FillValue", ["filling", "value"], ["filling"], True),
    # Осознанные переименования.
    ("Catalog", "Synonym", ["title"], ["title"], True),
    ("Document", "RegisterRecords", ["movements"], ["movements"], True),
    ("Sequence", "RegisterRecords", ["movements"], ["movements"], True),
    ("AccumulationRegister", "EnableTotalsSplitting", ["allow_totals_splitting"], ["allow_totals_splitting"], True),
    # Совпадение имя в имя.
    ("Catalog", "Indexing", ["indexing"], ["indexing"], True),
    ("CommonModule", "Comment", ["comment"], ["comment"], True),
    # Настоящие пробелы должны остаться пробелами.
    ("CommonModule", "Global", ["client", "server", "privileged"], ["client", "server", "privileged"], False),
    ("ChartOfAccounts", "StandardTabularSections", ["standard_attributes"], ["standard_attributes"], False),
    ("Subsystem", "UseOneCommand", ["members", "parent"], ["members", "parent"], False),
    # Врал молчанием наоборот: у документа эти свойства есть, лежат внутри
    # posting, и остаток объявил их отсутствующими. Отсюда же вывод, который
    # стоит помнить: позицию остатка надо сверить с моделью по имени, прежде чем
    # называть её пробелом.
    ("Document", "PostInPrivilegedMode", ["posting", "privileged"], ["posting"], True),
    ("Document", "RealTimePosting", ["posting", "real_time"], ["posting"], True),
    ("BusinessProcess", "CreateTaskInPrivilegedMode", ["create_tasks_privileged"], ["create_tasks_privileged"], True),
    ("InformationRegister", "EnableTotalsSliceFirst", ["slice_first"], ["slice_first"], True),
    # Свойство элемента состава - свойство вида: выгрузка считает так же.
    ("CommonAttribute", "ConditionalSeparation", ["content", "conditional_separation"], ["content"], True),
    ("Catalog", "LimitLevelCount", ["hierarchy", "limit_levels"], ["hierarchy"], True),
    ("ChartOfAccounts", "StandardTabularSections", ["standard_table_parts"], ["standard_table_parts"], True),
    # И то, что осталось пробелом после проверки по модели поимённо.
    ("DataProcessor", "ExtendedPresentation", ["explanation", "title"], ["explanation", "title"], False),
    ("Sequence", "Comment", ["name", "documents"], ["name", "documents"], False),
    # Дочерние объекты. Область сверки - дочерний вид, и это не мелочь: под
    # именем владельца те же два свойства команды не совпадали ни разу и стояли
    # в остатке у семи видов сразу.
    ("Command", "CommandParameterType", ["parameter", "parameter_use"], ["parameter", "parameter_use"], True),
    ("Command", "OnMainServerUnavalableBehavior", ["on_server_unavailable"], ["on_server_unavailable"], True),
    ("Dimension", "DocumentMap", ["document_attributes"], ["document_attributes"], True),
    ("Dimension", "RegisterRecordsMap", ["register_dimensions"], ["register_dimensions"], True),
    ("AddressingAttribute", "AddressingDimension", ["dimension"], ["dimension"], True),
    ("Operation", "XDTOReturningValueType", ["return_type"], ["return_type"], True),
    ("Parameter", "XDTOValueType", ["type"], ["type"], True),
    ("Parameter", "TransferDirection", ["direction"], ["direction"], True),
    ("Method", "HTTPMethod", ["method"], ["method"], True),
    # И то, что осталось пробелом: у команды под именем владельца алиаса нет.
    ("Catalog", "CommandParameterType", ["parameter", "parameter_use"], ["parameter"], False),
]


def self_check() -> int:
    """Прогнать таблицу выше. Материалы не нужны: проверяется сопоставитель."""
    failures = []
    for kind, prototype, names, top, expected in SELF_CHECK:
        ours = {name: tokens(name) for name in names}
        got = matches(prototype, ours, kind, frozenset(top))
        if got != expected:
            wanted = "совпадение" if expected else "расхождение"
            failures.append(f"{kind}.{prototype} против {names}: ожидалось {wanted}, вышло наоборот")
    for line in failures:
        print("  " + line)
    print(f"проверок сопоставителя: {len(SELF_CHECK)}, не прошло: {len(failures)}")
    return 1 if failures else 0


def dump_model() -> dict:
    with tempfile.TemporaryDirectory() as folder:
        out = pathlib.Path(folder) / "model.json"
        result = subprocess.run(
            ["go", "test", "./internal/metadata/", "-run", "TestDumpMetadataComposition", "-count=1"],
            cwd=ROOT, env={**os.environ, "ML_METADATA_DUMP": str(out)},
            capture_output=True, text=True,
        )
        if result.returncode != 0:
            sys.exit("не удалось выгрузить состав модели:\n" + result.stdout + result.stderr)
        return json.loads(out.read_text(encoding="utf-8"))


def main() -> int:
    parser = argparse.ArgumentParser(description="сверка состава модели метаданных с выгрузкой")
    parser.add_argument("--kind", help="один вид объекта метаданных, как он назван в выгрузке")
    parser.add_argument("--help-pass", action="store_true",
                        help="третий проход: свойства объектов метаданных по синтакс-помощнику 8.3.27")
    parser.add_argument("--self-check", action="store_true",
                        help="прогнать проверки на сам сопоставитель; материалы не нужны")
    arguments = parser.parse_args()
    if arguments.self_check:
        return self_check()
    if not EXPORT.exists():
        print(f"нет {EXPORT.relative_to(ROOT)}: выгрузка не входит в репозиторий, сверка запускается локально")
        return 0
    model = dump_model()
    if arguments.help_pass:
        return sweep_help(model, arguments.kind)
    ours, tops = {}, {}
    for typename, body in model.items():
        kind = typename[: -len("Definition")] if typename.endswith("Definition") else typename
        # OURS_ONLY: наши поля, которых у прототипа нет вовсе, из сопоставления
        # убираются - иначе они ловят чужие имена и скрывают настоящий пробел.
        # `format` у нас версия формата файла метаданных, и она совпадала с
        # «Форматом» прототипа, из-за чего константа выглядела полной.
        # Свойства подчинённых структур считаются свойствами вида, потому что
        # так их считает и выгрузка: property-index собран по видам, и
        # `ConditionalSeparation` элемента состава стоит там у общего реквизита.
        # Отдельная сверка самих подчинённых объектов - это другая работа, и её
        # тут нет.
        names = [name for name in body["own"] if name not in OURS_ONLY]
        for child in body.get("children", {}).values():
            names.extend(name for name in child["own"] if name not in OURS_ONLY)
        ours[kind.lower()] = {name: tokens(name) for name in names}
        tops[kind.lower()] = frozenset(name for name in body.get("top", []) if name not in OURS_ONLY)
    export: dict[str, dict[str, int]] = {}
    for prop in json.loads(EXPORT.read_text(encoding="utf-8"))["properties"]:
        for kind, count in prop["object_kinds"].items():
            export.setdefault(kind, {})[prop["name"]] = count
    for exported, mine in KIND_ALIAS.items():
        if mine.lower() in ours:
            ours[exported.lower()] = ours[mine.lower()]
            tops[exported.lower()] = tops[mine.lower()]
    unknown = sorted(kind for kind in export if kind.lower() not in ours)
    total, accepted_total = 0, 0
    for kind in sorted(export):
        if arguments.kind and kind.lower() != arguments.kind.lower():
            continue
        fields = ours.get(kind.lower())
        if fields is None:
            continue
        missing, accepted = [], []
        for name, count in sorted(export[kind].items()):
            if matches(name, fields, kind, tops.get(kind.lower(), frozenset())):
                continue
            where = ACCEPTED.get((kind, name))
            if where is not None:
                accepted.append(name)
                continue
            missing.append((name, count))
        if accepted:
            accepted_total += len(accepted)
        if not missing:
            continue
        total += len(missing)
        print(f"=== {kind}: {len(missing)}")
        print("    " + ", ".join(f"{name} ({count})" for name, count in missing))
    total += sweep_children(model, arguments.kind)
    print(f"\nв остатке свойств: {total}")
    if accepted_total:
        print(f"принятых расхождений пропущено: {accepted_total} (см. ACCEPTED в этом скрипте)")
    if unknown and not arguments.kind:
        print(f"видов выгрузки без структуры у нас: {', '.join(unknown)}")
    print("Остаток - вопросы к синтакс-помощнику, а не перечень ошибок: отсутствие")
    print("свойства в выгрузке значит только, что она им не пользуется.")
    return 0


def sweep_children(model: dict, only: str | None) -> int:
    """Сверить состав дочерних объектов: измерений, ресурсов, реквизитов, частей.

    Отдельным проходом, потому что источник другой - child-objects.json, где
    состав свойств лежит по тройке «вид, владелец, дочерний вид». Тройка, а не
    пара: два отношения выгрузки живут на второй ступени - метод шаблона URL и
    параметр операции веб-сервиса, - и по паре они бы не нашлись.

    Пара, которой у нас нет, называется вслух. Молчание здесь неотличимо от
    «расхождений нет», а именно молчанием четыре свойства измерения и простояли
    без сверки, пока этого прохода не было вовсе.
    """
    if not CHILDREN.exists():
        return 0
    relations = {}
    for relation in json.loads(CHILDREN.read_text(encoding="utf-8"))["relations"]:
        relations[(relation["metadata_kind"], relation["owner_kind"], relation["child_kind"])] = relation
    ours, tops = {}, {}
    for typename, body in model.items():
        kind = typename[: -len("Definition")] if typename.endswith("Definition") else typename
        for collection, dumped in body.get("children", {}).items():
            child = CHILD_COLLECTIONS_BY_KIND.get((kind, collection), CHILD_COLLECTIONS.get(collection))
            if child is None:
                continue
            key = (kind.lower(), kind, child)
            ours[key] = {name: tokens(name) for name in dumped["own"] if name not in OURS_ONLY}
            tops[key] = frozenset(name for name in dumped["top"] if name not in OURS_ONLY)
            for nested, below in dumped.get("children", {}).items():
                grandchild = GRANDCHILD_COLLECTIONS.get((child, nested))
                if grandchild is None:
                    continue
                key = (kind.lower(), child, grandchild)
                ours[key] = {name: tokens(name) for name in below["own"] if name not in OURS_ONLY}
                tops[key] = frozenset(name for name in below["top"] if name not in OURS_ONLY)
    # Виды, названные у нас иначе, - тот же список, что и в первом проходе.
    for exported, mine in KIND_ALIAS.items():
        for (kind, owner, child), fields in list(ours.items()):
            if kind == mine.lower():
                renamed = (exported.lower(), exported if owner == mine else owner, child)
                ours[renamed] = fields
                tops[renamed] = tops[(kind, owner, child)]
    total, unseen = 0, []
    for (kind, owner, child), relation in sorted(relations.items()):
        if only and kind.lower() != only.lower():
            continue
        fields = ours.get((kind.lower(), owner, child))
        if fields is None:
            # Пропуск называется вслух. Пропущенная пара - это молчание, а
            # молчание здесь неотличимо от «расхождений нет»: ровно так четыре
            # свойства измерения и простояли без сверки, пока второго прохода
            # не было вовсе.
            if relation["properties"]:
                unseen.append(f"{kind}.{child}" if owner == kind else f"{kind}.{owner}.{child}")
            continue
        # Имя свойства дочернего объекта сверяется в области самого дочернего
        # вида, а не его владельца: `CommandParameterType` значит у команды
        # одно и то же под справочником, документом и обработкой - потому она и
        # стояла в остатке семь раз подряд.
        missing = [(name, count) for name, count in sorted(relation["properties"].items())
                   if not matches(name, fields, child, tops[(kind.lower(), owner, child)])]
        if not missing:
            continue
        total += len(missing)
        path = f"{kind}.{child}" if owner == kind else f"{kind}.{owner}.{child}"
        print(f"=== {path} ({relation['count']}): {len(missing)}")
        print("    " + ", ".join(f"{name} ({count})" for name, count in missing))
    if unseen:
        print(f"дочерних видов выгрузки без коллекции у нас: {', '.join(unseen)}")
    return total


def sweep_help(model: dict, only: str | None) -> int:
    """Сверить состав модели со свойствами объектов метаданных синтакс-помощника.

    Выгрузка показывает, чем пользуется ОНА, и отсутствие свойства в ней
    ничего не доказывает. Синтакс-помощник перечисляет свойства объекта
    метаданных целиком - страницы «ОбъектМетаданных: Вид.Свойство» в разделе
    свойств, без методов. Английские имена в скобках там те же, что в
    выгрузке, поэтому сопоставитель и переименования общие.

    Подчинённый вид - измерение, ресурс, реквизит - сверяется с объединением
    одноимённых коллекций всех владельцев: свойство несём, если оно есть хоть
    у одного. Применимость по владельцу - другой вопрос, и этот проход его не
    решает.
    """
    if not HELP.exists():
        print(f"нет {HELP.relative_to(ROOT)}: справка не входит в репозиторий")
        return 0
    help_props: dict[str, set[str]] = {}
    pattern = re.compile(r"ОбъектМетаданных: [^.()]+\.[^ ()]+ \(MetadataObject: ([^.()]+)\.([^ ()]+)\)")
    for page in json.loads(HELP.read_text(encoding="utf-8"))["pages"]:
        found = pattern.match(page["title"])
        if found and "/properties/" in page["path"]:
            kind = HELP_KIND_ALIAS.get(found.group(1), found.group(1))
            help_props.setdefault(kind, set()).add(found.group(2))
    ours: dict[str, dict] = {}
    tops: dict[str, frozenset] = {}

    def add(kind: str, names, top) -> None:
        fields = ours.setdefault(kind.lower(), {})
        fields.update({name: tokens(name) for name in names if name not in OURS_ONLY})
        tops[kind.lower()] = tops.get(kind.lower(), frozenset()) | frozenset(name for name in top if name not in OURS_ONLY)

    for typename, body in model.items():
        kind = typename[: -len("Definition")] if typename.endswith("Definition") else typename
        names = list(body["own"])
        for child in body.get("children", {}).values():
            names.extend(child["own"])
        add(kind, names, body.get("top", []))
        def walk(owner: str, children: dict) -> None:
            for collection, dumped in children.items():
                child = (CHILD_COLLECTIONS_BY_KIND.get((owner, collection))
                         or GRANDCHILD_COLLECTIONS.get((owner, collection))
                         or HELP_CHILD_COLLECTIONS.get(collection)
                         or CHILD_COLLECTIONS.get(collection))
                if child is None:
                    continue
                add(child, dumped["own"], dumped["top"])
                walk(child, dumped.get("children", {}))

        walk(kind, body.get("children", {}))
    for exported, mine in KIND_ALIAS.items():
        if mine.lower() in ours:
            ours[exported.lower()] = ours[mine.lower()]
            tops[exported.lower()] = tops[mine.lower()]
    total, unseen = 0, []
    extensions, as_files, decided = 0, 0, 0
    for kind in sorted(help_props):
        if only and kind.lower() != only.lower():
            continue
        fields = ours.get(kind.lower())
        if fields is None:
            if kind not in HELP_DECIDED_KINDS:
                unseen.append(kind)
            continue
        missing = []
        for name in sorted(help_props[kind]):
            if matches(name, fields, kind, tops.get(kind.lower(), frozenset())) or (kind, name) in ACCEPTED:
                continue
            renamed = HELP_RENAMED.get((kind, name), HELP_RENAMED_ANY.get(name))
            if renamed is not None and renamed in fields:
                continue
            if name in HELP_EXTENSIONS:
                extensions += 1
                continue
            if name in HELP_AS_FILES or (kind, name) in HELP_AS_FILES_BY_KIND:
                as_files += 1
                continue
            if (kind, name) in HELP_DECIDED:
                decided += 1
                continue
            missing.append(name)
        if not missing:
            continue
        total += len(missing)
        print(f"=== {kind}: {len(missing)} из {len(help_props[kind])}")
        print("    " + ", ".join(missing))
    print(f"\nв остатке свойств по синтакс-помощнику: {total}")
    print(f"лежат файлами рядом с описанием (модули, тела объектов): {as_files}")
    print(f"решены в docs/requirements: {decided}")
    print(f"свойства расширений конфигурации, расширения не поддерживаются: {extensions}")
    if unseen:
        print(f"объектов метаданных справки без структуры у нас: {', '.join(unseen)}")
    print("Остаток - вопросы: свойство может лежать у нас под другим именем или")
    print("вложенным, и каждое проверяется глазами, прежде чем стать пунктом.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
