---
brief: mistake-proofing/03
title: Typed Verify-row obligation classes — carry the prose MUSTs as data and derive their presence from the diff
why: >-
  Three of the authoring rules the fleet leans on hardest are prose MUSTs with no device behind them
  - a brief that adds a check must carry a mutation row, a change to a shared surface must carry a
  flow row, a deliverable making factual claims must carry a dereference row. Each is enforced today
  by a reviewer remembering it. The row-class column already exists, already validates against a
  closed set, and already hard-fails an unknown value, so the obligation can be carried as data and
  its PRESENCE derived from the shape of the change. This is the methodology's own first rule -
  load-bearing facts live in islands, not prose - applied to the authoring rules themselves, which
  are the largest body of load-bearing prose left in the system.
wave: 1
depends: ["mistake-proofing/01"]
unblocks: ["mistake-proofing/05", "mistake-proofing/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (b). The facts do not pre-specify the central design call: the four existing
  class values answer WHO EXECUTES a row, the four new ones answer WHAT OBLIGATION the row
  discharges, and these are orthogonal axes. Collapsing them into one closed set silently exempts
  every obligation-classed row from execution routing. The legacy-default hinge — a table with no
  class column keeps its historical behaviour — must survive the change untouched, or every
  inherited table changes meaning in one commit.
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the mistake-proofing board)
sources:
  - "`docs/mistake-proofing.md` §4 B2: 'the prose MUSTs of brief authoring SHOULD be carried as typed row classes whose presence the lint derives from the shape of the change and enforces. Presence is the control; adequacy stays review (D7).'"
  - "Same spec §3 D7 — 'Do not proof judgment.' The honest split for a judgment-adjacent surface is that the PRESENCE of the judgment artifact is checkable and its ADEQUACY is not; a device must state which half it covers."
  - "`docs/brief-rules.md` — the row-class column, the legacy default, and the 'Verify row semantics: dereferencing vs. presence' section this brief turns into typed data."
  - "The device inventory behind this stream (2026-08-25), cost S–M: the row-class parser is named 'the most under-used seam in the system'. It also records the constraint this brief must respect: the flow-row check was always planned as advisory because prose Verify rows cannot be perfectly classified — but the PRESENCE of a typed row is checkable even though its adequacy is not, and that is the honest split."
  - "Same inventory — no mutation row on a brief adding a check, no neighbour row on a shared-lister change, no flow row on a shared-value change: all prose MUSTs, all unenforced."
  - "depends mistake-proofing/01: the flow-row obligation is derived from a brief's DECLARED PATHS, and 01 is the brief that first makes that line readable by the lint. Building the derivation here without it means writing 01's parser twice."
  - "freshness-checked 2026-08-25 @ 657cab1 (origin/main) — the closed class set holds exactly four values, none of them an obligation class; no obligation derivation exists."
---

# Brief 03 — Typed Verify-row obligation classes

## Context

files:
- `statusgen/` (implementation home) — the row-class source file (the closed set, the legacy-default
  hinge, the class-validation check), the consumer-routing source file whose branch-diff helper is
  reused for the derivation, and tests.
- `plugins/assay/skills/author-brief/SKILL.md` — the authoring guidance the new classes must be
  documented in. **Coordinate with mistake-proofing/04**: once 04's generator exists, the
  enforcement-status half of that documentation is generated, not typed.

facts:
- The existing closed class set holds exactly **four** values, and they all answer one question:
  who executes this row. Two are machine-executed (one hermetic and re-executed network-off against
  the candidate tree, one deterministic but environment-bound and executed by a runner), and two are
  judgment routings (a model reads it, or a human does).
- **The legacy default is the compatibility hinge and is load-bearing.** A Verify table with no class
  column at all is legacy, and every one of its rows is treated as the environment-bound executed
  class. That is the single fact that made the column additive: every inherited table kept the exact
  behaviour it had, and only a table that opts in by adding the column gains new routing. Any change
  here that alters what a column-less table means is a corpus-wide behaviour change disguised as a
  lint improvement.
- The parser already records one extra bit beyond the value: whether the TABLE carried the column at
  all, so an explicitly-declared environment-bound row can be told apart from a legacy-default one.
  That bit exists because collapsing the two would silently exempt every legacy row from a gate.
  The obligation axis needs the same care.
- The four obligations to carry are named by the prose rules: **mutation** (this row breaks the thing
  the change guards and proves the guard reddens), **neighbour** (this row exercises a sibling site
  the change did not touch), **flow** (this row exercises the cross-component path end to end, not
  just the changed site), **dereference** (this row resolves a claim rather than counting its
  presence).
