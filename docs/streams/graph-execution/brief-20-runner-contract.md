---
brief: assay:assay:graph-execution:20
title: Shared loop-admin runner protocol and offline conformance kit
why: Standing desks and workflow stages need the same checked model-launch contract so provider integrations, recovery
  and accounting are implemented once.
wave: 0
depends: []
unblocks:
- graph-execution/22
- graph-execution/23
- graph-execution/21
- graph-execution/26
- agent-execution/01
- agent-execution/10
effort: M
gate: model
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'loopadmin/supervisor: follow-up graph-execution/26'
- 'loopadmin/adapters: follow-up graph-execution/22'
- 'workflow/controller: follow-up graph-execution/21'
- 'standing desk bindings: follow-up graph-execution/27'
version: 2
id: 64e79118-7a61-43d3-b9a8-5ec8aed75ea7
---

# Brief 20 — Shared loop-admin runner protocol and offline conformance kit

## Context

files: `loopadmin/runner/` (planned), `loopadmin/testdata/runner/` (planned), `spec/loop-admin-runner-v1.md` (planned), `schemas/loop-admin-runner-v1.json` (planned), `loopadmin/README.md` (planned), `changelog/graph-execution-20-runner-contract.md` (planned).

facts: The loopadmin Go module is planned here and must build without the workflow module, the instance store (19) or a pattern instance. At main 307fe1699 a harness process runner (`tools/desk/internal/cellprocess`), a per-role cadence lease (`tools/desk/internal/cellcadence`) and cellctl's Claude/Codex/Cursor launch path already exist; this contract is the caller/adapter API over them, not a second launcher: 26 moves the process runner into loopadmin, 22 wraps cellctl's resolved argv, and 27 transfers enrolled roles from cellctl's launch path. `tools/desk/internal/loopengine` owns in-session desk queue behavior; graph consumers retain canonical instance/admission/recovery ownership. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the generation fence in the runner contract's result acceptance — behind it, each caller's own authority check (claim generation at the desk, admission at the controller), which rejects a fenced result that a faulty adapter still reports as success.

## Verification environment — local Linux containers (2026-10-18)

A local Docker Linux engine is now available on the macOS verification host. The
2026-10-15 Evidence block's "no running daemon" condition is historical, not a
current prerequisite for this brief. A remote Linux host or completion of the
verify-runner release-image automation is not required to attempt the existing
Linux witness locally.

Run the five existing Verify commands and the `statusgen verifyrun` witness in a
local Linux container with `--network none`. Use the supported
`--sandbox container-netns` mode for an already isolated container namespace;
Docker network isolation alone does not make a bare macOS witness valid. Check
that the container has only loopback before running the witness. Do not weaken
network isolation to download missing dependencies: prepare the approved image,
Linux toolchain, source at the exact candidate revision and dependency cache
before the network-off run. Keep credentials, host configuration and the Docker
socket outside the container; use a disposable source copy and capture evidence
outside the source under verification.

Record the immutable image identity, architecture, Go/statusgen versions, source
revision, network mode, row exits and witness output. A 2026-10-18 local preflight
confirmed a Linux Docker daemon and a cached Linux/amd64 verification image
reporting Go 1.27.2 and statusgen v1.0.46. This establishes local availability,
not image-release qualification or a new verification result for this brief.
The independent verifier must check the selected image/toolchain against the
current verification requirements and produce the actual witness; missing tools,
dependencies or namespace isolation remain could-not-check with their precise
cause. Preserve prior Evidence and the five Verify rows. This amendment neither
closes a fleet-wide environment issue nor advances implemented to verified/done.

This is a verification-environment update within the existing
[library-first ownership map](../../library-first.md) and
[future runtime architecture](../../proposals/non-ci-architecture/11-target-architecture.md).
Docker supplies disposable execution isolation; it does not become a second runner
protocol, queue, scheduler, evidence store or acceptance authority. Reuse the existing
`statusgen verifyrun` witness and this brief's conformance kit. Supervisor /26 and
adapter /22 retain process-launch ownership, and /27 retains standing-desk adoption;
this local verification path neither implements nor activates those consumers. Future
runner integration must consume the same contract and evidence, not recreate this
verification workflow. Image release/pinning automation remains its existing owner's
work and can proceed independently of this local witness attempt.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Task

1. Define start/observe/cancel/reconcile with stable launch request identity, caller namespace, execution mode (standing-desk or workflow-stage), role packet, pinned model/skill/tool profile, workspace reference, externally validated authority/claim generation and budget reservation. Require a desk binding ID for standing mode or canonical work/node/attempt references for workflow mode; never invent a workflow instance to launch a desk. Define typed results/artifacts/usage and unknown states; preserve source trust and credential exclusion.
2. Declare adapter capabilities for resume, cancellation acknowledgment, launch versus per-request budgets, telemetry and snapshot access. Never interpret missing telemetry as zero, process exit as accepted output, or cancel request as confirmed stop. No silent provider/model fallback.
3. Ship a fake runner and adapter conformance kit covering lost launch acknowledgment, late output from a fenced attempt, malformed result, unauthorized tool request, missing cost, cancelled-but-still-running and unsupported resume. Authentication/role enforcement is independent of model text. Include one fixture that bypasses the contract's generation fence to show the caller's own claim-generation check still refuses a fenced result.
4. Document the protocol and compatibility rules; bootstrap `loopadmin/go.mod` as an independent module, its module test job and path triggers, and a changelog fragment. There is no `go.work` at the repository root and this brief adds none: a consumer links the module by `require` plus a relative `replace`, the way `tools/loopresolve/go.mod` links `cellconfig`. No real provider client or provider call in this unit.

