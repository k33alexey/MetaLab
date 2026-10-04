#!/bin/bash
# Mutation pass (gremlins) over one Go package, run so that it finishes on a
# disk of ordinary size, does not overload the machine and loses nothing.
#
#   scripts/mutants.sh start <package> [--diff <commit>] [-- <gremlins flags>]
#   scripts/mutants.sh status [<run dir>]
#   scripts/mutants.sh stop   [<run dir>]
#
# Rule and when to run it: CLAUDE.md, "Мутационный прогон".
#
# Why a script and not a bare `gremlins unleash` — three lessons of the pass
# over internal/metadata on 04.10.2026:
#   - every mutant is built anew and its cache entry is never needed again;
#     the cache grows by about half a gigabyte a minute and runs the disk out;
#   - six workers on eight cores, each test taking every core, put the load at
#     37; tests stopped fitting the limit without hanging;
#   - a surviving mutant runs the whole suite, so under load it is the first
#     to time out, and gremlins counts TIMED OUT as caught: 340 of 369 were
#     alive. Every TIMED OUT is therefore rechecked on a quiet machine.
#
# What the script does:
#   - freezes HEAD by `git clone` (uncommitted changes in the package are
#     refused: they would not be measured);
#   - gives the run its own GOCACHE and TMPDIR and warms the cache first, so
#     dependencies are built once and fall under the base mark;
#   - WORKERS gremlins workers, TEST_CPU cores per test;
#   - timestamps every line of the gremlins log;
#   - every TRIM_EVERY seconds, at background IO priority, removes cache entries
#     and build/test temp dirs of finished mutants (older than TRIM_AGE minutes
#     and newer than the base mark);
#   - stops gremlins if free space drops below MIN_FREE_GB and says the result
#     is incomplete;
#   - rechecks every TIMED OUT (scripts/mutants_recheck.py) and writes
#     lived.tsv — the list to work through;
#   - with --diff, runs only the mutants on lines changed since the commit:
#     gremlins lists them in a dry run, mutants_recheck.py runs them;
#   - keeps the log, the JSON, the frozen tree and the summary; removes caches;
#   - when done, writes `done` and posts a macOS notification.
#
# While the pass runs, no other tests are run on the machine.

set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASE="${ML_MUTANTS_BASE:-${TMPDIR:-/tmp}/ml-mutants}"
WORKERS="${WORKERS:-3}"
TEST_CPU="${TEST_CPU:-2}"
TRIM_EVERY="${TRIM_EVERY:-120}"
TRIM_AGE="${TRIM_AGE:-15}"
MIN_FREE_GB="${MIN_FREE_GB:-20}"
RECHECK_TIMEOUT="${RECHECK_TIMEOUT:-90}"
GREMLINS="${GREMLINS:-$HOME/go/bin/gremlins}"

die() { echo "mutants: $*" >&2; exit 1; }

free_gb() { df -g "$1" | awk 'NR==2{print $4}'; }

outcomes() {
	grep -oE '(KILLED|LIVED|TIMED OUT|NOT COVERED|NOT VIABLE|SKIPPED) ' "$1" 2>/dev/null |
		sort | uniq -c
}

latest_run() {
	ls -d "$BASE"/run-* 2>/dev/null | sort | tail -1
}

notify() {
	command -v osascript >/dev/null && osascript -e "display notification \"$1\" with title \"MetaLab mutants\"" 2>/dev/null
	true
}

