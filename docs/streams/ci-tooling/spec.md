# CI tooling, shared mechanisms and source migration

**Status:** draft
**Date:** 2026-10-13
**Owner:** Assay product architecture; implementation stream `ci-tooling`.

## Purpose and scope

Separate desk clients, independently operated services and CI programs by consumer and
lifecycle. Give repeated mechanisms one implementation without making CI depend on the
cell service. Move reusable implementations from adopter repositories into public Assay;
keep private policy, identifiers, fixtures and private-only implementations in the adopter.
The [architecture extension](../../proposals/non-ci-architecture/24-ci-tooling.md) connects
this work to the product domains. This specification owns the CI migration contract.

This is authoring, not activation: the stream is parked pending revision-bound approval.
No directory, executable, import, workflow, permission or publication disposition changes
merely because this document lands. Each implementation brief preserves a working path.

## Observed baseline and inherited decisions

Inspected public source: `4696d598e`, 2026-10-13. `ci/staged-workflows/` already exists,
including `declared-changes.json`; `.github/workflows/` remains the executable workflow
location. Standalone tools and the `tools/desk` module coexist; `commsgw` is currently under
`tools/desk/cmd` and imports its internal packages. It cannot simply move outside that
module and keep those imports. The private migration source is supplied by each adopter;
public execution and qualification must not depend on access to it.

Existing owners remain binding: `statusgen/evaluation` owns eligibility and evidence
coverage (S-eligibility, S-evidence-coverage in [contracts](../../contracts.md)); `forgeread`
owns its narrow fact/completeness contract. [Library-first](../../library-first.md)
restricts offline libraries and online authority separately. This work neither republishes
`deskkit` wholesale nor silently replaces the statusgen online read boundary.

## Directory and module contract

| Home | Responsibility | Migration rule |
|---|---|---|
| `desk/cmd/` | Role-facing commands and clients | Move callers after their internal dependencies have an appropriate owner; retain role checks. |
| `operator/cmd/` | Installation, cell lifecycle, recovery | Keep operator capability separate from desk clients. |
| `services/<service>/` | Processes with independent lifecycle/custody, including communications | Gateway/consumer/runtime packages move as a tested unit; folders do not pool credentials or create new daemons. |
| `ci/cmd/<tool>/` | Thin standalone check commands | Preserve supported names/flags/results through the measured transition. |
| `ci/checks/<check>/` | Reusable check-specific logic | Explicit inputs; no ambient forge credentials or hidden provider reads. |
| `ci/report/`, `ci/invocation/` | CI presentation and command binding | Import shared result types; do not redefine acceptance. |
| `ci/actions/<action>/` | Reusable composite action entry points | Checked-in `action.yml`; external users pin the repository commit. |
| `ci/staged-workflows/` | Reviewable workflow candidates and drift declarations | One canonical staging location; activation still uses the authorized promotion path. |
| `.github/workflows/` | Active workflow triggers, jobs and permissions | Remains here because the provider requires it; delegates repeated steps to actions. |
| `core/checkresult/`, `core/repofiles/` | Narrow mechanisms shared beyond CI | Create only with named consumers and explicit contracts; no catch-all utilities package. |
| existing domain modules | Existing semantic owners | Reuse them; directory cleanup does not invent competing owners. |

Two library layers are deliberate:

1. **Assay-wide mechanisms:** typed check observations and provenance, deterministic file
   selection/read accounting, plus existing domain and forge-read APIs. `core/checkresult`
   has no filesystem, process, network, clock or environment effects. `core/repofiles` is
   a separate effectful package over explicit roots/filesystems; it cannot choose policy.
2. **CI-wide mechanisms:** check implementations, CI-specific rendering, invocation and
   action contracts. CI depends on Assay-wide packages; the reverse dependency is forbidden.
   Desk or service callers may use a pure check package without importing an action runner.

Use `core/go.mod` and `ci/go.mod` for the new externally consumed library surfaces, with
module paths `github.com/medici-finance/assay/core` and `/ci`. These module boundaries are
justified by adopter wrappers importing them; a directory alone does not require another
module. Keep other existing module boundaries until their own consumers are migrated.
No repository-root module/workspace is introduced by this stream.