## Interface contract

A caller-scoped invocation ID and validated authority envelope enter; workflow references are required only in workflow-stage mode. Existing work IDs remain references, not replacement IDs. Typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Module and ownership boundary

Add `loopadmin/go.mod` (planned) and `.github/workflows/loopadmin.yml` (planned) to files in scope; no `go.work` is created. This brief bootstraps the shared module independently of graph-execution/19. The contract exposes a narrow caller API plus adapter API; it does not select queue work, advance graph nodes, mint role credentials, approve results or offer operator administration. Supervisor /26 owns launch journaling. Optional session continuity is capability-declared and pinned to a role/profile; no heartbeat or idle poll creates another model request.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | exit 0; named PASS; unknown launch requires reconcile and cannot create a concurrent replacement; mutation: map a lost launch acknowledgment to definite failure — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | exit 0; named PASS; fenced generation cannot be accepted even when its result is well formed; mutation: skip the generation fence when accepting a result — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | exit 0; named PASS; secrets are rejected and missing usage remains unknown; mutation: default missing usage to zero — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | exit 0; named PASS; standing-desk request works with no graph fields; workflow-stage request missing canonical references is refused; shared mandatory capability checks reject both modes; mutation: make canonical work references optional in workflow-stage mode — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | exit 0; named PASS; with the contract's generation fence bypassed, a fenced attempt's well-formed result reported as success is refused by the caller's own claim-generation check; mutation: accept the result in the caller without rechecking its claim generation — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

### 2026-10-03 desk dispatch — 5/5 Verify rows pass on merged main d18adcd974d1, every named mutation killed

Subject: merged main d18adcd974d1 (git HEAD equals the forge API main head), loopadmin module landed by #2082. Each row ran its brief command verbatim from the loopadmin module directory with GOWORK=off. Test names are abbreviated below: TRC stands for TestRunnerConformance and TR for TestRunner. Host: darwin/arm64, go1.27.1. Linux witness: golang:1.25-bookworm (go1.25.14), network none, read-only mount.

| # | Command | Expected | Observed | Date / Runner |
|---|---|---|---|---|
| 1 | Verify row 1 command (go test -run the TRC UnknownLaunch test, grep for its named PASS) | exit 0; named PASS; lost ack mapped to definite failure turns it red | exit 0; `--- PASS: TRC UnknownLaunch`. Mutation map entry (lost ack read as definite failure) KILLED. Independent mutation (every adapter Start error becomes definite failure) KILLED: the test reports that a lost ack got launch definitely failed, wanting reconcile required | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 command (TRC LateResultDenied) | exit 0; named PASS; skipping the generation fence turns it red | exit 0; `--- PASS: TRC LateResultDenied`. Map entry (fence compare replaced by false) KILLED. Independent mutation (fence compare weakened from not-equal to less-than, so an older attempt passes) KILLED: late output from a fenced attempt got nil, wanting the fenced-generation refusal | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 command (TRC CredentialAndUsage) | exit 0; named PASS; missing usage defaulted to zero turns it red | exit 0; `--- PASS: TRC CredentialAndUsage`. Map entry (usage sum treats a nil figure as 0) KILLED. Independent mutation at a different site (the observation reading step maps a nil figure to 0) KILLED: missing cost must stay unknown, got known=true | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 command (TR BothModesWithoutGraph) | exit 0; named PASS; optional workflow references turn it red | exit 0; `--- PASS: TR BothModesWithoutGraph`. Map entry (workflow reference check disabled) KILLED. Independent mutation (only the work ID required, node and attempt IDs dropped) KILLED: a workflow request missing a canonical reference was not refused with the references error | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 command (TR CallerRecheck) | exit 0; named PASS; caller accepting without the claim recheck turns it red | exit 0; `--- PASS: TR CallerRecheck`. Map entry (caller recheck replaced by false) KILLED. Independent mutation (caller recheck weakened from not-equal to less-than) KILLED: the caller's own claim-generation check must refuse the fenced result, got nil | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |

