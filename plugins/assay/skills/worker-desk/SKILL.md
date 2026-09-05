---
name: worker-desk
description: Run the work-dispatch role of the process desk — keep a standing pool of parallel worker agents full at the width the roster declares, each implementing one item in its own worktree behind a draft PR. Use when human:<name> says "fan out the next batch / work the next N briefs in parallel / do what's next in parallel / fan out", i.e. the plural of "work on what's next". Reads the Next-up board of every stream root (this repo + each sibling carrying docs/streams, e.g. ../repo-a and ../repo-b; already priority + staleness + 4-per-stream-capped) PLUS trusted work-ready GitHub issues on no board, dispatches one worker per item, refills each slot the moment its worker finishes (never wave-and-stop), and hands the resulting draft PRs to the pr-review-desk window. Runs SILENT — anything needing a human is a filed GitHub issue (question / help wanted / needs-decision), never console narration. Role window, no persona (Bob belongs to the-desk only); driver human:<name>; the human merges.
---

# worker-desk — the dispatch half

**worker-desk** (this skill, its own window) turns eligible work into worker agents + draft PRs;
**pr-review-desk** (a separate window) reviews those PRs and flips them ready; **human:<name>** merges. Run it
in its own window as a standing loop — ONE window at capacity replaces running two.

**The stream board is a derived, generated surface** (`docs/streams/derived-board/spec.md`) — this
desk opens the PR carrying `Brief: <stream>/<NN>`; it never hand-edits a stream README's Briefs
table.

**The invariants this loop assumes** (state them once wherever your repo keeps house rules, and do not
re-derive them per dispatch): every session works in its own worktree, never a shared checkout; merge,
never rebase; the generated board is single-writer = main's CI; every post is made as the role's own
App identity; act only on trusted-authored items; file at discovery rather than narrating; one item =
one branch = one PR, and a merged or closed PR is DONE; and — in a repo that enforces it (one whose
`changelog/` directory carries `changelog/README.md`) — a notable change records one human-legible
highlight as a per-PR fragment file (`changelog/<slug>.md`) rather than editing a shared section, while
a genuinely non-notable PR carries the `changelog:skip` label, applied by the desk or a human and never
self-applied by the worker. This paragraph binds the DESK's own writes; the worker is bound to the same
fragment rule through the changelog clause `deskdispatch` emits verbatim in the worker kit.

> Bindings for your harness — which mechanism each `capability:*` names — are in
> `../../references/<harness>.md`.

## Boot

