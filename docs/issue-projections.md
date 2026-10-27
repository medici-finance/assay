# Issue discovery projections

The issue reader selects a population from the complete typed open-issue listing
before paying trust, comment and label-history reads. Collection stays in the
existing role-bound forge adapter; pure selection lives in
`deskcore/issueflow` through the internal compatibility facade. No cache, service, new credential or pool is added.

| Caller | Producer | Population and required enrichment |
|---|---|---|
| Worker skill and dispatch runbook | `issueboard issues --projection work-ready` | Unrepresented candidates without excluded/decision/addressed/help-wanted labels. Selected external authors still need current blessing and numeric identity; selected work still needs complete label history. Missing active placeholders retain positive state reads to detect a partial listing. A history hold is AWAIT, never dispatchable. |
| Intake skill | `issueboard issues --projection intake` | All open issues and active closed placeholders, with original trust/quarantine, escalation, addressed-response, label-history and positive closure behavior. `issueboard intake` separately reads local raw intake. |
| Addressed desk inbox | `issueboard issues --projection inbox --to ROLE` | Only the named addressee, selected before trust and comment enrichment; retains complete escalation histories and conservative overflow. Role aliases remain the existing addressee owner's responsibility. |
| Human decision inbox script | `deskinbox discovery REPOS...` | Case-insensitive, space-trimmed union of urgent, needs-decision, question and help wanted, ordered urgency then creation age. This is distinct from the addressed-role population. Selection precedes detail; the script retains its existing detail, positive ruling screen, cards, flow and partial-result exit semantics. |

Projection output declares population, current invocation cutoff, enumeration
completeness and row-level history degradation. It is an observation set, not an
atomic provider snapshot, and has no validity beyond the collecting invocation.
An empty worker result cannot establish intake/inbox idle. Excluded external issues
are absent from the worker projection; full/intake still show their quarantine. Failed complete-list
reads fail the whole issueboard projection; existing row-level history overflow
still degrades conservatively. The human inbox preserves partial output plus exit 2.
Neither observation grants write, claim, admission or executor authority.

## Compatibility and rollback

| Plugin/script and producer | Supported path |
|---|---|
| Older plugin with new producer | Legacy full/default/`issues`/`intake`/`--to` forms remain available, with unchanged tables and exit behavior. |
| New plugin with matching producer | Check `issueboard --version` for `issue-projection-v1` before selecting projected forms. New source is qualified together in this PR; a released pin must contain this capability. |
| New plugin with older producer | Use the documented legacy spellings before discovery, preserving manual readiness checks. Do not send an unsupported selector or retry after a partial projected read. |
| Inbox script with compatible producer | With a nonempty desk-loop identity, auto mode checks `deskinbox --version` for `issue-discovery-v1` before any discovery. It runs exactly one typed listing per requested repository; no legacy label queries run in that arm. `ASSAY_DESKINBOX` selects a pinned executable. |
| Human terminal without a desk-loop identity | Auto refuses with could-not-check before either typed or ambient discovery. Choose explicit `ASSAY_INBOX_READER=legacy` for an ambient `gh` read. Explicit typed mode still requires the canonical App role. |
| Inbox script with absent/older producer | With a nonempty desk-loop identity, auto selects legacy before discovery. `ASSAY_INBOX_READER=typed` instead refuses before any discovery; use it to qualify the migrated arm without broad reads. |
| Explicit rollback | `ASSAY_INBOX_READER=legacy` restores shell label discovery. Restore the previous plugin and producer pins together for worker/intake. No live pin, installation or executor switch is performed here. |

Typed discovery reads with the session’s minted App identity; the shell’s existing
per-item detail and trust-screen reads use the operator’s ambient `gh` identity.
Those identities can see different repositories: failures stay partial, and the
operator can select `ASSAY_INBOX_READER=legacy` before discovery. Typed stderr
retains effective-configuration and coverage diagnostics as well as read failures.

Typed rows must name a requested repository and carry the documented field types
before they can drive detail reads. The existing trust screen still judges every
row; discovery supplies no author or ruling authority.

Never fall back after a compatible producer starts: permission errors, malformed
output and page-cap overflow remain incomplete observations. The existing typed
reader's bounded serial fallback is used; no unsupported batching API is invented.
Its per-process caps do not coordinate independent processes or guarantee fleet quota.

## Remaining broad readers and acceptance holds

The default full board, `tools/desk/README.md` operator recipe and independently
selected legacy compatibility paths remain supported broad readers. Deskboard's
intake-depth note correctly names the separate local `issueboard intake` read;
dispatch/stalled/health remain their own board populations and are not switched to
a narrow worker result. Independent watcher health, all trust/edit checks, audit,
budget/breaker/admission controls and consequential rechecks remain required.

