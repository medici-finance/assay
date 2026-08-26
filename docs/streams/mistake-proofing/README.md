---
stream: mistake-proofing
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# mistake-proofing Stream — implement the TOOLING half of the poka-yoke spec

The spec [`docs/mistake-proofing.md`](../../mistake-proofing.md) gives the methodology
Shingo's vocabulary — **source / in-process / downstream** × **control / warning** × **who can
bypass it** — and lands seven normative device rules (D1–D7) plus ten brief-authoring rules
(B1–B10). Most of the doctrine describes what this repo already does. This stream implements the
part it does **not** do: the six rules whose devices are, today, prose.

Everything in this stream lands in this repo. The lint (`statusgen/`) and the desk tools
(`tools/desk/`) are canonical here, so each brief's diff, its tests and its status row all live
alongside the code they change.

**The shape of the gap, in one line:** the devices that guard *artifacts* are controls; the devices
that guard *identity* and *adequacy* are warnings — and the authoring rules, which govern every
brief the fleet executes, are the largest body of load-bearing prose left in the system. Spec §1
names why that matters more for a model fleet than for a human one: *the executor will not ask.*
Ambiguity in a specification is resolved silently and divergently, so "report NEEDS_CONTEXT, don't
guess" is fighting a measured tendency and cannot be the primary device. Source-level devices are.

## Which spec rules this stream implements, and which it does not