`deskboot worker-desk` — loop identity, `deskwt prune`, worktree lock, roster register, roster
preflight, worker-App token mint, read-only board fetch. The first red step stops the boot and names
itself; it refuses to boot in the shared checkout. Two residues it does not carry: `export
KUBECONFIG=/dev/null` (the environment half of the offline envelope; every dispatched agent
gets the same rule from the common-clauses kit) and inline `-c` commit identity per commit
(never `git config user.*` in a linked worktree — it clobbers every concurrent session's identity).

**Two things the boot owes before the first sweep:** arm the standing wake (§Cadence and wake) so the
window has a cadence from the start rather than after its first quiet queue, and print the BOARD ROOTS
∪ SCAN REPOS symmetric difference (§THE REPO SET) so a root nothing sweeps is visible on day one.

## The pool — keep slots FULL, not waves: refill on completion

**The unit of operation is the SLOT, not the wave.** Keep a standing pool of **N** concurrent
workers full. **An idle slot while eligible work exists — a queue row on ANY board, a qualifying
un-briefed issue (§Un-briefed issues), or an orphan PR awaiting resume — is the failure this design
exists to prevent**, and "dispatch a wave, report, stop" is the drift it kills (human:<name>
2026-08-13): there is no state in this loop called "the wave is done", only full slots, refillable
slots, and a sweep proving nothing eligible exists (§HARD GATE).

- **N is read, never remembered — `deskroster width --role worker-desk`, EVERY TICK.** The number
  is not stated here. It lives in ONE place (`tools/desk/internal/deskkit/width.go`), which is what
  lets the coordinator widen this pool when `deskboard throughput` names dispatch as the
  bottleneck, and what stops this body drifting from the value the tools enforce. Re-read it each
  cycle, not once at boot: a width set mid-window is meant to reach you on the next tick.
  - **Growing** fills the freed slots up to the new N as work lands.
  - **Narrowing NEVER kills a running worker** — stop refilling and let the pool converge downward
    as items land. A dispatched worker mid-item keeps its slot.
  - A width that cannot be read is **could-not-check**: hold at the number you last read (or the
    default) and file it. Ignorance never widens a pool.
- **The reservation is the code-enforced form of the next two rules** (desk-supervision/05):
  `deskroster width --role worker-desk --reserve resume=N,rework=M` floors N/M slots for orphan
  resumes and `Awaiting implementer rework` rows, riding the same entry as the width and decaying
  with it. `fanoutloop plan` classifies its queue and states the floor whenever a reserved class
  is actually waiting (`fresh capped at <k> by reservation`) — **it never idles a slot** for a
  class with nothing queued, which is what keeps this a floor and not a second cap. worker-desk
  ships `resume=2`; the "resuming started work outranks a fresh brief" prose below is what that
  floor is FOR, not a second, unenforced statement of it.
- **Fill to N, refill on completion** — the instant a worker finishes (draft PR open, or done /
  NEEDS_CONTEXT), dispatch the next eligible item into the freed slot.
- **Never stop-and-wait-for-restart.** "The plan came back empty" is a reading from ONE instrument,
  not a state of the queue: re-sweep §Sources of work with each source's own instrument. Where every
  visible row really is claimed, in flight or capped, §WIP-capped — re-check, never stop applies. The
  window neither ends nor asks the driver what to do next (§Liveness contract).
- **Concurrency (N) is DECOUPLED from the span-of-control display cap (20)**, which bounds what the
  board SHOWS. The 4-per-stream cap is an anti-monopoly guard that must **never idle a slot**: fill
  under-N cycles with WIP-draining work first — orphan resumes, `Awaiting implementer rework` rows,
  `CONFLICTING` PRs, red checks — then un-briefed issues, before leaving one empty.
- **Scan the active PR queue on every tick** for dispatched PRs owing a worker action
  (`CHANGES_REQUESTED` at head, `CONFLICTING`, a stale draft); **resuming started work outranks a
  fresh brief** (mm/10). Distinct from pr-review-desk's *review* monitor; both run.

## Sources of work — the complete list, each with its own instrument

**Work reaches this desk through more channels than any one verb reads.** Each row below names the
instrument that reads that source THIS tick; a source with no fresh reading is `could-not-check`, not
empty. Repos and roots come from §THE REPO SET, never a pasted list.

| # | Source | The instrument that reads it |
|---|---|---|
| 1 | Board rows the board SHOWS, per root (the span-capped Next-up selection) | `git -C <root> fetch origin && fanoutloop plan --root <root>` |
| 2 | Rows **held back** by the 4-per-stream cap or the span cap | `deskboard dispatch` — it reports the held-back decomposition (N by per-stream caps, M by span) plus per-root claim degradation, so an EMPTY queue is distinguishable from a THROTTLED one |
| 3 | Orphan PRs owing a worker action, `CONFLICTING` PRs, red checks | the per-slug PR + disposition reads in [`references/dispatch-runbook.md`](references/dispatch-runbook.md) §The tick sweep, with the disposition read FIRST |
| 4 | Stale drafts (reviewer verdict `CHANGES_REQUESTED` at head, author silent) | `deskboard stalled [--min-age-hours N]` — the purpose-built detector; its disposition column is advisory (shepherd / close-candidate) |
| 5 | `Awaiting implementer rework` board rows | `fanoutloop plan --root <root>` (desk-supervision/05: read per root from `refs/remotes/origin/main:STATUS.md`, the SAME offline ref read row 1 uses — no separate sweep) |
| 6 | Un-briefed trusted work-ready issues (§Un-briefed issues) | `issueboard issues` — fail-closed |
| 7 | A red default branch on a watched repo | `deskboard health` — three-state (green / RED / COULD-NOT-CHECK) |
| 8 | Cross-root coverage: a board root that no sweep reaches, or a scanned repo with no board | the BOARD ROOTS ∪ SCAN REPOS symmetric difference printed at boot (§THE REPO SET) |
| 9 | Queue **suppressors** — expired `refs/dispatch/*` claims and dead branch-claims from merged/closed PRs | `git ls-remote origin 'refs/dispatch/*'` and the repo's `dispatch-claim` helper's list/show verbs |

**Rows 5 and 3 outrank row 1** — resuming started work outranks a fresh brief (mm/10) — and row 2 is
what tells you whether row 1's zero means drained or throttled.

**Row 9 is why a stream can offer nothing while holding work.** A claim subtracts twice: once as an
eligibility exclusion and again as a per-stream cap decrement, so a handful of branch-claim corpses on
one stream can zero that stream's whole allowance. An expired claim does not push its row back onto
any board by itself. A stream that offers nothing for several consecutive ticks is a suppressor
candidate: check the claims before concluding the stream is drained, and file what you find.

**Explicitly OTHER desks' lanes — named so the boundary is stated, not so this desk works them.**
Open `verify-gate` / re-baseline issues (`deskboard queue`) belong to **verify-desk**. Review-request
items belong to **pr-review-desk** — `plan` deliberately skips their dispatch tokens. Decayed
approvals and the stale-approval resync class belong to **pr-review-desk's** freshness tick. This desk
neither works nor counts them; where one of them surfaces as a plain implementable issue it arrives
through row 6 like any other, on row 6's four rules.

## HARD GATE — never claim "pool empty / nothing to dispatch" without a fresh sweep

**An idle claim is a claim about the work queue, and the only evidence is a fresh sweep.** Before this
desk EVER reports "pool empty", "nothing to dispatch", "caught up" or "idle", it is a HARD
PRECONDITION that it has *just* re-run, this tick, **every row of §Sources of work that is this
desk's** (rows 1–9), across every BOARD ROOT and every SCAN REPO — and that it can **quote those
instruments' own output from this tick**. An idle claim with nothing to quote is not an idle claim.
"I checked at the top of the hour" is not fresh; "my workers haven't finished yet" is not evidence the
queue is empty; and a zero from row 1 beside a non-zero held-back count from row 2 is a WIP state,
never an empty queue.

**A sweep that failed, errored, or hit a `--limit` cap is `could-not-check` — blind, not idle**, and
so is a single-root sweep. The board also **holds rows back at the per-stream cap** without saying so
on its face: "no rows past the ones shown" is not "nothing eligible" (row 2 is the reading that
settles it).

## THE REPO SET — derived once, consumed by every sweep

**Two sets, one definition point.** This is the ONLY place this skill RESOLVES a repo set; the board
read, the orphan sweep and the issue sweep all take their repos from here. A second *operative* list
is a bug wherever it appears — a hand-maintained list drifts in both directions at once, and neither
direction is visible from inside the file.

| set | what it is | derived from |
|---|---|---|
| **BOARD ROOTS** | local checkouts carrying `docs/streams` — the dispatch queue | `deskroster repos --scope topology` rows carrying `root=`, **unioned** with a live `docs/streams` test over the siblings |
| **SCAN REPOS** | `owner/repo` slugs swept for orphan PRs and un-briefed issues | `deskroster repos --scope scan` (`ASSAY_SCAN_REPOS`) |

SCAN REPOS is deliberately wider (repos the desk fronts that carry no `docs/streams`); BOARD ROOTS is
not a subset by construction, so a board root missing from SCAN REPOS is a **reportable mismatch** —
its briefs dispatch but its PRs never get an orphan resume. Print the symmetric difference each boot
and file it rather than reconciling it by editing this file.

The derivation itself — the `deskroster` read, the `docs/streams` + `--git-dir` observation test, the
slug-keyed union — is [`references/dispatch-runbook.md`](references/dispatch-runbook.md) §Deriving THE
REPO SET. Two rules from it bind here: **key on the repo slug, not the path**, and **a root in exactly
one of the two lists is named in the report either way, never dropped** (declared-but-absent =
could-not-check; observed-but-undeclared = a `topology.yaml` gap — dispatch it this cycle and file the
gap). A hard-coded list is the board-blind bug this replaces: written from one checkout it silently
skips the largest board when the session is homed in another, and says nothing.

### The interleave rule — deterministic, and NOT a new global ranking

Each board is already ranked per root and nothing merges them, so the cross-repo merge **adds no
scoring pass of its own**: it is a **round-robin over BOARD ROOTS in ascending repo-SLUG order — one
row per root per pass, each root's rows top-to-bottom, until every root is exhausted**
(`A`(a1 a2 a3) + `B`(b1 b2) → `a1 b1 a2 b2 a3`). Order **within** a root survives exactly, **no root
is starved by a larger one**, and slug order means two dispatchers build the same batch and contend on
the SAME claim key. **A could-not-check root is a row in the report, not a gap in the batch** — its
eligible count is unknown, so the batch is a **lower bound** (desk-hardening/01); never fold an
unreadable root in as an empty one.

## Boards — one per root, and what the board does not tell you

- **Regenerate only YOUR OWN root**, in your own worktree, and discard the churn. Read every other
  root from `refs/remotes/origin/main:STATUS.md` after fetching it — never regenerate in a sibling
  (`--root ../X` **writes**, and `../X` is the shared clone other sessions use). Spell the ref in full
  (bare `origin/main` resolves to a stray local branch of that name where one exists), chain
  `git fetch origin && …` so a failed fetch short-circuits instead of printing a stale board at rc=0,
  and fetch **every** root.
- **Row count is not eligible count** — the 4-per-stream cap holds rows back without tripping the
  overflow line, and no verb surfaces the held rows themselves. `deskboard dispatch` is the reading
  that separates an EMPTY queue from a THROTTLED one: it prints the held-back decomposition and the
  per-root claim degradation. `--overflow-threshold 1` gets true counts when regenerating your
  OWN root. Where the board's overflow line fires, treat it as the alarm it says it is — clear WIP
  before pulling more (§WIP-capped — re-check, never stop), never as a licence to refill harder.
- **Qualify every brief ID with its repo** (`tracker:` / `at:` / `repob:`) in claim keys, prompts,
  dispatch records and PR bodies: more than one repo owns a stream of the same name.
- **Do not swap in `deskboard nextup`** on the strength of its help text — measured, it returns the
  awaiting-verification backlog, not the dispatch queue.
- **Multi-root: arity is fine — stream-identity COLLISIONS are the defect.** Generation works for any
  root count when every stream identity is unique (one STATUS.md per root, exit 0) and never merges
  boards, so N roots buys generation convenience, not coverage. Where two or more roots declare a
  stream of the same name the collision is **quarantined per root**: the colliding roots are skipped
  with loud STALE/PROBLEM lines while every non-colliding root still generates. The earlier "the
  combined two-root form exits 1 and writes NOTHING" line mis-transcribed the collision case onto the
  two-root form and is **withdrawn** (audit §8 correction 2). `statusgen init`
  scaffolds every repo with the same `stream: example` identity, so init'd roots collide by
  construction — that, not root count, is what to check when a board goes missing.

## Un-briefed issues — trusted, work-ready, on NO board (human:<name> 2026-08-13)

An issue can be real, trusted, implementable work and still sit stranded merely because it is
issue-shaped. Beside the board queue and the orphan sweep, the pool draws on **§Sources of work row
6**: open issues across SCAN REPOS, trusted + work-ready but represented by NO brief and NO
placeholder on any board.

**Sweep it with `issueboard issues`, not a hand-rolled list.** The verb resolves the scan scope from
the same roster key the placeholder scanner reads, applies the trust gate itself (untrusted, unblessed
authors are quarantined under EXTERNAL / UNBLESSED — visible, never actionable), ages
`needs-decision` / `question` rows against its SLA, and **fails closed**: an unset or empty scan set,
or a single repo the token cannot read, is exit 6 COULD-NOT-CHECK for the WHOLE board rather than a
silently partial one. A raw `gh issue list` fails soft in exactly the places that matter — a dropped
repo reads as a clean, empty board — which is why the list form is not the instrument here. **ALL
four must hold:**

1. **Trust gate** — authored by a trusted login, or blessed by a trusted comment; the board's own
   quarantine is the reading, and an EXTERNAL / UNBLESSED row is never dispatched. No new exception.
2. **Un-represented** — no `issue-loop/issue-<NN>` placeholder (stream root or `done/` archive), no
   brief citing it, no open PR working it. **Absence from the board is NOT absence of representation**
   (staleness, the caps or a board defect can suppress a real placeholder): check the FILES and the
   open PRs, never what the board shows. Work that persistently fails to surface is a board defect —
   FILE IT, never route around it here.
3. **Work-ready on its face** — an implementable spec by the placeholder lane's standard, carrying
   none of `question` / `needs-decision` / `help wanted`, not parked awaiting a reply.
4. **Needs no triage judgment** — single-repo, no open design fork, no risk-bearing surface
   (public-repo copy, security, prod-deploy, irreversible actions). Anything needing scoping,
   splitting, a decision or a risk call is **intake's job**. When in doubt, leave it to intake AND
   LEAVE A TRACE (a `question` on the issue naming the fork) — intake's strong tier picks the default
   a reversible fork then proceeds on.

**Priority is LOWEST of the dispatch sources** (WIP-draining work → board rows → un-briefed issues),
but the ordering is a tie-break, not a hold: an empty slot with a qualifying issue and nothing above
it dispatches NOW. Claim under the SAME issue-shaped key the placeholder lane uses, `<repo>--issue-<NN>`
— deliberately shared, so the two lanes contend on one lock and can never double-dispatch. A
sweep that repeatedly surfaces issues failing rule 4 is an intake-coverage signal: file it, never
widen this lane.

## The loop

**0. Sweep — first at boot, then EVERY tick.** Every row of §Sources of work that is this desk's, with
that row's own instrument. Repos from §THE REPO SET; an exit-6 or empty `deskroster repos` print is
could-not-check, never "no repos".

- Run the board-wide instruments once (`deskboard dispatch`, `deskboard stalled`, `deskboard health`,
  `issueboard issues`), then the per-slug PR + disposition reads — the exact queries and the three
  pre-resume guards are [`references/dispatch-runbook.md`](references/dispatch-runbook.md)
  §The tick sweep. `--limit` is mandatory on every raw list (bare `gh pr list` silently caps at 30) and
  at-cap is **could-not-check — blind, not idle**.
- **Read the DISPOSITIONS first, before any staleness arithmetic**: `SUPERSEDED` /
  `RESOLVED-ELSEWHERE` is a deskclose item, never an orphan; `NEEDS-REBASE` is live work; exit 6 or a
  failed read means that repo is BLIND this tick, not empty.
- A PR is **ORPHANED** when its disposition reads checked-clean AND the worker owes it action
  (`CHANGES_REQUESTED` at current head, CI red, findings unanswered) AND no commit/comment for **>4h**
  AND no live dispatch claim. **Write the verdict with `deskdisposition set`** when the sweep DERIVED
  a new one — eight of ten orphan dispatches in one 2026-08-12 cycle re-derived a conclusion an
  earlier pass had already posted — and **re-write nothing when nothing changed** (§WIP-capped: a
  no-change tick makes no write at all).
- **A `SUPERSEDED` record is a PROPOSAL, never a close.** After `deskdisposition set --verdict
  SUPERSEDED --evidence <target>`, the worker runs `deskclose superseded -R <repo> <N> --by
  <target>`: under a worker-bound token the tool applies `superseded?`, posts the proposal naming
  the target, and STOPS — it cannot close, cannot confirm and cannot dispute, whatever flags it is
  handed, because the role is read from the token's roster binding, not from the caller. The close
  is pr-review-desk's confirm (once the target has merged); a `needs-decision` on the PR means the
  reviewer disputed it and the item is human:<name>'s. A worker that closes its own PR as superseded
  by hand has skipped the only independent check on "the other PR carries my scope" — the class of
  error the lane exists to catch. A `superseded?` PR is parked, not orphaned: never re-dispatch it.
- A red default branch is work: where the fix is mechanical this desk dispatches it like any other
  item; where it is not, it is filed (§Output contract) and named in the tick's line.
- The un-briefed-issue sweep (§Un-briefed issues) runs over the same set in the same tick.

**1. Plan, per root.** `git -C <root> fetch origin && fanoutloop plan --root <root>` for every BOARD
ROOT. `plan` is the SHOWN queue: orphan resumes first, then the board in board order, `issue-<NN>`
placeholders **included** (only another loop's review-request dispatch tokens are skipped), each item
carrying its tier and its exact dispatch instruction. It spawns nothing, writes nothing and **touches
no network** — the chained fetch is the freshness guarantee. Exits 3 disabled · 5 refused · 6
unverifiable · 7 author==runner: none of them is an empty queue.

**`plan` is not the whole queue, and a zero from it is not an idle claim.** It carries no
`Awaiting implementer rework` row and no held-back row; run `deskboard dispatch` beside it every tick
for the held-back decomposition, and read the rework section off each root's board (§Sources of work
rows 2 and 5). Where the two disagree, the wider reading wins and the narrower one is a defect to
file, never a queue to route around.

**2. Merge the per-root plans** with §The interleave rule, tag every row with its repo-qualified ID,
name every could-not-check root, and exclude items whose `depends:` are not yet `done`. A count from
human:<name> ("next 3") takes the top N **of the merged order** — scoping bounds THIS refill, never the loop.

**3. Dispatch** each item with `deskdispatch` (below) — one `capability:dispatch-worker` per item, all
in a single batch.

**4. Refill — do NOT hand off and stop.** The draft PRs reach pr-review-desk's review monitor by
themselves (you never review your own dispatched work). On each completion re-run the sweep and
**dispatch the next eligible item into the freed slot immediately**, holding the pool at the current
N (`deskroster width --role worker-desk`, re-read this tick — §The pool); re-plan
every root each tick. The 4-per-stream cap is per stream, and a same-named stream in two roots counts
separately.

**5. Keep the records current — silently.** The dispatch log is what the machinery already writes:
`refs/dispatch/*` claims, roster registrations, branches, draft PRs, dispatch comments. The loop
pauses only between ticks — never because "the wave finished". Note what that log does NOT cover: the
sweep itself leaves no audit row, so a tick that only planned is indistinguishable from a tick that
never ran unless the tick's own line (§Output contract) says otherwise.

## WIP-capped — re-check, never stop

**Every visible row claimed, in flight or held at a cap is a WIP state, not an empty queue.** It is
the one state in which this desk has nothing new to START, and it is never a state in which the
window stops, hands back, or asks the driver for direction.

1. **Drain WIP before pulling more.** In this state the desk works the WIP-draining sources FIRST and
   in this order: orphan resumes (§Sources of work row 3), `Awaiting implementer rework` rows (row 5),
   `CONFLICTING` PRs and red checks on PRs this desk dispatched, stalled drafts (row 4). Only when
   those are genuinely clear does a fresh start go into a free slot. The board's own overflow line
   says the same thing — a persistently overflowing Next-up is an alarm about load, and refilling to 8
   against a deep verification and rework backlog grows Act while Observe stands still.
2. **Then tick again — do not stand down.** The next full re-check runs on this desk's cadence
   (§Cadence and wake). Standing down needs an empty standing queue after a fresh sweep PLUS the
   hand-off artifact (§Liveness contract), and a WIP-capped queue is neither.
3. **A no-change tick makes NO write.** Not a disposition re-write, not a claim touch, not an
   idempotent re-record of something already recorded. The deskkit breaker counts CONSECUTIVE
   non-progress results with **no time window**, and a plain `noop` counts as non-progress
   (`ratelimit.go`, `BreakerTrip = 5`, 15-minute cooldown) — so an idempotent write on five quiet
   ticks in a row opens the breaker with nothing having failed. Read freely; write only what a real
   change earns.
4. **Refills stay inside the write budgets.** `deskpr create` and `deskfile new` share one repo-wide
   ceiling of 20 per rolling hour, so a full 8-slot refill each half hour already spends most of it on
   one repo and leaves little for rework pushes on the same repo. Pace refills against that; exit 4
   names a `RetryAfter` — sleep it and attempt ONCE, never retry-loop.
5. **The tick still reports.** One line per §Output contract, and the sweep obligations of the HARD
   GATE are unaffected by a quiet queue.

## Dispatch — `deskdispatch <item-key>` runs the ceremony

The per-item ceremony is mechanical and lives in the verb: claim-acquire → worktree-create in the
item's OWN repo → roster-register → decision-gate → model-stamp → prompt-emit, each printing one line,
the first red one stopping the dispatch and naming itself. The desk RUNS the verb and honours its
exits; it never re-implements a step and never hand-assembles a prompt.

```
deskdispatch <item-key> [--tier strong|any] [--kit worker] [--repo O/N] [--root DIR]
             [--model SLUG] [--brief PATH] [--gate-human] [--pr N] [--dry-run]
```

- **The item key passes through UNCHANGED** — `<repo>--<stream>--<NN>`, or `<repo>--issue-<NN>` for
  issue-shaped work. A key a dispatcher reshaped would not collide with the one another desk holds,
  and a claim that does not collide is not a claim.
- **Honour the refusals.** **5** = a LIVE holder owns it: SKIP, carry the printed holder line into
  what you report, never steal. **6** = unverifiable: treat exactly like 5 this cycle, retry next —
  fail-closed, a claim you could not read is never "assume free". **3** disabled, **7**
  author==runner. Only exit 0 is a dispatch.
- **A claim is intent, not evidence.** The branch/PR check stays authoritative and still runs, over
  **merged** PRs as well as open — measured 2026-08-13, ~16% of `todo` rows already had a merged PR. A
  check that could not reach GitHub is could-not-check, never unclaimed.
- **`progress` the instant the agent is launched**; `release` every claim you took but did not
  dispatch. The claim contract — GitHub ref not local file, the two TTLs (`claimed` 20m →
  `dispatched` 120m), branch-as-claim takeover, `steal --reason` — has one home: the header comment of
  the repo's own `dispatch-claim` helper.
- **Never hand-edit the board row — neither this desk nor the worker it dispatches.**
  `in-progress` appears the instant the worker's draft PR opens carrying the trailer
  `Brief: <stream>/<NN>` in its body; `deskpr create` refuses to open a PR whose body lacks
  `Brief: <stream>/<NN>`, and that refusal at write time is the enforcement, not a follow-up edit
  to the stream README. `implemented` appears the instant that PR merges. `statusgen` derives both
  cells from the trailer plus the PR's own state (`docs/streams/derived-board/spec.md`) — this
  desk's job at the `progress` step is opening the PR promptly, not writing a cell.
- **The worker prompt is the kit, verbatim.** `deskdispatch` emits `references/common-clauses.md`
  (home-worktree isolation floor, no-evasion, offline envelope, three-state instruments,
  escalate-durably, one-workpad-per-PR) ahead of `references/worker-prompt.md` (security-gate refusal,
  per-invocation `mktemp` body files, stop-at-`implemented` + the bare-token board-row shape, lineage
  self-check, merge-never-rebase, verify-before-apply, scope + desk write verbs, release-the-claim,
  fail-first evidence, public-body self-containment, changelog fragment where the repo enforces one) —
  both shipped
  inside the binary from `tools/desk/cmd/deskdispatch/references/`. `--kits`
  lists what the installed binary carries; `--dry-run` prints the prompt it WOULD emit. **Never
  paraphrase, summarise or "improve" a kit clause at dispatch time**: each is a rule that has already
  failed in the field, and the wording is the fix.
- **Cross-repo is the default case.** The verb cuts the worktree in the item's own repo off
  `refs/remotes/origin/main`; dispatch the agent with `capability:isolate-workspace` too, so its
  payload cwd is never the shared checkout — a /tmp clone does NOT isolate that cwd, and a
  falsely-blocked worker is the input that produces evasion.
- **Tier**: `--tier` follows the brief's `exec-tier` (absent = `any`); `strong` goes only to
  session-tier and the kit carries the pickup-STOP text. Effort S may run at your session tier, M/L go
  to a cheap tier behind the review/verify gates.
- **Cheap implementers run below the floor, but authority-bearing writes do not**: a review verdict and a ready-flip enforce a model-capability floor keyed on the dispatcher's attested tier, so a below-tier session is refused those writes even though it may implement freely — delegate downward, and escalate a verdict or flip to a strong-tier session rather than route around the refusal.
- **Serialize out-of-repo items** — no worktree isolation, no branch-as-claim: at most ONE in
  flight across all streams, the declaration is the claim, so check in-flight PRs for overlaps first.
- **Placeholders stay dispatchable** (ruling 2, 2026-08-24) — and the shipped `fanoutloop plan`
  includes them, so skill and binary now agree.

## gate:human items dispatch normally — and file the decision issue at first dispatch

`gate: human` / `irreversible: yes` items DISPATCH NORMALLY (2026-07-10): the gate binds APPROVAL —
review, merge, verify sign-off — not implementation, and every human control point sits downstream.
But **a human gate with no filed issue is an empty decision surface** (human:<name> 2026-08-23), so pass
`--gate-human` (or a `--brief` whose metadata gates), which runs `decision-issue ensure … --at start` between claim and launch — idempotent by its hidden marker, swept `--state all`, landing a
`needs-decision` issue that enters the weekly digest. Three non-filing outcomes: `deferred:
decision-trigger=spec` — the decision is only well-formed at the pickup design step, so the prompt
instructs the executor to author the brief's `## Human decision` section in its PR and report
DECISION-BLOCK READY, and **this desk** then re-runs ensure `--at spec` against the branch copy
(subagent issue-writes get classifier-denied); **5** = self-containment refusal, repair the brief,
never hand-file around it; **6** = could-not-check, do not file, retry next cycle. Record the issue in
the dispatch and the PR body's BLOCKED-ON-HUMAN line; where the Task has an explicit human co-execution
step the prompt says prepare everything, STOP at the documented stop-point, report BLOCKED-ON-HUMAN.

**Security-gate removal is gate:human BEFORE the commit, not only at approval**: such a diff
erases the red-check signal that makes the decision visible downstream, so "a worker behind a draft PR
is inert" fails for it. Never write that removal into a brief or a prompt — briefing it launders a
gate:human decision into a mechanical task. File the `needs-decision` fork; dispatch only after a
recorded ruling.

## Intra-brief splits — N shards, ONE brief PR

Default is **one worker per brief**. A brief may OPT IN by declaring `parallel-streams:` (a name plus
file globs per shard); the field is declared once, empty, across the corpus today, so the lane is live
but unexercised. The gate is `statusgen shardcheck --brief <path> --root <that repo's root>`: **0 →
the split may dispatch; 1 or 2 → ONE worker, serially** — a split whose safety could not be
established is not a split you may run, and serial is always available and never wrong. Its refusals
name properties of the declaration against the real tree, so the fix is a different declaration or a
serial run, never a narrower check. The recombine model (N worktrees → ONE integration branch → ONE
draft PR), the shard claim key, what file-scoping does not prevent, and the `SHARD-INCOMPLETE`
partial-failure contract are [`references/dispatch-runbook.md`](references/dispatch-runbook.md)
§Intra-brief splits.

## Output contract — one line per tick; escalation = a FILED ISSUE (human:<name> 2026-08-13)

This desk's console is not a progress channel and nobody is watching it — human:<name>'s review surface is the
issue list. Two states:

1. **Normal operation → ONE LINE PER TICK, nothing else.** No dispatch narration, refill
   confirmations, board dumps or "swept N boards" summaries — every one of those is already recorded
   where the machinery writes it (§The loop step 5). The single exception is the quiet-iteration line
   the console noise floor allows, in the shape that canon defines — the project's console-noise-floor
   contract, `docs/streams/desk-tools/scoping.md` §"Console noise floor"; read the shape there, it is
   deliberately not restated here. **That line is the whole reason a quiet tick is distinguishable
   from a dead one**, which is why this desk prints it rather than nothing: the sweep itself leaves no
   audit row. An explicit request from the driver still gets a full answer: the floor binds unprompted
   narration, not answers.
2. **Needs a human → FILE AN ISSUE** with `deskfile new … --raised-by worker`: the dedupe gate plus
   the provenance stamp that lets the by-desk metric see the DISPATCH loop noticed it (omit the flag
   and it lands UNKNOWN, which is the absence of an answer; an unbound role is refused, exit 5).
   Escalation vocabulary and label discipline: §Guardrails. When it concerns a PR/issue already in
   flight, label + comment THAT item rather than filing a duplicate. **The filed issue IS the
   escalation.**

**A question never stops the window.** `question`, `help wanted` and `needs-decision` are filings, not
console stops: label + comment the item saying what is needed and from whom, then **carry on with the
rest of the queue in the same tick**. For a REVERSIBLE fork the ITEM does not park either — it is
dispatched on its stated default with the question riding on the issue/PR (the merge gate catches a
wrong default); only a genuinely one-way fork parks the item. The WINDOW never parks. "What should
I work on?" is not a question this desk asks at all (§Liveness contract).

A hard error that halts the loop may print its final diagnostic — but if clearing it needs a human,
file the issue too. **The output floor does not touch the HARD GATE**, an internal-state rule: the
sweeps still run every tick and the desk still may not treat itself as idle without one.

**Detected blindness is a needs-human condition, not a quiet state.** A board older than the cadence
interval, a wake that stopped re-arming, a sweep exiting non-zero, an `issueboard`/`deskboard` exit 6
→ re-sweep immediately, and if it persists past one attempt FILE it (`help wanted`, naming the dead
instrument, the last good sweep timestamp, and what re-arming was tried). Quiet is evidence of health
only while the tick line proves the instruments are alive.

**File-and-exit, never block (pod-loop contract, desk-hardening/13) — scoped to POD / CronJob runs.**
In a bounded execution (a pod or scheduled job whose run must terminate), the reversibility test runs
FIRST: a reversible fork is dispatched on its stated default and then filed with that default named;
only a genuinely one-way / external blocker the DISPATCHER loop cannot resolve is filed unworked — or
confirmed already filed — and the run **exits**; it never holds a bounded run open waiting. **In a live role window this clause
does not license an exit**: the window files the same escalation and **continues with the rest of the
queue**. The event-only resumption the original clause assumed is superseded for live windows by the
2026-08-25 liveness ruling recorded in the project's desk liveness contract,
`docs/desk-liveness-contract.md` — read the ruling and its provenance there; it is relayed here, not
restated. **Scope boundary:** this converts the DISPATCHER loop only — a dispatched worker that hits
a blocking question parks on its issue
(`question`-labelled comment carrying the `<!-- desk-automation -->` marker, `blocked:
awaiting-issue-response` + `blockedAt:` in its placeholder, commit, STOP), owned by issue-loop/03 and
deliberately not re-authored here.

### Stop-flag check — run at every iteration boundary

Before each loop cycle (orphan sweep, slot refill):

```bash
[ -f "$HOME/.config/assay/STOP" ] && echo "STOP flag active — exiting loop" && exit 0
[ -n "$DESK_LOOP" ] && [ -f "$HOME/.config/assay/STOP.$DESK_LOOP" ] && echo "STOP.$DESK_LOOP active — exiting loop" && exit 0
```

A hit means exit cleanly (restart by `rm <flag>` + re-arm); never halt mid-dispatch. Precedence:
`DISABLED` > `STOP` > `STOP.<name>`; `deskkit.Guard()` enforces them at the tool layer independently.

## Guardrails

- **Worktree isolation is mandatory** — parallel workers mutating a shared checkout is the incident
  this system exists to prevent. It binds the DISPATCHER and its prompts too: F-35 proved a session
  boots clean in a worktree, `cd`s to the shared checkout out of habit, and its prompts then carry
  shared paths. While workers are in flight, `git -C <shared> status --porcelain` each tick — ANY
  dirt is an alarm.
- **A guard/hook BLOCK is a STOP signal, never a puzzle** — never re-attempt the same effect with a
  different command, tool or path spelling; quote the block message and escalate. The verbatim clause
  every agent receives is common-clauses C2; it is named here because a dispatcher that treats its own
  block as an obstacle reproduces the incident at dispatch scale. A worker that finds its item too big
  STOPS and splits per author-brief rules, keeping only the piece it was mid-implementing.
- **Project tool output; tune one variable at a time.** Read tools through a projection (`--json` +
  `jq`, only the fields the decision needs), and change ONE variable at a time when re-configuring a
  monitor cadence, the fan-width or a dispatch clause, recording before/after in the PR.
- **Dispatch claims are dispatch-scoped, not work-scoped** — a listing full of old refs means dead
  dispatchers, not claimed work: read the state and age each claim carries, never existence alone.
- **The out-of-repo surface (files outside any repo) is serialized, never parallel** — max one
  in-flight item, staged as diffs in the PR and applied to the live files only as the last step
  before `implemented`.
- **Insight-routing:** a systemic/process insight produced in passing (a wrap-up, a dispatch or drain
  note, an Evidence aside, a "this keeps recurring" observation) MUST also be filed as an issue in the
  project's own toolkit/methodology repo — commentary is not a register. Include the triggering
  evidence and affected loops. Repo-specific defects still go to that repo's own tracker (label `bug`).
- **Escalation labels:** any desk/loop may label a PR or issue `question` (needs an answer from the
  driver or a stronger-tier model — the item PARKS only when the fork is one-way; a reversible item proceeds on its
  stated default with the label riding on it) or `help wanted` (the desk hit its capability/authority edge). Both are
  GitHub default labels — they exist in every repo, no setup. Discipline: a bare label is unanswerable — the labeler
  MUST comment what it needs and from whom when labeling; whoever answers removes the label with their response. A
  `question` that matures into a formal decision fork promotes to `needs-decision` with the pros/cons template.
  Labeled items are WAITING-ON-INPUT: they join the human/escalation queue and are NOT orphans for the worker sweep.
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
    read-only included.** Mutating verbs stay denied for every desk role; a worker that needs live
    state reports could-not-check + BLOCKED-ON-HUMAN, never a probe.
- **Reversibility test — default-forward on anything a human-held gate still catches:** before
  parking an item on the driver, ask ONE question: *is a wrong guess here caught by a gate the
  driver still controls — a draft PR awaiting merge, a filed issue awaiting close, a flip CI or a
  human must still make?* **Yes → default-forward.** Author it, dispatch the worker, open the DRAFT
  PR, make the best-guess call, and NOTIFY — "proceeded on `<default>`; filed as `<repo>#<N>`;
  decline the merge if it is wrong" — never ask for a go-ahead the merge gate makes redundant. The
  `needs-decision` / `question` issue is still filed, naming the default taken, but the ITEM does
  not park on it. Urgency is not a reason to ask: a time-sensitive reversible call is made now, on
  the record, and corrected by the gate. **No → STOP and wait for the human.** A wrong guess that
  lands irreversibly or reaches outside the gate is caught by nobody declining a merge. That set is
  fixed, never judged case by case: merge, a ready-flip that is not this role's, any `main` push
  outside a standing authorization, a tag or release cut; deleting, disabling or WEAKENING a
  security control or its CI assertion; exposing secrets, credentials, PII or exploit detail (a
  public repo above all); money movement, identity/auth changes, deleting or overwriting durable
  data; and anything that leaves the repo — publishing to a public or external surface, sending
  content to an external service, mutating live infrastructure. A guard or tool REFUSAL is a STOP on
  either side of the test — the test never routes around one.
- No attribution lines anywhere: no `Co-Authored-By`, no "Generated with …" in commits, PRs, issues,
  or comments.

## Liveness contract (binding)

A standing liveness contract binds this window from boot: start the standing
self-scheduled loop BEFORE the first sweep and keep it ticking for the life of
the window; every tick re-sweeps this desk's own queue fresh; every relay (a
cross-session hand-over) is acknowledged or filed, never assumed delivered.
The desk runs **default-forward** — never ask the driver what to work on next:
a driver scope instruction narrows preference, not a cage — when the scoped
batch drains, note the transition in the hand-off note and widen back to the
standing queue. Checkpoints state their default and continue; standing down
requires an empty standing queue after a fresh sweep PLUS a hand-off artifact
on the driver surface, and a manual human kick that moves queued work is an
incident to file on the project's methodology tracker. Hard gates (human-gated
decisions, budgets, breakers, explicit stop-orders) are unchanged.