Supporting checks, same subject:
- Implementer mutation gate (go run ./mutate -map testdata/mutations.json, run in a scratch copy): exit 0, 196 of 196 mutations KILLED, none survived or errored.
- Independent mutations (five, written by this verifier, different spellings and sites from the map) applied to a scratch copy of the module: 5 of 5 KILLED. The worktree stayed clean afterwards.
- Whole module: gofmt -l lists nothing; go vet ./... exit 0; go test -count=1 ./... exit 0 across mutate, runner and runner/conformance.
- Linux network-off rerun of all five row commands in the container: 5 of 5 exit 0 with the named PASS.
- Witness, statusgen verifyrun --brief on this brief with --dry-run under a throwaway HOME. The pinned v1.0.31 binary on the darwin host does not execute check:ci rows, by design: its network sandbox uses unshare --net, which is Linux-only. A statusgen built offline from main d18adcd974d1 source then ran the same dry-run in the Linux container (network none, SYS_ADMIN for unshare, read-only scratch clone at the same head): rows 1 to 5 pass exit=0, runner assay-verifier-app[bot] @ d18adcd974d1, exit 0. Nothing was written back.
- Context files: loopadmin/runner, loopadmin/testdata/runner, spec/loop-admin-runner-v1.md, schemas/loop-admin-runner-v1.json, loopadmin/README.md, the changelog fragment for this brief (changelog directory, graph-execution-20 runner-contract) and loopadmin/go.mod all exist on main. There is no go.work at the repository root, as the brief requires. The module test job is at ci/staged-workflows/loopadmin.yml, staged for maintainer promotion because no App here can push workflow files. It is not yet live at .github/workflows/loopadmin.yml, so no CI job runs these five tests yet: ci.yml only builds and vets the module. The changelog fragment and the staged-workflows README both say this.
- Vacuity: every row drives the production runner.Client against the in-memory fake adapter, and each one turns red under its mutation. Row 5's caller is the kit's ReferenceCaller fixture. Spec section 8 states that this proves the reference caller only, and each real consumer (26, 21) must prove its own recheck.

Risk-bearing value: the trigger did not fire. Risk metadata is present and every field is 'no', none of the diff paths match a glob in .assay-surfaces (the staged workflow sits outside .github/workflows), and no repo-pinned hard value changed. Enumeration over the runner package, for the record:
- RISK-VALUE: DERIVED — Version = "loop-admin-runner/v1" @ loopadmin/runner/request.go:15 — matches the spec's version field rule (spec line 35) and the schema const (schema line 12) exactly. Any other value is refused, and the version can be changed by an edit plus a re-release.
- RISK-VALUE: DERIVED — authority.generation zero refused (Generation == 0) @ loopadmin/runner/request.go:351 and loopadmin/runner/result.go:111 — the spec requires a generation of at least 1 (spec line 43). Zero is Go's value for an omitted field, so refusing it keeps an absent authority generation from passing as a real claim.
- RISK-VALUE: NAMED, NOT DERIVED — credential-floor minimum lengths {20,} / {10,} / {8,} @ loopadmin/runner/secrets.go:20-27 — these are heuristic token-shape thresholds. Spec line 57 declares the check a floor, not a scanner, and no first-principles length bound exists to derive from. All of them are reversible by an edit.
- Reversible knobs, ranked last and not derived: the mutation and CI per-test timeout of 120s, and go 1.25.0 in go.mod.

**VERIFY: PASS — all 5 Verify rows exit 0 with their named PASS on merged main d18adcd974d1 (darwin and Linux network-off), and each row's named mutation plus an independent one turns its test red**
### 2026-10-07 re-verify — 5/5 Verify rows pass on merged main 91f04b81ba06, every named mutation killed

Subject: merged main 91f04b81ba064394aa121a4940cf11a138b55402 (fetched 2026-10-07; the loopadmin module, spec, schema and staged workflow are byte-identical to the 2026-10-03 subject d18adcd974d1). Each row ran its brief command verbatim from the repo root with GOWORK=off. Host: darwin/arm64, go1.27.1. Linux witness: golang:1.25-bookworm (go1.25.14), network none, read-only mount of a scratch clone at the same head.

