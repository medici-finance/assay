---
brief: assay:assay:statusgen:19
title: Context handoff contracts and synthetic fixtures
why: "When a spec changes the plan, the author of the follow-up briefs gets a prose impact list and has to guess what to amend. A versioned, machine-checkable handoff (target, what was searched, what must change, what history must survive) lets a missing amendment fail a check instead of slipping through review."
wave: 1
depends: []
unblocks: ["statusgen/20"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
  - docs/proposals/non-ci-architecture/18-authoring-context-delivery.md — package "Context/plan contracts"
  - docs/proposals/non-ci-architecture/12-future-state-model.md — handoff records
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — step 2
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — step-2 owner row
  - "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
  - freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Defines a new shared wire contract that both canonical authoring skills and the resolver consume; versioning and canonical-digest choices are hard to change later.
domain: complicated
outcome: none
consumers:
  - "statusgen/contexthandoff, spec/context-handoff-v1.md, schemas/context-handoff-v1.json, schemas/amendment-manifest-v1.json, docs/contracts.md: follow-up statusgen/19"
  - "statusgen context subcommand and resolver: follow-up statusgen/20"
  - "plugins/assay/skills/author-spec, plugins/assay/skills/author-brief: follow-up statusgen/21"
  - "docs/proposals/non-ci-architecture/examples/architecture-change-v0.yaml: out-of-scope (proposal illustration, not a production schema; left unchanged)"
id: ce32913b-bf75-409f-8781-6fe845260a68
---

# Brief 19 — Context handoff contracts and synthetic fixtures

## Context

files: `spec/context-handoff-v1.md` (planned), `schemas/context-handoff-v1.json` (planned), `schemas/amendment-manifest-v1.json` (planned), `statusgen/contexthandoff/` (planned) (new pure package and tests), `statusgen/testdata/contexthandoff/` (planned), `docs/contracts.md` (one new owner row), `changelog/statusgen-19-context-handoff-contracts.md` (planned).

facts (a435badbd, 2026-10-13):
- The handoff records named by the ratified target are: target and intent, discovery receipt,
  impact obligation, amendment obligation, preserved history, active-work binding, and decisions
  and readiness (doc 12).
- The proposal's `docs/proposals/non-ci-architecture/examples/architecture-change-v0.yaml` exists
  and is illustrative only; it is not a production schema and nothing reads it.
- Schemas live in `schemas/`, prose contracts in `spec/`; semantic owners are registered in
  `docs/contracts.md` (three-part pattern: spec, schema, fixtures).
- statusgen already owns the brief parser and the deterministic graph export
  (`TestGraphJSONLDeterministic`).
- A qualified brief ID's canonical form is four segments, `<cell>:<repo>:<stream>:<NN>` (e.g.
  `assay:assay:statusgen:19`), with the repo segment an alias from `docs/streams/graph-repos.yaml`.
  References may use exactly two elisions, `<repo>:<stream>:<NN>` and `<stream>:<NN>`, where each
  omitted prefix means "same as the declaring brief" (`docs/brief-template.md`, the `brief:` field
  and its notes). The brief-v1 `<stream>/<NN>` form is a migration-window read form, not a
  qualified ID.
- No `contexthandoff` package, `context-handoff` spec or schema exists at a435badbd.
- The name is "context handoff" to stay distinct from graph-execution/09's execution work-input
  packet, which is a role view of a workflow instance.

layering: records plus pure validation and canonicalization only. Reading files, git or forges is
statusgen/20's adapter work. Verify 7 enforces the import boundary.

single-point-of-failure: the amendment-obligation check (an "amend" obligation without a bound
patch fails) — behind it, statusgen/20's patch-to-base check independently refuses a patch that
does not apply to its recorded base.

design-fit:
  owner: statusgen/contexthandoff under a new S-context-handoff row in docs/contracts.md
  contract: context-handoff-v1 and amendment-manifest-v1
  retires: [prose-only impact lists as the spec-to-brief handoff]
  weight: "verbs 0, flags 0 in this brief; one importable package and two schemas"
  why-add: "No existing record carries a discovery receipt or amendment obligation; the brief parser and graph export describe current state, not a proposed change."

## Read first

- `docs/proposals/non-ci-architecture/12-future-state-model.md`,
  `docs/proposals/non-ci-architecture/17-context-substrate.md`,
  `docs/proposals/non-ci-architecture/18-authoring-context-delivery.md`.
- `docs/contracts.md`.

## Ground rules

