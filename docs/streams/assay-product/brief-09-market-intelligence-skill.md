---
brief: assay-product/09
title: 'Market-intelligence skill — product-agnostic competitive/field scan as assay:market-intelligence, plus its first run on Assay itself'
wave: 2
depends: ["assay-dogfood/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable session (assay-toolkit#12 point 3 + human:<name>'s comment)
sources: ["assay-toolkit#12 (human:<name>, verbatim): 'Regularly scan the field/marketplace for others doing similar things and what lessons we can learn from them to make our process better for our needs'", "assay-toolkit#12 comment (the-org, 2026-07-12, verbatim): 'for the regularly scan comment. this is the beginning of a market-intelligence skill, and this skill should be developed as something we could potentially run on ALL products we as building with Assay'", "assay-product/02 (market analysis — the Assay seed corpus; implemented on OPEN PR #259 at authoring; its doc is a one-time analysis, this skill is the recurring scan that keeps it honest)", "assay-dogfood/01 (plugin scaffold — the assay:* skills home this skill ships in) and assay-dogfood/02 (the porting conventions this skill follows)", "methodology/38 (cadence-compression research — authored on OPEN PR #389; owns the recurring-run clock once landed)", "freshness-checked 2026-07-12 @ ab92e96e: no market-intelligence skill exists in ~/.claude, this repo's .claude/skills, or ../assay-toolkit; docs/market-analysis.md in assay-toolkit is a static document, not a runnable scan"]
why: >-
  Every product we build (Assay first; Medici, Plumb, Midnight next) needs the same recurring
  question answered — who else is doing this, what do they do better or differently, and what
  should we adopt — and today that answer exists only as one-off analysis documents that
  freeze the day they are written. One product-agnostic skill makes the scan repeatable,
  comparable across runs, and runnable on every Assay-built product for the cost of a
  product-context block.
---

# Brief 09 — Market-intelligence skill (product-agnostic; first run on Assay)

**CROSS-REPO:** the skill and the first-run report land in `../assay-toolkit` (its own git
repo — commit there, SHA in Evidence); this repo carries only the stream docs. Manifest/k8s:
none — nothing deploys.

## Context

files: `../assay-toolkit/plugins/assay/skills/market-intelligence/SKILL.md` (planned) (the
skill — marketplace-bound, ships as `assay:market-intelligence` in the assay-dogfood/01
plugin scaffold); `../assay-toolkit/docs/intel/assay/2026-07-report.md` (planned) (first
run's output; the `docs/intel/<product>/` convention is part of the deliverable);
`docs/streams/assay-product/README.md` (row).

facts:
- **Product-agnostic is the design constraint, not a nice-to-have** (human:<name>'s comment): the
  skill body carries NO product-specific facts outside a clearly marked example invocation.
  Its input is a **product-context block** the caller supplies: what the product does, target
  market/segment, seed competitor list, path to the prior report or seed analysis (optional),
  and where outputs land (the product's repo). Assay is merely the FIRST product it runs on;
  Medici (this repo's synthetic-asset product), Plumb (`../reconciler`), and the Midnight
  work are the next callers — the skill must be invocable for each with zero edits.
- **Scan structure the skill encodes:** (1) seed expansion — web search from the seed list
  to who's-doing-similar today (the field moves monthly; every claim dated + linked);
  (2) per-player structured read — what they do, what they do better/differently than us,
  evidence links; (3) lessons — what applies to OUR product/process *for our needs* (the
  issue's phrasing; adopting nothing from a strong player is a valid, argued outcome);
  (4) a dated intelligence report to `docs/intel/<product>/<YYYY-MM>-report.md` in the
  product's repo — findings, per-player table, lessons, an explicit "what would make us
  obsolete" line, and a delta-since-last-run section when a prior report exists;
  (5) adopted lessons become intake entries in the product's intake register (disposition
  `new` — the product's normal scoping flow takes over; the skill never edits briefs).
- **Dispatch discipline (encoded in the skill body):** sub-scanners are dispatched with
  neutral task wording — name the product domain and the information sought, never a
  judgment/threat frame for the scanner to confirm; framing the target biases the scan and
  trips dispatch classifiers. Verbatim-quote rule: competitor claims carry source links, our
  own product's claims obey the product's honest-framing rules (for Assay: [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) — no
  measured-ground-truth claims).
- **Reports are INTERNAL:** the report is an internal working document — writing it crosses
  no publishing gate, names no publication step, and anything outward-facing derived from it
  goes through the stream's standing human publication gate separately.
- **Relationship to assay-product/02:** 02's `../assay-toolkit/docs/market-analysis.md` (implemented on open
  PR #259) is the Assay seed corpus and stays the deep one-time analysis; this skill's runs
  are the recurring currency layer. First run seeds from 02's doc if merged, else from the
  [I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md) §11 corpus (`../oit/docs/superpowers/specs/2026-07-08-initiative-streams-design.md` §11) —
  and says which it used.
- **Recurrence:** run clock per product is owned by methodology/38's decision rule once
  landed (data half-life here is weeks-to-months, not days); until then, on-demand + the
  brief-07 freshness manifest may list each product's latest report with a max-age so
  staleness is at least visible.
- consumers (rule 6): assay-dogfood/02 skills bundle (this skill rides the same plugin —
  its parity checklist gains one row); brief-07's freshness manifest (optional max-age row);
  future product runs (Medici/Plumb/Midnight) consume the skill unchanged.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit commits local; push is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- The live `~/.claude` skills tree is NOT touched — this skill is born marketplace-bound in
  the plugin (no #221 out-of-repo declaration needed).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Write the skill at `../assay-toolkit/plugins/assay/skills/market-intelligence/SKILL.md` (planned)
   per facts: frontmatter (name + trigger-focused description per assay-dogfood/02's porting
   conventions), the product-context input contract, the five-step scan structure, dispatch
   neutrality + sourcing rules, internal-report rule, and one marked example invocation.
2. Run it once on Assay itself (product context: Assay the methodology product, seed corpus
   per facts) → `../assay-toolkit/docs/intel/assay/2026-07-report.md` (planned) + intake
   entries in this repo for adopted lessons (or the report's argued "nothing adopted" section).
3. Update the stream-README row; add the parity-checklist row noted in consumers if
   assay-dogfood/02 has landed its checklist (else record the handoff in Evidence).

## Verify (executable — presence gates; scan quality owned by the review gate)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "name: market-intelligence" ../assay-toolkit/plugins/assay/skills/market-intelligence/SKILL.md` | 1 |
| 2 | `grep -ci "product.context" ../assay-toolkit/plugins/assay/skills/market-intelligence/SKILL.md` | ≥ 2 (the product-agnostic input contract is real, not implied) |
| 3 | `test -f ../assay-toolkit/docs/intel/assay/2026-07-report.md && wc -w < ../assay-toolkit/docs/intel/assay/2026-07-report.md` | ≥ 1200 |
| 4 | `grep -ciE "lesson" ../assay-toolkit/docs/intel/assay/2026-07-report.md` | ≥ 3 (lessons section is substantive) |
| 5 | `grep -c "http" ../assay-toolkit/docs/intel/assay/2026-07-report.md` | ≥ 8 (per-player claims carry source links) |
| 6 | `ls docs/streams/intake/ \| grep -c "2026-07" ; grep -ci "nothing adopted\|no lessons adopted" ../assay-toolkit/docs/intel/assay/2026-07-report.md` | ≥ 1 intake entry from the run, OR the argued nothing-adopted section present |
| 7 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verify — VERIFY: FAIL — glm-5.2-verifier, 2026-07-24

Ground truth via `gh api` against `medici-finance/assay-toolkit` `main` HEAD `e97c5d13`
(local checkout `cc00259` is 18 commits behind main; oit main `8f336db3`). **The deliverables
never landed in the sibling repo** — no commit, no branch, no PR. The skill + report exist ONLY
as **untracked files** in the stale local `../assay-toolkit` working tree
(`?? plugins/assay/skills/market-intelligence/`, `?? docs/intel/`), never committed/pushed.
Evidence section was empty (no implementer run recorded); no sibling PR exists (open or closed);
no in-repo oit PR exists for this brief. This is the cross-repo-brief violation (unpushed local
files, no sibling PR — cf. cross-repo-brief-needs-sibling-pr).

| # | Command (ground truth: assay-toolkit `main`) | Exit | Result | Date | Runner |
|---|----------------------------------------------|------|--------|------|--------|
| 1 | `gh api …/contents/plugins/assay/skills/market-intelligence/SKILL.md` | 404 | FAIL — path Not Found on `main` (and not tracked at local `cc00259`); only copy is untracked local | 2026-07-24 | glm-5.2-verifier |
| 2 | product.context grep on the skill | — | FAIL — skill absent from `main`; the 7 local matches are untracked, not merged | 2026-07-24 | glm-5.2-verifier |
| 3 | `test -f …/docs/intel/assay/2026-07-report.md && wc -w` | 404 | FAIL — report Not Found on `main`; local 2768-word copy is untracked, not merged | 2026-07-24 | glm-5.2-verifier |
| 4 | lessons grep on the report | — | FAIL — report absent from `main` | 2026-07-24 | glm-5.2-verifier |
| 5 | http-link count in the report | — | FAIL — report absent from `main` | 2026-07-24 | glm-5.2-verifier |
| 6 | intake entry from THIS run | — | INTENT-FAIL — no intake entry was created from this run (the report never landed); the literal `ls docs/streams/intake \| grep 2026-07` counts unrelated entries, not one from this brief | 2026-07-24 | glm-5.2-verifier |
| 7 | `go run ./tools/statusgen --root . --lint` | 0 | PASS — orthogonal; brief docs untouched on main | 2026-07-24 | glm-5.2-verifier |

**VERIFY: FAIL (rows 1–6).** Task 1+2 require the skill + first-run report to land in
`../assay-toolkit` (committed, SHA in Evidence); nothing landed — no sibling PR, no merged
artifact, Evidence empty. Filed as `medici-finance/assay-toolkit`#142. Brief stays `implemented`.
(The untracked local copies were left untouched — not the verifier's to commit.)

### Non-implementer verify — VERIFY: FAIL (cross-repo still unmerged) — glm-5.2-verifier, merged main `75c8941c`, 2026-07-25

**Filed as #1298.** The 2026-07-24 root cause persists: deliverables are not on `medici-finance/assay-toolkit` `main`. Progress: the rebuild now exists as **assay-toolkit#143** (`fix/issue-142-market-intelligence`) — `assay-reviewer-app[bot]` APPROVED 2026-07-24, lint green — but it is **OPEN, `mergeable_state=dirty`** (conflict, likely docs/streams/INTAKE.md vs recent I-entries #140/#141), **not merged**. `gh api contents` for the skill + report → 404 on main; local checkout still shows the paths untracked.

| # | Command (ground truth: assay-toolkit `main`, HEAD `de6bc67a`) | Exit | Result | Date | Runner |
|---|------|------|--------|------|--------|
| 1 | `gh api …/contents/plugins/assay/skills/market-intelligence/SKILL.md` → grep `name` | 404 | FAIL — Not Found on main (only in open PR #143) | 2026-07-25 | glm-5.2-verifier |
| 2 | product.context count in SKILL.md | 404 | FAIL — file absent from main | 2026-07-25 | glm-5.2-verifier |
| 3 | `gh api …/contents/docs/intel/assay/2026-07-report.md` → wc -w | 404 | FAIL — Not Found on main | 2026-07-25 | glm-5.2-verifier |
| 4 | lesson count in report | 404 | FAIL — report absent from main | 2026-07-25 | glm-5.2-verifier |
| 5 | http link count in report | 404 | FAIL — report absent from main | 2026-07-25 | glm-5.2-verifier |
| 6 | intake entry from this run | 0/404 | FAIL (intent) — no merged intake entry; I-13 lives only in the unmerged PR #143 | 2026-07-25 | glm-5.2-verifier |
| 7 | `go run ./tools/statusgen --root . --lint` (oit) | 0 | PASS — orthogonal | 2026-07-25 | glm-5.2-verifier |

**VERIFY: FAIL (rows 1-6).** Unblock: merge assay-toolkit#143 (resolve the INTAKE.md conflict), then re-verify against assay-toolkit main + record the merged SHA here. Brief stays `implemented`. RISK-VALUE: N/A (pure markdown). **Refs:** #1298, assay-toolkit#142, #143.

## Review
Gate: model — the skill and its reports are internal process artifacts; no publication,
customer, or irreversible surface is touched (outward use of any report content re-enters the
human publication gate separately). Reviewer checks the skill is genuinely product-agnostic
(invocable for Plumb/Medici with zero edits), competitor claims are sourced, the lessons
apply to OUR needs rather than cargo-culting the field, and the dispatch wording stayed
neutral.

### Non-implementer re-verify (#142 closed) — VERIFY: PASS — k3-verifier (verify-desk dispatch), 2026-07-27

assay-toolkit `80d94f8f`; oit `fbca5989`. #142 resolution on assay-toolkit main: PR **#143** (`5993b5b`, App-APPROVED at head 2026-07-25).

| # | Command | Exit | Key output | Result |
|---|---------|------|------------|--------|
| 1 | `grep -c "name: market-intelligence" …/SKILL.md` | 0 | 1 | PASS |
| 2 | `grep -ci "product.context" …/SKILL.md` | 0 | 8 (≥2) | PASS |
| 3 | `test -f …/docs/intel/assay/2026-07-report.md && wc -w` | 0 | 1926 (≥1200) | PASS |
| 4 | `grep -ciE "lesson" …/2026-07-report.md` | 0 | 11 (≥3) | PASS |
| 5 | `grep -c "http" …/2026-07-report.md` | 0 | 17 (≥8) | PASS |
| 6 | intake entries from the run | 0 | 4 per-lesson entries dated 2026-07-16, each `source: assay-product/09 …` | PASS |
| 7 | `statusgen --lint` (oit) | 1 | exactly the 2 baseline #465 PROBLEM lines | PASS |

`gate: model` + Evidence PASS + App approval at head (assay-toolkit#143, 2026-07-25) → flipped `implemented → done` by the verify-desk. (Observation filed separately: report §6 narrates "a single consolidated intake entry (I-13)" but four per-lesson entries landed — content drift, not a row failure.)
