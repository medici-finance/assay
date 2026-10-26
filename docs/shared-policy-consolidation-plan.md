# Shared policy consolidation plan

Status: implementation candidate, 2026-10-25. Follow-up tracker: #4439.
The original scope and audit below are retained as the baseline. This candidate
implements the selected behaviors without changing acceptance authority.

Source baseline: main `4aeecf71626f485c4938dec3165e887a2f74e20d`, refreshed
2026-10-25; closure implementation in PR #4435, issues #4431 and #4437.
Architecture pickup: **continue** with narrow offline extraction under
[Library-first](library-first.md), its recorded offline/frozen-input direction,
and the existing semantic-owner index in [Contracts](contracts.md). Both
`statusgen/go.mod` and `tools/desk/go.mod` already depend on `deskcore`; desk-tools
also imports `statusgen/workref`. Separate modules do not require copied policy.
`deskkit` remains internal. Online statusgen reads remain on `deskread`.
No credential, forge write, service, cache or new authority is introduced.

## Answer: how many differ?

Of the seven nominated areas, **five have observable differences**. Four have
acceptance/parser mismatches to reconcile, including the closure defects repaired
by the accompanying change. One has an intentional capability restriction. Two
have no core-rule difference identified in this bounded audit. This is a count of
areas, not functions or proof of equivalence on every possible input.

| Area | Compared implementations | Observation and disposition |
|---|---|---|
| Closure acceptance | `statusgen/briefinfo.go`, `verifyclosure.go`, `witnessgate.go` | Before the fix, passing ordinary witnesses without Base cells passed `verifyclosure` and failed the other checks; empty Verify rows and model stamps over human rows also differed. Explicit checks now share `checkSelectedClosure`; lint shares `failFirstClosureChecks` and deliberately retains its transition scope. Issues #4431/#4437. |
| Brief parsing and section extraction | `statusgen/brieffile.go`, `verifyissues.go`; `tools/desk/cmd/{verifyloop,fanoutloop}/frontmatter.go`; `cmd/deskdispatch/packet_verifier.go`; `cmd/deskpathguard/rederive.go` | On the valid YAML input `risk: {regulatory: "yes"}`, statusgen reads `yes`; verifyloop reports false. The regex risk reader is also copied in fanoutloop. Heading grammars independently differ in whitespace, prefix and case handling. Share syntax first, then explicit projections; retain the packet's Evidence-removal obligation. |
| Roster/trust | `statusgen/{rosterconfig,trustgate}.go`; `tools/desk/internal/deskkit/{rosterconfig,trust}.go` | With a rostered bot pinned to ID 3003 and an observed ID 9999, statusgen `trustedAuthorID` returns true; deskkit `trustedContentAuthor` returns false. Current shared vectors omit this case. Separately, login-only and strict numeric-ID surfaces deliberately differ. Add explicit observation profiles and resolve the bot-ID mismatch through the trust owner before cutover. |
| Signed verdicts | `statusgen/transcribeverdict.go`, `transcribescan.go`; `tools/desk/internal/deskkit/verdict.go` | Canonical JSON, envelope and signature machinery are copied; no core-rule difference identified here. Verifier and issue-loop roles remain explicit policy inputs. Treat malformed JSON, duplicate keys and unusual numeric forms as characterization work, not proven equivalence or an authorized behavior change. |
| Attestation admission | `statusgen/containercredit.go`; `tools/desk/internal/deskkit/verifierattestation.go` | Container credit explicitly rejects version 2 cross-repository bindings; general attestation admission supports versions 1 and 2. This capability difference is intentional. Share binding/provenance judgments with an explicit allowed-version/profile input; retain current-state rechecks in each effectful adapter. |
| Risk-path matching | `statusgen/riskpathtriggers.go`; `tools/desk/internal/deskkit/riskpath.go` | Trigger matching and additive union are copied with shared vectors; no core matcher difference identified. The PR classifier additionally risk-classes public/unknown repos and missing observations, while the brief cross-read owns its separate could-not-check handling. Preserve those caller policies. |
| PR work trailers | `statusgen/{prlink,ghfetch,autoflip,mergedstatus}.go`; `tools/desk/internal/deskkit/trailer.go` | For `~~~`, a newline, `Brief: ff/01`, a newline and `~~~`, the legacy PR classifier and desk parser read a link; `mergeTrailerKeys` ignores it as fenced text. Historical merge readers and new-PR validation also differ on mixed trailers. Share lexical scanning and explicit read/write profiles; keep existing `statusgen/workref` identifier semantics. |

