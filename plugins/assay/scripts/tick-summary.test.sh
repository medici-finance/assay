#!/usr/bin/env bash
# tick-summary.test.sh — the proof that tick-summary.sh accepts exactly the
# summary lines references/tick-contract.md describes, and rejects the ones a
# loose parser would wave through.
#
# No network and no credential: the subject is a pure text checker, so the suite
# is hermetic and runs in CI through the plugin shell-suite glob.
#
# The rejection cases are the substance. A shape-only grammar accepts
# `outcome=noop swept=-` — a blind pass wearing a healthy pass's clothes — and a
# parser written against that grammar will report health nobody observed. The
# cross-field cases below are what make that line unspellable.
#
# Run all cases:        bash tick-summary.test.sh
# Run one named case:   bash tick-summary.test.sh --case <name>
#   cases: accepts-four-outcomes rejects-unknown-outcome rejects-unknown-role
#          rejects-field-order rejects-missing-token rejects-missing-field
#          rejects-space-in-value rejects-zero-for-unknown
#          rejects-unknown-for-completed rejects-ok-without-act
#          rejects-refused-with-sweep check-takes-last-line
#          check-refuses-empty regexp-is-the-applied-one contract-citation
#          portability
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/tick-summary.sh"

CASE_FILTER=""
if [[ "${1:-}" == "--case" ]]; then CASE_FILTER="${2:-}"; fi
want() { [[ -z "$CASE_FILTER" || "$CASE_FILTER" == "$1" ]]; }

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

# accepts <line> <what> — the checker must exit 0.
accepts() {
  if bash "$SCRIPT" validate "$1" >/dev/null 2>&1; then ok "$2"
  else no "$2" "rejected a line the contract permits: $1"; fi
}

# rejects <line> <what> [<expected substring of the reason>] — the checker must
# exit 1 AND say why. A rejection with no diagnosis sends the producer guessing.
rejects() {
  local out rc
  out=$(bash "$SCRIPT" validate "$1" 2>&1); rc=$?
  if [[ "$rc" -eq 0 ]]; then
    no "$2" "ACCEPTED a line the contract forbids: $1"
    return
  fi
  if [[ -n "${3:-}" ]] && [[ "$out" != *"$3"* ]]; then
    no "$2" "rejected, but the reason does not mention '$3'; got: $out"
    return
  fi
  ok "$2"
}

# ================================================ the four outcomes accepted ==
if want accepts-four-outcomes; then
  echo "accepts-four-outcomes — one well-formed line per outcome"
  accepts "tick role=pr-review-desk outcome=ok swept=14 acted=2 filed=0 duration=612" \
    "ok, with a numeric sweep and at least one act"
  accepts "tick role=pr-review-desk outcome=noop swept=14 acted=0 filed=0 duration=47" \
    "noop, with a numeric sweep and no act"
  accepts "tick role=worker-desk outcome=could-not-check swept=- acted=0 filed=1 duration=39" \
    "could-not-check, with swept unknown"
  accepts "tick role=verify-desk outcome=refused swept=0 acted=0 filed=0 duration=8" \
    "refused, before any sweep"
  # Every role that has a standing loop must be spellable.
  for role in the-desk intake-desk worker-desk pr-review-desk verify-desk; do
    accepts "tick role=$role outcome=noop swept=0 acted=0 filed=0 duration=1" \
      "role $role is accepted"
  done
  # A count may be unknown in any position, not only swept.
  accepts "tick role=the-desk outcome=could-not-check swept=- acted=- filed=- duration=3" \
    "every count may be unknown"
fi

# ======================================================== shape rejections ====
if want rejects-unknown-outcome; then
  echo "rejects-unknown-outcome — the outcome set is CLOSED"
  rejects "tick role=the-desk outcome=maybe swept=3 acted=1 filed=0 duration=12" \
    "an outcome outside the closed four is rejected" "does not match the published grammar"
  rejects "tick role=the-desk outcome=OK swept=3 acted=1 filed=0 duration=12" \
    "the outcome is case-sensitive"
  rejects "tick role=the-desk outcome= swept=3 acted=1 filed=0 duration=12" \
    "an empty outcome is rejected"
