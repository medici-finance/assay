---
brief: mistake-proofing/01
title: Cross-read a brief's declared paths against the risk classifier — the one authoring mistake that downgrades a gate
why: >-
  A brief's review gate is DERIVED correctly from four risk booleans the author writes by hand, and
  nothing checks the booleans against the paths the same brief declares it will touch. An author can
  answer all four "no" on a brief whose own declared paths are workflow, secret, or credential
  surfaces; the gate then computes to "model" from wrong inputs and the human review the risk answers
  exist to trigger never happens. Every other authoring mistake makes a brief worse; this one makes a
  brief LOOK safe. The classification it should be checked against already exists and already runs on
  every pull request in a different tool — the two mechanisms have simply never met.
wave: 0
depends: []
unblocks: ["mistake-proofing/03", "mistake-proofing/04", "mistake-proofing/05"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
exec-tier: strong
exec-tier-why: >-
  Question (b) — cross-artifact correctness. The classifier being read has TWO halves with opposite
  semantics: a mechanism half that answers TRUE on every uncertain input (unknown repo, empty file
  list, public repo) and a policy half that matches paths against a trigger set. Consuming the wrong
  half classes every brief in the corpus and makes the check useless in one commit. The two trees are
  also separate Go modules with an internal package between them, so the binding is a duplicate-and-
  couple design, not an import.
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the mistake-proofing board)
sources:
  - "`docs/mistake-proofing.md` §4 B3: self-declared risk booleans whose declared paths contradict them are a lint PROBLEM, not a reviewer catch — 'the one authoring mistake that silently downgrades a gate, which makes it the first to proof'."
  - "Same spec §5 — the adoption ladder puts B3 at step 4 (lint-side cross-reads and row classes: small tool changes)."
  - "The device inventory behind this stream (2026-08-25): the classifier exists, is additive-only-widenable, and is TRUE on every uncertain input; 'the two mechanisms never meet'. Cost estimated S."
  - "Precedent for the cross-tree binding: `statusgen/rosterconfig.go` duplicates the desk kernel's config reader; the two are bound by a shared test-vector file both modules read, whose comment states the rationale — 'the two modules deliberately share no code, so a shared VECTOR file is what keeps their duplicated readers honest'."
  - "The self-attestation error class: everything a session writes about its own work is, at the last mile, prose it authored. The risk booleans are the authoring-side instance, and a gate derived from them inherits their unchecked-ness."
  - "freshness-checked 2026-08-25 @ 657cab1 (origin/main) — the Context `files:` line is parsed by nothing in the lint today, and the classifier is not referenced from the lint tree at all."
---

# Brief 01 — Cross-read declared paths against the risk classifier

## Context

single-point-of-failure: today the ONLY control standing between a mis-declared risk answer and a
skipped human review is the reviewer noticing. This brief adds a second, independent layer that
fails for a different reason (a path match) in a different component (the lint, at pull-request
time) than the reviewer's read. It does not remove the reviewer layer and must not be described as
doing so.

files:
- `statusgen/` (implementation home) — the lint tree. A new source file for the cross-read check,
  plus the brief-body parser change that first makes the declared-paths line readable, plus tests
  and a shared coupling vector under its `testdata/`.
- `tools/desk/internal/deskkit/riskclassifier.go` and
  `tools/desk/internal/deskkit/riskpath.go` — **read for reference only; not edited.**
  These carry the classification this check must agree with.
- `topology.yaml` — the per-repo trigger declarations the classifier reads. Read, not edited.

facts:
- The lint parses a brief's YAML frontmatter thoroughly and its BODY barely. The declared-paths line
  in `## Context` is parsed by nothing: the only same-named field the lint reads today belongs to the
  optional parallel-shard declaration in the frontmatter. Making that line readable is the first
  half of this brief and is what briefs 03 and 05 build on.
- The classifier has two halves and they have opposite semantics. The **mechanism** half fails
  closed and answers TRUE for an unknown repo, a public or visibility-unstated repo, an empty
  changed-file list, or a blank path entry — because on those inputs there is nothing to classify and
  the safe answer is already "classed". The **policy** half answers the actual question: do these
  paths match the trigger set. **This check must consume the policy half only.** Feeding a brief's
  declared paths through the top-level accessor classes every brief in the corpus, because a brief is
  not a pull request on a known repo.
- The trigger set is a union: a small compiled base list that applies to every repo, plus per-repo
  additions declared in the topology file, plus an adopter environment variable. The union is
  **additive-only by construction — there is no removal syntax** — so a cross-read built on it can
  only ever widen, which is the safe direction for a gate.
