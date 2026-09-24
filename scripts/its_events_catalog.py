#!/usr/bin/env python3
"""Build a structured event catalog from the official 1C Syntax Assistant."""

from __future__ import annotations

import argparse
import json
import os
import re
from collections import Counter
from pathlib import Path
from typing import Any


SPECIAL_MODULES = {
    "objects/Global context/events/catalog375/": (
        "managed-application-module",
        "Модуль управляемого приложения",
    ),
    "objects/Global context/events/catalog201/": (
        "ordinary-application-module",
        "Модуль обычного приложения",
    ),
    "objects/Global context/events/catalog318/": (
        "session-module",
        "Модуль сеанса",
    ),
    "objects/Global context/events/catalog200/": (
        "external-connection-module",
        "Модуль внешнего соединения",
    ),
}

FORM_ELEMENT_MARKERS = (
    "поле",
    "поляформы",
    "таблицаформы",
    "таблицыформы",
    "табличноеполе",
    "табличногополя",
    "кнопка",
    "флажок",
    "надпись",
    "переключатель",
    "панель",
    "полосарегулирования",
    "диаграмма",
    "дендрограмма",
    "расширениеэлементовуправления",
    "расширениегруппыформы",
    "расширениедекорацииформы",
)


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def is_heading(symbol: dict[str, Any]) -> bool:
    return not symbol.get("syntaxes") and re.search(
        r"/events/catalog\d+\.html$", symbol.get("source_path", "")
    ) is not None


def special_module(symbol: dict[str, Any]) -> tuple[str, str] | None:
    path = symbol.get("source_path", "")
    for prefix, module in SPECIAL_MODULES.items():
        if path.startswith(prefix):
            return module
    return None


def classify(symbol: dict[str, Any]) -> tuple[str, str]:
    special = special_module(symbol)
    if special:
        return "special-modules", special[1]

    owner = symbol.get("owner_ru") or ""
    compact = owner.casefold().replace(" ", "")
    path = (symbol.get("source_path") or "").casefold()

    if owner.startswith(
        (
            "Расширение поля формы",
            "Расширение табличного поля",
            "Расширение таблицы формы",
            "Расширение группы формы",
            "Расширение декорации формы",
            "Расширение элементов управления",
        )
    ) or any(marker in compact for marker in FORM_ELEMENT_MARKERS):
        return "form-elements", owner or "Элемент формы"
    if (
        owner in {"Форма", "ФормаКлиентскогоПриложения"}
        or owner.startswith("Расширение формы")
        or "form extension" in path
    ):
        return "form-modules", owner or "Расширение формы"
    if "менеджер" in compact:
        return "manager-modules", owner
    if (
        "объект.<" in compact
        or "наборзаписей" in compact
        or owner in {"ВнешнийОтчет", "ВнешняяОбработка"}
    ):
        return "object-modules", owner
    if owner.startswith("Модуль "):
        return "service-modules", owner
    return "other-event-sources", owner or "Не определено"


def compact_event(
    symbol: dict[str, Any], group: str, module_kind: str, source_root: str
) -> dict[str, Any]:
    return {
        "id": symbol.get("id"),
        "group": group,
        "module_kind": module_kind,
        "owner_ru": symbol.get("owner_ru"),
        "owner_en": symbol.get("owner_en"),
        "name_ru": symbol.get("name_ru"),
        "name_en": symbol.get("name_en"),
        "syntaxes": symbol.get("syntaxes", []),
        "parameters": symbol.get("parameters", []),
        "availability": symbol.get("availability", []),
        "description": symbol.get("description", ""),
        "example": symbol.get("example", ""),
        "since_version": symbol.get("since_version"),
        "changed_versions": symbol.get("changed_versions", []),
        "source_path": symbol.get("source_path"),
        "source_html": source_root.rstrip("/") + "/" + symbol.get("source_path", ""),
    }