| Spec rule | What it asks for | Here |
|---|---|---|
| B3 | risk answers cross-read against declared paths | **01** |
| B4 | named identifiers dereference | **02** |
| B2 (+ D7) | Verify rows carry a typed obligation class; presence enforced, adequacy stays review | **03** |
| B9 | authoring guidance's enforcement claims derived, never hand-copied | **04** |
| B1 | scaffold, don't type — the generator front door | **05** |
| D1 | a control must be shown to fire — promoted from prose MUST to lint obligation | **06** |
| B5, B7, B8 | pre-mortem mapping, the dispatch do-confirm checklist, comprehension probes | **not here.** Spec §5 step 3 — pure process, no tooling. They belong in the desk skills, not in the lint |
| B6, B10 | negative control on the Verify table; facts carry their recheck | **not here.** B6 is a judgement about what a wrong-but-plausible deliverable looks like (D7 territory); B10 needs a fact-format decision this stream does not make |
| D2–D6 | fail closed, no silent bypass, warnings don't compose, retire dead devices, non-coverage honesty | **already practised.** The device inventory found these implemented across the tool surface; each new check here inherits them |

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Cross-read a brief's declared paths against the risk classifier (B3)](./brief-01-risk-files-cross-read.md) | 0 | S | todo | — | — |
| 02 | [Dereference named identifiers, not just backticked paths (B4)](./brief-02-identifier-dereference.md) | 0 | S | implemented | — | — |
| 03 | [Typed Verify-row obligation classes, derived from the diff shape (B2, D7)](./brief-03-verify-row-obligation-classes.md) | 1 | M | todo | — | — |
| 04 | [Derive the authoring guidance's enforcement-status claims from the lint (B9)](./brief-04-derived-enforcement-status.md) | 1 | M | todo | — | — |
| 05 | [`newbrief` — the scaffolder as the authoring front door (B1)](./brief-05-newbrief-scaffolder.md) | 2 | M | todo | — | — |
| 06 | [D1 promoted to a lint obligation — a new check must carry its mutation row](./brief-06-mutation-row-obligation.md) | 2 | S | todo | — | — |

## Normative source

Every brief here cites a B-rule or a D-rule as its authority, and a lint that fails a pull request
by citing a rule needs that rule to be normative — the reviewer's correct response to anything less
is "which document says so?". [`docs/mistake-proofing.md`](../../mistake-proofing.md) is on `main`,
so that authority exists and this stream has no external prerequisite. If the spec is materially
reshaped, re-scope this stream rather than implementing against a superseded version.

## Critical path

```
01 (risk × files cross-read) ──▶ 03 (typed obligation classes) ──┬──▶ 05 (newbrief front door)
        │                                                        └──▶ 06 (D1 as an obligation)
        └──────────────────────▶ 04 (derived enforcement status)
02 (identifier dereference) ─────────────────────────────────────────▶ 05
```

**Critical path: `01 → 03 → 05`.** The head is **01**.

- **Why 01 is genuinely the head, verified not assumed.** 01 is the brief that first teaches the
  lint to read a brief's declared `files:` at all — the Context line is parsed by **nothing** today
  (`git grep 'single-point-of-failure' -- statusgen/` returns no hits; the only `files:` parsing in
  `statusgen/brieffile.go` belongs to `parallel-streams:`). 03's `flow`-row obligation is derived
  from those same declared paths, and 05's scaffolder must derive `gate` through the same cross-read
  or it re-opens the exact hole 01 closes. Nothing upstream blocks 01: the classifier it reads is on
  `origin/main` today and the coupling pattern it copies ships with a live test vector.
- **The one thing 01 must resolve before it can be sized as S — and it is already answered.** The
  classifier lives in `tools/desk/internal/deskkit/riskclassifier.go`; the lint is a separate Go
  module (`module github.com/medici-finance/assay/statusgen`) and the classifier's package is
  `internal/`. **A direct import is impossible in both directions.** This repo has already solved
  this exact shape once — `statusgen/rosterconfig.go` duplicates the desk kernel's config reader and
  the two are bound by a shared test vector (`statusgen/testdata/roster_coupling.json`) that fails
  when either side drifts. 01 copies that pattern; it does not invent one, and it does not attempt
  an import.
- **Smallest correct first move:** 01 and 02 in parallel — both are contained lint changes against
  seams that exist on `origin/main` now, and neither depends on the other.
- **Tempting-but-wrong first step: starting with 05.** The scaffolder is the most satisfying item
  (B1: "every field a generator derives is an authoring mistake that stops existing") and it is the
  wrong opening move twice over. A `newbrief` built before 01 still derives `gate` *correctly* from
  *wrong inputs* — which is precisely the defect B3 names, now baked into the front door. And a
  `newbrief` built before 03 emits a Verify table shaped by a rule set that is about to change,
  which means rewriting it. The spec's own §5 ladder puts B1 last (step 5) for exactly this reason;
  follow the ladder.

## Dependency waves

```
Wave 0: [01, 02]
Wave 1: [03] ← 01,  [04] ← 01
Wave 2: [05] ← {01, 02, 03},  [06] ← 03
```

Ladder mapping (spec §5): wave 0 ≈ ladder step 2 (B4) + step 4 (B3); wave 1 ≈ ladder step 4 (B2)
and the head of step 5 (B9); wave 2 ≈ ladder step 5 (B1) plus D1's promotion. Step 1 (vocabulary +
classifying the existing devices) is already done — it is the device inventory this stream derives
from.

**Why 04 sits at wave 1 behind 01 rather than at wave 0.** 04 generates a block of authoring
guidance from the lint's own rule registry and byte-diffs it in CI. 01 *changes the enforcement
status of the single most-cited "self-declared, unchecked" claim in that guidance* (the risk
answers). Two pull requests writing the same generated region concurrently are each green alone and
red merged — the guard-hardening-versus-content interaction is a known and measured failure mode. So
04 lands after 01 and regenerates over it. **03 deliberately does NOT block 04**: once the generator
exists, 03's pull request regenerates the block as a mechanical step, which is exactly B9's promise
— derivation makes ordering cheap instead of load-bearing.

## Shared conventions (inherited by every brief)

- **Plan-only authoring PR.** The briefs in this stream describe work to be IMPLEMENTED later. The
  authoring pull request that files them touches no code, cuts no tag, and changes no lint.
