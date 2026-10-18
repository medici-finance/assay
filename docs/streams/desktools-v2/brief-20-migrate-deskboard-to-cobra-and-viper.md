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
version: 1
id: "b94c18ca-7a35-4495-8745-c07ff547e02f"
authored: "2026-10-04 by desktools-v2/15"
sources:
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

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

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

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