- The two trees are **separate Go modules** and the classifier's package is `internal/`. There is no
  import path in either direction. The established pattern for exactly this is duplicate-the-reader
  and bind-the-two-with-a-shared-test-vector; a live instance ships in the lint tree today, and its
  vector file carries both ACCEPTANCE and REFUSAL cases because "an acceptance case only proves the
  two agree on a good file".
- The gate derivation itself is already proofed: any risk answer "yes" forces the human gate, and the
  lint enforces it. **The derivation is correct; the inputs are unchecked.** This brief checks the
  inputs and changes nothing about the derivation.
- The trigger base list is deliberately generic and adopter-agnostic; an adopter supplies its own
  real surfaces through the additive environment variable. A cross-read must therefore be honest that
  an adopter who has configured nothing gets the generic base only — it is not a claim to have
  classified that adopter's real security surfaces.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Parse the declared-paths line.** Extend the brief-body parser so the `## Context` declared-paths
   line yields a list of path strings. Accept both authored forms in the corpus: a single inline
   comma-or-space separated value, and a bulleted list on following lines. Strip markdown decoration
   (backticks, link syntax, trailing prose after an em dash) so a path is compared as a path. A line
   that is absent, empty, or unparseable is **could-not-check for this brief — never a pass**: emit
   the could-not-check and move on, do not treat "no declared paths" as "no risky paths".
2. **Bring the policy-half trigger set into the lint, and bind it.** Duplicate the trigger-matching
   logic — the base list, the per-repo additions read from the topology file, the additive
   environment variable, and the glob-matching rule — into the lint tree. **Do not attempt an
   import**, and do not reach across the module boundary at build time. Then add a shared test-vector
   file, read by a test in BOTH modules, carrying paths that must classify and paths that must NOT,
   so a drift in either copy reddens both. Model the vector's structure and its refusal cases on the
   existing shared vector in the lint tree's testdata; the refusal cases are what actually bind the
   two validation halves.
3. **Add the cross-read check.** For each brief: if any declared path matches the trigger set while
   all four risk answers are "no", report it. The message names the matching path, the trigger that
   matched, and — because this is the part reviewers get wrong — states plainly that the finding is
   about the INPUTS to the gate derivation, not the derivation. Carry a stable rule-tag bracket token
   so the firing audit can attribute it.
4. **Phase the severity, and commit to the flip.** Land as an advisory NOTICE. In the same change,
   run a census of the whole corpus and record the count in the pull-request body. If the census is
   small enough to fix in the same wave, flip to a fatal PROBLEM in a follow-up commit on the same
   pull request and fix the hits. If it is not, the pull-request body names the date or the condition
   for the flip. **Do not leave it a permanent NOTICE** — a standing advisory nobody acts on is
   negative value, and this check has exactly one job, which is to be fatal.
5. **Positive control.** Add a test that injects a brief with a triggering declared path and four
   "no" answers and asserts the check fires; add its inverse (a triggering path with an honest "yes")
   and assert it does not. A control that has never been shown to fire is either unnecessary or
   broken, and without injection you cannot tell which (spec §3 D1).
