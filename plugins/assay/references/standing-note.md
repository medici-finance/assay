# Standing note — the loop-continuity record that survives the session

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
  `deskboard`, since a later cycle may have moved it again.
- **In-flight** — PR #935 (`https://github.com/medici-finance/assay/pull/935`) mid
  fix-and-re-review; re-check `gh pr view 935 --json reviewDecision` for the current verdict.
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
