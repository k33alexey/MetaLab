#!/usr/bin/env python3
"""Собрать служебный индекс страниц справки из `.st` и `__categories__`.

HTML-страница говорит, что элемент делает. Рядом с ней платформа кладёт два
служебных файла, которые говорят то, чего в тексте нет или что в тексте
написано прозой:

* `<страница>.st` — каноническое короткое имя элемента по-русски и по-английски
  (для методов — с подписью);
* `__categories__` на каталог — в каких контекстах элемент доступен, кодами
  (`CONF_SRV_ENTERPRISE` и прочие), и с какой версии платформы он есть.

Коды важнее прозы: «Тонкий клиент, веб-клиент, сервер» в тексте есть не у всех
страниц, а код доступности — у всех.

Использование:
    python3 scripts/its_page_index.py [--html <каталог>] [--output <файл>]
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from its_braced import BracedError, parse  # noqa: E402

DEFAULT_HTML = Path("docs/materials/its/8.3.27.2342/html")
DEFAULT_OUTPUT = Path("docs/materials/its/8.3.27.2342/page-index.json")
PLATFORM_VERSION = "8.3.27.2342"

# Коды контекстов справки оставлены как есть, без перевода на русские
# названия клиентов.
#
# Перевод напрашивается, но проверку он не прошёл. Сверил коды с русскими
# подписями доступности на тех же 20 508 страницах: для семи кодов слово ни
# разу не встречается без своего кода, и это согласуется с очевидным чтением
# (CONF_SRV_ENTERPRISE — сервер, CONF_WEB_ENTERPRISE — веб-клиент). Но коды
# стоят почти на всех страницах разом, поэтому то же самое верно сразу для
# нескольких кандидатов, и отличить CONF_MOBILE_ENTERPRISE от CONF_MA_CLIENT
# данные не позволяют. Выдавать догадку за перевод — ровно та ошибка, ради
# которой всё это и перепроверялось, так что здесь только коды.

def read_names(path: Path) -> dict[str, str]:
    """Достать русское и английское имя из `.st`."""
    try:
        data = parse(path.read_text(encoding="utf-8-sig", errors="ignore"))
    except (BracedError, OSError):
        return {}
    result: dict[str, str] = {}

    def walk(node) -> None:
        if not isinstance(node, list):
            return
        # {0, {"ru",0,0,"","Код"}} — язык первым, текст последним.
        if len(node) >= 2 and isinstance(node[1], list) and len(node[1]) >= 5:
            language, text = node[1][0], node[1][-1]
            if language in ("ru", "en") and isinstance(text, str) and text:
                result.setdefault(language, text)
        for item in node:
            walk(item)

    walk(data)
    return result


def read_categories(path: Path) -> dict[str, dict]:
    """Достать контексты и версию появления для страниц одного каталога."""
    try:
        data = parse(path.read_text(encoding="utf-8-sig", errors="ignore"))
    except (BracedError, OSError):
        return {}
    result: dict[str, dict] = {}
    index = 1  # нулевой элемент — счётчик записей
    while index + 2 < len(data) + 1:
        if index + 2 > len(data):
            break
        name, count, payload = data[index], data[index + 1], data[index + 2]
        index += 3
        if not isinstance(name, str) or not isinstance(payload, list):
            continue
        codes = [item for item in payload if isinstance(item, str) and item.startswith("CONF_")]
        rest = [item for item in payload if isinstance(item, str) and not item.startswith("CONF_") and item]
        entry: dict = {"contexts": codes}
        if rest:
            entry["since_version"] = rest[0]
        if isinstance(count, int) and count != len(codes):
            entry["declared_context_count"] = count
        result[name] = entry
    return result


def build(html_root: Path) -> dict:
    pages: dict[str, dict] = {}
    unknown_codes: Counter = Counter()
    st_count = category_files = 0

    for book in sorted(p for p in html_root.iterdir() if p.is_dir()):
        for directory, _, files in [(Path(root), dirs, names) for root, dirs, names in __import__("os").walk(book)]:
            names = set(files)
            if "__categories__" in names:
                category_files += 1
                for page, entry in read_categories(directory / "__categories__").items():
                    key = str((directory / page).relative_to(html_root))
                    pages.setdefault(key, {}).update(entry)
                    for code in entry.get("contexts", []):
                        unknown_codes[code] += 1
            for name in files:
                if not name.endswith(".st"):
                    continue
                st_count += 1
                names_found = read_names(directory / name)
                if not names_found:
                    continue
                key = str((directory / (name[:-3] + ".html")).relative_to(html_root))
                entry = pages.setdefault(key, {})
                if names_found.get("ru"):
                    entry["short_ru"] = names_found["ru"]
                if names_found.get("en"):
                    entry["short_en"] = names_found["en"]

    return {
        "schema": "metalab.its-page-index",
        "schema_version": 1,
        "platform_version": PLATFORM_VERSION,
        "source": "docs/materials/its/8.3.27.2342/html",
        "page_count": len(pages),
        "short_name_files": st_count,
        "category_files": category_files,
        "context_codes": dict(unknown_codes),
        "pages": pages,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--html", type=Path, default=DEFAULT_HTML)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()
    if not args.html.is_dir():
        print(f"Нет каталога {args.html}", file=sys.stderr)
        return 2
    index = build(args.html)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(index, ensure_ascii=False, separators=(",", ":")), encoding="utf-8")
    with_context = sum(1 for entry in index["pages"].values() if entry.get("contexts"))
    with_short = sum(1 for entry in index["pages"].values() if entry.get("short_ru"))
    print(
        f"Создан {args.output}: страниц {index['page_count']}, "
        f"с контекстами {with_context}, с коротким именем {with_short}, "
        f"файлов .st {index['short_name_files']}, __categories__ {index['category_files']}"
    )
    print("  коды контекстов:", ", ".join(sorted(index["context_codes"])))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
