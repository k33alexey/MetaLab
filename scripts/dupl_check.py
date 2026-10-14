#!/usr/bin/env python3
"""Refuses a new copy of Go code: CLAUDE.md, "Одно знание — одно место".

dupl finds groups of copied code. The groups that existed when the check was
introduced are written down in scripts/dupl-baseline.txt, keyed by the files
the copies lie in rather than by lines, so that an edit above a copy does not
count as a new one. The check fails when a group appears that the baseline
does not have, and also when a group from the baseline is gone: the baseline
only shrinks, and it is written down shrunk by --update in the same commit
that removed the copy. docs/duplicates-audit.md says what each group is.

--near is for the end of an iteration and never fails: it lists the smaller
copies, down to NEAR tokens, that touch a file changed since HEAD, so that
two functions differing only in what they return are seen while they are
being written. Each one is either merged or said in the report why it is not
one piece of knowledge.

    python3 scripts/dupl_check.py            # check
    python3 scripts/dupl_check.py --update   # write the baseline anew
    python3 scripts/dupl_check.py --near     # near copies in changed files
"""

import collections
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASELINE = os.path.join(ROOT, "scripts", "dupl-baseline.txt")
DUPL = ["go", "run", "github.com/mibk/dupl@v1.1.0"]
# 60 tokens is about eight lines of Go: it still sees two functions that differ
# only in what they return, which 80 let through (owner, 13.10.2026).
THRESHOLD = "60"
# Below 50 dupl reports error handling and field-by-field copying of structs.
NEAR = "50"


def sources():
    found = []
    for top in ("internal", "cmd"):
        for directory, _, names in os.walk(os.path.join(ROOT, top)):
            for name in names:
                if name.endswith(".go") and not name.endswith("_test.go"):
                    found.append(os.path.relpath(os.path.join(directory, name), ROOT))
    return sorted(found)


def clones(threshold):
    """Each group of copies as a list of "file:from,to"."""
    run = subprocess.run(DUPL + ["-t", threshold, "-files"], cwd=ROOT, input="\n".join(sources()),
                         capture_output=True, text=True)
    if run.returncode != 0:
        sys.exit("dupl failed:\n" + run.stderr)
    found, current = [], []
    for line in run.stdout.splitlines() + [""]:
        if line.startswith("  "):
            current.append(line.strip())
        elif current:
            found.append(current)
            current = []
    return found


def groups():
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


def near():
    touched = changed()
    found = [group for group in clones(NEAR) if any(place.rsplit(":", 1)[0] in touched for place in group)]
    for group in found:
        print("near copies:")
        for place in group:
            print("  " + place)
    print(f"dupl -t {NEAR}: {len(found)} groups touch the changed files; merge each, or say in the report why it is not one piece of knowledge")


def read_baseline():
    counted = collections.Counter()
    if os.path.exists(BASELINE):
        with open(BASELINE, encoding="utf-8") as file:
            for line in file:
                if line.strip() and not line.startswith("#"):
                    count, key = line.rstrip("\n").split("\t")
                    counted[key] = int(count)
    return counted


def main():
    if "--near" in sys.argv[1:]:
        near()
        return
    current = groups()
    if "--update" in sys.argv[1:]:
        with open(BASELINE, "w", encoding="utf-8") as file:
            file.write("# Written by scripts/dupl_check.py --update. Shrinks only.\n")
            for key in sorted(current):
                file.write(f"{current[key]}\t{key}\n")
        print(f"dupl baseline: {sum(current.values())} groups written")
        return
    baseline = read_baseline()
    grown = [(key, baseline[key], count) for key, count in sorted(current.items()) if count > baseline[key]]
    shrunk = [(key, count, current[key]) for key, count in sorted(baseline.items()) if current[key] < count]
    for key, was, now in grown:
        print(f"new copy of code ({was} -> {now} groups): {key}")
    for key, was, now in shrunk:
        print(f"copy removed ({was} -> {now} groups), write the baseline down: {key}")
    if grown:
        print("\nOne piece of knowledge lives in one place (CLAUDE.md): call the existing code instead of")
        print("copying it. See the copies with:")
        print(f"  find internal cmd -name '*.go' ! -name '*_test.go' | {' '.join(DUPL)} -t {THRESHOLD} -files")
    if shrunk:
        print("\nRun python3 scripts/dupl_check.py --update and commit scripts/dupl-baseline.txt.")
    if grown or shrunk:
        sys.exit(1)
    print(f"dupl: {sum(current.values())} groups, none new")


if __name__ == "__main__":
    main()
