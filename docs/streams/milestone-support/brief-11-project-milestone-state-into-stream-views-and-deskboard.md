---
brief: assay:assay:milestone-support:11
title: "Project milestone state into stream views and deskboard"
why: "People and agents need the same clear outcome state without hand-maintained membership or a false done percentage."
wave: 6
depends: ["milestone-support/07", "milestone-support/08"]
unblocks: ["milestone-support/13"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 1a6cf437-d216-400a-b596-efcc16ab42c0
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
  - "statusgen/streamview/: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "statusgen/main.go: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "statusgen/readmetable.go: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "tools/desk/cmd/deskboard/: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "docs/streams/README.md: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "docs/contracts.md: fixed-here (census: docs/streams/milestone-support/11-consumers.md)"
  - "docs/streams/milestone-support/brief-13-trace-and-report-a-selected-milestone-through-canonical-readers.md: follow-up milestone-support/13 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 11 — Project milestone state into stream views and deskboard

## Context

files: `statusgen/streamview/`, `statusgen/main.go`, `statusgen/readmetable.go`, `tools/desk/cmd/deskboard/`, `docs/streams/README.md`, `docs/contracts.md`, `changelog/milestone-support-11.md` (planned), `statusgen/testdata/milestone-support/11-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen/streamview and deskboard owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen/streamview and deskboard"
  contract: none — existing index does not yet name this exact typed record/projection; update its existing owner row in the implementation, not a second registry.
  retires: ["duplicated interpretation or implicit state inference at the named existing consumer boundary"]
  weight: "New binaries 0; new services 0; required pure API/schema/fixture changes only. Measure actual rule/flag/refusal deltas at implementation review; no current weight reduction is claimed."
  why-add: "Reuse the existing semantic owner and public module. This records a missing shared meaning; moving it into a helper process or copying another parser would add authority or drift without benefit."

risk-path disposition: deskboard changes are read-only projections/fixtures. They introduce no credential, admission, execution or ruling-policy changes; actual acceptance/dispatch remains at the independently gated existing owners. The declared model gate is intentional for this bounded slice.

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

1. Version the existing stream-view wire contract as required and add generated milestone summaries/reverse membership from canonical reader/evaluator results. Display supported scope, work remaining, acceptance outstanding, current availability, prerequisites/holds and evidence links.
2. Give milestone projections a supported separately marked generated region, preserving authored prose and the existing Briefs table ownership. Add no manually maintained status, inverse membership or fake milestones for fixture/outline streams. Leave non-opted-in views unchanged.
3. Make deskboard consume the versioned projection and refuse unsupported required wire semantics. Render source coverage and unknowns explicitly; escape untrusted text and do not leak inaccessible/private locators. Legacy aliases do not duplicate rows or change approval state.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneStreamView$' ./streamview/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneStreamView ' "$milestone_verify_out")` | exit 0 and named PASS; versioned projection preserves scope/holds/acceptance distinction and old view compatibility; dropping a mandatory unknown field fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskboardMilestones$' ./cmd/deskboard/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestDeskboardMilestones ' "$milestone_verify_out")` | exit 0 and named PASS; actual board displays zero work with acceptance outstanding truthfully, deduplicates aliases and escapes hostile labels; renderer-only done inference fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

Verifier pass, 2026-11-21, by a non-implementer (model claude-opus-5-5, tier strong) at merged main 2aafcfe485c74626db401b8209155f215015cc11. This is the brief's first verification. Delivering change: #7283, merge commit faa3490b391ff44f0f26a9df0aff7f6137d83b74 (on the first-parent chain of this source), first parent 2c7bfb0712cb67b68d206cbe0d13744ec92f1bb5. None of the delivered statusgen, tools/desk/cmd/deskboard, mutation-manifest or stream-view contract files differs between that merge and this source. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run d556b1fd7ef2, attestation #7358, tool v1.0.90, runtime darwin-arm64, capability verifier-attestation=checked-clean). The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive exit 0, base 2aafcfe485c7); the delivery did not change the Verify section, which equals the one at the merge's first parent.

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.90, pinned by the image index digest the house image lock names (Container-Image below), run on the host's local Docker engine with --network none, --read-only, tmpfs scratch, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null and a fresh module cache filled by the open step (Module-Cache below). The home, the receipt and the module cache were mounted read-only; only the witness output directory and the tmpfs were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Row 1's named test drives the public stream-view Decode and Validate API (statusgen/streamview/milestones_test.go lines 56, 73, 116-127: v2 round trip, v1 compatibility, v3 and unknown-field refusal). Row 2's named test drives deskboard's run entrypoint (tools/desk/cmd/deskboard/milestones_test.go line 107). Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match (bracket=L9 header=L1). This dispatch made one attempt at the table, and it is the credited one.

Row S1 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none and the same read-only module cache, against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | python3 -I statusgen/testdata/milestone-support/11-mutate.py, against a snapshot of 2aafcfe485c7 | every manifest plant turns its named test red | exit 0; 21 of 21 manifest cases printed CAUGHT (4 against TestMilestoneStreamView: unknown-coverage-lists-no-unknown, supported-without-acceptance, outstanding-with-work-remaining, unknown-work-carries-count; 5 against TestMilestoneStreamProjection: acceptance-inferred-from-zero-work, unknown-coverage-reported-known, non-opted-in-stream-gets-milestones, v2-offered-to-every-stream, membership-not-generated; 3 against TestMilestoneReadmeRegion: region-written-for-non-opted-in-stream, hand-edit-not-detected, cell-text-not-escaped; 2 against TestMilestoneRegionOverlap: region-overlap-guard-removed, region-begin-inside-briefs-accepted; 7 against TestDeskboardMilestones: renderer-infers-done-from-zero-work, alias-does-not-collapse, alias-row-supplies-state, private-locator-shown, inaccessible-view-rendered, unsupported-contract-skipped, v1-view-read-as-assessed); all five named baselines passed first; no case missed; the snapshot was clean afterwards (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible; its risk-path disposition states the deskboard change is a read-only projection. Enumerated over every non-test Go file in the diff of 2c7bfb0712cb..faa3490b391f (statusgen/streamview/contract.go, statusgen/streamview/bindings.go, statusgen/streamview/milestones.go, statusgen/streammilestones.go, statusgen/readmemilestones.go, statusgen/regen.go, statusgen/main.go, tools/desk/cmd/deskboard/milestones.go, tools/desk/cmd/deskboard/main.go, tools/desk/askassay/probe.go). bindings.go, regen.go, main.go, deskboard main.go and probe.go add only calls, a section name and verb-list entries, with no literal. Literals found: Version2 = "stream-view/v2" (statusgen/streamview/contract.go:33) and SupportedVersions order v2 then v1; the coverage and work states "known"/"unknown" (statusgen/streamview/milestones.go:27-34); the acceptance states accepted, outstanding, not-due, could-not-check (:40-43); the availability states supported, blocked, unknown (:48-50); the hold-kind set (:53); the README region markers (statusgen/readmemilestones.go:32-33); the not-opted-in reason text (statusgen/streammilestones.go:30); maxStreamViewBytes = 4 << 20 (tools/desk/cmd/deskboard/milestones.go:38); withheldLocator = "[withheld]" (:40); the evidence-URL gate Scheme "https" and Host "github.com" (:316); and display truncation widths 40, 80, 120 and 160 in the renderer. No value is irreversible; each is undone by an edit and a release. A wrong one mis-states, refuses or leaks a milestone projection. The state spellings, reason text, truncation widths and the 4 MiB read cap are reversible display or operational bounds ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — Version2 = "stream-view/v2" @ statusgen/streamview/contract.go:33 — Task 1 requires the existing wire contract to be versioned; docs/stream-view-contract.md:15 and :305 name exactly this value, v1 stays decodable (SupportedVersions lists both), and the S1 case v2-offered-to-every-stream shows non-opted-in streams stay on v1.
- RISK-VALUE: DERIVED — acceptance states = accepted | outstanding | not-due | could-not-check @ statusgen/streamview/milestones.go:40-43 — spec/milestone-v1.md:190-194 requires the view to distinguish work remaining (not-due), work complete with acceptance outstanding, and accepted for a subject; could-not-check is the three-state rule's third state, and :263 and :283 require outstanding to stay visible at zero remaining work, which the validator at :211 enforces.
- RISK-VALUE: DERIVED — validHoldKinds = {work, coverage, cycle, prerequisite, acceptance} @ statusgen/streamview/milestones.go:53 — each kind is a blocking category spec/milestone-v1.md names: work remaining (:192), acceptance outstanding (:193), prerequisites (§6), cycles (:173-176) and incomplete coverage (:259); docs/stream-view-contract.md:218 lists the same set.
- RISK-VALUE: DERIVED — evidence URL gate = Scheme "https" and Host "github.com" plus allow-listed owner/repo @ tools/desk/cmd/deskboard/milestones.go:316-317 — Task 3 forbids leaking inaccessible or private locators; a URL is shown only when it names a repository the board's allow-list admits, every other locator falls through to withheldLocator (:322), and the S1 case private-locator-shown is caught.
- RISK-VALUE: DERIVED — milestonesMarkerBegin/End = "<!-- statusgen:milestones:begin -->" / "<!-- statusgen:milestones:end -->" @ statusgen/readmemilestones.go:32-33 — Task 2 requires a separately marked generated region; docs/streams/README.md:18 names exactly these markers beside the existing statusgen:briefs region, and the S1 cases region-overlap-guard-removed and region-begin-inside-briefs-accepted are caught.

This brief is gate: model with every risk answer no. The desk owns the row-scoped landing of its stream index row; CI owns verified to done.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneStreamView$' ./streamview/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneStreamView ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-21 | assay-verifier-app[bot] @ 2aafcfe485c7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskboardMilestones$' ./cmd/deskboard/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestDeskboardMilestones ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-21 | assay-verifier-app[bot] @ 2aafcfe485c7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:38e3c6a36a474084ebbaad312458e9ebb4434734eaa3abe7f4d99a511de35e44
Container-Image: sha256:4e8e3c80a075c552de0966c886784c6020153c27e624f8a1ce988aa3b0590217
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7358 run=d556b1fd7ef23da055f01c1ab7864412e1924c353a12ebf3166fea4669063d2e source=2aafcfe485c74626db401b8209155f215015cc11 brief=docs/streams/milestone-support/brief-11-project-milestone-state-into-stream-views-and-deskboard.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d556b1fd7ef23da055f01c1ab7864412e1924c353a12ebf3166fea4669063d2e source=2aafcfe485c74626db401b8209155f215015cc11 tree=sha256:38e3c6a36a474084ebbaad312458e9ebb4434734eaa3abe7f4d99a511de35e44 witness=sha256:34208d4635e4fadea53bc2b05f5b523724bccf71073426b7f8ff04f16ed3fd4a opened=2026-11-21T16:57:01Z closed=2026-11-21T16:58:27Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at 2aafcfe485c7, and 21 of 21 manifest mutations were caught (supplemental, no acceptance credit).

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
