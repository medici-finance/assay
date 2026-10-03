#!/usr/bin/env bash
# Offline test for the "Request auto-merge" STEP of the evidence-automerge
# workflow — the shell around automerge-refusal.sh, not the decision inside it
# (that is automerge-refusal_test.sh). No GitHub, no token, no runner: the step's
# `run:` script is extracted from the staged workflow, `gh` is a stub on PATH, and
# the script is executed under `bash -e {0}` exactly as Actions runs a step that
# declares no `shell:` key.
#
# THE DEFECT (#2019, #2021). Under `bash -e`, `out="$(cmd)"` with a failing `cmd`
# exits the step on that line, so the `rc=$?` that follows is never read and the
# refusal classifier never runs: the step goes red with no classifier output, even
# for refusals the classifier calls benign. THE CLASS: a command-substitution
# assignment whose status is read by a later `$?` instead of being captured in
# the same AND-OR list (`… && rc=0 || rc=$?`) or inside a `set +e` region.
#
# FOUR PARTS
#   1. behaviour  — the real step, five stubbed GitHub answers, each must reach the
#                   classifier and exit the way the classifier decides.
#   2. mutation   — the same step with the fix reverted on the fly must FAIL part 1
#                   (so the part-1 cases really observe the defect).
#   3. class guard — a scan of every `run:` block in the staged / activation
#                   workflow copies for the defect shape; clean on the real files.
#   4. positive control — the scan must flag a PLANTED second instance (a step the
#                   fix does not touch) and the reverted fix, naming the step.
#
#   ./automerge-step_test.sh                 # all green
#   STEP_YML=<path-to-pre-fix-yml> ./automerge-step_test.sh   # RED (S2-S5) against a pre-fix copy
#
# Exit 0 when every check matches, 1 otherwise. Needs bash and python3.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
STEP_YML="${STEP_YML:-$root/ci/staged-workflows/evidence-automerge.yml}"
STEP_NAME="Request auto-merge"

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/automerge-step-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"

# extract_step <yml> <step-name> -> the step's `run: |` body, de-indented, on stdout.
extract_step() {
  python3 - "$1" "$2" <<'PY'
import re, sys
lines = open(sys.argv[1]).read().split("\n")
want = sys.argv[2]
i = 0
while i < len(lines):
    m = re.match(r"^(\s*)- name:\s*(.+?)\s*$", lines[i])
    if m and m.group(2).strip("'\"") == want:
        step_indent = len(m.group(1))
        j = i + 1
        while j < len(lines):
            r = re.match(r"^(\s*)run:\s*\|[-+]?\s*$", lines[j])
            if r:
                break
            n = re.match(r"^(\s*)- ", lines[j])
            if n and len(n.group(1)) <= step_indent:
                sys.exit("no run: block in step %r" % want)
            j += 1
        else:
            sys.exit("no run: block in step %r" % want)
        run_indent = len(r.group(1))
        body = []
        for k in range(j + 1, len(lines)):
            ln = lines[k]
            if ln.strip() == "":
                body.append("")
                continue
            ind = len(ln) - len(ln.lstrip())
            if ind <= run_indent:
                break
            body.append(ln[run_indent + 2:])
        print("\n".join(body).rstrip("\n"))
        sys.exit(0)
    i += 1
sys.exit("step %r not found" % want)
PY
}

