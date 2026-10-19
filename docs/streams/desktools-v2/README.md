---
stream: desktools-v2
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
spec: docs/streams/desktools-v2/spec.md
issues: []
board: generated
---

# desktools-v2 Stream

**Approved and unparked.** Ruled 2026-09-21 on #1319: the spec is `**Status:** approved`
([spec.md](spec.md)) and this stream is `status: active`.

Rebuild the desk tools' forge access **properly**, now that the flows have solidified. A
`Forge` interface already exists (`tools/desk/internal/deskkit/forge.go`, two complete
backends) and the desk verbs are ~mostly on it; the defect is that things still reach *past*
it — with a GitHub-specific fact (a `gh` subprocess, remote name, query shape) or with an
**ambient credential**. v2 makes reaching past it **structurally impossible**, on three
first-class principles (see [spec.md](spec.md) §2):

1. **CUSTODY is the foundation** — explicit minted-token only, never an ambient CLI credential;
   the re-minted credential is the App PEM + installation id, so **key presence is the custody
   boundary**; one role's key per environment, making even the desktop behave like a locked
   container.
2. **The read path covers statusgen — across the `deskread` verb boundary.** statusgen shells
   `gh` directly (the `scanloop`-in-container break, #628). It reaches the seam by RUNNING the
   `deskread` verb, never by importing `deskkit` (`statusgen/forgeread.go`), and that migration
   is owned by the sibling briefs `forge-neutral/18` and, for the control-feeding reads, `/35`.
   v2 does not redo it: it brings `statusgen/**` under the ban so the progress is measurable,
   then holds the zero. The [library-first amendment](../../library-first.md), ruled on #2395
   (`DR-forge-neutral-36`), lets statusgen link a narrow module's offline and frozen packages;
   every online forge read keeps the verb and `deskkit` stays internal.
3. **Purpose-built queries** — typed access-pattern operations (review-queue snapshot, head-sha
   batch, board sweep), each backend one tuned query: N+1 → one round-trip, rate-limit headroom,
   and one consistent snapshot (freshness), with the GraphQL document never crossing the seam.

A fourth commitment was added on 2026-09-17 ([spec.md](spec.md) §8): **one outbound-write
check at the write seam**. What a deployment may write to a forge is enforced by the tools,
keyed on the target's visibility, at the one place every write already passes — not remembered
per verb, and not carried as prose in a skill.

Boundaries with the sibling streams (`forge-neutral`, `desktools-go-git`, `desk-tools`) and the
open questions for the approver are in [spec.md](spec.md) §4/§7.

The 2026-10-03 request [#2111](https://github.com/medici-finance/assay/issues/2111)
adds **Cobra and Viper across every maintained CLI**, including cellctl and standalone
modules. [Spec §9](spec.md#9-standard-command-parsing-configuration-and-help--2026-10-03)
is the one contract; /15–/17 own its foundation, rollout and completion.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line)](brief-01-audit-and-inventory.md) | 1 | M | done | 2026-09-30 assay-verifier-app[bot] @ 35496323b8fc (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1317 @ a8380e4121a811637f55fd8024eeafd052a531aa) |
| 02 | [the v2 seam contract + the ban-lint (advisory/counting first)](brief-02-seam-contract-and-ban-lint.md) | 2 | M | done | 2026-09-27 assay-verifier-app[bot] @ 9585b4b6cc2e (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1322 @ 3209046e0a06dff80d7060e37227e4d87de9970c) |
| 03 | [native read-path installation-token client — retire gh shell-out in the read path (#1223 pilot)](brief-03-native-read-client.md) | 3 | M | implemented | — | — |
| 04 | [deskclose reads an authorizing comment by its stated kind — retire the kind-less default (#1019)](brief-04-deskclose-authorization-read-kind.md) | 2 | S | done | 2026-09-27 assay-verifier-app[bot] @ 9585b4b6cc2e (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1320 @ f6a6b8fcb28a03884f90590a5ad6557258f3c6f0) |
| 05 | [the push guards judge the remote actually being pushed to — deskpushguard base ref (#1201) and insteadOf in the push-transport gate (#884)](brief-05-push-guards-judge-the-real-remote.md) | 2 | M | verified | 2026-10-02 assay-verifier-app[bot] @ 454982f91a72 (claude-opus-5-5) | — |
| 06 | [installation-token scoping — explicit repo-scoped custody across Go, cellctl and dispatch](brief-06-installation-token-scoping.md) | 4 | M | todo | — | — |
| 08 | [hold statusgen at zero — the gh ban fails on statusgen and the scan is proven with no gh present](brief-08-hold-statusgen-at-zero.md) | 7 | M | todo | — | — |
| 09 | [purpose-built access-pattern query operations (one tuned snapshot, not N per-item calls)](brief-09-access-pattern-queries.md) | 3 | L | verified | 2026-09-30 assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5) | — |
| 10 | [one outbound-write check at the forge write seam, keyed on the target's visibility](brief-10-one-outbound-write-check.md) | 2 | L | implemented | — | — |
| 11 | [a house callout for the outbound-write check — deployment vocabulary stays out of the shipped tools](brief-11-outbound-house-callout.md) | 3 | M | implemented | — | — |
| 12 | [platform compatibility suite — Windows and GitLab semantics as pure-logic tests runnable on Linux/macOS](brief-12-platform-compatibility-suite.md) | 2 | L | implemented | — | — |
| 13 | [platform gates — cross-compile CI leg, forge-ban GitLab symmetry, Verify-row portability lint, brief-06 re-derivation and platform-issue triage](brief-13-platform-gates-and-reconciliation.md) | 2 | M | implemented | — | — |
| 14 | [regression floor — the behavior the desk tools pass today, pinned as tests seeded from resolved issues, which desktools-v2 must keep green](brief-14-regression-floor.md) | 2 | M | verified | 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (claude-opus-5-5) | — |
| 15 | [Cobra and Viper foundation and complete CLI migration routing](brief-15-cobra-and-viper-foundation-and-complete-cli-migration-routing.md) | 1 | L | verified | 2026-10-04 assay-verifier-app[bot] @ 5f5072d89b11 (claude-opus-5-5) | — |
| 16 | [Migrate cellctl to Cobra commands and Viper configuration](brief-16-migrate-cellctl-to-cobra-commands-and-viper-configuration.md) | 2 | L | implemented | — | — |
| 17 | [Enforce complete Cobra and Viper adoption across the tool suite](brief-17-enforce-complete-cobra-and-viper-adoption-across-the-tool-suite.md) | 5 | L | todo | — | — |
| 18 | [Migrate statusgen to Cobra and Viper](brief-18-migrate-statusgen-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 19 | [Migrate qualgen to Cobra and Viper](brief-19-migrate-qualgen-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 20 | [Migrate deskboard to Cobra and Viper](brief-20-migrate-deskboard-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 21 | [Migrate deskdispatch to Cobra and Viper](brief-21-migrate-deskdispatch-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 22 | [Migrate deskwt to Cobra and Viper](brief-22-migrate-deskwt-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 23 | [Migrate deskfleet to Cobra and Viper](brief-23-migrate-deskfleet-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 24 | [Migrate desksupervise to Cobra and Viper](brief-24-migrate-desksupervise-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 25 | [Migrate deskapps to Cobra and Viper](brief-25-migrate-deskapps-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 26 | [Migrate deskclose to Cobra and Viper](brief-26-migrate-deskclose-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 27 | [Migrate deskpost to Cobra and Viper](brief-27-migrate-deskpost-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 28 | [Migrate deskroster to Cobra and Viper](brief-28-migrate-deskroster-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 29 | [Port the assay-inbox launcher onto deskinbox](brief-29-port-the-assay-inbox-launcher-onto-deskinbox.md) | 4 | M | todo | — | — |
| 30 | [Port the GitLab fleet provisioner to a Go command](brief-30-port-the-gitlab-fleet-provisioner-to-a-go-command.md) | 3 | M | todo | — | — |
| 31 | [Migrate the push and write guards to Cobra and Viper](brief-31-migrate-the-push-and-write-guards-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 32 | [Migrate the scan and cluster guards to Cobra and Viper](brief-32-migrate-the-scan-and-cluster-guards-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 33 | [Migrate the token and claim custody verbs to Cobra and Viper](brief-33-migrate-the-token-and-claim-custody-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 34 | [Migrate the preflight and git transport verbs to Cobra and Viper](brief-34-migrate-the-preflight-and-git-transport-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 35 | [Migrate the install path to Cobra and Viper](brief-35-migrate-the-install-path-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 36 | [Migrate the merge authority verbs to Cobra and Viper](brief-36-migrate-the-merge-authority-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 37 | [Migrate deskflip and deskevidence to Cobra and Viper](brief-37-migrate-deskflip-and-deskevidence-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 38 | [Migrate the PR writer verbs to Cobra and Viper](brief-38-migrate-the-pr-writer-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 39 | [Migrate the issue writer verbs to Cobra and Viper](brief-39-migrate-the-issue-writer-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 40 | [Migrate verifyloop and reviewloop to Cobra and Viper](brief-40-migrate-verifyloop-and-reviewloop-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 41 | [Migrate the scan and monitor loops to Cobra and Viper](brief-41-migrate-the-scan-and-monitor-loops-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 42 | [Migrate the comms verbs to Cobra and Viper](brief-42-migrate-the-comms-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 43 | [Migrate fanoutloop, desktick and deskdigest to Cobra and Viper](brief-43-migrate-fanoutloop-desktick-and-deskdigest-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 44 | [Migrate the metrics and calibration verbs to Cobra and Viper](brief-44-migrate-the-metrics-and-calibration-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 45 | [Migrate the board reader verbs to Cobra and Viper](brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 46 | [Migrate the release and provenance verbs to Cobra and Viper](brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 47 | [Migrate the small admin verbs to Cobra and Viper](brief-47-migrate-the-small-admin-verbs-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 48 | [Migrate the small standalone module CLIs to Cobra and Viper](brief-48-migrate-the-small-standalone-module-clis-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 49 | [Migrate the harness and version guard CLIs to Cobra and Viper](brief-49-migrate-the-harness-and-version-guard-clis-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 50 | [Migrate the skill lint CLIs to Cobra and Viper](brief-50-migrate-the-skill-lint-clis-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 51 | [Migrate metrics-harvest and the plugin hook scripts](brief-51-migrate-metrics-harvest-and-the-plugin-hook-scripts.md) | 3 | M | todo | — | — |
| 52 | [Port the changelog and release check scripts](brief-52-port-the-changelog-and-release-check-scripts.md) | 3 | M | todo | — | — |
| 53 | [Port the CI and regression floor scripts](brief-53-port-the-ci-and-regression-floor-scripts.md) | 3 | M | todo | — | — |
| 54 | [Port the security control scripts](brief-54-port-the-security-control-scripts.md) | 4 | M | todo | — | — |
| 55 | [Port the go-analysis adapter entrypoints](brief-55-port-the-go-analysis-adapter-entrypoints.md) | 3 | M | todo | — | — |
| 56 | [Migrate deskperf to Cobra and Viper](brief-56-migrate-deskperf-to-cobra-and-viper.md) | 3 | S | todo | — | — |
| 57 | [Migrate intake-eval to Cobra and Viper](brief-57-migrate-intake-eval-to-cobra-and-viper.md) | 3 | S | todo | — | — |
| 58 | [Extract the role credential provider and retire internal desktoken callouts](brief-58-extract-the-role-credential-provider-and-retire-internal-desktoken-callouts.md) | 1 | L | todo | — | — |
| 59 | [Share concurrent credential lookups and bound reuse by credential expiry](brief-59-share-concurrent-credential-lookups-and-bound-reuse-by-credential-expiry.md) | 2 | M | todo | — | — |
| 60 | [Bound component activation discovery and reuse validated manifest snapshots](brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md) | 1 | M | todo | — | — |
| 61 | [Version shared performance boundaries and keep legacy reports comparable](brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md) | 1 | M | todo | — | — |
| 62 | [Instrument desk hot paths and qualify credential and startup savings](brief-62-instrument-desk-hot-paths-and-qualify-credential-and-startup-savings.md) | 3 | M | todo | — | — |
| 63 | [Extract shared write policy and make writeguard a hook adapter](brief-63-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md) | 1 | L | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`desktools-v2/01` (audit & inventory) -> `desktools-v2/02` (the ban-lint and its recorded
baseline) -> `desktools-v2/03` (the native read client's custody contract — **human gate**) ->
`desktools-v2/06` (installation-token scoping — **human gate**).

That is the longest chain, four deep with two human gates on it, so it paces the stream. A
second chain runs beside it and is the one the 2026-09-17 direction added:
`desktools-v2/01` -> `desktools-v2/10` (one outbound-write check — **human gate**) ->
`desktools-v2/11` (the house callout for it — **human gate**).

**Current heads, rechecked 2026-10-03 at b165003865f6.** The stream is active and its
spec approved; /01 and /02 are done, so the historical parked/draft prerequisite no longer
blocks it. The forge chain's remaining implementation and human gates retain their own
ownership. The new CLI chain can start with /15: the current cellctl parser remains manual
and the desk module has no Cobra/Viper dependency. Its smallest unblocking move is the
shared contract/inventory plus bounded remaining-tool owners, followed by /16's real pilot.

An earlier draft of this README put a different chain here — promote `deskkit` to an importable
library, then port statusgen onto it. That was the tempting-but-wrong first step: it was derived
without reading `statusgen/forgeread.go` or `forge-neutral/18`, which record the opposite
decision and already own that migration ([spec.md](spec.md) §2 Principle 2, §4). Brief 07 is
withdrawn and its number is not reused. The 2026-10-08 library-first amendment is not that
chain: as ruled (`DR-forge-neutral-36`, spec §2 Principle 2) it covers offline and frozen
inputs only, keeps `deskkit` internal and keeps statusgen's online reads on the verb. `desktools-v2/08` is the one brief here that waits on
another stream: it cannot start until `forge-neutral/18` and `/35` together reach zero `gh`
sites in statusgen (26 remained on 2026-09-17), which is why it sits in the last wave and on
no critical path.

## Dependency waves

- **Wave 1** — `desktools-v2/01` (no dependencies; the file:line audit across `statusgen/**`,
  `tools/desk/**`, `tools/cellctl/**`, skills and workflows, plus the outward-writes table).
- **Wave 2** — `desktools-v2/02` (ban-lint + seam contract, advisory first, scope includes
  `statusgen/**`, baseline written to a file), `desktools-v2/04` (deskclose's authorization
  read states its kind, #1019), `desktools-v2/05` (the push guards judge the real remote,
  #1201 / #884), `desktools-v2/10` (one outbound-write check — human-gated),
  `desktools-v2/12` (the platform compatibility suite), `desktools-v2/13` (the platform gates
  and reconciliation) and `desktools-v2/14` (the regression floor). All depend on 01 only;
  parallelizable.
- **Wave 3** — `desktools-v2/03` (native read-client custody contract + the 5 desk `gh`
  exceptions; depends 01+02 — human-gated), `desktools-v2/09` (purpose-built access-pattern
  queries; depends 02) and `desktools-v2/11` (the house callout; depends 10 — human-gated).
- **Wave 4** — `desktools-v2/06` (installation-token scoping; depends 02+03 — human-gated).
- **Wave 7** — `desktools-v2/08` (hold statusgen at zero; depends 02 and the siblings
  `forge-neutral/18` and `/35`, waves 5 and 6 of their own stream — the wave number follows
  the later edge).

Critical path: `01 → 02 → 03 → 06`, with the outbound-write chain `01 → 10 → 11` beside it.

## Relationship to the sibling streams

- **`forge-neutral`** owns the forge *write* path, the resolver/custody (`forge-neutral/01`)
  **and statusgen's forge path** (`forge-neutral/07`, `/08`, `/18` — statusgen off `gh`
  through the `deskread` verb, in progress — and `/35`, the ruling resolvers). v2 consumes the
  resolver, waits on `/18` and `/35`, and
  re-implements neither. v2's own contribution is the *ban* (extended to statusgen), the
  *custody contract* on the native read client, the *access-pattern query layer* and the
  *outbound-write check*. See [spec.md](spec.md) §4.
- **`desktools-go-git`** owns *git*-transport migration; v2 owns the forge-assumption half of
  the push-guard fixes (#1201/#884) and coordinates the transport half.
- **`desk-tools`** is the general planning board for the current suite; v2 is the
  architectural successor for forge abstraction and, since #2111, the shared CLI contract.
  Existing CLI work is reconciled by /15 rather than duplicated.

## Regression floor

The inherited Windows, GitLab and credential behaviors are registered in
[`MANIFEST.md`](../../../tools/desk/internal/regression/MANIFEST.md). Each seed has
an owning package, a passing behavior test and a recorded failure at the fixing
commit's parent. Existing tests are reused where they already pin the fix; the
manifest guard rejects an omitted starter or a test declaration that disappears.
The [harvest receipt](../../../tools/desk/internal/regression/HARVEST.md) records
additional closes and reasoned exclusions.

The floor rides `go test ./...` in PR CI. A desktools-v2 PR that turns a floor test
red may not delete or weaken it: port the test, and name the behavior change that
forced the port in the PR description. Both owning modules must run their tests.
The current desk CI case does; statusgen currently receives build/vet only, so
that half remains an enforcement hold until the maintainer applies the staged
[additive CI patch](../../../ci/staged-patches/desktools-v2-14-statusgen-tests.patch).
A staged patch is not a live gate.

For a bounded local check, run `bash tools/desk/internal/regression/check-floor.sh`
from the repository root. It runs every named seed in its owning package and
requires its top-level PASS line; missing selections and failures are red. The
fixtures use local git repositories, local HTTP servers and fake CLI executables.

## Cobra/Viper migration track

- **Wave 1 — /15:** shared command/configuration adapter and conformance fixtures, complete
  maintained-entrypoint inventory, and bounded migration briefs for all remaining tools.
- **Wave 2 — /16:** cellctl reference migration, preserving launch/custody behavior while
  making root and nested help work offline without configuration or credentials.
- **Wave 3 — /18–/28, /30–/53, /55–/57:** the migration children authored by /15, each depending
  only on /16's reference adapter. One complex entrypoint per child (statusgen, qualgen,
  deskboard, deskdispatch, deskwt, deskfleet, desksupervise, deskapps, deskclose, deskpost,
  deskroster, the GitLab fleet provisioner) or at most five simple ones (guards, custody,
  transport, install, merge authority, writers, loops, comms, metrics, readers, release,
  admin, standalone modules, lints, plugin scripts, changelog and CI scripts, the go-analysis adapter, deskperf, intake-eval). Human-gated
  children: /25–/28, /30–/37, /42, /54 — each carries credentials, refusals or authority.
- **Wave 4 — /29, /54:** /29 ports the assay-inbox launcher onto deskinbox after /45 migrates
  it; /54 adds token renewal to the GitLab command /30 creates.
- **Routing:** [cli-migration.json](cli-migration.json) maps every discovered entrypoint to its
  owning brief or to a classified exclusion; [cli-contract.md](cli-contract.md) is the shared
  command and configuration contract. TestCLIInventory fails on any unrouted, orphaned or
  over-budget row.
- **Final — wave 5, /17:** depends on /16 and every child; zero pending/unowned entrypoints,
  executed binary/configuration tests, generated reference help and a proven PR CI gate. It
  does not perform an omnibus port.

CLI critical path: `15 → 16 → 45 → 29 → 17` (tied with `15 → 16 → 30 → 54 → 17`, which
carries two human gates and so paces in practice); /16 is the next head once /15 lands. Library selection is settled by #2111; implementation risk gates remain.
The old forge path proceeds alongside this track. /06, /08 and /11 retain their original
deliverables under the configuration/CLI contract. /12 carries compatibility guidance only:
all new CLI platform fixtures and checks are owned by /15, /16, their migration children
and /17. /12 does not consume /15 or block that chain; its existing platform/refusal checks
and historical Evidence remain unchanged. The dependency waves and critical path above
therefore need no additional edge for /12.

## Performance delivery priority — 2026-10-16

This authoring amendment prioritizes credential extraction and concurrency, startup discovery
and trustworthy instrumentation. It grants no credential, deployment or collection authority.
Existing delivered performance work and its Evidence are preserved. CLI migration, future
mirror/service work and the local recorder each keep their current semantic owner.

| Priority | Owner | Delivery and current state |
|---|---|---|
| 1 | desktools-v2/58 | Internal credential provider; CLI and shared Go resolver share one implementation. New, todo; human acceptance of implemented custody parity. |
| 2 | desktools-v2/59 | Per-identity in-flight sharing and original-expiry reuse. New, todo; follows 58. |
| 3 | desktools-v2/60 | Source-validated activation discovery reuse. New, todo; independent local fixtures. |
| 3 | desktools-v2/61 | Shared perfrec boundary/schema/readers. New, todo; independent, no perfdata/DB prerequisite. |
| 4 | desktools-v2/62 | Actual desk hotpath attribution and matched before/after workload. New, todo; follows 58, 59, 60 and 61. |
| 4 | statusgen/26 | Shared-recorder phase attribution and invocation-local parse reuse. New, todo; follows 61; remains credential-free. |

Critical path for the credential outcome: `58 → 59 → 62`. Instrumentation starts independently:
`61 → 62` and `61 → statusgen/26`. Discovery 60 starts beside them and feeds 62's matched startup workload.
The smallest unblocking move is 58's internal extraction with synthetic custody/CLI parity;
it does not wait on Cobra, repo-scope policy changes, a service, a database or the mirror.
Review its concrete custody evidence under its own human gate. Do not export deskkit or move
a credential into statusgen to remove a process boundary.

- Wave 1: 58, 60 and 61 — separate owners; coordinate entrypoint integration on current main.
- Wave 2: 59 and statusgen/26 — consume the provider and shared recorder respectively.
- Wave 3: 62 — qualify the combined desk path after all four local inputs; no new remote activation dependency.

Freshness: inspected origin/main eba941721 on 2026-10-16. Open-PR and dispatch-claim reads
were collected at authoring. The reporting plan merged during authoring was reconciled at
9f9bb35de, preserving its new report owners and deriving their later waves; refresh again at pickup.
The tree already carries perfrec plus statusgen/perfrecord.go, board review snapshots,
conditional HTTP/per-PR fact reuse and cheap-clean cache admission. Historical desktop timings
from installed e70f08e4 are bounded motivation, not a representative week or causal savings.
The current 45-minute memo and per-call manifest walk remain inspectable concrete first defects.

Consumer routing: /06 keeps repository/role scoping; /20, /21, /33 and /56 keep CLI migration;
performance-collection/01–02 keep correlated collection/import; forge-neutral/37–45 keep
shared online ingestion, serving and qualification; statusgen/20 keeps complete offline
context discovery. Amend those unfinished owners; do not reopen delivered /09 or desk-tools/24–25.

## Write-policy delivery — 18 October 2026

[Contract](../../write-policy.md): /63 extracts `deskcore/writepolicy` and retains the
writeguard command-hook adapter; /31 then adapts its CLI without duplicating policy.
/63 is independent of allocator completion and starts from current legacy behavior.
Managed consumers separately bind allocation context and prove actual sandbox flow.
The dependency edge 63 → 31 → 17 preserves the existing CLI finish gate; other guard
work within /31 may proceed independently. Recheck current contracts and engineering
guidelines at pickup under brief-v1 §4.1.1; no implementation is claimed here.

Target-consumer clarification, 18 October 2026: graph-execution/22 directly consumes
/63 for the local harness boundary and independent sandbox proof. The pending
medici-finance/assay#3409 proposal may retire /31's parser work after its human
ruling; that never retires the shared policy or the target consumer. /63 does not
depend on any parser port. Until an authorized withdrawal lands, /31's existing
compatibility contract and human gate remain intact.