| # | Command | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | exit 0; named PASS; lost ack mapped to definite failure turns it red | exit 0; named PASS line present (UnknownLaunch case, 0.00s). Map mutation (lost ack read as definite failure) KILLED. Independent mutation (held-authority guard disabled, so a replacement can launch beside an unknown attempt) KILLED | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | exit 0; named PASS; skipping the generation fence turns it red | exit 0; named PASS line present (LateResultDenied case). Map mutation (fence compare replaced by false) KILLED. Independent mutation (fence compare fires only when the fence reads zero) KILLED with: late output from a fenced attempt: got nil, want runner: result is from a fenced authority generation | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | exit 0; named PASS; missing usage defaulted to zero turns it red | exit 0; named PASS line present (CredentialAndUsage case). Map mutations (usage sum treats a nil figure as 0; credential pattern match always says no) both KILLED. Independent mutation (Add sums cost with an unknown side read as zero) KILLED | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | exit 0; named PASS; optional workflow references turn it red | exit 0; named PASS line present (BothModesWithoutGraph case). Map mutation (workflow reference check disabled) KILLED. Independent mutation (attempt id alone dropped from the workflow reference check) KILLED | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | exit 0; named PASS; caller accepting without the claim recheck turns it red | exit 0; named PASS line present (CallerRecheck case). Map mutation (caller recheck replaced by false) KILLED. Independent mutation (caller reads the generation from the accepted result instead of its own claim record) KILLED with: the caller's own claim-generation check must refuse the fenced result, got nil | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Supporting checks, same subject:
- Implementer mutation gate (go run ./mutate -map testdata/mutations.json from the loopadmin directory; the gate copies the module to a temp dir per entry and counts a build failure as an error, never a kill): exit 0, 196 of 196 KILLED, none survived or errored.
- Independent mutations: five, written by this verifier at different sites or spellings from the map and from the 2026-10-03 verifier's, run through the same gate: 5 of 5 KILLED. The home worktree stayed clean throughout.
- Whole module: gofmt -l lists nothing; go vet ./... exit 0; go test -count=1 ./... exit 0 (mutate, runner, runner/conformance pass; runner/fake has no test files).
- Linux network-off rerun of all five row commands in the container: 5 of 5 exit 0 with the named PASS.
- Witness: statusgen verifyrun --brief on this brief. The pinned v1.0.32 darwin binary declines check:ci rows by design (its network sandbox is unshare --net, Linux-only), so it produced no pass witness on this host. A statusgen built offline from main 91f04b81ba06 source (linux/arm64) ran the same verifyrun with --dry-run in the Linux container (network none, SYS_ADMIN for unshare, read-only clone at the same head): rows 1 to 5 pass exit=0, runner assay-verifier-app[bot] @ 91f04b81ba06, exit 0. Nothing was written back.
- Context files: loopadmin runner package, loopadmin testdata runner fixtures, the runner spec (spec directory, loop-admin-runner-v1), the runner schema (schemas directory, loop-admin-runner-v1), the loopadmin README and loopadmin go.mod all exist on main; there is no go.work at the repository root. The changelog fragment was aggregated into CHANGELOG.md under v1.0.32 (line 51) by the release aggregation that clears the changelog directory. The module test job remains staged at ci/staged-workflows/loopadmin.yml for maintainer promotion and is not live under .github/workflows, so no CI job runs these five tests yet; ci.yml discovers every go.mod and builds and vets the module. Unchanged since 2026-10-03 and disclosed in CHANGELOG.md.
- Vacuity: every row drives the production runner Client against the in-memory fake adapter, and each turns red under its mutation. Row 5's caller is the kit's ReferenceCaller fixture; spec section 8 states this proves the reference caller only, and each real consumer (26, 21) must prove its own recheck.

Risk-bearing value: the fail-safe trigger did not fire. The frontmatter risk block is present with every field 'no' (irreversible 'no'), none of the diff paths match a glob in .assay-surfaces (the staged workflow sits outside .github/workflows), and no repo-pinned hard value changed. Enumeration over the runner package, spec and schema, for the record:
- RISK-VALUE: DERIVED — Version = "loop-admin-runner/v1" @ loopadmin/runner/request.go:15 — equals the spec's version rule (spec line 35) and the schema const (schema line 12); any other value is refused; changeable by edit plus re-release.
- RISK-VALUE: DERIVED — authority generation zero refused (Generation == 0) @ loopadmin/runner/request.go:351 and loopadmin/runner/result.go:111 — the spec requires generation at least 1 (spec line 43; schema minimum 1 at lines 67 and 143). Zero is Go's value for an omitted field, so refusing it keeps an absent generation from passing as a real claim.
- RISK-VALUE: DERIVED — budget limits refused below 0 (MaxTokens < 0 or MaxCostMicros < 0) @ loopadmin/runner/request.go:374 and usage figures below 0 read as unknown @ loopadmin/runner/usage.go:33 — matches schema minimum 0 (lines 79-80, 110-112); a negative figure is not a measurement.
- RISK-VALUE: NAMED, NOT DERIVED — credential-floor minimum lengths {20,} / {10,} / {8,} and AKIA [0-9A-Z]{16} @ loopadmin/runner/secrets.go:20-27 — heuristic token-shape thresholds; spec line 57 declares the check a floor, not a scanner, so there is no first-principles bound to derive from. All reversible by an edit.
- Reversible knobs, ranked last and not derived: per-test timeout 120s (mutate gate, staged workflow line 68) and go 1.25.0 in loopadmin go.mod line 3.

**VERIFY: PASS — all 5 Verify rows exit 0 with their named PASS on merged main 91f04b81ba06 (darwin and Linux network-off), and each row's named mutation plus an independent one turns its test red**

### 2026-10-11 re-verify — 5/5 Verify rows pass on merged main 1235e830437a, every named mutation killed

Subject: merged main 1235e830437adfaf08c69f93a4d894df85426f4e, a detached home cut from origin/main. Since the 2026-10-07 subject 91f04b81ba06, the runner package, conformance kit, spec and schema have not changed. The one loopadmin change is the mutation gate's per-run GOCACHE quarantine (#2506, in the mutate directory only). Admission: deskdispatch --check-verifier exited 0 before any row ran.

