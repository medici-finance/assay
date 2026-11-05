---
brief: "assay:assay:desktools-v2:79"
title: "Migrate the release object client bootstrap to the shared CLI contract"
why: >-
  The workflow executes a checksum-pinned client bootstrap as a real launcher. Keep its fixed positional protocol and security checks visible to an implementation owner instead of hiding the executable behind a library exclusion.
wave: 4
depends: ["desktools-v2/16", "desktools-v2/46"]
unblocks: ["desktools-v2/17"]
effort: "S"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "83058eba-a8f7-4c8f-90e8-0526b709079d"
authored: "2026-10-30 by worker (PR #4939 routing repair)"
sources:
  - "https://github.com/medici-finance/assay/issues/4887"
  - "https://github.com/medici-finance/assay/pull/4939#pullrequestreview-5707077174"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/library-first.md"
exec-tier: "strong"
exec-tier-why: "Cross-component CLI compatibility and release trust boundaries require domain controls to remain independent of parsing."
domain: "complicated"
gate-why: >-
  The adapter executes on a release runner and is adjacent to credential or executable
  integrity controls. Human acceptance must confirm that the exact compatibility plan
  preserves every existing custody, integrity and execution restriction.
decision-trigger: "spec"
consumers:
  - "tools/release/install-object-client.sh: follow-up desktools-v2/79 (thin compatibility adapter; preserve current protocol)"
  - "tools/desk/cmd/deskrelease: follow-up desktools-v2/79 (shared command adapter, typed input and owner tests)"
  - ".github/workflows/release.yml and ci/staged-workflows/release.yml: follow-up desktools-v2/79 (fixed workflow invocation)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/79 (pending row; migrated only on evidence)"
---

# Brief 79 — Migrate the release object client bootstrap to the shared CLI contract

## Context

