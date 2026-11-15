---
brief: "assay:assay:desktools-v2:23"
title: "Migrate deskfleet to Cobra and Viper"
why: >-
  deskfleet's verbs hand-dispatch FlagSets. Cobra gives each verb standard help and option
  forms while fleet output and exit codes stay the same.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "314a1bbe-f498-4396-9ada-bb0cbd377cce"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/23)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskfleet: follow-up desktools-v2/23 (command tree, declared configuration, contract tests)"
  - "plugins/assay/skills/install/SKILL.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/internal/regression/MANIFEST.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/23 (owned rows flip to migrated)"
---

# Brief 23 — Migrate deskfleet to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskfleet` — `deskfleet`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 1980 LOC, ~8 verbs, ~15 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskfleet/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-23.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-fleet.md` (planned).

facts:
- deskfleet reads fleet configuration; only declared keys bind through Viper.
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

1. Build a Cobra command tree for `deskfleet`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-23.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests; the command compares muhar's last line itself (`n` is the spec's mutation count), because muhar exits 0 when a mutation survives or cannot be applied |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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
### Independent verification — 2026-10-26 (run 1, source f2e232039, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main f2e232039c99455b0b9a7dacfe639f9f68b164d2, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#4555).

Delivery: the work landed in the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source.

Runner class: host, darwin-arm64, go1.27.2, statusgen v1.0.63. Every row is class check (none is check:ci), so no network-off sandbox applies and the rows ran on the host through `statusgen verifyrun`. The brief's risk answers are all "no" and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row owes a fail-first witness. Row 4's harness mutates the owned source in place and restores it; the home was confirmed clean after the run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-23.json` | pass exit=0 | sha256:5336c78b84d1 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:f4805e5aea45 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:da17a9e0b20a | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:f18a505d9ed3 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:2ec831d2fa39 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4555 run=bf50e3bd21fdcbdfa6cc07a228b4005e503cc8070139639bb1a7e5fe8fd8a687 source=f2e232039c99455b0b9a7dacfe639f9f68b164d2 brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Supplemental observations — diagnostics only, no acceptance credit for any row. The same commands re-run on the same source with their output kept, to name the key line behind each digest above.

| Probe | Command or check | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | rows 1–3 test commands, output kept | --- PASS: TestCLIHelpOffline (2.86s); --- PASS: TestCLIConfigFlow (0.05s); --- PASS: TestCLILegacyForms (0.02s) | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 | row 4 command, output kept | exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 | row 5 command, output kept | exit 0; ok for the deskfleet package | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S4 | row 7 command, output kept | exit 0; --- PASS: TestCLIOwnerMigrated (3.75s) | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S5 | row 8 command, output kept | exit 0; last line "seed passes=26" | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S6 | row 9 command, output kept | exit 0; "no brief files in the diff against f2e232039c99… — nothing to corroborate": on merged main the default three-dot diff is empty, so this row is green without judging anything | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S7 | statusgen --root . --consumers --brief desktools-v2/23 pinned to the delivery merge (base its first parent) | exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S8 | source read of the deskfleet package at the observed source | no import of the standard flag package and no FlagSet left in non-test source; parsing goes through the Cobra command tree in cli.go only | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Row 9 as written is non-discriminating after merge (S6). It is not a risk-bearing row and its Expect (exit 0, nothing disproved) is met; S7 shows the consumer routing holds when judged against the delivery itself.

Risk-bearing values. The brief's risk answers are all "no", gate model, not irreversible, and no risk-classed path is touched. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command) found these literals:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:136 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), the 1–365 bound at provision.go:72 is unchanged, and the legacy-forms test pins the default to 7.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:90, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:252 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the replaced parser returned exitUsage on every parse error, and row 4's positive control (UsageExit changed to 3) is caught.

Verdict accounting: 9 required rows; 9 pass; 0 fail; 0 could-not-run. No risk-bearing row, so no fail-first witness is owed.

**VERIFY: PASS** — 9/9 rows; mutation harness 11/11 caught (supplemental).

### 2026-10-27 closure: BLOCKED (check-definition)

The verified closure was refused by the closure lint: Verify row 9 [consumer-delivery-range] names no immutable delivery snapshot (no pinned `--delivery` and first-parent `--base`), so it cannot be the basis of a closure. The row-status flip was reverted to `implemented`. Every other row's witness above stands. The amendment of row 9 is tracked in #4560; it follows the protected-amendment path. The supplemental probe earns no row credit.

**VERIFY: BLOCKED**: row 9, check-definition, #4560.
### Independent verification — 2026-10-29 (run 2, source 1304d40f5, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 1304d40f5c32cae5136e87f6b1f0cbe5a008c72a, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#4915). The re-derived Verify table at this source has 9 rows, 0 author-added, every command and expect identical to the table run on 2026-10-26.

Delivery: the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source. The deskfleet command package and the owned mutation spec are byte-identical between the run-1 source f2e232039c99 and this source (empty diff).

Runner class: host, darwin-arm64, go1.27.2, statusgen v1.0.65. Every row is class check (none is check:ci), so no network-off sandbox applies and the rows ran on the host through `statusgen verifyrun`, with no forge token in its environment. The brief's risk answers are all "no" and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row owes a fail-first witness. Row 4's harness mutates the owned source in place and restores it; the home was confirmed clean apart from this Evidence section after the run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-23.json` | pass exit=0 | sha256:38e2e185ce0a | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:d155bf29cd3b | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:3fd60c1d77e5 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:a8c8da40718e | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:ec256dd0157f | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4915 run=dd8d4a99806bf4bd1180db431a58fa457c892ebb7d08370f8f87701f23709c7f source=1304d40f5c32cae5136e87f6b1f0cbe5a008c72a brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Supplemental observations — diagnostics only, no acceptance credit for any row, and not Evidence rows. The same commands re-run on the same source with their output kept, to name the key line behind each digest above. Observed on 2026-10-29 by the same verifier run as the witness above, at source 1304d40f5c32.

