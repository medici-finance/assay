# deskinbox — port contract

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
| table (default) | `deskinbox [owner/repo ...]` | one row per queue item, oldest-urgent-first | **ported** (windows-port/13) |
| walk | `deskinbox walk [--item K] [owner/repo ...]` | ONE item in the five-part decision format (Header/Context/Options/Reply shape/Verification); prints item 1 by default | **ported** (windows-port/13) |
| html | `deskinbox html OUT.html [owner/repo ...]` | the whole queue as self-contained HTML cards in the same five-part format, PLUS the flow section | **ported** (windows-port/15) |
| flow | `deskinbox flow [--root PATH ...] [--since YYYY-MM-DD]` | the pipeline flow model as a terminal table | **ported** (windows-port/15) |
| flow --html | `deskinbox flow --html OUT.html [--root PATH ...] [--since YYYY-MM-DD]` | the flow model as an inline-SVG stage diagram | **ported** (windows-port/15) |

windows-port/15 finishes the port: every mode the oracle offers now has a Go equivalent.
`plugins/assay/scripts/assay-inbox.sh` stays in the tree as the parity oracle the tests
extract programs from — the Ground rules forbid deleting or editing it, and the two parity
suites (`format_parity_test.go` from windows-port/13; `flow_parity_test.go`/
`html_parity_test.go` from this brief) depend on its heredocs staying put.

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

## Flags this PR implements (windows-port/13 + windows-port/15)

- `--item K` (implies `walk`; 1-based; out of range is a refusal, never a silent empty)
- `-h` / `--help`
- `--version`
- repo positional args, else `./.assay/repos.txt`, else the cwd's `origin` remote
- `html OUT.html` (windows-port/15): the output path is a required POSITIONAL, not a
  `--html` flag — see "CLI shape diverges from the oracle" below
- `flow [--root PATH ...] [--since YYYY-MM-DD] [--html OUT.html]` (windows-port/15):
  `--root` repeatable, `--since` validated `YYYY-MM-DD`, `--html` takes the diagram's
  output path

## CLI shape diverges from the oracle: subcommands, not `--html`/`--flow` flags

The oracle spells these two renderings as flags on one invocation (`assay-inbox.sh --html
OUT.html`, `assay-inbox.sh --flow --html OUT.html`). This port spells them as SUBCOMMANDS
(`deskinbox html OUT.html`, `deskinbox flow --html OUT.html`), exactly the shape
windows-port/13 already chose for `--walk` → `walk`. Reasons this stays a subcommand, not a
flag, on this port specifically:

- **`walk` already set the precedent.** A `deskinbox --walk` flag next to a `deskinbox html
  OUT.html` subcommand would be the one command in the tree spelling "which rendering" two
  different ways depending on which rendering.
- **`html`'s output path and `flow`'s cell/window flags are mode-specific grammars.** The
  oracle's own top-level flag loop already treats `--root`/`--since` as accepted-but-inert
  outside `--flow` (assay-inbox.sh:143-148, 226 — `ROOT_ARGS` is read only by
  `resolve_cells()`, which only mode `flow`/`flowhtml` calls). Making `flow` its own
  subcommand with its OWN flag parser (`runFlowCommand`, main.go) means a `--root` typo'd
  onto `table`/`walk`/`html` is refused as an unknown option instead of silently doing
  nothing — a strictly SAFER divergence, not a laxer one.

**Observable consequence for a caller migrating a script**: `assay-inbox.sh --html
inbox.html owner/repo` becomes `deskinbox html inbox.html owner/repo` (drop the leading
`--`); `assay-inbox.sh --flow --html flow.html --root ../a` becomes `deskinbox flow --html
flow.html --root ../a` (drop only the FIRST `--`, since flow's OWN `--html`/`--root` stay
flags under the subcommand). `plugins/assay/commands/inbox.md` and
`plugins/assay/skills/ask-decision/SKILL.md` are re-pointed to the new spelling by this
brief's Task 4.

