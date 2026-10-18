#!/usr/bin/env python3
"""Refuses a new copy of Go or JavaScript code: CLAUDE.md, "Одно знание — одно место".

dupl finds groups of copied Go code. dupl reads only Go, so JavaScript is read
here by the same rule: names and values do not count, only the shape of the
code, and a copy is a run of THRESHOLD tokens that occurs twice. jscpd, the
usual tool for JavaScript, compares the text itself and so misses a copy in
which only the names were changed, which dupl does find in Go.

The groups that existed when each check was introduced are written down in a
baseline of its own, scripts/dupl-baseline.txt for Go and
scripts/dupl-baseline-js.txt for JavaScript, keyed by the files the copies lie
in rather than by lines, so that an edit above a copy does not count as a new
one. The check fails when a group appears that the baseline does not have, and
also when a group from the baseline is gone: the baseline only shrinks, and it
is written down shrunk by --update in the same commit that removed the copy.
docs/duplicates-audit.md says what each group is.

--near is for the end of an iteration and never fails: it lists the smaller
copies, down to NEAR tokens, that touch a file changed since HEAD, so that
two functions differing only in what they return are seen while they are
being written. Each one is either merged or said in the report why it is not
one piece of knowledge.

    python3 scripts/dupl_check.py            # check
    python3 scripts/dupl_check.py --update   # write the baselines anew
    python3 scripts/dupl_check.py --near     # near copies in changed files
"""

import collections
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DUPL = ["go", "run", "github.com/mibk/dupl@v1.1.0"]
# 60 tokens is about eight lines of Go: it still sees two functions that differ
# only in what they return, which 80 let through (owner, 13.10.2026).
# The same number holds for JavaScript, where a token is a word or a sign of
# the text rather than a node of the syntax tree: 60 of them are five to eight
# lines, as in Go (2.317, measured on Studio and the client).
THRESHOLD = 60
# Below 50 dupl reports error handling and field-by-field copying of structs.
NEAR = 50


def sources(extension, test_suffix):
    found = []
    for top in ("internal", "cmd"):
        for directory, _, names in os.walk(os.path.join(ROOT, top)):
            for name in names:
                if name.endswith(extension) and not name.endswith(test_suffix):
                    found.append(os.path.relpath(os.path.join(directory, name), ROOT))
    return sorted(found)


# A block of constants that only names values - each line "Name Type = value"
# or "Name = value", the value a string or a number - is a list, not code: two
# such lists with as many lines are one copy to dupl, which compares the
# block whole and neither the names nor the values, though they hold nothing
# in common (the kinds of note and the rights of a role, 65 lines each, in
# 2.357). Knowledge cannot be written twice in it, so a group whose every
# place is such a block is no copy (owner, 18.10.2026). The threshold and the
# baseline stay as they are.
CONSTANT_LINE = re.compile(r'[A-Za-z_]\w*(\s+[A-Za-z_][\w.]*)?\s*=\s*("(?:[^"\\]|\\.)*"|`[^`]*`|-?\d[\w.]*)\s*(//.*)?')
# What the rule takes for a list and what it does not, checked on every run so
# that a looser pattern cannot let a piece of code through.
CONSTANT_LIST_SAMPLES = [
    (["const (", '\tNoteUnusedBound NoteKind = "unused-bound"', "\t// a comment", "",
      '\tNoteWithoutSample = "accepted-without-sample"', "\tmaxNameLength = 255", ")"], True),
    (["const (", '\tPermissionRead PermissionOperation = "read"', "\tlimit = len(names)", ")"], False),
    (["const (", "\tFirst Kind = iota", "\tSecond", ")"], False),
    (["var (", '\tname = "x"', ")"], False),
    (["func f() {", '\tname := "x"', "}"], False),
]


def constant_list(lines):
    """True when the lines are one const block that only names values."""
    body = [line.strip() for line in lines]
    if len(body) < 2 or body[0] != "const (" or body[-1] != ")":
        return False
    return all(not line or line.startswith("//") or CONSTANT_LINE.fullmatch(line) for line in body[1:-1])


def constant_list_place(place):
    path, span = place.rsplit(":", 1)
    start, end = (int(number) for number in span.split(","))
    with open(os.path.join(ROOT, path), encoding="utf-8") as file:
        return constant_list(file.read().splitlines()[start - 1:end])


def go_clones(threshold):
    """Each group of copies as a list of "file:from,to", but for lists of
    constants (constant_list)."""
    run = subprocess.run(DUPL + ["-t", str(threshold), "-files"], cwd=ROOT,
                         input="\n".join(sources(".go", "_test.go")),
                         capture_output=True, text=True)
    if run.returncode != 0:
        sys.exit("dupl failed:\n" + run.stderr)
    found, current = [], []
    for line in run.stdout.splitlines() + [""]:
        if line.startswith("  "):
            current.append(line.strip())
        elif current:
            if not all(constant_list_place(place) for place in current):
                found.append(current)
            current = []
    return found


