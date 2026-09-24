#!/usr/bin/env python3
"""Собрать каталог таблиц языка запросов из разобранной справки.

Раздел `tables/` справки описывает то, из чего пишется запрос: реальные
таблицы объектов, виртуальные таблицы вроде среза последних и остатков, поля
каждой таблицы с типами и параметры виртуальных таблиц с обязательностью.

В общей модели символов эти страницы лежат как «прочее»: у них нет ни
владельца, ни вида, потому что таблица запроса — не тип и не метод. Отдельный
каталог собирает их обратно в дерево «таблица → поля → параметры», без
которого движок запросов пришлось бы писать по прозе справки.

Использование:
    python3 scripts/its_query_tables.py [--json <файл>] [--output <файл>]
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

DEFAULT_PAGES = Path("docs/materials/its/8.3.27.2342/json/shcntx_ru.json")
DEFAULT_INDEX = Path("docs/materials/its/8.3.27.2342/page-index.json")
DEFAULT_OUTPUT = Path("docs/materials/its/query-tables/tables.json")
PLATFORM_VERSION = "8.3.27.2342"

TITLE = re.compile(r"^(?P<ru>.+?)\s+\((?P<en>[^()]*(?:\([^()]*\))?[^()]*)\)\s*$")
TYPE_LINE = re.compile(r"^Тип(?: параметра)?:\s*(?P<types>.+?)\.?$")
REQUIRED_LINE = re.compile(r"^(?P<name>.+?)\s+\((?P<need>обязательный|необязательный)\)$")
SERVICE_LINE = "Методическая информация"


def split_title(title: str) -> tuple[str, str | None]:
    match = TITLE.match(title.strip())
    if match:
        return match.group("ru").strip(), match.group("en").strip()
    return title.strip(), None


def parse_types(value: str) -> list[str]:
    return [item.strip() for item in re.split(r"[,;]| или ", value) if item.strip()]


def body_lines(page: dict) -> list[str]:
    title = (page.get("title") or "").strip()
    lines = [line.strip() for line in (page.get("text") or "").splitlines() if line.strip()]
    # Первые строки повторяют заголовок, последняя — служебная ссылка.
    while lines and title and lines[0] == title:
        lines.pop(0)
    return [line for line in lines if line != SERVICE_LINE]


def describe_field(page: dict) -> dict:
    name_ru, name_en = split_title(page.get("title", ""))
    field: dict = {"name_ru": name_ru, "source_path": page["path"]}
    if name_en:
        field["name_en"] = name_en
    description: list[str] = []
    for line in body_lines(page):
        match = TYPE_LINE.match(line)
        if match and "types" not in field:
            field["types"] = parse_types(match.group("types"))
            continue
        description.append(line)
    if description:
        field["description"] = " ".join(description)
    return field


def describe_parameter(page: dict) -> dict:
    name_ru, name_en = split_title(page.get("title", ""))
    parameter: dict = {"name_ru": name_ru, "source_path": page["path"], "required": True}
    if name_en:
        parameter["name_en"] = name_en
    description: list[str] = []
    for line in body_lines(page):
        need = REQUIRED_LINE.match(line)
        if need:
            parameter["required"] = need.group("need") == "обязательный"
            continue
        match = TYPE_LINE.match(line)
        if match and "types" not in parameter:
            parameter["types"] = parse_types(match.group("types"))
            continue
        description.append(line)
    if description:
        parameter["description"] = " ".join(description)
    return parameter


def table_key(path: str) -> str:
    """Каталог таблицы: «tables/catalog1/table3/fields/x.html» → «tables/catalog1/table3»."""
    trimmed = path[: -len(".html")] if path.endswith(".html") else path
    for marker in ("/fields/", "/params/"):
        if marker in trimmed:
            return trimmed.split(marker)[0]
    return trimmed


def build(pages: list[dict], index: dict) -> dict:
    tables: dict[str, dict] = {}
    for page in pages:
        path = page.get("path") or ""
        if not path.startswith("tables/"):
            continue
        key = table_key(path)
        table = tables.setdefault(key, {"id": key, "fields": [], "parameters": []})
        if "/fields/" in path:
            table["fields"].append(describe_field(page))
        elif "/params/" in path:
            table["parameters"].append(describe_parameter(page))
        else:
            name_ru, name_en = split_title(page.get("title", ""))
            table["name_ru"] = name_ru
            if name_en:
                table["name_en"] = name_en
            table["source_path"] = path
            entry = index.get("pages", {}).get("shcntx_ru/" + path, {})
            if entry.get("since_version"):
                table["since_version"] = entry["since_version"]
            if entry.get("contexts"):
                table["contexts"] = entry["contexts"]
            # Виртуальная таблица названа через точку от реальной: у неё имя
            # вида «РегистрНакопления.<Имя регистра>.Остатки».
            table["virtual"] = name_ru.count(".") >= 2

    result = []
    for key in sorted(tables):
        table = tables[key]
        if "name_ru" not in table:
            # Поля без своей таблицы означали бы потерю: такого быть не должно.
            table["name_ru"] = "(таблица без страницы: " + key + ")"
            table["virtual"] = False
        table["fields"].sort(key=lambda item: item["name_ru"])
        table["parameters"].sort(key=lambda item: item["source_path"])
        result.append(table)

    return {
        "schema": "metalab.its-query-tables",
        "schema_version": 1,
        "platform_version": PLATFORM_VERSION,
        "source": "docs/materials/its/8.3.27.2342/json/shcntx_ru.json",
        "table_count": len(result),
        "field_count": sum(len(item["fields"]) for item in result),
        "parameter_count": sum(len(item["parameters"]) for item in result),
        "tables": result,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--json", type=Path, default=DEFAULT_PAGES)
    parser.add_argument("--index", type=Path, default=DEFAULT_INDEX)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()
    if not args.json.exists():
        print("Нет " + str(args.json), file=sys.stderr)
        return 2
    pages = json.loads(args.json.read_text(encoding="utf-8"))["pages"]
    index = json.loads(args.index.read_text(encoding="utf-8")) if args.index.exists() else {}
    catalog = build(pages, index)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(catalog, ensure_ascii=False, indent=1), encoding="utf-8")
    virtual = sum(1 for item in catalog["tables"] if item.get("virtual"))
    print(
        f"Создан {args.output}: таблиц {catalog['table_count']} "
        f"(виртуальных {virtual}), полей {catalog['field_count']}, "
        f"параметров {catalog['parameter_count']}"
    )
    orphan = [item["id"] for item in catalog["tables"] if item["name_ru"].startswith("(таблица без страницы")]
    if orphan:
        print("  таблицы без собственной страницы:", ", ".join(orphan[:5]))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
