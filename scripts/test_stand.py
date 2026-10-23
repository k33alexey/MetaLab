#!/usr/bin/env python3
"""Собрать стенды тест-кейсов блока 2в: С1, С2 или С3 (`docs/test-cases/blok-2v.md`, «Стенды»).

Зачем. Кейсы окон роли и дерева проходятся на проекте, где есть объект каждого
вида с частями и подсистемы в три уровня. Собирать его руками по описанию
стенда — час работы и опечатки в идентификаторах; скрипт собирает его за миг и
одинаково каждый раз.

Запуск: `python3 scripts/test_stand.py <каталог> c1|c2|c3`. Каталог
перезаписывается, если в нём прежний стенд — его скрипт метит файлом
`.ml-test-stand`; каталог с чем-то другим, внутри репозитория или выше него
скрипт не трогает и отказывает (ревью 2.464, П1: опечатка в пути удаляла
бы рабочую копию). С1 — копия `examples/sales-and-warehouse`; С2 — С1 и объект
каждого вида, которого в демо нет, с командой, реквизитами, измерениями,
признаками учёта, перерасчётом, шаблоном URL, операцией, внешним источником с
таблицей, кубом, таблицей измерения и функцией; С3 — С2 и подсистемы «Продажи»
→ «Розница» → «Касса» и «Склад». Идентификаторы новых объектов —
`e7000000-0000-4000-8000-…` по порядку записи.

Студия по проекту открывается из ML Manager (`ML-MANAGER.md`); проект
проверяется тем, что Studio открывает его без ошибок и кнопка «По подсистемам»
над деревом доступна — она недоступна, пока не читается хоть одно описание.
"""
import os, shutil, sys, itertools

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if len(sys.argv) != 3 or sys.argv[2] not in ("c1", "c2", "c3"):
    sys.exit("usage: test_stand.py <folder> c1|c2|c3")
target = sys.argv[1]
level = sys.argv[2]

# Метка стенда: каталог с ней скрипт сделал сам, и перезаписать его можно.
MARK = ".ml-test-stand"

def refuse(reason):
    sys.exit(f"test_stand.py: {target}: {reason}")

real, repo = os.path.realpath(target), os.path.realpath(REPO)
if os.path.commonpath([real, repo]) in (real, repo):
    refuse("каталог внутри репозитория или выше него; укажите каталог вне репозитория")
if os.path.lexists(target):
    if not os.path.isfile(os.path.join(target, MARK)):
        refuse("каталог есть, и в нём не стенд этого скрипта; удалите его сами или укажите новый")
    shutil.rmtree(target)
shutil.copytree(os.path.join(REPO, "examples/sales-and-warehouse"), target)
with open(os.path.join(target, MARK), "w") as mark:
    mark.write("Стенд тест-кейсов, собран scripts/test_stand.py; каталог перезаписывается.\n")
if level == "c1":
    sys.exit(0)

counter = itertools.count(1)
def nid():
    return "e7000000-0000-4000-8000-%012x" % next(counter)

GOODS = "10000000-0000-4000-8000-000000000101"
PARTNERS = "10000000-0000-4000-8000-000000000102"
CMD_BSL = "&НаКлиенте\nПроцедура ОбработкаКоманды(ПараметрКоманды, ПараметрыВыполненияКоманды)\nКонецПроцедуры\n"