fi

if want rejects-unknown-role; then
  echo "rejects-unknown-role — only a role with a standing loop can tick"
  rejects "tick role=pr-shepherd outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "a worker-side role is not a tickable desk role"
  rejects "tick role=nope outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "an unknown role is rejected"
fi

if want rejects-field-order; then
  echo "rejects-field-order — the field order is FIXED, so a positional parser is safe"
  rejects "tick role=the-desk outcome=noop acted=0 swept=0 filed=0 duration=2" \
    "swept and acted transposed is rejected"
  rejects "tick outcome=noop role=the-desk swept=0 acted=0 filed=0 duration=2" \
    "outcome before role is rejected"
fi

if want rejects-missing-token; then
  echo "rejects-missing-token — the leading 'tick' is what makes a log grep context-free"
  rejects "role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "a line with no leading tick token is rejected"
  rejects "TICK role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "the leading token is case-sensitive"
  rejects "summary: tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "a prefixed line is rejected — the token must start the line"
fi

if want rejects-missing-field; then
  echo "rejects-missing-field — every field is required; there are no defaults"
  rejects "tick role=the-desk outcome=noop swept=0 acted=0 filed=0" \
    "a missing duration is rejected"
  rejects "tick role=the-desk outcome=noop swept=0 acted=0 duration=2" \
    "a missing filed count is rejected"
  rejects "tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=" \
    "an empty duration is rejected"
  rejects "tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=-" \
    "duration may NOT be unknown — a line that printed knows how long it ran"
fi

if want rejects-space-in-value; then
  echo "rejects-space-in-value — no value may contain whitespace"
  rejects "tick role=the desk outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "a space inside the role value is rejected"
  rejects "tick role=the-desk outcome=could not check swept=- acted=0 filed=0 duration=2" \
    "a spaced outcome is rejected — the token is hyphenated for exactly this reason"
  rejects "$(printf 'tick\trole=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2')" \
    "a tab separator is rejected" "contains a tab"
  rejects "tick  role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2" \
    "a doubled space is rejected"
fi

# ==================================================== cross-field rejections ==
# These four are the reason this file exists. Each is a line a shape-only
# grammar accepts and a reader would misread.

if want rejects-zero-for-unknown; then
  echo "rejects-zero-for-unknown — a blind pass may not report a count it never took"
  rejects "tick role=pr-review-desk outcome=could-not-check swept=0 acted=0 filed=0 duration=39" \
    "could-not-check with swept=0 is rejected" "requires swept=-"
  # The mutation this guards against: widening the rule to accept any count for
  # could-not-check. That reintroduces exactly the line above, which reads as
  # "the queue was empty" when the truth is "the instrument did not look".
  rejects "tick role=the-desk outcome=could-not-check swept=7 acted=0 filed=0 duration=5" \
    "could-not-check with a numeric swept is rejected whatever the number"
fi

if want rejects-unknown-for-completed; then
  echo "rejects-unknown-for-completed — noop is a POSITIVE claim about the queue"
  rejects "tick role=pr-review-desk outcome=noop swept=- acted=0 filed=0 duration=47" \
    "noop with an unknown sweep is rejected" "never noop"
  rejects "tick role=pr-review-desk outcome=ok swept=- acted=2 filed=0 duration=47" \
    "ok with an unknown sweep is rejected"
fi

if want rejects-ok-without-act; then
  echo "rejects-ok-without-act — ok means something landed"
  rejects "tick role=pr-review-desk outcome=ok swept=3 acted=0 filed=0 duration=12" \
    "ok with acted=0 is rejected" "requires acted >= 1"
  rejects "tick role=pr-review-desk outcome=ok swept=3 acted=- filed=0 duration=12" \
    "ok with an unknown act count is rejected"
  accepts "tick role=pr-review-desk outcome=ok swept=3 acted=1 filed=0 duration=12" \
    "ok with exactly one act is accepted"
fi

