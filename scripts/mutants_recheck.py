#!/usr/bin/env python3
"""Re-runs the TIMED OUT mutants of a gremlins pass on a quiet machine, or
runs the mutants of changed lines after a gremlins dry run.

    mutants_recheck.py <run dir> <package dir> [--workers N] [--timeout S]
    mutants_recheck.py <run dir> <package dir> --diff <commit> [...]

The second form exists because `gremlins --diff` (0.6.0) keys the changed
files by their path from the repository root and the mutants by their path
from the package, so on a single package nothing ever matches and every
mutant is SKIPPED. Here gremlins only lists the mutants (`-d`), the changed
lines are taken from `git diff -U0 <commit>`, and the runner below runs them.

Why: under load a mutant that survives runs the whole suite and is the first
to miss the limit, so gremlins reports it TIMED OUT and counts it as caught.
On 04.10.2026 340 of 369 such mutants turned out to be alive.

Each mutant is applied to a worker's own copy of the frozen tree, the package
tests are run with a generous limit, the file is restored from the frozen
original and checked by hash. Outcomes: KILLED, LIVED, HANG (still over the
limit), NOT-VIABLE (does not build). Writes recheck.tsv, lived.tsv (gremlins
LIVED plus rechecked LIVED, with the source line) and appends to summary.txt.
"""

import argparse
import collections
import concurrent.futures
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time

# The operator text gremlins replaces, per mutation kind.
TABLE = {
    "CONDITIONALS_BOUNDARY": {"<=": "<", ">=": ">", "<": "<=", ">": ">="},
    "CONDITIONALS_NEGATION": {"==": "!=", "!=": "==", "<=": ">", ">=": "<", "<": ">=", ">": "<="},
    "ARITHMETIC_BASE": {"+": "-", "-": "+", "*": "/", "/": "*", "%": "*"},
    "INCREMENT_DECREMENT": {"++": "--", "--": "++"},
    "INVERT_NEGATIVES": {"-": "+"},
}


def mutate(path, line, col, kind):
    with open(path, "rb") as f:
        lines = f.read().split(b"\n")
    text = lines[line - 1]
    at = col - 1  # gremlins columns are 1-based bytes
    # Longest operator first, so "<=" is not read as "<".
    for op in sorted(TABLE[kind], key=len, reverse=True):
        if text[at:at + len(op)] == op.encode():
            lines[line - 1] = text[:at] + TABLE[kind][op].encode() + text[at + len(op):]
            with open(path, "wb") as f:
                f.write(b"\n".join(lines))
            return
    raise ValueError(f"no {kind} operator at {line}:{col}: {text.decode(errors='replace').strip()}")


def changed_lines(tree, package, commit):
    """Lines added or changed since commit, per file name within the package."""
    out = subprocess.run(["git", "diff", "-U0", commit, "--", package], cwd=tree,
                         capture_output=True, text=True, check=True).stdout
    changed, name = {}, None
    for line in out.splitlines():
        if line.startswith("+++ "):
            path = line[4:]
            name = None if path == "/dev/null" else os.path.relpath(path[2:], package)
        elif line.startswith("@@") and name:
            new = line.split()[2]  # +start[,count]
            start, _, count = new[1:].partition(",")
            count = int(count) if count else 1
            changed.setdefault(name, set()).update(range(int(start), int(start) + count))
    return changed


