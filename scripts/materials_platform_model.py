#!/usr/bin/env python3
"""Build structured 1C 8.3.27 research catalogs for MetaLab."""

from __future__ import annotations

import json
import re
import xml.etree.ElementTree as ET
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any, Iterable


ROOT = Path("docs/materials")
DEMO = ROOT / "demo-base"
ITS = ROOT / "its"
VERSION = "8.3.27.2342"
OUTPUT = ROOT / "platform-model"
XSI_TYPE = "{http://www.w3.org/2001/XMLSchema-instance}type"


def local(value: str) -> str:
    return value.rsplit("}", 1)[-1]


def rel(path: Path, base: Path = OUTPUT) -> str:
    """Return a repository-relative path; base is accepted for call-site clarity."""
    del base
    return path.resolve().relative_to(Path.cwd().resolve()).as_posix()


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def sample_add(values: list[str], value: str, limit: int = 12) -> None:
    if value and value not in values and len(values) < limit:
        values.append(value)


def descriptor_files() -> list[Path]:
    result = [DEMO / "Configuration.xml"]
    for directory in sorted(path for path in DEMO.iterdir() if path.is_dir()):
        result.extend(sorted(directory.glob("*.xml")))
    return [path for path in result if path.exists()]


def metadata_catalog() -> None:
    kinds: dict[str, dict[str, Any]] = {}
    all_properties: dict[str, dict[str, Any]] = {}
    for path in descriptor_files():
        tree = ET.parse(path)
        root = tree.getroot()
        version = root.attrib.get("version")
        object_node = next(iter(root), root)
        kind = local(object_node.tag)
        info = kinds.setdefault(
            kind,
            {
                "kind": kind,
                "object_count": 0,
                "format_versions": Counter(),
                "properties": {},
                "generated_type_patterns": Counter(),
                "module_files": Counter(),
                "form_count": 0,
                "samples": [],
            },
        )
        info["object_count"] += 1
        info["format_versions"][version or "none"] += 1
        sample_add(info["samples"], rel(path))

        object_dir = path.with_suffix("")
        if object_dir.is_dir():
            for module in object_dir.glob("Ext/*.bsl"):
                info["module_files"][module.name] += 1
            info["form_count"] += len(list(object_dir.glob("Forms/*.xml")))

        for generated in object_node.iter():
            if local(generated.tag) == "GeneratedType":
                name = generated.attrib.get("name", "")
                pattern = re.sub(r"\.[^.]+$", ".<Имя>", name)
                info["generated_type_patterns"][pattern] += 1

        properties = next(
            (child for child in object_node if local(child.tag) == "Properties"), None
        )
        if properties is None:
            continue
        for prop in properties:
            name = local(prop.tag)
            prop_info = info["properties"].setdefault(
                name,
                {
                    "occurrences": 0,
                    "empty_count": 0,
                    "values": [],
                    "xsi_types": [],
                    "child_elements": Counter(),
                },
            )
            prop_info["occurrences"] += 1
            text = (prop.text or "").strip()
            if not text and len(prop) == 0:
                prop_info["empty_count"] += 1
            sample_add(prop_info["values"], text)
            xsi_type = prop.attrib.get(XSI_TYPE)
            if xsi_type:
                sample_add(prop_info["xsi_types"], xsi_type)
            for child in prop:
                prop_info["child_elements"][local(child.tag)] += 1

            aggregate = all_properties.setdefault(
                name,
                {"object_kinds": Counter(), "values": [], "xsi_types": []},
            )
            aggregate["object_kinds"][kind] += 1
            sample_add(aggregate["values"], text)
            if xsi_type:
                sample_add(aggregate["xsi_types"], xsi_type)

    def normalize_counter(value: Counter[str]) -> dict[str, int]:
        return dict(sorted(value.items()))

    normalized_kinds = []
    for kind in sorted(kinds):
        info = kinds[kind]
        info["format_versions"] = normalize_counter(info["format_versions"])
        info["generated_type_patterns"] = normalize_counter(info["generated_type_patterns"])
        info["module_files"] = normalize_counter(info["module_files"])
        info["properties"] = dict(sorted(info["properties"].items()))
        for prop in info["properties"].values():
            prop["child_elements"] = normalize_counter(prop["child_elements"])
        normalized_kinds.append(info)

    write_json(
        OUTPUT / "01-metadata/object-kinds.json",
        {
            "schema": "metalab.reference.metadata-object-kinds",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "source": rel(DEMO),
            "important_limit": "Каталог описывает встреченные в demo-base варианты, а не XSD всех допустимых конфигураций.",
            "kind_count": len(normalized_kinds),
            "descriptor_count": sum(item["object_count"] for item in normalized_kinds),
            "kinds": normalized_kinds,
        },
    )
    write_json(
        OUTPUT / "01-metadata/property-index.json",
        {
            "schema": "metalab.reference.metadata-properties",
            "schema_version": 1,
            "platform_version": VERSION,
            "properties": [
                {
                    "name": name,
                    "object_kinds": dict(sorted(info["object_kinds"].items())),
                    "values": info["values"],
                    "xsi_types": info["xsi_types"],
                }
                for name, info in sorted(all_properties.items())
            ],
        },
    )


def security_jobs_catalog() -> None:
    roles = []
    right_names: Counter[str] = Counter()
    protected_kinds: Counter[str] = Counter()
    restricted_right_count = 0
    for path in sorted(DEMO.glob("Roles/*/Ext/Rights.xml")):
        root = ET.parse(path).getroot()
        role_rights: Counter[str] = Counter()
        object_count = 0
        restriction_count = 0
        for object_node in (node for node in root if local(node.tag) == "object"):
            object_count += 1
            object_name = next((node.text or "" for node in object_node if local(node.tag) == "name"), "")
            protected_kinds[object_name.split(".", 1)[0] if "." in object_name else object_name] += 1
            for right in (node for node in object_node if local(node.tag) == "right"):
                right_name = next((node.text or "" for node in right if local(node.tag) == "name"), "")
                if right_name:
                    right_names[right_name] += 1
                    role_rights[right_name] += 1
                if any(local(node.tag) == "restrictionByCondition" for node in right):
                    restriction_count += 1
                    restricted_right_count += 1
        roles.append(
            {
                "role": path.parents[1].name,
                "path": rel(path),
                "object_count": object_count,
                "restriction_count": restriction_count,
                "rights": dict(sorted(role_rights.items())),
            }
        )

    schedules = []
    schedule_attributes: dict[str, set[str]] = defaultdict(set)
    for path in sorted(DEMO.glob("ScheduledJobs/*/Ext/Schedule.xml")):
        root = ET.parse(path).getroot()
        schedule = next((node for node in root if local(node.tag) == "Schedule"), None)
        if schedule is None:
            continue
        attributes = dict(sorted(schedule.attrib.items()))
        for name, value in attributes.items():
            schedule_attributes[name].add(value)
        schedules.append(
            {
                "job": path.parents[1].name,
                "path": rel(path),
                "attributes": attributes,
                "values": {local(node.tag): (node.text or "").strip() for node in schedule},
            }
        )

    write_json(
        OUTPUT / "01-metadata/security-and-jobs.json",
        {
            "schema": "metalab.reference.security-and-jobs",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "security": {
                "role_file_count": len(roles),
                "roles": roles,
                "right_names": dict(sorted(right_names.items())),
                "protected_object_kinds": dict(sorted(protected_kinds.items())),
                "restricted_right_count": restricted_right_count,
                "restriction_text_policy": "Kept in source Rights.xml; not duplicated in this index.",
            },
            "scheduled_jobs": {
                "schedule_file_count": len(schedules),
                "attribute_values": {name: sorted(values) for name, values in sorted(schedule_attributes.items())},
                "schedules": schedules,
            },
        },
    )


