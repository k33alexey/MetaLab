#!/usr/bin/env python3
"""Сверить перечни TYPES и STANDARD-LIBRARY с выгрузкой интерфейса 8.3.27.

Зачем. `docs/requirements/TYPES.md` называет каждый тип значения прототипа, а
`STANDARD-LIBRARY.md` — каждую функцию и свойство глобального контекста: по ним
блок 5 сверяет язык. Сверка с выгрузкой интерфейса (`bsl-bridge.json`) делалась
в 2.364 скриптом один раз, и скрипт не сохранился: тип, дописанный руками не в
тот перечень или потерянный при правке, не заметил бы никто. Этот скрипт
повторяет ту сверку и стоит в репозитории, чтобы её не писать заново.

Что сверяется:

- TYPES — каждый нешаблонный тип выгрузки назван в тексте (имена с пробелами и
  запятыми тоже: перевод строки внутри имени не в счёт), у каждого шаблонного
  (`СправочникСсылка.<Имя справочника>`) назван префикс с `.<`; в разделах
  перечня после вычёркивания всех имён выгрузки не остаётся ни одного слова,
  похожего на имя типа, — то есть лишнего типа нет. Числа в шапке совпадают с
  выгрузкой;
- STANDARD-LIBRARY — функции разделов перечня (строки `- \\`Имя(`) и
  свойства раздела «Свойства глобального контекста» совпадают с глобальным
  контекстом выгрузки в обе стороны, без повторов; числа в шапке — тоже.

Чего не делает: не сверяет сигнатуры, доступность и разделы, по которым имена
разложены, — это читается глазами по справке.

Как запускать:

    python3 scripts/language_lists_check.py

Выгрузка интерфейса не входит в репозиторий, поэтому без неё скрипт говорит об
этом и выходит с нулём, как `metadata_sweep.py`.
"""

from __future__ import annotations

import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
BRIDGE = ROOT / "docs" / "materials" / "its" / "8.3.27.2342" / "bsl-bridge" / "bsl-bridge.json"
TYPES = ROOT / "docs" / "requirements" / "TYPES.md"
LIBRARY = ROOT / "docs" / "requirements" / "STANDARD-LIBRARY.md"
GLOBAL_CONTEXT = "Глобальный контекст"


def named(name: str, text: str) -> bool:
    return re.search(r"(?<![\w.])" + re.escape(name) + r"(?!\w)", text) is not None


def section(text: str, first: str, after: str) -> str:
    return text[text.index(first):text.index(after)]


def check_types(symbols: list[dict]) -> list[str]:
    names = sorted({symbol["name_ru"] for symbol in symbols if symbol["kind"] == "type"})
    plain = [name for name in names if "<" not in name]
    templates = sorted({name.split(".<")[0] for name in names if "<" in name})
    text = re.sub(r"\s+", " ", TYPES.read_text(encoding="utf-8"))
    problems = [f"TYPES: нет типа «{name}»" for name in plain if not named(name, text)]
    problems += [f"TYPES: нет шаблонного типа «{prefix}.<…>»" for prefix in templates
                 if not re.search(r"(?<!\w)" + re.escape(prefix) + r"\.<", text)]
    rest = section(text, "## Примитивы и базовые значения", "## Методы и свойства типов")
    for name in sorted(names + [prefix + ".<" for prefix in templates], key=len, reverse=True):
        rest = re.sub(r"(?<![\w.])" + re.escape(name) + r"(?!\w)", " ", rest)
    # Имя типа — слово из нескольких слов с заглавной внутри: так назван любой
    # тип перечня, а прочий текст разделов так не пишется.
    for word in sorted(set(re.findall(r"(?<![\w`])[A-ZА-ЯЁ][\wЁё]*[a-zа-яё][A-ZА-ЯЁ][\wЁё]*", rest))):
        problems.append(f"TYPES: «{word}» — не тип выгрузки")
    head = text[:text.index("## COM")]
    total = f"{len(plain) + len(templates)} типа: {len(plain)} типа платформы и {len(templates)} тип"
    if total not in head:
        problems.append(f"TYPES: шапка не говорит «{total}…»")
    return problems


def check_library(symbols: list[dict]) -> list[str]:
    context = [symbol for symbol in symbols if symbol.get("owner_ru") == GLOBAL_CONTEXT]
    methods = {symbol["name_ru"] for symbol in context if symbol["kind"] == "method"}
    properties = {symbol["name_ru"] for symbol in context if symbol["kind"] == "property"}
    text = LIBRARY.read_text(encoding="utf-8")
    listed = re.findall(r"^- `([^`(]+)\(", section(text, "## Ошибки и исключения", "## Свойства глобального контекста"), re.M)
    block = section(text, "## Свойства глобального контекста", "## Имена механизмов, от которых ML отказался")
    written = [name.strip(" .\n") for name in block.split(":", 1)[1].split(",")]
    problems = []
    for what, ours, theirs in (("функции", listed, methods), ("свойства", written, properties)):
        problems += [f"STANDARD-LIBRARY, {what}: «{name}» названо дважды" for name in sorted({n for n in ours if ours.count(n) > 1})]
        problems += [f"STANDARD-LIBRARY, {what}: «{name}» не названо" for name in sorted(theirs - set(ours))]
        problems += [f"STANDARD-LIBRARY, {what}: «{name}» нет в выгрузке" for name in sorted(set(ours) - theirs)]
    head = re.sub(r"\s+", " ", text[:text.index("## Ошибки и исключения")])
    total = f"{len(methods)} функции и процедуры и {len(properties)} свойство"
    if total not in head:
        problems.append(f"STANDARD-LIBRARY: шапка не говорит «{total}»")
    return problems


def main() -> int:
    if not BRIDGE.exists():
        print(f"нет {BRIDGE.relative_to(ROOT)}: выгрузка не входит в репозиторий, сверка запускается локально")
        return 0
    symbols = json.loads(BRIDGE.read_text(encoding="utf-8"))["symbols"]
    problems = check_types(symbols) + check_library(symbols)
    for line in problems:
        print("  " + line)
    print(f"расхождений перечней с выгрузкой интерфейса: {len(problems)}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