# Runs detached: warm-up, gremlins, the trimmer and guard, the recheck.
run() {
	local R="$1" pkg="$2"; shift 2
	export GOCACHE="$R/gocache" TMPDIR="$R/gtmp"
	unset ML_TEST_DATABASE_URL ML_TEST_ADMIN_DATABASE_URL
	mkdir -p "$GOCACHE" "$TMPDIR"
	cd "$R/tree" || exit 1

	date '+%F %T started' > "$R/summary.txt"
	# The same build and vet a mutant gets, minus the mutation.
	go test -count=1 -run '^$' "$pkg" > "$R/warm.log" 2>&1 || { echo "warm-up failed, see warm.log" >> "$R/summary.txt"; cp "$R/summary.txt" "$R/done"; notify "warm-up failed"; return; }
	touch "$R/base.mark"

	(
		# With --diff gremlins only lists the mutants; mutants_recheck.py runs
		# those on changed lines (gremlins' own --diff never matches a package).
		"$GREMLINS" unleash "$pkg" ${DIFF:+-d} --workers "$WORKERS" --test-cpu "$TEST_CPU" -o "$R/gremlins.json" ${@+"$@"} 2>&1
		echo "EXIT $?"
	) | while IFS= read -r line; do printf '%s %s\n' "$(date +%T)" "$line"; done > "$R/gremlins.log" &
	local pipe=$!

	(
		local trim="taskpolicy -b" before after
		command -v taskpolicy >/dev/null || trim="nice -n 19"
		while kill -0 "$pipe" 2>/dev/null; do
			sleep "$TRIM_EVERY"
			before=$(free_gb "$R")
			$trim find "$GOCACHE" -type f -newer "$R/base.mark" -mmin +"$TRIM_AGE" -delete 2>/dev/null
			$trim find "$TMPDIR" -mindepth 1 -maxdepth 2 \( -name 'go-build*' -o -name 'Test*' \) \
				-mmin +"$TRIM_AGE" -exec rm -rf {} + 2>/dev/null
			after=$(free_gb "$R")
			printf '%s free %sG -> %sG cache %sM results %s\n' "$(date +%T)" "$before" "$after" \
				"$(du -sm "$GOCACHE" 2>/dev/null | cut -f1)" \
				"$(grep -cE 'KILLED|LIVED|TIMED OUT|NOT COVERED|NOT VIABLE' "$R/gremlins.log")" >> "$R/trim.log"
			if [ "$after" -lt "$MIN_FREE_GB" ]; then
				echo "$(date +%T) STOPPED: free ${after}G below ${MIN_FREE_GB}G, result incomplete" >> "$R/trim.log"
				pkill -f "gremlins unleash.*$R"
				pkill -f "$R/gtmp"
				break
			fi
		done
	) &
	local guard=$!

	wait "$pipe"
	{ kill "$guard" && wait "$guard"; } 2>/dev/null
	rm -rf "$GOCACHE" "$TMPDIR"
	{
		date '+%F %T gremlins finished'
		grep -oE 'EXIT [0-9]+' "$R/gremlins.log" | tail -1
		grep -q STOPPED "$R/trim.log" 2>/dev/null && echo "INCOMPLETE: stopped on low disk"
		# A dry run lists the whole package; the changed lines are counted below.
		[ -n "${DIFF:-}" ] || outcomes "$R/gremlins.log"
	} >> "$R/summary.txt"

	if [ -f "$R/gremlins.json" ]; then
		python3 "$ROOT/scripts/mutants_recheck.py" "$R" "${pkg#./}" --timeout "$RECHECK_TIMEOUT" \
			--workers "$WORKERS" ${DIFF:+--diff "$DIFF"} \
			> "$R/recheck.log" 2>&1 || echo "recheck failed, see recheck.log" >> "$R/summary.txt"
	fi
	date '+%F %T finished' >> "$R/summary.txt"
	cp "$R/summary.txt" "$R/done"
	notify "pass over $pkg finished: $(grep -oE 'LIVED after recheck: [0-9]+' "$R/summary.txt")"
}

cmd="${1:-}"; shift || true
case "$cmd" in
start)
	[ $# -gt 0 ] || die "start <package> [--diff <commit>] [-- <gremlins flags>]"
	pkg="$1"; shift
	case "$pkg" in ./*) ;; *) pkg="./$pkg" ;; esac
	extra=() DIFF=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--diff) [ $# -ge 2 ] || die "--diff needs a commit"; DIFF="$2"; shift 2 ;;
		--) shift; extra+=("$@"); break ;;
		*) die "unknown argument: $1" ;;
		esac
	done
	[ -x "$GREMLINS" ] || die "gremlins not found at $GREMLINS"
	pgrep -f "gremlins unleash" >/dev/null && die "a gremlins pass is already running"
	[ -d "$ROOT/$pkg" ] || die "no package $pkg"
	# The clone takes HEAD only; an uncommitted change in the package would
	# silently not be measured.
	[ -z "$(git -C "$ROOT" status --porcelain -- "$pkg")" ] || die "$pkg has uncommitted changes; commit first"
	R="$BASE/run-$(date +%Y%m%d-%H%M%S)"
	mkdir -p "$R"
	git clone --quiet --local --no-hardlinks "$ROOT" "$R/tree" || die "cloning the tree failed"
	git -C "$R/tree" checkout --quiet "$(git -C "$ROOT" rev-parse HEAD)" || die "checkout failed"
	{
		git -C "$ROOT" log -1 --format='%h %s'
		echo "package: $pkg"
		[ -n "$DIFF" ] && echo "diff: lines changed since $DIFF ($(git -C "$ROOT" rev-parse --short "$DIFF"))"
		echo "flags: workers $WORKERS, test-cpu $TEST_CPU ${extra[*]+${extra[*]}}"
		echo "free before: $(free_gb "$R")G"
	} > "$R/commit.txt"
	git -C "$ROOT" rev-parse --verify --quiet "$DIFF^{commit}" >/dev/null || [ -z "$DIFF" ] || die "no commit $DIFF"
	DIFF="${DIFF:+$(git -C "$ROOT" rev-parse "$DIFF")}" nohup "$0" __run "$R" "$pkg" ${extra[@]+"${extra[@]}"} > "$R/runner.log" 2>&1 &
	echo "$R"
	;;
__run)
	run "$@"
	;;
status)
	R="${1:-$(latest_run)}"; [ -n "$R" ] && [ -d "$R" ] || die "no run"
	echo "$R"; cat "$R/commit.txt"
	if [ -f "$R/done" ]; then cat "$R/done"; exit 0; fi
	tail -1 "$R/trim.log" 2>/dev/null
	tail -1 "$R/gremlins.log" 2>/dev/null
	outcomes "$R/gremlins.log"
	[ -f "$R/recheck.log" ] && echo "rechecking TIMED OUT: $(wc -l < "$R/recheck.log") done"
	;;
stop)
	R="${1:-$(latest_run)}"; [ -n "$R" ] && [ -d "$R" ] || die "no run"
	pkill -f "mutants.sh __run $R"
	pkill -f "gremlins unleash.*$R"
	pkill -f "mutants_recheck.py $R"
	pkill -f "$R/"
	echo "stopped: $R"
	;;
*)
	sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