Verification-Attestation: medici-finance/assay#2599 run=9be0fd70ead45fd7d4d7fc585d326a7d74c462a2af7ff444132e9fe1e1860dee source=1235e830437adfaf08c69f93a4d894df85426f4e brief=docs/streams/graph-execution/brief-20-runner-contract.md model=claude-opus-5-5 tier=any

Each row's command was taken character for character from the Verify table and run with bash from the repo root of the home. Host: darwin/arm64, go1.27.1, git 2.56.0. Each row also got a diagnostic rerun with GIT_CONFIG_GLOBAL=/dev/null and GIT_CONFIG_NOSYSTEM=1, so that host git signing config could not affect it. Both runs gave the same result on every row.

| # | Command | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | exit 0; named PASS; lost ack mapped to definite failure turns it red | as written: exit 0, named PASS line present (UnknownLaunch, 0.00s); diagnostic rerun without global git config: exit 0. Map mutation (lost ack read as definite failure) KILLED. Independent mutation IND-1 (the ack-lost branch deletes the held authority before it returns reconcile-required) KILLED with: concurrent replacement under the same authority: got nil, want runner: unknown or unfinished launch requires reconcile | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | exit 0; named PASS; skipping the generation fence turns it red | as written: exit 0, named PASS line present (LateResultDenied); diagnostic rerun: exit 0. Map mutation (fence compare replaced by false) KILLED. Independent mutation IND-2 (the fence reads the attempt's own generation instead of calling the caller's Fence, so the fence trusts the attempt's word) KILLED with: late output from a fenced attempt: got nil, want runner: result is from a fenced authority generation | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | exit 0; named PASS; missing usage defaulted to zero turns it red | as written: exit 0, named PASS line present (CredentialAndUsage); diagnostic rerun: exit 0. Map mutations (usage sum treats a nil figure as 0; credential pattern match always says no) both KILLED. Independent mutation IND-3 (Usage.Complete reports known when any one figure is known) KILLED with: missing cost must stay unknown, got CostMicros nil known=true | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | exit 0; named PASS; optional workflow references turn it red | as written: exit 0, named PASS line present (BothModesWithoutGraph); diagnostic rerun: exit 0. Map mutation (workflow reference check disabled) KILLED. Independent mutation IND-4 (a workflow-stage request with no work block at all is admitted; only a partly filled block is checked) KILLED with: workflow request with no references: got reconcile-required, want runner: references do not match execution mode | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | exit 0; named PASS; caller accepting without the claim recheck turns it red | as written: exit 0, named PASS line present (CallerRecheck); diagnostic rerun: exit 0. Map mutation (caller recheck replaced by false) KILLED. Independent mutation IND-5 (the reference caller overwrites its own claim record with the accepted result's generation before it rechecks) KILLED with: the caller's own claim-generation check must refuse the fenced result, got nil | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Supporting checks, same subject:
- Implementer mutation gate: go run ./mutate -map testdata/mutations.json from the loopadmin directory, with TMPDIR set to a verifier scratch directory outside the home. Exit 0; 196 of 196 KILLED; none survived or errored. Both scratch TMPDIRs were empty afterwards, so the #2506 per-run GOCACHE quarantine removed its cache as designed.
- Independent mutations: five, written by this verifier at sites and spellings that differ from the map and from the 2026-10-03 and 2026-10-07 verifiers' mutations. They ran through the same gate from a map kept in verifier scratch: exit 0, 5 of 5 KILLED. The home stayed clean.
- Whole module: gofmt -l lists nothing; go vet ./... exit 0; go test -count=1 ./... exit 0. The mutate, runner and runner/conformance packages pass; runner/fake has no test files.
- Witness, host: statusgen v1.0.37 verifyrun --brief on this brief with --dry-run, run from the home. Exit 2, rows 1 to 5 could-not-run. These are check:ci rows, and on darwin the network-off sandbox (unshare --net, Linux only) is unavailable. This is the tool's by-design refusal and says nothing about the rows. The rows were therefore run by hand, as the table above records.
- Witness, Linux network-off: the local image ghcr.io/medici-finance/verify-runner:v1.0.34 (linux/amd64, go1.25.14, statusgen v1.0.34), run with --network none; only the loopback interface was up. The tree was a scratch clone made from a git bundle of HEAD 1235e830437a, so no host path was mounted. Rows 1 to 5 run by hand there: all exit 0. statusgen verifyrun --dry-run --sandbox container-netns: exit 0, rows 1 to 5 pass exit=0. The runner it derived was assay-verifier-app[bot] @ 1235e830437a (git-config), from git identity that this verifier injected through the container's environment. Nothing was written back. This is not the harness image pinned in plugins/assay/paired-versions.yaml (desk-tools v1.0.29, by digest). That image is not present on this host, and pulling it was out of scope offline.
- Context files: the loopadmin runner package, the loopadmin testdata runner fixtures, the runner spec (spec directory, loop-admin-runner-v1), the runner schema (schemas directory, loop-admin-runner-v1), the loopadmin README and loopadmin go.mod (go 1.25.0, no requires) all exist on main. There is no go.work at the repository root. The changelog fragment has been aggregated into CHANGELOG.md, now at line 517. The module test job is still staged at ci/staged-workflows/loopadmin.yml and is not live under .github/workflows, so no CI job runs these five tests yet. This is unchanged since 2026-10-03 and disclosed.
- Vacuity: every row drives the production runner Client against the in-memory fake adapter, and each row turns red under both its map mutation and an independent one. Row 5's caller is the kit's ReferenceCaller fixture. Spec section 8 states that this proves the reference caller only; each real consumer (26, 21) must prove its own recheck.