# The stub `gh`. GH_MODE picks what GitHub "answers"; the mutation is always a
# NON-ZERO exit for refusals, which is the whole point.
cat > "$WORK/bin/gh" <<'STUB'
#!/usr/bin/env bash
case "$1 $2" in
  "api graphql")
    case "$GH_MODE" in
      ok)       echo '{"data":{"enablePullRequestAutoMerge":{"pullRequest":{"number":1}}}}'; exit 0 ;;
      unstable) echo 'gh: Pull request is in unstable status (enablePullRequestAutoMerge)'; exit 1 ;;
      clean)    echo 'gh: Pull request is in clean status (enablePullRequestAutoMerge)'; exit 1 ;;
      perm)     echo 'gh: Resource not accessible by integration (enablePullRequestAutoMerge)'; exit 1 ;;
    esac ;;
  "pr view")
    if [ "$GH_ROLLUP" = other ]; then
      echo '{"statusCheckRollup":[{"__typename":"CheckRun","name":"enable","status":"COMPLETED","conclusion":"FAILURE"},{"__typename":"CheckRun","name":"build-test","status":"COMPLETED","conclusion":"FAILURE"}]}'
    else
      echo '{"statusCheckRollup":[{"__typename":"CheckRun","name":"enable","status":"COMPLETED","conclusion":"FAILURE"},{"__typename":"CheckRun","name":"build-test","status":"COMPLETED","conclusion":"SUCCESS"}]}'
    fi
    exit 0 ;;
esac
echo "stub gh: unexpected call: $*" >&2
exit 99
STUB
chmod +x "$WORK/bin/gh"

# run_step <script> <gh-mode> <rollup> -> sets STEP_RC and STEP_LOG.
# `bash -e` is what Actions uses for a step with no `shell:`; cwd is the repo root
# because the step calls tools/evidence-automerge/automerge-refusal.sh relatively.
run_step() {
  local script="$1" mode="$2" rollup="$3"
  mkdir -p "$WORK/tmp"
  STEP_LOG="$(cd "$root" && env PATH="$WORK/bin:$PATH" GH_MODE="$mode" GH_ROLLUP="$rollup" \
      RUNNER_TEMP="$WORK/tmp" GH_TOKEN=x PR_NODE_ID=node PR=1 REPO=o/r SELF_CHECK=enable \
      bash -e "$script" 2>&1)"
  STEP_RC=$?
}

# expect_case <script> <label> <mode> <rollup> <want-exit> <want-log-regex>
# Returns 0 on match; prints nothing (callers decide how to report).
expect_case() {
  local script="$1" mode="$3" rollup="$4" want="$5" re="$6"
  run_step "$script" "$mode" "$rollup"
  [ "$STEP_RC" = "$want" ] && printf '%s' "$STEP_LOG" | grep -Eq -- "$re"
}

# fail_rec <label> -> one failure record on fd 3: the label, the step's exit status
# and what it printed (empty output is the signature of the defect).
fail_rec() {
  local first
  first="$(printf '%s' "$STEP_LOG" | head -1)"
  echo "$1 -> step exit $STEP_RC, output: ${first:-<none>}" >&3
}

# suite <script> -> number of failing behaviour cases (stdout), cases reported to fd 3.
suite() {
  local script="$1" n=0
  expect_case "$script" S1 ok        none  0 'auto-merge requested on PR 1'                  || { fail_rec "S1 accepted"; n=$((n+1)); }
  expect_case "$script" S2 unstable  none  0 'Benign; see #586'                               || { fail_rec "S2 enable-only unstable skips"; n=$((n+1)); }
  expect_case "$script" S3 clean     none  0 "refused as 'clean status'"                      || { fail_rec "S3 clean status skips"; n=$((n+1)); }
  expect_case "$script" S4 perm      none  1 '::error::could not enable auto-merge on PR 1'  || { fail_rec "S4 permission refusal reds with classifier output"; n=$((n+1)); }
  expect_case "$script" S5 unstable  other 1 "failing besides 'enable'"                       || { fail_rec "S5 other check red reds with classifier output"; n=$((n+1)); }
  echo "$n"
}

# ── 1. behaviour against the real step ────────────────────────────────────────
if ! extract_step "$STEP_YML" "$STEP_NAME" > "$WORK/step.sh" 2> "$WORK/extract.err"; then
  bad "extract the '$STEP_NAME' step from $STEP_YML ($(cat "$WORK/extract.err"))"
  echo; echo "passed: $pass  failed: $fail"; exit 1
