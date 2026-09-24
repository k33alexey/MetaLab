#!/usr/bin/env python3
"""Build EPUB and CHM books from the extracted Russian 1C help."""

from __future__ import annotations

import argparse
import hashlib
import html
from html.parser import HTMLParser
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tempfile
from urllib.parse import quote, unquote
import zipfile


TITLE = "1С:Предприятие 8.3.27.2342 — справка разработчика (исправленное издание)"
BOOK_ID = "urn:uuid:9d5c9e0b-03f2-4a2c-ae2b-832723420003"
LANGUAGE = "ru"
IMAGE_SUFFIXES = {".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg"}
VOID_TAGS = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"}
DROP_TAGS = {"script", "style"}
COLLECTION_TITLES = {
    "1cv8_ru": "1С:Предприятие",
    "config_ru": "Конфигуратор",
    "dcsui_ru": "Система компоновки данных",
    "debug_ru": "Отладка",
    "devtool_ru": "Инструменты разработчика",
    "mngbase_ru": "Управление информационной базой",
    "mngdsgn_ru": "Редакторы объектов конфигурации",
    "mngui_ru": "Управляемое приложение",
    "shclang_ru": "Общие конструкции встроенного языка",
    "shcntx_ru": "Синтакс-помощник: объекты, свойства и методы",
    "shlang_ru": "Встроенный язык",
    "shquery_ru": "Язык запросов",
}
V8HELP_COLLECTIONS = {
    "syntaxhelpercommonlanguage": "shclang_ru",
    "syntaxhelpercontext": "shcntx_ru",
    "syntaxhelperlanguage": "shlang_ru",
    "syntaxhelperqueries": "shquery_ru",
    "config": "config_ru",
}
EPUB_COLLECTIONS = (
    "1cv8_ru",
    "config_ru",
    "dcsui_ru",
    "debug_ru",
    "devtool_ru",
    "mngdsgn_ru",
    "shclang_ru",
    "shcntx_ru",
    "shlang_ru",
    "shquery_ru",
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--source",
        type=Path,
        default=Path("docs/materials/its/8.3.27.2342"),
        help="Directory containing html/ and json/",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("docs/materials/its/books"),
        help="Output directory",
    )
    parser.add_argument("--keep-work", action="store_true")
    parser.add_argument("--format", choices=("all", "epub", "chm"), default="all")
    return parser.parse_args()


def posix(path: Path | PurePosixPath | str) -> str:
    return PurePosixPath(path).as_posix()


def read_catalogs(json_dir: Path, html_dir: Path) -> dict[str, list[dict[str, str]]]:
    catalogs: dict[str, list[dict[str, str]]] = {}
    missing = 0
    for catalog_file in sorted(json_dir.glob("*.json")):
        collection = catalog_file.stem
        data = json.loads(catalog_file.read_text(encoding="utf-8-sig"))
        pages: list[dict[str, str]] = []
        for page in data.get("pages", []):
            relative = posix(PurePosixPath(collection) / page["path"])
            if not (html_dir / relative).is_file():
                missing += 1
                continue
            pages.append({"path": relative, "title": page.get("title_ru") or page.get("title") or page["path"]})
        catalogs[collection] = pages
    print(f"Catalogs: {len(catalogs)}, pages: {sum(map(len, catalogs.values()))}, missing sources: {missing}", flush=True)
    return catalogs


def epub_path(source_path: str) -> str:
    path = PurePosixPath(source_path)
    suffix = path.suffix.lower()
    if suffix in IMAGE_SUFFIXES:
        return posix(PurePosixPath("OEBPS") / "media" / path)
    if suffix in {".html", ".htm"}:
        path = path.with_suffix(".xhtml")
    else:
        path = PurePosixPath(str(path) + ".xhtml")
    return posix(PurePosixPath("OEBPS") / "text" / path)


def split_fragment(value: str) -> tuple[str, str]:
    if "#" not in value:
        return value, ""
    target, fragment = value.split("#", 1)
    return target, "#" + fragment


def resolve_v8help(value: str) -> str | None:
    match = re.match(r"(?i)^v8help://([^/]+)/?(.*)$", value)
    if not match:
        return None
    host, target = match.groups()
    collection = V8HELP_COLLECTIONS.get(host.lower())
    if collection is None:
        return None
    target, fragment = split_fragment(unquote(target))
    return posix(PurePosixPath(collection) / target) + fragment