Both actual mains use the shared `PerfRunMeasured` recorder. The real parser
selects immutable context before effects; compiled operation names keep private
argv out of measurement labels. Explicit invocation-owned observers reach the
existing native forge constructors. No ambient measurement setter is added.

With local telemetry consent and a selected profile, these callers emit the
canonical perf-v3 schema from desktools-v2/63–65. An unselected profile keeps the
legacy recorder path; consent off creates no record or optional cost selection.
Qualified GraphQL cost uses the same provider operation and its actual scalar
metadata, never a shared quota gauge or an extra request. Provider/child intervals
are not added to process wall time.

Issueboard's own native request inventory is covered. Inbox discovery and table
are native list-only modes with complete own-request coverage. Walk and HTML
observe native list/body reads but retain the legacy raw comment reader: its
pagination differs from the typed comment API, so changing it here would change
the existing bound. Those modes explicitly retain missing request coverage rather
than reporting raw comment traffic as measured zero. Credential-minter and other
subprocess work remains separately owned; parent measurements cannot prove full
child coverage or fleet cost.

Local caller flows exercise the actual native transport, writer, mixed history
reader and reducer, including consent off, per-invocation context isolation,
metadata failure, response parity and the raw-comment gap. Shared dependencies
remain subject to their own review and human acceptance. The separate
desk-tools/38 verification prerequisite is retained: source tests here do not
certify it or authorize runtime activation, releases, installations or pin changes.

Flow-only shell modes (`--flow`, including `--html`) read existing flow emitters and perform no issue discovery, producer capability probe or issue query. They therefore need no loop identity or reader selection. Shell discovery modes return 6 before any read when auto has no loop identity; select `ASSAY_INBOX_READER=legacy` explicitly for ambient discovery.


## Repository-scoped reader setup

Each issueboard collection resolves one authenticated, wrapped reader for each
repository it visits. Full, work-ready, intake and addressed-inbox views all use
that same collection path. The reader owns only the existing read operations;
selection and classification remain with their existing policy owners.

The reader shares client and credential setup within that repository's current
invocation. It does not cache issue observations, cross repository boundaries,
keep a process-global client or reuse success in a later invocation. Required trust and label-history reads retain their original operations and
order through the original wrapped forge. Positive closure reads retain their
existing typed batch contract. Selected trusted escalation-comment threads may
share the bounded read described below; every thread retains its own history. Outbound checks,
transport errors, pagination caps, quarantine and row-level degradation retain
their existing behavior. An expired or rejected credential stays a read failure;
there is no alternate identity or transparent retry path.

This follows the already-delivered desk-tools/42 projections. The older full
forms and the compatibility/rollback matrix above remain applicable. No selector,
capability, credential owner, service, runtime store, release or consumer pin is
added. Deskinbox and statusgen's separate online deskread bridge remain unchanged;
the pure issueflow selection contract is unchanged for future library consumers.

`TestCollectionScopedReader` checks repository setup counts, ordered provider
reads and fresh collection across two actual invocations of all four views.
`TestCollectionResolutionFailureIsFresh` rejects reuse after a later resolution
failure. Their fixture also runs against the immutable pre-change source; the
setup-count assertion fails there. Existing projection, trust, identity,
escalation, closure and actual-recorder tests remain in place.

The requested perf-v4 before/after qualification is pending the canonical codec
and producer. The baseline source is
`116a2f1494fca6c97209f10883861467f729216d`; that source has perf-v3 integration
but no perf-v4 codec or producer. The merged perf-v4 transition design is not a
shipped producer. Resolver-count regressions are not perf-v4 measurements or a
production latency/CPU/quota savings estimate. No v3 record is relabeled, no
second recorder is introduced, and this source qualification grants no new
collection consent or operational activation.


The matched synthetic fixture has fourteen repositories, one populated. Per
invocation, the pre-change and candidate setup counts are:

| View | Resolutions before | Resolutions after | Provider operations, both |
|---|---:|---:|---:|
| Full | 22 | 14 | 22 |
| Work-ready | 18 | 14 | 18 |
| Intake | 22 | 14 | 22 |
| Addressed inbox | 15 | 14 | 15 |

Each variant runs twice and repeats its ordered provider operations. These counts
include empty repositories and depend on the fixture's population. They do not
measure elapsed time, process startup, token-mint subprocesses, provider latency,
GraphQL points or production savings. Credential resolution may itself reuse an
existing custody token; fewer resolutions do not imply fewer minted credentials.

## Shared addressed policy and handoff observations

Ordinary statusgen scanning now calls the same public candidate owner as the
reader projections. An intentional `to:<role>` stamp stays in the receiving
inbox; it creates or reactivates no ordinary scan placeholder. Existing
projections are retained without manufacturing closure or resolution. Conflicting
or empty addressees are explicitly held out of ordinary scan/work-ready selection.
Historical role spelling remains case-insensitive and independent of today's
roster; current write-side roster and trust gates are unchanged. Full/intake
remain the supported broad views, including their legacy classifications.

