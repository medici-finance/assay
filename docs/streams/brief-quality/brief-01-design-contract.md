---
brief: "assay:assay:brief-quality:01"
title: "Define the brief assessment and outcome contract"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 0
depends: []
unblocks: ["brief-quality/02"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
id: "4a742172-c319-44ef-985d-48fcb1efdd0b"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 01 — Define the brief assessment and outcome contract

## Context

files:
- `docs/streams/brief-quality/spec.md`

facts:
- Read-first: `docs/streams/brief-quality/spec.md`; this is a proposed design, not active fleet policy.
- Current source was inspected at `7aa3835d7` on 2026-09-26; recheck before implementation.
- Existing brief-v2 IDs/version and qualgen telemetry/attribution are the integration seams.
- No live infrastructure, automatic publication, or raw session collection is needed.

layering: Extend existing tools; pure assessment/reduction rules in qualgen/briefquality, file/identity/dispatch effects at adapters. Test both separately and one full offline join.

## Ground rules
- Preserve human gates and capability minima. Stop at implemented; a non-implementer verifies.
- If facts conflict with current source, record NEEDS_CONTEXT rather than guessing.
- Follow the ruled pilot policy; unknown evidence is never a passing result.

## Task
1. Define the six anchored dimensions, independent review contract and separate outcome record.
2. Specify data ownership, reuse boundaries, three-state metrics, migration and open rulings.
3. Author dependent briefs and demonstrate the dependency graph is acyclic.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/spec.md').read_text(); assert all(x in s for x in ['Work size','Specification completeness','Reasoning difficulty','Coupling','Verification strength','Failure consequence'])"` | exit 0; six named dimensions present; adequacy remains review-only |
| 91 | check +dereference | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | exit 0; cited parser and telemetry seams resolve in this checkout; this does not prove document adequacy |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

Author checks on 2026-09-26 (not independent verification):

- `cd statusgen && go run . --root .. --lint`: exit 0, `LINT: PASS`; whole-corpus legacy notices remain, no brief-quality notices after correction.
- Loaded all nine YAML frontmatters and checked every dependency has its inverse unblock and strictly earlier wave: passed.
- Cited parser/telemetry paths and `parseBriefV2Keys` resolve at the recorded baseline.
- Independent editorial review found active-stream/draft-spec mismatch; fixed to parked with a spec pointer. That bounded review is not a full specification approval.

No verified/done claim. Proposed implementation tests are unrun because their deliverables do not exist yet.

### Independent post-merge verification (2026-10-21)

Verification-Attestation: medici-finance/assay#3870 run=4d459140179f6751fd197938bfa47420d7caab7a31f7728171e74569da1ecdd6 source=d6d2d4b94a613077583aaa051ff7bcaa2908ba7f brief=docs/streams/brief-quality/brief-01-design-contract.md model=claude-opus-5-5 tier=strong

Non-implementer verifier, merged main at the source commit above, run from the repository root.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/spec.md').read_text(); assert all(x in s for x in ['Work size','Specification completeness','Reasoning difficulty','Coupling','Verification strength','Failure consequence'])"` | exit 0; six named dimensions present; adequacy remains review-only | exit 0; no assertion error; each of the six dimension names found in the spec (Work size twice, the other five once each) | 2026-10-21 | assay-verifier-app[bot] @ d6d2d4b94a613077583aaa051ff7bcaa2908ba7f (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 91 | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | exit 0; cited parser and telemetry seams resolve in this checkout; this does not prove document adequacy | exit 0; no assertion error; parseBriefV2Keys is defined at statusgen/briefv2.go line 203; the telemetry source file exists | 2026-10-21 | assay-verifier-app[bot] @ d6d2d4b94a613077583aaa051ff7bcaa2908ba7f (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Rows: 2 of 2 required rows passed; 0 could-not-check.

Supplemental probe (no acceptance credit): parsed the frontmatter of all nine stream briefs; 10 depends edges, every dependency sits in a strictly earlier wave, so the dependency graph is acyclic.

Risk-bearing value. Enumeration covered the delivering change (twelve Markdown files, no code, config or data files) plus this brief's Deliverables. The spec introduces no numeric constant, bound, threshold or timeout; it says outright that no minimum sample size or improvement threshold is set here (spec line 118). The only literals are this brief's own authority bindings: gate = "model" (brief line 9) and exec-tier = "strong" (brief line 17), with wave = 0 (brief line 5) ranked last as an ordering knob. All are reversible by an edit, because the change is document-only and the spec states that nothing in fleet routing or review changes when it merges.

- RISK-VALUE: DERIVED — gate = "model" @ docs/streams/brief-quality/brief-01-design-contract.md:9 — the four risk answers (brief line 10) are all "no", and spec line 44 makes those answers the authority inputs for gate. A proposed, non-enacted design document with no irreversible act therefore does not require the human gate.
- RISK-VALUE: DERIVED — exec-tier = "strong" @ docs/streams/brief-quality/brief-01-design-contract.md:17 — this is a cross-contract design whose attribution errors survive happy-path tests (exec-tier-why), and spec line 44 keeps strong as a hard lower bound. Strong is the conservative floor, never a relaxation.

Scope note: rows 1 and 91 establish presence only. Semantic adequacy of the contract remains with review, as the Verify preamble states.

**VERIFY: PASS** — 2/2 rows; attested source d6d2d4b94a613077583aaa051ff7bcaa2908ba7f.

### 2026-10-22 — independent post-merge verification (witnessed)

Non-implementer verifier pass at merged main `05dc9fbc88f2`, admitted by the dispatcher-owned pre-work check (binding below). Both Verify rows were executed by `statusgen verifyrun` in write mode on a clean detached tree at that source; the witness table and attestation line below are exactly as the tool wrote them.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/spec.md').read_text(); assert all(x in s for x in ['Work size','Specification completeness','Reasoning difficulty','Coupling','Verification strength','Failure consequence'])"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 91 | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4029 run=a13332d4e1fcc50fce1f3671bbc74b52ac5de2e7dc63d98276dd029cb2529cc0 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/brief-quality/brief-01-design-contract.md model=claude-opus-5-5 tier=strong

Supplemental probes (diagnostic only; they discharge no Verify row):

- Row 1 discriminates: the same assertion against a scratch copy of the spec with one dimension name altered exits 1 (AssertionError).
- Task 3 graph: frontmatters of all nine briefs in this stream parsed; 10 dependency edges, each with its inverse `unblocks` entry and a strictly earlier wave, so the graph is acyclic.
- Per the brief's own Verify note, these rows establish presence only; semantic adequacy of the design contract stays with review.

Risk-bearing values (risk metadata is present and all four answers are "no"; enumeration covered the delivering change's spec and brief text in this stream): spec Version = 0.1-design @ docs/streams/brief-quality/spec.md:3; schema = brief-assessment-v1 @ docs/streams/brief-quality/spec.md:49; proposed qualgen briefs exit codes 0 complete / 2 invalid input / 3 report with gaps / 1 IO failure @ docs/streams/brief-quality/spec.md:112. All are reversible document edits in a proposed design; none is an irreversible authority binding.

RISK-VALUE: DERIVED — qualgen briefs exit codes = 0/2/3/1 @ docs/streams/brief-quality/spec.md:112 — a report with gaps (3) is kept distinct from a complete report (0), as the stream's ground rule that unknown evidence is never a passing result requires.

**VERIFY: PASS** — 2/2 Verify rows passed under the execution witness; no could-not-check rows.
### 2026-10-27 — independent post-merge verification (witnessed)

Non-implementer verifier pass at merged main 52719b9592b4 (full source in the attestation line), admitted by the dispatcher-owned pre-work check. The delivering change is #1719 (merge 212ea3edeaff, first parent 392a7bf84f93). Both Verify rows were executed by statusgen verifyrun (desk-tools v1.0.63) on the clean detached admitted tree; the witness table and attestation line below are exactly as the tool printed them. Runner class: dispatched model verifier on a darwin-arm64 host, offline; both rows are class check (no check:ci row, so no network sandbox applies).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/spec.md').read_text(); assert all(x in s for x in ['Work size','Specification completeness','Reasoning difficulty','Coupling','Verification strength','Failure consequence'])"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
| 91 | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4565 run=895af2c688e87e19ca7633871f11090f36d61397bba01b5b91d30463347d8b93 source=52719b9592b4443a784d246813117bf4023fb410 brief=docs/streams/brief-quality/brief-01-design-contract.md model=claude-opus-5-5 tier=strong

Rows: 2 of 2 required rows passed; 0 failed. Neither row is risk-bearing (class check, risk answers all "no"), so no fail-first witness is required.

Supplemental probes (diagnostic only; they discharge no Verify row):

- Row 1 discriminates: the same assertion against a scratch copy of the spec with "Coupling" altered exits 1 with AssertionError. At the delivering change's first parent the spec does not exist, so row 1 cannot pass there.
- Row 91 seams: parseBriefV2Keys is defined at statusgen/briefv2.go line 203; qualgen/telemetry/source.go exists.
- Task 3 graph: the frontmatter of all nine briefs in this stream parsed; 10 dependency edges, each with its inverse unblocks entry and a strictly earlier wave, so the dependency graph is acyclic.
- Per the brief's Verify preamble, these rows establish presence only; semantic adequacy of the design contract stays with review.

Risk-bearing values. Risk metadata is present and all four answers are "no" (brief line 10). Enumeration covered the delivering change #1719 (twelve Markdown files, no code, config or data) plus this brief's Deliverables. Literals found: wave = 0 @ docs/streams/brief-quality/brief-01-design-contract.md:5; gate = "model" @ docs/streams/brief-quality/brief-01-design-contract.md:9; exec-tier = "strong" @ docs/streams/brief-quality/brief-01-design-contract.md:17; Version = 0.1-design @ docs/streams/brief-quality/spec.md:3; schema = brief-assessment-v1 @ docs/streams/brief-quality/spec.md:49; proposed qualgen briefs exit codes 0 complete / 2 invalid input / 3 report with gaps / 1 IO failure @ docs/streams/brief-quality/spec.md:112. Ranked: the two authority bindings (gate, exec-tier) first; the exit-code contract next; version, schema name and wave last as reversible labels and ordering. Every one is reversible by a document edit, because the change is a proposed design that alters no fleet routing or review.

- RISK-VALUE: DERIVED — gate = "model" @ docs/streams/brief-quality/brief-01-design-contract.md:9 — the four risk answers (line 10) are all "no", and spec line 44 makes those answers the authority inputs for gate; a proposed, non-enacted design with no irreversible act does not require the human gate.
- RISK-VALUE: DERIVED — exec-tier = "strong" @ docs/streams/brief-quality/brief-01-design-contract.md:17 — a cross-contract design whose attribution errors can survive happy-path tests (exec-tier-why), and spec line 44 keeps strong as a hard lower bound; strong is the conservative floor, never a relaxation.
- RISK-VALUE: DERIVED — qualgen briefs exit codes = 0/2/3/1 @ docs/streams/brief-quality/spec.md:112 — a report with gaps (3) stays distinct from a complete report (0), as the stream's ground rule that unknown evidence is never a passing result requires.

**VERIFY: PASS** — 2/2 Verify rows passed under the execution witness.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
