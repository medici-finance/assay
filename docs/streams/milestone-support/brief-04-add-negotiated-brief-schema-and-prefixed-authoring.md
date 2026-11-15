---
brief: assay:assay:milestone-support:04
title: "Add negotiated brief schema and prefixed authoring"
why: "New brief references and milestone start requirements need a real schema and safe generator rather than tolerated unknown keys."
wave: 2
depends: ["milestone-support/01", "milestone-support/02"]
unblocks: ["milestone-support/05", "milestone-support/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: add5e7f2-1524-4cdb-b4c2-f035eeb32f26
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
  - "statusgen/briefv2.go: out-of-scope (the strict brief-v2 parser is preserved unchanged for v1-v3; brief-v4 parsing is added beside it in statusgen/briefv4.go)"
  - "statusgen/brieffile.go: fixed-here"
  - "statusgen/briefschemaversions.go: fixed-here"
  - "statusgen/newbrief.go: fixed-here"
  - "statusgen/newbrief_test.go: fixed-here"
  - "statusgen/conform_test.go: fixed-here"
  - "statusgen/testdata/milestone-support/04/: fixed-here"
  - "schemas/brief-v4.json: fixed-here"
  - "spec/brief-v1.md: fixed-here"
  - "docs/contracts.md: fixed-here"
  - "docs/streams/milestone-support/brief-05-build-the-combined-milestone-and-brief-dependency-graph.md: follow-up milestone-support/05 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md: follow-up milestone-support/08 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 04 — Add negotiated brief schema and prefixed authoring

## Context

files: `statusgen/briefv2.go`, `statusgen/brieffile.go`, `statusgen/briefschemaversions.go`, `statusgen/newbrief.go`, `statusgen/newbrief_test.go`, `statusgen/conform_test.go`, `statusgen/testdata/milestone-support/04/` (planned), `schemas/brief-v4.json` (planned), `spec/brief-v1.md`, `docs/contracts.md`, `changelog/milestone-support-04.md` (planned), `statusgen/testdata/milestone-support/04-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen brief parsing and newbrief owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen brief parsing and newbrief"
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

1. Allocate the next brief schema only after rechecking the supported-schema table and parallel work; brief-v4 is the current proposal, not an already shipped version. Extend validator, descriptive JSON schema and capability/version coverage together. Preserve v1-v3 validation and existing fullIds.
2. Implement full colon-only b- definition IDs, separate authored version, explicit requires-milestones, and optional exact brief revision selectors on references using workref. Bind an unversioned ref to the selected source snapshot; unavailable requested versions remain unknown. Do not mint a UUID or task for an alias/version selector.
3. Extend newbrief and stream scaffolding through the existing front door. Select the supporting identifier/schema pair only after /02 validates every required consumer. Unsupported output must refuse or use an explicitly selected supported legacy mode; never silently drop a requested gate. Keep filenames, source provenance, risk derivation, dependency inverses and existing generated cells intact.
4. Provide cross-schema and old-reader fixtures. No corpus migration, operational declaration, spec-state flip or supporting-release number is fabricated in this implementation.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedBriefSchema$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedBriefSchema ' "$milestone_verify_out")` | exit 0 and named PASS; actual parser validates new IDs/requirements, legacy forms still read once, duplicate UUID aliases and unavailable versions refuse/are unknown; a parser accepting a mismatched schema fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewbriefCaps$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNewbriefCaps ' "$milestone_verify_out")` | exit 0 and named PASS; actual generator writes a valid ID/schema pair and inverses; unsupported required consumers cause zero writes, even with a tolerant YAML decoder. |

## Notes

Known limits recorded by the milestone-support/04 follow-up. Neither is fixed there, because each needs a design choice rather than a few obviously-correct lines.

- A scoped lint run (`--changed` / `--scope`) builds the per-root brief snapshot only from the validated stream subset, while a full run builds it from every stream. A `depends:` reference pinned with `@<version>` to a brief in a stream outside the subset therefore binds as unknown (a notice plus a could-not-check gate) in a scoped run, where a full run would resolve it. Resolving it would need the snapshot to be built from the full stream set, which changes what a scoped run reads.
- A brief-v2 record that omits `version:` is read as revision 1, so an `@1` pin binds it as though revision 1 had been authored. Only brief-v1 (no `version:` field at all) is treated as carrying no authored revision. Telling an authored revision from a defaulted one would need the parser to record whether the field was present.

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

Verifier pass, 2026-11-02, by a non-implementer (model claude-opus-5-5, tier strong) at merged main cd08e97833cbf31395e302adf8922fe6b0bde137. This is the first verification of this brief. Delivering change: #5340, merge commit 0dbff54ce38e3d1e76d538d2c29502371bf34485, first parent fbd80a540b3bf8b039296f6ee67eb60c718c62db. No file the delivery changed has changed between that merge and this source. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run af2b32499660, attestation #5345, tool v1.0.70, runtime darwin-arm64). Installed statusgen v1.0.70 on the host and in the image. The Verify table re-derived with 2 rows and 0 author-added (deskpathguard rederive, base cd08e97833cb).

Rows 1-2 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.70, pinned by its linux/arm64 platform manifest digest (Container-Image below; the tag's manifest list is sha256:45926fbe65687db26f02c9b30493dd4bde6cce2702a81c9240554b4d6b52353b), go 1.27.3, git 2.56.0, run on the host's local Docker engine (OrbStack) with --network none, the engine's default seccomp profile, the host uid, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null and a fresh GOCACHE outside the home. The home, its git dir, the receipt and the Go module cache were mounted read-only; only the witness output and Go scratch directories were writable, both outside the home. No forge credential entered the container. Each row's output hash is the empty-output digest because the command sends its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. Neither row carries a risk-bearing tag and the brief answers every risk question no, so no fail-first base witness applies. The close step re-admitted the run and --bracket-digest reports verdict=match.

Row S1 is supplemental and carries no acceptance credit. It ran the brief's declared mutation manifest in the same image with --network none against a tracked-file snapshot of this source outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | python3 statusgen/testdata/milestone-support/04-mutate.py, against a snapshot of cd08e97833cb | every manifest plant turns its named test red | exit 0; 10 of 10 manifest cases printed CAUGHT (6 against TestPrefixedBriefSchema, 4 against TestNewbriefCaps); both named baselines passed first; no case missed; the snapshot was clean afterwards | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumerated over every non-test Go file in the diff of fbd80a540b3b..0dbff54ce38e (statusgen/briefv4.go, statusgen/newbriefv4.go, statusgen/brieffile.go, statusgen/newbrief.go, statusgen/briefschemaversions.go, statusgen/versiongate.go, statusgen/conform.go, statusgen/withdrawn.go, statusgen/briefinfo.go), plus the schema statusgen/schemas/brief-v4.json (byte-identical to schemas/brief-v4.json) and the spec. Literals found: the schema id "brief-v4", its first-release "" entry, the brief-id and milestone-reference patterns, the unversioned sentinels Revision == 0 and Version == 0 (statusgen/briefv4.go:253 and :256), the edge types "behavioural-gate" (:279) and "build-dep" (:296), the required operation "syntax" (statusgen/newbriefv4.go:57) and the selector words "" and "legacy" (statusgen/newbriefv4.go:28-29). No value is irreversible; each is undone by an edit and a release. A wrong one mis-reads or mis-writes a brief record. The sentinels, edge-type names and selector words are internal spellings ranked last. Ranked top entries:

- RISK-VALUE: DERIVED — briefSchemaV4 = "brief-v4" @ statusgen/brieffile.go:303 — spec/brief-v1.md:169 allocates exactly this schema name, and the shipped schema pins it as const at schemas/brief-v4.json:27; the parser, the embedded schema and the spec agree.
- RISK-VALUE: DERIVED — briefSchemaFirstRelease["brief-v4"] = "" @ statusgen/briefschemaversions.go:31 — spec/brief-v1.md:169 lists brief-v4 as unreleased, and the brief's Task 4 forbids fabricating a supporting-release number, so empty is the only correct value until a release cut fills it.
- RISK-VALUE: DERIVED — requires-milestones item pattern = "^([a-z0-9][a-z0-9-]*:){1,3}m-[0-9]{2,}@[1-9][0-9]*$" @ schemas/brief-v4.json:186 — the {1,3} prefix count matches the three colon-only forms in spec/milestone-v1.md:49-51, and @[1-9][0-9]* matches the positive-integer contract-version in spec/milestone-v1.md:82.
- RISK-VALUE: NAMED, NOT DERIVED — brief id pattern = "^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:b-[0-9]{2,4}[a-z]?$" @ schemas/brief-v4.json:32 — the four-segment colon-only b- shape matches spec/brief-v1.md:312, but the spec writes only <NN> and names no source for the 2-4 digit bound or the optional letter suffix. The schema is descriptive and the validator owns the semantic rule, so a too-loose bound is caught there.

This brief is gate: model with every risk answer no; on this PASS, the same branch flips the stream index row 04 from implemented to verified (the verifier makes that flip; CI owns verified to done).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedBriefSchema$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedBriefSchema ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewbriefCaps$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNewbriefCaps ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ cd08e97833cb (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:646b4e70934f4e4fd4018087336bea3f81a6aff3dec573f40fd5bf07adc7887e
Container-Image: sha256:c556f436ff2789b07b2097d50a9d7b5f1266b09f3c40f51b4f02e52bbac7f217
Verification-Attestation: medici-finance/assay#5345 run=af2b324996608071a47e15dcebf6d3834646fea725818bd099cfa870c5ab8f61 source=cd08e97833cbf31395e302adf8922fe6b0bde137 brief=docs/streams/milestone-support/brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=af2b324996608071a47e15dcebf6d3834646fea725818bd099cfa870c5ab8f61 source=cd08e97833cbf31395e302adf8922fe6b0bde137 tree=sha256:646b4e70934f4e4fd4018087336bea3f81a6aff3dec573f40fd5bf07adc7887e witness=sha256:6ebe2f6c488d504415db652884a34abc22cae286304d0aa3c5a99cce9f138e28 opened=2026-11-02T21:55:06Z closed=2026-11-02T21:56:33Z before=receipt after=check-verifier

**VERIFY: PASS** — rows 1-2 pass in one closed container bracket (verdict=match) at cd08e97833cb, and 10 of 10 manifest mutations were caught (supplemental, no acceptance credit).
### Re-verification by a non-implementer — 2026-11-10 at 612750a12406 (claude-opus-5-5, tier strong)

This run re-verifies the brief at merged main 612750a12406f19a96a83e6ea7687d794f772f9f. The earlier pass above was taken at cd08e97833cb. That source predates the follow-up PR #5347 (merge 29387eb13de7b1f2dadb8dc2a08f4d7834752600), which changed the brief-v4 parser, both schema copies, the spec, the named test file and the mutation manifest. The original delivery is #5340 (merge 0dbff54ce38e3d1e76d538d2c29502371bf34485). Since the follow-up, the only change to this brief's delivered files is #5379 (milestone-support/05, merge 178eab638). It moved the milestone start gate out of the brief-v4 parser and into the combined milestone graph adapter.

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row ran (attestation #6039, tool v1.0.81, runtime darwin-arm64). deskpathguard rederive found 2 rows and 0 author-added rows, at base 612750a12406. Both rows match the Verify table.

Rows 1 and 2 (check:ci) ran in one closed container bracket. The image was verify-runner v1.0.81, pinned by the digest in that release's verify-runner-image.txt (its checksum matches checksums.txt). The container ran with --network none, --read-only, all capabilities dropped, no-new-privileges, the host uid, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null, a tmpfs TMPDIR and a fresh module cache that the open step provisioned. The home, its git dir, the receipt and the module cache were mounted read-only. No forge credential entered the container. Each row prints the empty-output digest because the command writes its test output to a temporary file and checks it with grep -q, so exit 0 is the named-PASS result. The bracket opened at 11:09:54Z and closed at 11:11:53Z. --bracket-digest reports verdict=match. Before landing, --bracket-digest --credit prints credit=could-not-check because the witness file is not in a git work tree, so that check runs again on the landed commit.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrefixedBriefSchema$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestPrefixedBriefSchema ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewbriefCaps$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNewbriefCaps ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6039 run=916e2dae255be0c353f0d9117114c7d1e0d64b7ecbdaf4976e78aa07097545cf source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/milestone-support/brief-04-add-negotiated-brief-schema-and-prefixed-authoring.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=916e2dae255be0c353f0d9117114c7d1e0d64b7ecbdaf4976e78aa07097545cf source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:0a9ff1c5744d1cc530cc4f0d1aa68677836c5a239e4aaa8d1bba90a83a10bfee opened=2026-11-10T11:09:54Z closed=2026-11-10T11:11:53Z before=receipt after=check-verifier

Supplemental probes. These carry no acceptance credit. They ran in the same image with --network none, against a tracked-file snapshot of 612750a12406 held outside the home, and the snapshot was clean after each probe.

| Probe | Command | Expected | Observed | Date | Runner |
|-------|---------|----------|----------|------|--------|
| S1 | the brief's declared mutation manifest runner (statusgen testdata milestone-support 04-mutate.py), whole run, then one --name run per remaining case | every manifest plant turns its named test red | exit 1 on the whole run: 3 cases CAUGHT, then the runner stopped at case v4-milestone-gate-held with "expected exactly one anchor" (0 anchors in the parser at this source). Run one case at a time, the 11 remaining cases were all CAUGHT (7 against TestPrefixedBriefSchema, 4 against TestNewbriefCaps), with named baselines passing first. Total 14 of 15 caught, 1 stale anchor | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |
| S2 | the stale case's plant re-pointed at its new site in the milestone graph adapter, using the same runner: drop the appended behavioural-gate edge on (a) the held could-not-check path and (b) the resolved-gate path | TestPrefixedBriefSchema turns red | (b) CAUGHT by TestPrefixedBriefSchema. (a) survives TestPrefixedBriefSchema, because this brief's fixture takes the resolved path. The same (a) plant is CAUGHT by milestone-support/05's TestMilestoneGraphUnknowns and survives its TestMilestoneMixedGraph | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (on-behalf-of human:ian) |

Finding (no acceptance effect): the mutation manifest that this brief declares carries a stale case. Case v4-milestone-gate-held still anchors on the parser line that #5379 moved into the milestone graph adapter, so the runner stops there and reports no kill. The behaviour it guards is still covered, as S2 shows. The manifest needs that case re-anchored, and its whole-run exit is not a usable mutation result until it is.

Risk-bearing values. The brief carries risk metadata, all no, and is not irreversible. Enumeration covered the non-test Go and schema changes in the follow-up #5347 (statusgen/briefv4.go, schemas/brief-v4.json and its byte-identical statusgen copy), plus the entries ranked in the earlier pass, re-read at 612750a12406. Literals the follow-up adds: the envelope key "reader-requirements" (statusgen/briefv4.go:68), the envelope schema const "reader-requirements-v1" (schemas/brief-v4.json:196, statusgen/readercaps/caps.go:14), the required operation "syntax" (statusgen/briefv4.go:132) and the legacy provider and version spellings "legacy-brief-reader" and "legacy" (statusgen/briefv4.go:138). These are internal spellings that an edit and a release can undo, so they rank last. Ranked top entries:

- RISK-VALUE: DERIVED — briefSchemaV4 = "brief-v4" @ statusgen/brieffile.go:303 — spec/brief-v1.md:169 allocates exactly this schema name. The shipped schema pins it, and the two schema copies are byte-identical at this source.
- RISK-VALUE: DERIVED — briefSchemaFirstRelease["brief-v4"] = "" @ statusgen/briefschemaversions.go:31 — spec/brief-v1.md:169 lists brief-v4 as unreleased. Task 4 forbids inventing a supporting-release number, so empty is the only correct value until a release cut fills it.
- RISK-VALUE: DERIVED — requires-milestones item pattern = "^([a-z0-9][a-z0-9-]*:){1,3}m-[0-9]{2,}@[1-9][0-9]*$" @ schemas/brief-v4.json:187 — the {1,3} prefix count matches the three colon-only forms in spec/milestone-v1.md:49-51. @[1-9][0-9]* matches the positive contract version required by spec/milestone-v1.md:45.
- RISK-VALUE: DERIVED — envelope schema = "reader-requirements-v1" @ schemas/brief-v4.json:196 — it equals readercaps.RequirementsSchema at statusgen/readercaps/caps.go:14, which is the decoder the validator calls, so the descriptive schema and the validator agree.
- RISK-VALUE: NAMED, NOT DERIVED — brief id pattern = "^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:b-[0-9]{2,4}[a-z]?$" @ schemas/brief-v4.json:33 — the spec writes only <NN> and gives no source for the 2-4 digit bound or the optional letter suffix. The schema is descriptive and the validator owns the semantic rule.

The stream index already lists this brief as verified. This run flips nothing. CI owns the verified-to-done flip.

**VERIFY: PASS** — rows 1-2 of 2 pass in one closed container bracket (verdict=match) at 612750a12406. Supplemental S1 caught 14 of 15 manifest mutations, with 1 case left on a stale anchor (see the finding); no acceptance credit.
### Correction (2026-11-10): F-what-moved

The re-verification section above says: "Since the follow-up, the only change to this brief's delivered files is #5379 (milestone-support/05, merge 178eab638)." That is wrong. #5379 is the only change to the delivered code. Other delivered files also moved between the follow-up merge 29387eb13de7 and the source 612750a12406. The same section's list of what #5347 changed is also incomplete. The text above stays as it landed; this note replaces both statements. No earlier outcome record exists for this brief, so the range starts at the follow-up merge.

Delivered files. #5340 (merge 0dbff54ce38e, first parent fbd80a540b3b) changed 31 files. #5347 (merge 29387eb13de7, first parent bef30d4e0d54) changed 10. Together that is 33 distinct files. #5347's full list is: the brief-v4 parser statusgen/briefv4.go, both schema copies (schemas/brief-v4.json and statusgen/schemas/brief-v4.json), spec/brief-v1.md, the named test file statusgen/prefixedbrief_test.go, the mutation manifest statusgen/testdata/milestone-support/04-mutations.json, the fixture statusgen/testdata/milestone-support/04/tree/docs/streams/demo4/brief-03-pinned.md, tools/desk/internal/deskkit/capabilities_test.go, this brief's Notes section, and the changelog fragment changelog/feat-milestone-support-04-followup.md.

What moved over 29387eb13de7..612750a12406, delivered code. Of the 33 files, only statusgen/brieffile.go and statusgen/briefv4.go changed. Both changed in one commit, bacbb9981 in #5379, and the net diff of the two files over the range is identical to that commit's diff. The other delivered Go sources, the tests, both schema copies, the fixtures and the mutation runner and manifest did not change. These are the files the two rows build and read: the statusgen package, its embedded statusgen/schemas/brief-v4.json and the statusgen/testdata/milestone-support/04 tree. Both rows ran at 612750a12406, after this change, so the PASS above stands.

What moved over the same range, other delivered files. No row reads any of these. The two named tests take their fixtures from statusgen/testdata/milestone-support/04, and neither they nor the package code they call opens a file under spec/ or docs/. Outside test files, statusgen code names those spec files only in comments and message strings.

- spec/milestone-v1.md: 5 commits. Two are from #5379: bacbb9981 (+45) and 8bab0722f (+13/-1). Three are from #5911 (milestone-support/06, merge 8aedd1b42): 2e40bf6e1 (+78), fb35b7b75 (+2/-1) and a492d0a6f (+5/-1). The net change is +140 lines. The RISK-VALUE entries above cite spec/milestone-v1.md:45 and :49-51 as read at 612750a12406, so they already reflect this movement.
- spec/brief-v1.md: 11 commits, all release header stamps committed directly on main, from 9c188622f to 648e78628 (9c188622f, 438310c23, c747dc0cc, 468205923, 236fe20e0, eca1612a2, 7ff1c0bc7, 51796c3a1, d6b3dffe7, b1f0161ef, 648e78628). The net change is one line: the "Describes reference implementation" header moves from `statusgen` v1.0.71 to v1.0.82. The cited spec/brief-v1.md:169 was read at 612750a12406.
- docs/contracts.md: 41 commits from 20 merges. One commit is from #5379 (bacbb9981) and one from #5911 (2e40bf6e1). The rest are other streams' contract entries.
- docs/streams/milestone-support/README.md: 4 commits. 85d049afd (#5349) flips row 04 to verified, 9ace042b1 (#5354) flips row 03, cf8761853 (#5392) is a board reconcile, and 94cf127a0 (#5491) flips row 05.
- This brief: 2 commits, ec0f0b9ba and 89cbc52a6, both from #5349. They are the 2026-11-02 Evidence above.
- changelog/milestone-support-04.md and changelog/feat-milestone-support-04-followup.md: both removed by 9c188622f, the v1.0.71 changelog aggregation.

Commands, run in a full clone at 612750a12406:

- `git diff --name-only 0dbff54ce38e^1 0dbff54ce38e` (31 files) and `git diff --name-only 29387eb13de7^1 29387eb13de7` (10 files). Their sorted union is 33 files.
- For each of the 33 files, `git log --no-merges --format=%h 29387eb13de7..612750a12406 -- <file>`. Nine files report commits: 1 each for the two Go files and the two changelog fragments, 2 for this brief, 4 for the README, 5 for spec/milestone-v1.md, 11 for spec/brief-v1.md and 41 for docs/contracts.md. The other 24 report none.
- `git diff --stat 29387eb13de7 612750a12406` with the 33 files as separate pathspecs lists the same nine files.
- `git diff 29387eb13de7 612750a12406 -- statusgen/brieffile.go statusgen/briefv4.go` matches `git diff bacbb9981^ bacbb9981 -- statusgen/brieffile.go statusgen/briefv4.go` once the index lines are ignored.
- `git log --no-merges --numstat 29387eb13de7..612750a12406 -- spec/milestone-v1.md` gives the per-commit line counts. Each commit's PR was found by matching it to the first-parent merge on main that brought it in.

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