def write(rel, text):
    path = os.path.join(target, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(text)

def head(id_, name, title=None):
    return f"format: 1\nid: {id_}\nname: {name}\ntitle: {{ru: {title or name}}}\n"

def command(kind, obj, name="ОткрытьСписок"):
    write(f"metadata/{kind}/{obj}/commands/{name}/МодульКоманды.bsl", CMD_BSL)
    return f"commands:\n  - id: {nid()}\n    name: {name}\n    title: {{ru: Открыть список}}\n    group: navigation-panel-ordinary\n"

def folder(kind, name, body, with_command=True):
    id_ = nid()
    text = head(id_, name) + body
    if with_command:
        text += command(kind, name)
    write(f"metadata/{kind}/{name}/object.yaml", text)
    return id_

ids = {}
ids["info"] = folder("information-registers", "Курсы", f"""write_mode: independent
periodicity: day
dimensions:
  - {{id: {nid()}, name: Контрагент, title: {{ru: Контрагент}}, types: [{{kind: catalog, reference: {PARTNERS}}}]}}
resources:
  - {{id: {nid()}, name: Курс, title: {{ru: Курс}}, types: [{{kind: number, precision: 15, scale: 4}}]}}
""")
ids["pvh"] = folder("charts-of-characteristic-types", "ВидыСубконто", """code: {type: string, length: 9, auto: true}
description_length: 150
value_type: [{kind: string, length: 100}]
""")
ids["accounts"] = folder("charts-of-accounts", "Основной", f"""code: {{type: string, length: 9, auto: false}}
description_length: 150
ext_dimension_types: {ids['pvh']}
max_ext_dimension_count: 3
accounting_flags:
  - {{id: {nid()}, name: Количественный, title: {{ru: Количественный}}}}
ext_dimension_accounting_flags:
  - {{id: {nid()}, name: Суммовой, title: {{ru: Суммовой}}}}
""")
ids["pvr"] = folder("charts-of-calculation-types", "ВидыНачислений", """code: {type: string, length: 9}
description_length: 150
""")
dim = nid()
ids["calc"] = folder("calculation-registers", "Начисления", f"""chart_of_calculation_types: {ids['pvr']}
periodicity: month
dimensions:
  - {{id: {dim}, name: Контрагент, title: {{ru: Контрагент}}, types: [{{kind: catalog, reference: {PARTNERS}}}]}}
resources:
  - {{id: {nid()}, name: Результат, title: {{ru: Результат}}, types: [{{kind: number, precision: 15, scale: 2}}]}}
recalculations:
  - id: {nid()}
    name: Перерасчет
    title: {{ru: Перерасчёт}}
    dimensions:
      - {{id: {nid()}, name: Контрагент, title: {{ru: Контрагент}}, register_dimension: {dim}}}
""")
# Модуль перерасчёта: дерево Studio показывает перерасчёт и без папки модуля
# (с 2.439), а с ним стенд проверяет и открытие модуля из дерева.
write("metadata/calculation-registers/Начисления/recalculations/Перерасчет/МодульНабораЗаписей.bsl", "")
ids["accreg"] = folder("accounting-registers", "Хозрасчетный", f"""chart_of_accounts: {ids['accounts']}
correspondence: true
resources:
  - {{id: {nid()}, name: Сумма, title: {{ru: Сумма}}, types: [{{kind: number, precision: 15, scale: 2}}], balance: false}}
""")
taskdim = nid()
ids["addr"] = folder("information-registers", "Адресация", f"""write_mode: independent
periodicity: none
dimensions:
  - {{id: {taskdim}, name: Исполнитель, title: {{ru: Исполнитель}}, types: [{{kind: catalog, reference: {PARTNERS}}}]}}
""", with_command=False)
ids["task"] = folder("tasks", "Исполнение", f"""number: {{type: string, length: 14, auto: true, unique: true, periodicity: none}}
description_length: 150
number_prefix: business-process-number
addressing: {ids['addr']}
main_addressing_attribute: Исполнитель
addressing_attributes:
  - id: {nid()}
    name: Исполнитель
    title: {{ru: Исполнитель}}
    types: [{{kind: catalog, reference: {PARTNERS}}}]
    dimension: {taskdim}
""")
ids["bp"] = folder("business-processes", "Задание", f"""number: {{type: string, length: 11, auto: true, unique: true, periodicity: none}}
task: {ids['task']}
route:
  points:
    - {{id: {nid()}, name: Старт, kind: start}}
    - {{id: {nid()}, name: Завершение, kind: completion}}
  transitions:
    - {{from: Старт, to: Завершение}}
""")
ids["exchange"] = folder("exchange-plans", "Филиалы", f"""code: {{type: string, length: 36, auto: false}}
description_length: 150
content:
  - {{kind: catalogs, object: {GOODS}, auto_record: allow}}
""")
ids["dp"] = folder("data-processors", "ЗагрузкаЦен", "")
ids["report"] = folder("reports", "ОстаткиНаСкладе", "")
ids["enum"] = folder("enumerations", "СтатусыЗаказов", f"""values:
  - {{id: {nid()}, name: Новый, title: {{ru: Новый}}}}
  - {{id: {nid()}, name: Отгружен, title: {{ru: Отгружен}}}}
""")
ids["filter"] = folder("filter-criteria", "ПоКонтрагенту", f"""types: [{{kind: catalog, reference: {PARTNERS}}}]
""")
write("metadata/filter-criteria/ПоКонтрагенту/МодульМенеджера.bsl", "")
ids["const"] = nid()
write("metadata/constants/ОсновнойСклад/object.yaml", head(ids["const"], "ОсновнойСклад", "Основной склад") + "types: [{kind: string, length: 50}]\n")
ids["cc"] = nid()
write("metadata/common-commands/ОбновитьЦены/object.yaml", head(ids["cc"], "ОбновитьЦены", "Обновить цены") + "group: navigation-panel-ordinary\n")
write("metadata/common-commands/ОбновитьЦены/МодульКоманды.bsl", CMD_BSL)
ids["seq"] = nid()
write("metadata/sequences/Партии/object.yaml", head(ids["seq"], "Партии"))
ids["journal"] = folder("document-journals", "ОбщийЖурнал", "documents: [10000000-0000-4000-8000-000000000201]\n")
ids["http"] = nid()
write("metadata/http-services/Биллинг/object.yaml", head(ids["http"], "Биллинг") + f"""root_url: billing
templates:
  - id: {nid()}
    name: Версия
    title: {{ru: Версия}}
    template: /version
    methods:
      - {{id: {nid()}, name: Получить, title: {{ru: Получить}}, method: GET, handler: ВерсияПолучить}}
""")
write("metadata/http-services/Биллинг/МодульСервиса.bsl", "Функция ВерсияПолучить(Запрос)\n\tВозврат Неопределено;\nКонецФункции\n")
ids["ws"] = nid()
write("metadata/web-services/ОбменДанными/object.yaml", head(ids["ws"], "ОбменДанными", "Обмен данными") + f"""namespace: http://example.org/exchange/1.0
operations:
  - {{id: {nid()}, name: Проверка, title: {{ru: Проверка}}, procedure: Проверка, return_type: {{namespace: "http://www.w3.org/2001/XMLSchema", name: string}}}}
""")
write("metadata/web-services/ОбменДанными/МодульСервиса.bsl", "Функция Проверка()\n\tВозврат \"\";\nКонецФункции\n")
src = "metadata/external-data-sources/Склад"
ids["src"] = nid()
write(src + "/object.yaml", head(ids["src"], "Склад") + f"functions:\n  - {{id: {nid()}, name: Остаток, title: {{ru: Остаток}}, expression_in_data_source: 'remains(&1)'}}\n")
write(src + "/tables/Товары/object.yaml", head(nid(), "Товары") + f"""name_in_data_source: dbo.Goods
data_type: non-object
fields:
  - {{id: {nid()}, name: Код, title: {{ru: Код}}, types: [{{kind: string, length: 20}}], name_in_data_source: code}}
""")
write(src + "/cubes/Продажи/object.yaml", head(nid(), "Продажи") + f"""name_in_data_source: Sales
dimensions:
  - {{id: {nid()}, name: Период, title: {{ru: Период}}, types: [{{kind: date}}]}}
resources:
  - {{id: {nid()}, name: Количество, title: {{ru: Количество}}, types: [{{kind: number, precision: 15, scale: 3}}], name_in_data_source: Quantity}}
commands:
  - {{id: {nid()}, name: ОткрытьКуб, title: {{ru: Открыть куб}}, group: navigation-panel-ordinary}}
""")
write(src + "/cubes/Продажи/commands/ОткрытьКуб/МодульКоманды.bsl", CMD_BSL)
write(src + "/cubes/Продажи/dimension-tables/Склады/object.yaml", head(nid(), "Склады") + f"""name_in_data_source: Stores
fields:
  - {{id: {nid()}, name: Адрес, title: {{ru: Адрес}}, types: [{{kind: string, length: 50}}], name_in_data_source: address}}
commands:
  - {{id: {nid()}, name: ОткрытьСклады, title: {{ru: Открыть склады}}, group: navigation-panel-ordinary}}
""")
write(src + "/cubes/Продажи/dimension-tables/Склады/commands/ОткрытьСклады/МодульКоманды.bsl", CMD_BSL)

# Регистратор регистров расчёта и бухгалтерии: без него «Сохранить данные»
# отказывает (`METADATA-OBJECTS.md`), а стенд С5 сохраняет С2 и С3.
ids["accrual"] = folder("documents", "Начисление", f"""number: {{type: string, length: 11, auto: true, unique: true, periodicity: year}}
posting: {{allowed: true}}
movements: [{ids['calc']}, {ids['accreg']}]
""")
write("metadata/documents/Начисление/МодульОбъекта.bsl", "")

if level == "c2":
    sys.exit(0)

def subsystem(name, parent, members):
    id_ = nid()
    text = head(id_, name)
    if parent:
        text += f"parent: {parent}\n"
    if members:
        text += "members: [" + ", ".join(members) + "]\n"
    write(f"metadata/subsystems/{id_}.yaml", text)
    return id_

SALE = "10000000-0000-4000-8000-000000000202"
RECEIPT = "10000000-0000-4000-8000-000000000201"
sales = subsystem("Продажи", None, [SALE])
retail = subsystem("Розница", sales, [PARTNERS])
subsystem("Касса", retail, [GOODS, ids["calc"]])
subsystem("Склад", None, [RECEIPT, ids["src"]])
