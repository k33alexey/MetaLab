#!/usr/bin/env python3
"""Prepare an ephemeral 1C 8.3.27 DCS oracle configuration from an empty dump."""

from __future__ import annotations

import argparse
import shutil
import xml.etree.ElementTree as ET
from pathlib import Path


MD = "http://v8.1c.ru/8.3/MDClasses"
DCS_SCHEMA = "http://v8.1c.ru/8.1/data-composition-system/schema"
NAMESPACES = {
    "": MD,
    "app": "http://v8.1c.ru/8.2/managed-application/core",
    "cfg": "http://v8.1c.ru/8.1/data/enterprise/current-config",
    "cmi": "http://v8.1c.ru/8.2/managed-application/cmi",
    "ent": "http://v8.1c.ru/8.1/data/enterprise",
    "lf": "http://v8.1c.ru/8.2/managed-application/logform",
    "style": "http://v8.1c.ru/8.1/data/ui/style",
    "sys": "http://v8.1c.ru/8.1/data/ui/fonts/system",
    "v8": "http://v8.1c.ru/8.1/data/core",
    "v8ui": "http://v8.1c.ru/8.1/data/ui",
    "web": "http://v8.1c.ru/8.1/data/ui/colors/web",
    "win": "http://v8.1c.ru/8.1/data/ui/colors/windows",
    "xen": "http://v8.1c.ru/8.3/xcf/enums",
    "xpr": "http://v8.1c.ru/8.3/xcf/predef",
    "xr": "http://v8.1c.ru/8.3/xcf/readable",
    "xs": "http://www.w3.org/2001/XMLSchema",
    "xsi": "http://www.w3.org/2001/XMLSchema-instance",
    "dcscom": "http://v8.1c.ru/8.1/data-composition-system/common",
    "dcscor": "http://v8.1c.ru/8.1/data-composition-system/core",
    "dcsset": "http://v8.1c.ru/8.1/data-composition-system/settings",
}

for prefix, uri in NAMESPACES.items():
    ET.register_namespace(prefix, uri)