The worker's addressed-board ordering also uses this owner. This extraction
does not make the label-less STATUS.md board into a live issue inbox, change
legacy foreign-addressed board ordering, or arm successor discovery: that is
verification-handoff/04. The versioned companion assessment is described in
[verification-handoff-v1](../spec/verification-handoff-v1.md); ordinary bugs remain
discoverable without a mandatory `to:intake` stamp.

## Positive state reads for absent placeholders

The full/intake collection still positively reads every absent active
placeholder before proposing retirement. Listing absence is never closure.
For an explicitly capable wrapped GitHub reader, up to twenty issue numbers
share one fixed, read-only GraphQL document. Every alias must return its exact
requested number, a known open/closed state and a present title. Missing/null
aliases, changed identities, unknown states, transport errors and any GraphQL
error beside otherwise valid data fail the sweep. No partial retirement table
is published and a failed batch never retries through another identity/path.

The credential-free positive-state adapter lives in `forgeread/adapters/github`;
the desk composition supplies only its typed issue-state read through the
original authenticated transport. Repository admission, outbound decorators,
the CI read-only wrapper and invocation-owned request observers remain in place.
Unknown decorators and other providers retain their original `GetIssue` reads;
one remaining issue retains the original REST read. Duplicate local placeholder
identities retain separate serial reads rather than borrowing one observation. No observation survives the
repository's current invocation. A chunk may read up to twenty independent
positive states before an open issue is detected; no later chunk is read after
that refusal. This read batching grants no write, admission or lifecycle power.

Trust, comment and label histories keep their original independent read plans,
numeric actor/edit checks, completeness rules and hard bounds. Earlier projection
selection and repository-scoped client reuse remain unchanged. The state adapter
does not move authenticated adapters or credentials into statusgen; its online
read boundary remains `deskread`, with pure/offline contracts linked in-process.

`TestPositiveStateBatchNativeParity` exercises the actual wrapped native
transport, including the CI read-only wrapper, across zero, one, twenty-one and
ninety-five absent placeholders. It preserves exact output, re-observes the next
invocation and rejects null/missing/changed/errored state data without a fallback
or partial table. `TestPositiveIssueStatesScopeAndObserver` verifies admitted
repository refusal before sends and native request-observer parity. Shared
adapter tests cover bounded batches, positive identity and envelope failures.

The canonical desk `positive-state-mutations.json` and shared adapter
`issuestates-mutations.json` harnesses each have a distinct control, with seven
mutants in total for envelope errors, identity, batch bounds, request regression,
repository scope, unknown wrapper guards and live-issue misclassification. It is local qualification;
it does not claim enforcement by an unmodified CI truth-suite matrix.

## Bounded escalation-comment reads

Issueboard plans comment reads only for already-trusted authors whose selected
rows require an escalation clock. On the explicitly capable GitHub composition,
up to five independent issue threads share one fixed, read-only GraphQL document.
Each alias binds the requested issue number and retains its own cursor, original
100-comment page selection and twenty-page bound. A completed alias is removed
from subsequent pages. A missing cursor or history still open at the cap leaves
only that row incomplete, with the original conservative escalation behavior.

The response envelope is decoded incrementally by alias and reduced with the
same comment-to-event function as the serial reader. Five original bounded
histories are retained at most; no aggregate byte limit rejects a combination
of individually valid legacy histories. Missing, null, repeated or mismatched
aliases, missing page provenance, GraphQL errors even alongside data, truncated
JSON, trailing JSON and transport cancellation refuse the sweep without fallback.
The existing request/credential/observer path owns every send and same-operation
optional cost observation; no parallel client or retry policy is introduced.
Malformed, truncated or trailing wire data retains failed cost coverage. A
complete valid query response may retain its actual scalar cost even when its
business observation is refused; no clean metadata envelope hides a wire failure.

Untrusted authors retain their original independent trust-then-history sequence.
A singleton, duplicate listing identity, unknown decorator or other provider
keeps its serial reads. Results belong only to this repository's collection and
are consumed in issue order, never reused in another invocation. Full, intake,
work-ready and inbox still select their original populations. REST label history,
its deleted-label holds, pagination and order are unchanged. This introduces no
recorder migration, profile/consent change, release, installation or consumer pin.

The issueboard history-batch tests plant both default and addressed-inbox callers;
the native adapter controls cover per-thread pagination, near-limit histories,
row-local overflow, identity/provenance faults, unknown wrappers and actual
request/cost observation. The canonical local mutation specification is
`tools/desk/cmd/issueboard/history-mutations.json`; it is not CI-matrix enforcement.
