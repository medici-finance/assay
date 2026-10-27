---
brief: assay:assay:build-less-brittle:15
title: Witness inputs are recorded, not parsed
why: >-
  A verified brief reaches done only when its Verify witnesses still speak for the current tree.
  Today a witness speaks for almost every file in the repository, so an unrelated merge (a plugin
  index, a surfaces list) holds every gate:model brief at verified, and nothing auto-flips. The
  earlier fix guessed each row's inputs from its command text, which covers a few shell readers and
  none of the go, git or statusgen rows. Recording what a row really read, and comparing only that,
  releases a witness exactly when its real inputs are unchanged.
wave: 0
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  It narrows a verification control. Today a witness is judged against every path outside the
  board's bookkeeping; after this brief it is judged only against the paths its run recorded. An
  under-recorded read set lets a stale witness pass, which is the failure the coverage gate exists
  to prevent. The driver confirms the narrowing, the floor behind it, and the one-time re-witness.
decision-trigger: creation
decision-issue: 4512
ruling: "https://github.com/medici-finance/assay/issues/4512#issuecomment-6420851894"
issues: [2026]
regression-of: medici-finance/assay#2304
schema: brief-v2
version: 1
authored: 2026-10-26 by a worker-desk authoring session (design proposal; the driver ruled option 1 on 2026-10-26, and review round 1 amended it)
sources:
  - "medici-finance/assay#2026 (the mechanism's issue; its strike-two design note names options a-c)"
  - "medici-finance/assay#3105 (source issue: gate:model briefs held at verified)"
  - "medici-finance/assay#2018 (no gate or board line reports that zero briefs are released)"
  - "medici-finance/assay#2304 (prior fix: command-text input parser and witness landing check)"
  - "medici-finance/assay#4512 (decision issue; the driver ruled option 1)"
  - "docs/contracts.md S-evidence-coverage"
  - "freshness-checked 2026-10-26 @ 5ddf5a317 (origin/main)"
exec-tier: strong
exec-tier-why: "(a) the recording mechanism and its completeness rule are not pre-specified; (b) correctness spans verifyrun, the coverage library, three callers and the evidence-landing verb; (c) an error here lets a stale witness release a brief to done; (d) raised by an error-class trigger (strike two on the mechanism's issue)."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/coverage.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/autoflip.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/reconcilefold.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/evaluation/coverage.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/evaluation/coverage_api.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskevidence/main.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/contracts.md: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/domain/assess.go: out-of-scope (a fourth library entry, imported only by tests; it passes its own coverage input, and the library's go-run rule in Task step 6 makes it fail closed without an edit)"
  - "statusgen/evaluation/coverage_inputs.go: out-of-scope (the command-text parser stays until the measurement in Task step 10 shows it releases nothing on its own; its retirement is a later brief)"
---

# Brief 15 — Witness inputs are recorded, not parsed

Related: medici-finance/assay#2026 (this brief's issue), medici-finance/assay#4512 (its decision
issue), medici-finance/assay#3105, medici-finance/assay#2018.

The driver ruled option 1 on the decision issue (see `ruling:`): a recorded read set with the go
compiled-file floor, and one batch re-witness at a single main revision. Review round 1 then
amended the recording, storage, judging and batch rules below. Implementation is dispatched from
this brief once it merges.

## Context
files:
- `statusgen/verifyrun.go` — runs each row under the recorder and writes the read-set token into the witness row's Runner cell
- `statusgen/readset_linux.go` (planned), `statusgen/readset_other.go` (planned) — the recorder, Linux only; every other host reports `could-not-check` (precedent: the Linux-only facility in `statusgen/netns.go`)
- `statusgen/readset_platform.go` (planned) — the platform decision and the off-Linux token text, no build constraint, so it builds and is tested on every OS (row 28)
- `statusgen/coverage.go` — the adapter: one read-set source (`coverageOptionsFor`) and the go compile-input floor, both handed to the library as observations
- `statusgen/autoflip.go`, `statusgen/reconcilefold.go` — call `coverageOptionsFor` instead of passing `coverageOptions{}`
- `statusgen/evaluation/coverage.go`, `statusgen/evaluation/coverage_api.go` — judge a witness against its recorded read set
- `tools/desk/cmd/deskevidence/main.go` — lands a witness row and its read-set file in one commit
- `docs/contracts.md` — the S-evidence-coverage row names the new independent layer
- tests: `statusgen/evaluation/readset_test.go` (planned), `statusgen/readset_test.go` (planned), `statusgen/readset_linux_test.go` (planned), `tools/desk/cmd/deskevidence/readset_test.go` (planned)

facts:
- Current shape. With no dependency manifest, a witness speaks for every path outside
  `docs/streams/**` and `STATUS.md` (`witnessScope.describe`, `statusgen/evaluation/coverage.go`).
  All three callers pass an empty manifest: `statusgen/autoflip.go:1068` and
  `statusgen/reconcilefold.go:138` pass `coverageOptions{}`, and `--coverage`
  (`statusgen/coverage.go:323`) passes `coverageOptions{Patterns: patterns}`. The
  `Dependencies` field exists and nothing fills it. A fourth entry point,
  `statusgen/domain/assess.go:111`, calls `EvaluateCoverage` with a caller-built input; only
  tests import it.
