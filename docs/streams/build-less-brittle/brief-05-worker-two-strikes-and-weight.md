---
brief: assay:assay:build-less-brittle:05
title: "Worker two-strikes: the second fix in a class becomes a design note; PR bodies report measured weight"
why: >-
  A worker dispatched on a symptom fixes the symptom, even when the same mechanism was patched
  last week. Each fix then pins itself in place with a fail-first test and, under today's
  defect-class clause, a new guard. Stopping at the second fix in a class, and asking the worker
  for a short design note naming the root invariant, turns repeated patching into one design
  decision. Reporting measured weight in every PR makes additions visible where they are made.
wave: 3
depends: ["build-less-brittle/03", "build-less-brittle/04"]
unblocks: ["build-less-brittle/11", "build-less-brittle/13"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §2 D1/D3/D5, §3 rows 3 and 8, §4.3"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md clauses 8 (reuse ladder) and 14 (defect class)"
  - "tools/desk/cmd/deskdispatch/kitparity_test.go:44 (TestDefectClassClauseIsOneWordingAcrossImplementerKits)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no strike, design-note or Weight section in either implementer kit; clause 14 instructs adding a class guard"
exec-tier: strong
exec-tier-why: "(b) the same clause wording must land byte-identical in two kits under a parity test, and the worker-desk re-dispatch tier rule must agree with it."
domain: complicated
consumers:
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: fixed-here (clauses 8 and 14 amended)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md: fixed-here (the same wording, verbatim)"
  - "plugins/assay/skills/worker-desk/SKILL.md: fixed-here (re-dispatch tier; strike-two routing; defect-class summary aligned)"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 05 — Worker two-strikes and the measured Weight report

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md`: clause 8 (scope and reporting), clause 14 (defect class).
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`: its verbatim copies of both.
- `plugins/assay/skills/worker-desk/SKILL.md`: re-dispatch tier; routing of a returned design note.
- `tools/desk/cmd/deskdispatch/kitparity_test.go`: its required-token list for clause 14.
- `changelog/build-less-brittle-05.md` (planned)

single-point-of-failure: the worker's author check on a `bleed` reply (the driver's own login,
read from the forge, never from the text) is the ONE control that lifts the two-strikes hard stop.
Behind it: the bleed fix still goes through the review loop, and the driver's own merge (spec §4.7).

facts:
- The objective kit carries every load-bearing worker clause **verbatim**.
  `TestDefectClassClauseIsOneWordingAcrossImplementerKits` (kitparity_test.go:44) holds clause 14
  identical across the kits, and `TestKitsCarryNoPrivateReferences` (kittext_test.go:109)
  forbids private references in any kit.
- **The guard obligation is itself pinned by a test.** The parity test requires clause 14's body
  in both kits to contain `## Defect class`, `ALLOW-LIST`, `PLANTED SECOND` and `## Fail-first`
  (kitparity_test.go, the `must` list). `ALLOW-LIST` is the add-a-guard obligation this brief
  retires, so the token list changes with it: `ALLOW-LIST` is replaced by `unrepresentable`.
  This is the spec's D3 dynamic in miniature: a test entrenching a patch. Retiring the
  obligation retires the assertion, and the other three tokens stay.
- Clause 8's reuse ladder says "a briefed deliverable is never re-litigated as YAGNI". Strike two
  is the one carve-out.
- Strike detection needs no new tool. The item's `error-class` issue (brief 04) lists attached
  fixes. An item with no class issue: the worker searches the tracker for an open
  `error-class` issue naming the mechanism before coding.
- The Weight line comes from brief 03:
  `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v -args -rev=<sha>`,
  run once at the merge-base and once at the head, plus `git diff --shortstat <merge-base>...HEAD`.
  In a repository without the counter, the section says `could-not-check (no weight counter)`.
