# Delivery profiles for staged system outcomes

**Version:** v1.0-draft
**Status:** draft — proposed scope, not approved implementation or acceptance.
**Routes-to:** docs/streams/delivery-profiles/

## Outcome and decision

An operator can run a repeatable, isolated cockpit → real orchestration → fake runner
journey, observe and cancel scripted jobs, exercise failure/recovery cases, and export
an evidence bundle that identifies every substitute. Later, the same consumer journey
can qualify a real runner without inheriting simulated acceptance. This is a delivery
and integration capability, not a unit-test mock generator or a production environment.

Proposed default: one versioned delivery-profile contract, pure shared validation in
public Assay's existing `loopadmin` module, and lifecycle/transport adapters in the
existing public service and cockpit. No new service, authority, state store, universal
mock framework, dependency or workflow controller. Profiles describe composition and
qualification; they do not schedule work, grant permission or accept a milestone.

The driver approves scope and stream admission through the normal spec lifecycle.
This PR's existence authorizes neither. Implementation briefs retain their own gates.

## Sources and observed baseline — 30 October 2026

| Source | Revision / observation | Consequence |
|---|---|---|
| Assay public main | `15ab949bd6158c2a9ef469adf362f3fb5d2fca5a`, fresh fetch | `spec/milestone-v1.md`, `spec/loop-admin-runner-v1.md`, `docs/library-first.md`, `docs/go-engineering.md` control shared meanings. |
| Public service placement | `deskd/README.md`, `deskd/go.mod`, `deskd/internal/serve`, `deskd/internal/contract`, `deskd/cmd/deskd` at selected main | Real compatibility service is public and buildable in a standalone module; its TypeScript declaration is not a migrated desktop cockpit. |
| R7 public clients | `docs/streams/cell-api/README.md`, /01–04 at selected main | Public contract, service composition, mounted authorization and thin clients remain todo. Reuse these owners; initial operator surface is the real thin cockpit from /04, not a new desktop migration. |
| Target architecture | `docs/proposals/non-ci-architecture/11-target-architecture.md` and /20–21 with their recorded approval scope | Shared software is public; private adopter values and historical sources do not enter this plan. |
| Runner implementation | Assay `loopadmin/runner/{client,adapter,executionprofile}.go`, `runner/fake`, `runner/conformance` | Importable public module with in-memory fake and fault controls. It starts no process and makes no provider call. |
| Orchestration owner | public `graph-execution/26` is todo; `loopadmin` contains runner protocol but no delivered supervisor | Real shared supervisor integration is an external prerequisite. A Kubernetes job renderer or terminal session supervisor is not the /26 launch supervisor. |
| Real runner owner | public `graph-execution/22` is unfinished | Real-adapter replacement qualification waits for an actual compatible published adapter; it is not part of first fake-profile acceptance. |
| Quality evidence | `quality/18` stub-coverage report is todo; no measured seam coverage or performance baseline collected for this change | Use explicit contract and consumer journeys; do not claim existing universal coverage or measured savings. |

Observed interfaces do not establish a runnable cockpit-to-supervisor route today.
The profile lifecycle and bridge belong in the selected public service host after cell-api/02,
reusing /26's public API after it ships and preserving cell-api/03 authorization. Do not mock that bridge to claim the first integrated outcome. If /26 cannot
be imported at its actual module/version boundary, /03–04 hold while /01–02 can proceed.
Active claims and file-level PR overlap were not exhaustively inspected at authoring;
pickup must refresh them and checkpoint affected owners before changing shared seams.

## Terms and contract ownership

A **delivery profile** is a versioned, reviewable composition for a named outcome and
supported environment. A **stub** supplies canned outputs; a **mock** also checks expected
interactions; a **fake** implements simplified stateful behaviour; a **simulator** models
selected conditions. These modes say what is substituted, not how trustworthy it is.

The proposed generic wire contract is `delivery-profile-v1`, published with JSON schema
and conformance fixtures by /01. Until then the shape below is design prose, not live YAML
or a new milestone field. The runtime decoder and cross-field validator have one owner:
`github.com/medici-finance/assay/loopadmin/deliveryprofile` (planned).

Required semantic content:

- Profile identity and positive contract version; intended outcome/milestone version
  when one is supported; exact source/configuration subject and required reader version.
- Component entries: stable component key, mode (`real`, `stub`, `mock`, `fake`,
  `simulator`), exact artifact revision/digest, interface contract/version, allowed
  capabilities, limitations and scenario references. Missing/unknown modes refuse.
- Connection edges: consumer/provider keys, contract/version and supported operation.
  Selection binds exact entries; cycles inappropriate to startup and unknown endpoints
  are diagnosed. No URL, command or capability is inferred from a display label.
