#!/usr/bin/env python3
"""Собрать docs/PROGRESS.md и docs/requirements/progress-table.rst из docs/BLOCKS.md.

Карта блоков — единственный источник плана и галочек. Таблица прогресса
целиком выводится из неё, поэтому расходиться им не с чем: любое изменение
в карте попадает сюда следующим прогоном.

Ручные врезки. Кое-что прогрессом является, но в карте не живёт: строки
прогонов отчёта импорта и заметки по блоку, который ещё не разбит на
итерации. Такие куски лежат между маркерами

    <!-- manual:имя -->
    ...
    <!-- /manual:имя -->

и переносятся из прежнего PROGRESS.md слово в слово. Всё остальное между
прогонами перезаписывается, править его руками бесполезно — правится карта.

Прогонять после каждой отметки итерации в карте — см. «Правило завершения
итерации» в CLAUDE.md.

Использование:
    python3 scripts/progress_table.py           # перезаписать оба файла
    python3 scripts/progress_table.py --check   # не писать, сверить (код 1 при расхождении)
    python3 scripts/progress_table.py --stdout  # напечатать, ничего не трогая
"""

from __future__ import annotations

import argparse
import datetime
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
BLOCKS = ROOT / "docs" / "BLOCKS.md"
PROGRESS = ROOT / "docs" / "PROGRESS.md"
TABLE = ROOT / "docs" / "requirements" / "progress-table.rst"

# Короткие имена блоков для сводной таблички: в карте у блока полное имя, а
# табличка стоит рядом с требованиями и читается одним взглядом, поэтому имя
# в ней сокращено. Сокращения заданы явно — вывести их из полного имени
# нельзя, а угадывание давало бы разные имена от прогона к прогону.
SHORT_NAMES = {
    1: "Расчистка",
    2: "Модель метаданных",
    3: "Миграция",
    4: "Импорт",
    5: "Язык",
    6: "Запросы",
    7: "Формы: каркас",
    8: "Формы: оформление",
    9: "Прикладные объекты",
    10: "Права и ограничения",
    11: "Табличный документ",
    12: "Компоновка данных",
    13: "Редакторы компоновки",
    14: "Данные приложения",
    15: "Задания и эксплуатация",
    16: "Импорт данных",
    17: "Интеграции",
    18: "Интерфейс и локализация",
    19: "Качество и выпуск",
    20: "Оборудование",
    21: "Инструменты разработчика",
}

# Состояние итерации: как отмечена в карте -> как показывается в таблице.
# Отметок ровно две, как в карте: галочка ставится после подтверждения
# владельцем, и до него итерация стоит неотмеченной.
MARKS = {
    "x": "✅",
    " ": "⬜",
}

MANUAL_RE = re.compile(
    r"<!-- manual:(?P<name>[a-z0-9-]+) -->\n(?P<body>.*?)<!-- /manual:(?P=name) -->",
    re.DOTALL,
)


class Block:
    def __init__(self, number: int, title: str) -> None:
        self.number = number
        self.title = title
        self.iterations: list[tuple[str, str]] = []
        self.paragraphs: list[str] = []
        self.depends = ""
        self.gives = ""
        self.closed_on = ""

    @property
    def done(self) -> int:
        return sum(1 for mark, _ in self.iterations if mark == "x")

    @property
    def status(self) -> str:
        if self.closed_on:
            return f"✅ закрыт {self.closed_on}"
        if self.iterations and self.done:
            return "🔄 в работе"
        return "⬜"


def unwrap(lines: list[str]) -> str:
    """Склеить перенесённые строки абзаца в одну."""
    return " ".join(line.strip() for line in lines if line.strip())


