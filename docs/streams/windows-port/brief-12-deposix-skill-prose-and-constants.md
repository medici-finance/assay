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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -n -e 'mktemp' -e '/tmp/' -e '~/.config' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/worker-desk/SKILL.md \| grep -vc 'desk-shell.md' \|\| true` | fail exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '## Scratch files' -e '## Config home' plugins/assay/references/desk-shell.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `deskboard actions --help 2>&1 \| grep -c -- '--out'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test -count=1 -run 'TestHookInstallUnix' ./cmd/deskpushguard/` | pass exit=0 | sha256:f2295dec3832 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOOS=windows go test -count=1 -run 'TestHookInstallWindows' ./cmd/deskpushguard/` | fail exit=1 | sha256:9ddc259f3fc4 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `git grep -n '"/opt/desk-tools' HEAD -- 'tools/desk/**/*.go' \| grep -vc _test.go \|\| true` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/skillslint && go test -count=1 -run 'TestPosixTokenRow' ./...` | pass exit=0 | sha256:86ac6c9b04ad | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 8 | `deskboard actions --out /tmp/a.json && reviewloop plan --actions /tmp/a.json --dry-run; echo rc=$?` | pass exit=0 | sha256:e14269118d70 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers windows-port/12; echo $?` | pass exit=0 | sha256:511beaa45d1c | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --lint` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test -count=1 -run 'TestHookInstallIdempotentAndForeignRefusal' ./cmd/deskpushguard/` | pass exit=0 | sha256:6306ade6e327 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

**Verifier notes — 2026-09-27, assay-verifier-app[bot] (dispatched non-implementer, claude-opus-5-5), merged main 9585b4b6cc2e, darwin host.** Implementing change: #1501 (squash 34e93fbd6).

Per-row observed output (hand re-runs of the exact row commands, same host and head):