- S1 — rows 1–3 test commands combined in one run, output kept: exit 0; --- PASS: TestCLIHelpOffline (4.75s); --- PASS: TestCLIConfigFlow (0.29s); --- PASS: TestCLILegacyForms (0.03s).
- S2 — row 4 command, output kept: exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count (11 entries in its mutations list).
- S3 — row 5 command, output kept: exit 0; ok for the deskfleet package.
- S4 — row 7 command, output kept: exit 0; --- PASS: TestCLIOwnerMigrated (7.05s).
- S5 — row 8 command, output kept: exit 0; last line "seed passes=26".
- S6 — row 9 command, output kept, re-run after the witness above was appended to this Evidence section: exit 0; "summary: 0 corroborated, 0 disproved, 7 unchecked": every declared consumer is unchanged since the merge-base, so the diff against main is no evidence about any of them and the row is green without judging anything.
- S7 — statusgen --root . --consumers --brief desktools-v2/23 --delivery 3f15da841717fa597b7473eb8c0efbbd46e594c3 --base 90da708cb3a192d0fd32b7b85185c0c3fead5392: exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot.

Row 9 as written is still non-discriminating after merge (S6), and its command is unchanged at this source: it names no immutable delivery snapshot (no pinned delivery commit and first-parent base), which the consumer-delivery-range closure lint refuses as the basis of a verified closure ("new closures are blocking. No override."). The reviewed amendment tracked in #4560 has not landed and that issue is open. S7 shows the consumer routing holds when judged against the delivery itself, but a supplemental probe earns no row credit, so row 9 is held for the amendment and given no acceptance credit at closure.

Risk-bearing values. The brief's risk answers are all "no", gate model, not irreversible, and no risk-classed path is touched. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command) found these literals, at the same values and lines as run 1:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:136 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), and the 1–365 bound (provision.go:72 here, provision.go:88 at the first parent) keeps the same literals.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:90, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:252 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the first parent declared the same exitUsage = 2 (main.go:128), and row 4's positive control (UsageExit changed to 3) is caught (S2).