- Prior fix (medici-finance/assay#2304). `deriveRowInputs` (`statusgen/evaluation/coverage_inputs.go`)
  reads the command text with a closed shell grammar. A row it cannot parse keeps the whole-tree
  scope. Its header states why `go` is outside the grammar: the modules most rows test read
  outside themselves.
- Before measurement 1, relayed by the coordinator, not re-run by the author: the model
  auto-flip job of 2026-10-26 14:59Z refused 35 briefs at item revision 9ab9fa76ab4d.
- Before measurement 2, taken by the author: on 5ddf5a317 (main, 2026-10-26), a statusgen built
  from that tree (`cd statusgen && go build -o <tmp>/sg .`) ran
  `<tmp>/sg --root . --auto-flip-model --dry-run`. Exit 0, 36 briefs
  `REFUSED (stays verified) — coverage is not released`, 0 flips, 301 refused Verify rows.
  - First differing path the witness spoke for: `.claude-plugin/marketplace.json` on 186 rows,
    `.assay-surfaces` on 101, `.gitattributes` on 7, any other path on at most 1 row each.
  - Why the parser kept the whole-tree scope: 269 rows on 36 briefs it cannot parse. Of these,
    164 have a shell metacharacter, 76 run a command outside the set (`go` 53, `git` 10,
    `statusgen` 6, other 7), 21 have a double-quote expansion, 7 an assignment, and 1 a
    `grep -r`. A further 59 rows on 7 briefs have `+dirty` witnesses, and 4 rows on 4 briefs
    were edited after their witness ran.
  - A `--coverage --json` pass over all 568 briefs at the same revision released 0 briefs.
    42 claims pass only because the parser derived their inputs. 133 briefs are fully witnessed
    and held only by `wrong-revision`.
- A derived input set for go rows (`cd <module> && go test|build|vet`, inputs = the module's
  files plus `go list -deps -test`) does not release enough to justify itself, and it is unsound:
  - Yield. 25 of the 301 refused rows have that clean shape. Every one of the 36 briefs has some
    other row shape or a `+dirty` or edited witness. So 0 of 36 would release.
  - Soundness. build-less-brittle/10 row 1 (`cd tools/desk && go test ./internal/arch/`) runs
    `tools/desk/internal/arch/arch_test.go`. That test opens `docs/contracts.md` and stats
    `statusgen/evaluation` at the repository root. Neither path is in the `tools/desk` module or
    its `go list -deps -test` graph. In `tools/desk`, 117 test lines climb three directories
    with `../`, and 6 test files ask git for the toplevel. A derived set misses these reads,
    which is an under-derivation: a stale witness passes.
- What `go list -deps -test` can and cannot establish:
  - It CAN list each in-graph package's Go, cgo, C/assembly, ignored, test and `//go:embed` files.
  - It CANNOT see:
    - files a test opens at run time: fixtures, `testdata/` that is not embedded, `../` climbs,
      the repository root;
    - headers a cgo `#include` pulls from outside the package directory;
    - files whose generator `go test` does not run;
    - a `go.work` or `GOFLAGS` the row's environment supplies;
    - the module cache and the toolchain (environment, outside the tree);
    - anything a test reads by running `git`.
  - Build tags change which files compile, but go/build still reads every Go file's header, so
    the listed files are read whatever the tags.
  - Conclusion: the list is a sound LOWER bound on what a go run reads and an unsound upper bound.
    This brief uses it only as the floor below.
- History reads. A row that runs `git` (42 refused rows by command text), `statusgen` (86, most
  of which read trailers), or a go test that asks git for the toplevel reads the git directory.
  So does `go build` or `go install` of a main package inside a repository, which stamps version
  control data by running git unless `-buildvcs=false` is set. The witness revision's own `HEAD`
  differs from the item revision's, so a history-reading row can be judged only at its exact
  revision. 28 of the 36 refused briefs have at least one such row by command text. Count go
  tests that shell out and the number is higher. A recorded read set does not release those
  briefs. They keep today's route.
- Witnesses reach main through the evidence-landing verb, which today commits one file per call
  (`tools/desk/cmd/deskevidence/main.go`). verifyrun computes a brief run's tree once, before its
  rows run, and exempts only that brief's own Evidence edit from `+dirty`
  (`briefTreeSHA`, `statusgen/verifyrun.go`). Any other uncommitted change in the tree, including
  another brief's Evidence, makes the witness `+dirty`.
- single-point-of-failure: the recorder's completeness, and for a forged record, the binding of
  the read set to the run that wrote it. Coverage releases a witness for any path outside the
  read set, so a read the recorder misses, or a read set attached afterwards, is a stale pass.
  Layers behind it:
  - The go compile-input floor. For every go build-family invocation the trace records, the
    statusgen adapter computes `go list -deps -test` at the witness revision with that
    invocation's own arguments, and any in-tree file it lists that the recorded set lacks is
    refused. That is a static dependency graph computed by a different component at evaluation
    time, not a run-time trace. The library fails closed when the observation is missing.
  - Default-deny recording. The recorder traces every system call except a fixed list known to
    take no path and acquire no descriptor. Any call or read it cannot attribute writes no read
    set, and the witness says `could-not-check` for its read set.
  - Introduction binding. A read set counts only when its token and its file were both present in
    the commit that first introduced the witness row (row identity: Task step 5).
  For a non-go row the floor is NONE: no static derivation exists for an arbitrary command. The
  default-deny rule is the guard there. A sampled re-run at the main head (Task step 9) checks
  the batch once, before any of it lands. A standing second layer for non-go rows is not built
  here.
  Residual, unchanged by this brief: a witness row's own authenticity sits upstream, with the
  landing identity. A forged row that carries its own read set would speak for every tree that
  leaves its recorded paths alone, where today it speaks for one tree.

design-fit:
  owner: statusgen coverage library (`statusgen/evaluation/coverage_api.go`, `EvaluateCoverage`), fed by the statusgen adapter (`statusgen/coverage.go`)
  contract: S-evidence-coverage
  retires: []   # the command-text parser and its .gitattributes-ancestor rule retire in a later brief, gated on Task step 10's measurement
  weight: verbs 0, flags 0, refusals +5 (unreadable read set, floor not covered, go run with no floor, history read at a different revision, a landing whose read-set file is absent), rule-text lines +0 (no skill or kit text changes)
  why-add: each refusal is the fail-closed branch of a narrowing that does not exist today; the alternative is the parser, which narrows only for shell readers and is retired once this measures it redundant
  relation: the read set is a new observation (`CoverageInput.ReadSets` and `CoverageInput.GoFloors`), not a dependency manifest. A manifest names a claim's declared dependencies by kind and stays graph-execution/30's input. When a claim has both, the read set replaces only the source scope and the manifest's policy, build and environment checks still run; the source scope is the union, so a manifest can only widen it.

## Human decision
Should a verification witness be judged only against the files its run actually read? Today any
later change anywhere in the repository makes every earlier witness stale, so no verified item
reaches done without being run again. The proposal narrows that check, and narrowing a
verification control is a human call.

Options:
1. **Recorded read set with a static floor (recommended).**
   - The verify runner records every file each row reads (Linux only). An item stays released
     while none of those files changed.
   - For go test rows, the evaluator also requires the record to include every file the go
     toolchain lists as compiled. A recorder that missed one is caught by a different signal.
   - Anything the recorder cannot attribute, and any run on a host that is not Linux, is marked
     could-not-check for its read set and keeps today's whole-tree check. So does any row that
     reads repository history; those items stay on today's manual route.
   - The verify desk re-runs about 133 already-witnessed items once, at one main revision.
   - The command-text guesser stays until a measurement shows it adds nothing.
2. **The same, plus a derived input set for existing go rows with no re-run.** This releases
   none of the 36 items held today: each has other row shapes or edited or dirty witnesses. It
   also misses files that tests open at run time, so a stale result can pass.
3. **No new mechanism.** The verify desk re-runs the rows at the head of main before each flip.
   The check is not narrowed; every unrelated merge still costs a re-run.

Default if no answer: none — blocks until answered.

Ruled 2026-10-26: option 1, recorded read set with the go compiled-file floor and one batch
re-witness at a single main revision, on decision issue #4512 (the `ruling:` frontmatter link).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not run the batch re-witness (Task step 9 names it; the verify desk runs it after merge).
  Do not edit any brief's Verify table, Evidence or witness rows.
- Do not remove or weaken `deriveRowInputs`, the command-text guard, the Expect guard, the
  landing check, or the `+dirty` / `+unknown` conservative scope. The one change to `+dirty`
  that is in scope is Task step 9's last paragraph: a brief's own read-set files join its
  own-brief exemption, exactly as its own Evidence already does.
- No flag, environment variable or override may switch the floor, the default-deny recording or
  the introduction binding off.
- Pure verification: every new check is a closed-form comparison over recorded sets and trees.
  Add no runtime assertions to the code under test.

## Task
1. **Freshness.** On fresh `origin/main` re-read the files in Context and record the commit
   read. If a merged change already fills `Dependencies` from any source, stop and report
   NEEDS_CONTEXT.
2. **Recorder (verifyrun; Linux only).** Run each row under a tracer that follows every child
   process from its first instruction (ptrace-class, with a seccomp filter that traps every
   system call outside a fixed list of calls that take no path and acquire no descriptor).
   `LD_PRELOAD` is not acceptable: Go binaries make raw system calls. The row starts with only
   descriptors 0, 1 and 2 open, and stdin is `/dev/null`.
   - **The limit is Linux.** On any other host verifyrun runs the row as today and writes the
     read-set token `could-not-check: recorder is Linux-only (<os>)`. It never writes an empty
     read set. The file pair `readset_linux.go` / `readset_other.go` carries this, as
     `statusgen/netns.go` does for its facility. The decision itself (which OS names get the
     recorder, and the token text for the rest) lives in `statusgen/readset_platform.go` (planned), a
     file with no build constraint, so it compiles and is tested on every OS (row 28);
     `readset_other.go` only calls it. The recorder's own tests follow
     `statusgen/netns_test.go`'s `requireSandbox`: where the facility is absent (not Linux, the
     host refuses the tracer, or, for row 16, the host refuses the private mount namespace the
     test plants into) the test's FIRST statement, before any subtest, is
     `t.Skipf("SKIP (could-not-check, not a pass): ...")`, so it prints `--- SKIP` and never a
     `--- PASS` line for its name. Where the facility works, the test requires full recording.
   - **Path resolution.** Every path is resolved to an absolute, symlink-free path before it is
     classified. A path relative to a directory descriptor resolves through the tracee's
     descriptor table and working directory. A path through an in-tree symbolic link records
     both the link and its target. A `/proc/<pid>/fd/<n>`, `/proc/<pid>/cwd`, `/proc/<pid>/root`
     or `/dev/fd/<n>` path resolves to its target first.
   - **History first.** Before anything else, the recorder resolves the run's git directory and
     common directory (`git rev-parse --absolute-git-dir --git-common-dir` at the row's start,
     plus any `GIT_DIR` or `GIT_COMMON_DIR` a traced process sets) and the `.git` entry at the
     tree root. Any access to those paths or beneath them, in-tree or not, is recorded as one
     `history` entry. Only then do the untracked and out-of-tree rules below apply. So a linked
     worktree, whose git directory is outside the tree, still records the history read.
   - **What it records**, for in-tree paths:
     - every path opened for reading, mapped, or executed (`execve`, `execveat`); an executed
       path is a read of that file;
     - every path stat-ed or probed (`stat` family, `statx`, `access`, `faccessat`,
       `faccessat2`, `readlink`, `readlinkat`); a stat-ed tracked regular file is judged by its
       content (Task step 5), so a size test or a go test-cache check is covered;
     - every directory listed (`getdents`, `getdents64`); the names of its direct children are an
       input;
     - every path looked up and found absent; its continued absence is an input;
     - for every exec of the go tool: its working directory, arguments, and its `GOFLAGS`,
       `GOWORK`, `GOOS`, `GOARCH` and `CGO_ENABLED` values, as one `go-run` entry.
     A directory collapses to one entry only when every tracked file beneath it was read.
   - **Environment.** Out-of-tree reads under a fixed environment set are not inputs:
     - the distribution-managed system paths `/usr` (but never `/usr/local`), `/bin`, `/sbin`,
       `/lib*`, `/etc`, `/dev`, `/sys`, and the resolved `/proc` entries of traced processes;
     - `go env GOROOT`, wherever it sits (a GOROOT under `/usr/local` counts), but only a file
       not modified since the verify session began. A changed one is outside the environment
       set;
     - `GOMODCACHE` and `GOCACHE`, but only when the reading process is one of the toolchain's
       own binaries (the exec rule below names them). The toolchain re-hashes the in-tree inputs behind every cache entry it
       uses, and those reads are recorded. Any other process reading these directories is
       outside the environment set, so an earlier row cannot hand tree content to a later one
       through them;
     - `$HOME/.gitconfig`, `$HOME/.config/git` and `$HOME/.config/go`, but only a file not
       modified since the verify session began. A changed one is outside the environment set;
     - files a traced process created during this row.
   - **Default deny.** Write no read set, and write the token
     `could-not-check: <reason>`, when any of these happens:
     - a traced system call the recorder does not classify (for example `open_by_handle_at`,
       `name_to_handle_at`, `io_uring_setup`, `io_uring_enter`);
     - a call that reaches into another process without a path: `process_vm_readv`,
       `process_vm_writev`, `ptrace`, `pidfd_open`, `pidfd_getfd` or `kcmp`. None is on the
       path-free list, and any one aimed at a process outside the traced set writes no read set;
     - a connect to a non-loopback address, or to a loopback or local socket whose listener no
       traced process created in this row;
     - a namespace, mount or root change (`unshare`, `setns`, `clone` with a new-namespace flag,
       `mount`, `pivot_root`, `chroot`);
     - an out-of-tree read outside the environment set;
     - an exec of an out-of-tree binary, unless a traced process wrote it during this row (a
       tool built inside the row is covered because its build's reads are recorded; a tool
       built by an earlier row, or one installed on PATH, is not), or unless both of these hold:
       - it is under a distribution-managed prefix (`/usr/bin`, `/usr/sbin`, `/usr/lib*`,
         `/usr/libexec`, `/bin`, `/sbin`), or it is one of the go toolchain's own binaries:
         `$GOROOT/bin/go`, `$GOROOT/bin/gofmt`, or a file in
         `$GOROOT/pkg/tool/<goos>_<goarch>/`. Nothing else under GOROOT is exempt, so a tool
         copied into `$GOROOT/bin` is not. `/usr/local` is never a prefix: this repository's
         CI-runner image installs its own tools in `/usr/local/bin`;
       - its file name is not the name of a `main` package directory or of an executable file
         in the witness tree, inside GOROOT or out, so a copy of a tree tool placed anywhere
         still writes no read set;
     - a recorded in-tree path that is untracked in the witness tree (after the history rule);
     - the tracer loses a process.
     A row that runs a prebuilt tool therefore keeps today's whole-tree scope. So does a row on
     a host that installs a tool the row needs, such as git, under `/usr/local`. To gain a read
     set, the row builds the tool itself (`go build -buildvcs=false -o <tmp> .` then runs it).