def digest(path):
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("run")
    ap.add_argument("package", help="package directory relative to the tree, e.g. internal/metadata")
    ap.add_argument("--workers", type=int, default=3)
    ap.add_argument("--timeout", type=int, default=90)
    ap.add_argument("--diff", help="run the RUNNABLE mutants of lines changed since this commit")
    a = ap.parse_args()

    tree = os.path.join(a.run, "tree")
    with open(os.path.join(a.run, "gremlins.json")) as f:
        report = json.load(f)
    if a.diff:
        changed = changed_lines(tree, a.package, a.diff)
        for m_file in report["files"]:
            m_file["mutations"] = [m for m in m_file["mutations"]
                                   if m["line"] in changed.get(m_file["file_name"], ())]
            for m in m_file["mutations"]:
                if m["status"] == "RUNNABLE":
                    m["status"] = "TIMED OUT"  # run below exactly like a recheck
                    m["diff"] = True
        report["files"] = [f for f in report["files"] if f["mutations"]]
    todo = [(m_file["file_name"], m["line"], m["column"], m["type"])
            for m_file in report["files"] for m in m_file["mutations"] if m["status"] == "TIMED OUT"]

    # Own cache and temp dir: the pass's TMPDIR is removed before this runs,
    # and a `go test` that cannot start must not read as a killed mutant.
    env = dict(os.environ, GOFLAGS="-p=2", GOCACHE=os.path.join(a.run, "recheck-cache"),
               TMPDIR=os.path.join(a.run, "recheck-tmp"))
    os.makedirs(env["TMPDIR"], exist_ok=True)
    env.pop("ML_TEST_DATABASE_URL", None)
    env.pop("ML_TEST_ADMIN_DATABASE_URL", None)
    copies = []
    for w in range(a.workers):
        dst = os.path.join(a.run, f"recheck-w{w}")
        shutil.rmtree(dst, ignore_errors=True)
        shutil.copytree(tree, dst, symlinks=True, ignore=shutil.ignore_patterns(".git"))
        copies.append(dst)
    free = list(copies)

    def one(item):
        name, line, col, kind = item
        work = free.pop()
        try:
            orig = os.path.join(tree, a.package, name)
            dst = os.path.join(work, a.package, name)
            try:
                mutate(dst, line, col, kind)
            except ValueError as e:
                return item, "APPLY-FAILED", 0, str(e)
            start = time.time()
            p = subprocess.run(["go", "test", "-count=1", "-failfast", "-cpu", "2",
                                f"-timeout={a.timeout}s", "./" + a.package],
                               cwd=work, env=env, capture_output=True, text=True)
            took = round(time.time() - start)
            out = p.stdout + p.stderr
            if p.returncode == 0:
                res = "LIVED"
            elif "test timed out" in out:
                res = "HANG"
            elif "--- FAIL" in out or "\npanic: " in "\n" + out:
                res = "KILLED"
            elif "[build failed]" in out:
                res = "NOT-VIABLE"
            else:
                # go test did not get as far as the tests: not an outcome.
                res = "ERROR"
                note = out.strip().splitlines()[0] if out.strip() else f"exit {p.returncode}"
            shutil.copyfile(orig, dst)
            if digest(orig) != digest(dst):
                raise SystemExit(f"restore failed: {dst}")
            return item, res, took, note if res == "ERROR" else ""
        finally:
            free.append(work)

    results = []
    with concurrent.futures.ThreadPoolExecutor(a.workers) as ex:
        for item, res, took, note in ex.map(one, todo):
            results.append((item, res))
            print(f"{item[0]}:{item[1]}:{item[2]}\t{item[3]}\t{res}\t{took}s\t{note}", flush=True)


    with open(os.path.join(a.run, "recheck.tsv"), "w") as o:
        o.write("file\tline\tcol\ttype\toutcome\n")
        for (name, line, col, kind), res in results:
            o.write(f"{name}\t{line}\t{col}\t{kind}\t{res}\n")

    rechecked = {item: r for item, r in results}
    lived = []
    for m_file in report["files"]:
        name = m_file["file_name"]
        path = os.path.join(tree, a.package, name)
        with open(path, encoding="utf-8", errors="replace") as f:
            src = f.read().split("\n")
        for m in m_file["mutations"]:
            st = m["status"]
            origin = "gremlins"
            if st == "TIMED OUT":
                st = rechecked.get((name, m["line"], m["column"], m["type"]), "?")
                origin = "diff" if m.get("diff") else "recheck"
            if st == "LIVED":
                lived.append((name, m["line"], m["column"], m["type"], origin, src[m["line"] - 1].strip()))
    with open(os.path.join(a.run, "lived.tsv"), "w") as o:
        o.write("file\tline\tcol\ttype\tsource\tcode\toutcome\n")
        for r in sorted(lived):
            o.write("\t".join(map(str, r)) + "\t\n")

    # gremlins reports a string + turned into - (it does not build) as KILLED;
    # take those out so the efficacy says what the tests catch.
    here = os.path.dirname(os.path.abspath(__file__))
    strplus = set(subprocess.run(["go", "run", "./scripts/mutants-strplus", os.path.join(tree, a.package)],
                                 cwd=os.path.dirname(here), env=env, capture_output=True, text=True,
                                 check=True).stdout.split())
    final = collections.Counter()
    for m_file in report["files"]:
        for m in m_file["mutations"]:
            key = (m_file["file_name"], m["line"], m["column"], m["type"])
            st = rechecked.get(key, m["status"]) if m["status"] == "TIMED OUT" else m["status"]
            if m["type"] == "ARITHMETIC_BASE" and f"{key[0]}:{key[1]}:{key[2]}" in strplus:
                st = "NOT-VIABLE"
            final["KILLED" if st == "HANG" else st] += 1
    caught, alive = final["KILLED"], final["LIVED"]

    counts = {}
    for _, r in results:
        counts[r] = counts.get(r, 0) + 1
    with open(os.path.join(a.run, "summary.txt"), "a") as o:
        if a.diff:
            nc = sum(1 for f in report["files"] for m in f["mutations"] if m["status"] == "NOT COVERED")
            o.write(f"changed lines since {a.diff}: NOT COVERED {nc}\n")
        o.write(f"{'run' if a.diff else 'recheck'} of {len(todo)} {'mutants' if a.diff else 'TIMED OUT'}: " +
                ", ".join(f"{k} {v}" for k, v in sorted(counts.items())) + "\n")
        o.write(f"LIVED after recheck: {len(lived)} (lived.tsv)\n")
        o.write("corrected: " + ", ".join(f"{k} {v}" for k, v in sorted(final.items())) +
                (f"; efficacy {100 * caught / (caught + alive):.1f}%" if caught + alive else "") +
                " (HANG counted as KILLED, string + mutants as NOT-VIABLE)\n")
        if counts.get("ERROR"):
            o.write(f"ERROR: {counts['ERROR']} mutants did not reach the tests, see recheck.log; result not valid\n")
    for c in copies:
        shutil.rmtree(c, ignore_errors=True)
    shutil.rmtree(env["GOCACHE"], ignore_errors=True)
    shutil.rmtree(env["TMPDIR"], ignore_errors=True)
    return 1 if counts.get("ERROR") else 0


if __name__ == "__main__":
    sys.exit(main())
