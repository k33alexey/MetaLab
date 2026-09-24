#!/usr/bin/env python3
"""Сверить разобранную справку с реальным кодом конфигурации.

Выборочные вопросы проверяют модель там, где мы и так подозреваем пробел.
Реальные модули проверяют её там, где мы не подозреваем ничего: берём всё, что
код действительно вызывает и создаёт, и смотрим, знает ли об этом модель.

Три проверки:

* вызовы глобального контекста — имя, вызванное без точки и не объявленное в
  самой конфигурации, обязано быть в глобальном контексте платформы;
* «Новый <Тип>» — тип, создаваемый кодом, обязан быть в справке;
* цепочки через точку от известного корня — каждый шаг обязан находиться
  членом своего типа.

Использование:
    python3 scripts/bsl_check_modules.py [--modules docs/materials/demo-base] [--limit 40]
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

DEFAULT_MODEL = Path("docs/materials/its/8.3.27.2342/bsl-bridge/bsl-bridge.json")
DEFAULT_MODULES = Path("docs/materials/demo-base")

# Ключевые слова языка: они выглядят как вызов, но вызовом не являются.
KEYWORDS = {
    "если", "тогда", "иначе", "иначеесли", "конецесли", "для", "каждого", "из", "по", "цикл",
    "конеццикла", "пока", "процедура", "конецпроцедуры", "функция", "конецфункции", "возврат",
    "перем", "новый", "и", "или", "не", "истина", "ложь", "неопределено", "null", "попытка",
    "исключение", "конецпопытки", "вызватьисключение", "прервать", "продолжить", "экспорт",
    "знач", "выполнить", "асинх", "ждать", "добавитьобработчик", "удалитьобработчик",
    "если;", "тогда;", "перейти",
}

DECLARATION = re.compile(r"^\s*(?:&[^\n]*\n\s*)?(?:Асинх\s+)?(?:Процедура|Функция)\s+([А-Яа-яЁёA-Za-z_][\wА-Яа-яЁё]*)", re.IGNORECASE | re.MULTILINE)
CALL = re.compile(r"(?<![.\w])([А-Яа-яЁёA-Za-z_][\wА-Яа-яЁё]*)\s*\(")
NEW_TYPE = re.compile(r"(?<![\w.])Новый\s+([А-Яа-яЁёA-Za-z_][\wА-Яа-яЁё]*)", re.IGNORECASE)
CHAIN = re.compile(r"(?<![\w.])([А-Яа-яЁёA-Za-z_][\wА-Яа-яЁё]*(?:\.[А-Яа-яЁёA-Za-z_][\wА-Яа-яЁё]*)+)")


def strip_code(text: str) -> str:
    """Убрать строковые литералы и комментарии — в них бывает что угодно.

    Разбор посимвольный, а не регулярным выражением: во встроенном языке
    кавычка внутри строки удваивается, и выражение с чередованием спаривает
    кавычки не те. На тексте вида НСтр("ru = 'Установить новый пароль?'")
    это оставляло русскую прозу снаружи строки, и «пароль» попадал в отчёт
    выдуманным типом.
    """
    result: list[str] = []
    position, length = 0, len(text)
    while position < length:
        symbol = text[position]
        if symbol == '"':
            position += 1
            while position < length:
                if text[position] == '"':
                    if position + 1 < length and text[position + 1] == '"':
                        position += 2
                        continue
                    position += 1
                    break
                position += 1
            result.append('""')
            continue
        if symbol == "/" and position + 1 < length and text[position + 1] == "/":
            while position < length and text[position] != "\n":
                position += 1
            continue
        result.append(symbol)
        position += 1
    return "".join(result)


class Model:
    def __init__(self, path: Path) -> None:
        data = json.loads(path.read_text(encoding="utf-8"))
        self.members: dict[str, dict[str, dict]] = defaultdict(dict)
        self.types: set[str] = set()
        self.pages: dict[str, dict] = {}
        self.globals: set[str] = set()
        for symbol in data["symbols"]:
            name = (symbol.get("name_ru") or "").lower()
            owner = (symbol.get("owner_ru") or "").lower()
            if owner and name:
                self.members[owner][name] = symbol
            if symbol["kind"] in ("type", "global_context") and name:
                self.types.add(name)
                # Код пишут обоими написаниями: «Новый COMОбъект» и
                # «Новый COMObject» — это один тип, и сверять надо оба имени.
                if symbol.get("name_en"):
                    self.types.add(symbol["name_en"].lower())
                self.pages[name] = symbol
        self.globals = set(self.members.get("глобальный контекст", {}))
        # В модуле формы и в модуле менеджера доступен контекст самого объекта:
        # РеквизитФормыВЗначение() пишут без точки, но это метод формы, а не
        # глобальная функция. Без этого сверка объявит пробелом то, что в
        # справке есть.
        self.implicit: dict[str, set[str]] = {}
        for owner in list(self.members):
            # Контекст модуля: в модуле формы, объекта, набора записей и
            # менеджера собственные методы пишутся без точки. Без этого сверка
            # объявит пробелом то, что в справке есть.
            if (
                owner.startswith("формаклиентскогоприложения")
                or owner == "форма"
                or "менеджер" in owner
                or "объект." in owner
                or owner.endswith("объект")
                or "наборзаписей" in owner
                or owner.startswith("расширение формы")
                or owner.startswith("расширение управляемой формы")
            ):
                self.implicit[owner] = set(self.members[owner])

    def implicit_owner(self, name: str) -> str | None:
        folded = name.lower()
        for owner, names in self.implicit.items():
            if folded in names:
                return owner
        return None

    def type_exists(self, name: str) -> bool:
        folded = name.lower()
        if folded in self.types or folded in self.members:
            return True
        return any(key.startswith(folded + ".<") for key in self.types)

    def member_of(self, type_name: str, member: str) -> dict | None:
        return self.members.get(type_name.lower(), {}).get(member.lower())

    def collection_element(self, type_name: str) -> str | None:
        """Тип элемента коллекции менеджеров.

        Справка описывает их одинаково: «СправочникиМенеджер предоставляет
        доступ к значениям типа СправочникМенеджер.<Имя справочника>».
        Имя члена здесь — имя объекта прикладной конфигурации, а не справки,
        поэтому проверять его по справке бессмысленно: она его знать не может.
        """
        description = ""
        for symbol in self.members.get(type_name.lower(), {}).values():
            break
        page = self.pages.get(type_name.lower())
        if page:
            description = page.get("description") or ""
        match = re.search(r"значениям типа ([А-Яа-яЁё]+\.<[^>]+>)", description)
        return match.group(1) if match else None

    def following_type(self, symbol: dict) -> str | None:
        """Тип, которым продолжается выражение после этого члена."""
        if symbol["kind"] == "method":
            raw = (symbol.get("return") or {}).get("types") or []
        else:
            raw = symbol.get("value_types") or []
        for item in raw:
            for part in re.split(r"\s+или\s+|,", item):
                part = part.strip()
                if part and part.lower() not in {"неопределено", "null"} and self.type_exists(part):
                    return part
        return None


def drop_constructors(text: str) -> str:
    """Убрать «Новый <Тип>» перед поиском вызовов.

    Имя после «Новый» — конструктор типа, а не вызов процедуры. Отсекать его
    заглядыванием назад в самом выражении вызова ненадёжно: между словом и
    именем бывает перевод строки, и тогда «Новый Структура(...)» снова
    считается неизвестной функцией.
    """
    return NEW_TYPE.sub("Новый", text)


def read_modules(root: Path) -> list[tuple[Path, str]]:
    result = []
    for path in sorted(root.rglob("*.bsl")):
        try:
            result.append((path, strip_code(path.read_text(encoding="utf-8", errors="ignore"))))
        except OSError:
            continue
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--model", type=Path, default=DEFAULT_MODEL)
    parser.add_argument("--modules", type=Path, default=DEFAULT_MODULES)
    parser.add_argument("--limit", type=int, default=30)
    args = parser.parse_args()

    if not args.model.exists() or not args.modules.exists():
        print("Нет модели или модулей", file=sys.stderr)
        return 2

    model = Model(args.model)
    modules = read_modules(args.modules)
    print(f"модулей: {len(modules)}, строк: {sum(text.count(chr(10)) for _, text in modules)}")

    declared: set[str] = set()
    for _, text in modules:
        declared.update(name.lower() for name in DECLARATION.findall(text))
    print(f"процедур и функций объявлено в конфигурации: {len(declared)}")

    calls, unknown_calls, implicit_calls = Counter(), Counter(), Counter()
    new_types, unknown_types = Counter(), Counter()
    for _, text in modules:
        for name in CALL.findall(drop_constructors(text)):
            folded = name.lower()
            if folded in KEYWORDS:
                continue
            calls[folded] += 1
            if folded in declared or folded in model.globals:
                continue
            owner = model.implicit_owner(folded)
            if owner:
                implicit_calls[owner] += 1
                continue
            unknown_calls[name] += 1
        for name in NEW_TYPE.findall(text):
            # Имя типа во встроенном языке всегда с заглавной: строчное слово
            # после «Новый» — это проза, а не конструктор.
            if name[:1].islower():
                continue
            new_types[name] += 1
            if not model.type_exists(name):
                unknown_types[name] += 1

    print(f"\n=== вызовы без точки: {sum(calls.values())} обращений, {len(calls)} имён")
    print(f"    не объявлены в конфигурации и не найдены в глобальном контексте: {len(unknown_calls)} имён, {sum(unknown_calls.values())} обращений")
    for name, count in unknown_calls.most_common(args.limit):
        print(f"      {name}: {count}")

    print(f"\n    из них разрешились контекстом модуля (форма, менеджер): {sum(implicit_calls.values())} обращений")
    for owner, count in implicit_calls.most_common(5):
        print(f"      {owner}: {count}")

    # Цепочки через точку от корня, тип которого известен: глобальный контекст
    # даёт Справочники, Документы, РегистрыСведений, ПараметрыСеанса и прочее,
    # и от них дерево раскручивается без догадок о типах переменных.
    chains = Counter()
    broken: Counter = Counter()
    applied: Counter = Counter()
    checked = steps_ok = 0
    for _, text in modules:
        for chain in CHAIN.findall(text):
            segments = [segment for segment in chain.split(".") if segment]
            head = segments[0].lower()
            if head not in model.globals:
                continue
            chains[chain] += 1
    for chain, count in chains.items():
        segments = chain.split(".")
        symbol = model.members["глобальный контекст"][segments[0].lower()]
        current = model.following_type(symbol)
        checked += 1
        for index, segment in enumerate(segments[1:], start=1):
            if current is None:
                broken[f"{'.'.join(segments[:index])} — тип не определён"] += count
                break
            member = model.member_of(current, segment)
            if member is None:
                element = model.collection_element(current)
                if element:
                    # Имя объекта конфигурации: справка его знать не может,
                    # и это не обрыв цепочки, а переход к прикладному типу.
                    applied[current] += count
                    current = element
                    steps_ok += 1
                    continue
                broken[f"{current}.{segment} — члена нет"] += count
                break
            steps_ok += 1
            current = model.following_type(member)
        else:
            continue

    print(f"\n=== цепочки через точку от известного корня: {checked} разных, {sum(chains.values())} обращений")
    print(f"    шагов пройдено: {steps_ok}, цепочек оборвалось: {len(broken)}")
    print(f"    шагов, ушедших в прикладной тип конфигурации: {sum(applied.values())}")
    for reason, count in broken.most_common(args.limit):
        print(f"      {reason}: {count}")

    print(f"\n=== «Новый <Тип>»: {sum(new_types.values())} обращений, {len(new_types)} типов")
    print(f"    типов, которых нет в справке: {len(unknown_types)}")
    for name, count in unknown_types.most_common(args.limit):
        print(f"      {name}: {count}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
