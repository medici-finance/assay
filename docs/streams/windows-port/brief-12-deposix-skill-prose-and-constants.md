---
brief: assay:assay:windows-port:12
title: De-POSIX the desk-role skill prose, and close the two needs-port constants the install brief left behind
why: >-
  The desk roles are driven by prose the agent follows literally, and that prose still says
  `> /tmp/actions.json`, "mint the file per invocation (mktemp)", and `~/.config/assay/HEARTBEAT`
  as if every adopter had a POSIX shell. On native Windows the agent either fails the step or
  improvises — and improvisation in a role procedure is exactly what the skills exist to prevent.
  Two code constants brief 03 was meant to retire also survived: the push-guard shim is still
  `#!/bin/sh` exec-ing an absolute `/opt` path, and the release tool hard-codes
  `/opt/desk-tools/bin/desktoken`. Small, mechanical, and the last of the "audit said needs-port"
  rows that nothing else owns.
wave: 4
depends: ["windows-port/02", "windows-port/03"]
unblocks: ["windows-port/14"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-21 by the-desk (Bob) — windows-port authoring session, driver ask 2026-09-21
sources:
  - "plugins/assay/skills/pr-review-desk/SKILL.md:288-290 — `deskboard actions > /tmp/actions.json` … `reviewloop plan --actions /tmp/actions.json --prs /tmp/prs.json` (a /tmp path + a shell redirect, then read back by a verb)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md:191 — 'Mint the file per invocation (`mktemp`)'"
  - "plugins/assay/skills/worker-desk/SKILL.md:452 — 'per-invocation `mktemp` body files'; :794 — `~/.config/assay/HEARTBEAT` (resolves on Windows via the home dir; the prose should say the config-home rule brief 02 ruled, not a literal)"
  - "tools/desk/hooks/pre-push — `#!/bin/sh` + `exec /opt/desk-tools/bin/deskpushguard \"$@\"` (Git-for-Windows runs sh hooks, the /opt target does not exist there); docs/streams/windows-port/portability-audit.md row 'pre-push shim' = needs-port/03"
  - "tools/desk/cmd/deskrelease/github.go:32 `const deskTokenPath = \"/opt/desk-tools/bin/desktoken\"` — audit row needs-port/03; maintainer-only tool, low adopter reach, still a literal"
  - "docs/streams/windows-port/brief-03-install-path.md (implemented) — shipped the installer; the two constants above are what it did not touch"
  - "plugins/assay/references/desk-shell.md — the neutral shell-mechanics reference the skill bodies defer to; the right home for a 'temp file / scratch path' mechanism statement so each skill says 'a per-invocation scratch file (desk-shell.md §scratch)' instead of `mktemp`"
  - "freshness-checked 2026-09-21 @ 56491ce (origin/main): all cited lines present as quoted"
exec-tier: any
domain: clear
consumers:
  - "plugins/assay/skills/pr-review-desk/SKILL.md, pr-shepherd/SKILL.md, worker-desk/SKILL.md (prose sites): follow-up windows-port/12 (this brief)"
  - "plugins/assay/references/desk-shell.md (new §scratch files + §config home): follow-up windows-port/12 (this brief)"
  - "tools/desk/hooks/pre-push (+ new pre-push.cmd or the Go `deskpushguard --hook-install` path that writes a platform-correct shim): follow-up windows-port/12 (this brief)"
  - "tools/desk/cmd/deskrelease/github.go:32 (resolve desktoken from PATH via exec.LookPath, fall back to the install dir the installer records): follow-up windows-port/12 (this brief)"
  - "docs/streams/windows-port/portability-audit.md (flip the two rows to works): follow-up windows-port/12 (this brief)"
  - ".claude/skills/author-brief/SKILL.md house copies in adopter repos: out-of-scope (the bundle re-syncs downstream per SYNC-FROM)"
version: 1
id: 68b6b898-f0a8-44c8-9b3a-bfd572e2cbb6
---

# Brief 12 — De-POSIX the skill prose; close the two constants

## Context
files: plugins/assay/skills/pr-review-desk/SKILL.md, plugins/assay/skills/pr-shepherd/SKILL.md, plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/references/desk-shell.md, tools/desk/hooks/pre-push, tools/desk/cmd/deskpushguard (hook-install subcommand), tools/desk/cmd/deskrelease/github.go, docs/streams/windows-port/portability-audit.md, changelog/windows-port-12-deposix.md
facts:
- prose rule: a skill body names a MECHANISM in desk-shell.md, never a POSIX command — `mktemp` → "a per-invocation scratch file (desk-shell.md §Scratch files)"; `> /tmp/x.json` → "write the JSON to a scratch file and pass its path" with the verb's own `--out <path>` if it has one (deskboard actions: check whether `--out` exists; if not, this brief adds it — one flag, documented); `~/.config/assay/…` → "the config home (desk-shell.md §Config home: `os.UserConfigDir()/assay`)"
- desk-shell.md gains two short sections: §Scratch files (per-invocation, private, cleaned; unix `mktemp`, PowerShell `New-TemporaryFile`, Go `os.CreateTemp`) and §Config home (the brief-02 ruling: `~/.config/assay` on unix = `%USERPROFILE%\.config\assay` on Windows)
- pre-push: keep the sh shim for unix; add `deskpushguard hook-install` that writes the platform-correct shim into `.githooks/` (sh on unix; `pre-push` + `pre-push.cmd` pair on Windows, the .cmd calling `deskpushguard.exe` from PATH) — `make desk-install` and the PowerShell installer both call it; the audit row flips to works
- deskrelease: `exec.LookPath("desktoken")` first, then the recorded install dir (the installer writes it; brief 03/06), never a bare `/opt` literal
single-point-of-failure: for the prose half the control is the skillslint PARITY/lint pass (a POSIX token that slips back in is caught by a new `posix-token` lint row: `mktemp|/tmp/|~/.config` in a skill body outside a fenced "unix example" block); for the hook half it is the hook-install test on both GOOS.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Prose edits change MECHANISM references only — no procedure step is added, removed or
  reordered (the diff must be a rename of the how, not the what).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. desk-shell.md §Scratch files + §Config home.
2. Edit the five prose sites to name the mechanism; add `deskboard actions --out` if absent.
3. `deskpushguard hook-install` + the Windows .cmd shim; wire into both installers.
4. deskrelease desktoken resolution.
5. skillslint `posix-token` row (advisory first, per the lint-debt cadence).
6. Flip the two audit rows; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -n -e 'mktemp' -e '/tmp/' -e '~/.config' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/worker-desk/SKILL.md \| grep -vc 'desk-shell.md' \|\| true` | `0` — every surviving mention points at the mechanism section (`\|\| true` neutralises `grep -v`'s exit 1 on zero matches, per statusgen lint's grep-zero-count NOTICE) | `check` |
| 2 | `grep -c -e '## Scratch files' -e '## Config home' plugins/assay/references/desk-shell.md` | `2` | `check` |
| 3 | **DEREFERENCE — the documented flag exists**: `deskboard actions --help 2>&1 \| grep -c -- '--out'` | `>= 1` | `check +dereference` |
| 4 | Hook install on unix: `cd tools/desk && go test -count=1 -run 'TestHookInstallUnix' ./cmd/deskpushguard/` | PASS — writes `pre-push` with `#!/bin/sh` and no `/opt` literal (uses `command -v deskpushguard`) | `check` |
| 5 | Hook install on windows: `cd tools/desk && GOOS=windows go test -count=1 -run 'TestHookInstallWindows' ./cmd/deskpushguard/` (or the cross-compiled test binary run on the Windows CI leg) | PASS — writes the `pre-push.cmd` pair | `check` |
| 6 | `git grep -n '"/opt/desk-tools' HEAD -- 'tools/desk/**/*.go' \| grep -vc _test.go \|\| true` | `1` — the sole remaining hit is `tools/desk/cmd/cellctl/cell.go`'s `DESK_TOOLS_BIN`-overridable default for the Linux-only cell tooling (not `deskrelease`, and not named in this brief's `files:` — cellctl only ever targets the Linux desk-container images the portability audit already rules out-of-scope). `deskrelease`'s own occurrence (the one this brief's facts name) is the one this row closes to zero | `check` |
| 7 | skillslint row fires on a planted token: `cd tools/skillslint && go test -count=1 -run 'TestPosixTokenRow' ./...` | PASS (fail-first fixture) | `check` |
| 8 | **Flow — the prose still drives the verb**: follow pr-review-desk SKILL.md's rewritten step on this machine: `deskboard actions --out /tmp/a.json && reviewloop plan --actions /tmp/a.json --dry-run; echo rc=$?` | `rc=0` | `check +flow` |
| 9 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/12; echo $?` | `0` | `check` |
| 10 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |
| 11 | **Mutation — the foreign-hook control reddens**: `cd tools/desk && go test -count=1 -run 'TestHookInstallIdempotentAndForeignRefusal' ./cmd/deskpushguard/` | PASS — the test plants a foreign (non-`deskpushguard`) `pre-push` hook and asserts `writeHooks` REFUSES to overwrite it without `--force`, then asserts it succeeds and overwrites WITH `--force`; a `hook-install` that silently clobbered a foreign hook would redden this row | `check +mutation` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — VERIFY: BLOCKED — 7/11 pass, 4 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | grep -n -e 'mktemp' -e '/tmp/' -e '~/.config' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/worker-desk/SKILL.md \| grep -vc 'desk-shell.md' \|\| true | 0 — every surviving mention points at the mechanism section | PASS — exit 0; output `0`; no non-desk-shell hit in any of the three bodies | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | grep -c -e '## Scratch files' -e '## Config home' plugins/assay/references/desk-shell.md | 2 | PASS — exit 0; output `2` — both sections present | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | deskboard actions --help 2>&1 \| grep -c -- '--out' | >= 1 | PASS — exit 0; output `3`; help text documents --out at deskboard/main.go:211 (built from merged-main source). Note: the installed shim binary on PATH is stale and returns 0. | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | cd tools/desk && go test -count=1 -run 'TestHookInstallUnix' ./cmd/deskpushguard/ | PASS — writes pre-push with #!/bin/sh, no /opt literal | PASS — exit 0; `ok ...deskpushguard 0.386s` | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | cd tools/desk && GOOS=windows go test -count=1 -run 'TestHookInstallWindows' ./cmd/deskpushguard/ | PASS — writes the pre-push.cmd pair | COULD-NOT-CHECK — environment: a Windows PE test binary cannot exec on darwin (`exec format error`). The binary cross-compiles AND vets clean (GOOS=windows go test -c rc 0; go vet rc 0); designed to run on the Windows CI leg per the brief. Corroborated by rows 4 and 11, which pass and exercise the shared writeHooks path. | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | git grep -n '"/opt/desk-tools' HEAD -- 'tools/desk/**/*.go' \| grep -vc _test.go \|\| true | 1 — sole hit is cellctl's DESK_TOOLS_BIN default | PASS — exit 0; output `1`; the one hit is tools/desk/cmd/cellctl/cell.go:259 (the sanctioned out-of-scope survivor). deskrelease github.go carries /opt only in comments, no code literal | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | cd tools/skillslint && go test -count=1 -run 'TestPosixTokenRow' ./... | PASS (fail-first fixture) | PASS — exit 0; `ok ...skillslint 0.221s` | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | deskboard actions --out /tmp/a.json && reviewloop plan --actions /tmp/a.json --dry-run; echo rc=$? | rc=0 | COULD-NOT-CHECK — environment: the offline envelope forbids `deskboard actions` as a live composed run (it enumerates open PRs across the repo set = live GitHub). Both halves proven OFFLINE: deskboard actions unit tests pass (go test -run Action ./cmd/deskboard rc 0) and reviewloop plan --actions/--dry-run tests pass (go test -run Plan ./cmd/reviewloop rc 0). | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | statusgen --root . --consumers windows-port/12; echo $? | 0 | COULD-NOT-CHECK — merged tree; exit 0 but the instrument checked nothing: "no brief files in the diff against 50989dbc... — nothing to corroborate". On merged main the brief's own diff is empty, so no consumers entry was corroborated; the exit code alone does not meet "Consumers routing corroborated". Same condition as desk-supervision/13 row 10, contributor-trust/04 row 12 and graph-execution/07 row 8. Not a pass, not a fail. | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | statusgen --root . --lint | 0 PROBLEMs | COULD-NOT-CHECK — hermetic witness owed (darwin): the check:ci hermetic re-exec needs Linux unshare --net; host is darwin. Direct non-hermetic run: exit 0, `LINT: PASS`, PROBLEM count 0 (501 advisory NOTICEs, not gating). | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | cd tools/desk && go test -count=1 -run 'TestHookInstall.*AndForeignRefusal' ./cmd/deskpushguard/ | PASS — refuses foreign hook without --force, overwrites with --force | PASS — exit 0; `ok ...deskpushguard 0.355s` (green run 2026-09-23, claude-opus-4-8-verifier) (`go test -list 'TestHookInstall.*AndForeignRefusal'` selects exactly the one test this row names — the idempotent-and-foreign-refusal hook-install test.) MUTATION half, run 2026-09-25 at merge commit e7a314b8 (hookinstall.go md5 b00bdbb373c20c6271dceb310c961dec, byte-identical to 50989dbc): BASELINE exit 0 `ok`. MUTATION: in writeShimIfClear (hookinstall.go line 160) changed "if !force {" to "if false && !force {", so a foreign hook is silently clobbered; the same command (with -v) REDDENED, exit 1: "hookinstall_test.go:127: writeHooks over a foreign hook without --force should refuse, got no error". Restored with git checkout; md5 b00bdbb373c20c6271dceb310c961dec again (byte-identical), git status clean; re-run exit 0 `ok`. | 2026-09-25 | claude-opus-5-5-verifier |

Rows observed passing: 7/11 (1,2,3,4,6,7,11). Could-not-check: 4 (5,8,9,10) — rows 5 and 8 are
environment-limited (Windows exec on darwin; offline envelope) each corroborated by an executed
offline proof, row 9 is the consumers instrument on merged main where the brief's diff is empty (it
checked nothing), and row 10 is a check:ci hermetic re-exec that needs a Linux runner though its
direct run passed; none observed failing. Row 11's mutation half is recorded: the foreign-hook
refusal reddens when removed (mutation run 2026-09-25 by claude-opus-5-5-verifier).

**Risk-bearing value (kit §4 — enumerate → rank → derive).**

Enumerated every literal this item's diff introduces or changes:
- `name = "desktoken"` (and `"desktoken.exe"` on Windows) @ tools/desk/cmd/deskrelease/github.go:70,72 — the binary filename `resolveDeskTokenPath` resolves (co-located dir then PATH). This REPLACES the removed `const deskTokenPath = "/opt/desk-tools/bin/desktoken"`.
- `"/opt/desk-tools/bin"` @ tools/desk/cmd/cellctl/cell.go:259 — the DESK_TOOLS_BIN default. Out of scope, unchanged by this brief, sanctioned survivor of Verify row 6.
- skillslint posix-token pattern `mktemp|/tmp/|~/.config` — advisory lint pattern literal.
- desk-shell §Config home rule `os.UserConfigDir()/assay` — a path-resolution rule, not a numeric constant.

Ranked by irreversibility: all four are reversible by edit + redeploy; none is an irreversible
financial / settlement / authority-binding value. Item metadata: regulatory/customer/irreversible/
sensitive-data all no, gate:model. Top-ranked is the desktoken resolution (maintainer-only release
tool; fail-closed on miss, pinned by TestTokenMintFailureFailsClosed).

RISK-VALUE: DERIVED — name = "desktoken" @ tools/desk/cmd/deskrelease/github.go:70 — the resolved
binary is the identity-mint tool this repo ships under exactly that name; resolution is
co-located-directory-then-PATH and fail-closed when neither yields it, so a wrong value cannot
silently mint against the wrong binary and is reversible by edit + redeploy (irreversible=no). The
other enumerated entries are reversible path / pattern / rule literals, none irreversible; N/A does
not apply because enumeration was non-empty.


## Review
Gate: **model**. Reviewer's questions: (1) is the prose diff a pure how-rename — same steps, same
order? (2) does the unix hook still work from a linked worktree (core.hooksPath resolution)?
(3) row 3 — was `--out` added or already there, and is it documented in the verb's usage?