One quirk carried over deliberately, for parity rather than convenience: like the oracle,
`deskinbox flow`'s own positional arguments (anything not `--root`/`--since`/`--html`) are
accepted and silently ignored, because `flow` resolves CELLS (a statusgen-root axis), never
REPOS — the same non-effect a bare repo token has after `--flow` in the oracle
(assay-inbox.sh:265-292: `ARGS` accumulates it, but `resolve_repos` is never called under
`flow`/`flowhtml`). This is why the brief's own Verify row 8 example (`deskinbox flow
medici-finance/assay`) is a valid invocation at all — the repo-shaped argument does nothing
in either implementation, and cells still resolve from `--root`/`./.assay/cells.txt`/`.`.

## Flags this PR refuses (not silently ignored)

A bare `--html`/`--flow`/`--root`/`--since` FLAG (the oracle's own spelling, not this
port's subcommand spelling) under `table`/`walk` parsing is refused as an unknown option —
see "CLI shape diverges from the oracle" above for why these are subcommands here, not
flags. No flag is silently accepted and no-op'd.

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
   `deskinbox` 5; oracle 2 → `deskinbox` 6. `flow`/`flow --html` fold reader failures into
   this SAME taxonomy (6), matching the oracle's own choice to fold `flow_failures` into
   `query_failures` for those two modes only (assay-inbox.sh:1291-1292) — `html`'s reader
   failures do NOT redden its exit code (divergence point 6 below).
5. **The flow readers' env var names are the ORACLE's, not this tree's own convention.**
   `cmd/deskboard` resolves its own statusgen via `STATUSGEN_BIN`; this port instead uses
   `ASSAY_STATUSGEN`/`ASSAY_DESKBOARD` (flow.go's `statusgenBinEnv`/`deskboardBinEnv`
   constants) — the oracle's own override names (assay-inbox.sh:107-110), quoted verbatim
   in the brief and in `plugins/assay/commands/inbox.md`. This is the one place in this
   package where matching the ORACLE's spelling wins over matching this TREE's existing
   convention, because the contract being ported is the oracle's documented environment,
   not deskboard's.
6. **The board sha (`git rev-parse --short HEAD`) is read via `gitcore.Open` +
   `Repo.Resolve("HEAD")`, never a `git` subprocess.** Verify row 5 scopes every
   `exec.Command` in this package to `statusgen`/`deskboard` — a `git` shell-out for the
   sha line would be the one unscoped site. `gitcore` is the same pure-Go path
   `deskkit.RepoSlugForDir` (repos.go, windows-port/13) already uses for the origin-remote
   read; this is that same discipline applied to a HEAD lookup. A directory that is not a
   git checkout (or has no commits yet) reads `could-not-check` for its sha, exactly as the
   oracle's own `2>/dev/null || printf 'could-not-check'` fallback does.
7. **`html`'s Flow-section reader failures never redden the html exit code; `flow`'s
   own reader failures always do.** This is the oracle's OWN asymmetry
   (assay-inbox.sh:112-118, `finish()`), not a divergence this port introduces — quoted
   here because it is easy to read as inconsistent until the reason is stated: `html`'s
   exit code is a statement about the DECISION QUEUE (did every issue query succeed), and
   a stale `statusgen` must not make a complete, correctly-rendered decision page report
   itself incomplete. `flow`'s exit code is a statement about the FLOW MODEL, which IS
   the output in that mode. Both cases say so in their own summary line either way — see
   `htmlSummary`/`pageSummaryText` (html.go) and `flowSummary` (flow.go).
8. **`html.go`'s Flow section is built over `.` (the current directory) as a single
   cell, never over the `--repo` list `html` was given.** This is the oracle's own
   comment, quoted in html.go's file header (assay-inbox.sh:1370-1372): the decision
   page's repo list is a REPO axis (which forge to query for issues); the Flow section's
   cell list is a statusgen-ROOT axis. Inventing a cell list from the repo args would
   claim a coverage no reader was given. `deskinbox flow --root ...` is the multi-cell
   form of the same model.

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

**windows-port/15 extends this with two more real-jq extractions, the same discipline:**

- `flow_parity_test.go`'s `TestParityFlow` extracts `write_flow_program()` (the `JQFLOW`
  heredoc) verbatim and runs it through the system `jq` on three raw-envelope fixtures
  (single cell all-ok; two cells with one fully blind, exercising the could-not-check /
  `na` / `AT LEAST` distinction together; throughput itself unread), decoding the real
  jq's output straight into this port's OWN `flowModel`/`flowRow`/`flowStage`/`flowArrow`
  Go types (their `json` tags spell the oracle's own field names) and comparing with
  `reflect.DeepEqual` against `interpretFlow`'s result on the SAME fixture — no
  hand-written second expectation. `TestParityFlowText` does the same for
  `render_flow_text`'s inline jq program (extracted by its own bounding markers, since it
  is a quoted string literal, not a heredoc) against `renderFlowText`'s output, byte for
  byte.
- `html_parity_test.go`'s `TestParityHTML` extracts `write_html_program()` (the `JQHTML`
  heredoc) verbatim and runs it — with the SAME `--arg summary`/`--argjson
  flowonly`/`--slurpfile flowdoc` invocation the oracle's `html`/`flowhtml` MODE cases use
  — against a fixture items array (including a title carrying `<`, `&` and `"` , to prove
  the escaping matches jq's OWN `@html` table: `&apos;`/`&quot;`, NOT Go's
  `html.EscapeString`, which spells the quote `&#34;`) and a flow-model fixture, comparing
  the ENTIRE rendered page — doctype, `<style>` block, cards, Flow section, SVG diagram,
  equivalent `<table>` — byte for byte against `buildDecisionPage`/`buildFlowOnlyPage`.
  `TestSelfContainedPage` is a standing, forge-free check of Verify row 4's own assertion
  (no `url(`, `<script`, ` src=`) so a regression is caught by `go test` alone.
