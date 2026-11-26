---
brief: assay:assay:milestone-support:08
title: "Normalize PR trailers and claims through the shared resolver"
why: "Different spellings must not acquire two claims, attach a PR to the wrong work, or make an authoring PR look delivered."
wave: 3
depends: ["milestone-support/01", "milestone-support/02", "milestone-support/04"]
unblocks: ["milestone-support/09", "milestone-support/10", "milestone-support/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: bef0746f-0619-4b99-bf3a-fd53ce31a1d2
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes PR/claim-to-work binding; an alias collision or lost repository scope could associate a foreign owner or authorization with the wrong work."
decision-trigger: spec
design: DR-milestone-support-08
decision-issue: 5704
ruling: "https://github.com/medici-finance/assay/issues/5704#issuecomment-6715665207"
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "tools/desk/internal/deskkit/trailer.go: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "tools/desk/internal/deskkit/briefauthoring.go: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "tools/desk/internal/deskkit/claimstore.go: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "gitcore/claimref.go: out-of-scope semantic parsing; opaque payload transport and exact-old CAS retained, same-payload readers tested"
  - "tools/desk/cmd/deskpr/: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "tools/desk/cmd/deskfile/: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "tools/desk/cmd/deskclaim-ref/: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "tools/desk/internal/loopengine/: fixed-here: selected identity/subject adapter and input admission; operational activation remains /10 and /14"
  - "docs/streams/milestone-support/brief-09-preserve-workflow-subjects-and-human-rulings-across-aliases.md: follow-up milestone-support/09 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-11-project-milestone-state-into-stream-views-and-deskboard.md: follow-up milestone-support/11 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 08 — Normalize PR trailers and claims through the shared resolver

## Context

files: `tools/desk/internal/deskkit/trailer.go`, `tools/desk/internal/deskkit/briefauthoring.go`, `tools/desk/internal/deskkit/claimstore.go`, `gitcore/claimref.go`, `tools/desk/cmd/deskpr/`, `tools/desk/cmd/deskfile/`, `tools/desk/cmd/deskclaim-ref/`, `tools/desk/internal/loopengine/`, `changelog/milestone-support-08.md` (planned), `statusgen/testdata/milestone-support/08-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: desk-tools identity adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "desk-tools identity adapters"
  contract: S-claim
  retires: ["duplicated interpretation or implicit state inference at the named existing consumer boundary"]
  weight: "New binaries 0; new services 0; required pure API/schema/fixture changes only. Measure actual rule/flag/refusal deltas at implementation review; no current weight reduction is claimed."
  why-add: "Reuse the existing semantic owner and public module. This records a missing shared meaning; moving it into a helper process or copying another parser would add authority or drift without benefit."

single-point-of-failure: incorrect shared identity or capability interpretation; retain independent current-subject, authority and consumer checks and plant faults with the outer check bypassed where Verify names that boundary.

## Read first

- [Implementation scope and consumer census](README.md#implementation-scope).
- [Milestone contract](../../../spec/milestone-v1.md).
- [Brief contract and compatibility](../../../spec/brief-v1.md).
- [Stream contract](../../../spec/stream-v1.md).
- [Library-first direction](../../library-first.md).
- [Existing semantic owners](../../contracts.md).
- [Dependency waves and adoption holds](README.md).

## Ground rules

- Own isolated branch and draft implementation PR. Do not merge, flip status/spec/stream/PR readiness, dispatch work, cut a tag, change installed pins or contact a cluster.
- Scope approval and stream activation take effect on the maintainer's merge of PR #4182; this brief has no completion evidence. A non-implementer owns post-merge verification; never fill Evidence from authorship or a fixture expectation.
- Refresh main, active claims/PRs, module imports, source contracts and consumer inventory at pickup. Record brief/source revisions, actual public API availability, coverage/unknowns and continue/adapt/hold in the implementation handoff. No obsolete active claim is overwritten.
- Preserve existing credentials, independent human/risk gates, UUIDs, archived evidence and exact-subject meanings. No ambient-auth fallback or automatic bulk migration.
- Shared resolution and capability assessment MUST be direct library calls, not subprocess calls to statusgen, desk-tools or a new helper executable. External I/O remains in existing adapters.

## Human decision

Before changing the governed boundary, present the concrete implementation/design through the existing decision mechanism. Confirm the preserved actor, subject, revocation and effect-admission rules. Options: accept the bounded implementation; request corrections; hold. No answer supplies no approval. Do not infer this ruling from the planning request.

Default if no answer: none — blocks until answered

### Bounded design and recorded ruling

**Design approved, implementation outstanding.** The driver accepted the proposal at
`5e8f7424a78027d9988801182e9ac8a82ba4977f` through the
[unedited human ruling](https://github.com/medici-finance/assay/issues/5704#issuecomment-6715665207).
The approval requires duplicate or note-injected subject tokens to be rejected, and
spellings refused by the strict resolver never to be admitted through legacy
normalization. [DR-milestone-support-08](../decisions/DR-milestone-support-08.md)
records this bounded approval. The original Task and Verify rows below remain the
implementation contract; no implementation, lifecycle advance or Evidence credit is
claimed here. The proposal text below records what was reviewed; its earlier
pending-ruling statements are superseded only by this exact approval and conditions.

#### Census correction after the ruling (Task 1 pickup obligations)

Review of the ruled proposal found that the consumer census below is incomplete as a
census of direct calls. The ruled design text is left unedited. The callers below
are recorded here as Task 1 obligations: each needs an explicit fixed-here, follow-up
or out-of-scope disposition before any shared trailer, authoring or claim-payload
helper changes. This note changes no step, case or decision.

Direct callers missing from the table, at the ruled source and still present at
current main:

- `tools/desk/cmd/deskdispatch/phantom.go` (`crossRepoRepresents`, `ParseTrailers`):
  the cross-repo duplicate-dispatch reader. It compares the trailer value with
  `strings.EqualFold`, so it is also a case-fold site for the strict-spelling
  safeguard.
- `tools/desk/cmd/deskpr/titleprefix.go` (`ParseTrailers`, `SplitCrossRepoBriefTrailer`,
  and the package's `splitBriefTrailer` wrapper), `tools/desk/cmd/deskpr/edit.go`
  (`trailerLink`, `ParseTrailers`) and `tools/desk/cmd/deskpr/decisiongatebanner.go`
  (`withDecisionGateBanner`, `ParseTrailers` and `splitBriefTrailer`).
- `tools/desk/internal/deskkit/apptrailergate.go` and `tools/desk/internal/deskkit/autolane.go`
  (`ParseTrailers`, used for trailer presence and kind only).
- `tools/desk/internal/deskkit/pendingchangebatch.go` and `tools/desk/cmd/verifyloop/pending.go`
  (`CompleteChangedFilesOf`, the changed-file completeness helper).

Claim-payload readers the same-payload compatibility tests (step 4) must cover:

- `tools/desk/cmd/deskclaim-ref/claim.go` (`fieldOf`, first match per key) and
  `tools/desk/cmd/deskclaim-ref/alias.go`.
- `tools/desk/cmd/desksupervise/live.go`: a positional parse of the first five message
  fields, plus `state=`/`age=`/`owner=`/`branch=` field patterns over `show` output.
- `tools/desk/cmd/deskdispatch/dispatch.go` (`claimStateFieldRe`, `claimAgeFieldRe` over
  `show` output).
- The bash wire peer, dispatch-claim.sh, that `claimMessage` in `claim.go` names as
  sharing the message grammar. It is not in this repository, so its compatibility
  remains could-not-check here and must be established at pickup.

How this list was produced: `git grep -n -E` for direct calls of
`SplitBriefTrailer`, `CanonicalBriefID`, `SplitAuthorsTrailer`, `SplitCrossRepoBriefTrailer`,
`ParseTrailers`, `ResolveWorkTrailer`, `BriefAuthoringOnly`, `CompleteChangedFiles`,
`CompleteChangedFilesOf` and `RepresentedBriefs` under `tools/desk`, excluding test files
and the unrelated `testledger` function of the same name; then a search for
`owner=`/`state=`/`branch=` message readers and `fieldOf` callers under `tools/`. Every other
non-test Go hit is already a file in the table. Transitive callers still need the refresh
the table's intro requires.

Two path and owner notes. The table's closing line roots desk paths at `tools/desk/`;
`gitcore/claimref.go` is the exception, the repository-root `gitcore` module (the
frontmatter `consumers:` entry now names it there). Shared `workref` interpretation is
owned by semantic-owner row S-context-handoff in `docs/contracts.md`, which leaves
claims/trailer activation to this brief. Both S-claim and S-context-handoff therefore bind
the implementation.

#### Architecture pickup

Manual bootstrap at source commit `5142af8c66f4671d7cb3c575d5b65d5eadd6275b`:

- Controlling sources: this brief (version 1, permanent ID unchanged), the stream's
  approved implementation scope, `spec/brief-v1.md` §3.8,
  `spec/milestone-v1.md` §2/§6/§10, `spec/stream-v1.md`,
  `spec/reader-capabilities-v1.md`, `docs/library-first.md`,
  `docs/go-engineering.md`, and semantic-owner row S-claim in `docs/contracts.md`.
  All references in this assessment are to that source commit. The large brief and
  semantic-owner documents were inspected in the affected sections, not audited
  in their entirety.
- Observed prerequisite code: `statusgen/workref` exposes `NewRegistry`, `Parse`
  and `Resolve`, including `TypedBrief` and distinct legacy profiles;
  `statusgen/readercaps.Assess` assesses explicit operation requirements.
  `tools/desk/go.mod` already requires statusgen with a source-tree replacement.
  These APIs are available in the actual consumer module; a published module or
  installed capability is not established by that replacement.
- The board records /01 done, /02 implemented and /04 verified. /02 has not thereby
  acquired independent completion credit. The old Context baseline describes the
  original authoring snapshot; its absent-package statements are historical.
- Observed seams: `deskkit.ResolveWorkTrailer` already calls `TypedBrief`, but
  operational trailer helpers still select legacy reduction. Durable claims use
  `ClaimStoreRecord.Version` for CAS and an owner/state/branch message;
  `tools/desk/cmd/deskclaim-ref/alias.go` recognizes only its existing per-PR claim family.
  `deskdispatch.claimKeyFor` separately derives brief transport keys.
  `loopengine.Claim` uses a separate local, flock-backed primitive.
- Source tests establish the intended cases to retain, not results of this pickup:
  `TestWorkrefGrammarAndAliases` covers root/kind/key/revision separation;
  the claim reader and alias tests cover existing transport and lane contracts.
  No new churn, complexity or performance measurement was made. S-claim records
  existing namespace/release duplication; this proposal does not claim to repair it.
- Coverage: local source, manifests, contracts and the direct helper caller census.
  Current forge decisions, open changes, active claims and error-class history
  could not be checked because authenticated forge access was unavailable.
  Recheck those facts, file overlap and owner acknowledgments before implementation
  or publication. No decision-issue number or ruling is inferred from this gap.

Disposition: **continue** bounded design authoring; **hold** governed production
changes until the recorded human ruling and current overlap/decision checks.
Reuse the existing public packages and S-claim owner. No dependency installation,
new module, process boundary, claim store or credential owner is proposed.

#### Proposed flow and preserved boundaries

1. At the named desk consumer seams, collect the actual declaring cell/root,
   supplied graph aliases and existing UUID/rename bindings through their current
   host adapters. Call `workref.NewRegistry` and `Resolve(..., TypedBrief)` in
   process. Keep the returned canonical identity, stable logical key, original
   spelling, compatibility fullId and requested revision together. Do not first
   strip `b-`, `m-`, qualifiers or selectors. A root/UUID collision refuses;
   unavailable registry or source observations hold as could-not-check.
2. Bind a separate selected subject containing the existing brief UUID, exact
   authored version, owning repository, source commit and repository-relative
   record path. An explicit selector must match that version at the selected
   source; an unversioned reference binds the supplied source snapshot. Missing
   exact revisions remain unknown. Never use a revision, path or claim CAS token
   as logical work identity, and never treat a resolver fallback key as proof
   that an executable record exists or has a permanent UUID.
3. Adapt that resolved brief to the existing repo-scoped transport key shape
   `<repo-label>--<stream>--<NN>`, derived from its canonical compatibility identity
   and the existing repository-label owner. Keep this physical key distinct from
   the UUID/logical key. Ordinary legacy keys retain their bytes. Resolve supplied
   historical aliases to that same work before deciding whether it is held.
   If an existing live alias uses a different physical key, report its holder and
   hold: normalization itself never renames, removes, releases or takes it over.
   Multiple live aliases, incomplete enumeration or ambiguous label configuration
   also hold. A selector never creates another physical key.
4. Persist the selected subject with a newly acquired negotiated claim, in the
   same stored payload and therefore under the same CAS version as its holder.
   Proposed encoding: retain the existing first-line message and append one
   versioned `subject-v1=` token containing base64url-encoded JSON with the subject
   fields from step 2. Use the standard JSON/base64 codecs, a closed bounded
   decoder and compatibility tests against the actual old readers. Do not put
   subject data in `note`, a second ref or a sidecar store. Unknown/malformed
   required subject metadata holds. An existing payload without metadata remains
   readable as held; it cannot satisfy a pinned request or be silently upgraded.
   Conflicting pinned subjects hold without calling UpdateFrom, Remove or steal.
5. Retain the current claim-store resolver, role/credential admission, holder
   checks, branch liveness, TTL policy and server CAS. Progress must preserve the
   admitted subject and use the exact store version read. The dispatch generation
   (`dispatch_ref`) also remains separate: aliases do not mint a new generation,
   revive an old one or transfer custody. Explicit release/steal operations retain
   their existing admission semantics; merely resolving a reference invokes neither.
   The new adapter reads no ambient credential and adds no auth fallback.
6. Preserve link kinds before resolving their values. `Brief:` represents delivery;
   `Authors:` represents authoring and cannot represent delivery. Retain trailer
   multiplicity/fence rules, changed-file completeness, authoring classification,
   risk checks and cross-repository tracking checks. A milestone refuses at every
   executable-brief/claim/trailer boundary, including when the outer check is
   bypassed. Only after qualified root/kind checks may a legacy API receive the
   compatibility fullId; pass its subject separately rather than erase the selector.
7. Reuse operation-scoped reader assessment before new semantics can cause effects.
   Syntax support alone never grants dispatch/claim admission. Keep operational
   advertisements and rollout held for /10 and /14 qualification. An older writer
   that cannot participate in subject/alias admission is not safe to co-dispatch
   negotiated work. Mixed-writer support must be established, or the new operation
   holds; a scan of aliases alone is not an atomic cross-version exclusion proof.
   Local loopengine claims must not be described as cross-machine exclusion.

The shared result is interpretation, not a grant. Existing trusted actor and
responsible-principal checks remain at effect admission. This slice does not
authenticate a ruling, alter its subject or carry approval across a revision.
Revoked, superseded, edited, wrong-actor or wrong-subject rulings cannot be made
valid by an alias; /09 owns their existing-record binding and /10 owns dispatch
eligibility. Their independent lower-boundary checks remain required.

#### Consumer census and routing

This is a pickup census of direct calls to the trailer/authoring helpers and the
named claim seams at the pinned source. The dispositions describe proposed work,
not code already fixed. Implementation must refresh transitive callers before
changing a shared helper; do not widen a legacy signature in place and assume
its callers have acquired context.

| Consumer | Proposed disposition |
|---|---|
| `tools/desk/internal/deskkit/trailer.go`, `briefauthoring.go`, `prrepr.go` | fixed-here after ruling: shared typed identity at negotiated seams; preserve legacy profiles, link kind and complete-file classification. |
| `tools/desk/cmd/deskpr/deskpr.go`, `crossrepotrailer.go` | fixed-here after ruling: validate owning root, record and exact subject before write; retain Authors/delivery and tracking-repo checks. |
| `tools/desk/cmd/deskfile/bound.go` | fixed-here after ruling: replace argument/frontmatter/trailer qualifier comparisons with shared resolution while preserving the open-PR, full diff and tracking-repo checks. |
| `tools/desk/internal/deskkit/claimstore.go`, `tools/desk/cmd/deskclaim-ref/{args,claim,alias,gogit}.go`, `gitcore/claimref.go` | fixed-here at identity/payload adapters after ruling; storage selection, transport custody and CAS mechanics stay with their current owners. |
| `tools/desk/cmd/deskdispatch/dispatch.go` (`claimKeyFor`), `tools/desk/internal/loopengine/{claim,probes,engine}.go` | fixed-here at brief identity/key inputs after ruling; keep local and forge claim scopes distinct and retain work-evidence/author checks. |
| `tools/desk/cmd/deskdispatch/authoring.go`, `tools/desk/cmd/fanoutloop/{represented,represented_live}.go` | fixed-here after ruling: qualified linkage/reconciliation inputs; preserve completeness and historical authoring exemptions. |
| `tools/desk/internal/deskkit/{briefrisk,stampage}.go`, `tools/desk/cmd/deskflip/flip.go` | fixed-here for affected reference adapters after ruling; independent security, current-head and flip authority remain unchanged. |
| `tools/desk/internal/deskkit/repairobligation.go`, `tools/desk/cmd/verifyloop/repair.go` | fixed-here only if adapting the shared reference input is necessary; desk-tools/49 retains filing identity/deduplication. Do not reimplement it. |
| Queue classification and verify planning | out-of-scope: desk-tools/45 owns these meanings; preserve their inputs and existing classification. |
| Workflow/domain/ruling readers; dispatch/pickup eligibility; generated views | follow-up milestone-support/09, /10 and /11 respectively; no acceptance, readiness or projection credit from this slice. |
| Historical claim namespaces, non-brief issue claims and per-PR review/security lanes | out-of-scope for migration; preserve their existing separation and admission. Do not reinterpret them by splitting arbitrary `--` keys. |

All desk consumer paths in the table are rooted at `tools/desk/`; grouped names share the preceding directory.

#### Acceptance examples for implementation review

These are proposed cases for the two existing Verify targets and their mutation
manifest, not additional executed checks or replacement Verify rows.

| Case | Required observation |
|---|---|
| All six specified legacy forms, canonical/elided `b-` forms and the two permitted draft slash aliases for the same source | Same canonical root/UUID and physical claim key. Subject/selector remains separate; no second worker. |
| Same stream/number in two different owning roots | Remain distinct; supplying another root never binds a local brief just because its filename matches. |
| Unknown root, duplicate UUID alias, `m-` input, malformed/zero/overflow selector or unavailable exact version | Refusal or explicit could-not-check as appropriate, with zero claim/PR/filing effects; never prefix-strip into a brief. |
| `Authors:` versus `Brief:`, fenced sample, duplicate trailers, incomplete changed-file list and cross-repo alias | Existing distinction, completeness and tracking checks survive through deskpr and reconciliation; an authoring proposal cannot suppress delivery dispatch. |
| Existing claim at an old physical alias; two aliases acquired concurrently; missing/partial enumeration | No overwrite, migration or second admitted worker. Exercise pre/post race handling with a fake transport and show why mixed unsupported writers hold. |
| Same key requested at version 2 then version 3, or pinned request meets an old payload with no subject | Existing holder/version/generation unchanged; conflicting or unknown subject holds without a write. |
| Claim advanced or reassigned between read and update | Exact-old CAS loses; alias equality cannot restore the old owner, subject or generation. |
| Unsupported/stale capability observation, wrong actor/subject, revoked ruling | Existing effect boundary still holds with the upper resolver/preflight bypassed; library support is never approval. Ruling integration remains /09. |

After ruling, produce fail-first runs and the declared mutation map before making
test-based implementation claims. Plant a second caller that erases root/kind or
puts the selector in the claim key; the production-boundary test must turn red
naming that repeat. Preserve old reader output and actual fake-transport entrypoint
coverage, not just resolver-unit-test coverage.

#### Exact decision requested

The human driver is asked to record one ruling on the existing decision issue,
bound to this proposal's reviewed commit:

1. **Accept the bounded implementation** described above, including the separate
   logical/physical/subject identities and versioned same-payload subject extension,
   while preserving actor, exact-subject, revocation and effect-admission rules.
2. **Request corrections**, naming the steps/cases that must change before approval.
3. **Hold**, leaving production behavior unchanged.

Recommendation: accept only after the current claim/PR overlap check and the
same-payload compatibility cases are judged adequate; otherwise request the
specific correction. Replacing every existing claim key with a UUID or moving
normalization into a new helper process are rejected alternatives: the former
would split live custody during migration, and the latter adds no required
authority or isolation. Silence selects none of these options.

## Task

1. Route all brief linkage and logical claim-key normalization through workref, preserving cell/repository ownership and UUID. Retire parallel grammar fragments in PR trailers, work-item/claim aliases and issue-bound brief lookups; inventory every caller with an explicit disposition.
2. Read all specified legacy forms and new b- references with optional revision selectors. Persist the selected subject/revision separately from the logical claim key. Mixed aliases must join the same existing claim; conflicting revision requirements hold, never create a second worker or overwrite a live claim.
3. Retain each role's existing custody boundary. Preserve Brief versus Authors semantics, ownership/generation checks and existing issue/PR alias behavior. Keep the no-ambient-auth rule. Milestones are not executable brief claims or delivery trailers. Migration must not close, steal, release or reassign real claims.
4. Do not reimplement desk-tools/49 repair-obligation deduplication or desk-tools/45 queue classification. Extend the currently available identity adapters only; reconcile simultaneous edits with those owners at pickup and add an explicit dependency if a not-yet-delivered API is truly required.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedTrailerIdentityJoin$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedTrailerIdentityJoin ' "$milestone_verify_out")` | exit 0 and named PASS; old/new forms join the same work, Authors remains authoring, wrong root/kind/version refuses; stripping the prefix before validating kind fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedClaimAliases$' ./cmd/deskclaim-ref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedClaimAliases ' "$milestone_verify_out")` | exit 0 and named PASS; real claim adapter with fake transport treats aliases as one owner/key, refuses conflicting pinned subjects and preserves generation; a duplicate logical claim fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

Verifier pass, 2026-11-18, by a non-implementer (model claude-opus-5-5, tier strong) at merged main 1999f670d0be3708b99cfade6f013318bbf89980. This is the first verification of this brief. Delivering change: #5915, merge commit 7d119c295ffd26e069c4fedee55171437ec9d2f3, first parent b6f1c33ace097cae4c361bb9646d5cdf04f16ad2 (implementation commit a0346c885501676f69667d5ee265ccb1f6a0acc2 and four follow-up commits on the same branch). Between that merge and this source, 13 of the files the delivery changed were changed again by later merges. Seven are production Go files: tools/desk/cmd/deskdispatch/dispatch.go (12 later merges, among them #6899, #6871 and #6813), tools/desk/cmd/deskdispatch/phantom.go (#6727, #6752), tools/desk/cmd/deskpr/deskpr.go (#6766, #6702, #6684, #6512), tools/desk/cmd/deskclaim-ref/args.go (#6740, #6716), tools/desk/cmd/deskfile/bound.go (#5480), tools/desk/internal/deskkit/prrepr.go (#6722) and tools/desk/internal/deskkit/worksource.go (#6621, and #6683, merge commit 584e77343853077230e7ab9d0265787ec1774d44, which reads the work source through gitcore and fails closed on an unmerged config scope). The other six are test and doc files. Both rows ran against this source, which includes all of those later changes. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run e42fbaab3cdc, attestation #6934, tool v1.0.87, runtime darwin-arm64). The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base 1999f670d0be).

Rows 1-2 (check:ci) ran in one closed container bracket, with the fail-first base run in the same container. Image: verify-runner v1.0.87, named by its manifest-list digest (Container-Image below), run as its linux/arm64 platform image with go 1.27.4, git 2.56.1 and statusgen v1.0.87, on the host's local Docker engine with --network none, --read-only, the engine's default seccomp profile, the host uid, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly and a fresh GOCACHE in container tmpfs. The home, the receipt, the exported history pack and the Go module cache (provisioned by the open step; Module-Cache below) were mounted read-only; only the witness output directory and tmpfs were writable, both outside the home. No forge credential and no Git configuration variable entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. The close step re-admitted the run and --bracket-digest reports verdict=match.

Fail-first. The brief's risk line answers sensitive-data: yes, so both rows are risk-bearing and each owes a fail-first witness. The base is the delivering merge's first parent, b6f1c33ace09 (--fail-first --delivering-commit 7d119c295ffd, baseline checked out from the exported history inside the container). Both rows are red at base (rc=1) and pass at head, so both discriminate. The retained base logs are empty for the same reason the head output is: the command writes go test output to a temporary file. A supplemental host observation, with no acceptance credit: at b6f1c33ace09 neither TestPrefixedTrailerIdentityJoin nor TestPrefixedClaimAliases exists (both test files were added by a0346c885501), so the named-PASS grep cannot match at base.

Row S1 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest (statusgen/testdata/milestone-support/08-mutations.json, 23 cases) in the same image with --network none against a tracked-file snapshot of this source outside the home. It covers the Expect cells' named mutations: prefix/kind erasure before validation (trailer-kind-erasure) for row 1, and a duplicate logical claim key (second-caller-selector, physical-key-selector) for row 2.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | python3 statusgen/testdata/milestone-support/08-mutate.py, against a snapshot of 1999f670d0be | every manifest plant turns its named test red | exit 0; 23 of 23 manifest cases printed CAUGHT (2 against TestPrefixedTrailerIdentityJoin, 3 against TestPrefixedClaimAliases, 18 against 12 further named tests); every named baseline printed BASELINE PASS first; no case missed | 2026-11-18 | assay-verifier-app[bot] @ 1999f670d0be (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief answers sensitive-data yes and irreversible no. Enumerated over every non-test Go file in the diff of b6f1c33ace09..7d119c295ffd, plus tools/desk/internal/deskkit/trailer.go, which that diff does not change but the resolver calls (tools/desk/internal/deskkit/workselection.go, worksource.go, trailer.go, briefauthoring.go, prrepr.go; tools/desk/cmd/deskclaim-ref/args.go, claim.go, workclaim.go; tools/desk/cmd/deskdispatch/dispatch.go, phantom.go; tools/desk/cmd/deskfile/bound.go; tools/desk/cmd/deskpr/deskpr.go; tools/desk/internal/loopengine/claim.go), read at this source. The files were read as they stand at this source, so the later merges named above are in what was read, but the enumeration targets the values the delivery introduced; values those later merges introduced are outside its scope. Literals found: the subject UUID pattern (workselection.go:26), the subject commit pattern (:27), the work label pattern (:28), the Version == 0 refusal and the path character set "\\ \t\r\n:" (:31, :34), the wire token "subject-v1=" (:153), the five-field header position (:175, :197, :276), the admitted states "state=claimed" and "state=dispatched" (:284), the decoded subject bound 2048 (:212), the brief path shape "docs/streams/" / "brief-" / ".md" at depth 1 (worksource.go:96) and the single-origin binding len(urls) != 1 (worksource.go:51). No value is irreversible; each is undone by an edit and a release, though a wire-format change would leave already-written claim payloads to be read under the old spelling. The label, path-shape and state literals are existing repository spellings and rank last. Ranked top entries:

- RISK-VALUE: DERIVED — subjectToken = "subject-v1=" @ tools/desk/internal/deskkit/workselection.go:153 — the brief's ruled design (Proposed flow step 4) names exactly one versioned subject-v1= token carrying base64url JSON, and docs/streams/milestone-support/08-consumers.md:41 records the same token; the encoder, decoder and the old-reader tests agree.
- RISK-VALUE: DERIVED — subject token position = 5 (fields[:5], i != 5) @ tools/desk/internal/deskkit/workselection.go:175 and :197 — claimMessage builds the historical header as exactly five fields, "dispatch-claim <id> owner= state= branch=" (tools/desk/cmd/deskclaim-ref/claim.go:133), and step 4 requires the token appended after that unchanged first-line message and before any note; any other position is a misplaced or note-injected token, which the ruling requires to refuse.
- RISK-VALUE: DERIVED — subjectCommit = "^[0-9a-f]{40}$" @ tools/desk/internal/deskkit/workselection.go:27 — the subject binds the full source commit (step 2); this repository uses SHA-1 object names, whose full lowercase hex form is 40 characters. An abbreviated or SHA-256 name is refused, which holds rather than binding the wrong subject.
- RISK-VALUE: DERIVED — owning-origin binding len(urls) != 1 @ tools/desk/internal/deskkit/worksource.go:51 — the owning repository must be unambiguous (step 1: an unavailable or ambiguous source observation holds as could-not-check); zero or several origin URLs give no single owning repository, so 1 is the only admissible count.
- RISK-VALUE: NAMED, NOT DERIVED — decoded subject bound = 2048 @ tools/desk/internal/deskkit/workselection.go:212 — step 4 requires a closed, bounded decoder but neither the brief nor 08-consumers.md names the bound. The fixed fields of a maximal valid subject (36-character UUID, a 20-digit version, a slug of at most about 140 characters, a 40-character commit and the JSON keys) take about 300 bytes, so the bound leaves roughly 1.7 KB for the record path; a longer path refuses (holds) instead of binding. The margin is plausible, but no source states the intended maximum path length, so the number itself is not derived.

This brief is gate: human with sensitive-data: yes. Its Evidence rows are recorded here and its status is left where it is; the human gate decides the transition.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedTrailerIdentityJoin$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedTrailerIdentityJoin ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 1999f670d0be (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=b6f1c33ace09 red rc=1 · head=1999f670d0be · delivering=7d119c295ffd output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000001.log |
| 2 | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedClaimAliases$' ./cmd/deskclaim-ref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedClaimAliases ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 1999f670d0be (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=b6f1c33ace09 red rc=1 · head=1999f670d0be · delivering=7d119c295ffd output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000002.log |
Tree-Digest: sha256:b2d44c3661389b9a0b99e04e45a2e67744bf46032cda9c6e0e588a8af75ed612
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6934 run=e42fbaab3cdcbcbf9a4464288cedb1ee751d4bcdc2f958190bff21993d5ad47e source=1999f670d0be3708b99cfade6f013318bbf89980 brief=docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e42fbaab3cdcbcbf9a4464288cedb1ee751d4bcdc2f958190bff21993d5ad47e source=1999f670d0be3708b99cfade6f013318bbf89980 tree=sha256:b2d44c3661389b9a0b99e04e45a2e67744bf46032cda9c6e0e588a8af75ed612 witness=sha256:33fec2f96c85253cf7edc82bd2136d3818a1d5316a5f8800d3ff5617679eecc3 opened=2026-11-18T16:35:12Z closed=2026-11-18T16:40:03Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass at 1999f670d0be in one closed container-netns bracket (verdict=match), each red at base b6f1c33ace09 and passing at head (fail-first discriminating); supplemental mutation manifest 23/23 caught; gate: human with sensitive-data yes, so the status is left for the human gate.

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
