---
stream: delivery-profiles
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/delivery-profiles/spec.md
---

# Delivery profiles with explicit service substitutes

[Draft specification](spec.md). Authoring only: parked pending design approval and
stream admission. No implementation, environment, installed pin or milestone is activated.
Public shared software only; private adopter composition and operational records remain private.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Define the shared delivery-profile contract and pure reader](brief-01-define-the-shared-delivery-profile-contract-and-pure-reader.md) | 0 | M | todo | — | — |
| 02 | [Qualify deterministic runner scenarios against the shared contract](brief-02-qualify-deterministic-runner-scenarios-against-the-shared-contract.md) | 1 | M | todo | — | — |
| 03 | [Build the isolated local profile lifecycle in the existing service host](brief-03-build-the-isolated-local-profile-lifecycle-in-the-existing-service-host.md) | 1 | M | todo | — | — |
| 04 | [Connect the real orchestration supervisor to the selected runner](brief-04-connect-the-real-orchestration-supervisor-to-the-selected-runner.md) | 10 | M | todo | — | — |
| 05 | [Expose the real profile journey and simulation scope in the cockpit](brief-05-expose-the-real-profile-journey-and-simulation-scope-in-the-cockpit.md) | 15 | M | todo | — | — |
| 06 | [Qualify the complete fake-runner journey and export scoped evidence](brief-06-qualify-the-complete-fake-runner-journey-and-export-scoped-evidence.md) | 16 | M | todo | — | — |
| 07 | [Qualify a real-runner replacement without inheriting simulated acceptance](brief-07-qualify-a-real-runner-replacement-without-inheriting-simulated-acceptance.md) | 17 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

Local work: 01 -> 02 -> 04 -> 05 -> 06 -> 07; 01 -> 03 -> 05 and 06.
External integration: graph-execution/26 + cell-api/02 -> 04; cell-api/03 + cell-api/04 -> 05; graph-execution/22 -> 07.

The true initial head is approval of this scoped draft and 01's public importable
contract; the first integrated journey additionally needs the REAL shared supervisor
from graph-execution/26 and actual public service/thin cockpit from cell-api/02–04. Its dependency chain is not replaced by a fake controller.
The fake runner already exists. Building another fake runner or a simulated cockpit
would not unblock the promised real journey. Actual publication of public modules and
consumer-compatible versions is an explicit pickup prerequisite, not proved by local replace.

## Dependency waves

| Wave | Briefs | Result |
|---|---|---|
| 0 | 01 | Shared profile identity and validation |
| 1 | 02, 03 | Scripted runner scenarios; isolated environment lifecycle |
| 10 | 04 | Real supervisor bridge, after graph-execution/26 and cell-api/02 |
| 15 | 05 | Real cockpit actions after cell-api/03–04 |
| 16 | 06 | Qualified fake-runner journey and evidence bundle |
| 17 | 07 | Separately qualified real-runner profile, after graph-execution/22 |

Numeric waves in briefs and this summary are generator-derived from
all execution dependencies, including external ones. Outcomes are proposed in the spec,
not operational milestone YAML. No dependency on milestone-support reader rollout is
needed to author, build or test profiles; automated milestone consumption waits for it.

## Handoff

Before pickup read the spec's source inventory, component matrix, criteria, gates and
consumer routing. Recheck current architecture, claims, unfinished producer work and
module publication; record continue/adapt/hold with revisions. Do not amend an existing
owner's in-flight brief, or re-author its completed evidence. Public implementation PRs
must remain self-contained without private source or paths.
