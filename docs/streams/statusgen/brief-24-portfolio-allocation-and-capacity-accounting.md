---
brief: assay:assay:statusgen:24
title: Portfolio allocation and capacity accounting
why: "Allocation targets cannot be compared with agent usage or human attention reliably until their units, identities and missing data are explicit."
wave: 2
depends: ["statusgen/22"]
unblocks: ["statusgen/25"]
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
id: 5762abad-67ee-467a-9773-619f25a0dd06
consumers:
  - "spec/portfolio-measurement-v1.md, schemas/portfolio-measurement-v1.json, statusgen/portfolio: follow-up statusgen/24"
  - "statusgen portfolio CLI, projection consumers: follow-up statusgen/25"
  - "loopadmin/runner telemetry production: out-of-scope (live active-time collector is deferred; consume explicit attributed intervals)"
---

# Brief 24 — Portfolio allocation and capacity accounting

## Context

files: `spec/portfolio-measurement-v1.md` (planned), `schemas/portfolio-measurement-v1.json` (planned), `statusgen/portfolio/` (planned), `statusgen/testdata/portfolio/` (planned), `docs/contracts.md`; `changelog/statusgen-24-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- loopadmin/runner carries caller/id, profile, actual_model and usage; it does not promise active-time intervals.
- No current opmetrics aggregate proves a per-bucket decision identity or duration.
- Adopter policy supplies numeric weights and allocation targets; this brief introduces no default or dispatch effect.

layering: pure typed records/evaluators in `statusgen/portfolio`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/portfolio
  contract: portfolio-measurement-v1 (planned extension; register its semantic owner in docs/contracts.md)
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

1. Define portfolio policy records with scope/window/revision, qualified REQ bucket membership, independent agent/human targets, explicit weight policy and optional already-authorized limits. Validate units, overlapping/ambiguous assignment and source coverage without deciding policy.
2. Implement a pure ledger over explicit invocation segments and decision records. Deduplicate source reports by stable identity; distinct retries count separately. Preserve profile and actual model. Label elapsed-time proxies separately from active time and retain unknown measures, known subtotals, unattributed decisions and undefined zero-denominator shares.
3. Compare measured allocation with policy without modifying the target or dispatching filler. Provide a documented adapter example from the existing runner observation fixture, explicitly requiring a supplied interval source; preserve the runner contract and existing Evidence. Live collectors and default policies are out of scope.

## Interface contract

Input: versioned allocation policy, invocation/interval observations, decision records and visibility-filtered REQ references. Output: independent capacity ledgers, coverage, attributed divergence and explicit undefined/unknown states; never an admission decision.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioLedgerIdentityAndUnits$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioLedgerIdentityAndUnits " "$check_output")` | exit 0; named PASS; duplicate observations do not double count, retries count once each and human counts never join agent durations; mutation: count every report |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioUnknownAndZero$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioUnknownAndZero " "$check_output")` | exit 0; named PASS; unknown weight/time/cost only taints its measure; subtotal/coverage remain explicit; all-zero shares undefined; mutation: coerce missing to zero |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioRunnerAdapterFlow$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioRunnerAdapterFlow " "$check_output")` | exit 0; named PASS; actual runner-shaped fixture plus attributed interval reaches the ledger with profile/model preserved; missing interval cannot masquerade as active time; mutation: use elapsed duration silently |
| 4 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioTargetNotConsumption$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioTargetNotConsumption " "$check_output")` | exit 0; named PASS; 5 percent target with 2 percent observed remains divergent without a policy rewrite or effect; mutation: normalize target to consumption |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->
### Independent verification — 2026-10-29 (run 1, source 4264d1027, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 4264d1027198b8f257a95e1630f07cac358fcd6f, detached home admitted by `deskdispatch --check-verifier` (#4850).

