---
brief: methodology/10
title: Article 2 — "Prevention and reconciliation" (convergence thesis, rescoped 2026-07-09)
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["[I-04](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-convergence-thesis-article-second-publication.md)", "../reconciler/docs/convergence-thesis.md (primary material, ~70% written — needs the same rescope)", "spec §11 publication plan (article 2 outline)", "spec §13 (human-free limit)", "[F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)/[F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) + red-team-2026-07-09.md B1/B2/C2 (2026-07-09 rescope)"]
gate-why: >-
  Publishes Article 2 asserting the identity-reconciler as the flagship exemplar of the
  thesis it's making, permanently and publicly; the brief's own gate is sequenced behind
  needs-fixing items C4/H10-H12 landing so the exemplar doesn't visibly contradict the
  article (inert grants invisible, prod reporting green with the Canton half unwired) —
  sign-off confirms that re-check actually happened against merged main at finalization.
---

# Brief 10 — Article 2: "Prevention and reconciliation" (rescoped 2026-07-09 per [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md))

**CROSS-REPO: primary source doc lives in `../reconciler`; deck in `../decks`.**

## Context
files: docs/articles/prevention-and-reconciliation.md (new; renamed from one-architecture-three-domains per [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md)); source material ../reconciler/docs/convergence-thesis.md (apply the same rescope when expanding — do not import its "three domains" frame); deck source in ../decks
facts:
- Thesis (RESCOPED 2026-07-09 per [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md), red-team B2/C2): declare / converge / measure drift / never trust self-reports — built twice at Medici as true reconcilers (identity reconciler → identity; statusgen/streams → work), and the article says plainly that both are instances of the control-loop pattern GitOps occupied years ago; the positioning is on the *work-state* instance. DAML invariants are NOT a third reconciler — they are transactional *prevention* (no drift, no converge loop, a type system not a control room). Frame them as the prevention end of a **prevention↔reconciliation spectrum**: when can you make bad states unrepresentable, and when must you detect and converge on drift? That spectrum is the article's more interesting spine, not a claimed third domain.
- SCADA is the ANCESTOR, not a convergent twin (red-team B1/B2): "the industrial-control canon is a ready-made maturity model for agent-fleet observability; here are the mechanisms we imported" — with the Have/Partial/Lack gap list, honest about the seam SCADA would call disqualifying (our sensors are writable by our actuators; [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) is the existence proof). DROP "recognized, not designed" and any convergent-evolution claim — independent rediscovery by a model trained on the SCADA/ISA-18.2/Boyd literature is latent retrieval, and the design spec's own cited adoptions contradict it.
- Twin founding incidents: issue #23 inert grants ≡ the honest-status footgun — same epistemology, independent discovery.
- Prior-art map (verified 2026-07-08, in the thesis doc): gated pipelines (Advance, Kiro) on one side; config/knowledge reconcilers (GitOps-for-agents, Context Kubernetes arXiv:2604.11623) on the other; the work-state intersection unoccupied. Claim discipline: "no prior art found at this intersection", never "does not exist".
- Wave 0 / no depends: drafting is unblocked NOW. PUBLICATION SEQUENCED (2026-07-09, [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md)): additionally gated behind the reconciler needs-fixing items C4 and H10–H12 landing (`../oit/docs/needs-fixing.md`) — the flagship exemplar currently violates the thesis (inert-grant ambiguity invisible, prod Keycloak-only reporting green, failed readback = converged), and a hostile reader diffing article against repo sinks it. Re-check those items against merged main at draft-finalization; record the check in Evidence.
- FORBIDDEN NUMBERS (added 2026-07-09, [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)): no ≈30:1 / "27–40 person-days" / "controlled experiment" unless ledger-recomputed; tier observations are a case series.

- NAME (decided 2026-07-09 by human:<name>, brief-13): the methodology is **Assay** — standalone or
  "Assay by Medici"; home domain assay.guide. This article helps mint the name publicly:
  introduce Assay as the name of the practice (an assayer tests the metal, not the stamp —
  evidence-not-claims in one word). Decision + due-diligence record:
  ../reconciler/docs/naming.md.
