#!/usr/bin/env python3
"""Разобрать книги по платформе-прототипу в грепаемый постраничный текст.

Книги лежат в `docs/materials/books/` рядом со справкой и выгрузкой, под тем
же правилом: ориентир по составу и поведению, не источник текста. Каталог
целиком в `.gitignore`, в репозиторий не попадает ни PDF, ни разбор.

Зачем разбор вообще нужен. По PDF нельзя искать, а читать книгу целиком, чтобы
найти один разворот про макет компоновки, дороже, чем она стоит. Разбор даёт
три вещи: текст постранично (`pages/NNNN.txt`), оглавление с диапазонами
страниц (`toc.json`) и описание книги (`book.json`). Дальше ищет
`scripts/books_find.py`, а прочитать найденное можно как обычный текстовый
файл.

Что делается с текстом, кроме извлечения:

* снимаются колонтитулы — в этих книгах на каждой странице стоят номер и
  название книги или главы, и без их снятия поиск по «язык запросов» находит
  все 370 страниц сразу;
* склеиваются переносы: «информаци-\\nонной» → «информационной»;
* выбрасываются маркеры списков, которые извлекаются отдельными строками.

Требуется PyMuPDF; ставится в локальное окружение рядом с книгами:

    python3 -m venv docs/materials/books/.venv
    docs/materials/books/.venv/bin/pip install pymupdf

Использование:
    docs/materials/books/.venv/bin/python scripts/books_index.py [--books <каталог>]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from collections import Counter
from pathlib import Path
from typing import Any

# PyMuPDF нужен только для самого разбора. Обновление описания по готовому
# разбору обходится без него, и обычный `python3` должен доводить такой
# прогон до конца, а не падать на импорте.
try:
    import pymupdf
except ImportError:  # pragma: no cover - разбирать будет нечем, обновлять - есть чем
    pymupdf = None

NO_PYMUPDF = (
    "нужен PyMuPDF; запускать разбор так:\n"
    "    docs/materials/books/.venv/bin/python scripts/books_index.py\n"
    "окружение создаётся один раз:\n"
    "    python3 -m venv docs/materials/books/.venv\n"
    "    docs/materials/books/.venv/bin/pip install pymupdf"
)

DEFAULT_BOOKS = Path("docs/materials/books")
SCHEMA = "metalab.materials-books"
SCHEMA_VERSION = 1

# Реестр ведётся руками: выходные данные внутри PDF либо пусты, либо испорчены
# кодировкой шрифта на обложке, поэтому доверять им нельзя. `sha256` стоит
# здесь, чтобы подмена файла под тем же именем была видна, а не молча попала
# в разбор под чужим названием.
BOOKS: list[dict[str, Any]] = [
    {
        "slug": "query-language",
        "file": "Запросы.pdf",
        "sha256": "b302716b1a892f34b1fd45b4037e5ef40143bb106927b783e33e5bd32b225885",
        "title": "Язык запросов «1С:Предприятия 8»",
        "author": "Хрусталева Е. Ю.",
        "isbn": "978-5-9677-3041-2",
        "publisher": "1С-Паблишинг",
        "blocks": [6],
        "note": "механизм запросов, исходные и виртуальные таблицы, соединения, итоги",
    },
    {
        "slug": "data-composition",
        "file": "СКД.pdf",
        "sha256": "1367f06ee737a5458d38f86679a100327d5d205aec1634eaede4a3a8119dbd99",
        "title": "Разработка сложных отчетов в «1С:Предприятии 8». Система компоновки данных",
        "author": "Хрусталева Е. Ю.",
        "edition": "издание 2",
        "isbn": "978-5-9677-2509-8",
        "publisher": "1С-Паблишинг",
        "blocks": [12, 13],
        "note": "устройство компоновки: схема, наборы данных, настройки, макет компоновки, процессоры",
    },
    {
        "slug": "integration",
        "file": "Технологии Интеграции.pdf",
        "sha256": "28eb8a94c6414ea6378ba87b501fcd5311b0e2c980e262a23d37b25e5f5d2b46",
        "title": "Технологии интеграции «1С:Предприятия 8.3»",
        "author": "Хрусталева Е. Ю.",
        "isbn": "978-5-9677-2964-5",
        "publisher": "1С-Паблишинг",
        "blocks": [16, 17],
        "note": "интернет-технологии, обмен данными, внешние компоненты, COM",
        "duplicates": [
            {
                "file": "tehnologii-integracii-1spredpriyatiya-83-2-izd.pdf",
                "sha256": "3cbf4a2f2189527dd4a7aa8fd1b9ae2cf3ea916beae6c88a80f7453c4722e561",
                "note": (
                    "2-е стереотипное издание той же книги: страницы 3-502 совпадают "
                    "с основным файлом посимвольно, различаются только обложка, "
                    "выходные данные и копирайт, причём в них сломана кодировка "
                    "шрифта. Не разбирается."
                ),
            }
        ],
    },
]

SOFT_HYPHEN = "­"
MARKER_CHARS = set("■□▪▫•·‣◦")
LETTER = r"[^\W\d_]"


def digest(path: Path) -> str:
    """Контрольная сумма файла книги."""
    sha = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            sha.update(chunk)
    return sha.hexdigest()


def normalize_running(line: str) -> str:
    """Свести строку к виду, в котором колонтитулы разных страниц совпадают.

    Пробелы сводятся к обычным: в колонтитулах этих книг стоят тонкие пробелы
    (U+2009), а в оглавлении того же заголовка — обычные, и без приведения
    заголовок главы со своим колонтитулом не совпадёт.
    """
    return " ".join(re.sub(r"\d+", "#", line).split())


def find_running(pages: list[str], threshold: float) -> set[str]:
    """Найти колонтитулы: строки, повторяющиеся в начале и конце многих страниц.

    Берутся две первые и две последние непустые строки каждой страницы —
    в этих книгах колонтитул занимает одну строку, вторая взята с запасом на
    страницы, где над ним стоит номер отдельной строкой.
    """
    counter: Counter[str] = Counter()
    for text in pages:
        lines = [line for line in text.split("\n") if line.strip()]
        for line in lines[:2] + lines[-2:]:
            counter[normalize_running(line)] += 1
    limit = max(3, int(len(pages) * threshold))
    return {line for line, count in counter.items() if count >= limit and line}


def strip_running(text: str, running: set[str]) -> str:
    """Снять колонтитулы, но только с краёв страницы.

    Ограничение по позиции существенно: название главы встречается и в тексте,
    и вычёркивать его везде значило бы терять содержательные попадания.
    """
    lines = text.split("\n")
    head = 0
    while head < len(lines) and (
        not lines[head].strip() or normalize_running(lines[head]) in running
    ):
        head += 1
    tail = len(lines)
    while tail > head and (
        not lines[tail - 1].strip() or normalize_running(lines[tail - 1]) in running
    ):
        tail -= 1
    return "\n".join(lines[head:tail])


def chapter_running(toc: list[dict[str, Any]]) -> dict[str, int]:
    """Названия глав как колонтитулы: заголовок → страница, на которой глава начинается.

    В этих книгах чётные страницы несут название книги, нечётные — название
    текущей главы. По частоте второе не ловится: глава занимает заметно меньше
    четверти книги. Зато оно в точности совпадает с записью оглавления, и это
    надёжнее любого порога. Страница начала главы исключается, иначе разбор
    потерял бы единственное место, где заголовок стоит по делу.
    """
    return {
        normalize_running(entry["title"]): entry["page"]
        for entry in toc
        if entry["level"] <= 2
    }


def clean(text: str) -> str:
    """Привести текст страницы к виду, по которому осмысленно искать."""
    text = text.replace(SOFT_HYPHEN, "").replace("\xa0", " ")
    lines: list[str] = []
    for line in text.split("\n"):
        stripped = line.strip()
        # Маркеры списков извлекаются отдельными строками и иногда дублируются.
        if stripped and all(char in MARKER_CHARS for char in stripped):
            continue
        # Осиротевшая пунктуация после снятого маркера прирастает к предыдущей строке.
        if stripped in {",", ".", ";", ":"} and lines:
            lines[-1] = lines[-1].rstrip() + stripped
            continue
        # Пункт списка, у которого сняли маркер, прирастает к своему термину:
        # «Запрос» и «- содержит запрос к базе данных» — одна строка списка.
        if re.match(r"^[-\u2010-\u2015]\s", stripped) and lines:
            previous = next((index for index in range(len(lines) - 1, -1, -1) if lines[index].strip()), None)
            if previous is not None and not lines[previous].rstrip().endswith((".", ";", ":")):
                lines[previous] = lines[previous].rstrip() + " " + stripped
                del lines[previous + 1 :]
                continue
        lines.append(line.rstrip())
    text = "\n".join(lines)
    # Перенос по слогам: дефис в конце строки перед строчной буквой.
    text = re.sub(rf"({LETTER})-\n({LETTER})", r"\1\2", text)
    text = re.sub(r"[ \t]+", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip() + "\n"


def build_toc(document: "pymupdf.Document") -> list[dict[str, Any]]:
    """Оглавление с диапазонами страниц.

    Конец раздела — страница перед началом следующего раздела того же или
    более высокого уровня; у последнего это последняя страница книги. Записи
    без страницы (в некоторых книгах так помечены обложка и копирайт)
    отбрасываются.
    """
    entries = [
        {"level": level, "title": " ".join(title.split()), "page": page}
        for level, title, page in document.get_toc()
        if page >= 1
    ]
    for index, entry in enumerate(entries):
        end = document.page_count
        for following in entries[index + 1 :]:
            if following["level"] <= entry["level"]:
                end = max(entry["page"], following["page"] - 1)
                break
        entry["page_end"] = end
    return entries


def refresh(book: dict[str, Any], books_dir: Path) -> dict[str, Any]:
    """Обновить описание книги по готовому разбору, без самого PDF.

    Разбор — самостоятельный материал: искать по нему можно и тогда, когда
    PDF рядом уже нет, а книги как раз держат не все и не всегда. Поэтому
    отсутствие исходника не ошибка, пока страницы на месте: переписывается
    только описание, страницы и оглавление остаются как были.
    """
    target = books_dir / book["slug"]
    pages_dir = target / "pages"
    pages = sorted(pages_dir.glob("*.txt"))
    if not pages:
        raise FileNotFoundError(
            f"нет ни файла книги {books_dir / book['file']}, ни её разбора в {pages_dir}"
        )
    previous = json.loads((target / "book.json").read_text(encoding="utf-8"))
    record = {key: value for key, value in book.items() if key != "sha256"}
    record.update(
        {
            key: previous[key]
            for key in ("schema", "schema_version", "source", "sha256", "page_count",
                        "toc_entries", "pages_without_text", "characters", "running_titles")
            if key in previous
        }
    )
    record["source_present"] = False
    (target / "book.json").write_text(
        json.dumps(record, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return record


def process(book: dict[str, Any], books_dir: Path, threshold: float) -> dict[str, Any]:
    """Разобрать одну книгу и записать её разбор рядом с PDF."""
    source = books_dir / book["file"]
    if not source.is_file():
        return refresh(book, books_dir)
    if pymupdf is None:
        sys.exit(NO_PYMUPDF)
    actual = digest(source)
    if actual != book["sha256"]:
        raise ValueError(
            f"{book['file']}: контрольная сумма не совпала с реестром\n"
            f"  в реестре: {book['sha256']}\n"
            f"  у файла:   {actual}"
        )

    target = books_dir / book["slug"]
    pages_dir = target / "pages"
    for stale in sorted(pages_dir.glob("*.txt")):
        stale.unlink()
    pages_dir.mkdir(parents=True, exist_ok=True)

    with pymupdf.open(source) as document:
        raw = [page.get_text() for page in document]
        toc = build_toc(document)
        page_count = document.page_count

    running = find_running(raw, threshold)
    chapters = chapter_running(toc)
    empty = 0
    characters = 0
    for number, text in enumerate(raw, start=1):
        on_page = running | {
            title for title, start in chapters.items() if start != number
        }
        page = clean(strip_running(text, on_page))
        if len(page.strip()) < 40:
            empty += 1
        characters += len(page)
        (pages_dir / f"{number:04d}.txt").write_text(page, encoding="utf-8")

    (target / "toc.json").write_text(
        json.dumps(
            {
                "schema": f"{SCHEMA}.toc",
                "schema_version": SCHEMA_VERSION,
                "slug": book["slug"],
                "entries": toc,
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    record = {key: value for key, value in book.items() if key != "sha256"}
    record.update(
        {
            "schema": f"{SCHEMA}.book",
            "schema_version": SCHEMA_VERSION,
            "source": f"docs/materials/books/{book['file']}",
            "sha256": actual,
            "page_count": page_count,
            "toc_entries": len(toc),
            "pages_without_text": empty,
            "characters": characters,
            "running_titles": sorted(running),
        }
    )
    (target / "book.json").write_text(
        json.dumps(record, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return record


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--books", type=Path, default=DEFAULT_BOOKS)
    parser.add_argument(
        "--running-threshold",
        type=float,
        default=0.25,
        help="доля страниц, с которой строка считается колонтитулом",
    )
    arguments = parser.parse_args()

    records = []
    for book in BOOKS:
        record = process(book, arguments.books, arguments.running_threshold)
        records.append(record)
        note = "" if record.get("source_present", True) else "  (PDF нет, обновлено описание)"
        print(
            f"{record['slug']:<18} {record['page_count']:>4} стр.  "
            f"{record['toc_entries']:>3} разделов  "
            f"{record['characters'] // 1000:>4}k символов  "
            f"без текста: {record['pages_without_text']}{note}"
        )

    index = {
        "schema": SCHEMA,
        "schema_version": SCHEMA_VERSION,
        "role": "secondary-methodology",
        "note": (
            "Сторонние книги по платформе-прототипу. Ориентир по устройству "
            "механизмов и по составу возможностей, не источник текста и кода. "
            "При расхождении с наблюдаемым поведением 8.3.27 и официальной "
            "справкой правы они: книги описывают более ранние версии."
        ),
        "search": "scripts/books_find.py",
        "generator": "scripts/books_index.py",
        "books": [
            {
                "slug": record["slug"],
                "title": record["title"],
                "author": record["author"],
                "page_count": record["page_count"],
                "blocks": record["blocks"],
                "path": f"docs/materials/books/{record['slug']}",
            }
            for record in records
        ],
    }
    (arguments.books / "index.json").write_text(
        json.dumps(index, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
