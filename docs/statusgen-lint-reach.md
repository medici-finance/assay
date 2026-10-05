# What `statusgen --lint` may reach

A separate contract from `docs/telemetry.md`'s opt-in analytics payload: this page states what
`statusgen --lint` — the check that gates every pull request — is allowed to touch on the
network and the process table, and how to find that reach by reading one page instead of
grepping the source for `exec.Command`.

This is forge-neutral/18's Task 7 deliverable: taking `statusgen` off `gh` is only half the
change if nothing states, in one place, what the offline default actually guarantees.

## The contract

| Without `--forge` | With `--forge` |
|---|---|
| **Zero network calls.** No socket is opened to any forge. | Reads go through the desk-tools `deskread` verb, over whatever transport it resolves (HTTPS to the forge's API). |
| **Zero forge processes started.** No `gh`, no `deskread`, nothing on `PATH` is invoked for a forge-backed read. | `deskread` is started — once per read kind per repo SET, never once per repo (forge-neutral/18 Task 1). |
| Every forge-backed check reads **could-not-check**, rendered as itself — never rounded up to a clean pass. | Every forge-backed check reads live data, or could-not-check per repo when `deskread` reports a `partial` result. |
| `docs/streams/.history.jsonl`, brief frontmatter, and the working tree are read normally — these are **local, not forge**. | Same, unchanged. |
| **Claims are not read.** Origin's branch heads (`git ls-remote --heads origin`) are a network read and dead-claim decay is a forge read (`gh pr list`, or the GitLab API), so neither runs; the claim set reads could-not-check (`claim filtering UNAVAILABLE — could-not-check: --lint ran offline …`), and `--require-claims` fails rather than passing blind. No PROBLEM depends on the claim set, so the verdict is unchanged. | The claim read and its decay run as they do when regenerating the board. |

`--forge` is the ONLY thing that swaps the reader. There is no environment variable, no
ambient-credential fallback, and no "try the forge, degrade on error" path: the default reader
(`offlineReader`, `statusgen/forgeread.go`) performs no process start and no network call by
construction — a forge-backed check that has not been wired to handle could-not-check fails to
compile against it, rather than failing at runtime against a live forge that happened to answer.

## The enumerated reads `--forge` performs

Each is one method on statusgen's own `forgeReader` interface (`statusgen/forgeread.go`),
implemented by running `deskread <kind>` once per repo set and parsing its versioned JSON
envelope. The method set is derived from the call sites that consume it — statusgen adds no
speculative read, and no read here widens `deskkit`'s own frozen `Forge` interface
(`tools/desk/internal/deskkit/forge.go`): every kind below maps onto an operation that already
existed, on both backends, before this brief.

| Read kind | statusgen consumer | Forge operation it maps to |
|---|---|---|
| `issues` | the `--lint` issue-debt notice (`openIssueDebtNotice`, opt-in, opens no line without `--forge`) | `Forge.ListOpenIssues` |

The remaining kinds this brief's Task 6 enumerates (a change's head/reviews/check rollup,
merged-change lists, comment lists) are added to this table one row per migrated call site, in
the same change that migrates it — never ahead of a consumer. A reader auditing "does the table
match the tree" runs `grep -rn 'exec.Command("gh"' statusgen/ --include='*.go' | grep -v _test.go
| wc -l` (Verify row 3 of forge-neutral/18): a non-zero count names call sites not yet on this
table.

## `--changed-only`: never the gate

`--lint --changed-only <paths>` is a local pre-push convenience that reuses the same
`changed []string` plumbing `--changed` already has: it demotes a pre-existing defect outside
the named path set, in the DAR-sync, stream-cap, stream-source, register-integrity and
verify-script-diff checks, from PROBLEM to NOTICE. It is **not** a full-tree scope — every
check, including those five checks' own defect-detection, still runs across the whole tree; a
defect outside the named set is never simply absent from the output. It is **not** part of the
reach contract above either — it changes nothing about whether a forge is reached — but it
carries its own hard rule worth stating beside this one: **it refuses outright, non-zero, with
no override, the moment it detects it is running inside the CI gate** (`GITHUB_ACTIONS=true`).
A scoped lint that could serve as the gate is a gate that stops checking the moment someone
finds it convenient; the CI gate always runs the full, unscoped `statusgen --root . --lint`.

## Auditing this contract

1. `statusgen --root . --lint` with every forge/network tool on `PATH` replaced by a shim that
   records its argv and fails (`gh`, `glab`, `deskread`, `curl`, …), `git` wrapped to record
   its argv — must complete with the same verdict, with no shim invoked and no `git`
   subcommand that contacts a remote (`ls-remote`, `fetch`, …). Removing the tools from
   `PATH` is NOT this audit: a call site that looks a tool up, fails, and degrades to
   could-not-check passes that check while still being a forge read wherever the tool is
   installed. `statusgen/lintoffline_test.go` runs this audit over a whole `--lint`, plus a
   refusing `http.DefaultTransport`, and a positive control that shows the shims DO record
   the claim read and decay once forge reads are opted in.
2. `grep -rn 'exec.Command("gh"' statusgen/ --include='*.go' | grep -v _test.go | wc -l` — the
   completion test for forge-neutral/18 in full: `0` means every read in `statusgen/` is on this
   table or does not exist yet.
3. Diff the "enumerated reads" table above against `statusgen/forgeread.go`'s `forgeReader`
   interface — a method with no row here, or a row with no method, is this document drifting
   from the source it describes.
