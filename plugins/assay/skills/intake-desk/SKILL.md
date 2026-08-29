---
name: intake-desk
description: Run the intake-desk — the generic front door of the process desk and the first of four desks in the pipeline (intake-desk → worker-desk → pr-review-desk → verify-desk). Ingests ALL inbound — open GitHub issues, the intake register (docs/streams/intake entries), and any incoming request or idea — and converts each into one of five tracked exits: spec/brief · bug/issue · finding · needs-decision · rejected/watching. Scans issues into placeholders, triages raw intake entries through their four disposition exits, files human-decision issues, and closes out resolved issues. Use when starting or resuming the dedicated intake window, or when asked to "run the intake desk / work the front door / triage inbound / triage the front door / work the incoming / intake / run the issue loop / work the issue queue / watch the inbound queue". Role window, no persona (Bob belongs to the-desk only); driver human:<name>; the human decides and merges.
---

# Intake Desk (the generic front door)

> **Home, as of this port.** This file is the portable core of the intake-desk skill — the
> front door of the desk pipeline. A project adopting Assay pairs it with its own project-local
> configuration: the roster values that name its owned-repo scan scope, its role-to-App
> bindings, and its own escalation labels. Those pieces are project config, not part of this
> portable core.

The **intake-desk** is the generic front door of the process-desk pipeline — the first of the four
desks (`intake-desk → worker-desk → pr-review-desk → verify-desk`). Where pr-review-desk watches
work *leaving* the system (PRs → ready), this desk watches work *arriving* from **any** source:
GitHub issues filed by anyone, intake-register entries (raw ideas), and any incoming request. Its
job is to convert each inbound item into exactly one of **five tracked exits**:

| Exit | What it becomes | Route |
|------|-----------------|-------|
| **spec/brief** | A stream brief (via the author-brief flow) | `scoped → <stream>` |
| **bug/issue** | A GitHub issue (label `bug` when bug-shaped) | `scoped → issue #NN` |
| **finding** | An F-NN finding entry | `docs/streams/findings/` |
| **needs-decision** | A `needs-decision` issue (the single human-decision queue) | `decision-needed` |
| **rejected/watching** | An explicit rejection or watch entry | `rejected — <why>` / `watching` |

An item that lands with none of them, or with two, is a refusal — `scanloop` records exactly one
tracked exit per item and fails the pass otherwise.

- **worker-desk** dispatches workers against the Next-up batch, **including the issue-placeholders
  this desk emits** (`F-desk-emits-briefs`, human:<name> 2026-07-20 — self-dispatch, issue-loop/12, is
  superseded). This desk's issue-lane output is the placeholder; it does **not** fan out workers. The
  shared dispatch claim (a GitHub ref keyed `<repo>--issue-<NN>`, methodology/42, so it is visible to
  a desk on any machine) is worker-desk's, not this desk's.
- **pr-review-desk** **reviews this desk's PRs** and flips them ready. Every write this desk makes
  leaves as a draft PR (scan commits, tooling, brief authoring, close-out carriers). This desk never
  flips its own PRs ready and never merges.
- **verify-desk** drains `implemented → verified`. **human:<name>** decides the `needs-decision`
  issues and merges. Both are always the human's.

Run this in a **dedicated window** so its per-minute monitor churn does not fragment the coordinator
desk (`the-desk`). **Only this window runs the inbound monitor** — a second monitor double-files
placeholders and double-triages. Role window, no persona (Bob belongs to `the-desk` only); the
register/evidence discipline of `the-desk` applies (read it if not already booted).

## Model requirement — run this desk on a SMART model (human:<name>, 2026-07-16)

**This desk's core work is judgment, so it MUST run on a strong/smart tier — not an economy tier.**
Unlike a worker (which executes a spec someone else already scoped) or a mechanical scan, the
inbound loop *decides*: whether an inbound thing is work or an idea (the routing test), which of the
five exits an entry takes, what a `needs-decision` issue's Situation/Options actually are, and how
to scope an idea toward a brief. Those are the same class of design-tier calls the `author-brief`
model-tier gate protects — errors here compound downstream through every worker the placeholder or
brief spawns. `scanloop` is built around this split: it EMITS the judgment half for a model tier and
never computes it. This is why the desk exists as its own smart window rather than a cron: the front
door needs a mind, not a trigger.

- **If you are a cheap/economy-tier session** (haiku-class or equivalent): do **mechanical work
  only** — run `scanloop`, keep the board current, post already-formed placeholders — and **do not
  triage-classify, author decision issues, or scope ideas**. Say which model you are and ask for a
  strong-tier session to take the judgment work. Do not triage anyway "just to keep the queue moving."