3. **Storage.** verifyrun writes the read set to `docs/streams/<stream>/reads/<sha256>`, named by
   the full sha256 of its content. The file starts with a header and then the sorted,
   newline-separated entries:
   - `witness-commit <40 hex>`, the full commit the row ran on;
   - `command sha256:<64 hex>`, the digest of the row's Command text;
   - `brief <repo path>` and `row <n>`.
   The witness row's Runner cell carries the token as a parenthetical after the tree, in the same
   write as the row: `<runner> @ <tree> (reads sha256:<64 hex>)`, or
   `(reads could-not-check: <reason>)`. The existing tree token parser stops at `(`, so the cell
   stays readable by today's code. There is no separate `Read-Set:` line.
4. **One source, three callers.** Add `coverageOptionsFor(root string, patterns []string) coverageOptions`
   in `statusgen/coverage.go`. It reads each witness's read-set token and file and the
   introduction observation (step 5), and computes the go floor for every `go-run` entry
   (step 6). `autoflip.go`, `reconcilefold.go` and `runCoverage` call it. No caller builds
   `coverageOptions{}` itself. `statusgen/domain/assess.go:111` stays as it is; step 6's library
   rule makes it fail closed.
5. **Judging (library).** A read set counts only when all of the following hold. Otherwise the
   witness keeps today's scope and reason, and the claim never narrows:
   - The token's file exists and its content's sha256 is the token's digest. If not, the claim
     is `could-not-check` with the reason
     `the witness's recorded read set cannot be read or does not match its digest`.
   - The file's header matches the witness: its commit is the witness tree's full commit, its
     command digest is the row's Command text, and its brief and row are this row.
   - Introduction binding: in the commit that first introduced this witness row into the brief
     file, the row's Runner cell already carried the same token, and the file existed at that
     commit. The row's identity key is its `#`, Command and Date cells plus its Runner cell with
     the `(reads ...)` parenthetical removed (runner and tree only), so the key never includes
     the token and an edit to a result or output cell does not make a new row. The introducing
     commit is the earliest commit whose brief file holds a row with that key. The adapter
     supplies that commit's row text and file presence as an observation. A token or file added
     by a later commit is ignored, including when that later commit adds both at once.
   - After exempt paths are subtracted, the set holds at least one in-tree path. An empty set, or
     one holding only exempt paths, is ignored.
   Exempt paths: a recorded read of the brief's own file, of `docs/streams/**`, of `STATUS.md`,
   or of any other path verify and regen write, is subtracted by the same function the
   conservative scope uses, so the read-set scope never covers more of the bookkeeping surface
   than today's scope. The claim's reason states how many recorded reads were exempt. The
   brief's own Verify rows stay bound by the Command and Expect guards.
   A counted read set makes the witness speak for exactly its remaining entries. It is stale when
   any of the following holds:
   - a recorded or stat-ed tracked file's blob or mode differs between the witness tree and the
     item tree;
   - a listed directory's direct-children names differ;
   - a path recorded absent now exists;
   - the result of `git check-attr` for `text`, `eol`, `filter`, `working-tree-encoding` and
     `ident` on a recorded path differs.
   So a `.gitattributes` change that alters no recorded path's checkout releases. Exact refusal
   strings, which the tests assert:
   - `the witness's recorded read set cannot be read or does not match its digest` (above).
   - `the row read repository history (.git), so only a witness at the item's exact revision speaks for it`:
     a read set with a `history` entry. Exact revision only.
   Unchanged:
   - A witness with no read-set token, or a `could-not-check` token, keeps today's scope.
   - A `+dirty` or `+unknown` witness keeps the conservative scope, read set or not.
   - The command-text and Expect guards run before any of this.
