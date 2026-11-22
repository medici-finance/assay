---
brief: assay:assay:milestone-support:09
title: "Preserve workflow subjects and human rulings across aliases"
why: "Reference compatibility must preserve exact work/evidence subjects and must never transfer a human ruling to a different revision or brief."
wave: 4
depends: ["milestone-support/05", "milestone-support/08"]
unblocks: ["milestone-support/10", "milestone-support/12"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: a25c8c4e-7b7b-4c59-8259-92b8f3f65469
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes how human rulings and verification evidence bind aliases and revisions; the bridge must never transfer a ruling or accepted evidence to another subject."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/contexthandoff/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/domain/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/decisionruling.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/decisiongatehold.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/rulinggate.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/briefschemagate.go: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskevidence/: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "spec/context-handoff-v1.md: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "spec/verification-domain-v1.md: follow-up milestone-support/09 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md: follow-up milestone-support/12 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 09 — Preserve workflow subjects and human rulings across aliases

## Context

files: `statusgen/contexthandoff/`, `statusgen/domain/`, `statusgen/decisionruling.go`, `statusgen/decisiongatehold.go`, `tools/desk/internal/deskkit/rulinggate.go`, `tools/desk/internal/deskkit/briefschemagate.go`, `tools/desk/cmd/deskevidence/`, `spec/context-handoff-v1.md`, `spec/verification-domain-v1.md`, `changelog/milestone-support-09.md` (planned), `statusgen/testdata/milestone-support/09-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: existing handoff/domain and ruling adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "existing handoff/domain and ruling adapters"
  contract: S-verification-domain
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

## Task

1. Introduce version-aware edge adapters around existing context-handoff, verification-domain and human-gate readers. Resolve the new typed reference with workref, then pass existing numeric fullId plus an independently retained subject/version binding to legacy APIs. Do not globally broaden their existing strict grammar or reinterpret old digests.
2. Bind ruling-link board IDs and aliases to the same exact logical brief and approved subject using existing human identity, issue/link and scope checks. Refuse missing/changed/mismatched subjects even if an upper parser or capability check is bypassed. Agent-authored metadata cannot create a human ruling.
3. Preserve archived evidence, withdrawal context, source revisions, workflow/verification identities and immutable acceptance digests. Current modules contain no workflow store/controller: cover existing records/adapters now and document that future workflow owners must use the same bridge, without depending on or inventing that absent implementation.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedHandoffSubject$' ./contexthandoff/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedHandoffSubject ' "$milestone_verify_out")` | exit 0 and named PASS; old records retain their digests/meaning, new adapter binds exact revision and wrong-subject inputs fail even with upper checks bypassed. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRulingAliasAuthority$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestRulingAliasAuthority ' "$milestone_verify_out")` | exit 0 and named PASS; real ruling gate accepts only the same human/issue/work subject across aliases; copied ruling, wrong revision/root and bot actor fail. |
| 3 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedVerifySubject$' ./domain/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedVerifySubject ' "$milestone_verify_out")` | exit 0 and named PASS; normalized legacy identity alone cannot erase the requested revision; stale or substituted candidate evidence fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.

## Proposed alias and ruling-subject bridge — pending human ruling

**Proposal only. No implementation, Task/Verify change, Evidence or lifecycle credit is
claimed.** This brief is human-gated, and its decision issue is open with no ruling yet.
This appendix sets out the concrete design that the Human decision section asks for, so
the maintainer can accept it, correct it or hold it. Until an unedited human ruling
names this proposal, nothing here changes a shared value, a record digest, a reader
capability or the meaning of an existing ruling.

### Architecture pickup and source coverage

Manual bootstrap at source commit `ae32b8d3e1e321221527ee4ff7ab8bd6376c3886`, brief
`assay:assay:milestone-support:09`, authored revision 1. Both dependencies are delivered
on main: /05 is verified, and /08 is implemented. The brief's authoring baseline said
public `statusgen/workref` did not exist; it does now, and this design consumes it. The
table lists the source contracts that were inspected. None of them is a new approval.

| Source | What the design relies on |
|---|---|
| `statusgen/workref` (`Parse`, `Resolve`, `Registry`, `Result`) | The one shared reference owner. A typed reference resolves to an `Identity` (kind, cell, repo, stream, `b-`/`m-` number), an optional exact positive `Revision` and a `LogicalKey` (the bound key or UUID, else the canonical typed id). `Compatibility()` returns the old unprefixed brief `fullId`, and returns nothing for a milestone. `NewRegistry` refuses alias collisions and kind changes. Legacy profiles keep their own acceptance sets and carry no revision. |
| `statusgen/contexthandoff/ids.go` (`ResolveQualifiedID`) | The strict four-segment `fullId` grammar of `spec/context-handoff-v1.md` §2, reached through `workref` with the `LegacyQualified` profile and an empty registry. Records are digested in that form (§6). History kinds are evidence, source-account, decision-account and source-identity (§3.5). |
| `statusgen/domain/records.go` (`NewWork`, `Candidate`, `BindAcceptance`) | `WorkIdentity` is built from `ResolveQualifiedID`. An `AcceptanceBinding` carries version, digest, candidate and required facts, and is refused on a candidate mismatch. Refusals are `checked-failed` or `could-not-check`, and record digests are sha256 over canonical JSON. |
| `statusgen/decisionruling.go` (`resolveRulingOn`) | The one network resolver for a ruling link. It requires: the link parses; it is in the same repository; the comment sits on the named issue; it is unedited; the author is a personal account mapped as a human; the comment names the record; and the issue rules on that record. |
| `statusgen/decisiongatehold.go` (`offlineRuling`, `briefIssueRulesOn`, `gateSameBriefPairs`) | Layer one checks frontmatter offline. Layer two resolves the ruling link. A brief matches its decision-gate marker by board id, by verbatim `brief:` value, or by a colon form whose stream and number match and whose repository part names the repo or a registry alias. Identity is one-to-one: only the same board id, or an exact one-to-one renumber, keeps a ruling. A present typed gate card carries `brief` and `version` but today only `brief` is compared. |
| `tools/desk/internal/deskkit/rulinggate.go`, `signoff.go` and the `deskclose`/`deskmerge` authority files | The register sign-off reader (granted, unsigned or could-not-read), followed by a fetched comment whose author must be a mapped human, never a bot or App. These gates authorize register rulings (`R-N`). They hold no brief subject today. |
| `tools/desk/internal/deskkit/briefschemagate.go` (`ReaderCapabilities`) | Advertises only `syntax` and `preflight`. Prefixed-reference support is not advertised. |
| `tools/desk/cmd/deskevidence/decisiongate.go` | An Evidence landing runs `statusgen --decision-gate` before and after staging the write and refuses only the REFUSED lines the write adds. A statusgen too old for the flag is could-not-check. |

Import direction fixes the boundary. `tools/desk` may import public statusgen packages
(it already imports `statusgen/workref`), and statusgen cannot import
`tools/desk/internal`. The shared comparison therefore lives in `statusgen/workref`, and
each module keeps its own independent check around it.

### Bounded design to rule on

The design adds one pure value to the existing reference owner, plus thin adapters at
the existing boundaries. It reuses the typed reference grammar and adds one ruling
comment field. It adds no store, binary or subprocess seam.

1. **One subject value.** `statusgen/workref` gains `Subject{LogicalKey, Revision}`,
   built only from a resolved `Result`, and one comparison, `SameSubject(approved,
   presented)`. They are the same only when both logical keys are equal and both
   revisions are equal. Revision zero means "no revision was supplied". It never equals
   a positive revision and is never filled in from the current brief. A milestone subject
   is never equal to a brief subject. The value fetches nothing and decides nothing
   beyond the comparison. A caller compares two subjects only after building both under
   the same revision regime: both sides carry a revision, or neither does. Each caller
   below names how it builds its two sides, so the strict rule never meets a revisioned
   side and an unrevisioned side by accident. The one place the two regimes meet on
   purpose is point 3's `could-not-check` case.
2. **Handoff edge adapter.** `contexthandoff` gains `ResolveWorkSubject(ref, declaring,
   registry)`. It resolves `ref` with the `TypedBrief` profile, gets the `QID` by passing
   `Compatibility()` to the unchanged `ResolveQualifiedID`, and returns that `QID` with
   the retained `Subject`. A milestone reference is refused, because it has no `fullId`
   alias. The legacy grammar is not broadened: `ResolveQualifiedID` still refuses a
   prefixed or revisioned spelling. No handoff record field is added, so every old and
   new record keeps its canonical form and digest. Persisting a revision inside a handoff
   record would be a schema version change, and is out of scope.
3. **Verification edge adapter.** `domain` gains `NewWorkAt`, which resolves through
   `ResolveWorkSubject` and then calls the unchanged `NewWork` with the compatibility id.
   It returns the existing `WorkIdentity` together with the `Subject`. A new
   `CheckSubject(approved, presented Subject)` is refused with `checked-failed`
   `work-subject` when the two differ. It is `could-not-check` `work-subject` when the
   approved side has a revision and the presented side has none, so a normalized legacy
   id can never erase a requested revision. The approved side is not read from any
   record, because points 2 and 3 persist no revision. At Evidence time it is rebuilt
   from the brief itself: the logical key resolved from the brief's board id, and the
   revision taken from the brief's `version:` frontmatter at the judged base, the same
   commit the decision-gate hold judges. The presented side is the reference the
   Evidence names, resolved through `ResolveWorkSubject`. A brief with no `version:`
   yields approved revision zero, so only an unrevisioned presented reference matches
   it. The check runs only on a landing whose Verify row or Evidence names the work by a
   typed reference. A landing whose references are all legacy unprefixed ids keeps
   today's path unchanged, as point 7 requires. It runs alongside the existing
   `AcceptanceBinding.Validate(candidate)` and replaces nothing: a stale or substituted
   candidate is still refused by the candidate check, even when the subject matches.
4. **Ruling subject in statusgen.** The decision-gate hold and ruling-link lane keep
   the existing same-repository, issue, human-account, unedited-comment and
   record-binding checks. Every board id, permanent id, `brief:` value and marker
   spelling must resolve to one logical key using the registry at the judged base.
   Resolve prefixed or revisioned markers through `workref`; refuse a different cell,
   root or work key even when stream and number match.

   For a revision-bound ruling, the existing comment-binding hook reads one explicit
   line from the unedited human ruling comment:

   ```text
   Ruling-Subject: assay:assay:milestone-support:b-09@1 card=none
   ```

   Resolve the comment's typed brief reference with `workref`. Its key and positive
   revision are the ruled side; the judged brief's resolved key and explicit
   `version:` are the presented side. Require equality. Missing `version:` stays
   missing, even if the card writer emits version 1. Duplicate, malformed or
   conflicting subject lines refuse. Neither the issue body nor frontmatter can
   supply a missing ruled revision.

   The same line records the card the human saw. `card=none` requires a successful
   issue read with the matching marker and no typed card. If a typed card is present,
   use `card=sha256:<hex>` over its exact UTF-8 card block bytes, including the
   delimiters and excluding surrounding issue text. The decision prompt must show
   the card and the digest together. Require one well-formed card whose digest,
   resolved key and version all match the comment. A missing, changed or unexpected
   card, malformed card data or unreadable source is `could-not-check` and holds
   the ruling. Edits to surrounding issue text do not affect the digest and still
   have to satisfy the existing marker check.

   Only an unedited comment with no subject line, created strictly before the
   bridge's fixed cutover and validated by the existing legacy checks, can keep
   logical-key-only meaning. The cutover is a fixed implementation rollout value,
   recorded when the bridge is enabled and shared by both checks. Read the comment's
   creation time from the forge, never from issue text or the caller's clock.
   An absent or unreadable timestamp holds. A typed subject line selects the
   revision-bound path even for an older comment; a broken or missing card can
   never switch that comment to legacy. At and after cutover every new ruling
   requires the subject line, including on marker-only issues.

   Layer one validates local identity and explicit version inputs offline. It
   cannot establish which revision a human ruled on. Layer two uses the fetched
   ruling comment and current issue to establish that binding before a governed
   transition can pass.

5. **Ruling subject in tools/desk.** `deskkit` gains a pure `RulingSubjectGate`
   that takes the fetched comment and issue observations, the resolved brief
   context and registry, and the expected issue number. It checks the comment's
   issue, edit timestamps, actor type and existing human mapping, then independently
   applies point 4's subject, card and cutover rules. The revision-bound comparison
   uses the human comment and the explicit brief version. The historical comparison
   uses revision zero on both sides only after the legacy eligibility checks pass.
   The two modules reuse the same pure subject and card-binding rules in the public
   reference layer; neither accepts the other module's approval boolean in place of
   source observations. Preserve distinct reasons for issue, actor, subject and
   unavailable card evidence. `deskevidence` calls the in-process gate alongside
   its existing `statusgen --decision-gate` run. Register gates (`R-N`) are unchanged.
6. **Capability stays fail-closed.** No reader capability is advertised here. Where a
   consumer meets a prefixed or revisioned reference but the reader has not advertised
   prefixed references, the result is `could-not-check`. It is never silently reduced to
   the legacy id. Advertisement and dispatch admission stay with /10.
7. **History is preserved.** Archived Evidence rows, withdrawal context, source
   revisions, work, instance, node and attempt identities, and acceptance digests are
   never rewritten, renumbered or re-digested. A legacy record is read under its
   original meaning. The bridge only adds a comparison when a new subject is presented.
8. **Future workflow owner.** Main now has the PostgreSQL persistence foundation in
   `workflow/store`. It has no workflow execution controller and is outside this
   bridge. The two canonical specs gain one obligation:
   a future workflow owner must bind work, evidence and rulings through `Subject` and
   `SameSubject`, and must not add its own alias reduction.

### Preserved actor, subject, revocation and effect-admission rules

The Human decision section asks for these four rules to be confirmed explicitly.

- **Actor.** Only an unedited comment by a personal account mapped as a human on the
  named decision issue can rule. A bot, an App, an unmapped login and agent-authored
  metadata (frontmatter, a gate card, a DR record or a relayed comment) never create a
  ruling. They can only point at one that is then checked. Both modules check this
  independently.
- **Subject.** A revision-bound ruling covers the logical key and declared version
  in the unedited human comment. An alias carries the ruling only when it resolves
  to the same key and revision. The declared version is author-maintained; it is not
  a content digest. Changes to governed text require a version bump and a fresh
  ruling. First adding an explicit version introduces a revision, even when a
  previous card displayed a default of 1. Historical unrevisioned decisions retain
  their original logical-key meaning under the limited compatibility rule in point 4.
- **Revocation.** An edited or deleted ruling comment no longer stands. An issue
  that no longer identifies the record, or a card observation that no longer
  agrees with the comment, holds the ruling. A revision-bound ruling needs a new
  comment for a new revision. An unreadable source is `could-not-check`.
- **Effect admission.** None. The subject check grants no permission, claim, dispatch,
  status move or acceptance. It can only refuse. Status moves still go through the
  existing decision-gate hold, and dispatch admission stays with /10. No new store,
  queue or acceptance authority is introduced.

### Acceptance cases for the post-ruling implementation

These are proposed cases for the three existing Verify rows. They are not new
executable checks or reported results. The original commands and Evidence remain
unchanged.

| Existing future test | Case and expected discrimination |
|---|---|
| `TestPrefixedHandoffSubject` (planned) | Recorded handoff fixtures keep their canonical bytes and digests. `ResolveQualifiedID` still refuses `b-09` and `@2` spellings. Mutations: broadening the legacy grammar, or adding a revision field to the digested record, fail. |
| `TestPrefixedHandoffSubject` (planned) | `ResolveWorkSubject` on `b-09@2` returns the legacy `QID` and revision 2. A milestone reference is refused. A wrong-subject input is refused by `SameSubject` called directly, with the adapter bypassed. Mutation: comparing `Compatibility()` instead of the logical key passes a re-bound alias, so the test fails. |
| `TestRulingAliasAuthority` (planned) | A mapped human's unedited subject line passes for an alias of the same key and explicit revision, with either the declared absent card or the declared matching card digest. Refuse a copied ruling, different issue or root, wrong or absent explicit version (including a card default of 1), duplicate subject lines, an edited comment, and a bot, App or unmapped actor. Mutations: use the card's version as authority, default a missing brief version to 1, match on stream and number alone, or accept a bot. |
| `TestRulingAliasAuthority` (planned) | A typed revision-1 ruling holds if its card is edited to version 2 and the brief moves to revision 2. Also hold if the card changes while the brief stays at revision 1, if the card is removed while its marker remains, or if a card is added to a comment that declared `card=none`. Exercise the real lower gate with the upper parser bypassed. Mutations: skip digest verification, treat a removed card as a legacy marker, or ignore unexpected card presence. |
| `TestRulingAliasAuthority` (planned) | An otherwise valid, unrevisioned pre-cutover ruling keeps logical-key-only meaning. An otherwise identical comment at or after cutover, or with an unknown creation time, holds even on a marker-only issue. A pre-cutover comment with a typed subject and missing card also holds. Mutations: omit the timestamp check or let a failed typed binding fall through to legacy. |
| `TestPrefixedVerifySubject` (planned) | `NewWorkAt` on `b-09@3` returns the existing `WorkIdentity` and revision 3. Evidence presented for revision 2, or for the legacy id with no revision, is refused (`checked-failed` and `could-not-check` respectively). Mutation: building the subject from the normalized `fullId` erases the revision, so the test fails. |
| `TestPrefixedVerifySubject` (planned) | A matching subject with a stale or substituted candidate is still refused by `AcceptanceBinding.Validate`. Mutation: letting a matching subject short-circuit the candidate check fails. |

The statusgen decision-gate checks from design point 4 are also covered by existing
gate test files, extended for the same alias, comment and card cases. After the ruling, these
cases are implemented through the actual entrypoints. The compiling mutations are
recorded in the declared `09-mutations.json` (planned), and fail-first results are
shown before the implementation PR opens. None of the three named tests exists at this
source revision.

### Consumer census and transition

| Consumer | Disposition |
|---|---|
| This brief's design appendix and design changelog fragment | fixed-here: authoring material only; no shared runtime value changes. |
| `statusgen/workref` (`Subject`, `SameSubject`) and its README | follow-up /09 after the ruling; one pure value and comparison. |
| `statusgen/contexthandoff/` and `spec/context-handoff-v1.md` §2 and §6 | follow-up /09; the adapter, plus a note that the strict grammar and digests are unchanged. |
| `statusgen/domain/` and `spec/verification-domain-v1.md` "Records and identity" | follow-up /09; `NewWorkAt` and `CheckSubject` beside the unchanged candidate check, plus the future-workflow obligation. |
| `statusgen/decisiongatehold.go`, `statusgen/decisionruling.go` and `statusgen/briefruling.go` | follow-up /09; local identity checks and the fetched human-comment binding. |
| `tools/desk/internal/deskkit/` (`RulingSubjectGate`) and `tools/desk/cmd/deskevidence/` | follow-up /09; the in-process subject check on a landing, beside the unchanged `statusgen --decision-gate` run. |
| `tools/desk/internal/deskkit/briefschemagate.go` | follow-up /09: an unadvertised prefixed reference stays `could-not-check`. The advertisement itself is follow-up /10. |
| `tools/desk/internal/deskkit/rulinggate.go`, `signoff.go` and the `deskclose`/`deskmerge` register gates | out-of-scope: register rulings (`R-N`) bind no brief subject; unchanged. |
| `tools/desk/internal/deskkit/briefadmission.go` and `decisionensure.go` (marker lookup by brief id) | out-of-scope for authority: they find or file the decision issue and grant nothing. Typed marker lookup is follow-up /10 with dispatch admission. |
| Historical unrevisioned rulings | compatibility only: point 4 uses the forge creation time and existing binding checks; current card absence cannot qualify a new or revision-bound ruling. |
| Dispatch and pickup, history and exports | follow-up /10 and /12; they consume the bridge. |
| `workflow/store` and future workflow controller | out-of-scope: the persistence foundation is present and has no ruling-subject consumer. The future controller obligation is documented in the specs only. |

### Decision block

**Owner:** the maintainer authorized by the existing /09 decision issue. This gate
approves how /09 will be implemented. It does not accept any milestone outcome.

1. **Accept the bounded bridge (recommended).** This authorizes:
   - `Subject` and `SameSubject` in `statusgen/workref`;
   - the handoff and verification edge adapters, with the strict grammar and every
     record digest unchanged;
   - local identity checks and the human-comment revision binding in statusgen;
   - the in-process deskkit subject gate on Evidence landings;
   - the bounded compatibility rule for pre-cutover unrevisioned rulings;
   - the actor, subject, revocation and no-admission rules above.
2. **Request corrections.** Name the adapter, condition, preserved rule or consumer to
   change. For example, binding legacy rulings to a revision, or persisting the revision
   in handoff records under a new schema version. Implementation stays held until a
   revised proposal.
3. **Hold.** Leave /09 at its current lifecycle state, and say what evidence or
   ownership resolution is needed before another proposal.

Exact ruling needed: an unedited human comment on the existing decision issue. It names
`milestone-support/09`, the proposal PR and its head, and `DR-milestone-support-09`,
selects one option, and states any corrections. Include the revision-1 subject line
shown in point 4 if the issue still has no card. If it has a card, use the digest
form from the decision prompt. The reserved record id is
`DR-milestone-support-09`. If the proposal is accepted, the ruling is transcribed into
that decision-v1 record, with alternatives, consequences and a ruling link that can be
corroborated. The brief then cites the record through its `design:` field before
governed implementation starts. Nothing in this proposal creates an approved DR, fills
`decided-by`, records Evidence or advances the lifecycle status.
