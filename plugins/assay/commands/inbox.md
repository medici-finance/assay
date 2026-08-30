---
description: What's waiting on you across your configured repos — needs-decision, question, help wanted, and urgent issues, sorted urgency-then-age. Add --walk to be asked one decision at a time, --html FILE to read them as a page, or --flow to see where the pipeline is stuck. Read-only, works from any terminal.
argument-hint: "[--walk [--item K] | --html OUT.html | --flow [--html OUT.html]] [owner/repo ...]"
---

# assay:inbox

The inbox: a **derived query**, not a stored roll-up or a service. This command
shells `../scripts/assay-inbox.sh` — a plain `gh` + `jq` helper — across your configured
repos and prints what's waiting on a human decision, most urgent (then oldest) first.

**Read-only.** It never writes to, comments on, or closes any issue. **Terminal-agnostic.**
It is plain `git`/`gh` shell — it runs from any terminal.

## What it queries

The four escalation label tokens. `needs-decision` is created and owned by the escalation
contract; the other three are stock GitHub labels this inbox folds in alongside it:

- `urgent`
- `needs-decision`
- `question`
- `help wanted`

An issue carrying more than one of these is shown once, ranked by the most urgent label it
carries. Each row prints label, repo, `#number`, title, age, and the issue URL.

## Three renderings of one queue

The ordering and the item format are computed ONCE and rendered three ways, so the table, the
walk and the page can never disagree about what is waiting or which item is first.

| Mode | What it does |
|---|---|
| *(none)* | The terminal table — one row per item. Unchanged; still one `gh issue list` per repo per label and nothing else. |
| `--walk` | Prints **one** item in the five-part decision format: `<repo>#<N> — question k of n`, then **Context** (3–6 lines: what it is, why it is blocked on a human, what it unblocks, the evidence links), **Options** (lettered, recommended default first and labelled, at most four), **Reply shape** (what a one-word answer must contain), **Verification** (what the desk checks after the act, and what it moves to next). Prints item 1. |
| `--item K` | With `--walk`, and implying it: print item **K** (1-based) instead of item 1. Out of range is an error — never a silent empty. |
| `--html OUT.html` | Writes the whole queue to `OUT.html` as cards in that same five-part format, followed by the **Flow** section below. One self-contained file: inline CSS, no scripts, no external assets, light/dark via `prefers-color-scheme`. The only URLs on the page are the issue links. No server. |
| `--flow` | A different question — *how is the system performing*. Prints the pipeline flow model as a terminal table and exits. See "The Flow page" below. |
| `--flow --html OUT.html` | The same model as a left-to-right inline-SVG stage diagram, that section alone. |

`--walk` is **non-interactive by design**: it prints one item and exits, never prompts, never
blocks on a tty. The turn-taking — ask one, wait, record the ruling, ask the next — belongs to
the [`assay:ask-decision`](../skills/ask-decision/SKILL.md) skill, which shells this command
with an incrementing `--item`. Keeping the loop out of the script is what lets an agent, a
human at a prompt, and a CI job all use the same renderer.

`--walk` and `--html` are alternative renderings; passing both is refused rather than silently
resolved to one.

### Where Context and Options come from

`--walk` and `--html` read each issue's **body and latest desk/bot comment** (one extra
`gh issue view` per rendered item; the plain table still makes no such call) and lift:

- **Context** from a `## Context` / `## Situation` / `## Ask` / `## Summary` / `## Problem`
  section, else the body's opening prose — plus the escalation label it is blocked on, an
  `Unblocks:`/`Depends:` line if the issue states one, the latest desk note, and a gate line
  carrying the labels, the age and the URL.
- **Options** from a `## Options` section's lettered or numbered lines. The option whose text
  says "recommended" is promoted to **A** and labelled; at most four are shown.

Neither is invented. An issue that states no options renders
`options not yet stated — desk to fill`, and the Reply shape degrades from "reply with one
letter" to "reply with the ruling in one line" — the desk is expected to fill the options in
before putting the question. An issue whose body could not be read renders
`could-not-check: … the item is UNREAD, not empty` and the run exits `2`.

## The Flow page — where is the system stuck

`--flow` answers the question the decision queue does not: *how is the system performing?* It
is the read to take **before** asking the driver anything, because it says whether the
question in front of them is the constraint or a symptom of one three stages upstream.

It is **derived, not probed.** Every number comes from a JSON emitter that already exists;
this command runs those readers and arranges what they say. It opens no new source, parses no
`STATUS.md`, and reaches nothing over the network that the readers do not reach themselves.