def rewrite_link(value: str, current: str, target_mode: str, page_paths: set[str], asset_paths: set[str]) -> str:
    if not value or value.startswith(("http://", "https://", "mailto:", "javascript:", "data:")):
        return value
    if value.lower().startswith("v8help://service_book/"):
        return ""
    v8_target = resolve_v8help(value)
    if v8_target is not None:
        value = v8_target
        absolute = True
    else:
        absolute = False
    target, fragment = split_fragment(value)
    if not target:
        return fragment
    target = unquote(target).replace("\\", "/")
    if absolute:
        resolved = posix(PurePosixPath(target))
    else:
        resolved = posix(PurePosixPath(current).parent / target)
        resolved = posix(PurePosixPath(os.path.normpath(resolved)))
    if resolved not in page_paths and resolved not in asset_paths:
        return value
    if target_mode == "chm":
        destination = resolved
        current_output = current
    else:
        destination = epub_path(resolved)
        current_output = epub_path(current)
    relative = posix(PurePosixPath(os.path.relpath(destination, PurePosixPath(current_output).parent)))
    return quote(relative, safe="/:._-~") + fragment


class XHTMLBodyConverter(HTMLParser):
    def __init__(
        self,
        current: str,
        current_output: str,
        current_anchor: str,
        page_targets: dict[str, tuple[str, str]],
        asset_paths: set[str],
    ):
        super().__init__(convert_charrefs=True)
        self.current = current
        self.current_output = current_output
        self.current_anchor = current_anchor
        self.page_targets = page_targets
        self.asset_paths = asset_paths
        self.output: list[str] = []
        self.stack: list[str] = []
        self.in_body = False
        self.saw_body = False
        self.drop_depth = 0

    def _close_to(self, tag: str) -> None:
        if tag not in self.stack:
            return
        while self.stack:
            current = self.stack.pop()
            self.output.append(f"</{current}>")
            if current == tag:
                break

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        tag = re.sub(r"[^a-zA-Z0-9_.-]", "_", tag.lower())
        if tag == "body":
            self.in_body = True
            self.saw_body = True
            return
        if tag in {"html", "head"} or not self.in_body:
            return
        if tag in DROP_TAGS:
            self.drop_depth += 1
            return
        if self.drop_depth:
            return
        if tag == "p" and "p" in self.stack:
            self._close_to("p")
        if tag == "li" and "li" in self.stack:
            self._close_to("li")
        if tag == "tr" and "tr" in self.stack:
            self._close_to("tr")
        if tag in {"td", "th"} and self.stack and self.stack[-1] in {"td", "th"}:
            self._close_to(self.stack[-1])
        rendered_attrs: list[str] = []
        seen_attrs: set[str] = set()
        has_explicit_id = any(name.lower() == "id" for name, _ in attrs)
        for name, value in attrs:
            name = name.lower()
            if not re.fullmatch(r"[a-zA-Z_][a-zA-Z0-9_.:-]*", name):
                continue
            name = name.replace(":", "_")
            if name == "name":
                if has_explicit_id:
                    continue
                name = "id"
            if name in seen_attrs:
                continue
            seen_attrs.add(name)
            if value is None:
                value = name
            if name in {"id", "name"}:
                value = f"{self.current_anchor}_{value}"
            if name in {"href", "src"}:
                value = rewrite_epub_link(
                    value,
                    self.current,
                    self.current_output,
                    self.current_anchor,
                    self.page_targets,
                    self.asset_paths,
                )
                if not value:
                    continue
            if name.startswith("on"):
                continue
            rendered_attrs.append(f' {name}="{html.escape(value, quote=True)}"')
        attrs_text = "".join(rendered_attrs)
        if tag in VOID_TAGS:
            self.output.append(f"<{tag}{attrs_text} />")
        else:
            self.output.append(f"<{tag}{attrs_text}>")
            self.stack.append(tag)

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        self.handle_starttag(tag, attrs)
        if tag.lower() not in VOID_TAGS and self.stack and self.stack[-1] == tag.lower():
            self._close_to(tag.lower())

    def handle_endtag(self, tag: str) -> None:
        tag = re.sub(r"[^a-zA-Z0-9_.-]", "_", tag.lower())
        if tag == "body":
            while self.stack:
                self.output.append(f"</{self.stack.pop()}>")
            self.in_body = False
            return
        if tag in DROP_TAGS and self.drop_depth:
            self.drop_depth -= 1
            return
        if self.in_body and not self.drop_depth:
            self._close_to(tag)

    def handle_data(self, data: str) -> None:
        if self.in_body and not self.drop_depth:
            self.output.append(html.escape(data, quote=False))

    def handle_entityref(self, name: str) -> None:
        if self.in_body and not self.drop_depth:
            self.output.append(f"&amp;{name};")

    def handle_charref(self, name: str) -> None:
        if self.in_body and not self.drop_depth:
            self.output.append(f"&#{name};")

    def body(self) -> str:
        while self.stack:
            self.output.append(f"</{self.stack.pop()}>")
        return "".join(self.output)


