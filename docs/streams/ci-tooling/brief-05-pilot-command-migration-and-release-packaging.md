---
brief: assay:assay:ci-tooling:05
title: "Pilot command migration and release packaging"
why: "A library that no shipped binary uses leaves both implementations alive. The pilot proves commands, release composition and external pins work before wider relocation."
wave: 5
depends: ["ci-tooling/03", "ci-tooling/04"]
unblocks: ["ci-tooling/06", "ci-tooling/07"]
effort: "L"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "8e16d89e-fe8c-4176-9c9d-f79e139dcc0f"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/24-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/cmd/claimsguard/: follow-up ci-tooling/05", "ci/cmd/closecheck/: follow-up ci-tooling/05", "ci/cmd/releaseguard/: follow-up ci-tooling/05", "ci/cmd/claimguard/: follow-up ci-tooling/05", "ci/tests/moved_suites.py: follow-up ci-tooling/05", "ci/tests/pilot_parity.py: follow-up ci-tooling/05", "ci/migration/inventory.json: follow-up ci-tooling/05", "Makefile: follow-up ci-tooling/05", "scripts/build-windows.ps1: follow-up ci-tooling/05", "ci/staged-workflows/release.yml: follow-up ci-tooling/05", "ci/staged-workflows/ci.yml: follow-up ci-tooling/05"]
gate-why: "This is the only point where adopter-held check logic may enter this public tree, and it changes workflow execution boundaries. Each imported tool needs its source owner's recorded per-tool human publication ruling; review the cleared set, concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 05 — Pilot command migration and release packaging

## Context

files: `ci/cmd/claimsguard/` (planned), `ci/cmd/closecheck/` (planned), `ci/cmd/releaseguard/` (planned), `ci/cmd/claimguard/` (planned; fallback pilot), `ci/tests/moved_suites.py` (planned), `ci/tests/pilot_parity.py` (planned), `ci/migration/inventory.json` (planned), `Makefile`, `scripts/build-windows.ps1`, `ci/staged-workflows/release.yml`, `ci/staged-workflows/ci.yml` (planned), `changelog/ci-tooling-05.md` (planned)

facts:
- Source inspection: 2026-10-13; public Assay base 4696d598e. Refresh inventory and consumer pins at pickup.
- The stream spec defines the two library layers, private namespace and staged/released/adopted distinction.
- This is an authored migration obligation; the named new tests and paths are deliverables, not existing capabilities.
- `claimsguard`, `closecheck` and `releaseguard` have no source in this tree; the public release workflow records that it does not carry `releaseguard`. Their publication is not established. The pilot set is conditional (spec "First vertical slice"): only tools with a recorded per-tool owner publication ruling, else the fallback pilot, public `tools/claimguard`.

layering: Keep domain checks explicit-input and shared mechanisms narrow; process/provider and CI presentation adapters stay outside pure evaluators. Task and Verify below exercise the complete consumer flow.

design-fit:
  owner: CI tool family and the existing domain owners named in the spec
  contract: none — this draft proposes the packaging/check-result seam; existing S-eligibility and S-evidence-coverage remain unchanged
  retires: [duplicated adapters or obsolete paths named in this brief after consumer proof]
  weight: verbs +0, flags +0, refusals +0 intended externally; rule-text lines added for the bounded migration contract
  why-add: a source move alone cannot preserve pin, publication and workflow authority; this contract assigns the retirement proof to its owner

## Read first

- [Stream spec](spec.md) and [dependency map](README.md).
- Public Assay `docs/contracts.md` and `docs/library-first.md`; do not introduce a rival semantic owner.

## Human decision

The pilot either imports adopter-held check tools into this public repository or, when
none is cleared, moves a public tool instead. Importing a tool publishes its source, so
it needs that tool's source owner to have recorded a human publication ruling for that
tool; a model review, scrub or desk relay does not count. Approve only after the
implementation identifies the cleared set, actual versions, consumer coverage and
negative-control results. Approval does not expand credentials.

Options:
1. **Approve publication and cutover for the cleared tools** — each imported tool cites its owner's recorded per-tool publication ruling; uncleared tools stay at their adopter location; retain current authority and use the tested rollback combination.
2. **Approve the fallback pilot only** — move public `tools/claimguard`; import no adopter-held source.
3. **Hold the cutover** — keep the supported implementation until the missing proof is supplied.

Default if no answer: none — activation blocks until answered.

## Ground rules

- Author staged workflows only; no workflow activation, credential expansion or live infrastructure probes.
- Preserve historical Evidence and independent controls. Stop at implementation; do not assert verified/done.
- Missing publication, release or consumer evidence is could-not-check, never permission to delete source.

## Task

1. Determine the pilot set before any source moves. For each adopter-held tool, check for the source owner's recorded per-tool human publication ruling and a recorded existing-path library adoption run; no source is exported or pushed anywhere public before both exist. An absent or refused ruling drops that tool (it stays at its adopter location and is not re-requested). If no tool is cleared, run the fallback pilot: public `tools/claimguard` adopting `checks/citations`, moved to `ci/cmd/claimguard`, and record in the inventory that claimguard leaves the later cohort relocation. Rewrite this brief's `consumers:` to the selected set, routing the rest as out-of-scope with the reason. Public development never depends on private access.
2. Move the selected thin pilot entry points to ci/cmd and remove duplicated check logic. Preserve claimsguard versus the separate singular claimguard tool. If releaseguard is cleared, it calls the typed claims API when no isolation boundary requires a subprocess.
3. Update Unix/Windows builds, assets/checksums, module dependencies, examples, cross-module readers and staged CI path filters. Ensure both the old path and newly shared core/ci paths trigger relevant checks throughout transition. A moved public tool's old tree stays authoritative and selected by the live workflow legs until a promoted workflow is observed running the relocated suite (spec "Live coverage is never dark"). Keep required-check identities unchanged.
4. Build actual binaries and compare library, binary and action results over the same corpus. Record reviewed deltas for incomplete advisory output. Qualify a cold external consumer using released artifacts before marking released/adopted; retain source-owner copies until their independent consumer proof permits deletion.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `python3 ci/tests/moved_suites.py --brief ci-tooling/05` | exit 0; for every pilot command the inventory marks moved by this brief, the new package exists and `go test -json` (GOWORK=off) reports at least the recorded pre-move test count; an empty set is refused. Fail-first: the same command at brief 04's merged head exits non-zero (pilot packages absent), and that red output is quoted in Evidence | check +mutation |
| 2 | `python3 ci/tests/pilot_parity.py` | exit 0; PILOT PARITY PASS; built binaries and actions cover clean/fail/incomplete states | check +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 is fail-first (it cannot pass on brief 03's module alone) and binds the moved
packages; row 2 is the behavior and negative-control row over built binaries and actions;
row 3 covers consumer routing. The reviewer confirms every imported tool cites its owner's
recorded per-tool publication ruling, and that no other adopter source entered the diff. Publication adequacy and retained authority also require review.
