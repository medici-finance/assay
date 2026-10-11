# muhar mutation gate — GitHub Action

A composite action that runs [muhar](../../tools/desk/cmd/muhar/) over one or more
mutation specs and turns its verdict into a CI step: pass, fail, or "harness broken —
not clean". muhar plants each mutation (a one-site edit that disarms a guard), runs your
test suite, and reports whether the suite CAUGHT it.

```yaml
- uses: medici-finance/assay/actions/muhar@<full-commit-sha>  # pin by full commit SHA
  with:
    spec: mutation/fee-guards.json
```

**Pin by full commit SHA**, never by branch or tag. A tag can be moved; a commit cannot.
The action also builds muhar from source at the commit you pin, so the SHA fixes both the
action's logic and the muhar binary it runs.

## What fails the step

| muhar outcome | Step | `result` output |
|---|---|---|
| Exit 0, every mutation CAUGHT | passes | `clean` |
| Exit 0, any NOT CAUGHT | fails (warning only if `fail-on-not-caught: false`) | `not-caught` |
| Exit 0, any could-not-mutate | fails (warning only if `fail-on-could-not-mutate: false`) | `could-not-mutate` |
| Exit 0 but no single readable `Totals:` line | fails — could-not-check, not clean | `error` |
| **Exit 2 — HARNESS BROKEN** (red baseline, or the positive control was not caught) | **always fails, exit 2, "harness broken — not clean"** | `harness-broken` |
| Exit 1 — usage, spec or IO error | fails | `error` |
| Any other exit (e.g. 130 on a signal) | fails with that exit code | `error` |
| A bad input or a failed build, before muhar runs | fails | `error` |

Exit 2 is never a pass. No input turns it off: a broken harness means none of the
run's verdicts can be trusted, so "nothing was NOT CAUGHT" says nothing.

could-not-mutate fails by default for the same reason: those guards were never tested.
Treating it as clean would let a stale spec (whose `old` text no longer matches) pass
forever.

With several specs, every spec runs; one broken spec makes the whole step exit 2.

muhar also exits 2 when it rejects a flag before any baseline runs. The action still
treats that as "harness broken — not clean" (exit 2), but its error then says muhar
printed no HARNESS BROKEN report and points at muhar's stderr instead of at the baseline.

## Inputs

| Input | Default | Meaning |
|---|---|---|
| `spec` | (required) | Spec path(s), one per line, relative to `working-directory`. Blank lines and `#` lines are skipped. |
| `working-directory` | `.` | Where muhar runs. |
| `jobs` | `1` | muhar `-j`: mutations in flight at once, `0` to `9999`. `0` sizes to the CPU count (capped by muhar). |
| `shard` | empty | muhar `-shard i/n` (0-based) for a matrix leg. Empty runs the whole spec. |
| `fail-on-not-caught` | `true` | `false` reports NOT CAUGHT as a warning instead of failing. |
| `fail-on-could-not-mutate` | `true` | `false` reports could-not-mutate as a warning instead of failing. |
| `setup-go` | `true` | Install Go with `actions/setup-go` (pinned by SHA inside this action). Set `false` when the runner already has the Go your suite needs on `PATH` — for example a self-hosted runner without access to the Go download CDN. |
| `go-version` | empty | Go version for `actions/setup-go`. Empty uses the version in this action's own `tools/desk/go.mod` (at the commit you pinned), the toolchain muhar is built with. |
| `muhar-version` | empty | **Reserved, not implemented.** Meant for installing a pinned muhar release binary instead of building from source. Any non-empty value fails the step with "installing a release binary is not implemented yet". |

muhar builds with Go; your test command (the spec's `test`) runs with whatever is on
`PATH`. If your suite needs a different Go than muhar's, set `go-version` to yours (muhar
builds with any recent Go) or set up your own toolchain and pass `setup-go: false`.

**Runners.** The action's steps run under `bash`. It is exercised on Linux and macOS
runners; Windows runners are untested.

## Outputs

| Output | Meaning |
|---|---|
| `caught` | Mutations CAUGHT, summed over every spec. |
| `not-caught` | Mutations NOT CAUGHT, summed. |
| `could-not-mutate` | Mutations that could not be planted, summed. |
| `result` | `clean`, `not-caught`, `could-not-mutate`, `harness-broken` or `error`. |

