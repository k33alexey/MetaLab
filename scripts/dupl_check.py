#!/usr/bin/env python3
"""Refuses a new copy of Go code: CLAUDE.md, "Одно знание — одно место".

dupl finds groups of copied code. The groups that existed when the check was
introduced are written down in scripts/dupl-baseline.txt, keyed by the files
the copies lie in rather than by lines, so that an edit above a copy does not
count as a new one. The check fails when a group appears that the baseline
does not have, and also when a group from the baseline is gone: the baseline
only shrinks, and it is written down shrunk by --update in the same commit
that removed the copy. docs/duplicates-audit.md says what each group is.

    python3 scripts/dupl_check.py            # check
    python3 scripts/dupl_check.py --update   # write the baseline anew
"""

import collections
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASELINE = os.path.join(ROOT, "scripts", "dupl-baseline.txt")
DUPL = ["go", "run", "github.com/mibk/dupl@v1.1.0"]
# 80 tokens is about a dozen lines of Go: below it dupl reports the shape of
# an if err != nil block, above it what it reports is copied logic.
THRESHOLD = "80"


def sources():
    found = []
    for top in ("internal", "cmd"):
        for directory, _, names in os.walk(os.path.join(ROOT, top)):
            for name in names:
                if name.endswith(".go") and not name.endswith("_test.go"):
                    found.append(os.path.relpath(os.path.join(directory, name), ROOT))
    return sorted(found)


def groups():
    run = subprocess.run(DUPL + ["-t", THRESHOLD, "-files"], cwd=ROOT, input="\n".join(sources()),
                         capture_output=True, text=True)
    if run.returncode != 0:
        sys.exit("dupl failed:\n" + run.stderr)
    counted, current = collections.Counter(), []
    for line in run.stdout.splitlines() + [""]:
        if line.startswith("  "):
            current.append(line.strip().rsplit(":", 1)[0])
        elif current:
            counted[" ".join(sorted(current))] += 1
            current = []
    return counted


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
        print("  find internal cmd -name '*.go' ! -name '*_test.go' | go run github.com/mibk/dupl@v1.1.0 -t 80 -files")
    if shrunk:
        print("\nRun python3 scripts/dupl_check.py --update and commit scripts/dupl-baseline.txt.")
    if grown or shrunk:
        sys.exit(1)
    print(f"dupl: {sum(current.values())} groups, none new")


if __name__ == "__main__":
    main()