6. **The go floor (adapter, lower layer).** A row is a go row when its read set has a `go-run`
   entry, whatever the command text looks like: a pipe tail, `go -C`, or a script that runs go
   all count. For each `go-run` entry whose subcommand is `build`, `test`, `vet`, `run`,
   `install` or `list`, the adapter:
   - exports the witness tree to a scratch directory;
   - runs `go list -deps -test -json` there, in the recorded working directory and with the
     recorded `-C` directory when the run had one, with the recorded package arguments, `-tags`
     and `GOOS`/`GOARCH`/`CGO_ENABLED`. Its environment appends `-mod=readonly` to the recorded
     `GOFLAGS` and sets two separate variables, `GOPROXY=off` and `GOTOOLCHAIN=local`, so a
     `toolchain` line in the witness tree's `go.mod` fails closed without a download;
   - collects every in-tree file listed: Go, cgo, C, H, S, ignored, test, x-test and embed files,
     plus each in-tree module's `go.mod` and `go.sum` and any in-tree `go.work` the run used;
   - hands the result to the library as an observation keyed by that entry.
   The library refuses the claim with
   `the recorded read set omits a file the go toolchain compiles for this row` when the recorded
   set lacks any listed file. If `go list` fails, the claim is `could-not-check` (for example,
   the module cache misses a dependency). A read set with a build-family `go-run` entry and no
   floor observation for it is `could-not-check` with the reason
   `the read set records a go build but no compile-input floor was supplied`, so a caller that
   skips the floor fails closed. The floor never widens a release. It only refuses.
