#!/usr/bin/env python3
"""Пересобрать справку платформы целиком: из `source` во все каталоги.

Цепочка длинная, и собирать её по частям опасно: модель, собранная старым
разбором, внешне неотличима от свежей, а отвечает иначе. Поэтому одна команда
проходит все шаги в правильном порядке и печатает, что получилось.

    source/*.hbk  →  html/         извлечение контейнеров
    html/         →  json/         страницы в машинный вид
    html/         →  page-index    коды доступности и короткие имена
    json/         →  bsl-bridge    модель символов
    bsl-bridge    →  events/, types/, query-tables/

Шаг извлечения самый долгий и повторяется только по требованию: контейнеры не
меняются, пока их не переложили заново. `--force-extract` заставляет.

Использование:
    python3 scripts/its_rebuild.py [--force-extract] [--version 8.3.27.2342]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SCRIPTS = ROOT / "scripts"


def run(title: str, command: list[str]) -> bool:
    started = time.monotonic()
    print(f"\n=== {title}")
    result = subprocess.run(command, cwd=ROOT)
    spent = time.monotonic() - started
    if result.returncode != 0:
        print(f"    ОШИБКА, код {result.returncode}, {spent:.1f} с")
        return False
    print(f"    готово за {spent:.1f} с")
    return True


def extract_books(source: Path, html: Path, manifests: Path, force: bool) -> bool:
    books = sorted(source.glob("*_ru.hbk"))
    if not books:
        print(f"В {source} нет русских книг справки", file=sys.stderr)
        return False
    print(f"\n=== извлечение: книг {len(books)}")
    for book in books:
        name = book.stem
        target = html / name
        manifest = manifests / f"{name}.json"
        digest = hashlib.sha256(book.read_bytes()).hexdigest()
        if not force and manifest.exists() and target.is_dir():
            stored = json.loads(manifest.read_text(encoding="utf-8")).get("source_sha256")
            if stored == digest:
                continue
            print(f"    {name}: контейнер изменился, извлекаю заново")
        command = [
            sys.executable,
            str(SCRIPTS / "its_hbk_extract.py"),
            str(book),
            str(target),
            "--manifest",
            str(manifest),
        ]
        if not run(f"извлечение {name}", command):
            return False
    print("    все книги на месте")
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--version", default="8.3.27.2342")
    parser.add_argument("--force-extract", action="store_true")
    args = parser.parse_args()

    base = ROOT / "docs/materials/its"
    version = base / args.version
    source = base / "source" / args.version
    html, json_dir = version / "html", version / "json"
    manifests, bridge = version / "manifests", version / "bsl-bridge/bsl-bridge.json"
    page_index = version / "page-index.json"

    if not source.is_dir():
        print(f"Нет исходных контейнеров: {source}", file=sys.stderr)
        return 2
    manifests.mkdir(parents=True, exist_ok=True)
    html.mkdir(parents=True, exist_ok=True)
    json_dir.mkdir(parents=True, exist_ok=True)

    if not extract_books(source, html, manifests, args.force_extract):
        return 1

    steps: list[tuple[str, list[str]]] = []
    for book in sorted(p.name for p in html.iterdir() if p.is_dir()):
        steps.append((
            f"страницы {book}",
            [sys.executable, str(SCRIPTS / "its_hbk_to_json.py"), str(html / book),
             str(json_dir / f"{book}.json"), "--platform-version", args.version,
             "--source", f"docs/materials/its/source/{args.version}/{book}.hbk"],
        ))
    steps += [
        ("служебный индекс страниц",
         [sys.executable, str(SCRIPTS / "its_page_index.py"), "--html", str(html), "--output", str(page_index)]),
        ("модель символов",
         [sys.executable, str(SCRIPTS / "its_bsl_bridge_model.py"), str(json_dir), str(bridge),
          "--page-index", str(page_index)]),
        ("каталог событий", [sys.executable, str(SCRIPTS / "its_events_catalog.py")]),
        ("каталог типов", [sys.executable, str(SCRIPTS / "its_types_catalog.py")]),
        ("таблицы языка запросов", [sys.executable, str(SCRIPTS / "its_query_tables.py")]),
        ("проверка материалов", [sys.executable, str(SCRIPTS / "materials_validate.py")]),
    ]
    for title, command in steps:
        if not run(title, command):
            return 1

    model = json.loads(bridge.read_text(encoding="utf-8"))
    print(f"\nГотово. Символов в модели: {model['symbol_count']}, версия {model['platform_version']}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
