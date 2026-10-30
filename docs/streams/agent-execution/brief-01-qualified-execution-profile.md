---
brief: "assay:assay:agent-execution:01"
title: "Qualified harness, model and backend capability contract"
why: "A resume flag cannot tell a caller whether files, conversation or running processes survive. A qualified profile makes those promises explicit before work is launched."
wave: 1
depends: ["graph-execution/20"]
unblocks: ["agent-execution/02", "agent-execution/06", "agent-execution/07"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-10-16 by execution architecture authoring session"
sources: ["docs/proposals/non-ci-architecture/27-agent-execution-environments.md", "freshness-checked 2026-10-16 @ ec8d77e56627a7c2269f3d143ef6a31f3e41bd02 (origin/main)", "spec/loop-admin-runner-v1.md", "docs/celld-session-host-spec.md"]
exec-tier: "strong"
exec-tier-why: "Cross-component protocol, recovery and authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["loopadmin/runner: fixed-here", "statusgen/portfolio: fixed-here", "loopadmin/workspace: follow-up agent-execution/02", "loopadmin/supervisor: follow-up agent-execution/03", "tools/desk/internal/cellop: follow-up agent-execution/05", "loopadmin/adapters/axcompat: follow-up agent-execution/06"]
version: 2
id: "b402ec83-8a07-4551-b382-5dc017a45315"
---

# Brief 01 — Qualified harness, model and backend capability contract

## Context

files: `spec/loop-admin-execution-profile-v1.md` (planned), `schemas/loop-admin-execution-profile-v1.json` (planned), `loopadmin/runner/executionprofile.go` (planned), `loopadmin/runner/executionprofile_test.go` (planned), `loopadmin/runner/conformance/`, `loopadmin/testdata/executionprofile/` (planned), `loopadmin/testdata/mutations.json`, `loopadmin/README.md` — All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: required-extension validation at the client boundary — backed by the adapter capability check and caller-owned current-profile validation, each exercised with the upper check bypassed.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: runner/request.go implements strict v1 decoding and ext/require negotiation; runner/usage.go uses boolean Resume and Snapshot fields. The baseline conformance suite passes. The new typed profile does not exist. The loopadmin module has no workflow dependency.

## Read first

- [Controlling future-state amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md).
- [Stream ownership and dependency waves](README.md).
- [Existing runner protocol](../../../spec/loop-admin-runner-v1.md).
- [Runtime and recovery owners](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md).
- [Cell operator lifecycle](../../proposals/non-ci-architecture/21-cell-operator-surface.md).

## Ground rules

- Own isolated branch and draft PR; no merge, deployment, workflow dispatch or live infrastructure contact.
- Offline fixtures and synthetic credentials only; no model/provider calls, new credential custody or profile activation.
- Preserve existing work identity, coordination/effect owners, historical Evidence and independent acceptance. Stop at implemented; a non-implementer owns verification.
- Refresh source, claims and active PRs at pickup. Missing prerequisites or contradictory contracts require a recorded hold, not a replacement implementation.

## Task

1. Define a schema-backed execution-profile/v1 extension over existing runner v1 ext/require negotiation. Bind the harness implementation/version, immutable harness-component bundle digest/provenance, model profile, backend/platform profile, profile digest and qualification evidence. Record requested versus actual reported identities; a requested model is never an observation.
2. Declare files, conversation, process memory, descendants and pending-interaction preservation separately, alongside cancellation confirmation, budget scope, telemetry and qualification limits. Retain existing v1 resume/snapshot meanings and doc 21 lifecycle vocabulary. Unknown and unsupported remain distinct; do not infer one capability from another.
3. Implement validation and compatibility in the existing runner package/client and adapter conformance kit. Old v1 requests remain accepted; mandatory new semantics on an old adapter refuse before Start. No new launcher, credentials, SDK dependency or workflow-store import. Include schema-to-source coverage and a malformed-artifact negative case.
4. Add the named production-boundary tests and executable mutations to the existing mutation map. Prove both desk and workflow request modes, role/profile substitution and a caller check with the upper contract check bypassed. Update protocol docs and changelog; no installed profile is changed.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row reads the immutable delivered snapshot and its first-parent base explicitly; it never uses an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | exit 0; named PASS; old v1 requests still work, while an old adapter refuses a required extension before launch; mutation: ignore the new mandatory extension — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | exit 0; named PASS; file-only restoration cannot satisfy conversation or process preservation in either caller mode; mutation: treat every resume declaration as full process preservation — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | exit 0; named PASS; a substituted backend/profile and drifted schema are rejected; bypassing the client check still leaves adapter/caller checks effective; mutation: accept a mismatched qualified profile digest — the named test must fail |
| 4 | check:ci | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief agent-execution/01 --delivery 65ca2b848584b1ccde22f28afa4137767317fe4b --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7 > "$consumer_out" && cat "$consumer_out" && grep -Fqx -- "summary: 6 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" "$consumer_out")` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Command amendment — 2026-10-27

Version 2 corrects original row 4's Command and updates the Verify preamble to describe its immutable delivered range, under issue #4560. Delivery is the merge of #4501 (`65ca2b848584b1ccde22f28afa4137767317fe4b`); its first parent is `b1a7e42333a4384da45d399bc723e33f4dc3a3f7`. The command pins both endpoints and asserts the delivered summary, including zero unchecked entries. Row 4's original Expect and class, rows 1–3, Task, consumer routes and all historical Evidence remain unchanged. This definition repair is not acceptance or independent verification. After the amendment lands, the explicit human-approved acceptance-transition route in [Protected verifier paths](../../protected-paths.md#explicit-human-approved-acceptance-transitions) must bind this landed amendment and row 4 before fresh independent execution can earn credit. Earlier supplemental probes remain uncredited.

## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

### Independent verification — 2026-10-26 (run 1, source 8a22d6b5a, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 8a22d6b5a16bc3f41e80c4c757376ff195659bb2, detached home, unchanged except this Evidence section. Admission: this run's own attestation (medici-finance/assay#4508). `deskdispatch --check-verifier` exited 0 before any row ran, and again at each bracket's open and close steps.

Delivery: the work landed in the merge of medici-finance/assay#4501 (65ca2b848584b1ccde22f28afa4137767317fe4b; first parent b1a7e42333a4384da45d399bc723e33f4dc3a3f7, branch head cd3d6b7525ec787a7b45c449c580377fc1e33041), an ancestor of the observed source. The Verify section is byte-identical at the delivery base, at the delivery and at the observed source (brief version 1 throughout), and no commit after the delivery touches loopadmin or this brief.

Runner class: local container, `--network none`. Every row is check:ci and the darwin host has no `unshare --net`, so all four rows ran inside closed admission brackets: `statusgen verifyrun --open-bracket` on the host, then `statusgen verifyrun --sandbox=container-netns --rows <ids>` inside the container, then `statusgen verifyrun --close-bracket` on the host, with no edit to the home between the steps. Two brackets, with disjoint row selections that together cover the table: rows 1–3, then row 4. The image is the published verify-runner v1.0.62 image, run by digest (ghcr.io/medici-finance/verify-runner@sha256:c704b01c271a8b76c04afbcff6396f3bd88ea91d33a9cad56fccb9fadc12b219; linux/amd64, go1.27.2, git 2.56.0, its own installed statusgen v1.0.62, the same release as the host's), under the engine's default seccomp profile with no added privileges. Mounts: the home bind-mounted read-only at its own path; the admission receipt read-only; a new directory for the witness file, the only writable mount. Bracket 2 (row 4, which reads git history) also mounted the home's repository git directory read-only at its own path and ran as the host user. No module cache was mounted: loopadmin depends only on the in-repo perfrec module, so the rows build with GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, KUBECONFIG=/dev/null. Each container step recomputed the mounted home's tree digest and matched the value the host recorded at the open step.

Rows 1–3 (bracket 1). The Output digest is of the row's stdout, which is empty because each command redirects the test log and greps it quietly; exit 0 means the named `--- PASS:` line was present.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ 8a22d6b5a16b (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ 8a22d6b5a16b (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ 8a22d6b5a16b (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bfa735da8c688e96c61feafd9fe6b183a981dc5f1445e00dc3460fbade1cefa2
Container-Image: sha256:c704b01c271a8b76c04afbcff6396f3bd88ea91d33a9cad56fccb9fadc12b219
Verification-Attestation: medici-finance/assay#4508 run=2a7bb40ea8691af9141de2e068436e27f50d7f5557bf8843fe24818d0925e33b source=8a22d6b5a16bc3f41e80c4c757376ff195659bb2 brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=2a7bb40ea8691af9141de2e068436e27f50d7f5557bf8843fe24818d0925e33b source=8a22d6b5a16bc3f41e80c4c757376ff195659bb2 tree=sha256:bfa735da8c688e96c61feafd9fe6b183a981dc5f1445e00dc3460fbade1cefa2 witness=sha256:1bbc636e16ff67f8d93082619c226ae8e961e28583806def3a51384fd372b973 opened=2026-10-26T15:53:29Z closed=2026-10-26T15:54:06Z before=receipt after=check-verifier

Row 4 (bracket 2). The witness records `fail exit=2`. Exit 2 is the consumers gate's COULD-NOT-CHECK exit ("the diff could not be taken"), not a disproved claim. Observed stdout of the same command in the same image and mounts, outside the bracket: `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:agent-execution:01 is not in the diff against 8a22d6b5a16bc3f41e80c4c757376ff195659bb2, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` On merged main, `$(git merge-base origin/main HEAD)` resolves to HEAD itself, so the command diffs main against main. The brief's own Verify preamble says post-merge verification uses the retained delivery branch and base, not an empty main-to-main diff, but the command as written cannot reach that branch from an admitted home detached at merged main. Setting ASSAY_DELIVERY_SHA and ASSAY_DELIVERY_BASE_SHA in the container does not change this: the command passes neither `--delivery` nor those variables. Row 4 therefore has no acceptance result in this run. This is a row-definition gap, routed to the desk for a reviewed definition repair; the command was not rewritten here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `statusgen --consumers --brief agent-execution/01 --base $(git merge-base origin/main HEAD)` | fail exit=2 sandbox=container-netns | sha256:bc6eb88ec0a7 | 2026-10-26 | assay-verifier-app[bot] @ 8a22d6b5a16b (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bfa735da8c688e96c61feafd9fe6b183a981dc5f1445e00dc3460fbade1cefa2
Container-Image: sha256:c704b01c271a8b76c04afbcff6396f3bd88ea91d33a9cad56fccb9fadc12b219
Verification-Attestation: medici-finance/assay#4508 run=2a7bb40ea8691af9141de2e068436e27f50d7f5557bf8843fe24818d0925e33b source=8a22d6b5a16bc3f41e80c4c757376ff195659bb2 brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=2a7bb40ea8691af9141de2e068436e27f50d7f5557bf8843fe24818d0925e33b source=8a22d6b5a16bc3f41e80c4c757376ff195659bb2 tree=sha256:bfa735da8c688e96c61feafd9fe6b183a981dc5f1445e00dc3460fbade1cefa2 witness=sha256:c4f06fc295ea62e2f0a59c467dba45e976aca9244cb576aa8b4be6d17bb3dd74 opened=2026-10-26T15:55:04Z closed=2026-10-26T15:55:23Z before=receipt after=check-verifier

Supplemental observations — diagnostics only, no acceptance credit for any row. The mutation probes ran in the same image under `--network none`, on scratch copies of loopadmin, perfrec and schemas exported from the observed source, outside the home; each mutant applies one entry of the brief's mutation map (loopadmin/testdata/mutations.json) and runs the row's own command for its named test. Probes S2 and S3 ran the installed statusgen v1.0.62 on the host against a scratch clone, outside the home.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | the three named tests with `-v`, unmutated scratch copy | `--- PASS: TestExecutionProfileV1Compatibility`, `--- PASS: TestExecutionProfilePreservationFlow`, `--- PASS: TestExecutionProfileBindingAndSchema`; each row command exit 0 | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 | row 4 command form at the delivery branch head cd3d6b752 with its own merge-base: `statusgen --consumers --brief agent-execution/01 --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7` | exit 0; 2 corroborated (loopadmin/runner, statusgen/portfolio: fixed-here), 0 disproved, 4 unchecked (the four follow-up entries: unchanged since the merge-base) | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 | delivery form at the observed source: `statusgen --consumers --brief agent-execution/01 --delivery 65ca2b848584b1ccde22f28afa4137767317fe4b --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7` | exit 0; immutable delivered snapshot; 6 corroborated, 0 disproved, 0 unchecked | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M1 | row 1: an adapter that does not declare a required extension accepts it anyway (runner/request.go) | killed: row 1 command exit 1, `--- FAIL: TestExecutionProfileV1Compatibility` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M2 | row 1: the client ignores require and checks the profile only when a value is carried (runner/executionprofile.go) | killed: row 1 command exit 1, `--- FAIL: TestExecutionProfileV1Compatibility` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M3 | row 2: admission treats every resume declaration as full process preservation (runner/executionprofile.go) | killed: row 2 command exit 1, `--- FAIL: TestExecutionProfilePreservationFlow` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M4 | row 2: acceptance treats every reported restoration as full process preservation (runner/executionprofile.go) | killed: row 2 command exit 1, `--- FAIL: TestExecutionProfilePreservationFlow` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M5 | row 3: accept a declared profile whose digest is not the digest of its content (runner/executionprofile.go) | killed: row 3 command exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M6 | row 3: accept a report naming another qualified profile digest (runner/executionprofile.go) | killed: row 3 command exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M7 | row 3: the adapter trusts the client's admission and drops its own check (runner/fake/fake.go) | killed: row 3 command exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M8 | row 3: the caller commits without checking its own current profile (runner/conformance/profiles.go) | killed: row 3 command exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M9 | row 3: a Go type drifts from the schema, a required dimension becomes optional (runner/executionprofile.go) | killed: row 3 command exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Mutation tooling note: the module's own gate (`cd loopadmin && go run ./mutate -map testdata/mutations.json -test <name>`) reported every entry above as "mutation does not build" in this image. It copies only the loopadmin directory to a temporary directory, so the go.mod replacement `../perfrec` does not resolve, and TestExecutionProfileBindingAndSchema also reads its schema from two directories up. The mutations above were therefore applied by hand, one per scratch copy of the full module layout. Reported to the desk for routing.

Risk-bearing values. The brief's risk answers are all "no" and it changes no installed profile. Enumeration over the delivery diff's non-test Go source and the new schema found these literals. All are protocol constants that an edit and a redeploy reverse. The extension name ranks first because adopters may persist it. Test-fixture literals such as the synthetic harness version are excluded.

- RISK-VALUE: DERIVED — ExecutionProfileExt = "execution-profile/v1" @ loopadmin/runner/executionprofile.go:24, with RequireExecutionProfile = "ext:" + ExecutionProfileExt @ loopadmin/runner/executionprofile.go:28 — the extension name and require entry fixed by the execution-profile spec (spec/loop-admin-execution-profile-v1.md lines 17 and 19); "ext:<name>" is runner v1's require form for an extension (spec/loop-admin-runner-v1.md line 94).
- RISK-VALUE: DERIVED — MaxDocumentBytes = 1 << 20 @ loopadmin/runner/request.go:166 — 1,048,576 bytes, the 1 MiB limit runner v1 section 3 sets for a document and any extension value decoded on its own (spec/loop-admin-runner-v1.md line 30), applied to declarations, requirements and reports by spec/loop-admin-execution-profile-v1.md line 43.
- RISK-VALUE: DERIVED — profileDigest = `^sha256:[0-9a-f]{64}$` @ loopadmin/runner/executionprofile.go:174 — a SHA-256 digest is 256 bits, which is 64 hex digits; the schema's digest pattern (schemas/loop-admin-execution-profile-v1.json line 17) is the same.
- RISK-VALUE: DERIVED — SupportYes/SupportNo/SupportUnknown = "supported"/"unsupported"/"unknown" @ loopadmin/runner/executionprofile.go:37-39 — the closed tri-state the brief's Task 2 requires (unknown and unsupported distinct), matching the schema enum (schemas/loop-admin-execution-profile-v1.json line 20).

Verdict accounting: 4 required rows. Rows 1–3 pass, witnessed in bracket 1. Row 4 has no acceptance result: the consumers gate returned COULD-NOT-CHECK, exit 2, because the command as written diffs merged main against itself. No obligation was observed to fail. The supplemental probes S2 and S3 match row 4's Expect, but they carry no acceptance credit.

**VERIFY: BLOCKED** — 3 of 4 rows pass; row 4's consumers check could not run against the delivery from the admitted merged-main home (COULD-NOT-CHECK, exit 2) and needs a reviewed row-definition repair or a desk ruling on running it at the retained delivery branch.
### Independent verification — 2026-10-27 (run 2, source 47ef38f96, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 47ef38f961aa1006ff296034d118630efdc177cb, detached home, unchanged except this Evidence section. Admission: this run's own attestation (#4577). `deskdispatch --check-verifier` exited 0 before any row ran, and again at the bracket's open and close steps.

Delivery: the work landed in the merge of #4501 (65ca2b848584b1ccde22f28afa4137767317fe4b; first parent b1a7e42333a4384da45d399bc723e33f4dc3a3f7, branch head cd3d6b7525ec787a7b45c449c580377fc1e33041), an ancestor of the observed source. The Verify section is byte-identical at the delivery base, at the delivery and at the observed source (brief version 1 throughout). Between run 1's source (8a22d6b5a) and this source, nothing under loopadmin, perfrec, the execution-profile schema or spec changed; the brief gained only run 1's Evidence.

Runner class: local container, `--network none`. Every row is check:ci and the darwin host has no `unshare --net`, so all four rows ran inside one closed admission bracket: `statusgen verifyrun --open-bracket` on the host, then `statusgen verifyrun --sandbox=container-netns` inside the container, then `statusgen verifyrun --close-bracket` on the host, with no edit to the home between the steps. `statusgen verifyrun --bracket-digest` on the closed witness reports verdict=match. The image is the published verify-runner v1.0.63 image, run by digest (ghcr.io/medici-finance/verify-runner@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8; linux/amd64, go1.27.2, git 2.56.0, its own installed statusgen v1.0.63, the same release as the host's), under the engine's default seccomp profile with no added privileges, as the host user. Mounts: the home read-only at its own path; the home's repository git directory read-only at its own path (row 4 reads git history); the admission receipt read-only; a new directory for the witness file, the only writable mount. No module cache was mounted: loopadmin depends only on the in-repo perfrec module, so the rows build with GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, KUBECONFIG=/dev/null. The container step recomputed the mounted home's tree digest and matched the value the host recorded at the open step.

Rows 1–3: the Output digest is of the row's stdout, which is empty because each command redirects the test log and greps it quietly; exit 0 means the named `--- PASS:` line was present. Row 4: the witness records `fail exit=2`. Exit 2 is the consumers gate's COULD-NOT-CHECK exit ("the diff could not be taken"), not a disproved claim. Observed output of the same command in the same image and mounts, outside the bracket: `merge-base=47ef38f961aa1006ff296034d118630efdc177cb head=47ef38f961aa1006ff296034d118630efdc177cb` and `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:agent-execution:01 is not in the diff against 47ef38f961aa1006ff296034d118630efdc177cb, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` On merged main the command's `$(git merge-base origin/main HEAD)` resolves to HEAD itself, so it diffs main against main. The row-definition gap run 1 recorded is unchanged; the command was not rewritten here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --consumers --brief agent-execution/01 --base $(git merge-base origin/main HEAD)` | fail exit=2 sandbox=container-netns | sha256:1ae085d926d9 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4577 run=caa921043e380e3b6a9e3afa72b64aceb11b71a08a3942219c0a15b9e5957086 source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=caa921043e380e3b6a9e3afa72b64aceb11b71a08a3942219c0a15b9e5957086 source=47ef38f961aa1006ff296034d118630efdc177cb tree=sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd witness=sha256:1224cb7e59ef656b0e080691a57c4389d2606f6ca699ae8cabac9d2bd674095a opened=2026-10-27T01:27:33Z closed=2026-10-27T01:29:45Z before=receipt after=check-verifier