- Isolated branch; draft PR only. No merge or release.
- Synthetic, neutral fixtures only; no private repositories, rosters or cell data.
- Stop at implemented; independent verification owns verified/done.

## Task

1. Write `spec/context-handoff-v1.md` (planned) and `schemas/context-handoff-v1.json` for the seven records
   above. Every brief reference is a qualified ID: the canonical form is the four-segment
   `<cell>:<repo>:<stream>:<NN>`; the only accepted elisions are `<repo>:<stream>:<NN>` and
   `<stream>:<NN>`, each expanded against the declaring record's cell and repo before validation,
   so validated records and canonical digests always carry the four-segment form. Bare `#N`,
   path-only and brief-v1 `<stream>/<NN>` references are refused. A draft or proposed target is a
   distinct state from an approved one, with the approving revision recorded.
2. Write `schemas/amendment-manifest-v1.json` (planned): per obligation, the owning brief, delivery paths,
   contract version, dependencies, test commands, active-work treatment, effort, and the bound
   patch (base revision and patch digest) or an explicit follow-up reference.
3. Implement `statusgen/contexthandoff` (planned) (pure): decode with unknown-mandatory-field refusal,
   validate, and canonicalize to a stable digest. An obligation marked "amend" with no bound patch
   fails. A discovery receipt without stated coverage (what was enumerated, what was unavailable)
   fails.
4. Add positive and negative fixtures and the S-context-handoff owner row; add the changelog
   fragment.

## Interface contract

statusgen/20 produces and rechecks these records; statusgen/21's skills read and emit them. Both
consume this package and schema version; neither defines a second shape. Canonical digests are
stable across field order and whitespace.

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffAmendWithoutPatchFails$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffAmendWithoutPatchFails " "$out")` | exit 0; named PASS; an "amend" obligation with no bound patch or follow-up reference is refused; mutation: accept an empty patch binding — the test must fail |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCoverageGapRequired$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCoverageGapRequired " "$out")` | exit 0; named PASS; a discovery receipt without coverage, or with an unavailable source silently omitted, is refused; mutation: default missing coverage to complete — the test must fail |
| 3 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffDraftIsNotApproved$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffDraftIsNotApproved " "$out")` | exit 0; named PASS; a draft target cannot satisfy a check that requires an approved target, and approval without a recorded revision is refused; mutation: treat any target as approved — the test must fail |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCanonicalDigestStable$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCanonicalDigestStable " "$out")` | exit 0; named PASS; reordered fields give the same digest and a changed value gives a different one; mutation: hash the raw input bytes — the test must fail |
| 5 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffQualifiedIDsResolve$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffQualifiedIDsResolve " "$out")` | exit 0; named PASS; a four-segment `<cell>:<repo>:<stream>:<NN>` reference is accepted; the elisions `<repo>:<stream>:<NN>` and `<stream>:<NN>` are accepted and canonicalize to the same four-segment ID and the same digest as the full form; bare `#N`, path-only, brief-v1 `<stream>/<NN>` and other unqualified references are refused; mutation: accept a path as a brief identity, or keep an elided reference unexpanded in the canonical record — the test must fail |
| 6 | check:ci +dereference +flow | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffSchemaFixtures$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffSchemaFixtures " "$out")` | exit 0; named PASS; each fixture goes through the production decoder and the JSON schema (spec → schema → package), both agree with the fixture's positive/negative label, and every schema record is named in the spec; mutation: drop a required field from the schema only — the test must fail |
| 7 | check:ci +mutation | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./contexthandoff/) && test "$name" = contexthandoff && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./contexthandoff/) && test -z "$bad")` | exit 0; the package exists and imports no filesystem, process or network package; mutation: add an `os` import — the row must fail |

## Pre-mortem

Likely wrong builds: an "amend" row accepted without a patch (row 1, the exact failure this
contract exists to stop), silent coverage gaps (row 2), drafts treated as approved (row 3),
unstable digests (row 4), path identities (row 5), schema drift (row 6) and effects in the
contract package (row 7).

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 56132c5ffb27 (7/7 rows pass; all 10 mutation probes caught)

Verification-Attestation: medici-finance/assay#2949 run=47d272e15641b7fddab4611c9f8dd2f08c498d7f4e22e7d8f548578b61daa4c2 source=56132c5ffb27b5c614a9be2111121c525a7afb50 brief=docs/streams/statusgen/brief-19-context-handoff-contracts.md model=claude-opus-5-5 tier=strong

Fresh non-implementer run of all seven rows, verbatim, on a detached home at merged main 56132c5ffb27 (go1.27.2, git 2.56.0, offline), admitted by `deskdispatch --check-verifier` (rc 0). Delivered by medici-finance/assay#2763 (merge 13df71407); no later commit touches the package, schemas, spec or fixtures. Each Expect-cell mutation was applied to a separate archived copy, never the home; every one turned its row red. Full package suite and `go vet` pass.

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffAmendWithoutPatchFails$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffAmendWithoutPatchFails " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Amend Without Patch Fails (0.00s)`. Mutation (patch-binding check disabled, so an empty patch is accepted): exit 1, "decode empty patch: rule = "" want amend-unbound" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCoverageGapRequired$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCoverageGapRequired " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Coverage Gap Required (0.00s)`. Mutation A (missing coverage defaults to complete): exit 1, "no coverage: rule = "" want coverage". Mutation B (included-but-unaccounted check removed): exit 1, "unavailable source silently omitted ... want coverage" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffDraftIsNotApproved$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffDraftIsNotApproved " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Draft Is Not Approved (0.00s)`. Mutation (the require-approved check always returns nil): exit 1, "draft RequireApproved: rule = "" want not-approved" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCanonicalDigestStable$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCanonicalDigestStable " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Canonical Digest Stable (0.00s)`. Mutation (the handoff digest hashes the raw input bytes): exit 1, "digest changed with field order and whitespace" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffQualifiedIDsResolve$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffQualifiedIDsResolve " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Qualified IDs Resolve (0.00s)`. Mutation A (a path is accepted as a brief ID): exit 1, "... accepted ...; it is not a qualified brief ID". Mutation B (the decoder keeps the elided form): exit 1, "names gadgets:04, which has no impact record" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffSchemaFixtures$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffSchemaFixtures " "$out")` | PASS | exit 0; `--- PASS: Test Context Handoff Schema Fixtures (0.06s)`. Mutation ("intent" dropped from the target's schema required list only): exit 1, "schema accepts a fixture both layers must refuse ... (schema drift)" | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./contexthandoff/) && test "$name" = contexthandoff && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./contexthandoff/) && test -z "$bad")` | PASS | exit 0; the imports are bytes, crypto/sha256, encoding/hex, encoding/json, fmt, io, regexp, sort, strconv, strings and unicode/utf8. Mutation (an `os` import added): exit 1, bad=os | 2026-10-14 | assay-verifier-app[bot] @ 56132c5ffb27 (on-behalf-of human:ian) (git-config) |