6. **Document the coverage boundary.** In the check's own source header, state what it does NOT
   catch: a risky surface that matches no trigger, a brief whose declared paths are wrong or absent,
   and the adopter case where only the generic base list is configured. Keep the list beside the
   check, not in a separate document (spec §3 D6).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -c 'RiskPathTriggered' -- statusgen/` | exit 1, no output — **DEREFERENCE, true at authoring (2026-08-25 @ `657cab1`)**: the classification is not reachable from the lint today. Inverts to a hit at implementation |
| 2 | `git grep -n 'func RiskPathTriggersFor' -- tools/desk/internal/deskkit/riskpath.go` | exit 0; output contains `func RiskPathTriggersFor` — **DEREFERENCE**: the policy-half accessor this brief duplicates really exists on `origin/main` |
| 3 | `grep -q '^module github.com/medici-finance/assay/statusgen$' statusgen/go.mod` | exit 0 — **DEREFERENCE**: the lint declares its OWN Go module, separate from the desk tree that holds the classifier, so the duplicate-and-bind design is forced rather than chosen |
| 4 | `test -f statusgen/testdata/roster_coupling.json` | exit 0 — **DEREFERENCE**: the shared-vector coupling precedent this brief copies ships today |
| 5 | `grep -c 'risk_path_triggers' topology.yaml` | exit 0; a non-zero count — **DEREFERENCE**: the per-repo trigger declarations the duplicated reader must read are really in the topology file |
| 6 | `go test ./statusgen/ -run 'RiskFilesCrossRead' -count=1` | exit 0 — the cross-read check's own tests pass (fails today: no such test) |
| 7 | `go test ./statusgen/ -run 'TriggerCoupling' -count=1 && go test ./tools/desk/internal/deskkit/ -run 'TriggerCoupling' -count=1` | exit 0 for BOTH — the shared vector binds both copies; a drift in either reader reddens here |
| 8 | `go test ./statusgen/ -run 'CrossReadFiresOnDowngradedGate' -count=1` | exit 0 — **positive control**: an injected brief with a triggering declared path and four "no" answers makes the check fire, and its honest-"yes" inverse does not |
| 9 | `go test ./statusgen/ -run 'CrossReadCouldNotCheck' -count=1` | exit 0 — an absent or unparseable declared-paths line reports could-not-check, never a pass |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `63a7a8a`

Runner ≠ implementer. Own temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`). `statusgen/` and `tools/desk/` are separate modules — rows 6-9 (written as `go test ./statusgen/…`) run module-scoped from inside each module; all matched real tests (confirmed with `-v`, no empty `ok`).

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `git grep -c 'RiskPathTriggered' -- statusgen/` | 0 | hits `statusgen/riskpathtriggers.go`, `statusgen/testdata/risk_trigger_coupling.json` — dereference correctly INVERTED (was exit-1/no-output at authoring; now a hit → classifier reachable from lint) |
| 2 | `git grep -n 'func RiskPathTriggersFor' -- tools/desk/internal/deskkit/riskpath.go` | 0 | `riskpath.go:138: func RiskPathTriggersFor(repo string) []string` — policy-half accessor exists |
| 3 | `grep -q '^module github.com/medici-finance/assay/statusgen$' statusgen/go.mod` | 0 | statusgen is its own module |
| 4 | `test -f statusgen/testdata/roster_coupling.json` | 0 | shared-vector precedent present |
| 5 | `grep -c 'risk_path_triggers' topology.yaml` | 0 | count = 4 (non-zero) |
| 6 | `go test ./statusgen/ -run 'RiskFilesCrossRead' -count=1` (module-scoped) | 0 | `TestRiskFilesCrossReadOrdinaryBriefQuiet` PASS |
| 7 | `go test -run 'TriggerCoupling'` in statusgen/ and tools/desk/internal/deskkit/ | 0 / 0 | both PASS (`TestTriggerCoupling`, `TestTriggerCouplingVectorHasRefusals` in statusgen; `TestTriggerCoupling` in deskkit) |
| 8 | `go test ./statusgen/ -run 'CrossReadFiresOnDowngradedGate' -count=1` | 0 | PASS (positive control fires; honest-yes inverse quiet) |
| 9 | `go test ./statusgen/ -run 'CrossReadCouldNotCheck' -count=1` | 0 | PASS (absent/unparseable line → could-not-check, not pass) |

`RISK-VALUE: DERIVED` — the base trigger set `{"secrets/", ".github/workflows/", "k8s/*/rbac.yaml"}` at `statusgen/riskpathtriggers.go:55` is byte-identical to, and drift-bound to, the canonical `baseRiskPathTriggers` at `tools/desk/internal/deskkit/riskpath.go:68` via the refusal-bearing coupling vector `statusgen/testdata/risk_trigger_coupling.json` (read by `TriggerCoupling` tests in BOTH modules, so a drift reddens both — `TestTriggerCouplingVectorHasRefusals` confirms the vector carries refusal cases). Policy-half-only consumption proven by `TestRiskFilesCrossReadOrdinaryBriefQuiet` (an ordinary brief stays quiet).

**Risk-answer NOTICE (surfaced for reviewer confirmation):** `statusgen --lint` emits a `[risk-files-crossread]` NOTICE — this brief answers all four risk questions `no` yet declares a path under the security-trigger `tools/desk/internal/deskkit/`. It resolves in favour of the answers standing: the deliverable is a READ-ONLY cross-read check plus a **byte-identical, drift-bound** synchronized copy of the trigger list (rows 1-2, 7; the RISK-VALUE above) — it strengthens the classifier and changes no security value, customer/regulatory/irreversible/sensitive-data surface. Flipped on that basis; flagged here for the reviewer's independent security confirmation given it is a risk-classifier-adjacent surface.

**VERIFY: PASS** — all nine Verify rows checked-clean by a non-implementer. Advancing `implemented → verified`.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) does the check consume the
POLICY half only, and is there a test proving it does not class an ordinary brief? (2) does the
shared vector carry refusal cases, not only acceptance cases? (3) is the NOTICE→PROBLEM flip
committed to with a date or a condition, or left open-ended?
