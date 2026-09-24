#!/usr/bin/env python3
"""Build 1C type catalogs for ML Studio from the extracted official help."""

from __future__ import annotations

import argparse
import json
import os
import shutil
from pathlib import Path


def write_json(path: Path, value: object) -> None:
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def source(kind: str, location: str, note: str) -> dict[str, str]:
    return {"kind": kind, "location": location, "note": note}


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
        "--managed-form-help",
        type=Path,
        default=Path("docs/materials/its/8.3.27.2342/html/mngdsgn_ru"),
    )
    parser.add_argument(
        "--output", type=Path, default=Path("docs/materials/its/types")
    )
    args = parser.parse_args()

    bridge = json.loads(args.bridge.read_text(encoding="utf-8"))
    platform_version = bridge["platform_version"]
    output = args.output
    bridge_relative = Path(os.path.relpath(args.bridge, output)).as_posix()
    source_dir = output / "source" / "mngdsgn_ru"
    source_dir.mkdir(parents=True, exist_ok=True)

    module_types = sorted(
        (symbol for symbol in bridge["symbols"] if symbol["kind"] == "type"),
        key=lambda symbol: (symbol.get("name_ru") or "").casefold(),
    )
    write_json(
        output / "module-types.json",
        {
            "schema": "metalab.its.module-types",
            "schema_version": 1,
            "platform_version": platform_version,
            "scope": (
                "Типы значений платформы, доступные из BSL. Фактическая доступность "
                "зависит от контекста выполнения и указана в поле availability."
            ),
            "source": source(
                "local-official-help",
                bridge_relative,
                "Нормализованный Синтакс-помощник shcntx_ru.hbk.",
            ),
            "type_count": len(module_types),
            "types": module_types,
        },
    )

    metadata_sources = [
        source(
            "official-its",
            "https://its.1c.ru/db/content/metod8dev/src/developers/platform/metod/other/i8101828.htm",
            "Особенности хранения составных типов данных.",
        ),
        source(
            "official-1c-dn",
            "https://1c-dn.com/library/tutorials/1c_enterprise_developer_guide_8_3_27/",
            "Developer Guide 8.3.27: типы и типообразующие объекты конфигурации.",
        ),
    ]
    write_json(
        output / "metadata-attribute-types.json",
        {
            "schema": "metalab.its.metadata-attribute-types",
            "schema_version": 1,
            "platform_version": platform_version,
            "scope": (
                "Базовая модель типов реквизитов объектов метаданных. Конкретный набор "
                "в редакторе типа зависит от вида объекта метаданных и свойства."
            ),
            "sources": metadata_sources,
            "categories": [
                {
                    "id": "primitive-stored",
                    "name_ru": "Примитивные хранимые типы",
                    "types": [
                        {
                            "name_ru": "Булево",
                            "name_en": "Boolean",
                            "qualifiers": [],
                        },
                        {
                            "name_ru": "Число",
                            "name_en": "Number",
                            "qualifiers": ["Длина", "Точность", "Знак"],
                        },
                        {
                            "name_ru": "Строка",
                            "name_en": "String",
                            "qualifiers": ["Длина", "Допустимая длина"],
                        },
                        {
                            "name_ru": "Дата",
                            "name_en": "Date",
                            "qualifiers": ["Состав даты"],
                        },
                    ],
                },
                {
                    "id": "value-storage",
                    "name_ru": "Хранилище произвольного значения",
                    "types": [
                        {
                            "name_ru": "ХранилищеЗначения",
                            "name_en": "ValueStorage",
                            "note": (
                                "Допустимо для хранения в метаданных, но недоступно "
                                "как данные управляемой формы."
                            ),
                        }
                    ],
                },
                {
                    "id": "configuration-defined",
                    "name_ru": "Типы, создаваемые объектами конфигурации",
                    "patterns": [
                        "СправочникСсылка.<Имя>",
                        "ДокументСсылка.<Имя>",
                        "ПеречислениеСсылка.<Имя>",
                        "ПланВидовХарактеристикСсылка.<Имя>",
                        "ПланСчетовСсылка.<Имя>",
                        "ПланВидовРасчетаСсылка.<Имя>",
                        "БизнесПроцессСсылка.<Имя>",
                        "ЗадачаСсылка.<Имя>",
                        "ПланОбменаСсылка.<Имя>",
                    ],
                    "note": (
                        "Набор формируется метаданными конкретной конфигурации; "
                        "фиксированного общего перечня имен нет."
                    ),
                },
                {
                    "id": "reference-type-sets",
                    "name_ru": "Наборы ссылочных типов",
                    "types": [
                        "ЛюбаяСсылка",
                        "СправочникСсылка",
                        "ДокументСсылка",
                        "ПеречислениеСсылка",
                        "ПланВидовХарактеристикСсылка",
                        "ПланСчетовСсылка",
                        "ПланВидовРасчетаСсылка",
                        "БизнесПроцессСсылка",
                        "БизнесПроцессМаршрутнаяТочкаСсылка",
                        "ЗадачаСсылка",
                        "ПланОбменаСсылка",
                    ],
                },
                {
                    "id": "composite",
                    "name_ru": "Составной тип",
                    "note": (
                        "Объединяет несколько допустимых типов. Физическое хранение и "
                        "ограничения зависят от состава и используемой СУБД."
                    ),
                },
                {
                    "id": "configuration-aliases",
                    "name_ru": "Определяемые типы и типы характеристик",
                    "types": ["ОпределяемыйТип.<Имя>", "Характеристика.<Имя>"],
                    "note": "Состав раскрывается по метаданным конфигурации.",
                },
            ],
        },
    )

    special_names = {
        "ДанныеФормыСтруктура",
        "ДанныеФормыКоллекция",
        "ДанныеФормыСтруктураСКоллекцией",
        "ДанныеФормыДерево",
        "ДинамическийСписок",
    }
    conversion_names = {
        "ЗначениеВДанныеФормы",
        "ДанныеФормыВЗначение",
        "ЗначениеВРеквизитФормы",
        "РеквизитФормыВЗначение",
    }
    special_types = [x for x in module_types if x.get("name_ru") in special_names]
    conversions = [
        x
        for x in bridge["symbols"]
        if x.get("name_ru") in conversion_names and x.get("kind") == "method"
    ]
    form_sources = [
        source(
            "local-official-help",
            "source/mngdsgn_ru/form_lfpropertiesedit.html",
            "Реквизиты формы, извлечено из mngdsgn_ru.hbk.",
        ),
        source(
            "official-its",
            "https://its.1c.ru/db/v8327doc/bookmark/dev/TI000002900",
            "Руководство разработчика 8.3.27, глава 7: Формы.",
        ),
        source(
            "official-1c-dn",
            "https://1c-dn.com/library/tutorials/practical_developer_guide_form_data_types/",
            "Form data types and conversion.",
        ),
    ]
    write_json(
        output / "form-attribute-types.json",
        {
            "schema": "metalab.its.form-attribute-types",
            "schema_version": 1,
            "platform_version": platform_version,
            "scope": "Типы реквизитов управляемой формы и преобразование данных.",
            "sources": form_sources,
            "rules": [
                {
                    "id": "client-compatible-values",
                    "description": (
                        "Типы значений, доступные в клиентском контексте, могут "
                        "использоваться непосредственно в данных формы. Проверять "
                        "поле availability в module-types.json недостаточно: редактор "
                        "формы дополнительно проверяет допустимость типа."
                    ),
                },
                {
                    "id": "application-object-projection",
                    "description": (
                        "Прикладные объекты на сервере автоматически преобразуются "
                        "в специализированные типы данных формы и обратно."
                    ),
                },
                {
                    "id": "unsupported-example",
                    "type_ru": "ХранилищеЗначения",
                    "editor_mark": "Недоступен в данных формы",
                },
            ],
            "special_form_types": special_types,
            "conversion_methods": conversions,
            "designer_specific_types": [
                "ТаблицаЗначений",
                "ДинамическийСписок",
                "ДиаграммаГанта",
                "Диаграмма",
                "Дендрограмма",
                "ТабличныйДокумент",
                "ГрафическаяСхема",
                "ГеографическаяСхема",
            ],
        },
    )

    for name in ("form_lfpropertiesedit", "form_lfpropertycontentedit"):
        src = args.managed_form_help / name
        if not src.exists():
            raise SystemExit(f"Managed form help page not found: {src}")
        shutil.copyfile(src, source_dir / f"{name}.html")

    write_json(
        output / "index.json",
        {
            "schema": "metalab.its.types-index",
            "schema_version": 1,
            "platform_version": platform_version,
            "catalogs": [
                {
                    "file": "module-types.json",
                    "purpose": "BSL-типы и их контексты выполнения",
                    "count": len(module_types),
                },
                {
                    "file": "metadata-attribute-types.json",
                    "purpose": "типы реквизитов метаданных",
                },
                {
                    "file": "form-attribute-types.json",
                    "purpose": "типы реквизитов управляемых форм",
                    "special_type_count": len(special_types),
                    "conversion_method_count": len(conversions),
                },
            ],
            "important_distinction": (
                "Тип BSL-значения, тип реквизита метаданных и тип данных формы — "
                "три связанных, но не одинаковых множества."
            ),
        },
    )


if __name__ == "__main__":
    main()
