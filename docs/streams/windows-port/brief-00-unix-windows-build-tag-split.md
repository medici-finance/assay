---
brief: assay:assay:windows-port:00
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
unblocks: ["windows-port/01", "windows-port/08"]
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
schema: brief-v2
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
version: 6
id: 381120b3-8be8-40a0-afb9-24d95d56dae0
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
| 2 | `n=0; for f in $(find statusgen tools/desk -name '*_unix.go' -o -name '*_unix_test.go'); do for a in amd64 arm64; do I=$(cd "$(dirname "$f")" && CGO_ENABLED=1 GOOS=windows GOARCH=$a go list -e -f '{{join .IgnoredGoFiles " "}}' .) \|\| { echo "COULD-NOT-CHECK go list for $f ($a)"; exit 2; }; case " $I " in *" $(basename "$f") "*) ;; *) echo "COMPILED ON windows/$a: $f"; exit 1;; esac; done; n=$((n+1)); done; test "$n" -gt 0 \|\| { echo "VACUOUS: no _unix files found"; exit 1; }; echo "OK $n"` — **Every `_unix.go` and `_unix_test.go` file is left out of the windows build** (the suffix alone does nothing — see `facts:`). Re-baselined 2026-10-06 (issue #2297): the row used to accept only the two literal spellings `//go:build unix` and `//go:build !windows` in a file's first five lines, so it reddened on `tools/desk/internal/cellcache/platform_unix.go` (added by #2234), whose `//go:build darwin \|\| linux` is a valid constraint that excludes windows. The row now asks the Go toolchain instead of matching a spelling: for every `_unix` file under statusgen and tools/desk it runs `go list` on the file's own package with `GOOS=windows` on BOTH arches rows 3 and 4 build (amd64 and arm64), and requires the file's name in that package's ignored-files list — the files the windows build leaves out because of their build constraints, test files included. Any constraint that keeps the file out of the windows build passes; a missing constraint, or one that still admits windows on either arch, fails. `CGO_ENABLED=1` is pinned because a cross-compile defaults cgo off, which would also drop an UNCONSTRAINED cgo file from the windows build and let it pass. `-e` keeps a package listable when every one of its files is excluded on windows; the row passes a file only on a POSITIVE listing, so a package load error can never pass it. The test-file form was added 2026-09-30 (issue #1454) because row 8 then excluded `_unix` test files by name too, and this row was what proved such a file was left out of the windows build. Since 2026-10-08 (issue #2297) row 8 asks `go list` itself and no longer depends on this row. | `OK <n>`, n = the number of `_unix` files checked (32 at `11228951d`) — exit 0. Exit 1 printing `COMPILED ON windows/<arch>: <file>` for the first file the windows build includes; exit 1 printing `VACUOUS` when no `_unix` file is found (n = 0 never passes); exit 2 printing `COULD-NOT-CHECK` when `go list` itself fails, which is never a pass |
| 3 | **Dereferencing — statusgen cross-compiles for both windows arches** (01's row 5, verbatim): `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-sg-arm64.exe . && file /tmp/wp00-sg-amd64.exe /tmp/wp00-sg-arm64.exe` | exit 0; each `file` line contains `PE32` and `MS Windows` |
| 4 | **Dereferencing — a representative desk verb cross-compiles for both windows arches** (01's row 6, verbatim + arm64): `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-dt-amd64.exe ./cmd/deskpost && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-dt-arm64.exe ./cmd/deskpost && file /tmp/wp00-dt-amd64.exe /tmp/wp00-dt-arm64.exe` | exit 0; each `file` line contains `PE32` and `MS Windows` |
| 5 | **The WHOLE desk suite builds, not just one verb** (deskkit is imported by 38 of the 39 commands, so one verb is not proof): `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | `0` |
| 6 | `GOOS=windows` vet is clean in both modules: `cd statusgen && GOOS=windows GOARCH=amd64 go vet ./...; echo "sg=$?"; cd ../tools/desk && GOOS=windows GOARCH=amd64 go vet ./...; echo "dt=$?"` | `sg=0` and `dt=0` |
| 7 | `cd statusgen && go test ./...; s=$?; echo "sg=$s"; cd ../tools/desk && O=$(mktemp) && go test -count=1 -v ./internal/deskkit/... ./internal/loopengine/... ./cmd/deskpost/... ./cmd/deskevidence/... ./cmd/deskrelease/... > "$O" 2>&1; d=$?; echo "dt=$d"; [ "$d" -eq 0 ] \|\| grep -e '^--- FAIL' -e '^FAIL' "$O"; c=$(grep -c -e '^--- PASS: TestRegistryCoversCmdBinaries' -e '^--- PASS: TestReStampRecovery' "$O"); echo "pass-lines=$c"; test "$s" -eq 0 && test "$d" -eq 0 && test "$c" -eq 2` — **The unix suites still pass** — the split changed no host behaviour. statusgen's split is its single root package, so `./...` IS the split package; tools/desk is pinned to the split-affected packages (the flock and owner-check sites). Re-baselined 2026-10-06 (issue #2297): the row used to `-skip` #555's two unrelated deskkit test reds (`deskinstall` registration, model-stamp floor recovery) as a TEMPORARY measure, to be removed once #547 + #550 merged and `internal/deskkit` was green again. Both have merged and both tests pass at `11228951d`, so the skip is gone, and the row now proves the tests the skip used to suppress really run: the suite's `-v` output goes to a temp file and the trailing grep counts top-level `--- PASS:` lines under the SAME two name prefixes the old `-skip` selector used (one `-e` pattern per prefix, no regex alternation). Today each prefix names exactly one test (the cmd-binary registry test and the restamp-recovery floor test), so the count is 2; the row fails if either is skipped, renamed away, or not built, and a count other than 2 (a test added or removed under either prefix) fails it too, so the row is re-read rather than silently widened. The row now leads with its command (a witness run used to execute the prose's first code span, `./...`, and record could-not-run) and its exit status carries the verdict: it exits 0 only when both suites pass AND the count is 2, and a red tools/desk run prints its `FAIL` lines from the temp file. Since 2026-10-08 (issue #2297) the statusgen leg does not depend on the runner's `DESK_LOOP`: one statusgen test used to inherit it, and with a verify loop set, verifyrun's pre-work verifier admission refused that test before the refusal it asserts. The package's test main now clears `DESK_LOOP` for every test, and a statusgen test pins that by re-running the test binary with the variable set. | `sg=0`, `dt=0`, `pass-lines=2` (one top-level `--- PASS:` line for each formerly skipped test), exit 0. Exit 1 when either suite fails or the count is not exactly 2 |
| 8 | `L=$(grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -Ev -e ':[0-9]+:[[:space:]]*//' -e ':[0-9]+:[[:space:]]*\*'); test -n "$L" \|\| { echo "VACUOUS: no syscall lines found"; exit 1; }; n=0; for f in $(printf '%s\n' "$L" \| cut -d: -f1 \| sort -u); do for a in amd64 arm64; do I=$(cd "$(dirname "$f")" && CGO_ENABLED=1 GOOS=windows GOARCH=$a go list -e -f '{{join .IgnoredGoFiles " "}}' .) \|\| { echo "COULD-NOT-CHECK go list for $f ($a)"; exit 2; }; case " $I " in *" $(basename "$f") "*) ;; *) echo "COMPILED ON windows/$a: $f"; exit 1;; esac; done; n=$((n+1)); done; echo "OK $n"` — **No REAL unix-only syscall use is compiled into the windows build.** Re-baselined 2026-10-08 (issue #2297): the row used to drop hits by file name, keeping only lines outside `_unix(_test).go` and `_windows(_test).go` files, so it reddened (rc=0) on `statusgen/readmemo_ctime_linux.go` and `statusgen/readmemo_ctime_darwin.go` (added by #2312). Go leaves both out of the windows build because of their GOOS filename suffixes, which the name filter did not list. The row now asks the Go toolchain instead of matching names, the same way row 2 does: it greps the syscall pattern over statusgen and tools/desk, drops full-line `//`/`*` comment lines (a "no `syscall.Kill` here" comment in a windows file is not use), and for every file left it runs `go list` on the file's own package with `GOOS=windows` on BOTH arches rows 3 and 4 build (amd64 and arm64), requiring the file's name in that package's ignored-files list. Any build constraint or GOOS/GOARCH filename suffix that keeps the file out of the windows build passes; a file the windows build compiles on either arch fails, whatever its name, so an unconstrained `_unix.go` file now fails here directly rather than only through row 2. `CGO_ENABLED=1` and `-e` are pinned for row 2's reasons (an unconstrained cgo file must not pass; a package whose every file is excluded on windows must stay listable). The row refuses an empty grep (`VACUOUS`), so a broken pattern cannot pass by checking nothing; row 9 stays the separate positive control. Earlier re-baselines of the name filter: 2026-09-22 (issue #1454: a prose comment in a `_windows.go` file, `763d46ca8`, added the `_windows.go` and comment-line exclusions) and 2026-09-30 (issue #1454: `tools/desk/cmd/cellctl/policy_hang_unix_test.go`, #1594, added the test-file suffixes). | `OK <n>`, n = the number of files with a syscall line that were checked (17 at `966902f7c`) — exit 0. Exit 1 printing `COMPILED ON windows/<arch>: <file>` for the first file the windows build includes; exit 1 printing `VACUOUS` when the grep finds no syscall line; exit 2 printing `COULD-NOT-CHECK` when `go list` itself fails, which is never a pass |
| 9 | **Positive control for row 8** — the same grep WITHOUT the exclusion still finds the unix implementations, so row 8's clean result is a real absence and not a broken pattern: `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -c '_unix\.go:'` | `>= 4` (flock lock + unlock, the kill, the two `Stat_t` checks) |
| 10 | **The Windows roster-owner path never silently passes a permission check it cannot perform** — re-baselined 2026-09-22 (issue #1454): the original loud-skip `NOTICE`-stub anchor (`grep -qF 'NOTICE' …rosterowner_windows.go`) was SUPERSEDED on `a0152b54b` (#640/#641) and `c30ea03e7` (#667), which replaced the skip-and-return-nil stub with full ACL-based enforcement — stronger than this brief's original bar, so the row now asserts the enforcement in place rather than the retired stub. Both Windows variants read the owner SID + DACL, refuse on an unreadable descriptor or ACE, and model a nil DACL as world-writable to force refusal, handing the decision to `evaluateRosterACL` (`rosteracl.go`): `for f in statusgen/rosterowner_windows.go tools/desk/internal/deskkit/rosterowner_windows.go; do grep -qF 'GetNamedSecurityInfo' "$f" && grep -qF 'evaluateRosterACL' "$f" && grep -qF 'S-1-1-0' "$f" \|\| { echo "MISSING acl-enforcement in $f"; exit 1; }; done; echo "wired=OK"; (cd statusgen && go test ./... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "sg-test=OK"); (cd tools/desk && go test ./internal/deskkit/... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "dt-test=OK")` | `wired=OK`, `sg-test=OK`, `dt-test=OK` — both Windows paths read the owner SID + DACL (`GetNamedSecurityInfo`), model a nil DACL as the World SID `S-1-1-0` (→ refusal), and defer to `evaluateRosterACL`, whose `TestEvaluateRosterACL` proves the world-writable / foreign-writer / undeterminable-owner / uninterpretable-ACE cases are all refused |
| 11 | **The windows process-group caveat is written down where it is lost**: `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | `0` |
| 12 | **The windows lock fails closed** — it can report busy, so a contended claim is refused rather than granted: `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | `0` |
| 13 | **`prune.go` was left alone** (portable already; a needless split here is churn) — judged against the IMPLEMENTING diff itself, pinned at both ends to PR #373's merge commit `M`: its branch head `M^2` against `merge-base M^1 M^2` (= `310ef7087`, the branch's own base), so the row means the same thing on merged main as it did on the implementer's branch: `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" && git diff --quiet "$BASE" "$M^2" -- tools/desk/cmd/deskwt/prune.go` | exit 0 — the implementing diff is non-empty (the `!` leg: the pinned range is the real change, not a vacuous one) and `tools/desk/cmd/deskwt/prune.go` is not in it. Exit 1 if the implementation touched `prune.go`; a checkout that cannot resolve `M` (a shallow clone) fails at `git merge-base` with a missing-object error, which is could-not-check, never a pass. _Re-written 2026-09-24 per #1651: the former `BASE=$(git merge-base origin/main HEAD)` resolves to `HEAD` itself on any merged checkout, so its diff was empty and it printed `0` whether or not the property held._ |
| 14 | **No new dependency and no statusgen-module change** (same pinned range as row 13): `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" -- tools/desk/go.mod && git diff --quiet "$BASE" "$M^2" -- tools/desk/go.sum statusgen/go.mod statusgen/go.sum` | exit 0 — `tools/desk/go.sum` and the whole statusgen module graph are untouched by the implementing diff. The `!` leg is the positive control: the same range DOES show `tools/desk/go.mod` changing (x/sys moving from the indirect to the direct block, row 14a), so a zero here is a real absence in the right range. Exit 1 if any of the three files changed. _Re-written 2026-09-24 per #1651 — same vacuous-`BASE` defect as row 13._ |
| 14a | **x/sys is still at the version it was already on** — the direct/indirect move is not a bump: `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | `1` |
| 15 | **Consumers routing corroborated by the implementing diff** (same pinned range as row 13) — each `fixed-here` path in this brief's `consumers:` frontmatter is in the diff, the `out-of-scope` brief-02 is not, and the `follow-up` target brief-01 exists and depends on this brief: `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && C=$(git diff --name-only "$BASE" "$M^2") && test -n "$C" && P=$(awk '/^---$/{n++;next} n==1&&/^consumers:/{f=1;next} f&&/^[^ ]/{f=0} f&&/: fixed-here/' docs/streams/windows-port/brief-00-unix-windows-build-tag-split.md \| sed -e 's/^ *- "//' -e 's/: fixed-here.*//' \| tr ',' '\n' \| tr -d ' ') && test "$(printf '%s\n' "$P" \| grep -c .)" -eq 9 && (for p in $(printf '%s\n' "$P"); do printf '%s\n' "$C" \| grep -qxF "$p" \|\| { echo "NOT IN DIFF $p"; exit 1; }; done) && ! printf '%s\n' "$C" \| grep -qxF docs/streams/windows-port/brief-02-portability-audit.md && grep -qE '^depends:.*"windows-port/00"' docs/streams/windows-port/brief-01-release-build-matrix.md` | exit 0 — all 9 `fixed-here` paths are in the implementing diff, `brief-02-portability-audit.md` (out-of-scope) is not, and brief-01's frontmatter `depends:` line names `windows-port/00` (anchored at line start, so brief-01's body prose quoting that key does not count). Exit 1 on the first `fixed-here` path missing from the diff (printing `NOT IN DIFF <path>`), on a frontmatter parse that yields other than 9 paths (so a broken extraction cannot pass by checking nothing), on a changed brief-02, or on a brief-01 that no longer depends on this brief. _Re-written 2026-09-24 per #1651: `statusgen --consumers` cannot corroborate this brief from its own implementing diff, even run at PR #373's head against its own base — the brief file is not in that diff (it was authored earlier, in `638c79494`), so without `--brief` the tool reports "no brief files in the diff … nothing to corroborate" and exits 0 on ANY tree, and with `--brief windows-port/00` it refuses COULD-NOT-CHECK (exit 2). The row now applies the tool's own per-routing checks (`corroborateBrief`, `statusgen/consumers.go`) directly to the pinned range._ |

## Evidence
### Non-implementer verifier run — VERIFY: HELD (deliverable sound; row 7 blocked on the pre-existing deskkit red #555, row 8 a too-broad grep catching an unrelated comment) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`). `gate: model`, all risk `no`. Impl commit `9109b41`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | _unix.go/_windows.go pairs exist | OK | exit 0 — OK (all 4 pairs) | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | every _unix.go has explicit build constraint | OK | exit 0 — OK | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | statusgen cross-compile amd64+arm64 | PE32/MS Windows both | exit 0 — PE32+ x86-64 + Aarch64, MS Windows | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | deskpost cross-compile amd64+arm64 | PE32/MS Windows both | exit 0 — both PE32+ MS Windows | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | GOOS=windows go build ./... (desk) | 0 | 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | GOOS=windows go vet both modules | sg=0 dt=0 | sg=0, dt=0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | host go test ./... both modules | sg=0 dt=0 | sg=0; **dt=1** — the two failures are the PRE-EXISTING deskkit whole-module red #555 (deskinstall unregistered; model-stamp floor), unrelated to this brief's files; all lock/owner/claim tests PASS incl. acquire-contended-lock-is-unverifiable-not-free. COULD-NOT-CHECK for this brief (blocked on #555) | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | no unix-only syscall outside _unix.go | rc=1 (no output) | **rc=0, one match** — a PROSE COMMENT in tools/desk/internal/deskkit/hookprocess_windows.go:8 (a _windows.go file) added by unrelated later commit 763d46c; NOT actual syscall use (rows 3/5/6/9 prove no leak). Too-broad grep catches comments — re-baseline row 8 to exclude comment lines / _windows.go files | 2026-09-06 | opus-4.8[1m]-verifier |
| 9 | positive control: _unix.go matches | ≥4 | 9 | 2026-09-06 | opus-4.8[1m]-verifier |
| 10 | windows owner check prints NOTICE (both) | 0 | 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 11 | windows procgroup caveat written | 0 | 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 12 | windows lock references ErrLockBusy | 0 | 0 | 2026-09-06 | opus-4.8[1m]-verifier |
| 13 | prune.go untouched | 0 | 0 (also clean vs 9109b41^) | 2026-09-06 | opus-4.8[1m]-verifier |
| 14 | no go.sum / statusgen-module change | 0 | 0 (also clean vs 9109b41^) | 2026-09-06 | opus-4.8[1m]-verifier |
| 14a | x/sys still v0.46.0 | 1 | 1 (indirect→direct move only, version identical) | 2026-09-06 | opus-4.8[1m]-verifier |
| 15 | statusgen --consumers windows-port/00 | 0 | rc=0 — vacuous on merged main (no brief files in the diff vs 5d20ff9); no #557 abort (fresh statusgen); manually corroborated vs 9109b41 diff — fixed-here files present, prune.go/statusgen-module absent | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — windows lock byte-range (offsetLow=0, nBytesLow=1, offsetHigh=0) @ tools/desk/internal/deskkit/filelock_windows.go:29 (LockFileEx) + :43 (UnlockFileEx) — lock and unlock cover the IDENTICAL 1-byte range at offset 0 (standard whole-file advisory lock); equal ranges are the sole correctness requirement (a mismatch = lock leak → double-dispatch) and they match. Companion fail-closed flags LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY @ :28 mirror syscall.LOCK_EX|LOCK_NB @ filelock_unix.go:26 — contention refuses (ErrLockBusy), proven by row 12. The 0o022 mode mask, the 60s lock deadlines, and x/sys v0.46.0 are preserved byte-identical (rank last).`
**VERIFY: HELD — deliverable sound, blocked on two EXTERNAL row-failures.** Rows 1-6, 9-15 PASS: the source cross-compiles for both windows arches in both modules, GOOS=windows vet clean, unix lock/owner/procgroup behaviour preserved byte-for-byte, all three windows degradations loud, the windows lock fails closed. Row 7 fails only on the pre-existing #555 deskkit whole-module red (same blocker as desk-tools/09) — could-not-check for this brief. Row 8 fails only because its grep is too broad and catches a comment in a _windows.go file from unrelated commit 763d46c (no real leak) — a row re-baseline. Held at `implemented` for consistency with dt09; flips once #555 is fixed (row 7) and row 8's grep is tightened. Not a change-failure (no defect in the brief) — no bug, no CFR row; the two items are a #555 dependency + a row re-baseline.

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
### Non-implementer verifier re-run — VERIFY: FAIL (stale Verify-row anchors, not a regression in this brief's own diff) — sonnet-5-verifier (verify-desk dispatch), @ merged main `ee79ed43e920d933e2dabd00d9b5c485ebf30f0b`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main (HEAD already at origin/main). Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted. Brief's implementation commit `9109b4176` (PR #373).

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | pair-existence loop (4 pairs) | OK | exit 0, OK | 2026-09-18 | sonnet-5-verifier |
| 2 | `_unix.go` build-constraint grep | OK | exit 0, OK | 2026-09-18 | sonnet-5-verifier |
| 3 | statusgen GOOS=windows amd64+arm64 build + file | PE32/MS Windows both | exit 0, both confirmed | 2026-09-18 | sonnet-5-verifier |
| 4 | deskpost GOOS=windows amd64+arm64 build + file | PE32/MS Windows both | exit 0, both confirmed | 2026-09-18 | sonnet-5-verifier |
| 5 | `GOOS=windows go build ./...` (tools/desk) | 0 | RC=0 | 2026-09-18 | sonnet-5-verifier |
| 6 | `GOOS=windows go vet ./...` both modules | 0/0 | sg=0, dt=0 | 2026-09-18 | sonnet-5-verifier |
| 7 | host go test both modules with the #555 -skip | 0/0 | sg=0, dt=0. Note: #555's -skip is now stale (all 5 packages pass with no -skip either, since #547/#550 are ancestors of HEAD) — documentation-debt, not a Verify failure | 2026-09-18 | sonnet-5-verifier |
| 8 | grep for bare syscall.(Flock/Kill/Stat_t/SysProcAttr{Setpgid) outside _unix.go | rc=1 (no output) | **FAIL — rc=0**, one match: a prose comment inside a _windows.go file, not real syscall use, in a file this brief never touched (added by unrelated commit 763d46ca8). Same false-positive already flagged by the 2026-09-06 pass, apparently not yet re-baselined. Stale/too-broad grep, not a regression | 2026-09-18 | sonnet-5-verifier |
| 9 | positive control, grep with _unix.go: kept | ≥4 | 9 | 2026-09-18 | sonnet-5-verifier |
| 10 | both rosterowner_windows.go contain NOTICE | 0 | **FAIL — rc=1**, neither file has NOTICE. Root cause: both files were superseded by later, unrelated fixes (a0152b54b #640/#641, c30ea03e7 #667) that replaced the loud-skip stub with full ACL-based enforcement — not a weakening, it exceeds this brief's own bar. Stale anchor from later superseding work, not a fresh regression | 2026-09-18 | sonnet-5-verifier |
| 11 | windows procgroup caveat grep | 0 | exit 0 | 2026-09-18 | sonnet-5-verifier |
| 12 | windows lock ErrLockBusy reference | 0 | exit 0 | 2026-09-18 | sonnet-5-verifier |
| 13 | prune.go untouched, pinned base | 0 | 0 — vacuous on a post-merge run (HEAD==base by construction), same shape the 2026-09-06 pass hit | 2026-09-18 | sonnet-5-verifier |
| 14 | no go.sum/statusgen-module diff, pinned base | 0 | 0 — vacuous, same reason | 2026-09-18 | sonnet-5-verifier |
| 14a | x/sys still v0.46.0 | 1 | 1 | 2026-09-18 | sonnet-5-verifier |
| 15 | `statusgen --consumers windows-port/00` | 0, every claim proved by the diff | exit 0 but vacuous (no brief files in the diff against a synced HEAD); corroborated manually against 9109b4176's own diff instead — fixed-here files present, prune.go/statusgen-module absent, matching rows 13/14 | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all 15 rows map 1:1 to Verify rows; rows 13-15's vacuous-on-post-merge shape is expected, not a defect.

RISK-VALUE: DERIVED — windows advisory-lock byte range (offsetLow=0, nBytesLow=1, offsetHigh=0) @ tools/desk/internal/deskkit/filelock_windows.go:29,43 — lock and unlock cover the identical range; a mismatch is the sole correctness failure mode named in this brief's own ground rules. Confirmed matching this pass. Fail-closed selection and error mapping mirror the unix side exactly, proven live by rows 7/12.
RISK-VALUE: N/A — enumeration over the rest of the diff found no other literal the diff itself introduces; unix signal semantics moved verbatim, the 0o022 mask predates this diff.

VERIFY: FAIL — held at implemented. 13/15 rows checked-clean. Rows 8 and 10 are both stale Verify-row anchors from later, unrelated commits (763d46ca8; a0152b54b/c30ea03e7) that superseded implementation details the rows literally check for — the underlying security intent is met or exceeded on current main (no real syscall leak; the windows owner-check is now full ACL enforcement, stronger than the original design). Not a regression in this brief's own diff, which passes cleanly against its own tree. Recommend re-baselining row 8's grep (exclude comment lines / already-excluded files) and either re-baselining row 10 to assert the ACL enforcement or retiring it as permanently superseded — a driver/coordinator call, not this verifier's. Not filed as a new issue this pass: this session's deskfile budget on medici-finance/assay is fully exhausted for the next ~21h (3 regular + 1 audited override already used today); recording here so it's visible for the next verify pass or another session to file.

### Non-implementer verifier run — VERIFY: BLOCKED — 13/16 pass, 3 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner not the implementer. Own detached temp worktree cut off origin/main at the merged head
(HEAD == origin/main == 438dd26a3e95). Offline envelope observed (KUBECONFIG=/dev/null). No PR
opened, no push, no status flip. gate: model; all four risk answers `no`. Rows 8 and 10 run in
their 2026-09-22 re-baselined form (issue #1454). This brief's Verify table has no `check:ci`-classed
rows. Rows 13-15 (pinned-base diff + consumers routing) corroborate nothing on the fully merged tree
(BASE == HEAD, empty diff) and are could-not-check. Fresh classification pass; rows re-run from
scratch, not carried forward.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `for f in statusgen/procgroup statusgen/rosterowner tools/desk/internal/deskkit/filelock tools/desk/internal/deskkit/rosterowner; do test -f "${f}_unix.go" && test -f "${f}_windows.go" \|\| { echo "MISSING pair $f"; exit 1; }; done; echo OK` | OK | PASS — pairs-present; exit 0; OK | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `for f in $(find statusgen tools/desk -name '*_unix.go'); do head -5 "$f" \| grep -qE -e '^//go:build unix' -e '^//go:build !windows' \|\| { echo "NO CONSTRAINT $f"; exit 1; }; done; echo OK` | OK | PASS — constraints-present; exit 0; OK | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-sg-arm64.exe . && file /tmp/wp00-sg-amd64.exe /tmp/wp00-sg-arm64.exe` | exit 0; each file line PE32 and MS Windows | PASS — pe32-windows; exit 0; /tmp/wp00-sg-amd64.exe: PE32+ executable (console) x86-64, for MS Windows ⏎ /tmp/wp00-sg-arm64.exe: PE32+ executable (console) Aarch64, for MS Windows | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-dt-amd64.exe ./cmd/deskpost && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-dt-arm64.exe ./cmd/deskpost && file /tmp/wp00-dt-amd64.exe /tmp/wp00-dt-arm64.exe` | exit 0; each file line PE32 and MS Windows | PASS — pe32-windows; exit 0; /tmp/wp00-dt-amd64.exe: PE32+ executable (console) x86-64, for MS Windows ⏎ /tmp/wp00-dt-arm64.exe: PE32+ executable (console) Aarch64, for MS Windows | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | 0 | PASS — build-clean; exit 0; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `cd statusgen && GOOS=windows GOARCH=amd64 go vet ./...; echo "sg=$?"; cd ../tools/desk && GOOS=windows GOARCH=amd64 go vet ./...; echo "dt=$?"` | sg=0 and dt=0 | PASS — vet-clean; exit 0; sg=0 ⏎ dt=0 | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `cd statusgen && go test ./...; echo "sg=$?"; cd ../tools/desk && go test ./internal/deskkit/... ./internal/loopengine/... ./cmd/deskpost/... ./cmd/deskevidence/... ./cmd/deskrelease/... -skip '^(TestRegistryCoversCmdBinaries\|TestReStampRecovery)'; echo "dt=$?"` | sg=0 and dt=0 | PASS — suites-green; exit 0; sg=0 ⏎ dt=0 | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -Ev '_(unix\|windows)\.go:' \| grep -Ev ':[0-9]+:[[:space:]]*(//\|\*)' ; echo "rc=$?"` | rc=1 (no output) | PASS — no-bare-syscall; exit 0; rc=1 | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -c '_unix\.go:'` | >= 4 | PASS — control-matches; exit 0; 9 | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | `for f in statusgen/rosterowner_windows.go tools/desk/internal/deskkit/rosterowner_windows.go; do grep -qF 'GetNamedSecurityInfo' "$f" && grep -qF 'evaluateRosterACL' "$f" && grep -qF 'S-1-1-0' "$f" \|\| { echo "MISSING acl-enforcement in $f"; exit 1; }; done; echo "wired=OK"; (cd statusgen && go test ./... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "sg-test=OK"); (cd tools/desk && go test ./internal/deskkit/... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "dt-test=OK")` | wired=OK, sg-test=OK, dt-test=OK | PASS — acl-wired; exit 0; wired=OK ⏎ sg-test=OK ⏎ dt-test=OK | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | 0 | PASS — caveat-present; exit 0; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | 0 | PASS — errlockbusy-present; exit 0; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | `BASE=$(git merge-base origin/main HEAD); git diff --name-only "$BASE" HEAD \| grep -c 'cmd/deskwt/prune.go' \|\| true` | 0 | COULD-NOT-CHECK — merged-tree-vacuous; exit 0; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | `BASE=$(git merge-base origin/main HEAD); git diff "$BASE" HEAD -- tools/desk/go.sum statusgen/go.mod statusgen/go.sum \| wc -l` | 0 | COULD-NOT-CHECK — merged-tree-vacuous; exit 0; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | 1 | PASS — xsys-pinned; exit 0; 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 15 | `statusgen --root . --consumers windows-port/00; echo $?` | 0 | COULD-NOT-CHECK — merged-tree; exit 0; consumers: no brief files in the diff against 438dd26a3e959707d2eb4347aa1310d9173777f9 — nothing to corroborate | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — windows advisory-lock byte range (reserved=0, nBytesLow=1, nBytesHigh=0, Overlapped offset=0) @ tools/desk/internal/deskkit/filelock_windows.go:29 (LockFileEx) and :43 (UnlockFileEx) — lock and unlock cover the IDENTICAL 1-byte range at offset 0 (the standard whole-file advisory lock). Equal lock/unlock ranges are the sole correctness requirement this brief's own ground rules name (a mismatch is a lock leak → double-dispatch); they match. The fail-closed selector LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY @ :28 mirrors syscall.LOCK_EX|syscall.LOCK_NB @ filelock_unix.go:26, and ERROR_LOCK_VIOLATION → ErrLockBusy @ :34-35 mirrors EWOULDBLOCK → ErrLockBusy @ filelock_unix.go:30-31, so contention refuses rather than silently granting — proven live by row 12.

RISK-VALUE: N/A — enumeration over the rest of this brief's own diff found no further introduced literal: the unix flock/kill/owner constants (LOCK_EX, LOCK_NB, LOCK_UN, SIGKILL, the 0o022 group/world-writable mode mask) moved verbatim from the pre-split sites and predate this diff, the 50ms retry sleep and 60s lock deadlines are preserved byte-identical (reversible operational knobs, rank last), and x/sys stays at v0.46.0 (row 14a). The World SID literal S-1-1-0 that row 10 now checks is not introduced by this brief's diff — it entered on later unrelated commits (#640/#641, #667) that superseded the original loud-skip stub with ACL enforcement; it is exercised and correct per row 10, but outside this brief's enumeration scope.

Rows 8 and 10 (re-baselined 2026-09-22) now pass; row 7 ran with the #555 `-skip` in place (`sg=0`, `dt=0`). Rows 13-15 are could-not-check on the merged tree (nothing to corroborate); no defect found in the deliverable.

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

Re-verify of this brief after the 2026-09-24 re-write of rows 13-15 (#1651, which moved them
onto PR #373's merge commit instead of a self-referential merge-base). Runner is not the
implementer; own detached worktree cut off origin/main at merged main
893cd6114b0382a1f71e6ef763c6601a5e19d270 (HEAD == origin/main). Offline envelope observed
(KUBECONFIG=/dev/null). gate: model; all four risk answers `no`. This brief has no
check:ci-classed rows and no row reads a sibling checkout.

**Execution witness.** statusgen built from this tree (`GOWORK=off go build`), then
`statusgen verifyrun --brief` run from the repo root. Its table is landed verbatim below;
exit 2. verifyrun lifts the FIRST backtick span of each Command cell, and seven of this
table's Command cells open with bold prose that itself contains a backticked token, so
verifyrun executed that token instead of the command: rows 2, 7, 8, 10, 13, 15 recorded
could-not-run (exit 127), and row 6 executed only `GOOS=windows` (exit 0, empty output),
so the row 6 witness pass is vacuous and is not counted. That is a check-definition defect
in the Verify table (commands not machine-extractable), not a finding about the deliverable.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `for f in statusgen/procgroup statusgen/rosterowner tools/desk/internal/deskkit/filelock tools/desk/internal/deskkit/rosterowner; do test -f "${f}_unix.go" && test -f "${f}_windows.go" \|\| { echo "MISSING pair $f"; exit 1; }; done; echo OK` | pass exit=0 | sha256:a12b7cb43c9d | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 2 | `_unix.go` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:7b1418af3dd2 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-sg-arm64.exe . && file /tmp/wp00-sg-amd64.exe /tmp/wp00-sg-arm64.exe` | pass exit=0 | sha256:69bb1610b2ff | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-dt-amd64.exe ./cmd/deskpost && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-dt-arm64.exe ./cmd/deskpost && file /tmp/wp00-dt-amd64.exe /tmp/wp00-dt-arm64.exe` | pass exit=0 | sha256:a1667f7eab42 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 6 | `GOOS=windows` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 7 | `./...` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:fa4b756ef454 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 8 | `_unix.go` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:7b1418af3dd2 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -c '_unix\.go:'` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 10 | `NOTICE` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:bac347a4d727 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 13 | `prune.go` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:8aa1a529c203 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 14 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" -- tools/desk/go.mod && git diff --quiet "$BASE" "$M^2" -- tools/desk/go.sum statusgen/go.mod statusgen/go.sum` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 15 | `fixed-here` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:8aafbcf7e173 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |

**Direct runs of the seven mis-extracted rows** — the real command (the longest code span
in the cell), each in `bash -o pipefail -c` at the repo root on the same tree:

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 2 | `for f in $(find statusgen tools/desk -name '*_unix.go'); do head -5 "$f" \| grep -qE -e '^//go:build unix' -e '^//go:build !windows' \|\| { echo "NO CONSTRAINT $f"; exit 1; }; done; echo OK` | OK | PASS — exit 0; OK | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd statusgen && GOOS=windows GOARCH=amd64 go vet ./...; echo "sg=$?"; cd ../tools/desk && GOOS=windows GOARCH=amd64 go vet ./...; echo "dt=$?"` | sg=0 and dt=0 | PASS — exit 0; sg=0 ⏎ dt=0 (the witness row 6 above ran only the leading prose token GOOS=windows, an env assignment that exits 0 with empty output, sha256:e3b0c44298fc — that witness pass is vacuous; this direct run is the real one) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test ./...; echo "sg=$?"; cd ../tools/desk && go test ./internal/deskkit/... ./internal/loopengine/... ./cmd/deskpost/... ./cmd/deskevidence/... ./cmd/deskrelease/... -skip '^(TestRegistryCoversCmdBinaries\|TestReStampRecovery)'; echo "dt=$?"` | sg=0 and dt=0 | FAIL — exit 0; sg=0 ⏎ FAIL github.com/medici-finance/assay/tools/desk/internal/loopengine ⏎ dt=1 — first attempt; the failing test is TestDrain (drain_test.go:97: max concurrency observed 1; the pool never sustained >1 in flight). A second full run of the row reproduced dt=1 on the same test; an isolated TestDrain -count=5 failed 5/5 (3x the concurrency assertion, 2x drain_test.go:82 engine did not stop within deadline). Host load average 59-67 during every run. Same class as the open load-induced loopengine timing flake #612; TestDrain exercises pool concurrency and a stop deadline, not the lock split this brief made. sg=0 and deskpost, deskevidence, deskrelease, deskkit green | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -Ev '_(unix\|windows)\.go:' \| grep -Ev ':[0-9]+:[[:space:]]*(//\|\*)' ; echo "rc=$?"` | rc=1 (no lines) | FAIL — exit 0; rc=0, two lines printed: tools/desk/cmd/cellctl/policy_hang_unix_test.go:43 (SysProcAttr Setpgid) and :53 (syscall.Kill). That file carries //go:build unix on line 1 and was added 2026-09-24 by unrelated work (54ce8dcd2, #1594); its _unix_test.go suffix escapes the row anchor, which excludes only _unix.go / _windows.go. The use is correctly build-constrained (row 6 windows vet is clean), so this is a stale anchor, not a surviving unguarded syscall | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `for f in statusgen/rosterowner_windows.go tools/desk/internal/deskkit/rosterowner_windows.go; do grep -qF 'GetNamedSecurityInfo' "$f" && grep -qF 'evaluateRosterACL' "$f" && grep -qF 'S-1-1-0' "$f" \|\| { echo "MISSING acl-enforcement in $f"; exit 1; }; done; echo "wired=OK"; (cd statusgen && go test ./... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "sg-test=OK"); (cd tools/desk && go test ./internal/deskkit/... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "dt-test=OK")` | wired=OK, sg-test=OK, dt-test=OK | PASS — exit 0; wired=OK ⏎ sg-test=OK ⏎ dt-test=OK | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" && git diff --quiet "$BASE" "$M^2" -- tools/desk/cmd/deskwt/prune.go` | exit 0 | PASS — exit 0; no output (pinned range M=ca92fda79 resolved both parents; implementing diff non-empty; prune.go absent from it) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 15 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && C=$(git diff --name-only "$BASE" "$M^2") && test -n "$C" && P=$(awk '/^---$/{n++;next} n==1&&/^consumers:/{f=1;next} f&&/^[^ ]/{f=0} f&&/: fixed-here/' docs/streams/windows-port/brief-00-unix-windows-build-tag-split.md \| sed -e 's/^ *- "//' -e 's/: fixed-here.*//' \| tr ',' '\n' \| tr -d ' ') && test "$(printf '%s\n' "$P" \| grep -c .)" -eq 9 && (for p in $(printf '%s\n' "$P"); do printf '%s\n' "$C" \| grep -qxF "$p" \|\| { echo "NOT IN DIFF $p"; exit 1; }; done) && ! printf '%s\n' "$C" \| grep -qxF docs/streams/windows-port/brief-02-portability-audit.md && grep -qE '^depends:.*"windows-port/00"' docs/streams/windows-port/brief-01-release-build-matrix.md` | exit 0 | PASS — exit 0; no output (9 fixed-here paths parsed, all in the pinned implementing diff; brief-02 not in it; brief-01 depends on windows-port/00) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Combined: 14 of 16 rows meet their Expect on direct execution (1, 3, 4, 5, 9, 11, 12, 14,
14a by witness; 2, 6, 10, 13, 15 by direct run). Row 7 FAIL (dt=1, loopengine TestDrain under
host load, class #612) and row 8 FAIL (rc=0, stale anchor catching a build-constrained
_unix_test.go file). Neither failure is in this brief's diff; the deliverable itself (the
pairs, constraints, windows cross-compile, windows vet, fail-closed lock, ACL-enforced owner
check, pinned-range rows 13-15) checks clean.

Suggested amendments (for the desk to route): (a) move every leading backticked token out of
rows 2, 6, 7, 8, 10, 13, 15's Command cells (put the prose after the command, or de-backtick
it) so verifyrun lifts the real command; (b) row 8's exclusion should also drop
`_(unix|windows)_test.go` files (or any file whose line 1 is a //go:build unix constraint);
(c) row 7's loopengine leg stays red under load until #612 is fixed.

RISK-VALUE: DERIVED — TryLockExclusive / UnlockFile byte range (reserved=0, nBytesLow=1, nBytesHigh=0, offset 0 via a zero Overlapped) @ tools/desk/internal/deskkit/filelock_windows.go:29 (LockFileEx) and :43 (UnlockFileEx) — lock and unlock cover the IDENTICAL one-byte range at offset 0, the standard whole-file advisory lock; equal ranges are the only correctness requirement the brief's ground rules name (a mismatch leaks the lock and invites double-dispatch), and they match.

RISK-VALUE: DERIVED — lock flags = LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY @ tools/desk/internal/deskkit/filelock_windows.go:28, busy mapping ERROR_LOCK_VIOLATION → ErrLockBusy @ :34-35 — the exact windows analogue of LOCK_EX|LOCK_NB and EWOULDBLOCK → ErrLockBusy @ tools/desk/internal/deskkit/filelock_unix.go:26 and :30; non-blocking plus a busy sentinel, with every other error returned raw, is what makes a contended claim refuse rather than grant (fail closed, row 12).

RISK-VALUE: N/A — enumeration over the rest of the implementing diff (pinned range of PR #373) found no further INTRODUCED literal: the group/world-writable mask 0o022 (statusgen/rosterowner_unix.go:24, tools/desk/internal/deskkit/rosterowner_unix.go:23), Setpgid and SIGKILL (statusgen/procgroup_unix.go:21, :23), cmd.WaitDelay = time.Second (statusgen/gitinfo.go:115) and the 50ms retry sleeps (claim.go:185 and the three writeflow.go copies) moved verbatim from pre-split sites and are reversible operational knobs; golang.org/x/sys v0.46.0 (tools/desk/go.mod:14) is unchanged (row 14a); the World SID S-1-1-0 row 10 checks entered on later commits (#640/#641, #667), outside this diff.

**VERIFY: FAIL** — 9/16 rows pass by execution witness (row 6's witness pass is vacuous and not counted); 14/16 meet Expect on direct execution; rows 7 and 8 fail (environment load flake #612; stale row-8 anchor), and rows 2, 6, 7, 8, 10, 13, 15 are check-definition defects for the witness. No defect found in the deliverable. Status stays implemented; no flip.
### Non-implementer verifier run — VERIFY: FAIL — 15/16 rows by direct run, row 8 (stale anchor, #1454); rows 13-15 now self-proving (#1651 fixed by #1656) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `43420f7ecd743f5c930dc479f54f5ef5ca7b82ed` (HEAD == the forge's `commits/main`). Offline, rows run directly in `bash -o pipefail`. No row is check:ci and none needs native Windows. `statusgen verifyrun --dry-run` (main-source build): exit 2, 9 pass, 7 could-not-run (rows 2, 6, 7, 8, 10, 13, 15 — the witness runs a prose code span, #1805), so no witnessed PASS is possible yet. Status stays `implemented`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1-6, 9, 11, 12, 14, 14a | each row's command verbatim | per row | PASS — rows 3/4 both PE32+ (x86-64 and Aarch64, MS Windows); row 5 `0`; row 6 `sg=0 dt=0`; row 9 `9`; rows 11, 12 `0`; row 14a `1` | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | row 7 command verbatim (statusgen + desk-tools test suites) | sg=0 and dt=0 | PASS on re-run — run 1 `sg=0 dt=1` (only loopengine TestDrain, flake #612); run 2 `sg=0 dt=0`; TestDrain alone 3/3 | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -Ev '_(unix\|windows)\.go:' \| grep -Ev ':[0-9]+:[[:space:]]*(//\|\*)' ; echo "rc=$?"` | rc=1 (no output) | **FAIL** — `rc=0`, two lines: `tools/desk/cmd/cellctl/policy_hang_unix_test.go:43` and `:53`. The file is `//go:build unix` (line 1) from unrelated #1594; its `_unix_test.go` suffix escapes the anchor. Extending the exclusion to `_(unix\|windows)(_test)?\.go:` gives `rc=1` | 2026-09-30 | claude-opus-5-5-verifier |
| 10 | row 10 command verbatim | wired=OK sg-test=OK dt-test=OK | PASS — all three OK; 10 subtests ran in each module | 2026-09-30 | claude-opus-5-5-verifier |
| 13 | row 13 command verbatim (pinned BASE range) | exit 0; planted defects fail | PASS — exit 0; both planted-defect variants exit 1 | 2026-09-30 | claude-opus-5-5-verifier |
| 14 | row 14 command verbatim | exit 0; planted defects fail | PASS — exit 0; both planted-defect variants exit 1 | 2026-09-30 | claude-opus-5-5-verifier |
| 15 | row 15 command verbatim | exit 0; planted defects fail | PASS — exit 0; four planted defects each exit 1, including `NOT IN DIFF` for an invented path | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE: DERIVED — LockFileEx / UnlockFileEx range = (0, 1, 0, zero Overlapped) @ tools/desk/internal/deskkit/filelock_windows.go:29 and :43 — argument order checked against the x/sys v0.46.0 signatures; lock and unlock cover the same one-byte range, so the lock cannot leak.
RISK-VALUE: DERIVED — flags = LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY @ tools/desk/internal/deskkit/filelock_windows.go:28, ERROR_LOCK_VIOLATION → ErrLockBusy @ :34-35 — mirrors LOCK_EX|LOCK_NB and EWOULDBLOCK → ErrLockBusy @ filelock_unix.go:26/:30; busy is refused, other errors return raw, so the lock fails closed.

Findings: (F1) #1651 is resolved by #1656 (a0b3c5218): the pinned range is the real 20-file implementing diff and every planted-defect variant fails. (F2, the FAIL) row 8's anchor predates `_unix_test.go` files; the use is build-constrained, so this is a stale row, not a surviving syscall — routed to #1454. (F3) seven rows still witness as could-not-run until #1805 is fixed for this brief. (F4) row 7 flakes under load on #612. (F5) the 0o022 mode check is unix-only on main; Windows enforces owner SID + DACL instead (#640/#641, #667), which row 10 asserts.
### Verification — 2026-10-01 (assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian)) — VERIFY: PASS, 16/16

**What moved since the last run (2026-09-30, VERIFY: FAIL on row 8 only):** row 8 and row 2 were re-authored by #1880 (aaec59245, an ancestor of this tree). Row 8's exclusion now also drops the `_unix_test.go` / `_windows_test.go` forms. Row 2 now also requires an explicit build constraint on every `_unix_test.go` file, so the wider exclusion cannot hide an unconstrained file. The build-constrained cellctl policy-hang unix test file that reddened row 8 on 09-25 and 09-30 is now excluded, and row 2 proves it carries `//go:build unix`. No change to the deliverable code since the last run.

Runner is not the implementer. Detached worktree cut from refs/remotes/origin/main at merged main b7ca79ab798de5f2faa5861818e72fdb38616386, not shallow. Offline envelope (KUBECONFIG=/dev/null). go1.27.1 darwin/arm64 host. gate: model, all four risk answers `no`. No row is check:ci and no row needs native Windows. `statusgen verifyrun` was not used this pass. Its prose-span extraction defect (#1805) still makes rows 2, 6, 7, 8, 10, 13 and 15 could-not-run under the witness, so every row below was run directly, verbatim.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `for f in statusgen/procgroup statusgen/rosterowner tools/desk/internal/deskkit/filelock tools/desk/internal/deskkit/rosterowner; do test -f "${f}_unix.go" && test -f "${f}_windows.go" \|\| { echo "MISSING pair $f"; exit 1; }; done; echo OK` | OK | PASS — exit 0; `OK` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `for f in $(find statusgen tools/desk -name '*_unix.go' -o -name '*_unix_test.go'); do head -5 "$f" \| grep -qE -e '^//go:build unix' -e '^//go:build !windows' \|\| { echo "NO CONSTRAINT $f"; exit 1; }; done; echo OK` | OK | PASS — exit 0; `OK` over 15 files (the find matched 15 `_unix.go` / `_unix_test.go` files, the cellctl policy-hang unix test file among them) | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-sg-arm64.exe . && file /tmp/wp00-sg-amd64.exe /tmp/wp00-sg-arm64.exe` | exit 0; each line PE32 + MS Windows | PASS — exit 0; `PE32+ executable (console) x86-64, for MS Windows` and `PE32+ executable (console) Aarch64, for MS Windows` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp00-dt-amd64.exe ./cmd/deskpost && GOOS=windows GOARCH=arm64 go build -o /tmp/wp00-dt-arm64.exe ./cmd/deskpost && file /tmp/wp00-dt-amd64.exe /tmp/wp00-dt-arm64.exe` | exit 0; each line PE32 + MS Windows | PASS — exit 0; `PE32+ executable (console) x86-64, for MS Windows` and `PE32+ executable (console) Aarch64, for MS Windows` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | 0 | PASS — `0`, no compiler output | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd statusgen && GOOS=windows GOARCH=amd64 go vet ./...; echo "sg=$?"; cd ../tools/desk && GOOS=windows GOARCH=amd64 go vet ./...; echo "dt=$?"` | sg=0 and dt=0 | PASS — `sg=0` then `dt=0`, no vet findings | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | row 7 command verbatim. Exact text is in block R7 below; it is kept out of this cell because its quoted -skip pattern contains an alternation pipe | sg=0 and dt=0 | PASS, first run — `ok` for statusgen (43.8s) and `sg=0`. deskkit, deskkit/untrustcorpus, loopengine, deskpost, deskpost/internal/bodycheck, deskevidence and deskrelease all `ok`, and `dt=0`. The loopengine TestDrain flake (#612) did not recur. The `-v -count=1` re-run (block R7v) also gives sg=0 and dt=0, with 0 FAIL lines. Every split-relevant test ran and passed, none skipped: the claim, lineage and write-flow lock tests; the roster owner, ACL and custody tests; and the ListRemoteBranches timeout and process-tree kill tests. The 21 SKIP lines all come from fixture-absent or env-gated tests that do not touch this brief's split (workflow, skills, go.work and leak-token fixtures absent from this repo's published file set; live-census, statusgen-binary and drift-register env vars unset). They are listed as could-not-check for themselves, not for this row | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -Ev -e '_unix(_test)?\.go:' -e '_windows(_test)?\.go:' \| grep -Ev -e ':[0-9]+:[[:space:]]*//' -e ':[0-9]+:[[:space:]]*\*' ; echo "rc=$?"` | rc=1, no lines | PASS — `rc=1`, no lines printed. The unfiltered grep lists 12 hits, all in `_unix.go`, `_unix_test.go` or `_windows.go` files (the last is a comment line). Row 2 proves every unix one is build-constrained | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `grep -rn --include='*.go' -E 'syscall\.(Flock\|Kill\|Stat_t\|SysProcAttr\{Setpgid)' statusgen tools/desk \| grep -c '_unix\.go:'` | >= 4 | PASS — `9` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `for f in statusgen/rosterowner_windows.go tools/desk/internal/deskkit/rosterowner_windows.go; do grep -qF 'GetNamedSecurityInfo' "$f" && grep -qF 'evaluateRosterACL' "$f" && grep -qF 'S-1-1-0' "$f" \|\| { echo "MISSING acl-enforcement in $f"; exit 1; }; done; echo "wired=OK"; (cd statusgen && go test ./... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "sg-test=OK"); (cd tools/desk && go test ./internal/deskkit/... -run '^TestEvaluateRosterACL$' -count=1 >/dev/null && echo "dt-test=OK")` | wired=OK, sg-test=OK, dt-test=OK | PASS — `wired=OK`, `sg-test=OK`, `dt-test=OK`. The `-v` re-run shows 11 PASS lines per module (the parent plus 10 subtests), 0 SKIP and 0 FAIL | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | 0 | PASS — `0` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12 | `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | 0 | PASS — `0` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" && git diff --quiet "$BASE" "$M^2" -- tools/desk/cmd/deskwt/prune.go` | exit 0 | PASS — exit 0, no output. BASE resolved to 310ef7087121b7487e5c1b5f7e7864f442b31d02 (the brief's stated branch base) | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 14 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && ! git diff --quiet "$BASE" "$M^2" -- tools/desk/go.mod && git diff --quiet "$BASE" "$M^2" -- tools/desk/go.sum statusgen/go.mod statusgen/go.sum` | exit 0 | PASS — exit 0, no output | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | 1 | PASS — `1` | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 15 | `M=ca92fda79eb28afa72129578196fcf640c56053e && BASE=$(git merge-base "$M^1" "$M^2") && C=$(git diff --name-only "$BASE" "$M^2") && test -n "$C" && P=$(awk '/^---$/{n++;next} n==1&&/^consumers:/{f=1;next} f&&/^[^ ]/{f=0} f&&/: fixed-here/' docs/streams/windows-port/brief-00-unix-windows-build-tag-split.md \| sed -e 's/^ *- "//' -e 's/: fixed-here.*//' \| tr ',' '\n' \| tr -d ' ') && test "$(printf '%s\n' "$P" \| grep -c .)" -eq 9 && (for p in $(printf '%s\n' "$P"); do printf '%s\n' "$C" \| grep -qxF "$p" \|\| { echo "NOT IN DIFF $p"; exit 1; }; done) && ! printf '%s\n' "$C" \| grep -qxF docs/streams/windows-port/brief-02-portability-audit.md && grep -qE '^depends:.*"windows-port/00"' docs/streams/windows-port/brief-01-release-build-matrix.md` | exit 0 | PASS — exit 0, no output. The frontmatter extraction yielded exactly the 9 fixed-here paths: statusgen gitinfo.go and rosterconfig.go, deskkit claim.go and rosterconfig.go, loopengine lineagelock.go, the three writeflow.go files, and tools/desk go.mod | 2026-10-01 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Block R7: row 7's exact command, run from the repo root:**

```
cd statusgen && go test ./...; echo "sg=$?"; cd ../tools/desk && go test ./internal/deskkit/... ./internal/loopengine/... ./cmd/deskpost/... ./cmd/deskevidence/... ./cmd/deskrelease/... -skip '^(TestRegistryCoversCmdBinaries|TestReStampRecovery)'; echo "dt=$?"
```

**Block R7v: the verbose re-run behind row 7's skip audit:**

```
cd statusgen && go test -count=1 -v ./...; echo "sg=$?"; cd ../tools/desk && go test -count=1 -v ./internal/deskkit/... ./internal/loopengine/... ./cmd/deskpost/... ./cmd/deskevidence/... ./cmd/deskrelease/... -skip '^(TestRegistryCoversCmdBinaries|TestReStampRecovery)'; echo "dt=$?"
```

Result: `sg=0`, `dt=0`. statusgen shows 3101 PASS, 0 FAIL, 2 SKIP. The tools/desk packages show 4321 PASS, 0 FAIL, 19 SKIP.

**Scope traceability.** Every row above discharges the Verify row of the same number. The extra checks map to rows too:

- The `-v` skip audit and the check of the two tests row 7 skips both belong to row 7.
- The `-v` subtest count belongs to row 10.
- The unfiltered grep belongs to rows 8 and 9.

No verified work maps to no row.

**Findings, for the desk to route:**

- **F1: row 7's TEMPORARY skip has expired.** Row 7 says its `-skip` is removed once `internal/deskkit` is green again. Both tests it names now pass on this tree when run on their own:

  ```
  cd tools/desk && go test -count=1 -v ./internal/deskkit/ -run '^(TestRegistryCoversCmdBinaries|TestReStampRecovery)'
  ```

  Result: TestRegistryCoversCmdBinaries PASS, the one TestReStampRecovery-prefixed test PASS, package `ok`. The skip can now be dropped from row 7. Left in place, it is the "skip with no expiry" the row itself warns against.
- **F2: rows 13-15 are still self-proving.** They resolved the pinned PR #373 range, and BASE matched the stated 310ef7087.
- **F3: #1805 is still open for this brief.** Seven Command cells open with a prose code span, so `statusgen verifyrun` could not produce a witnessed PASS for them. This pass used direct runs only.
- **F4: the mode check has moved since the brief was written.** The group/world-writable 0o022 mode check now lives in the unix owner-check files, where the brief said it would stay in rosterconfig.go. Windows enforces the owner SID and DACL instead, which row 10 asserts. This carries over unchanged from the prior run's F5.
- **F5: no row shows the lock failing closed on real Windows.** Row 12 shows only that the busy sentinel is referenced, and no row exercises LockFileEx under contention on a Windows host. That is outside this brief's Verify table and is noted for the Windows CI leg (brief 04 / 14).

rows_passed=16 rows_total=16

RISK-VALUE: DERIVED — LockFileEx / UnlockFileEx range (reserved, nBytesLow, nBytesHigh) = 0, 1, 0 with a zero Overlapped (offset 0) @ tools/desk/internal/deskkit/filelock_windows.go:29 (lock) and :43 (unlock). Lock and unlock cover the identical one-byte range at offset 0, the standard advisory whole-file lock idiom. Equal ranges are the only correctness requirement the brief names, because a mismatch leaks the lock and reopens double-dispatch. Reversible by source edit.

RISK-VALUE: DERIVED — lock flags = windows.LOCKFILE_EXCLUSIVE_LOCK or windows.LOCKFILE_FAIL_IMMEDIATELY @ tools/desk/internal/deskkit/filelock_windows.go:28, and busy mapping windows.ERROR_LOCK_VIOLATION to ErrLockBusy @ :34-35. This is the exact Windows analogue of syscall.LOCK_EX or syscall.LOCK_NB with EWOULDBLOCK mapped to ErrLockBusy @ tools/desk/internal/deskkit/filelock_unix.go:26 and :30. Non-blocking plus a busy sentinel, with every other error returned raw and nil only on success, is what makes a contended claim refuse rather than grant (fail closed).

RISK-VALUE: N/A — the rest of the implementing diff (pinned range 310ef7087..ca92fda79^2) introduces no further literal. Enumerated and ranked below, all moved verbatim from pre-split sites or unchanged:

- mode mask 0o022 @ statusgen/rosterowner_unix.go:24 and tools/desk/internal/deskkit/rosterowner_unix.go:23
- Setpgid true and SIGKILL @ statusgen/procgroup_unix.go:21 and :23
- cmd.WaitDelay = time.Second @ statusgen/gitinfo.go:115
- 50 * time.Millisecond retry sleeps @ tools/desk/internal/deskkit/claim.go:185, tools/desk/cmd/deskpost/writeflow.go:134, tools/desk/cmd/deskevidence/writeflow.go:66 and tools/desk/cmd/deskrelease/writeflow.go:128. These are reversible operational knobs.
- golang.org/x/sys v0.46.0 @ tools/desk/go.mod:14, unchanged (row 14a)

The World SID S-1-1-0 that row 10 checks entered later (#640/#641, #667), outside this diff. Nothing is irreversible here: every change is a git-revertible source edit.

**VERIFY: PASS** — 16/16 rows meet Expect on direct execution against merged main b7ca79ab798d. No row is could-not-check for this brief's scope. gate: model. Status stays `implemented` in this batch: the only execution witness table for this brief (2026-09-25, at 893cd6114b03) predates the #1880 row changes and records rows 2, 7, 8, 10, 13 and 15 as could-not-run, and `statusgen verifyrun` was not used this pass. The `verified` close waits for a witnessed re-run on a Linux host (could-not-run rows disclosed under #1805).
2026-10-06 non-implementer re-verification on merged main 11228951d0a8 (rev-parse and the commits API agree; not shallow; go1.27.1 darwin/arm64, KUBECONFIG=/dev/null). Delivering PR #373 (merge ca92fda79eb2) is an ancestor. Since the last PASS at b7ca79ab798d only unrelated edits touched this brief's files; #2234 added a new `_unix` file that row 2 rejects. — VERIFY: FAIL, 15/16 (check-definition, #2297).

| # | Command | Exit | Observed output | Date | Runner |
|---|---------|------|-----------------|------|--------|
| 1 | Verify row 1 as written (the four `_unix` / `_windows` file-pair checks) | 0 | `OK` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (head -5 of every `_unix` file must carry `//go:build unix` or `//go:build !windows`) | 1 | **FAIL**: `NO CONSTRAINT tools/desk/internal/cellcache/platform_unix.go` (1 of 32 files). Its line 1 is `//go:build darwin \|\| linux`, a valid constraint that excludes windows; the row's regex only accepts the unix / !windows spellings. File added by #2234 (2026-10-05). Independent check: `GOOS=windows go list -f '{{.IgnoredGoFiles}}' ./internal/cellcache` in tools/desk lists it, so it is not compiled on windows. Stale check, not a regression; filed #2297 | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (statusgen windows amd64 + arm64 builds, then file) | 0 | `PE32+ executable (console) x86-64, for MS Windows` and `PE32+ executable (console) Aarch64, for MS Windows`; -o targets moved into the verifier's own worktree scratch dir | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (deskpost windows amd64 + arm64 builds, then file) | 0 | the same two PE32+ lines; same -o change as row 3 | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | 0 | `0`, no compiler output | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as written (windows go vet over both modules) | 0 | `sg=0`, `dt=0`, no vet findings | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as written (statusgen go test, then the five tools/desk package sets with the row's -skip) | 0 | `sg=0`, `dt=0`; statusgen ok (91.3s, not cached), streamview ok; deskkit, untrustcorpus, loopengine, deskpost, bodycheck, deskevidence, deskrelease all ok. A -count=1 -v re-run of the lock, owner, claim and roster tests gave 0 FAIL. The row's temporary -skip has expired: both skipped tests pass on their own at this SHA (noted on #2297) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (unconstrained Flock / Kill / Stat_t / Setpgid grep, `_unix` / `_windows` files and comments filtered) | 0 | `rc=1`, no lines. Unfiltered: 26 hits, all in `_unix`, `_unix_test` or `_windows` files (the windows one a comment); three are in the cellcache platform_unix.go file, whose exclusion from windows row 2 should prove and the go list check above proves instead | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (count of those calls in `_unix` files) | 0 | `19` (≥ 4) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as written (ACL wiring greps over both Windows roster-owner files, then the roster-ACL test per module) | 0 | `wired=OK`, `sg-test=OK`, `dt-test=OK`; -v shows 11 PASS lines per module (parent plus 10 subtests) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | 0 | `0` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | 0 | `0` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | Verify row 13 as written (pinned merge ca92fda7: diff non-empty, prune.go untouched) | 0 | exit 0, no output; BASE resolved to 310ef7087121 | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | Verify row 14 as written (tools/desk go.mod changed; go.sum and statusgen go.mod / go.sum untouched) | 0 | exit 0, no output | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | 0 | `1` | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | Verify row 15 as written (pinned #373 range; consumers frontmatter; brief-02 absent; brief-01 depends line) | 0 | exit 0, no output: all 9 fixed-here paths in the diff, brief-02 not, brief-01 depends on windows-port/00 | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Advisory: go.mod now makes go-winio, cobra, viper, cast and pflag direct requires (#2147, #2141), outside this brief's pinned range; row 14a still holds.

RISK-VALUE: DERIVED — LockFileEx range (reserved, nBytesLow, nBytesHigh) = 0, 1, 0 at offset 0 @ tools/desk/internal/deskkit/filelock_windows.go:29, UnlockFileEx = 0, 1, 0 @ :43 — lock and unlock cover the same one-byte range, the standard advisory whole-file lock; matching ranges are the only correctness requirement the brief names.
RISK-VALUE: DERIVED — flags = LOCKFILE_EXCLUSIVE_LOCK\|LOCKFILE_FAIL_IMMEDIATELY @ tools/desk/internal/deskkit/filelock_windows.go:28, ERROR_LOCK_VIOLATION→ErrLockBusy @ :34-35 — exact analogue of LOCK_EX\|LOCK_NB with EWOULDBLOCK→ErrLockBusy @ filelock_unix.go:26/:30; non-blocking, busy sentinel on contention, other errors passed through, nil only on success (fails closed).
RISK-VALUE: DERIVED — owner binding `int(st.Uid) != os.Getuid()` @ statusgen/rosterowner_unix.go:40 and tools/desk/internal/deskkit/rosterowner_unix.go:39 — byte-identical move from the pre-split rosterconfig.go at 310ef7087, unix behaviour unchanged as the brief requires. All other values (Setpgid/SIGKILL, WaitDelay 1s, 50ms retry sleeps, x/sys v0.46.0) are reversible verbatim moves.

**VERIFY: FAIL** — check-definition: row 2's build-constraint regex rejects the valid `//go:build darwin || linux` on a file #2234 added later; not a regression in this brief's deliverable. Re-baseline tracked on #2297 (same class as #1454). Status stays implemented.
### 2026-10-07 desk dispatch — VERIFY: FAIL (windows-port/00 @ fe2217521989, 14/16 rows)

Non-implementer verifier re-run on merged main fe2217521989c9925a83e57c083fc11028990824 (includes #2307, merge a641efac9). Every row was run by hand at the repo toplevel; the witness dry-run (statusgen verifyrun v1.0.32) was also taken and agrees on rows 1-5, 7, 9, 11, 12, 14, 14a, but it extracted only a prose span for rows 6, 10, 13 and 15 (could-not-run, prose-led-command) and scored row 8 on exit status alone, so the hand runs below are the record.

| Row | Command | Exit | Observed | Date | Runner |
|-----|---------|------|----------|------|--------|
| 1 | Verify row 1 as written (planned-pair existence loop) | 0 | prints OK | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (go list IgnoredGoFiles loop, windows amd64 + arm64, CGO_ENABLED=1) | 0 | prints OK 32 — every _unix.go / _unix_test.go file is in the windows ignored-files list on both arches | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (statusgen windows amd64 + arm64 build, then file) | 0 | PE32+ executable (console) x86-64, for MS Windows; PE32+ executable (console) Aarch64, for MS Windows | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (deskpost windows amd64 + arm64 build, then file) | 0 | PE32+ executable (console) x86-64, for MS Windows; PE32+ executable (console) Aarch64, for MS Windows | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | 0 | prints 0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as written (GOOS=windows GOARCH=amd64 go vet in both modules) | 0 | sg=0, dt=0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as written (statusgen go test, then tools/desk -v suite into a temp file, pass-line count) | 1 | FAIL — sg=1, dt=0, pass-lines=2. The single red is the statusgen depscope test in coverage_inputs_test.go (added by #2304 today), asserting verifyrun with a subdirectory root refuses with exit 2 naming the toplevel; it got exit 2 from the pre-work verifier admission instead, because the test inherits DESK_LOOP=verify-desk from the verifier session and verifieradmission.go requires admission whenever DESK_LOOP contains "verify". Diagnostic re-run of the same command with DESK_LOOP removed from the environment: sg=0, dt=0, pass-lines=2, exit 0. The split's own packages are green; the red is a non-hermetic test, not a regression in this brief's diff — but the row as run in a verify-desk session exits 1 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (syscall grep minus _unix/_windows files and comment lines, then echo rc) | 0 | FAIL — rc=0, two lines printed: statusgen/readmemo_ctime_linux.go:13 and statusgen/readmemo_ctime_darwin.go:13, each a fi.Sys() type assertion to syscall.Stat_t. Both files were added by #2312 today and carry GOOS filename suffixes (and a //go:build linux line); go list with GOOS=windows on amd64 and arm64 lists both in the statusgen ignored-files list, so windows never compiles them. Stale anchor: the exclusion covers only the _unix/_windows suffixes, not other GOOS suffixes — build-constrained use, not a surviving unix-only syscall | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (same grep, count of _unix.go hits) | 0 | prints 19 (expect at least 4) | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as written (ACL wiring greps on both rosterowner_windows.go files, then the two evaluate-roster-ACL tests) | 0 | wired=OK, sg-test=OK, dt-test=OK | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | Verify row 11 as written (caveat grep on statusgen procgroup_windows.go) | 0 | prints 0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | Verify row 12 as written (ErrLockBusy grep on deskkit filelock_windows.go) | 0 | prints 0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | Verify row 13 as written (pinned M=ca92fda79, diff from merge-base to M^2 non-empty and prune.go absent) | 0 | exit 0; merge-base resolved to 310ef7087 as the row states | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | Verify row 14 as written (same pinned range; desk go.mod changed, desk go.sum and statusgen go.mod/go.sum untouched) | 0 | exit 0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | 0 | prints 1 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | Verify row 15 as written (frontmatter fixed-here extraction = 9 paths, all in the pinned diff; brief-02 absent; brief-01 depends on windows-port/00) | 0 | exit 0 | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |

#2297 (row 2 build-constraint regex) is resolved by #2307: row 2 now asks go list and passes (OK 32, both arches), and row 7 runs with the old exclusion removed and finds both formerly excluded tests (pass-lines=2). The two reds on this pass are both new since #2307 and both are in the check, not in the brief's deliverable: row 7 is a non-hermetic statusgen test from #2304 that reads DESK_LOOP, and row 8 is a stale exclusion anchor tripped by the GOOS-suffixed files from #2312.

Risk enumeration (trigger: the diff touches the deskkit security-path; risk metadata all no, not irreversible). Literals in the split: windows lock range offset 0 / length 1 (LockFileEx and UnlockFileEx args 0, 1, 0) at tools/desk/internal/deskkit/filelock_windows.go lines 29 and 43; group/world-writable mask 0o022 at statusgen/rosterowner_unix.go:24 and tools/desk/internal/deskkit/rosterowner_unix.go:23; World SID S-1-1-0 at statusgen/rosterowner_windows.go:62 and tools/desk/internal/deskkit/rosterowner_windows.go:81; WaitDelay one second at statusgen/gitinfo.go:115 (pre-existing, unchanged); golang.org/x/sys v0.46.0 at tools/desk/go.mod:19. All are git-revertible source edits; the top-ranked is the lock range, since a mismatch there would silently un-serialise the claim path (double dispatch). The mask 0o022 is exactly group-write 0o020 plus other-write 0o002, and S-1-1-0 is the well-known Everyone SID.

RISK-VALUE: DERIVED — windows lock range (offset, length) = (0, 1) @ tools/desk/internal/deskkit/filelock_windows.go:29 (unlock :43) — every locker of a lock file takes the same single byte at offset 0 exclusively with fail-immediately, so any two contenders collide on that byte and the second gets ERROR_LOCK_VIOLATION mapped to ErrLockBusy (fail closed); unlock names the identical range, so no lock leaks; Windows byte-range locks may extend past end of file, so a zero-length lock file still locks.

Filed: rows 7 and 8 are recorded on #2297 (check-definition; the original row 2 subject is resolved by #2307). The brief does not advance.

**VERIFY: FAIL**

### Non-implementer verifier run — 2026-10-08 assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian)

Second verify pass of 2026-10-08, taken after #2392 merged. Subject: main aff2ef11c744e27bfd328ee4fc874139859dd152, confirmed equal to the forge's main by an independent API read; the delivering merge ca92fda79eb2 (#373) is an ancestor and the repository is not shallow. All sixteen rows were run by hand from the repository root (go1.27.1 darwin/arm64, no network or cluster contact), each command extracted mechanically from the Verify table. The desk re-ran rows 1, 2, 8, 9, 11, 12, 13, 14, 14a and 15 itself at the same sha: exit 0 each, with the same output. **No execution witness was written in this run**; the first bullet under the table says why, and the hand runs below are the record.

| Row | Command | Exit | Observed | Date | Runner |
|-----|---------|------|----------|------|--------|
| 1 | Verify row 1 as written (planned-pair existence loop) | 0 | prints OK | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (go list IgnoredGoFiles loop, windows amd64 + arm64, CGO_ENABLED=1) | 0 | prints OK 36 — every _unix.go / _unix_test.go file is in the windows ignored-files list on both arches (the count is 36 at this sha; the row's pass condition is the OK line and exit 0, not the number) | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (statusgen windows amd64 + arm64 build, then file), with the four output files written to a scratch directory instead of the row's fixed temp names | 0 | PE32+ executable (console) x86-64, for MS Windows; PE32+ executable (console) Aarch64, for MS Windows | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (deskpost windows amd64 + arm64 build, then file), same output-file change as row 3 | 0 | PE32+ executable (console) x86-64, for MS Windows; PE32+ executable (console) Aarch64, for MS Windows | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./...; echo $?` | 0 | prints 0, no compiler output | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 as written (GOOS=windows GOARCH=amd64 go vet in both modules) | 0 | sg=0, dt=0, no vet findings | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 as written (statusgen go test, then the tools/desk -v suite into a temp file, pass-line count), run with HOME pointed at a throwaway directory and DESK_LOOP left set to a verify loop | 0 | ok github.com/medici-finance/assay/statusgen 95.764s; sg=0, dt=0, pass-lines=2 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (syscall grep minus comment lines, then go list IgnoredGoFiles per file, windows amd64 + arm64) | 0 | prints OK 17 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (same grep, count of _unix.go hits) | 0 | prints 20 (expect at least 4) | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10 as written (ACL wiring greps on both rosterowner_windows.go files, then the two evaluate-roster-ACL tests), run under the same throwaway HOME | 0 | wired=OK, sg-test=OK, dt-test=OK; with -v the named test shows 11 RUN lines in each module (the parent and 10 subtests) | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `grep -qiE -e 'grandchild' -e 'process group' -e 'orphan' statusgen/procgroup_windows.go; echo $?` | 0 | prints 0 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `grep -qF 'ErrLockBusy' tools/desk/internal/deskkit/filelock_windows.go; echo $?` | 0 | prints 0 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | Verify row 13 as written (pinned M=ca92fda79, diff from merge-base to M^2 non-empty and prune.go absent) | 0 | exit 0, no output; merge-base resolved to 310ef7087121, the range names 20 files | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | Verify row 14 as written (same pinned range; desk go.mod changed, desk go.sum and statusgen go.mod/go.sum untouched) | 0 | exit 0, no output | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 14a | `grep -c 'golang.org/x/sys v0.46.0' tools/desk/go.mod` | 0 | prints 1 | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |
| 15 | Verify row 15 as written (frontmatter fixed-here extraction = 9 paths, all in the pinned diff; brief-02 absent; brief-01 depends on windows-port/00) | 0 | exit 0, no output | 2026-10-08 | assay-verifier-app[bot] @ aff2ef11c744 (claude-opus-5-5) (on-behalf-of human:ian) |

- **Why there is no witness.** Two separate reasons, either of which is enough.
  - Rows 6, 10, 13 and 15 open with a prose code span and not with their command (GOOS=windows, NOTICE, prune.go and fixed-here), read from the row text at the subject sha. The 2026-10-07 block above records the witness runner, on the same tool version, extracting only a prose span for exactly those four rows and marking them could-not-run. Rows 7, 8 and 9 now lead with their command. Until the four rows are re-authored command-first, no witness for this table can cover every row. Recorded on #2297.
  - Rows 7 and 10 run go test over tools/desk packages, and those suites write an audit log under the runner's home directory. The hand runs pointed HOME at a throwaway directory, which held 14 audit lines afterwards. A witness run executes rows against the operator's real home, so those lines would have landed in the live log (#1618).
- **Rows 7 and 8 across #2392**, compared against the table before that change. Rows 2 and 9 changed in prose only; no other row changed.
  - Row 7: the command and the Expect are byte-identical before and after; three sentences of prose were appended. The fix is in statusgen test code: the package's test main now clears DESK_LOOP, and a new test re-runs the test binary with the variable set to pin that. No test was skipped, removed or loosened. This run had DESK_LOOP set to a verify loop, the exact condition of the 2026-10-07 red, and the row passed.
  - Row 8: re-authored, command and Expect both. Before, a grep that excused a hit by file name and always exited 0, with the verdict only in a printed rc line. After, the same grep followed by a go list question per file: is this file ignored on windows, on both amd64 and arm64. The exit status now carries the verdict, and an empty grep prints VACUOUS and exits 1 where it used to be the passing output. It is narrower in one respect: a file that carries a correct build constraint but not a platform-split name is now accepted, where the earlier row flagged it. That matches the row's own promise, which is about what the windows build compiles. No assertion was dropped, and row 9 is still the separate positive control.
  - Both rows were checked under mutation on a scratch copy outside the worktree. Row 8 exits 1 naming the file when the unix build constraint is removed from a split file, when an unconstrained file with a syscall use is added, when a real syscall use is put in a _windows.go file, and when a constraint excludes only one windows arch; it exits 0 when the added file is constrained to linux, and exits 1 printing VACUOUS on a tree with no Go files. For row 7, making the unix lock fail open turns four deskkit lock tests red, and making the statusgen owner check a no-op turns TestConfigHomePermissionsEnforced red; both packages are in the row's package lists. Those two were run on the covering tests, not on the whole row.

RISK-VALUE scope: all four risk answers are no and the item is not irreversible; the enumeration was done because the implementing diff touches tools/desk/internal/deskkit. It covered every added non-comment line of the Go files and tools/desk/go.mod in the implementing range (merge-base 310ef7087121 to the second parent of ca92fda79eb2, 20 files). The unix lock flags, the owner-uid comparison and the process-group kill are verbatim moves; the x/sys version is unchanged (row 14a). Line numbers are at the subject sha.

RISK-VALUE: DERIVED — windows lock range (reserved, bytesLow, bytesHigh) = (0, 1, 0) at offset 0 @ tools/desk/internal/deskkit/filelock_windows.go:29, unlock (0, 1, 0) @ tools/desk/internal/deskkit/filelock_windows.go:43 — checked against the pinned x/sys v0.46.0 signatures: reserved must be 0, the range is one byte at offset 0, and lock and unlock name the identical range. Every locker of a lock file contends for the same byte, so the second is refused. This is the value the brief's Task specifies.

RISK-VALUE: DERIVED — windows lock flags = LOCKFILE_EXCLUSIVE_LOCK with LOCKFILE_FAIL_IMMEDIATELY @ tools/desk/internal/deskkit/filelock_windows.go:28, and ERROR_LOCK_VIOLATION mapped to ErrLockBusy @ tools/desk/internal/deskkit/filelock_windows.go:34 — the analogue of LOCK_EX with LOCK_NB and EWOULDBLOCK mapped to ErrLockBusy @ tools/desk/internal/deskkit/filelock_unix.go:26 and :30: exclusive, non-blocking, a busy sentinel on contention, every other error passed through, nil only when the lock was taken.

RISK-VALUE: NAMED, NOT DERIVED — runtime effect of both values on a Windows host @ tools/desk/internal/deskkit/filelock_windows.go:28 — the two derivations above are from the API contract and the source. No Verify row executes the windows lock, and this host cannot. A contended-lock test run on Windows is the missing observation.

VERIFY: PASS — 16 of 16 rows meet Expect by hand; no defect was found in the deliverable. The pass is hand Evidence only: no execution witness was written.

**The board row is not changed by this run and stays `implemented`.** A verified row needs a witness for every Verify row, and this table cannot take one yet (first bullet). Row 8 was also re-authored by the change that turned it green (second bullet), so the status change is left to a maintainer. Review notes recorded internally; maintainer follow-up required (#2392).

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
