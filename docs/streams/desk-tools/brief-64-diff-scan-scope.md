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
exec-tier-why: "A trust-boundary scope change with a pinned test that must agree with the recorded ruling, and that must keep desk-tools/62's line mapping intact."
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
  A narrower scope is a side filter over that index, and the filter blanks unselected lines in
  place so 62's file-and-line mapping still holds.
- Independent of desk-tools/61 and 63. The armor fix in 63 stands under every answer here: an
  armor line the change adds is in scope under every option.
- Reverses or keeps #6062. Option 4 reverses its documented breadth for `update`; the others
  keep it.

## Context

files:
- `tools/desk/cmd/deskpr/deskpr.go` — `scanWrite`'s branch-diff surface
- `tools/desk/cmd/deskpr/remotescan_test.go` — `TestRemoteCheckScansWhole` changes only as the ruling says
- `tools/desk/cmd/deskpr/diffscope_test.go` (planned) — NEW
- `tools/desk/cmd/deskpr/diffscope-mutations.json` (planned) — NEW
- `tools/desk/README.md`, `docs/streams/desktools-v2/spec.md`
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- `scanWrite` scans the meta-stripped text of the symmetric diff from the default branch to
  HEAD, with three context lines. The create path and the update path (in override mode) both
  use the default branch as the base.
- The ruling-claim guard on the same push reads added lines only.
- deskevidence and the outbound push check judge added lines only.
- The branch-diff text is already on the default branch for its removed and context lines, so
  none of the options changes what the push itself newly publishes. They differ in what the scan
  re-reports.

design-fit:
  owner: tools/desk/cmd/deskpr/deskpr.go — the branch-diff scan surface
  contract: S-publication-scan
  retires: [the overrides spent on unchanged context lines, under options 2 to 4; nothing under option 1]
  weight: "verbs 0, flags 0, refusals 0 or a narrowing; one side filter over the existing index"
  why-add: "n/a under option 1. Under options 2 to 4 the filter is the narrowing itself, and it reuses the index rather than adding a second diff reader."

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
recommendation.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement exactly the recorded answer. With no recorded answer, implement option 1.
- Never write a credential-shaped literal into a test. Generate planted runs from a fixed seed.
- The scanner (`bodycheck.go`, `structured.go`) is not touched. The scope is chosen in deskpr.

## Task
1. Add one constant in deskpr naming the ruled scope: the set of sides the branch-diff scan
   reads, and, for option 4, the update base. The refusal text and `--explain` do not change.
2. Under options 2 to 4, build the scanned text from the desk-tools/62 index. A line on an
   unselected side is blanked in place, so line positions and 62's mapping are unchanged. Under
   option 4, the update path's base is the confirmed remote head the push already reads. Under
   option 1, the text is what is scanned today.
3. `TestDiffScopeSides` (planned) plants a generated run on each side (added, removed, context), in
   separate diffs, and asserts refused or clean for each side exactly as the constant says. For
   option 4, `TestDiffScopeUpdateBase` (planned) plants a run in an already-pushed commit and in a new
   one, and asserts the ruled verdict for each. Update `TestRemoteCheckScansWhole` only as far
   as the ruling changes its claim, and name the ruling in its comment.
4. `TestDiffScopeDocs` (planned) asserts that the deskpr row in `tools/desk/README.md` and the §8.2
   branch-diff row in `docs/streams/desktools-v2/spec.md` both name the scope the constant holds.
5. Add `diffscope-mutations.json` with two mutants: one widens the side set by one side and one
   narrows it by one side. Each must be caught by `TestDiffScopeSides` (planned).
6. Record the ruling, as the human's own comment on the decision issue, in the README row. Change
   every consumer entry to `fixed-here`.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffScope' -count=1 -v > "${TMPDIR:-/tmp}/dt64-scope.log" 2>&1 && grep -F -e '--- PASS: TestDiffScopeSides ' "${TMPDIR:-/tmp}/dt64-scope.log" && grep -F -e '--- PASS: TestDiffScopeDocs ' "${TMPDIR:-/tmp}/dt64-scope.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt64-scope.log"` | exit 0; both named PASS lines: each side is scanned or skipped exactly as ruled, and the README and spec name the same scope |
| 2 | check:ci +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -spec cmd/deskpr/diffscope-mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e 'Totals: 2 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$O"` | exit 0; widening or narrowing the ruled scope by one side is caught |
| 3 | check:ci +neighbour | `cd tools/desk && test -f cmd/deskpr/diffscope_test.go && go test ./cmd/deskpr -count=1 -v > "${TMPDIR:-/tmp}/dt64-all.log" 2>&1 && grep -F -e '--- PASS: TestRemoteCheckScansWhole ' "${TMPDIR:-/tmp}/dt64-all.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt64-all.log" && go test ./internal/deskkit/ -count=1` | exit 0; deskpr's remote-check, override and diff-finding tests pass with the ruled scope, and the scanner package is unchanged and green |
| 4 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/bodycheck.go tools/desk/internal/deskkit/structured.go && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-64-diff-scan-scope.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/64 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the scanner is untouched, and deskpr, the README row and the spec row are in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The implemented scope is wider or narrower than the ruling | rows 1 and 2 |
| A narrower scope also drops added lines, so a new credential is pushed | row 1 (`TestDiffScopeSides` (planned), the added-line plant refuses under every option) |
| Blanking lines shifts positions, so 62's file-and-line report names the wrong line | row 3 (62's diff-finding tests stay green) |
| The README or spec claims a scope the code does not have | row 1 (`TestDiffScopeDocs` (planned)) |
| The scope is narrowed by editing the scanner rather than deskpr | row 4 |
| Option 4's remote head is read from a ref the pusher controls, so a forged head skips content | row 1 (`TestDiffScopeUpdateBase` (planned), under option 4); review: the base must be the remote head the push already confirms |
| A credential already on the main branch goes unreported once context is skipped | No row; it is the threat the ruling accepts. Review-only: the PR names it, and the ruling records it |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human. The decision issue carries the human's own answer before work starts, or the
14-day default applies. Before it leaves `todo`, a narrowing (options 2 to 4) needs an approved
`DR-<slug>` record cited by `design:` (brief-rules 48). The implementation PR needs a
`Security-Review: pass` at its head before any ready flip. Every Verify row is risk-bearing,
because the brief answers `sensitive-data: yes`. Each row is written to fail on the delivery
base, and the verifier records it with `verifyrun --fail-first` against the parent of the
delivering merge.
