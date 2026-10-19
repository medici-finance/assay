# deskdispatch CLI compatibility matrix

Captured before the port at `e3710c261` for desktools-v2/21. The item command and
three leading mode selectors share one Cobra/pflag parser implementation. Selectors
remain flags in the public interface, so no previously valid item name becomes a verb.
`help --stamp-only`, `--stamp-only --help` and the equivalent verifier forms select
mode-specific generated help. There are no existing sub-verbs or hidden entrypoints.

## Resolution, before migration

Every CLI setting below is flag > literal default. There is no CLI config file or
implicit environment binding. Present-empty strings override the default; unset flags
fall through. Scalar repeats keep the last value. Config maps cannot supply a CLI setting: a declared flag supplied through a config
map is refused; undeclared map keys are ignored. Similarly named environment variables
are never read. This is intentionally narrower than
the four-source fixture: undeclared config/env inputs retain the
literal default; explicit flags (including empty) win over every undeclared input.

| Mode | Flag | Type | Default |
|---|---|---|---|
| item | `--tier` | string | `"any"` |
| item | `--kit` | string | `"worker"` |
| item | `--repo` | string | `""` |
| item | `--root` | string | `"."` |
| item | `--claim-root` | string | `""` |
| item | `--model` | string | `""` |
| item | `--branch` | string | `""` |
| item | `--brief` | string | `""` |
| item | `--gate-human` | bool | `false` |
| item | `--pr` | int | `0` |
| item | `--prompt-file` | string | `""` |
| item | `--quiet` | bool | `false` |
| item | `--dry-run` | bool | `false` |
| item | `--worktree` | string | `""` |
| item | `--rework` | bool | `false` |
| item | `--attest-worker` | bool | `false` |
| item | `--cross-repo` | bool | `false` |
| --stamp-only | `--repo` | string | `""` |
| --stamp-only | `--pr` | int | `0` |
| --stamp-only | `--model` | string | `""` |
| --stamp-only | `--tier` | string | `""` |
| --stamp-only | `--kit` | string | `"worker"` |
| --stamp-only | `--attestation` | int | `0` |
| --stamp-only | `--dry-run` | bool | `false` |
| verifier checks/recovery | `--root` | string | `""` |
| verifier checks/recovery | `--brief` | string | `""` |
| verifier checks/recovery | `--dry-run` | bool | `false` |
| item | `--force-scan-override` | string | empty |
| all execution modes | `--trace` | bool | false; absent retains DESK_TRACE owner |

`--kits` lists embedded kits; `--version` preserves its existing single-line metadata.
All modes accept conventional help before required execution arguments and before reads.
Generated help prints to stdout with exit 0. Help, version and `--kits` run no config echo,
activation discovery, credential lookup or child process. The one opt-in performance
finalizer wraps the whole command, parsing included. Under `ASSAY_TELEMETRY=1` it writes
one local record classified from the registered first argument, or from the resolved kit
for dispatch and `--stamp-only` execution. An explicit `--trace=false` turns off an
inherited `DESK_TRACE`.
The old root no-argument refusal remains exit 5. Syntax errors (unknown flags, missing or
malformed values, extra positionals) adopt the shared contract's exit 2; domain refusals
remain 5, unavailable reads 6 and disabled guards 3. Existing machine results and
stamp/verifier audit behavior stay in their domain handlers.

## Authority that remains outside Viper

No setting below is promoted into a CLI source. The roster's protected file-only
configuration, activation, kill switches, role binding and repo allowlist remain in
deskkit. DESK_ROOTS is resolved by that existing owner, not a generic CLI environment
binding. The inherited GH_TOKEN still goes through role-identity validation; GitLab
custody remains its existing owner. DESK_SESSION/session suffix, DESK_MIN_FREE_BYTES,
cache/storage policy, repair-admission switches, claim store and hook configuration
keep their existing checks and first-nonempty behavior. Help/version do not invoke them.
The dispatch handler still owns source/head proof, claims, allocator, identity,
rollback, hooks, queue labels, prompt/packet output and model attestation. Stamp-only
and verifier recovery retain their independent role checks even when called directly.