fi
ok "extracted the '$STEP_NAME' step ($(wc -l < "$WORK/step.sh" | tr -d ' ') lines) from ${STEP_YML#"$root"/}"

exec 3> "$WORK/failed.txt"
real_fails="$(suite "$WORK/step.sh")"
exec 3>&-
if [ "$real_fails" = 0 ]; then
  ok "S1-S5 the step reaches the classifier under bash -e for every stubbed GitHub answer"
else
  while read -r c; do bad "$c"; done < "$WORK/failed.txt"
fi

# ── 2. mutation: revert the fix on the fly; part 1 must go red ───────────────
python3 - "$WORK/step.sh" "$WORK/step.mut.sh" <<'PY'
import re, sys
src = open(sys.argv[1]).read()
mut, n = re.subn(r'(\)"|\))\s*&&\s*rc=0\s*\|\|\s*rc=\$\?', r'\1\nrc=$?', src, count=1)
if n != 1:
    sys.exit("mutation did not apply: no `&& rc=0 || rc=$?` capture in the step")
open(sys.argv[2], "w").write(mut)
PY
if [ $? -ne 0 ]; then
  # A pre-fix copy has nothing to revert; it is then already the mutant.
  cp "$WORK/step.sh" "$WORK/step.mut.sh"
  echo 'note - step carries no "&& rc=0 || rc=$?" capture; treating it as its own mutant'
fi
exec 3> "$WORK/failed.mut.txt"
mut_fails="$(suite "$WORK/step.mut.sh")"
exec 3>&-
if [ "$mut_fails" -gt 0 ] 2>/dev/null; then
  ok "MUTANT (fix reverted: assignment then rc=\$?) is caught: $mut_fails of 5 cases red ($(cut -d' ' -f1 "$WORK/failed.mut.txt" | paste -sd, -))"
else
  bad "MUTANT (fix reverted) was NOT caught: part-1 cases stayed green against the pre-fix shape"
fi

# ── 3 + 4. class guard over every run: block, with positive controls ─────────
cat > "$WORK/guard.py" <<'PY'
"""Flag a `run:` block that reads `$?` from a command-substitution assignment
while errexit may still be on: the assignment itself would end the step first.
Usage: guard.py FILE...   Prints `FILE: step 'NAME' line N: ...`; exit 1 on any."""
import re, sys

ASSIGN = re.compile(r'^\s*[A-Za-z_]\w*=("?)\$\(')            # x=$(  /  x="$(
RC_LINE = re.compile(r'^\s*[A-Za-z_]\w*="?\$\?"?\s*(?:;.*)?$')  # rc=$?
ONE_LINE = re.compile(r'[A-Za-z_]\w*="?\$\(.*\)"?\s*;\s*[A-Za-z_]\w*="?\$\?')
SET_OFF = re.compile(r'^\s*set\s+\+[a-z]*e')
SET_ON = re.compile(r'^\s*set\s+-[a-z]*e')

def blocks(text):
    lines = text.split("\n")
    name = "(unnamed)"
    i = 0
    while i < len(lines):
        n = re.match(r'^\s*- name:\s*(.+?)\s*$', lines[i])
        if n:
            name = n.group(1).strip("'\"")
        r = re.match(r'^(\s*)run:\s*[|>][-+]?\s*$', lines[i])
        if r:
            ind = len(r.group(1))
            body, start = [], i + 2
            k = i + 1
            while k < len(lines) and (lines[k].strip() == "" or len(lines[k]) - len(lines[k].lstrip()) > ind):
                body.append(lines[k]); k += 1
            yield name, start, body
            i = k
            continue
        i += 1