if want rejects-refused-with-sweep; then
  echo "rejects-refused-with-sweep — a refusal happens BEFORE the sweep"
  rejects "tick role=the-desk outcome=refused swept=4 acted=0 filed=0 duration=2" \
    "refused with a non-zero sweep is rejected" "requires swept=0"
  rejects "tick role=the-desk outcome=refused swept=0 acted=1 filed=0 duration=2" \
    "refused that nonetheless acted is rejected"
  rejects "tick role=the-desk outcome=refused swept=- acted=0 filed=0 duration=2" \
    "refused with an unknown sweep is rejected — nothing about a refusal is unknown"
fi

# ============================================================ the check verb ==
if want check-takes-last-line; then
  echo "check-takes-last-line — the summary line is the LAST line of the output"
  good="tick role=verify-desk outcome=ok swept=9 acted=3 filed=1 duration=201"
  if printf 'some work\nmore work\n%s\n' "$good" | bash "$SCRIPT" check >/dev/null 2>&1; then
    ok "a transcript ending in the summary line passes"
  else no "a transcript ending in the summary line passes" "check rejected it"; fi

  if printf 'some work\n%s\n' "$good" | bash "$SCRIPT" check >/dev/null 2>&1 && \
     printf 'some work\n%s\n\n\n' "$good" | bash "$SCRIPT" check >/dev/null 2>&1; then
    ok "trailing blank lines do not hide the summary line"
  else no "trailing blank lines do not hide the summary line" "check rejected a padded transcript"; fi

  # The failure this guards: the pass keeps printing after its summary line, so
  # a consumer reading the last line gets prose instead of a verdict.
  if printf '%s\nand then something else\n' "$good" | bash "$SCRIPT" check >/dev/null 2>&1; then
    no "output after the summary line is rejected" "check ACCEPTED a transcript that kept printing"
  else ok "output after the summary line is rejected"; fi

  # The failure that produced the contract: the pass was killed, so there is no
  # line at all. An empty log must not read as a pass.
  if printf 'booting\nsweeping\n' | bash "$SCRIPT" check >/dev/null 2>&1; then
    no "a killed transcript with no summary line is rejected" "check ACCEPTED it"
  else ok "a killed transcript with no summary line is rejected"; fi
fi

if want check-refuses-empty; then
  echo "check-refuses-empty — no output at all is the killed-pass signal"
  out=$(printf '' | bash "$SCRIPT" check 2>&1); rc=$?
  if [[ "$rc" -ne 0 && "$out" == *"did not complete"* ]]; then
    ok "empty input is refused, and says the pass did not complete"
  else no "empty input is refused, and says the pass did not complete" "rc=$rc out=$out"; fi
fi

# ================================================== the regexp is the applied ==
if want regexp-is-the-applied-one; then
  echo "regexp-is-the-applied-one — the published expression is not a second copy"
  re=$(bash "$SCRIPT" regexp)
  good="tick role=intake-desk outcome=noop swept=2 acted=0 filed=0 duration=11"
  bad="tick role=intake-desk outcome=nope swept=2 acted=0 filed=0 duration=11"
  if printf '%s' "$good" | grep -Eq "$re"; then ok "the printed expression matches a valid line"
  else no "the printed expression matches a valid line" "it did not"; fi
  if printf '%s' "$bad" | grep -Eq "$re"; then
    no "the printed expression rejects an invalid outcome" "it matched"
  else ok "the printed expression rejects an invalid outcome"; fi
  # A consumer that greps with the published expression and a consumer that
  # shells out to `validate` must agree on shape. (They differ only on the
  # cross-field rules, which a regular expression cannot express — that is
  # exactly why `validate` exists and is the form a producer should use.)
  if bash "$SCRIPT" validate "$good" >/dev/null 2>&1; then ok "validate agrees with the expression on a valid line"
  else no "validate agrees with the expression on a valid line" "validate rejected it"; fi
fi