Release qualification must resolve both modules from outside this checkout, with
`GOWORK=off`, no consumer `replace`, and no private source. Existing umbrella releases
must publish resolvable module versions as well as binaries. If the current tag layout
does not resolve a submodule at the advertised version, stop the consumer cutover until
the release owner supplies a working module version; never substitute a placeholder
pseudo-version. Local-proxy tests prove packaging, not a published release.

## Check results, policy and authority

A result carries a schema version, check/version, source revision or content digest,
policy digest, coverage (examined, excluded, unavailable with reasons), findings and an
explicit outcome: checked-clean, checked-failed or could-not-check. A partial scan retains
its definite findings as well as incomplete coverage. Unknown schemas and truncated
required inputs cannot become clean. Retain detailed distinctions in domain payloads.

The CLI/action applies **exit policy separately**: advisory may exit zero on an incomplete
check, but its structured result and summary still say could-not-check. Blocking policy
fails on findings or incomplete required coverage. Input selection and exclusions remain
explicit: tracked files, working-tree files and artifact directories are not interchangeable.
Preserve glob semantics, symlink handling and file/byte bounds with contract fixtures.

CI supplies evidence to the existing acceptance owner; it cannot satisfy human approval,
change policy from the candidate branch, mint role credentials or publish acceptance.
Offline file checks work without PostgreSQL, cell services or forge access. Online checks
require a separately supplied authorized adapter, explicit repository/host scope and the
existing CI token profile. No generic credential getter or write API enters core packages.
Private consumers may import public code; public packages never import private consumers.

## First vertical slice and factoring limits

Start with claims checking and issue-reference checking. `claimsguard` checks literal
prohibited claims against a supplied manifest; `closecheck` evaluates referenced issues
against supplied observations. Public `tools/claimguard` already checks uncited product
names and is a different tool. Preserve both executable names initially, document the
singular/plural distinction and use distinct packages `checks/claims` and
`checks/citations`. Any later rename needs its own caller migration.

Extract `checks/claims` so a release-check caller can use a typed result instead of
launching a checker and parsing its stdout solely to cross a Go module boundary. Retain
a process boundary where it actually provides isolation or independent deployment; do
not delete a security check because it resembles repeated code. A command adapter stays
for shell consumers. `checks/closure` takes a resolver interface; the existing `forgeread`
open-issue list is not a substitute for PR-body and individual-issue lookup semantics.
Extend its API only with a separately reviewed compatible contract and named consumers.

Results/reporting, explicit file selection and process adapters are factoring candidates.
Rule-specific matching, private vocabularies, acceptance policy and independent controls
are not collapsed. Report net production and test changes after adding the libraries;
file moves and moved comments are not savings. No percentage reduction is an acceptance
gate; consumers removed and semantics preserved are the primary evidence.

## Four stages, with working transitions

### 1. Establish layout and staging rules

Create the directory ownership documentation and module skeletons needed by the first
slice. Inventory every executable, package, action, workflow reader, build/release entry,
consumer pin and open work item affected by a path. Assign desk/operator/service/CI/private
or explicit retention, with an owner and retirement condition. This inventory extends
beyond Go: shell/Python guards, workflow helpers, bootstrap scripts and hooks count too.

Public staging already uses `ci/staged-workflows/`. Migrate any adopter's legacy staging
path with its promoter, path restrictions, drift manifests, tests, instruction files,
skills and all path filters. Never run a blanket textual replacement over dated Evidence.
Keep a bounded dual-read transition only if needed: new writes use the canonical path;
same workflow in both roots is refused unless bytes match; every alias has a removal step.
A normal move is not permission to activate workflows or widen workflow-write authority.

### 2. Libraries first, existing commands adopt them in place

Publish narrow Assay-wide and CI-wide libraries with offline positive/negative fixtures.
Cut over the existing command locations to those released versions before moving paths.
The first consumers are the claims command, release-check composition and closure command;
add a second check consumer for each proposed shared mechanism before generalizing it.
Do not make an all-tools rewrite a prerequisite to the pilot.

Private-only wrappers, fixtures, policy and source stay private. Their eventual local home
is an adopter-owned private CI directory, with an adopter-owned module path, never the public `/ci` module identity.
Private commands must have distinct install names when co-installed (for example an
adopter prefix); public actions never discover or execute a private binary through PATH.

