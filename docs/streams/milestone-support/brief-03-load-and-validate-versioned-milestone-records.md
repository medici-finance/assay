---
brief: assay:assay:milestone-support:03
title: "Load and validate versioned milestone records"
why: "Milestone ownership, versions and missing roots need a canonical reader before graph or report consumers can rely on them."
wave: 2
depends: ["milestone-support/01", "milestone-support/02"]
unblocks: ["milestone-support/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: d4a924d8-f812-4434-a1b8-c620bf4679e9
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
  - "statusgen/milestones.go: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "statusgen/milestones_test.go: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "statusgen/parse.go: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "statusgen/load.go: fixed-here milestone-support/03 (production loadStreams seam attaches plans after stream discovery)"
  - "statusgen/model.go: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "statusgen/testdata/milestone-support/03/: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "schemas/milestone-v1.json: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "spec/milestone-v1.md: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "spec/stream-v1.md: fixed-here milestone-support/03 (opt-in record loader, validation and unknown coverage)"
  - "docs/streams/milestone-support/brief-05-build-the-combined-milestone-and-brief-dependency-graph.md: follow-up milestone-support/05 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 03 — Load and validate versioned milestone records

## Context

files: `statusgen/milestones.go` (planned), `statusgen/milestones_test.go` (planned), `statusgen/parse.go`, `statusgen/model.go`, `statusgen/testdata/milestone-support/03/` (planned), `schemas/milestone-v1.json` (planned), `spec/milestone-v1.md`, `spec/stream-v1.md`, `changelog/milestone-support-03.md` (planned), `statusgen/testdata/milestone-support/03-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen milestone discovery owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen milestone discovery"
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

## Ground rules

- Own isolated branch and draft implementation PR. Do not merge, flip status/spec/stream/PR readiness, dispatch work, cut a tag, change installed pins or contact a cluster.
- Scope approval and stream activation take effect on the maintainer's merge of PR #4182; this brief has no completion evidence. A non-implementer owns post-merge verification; never fill Evidence from authorship or a fixture expectation.
- Refresh main, active claims/PRs, module imports, source contracts and consumer inventory at pickup. Record brief/source revisions, actual public API availability, coverage/unknowns and continue/adapt/hold in the implementation handoff. No obsolete active claim is overwritten.
- Preserve existing credentials, independent human/risk gates, UUIDs, archived evidence and exact-subject meanings. No ambient-auth fallback or automatic bulk migration.
- Shared resolution and capability assessment MUST be direct library calls, not subprocess calls to statusgen, desk-tools or a new helper executable. External I/O remains in existing adapters.

## Task

1. Implement negotiated stream opt-in and milestone discovery using /02's envelope. Parse the canonical record path and closed field/body contract. Full record identity must match its owning root/stream; exactly one record exists per identity/contract-version. Filenames are locators, not version authorities.
2. Use workref for milestone and contribution refs, preserving exact optional brief revisions. Validate criterion IDs, coverage/provenance and prerequisite versions; distinguish an empty contribution list justified by an existing capability from missing/unmapped work. Preserve all legacy non-opted-in streams.
3. Reject duplicate versions, wrong-kind refs, escaping/unreadable/ambiguous locations and alias collisions without hiding known records. Missing required root, source or evidence produces explicit coverage unknown; never treat it as an empty plan. Do not import secrets or cross private/public trust boundaries into projections.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRecords$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRecords ' "$milestone_verify_out")` | exit 0 and named PASS; actual loader accepts canonical and compatible records, derives version from keys, diagnoses duplicate/owner/unknown-key defects; filename-version inference fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneUnknownCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneUnknownCoverage ' "$milestone_verify_out")` | exit 0 and named PASS; an inaccessible root, unreadable record, path escape or missing criterion stays unknown/refused; treating unavailable as empty fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

Verifier pass, 2026-11-02, by a non-implementer (model claude-opus-5-5, tier strong) at merged main fede422ee8a5da893f395a34c62b1a982b61a2ff. This is the first verification of this brief. Delivering change: #5337, merge commit f4dffce678faea6be3c977d0e6b0ade4946f9b2a, first parent 5eb4ad04881243f9414658dae103b7b7d9c48059. No file the delivery changed under statusgen or schemas has changed between that merge and this source. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run 056d54d37fb0, attestation #5352, tool v1.0.70, runtime darwin-arm64). Installed statusgen v1.0.70 on the host and in the image. The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base fede422ee8a5).

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.70, pinned by its linux/arm64 platform manifest digest (Container-Image below; the tag's manifest list is sha256:45926fbe65687db26f02c9b30493dd4bde6cce2702a81c9240554b4d6b52353b), go 1.27.3, git 2.56.0, run on the host's local Docker engine (OrbStack) with --network none, the engine's default seccomp profile, the host uid, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null and a fresh GOCACHE outside the home. The home, its git dir, the receipt and the Go module cache were mounted read-only; only the witness output and Go scratch directories were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Both named tests drive the production loadStreams seam (statusgen/milestones_test.go calls loadStreams at lines 236, 383, 413, 427 and 442). Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match.

Row S1 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | python3 statusgen/testdata/milestone-support/03-mutate.py, against a snapshot of fede422ee8a5 | every manifest plant turns its named test red | exit 0; 19 of 19 manifest cases printed CAUGHT (6 against TestMilestoneRecords, including filename-version-inference, duplicate-version-accepted, owner-not-checked and unknown-key-ignored; 13 against TestMilestoneUnknownCoverage, including missing-dir-as-empty, unreadable-dir-as-empty, unreadable-record-parsed, escape-not-refused and uncovered-criterion-hidden); both named baselines passed first; no case missed; the snapshot was clean afterwards | 2026-11-02 | assay-verifier-app[bot] @ fede422ee8a5 (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumerated over every non-test Go file in the diff of 5eb4ad048812..f4dffce678fa (statusgen/milestones.go, statusgen/load.go, statusgen/model.go, statusgen/parse.go) and the schema schemas/milestone-v1.json. load.go, model.go and parse.go add only a call, struct fields and an opt-in read, with no literal. Literals found: milestoneSchema = "milestone-v1" (statusgen/milestones.go:53); the coverage states "known" and "unknown" (:60-61); the record filename pattern (:165), criterion ID pattern (:166) and ID-or-wave-label title pattern (:167); the closed frontmatter key set (:169-170) and required body sections (:173-174); the contract-version floor version < 1 (:310); the canonical directory path docs/streams/<stream>/milestones (:656) and the ignored README.md index (:239); the empty-cell markers "—", "-" and "none" (:580); and in the schema the schema const (schemas/milestone-v1.json:20), the milestone ID pattern (:25), contract-version minimum 1 (:29) and the contributes and requires-milestones reference patterns (:52, :60). No value is irreversible; each is undone by an edit and a release. A wrong one mis-reads, refuses or wrongly accepts a milestone record. The coverage state names, the empty-cell markers and the README.md exclusion are internal spellings ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — milestoneSchema = "milestone-v1" @ statusgen/milestones.go:53 — spec/milestone-v1.md:81 and :103 name exactly this schema, and schemas/milestone-v1.json:20 pins it as const; the loader, the schema and the spec agree.
- RISK-VALUE: DERIVED — contract-version floor = version < 1 refused @ statusgen/milestones.go:310 — spec/milestone-v1.md:82 and :105 define contract-version as a positive integer, and schemas/milestone-v1.json:29 sets minimum 1; zero and negatives are refused and the version is taken only from this key (:314), as spec/milestone-v1.md:35 requires.
- RISK-VALUE: DERIVED — milestoneFileRe = `^m-[0-9]{2,}-[a-z0-9][a-z0-9-]*\.md$` @ statusgen/milestones.go:165 — spec/milestone-v1.md:354 names records m-<NN>-<slug>.md and :40-41 pins NN to at least two digits; the slug class admits the -v2 suffix spec/milestone-v1.md:35 allows, and a name that does not match leaves coverage unknown rather than hiding a record.
- RISK-VALUE: DERIVED — milestoneCritRe = `^C[0-9]{2,}$` @ statusgen/milestones.go:166 — spec/milestone-v1.md:381 requires criterion IDs of the form C<NN>, with the same at-least-two-digit padding as m-<NN>.
- RISK-VALUE: DERIVED — milestone ID pattern = `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*[:/]m-[0-9]{2,}$` @ schemas/milestone-v1.json:25 — the three segments plus m-<NN> match the full colon-only ID at spec/milestone-v1.md:39-41, and the [:/] alternative admits the earlier slash spelling that spec/milestone-v1.md:58-59 and :365 keep as a compatibility input.
- RISK-VALUE: NAMED, NOT DERIVED — milestoneLabelRe = `(?i)^(m-?[0-9]+|wave[ -]?[0-9]+|w[0-9]+)$` @ statusgen/milestones.go:167 — spec/milestone-v1.md:106 and :376 refuse a title that is only an ID or wave label but give no grammar for a wave label; the w<N>, wave <N> and m<N> spellings are the implementer's reading and the spec offers no list to derive the set from.

This brief is gate: model with every risk answer no; on this PASS, the same branch flips the stream index row 03 from implemented to verified (the verifier makes that flip; CI owns verified to done).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRecords$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRecords ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ fede422ee8a5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneUnknownCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneUnknownCoverage ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ fede422ee8a5 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:50b95da02cf2100dcfdf85f5f03d28396ff13cda4a48a5c31bd11d09a1662cfe
Container-Image: sha256:c556f436ff2789b07b2097d50a9d7b5f1266b09f3c40f51b4f02e52bbac7f217
Verification-Attestation: medici-finance/assay#5352 run=056d54d37fb04a5a8ee8e65c3ec4b8393f3be97152131a93ed88788364a7db4b source=fede422ee8a5da893f395a34c62b1a982b61a2ff brief=docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=056d54d37fb04a5a8ee8e65c3ec4b8393f3be97152131a93ed88788364a7db4b source=fede422ee8a5da893f395a34c62b1a982b61a2ff tree=sha256:50b95da02cf2100dcfdf85f5f03d28396ff13cda4a48a5c31bd11d09a1662cfe witness=sha256:00269f6b7c7ab5327c10759ab9ce360345815a8d90ff10397a8bb1b8d68141d9 opened=2026-11-02T22:34:29Z closed=2026-11-02T22:35:16Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at fede422ee8a5, and 19 of 19 manifest mutations were caught (supplemental, no acceptance credit).
Verifier pass, 2026-11-10, by a non-implementer (model claude-opus-5-5, tier strong) at merged main 612750a12406f19a96a83e6ea7687d794f772f9f. This is a re-verification of a brief already at verified. Delivering change: #5337, merge commit f4dffce678faea6be3c977d0e6b0ade4946f9b2a, first parent 5eb4ad04881243f9414658dae103b7b7d9c48059. No delivered file under statusgen or schemas (milestones.go, its test, load.go, model.go, parse.go, the 03 fixtures and mutation manifest, and schemas/milestone-v1.json) differs between that merge and this source. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run 10829a4c0516, attestation #6038, tool v1.0.81, runtime darwin-arm64). Installed statusgen v1.0.81 on the host and in the image. The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base 612750a12406).

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.81, pinned by the digest in the release's image manifest (Container-Image below), go 1.27.3 linux/arm64, run on the host's local Docker engine (OrbStack) with --network none, --read-only, --cap-drop ALL, no-new-privileges, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null and a fresh module cache filled by the open step (Module-Cache below). The home, its git common dir, the receipt and the module cache were mounted read-only; only the witness output directory and a tmpfs were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Both named tests drive the production loadStreams seam (statusgen/milestones_test.go calls loadStreams at lines 236, 383, 413, 427 and 442). Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match.

Row S2 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none and the same read-only module cache, against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S2 | python3 -I statusgen/testdata/milestone-support/03-mutate.py, against a snapshot of 612750a12406 | every manifest plant turns its named test red | exit 0; 19 of 19 manifest cases printed CAUGHT (6 against TestMilestoneRecords: filename-version-inference, duplicate-version-accepted, owner-not-checked, unknown-key-ignored, wrong-kind-contribution, legacy-stream-read; 13 against TestMilestoneUnknownCoverage, including missing-dir-as-empty, unreadable-dir-as-empty, symlink-followed, unreadable-record-parsed, escape-not-refused, uncovered-criterion-hidden and loader-ungated); both named baselines passed first; no case missed; the snapshot was clean afterwards | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumerated over every non-test Go file in the diff of 5eb4ad048812..f4dffce678fa (statusgen/milestones.go, statusgen/load.go, statusgen/model.go, statusgen/parse.go) and the schema schemas/milestone-v1.json. load.go, model.go and parse.go add only a call, struct fields and an opt-in read, with no literal. Literals found: milestoneSchema = "milestone-v1" (statusgen/milestones.go:53); the coverage states "known" and "unknown" (:60-61); the record filename pattern (:165), criterion ID pattern (:166) and ID-or-wave-label title pattern (:167); the closed frontmatter key set (:169-170) and required body sections (:173-174); the contract-version floor version < 1 (:310); the ignored README.md index (:239); the empty-cell markers "—", "-" and "none" (:580); the canonical directory docs/streams/<stream>/milestones (:656); and in the schema the schema const (schemas/milestone-v1.json:20), the milestone ID pattern (:25), contract-version minimum 1 (:29) and the contributes and requires-milestones reference patterns (:52, :60). No value is irreversible; each is undone by an edit and a release. A wrong one mis-reads, refuses or wrongly accepts a milestone record. The coverage state names, the empty-cell markers and the README.md exclusion are internal spellings ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — milestoneSchema = "milestone-v1" @ statusgen/milestones.go:53 — spec/milestone-v1.md:81 and :103 name exactly this schema, and schemas/milestone-v1.json:20 pins it as const; the loader, the schema and the spec agree.
- RISK-VALUE: DERIVED — contract-version floor = version < 1 refused @ statusgen/milestones.go:310 — spec/milestone-v1.md:82 and :105 define contract-version as a positive integer, and schemas/milestone-v1.json:29 sets minimum 1; the reference patterns at schemas/milestone-v1.json:52 and :60 require the same positive version selector [1-9][0-9]*.
- RISK-VALUE: DERIVED — milestoneFileRe = `^m-[0-9]{2,}-[a-z0-9][a-z0-9-]*\.md$` @ statusgen/milestones.go:165 — spec/milestone-v1.md:35 and :354 name records m-<NN>-<slug>.md and :42 pins NN to at least two digits; the slug class admits the -v2 suffix spec/milestone-v1.md:35 allows, and the mutation filename-version-inference shows the version is not read from the name.
- RISK-VALUE: DERIVED — milestoneCritRe = `^C[0-9]{2,}$` @ statusgen/milestones.go:166 — spec/milestone-v1.md:381 requires criterion IDs of the form C<NN>, with the same at-least-two-digit padding as m-<NN>.
- RISK-VALUE: DERIVED — milestone ID pattern = `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*[:/]m-[0-9]{2,}$` @ schemas/milestone-v1.json:25 — the three segments plus m-<NN> match the full colon-only ID at spec/milestone-v1.md:40-42, and the [:/] alternative admits the earlier slash spelling that spec/milestone-v1.md:58-59 and :365 keep as a compatibility input.
- RISK-VALUE: NAMED, NOT DERIVED — milestoneLabelRe = `(?i)^(m-?[0-9]+|wave[ -]?[0-9]+|w[0-9]+)$` @ statusgen/milestones.go:167 — spec/milestone-v1.md:106 and :376 refuse a title that is only an ID or wave label but give no grammar for a wave label; the w<N>, wave <N> and m<N> spellings are the implementer's reading and the spec offers no list to derive the set from.

This brief is gate: model with every risk answer no. Its stream index row 03 is already verified (since 2026-11-02); this pass flips nothing, and CI owns verified to done.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRecords$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRecords ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneUnknownCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneUnknownCoverage ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6038 run=10829a4c05164da85442095945d65def8f39191abaa503b0a1d7a82685cfe35d source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=10829a4c05164da85442095945d65def8f39191abaa503b0a1d7a82685cfe35d source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:2d7ebd446489fbc04e1700420e3d40dce86c5da03d754b872bc1a239d074eda6 opened=2026-11-10T11:09:57Z closed=2026-11-10T11:10:41Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at 612750a12406, and 19 of 19 manifest mutations were caught (supplemental, no acceptance credit).

Verifier pass, 2026-11-11, by a non-implementer (model claude-opus-5-5, tier strong) at merged main d0809b9691ed210a8e27ff1147457b9c91f6bffc. This is a re-verification of a brief already at verified. Delivering change: #5337, merge commit f4dffce678faea6be3c977d0e6b0ade4946f9b2a, first parent 5eb4ad04881243f9414658dae103b7b7d9c48059. No delivered file under statusgen or schemas (milestones.go, its test, load.go, model.go, parse.go, the 03 fixtures and mutation manifest, the shared 01 mutation runner and schemas/milestone-v1.json) differs between that merge and this source. The delivered spec/milestone-v1.md did change after that merge: later milestone-support commits inserted 140 lines after its line 401. Every spec line cited below sits above that insert and reads the same, and the spec file is unchanged since the previous pass at 612750a12406. Since the previous pass at 612750a12406 only the milestone-support/08 mutation files changed under the milestone-support testdata directory. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run 36f9f5c02b00, attestation #6220, tool v1.0.82, runtime darwin-arm64). Installed statusgen v1.0.82 on the host and in the image. The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base d0809b9691ed).

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.82, pinned by the image index digest the v1.0.82 release records in its verify-runner image reference (Container-Image below; the reference file's sha256 matches the release checksums), go 1.27.3 linux/arm64, git 2.56.0, run on the host's local Docker engine (OrbStack) with --network none, --read-only, --cap-drop ALL, no-new-privileges, the host uid, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null and a fresh module cache filled by the open step (Module-Cache below). The home, its git common dir, the receipt and the module cache were mounted read-only; only the witness output directory and a tmpfs were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Both named tests drive the production loadStreams seam (statusgen/milestones_test.go calls loadStreams at lines 236, 383, 413, 427 and 442). Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match.

Row S3 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none and the same read-only module cache, against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S3 | python3 -I statusgen/testdata/milestone-support/03-mutate.py, against a snapshot of d0809b9691ed | every manifest plant turns its named test red | exit 0; 19 of 19 manifest cases printed CAUGHT (6 against TestMilestoneRecords: filename-version-inference, duplicate-version-accepted, owner-not-checked, unknown-key-ignored, wrong-kind-contribution, legacy-stream-read; 13 against TestMilestoneUnknownCoverage: missing-dir-as-empty, unreadable-dir-as-empty, symlink-followed, unreadable-record-parsed, escape-not-refused, missing-evidence-assumed, uncovered-criterion-hidden, unmapped-empty-contributes, prerequisite-assumed, keep-refused-record, keep-refused-source, keep-refused-evidence, loader-ungated); both named baselines passed first; no case missed; the snapshot was clean afterwards | 2026-11-11 | assay-verifier-app[bot] @ d0809b9691ed (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumerated over every non-test Go file in the diff of 5eb4ad048812..f4dffce678fa (statusgen/milestones.go, statusgen/load.go, statusgen/model.go, statusgen/parse.go) and the schema schemas/milestone-v1.json, each re-read at d0809b9691ed with unchanged line numbers. load.go, model.go and parse.go add only a call, struct fields and an opt-in read, with no literal. Literals found: milestoneSchema = "milestone-v1" (statusgen/milestones.go:53); the coverage states "known" and "unknown" (:60-61); the record filename pattern (:165), criterion ID pattern (:166) and ID-or-wave-label title pattern (:167); the closed frontmatter key set (:169-170) and required body sections (:173-174); the ignored README.md index (:239); the contract-version floor version < 1 (:310); the empty-cell markers "—", "-" and "none" (:580); the canonical directory docs/streams/<stream>/milestones (:656); and in the schema the schema const (schemas/milestone-v1.json:20), the milestone ID pattern (:25), contract-version minimum 1 (:29) and the contributes and requires-milestones reference patterns (:52, :60). No value is irreversible; each is undone by an edit and a release. A wrong one mis-reads, refuses or wrongly accepts a milestone record. The coverage state names, the empty-cell markers and the README.md exclusion are internal spellings ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — milestoneSchema = "milestone-v1" @ statusgen/milestones.go:53 — spec/milestone-v1.md:81 and :103 name exactly this schema, and schemas/milestone-v1.json:20 pins it as const; the loader, the schema and the spec agree.
- RISK-VALUE: DERIVED — contract-version floor = version < 1 refused @ statusgen/milestones.go:310 — spec/milestone-v1.md:82 and :105 define contract-version as a positive integer, and schemas/milestone-v1.json:29 sets minimum 1; the reference patterns at schemas/milestone-v1.json:52 and :60 require the same positive version selector [1-9][0-9]*.
- RISK-VALUE: DERIVED — milestoneFileRe = `^m-[0-9]{2,}-[a-z0-9][a-z0-9-]*\.md$` @ statusgen/milestones.go:165 — spec/milestone-v1.md:35 and :354 name records m-<NN>-<slug>.md and :42 pins NN to at least two digits; the slug class admits the -v2 suffix spec/milestone-v1.md:35 allows, and the mutation filename-version-inference shows the version is not read from the name.
- RISK-VALUE: DERIVED — milestoneCritRe = `^C[0-9]{2,}$` @ statusgen/milestones.go:166 — spec/milestone-v1.md:381 requires criterion IDs of the form C<NN>, with the same at-least-two-digit padding as m-<NN>.
- RISK-VALUE: DERIVED — milestone ID pattern = `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*[:/]m-[0-9]{2,}$` @ schemas/milestone-v1.json:25 — the three segments plus m-<NN> match the full colon-only ID at spec/milestone-v1.md:40-42, and the [:/] alternative admits the earlier slash spelling that spec/milestone-v1.md:58-59 and :365 keep as a compatibility input.
- RISK-VALUE: NAMED, NOT DERIVED — milestoneLabelRe = `(?i)^(m-?[0-9]+|wave[ -]?[0-9]+|w[0-9]+)$` @ statusgen/milestones.go:167 — spec/milestone-v1.md:106 and :376 refuse a title that is only an ID or wave label but give no grammar for a wave label; the w<N>, wave <N> and m<N> spellings are the implementer's reading and the spec offers no list to derive the set from.

This brief is gate: model with every risk answer no. Its stream index row 03 is already verified (since 2026-11-02); this pass flips nothing, and CI owns verified to done.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRecords$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRecords ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-11 | assay-verifier-app[bot] @ d0809b9691ed (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneUnknownCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneUnknownCoverage ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-11 | assay-verifier-app[bot] @ d0809b9691ed (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9bcac3a5db2108cdd91a6af84c9ac32b3d3356eff8839ceeeb0abe1177946a4e
Container-Image: sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6220 run=36f9f5c02b00abfb05640fdfd5f7b0e47afe0fb08bf60d01607f832ce9074265 source=d0809b9691ed210a8e27ff1147457b9c91f6bffc brief=docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=36f9f5c02b00abfb05640fdfd5f7b0e47afe0fb08bf60d01607f832ce9074265 source=d0809b9691ed210a8e27ff1147457b9c91f6bffc tree=sha256:9bcac3a5db2108cdd91a6af84c9ac32b3d3356eff8839ceeeb0abe1177946a4e witness=sha256:048a4ac4e78ca841c2eb20d25100d9cabb5b687845c07147931b4228f5fa0b28 opened=2026-11-11T23:18:25Z closed=2026-11-11T23:19:29Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at d0809b9691ed, and 19 of 19 manifest mutations were caught (supplemental, no acceptance credit).
Verifier pass, 2026-11-13, by a non-implementer (model claude-opus-5-5, tier strong) at merged main 5d0fa120d3da3702ac3a717f9ed9fa68f7a6ca10. This is a re-verification of a brief already at verified. Delivering change: #5337, merge commit f4dffce678faea6be3c977d0e6b0ade4946f9b2a, first parent 5eb4ad04881243f9414658dae103b7b7d9c48059. Since that merge, the only delivered file under statusgen or schemas that differs is statusgen/model.go: the milestone-support/07 delivery added an evaluation import and one Stream field, MilestoneReadiness, with no literal. milestones.go, its test, load.go, parse.go, the 03 fixtures and mutation manifest, the shared 01 mutation runner and schemas/milestone-v1.json are unchanged. The delivered spec/milestone-v1.md changed after the previous pass at d0809b9691ed only in hunks from its line 425 down, and every spec line cited below sits above them and reads the same. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run 2177d17d7302, attestation #6342, tool v1.0.83, runtime darwin-arm64). Installed statusgen v1.0.83 on the host and in the image. The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base 5d0fa120d3da).

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.83, pinned by digest (Container-Image below), go 1.27.3 linux/arm64, git 2.56.0, run on the host's local Docker engine (OrbStack) with --network none, --read-only, --cap-drop ALL, no-new-privileges, the host uid, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null and a fresh module cache filled by the open step (Module-Cache below). The home, its git common dir, the receipt and the module cache were mounted read-only; only the witness output directory and a tmpfs were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Both named tests drive the production loadStreams seam (statusgen/milestones_test.go calls loadStreams at lines 236, 383, 413, 427 and 442). Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match. This dispatch made one attempt at the table, and it is the credited one.

Row S4 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none and the same read-only module cache, against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S4 | python3 -I statusgen/testdata/milestone-support/03-mutate.py, against a snapshot of 5d0fa120d3da | every manifest plant turns its named test red | exit 0; 19 of 19 manifest cases printed CAUGHT (6 against TestMilestoneRecords: filename-version-inference, duplicate-version-accepted, owner-not-checked, unknown-key-ignored, wrong-kind-contribution, legacy-stream-read; 13 against TestMilestoneUnknownCoverage: missing-dir-as-empty, unreadable-dir-as-empty, symlink-followed, unreadable-record-parsed, escape-not-refused, missing-evidence-assumed, uncovered-criterion-hidden, unmapped-empty-contributes, prerequisite-assumed, keep-refused-record, keep-refused-source, keep-refused-evidence, loader-ungated); both named baselines passed first; no case missed; the snapshot was clean afterwards | 2026-11-13 | assay-verifier-app[bot] @ 5d0fa120d3da (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumerated over every non-test Go file in the diff of 5eb4ad048812..f4dffce678fa (statusgen/milestones.go, statusgen/load.go, statusgen/model.go, statusgen/parse.go) and the schema schemas/milestone-v1.json; milestones.go and the schema were re-read at 5d0fa120d3da with unchanged line numbers, and the later model.go change adds no literal. load.go, model.go and parse.go add only a call, struct fields and an opt-in read, with no literal. Literals found: milestoneSchema = "milestone-v1" (statusgen/milestones.go:53); the coverage states "known" and "unknown" (:60-61); the record filename pattern (:165), criterion ID pattern (:166) and ID-or-wave-label title pattern (:167); the closed frontmatter key set (:169-170) and required body sections (:173-174); the ignored README.md index (:239); the contract-version floor version < 1 (:310); the empty-cell markers "—", "-" and "none" (:580); the canonical directory docs/streams/<stream>/milestones (:656); and in the schema the schema const (schemas/milestone-v1.json:20), the milestone ID pattern (:25), contract-version minimum 1 (:29) and the contributes and requires-milestones reference patterns (:52, :60). No value is irreversible; each is undone by an edit and a release. A wrong one mis-reads, refuses or wrongly accepts a milestone record. The coverage state names, the empty-cell markers and the README.md exclusion are internal spellings ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — milestoneSchema = "milestone-v1" @ statusgen/milestones.go:53 — spec/milestone-v1.md:81 and :103 name exactly this schema, and schemas/milestone-v1.json:20 pins it as const; the loader, the schema and the spec agree.
- RISK-VALUE: DERIVED — contract-version floor = version < 1 refused @ statusgen/milestones.go:310 — spec/milestone-v1.md:82 and :105 define contract-version as a positive integer, and schemas/milestone-v1.json:29 sets minimum 1; the reference patterns at schemas/milestone-v1.json:52 and :60 require the same positive version selector [1-9][0-9]*.
- RISK-VALUE: DERIVED — milestoneFileRe = `^m-[0-9]{2,}-[a-z0-9][a-z0-9-]*\.md$` @ statusgen/milestones.go:165 — spec/milestone-v1.md:35 and :354 name records m-<NN>-<slug>.md and :42 pins NN to at least two digits; the slug class admits the -v2 suffix spec/milestone-v1.md:35 allows, and the mutation filename-version-inference shows the version is not read from the name.
- RISK-VALUE: DERIVED — milestoneCritRe = `^C[0-9]{2,}$` @ statusgen/milestones.go:166 — spec/milestone-v1.md:381 requires criterion IDs of the form C<NN>, with the same at-least-two-digit padding as m-<NN>.
- RISK-VALUE: DERIVED — milestone ID pattern = `^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*[:/]m-[0-9]{2,}$` @ schemas/milestone-v1.json:25 — the three segments plus m-<NN> match the full colon-only ID at spec/milestone-v1.md:40-42, and the [:/] alternative admits the earlier slash spelling that spec/milestone-v1.md:58-59 and :365 keep as a compatibility input.
- RISK-VALUE: NAMED, NOT DERIVED — milestoneLabelRe = `(?i)^(m-?[0-9]+|wave[ -]?[0-9]+|w[0-9]+)$` @ statusgen/milestones.go:167 — spec/milestone-v1.md:106 and :376 refuse a title that is only an ID or wave label but give no grammar for a wave label; the w<N>, wave <N> and m<N> spellings are the implementer's reading and the spec offers no list to derive the set from.

This brief is gate: model with every risk answer no. Its stream index row 03 is already verified (since 2026-11-02); this pass flips nothing, and CI owns verified to done.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRecords$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRecords ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 5d0fa120d3da (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneUnknownCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneUnknownCoverage ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 5d0fa120d3da (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:00bd341253e4395ec9ef56840e2ab24fb2035f6523ec58facee18521cbd7f2e9
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6342 run=2177d17d7302fc2b7aa234ab38835cf878bfbb0f1f3ad1c1ee2895b64d35ef6a source=5d0fa120d3da3702ac3a717f9ed9fa68f7a6ca10 brief=docs/streams/milestone-support/brief-03-load-and-validate-versioned-milestone-records.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=2177d17d7302fc2b7aa234ab38835cf878bfbb0f1f3ad1c1ee2895b64d35ef6a source=5d0fa120d3da3702ac3a717f9ed9fa68f7a6ca10 tree=sha256:00bd341253e4395ec9ef56840e2ab24fb2035f6523ec58facee18521cbd7f2e9 witness=sha256:d9f4338f54dfff7867a566aa954494360018a335d39a04028ad2919fb98b55af opened=2026-11-13T01:29:57Z closed=2026-11-13T01:30:36Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at 5d0fa120d3da, and 19 of 19 manifest mutations were caught (supplemental, no acceptance credit).

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
