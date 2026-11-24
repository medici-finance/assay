# `tools/gremlins` — daily mutation discovery

`run.sh` runs [Gremlins](https://github.com/go-gremlins/gremlins) over the desk tools'
muhar-spec packages once a day and reports the mutants the tests did not catch. It is
**discovery, never a gate** (#2647). Nothing it finds fails a build.

## Discovery vs proof

The repo has two mutation tools. They answer different questions.

| | muhar (proof) | Gremlins (discovery) |
|---|---|---|
| What is mutated | A hand-written list per package (`mutations*.json`): each entry names one edit to one guard | Every comparison and `&&`/`\|\|` Gremlins finds in the package |
| What a survivor means | A named guard has no test: a real defect in the suite | A lead. Many survivors are equivalent mutants, or code nobody needs to guard |
| Cost | One targeted `go test -run` per entry | One full package test run per mutant |
| Role | A gate: the release mutation gate and the truth suite run muhar specs | A daily sweep whose output a person reads |

muhar is documented in [`../desk/cmd/muhar/README.md`](../desk/cmd/muhar/README.md) (added by #2648).

The two tools feed each other. A Gremlins survivor in a function no muhar spec touches is
a **candidate guard**. Someone reads it and either:

- writes the missing test plus a muhar entry, so the guard becomes proof; or
- leaves it, because the mutant is equivalent or the code is not a guard.

Gremlins never decides that on its own. That is why the sweep files no issues.

## What a run does

1. **Install and verify.** The pin is `gremlins.sum`, one go.sum line: module, version and
   `h1:` sum. It installs `github.com/go-gremlins/gremlins@v0.6.0` with the
   checksum database forced on: `GOPROXY`, `GOSUMDB` and `GOFLAGS` are pinned, the
   `GONOSUMDB`/`GOPRIVATE`/`GOINSECURE` family is cleared, and `GOENV=off` keeps a
   `go env -w` file from setting them again (go reads an empty variable as unset). The
   module cache and the build cache are fresh directories, so caches already on a
   persistent runner are never trusted in place of the download or its build. It never
   installs `@latest`.
   Then it checks the binary's build info (`go version -m`) for that module, version and
   `h1:` sum. A binary that does not match is refused (exit 2). The upstream binary
   reports `dev` for `--version`, so the version check uses the build info.
2. **Dry run per package.** This lists the runnable mutants per file and times one
   uncached test run (the "Gathering coverage" line).
3. **Plan.** `gremlintriage plan` estimates how many mutants fit the package's budget:
   (budget − coverage time) × workers ÷ coverage time. It packs the files, in name order,
   into bins of that size and picks bin `day mod bins`. All other files are excluded with
   one `--exclude-files` regex. A large package is therefore covered over several days
   instead of the same first files every day.
4. **Copy probe.** This is the positive control for the layout. The package's tests must
   pass in a copy of the module two levels below the root of the swept tree (see "The swept
   tree" below), which is where Gremlins puts every mutant. If they fail there, every "killed" result would be false, so the
   sweep records `probe-red` and counts as broken.
5. **Unleash.** This runs Gremlins with `--workers 2` under `timeout --signal=INT`. Gremlins
   writes no JSON when it is interrupted, so a run stopped at its budget is read back from
   its log and recorded as `partial`. A partial run is still evidence. The log is read up to
   Gremlins' `Shutting down gracefully` line. The mutants printed after it had their
   `go test` cut short by the interrupt, so their status is not counted. A partial run in
   which no mutant finished counts as broken.
6. **Triage.** `gremlintriage triage` merges the reports and cross-references every LIVED
   mutant against all muhar specs in the module:
   - **Lived near a spec:** the mutant is on a line a spec entry mutates, or in the same
     function.
   - **Candidate guard:** no spec entry touches that function. Candidates are listed with
     `file:line:col`, function and mutator.
   - **Did not resolve:** spec entries whose `old` text no longer appears in their file are
     listed as well. A stale spec is a finding too.

## Mutators

Three mutators are on: `conditionals-negation`, `conditionals-boundary` and `invert-logical`.
These are the guard mutations: a flipped or shifted comparison, or a swapped `&&`/`||`.

Three are off: `arithmetic-base`, `increment-decrement` and `invert-negatives`. In these
packages they mostly hit string concatenation and loop counters, and they would roughly
triple the run.

## Usage

```sh
# From any checkout. It is read, never written: the sweep copies it without .git.
# TMPDIR must be outside every git repository (the run refuses otherwise).
bash tools/gremlins/run.sh -o /tmp/gremlins-out                    # all six packages
bash tools/gremlins/run.sh -o /tmp/gremlins-out cmd/desksourceguard  # one package
bash tools/gremlins/run.sh -o /tmp/gremlins-out -b 300 cmd/deskclose:900
```

| Flag | Default | Meaning |
|---|---|---|
| `-o DIR` | (required) | Output directory |
| `-b SECONDS` | 600 | Per-package budget. A `PKG:SECONDS` argument overrides it for that package |
| `-w N` | 2 | Gremlins workers. Sized for the 2-CPU public runner |
| `--day N` | days since the epoch (UTC) | Which bin of a rotated package runs |
| `--root DIR` | this checkout | Checkout whose `tools/desk` is swept. It is copied without `.git`; it is never written |
| `--gremlins BIN` | install the pin | Use an existing binary. The pin check still runs |

**The swept tree.** Gremlins copies the module to `$TMPDIR/gremlins-*/wd-*`. The desk
module reaches two levels up: `go.mod` replaces `../../forgeread`, and tests read
`../../docs`. So `TMPDIR` must be a root laid out like the checkout. `run.sh` makes one: a
copy of the checkout without `.git`, in a fresh directory under the caller's `TMPDIR`. It
runs the copy probe and Gremlins with `TMPDIR` set to that copy, and deletes the copy on
exit. The checkout is never written, so a dirty checkout or leftovers from an earlier run
cannot affect a sweep.

The copy has no `.git` because the packages' tests use the shared git test harness
(`gitquiet.Run`), which refuses to run when `TMPDIR` or `GOTMPDIR` is inside a git
repository. A test that runs git in an empty temporary directory would otherwise discover
the enclosing repository and could write to it. With `TMPDIR` at a checkout root that
refusal would put every such package at `probe-red`. `run.sh` also refuses up front when
the caller's own `TMPDIR` is inside a repository, because the copy would be too.

**Exit codes.**

- `0`: the sweep ran, whatever it found.
- `2`: the sweep broke. This covers a failed install or pin check, a Gremlins failure, a
  failed dry run, a red copy probe, an empty report, and a sweep with no evidence at all.

Outputs in `-o`:

- per package: `<pkg>/{dry.json,dry.log,plan.txt,plan.log,probe.log,report.json,run.log}`
- `manifest.tsv`: one row per package with state, wall seconds, exit code and bin
- `summary.md`: the Markdown summary
- `triage.json`: schema `gremlins-triage/v1`

## Default packages and budgets

The defaults are the six muhar-spec packages named in #2647, swept one after another:

| Package | Budget |
|---|---|
| `internal/deskkit` | 1200 s |
| `cmd/deskmerge` | 600 s |
| `internal/loopengine` | 600 s |
| `cmd/deskclose` | 600 s |
| `cmd/reviewloop` | 600 s |
| `cmd/desksourceguard` | 600 s |

The budgets total 70 minutes. Install, dry runs and copy probes come on top. The daily
staged workflow allows 110 minutes. The dry run, the copy probe and the run each have their
own `timeout` at the package budget, so the worst case is about three times the budgets,
past the job cap. A job that hits the cap fails with "No summary", so it fails closed.

`internal/deskkit` is the outlier. One run of its suite takes minutes, so only a handful of
its roughly 5,600 runnable mutants fit in a day. For deskkit, Gremlins is a slow sample;
muhar's targeted `-run` per entry is the right tool there.

The swept copy has no `.git`, and the sweep's workdirs sit inside it. Several deskkit tests
walk the whole tree or read the checkout's remote: the leak-shape, install-mode and
multipath-TCP scans, the forge-body guard roots and `TestSharedReadLowerScope`. In a
copy-probe of that layout (six failing tests, 2026-11-22) they fail in the baseline, so
`internal/deskkit` is expected to report `probe-red` until those tests stop depending on
`.git` and on what sits beside the module. The other five default packages pass the same
probe. A `probe-red` row is reported as such, never as a clean sweep.

### Measured (2026-10-11)

These are local runs with `GOMAXPROCS=2` to approximate the 2-CPU runner. Wall time
covers the dry run, the copy probe and the run.

| Package | Bin | Wall | Killed | Lived | Not covered | Candidates |
|---|---|---|---|---|---|---|
| `internal/deskkit` | 27/139 (partial) | 1890 s | 9 | 1 | 0 | 1 |
| `cmd/deskmerge` | 4/7 | 211 s | 22 | 0 | 14 | 0 |
| `internal/loopengine` | 2/2 | 258 s | 82 | 42 | 35 | 37 |
| `cmd/deskclose` | 2/2 | 360 s | 142 | 0 | 1 | 0 |
| `cmd/reviewloop` | 1/1 | 243 s | 65 | 0 | 27 | 0 |
| `cmd/desksourceguard` | 1/1 | 104 s | 12 | 8 | 3 | 8 |

That is about 51 minutes in total. deskkit alone is about 31 of them, and it is
budget-bound. The rest finished well inside their budgets. deskkit's counts leave out the
two mutants its log printed after the shutdown line.

The capacity estimate (one coverage run per mutant) is conservative. deskmerge's bin of 22
mutants finished in under four minutes, so its 7-day rotation could be shortened once
runner timings are in.

## Tests

- `run_test.sh` covers this script offline: a stub Gremlins, plus a `go` wrapper that
  answers `go version -m` with canned build info. It is run by `TestRunScriptOffline` in
  `tools/desk/internal/gremlintriage`.
- `gremlintriage` has its own tests, and a muhar spec (`mutations.json` next to it) for its
  break-detection guards.

The staged workflow is `ci/staged-workflows/gremlins-daily.yml`.
