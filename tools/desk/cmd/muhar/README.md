# muhar: mutation testing that proves each mutation landed

`muhar` checks whether your tests would notice if a piece of code were broken.
You list small edits ("mutations") that each break one behaviour: a guard
removed, a boundary moved, a constant changed. `muhar` plants each edit, runs
your test command, puts the file back, and reports whether the suite went red.

A mutation the suite does not catch is a real finding: that behaviour has no
test that would notice it breaking. The hard part of a mutation harness is
making sure that finding is real. If the edit silently fails to land, or the
suite cannot see the source, the suite stays green, and that looks exactly like
a missing test. `muhar` is built so this cannot happen without being reported:

- **Every edit is proven on disk.** The edit has to match, change the file, and
  read back from disk as the intended bytes before the suite runs. If any of
  that fails, the verdict is `COULD_NOT_MUTATE`, never `NOT_CAUGHT`.
- **Three verdicts, never two.** `CAUGHT`, `NOT_CAUGHT`, `COULD_NOT_MUTATE`. An
  edit that could not be planted is reported as itself.
- **A positive control runs every time.** The spec must name one mutation the
  suite is known to catch. If it is not caught, the whole run is thrown away as
  `HARNESS BROKEN` (exit 2), and no per-mutation verdict is printed.
- **A baseline runs first.** The suite must pass on unmodified source. A suite
  that is already red would make every "caught" meaningless, so that run is
  thrown away too.