The quoted-risk, wrong-bot-ID and tilde-fence results above were executed with
temporary package-local Go probes against the real functions on this source
snapshot. Closure mismatches have committed regression coverage. The temporary
probes assert *observed* behavior, not the desired future policy. Their minimal
inputs and outputs are recorded above so characterization can reproduce them.
The count includes intended differences; it does not label all five as defects.

## Delivery outcomes and dependency waves

Implementation units below are proposed scopes, with effort capped at M. Before
pickup, map them to existing owner briefs, preserve their accepted history, and
author or amend executable briefs through `assay:author-brief`. No invented brief
IDs, generated board edits or lifecycle claims are part of this plan.

| Unit | Wave / depends | Effort | Deliverable and consumers | Acceptance |
|---|---|---|---|---|
| A: characterization and policy profiles | 0 / none | M | Cross-caller fixture matrix for all seven areas; explicit desired outcomes and named intentional profiles. Record source revisions, existing tests, consumer roster, unavailable coverage and trust/grammar decisions in the existing owner contracts. | Reproduce the three minimal non-closure mismatches above and closure regressions before choosing an oracle. Every differing result is resolved, intentionally profiled, or held with an owner. Equal answers from two broken copies are insufficient. |
| B: risk-path primitive | 1 / A's risk profile | S | Extract union and segment matching into an offline importable package under `deskcore`; convert statusgen lint and deskkit risk-path callers. Supply trigger values as data using existing topology derivations. | Existing trigger coupling vectors pass through both actual callers; additive-only and wildcard/prefix controls discriminate; PR uncertainty and visibility behavior stay unchanged. Delete copied matcher bodies. |
| C: verdict primitives | 1 / A's verdict profile | M | Extract canonical bytes, envelope parse, public-key verification and role declaration into an offline importable package under `deskcore`; migrate signer/verify adapters, verdict transcription and scan-delta verification. Keep signing-key custody in its existing owner. | Existing signed payloads verify byte-for-byte through both callers. Wrong role, modified payload, wrong key, trailing data and encoding controls refuse. No new credential/network imports; remove duplicate canonicalizers and parsers. |
| D: brief document syntax | 1 / A's document grammar | M | Expose canonical frontmatter and section parsing under `statusgen/briefdoc` or the existing owner-approved offline package. Convert statusgen plus verifyloop, fanoutloop, dispatcher packets, rebaseline and path-guard consumers. | Valid quoted YAML agrees; comment/CRLF/heading and malformed-document cases use the chosen oracle. A packet built from every accepted heading variant contains no earlier Evidence or observed verdict. Delete local regex/parser copies only after every consumer migrates. |
| E: trust and roster interpretation | 1 / A's trust decision | M | Shared pure roster normalization and identity/content/blessing evaluation under an owner-approved `deskcore` package. Keep file/env selection, permission checks, roster echo and forge observation in adapters. | Wrong bot ID, missing human ID, unpinned human, recycled login, bot rendering, absent roster and post-blessing edit cases exercise the real scanner and desk callers. Strict versus login-only is explicit. Preserve write-class rejection of env roster and CI read transport. No copied trust judges remain. |
| F: work-trailer interpretation | 1 / A's trailer profiles | M | Extend the existing `statusgen/workref` owner with shared fence-aware trailer scanning, retaining legacy and strict projections. Migrate PR validation, link classification, fetch, autoflip and historical merge readers. | Backtick/tilde/mixed fences, CRLF, duplicate and mixed trailers, Authors and Cross-repo-brief exercise all callers. Identifier compatibility and revision pins remain unchanged. Historical exceptions are named profiles, not independent parsers. |
| G: attestation judgments | 2 / A's attestation profiles, E | M | Shared offline binding/provenance/stamp judgments under an owner-approved `deskcore` package. Convert container credit and general admission; freeze supplied observations and keep record/history/current-state collection in adapters. | Shared v1 fixtures agree; the same v2 record is refused by the container profile and admitted only by the capable general profile. Reopen/edit/label races still refuse independently at dispatch and landing. Delete copied field/provenance judges without removing rechecks. |
| H: importable closure API | 2 / D; closure fix merged | M | Move the selected-closure predicate and typed findings from command-local code into the existing statusgen evidence/evaluation owner, using explicit immutable observations for Git-derived provenance. `brief`, `verifyclosure`, lint and applicable desk consumers call the same offline rules with explicit scope. | Three-entrypoint missing/green/unproven/stale/invalid-base cases agree for new closures. Inherited lint remains scoped while explicit outcomes remain strict. Protected originals, supplemental rows, human rows, empty rows and unavailable history retain their selected contracts. Remove command-local policy copies. |
| I: release, consumer cutover and retirement | 3 / B–H | M | Complete module/import, build/container, CI trigger and consumer inventory; update library-first migration pointers and existing contract rows. Use existing release/pin owners, preserving paired-version requirements. | `GOWORK=off` builds both modules; offline probes and end-to-end caller journeys pass; a source/import census finds one rule body per meaning. Test minimum supported old/new wire and binary combinations. Publish only through existing release authority; migrate pins through their existing owners. |