Delivery: the work landed in the merge of #4337 (08b39a8cc951f0eaca1942d2e450c372f0d92e7a; first parent 5e54bfe7baed0d39cc76553f7d840644b5902191, branch head 3ee8abe79375040f21c721e62edb17d27146256a), an ancestor of the observed source. One later commit (294659a59, execution-profile contract) touched the delivered paths; this run observes the rows at the newer source.

Runner class: local container, `--network none`, default seccomp, image ghcr.io/medici-finance/verify-runner:v1.0.65 by digest sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f (statusgen v1.0.65, go1.27.3 linux/amd64, module cache baked into the image; no extra mount). Every row is check:ci and the darwin host has no `unshare --net`, so rows 1–4 ran inside one closed admission bracket: `statusgen verifyrun --open-bracket` on the host, `--sandbox=container-netns` in the container with the home bind-mounted read-only at /work, `--close-bracket` on the host; `--bracket-digest` on the closed witness reports verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. The table and the four lines after it are the closed witness bytes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioLedgerIdentityAndUnits$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioLedgerIdentityAndUnits " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:981a1d25eb84 | 2026-10-29 | assay-verifier-app[bot] @ 4264d1027198 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioUnknownAndZero$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioUnknownAndZero " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:0fde1b483ef1 | 2026-10-29 | assay-verifier-app[bot] @ 4264d1027198 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioRunnerAdapterFlow$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioRunnerAdapterFlow " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:5ff8701ae44b | 2026-10-29 | assay-verifier-app[bot] @ 4264d1027198 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioTargetNotConsumption$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioTargetNotConsumption " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:ced71cb955fa | 2026-10-29 | assay-verifier-app[bot] @ 4264d1027198 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:c3b9c02a14b638ecab5889a16e3de6d00f2cdd40631b04114a808fa12fcd1016
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4850 run=dd35a274b41b6c84975b6fb54111b82d8b41632ef8b2f16d77f62217b583754d source=4264d1027198b8f257a95e1630f07cac358fcd6f brief=docs/streams/statusgen/brief-24-portfolio-allocation-and-capacity-accounting.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=dd35a274b41b6c84975b6fb54111b82d8b41632ef8b2f16d77f62217b583754d source=4264d1027198b8f257a95e1630f07cac358fcd6f tree=sha256:c3b9c02a14b638ecab5889a16e3de6d00f2cdd40631b04114a808fa12fcd1016 witness=sha256:2d77f1ae8e677d995a6ee631366dcf12861649b144ec4cdbdf84a5b11c4670a6 opened=2026-10-29T18:54:12Z closed=2026-10-29T18:55:50Z before=receipt after=check-verifier

