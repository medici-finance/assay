---
brief: windows-port/00
title: Build-tag split for the unix-only syscall sites in statusgen and desk-tools
why: >-
  The stream's founding premise — "the Go binaries are already portable, no source change
  needed" — was measured on 2026-08-07 and is now false. Today's main does not compile for
  windows at all: statusgen fails on a process-group kill and a Stat_t owner check, and
  internal/deskkit fails on flock, which 38 of the 39 desk-tools commands import. Until the
  source itself cross-compiles, brief 01 can add every windows target it likes to the release
  matrix and the release build will simply break. This brief makes the source build.
wave: 0
depends: []
unblocks: ["windows-port/01"]
effort: M
gate: model
gate-why: >-
  Recorded because this brief touches tools/desk/internal/deskkit/, which is on the
  security-path trigger list, and the four risk answers are nonetheless all "no" — the ruling on
  medici-finance/assay#322 set gate: model deliberately. The answers stand because nothing here
  changes what any control DOES on the platform that has it: the flock and the roster owner check
  keep their exact unix behaviour and error strings (Verify row 7 is the host test suite), the
  windows lock fails closed on contention, and the windows owner check is skipped LOUDLY rather
  than silently no-opped, with the portable group/world-writable mode check still enforced on
  both platforms. Nothing is published, no workflow is touched, and every change is a
  git-revertible source edit. If an implementer finds that compiling for windows would require
  weakening a control on unix too, that is the STOP condition in Ground rules, and the gate is
  re-derived rather than worked around.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [322]
schema: brief-v1
authored: 2026-09-02 by windows-port authoring session
sources:
  - "medici-finance/assay#322 (ruling ratified 2026-09-02): option (b) — author a new wave-0 brief for the source portability fix; windows-port/01 keeps its two-file scope and gains depends-on 00. The ruling names S-M effort and gate: model."
  - "medici-finance/assay#322 body: the reproduction — `GOOS=windows GOARCH=amd64 go build` failing in statusgen (gitinfo.go Setpgid/Kill, rosterconfig.go Stat_t) and in tools/desk (deskkit/claim.go Flock, deskkit/rosterconfig.go Stat_t)"
  - "freshness-checked 2026-09-02 @ origin/main c5498cf: both failures reproduce verbatim; `grep -rn --include='*.go' 'syscall\\.'` over statusgen + tools/desk returns 8 unix-only sites (5 flock, 2 Stat_t, 1 process-group kill) plus one portable signal.Notify site"
  - "docs/streams/windows-port/README.md (stream premise) and brief-01's `sources:` — both carry the stale 'no source change' claim sourced from harness-portability, measured 2026-08-07"
  - "tools/desk/go.mod @ origin/main: golang.org/x/sys v0.46.0 is already in the module graph (indirect, via go-git/go-winio), so the windows lock has a real API available without a new dependency; statusgen/go.mod requires only gopkg.in/yaml.v3 and must stay that way"
consumers:
  - "statusgen/gitinfo.go, statusgen/rosterconfig.go: fixed-here (split into _unix.go/_windows.go pairs)"
  - "tools/desk/internal/deskkit/claim.go, tools/desk/internal/deskkit/rosterconfig.go, tools/desk/internal/loopengine/lineagelock.go, tools/desk/cmd/deskpost/writeflow.go, tools/desk/cmd/deskevidence/writeflow.go, tools/desk/cmd/deskrelease/writeflow.go: fixed-here (all five flock sites move onto one shared deskkit helper; the deskkit owner check splits)"
  - "tools/desk/go.mod: fixed-here (golang.org/x/sys moves from the indirect block to the direct require block; the version and go.sum are unchanged)"
  - "docs/streams/windows-port/brief-01-release-build-matrix.md: follow-up windows-port/01 (01's Verify rows 5 and 6 become satisfiable once this lands; 01's premise sentence now points here)"
  - "docs/streams/windows-port/brief-02-portability-audit.md: out-of-scope (02 audits the delivery and glue layer — hooks, install path, config home, shell-outs — not the Go source this brief splits; nothing 02 reads changes here, so it is a wave-0 peer, not a consumer)"