7. **Landing (deskevidence).** When a landed Evidence row's Runner cell carries a
   `reads sha256:` token, deskevidence commits the brief and the named
   `docs/streams/<stream>/reads/<sha256>` file in ONE commit. It refuses the landing, before any
   write, when the file is absent locally or its digest differs. The same holds for the
   Evidence-PR flow: the PR adds the row and its file in one commit, because step 5's
   introduction binding ignores a file that arrives in a later commit than its row.
8. **Contract.** In `docs/contracts.md`, rewrite the S-evidence-coverage row's
   independent-layer column to name the compile-input floor, its signal (a static dependency
   graph computed at evaluation time) and its gap (rows that are not go rows). Correct the stale
   comment in `statusgen/evaluation/coverage.go` that says graph-execution/09 supplies manifests:
   name graph-execution/30 as the manifest's owner and this brief's read set as a separate
   observation.
9. **Name the batch re-witness. Do not run it.** Add a section `## Batch re-witness` to this
   brief's stream README that states:
   - Who runs it: the verify desk, a non-implementing identity, with writing-form `verifyrun`
     under the recorder on a Linux host (a container qualifies).
   - Which revision: the first main commit that contains the merge of the implementation PR
     that delivers this Task (not the docs PR that authored this brief), the same one for every
     row.
   - "One fresh detached worktree per brief" at that revision. Each brief's rows run in a tree
     that is clean apart from that brief's own Evidence and read-set files, so no witness is
     `+dirty` from another brief's edits. When its rows finish, the brief's Evidence rows and
     read-set files are committed together, in one commit, to a per-brief branch. Nothing is
     pushed to main or merged yet.
   - Its bound: the briefs at `verified` whose only hold is `wrong-revision`, listed by
     `statusgen --coverage --json` at that revision before the run starts (133 at 5ddf5a317).
     Nothing is added mid-batch.
   - "Nothing lands until the pre-landing checks pass" over the whole batch. The checks run
     after every brief's branch exists, over all of them merged into one batch tree: the dry-run
     release count with the batch's witnesses, recorded in the batch's Evidence PR; and a re-run
     at the main head of ten released claims whose rows are not go rows. This re-run is the one
     check behind non-go rows (Context, single point of failure). Any re-run that disagrees with
     its released witness stops the batch: no brief lands, and the disagreement is filed as a
     finding.
   - Landing: only after both checks pass, through the ordinary Evidence-PR flow. Each brief's
     rows and read-set files keep their one-commit pairing (step 7).
   - Failures: a row that fails takes the ordinary red path and is never re-run to green.
   - Exclusions: history-reading rows and rows that run a prebuilt tool gain nothing from it,
     and the section says so.
   The two quoted phrases appear verbatim in the section, each on its own line (row 27).
   verifyrun's own-brief exemption from `+dirty` extends to that brief's own read-set files named
   by its Evidence, so resuming a brief's run in the same worktree stays clean. Another brief's
   read-set file stays dirty.