Verdict accounting: 9 required rows; 8 pass with acceptance credit (rows 1–8); 0 fail; row 9 exits 0 but is held, with no acceptance credit, for the check-definition amendment in #4560. No deliverable defect was observed.

**VERIFY: BLOCKED** — row 9, check-definition, #4560; rows 1–8 pass, mutation harness 11/11 caught (supplemental).

### Independent verification — 2026-11-06 (run 3, source bc078e917, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main bc078e9179c862dde02b367d35177f8ffdf1b421, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#5624). The re-derived Verify table at this source (`deskpathguard rederive`) has 9 rows, 0 author-added, every command and expect identical to the table run on 2026-10-26 and 2026-10-29.

Delivery: the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source. Since run 2 (source 1304d40f5c32) the deskfleet command package gained a push-token verb from later forge-gitlab work; its parsing is in the same Cobra tree and the owned contract and mutation rows below were re-run against it.

Runner class: host, darwin-arm64, go1.27.2, statusgen v1.0.75, git 2.56.0, normal HOME, KUBECONFIG=/dev/null, no forge token in the environment. Every row is class check (none is check:ci), so no network-off sandbox applies and the rows ran on the host through `statusgen verifyrun`. Row 4's harness mutates the owned source in place and restores it; after the run the only change in the home was this Evidence section.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-23.json` | pass exit=0 | sha256:473102fbf608 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:830602216cb9 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:a6de751c22f4 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:511946f753b5 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:aa21a4eea11a | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5624 run=8d4ab32d44b9982bf621fa761d659695b6258a4b01b26113c97202989400a6ce source=bc078e9179c862dde02b367d35177f8ffdf1b421 brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Supplemental observations — diagnostics only, no acceptance credit for any row, and not Evidence rows. The same commands re-run on the same source with their output kept, to name the key line behind each digest above. Observed on 2026-11-06 by the same verifier run as the witness above, at source bc078e9179c8.

- S1 — rows 1–3 test commands combined in one run, output kept: exit 0; --- PASS: TestCLIHelpOffline (3.44s); --- PASS: TestCLIConfigFlow (0.09s); --- PASS: TestCLILegacyForms (0.02s).
- S2 — row 4 command, output kept: exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count (11 entries in its mutations list).
- S3 — row 5 command, output kept: exit 0; ok for the deskfleet package.
- S4 — row 7 command, output kept: exit 0; --- PASS: TestCLIOwnerMigrated (7.14s).
- S5 — row 8 command, output kept: exit 0; last line "seed passes=26".
- S6 — row 9 command, output kept, re-run after the witness above was appended: exit 0; "summary: 0 corroborated, 0 disproved, 7 unchecked": the default base is HEAD on merged main, so the diff is empty and the row is green without judging any consumer.
- S7 — statusgen --root . --consumers --brief desktools-v2/23 --delivery 3f15da841717fa597b7473eb8c0efbbd46e594c3 --base 90da708cb3a192d0fd32b7b85185c0c3fead5392: exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot.
- S8 — statusgen --root . --lint, lines naming this brief: a NOTICE for row 9 [consumer-delivery-range], "consumer command has no explicit --base (the default becomes HEAD on merged main)"; this rule refuses a verified closure. NOTICEs for rows 1–3 [gotest-run-vacuous] do not fit the commands as written, which assert the PASS line with grep -c and test -eq 1.
- S9 — statusgen verifyrun --fail-first --base 90da708cb3a1 --dry-run --rows 6: exit 0; "no risk-bearing row among the selected rows"; dry run, nothing written.

Row 9 as written is still non-discriminating after merge (S6), and its command is unchanged at this source: it names no immutable delivery snapshot (no pinned delivery commit and first-parent base), so the consumer-delivery-range closure lint refuses it as the basis of a verified closure. The reviewed amendment tracked in medici-finance/assay#4560 has not landed and that issue is open. S7 shows the consumer routing holds when judged against the delivery itself, but a supplemental probe earns no row credit, so row 9 gets no acceptance credit until the amendment lands and a fresh dispatch runs it.

