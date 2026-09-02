---
brief: windows-port/02
title: Portability audit — the shell-assuming / unix-only surfaces, triaged
status: implemented
---

# Portability audit — enumerate + triage the shell-assuming surfaces

This is the map `windows-port/03` (install path), `windows-port/04` (Windows CI leg), and
`windows-port/05` (adoption-doc delta) all read. It covers the **delivery + glue layer**
only: `statusgen` and everything under `tools/**` are plain Go argv CLIs, already measured
as harness-agnostic (`docs/streams/harness-portability/README.md`, 2026-08-07) and proven to
cross-compile for Windows (`windows-port/01`) — this audit does not re-measure the binaries.

Every row below was checked against the live tree at the commit this file is added on
(`origin/main` merged in), not carried forward from an earlier survey without re-verifying.
Two of the starting facts handed to this brief needed a correction on dereference — noted
inline where they occur.

Disposition vocabulary (fixed, four tokens): **`works`** (runs on native Windows as-is) ·
**`needs-port`** (must change to run on Windows) · **`documented-workaround`** (cannot be made
native cheaply; ship a stated manual step) · **`out-of-scope`** (not part of the Windows
claim).

## Triage table

| Surface | File(s) | Assumes | Disposition | Owner | Note |
|---|---|---|---|---|---|
| SessionStart hooks | `plugins/assay/hooks/inject-resident-rules.sh`, `inject-board-state.sh`, wired via `plugins/assay/hooks/hooks.json` (`"command": "bash \"${CLAUDE_PLUGIN_ROOT}/hooks/…\""`) | `#!/bin/bash` interpreter, `jq` on PATH; both invoked as `bash "…"` from `hooks.json` | `documented-workaround` | follow-up windows-port/05 (adoption doc names the workaround) | Windows Claude Code ships no `bash`/`jq` by default. `hooks.json`'s `bash "…"` command string is the load-bearing SessionStart mechanism (delivers the resident operating rules) and is itself POSIX-only — rewriting it to a portable invocation is a plugin-host-level change, not a per-hook fix, so the cheap near-term answer is a documented prerequisite (Git-Bash/WSL provides `bash`+`jq`) rather than a port. Revisit if the plugin host ever supports a per-OS hook command. |
| Push-guard shim | `tools/desk/hooks/pre-push` (installed by `make desk-hook-install` into `.githooks/pre-push`, activated via `core.hooksPath`) | `#!/bin/sh`; `exec /opt/desk-tools/bin/deskpushguard "$@"` — a hardcoded POSIX absolute path | `needs-port` | windows-port/03 | The GUARD itself (`deskpushguard`) is compiled Go and portable — confirmed cross-compilable per `windows-port/01`. Only the SHIM (`#!/bin/sh` + the `/opt/desk-tools/bin` absolute path) is POSIX-only; git on Windows needs either a `.githooks/pre-push.cmd`/`.ps1` equivalent invoking the `.exe`, or a Git-for-Windows shell (which does ship `sh.exe`) as a documented fallback. The absolute install path is the same one the install-path class below assumes — one fix serves both. |
| Install path | `Makefile` `desk-install` target (`SHELL := /bin/sh`, `su "$build_user" -c …`, `install -o root -g 0 -m 0755 … /opt/desk-tools/bin`); `INSTALL_DIR := /opt/desk-tools/bin` | POSIX `make`, `/bin/sh`, `su`, POSIX file ownership (`-o root -g 0`), a root-owned `/opt` tree | `needs-port` | windows-port/03 | `sudo make desk-install` has no Windows analogue: no `sudo`/`su`, no `/opt`, and POSIX file-mode/ownership bits don't map to Windows ACLs. This is the surface windows-port/03's PowerShell-vs-Go-installer fork exists to replace; this audit records *why* it needs replacing, not which fork wins. |
| `deskrelease`'s desktoken path | `tools/desk/cmd/deskrelease/github.go:32` — `const deskTokenPath = "/opt/desk-tools/bin/desktoken"` | Same hardcoded `/opt/desk-tools/bin` absolute path as the install target, baked into a second binary as a Go string constant (not just the shim/Makefile) | `needs-port` | windows-port/03 | Found during this audit, not named in the brief's starting facts — the `/opt/desk-tools/bin` assumption is baked into more than the shim and the Makefile; a full port must also make this constant install-location-aware (env override or a resolved install dir) rather than a second copy of the same literal. |
| Config home | `tools/desk/internal/deskkit/rosterconfig.go:253` — `const configHomeFile = "~/.config/assay/roster.env"`; `appconfig.go:34`'s `expandHome` (only expands a leading `~/`, via `os.UserHomeDir()`) | `~/`-relative config path; no `%APPDATA%` branch in the main config-home resolver | `works` | n/a (recommendation below feeds windows-port/03 and /05) | `os.UserHomeDir()` returns `%USERPROFILE%` on Windows, so `~/.config/assay` resolves to `C:\Users\<u>\.config\assay` — it runs, it is just not the Windows-idiomatic `%APPDATA%\assay` location. Runs as-is; see the config-home recommendation below for whether to leave it or branch. |
| Config-home XDG branches | `tools/desk/cmd/writeguard/main.go:218` (`sentinelPath`) and `tools/desk/internal/deskkit/publicbless.go:78` | `$XDG_CONFIG_HOME`, falling back to `~/.config/assay/…` | `works` | n/a | **Correction to the brief's starting fact**: the brief states "one XDG branch exists only in `publicbless.go`" — dereferenced against the live tree, there are now **two** XDG-aware call sites (`writeguard/main.go` and `publicbless.go`), and both fall back to the same `~/.config/assay` path `expandHome` uses. Neither has a `%APPDATA%` branch. Functionally `works` (same reasoning as the config-home row) but the two-sites-not-one correction matters for windows-port/03/05, which should treat XDG-awareness as already-partially-present rather than absent. |
| Desk verbs that shell out (git/gh) | ~94 non-test `exec.Command("git"\|"gh", …)` call sites across `tools/desk/**` and `statusgen/**` (19 in `tools/desk` alone were sampled directly) | `git`/`gh` on PATH, invoked via argv arrays (no shell interpolation) | `works` | n/a | Both `git.exe` and `gh.exe` ship as native Windows binaries and every sampled site passes an argv slice to `exec.Command`, not a shell string — no `/bin/sh -c` wrapping, no `/`-rooted path baked into the git invocation itself. A separate, already-tracked migration (`tools/desk/scripts/count-git-exec.sh`, "desktools-go-git migration") is independently driving direct `exec.Command("git", …)` sites toward a shared `internal/gitexec` seam; that migration is orthogonal to Windows portability (it is about spawn-site consolidation, not OS assumptions) and needs no coordination here. |
| Desk verbs that shell out (`sh`/`bash`) | `tools/desk/cmd/verifyloop/verdictrun.go:42` (`shellExec`, `exec.Command("sh", "-c", command)` — runs every Verify-row command); `statusgen/mergecheck.go:316` (`execOverTree`, `exec.Command("sh", "-c", cmdline)`); `tools/desk/cmd/muhar/main.go:215` (`exec.Command("sh", "-c", s.Test)`, operator-supplied test command); `tools/desk/cmd/scanloop/monitor.go:311` (`exec.Command("/bin/bash", …)`, hardcoded interpreter path running `inbound-monitor.sh`) | A `sh`/`bash` interpreter on PATH (or, for scanloop, literally at `/bin/bash`) | `needs-port` | windows-port/04 (the CI-leg brief is the one that runs Verify rows and the inbound-monitor lane on the Windows runner) | This is the sharpest edge in the "desk verbs that shell out" class: `verifyloop`'s `shellExec` is how EVERY Verify-row command in every brief on this stream (including this brief's own table) actually executes — if `windows-port/04`'s Windows CI leg exercises `verifyloop`, `sh` must be present on the runner (GitHub's `windows-latest` ships Git-for-Windows' `sh.exe` on PATH, so this may already be `works` in practice on that specific runner — 04 should verify PATH-presence directly rather than assume; if it must be exercised outside a Git-for-Windows-provisioned shell, it is a real `needs-port` to `os/exec` + a native shell resolver). `scanloop/monitor.go`'s hardcoded `/bin/bash` is a harder case: no PATH lookup at all, so it fails even on a runner that has `bash.exe` at a different location, unless windows-port/04 changes it to look up `bash` on PATH or the underlying `inbound-monitor.sh` gets a Go rewrite. |
| Path separators | `filepath.Separator` used correctly at 13 non-test call sites (`tools/desk/cmd/writeguard/guard.go`, `deskwt/deskwt.go`, `scanloop/adapter.go`, `scanloop/lane.go`, `deskkit/migrate.go`, `internal/acp/policy.go`, `statusgen/{fixturecorpus,registerrefs,linkcheck}.go`, `tools/skillbench/main.go`); ~56 non-test sites build strings with a literal `"/" +` | Mostly `filepath.Join`/`filepath.Separator` for real filesystem paths | `works` | n/a | Sampled a representative set of the ~56 literal-`"/"` sites: every one sampled builds a semantic identifier (a `"owner/repo"` slug, a `"stream/NN"` brief id, a dispatch-ref namespace) rather than a filesystem path — those are correctly `/`-joined regardless of OS, since they are not paths. No hardcoded-`/`-as-path-separator site was found in non-test Go code; actual filesystem paths consistently go through `filepath.Join`/`os.UserHomeDir`. |
| `/tmp` | Shell scripts: `tools/create-fleet-gitlab.sh`, `tools/changelog/{aggregate_test,check_test,check,testdata/filename-only-check}.sh`, `plugins/assay/scripts/inbound-monitor.sh` all default `${TMPDIR:-/tmp}`; Go: `tools/desk/cmd/scanloop/plan.go:83` falls back to a literal `"/tmp"` when `$TMPDIR` is unset | `$TMPDIR` env var or a literal `/tmp` | `works` (Go site) / `documented-workaround` (shell sites, inherits their script's disposition) | windows-port/04 for the Go site's Windows behavior; n/a beyond the SessionStart-hooks row for the shell sites | The Go fallback (`scanloop/plan.go`) is reachable on Windows (Go sets `%TMP%`/`%TEMP%` into the process environment but not into `$TMPDIR`, so the `/tmp` literal fallback would fire) — low-risk (`filepath.Join(tmp, "assay-inbound-monitor")` still produces a syntactically valid, if non-idiomatic, path under `C:\`), but `os.TempDir()` is the idiomatic fix if `scanloop` is ever exercised natively on Windows (feeds windows-port/04's inbound-monitor-lane decision). The shell-script sites are downstream of whichever shell already gates them above — no separate disposition. |
| Container entrypoint | `containers/entrypoint.sh` (`#!/bin/sh`, shared boot for all five desk images) | A Linux container runtime | `out-of-scope` | out-of-scope (stream README: "a Windows container image" is explicitly out of scope) | Confirmed `#!/bin/sh`, POSIX-only, and deliberately so — the desk images are Linux images regardless of the host OS a human or CI runner uses. |
| `.test.sh` suites | `containers/scripts/layer-secret-scan.test.sh`, `plugins/assay/scripts/inbound-monitor.test.sh`, `plugins/assay/scripts/assay-inbox.test.sh` | POSIX shell test harnesses for POSIX shell scripts under test | `out-of-scope` | out-of-scope | Test harnesses for shell scripts that are themselves `documented-workaround` (SessionStart hooks) or otherwise out-of-scope (container). Porting the tests without porting what they test is not a real deliverable. |

## Recommendations

**Must port for the end state** (feeds `windows-port/03`'s install path and whatever
`windows-port/04`'s CI smoke touches):

- The push-guard shim (`tools/desk/hooks/pre-push`) — needs a Windows-runnable equivalent
  that git will invoke via `core.hooksPath`, pointing at wherever `deskpushguard.exe` actually
  lives.
- The install path itself (`make desk-install`'s `SHELL := /bin/sh` + `su` + `/opt` target) —
  the whole reason windows-port/03 exists.
- `deskrelease`'s `deskTokenPath` constant — the same `/opt/desk-tools/bin` literal baked into
  a second binary; must resolve the install location rather than assume it, in lockstep with
  whatever windows-port/03's fork decides.
- `verifyloop`'s `shellExec` (`sh -c`) if windows-port/04's Windows CI leg is meant to run
  actual Verify-row commands on that runner rather than only `statusgen --lint` — check
  PATH-presence of `sh` on `windows-latest` first (Git-for-Windows ships one) before assuming
  a code change is required.
- `scanloop`'s hardcoded `/bin/bash` invocation, if the inbound-monitor lane is exercised on
  Windows CI.

**Get a documented workaround** (no cheap native port; state the manual step and move on):

- The SessionStart hooks (`bash "…"` + `jq`) — the most likely candidate named in this
  brief's own facts, confirmed correct: the fix is a stated prerequisite (install Git-Bash,
  or run inside WSL for local dev only — never presented as the Windows claim itself), not a
  hook rewrite, because the load-bearing `hooks.json` command string is itself the POSIX-only
  edge and rewriting the plugin-host's hook-invocation mechanism is out of this stream's
  bound scope.

**Out of scope** (not part of the Windows claim, confirmed against the stream README):

- The container entrypoint and its Linux images (`containers/entrypoint.sh`).
- The `.test.sh` suites for shell scripts that are themselves out-of-scope or
  documented-workaround.

**Works as-is, no action needed:**

- The desk verbs' git/gh shell-outs (argv-based, no shell interpolation).
- `filepath.Separator`/`filepath.Join` usage for real filesystem paths.
- The config-home resolver's core behavior (`~/.config/assay` resolves via
  `os.UserHomeDir()` on Windows too) and both existing XDG branches.

## Config-home recommendation

Two options, decided here so `windows-port/03` and `/05` don't each re-litigate it:

- **Keep `~/.config/assay`** (i.e., `%USERPROFILE%\.config\assay` on Windows), unchanged.
  **Recommended.** Consequence: zero code change, one config-home path across all three
  platforms (documented once in the adoption doc), at the cost of not matching native Windows
  convention (`%APPDATA%`) — but `%USERPROFILE%\.config\` is also what Git-for-Windows, many
  Node/Python CLIs, and WSL-adjacent tooling already use on Windows, so it will not read as
  foreign to the audience most likely to be running a CLI-first Windows install of this
  toolkit.
- **Add a `%APPDATA%` branch** (mirroring the two existing `XDG_CONFIG_HOME` branches, adding
  a Windows-only `os.Getenv("APPDATA")` check ahead of the `~/.config` fallback in both
  `expandHome`/`ConfigHomePath` and the two XDG call sites). Consequence: matches native
  Windows convention, but adds a second config-home location that windows-port/05's adopter
  doc, every future support/debug instruction, and any human manually inspecting
  `~/.config/assay` on a Windows box must all know to check — a real ongoing cost, not a
  one-time one, for a config path a user only ever touches by following the adoption doc in
  the first place.

Recommendation: **keep `~/.config/assay`** — the win from matching `%APPDATA%` is small
(nothing here is a GUI app that Windows Explorer surfaces to an end user) and the ongoing
two-location cost is real. `windows-port/03` and `/05` should treat this as decided rather
than reopen it, unless the human gate in `windows-port/03` explicitly overrides it.