10. **Parser measurement (report only).** On the implementation branch, count the claims that
    pass only through `deriveRowInputs` (42 at 5ddf5a317) and put that number in the PR body.
    The parser's retirement is a later brief, opened only once that count is 0 after the batch
    re-witness.

## Verify (executable — no prose-only DoD items)
Baselines: rows 1–14, 16, 17 and 19–29 are fail-first. Each names a test or check this brief
adds, so on the base commit each prints 0 or exits 1. Row 15 compares a build of the
implementation merge against a build of its first parent. Row 18 is the lint. Before-measurement
2 (36 at 5ddf5a317, author-run) is the reference row 15's Evidence sits beside. Before-measurement
1 (35 at 9ab9fa76ab4d, relayed) is context only, and no row asserts against it.
Linux rows: rows 14, 16, 21, 23 and 25 drive the recorder. Run them on a Linux runner (a
container qualifies). Their Expect cells require the output line `1` and the default exit 0;
they carry no unquoted exit number, because verifyrun's `parseExpect` reads the first unquoted
`exit <n>` as the required process exit. Where the recorder did not run, the row cannot pass:
- Off Linux, each command's leading `test "$(uname -s)" = Linux &&` fails, so the row prints
  nothing and exits non-zero. verifyrun records `fail` (exit differs from the expected 0).
- On a Linux host where the facility is absent (Task step 2), the test skips as its first
  statement, before any subtest, so `grep -c` prints `0` and exits non-zero. verifyrun again
  records `fail`.
