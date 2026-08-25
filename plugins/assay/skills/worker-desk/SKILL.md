---
name: worker-desk
description: >-
  Run the work-dispatch role of the process desk — the second of the four desks in the pipeline
  (intake-desk → worker-desk → pr-review-desk → verify-desk). Keep a standing pool of parallel
  worker agents full, each implementing one board item in its own worktree behind a draft PR, and
  hand the resulting PRs to the pr-review-desk window. Use when asked to "fan out the next batch /
  work the next N briefs in parallel / do what's next in parallel / fan out" — the plural of "work
  on what's next". Reads the Next-up board of every stream root (every sibling checkout carrying
  docs/streams, already priority + staleness + per-stream-capped) PLUS trusted work-ready GitHub
  issues on no board, dispatches one worker per item, and refills each slot the moment its worker
  finishes (never wave-and-stop). Runs SILENT — anything needing a human is a filed GitHub issue
  (question / help wanted / needs-decision), never console narration. Role window, no persona; the
  human driver merges.
---

# Worker Desk (the dispatch half)

> **Home, as of this port.** This file is the portable core of the worker-desk skill — the
> dispatch half of the desk pipeline. A project adopting Assay pairs it with its own project-local
> mechanics: a board generator (`statusgen`), a token-minting helper for whichever write identity
> it chooses, a dispatch-claim helper, and its own owned-repo/root set. Those pieces are project
> config, not part of this portable core, and are shown here only as illustrative examples a
> project's own wrapper fills in with real values.

The **worker-desk** is the dispatch half of the process-desk pipeline — the second of the four
desks (`intake-desk → worker-desk → pr-review-desk → verify-desk`):

- **worker-desk** (this skill, its own window) turns the Next-up board into worker agents + draft PRs.
- **pr-review-desk** (a separate window) reviews those PRs and flips them ready-for-human.
- **The human driver** merges.

This is the parallel, **continuous** form of "work on what's next". Run it in its **own window** as a
standing loop — **ONE window at capacity replaces running two**. This is a role-window with no
persona (the persona convention, if a project uses one, belongs to `the-desk` only); the
register/evidence discipline of `the-desk` applies (read it if not already booted).

**Boot isolation.** Resolve this skill from the shared checkout READ-ONLY, then **immediately
isolate** — `git -C <shared> worktree add <path> refs/remotes/origin/main` (or `EnterWorktree`) —
**before any other command**, then LOCK it: `git worktree lock --reason "worker-desk live session"
<path>` (the cooperative half of the prune liveness guard — prune never touches locked trees;
unlock is automatic when the worktree is removed at session end). The window itself must NEVER
remain homed in the shared checkout: every stray write there lands in the shared tree, and a
shared-homed session is where a dispatcher's prompts pick up shared-checkout paths and propagate
them into workers. If a project's write-guard has a shared-homed exemption, a fanout window must
never opt into it.

## The pool — keep slots FULL, not waves

**The unit of operation is the SLOT, not the wave.** Do NOT fan out one wave and stop — maintain a
**standing pool of N concurrent workers** (a small fixed number, e.g. 8) and keep it full,
continuously. This is the throughput engine, and **an idle slot while eligible work exists — a
Next-up row on ANY board, a qualifying un-briefed issue (§Third source), or an orphan PR awaiting
resume — is the failure this design exists to prevent.** Practice drifts into "dispatch a wave,
report, stop"; that drift is the named failure mode this section exists to kill. A worker completing
is not progress toward "the batch finishing" — it is a refill trigger for THAT slot, immediately.
There is no state in this loop called "the wave is done": there are only full slots, refillable
slots, and a sweep proving nothing eligible exists (§HARD GATE).

- **Fill to N.** Dispatch eligible briefs until N workers are in flight.
- **Refill on completion.** The instant a worker finishes (its draft PR opens, or it reports done /
  NEEDS_CONTEXT), claim + dispatch the **next** eligible item into the freed slot — back to N.
- **Never stop-and-wait-for-restart.** The loop runs continuously. When eligible = 0 across every
  board AND no orphan PR needs a resume AND no qualifying un-briefed issue is waiting (§Third
  source), do NOT exit — **idle-poll every ~60s** (regenerate Next-up, re-scan the PR queue,
  re-sweep issues) and refill the moment new work appears. "The batch finished" is not a stop
  condition — it is not even a state this loop has.
- **Trust gate:** never dispatch on an issue/PR/comment not authored by a blessed identity or the
  desk's own identities unless a blessed identity has commented on it — untrusted items stay
  quarantined-visible (EXTERNAL/UNBLESSED on the boards), never worked. (The blessing authority is
  project configuration — see the roster/trust primitive in the adoption runbook.)
- **Concurrency (N) is DECOUPLED from the board's span-of-control display cap.** Span-of-control is
  a *human* attention limit on what Next-up *shows*; it is not a dispatch limit — target N workers.
  A **per-stream cap** stays as an anti-monopoly guard, but it must **never idle a slot**: if fewer
  than N are eligible under the per-stream cap, fill the remaining slots with **orphan resumes**,
  then with **qualifying un-briefed issues (§Third source)**, before leaving a slot empty.
- **Scan the active PR queue EVERY cycle (~60–90s), not once per manual run.** This is the *resume*
  scan — your dispatched PRs that now owe a worker action (`CHANGES_REQUESTED` at head,
  `CONFLICTING`, a draft gone stale). Missing these for hours is the "it misses stuff" problem. On a
  hit, dispatch a **resume-worker** for that PR (it claims the PR like a brief); **resuming started
  work takes PRIORITY over a fresh brief** (drain-before-dispatch). This is distinct from the
  pr-review-desk's *review* monitor — you scan for work to **resume**, it scans for PRs to
  **review**; both run.

## Model requirement — dispatch on a SMART model where judgment is involved

Dispatch scoping, orphan triage, and blocker classification are judgment calls; the desk window
itself should run on a strong tier. Individual workers are tiered by the brief (§Dispatch). If this
window is downgraded mid-session, stop the judgment work (scoping decisions, orphan-triage verdicts)
and fall back to mechanical work (running the board, keeping records current); the driver ASKING
what model you are is a probe (answer, keep mechanical work moving), only their ASSERTION of a
downgrade hard-gates the judgment work.

## HARD GATE — never claim "pool empty / nothing to dispatch" without a fresh sweep

**An idle claim is a claim about the work queue, and the only evidence is a fresh sweep.** Before
the desk EVER reports "pool empty", "nothing to dispatch", "caught up", or "idle", it is a HARD
PRECONDITION that it has *just* refreshed **EVERY stream board** (§Two boards) AND the per-cycle
orphan scan (`gh pr list --limit 100` across SCAN REPOS) AND the un-briefed-issue sweep (§Third
source, `gh issue list --limit 100` across the same set — both resolved from §THE REPO SET, never a
pasted list) and confirmed zero eligible Next-up rows **on every board** AND zero orphan PRs needing
a resume AND zero qualifying un-briefed issues. No fresh sweep → no idle claim.

**A single-root sweep is `could-not-check`, not idle** — a second, larger board on another root is
routinely invisible from the first. And the board **silently truncates at the per-stream cap**
(§Two boards): "no rows past the ones shown" is not "nothing eligible". "I checked at the top of the
hour" is not fresh; "my dispatched workers haven't finished yet" is not evidence the queue is empty.

**A sweep that failed, errored, or hit the `--limit` cap is `could-not-check` — blind, not idle.**
At 100 results, treat the sweep as possibly truncated and widen rather than claiming zero. Report
that the instrument could not be read and re-sweep before making any state-of-play claim.

## Two boards — the stream queue spans more than one repo

The PR/orphan sweep has always been multi-repo. **The stream board is per-root**, and a single-root
board silently hides a second, larger queue: sibling checkouts carry their own `docs/streams`, and
those are dispatchable work — missing them is the same class of failure as a truncated PR sweep.

### The board truncates SILENTLY — row count is not eligible count

The rows a board shows may be held back by the **per-stream cap**, not by the span-of-control cap,
and a board at exactly the overflow threshold may not emit any overflow line at all — so a desk
reading N rows has **no indication that more eligible briefs exist**. **Never read "N rows on the
board" as "N eligible."** When you need the true counts, regenerate with the option that forces the
counter line to print (e.g. `--overflow-threshold 1`).

### statusgen writes ONE STATUS.md PER ROOT — it does not merge boards

`--root` is repeatable but writes one STATUS.md per root; it does **not** fold one root's briefs
into another's Next-up. Coverage therefore comes from **reading both boards**, not from passing both
roots. And a single PROBLEM (e.g. a stream named the same under two roots) makes statusgen exit
non-zero and suppress *every* board — so an `&&` chain combining roots can short-circuit into *no*
Next-up at all. **Generate each root's board with its own invocation** and read them all; that form
is correct in every case regardless.