def scan(path):
    found = []
    for name, start, body in blocks(open(path).read()):
        errexit_off = False
        last = None   # index of the previous non-blank, non-comment line
        for idx, ln in enumerate(body):
            s = ln.strip()
            if not s or s.startswith("#"):
                continue
            if SET_OFF.match(ln):
                errexit_off = True
            elif SET_ON.match(ln):
                errexit_off = False
            if not errexit_off:
                hit = False
                if ONE_LINE.search(ln):
                    hit = True
                elif RC_LINE.match(ln) and last is not None:
                    prev = body[last].rstrip()
                    if prev.endswith(')"') or prev.endswith(")"):
                        back = body[max(0, last - 12):last + 1]
                        hit = any(ASSIGN.match(b) for b in back)
                if hit:
                    found.append("%s: step '%s' line %d: %s" % (path, name, start + idx, s))
            last = idx
    return found

hits = []
for p in sys.argv[1:]:
    hits += scan(p)
for h in hits:
    print(h)
sys.exit(1 if hits else 0)
PY

# The staged lane's own file and the promote-candidate copy, plus every other
# staged workflow: any new `x="$(…)"; rc=$?` anywhere in them is red.
guard_files=()
for f in "$root"/ci/staged-workflows/*.yml "$root"/tools/ci-load/activation/*.yml; do
  [ -f "$f" ] && guard_files+=("$f")
done
guard_out="$(python3 "$WORK/guard.py" "${guard_files[@]}" 2>&1)"; guard_rc=$?
if [ "$guard_rc" = 0 ]; then
  ok "CLASS GUARD: no bash -e capture-then-\$? shape in ${#guard_files[@]} staged/activation workflow files"
else
  bad "CLASS GUARD: capture-then-\$? shape found"
  printf '%s\n' "$guard_out" | sed 's/^/       | /'
fi

# Positive control 1: a PLANTED second instance in a step this fix never touched.
cat > "$WORK/planted.yml" <<'YML'
jobs:
  other:
    steps:
      - name: Some other step
        run: |
          set -uo pipefail
          answer="$(curl -fsS https://example.invalid/x 2>&1)"
          rc=$?
          echo "rc=$rc"
      - name: Scoped capture is fine
        run: |
          set +e
          answer="$(false)"
          rc=$?
          set -e
      - name: Or-list capture is fine
        run: |
          answer="$(false)" || rc=$?
YML
planted_out="$(python3 "$WORK/guard.py" "$WORK/planted.yml" 2>&1)"; planted_rc=$?
if [ "$planted_rc" = 1 ] && printf '%s' "$planted_out" | grep -q "step 'Some other step'" \
   && ! printf '%s' "$planted_out" | grep -Eq "Scoped capture|Or-list capture"; then
  ok "POSITIVE CONTROL: guard flags a planted second instance, naming 'Some other step', and passes the two safe shapes"
else
  bad "POSITIVE CONTROL: guard did not flag exactly the planted instance (exit $planted_rc): $planted_out"
fi

# Positive control 2: the reverted fix, in the real file's own text.
python3 - "$STEP_YML" "$WORK/reverted.yml" <<'PY'
import re, sys
src = open(sys.argv[1]).read()
mut, n = re.subn(r'(\)"|\))\s*&&\s*rc=0\s*\|\|\s*rc=\$\?', lambda m: m.group(1) + '\n          rc=$?', src, count=1)
open(sys.argv[2], "w").write(mut if n else src)
PY
rev_out="$(python3 "$WORK/guard.py" "$WORK/reverted.yml" 2>&1)"; rev_rc=$?
if [ "$rev_rc" = 1 ] && printf '%s' "$rev_out" | grep -q "step 'Request auto-merge'"; then
  ok "POSITIVE CONTROL: guard flags the reverted fix in the real workflow text"
else
  bad "POSITIVE CONTROL: guard missed the reverted fix (exit $rev_rc): $rev_out"
fi

echo
echo "passed: $pass  failed: $fail  (step: ${STEP_YML#"$root"/})"
[ "$fail" -eq 0 ]