- The bleed exception (spec §10 D-B, **ratified by the driver on #1660**): two-strikes is a **hard stop**. A
  `bleed` reply by the driver on the class issue lets a production-down or security fix
  proceed, and nothing else does; the class stays `design-owed`. The kit text carries exactly
  that form.
- **Who may say `bleed`** (spec §4.7). The reply counts only when its author, read from the
  forge's comment record and never from the comment text, is the driver's own login, which the
  project layer names. A `bleed` from any other login is quarantined: the worker notes it on
  the class issue and keeps the hard stop. On a public repository anyone can comment, so
  without this check any account could lift the stop on exactly the classes this stream exists
  to redesign.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): worker-prompt.md 358, worker-prompt-objective.md 509, worker-desk 868.

design-fit:
  owner: tools/desk/cmd/deskdispatch/references/worker-prompt.md (the implementer contract)
  contract: none — dispatch kits have no semantic-owner row
  retires: ["clause 14's default of adding a class guard (replaced by: remove the hazard, or make it unrepresentable; a guard is weight)", "kitparity_test.go's ALLOW-LIST required token (replaced by unrepresentable)"]
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 in each of the three files
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Kit wording that the parity test pins must change in both kits in the same commit.
- Each file is net ≤ 0 lines. Clause 14's ~45 lines hold worked examples that can be tightened.
  Never drop its "name the class" or "fail-first against a planted second instance" obligations,
  or its positive-control sentence ("A guard whose own matcher could silently stop matching
  carries a positive control"), which still binds every guard added when removal is infeasible.

## Task

1. **Clause 8, strike two** (≤ 10 lines, in both kits). Before coding a defect fix, check the
   item's class record. If it already records a merged fix, STOP. Post a design note on the
   class issue with `deskfile attach`, carrying: root invariant; the owner it should live in
   (a semantic-owner row, or `unknown`); what the prior fixes added that a design would retire;
   a proposed design-brief title. Report `NEEDS_CONTEXT: strike two — design note posted`.
   The exception is a `bleed` reply on the class issue that names THIS item (a release `bleed`
   from intake counts), for a production-down or security fix (the class stays `design-owed`),
   whose author is the driver's own login (the project value; none configured: the stop stands),
   read from the forge's comment record. Any other `bleed` is quarantined, noted on the class
   issue and never acted on.
2. **Clause 8, Weight** (≤ 4 lines, both kits). Every PR body carries `## Weight`: the two counter
   lines and the shortstat. A positive delta in any ratcheted dimension also carries
   `why-add:`. The section is a material claim: a wrong line is a review finding.
3. **Clause 14 amended** (both kits, identical). Keep obligations 1 (name the class) and 3
   (fail-first against a planted second instance). Obligation 2 becomes: "Close the class by
   REMOVING the hazardous path or making it unrepresentable (a type, a single choke point).
   Only when removal is infeasible, add a guard, and report it as weight in `## Weight` and as
   a rule-register row."
4. **worker-desk** (≤ 4 lines). A re-dispatch or shepherd pass on a PR whose open finding class
   is at round ≥ 2 runs at strong tier. A worker's `strike two` NEEDS_CONTEXT returns the item
   to intake as `design-owed`. It is not a failure to retry.
5. In `kitparity_test.go`, replace `ALLOW-LIST` with `unrepresentable` in the `must` list, and
   update the file's header comment ("guard the class" becomes "close the class").
6. Offset lines in each file, run the deskdispatch tests, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Row 2 is the neighbour/guard row: the parity
test still holds. Row 3 is the mutation row for that guard. Rows 7–9 are net ≤ 0 weight rows. Row 10 checks the pinned token moved. Row 13 checks the `bleed` lift is bound to the item and to D-B's scope in each kit.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 -v` | `--- PASS` |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/worker-prompt-objective.md && cp "$f" /tmp/bl05-kit.bak && sed 's/unrepresentable/unrepresentible/' /tmp/bl05-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 > /tmp/bl05-mut.out 2>&1; rc=$?; cp /tmp/bl05-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | `RED-ON-DRIFT` (the parity test catches a one-word drift in the amended clause) |
| 4 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -ciE 'strike two' "$f"; done \| grep -c '^[1-9]'` | `2` |
| 5 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c '## Weight' "$f"; done \| grep -c '^[1-9]'` | `2` |
| 6 | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/worker-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | `1`, then `COUNTER-EXISTS`; output is `COUNTER-EXISTS` (dereference: the command the kit tells workers to run exists; `grep -c` fails on a zero count, so the token prints only when the kit names the command and the counter test file exists). Expect re-phrased 2026-10-06 (#1915): the earlier count-floor wording read the floor from the last output line, which is the token, so the witness could not decide the row. |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 10 | `f=tools/desk/cmd/deskdispatch/kitparity_test.go; grep -q '"unrepresentable"' "$f" && ! grep -q '"ALLOW-LIST"' "$f" && echo MOVED` | `MOVED` (the pinned token moved with the retired obligation) |
| 11 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "$PWD/.bl05-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/05 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | exit 0; output is `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` (the `consumers:` claims are judged against the delivering change itself, in a throwaway clone: after the merge, the first-parent commit on main carrying the `Brief: build-less-brittle/05` trailer (the net-weight rows resolve the same commit) diffed against its parent; before the merge, the PR head diffed against its merge-base; the three `fixed-here` entries are corroborated, and the UNCHECKED one is the out-of-scope installed-binaries entry, which the delivering diff does not touch and which stays the reviewer's call per brief-rule 9). A disproved claim fails the row and names the claim; an unresolvable delivering change fails the row rather than passing on an empty diff. Row re-authored 2026-10-06 (#1915): the unpinned form ran against merged main, where the brief is no longer in the diff, and reported COULD-NOT-CHECK; this form runs the same way before and after the merge (the #1908 pattern). Expect first re-written 2026-10-03 (#1862). | |
| 12 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c "driver's own login" "$f"; done \| grep -c '^[1-9]'` | `2` (both kits state that only a `bleed` from the driver's own login lifts strike two; the check is procedure the worker runs, not code, so this row gates its presence in each kit) |
| 13 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c 'THIS item, for a production-down or security fix' "$f"; done \| grep -c '^[1-9]'` | `2` (both kits bind a `bleed` to the item it names and to D-B's production-down/security scope; a class-wide or unscoped `bleed` lifts nothing) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### 2026-10-06 non-implementer verification on merged main (56140a33d)

Run on merged main 56140a33d0b4cde167f9e8269e624769fc3013ca (the forge's main at run time). The implementing change is the squash commit e77868565 (#2245, single parent 9cd7271bb). statusgen was built from the tree's own statusgen module with a throwaway HOME.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | 0 | `ok  github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 38.551s` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 -v` | 0 | `--- PASS: TestDefectClassClauseIsOneWordingAcrossImplementerKits (0.00s)`, then `ok` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/worker-prompt-objective.md && cp "$f" /tmp/bl05-kit.bak && sed 's/unrepresentable/unrepresentible/' /tmp/bl05-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 > /tmp/bl05-mut.out 2>&1; rc=$?; cp /tmp/bl05-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | 0 | `RED-ON-DRIFT`. The mutant run failed with: kit "worker-objective" defect-class clause lost "unrepresentable", and the two kits' defect-class clauses differ. The kit was restored and the tree was clean afterwards | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -ciE 'strike two' "$f"; done \| grep -c '^[1-9]'` | 0 | `2` (per-kit counts 2 and 2) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c '## Weight' "$f"; done \| grep -c '^[1-9]'` | 0 | `2` (per-kit counts 2 and 2) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/worker-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | 0 | `1`, then `COUNTER-EXISTS`. The counter test function TestPrintWeight exists in the weight package's test file. The witness gives this row a non-pass status only because its parser cannot read "a count ≥ 1" from the Expect cell. Its output hash bd8115622356 equals the sha256 of exactly `1` + newline + `COUNTER-EXISTS` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" && echo NET-OK` | 0 | `NET-OK` (impl e77868565, base 9cd7271bb; 388 lines before and 388 after) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" && echo NET-OK` | 0 | `NET-OK` (522 lines before and 522 after) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/05$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | 0 | `NET-OK` (923 lines before and 921 after) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `f=tools/desk/cmd/deskdispatch/kitparity_test.go; grep -q '"unrepresentable"' "$f" && ! grep -q '"ALLOW-LIST"' "$f" && echo MOVED` | 0 | `MOVED` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `statusgen --consumers --root . --brief build-less-brittle/05; echo "exit=$?"` | 0 | Pass, judged at the PR head as the Expect specifies. On merged main the command printed `exit=2` with a COULD-NOT-CHECK status: the brief is not in the diff against 56140a33d, so that run is no evidence either way. That is why the witness records this row as fail. The re-run the tool itself prescribes for a merged brief was made at the implementing commit e77868565 with `--base 9cd7271bb6d20900208f91c4f11738324b99142c`. It printed 3 corroborated and 0 disproved, then `exit=0`: the three fixed-here entries were corroborated, and the out-of-scope installed-binaries entry was unchanged since the merge-base | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c "driver's own login" "$f"; done \| grep -c '^[1-9]'` | 0 | `2` (per-kit counts 1 and 1) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c 'THIS item, for a production-down or security fix' "$f"; done \| grep -c '^[1-9]'` | 0 | `2` (per-kit counts 1 and 1) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Review-gate check: clause 14 still carries obligation 3, the fail-first against a planted second instance, in both kits. The kits read "Show the class closed against a PLANTED SECOND instance", and the parity test pins `PLANTED SECOND` and `## Fail-first`. The positive-control sentence is kept verbatim. The clause 8 strike-two and Weight blocks are byte-identical between the two kits on merged main.

Risk-bearing values: the risk metadata is present and every field reads no (irreversible: no). The diff touches dispatch-kit text, one skill and one parity test, so the fail-safe trigger does not fire. The enumeration was done anyway, over the implementing diff:
- strike threshold = second merged fix in one class @ tools/desk/cmd/deskdispatch/references/worker-prompt.md:154 (and the objective kit at line 226)
- re-dispatch tier threshold `round ≥ 2` @ plugins/assay/skills/worker-desk/SKILL.md:508
- parity required tokens `"unrepresentable"`, `"REMOVING the hazardous path"` @ tools/desk/cmd/deskdispatch/kitparity_test.go:56
- weight counter invocation `-count=1` @ tools/desk/cmd/deskdispatch/references/worker-prompt.md:164

Ranking: every value can be undone with a text edit and the next desk-tools release, because the kits are embedded in the binary. The two tier and strike thresholds rank first because they change worker behaviour. The tokens and the test flag are operational and need no derivation.

RISK-VALUE: DERIVED — strike threshold = 2 (the second merged fix in one class) @ tools/desk/cmd/deskdispatch/references/worker-prompt.md:154 — spec §2 row 3 and §4.3 define two-strikes as the stop at the second fix keyed to the same class issue. One prior merged fix in the class record is the earliest point where a repeat is observable, so a lower bound would block first fixes and a higher one would allow repeated patching.
RISK-VALUE: DERIVED — round threshold = 2 @ plugins/assay/skills/worker-desk/SKILL.md:508 — spec §2 row 8 folds model tiering into this brief as "round ≥ 2 on a finding class … runs strong". It is a reversible tier knob that matches the strike-two trigger.

**VERIFY: PASS — 13/13 Verify rows on merged main 56140a33d**

Execution witness: `statusgen verifyrun` ran for real on merged main 56140a33d (11 pass; row 6 could-not-run, an Expect-parser limit, with the output hash matching `1
COUNTER-EXISTS
`; row 11 fail, the at-main COULD-NOT-CHECK above). The witness table is not landed here because that run stamped no on-behalf-of principal. It is re-run under the stamped write path once the rows are re-authored. Status stays `implemented` with no flip. Row re-authoring is routed to #1915 (https://github.com/medici-finance/assay/issues/1915#issuecomment-6004541583).

## Review
Gate: model (from frontmatter). The reviewer confirms the clause 14 rewrite keeps the planted-
second-instance fail-first obligation. Losing it would quietly weaken the class discipline this
stream relies on.