# JavaScript is cut into tokens by its lexical grammar. A name becomes NAME and
# a string, a number, a template or a regular expression becomes VALUE, so two
# pieces of code differing only in them are one copy; keywords and signs stay
# as written. The expressions inside a template's ${} are tokens of their own,
# as dupl sees the arguments of a call.
JS_KEYWORDS = set("""async await break case catch class const continue debugger default delete do
    else export extends false finally for function if import in instanceof let new null of return
    static super switch this throw true try typeof var void while with yield""".split())
JS_SIGNS = sorted(""">>>= ... === !== **= <<= >>= >>> &&= ||= ??= ?? ?. => == != <= >= && || ++ --
    += -= *= /= %= &= |= ^= ** << >> { } ( ) [ ] ; , < > + - * / % & | ^ ! ~ ? : = . @""".split(),
                  key=len, reverse=True)
# After these a slash opens a regular expression; after a name, a value or a
# closing bracket it divides.
JS_REGEX_AFTER = set("""( , = : [ ! & | ? { } ; + - * % < > ~ ^ => == != === !== <= >= && || ??
    += -= *= /= %= &&= ||= ??= return typeof case do else in of new delete void throw yield await""".split())
# A private name, #field, is one token, as a name is.
JS_NAME = re.compile(r"#?[A-Za-z_$][\w$]*")
JS_NUMBER = re.compile(r"0[xXoObB][\da-fA-F_]+n?|(\d[\d_]*\.?[\d_]*|\.\d[\d_]*)([eE][+-]?\d+)?n?")


def js_tokens(text):
    """Each token as (kind, line)."""
    found, braces = [], []
    at, line, previous = 0, 1, None

    def template(at, line):
        """Reads a template from after ` or } up to its end or to ${."""
        while True:
            if text[at] == "\\":
                at += 2
            elif text[at] == "`":
                return at + 1, line, False
            elif text.startswith("${", at):
                return at + 2, line, True
            else:
                line += text[at] == "\n"
                at += 1

    while at < len(text):
        char = text[at]
        if char.isspace():
            line += char == "\n"
            at += 1
            continue
        if text.startswith("//", at):
            end = text.find("\n", at)
            at = len(text) if end < 0 else end
            continue
        if text.startswith("/*", at):
            end = text.index("*/", at + 2)
            line += text.count("\n", at, end)
            at = end + 2
            continue
        start = line
        if char == "}" and braces and braces[-1] == "${":
            braces.pop()
            at, line, opened = template(at + 1, line)
            if opened:
                braces.append("${")
            found += [("}", start), ("VALUE", start)]
            previous = "VALUE"
            continue
        if char in "'\"":
            end = at + 1
            while text[end] != char:
                end += 2 if text[end] == "\\" else 1
            at, kind = end + 1, "VALUE"
        elif char == "`":
            at, line, opened = template(at + 1, line)
            if opened:
                braces.append("${")
            kind = "VALUE"
        elif char == "/" and (previous is None or previous in JS_REGEX_AFTER):
            end, in_class = at + 1, False
            while in_class or text[end] != "/":
                if text[end] == "\\":
                    end += 1
                elif text[end] == "[":
                    in_class = True
                elif text[end] == "]":
                    in_class = False
                end += 1
            end += 1
            while end < len(text) and text[end].isalpha():
                end += 1
            at, kind = end, "VALUE"
        elif JS_NAME.match(text, at):
            word = JS_NAME.match(text, at).group()
            at += len(word)
            kind = word if word in JS_KEYWORDS else "NAME"
            if kind in JS_KEYWORDS:
                found.append((kind, start))
                previous = word
                continue
        elif char.isdigit() or (char == "." and text[at + 1:at + 2].isdigit()):
            at, kind = JS_NUMBER.match(text, at).end(), "VALUE"
        else:
            sign = next((sign for sign in JS_SIGNS if text.startswith(sign, at)), None)
            if sign is None:
                raise ValueError(f"line {line}: cannot read {text[at:at + 20]!r}")
            at, kind = at + len(sign), sign
            if sign == "{":
                braces.append("{")
            elif sign == "}" and braces:
                braces.pop()
        found.append((kind, start))
        previous = kind
    return found


