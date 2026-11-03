---
brief: assay:assay:desktools-v2:66
title: Extract shared write policy and make writeguard a hook adapter
why: "Write-isolation decisions are trapped in a command package. Extract one reusable evaluator so command hooks and managed adapters share policy without sharing effects or weakening enforcement."
wave: 1
depends: []
unblocks: ["graph-execution/22"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: Local write-isolation and exemption behavior are security controls; review must confirm policy relocation and all adapters preserve refusals and independent sandbox enforcement.
decision-trigger: spec
issues: [3752]
decision-issue: 3752
design: DR-write-policy-core
ruling: "https://github.com/medici-finance/assay/issues/3752#issuecomment-6299695731"
schema: brief-v2
version: 2
id: 26781d38-3bac-4c4e-8ba2-0688894744fb
authored: 2026-10-18 by Codex planning session
sources:
  - docs/write-policy.md
  - docs/library-first.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-18 @ c3d073572f41e09bedcac05944f9a202c3267353; current writeguard, deskcore and consumer module manifests inspected
exec-tier: strong
exec-tier-why: Cross-module extraction, command classification and security-boundary compatibility require adversarial flow and packaging judgment.
domain: complicated
outcome: none
consumers:
  - "deskcore/writepolicy: follow-up desktools-v2/66"
  - "tools/desk/cmd/writeguard, tools/desk/go.mod, tools/desk/go.sum: follow-up desktools-v2/66"
  - "docs/contracts.md, docs/write-policy-consumers.md, tools/desk/README.md: follow-up desktools-v2/66"
  - "tools/desk/cmd/writeguard CLI parsing: out-of-scope (the parser port, desktools-v2/31, is withdrawn; writeguard keeps its current parser)"
  - "loopadmin/adapters/localharness: follow-up graph-execution/22"
  - "managed launch/harness adapters outside the public runner: out-of-scope (separate consumer plans bind the producer version and qualify allocation/sandbox flow)"
  - "statusgen workspace lifecycle: out-of-scope (consumes workspace; only an actual duplicate write-policy evaluator requires migration)"
  - "push, credential, process, scratch and retirement guards: out-of-scope (independent owners and enforcement boundaries)"
---

# Brief 66 — Extract shared write policy and make writeguard a hook adapter

## Context

files: `deskcore/writepolicy/` (planned), `deskcore/archtest/`, `tools/desk/cmd/writeguard/`, `tools/desk/go.mod`, `tools/desk/go.sum`, `docs/write-policy.md`, `docs/write-policy-consumers.md` (planned), `docs/contracts.md`, `tools/desk/README.md`, `changelog/write-policy.md` (planned) — Release/CI dependency readers and paths are re-enumerated at pickup.

facts:
- `Config`, `Verdict`, path checking and shell classification currently live in writeguard's `package main`; the types are not an importable consumer API.
- Path resolution reads the filesystem; exemption expiry reads time; main/callout read config, Git and process state. Moving files unchanged would not produce a pure evaluator.
- `deskcore` is the existing pure shared module, while neither tools/desk nor statusgen currently requires it at this source cut; explicit compatible module/release integration is owed.
- Legacy parse/Git failure allows; configured callout failure blocks. The semantic distinction and existing protocol must survive extraction.

layering: Pure `deskcore/writepolicy` receives explicit observations; hook/managed adapters collect effects and map outcomes. Task 2–4 and Verify 1–4 prove the boundary; sandbox enforcement remains independent.
design-fit:
  owner: S-write-policy, migrated from command-local decisions to deskcore/writepolicy only when implementation lands
  contract: S-write-policy in docs/contracts.md; docs/write-policy.md
  retires: [command-local policy implementation, duplicate evaluator sites named in the pickup census]
  weight: package +1 inside existing module; production verbs +0; flags +0; legacy refusals +0; one checked consumer inventory
  why-add: Reuse the existing core module; a narrow evaluator removes duplicated policy without moving observations or authority into libraries
single-point-of-failure: Heuristic command recognition cannot confine arbitrary filesystem effects; actual harness/OS write roots remain independent. Verify 3 and managed qualification test those lower boundaries separately.

## Read first

- `docs/write-policy.md`, `docs/library-first.md`, `docs/contracts.md` and `docs/go-engineering.md`.
- `plugins/assay/references/implementation-pickup.md` and current approved target/transition.
- Existing writeguard implementation/tests, deskcore architecture checks and real consumer manifests.

## Human decision

At pickup, present the source-bound compatibility report and proposed core/adapter interface.
Confirm that every existing denial and exemption condition is preserved, the legacy no-decision
failure path remains explicit, and managed admission does not inherit a legacy fallback.
Options:
1. **Approve the bounded extraction** — preserve legacy behavior and qualify the separate managed profile before activation.
2. **Narrow or hold it** — retain current behavior until the missing boundary or compatibility evidence is supplied.
Default if no answer: none — blocks until answered.

The undecided security change remains blocked until answered.

## Ground rules

Own isolated branch and draft PR; no merge, installation, release or workflow trigger.
Offline disposable fixtures only; export KUBECONFIG=/dev/null. Stop at implemented.
No existing control removal/weakening without its recorded authorization; preserve Evidence.

## Task

1. Perform architecture pickup. Census current and planned write-policy consumers and duplicates from fresh source; record revisions, queries, exclusions and each site's migrate/retain/follow-up owner. Migrate bounded actual duplicates here; route larger ones to concrete briefs before claiming closure. Do not relabel allocator users as policy duplicates.
2. Define versioned typed operation/observation/outcome fixtures and implement pure Classify/Evaluate in deskcore/writepolicy. Separate lexical classification from filesystem/ancestor observations. Unknown remains explicit; no environment, filesystem, clock, Git, process, callout, network or credential capability in the transitive core. Preserve sentinel-before-exemption and only-adds-refusals ordering.
3. Convert writeguard into a thin protocol/observation/callout adapter. Retain all hook tool names, JSON/exit/stdout, activation, configuration-source, sentinel and legacy failure behavior. Remove old decision functions when common fixtures pass; command-local wrappers may compose, never reimplement policy. CLI parsing is unchanged: the parser port (/31) is withdrawn, which does not retire this evaluator or /22's consumer.
4. Add the tools/desk module requirement/build wiring and qualify a separate external Go consumer with GOWORK=off. Settle compatible deskcore version/profile and release dependency resolution; source replacements alone are insufficient. Update source/consumer contract coverage and CI path triggers through the existing workflow-landing path.
5. Hand back exact API/profile/source versions to managed consumers, update the semantic owner and checked census, and preserve source-bound regression cases plus before/after observation-cost measurements. No claim that publishing source installs a hook.

## Interface contract

`docs/write-policy.md` defines proposed Classify/Evaluate, explicit path/identity/claim/callout observations and allow/deny/unknown. Settle exact Go/schema surface together. Core imports no workspace or desk internals; managed adapters map validated workspace records. Unknown protocol mapping belongs to named profiles. No exported raw credential/config getter.

## Verify

Named tests and paths below are implementation deliverables, not tests claimed to exist now.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyCases$' ./writepolicy > "$out" && grep -q '^--- PASS: TestWritePolicyCases ' "$out")` | exit 0; independent expected decisions/reasons for shared/owned paths, aliases/ancestors, cwd/quotes/heredocs, unresolved expressions, claim expiry/scoping, agent-supplied exemption evidence refused (an agent cannot mint or renew its own exemption) and sentinel order; identical explicit observations give identical results |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteHookParity$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWriteHookParity ' "$out")` | exit 0; real built hook and direct import use the same production evaluator/fixtures; golden legacy denial/no-decision/activation/callout behavior preserved. Plant skipped deny, exemption-before-sentinel, callout-clears-deny and a managed profile that inherits the legacy no-decision for unknown or missing allocation context in production; each must redden its case |
| 3 | check:ci +flow +mutation | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyBoundary$' ./archtest > "$out" && grep -q '^--- PASS: TestWritePolicyBoundary ' "$out")` | exit 0; transitive effect/import checks plus explicit-input behavior; planted filesystem/helper, clock and command dependency each caught. No pure-library claim from an allowlist alone |
| 4 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyConsumers$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWritePolicyConsumers ' "$out")` | exit 0; source-bound census covers actual adapters/duplicates; added unregistered evaluator and stale consumer fail; external-module import builds offline from a supplied local module proxy with GOWORK=off and no relative replace |
| 5 | check:ci | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/writeguard/... && out=$(mktemp) && report=$(mktemp) && trap 'rm -f "$out" "$report"' 0 && GOWORK=off go test -count=1 -coverpkg=github.com/medici-finance/assay/deskcore/writepolicy -coverprofile="$out" -run '^TestCheck' ./cmd/writeguard && GOWORK=off go tool cover -func="$out" -o "$report" && awk '$2 == "Evaluate" && $3+0 > 0 {found=1} END {exit !found}' "$report")` | exit 0; inherited regression cases retained or explicitly mapped to equivalent owner tests; POSIX and Windows lexical fixtures; actual native support claimed only with corresponding evidence |

## Definition of Done

All Task and named Verify obligations delivered, changelog and consumer docs updated,
module/release/CI reads accounted for, exact hand-back version recorded. Managed sandbox
flow and release qualification remain owned by their consumer plans; extraction alone
never marks them complete. Record invocation counts/elapsed time for identical fixtures
without inventing a performance target or claiming a heuristic is containment.

## Pre-mortem

A moved package still reads disk/time: Verify 3's planted helper and explicit-input tests.
A hook-shaped unit test never runs the binary: Verify 2 must execute the built hook.
A future caller keeps its own rule: Verify 4's census and separate managed flow proof.
A source replace hides a missing published dependency: Verify 4's external consumer.
A refactor weakens a guard while tests only check allow: deny/order mutations in Verify 2.

## Evidence

No implementation or independent verification claimed. Record actual commands, named
results, fixture/source/profile versions, environment and independent runner at delivery.
### Independent merged-main verification — 2026-10-28

Independent non-implementer, model gpt-6.1-sol, admitted tier strong. Observed source 30ca40aae3e9154c001af244fd7e73b4832eaff5. Actual extraction delivery #3772 and CI integration #3793 were read from the forge; later lifecycle reconciliation provides no code-delivery credit. The actual extraction first parent is 4395628b9ff5575e2c5d7d62f8ded2902d1c9ee7. All original Verify definitions are preserved and match the pre-delivery table.

Two independent related-row clusters ran the exact original commands in the release v1.0.64 Go verify-runner, with network none. The runner image reference was read from the checksum-verified release asset verify-runner-image.txt; its checksum a1a8ba23ebb010ce40e796dba906f5050aea9d1f846a7e4b11899af6c39d170c matches that release's checksums.txt. Exact image digest is recorded below. Native admission preceded the rows and each close re-admitted unchanged source. A disposable full-history clone matched the admitted source bytes and the native tree digest; native fail-first created its own baseline worktree without editing the admitted home.

Original rows 1, 2 and 5:

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyCases$' ./writepolicy > "$out" && grep -q '^--- PASS: TestWritePolicyCases ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 30ca40aae3e9 (on-behalf-of human:ian) (forge-identity) | base=4395628b9ff5 red rc=1 · head=30ca40aae3e9 |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteHookParity$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWriteHookParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 30ca40aae3e9 (on-behalf-of human:ian) (forge-identity) | base=4395628b9ff5 red rc=1 · head=30ca40aae3e9 |
| 5 | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/writeguard/...` | pass exit=0 sandbox=container-netns | sha256:9ea29137ee86 | 2026-10-28 | assay-verifier-app[bot] @ 30ca40aae3e9 (on-behalf-of human:ian) (forge-identity) | base=4395628b9ff5 green rc=0 · head=30ca40aae3e9 |
Tree-Digest: sha256:d1168308234518ea9fc731fb1837b19cbf90779824125f916dc7b982bbbee28e
Container-Image: sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0
Verification-Attestation: medici-finance/assay#4787 run=e4318ab7926b69ae986b4b65e270df95aa4bf16ad7d6881af87ee8250a54315f source=30ca40aae3e9154c001af244fd7e73b4832eaff5 brief=docs/streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md model=gpt-6.1-sol tier=strong
Admission-Bracket: run=e4318ab7926b69ae986b4b65e270df95aa4bf16ad7d6881af87ee8250a54315f source=30ca40aae3e9154c001af244fd7e73b4832eaff5 tree=sha256:d1168308234518ea9fc731fb1837b19cbf90779824125f916dc7b982bbbee28e witness=sha256:97fa997a529122aee585d7b5653d62d18291a0e14a0664eaeb37a8e39a1ad5e2 opened=2026-10-28T21:07:10Z closed=2026-10-28T21:14:03Z before=receipt after=check-verifier