Fail-first. The brief's risk answers are all "no", gate model, not irreversible, and no row carries a risk-bearing, live, mutating or end-to-end tag (the tags are +dereference, +flow and +mutation only), so no row owes a fail-first witness; S9 shows the tool selects none.

Risk-bearing values. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command, 90da708cb3a1..3f15da841717) found these literals: the pat-expiry-days default, the 1–365 bound (moved, literals unchanged) and the exit codes. Line numbers below are at the observed source:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:145 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), and the 1–365 bound (provision.go:72 here) keeps the same literals.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:96, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:261 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the first parent declared the same exitUsage = 2, and row 4's positive control is caught (S2).

Verdict accounting: 9 required rows; 8 pass with acceptance credit (rows 1–8); 0 fail; row 9 exits 0 but gets no acceptance credit until the check-definition amendment in medici-finance/assay#4560 lands. No deliverable defect was observed.

**VERIFY: BLOCKED** — row 9, check-definition, medici-finance/assay#4560; rows 1–8 pass, mutation harness 11/11 caught (supplemental).
### Independent verification — 2026-11-10 (run 4, source 612750a12, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 612750a12406f19a96a83e6ea7687d794f772f9f, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#6015), checked again after the witness was saved and the brief restored. The re-derived Verify table at this source (`deskpathguard rederive`) has 9 rows, 0 author-added; row 4's command is the reviewed form that compares muhar's last line against the spec's mutation count, and every other command and expect is the table run on 2026-11-06.

Delivery: the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source. Since run 3 (source bc078e9179c8) the deskfleet command package and the owned mutation spec are unchanged; only the muhar harness changed, and row 4 ran against it.

Runner class: host, darwin-arm64, go1.27.3, statusgen v1.0.81, git 2.56.0, normal HOME, KUBECONFIG=/dev/null, TMPDIR under the resolved private tmp, no forge credential variable set. Every row is class check (none is check:ci), so no network-off sandbox or container bracket applies and the rows ran on the host through `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9`. The tool's Runner cells note that its read recorder is Linux-only and could not check reads on darwin. Row 4's harness mutates the owned source in place and restores it; after the run the only change in the home was the appended witness, which was saved and then reverted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-23.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:78966c0df7d0 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:c3ed513a4da4 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:71a0734d92f7 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:f6b33e22d043 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:938a538575ae | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6015 run=92294a7fcdc8eeb3d8e805d5f27718c5786bb5ef6da841d99957ff5b22dc6f48 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


Supplemental observations — diagnostics only, no acceptance credit for any row, and not Evidence rows. The same commands re-run on the same source after the witness run with their output kept, to name the key line behind each digest above. Observed on 2026-11-10 by the same verifier run, at source 612750a12406.

- S1 — rows 1–3 test commands combined in one run, output kept: exit 0; --- PASS: TestCLIHelpOffline (3.02s); --- PASS: TestCLIConfigFlow (0.06s); --- PASS: TestCLILegacyForms (0.02s).
- S2 — row 4's muhar run, output kept: exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count (jq reads 11 entries in its mutations list).
- S3 — row 5 command, output kept: exit 0; ok for the deskfleet package.
- S4 — row 7 command, output kept: exit 0; --- PASS: TestCLIOwnerMigrated (6.69s).
- S5 — row 8 command, output kept: exit 0; last line "seed passes=26".
- S6 — row 9 command, output kept: exit 0; "consumers: no brief files in the diff against 612750a12406… — nothing to corroborate": the default base is HEAD on merged main, so the diff is empty and the row is green without judging any consumer.
- S7 — statusgen --root . --consumers --brief desktools-v2/23 --delivery 3f15da841717fa597b7473eb8c0efbbd46e594c3 --base 90da708cb3a192d0fd32b7b85185c0c3fead5392: exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot.
- S8 — statusgen --root . --lint, lines naming this brief: a NOTICE for row 9 [consumer-delivery-range], "consumer command has no explicit --base (the default becomes HEAD on merged main)"; this rule refuses a verified closure. NOTICEs for rows 1–3 [gotest-run-vacuous] do not fit the commands as written, which assert the PASS line with grep -c and test -eq 1.
- S9 — source read of the deskfleet package's non-test Go files: no import of the standard flag package and no FlagSet; command parsing goes through the Cobra tree in cli.go and pushcli.go.