The test command is any shell command, so `muhar` works on code in any
language. Go is only needed to build it, and for `-j` > 1 on a tree whose
`go.mod` has `replace` directives (see [Parallel runs](#parallel-runs--j)).

The standalone docs site is being built at <https://muhar.assay.guide>.

## Install

### From a release (no Go needed)

Every umbrella release of this repository ships `muhar` inside the
`desk-tools-<platform>.tar.gz` archive, next to the other desk tools. The
release workflow builds every directory under `tools/desk/cmd/` into that
archive. Platforms: `darwin-arm64`, `darwin-amd64`, `linux-amd64`,
`windows-amd64`, `windows-arm64`. To extract just `muhar` into the current
directory, pinned to a release tag:

```sh
curl -fsSL https://github.com/medici-finance/assay/releases/download/<tag>/desk-tools-linux-amd64.tar.gz | tar -xz ./muhar
```

Replace `<tag>` with a tag from the
[releases page](https://github.com/medici-finance/assay/releases) (for example
`v1.0.39`), and `linux-amd64` with your platform. Swap
`releases/download/<tag>` for `releases/latest/download` to take whatever the
newest release is. Pinning a tag is the supported way to consume this
repository; `latest` moves. This one-liner does not verify the download. The
recipe below does, and it is the one to use in scripts.

To extract `muhar` only if the archive matches the release's `checksums.txt`:

```sh
TAG=v1.0.39 P=linux-amd64
B=https://github.com/medici-finance/assay/releases/download/$TAG
curl -fsSLO "$B/desk-tools-$P.tar.gz" &&
  line=$(curl -fsSL "$B/checksums.txt" | grep " desk-tools-$P.tar.gz\$") &&
  printf '%s\n' "$line" | sha256sum -c - &&
  tar -xzf "desk-tools-$P.tar.gz" ./muhar
```

On macOS without `sha256sum`, use `shasum -a 256 -c -`. Each step runs only if
the one before it succeeded. A failed download, a missing `checksums.txt` line
(the `grep` exits 1), or a hash mismatch stops the chain before `tar`, so
nothing is extracted. It does not use `set -e`, so it is safe to paste into an
interactive shell.

This catches a corrupt or truncated download. It does not catch a replaced
release, because `checksums.txt` comes from the same release as the archive.
To pin a binary, record its sha256 yourself the first time and check against
that. Releases cut by a release workflow that attests its assets also carry a
Sigstore provenance bundle beside each one (`<asset>.sigstore.json`); with the
GitHub CLI, `gh attestation verify <asset> --repo medici-finance/assay` checks
that the bytes were built by this repository's release workflow.

On Windows the archive member is `./muhar.exe`. `muhar` runs the test command
with `sh -c`, so it needs a POSIX `sh` on `PATH` there.

### From source

You need the Go version named in [`tools/desk/go.mod`](../../go.mod).

```sh
git clone https://github.com/medici-finance/assay.git
cd assay/tools/desk
go install ./cmd/muhar          # installs into $(go env GOBIN), or $(go env GOPATH)/bin
```

Inside this repository you can also run it in place, as CI does:
`cd tools/desk && go run ./cmd/muhar -spec <file>`.

### Why not `go install …@version`

`go install github.com/medici-finance/assay/tools/desk/cmd/muhar@<version>`
does not work today, for two separate reasons:

1. **No module tag.** `tools/desk` is its own Go module, nested in the
   repository. The go command looks for a version of a nested module under a
   tag prefixed with its directory (`tools/desk/vX.Y.Z`). Releases are tagged
   `vX.Y.Z`, which belongs to the repository's root module, so `@v1.0.39` fails
   with `module github.com/medici-finance/assay@v1.0.39 found, but does not
   contain package github.com/medici-finance/assay/tools/desk/cmd/muhar`.
2. **A `replace` directive.** `tools/desk/go.mod` replaces a sibling module with
   a path inside this repository. `go install pkg@version` refuses any module
   whose `go.mod` has a `replace`, so `@main` (or any commit) fails with
   `The go.mod file for the module providing named packages contains one or
   more replace directives.`

Use one of the two routes above instead.

## Worked example

A small Go package, a spec with a control and three mutations, the run, and
what to do about each verdict. Everything below was actually run; the output is
copied as printed.

### The code under test

`go.mod`:

```
module example.com/shipping

go 1.21
```

`shipping.go`:

```go
package shipping

// Cost returns the shipping charge, in cents, for an order total in cents.
// Orders of $50.00 or more ship free.
func Cost(total int) int {
	if total < 0 {
		panic("shipping: negative order total")
	}
	if total >= 5000 {
		return 0 // free shipping
	}
	return 499
}
```

`shipping_test.go`, a suite that looks reasonable and passes:

```go
package shipping

import "testing"

func TestCost(t *testing.T) {
	if got := Cost(1000); got != 499 {
		t.Errorf("Cost(1000) = %d, want 499", got)
	}
	if got := Cost(6000); got != 0 {
		t.Errorf("Cost(6000) = %d, want 0", got)
	}
}
```

### The spec

`mutations.json`, next to `go.mod`:

```json
{
  "test": "go test ./... -count=1",
  "control": {
    "name": "CONTROL: free orders are charged",
    "file": "shipping.go",
    "old": "return 0 // free shipping",
    "new": "return 499 // free shipping"
  },
  "mutations": [
    {
      "name": "flat rate drops to zero",
      "file": "shipping.go",
      "old": "return 499",
      "new": "return 0"
    },
    {
      "name": "free-shipping threshold becomes exclusive",
      "file": "shipping.go",
      "old": "total >= 5000",
      "new": "total > 5000"
    },
    {
      "name": "negative-total guard removed",
      "file": "shipping.go",
      "old": "if total <= 0 {",
      "new": "if false {"
    }
  ]
}
```

The control is a mutation the current suite is known to catch: `Cost(6000)`
is asserted to be `0`. The third mutation's `old` text has a typo on purpose
(`<=` where the code says `<`), so you can see what a stale spec looks like.

### The run

From the module directory, with the suite's own output sent to a file:

```console
$ muhar -spec mutations.json 2>suite.log
Harness healthy: baseline GREEN, positive control CAUGHT.
  CAUGHT           flat rate drops to zero
  NOT_CAUGHT       free-shipping threshold becomes exclusive
  COULD_NOT_MUTATE negative-total guard removed  (mutation "negative-total guard removed": Old text not found in shipping.go — edit did not land)
Totals: 1 caught, 1 NOT CAUGHT, 1 could-not-mutate.
NOTE: could-not-mutate is NOT a clean gate — those guards were never actually tested.
$ echo $?
0
```

Line by line:

- `Harness healthy: baseline GREEN, positive control CAUGHT.` The suite passed
  on unmodified source, and the control mutation turned it red. The instrument
  works, so the verdicts below can be trusted.
- `CAUGHT flat rate drops to zero`: with `return 499` changed to `return 0`,
  `Cost(1000)` failed its assertion. This behaviour is tested.
- `NOT_CAUGHT free-shipping threshold becomes exclusive`: with `>=` changed to
  `>`, an order of exactly 5000 cents is now charged, and nothing failed. The
  edit was proven on disk, so this is a real finding: no test pins the
  boundary.
- `COULD_NOT_MUTATE negative-total guard removed`: the spec's `old` text is not
  in the file, so nothing was planted and the suite was not run for it. This
  says nothing about the guard's tests. It means the spec is stale.
- `Totals:` and `NOTE:` summarise the run. The exit code is `0` even though one
  mutation survived: exit `0` means the harness was healthy, not that every
  mutation was caught. See [Using muhar as a CI gate](#using-muhar-as-a-ci-gate).

`suite.log` holds the test command's own output for each run, in order:
baseline, control, then each mutation that was planted (the `<-` notes are
added here, not printed):

```
ok  	example.com/shipping	0.094s                    <- baseline: green
--- FAIL: TestCost (0.00s)                                <- control: caught
    shipping_test.go:10: Cost(6000) = 499, want 0
FAIL
FAIL	example.com/shipping	0.089s
FAIL
--- FAIL: TestCost (0.00s)                                <- "flat rate drops to zero": caught
    shipping_test.go:7: Cost(1000) = 0, want 499
FAIL
FAIL	example.com/shipping	0.083s
FAIL
ok  	example.com/shipping	0.092s                    <- "threshold becomes exclusive": survived
```

There is no entry for the third mutation, because it was never planted.

### Acting on the verdicts

Fix the spec's stale `old` text to `"if total < 0 {"`, and add the two tests
the run showed were missing:

```go
	if got := Cost(5000); got != 0 { // the boundary itself ships free
		t.Errorf("Cost(5000) = %d, want 0", got)
	}
}

func TestCostRejectsNegative(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Cost(-1) did not panic")
		}
	}()
	Cost(-1)
}
```

Run it again:

```console
$ muhar -spec mutations.json 2>/dev/null
Harness healthy: baseline GREEN, positive control CAUGHT.
  CAUGHT           flat rate drops to zero
  CAUGHT           free-shipping threshold becomes exclusive
  CAUGHT           negative-total guard removed
Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.
$ echo $?
0
```

Every listed behaviour now has a test that fails when it breaks.

### What a broken run looks like

If the suite is red before any mutation (here, a test that fails on unmodified
source was added):

```console
$ muhar -spec mutations.json 2>/dev/null
HARNESS BROKEN — run discarded.
  baseline suite is RED on pristine source — cannot distinguish a caught mutation from a pre-existing failure; discard the run
$ echo $?
2
```

If the test command cannot see the behaviour at all (here `"test"` was changed
to `"go vet ./..."`, which never runs the tests):

```console
$ muhar -spec mutations.json 2>/dev/null
HARNESS BROKEN — run discarded.
  positive control was NOT CAUGHT — a mutation the suite must catch stayed green, so the harness is broken (not mutating, or the suite cannot see the source); discard the run
$ echo $?
2
```

Without the control, this run would have reported every mutation as
`NOT_CAUGHT`, which looks like a full set of real findings. A broken run never
prints per-mutation verdicts.

## Spec reference

The spec is one JSON object. Fields not listed here are ignored.

| Field | Required | Meaning |
|---|---|---|
| `test` | yes | The suite command, run with `sh -c`. After the mutant builds, an ordinary non-zero test exit means `CAUGHT`; a directly observed Unix termination/resource-signal death (`SIGKILL`, `SIGTERM`) is non-definitive (see Verdicts); a timeout is also `CAUGHT` and is marked on that mutant's line. It may use pipes and `&&`. |
| `build` | no | Optional build command. When set, it runs before the suite for every mutation, whatever the file. If omitted, a `.go` mutation is checked with `go test -vet=off -c` for its package, carrying the leading `NAME=value` environment assignments and the build flags (`-tags`, `-race`, `-gcflags` and the like) from `test`; test-run flags such as `-run`, `-count` and `-timeout` are dropped. It compiles without executing tests or package initializers. A `test` that is not a plain `go test` command, or that carries a flag `muhar` does not know (or `-C`), uses the mutated package alone, with no flags; set `build` when that needs extra tags or a broader build. A build failure is `COULD_NOT_MUTATE`, and the suite is not run. Non-Go mutations have no automatic build step. |
| `timeout` | no | Object with `multiplier` (default `5`), `floor` (default `20s`) and optional `baseline` limit (default `30m`). The pristine suite must finish within `baseline`. The per-mutation limit is `max(floor, measured baseline × multiplier)`, and each phase of a mutation gets all of it: the build, the suite, and with `schemata` the schema run and any classic fallback. No phase runs on what an earlier one left over. A suite killed at the limit is `CAUGHT`, flagged `TIMEOUT` on its line, because it had the whole limit and did not terminate. A build timeout is `COULD_NOT_MUTATE`. A positive control caught only by a timeout makes the run `HARNESS BROKEN`: the control must turn the suite red, and a suite that asserts nothing still times out when it is slow enough. |
| `control` | yes | A mutation object (below) the suite **must** catch. `control.name` and `control.old` must be non-empty, or `muhar` exits 1 before running anything. |
| `mutations` | no | An array of mutation objects, run in order. An empty or missing array is allowed: the run checks the baseline and the control, then reports `Totals: 0 caught, …`. |
| `root` | no | The directory that relative `file` paths resolve against and that the test command runs in. If it is empty, both use the directory `muhar` was started in. A relative `root` is resolved from that directory too, not from the spec file's location. |

A mutation object:

| Field | Meaning |
|---|---|
| `name` | The label printed in the report. Make it describe the behaviour the edit breaks. |
| `file` | The file to edit. Relative paths resolve against `root` (see above). **Keep it relative** if you use `-j` > 1: see [Parallel runs](#parallel-runs--j). |
| `old` | The exact text to replace, byte for byte, including whitespace. In JSON, write a tab as `\t` and a newline as `\n`. Only the **first** occurrence in the file is replaced, so make `old` long enough to be unique. |
| `new` | The replacement text. It must differ from `old`. |

A mutation becomes `COULD_NOT_MUTATE` when:
`old` is empty, `old` equals `new`, `old` is not in the file, the file cannot be
read, stat'ed or written, or the bytes read back from disk after writing are
not the intended mutant. It also applies when the mutated Go package cannot be
built, or when the direct command child is killed by a termination or
resource signal (`SIGKILL`, `SIGTERM`; Unix only). That rule holds for every run, with or without the host
budget or `-state`. A fault signal such as `SIGABRT` or `SIGSEGV` is an
ordinary test failure, so a crashing mutant is `CAUGHT`. The observation does
not reach through wrappers: if a shell or `go test` turns a signaled
descendant into an ordinary non-zero exit, it is still `CAUGHT`. When the control fails any of these checks, the
whole run is `HARNESS BROKEN`.

After each mutation the original bytes are written back, whatever the verdict.
(Uncatchable kills cannot run restoration; see [Interrupting a run](#interrupting-a-run).)

## Flags

```
  -j int
    mutations in flight at once (1 = sequential, in place; 0 = one per CPU, capped at 2 to bound peak memory)
  -select
    run each mutation against only the tests that execute the mutated function (plus the spec's select.always list); opt-in
  -shard string
    run only this invocation's share of the spec: "i/n" (0-based shard i of n; the n shards partition the spec's mutations). Baseline + control still run in full per shard.
  -spec string
    `path` to the mutation spec JSON (required)
  -state string
    caller-owned resumable JSONL state path
```

`-spec` is required.

### Parallel runs (`-j`)

- `-j 1` (the default) runs everything in place, one mutation at a time, in your
  own tree.
- `-j N` with N > 1 runs up to N mutations at once. Each worker gets its own
  full copy of `root` (or of the starting directory when `root` is empty) in a
  temporary directory, so no two workers ever edit the same file. The baseline
  and the control run in the first copy, which also proves the copy builds
  before any verdict depends on it. `muhar` never makes more copies than there
  are mutations. Copies are deleted when the run ends.
- **One mutation or none means an in-place run.** The worker count is capped
  at the number of mutations selected for this run (after `-shard`). When
  that count is 1 or 0, `muhar` runs in place in your own tree, exactly as
  with `-j 1`, whatever `-j` says. For example, `-j 0 -shard 1/3` on a
  three-mutation spec runs in place. The stderr line below is still printed in
  that case and names copies that are never made. Treat such a run like
  `-j 1`.
- `-j 0` means one worker per CPU, **capped at 2**. Each worker holds a full
  copy of the tree, so memory grows with the number of workers. An explicit
  `-j N` is used as given. A negative value exits 1.
- The verdicts, their order and the report text are the same for every `-j`;
  only the wall time changes. `muhar` prints
  `muhar: N mutations in flight, one isolated copy of <dir> per worker` to
  stderr whenever the resolved `-j` is greater than 1, including the in-place
  case above.
- The copy includes everything under the directory (including `.git`).
  Symlinks are recreated as symlinks; sockets and devices are skipped.
- **Go `replace` directives.** A `replace X => ../../sibling` in a copied
  `go.mod` would point outside the copy and break the build. For every
  `go.mod` in the copy (skipping `testdata` and directories starting with `.`
  or `_`), `muhar` re-points relative filesystem replacements that lead
  outside the tree to the absolute path of the original target, using
  `go mod edit`, and reads the result back to check it. Replacements inside the
  tree, module-version replacements and absolute paths are left alone. This is
  the one place `muhar` needs the `go` command at run time.
- **Use relative `file` paths.** An absolute `file` path is used as-is, so
  under `-j` > 1 every worker edits the same original file while the suite runs
  in its own unmodified copy. The suite then cannot see the mutation, and the
  report shows `NOT_CAUGHT` (or `COULD_NOT_MUTATE` when two workers collide)
  for mutations that are in fact caught. An absolute `file` on the control
  turns this into `HARNESS BROKEN`, but an absolute path on a mutation alone
  does not. With `-j 1` absolute paths work. (Tracked as
  [#2645](https://github.com/medici-finance/assay/issues/2645).)

If a worker's copy cannot be made, the run is `HARNESS BROKEN` (exit 2). It
never falls back to sharing one tree.

### Sharing a host budget

The budget is opt-in. Set **both** `MUHAR_LEASE_DIR` (an existing absolute
shared directory) and `MUHAR_TOTAL_SLOTS` (a positive integer) in every
participating process. For example, after an operator creates a suitable
private shared directory that other local users cannot modify:

```sh
MUHAR_LEASE_DIR=/path/to/shared-muhar-leases MUHAR_TOTAL_SLOTS=2 muhar -j 0 -spec mutations.json
```

The directory and existing slot files are trusted operator inputs. The current
implementation follows slot symlinks and does not enforce directory ownership,
permissions, regular-file type or a maximum total. It has no `O_NOFOLLOW`
hardening; do not use a directory writable by untrusted local users.
Independent roles on the same host must use the same directory and total.
Both unset preserves the existing worker scheduling. Partial, empty, non-positive,
unreadable or invalid configuration refuses before the baseline. There is no
implicit directory, total or change to existing cells. A requested worker pool
larger than the total is clamped and echoed on stderr; the harness still caps
workers at the mutation count and uses one worker without isolated workspaces.
**A total of one changes a requested `-j 0` or `-j N > 1` into an in-place
run**, because the clamp precedes workspace selection. Commit or stash first,
use separate checkouts for independent runs, and check `git diff` after
interruption. Reads outside a lease are not isolated from another run planting
a mutant in the same checkout. No mutation is dropped, reordered or sharded
to fit capacity.

One OS-backed exclusive slot lock admits each independently progressing
worker before its own schemata setup or mutation build. Workspace preparation,
first schemata setup, baseline and control acquire and release their own slot.
Selection runs between the baseline and control leases: its discovery,
coverage builds, coverage tests and pristine-set proofs each acquire their own
slot. A worker never holds one slot while waiting for another. Workers poll
nonblocking locks, and a waiting worker stops when the shared mutation queue
is drained. Empty slot files are harmless: a file's existence proves no holder.

Stderr distinguishes `waiting for a host slot` from command execution and
reports `queue-wait=...` when admitted or when the queue drains. Baseline
measurement and build/test deadlines start after admission; queue time cannot
become a mutation timeout or a `CAUGHT` result. Admission errors and directly
observed termination/resource signal deaths (listed under `COULD_NOT_MUTATE`
above) stay nondefinitive; preflight admission failure
discards the run. A descendant failure translated by a wrapper into an
ordinary non-zero exit remains subject to ordinary verdict classification.

Locks release on completion, harness failure and catchable SIGINT/SIGTERM.
The signal handler kills tracked suite, build and discovery process groups
before ordinary source, workspace and cache cleanup unwinds. Unix kills the
whole group; Windows retains its existing top-process-only kill limitation.
**SIGKILL and OOM kills cannot run cleanup:** OS lock descriptors close, but
suite process groups may continue leaseless until they exit. During that
interval the configured total can be exceeded. Lease descriptors are not
inherited by suite children. Stop or wait for those orphan suites before
starting replacement work. Do not delete live slot files: replacing an inode
can admit two owners of the same slot. For recovery, stop the affected run,
inspect its suites, repair directory permissions/configuration, then retry;
never switch directories to evade a held lease.

Containers are covered only when they explicitly share this same lock
filesystem and compatible OS locking semantics with all participating runs.
Separate container filesystems are **could-not-check** for a host-wide bound.
The budget limits muhar workers, not the internal process count or CPU usage
of an arbitrary suite command or unrelated Go work.

### Splitting one spec across machines (`-shard`)

`-shard i/n` runs only the mutations whose position in the spec (counting from
0) leaves remainder `i` when divided by `n`. For a fixed `n`, the `n` shards
never overlap and together cover every mutation exactly once. The baseline and
the control run in full in every shard, so each shard is trustworthy on its
own, and a broken harness turns every shard red.

Each shard's report covers only its own mutations, and `muhar` says so on
stderr:

```console
$ muhar -shard 1/2 -spec mutations.json 2>&1 | grep -v -E '^(ok|FAIL|---|    |PASS|panic)'
muhar: shard 1/2 — running 1 of 3 mutations (indices ≡ 1 mod 2); the other shards own the rest
Harness healthy: baseline GREEN, positive control CAUGHT.
  CAUGHT           free-shipping threshold becomes exclusive
Totals: 1 caught, 0 NOT CAUGHT, 0 could-not-mutate.
```

You are responsible for running all `n` shards. `i` must satisfy `0 <= i < n`
and `n >= 1`; anything else (`3/3`, `-1/3`, `0/0`, `a/3`, `0 / 3`) exits 1.

## Output

- **stdout** carries only the report.
- **stderr** carries the test command's own stdout and stderr, plus `muhar`'s
  own notes (`-j` and `-shard` lines, and errors).

A healthy run's report:

```
Harness healthy: baseline GREEN, positive control CAUGHT.
  <VERDICT>        <mutation name>  (<reason, for COULD_NOT_MUTATE>)
  …
Totals: <a> caught, <b> NOT CAUGHT, <c> could-not-mutate.
NOTE: could-not-mutate is NOT a clean gate — those guards were never actually tested.
```

The `NOTE:` line appears only when `<c>` is greater than 0. A broken run's
report is only the two `HARNESS BROKEN` lines.

## Verdicts

| Verdict | Means | What to do |
|---|---|---|
| `CAUGHT` | The mutation was proven on disk and its test command exited ordinarily with non-zero status (including a wrapper translating a descendant failure), or ran for the whole per-mutation limit without finishing. A timeout is marked on the per-mutant line. | Nothing: the behaviour is tested, or the mutant did not terminate within its measured bound. |
| `NOT_CAUGHT` | The mutation was proven on disk and the test command exited 0. | A real finding. Add a test that fails with this edit in place, then re-run. |
| `COULD_NOT_MUTATE` | The edit could not be planted or proven, the mutated Go package could not be built, or the direct command child was killed by a termination or resource signal (`SIGKILL`, `SIGTERM`; Unix; every run, with or without budgeting or `-state`). Fault signals such as `SIGSEGV` stay ordinary failures. Descendant signal deaths translated to ordinary exits are not detected here. The reason is printed on the line. | Fix the spec or source. This is **not** a pass: that behaviour was not checked. |
| `HARNESS BROKEN` | The baseline was red or timed out, the control could not be planted, built or caught, or a `-j` copy could not be made. | Fix the suite, the control or the environment. Ignore everything else about this run. |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | The harness was healthy: baseline green and control caught. **This includes runs with `NOT_CAUGHT` or `COULD_NOT_MUTATE` verdicts.** Read the report. |
| `1` | A usage, spec or I/O error before the run started: `-spec` missing, the spec unreadable or not valid JSON, `test` empty, `control.name` or `control.old` empty, a bad `-shard`, a negative `-j`, or the temporary build cache or the current directory could not be read or created. |
| `2` | `HARNESS BROKEN`: the run was discarded and carries no verdicts. Never read this as "the gate is clean". |
| `130` | Interrupted (SIGINT or SIGTERM). |

Two edge cases come from Go's flag parser rather than from `muhar`: an unknown
flag prints `flag provided but not defined` and the usage text, and exits `2`,
the same code as a broken harness; `-h` prints the usage text and exits `0`.
A script that treats exit 2 as `HARNESS BROKEN` can tell the two apart because
a flag error prints nothing to stdout. (Tracked as
[#2646](https://github.com/medici-finance/assay/issues/2646).)

## Using muhar as a CI gate

With `-state`, the state file is trusted verdict input. Never restore it from a
cache or artifact writable by an untrusted branch. Keep it in a private,
caller-owned directory. Its identity digest detects incompatible inputs; it does
not authenticate the observations stored in the file.

Exit 0 only says the instrument worked, so a gate has to check the totals line
too. This is the pattern this repository's own workflows use:

```sh
set -euo pipefail
report=$(muhar -j 0 -spec mutations.json)   # exit 2 (broken harness) stops here under set -e
echo "$report"
if ! grep -qE 'Totals: [0-9]+ caught, 0 NOT CAUGHT, 0 could-not-mutate' <<<"$report"; then
  echo "a mutation survived, or could not be planted"
  exit 1
fi
```

This repository's specs are real-world examples, for instance
[`internal/deskkit/mutations.json`](../../internal/deskkit/mutations.json) and
[`cmd/deskmerge/mutations.json`](../deskmerge/mutations.json). They are run
from `tools/desk` with no `root`, so their `file` paths are relative to
`tools/desk`.

## Environment and side effects

- The test command inherits `muhar`'s environment, except `GOCACHE`. `muhar`
  points `GOCACHE` at a fresh temporary directory for the whole run and
  deletes it at the end, so mutant builds do not pile up in your normal Go
  build cache. `GOMODCACHE` is left alone. A Go suite therefore compiles its
  dependencies from scratch on the first (baseline) run.
- With `-j 1`, and with any `-j` when the run has one mutation or none (see
  [Parallel runs](#parallel-runs--j)), the mutations are written into your own
  tree and restored after each one. Commit or stash your work first, so `git status` after a run shows
  anything left behind.
- `muhar` makes no network calls. It writes to the tree under test, its
  temporary directories and, when opted in, the explicit shared lease
  directory. Whatever your test command does is up to the test command.
- **A spec is code.** Its `test` field runs with `sh -c` with `muhar`'s
  environment. Treat a `mutations.json` like a script. In CI, do not run a
  spec from an untrusted fork's pull request in a job that has secrets.

### Interrupting a run

When `muhar` receives SIGINT or SIGTERM, it kills every tracked suite, build
and discovery command (Unix process groups, children included; Windows only
the top process), then unwinds source restoration, temporary workspaces,
coverage output, the build cache and host leases before returning `130`.
Check `git diff` after an interrupted in-place run, especially if the storage
could not accept restoration. SIGKILL/OOM cannot unwind source restoration or
temporary-directory cleanup; inspect the source and orphan suites before
retrying. During this unwind the first signal handler has already returned,
but signal notification remains registered until the sweep returns: further
SIGINT/SIGTERM signals are absorbed. A wedged cleanup may require SIGKILL,
which cuts cleanup short and has the limits described above. The host-budget
section describes the lock-descriptor limit.
With `-state`, the state lock is released during the same unwind. A completed
state record remains available to a later invocation; an interrupted mutation
has no reusable record.

### Resuming and measuring a sweep (`-state PATH`)

Pass an operator-owned path for this run, outside the tree under test and outside
any disposable per-run directory:

```sh
muhar -spec mutations.json -state "$RUN_STATE_DIR/shipping.jsonl"
# After SIGINT/SIGTERM, repeat the same command and path.
```

The state file is trusted verdict input: never resume from an untrusted branch's
cache or artifact. The opened descriptor must be a regular file; on Unix an
existing group- or world-writable file is refused. Windows access control remains
the caller's responsibility.

There is no manifest state path or shared default. The caller owns retention,
permissions and removal; separate specs, shards, modes or changed inputs need
separate paths. One invocation holds a nonblocking exclusive file lock through
the existing cross-platform desk lock adapter. A second writer refuses before
writing or running a command. Do not replace or remove a locked state file.

The version 1 format is UTF-8 JSONL: a `header` record contains `schema` and a
SHA-256 `identity`; each `phases` record measures this invocation's pristine
`baseline_ns` and positive `control_ns` wall durations, plus control `build_ns`,
`test_ns` and `selection`. Every completed mutation appends a `mutation` record
with its original zero-based spec `index`, literal `verdict`, `complete`,
`build_ns`, `test_ns`, `selection`, optional `error`, and `timeout` classification
(`build` or `test`; absent means no timeout). Test timeouts also retain `limit_ns`
so a resumed report preserves their exact reason. Durations are nanoseconds;
selection describes the full command, selected Go arguments, uncovered skip,
schemata path or schemata fallback into the classic command. Test duration counts
only command execution; coverage-selection decisions are excluded. Shared schemata preparation is outside per-mutation build time.
Each JSON line is written under the writer lock and synced before the mutation
is considered complete. A malformed line or partial trailing line refuses the
whole retry before baseline/control; recover the tail offline or use a new path.

Identity covers the exact spec bytes, canonical root, all source/test/fixture
files under that root (excluding `.git` metadata and the selected state file),
mutation/control files, runner schema, the running muhar executable digest, runner Go version, inherited environment
(excluding disposable cache/temp paths), `-select` and the literal `-shard` mode.
For a plain `go test`, it also hashes the selected package compilation closure's
source, tests, embedded and native files, dependency metadata, Go executable,
the resolved Go environment, compiler/tool binaries and file modes. Generated GCC work-directory flags are excluded; their target/toolchain and inherited CGO inputs remain bound. Build/test commands and their
flags are bound through the spec. The complete-tree input set is intentionally
conservative: even an unrelated fixture edit invalidates reuse. Keep generated
logs/output outside the tree. File symlinks are read through; directory symlinks
refuse because a complete input inventory cannot be proved. `-j` is scheduling
and does not change identity. With `-state`, external-input flags (`-overlay`, `-modfile`, `-pgo`,
`-toolexec`, `-exec`) and unknown Go flags are refused. The same check applies to
GOFLAGS from the environment, a command-local assignment or the go env file.
Go shell compositions (including subshells, substitutions, braces and bang forms)
and command-local PATH overrides also refuse. Set PATH for
the whole invocation, with the chosen Go executable named `go`; absolute or
relative paths to a Go executable are unsupported with `-state`.
Other shell suites use the conservative tree inventory; their external data and
executables must remain stable under the caller's control.

The environment binding is intentionally conservative: shell variables such as
`TERM_SESSION_ID`, `OLDPWD` and `SHLVL` remain inputs because a suite can read
them. A retry from a different shell refuses unless its bound environment is
identical. Use a stable launcher/environment for resumable work; removing unknown
variables would weaken the identity contract. Rebuilding or replacing muhar also
requires a new state path, even when its display version is unchanged.

Every retry reruns baseline, selection preparation and positive control. Only
complete `CAUGHT` and `NOT_CAUGHT` mutation records with no infrastructure error
are reused; a complete test timeout is a definitive `CAUGHT`. `COULD_NOT_MUTATE`,
`HARNESS BROKEN`, interrupted and error observations are rerun. A directly observed Unix suite process killed by a termination or resource
signal (`SIGKILL`, `SIGTERM`) is an
infrastructure error and is not reusable. Fault signals (`SIGABRT`, `SIGSEGV`
and the like) keep their ordinary failed-test classification. A wrapper such as `go test` can convert a
child's OOM or signal death into an ordinary exit 1; that hidden signal is not
detected or qualified by this rule. This tool does not claim OOM detection
through arbitrary wrappers. The final report
combines resumed and new observations in spec order and keeps ordinary stdout
and stderr reporting. An identity/schema mismatch refuses before any mutation.

## See also

- [`tools/desk/README.md`](../../README.md), the index of the desk tools
  `muhar` ships with.
- <https://muhar.assay.guide>, the standalone docs site (being built).