Supplemental observations — diagnostics only, no acceptance credit for any row. All ran in the same image under `--network none`, outside the home. M1–M9 ran on scratch copies of loopadmin, perfrec and schemas exported from the observed source with git archive; each mutant applies one entry of the brief's mutation map (loopadmin/testdata/mutations.json, the nine entries naming the three row tests, each matched exactly once) and runs the row's own command form for its named test. S2 and S3 ran against the read-only home and git directory.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | the three named tests, unmutated scratch copy | each row command form exit 0 with its named `--- PASS:` line | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 | consumers with no --base: `statusgen --consumers --brief agent-execution/01` | exit 2, COULD-NOT-CHECK: the committed Evidence has no Verification-Table marker naming the delivery | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 | delivery form: `statusgen --consumers --brief agent-execution/01 --delivery 65ca2b848584b1ccde22f28afa4137767317fe4b --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7` | exit 0; immutable delivered snapshot; 6 corroborated, 0 disproved, 0 unchecked | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M1 | row 1: an adapter that does not declare a required extension accepts it anyway (runner/request.go) | killed: exit 1, `--- FAIL: TestExecutionProfileV1Compatibility` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M2 | row 1: the client ignores require and checks the profile only when a value is carried (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfileV1Compatibility` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M3 | row 2: admission treats every resume declaration as full process preservation (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfilePreservationFlow` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M4 | row 2: acceptance treats every reported restoration as full process preservation (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfilePreservationFlow` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M5 | row 3: accept a declared profile whose digest is not the digest of its content (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M6 | row 3: accept a report naming another qualified profile digest (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M7 | row 3: the adapter trusts the client's admission and drops its own check (runner/fake/fake.go) | killed: exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M8 | row 3: the caller commits without checking its own current profile (runner/conformance/profiles.go) | killed: exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M9 | row 3: a Go type drifts from the schema, a required dimension becomes optional (runner/executionprofile.go) | killed: exit 1, `--- FAIL: TestExecutionProfileBindingAndSchema` | 2026-10-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Risk-bearing values. The brief's risk answers are all "no", no Verify row carries a risk-bearing, live, mutating or end-to-end tag, and the brief changes no installed profile, so no fail-first witness is owed. Enumeration over the delivery diff's non-test Go source and the new schema found these literals, re-read at the observed source. All are protocol constants that an edit and a redeploy reverse. The extension name ranks first because adopters may persist it. Test-fixture literals are excluded.

- RISK-VALUE: DERIVED — ExecutionProfileExt = "execution-profile/v1" @ loopadmin/runner/executionprofile.go:24, with RequireExecutionProfile = "ext:" + ExecutionProfileExt @ loopadmin/runner/executionprofile.go:28 — the extension name and require entry fixed by the execution-profile spec (spec/loop-admin-execution-profile-v1.md lines 17 and 19); "ext:<name>" is runner v1's require form for an extension (spec/loop-admin-runner-v1.md line 94).
- RISK-VALUE: DERIVED — MaxDocumentBytes = 1 << 20 @ loopadmin/runner/request.go:166 — 1,048,576 bytes, the 1 MiB limit runner v1 section 3 sets for a document and any extension value decoded on its own (spec/loop-admin-runner-v1.md line 30), applied to declarations, requirements and reports by spec/loop-admin-execution-profile-v1.md line 43.
- RISK-VALUE: DERIVED — profileDigest = `^sha256:[0-9a-f]{64}$` @ loopadmin/runner/executionprofile.go:174 — a SHA-256 digest is 256 bits, which is 64 hex digits; the schema's digest pattern (schemas/loop-admin-execution-profile-v1.json line 17) is the same.
- RISK-VALUE: DERIVED — SupportYes/SupportNo/SupportUnknown = "supported"/"unsupported"/"unknown" @ loopadmin/runner/executionprofile.go:37-39 — the closed tri-state the brief's Task 2 requires (unknown and unsupported distinct), matching the schema enum (schemas/loop-admin-execution-profile-v1.json line 20).

Verdict accounting: 4 required rows. Rows 1–3 pass, witnessed in the bracket. Row 4 has no acceptance result: the consumers gate returned COULD-NOT-CHECK, exit 2, because the command as written diffs merged main against itself. No obligation was observed to fail. Probe S3 matches row 4's Expect but carries no acceptance credit. The row-definition repair is tracked with the same class in #4560.

**VERIFY: BLOCKED** — 3 of 4 rows pass; row 4's consumers check cannot reach the delivery from the admitted merged-main home (COULD-NOT-CHECK, exit 2) and still needs a reviewed row-definition repair pinning --delivery and its first-parent --base.

### Independent reverification — 2026-10-27

Fresh independent run after the exact human approval recorded on issue #4703. Source 384d5eda2ca230e7e748750fdaed6ba4d1d50ed8; gpt-6.1-sol, strong. Native pre-work admission is record #4704. Installed host statusgen and deskdispatch are v1.0.63. The container used the digest recorded below, with network none and the engine default seccomp profile. All source files remained read-only during execution. Historical Evidence is preserved.

Verification-Table: delivery=65ca2b848584b1ccde22f28afa4137767317fe4b base=b1a7e42333a4384da45d399bc723e33f4dc3a3f7 original=6ae7933e2f92aecce05ac80858690a1e31ba304b
Acceptance-Transition: amendment=04c831abcd9dffa7f4ef86fce2d1c91231eda221 approval=https://github.com/medici-finance/assay/issues/4703#issuecomment-6455692591

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief agent-execution/01 --delivery 65ca2b848584b1ccde22f28afa4137767317fe4b --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7 > "$consumer_out" && cat "$consumer_out" && grep -Fqx -- "summary: 6 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" "$consumer_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4704 run=d61352f30225d34b46bdbe6c942b9a3922485483fbb4ccaa8bd78e77e808521e source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=gpt-6.1-sol tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:6963ad5593dc6a427eb55e64b98194a327862d2882567ed257475fd4810eb876
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4704 run=d61352f30225d34b46bdbe6c942b9a3922485483fbb4ccaa8bd78e77e808521e source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=gpt-6.1-sol tier=strong
Admission-Bracket: run=d61352f30225d34b46bdbe6c942b9a3922485483fbb4ccaa8bd78e77e808521e source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 tree=sha256:6963ad5593dc6a427eb55e64b98194a327862d2882567ed257475fd4810eb876 witness=sha256:1fe92b207b76786253ab9306fe859f748dee91ae139b57aa6f242227d1aef8e1 opened=2026-10-27T23:02:08Z closed=2026-10-27T23:03:55Z before=receipt after=check-verifier

**VERIFY: BLOCKED** — 3 of 4 original obligations have fresh execution witnesses; row 4 could not run on the Darwin host because its check:ci class requires Linux unshare. The native acceptance-transition resolver independently corroborated the exact human approval, and original rows 1–3 passed inside the closed container admission bracket. Row 4 requires a supported independently admitted Linux host with git history and unshare; mounting host Git custody into the container or weakening the row sandbox is outside this run. Row 4's executor gap is tracked on #4327 (the container lane does not mount a linked worktree's git directory) and #1800 (the darwin host has no network-off sandbox). No acceptance obligation was observed failing, and no lifecycle advancement is claimed.

No blocked outcome record landed for this run. The outcome writer's mandatory dry-run (`deskevidence --outcome-record --dry-run`) refused with exit 5, "unsupported Context files: declaration tail": this brief's Context `files:` line ends in a prose tail after its last path, which the declared-scope parser does not accept. That refusal class is tracked on #3697. Context is outside the verifier lane and was not edited; the record is held until the declaration parses.

Risk-value procedure: gate model, all risk answers no; the delivery diff contains no configured risk-classed path or changed house-pinned hard constraint. No risk-bearing-value trigger was found.
### Independent verification — 2026-10-29 (run 4, source 2f1ede291, container bracket plus host transition)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 2f1ede291fb4296a9bf657df59a7a20857627dcb, detached home, unchanged except this Evidence section. Admission: this run's own attestation (#4863). `deskdispatch --check-verifier` exited 0 before any row ran, and again at the bracket's open and close steps. Host statusgen and deskdispatch are v1.0.65.

Delivery and table provenance, re-derived in this run rather than copied: the delivery is the two-parent merge 65ca2b848584b1ccde22f28afa4137767317fe4b on the observed source's first-parent chain; its parents are b1a7e42333a4384da45d399bc723e33f4dc3a3f7 and cd3d6b7525ec787a7b45c449c580377fc1e33041, whose unique merge-base is b1a7e4233 itself. The original revision 6ae7933e2f92aecce05ac80858690a1e31ba304b is the commit that added this brief, an ancestor of that base, and its Verify section is byte-identical to the section at the base and at the delivery. The amendment 04c831abcd9dffa7f4ef86fce2d1c91231eda221 (merge of #4620) is on the first-parent chain after the delivery. The approval comment on #4703 is unedited, by a User-type account, and its payload names exactly these coordinates with rows=4. `deskpathguard rederive` reports 4 rows, 0 author-added. Between the previous run's source (384d5eda2) and this source nothing under loopadmin, perfrec, the execution-profile schema or either runner spec changed; the brief's Context files: block was rewritten into the declaration grammar (#3697) and Evidence was appended.

Lanes. Every row is check:ci and the darwin host has no `unshare --net`. Row 4 is the approved command correction, so it owes the protected acceptance route: `statusgen verifyrun --acceptance-record <descriptor> --acceptance-forge --rows 4`, host execution only (the protected-paths contract runs a transition on the host, it needs a fresh approval read the network-off container cannot make, and the container executes only the current table). The descriptor held the two coordinate lines below and sat outside the home. On this host that row is could-not-run (witness below). Rows 1–3 are unchanged original rows and ran inside one closed admission bracket with `--rows 1,2,3`: `statusgen verifyrun --open-bracket` on the host, `statusgen verifyrun --sandbox=container-netns` inside the container, then `statusgen verifyrun --close-bracket` on the host, with no edit to the home between the steps; the host transition invocation ran after the bracket closed. The two selections are disjoint and cover the table. The image is the published verify-runner v1.0.65 image, run by digest (ghcr.io/medici-finance/verify-runner@sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f; linux/amd64, go1.27.3, git 2.56.0, its own installed statusgen v1.0.65, the same release as the host's), `--network none`, the engine's default seccomp profile, no added privileges, as the host user. Mounts: the home read-only at its own path; the repository git directory read-only at its own path (the tool's tree digest reads the index); the admission receipt read-only; a new directory for the witness file, the only writable mount. GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, KUBECONFIG=/dev/null, HOME and the build cache container-local. Two earlier container attempts in this run refused at admission before any row (the mounted root digested to the empty-tree value) because the shell mangled the bind-mount specification; no row executed and no witness was written by either. `statusgen verifyrun --bracket-digest` on the closed witness reports verdict=match with span-lines tree-digest:1, container-image:1, attestation:1.

Row 4, host transition invocation:

Acceptance-Changes: 4=Command
Verification-Table: delivery=65ca2b848584b1ccde22f28afa4137767317fe4b base=b1a7e42333a4384da45d399bc723e33f4dc3a3f7 original=6ae7933e2f92aecce05ac80858690a1e31ba304b
Acceptance-Transition: amendment=04c831abcd9dffa7f4ef86fce2d1c91231eda221 approval=https://github.com/medici-finance/assay/issues/4703#issuecomment-6455692591

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief agent-execution/01 --delivery 65ca2b848584b1ccde22f28afa4137767317fe4b --base b1a7e42333a4384da45d399bc723e33f4dc3a3f7 > "$consumer_out" && cat "$consumer_out" && grep -Fqx -- "summary: 6 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" "$consumer_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4296a9bf657df59a7a20857627dcb (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4863 run=bfdc87ce44c31a05b4fc9b22dffeecaa18bcc3e91851d61b086920c6a3427cb2 source=2f1ede291fb4296a9bf657df59a7a20857627dcb brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=claude-opus-5-5 tier=strong

Rows 1–3, container bracket. The Output digest is of the row's stdout, which is empty because each command redirects the test log and greps it quietly; exit 0 means the named `--- PASS:` line was present.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:2d978c9cba4dbbec14f5b15af4ea5c0d911aab5be9f4c5a5173bddb8a8b1b3e9
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4863 run=bfdc87ce44c31a05b4fc9b22dffeecaa18bcc3e91851d61b086920c6a3427cb2 source=2f1ede291fb4296a9bf657df59a7a20857627dcb brief=docs/streams/agent-execution/brief-01-qualified-execution-profile.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=bfdc87ce44c31a05b4fc9b22dffeecaa18bcc3e91851d61b086920c6a3427cb2 source=2f1ede291fb4296a9bf657df59a7a20857627dcb tree=sha256:2d978c9cba4dbbec14f5b15af4ea5c0d911aab5be9f4c5a5173bddb8a8b1b3e9 witness=sha256:5cd78405f40898b0cb2daf3a237a978785ce3a2506f841305ba1068799ca51dc opened=2026-10-29T19:50:14Z closed=2026-10-29T19:53:13Z before=receipt after=check-verifier

Supplemental observation — diagnostic only, no acceptance credit for any row. S1 ran row 4's exact command on the darwin host from the home, outside any network-off sandbox and with no forge token in its environment; it is not a check:ci witness.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | row 4's exact command, host shell, no sandbox | exit 0; 6 CORROBORATED (loopadmin/runner and statusgen/portfolio fixed-here; agent-execution/02, 03, 05 and 06 follow-ups); `summary: 6 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing` matched exactly; home unchanged afterwards | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Mutation obligations of rows 1–3: this pass made no new mutation observation. The mutation map entries and every file they patch are byte-identical to run 2's source (47ef38f96), where all nine (M1–M9) were killed by their named row commands.

Risk-bearing values. The brief's risk answers are all "no", it is not irreversible, and it changes no installed profile. Enumeration over the delivery diff's non-test Go source and the new schema, re-read at this source, finds the same protocol literals as runs 1 and 2; each is reversed by an edit and a redeploy. The extension name ranks first because adopters may persist it.

- RISK-VALUE: DERIVED — ExecutionProfileExt = "execution-profile/v1" @ loopadmin/runner/executionprofile.go:24, with RequireExecutionProfile = "ext:" + ExecutionProfileExt @ loopadmin/runner/executionprofile.go:28 — the extension name and require entry fixed by the execution-profile spec (spec/loop-admin-execution-profile-v1.md lines 17 and 19); "ext:<name>" is runner v1's require form for an extension (spec/loop-admin-runner-v1.md line 94).
- RISK-VALUE: DERIVED — MaxDocumentBytes = 1 << 20 @ loopadmin/runner/request.go:166 — 1,048,576 bytes, the 1 MiB limit runner v1 section 3 sets for a document and any extension value decoded on its own (spec/loop-admin-runner-v1.md line 30), applied to declarations, requirements and reports by spec/loop-admin-execution-profile-v1.md line 43.
- RISK-VALUE: DERIVED — profileDigest = `^sha256:[0-9a-f]{64}$` @ loopadmin/runner/executionprofile.go:174 — SHA-256 is 256 bits, which is 64 hex digits; the schema's digest pattern (schemas/loop-admin-execution-profile-v1.json line 17) is the same.
- RISK-VALUE: DERIVED — SupportYes/SupportNo/SupportUnknown = "supported"/"unsupported"/"unknown" @ loopadmin/runner/executionprofile.go:37-39 — the closed tri-state Task 2 requires (unknown and unsupported distinct), matching the schema enum (schemas/loop-admin-execution-profile-v1.json line 20).

Verdict accounting: 4 required original rows. Rows 1–3 pass, witnessed in the closed container bracket at this source. Row 4 has no acceptance result: its owed route is the host transition invocation, and on this darwin host the check:ci row is could-not-run (no `unshare --net`). No obligation was observed to fail; probe S1 matches row 4's Expect but carries no credit. Row 4 needs an independently admitted Linux host with `unshare --net`, git history and forge read access for the approval, the executor gap tracked on #1800 (darwin has no network-off sandbox) and #4327 (the container lane does not carry a linked worktree's git directory).

**VERIFY: BLOCKED** — 3 of 4 original rows pass at 2f1ede291; row 4 (approved command correction, host-only transition route) is could-not-run on the darwin host and needs a Linux runner with `unshare --net`.

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