- Row 1 — prints `0` (exit 0). Expect `0` met. The witness `fail` is a witness-parse artefact: the Expect cell's parenthetical ("grep -v's exit 1 on zero matches") was read as an expected exit status of 1. The single surviving raw hit (worker-desk SKILL.md line 860, the config-home line) carries the desk-shell.md mechanism reference, exactly as the brief's rule intends. Substantively PASS; check-definition wording defect in the Expect cell.
- Row 2 — prints `2`. PASS.
- Row 3 — prints `3` (three `--out` lines in the installed deskboard's usage; a build of deskboard from this head prints the same `3`). `--out` was ADDED by #1501 and is documented in the verb usage (answers Review question 3). PASS.
- Row 4 — `--- PASS: TestHookInstallUnix`. PASS.
- Row 5 — could-not-check on this host. `GOOS=windows go test` builds a Windows test binary and then cannot exec it on darwin: `fork/exec …deskpushguard.test.exe: exec format error`, FAIL exit 1. That is the environment, not the code: `GOOS=windows go test -c` of the package builds clean (exit 0), and `TestHookInstallWindows` is target-parameterised (writeHooks takes the target OS), so on the host it runs and prints `--- PASS: TestHookInstallWindows`. The row's alternative ("run on the Windows CI leg") has no instrument today: the Windows CI leg workflow runs only the statusgen lint/version smoke and the bootstrap hash-check, not the deskpushguard tests. Native-Windows execution of this row remains open.
- Row 6 — prints `1`; the sole non-test hit is cellctl's `DESK_TOOLS_BIN`-overridable default, as the Expect cell names; deskrelease carries none. PASS.
- Row 7 — `--- PASS: TestPosixTokenRow` (plus the two sibling posix-token tests). PASS.
- Row 8 — could-not-check. The exact row command prints `rc=6`, not `rc=0`: `deskboard actions` fails closed on an open-PR read of one private watched repo (`Resource not accessible by integration` under the verifier App identity), so no actions file is written and `reviewloop plan` is never reached. The witness `pass` is vacuous — the row ends in `; echo rc=$?`, so its exit status is always 0 and an exit-status witness cannot see the printed rc (check-definition defect). Partial evidence on the flow: `reviewloop plan --actions <file> --dry-run` parses the flag (a bogus flag is refused with "flag provided but not defined"; `--dry-run` reaches payload decoding). The full flow needs an identity with open-PR read on every watched repo.
- Row 9 — exit 0; summary "0 corroborated, 0 disproved, 6 unchecked". PASS (exit-status row).
- Row 10 — the witness could not run it (check:ci needs a network-off Linux sandbox). Host run of the same command: exit 0, `LINT: PASS`, 0 PROBLEM lines. Recorded as a host observation, not the network-off check:ci run.
- Row 11 — `--- PASS: TestHookInstallIdempotentAndForeignRefusal`. PASS.

Risk-bearing values. Trigger: the diff touches a risk-classed path (tools/desk/cmd/deskpushguard; statusgen lint raises a `risk-files-crossread` NOTICE on this brief for it) and changes an authority binding (the identity-mint binary deskrelease executes). Enumeration over the #1501 diff (hookinstall.go, deskrelease/github.go, deskboard/main.go, reviewloop, skillslint/posixtoken.go):

- `deskTokenPath` resolution (authority binding) — deskrelease/github.go:69-84: sibling `desktoken`/`desktoken.exe` next to os.Executable() first, then exec.LookPath, then the bare name. Replaces the removed `/opt/desk-tools/bin/desktoken` literal.
- `name = "desktoken"` / `"desktoken.exe"` @ deskrelease/github.go:70-72.
- `hookMarker = "deskpushguard"` @ deskpushguard/hookinstall.go:66 (decides clobber vs idempotent skip).
- `unixShim` exec target `"$(command -v deskpushguard)"` @ deskpushguard/hookinstall.go:49; `windowsCmdShim` target `deskpushguard.exe` @ deskpushguard/hookinstall.go:59.
- File modes `0o755` (dir and pre-push) @ hookinstall.go:79,84; `0o644` (pre-push.cmd) @ hookinstall.go:94.
- `posixTokenPattern = mktemp|/tmp/|~/\.config` @ skillslint/posixtoken.go:34; `unixExampleFenceMarker = "unix"` @ posixtoken.go:42.

Ranked: (1) the deskTokenPath binding — a wrong binary would receive the identity-mint call during a release; a leaked credential cannot be undone by a redeploy. (2) hookMarker — decides whether an existing hook is overwritten; a wrong value would clobber or silently skip a guard. (3) shim exec targets — wrong target means the push guard does not run. Modes and the lint pattern are reversible, advisory knobs and rank last.

- RISK-VALUE: DERIVED — deskTokenPath = sibling-of-os.Executable() "desktoken" @ deskrelease/github.go:74-79 — both shipped installers put every tools/desk/cmd binary into one directory in one step (Makefile desk-install loops over DESK_CMDS into INSTALL_DIR; build-windows.ps1 enumerates every cmd directory), so the sibling is the installed desktoken, and substituting it needs control of the directory that already holds the running deskrelease, the same threat model the old absolute literal defended.
- RISK-VALUE: NAMED, NOT DERIVED — deskTokenPath fallback = exec.LookPath("desktoken") @ deskrelease/github.go:80-82 — when no sibling exists (dev builds, or a relocated binary) the identity mint resolves from PATH, which the removed comment said the literal existed to prevent. The code comment documents this as a deliberate dev-workflow fallback, but the portability-audit row's claim of "no widening" of that property holds only in the installed case. Whether a PATH fallback is acceptable for a privileged, identity-minting maintainer tool (versus refusing when no sibling exists) is a policy call this verifier cannot derive. Open question for a human.
- RISK-VALUE: DERIVED — hookMarker = "deskpushguard" @ deskpushguard/hookinstall.go:66 — both the new shims and the older committed pre-push shim contain the tool name, so a re-run is an idempotent skip and any hook without it is refused unless --force is given (row 11 proves the refusal and the --force overwrite). Observation: the marker is a plain substring, so a foreign hook that merely mentions the name (for example a comment) is also reported as installed, and an older `/opt`-exec shim is left in place instead of being upgraded to the PATH-resolving one. Advisory, not a row failure.
- RISK-VALUE: DERIVED — unixShim exec target = "$(command -v deskpushguard)" @ deskpushguard/hookinstall.go:49 — the brief's fact requires PATH resolution with no `/opt` literal; with deskpushguard missing from PATH, `exec ""` fails non-zero and git aborts the push (fail-closed). The .cmd pair's `deskpushguard.exe %*` likewise fails non-zero (errorlevel 9009) when the binary is absent.

Observations: (a) the brief answers all four risk questions "no" but touches a security-path trigger (the lint NOTICE above); per the verifier kit a risk-classed path means a model does not sign this off alone, so the desk should route that question. (b) Rows 1, 5 and 8 have check-definition defects (Expect parse, no Windows instrument, vacuous exit-status witness) that should be re-authored before the next re-verify.

VERIFY: BLOCKED — rows 5 and 8 could-not-check (row 5 needs a native-Windows run; row 8 needs a board-reading identity with open-PR read on every watched repo); every row that ran on this host met its Expect on observed output; one risk-bearing value is NAMED, NOT DERIVED (the deskrelease PATH fallback).

## Review
Gate: **model**. Reviewer's questions: (1) is the prose diff a pure how-rename — same steps, same
order? (2) does the unix hook still work from a linked worktree (core.hooksPath resolution)?
(3) row 3 — was `--out` added or already there, and is it documented in the verb's usage?