def parse(text: str) -> tuple[list[Block], list[str], str, str]:
    lines = text.splitlines()
    blocks: list[Block] = []
    current: Block | None = None
    later: list[str] = []
    in_later = False
    pending: list[str] = []  # накопленный абзац
    pending_iteration: list[str] | None = None

    version = ""
    date = ""
    match = re.search(r"^Версия: (.+)$", text, re.M)
    if match:
        version = match.group(1).strip()
    match = re.search(r"^Дата: (.+)$", text, re.M)
    if match:
        date = match.group(1).strip()

    def flush_iteration() -> None:
        nonlocal pending_iteration
        if pending_iteration is not None and current is not None:
            mark = pending_iteration[0]
            current.iterations.append((mark, unwrap(pending_iteration[1:])))
        pending_iteration = None

    def flush_paragraph() -> None:
        nonlocal pending
        if pending and current is not None:
            current.paragraphs.append(unwrap(pending))
        pending = []

    for line in lines:
        heading = re.match(r"^## Блок (\d+)\.\s*(.+?)\s*$", line)
        if heading:
            flush_iteration()
            flush_paragraph()
            in_later = False
            current = Block(int(heading.group(1)), heading.group(2))
            blocks.append(current)
            continue

        if line.startswith("## "):
            flush_iteration()
            flush_paragraph()
            current = None
            in_later = line.startswith("## После ")
            continue

        if in_later:
            # Раздел переносится как есть, вместе с разбивкой на абзацы.
            if line.strip() or later:
                later.append(line.rstrip())
            continue

        if current is None:
            continue

        bullet = re.match(r"^- \[(.)\] (.*)$", line)
        if bullet:
            flush_iteration()
            flush_paragraph()
            pending_iteration = [bullet.group(1).lower(), bullet.group(2)]
            continue

        # Продолжение строки итерации — отступ в два пробела, пустых строк нет.
        if pending_iteration is not None:
            if line.startswith("  ") and line.strip():
                pending_iteration.append(line)
                continue
            flush_iteration()

        if not line.strip():
            flush_paragraph()
            continue

        pending.append(line)

    flush_iteration()
    flush_paragraph()

    # «Зависит от» и «Даёт» — отдельный абзац в конце блока.
    for block in blocks:
        rest: list[str] = []
        for paragraph in block.paragraphs:
            if paragraph.startswith("*Зависит от:*"):
                body = paragraph
                gives = re.search(r"\*Даёт:\*\s*(.+)$", body)
                if gives:
                    block.gives = gives.group(1).strip().rstrip(".")
                    body = body[: gives.start()]
                block.depends = (
                    body.replace("*Зависит от:*", "").strip().rstrip(".").strip()
                )
                continue
            if paragraph.startswith("Итерации:"):
                continue
            rest.append(paragraph)
        block.paragraphs = rest

    # Даты закрытия — из раздела «Текущее состояние».
    by_number = {block.number: block for block in blocks}
    for number, when in re.findall(r"Блок (\d+) закрыт (\d{2}\.\d{2}\.\d{4})", text):
        block = by_number.get(int(number))
        if block:
            block.closed_on = when

    return blocks, later, version, date


def manual_regions(path: pathlib.Path) -> dict[str, str]:
    if not path.exists():
        return {}
    return {
        match.group("name"): match.group("body")
        for match in MANUAL_RE.finditer(path.read_text(encoding="utf-8"))
    }


def region(name: str, saved: dict[str, str], default: str) -> list[str]:
    body = saved.get(name, default)
    if not body.endswith("\n"):
        body += "\n"
    return [f"<!-- manual:{name} -->", body.rstrip("\n"), f"<!-- /manual:{name} -->"]


def cell(text: str) -> str:
    return text.replace("|", "\\|").strip()


