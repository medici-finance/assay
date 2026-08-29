# worker-desk — dispatch runbook

The mechanical detail the skill body points at: the repo-set derivation, the per-tick sweep queries
and their pre-resume guards, and the intra-brief split procedure. These are procedures, not judgment
calls, which is why they live here rather than in `SKILL.md` — the rules that govern them stay in the
skill, and `SKILL.md` §Sources of work is the list of what a tick must read.

## Deriving THE REPO SET

`SKILL.md` §THE REPO SET declares the two sets and their sources. This is the derivation itself. Run
it under **bash**: zsh does not word-split an unquoted `$VAR`, so a `for R in $OBSERVED` form silently
collapses the whole thing to a single root.

```bash
DECLARED=$(deskroster repos --scope topology 2>/dev/null \
           | awk -F'\t' '{for (i=1;i<=NF;i++) if ($i ~ /^root=/) { sub(/^root=/,"",$i); print $i }}')
# observed: siblings carrying docs/streams. The --git-dir test keeps only real clones — a LINKED
# WORKTREE answers with an absolute path, a clone answers ".git". Without it a sibling glob returns
# every session worktree on the machine (measured: 213 paths for 4 repos, each board swept dozens of
# times).
OBSERVED=$(for R in . ../*; do
  [ -d "$R/docs/streams" ] && [ "$(git -C "$R" rev-parse --git-dir 2>/dev/null)" = .git ] && echo "$R"
done)
ROOTS=$(printf '%s\n' "$DECLARED" "$OBSERVED" | grep -v '^$' | while read -r R; do   # UNION, keyed
  [ -d "$R" ] || continue                                                            # by repo SLUG
  U=$(git -C "$R" remote get-url origin 2>/dev/null) || continue
  SLUG=$(printf '%s' "$U" | sed -e 's#.*[:/]\([^/]*/[^/]*\)$#\1#' -e 's#\.git$##')
  printf '%s\t%s\n' "$SLUG" "$(cd "$R" && pwd -P)"
done | sort -u | awk -F'\t' '!seen[$1]++')          # one line per repo: <slug>\t<path>
```

**Key on the repo slug, not the path** — two clones of one repo are one queue, and the boards are read
out of `refs/remotes/origin/main`, so which clone the dedupe keeps does not change the board you get.
Slug order is also what makes the interleave reproducible across machines whose local directory names
differ.

**The union is the non-narrowing direction, and a root in exactly one list is named in the report
either way — never dropped.** Declared-but-absent = could-not-check (the checkout is not on this
machine), not an empty queue. Observed-but-undeclared = a `topology.yaml` gap: dispatch it anyway this
cycle and file the gap, because refusing to dispatch a real queue to punish a missing declaration
starves the queue.

## The tick sweep — the queries, and the three pre-resume guards

Two shapes run every tick: the **board-wide instruments**, once for the whole scan set, and the
**per-slug PR reads**. `SKILL.md` §Sources of work says which source each answers.

```bash
# board-wide, once per tick — each is fail-closed and three-state
deskboard dispatch                          # the work to START + the HELD-BACK decomposition (#321)
deskboard stalled --min-age-hours 48        # stalled drafts + shepherd list (--json for the machine shape)
deskboard health                            # is main red? green / RED / COULD-NOT-CHECK
issueboard issues                           # un-briefed trusted issues; EXTERNAL/UNBLESSED quarantined
```

`issueboard` replaces the raw `gh issue list` form this runbook used to carry. It resolves the scan
scope from the same roster key the placeholder scanner reads, applies the trust gate itself, and
**exits 6 for the WHOLE board** when the scan set is unset/empty or any one repo is unreadable — where
the list form returned a clean, empty board instead. An exit 6 from any of the four is
could-not-check: report the surface BLIND for the tick, never zero.

Then per slug in SCAN REPOS:

```bash
gh pr list --repo <r> --state open --limit 100 \
  --json number,isDraft,updatedAt,reviewDecision,statusCheckRollup,labels
deskdisposition sweep -R <r> --limit 100     # number  state  verdict  dispatch-eligible  title
```

