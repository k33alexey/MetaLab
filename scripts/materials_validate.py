#!/usr/bin/env python3
"""Validate generated 1C 8.3.27 reference materials without third-party packages."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


EXPECTED_VERSION = "8.3.27.2342"


def strings(value: Any):
    if isinstance(value, dict):
        for nested in value.values():
            yield from strings(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from strings(nested)
    elif isinstance(value, str):
        yield value


def unique(items: list[dict[str, Any]], key: str, label: str, errors: list[str]) -> None:
    values = [item.get(key) for item in items]
    duplicates = sorted({value for value in values if value is not None and values.count(value) > 1})
    if duplicates:
        errors.append(f"{label}: duplicate {key}: {duplicates[:10]}")


def validate(repository: Path) -> list[str]:
    base = repository / "docs/materials/platform-model"
    errors: list[str] = []
    documents: dict[Path, Any] = {}

    if not base.is_dir():
        return [f"missing directory: {base}"]

    for path in sorted(base.rglob("*.json")):
        try:
            documents[path] = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            errors.append(f"{path.relative_to(repository)}: {error}")

    for path, document in documents.items():
        relative = path.relative_to(repository)
        if path.name != "result-schema.json" and not document.get("schema"):
            errors.append(f"{relative}: missing schema")
        version = document.get("platform_version")
        if version is not None and version != EXPECTED_VERSION:
            errors.append(f"{relative}: unexpected platform_version {version!r}")
        for value in strings(document):
            if value.startswith(("docs/", "internal/")) and not (repository / value).exists():
                errors.append(f"{relative}: missing local reference {value}")

    # Разобранная справка: без неё «валидация прошла» означала бы только то,
    # что цела производная модель платформы, а сама справка могла остаться
    # собранной прошлым разбором.
    its = repository / "docs/materials/its"
    checks = [
        (its / "8.3.27.2342/bsl-bridge/bsl-bridge.json", "symbols", 20000),
        (its / "8.3.27.2342/page-index.json", "pages", 40000),
        (its / "query-tables/tables.json", "tables", 50),
        (its / "events/all-events.json", "events", 600),
    ]
    for path, key, least in checks:
        if not path.exists():
            errors.append(f"{path.relative_to(repository)}: файла нет — прогоните scripts/its_rebuild.py")
            continue
        try:
            document = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            errors.append(f"{path.relative_to(repository)}: {error}")
            continue
        items = document.get(key)
        if not isinstance(items, (list, dict)) or len(items) < least:
            errors.append(f"{path.relative_to(repository)}: {key} — {len(items or [])}, ожидалось не меньше {least}")
    events = documents.get(its / "events/all-events.json")
    if events is None and (its / "events/all-events.json").exists():
        events = json.loads((its / "events/all-events.json").read_text(encoding="utf-8"))
    if isinstance(events, dict):
        # Параметры событий однажды уже терялись целиком и молча.
        with_parameters = sum(1 for item in events.get("events", []) if item.get("parameters"))
        if with_parameters < 500:
            errors.append(f"docs/materials/its/events/all-events.json: событий с разобранными параметрами {with_parameters}, ожидалось не меньше 500")

    scenarios = documents.get(base / "06-conformance/scenarios.json", {}).get("scenarios", [])
    unique(scenarios, "id", "conformance scenarios", errors)
    for scenario in scenarios:
        if scenario.get("status") not in {"needs-oracle-run", "verified"}:
            errors.append(f"scenario {scenario.get('id')}: invalid status {scenario.get('status')!r}")

    metadata = documents.get(base / "01-metadata/object-kinds.json", {})
    unique(metadata.get("kinds", []), "kind", "metadata object kinds", errors)

    ui = documents.get(base / "07-ui-reference/index.json", {})
    unique(ui.get("workflows", []), "id", "UI workflows", errors)

    dcs_language = documents.get(base / "04-query-dcs/expression-language.json", {})
    dcs_pages = dcs_language.get("pages", [])
    unique(dcs_pages, "id", "DCS expression pages", errors)
    if dcs_language.get("page_count") != len(dcs_pages):
        errors.append("DCS expression language: page_count mismatch")
    if not str(dcs_language.get("source", "")).endswith("dcsui_ru.json"):
        errors.append("DCS expression language: wrong official help source")

    dcs_model = documents.get(base / "04-query-dcs/schema-model.json", {})
    if dcs_model.get("schema_count") != len(dcs_model.get("schemas", [])):
        errors.append("DCS schema model: schema_count mismatch")
    dcs_scenarios = documents.get(base / "04-query-dcs/conformance-scenarios.json", {}).get("scenarios", [])
    unique(dcs_scenarios, "id", "DCS conformance scenarios", errors)

    index = documents.get(base / "index.json", {})
    for section in index.get("sections", []):
        directory = base / section.get("directory", "")
        if not directory.is_dir() or not (directory / "README.md").is_file():
            errors.append(f"index section is incomplete: {directory.relative_to(repository)}")

    materials_index_path = repository / "docs/materials/index.json"
    try:
        materials_index = json.loads(materials_index_path.read_text(encoding="utf-8"))
        for value in strings(materials_index):
            if value.startswith(("docs/", "scripts/")) and not (repository / value).exists():
                errors.append(f"docs/materials/index.json: missing local reference {value}")
    except (OSError, json.JSONDecodeError) as error:
        errors.append(f"docs/materials/index.json: {error}")

    its_index_path = repository / "docs/materials/its/index.json"
    try:
        its_index = json.loads(its_index_path.read_text(encoding="utf-8"))
        if its_index.get("target_platform_version") != EXPECTED_VERSION:
            errors.append("docs/materials/its/index.json: target platform version mismatch")
        if its_index.get("target_export_format_version") != "2.20":
            errors.append("docs/materials/its/index.json: export format mismatch")
    except (OSError, json.JSONDecodeError) as error:
        errors.append(f"docs/materials/its/index.json: {error}")

    configuration_path = repository / "docs/materials/demo-base/Configuration.xml"
    try:
        import xml.etree.ElementTree as ET

        if ET.parse(configuration_path).getroot().attrib.get("version") != "2.20":
            errors.append("docs/materials/demo-base/Configuration.xml: expected export format 2.20")
    except (OSError, ET.ParseError) as error:
        errors.append(f"docs/materials/demo-base/Configuration.xml: {error}")

    # Разобранные книги: без проверки «валидация прошла» означало бы, что
    # реестр книг цел, хотя страницы мог стереть прерванный разбор.
    books_index_path = repository / "docs/materials/books/index.json"
    if books_index_path.exists():
        try:
            books_index = json.loads(books_index_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            errors.append(f"docs/materials/books/index.json: {error}")
        else:
            unique(books_index.get("books", []), "slug", "books", errors)
            for book in books_index.get("books", []):
                directory = repository / book.get("path", "")
                pages = sorted((directory / "pages").glob("*.txt")) if directory.is_dir() else []
                if len(pages) != book.get("page_count"):
                    errors.append(
                        f"{book.get('path')}: страниц разобрано {len(pages)}, "
                        f"в книге {book.get('page_count')} — прогоните scripts/books_index.py"
                    )
                if not (directory / "toc.json").is_file():
                    errors.append(f"{book.get('path')}: нет toc.json")

    obsolete = repository / "docs/materials/its/8.5.1.1150"
    if obsolete.exists():
        errors.append("obsolete 8.5.1.1150 materials are present")

    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    errors = validate(args.repository.resolve())
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        print(f"Validation failed: {len(errors)} error(s)")
        return 1
    print("Materials validation passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