def render(
    blocks: list[Block],
    later: list[str],
    version: str,
    date: str,
    saved: dict[str, str],
) -> str:
    today = datetime.date.today().strftime("%d.%m.%Y")
    out: list[str] = []
    add = out.append

    add("# MetaLab — таблица прогресса")
    add("")
    add(
        f"Собрано {today} из `docs/BLOCKS.md` "
        f"(версия {version or '—'}, дата {date or '—'}) "
        "скриптом `scripts/progress_table.py`."
    )
    add("")
    add(
        "**Файл генерируемый — правится карта, а не он.** Требования — в "
        "`docs/requirements/`, план и галочки итераций — в [docs/BLOCKS.md](BLOCKS.md). "
        "Здесь то же самое в таблицах, чтобы одним взглядом видеть пройденное и "
        "оставшееся. Исключение — куски между `<!-- manual:имя -->`: они "
        "переносятся при сборке и заполняются руками."
    )
    add("")
    add("Статус: ✅ закрыто · 🔄 в работе · ⬜ не начато.")
    add("")

    add("## Сводка по блокам")
    add("")
    add("| № | Блок | Статус | Итерации | Зависит от | Что даёт |")
    add("|---|------|--------|----------|------------|----------|")
    for block in blocks:
        if block.iterations:
            counter = f"{block.done} / {len(block.iterations)}"
        else:
            counter = "не разбит"
        add(
            f"| {block.number} | [{cell(block.title)}](#блок-{block.number}) "
            f"| {block.status} | {counter} | {cell(block.depends) or '—'} "
            f"| {cell(block.gives) or '—'} |"
        )
    add("")
    add(
        "«Не разбит» — итерации блока ещё не выписаны в карте: они выписываются "
        "перед началом блока, а не заранее. Состав блока при этом известен и "
        "приведён ниже."
    )
    add("")

    add("## Ход по прибору")
    add("")
    add(
        "Ход меряется отчётом импорта конфигураций, а не числом закрытых задач "
        "(`docs/requirements/CONFORMANCE.md`). Прогон — после каждого блока."
    )
    add("")
    out.extend(
        region(
            "runs",
            saved,
            "| Прогон | После блока | Перенесено и работает | "
            "Перенесено, не реализовано | Перенесено с потерей смысла |\n"
            "|--------|-------------|----------------------|"
            "----------------------------|-----------------------------|\n"
            "| — | первый прогон возможен с блока 4 | — | — | — |",
        )
    )
    add("")
    add(
        "Новая строка в третьей категории блокирует закрытие блока — с первого "
        "же прогона. Движения строк из второй категории в первую ждём с блока 9."
    )
    add("")

    for block in blocks:
        add(f'<a id="блок-{block.number}"></a>')
        add("")
        head = f"## Блок {block.number}. {block.title}"
        if block.closed_on:
            head += f" — закрыт {block.closed_on}"
        elif block.done:
            head += " — в работе"
        add(head)
        add("")
        for paragraph in block.paragraphs:
            add(paragraph)
            add("")
        if block.iterations:
            add("| № | Итерация | Статус |")
            add("|---|----------|--------|")
            for index, (mark, text) in enumerate(block.iterations, start=1):
                icon = MARKS.get(mark, "❓")
                add(
                    f"| {block.number}.{index} | {cell(text.rstrip('.;'))} | {icon} |"
                )
            add("")
        else:
            add("Итерации не выписаны. Заметки по составу — врезка ниже.")
            add("")
        out.extend(region(f"block-{block.number}", saved, "_пусто_"))
        add("")

    add("## За рамками 1.0.1")
    add("")
    while later and not later[-1]:
        later.pop()
    out.extend(later)
    add("")

    add("## Как вести таблицу")
    add("")
    add(
        "1. Итерация закончена и подтверждена владельцем → галочка ставится в "
        "`docs/BLOCKS.md`, затем `python3 scripts/progress_table.py`. Один "
        "прогон пересобирает и этот файл, и короткую таблицу "
        "`docs/requirements/progress-table.rst`."
    )
    add(
        "2. Блок закрыт → дата закрытия пишется в раздел «Текущее состояние» "
        "карты, строка прогона — во врезку `manual:runs`."
    )
    add(
        "3. Блок разбит на итерации → строки итераций пишутся в карте, врезка "
        "`manual:block-N` очищается."
    )
    add("")
    return "\n".join(out).rstrip("\n") + "\n"


def current_iteration(blocks: list[Block]) -> tuple[Block, int, str] | None:
    """Первая неотмеченная итерация — та, которая делается сейчас.

    Галочка ставится после подтверждения владельцем, поэтому работа идёт над
    итерацией, следующей за последней отмеченной.
    """
    for block in blocks:
        for index, (mark, text) in enumerate(block.iterations, start=1):
            if mark != "x":
                return block, index, text
    return None


def short_iteration(text: str) -> str:
    """Оставить от строки итерации её название, без перечня свойств."""
    for separator in (" — ", ": ", ", "):
        head = text.split(separator, 1)[0]
        if head != text:
            text = head
    return text.strip().rstrip(".;")


