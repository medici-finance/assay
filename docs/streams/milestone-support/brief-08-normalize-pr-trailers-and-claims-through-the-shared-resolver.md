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
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "tools/desk/internal/deskkit/trailer.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/briefauthoring.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/claimstore.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/gitcore/claimref.go: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskpr/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskfile/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskclaim-ref/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/loopengine/: follow-up milestone-support/08 (this delivery; replace with fixed-here in implementation)"
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

### Bounded design for the pending ruling

**Proposal only, inspected 7 November 2026.** This section prepares the decision
above. It does not approve a design, implement reference support, advance this
brief, or supply Verify/Evidence credit. The original Task and Verify rows below
remain the implementation contract. Record accept, corrections or hold through the
[existing decision issue](https://github.com/medici-finance/assay/issues/5704); acceptance must identify the reviewed proposal revision.

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

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