The action sets the three counts only when every spec ran healthy. After a broken or
errored run it leaves them unset — "no verdict", never zero. Use `result` to branch. (The
suite under test runs without the workflow file-command variables — `GITHUB_OUTPUT`,
`GITHUB_ENV`, `GITHUB_PATH`, `GITHUB_STATE`, `GITHUB_STEP_SUMMARY` — so it cannot set
these outputs by accident. That is not a boundary against hostile code: see below.)

The counts come from muhar's own `Totals:` line, read exactly as muhar prints it:

```
Totals: 3 caught, 1 NOT CAUGHT, 0 could-not-mutate.
```

## Trust model — read before wiring it to pull requests

muhar runs each spec's `test` command with `sh -c`, with the job's environment. In a
`pull_request` job that is code the pull request's author wrote — the spec, the suite and
the source being mutated all come from the PR head. So:

- Never run this action under `pull_request_target` or `workflow_run` with the PR head
  checked out: that hands PR code your repository's secrets and a write token.
- Keep the job's `permissions` read-only (`contents: read`) and give it no secrets.
- The gate is not a boundary against a PR that edits its own spec or suite: a PR can
  delete a mutation or weaken the test that catches it. Protect the spec paths (for example
  with CODEOWNERS), or read the specs from the base ref, if that matters to you.

## Job summary

Each spec gets a section in the job summary with muhar's full report — the baseline and
positive-control line, every per-mutant verdict, and the `Totals:` line — followed by a
table of every spec's result. When muhar exits non-zero, the section also carries the
last 40 lines of its stderr, where muhar writes why it stopped. A sharded run is labelled **Partial sweep**: its Totals
cover only that shard's share.

## Example workflows

These are documentation, not workflows this repository runs. Copy one into your own
`.github/workflows/` and replace `<full-commit-sha>`.

One spec, defaults (NOT CAUGHT and could-not-mutate both fail):

```yaml
name: mutation gate
on: [pull_request]
permissions:
  contents: read
jobs:
  muhar:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<full-commit-sha>  # pin by SHA
      - id: muhar
        uses: medici-finance/assay/actions/muhar@<full-commit-sha>  # pin by SHA
        with:
          spec: |
            mutation/fee-guards.json
            mutation/auth-guards.json
          jobs: "0"
      - if: always()
        env:  # pass outputs through env, never spliced into the run line
          RESULT: ${{ steps.muhar.outputs.result }}
          CAUGHT: ${{ steps.muhar.outputs.caught }}
        run: echo "result=$RESULT caught=$CAUGHT"
```

A large spec split across matrix legs (every leg must run for the sweep to be complete):

```yaml
name: mutation gate (sharded)
on: [pull_request]
permissions:
  contents: read
jobs:
  muhar:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        shard: ["0/3", "1/3", "2/3"]
    steps:
      - uses: actions/checkout@<full-commit-sha>  # pin by SHA
      - uses: medici-finance/assay/actions/muhar@<full-commit-sha>  # pin by SHA
        with:
          spec: mutation/all-guards.json
          shard: ${{ matrix.shard }}
```

A runner that already carries Go:

```yaml
name: mutation gate (own Go)
on: [pull_request]
permissions:
  contents: read
jobs:
  muhar:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@<full-commit-sha>  # pin by SHA
      - uses: medici-finance/assay/actions/muhar@<full-commit-sha>  # pin by SHA
        with:
          spec: mutation/fee-guards.json
          setup-go: "false"
```

## How it works

`action.yml` is thin: it optionally sets up Go, then calls
[`muhar-action.sh`](muhar-action.sh) twice — `install` builds `tools/desk/cmd/muhar`
from the action's own checkout (with `GOFLAGS=-mod=readonly`, so a caller's own `GOFLAGS`
cannot change that build), and `run` runs every spec, writes the summary, sets the
outputs and decides the exit code. Every input reaches the script as an environment
variable, never spliced into a shell line.

`muhar-action.test.sh` tests the script offline against recorded muhar reports in
`testdata/` (exit 0, exit 2, exit 1, a partial shard, a missing or doubled Totals line, a
mutation named like a Totals line, and the input checks). It runs inside the regular
`go test` of `tools/desk` (`cmd/muhar/action_test.go`), which also feeds the parser the
text muhar's report code really prints and runs `run` against a freshly built muhar — so
a change to muhar's output or flags that the action would misread fails muhar's own
tests.
