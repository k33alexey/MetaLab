#!/usr/bin/env python3
"""Запросник по разобранной справке платформы (bsl-bridge.json).

Отвечает на вопрос «что можно написать дальше»: проходит выражение по точкам,
на каждом шаге подставляя тип свойства или тип возврата метода, и показывает
состав того типа, до которого дошёл.

Нужен потому, что поиск по имени метода в общем списке из двадцати пяти тысяч
символов даёт ложный ответ: «Найти» есть у сотни коллекций, и по одному имени
не видно, у той ли она, к которой вы обращаетесь.

Примеры:

    python3 scripts/bsl_query.py КомпоновщикНастроек.Настройки.ПараметрыДанных
    python3 scripts/bsl_query.py ТаблицаЗначений --member Найти
    python3 scripts/bsl_query.py --whose Найти
    python3 scripts/bsl_query.py --search КомпоновщикНастроек
    python3 scripts/bsl_query.py --table РегистрНакопления.Остатки
    python3 scripts/bsl_query.py --syntax НАЧАЛОПЕРИОДА
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any

DEFAULT_MODEL = Path("docs/materials/its/8.3.27.2342/bsl-bridge/bsl-bridge.json")
DEFAULT_TABLES = Path("docs/materials/its/query-tables/tables.json")
TYPE_KINDS = {"type", "global_context"}
# Значения, которые тип не продолжают: дальше по ним идти некуда.
DEAD_TYPES = {"неопределено", "null", "произвольный", "булево", "строка", "число", "дата"}


class Model:
    def __init__(self, path: Path) -> None:
        data = json.loads(path.read_text(encoding="utf-8"))
        self.version = data.get("platform_version", "?")
        self.symbols: list[dict[str, Any]] = data["symbols"]
        self.members: dict[str, list[dict[str, Any]]] = defaultdict(list)
        self.types: dict[str, dict[str, Any]] = {}
        for symbol in self.symbols:
            owner = symbol.get("owner_ru")
            if owner:
                self.members[owner.lower()].append(symbol)
            if symbol["kind"] in TYPE_KINDS and symbol.get("name_ru"):
                self.types.setdefault(symbol["name_ru"].lower(), symbol)

    def find_type(self, name: str, strict: bool = False) -> tuple[str | None, list[str]]:
        """Вернуть точное имя типа либо список кандидатов.

        strict запрещает догадку по части имени: она уместна для первого слова
        выражения, но не там, где строка уже могла оказаться «тип.член».
        """
        folded = name.lower()
        if folded in self.types:
            return self.types[folded]["name_ru"], []
        # Параметризованный тип: «СправочникСсылка.Товары» описан в справке как
        # «СправочникСсылка.<Имя справочника>».
        if "." in name:
            head = name.rsplit(".", 1)[0].lower()
            pattern = re.compile(rf"^{re.escape(head)}\.<[^>]+>$", re.IGNORECASE)
            for key, symbol in self.types.items():
                if pattern.match(key):
                    return symbol["name_ru"], []
        # Тип, у которого есть члены, но своей страницы нет.
        if folded in self.members:
            return name, []
        if strict:
            return None, []
        candidates = sorted(
            symbol["name_ru"] for key, symbol in self.types.items() if folded in key
        )
        if len(candidates) == 1:
            return candidates[0], []
        return None, candidates[:40]

    def member(self, owner: str, name: str) -> tuple[dict[str, Any] | None, list[str]]:
        """Только точное совпадение имени.

        Похожие имена возвращаются подсказкой, но никогда не подставляются
        сами: «Найти» и «НайтиЗначениеПараметра» — разные методы у разных
        типов, и подстановка по похожести даёт ровно ту ошибку, ради которой
        запросник и написан.
        """
        items = self.members.get(owner.lower(), [])
        folded = name.lower()
        for item in items:
            if (item.get("name_ru") or "").lower() == folded:
                return item, []
        near = sorted({item["name_ru"] for item in items if folded in (item.get("name_ru") or "").lower()})
        return None, near[:40]


def value_types(symbol: dict[str, Any]) -> list[str]:
    """Типы, которыми продолжается выражение после этого члена."""
    if symbol["kind"] == "method":
        result = (symbol.get("return") or {}).get("types") or []
    else:
        result = symbol.get("value_types") or []
    # Справка иногда пишет «А или Б» одной строкой.
    expanded: list[str] = []
    for item in result:
        expanded.extend(part.strip() for part in re.split(r"\s+или\s+|,", item) if part.strip())
    return [item for item in expanded if item.lower() not in DEAD_TYPES]


def signature(symbol: dict[str, Any]) -> str:
    syntaxes = symbol.get("syntaxes") or []
    if syntaxes:
        return syntaxes[0]
    types = ", ".join(symbol.get("value_types") or [])
    return f"{symbol['name_ru']}: {types}" if types else symbol["name_ru"]


def print_member(symbol: dict[str, Any], full: bool = False) -> None:
    print(f"  {symbol['kind']:9} {signature(symbol)}")
    if not full:
        return
    if symbol.get("access"):
        print(f"      использование: {symbol['access']}")
    for parameter in symbol.get("parameters") or []:
        need = "обязательный" if parameter.get("required") else "необязательный"
        if not parameter.get("requirement_stated", True):
            need = "обязательность не указана"
        types = ", ".join(parameter.get("types") or []) or "тип не указан"
        print(f"      <{parameter['name']}> ({need}) — {types}")
        if parameter.get("description"):
            print(f"          {parameter['description'].splitlines()[0]}")
    returned = symbol.get("return")
    if returned:
        print(f"      возврат: {', '.join(returned.get('types') or []) or '—'}")
        if returned.get("description"):
            print(f"          {returned['description'].splitlines()[0]}")
    if symbol.get("availability"):
        print(f"      доступность: {', '.join(symbol['availability'])}")
    if symbol.get("description"):
        print(f"      {symbol['description'].splitlines()[0]}")


def show_type(model: Model, name: str, member_filter: str | None) -> int:
    items = model.members.get(name.lower(), [])
    if not items:
        print(f"У типа {name} членов в справке нет.")
        return 1
    if member_filter:
        symbol, near = model.member(name, member_filter)
        if symbol is None:
            print(f"У типа {name} нет члена {member_filter!r}.")
            if near:
                print("  Похожие:", ", ".join(near))
            return 1
        print(f"{name}.{symbol['name_ru']}")
        print_member(symbol, full=True)
        following = value_types(symbol)
        if following:
            print(f"      продолжается типом: {', '.join(following)}")
        return 0
    print(f"{name} — членов {len(items)}")
    for kind in ("method", "property", "event"):
        group = sorted((item for item in items if item["kind"] == kind), key=lambda item: item["name_ru"])
        if not group:
            continue
        titles = {"method": "методы", "property": "свойства", "event": "события"}
        print(f"\n  {titles[kind]} ({len(group)}):")
        for item in group:
            print(f"    {signature(item)}")
    return 0


def walk(model: Model, expression: str, member_filter: str | None) -> int:
    segments = [segment.strip() for segment in split_path(expression) if segment.strip()]
    if not segments:
        print("Пустое выражение.")
        return 2
    # Параметризованные типы пишутся через точку — «СправочникСсылка.Товары»
    # это один тип, а не тип и его член. Поэтому корень ищется с самого
    # длинного начала выражения, и только оно может быть угадано по части имени.
    consumed, current, candidates = 1, None, []
    for length in range(min(3, len(segments)), 0, -1):
        head = ".".join(segment.split("(")[0] for segment in segments[:length])
        current, candidates = model.find_type(head, strict=length > 1)
        if current is not None:
            consumed = length
            break
    head = ".".join(segment.split("(")[0] for segment in segments[:consumed])
    if current is None:
        print(f"Тип {head!r} в справке не найден.")
        if candidates:
            print("  Возможно, имелось в виду:")
            for candidate in candidates:
                print(f"    {candidate}")
        return 1
    if current.lower() != head.lower():
        print(f"{head} → {current}")
    trail = current
    for segment in segments[consumed:]:
        called = "(" in segment
        name = segment.split("(")[0]
        symbol, near = model.member(current, name)
        if symbol is None:
            print(f"\nУ типа {current} нет члена {name!r}.")
            if near:
                print("  Похожие:", ", ".join(near))
            else:
                owners = [
                    item.get("owner_ru")
                    for item in model.symbols
                    if (item.get("name_ru") or "").lower() == name.lower() and item.get("owner_ru")
                ]
                if owners:
                    print(f"  {name} есть у других типов, например: {', '.join(sorted(set(owners))[:8])}")
                    print("  Значит цепочка оборвалась раньше — проверьте предыдущий шаг.")
            return 1
        if symbol["kind"] == "method" and not called:
            print(f"  ({name} — метод, не свойство: {signature(symbol)})")
        following = value_types(symbol)
        trail = f"{trail}.{symbol['name_ru']}"
        if not following:
            print(f"\n{trail} — {symbol['kind']}, дальше по нему идти некуда.")
            print_member(symbol, full=True)
            return 0
        if len(following) > 1:
            print(f"  {symbol['name_ru']} → {', '.join(following)} (иду по первому)")
        resolved, candidates = model.find_type(following[0])
        if resolved is None:
            print(f"\n{trail} имеет тип {following[0]}, но его страницы в справке нет.")
            return 1
        current = resolved
    if trail != current:
        print(f"\n{trail} — тип {current}")
    else:
        print()
    return show_type(model, current, member_filter)


def split_path(expression: str) -> list[str]:
    """Разбить по точкам, не трогая точки внутри скобок и угловых скобок."""
    parts: list[str] = []
    depth = 0
    buffer: list[str] = []
    for symbol in expression:
        if symbol in "(<":
            depth += 1
        elif symbol in ")>":
            depth = max(0, depth - 1)
        if symbol == "." and depth == 0:
            parts.append("".join(buffer))
            buffer = []
            continue
        buffer.append(symbol)
    parts.append("".join(buffer))
    return parts


def whose(model: Model, name: str) -> int:
    folded = name.lower()
    owners = sorted(
        {
            (symbol.get("owner_ru") or "—", symbol["kind"], signature(symbol))
            for symbol in model.symbols
            if (symbol.get("name_ru") or "").lower() == folded
        }
    )
    if not owners:
        print(f"Члена {name!r} в справке нет.")
        return 1
    print(f"{name} встречается у {len(owners)} типов:")
    for owner, kind, sign in owners:
        print(f"  {owner:55} {kind:9} {sign}")
    return 0


def show_query_table(path: Path, name: str) -> int:
    """Показать таблицу языка запросов: поля с типами и параметры."""
    if not path.exists():
        print(f"Нет каталога таблиц {path} — прогоните scripts/its_rebuild.py", file=sys.stderr)
        return 2
    catalog = json.loads(path.read_text(encoding="utf-8"))
    folded = name.lower()
    # Имя таблицы в справке параметризовано: «РегистрНакопления.<Имя регистра
    # накопления>.Остатки». Спрашивают обычно короче, поэтому сверяем по
    # словам имени, а не по строке целиком.
    words = [word for word in re.split(r"[.\s]+", folded) if word]
    hits = []
    for table in catalog["tables"]:
        haystack = table["name_ru"].lower()
        if all(word in haystack for word in words):
            hits.append(table)
    if not hits:
        print(f"Таблицы {name!r} в справке нет.")
        return 1
    for table in hits:
        kind = "виртуальная" if table.get("virtual") else "реальная"
        since = f", с версии {table['since_version']}" if table.get("since_version") else ""
        print(f"\n{table['name_ru']} — {kind} таблица{since}")
        if table.get("parameters"):
            print(f"  параметры ({len(table['parameters'])}):")
            for parameter in table["parameters"]:
                need = "обязательный" if parameter.get("required") else "необязательный"
                types = ", ".join(parameter.get("types") or []) or "тип не указан"
                print(f"    {parameter['name_ru']} ({need}) — {types}")
        print(f"  поля ({len(table['fields'])}):")
        for field in table["fields"]:
            types = ", ".join(field.get("types") or []) or "—"
            print(f"    {field['name_ru']}: {types}")
    return 0


def show_syntax(model: Model, name: str) -> int:
    """Показать конструкцию языка или языка запросов."""
    folded = name.lower()
    hits = [
        symbol
        for symbol in model.symbols
        if symbol["kind"] in ("query_syntax", "language_syntax") and folded in (symbol.get("name_ru") or "").lower()
    ]
    if not hits:
        print(f"Конструкции {name!r} в справке нет.")
        return 1
    for symbol in hits[:5]:
        kind = "язык запросов" if symbol["kind"] == "query_syntax" else "встроенный язык"
        print(f"\n{symbol['name_ru']} — {kind}")
        for syntax in symbol.get("syntaxes") or []:
            print(f"  синтаксис: {syntax}")
        for parameter in symbol.get("parameters") or []:
            print(f"    <{parameter['name']}> — {parameter['description']}")
        returned = symbol.get("return")
        if returned and returned.get("description"):
            print(f"  возврат: {returned['description']}")
        description = (symbol.get("description") or "").splitlines()
        if description:
            print("  " + " ".join(description)[:400])
        if symbol.get("example"):
            print("  пример:")
            for line in symbol["example"].splitlines()[:6]:
                print(f"    {line}")
    if len(hits) > 5:
        print(f"\n… и ещё {len(hits) - 5} подходящих конструкций")
    return 0


def search(model: Model, fragment: str) -> int:
    folded = fragment.lower()
    hits = sorted({symbol["name_ru"] for key, symbol in model.types.items() if folded in key})
    if not hits:
        print(f"Типов с {fragment!r} в имени нет.")
        return 1
    print(f"Типов с {fragment!r} в имени: {len(hits)}")
    for hit in hits:
        print(f"  {hit}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("expression", nargs="?", help="выражение вида Тип.Свойство.Свойство")
    parser.add_argument("--member", help="показать один член подробно")
    parser.add_argument("--whose", help="у каких типов есть член с таким именем")
    parser.add_argument("--search", help="искать тип по части имени")
    parser.add_argument("--table", help="таблица языка запросов: поля и параметры")
    parser.add_argument("--syntax", help="конструкция языка или языка запросов")
    parser.add_argument("--tables-catalog", type=Path, default=DEFAULT_TABLES)
    parser.add_argument("--model", type=Path, default=DEFAULT_MODEL)
    args = parser.parse_args()

    if not args.model.exists():
        print(f"Нет модели справки: {args.model}", file=sys.stderr)
        return 2
    model = Model(args.model)

    if args.table:
        return show_query_table(args.tables_catalog, args.table)
    if args.syntax:
        return show_syntax(model, args.syntax)
    if args.whose:
        return whose(model, args.whose)
    if args.search:
        return search(model, args.search)
    if not args.expression:
        parser.print_help()
        return 2
    return walk(model, args.expression, args.member)


if __name__ == "__main__":
    raise SystemExit(main())