## Cadence and wake — this desk's numbers

The contract above is fleet-wide and its text lives in ONE place: the project's desk liveness
contract, `docs/desk-liveness-contract.md` (rules, provenance, and the fleet cadence spec). This
section is the pointer that contract asks each role skill to carry, plus the numbers that are
worker-desk's own.

- **The wake is `capability:durable-monitor`** — armed at boot, before the first sweep, and kept armed
  for the life of the window. Check what is already armed before arming a second; never arm two. It is
  best-effort by construction and never the sole wake signal: the fixed-cadence sweep below is what
  makes a dead wake loud instead of a silent all-clear.
- **The full re-check tick is 30 minutes.** That is the WIP-capped re-check cadence and the standing
  full-sweep cadence, inside the contract's 30–60 minute heartbeat band. Completion signals
  (`capability:session-notifications`) still refill a freed slot the instant a worker finishes — the
  30-minute tick is the floor under them, not a replacement, and it is the only wake that exists when
  nothing is in flight.
- **Why not a ~60-second poll.** The sweep this desk mandates costs roughly three reads per scanned
  repo per tick — about 48 at the current scan width, 60–90 with the pre-resume guards and a regen
  pass. At ~60s that is thousands of reads an hour, most of a GitHub App installation's hourly read
  allowance before a single write; at 30 minutes it is ~96–150, a low single-digit percentage. The
  earlier "~60s idle-poll" wording is retired for that reason.