- **A downgrade mid-session hard-gates the judgment work**, not the mechanical work. Probe vs
  assertion (R6, 2026-07-10): human:<name> ASKING what model you are is a probe — answer with the env
  model line verbatim and keep mechanical work moving; only their ASSERTION of a downgrade hard-gates
  judgment.
- **Trust gate:** inbound not authored by a configured trusted identity is ignored unless the
  blessing authority has commented on it. `scanloop` applies the gate BEFORE anything is queued and
  `issueboard` renders quarantined items under EXTERNAL / UNBLESSED — visible and counted, never
  routed. Never triage, placeholder, or route them. (Both identities are project configuration — a
  house declares its roster and trust primitive in its own config; see the adoption runbook for the
  generic shape.)

## Boot sequence

`deskboot intake-desk` — loop identity, `deskwt prune`, worktree lock, roster self-declare, the
five-check `deskroster preflight --role issue-loop`, the issue-loop token mint, and a read-only
board fetch. Each step prints one line; the first red one stops the boot and names itself. **A red
preflight is `could-not-run` for the WHOLE pass:** report the ONE summary line and STOP — do not
claim work, and do not file an issue about the desk's own envelope (each failing check names the
issue that owns it). Two intake-specific residues after boot:

- **The scan scope comes from the tool; this skill carries no repo list.** `deskroster repos
  --scope scan` prints the front door this desk owns — the same roster value the scanner and
  `issueboard` read, so one scan covers exactly what it prints. Widen coverage by editing the
  roster, never by editing prose (the hand-maintained pair drifted in BOTH
  directions — phantom coverage of repos the tools refuse, and a silent blind spot on one they
  cover). `deskroster` exiting 6 is COULD-NOT-CHECK, not an empty world.
- **Labels are per-repo and not GitHub defaults.** `needs-decision` and `raised-by:issue-loop` must
  exist on a repo before use there: `gh label create <label> --repo <slug>`.

**Every issue this desk files goes out as `deskfile new … --raised-by issue-loop`**
(methodology-metrics/29) — the `scoped → issue #NN` filings and the `needs-decision` ones alike.
`issue-loop` is this desk's role in the roster's role-bindings, and that roster is the declared
vocabulary: `deskfile` REFUSES (exit 5) a role the roster does not bind, and prints the bound set.
The stamp itself only degrades — omit the flag, or have the label missing, or fail to check, and the
issue is still filed UNSTAMPED with a NOTICE, provenance reading UNKNOWN. Unknown is the absence of
an answer, never "a human raised it".

### Worktree hygiene

Worktree sprawl is owned by `deskwt prune` — it runs at boot and under its own interval
supervisor; no loop carries an hourly prune tick and nobody hand-deletes worktrees (the
ENFILE incident, 2026-07-23: sprawl exhausted the system open-file table).

### Console noise floor (issue-flow/10 deskquiet)

Both modes report transitions, not standing state, so an untriaged issue that has sat for days is
silent after its first sighting — satisfying the class-1 "always print actionable" duty needs a
periodic FULL sweep of the intake board beside the quiet loop, which `--delta`/`--quiet` alone
cannot give. The intake-debt NOTICE line (issue-loop/07) is class 1 and is never compressed away.

## The board

`issueboard` — read-only, one ACTION per open issue plus the intake lane's untriaged entries,
age-flagged (`issueboard issues` / `issueboard intake` narrow to one lane). Its scan scope is the
roster's, resolved at runtime; an unset/empty scan-repo roster value or a repo the token cannot read
fails the WHOLE board with exit 6, so the board is never silently partial and an empty sweep is never
reported as a clean board. A `needs-decision` or `question` issue ages against `--sla-days` (default
6) from the last HUMAN response — under it AWAIT, past it ESCALATE, sorted to the top.

## The loop — issue lane

The **GitHub issue body IS the spec** (stream convention). A placeholder never duplicates it; it
points (`issue: <NN>`, `repo: <owner/name>`) and carries only the scheduling metadata Next-up needs.

```bash
scanloop plan --root <repo> --inbound <captured-poll>   # READ-ONLY: the inbound queue
scanloop run  --root <repo>                             # the drain
```