### Why NOT a merged board tool on the strength of its help text

A merged cross-repo board tool may advertise exactly the shape this step wants (one ranked board
with a `REPO` column, read-only, pin-verifying) and still return the **wrong rows** — e.g. the
awaiting-verification backlog rather than the dispatch queue. worker-desk dispatches `todo` briefs;
a tool that emits `implemented`/`verified` rows would point every worker at work that is already
done. **Verify what a board tool actually emits against the roots' own Next-up sections before
adopting it here** — a tool named for this job does not necessarily do it. If it is a real defect,
file it against the tool rather than working around it.

### Brief IDs are ambiguous across roots — always qualify

A bare `<stream>/01` can name a *different* brief in each root. In dispatch logs, claim files,
worker prompts and PR bodies, **qualify every brief ID with its repo** — `<repoA>:<stream>/01` vs
`<repoB>:<stream>/01` — and carry that tag into the claim key too, so one root's brief can never
lock another's.

## THE REPO SET — derived once, consumed by both sweeps

**Two sets, one definition point.** This is the ONLY place this skill RESOLVES a repo set; the board
loop (§Procedure 1), the orphan PR sweep and the un-briefed-issue sweep (§Procedure 0) all get their
repos from here. Repo names appearing elsewhere as prose evidence are not read by any step. A second
**operative** list is a bug wherever it appears: a hand-maintained repo list in a skill drifts in
both directions at once — naming repos the tools refuse to act on while omitting one they cover — and
neither drift is visible from inside the file.

| set | what it is | derived from | consumed by |
|---|---|---|---|
| **BOARD ROOTS** | local checkouts carrying `docs/streams` — the dispatch queue | the project's declared topology (`deskroster repos --scope topology`, the rows carrying `root=`) **unioned** with a live `docs/streams` test over the sibling checkouts | §Procedure 1 board loop, §Procedure 2 eligible set |
| **SCAN REPOS** | `owner/repo` slugs swept for orphan PRs and un-briefed issues | the project's declared scan set (`deskroster repos --scope scan`) | §Procedure 0 orphan sweep, §Third source |

**The two sets differ on purpose, and the difference is stated rather than silent.** SCAN REPOS is
the wider one: it covers repos the desk is a front door for but that carry no `docs/streams` (site /
proposal repos — issues and PRs to sweep, no briefs to dispatch). A board root whose slug is missing
from the scan set is a **reportable mismatch**, not something to paper over — its briefs dispatch but
its PRs never get an orphan resume. Print the symmetric difference each boot and file it (§Output
contract) rather than reconciling it by editing this file.

**Deriving BOARD ROOTS.** Take the **union** of the declared roots with a filesystem test, never the
declaration alone:

```bash
# 1. declared roots (may be unavailable — see the three-state rule below)
DECLARED=$(deskroster repos --scope topology 2>/dev/null \
           | awk -F'\t' '{for (i=1;i<=NF;i++) if ($i ~ /^root=/) { sub(/^root=/,"",$i); print $i }}')
# 2. observed roots: sibling checkouts carrying docs/streams. The --git-dir test keeps only real
#    clones: a LINKED WORKTREE answers with an absolute path, a clone answers ".git". Without it a
#    sibling glob returns every session worktree on the machine, i.e. each board swept many times.
OBSERVED=$(for R in . ../*; do
  [ -d "$R/docs/streams" ] && [ "$(git -C "$R" rev-parse --git-dir 2>/dev/null)" = .git ] && echo "$R"
done)
# 3. the set you sweep is the UNION, keyed and ordered by REPO SLUG (never by path)
ROOTS=$(printf '%s\n' "$DECLARED" "$OBSERVED" | grep -v '^$' | while read -r R; do
  [ -d "$R" ] || continue
  U=$(git -C "$R" remote get-url origin 2>/dev/null) || continue
  SLUG=$(printf '%s' "$U" | sed -e 's#.*[:/]\([^/]*/[^/]*\)$#\1#' -e 's#\.git$##')
  printf '%s\t%s\n' "$SLUG" "$(cd "$R" && pwd -P)"
done | sort -u | awk -F'\t' '!seen[$1]++')          # one line per repo: <slug>\t<path>
```

**Key on the repo slug, not the path** — two clones of one repo are one queue, and the boards are
read out of `refs/remotes/origin/main` (§Procedure 1), so *which* clone the dedupe keeps does not
change the board. Slug order also makes the interleave reproducible across machines, where local
directory names differ. Run under **bash**: zsh does not word-split unquoted `$VAR`, so `for R in
$OBSERVED` silently collapses to a single root — read the list, never split it.

**The union is the non-narrowing direction, and that is the whole reason for it.**
Declaration-only goes blind to a root provisioned since the last topology edit (a stream directory
that appeared and the hard-coded list was stale within hours). Observation-only goes blind to a
declared root whose checkout is missing from this machine — which is a **could-not-check**, not an
empty queue. **A root in exactly one of the two lists is named in the report either way; it is never
dropped.** Declared-but-absent = could-not-check (the checkout is not here). Observed-but-undeclared
= a topology gap — dispatch it anyway this cycle and file the gap, because refusing to dispatch a
real queue to punish a missing declaration starves the queue.

**Three-state.** If the declared-topology read is unavailable (the tool does not ship the verb yet,
or exits with a `COULD-NOT-CHECK` line when every requested set is empty), `DECLARED` comes back
empty and BOARD ROOTS is the observed set alone. That is a documented degradation, not a pass: say
**"declared root set could-not-check — swept the observed roots only"** in whatever you report, and
re-check each boot. An empty print is **blind, never zero roots** (§HARD GATE).

**The single-root-blind bug this replaces.** A hard-coded `for R in ../<sibling-a> ../<sibling-b>`
list is written from inside one checkout, where `.` is that repo. Run the same block from a
*different* checkout and `.` is a different repo, one sibling is read twice, and the largest board is
never read at all — the queue leaves with no message and the desk reports a confident short list.
Derivation fixes this because `.` and every sibling are tested the same way, wherever the session is
homed.

### The interleave rule — deterministic, and NOT a new global ranking

Each board is already ranked (statusgen applies priority, staleness exclusion and the per-stream cap
**per root**, and never merges roots). The cross-repo merge therefore **adds no scoring pass of its
own** — a global rank invented here would reorder work against a ranking no board agrees with.

The merge is a **round-robin over BOARD ROOTS in ascending repo-SLUG order: one row per root per
pass, each root's rows consumed top-to-bottom in board order, until every root is exhausted.** Worked
example — roots `A` (rows a1 a2 a3) and `B` (rows b1 b2) merge to `a1 b1 a2 b2 a3`. Two properties
make this the right rule:

- **Order within a root is preserved exactly.** Row *k* never overtakes row *k−1* of the same board,
  so each board's own priority / staleness / cap ordering survives the merge intact.
- **No root is starved by a larger one.** All-of-A-then-B lets a large board hold all N slots
  indefinitely while a small board never dispatches — the asymmetry this rule exists to close.

Slug order — not directory order — makes the result reproducible across sessions and machines, whose
local checkout names differ, so two dispatchers reading the same boards build the same batch and
contend on the SAME claim (§CLAIM before dispatch) instead of racing down different orderings. Every
merged row carries its repo-qualified brief ID (§Brief IDs), and it flows into the claim key, the
worker prompt and the PR.

**A could-not-check root is a row in the report, not a gap in the batch.** A root you could not read
has an **unknown** number of eligible rows, so the batch is a lower bound on the queue and has to be
described as one. Never fold an unreadable root in as an empty one: that is how a narrowed queue
comes to look like a finished one.

## Third source — trusted, work-ready issues on NO board

The boards do not carry everything. A GitHub issue can be real, trusted, implementable work and
still sit stranded for days merely because it is issue-shaped: the intake scanner has not filed its
placeholder yet, or it lives in a repo the placeholder pipeline does not cover. The dispatch pool
therefore has a **third work source** beside the Next-up boards and the orphan-PR sweep: **open
issues, across the same watched-repo set as the orphan sweep, that are trusted + work-ready but
represented by NO brief and NO placeholder on any board.**

**Sweep mechanics.** Each cycle, alongside the §Procedure-0 orphan scan, sweep `gh issue list --repo
<r> --state open --limit 100 --json number,author,title,labels,updatedAt` over the watched repos.
The `--limit` is mandatory and the same truncation rule applies: a sweep that failed, errored, or
hit the cap is **could-not-check — blind, not idle** (§HARD GATE).

**Eligibility — ALL of the following must hold before an issue may fill a slot:**

