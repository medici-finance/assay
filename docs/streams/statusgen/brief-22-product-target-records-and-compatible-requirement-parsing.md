---
brief: assay:assay:statusgen:22
title: Product target records and compatible requirement parsing
why: "A product target currently disappears into prose when a brief closes. Preserve direction, target revisions and observed evidence without changing existing requirement meaning."
wave: 1
depends: []
unblocks: ["statusgen/23", "statusgen/24", "continuing-operations/01"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: 2951f3ce-01cc-46a6-a456-9e6c32e1fa56
consumers:
  - "statusgen/requirements.go, spec/registers-v1.md, schemas/product-intent-v1.json: follow-up statusgen/22"
  - "statusgen/contexthandoff: follow-up statusgen/23"
  - "statusgen/portfolio: follow-up statusgen/24"
  - "statusgen/operations: follow-up continuing-operations/01"
---

# Brief 22 — Product target records and compatible requirement parsing

## Context

files: `spec/registers-v1.md`, `schemas/product-intent-v1.json` (planned), `statusgen/productintent/` (planned), `statusgen/requirements.go`, `statusgen/requirements_test.go`, `docs/contracts.md`; `changelog/statusgen-22-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- The existing requirementEntry stores acceptance/status/satisfied-by, not revisioned product targets.
- REQ IDs and the outcome grammar already exist; satisfied is a claim, not an observed benefit.
- The context handoff implementation is separately owned by statusgen/19; this brief does not edit it.

layering: pure typed records/evaluators in `statusgen/productintent`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/productintent
  contract: product-intent-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Implement the optional product-intent-v1 extension in the REQUIREMENTS owner with direction, target and observed references as specified in the amendment. Retain unique revision history, tombstones and original acceptance/satisfaction claims. Resolve approval from supplied decision evidence, never a record boolean.
2. Provide pure decode/validate/resolve APIs in statusgen/productintent; schema and parser agree on supported versions, required fields and legacy absence. Record unsupported extension versions explicitly. Add typed qualified operational references without implementing operational definitions here.
3. Connect the existing requirement reader without silently accepting unsupported fields as working behavior. Publish the exact supported syntax, compatibility matrix and semantic owner; old records remain valid and unbound.

## Interface contract

Input: base REQ plus optional versioned product-intent extension and explicit decision/assessment references. Output: immutable qualified revisions and known/unknown/unbound states. No observation or approval is manufactured by parsing.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | exit 0; named PASS; legacy REQ parsing and new extension resolve through the actual reader; two target revisions keep their original direction and evidence; mutation: overwrite prior target history |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | exit 0; named PASS; a self-declared approval, unavailable decision or unsupported mandatory version cannot become an approved target; mutation: trust the supplied approved flag |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | exit 0; named PASS; same valid/invalid corpus through JSON schema and parser; include wrong-kind operational reference and withdrawn identity; mutation: permit missing target revision |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

### Independent verification — 2026-10-19, claude-opus-5-5 / assay-verifier-app[bot]

Subject: merged main d8a4d0a941a1c312ebfbce16ecda782a5111eadb (delivering change medici-finance/assay#3296). Verifier is not the implementer. Runtime: go1.27.2 darwin/arm64 on Darwin 27.0.0 (host); go1.27.2 linux/arm64 in a container started with `--network none` (loopback the only interface, tree mounted read-only, GOPROXY=off) for the network-off re-run. No live endpoint was contacted.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | exit 0; named PASS; mutation overwrite-prior-target caught | host exit 0, `--- PASS: TestProductIntentLegacyAndRevisionFlow (0.00s)`; network-off container exit 0, same named PASS. Test drives parseRequirementsDir (the actual reader), legacy entry resolves unbound, two target revisions keep directions d1/d2 and their own observed evidence, CheckHistory refuses a prior-target overwrite. Planted overwrite-prior-target in a scratch snapshot: CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | exit 0; named PASS; mutation trust-record-claim caught | host exit 0, `--- PASS: TestProductIntentApprovalAndUnknown (0.00s)`; network-off container exit 0, same named PASS. Self-claim with no evidence stays unknown; unavailable, wrong-revision, wrong-kind, unauthorized, shared-reference and conflicting decisions stay unknown; mandatory product-intent-v2 resolves unsupported with no targets. Planted trust-record-claim and return-stored-record: both CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | exit 0; named PASS; mutation permit-missing-revision caught | host exit 0, `--- PASS: TestProductIntentSchemaParity (0.87s)`; network-off container exit 0, `--- PASS: TestProductIntentSchemaParity (0.43s)`. One frozen corpus goes through the conform schema validator bridge and the parser; it includes wrong-operational-kind (invalid) and withdrawn (valid). Planted permit-missing-revision, allow-case-alias and schema-missing-revision: all CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Mutation harness: the shipped productintent/testdata/mutate.py was run unmodified against a git-archive snapshot of d8a4d0a94 in verifier scratch, never in this tree. Result: exit 0, 13/13 plants CAUGHT, each by its named test.

Execution witness: `statusgen verifyrun --brief <this brief> --dry-run` (v1.0.50, host) exits 2 with every row could-not-run, because check:ci rows need the `unshare --net` sandbox and this host is darwin. A Linux network-none container run through `--sandbox container-netns` stops before any row with `pre-work verifier admission failed; no Verify rows executed: exec: "deskdispatch": executable file not found in $PATH`, because admission needs the forge and the container has no network. Writing the host witness for real was refused by this session's permission layer. No witness table is recorded here. A Linux runner with `unshare --net` still has to record the hermetic witness before closure.

Risk-bearing values. The brief's risk fields are all present and all `no`, and nothing here is irreversible. Enumeration covered the Go and schema diff of medici-finance/assay#3296 (statusgen/productintent decode.go and records.go, statusgen/requirements.go, schemas/product-intent-v1.json). Literals introduced: Schema = "product-intent-v1" @ statusgen/productintent/records.go:7; nameRE, repoRE, reqRE, textRE and dateRE @ statusgen/productintent/decode.go:15-19; kind enum feature/capability/journey and status enum active/withdrawn @ decode.go:194; confidence enum low/medium/high/unknown @ decode.go:182; operational-reference kinds journey/operational-workflow/practice @ decode.go:219; requirement status set proposed/accepted/satisfied/withdrawn @ decode.go:263; decision verdicts approved/rejected @ decode.go:311. They rank as reversible grammar values: a wrong one mis-validates a record and is fixed by an edit. The two that matter most are derived below.

RISK-VALUE: DERIVED — reqRE = `^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$` @ statusgen/productintent/decode.go:17 — the same pattern as the existing requirementIDRe @ statusgen/requirements.go:115 (registers-v1 §3.4, slug of 10-20 chars) and the schema subject.requirement pattern, so the extension cannot widen or narrow which REQ IDs exist. The reader also refuses a subject that differs from the base id.

RISK-VALUE: DERIVED — operational-reference kinds = "journey", "operational-workflow", "practice" @ statusgen/productintent/decode.go:219 — these match spec registers-v1 line 450 and the schema enum. Delivery workflow-pattern is refused, which keeps continuing-operations identities separate from workflow-pattern-v1 (continuing-operations spec lines 56-69). The requirement status set @ decode.go:263 matches requirementStatusOrder @ statusgen/requirements.go:97.

**VERIFY: PASS** — 3/3 Verify rows exit 0 with the named PASS on host and network-off; 13/13 mutations caught. The hermetic verifyrun witness could not run on this host (see above), so closure still needs a Linux witness.

Verification-Attestation: medici-finance/assay#3438 run=31eb8fda42e94edb8df9ea0774931a64c24621351cfe4488ded9dc08c569328c source=d8a4d0a941a1c312ebfbce16ecda782a5111eadb brief=docs/streams/statusgen/brief-22-product-target-records-and-compatible-requirement-parsing.md model=claude-opus-5-5 tier=strong

### Container-netns execution witness — 2026-10-19, claude-opus-5-5 / assay-verifier-app[bot]

Subject: merged main 4b15482af5c6c71b9312ca7df0f23b219de3f840; the delivery files from medici-finance/assay#3296 are byte-identical to the earlier subject d8a4d0a94 (only docs/contracts.md rows for other rules changed in the declared scope). Verifier is not the implementer. Runner: a local docker container started with `--network none`, GOPROXY=off, GOTOOLCHAIN=local, the engine's default seccomp profile, statusgen and the desk tools built from this tree inside the container, and `statusgen verifyrun --sandbox container-netns` run for real (go1.27.2 linux/arm64), per the driver's 2026-10-19 interim-runner ruling (recorded on the private house tracker). verifyrun exited 0 and wrote the witness table below; `verifyrun --check` in the container reported 3 pass, 0 fail of 3 Verify rows. Admission: `deskdispatch --check-verifier` exited 0 on the host before and after the container run, binding attestation run 264eac3beb7d (full binding on the Verification-Attestation line at the end of this subsection).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:4ad9581a91d6 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:09bf6758cd27 | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:ad3bb01cdeaa | 2026-10-19 | assay-verifier-app[bot] @ 4b15482af5c6 (on-behalf-of human:ian) (forge-identity) |

By-hand runs at 4b15482af5c6 on the host (go1.27.2 darwin/arm64), each Verify command run exactly as written, once plainly and once under a sandbox-exec profile that denies outbound network (control: curl to a public host under the same profile exited 6, could not resolve host).

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | exit 0; named PASS; mutation overwrite-prior-target caught | host exit 0, `--- PASS: TestProductIntentLegacyAndRevisionFlow (0.01s)`; deny-network sandbox exit 0, same named PASS. Shipped mutation harness in a scratch clone at 4b15482af5c6: overwrite-prior-target CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | exit 0; named PASS; mutation trust-record-claim caught | host exit 0, `--- PASS: TestProductIntentApprovalAndUnknown (0.00s)`; deny-network sandbox exit 0, same named PASS. Mutation harness: trust-record-claim and return-stored-record both CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | exit 0; named PASS; mutation permit-missing-revision caught | host exit 0, `--- PASS: TestProductIntentSchemaParity (0.73s)`; deny-network sandbox exit 0, `--- PASS: TestProductIntentSchemaParity (0.75s)`. Mutation harness: permit-missing-revision, allow-case-alias and schema-missing-revision all CAUGHT (exit 1). | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Mutation harness: the shipped mutate.py under statusgen/productintent/testdata, run unmodified in a scratch clone at 4b15482af5c6 outside this tree: exit 0, 13/13 plants CAUGHT.

Risk-bearing values: the risk fields are all present and all `no`; the delivery code is unchanged since the earlier subsection, and every literal it enumerated was re-read at 4b15482af5c6 at the same file:line. The two derived verdicts stand:

RISK-VALUE: DERIVED — reqRE = `^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$` @ statusgen/productintent/decode.go:17 — identical to requirementIDRe @ statusgen/requirements.go:115 and the schema subject.requirement pattern, so the extension neither widens nor narrows the REQ id space.

RISK-VALUE: DERIVED — operational-reference kinds = "journey", "operational-workflow", "practice" @ statusgen/productintent/decode.go:219 — matches spec registers-v1 line 450, which names exactly these three and refuses workflow-pattern and workflow-instance.

**VERIFY: PASS** — 3/3 Verify rows pass in the container-netns witness (verifyrun rc 0, --check 3 pass) and by hand on the host and under deny-network, with 13/13 mutations caught.

Verification-Attestation: medici-finance/assay#3486 run=264eac3beb7d2d4fbd252a37f5a057efd8e3069c56118dc5ef2f6d4aead29039 source=4b15482af5c6c71b9312ca7df0f23b219de3f840 brief=docs/streams/statusgen/brief-22-product-target-records-and-compatible-requirement-parsing.md model=claude-opus-5-5 tier=strong

### Receipt-bearing container-netns witness — 2026-10-20, claude-opus-5-5 / assay-verifier-app[bot]

Subject: merged main e460818e309a9c585c3f4ddcbb2146ae6f860248. The delivery files from medici-finance/assay#3296 (statusgen/productintent, statusgen/requirements.go and its test, schemas/product-intent-v1.json, spec/registers-v1.md) are byte-identical to the prior subject 4b15482af5c6; only docs/contracts.md rows for other rules changed in the declared scope. Verifier is not the implementer. This is a fresh receipt-bearing pass at current main; the latest prior Evidence on main is the 2026-10-19 container-netns witness subsection above at 4b15482af5c6, landed by medici-finance/assay#3488. Runner: a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built from golang:1.27.2-trixie, git 2.47), started with `--network none`, GOPROXY=off, GOTOOLCHAIN=local and the engine's default seccomp profile; statusgen and the desk tools were built from this tree inside the container and `statusgen verifyrun --sandbox container-netns` ran for real, per the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun exited 0 and wrote the witness table below; `verifyrun --check` in the container reported 3 pass, 0 fail, 0 could-not-run of 3 Verify rows. Admission: `deskdispatch --check-verifier` exited 0 on the host before and after the container run, binding attestation medici-finance/assay#3654.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:4ad9581a91d6 | 2026-10-20 | assay-verifier-app[bot] @ e460818e309a (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:09bf6758cd27 | 2026-10-20 | assay-verifier-app[bot] @ e460818e309a (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:228d75f91292 | 2026-10-20 | assay-verifier-app[bot] @ e460818e309a (on-behalf-of human:ian) (forge-identity) |

Substance: the by-hand runs dated 2026-10-19 at source 4b15482af5c6 (each Verify command exactly as written, on the host and under deny-network, 3/3 named PASS, 13/13 mutations caught) are the by-hand Evidence this witness re-confirms mechanically at e460818e309a; the risk-value enumeration and its two DERIVED verdicts stand, every literal re-read at the same file:line at this source.

**VERIFY: PASS** — all 3 Verify rows pass with 0 fail in the container-netns witness at e460818e309a (verifyrun rc 0, --check 3 pass), on unchanged delivery code whose by-hand Evidence also passed 3/3.

Verification-Attestation: medici-finance/assay#3654 run=27d6f290fc7b6e7e45052e7352234e6ee4ce2f86230a5d77dabeb5d48c3a3801 source=e460818e309a9c585c3f4ddcbb2146ae6f860248 brief=docs/streams/statusgen/brief-22-product-target-records-and-compatible-requirement-parsing.md model=claude-opus-5-5 tier=strong

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