- `flow_test.go` / `html_test.go` cover what the jq extractions do not: cell resolution
  (`--root`, `.assay/cells.txt`, the `.` fallback), the reader seam's could-not-check
  shapes (`readJSON` — non-zero exit, exit-0-non-JSON, the `assay-config:` banner filter),
  and end-to-end CLI dispatch for `flow`/`flow --html`/`html` against a stubbed
  statusgen/deskboard (`runReaderFn`, `lookPathFn`) and a fake `Forge` — no real binary, no
  network, matching main_test.go's existing style for table/walk.

## The html/flow contract (Task 1: page structure, the 7-stage model, the reader invocations)

**`html OUT.html`** writes ONE self-contained file: inline `<style>`, no `<script>`, no
` src=`, no `@import`, no `url(` (the only URLs on the page are the issue links
themselves — Verify row 4). Structure, top to bottom: `<h1>Decisions waiting on the
driver</h1>`, the summary paragraph, one `<article class="card">` per queued item (or the
positive "Nothing is waiting" paragraph when the queue is empty) in the SAME five-part
format `walk` prints (reusing `buildRendered`, format.go — never a second copy), then the
Flow section (below) unconditionally. Light/dark is `prefers-color-scheme`-driven CSS
custom properties, not a second palette.

**The Flow section / `flow` mode's 7-stage model**, in pipeline order, each with WHERE its
COUNT comes from and which `deskboard throughput` stage (if any) owns its QUEUE/SLOTS:

| # | Stage | Label | Count source | `throughput` stage (`tp`) |
|---|---|---|---|---|
| 1 | `intake` | raw-intake front door | `statusgen --intake-debt --json` `.untriaged` | `intake` |
| 2 | `todo` | authored, not started | `statusgen --bottleneck --json` `.stages[].wip` | `dispatch` |
| 3 | `in-progress` | dispatched, being worked | same reader, `.stages[].wip` | *(none — no loop owns it)* |
| 4 | `review` | open PRs, no verdict at head | `deskboard throughput --json` review depth (no per-cell figure — `na`, not `blind`) | `review` |
| 5 | `implemented` | merged, awaiting a verifier | `statusgen --bottleneck --json` `.stages[].wip` | `verify` |
| 6 | `verified` | verified, awaiting the flip | same reader, `.stages[].wip` | *(none)* |
| 7 | `done` | exited the pipeline | same reader, `.stages[].wip` | *(none)* |

Fleet-wide `deskboard throughput --json` bottleneck names translate through ONE fixed map
(`tp2flow`, flow.go): `dispatch→todo`, `review→review`, `verify→implemented`,
`intake→intake`; anything else (including throughput unread) names no bottleneck.

**Reader invocations** (flow.go's `collectFlow`, the ONE place this package shells a
subprocess — Verify row 5): per cell, `<statusgen> --root <path> --bottleneck --json`,
`<statusgen> --root <path> --intake-debt --json`, `<statusgen> --root <path> --net-flow
--json [--since <date>]`; ONE fleet-wide `<deskboard> throughput --json` (never per cell —
its depths/slots are already resolved across the whole configured root set, so a per-cell
call would report the same fleet numbers under N different cell names). `<statusgen>`/
`<deskboard>` resolve `$ASSAY_STATUSGEN`/`$ASSAY_DESKBOARD` else the bare name on `PATH`
(divergence point 5 above).

**Two honesty rules, load-bearing and parity-tested (brief §Review question 1):**

- **`blind` (could-not-check) vs `na` (not applicable) are kept apart.** `blind` means a
  reader was asked and did not answer (non-zero exit, or exit 0 with unparseable/`null`/
  `false` output) — it carries the reader's own diagnostic. `na` means nothing failed; the
  figure legitimately does not exist at that granularity (`review`'s per-cell count: it is
  read fleet-wide by `deskboard throughput`, which resolves no per-cell figure). Collapsing
  the two would cry wolf on every per-cell `review` row.
- **A fleet count summed over cells that were not ALL read is `countPartial` ("AT LEAST"),
  never printed as a bare total.** `review`'s fleet count is EXEMPT from this flag (it comes
  from ONE fleet-wide reader, never a per-cell sum), matching the oracle's own
  `$sd.stage != "review"` guard in both the count-partial computation and the terminal
  render's note line.
