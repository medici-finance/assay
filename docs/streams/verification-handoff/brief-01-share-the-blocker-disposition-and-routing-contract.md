---
brief: assay:assay:verification-handoff:01
title: "Share the blocker disposition and routing contract"
why: "The scanner and queue readers disagree about addressed issues, and a triage comment does not establish accountable repair work. One shared definition prevents callers assigning different meanings to the same blocker."
wave: 0
depends: []
unblocks: ["verification-handoff/02", "verification-handoff/03"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes the deskcore/issueflow boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
decision-issue: 4216
ruling: "https://github.com/medici-finance/assay/issues/4216#issuecomment-6373774704"
issues: []
schema: brief-v2
version: 1
id: 30004285-8520-4c89-aa26-c7a0be53d3a5
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "deskcore/issueflow/: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "tools/desk/internal/issueflow/: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "statusgen/go.mod: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "statusgen/go.sum: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "statusgen/scanissues.go: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "tools/desk/go.mod: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "spec/intake-exit-v1.md: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "spec/verification-handoff-v1.md: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "docs/contracts.md: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/01-mutations.json: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-01.md: follow-up verification-handoff/01 (this delivery; existing owner or planned extension)"
---

# Brief 01 — Share the blocker disposition and routing contract

## Context

files: deskcore/issueflow/ (planned), tools/desk/internal/issueflow/, statusgen/go.mod, statusgen/go.sum, statusgen/scanissues.go, tools/desk/go.mod, spec/intake-exit-v1.md, spec/verification-handoff-v1.md (planned), docs/contracts.md, statusgen/testdata/verification-handoff/01-mutations.json (planned), changelog/verification-handoff-01.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: deskcore/issueflow owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "deskcore/issueflow"
  contract: none — reuse the existing issue projection, intake-exit, repair-obligation and wake owners; update their canonical index on implementation.
  retires: ["implicit routing/assignment inference and the duplicate predicate or partial-success ambiguity named in Task"]
  weight: "New binaries 0; new services 0; bounded contract/library/adapter changes. Record actual rule, field, refusal and flag deltas at implementation review."
  why-add: "Reuse existing semantic owners; a shared meaning is needed across existing callers. A new process, queue or acceptance authority would add failure modes without closing the handoff."

single-point-of-failure: stale disposition or successor identity; independent trust/admission/claim and exact-source acceptance checks remain at effect boundaries.

## Read first

- [Controlling scope, coverage and owner decisions](README.md#implementation-scope).
- [Intake exit contract](../../../spec/intake-exit-v1.md).
- [Existing repair-obligation contract](../desk-supervision/repair-obligation-v1.md).
- [Wake and retry contract](../../verify-wake.md).
- [Library-first direction](../../library-first.md).
- [Semantic owners](../../contracts.md).

## Human decision

Confirm that the concrete implementation preserves independent verification and existing human-only decisions while changing how failed work is retained and assigned. A routing record must never grant acceptance or wider execution permissions.

Options:
1. Approve the bounded implementation after its independent review and verification.
2. Request corrections while retaining the existing holds.
3. Hold the proposed change.

Default if no answer: none — blocks until answered.

### Approval and evidence reconciliation — 2026-10-24

The driver chose option 1 for `verification-handoff/01`, from the driver's own account, on 2026-10-23: https://github.com/medici-finance/assay/issues/4216#issuecomment-6373774704

The delivered external-owner vocabulary was separately confirmed on 2026-10-24: https://github.com/medici-finance/assay/issues/4257#issuecomment-6378313940

Delivery is PR #4225. Independent verification is the appended PASS in PR #4255,
attested by #4254 against source `cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a`.
On 2026-10-24, `statusgen verifyrun --bracket-digest --witness <this-file>
--credit --source cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a` reported
`verdict=match credit=credited` for witness
`sha256:97d3da70383bbea33761d965763df798a7dc14fe020dde5d84bc4f2f8cd23406`.
This is a read-back of the independent witness, not another verification run.
The original blocked attempt and the later independent PASS remain unchanged below.

The remaining lifecycle gap is explicit: this brief has no approved `design:`
record. The existing approval names the brief, as its decision issue requested,
but does not name a `DR-` ID. Under registers-v1 §7.5 it cannot be represented as
approval of a subsequently authored design record. Recording the approval above
repairs the missing decision metadata; it does not claim that the design gate,
human final sign-off, or the dependent briefs' gates have passed.

The bounded design to adopt as `DR-handoff-shared` is the delivered
[`verification-handoff-v1`](../../../spec/verification-handoff-v1.md) contract:
`deskcore/issueflow` owns pure population selection and advisory handoff assessment;
callers retain trust, provider reads, custody, claims, effects and independent
acceptance. Legacy or incomplete observations remain unknown. An observed routing
record grants no readiness or acceptance. The five existing intake exits remain
unchanged. Alternatives rejected by this scope are per-caller policy copies
(drift), a helper process (an unnecessary process boundary), and using labels or
local logs as assignment authority (loss of source and actor binding).
The accepted cost is an additive versioned observation and source-module dependency;
operational writer activation, release and consumer qualification remain separate.
No design-approval stamp is asserted by this proposal.

## Ground rules

- Isolated feature branch and draft PR; no merge, source/stream/brief/PR-ready flip, release, installed pin change, workflow dispatch or live infrastructure effect in this brief.
- Recheck current main, claims, active PR overlap, controlling sources, semantic owners and module imports at pickup/resume; record observed revisions/coverage and continue, adapt or hold. A future API or draft spec is not shipped support.
- Preserve actor/authority separation, legacy identities and immutable Evidence/outcomes. Do not fabricate intake acknowledgement, assignment, historical execution or verified/done from labels or issue closure.
- New shared parsing/policy must be direct library calls, never a new helper executable. Preserve existing online read/custody boundaries.

## Task

1. Adopt the existing intake-exit, repair-obligation, claim and outcome contracts as inputs. Define a versioned extension for accountable handoff: source repo/brief and version/revision, receipt/case and held row identities, blocker ref, disposition and its deciding role, successor typed ref or external owner role/action, observation freshness and incomplete/unknown states. Reuse existing canonical case identity; do not equate a whole brief with one defect. Forge issue/PR records and existing dispatch claims remain authority; local intake exit logs and committed per-file projections are evidence/projections, never a competing lifecycle store.
2. Define the disposition semantics: needs-scoping/brief-authoring, directly implementable repair, external prerequisite, duplicate existing owner, and explicit unresolved/unknown. A mechanical placeholder or investigated comment alone is not an accepted disposition. Existing five intake exits remain distinct from repair scheduling state; extend or version their contract rather than silently repurpose bug/placeholder. Settle version negotiation for both writers/readers before dependent work emits required fields; old observations are unknown, not retroactively approved or discarded.
3. Move the existing pure candidate population selection into a public deskcore/issueflow library behind the existing internal compatibility facade. Extend that owner with pure handoff assessment over caller-supplied facts. Statusgen scanner, issueboard, intake and worker consumers must import and call the same functions in-process. Statusgen currently has no deskcore dependency: add the real module require/replace and prove GOWORK=off imports from both modules and a standalone consumer. Keep trust decisions, effect authorization, provider I/O, clocks and credentials in existing adapters; no helper executable, daemon or process RPC.
4. Specify a consistent addressed policy: ordinary unaddressed verifier bugs remain discoverable by intake; deliberately addressed issues stay in their receiving inbox and cannot leak into the ordinary scanner/work-ready lane. Preserve independently enforced trust, label-history, human and eligibility gates. Update the existing semantic owner index; retire duplicate population predicates only after parity fixtures prove old supported behavior.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffPolicyParity$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffPolicyParity ' "$handoff_out")` | exit 0 and named PASS; scanner/reader consumer fixtures agree on ordinary trusted repair, addressed role, absent disposition, human hold and unreadable facts; bypassing the shared addressed predicate fails. |
| 2 | check:ci +flow +mutation | sh | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffModuleImports$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffModuleImports ' "$handoff_out")` | exit 0 and named PASS; actual statusgen and desk-tools module consumer builds with GOWORK=off call the same public library; an internal import, duplicated predicate or hidden process/env dependency fails. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->
**VERIFY: BLOCKED** — both required rows could-not-run; base proof unproven.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffPolicyParity$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffPolicyParity ' "$handoff_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f3248 (on-behalf-of human:ian) (forge-identity) | base=254a8fb3d5b1 unproven · head=4a4b8e4f3248 |
| 2 | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffModuleImports$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffModuleImports ' "$handoff_out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f3248 (on-behalf-of human:ian) (forge-identity) | base=254a8fb3d5b1 unproven · head=4a4b8e4f3248 |
Verification-Attestation: medici-finance/assay#4231 run=ca6c0ea6b39dff629c3ce40ed424debeeaa6824ebfdc8d3136b3a8e373713acf source=4a4b8e4f324834ba75aa2f211c14655dda4d4bfd brief=docs/streams/verification-handoff/brief-01-share-the-blocker-disposition-and-routing-contract.md model=gpt-6-astra tier=strong


Independent runner: gpt-6-astra, strong, admitted by #4231. Observed source `4a4b8e4f324834ba75aa2f211c14655dda4d4bfd`; delivery #4225 merged as `9d9f603ef2d23e0eec09c9800a8603f46579b7a9`; fail-first base is its first parent `254a8fb3d5b13187f1fa35bfb3ed158c16fcd899`.

Required rows: 2; executed: 0; passed: 0; failed: 0; could-not-run: 2. No acceptance obligation was observed failing. No closed container witness or base-red/head-green proof exists.

The supported network-off container attempt used released statusgen v1.0.57 and `ghcr.io/medici-finance/verify-runner:v1.0.57@sha256:1b0f3a9a32a452b618fcf6761571af35f33239416482b7f748210f7b772566f3`. It refused before any row with exit 2: `could-not-attribute — no executing identity is available`. This reproduces existing blocker #4208. Host admission and bracket opening succeeded; the receipt did not carry the runner fields needed by the container. No identity was injected and no sandbox was disabled.

Disposition: waiting for release/installation/qualification of the existing runner repair, not another implementation of the handoff policy. Source repair #4214 is merged; source presence is not installed-runner qualification. The companion image Git-floor repair #4210 is also merged and belongs to the existing #4207 release work; the observed container refusal in this run is attribution, not a claimed Git-floor failure.

Next owner/action: the existing runner release and qualification owners carry #4208 through a containing published release, aligned consumer pins, actual installed host/image identity checks, and fresh independent qualification. Then the verify desk reruns both unchanged rows and the delivering-parent fail-first proof through a supported isolated execution path. Wake on that qualified installation, not merely on an issue closing or a source merge. The brief's existing human gate remains unchanged.

### Independent run 2026-10-24 (claude-opus-5-5, strong) — container-netns admission bracket

The run above closed at VERIFY: BLOCKED; this block is a separate run under a fresh admission and credits nothing from it.

Admission: medici-finance/assay#4254 (run 1e7f8131), source cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a, checked with deskdispatch --check-verifier before any row (exit 0) and re-checked by the close step. Installed host tools: statusgen v1.0.58; deskdispatch releaseTag=v1.0.58 sourceSHA=832f9c8f; git 2.56.0. Runner image ghcr.io/medici-finance/verify-runner:v1.0.58 pinned by digest sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204, image Id sha256:767f6a3545748b57a85f85dc80216e97203a39aa07e2ef137483d197dd60b175, linux/amd64 (run with --platform linux/amd64 and --network none; the home was mounted read-only; no gitdir mount, no identity injection, released in-image statusgen v1.0.58 only). Both rows are check:ci and ran in this one bracketed table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffPolicyParity$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffPolicyParity ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd deskcore && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffModuleImports$' ./issueflow/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffModuleImports ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ cb17e8f99a55 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:02c3cffe5a0ce6e37a9c7ddda5941024f04ab4eba05b05c2549c10a2d595f40c
Container-Image: sha256:be2d252957e0a5f152b42cd21860c13ed02c63d4bb98ad4234ca3e1d5e800204
Verification-Attestation: medici-finance/assay#4254 run=1e7f81311c7da592756c115e4415eb61538c53636d6df319906e1e8bcbea0d83 source=cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a brief=docs/streams/verification-handoff/brief-01-share-the-blocker-disposition-and-routing-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=1e7f81311c7da592756c115e4415eb61538c53636d6df319906e1e8bcbea0d83 source=cb17e8f99a550ce4b08a6d61030c3f9d0d1f310a tree=sha256:02c3cffe5a0ce6e37a9c7ddda5941024f04ab4eba05b05c2549c10a2d595f40c witness=sha256:97d3da70383bbea33761d965763df798a7dc14fe020dde5d84bc4f2f8cd23406 opened=2026-10-24T01:23:31Z closed=2026-10-24T01:24:05Z before=receipt after=check-verifier

Bracket: statusgen verifyrun --bracket-digest on the closed witness file printed bracket=L8 header=L1 verdict=match span-lines=tree-digest:1,container-image:1,attestation:1 (exit 0); opened 2026-10-24T01:23:31Z before closed 2026-10-24T01:24:05Z. Container-row credit is decided on the landed brief by --bracket-digest --credit, not by this block.

Row inventory: deskpathguard rederive reported 2 rows, 0 author-added, both from-base and identical to the current table; the delivery did not edit the Verify section. Required rows 2; executed 2; passed 2; failed 0; could-not-run 0.

Delivery and fail-first base: #4225 merged as 9d9f603ef2d23e0eec09c9800a8603f46579b7a9 on the first-parent chain of the verified source; its first parent and unique merge-base 254a8fb3d5b13187f1fa35bfb3ed158c16fcd899 has no deskcore/issueflow package (git ls-tree empty), so both named tests are absent at base and both row commands would fail there. Observed by tree inspection only; no base execution was performed.

Mutation obligations (supplemental, no row credit): the delivered manifest statusgen/testdata/verification-handoff/01-mutations.json was executed with its own runner (python3 -I statusgen/testdata/verification-handoff/mutate.py, all cases) on the darwin host, offline (GOPROXY=off, GOWORK=off), each case in a bounded temporary snapshot outside the home; the home was left unchanged (git status empty afterwards). Exit 0, log sha256:e9fc7d701f8a. Unmodified baselines: checked-clean for TestHandoffPolicyParity and TestHandoffModuleImports. All 18 mutations checked-failed as expected for the named reason, including: shared-addressed (selection.go Scanner predicate drops the addressed terms; fails with population parity addressed), scanner-second-site and consumer-route (statusgen scanissues.go bypasses the shared Select; fail with addressed scanner policy and consumer bypassed owner), source-binding, incomplete-facts, human-hold, activation-hold, freshness, successor, external-owner, version, row-shape, expected-rows, deciding-role, cli-projection, and planted second-clock, second-process and second-env effects in the pure package (fail via the archtest clock, os/exec and os allow-list checks). Row 2's internal-import clause has no dedicated mutation case; it rests on the test's public import-path route check and the Go toolchain's refusal of cross-module internal imports.

Envelope disclosures (uncredited): before opening the bracket, offline probes in the same image checked module availability (go list -deps for statusgen and tools/desk) and one go build of statusgen, then a separate container compiled the test packages with no tests selected to warm a dedicated Go build-cache volume, so the tests' own 90-second child-build limits are not consumed by cold compilation under amd64 emulation. These probes ran no Verify row and are not witnesses. A first probe that mounted the home at its host path saw an empty directory (container mount quirk) and ran nothing; every later mount used a fixed container path.

Risk-bearing values (enumerated over the delivery diff 254a8fb3..9d9f603e, deskcore/issueflow and the mutation manifest): HandoffSchema = "verification-handoff-v1" @ deskcore/issueflow/handoff.go:12; dispositions unresolved/needs-scoping/repair/external-prerequisite/duplicate @ deskcore/issueflow/handoff.go:17-21; deciding roles the-desk, worker-desk, pr-review-desk, verify-desk, intake-desk, driver @ deskcore/issueflow/handoff.go:90; external roles human, operator, brief-author, verifier @ deskcore/issueflow/handoff.go:142; held row floor row < 1 @ deskcore/issueflow/handoff.go:112; AddressPrefix = "to:" @ deskcore/issueflow/selection.go:7; InboxRank order urgent, needs-decision, question, help wanted and sentinel 99 @ deskcore/issueflow/selection.go:54-57; ref patterns @ deskcore/issueflow/handoff.go:83-85; operational limits (child go 90s/45s/30s in imports_test.go, mutation snapshot 16000 files / 96 MiB and 270s in mutate.py) rank last and need no derivation. All are reversible by edit and redeploy; the least reversible is the persisted schema and vocabulary strings.

- RISK-VALUE: DERIVED — HandoffSchema = "verification-handoff-v1" @ deskcore/issueflow/handoff.go:12 — the brief's Task 2 requires exact version negotiation with old observations unknown; SupportsHandoff is exact equality (handoff.go:69) and the legacy and future cases assess unknown.
- RISK-VALUE: DERIVED — Disposition set @ deskcore/issueflow/handoff.go:17-21 — matches Task 2's five dispositions one-to-one (needs-scoping/brief-authoring, directly implementable repair, external prerequisite, duplicate existing owner, explicit unresolved/unknown).
- RISK-VALUE: DERIVED — knownRole set @ deskcore/issueflow/handoff.go:90 — equals the six-role closed set in spec/intake-exit-v1.md (adopted per Task 1); the parity test reads that table and fails if its count or membership diverges.
- RISK-VALUE: NAMED, NOT DERIVED — external owner roles human, operator, brief-author, verifier @ deskcore/issueflow/handoff.go:142 — the only source fixing this set is the spec authored in the same delivery (spec/verification-handoff-v1.md line 35); the brief names an external owner role without enumerating it, so no independent derivation exists. Routed to the human gate. Question filed: #4257; ruled A by the ratifying human on 2026-10-24, confirming human, operator, brief-author, verifier as the intended closed set: https://github.com/medici-finance/assay/issues/4257#issuecomment-6378313940
- RISK-VALUE: DERIVED — AddressPrefix = "to:" @ deskcore/issueflow/selection.go:7 — unchanged from the pre-existing deskkit AddressedToPrefix literal at the base and the forge's to:<role> label convention.

Gate: the brief is gate: human with risk sensitive-data: yes. This run records Evidence only; a model cannot sign it off, and no lifecycle cell, board row or PR state was changed. Next actor: the desk lands this block and runs the credit check on the landed brief; the human gate decides the brief's Human decision.

**VERIFY: PASS** — 2/2 rows pass exit=0 sandbox=container-netns in one closed admission bracket (verdict=match); 18/18 supplemental mutations fail as named; human gate and post-landing credit check remain.

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