---

# Brief 00 — Build-tag split for the unix-only syscall sites in statusgen and desk-tools

## Context

files:
- **create** `statusgen/procgroup_unix.go` (planned) + `statusgen/procgroup_windows.go` (planned) — the
  process-group kill helper `gitinfo.go` calls.
- **create** `statusgen/rosterowner_unix.go` (planned) + `statusgen/rosterowner_windows.go` (planned) — the file-owner
  check `rosterconfig.go` calls.
- **amend** `statusgen/gitinfo.go` — replace the inline `SysProcAttr`/`Kill` block with the helper
  call; drop the now-unused `syscall` import.
- **amend** `statusgen/rosterconfig.go` — replace the inline `*syscall.Stat_t` block with the
  helper call; drop the now-unused `syscall` import.
- **create** `tools/desk/internal/deskkit/filelock_unix.go` (planned) +
  `tools/desk/internal/deskkit/filelock_windows.go` (planned) — ONE exported advisory-lock helper for the
  whole module.
- **create** `tools/desk/internal/deskkit/rosterowner_unix.go` (planned) +
  `tools/desk/internal/deskkit/rosterowner_windows.go` (planned) — the deskkit owner check.
- **amend** `tools/desk/internal/deskkit/claim.go`, `tools/desk/internal/loopengine/lineagelock.go`,
  `tools/desk/cmd/deskpost/writeflow.go`, `tools/desk/cmd/deskevidence/writeflow.go`,
  `tools/desk/cmd/deskrelease/writeflow.go` — the five flock call sites move onto the deskkit
  helper; each site's retry loop, deadline, and error text stay byte-identical apart from the
  locking line and the busy-comparison.
- **amend** `tools/desk/internal/deskkit/rosterconfig.go` — owner check moves to the helper.
- **amend** `tools/desk/go.mod` — `golang.org/x/sys` moves out of the `// indirect` block
  (`go mod tidy` does this; the version does not change and `go.sum` is untouched).

facts:
- **eight-unix-only-sites-not-four**: `grep -rn --include='*.go' 'syscall\.' statusgen tools/desk`
  (2026-09-02 @ `c5498cf`, tests excluded) returns EIGHT blocking sites, not the four named in
  the reproduction on #322. The extra four are flock copies —
  `tools/desk/internal/loopengine/lineagelock.go` lines 63 and 78, `tools/desk/cmd/deskpost/writeflow.go` lines 117, 121 and 135,
  `tools/desk/cmd/deskevidence/writeflow.go` lines 52, 56 and 71, `tools/desk/cmd/deskrelease/writeflow.go` lines 115, 119 and 133. They are
  invisible in #322's transcript because the `tools/desk/internal/deskkit` package fails FIRST and every one of those
  packages imports it, so the compiler never type-checks them. Confirmed by
  `GOOS=windows go vet ./internal/loopengine/` and `GOOS=windows go vet ./cmd/deskpost/` (run from `tools/desk`), which each report only the
  deskkit errors.
- **prune.go-is-portable-do-not-touch**: `tools/desk/cmd/deskwt/prune.go:188` uses
  `signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)`. It appears on #322's affected-files
  list but does NOT block: `syscall.SIGINT` and `syscall.SIGTERM` are defined on windows
  (`GOOS=windows go doc syscall.SIGINT`). It needs no split and MUST NOT get one.
- **_unix.go-is-not-a-recognised-goos-suffix**: Go's implicit filename constraint recognises
  `_windows.go` but NOT `_unix.go` — `unix` is a valid `//go:build` TERM (Go 1.19+) and nothing
  more. Every `_unix.go` file created here therefore carries an EXPLICIT `//go:build unix` line;
  without it the file compiles on windows too and the split silently does nothing.