In both cases the verifier does not land the run as the recorder's proof. The verifier records the row
as `could-not-check` for the recorder, citing the empty output or the test's `SKIP
(could-not-check, not a pass)` line, and rerun it on a host with the facility. Neither case is
ever a `pass`, and row 29 checks that mechanically against these cells. Row 28, which runs on
every host, proves the non-Linux token itself.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetReleasesUnreadPathChange$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetReleasesUnreadPathChange \('` | output is `1` — the test asserts a witness whose read set omits `.claude-plugin/marketplace.json`, `.assay-surfaces` and `.gitattributes` is released (claim `pass`) when the item differs from the witness only in those three paths | check:ci |
| 2 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetRefusesChangedDependency$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetRefusesChangedDependency \('` | output is `1` — the test asserts that, with the row-1 unrelated-path release in place, a one-line change to a recorded file of a depended-on package leaves the claim `wrong-revision` with a reason containing `differs between the witness's tree and the item's and the witness speaks for it` and naming that file | check:ci +mutation |
| 3 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetFloorCatchesUnderRecordedGoInput$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetFloorCatchesUnderRecordedGoInput \('` | output is `1` — LOWER layer with the UPPER bypassed: the read set deliberately omits a depended-on Go file and only that file changed, so the tree-wise comparison alone would release; the test asserts the claim is refused with the reason `the recorded read set omits a file the go toolchain compiles for this row` | check:ci +mutation |
| 4 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetMissingFailsClosed$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetMissingFailsClosed \('` | output is `1` — the test asserts a read-set token whose file is absent, and one whose file has a different digest, each give claim `could-not-check` with the reason `the witness's recorded read set cannot be read or does not match its digest`, and never `pass` | check:ci +mutation |
| 5 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetDirtyWitnessStillRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetDirtyWitnessStillRefuses \('` | output is `1` — the test asserts a `+dirty` witness carrying a valid read set, with only an unrecorded path changed, stays `wrong-revision` with a reason containing `uncommitted edits (+dirty)` | check:ci |
| 6 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetEditedRowStillRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetEditedRowStillRefuses \('` | output is `1` — the test asserts a witness with a valid read set whose row Command was edited after the run is refused with a reason containing `the witness records a different command than the Verify row now carries` | check:ci |
| 7 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetHistoryReadNeedsExactRevision$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetHistoryReadNeedsExactRevision \('` | output is `1` — the test asserts a read set with a `history` entry is refused at any other revision with the reason `the row read repository history (.git), so only a witness at the item's exact revision speaks for it`, and released at the exact revision | check:ci +mutation |
| 8 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetNegativeInputsRefuse$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetNegativeInputsRefuse \('` | output is `1` — the test asserts two refusals: a file added to a directory the row listed, and a file appearing at a path the row looked up and found absent; each leaves the claim `wrong-revision` | check:ci +mutation |
| 9 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetContentAttributeChangeRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetContentAttributeChangeRefuses \('` | output is `1` — the test asserts a `.gitattributes` change that sets `eol` on a recorded path refuses the claim, while one that touches no recorded path releases | check:ci +mutation |
| 10 | `cd statusgen && go test ./evaluation/ -run '^TestLegacyWitnessKeepsConservativeScope$' -count=1 -v \| grep -c -E '^--- PASS: TestLegacyWitnessKeepsConservativeScope \('` | output is `1` — the test asserts a witness with no read-set token, and one with a `could-not-check` token, whose row is `cd tools/x && go test ./...` each stay `wrong-revision` when only `.claude-plugin/marketplace.json` changed | check:ci |
| 11 | `cd statusgen && go test . -run '^TestCoverageCallersApplyFloor$' -count=1 -v \| grep -c -E '^--- PASS: TestCoverageCallersApplyFloor \('` | output is `1` — over one fixture repository the test asserts the auto-flip dry run flips the brief, the reconcile fold folds it, and `--coverage` reports it released for one recorded witness; then, with a compiled file planted missing from the record and only that file changed, all three refuse with the floor reason, the floor computed by `coverageOptionsFor` and not injected | check:ci +flow +mutation |
| 12 | `! grep -n -F 'coverageOptions{}' statusgen/autoflip.go statusgen/reconcilefold.go statusgen/coverage.go` | exit 0 — no caller builds an empty manifest; all three go through `coverageOptionsFor` | check |
| 13 | `cd statusgen && go test . -run '^TestGoFloorFromWitnessTree$' -count=1 -v \| grep -c -E '^--- PASS: TestGoFloorFromWitnessTree \('` | output is `1` — over a fixture module with a cgo file, an embedded file, a build-tag-excluded file and an x-test file, the test asserts the floor lists all four and `go.mod`; that a recorded `-tags` value and package argument reach `go list`; and with a dependency missing from the module cache it asserts `could-not-check` | check:ci |
| 14 | `test "$(uname -s)" = Linux && cd statusgen && go test . -run '^TestVerifyrunRecordsReadSet$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunRecordsReadSet \('` | output is `1` on a Linux runner with the recorder facility (elsewhere the row cannot pass; see "Linux rows" above) — the test runs a row that reads a fixture file through a child process, execs an in-tree script, follows an in-tree symbolic link, opens a file relative to a directory descriptor, lists a directory, stats a file and probes an absent path, and asserts each appears in the written read set (the link and its target both), the Runner cell's token digest matches the file, and the file's header names the full witness commit and the command digest | check:ci +dereference |
| 15 | `M=$(git log --first-parent --diff-filter=A --format=%H -1 -- statusgen/evaluation/readset_test.go) && T=$(mktemp -d) && git worktree add --detach "$T/at" "$M" && mkdir -p "$T/base" && git archive "$M^1" \| tar -x -C "$T/base" && (cd "$T/at/statusgen" && go build -buildvcs=false -o "$T/sg-new" .) && (cd "$T/base/statusgen" && go build -buildvcs=false -o "$T/sg-old" .) && a=$("$T/sg-new" --root "$T/at" --auto-flip-model --dry-run \| grep -c 'REFUSED (stays verified)'); b=$("$T/sg-old" --root "$T/at" --auto-flip-model --dry-run \| grep -c 'REFUSED (stays verified)'); git worktree remove --force "$T/at"; echo "new=$a old=$b"` | one line `new=<n> old=<n>` with the two numbers equal: at the implementation merge, a statusgen built from it and one built from its first parent refuse the same briefs, because no existing witness yet carries a read set. Both numbers are recorded in Evidence beside before-measurement 2 (36 at 5ddf5a317) | gate:model |
| 16 | `test "$(uname -s)" = Linux && cd statusgen && go test . -run '^TestVerifyrunUntraceableReadOmitsReadSet$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunUntraceableReadOmitsReadSet \('` | output is `1` on a Linux runner with the recorder facility (elsewhere the row cannot pass; see "Linux rows" above) — the test asserts each of these writes no read set and a Runner token starting `reads could-not-check:`, never an empty set: a non-loopback connect; a connect to a local socket no traced process listens on; an `unshare`; an `open_by_handle_at`; an out-of-tree read outside the environment set; an exec of a binary built by an earlier row; an exec of a copy of a tree tool placed in `/usr/local/bin`; an exec of a binary in an allowed prefix whose name matches a tree `main` package (both copies planted in a private mount namespace the test sets up before tracing begins); a `process_vm_readv` of an untraced process; a read of a `GOCACHE` file by a process that is not the go tool; and a read of a `$HOME/.config/git` file modified after the session began | check:ci +mutation |
| 17 | `grep -E '^[\|] S-evidence-coverage [\|]' docs/contracts.md \| grep -c -F 'compile-input floor'` | output is `1` — the S-row itself, anchored at the line start, names the new independent layer; an R-row that names S-evidence-coverage as its owner does not match | check |
| 18 | `T=$(mktemp -d) && (cd statusgen && go build -buildvcs=false -o "$T/sg" .) && "$T/sg" --root . --lint` | exit 0; no PROBLEM line names `build-less-brittle/15` | check |
| 19 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetAddedAfterWitnessIgnored$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetAddedAfterWitnessIgnored \('` | output is `1` — the test asserts four cases keep today's scope and stay `wrong-revision` when only an unrecorded path changed: a read-set token and its file added together by one later commit to a witness row an earlier commit introduced (identity key per Task step 5); a read-set file first committed after the row; a file whose header names another row or commit; and a read set holding only exempt paths | check:ci +mutation |
| 20 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetStatComparesContent$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetStatComparesContent \('` | output is `1` — the test asserts a tracked file the row only stat-ed, changed in content with the same size and mode, leaves the claim `wrong-revision` naming that file | check:ci +mutation |
| 21 | `test "$(uname -s)" = Linux && cd statusgen && go test . -run '^TestVerifyrunCachedGoTestRecordsFixtures$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunCachedGoTestRecordsFixtures \('` | output is `1` on a Linux runner with the recorder facility (elsewhere the row cannot pass; see "Linux rows" above) — the test runs a go test that opens a fixture file twice with a shared build cache, the second run served from the test cache, and asserts the second read set still lists the fixture | check:ci |
| 22 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetGoRunWithoutFloorCouldNotCheck$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetGoRunWithoutFloorCouldNotCheck \('` | output is `1` — the test asserts a read set with a `go-run test` entry evaluated with no floor observation is `could-not-check` with the reason `the read set records a go build but no compile-input floor was supplied`, through `EvaluateCoverage` directly as `statusgen/domain/assess.go` calls it | check:ci +mutation |
| 23 | `test "$(uname -s)" = Linux && cd statusgen && go test . -run '^TestVerifyrunHistoryReadInLinkedWorktree$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunHistoryReadInLinkedWorktree \('` | output is `1` on a Linux runner with the recorder facility (elsewhere the row cannot pass; see "Linux rows" above) — in a primary checkout and in a linked worktree whose git directory is outside the tree, the test runs a `git log` row and asserts the read set carries a `history` entry and is not dropped by the untracked or out-of-tree rules | check:ci +mutation |
| 24 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetExemptPathsSubtracted$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetExemptPathsSubtracted \('` | output is `1` — the test asserts recorded reads of the brief's own file, a `docs/streams/` file and `STATUS.md` do not hold the claim when they change, that the reason states the exempt count, and that a non-exempt recorded path still does | check:ci |
| 25 | `test "$(uname -s)" = Linux && cd statusgen && go test . -run '^TestVerifyrunReadSetFilesAndDirty$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunReadSetFilesAndDirty \('` | output is `1` on a Linux runner with the recorder facility (elsewhere the row cannot pass; see "Linux rows" above) — the test asserts a resumed run of a brief over its own uncommitted Evidence and read-set files writes a clean tree token, and a run with another brief's uncommitted read-set file writes `+dirty` | check:ci |
| 26 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestDeskevidenceLandsReadSetWithRow$' -count=1 -v \| grep -c -E '^--- PASS: TestDeskevidenceLandsReadSetWithRow \('` | output is `1` — against a fake forge the test asserts one commit carries both the Evidence row and its read-set file, and that a row whose read-set file is absent locally, or has another digest, is refused before any write | check:ci +flow |
| 27 | `awk '/^## Batch re-witness/{f=1} f' docs/streams/build-less-brittle/README.md \| grep -c -F -e 'One fresh detached worktree per brief' -e 'Nothing lands until the pre-landing checks pass'` | output is `2` — the README's batch section requires a clean per-brief tree and holds every landing until the dry-run count and the sampled re-run pass over the whole batch, each phrase on its own line | check |
| 28 | `cd statusgen && go test . -run '^TestRecorderOffLinuxWritesCouldNotCheck$' -count=1 -v \| grep -c -E '^--- PASS: TestRecorderOffLinuxWritesCouldNotCheck \('` | output is `1` on any host — off Linux the test runs a row through verifyrun and asserts its Runner token is `reads could-not-check: recorder is Linux-only (<os>)` and that no read-set file is written; on Linux it asserts the same through the platform-neutral function `readset_other.go` calls, given a non-Linux OS name | check:ci +mutation |
| 29 | `cd statusgen && go test . -run '^TestBrief15RecorderRowsNeverPassWithoutRecorder$' -count=1 -v \| grep -c -E '^--- PASS: TestBrief15RecorderRowsNeverPassWithoutRecorder \('` | output is `1` — the test reads this brief's Verify table from the repository and feeds the Expect cells of rows 14, 16, 21, 23 and 25 through verifyrun's own `parseExpect` and `verdict`: a run that exits non-zero with no output (the uname guard) and one that exits non-zero printing `0` (the skip) are each not `pass`, and a run that exits 0 printing `1` is `pass`; for row 27 a run printing `1` is not `pass` and one printing `2` is | check:ci +mutation |

<!--
Pre-mortem → detection map (author-brief B5)
| Failure mode of the work | Caught by |
|---|---|
| Recorder misses a read on a go row, stale witness released | row 3 (floor, upper bypassed, library); row 11 (floor through the real callers); row 14 (recorder sees child, exec, symlink and dirfd reads) |
| Recorder misses a read on a non-go row | row 16 for the default-deny channels; row 20/21 for stat-only reads; otherwise review-only — no static floor exists, stated as the SPOF |
| A caller skips the floor | rows 12, 22 |
| Read set attached after the witness, or empty | row 19 |
| Narrowing also releases a real change | rows 2, 8, 9, 20 |
| Existing witnesses quietly released before the re-witness | rows 10, 15 |
| +dirty / edited / history rows loosened | rows 5, 6, 7, 23 |
| Bookkeeping exemptions drift | row 24 |
| Read set file tampered or lost, or never landed | rows 4, 26 |
| Batch makes later witnesses +dirty | rows 25, 27 |
| Batch lands before its sampled re-run | row 27 (README order) |
| Recorder rows read as passed on a host where the recorder never ran | row 29 (the guard and skip outcomes of rows 14, 16, 21, 23, 25 are `fail` under verifyrun's own Expect parser, never `pass`, and the verifier routes them to could-not-check by hand); row 28 proves the off-Linux token |
| Batch-order row passes on half the README | row 29 (row 27 needs the exact count `2`) |
| A tree tool installed under `/usr/local` or renamed into a system path gains a read set | row 16 |
| Batch re-witness run by the implementer or unbounded | Ground rules; README section (step 9) is review-checked |
-->

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter `gate: human`, the narrowing of a verification control). The
reviewer answers both design-fit questions in the verdict: (1) the single control between a
missed read and a stale release is the recorder's completeness, and for a forged record the
introduction binding; is the go floor plus default-deny recording an acceptable guard for go
rows, and is the absence of a floor for other rows acceptable? (2) rows 3 and 11 prove the lower
layer (the floor) refuses with the upper layer (the tree-wise comparison) bypassed, row 11
through the real callers; confirm both plant the omission rather than walking the happy path.
