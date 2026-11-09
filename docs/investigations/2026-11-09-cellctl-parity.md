# Cellctl parity and PostgreSQL acceptance correction

Issues #5823 and #5822 share one follow-up. The source baseline is
`aae4fbf71bca3419f1ac7ece8e83e8e5a651983e`.

## Architecture pickup

Disposition: **adapt** the test oracle and acceptance commands to the shipped
contracts. The Go launcher, credential custody and isolation controls retain their
current behavior. Reviewed at the source baseline: `docs/cellctl.md`,
`docs/cellctl-cadence.md`, `docs/cellctl-cli-compat.md`, the desk-containers/10,
/17 and /26 briefs, `docs/protected-paths.md`, the implementation-pickup procedure,
`docs/go-engineering.md`, and the affected shell/Go verb implementations.
No dependency or runtime implementation is added.

The full matrix reproduced 44 divergences in 200 comparisons. The stale oracle
sites account for all of them:

| Sites | Current contract | Oracle correction |
|---|---|---|
| Host `down` (12) | Dry-run performs no teardown | Print the stop plan and return before cockpit/process effects |
| House `status` (6) | Status reports each role's cadence | Report the absent-checkpoint baseline independently |
| Host Codex `up` (6) | Printed commands retain the explicit cell root | Carry and parse `--cells-root` |
| `check` (15) | Codex effective features and per-role capacity; house cockpit prerequisites; scrubbed display restrictions | Probe effective features, declare fixture capacities, preserve applicable harness checks and refuse scrubbed Orca |
| Scrubbed `desk` (5) | Private tmux owns the process; Herdr displays it; Codex uses native standing instructions | Preserve the private owner and compare a literal expected prompt |

The shell oracle remains independent: it neither invokes the Go launcher nor
loads production Go source. The expected Codex prompt is a checked-in literal
fixture. The matrix still has all 200 cells and its normalization rules are
unchanged. Its Codex stub explicitly reports the effective enabled feature, so
healthy feature checks are exercised. Stored width overrides and existing house
cadence checkpoints are outside this baseline fixture; the oracle explicitly
refuses them instead of reporting a guessed default. Go behavior tests own those
configured cases.

The existing KUBECONFIG mutation still causes a named divergent scrubbed desk
comparison. The PostgreSQL runner's controls reject absent inputs, an unrelated
Compose error and credential use planted in a second file. Row 10 retains the
whole-package run and also requires the delivery's named database test. It names
the physical temporary directory before Go fixtures run, preserving cache
validation's rejection of symlink path components on macOS. Docker container,
volume and network inspection errors remain failures, separate from empty results.

Sibling routing: the older standalone `harness.test.sh` feature-row expectations
and the `harness.test.sh` / `scrubbed-cell.test.sh` Codex config-get stubs need a
separate behavioral-suite fixture refresh. Those suites also carry older live
launch/version assumptions. This change repairs the complete 200-cell parity
fixture and does not claim those broader suites passed.

## Acceptance transition handoff

The proposed v2 changes only Commands in rows `3,5,9,10,12`; original row IDs,
Class, Expect, order, surrounding command prose and all Evidence are retained.
The scratch flow derives a unique `assay-postgres-scratch-` project from its owned
temporary-directory identity, so simultaneous runs cannot own the same project.
It creates it only after proving there are no existing containers, volumes or
networks. It uses a cached pinned image,
`--pull never`, a fresh volume, a free loopback port, synthetic configuration and
runtime-only generated credentials, then tears down its own resources.

This source amendment is **not acceptance approval**. After it lands, the
coordinator identifies the actual amendment on main's first-parent chain and
opens or reuses a same-repository decision issue whose body contains:

```text
<!-- acceptance-transition: docs/streams/desk-containers/brief-26-local-docker-postgres.md -->
```

The mapped human then posts the exact unedited payload below, replacing only the
amendment coordinate with that actual landed immutable Git ID:

```text
Approve acceptance-transition v1
repo=medici-finance/assay
brief=docs/streams/desk-containers/brief-26-local-docker-postgres.md
delivery=bfcfe9a6f0f56673b7590ee2577775a480be0b2d
base=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1
original=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1
amendment=<actual-landed-amendment-Git-ID>
rows=3,5,9,10,12
```

The base above is the unique merge-base of the delivery's two parents. Its
complete original Verify table equals the first-parent table and the delivered
table. The first parent's `045546c4` revision remains the reported fail-first
source; it is not the protected transition's merge-base coordinate.

After the human comment exists, the independent verifier retains its actual URL
in an external two-line acceptance descriptor, runs `statusgen verifyrun` with
`--acceptance-record` and `--acceptance-forge` from admitted landed main, inspects
primary outputs and records fresh independent Evidence. The original sensitive
data design approval, this exact command amendment approval, code review and
verification remain separate obligations. No prior result receives new credit.
