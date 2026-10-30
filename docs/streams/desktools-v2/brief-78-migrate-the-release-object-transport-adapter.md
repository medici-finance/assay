---
brief: "assay:assay:desktools-v2:78"
title: "Migrate the release object transport adapter to the shared CLI contract"
why: >-
  The checked object handoff has a maintained upload/download command surface. Giving it a bounded migration owner keeps it visible to CLI conformance without pretending that a runtime executable is a library or excluding it as a test.
wave: 4
depends: ["desktools-v2/16", "desktools-v2/46"]
unblocks: ["desktools-v2/17"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "b149beef-477a-4bda-9093-74b8b3193449"
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
  - "tools/release/transport.py: follow-up desktools-v2/78 (thin compatibility adapter; preserve current protocol)"
  - "tools/desk/cmd/deskrelease: follow-up desktools-v2/78 (shared command adapter, typed input and owner tests)"
  - ".github/workflows/release.yml and ci/staged-workflows/release.yml: follow-up desktools-v2/78 (fixed workflow invocation)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/78 (pending row; migrated only on evidence)"
---

# Brief 78 — Migrate the release object transport adapter to the shared CLI contract

## Context

- `tools/release/transport.py` is a maintained workflow entrypoint at `caf3ef11d9a387218d0128ba90210ea55b30f9b3`: Python argparse; upload/download, nine required coordinate/path flags and three download proof flags.
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

1. Preserve the current `upload`/`download` argv and exit contract through a thin compatibility wrapper, and implement the typed command adapter under the existing `deskrelease` command as internal workflow-only `object-handoff` operations. Use the shared Cobra/Viper foundation; do not create a second archive, transport, resolver, recorder or publication owner. Keep the existing Python domain implementation until a separately evidenced domain replacement is approved; the script wrapper must delegate to the inventoried Go entrypoint before its row is marked migrated.
2. Separate parsing from the existing prepare/restore/client handlers. Manifest membership and hashes, producing-attempt binding across publisher reruns, pre-extraction verification, fixed runner routing, environment-only credentials, TLS, bounded transfers and no fallback remain independent handler controls. Keep the credential environment scrub and named error behavior; secrets never enter argv or error text.
3. Root and nested help/version execute offline before configuration, credential, file, network or child-process effects. Preserve the currently used flag/value spellings and byte-exact coordinates, native paths, explicit empty/unset behavior and usage versus handler exit codes. Bind only declared sources through command-local Viper; do not add ambient config discovery or credential fallback.
4. Add the CLI contract's help, configuration-flow, legacy-form and admission-boundary tests to `tools/desk/cmd/deskrelease/cli_test.go` (planned), using fake transport/filesystem effects and the retained Python refusal fixtures. Run the original transport, workflow and full release suites. Add a canonical mutation spec at `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-78.json` (planned) with healthy baseline, distinct caught control, every mutation caught and exact restoration. Plant removed expected-hash binding, use of the consumer attempt, escaped credential fallback and parsing-before-offline-help failures.
5. Re-enumerate active/staged release callers and CI, update the thin workflow invocation without changing its authority, and keep the original checked roster, output and exit fixtures. Mark this script row migrated only after actual delegation and all owner tests; then independently complete the owning brief. No measured performance improvement or producer-format migration is part of this parser port.

## Verify (implementation completion; not run by this authoring change)

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIInventory$'` | exit 0; exact discovery, real owner, no exemption, no duplicate route |
| 2 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIHelpOffline$'` | exit 0; root and internal `object-handoff` help/version have zero credential/file/network/process effects |
| 3 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIConfigFlow$'` | exit 0; declared precedence, unset/empty and paths reach the typed handler byte-identically |
| 4 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLILegacyForms$'` | exit 0; current workflow argv/exit forms preserved; malformed input refuses before handler effects |
| 5 | check +flow | `cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIAdmissionBoundary$'` | exit 0; original domain refusals hold both through the adapter and with parsing bypassed |
| 6 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-78.json` | exit 0; healthy baseline, distinct caught control, all Task mutations caught, zero survivor/unapplied, exact restoration |
| 7 | check | `python3 -B -m unittest discover -s tools/release -p '*_test.py' -v` | exit 0; every original release fixture retained and passing |
| 8 | check +dereference | `cd tools/desk && CLI_OWNER=desktools-v2/78 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$'` | exit 0; actual Go delegation and owner conformance, not a pending row claimed complete |
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
