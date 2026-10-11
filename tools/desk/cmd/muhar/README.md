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
repository; `latest` moves.

To check the archive against the release's `checksums.txt` before you extract
it:

```sh
TAG=v1.0.39 P=linux-amd64
curl -fsSLO https://github.com/medici-finance/assay/releases/download/$TAG/desk-tools-$P.tar.gz
curl -fsSL  https://github.com/medici-finance/assay/releases/download/$TAG/checksums.txt \
  | grep " desk-tools-$P.tar.gz\$" | sha256sum -c -     # macOS: shasum -a 256 -c -
tar -xzf desk-tools-$P.tar.gz ./muhar
```

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
| `test` | yes | The suite command, run with `sh -c`. A **non-zero exit means the suite failed**, which for a mutation means `CAUGHT`. It may use pipes and `&&`. |
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

A mutation becomes `COULD_NOT_MUTATE`, and the suite is not run for it, when:
`old` is empty, `old` equals `new`, `old` is not in the file, the file cannot be
read, stat'ed or written, or the bytes read back from disk after writing are
not the intended mutant. When the control fails any of these checks, the whole
run is `HARNESS BROKEN`.

After each mutation the original bytes are written back, whatever the verdict.
(One exception: see [Interrupting a run](#interrupting-a-run).)

## Flags

```
  -j int
    	mutations in flight at once (1 = sequential, in place; 0 = one per CPU, capped at 2 to bound peak memory) (default 1)
  -shard string
    	run only this invocation's share of the spec: "i/n" (0-based shard i of n; the n shards partition the spec's mutations). Baseline + control still run in full per shard.
  -spec string
    	path to the mutation spec JSON
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
- `-j 0` means one worker per CPU, **capped at 2**. Each worker holds a full
  copy of the tree, so memory grows with the number of workers. An explicit
  `-j N` is used as given. A negative value exits 1.
- The verdicts, their order and the report text are the same for every `-j`;
  only the wall time changes. `muhar` prints
  `muhar: N mutations in flight, one isolated copy of <dir> per worker` to
  stderr when it runs in parallel.
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
| `CAUGHT` | The mutation was proven on disk and the test command exited non-zero. | Nothing: the behaviour is tested. |
| `NOT_CAUGHT` | The mutation was proven on disk and the test command exited 0. | A real finding. Add a test that fails with this edit in place, then re-run. |
| `COULD_NOT_MUTATE` | The edit could not be planted or proven, so the suite was not run. The reason is printed in brackets. | Fix the spec (usually `old` no longer matches the code). This is **not** a pass: that behaviour was not checked. |
| `HARNESS BROKEN` | The baseline was red, the control was not caught, or a `-j` copy could not be made. | Fix the suite, the control or the environment. Ignore everything else about this run. |

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
- With `-j 1` the mutations are written into your own tree and restored after
  each one. Commit or stash your work first, so `git status` after a run shows
  anything left behind.
- `muhar` makes no network calls and writes nothing outside the tree under test
  and its temporary directories. Whatever your test command does is up to the
  test command.

### Interrupting a run

When `muhar` receives SIGINT or SIGTERM, it deletes its temporary build cache
and exits `130` straight away. With `-j 1`, a mutation that is in place at that
moment is **not** restored: the mutant stays in your working tree. After
interrupting a sequential run, check `git status` (or `git diff`) and restore
the file. With `-j` > 1 the mutants live in the temporary copies, so your tree
is not touched, but the copies themselves (`muhar-<n>-*` directories under the
system temporary directory) are left behind; delete them. (Tracked as
[#2646](https://github.com/medici-finance/assay/issues/2646).)

## See also

- [`tools/desk/README.md`](../../README.md), the index of the desk tools
  `muhar` ships with.
- <https://muhar.assay.guide>, the standalone docs site (being built).