def render_table(blocks: list[Block]) -> str:
    """Собрать сводную табличку рядом с требованиями.

    Блоки в два столбца: так они видны целиком, без прокрутки, а это и есть
    назначение таблички. При нечётном числе блоков правый столбец короче
    левого на одну строку, и последняя его ячейка остаётся пустой.
    """
    numbered = {block.number: block for block in blocks}
    half = (len(SHORT_NAMES) + 1) // 2

    def name(number: int) -> str:
        if number not in numbered and number not in SHORT_NAMES:
            return ""
        return f"{number}. {SHORT_NAMES.get(number, numbered[number].title)}"

    def counts(number: int) -> tuple[str, str]:
        block = numbered.get(number)
        if block is None:
            return ("—", "—") if number in SHORT_NAMES else ("", "")
        if not block.iterations:
            return "—", "—"
        return str(len(block.iterations)), str(block.done)

    headers = ["Блок", "Всего", "Закрыто", "", "Блок", "Всего", "Закрыто"]
    rows = []
    for offset in range(half):
        left, right = offset + 1, offset + 1 + half
        total_left, done_left = counts(left)
        total_right, done_right = counts(right)
        rows.append([name(left), total_left, done_left, "",
                     name(right), total_right, done_right])

    widths = [max(len(header), *(len(row[column]) for row in rows)) + 2
              for column, header in enumerate(headers)]
    # Столбец между половинами таблицы пуст: он разводит их глазом, поэтому
    # его ширина задана, а не выведена из содержимого.
    widths[3] = 5

    def line(left: str, middle: str, right: str) -> str:
        return "  " + left + middle.join("─" * width for width in widths) + right

    def row(cells: list[str], centered: bool = False) -> str:
        parts = []
        for cell_text, width in zip(cells, widths):
            if centered:
                left = (width - len(cell_text)) // 2
                parts.append(" " * left + cell_text + " " * (width - left - len(cell_text)))
            else:
                parts.append(f" {cell_text}".ljust(width))
        return "  │" + "│".join(parts) + "│"

    out = [line("┌", "┬", "┐"), row(headers, centered=True), line("├", "┼", "┤")]
    for index, cells in enumerate(rows):
        if index:
            out.append(line("├", "┼", "┤"))
        out.append(row(cells))
    out.append(line("└", "┴", "┘"))

    out.append("")
    current = current_iteration(blocks)
    if current is None:
        out.append("  Все итерации карты закрыты.")
    else:
        block, index, text = current
        out.append(
            f"  Текущая итерация — {block.number}. "
            f"{SHORT_NAMES.get(block.number, block.title)}, "
            f"{index} из {len(block.iterations)}: {short_iteration(text)}."
        )
    return "\n".join(out) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="сверить, не записывая")
    parser.add_argument("--stdout", action="store_true", help="напечатать результат")
    args = parser.parse_args()

    blocks, later, version, date = parse(BLOCKS.read_text(encoding="utf-8"))
    if not blocks:
        print("в docs/BLOCKS.md не нашлось ни одного блока", file=sys.stderr)
        return 2
    text = render(blocks, later, version, date, manual_regions(PROGRESS))
    table = render_table(blocks)

    if args.stdout:
        sys.stdout.write(text)
        sys.stdout.write("\n" + table)
        return 0
    if args.check:
        failed = False
        for path, expected in ((PROGRESS, text), (TABLE, table)):
            current = path.read_text(encoding="utf-8") if path.exists() else ""
            if current != expected:
                print(
                    f"{path.relative_to(ROOT)} разошёлся с картой — "
                    "прогоните python3 scripts/progress_table.py",
                    file=sys.stderr,
                )
                failed = True
        if failed:
            return 1
        print("docs/PROGRESS.md и docs/requirements/progress-table.rst соответствуют карте")
        return 0

    PROGRESS.write_text(text, encoding="utf-8")
    TABLE.write_text(table, encoding="utf-8")
    print(f"docs/PROGRESS.md и docs/requirements/progress-table.rst собраны: блоков {len(blocks)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
