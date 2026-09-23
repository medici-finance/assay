# deskinbox — port contract (windows-port/13)

Read against the oracle end to end (`plugins/assay/scripts/assay-inbox.sh`, 1,403 lines) at
pickup. This file is the implementer's contract the brief's Task 1 calls for, and the
reviewer's checklist against the oracle (brief §Review, question 1).

## Correction to the brief's own sourced claim

The brief's `sources:` frontmatter states the oracle calls `gh×20, jq×29, make×4,
mktemp×22`. Verified against the primary artifact (`grep -n '\bmake\b'
plugins/assay/scripts/assay-inbox.sh`): **the oracle invokes `make` ZERO times.** Every
hit for the bare word "make" in the script is inside a comment or prose ("must not
make…", "the table mode still costs exactly what it always did… modes make", "would make
a rule about syntax", "a coverage claim the reader does not make") — none of them a
`make <target>` shell-out. The brief's Task 1 ("the four `make` invocations: identify each
target at pickup") and its Review question 2 ("do the four make targets resolve to the same
underlying tool calls") are therefore inapplicable as written — there is nothing to port on
that axis. This is flagged per the dispatch's verify-before-apply rule (disagreeing with a
desk-issued claim after checking the primary artifact is the expected outcome, not
insubordination) rather than silently ignored or silently "fixed" by inventing four make
targets that do not exist.

The `gh×20` / `jq×29` / `mktemp×22` counts were not re-verified line-by-line (they are not
load-bearing for this port — the port replaces the whole gh+jq engine, not a subset of its
calls), but the `make×4` claim specifically does not survive a direct check.

## Modes and their contracts (from the oracle's own `--help`, re-read at pickup)

| Mode | Invocation | Behaviour | This PR |
|---|---|---|---|
| table (default) | `deskinbox [owner/repo ...]` | one row per queue item, oldest-urgent-first | **ported** |
| walk | `deskinbox walk [--item K] [owner/repo ...]` | ONE item in the five-part decision format (Header/Context/Options/Reply shape/Verification); prints item 1 by default | **ported** |
| html | `assay-inbox.sh --html OUT.html [owner/repo ...]` | the whole queue as self-contained HTML cards in the same five-part format, PLUS the flow section | follow-up (windows-port/15) |
| flow | `assay-inbox.sh --flow [--root PATH ...] [--since YYYY-MM-DD]` | the pipeline flow model as a terminal table | follow-up (windows-port/15) |
| flow --html | `assay-inbox.sh --flow --html OUT.html` | the flow model as an inline-SVG stage diagram | follow-up (windows-port/15) |

**Why the split.** The dispatch brief pre-authorizes splitting an oversized port and
keeping only the piece mid-implementation. table+walk share ONE engine (repo resolution,
label-filtered issue query, dedupe/rank/sort — query.go) and ONE format builder
(format.go, shared by walk today and by html in the follow-up, exactly as the oracle
shares write_format_program between its own `--walk` and `--html`). html additionally
needs the self-contained-page renderer; flow needs an entirely separate reader
(`statusgen --bottleneck/--intake-debt/--net-flow`, `deskboard throughput`) and an
inline-SVG diagram builder. Landing table+walk+the shared format builder is a complete,
independently useful, independently testable increment — walk is the `ask-decision`
skill's actual entry point — where landing all five modes in one PR would have meant one
untested giant diff.

## Flags this PR implements

- `--item K` (implies `walk`; 1-based; out of range is a refusal, never a silent empty)
- `-h` / `--help`
- `--version`
- repo positional args, else `./.assay/repos.txt`, else the cwd's `origin` remote

## Flags this PR refuses (not silently ignored)

`--html`, `--flow`, `--root`, `--since` — each prints a message naming the oracle as the
fallback (`bash plugins/assay/scripts/assay-inbox.sh <flag> ...`) and exits refused (5).
No flag is silently accepted and no-op'd.

## Mechanism divergences from the oracle (deliberate; observable behaviour unchanged unless noted)

1. **Identity.** The oracle authenticates as whatever the ambient `gh` CLI keyring holds.
   `deskinbox` authenticates as this session's minted App token
   (`deskkit.ForgeFor`/`SessionTokenRole`), the same seam `cmd/deskboard` and
   `cmd/issueboard` already use. This is a fix, not a drift — the ambient-identity failure
   mode is documented at `internal/deskkit/roletoken.go`'s header. **Observable
   consequence:** `deskinbox` requires `$DESK_LOOP` to be set (the same requirement every
   other migrated desk read carries); a bare terminal invocation outside a booted desk
   window gets `RequireLoopIdentity`'s refusal naming the fix (`export DESK_LOOP=<loop>`),
   where the oracle would have used whatever `gh` was logged in as. This is a known,
   accepted gap for the "any terminal" use case the `assay:inbox` skill markets — the
   skill's own primary callers (`ask-decision`, and a human running it from inside a
   booted desk window) already carry a loop identity.
2. **Issue query.** The oracle issues one `gh issue list --label <L>` call per label per
   repo (4 calls/repo), capped at `--limit` (default 500) per label, and unions the
   results. This reads each repo's open issues ONCE via the resolved forge's
   `ListOpenIssues` (the same frozen op `cmd/issueboard` already consumes) and filters to
   the four labels client-side. The union is identical; the truncation THRESHOLD differs
   (oracle: 500 per label per repo; this port: 10,000 open issues total per repo,
   `forgeMaxIssuePages*forgeIssuePerPage`) — the two mechanisms disagree only on a repo
   with more open issues than that, labelled or not, which none of ours are.
3. **Comment bodies (walk mode's "latest desk note").** No typed `Forge` op returns
   comment BODY text (`ContentEvent`, the trust-gate read, deliberately carries only
   author+time). Rather than widen the frozen `Forge` interface — which would mean
   implementing and golden-pinning a GitLab discussion-notes mapping this brief does not
   need — `detail.go` keeps its own small, package-local, **GitHub-only** REST reader for
   comments, the same shape `cmd/deskpost`'s `ghClient` already uses for reads the
   interface does not cover. The oracle itself only ever worked against GitHub (`gh issue
   view`), so this is not a narrowing in practice: a non-GitHub repo's detail fetch
   reports `Unavailable` (the SAME could-not-check/unread state the oracle renders when
   its own detail fetch fails), never a crash or a silent partial render. A GitLab
   discussion-notes comments reader is a natural, separately-sized follow-up if/when a
   GitLab-hosted repo needs walk/html.
4. **Exit codes.** The oracle's own taxonomy (0 ok / 1 precondition / 2 partial) is
   bespoke to this one script. `deskinbox` uses the shared `deskkit` taxonomy every other
   desk verb in this tree uses: 0 ok, 5 refused (bad arguments/preconditions), 6
   unverifiable (a repo's read failed — output, if any, is PARTIAL). Mapping: oracle 1 →
   `deskinbox` 5; oracle 2 → `deskinbox` 6.

## Parity-testing approach

`format_parity_test.go`'s `TestParityWalk` extracts the oracle's own
`write_format_program()` jq heredoc VERBATIM at test time (no hand-copied second
expectation to drift) and runs it through the system `jq` binary on identical fixture
input, asserting the Go `buildRendered` port byte-for-byte against the real jq program's
output — independent of `gh`'s wire format entirely, so it needs no network, no recorded
HTTP fixtures, and no bash 3.2 environment. It requires `jq` on the runner (the oracle's
own hard dependency); its absence is a `t.Skip` naming why (could-not-check), never a
silent pass.

`query_test.go` / `table_test.go` / `repos_test.go` / `main_test.go` cover the parts the
jq extraction does not reach: the repo-resolution order, the dedupe/rank/sort ordering,
the display-hygiene `clean`/truncate rules, and end-to-end exit-code/mode-dispatch
behaviour — all against a fake `Forge` (no network).
