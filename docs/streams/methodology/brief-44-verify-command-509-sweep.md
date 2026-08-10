---
brief: methodology/44
title: 'Verify-command #509 sweep — fix every unfailable grep/go-test row, and close the detection gap the sweep list was hiding'
why: >-
  statusgen --lint flags Verify rows written so they can never actually fail — a row that passes
  whatever the file contains, or one that FAILS on its own success path. A Verify table that lies is
  worse than none: it launders "unverified" as "verified". These are mechanical, intent-preserving
  fixes; the value is that every one of these rows starts telling the truth when re-run. The second
  half is worth more than the first: the lint's output was being read as the complete set of broken
  rows when it never was, so this brief also extends detection to a class it was blind to and writes
  down, in the brief, exactly what remains undetectable.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [509]
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
amended: 2026-08-02 — absorbed F-impl-claims-unproven; scope, affected-row list, Task and Verify
  rewritten against the live lint (the frozen 2026-07-17 list had gone stale, and the DoD claimed a
  completeness the check did not have)
sources:
  - "statusgen --lint #509 notices — regenerate; do not trust a frozen count"
  - "issue #509 — Verify rows using `\\|` inside grep -E, and grep -c expecting 0"
  - "[[F-impl-claims-unproven]] case 3 (desk-tools/10) — a Verify row that fails as literally written
     because it passes a bare `url` placeholder the implementation rejects before the testing seam"
  - "freshness-checked 2026-08-02 @ origin/main 3f2b921: 47 #509 notices across 34 briefs — NOT the
     41/28 the brief was authored against; briefs moved repos in between"

---

# Brief 44 — Verify-command #509 sweep

## Context
files: the **Verify-table cells** of the affected briefs, plus `statusgen/verifyrows.go` and its tests
for the detection extension in Task 3. Touch no Status/Reviewed/risk cells and no brief bodies.

facts — **four bug classes the lint already detects, all deterministic to fix:**
1. **`\|` inside `grep -E` / `-cE` / `egrep` (ERE)** — there `\|` is a LITERAL pipe, not alternation, so
   the pattern matches almost nothing and the row passes whatever the file holds. **Fix:** rewrite the
   alternation as separate patterns: `grep -E -e alpha -e beta` (no pipe in the pattern at all). For a
   genuine literal pipe (matching a `| cell |`) use a bracket class `[\|]`.
2. **`grep -c …` (or `-ci`) expecting a count of `0`** — `grep -c` exits 1 when it matches nothing, so a
   row expecting 0 FAILS on the success path. **Fix:** gate on exit status instead — `! grep -qE …`
   (exit 0 when absent) — or neutralise the status — `grep -cE … || true`.