# ======================================================== contract citation ===
# Nothing else in the tree checks that a file under references/ is cited by
# anything: the plugin-tree lint reads every markdown under plugins/ for CONTENT
# (house values, hidden characters) and never for LINKAGE. So an orphaned
# reference — added, cited by nobody, quietly going stale — is invisible. These
# cases are the check, and they live here because the reference and this script
# are the same deliverable: the reference states the grammar, this script applies
# it, and neither is of any use unless a role body actually points at them.
if want contract-citation; then
  echo "contract-citation — the reference is reachable from every desk body"
  REFS="$HERE/../references"
  SKILLS="$HERE/../skills"
  ref="$REFS/tick-contract.md"

  if [[ -f "$ref" ]]; then ok "the reference exists"
  else no "the reference exists" "missing: $ref"; fi

  # Every role with a standing loop must carry the pointer, exactly once. More
  # than once is a paraphrase creeping in beside the derived block.
  for role in the-desk intake-desk worker-desk pr-review-desk verify-desk; do
    body="$SKILLS/$role/SKILL.md"
    n=$(grep -c 'references/tick-contract\.md' "$body" 2>/dev/null || true)
    # The one citation is a markdown link whose label is the same path, so the
    # path appears twice on that line — once as label, once as target.
    lines=$(grep -c '^.*references/tick-contract\.md.*$' "$body" 2>/dev/null || true)
    if [[ "$lines" -eq 1 ]]; then ok "$role cites the reference on exactly one line"
    else no "$role cites the reference on exactly one line" "lines: $lines (occurrences: $n)"; fi

    # The path must RESOLVE from that body's own directory — a citation that
    # 404s is worse than none, because it reads as a pointer to something real.
    if [[ -f "$SKILLS/$role/../../references/tick-contract.md" ]]; then
      ok "$role's relative citation resolves"
    else no "$role's relative citation resolves" "../../references/tick-contract.md is not a file from $SKILLS/$role"; fi

    # And it must POINT, not restate. The grammar line itself belongs to the
    # reference alone; a body that inlines it owns a second copy that will drift.
    if grep -q 'tick role=.*outcome=.*swept=' "$body"; then
      no "$role does not restate the grammar line" "the summary-line grammar is inlined in the body"
    else ok "$role does not restate the grammar line"; fi
  done

  # The executable form the reference points at must exist at the path the
  # bodies' own relative spelling reaches.
  if [[ -f "$SKILLS/the-desk/../../scripts/tick-summary.sh" ]]; then
    ok "the grammar's executable form resolves from a body"
  else no "the grammar's executable form resolves from a body" "../../scripts/tick-summary.sh not found"; fi

  # A role with no standing loop has nothing to bound, so it must NOT carry the
  # block: a pointer nobody needs is a rule nobody applies.
  if grep -q 'references/tick-contract\.md' "$SKILLS/pr-shepherd/SKILL.md" 2>/dev/null; then
    no "a worker-side role does not carry the tick block" "pr-shepherd cites it"
  else ok "a worker-side role does not carry the tick block"; fi
fi

# ============================================================== portability ===
if want portability; then
  echo "portability — bash 3.2 (stock /bin/bash on macOS)"
  for builtin in mapfile readarray; do
    if grep -qE "^[^#]*\\b$builtin\\b" "$SCRIPT"; then
      no "no bash-4-only builtin '$builtin'" "found in $SCRIPT"
    else ok "no bash-4-only builtin '$builtin'"; fi
  done
  for decl in "declare -A" "local -A"; do
    if grep -qE "^[^#]*$decl" "$SCRIPT"; then
      no "no bash-4-only associative-array declaration '$decl'" "found in $SCRIPT"
    else ok "no bash-4-only associative-array declaration '$decl'"; fi
  done
  if grep -qE '^[^#]*\$\{[A-Za-z_][A-Za-z0-9_]*[,^]{1,2}\}' "$SCRIPT"; then
    no "no bash-4-only case-modification expansion" "found in $SCRIPT"
  else ok "no bash-4-only case-modification expansion"; fi
  # The checker is pure text: it must reach no network and read no credential.
  if grep -qE '^[^#]*\b(curl|wget|gh|git|desktoken)\b' "$SCRIPT"; then
    no "the checker shells out to no network or forge tool" "found one in $SCRIPT"
  else ok "the checker shells out to no network or forge tool"; fi
  if bash "$SCRIPT" --version >/dev/null 2>&1; then ok "--version works"
  else no "--version works" "it did not"; fi
fi

echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