### The stages

| Stage | What it holds | Where the count comes from |
|---|---|---|
| `intake` | the raw-intake front door, untriaged | `statusgen --intake-debt --json` (`.untriaged`) |
| `todo` | authored, not started | `statusgen --bottleneck --json` (`todo` WIP) |
| `in-progress` | dispatched, being worked | `statusgen --bottleneck --json` (`in-progress` WIP) |
| `review` | open PRs with no reviewer verdict at the current head | `deskboard throughput --json` (review depth) |
| `implemented` | merged, awaiting a verifier | `statusgen --bottleneck --json` (`implemented` WIP) |
| `verified` | verified, awaiting the done flip | `statusgen --bottleneck --json` (`verified` WIP) |
| `done` | exited the pipeline | `statusgen --bottleneck --json` (`done` WIP) |

`review` is the **forge lane** that runs alongside `in-progress`: a brief is in-progress while
its PR is open and reaches `implemented` when that PR merges. It sits between them for that
reason, and it is the one stage with no board status behind it.

**COUNT and QUEUE are different numbers, and both are shown.** COUNT is the board's WIP for
the stage. QUEUE is the depth the owning loop actually dispatches from — for `todo` that is
the *eligible, unclaimed* subset, not the whole column. The ratio is QUEUE/SLOTS, because that
is the ratio `deskboard throughput` defines and the one a "should this desk widen?" decision
turns on. SLOTS are the loop's resolved pool **capacity**, not a live count of working agents.