Risk enumeration (trigger not fired: risk metadata present, all no, reversible): revisionRe, digestRe, nameRe, timestampRe, schemeRe, idSegmentRe, idNumberRe, issueRefRe, HandoffSchema, ManifestSchema. Digest and revision shapes rank first because recorded digests outlive a code change.

RISK-VALUE: DERIVED — digestRe = `^sha256:[0-9a-f]{64}$` @ statusgen/contexthandoff/decode.go:69 — sha256 yields 32 bytes, which is 64 lower-case hex characters. This is exactly what the digest function in canonical.go emits ("sha256:" followed by the hex encoding of the sha256 sum), and the manifest schema's digest pattern is the same.
RISK-VALUE: DERIVED — revisionRe = `^[0-9a-f]{7,64}$` @ statusgen/contexthandoff/decode.go:68 — 7 is git's minimum abbreviated object name, 40 (full SHA-1) falls inside the range, and 64 is the full SHA-256 object-format ID. Git prints lower-case hex. The schema revision pattern is identical.
RISK-VALUE: NAMED, NOT DERIVED — idNumberRe = `^[0-9]{2,4}[a-z]?$` @ statusgen/contexthandoff/ids.go:22 — it agrees with the schema fullId pattern. However, the brief template gives NN without a stated digit bound or letter-suffix rule, so no repo source pins the 2-4 width. The value is reversible (an edit plus a schema version).

**VERIFY: PASS** — 7/7 rows exit 0 with the named PASS; every Expect-cell mutation was caught.

Container execution witness, 2026-10-19, under the driver's 2026-10-19 interim-runner ruling (recorded on the private house tracker): `statusgen verifyrun` ran all seven rows non-dry at source 4b15482af5c6c71b9312ca7df0f23b219de3f840 inside a local docker container started with `--network none`, sandbox=container-netns, with the engine's default seccomp profile, admitted by `deskdispatch --check-verifier` (rc 0) under the attestation below.