- `tools/release/install-object-client.sh` (future: re-landed with the release object handoff, which #5089 reverted) was a maintained workflow entrypoint at `caf3ef11d9a387218d0128ba90210ea55b30f9b3`: bash; one required job-local destination positional argument, no flags.
- The existing checker refuses a shipped launcher hidden by a library exclusion. This
  authoring gives it a real pending owner; no checker or ownership budget is changed.
- `/16` supplies the reference adapter; `/46` owns the existing release command migration,
  which must land before adding internal operations. This brief feeds `/17`'s complete CLI conformance
  gate. The port itself is not a prerequisite for PR #4939's routing repair.
- Planned implementation and test files stay in `tools/desk/cmd/deskrelease`, with the
  domain semantics retained in their existing owner. No new host-installed command is added.

## Architecture pickup

This authoring routes an existing executable; it does not implement a parser port or claim
that the port is complete. The initial reviewed source checkpoint was
`caf3ef11d9a387218d0128ba90210ea55b30f9b3`; it is historical, not the final delivery identity.
The delivering revision is the merged PR #4939 revision, to be recorded by independent
Evidence after merge. At this repair pickup, main
`0006683ef1c8dc7341d0c2e67a4cb6b566e7814e` still allocated through /77; /78 and /79 remain
this PR's new pending owners. The CLI contract, library-first component contract and
selected transition were read from the reviewed checkout. At implementation
pickup, refresh the approved target, interfaces and consumers and record continue/adapt/hold;
a source snapshot is not permission to change custody or publication authority.

The existing readers use legacy `brief-v2` IDs and the routing checker accepts `<stream>/<NN>`
owner IDs. This brief retains that supported pair. No operational milestone declaration is
introduced: this is one bounded maintenance owner, not a new release outcome promise.

## Ground rules

Draft PR only. No release, tag, deployment, workflow dispatch, installation on an operator
host, storage probe or credential/policy change. Do not weaken any existing checksum,
source identity, extraction, credential-source, runner isolation, environment, approval or
publication control. No work on desktools-v2/68 or the separately owned statusgen PR.
Independent verification owns Evidence and lifecycle advancement. Planned tests and files
below are deliverables, not claims that they exist or pass today.

## Human decision

At implementation pickup, record the concrete compatibility plan and approve only a
parser/adapter migration that retains the existing domain controls. No control removal,
credential-source expansion, dependency update or deployment is authorized by this brief.
Default if no answer: hold the affected implementation, leaving its current entrypoint intact.

## Task

1. Preserve the exact one-destination positional protocol through a thin compatibility wrapper, and implement its argument adapter under the existing `deskrelease` command as an internal workflow-only `object-client-install` operation. Use the shared Cobra/Viper foundation, coordinate separate leaf registration with the other release adapter owner, and keep only one parser. Do not add a standalone operator installer or change release component pins.
2. Retain the pinned AWS CLI archive version and digest, HTTPS source, checksum-before-extraction/execution order, job-local destination, version readback, no sudo/global install, and fail-closed behavior. Argument/configuration resolution grants no permission to contact infrastructure or change the pinned client. Any dependency update remains a separate reviewed change with its existing assessment.
3. Root and nested help/version must be offline and effect-free even with missing/malformed config and nonexistent destinations. Preserve byte-exact POSIX/Windows path handling and malformed/extra-argument refusal. Viper binds only the declared destination and defaults; it never discovers configuration or credentials.
4. Add help, configuration-flow, legacy-form and admission-boundary cases in `tools/desk/cmd/deskrelease/cli_test.go` (planned), with synthetic downloader/archive/process effects. Preserve all original installer-order and workflow assertions. Add the canonical mutation spec `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-79.json` (planned): a healthy baseline, distinct caught control and checksum-order, destination-escape and help-side-effect mutations must each be caught with exact restoration.
5. Re-enumerate both release workflow copies and the transport workflow fixtures. Change only their adapter invocation after parity is evidenced; never replace a binary checksum with a source SHA or remove any publication gate. Mark the script row migrated only after the wrapper delegates to the inventoried Go entrypoint and owner tests pass. Independent Evidence, not the routing row, completes this brief.

## Verify (implementation completion; not run by this authoring change)

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIInventory$'` | exit 0; exact discovery, real owner, no exemption, no duplicate route |
| 2 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIHelpOffline$'` | exit 0; root and internal `object-client-install` help/version have zero credential/file/network/process effects |
| 3 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIConfigFlow$'` | exit 0; declared precedence, unset/empty and paths reach the typed handler byte-identically |
| 4 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLILegacyForms$'` | exit 0; current workflow argv/exit forms preserved; malformed input refuses before handler effects |
| 5 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIAdmissionBoundary$'` | exit 0; original domain refusals hold both through the adapter and with parsing bypassed |
| 6 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-79.json` | exit 0; healthy baseline, distinct caught control, all Task mutations caught, zero survivor/unapplied, exact restoration |
| 7 | check | `python3 -B -m unittest discover -s tools/release -p '*_test.py' -v` | exit 0; every original release fixture retained and passing |
| 8 | check +dereference | `cd tools/desk && CLI_OWNER=desktools-v2/79 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$'` | exit 0; actual Go delegation and owner conformance, not a pending row claimed complete |
| 9 | check | `cd tools/desk && go test -count=1 ./cmd/deskrelease && go vet ./cmd/deskrelease` | exit 0; full existing owner package remains green |

## Pre-mortem

A registration can conceal a missing port: rows 1 and 8 separate pending ownership from
completed delegation. A parser can admit an invalid operation: row 5 calls domain admission
directly as well as through the adapter. A green fixture can miss removed controls: row 6
requires a healthy baseline and distinct planted failures. No live release or storage access
is needed or authorized by these checks.

## Evidence
<!-- Independent verifier records actual commands, exits, outputs, date and runner after implementation. -->

## Review
Gate: human. Confirm the compatibility plan preserves release trust boundaries before
implementation; reject any claim that this ownership-only authoring completed the port.
