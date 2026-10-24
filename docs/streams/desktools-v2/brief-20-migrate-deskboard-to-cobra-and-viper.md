---
brief: "assay:assay:desktools-v2:20"
title: "Migrate deskboard to Cobra and Viper"
why: >-
  deskboard renders the cross-stream board every desk reads at boot, and parses its arguments
  by hand. A Cobra tree gives it standard help and option forms without changing the board
  text the desks parse.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 2
id: "b94c18ca-7a35-4495-8745-c07ff547e02f"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "assay:assay:desktools-v2:60 — consumer routing for the 2026-10-16 performance amendment"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/20)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskboard: fixed-here (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: out-of-scope (invocation forms preserved and replayed by cli_test.go; no quoted help changed)"
  - "tools/desk/README.md: fixed-here (roster inspection uses deskroster repos --scope write; help and version read no roster)"
  - "docs/adopting-assay.md: fixed-here (applicability and verification use local deskroster write-scope inspection without board roots; retain command exit and inspect effective values)"
  - "plugins/assay/scripts/assay-inbox.sh: out-of-scope (invocation forms preserved and replayed by cli_test.go; no quoted help changed)"
  - "plugins/assay/skills/worker-desk/SKILL.md: out-of-scope (invocation forms preserved and replayed by cli_test.go; no quoted help changed)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: out-of-scope (invocation forms preserved and replayed by cli_test.go; no quoted help changed)"
  - "docs/streams/desktools-v2/cli-migration.json: fixed-here (owned rows flip to migrated)"
---

# Brief 20 — Migrate deskboard to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskboard` — `deskboard`; module tools/desk; parser: manual os.Args parsing; 8975 LOC, ~19 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskboard/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-20.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-board.md` (planned).

facts:
- deskboard parses `os.Args` manually; its stdout is read by desk skills and loops, so non-help output is byte-compatible.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
design-fit:
  owner: each command's own package; domain owners unchanged
  contract: none — consumes cli-contract.md delivered by /15
  retires: [hand-rolled flag loops and usage strings]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve existing domain checks, custody decisions and exit codes; a
CLI library is not a new authority source. Independent verification owns Evidence and
lifecycle advancement. Proposed tests and files below are deliverables, not claims that
they already exist.

## Task

1. Build a Cobra command tree for `deskboard`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

| # | Class | Command | Expect |
|---|---|---|---|
| 10 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskboard/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | exit 0; named PASS; compiled help/version with malformed config and a large manifest fixture performs zero operational config/discovery/custody reads; moving discovery before help fails |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Performance reuse amendment — 2026-10-16

Keep this brief's parser ownership and existing compatibility rows. The shared credential
implementation belongs to /58–59, runtime manifest discovery to /60 and timing semantics to
/61–62. Help/version acceptance includes every pre-handler entrypoint step: no config echo,
activation discovery, file/index read, credential lookup or child process may occur first.
Count those effects in the compiled command test, not only inside the handler. If the shared
performance recorder is armed, a local best-effort performance record remains subject to its
existing explicit consent; help must not read operational configuration to classify itself.
Retain the one measurement finalizer when moving to Cobra. Do not reset the timer after
parsing, add a second writer or reintroduce a minter callout in an adapter.


## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->
<!-- Independent verifier records actual command, exit, output, date and runner. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | pass exit=0 | sha256:5af695d3f0fc | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | pass exit=0 | sha256:ca8052c3bfe2 | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:5682054eb457 | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:93db08669734 | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:5c9d86c64ea1 | 2026-10-18 | assay-verifier-app[bot] @ 56615f9ab834 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3359 run=f26761b905f34c6eaf1288515597d0eb1ea3728ef1bf5ad0a5dd575db83f94a2 source=56615f9ab834a2218709271e55ce3cc4ba80c991 brief=docs/streams/desktools-v2/brief-20-migrate-deskboard-to-cobra-and-viper.md model=gpt-6.1-sol tier=strong


### Independent observations — 2026-10-18

Source: 56615f9ab834a2218709271e55ce3cc4ba80c991. Delivering PR [#3248](https://github.com/medici-finance/assay/pull/3248) is merged as cc86c03eb5d44f91b7a4cf9aa1fb777c63ef62f0; it is an ancestor of the observed source. Runner: assay-verifier-app[bot], independent gpt-6.1-sol / strong. Expectations were written from the brief and CLI contract before implementation/diff inspection.

The execution-witness table above is the original statusgen verifyrun result (exit 0). The following table is a separate exact-command replay retaining complete output, including the files to which rows 1–3 and 7 redirect. Each numbered row discharges the matching Verify row.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | Verify 1: exit 0 | exit=0; --- PASS: TestCLIHelpOffline (1.14s) | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | Verify 2: exit 0 | exit=0; --- PASS: TestCLIConfigFlow (0.01s) | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | Verify 3: exit 0 | exit=0; --- PASS: TestCLILegacyForms (0.04s) | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | Verify 4: exit 0; all 7 mutations caught | exit=0; Harness healthy: baseline GREEN, positive control CAUGHT. Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate. | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | Verify 5: exit 0 | exit=0; ok github.com/medici-finance/assay/tools/desk/cmd/deskboard 12.011s | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | Verify 6: exit 0 | exit=0; no output; Windows-target vet completed | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | Verify 7: exit 0 | exit=0; --- PASS: TestCLIOwnerMigrated (2.61s) | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | Verify 8: exit 0 | exit=0; seed passes=26 | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |
| 9 | `statusgen --root . --consumers` | Verify 9: exit 0 | exit=0; summary: 0 corroborated, 0 disproved, 8 unchecked, 0 brief(s) claiming nothing | 2026-10-18 assay-verifier-app[bot] / gpt-6.1-sol |

Row 9 satisfies only its stated no-disproved-routing criterion. Its eight UNCHECKED claims were unchanged relative to this already-merged checkout; this instrument does not corroborate them. Source review of the delivering diff confirmed the roster-inspection documentation routes through deskroster repos --scope write; parser replay covers the retained board callers. Native Windows behavior remains with the CI lane; row 6 checks cross-compilation only.

Protected-path follow-up: a complete forge label-history read found current wrote-to-the-test and approval-needed on PR #3248. That label was discovered after the original witness had started. deskpathguard rederive exited 0 with "0 row(s), 0 author-added, base=56615f9ab834a2218709271e55ce3cc4ba80c991"; zero rows is a tool limitation here, not proof of coverage. A direct byte comparison of the entire Verify table at delivering parent cc86c03^ and the admitted source returned True, establishing that all nine commands and expectations predate this delivery. The full-output replay ran after that comparison. The status-transition wording in docs/protected-paths.md lines 62–65 and the brief's gate:model metadata are returned to the supervisor for lifecycle handling; this verifier changes no lifecycle field.

Risk enumeration: topology.yaml line 206 classes the changed deskboard path. The retained mechanical inventory covers all 14 files in cc86c03^..cc86c03, with 613 added-line string, number and boolean tokens, each source-located and ranked. Highest are the dispatch filter binding and accepted vocabulary; CLI bounds/arity/output tokens follow; documentation, registry, test and mutation literals rank last. All are reversible by edit/redeploy. Operational defaults remain min-age-hours="48" (main.go:108), merge-now-threshold="20m" (:109), unreviewed-threshold="30m" (:110), false output/throughput flags (:103–106,115), empty out (:107), quota floor 500 (readcost.go:63), floor minimum 0 (main.go:251), positive age/PR minimum 1 (:232,:207), and reviews/diff arity 2 versus files 2..3 (:199–203). No risk yes or risk-bearing/live/mutating/end-to-end Verify class exists, so fail-first is not required.

RISK-VALUE: DERIVED — tier.default = "" @ tools/desk/cmd/deskboard/main.go:114 — omitted tier retains the pre-delivery admission contract: a board without a dispatcher tier applies no tier filter; an explicitly supplied empty or unknown tier is refused before execution.

RISK-VALUE: DERIVED — execTierValues = []string{"any", "strong"} @ tools/desk/internal/deskkit/modelstamp.go:183 — the CLI and direct domain boundary both call CanonicalDispatchTier, whose single vocabulary mirrors the brief-schema exec-tier set; the parser migration introduces no new tier authority.

**VERIFY: PASS** — all nine stated Verify rows passed at the admitted source; lifecycle eligibility remains with the supervisor.

### Independent re-verification after the 2026-10-16 amendment — 2026-10-18

Source: a724e99dcd89a00f22de274f92ef0ca54fa918da (merged main). The earlier verification above ran at 56615f9ab834, before commit 379310083 (#3190) raised this brief to version 2 and added Verify row 10 plus the performance reuse amendment, so it covered nine rows. This pass runs all ten rows at the admitted source. Runner: assay-verifier-app[bot], dispatched non-implementer, claude-opus-5-5 / strong. Rows 1-3, 7 and 10 were run twice: once exactly as written (exit code recorded below) and once with the redirected file printed, for the key output line.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | Verify 1: exit 0; one named PASS | exit=0; --- PASS: TestCLIHelpOffline (1.65s) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | Verify 2: exit 0; one named PASS | exit=0; --- PASS: TestCLIConfigFlow (0.03s) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | Verify 3: exit 0; one named PASS | exit=0; --- PASS: TestCLILegacyForms (0.06s) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | Verify 4: exit 0; Totals N caught, 0 NOT CAUGHT, 0 could-not-mutate, N = spec count | exit=0; Harness healthy: baseline GREEN, positive control CAUGHT. Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate. (spec lists 7 mutations) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | Verify 5: exit 0 | exit=0; ok github.com/medici-finance/assay/tools/desk/cmd/deskboard 14.011s | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | Verify 6: exit 0 | exit=0; empty output (Windows-target vet only; native Windows behaviour stays with the CI lane) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | Verify 7: exit 0; named PASS | exit=0; --- PASS: TestCLIOwnerMigrated (2.47s) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | Verify 8: exit 0 | exit=0; seed passes=26 | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 9 | `statusgen --root . --consumers` | Verify 9: exit 0; no routing disproved | exit=0; consumers: no brief files in the diff against a724e99dcd89a00f22de274f92ef0ca54fa918da — nothing to corroborate (zero disproved; on merged main this instrument corroborates nothing either) | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskboard/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | Verify 10: exit 0; named PASS | exit=1; go test exit 0 but the `-run` pattern matched zero test functions ("ok .../cmd/deskboard 0.202s" with the go test zero-match warning); grep found no PASS line. A search of tools/ at the admitted source finds no function named TestCLIHelpDoesNotDiscoverManifests in any package. | 2026-10-18 assay-verifier-app[bot] / claude-opus-5-5 |

Row 10 is this brief's own deliverable under its 2026-10-16 performance reuse amendment (help/version must perform no operational config, discovery or custody read, counted in the compiled command test). The delivering PR #3248 (cc86c03eb5d4) predates the amendment, and no later commit to tools/desk/cmd/deskboard adds the test. Rows 1-9 still pass at the admitted source. Failure class: amended Verify row with no implementation; this is a re-opened scope gap, not a regression in the merged migration.

Risk enumeration (diff scope: cc86c03^..cc86c03, the 14 files of the delivering change, plus the brief's Deliverables). Literals introduced or carried through the new bindings in tools/desk/cmd/deskboard/main.go: min-age-hours Default "48" @ main.go:108 (pre-migration stalledMinAgeDefault = 48 @ stalled.go:50); merge-now-threshold Default "20m" @ main.go:109 (pre-migration 20 * time.Minute); unreviewed-threshold Default "30m" @ main.go:110 (pre-migration 30 * time.Minute); table/json/delta/quiet/throughput Default false @ main.go:103-106,115; out Default "" @ main.go:107; tier Default "" @ main.go:114; files arity max 3 versus 2 for the other PR verbs @ main.go:199-203; PR number lower bound n <= 0 refused @ main.go:207; min-age lower bound o.minAge <= 0 refused @ main.go:232; rate floor o.floor < 0 refused @ main.go:251; UsageExit deskkit.ExitRefused = 5 @ internal/deskkit/exitcodes.go:51; defaultRateFloor = 500 @ readcost.go:63 (unchanged, now reached through the Viper fallback); execTierValues = []string{"any", "strong"} @ internal/deskkit/modelstamp.go:183 (unchanged vocabulary, consumed by the tier binding). Ranking: none is irreversible. The highest is the tier binding and its vocabulary (a wrong value mis-filters dispatch candidates), then the usage exit code (callers branch on it), then the operational thresholds and output flags; every one is undone by an edit and redeploy, and the brief's risk frontmatter is all "no".

RISK-VALUE: DERIVED — tier Default = "" @ tools/desk/cmd/deskboard/main.go:114 — only a flag-sourced value enters CanonicalDispatchTier (main.go:254); an omitted tier keeps the pre-migration behaviour of applying no tier filter, and an explicit unknown value is refused before any read.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/deskboard/main.go:219 with the constant at tools/desk/internal/deskkit/exitcodes.go:51 — the CLI contract maps unknown flags and bad values to the shared refused exit, the same code the hand-rolled parser returned for refusals, so callers that branch on exit 5 keep working.

RISK-VALUE: DERIVED — min-age-hours Default = "48", merge-now-threshold Default = "20m", unreviewed-threshold Default = "30m" @ tools/desk/cmd/deskboard/main.go:108-110 — each equals the pre-migration constant (stalledMinAgeDefault = 48, 20 * time.Minute, 30 * time.Minute), so the migration changes no board threshold; these are reversible operational knobs.

Verification-Attestation: medici-finance/assay#3413 run=101d9fb421a83b8631166f19204c99366533a5c5c52ea0b5f2cf294d1dc718f0 source=a724e99dcd89a00f22de274f92ef0ca54fa918da brief=docs/streams/desktools-v2/brief-20-migrate-deskboard-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — row 10: TestCLIHelpDoesNotDiscoverManifests does not exist in ./cmd/deskboard at a724e99dcd89 (exit=1, no PASS line); rows 1-9 pass. The amendment's help-before-discovery test is undelivered.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | pass exit=0 sandbox=container-netns | sha256:47c29ff10c70 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | pass exit=0 sandbox=container-netns | sha256:2008d98f7c97 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 sandbox=container-netns | sha256:3b45f7b14f13 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 sandbox=container-netns | sha256:b4f6f81c5f90 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 sandbox=container-netns | sha256:40df24304ba3 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskboard/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |

### Re-wake verification — 2026-10-19

Runner: local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), `--network none`, sandbox=container-netns, default seccomp, per the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Source 0e661b6e15de107efcf05ed7d8d0d21983eb3afa; independent verifier claude-opus-5-5 / strong.

Verification-Attestation: medici-finance/assay#3518 run=4964d0f10c282156d5b5d38e3bb86366b3ee4649607af09ac2c3fc48aab92790 source=0e661b6e15de107efcf05ed7d8d0d21983eb3afa brief=docs/streams/desktools-v2/brief-20-migrate-deskboard-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

The prior Evidence verdict (2026-10-18 at a724e99dcd89) was FAIL on row 10 alone; since then the only change under tools/desk/cmd/deskboard is f4f2a9209 (a withdrawn-brief reconcile disposition in the stalled report), which adds no test.

Row 10 — real deliverable defect. The witness records exit=1, and a by-hand run on darwin agrees: go test exits 0 with "testing: warning: no tests to run" and "ok .../cmd/deskboard [no tests to run]", so the grep finds no PASS line. No function named TestCLIHelpDoesNotDiscoverManifests exists in tools/desk/cmd/deskboard at the source or at origin/main 9f0cd4a5f79f; the only one is in tools/desk/cmd/deskdispatch (brief desktools-v2/21's matching row, which passes there). The row is not stale: the test name was never renamed or removed from this package, because it was never delivered here. The 2026-10-16 performance amendment requires a compiled-command help-before-discovery effect count, and that test has not been written for deskboard. Rows 1-9 pass in the witness and by hand.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/deskboard/main.go:219, constant at tools/desk/internal/deskkit/exitcodes.go:51 — unchanged since the prior derivation; the shared refused exit is what callers branch on.
RISK-VALUE: DERIVED — min-age-hours "48", merge-now-threshold "20m", unreviewed-threshold "30m" @ tools/desk/cmd/deskboard/main.go:108-110 — unchanged, equal to the pre-migration constants; reversible operational knobs.

**VERIFY: FAIL** — row 10: TestCLIHelpDoesNotDiscoverManifests is still absent from ./cmd/deskboard at 0e661b6e15de (exit=1, no PASS line); rows 1-9 pass.

### Re-verification — 2026-10-21

Runner: rows 1-8 and 10 in the local container image assay-verify:go1.27-py (id sha256:9951b77a257f, go1.27.2 linux/arm64, git 2.47.3) with `--network none`, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, the source and module cache mounted read-only and the commands run in a tmpfs copy. Row 8 was also run on the darwin host in a scratch clone at the source (GOPROXY=off), because the container result is environmental (see below). Row 9 ran on the darwin host (statusgen v1.0.54). Source c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a; independent verifier claude-opus-5-5 / strong.

Verification-Attestation: medici-finance/assay#3966 run=ce10944622256969b914901b9cdbb210bf6c769756569680e932c250ee128f83 source=c82a8c5f807bbaadfc7dd6a0f0f82bc388ab0c0a brief=docs/streams/desktools-v2/brief-20-migrate-deskboard-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; one TestCLIHelpOffline PASS line | pass exit=0 (container, network none) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; one TestCLIConfigFlow PASS line | pass exit=0 (container, network none) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskboard -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; one TestCLILegacyForms PASS line | pass exit=0 (container, network none) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-20.json` | exit 0; Totals N caught, 0 NOT CAUGHT, 0 could-not-mutate, N = spec count | pass exit=0; "Harness healthy: baseline GREEN, positive control CAUGHT."; "Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate."; spec lists 7 mutations | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskboard/...` | exit 0 | pass exit=0; ok .../tools/desk/cmd/deskboard 9.010s | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskboard` | exit 0 | pass exit=0, no output | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/20 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS | pass exit=0; --- PASS: TestCLIOwnerMigrated (5.16s) | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every floor seed passes | pass exit=0 on the darwin host clone at the source; "seed passes=26", zero FAIL lines. In the container the same command exited 1 on one fixture (TestReg1145ShimCredential, no-gh case expects GH_CALL_RC=127 under PATH=/usr/bin:/bin, but that image ships gh in /usr/bin): an environment assumption of the fixture, not a deskboard change | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --consumers` | exit 0 | pass exit=0; "consumers: no brief files in the diff against c82a8c5f807b — nothing to corroborate" | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskboard/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | exit 0; named PASS | fail exit=1 (container, network none); go test alone reports "testing: warning: no tests to run" and "ok .../cmd/deskboard [no tests to run]", so no PASS line; the only function of that name is in tools/desk/cmd/deskdispatch | 2026-10-21 | assay-verifier-app[bot] @ c82a8c5f807b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row 10 stays a real deliverable defect, the same as the 2026-10-18 and 2026-10-19 passes: the performance amendment's compiled-command help-before-discovery test was never written for deskboard. Recent deskboard commits (the token-classification fixes and their tests) add no test of that name.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/deskboard/main.go:225, constant at tools/desk/internal/deskkit/exitcodes.go:51 — the CLI contract maps unknown flags and bad values to the shared refused exit, the code the hand-rolled parser returned, so callers that branch on exit 5 keep working.
RISK-VALUE: DERIVED — min-age-hours Default "48", merge-now-threshold Default "20m", unreviewed-threshold Default "30m" @ tools/desk/cmd/deskboard/main.go:110-112 — equal to the pre-migration values (stalledMinAgeDefault = 48 at tools/desk/cmd/deskboard/stalled.go:50, 20 and 30 minutes); reversible operational knobs.

**VERIFY: FAIL** — row 10: TestCLIHelpDoesNotDiscoverManifests is still absent from ./cmd/deskboard at c82a8c5f807b (exit=1, no PASS line); rows 1-9 pass (row 8 on host; its container red is the fixture's environment assumption). (blocker: #2111)

Outcome record: no verify-fail outcome record has landed for this pass yet; it lands on this Evidence branch through #4181 (a fresh admitted re-dispatch writes it with `deskevidence --outcome-record`; records are never hand-written).


## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
