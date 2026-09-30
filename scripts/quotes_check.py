#!/usr/bin/env python3
"""Find phrases of the prototype's help, books and ITS articles quoted verbatim.

The materials are proprietary, the repository is Apache 2.0: their behaviour
is described in our own words, never in theirs (CLAUDE.md, "Материалы"). A
quotation in «» is where a phrase of theirs most easily slips in, so every
quotation in the Go sources and the requirements is looked up in the
materials, and the ones found there word for word are reported.

Only a match is reported. A quotation of our own wording - a term of the
requirements, an example - is found nowhere and passes. Names shorter than
MIN_WORDS words are not looked up at all, and longer names of properties,
commands and captions that we carry on purpose are listed in
scripts/quotes_names.txt: a name is composition, not wording. A sentence of
the help never goes into that list.

    python3 scripts/quotes_check.py            # report, exit 1 if anything is found
    python3 scripts/quotes_check.py --summary  # counts only
    python3 scripts/quotes_check.py --phrases  # runs of words outside quotes, for review

--phrases looks for a phrase that slipped in without quotes: every run of
PHRASE_WORDS consecutive words of the Go comments and the requirements that
the materials contain too. It is for reading, not for failing: a list of
names - kinds of objects, types, events of a form written one after another -
matches a list in the help just as well, and only a reader tells a list of
names from a sentence.

The materials live in docs/materials, which is not in the repository; without
them the check cannot run and says so.
"""

import argparse
import glob
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MIN_WORDS = 4
PHRASE_WORDS = 8
QUOTE = re.compile(r"«([^«»]*)»", re.S)
WORD = re.compile(r"[0-9a-zа-я]+")


def normalize(text):
    text = text.lower().replace("ё", "е")
    return " ".join(WORD.findall(text))


def load_corpora():
    corpora = {}
    help_files = glob.glob(os.path.join(ROOT, "docs/materials/its/*/json/*.json"))
    if help_files:
        parts = []
        for path in help_files:
            with open(path, encoding="utf-8") as handle:
                data = json.load(handle)
            for page in data.get("pages", []):
                parts.append(page.get("text") or "")
        corpora["справка"] = " " + normalize("\n".join(parts)) + " "
    book_pages = glob.glob(os.path.join(ROOT, "docs/materials/books/*/pages/*.txt"))
    if book_pages:
        parts = []
        for path in book_pages:
            with open(path, encoding="utf-8", errors="replace") as handle:
                parts.append(handle.read())
        corpora["книги"] = " " + normalize("\n".join(parts)) + " "
    articles = glob.glob(os.path.join(ROOT, "docs/materials/its/metod8dev/*.md"))
    if articles:
        parts = []
        for path in articles:
            with open(path, encoding="utf-8") as handle:
                parts.append(handle.read())
        corpora["статьи ИТС"] = " " + normalize("\n".join(parts)) + " "
    return corpora


def names():
    path = os.path.join(ROOT, "scripts/quotes_names.txt")
    if not os.path.exists(path):
        return set()
    with open(path, encoding="utf-8") as handle:
        return {normalize(line) for line in handle if line.strip() and not line.startswith("#")}


def sources():
    for pattern in ("internal/**/*.go", "cmd/**/*.go", "docs/requirements/**/*.md"):
        for path in glob.glob(os.path.join(ROOT, pattern), recursive=True):
            yield path


def quotations(path):
    with open(path, encoding="utf-8") as handle:
        text = handle.read()
    go = path.endswith(".go")
    for match in QUOTE.finditer(text):
        body = match.group(1)
        if go:
            # A quotation carried over lines of a comment carries the
            # comment markers with it.
            body = re.sub(r"\n\s*//\s?", " ", body)
        line = text.count("\n", 0, match.start()) + 1
        # An ellipsis inside a quotation joins pieces that are each quoted.
        for piece in re.split(r"\.\.\.|…", body):
            words = normalize(piece)
            if len(words.split()) >= MIN_WORDS:
                yield line, " ".join(piece.split()), words


def text_runs(path):
    """Runs of text lines: whole requirements, or the comment blocks of Go."""
    with open(path, encoding="utf-8") as handle:
        lines = handle.read().split("\n")
    if not path.endswith(".go"):
        yield list(enumerate(lines, 1))
        return
    run = []
    for number, line in enumerate(lines, 1):
        stripped = line.strip()
        if stripped.startswith("//"):
            run.append((number, stripped[2:]))
        elif run:
            yield run
            run = []
    if run:
        yield run


def phrases(corpora):
    ours = {}
    for path in sorted(sources()):
        for run in text_runs(path):
            tokens = [(word, number) for number, line in run for word in normalize(line).split()]
            for start in range(len(tokens) - PHRASE_WORDS + 1):
                phrase = " ".join(word for word, _ in tokens[start:start + PHRASE_WORDS])
                if re.search("[а-я]", phrase):
                    ours.setdefault(phrase, (os.path.relpath(path, ROOT), tokens[start][1]))
    hits = {}
    for name, corpus in corpora.items():
        words = corpus.split()
        for start in range(len(words) - PHRASE_WORDS + 1):
            phrase = " ".join(words[start:start + PHRASE_WORDS])
            if phrase in ours:
                hits.setdefault(ours[phrase], (set(), phrase))[0].add(name)
    for (path, line), (names_found, phrase) in sorted(hits.items()):
        print(f"{path}:{line}: [{', '.join(sorted(names_found))}] {phrase}")
    print(f"совпадений по {PHRASE_WORDS} слов подряд: {len(hits)} - просмотреть: перечень имён законен, фраза - нет")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--summary", action="store_true", help="print counts only")
    parser.add_argument("--phrases", action="store_true", help="list runs of words found in the materials, for review")
    arguments = parser.parse_args()
    corpora = load_corpora()
    if "справка" not in corpora:
        print("docs/materials/its is missing: the help cannot be searched, nothing was checked", file=sys.stderr)
        return 2
    if arguments.phrases:
        return phrases(corpora)
    found, checked, known = [], 0, names()
    for path in sorted(sources()):
        for line, quoted, words in quotations(path):
            if words in known:
                continue
            checked += 1
            hits = [name for name, corpus in corpora.items() if " " + words + " " in corpus]
            if hits:
                found.append((os.path.relpath(path, ROOT), line, quoted, hits))
    if not arguments.summary:
        for path, line, quoted, hits in found:
            print(f"{path}:{line}: [{', '.join(hits)}] «{quoted}»")
    by_kind = {}
    for path, *_ in found:
        kind = "код" if path.endswith(".go") else "ТЗ"
        by_kind[kind] = by_kind.get(kind, 0) + 1
    print(f"цитат проверено: {checked}, дословно из материалов: {len(found)}"
          + (" (" + ", ".join(f"{kind} {count}" for kind, count in sorted(by_kind.items())) + ")" if by_kind else ""))
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
