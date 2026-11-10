---
brief: assay:assay:desk-tools:64
title: "Decide which lines of a branch diff the secret scan reads, and pin the answer"
why: >-
  deskpr's branch-diff scan reads every line of the diff against the default branch: the lines
  the change adds, the lines it removes, and three unchanged context lines around each hunk.
  Many of the false positives on record were not written by the change at all. They were
  unchanged context already on the default branch, or a removal. Each one needed an override
  or a rename of text the change did not touch. deskevidence and the outbound push check already
  judge only added lines. Whether deskpr should do the same is a trade between catching a
  credential that is already published and stopping work on text nobody changed. It is a
  human decision. This brief puts it as options, each with the threat it opens, and then
  implements and pins whichever answer is recorded.
wave: 2
depends: ["desk-tools/62"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  Narrowing what the scan reads is narrowing a trust-boundary control. Each narrower option
  stops flagging a class of credential-shaped text in a diff. The text is already on the
  default branch, but nothing else on the desk would surface it again. Only the human can
  weigh that against the overrides the full read costs.
decision-trigger: creation
issues: [4284, 929, 1642]
outcome: none
design: DR-secret-scan-scope
schema: brief-v2
version: 1
authored: 2026-11-10 by a desk-dispatched authoring session (error-class design for issue 4284; the diff-scope decision)
sources:
  - "medici-finance/assay#929 part 1 — deskpr update and edit scan against the default branch, not the remote head"
  - "medici-finance/assay#1642 — a Dockerfile substitution flagged in an unchanged context line; the 2026-11-10 occurrence, Verify-row selectors already on the default branch refused as context next to a new row"
  - "medici-finance/assay#6060, closed by #6062 — the full-diff breadth of the remote check is deliberate and pinned by TestRemoteCheckScansWhole"
  - "deskevidence scans only added bytes (#966, #1161); its line numbers were fixed by blanking unchanged lines in place (#3283, closed by #3300)"
  - "docs/streams/desktools-v2/spec.md §8.2 — the branch-diff row reads secret arms + ruling-claim guard only; the ruling-claim guard already reads added lines only"
  - "freshness-checked 2026-11-10 @ 450ac4047 (origin/main)"
exec-tier: strong
exec-tier-why: "A trust-boundary scope change, conditional on a remote read that must fail closed, with pinned tests that must agree with the recorded ruling and keep desk-tools/62's line mapping intact."
domain: complicated
consumers:
  - "tools/desk/cmd/deskpr/deskpr.go: fixed-here (the branch-diff scan reads the ruled sides)"
  - "tools/desk/README.md: fixed-here (the deskpr row states which lines the branch-diff scan reads)"
  - "docs/streams/desktools-v2/spec.md: fixed-here (the §8.2 branch-diff row names the ruled scope)"
id: e820189a-d68b-4eaf-bcbd-731f831438d9
---

# Brief 64 — Decide which lines of a branch diff the secret scan reads

## Dependencies
- **desk-tools/62** — the diff line index names every line's side (added, removed or context).
  The ruled scope is a side filter over that index, and the filter blanks unselected lines in
  place so 62's file-and-line mapping still holds.
- Independent of desk-tools/61 and 63. The armor fix in 63 stands under the ruled scope: an
  armor line the change adds is always in scope.
- Keeps #6062. The ruled scope is option 2, which leaves the remote check's documented breadth
  over commits as it is. Only the context side of each hunk is narrowed.

## Context

files:
- `tools/desk/cmd/deskpr/deskpr.go` — `scanWrite`'s branch-diff surface, and the remote read
  `requireRemoteHoldsHead` already makes (`git ls-remote` on the push URL)
- `tools/desk/cmd/deskpr/remotescan_test.go` — `TestRemoteCheckScansWhole` is not edited
- `tools/desk/cmd/deskpr/diffscope_test.go` (planned) — NEW
- `tools/desk/cmd/deskpr/diffscope-mutations.json` (planned) — NEW
- `tools/desk/README.md`, `docs/streams/desktools-v2/spec.md`
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- `scanWrite` scans the meta-stripped text of the symmetric diff from the default branch to
  HEAD, with three context lines. The create path and the update path (in override mode) both
  use the default branch as the base.
- The diff base is the merge-base of the local default-branch ref and HEAD. That ref is local,
  so a commit on it may never have been pushed. Its context lines are then not public, and
  skipping them would let unpublished text through unscanned.
- deskpr already reads the push destination's own refs with `git ls-remote` on the push URL
  (`requireRemoteHoldsHead`). That read is the remote's answer, not a local ref the pusher
  controls.
- The ruling-claim guard on the same push reads added lines only. deskevidence and the outbound
  push check judge added lines only.

design-fit:
  owner: tools/desk/cmd/deskpr/deskpr.go — the branch-diff scan surface
  contract: S-publication-scan
  retires: [the overrides spent on unchanged context lines whose base is already on the remote]
  weight: "verbs 0, flags 0, refusals 0; one side filter over the existing index and one ancestry check over a remote read deskpr already makes"
  why-add: "The filter is the narrowing itself. It reuses the 62 index and the existing ls-remote read rather than adding a second diff reader or a second remote client."

## Human decision

When the desk opens or updates a pull request, its secret scan reads the code diff against the
main branch. That diff holds the lines the change adds, the lines it removes, and a few
unchanged lines around each edit. Many past false alarms were on lines the change did not
write: unchanged lines already on the main branch, or lines being deleted. Each one needed an
audited override or a rename. Other desk tools already read only the lines a change adds.
Removed and unchanged lines are already public on the main branch, so no option changes what a
push newly exposes. The options differ in what the scan reports again.

Options:
1. **Keep reading the whole diff** — no change. The scan keeps re-reporting credential-shaped
   text that sits unchanged next to an edit, so it costs overrides and renames on text the change
   did not write. It opens no new threat.
2. **Read added and removed lines, skip unchanged context** — the recommendation. It ends the
   context-line false alarms. The threat it opens: a credential already sitting unchanged on the
   main branch, next to an edit, is no longer flagged again. A removed credential still raises
   the alarm, which is the prompt to rotate it.
3. **Read added lines only** — the same scope as the other desk tools. The threat it opens is
   option 2's, and also that deleting a leaked credential raises no alarm, so nobody is prompted
   to rotate it.
4. **Option 2, and on an update read only the commits the remote does not hold yet** — this
   also stops re-judging earlier pushes. The threat it opens is option 2's, and also that
   content already pushed, including content pushed under an override, is never judged again on
   later updates.

Default if no answer: option 1 after 14 days. Silence changes nothing. Option 2 is the
recommendation. The ruling below takes option 2 and adds a condition: unchanged context is
skipped only when the diff's base is proven to be on the remote.

## Ground rules
- Ruled option B on the decision issue, which is option 2 above: https://github.com/medici-finance/assay/issues/4284#issuecomment-6774347835
  The ruling scans added and removed lines and skips unchanged context, with tests proving the
  base is on the remote before context is skipped. Options 1, 3 and 4 are not taken.
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement exactly the recorded answer: option 2, conditional on the base being on the remote.
- Fail closed. When the remote read fails or cannot show the base is on the remote, context is
  scanned in full, exactly as today. Never treat could-not-check as on the remote.
- Never write a credential-shaped literal into a test. Generate planted runs from a fixed seed.
- The scanner (`bodycheck.go`, `structured.go`) is not touched. The scope is chosen in deskpr.

## Task
1. Add one constant in deskpr naming the ruled sides: added and removed. The refusal text and
   `--explain` do not change.
2. Add `baseOnRemote` (planned). It reads the push destination's default-branch head with the
   same `git ls-remote` on the push URL that `requireRemoteHoldsHead` uses, and returns on-remote
   only when that read succeeds, the returned commit is present locally, and the diff base is
   an ancestor of it (`git merge-base --is-ancestor`). Every other outcome returns not-proven:
   the read errors, the ref is absent, the object is missing, or the base is not an ancestor.
3. Build the scanned text from the desk-tools/62 index. When `baseOnRemote` returns on-remote, a
   context line is blanked in place, so line positions and 62's mapping are unchanged. Otherwise
   no line is blanked, and the text is what is scanned today. Added and removed lines are never
   blanked.
4. `TestDiffScopeSides` (planned) uses a local bare repository as the remote with the base pushed.
   It plants a generated run on each side (added, removed, context) in separate diffs, and
   asserts added refused, removed refused and context clean.
5. `TestDiffScopeBaseOffRemote` (planned) commits the base on the local default branch without
   pushing it, and points the local remote-tracking ref at that unpushed base, so a check that
   trusted the local ref would wrongly pass. It plants a generated run in a context line and
   asserts it is refused.
   `TestDiffScopeBaseUnknown` (planned) makes the remote read fail (an unreachable push URL) and
   asserts the context plant is refused. Neither test edits `TestRemoteCheckScansWhole`.
6. `TestDiffScopeDocs` (planned) asserts that the deskpr row in `tools/desk/README.md` and the §8.2
   branch-diff row in `docs/streams/desktools-v2/spec.md` both name the ruled scope and its
   condition.
7. Add `diffscope-mutations.json` with four mutants, each paired with the test that catches it:
   add context to the side set (`TestDiffScopeSides` (planned)), drop removed from the side set
   (`TestDiffScopeSides` (planned)), make `baseOnRemote` skip the ancestry check
   (`TestDiffScopeBaseOffRemote` (planned)), and make `baseOnRemote` return on-remote when the read fails
   (`TestDiffScopeBaseUnknown` (planned)).
8. Record the ruling, as the human's own comment on the decision issue, in the README row. Change
   every consumer entry to `fixed-here`.

## Verify (executable — no prose-only DoD items)
Every row below is risk-bearing and reds at the delivery base, where none of the named tests or
the mutation spec exists. The verifier records each with `verifyrun --fail-first` against the
parent of the delivering merge. Rows 1 and 2 are the two halves of the ruled condition, and row
4 holds the mutant paired with each.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffScope' -count=1 -v > "${TMPDIR:-/tmp}/dt64-scope.log" 2>&1 && grep -F -e '--- PASS: TestDiffScopeSides ' "${TMPDIR:-/tmp}/dt64-scope.log" && grep -F -e '--- PASS: TestDiffScopeDocs ' "${TMPDIR:-/tmp}/dt64-scope.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt64-scope.log"` | exit 0; with the base on the remote, added and removed plants are refused and the context plant is clean; the README and spec name the same scope and condition. Red at the delivery base (no such test) |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffScopeBase' -count=1 -v > "${TMPDIR:-/tmp}/dt64-base.log" 2>&1 && grep -F -e '--- PASS: TestDiffScopeBaseOffRemote ' "${TMPDIR:-/tmp}/dt64-base.log" && grep -F -e '--- PASS: TestDiffScopeBaseUnknown ' "${TMPDIR:-/tmp}/dt64-base.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt64-base.log"` | exit 0; with the base not on the remote, or the remote read failing, the context plant is refused (context scanned in full). Red at the delivery base (no such test) |
| 3 | check:ci +neighbour | `cd tools/desk && test -f cmd/deskpr/diffscope_test.go && go test ./cmd/deskpr -count=1 -v > "${TMPDIR:-/tmp}/dt64-all.log" 2>&1 && grep -F -e '--- PASS: TestRemoteCheckScansWhole ' "${TMPDIR:-/tmp}/dt64-all.log" && grep -F -e '--- PASS: TestDiffScopeBaseOffRemote ' "${TMPDIR:-/tmp}/dt64-all.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt64-all.log" && go test ./internal/deskkit/ -count=1` | exit 0; deskpr's remote-check, override and diff-finding tests pass with the ruled scope, and the scanner package is unchanged and green. Red at the delivery base (no test file) |
| 4 | check:ci +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -spec cmd/deskpr/diffscope-mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e 'Totals: 4 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$O"` | exit 0; all four mutants caught: the two side-set mutants pair with row 1, and the ancestry-skip and read-failure mutants pair with row 2. Red at the delivery base (no spec) |
| 5 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/bodycheck.go tools/desk/internal/deskkit/structured.go tools/desk/cmd/deskpr/remotescan_test.go && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-64-diff-scan-scope.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/64 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the scanner and the remote-check test are untouched, and deskpr, the README row and the spec row are in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The implemented sides are wider or narrower than the ruling | row 1; row 4 (side-set mutants) |
| Context is skipped when the base was never pushed, so unpublished text goes unscanned | row 2 (`TestDiffScopeBaseOffRemote` (planned)); row 4 (ancestry-skip mutant) |
| A failed or ambiguous remote read is treated as on the remote | row 2 (`TestDiffScopeBaseUnknown` (planned)); row 4 (read-failure mutant) |
| The remote head is read from a local ref the pusher controls | review: `baseOnRemote` (planned) must use the push-URL `ls-remote` read, never a remote-tracking ref; row 2's off-remote test runs with a stale local ref present |
| The narrowing also drops added lines, so a new credential is pushed | row 1 (the added-line plant refuses) |
| Blanking lines shifts positions, so 62's file-and-line report names the wrong line | row 3 (62's diff-finding tests stay green) |
| The README or spec claims a scope the code does not have | row 1 (`TestDiffScopeDocs` (planned)) |
| The scope is narrowed by editing the scanner or the remote-check test | row 5 |
| A credential already on the remote's main branch goes unreported once context is skipped | No row; it is the threat the ruling accepts. Review-only: the PR names it, and DR-secret-scan-scope records it |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human, answered: the ruling above, recorded as DR-secret-scan-scope and cited by
`design:`. The implementation PR needs a `Security-Review: pass` at its head before any ready
flip. Every Verify row is risk-bearing, because the brief answers `sensitive-data: yes`. Each
row is written to fail on the delivery base, and the verifier records it with
`verifyrun --fail-first` against the parent of the delivering merge.