Risk-bearing value: the fail-safe trigger did not fire. The frontmatter risk block is present and every field is 'no', including irreversible. No loopadmin, spec, schema or staged-workflow path matches an entry in .assay-surfaces. No repo-pinned hard value changed: the runner package is unchanged since the last subject. Enumeration over the runner package, spec and schema at this head, for the record:
- RISK-VALUE: DERIVED — Version = "loop-admin-runner/v1" @ loopadmin/runner/request.go:15. It equals the schema const (schema line 12), and any other value is refused. It can be changed by an edit plus a re-release.
- RISK-VALUE: DERIVED — authority generation zero refused (Generation == 0) @ loopadmin/runner/request.go:351 and loopadmin/runner/result.go:111. The spec requires a generation of at least 1 (spec line 43), and the schema sets minimum 1 at lines 67 and 143. Zero is Go's value for an omitted field, so refusing it keeps an absent generation from passing as a real claim.
- RISK-VALUE: DERIVED — budget limits refused below 0 (MaxTokens < 0 or MaxCostMicros < 0) @ loopadmin/runner/request.go:374, and a usage figure below 0 reads as unknown @ loopadmin/runner/usage.go:33. Both match the schema's minimum 0 (lines 79-80 and 110-112). A negative figure is not a measurement.
- RISK-VALUE: NAMED, NOT DERIVED — credential-floor minimum lengths {20,} / {10,} / {8,} and AKIA [0-9A-Z]{16} @ loopadmin/runner/secrets.go:20-27. These are heuristic token-shape thresholds. Spec line 57 declares the check a floor, not a scanner, so there is no first-principles bound to derive them from. All of them are reversible by an edit.
- Reversible knobs, ranked last and not derived: the per-test timeout of 120s (mutate gate line 130, staged workflow line 68) and go 1.25.0 at loopadmin go.mod line 3.

**VERIFY: PASS** — all 5 Verify rows exit 0 with their named PASS on merged main 1235e830437a (darwin as written, darwin with no global git config, and Linux network-off), and each row's named mutation plus an independent one turns its test red
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 0cb90058470f (5/5 rows exit 0 by hand, every named and independent mutation killed; witness held on the darwin sandbox gap)

Subject: merged main 0cb90058470f9608f76b96e2055e7327d9c5c178, a detached home cut from origin/main. Admission: deskdispatch --check-verifier exited 0 before any row ran. Since the 2026-10-11 subject 1235e830437a (an ancestor of this head), nothing under loopadmin, the runner spec, the runner schema or the staged loopadmin workflow has changed; git diff between the two heads over those paths is empty.

Verification-Attestation: medici-finance/assay#2896 run=af935d658fa5c965ec966e553f73a2a68e9f2333b273082da3ee0d8ea80c3146 source=0cb90058470f9608f76b96e2055e7327d9c5c178 brief=docs/streams/graph-execution/brief-20-runner-contract.md model=claude-opus-5-5 tier=strong

Each row's command was copied character for character from the Verify table and run with bash from the repo root of the home, with TMPDIR set to verifier scratch outside the home. Host: darwin/arm64, go1.27.2, git 2.56.0. The home was clean afterwards.

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | pass exit=0 | exit 0; named PASS line present (UnknownLaunch case, 0.00s). Map mutation (lost ack read as definite failure) KILLED. Independent mutation IND-A (an unknown invocation no longer counts as holding its authority) KILLED | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 2 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | pass exit=0 | exit 0; named PASS line present (LateResultDenied case, 0.00s). Map mutation (fence compare replaced by false) KILLED. Independent mutation IND-B (the fence tolerates one generation of drift: refuses only when current exceeds the attempt by more than one) KILLED | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 3 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | pass exit=0 | exit 0; named PASS line present (CredentialAndUsage case, 0.00s). Map mutations (missing usage defaulted to zero; credential pattern match always says no) both KILLED. Independent mutation IND-C (acceptance reports usage known when input tokens alone are known) KILLED | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 4 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | pass exit=0 | exit 0; named PASS line present (BothModesWithoutGraph case, 0.00s). Map mutation (workflow reference check disabled) KILLED. Independent mutation IND-D (workflow references refused only when all three are blank together) KILLED | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 5 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | pass exit=0 | exit 0; named PASS line present (CallerRecheck case, 0.00s). Map mutation (caller accepts without its claim recheck) KILLED. Independent mutation IND-E (the reference caller tolerates one generation of drift between its claim and the result) KILLED | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |

Supporting checks, same subject:
- Implementer mutation gate: go run ./mutate -map testdata/mutations.json from the loopadmin directory of a no-hardlinks scratch clone at this head, TMPDIR in verifier scratch. Exit 0; 196 of 196 KILLED; none survived or errored.
- Independent mutations: five, IND-A to IND-E, written by this verifier at sites or spellings that differ from the map and from the 2026-10-03, 2026-10-07 and 2026-10-11 verifiers' mutations. They ran through the same gate from a map kept in verifier scratch: exit 0, 5 of 5 KILLED.
- Whole module, in the home: gofmt -l lists nothing; go vet ./... exit 0; go test -count=1 ./... passes the mutate, runner and runner/conformance packages (runner/fake has no test files). The home stayed clean.
- Witness, host: statusgen v1.0.41 verifyrun --brief on this brief with --dry-run, run from the home. Exit 2; rows 1 to 5 could-not-run with sandbox=unshare, because these are check:ci rows and the network-off sandbox (unshare --net) is Linux-only and this host is darwin. This is the tool's by-design refusal and says nothing about the rows; it is the open hold medici-finance/assay#1800. Nothing was written back.
- Witness, Linux network-off: could not be produced this run. The container engine on this host has no running daemon, so neither the pinned harness image (plugins/assay/paired-versions.yaml, desk-tools v1.0.29 by digest) nor a local verify-runner image could be started. No Linux execution witness exists at this head.
- Context files: the loopadmin runner package, the loopadmin testdata runner fixtures, the runner spec and schema (loop-admin-runner-v1), the loopadmin README and loopadmin go.mod (go 1.25.0, no requires) exist on main, and there is no go.work at the repository root. The module test job is still staged at ci/staged-workflows/loopadmin.yml and is not live under .github/workflows, so no CI job runs these five tests yet. This has not changed since 2026-10-03 and is disclosed.
- Vacuity: every row drives the production runner Client against the in-memory fake adapter, and each row turns red under its map mutation and under an independent one. Row 5's caller is the kit's ReferenceCaller fixture; spec section 8 says this proves the reference caller only, and each real consumer (26, 21) must prove its own recheck.

Risk-bearing value: the fail-safe trigger did not fire. The frontmatter risk block is present and every field is 'no', irreversible included. No loopadmin, spec, schema or staged-workflow path changed since the last subject, so no repo-pinned hard value changed. Enumeration over the runner package, spec and schema at this head, for the record:
- RISK-VALUE: DERIVED — Version = "loop-admin-runner/v1" @ loopadmin/runner/request.go:15 — equals the schema const (schemas/loop-admin-runner-v1.json line 12), and any other value is refused; it changes only by an edit plus a re-release.
- RISK-VALUE: DERIVED — authority generation 0 refused (Generation == 0) @ loopadmin/runner/request.go:351 and loopadmin/runner/result.go:111 — the spec requires a generation of at least 1 (spec line 43) and the schema sets minimum 1 (lines 67 and 143). Zero is Go's value for an omitted field, so refusing it keeps an absent generation from passing as a real claim.
- RISK-VALUE: DERIVED — budget limits refused below 0 (MaxTokens < 0 or MaxCostMicros < 0) @ loopadmin/runner/request.go:374, and a usage figure below 0 reads as unknown @ loopadmin/runner/usage.go:33 — both match the schema's minimum 0 (lines 79-80 and 110-112); a negative figure is not a measurement.
- RISK-VALUE: DERIVED — artifact hash shape {64} @ loopadmin/runner/result.go:102 — a sha256 digest is 256 bits, which is 64 lowercase hex characters at 4 bits each.
- RISK-VALUE: NAMED, NOT DERIVED — credential-floor minimum lengths {20,} @ loopadmin/runner/secrets.go:20, 21, 22 and 25; AKIA [0-9A-Z]{16} @ loopadmin/runner/secrets.go:23; {10,} @ loopadmin/runner/secrets.go:24; {8,} @ loopadmin/runner/secrets.go:27; basic-auth {4,} @ loopadmin/runner/secrets.go:76 — heuristic token-shape thresholds. Spec line 57 declares the check a floor, not a scanner, so there is no first-principles bound to derive them from. All are reversible by an edit.
- Reversible knobs, ranked last and not derived: the per-test timeout of 120s (loopadmin mutate gate line 130, staged workflow line 68) and go 1.25.0 at loopadmin go.mod line 3.

**VERIFY: BLOCKED** — all 5 Verify rows exit 0 with their named PASS on merged main 0cb90058470f (darwin, as written), and each row's map mutation plus an independent one turns its test red; the statusgen execution witness could-not-run on this darwin host and no Linux witness was available, so the environment hold medici-finance/assay#1800 stays open and status stays implemented.

### 2026-10-19 Linux network-off execution witness at 3d29dd923ab7 (container-netns, 5/5 pass)