def child_objects_catalog() -> None:
    relations: dict[tuple[str, str, str], dict[str, Any]] = {}

    def visit(container: ET.Element, root_kind: str, owner_kind: str, source: Path) -> None:
        for child in container:
            child_kind = local(child.tag)
            key = (root_kind, owner_kind, child_kind)
            info = relations.setdefault(
                key,
                {
                    "metadata_kind": root_kind,
                    "owner_kind": owner_kind,
                    "child_kind": child_kind,
                    "count": 0,
                    "properties": Counter(),
                    "type_references": Counter(),
                    "samples": [],
                },
            )
            info["count"] += 1
            sample_add(info["samples"], rel(source), 6)
            properties = next((node for node in child if local(node.tag) == "Properties"), None)
            if properties is not None:
                for prop in properties:
                    info["properties"][local(prop.tag)] += 1
                    if local(prop.tag) == "Type":
                        for type_node in prop.iter():
                            if local(type_node.tag) == "Type" and (type_node.text or "").strip():
                                info["type_references"][(type_node.text or "").strip()] += 1
            nested = next((node for node in child if local(node.tag) == "ChildObjects"), None)
            if nested is not None:
                visit(nested, root_kind, child_kind, source)

    for path in descriptor_files():
        root = ET.parse(path).getroot()
        object_node = next(iter(root), root)
        root_kind = local(object_node.tag)
        container = next((node for node in object_node if local(node.tag) == "ChildObjects"), None)
        if container is not None:
            visit(container, root_kind, root_kind, path)

    output = []
    for key in sorted(relations):
        info = relations[key]
        info["properties"] = dict(sorted(info["properties"].items()))
        info["type_references"] = dict(sorted(info["type_references"].items()))
        output.append(info)
    write_json(
        OUTPUT / "01-metadata/child-objects.json",
        {
            "schema": "metalab.reference.metadata-child-objects",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "source": rel(DEMO),
            "important_limit": "Observed child-object relations and types from demo-base, not a complete platform XSD.",
            "relation_count": len(output),
            "relations": output,
        },
    )


def form_files() -> list[Path]:
    return sorted(DEMO.glob("**/Forms/*/Ext/Form.xml"))


def walk_form_items(container: ET.Element) -> Iterable[ET.Element]:
    for child in container:
        tag = local(child.tag)
        if tag in {"ChildItems", "ContextMenu", "ExtendedTooltip"}:
            yield from walk_form_items(child)
            continue
        if "name" in child.attrib or "id" in child.attrib:
            yield child
        for nested in child:
            if local(nested.tag) == "ChildItems":
                yield from walk_form_items(nested)


def managed_forms_catalog() -> None:
    element_types: dict[str, dict[str, Any]] = {}
    form_events: dict[str, dict[str, Any]] = {}
    element_events: dict[str, dict[str, Any]] = {}
    form_properties = Counter()
    attribute_types = Counter()
    command_properties = Counter()
    parsed = 0

    def event_add(target: dict[str, dict[str, Any]], node: ET.Element, path: Path, owner: str) -> None:
        name = node.attrib.get("name", "")
        if not name:
            return
        info = target.setdefault(name, {"count": 0, "handlers": [], "owners": Counter(), "samples": []})
        info["count"] += 1
        info["owners"][owner] += 1
        sample_add(info["handlers"], (node.text or "").strip(), 20)
        sample_add(info["samples"], rel(path), 5)

    for path in form_files():
        root = ET.parse(path).getroot()
        parsed += 1
        for child in root:
            tag = local(child.tag)
            if tag not in {"ChildItems", "Attributes", "Commands", "Parameters", "Events", "CommandInterface"}:
                form_properties[tag] += 1
            if tag == "Events":
                for event in child:
                    event_add(form_events, event, path, "Форма")
            elif tag == "Attributes":
                for attribute in child:
                    for nested in attribute.iter():
                        if local(nested.tag) in {"Type", "TypeSet"} and nested.text and nested.text.strip():
                            attribute_types[nested.text.strip()] += 1
            elif tag == "Commands":
                for command in child:
                    for prop in command:
                        command_properties[local(prop.tag)] += 1
            elif tag == "ChildItems":
                for item in walk_form_items(child):
                    item_type = local(item.tag)
                    info = element_types.setdefault(
                        item_type,
                        {"count": 0, "properties": Counter(), "events": Counter(), "samples": []},
                    )
                    info["count"] += 1
                    sample_add(info["samples"], rel(path), 5)
                    for prop in item:
                        prop_name = local(prop.tag)
                        info["properties"][prop_name] += 1
                        if prop_name == "Events":
                            for event in prop:
                                event_name = event.attrib.get("name", "")
                                if event_name:
                                    info["events"][event_name] += 1
                                    event_add(element_events, event, path, item_type)

    def normalized_events(values: dict[str, dict[str, Any]]) -> list[dict[str, Any]]:
        result = []
        for name, info in sorted(values.items()):
            info["name"] = name
            info["owners"] = dict(sorted(info["owners"].items()))
            result.append(info)
        return result

    elements = []
    for name, info in sorted(element_types.items()):
        elements.append(
            {
                "name": name,
                "count": info["count"],
                "properties": dict(sorted(info["properties"].items())),
                "events": dict(sorted(info["events"].items())),
                "samples": info["samples"],
            }
        )
    write_json(
        OUTPUT / "02-managed-forms/form-schema.json",
        {
            "schema": "metalab.reference.managed-form-schema",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "source": rel(DEMO),
            "form_count": parsed,
            "form_properties": dict(sorted(form_properties.items())),
            "attribute_types": dict(sorted(attribute_types.items())),
            "command_properties": dict(sorted(command_properties.items())),
            "element_type_count": len(elements),
            "element_types": elements,
        },
    )
    write_json(
        OUTPUT / "02-managed-forms/observed-events.json",
        {
            "schema": "metalab.reference.observed-form-events",
            "schema_version": 1,
            "platform_version": VERSION,
            "official_complete_catalog": rel(ITS / "events/form-modules.json", OUTPUT / "02-managed-forms"),
            "official_element_catalog": rel(ITS / "events/form-elements.json", OUTPUT / "02-managed-forms"),
            "form_events": normalized_events(form_events),
            "element_events": normalized_events(element_events),
        },
    )


