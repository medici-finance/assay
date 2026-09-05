---
brief: quality/16
title: code-slop forensic sweep lane — deterministic suspects → agent verification → evidenced report
why: >-
  History mining (the rest of this stream) says whether the code is getting better
  over time; nothing in the stream looks at the CURRENT tree and names specific,
  evidenced suspects — the dead helpers, swallowed errors, duplicated structures, and
  overgrown modules that high-velocity authoring accumulates. This brief adds the
  standing sweep lane that does: existing linters nominate, an agent verifies each
  suspect against its surrounding code, and an evidenced report — never an auto-fix —
  is what a human triages from.
wave: 1
depends: ["quality/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-30 by assay-worker session
sources:
  - "Brokk, 'SlopCop: Forensics for your codebase' — https://blog.brokk.ai/slopcop-forensics-for-your-codebase/ — static analysis finds leads across defect categories (dead code, duplication, swallowed errors, complexity hotspots); agents verify each lead against the surrounding code; synthesis emits an evidenced report; humans decide what is worth fixing"
  - "docs/streams/quality/spec.md §3 — architecture (repo-agnostic binary; committed-artifact model; modes read-only against the target repo, writes only under the tracking root)"
  - "docs/streams/quality/spec.md §3.1 — Profile-B: no in-repo writes; artifacts land in the operator-chosen tracking root"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant (measured / measured-zero / could-not-measure)"
  - "docs/streams/quality/spec.md §2 — non-goals: qualgen authors no source-semantics analyzer; the sweep ORCHESTRATES existing linters and never parses source itself"
  - "freshness-checked 2026-08-30 @ 369c827 — quality stream carries briefs 01–15 on origin/main; no existing sweep/slop brief; 16 is free"
exec-tier: strong
exec-tier-why: >-
  (b) correctness is cross-component by construction — ONE suspect schema must flow
  through three legs (linter-output normalization → agent verdicts → report
  synthesis), and drift between any two silently misattributes evidence.
---

# Brief 16 — code-slop forensic sweep lane: deterministic suspects → agent verification → evidenced report

## Context

files:
- `qualgen/main.go` — register the `sweep` mode in `dispatch` + `usage` (sibling of
  `mine` / `report` / `pr` / `check`).
- NEW `qualgen/sweep.go` (planned) (+ `sweep_test.go` (planned)) — the `sweep` mode
  orchestrator: runs the
  three legs in order, computes the standing-lane diff (new / persistent / cleared
  suspects) against prior artifacts, writes only under `--out`.
- NEW `qualgen/suspects.go` (planned) (+ `suspects_test.go` (planned)) — leg 1, the
  deterministic suspects
  front-end: runs the CONFIGURED set of existing external linters against `--repo`
  and normalizes their output into the suspect record schema. No new parser.
- NEW `qualgen/verifier/verifier.go` (planned) — the pluggable `AgentVerifier`
  interface:
  `Verify(suspect, contextPack) → Verdict`. Same convention as the stream's existing
  pluggable adapters (`LinkageAdapter` in `qualgen/fixlinkage.go`, reference adapter
  in `qualgen/adapters/`).
- NEW `qualgen/verifier/fixture.go` (planned) (+ `fixture_test.go` (planned)) — the
  deterministic
  scripted-verdict reference adapter (reads verdicts from testdata), so the lane is
  fully testable offline. A live adapter (a headless coding-agent CLI) is
  CONFIGURATION, shipped separately — not in this brief.
- NEW `qualgen/verdicts.go` (planned) (+ `verdicts_test.go` (planned)) — leg 2, agent
  verification:
  assembles the size-capped context pack per NEW suspect (the suspect's file region,
  the linter's raw evidence, related caller/callee excerpts), invokes the configured
  `AgentVerifier` with the per-category prompt, validates and records the verdict.
- NEW `qualgen/sweepreport.go` (planned) (+ `sweepreport_test.go` (planned)) — leg 3,
  the evidenced
  report emitter: renders the per-run markdown report from suspects + verdicts.
- NEW `qualgen/testdata/sweep/` (planned) — planted fixtures: a minimal target tree
  containing
  one known dead function, one known swallowed error, one oversized module, one
  duplicated block; canned linter outputs per category; scripted verdicts.
- Artifacts (under the `--out` tracking root, spec §3.1/§9.4 discipline — NEVER in
  the target repo): `docs/quality/sweep/suspects.jsonl` (append-only),
  `docs/quality/sweep/verdicts.jsonl` (append-only),
  `docs/quality/sweep/report-<run-date>.md` (one per run).

facts:
- **Pattern (public prior art, Brokk's SlopCop writeup):** static analysis finds
  leads; agents verify the surrounding code; the report carries evidence for every
  claim and a human decides what is worth fixing. The two-stage split exists because
  each stage covers the other's failure mode: linters are deterministic but
  context-blind (false positives and negatives); agents have context but overclaim
  (a persuasive paragraph is not evidence). This brief ports that pattern into
  qualgen as a STANDING lane (run on a CI/cron cadence against a target repo), not a
  one-shot.
- **Reuse, don't rebuild (quality/01, verified):** the three-state `Measure[T]` type
  is `qualgen/measure.go` (`Measured` / `MeasuredZero` / `CouldNotMeasure`); the
  append-only artifact store is `qualgen/store.go` (`Store.Append(kind, record)` +
  typed stream readers over the tracking root); mode dispatch is `qualgen/main.go`.
  Extend these; do not re-invent artifact plumbing.
- **Suspect record:** `{fingerprint, category, file, line-window, tool, rule,
  raw-evidence}`. Fingerprint = category + normalized path + symbol/line-window hash.
  **Verdict record:** `{fingerprint, class, evidence-pointer, rationale}`, class ∈
  `confirmed | false-positive | needs-human | could-not-verify`.
- **Standing lane ⇒ incremental:** a run reads prior `suspects.jsonl` /
  `verdicts.jsonl`, fingerprints the current suspects, and sends only NEW
  fingerprints to leg 2 (a `--reverify-all` flag overrides). Prior verdicts carry
  forward; the report sections suspects as new / persistent / cleared. This is the
  cost control that makes a cadence viable — without it every run re-spends the full
  agent pass.
- **Evidence is enforced, not requested:** a verdict claiming `confirmed` or
  `needs-human` with an EMPTY evidence-pointer is malformed; leg 2's validation
  records it `could-not-verify`, and leg 3 MUST NOT render it as actionable. A
  persuasive paragraph with no pointer to file:line plus quoted code is exactly the
  failure mode this lane exists to refuse.
- **Three-state per category (spec §3.2):** a configured linter that is not
  installed, or a category with no configured tool, is `could-not-measure` for that
  category — never a silent zero. A sweep report can never read "clean" when it
  merely "didn't look."
- **Leg-1 tool set is CONFIG (per-target, per-language), never hardcoded:** e.g. for
  a Go target, a `staticcheck`-class dead-code check, an unchecked/swallowed-error
  linter, a module-size threshold, a `dupl`-class clone detector. qualgen shells out
  and parses tool OUTPUT only — the spec §2 non-goal holds: qualgen authors no
  source-semantics parser.
- single-point-of-failure: the agent-verification leg is the ONE control between a
  linter false-positive and the human triager's time — behind it sit two independent
  layers: (1) emitter-side evidence enforcement (a DIFFERENT component reclassifies
  an evidence-free verdict, so a buggy or overconfident verifier cannot
  self-confirm), and (2) the report carries the evidence verbatim so human triage
  re-adjudicates cheaply — and the lane NEVER auto-acts, so that human layer is
  always present.
- **Scope boundary vs spec §2:** the §2 non-goal bars qualgen from BECOMING a
  source-semantics analyzer. This lane orchestrates external analyzers and adds what
  qualgen exists for — evidence discipline, three-state honesty, committed
  artifacts. It is a current-TREE lane alongside the history-mining layers M1–M4: a
  new `sweep` mode, not a new M layer.

## Ground rules
- NEVER git push to main / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- The lane is REPORT-ONLY: it never edits the target repo, never auto-fixes, never
  files issues, never dispatches work. Triage is a human/downstream step.
- No new parser: leg 1 orchestrates EXISTING external linters via config. If a
  needed tool does not exist for a target language, that category is
  `could-not-measure`, not a qualgen feature.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't
  guess.

## Task
1. Register `sweep` in `qualgen/main.go`: flags `--repo <dir>` (target), `--out
   <dir>` (tracking root), `--config <file>` (tool set + category thresholds +
   verifier adapter selection), `--reverify-all`.
2. Leg 1 (`suspects.go`): run each configured linter against `--repo` (shell out;
   missing tool ⇒ that category `could-not-measure`); normalize outputs into suspect
   records; compute fingerprints; append to `suspects.jsonl` via the quality/01
   store.
3. Orchestrator (`sweep.go`): read prior suspects/verdicts; partition current
   suspects into new / persistent / cleared by fingerprint; only NEW suspects
   proceed to leg 2 (unless `--reverify-all`).
4. Leg 2 (`verdicts.go` + `verifier/`): define `AgentVerifier`; per-category prompt
   templates demanding an evidence pointer (file:line + quoted excerpt); assemble
   the size-capped context pack; validate verdicts (evidence-free
   `confirmed`/`needs-human` ⇒ `could-not-verify`); append to `verdicts.jsonl`.
   Ship the scripted-fixture reference adapter; the live agent adapter is
   configuration, out of scope here.
5. Leg 3 (`sweepreport.go`): render `report-<run-date>.md` — header states target
   SHA, tool set + versions, and per-category measure-state; sections new /
   persistent / cleared; actionable sections carry ONLY evidence-bearing verdicts;
   `false-positive` verdicts render in a suppressed section WITH their reasons;
   `could-not-verify` items are listed as such, never dropped.
6. Fixtures + tests under `qualgen/testdata/sweep/` per the Verify table.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./...` | exit 0 — sweep tests pass; no regression in mine/report/pr/check tests |
| 3 (DEREFERENCE, flow) | `cd qualgen && go test -run TestSweep_FixtureRepo_EndToEnd -v` | exit 0 — full lane over the planted fixture (canned linter outputs + scripted verdicts): the rendered report NAMES the planted dead function's file path and the linter rule that flagged it (proves legs 1→2→3 dereferenced the same evidence, not merely produced a well-formed document) |
| 4 (negative path — evidence enforcement) | `cd qualgen && go test -run TestSweep_EvidenceFreeVerdictNotConfirmed -v` | exit 0 — a scripted verdict claiming `confirmed` with an EMPTY evidence pointer is reclassified `could-not-verify` and the report does NOT list the suspect as actionable |
| 5 (standing-lane incrementality) | `cd qualgen && go test -run TestSweep_RerunSkipsAdjudicated -v` | exit 0 — a second run over an unchanged tree sends ZERO previously-fingerprinted suspects to the verifier (fixture adapter's recorded call count == 0) and sections them `persistent`; a newly planted suspect is verified exactly once and sectioned `new` |
| 6 (read-only posture) | `cd qualgen && go test -run TestSweep_TargetTreeUnmodified -v` | exit 0 — the fixture target's tree hash is byte-identical before and after the sweep; all writes land under the temp `--out` dir |
| 7 (three-state, config-driven) | `cd qualgen && go test -run TestSweep_NoToolsConfigured_CouldNotMeasure -v` | exit 0 — empty tool config yields a report whose every category is `could-not-measure`, NOT measured-zero (a hardcoded default tool set would fail this row) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — VERIFY: PASS — 2026-09-04 opus-4.8[1m]-verifier (verify-desk dispatch), merged main 4e500df

Runner != implementer. Offline (KUBECONFIG=/dev/null). gate: model, risk {all no}, irreversible: no.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | cd qualgen && go build ./... && go vet ./... | 0 | clean build + vet | 2026-09-04 | opus-4.8[1m]-verifier |
| 2 | cd qualgen && go test ./... | 0 | all 11 packages ok (qualgen, adapters, attribution, consumers, dorajoin, filer, m4, reflex, riskscore, telemetry, verifier) | 2026-09-04 | opus-4.8[1m]-verifier |
| 3 | cd qualgen && go test -run TestSweep_FixtureRepo_EndToEnd -v | 0 | PASS — report names planted dead-fn path + rule | 2026-09-04 | opus-4.8[1m]-verifier |
| 4 | cd qualgen && go test -run TestSweep_EvidenceFreeVerdictNotConfirmed -v | 0 | PASS — evidence-free verdict -> could-not-verify (never silent confirm) | 2026-09-04 | opus-4.8[1m]-verifier |
| 5 | cd qualgen && go test -run TestSweep_RerunSkipsAdjudicated -v | 0 | PASS — rerun sends 0 to verifier | 2026-09-04 | opus-4.8[1m]-verifier |
| 6 | cd qualgen && go test -run TestSweep_TargetTreeUnmodified -v | 0 | PASS — target tree byte-identical | 2026-09-04 | opus-4.8[1m]-verifier |
| 7 | cd qualgen && go test -run TestSweep_NoToolsConfigured_CouldNotMeasure -v | 0 | PASS — empty config -> could-not-measure | 2026-09-04 | opus-4.8[1m]-verifier |

**VERIFY: PASS** — all 7 Verify rows offline-clean; none unrun. Report-only lane (never auto-acts; a human re-adjudicates from verbatim evidence).

**RISK-VALUE: DERIVED** — maxContextBytes = 8*1024 @ qualgen/verdicts.go:26 — caps the file region carried in an agent context pack; 8 KiB is enough surrounding code to adjudicate one lead without ballooning to a runaway module, well within any agent prompt budget; reversible knob, degrades fail-safe to could-not-verify if undersized (never a silent confirm, enforced emitter-side per row 4).
**RISK-VALUE: DERIVED** — contextMarginLines = 8 @ qualgen/verdicts.go:30 — lines of context above/below the flagged region; secondary sizing knob subordinate to maxContextBytes, same fail-safe degradation, no irreversible exposure.

## Review
Gate: model (all four risk answers no — OSS, repo-agnostic, read-only against the
target; report-only with no auto-fix, no filing, no dispatch; the agent leg sits
behind a pluggable interface with an offline reference adapter). Reviewer confirms:
(1) evidence enforcement lives emitter-side as well as verifier-side (row 4 is the
proof); (2) the spec §2 non-goal boundary holds — qualgen parses no source; (3) the
report-only scope held (no fix/file/dispatch path). Record verdict + date in the
stream README table.