- Named deterministic scenarios: seed/logical progression, initial state, inputs,
  expected observations, supported fault controls, bounded completion conditions and
  unsupported behaviour. Unknown outcomes remain unknown.
- Disposable resource requirements, lifecycle recipe references, evidence destinations,
  teardown ownership and real-service replacement qualification requirements.

Schema validates shape; the pure library validates semantics; host adapters perform I/O
and enforce containment. Store all execution recipes as reviewed code, not shell strings
or arbitrary executables carried by a profile. Caller-supplied configuration cannot
select an arbitrary production target or provider, bypass authority or mint a credential.
The runtime observation of each component's mode and artifact must come from the selected
host/adapter binding, not merely echo a user-authored `real` label. Compare declared and
observed identity before enabling actions and exporting qualification; mismatch holds.
The profile identity is distinct from the runner's existing execution-profile extension:
composition is not model/harness qualification. Carry both identities, do not repurpose
or replace `runner.ProfileRequirement` (decoded by `LaunchRequest.ExecutionProfile()`) or the current capability negotiation contract.

## First profile and exact proof boundary

Proposed name `cockpit-fake-runner`, version 1. Single operator, one synthetic job at a
time, loopback-only disposable environment on a supported development host. Initial
qualification uses the actual thin public client delivered by cell-api/04 and its
supported local host/test setup; a desktop Electron shell is outside this first
profile. Record actual OS/runtime at pickup; untested hosts remain unqualified. This is a bounded correctness
workload, not a throughput or latency promise.

| Component | Mode | Boundary and qualification |
|---|---|---|
| Public thin cockpit and generated client | real | Actual operator actions traverse the real public client and request validation; no frontend-only success fixture. |
| Existing service host and its profile routes | real | Real handlers, scoped local session, actual serialization and error mapping. |
| Shared supervisor and runner client | real | Delivered public /26 API plus existing /20 client; unchanged identity, dedupe, cancel and recovery rules. |
| Runner adapter | fake | Existing in-memory runner fake with explicit scenario controls and stable job identity. No model invocation or subprocess job. |
| Work inputs, actors and data | synthetic fixtures | Dedicated disposable work identity and local credentials; no real forge claims, GitHub writes or production credentials. |
| Workflow controller, real agent, Kubernetes and production forge | absent | Not qualified, not silently emulated, not required for this profile. |

Orchestration means the shared supervisor's launch lifecycle, not graph-execution/21's
future workflow controller. No acceptance claim about durable workflow recovery follows
from an in-memory fake. If the delivered /26 supervisor requires a coordination journal,
its real disposable PostgreSQL harness remains required: this plan does not invent a
no-journal supervisor or substitute its store. Missing database capability holds the
integrated profile, not the independent contract/fake work. Backend integration must prove selected profile resources stay
separate from operational stores and claims. The synthetic actor is admitted by the same issuer and admission machinery selected
and approved by cell-api/03 (R7 ingress, existing R3/R4 capability/executor custody).
That owner supplies the disposable caller credential for a profile-only audience and
run namespace; the profile host neither issues nor brokers credentials. Record the
selected issuer/configuration identity at implementation pickup. If that owner has no
supported disposable credential mechanism, hold this profile and obtain its scoped
owner amendment; do not add a second issuer. Ordinary-mode hosts reject profile-audience
credentials and profile routes. Independently, the lower store/claim/effect admission
rejects a synthetic principal targeting any non-profile cell, claim or store even with
ingress checks bypassed. No production no-auth mode or authorization bypass is added.

## Lifecycle, isolation and observation

The R7-selected existing service binary gains an explicit opt-in disposable profile mode
after cell-api/02 and graph-execution/26; ordinary
startup is unchanged. It validates the complete profile before creating resources or
starting children, then prints the selected profile, component modes and loopback targets.
Host adapters allocate an owned temporary root, scoped synthetic session, ephemeral
ports and process-group handles; reject symlink escapes, unowned output paths, remote
binds and ambient provider credentials. Every created resource has recorded ownership.
The launcher supplies an allowlisted child environment, not the user's whole environment.
For the separately authorized real-runner profile only, the driver supplies a credential
through the selected graph-execution/22 adapter's existing approved custody boundary,
scoped to the named local invocation. The host sees only an opaque credential reference;
it does not read, issue or broker the secret. Missing/invalid credential or unsupported
custody fails before launch, with no ambient/config-file fallback. The adapter's approved
secret delivery is the only permitted additional capability; /03's remote-target,
ordinary-mode, synthetic-principal and owned-resource restrictions remain in force.
Any provider contact needs its own explicit grant; a profile is no such grant. Test
credential-shaped sentinels, never real secrets, in stderr, provider errors and traces;
none may survive export, logs or retained failed-run residue.

