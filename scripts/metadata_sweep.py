#!/usr/bin/env python3
"""Сверить состав нашей модели метаданных с выгрузкой конфигурации-прототипа.

Зачем. Правило закрытия блока требует сверки с `docs/materials` по области
блока целиком, а не по его диффу. Состав свойств можно сверить машинно: в
`docs/materials/platform-model/01-metadata/property-index.json` лежат свойства
по видам объектов, собранные из XML-выгрузки, и их можно сопоставить с полями
наших структур. Находить такие расхождения по одному, спотыкаясь, — худший
способ: так нашлись индексирование у ресурса, признак учёта субконто у
измерения и связи параметров выбора, не достающие до измерений.

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
    ("Document", "RegisterRecords"): "movements",
    ("Sequence", "RegisterRecords"): "movements",
    ("Subsystem", "Content"): "members",
    ("FilterCriterion", "Content"): "fields",
    ("CommonTemplate", "TemplateType"): "kind",
    ("EventSubscription", "Handler"): "procedure",
    ("EventSubscription", "Source"): "objects",
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
KIND_ALIAS = {"Enum": "Enumeration", "DefinedType": "DefinedTypeObject"}


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


def matches(prototype: str, ours: dict[frozenset[str], str], kind: str = "", top: frozenset[str] = frozenset()) -> bool:
    named = RENAMED_BY_KIND.get((kind, prototype), FLAT_ALIAS.get(prototype))
    if named is not None:
        return any(name == named for name in ours.values())
    theirs = tokens(prototype)
    # Прототип называет каждый слот формы отдельным свойством - DefaultForm,
    # DefaultListForm, DefaultFolderChoiceForm; у нас слоты лежат в одной
    # структуре forms, и сверять их состав надо отдельно, а не по этим именам.
    if "form" in theirs and any(name == "forms" for name in ours.values()):
        return True
    renamed = frozenset(filter(None, (RENAMED.get(word, word) for word in theirs)))
    flat_theirs = flat(prototype)
    for our_tokens, our_name in ours.items():  # noqa: PLR1702
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
]


def self_check() -> int:
    """Прогнать таблицу выше. Материалы не нужны: проверяется сопоставитель."""
    failures = []
    for kind, prototype, names, top, expected in SELF_CHECK:
        ours = {tokens(name): name for name in names}
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
    parser.add_argument("--self-check", action="store_true",
                        help="прогнать проверки на сам сопоставитель; материалы не нужны")
    arguments = parser.parse_args()
    if arguments.self_check:
        return self_check()
    if not EXPORT.exists():
        print(f"нет {EXPORT.relative_to(ROOT)}: выгрузка не входит в репозиторий, сверка запускается локально")
        return 0
    model = dump_model()
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
            names.extend(name for name in child if name not in OURS_ONLY)
        ours[kind.lower()] = {tokens(name): name for name in names}
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
    print(f"\nв остатке свойств: {total}")
    if accepted_total:
        print(f"принятых расхождений пропущено: {accepted_total} (см. ACCEPTED в этом скрипте)")
    if unknown and not arguments.kind:
        print(f"видов выгрузки без структуры у нас: {', '.join(unknown)}")
    print("Остаток - вопросы к синтакс-помощнику, а не перечень ошибок: отсутствие")
    print("свойства в выгрузке значит только, что она им не пользуется.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