3. **A pipeline ending in `tee`** — the row reports `tee`'s exit status, not the check's.
4. **`\|` inside a `go test -run` pattern (#580)** — RE2, same defect as class 1, and worse: a `-run`
   that matches nothing prints "no tests to run" and exits **0**.

**Three caveats that bound the fix (do NOT get these wrong):**
- **BRE keeps `\|` as alternation.** `grep -c` / `grep -ci` WITHOUT `-E` are Basic regex, where `\|` IS
  alternation and works correctly. statusgen only flags `\|` under ERE (`-E`/`-cE`/`egrep`). So for a
  count-0 row that is BRE, fix ONLY the exit-status half (rule 2) — do NOT "upgrade" it to `-E` or touch
  its `\|`.
- **Unquoted `|` between commands is a real shell pipe** — correct table authoring; leave it. Only the
  `\|` INSIDE a grep pattern is the bug.
- **Class 2 cannot be fixed in the Command cell alone.** Turning `grep -c … | 0` into `! grep -q … |
  exit 0` moves the assertion from stdout to the exit status, so the Expect cell has to move with it or
  the row becomes internally inconsistent. **Editing the Expect cell is in scope for class-2 rows only,**
  and only to restate the same assertion — never to weaken it. (Class 4 is the same: proving a named
  test EXISTS needs `--- PASS`, because a `-run` miss exits 0, and that changes what Expect asserts.)

**Never make a row pass by making it check less.** A row that passes because it now asserts nothing is
the defect this brief exists to remove, not a fix for it. Every touched row must still fail if the thing
it was written to prove stops being true.

## Affected rows — regenerate, never read from this file
The set moves (briefs relocate between repos; new briefs land). Get the live set with:

```
go run ./statusgen --root . --lint 2>&1 | grep '#509'
```

At the 2026-08-02 sweep head that was **47 notices across 34 briefs** — not the 41/28 this brief was
authored against in July. **Any frozen list in a brief is a snapshot, not an inventory.**

**The `#509` set spans TWO repos, so `--lint` in one of them is half the answer.** This brief was
authored 2026-07-17, when every stream lived in `oit`; `assay-selfcontain/03` has
since moved the methodology streams into `assay-toolkit`. The sweep therefore lands as two sibling
PRs against one brief — that split is deliberate, not a duplicate. **Read Verify rows 1–2 as "both
repos' lints are clean of #509"**: run `go run ./statusgen --root . --lint` once per checkout, in
`assay-toolkit` (this PR) and in `oit` (sibling PR
[oit#1697](https://github.com/example-org/oit/pull/1697)).
A green count in one checkout says nothing about the other.

Non-obvious rows (intent unambiguous, fix non-trivial): `methodology/33` r3 (`(F\|I)` is an intended
alternation inside a bracket-heavy ERE → two `-e` patterns), `assay-product/05` r4 (two commands in one
cell, only the first of which the lint can see — see Detection limits), `methodology-metrics/17` r2 (the
Expect is a human judgement a count cannot make; the honest fix prints located hits, not a number).

## Detection limits — what `--lint | grep '#509'` does NOT catch
**This section is the point of the brief.** The lint output is a **lower bound** on broken Verify rows,
never the complete set. Reading it as authoritative is how a sweep can be run to completion and still
leave the class alive. **Six** known blind spots — 5 and 6 were found by the reviewer IN this PR's own
rewrites, which is the point restated one level up: a remedy can carry its own unfailable shape, and
neither the lint nor the author caught it. The first is unbounded; the others are recorded
with the live instances each one hides, so the next sweep starts from the real number rather than from
the lint's:

1. **Bare-word placeholders — UNDECIDABLE, no check exists or can.** `deskpushguard origin url` passes a
   metavariable named `url`; nothing in the command text distinguishes it from a literal argument that
   reads like one. Deciding it needs the callee's argument contract. This is exactly
   [[F-impl-claims-unproven]] case 3 (`desk-tools/10`), and it is why Task 3's new rule 5 is documented
   in-code as a lower bound rather than a gate. Catching this class needs a reviewer, not a lint.
2. **An unescaped `|` inside a grep pattern truncates the cell.** `grep -ciE 'plumb|ferrule' f` splits
   the markdown row at the `|`, so statusgen's cell splitter sees a truncated command and the Expect
   column shifts — the row is unfailable AND invisible to every rule above. Live instances:
   `desk-apps/01` **r3, r4 and r5** (all three carry a bare `|` in a `grep -ciE` pattern — an inventory
   of r5 alone was itself short by two, which is the failure mode this section exists to prevent),
   `methodology-metrics/23` r3, `methodology/34` r8. **Not fixed here** (it needs a splitter-level
   check, not a pattern rule).
3. **A terminal `grep` whose Expect asserts EMPTY output — class 2 in a different costume, and no rule
   catches it.** Rule 2 keys on `grep -c`; it is blind to a row that ends in a plain or inverted `grep`
   and expects nothing back, even though `grep` exits **1** when it selects no line. The row therefore
   fails on its own success path, exactly like a `grep -c` expecting `0`:

   ```
   $ grep -rniE 'issue-loop' src.txt | grep -viE -e 'docs/streams/issue-loop' -e 'intake-desk-scoping'
     exit=1        # 1 on the SUCCESS path; the Expect cell reads "empty"
   ```

   **10 live instances**, in two costumes. Inverted-filter pipelines (the `\| grep -v …` tail):
   `issue-loop/13` r4, `methodology/45` r2, `methodology/46` r4,
   `code-review-2026-07-23-canton-k8s/05` r3. Plain terminal greps expecting no match:
   `methodology/25` r5, `methodology/46` r3, `code-review-2026-07-23-canton-k8s/01` r4, `/03` r2,
   `/04` r2, `/04` r4. Rows ending in `wc -l`, in `; echo $?`, or negated with a leading `!` are
   **sound** and are excluded from that count — the neutraliser is what distinguishes them.
   **Not fixed here**: the fix is per-row (some want `\|\| true`, some want the assertion moved to the
   exit status) and the Expect cell moves with it, which is the class-2 coupling rule all over again.
   A separate shape worth a look but *not* counted above: `grep -L` exits **0 whether or not it prints**
   (`methodology/47` r2), so that row is unfailable by exit status in both directions.
4. **Only the FIRST code span in a Command cell is analysed.** A cell holding two commands
   (`` `cmd A` AND `cmd B` ``) is checked on `A` alone — `B` is unexamined whatever it does. Found on
   `assay-product/05` r4, where the second command carried a class-2 defect the lint never reported.
5. **A pipeline discards the exit status of everything left of the last `|` — including `go test`'s.**
   The class-4 remedy (`go test … -v 2>&1 \| grep -q -- '--- PASS'`) proves the named test EXISTS but
   reports only `grep`'s status, so a group holding one passing and one FAILING test still goes green.
   Measured: with `t.Errorf` injected into `TestStripQuotedRegions`, `go test -run StripQuotedRegions`
   exits 1 while the piped row exits **0**. No rule catches it — the shape is textually identical to a
   sound one. **Fixed in this sweep** on all 8 instances (`issue-loop/14` r4, `methodology/23` r5,
   `/44` r4, `methodology-metrics/10` r1, `/17` r3, `/20` r1, `/25` r4, `/37` r5) by capturing the run
   once and asserting its status first: `out=$(go test … -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf
   '%s' "$out" \| grep -q -- '--- PASS'; } \|\| …`. Note bare `set -o pipefail` is NOT the remedy:
   `grep -q` exits on first match and SIGPIPEs `go test`, producing a false red on a clean tree.
   **5a — the same defect with `!` in front and any tool behind it, added 2026-08-03.** `! TOOL … 2>&1
   \| grep -q 'PATTERN'` is strictly worse than the `go test` case: a TOOL that is missing, unbuilt or
   crashing writes its error INTO the pipe, `grep -q` matches nothing, and the leading `!` converts the
   tool's failure into a **pass**. Measured on `methodology/24` r1 (`! statusgen --root . --lint 2>&1 \|
   grep -q 'has no gate-why'`) with the binary off `PATH`: **rc=0**, a vacuous pass; the same check
   written `TOOL … > /tmp/f 2>&1 && ! grep -q 'PATTERN' /tmp/f` gave **rc=127**. Rule 3 does not catch
   it — that rule is deliberately scoped to `tee`, and here the last stage is `grep`, whose status can
   legitimately be the gate. Remedy: capture to a file, gate on `&&`, negate the `grep` alone. **Fixed
   in this sweep** on `methodology/24` r1 and on this brief's own r2 (which piped the lint into
   `grep -c`, so a build failure printed `0` — read by the Expect cell as "the rows were fixed").
   Prefer `go run ./statusgen` over a bare `statusgen`: a stale binary on `PATH` is not the tree
   under test.
6. **`! grep … FILE` returns 0 when FILE does not exist.** `grep` exits **2** on a missing target and
   the leading `!` inverts that to a pass — so the class-2 remedy (moving a count-0 assertion onto the
   exit status) silently converts "this file was never read" into a green row, and the Expect cell now
   reads `exit 0`, which is what a missing file also produces. The old `grep -c …` form exited 2 with
   no stdout, which no one could record as a pass without lying. No rule catches it. **Fixed in this
   sweep** on all 19 negated-grep rows by prefixing an existence assertion — `test -f FILE && ! grep …`
   (`test -d` for a recursive directory target) — so a moved or absent target now exits 1, loudly.
   Measured after the fix: **15 of the 19** name a target absent from this checkout and now exit 1 for
   that reason (6 × the assay-site landing page, 4 × a removed `docs/articles/` piece, the deskd
   `deploy/` tree, the pitch-deck directory, the assay-site public index, and the in-repo batch-fanout
   skill file); the other 4 resolve and
   exit 0. A red there is the row working, not the row broken — it says "this table cannot be evaluated
   in this checkout", which is what the silent `exit 0` was concealing.

## Task
1. Fix every row in the live `#509` set per the four rules + three caveats. Edit the Command cell, and
   the Expect cell only where class 2 or 4 requires it (above).
2. For rows in `done`/`verified` briefs: the past verification stands — you are correcting the command
   text for future re-runs only. Do NOT change their Status or Evidence, and do not touch an Evidence
   block that records a command as it was actually run.
3. **Close the gap on the decidable half of the placeholder class**: add statusgen rule 5 —
   `unsubstitutedMetavars` — flagging a Verify command that carries an angle-bracket metavariable
   (`<N>`, `<mm/22 workflow file>`) or an ellipsis elision (`...`, `…`) outside quotes, because such a
   row cannot run as written. Quoted text is exempt (`"<script[^>]*src="` is a regex); so are `./...`,
   heredocs, redirections and `<(…)`. Emit NOTICE, matching the phased path rules 1–4 took. **The rule's
   own doc comment must state that the bare-word shape is undecidable and that the check is a lower
   bound** — a check that silently implies completeness is the same defect one level up.
4. Backfilling the rows rule 5 finds is **NOT in this brief's scope** — it is a distinct class with a
   distinct fix per row (some placeholders are genuinely unknowable until run time). Record the census
   in Verify row 2 so a follow-up brief has a baseline, and note the follow-up in the PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go run ./statusgen --root . --lint > /tmp/lint44.txt 2>&1 && ( grep '#509' /tmp/lint44.txt \| grep -cv 'unsubstituted placeholder' \|\| true )` | `0` — **no** rule-1..4 notice remains, in any brief. No deferral carve-out: `assay-dogfood/brief-04` r2 and `methodology/brief-09` r7 were the last two and are now fixed. The lint is captured to a file rather than piped, so a statusgen that fails to build goes red here instead of emptying the pipeline into a vacuous pass. Per the note above, this is one repo's half — the sibling PR carries the other |
| 2 | `go run ./statusgen --root . --lint > /tmp/lint44m.txt 2>&1 && grep -c 'unsubstituted placeholder' /tmp/lint44m.txt` | `62` at the sweep head — the rule-5 census. Not zero, and not meant to be: Task 4 puts the backfill out of scope, so this row records the baseline a follow-up brief measures against. A number BELOW 62 means rows were fixed; ABOVE means new ones landed. Was `61`; re-measured after merging `origin/main` at `1d2e009`, which landed `docs/streams/assay-dogfood/brief-02-skills-bundle.md` row 9 (`… .claude/skills/<skill>/SKILL.md`) — a new rule-5 row authored elsewhere, not one this sweep introduced or missed. The baseline moves with the tree by design; that is the row working, not a retune. Form amended 2026-08-03 for consistency with r1 and r5: the lint was piped straight into `grep -c`, which discards `go run`'s status — a statusgen that fails to BUILD writes its error into the pipe, `grep -c` prints `0`, and the cell reads that `0` as "every rule-5 row was fixed". Captured to a file and gated on `&&`, a build failure now exits non-zero before `grep` runs at all |
| 3 | `go test ./statusgen/ -count=1 && go vet ./statusgen/` | exit 0 — the whole statusgen suite, including the rule-5 cases added to `TestUnfailableRowRules` and the fixture-brief count in `TestUnfailableRowNoticesOnFixture` |
| 4 | `(rc=0; for t in StripQuotedRegions UnsubstitutedMetavars; do out=$(go test ./statusgen/ -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — both rule-5 unit tests EXIST and pass. Asserted from `--- PASS` because a `-run` pattern matching no test exits 0 with "no tests to run". Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite |
| 5 | `go run ./statusgen --root . --lint > /tmp/lint44p.txt 2>&1 && ! grep -q '^PROBLEM' /tmp/lint44p.txt` | exit 0 — no PROBLEM on the tree (baseline was 0; the sweep introduces none). Captured to a file rather than piped, for the same reason as row 1: under the old `! go run … \| grep -q` form a statusgen that failed to build wrote its error to the pipeline, `grep` matched no `^PROBLEM`, and `!` turned the build failure into a pass. The `&&` now propagates a non-zero `go run` |
| 6 | `gh pr diff 372 -R medici-finance/assay-toolkit --name-only > /tmp/pr372files.txt && test -s /tmp/pr372files.txt && ! grep -qv -e '^docs/streams/' -e '^statusgen/' /tmp/pr372files.txt` | exit 0 — the PR's file list is NON-EMPTY and every path in it is under `docs/streams/` or `statusgen/`. Anchored to the PR's own three-dot file list rather than `git diff --name-only origin/main`: on merged main that diff is empty by construction, `grep -qv` on empty input exits 1, and the old row therefore printed its expected `1` having examined nothing — exactly the vacuous pass a non-implementer re-running this table would have recorded. `test -s` makes an empty or failed listing exit 1; any out-of-scope path exits 1 |
| 7 | `gh pr diff 372 -R medici-finance/assay-toolkit --name-only > /tmp/pr372files.txt && test -s /tmp/pr372files.txt && ! grep -q '^STATUS.md$' /tmp/pr372files.txt` | exit 0 — `STATUS.md` is absent from a NON-EMPTY PR file list (single-writer rule). Same anchoring fix as row 6: the previous `! git diff --name-only origin/main \| grep -q …` form returned 0 on an empty diff, so it passed vacuously on merged main. Add `STATUS.md` to the branch and this row exits 1 |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.

**For the reviewer, specifically:** rule 5 is an addition to the anti-falsification lint. It is NOTICE-only
and changes no gate's pass/fail, but it is the same file as the integrity checks — judge whether that
warrants a human gate rather than the model gate this brief carries.