Original rows 3 and 4:

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 3 | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyBoundary$' ./archtest > "$out" && grep -q '^--- PASS: TestWritePolicyBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 30ca40aae3e9 (on-behalf-of human:ian) (forge-identity) | base=4395628b9ff5 red rc=1 · head=30ca40aae3e9 |
| 4 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyConsumers$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWritePolicyConsumers ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ 30ca40aae3e9 (on-behalf-of human:ian) (forge-identity) | base=4395628b9ff5 red rc=1 · head=30ca40aae3e9 |
Tree-Digest: sha256:d1168308234518ea9fc731fb1837b19cbf90779824125f916dc7b982bbbee28e
Container-Image: sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0
Verification-Attestation: medici-finance/assay#4787 run=e4318ab7926b69ae986b4b65e270df95aa4bf16ad7d6881af87ee8250a54315f source=30ca40aae3e9154c001af244fd7e73b4832eaff5 brief=docs/streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md model=gpt-6.1-sol tier=strong
Admission-Bracket: run=e4318ab7926b69ae986b4b65e270df95aa4bf16ad7d6881af87ee8250a54315f source=30ca40aae3e9154c001af244fd7e73b4832eaff5 tree=sha256:d1168308234518ea9fc731fb1837b19cbf90779824125f916dc7b982bbbee28e witness=sha256:5b36b8880cc3ddba994470b4f902b918eeeb87699b132aea0e8bddfc2bfc7270 opened=2026-10-28T20:57:46Z closed=2026-10-28T21:01:02Z before=receipt after=check-verifier