- AUTHORSHIP (amended 2026-07-08, per human:<name>): the article discloses its AI co-author — drafted with Claude (the fleet's coordinating agent, "Bob"), reviewed and stood behind by human:<name> — following the work-sample precedent: disclosed AI authorship of a document about verifying AI work is not a caveat, it's a demonstration. Include the division-of-labor line (human judgment about where the gates go; agent drafting/verification labor).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only. PUBLISHING is exclusively the human's action.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Expand ../reconciler/docs/convergence-thesis.md into the article (2000–3000 words), applying the [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md) rescope: the two-instance table + the prevention↔reconciliation spectrum with DAML at the prevention end, the epistemological core, where the loop bends for work (non-deterministic actuator → agents behind evidence gates; sensors writable by actuators — [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) disclosed), SCADA-as-imported-maturity-model with the gap list, the prior-art positioning, closing on the work-state intersection.
2. Product tie: one section connecting the reconciler product and the methodology as one competence, two applications — without turning the article into a pitch.
3. Deck source in ../decks per house pattern; cross-cite articles 1 and 3.

## Verify (presence gate — quality is owned by the human review gate)
Honesty note (2026-07-09, [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)): these rows gate *presence*, not quality — quality is owned
by the human gate in Review. Table rewritten 2026-07-09 for the [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md) rescope (new filename,
spectrum/ancestor framing, forbidden-frame and forbidden-number absences).

| # | Command | Expect |
|---|---------|--------|
| 1 | `wc -w docs/articles/prevention-and-reconciliation.md` | ≥2000 |
| 2 | `grep -c "inert grant\|honest-status" docs/articles/prevention-and-reconciliation.md` | ≥2 (twin incidents present) |
| 3 | `grep -ci "no prior art found\|nearest neighbors" docs/articles/prevention-and-reconciliation.md` | ≥1 (claim discipline held) |
| 4 | `ls ../decks \| grep -ci "converg\|prevention"` | ≥1 (deck source exists) |
| 5 | `statusgen --root . --check` | exit 0 |
| 6 | `grep -ci "co-auth\|drafted with Claude\|Bob" docs/articles/prevention-and-reconciliation.md` | ≥1 (AI co-authorship disclosed; row added 2026-07-08) |
| 7 | `grep -ci "prevention" docs/articles/prevention-and-reconciliation.md` | ≥3 (spectrum frame present; row added 2026-07-09 per F-11) |
| 8 | `test -f docs/articles/prevention-and-reconciliation.md && ! grep -qi -e "recognized, not designed" -e "recognized not designed" -e "convergent evolution" -e "three domains" docs/articles/prevention-and-reconciliation.md` | exit 0 (dropped frames absent; row added 2026-07-09 per F-11). Guarded by `test -f docs/articles/prevention-and-reconciliation.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 9 | `test -f docs/articles/prevention-and-reconciliation.md && ! grep -q -e "30:1" -e "27–40" -e "27-40" -e "controlled experiment" docs/articles/prevention-and-reconciliation.md` | exit 0 (F-12: unsourced numbers absent; row added 2026-07-09). Guarded by `test -f docs/articles/prevention-and-reconciliation.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 10 | `grep -c "F-05" docs/articles/prevention-and-reconciliation.md` | ≥1 (writable-sensors seam disclosed; row added 2026-07-09 per B1) |

## Evidence

### Implementer run — 2026-07-09, runner: implementer (opus)

Article: `docs/articles/prevention-and-reconciliation.md` (new). Deck source (uncommitted,
separate `../decks` repo — absolute path, not repo-relative, so not backticked to keep statusgen's
path-existence lint honest): ~/work/decks/prevention-and-reconciliation/pitch-deck.md

| # | Command | Expect | Result | Notes |
|---|---------|--------|--------|-------|
| 1 | `wc -w docs/articles/prevention-and-reconciliation.md` | ≥2000 | **3508** ✓ | Above the 2000–3000 Task target; within the sibling briefs' (09/11) 2000–3500 band. Article carries more mandatory sections than its siblings (two-instance table + spectrum + loop-bend/F-05 + SCADA gap table + four-defect honesty section + prior-art + product-tie). Every element load-bearing; not padded. Flagged for the human review gate. |
| 2 | `grep -c "inert grant\|honest-status" …` | ≥2 | **8** ✓ | Twin founding incidents present (§1, §5, §9). |
| 3 | `grep -ci "no prior art found\|nearest neighbors" …` | ≥1 | **1** ✓ | Claim discipline held (§7). |
| 4 | `ls ../decks \| grep -ci "converg\|prevention"` | ≥1 | **1** ✓ | **SUBSTITUTION:** worktree `../decks` does not resolve (`.claude/worktrees/decks` absent); ran against absolute decks repo `~/work/decks` per task instruction. Dir `prevention-and-reconciliation/` matches. |
| 5 | `go run ./tools/statusgen --root . --check` | exit 0 | **SUBSTITUTED → `--lint` exit 0** ✓ | `--check` cannot pass on a status-changing branch (README row 10 → implemented); ran `go run ./tools/statusgen --root . --lint` instead per task instruction (brief-02/04 precedent). Exit 0. |
| 6 | `grep -ci "co-auth\|drafted with Claude\|Bob" …` | ≥1 | **2** ✓ | AI co-authorship disclosed (Authorship section + abstract). |
| 7 | `grep -ci "prevention" …` | ≥3 | **7** ✓ | Prevention↔reconciliation spectrum frame present. |
| 8 | `grep -ci "recognized, not designed\|recognized not designed\|convergent evolution\|three domains" …` | 0 | **0** ✓ | Dropped frames absent, including in negation — written around per F-11. |
| 9 | `grep -c "30:1\|27–40\|27-40\|controlled experiment" …` | 0 | **0** ✓ | Forbidden numbers absent (F-12). |
| 10 | `grep -c "F-05" …` | ≥1 | **4** ✓ | Writable-sensors seam disclosed (§4, §5, §9). |

### Publication-gate re-check (needs-fixing C4 / H10–H12) — 2026-07-09

Per [F-11](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-article-2-s-three-domains-collapses-to-one-and-its-exemplar-.md), publication is sequenced behind reconciler defects C4, H10, H11, H12 landing.
State of `../oit/docs/needs-fixing.md` at draft-finalization: **all four remain OPEN** — C4 (inert-grant
guard removed from real provisioner; regression test asserts a mock), H10 (prod reconciler
Keycloak-only, reports green while Canton half skipped), H11 (`hint:` field dropped → wrong-party
allocation, converges green), H12 (failed primaryParty readback treated as "nothing to
reconcile"). No fix commits observed against these rows. **Publication remains BLOCKED**; drafting
is complete. The article is written to be honest about exactly these four open defects (§6) rather
than to wait for them — the honesty is the point (the system's own registers caught them). Re-run
this check against merged main before any human publication go.

Independent verification (non-implementer opus re-run on merged main 37c0eab2 incl. #117 + #123, 2026-07-09):

| # | Command | Expect | Result | Notes |
|---|---------|--------|--------|-------|
| 1 | `wc -w docs/articles/prevention-and-reconciliation.md` | ≥2000 | 3509 ✓ | +1 word vs implementer's 3508 (#123 scrub) |
| 2 | `grep -c "inert grant\|honest-status" …` | ≥2 | 8 ✓ | twin incidents present |
| 3 | `grep -ci "no prior art found\|nearest neighbors" …` | ≥1 | 1 ✓ | claim discipline held |
| 4 | `ls ../decks \| grep -ci "converg\|prevention"` | ≥1 | 1 ✓ | same abs-path substitution as implementer run (worktree `../decks` doesn't resolve) |
| 5 | `go run ./tools/statusgen --root . --check` | exit 0 | exit 0 ✓ | ran exactly as written on merged main (no `--lint` fallback) |
| 6 | `grep -ci "co-auth\|drafted with Claude\|Bob" …` | ≥1 | 2 ✓ | co-authorship disclosed |
| 7 | `grep -ci "prevention" …` | ≥3 | 7 ✓ | spectrum frame present |
| 8 | dropped-frames grep | 0 | 0 ✓ | absent incl. in negation |
| 9 | forbidden-numbers grep | 0 | 0 ✓ | F-12 held |
| 10 | `grep -c "F-05" …` | ≥1 | 4 ✓ | writable-sensors seam disclosed |
| +A | `grep -c "Nine of eleven\|fourteen-hour" …` | 0 | 0 ✓ | #123 incident-figure scrub confirmed |
| +B | `grep -niE "nine of\|eleven agents\|hundreds of errors\|fourteen" …` | none | no matches ✓ | no untraced incident numerics remain |

Date: 2026-07-09 · Runner: independent (opus-verifier). Substance notes for the human
gate: rescoped frame holds end-to-end (two instances; DAML excluded from the reconciler
table; SCADA as imported ancestor with gap list); the article is falsification-forward
(abstract + §6 + §9) though [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) itself lands in §4 rather than the literal opening —
judged compliant with intent, human's call; all remaining quantitative claims trace
(issue #23, [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md), C4/H10–H12, 60s cycle) or are non-numeric hedges. Publication gate
re-confirmed still closed: C4/H10–H12 remain open in docs/needs-fixing.md.

## Review
Gate: human (customer: yes; irreversible: yes — publication). Reviewer records
`human:<name>` + date in the stream README table. Publication only on explicit human go.


## Addendum — source material (2026-07-10, desk; Verify table unchanged)

**Fourth-domain reinforcement (fold into the SCADA/convergence arc):** the admin-pod
chokepoint design ([I-26](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-skill-register-instruction-drift.md)) extends the article's field-instrumentation lineage from process
state to AGENT BEHAVIOR: probes as the only sanctioned verbs make off-vocabulary access an
ISA-18.2 alarm on a defined point list — the pod is the RTU, the desk the control room, and
mischief detection becomes alarm rationalization rather than forensics. Pair it with the
three-layer sensor-independence theme (brief-09 addendum: desk-tools audit / assay-tools
external governance / pod evidence receipts as one idea at three layers — the assembled
[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) answer) — Article 2's "prevention and reconciliation" gains a concrete claim: the
reconciler pattern applied to the *methodology itself*, with each layer's declared state
policed by a sensor the actor cannot write. Same honesty rule: [I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md)/[I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)/[I-26](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-skill-register-instruction-drift.md) are scoped
design at time of writing, not shipped history.