### 3. Move entry points and rewire consumers

Move the qualified pilot to `ci/cmd/`; update build targets, release assets, examples,
cross-module readers, source triggers, staged workflow candidates and compatibility tests.
Do not claim the live workflow changed until promotion is observed. Qualify the new binary
release before a consumer pin changes; prove consumer pin adoption before deleting its old
source. If a public copy exists already, select one canonical implementation and prove
parity/divergence disposition rather than publishing another copy.

Private adapters move to an adopter-owned private CI directory in private PRs. Pending-publication tools remain
there or at their recorded old location until independent scrub, publication approval,
public build, release and consumer parity pass. A generic-looking tool is not automatically
publishable. Private detector fixtures and publication policy never accompany a move.

Desk and service moves share the directory contract, but their custody/module extraction
is a separate bounded lane under the runtime migration owner. The initial inventory must
route every such move to an existing or newly authored brief before declaring the overall
`tools/` split complete. This CI stream delivers the CI slice, not an unmeasured rewrite
of every desk command or service. Service moves must preserve restart, queue durability,
role separation and installer behavior; no new activation is implied.

### 4. Reusable actions and workflow adoption

Design the action contract with the libraries. Implement and test it **before or alongside
stage 3's workflow conversion**; roll it out broadly after the pilot passes. The deployment
order is library release → binary release → action qualification/pin → workflow promotion
→ observed consumer run → source retirement. Stage numbers group outcomes, not a reason
to strand a workflow while its action does not exist.

Initial actions: `ci/actions/install-tools`, `ci/actions/claims-check`,
`ci/actions/closure-check`. Keep check entrypoints fixed; no input is an arbitrary shell
command. Installation requires an explicit release and platform checksum, verifies before
execution, refuses missing/mismatched pins and reports the selected version. There is no
floating latest or fallback to a source build. Check actions accept explicit target root,
policy/observation paths and advisory/blocking mode where supported; produce result path,
outcome and summary. Structured findings are escaped before Actions logging.

External callers use `medici-finance/assay/ci/actions/<action>@<commit-sha>` with an actual
reviewed SHA at implementation. Action source is resolved from `github.action_path`;
inputs point at the consumer checkout. A local `./ci/actions/...` reference is appropriate
only in Assay's own checked-out tree. Consumer token permissions remain in workflow jobs;
actions request no broader credential and do not use privileged events to execute an
untrusted candidate. Pin action dependencies too. Active triggers/concurrency/permissions
and required-check names must survive the conversion unchanged unless explicitly approved.

## Migration evidence and rollback

Each inventory row records old/new source and module paths, executable/asset identity,
consumers, policy owner, publication disposition, pin versions, activation state and a
retirement condition. Public records omit private locations and identifiers; each adopter
owns its private counterpart. Independently prove public staging from a scrubbed export,
including tests and fixtures, rather than testing only the richer private checkout.

Preserve three distinct milestones: staged, released, adopted. Retire source only when a
published version contains the qualified implementation, each named consumer uses it,
relevant old/new corpus results agree or have approved deltas, and no unmerged fix is lost.
Compatibility shims are bounded with named consumers and a removal condition; no silent
fallback to the old source. Roll back consumer binary/action pins and workflow promotion
to the last tested combination. Never reintroduce a private implementation into public
source as a rollback mechanism. Retain the old release until the adoption window closes.

## Qualification and decisions

Before implementation, approve this revision and the directory/library contracts; keep
separate publication and workflow-promotion acts at their existing boundaries. The parked
stream has executable Verify contracts for future deliverables; none is marked delivered.

Qualification must prove: external module resolution; forbidden imports absent; missing
observations and empty coverage not clean; CLI/library/action parity; malicious annotation
text escaped; checksum mismatch refused before execution; same input returns the same
findings; old staging ambiguity refused; unchanged required checks still run after a move;
private consumers work with public pins without private code becoming a public dependency.

Consumer-specific adoption and the broader desk/service move inventory remain explicit
obligations, not hidden prerequisites to initial library development. Update existing
contracts/library-first records when a new owner is implemented; do not add duplicate
semantic owners merely because a directory was created.