- **x/sys-is-already-in-the-desk-module-graph**: `tools/desk/go.mod` carries
  `golang.org/x/sys v0.46.0 // indirect` and `go.sum` has its entries, so
  `golang.org/x/sys/windows` (`LockFileEx`, `UnlockFileEx`, `LOCKFILE_EXCLUSIVE_LOCK`,
  `LOCKFILE_FAIL_IMMEDIATELY`, `ERROR_LOCK_VIOLATION` — all verified present at that version)
  is importable with no new dependency and no version bump. The ruling's documented-single-writer
  fallback is therefore NOT taken. `statusgen/go.mod` by contrast requires only
  `gopkg.in/yaml.v3`; its two splits need no import beyond the standard library and MUST NOT
  add one.
- **degradation-must-be-loud**: every windows variant here loses something a unix variant
  provides. The rule for all three is the same — the loss is stated in the code, at the place
  that loses it, in words an operator reads: a doc comment for the kill caveat, a printed
  `NOTICE:` line for the skipped owner check. A windows variant that silently returns `nil` where
  the unix one enforced something is the failure mode this brief exists to prevent.

## Ground rules
- NEVER git push / trigger workflows / run a release / run mutating infra commands. Commit only
  per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- **Behaviour on unix MUST NOT change.** This is a compile-target split, not a redesign. Every
  unix code path keeps its current semantics, its current error strings, and its current retry
  timings; `go test ./...` on the host proves it. If a refactor would be tidier but changes a
  unix behaviour, do not do it here.
- **Never weaken a control to make windows compile.** The roster owner check exists so a file
  another account can write cannot name the accounts these tools trust. On windows it is SKIPPED
  and SAID OUT LOUD; it is never quietly turned into a no-op, and the group/world-writable mode
  check that precedes it stays on both platforms. If making the build pass would require
  removing a security check on unix too, STOP and escalate.
- **The lock must fail closed on windows.** A windows lock helper that cannot acquire returns the
  busy sentinel; one that errors for any other reason returns that error. It NEVER returns `nil`
  on failure — a silently-unlocked claim path is a double-dispatch, which is the exact fault
  `claim.go`'s comments say the lock exists to close.