1. **Trust gate (unchanged, non-negotiable — §The pool):** authored by a blessed identity or the
   desk's own identities, or a blessed identity has commented on it. Untrusted issues stay
   quarantined-visible, never worked.
2. **Un-represented:** no issue placeholder exists for it (stream root or `done/` archive), no brief
   cites it (frontmatter `issues:` or body reference), and no open PR is already working it. If a
   placeholder or brief exists, the item flows through the board like any other row — this source
   never bypasses or duplicates a board entry. **Absence from Next-up is NOT absence of
   representation:** a placeholder or brief can exist yet be board-suppressed (a staleness flag, the
   caps, or a board defect). The un-represented check runs against the FILES and open PRs, never
   against what Next-up currently shows. If represented, trusted, dispatch-worthy work persistently
   fails to surface on any board across cycles, that is a board defect — FILE IT (§Output contract);
   never route around it through this lane.
3. **Work-ready on its face:** the issue body is an implementable spec by the same standard the
   placeholder lane uses (issue body = the spec); it carries none of `question`, `needs-decision`,
   `help wanted`, and is not parked awaiting a human reply.
4. **Needs no triage judgment (the conservative scope line):** single-repo, no open design fork, no
   risk-bearing surface (public-repo copy, security, prod-deploy, irreversible actions). An issue
   that needs scoping, splitting, a decision, or risk assessment is **intake's job, not yours** —
   leave it for the intake desk. When in doubt, leave it: this source exists to catch *obviously
   ready* strays, and erring toward intake preserves the triage role by design.

**Priority — LOWEST of the three sources.** Orphan resumes first (drain-before-dispatch), then
Next-up rows (the curated, prioritized queue), then un-briefed issues into whatever slots remain.
This source prevents idle slots; it does not outrank the board. But the ordering is a tie-break, not
a hold: an empty slot with a qualifying issue and nothing above it in the order dispatches NOW.

**Claim discipline.** Claim under the SAME issue-shaped key the placeholder lane uses —
`<repo>--issue-<NN>` — never a stream-shaped key. Deliberately shared: if the intake scanner files a
placeholder for the same issue mid-flight, both dispatch lanes contend on ONE lock and can never
double-dispatch. The branch/PR check stays authoritative; the worker's PR carries `Refs #NN` (or
`Closes #NN` when the PR is the full fix) so the claim is visible on GitHub.

**Worker conventions** are the issue-placeholder ones: issue body = the spec, own worktree, one
issue = one branch = one draft PR, stop at `implemented`. A blocking question is posted on the ISSUE
with a desk-automation marker + the `question` label, then STOP — with no placeholder file to flip,
the label is the parked-state record (labeled items are WAITING-ON-INPUT, not orphans, and do not
re-qualify for this source while labeled).

**This source does not gut intake triage.** The intake desk remains the front door that turns issues
into briefs; this lane is the safety net underneath it, bounded by rule 4 to strays that need no
triage. If the sweep repeatedly surfaces issues that FAIL the scope line and sit unbriefed, that is
an intake-coverage signal — file it (insight-routing, §Guardrails), do not widen this lane
unilaterally.

## Intra-brief splits — N shards, ONE brief PR (opt-in)

The default is unchanged and stays unchanged: **one worker per brief.** A brief may OPT IN to being
worked by several concurrent shard-workers by declaring `parallel-streams:` (a name plus file globs
per shard). This section is the whole of what that changes. Briefs without the field — every brief
on the boards by default — dispatch exactly as before, and nothing below runs for them.

### The precondition — a declared split is a request, not a permission

Before dispatching any split, run the checker and read its exit code:

```
statusgen shardcheck --brief docs/streams/<stream>/brief-<NN>-<slug>.md --root <that repo's root>
```

**0 checked-clean → the split may dispatch. 1 checked-failed → ONE worker, serially. 2
could-not-check → ONE worker, serially.** 1 and 2 carry the same instruction on purpose: a split
whose safety could not be established is not a split you may run. Refusing a safe split costs
wall-clock; approving an unsafe one costs a red main, so every unproven state resolves to serial.
Falling back to serial is always available and never wrong. **Do not read a refusal as a defect to
route around** — the classes it names (path-overlap, shared-surface, symbol-coupling, dead-glob,
plan-shape) are properties of the brief's declaration against the real tree; the fix is a different
declaration or a serial run, never a narrower check.

### What file-scoping does NOT prevent — the cap, named

File-scoping prevents shards from writing the same bytes. On its own it prevents nothing else:

- **Semantic collision with no textual conflict.** A function grows a new parameter on one branch
  while a second branch still calls it with the old arity. Different files. Both green independently.
  Main red on merge. `shardcheck` catches this class **within one package** and reports every other
  cross-shard pair as a coverage gap → could-not-check → serial.
- **A shared numbering space.** Two briefs authored into one shared doc in parallel each pick the
  same next number, because neither diff shows the other's. `shardcheck` withdraws these shared
  surfaces from every shard; those edits belong to the coordinator, after the shards land.
- **A collision with a DIFFERENT branch.** Nothing here sees outside this brief; merge-time
  re-checking and merge-currency are separate gates and this one does not replace them.

### The recombine model — N worktrees, ONE integration branch, ONE draft PR

One brief = one branch = one PR still holds, so the ambiguity is resolved ONE way:

1. The dispatcher creates the brief's integration branch from fresh `origin/main` and pushes it
   empty. This is the brief's single branch and the eventual PR's head.
2. Each shard-worker gets its **own worktree** (the isolation rule is per worker, unchanged), cut
   from that same integration commit, and pushes a shard branch. A shard-worker never pushes to the
   integration branch and never opens a PR.
3. When every shard has pushed, the **coordinator** merges each shard branch into the integration
   branch, in declared order, and opens the ONE draft PR. The merges are conflict-free *by
   construction* — path-overlap already proved no two shards touch a common file — so a conflict
   here means the precondition was violated between approval and merge: STOP, do not resolve it by
   hand, re-run `shardcheck` and report.
4. The coordinator then makes the shared-surface edits the shards were forbidden (the stream README
   row, any numbering-space entry) on the integration branch, and runs the brief's Verify table
   against the RECOMBINED tree. A per-shard green is not a brief-level green.

### Claim + fail-safe

The brief-level claim (§CLAIM before dispatch) is acquired ONCE by the coordinator and held for the
whole split; shards claim UNDER it with a suffixed key. Any shard that fails, or whose state cannot
be read, blocks brief-level completion: the coordinator leaves the draft PR carrying only the shards
that landed with a `SHARD-INCOMPLETE` block naming each missing shard and its state, the brief stays
`in-progress` (never flipped `implemented` on a partial), and **a shard whose siblings' state could
not be read reports could-not-check, not success** — "my shard is green" is not "the brief is green".

## Setup (once per session)

- **Set the loop identity (for the stop-flag system):** export the loop name this window runs as
  (e.g. `DESK_LOOP=worker-desk`) so per-loop stop flags are honored. Run once at boot and before
  every iteration.
- **Offline-by-default envelope (if the project touches live infrastructure):** quarantine the
  cluster/production credential at boot (e.g. `export KUBECONFIG=/dev/null`) and hand the same line
  to every worker. This is the environment-layer half of an offline default: a probing tool finds NO
  live context and degrades to could-not-check instead of contacting production. An operator who
  genuinely needs live state runs a single command with an explicit per-invocation override from an
  interactive shell; the desk window never re-exports a real config wholesale, and a dispatched
  worker never overrides it at all.
- **Register in the roster:** self-declare this session with `deskroster set --role "worker-desk
  dispatcher"` (out-of-git roster store). The roster keys one beacon per session name, so the
  identity must be per-session: prefer the real session id, falling back to this role's own name —
  never a persona name (personas belong to `the-desk`).
