---
brief: assay:assay:desktools-v2:63
title: "Define perf-v3 attribution, profiles and compatible local readers"
why: "Operators cannot associate current local measurements with a brief, issue or agent. One versioned local contract must add that attribution and forge-cost vocabulary without widening the old records or silently collecting identifiable workflow data."
wave: 2
depends: ["desktools-v2/61"]
unblocks: ["desktools-v2/64", "desktools-v2/65", "desktools-v2/62", "statusgen/26", "performance-collection/02", "desktools-v2/67"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Workflow attribution exposes local work linkage; review the implemented field, consent and propagation boundary before acceptance."
decision-trigger: spec
decision-issue: 3536
ruling: "https://github.com/medici-finance/assay/issues/3536#issuecomment-6282045526"
design: DR-perf-v3-local
issues: []
schema: brief-v3
version: 3
id: 3c6c55aa-ddde-467a-8567-763d7072541f
authored: 2026-10-19 by performance attribution authoring
sources:
  - docs/performance-attribution-spec.md
  - docs/streams/desktools-v2/spec.md
  - docs/telemetry.md
  - docs/library-first.md
  - "freshness-checked 2026-10-19 @ 2e02111ed; perf-v2 implementation 2013b8dc6, no perf-v3 or perfdata module"
exec-tier: strong
exec-tier-why: "Cross-process identity, compatibility and negative-path coverage require design judgment across shared consumers."
domain: complicated
outcome: none
consumers:
  - "perfrec/: fixed-here"
  - "tools/desk/internal/deskkit/perf.go and perfread.go: fixed-here"
  - "tools/desk/cmd/deskperf/: fixed-here"
  - "tools/desk/internal/cli/ and launchers: follow-up desktools-v2/64"
  - "tools/desk/internal/deskkit/forgetransport.go: follow-up desktools-v2/65"
  - "tools/desk/cmd/deskboard and deskdispatch: follow-up desktools-v2/62"
  - "statusgen/: follow-up statusgen/26"
  - "perfdata/: follow-up performance-collection/01"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "docs/telemetry.md: fixed-here"
---

# Brief 63 — Define perf-v3 attribution, profiles and compatible local readers

## Context

files:
- `perfrec/schema.go`
- `perfrec/session.go`
- `perfrec/perfrec.go`
- `perfrec/context.go (planned)`
- `perfrec/context_test.go (planned)`
- `perfrec/testdata/`
- `perfrec/README.md`
- `tools/desk/internal/deskkit/perf.go`
- `tools/desk/internal/deskkit/perfread.go`
- `tools/desk/cmd/deskperf/`
- `docs/telemetry.md`
- `docs/performance-attribution-spec.md`
- `changelog/perf-v3-contract.md (planned)`

facts (source and date in frontmatter):
- perf-v2 is implemented by 2013b8dc6; its closed schema excludes all work/session IDs. Its Session is a timer, not agent identity. Preserve the completed /61 and its Evidence.
- The existing stdlib-only perfrec module is importable from standalone modules via relative require/replace; deskkit is internal. Do not move credential, forge or database dependencies into the recorder.
- perfdata remains absent. Its future spool/alias/transport contract is separate; it must not block local v3 or be duplicated here.

layering: perfrec owns typed optional observations; existing CLI, launcher and provider adapters retain effects and authority. No new service or credential boundary.

single-point-of-failure: a producer can mislabel optional context or coverage; independent writer/profile validation and production-flow negative controls expose this without granting operational authority.

design-fit:
  owner: perfrec and existing producer adapters
  contract: docs/performance-attribution-spec.md
  retires: [unattributed new performance samples and implicit unknown-as-zero accounting]
  weight: "verbs +0; profiles +2; shared flags +8 across the track, owned by /64; no new sender, service or credential holder"
  why-add: "Reuse the local recorder and effect seams; three bounded M briefs separate schema, propagation and provider observation."

## Read first

Read docs/performance-attribution-spec.md, the Context sources and the implemented /61.
Refresh current source and active work before pickup; all new paths and named tests below are
deliverables, not existing implementation or acceptance evidence.

## Human decision

Review the implemented identifiable local workflow profile and its demonstrated field and
propagation boundaries. Confirm that opt-out and tool-only use cannot retain work or agent
linkage, and that invalid metadata cannot affect the underlying operation.

Options:
1. **Accept the bounded implementation** — accept the demonstrated local disclosure and isolation; retain separate remote activation controls.
2. **Hold for correction** — identify the field, lifecycle or failure boundary that needs correction.

Default if no answer: none — blocks until answered

Acceptance waits for concrete implementation evidence.

Decision recorded 2026-10-19: the mapped human selected option 1, accepting the bounded implementation. The design record transcribes that ruling; independent review and verification remain required.
The ruling comment is on decision issue #3536.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure.
Synthetic providers, isolated homes and disposable local processes only. Preserve custody,
claims, budgets, audit, help/version and operation results. Optional collection cannot become
required accounting. Stop at implemented; independent verification owns Evidence.

## Interface contract

Freeze perf-v3 per docs/performance-attribution-spec.md: explicit tool/workflow profile, bounded typed context, session/agent/parent and fresh invocation identity, qualified local brief/issue references, deskRole versus assigned role, and request-duration/GraphQL-cost units and coverage. Existing opt-in with no profile retains existing schemas. Only workflow permits identity; tool-only cannot carry it. Flag > named environment > unavailable; present empty clears. Bad optional context cannot fail the operation. perfrec owns typed resolution; CLI adapters only bind values.

## Task

1. Freeze the wire schema, bounds, vocabulary, work JSON-object flag encoding and env JSON-array encoding, coverage and synthetic examples. Keep raw qualified work references local under workflow consent, restrictive permissions and seven-day retention. Unknown/invalid input is never copied; writer/decoder independently reject disallowed fields even with parser bypassed. Document the aliased perfdata export boundary as future work, without building it.
2. Implement credential-free typed context resolution, random per-invocation identity and the selected-profile measurement API. Session/agent IDs are caller-supplied validated random IDs; absence remains unavailable. Do not search branches, files, transcripts or Git for attribution. Freeze immutable per-invocation context for concurrency. Without an explicit profile preserve legacy producer behavior.
3. Extend the same writer and mixed-version reader/report. Keep v1/v2 output/meanings and unsupported data intact; v3 grouping/filtering by work/session/agent/deskRole/role requires workflow permission. Deduplicate v3 by source/invocation and expose conflicting duplicates; local read filters do not resolve private references through provider calls. Request distributions retain sample/coverage counts; measured, estimated and unknown points never merge.
4. Supply reusable API fixtures for /64, /65, /62, statusgen/26 and the collection importer. Prove old fixtures remain readable, tool profile strips/refuses workflow fields, and bad context/full disk preserves original results. Match on/off timing, concurrency, bytes and drops; record p50/p95/max overhead rather than claiming zero overhead. Update the existing docs/help/API references; no docs-site generator is required for these Markdown/API pages.

## Verify

Named tests below are planned. Each command requires its exact PASS line; a missing test
cannot pass. Plant the stated defect through the production boundary, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | exit 0; named PASS. Direct library/writer bypass rejects workflow identifiers in tool profile; invalid/empty/overridden values and qualified same-number issues differ; no-profile v1/v2 fixtures remain unchanged. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | exit 0; named PASS. Real writer to mixed-version deskperf report; duplicates/conflicts, missing IDs, measured-zero/missing points and inclusive times retain correct grouping and coverage. |
| 3 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | exit 0; named PASS. External module builds without desk/network/credential/DB imports; off creates nothing, bounded failed writes preserve results and matched on/off overhead is reported. |
| 4 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/63 --delivery 3937f39ae2d00d150633f2c22568a79d1f5efb5a --base b787e831d0be847f1ca81cc48496c8c0deb01fe5` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass. |

## Pre-mortem and detection

Rows 1–3 plant the counterexamples named in Expect: identity/consent or request-count bugs,
false-zero/compatibility bugs, and changed operational behavior. Row 4 checks shared consumer
routing. Sufficiency of real-world coverage and local disclosure remains reviewer judgment;
fixture success never asserts live adoption or permission.

## Query/cache measurement amendment — 19 October 2026

Extend the schema/reducer fixtures in docs/performance-attribution-spec.md's query/cache
section. Distinguish client logical calls/transport attempts, handler observations and actual
provider attempts; include bounded direct/passthrough/refresh/poll/repair origin, cache source
and coalescing coverage. Optional diagnostic links require the workflow profile and an
explicitly admitted service-protocol option, never automatic raw work/identity forwarding.
Use explicit observation kinds to avoid counting request detail plus a process summary.
Handler-scoped CPU cannot be inferred from concurrent whole-process CPU. Extend
TestPerfV3RecordToReportFlow with warm hit, 50-to-one shared three-page refresh, passthrough
retry, background poll and missing server data; planted duplicate points, false zero and
client-plus-server double logical query counts must fail. This is synthetic contract/reducer
work; it adds no dependency on an unshipped service.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit and named result. -->

### Independent verification — 2026-10-20 at 98763cb13757 (container witness plus by-hand runs)

Verification-Attestation: medici-finance/assay#3770 run=56dc2c8dfe0c68b129e2c07c891a495b9ee71ad0b07defa118c7ccfe6f528a10 source=98763cb13757e9ad2fa9214501102788d2e60404 brief=docs/streams/desktools-v2/brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md model=claude-opus-5-5 tier=strong

Subject revision: merged main 98763cb13757e9ad2fa9214501102788d2e60404, detached and clean. Delivery: the two-parent merge 3937f39ae2d00d150633f2c22568a79d1f5efb5a, on that commit's first-parent chain. Its first parent is b787e831d0be847f1ca81cc48496c8c0deb01fe5 and the parents' unique merge-base is ff225b351d7d69de90567d37940077e0a1a16639. The dispatcher check (deskdispatch --check-verifier) gave exit 0 before any row ran and exit 0 again before landing, with the same binding both times (above). The verifier is not the implementer.

Protected table: deskpathguard rederive found 4 original rows, 0 author-added rows. The parsed Verify table is byte-identical at the authoring commit 8cf52223, at the merge-base, at the first parent, at the delivery and at the subject revision. Every original row ID, Command, Expect and class is kept below.

Verification-Table: delivery=3937f39ae2d00d150633f2c22568a79d1f5efb5a base=ff225b351d7d69de90567d37940077e0a1a16639 original=ff225b351d7d69de90567d37940077e0a1a16639

Runner class: a local docker container under the driver's 2026-10-19 interim-runner ruling (recorded on the private house tracker). The image is assay-verify:go1.27, id sha256:955fd6c46c93ee746d8f560b4c90b172a759d2543337d8173cdaabcd2013bca4 (Debian trixie, git 2.47.3, go1.27.2 linux/arm64). It ran with --network none, GOPROXY=off, the engine's default seccomp profile and statusgen verifyrun --sandbox container-netns, KUBECONFIG=/dev/null, and no live infrastructure. Each row was also run by hand at the subject revision on the host (git 2.56.0, go1.27.2 darwin/arm64, statusgen v1.0.52): once plain, and once more for rows 1 to 3 under a macOS sandbox profile that denies outbound network (a probe connection from inside it failed with curl exit 7), with GOPROXY=off.

#### Container witness (statusgen verifyrun, sandbox=container-netns)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | pass exit=0 sandbox=container-netns | sha256:94ae9dfaafb6 | 2026-10-20 | assay-verifier-app[bot] @ 98763cb13757 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:d9df2704f70e | 2026-10-20 | assay-verifier-app[bot] @ 98763cb13757 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | pass exit=0 sandbox=container-netns | sha256:63c04c51e730 | 2026-10-20 | assay-verifier-app[bot] @ 98763cb13757 (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/63` | fail exit=2 sandbox=container-netns | sha256:d70337368bf8 | 2026-10-20 | assay-verifier-app[bot] @ 98763cb13757 (on-behalf-of human:ian) (forge-identity) |

verifyrun --check on the container clone: 3 pass, 1 fail, 0 could-not-run (of 4 Verify rows); 0 author-added. Exit 2 is statusgen's COULD-NOT-CHECK exit for the consumers gate, not a disproved routing claim. The by-hand row 4 below shows the same exit with its full reason.

#### By-hand runs at 98763cb13757 (host)

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3ContextAndConsent (0.00s)` (plain host); exit 0, same line, under the deny-network sandbox (output sha256 94ae9dfaafb6, equal to the container witness hash) | 2026-10-20 independent verifier (host, darwin/arm64) |
| 2 | Verify row 2 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3RecordToReportFlow (0.07s)` (plain host); exit 0; `--- PASS: TestPerfV3RecordToReportFlow (0.09s)` under the deny-network sandbox | 2026-10-20 independent verifier (host, darwin/arm64) |
| 3 | Verify row 3 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3IsolationAndOverhead (1.11s)` (plain host); exit 0; `--- PASS: TestPerfV3IsolationAndOverhead (0.48s)` under the deny-network sandbox | 2026-10-20 independent verifier (host, darwin/arm64) |
| 4 | Verify row 4 command, verbatim | exit 0; consumer routing corroborates the actual diff | exit 2; `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desktools-v2:63 is not in the diff against 98763cb13757e9ad2fa9214501102788d2e60404, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved.` | 2026-10-20 independent verifier (host, darwin/arm64) |

Row 3 overhead, from a verbose supplemental run of the same test: n=100, concurrency=1, 804 bytes per record, 0 drops; off p50/p95/max = 500/625/750 ns; on p50/p95/max = 303250/673625/899000 ns; difference p50/p95/max = 302750/673000/898250 ns; at concurrency=4, 200 records, 158400 bytes, 0 drops. The test reports overhead; it does not claim zero.

#### Row 4 supplemental probes (diagnostic only, no acceptance credit)

- With the delivery passed by hand (statusgen --root . --consumers --brief desktools-v2/63 --delivery 3937f39ae2d0 --base b787e831d0be, full SHAs): exit 0; `summary: 10 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. Fixed-here entries corroborated: perfrec/ (13 changed paths), deskkit perf.go and perfread.go, cmd/deskperf/ (3 paths), docs/telemetry.md. Follow-up entries corroborated: desktools-v2/64, /65 and /62, statusgen/26, performance-collection/01 and /02. All six follow-up brief files exist.
- In a scratch clone (never pushed), the Verification-Table marker above was committed into this brief's Evidence section, and the exact row 4 command was run: exit 0, with the same 10 corroborated / 0 disproved / 0 unchecked output over b787e831..3937f39a. Both outputs hash to sha256 427cf0644134.

#### Mutation and fail-first proofs (rows 1 to 3)

Each defect below was planted in production code in a scratch clone at 98763cb13757 and run through the exact row 1, 2 and 3 commands, then restored. All three rows exit 0 on the unmutated tree, before and after each batch.

| Plant (production file) | Row 1 | Row 2 | Row 3 | Result |
|---|---|---|---|---|
| CONTROL: the tool-profile writer bypass admits linked identity (context.go) | 1 | 0 | 0 | killed |
| The direct v3 writer bypasses the exact telemetry opt-in (perfrec.go) | 1 | 0 | 0 | killed |
| V-E1: the reducer hides a conflicting duplicate (reduce.go) | 0 | 1 | 0 | killed |
| V-E2: the reducer stops deduplicating by source/invocation (reduce.go) | 0 | 1 | 0 | killed |
| V-E3: workflow grouping without workflow permission (reduce.go) | 0 | 1 | 0 | killed |
| V-E4: a present (empty) flag no longer overrides the environment (context.go) | 1 | 1 | 0 | killed |
| V-E5: an invalid identity value is copied as measured (context.go) | 1 | 0 | 1 | killed |
| V-E6: the telemetry opt-in is always on (perfrec.go) | 1 | 0 | 1 | killed |
| V-E7: a failed optional sink aborts the operation (perfrec.go) | 0 | 0 | 1 | killed |
| V-E8: qualified same-number issues conflated, repo dropped from the dedupe key (context.go) | 1 | 0 | 0 | killed |
| V-E9: no explicit profile defaults to tool, changing the legacy producer (context.go) | 1 | 0 | 0 | killed |
| V-E10: a network import enters the external module (context.go) | 0 | 0 | 1 | killed |
| V-E11: missing measured cost accepted as measured, a false zero (v3.go) | 0 | 1 | 0 | killed |
| V-E12: missing invocation identity accepted by the decoder/writer (context.go) | 0 | 1 | 0 | killed |

Every counterexample that an Expect cell names was killed by its own row. The implementer's mutation manifest also lists seven plants that all three acceptance commands miss: a later request seam erases earlier drops; the writer accepts measured coverage after drops; a duration maximum exceeds its total; the reducer merges requests across observation kinds; the reader accepts an invalid work filter; JSON null work collapses to the empty case; the direct writer accepts empty measured work. The implementer's wider command (perfrec, -run 'TestPerfV3ContextAndConsent|TestV3DroppedCoverage|TestV3ReviewBoundaries') kills all seven (exit 1 each). Those guards are tested, but outside the acceptance rows. Under the brief's pre-mortem this is left to reviewer judgment, as a sufficiency note on the row 1 and row 2 selectors.

#### Risk-bearing values (trigger: risk sensitive-data yes)

Enumerated over the delivery diff b787e831..3937f39a (perfrec, deskkit perf/perfread, deskperf) plus the values the brief's Task names:

- ProfileTool = "tool" @ perfrec/context.go:18
- ProfileWorkflow = "workflow" @ perfrec/context.go:19
- the tool-profile writer identity check: all six linked fields empty and len(d.Work) == 0 @ perfrec/context.go:313
- the opt-in check: enabled != "1" @ perfrec/context.go:207
- Enabled: == "1" @ perfrec/perfrec.go:127
- EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47
- NewID: [16]byte from crypto/rand @ perfrec/context.go:114
- RetentionDays = 7 @ perfrec/perfrec.go:50
- directory mode 0o700 @ perfrec/perfrec.go:351 and :358
- file mode 0o600 @ perfrec/perfrec.go:367, :387, :273 and :282
- MaxWork = 8 @ perfrec/context.go:76
- MaxWorkBytes = 4096 @ perfrec/context.go:77
- MaxRequestGroups = 64 @ perfrec/v3.go:80
- issue upper bound 2147483647 @ perfrec/context.go:152
- InstallIDPattern 32 lowercase hex @ perfrec/perfrec.go:256

Ranked by what breaks. First come the values that decide whether linked identity or raw qualified work references reach disk or another reader: the tool/workflow split, the opt-in, the file modes and the retention. A disclosure cannot be undone by an edit and a redeploy. After those comes the ID entropy. The size bounds and patterns rank last: they are reversible operational knobs, and a wrong value means rejected or truncated optional context, never a disclosure.

RISK-VALUE: DERIVED — ProfileTool = "tool" @ perfrec/context.go:18, with its writer check @ perfrec/context.go:313 — the spec freezes "Only the workflow profile carries the linked context" (docs/performance-attribution-spec.md line 59). The check makes the writer refuse any session, agent, parent-agent, parent-invocation, deskRole, role or work value under the tool profile, even when the parser is bypassed. That is what Task 1 requires ("writer/decoder independently reject disallowed fields even with parser bypassed"). The CONTROL plant shows that row 1 detects a bypass.

RISK-VALUE: DERIVED — Enabled == "1" @ perfrec/perfrec.go:127 (and enabled != "1" @ perfrec/context.go:207, EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47) — the spec says "New writing needs ASSAY_TELEMETRY=1" and that existing ASSAY_TELEMETRY=1 with no profile keeps the existing producer (lines 33 and 35). An exact match on "1" means no other value turns collection on. Plants V-E6 and the direct-writer opt-in bypass are both killed.

RISK-VALUE: DERIVED — directory mode 0o700 @ perfrec/perfrec.go:351/:358 and file mode 0o600 @ perfrec/perfrec.go:367/:387 — the spec requires "existing restrictive file permissions" (line 47) and names 0700 directories / 0600 files for the private local mapping (line 112). Owner-only access is the least privilege that still lets the owning user's own reader work, and this delivery adds chmod calls that tighten pre-existing paths to those modes.

RISK-VALUE: DERIVED — RetentionDays = 7 @ perfrec/perfrec.go:50 — the spec keeps "seven-UTC-day history" (line 32) and "retained no more than seven UTC days" (line 113). The value is unchanged by this diff and is named by Task 1.

RISK-VALUE: DERIVED — NewID [16]byte from crypto/rand @ perfrec/context.go:114 — the spec requires random IDs that are not user names, PIDs, paths or timestamps (lines 46 and 69), but does not fix a width. 128 random bits put the birthday bound for a collision near 2^64 IDs, far above any local seven-day history, and the IDs cannot be guessed or linked to a person. This is a first-principles derivation, because the spec names no number.

RISK-VALUE: NAMED, NOT DERIVED — MaxWork = 8 @ perfrec/context.go:76, MaxWorkBytes = 4096 @ perfrec/context.go:77, MaxRequestGroups = 64 @ perfrec/v3.go:80, issue bound 2147483647 @ perfrec/context.go:152 — these are reversible size bounds on optional context. The spec asks for "bounded" values without numbers, so no source fixes the exact values. Exceeding a bound marks the context invalid and never fails the operation (plant V-E7 is killed), so they rank last and need no derivation.

**VERIFY: BLOCKED** — rows 1, 2 and 3 pass in the container witness and by hand, and every planted counterexample their Expect cells name is killed. Row 4 returns COULD-NOT-CHECK (exit 2) at 98763cb13757 because the committed Evidence had no Verification-Table marker naming the delivery; no routing claim was disproved. The marker is recorded in this run. Re-run the bare row 4 command once this Evidence has landed: the scratch-clone simulation with the marker gives exit 0, 10 corroborated, 0 disproved. Gate: human; status left unchanged.

### Row 4 at the Evidence landing commit — 2026-10-20 at d34483321afd

Verification-Attestation: medici-finance/assay#3770 run=56dc2c8dfe0c68b129e2c07c891a495b9ee71ad0b07defa118c7ccfe6f528a10 source=98763cb13757e9ad2fa9214501102788d2e60404 brief=docs/streams/desktools-v2/brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md model=claude-opus-5-5 tier=strong

Subject revision: d34483321afd8079913f9cdadbfe9126db31aec7. Its single parent is the attested source 98763cb13757, and it changes only this brief's Evidence section: the run above, together with its Verification-Table marker. Bare Verify row 4 was re-run there through the same container witness as before. That witness run re-executes the whole Verify table, so all four rows are recorded below; rows 1 to 3 come from the same run, because the protected-table check reads every row's witness from the current run. Image assay-verify:go1.27, id sha256:955fd6c46c93ee746d8f560b4c90b172a759d2543337d8173cdaabcd2013bca4, --network none, GOPROXY=off, default seccomp, statusgen verifyrun --sandbox container-netns, KUBECONFIG=/dev/null. The witness clone pins origin/main to the subject revision. That is how the brief reads once this commit is on main.

Verification-Table: delivery=3937f39ae2d00d150633f2c22568a79d1f5efb5a base=ff225b351d7d69de90567d37940077e0a1a16639 original=ff225b351d7d69de90567d37940077e0a1a16639

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | pass exit=0 sandbox=container-netns | sha256:94ae9dfaafb6 | 2026-10-20 | assay-verifier-app[bot] @ d34483321afd (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:424d14182701 | 2026-10-20 | assay-verifier-app[bot] @ d34483321afd (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | pass exit=0 sandbox=container-netns | sha256:9f725f0a243c | 2026-10-20 | assay-verifier-app[bot] @ d34483321afd (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/63` | pass exit=0 sandbox=container-netns | sha256:5bbac2405598 | 2026-10-20 | assay-verifier-app[bot] @ d34483321afd (on-behalf-of human:ian) (forge-identity) |

Output line: `summary: 10 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`, over `base b787e831d0be847f1ca81cc48496c8c0deb01fe5..3937f39ae2d00d150633f2c22568a79d1f5efb5a (immutable delivered snapshot)`. Corroborated entries:

- fixed-here: perfrec/ (13 changed paths), deskkit perf.go and perfread.go, cmd/deskperf/ (3 changed paths), docs/telemetry.md
- follow-up: desktools-v2/64, /65 and /62, statusgen/26, performance-collection/01 and /02

By hand on the host (statusgen v1.0.52), in a scratch clone at d34483321afd with origin/main pinned the same way, the same command exited 0 with the identical summary line.

The same command was also run by hand in a scratch worktree at d34483321afd, where origin/main still points at a later main that does not contain this commit. There the exit is 0, but the output reads `summary: 0 corroborated, 0 disproved, 10 unchecked`. The reason: the branch's own diff from 98763cb13757 contains this brief, so the gate judges the branch's diff rather than the marked delivery. UNCHECKED is not a pass, so that run is no acceptance evidence. It is recorded so that a branch-context re-run is not mistaken for corroboration.

Rows 1, 2 and 3 pass again in this witness run at d34483321afd, exit 0 each. Their substance stands on the run above: they pass in the container witness and by hand at 98763cb13757, with every planted counterexample their Expect cells name killed. d34483321afd changes no code, only this brief.

**VERIFY: PASS** — 4/4 Verify rows pass in the container witness at d34483321afd (rows 1 to 3 also pass by hand at 98763cb13757, above). Row 4 exits 0 with 10 corroborated, 0 disproved, 0 unchecked. Gate: human; status left for the human gate to advance.

### Second-stamp re-verify — 2026-10-20 at 671d33a050bc (container witness plus by-hand runs)

#### Container witness (statusgen verifyrun, sandbox=container-netns)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | pass exit=0 sandbox=container-netns | sha256:94ae9dfaafb6 | 2026-10-20 | assay-verifier-app[bot] @ 671d33a050bc (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:d9df2704f70e | 2026-10-20 | assay-verifier-app[bot] @ 671d33a050bc (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | pass exit=0 sandbox=container-netns | sha256:63c04c51e730 | 2026-10-20 | assay-verifier-app[bot] @ 671d33a050bc (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/63` | pass exit=0 sandbox=container-netns | sha256:5bbac2405598 | 2026-10-20 | assay-verifier-app[bot] @ 671d33a050bc (on-behalf-of human:ian) (forge-identity) |

Verification-Attestation: medici-finance/assay#3786 run=ddec25df7dc4ad526c4e40349d72368292f8f6cac1da480b2ec75bae0d553688 source=671d33a050bcc8516d2ba3776d2754a6b330beb3 brief=docs/streams/desktools-v2/brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md model=claude-opus-5-5 tier=strong

Subject revision: merged main 671d33a050bcc8516d2ba3776d2754a6b330beb3, detached and clean, with origin/main at the same commit. It contains the delivery 3937f39ae2d00d150633f2c22568a79d1f5efb5a and the earlier Evidence above (d34483321afd and its follow-up). This is the second stamp, from a different session than the first-stamp verifier; it is not the implementer. The dispatcher check (deskdispatch --check-verifier) gave exit 0 before any row ran, with the binding above. Runner class: a local docker container under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). The image is assay-verify:go1.27, id sha256:955fd6c46c93 (built FROM golang:1.27.2-trixie, git 2.47). It ran with --network none, GOPROXY=off, GOTOOLCHAIN=local, the engine's default seccomp profile and statusgen verifyrun --sandbox container-netns, KUBECONFIG=/dev/null, and no live infrastructure. Each row was also run by hand at the subject revision on the host (git 2.56.0, go1.27.2 darwin/arm64, statusgen v1.0.52).

Protected table: deskpathguard rederive found 4 original rows and 0 author-added rows at this source. The parsed Verify section is byte-identical at the merge-base ff225b351d7d, at the delivery 3937f39ae2d0 and at the subject revision. The marker below is the one already on main in the Evidence above, carried into this run.

Verification-Table: delivery=3937f39ae2d00d150633f2c22568a79d1f5efb5a base=ff225b351d7d69de90567d37940077e0a1a16639 original=ff225b351d7d69de90567d37940077e0a1a16639

Prior Evidence: a PASS verdict dated 2026-10-20 at d34483321afd, which followed the run at source 98763cb13757 above. Since 98763cb13757 the deliverable code is byte-identical: git diff over perfrec/, the deskkit perf.go and perfread.go files and cmd/deskperf/ is empty. Two related files did change. docs/telemetry.md gained the statusgen/26 perf-v3 section. In docs/performance-attribution-spec.md one flag-table line was renamed, --perf-desk-role to --perf-loop-role. That flag is bound by the CLI adapter that desktools-v2/64 owns, and the spec line numbers cited below are unchanged.

#### By-hand runs at 671d33a050bc (host)

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3ContextAndConsent (0.00s)` | 2026-10-20 independent verifier, second stamp (host, darwin/arm64) |
| 2 | Verify row 2 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3RecordToReportFlow (0.05s)` | 2026-10-20 independent verifier, second stamp (host, darwin/arm64) |
| 3 | Verify row 3 command, verbatim | exit 0; named PASS | exit 0; `--- PASS: TestPerfV3IsolationAndOverhead (0.52s)` | 2026-10-20 independent verifier, second stamp (host, darwin/arm64) |
| 4 | Verify row 4 command, bare and verbatim | exit 0; consumer routing corroborates the actual diff | exit 0; `summary: 10 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`, over base b787e831d0be847f1ca81cc48496c8c0deb01fe5..3937f39ae2d00d150633f2c22568a79d1f5efb5a (immutable delivered snapshot) | 2026-10-20 independent verifier, second stamp (host, darwin/arm64) |

Row 4 now gets its delivery from the Verification-Table marker on main. No delivery or base was passed by hand. The corroborated entries match the first stamp. Fixed-here: perfrec/ (13 changed paths), deskkit perf.go and perfread.go, cmd/deskperf/ (3 changed paths), docs/telemetry.md. Follow-up: desktools-v2/64, /65 and /62, statusgen/26, performance-collection/01 and /02. The witness row 4 output hash (sha256:5bbac2405598) equals the hash of the row 4 witness at d34483321afd.

#### Fail-first spot check (rows 1 to 3)

The deliverable code is unchanged since the first stamp's full mutation table. To check that the rows still discriminate at this source, three of its plants were applied again. Each went into production code in a scratch clone at 671d33a050bc, ran through the exact row 1, 2 and 3 commands, and was then restored. All three rows exit 0 on the unmutated tree, before and after.

| Plant (production file) | Row 1 | Row 2 | Row 3 | Result |
|---|---|---|---|---|
| CONTROL: the tool-profile writer identity check always admits (context.go line 313) | 1 | 0 | 0 | killed |
| V-E6: the telemetry opt-in is always on (perfrec.go line 127) | 1 | 0 | 1 | killed |
| V-E3: workflow grouping without workflow permission (reduce.go line 72) | 0 | 1 | 0 | killed |

#### Risk-bearing values (trigger: risk sensitive-data yes)

The enumeration is the first stamp's, over the delivery diff b787e831..3937f39a. Every literal was re-read at this source at the same file and line: ProfileTool = "tool" @ perfrec/context.go:18; ProfileWorkflow = "workflow" @ perfrec/context.go:19; MaxWork = 8 @ perfrec/context.go:76; MaxWorkBytes = 4096 @ perfrec/context.go:77; NewID [16]byte @ perfrec/context.go:114; issue bound 2147483647 @ perfrec/context.go:152; opt-in enabled != "1" @ perfrec/context.go:207; tool-profile identity check @ perfrec/context.go:313; EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47; RetentionDays = 7 @ perfrec/perfrec.go:50; Enabled == "1" @ perfrec/perfrec.go:127; InstallIDPattern 32 lowercase hex @ perfrec/perfrec.go:256; file mode 0o600 @ perfrec/perfrec.go:273, :282, :367 and :387; directory mode 0o700 @ perfrec/perfrec.go:351 and :358; MaxRequestGroups = 64 @ perfrec/v3.go:80. None changed, so the ranking and derivations above still hold.

RISK-VALUE: DERIVED — ProfileTool = "tool" @ perfrec/context.go:18, with its writer check @ perfrec/context.go:313 — only the workflow profile may carry linked context (spec line 59), and the writer refuses it under the tool profile even with the parser bypassed; the CONTROL plant is killed by row 1 again at this source.

RISK-VALUE: DERIVED — Enabled == "1" @ perfrec/perfrec.go:127 (with enabled != "1" @ perfrec/context.go:207 and EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47) — new writing needs ASSAY_TELEMETRY=1 exactly (spec lines 33 and 35); plant V-E6 is killed by rows 1 and 3 again at this source.

RISK-VALUE: DERIVED — directory mode 0o700 @ perfrec/perfrec.go:351/:358 and file mode 0o600 @ perfrec/perfrec.go:367/:387 — owner-only access, as the spec requires (lines 47 and 112).

RISK-VALUE: DERIVED — RetentionDays = 7 @ perfrec/perfrec.go:50 — seven UTC days of history (spec lines 32 and 113).

RISK-VALUE: DERIVED — NewID [16]byte from crypto/rand @ perfrec/context.go:114 — 128 random bits put the birthday bound near 2^64 IDs, far above any local seven-day history. The spec names no width, so this derivation is from first principles.

RISK-VALUE: NAMED, NOT DERIVED — MaxWork = 8 @ perfrec/context.go:76, MaxWorkBytes = 4096 @ perfrec/context.go:77, MaxRequestGroups = 64 @ perfrec/v3.go:80, issue bound 2147483647 @ perfrec/context.go:152 — reversible size bounds on optional context. The spec asks for "bounded" without numbers, and exceeding a bound marks the context invalid without failing the operation.

**VERIFY: PASS** — 4/4 Verify rows pass at 671d33a050bc, in the network-off container witness and by hand; bare row 4 exits 0 with 10 corroborated, 0 disproved, 0 unchecked. This is the second stamp; gate:human sign-off is pending, and the status is left for the human gate.

### Re-verify on the done path — 2026-11-24 at 52f341a6e0dc (container witness for rows 1 to 3, host witness for row 4)

Subject revision: public merged main 52f341a6e0dc25f7cdc915377f3e9ce6728df746, detached and clean. Delivery: the two-parent merge 3937f39ae2d00d150633f2c22568a79d1f5efb5a, whose first parent b787e831d0be847f1ca81cc48496c8c0deb01fe5 is the fail-first base. The dispatcher check (deskdispatch --check-verifier) exited 0 before any row and again after the bracket closed, with run 2e3e65e48eb6 and source 52f341a6e0dc both times. Reason for the re-run: deliverable files changed after the 2026-10-20 PASS (perfrec gained the perf-v4 and perf-v5 work, and perfrec.go, v3.go, reduce.go, schema.go, deskkit perf.go and perfread.go, and cmd/deskperf all changed since 671d33a050bc), so the earlier PASS does not carry forward.

Protected table: deskpathguard rederive at this source reports 4 rows, 0 author-added; it lists every row as from-base, with row 4 in its current form (pinned delivery and base, from 53d27c1887). No run in this pass refused with a protected-table error: verifyrun took the current table without an acceptance record on both the host and in the container. The Verify table was not edited.

Runner class: the pinned verify-runner image (digest sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380, statusgen v1.0.92, go1.27.4 linux/arm64, git 2.56.1), run under local docker with --network none, --read-only, --cap-drop ALL, the home mounted read-only, the bracket module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly and the history pack from the open step. Host: darwin/arm64, statusgen v1.0.92, go1.27.4, git 2.56.1. Rows 1 to 3 are check:ci and run only in the container lane, because the host has no unshare sandbox. Row 4 is a plain check and runs on the host, where it reads the git history.

#### Container witness, rows 1 to 3 (statusgen verifyrun, sandbox=container-netns, --fail-first)

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | pass exit=0 sandbox=container-netns | sha256:94ae9dfaafb6 | 2026-11-24 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=b787e831d0be unproven · head=52f341a6e0dc |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:9266e90d8bb8 | 2026-11-24 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=b787e831d0be unproven · head=52f341a6e0dc |
| 3 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | pass exit=0 sandbox=container-netns | sha256:f830b78791b2 | 2026-11-24 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=b787e831d0be unproven · head=52f341a6e0dc |
Tree-Digest: sha256:3cc0bbcb8407f28d67721c22b23324b961dba4995fbcd08fb0f0e4058f247071
Container-Image: sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7507 run=2e3e65e48eb619fbe54fb08a7584d615a1d7698132e578785688e61248c8b3b7 source=52f341a6e0dc25f7cdc915377f3e9ce6728df746 brief=docs/streams/desktools-v2/brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=2e3e65e48eb619fbe54fb08a7584d615a1d7698132e578785688e61248c8b3b7 source=52f341a6e0dc25f7cdc915377f3e9ce6728df746 tree=sha256:3cc0bbcb8407f28d67721c22b23324b961dba4995fbcd08fb0f0e4058f247071 witness=sha256:8477376c4df3390d35be16dda7a436f902d2f682fd55b63377553eb90f5315cc opened=2026-11-23T23:21:12Z closed=2026-11-24T00:03:57Z before=receipt after=check-verifier

The bracket-digest check on this file reports `bracket=L10 header=L1 verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`, with exit 0. It has not been checked for credit yet: the `--credit` check belongs to the landed brief.

#### Host witness, row 4 (statusgen verifyrun --rows 4 --fail-first --delivering-commit 3937f39ae2d0, dry run)

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 4 | `statusgen --root . --consumers --brief desktools-v2/63 --delivery 3937f39ae2d00d150633f2c22568a79d1f5efb5a --base b787e831d0be847f1ca81cc48496c8c0deb01fe5` | pass exit=0 | sha256:3eb8e84ce880 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=b787e831d0be red rc=2 · head=52f341a6e0dc · delivering=3937f39ae2d0 output=sha256:0ae5c2e6b56d8b0cb98ece339b25cd89b1ff04f902745fae69f1cd25274748a7 log=row-000001.log |
Verification-Attestation: medici-finance/assay#7507 run=2e3e65e48eb619fbe54fb08a7584d615a1d7698132e578785688e61248c8b3b7 source=52f341a6e0dc25f7cdc915377f3e9ce6728df746 brief=docs/streams/desktools-v2/brief-63-define-perf-v3-attribution-profiles-and-compatible-local-readers.md model=claude-opus-5-5 tier=strong

#### By-hand checks on the host at 52f341a6e0dc

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1 test command (go test and grep only; the subshell wrapper is held, see the note below) | exit 0; named PASS | go test exit 0; grep exit 0; --- PASS: TestPerfV3ContextAndConsent (0.02s) | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 test command (go test and grep only) | exit 0; named PASS | go test exit 0; grep exit 0; --- PASS: TestPerfV3RecordToReportFlow (0.18s) | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 test command (go test and grep only) | exit 0; named PASS | go test exit 0; grep exit 0; --- PASS: TestPerfV3IsolationAndOverhead (1.28s); matched workload n=100 concurrency=1 bytes/record=804 drops=0; on-minus-off ns p50=397917 p95=700458 max=2272084; concurrency=4 records=200 bytes=158400 drops=0 | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command, verbatim | exit 0; consumer routing corroborates the actual diff | exit 0; summary: 10 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing, over base b787e831d0be..3937f39ae2d0 (immutable delivered snapshot) | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

These by-hand rows earn no acceptance credit. For rows 1 to 3 the host has no network-off sandbox. The exact subshell form was refused by the session's command-safety check, because it hands rm to bash -c. That refusal was not worked around: the by-hand runs ran only the go test and grep steps, with no temp-file wrapper.

#### Fail-first (risk: sensitive-data yes makes every row risk-bearing)

| # | Base result | Reading |
|---|-------------|---------|
| 1 | unproven at base b787e831d0be (container) | The witness says "the row could not run there, so it was never shown to fail". The base-log directory from a single-row diagnostic re-run stayed empty, so the base command never ran. The base tree has no TestPerfV3ContextAndConsent. A host probe of the row's test step on an archive of base perfrec gives "no tests to run" and grep exit 1, which would be red. That probe earns no credit. |
| 2 | unproven at base b787e831d0be (container) | Same unproven result, with no base log. The host probe on a partial archive could not build: the forgeread replace directory was not in the archive. That is a probe limitation, not a result. |
| 3 | unproven at base b787e831d0be (container) | Same unproven result. The host probe of the test step gives grep exit 1, which would be red. No credit. |
| 4 | red rc=2 at base b787e831d0be (host), delivering=3937f39ae2d0 | The base log reads "--consumers --delivery: COULD-NOT-CHECK: delivery must be an ancestor of the checkout's HEAD". The row is red at base because the base cannot contain its own delivery. It is not red because a routing claim was disproved. |

#### Risk-bearing values (trigger: risk sensitive-data yes)

Enumerated over the delivery diff b787e831..3937f39a and re-read at this source. Values in files changed since the last verify sit on new lines.

- ProfileTool = "tool" @ perfrec/context.go:18; ProfileWorkflow = "workflow" @ perfrec/context.go:19
- tool-profile writer identity check: all linked fields empty and len(d.Work) == 0 @ perfrec/context.go:313
- opt-in enabled != "1" @ perfrec/context.go:207; Enabled: os.Getenv(EnvVar) == "1" @ perfrec/perfrec.go:136; EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47
- NewID [16]byte from crypto/rand @ perfrec/context.go:114
- RetentionDays = 7 @ perfrec/perfrec.go:50
- directory mode 0o700 @ perfrec/perfrec.go:378 and :385; file mode 0o600 @ perfrec/perfrec.go:282, :291, :394 and :414
- MaxWork = 8 @ perfrec/context.go:76; MaxWorkBytes = 4096 @ perfrec/context.go:77; MaxRequestGroups = 64 @ perfrec/v3.go:80; issue bound 2147483647 @ perfrec/context.go:152

Ranked by what breaks. The tool/workflow split, the opt-in, the file modes and the retention decide whether linked identity reaches disk; a disclosure cannot be undone by an edit and a redeploy. ID entropy comes next. The size bounds rank last as reversible knobs. Since 671d33a050bc, perfrec.go gained WriteEnabled. The v3 and v4 schemas still require the exact process opt-in at the writer. Legacy schemas keep their earlier writer behaviour, with no opt-in check at Write.

RISK-VALUE: DERIVED — ProfileTool = "tool" @ perfrec/context.go:18, writer check @ perfrec/context.go:313 — the spec says only the workflow profile carries linked context (docs/performance-attribution-spec.md line 61). The writer refuses session, agent, parent, deskRole, role and work values under the tool profile even with the parser bypassed.

RISK-VALUE: DERIVED — Enabled == "1" @ perfrec/perfrec.go:136 (enabled != "1" @ perfrec/context.go:207, EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:47) — new writing needs ASSAY_TELEMETRY=1 exactly (spec lines 35 and 37), and WriteEnabled keeps that exact check for v3 and v4 whatever consent the application resolved.

RISK-VALUE: DERIVED — directory mode 0o700 @ perfrec/perfrec.go:378/:385 and file mode 0o600 @ perfrec/perfrec.go:394/:414 — owner-only access, per the spec's restrictive permissions and its 0700 directories and 0600 files (spec lines 49 and 114).

RISK-VALUE: DERIVED — RetentionDays = 7 @ perfrec/perfrec.go:50 — seven UTC days (spec lines 34 and 115).

RISK-VALUE: DERIVED — NewID [16]byte from crypto/rand @ perfrec/context.go:114 — 128 random bits put the birthday bound near 2^64 IDs, far above any seven-day local history. The spec asks for random IDs (line 48) but names no width, so this derivation is from first principles.

RISK-VALUE: NAMED, NOT DERIVED — MaxWork = 8 @ perfrec/context.go:76, MaxWorkBytes = 4096 @ perfrec/context.go:77, MaxRequestGroups = 64 @ perfrec/v3.go:80, issue bound 2147483647 @ perfrec/context.go:152 — reversible size bounds. The spec asks for bounded values but gives no numbers.

#### Attempts

| # | Started (UTC) | Finished (UTC) | Lane | Rows run | Disposition |
|---|---------------|----------------|------|----------|-------------|
| A1 | 2026-11-23T23:19:06Z | 2026-11-23T23:19:27Z | host verifyrun --rows 4 --dry-run | 4 | uncredited, replaced by A2 (no fail-first) |
| A2 | 2026-11-23T23:20:45Z | 2026-11-23T23:21:16Z | host verifyrun --rows 4 --fail-first --dry-run | 4 | credited (row 4 at head); base red is COULD-NOT-CHECK (see Fail-first) |
| A3 | 2026-11-23T23:20:35Z | 2026-11-24T00:00:04Z | open bracket (history export) | 0 | bracket opened 2026-11-23T23:21:12Z |
| A4 | 2026-11-24T00:00:35Z | 2026-11-24T00:00:40Z | container verifyrun rows 1 to 3 | 0 | discarded: the brief path was relative to the container cwd and admission refused ("the receipt admits brief … but this run's brief is …"); no rows executed |
| A5 | 2026-11-24T00:01:07Z | 2026-11-24T00:03:08Z | container verifyrun rows 1 to 3, fail-first | 1, 2, 3 | credited at head; base unproven for every row. Bracket closed 00:03:57Z after one relative-path close refusal ("refused, nothing appended") was fixed with an absolute brief path |
| A6 | 2026-11-24T00:04:14Z | 2026-11-24T00:05:24Z | container verifyrun row 1, fail-first, --base-log (diagnostic, separate witness, not closed) | 1 | uncredited diagnostic: the same unproven base, and the base-log directory stayed empty |
| A7 | 2026-11-23 | 2026-11-23 | host by hand, rows 1 to 4 | 1, 2, 3, 4 | uncredited supplemental (table above) |
| A8 | 2026-11-24 | 2026-11-24 | host base probes, rows 1 to 3, on an archive of the base tree | 1, 2, 3 | uncredited probe; rows 1 and 3 grep exit 1; row 2 build failed because the archive was partial (environment) |

Refusals hit in this pass, quoted and not worked around:
- Claude Code command-safety check, on the by-hand form of rows 1 to 3: "This command passes a shell -c script that runs rm, and Claude Code could not check the script for dangerous removals."
- writeguard, on a process-listing pipeline whose grep filter contained a credential-shaped word: "writeguard: BLOCKED — ambient forge credential read". That line of work stopped. It was not needed.

**VERIFY: BLOCKED** — environment/instrument, rows 1, 2 and 3. All 4 rows pass at head: rows 1 to 3 in the bracketed network-off container witness (verdict=match), and row 4 on the host with 10 corroborated, 0 disproved, 0 unchecked. Fail-first did not complete. Rows 1 to 3 came back unproven at base b787e831d0be: the container's base run never ran a command, and its base log is empty. Row 4 is red at base only through COULD-NOT-CHECK, because the base cannot contain the delivery. No acceptance obligation failed. Gate: human; the status stays where it is.

## Review

Gate: human. Reject changed legacy meanings, guessed identity/cost, lost coverage or optional measurement that changes an operation.