`labels` is not optional — an ad-hoc query without it re-dispatched a `question`-labelled PR the desk
had already routed to a human (#827). `--limit` is mandatory: bare `gh pr list` silently caps at 30.
At 100 results treat the sweep as possibly truncated and widen rather than claiming zero.

**Read the dispositions before any staleness arithmetic** (#728, #827). `SUPERSEDED` /
`RESOLVED-ELSEWHERE` is a deskclose item, never an orphan; `NEEDS-REBASE` is live work; a tool exit 6
or a failed list read is could-not-check — report the repo BLIND for the cycle.

A PR is ORPHANED when its disposition reads checked-clean AND the worker owes it action AND it has had
no commit/comment for >4h AND no live dispatch claim exists. Three guards run on every candidate
first:

1. **Label exclusion.** `question` / `help wanted` = WAITING-ON-INPUT, not an orphan, however stale.
2. **Already-triaged is not neglect.** "No activity for >4h" cannot tell a neglected PR from a
   resolved one — both are silent. The durable form of this guard is the disposition record; fall back
   to reading the latest comment only for PRs whose verdict predates it, and if that comment is itself
   a bot/worker/desk marker the PR is ALREADY-TRIAGED: surface it to the human queue, do not
   re-dispatch, and record the verdict with `deskdisposition set` so the next sweep need not re-read
   the prose.
3. **Supersession check (cheap, pre-claim).** If the body references `closes #N` / `fixes #N`, verify
   that issue is not already closed by a *different* merged PR, and check the target brief's stream
   README for `status: implemented` pointing at another PR. A superseded PR is surfaced as
   recommend-close and written with
   `deskdisposition set -R <r> --pr <N> --verdict SUPERSEDED --evidence <url>` — the write is what
   stops the same guard being re-run four times on one PR.

A resume-worker's dispatch carries the write side of the same record: `SUPERSEDED` when something else
landed it, `RESOLVED-ELSEWHERE` when the outcome was reached another way, `NEEDS-REBASE` when it is
still live work. The verb writes a label plus an evidence-carrying marker comment and is idempotent.
**It does not close the PR** — the close is deskclose's human-authorized act, and a stated-but-unexecuted
close intent is its own failure.

### Queue suppressors — read them when a stream offers nothing for several consecutive ticks

A claim subtracts twice: once as an eligibility exclusion and again as a per-stream cap decrement, so
branch-claim corpses from merged/closed PRs — and expired `refs/dispatch/*` claims, which nothing
re-surfaces onto a board — can zero a stream's whole allowance while it still holds work. Read them
with `git ls-remote origin 'refs/dispatch/*'` plus the repo's own `dispatch-claim` helper's list/show
verbs, and file what the read shows rather than concluding the stream is drained.

## Intra-brief splits — N shards, ONE brief PR (methodology/43)

The default is one worker per brief. A brief OPTS IN by declaring `parallel-streams:` (a name plus
file globs per shard); nothing here runs for a brief without the field. The field is declared once,
empty, across the corpus today — the lane is live but unexercised.

1. **The gate.** `statusgen shardcheck --brief docs/streams/<stream>/brief-<NN>-<slug>.md --root <that
   repo's root>`. **0 checked-clean → the split may dispatch. 1 checked-failed → ONE worker, serially.
   2 could-not-check → ONE worker, serially.** 1 and 2 carry the same instruction on purpose: a split
   whose safety could not be established is not a split you may run. Refusing a safe split costs
   wall-clock; approving an unsafe one costs a red main. Its refusal classes (`path-overlap`,
   `shared-surface`, `symbol-coupling`, `dead-glob`, `plan-shape`) are properties of the declaration
   against the real tree — the fix is a different declaration or a serial run, never a narrower check.
2. **What file-scoping does NOT prevent.** It stops shards writing the same bytes and nothing else.
   Semantic collision with no textual conflict (a function that grew a parameter on one branch while
   another still called it with the old arity — different files, both green alone, main red on merge)
   is caught only within one Go package; every other cross-shard pair is reported `COVERAGE-GAP` →
   could-not-check → serial. A shared numbering space (rule numbers, IDs) is withdrawn from every
   shard and belongs to the coordinator after the shards land. A collision with a DIFFERENT branch is
   merge-time re-checking — a separate instrument, and this gate must not be described as replacing it.
3. **The recombine model.** The dispatcher creates the brief's integration branch from fresh
   `origin/main` and pushes it empty; that branch is the brief's single branch and the PR's head. Each
   shard-worker gets its own worktree cut from that same commit and pushes
   `brief/<stream>-<NN>-shard-<name>` — never to the integration branch, never opening a PR. When
   every shard has pushed, the coordinator merges the shard branches in declared order, opens the ONE
   draft PR, then makes the shared-surface edits the shards were forbidden and runs the brief's Verify
   table against the RECOMBINED tree. A per-shard green is not a brief-level green. The merges are
   conflict-free by construction (`path-overlap` proved no two shards touch a common file), so a
   conflict means the precondition was violated between approval and merge: STOP, do not resolve by
   hand, re-run the check and report.
4. **Claim granularity.** The brief-level claim is acquired ONCE by the coordinator and held for the
   whole split; shards claim UNDER it with the suffixed key `<repo>--<stream>--<NN>--shard-<name>`,
   using the same verbs and the same liveness rule. The suffix is what stops two shard-workers
   colliding; the shared prefix is what keeps a second dispatcher off the whole brief. The branch/PR
   check runs **per shard** — N shards is N chances to redo work someone already pushed.
5. **Fail-safe.** Any shard that fails, or whose state cannot be read, blocks brief-level completion:
   the draft PR carries a `SHARD-INCOMPLETE` block naming each missing shard and which of the three
   states it is in, and the brief stays `in-progress` — never flipped `implemented` on a partial. A
   coordinator that cannot read shard N reports could-not-check, not success.
