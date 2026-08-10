---
brief: methodology-metrics/21
title: 'Consumer-enumeration gate — a shared-value brief carries a consumers: list whose routing claims are corroborated against the diff, not merely present'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [159]
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction; today's sweep finding)
sources: ["author-brief rule 6 / brief-rules.md rule 9 (SHARED VALUE → enumerate consumers + verify the FLOW — the prose being mechanized)", "the party-identity incident that forced the rule (frontend fallback assumed the old coupling; no CollateralVault ever minted)", "F-impl-claims-unproven (2026-07-23) — the redesign driver: implementer-written claims reach `implemented` uncorroborated, and presence-checking machinery makes them look enforced", "docs/three-state-instrument-rule.md (checked-clean / checked-failed / could-not-check)", "assay-toolkit#159 (the port issue; its open grammar questions are settled here)", "methodology/31 (#216 — same 'this brief touches a shared surface' trigger derivation)", "issue #272 (the cross-REPO sibling of this cross-COMPONENT case)"]
value: high
why: >-
  brief-rules rule 9 ('a brief changing a shared value must enumerate consumers and verify
  the flow') is prose a strong author follows and a cheap-tier worker silently skips — it
  slipped again in the 2026-07-10 sweep. Mechanizing it is only worth doing if the
  mechanism can tell a true consumer list from a plausible-looking one: a routing list
  nothing corroborates is a second place for confident, unchecked claims to live, which is
  the defect F-impl-claims-unproven is about, not a fix for it.
consumers:
  - "statusgen/consumers.go: fixed-here (routing grammar, site classification, the offline lint half, the --consumers corroboration mode)"
  - "statusgen/consumers_test.go: fixed-here (the red/green pairs — the gate is only a gate where it has been shown to fire AND to clear)"
  - "docs/streams/methodology-metrics/README.md: fixed-here (this brief's status row)"
  - "statusgen/brieffile.go: fixed-here (the consumers:/ConsumersProse frontmatter fields)"
  - "statusgen/main.go: fixed-here (--consumers/--base/--brief flags; the lint-half wiring in run(); the single-root sub-command refusal list — a multi-root run must not corroborate one repo and report the rest clean unlooked-at)"
  - "statusgen/README.md: fixed-here (the sub-command list that states which flags refuse multi-root)"
  - "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (the cross-module-reader registry flags consumers_test.go for its root-escape REJECTED-INPUT fixture; classified as an opt-out with the reason, not silenced)"
  - "docs/brief-rules.md: fixed-here (rule 9 gains the field, the three-state verdict table and the Verify-row obligation)"
  - "docs/brief-template.md: fixed-here (the frontmatter field and the corroboration Verify row)"
  - ".github/workflows/statusgen.yml: fixed-here (the PR-time gate step — this is what makes the check live rather than available)"
  - "~/.claude/skills/author-brief/SKILL.md and the oit copy: out-of-scope (both live outside this repo, under oit's #221 out-of-repo-edit protocol; docs/brief-rules.md is this repo's canonical authoring rule and the gate reads brief files, not skills — a reviewer confirms neither skill copy states enforcement this repo does not have)"
  - "adopter repos pinning statusgen via .assay-versions: out-of-scope (the flag reaches them only through a statusgen release + pin bump — the cut-release path, tracked separately from this brief)"
  - "the 12 briefs already carrying consumers: on main: out-of-scope (a closed brief is never rewritten to satisfy a gate added after it landed — that edit is the falsification this subsystem exists to catch; consumersCheck suppresses shape notices on verified/done for the same reason)"
---

# Brief 21 — Consumer-enumeration gate

## Context
files: `statusgen/consumers.go` (+ test), `statusgen/brieffile.go`, `statusgen/main.go`,
`docs/brief-rules.md` rule 9, `docs/brief-template.md`, .github/workflows/statusgen.yml
facts:
- **`consumers:` is an optional brief-v1 frontmatter list.** Each item is
  `<path-or-component>: fixed-here | follow-up <stream>/<NN> | out-of-scope (<why>)`,
  with trailing detail allowed on any routing. Absent = the brief asserts it changes no
  shared value (the default, true of most briefs).
- **Every entry is an implementer-written claim about the implementer's own work**, and is
  treated as such. `statusgen --consumers` corroborates each one against the branch's own
  three-dot diff and reports `CORROBORATED` / `DISPROVED` / `UNCHECKED`. `DISPROVED` exits
  1; an unreachable diff exits 2; `UNCHECKED` is printed and counted, never a silent pass.
- **Severity splits by what the instrument established, not by the brief's age.** `--lint`
  runs corpus-wide with no diff, so it hard-fails only on what the stream tables alone
  disprove — a `follow-up` naming a brief that does not exist. Everything a diff would be
  needed to settle is a NOTICE there and the gate for it is `--consumers`.
- **The unit of judgement is the ENTRY, not the brief file.** Scoping to "briefs this branch
  touches" is not the same as "claims this branch makes": a verify-desk Evidence commit, a
  status flip or a typo fix edits a merged brief's file while changing none of its claims,
  and putting its year-old `fixed-here` entries on trial against the editor's diff disproves
  all of them. (Measured on main: 6 of the 18 briefs carrying `consumers:` exited 1 the
  moment any PR touched their file — the false-positive class that gets diff-keyed checks
  switched off, firing on people who changed nothing about the claim.) So an entry
  byte-identical to the one at the merge-base is `UNCHECKED` with that reason stated; an
  entry this branch wrote or rewrote is judged. `--brief <id>` lifts the file scoping but
  not the need for a diff — on merged main it exits 2, `COULD-NOT-CHECK`, and prints the
  recipe for handing it the branch diff that made the claims.
- **Deletion counts as fixing.** A `fixed-here` path absent from the tree but present in
  the diff is corroborated — the branch removed the consumer. Offline this is genuinely
  ambiguous (typo vs deletion), which is why the offline half declines to rule on it.
- **This is the cross-COMPONENT case; #272 is the cross-REPO case** — they compose: a
  change crossing both boundaries carries a `consumers:` list (here) and a sibling SHA
  (#272).
- Offline and deterministic apart from one `git diff --name-only <base>...HEAD`; no network.

### Why this design changed — F-impl-claims-unproven (amended 2026-08-02)

The version of this brief authored 2026-07-10 specified a lint that checked the
`consumers:` field was **present**, with the routing token matched against a regex. That
is the defect [[F-impl-claims-unproven]] describes, reproduced at a new site:

> implementer-written evidence-claims (test names, deliverable refs) belong ONLY in the
> Verify table's "Expect" column — which the verifier corroborates — NOT as asserted fact
> in the brief body.

A `consumers:` entry is exactly such a claim — "this reader is fixed here" — and a
presence check cannot distinguish a true list from a plausible one. Worse, shipping the
check would have made the uncorroborated claim *look* enforced: a green `--lint` next to a
`consumers:` block reads as "the consumers were verified" to every later reader. The
finding's three cases all failed that way — a brief naming three tests that did not exist,
a brief whose deliverable was never landed, a Verify table that failed as literally
written — and in each the record said otherwise for days.

So the field stayed and the checking changed:

| Half of the claim | Who settles it | Where it lives |
|---|---|---|
| `fixed-here` — the named path really is in this change | `statusgen --consumers` vs the branch diff | exit 1 when the diff contradicts it |
| `follow-up <stream>/<NN>` — a brief really carries the deferral | stream tables (exists) + the target's own text (references back) | PROBLEM in `--lint` when the target does not exist |
| `out-of-scope (<why>)` — the exclusion is *legitimate* | nobody, mechanically — it is a judgement about intent | `UNCHECKED`, restated in the Verify row's Expect column for the reviewer |
| out-of-repo and prose-named consumers | nobody from this root — the artifact is not visible | `UNCHECKED` with the reason named |

The uncorroborable residue is not deleted and not softened into frontmatter that reads as
settled: it is *named*, counted in the summary line, and handed to the reviewer in the
Verify "Expect" column, which is where the finding says implementer claims belong.

Two consequences worth stating because they look like scope cuts and are not:
- **The flow-Verify-row check stays advisory and heuristic**, as originally specified.
  Prose Verify rows cannot be classified as site-local vs cross-component with any
  precision, so this remains a prompt to the author. What replaced the missing rigour is
  the corroboration row itself, which is machine-run and cannot be satisfied by wording.
- **The shared-value trigger is a prompt, never a verdict** — deliberately narrow phrases
  ("shared value", "consumers of", "wire format", …) and never a PROBLEM. Single common
  words (`secret`, `party`, `identity`, proposed on #159) were dropped: on a prose corpus
  they fire everywhere, and an advisory that fires everywhere is trained away rather than
  acted on. Measured on this repo's corpus at implementation time (Verify row 6): 11 of
  ~160 open briefs fire, and all 11 are true positives — briefs enumerating consumers in
  prose that have no structured field.

### Settling assay-toolkit#159's two open questions
- **`^fixed-here$` is too strict** — real entries carry the detail that makes them useful
  (`fixed-here (--changed input + darInDomain gate)`). Conforming to the strict form means
  deleting the most informative part. Settled: trailing detail is allowed on every routing,
  and the routing token is located by known-prefix search taking the rightmost match, so a
  reason containing `": "` (`out-of-scope (blocked: #123)`) still parses.
- **`follow-up` with no target** — settled as a NOTICE naming the gap ("a follow-up nobody
  owns is a deferral with no holder"), not a rewrite of the entry. Four briefs on main use
  the bare form; silently reinterpreting them would flip a logged deferral into a logged
  exclusion.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. `statusgen/consumers.go` (+ `consumers_test.go`): the routing grammar, site
   classification (repo path / out-of-repo / prose), the offline `--lint` half, and the
   diff-aware `--consumers` corroboration mode with three-state output and exit codes
   0/1/2.
2. `statusgen/brieffile.go`: parse `consumers:` (a scalar prose form is captured, not
   rejected — several briefs on main use it and a hard error would red-gate the board).
3. `statusgen/main.go`: `--consumers` / `--base` / `--brief` flags; wire the offline half
   into `run()`.
4. `docs/brief-rules.md` rule 9 + `docs/brief-template.md`: the field, the three-state
   verdict table, and the obligation to carry the corroboration Verify row with the
   UNCHECKED residue in its Expect column.
5. .github/workflows/statusgen.yml: run the gate on every PR. A check that exists but is
   wired into nothing is availability, not enforcement.

## Verify (executable — no prose-only DoD items)
Run from the repo root unless noted. Rows 2–3 are the mutation-test pair (brief-rule 16):
row 2 proves the gate goes RED on a false claim, row 3 proves it goes GREEN when the claim
is corrected — a gate never shown to fire is not a gate.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test ./... && go vet ./... && gofmt -l .` | exit 0, no files listed |
| 2 | `yq -i --front-matter=process '.consumers += ["statusgen/NOT-A-REAL-FILE.go: fixed-here"]' docs/streams/methodology-metrics/brief-21-consumer-enumeration-gate.md; cd statusgen && go run . --root .. --consumers --base origin/main; echo $?` | **1** — output names the entry `DISPROVED`; restore the file afterwards. (`--front-matter=process` is required: the brief is markdown, and plain `yq -i` errors out on the body rather than editing the frontmatter) |
| 3 | `git checkout docs/streams/methodology-metrics/brief-21-consumer-enumeration-gate.md; cd statusgen && go run . --root .. --consumers --base origin/main; echo $?` | **0** — every `fixed-here` entry `CORROBORATED` (10). The summary also reports 3 `UNCHECKED`: the out-of-repo author-brief skill copies, the adopter-repo pin path, and the 12-brief backfill. **Those three are the reviewer's call, not the tool's** — confirm (a) neither author-brief copy claims enforcement this repo does not have, (b) the pin-bump route is the real one for adopters, (c) rewriting closed briefs to conform is correctly refused |
| 4 | `cd statusgen && go test -run FollowUp -v` | 3 tests PASS, not 0: `TestFollowUpToNonexistentBriefIsProblem`, `TestFollowUpBackReferenceCorrectsIt`, and `TestUnrunRoutedFollowUpIsCleanButStillMarked` — the offline half hard-fails a follow-up to a nonexistent brief, clears once the target references back, and a routed follow-up that ran nothing stays marked rather than passing silently |
| 5 | `cd statusgen && go test -run ConsumersCouldNotCheck -v && go test -run ConsumersUnchecked -v` | both PASS — could-not-check exits 2, and UNCHECKED entries carry a stated reason instead of reading as passes |
| 6 | `cd statusgen && go run . --root .. --lint 2>&1 \| grep -c 'enumerates no consumers'` | 11 — the measured firing count of the shared-value prompt on this corpus (the figure quoted in the design note; re-measure, do not assume) |
| 7 | `statusgen --root . --lint; echo $?` | 0 |
| 8 | `grep -c 'UNCHECKED' docs/brief-rules.md` | ≥1 — rule 9 states the three-state contract and where the unsettleable residue goes |
| 9 | `grep -n 'consumers' .github/workflows/statusgen.yml` | the PR job runs `--consumers --base origin/main`; the gate is wired, not merely available |
| 10 | On merged main: `cd statusgen && go run . --root .. --consumers --base origin/main --brief methodology-metrics/21; echo $?` | **2** — stderr says `COULD-NOT-CHECK`, names that no entry was corroborated *and* none was disproved, and prints the re-run recipe. `--brief` lifts the diff SCOPING, not the need for a diff: after the merge HEAD equals the base, and the branch that made the claims true is gone. Exit 1 there would hand the verifier a page of false `DISPROVED`s; exit 0 would be a pass nothing established |
| 11 | The post-merge verifier path, run as row 10's message instructs — for this PR's merge commit `M`: `git checkout $M^2 && cd statusgen && go run . --root .. --consumers --brief methodology-metrics/21 --base $(git merge-base $M^1 $M^2); echo $?` | **0** — 10 `CORROBORATED`, 3 `UNCHECKED`, same as row 3. This is the row that proves the recipe printed at row 10 is real and not advice |
| 12 | `cd statusgen && go test -run InheritedClaims -v > /tmp/at371-r12.txt 2>&1 && go test -run BriefFilterWithoutDiff -v >> /tmp/at371-r12.txt 2>&1 && test "$(grep -c '^=== RUN   [A-Za-z0-9_]*$' /tmp/at371-r12.txt)" = 2; echo $?` | **0** — both PASS *and* exactly 2 top-level tests actually ran. An entry unchanged since the merge-base is `UNCHECKED` (the branch that edits a merged brief's Evidence did not make its claims), and an entry that branch *rewrites* is judged again. Measured on main before the fix: 6 of the 18 briefs carrying `consumers:` exited 1 the moment any PR touched their file; after it, 0 of 18. The count assertion is the point: `go test -run` exits **0** when its pattern matches nothing, so a row that only checks `go test`'s exit code passes having run nothing — the same could-not-check-reads-as-pass defect this brief exists to close. Typo either selector and the row exits 1 |
| 13 | `cd statusgen && go test -run RoutingTokenIsNormalized -v > /tmp/at371-r13.txt 2>&1 && go test -run UnroutableEntry -v >> /tmp/at371-r13.txt 2>&1 && go test -run WildcardSite -v >> /tmp/at371-r13.txt 2>&1 && go test -run DirectorySiteMatchesItsContents -v >> /tmp/at371-r13.txt 2>&1 && go test -run OutOfScopeContradicted -v >> /tmp/at371-r13.txt 2>&1 && test "$(grep -c '^=== RUN   [A-Za-z0-9_]*$' /tmp/at371-r13.txt)" = 5; echo $?` | **0** — all PASS *and* exactly 5 top-level tests actually ran (the `[A-Za-z0-9_]*$` anchor counts top-level tests only, excluding `TestRoutingTokenIsNormalized`'s 6 subtests). The `DirectorySiteMatchesItsContents` selector is spelled in full on purpose: the shorter `DirectorySite` also matches row 15's test, and the count assertion caught exactly that when row 15 was added — which is the guard doing its job. These are the ways a claim could dodge or falsely trip the gate: a doubled space or capital letter no longer demotes `DISPROVED` to `UNCHECKED`; a glob claims its whole set instead of passing on one match; a directory site matches its contents instead of being falsely disproved; `out-of-scope` on a file this diff edits is a self-contradiction the diff settles; an entry stating no routing is disproved, not skipped. Typo any one selector and the row exits 1 |
| 14 | `cd statusgen && go test -run ChangedPathsSinceReadsRealGit -v` | PASS — the code that decides what the diff IS runs against a real git repo: three-dot excludes commits made on the base after the branch point, and the porcelain rename split reports both halves. Every other gate test substitutes the diff, which left this seam green-but-untested |
| 15 | `cd statusgen && go test -run DirectorySiteShowsItsEvidence -v > /tmp/at371-r15.txt 2>&1; test "$(grep -c '^=== RUN   [A-Za-z0-9_]*$' /tmp/at371-r15.txt)" = 1; echo $?` | **0** — PASS *and* exactly 1 top-level test actually ran (the same could-not-check-reads-as-pass shape rows 12–14 close: `go test -run` exits 0 having matched nothing, so a row checking only the exit code passes having run zero tests. Typo the selector and this row exits 1). The test itself: a corroborated *directory* site names the paths it rests on and states the exact count — including above the 5-path cap, where the count stays exact and the shown list carries a `+N more` suffix — and a plain *file* site still prints no reason. A directory is touched when anything under it is (correct, and kept), which makes a trailing slash the cheapest widening; printing a bare `CORROBORATED` for it left `docs/: fixed-here` and `docs/brief-rules.md: fixed-here` byte-identical in the log, so the gate corroborated the entry and told nobody how thinly. Mutation-proven four ways at the matching assertion: emptying the reason (equivalently, gutting `directoryEvidence` to return `""`), inflating the count, emptying the path list, or stopping collection at the first match (`touchedBy`'s loop) each redden this test; dropping `sort.Strings(out)` makes the path order a coin flip on Go's randomized map iteration rather than a deterministic red. (Re-review finding 2: the original commit message claimed a fifth kill — dropping the `changed[rel]` file-site guard in `directoryEvidence` — but that guard is redundant with the very next `!st.IsDir()` check for every input the diff can actually produce, so removing it alone leaves the suite green; only removing both guards reddens the control assertion at consumers_test.go's file-site case. The guard is harmless and stays; the claim is corrected to four.) |

## Evidence
<!-- non-implementer rows. -->

## Review
Gate: model. Reviewer confirms:
(a) **no check reports a claim it cannot establish** — every `--lint` PROBLEM is something
the stream tables disprove outright, and everything diff-dependent is either a NOTICE or a
`--consumers` verdict;
(b) the three `UNCHECKED` entries in this brief's own `consumers:` are correctly
unsettleable, and their judgement is stated in Verify row 3's Expect column rather than
asserted in the frontmatter (this is the F-impl-claims-unproven correction — check it on
the brief's own dogfooded list, not just on the code);
(c) the mutation pair (rows 2–3) actually fires — run it, do not read it;
(d) the shared-value prompt and the flow-row heuristic are described as advisory
everywhere they appear, and neither can become a PROBLEM;
(e) diff-scoping cannot red-gate a brief the branch did not touch
(`TestConsumersUntouchedBriefsAreNotJudged`);
(f) nothing in either author-brief skill copy (out-of-repo, UNCHECKED above) claims
enforcement that only exists in this repo.
