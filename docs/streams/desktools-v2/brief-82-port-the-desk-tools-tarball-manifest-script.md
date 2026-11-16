---
brief: "assay:assay:desktools-v2:82"
title: "Port the desk-tools tarball manifest script"
why: >-
  The release writes and checks each desk-tools tarball's MANIFEST.sha256 with a Python
  script that parses its own argv. It is a maintained entrypoint the release runs and an
  operator can run from a checkout, so it needs an owner under the CLI contract. No existing
  child covers it without widening its scope, so this brief owns it.
wave: 4
depends: ["desktools-v2/16", "desktools-v2/46"]
unblocks: ["desktools-v2/17"]
effort: "S"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [6604]
schema: "brief-v2"
outcome: none
version: 1
id: "582f1fee-71af-40d4-b420-60a3eba01fb3"
authored: "2026-11-16 by worker-desk (routing the entrypoint added for #6604)"
sources:
  - "https://github.com/medici-finance/assay/issues/6604"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/brief-80-port-the-release-spec-header-stamp-script.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/82)"
  - "tools/release/desk_manifest.py, tools/release/desk_manifest_test.py, tools/release/README.md and both release.yml copies, read at authoring"
exec-tier: "strong"
exec-tier-why: >-
  Release-path compatibility: both release workflow copies call the module form with
  positional arguments, and the check's fail-closed directions (changed, missing and
  unlisted files) must survive the port unchanged.
domain: "complicated"
consumers:
  - "tools/release/desk_manifest.py: follow-up desktools-v2/82 (transparent adapter to the Go verb)"
  - "tools/desk/cmd/deskrelease: follow-up desktools-v2/82 (manifest write and check verbs)"
  - ".github/workflows/release.yml: follow-up desktools-v2/82 (module invocation kept; no workflow edit)"
  - "ci/staged-workflows/release.yml: follow-up desktools-v2/82 (module invocation kept; no workflow edit)"
  - "tools/release/README.md: follow-up desktools-v2/82 (quoted usage reconciled)"
  - "docs/distribution.md: follow-up desktools-v2/82 (quoted check command reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/82 (owned row flips to migrated)"
---

# Brief 82 — Port the desk-tools tarball manifest script

## Context

files:
- `tools/release/desk_manifest.py` — `desk_manifest.py`; python3; parser: hand-rolled `sys.argv`, two positionals (`write|check <dir>`), no flags; 142 lines. Target: `tools/desk/cmd/deskrelease`.
- `tools/release/desk_manifest_test.py` — the script's offline suite; it runs against the adapter unchanged.
- `tools/desk/cmd/deskrelease/cli_test.go` — TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms gain the manifest verbs' cases.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-82.json` (planned) — muhar mutation spec for the manifest verbs.
- `docs/streams/desktools-v2/cli-migration.json` — the owned row flips to migrated.

facts:
- The script was added with #6604. The release workflow's desk-tools package step runs `python3 -B -m tools.release.desk_manifest write "$stage"` and then `check "$stage"` before `tar`, in both `.github/workflows/release.yml` and `ci/staged-workflows/release.yml`. `tools/release/README.md` and `docs/distribution.md` quote the `check` form for operators. Re-enumerate at pickup.
- Its contract: exit 0 on a written or clean manifest; exit 1 when `check` finds a problem; exit 2 for a usage error or when it cannot run (no directory, no manifest, an empty directory on `write`, a symlink or non-regular entry). `check` refuses a changed file, a listed file that is missing, a file present but not listed, a duplicate line and a malformed line. `write` goes through a temp file and `os.replace`.
- The row was routed when the script landed. /46 owns the existing `deskrelease` command and /52 is at the five-simple-row ceiling and /47 is already `implemented`, so this brief owns it rather than widening another child.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). A refused positional count exits 2 (`ExitUsage`), which matches the script's usage exit.

layering: Cobra adds `manifest write` and `manifest check` under `deskrelease`, passing the validated directory into handlers that keep the script's walk, refusals and exit codes. No configuration key, flag or environment variable relaxes the unlisted-file refusal or skips a direction of the check.
single-point-of-failure: the `check` handler's three directions (changed, missing, unlisted); the existing Python suite pins each one and runs against the adapter, and the mutation spec plants the removal of each.
design-fit:
  owner: the deskrelease command /46 migrates; manifest semantics stay in the handlers
  contract: none — consumes cli-contract.md delivered by /15
  retires: [the hand-rolled argv parser in desk_manifest.py]
  weight: verbs +2 (manifest write and check, replacing a script entrypoint); flags +0 except conventional help spellings; refusals +0
  why-add: two verbs beside the release verbs keep the release's integrity helpers in one command; a separate binary would add a host-installed command for a release-step helper

## Ground rules

Feature branch and draft PR only. No release, tag, workflow dispatch or edit to a workflow
file: the script stays at its path as a transparent adapter so both release.yml copies keep
calling `python3 -B -m tools.release.desk_manifest` unchanged. Preserve the exit codes, the
three fail-closed directions of `check`, the temp-file write and the symlink and
non-regular-file refusals. A CLI library is not a new authority source. Independent
verification owns Evidence and lifecycle advancement. Planned tests and files below are
deliverables, not claims that they already exist.

## Task

1. Port `write` and `check` into `deskrelease manifest write|check <dir>`, keeping the script's output lines and exit codes. Reduce `tools/release/desk_manifest.py` to a transparent adapter that execs the verb with the same arguments, keeping `python3 -B -m tools.release.desk_manifest` working.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any directory read or write, even with absent or malformed configuration.
3. Write the golden precedence matrix BEFORE porting. The verbs declare no configuration keys unless one is needed; none may relax a check direction.
4. Add the manifest verbs' cases to TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms, using the fixture cases in cli-contract.md. Flip this brief's registry row to migrated, with `migrated_to` naming the Go entrypoint, only after they and the Python suite pass against the adapter; then reconcile the quoted usage in `tools/release/README.md` and `docs/distribution.md`.

## Verify (implementation completion; not run by this authoring change)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIInventory$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIInventory " "$f")" -eq 1` | exit 0; TestCLIInventory PASS; the row is routed to this brief with no duplicate route |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; the manifest verbs' help runs under absent and malformed config with zero directory reads or writes |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; the directory reaches the typed handler unchanged and no key relaxes a check direction |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; the two-positional forms behave as the script did, and a wrong count or unknown verb exits 2 before any read |
| 5 | check +mutation | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-82.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | exit 0; the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N the spec's mutation count; plants: help after a directory read, the unlisted-file refusal removed, the changed-file refusal removed; the command compares muhar's last line itself, because muhar exits 0 when a mutation survives or cannot be applied |
| 6 | check +flow | `python3 -B -m unittest discover -s tools/release -p desk_manifest_test.py -v` | exit 0; the script's offline suite passes against the adapter |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/82 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; the owned row is migrated and delegates to the Go entrypoint |
| 8 | check | `cd tools/desk && go test -count=1 ./cmd/deskrelease/... && GOOS=windows go vet ./cmd/deskrelease` | exit 0; the existing package stays green and compiles for Windows |

## Pre-mortem

Help touches the directory before printing: row 2. A flag or key lets a caller skip the
unlisted-file direction: rows 3 and 5. The release's module invocation stops parsing or a
usage error exits 1 instead of 2: rows 4 and 6. Only imports change while the argv parser
stays: row 7 and /17's source checks.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check that the adapter keeps both release.yml calls working without a workflow
edit, that every direction of `check` still fails closed, and that the argv parser is gone.