Verification-Attestation: medici-finance/assay#3494 run=ced3c60168b4c27a466c71eeefa9908ce7683d49a1a309bdfbafad4fda0ff418 source=4b15482af5c6c71b9312ca7df0f23b219de3f840 brief=docs/streams/statusgen/brief-19-context-handoff-contracts.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffAmendWithoutPatchFails$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffAmendWithoutPatchFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:d960a6903590 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCoverageGapRequired$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCoverageGapRequired " "$out")` | pass exit=0 sandbox=container-netns | sha256:e41a75e96928 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffDraftIsNotApproved$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffDraftIsNotApproved " "$out")` | pass exit=0 sandbox=container-netns | sha256:ff5f135921e4 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCanonicalDigestStable$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCanonicalDigestStable " "$out")` | pass exit=0 sandbox=container-netns | sha256:85ac25386432 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffQualifiedIDsResolve$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffQualifiedIDsResolve " "$out")` | pass exit=0 sandbox=container-netns | sha256:370d3e55098f | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffSchemaFixtures$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:5d866f9e3bfd | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./contexthandoff/) && test "$name" = contexthandoff && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./contexthandoff/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |

The substance is the by-hand non-implementer run recorded above (2026-10-14, source 56132c5ffb27b5c614a9be2111121c525a7afb50: 7/7 rows pass, every Expect-cell mutation caught); this witness re-executes the same seven commands at the later source.

**VERIFY: PASS** — the 2026-10-14 by-hand run passed all 7 rows with 0 fail, and the container witness at 4b15482af5c6 passed all 7 rows (exit 0, 0 fail, 0 could-not-run).

### Receipt-bearing container witness — 2026-10-20 claude-opus-5-5 verifier at 742839555dcd (7/7 rows pass)

Container execution witness, 2026-10-20, under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker): `statusgen verifyrun` ran all seven rows non-dry at source 742839555dcd1731139b15a295cf78cdbad9dc91 inside a local docker container (image assay-verify:go1.27, built from golang:1.27.2-trixie, git 2.47) started with `--network none`, sandbox=container-netns, with the engine's default seccomp profile, admitted by `deskdispatch --check-verifier` (rc 0) under the attestation below. This is a fresh receipt-bearing pass at current main; the latest prior Evidence on main is the 2026-10-19 container witness at 4b15482af5c6 directly above, landed by medici-finance/assay#3503.

Verification-Attestation: medici-finance/assay#3648 run=14ca283a9faba278c38d348ab990393e1ce48a025227803e6d0bb6c633c3d1d7 source=742839555dcd1731139b15a295cf78cdbad9dc91 brief=docs/streams/statusgen/brief-19-context-handoff-contracts.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffAmendWithoutPatchFails$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffAmendWithoutPatchFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:d960a6903590 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCoverageGapRequired$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCoverageGapRequired " "$out")` | pass exit=0 sandbox=container-netns | sha256:e41a75e96928 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffDraftIsNotApproved$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffDraftIsNotApproved " "$out")` | pass exit=0 sandbox=container-netns | sha256:ff5f135921e4 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffCanonicalDigestStable$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffCanonicalDigestStable " "$out")` | pass exit=0 sandbox=container-netns | sha256:85ac25386432 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffQualifiedIDsResolve$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffQualifiedIDsResolve " "$out")` | pass exit=0 sandbox=container-netns | sha256:370d3e55098f | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextHandoffSchemaFixtures$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextHandoffSchemaFixtures " "$out")` | pass exit=0 sandbox=container-netns | sha256:2b454af76929 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd statusgen && name=$(GOWORK=off go list -f '{{.Name}}' ./contexthandoff/) && test "$name" = contexthandoff && bad=$(GOWORK=off go list -f '{{range .Imports}}{{if or (eq . "os") (eq . "os/exec") (eq . "net") (eq . "net/http") (eq . "syscall") (eq . "io/ioutil") (eq . "path/filepath")}}{{.}} {{end}}{{end}}' ./contexthandoff/) && test -z "$bad")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |

The substance is the by-hand non-implementer run recorded above (2026-10-14, source 56132c5ffb27b5c614a9be2111121c525a7afb50: 7/7 rows pass, every Expect-cell mutation caught); this witness re-executes the same seven commands at the current source.

**VERIFY: PASS** — the 2026-10-14 by-hand run passed all 7 rows with 0 fail, and the container witness at 742839555dcd passed all 7 rows (exit 0, 0 fail, 0 could-not-run).

## Review

Gate: model (all four risk answers no). The reviewer checks the record set against doc 12 and
confirms no field carries private content or counts.
