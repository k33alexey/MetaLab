#!/usr/bin/env python3
"""Разбор внутреннего формата 1С: вложенные списки в фигурных скобках.

В таком виде платформа хранит служебные файлы справки — `.st` с именами
элементов и `__categories__` с контекстами доступности. Формат простой:
список в фигурных скобках, элементы через запятую, строки в двойных кавычках
с удвоением кавычки внутри, числа без кавычек.
"""

from __future__ import annotations


class BracedError(ValueError):
    pass


def parse(text: str) -> list:
    """Вернуть первый список верхнего уровня."""
    position = 0
    text = text.lstrip("﻿")
    length = len(text)

    def skip_space() -> None:
        nonlocal position
        while position < length and text[position] in " \t\r\n":
            position += 1

    def read_string() -> str:
        nonlocal position
        position += 1  # открывающая кавычка
        parts: list[str] = []
        while position < length:
            symbol = text[position]
            if symbol == '"':
                if position + 1 < length and text[position + 1] == '"':
                    parts.append('"')
                    position += 2
                    continue
                position += 1
                return "".join(parts)
            parts.append(symbol)
            position += 1
        raise BracedError("строка не закрыта")

    def read_atom() -> str | int | float:
        nonlocal position
        start = position
        while position < length and text[position] not in ",}":
            position += 1
        raw = text[start:position].strip()
        try:
            return int(raw)
        except ValueError:
            pass
        try:
            return float(raw)
        except ValueError:
            return raw

    def read_list() -> list:
        nonlocal position
        position += 1  # открывающая скобка
        items: list = []
        while position < length:
            skip_space()
            if position >= length:
                break
            symbol = text[position]
            if symbol == "}":
                position += 1
                return items
            if symbol == ",":
                position += 1
                continue
            if symbol == "{":
                items.append(read_list())
            elif symbol == '"':
                items.append(read_string())
            else:
                items.append(read_atom())
        raise BracedError("список не закрыт")

    skip_space()
    if position >= length or text[position] != "{":
        raise BracedError("файл не начинается со списка")
    return read_list()
