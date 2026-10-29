# Write-policy source and consumer census

Source delivery for desktools-v2/66, 20 October 2026. Pickup inspected
`c7afc51121d636c0a6200f454dc45287ad35e32f`. Before implementation, ordinary
merge-current advanced the branch to `f09ef6239aa1439473717f9c795aacf393e5462a`;
writeguard, deskcore, contracts and the build/workflow readers below were unchanged
between those revisions. Subsequent merge-current reached
`ce25be49efeb8a345019c6a663002edfd705da27`; no target policy source changed.
This PR carries the producer handoff and adapter repair. An immutable public
module source version is qualified below; this is not an installed hook receipt.

The human approved option 1 of the bounded extraction and closed
[decision #3752](https://github.com/medici-finance/assay/issues/3752#issuecomment-6299695731).
That pickup approval preserves legacy behavior and requires separate managed
qualification; it grants no merge, installation, release or activation authority.
See [the brief](streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md),
[write-policy contract](write-policy.md), [semantic index](contracts.md),
[library-first](library-first.md) and [Go engineering](go-engineering.md).

## Architecture pickup and result

`deskcore/writepolicy` owns the pure evaluator inside the existing core module.
`tools/desk/cmd/writeguard` remains the stdin/JSON process boundary. All original
regression assertions remain in place; test-only compatibility names delegate to
the production adapter. The old command-local policy implementation is removed.
Observation, confinement, allocation and forge authority remain separate owners.
The target architecture amendment remains proposed history; the recorded pickup
ruling above is this extraction's authority.

## Search and dispositions

The pickup searched tracked Go, shell, Python, JSON and workflow sources for
`writeguard`, `write.policy`, `write-isolation`, `shared.checkout`, `SharedRoot`,
`CheckFilePath`, `CheckBash`, and `CheckClaimSentinel`, then inspected the
decision-bearing candidates listed below. Tests, prose, mutation data and path references are
not counted as another production evaluator. These queries discover candidates;
the consumer test additionally detects unregistered
decision sites and stale listed consumers.

| Source / consumer | Observed meaning | Disposition and owner | Retirement or acceptance condition |
|---|---|---|---|
| `tools/desk/cmd/writeguard/guard.go` | Denial rendering from core outcome details | Policy moved to `deskcore/writepolicy`; denial text remains in the adapter | All inherited regression cases plus independently expected core/built-hook parity pass; old command-local decision implementations removed |
| `tools/desk/cmd/writeguard/observe.go` | Explicit observation collector and direct evaluator import | Hook adapter under /66 | Built-hook parity and observation request validation |
| `tools/desk/cmd/writeguard/main.go` | Hook input/output, activation, Git discovery, configuration, sentinel file/time observations and ordered composition | Retained adapter; decision order and exemption semantics now live in the core | Actual built binary preserves tool names, no-decision paths, denial JSON and exit status |
| `tools/desk/cmd/writeguard/callout.go` | Trusted callout execution, request/answer protocol and diagnostics | Retain effects/protocol under /66; provide explicit callout observations to the core | A configured failure refuses; an allow answer cannot clear any core deny; invocation order/counts retained |
| `tools/desk/internal/deskkit/callout.go` and `rosterconfig.go` | Executable trust checks and approved configuration source | Retain independent trust owners | Existing callout trust and roster-source tests continue to pass; no parallel credential/config reader in the core |
| `tools/desk/cmd/writeguard/settings_wiring_test.go` | Installed hook wiring and missing-binary expectations | Retain deployment-contract test under /66 | Test checks actual supported wiring or accurately reports unavailable adopter configuration; source publication is not activation |
| `loopadmin/adapters/localharness/` | Planned direct evaluator consumer | Follow-up graph-execution/22 | Exact producer API/profile pin; admitted allocation reaches real adapter and independent sandbox; absent/stale/foreign evidence holds |
| `tools/desk/cmd/cellctl/launch.go` and related launch builders | Existing harness argv and environment construction | Retain launch owner; integrate through graph-execution/22 and /27 | Approved profile qualification, sanitized environment and confinement evidence; no implicit enrollment from a library import |
| `statusgen/scanguard.go` | Primary-checkout refusal for `--scan-issues`, including non-hook contexts and a distinct human override | Retain independent scan boundary | Not a duplicate of the shared-home hook exemption: a hook-exempt or non-hook scan must still face this control |
| `tools/desk/cmd/scanloop/lane.go` and `deskreconcile/reconcile.go` | Require isolated scan/reconcile destinations | Retain operation-specific isolation boundaries | Existing refusal tests remain; their allocation migration follows workspace ownership |
| `workspace/`, `tools/desk/cmd/deskwt/`, `deskdispatch/worktree.go`, `deskboot/boot.go` | Allocation, lifecycle, home admission, identity and cleanup refusal | Retain `S-worktree` and admission owners | Allocation is not write permission; no blanket evaluator import or retirement of these controls |
| `statusgen/verifysandbox.go` | Independent verification runner sandbox | Retain sandbox owner | Existing bypass-the-hook confinement checks remain independent of a library allow result |
| Push, credential, process, scratch and retirement guards | Independent effect/trust boundaries | Out of scope for /66 | Preserve controls and their tests; no authority consolidation |

No additional production copy of the hook's file/shell/sentinel evaluator was
identified in this pickup. That is a source-bounded observation, not an assertion
that future adapters cannot duplicate it.

## Compatibility report

The following behaviors are preservation obligations, not newly granted authority.

| Boundary | Source behavior to preserve | Qualification |
|---|---|---|
| Hook protocol | `Edit`, `Write`, `MultiEdit`, `NotebookEdit`, `Bash`; file path then notebook path; unsupported/empty target produces no decision | Built-binary golden inputs and stdout/exit expectations |
| Legacy discovery failure | Malformed top-level/tool JSON, missing project directory or failed main-checkout Git discovery gives no stdout and exit zero | Malformed input and disposable non-repository binary fixtures |
| Terminal core Unknown | Invalid, unsupported or otherwise unresolved terminal evaluations refuse in the hook adapter; they are not legacy parse/Git no-decision exits | Actual adapter mapping fixtures and an Unknown-to-allow production mutation |
| Activation/configuration | Existing activation check and `ClassWrite` config-home source remain; environment cannot replace callout policy | Built hook activation and environment-versus-config fixtures |
| Sentinel order | Sentinel mutation is refused before shared-home exemption, including already-exempt sessions | Direct/core binary cases plus order-reversal mutation |
| Claim | Any nonempty environment claim, or non-directory readable sentinel; default 12-hour expiry; nonblank non-comment scoped roots, or unscoped empty/comment body | Explicit observation tests and adapter fixtures, including the existing inclusive expiry boundary and future-mtime behavior, aliases and invalid observations |
| Shared-home exemption | Claimed token plus project home equal shared root; supplied Git cwd top must agree; cwd outside the worktree carve-out; legacy missing cwd/top fallback retained | Legacy matrices and separate managed missing-context negatives |
| File paths | Home expansion, relative cwd resolution, deepest existing symlink ancestor, `.claude/worktrees` carve-out | Real symlink/new-descendant observation round trip and deterministic supplied-observation tests |
| Shell scope | Shared-path mention or shared cwd arms shared checks; sentinel check is independent of that scope | Pure reads, outside writes, encoded-prefix false positives and sentinel-only writes |
| Shell classification | Existing literal/heredoc masking, executable substitutions, invocation recognition, copy destination/source-mutating flag handling and every indicator occurrence | Retain source-bound regressions and shared independent fixture cases |
| Effective cwd | Advance only through the supported leading `&&` cd chain; unmodelled changes fall back to payload cwd in the legacy profile | Failed/conditional/subshell/semicolon cd and known/unknown expansion cases |
| Unknown target | Legacy unresolved write heads use the existing cwd fallback; unresolved cd does not advance cwd; missing write target remains fail-safe when shared scope is armed | Separate unknown observations and unchanged legacy decision oracle |
| Callout | Runs last and only for in-scope Bash after compiled checks decline; missing configuration is allowed, configured execution/timeout/answer failure refuses | Invocation recorder, failure cases, deny-clearing mutation |
| Roster failure | Invalid roster may collapse to unconfigured callout and is reported by existing configuration echo | Preserve and disclose actual fallback; do not relabel it fail-closed |
| Coverage limits | Heuristic command recognition cannot contain arbitrary interpreter/helper effects | Bypass-hook sandbox tests; no claim of universal command detection |

The stale README entry for `find . -delete` is corrected from existing
`findMutationTargets` and the 18 inherited `TestIssue1193FindDeleteExec` cases.
This extraction does not broaden parsing or claim all `find` forms.
Callout response parsing must preserve the actual `strings.Cut(..., " ")`
delimiter and case/whitespace handling in `callout.go`; its introductory prose
describes a whitespace token more broadly than that implementation.

## Implemented API and profile handoff

Import `github.com/medici-finance/assay/deskcore/writepolicy`. The exact exported
surface is [policy.go](../deskcore/writepolicy/policy.go). `APIVersion` is
`writepolicy/v1`; `LegacyHook` is `legacy-hook/v1` and `Managed` is `managed/v1`.
These identify source contracts, not published versions or qualified deployments.

- `Classify(Request, LexicalContext) Classification` freezes tool/target, platform,
  cwd, home and a defensive copy of the explicit environment. Its hash binds this
  snapshot; token and literal-mask accessors return values or defensive copies.
  `ReadsAmbientForgeToken()` reports, lexically, whether a classified Bash command
  runs the gh CLI's token read; the hook adapter gates the refusal on its own
  environment ([write-policy.md](write-policy.md#ambient-forge-credential-refusal)).
- `Evaluate(Evaluation) Outcome` accepts that classification and its matching ID,
  policy roots, path/claim/context/callout observations. It returns `allow`, `deny`
  or `unknown`, a stable reason code, schema revision and rendering details.
- `Outcome.Need.Paths` requests exact cleaned lexical inputs. Supply one
  `PathObservation` per request, with `resolved` plus deepest existing ancestor,
  resolved ancestor and suffix, or an explicit `failed`. Duplicate, inconsistent
  and unbound observations yield `unknown`. No resolver callback enters the core.
- Reevaluate the same classification after each batch. Later relative paths can
  depend on an earlier observed symlink/cwd; they are requested in a later round.
  Provisional lexical paths never escape as a final allow/deny while a requested
  observation is missing. This iterative protocol settles the proposal's dependency
  graph without exporting speculative candidate offsets or an eager flat path list.
- `Need.ClaimBody` requests a sentinel body only after supplied metadata passes the
  legacy directory/expiry checks. `Need.Callout` requests the configured callout
  only after earlier compiled checks decline. The adapter supplies its result and
  retains the exact existing diagnostics. A configured failure still refuses.

`managed/v1` requires a validated, matching allocation/generation/repository/owner/
session binding, supported producer API, supplied freshness interval, and mandatory
cwd/project/shared/Git-root context. Missing, stale or foreign evidence, an agent
claim, a legacy environment exemption, or unresolved destinations hold the operation.
The caller must authenticate and revalidate these observations at its effect
boundary; setting `Validated` is not proof of authentication. Explicit claim times
never consult the host clock. Legacy unknown/Git no-decision mappings cannot be used
by a managed adapter. No managed consumer is activated or qualified here.

The public adapter currently uses `legacy-hook/v1` only. Its malformed protocol and
Git-discovery no-decision paths remain in `main.go`; a configured callout failure
remains different from an unconfigured/invalid-roster fallback. No blanket claim of
shell, sentinel or sandbox coverage follows from the positive fixtures.

## Module, build and workflow handoff

Both modules use Go 1.27.0. `deskcore` has no external dependencies. `tools/desk`
now requires it using the repository's existing sibling source-build convention;
that zero pseudo-version and relative replacement are not public resolution proof.
Both Docker build contexts copy deskcore alongside the other sibling modules.
Release command builds use the full source checkout and therefore see that module.

`TestWritePolicyConsumers/external-offline-module` packages the actual deskcore
source into a supplied file proxy under the explicitly synthetic `v0.0.0-fixture`,
then runs a separate module with `GOWORK=off`, an empty module cache, no `replace`,
no network and no ambient private-module configuration. The consumer asserts a
shared-path denial and managed missing-context hold. This proves module packaging
and import behavior, not the existence of a release or checksum in a public proxy.
The concrete immutable source handoff is instead:

| Identity | Exact value |
|---|---|
| Module | `github.com/medici-finance/assay/deskcore` |
| Available source pseudo-version | `v0.0.0-20261020202831-a5c75d60d584` |
| Public source commit | `a5c75d60d584e7bcfe7a97510a502567f1dc97a6` |
| Complete module Git tree at that commit | `158db710b9860994fa7a680d259c8e05c32d05a3` |
| Write-policy Git tree at that commit | `ecef94d5be39d593c3286d3f72957e220cecb32b` |
| API / compatible profiles | `writepolicy/v1` / `legacy-hook/v1`, `managed/v1` |

The verified external consumer uses this exact module requirement:

```go.mod
require github.com/medici-finance/assay/deskcore v0.0.0-20261020202831-a5c75d60d584
```

Its corresponding authentic `go.sum` records are:

```go.sum
github.com/medici-finance/assay/deskcore v0.0.0-20261020202831-a5c75d60d584 h1:EFIFHAvlGWz8l0/W19D/6AQq29MWxfcCKnF58qC6OZ0=
github.com/medici-finance/assay/deskcore v0.0.0-20261020202831-a5c75d60d584/go.mod h1:fT/P/8tLzSLzogIRNMLXVu8yWnqVOm3xwqf/tMjjPTg=
```

On 20 October 2026 the public Go proxy resolved that exact commit and served the
module with checksum verification enabled. All 57 module source blobs matched
the Git objects byte for byte; the zip's inherited root license also matched.
All 11 production Go/module-manifest blobs are identical to this PR's repaired
source. Subsequent adapter repairs and added core regression tests do not change
that module's production API or behavior; the published source version does not
contain those later tests or the command-local adapter.

A second qualification copied the actual downloaded `.info`, `.mod` and `.zip`
into a supplied offline file proxy, then ran the external consumer with an empty
module cache, the two recorded sums, `GOWORK=off`, `-mod=readonly`, no `replace`,
VCS disabled and no network. It returned `writepolicy/v1 shared=deny
managed-missing=unknown`. This proves the exact available source version resolves
and imports independently, beyond the synthetic packaging fixture. The read-only
public resolution can be repeated with:

```sh
GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  go mod download -json github.com/medici-finance/assay/deskcore@v0.0.0-20261020202831-a5c75d60d584
```

For an independently admitted Go consumer, that exact module requirement and
those sums are the compatible source pin to qualify; no consumer pin is changed
by this handoff. The umbrella release builds tools/desk from its complete source
checkout with the existing sibling replacement, including the core copied by
both Docker contexts. It does not resolve the placeholder version from a proxy.
The independent route above requires neither a workspace nor that replacement.
The nested module has no semantic release tag at this check; umbrella `v*` tags
release tools and do not themselves name `deskcore/vX.Y.Z`. No tag or release was
created here, and a future named release still follows the normal release owner.
Managed profile activation, observation authentication and sandbox qualification
remain with graph-execution/22 and /27; source availability grants none of them.

The unfiltered `.github/workflows/ci.yml` runs the desk suite on all PR/push
source changes, including the consumer/parity fixtures, and now invokes the
standalone deskcore suite in its module loop. `.github/workflows/release.yml`
also invokes deskcore tests and vet in its desk-tools test leg. The human workflow
owner landed those additions in the follow-up
[#3793](https://github.com/medici-finance/assay/pull/3793); the recorded hunk and
landing procedure remain in [write-policy-ci.diff](runsheets/write-policy-ci.diff)
and [write-policy-ci.md](runsheets/write-policy-ci.md).

`ci/staged-workflows/release.yml` is a declared identical twin of the live release
workflow. It carries the same deskcore invocation; the release-image contract and
staged-drift guard require byte equality, independently of the publication and
approval checks. The historical `tools/release/release.yml.proposed` is a separate
proposal, not a parity twin or a promotion source. No workflow permission, trigger
or installed pin changes here. Landing the invocations does not prove their
execution: current-head CI, the normal release lane and managed-consumer
qualification retain their separate evidence requirements.

The checked census scans production Go imports/declarations, skipping hidden,
vendor, testdata and test sources. It catches known duplicate decision names and
indicator owners plus unregistered/stale imports; independent design review still
owns differently named semantic duplicates. The semantic marker/ceiling checks
pin `S-write-policy` to its actual evaluator. Cross-module readers are registered
against the unfiltered build-test job. No allocator was relabelled a policy duplicate.

## Local qualification and cost

The named brief tests cover independent typed expectations, real built-hook parity,
actual production denial/order/admission and terminal-Unknown mapping mutations, transitive effect-helper
mutations, and external module resolution. All inherited hook tests remain. The
real pre-extraction binary is an additional local byte-for-byte stdout/exit oracle;
it is never the sole decision oracle. Tests use disposable files and Git repositories.
The injected statusgen sandbox precheck/no-exec and CI-mode refusal tests run
independently of the hook. They do not certify an installed managed filesystem
sandbox. Installed hook wiring remains could-not-check: the inherited
`TestSettingsJSONWiresWriteguard` skips because this public tree has no
`.claude/settings.json`. Native execution was Darwin arm64, Go 1.27.2. Windows
lexical fixtures are supplied; native Windows behavior is not qualified here.
The Windows qualification owner must cover backslash command basenames and
case-insensitive environment names before activation. The observation loop
currently terminates structurally: each requested path is memoized, the frozen
command and scoped claim body are finite, and claim-body/callout pending states
advance once. A numeric round cap is deferred to the adapter protocol owner;
an arbitrary cap would change valid long-command legacy behavior. Future changes
to request generation must preserve progress and this termination argument.

The same nine disposable hook fixtures were measured before and after extraction.
Counters were added only to private source copies; their stdout/exit matched the
uninstrumented binaries. Path resolver requests were 62 before / 36 after;
`EvalSymlinks` attempts 102 / 67; Git API invocations 18 / 18; configured callout
invocations 1 / 1. Five uninstrumented workload rounds measured median 112.952 ms /
115.899 ms on the same local host. These are a bounded observation receipt, not a
performance target or a general speedup claim. Memoized observations reduce repeated
filesystem reads within one evaluation; filesystem races remain an adapter and
sandbox concern. Full machine-local evidence and baseline coverage observations
remain private; no newly observed parser behavior is repaired by this extraction.