- **Commit identity — inline per commit, NEVER `git config user.email`.** Worker branch commits
  author as the worker App with the **bot USER id** prefix (not the App id): pass
  `git -c user.name="<worker-app-bot>" -c user.email="<bot-user-id>+<worker-app-bot>@users.noreply.github.com" commit …`
  (or export the four `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env vars in the worker's own shell — hand this
  line to every dispatched worker). A plain `git config user.email` is FORBIDDEN in any linked
  worktree: this repo family sets `extensions.worktreeConfig=true` and linked worktrees share one
  `.git/config`, so a plain set clobbers every concurrent session's commit identity — observed live
  as worker/reviewer sessions overwriting another desk's identity. Inline `-c` / per-process env is
  race-proof by construction.
- **Prune stale worktrees at boot and periodically while the loop runs** (bounded growth; the bash
  sandbox and any write-guard depend on it — fanout is the biggest worktree producer, and sprawl
  trips shell resource limits and false-positive alarms): `deskwt prune` (installed binary at
  /opt/desk-tools/bin/). It only removes tracked-clean, fully-merged worktrees; every in-flight
  worker's unmerged/dirty/unpushed worktree is always left. The steady-state form is a supervisor
  timer (`deskwt prune --interval 30m`, launchd / k8s pod / cron). Never hand-delete worktrees.

### Tool wiring — the table workers get verbatim

The installed desk-tools at `/opt/desk-tools/bin/` are the primary path for all outward workflow
verbs. Use them first; fall back to the raw `git`/`gh` path only on exit codes 3 (disabled) or 6
(unverifiable). **Exit 5 (refused) is NEVER a fallback trigger** — the tool returns it precisely for
deliberate safety stops (secret detected by the body-check, body over the size cap, "not this
worktree's origin", "PR not OPEN", head-branch mismatch), and falling back to raw `gh` on exit 5
routes around the very scan/cap/own-PR guard that produced the refusal.

| Verb | Primary (zero-prompt) | Fallback (exit 3/6) |
|------|----------------------|----------------------|
| Create draft PR | `deskpr create --title T (--body-file F \| --body-min B)` | `git push -u origin <branch>` + `gh pr create --draft` |
| Push follow-up | `deskpr update` | `git push` |
| Reply on findings | `deskreply <owner/repo> <pr> --body-file <f>` | `gh pr comment` |
| Worktree prune | `deskwt prune` | (project's prune tool) |

EVERY `--body-file` argument in this table MUST be a per-invocation unique temp file the worker
mints itself — `BODY=$(mktemp "${TMPDIR:-/tmp}/pr-body.XXXXXX")` — never a bare `pr-body.md` in a
shared session scratchpad. Parallel fanout workers each run in their own worktree but share the
session scratchpad; two converging on the same shared body path race, and the loser opens its PR (or
posts its reply) carrying the *other* worker's body — wrong `Closes`/`Refs`, wrong diagnosis text on
a real PR. `mktemp` is collision-proof: it creates the file with `O_EXCL` and echoes the name that
won. Prefer it over a per-worktree path too: an untracked `.pr-body.md` in a worker worktree makes
the prune tools count that worktree `dirty` and never auto-prune it (the exact open-file-table
sprawl the prune step exists to prevent), and a `$(…)`-carrying path can be expanded at
prompt-compose time, baking the *dispatcher's* shared-checkout toplevel into every worker.

`deskreply` has exactly one verb — the two positionals shown above, no `comment` subcommand. A
worker prompt that appends an extra leading token (`deskreply comment …`) gets refused with exit 5
before any reply is posted. **This table is the copy-paste source for worker prompts** precisely so
that error can't recur: copy it from here, not from a per-machine note that can drift.

### Stop-flag check — run at every iteration boundary

Before each loop cycle (orphan sweep, slot refill), check for active stop flags in the project's
roster/config dir:

```bash
[ -f "<config-dir>/STOP" ] && echo "STOP flag active — exiting loop" && exit 0
[ -n "$DESK_LOOP" ] && [ -f "<config-dir>/STOP.$DESK_LOOP" ] && echo "STOP.$DESK_LOOP active — exiting loop" && exit 0
```

A hit means exit cleanly (restart by removing the flag + re-arming). Never halt mid-dispatch.
Precedence: `DISABLED` > `STOP` > `STOP.<name>`. The tool layer independently enforces these flags.

### Output contract — SILENT unless a human is needed; escalation = a FILED ISSUE

This desk's console is not a progress channel, and nobody is watching it — the driver's review
surface is the ISSUE LIST, not console streams. The states collapse to two:

1. **Normal operation → SILENT.** No per-cycle output at all: no dispatch narration, no refill
   confirmations, no idle-poll lines, no board dumps, no "swept N boards" summaries. Every one of
   those events is already recorded where the machinery writes — the dispatch claims
   (`git ls-remote origin 'refs/dispatch/*'`), the roster registrations, the branches and draft PRs
   themselves, and dispatch comments on PRs/issues. Those records ARE the dispatch log; the console
   duplicates none of it. An explicit request from the driver ("show me the board", "what's in
   flight?") still gets a full answer — silence binds unprompted narration, not answers to a human.
2. **Needs a human → FILE A GITHUB ISSUE, never a console print.** A decision fork, a blocker the
   loop cannot resolve, a capability/authority edge, a repeated failure pattern: file an issue in the
   project's own toolkit/methodology repo (insight-routing, §Guardrails; a repo-specific defect goes
   to that repo's own tracker) so the-desk or the intake-desk can review it. Apply the
   escalation-label discipline (§Guardrails): label `question` or `help wanted` WITH a comment
   stating exactly what is needed and from whom; a matured formal fork promotes to `needs-decision`.
   When the escalation concerns a PR/issue already in flight, label + comment THAT item instead of
   filing a duplicate. **The filed issue IS the escalation** — a console line is not.

   **File it with `deskfile new … --raised-by worker`** — the dedupe gate plus the provenance stamp,
   which is what lets the by-desk issue metric see that the DISPATCH loop noticed the problem, as
   opposed to whichever App happened to post it. The role vocabulary is DERIVED from the roster's
   role-bindings; `deskfile` refuses (exit 5) any role the roster does not bind and prints the bound
   set. Omitting the flag is not neutral: the issue lands with **UNKNOWN** provenance, which is the
   absence of an answer and never "a human raised it".

A genuine hard error that halts the loop may still print its final diagnostic — but if the condition
needs a human to clear it, file the issue too; prefer filing over printing in every ambiguous case.

**What silence does NOT change.** The HARD GATE is an *internal-state* rule, not a print
requirement, and survives intact: the per-cycle sweeps still run every cycle, and the desk still may
not TREAT itself as idle — or answer "are you caught up?" — without a fresh sweep proving the
zero-condition.

## Drives — the campaign steer + the at-a-blocker rule

A **drive** is an operator-declared campaign (a PR-reviewed manifest, e.g.
`docs/roadmap/drives/<slug>.yaml`) that the board generator folds into the **Next-up score** as an
additive, attributed steer — so a declared campaign self-prioritizes the fleet with no per-dispatch
hand-holding. The worker-desk **loop does not change**: you read the same re-scored Next-up board and
dispatch the same way. A drive only shifts what floats to the top; it never bypasses eligibility, the
per-stream cap, the span-of-control cap, or the claim filter. Membership is pull-only (in the
manifest) — nothing about a brief in a diff tells you it is drive-covered; the board's decomposed
score is the only signal. A malformed / expired / over-concurrent manifest fails **neutral**: the
board still generates with zero boost — dispatch off it normally, and leave the manifest fix to the
operator.

Two deltas apply to a **drive in flight**:

- **At a blocker, classify-and-take-other-work — never idle, never ping.** When a drive item is
  blocked (an unlanded dep, awaiting a review verdict, needing an operator act, or needing a brief
  authored), the worker **classifies the blocker and picks the next dispatchable item** — from the
  same drive or any other stream. A worker never sits idle on a drive blocker and never pings a human
  from the loop; the operator is surfaced through the board generator's own tracking-issue channel,
  run by the desk/CI, not by a worker. A `needs: brief` plan-gap is **dispatchable authoring work,
  not a stall**.
- **Drive in-flight WIP cap.** Hold concurrent in-flight drive items to a small number, well under
  the concurrent-agent secondary-rate-limit ceiling. This is a drive-scoped ceiling **inside** the
  pool's N slots, not a second global pool: the rest of the pool keeps pulling non-drive work so the
  campaign never starves the board.

## Procedure

0. **Orphan sweep — FIRST at boot, then EVERY cycle.** This is the resume scan the pool runs each
   ~60–90s, not a one-time boot step. Before dispatching any fresh brief, scan the open PRs across
   **SCAN REPOS — resolve the set from §THE REPO SET, do not paste a list here** (exit 6 or an empty
   print is could-not-check, never "no repos"):
   `gh pr list --repo <r> --state open --limit 100 --json number,isDraft,updatedAt,reviewDecision,statusCheckRollup,labels`.
   `labels` is NOT optional — dropping it blinds the sweep to its own §Guardrails WAITING-ON-INPUT
   rule (an ad-hoc query without `labels` re-dispatched a `question`-labeled PR already routed to a
   human). The `--limit` is mandatory — bare `gh pr list` silently caps at 30. At 100 results, treat
   the sweep as possibly truncated and widen rather than claiming zero.

   **READ THE DISPOSITIONS FIRST — before any staleness arithmetic.**
   ```bash
   deskdisposition sweep -R <owner/repo> --limit 100
   # number <TAB> state <TAB> verdict <TAB> dispatch-eligible <TAB> title
   ```
   A PR whose state is `checked-failed` with `SUPERSEDED` or `RESOLVED-ELSEWHERE` is **not an orphan
   and is never re-dispatched** — it is a close-out item for the human/close queue. List it and move
   on. This is the largest waste class: a real cycle re-derived a conclusion an earlier pass had
   already posted, on most of its completed orphan dispatches. `NEEDS-REBASE` is live work and stays
   eligible. `could-not-check` (the tool exits 6, or the repo's list read failed) is **NOT** an empty
   queue and **NOT** dispatch-eligible: report the repo as BLIND for the cycle. Skipping one live PR
   for a cycle is one cycle; re-dispatching on an unread record is the cost this check exists to
   avoid.

   A PR is **ORPHANED** when its disposition read is `checked-clean` AND the worker owes it action
   (`CHANGES_REQUESTED` at the current head, CI red, or a draft whose findings were never answered)
   AND it has had no commit/comment for **> 4h** AND no live dispatch claim exists for it on GitHub.
   **Three guards run on every candidate BEFORE an orphan-resume dispatch** (without them a real
   sweep re-dispatched already-resolved/superseded PRs):
   - **Label exclusion.** A PR carrying `question` or `help wanted` is WAITING-ON-INPUT, NOT an
     orphan — exclude it however stale it looks.
   - **Already-triaged is not neglect.** "No activity for >4h" cannot distinguish a neglected PR from
     a resolved one — both are silent. The durable form of this guard is the disposition read above.
     Fall back to reading the PR's most recent comment ONLY for PRs whose verdict predates the
     record: if it is itself a bot/worker/desk marker (a worker's recommend-close note, a desk
     `question`-routing comment), the PR is **ALREADY-TRIAGED** — surface it to the human queue
     (label + comment on the PR), do NOT re-dispatch, and record the verdict with `deskdisposition
     set` so the NEXT sweep does not re-read the prose.
   - **Supersession check (cheap, pre-claim).** If the PR body references `closes #N`/`fixes #N`,
     verify that issue is not already CLOSED by a *different* merged PR; and/or check the target
     brief's stream README for `status: implemented` pointing at another PR. A superseded PR gets
     surfaced as recommend-close — never a resume dispatch — **and the finding is written with
     `deskdisposition set --verdict SUPERSEDED --evidence <url>`**, so this check runs once for that
     PR and never again.

   **Resuming an orphan takes PRIORITY over starting a fresh brief.** Dispatch the resume-worker WITH
   the PR's open findings as its task; it claims the PR like a brief. PRs that are
   approved-awaiting-merge or ready-flipped are NOT orphans (they wait on the human, not a worker).
   **The resume-worker's dispatch MUST carry the write side of the same record** — a worker that
   concludes the PR is dead records it before releasing, never as prose:
   ```bash
   deskdisposition set -R <owner/repo> --pr <N> --verdict SUPERSEDED --evidence <url that landed it instead>
   ```
   (`RESOLVED-ELSEWHERE` when the outcome was reached another way; `NEEDS-REBASE` when it is still
   live work.) The verb writes a label plus an evidence-carrying marker comment and is idempotent. It
   does **not** close the PR — the close is the human-authorized act. Without this write, the next
   cycle's sweep has nothing to read and the re-dispatch happens again. **In the same cycle, run the
   un-briefed-issue sweep** (§Third source) over the same repo set, with the same mandatory `--limit`
   and the same at-cap = could-not-check rule.

1. **Sync to fresh `origin/main`, then refresh + read Next-up — on EVERY stream board (§Two boards).**
   Read from current `origin/main` for each root (Next-up is generated from main — a stale checkout
   yields a stale board, so workers collide on already-taken briefs or miss new ones). The root list
   is **derived, never pasted** — `$ROOTS` is the union computed in §THE REPO SET, and `$MINE` is your
   own repo's origin so the root you regenerate is not also read from `origin/main`:
   ```bash
   # your own repo: fetch and regenerate, chained so a failed fetch never yields a board.
   # Regen is WRITE mode (STATUS.md + the generated FINDINGS.md view) — safe ONLY because this desk
   # runs in its own worktree; discard the churn once read, never commit it, never regen in shared.
   git fetch origin && statusgen --root . && sed -n '/Next up/,/^## /p' STATUS.md
   git checkout -- STATUS.md docs/streams/FINDINGS.md 2>/dev/null || true
   MINE=$(git remote get-url origin)
   SCRATCH=${SCRATCH:-${TMPDIR:-/tmp}}   # local scratch — never a sibling's working tree

   # every OTHER root in $ROOTS (<slug>\t<path>, slug-sorted): fetch, then read main's board
   # WITHOUT writing to it. Read the list — do NOT `for R in $ROOTS` (zsh will not split it).
   printf '%s\n' "$ROOTS" | while IFS=$'\t' read -r SLUG R; do
     [ -n "$R" ] || continue
     [ "$(git -C "$R" remote get-url origin 2>/dev/null)" = "$MINE" ] && continue   # own repo, done above
     [ -d "$R/docs/streams" ] || { echo "$SLUG: no docs/streams — not a board, skipped"; continue; }
     if ! git -C "$R" fetch --quiet origin; then
       echo "$SLUG: COULD-NOT-CHECK (fetch failed) — this is NOT an empty queue"; continue
     fi
     if ! git -C "$R" cat-file -e refs/remotes/origin/main:STATUS.md 2>/dev/null; then
       echo "$SLUG: has docs/streams but no STATUS.md on main — board not generated yet"; continue
     fi
     BOARD=$(mktemp "$SCRATCH/board-${SLUG//\//-}.XXXXXX")           # per-invocation scratch path
     git -C "$R" show refs/remotes/origin/main:STATUS.md > "$BOARD"
     echo "== $SLUG =="; sed -n '/Next up/,/^## /p' "$BOARD"; rm -f "$BOARD"
   done
   ```
   Every root declared in the topology but absent from `$ROOTS` (no checkout on this machine) is
   **could-not-check** and is named next to the boards you did read. Seven things about this block are
   load-bearing; do not "simplify" any of them back:
   - **`$ROOTS` is derived per cycle, not pasted.** A literal root list here is the single-root-blind
     bug (§THE REPO SET): read from a different worktree it skips the largest board and says nothing.
   - **Boards are labelled by repo SLUG, not by directory name**, so a board read from a
     differently-named clone still reports as the repo it is, matching the tag on every dispatched row.
   - **`refs/remotes/origin/main`, spelled in full.** A bare `origin/main` resolves against a stale
     LOCAL branch named `origin/main` if a sibling has one, so the board you read is not the remote's.
   - **The `&&` chain is the freshness guarantee.** Put `git fetch origin` on its own line and a
     failed fetch still prints a board at rc=0, and the desk reads a stale sweep as a fresh one.
   - **Every root gets fetched.** Fetching only your own repo and reading a sibling's working tree is
     the same stale-board failure, moved to a checkout nobody owns. A shared sibling clone can sit
     dozens of commits behind `origin/main` for a day at a time.
   - **Other roots are read `origin/main:STATUS.md`, never regenerated.** `statusgen --root ../X`
     **writes** and leaves that shared clone dirty; `git show` reads the object straight out of the
     freshly-fetched ref — no write, no working-tree dependency, and it is main's own CI-generated
     board. The trade is that its claim-filtering is as of the last CI regen; that is safe, because
     the actual mutual exclusion is the GitHub-durable dispatch claim plus the branch check at
     dispatch time (§Procedure 2), which runs against live state.
   - **Separate invocations, deliberately** — do NOT combine roots into one `statusgen` call
     (§Two boards: a single PROBLEM makes it exit non-zero and write NOTHING).

   **Run this — and EVERY command — from YOUR OWN worktree.** Never `cd` into the shared checkout;
   "sync fresh origin/main" means `git fetch` + regenerate IN YOUR WORKTREE, not "go where main is."
   The shared checkout is read-only to you (`git -C <shared>` at most). A confirmed leak: a fanout
   session `cd`'d to the shared checkout for this exact boot step, then propagated shared-checkout
   paths into its dispatch prompts — workers trashed the shared tree. Next-up already applies
   priority, staleness (⚠ stale-flagged briefs are excluded) and the per-stream cap — do NOT
   hand-pick "the next brief in a stream"; take from the board.

2. **Scope the refill.** The eligible set = every Next-up row across every board, **merged by the
   round-robin interleave rule (§The interleave rule)** — sorted root order, one row per root per
   pass, each row tagged with its repo-qualified brief ID — plus qualifying §Third-source issues for
   slots the boards and orphan-resumes cannot fill. The merge applies no scoring of its own. **Roots
   that came back could-not-check are named alongside the batch**, and the batch is described as a
   lower bound on the queue — never as the whole of it. If the driver gave a count ("next 3"), take
   the top N **of the merged order**; scoping bounds THIS refill, never the loop. **Exclude briefs
   whose `depends:` are not yet `done`.**

   **INCLUDE issue-placeholders — `issue-<NN>` rows ARE yours to dispatch.** They flow through the
   normal `Next-up → worker-desk → draft-PR → review` pipeline like any other brief: the intake-desk
   only *files* the placeholder (the scanner PR); dispatch belongs here. Claim an issue placeholder
   under the issue-shaped key `<repo>--issue-<NN>` (NOT the general `<repo>--<stream>--<NN>` form — a
   placeholder has no `<stream>`). The shared claim stays as the double-dispatch guard: use this SAME
   key for every issue-shaped brief so any two dispatchers share one claim and never collide.

   **`gate: human` / `irreversible: yes` briefs DISPATCH NORMALLY** — the human gate binds APPROVAL
   (review, merge, verify sign-off), not implementation: a worker in a worktree behind a draft PR is
   inert, and every human control point already sits downstream. The worker prompt for a gate:human
   brief must say so: stop at `implemented`, sign-off is human, and if the brief's Task has an
   explicit human co-execution step (cutovers, repo bootstrap), prepare everything, STOP at the
   documented stop-point, and report BLOCKED-ON-HUMAN on the PR rather than waiting silently. Record
   which dispatched briefs are gate:human in the dispatch records so the sign-off volume is visible
   from the PR queue, not a console stream.

   **CLAIM before dispatch — the claim is GITHUB-DURABLE, never a local file.** The branch/PR check
   alone has a race window — between reading the board and the worker's first push, a second
   dispatcher sees the brief as free. The claim that closes it lives on GitHub, in the brief's own
   repo, where **every desk on every machine reads it**: a `refs/dispatch/<id>` ref created with
   `POST /repos/{o}/{r}/git/refs`, which is atomic create-if-absent (201 for exactly one racer, 422
   for all the others). A machine-local file lock does NOT do this job — it does nothing for two
   desks on different machines. **Do not fall back to it for dispatch, and do not re-introduce the
   shell noclobber idiom `(set -C; … > "$f")`** — a write-guard blocks that redirect and the
   Write-tool fallback has no `O_EXCL`, so every racer "succeeds".
   ```
   # the project's dispatch-claim helper wraps these; the gh calls are the contract:
   dispatch-claim acquire  "<repo>--<stream>--<NN>" --repo <owner/repo> --owner "$SESSION_NAME"
   dispatch-claim progress "<repo>--<stream>--<NN>" --repo <owner/repo> --owner "$SESSION_NAME" --branch <b>
   dispatch-claim release  "<repo>--<stream>--<NN>" --repo <owner/repo>
   ```
   If the helper is not reachable, the mechanism is three `gh` calls and you run them directly — the
   ref-create is the whole lock:
   ```
   base=$(gh api repos/$R/git/ref/heads/main --jq .object.sha)
   tag=$(gh api repos/$R/git/tags -f tag="dispatch/$ID" -f object="$base" -f type=commit \
         -f message="dispatch-claim $ID owner=$SESSION_NAME state=claimed branch=-" --jq .sha)
   gh api repos/$R/git/refs -f ref="refs/dispatch/$ID" -f sha="$tag"   # 201 = yours · 422 = taken
   ```
   The tag hangs off main's own commit, so a claim is **zero-diff**: no commit, no PR, no issue
   clutter, and no workflow trigger. `tagger` is deliberately omitted so **GitHub** stamps the time —
   one clock for every machine, so a racing desk cannot back-date its own claim with a skewed local
   clock (skew resistance, not an authorization boundary). A human sees the whole set with
   `git ls-remote origin 'refs/dispatch/*'`. The `<repo>` prefix is **mandatory** — two repos can own
   a stream of the same name, so an unqualified name cross-locks the wrong brief.
   - **Exit 0 → acquired** (a fresh create, or a stale reclaim the tool already applied): dispatch it.
   - **Exit 5 → refused**, a live holder owns it: SKIP. The refusal prints a `DEDUP <id> — already
     claimed by owner=<desk> state=<…> …` line naming what it deduplicated against — carry that line
     into whatever you report.
   - **Exit 6 → unverifiable** (GitHub unreachable, an unreadable claim): do **not** proceed as if
     free — treat it like exit 5 for this cycle (skip; if it persists, file it, §Output contract) and
     retry next cycle. Fail-closed is the design: a claim you could not read is never "assume free."

   **Two-phase liveness — `acquire` then `progress`.** A claim carries a STATE, because a TTL alone
   cannot tell claimed-and-working from claimed-and-died: a dispatcher that claims briefs and never
   launches a worker blocks a second dispatcher off the whole critical path for the TTL. So `acquire`
   writes `state=claimed` with a SHORT TTL (e.g. 20 minutes); the instant the worker agent is
   actually launched you run `progress … --branch <its branch>`, which writes `state=dispatched` and
   restores the LONG TTL (e.g. 120 minutes). A dispatcher that cannot get from `acquire` to
   `progress` inside the short TTL is dead and its claim is reclaimable. **Running `progress` is not
   optional** — skipping it makes your own live worker look dead. Breaking a dead claim is `steal <id>
   --reason "<why>"`, never a hand-delete: it requires a reason and writes it into the replacement
   claim. Never claim more briefs than you are dispatching this cycle, and **release every claim you
   did not dispatch** before the cycle ends.

   **Acquiring a claim is NOT evidence the work is unclaimed.** A claim records only "this dispatcher
   is working on it". A board row reading `todo` can already have an OPEN or even MERGED PR — measured
   in practice at roughly one `todo` row in six. So the gh branch/PR check is not an optimisation you
   may skip on a green `acquire`: it is the evidence, and it must cover **merged** PRs as well as open
   ones. A check that could not reach GitHub is `could-not-check`, never `unclaimed`.

   **Serialize out-of-repo briefs.** A brief whose Context declares `out-of-repo files:` (paths
   outside the repo) gets no worktree isolation and no branch-as-claim, and its edits go live in every
   session immediately — dispatch at most ONE such brief at a time across ALL streams; the Context
   declaration is the claim, so check in-flight PRs/briefs for overlapping declarations first.

   The human-legible **board claim** — the brief's row in its stream README — is the WORKER's, on its
   own branch: the first commit flips the row `todo` → `in-progress`; the last commit flips it to
   `implemented` with the PR link (§Board row below). It lands with the PR — nobody pushes board
   rows to main directly. The `refs/dispatch/<id>` claim above is the one that arbitrates the
   dispatch race; the row is what a human reads the stream's state from.

3. **Dispatch one worker per brief**, concurrently (single message, multiple `Agent` calls).
   **Exception, opt-in only:** a brief declaring `parallel-streams:` whose `statusgen shardcheck`
   exits **0** dispatches one worker per shard onto ONE integration branch and ONE draft PR (§Intra-
   brief splits). Exit 1 or 2 (and any brief without the field) dispatches exactly one worker. Each
   worker:
   - **isolation: worktree** — its own worktree, NEVER the shared checkout. Create from fresh main
     with `git worktree add <path> refs/remotes/origin/main --detach`. Two primitives there are
     load-bearing:
     - `--detach`: without it, `git worktree add -b <branch>` off HEAD copies the current checkout's
       branch, dragging a sibling's unreviewed commits into the new branch. Every branch MUST start
       from main, never from a sibling.
     - **`refs/remotes/origin/main`, spelled in full**: the bare `origin/main` is AMBIGUOUS in any
       checkout that has ever acquired a local branch literally named `origin/main`. rev-parse
       precedence puts `refs/heads/` AHEAD of `refs/remotes/`, so the worktree silently checks out the
       STALE local branch and prints only `warning: refname 'origin/main' is ambiguous.` — the worker
       then starts dozens of commits behind and every push carries the deficit.
   - **Worktree the brief's OWN repo** (§Two boards, §THE REPO SET): a brief off another board
     branches from THAT repo's `origin/main` and its PR opens THERE. **A worker never assumes the
     desk's own repo.** Every dispatched row carries a repo, so **four things go in the prompt
     verbatim, from the merged row's tag** — no worker infers any of them:
     1. **target repo** — the `owner/repo` slug the brief belongs to.
     2. **checkout base** — that repo's BOARD ROOT path, used only as the `git -C <root>` source to
        branch from; it is never the worker's writable root.
     3. **"isolate in an owned worktree OF THAT REPO"** — the literal instruction, plus the command:
        `git -C <that repo's root> worktree add <path> refs/remotes/origin/main --detach` (or a
        `git clone` where no checkout exists). Give the worker the command, not just the repo name.
     4. **open the draft PR IN THAT REPO** — `deskpr create` run from inside that worktree.

     A worker handed a foreign-repo brief but a wrong-repo worktree will "helpfully" recreate the
     brief's files in the wrong repo, and the PR then reads as real work on a repo that never asked
     for it. **A cross-repo worker MUST also be dispatched with Agent `isolation: worktree`** (the
     harness layer, not just the git layer) so its hook-payload cwd is never the shared checkout — a
     /tmp clone does NOT isolate the payload cwd, and a shared-homed worker gets false-positive
     write-guard blocks it then tunnels around, which is the real failure.
   - **tier by effort, overridden by exec-tier**: the brief's `exec-tier` field (absent = `any`)
     signals a minimum execution-model tier — `strong` dispatches only to a session-tier (strong)
     model, never an economy tier; `any` follows the effort-keyed default. A worker prompt for a
     `strong` brief MUST include the pickup-STOP text: "If you are a fast/cheap-tier model, STOP —
     this brief requires a strong implementer. Report which model you are and hand back." The
     effort-keyed default underneath: small effort may run at your session tier; medium/large dispatch
     to a cheap-tier subagent behind the verify/review gates.
   - **task**: implement the brief per its Context + Task + Ground rules; run the brief's **Verify
     table locally**; then **push its own feature branch and open a DRAFT PR** (`deskpr create`, or
     `git push -u origin <branch>` + `gh pr create --draft`); **stop at `implemented`** (never set
     verified/done, never flip ready — that's the review desk + the human). One brief = one branch =
     one PR.
   - **Board row — part of the deliverable, not a state name.** `implemented` is an EDIT the worker
     makes, not a status it reaches: the PR's last commit flips the brief's row in
     `docs/streams/<stream>/README.md` from `in-progress` to `implemented` (Status cell → `implemented`,
     Reviewed/Verified untouched) and links the PR. A PR whose diff has no `docs/streams/<stream>/README.md`
     change is INCOMPLETE — the review desk bounces it, and the verify desk's `statusgen` board will
     keep reporting the brief as open after merge (the phantom-merged class: deliverable on main,
     row still `todo`). Workers on a GitHub issue with no brief have no row to flip and skip this.
     The first commit on the branch made the `in-progress` flip (§Board claim); if it didn't,
     make both flips now.
   - **Self-register on the roster** the instant the draft PR is open: `deskroster set --repo
     <short-repo> --pr <N> --what "<brief description>"` — so `deskroster list` answers "open work →
     session" without a manual round-trip.
   - **Model-stamp the draft PR — the DESK does this, not the worker:** the instant the worker's draft
     PR opens, the dispatcher applies TWO labels **under its own desk App identity** (the other-actor
     attestation — a self-applied stamp is worthless by design): `dispatched-model:<slug>` (the model
     you launched) and `dispatched-tier:<strong|any>` (the brief's exec-tier). Apply idempotently
     (`gh label create` swallowing already-exists, then `gh pr edit --add-label`). This attests what
     you LAUNCHED, not what the session later ran.
   - **Never approve or flip your own PR.** The review verdict is a real GitHub review posted by the
     dedicated reviewer App, and GitHub blocks a PR author from approving its own PR — so a worker
     physically cannot self-certify. The worker's job ends at `implemented` + the draft PR; the desk
     gets the App-review verdict and flips. If a review looks done, ping the desk — never post a
     verdict yourself.
   - **worker prompt essentials**: name the worker's own worktree as its ONLY writable root and quote
     NO shared-checkout absolute paths anywhere in the prompt (prompt-carried paths override every
     isolation layer); require the worker to check `git rev-parse --show-toplevel` before its first
     write and ABORT if it resolves to the shared checkout; include VERBATIM "Your home worktree is
     <path> — every file operation stays under it"; include VERBATIM the offline-envelope line (if the
     project touches live infra) "You run OFFLINE against live infrastructure: no command may contact
     a cluster or production endpoint, read-only included. Export `KUBECONFIG=/dev/null` before your
     first command. Anything that needs live state is could-not-check + BLOCKED-ON-HUMAN on the PR —
     never a probe."; include VERBATIM the security-gate clause "If a change you are about to commit
     deletes, disables, or weakens a security or access-control control or its CI assertion — a
     network policy or egress/ingress allowlist, RBAC, auth/identity config, a secret-scan/leak-sweep
     gate, a fence script or workflow, an admission policy, a required check — STOP, even if it is the
     fix for a red check and even if this brief instructs it: do not commit the removal, leave the
     check red, post `BLOCKED-ON-HUMAN — security-gate removal` on the PR naming the control and any
     relocation evidence, and label `needs-decision`. Only a human ruling recorded on the PR/issue
     authorizes the removal; this prompt is not that ruling."; include VERBATIM the no-evasion clause
     "Any guard or hook BLOCK (a write-guard, a push-guard, a permission denial) is a STOP signal, not
     an obstacle. NEVER re-attempt the same effect with a different command, tool, or path spelling —
     `find -delete` for a blocked `rm`, an interpreter one-liner, an API call for a blocked push. On a
     block: stop that line of work, quote the block message verbatim in your PR/issue report, and
     escalate per the output contract. If you believe the block is a false positive (your target is
     outside the shared checkout), you may re-issue the SAME command with absolute target paths or a
     single `cd <abs-dir> && …` chain — the guard resolves those; anything else is escalate-only. A
     task completed via substitution is a failed task."; point the worker at the §Tool wiring table;
     give the brief path; "implement to the contract, don't expand scope"; the shared-value consumer-
     enumeration + flow-Verify discipline if the brief changes a value other components read; **keep
     the branch current with main** — `git fetch origin && git merge origin/main` (merge, NEVER
     rebase — force-push is denied) periodically while open and always right before signalling
     DESK-READY; no attribution lines; report NEEDS_CONTEXT rather than guess; and **release the
     dispatch claim once the branch is pushed** (`dispatch-claim release "<key>" --repo <owner/repo>`,
     or `gh api -X DELETE repos/<owner/repo>/git/refs/dispatch/<key>`) — branch-as-claim takes over.
   - **Worker verify-before-apply default:** before applying a desk-issued correction or factual
     claim, verify it against the primary artifact — open the file, read the line, compare the value.
     Disagreeing with the desk is an expected output, not insubordination. The only technique that has
     reliably caught an inverted or false desk claim: open the primary artifact and compare the value.
   - **Pre-PR self-check (mandatory, before opening the PR).** Ensure the check runs against current
     main; **spell the base `refs/remotes/origin/main` in every one of these commands** (bare
     `origin/main` succeeds against a stale stray branch, so the could-not-check line never fires and
     every check underneath silently runs against the wrong base):
     ```
     git fetch origin
     git rev-parse --verify refs/remotes/origin/main >/dev/null 2>&1 || { echo "could-not-check: refs/remotes/origin/main unresolvable"; exit 1; }
     git rev-parse --verify --quiet refs/heads/origin/main >/dev/null 2>&1 && echo "WARNING: a stray local branch named origin/main exists here — every bare origin/main below would resolve to IT"
     git merge-base --is-ancestor refs/remotes/origin/main HEAD || echo "NOTE: this branch is not on top of current main (behind by $(git rev-list --count HEAD..refs/remotes/origin/main))"
     ```
     Then `git log --oneline refs/remotes/origin/main..HEAD`: if it lists ANY commit this worker did
     not author (a sibling's unreviewed code from a stale base), STOP and re-cut. Then assert every
     commit whose subject claims to be a merge has at least two parents:
     ```
     git log --format='%H %s' refs/remotes/origin/main..HEAD | grep -iE '(^|[[:space:]])merged?([[:space:][:punct:]]|$)' | while read h rest; do
       [ "$(git cat-file -p $h | grep -c '^parent ')" -ge 2 ] || echo "SINGLE-PARENT MASQUERADE: $h $rest"
     done
     ```
     A commit whose subject claims a merge but has one parent is a fake rebase — STOP and re-cut. If
     both checks pass, output `echo "CLEAN: 0 foreign commits"` so the worker and the desk can
     distinguish "looked and found nothing" from "never looked." These checks are ALSO enforced
     mechanically by the repo's pre-push hook, so a worker who skips the manual step still gets caught
     at push time; when the hook cannot determine the base it says `COULD-NOT-CHECK` and allows the
     push (it is a client-side hook, not a boundary) — read that as "unverified", never "clean".
   - **PR watch-loop (after opening the draft PR)**: arm a Monitor (or poll each turn) on `gh pr view
     <N> --json state,mergeable,reviews` — ONE loop, three triggers: new review → work the findings;
     `mergeable: CONFLICTING` → `git fetch origin && git merge origin/main`, resolve, push
     immediately, and note the resolution on the PR; `state: MERGED|CLOSED` → `deskroster drop --repo
     <short-repo> --pr <N>`, then STOP immediately. **Squash-merge awareness**: a sibling PR landing
     via squash makes same-content conflicts likely — take main's side, re-apply your own edits.
   - **issue-placeholder worker prompt.** An `issue-<NN>` placeholder points at a GitHub issue whose
     **body IS the spec** — the placeholder carries only scheduling metadata. Hand the worker: "Fix
     `<repo>#<NN>` — run `gh issue view <NN> -R <repo>`, its body is your spec. Work in your OWN
     worktree off `origin/main`, never the shared checkout. Implement, run the issue's stated checks,
     push your branch + open a **DRAFT PR** — `Refs <repo>#<NN>` normally, `Closes #<NN>` **only** when
     the merge fully resolves it with no on-cluster/human follow-up (a wrong `Refs`/`Closes` choice
     either leaves a fixed issue open or closes one prematurely) — stop at `implemented`, never flip
     ready or merge." The worker-App mint, the tier-by-effort rule, and the pre-PR self-check apply
     unchanged.
   - **close-candidate variant.** A placeholder the intake desk marked `close-candidate:
     <FIXED-NOT-CLOSED|WONTFIX|DUPLICATE|STALE>` is work whose **deliverable is a reviewed close
     carrier**, NOT an implementation fix. Dispatch it like any issue row (same `<repo>--issue-<NN>`
     claim key); tell the worker to author the resolution claim + evidence for the stated verdict with
     `Closes #<N>` and open the draft PR — the reviewer judges the close CLAIM, and the merge closes
     the issue.
   - **issue-placeholder question protocol**: a worker on an `issue-<NN>` placeholder that hits a
     blocking question MUST (a) post the question as a comment on the GitHub issue, including a
     desk-automation marker in the body so the unblock scanner can tell the automation's own question
     from a human answer (a shared bot login alone cannot gate); (b) set `blocked:
     awaiting-issue-response` and `blockedAt: <ISO 8601>` in the placeholder frontmatter; (c) commit
     the placeholder change; (d) STOP. A non-bot, non-marker human reply on the issue clears it on the
     next scanner sweep.

   **The instant the `Agent` calls are made, advance every claim you just dispatched** —
   `dispatch-claim progress "<key>" --repo <owner/repo> --branch <the worker's branch>`. This is the
   liveness half of the claim: it moves the claim from `state=claimed` (short TTL) to
   `state=dispatched` (long TTL) and records the branch that will take over. A brief you claimed but
   decided NOT to dispatch is `release`d in the same breath.

4. **Refill — do NOT hand off and stop.** The draft PRs surface in the pr-review-desk window's
   *review* monitor automatically (you don't review your own dispatched work — independent review is
   the point). But YOU keep going: on each worker completion, run the per-cycle PR scan (§The pool),
   then **immediately dispatch the next eligible item — an orphan-resume first, then a Next-up row,
   then a qualifying un-briefed issue (§Third source) — to refill the freed slot**, holding the pool
   at N. **Refresh EVERY Next-up board each cycle** (re-run step 1's block in your worktree) so work
   that just became eligible on *any* board enters the pool. The per-stream cap is per stream, and a
   same-named stream in different roots counts separately.

5. **Keep the records current — silently (§Output contract).** The running dispatch log
   (repo-qualified brief → worker → branch, each slot refill and resume) lives in the records the
   machinery already writes — the `refs/dispatch/*` claims, `deskroster` registrations, and comments
   on the PRs/issues themselves — so the driver and the review desk can see what's in flight without a
   console stream. Anything that needs a human is a FILED ISSUE, never narration. The loop pauses only
   to **idle-poll** (§The pool) when eligible = 0 on every board AND no orphan needs a resume AND no
   qualifying un-briefed issue is waiting — never because "the wave finished."

## Guardrails

- **Insight-routing:** a systemic/process insight produced in passing (a wrap-up, a dispatch or drain
  note, an Evidence aside, a "this keeps recurring" observation) MUST also be filed as an issue in the
  project's own toolkit/methodology repo — commentary is not a register. Include the triggering
  evidence and affected loops. Repo-specific defects still go to that repo's own tracker (label `bug`).
- **Escalation labels:** any desk/loop may label a PR or issue `question` (needs an answer from the
  driver or a stronger-tier model to proceed — the item PARKS) or `help wanted` (the desk hit its
  capability/authority edge). Both are GitHub default labels — they exist in every repo, no setup.
  Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from whom when
  labeling; whoever answers removes the label with their response. A `question` that matures into a
  formal decision fork promotes to `needs-decision` with the pros/cons template. Labeled items are
  WAITING-ON-INPUT: they join the human/escalation queue and are NOT orphans for the worker sweep.
- **One brief = one branch = one PR.** A worker that discovers its brief is too big STOPS and splits
  per author-brief rules; it keeps only the piece it was mid-implementing, the rest returns to the
  board as `todo`.
- **Worktree isolation is mandatory** — parallel workers mutating a shared checkout is the incident
  this system exists to prevent (`git restore`/`clean` wiped work; a broad `git add` swept another
  session's staging). Each worker owns its worktree. Isolation must hold for the DISPATCHER and its
  prompts too, not just workers: a session boots clean in a worktree, `cd`s to the shared checkout
  out of habit, and its dispatch prompts then carry shared paths. While workers are in flight,
  `git -C <shared> status --porcelain` each cycle — ANY dirt is an alarm. Never `git restore`/`clean`
  a shared checkout; isolate in your own worktree.
- **A guard/hook BLOCK is a STOP signal, never a puzzle.** The same clause the worker-prompt
  essentials hand every worker VERBATIM, standing here so it survives prompt truncation: "Any guard
  or hook BLOCK is a STOP signal, not an obstacle. NEVER re-attempt the same effect with a different
  command, tool, or path spelling. On a block: stop that line of work, quote the block message
  verbatim in your PR/issue report, and escalate per the output contract. If you believe the block is
  a false positive (your target is outside the shared checkout), you may re-issue the SAME command
  with absolute target paths or a single `cd <abs-dir> && …` chain — anything else is escalate-only.
  A task completed via substitution is a failed task."
- **gate:human briefs dispatch like any other** — the gate is the human's APPROVAL at
  review/merge/verify, never a dispatch hold; co-execution steps stop and report BLOCKED-ON-HUMAN.
- **Security-gate removal is gate:human BEFORE the commit, not only at approval.** The bullet above
  ("the gate binds approval, not implementation") does NOT extend to a diff whose effect is deleting,
  disabling, or weakening a security or access-control control **or its CI assertion** (network
  policy / egress allowlist, RBAC, auth config, secret-scan / leak-sweep gate, fence script/workflow,
  admission policy, required check — including to green a red check). Such a diff erases the red-check
  signal that makes the decision visible downstream, so "a worker behind a draft PR is inert" fails
  for it. A dispatcher never writes that removal into a brief or worker prompt — briefing it launders
  a gate:human decision into a mechanical task; the dispatcher files the `needs-decision` fork
  instead, and dispatches the removal only after a human ruling is recorded on the PR/issue.
- **Dispatch claims are dispatch-scoped, not work-scoped** — `refs/dispatch/<id>` is created just
  before dispatch and deleted at the worker's first branch push; a `git ls-remote origin
  'refs/dispatch/*'` full of old refs means dead dispatchers, not claimed work (check the state and
  age the claim carries, not existence alone — `state=claimed` past the short TTL is a dead
  dispatcher).
- **The out-of-repo surface is serialized, never parallel** — max one in-flight brief touching it.
  Workers stage those edits as diffs in the PR and apply to the live files only as the LAST step
  before `implemented`.
- **A merged/closed PR is DONE** — the worker stops; follow-up is a new PR.
- **Git push policy (ONE policy, role-keyed):** MERGE IS ALWAYS the driver's, and nobody triggers
  workflows or runs mutating cluster commands without their go. **Branch push + draft PR is
  standing-authorized for every desk/loop** — the worker loop (`git push -u origin <branch>` +
  `gh pr create --draft`). **The verify desk lands its own work**: its Evidence + status flips commit
  straight to `main` as the project directs — no push-go is needed there and none should be waited
  for. Any `main` push not covered by a standing authorization is gated on the driver's explicit go;
  committing local work is always fine. A guard/hook-BLOCKED push is a STOP signal — never route the
  same write through another tool. Each desk's own grants and denials (what it may flip, file, close,
  or land) stay in its skill, directly below this block.
  - Desk-specific: **any cluster or live-infrastructure contact is outside a worker's envelope —
    read-only included: a read-only probe against a live control plane is a policy violation, not a
    safe shortcut, and "it only read" is not a defense.** A worker that needs live state reports
    could-not-check + BLOCKED-ON-HUMAN, never a probe.
- No attribution lines anywhere: no `Co-Authored-By`, no "Generated with …" in commits, PRs, issues,
  or comments.