Accounting: all five exact head commands passed. Original rows 1, 2, 3 and 4 were red at the actual pre-extraction base and passed at head. Row 5 was green at base and head; native verifyrun marked it non-discriminating and exited 1. Four acceptance obligations passed; one failed the risk-bearing fail-first requirement. This is an acceptance-definition failure, not an observed policy implementation failure.

Tracked repair: medici-finance/assay#4794. Preserve the inherited regression floor and correct the discriminating obligation through the reviewed acceptance-definition path, then obtain fresh independent admission and verification. The verifier changed no Verify or Context definitions. Final gate:human acceptance remains separate.

Uncredited preparation observations are retained: the generic desk-tools image lacked Go; the checksum-qualified Go runner corrected that image selection. A close call with a relative brief from the wrong working directory was refused, then the identical native close succeeded after correcting only invocation paths with the original receipt and witness. A child-added temporary filesystem mounted noexec prevented Go test binaries in an earlier attempt; the corrected run restored the documented default executable temporary directory without changing adopted controls. These setup attempts establish no code failure and no acceptance credit. The successful current closed witnesses above supersede their setup results.

Literal census covered added lines across the actual delivery diff, including relocated legacy literals, fixtures, manifests and docs; source coordinates were retained against the immutable delivery. Highest authority bindings: Managed Profile = managed/v1 at deskcore/writepolicy/policy.go:20, LegacyHook Profile = legacy-hook/v1 at line 17, AgentClaim = agent at line 116, and the TTL nonpositive bound 0 at line 323. The approved bounded extraction requires distinct legacy/managed contracts, preserves the legacy fallback only in its named profile, and refuses agent-minted exemption authority; a nonpositive TTL cannot grant a positive-duration exemption. Classifier tokens and fixture/version numbers rank below these authority bindings and are reversible source/configuration values.