Startup is all-or-cleanup: record partial resources, stop only owned children and report
what remains when cleanup fails. Reset creates a fresh run identity and clean fake state;
old events/results cannot enter it. Teardown is idempotent, scoped to owned resources,
and never deletes an arbitrary caller directory or kills an unrelated process. Killing
the launcher must have a tested cleanup/recovery path; retained failed-run evidence is
explicit and cannot keep a credential. Loopback binds alone are not containment: test
remote dialing and credential access at the actual host boundary with a faulting transport
and sentinel filesystem, including an upper-check bypass. No live production probe.

Both the UI and exported evidence identify simulated components. Missing profile metadata
shows unqualified scope and disables effectful profile actions; it never renders real.
A completed scripted job says completed in this profile, not deployed, merged or accepted.
Do not add implementation details to the normal product flow; the explicit demo/test
profile names its simulated runner because that changes how users interpret the result.

## Scenarios and acceptance

All checks below are proposed implementation deliverables. No scenario or test has been
run by this authoring change. A non-implementer reruns them at the exact artifact/profile
subject and uses existing Verify/Evidence and authorized milestone acceptance mechanisms.

| ID | Observable condition and evidence | Owner / contributing work |
|---|---|---|
| C01 | Unknown schema/component/mode/contract or digest mismatch refuses before startup; external Go consumer and public service adapter use the same pure validator. | independent verifier; /01, /03 |
| C02 | Same seeded input/reset yields the same logical events and declared result; success, definite failure, lost launch acknowledgement, delayed observation, cancellation requested/confirmed and cancel refusal remain distinct. No blind retry after unknown launch. | independent verifier; /02, /04 |
| C03 | Actual public thin cockpit → real host → real supervisor → fake adapter executes a synthetic job, observes the same identity/result, and cancels through the same path. Request/handler/adapter traces prove no layer was skipped. | independent verifier; /04–06 |
| C04 | UI, report and logs identify the fake and its limitations; missing/changed metadata or stale run evidence cannot become real-service acceptance. No mocked requests can satisfy a criterion requiring a real provider. | independent verifier and driver acceptance; /01, /05–06 |
| C05 | Partial startup, interrupted run, reset and repeated teardown leave no owned running child or writable credential; unowned sentinel files/processes survive. Fault controls cannot contact an external endpoint or use ambient credentials. | independent verifier; /03, /06 |
| C06 | Same contract/lifecycle cases run against the fake and each selected real replacement at pinned versions; unsupported semantics are reported, mismatches fail, and real substitution creates a new subject and scoped acceptance. | independent verifier; /02, /07; real adapter is external prerequisite |
| C07 | Evidence export binds source/artifact/profile/scenario digests, OS/runtime, component modes, actual exercised edges, named criteria/results, event trace, verifier, time, cleanup result and unknowns. No secret, fabricated provider telemetry or automatic acceptance stamp. Export normalizes local paths/hostnames/usernames to opaque references; keep private local artifacts outside the public bundle. | independent verifier; /06–07 |

The negative controls must plant wrong-but-plausible behaviour: fake labelled real,
ignored required mode/contract, UI-only synthetic completion, replay of a stale result,
unknown launch retried, cancellation request shown as stop, and cleanup deleting an
unowned path. Each must fail its named production-path test; shared-fixture agreement
alone is insufficient. Model quality, cost, real token usage, cluster admission and
production credentials are not measured by this profile and remain unqualified.

## Proposed milestone promises — no operational declarations

Owner: product integration. Acceptance authority: driver through existing scoped design
and acceptance decisions after independent verification; ownership alone grants none.
No automatic readiness or activation is introduced. These proposed records remain prose
while required milestone readers/receipt adapters lack qualified support.

| Promise / version | Supported outcome and exclusions | Direct contributors / criteria | Final acceptance needs |
|---|---|---|---|
| `assay:assay:delivery-profiles:m-01@1` | Reusable validated profile + deterministic fake; library/contract usable without cockpit or supervisor. No end-to-end claim. | /01, /02; C01, C02, relevant C06 fake cases | published compatible module or explicit source-qualified scope; independent evidence |
| `assay:assay:delivery-profiles:m-02@1` | Operator completes first real cockpit/supervisor journey with fake runner in the stated local profile; no real agent/workflow/cluster claim. | /03, /04, /05, /06; C01–05, C07 | qualified m-01@1; actual delivered /26 supervisor, cell-api/02–04 and public-module availability; driver acceptance |
| `assay:assay:delivery-profiles:m-03@1` | Same journey against an actual selected real runner in a separate disposable local subject. No production/cluster promise. | /07; C03, C04, C06, C07 rerun against real subject | qualified m-02@1; compatible delivered /22 adapter; separately authorized local invocation and acceptance |