- **One home.** Every deliverable lands in this repo, under `statusgen/`, `tools/` or
  `plugins/assay/skills/`. Consumers of the published binaries pick each change up on the next
  release and version-pin bump.
- **Verify rows run from the repo root.** Every row is written repo-relative and is executed from
  the root of a checkout of this repo. A row that cannot resolve its target is
  **could-not-check**, not a pass — record it that way in Evidence rather than marking the row
  green ([`docs/three-state-instrument-rule.md`](../../three-state-instrument-rule.md)).
- **Verify rows were dereferenced at authoring time.** Every brief carries at least one row that
  greps the CURRENT source for the ABSENCE of the feature it proposes, and each was run against
  `origin/main` @ `657cab1` on 2026-08-25. Those rows are expected to **invert** at implementation;
  a row that still reports absence after the work lands means the work did not land
  ([`docs/brief-rules.md`](../../brief-rules.md), "Verify row semantics: dereferencing vs.
  presence").
- **Three states, always (spec D2).** Every new check added by this stream reports PASS / PROBLEM /
  could-not-check. A check that cannot establish its condition — no git dir, an unreadable tree, an
  unresolvable base — says so and refuses; it never waves through. A control that fails open under
  error is a warning wearing a control's label.
- **Phase NOTICE → PROBLEM, and say when (spec D4).** Every check here lands against an inherited
  corpus of briefs it did not author. Land advisory, census the corpus, then flip fatal on a named
  date or a named condition. Do not add a permanent NOTICE: spec D4 is explicit that warnings do not
  compose and a standing ignored warning is negative value. If a check will never be promoted, it
  should not be added.
- **Presence is the control; adequacy is review (spec D7).** Every check here enforces that an
  artifact IS THERE. None of them judges whether it is any good. Each new check's failure message
  must say which half it covers, or it will be read as enforcement it is not.
- **Rule-tag every emitted message.** New PROBLEM/NOTICE lines carry a stable `[rule-tag]` bracket
  token, the convention `statusgen/lintaudit.go` already extracts. An untagged line falls into that
  file's `unattributed:` bucket, which makes the rule invisible to the 30-day firing audit and
  un-derivable by 04.

## Sources

- [`docs/mistake-proofing.md`](../../mistake-proofing.md) — the normative spec. §4 (B1–B10), §3
  (D1–D7), §1 (doctrine: source over informative over judgment; the executor will not ask), §5 (the
  adoption ladder this stream's waves follow).
- [`docs/brief-rules.md`](../../brief-rules.md) — the brief format the authoring rules govern, and
  in particular its "Verify row semantics: dereferencing vs. presence" and "Three-state instrument
  invariant" sections.
- [`docs/three-state-instrument-rule.md`](../../three-state-instrument-rule.md) — PASS / PROBLEM /
  could-not-check, and the positive-control requirement D1 promotes.
- The device inventory this stream derives from (2026-08-25): the authoring mistakes the brief
  format still permits, the places where authoring guidance contradicts a shipped gate, the four
  tooling moves with their seams and costs, and what must NOT be proofed.
- A measured incident behind **02**: three briefs reached `implemented` in one pass naming three
  tests that existed in no file — every presence-check row passed on a factually wrong deliverable,
  because a presence check on a claim is judgment inspection, not source inspection.
- A measured incident behind **04**: a shared guidance block was maintained as a hand-typed second
  copy of a normative source and drifted from it; the closed fix was declared source → generator →
  CI byte-diff, which is the pattern 04 applies to the authoring skill.
- The self-attestation error class behind **01** and **06**: everything a session writes about its
  own work is, at the last mile, prose it authored, and a gate derived from those self-declared
  answers inherits their unchecked-ness.
- Freshness: `origin/main` read 2026-08-25 @ `657cab1`. Every seam named in these briefs, and every
  DEREFERENCE row, was checked against that commit.