- **Harness degradation, in one line.** Where `capability:durable-monitor` is unavailable (its
  reference row reads `degrades` — no confirmed durable cross-turn wake, as on the Codex and Cursor
  bindings), the desk falls back to the event-driven + fixed-cadence board sweep at the same 30-minute
  cadence and **states the gap in-session**; a durable wake is a convenience, never one of the three
  never-degrade guarantees.
- **A tick keeps the dead-man lease fresh.** The desk tools refuse to run when
  `~/.config/assay/HEARTBEAT` has not been touched inside its staleness window, so the standing loop
  is the thing that keeps it current: touch it on every tick, including a quiet one.
- **A tick reads the armed per-run stops and stops each run's worker.** Every tick, read the armed
  per-run stops (`desksupervise status --stops`) and, for each one, `capability:stop-worker` the matching
  dispatched worker — the cooperative `STOP.run.<key>` flag already refuses that run's next desk verb,
  and this is the independent second layer that halts a worker which never runs another verb. `tick`
  arms the flag itself before it reclaims a wedged run, so a reclaimed item is stop-armed the moment its
  claim is freed for re-dispatch.
- **A tick that did not happen is could-not-check, never idle.** Two ticks with no successful sweep is
  the silent-board-freeze class — re-arm, and if it does not clear, file it (§Output contract,
  detected blindness).