Subject: merged main 3d29dd923ab71561ad9a0fc7eec3965367587f76, a detached home cut from origin/main. Admission: deskdispatch --check-verifier exited 0 before the witness ran and again after it. Runner: a local docker container, image assay-verify:go1.27 (image id sha256:955fd6c46c93, linux/arm64, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e; go1.27.2, git 2.47.3), started with --network none and the engine's default seccomp profile; a probe of the same image under --network none listed only the lo interface. The tree was a throwaway clone of the home at this head with modules pre-fetched before the network-off run (GOPROXY=off, GOTOOLCHAIN=local); statusgen and the desk tools were built from that tree inside the container, and statusgen verifyrun --sandbox container-netns --brief ran non-dry and exited 0, writing the table below. This run follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Since the 2026-10-14 subject 0cb90058470f, the only change under loopadmin, the runner spec and schema, and the staged loopadmin workflow is the Go toolchain directive (go 1.25.0 to go 1.27.0 in the loopadmin go.mod, GO_VERSION 1.25.0 to 1.27.2 in the staged workflow), a reversible knob; the risk block is present with every field 'no', so the fail-safe risk-value trigger did not fire and the 2026-10-14 enumeration stands. Container inputs, by name: the network-off run mounted only the throwaway clone of the attested tree (read-write, as the working directory), the Go module cache and the Go build cache (read-write), and the desk roster configuration file (read-only); the clone carried a local git user.name and user.email setting, and the environment passed in was GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-count=1 and KUBECONFIG=/dev/null. The earlier module pre-fetch was a separate networked run that mounted only the clone and the module cache. No credential, token, SSH key, kubeconfig, forge configuration or Docker socket was mounted or passed into either run.

Verification-Attestation: medici-finance/assay#3555 run=1fb61b2f5760b856bf6c167fc45a01731c4f78f95f8a5005cf0969cc6e61f74b source=3d29dd923ab71561ad9a0fc7eec3965367587f76 brief=docs/streams/graph-execution/brief-20-runner-contract.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3d29dd923ab7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3d29dd923ab7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3d29dd923ab7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3d29dd923ab7 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3d29dd923ab7 (on-behalf-of human:ian) (forge-identity) |

The substance is the 2026-10-14 by-hand Evidence block above at merged main 0cb90058470f: all 5 Verify rows exit 0 with their named PASS and every map and independent mutation killed, held only on the execution environment.

**VERIFY: PASS** — all 5 Verify rows pass with 0 fail in the statusgen verifyrun witness on a Linux network-off container (sandbox=container-netns) at merged main 3d29dd923ab7, which closes the environment-only hold on the 2026-10-14 by-hand result.

### 2026-10-20 receipt-bearing re-pass — Linux network-off execution witness at b73d90d774fc (container-netns, 5/5 pass)

Subject: merged main b73d90d774fca8aec8974ef410da63ebcb10eece, a detached home cut from origin/main. This pass re-runs the witness at a current head so that a verify-outcome receipt can be derived from it; the latest prior Evidence on main is the 2026-10-19 execution-witness block above at 3d29dd923ab7, landed by medici-finance/assay#3579. Admission: deskdispatch --check-verifier exited 0 before the witness ran and again after it. Runner: a local docker container, image assay-verify:go1.27 (image id sha256:955fd6c46c93, linux/arm64, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e; go1.27.2, git 2.47), started with --network none and the engine's default seccomp profile. The tree was a throwaway clone of the home at this head with modules pre-fetched before the network-off run (GOPROXY=off, GOTOOLCHAIN=local); statusgen and the desk tools were built from that tree inside the container, and statusgen verifyrun --sandbox container-netns --brief ran non-dry and exited 0, writing the table below. This run follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Between 3d29dd923ab7 and this head, nothing changed under loopadmin, the runner spec and schema, or the staged loopadmin workflow (git diff over those paths is empty), so the risk block (present, every field 'no') and the 2026-10-14 risk-value enumeration stand. Each row's output hash is the empty-input sha256, as expected: every row command redirects its go test output to a temp file and greps it quietly.

Verification-Attestation: medici-finance/assay#3605 run=139baab1c8ae750cbe8aba2fdb29b3982fa5427b9682068172a23d7df1faac7b source=b73d90d774fca8aec8974ef410da63ebcb10eece brief=docs/streams/graph-execution/brief-20-runner-contract.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerBothModesWithoutGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerBothModesWithoutGraph " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerCallerRecheck$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerCallerRecheck " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |

The substance is the 2026-10-14 by-hand Evidence block above at merged main 0cb90058470f: all 5 Verify rows exit 0 with their named PASS and every map and independent mutation killed, held then only on the execution environment.

**VERIFY: PASS** — all 5 Verify rows pass with 0 fail in the statusgen verifyrun witness on a Linux network-off container (sandbox=container-netns) at merged main b73d90d774fc, with the deliverable paths unchanged since the 2026-10-19 witness.

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Execution-environment follow-on routing — 2026-10-16

Reciprocal unblocks only: agent-execution/01. The [execution amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md) adds a follow-on obligation over this brief's existing output. Its Task, Verify, gate, version and historical Evidence are unchanged; this note grants no delivery, reassignment or activation.