## Consuming forms

The production Go caller outside the command package is
`statusgen/verifieradmission.go:29`; it invokes
`deskdispatch --check-verifier --root ROOT --brief BRIEF`. The typed verifier
mode and every path spelling are covered by the owner tests; the existing statusgen
admission tests keep the subprocess failure/receipt contract.

| Consuming surface | Invocation retained | Coverage |
|---|---|---|
| worker skill and tools README | `deskdispatch ITEM --tier TIER --kit worker --repo REPO --root ROOT --claim-root ROOT --model MODEL --branch BRANCH --brief BRIEF --gate-human --pr N --rework --attest-worker --prompt-file FILE --quiet --dry-run --worktree PATH --cross-repo` (applicable subsets) | every item flag reaches typed dispatch options; inherited domain tests retain invalid-combination refusal |
| worker skill and tools README | `deskdispatch --stamp-only --repo REPO --pr N --model MODEL --tier TIER --kit KIT --attestation N --dry-run` | stamp mode flow/defaults/unknown input; existing stamp custody/readback tests |
| review skill and tools README | `deskdispatch ALIAS--pr-N[--security] --kit review --tier TIER --model MODEL --repo REPO --pr N --prompt-file FILE` | item mode flag flow; existing review-key, fresh allocation and source/round tests |
| verifier skill, statusgen and tools README | `deskdispatch --check-verifier --root HOME --brief PATH` | verifier typed flow/path forms; existing verifier admission tests |
| verifier skill and tools README | `deskdispatch --attest-verifier --root HOME --dry-run` | recovery typed flow; existing independent coordinator admission tests |
| verifier skill and tools README | `deskdispatch ITEM --kit verifier --model MODEL --tier TIER --brief PATH` | item options; existing verifier detached/worktree/receipt tests |
| command reference and embedded prompts | `deskdispatch --kits`, `deskdispatch --version`, root and mode help | actual binary under absent/malformed config; zero config/effect counters |
| tools README diagnostics | `DESK_TRACE=1 deskdispatch ITEM ...`, `deskdispatch --trace ITEM ...`, `deskdispatch ITEM ... --trace` | existing trace owner plus global placement and trace-like values |
| outbound override owner | `deskdispatch ITEM --force-scan-override REASON ...` | string reaches typed handoff; existing validator/context owner preserved |

The generated Cursor skill copies use the same forms as their canonical plugin skills.
No invocation or quoted flag listing in the named contract/quality/manifest consumers
requires rewriting: they reference domain behavior, source ownership or regression names.


The inventory below is all tracked Go, shell, PowerShell, Markdown and workflow files
mentioning this entrypoint at pickup, excluding its own package and generated views.
Historical Evidence and unrelated quoted prose are retained. Executable and documented
forms use item then flags, leading --stamp-only/--check-verifier/--attest-verifier,
--kits/--version and global --trace. Both equals/space and single-dash Go forms remain
accepted; all path fixtures are byte-preserved. Flags before the item are also accepted
by Cobra; -- terminates flags. There are no list flags or opaque child-command argv.
The owner tests replay each mode and each declared flag, repeats, termination, dash
values, persistent trace, parser refusals and all shared path spellings. The inherited
package suite and regression floor exercise real domain consumers and rollback.