Wave 1 units may proceed independently only after their own A profile is settled.
A policy hold in trust does not block risk matching or verdict canonicalization.
Unit G genuinely needs E's identity/profile boundary; unit H genuinely needs D's
shared parsed document. Neither waits for a daemon, mirror or runtime rewrite.

Critical path for complete retirement: **A → E → G → I** and **A → D → H → I**.
These are proposed dependency chains, not a duration estimate or a measured claim
that one chain dominates. C's signing compatibility can independently hold I.
The smallest unblocking move is A's explicit behavior matrix and bot-ID/grammar
reconciliation. Moving the old bodies first would preserve or accidentally widen
the disagreement. Module access is already available and is not the blocker.

## Interface and migration contract

- One semantic owner and one implementation per shared rule. Update the existing
  `docs/contracts.md` row; package placement does not create another policy owner.
- Parse/evaluate frozen values in libraries. Return typed accepted/refused/unknown
  findings and reason codes; adapters own CLI text, JSON envelopes and exit codes.
- Pass policy differences as small named profiles with tests for the exact allowed
  delta: strict ID versus login-only, container-v1 versus general-v1/v2, historical
  read versus new-write trailers, selected outcome versus inherited corpus lint.
  Do not use a generic bypass boolean or union all currently accepted inputs.
- Preserve process boundaries that carry authority. No `deskkit` import from
  statusgen, role token transfer, online-read relocation or cached acceptance.
  Dispatch and landing must still perform their own consequential state reads.
- Keep old commands and wire shapes until all consumers are inventoried and
  migrated. Wrapper functions may forward during the supported transition;
  they may not keep a second implementation behind an old name.
- Retire parity tests that compare independent rule bodies only when those bodies
  are removed. Retain the fixtures as contract/caller tests, with negative controls
  and end-to-end tests proving the production adapters call the selected owner.

## Verification and failure modes

