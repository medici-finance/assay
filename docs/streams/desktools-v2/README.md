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
   then holds the zero. The [library-first amendment](../../library-first.md) of 2026-10-08
   (`DR-forge-neutral-36`) let statusgen link a narrow module's offline and frozen packages.
   **Superseded 2026-10-25** by the driver's later ruling on #2395 (`DR-forge-library`): one
   shared forge library for reads and writes, built here (`desktools-v2/75`); `deskread` and the
   write verbs become thin wrappers around it, statusgen links it for its forge calls
   (`desktools-v2/77`), the library owns the per-role-and-repository credential, gh/glab
   launches are held to at most 7 and git uses go-git (`desktools-v2/76`). `deskkit` stays
   internal.
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
| 12 | [platform compatibility suite — Windows and GitLab semantics as pure-logic tests runnable on Linux/macOS](brief-12-platform-compatibility-suite.md) | 2 | L | verified | 2026-10-19 assay-verifier-app[bot] @ e4d474af8bca (claude-opus-5-5) | — |
| 13 | [platform gates — cross-compile CI leg, forge-ban GitLab symmetry, Verify-row portability lint, brief-06 re-derivation and platform-issue triage](brief-13-platform-gates-and-reconciliation.md) | 2 | M | implemented | — | — |
| 14 | [regression floor — the behavior the desk tools pass today, pinned as tests seeded from resolved issues, which desktools-v2 must keep green](brief-14-regression-floor.md) | 2 | M | verified | 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (claude-opus-5-5) | — |
| 15 | [Cobra and Viper foundation and complete CLI migration routing](brief-15-cobra-and-viper-foundation-and-complete-cli-migration-routing.md) | 1 | L | verified | 2026-10-04 assay-verifier-app[bot] @ 5f5072d89b11 (claude-opus-5-5) | — |
| 16 | [Migrate cellctl to Cobra commands and Viper configuration](brief-16-migrate-cellctl-to-cobra-commands-and-viper-configuration.md) | 2 | L | implemented | — | — |
| 17 | [Enforce Cobra and Viper on every new command and every surviving CLI](brief-17-enforce-complete-cobra-and-viper-adoption-across-the-tool-suite.md) | 5 | L | todo | — | — |
| 18 | [Migrate statusgen to Cobra and Viper](brief-18-migrate-statusgen-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 19 | [Migrate qualgen to Cobra and Viper](brief-19-migrate-qualgen-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 20 | [Migrate deskboard to Cobra and Viper](brief-20-migrate-deskboard-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 21 | [Migrate deskdispatch to Cobra and Viper](brief-21-migrate-deskdispatch-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 22 | [Migrate deskwt to Cobra and Viper](brief-22-migrate-deskwt-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
| 23 | [Migrate deskfleet to Cobra and Viper](brief-23-migrate-deskfleet-to-cobra-and-viper.md) | 3 | M | implemented | — | — |
| 24 | [Migrate desksupervise to Cobra and Viper](brief-24-migrate-desksupervise-to-cobra-and-viper.md) | 3 | M | withdrawn | — | — |
| 25 | [Migrate deskapps to Cobra and Viper](brief-25-migrate-deskapps-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 26 | [Migrate deskclose to Cobra and Viper](brief-26-migrate-deskclose-to-cobra-and-viper.md) | 3 | M | withdrawn | — | — |
| 27 | [Migrate deskpost to Cobra and Viper](brief-27-migrate-deskpost-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 28 | [Migrate deskroster to Cobra and Viper](brief-28-migrate-deskroster-to-cobra-and-viper.md) | 3 | M | withdrawn | — | — |
| 29 | [Port the assay-inbox launcher onto deskinbox](brief-29-port-the-assay-inbox-launcher-onto-deskinbox.md) | 4 | M | todo | — | — |
| 30 | [Port the GitLab fleet provisioner to a Go command](brief-30-port-the-gitlab-fleet-provisioner-to-a-go-command.md) | 3 | M | todo | — | — |
| 31 | [Migrate the push and write guards to Cobra and Viper](brief-31-migrate-the-push-and-write-guards-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 32 | [Migrate the scan and cluster guards to Cobra and Viper](brief-32-migrate-the-scan-and-cluster-guards-to-cobra-and-viper.md) | 3 | M | withdrawn | — | — |
| 33 | [Migrate the token and claim custody verbs to Cobra and Viper](brief-33-migrate-the-token-and-claim-custody-verbs-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 34 | [Migrate the preflight and git transport verbs to Cobra and Viper](brief-34-migrate-the-preflight-and-git-transport-verbs-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 35 | [Migrate the install path to Cobra and Viper](brief-35-migrate-the-install-path-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 36 | [Migrate the merge authority verbs to Cobra and Viper](brief-36-migrate-the-merge-authority-verbs-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 37 | [Migrate deskflip and deskevidence to Cobra and Viper](brief-37-migrate-deskflip-and-deskevidence-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 38 | [Migrate the PR writer verbs to Cobra and Viper](brief-38-migrate-the-pr-writer-verbs-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 39 | [Migrate the issue writer verbs to Cobra and Viper](brief-39-migrate-the-issue-writer-verbs-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 40 | [Migrate verifyloop and reviewloop to Cobra and Viper](brief-40-migrate-verifyloop-and-reviewloop-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 41 | [Migrate the scan and monitor loops to Cobra and Viper](brief-41-migrate-the-scan-and-monitor-loops-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 42 | [Migrate the comms verbs to Cobra and Viper](brief-42-migrate-the-comms-verbs-to-cobra-and-viper.md) | 3 | L | todo | — | — |
| 43 | [Migrate fanoutloop, desktick and deskdigest to Cobra and Viper](brief-43-migrate-fanoutloop-desktick-and-deskdigest-to-cobra-and-viper.md) | 3 | L | withdrawn | — | — |
| 44 | [Migrate the metrics and calibration verbs to Cobra and Viper](brief-44-migrate-the-metrics-and-calibration-verbs-to-cobra-and-viper.md) | 3 | L | implemented | — | — |
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
| 56 | [Migrate deskperf and qualify deskusage on the shared CLI](brief-56-migrate-deskperf-to-cobra-and-viper.md) | 3 | M | todo | — | — |
| 57 | [Migrate intake-eval to Cobra and Viper](brief-57-migrate-intake-eval-to-cobra-and-viper.md) | 3 | S | todo | — | — |
| 58 | [Extract the role credential provider and retire internal desktoken callouts](brief-58-extract-the-role-credential-provider-and-retire-internal-desktoken-callouts.md) | 1 | L | todo | — | — |
| 59 | [Share concurrent credential lookups and bound reuse by credential expiry](brief-59-share-concurrent-credential-lookups-and-bound-reuse-by-credential-expiry.md) | 2 | M | todo | — | — |
| 60 | [Bound component activation discovery and reuse validated manifest snapshots](brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md) | 1 | M | implemented | — | — |
| 61 | [Version shared performance boundaries and keep legacy reports comparable](brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md) | 1 | M | verified | 2026-10-19 assay-verifier-app[bot] @ 0e661b6e15de (claude-opus-5-5) | — |
| 62 | [Instrument desk hot paths and qualify credential and startup savings](brief-62-instrument-desk-hot-paths-and-qualify-credential-and-startup-savings.md) | 4 | M | todo | — | — |
| 63 | [Define perf-v3 attribution, profiles and compatible local readers](brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md) | 2 | M | implemented | — | — |
| 64 | [Propagate performance context through desks, agents and tool CLIs](brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md) | 3 | M | implemented | — | — |
| 65 | [Measure own forge request timing and GraphQL cost](brief-65-measure-own-forge-request-timing-and-graphql-cost.md) | 3 | M | verified | 2026-10-27 assay-verifier-app[bot] @ e25479a69d69 (forge-identity) | — |
| 66 | [Extract shared write policy and make writeguard a hook adapter](brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md) | 1 | L | implemented | — | — |
| 67 | [Define an extensible perf-v4 envelope and compatible readers](brief-67-define-an-extensible-perf-v4-envelope-and-compatible-readers.md) | 3 | M | todo | — | — |
| 68 | [Select perf-v4 in shared producers and qualify the direct migration](brief-68-select-perf-v4-in-shared-producers-and-qualify-the-direct-migration.md) | 4 | M | todo | — | — |
| 69 | [Specify extensible JSON contracts and migration targets](brief-69-specify-extensible-json-contracts-and-migration-targets.md) | 1 | M | done | 2026-10-24 assay-verifier-app[bot] @ 385cb29e5682 (forge-identity) | 2026-10-27 assay-reviewer-app[bot] (approved PR #3830 @ 8ee5cc30f25b13843667a5f174951e9d0cdeb4b4) |
| 70 | [Exempt workflow-bound issue filings from the filing budget](brief-70-exempt-workflow-bound-issue-filings-from-the-filing-budget.md) | 1 | M | todo | — | — |
| 71 | [Credit container witness rows bracketed by host verifier admission](brief-71-credit-container-witness-rows-bracketed-by-host-verifier-admission.md) | 1 | M | implemented | — | — |
| 72 | [Land verifier outcome records from a dispatcher-named path outside the home](brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md) | 1 | M | implemented | — | — |
| 73 | [Make deskevidence own the on-behalf-of principal on Runner cells](brief-73-make-deskevidence-own-the-on-behalf-of-principal-on-runner-cells.md) | 1 | M | todo | — | — |
| 74 | [Add a verifyrun row selector that records only the named Verify rows](brief-74-add-a-verifyrun-row-selector-that-records-only-the-named-verify-rows.md) | 1 | M | implemented | — | — |
| 75 | [Build the shared forge library and make the desk verbs thin wrappers](brief-75-build-the-shared-forge-library-and-make-the-desk-verbs-thin-wrappers.md) | 2 | L | todo | — | — |
| 76 | [Move statusgen's git launches to go-git and ratchet git launches down](brief-76-move-statusgens-git-launches-to-go-git-and-ratchet-git-launches-down.md) | 1 | L | todo | — | — |
| 77 | [Retire the deskread bridge for statusgen's forge calls](brief-77-retire-the-deskread-bridge-for-statusgens-forge-calls.md) | 8 | M | todo | — | — |
| 80 | [Port the release spec-header stamp script](brief-80-port-the-release-spec-header-stamp-script.md) | 4 | S | todo | — | — |
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
withdrawn and its number is not reused. The 2026-10-08 library-first amendment was not that
chain (`DR-forge-neutral-36`: offline and frozen inputs only). The 2026-10-25 ruling
(`DR-forge-library`, spec §2 Principle 2) does give statusgen a linked forge library, but not by
exporting `deskkit`: briefs 75 and 76 build one narrow typed library with its own credential,
outside `tools/desk/internal/`, and the git path beside it. `desktools-v2/08` waits on <!-- graph: encoded forge-neutral/18→desktools-v2/08 -->
another stream: it cannot start until `forge-neutral/18` and `/35` together reach zero `gh`
sites in statusgen (26 remained on 2026-09-17), which is why it sits in wave 7 (only /77, which depends on it, is later) and on
no critical path. Brief 77, which moves statusgen's reader onto the library, has the same
cross-stream edges plus /75, /58 and /08, all in its frontmatter; the /08 edge is there so /08's
class a gate lands first and /77 widens it, never the reverse. <!-- graph: encoded desktools-v2/08→desktools-v2/77 -->

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
- **Wave 8** — `desktools-v2/77` (retire the deskread bridge for statusgen; depends 75, 58,
  `forge-neutral/18`, `/35` and 08 — the wave number follows the later edge, /08 at wave 7).

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
- **Wave 4 — /29, /54, /80:** /29 ports the assay-inbox launcher onto deskinbox after /45 migrates
  it; /54 adds token renewal to the GitLab command /30 creates; /80 ports the release
  spec-header stamp script into the command /52 creates, beside the check it reads back through
  (its registry row was routed to /47, which never covered it; #5009).
- **Routing:** [cli-migration.json](cli-migration.json) maps every discovered entrypoint to its
  owning brief or to a classified exclusion; [cli-contract.md](cli-contract.md) is the shared
  command and configuration contract. TestCLIInventory fails on any unrouted, orphaned or
  over-budget row.
- **Final — wave 5, /17:** depends on /16 and every surviving child; zero pending/unowned entrypoints (withdrawn verbs' rows are `exempt` until retired),
  executed binary/configuration tests, generated reference help and a proven PR CI gate. It
  does not perform an omnibus port.

CLI critical path: `15 → 16 → 45 → 29 → 17` (tied with `15 → 16 → 30 → 54 → 17`, which
carries two human gates and so paces in practice); /16 is the next head once /15 lands. Library selection is settled by #2111; implementation risk gates remain.
**Scope amendment — 2026-10-18.** Ratification of the target architecture at revision 23f655f5d: https://github.com/medici-finance/assay/issues/2500#issuecomment-6137853872
The target retires or
re-homes the domains of many desk verbs: desk-tools verbs become thin adapters to typed use
cases and role-bound effects, and the command matrix is not ported as a milestone
([target architecture](../../proposals/non-ci-architecture/11-target-architecture.md),
"What happens to today's tools"); no parser migration is a prerequisite, and obsolete
parser work is dropped once its consumers are identified
([migration](../../proposals/non-ci-architecture/13-migration-and-decisions.md)). The
per-verb ports /24, /26–/28, /31–/34 and /36–/41, /43 are therefore `withdrawn`;
the human-gated withdrawals (/26–/28, /31–/34, /36–/39) are ruled at https://github.com/medici-finance/assay/issues/3420#issuecomment-6257901942;
each names its successor in front matter. /21 (deskdispatch) is not withdrawn: its port
merged before this amendment, so it stays an implemented child of /17. The withdrawn ports'
entrypoints stay on their current parsers,
routed `exempt` in the registry until retired. Withdrawing /31 and /32 withdraws the
parser port only: the guard controls themselves stay and re-home to executor checks.
Cobra/Viper becomes a **requirement for new tools** instead (no ruling of its own: it applies the ratification above and the request in medici-finance/assay#2111)
([cli-contract.md §7](cli-contract.md#7-new-tools-and-retiring-tools--2026-10-18)), and /17
enforces it on every surviving CLI and every new `main` package. /54 (including GitLab token
renewal) is unchanged, pending medici-finance/assay#2987.

The old forge path proceeds alongside this track. /06, /08 and /11 retain their original
deliverables under the configuration/CLI contract. /12 carries compatibility guidance only:
all new CLI platform fixtures and checks are owned by /15, /16, their migration children
and /17. /12 does not consume /15 or block that chain; its existing platform/refusal checks
and historical Evidence remain unchanged. The dependency waves and critical path above
therefore need no additional edge for /12.

## Performance delivery priority — 2026-10-16

**Historical schedule:** the wave bullets and direct instrumentation edges below describe
the 16 October plan. Use the 19 October attributed-performance table for current pickup.

This authoring amendment prioritizes credential extraction and concurrency, startup discovery
and trustworthy instrumentation. It grants no credential, deployment or collection authority.
Existing delivered performance work and its Evidence are preserved. CLI migration, future
mirror/service work and the local recorder each keep their current semantic owner.

| Priority | Owner | Delivery and current state |
|---|---|---|
| 1 | desktools-v2/58 | Internal credential provider; CLI and shared Go resolver share one implementation. New, todo; human acceptance of implemented custody parity. |
| 2 | desktools-v2/59 | Per-identity in-flight sharing and original-expiry reuse. New, todo; follows 58. |
| 3 | desktools-v2/60 | Source-validated activation discovery reuse. New, todo; independent local fixtures. |
| 3 | desktools-v2/61 | Implemented perf-v2 boundary/schema/readers (PR #3431); preserve independent verification/Evidence. |
| 4 | desktools-v2/62 | Actual desk hotpath attribution and matched before/after workload. Todo; now follows 58, 59, 60, 63, 64 and 65. |
| 4 | statusgen/26 | Shared-recorder phase attribution and invocation-local parse reuse. Todo; now follows 63/64; remains credential-free. |

Historical 16 October critical path for the credential outcome: `58 → 59 → 62`. Instrumentation starts independently:
`61 → 62` and `61 → statusgen/26`. Discovery 60 starts beside them and feeds 62's matched startup workload.
The smallest unblocking move is 58's internal extraction with synthetic custody/CLI parity;
it does not wait on Cobra, repo-scope policy changes, a service, a database or the mirror.
Review its concrete custody evidence under its own human gate. Do not export deskkit or move
a credential into statusgen to remove a process boundary.

- Historical wave 1: 58, 60 and 61 — separate owners; coordinate entrypoint integration on current main.
- Historical wave 2: 59 and statusgen/26 — consume the provider and shared recorder respectively.
- Historical wave 3: 62 — qualify the combined desk path after all four local inputs; no new remote activation dependency.

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

## Attributed performance delivery — 19 October 2026

[The v3 contract](../../performance-attribution-spec.md) supersedes v2-only future intent;
completed /61 remains implemented, with existing Evidence untouched. No collection is activated.

| Owner | Wave | Deliverable |
|---|---|---|
| /63 | 2 | New perf-v3 contract/context/profile/readers after implemented /61; no DB or perfdata dependency. |
| /64 | 3 | Launchers, agent lifecycle and shared real CLI/env attribution after /63. |
| /65 | 3 | Own-request timing and attributable GraphQL cost after /63; unknown is not zero. |
| statusgen/26 | 4 | Credential-free v3 producer after /63 and /64. |
| /62 | 4 | Matched desk workload after /58, /59, /60, /63, /64 and /65. |

Attribution critical path: `61 (implemented) → 63 → 64`; observation proceeds `63 → 65`.
Combined desk qualification has both that path and `58 → 59 → 62`; /60 remains independent.
The next unblocking move for attribution is /63's schema/profile/resolver, verified absent
at 2e02111ed. /64 and /65 consume its fixtures independently. Final CLI qualification /17
includes /64; existing CLI migrations adopt shared bindings without duplicating them.
Collection/02 follows /01 and /63–65 in wave 4; it never blocks local v3.
The earlier 16 October schedule is historical; these amended dependencies control pickup.

## Write-policy delivery — 18 October 2026

[Contract](../../write-policy.md): /66 extracts `deskcore/writepolicy` and retains the
writeguard command-hook adapter.
/66 is independent of allocator completion and starts from current legacy behavior.
Managed consumers separately bind allocation context and prove actual sandbox flow.
Recheck current contracts and engineering guidelines at pickup under brief-v1 §4.1.1; no implementation is claimed here.

Target-consumer clarification, 19 October 2026: graph-execution/22 directly consumes
/66 for the local harness boundary and independent sandbox proof. desktools-v2/31's
parser port is withdrawn (medici-finance/assay#3409 merged, ruled at
medici-finance/assay#3420), so /66 has no parser-port edge: writeguard keeps its current
parser, routed exempt, and /66 owns the hook adapter. The guard controls are not retired.

## Extensible performance successor — 21 October 2026

[The contract census](../../proposals/non-ci-architecture/29-extensible-json-contracts.md)
updates the future architecture across JSON owners. Critical path for selected-profile
performance is /69 (design/specification) → /67 (reader/core/extensions) → /68 (producer
selection). /67 also needs /63; /68 also needs /64 and /65 verification. Authoring
these briefs does not claim their implementation. Existing /26 and /62 work retains
its snapshot, phase, benchmark and verification obligations; future format tasks
migrate to v4 through dated amendments rather than editing completed evidence.

## Workflow filings and witness records — 22 October 2026

Four independent wave-1 briefs carry driver rulings of 22 October 2026 for ratification at merge: the public record of those rulings is the driver's merge of the PR that adds these briefs, #4069, merged 2026-10-22.
/70 binds
workflow issue filings to their PR (and brief), outside the process-error filing
budget, with a per-role per-PR cap, its own audit class and the unchanged repo-wide
ceiling as the compensating bound (proposal 11 brakes). /71 credits Linux container
witness rows only when host verifier admission brackets the run and a tree digest
binds the bytes the container ran to the admitted home, and makes
`verifyrun --sandbox=container-netns` fail closed without that bracket. /72 has the
dispatcher name an outcome-record path outside the verifier home, which `deskevidence`
lands; admission and the record format are unchanged. /73 makes `deskevidence` restamp
or refuse an on-behalf-of annotation that is not the roster principal, with a lint
parity test. /71, /72 and /73 each edit the verify-desk skill, so whichever lands later
merges the others' text. Authoring these briefs does not claim their implementation.

## Shared planner consumers — requested 23 October 2026

[The efficiency amendment](../../desk-workflow-efficiency-spec.md#21-candidate-plus-priority-planner--requested-amendment-23-october-2026)
uses v2's existing typed/custody/query boundary. Reuse /09's completed snapshot and batches;
coordinate /20–21 CLI surfaces with desk-tools/39–43. graph-execution/27 owns the five-role
planner binding and existing runtime owners handle claims/refill. Neither parser migration
nor transport optimization creates a new priority engine. Ranked GH savings and parity
qualification stay with desk-tools/44 and R6; completed Evidence is preserved.

## Shared forge library — ruled 25 October 2026

The driver's ruling on #2395, recorded in
[`DR-forge-library`](../decisions/DR-forge-library.md), supersedes `DR-forge-neutral-36`. Three
human-gated briefs carry it; each cites the record with its `design:` key:

- **/75** (wave 2, after /58) builds the one shared forge library for reads and writes and turns
  `deskread` and the write verbs into thin wrappers around it. The library owns the credential
  (one client per role and repository, private field, no ambient fallback) and carries the five
  named custody records unchanged. It adds the combined gh/glab launch ratchet across desk-tools
  and statusgen.
- **/76** (wave 1) moves statusgen's git launches to go-git on top of the existing `gitcore`,
  with parity goldens for the history and merge-base semantics `statusgen/verifyflip.go`
  depends on, and adds statusgen's down-only git-launch ratchet beside the desk-tools counter
  that `desktools-go-git/08` flips.
- **/77** (wave 8, after /75, /58, /08, `forge-neutral/18` and `/35`) swaps statusgen's reader seam
  from the `deskread` process to the library and retires statusgen's environment-token reads.

Path: `58 → 75 → 77`, with `{forge-neutral/18, /35} → 08 → 77` and `{forge-neutral/18, /35} → 77`; /76 is independent. The /08
edge fixes landing order: /08's Task 2 gate and Task 5 path entry land before /77 widens them, so
the two cannot land the other way round and narrow /77's gate. /08's row 10 admits the library's caller package only when /77 lands. /08 holds statusgen's gh
column of the counter at zero; the rest of the statusgen total (the `origin` classifier and the
API host literals) is /77's: /77 row 9 asserts it at `0`, and /77 Task 7 widens /08's gate to the
whole total so it stays there (row 11 proves the script exits non-zero).