- `.cursor/skills/ask-decision/SKILL.md`
- `.cursor/skills/intake-desk/SKILL.md`
- `.cursor/skills/pr-review-desk/SKILL.md`
- `.cursor/skills/pr-review-desk/references/merge-time-recheck.md`
- `.cursor/skills/pr-review-desk/references/verdict-format.md`
- `.cursor/skills/pr-shepherd/SKILL.md`
- `.cursor/skills/the-desk/SKILL.md`
- `.cursor/skills/verify-desk/SKILL.md`
- `.cursor/skills/worker-desk/SKILL.md`
- `.cursor/skills/worker-desk/references/dispatch-runbook.md`
- `.leakshape-allow.yaml`
- `bugs/2199.md`
- `ci/staged-workflows/cellcache-mutate.yml`
- `docs/cellctl-go-cache.md`
- `docs/contracts.md`
- `docs/desk-tools/deskpr.md`
- `docs/desk-tools/hooks.md`
- `docs/quality/QUALITY.md`
- `docs/streams/apps-installer/brief-02-deskapps-manifest-flow.md`
- `docs/streams/brief-quality/brief-06-acceptance-admission.md`
- `docs/streams/brief-quality/spec.md`
- `docs/streams/build-less-brittle/brief-01-semantic-owner-index.md`
- `docs/streams/build-less-brittle/brief-03-weight-ratchet.md`
- `docs/streams/build-less-brittle/brief-05-worker-two-strikes-and-weight.md`
- `docs/streams/build-less-brittle/brief-06-design-fit-review-stage.md`
- `docs/streams/build-less-brittle/brief-07-rule-register-and-diet.md`
- `docs/streams/build-less-brittle/brief-08-hotspot-metric-and-brittle-mark.md`
- `docs/streams/build-less-brittle/brief-09-brittle-investigation.md`
- `docs/streams/build-less-brittle/brief-11-regression-test-ratchet.md`
- `docs/streams/build-less-brittle/brief-12-refactor-oracle.md`
- `docs/streams/build-less-brittle/brief-13-incident-refactor-by-agent.md`
- `docs/streams/composability/brief-01-reactive-activation.md`
- `docs/streams/contributor-trust/brief-04-review-depth-by-tier.md`
- `docs/streams/decisions/DR-forge-neutral-21.md`
- `docs/streams/derived-board/brief-03-reconcile-derivation.md`
- `docs/streams/desk-containers/brief-10-cellctl-go-port.md`
- `docs/streams/desk-containers/brief-25-desk-force-repair.md`
- `docs/streams/desk-supervision/08-report.md`
- `docs/streams/desk-supervision/README.md`
- `docs/streams/desk-supervision/brief-02-run-stop-signal.md`
- `docs/streams/desk-supervision/brief-04-lifecycle-hooks.md`
- `docs/streams/desk-supervision/brief-06-workpad.md`
- `docs/streams/desk-supervision/brief-08-objectives-over-transitions.md`
- `docs/streams/desk-supervision/brief-09-ci-fanout-per-push.md`
- `docs/streams/desk-supervision/brief-13-worker-operations-vitals.md`
- `docs/streams/desk-supervision/brief-17-verification-repair-obligations.md`
- `docs/streams/desk-supervision/brief-18-repair-admission.md`
- `docs/streams/desk-supervision/brief-19-review-finding-continuity.md`
- `docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md`
- `docs/streams/desk-supervision/brief-21-external-prerequisite-reverification.md`
- `docs/streams/desk-supervision/brief-22-cell-model-policy.md`
- `docs/streams/desk-supervision/brief-24-per-file-verify-outcomes.md`
- `docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md`
- `docs/streams/desk-supervision/brief-28-dispatch-record.md`
- `docs/streams/desk-supervision/brief-32-human-decision-record.md`
- `docs/streams/desk-supervision/brief-34-worker-usage-counts.md`
- `docs/streams/desk-tools/README.md`
- `docs/streams/desk-tools/brief-01-binary-channel-and-pin-contract.md`
- `docs/streams/desk-tools/brief-07-clusterguard-exec-shim.md`
- `docs/streams/desk-tools/brief-15-deskdispatch-dryrun-worktree.md`
- `docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md`
- `docs/streams/desk-tools/brief-23-usage-and-timing-telemetry.md`
- `docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md`
- `docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md`
- `docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md`
- `docs/streams/desk-tools/brief-29-verifyloop-unsigned-compose.md`
- `docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md`
- `docs/streams/desk-tools/brief-33-author-disposition-finality.md`
- `docs/streams/desk-tools/brief-34-pinned-golangci-lint-across-every-go-module.md`
- `docs/streams/desktools-go-git/brief-06-migrate-push-retire-ambient.md`
- `docs/streams/desktools-v2/README.md`
- `docs/streams/desktools-v2/brief-06-installation-token-scoping.md`
- `docs/streams/desktools-v2/brief-09-access-pattern-queries.md`
- `docs/streams/desktools-v2/brief-10-one-outbound-write-check.md`
- `docs/streams/desktools-v2/brief-11-outbound-house-callout.md`
- `docs/streams/desktools-v2/brief-12-platform-compatibility-suite.md`
- `docs/streams/desktools-v2/brief-14-regression-floor.md`
- `docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md`
- `docs/streams/desktools-v2/brief-38-migrate-the-pr-writer-verbs-to-cobra-and-viper.md`
- `docs/streams/desktools-v2/inventory.md`
- `docs/streams/desktools-v2/spec.md`
- `docs/streams/desktools-v2/staged/portability-baseline.md`
- `docs/streams/findings/F-path-claim-tree.md`
- `docs/streams/forge-gitlab/README.md`
- `docs/streams/forge-gitlab/brief-08-close-the-forge-surface.md`
- `docs/streams/forge-gitlab/brief-11-guard-read-custody.md`
- `docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md`
- `docs/streams/forge-gitlab/brief-13-board-reads-degrade-per-row.md`
- `docs/streams/forge-gitlab/inventory.md`
- `docs/streams/forge-gitlab/pilot-report.md`
- `docs/streams/forge-neutral/README.md`
- `docs/streams/forge-neutral/brief-05-claim-layer-forge-shape.md`
- `docs/streams/forge-neutral/brief-06-read-verbs-on-the-seam.md`
- `docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md`
- `docs/streams/forge-neutral/brief-14-run-and-gate-approval-verbs-deskrun.md`
- `docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md`
- `docs/streams/forge-neutral/brief-16-deskclose-widened-lanes.md`
- `docs/streams/forge-neutral/brief-20-claim-store-measurements.md`
- `docs/streams/forge-neutral/brief-21-claim-store-seam-and-resolver.md`
- `docs/streams/forge-neutral/brief-24-served-claim-store.md`
- `docs/streams/forge-neutral/brief-31-remaining-roles-write-audit.md`
- `docs/streams/forge-neutral/brief-32-forge-store-deletion.md`
- `docs/streams/forge-neutral/brief-33-forge-reads-for-statusgen-s-remaining-sites.md`
- `docs/streams/forge-neutral/conformance-report.md`
- `docs/streams/forge-neutral/reviewer-write-boundary.md`
- `docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md`
- `docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`
- `docs/streams/graph-execution/brief-07-flow-instruments.md`
- `docs/streams/graph-execution/brief-14-admission-dispatch-binding.md`
- `docs/streams/graph-execution/brief-20-runner-contract.md`
- `docs/streams/harness-portability/brief-09-jcode-desk-harness-spike.md`
- `docs/streams/harness-portability/brief-10-specmem-portable-memory-spike.md`
- `docs/streams/harness-portability/brief-15-ci-wiring-harnesslint.md`
- `docs/streams/harness-portability/brief-16-codex-long-context-cap.md`
- `docs/streams/iso-9001/brief-03-corrective-action-effectiveness.md`
- `docs/streams/quality/brief-19-refix-metric.md`
- `docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md`
- `docs/streams/statusgen/brief-19-context-handoff-contracts.md`
- `docs/streams/windows-port/brief-12-deposix-skill-prose-and-constants.md`
- `docs/streams/windows-port/brief-14-windows-leg-proves-desk-role-paths.md`
- `docs/streams/windows-port/brief-15-inbox-html-flow-port.md`
- `docs/workspace-callers.md`
- `plugins/assay/skills/ask-decision/SKILL.md`
- `plugins/assay/skills/intake-desk/SKILL.md`
- `plugins/assay/skills/pr-review-desk/SKILL.md`
- `plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md`
- `plugins/assay/skills/pr-review-desk/references/verdict-format.md`
- `plugins/assay/skills/pr-shepherd/SKILL.md`
- `plugins/assay/skills/the-desk/SKILL.md`
- `plugins/assay/skills/verify-desk/SKILL.md`
- `plugins/assay/skills/worker-desk/SKILL.md`
- `plugins/assay/skills/worker-desk/references/dispatch-runbook.md`
- `statusgen/admittedrowenv_test.go`
- `statusgen/rosterfixture_test.go`
- `statusgen/verifieradmission.go`
- `statusgen/verifieradmission_test.go`
- `tools/cellctl/testdata/cellctl-shell-oracle.sh`
- `tools/cellctl/tests/cell-set.test.sh`
- `tools/cellctl/tests/cockpit.test.sh`
- `tools/cellctl/tests/desk-worktree-merge.test.sh`
- `tools/cellctl/tests/gen-shims-gh-token.test.sh`
- `tools/cellctl/tests/gitlab-fetch-cred.test.sh`
- `tools/cellctl/tests/harness.test.sh`
- `tools/cellctl/tests/house-cell.test.sh`
- `tools/cellctl/tests/model-namespace.test.sh`
- `tools/cellctl/tests/model-override.test.sh`
- `tools/cellctl/tests/model-pin.test.sh`
- `tools/cellctl/tests/parity.test.sh`
- `tools/cellctl/tests/provider.test.sh`
- `tools/cellctl/tests/role-context.test.sh`
- `tools/cellctl/tests/root-drift.test.sh`
- `tools/cellctl/tests/scrubbed-cell.test.sh`
- `tools/desk/README.md`
- `tools/desk/cmd/cellctl/cell.go`
- `tools/desk/cmd/cellctl/codex_environment.go`
- `tools/desk/cmd/cellctl/desk.go`
- `tools/desk/cmd/cellctl/launch.go`
- `tools/desk/cmd/cellctl/repairadmission_binary_test.go`
- `tools/desk/cmd/cellctl/shims.go`
- `tools/desk/cmd/cellctl/show.go`
- `tools/desk/cmd/deskboard/admission.go`
- `tools/desk/cmd/deskboard/admission_test.go`
- `tools/desk/cmd/deskboard/dispatch.go`
- `tools/desk/cmd/deskboard/main.go`
- `tools/desk/cmd/deskclaim-ref/claim.go`
- `tools/desk/cmd/deskclaim-ref/deskclaimref_test.go`
- `tools/desk/cmd/deskclaim-ref/main.go`
- `tools/desk/cmd/deskclaim-ref/releaserecord_test.go`
- `tools/desk/cmd/desklabel/vocabulary.go`
- `tools/desk/cmd/deskpost/claimliveness.go`
- `tools/desk/cmd/deskpost/modelfloor_test.go`
- `tools/desk/cmd/deskpr/crossrepotrailer.go`
- `tools/desk/cmd/deskreply/workpaddoc_test.go`
- `tools/desk/cmd/deskrestamp/forge.go`
- `tools/desk/cmd/desksupervise/actions.go`
- `tools/desk/cmd/desksupervise/live.go`
- `tools/desk/cmd/desksupervise/reconcile.go`
- `tools/desk/cmd/desksupervise/stop.go`
- `tools/desk/cmd/deskwt/deskwt.go`
- `tools/desk/cmd/deskwt/huskremove.go`
- `tools/desk/cmd/deskwt/main.go`
- `tools/desk/cmd/deskwt/roleinit.go`
- `tools/desk/cmd/deskwt/trace_test.go`
- `tools/desk/cmd/fanoutloop/repair.go`
- `tools/desk/cmd/fanoutloop/represented.go`
- `tools/desk/cmd/fanoutloop/represented_live.go`
- `tools/desk/cmd/reviewloop/coalesce.go`
- `tools/desk/cmd/reviewloop/surfaces.go`
- `tools/desk/cmd/scanloop/monitor.go`
- `tools/desk/cmd/verifyloop/dispatch.go`
- `tools/desk/cmd/verifyloop/dispatch_native.go`
- `tools/desk/cmd/verifyloop/dispatch_sync_test.go`
- `tools/desk/floorconsumers_test.go`
- `tools/desk/hooks.example.yaml`
- `tools/desk/internal/cachequiet/cachequiet.go`
- `tools/desk/internal/cachequiet/cachequiet_test.go`
- `tools/desk/internal/cellcache/admit.go`
- `tools/desk/internal/deskkit/ambienttoken_guard_test.go`
- `tools/desk/internal/deskkit/audittoolkey.go`
- `tools/desk/internal/deskkit/briefadmission.go`
- `tools/desk/internal/deskkit/briefadmission_test.go`
- `tools/desk/internal/deskkit/briefauthoring.go`
- `tools/desk/internal/deskkit/briefrisk.go`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `tools/desk/internal/deskkit/claimref.go`
- `tools/desk/internal/deskkit/claimstore_test.go`
- `tools/desk/internal/deskkit/crossrepotrailer_test.go`
- `tools/desk/internal/deskkit/dispatchrecord.go`
- `tools/desk/internal/deskkit/forge.go`
- `tools/desk/internal/deskkit/forge_gitlab_test.go`
- `tools/desk/internal/deskkit/forgeresolve.go`
- `tools/desk/internal/deskkit/forgeresolve_test.go`
- `tools/desk/internal/deskkit/killswitch.go`
- `tools/desk/internal/deskkit/modelfloor.go`
- `tools/desk/internal/deskkit/modelfloorrisk_test.go`
- `tools/desk/internal/deskkit/modelstamp.go`
- `tools/desk/internal/deskkit/prrepr.go`
- `tools/desk/internal/deskkit/publishidentity.go`
- `tools/desk/internal/deskkit/repairadmission.go`
- `tools/desk/internal/deskkit/repairobligation.go`
- `tools/desk/internal/deskkit/reviewlanes.go`
- `tools/desk/internal/deskkit/reviewlanes_test.go`
- `tools/desk/internal/deskkit/roleworktreeidentity.go`
- `tools/desk/internal/deskkit/roleworktreeidentity_test.go`
- `tools/desk/internal/deskkit/runkey_test.go`
- `tools/desk/internal/deskkit/runtool.go`
- `tools/desk/internal/deskkit/scrub.go`
- `tools/desk/internal/deskkit/stampage.go`
- `tools/desk/internal/deskkit/stampage_test.go`
- `tools/desk/internal/deskkit/trace_test.go`
- `tools/desk/internal/deskkit/trailer.go`
- `tools/desk/internal/fleetharness/fleetharness_test.go`
- `tools/desk/internal/forgeban/allowlist.go`
- `tools/desk/internal/regression/MANIFEST.md`
- `tools/desk/internal/regression/REVIEW-REPAIR.md`
- `tools/desk/internal/testledger/ledger_test.go`
- `tools/desk/internal/weight/weight.go`
- `tools/desk/internal/weight/weight_test.go`
- `tools/harnessgen/redaction_test.go`
- `tools/lint-human-decision-defaults.sh`

## Other script consumers

The Python/JavaScript and extensionless-launcher sweep found no additional production
invocation. Its two Python hits are mutation fixtures:
`tools/desk/cmd/deskdispatch/testdata/stamp_mutations.py` and
`tools/desk/internal/deskkit/testdata/crossrepo_mutations.py`.
All 15 stamp and 48 cross-repository anchors were checked against the migrated tree.
The stamp runner's required-tier default plant now targets the Cobra declaration in
`cli.go`, preserving the same mutation and required-input assertion after the Go FlagSet
was removed. The other 62 anchors remain unchanged. The stamp runner executes each
selected Go test with its existing 60-second bound; the CLI owner's muhar spec separately
covers eager config or activation discovery, source widening, legacy flags, generated help
and the performance finalizer staying outside parsing.
