#!/usr/bin/env python3
"""Поиск по разобранным книгам из `docs/materials/books/`.

Разбор делает `scripts/books_index.py`; здесь только чтение, поэтому скрипт
обходится стандартной библиотекой и запускается обычным `python3`.

Что он добавляет к простому `grep`:

* говорит, в каком разделе книги найдено, а не только на какой странице —
  раздел берётся из оглавления по диапазону страниц;
* не спотыкается о переносы и составные слова: «веб-сервис» находится и там,
  где в книге написано «веб-сервис», и там, где слово разорвано переносом;
* показывает страницу файлом, который можно открыть целиком.

Примеры:
    python3 scripts/books_find.py "макет компоновки"
    python3 scripts/books_find.py --book query-language "виртуальн.* таблиц"
    python3 scripts/books_find.py --toc COM
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Iterator

DEFAULT_BOOKS = Path("docs/materials/books")


def load_books(base: Path, only: list[str] | None) -> list[dict[str, Any]]:
    """Прочитать реестр разобранных книг."""
    index_path = base / "index.json"
    if not index_path.is_file():
        sys.exit(
            f"нет {index_path}; сначала разобрать книги:\n"
            f"    {base}/.venv/bin/python scripts/books_index.py"
        )
    index = json.loads(index_path.read_text(encoding="utf-8"))
    books = []
    for entry in index["books"]:
        if only and entry["slug"] not in only:
            continue
        directory = base / entry["slug"]
        toc = json.loads((directory / "toc.json").read_text(encoding="utf-8"))
        books.append({**entry, "directory": directory, "toc": toc["entries"]})
    if only:
        known = {entry["slug"] for entry in index["books"]}
        for slug in only:
            if slug not in known:
                sys.exit(f"нет такой книги: {slug}; есть: {', '.join(sorted(known))}")
    return books


def section_of(toc: list[dict[str, Any]], page: int) -> str:
    """Путь по оглавлению до раздела, в который попадает страница."""
    path: dict[int, str] = {}
    for entry in toc:
        if entry["page"] <= page <= entry["page_end"]:
            path[entry["level"]] = entry["title"]
    return " › ".join(path[level] for level in sorted(path)) or "—"


def build_pattern(query: str, literal: bool) -> re.Pattern[str]:
    """Собрать выражение поиска, терпимое к дефисам и переносам.

    Книга набрана с переносами по слогам, а разбор их склеивает, поэтому
    «веб-сервис» в тексте может оказаться и «вебсервис». Пробел и дефис в
    запросе поэтому соответствуют любому их сочетанию, в том числе пустому.
    """
    if literal:
        parts = [re.escape(part) for part in re.split(r"[\s\-]+", query) if part]
    else:
        parts = [part for part in re.split(r"(?<!\\)[\s\-]+", query) if part]
    return re.compile(r"[\s\-]*".join(parts), re.IGNORECASE | re.MULTILINE)


# Разделы, по которым искать бессмысленно: печатное оглавление повторяет все
# заголовки книги и потому находится на любой запрос по заголовку.
FRONT_MATTER = {"Оглавление", "Обложка", "Выходные данные", "Авторские права"}


def pages(book: dict[str, Any], front_matter: bool) -> Iterator[tuple[int, str]]:
    for path in sorted((book["directory"] / "pages").glob("*.txt")):
        page = int(path.stem)
        if not front_matter and section_of(book["toc"], page) in FRONT_MATTER:
            continue
        yield page, path.read_text(encoding="utf-8")


def report(
    book: dict[str, Any],
    page: int,
    text: str,
    pattern: re.Pattern[str],
    context: int,
) -> str:
    """Собрать вывод по одной найденной странице."""
    lines = text.split("\n")
    hits = [index for index, line in enumerate(lines) if pattern.search(line)]
    shown: list[str] = []
    printed: set[int] = set()
    for hit in hits:
        low, high = max(0, hit - context), min(len(lines), hit + context + 1)
        if printed and low > max(printed) + 1:
            shown.append("    …")
        for index in range(low, high):
            if index in printed:
                continue
            printed.add(index)
            mark = ">" if index == hit else " "
            shown.append(f"  {mark} {lines[index]}")
    header = (
        f"{book['slug']}  стр. {page}  ·  {section_of(book['toc'], page)}\n"
        f"  {book['directory']}/pages/{page:04d}.txt"
    )
    return header + "\n" + "\n".join(shown)


def search_toc(books: list[dict[str, Any]], pattern: re.Pattern[str]) -> int:
    """Искать только по заголовкам оглавления."""
    found = 0
    for book in books:
        for entry in book["toc"]:
            if pattern.search(entry["title"]):
                found += 1
                print(
                    f"{book['slug']}  стр. {entry['page']}-{entry['page_end']}  "
                    f"{'  ' * (entry['level'] - 1)}{entry['title']}"
                )
    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("query", help="искомое; по умолчанию — регулярное выражение")
    parser.add_argument("--books", type=Path, default=DEFAULT_BOOKS)
    parser.add_argument(
        "--book",
        action="append",
        dest="only",
        help="ограничить поиск книгой (можно повторять)",
    )
    parser.add_argument("--toc", action="store_true", help="искать только по оглавлению")
    parser.add_argument(
        "--literal", action="store_true", help="искать запрос буквально, не как выражение"
    )
    parser.add_argument("--context", type=int, default=2, help="строк контекста (по умолчанию 2)")
    parser.add_argument("--limit", type=int, default=20, help="сколько страниц показать")
    parser.add_argument(
        "--front-matter",
        action="store_true",
        help="искать и по печатному оглавлению, обложке и копирайту",
    )
    arguments = parser.parse_args()

    books = load_books(arguments.books, arguments.only)
    pattern = build_pattern(arguments.query, arguments.literal)

    if arguments.toc:
        found = search_toc(books, pattern)
        print(f"\nразделов: {found}")
        return 0

    shown = 0
    total = 0
    for book in books:
        for page, text in pages(book, arguments.front_matter):
            if not pattern.search(text):
                continue
            total += 1
            if shown < arguments.limit:
                shown += 1
                print(report(book, page, text, pattern, arguments.context))
                print()
    print(f"страниц найдено: {total}" + (f", показано: {shown}" if total > shown else ""))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