**The bottleneck** is `deskboard throughput`'s: the largest queue/slots ratio among the stages
it read, translated into the stage names above (its `verify` is this model's `implemented`).
The dwell-weighted Theory-of-Constraints constraint (`statusgen --bottleneck`'s WIP × dwell)
is reported *beside* it, not instead of it — they answer different questions and can disagree.

**Flow for the window** is arrivals into the pipeline and completions out of it, from
`statusgen --net-flow --json`. The steps *between* stages are `could-not-check`: no reader in
the tree emits per-stage transition counts for a window, and a delta inferred from two board
reads would be a measurement this system does not take. Those arrows are drawn dashed.

### Honesty rules

- **Every number carries its source** — the emitter that produced it and the board sha it was
  read at.
- **A stage whose reader could not be read renders `could-not-check`, never `0`.** A `0` reads
  as *drained*, which is the opposite of *unread*. The reader's own diagnostic is carried onto
  the render and onto stderr, and `--flow` exits `2`.
- **A fleet count summed over cells that were not all read is flagged `AT LEAST`**, never
  printed as a total.
- **The diagram renders authored status.** The board lints consistency between status cells,
  Evidence and PRs; it does not measure whether the work is done. The page says so in one line.

### Cells

A cell is one statusgen root — a checkout carrying `docs/streams`. Resolution order:

1. `--root PATH` (repeatable) — each path is one cell.
2. `./.assay/cells.txt` — one cell per line as `NAME PATH` (or a bare `PATH`); blank lines and
   `#`-comments ignored. This is the inbox's flat mirror of `cells.yaml`'s `name` /
   `streams_root` pairs, exactly as `.assay/repos.txt` is its flat mirror of the repo roster.
3. `.` — the current directory, as a single cell.

The fleet row is always printed. Per-cell row blocks appear only when more than one cell
resolves — with one cell they would repeat the fleet row verbatim. Pool width is resolved
fleet-wide, so SLOTS appear on the fleet row only; per-cell rows say so rather than repeating
a number that does not describe them.

`--since YYYY-MM-DD` sets the flow window (default: the reader's own).

### If a reader is missing or too old

`statusgen` and `deskboard` are looked up on `PATH` (override with `ASSAY_STATUSGEN` /
`ASSAY_DESKBOARD`). A binary that predates one of these flags refuses it, and that refusal
becomes a `could-not-check` stage carrying the binary's own message — for example
`flag provided but not defined: -net-flow`. That is the expected reading against a pinned
build older than the flag, and it is reported as itself rather than as an empty queue.

## A failed query is never shown as an empty inbox

The run always ends with a summary line — `assay-inbox: N item(s) across M repo(s)` — so
"nothing is waiting" is *positively* stated rather than inferred from silence. If any `gh`
query fails (expired token, missing repo, rate limit), the helper prints `gh`'s own
diagnostic to stderr, marks the summary `THIS INBOX IS INCOMPLETE`, and exits non-zero.

| Exit | Meaning |
|---|---|
| `0` | every query succeeded — the output is complete |
| `1` | precondition failure: `gh`/`jq` missing, no repos/cells resolvable, or bad arguments (unknown flag, non-numeric or out-of-range `--item`, `--walk` together with `--html` or with `--flow`, a malformed `--since`) |
| `2` | one or more queries FAILED — the output is **partial**, see stderr. In `--walk`/`--html` this includes an issue whose body could not be read: it is rendered as `could-not-check`, never as an item with nothing to say. In `--flow`/`--flow --html` it includes any flow reader that could not be read |

A blind Flow section on the `--html` page does **not** redden that run. The exit code of the
decision modes is a statement about the *decision queue* — a caller checking it is asking
whether it saw all the decisions — so a stale `statusgen` is reported in the summary line and
on the page instead. Where the flow *is* the output (`--flow`), a blind reader exits `2`.

Per-repo-per-label fetch cap is `ASSAY_INBOX_LIMIT` (default 500); `gh issue list` would
otherwise default to 30 and silently discard the *oldest* items — the exact ones this
ordering exists to surface. Hitting the cap is reported, not swallowed.

## Repo resolution (in order)

1. **Arguments** — pass one or more `owner/repo` pairs: `/assay:inbox owner/repo1 owner/repo2`.
2. **`./.assay/repos.txt`** — if no arguments are given and this file exists in the current
   directory, it is read as a flat newline-delimited list of `owner/repo` (blank lines and
   `#`-comments ignored). This is the inbox's repo config — a flat list, nothing more.
3. **Origin default** — if neither of the above applies, the command falls back to the
   current repo's `origin` git remote.

## Usage

```
/assay:inbox                          # uses .assay/repos.txt, else origin
/assay:inbox example-org/app example-org/service   # one or more repos
/assay:inbox --walk                   # ask me about the first decision
/assay:inbox --walk --item 3          # ask me about the third
/assay:inbox --html ~/inbox.html      # write the whole queue as a page
/assay:inbox --flow                   # where is the pipeline stuck?
/assay:inbox --flow --root ../app --root ../service   # two cells, plus the fleet total
/assay:inbox --flow --html ~/flow.html                # the stage diagram as a page
```

## Instructions for the agent running this command

1. Resolve `$ARGUMENTS` as the repo list (may be empty).
2. Before running anything, check that every repo argument matches `owner/repo`
   (`[A-Za-z0-9._-]+/[A-Za-z0-9._-]+`). Refuse to run the command if any token does not
   match, and say why. Then pass each repo as its own separately-quoted argument — never a
   bare unquoted `$ARGUMENTS` expansion:
   ```
   bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" "owner/repo1" "owner/repo2"
   ```
   (`CLAUDE_PLUGIN_ROOT` resolves to this plugin's installed root; if unset, resolve the
   script relative to this command file's own `../scripts/assay-inbox.sh`.)
   Flags go ahead of the repo list and are passed as their own arguments:
   ```
   bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" --walk --item 2 "owner/repo1"
   bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" --html "$HOME/inbox.html" "owner/repo1"
   ```
3. Print the helper's table output verbatim to the user — do not summarize away rows, do not
   edit, close, or comment on any issue found. If the helper errors (missing `gh`/`jq` auth,
   no repos resolvable), surface the error message as-is; do not guess at issue state.
4. **Check the exit code.** A non-zero exit means the inbox you are looking at is incomplete
   or absent — say so plainly and relay the stderr diagnostic. Never report "nothing is
   waiting" on a non-zero exit; that is the failure mode this command exists to avoid.
5. **In `--walk`, print the item and STOP.** Ask that one question and wait for the answer;
   do not run `--item 2` in the same turn, and do not act on the recommended default in place
   of an answer. When the answer comes back, record it on the issue as a **relay** and move
   the escalation label before presenting the next item — the
   [`assay:ask-decision`](../skills/ask-decision/SKILL.md) skill holds that contract, the
   relay wording, and the verification step. This command only renders.
6. **In `--html`, report the path you wrote and the item count**, and hand the file over. Do
   not paste the page's markup into the conversation.
7. **In `--flow`, print the table verbatim and read the bottleneck line.** Do not summarize
   away the `could-not-check` rows — an unread stage is the one thing the reader most needs to
   see, and it is exactly what a summary drops. If the run exits `2`, say which readers were
   blind before quoting any number from the rest.
