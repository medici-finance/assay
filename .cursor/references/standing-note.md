<!-- GENERATED from plugins/assay/references/standing-note.md by deskinstall --harness cursor. Do not edit. -->
# Standing note — the loop-continuity record that survives the session

<!-- assay:harnesslint non-matrix-reference — harness-neutral loop-continuity record schema, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

A desk loop's working state lives in tool output that gets evicted between calls, and does
not survive a session ending outright — a usage-limit reset, a crash, a hand-off to a fresh
window. The standing note is the fix: a short, named-schema record a session writes at its
own iteration boundaries so that whatever picks the loop back up — the same session resumed,
or a different one — has an index of what to re-check instead of nothing at all. It is not a
transcript and not a cache of conclusions; it is a map of where the live answers are.

## The nine sections

Every standing note carries exactly these nine headings, in this shape, every time. A role
with nothing to report under one still writes the heading with an explicit "nothing" —
dropping a heading and reporting an empty one are different facts, and only one of them says
"checked and clear."

### Boot
The role, loop and session identity established this run, and the worktree it is bound to.

### Monitors
Which live watches (an event monitor, a cadenced sweep) are open, and each one's own
liveness signal.

### Hands-off
Items handed to another role or to the driver, and what each is waiting on.

### Merged
What landed on the default branch this cycle.

### Flipped
Which rows, PRs, or states changed status this cycle (a ready-flip, a verify-to-done
transition, a claim release).

### In-flight
Work dispatched but not yet landed — open draft PRs, running dispatches, held claims.

### Filed
Issues or findings opened this cycle, and their disposition at the time of filing.

### Teammates
Other concurrent sessions or roles known to be active, and what each is working, so two
sessions do not converge on the same item.

### Board
The shape of the queue as last read — counts, cadence, freshness — at the time of that read.

## The re-probe rule

The load-bearing rule, and the reason this is a schema and not just a list of headings: every
row names the primary state to **re-probe**, never the value it last observed. A note is an
index of what to re-read on resume, not an answer key a resumed session can act on directly —
acting on a remembered answer is exactly how a resumed session collides with a board that kept
moving while it was gone. Write "PR #N handed to the driver; re-read its state before assuming
it is still waiting," never "PR #N is waiting on the driver." The difference is small on the
page and decisive on resume.

## The hold rule

A hold is a standing restriction the session carries forward from one cycle to the next: do
not flip on one repository, do not dispatch one stream, leave one PR alone until a fix
ships. A hold is the row most likely to change shape when a session's context is compacted
or summarised. A note that says "no flips on repo A" can come back as "no flips", and the
wider rule then binds work its source never covered. So a hold line carries its own source:

- **The source goes on the same line as the hold.** Write it as one line, in this shape:
  `hold: <what is held> on <scope> (source: <issue, PR or ruling-comment reference>; lifts when <condition>)`.
  The source is the record that set the hold, such as an issue, a PR, or the comment that
  carries the ruling, and it is written as a reference a later session can re-read. "The
  coordinator said so" and "per earlier" are not sources.
- **The scope is copied from the source.** Name the repository, ref, stream or PR the source
  names, in the source's own terms. A summary or a resumed session never widens it. When
  the line and the source disagree, the source wins.
- **A hold that starts in conversation gets a record first.** If a hold arrives only as an
  instruction inside the session, put it on the forge (a comment on the item, or an issue)
  before writing the hold line, and cite that. The record quotes the instruction and names
  who gave it, so a later scope check has something to compare against. An instruction that
  lives only in context does not survive compaction, and neither does a hold that rests on
  it. If the record cannot be written (a refused filing or mint, an exhausted write budget,
  content that may not go on that tracker), keep the hold anyway: write the line with
  `source: none recorded — <why>`, treat it as *unsourced* below, and escalate once writing
  works again. Never leave the hold out because its record could not be written.
- **A hold lifts on its own condition, never on a closed container.** A sourced hold lifts
  only when its `lifts when` condition is observed met, or when a later record releases or
  replaces it from an identity that could have set it: the driver's own login for a hold
  that rests on a human ruling or gate, the security lane's own clearance for a hold a
  security finding set, the desk itself only for a hold it set on its own judgement. A
  closed issue or a merged PR is not a lift by itself. A tracking issue closes when its fix
  merges, and a decision issue closes once its ruling is recorded, while the hold still binds.