Supplemental observations — diagnostics only, no acceptance credit for any row. The probes ran in the same image under `--network none`, on a scratch copy of the observed source tree (git archive of 4264d1027) outside the home, each applying one entry of the delivery's own mutation catalogue (statusgen/portfolio/testdata/mutations.json, exact before/after replacement, before text matched exactly once) and then running that row's exact command shape.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | rows 1–4 command shapes on the unmutated scratch copy | each exit 0 with its named PASS line | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M1 | row 1, catalogue entry count-every-report (the brief's named mutation: every source report is counted) | killed: row 1 command exit 1, named test FAIL; "reliability active: got {Known:120 KnownRecords:1 UnknownRecords:1}, want {Known:240 KnownRecords:2 UnknownRecords:0}" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M2 | row 1, catalogue entry count-retry-as-duplicate (a distinct retry is folded into the first attempt) | killed: row 1 command exit 1, named test FAIL; "reliability active: got {Known:120 KnownRecords:1 UnknownRecords:0}, want {Known:240 KnownRecords:2 UnknownRecords:0}" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M3 | row 1, catalogue entry drop-overlap-reconciliation | killed: row 1 command exit 1, named test FAIL; "overlap double-counted" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M4 | row 2, catalogue entry coerce-missing-to-zero (the brief's named mutation) | killed: row 2 command exit 1, named test FAIL; "reliability cost: got {Known:0 KnownRecords:1 UnknownRecords:0}, want {Known:0 KnownRecords:0 UnknownRecords:1}" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M5 | row 2, catalogue entry zero-total-share-defined (an all-zero denominator yields a defined share) | killed: row 2 command exit 1, named test FAIL; "zero total produced a share" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M6 | row 3, catalogue entry use-elapsed-silently (the brief's named mutation: elapsed duration stands in for a missing active interval) | killed: row 3 command exit 1, named test FAIL; "missing interval: got {Known:900 KnownRecords:1 UnknownRecords:0}, want {Known:0 KnownRecords:0 UnknownRecords:1}" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M7 | row 3, catalogue entry pinned-model-as-actual (the profile's pinned model replaces the observed actual model) | killed: row 3 command exit 1, named test FAIL; "pinned model substituted" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M8 | row 4, catalogue entry normalize-target-to-consumption (the brief's named mutation) | killed: row 4 command exit 1, named test FAIL; "5% target against 2% consumption: ... TargetBasis:200 ... State:aligned" | 2026-10-29 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Expect coverage read from the named tests at the observed source: row 1's test also asserts that no human measure is tallied among agent measures and that an unattributed decision stays visible; row 4's test asserts the policy bytes are unchanged by the comparison, the comparison is a pure function of its inputs, and re-measuring after it yields the same ledger.

Fail-first: no row is risk-bearing (the brief's risk answers are all "no", and no row carries a risk-bearing, live, mutating or end-to-end tag), so no fail-first base witness is required; the named mutations above are the rows' discrimination evidence.

Risk-bearing values. Gate model, not irreversible, no risk-classed path touched. Enumeration over the delivery's non-test Go source (statusgen/portfolio/records.go, ledger.go, compare.go, decode.go, runner.go, checked.go) and the new schema found these literals:

- RISK-VALUE: DERIVED — Schema = "portfolio-measurement/v1" @ statusgen/portfolio/records.go:12 — the brief asks for a versioned contract; the identifier equals the schema's const and the spec's required `schema` member, and the decoder evaluates only this version.
- RISK-VALUE: DERIVED — basis-point ceiling 10000 in share() and the tolerance and per-party sum bounds @ statusgen/portfolio/decode.go:140, decode.go:171, decode.go:190 — one basis point is 1/10000 of the whole, so 10000 is exactly 100 percent; the spec fixes tolerance to 0–10000 and refuses one party's targets summing past 10000.
- RISK-VALUE: DERIVED — share scale 10000 in the exact comparison @ statusgen/portfolio/compare.go:76 — the observed share numerator/denominator is compared with a target in basis points, so the numerator is scaled by the same 10000; the comparison is exact integer arithmetic (big.Int), matching the spec's aligned-when-within-tolerance rule.
- RISK-VALUE: DERIVED — REQ slug bound {8,18} in reqRE = ^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$ @ statusgen/portfolio/decode.go:16 — one leading and one trailing [a-z0-9] plus 8–18 middle characters gives 10–20 characters, exactly the REQ-<slug> grammar spec/registers-v1.md line 120 fixes; the schema carries the same pattern.
- RISK-VALUE: DERIVED — RunnerVersion = "loop-admin-runner/v1" @ statusgen/portfolio/runner.go:13 — equals the const the existing runner contract schema (schemas/loop-admin-runner-v1.json line 12) requires, so the adapter consumes the runner contract unchanged.
- RISK-VALUE: DERIVED — ModelWeight.Milli unit 1000 = 1.0 @ statusgen/portfolio/records.go:86 — weighted active time is active seconds times a weight in thousandths, so the product is in weighted milliseconds, matching the measure name weighted-active-milliseconds and the spec's weights table; no default weight is shipped.
- Ranked last, no derivation needed: token grammar length {0,127} in tokenRE @ statusgen/portfolio/decode.go:14 — an identifier length cap on synthetic offline records, reversible by an edit.

Verdict accounting: 4 required rows; 4 pass, witnessed in one closed bracket; 0 fail; 0 could-not-run.

**VERIFY: PASS** — 4/4 rows at 4264d1027, each named mutation killed (supplemental).

### Independent verification — 2026-10-30 (run 2, source 011f14965, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 011f14965ef2cf173ee6be4ec3d4f319bb55ccae, detached home admitted by `deskdispatch --check-verifier` (#5071).

Delivery: the work landed in the merge of #4337 (08b39a8cc951f0eaca1942d2e450c372f0d92e7a, second parent 3ee8abe79), an ancestor of the observed source. Between the run 1 source 4264d1027 and this source, the delivered paths are byte-identical; only this brief and its stream index differ. This run's rows were executed afresh and take nothing from run 1.

Runner class: local container, `--network none`, default seccomp, image ghcr.io/medici-finance/verify-runner:v1.0.67 by digest sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa (statusgen v1.0.67, go1.27.3 linux/amd64 under emulation, module cache baked into the image). Every row is check:ci and the darwin host has no `unshare`, so rows 1–4 ran inside one closed admission bracket: `statusgen verifyrun --open-bracket` on the host, `--sandbox=container-netns` in the container with the home bind-mounted read-only at /work, `--close-bracket` on the host; `--bracket-digest` on the closed witness reports verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. The table and the four lines after it are the closed witness bytes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioLedgerIdentityAndUnits$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioLedgerIdentityAndUnits " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:d7f558bbe61f | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioUnknownAndZero$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioUnknownAndZero " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:82821d2542c7 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioRunnerAdapterFlow$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioRunnerAdapterFlow " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:ceebb943e80f | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioTargetNotConsumption$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioTargetNotConsumption " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:6e567f7bb055 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5071 run=acc86747a11c3c0678171611c51fa1b794dc98be11a959758bbacec294910b9e source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/statusgen/brief-24-portfolio-allocation-and-capacity-accounting.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=acc86747a11c3c0678171611c51fa1b794dc98be11a959758bbacec294910b9e source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae tree=sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d witness=sha256:f90bffd7f9176ffda9f94d53abe124051b08cc4545e14bfdce117724aecf5c2a opened=2026-10-30T23:38:58Z closed=2026-10-30T23:39:36Z before=receipt after=check-verifier

Supplemental observations — diagnostics only, no acceptance credit for any row. The probes ran in the same image under `--network none`, on a scratch copy of the observed source (git archive of 011f14965) outside the home, each applying one entry of the delivery's own mutation catalogue (statusgen/portfolio/testdata/mutations.json, exact before/after replacement, before text asserted to match exactly once, the patched file restored afterwards and the portfolio package confirmed identical to the home at the end) and then running that row's test selector and named PASS grep. The scratch copy held statusgen, its five replaced sibling modules, loopadmin and schemas; earlier attempts on narrower copies stopped on missing sibling files and are discarded.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | rows 1–4 selectors on the unmutated scratch copy | each exit 0 with its named PASS line | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M1 | row 1, catalogue entry count-every-report (the brief's named mutation) | killed: exit 1, named test FAIL; "reliability active: got {Known:120 KnownRecords:1 UnknownRecords:1}, want {Known:240 KnownRecords:2 UnknownRecords:0}" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M2 | row 1, catalogue entry count-retry-as-duplicate | killed: exit 1, named test FAIL; "reliability active: got {Known:120 KnownRecords:1 UnknownRecords:0}, want {Known:240 KnownRecords:2 UnknownRecords:0}" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M3 | row 1, catalogue entry drop-overlap-reconciliation | killed: exit 1, named test FAIL; "overlap double-counted" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M4 | row 2, catalogue entry coerce-missing-to-zero (the brief's named mutation) | killed: exit 1, named test FAIL; "reliability cost: got {Known:0 KnownRecords:1 UnknownRecords:0}, want {Known:0 KnownRecords:0 UnknownRecords:1}" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M5 | row 2, catalogue entry zero-total-share-defined | killed: exit 1, named test FAIL; "zero total produced a share" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M6 | row 3, catalogue entry use-elapsed-silently (the brief's named mutation) | killed: exit 1, named test FAIL; "missing interval: got {Known:900 KnownRecords:1 UnknownRecords:0}, want {Known:0 KnownRecords:0 UnknownRecords:1}" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M7 | row 3, catalogue entry pinned-model-as-actual | killed: exit 1, named test FAIL; "pinned model substituted" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M8 | row 4, catalogue entry normalize-target-to-consumption (the brief's named mutation) | killed: exit 1, named test FAIL; "5% target against 2% consumption: ... TargetBasis:200 ... State:aligned" | 2026-10-30 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Expect coverage read from the named tests at the observed source: row 1's test fails if any human measure is tallied among agent measures and keeps an unattributed decision visible; row 3's test feeds the loopadmin runner fixtures through the adapter with an attributed interval and checks profile and model survive, then removes the interval and requires active time unknown with the elapsed proxy kept separate; row 4's test holds a 500 basis-point target against 120/6000 observed as divergent, with policy bytes unchanged and the comparison a pure function of its inputs.

Fail-first: no row is risk-bearing (the brief's risk answers are all "no", and no row carries a risk-bearing, live, mutating or end-to-end tag), so no fail-first base witness is required; the named mutations above are the rows' discrimination evidence.

Risk-bearing values. Gate model, not irreversible, no risk-classed path touched. Enumeration over the delivery's non-test Go source (statusgen/portfolio records.go, ledger.go, compare.go, decode.go, runner.go, checked.go), read at the observed source, found these literals; none is irreversible, since all are revisable by an edit and a release of a read-only capability.

- RISK-VALUE: DERIVED — Schema = "portfolio-measurement/v1" @ statusgen/portfolio/records.go:12 — equals the const at schemas/portfolio-measurement-v1.json line 9; the brief asks for a versioned contract.
- RISK-VALUE: DERIVED — tolerance ceiling, per-party sum bound and share ceiling 10000 @ statusgen/portfolio/decode.go:140, decode.go:171, decode.go:190 — one basis point is 1/10000 of the whole, so 10000 is exactly 100 percent; spec/portfolio-measurement-v1.md lines 47 and 54 fix the same bounds.
- RISK-VALUE: DERIVED — share scale 10000 @ statusgen/portfolio/compare.go:76 — a numerator/denominator share is compared with a basis-point target as |numerator*10000 - want*denominator| <= tolerance*denominator, which is the aligned rule with the fraction cleared, in exact big.Int arithmetic.
- RISK-VALUE: DERIVED — reqRE slug bound {8,18} @ statusgen/portfolio/decode.go:16 — one leading and one trailing [a-z0-9] plus 8–18 middle characters gives 10–20 characters, the REQ-slug grammar of spec/registers-v1.md line 120; the schema carries the same pattern at line 121.
- RISK-VALUE: DERIVED — RunnerVersion = "loop-admin-runner/v1" @ statusgen/portfolio/runner.go:13 — equals the const at schemas/loop-admin-runner-v1.json line 12, so the adapter consumes the runner contract unchanged.
- RISK-VALUE: DERIVED — ModelWeight milli unit 1000 = 1.0 @ statusgen/portfolio/records.go:86 — weights are adopter-supplied thousandths with no default shipped; the runner test's 240 seconds at weight 1000 yields 240000, the weighted measure's unit.
- Ranked last, no derivation needed: tokenRE length {0,127} @ statusgen/portfolio/decode.go:14 — an identifier length cap on synthetic offline records, matching the schema pattern.

Verdict accounting: 4 required rows; 4 pass, witnessed in one closed bracket; 0 fail; 0 could-not-run.

**VERIFY: PASS** — 4/4 rows at 011f14965, each named mutation killed (supplemental).

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