- Do NOT touch `.github/workflows/**` — the release matrix is brief 01's two-file scope and needs
  a workflow-scoped credential this brief does not use.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **statusgen process-group kill.** Extract `gitinfo.go`'s two lines
   (`cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and the `cmd.Cancel` closure calling
   `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`) into a helper — e.g.
   `killWholeProcessGroup(cmd *exec.Cmd)` — called from `listRemoteBranches` where the inline
   block is today.
   - `procgroup_unix.go` (`//go:build unix`): today's behaviour, verbatim, including the comment
     explaining WHY the group kill exists (orphaned `git-remote-<scheme>` helpers piling up).
   - `procgroup_windows.go` (`//go:build windows`): leaves `cmd.Cancel` at the exec default, which
     kills only the direct child. Its doc comment states the caveat plainly — on windows a hung
     remote-helper GRANDCHILD can outlive the timeout, and `cmd.WaitDelay` (unchanged, and set by
     the shared caller, not by either variant) is what still makes the deadline real for the
     caller. Name the windows job-object approach as the follow-up that would close it.
2. **statusgen owner check.** Extract `rosterconfig.go`'s `fi.Sys().(*syscall.Stat_t)` block into
   a helper — e.g. `checkFileOwner(path string, fi os.FileInfo) error`.
   - `rosterowner_unix.go` (`//go:build unix`): today's behaviour and today's two error strings,
     verbatim.
   - `rosterowner_windows.go` (`//go:build windows`): returns `nil`, and FIRST prints one
     `NOTICE:` line to stderr naming the path and saying the owner check is skipped because
     windows has no uid to compare (ownership there is an ACL question needing a different check),
     so the roster's trust rests on the group/world-writable mode check alone. Print it once per
     path, not once per call, if the call is hot.
   - The `mode&0o022` group/world-writable check ABOVE the extraction is portable Go and stays in
     `rosterconfig.go`, on both platforms.
3. **One shared file lock for the desk module.** Create the pair in `internal/deskkit` exporting
   two functions and one sentinel — e.g. `TryLockExclusive(f *os.File) error`,
   `UnlockFile(f *os.File) error`, and `ErrLockBusy`.
   - `filelock_unix.go` (`//go:build unix`): `syscall.Flock(int(f.Fd()), LOCK_EX|LOCK_NB)`,
     mapping `syscall.EWOULDBLOCK` to `ErrLockBusy`; `UnlockFile` is `LOCK_UN`.
   - `filelock_windows.go` (`//go:build windows`): `windows.LockFileEx(windows.Handle(f.Fd()),
     LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))`,
     mapping `windows.ERROR_LOCK_VIOLATION` to `ErrLockBusy`; `UnlockFile` is `UnlockFileEx` over
     the same one-byte range. Lock the same byte range both ways — a lock and an unlock over
     different ranges is a leak.
4. **Move all five flock sites onto it**: `tools/desk/internal/deskkit/claim.go`,
   `tools/desk/internal/loopengine/lineagelock.go`, and the three `tools/desk/cmd/*/writeflow.go` copies. At each site
   the only edits are the `syscall.Flock` call → `deskkit.TryLockExclusive`, and the
   `lerr != syscall.EWOULDBLOCK` comparison → `!errors.Is(lerr, deskkit.ErrLockBusy)`. The
   deadline, the 50ms sleep, the stale-lock message, and every `Unverifiable(...)` string stay
   exactly as they are.
5. **deskkit owner check**: same treatment as step 2, in
   `tools/desk/internal/deskkit/rosterowner_{unix,windows}.go`, preserving deskkit's own error
   strings (they differ slightly from statusgen's — do not converge them here).
6. **Leave `tools/desk/cmd/deskwt/prune.go` alone** — verified portable (see `facts:`). Do not add a split
   for it, and do not "tidy" its `syscall` import away.
7. Run `go mod tidy` in `tools/desk` so `golang.org/x/sys` sits in the direct require block.
   Confirm the version is still `v0.46.0` and `go.sum` is unchanged; if either moves, report
   NEEDS_CONTEXT rather than landing a dependency bump inside a portability split.

## Verify (executable — no prose-only DoD items)

Rows 3 and 4 are `windows-port/01`'s Verify rows 5 and 6, moved here verbatim: they are the
dereferencing proof that the split actually produces windows executables, and 01 keeps its own
copies as its dereferencing check.

| # | Command | Expect |
|---|---------|--------|
| 1 | Every planned pair exists: `for f in statusgen/procgroup statusgen/rosterowner tools/desk/internal/deskkit/filelock tools/desk/internal/deskkit/rosterowner; do test -f "${f}_unix.go" && test -f "${f}_windows.go" \|\| { echo "MISSING pair $f"; exit 1; }; done; echo OK` | `OK` |
| 2 | **Every `_unix.go` carries an explicit build constraint** (the suffix alone does nothing — see `facts:`): `for f in $(find statusgen tools/desk -name '*_unix.go'); do head -5 "$f" \| grep -qE -e '^//go:build unix' -e '^//go:build !windows' \|\| { echo "NO CONSTRAINT $f"; exit 1; }; done; echo OK` | `OK` |
| 3 | **Dereferencing — statusgen cross-compiles for both windows arches** (01's row 5, verbatim): `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-sg-arm64.exe . && file /tmp/wp00-sg-amd64.exe /tmp/wp00-sg-arm64.exe` | exit 0; each `file` line contains `PE32` and `MS Windows` |
| 4 | **Dereferencing — a representative desk verb cross-compiles for both windows arches** (01's row 6, verbatim + arm64): `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-dt-amd64.exe ./cmd/deskpost && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-dt-arm64.exe ./cmd/deskpost && file /tmp/wp00-dt-amd64.exe /tmp/wp00-dt-arm64.exe` | exit 0; each `file` line contains `PE32` and `MS Windows` |
| 5 | **The WHOLE desk suite builds, not just one verb** (deskkit is imported by 38 of the 39 commands, so one verb is not proof): `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | `0` |
| 6 | `GOOS=windows` vet is clean in both modules: `cd statusgen && GOOS=windows GOARCH=amd64 go vet ./...; echo "sg=$?"; cd ../tools/desk && GOOS=windows GOARCH=amd64 go vet ./...; echo "dt=$?"` | `sg=0` and `dt=0` |
| 7 | **The unix suites still pass** — the split changed no host behaviour: `cd statusgen && go test ./...; echo "sg=$?"; cd ../tools/desk && go test ./...; echo "dt=$?"` | `sg=0` and `dt=0` |
| 8 | **No bare unix-only syscall use survives outside a `_unix.go` file**: `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -v '_unix\.go:' ; echo "rc=$?"` | `rc=1` — grep found nothing outside the `_unix.go` files (no lines printed) |
| 9 | **Positive control for row 8** — the same grep WITHOUT the exclusion still finds the unix implementations, so row 8's zero is a real absence and not a broken pattern: `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -c '_unix\.go:'` | `>= 4` (flock lock + unlock, the kill, the two `Stat_t` checks) |
| 10 | **The windows owner check degrades LOUDLY** (a silent `return nil` is the failure mode): `grep -qF 'NOTICE' statusgen/rosterowner_windows.go && grep -qF 'NOTICE' tools/desk/internal/deskkit/rosterowner_windows.go; echo $?` | `0` |
| 11 | **The windows process-group caveat is written down where it is lost**: `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | `0` |
| 12 | **The windows lock fails closed** — it can report busy, so a contended claim is refused rather than granted: `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | `0` |
| 13 | **`prune.go` was left alone** (portable already; a needless split here is churn) — base PINNED so the row is a function of this tree, not of main's moving tip: `BASE=$(git merge-base origin/main HEAD); git diff --name-only "$BASE" HEAD \| grep -c 'cmd/deskwt/prune.go' \|\| true` | `0` |
| 14 | **No new dependency and no statusgen-module change** (same pinned base): `BASE=$(git merge-base origin/main HEAD); git diff "$BASE" HEAD -- tools/desk/go.sum statusgen/go.mod statusgen/go.sum \| wc -l` | `0` — `go.sum` and the whole statusgen module graph are untouched |
| 14a | **x/sys is still at the version it was already on** — the direct/indirect move is not a bump: `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | `1` |
| 15 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/00; echo $?` | `0` — every `consumers:` claim is proved by the branch's own diff |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review
Gate: **model** (from frontmatter). All four risk answers are `no` — this is a compile-target
split of existing logic in git-revertible source; it publishes nothing, touches no workflow, and
changes no unix behaviour.

The review is not "does it compile" — rows 3 to 7 already prove that. The reviewer answers three
questions the build cannot:

1. **Is every windows degradation loud?** Read all three windows variants. Each one loses
   something (a group kill, an owner check, a flock). The reviewer confirms the loss is stated at
   the site — a doc comment for the kill, a printed `NOTICE:` for the owner check — and that no
   variant silently returns success where the unix one enforced a control. A quiet `return nil` in
   `rosterowner_windows.go` passes every executable row in this table and is exactly what this
   brief must not ship.
2. **Does the windows lock fail CLOSED?** `claim.go`'s own comments say the lock is what stops
   double-dispatch. The reviewer confirms the windows helper returns the busy sentinel on
   contention and a real error otherwise — never `nil` — and that lock and unlock cover the same
   byte range.
3. **Did unix behaviour move?** Diff the five flock call sites and the two owner checks against
   `origin/main`: the retry deadline, the 50ms sleep, and every operator-facing error string
   should be unchanged. `go test ./...` passing is necessary, not sufficient — the reviewer reads
   the strings.