def js_clones(threshold):
    """Each group of copies as a list of "file:from,to", as dupl prints them.

    Every run of threshold tokens met twice is grown to the longest copy; the
    places of copies that overlap are joined into one place, and places linked
    by a copy are one group."""
    kinds, places = [], []
    for name in sources(".js", ".test.js"):
        with open(os.path.join(ROOT, name), encoding="utf-8") as file:
            try:
                tokens = js_tokens(file.read())
            except (ValueError, IndexError) as error:
                sys.exit(f"{name}: {error or 'unexpected end of file'}")
        for kind, line in tokens:
            kinds.append(kind)
            places.append((name, line))
        kinds.append(None)  # a copy does not cross the end of a file
        places.append((name, 0))
    runs = collections.defaultdict(list)
    for at in range(len(kinds) - threshold + 1):
        run = tuple(kinds[at:at + threshold])
        if None not in run:
            runs[run].append(at)
    copies = []
    for starts in runs.values():
        for index, first in enumerate(starts):
            for second in starts[index + 1:]:
                if first + threshold > second:
                    continue  # a run overlapping itself is a repetition, not a copy
                if first > 0 and kinds[first - 1] is not None and kinds[first - 1] == kinds[second - 1]:
                    continue  # found already, from where it starts
                length = threshold
                while first + length < second and kinds[first + length] is not None \
                        and kinds[first + length] == kinds[second + length]:
                    length += 1
                copies.append(((first, first + length - 1), (second, second + length - 1)))
    spans = sorted({span for copy in copies for span in copy})
    joined, owner = [], {}
    for span in spans:
        if joined and span[0] <= joined[-1][1]:
            joined[-1][1] = max(joined[-1][1], span[1])
        else:
            joined.append([span[0], span[1]])
        owner[span] = len(joined) - 1
    parent = list(range(len(joined)))

    def root(index):
        while parent[index] != index:
            parent[index] = parent[parent[index]]
            index = parent[index]
        return index

    for first, second in copies:
        parent[root(owner[first])] = root(owner[second])
    linked = collections.defaultdict(list)
    for index, (start, end) in enumerate(joined):
        linked[root(index)].append(f"{places[start][0]}:{places[start][1]},{places[end][1]}")
    return list(linked.values())


CHECKS = [
    ("dupl", go_clones, os.path.join(ROOT, "scripts", "dupl-baseline.txt"),
     f"find internal cmd -name '*.go' ! -name '*_test.go' | {' '.join(DUPL)} -t {THRESHOLD} -files"),
    ("js", js_clones, os.path.join(ROOT, "scripts", "dupl-baseline-js.txt"),
     "python3 scripts/dupl_check.py --near --all"),
]


def groups(clones):
    counted = collections.Counter()
    for group in clones(THRESHOLD):
        counted[" ".join(sorted(place.rsplit(":", 1)[0] for place in group))] += 1
    return counted


def changed():
    names = set()
    for command in (["git", "diff", "--name-only", "HEAD"], ["git", "ls-files", "--others", "--exclude-standard"]):
        run = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, check=True)
        names.update(run.stdout.split())
    return names


def near(everything):
    """--all lists every copy from THRESHOLD tokens rather than the near ones in changed files."""
    touched = None if everything else changed()
    size = THRESHOLD if everything else NEAR
    for title, clones, _, _ in CHECKS:
        found = [group for group in clones(size)
                 if touched is None or any(place.rsplit(":", 1)[0] in touched for place in group)]
        for group in found:
            print("copies:" if everything else "near copies:")
            for place in group:
                print("  " + place)
        if everything:
            print(f"{title} -t {size}: {len(found)} groups")
        else:
            print(f"{title} -t {size}: {len(found)} groups touch the changed files; merge each, "
                  "or say in the report why it is not one piece of knowledge")


def read_baseline(path):
    counted = collections.Counter()
    if os.path.exists(path):
        with open(path, encoding="utf-8") as file:
            for line in file:
                if line.strip() and not line.startswith("#"):
                    count, key = line.rstrip("\n").split("\t")
                    counted[key] = int(count)
    return counted


def check(title, clones, baseline_path, hint):
    """Says what differs from the baseline; True when nothing does."""
    current = groups(clones)
    baseline = read_baseline(baseline_path)
    grown = [(key, baseline[key], count) for key, count in sorted(current.items()) if count > baseline[key]]
    shrunk = [(key, count, current[key]) for key, count in sorted(baseline.items()) if current[key] < count]
    for key, was, now in grown:
        print(f"new copy of code ({was} -> {now} groups): {key}")
    for key, was, now in shrunk:
        print(f"copy removed ({was} -> {now} groups), write the baseline down: {key}")
    if grown:
        print("\nOne piece of knowledge lives in one place (CLAUDE.md): call the existing code instead of")
        print("copying it. See the copies with:")
        print("  " + hint)
    if shrunk:
        print(f"\nRun python3 scripts/dupl_check.py --update and commit {os.path.relpath(baseline_path, ROOT)}.")
    if not grown and not shrunk:
        print(f"{title}: {sum(current.values())} groups, none new")
    return not grown and not shrunk


def main():
    for lines, expected in CONSTANT_LIST_SAMPLES:
        if constant_list(lines) != expected:
            sys.exit(f"dupl_check: constant_list takes {lines} for {'no ' if expected else ''}list of constants")
    arguments = sys.argv[1:]
    if "--near" in arguments:
        near("--all" in arguments)
        return
    if "--update" in arguments:
        for title, clones, path, _ in CHECKS:
            current = groups(clones)
            with open(path, "w", encoding="utf-8") as file:
                file.write("# Written by scripts/dupl_check.py --update. Shrinks only.\n")
                for key in sorted(current):
                    file.write(f"{current[key]}\t{key}\n")
            print(f"{title} baseline: {sum(current.values())} groups written")
        return
    passed = [check(*each) for each in CHECKS]
    if not all(passed):
        sys.exit(1)


if __name__ == "__main__":
    main()