- The derivation inputs already exist. The branch's own three-dot diff is available through the
  consumer-routing check's helper, which pins the base to a concrete commit once per run so the whole
  pass is a pure function of base, head and tree. A brief's declared paths become readable in
  mistake-proofing/01. Between them, the shape of a change is computable without new machinery.
- **This brief derives PRESENCE only.** Whether a mutation row actually mutates anything, whether a
  flow row actually crosses a component boundary, whether a dereference row actually dereferences —
  none of that is decidable from row text, and attempting it is how a metric gets satisfied without
  the quality it proxies. A text-level lint must not be extended into judgement, and the line already
  drawn is the right one: *"this assertion is shallow"* stays with the reviewer; *"this assertion is
  inert"* belongs to the lint.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Decide and record the axis question first, in the source header, before writing code.** The
   four existing values route EXECUTION; the four new ones record an OBLIGATION. They are
   orthogonal: a mutation row is still executed by someone. Choose one of two encodings and state
   why in the header — a compound cell (an execution value, then a separator, then zero or more
   obligation tokens) or a second column. **Do not add the obligation values to the existing closed
   execution set**, which would force every author to choose between saying who runs a row and
   saying what it discharges, and would exempt obligation-classed rows from execution routing
   entirely. A brief-shaped worked example of the chosen encoding goes in the header.
2. **Extend the closed set with the four obligation values**, on the chosen axis, keeping unknown
   values fatal exactly as they are today. Preserve the legacy-default hinge byte for byte: a table
   with no class column must resolve exactly as it does now, and a test must assert that against a
   fixture copied from the inherited corpus.
3. **Derive the obligation from the shape of the change**, using the branch diff and the declared
   paths:
   - a diff that adds or changes a check-shaped path ⇒ a **mutation** row is owed;
   - declared paths spanning more than one top-level directory, or a task touching a shared-surface
     keyword, ⇒ a **flow** row is owed;
   - a deliverable that asserts checkable facts ⇒ a **dereference** row is owed. This last trigger
     is the least mechanically decidable of the three: derive it conservatively and let it
     under-fire rather than over-fire.
   - the **neighbour** obligation is defined and validated in this brief but its derivation trigger
     may be left for a follow-up if no honest signal exists; if so, say so in the header rather than
     shipping a guess.
   When the diff is unavailable — no git dir, an unresolvable base, a shallow clone — the derivation
   is **could-not-check** and reports as such. It does not fall back to "nothing is owed".
4. **Phase it: NOTICE first, then PROBLEM, and name the flip.** Land every derived obligation as an
   advisory NOTICE, exactly as prior lints on this surface were phased. Record a corpus census in
   the pull-request body. The mutation obligation's promotion to fatal is
   **mistake-proofing/06** — do not promote it here. Name in the pull-request body what the other
   three need before they can be promoted, or that they are intended to stay advisory and why.
5. **Say which half each message covers.** Every emitted line states that it checks the PRESENCE of
   the obligation row and that its adequacy is the reviewer's call. A message that reads as
   enforcement of quality will be believed, and then relied on. Carry a stable rule-tag bracket token
   on each line.
6. **Positive control per obligation.** For each derived obligation, a test that injects a change of
   the triggering shape with the row ABSENT and asserts the obligation is reported, plus the inverse
   with the row present asserting silence. Four obligations, eight cases minimum, plus the
   could-not-check case.
