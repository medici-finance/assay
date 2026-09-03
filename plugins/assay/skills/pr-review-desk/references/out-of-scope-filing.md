# Out-of-scope discoveries → file an issue, not an in-review flag

The filing contract for anything a reviewer discovers that the PR's author cannot fix on that PR.
`pr-review-desk/SKILL.md` § The reviewer's bar points here. Convention settled 2026-07-14; the
dual-track rule added 2026-08 after the desk filed the same defect twice off its own two lanes.

## The line

A **review finding** is something **the PR author can fix on THIS PR** — those stay in the review
as findings (blocking → `--request-changes`; non-blocking → a note in the review body or the flip
wrap-up). **Everything else a reviewer discovers is NOT a finding and MUST be filed as a GitHub
issue at discovery time** — defects in files outside the PR's diff, systemic/process insights,
brief/register defects, work owned by another PR or person. A discovery buried in a PR thread
depends on a human or the desk happening to notice it later; the convention that settled this for
questions ("we can't just have questions buried in PRs — they need to be specific issues") holds
for every out-of-scope discovery.

## Filing goes through `deskfile`, never a bare `gh issue create`

`deskfile check -R <repo> --title "<t>"` first — a dry run over the repo's open issues, same dedupe
search, writes nothing. If it reports no candidate at or above threshold:

```
deskfile new -R <owner/repo> --title "<t>" --body-file f.md --raised-by reviewer [--label bug]
```

If it reports a likely duplicate it refuses (exit 5) and prints the exact attach command:

```
deskfile attach -R <owner/repo> --to <N> --body-file f.md
```

Post your evidence there instead of minting a second issue. **Do not silently drop it as "already
filed" and move on** — two lanes' evidence is typically complementary, not redundant (one carries
the mechanism, the other the CI-run proof); `attach` preserves both halves on one issue rather than
losing one to a duplicate that gets closed unread. A dedupe-search failure fails CLOSED (exit 6):
minting a possible duplicate is the more expensive direction, so retry rather than falling back to
raw `gh issue create`. `deskfile` is documented in `tools/desk/README.md`
§ "deskfile — the issue-filing gate", and it gates WHETHER/WHERE, never WHO — it never mints an App
token, filing under whatever `gh` credential is ambient in the caller's session.

**Why a bare `gh issue create` is banned.** The intake desk's claims lock is keyed per issue
NUMBER, so two filings of one defect look like two work items and a naive dispatch fans two workers
onto one defect, who then collide on the same files. This is not hypothetical: two lanes over the
same PR filed the identical item seconds apart, repeatedly, before the gate existed.

**Always pass `--raised-by reviewer`.** It stamps the `raised-by:reviewer` label so the by-desk
issue metric can tell which loop NOTICED the problem — a different question from which App posted
it, and the only one that answers "is one loop generating all the churn?". `reviewer` is this
desk's role in the roster's role-bindings, and the roster is the declared source for the
vocabulary, so do not invent a label spelled after this skill's name: `deskfile` refuses (exit 5)
any role the roster does not bind, and prints the bound set. Omitting the flag is not neutral — the
issue lands with **UNKNOWN** provenance, which is the absence of an answer and never "a human
raised it". If `raised-by:reviewer` does not exist on the target repo yet, `deskfile` files the
issue anyway, unstamped, and prints the one-off `gh label create` command: run it, and future
filings stamp.

## Dual-track PRs — hold until both tracks report, dedupe the union, file once

A risk-classed PR gets TWO independently-dispatched reviewers over the same diff (the correctness
reviewer plus a separate `/security-review` agent), **dispatched in the SAME turn and running
concurrently — this hold binds the desk's FILING only; the two VERDICTS never wait for each other,
and the ready-flip reads both at head on its own.** Both can notice the same out-of-scope item, and
if each filed it the moment it found it, a `deskfile check` alone still loses the race: track B's
search can run and come back clean *before* track A's `deskfile new` has landed. Filings 44 seconds
apart, and 17 minutes apart, are both on record. The fix is desk-side, not reviewer-side:

- **On a dual-tracked PR, neither reviewer calls `deskfile` itself.** Each track reports its
  out-of-scope discoveries back to the desk as a `## Out-of-scope discoveries` list in its own
  review body (repo, one-line description, evidence/file:line per item) and returns without filing.
- **The desk holds filing until BOTH tracks have reported at the same head.** A single-track PR
  (risk-clear, one reviewer) has no second track to wait for and keeps the base rule above.
- **Once both are in, the desk dedupes the UNION of the two lists itself, before calling `deskfile`
  at all** — same repo + same file:line, or two near-identical one-line descriptions, collapse into
  ONE item. This is what actually closes the race: it compares the two tracks' findings against
  each other directly, rather than against a search that cannot see a sibling's unfiled report.
- **The desk then files each distinct item exactly once**, via `deskfile check` →
  `new --raised-by reviewer` / `attach` (the search is still worth running — it catches a duplicate
  against a PRIOR PR's review, which the union-dedup does not).
- If one track fails to report (agent error, timeout), the desk does not file on its behalf — treat
  a missing track's discoveries as unknown, not empty; re-dispatch or note the gap.

## Route by type

- **repo-specific defect** → an issue on that repo's own tracker (label `bug` where apt).
- **systemic / process insight** → an issue on **medici-finance/assay** (the
  insight-routing rule).
- **needs human:<name> or a stronger model to proceed** → label `question` with a comment stating what is
  needed and from whom (CLAUDE.md § Filing & escalation).

The review then carries **one line per item — `filed as <repo>#<N>`** (or, on a dedupe hit,
`attached to <repo>#<N>`) — a pointer, never the register. If it is worth the desk's attention it
is worth its own issue, or its own comment on the existing one.

## File-and-exit, never block — the pod-loop contract (the pod-loop contract)

The orthogonal half of the autonomous-drive rule: *file at discovery, don't ask permission* is one
half; **never hold the run open after you have filed** is the other — but the reversibility test runs
FIRST. When the review loop hits a decision fork, a human gate, or an external blocker it cannot
resolve, a REVERSIBLE fork is acted on at its best-guess default and the filing NAMES that default
rather than asking (the merge gate catches a wrong default); only a genuinely one-way fork makes it
**file (or confirm already-filed) the escalation and exit the run** — a pod CronJob run terminates; a live session
window yields to the next PR in the queue. It never blocks and never waits in-line for the answer;
resumption is event-driven — a fresh run picks the PR up when the answer or the label lands, and
until then the run does not hold on it. A documented wait-state that is merely SURFACED (a
`WAIT-CI` PR reported and moved past, a MERGE-NOW awaiting the human's merge) is already
file-and-exit-shaped: keep it, and note explicitly that the run does not hold on it either. Route a
genuine ONE-WAY decision FORK to a `needs-decision` issue in the self-contained shape (Situation + 2–4
Options with pros/cons + what-happens-on-each-answer + links, answerable without opening the repo);
lighter input needs use `question` / `help wanted` — a bare label is not filing. **Why:** a loop
that blocks in-run is undebuggable in a pod — its blocked state must be an at-rest FILED issue
anyone can inspect, never a hung process with no record.

## `DESK-FLAG:` is retired (2026-07-14)

The structured in-review marker buried actionable items inside PR review bodies behind a
**review-body parser that was never built** — the issue scanner exists and scans *issues*, but the
DESK-FLAG review-comment parser its brief specified was never written (confirmed: zero hits for
`DESK-FLAG` outside the brief's own text). File the issue directly instead of depending on a parser
that does not exist. Free-form "flagged for the desk" prose was never a register either.