RISK-VALUE: DERIVED — Managed Profile = managed/v1 at deskcore/writepolicy/policy.go:20 and LegacyHook Profile = legacy-hook/v1 at deskcore/writepolicy/policy.go:17 — the approved contract requires separate named profiles so managed admission cannot inherit legacy no-decision semantics; these identifiers bind that versioned separation and grant no activation authority.
RISK-VALUE: DERIVED — AgentClaim = agent at deskcore/writepolicy/policy.go:116 and TTL bound = 0 at deskcore/writepolicy/policy.go:323 — the accepted original contract rejects agent-supplied exemption authority and requires a positive expiry; only the explicit trusted observation sources can satisfy the exemption predicate.

**VERIFY: FAIL** — original row 5 is non-discriminating at the actual pre-extraction base. Status remains implemented.
### Independent merged-main re-verification — 2026-11-01

Independent non-implementer, model claude-opus-5-5, admitted tier strong (statusgen v1.0.67). Observed source 1bfcf9a4dcb5689ed5c9172e35abf95fc2241a88. This run follows the reviewed acceptance-definition repair of the FAIL above: amendment ed72c82a66ba07849fc51378e82e6c3ab400aae3 (medici-finance/assay#4828) changed only the row 5 Command, and the human approval https://github.com/medici-finance/assay/issues/5108#issuecomment-6550841031 (unedited) names exactly that delivery, base, original, amendment and five rows. The repair-tracking issue medici-finance/assay#4794 is still open. Row derivation (deskpathguard rederive) found 5 rows, 0 author-added.

Verification-Table: delivery=86eec0f73399f679d5ea9b8ed81687e186441012 base=ce25be49efeb8a345019c6a663002edfd705da27 original=ce25be49efeb8a345019c6a663002edfd705da27

The delivery 86eec0f7 (medici-finance/assay#3772) is a merge on the first-parent chain of the observed source. Its unique merge base is ce25be49, and its first parent, the actual pre-extraction main snapshot, is 4395628b9ff5575e2c5d7d62f8ded2902d1c9ee7.

Required transition route: could-not-run on this host. A host run of statusgen verifyrun with the acceptance-record descriptor and the forge acceptance check read the approval and reported Acceptance-Changes: 5=Command. Every row then came back could-not-run with this message: "check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin." The container route cannot carry the transition: the bracket verbs refused it with "--open-bracket and --close-bracket are host steps around a container run; they run no rows and take no --sandbox, --container-image-digest, --check, --dry-run, --fail-first, --in-container or --acceptance-record". docs/protected-paths.md also documents container transition execution as unsupported.

Plain bracketed container run of the five current rows. The image is the release v1.0.67 verify-runner, pinned by digest, with network none. The image reference comes from the release asset verify-runner-image.txt, and its sha256 3d56c89c matches that release's checksums.txt. Release v1.0.67 is an ancestor of the source, and the go.mod and go.sum files are identical. Native admission came before the rows, and the close re-admitted unchanged source (bracket verdict match):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyCases$' ./writepolicy > "$out" && grep -q '^--- PASS: TestWritePolicyCases ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteHookParity$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWriteHookParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyBoundary$' ./archtest > "$out" && grep -q '^--- PASS: TestWritePolicyBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyConsumers$' ./cmd/writeguard > "$out" && grep -q '^--- PASS: TestWritePolicyConsumers ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/writeguard/... && out=$(mktemp) && report=$(mktemp) && trap 'rm -f "$out" "$report"' 0 && GOWORK=off go test -count=1 -coverpkg=github.com/medici-finance/assay/deskcore/writepolicy -coverprofile="$out" -run '^TestCheck' ./cmd/writeguard && GOWORK=off go tool cover -func="$out" -o "$report" && awk '$2 == "Evaluate" && $3+0 > 0 {found=1} END {exit !found}' "$report")` | pass exit=0 sandbox=container-netns | sha256:a3fd980e9f42 | 2026-11-01 | assay-verifier-app[bot] @ 1bfcf9a4dcb5 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:93796457ee1c2fb9b349ec6bbef3bfdc96fa55c7877b4c2ac9082ed859095fb0
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5195 run=dc2f28eaecb0296ca45aeb148f6a7670962dfc763692feaf70dc42ced066f8d8 source=1bfcf9a4dcb5689ed5c9172e35abf95fc2241a88 brief=docs/streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=dc2f28eaecb0296ca45aeb148f6a7670962dfc763692feaf70dc42ced066f8d8 source=1bfcf9a4dcb5689ed5c9172e35abf95fc2241a88 tree=sha256:93796457ee1c2fb9b349ec6bbef3bfdc96fa55c7877b4c2ac9082ed859095fb0 witness=sha256:9de32242c0079a9283b728c36ef45b1d498ebdabd23e5a72d950a1d48fdf9dc5 opened=2026-11-01T21:22:04Z closed=2026-11-01T21:32:47Z before=receipt after=check-verifier

Supplemental fail-first probe, for information only. It earns no acceptance credit because it ran outside the bracket and the transition route. A disposable clone was checked out at the actual pre-extraction first parent 4395628b9ff5 and the exact current row commands were run in the same image with network none, offline, using a read-only module cache. All five rows exited 1 at that base. Rows 1–4 are red because the writepolicy package and the named tests do not exist there ("stat /src/deskcore/writepolicy: directory not found" for row 1). Amended row 5 is red because nothing tested there reaches the policy package ("warning: no packages being tested depend on matches for pattern github.com/medici-finance/assay/deskcore/writepolicy", coverage "[no statements]", awk exit 1). It is green at head. So the amended row 5 discriminates, which repairs the earlier FAIL's non-discrimination finding, but the probe is not the credited transition run.

| # | Base | Head |
|---|------|------|
| 1 | base=4395628b9ff5 red rc=1 | head=1bfcf9a4dcb5 pass |
| 2 | base=4395628b9ff5 red rc=1 | head=1bfcf9a4dcb5 pass |
| 3 | base=4395628b9ff5 red rc=1 | head=1bfcf9a4dcb5 pass |
| 4 | base=4395628b9ff5 red rc=1 | head=1bfcf9a4dcb5 pass |
| 5 | base=4395628b9ff5 red rc=1 | head=1bfcf9a4dcb5 pass |

Literal census at the observed source covered deskcore/writepolicy/policy.go. These are the authority bindings: APIVersion writepolicy/v1 at line 12; LegacyHook Profile legacy-hook/v1 at line 17; Managed Profile managed/v1 at line 20; AgentClaim agent at line 135; the sentinel TTL nonpositive bound at line 342; the expiry comparison at line 345; the managed future-mtime refusal at line 348; and validManaged's exclusion of agent and environment claims at line 379. The delivery did not change the writeguard sentinel default TTL (12h, tools/desk/cmd/writeguard/main.go) or the 8 MiB stdin limit. The only literal the amendment adds is row 5's coverage threshold, greater than 0 statements.

RISK-VALUE: DERIVED — Managed Profile = managed/v1 at deskcore/writepolicy/policy.go:20 and LegacyHook Profile = legacy-hook/v1 at deskcore/writepolicy/policy.go:17 — the approved bounded extraction (Human decision; medici-finance/assay#3752 ruling) requires separately named profiles so managed admission never inherits legacy no-decision semantics; the identifiers bind that versioned separation and grant no activation authority.
RISK-VALUE: DERIVED — AgentClaim = agent at deskcore/writepolicy/policy.go:135, excluded by validManaged at deskcore/writepolicy/policy.go:379, and TTL bound 0 at deskcore/writepolicy/policy.go:342 — the accepted contract refuses agent-supplied exemption authority and requires a positive expiry; a nonpositive TTL cannot grant a positive-duration exemption.
RISK-VALUE: DERIVED — row 5 coverage threshold `$3+0 > 0` (amendment ed72c82a66ba) — the approved amendment requires that the inherited writeguard tests actually execute the shared policy's Evaluate; any nonzero statement coverage proves the adapter reaches the shared package, which is the property the original row 5 failed to discriminate.

Disclosures: the docker engine was the local OrbStack engine (linux/amd64 under emulation), with no cluster context. The house Go module cache was mounted read-only for the supplemental probe only. No probe rewrote tracked files in the admitted home.

**VERIFY: BLOCKED** — the head is green on all five current rows (bracket verdict match), and amended row 5 is red at the pre-extraction base. But the approved acceptance transition for row 5 must execute on a host with a network-off `unshare --net` sandbox. This darwin host has none, and the container route cannot carry an acceptance record. Next actor: a Linux verify runner re-runs the transition (verifyrun with the acceptance-record descriptor and forge acceptance check) at a fresh admission. Repair tracking: medici-finance/assay#4794; handoff medici-finance/assay#5112. Final gate:human acceptance remains separate. Status remains implemented.

## Review

Gate: human. Review the compatibility report, all outcomes, consumer closure and
independent enforcement; no review grants merge, install or activation authority.
