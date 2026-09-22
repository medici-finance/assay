---
brief: assay:assay:windows-port:11
title: Portable desk-role pollers — inbound + PR monitors and the tick emitter as Go verbs; scanloop arms a binary, not /bin/bash
why: >-
  A native-Windows adopter who installs the pinned release and opens a desk role has no inbound
  surface: the intake desk's monitor is a 362-line bash script that scanloop arms by executing a
  hard-coded `/bin/bash`, the review desk's PR monitor is its 374-line twin armed the same way
  through the harness Monitor tool, and every role's tick summary is a bash emitter that shells
  to make and sed. Briefs 00–05 made the binaries run on Windows; these three scripts are the
  glue that still does not, and the portability audit (brief 02) listed only one of them, for its
  /tmp literal. This brief ports the three to Go verbs the release already ships, keeps the bash
  scripts as the parity oracle until the verbs prove identical, and makes scanloop arm the verb.
wave: 4
depends: ["windows-port/00", "windows-port/01"]
unblocks: ["windows-port/14"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-21 by the-desk (Bob) — windows-port authoring session, driver ask 2026-09-21 ("create the windows-port/11 brief; what other things do we need for the windows port? I hate these 1-by-1's")
sources:
  - "medici-finance/assay#1435 — scanloop: inbound monitor is a bash script armed via hard-coded /bin/bash — no native-Windows inbound surface"
  - "tools/desk/cmd/scanloop/monitor.go:311 `exec.Command(\"/bin/bash\", append([]string{script}, args...)...)` and the file header's argument that a Go re-implementation would re-acquire three bugs (explicit identity, per-repo retained state, burst collapse) — the three properties the port MUST carry, stated as its acceptance tests"
  - "plugins/assay/scripts/inbound-monitor.sh (362 lines, #!/usr/bin/env bash; gh×20, jq, sed, sort, comm, mktemp×3; ~35 bashism sites) — the inbound poller; its header §A/§B/§C is the spec"
  - "plugins/assay/scripts/pr-monitor.sh (374 lines, same shape) — the review desk's twin, armed by the harness Monitor per plugins/assay/skills/pr-review-desk/SKILL.md:83 ('do NOT hand-write the poll'); NOT enumerated by brief 02's audit"
  - "plugins/assay/scripts/tick-summary.sh (184 lines; make, sed, grep) — the one executable form of the tick grammar (plugins/assay/references/tick-contract.md:166-170); every desk role in tick mode calls it"
  - "docs/streams/windows-port/portability-audit.md (brief 02, done) — names inbound-monitor.sh once, for its /tmp literal; pr-monitor.sh, tick-summary.sh, and the harness-Monitor invocation path are absent from its table (the audit gap this brief closes in Task 1)"
  - "tools/desk/cmd/scanloop/plan.go:83 `tmp := os.Getenv(\"TMPDIR\")` with a `/tmp` fallback — replaced by os.TempDir() in passing"
  - "freshness-checked 2026-09-21 @ 56491ce (origin/main): monitor.go:311 unchanged; the three scripts present at the cited line counts; no Go verb named deskmonitor/desktick exists"
exec-tier: strong
exec-tier-why: >-
  Question (b): three scripts become three verbs whose output MUST be byte-identical to the
  scripts' on the same forge state (the parity oracle), and scanloop's parser (monitor.go
  ParseMonitorOutput) reads that output; Question (c): the retained-baseline property (§B) is
  exactly the kind of state bug that survives a happy-path test.
domain: complicated
consumers:
  - "tools/desk/cmd/deskmonitor/** (new verb: `deskmonitor inbound|pr`): follow-up windows-port/11 (this brief)"
  - "tools/desk/cmd/desktick/** (new verb: the tick-summary grammar): follow-up windows-port/11 (this brief)"
  - "tools/desk/cmd/scanloop/monitor.go (arms `deskmonitor inbound` via the seam; FindMonitorScript keeps the .sh search for parity mode only): follow-up windows-port/11 (this brief; the bounded-exec seam side is code-risk/06 on the house tracker)"
  - "plugins/assay/scripts/{inbound-monitor,pr-monitor,tick-summary}.sh: follow-up windows-port/11 (this brief — kept as parity oracles; retirement is windows-port/14's call after the CI leg proves parity)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md:83 and intake-desk/SKILL.md:159, references/tick-contract.md:166-170 (name the verb first, the script as fallback): follow-up windows-port/12"
  - "docs/streams/windows-port/portability-audit.md (add the missed rows): follow-up windows-port/11 (this brief, Task 1)"
  - "plugins/assay/paired-versions.yaml / release matrix: out-of-scope (new verbs ride the existing desk-tools tarball; no new artifact)"
version: 1
id: f032490f-5170-4cb9-95f7-f97cfab1f592
---

# Brief 11 — Portable desk-role pollers

## Context
files: tools/desk/cmd/deskmonitor/{main.go,inbound.go,pr.go,state.go,parity_test.go}, tools/desk/cmd/desktick/{main.go,grammar.go,grammar_test.go}, tools/desk/cmd/scanloop/monitor.go (+ monitor_test.go), tools/desk/cmd/scanloop/plan.go, docs/streams/windows-port/portability-audit.md, changelog/windows-port-11-pollers.md
facts:
- the three properties the header of monitor.go says a Go port loses — each becomes a NAMED test: (A) explicit identity: the verb unsets GH_TOKEN/GITHUB_TOKEN from its own environment and reads with the keyring/roster identity the script uses today (inbound-monitor.sh top of file); (B) per-repo retained state: one state file per repo under `INBOUND_MONITOR_STATE_DIR` (same env, same file names, same line format as the script — scanloop's arming evidence reads them); a read that fails, returns zero-after-nonzero, comes back AT --limit, or collapses below the retain floor RETAINS the baseline and emits the same `MONITOR-DEGRADED <repo>` line; (C) burst collapse: a mass update collapses to one burst line, same text
- output grammar: byte-identical to the scripts' stdout on the same forge snapshot — scanloop's `ParseMonitorOutput` (monitor.go) is the consumer and is NOT changed; parity test = run script and verb against a recorded fixture of forge responses (httptest server replaying `gh api` JSON) and diff stdout
- `desktick`: implements `../../scripts/tick-summary.sh`'s grammar (tick-contract.md §grammar; `tick-summary.sh regexp` prints the published ERE — the verb prints the same regexp, and a test asserts the two strings are equal while the script is kept)
- scanloop: `execMonitor` arms `deskmonitor inbound` (a desk verb on PATH, resolved like every other verb scanloop already calls at lane.go:117-125 — literal argv per binary); `--monitor <path.sh>` and `ASSAY_INBOUND_MONITOR` remain as an explicit PARITY mode that arms the script through `bash` resolved from PATH (`exec.LookPath`), never `/bin/bash`; absent bash in parity mode → Unverifiable naming the mode
- external commands the scripts use and the verbs replace in-process: gh (→ the forge client already used by deskboard), jq (→ encoding/json), sort/comm (→ sort.Strings + set diff), mktemp (→ os.CreateTemp), sed/grep (→ strings/regexp), sleep (→ time)
- Windows: state dir default = `os.UserConfigDir()/assay/monitor` when INBOUND_MONITOR_STATE_DIR is unset (the script's default is under ~/.config/assay — same resolved place on unix)
single-point-of-failure: the ONE control is the parity test (verb ≡ script on recorded fixtures). Independent layer: windows-port/14 runs the verb on the Windows CI leg against a live read (a different signal, a different runner) — the parity fixtures could be stale; the live leg cannot be.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do NOT delete or edit the three .sh scripts in this brief (parity oracle; windows-port/14
  retires them if it can).
- Do NOT change `ParseMonitorOutput`'s grammar; the verb conforms to the parser, not the reverse.
- The verb never inherits a role token: test (A) is the negative-path proof.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Close the audit gap first:** add rows to `portability-audit.md` for pr-monitor.sh,
   tick-summary.sh, the harness-Monitor invocation path (the .sh armed directly by the harness
   on the adopter's box), and the `jq` prerequisite that no bootstrap installs — each with the
   disposition this brief gives it (needs-port → this brief) so the audit is complete before the
   port starts.
2. `deskmonitor inbound` and `deskmonitor pr`: port the two scripts per `facts:` with the three
   named property tests plus the parity test on recorded fixtures.
3. `desktick`: the tick grammar + `regexp` subcommand; equality test against the script's
   `regexp` output.
4. scanloop: arm the verb; parity mode via LookPath("bash"); plan.go:83 → `os.TempDir()`.
5. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go vet ./cmd/deskmonitor/ ./cmd/desktick/ ./cmd/scanloop/ && go test -count=1 ./cmd/deskmonitor/ ./cmd/desktick/ ./cmd/scanloop/` | exit 0 | `check` |
| 2 | **Parity (the oracle)**: `cd tools/desk && go test -count=1 -run 'TestParityInboundMonitor' -v ./cmd/deskmonitor/ \| grep -c -- '--- PASS'` | `>= 1` — verb stdout equals script stdout on every recorded fixture (requires bash on the runner; records could-not-check on a bash-less runner with that reason) | `check +dereference` |
| 3 | **Negative (A) — identity**: `cd tools/desk && GH_TOKEN=leak go test -count=1 -run 'TestMonitorDropsRoleToken' ./cmd/deskmonitor/` | PASS — the forge client the verb builds carries no `leak` credential | `check` |
| 4 | **Negative (B) — retention**: `cd tools/desk && go test -count=1 -run 'TestMonitorRetainsOnFailedRead' ./cmd/deskmonitor/` | PASS — a 404/zero/at-limit/collapsed read keeps the baseline and prints `MONITOR-DEGRADED` | `check` |
| 5 | **Negative (C) — burst**: `cd tools/desk && go test -count=1 -run 'TestMonitorBurstCollapse' ./cmd/deskmonitor/` | PASS | `check` |
| 6 | Tick grammar equality: `bash plugins/assay/scripts/tick-summary.sh regexp > /tmp/a.txt; desktick regexp > /tmp/b.txt; diff /tmp/a.txt /tmp/b.txt; echo rc=$?` | `rc=0` | `check +dereference` |
| 7 | No hard-coded interpreter: `git grep -n '"/bin/bash"' HEAD -- tools/desk/cmd/scanloop/ \| wc -l` | `0` | `check` |
| 8 | Windows build of the three verbs: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskmonitor/ ./cmd/desktick/ ./cmd/scanloop/` | exit 0 | `check` |
| 9 | **Flow — scanloop arms the verb**: `cd tools/desk && go test -count=1 -run 'TestScanloopArmsDeskmonitor' ./cmd/scanloop/` | PASS — `execMonitor` argv[0] is `deskmonitor`, and the state dir it passes is read back as arming evidence | `check +flow` |
| 10 | Audit rows added: `grep -c -e 'pr-monitor.sh' -e 'tick-summary.sh' -e 'harness Monitor' -e 'jq' docs/streams/windows-port/portability-audit.md` | `>= 4` | `check` |
| 11 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/11; echo $?` | `0` | `check` |
| 12 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: **model**. Reviewer's questions: (1) row 2 — do the fixtures cover the four degraded
cases of §B, or only the happy poll? (2) did `ParseMonitorOutput` change (it must not)? (3) is
parity mode the ONLY path that touches bash, and does it LookPath rather than assume?