Row 9 as written is still non-discriminating after merge (S6), and its command is unchanged at this source: it names no immutable delivery snapshot (no pinned delivery commit and first-parent base), so the consumer-delivery-range closure lint refuses it as the basis of a verified closure (S8). The reviewed amendment tracked in medici-finance/assay#4560 has not landed and that issue is open. S7 shows the consumer routing holds when judged against the delivery itself, but a supplemental probe earns no row credit, so row 9 gets no acceptance credit until the amendment lands and a fresh dispatch runs it.

Fail-first. The brief's risk answers are all "no", gate model, not irreversible, and no row carries a risk-bearing, live, mutating or end-to-end tag (the tags are +dereference, +flow and +mutation only), so no row owes a fail-first witness.

Risk-bearing values. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command, 90da708cb3a1..3f15da841717) found these literals: the pat-expiry-days default, the 1–365 bound (moved, literals unchanged) and the exit codes. Line numbers below are at the observed source and match run 3, since the package is unchanged:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:145 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), and the 1–365 bound (provision.go:72 here) keeps the same literals.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:96, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:261 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the first parent declared the same exitUsage = 2, and row 4's positive control is caught (S2).

Verdict accounting: 9 required rows; 8 pass with acceptance credit (rows 1–8); 0 fail; 0 could-not-run; row 9 exits 0 but gets no acceptance credit until the check-definition amendment in medici-finance/assay#4560 lands. No deliverable defect was observed.

**VERIFY: BLOCKED** — row 9, check-definition, medici-finance/assay#4560; rows 1–8 pass, mutation harness 11/11 caught.

### Independent verification — 2026-11-13 (run 5, source 570cc3149, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 570cc3149c2982273060e8934901562951dfa866, the commit the dispatch packet named, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#6350, run a8202ee562d4) before any row and again before the credited attempt. The re-derived Verify table at this source (`deskpathguard rederive`) has 9 rows, 0 author-added, every command and expect identical to the table run on 2026-11-10.

Delivery: the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source. Since run 4 (source 612750a12406) the deskfleet command package changed through later fleet-label work: the labels verb gained `--roles` and `--check`, both declared in the same Cobra command spec, and its label table is now built from the roster roles. The owned mutation spec is unchanged; the muhar harness changed. Every row below ran against the current package.

Runner class: host, darwin-arm64, go1.27.4, statusgen v1.0.83, git 2.56.1, KUBECONFIG=/dev/null, no forge credential variable set. Every row is class check (none is check:ci), so no network-off sandbox or container bracket applies and the rows ran on the host through `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9`. The tool's Runner cells note that its read recorder is Linux-only and could not check reads on darwin. Row 4's harness mutates the owned source in place and restores it.

Attempts. Attempt 1 (2026-11-13T02:11:25Z to 02:13:14Z, 9 rows) is discarded as an environment fault: rows 1–7 and 9 passed, and row 8 exited 1 because one inherited floor seed in the deskdispatch package (the decision-gate credential test, go-binary case) resolves an installed desk-tools decision helper found on the host PATH ahead of the fake it plants, so the fake never runs. That test is not part of this brief's delivery; the fault is tracked as medici-finance/assay#6279. A diagnostic re-run of that test alone with no decision helper on the PATH passed both cases. Attempt 2 (02:15:37Z to 02:17:24Z, 9 rows) is the credited one: the same host and the same commands, with the PATH changed only to drop the directories that hold an installed decision helper; statusgen and every other installed desk tool stayed reachable. The table below is the tool's own in-home write from attempt 2, copied byte for byte, after which the brief was restored and the home left clean.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-23.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:2ace3c3aeccd | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:45eeb4169ca1 | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:d92700ba3bd2 | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:027d922ebdc9 | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:60b15c35a005 | 2026-11-13 | assay-verifier-app[bot] @ 570cc3149c29 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6350 run=a8202ee562d439de6a55a378b551bae89ab76eb76cda83d1844fb4a60012bd54 source=570cc3149c2982273060e8934901562951dfa866 brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