- **Every fresh sweep re-checks every hold.** At the next fresh sweep, and on any resume
  after compaction, re-read each hold's source. A line that names no source is looked up
  first: search the item's comments, its decision issues and the driver's issues for the
  record that set it. If one is found, cite it on the line and check the hold at that
  record's scope. Each hold then ends in one of four states:
  - *live*: the source is readable and neither its lift condition nor a release has been
    observed, even if the source is now closed. Keep the hold at the scope the source states.
  - *dropped*: the lift condition is observed met, or an entitled identity released or
    replaced the hold. Drop it, and say so: write
    `dropped hold: <what> on <scope> (<lift met: reference | released: reference>)` under
    **Flipped**, and in the sweep's own report. A hold must never vanish silently, and it
    must never outlive its lift silently either.
  - *could-not-check*: the source exists but could not be read this sweep, for example
    because of a forge error or a missing permission. Report the hold as could-not-check
    and keep it marked that way. Never drop it as unsupported, and never count it as
    confirmed. Re-read it at the next sweep.
  - *unsourced*: the line names no source and the search found none. Keep the hold, report it
    as `unsourced hold: <what> on <scope>`, and escalate it once: a `question` to the driver
    naming the hold and its scope. Drop it only on the driver's own answer, cited as
    `released:`. A hold kept too wide costs throughput; a hold dropped wrongly can release a
    ruling, so the unsourced case keeps the hold and asks.
- **Dropping a hold line lifts no gate.** The note's hold lines are this session's carried
  restrictions. Dropping one removes only that line. It never lifts a gate a role body, a
  forge label, a decision issue or a desk tool enforces, and never authorizes an act such a
  gate refuses.

## Where it lives

The note is session state, not a project artifact: it belongs in the session's own scratch or
temp workspace, never as a file added to the project's tracked source. A version-controlled
copy becomes a merge conflict and a stale record at the same moment two sessions touch it, and
neither failure mode is recoverable by writing more carefully — only by not putting it there.

## When it is written

At every iteration boundary, and before any long wait — a rate-limit park, a queue drain, a
hand-off to another role or to a human. Anywhere a session might not get to finish its next
thought, the note is what makes the gap survivable.

## Boundary

This is not the durable cross-session memory store, and it is not a status board. It carries
no compaction rule, no retention policy, and no shared-write contract — those all belong to a
durable store's own design, which has a concurrent-write failure mode a single session's note
does not need to solve. A standing note is scoped to one loop's own continuity, nothing wider.

## Worked example

Here is a worked example note, from a pr-review-desk session, written just before a
rate-limit park mid-cycle:

- **Boot** — role pr-review-desk, loop and session ids exported this run; re-verify with
  `deskboot pr-review-desk --check`, not this line.
- **Monitors** — event monitor and cadenced sweep both watching the open-PR queue; re-verify
  liveness from the monitor's own heartbeat, surfaced by `deskboard --watch-status`.
- **Hands-off** — PR #941 handed to the driver for a `needs-decision` call; re-read
  `gh pr view 941 --json state,isDraft` before assuming it is still waiting.
- **Merged** — none this cycle; re-confirm against `git fetch --no-tags origin main` and diff
  against `origin/main`, not this note.
- **Flipped** — PR #928 flipped ready-for-review; re-read its row from a freshly run
  `deskboard`, since a later cycle may have moved it again. `dropped hold: no dispatch on
  stream S (released: the driver's comment on issue #950)`: the driver lifted it, so the
  hold was dropped and the drop was reported in the sweep.
- **In-flight** — PR #935 (`https://github.com/medici-finance/assay/pull/935`) mid
  fix-and-re-review; re-check `gh pr view 935 --json reviewDecision` for the current verdict.
  `hold: no ready-flip on repository A only (source: issue #947; lifts when the fix it
  tracks ships in a pinned release)`; re-read issue #947 at the next sweep. Repository B
  keeps flipping, because the source names A alone. Issue #947 closed when its fix merged,
  but no pinned release carries the fix yet, so the hold stays live.
- **Filed** — issue #952 opened for an out-of-scope finding surfaced on PR #930; re-read
  `gh issue view 952` to see whether it has since been triaged.
- **Teammates** — a worker-desk session was dispatching against the same stream at last
  check; re-read the roster registration via `deskroster`, not this line, before claiming an
  item.
- **Board** — 6 actionable (4 NEEDS-REVIEW, 2 RE-REVIEW) as of the last sweep; re-generate
  with `deskboard` rather than carry this count into the next cycle.

Every row above names something re-readable — a command, a URL, a ref — never a bare
assertion. An entry that only says "the queue looked clear" or "PR #935 looked fine" is not a
standing note; it is a cached conclusion wearing the schema's shape.