`plan` prints surface, item, lane, age and claim state for every inbound item, plus the monitor's
arming coverage, the trust gate's three-state tally, the coalesce decision, and the surfaces this
drain does not read. It spawns nothing and writes nothing, and deliberately does NOT poll — a poll
advances the per-repo baselines and would consume the events it reports, so pass the standing
window's captured poll with `--inbound`. `run` is the drain, and it owns everything mechanical:
**arming** the poller if it is not armed (arming and draining are the same act — the seeding pass
reports no inbound rather than replaying the backlog), the **trust gate** before anything is queued,
the **coalesce window** (an open scan PR younger than the window absorbs the batch; at or past it
the PR stays sealed at a stable head and a fresh one is cut; an unreadable age never coalesces),
**body regeneration** (title and body re-derived from the branch's own diff on EVERY push, never
carried over, never hand-edited), worktree isolation, and one scan per pass — a run derives the
delta for every issue in scope, so the pass's mechanical items share ONE scan dispatch against one
branch and one PR while each still leaves by its own tracked exit. Exit 0 ok · 3 disabled · 5 refused
· 6 unverifiable · 7 author==runner. **Blind never exits 0:** a degraded repo, a suppressed burst, an
unarmed poller or an untakeable trust read all make the pass unverifiable — read a 6 as blind, never
as idle, and confirm `plan`'s arming-coverage line before trusting a quiet pass. `--dry-run` prints
every lane step without running it; `--offline --inbound <file>` opens no network read at all.

**Standing-doctrine pointer.** A successor boarding architecture — the scan-transcription lane
(ruling R-7 in `docs/streams/issue-flow/rulings.md`; work stream `docs/streams/scan-lane/`) would
have an `issues`-event workflow re-derive and commit the placeholder delta itself for
trusted-author issues. It is a policy change (it removes the human merge that today skims
machine-raised work onto the board), so it takes a recorded operator ruling plus a deployed, armed
lane before it applies — **R-7 is still unsigned** (its Sign-off line is empty, checked 2026-08-25),
so the scan-carrier flow above stands until R-7 signs and the cutover lands (scan-lane/03);
`scanloop`'s dispatch lane is built swappable for it. Do not anticipate the cutover.

### The judgment half — what `scanloop` emits for you to decide

**OWNERSHIP — the ORIGIN test** (human:<name>, 2026-08-02: *"monitor-fired goes to the
intake-desk"*). The cut is not *who owns issues* — it is **who told you to do this, a monitor or a
human?**

- **Monitor-fired → THIS desk owns it, exclusively**: routing comments, decision-consumption and
  recording, relabeling, triage, duplicate-marking, small inline pipeline-unblock fixes. The
  coordinator (`the-desk`) does not fire on inbound GitHub **issue or comment** events. (Its
  PR-queue watch — filing `review-request` issues on new/updated heads — is a different surface and
  is unaffected; so is its own board-sweep autonomous-drive rule.)
- **Human-directed → the session that received the instruction owns it**, no claim needed — only one
  window got the instruction, so there is no race to arbitrate.

This partitions **autonomous** reactions only: a coordinator working an issue because a human pointed
at it is NOT a violation. **Exclusivity, not a claim, because coordination-by-announcement does not
work** — both desks react to the same event, so the announcement lands after the other has already
started (the incident that set this rule: one fix got two full worker implementations *despite* a
routing comment naming both dispatches 23 minutes earlier). **Decision: claims are deliberately NOT
extended to response actions — do not re-propose the ceremony.** human:<name>'s calibration: *"while
doing double work on some issues is annoying, we are catching them, and it isn't really affecting the
integrity of the overall system (besides burning tokens)"* — the cost is duplicated tokens, not
correctness, so it gets one cheap ownership rule. Extending a leaky mechanism to more surface buys
ceremony, not reliability. The same incident **predicted the next one** — it warned that the same
race on a **mutating** response "would not converge harmlessly", and a desk unilaterally closing
issues landed three days later. That is why the two rules sit together.

1. **CREATE-PLACEHOLDER triage.** `scanloop` writes the placeholder; your job at placeholder time is
   **triage only**. An issue that fails the routing test (thin / ambiguous) is NOT left for a worker
   — label it `question` with what is missing, or scope it; a worker-legible issue simply rides
   Next-up. **Do not fan out workers, do not take a dispatch claim, do not author implementation or
   close PRs from this window.**
2. **`close-candidate` — the brief-write for a no-merge close.** When an issue must close with NO
   merged fix (`FIXED-NOT-CLOSED | WONTFIX | DUPLICATE | STALE`), mark the placeholder frontmatter
   `close-candidate: <verdict>` — a brief write, path-confined to `docs/streams/**`, on the scan
   carrier PR. The row stays on Next-up as work whose **deliverable IS the close-PR**; a worker-desk
   worker authors `bugs/<N>.md` + `Closes #N`, review judges the claim, the merge closes the issue,
   and the next scan retires the placeholder. A fix-then-close composes and needs no verdict.
3. **AWAIT / unblock.** A placeholder whose worker parked a question on the issue; a new human
   comment unblocks it and the worker resumes on its own PR. Keep the parked set visible — they are
   WAITING-ON-INPUT, not orphans for the fanout sweep.
4. **needs-decision.** An issue or brief that hits a human gate. File (or confirm) a
   `needs-decision` issue per the **brief-06 template**: self-contained (Situation / Options 2–4
   with pros-cons at the mm/12 trade-off bar / What-happens-on-each-answer / Links). The decider is
   the human, and only a verified human account is honored. This is the SINGLE decision queue
   — the intake lane routes into it too, never a second one.
5. **DUPLICATE — merge the evidence first, and this desk never closes it** (human:<name>, 2026-08-02).
   Spotting a duplicate is not authority to close one: it is a unilateral judgement that two reports
   are the same defect, and the loser's non-overlapping evidence dies with it (in the incident that
   set this rule, the closed report carried the run-ID proof its survivor lacked). Issues and PRs
   alike:
   - **Fan out a STRONG-tier worker** — never inline at this desk, never a cheap tier.
   - **The evidence-merge comes FIRST and is mandatory.** The worker folds every missing bit of the
     loser into the survivor *before* anything is marked duplicate. This is the step that incident
     reports.
   - **Pick the survivor on MERIT; first-filed breaks a genuine tie only.** Verify the competing
     claims — a claim that does not reproduce is not evidence. On this procedure's first run,
     mechanical first-filed picked the **less** accurate issue and only the mandatory
     fold rescued the record; the pair were also lossy compressions of one parent record filed 2m28s
     apart by the same bot, so check provenance before reading agreement as corroboration.
   - **The dispatcher names the candidates and the tiebreak RULE — never which one survives.** Naming
     the survivor up front deletes the judgement the strong tier was dispatched to exercise.
   - **The worker marks; the reviewer closes.** The worker labels the loser `duplicate`, cross-links
     the survivor, and stops. A pass independent of that worker closes it, only after agreeing all
     content moved over — `--reason "not planned"`, **with a space**; `not_planned` is rejected.
