---
brief: assay:assay:milestone-support:01
title: "Extract one shared typed work-reference resolver"
why: "Separate parsers currently assign meaning to the same work references. One public pure implementation lets both binaries agree without moving credentials or execution authority."
wave: 0
depends: []
unblocks: ["milestone-support/02", "milestone-support/03", "milestone-support/04", "milestone-support/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Refactors identity matching consumed by human rulings and claim ownership; preservation of existing strict profiles and exact work binding requires human design confirmation."
decision-trigger: spec
decision-issue: 4219
ruling: "https://github.com/medici-finance/assay/issues/4219#issuecomment-6389330807"
design: DR-milestone-workref
issues: []
schema: brief-v2
version: 1
id: eb74beee-5f67-4490-83db-2fd68255cc41
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/workref/: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "statusgen/contexthandoff/ids.go: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "statusgen/contexthandoff/ids_test.go: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "tools/desk/internal/deskkit/trailer.go: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "tools/desk/internal/deskkit/trailer_test.go: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "tools/desk/go.mod: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "tools/desk/go.sum: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "statusgen/domain/mutate.py: fixed-here milestone-support/01 (snapshot includes the extracted workref dependency)"
  - "statusgen/briefv2.go: follow-up milestone-support/04 (strict legacy grammar retained)"
  - "statusgen/domain/: follow-up milestone-support/09 (strict compatibility fullIds retained)"
  - "docs/contracts.md: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "docs/library-first.md: fixed-here milestone-support/01 (shared parser, actual adapters and bounded qualification)"
  - "docs/streams/milestone-support/brief-02-negotiate-reader-capabilities-through-one-shared-contract.md: follow-up milestone-support/02 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md: follow-up milestone-support/03 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md: follow-up milestone-support/04 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md: follow-up milestone-support/08 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 01 — Extract one shared typed work-reference resolver

## Context

files: `statusgen/workref/` (planned), `statusgen/contexthandoff/ids.go`, `statusgen/contexthandoff/ids_test.go` (planned), `tools/desk/internal/deskkit/trailer.go`, `tools/desk/internal/deskkit/trailer_test.go`, `tools/desk/go.mod`, `tools/desk/go.sum`, `docs/contracts.md`, `docs/library-first.md`, `changelog/milestone-support-01.md` (planned), `statusgen/testdata/milestone-support/01-mutations.json` (planned). New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen/workref owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen/workref"
  contract: none — existing index does not yet name this exact typed record/projection; update its existing owner row in the implementation, not a second registry.
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

## Architecture pickup — implementation assessment

Disposition: **continue** within the user's explicitly requested bounded implementation.
The gate remains human; issue #4219 has no recorded ruling at pickup. This delivery supplies
a reviewable implementation, not human design sign-off, acceptance or reader activation.
The controlling brief includes the parseable Human decision repair from source commit
`01b032dc`; its pre-implementation source SHA-256 is `c7276db0aec9d74b629ec49b8aa4c3e9e108775a4322be36cac25552cb9d3a27`.
Implementation/packet baseline is `0221bf5c15bde20fa69a4b7e47fd492fd596736d`.

Read current project rules, implementation-pickup, Go engineering/catalogue, brief §3.8,
milestone §2, stream integration, library-first and existing semantic-owner rows. The public
workref API was absent at pickup; the two Go modules and their actual legacy entrypoints
were inspected. No third-party parser fits this project-specific grammar; the brief requires
a stdlib-only owner, reusing the existing statusgen module and Go text/number primitives.
No new dependency version, process, credential or online forge authority is introduced.

Consumer coverage: QID decoding/checking and domain consumers retain the existing fullId;
trailer authors/representation/claim readers retain their existing reductions. Those actual
entrypoints call the new core. The domain mutation snapshot is updated to include the
extracted pure dependency. Separate brief-v2 grammar, schema/generator, milestone loaders,
capability assessment, claims/rulings and history are routed to the named follow-ups above.
The concurrent verification-handoff/01 work touches separate deskcore/module and semantic-owner
lines; it does not provide or change workref behavior. No active claim is overwritten.
Coverage is bounded to these source readers and the observed open PR set, not a claim that
all future/private readers or published module versions have been qualified.

Source records (SHA-256 at pickup):

| Source | Digest |
|---|---|
| `spec/brief-v1.md` | `2ded10f427b07489a9b2701566ee0f0b08d5a8ad8ce94f16738597a3b4ad9842` |
| `spec/milestone-v1.md` | `372430ec71ac115935ae33bdd19b44c44460c607be296faa505e540f521dd3cb` |
| `spec/stream-v1.md` | `8ddceacbb239bdc2ee537d576827387914293d74adbf9cd844fdc786f9cfdca9` |
| `docs/contracts.md` | `71c68c56b1c13da48feb00596b0e07e590814273255cfd94824568fcb48f4c0d` |
| `docs/go-engineering.md` | `dbb0f6bf68f6f0ccad88da47531c14c993d4cd2c07776df24e385ce35480f957` |
| `docs/library-first.md` | `a844301545abea0d3708894b44c151665a43f1a94063e8b95b1d1ab2d642b9b2` |

The named test/mutation commands below are implementation qualification. Independent
post-merge Evidence remains empty until a non-implementer verifies the merged delivery.

## Lifecycle reconciliation — 24 October 2026

Implementation PR #4224 and independent PASS Evidence PR #4256 are merged.
The bounded design ruling is option 1 on decision issue #4219, reaffirmed by the
maintainer in the unedited own-account comment naming DR-milestone-workref. This repair
attaches decision-issue, ruling and design links; the exact ruling corroborates under
registers-v1 §7.5. Existing Evidence,
Task, Verify, authored version and work identity are unchanged.

Use the canonical reconciliation writer for any todo-to-implemented update. Do not
manually advance verified/done or treat this reconciliation as human verification
sign-off. Canonical eligibility allows a gate:human prerequisite at implemented with
a recorded PASS to satisfy its dependent; /02 retains its own design/admission gate.

## Ground rules

- Own isolated branch and draft implementation PR. Do not merge, flip status/spec/stream/PR readiness, dispatch work, cut a tag, change installed pins or contact a cluster.
- Scope approval and stream activation take effect on the maintainer's merge of PR #4182; this brief has no completion evidence. A non-implementer owns post-merge verification; never fill Evidence from authorship or a fixture expectation.
- Refresh main, active claims/PRs, module imports, source contracts and consumer inventory at pickup. Record brief/source revisions, actual public API availability, coverage/unknowns and continue/adapt/hold in the implementation handoff. No obsolete active claim is overwritten.
- Preserve existing credentials, independent human/risk gates, UUIDs, archived evidence and exact-subject meanings. No ambient-auth fallback or automatic bulk migration.
- Shared resolution and capability assessment MUST be direct library calls, not subprocess calls to statusgen, desk-tools or a new helper executable. External I/O remains in existing adapters.

## Human decision

Review the concrete identity/capability boundary and preserved existing authority checks through the existing decision process before implementation. The authoring request does not grant implementation sign-off.

Options:
1. Approve the bounded design while preserving independent authority checks and legacy parser acceptance sets.
2. Request corrections while retaining the existing holds.
3. Hold the proposed change.

Default if no answer: none — blocks until answered

## Task

1. Add public stdlib-only package github.com/medici-finance/assay/statusgen/workref in the existing statusgen module, not a new module, daemon or tools/desk/internal import. Expose typed Parse/Resolve operations over explicit declaring context and an immutable supplied alias registry. Return kind, full identity, optional exact revision selector and compatibility spelling; do not read files, environment, network or tokens.
2. Implement final-segment-only b-/m- classification, colon elisions, all legacy aliases in brief §3.8, exact positive revision syntax, namespace collisions and missing/ambiguous context. Keep stable logical UUID/key distinct from spelling, source revision and version selector. Prefix-shaped names elsewhere in the ID must not select the kind. Overflow, whitespace, malformed and unknown required forms refuse/are unknown under the contracts.
3. Refactor existing contexthandoff.ResolveQualifiedID and deskkit trailer parsing to delegate to the common implementation through explicit legacy profiles preserving their DIFFERENT existing acceptance sets and outputs. Do not broaden old APIs to accept b-/m-/@ implicitly. Both production consumers must actually call the package; retaining independent regular expressions for the same semantics is not completion.
4. Wire tools/desk to the public statusgen module using the repository's require/replace pattern. Prove GOWORK=off compilation from both modules and a temporary external-consumer fixture; local replace success is source-build proof, not published-release proof. Record exact consumer/module closure; update semantic-owner and library-first documentation. No credential or online forge owner moves.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestWorkrefGrammarAndAliases$' ./workref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestWorkrefGrammarAndAliases ' "$milestone_verify_out")` | exit 0 and named PASS; mixed colon/slash aliases yield one identity, exact revision survives, wrong kind/whitespace/unknown root/overflow fail; remove alias context or classify a prefix-shaped stream as the kind and the test fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestLegacyQIDParity$' ./contexthandoff/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestLegacyQIDParity ' "$milestone_verify_out")` | exit 0 and named PASS; actual legacy API still refuses prefixes/slashes/revision selectors while valid old QIDs preserve meaning; broadening the legacy profile fails. |
| 3 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestSharedWorkrefTrailerParity$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestSharedWorkrefTrailerParity ' "$milestone_verify_out")` | exit 0 and named PASS; actual trailer parser delegates and preserves existing outputs; a divergent alias interpretation or dropped revision in the new explicit adapter fails. |
| 4 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestWorkrefPublicImportBoundary$' ./workref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestWorkrefPublicImportBoundary ' "$milestone_verify_out")` | exit 0 and named PASS; both module-consumer fixtures import the public package with GOWORK=off; planted tools/desk/internal, net/http, os/exec or credential dependency fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

### Independent run 2026-10-24 (claude-opus-5-5, strong) — container-netns admission bracket

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestWorkrefGrammarAndAliases$' ./workref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestWorkrefGrammarAndAliases ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestLegacyQIDParity$' ./contexthandoff/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestLegacyQIDParity ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestSharedWorkrefTrailerParity$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestSharedWorkrefTrailerParity ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -timeout=160s -v -run '^TestWorkrefPublicImportBoundary$' ./workref/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestWorkrefPublicImportBoundary ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:02c3cffe5a0ce6e37a9c7ddda5941024f04ab4eba05b05c2549c10a2d595f40c
Container-Image: sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204
Verification-Attestation: medici-finance/assay#4253 run=ba4b7302fd2f6d1b9b585a1a1b825196ef650a30982af23502990ae516afea1c source=cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a brief=docs/streams/milestone-support/brief-01-extract-one-shared-typed-work-reference-resolver.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ba4b7302fd2f6d1b9b585a1a1b825196ef650a30982af23502990ae516afea1c source=cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a tree=sha256:02c3cffe5a0ce6e37a9c7ddda5941024f04ab4eba05b05c2549c10a2d595f40c witness=sha256:0b44b107266b045c6bdadda3ca5ed6827a1fed199ba2e5257911c8a0e5aabae6 opened=2026-10-24T01:20:05Z closed=2026-10-24T01:21:28Z before=receipt after=check-verifier

Verified source cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a (merged main; delivery merged as #4224, merge commit 3de4d1295d64236ccfcda8e8925cbda332fef385). Independent non-implementer verifier: claude-opus-5-5, strong tier, attestation medici-finance/assay#4253 (run ba4b7302fd2f6d1b9b585a1a1b825196ef650a30982af23502990ae516afea1c). Admission check before the bracket exited 0. Host tools: statusgen v1.0.58; deskdispatch sourceSHA=832f9c8f builtAt=2026-10-23T22:20:17Z releaseTag=v1.0.58. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner:v1.0.58@sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204, image Id sha256:767f6a3545748b57a85f85dc80216e97203a39aa07e2ef137483d197dd60b175, linux/amd64, run with --network none. All four check:ci rows ran in ONE closed admission bracket (open 2026-10-24T01:20:05Z, close 2026-10-24T01:21:28Z); bracket-digest reported verdict=match. Rows 1-4: pass, exit 0, named PASS guard satisfied. The home tree was clean before and after the bracket.

Mutation obligations (supplemental probe, discharges no original row). The repo's own mutation driver, statusgen/testdata/milestone-support/01-mutate.py, was run in the same offline release-pinned image against a git-archive copy of cb17e8f99 outside the verifier home. Every baseline passed and all 10 manifest mutations were caught (exit 0):
- Row 1, TestWorkrefGrammarAndAliases: declaring-root (alias context removed), final-segment-kind (prefix-shaped stream classified as the kind), exact-overflow, alias-collision. All caught.
- Row 2, TestLegacyQIDParity: legacy-qid-broadens. Caught.
- Row 3, TestSharedWorkrefTrailerParity: trailer-projection (divergent alias interpretation), trailer-exact-revision (dropped revision). Both caught.
- Row 4, TestWorkrefPublicImportBoundary: second-process-import (os/exec), second-network-import (net/http), second-environment-import (os). All caught.
- Row 4 additional hand plants on the same copy. A planted tools/desk/internal/deskkit import turns the row red only as a build failure ("no required module provides package ... [setup failed]", exit 1). The row exits non-zero, but the named boundary assertion is not what fires, because statusgen cannot resolve the tools/desk module. A planted credential-named in-module package (statusgen/internal/credential) is caught by the named assertion itself ("pure workref boundary", "non-stdlib dependency", "forbidden dependency"; exit 1). By inspection, the AST import allowlist (fmt, regexp, strconv, strings, unicode) refuses any other import.

Risk-bearing values. Frontmatter: risk sensitive-data yes, irreversible no. Enumeration covered the workref package (workref.go, legacy.go) and the legacy grammars it replaced. Ranked: grammar and bounds first (a wrong value silently re-identifies or merges work identities); test timeouts last (operational, reversible).
RISK-VALUE: DERIVED — nameRe = ^[a-z0-9][a-z0-9-]*$ @ statusgen/workref/workref.go:139 — byte-identical to the legacy idSegmentRe still at statusgen/contexthandoff/ids.go:20, so it preserves legacy segment meaning
RISK-VALUE: DERIVED — numberRe = ^[0-9]{2,4}[a-z]?$ @ statusgen/workref/workref.go:140 — identical to legacy idNumberRe (statusgen/contexthandoff/ids.go:21); brief section 3.8 says number grammar follows context-handoff section 2
RISK-VALUE: DERIVED — milestoneRe = ^[0-9]{2,}$ (and n != 0) @ statusgen/workref/workref.go:141, :218 — spec/milestone-v1.md: a positive decimal number padded to at least two digits; m-3 is rejected
RISK-VALUE: DERIVED — issueRe = ^[A-Za-z0-9._/-]*#[0-9]+$ @ statusgen/workref/workref.go:142 — identical to the removed contexthandoff issueRefRe at the delivery parent 0221bf5c
RISK-VALUE: DERIVED — crossRepoRe = ^([a-z][a-z0-9_-]{0,31}):([^:/\s]+)/([0-9]+)$ @ statusgen/workref/workref.go:143 — identical to the removed deskkit reCrossRepoBriefValue at 0221bf5c, which preserves trailer outputs
RISK-VALUE: DERIVED — segment bounds 2..4 @ statusgen/workref/workref.go:193, statusgen/workref/legacy.go:25 — identical to the removed bounds; the longest spec form is cell:repo:stream:NN (four segments)
RISK-VALUE: DERIVED — revision == 0 refused, uint64 bound @ statusgen/workref/workref.go:176-177 — section 3.8: a present version is a positive integer. uint64 is an implementation bound; overflow is refused rather than truncated, and the maximum value is kept exactly
RISK-VALUE: DERIVED — milestone requires revision != 0 @ statusgen/workref/workref.go:218 — milestone-v1: a milestone reference pins an exact positive contract version
RISK-VALUE: DERIVED — prefixed slash alias with 3 segments refused @ statusgen/workref/workref.go:213 — the spec's draft slash forms are only stream/ID and the full cell:repo:stream/ID

Gate. The brief is gate: human with risk sensitive-data yes. Its start decision #4219 was ruled option 1 (approve the bounded design) by the ratifying human at 2026-10-23T20:23Z. That was after implementation began (#4224 opened at 14:40Z, and the pickup section above records no ruling at pickup) and before the delivery merged (#4224, merge 3de4d1295, at 20:50Z). That ruling approves the design and signs off no verification run. A model PASS does not flip a gate: human item, so the verified transition stays with the human gate (human:ian).

Output column. Every row's Output cell is the SHA-256 of empty input (sha256:e3b0c44298fc…). This follows from the commands themselves, not a capture gap: each row redirects its go test output to a mktemp file and gates on grep -q, so it prints nothing to stdout. The cells therefore attest each row's exit code and its named-PASS grep guard, not test output.

Disclosure (#4163). There were no discarded or failed attempts. One admitted bracket run produced the table above. The only other runs were the supplemental mutation probe and the hand plants. They ran on a copy outside the verifier home and receive no row credit.

**VERIFY: PASS** — 4/4 rows pass exit=0 in one closed container-netns admission bracket at cb17e8f99a55 (bracket-digest verdict=match). Container-row credit is deferred to --bracket-digest --credit on the landed brief. The human gate is still owed: the brief is gate: human (risk sensitive-data yes), and its start decision #4219 approved the design but signs off no verification.

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