| Failure mode | Required evidence |
|---|---|
| Common helper exists but a production caller bypasses it | Real CLI/adapter fixture journeys plus planted bypass controls; an import or text search alone does not prove conformance. |
| Extraction silently chooses the more permissive copy | A's reviewed expected verdicts, wrong-bot-ID and quoted-risk examples, explicit profile matrix and protected acceptance negative controls. |
| Intentional difference is accidentally erased | Same v2 attestation through both profiles, missing-ID through strict/login-only profiles, inherited closure through corpus/selected scope. |
| Duplicate path survives under a wrapper | Consumer inventory and function/call census after migration, followed by an actual source deletion diff and caller tests. |
| Sharing policy weakens independent admission | Bypass the upper layer; changed body/current labels/reopened record must still fail the later landing/admission boundary. |
| Local workspace hides a broken release | Build/test each module with `GOWORK=off`; prove release assets include each required module and execute supported paired pins. |
| New cross-module fixtures never run in CI | Update the existing cross-module registry and affected workflow path filters through the workflow owner; plant a shared-fixture change and show both suites run. |

Run targeted owner and real-caller tests before corpus tests. Record actual exit
codes and time limits. The original full statusgen invocation hit its default
10-minute budget; focused closure/Protected/Vflip coverage subsequently passed in
209.586 seconds. A timeout is not an equivalence result. Larger qualification runs
must use an explicit sufficient budget or bounded test partitions, with every
partition accounted for. This plan adds no performance claim or invented budget.

## Completion and selected behavior

Completion means the real consumers use one owner per meaning, obsolete bodies
are deleted, intentional profiles remain explicit, compatible artifacts/pins are
qualified and independent verification has supplied the evidence. A plan, helper
addition or green unit test alone does not establish that migration.

The selected behavior is now explicit in the shared functions and regression
fixtures: supplied numeric IDs must match pins, valid quoted YAML risk answers
retain their meaning, and fenced work-link examples grant no delivery credit.
Container-v1 versus general-v1/v2, login-only versus numeric-ID observations,
historical-read versus strict new-write trailers and inherited lint versus new
selected outcomes remain explicit profiles. No additional policy choice is left
for a caller to reconstruct from a copied implementation.

## Implementation inventory

| Original unit | Candidate disposition |
|---|---|
| A | Wrong-bot-ID, quoted-risk, fenced-link, capability-profile and negative evidence cases are exercised through actual library/caller tests; the shared roster vectors now include the omitted wrong-bot-ID case. |
| B | `deskcore/riskpath` owns base values, additive union and matching; lint/deskkit functions forward. |
| C | `deskcore/verdict` owns canonical bytes, envelope, role and public-key verification; custody/source adapters stay local. |
| D | `statusgen/briefdoc` owns the shared frontmatter/section grammar; all nominated command/admission adapters consume it, including packet and rebaseline section boundaries. Statusgen consumes the fence split, section boundaries and risk-key list, but still decodes frontmatter fields with its own single-document reader, so the misplaced-risk, multi-document and non-printing-key refusals bind desk readers only. |
| E | `deskcore/trust` owns the shared key schema, lexical/forge identity parsing, rendering, strict/profiled identity and edit-aware blessing judgments. Config field projection, source transport/permission/echo and forge reads remain adapters. |
| F | `statusgen/workref/trailers` owns scanning and new-write cardinality/errors; historical projections retain their existing attribution responsibilities. |
| G | `deskcore/attestation` owns binding types/decoding/rendering/validation, immutable-record/target judgments and standing-stamp judgments. Record reads and current-state rechecks remain separate. |
| H | `statusgen/evaluation/selectedclosure.go` exposes frozen-input selected-closure and fail-first row decisions; command-local code collects parser/Git observations and renders diagnostics. |
| I | Both modules already pin/replace the existing shared modules and image builds already copy them; no new module or credential packaging is needed. Builds and tests use `GOWORK=off`. Actual release/tag/pin publication remains the existing release owner's operation. |

Independent review/verification still decides acceptance. The implementation
candidate does not assert a completed release or rewrite historical Evidence.