7. **Document the four values** in the authoring guidance, with the encoding and one example row
   each. Keep the enforcement-status wording minimal and factual — mistake-proofing/04 will generate
   that column, and a hand-written status line here becomes the next stale second copy.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -c -e classMutation -e classFlow -e classDereference -e classNeighbour -- statusgen/` | exit 1, no output — **DEREFERENCE, true at authoring (2026-08-25 @ `657cab1`)**: no obligation class exists in the closed set. Inverts at implementation |
| 2 | `git grep -n 'knownRowClasses' -- statusgen/rowclass.go` | exit 0; output contains `knownRowClasses` — **DEREFERENCE**: the closed-set seam this brief extends really exists |
| 3 | `git grep -n 'legacyRowClass' -- statusgen/rowclass.go` | exit 0 — **DEREFERENCE**: the legacy-default hinge this brief must preserve is real and named in source |
| 4 | `git grep -n 'func verifyRowClassProblems' -- statusgen/rowclass.go` | exit 0 — **DEREFERENCE**: the check that hard-fails an unknown class value is where the new values plug in |
| 5 | `git grep -n 'changedPathsSince' -- statusgen/consumers.go` | exit 0 — **DEREFERENCE**: the branch-diff helper the obligation derivation reuses ships today; no new diff machinery is needed |
| 6 | `go test ./statusgen/ -run 'RowClass' -count=1` | exit 0 — the extended class parser's tests pass, including the four new values and the unknown-value refusal |
| 7 | `go test ./statusgen/ -run 'RowClassLegacyDefaultUnchanged' -count=1` | exit 0 — **regression control**: a fixture table with no class column resolves exactly as before the change |
| 8 | `go test ./statusgen/ -run 'ObligationDerivation' -count=1` | exit 0 — **positive control**: for each derived obligation, the triggering diff shape with the row absent reports it, and with the row present is silent |
| 9 | `go test ./statusgen/ -run 'ObligationDerivationCouldNotCheck' -count=1` | exit 0 — an unavailable diff reports could-not-check, never "nothing is owed" |
| 10 | `git grep -c 'adequacy' -- statusgen/rowclass.go` | exit 0; a non-zero count — the D7 boundary (presence is the control, adequacy is review) is stated in the source the messages come from. Zero hits today (2026-08-25 @ `657cab1`); inverts at implementation |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Non-implementer verifier run — VERIFY: PASS — 2026-09-04 opus-4.8[1m]-verifier (verify-desk dispatch), merged main 4e500df

Runner != implementer. Offline (KUBECONFIG=/dev/null). gate: model, risk {all no}, irreversible: no. Rows 6-9 run inside the statusgen/ module.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | git grep -c classMutation classFlow classDereference classNeighbour -- statusgen/ | 0 | matches in rowclass.go, obligationderivation.go (+tests) | 2026-09-04 | opus-4.8[1m]-verifier |
| 2 | git grep -n knownRowClasses -- statusgen/rowclass.go | 0 | rowclass.go:90 | 2026-09-04 | opus-4.8[1m]-verifier |
| 3 | git grep -n legacyRowClass -- statusgen/rowclass.go | 0 | rowclass.go:86 const legacyRowClass = classCheck | 2026-09-04 | opus-4.8[1m]-verifier |
| 4 | git grep -n 'func verifyRowClassProblems' -- statusgen/rowclass.go | 0 | rowclass.go:311 | 2026-09-04 | opus-4.8[1m]-verifier |
| 5 | git grep -n changedPathsSince -- statusgen/consumers.go | 0 | consumers.go:949 | 2026-09-04 | opus-4.8[1m]-verifier |
| 6 | go test ./statusgen/ -run RowClass -count=1 | 0 | ok statusgen; 15 tests 0 FAIL | 2026-09-04 | opus-4.8[1m]-verifier |
| 7 | go test ./statusgen/ -run RowClassLegacyDefaultUnchanged -count=1 | 0 | ok; legacy no-class resolves unchanged | 2026-09-04 | opus-4.8[1m]-verifier |
| 8 | go test ./statusgen/ -run ObligationDerivation -count=1 | 0 | ok; 11 tests | 2026-09-04 | opus-4.8[1m]-verifier |
| 9 | go test ./statusgen/ -run ObligationDerivationCouldNotCheck -count=1 | 0 | ok; unavailable diff -> could-not-check | 2026-09-04 | opus-4.8[1m]-verifier |
| 10 | git grep -c adequacy -- statusgen/rowclass.go | 0 | count 2 (D7 boundary stated) | 2026-09-04 | opus-4.8[1m]-verifier |

**VERIFY: PASS** — all 10 rows ran; dereference rows 1/4/10 inverted authoring->implementation as specified. Advisory.

**RISK-VALUE: DERIVED** — obligationDerivationFatal = false @ statusgen/obligationderivation.go:53 — derived obligations land advisory NOTICE; fatal promotion assigned to mistake-proofing/06; true would flip every owed-but-absent obligation to a corpus-wide PROBLEM.
**RISK-VALUE: DERIVED** — flow threshold = 1 (topLevelSpan > 1) @ statusgen/obligationderivation.go:140 — spanning >1 top-level dir is the flow-obligation definition (crosses a component boundary); advisory, reversible.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) are execution and obligation on
separate axes, and is the reason recorded in the source header rather than only in the pull request?
(2) does a column-less legacy table resolve byte-identically, with a test asserting it? (3) does
every derived obligation have a positive control showing it fires on the absent case? (4) does each
message state that it checks presence, not adequacy?