MODULE_FILE_KINDS = {
    "ManagedApplicationModule.bsl": "managed-application-module",
    "OrdinaryApplicationModule.bsl": "ordinary-application-module",
    "SessionModule.bsl": "session-module",
    "ExternalConnectionModule.bsl": "external-connection-module",
    "ObjectModule.bsl": "object-module",
    "ManagerModule.bsl": "manager-module",
    "RecordSetModule.bsl": "record-set-module",
    "ValueManagerModule.bsl": "value-manager-module",
    "CommandModule.bsl": "command-module",
    "Module.bsl": "service-module",
}


def procedure_names(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8-sig", errors="replace")
    return re.findall(r"(?im)^\s*(?:Процедура|Функция)\s+([A-Za-zА-Яа-яЁё_][\wА-Яа-яЁё]*)", text)


def module_catalog() -> None:
    kinds: dict[str, dict[str, Any]] = {}
    official = json.loads((ITS / "events/all-events.json").read_text(encoding="utf-8"))
    official_names = {event["name_ru"] for event in official["events"]}
    for path in sorted(DEMO.rglob("*.bsl")):
        kind = MODULE_FILE_KINDS.get(path.name, "other-module")
        if "/Ext/Form/" in path.as_posix() and path.name == "Module.bsl":
            kind = "form-module"
        elif "/CommonModules/" in path.as_posix() and path.name == "Module.bsl":
            kind = "common-module"
        info = kinds.setdefault(kind, {"count": 0, "samples": [], "observed_predefined_handlers": Counter()})
        info["count"] += 1
        sample_add(info["samples"], rel(path), 8)
        for name in procedure_names(path):
            if name in official_names:
                info["observed_predefined_handlers"][name] += 1

    contexts = [
        {"id": "managed-application-module", "context": "client/application", "events": rel(ITS / "events/special-modules/managed-application-module.json", OUTPUT / "03-module-contexts")},
        {"id": "ordinary-application-module", "context": "client/application", "events": rel(ITS / "events/special-modules/ordinary-application-module.json", OUTPUT / "03-module-contexts")},
        {"id": "session-module", "context": "server/session", "events": rel(ITS / "events/special-modules/session-module.json", OUTPUT / "03-module-contexts"), "predefined_variables": ["ПараметрыСеанса"]},
        {"id": "external-connection-module", "context": "server/external-connection", "events": rel(ITS / "events/special-modules/external-connection-module.json", OUTPUT / "03-module-contexts")},
        {"id": "object-module", "context": "server/object", "events": rel(ITS / "events/object-modules.json", OUTPUT / "03-module-contexts"), "predefined_variables": ["ЭтотОбъект"]},
        {"id": "record-set-module", "context": "server/record-set", "events": rel(ITS / "events/object-modules.json", OUTPUT / "03-module-contexts"), "predefined_variables": ["ЭтотОбъект"]},
        {"id": "manager-module", "context": "server-and-client-by-method-directive", "events": rel(ITS / "events/manager-modules.json", OUTPUT / "03-module-contexts")},
        {"id": "form-module", "context": "managed-client-and-server", "events": rel(ITS / "events/form-modules.json", OUTPUT / "03-module-contexts"), "predefined_variables": ["ЭтаФорма"]},
        {"id": "command-module", "context": "managed-client-or-server", "events": rel(ITS / "events/service-modules.json", OUTPUT / "03-module-contexts")},
        {"id": "service-module", "context": "server/service-request", "events": rel(ITS / "events/service-modules.json", OUTPUT / "03-module-contexts")},
        {"id": "value-manager-module", "context": "server/value-manager", "events": rel(ITS / "events/manager-modules.json", OUTPUT / "03-module-contexts")},
        {"id": "common-module", "context": "defined-by-module-properties", "events": None},
    ]
    write_json(
        OUTPUT / "03-module-contexts/module-contexts.json",
        {
            "schema": "metalab.reference.module-contexts",
            "schema_version": 1,
            "platform_version": VERSION,
            "contexts": contexts,
            "demo_inventory": [
                {
                    "id": name,
                    "count": info["count"],
                    "samples": info["samples"],
                    "observed_predefined_handlers": dict(sorted(info["observed_predefined_handlers"].items())),
                }
                for name, info in sorted(kinds.items())
            ],
            "api_availability_source": rel(ITS / f"{VERSION}/bsl-bridge/bsl-bridge.json", OUTPUT / "03-module-contexts"),
        },
    )


def query_catalog() -> None:
    query = json.loads((ITS / VERSION / "json/shquery_ru.json").read_text(encoding="utf-8"))
    dcs = json.loads((ITS / VERSION / "json/dcsui_ru.json").read_text(encoding="utf-8"))
    expression_pages = [
        page for page in dcs["pages"]
        if page["id"] in {"PresentSKD", "SKD"} or page["id"].startswith("SKD_")
    ]
    virtual_tables = Counter()
    examples: list[str] = []
    pattern = re.compile(
        r"(?i)(Регистр(?:Накопления|Сведений|Бухгалтерии|Расчета)\.[\wА-Яа-яЁё]+\.(?:ОстаткиИОбороты|Остатки|Обороты|СрезПоследних|СрезПервых))"
    )
    for path in DEMO.rglob("*.bsl"):
        text = path.read_text(encoding="utf-8-sig", errors="replace")
        found = pattern.findall(text)
        for value in found:
            virtual_tables[value] += 1
            sample_add(examples, rel(path), 20)
    template_files = sorted(DEMO.glob("**/Templates/*/Ext/Template.xml"))
    schema_examples = []
    element_counts: Counter[str] = Counter()
    xsi_types: Counter[str] = Counter()
    element_attributes: dict[str, Counter[str]] = defaultdict(Counter)
    parent_child: Counter[str] = Counter()
    element_paths: Counter[str] = Counter()
    namespaces: Counter[str] = Counter()
    schema_inventory = []
    for path in template_files:
        root = ET.parse(path).getroot()
        if local(root.tag) != "DataCompositionSchema":
            continue
        schema_examples.append(path)
        local_elements: Counter[str] = Counter()
        local_types: Counter[str] = Counter()
        def visit_dcs(node: ET.Element, parents: tuple[str, ...] = ()) -> None:
            name = local(node.tag)
            namespace = node.tag[1:].split("}", 1)[0] if node.tag.startswith("{") else ""
            element_counts[name] += 1
            local_elements[name] += 1
            namespaces[namespace] += 1
            current_path = parents + (name,)
            element_paths["/".join(current_path)] += 1
            if parents:
                parent_child[f"{parents[-1]}>{name}"] += 1
            for attribute, value in node.attrib.items():
                attribute_name = local(attribute)
                element_attributes[name][attribute_name] += 1
                if attribute_name == "type":
                    xsi_types[value] += 1
                    local_types[value] += 1
            for nested in node:
                visit_dcs(nested, current_path)

        visit_dcs(root)
        schema_inventory.append(
            {
                "path": rel(path),
                "size": path.stat().st_size,
                "element_count": sum(local_elements.values()),
                "data_sets": {name: count for name, count in sorted(local_types.items()) if name.startswith("DataSet")},
                "fields": local_elements["field"],
                "calculated_fields": local_elements["calculatedField"],
                "parameters": local_elements["dataParameters"],
                "settings_variants": local_elements["settingsVariant"],
                "queries": local_elements["query"],
            }
        )
    write_json(
        OUTPUT / "04-query-dcs/catalog.json",
        {
            "schema": "metalab.reference.query-dcs",
            "schema_version": 1,
            "platform_version": VERSION,
            "query_syntax_page_count": query["page_count"],
            "query_pages": [{"id": p["id"], "title": p["title"], "source_path": p["path"]} for p in query["pages"]],
            "dcs_help_page_count": dcs["page_count"],
            "dcs_pages": [{"id": p["id"], "title": p["title"], "source_path": p["path"]} for p in dcs["pages"]],
            "dcs_expression_page_count": len(expression_pages),
            "dcs_expression_pages": [{"id": p["id"], "title": p["title"], "source_path": p["path"]} for p in expression_pages],
            "observed_virtual_tables": dict(sorted(virtual_tables.items())),
            "observed_query_module_samples": examples,
            "template_xml_count": len(schema_examples),
            "template_samples": [rel(path) for path in schema_examples[:30]],
        },
    )
    write_json(
        OUTPUT / "04-query-dcs/expression-language.json",
        {
            "schema": "metalab.reference.dcs-expression-language",
            "schema_version": 1,
            "platform_version": VERSION,
            "source": rel(ITS / VERSION / "json/dcsui_ru.json", OUTPUT / "04-query-dcs"),
            "page_count": len(expression_pages),
            "pages": [
                {
                    "id": page["id"],
                    "title": page["title"],
                    "text": page.get("text", ""),
                    "links": page.get("links", []),
                    "source_path": page["path"],
                }
                for page in expression_pages
            ],
            "coverage": [
                "operations", "comparison", "boolean logic", "NULL", "priority",
                "aggregate functions", "dates", "numbers", "strings", "values",
                "record position", "expression evaluation", "extended query language",
            ],
        },
    )
    write_json(
        OUTPUT / "04-query-dcs/schema-model.json",
        {
            "schema": "metalab.reference.dcs-schema-model",
            "schema_version": 1,
            "platform_version": VERSION,
            "source": rel(DEMO, OUTPUT / "04-query-dcs"),
            "schema_count": len(schema_examples),
            "element_counts": dict(sorted(element_counts.items())),
            "xsi_types": dict(sorted(xsi_types.items())),
            "namespaces": dict(sorted(namespaces.items())),
            "parent_child": dict(sorted(parent_child.items())),
            "element_paths": dict(sorted(element_paths.items())),
            "element_attributes": {
                name: dict(sorted(attributes.items())) for name, attributes in sorted(element_attributes.items())
            },
            "schemas": schema_inventory,
            "important_limit": "Observed XML vocabulary from demo-base; unknown elements and attributes must be preserved.",
        },
    )

    bridge = json.loads((ITS / VERSION / "bsl-bridge/bsl-bridge.json").read_text(encoding="utf-8"))
    runtime_owners = {
        "КомпоновщикМакетаКомпоновкиДанных",
        "ПроцессорКомпоновкиДанных",
        "ПроцессорВыводаРезультатаКомпоновкиДанныхВТабличныйДокумент",
        "ПроцессорВыводаРезультатаКомпоновкиДанныхВКоллекциюЗначений",
        "ЗначенияПараметровДанныхКомпоновкиДанных",
        "КоллекцияЗначенийПараметровКомпоновкиДанных",
    }
    runtime_symbols = []
    for symbol in bridge["symbols"]:
        if symbol.get("name_ru") in runtime_owners or symbol.get("owner_ru") in runtime_owners:
            runtime_symbols.append(
                {
                    "kind": symbol["kind"],
                    "owner_ru": symbol.get("owner_ru"),
                    "name_ru": symbol.get("name_ru"),
                    "name_en": symbol.get("name_en"),
                    "syntaxes": symbol.get("syntaxes", []),
                    "source_path": symbol.get("source_path"),
                }
            )
    write_json(
        OUTPUT / "04-query-dcs/runtime-pipeline.json",
        {
            "schema": "metalab.reference.dcs-runtime-pipeline",
            "schema_version": 1,
            "platform_version": VERSION,
            "stages": [
                {"order": 1, "id": "schema", "input": "DataCompositionSchema XML", "result": "СхемаКомпоновкиДанных"},
                {"order": 2, "id": "settings", "input": "default, variant and user settings", "result": "НастройкиКомпоновкиДанных"},
                {"order": 3, "id": "layout", "api": "КомпоновщикМакетаКомпоновкиДанных.Выполнить", "result": "МакетКомпоновкиДанных"},
                {"order": 4, "id": "compose", "api": "ПроцессорКомпоновкиДанных.Инициализировать/Следующий", "result": "elements of composition result"},
                {"order": 5, "id": "output", "api": "ПроцессорВыводаРезультатаКомпоновкиДанных*", "result": "tabular document or value collection"},
            ],
            "runtime_symbols": runtime_symbols,
            "verification_rule": "Settings merge, totals, hierarchy, generated queries and output must be verified on the 1C oracle.",
        },
    )
    write_json(
        OUTPUT / "04-query-dcs/parameter-access.json",
        {
            "schema": "metalab.reference.dcs-parameter-access",
            "schema_version": 1,
            "platform_version": VERSION,
            "access_chain": [
                {"expression": "Настройки.ПараметрыДанных", "type": "ЗначенияПараметровДанныхКомпоновкиДанных"},
                {"expression": "Настройки.ПараметрыДанных.Элементы", "type": "КоллекцияЗначенийПараметровКомпоновкиДанных"},
                {"expression": "Настройки.ПараметрыДанных.Элементы.Найти(Имя)", "type": "ЗначениеПараметраНастроекКомпоновкиДанных"},
            ],
            "direct_methods": ["НайтиЗначениеПараметра", "УстановитьЗначениеПараметра"],
            "source_pages": [
                "objects/catalog63/catalog1064/catalog1077/DataCompositionDataParameterValues",
                "objects/catalog63/catalog1064/catalog1077/DataCompositionDataParameterValues/properties/Items5173",
                "objects/catalog63/catalog1064/catalog1077/DataCompositionDataParameterValues/methods/FindParameterValue2532",
                "objects/catalog63/catalog1064/catalog1077/DataCompositionDataParameterValues/methods/SetParameterValue3698",
            ],
            "verified_example": "Настройки.ПараметрыДанных.Элементы.Найти(\"ИсключитьВидыОперации\")",
        },
    )

    dcs_scenarios = [
        ("expression-null", "NULL and boolean three-valued expressions"),
        ("expression-functions", "string, number, date and value presentation functions"),
        ("calculated-field", "calculated field dependencies and evaluation errors"),
        ("parameters", "schema parameters, defaults and user values"),
        ("filter", "fixed and user filters with nested AND/OR groups"),
        ("group-and-totals", "grouping, resources, totals and percentage calculation"),
        ("hierarchy", "hierarchical grouping and hierarchy-only selection"),
        ("order", "explicit, automatic and presentation-based ordering"),
        ("conditional-appearance", "conditional formatting priority and application area"),
        ("dataset-query", "query data set and DCS query-language extensions"),
        ("dataset-object", "object and external data sets"),
        ("dataset-union", "union data sets, field mapping and missing values"),
        ("settings-merge", "default, variant and user settings merge"),
        ("output-tabular-document", "layout and output to a tabular document"),
        ("output-value-collection", "output to a value collection"),
        ("drilldown", "drill-down data and field decoding"),
    ]
    write_json(
        OUTPUT / "04-query-dcs/conformance-scenarios.json",
        {
            "schema": "metalab.reference.dcs-conformance-scenarios",
            "schema_version": 1,
            "platform_version": VERSION,
            "oracle": "1C:Enterprise 8.3.27.2342",
            "scenarios": [
                {
                    "id": identifier,
                    "subject": subject,
                    "capture": ["generated_query", "parameters", "rows", "group_levels", "totals", "output"],
                    "status": "verified" if identifier == "dataset-object" else "needs-oracle-run",
                    **(
                        {"result": "docs/materials/platform-model/04-query-dcs/oracle-results/1c/dataset-object.jsonl"}
                        if identifier == "dataset-object" else {}
                    ),
                }
                for identifier, subject in dcs_scenarios
            ],
        },
    )


def supporting_catalogs() -> None:
    lifecycle_scenarios = [
        ("catalog-write", "Запись нового и существующего элемента справочника", "Создать элемент, затем изменить его и записать повторно", ["event", "sequence", "is_new", "cancel", "transaction_active"]),
        ("catalog-delete", "Пометка и непосредственное удаление элемента справочника", "Сравнить пометку удаления и непосредственное удаление", ["event", "sequence", "deletion_mark", "cancel"]),
        ("document-write", "Обычная запись документа", "Записать новый документ без проведения и повторить для существующего", ["event", "sequence", "write_mode", "posting_mode", "cancel"]),
        ("document-post", "Проведение документа", "Провести документ с движениями регистра", ["event", "sequence", "write_mode", "posting_mode", "movements_visible", "transaction_active"]),
        ("document-undo-posting", "Отмена проведения документа", "Отменить проведение ранее проведённого документа", ["event", "sequence", "write_mode", "cancel", "movement_count"]),
        ("record-set-write", "Запись и замещение набора записей регистра", "Записать набор с Замещение=Ложь и Замещение=Истина", ["event", "sequence", "replace", "cancel"]),
        ("form-open", "Создание формы на сервере и открытие на клиенте", "Открыть форму объекта и форму списка", ["event", "sequence", "execution_context", "form_parameters"]),
        ("form-write", "Запись объекта из управляемой формы", "Изменить реквизит и выполнить стандартную команду записи", ["event", "sequence", "execution_context", "current_object_state", "cancel"]),
        ("session-start", "Инициализация параметров сеанса", "Запросить параметр до и после первичной инициализации", ["event", "sequence", "requested_parameters", "values"]),
        ("application-start-stop", "Запуск и завершение приложения", "Запустить клиент, штатно закрыть и отдельно отменить завершение", ["event", "sequence", "cancel", "warning_text"]),
        ("transaction-rollback", "Отказ обработчика и откат транзакции", "Изменить два объекта и вызвать исключение до фиксации", ["event", "sequence", "transaction_active", "persisted_values"]),
        ("query-null-undefined", "NULL, Неопределено и пустые ссылки в запросе", "Выполнить выражения сравнения, ЕСТЬNULL и соединение", ["expression", "value", "value_type", "row_count"]),
        ("by-reference-arguments", "Изменение параметров обработчика по ссылке", "Изменить Отказ и другие параметры в обработчике", ["event", "initial_arguments", "final_arguments", "operation_result"]),
    ]
    write_json(
        OUTPUT / "05-runtime/runtime-reference.json",
        {
            "schema": "metalab.reference.runtime",
            "schema_version": 1,
            "platform_version": VERSION,
            "event_catalog": rel(ITS / "events/index.json", OUTPUT / "05-runtime"),
            "type_catalog": rel(ITS / "types/index.json", OUTPUT / "05-runtime"),
            "local_help": {
                "application": rel(ITS / VERSION / "json/1cv8_ru.json", OUTPUT / "05-runtime"),
                "managed_client": rel(ITS / VERSION / "json/mngcln_ru.json", OUTPUT / "05-runtime"),
                "managed_base": rel(ITS / VERSION / "json/mngbase_ru.json", OUTPUT / "05-runtime"),
            },
            "verification_rule": "Порядок и изменение параметров фиксируются только после запуска сценария на 1С 8.3.27; наличие события в справке не задаёт последовательность.",
        },
    )
    write_json(
        OUTPUT / "03-module-contexts/compiler-contexts.json",
        {
            "schema": "metalab.reference.bsl-compiler-contexts",
            "schema_version": 1,
            "platform_version": VERSION,
            "source": rel(ITS / VERSION / "json/shlang_ru.json", OUTPUT / "03-module-contexts"),
            "source_pages": ["Pragma", "Instructions", "annotations"],
            "compilation_directives": [
                {"name_ru": "НаКлиенте", "name_en": "AtClient", "modules": ["form", "command", "common"]},
                {"name_ru": "НаСервере", "name_en": "AtServer", "modules": ["form", "command", "common"], "default_for_form_method": True},
                {"name_ru": "НаСервереБезКонтекста", "name_en": "AtServerNoContext", "modules": ["form"]},
                {"name_ru": "НаКлиентеНаСервереБезКонтекста", "name_en": "AtClientAtServerNoContext", "modules": ["form"]},
                {"name_ru": "НаКлиентеНаСервере", "name_en": "AtClientAtServer", "modules": ["command"]},
            ],
            "preprocessor_instructions": [
                "Если", "Тогда", "ИначеЕсли", "Иначе", "КонецЕсли",
                "Область", "КонецОбласти", "Вставка", "КонецВставки", "Удаление", "КонецУдаления",
            ],
            "preprocessor_symbols": [
                "Сервер", "НаСервере", "Клиент", "НаКлиенте", "ТонкийКлиент", "МобильныйКлиент",
                "ВебКлиент", "ВнешнееСоединение", "ТолстыйКлиентУправляемоеПриложение",
                "ТолстыйКлиентОбычноеПриложение", "МобильныйАвтономныйСервер",
                "МобильноеПриложениеКлиент", "МобильноеПриложениеСервер",
            ],
            "extension_annotations": [
                {"name_ru": "Перед", "name_en": "Before", "procedures": True, "functions": False},
                {"name_ru": "После", "name_en": "After", "procedures": True, "functions": False},
                {"name_ru": "Вместо", "name_en": "Around", "procedures": True, "functions": True},
                {"name_ru": "ИзменениеИКонтроль", "name_en": "ChangeAndValidate", "procedures": True, "functions": True},
            ],
            "notes": [
                "Preprocessor instructions are evaluated before compilation directives.",
                "A method cannot have multiple compilation directives.",
                "Compilation directives and extension annotations may be used together.",
            ],
        },
    )
    official_events = json.loads((ITS / "events/all-events.json").read_text(encoding="utf-8"))["events"]
    phases: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for event in official_events:
        name = event.get("name_ru") or ""
        if name.startswith(("Перед", "ОбработкаПроверки")):
            phase = "before-or-validation"
        elif name.startswith(("При", "Обработка")):
            phase = "operation"
        elif name.startswith(("После", "Завершение")):
            phase = "after"
        else:
            phase = "interaction-or-other"
        phases[phase].append(
            {
                "name_ru": name,
                "name_en": event.get("name_en"),
                "module_kind": event.get("module_kind"),
                "owner_ru": event.get("owner_ru"),
                "syntaxes": event.get("syntaxes", []),
                "source_path": event.get("source_path"),
            }
        )
    write_json(
        OUTPUT / "05-runtime/lifecycle-events.json",
        {
            "schema": "metalab.reference.lifecycle-events",
            "schema_version": 1,
            "platform_version": VERSION,
            "classification_note": "Фазы получены по назначению имени и служат навигацией; фактический порядок устанавливают oracle-сценарии.",
            "phases": {name: values for name, values in sorted(phases.items())},
        },
    )
    write_json(
        OUTPUT / "06-conformance/scenarios.json",
        {
            "schema": "metalab.reference.conformance-scenarios",
            "schema_version": 1,
            "platform_version": VERSION,
            "scenarios": [
                {
                    "id": identifier,
                    "title": title,
                    "action": action,
                    "capture": capture,
                    "status": "needs-oracle-run",
                    "oracle": "1C:Enterprise 8.3.27.2342",
                    "result_format": "ordered JSON Lines",
                }
                for identifier, title, action, capture in lifecycle_scenarios
            ],
            "corpora": [
                {"kind": "real-configuration", "path": rel(DEMO, OUTPUT / "06-conformance"), "export_format": "2.20"},
                {"kind": "language-reference", "path": rel(ROOT / "onescript/tests", OUTPUT / "06-conformance"), "role": "secondary BSL corpus, not platform oracle"},
                {"kind": "metalab-language-spec", "path": rel(Path("internal/bsl/spec/corpus"), OUTPUT / "06-conformance")},
            ],
        },
    )
    write_json(
        OUTPUT / "06-conformance/result-schema.json",
        {
            "$schema": "https://json-schema.org/draft/2020-12/schema",
            "$id": "metalab.reference.oracle-event.schema.json",
            "title": "One ordered observation from a 1C/MetaLab conformance run",
            "type": "object",
            "required": ["scenario", "run_id", "sequence", "event", "runtime", "platform_version"],
            "properties": {
                "scenario": {"type": "string"},
                "run_id": {"type": "string"},
                "sequence": {"type": "integer", "minimum": 1},
                "event": {"type": "string"},
                "runtime": {"enum": ["1c-oracle", "metalab"]},
                "platform_version": {"type": "string"},
                "timestamp": {"type": "string"},
                "execution_context": {"type": ["string", "null"]},
                "transaction_active": {"type": ["boolean", "null"]},
                "arguments_before": {"type": "object"},
                "arguments_after": {"type": "object"},
                "state": {"type": "object"},
                "result": {},
                "error": {"type": ["string", "null"]},
            },
            "additionalProperties": True,
        },
    )
    (OUTPUT / "06-conformance/oracle-run.md").write_text(
        "# Протокол эталонного прогона\n\n"
        "Эталон — платформа 1С:Предприятие 8.3.27.2342. Каждый сценарий из `scenarios.json` "
        "выполняется отдельно на чистой копии тестовой базы.\n\n"
        "1. Перед запуском присвоить уникальный `run_id` и обнулить счётчик `sequence`.\n"
        "2. В каждом задействованном обработчике записывать одну JSON-строку по `result-schema.json`.\n"
        "3. Фиксировать аргументы до и после обработчика, контекст выполнения и состояние транзакции.\n"
        "4. Сохранять результат в `results/1c/<scenario>/<run_id>.jsonl`; результат ML — в симметричный `results/metalab/`.\n"
        "5. Сравнивать порядок событий и нормализованные значения. Время и идентификаторы объектов в точное сравнение не включать.\n\n"
        "Каталог `results/` намеренно не создаётся генератором: в него помещаются только фактически полученные результаты.\n",
        encoding="utf-8",
    )

    image_counts = {}
    images = []
    for directory in sorted((ITS / VERSION / "html").iterdir()):
        if directory.is_dir():
            count = sum(1 for p in directory.rglob("*") if p.suffix.lower() in {".png", ".gif", ".jpg", ".jpeg", ".svg"})
            if count:
                image_counts[directory.name] = count
                images.extend(
                    rel(path)
                    for path in sorted(directory.rglob("*"))
                    if path.suffix.lower() in {".png", ".gif", ".jpg", ".jpeg", ".svg"}
                )
    write_json(
        OUTPUT / "07-ui-reference/index.json",
        {
            "schema": "metalab.reference.ui",
            "schema_version": 1,
            "platform_version": VERSION,
            "official_help_image_counts": image_counts,
            "official_help_images": images,
            "official_help_root": rel(ITS / VERSION / "html", OUTPUT / "07-ui-reference"),
            "secondary_onebase_images": rel(ROOT / "onebase/docs/images", OUTPUT / "07-ui-reference"),
            "managed_form_examples": rel(DEMO, OUTPUT / "07-ui-reference"),
            "workflows": [
                {"id": "configuration-tree", "surface": "configurator", "evidence": ["managed_form_examples"]},
                {"id": "property-editor", "surface": "configurator", "evidence": ["managed_form_examples"]},
                {"id": "module-editor", "surface": "configurator", "evidence": ["official_help_root", "managed_form_examples"]},
                {"id": "managed-form-editor", "surface": "configurator", "evidence": ["managed_form_examples"]},
                {"id": "role-editor", "surface": "configurator", "evidence": ["managed_form_examples"]},
                {"id": "query-builder", "surface": "configurator", "evidence": ["official_help_root"]},
                {"id": "syntax-assistant", "surface": "configurator", "evidence": ["official_help_root"]},
                {"id": "enterprise-object-form", "surface": "enterprise", "evidence": ["official_help_images"]},
                {"id": "enterprise-list-form", "surface": "enterprise", "evidence": ["official_help_images"]},
            ],
        },
    )
    write_json(
        OUTPUT / "07-ui-reference/workflows.json",
        {
            "schema": "metalab.reference.ui-workflows",
            "schema_version": 1,
            "platform_version": VERSION,
            "workflows": [
                {"id": "configuration-tree", "required_actions": ["expand-collapse", "search", "create", "rename", "delete", "open-properties"], "state": ["selection", "expanded-nodes", "modified"]},
                {"id": "property-editor", "required_actions": ["inspect", "edit", "reset", "choose-type"], "state": ["value", "default-value", "validation-error"]},
                {"id": "module-editor", "required_actions": ["edit", "find", "navigate-to-definition", "syntax-help", "diagnostics"], "state": ["cursor", "selection", "modified", "diagnostics"]},
                {"id": "managed-form-editor", "required_actions": ["edit-elements", "edit-attributes", "edit-commands", "bind-event", "preview"], "state": ["element-tree", "attribute-tree", "selection"]},
                {"id": "role-editor", "required_actions": ["select-object", "toggle-right", "set-restriction"], "state": ["role", "object", "rights"]},
                {"id": "query-builder", "required_actions": ["select-table", "select-field", "join", "filter", "group", "order", "view-query"], "state": ["tables", "fields", "query-text"]},
                {"id": "syntax-assistant", "required_actions": ["browse-tree", "search", "open-topic", "insert-syntax"], "state": ["topic", "history", "search-query"]},
                {"id": "enterprise-object-form", "required_actions": ["open", "edit", "save", "save-and-close", "close"], "state": ["object", "modified", "validation"]},
                {"id": "enterprise-list-form", "required_actions": ["open", "search", "filter", "sort", "create", "open-item"], "state": ["rows", "selection", "filter", "sort"]},
            ],
            "scope_note": "This is an interaction-coverage checklist, not a pixel-perfect UI specification.",
        },
    )
    write_json(
        OUTPUT / "08-developer-guide/sources.json",
        {
            "schema": "metalab.reference.developer-guide-sources",
            "schema_version": 1,
            "platform_version": VERSION,
            "official_sources": [
                {"title": "1C:Enterprise Developer Guide 8.3.27", "url": "https://1c-dn.com/library/tutorials/1c_enterprise_developer_guide_8_3_27/", "language": "en"},
                {"title": "1С:Предприятие 8.3.27. Документация", "url": "https://its.1c.ru/db/v8327doc", "language": "ru"},
                {"title": "Глава 2. Работа с конфигурацией", "url": "https://its.1c.ru/db/v8327doc/bookmark/dev/TI000001619", "language": "ru"},
                {"title": "Глава 7. Формы", "url": "https://its.1c.ru/db/v8327doc/bookmark/dev/TI000002900", "language": "ru"},
            ],
            "local_help_catalogs": [
                rel(path, OUTPUT / "08-developer-guide")
                for path in sorted((ITS / VERSION / "json").glob("*.json"))
            ],
            "required_chapters": ["configuration-and-xml-export", "language-and-types", "queries", "forms", "application-objects", "registers-and-posting", "transactions-and-locks", "rights", "jobs", "debugging"],
        },
    )
    write_json(
        OUTPUT / "08-developer-guide/coverage.json",
        {
            "schema": "metalab.reference.material-coverage",
            "schema_version": 1,
            "platform_version": VERSION,
            "areas": [
                {"id": "metadata", "local": "docs/materials/platform-model/01-metadata", "primary": "demo-base XML 2.20"},
                {"id": "managed-forms", "local": "docs/materials/platform-model/02-managed-forms", "primary": "demo-base and official help"},
                {"id": "module-contexts", "local": "docs/materials/platform-model/03-module-contexts", "primary": "official events and demo-base modules"},
                {"id": "query-dcs", "local": "docs/materials/platform-model/04-query-dcs", "primary": "official local help and demo-base"},
                {"id": "runtime", "local": "docs/materials/platform-model/05-runtime", "primary": "official help; ordering requires oracle run"},
                {"id": "conformance", "local": "docs/materials/platform-model/06-conformance", "primary": "1C 8.3.27.2342 oracle"},
                {"id": "ui", "local": "docs/materials/platform-model/07-ui-reference", "primary": "official help images and demo-base forms"},
                {"id": "developer-guide", "local": "docs/materials/platform-model/08-developer-guide", "primary": "official online guide and local help"},
            ],
        },
    )


def root_index() -> None:
    sections = [
        ("01-metadata", "Метаданные и XML 2.20", "cataloged-from-demo"),
        ("02-managed-forms", "Управляемые формы", "cataloged-from-help-and-demo"),
        ("03-module-contexts", "Контексты модулей", "cataloged"),
        ("04-query-dcs", "Запросы и СКД", "cataloged"),
        ("05-runtime", "Поведение runtime", "source-indexed"),
        ("06-conformance", "Эталонные сценарии", "specified-needs-oracle-run"),
        ("07-ui-reference", "Визуальный эталон", "source-indexed"),
        ("08-developer-guide", "Руководство разработчика", "source-indexed"),
    ]
    write_json(
        OUTPUT / "index.json",
        {
            "schema": "metalab.platform-reference-index",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "sections": [{"directory": directory, "title": title, "status": status} for directory, title, status in sections],
            "source_priority": ["official 1C 8.3.27 help", "demo-base XML 2.20", "observed 1C behavior", "OneScript/OneBase as secondary references"],
        },
    )
    write_json(
        OUTPUT / "known-gaps.json",
        {
            "schema": "metalab.platform-reference-known-gaps",
            "schema_version": 1,
            "platform_version": VERSION,
            "development_can_start": True,
            "gaps": [
                {
                    "id": "complete-metadata-xsd",
                    "status": "open-nonblocking",
                    "impact": "Unknown XML variants must be preserved and handled incrementally.",
                    "current_coverage": "47 metadata kinds and 10,310 observed child objects from demo-base.",
                },
                {
                    "id": "runtime-event-order",
                    "status": "needs-oracle-run",
                    "impact": "Exact lifecycle compatibility cannot be claimed before running the conformance scenarios.",
                    "current_coverage": "689 documented events and 13 executable scenario specifications.",
                },
                {
                    "id": "pixel-perfect-ui-reference",
                    "status": "open-nonblocking",
                    "impact": "Interaction compatibility can be implemented before exact visual matching.",
                    "current_coverage": "Official help images, 930 managed forms and nine workflow checklists.",
                },
                {
                    "id": "developer-guide-local-copy",
                    "status": "indexed-not-copied",
                    "impact": "Online access may be required for methodological chapters not present in local help.",
                    "current_coverage": "Official 8.3.27 guide links and 38 local help catalogs.",
                },
            ],
            "implementation_rule": "Treat unknown metadata, properties, types and events as forward-compatible data; do not silently discard them.",
        },
    )
    lines = [
        "# Модель платформы 1С 8.3.27 для MetaLab",
        "",
        "Машинные каталоги и эталонные материалы, построенные из официальной локальной справки 1С 8.3.27.2342 и XML-выгрузки demo-base формата 2.20.",
        "",
    ]
    for directory, title, _ in sections:
        lines.append(f"- `{directory}/` — {title}.")
    lines.extend(["", "OneScript и OneBase используются только как вторичные инженерные источники, не как эталон поведения платформы 1С.", ""])
    lines.extend([
        "Повторная генерация: `python3 scripts/materials_platform_model.py`.",
        "Проверка целостности: `python3 scripts/materials_validate.py`.",
        "Известные неполные области: `known-gaps.json`.",
        "",
    ])
    (OUTPUT / "README.md").write_text("\n".join(lines), encoding="utf-8")


def representative_fixtures() -> None:
    fixtures = []
    by_kind: dict[str, list[Path]] = defaultdict(list)
    for path in descriptor_files():
        root = ET.parse(path).getroot()
        node = next(iter(root), root)
        by_kind[local(node.tag)].append(path)
    for kind, paths in sorted(by_kind.items()):
        paths.sort(key=lambda path: (path.stat().st_size, path.as_posix()))
        selected = [paths[0]]
        if len(paths) > 1 and paths[-1] != paths[0]:
            selected.append(paths[-1])
        for role, path in zip(("minimal-observed", "rich-observed"), selected):
            object_dir = path.with_suffix("")
            fixtures.append(
                {
                    "kind": kind,
                    "role": role,
                    "descriptor": rel(path),
                    "size": path.stat().st_size,
                    "modules": [rel(module) for module in sorted(object_dir.glob("Ext/*.bsl"))] if object_dir.is_dir() else [],
                    "forms": [rel(form) for form in sorted(object_dir.glob("Forms/*.xml"))[:5]] if object_dir.is_dir() else [],
                }
            )
    write_json(
        OUTPUT / "06-conformance/representative-fixtures.json",
        {
            "schema": "metalab.reference.representative-fixtures",
            "schema_version": 1,
            "platform_version": VERSION,
            "export_format": "2.20",
            "selection": "smallest and largest descriptor observed for every metadata kind",
            "fixtures": fixtures,
        },
    )


def section_readmes() -> None:
    content = {
        "01-metadata": ("Метаданные", "`object-kinds.json` описывает 47 встреченных видов объектов и их свойства. `property-index.json` даёт обратный индекс свойств. `child-objects.json` описывает реквизиты, табличные части, измерения и ресурсы. `security-and-jobs.json` индексирует права, RLS и расписания. Это эмпирическая схема XML 2.20 из demo-base."),
        "02-managed-forms": ("Управляемые формы", "`form-schema.json` содержит элементы, свойства, типы реквизитов и команды 930 форм. `observed-events.json` связывает использованные обработчики с полным официальным каталогом событий ИТС."),
        "03-module-contexts": ("Контексты модулей", "`module-contexts.json` объединяет виды модулей, события, предопределённые переменные и наблюдаемые обработчики. `compiler-contexts.json` фиксирует директивы компиляции, препроцессор и аннотации расширений."),
        "04-query-dcs": ("Запросы и СКД", "`catalog.json` — общий индекс. `expression-language.json` содержит официальный язык выражений СКД. `schema-model.json` описывает 57 наблюдаемых XML-схем. `runtime-pipeline.json` фиксирует этапы и API исполнения. `parameter-access.json` уточняет доступ через `ПараметрыДанных.Элементы`. `conformance-scenarios.json` задаёт проверки на 1С."),
        "05-runtime": ("Runtime", "`runtime-reference.json` задаёт первичные источники. `lifecycle-events.json` группирует события для реализации жизненного цикла. Точный порядок должен подтверждаться сценариями 1С."),
        "06-conformance": ("Эталонные сценарии", "`scenarios.json` задаёт наблюдения для 1С 8.3.27. `result-schema.json` и `oracle-run.md` фиксируют машинный формат и протокол прогона. `representative-fixtures.json` выбирает минимальный и насыщенный XML-образец каждого вида метаданных."),
        "07-ui-reference": ("Визуальный эталон", "`index.json` содержит 141 изображение официальной встроенной справки и ссылки на формы demo-base. `workflows.json` задаёт обязательные действия и состояния основных экранов. OneBase — только вторичный пример."),
        "08-developer-guide": ("Руководство разработчика", "`sources.json` индексирует официальное руководство 8.3.27, необходимые главы и все локальные разделы справки. `coverage.json` связывает восемь направлений с их первичными источниками."),
    }
    for directory, (title, body) in content.items():
        path = OUTPUT / directory / "README.md"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(f"# {title}\n\n{body}\n", encoding="utf-8")


def main() -> None:
    metadata_catalog()
    security_jobs_catalog()
    child_objects_catalog()
    managed_forms_catalog()
    module_catalog()
    query_catalog()
    supporting_catalogs()
    representative_fixtures()
    root_index()
    section_readmes()
    print(f"Created platform reference in {OUTPUT}")


if __name__ == "__main__":
    main()