Mapping shorthand /NN denotes full contributor
`assay:assay:delivery-profiles:b-NN`; future operational contributes entries use
those full IDs after negotiated reader support. Execution briefs retain supported
brief-v2 identities. Final milestone prerequisites do not become every contributor's
start dependency. No fake-profile acceptance satisfies a real-required consuming brief.
Changing supported scope or required provider semantics changes the milestone promise;
retain old evidence/version and explicitly qualify adoption. Keep the fake profile for
regression tests rather than deleting it when the real one arrives.

## Delivery ownership, compatibility and handoff

- /01–02 deliver shared contracts/libraries and fake/conformance changes in public Assay.
- /03–06 deliver lifecycle/adapter/UI/journey qualification in public Assay. Extend the
  selected cell-api/02 service implementation and /04 thin cockpit, not their historical
  source copy. Current `deskd/` is a compatibility placement, not proof of R7 completion.
  Checkpoint those owners; use their actual delivered paths at pickup. No second
  orchestrator, runner state machine, credential broker, database or claim store.
- /07 delivers real-replacement qualification. Extend the shared conformance owner only
  if /02's published contract is inadequate, through an owner-acknowledged change.
- The public author-spec, author-brief and shared milestone procedure receive a neutral
  guidance amendment in this authoring session. That amendment is not runtime support,
  scope approval or rollout. No installed plugin cache is edited.
- milestone-support/06–07 remain acceptance/readiness owners; /11–13 remain projection/report
  owners. Export scoped evidence for those existing adapters; no competing acceptance store.
- Private adopter composition, policy, source handoff crosswalks and operational records
  remain private and are not required inputs to generic profile qualification.

Rollback means stop the disposable profile and remove its owned resources; normal host
startup and existing user flows remain unchanged. A failed or partial cleanup is visible,
not a successful run. Published contract changes follow module/schema versioning; pin
release artifacts only through their existing publication/adoption path. /01 must verify
`GOWORK=off` public imports, including external consumer closure without a foreign internal import;
local replace proves source compatibility only. No local source is presented as a release.

Design alternatives: ad hoc mocks are quick but have no shared composition/provenance;
a universal mock service adds an unnecessary authority/deployment boundary; a second
orchestrator makes the demo easier while bypassing the capability we need to prove.
The selected mechanism reuses existing owners and adds bounded profile validation,
scenario and lifecycle glue. It retires duplicate selected-journey setup recipes, not
all existing test doubles. Actual change/complexity and startup-cost baselines must be
recorded during implementation; no measured reduction or performance claim is made now.

The lifecycle adapter belongs in `tools/desk/internal/cellapi/deliveryprofile/` under
the delivered R7 host, with its host option in `tools/desk/cmd/deskd/`. These are planned
placements reconciled with cell-api/02 before effects. No adapter imports another
module's internal package. The current standalone `deskd/` compatibility module is an
inventory source, not a second profile implementation target. Delegate subprocess
lifetime to the delivered `loopadmin/process`/supervisor API, never a new exec owner.

## Revision following design review — 30 October 2026

Review of the initial draft identified missing named synthetic-credential ownership,
real-runner custody/redaction proof and detached Verify rows. The amendment above and
briefs /03, /06 and /07 preserve existing issuer/admission and custody owners and add
explicit ordinary-mode, lower-boundary isolation and real-output negative controls.
No permission or operational gate is relaxed. Original authoring baseline remains
recorded; current main was merged at `5fdae0d07` for this revision. Existing R7 and
supervisor Tasks, Verify and Evidence are unchanged; only their inverse edges differ.

## Decisions and remaining gates

D1 driver: approve this bounded design/admission, request corrections, or hold.
Recommendation: approve for parked-to-active admission through a separate recorded ruling;
this draft does not make that transition. Real-supervisor availability is not assumed.
D2 at real-replacement pickup: select the delivered adapter, supported host and explicitly
permitted local invocation scope; absent selection keeps /07 held without blocking otherwise eligible /01–06.
No production tokens, external provider calls, cluster access, workflow activation, tag,
installed pin, ready flip, merge or actual milestone acceptance is authorized by authorship.

A material source change requires architecture pickup with source/consumer revisions,
coverage and continue/adapt/hold. Existing in-flight work and completed Evidence survive;
this draft is not a veto over approved work. All implementation tests are future work.

## Authoring validation record

See [the source-bound authoring validation](authoring-validation.md) for observed
checks and the publication-runner investigation. It supplies no implementation,
independent verification or milestone acceptance evidence.