6. **RETIRE / close-out.** The scan retires placeholders whose issues are now closed (`status: done`
   + `resolved: issue-close`); a closed issue that reopens re-activates on the next scan. Without
   this the board fills with ghosts.
7. **System-emitted labels are excluded** from scanning — `verify-gate`, `live-verify` and
   `needs-decision` issues are closeable *states*, not work; a placeholder for them is noise.

### Close authority (stated once)

**`needs-decision` issues are human-only-close. An issue with human:<name>'s ruling recorded on it
may be closed by the desk, citing the ruling** (2026-08-24 ruling 7). Implementation still
outstanding is not a reason to hold a decided issue open — but the close comment **must NAME the
tracker**: the actual brief id, PR number or issue carrying the remaining work, never the assertion
"the work is tracked"; no tracker, create it first, then close. The relabel is the load-bearing half
and is mandatory — flip `needs-decision` → `human-decided`, which is what takes it off
human:<name>'s queue; the close is board hygiene on top, and it is the close the tracker condition
gates.

The other two close paths are mechanical. A work issue closes when its fixing PR **MERGES** — the
merge is the authorization (`--reason completed`, comment naming the PR and merge date), on the
issue's STATED scope, filing any genuine out-of-scope residue as a NEW issue; approved-but-unmerged
is NOT a close (the merge is human:<name>'s and may not happen), and a draft or mention-only PR never
is. A **duplicate** closes only via step 5 — marked by a strong-tier worker, closed by an independent
reviewer.

## The loop — intake lane (the front door for raw ideas)

Intake entries are **in-git register files** (`docs/streams/intake/*`), not GitHub objects —
lint-checked, typed-linked, tombstoned, portable with the repo. An entry is IDEA-shaped: it needs
judgment before it is work, and triage is the exit.

**Routing test (the load-bearing distinction):** *if you could hand it to a worker as-is, it's an
issue; if it needs judgment first, it's intake.* Intake routes INTO the issue lane
(`disposition: issue`), never the reverse — the two lanes are sequential stages of one funnel
(ideas → work), not duplicates.

**Split-layout repos (issue-loop/15, opt-in per repo):** once a repo has migrated
`docs/streams/intake/` into the five disposition subdirs (`new/`, `decision-needed/`, `watching/`,
`completed/`, `rejected/` — `ls intake/new/` IS the triage board), a triage verb below is **one
commit that does both**: the frontmatter `disposition:` update AND a `git mv` into the matching
subdir. Never one without the other — a mismatch is what `--lint` catches. New entries file under
`intake/new/`; flat-layout repos (and this repo's append-only `docs/streams/INTAKE.md`) are
unaffected, since both layouts parse.

1. **Untriaged-age alarm** (issue-loop/07) → the intake-debt line NOTICEs entries past **3 days** in
   `disposition: new`. Draining that list is this desk's standing job — an untriaged front door is
   exactly the invisibility this loop exists to kill. **Work oldest-first.**
2. **The four triage exits** (issue-loop/08 — every entry leaves `new` as exactly ONE; a reason is
   never optional):
   - **`scoped → <stream>`** — becomes a brief. **Tier gate: triage only QUEUES authoring.** Brief
     authoring is design-tier work (author-brief model-tier gate); a cheap-tier triage session
     **never authors inline** — it marks the entry and the strong-tier author picks it up. If *this*
     window is strong-tier and human:<name> wants it, author then; otherwise queue.
   - **`scoped → issue #NN`** — operational / bug-shaped work → file a GitHub issue (label `bug`
     when bug-shaped, per the project's own convention); record the issue number. It then enters the
     issue lane above.
   - **`decision-needed`** — a call that is human:<name>'s. Requires filing (or already having) a
     `needs-decision` issue per brief-06's template, recorded in the entry's `decision-issue: <NN>`
     field. The intake view renders these at the top as "waiting on a human" — a **pointer** into the
     issue lane's decision queue, never a second queue.
   - **`rejected — <why>`** / **`watching`** — an explicit reason is required. A withdrawn entry is
     tombstoned in-place (`disposition: rejected`), never deleted (deleting trips `--lint`).
3. **Brainstorming ends in an INTAKE entry, never an ad-hoc docs/ dir.** If human:<name>
   brainstorms in this window, the exit is a `disposition: new` intake file for the next triage pass.

## Shared desk rules

Stated once for every desk; this skill adds only what is its own above.

- **Escalation labels:** any desk/loop may label an issue `question` (needs an answer from
  human:<name> or a stronger-tier model to PROCEED — the item PARKS) or `help wanted` (hit a
  capability/authority edge). A bare label is unanswerable — **comment what you need and from whom**
  when labelling; whoever answers removes the label with their response. A `question` that matures
  into a formal decision fork promotes to `needs-decision` (issue-loop/06). Labelled items are
  WAITING-ON-INPUT — they join the human/escalation queue, they are NOT orphans for the fanout sweep.
- **Insight-routing:** a systemic/process insight produced in passing MUST be filed as an issue in
  the shared methodology repo — `medici-finance/assay` by default, or the repo a house's own
  instructions file names for this — commentary is not a register. Repo-specific defects go to that
  repo's own tracker (label `bug`).
- **Main-red → file-first (pointer only).** A `main` / post-merge CI gate going RED is a discovery
  this desk FILES (a `bug` + a draft fixing PR when mechanical), not a stop-and-ask — escalate only
  for a genuine fork, and even then file the issue with a recommended default. Check for an existing
  claiming PR/branch first. The full rule and its CI-native half live in `assay:the-desk`.
- **Refresh, don't remember.** Every decision binds to state fetched *this cycle*: an issue you
  "remember triaging" is not proof of its current label, state or comment thread. The sanctioned
  memory channel is a rolling cycle summary; it tells you *where to look*, the fresh read tells you
  *what is true*.
- **Identity.** Post and file as this desk's own App via the desk verbs (`deskfile`, `deskpost`,
  `deskreply`, `deskpr`), minting with `desktoken <role>` — never a hand-rolled mint script. A shared
  human/operator login makes authorship ambiguous; a token 404-ing on a repo it should cover means
  the wrong installation, never "App not installed". **Fallback:** the App genuinely not installed on
  a target repo → work as the shared operator account WITH a logged note in the artifact (PR body /
  issue comment / register entry), never silently.
- **Git push policy (R8).** NEVER push to `main`, merge, close a human-decision issue, or trigger
  workflows / mutating `kubectl` without human:<name>'s go. Branch push + draft PR is
  standing-authorized (scan commits, tooling, brief authoring, close-out carriers). A blocked push or
  guard refusal is a STOP, never a prompt to route around. Never `git restore`/`clean` a shared
  checkout; isolate in your own temp worktree. No attribution lines anywhere.

**This desk's own grants and denials.** Branch push + draft PR is standing-authorized (scan commits,
tooling, brief authoring, close-out carriers). This desk does **not** flip its own PRs ready
(pr-review-desk), merge, commit Evidence (verify-desk), fan out workers (worker-desk), or trigger
workflows / mutating `kubectl`. Filing issues is allowed; closing is bounded by §"Close authority".

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