Supplemental observations — diagnostics only, no acceptance credit for any row, and not Evidence rows. The same commands re-run on the same source after the credited attempt, in the attempt-2 environment, with their output kept to name the key line behind each digest above. Observed on 2026-11-13 by the same verifier run, at source 570cc3149c29.

- S1 — rows 1–3 test commands combined in one run, output kept: exit 0; --- PASS: TestCLIHelpOffline (2.22s); --- PASS: TestCLIConfigFlow (0.04s); --- PASS: TestCLILegacyForms (0.01s).
- S2 — row 4's muhar run, output kept: exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count (jq reads 11 entries in its mutations list).
- S3 — row 5 command, output kept: exit 0; ok for the deskfleet package.
- S4 — row 7 command, output kept: exit 0; --- PASS: TestCLIOwnerMigrated (3.96s).
- S5 — row 8 command, output kept: exit 0; last line "seed passes=26".
- S6 — row 9 command, output kept: exit 0; "consumers: no brief files in the diff against 570cc3149c29… — nothing to corroborate": the default base is HEAD on merged main, so the diff is empty and the row is green without judging any consumer.
- S7 — statusgen --root . --consumers --brief desktools-v2/23 --delivery 3f15da841717fa597b7473eb8c0efbbd46e594c3 --base 90da708cb3a192d0fd32b7b85185c0c3fead5392: exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot.
- S8 — statusgen --root . --lint, lines naming this brief: a NOTICE for row 9 [consumer-delivery-range], "consumer command has no explicit --base (the default becomes HEAD on merged main)"; this rule refuses a verified closure. NOTICEs for rows 1–3 [gotest-run-vacuous] do not fit the commands as written, which assert the PASS line with grep -c and test -eq 1.
- S9 — source read of the deskfleet package's non-test Go files: no import of the standard flag package and no FlagSet; the new labels flags are declared keys in the Cobra command spec in cli.go.

Row 9 as written is still non-discriminating after merge (S6), and its command is unchanged at this source: it names no immutable delivery snapshot (no pinned delivery commit and first-parent base), so the consumer-delivery-range closure lint refuses it as the basis of a verified closure (S8). The reviewed amendment tracked in medici-finance/assay#4560 has not landed and that issue is open (read 2026-11-13). S7 shows the consumer routing holds when judged against the delivery itself, but a supplemental probe earns no row credit, so row 9 gets no acceptance credit until the amendment lands and a fresh dispatch runs it.

Fail-first. The brief's risk answers are all "no", gate model, not irreversible, and no row carries a risk-bearing, live, mutating or end-to-end tag (the tags are +dereference, +flow and +mutation only), so no row owes a fail-first witness.

Risk-bearing values. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command, 90da708cb3a1..3f15da841717) found these literals: the pat-expiry-days default, the 1–365 bound (moved, literals unchanged) and the exit codes. The later labels change adds two flags with no numeric default and documents exit 6 for `--check`, outside this brief's delivery. Line numbers below are at the observed source:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:156 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), and the 1–365 bound (provision.go:72 here) keeps the same literals.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:96, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:274 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the first parent declared the same exitUsage = 2, and row 4's positive control is caught (S2).

Verdict accounting: 9 required rows; 8 pass with acceptance credit (rows 1–8, attempt 2); 0 fail; 0 could-not-run; row 9 exits 0 but gets no acceptance credit until the check-definition amendment in medici-finance/assay#4560 lands. Attempt 1's row 8 exit is an environment fault (medici-finance/assay#6279), not a deliverable defect. No deliverable defect was observed.

**VERIFY: BLOCKED** — row 9, check-definition, medici-finance/assay#4560; rows 1–8 pass, mutation harness 11/11 caught.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
