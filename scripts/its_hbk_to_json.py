#!/usr/bin/env python3
"""Build a searchable JSON catalog from extracted 1C HBK HTML pages."""

from __future__ import annotations

import argparse
import json
import re
from html.parser import HTMLParser
from pathlib import Path


class PageParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self._title_depth = 0
        self._title_parts: list[str] = []
        self._text_parts: list[str] = []
        self.links: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag == "h1":
            self._title_depth += 1
        if tag in {"p", "div", "br", "hr", "h1", "h2", "h3", "li", "tr"}:
            self._text_parts.append("\n")
        if tag == "a":
            href = dict(attrs).get("href")
            if href and href.startswith("v8help://"):
                self.links.append(href)

    def handle_endtag(self, tag: str) -> None:
        if tag == "h1" and self._title_depth:
            self._title_depth -= 1
        if tag in {"p", "div", "h1", "h2", "h3", "li", "tr"}:
            self._text_parts.append("\n")

    def handle_data(self, data: str) -> None:
        self._text_parts.append(data)
        if self._title_depth:
            self._title_parts.append(data)

    @property
    def title(self) -> str:
        return normalize_text(" ".join(self._title_parts))

    @property
    def text(self) -> str:
        lines = [normalize_text(line) for line in "".join(self._text_parts).splitlines()]
        return "\n".join(line for line in lines if line)


def normalize_text(value: str) -> str:
    return re.sub(r"\s+", " ", value.replace("\ufeff", " ")).strip()


def split_title(title: str) -> tuple[str, str | None]:
    match = re.match(r"^(.*?)\s*\(([^()]*)\)\s*$", title)
    if not match:
        return title, None
    return match.group(1).strip(), match.group(2).strip()


def page_kind(relative_path: Path) -> str:
    parts = set(relative_path.parts)
    for directory, kind in (
        ("methods", "method"),
        ("properties", "property"),
        ("events", "event"),
        ("constructors", "constructor"),
    ):
        if directory in parts:
            return kind
    return "page"


def build_catalog(
    html_root: Path,
    output: Path,
    platform_version: str,
    source: str,
) -> int:
    pages: list[dict[str, object]] = []

    for html_path in sorted(path for path in html_root.rglob("*") if path.is_file()):
        content = html_path.read_text(encoding="utf-8-sig", errors="replace")
        if "<html" not in content[:1024].lower():
            continue
        parser = PageParser()
        parser.feed(content)
        relative_path = html_path.relative_to(html_root)
        title_ru, title_en = split_title(parser.title)
        pages.append(
            {
                "id": relative_path.with_suffix("").as_posix(),
                "path": relative_path.as_posix(),
                "kind": page_kind(relative_path),
                "title": parser.title,
                "title_ru": title_ru,
                "title_en": title_en,
                "text": parser.text,
                "links": sorted(set(parser.links)),
            }
        )

    catalog = {
        "schema_version": 1,
        "platform_version": platform_version,
        "language": "ru",
        "source": source,
        "page_count": len(pages),
        "pages": pages,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(catalog, ensure_ascii=False, separators=(",", ":")),
        encoding="utf-8",
    )
    return len(pages)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("html_root", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--platform-version", required=True)
    parser.add_argument("--source", required=True)
    args = parser.parse_args()

    count = build_catalog(
        args.html_root,
        args.output,
        args.platform_version,
        args.source,
    )
    print(f"Created {args.output} with {count} HTML pages")


if __name__ == "__main__":
    main()