def local(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def child(parent: ET.Element, name: str) -> ET.Element:
    return next(node for node in parent if local(node.tag) == name)


def set_property(properties: ET.Element, name: str, value: str) -> None:
    child(properties, name).text = value


def prepare(target: Path, repository: Path) -> None:
    (target / "ConfigDumpInfo.xml").write_text(
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        '<ConfigDumpInfo xmlns="http://v8.1c.ru/8.3/xcf/dumpinfo" '
        'xmlns:xen="http://v8.1c.ru/8.3/xcf/enums" '
        'xmlns:xs="http://www.w3.org/2001/XMLSchema" '
        'xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" '
        'format="Hierarchical" version="2.20"><ConfigVersions/></ConfigDumpInfo>\n',
        encoding="utf-8",
    )
    configuration_path = target / "Configuration.xml"
    tree = ET.parse(configuration_path)
    configuration = child(tree.getroot(), "Configuration")
    children = child(configuration, "ChildObjects")
    if not any(local(node.tag) == "Report" and node.text == "DCSOracle" for node in children):
        ET.SubElement(children, f"{{{MD}}}Report").text = "DCSOracle"
    tree.write(configuration_path, encoding="utf-8", xml_declaration=True)

    source_report = repository / "docs/materials/demo-base/Reports/_ДемоФайлыВспомогательный.xml"
    report_dir = target / "Reports/DCSOracle"
    (report_dir / "Templates/Main/Ext").mkdir(parents=True, exist_ok=True)
    report_path = target / "Reports/DCSOracle.xml"
    shutil.copyfile(source_report, report_path)
    report_tree = ET.parse(report_path)
    report = child(report_tree.getroot(), "Report")
    properties = child(report, "Properties")
    set_property(properties, "Name", "DCSOracle")
    for name in ("DefaultForm", "AuxiliaryForm", "DefaultSettingsForm", "AuxiliarySettingsForm", "DefaultVariantForm", "VariantsStorage", "SettingsStorage"):
        set_property(properties, name, "")
    set_property(properties, "MainDataCompositionSchema", "Report.DCSOracle.Template.Main")
    template_ref = child(child(report, "ChildObjects"), "Template")
    template_ref.text = "Main"
    report_tree.write(report_path, encoding="utf-8", xml_declaration=True)

    source_template = repository / "docs/materials/demo-base/Reports/_ДемоФайлыВспомогательный/Templates/ПустаяСхемаКомпоновкиДанных.xml"
    template_path = report_dir / "Templates/Main.xml"
    shutil.copyfile(source_template, template_path)
    template_tree = ET.parse(template_path)
    template = child(template_tree.getroot(), "Template")
    set_property(child(template, "Properties"), "Name", "Main")
    template_tree.write(template_path, encoding="utf-8", xml_declaration=True)

    source_schema = repository / "docs/materials/demo-base/Reports/МестаИспользованияСсылок/Templates/ОсновнаяСхемаКомпоновкиДанных/Ext/Template.xml"
    schema_path = report_dir / "Templates/Main/Ext/Template.xml"
    shutil.copyfile(source_schema, schema_path)
    schema_tree = ET.parse(schema_path)
    schema_root = schema_tree.getroot()
    schema_root.set("xmlns:xs", "http://www.w3.org/2001/XMLSchema")
    for parent in schema_root.iter():
        for node in list(parent):
            parameter_names = [
                (nested.text or "").strip()
                for nested in node.iter()
                if local(nested.tag) in {"name", "parameter"}
            ]
            if "НаборСсылок" in parameter_names and local(node.tag) in {"parameter", "item"}:
                parent.remove(node)
    ET.register_namespace("", DCS_SCHEMA)
    schema_tree.write(schema_path, encoding="utf-8", xml_declaration=True)

    module = target / "Ext/ManagedApplicationModule.bsl"
    module.parent.mkdir(parents=True, exist_ok=True)
    module.write_text(BSL, encoding="utf-8")


BSL = '''Процедура ЗаписатьРезультат(Запись, Сценарий, Событие, Результат = "")
    Кавычка = Символ(34);
    ЭкранированноеЗначение = СтрЗаменить(Строка(Результат), Кавычка, Символ(92) + Кавычка);
    СтрокаJSON = "{" + Кавычка + "scenario" + Кавычка + ":" + Кавычка + Сценарий + Кавычка
        + "," + Кавычка + "event" + Кавычка + ":" + Кавычка + Событие + Кавычка
        + "," + Кавычка + "result" + Кавычка + ":" + Кавычка + ЭкранированноеЗначение + Кавычка + "}";
    Запись.ЗаписатьСтроку(СтрокаJSON);
КонецПроцедуры

Процедура ПриНачалеРаботыСистемы(Отказ)
    Запись = Новый ЗаписьТекста("/private/tmp/metalab-dcs-oracle-results.jsonl", КодировкаТекста.UTF8);
    Попытка
        ЗаписатьРезультат(Запись, "dataset-object", "start");
        ОбъектОтчета = Отчеты.DCSOracle.Создать();
        Схема = ОбъектОтчета.СхемаКомпоновкиДанных;
        ЗаписатьРезультат(Запись, "dataset-object", "schema-loaded", ТипЗнч(Схема));

        Данные = Новый ТаблицаЗначений;
        Данные.Колонки.Добавить("Ссылка");
        Данные.Колонки.Добавить("Данные");
        Данные.Колонки.Добавить("ПредставлениеДанных");
        Данные.Колонки.Добавить("ТипСсылки");
        Данные.Колонки.Добавить("ВспомогательныеДанные");
        Данные.Колонки.Добавить("ЭтоСлужебныеДанные");

        НоваяСтрока = Данные.Добавить();
        НоваяСтрока.Ссылка = "A";
        НоваяСтрока.Данные = "A-1";
        НоваяСтрока.ПредставлениеДанных = "Первая";
        НоваяСтрока.ВспомогательныеДанные = Ложь;
        НоваяСтрока.ЭтоСлужебныеДанные = Ложь;

        НоваяСтрока = Данные.Добавить();
        НоваяСтрока.Ссылка = "A";
        НоваяСтрока.Данные = "A-служебная";
        НоваяСтрока.ПредставлениеДанных = "Служебная";
        НоваяСтрока.ВспомогательныеДанные = Истина;
        НоваяСтрока.ЭтоСлужебныеДанные = Истина;

        НоваяСтрока = Данные.Добавить();
        НоваяСтрока.Ссылка = "B";
        НоваяСтрока.Данные = "B-1";
        НоваяСтрока.ПредставлениеДанных = "Вторая";
        НоваяСтрока.ВспомогательныеДанные = Ложь;
        НоваяСтрока.ЭтоСлужебныеДанные = Ложь;
        ЗаписатьРезультат(Запись, "dataset-object", "input-rows", Данные.Количество());

        Настройки = Схема.ВариантыНастроек[0].Настройки;
        Компоновщик = Новый КомпоновщикМакетаКомпоновкиДанных;
        ТипГенератора = Тип("ГенераторМакетаКомпоновкиДанныхДляКоллекцииЗначений");
        Макет = Компоновщик.Выполнить(Схема, Настройки, Неопределено, Неопределено, ТипГенератора);
        ЗаписатьРезультат(Запись, "dataset-object", "layout-created", ТипЗнч(Макет));

        ВнешниеДанные = Новый Структура;
        ВнешниеДанные.Вставить("МестаИспользования", Данные);
        Процессор = Новый ПроцессорКомпоновкиДанных;
        Процессор.Инициализировать(Макет, ВнешниеДанные);

        Результат = Новый ТаблицаЗначений;
        Вывод = Новый ПроцессорВыводаРезультатаКомпоновкиДанныхВКоллекциюЗначений;
        Вывод.УстановитьОбъект(Результат);
        Вывод.Вывести(Процессор);
        ЗаписатьРезультат(Запись, "dataset-object", "output-rows", Результат.Количество());
        ЗаписатьРезультат(Запись, "dataset-object", "success");
    Исключение
        ЗаписатьРезультат(Запись, "dataset-object", "error", ОписаниеОшибки());
    КонецПопытки;
    Запись.Закрыть();
    ЗавершитьРаботуСистемы(Ложь);
КонецПроцедуры
'''


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("target", type=Path)
    parser.add_argument("--repository", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--snapshot", type=Path, help="Copy the prepared XML configuration to this directory")
    args = parser.parse_args()
    prepare(args.target.resolve(), args.repository.resolve())
    if args.snapshot:
        shutil.copytree(args.target.resolve(), args.snapshot.resolve(), dirs_exist_ok=True)


if __name__ == "__main__":
    main()
