#!/usr/bin/env python3
"""Convert extracted 1C help JSON into a normalized BSL bridge model."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path
from typing import Any


SECTION_HEADERS = {
    "Синтаксис:",
    "Параметры:",
    "Возвращаемое значение:",
    "Описание:",
    "Доступность:",
    "Использование:",
    "Пример:",
    "Примечание:",
    "Внимание:",
    "См. также:",
    "Использование в версии:",
    "Конструкторы:",
    "Методы:",
    "Свойства:",
    "События:",
}


def is_section_header(line: str) -> bool:
    return any(line.startswith(header) for header in SECTION_HEADERS) or line.startswith(
        "Вариант синтаксиса:"
    )


def section(lines: list[str], header: str) -> list[str]:
    try:
        start = lines.index(header) + 1
    except ValueError:
        return []
    result: list[str] = []
    for line in lines[start:]:
        if is_section_header(line):
            break
        result.append(line)
    return result


def all_syntaxes(lines: list[str]) -> list[str]:
    result: list[str] = []
    for index, line in enumerate(lines):
        if line != "Синтаксис:":
            continue
        for value in lines[index + 1 :]:
            if is_section_header(value):
                break
            if value:
                result.append(value)
    return list(dict.fromkeys(result))


def parse_types(value: str) -> list[str]:
    value = value.removeprefix("Тип:").strip().rstrip(".")
    return [item.strip() for item in value.split(",") if item.strip()]


def parse_parameters(lines: list[str]) -> list[dict[str, Any]]:
    block = section(lines, "Параметры:")
    parameters: list[dict[str, Any]] = []
    current: dict[str, Any] | None = None

    for line in block:
        # Справка помечает обязательность у методов, но не у событий: там
        # параметр объявлен одной строкой «<Имя>». Требовать пометку значит
        # потерять параметры всех событий разом, а их там 596 из 689.
        match = re.match(r"^<(.+?)>(?:\s+\((обязательный|необязательный)\))?$", line)
        if match:
            if current:
                current["description"] = "\n".join(current.pop("description_lines"))
                parameters.append(current)
            current = {
                "name": match.group(1),
                "required": match.group(2) != "необязательный",
                "requirement_stated": match.group(2) is not None,
                "types": [],
                "default": None,
                "description_lines": [],
            }
            continue
        if current is None:
            continue
        if line.startswith("Тип:"):
            current["types"] = parse_types(line)
        elif line.startswith("Значение по умолчанию:"):
            current["default"] = line.removeprefix("Значение по умолчанию:").strip()
        else:
            current["description_lines"].append(line)

    if current:
        current["description"] = "\n".join(current.pop("description_lines"))
        parameters.append(current)
    return parameters


def parse_return(lines: list[str]) -> dict[str, Any] | None:
    block = section(lines, "Возвращаемое значение:")
    if not block:
        return None
    types: list[str] = []
    description: list[str] = []
    for line in block:
        if line.startswith("Тип:"):
            types = parse_types(line)
        else:
            description.append(line)
    return {"types": types, "description": "\n".join(description)}


def parse_availability(lines: list[str]) -> list[str]:
    value = " ".join(section(lines, "Доступность:")).rstrip(".")
    return [item.strip() for item in value.split(",") if item.strip()]


def split_owner(name: str | None) -> tuple[str | None, str | None]:
    if not name:
        return None, None
    if "." not in name:
        return None, name
    return tuple(name.rsplit(".", 1))  # type: ignore[return-value]


def classify_page(page: dict[str, Any]) -> str:
    kind = page["kind"]
    if kind != "page":
        return kind
    if page["title_ru"] == "Глобальный контекст":
        return "global_context"
    text = page["text"]
    if any(header in text for header in ("\nКонструкторы:\n", "\nМетоды:\n", "\nСвойства:\n", "\nСобытия:\n")):
        return "type"
    return "topic"


def normalize_context_page(page: dict[str, Any]) -> dict[str, Any]:
    lines = page["text"].splitlines()
    kind = classify_page(page)
    # У члена типа заголовок — «Тип.Член», и владелец берётся отсечением
    # последней точки. У самого типа точка тоже бывает, но она часть имени:
    # «СправочникОбъект.<Имя справочника>» — это один тип, а не член типа
    # «СправочникОбъект». Отсекать её значит разорвать дерево ровно там, где
    # живут параметризованные типы, и члены такого типа повисают сиротами.
    if kind in ("type", "global_context", "topic"):
        owner_ru, name_ru = None, page.get("title_ru")
        owner_en, name_en = None, page.get("title_en")
    else:
        owner_ru, name_ru = split_owner(page.get("title_ru"))
        owner_en, name_en = split_owner(page.get("title_en"))

    description_lines = section(lines, "Описание:")
    property_types: list[str] = []
    if kind == "property":
        for line in description_lines:
            if line.startswith("Тип:"):
                property_types = parse_types(line)
                break

    version_pattern = r"[0-9]+(?:\.[0-9]+)*"
    since_match = re.search(
        rf"Доступен, начиная с версии ({version_pattern})",
        page["text"],
    )
    changed_versions = re.findall(
        rf"Описание изменено в версии ({version_pattern})",
        page["text"],
    )

    return {
        "id": page["id"],
        "kind": kind,
        "name_ru": name_ru,
        "name_en": name_en,
        "owner_ru": owner_ru,
        "owner_en": owner_en,
        "syntaxes": all_syntaxes(lines),
        "parameters": parse_parameters(lines),
        "return": parse_return(lines),
        "value_types": property_types,
        "access": "\n".join(section(lines, "Использование:")),
        "availability": parse_availability(lines),
        "description": "\n".join(description_lines),
        "example": "\n".join(section(lines, "Пример:")),
        "since_version": since_match.group(1) if since_match else None,
        "changed_versions": list(dict.fromkeys(changed_versions)),
        "source_path": page["path"],
        "source_text": page["text"],
    }


SIGNATURE = re.compile(r"^[A-Za-zА-Яа-яЁё][A-Za-zА-Яа-яЁё_0-9]*\s*\(")
ANGLE_PARAMETER = re.compile(r"^<(?P<name>[^>]+)>\s*[—–-]?\s*(?P<description>.*)$")
PLAIN_PARAMETER = re.compile(r"^(?P<name>[A-Za-zА-Яа-яЁё_][\wА-Яа-яЁё ]{0,40}?)\s+[—–-]\s+(?P<description>.+)$")
SYNTAX_SECTIONS = {
    "синтаксис:": "syntax",
    "параметры:": "parameters",
    "описание:": "description",
    "возвращаемое значение:": "return",
    "пример:": "example",
    "примеры:": "example",
    "доступность:": "availability",
    "использование в версии:": "skip",
    "методическая информация": "skip",
}


def normalize_syntax_page(page: dict[str, Any], kind: str) -> dict[str, Any]:
    """Разобрать страницу конструкции языка или языка запросов.

    Такая страница устроена не как страница метода. У конструкций языка есть
    явные разделы «Синтаксис:», «Параметры:», «Описание:». У функций языка
    запросов разделов нет вовсе: подпись стоит первой строкой, под ней
    параметры без угловых скобок, и возврат отдельной строкой. Раньше всё это
    оставалось одним куском текста, то есть функции языка запросов лежали в
    модели только именами и для написания движка были бесполезны.
    """
    title = (page.get("title") or "").strip()
    lines = [line.strip() for line in (page.get("text") or "").splitlines() if line.strip()]
    while lines and lines[0] in (title, page.get("title_ru"), f"{page.get('title_ru')} ({page.get('title_en')})"):
        lines.pop(0)

    section = "head"
    syntaxes: list[str] = []
    parameters: list[dict[str, Any]] = []
    description: list[str] = []
    examples: list[str] = []
    returns: list[str] = []
    see_also: list[str] = []

    def remember_parameter(line: str) -> bool:
        match = ANGLE_PARAMETER.match(line)
        if match and match.group("description"):
            parameters.append({"name": match.group("name").strip(), "description": match.group("description").strip()})
            return True
        match = PLAIN_PARAMETER.match(line)
        if not match:
            return False
        name = match.group("name").strip()
        # Вне раздела «Параметры:» строка «Имя — описание» опознаётся только
        # если это имя действительно стоит в подписи функции. Иначе под правило
        # попадёт обычная проза, и в параметрах окажется половина описания.
        if section == "parameters" or (syntaxes and f"<{name}>" in syntaxes[0]):
            parameters.append({"name": name, "description": match.group("description").strip()})
            return True
        return False

    for index, line in enumerate(lines):
        lowered = line.lower()
        if lowered in SYNTAX_SECTIONS:
            section = SYNTAX_SECTIONS[lowered]
            continue
        if lowered.startswith("см. также:"):
            see_also.append(line.split(":", 1)[1].strip())
            continue
        if lowered.startswith("возвращаемое значение:"):
            returns.append(line.split(":", 1)[1].strip())
            continue
        if section == "skip":
            continue
        if section == "syntax":
            syntaxes.append(line)
            continue
        if section == "example":
            examples.append(line)
            continue
        if section == "return":
            returns.append(line)
            continue
        # Подпись функции языка запросов стоит первой строкой, без раздела.
        if section == "head" and not syntaxes and index == 0 and SIGNATURE.match(line):
            syntaxes.append(line)
            continue
        if remember_parameter(line):
            continue
        description.append(line)

    return {
        "id": page["id"],
        "kind": kind,
        "name_ru": page.get("title_ru"),
        "name_en": page.get("title_en"),
        "syntaxes": syntaxes,
        "parameters": parameters,
        "return": {"types": [], "description": "\n".join(returns)} if returns else None,
        "description": "\n".join(description),
        "example": "\n".join(examples),
        "see_also": see_also,
        "source_path": page["path"],
        "source_text": page["text"],
    }


def load_json(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as stream:
        return json.load(stream)


def build_model(input_dir: Path, output: Path, page_index: Path | None = None) -> dict[str, int]:
    context = load_json(input_dir / "shcntx_ru.json")
    language = load_json(input_dir / "shlang_ru.json")
    query = load_json(input_dir / "shquery_ru.json")

    symbols = [normalize_context_page(page) for page in context["pages"]]
    symbols.extend(normalize_syntax_page(page, "language_syntax") for page in language["pages"])
    symbols.extend(normalize_syntax_page(page, "query_syntax") for page in query["pages"])

    # Служебный индекс добавляет то, чего в тексте страницы нет: коды
    # доступности там, где прозой она не написана, и каноническое короткое имя.
    if page_index and page_index.exists():
        index = json.loads(page_index.read_text(encoding="utf-8")).get("pages", {})
        books = {"shcntx_ru": context, "shlang_ru": language, "shquery_ru": query}
        for symbol in symbols:
            path = symbol.get("source_path")
            if not path:
                continue
            for book in books:
                entry = index.get(book + "/" + path)
                if not entry:
                    continue
                if entry.get("contexts"):
                    symbol["availability_codes"] = entry["contexts"]
                if entry.get("short_ru"):
                    symbol["short_ru"] = entry["short_ru"]
                if entry.get("short_en"):
                    symbol["short_en"] = entry["short_en"]
                if entry.get("since_version") and not symbol.get("since_version"):
                    symbol["since_version"] = entry["since_version"]
                break

    counts: dict[str, int] = {}
    for symbol in symbols:
        counts[symbol["kind"]] = counts.get(symbol["kind"], 0) + 1

    model = {
        "schema": "metalab.bsl-bridge",
        "schema_version": 1,
        "platform_version": context["platform_version"],
        "language": "ru",
        "symbol_count": len(symbols),
        "counts": counts,
        "symbols": symbols,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(model, ensure_ascii=False, separators=(",", ":")),
        encoding="utf-8",
    )
    return counts


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("input_dir", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument(
        "--page-index",
        type=Path,
        default=Path("docs/materials/its/8.3.27.2342/page-index.json"),
        help="служебный индекс страниц: коды доступности и короткие имена",
    )
    args = parser.parse_args()
    counts = build_model(args.input_dir, args.output, args.page_index)
    print(f"Created {args.output}: {json.dumps(counts, ensure_ascii=False, sort_keys=True)}")


if __name__ == "__main__":
    main()