class HelpTextExtractor(HTMLParser):
    """Turn legacy, often invalid help HTML into conservative EPUB-safe text."""

    BREAK_BEFORE = {"address", "article", "blockquote", "caption", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tbody", "tfoot", "thead", "tr", "ul"}
    BREAK_AFTER = BREAK_BEFORE | {"br"}

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.parts: list[str] = []
        self.in_body = False
        self.drop_depth = 0

    def _break(self) -> None:
        if self.parts and self.parts[-1] != "\n":
            self.parts.append("\n")

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        tag = tag.lower()
        if tag == "body":
            self.in_body = True
            return
        if tag in {"script", "style", "head"}:
            self.drop_depth += 1
            return
        if not self.in_body or self.drop_depth:
            return
        if tag in self.BREAK_BEFORE:
            self._break()
        if tag == "li":
            self.parts.append("• ")
        elif tag in {"td", "th"}:
            self.parts.append("  ")

    def handle_endtag(self, tag: str) -> None:
        tag = tag.lower()
        if tag in {"script", "style", "head"} and self.drop_depth:
            self.drop_depth -= 1
            return
        if tag == "body":
            self.in_body = False
            return
        if self.in_body and not self.drop_depth and tag in self.BREAK_AFTER:
            self._break()

    def handle_data(self, data: str) -> None:
        if self.in_body and not self.drop_depth:
            self.parts.append(data)

    def xhtml(self) -> str:
        text = "".join(self.parts).replace("\r", "\n")
        lines: list[str] = []
        for line in text.splitlines():
            line = re.sub(r"[\t \f\v]+", " ", line).strip()
            if line:
                lines.append(line)
            elif lines and lines[-1] != "":
                lines.append("")
        while lines and not lines[-1]:
            lines.pop()
        paragraphs: list[str] = []
        block: list[str] = []
        for line in lines + [""]:
            if line:
                block.append(line)
            elif block:
                paragraphs.append("<p>" + "<br/>".join(html.escape(item, quote=False) for item in block) + "</p>")
                block = []
        return "".join(paragraphs)


def rewrite_epub_link(
    value: str,
    current_source: str,
    current_output: str,
    current_anchor: str,
    page_targets: dict[str, tuple[str, str]],
    asset_paths: set[str],
) -> str:
    if not value or value.startswith(("http://", "https://", "mailto:", "javascript:", "data:")):
        return value
    if value.lower().startswith("v8help://service_book/"):
        return ""
    v8_target = resolve_v8help(value)
    if v8_target is not None:
        value = v8_target
        absolute = True
    else:
        absolute = False
    target, fragment = split_fragment(value)
    if not target:
        if not fragment:
            return ""
        return f"#{current_anchor}_{fragment.removeprefix('#')}"
    target = unquote(target).replace("\\", "/")
    if absolute:
        resolved = posix(PurePosixPath(target))
    else:
        resolved = posix(PurePosixPath(os.path.normpath(posix(PurePosixPath(current_source).parent / target))))
    if resolved in page_targets:
        destination, anchor = page_targets[resolved]
        relative = posix(PurePosixPath(os.path.relpath(destination, PurePosixPath(current_output).parent)))
        return quote(relative, safe="/:._-~") + "#" + anchor
    if resolved in asset_paths:
        destination = epub_path(resolved)
        relative = posix(PurePosixPath(os.path.relpath(destination, PurePosixPath(current_output).parent)))
        return quote(relative, safe="/:._-~")
    return value


def xhtml_document(title: str, body: str, css_href: str) -> str:
    return f'''<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="ru" lang="ru">
<head><meta charset="utf-8"/><title>{html.escape(title)}</title><link rel="stylesheet" href="{css_href}" type="text/css"/></head>
<body>{body}</body>
</html>
'''


def extract_title(source: str, fallback: str) -> str:
    match = re.search(r"(?is)<h1[^>]*>(.*?)</h1>", source) or re.search(r"(?is)<title[^>]*>(.*?)</title>", source)
    if not match:
        return fallback
    return re.sub(r"<[^>]+>", "", html.unescape(match.group(1))).strip() or fallback


def rewrite_chm_html(source: str, current: str, page_paths: set[str], asset_paths: set[str]) -> str:
    style = '<link rel="stylesheet" type="text/css" href="' + posix(PurePosixPath(os.path.relpath("_book.css", PurePosixPath(current).parent))) + '">'
    source = re.sub(r"(?i)<link\b[^>]*v8help://service_book/[^>]*>", style, source)

    def replace(match: re.Match[str]) -> str:
        prefix, quote_char, value = match.groups()
        rewritten = rewrite_link(value, current, "chm", page_paths, asset_paths)
        if not rewritten:
            return ""
        return f"{prefix}{quote_char}{rewritten}{quote_char}"

    source = re.sub(r"(?i)(<(?:a|link|img)\b[^>]*?\b(?:href|src)\s*=\s*)(['\"])(.*?)\2", replace, source)
    return source


def collection_title(collection: str) -> str:
    return COLLECTION_TITLES.get(collection, collection.removesuffix("_ru"))


def build_collection_landing(
    collection: str,
    pages: list[dict[str, str]],
    mode: str,
    epub_parts: list[tuple[str, list[dict[str, str]]]] | None = None,
) -> str:
    items = []
    if mode == "epub" and epub_parts is not None:
        current = f"OEBPS/text/_collections/{collection}.xhtml"
        for number, (target, part_pages) in enumerate(epub_parts, start=1):
            relative = posix(PurePosixPath(os.path.relpath(target, PurePosixPath(current).parent)))
            first_title = part_pages[0]["title"]
            last_title = part_pages[-1]["title"]
            items.append(
                f'<li><a href="{quote(relative, safe="/:._-~")}">Часть {number}</a>: '
                f'{html.escape(first_title)} — {html.escape(last_title)}</li>'
            )
        return f"<h1>{html.escape(collection_title(collection))}</h1><p>{len(pages)} страниц</p><ol>{''.join(items)}</ol>"
    for page in sorted(pages, key=lambda item: item["title"].casefold()):
        target = page["path"] if mode == "chm" else epub_path(page["path"])
        current = f"_collections/{collection}.html" if mode == "chm" else f"OEBPS/text/_collections/{collection}.xhtml"
        relative = posix(PurePosixPath(os.path.relpath(target, PurePosixPath(current).parent)))
        if mode == "epub":
            relative = quote(relative, safe="/:._-~")
        items.append(f'<li><a href="{relative}">{html.escape(page["title"])}</a></li>')
    return f"<h1>{html.escape(collection_title(collection))}</h1><p>{len(pages)} страниц</p><ul>{''.join(items)}</ul>"


def write_text(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value, encoding="utf-8")


def build_epub(source_root: Path, output_file: Path, catalogs: dict[str, list[dict[str, str]]], work: Path) -> None:
    catalogs = {name: catalogs[name] for name in EPUB_COLLECTIONS if name in catalogs}
    html_root = source_root / "html"
    page_titles = {page["path"]: page["title"] for pages in catalogs.values() for page in pages}
    root = work / "epub"
    write_text(root / "META-INF/container.xml", '''<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>''')
    css = "body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.45;margin:5%;}a{color:#075985;}section.help-page{break-before:page;page-break-before:always;border-top:1px solid #ccc;padding-top:1em}.source-path{color:#777;font-size:.75em}.help-content{white-space:normal}"
    write_text(root / "OEBPS/styles/book.css", css)
    manifest: list[tuple[str, str, str, str]] = []
    spine: list[str] = []
    nav_items: list[str] = []
    ncx_items: list[str] = []
    play_order = 1
    chunk_size = 200
    page_targets: dict[str, tuple[str, str]] = {}
    collection_parts: dict[str, list[tuple[str, list[dict[str, str]]]]] = {}
    for collection, pages in catalogs.items():
        sorted_pages = sorted(pages, key=lambda item: item["title"].casefold())
        parts: list[tuple[str, list[dict[str, str]]]] = []
        for offset in range(0, len(sorted_pages), chunk_size):
            part_pages = sorted_pages[offset : offset + chunk_size]
            part_number = offset // chunk_size + 1
            target = f"OEBPS/text/{collection}/part-{part_number:04d}.xhtml"
            parts.append((target, part_pages))
            for page in part_pages:
                anchor = "p_" + hashlib.sha1(page["path"].encode()).hexdigest()[:16]
                page_targets[page["path"]] = (target, anchor)
        collection_parts[collection] = parts
    for collection, pages in catalogs.items():
        landing_relative = f"OEBPS/text/_collections/{collection}.xhtml"
        landing_id = f"collection_{collection}"
        css_href = "../../styles/book.css"
        landing_body = build_collection_landing(collection, pages, "epub", collection_parts[collection])
        write_text(root / landing_relative, xhtml_document(collection_title(collection), landing_body, css_href))
        manifest.append((landing_id, landing_relative.removeprefix("OEBPS/"), "application/xhtml+xml", ""))
        spine.append(landing_id)
        nav_href = quote(landing_relative.removeprefix("OEBPS/"), safe="/:._-~")
        nav_items.append(f'<li><a href="{nav_href}">{html.escape(collection_title(collection))}</a></li>')
        ncx_items.append(f'<navPoint id="nav{play_order}" playOrder="{play_order}"><navLabel><text>{html.escape(collection_title(collection))}</text></navLabel><content src="{nav_href}"/></navPoint>')
        play_order += 1
        for part_number, (destination_relative, part_pages) in enumerate(collection_parts[collection], start=1):
            article_toc = []
            article_bodies = []
            for page in part_pages:
                original = html_root / page["path"]
                source = original.read_text(encoding="utf-8-sig", errors="replace")
                anchor = page_targets[page["path"]][1]
                converter = HelpTextExtractor()
                converter.feed(source)
                title = extract_title(source, page["title"])
                article_toc.append(f'<li><a href="#{anchor}">{html.escape(title)}</a></li>')
                article_bodies.append(
                    f'<section id="{anchor}" class="help-page"><p class="source-path">{html.escape(page["path"])}</p>'
                    f'<div class="help-content">{converter.xhtml()}</div></section>'
                )
            part_title = f"{collection_title(collection)} — часть {part_number}"
            body = f'<h1>{html.escape(part_title)}</h1><ol>{"".join(article_toc)}</ol>{"".join(article_bodies)}'
            css_relative = posix(PurePosixPath(os.path.relpath("OEBPS/styles/book.css", PurePosixPath(destination_relative).parent)))
            write_text(root / destination_relative, xhtml_document(part_title, body, quote(css_relative, safe="/:._-~")))
            item_id = f"part_{collection}_{part_number:04d}"
            manifest.append((item_id, destination_relative.removeprefix("OEBPS/"), "application/xhtml+xml", ""))
            spine.append(item_id)
        print(f"EPUB: {collection} ({len(pages)})", flush=True)
    nav = xhtml_document("Оглавление", f'<nav epub:type="toc" id="toc" xmlns:epub="http://www.idpf.org/2007/ops"><h1>Оглавление</h1><ol>{"".join(nav_items)}</ol></nav>', "styles/book.css")
    write_text(root / "OEBPS/nav.xhtml", nav)
    ncx = f'''<?xml version="1.0" encoding="utf-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="{BOOK_ID}"/></head><docTitle><text>{html.escape(TITLE)}</text></docTitle><navMap>{''.join(ncx_items)}</navMap></ncx>'''
    write_text(root / "OEBPS/toc.ncx", ncx)
    manifest_lines = ['<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>', '<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>', '<item id="css" href="styles/book.css" media-type="text/css"/>']
    for item_id, href, media, properties in manifest:
        prop = f' properties="{properties}"' if properties else ""
        manifest_lines.append(f'<item id="{item_id}" href="{quote(href, safe="/:._-~")}" media-type="{media}"{prop}/>')
    spine_lines = "".join(f'<itemref idref="{item_id}"/>' for item_id in spine)
    opf = f'''<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="ru"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="bookid">{BOOK_ID}</dc:identifier><dc:title>{html.escape(TITLE)}</dc:title><dc:language>{LANGUAGE}</dc:language><dc:creator>Фирма «1С»</dc:creator><meta property="dcterms:modified">2026-09-22T00:00:00Z</meta></metadata><manifest>{''.join(manifest_lines)}</manifest><spine toc="ncx">{spine_lines}</spine></package>'''
    write_text(root / "OEBPS/package.opf", opf)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output_file, "w") as archive:
        archive.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
        for path in sorted(root.rglob("*")):
            if path.is_file():
                archive.write(path, posix(path.relative_to(root)), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
    print(f"EPUB ready: {output_file}", flush=True)


def build_chm(source_root: Path, output_file: Path, catalogs: dict[str, list[dict[str, str]]], work: Path) -> None:
    compiler = shutil.which("chmcmd")
    if compiler is None:
        raise RuntimeError("chmcmd is required to build CHM")
    html_root = source_root / "html"
    page_titles = {page["path"]: page["title"] for pages in catalogs.values() for page in pages}
    page_paths = set(page_titles)
    assets = [path for path in html_root.rglob("*") if path.is_file() and path.suffix.lower() in IMAGE_SUFFIXES]
    asset_paths = {posix(path.relative_to(html_root)) for path in assets}
    root = work / "chm"
    root.mkdir(parents=True, exist_ok=True)
    css = "body{font-family:'Segoe UI',Arial,sans-serif;line-height:1.4;margin:24px;}table{border-collapse:collapse;}td,th{border:1px solid #aaa;padding:4px;}img{max-width:100%;height:auto;}"
    write_text(root / "_book.css", css)
    for asset in assets:
        destination = root / asset.relative_to(html_root)
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(asset, destination)
    files = ["index.html", "_book.css"]
    toc_items = []
    landing_links = []
    for collection, pages in catalogs.items():
        landing = f"_collections/{collection}.html"
        body = build_collection_landing(collection, pages, "chm")
        landing_html = f'<!doctype html><html><head><meta charset="utf-8"><title>{html.escape(collection_title(collection))}</title><link rel="stylesheet" href="../_book.css"></head><body>{body}</body></html>'
        write_text(root / landing, landing_html)
        files.append(landing)
        landing_links.append(f'<li><a href="{landing}">{html.escape(collection_title(collection))}</a> — {len(pages)} страниц</li>')
        toc_items.append(f'<li><object type="text/sitemap"><param name="Name" value="{html.escape(collection_title(collection), quote=True)}"><param name="Local" value="{landing}"></object>')
        for page in pages:
            source = (html_root / page["path"]).read_text(encoding="utf-8-sig", errors="replace")
            destination = root / page["path"]
            write_text(destination, rewrite_chm_html(source, page["path"], page_paths, asset_paths))
            files.append(page["path"])
        print(f"CHM: {collection} ({len(pages)})", flush=True)
    for asset in assets:
        files.append(posix(asset.relative_to(html_root)))
    index_html = f'<!doctype html><html><head><meta charset="utf-8"><title>{html.escape(TITLE)}</title><link rel="stylesheet" href="_book.css"></head><body><h1>{html.escape(TITLE)}</h1><p>{sum(map(len, catalogs.values()))} страниц локальной русской справки.</p><ul>{"".join(landing_links)}</ul></body></html>'
    write_text(root / "index.html", index_html)
    hhc = '<!doctype html><html><body><ul><li><object type="text/sitemap"><param name="Name" value="' + html.escape(TITLE, quote=True) + '"><param name="Local" value="index.html"></object><ul>' + "".join(toc_items) + '</ul></li></ul></body></html>'
    write_text(root / "book.hhc", hhc)
    compiled = output_file.resolve()
    project = f'''[OPTIONS]
Compatibility=1.1 or later
Compiled file={compiled}
Contents file=book.hhc
Default topic=index.html
Display compile progress=No
Full-text search=Yes
Language=0x419 Russian
Title={TITLE}

[FILES]
{os.linesep.join(files)}
'''
    write_text(root / "book.hhp", project)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run([compiler, "--verbosity", "2", "book.hhp"], cwd=root, check=True)
    print(f"CHM ready: {output_file}", flush=True)


def main() -> None:
    args = parse_args()
    source = args.source.resolve()
    output = args.output.resolve()
    catalogs = read_catalogs(source / "json", source / "html")
    work_root = Path(tempfile.mkdtemp(prefix="metalab-its-books-"))
    try:
        if args.format in {"all", "epub"}:
            build_epub(source, output / "1C_Enterprise_8.3.27.2342_RU.epub", catalogs, work_root)
        if args.format in {"all", "chm"}:
            build_chm(source, output / "1C_Enterprise_8.3.27.2342_RU.chm", catalogs, work_root)
    finally:
        if args.keep_work:
            print(f"Work directory kept: {work_root}")
        else:
            shutil.rmtree(work_root, ignore_errors=True)


if __name__ == "__main__":
    main()