def catalog_file(
    group: str,
    title: str,
    events: list[dict[str, Any]],
    platform_version: str,
) -> dict[str, Any]:
    owners = Counter(event["module_kind"] for event in events)
    return {
        "schema": "metalab.its.events",
        "schema_version": 1,
        "platform_version": platform_version,
        "group": group,
        "title": title,
        "event_count": len(events),
        "module_kinds": [
            {"name": name, "event_count": count}
            for name, count in sorted(owners.items(), key=lambda item: item[0].casefold())
        ],
        "events": sorted(
            events,
            key=lambda event: (
                event["module_kind"].casefold(),
                (event["name_ru"] or "").casefold(),
                event["id"] or "",
            ),
        ),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--bridge",
        type=Path,
        default=Path(
            "docs/materials/its/8.3.27.2342/bsl-bridge/bsl-bridge.json"
        ),
    )
    parser.add_argument(
        "--output", type=Path, default=Path("docs/materials/its/events")
    )
    args = parser.parse_args()

    bridge = json.loads(args.bridge.read_text(encoding="utf-8"))
    platform_version = bridge["platform_version"]
    version_root = args.bridge.parent.parent
    source_root = Path(
        os.path.relpath(version_root / "html" / "shcntx_ru", args.output)
    ).as_posix()
    raw_events = [
        symbol
        for symbol in bridge["symbols"]
        if symbol.get("kind") == "event" and not is_heading(symbol)
    ]

    grouped: dict[str, list[dict[str, Any]]] = {}
    special: dict[str, list[dict[str, Any]]] = {
        module[0]: [] for module in SPECIAL_MODULES.values()
    }
    for symbol in raw_events:
        group, module_kind = classify(symbol)
        event = compact_event(symbol, group, module_kind, source_root)
        grouped.setdefault(group, []).append(event)
        special_info = special_module(symbol)
        if special_info:
            special[special_info[0]].append(event)

    output = args.output
    titles = {
        "form-modules": "События модулей управляемых форм",
        "form-elements": "События элементов управляемых форм",
        "object-modules": "События модулей объектов и наборов записей",
        "manager-modules": "События модулей менеджеров",
        "service-modules": "События специальных сервисных модулей",
        "other-event-sources": "Остальные источники событий платформы",
    }
    files: list[dict[str, Any]] = []

    for key, title in titles.items():
        events = grouped.get(key, [])
        relative = f"{key}.json"
        write_json(output / relative, catalog_file(key, title, events, platform_version))
        files.append({"file": relative, "title": title, "event_count": len(events)})

    for prefix, (key, title) in SPECIAL_MODULES.items():
        events = special[key]
        relative = f"special-modules/{key}.json"
        write_json(output / relative, catalog_file(key, title, events, platform_version))
        files.append({"file": relative, "title": title, "event_count": len(events)})

    all_events = sorted(raw_events, key=lambda item: item.get("id", ""))
    all_compact = []
    for symbol in all_events:
        group, module_kind = classify(symbol)
        all_compact.append(compact_event(symbol, group, module_kind, source_root))
    write_json(
        output / "all-events.json",
        catalog_file("all", "Все события Синтакс-помощника", all_compact, platform_version),
    )
    files.insert(
        0,
        {
            "file": "all-events.json",
            "title": "Все события Синтакс-помощника",
            "event_count": len(all_compact),
        },
    )

    index = {
        "schema": "metalab.its.events-index",
        "schema_version": 1,
        "platform_version": platform_version,
        "source": Path(os.path.relpath(args.bridge, output)).as_posix(),
        "source_kind": "official-local-syntax-assistant",
        "event_count": len(all_compact),
        "files": files,
        "notes": [
            "Перечень описывает возможности платформы, а не текущую готовность runtime MetaLab.",
            "Одинаковые имена не удаляются: их сигнатуры и назначение зависят от владельца и вида модуля.",
            "События элементов формы выполняются обработчиками модуля формы, но вынесены отдельно для редактора формы.",
        ],
    }
    write_json(output / "index.json", index)

    readme_lines = [
        "# События платформы 1С",
        "",
        f"Источник: локальный официальный Синтакс-помощник платформы {platform_version}.",
        "",
        "Каталог разделяет предопределенные процедуры специальных модулей, события объектных и менеджерских модулей, события форм и их элементов.",
        "",
        "Важно: наличие события здесь не означает, что оно уже реализовано в runtime MetaLab.",
        "",
        "## Файлы",
        "",
    ]
    for item in files:
        readme_lines.append(
            f"- `{item['file']}` — {item['title']} ({item['event_count']})."
        )
    readme_lines.extend(
        [
            "",
            "## Формат",
            "",
            "Каждая запись содержит владельца события, русское и английское имя, сигнатуру, параметры, доступность, описание, версию появления и путь к исходной HTML-странице.",
            "",
        ]
    )
    (output / "README.md").write_text("\n".join(readme_lines), encoding="utf-8")


if __name__ == "__main__":
    main()
