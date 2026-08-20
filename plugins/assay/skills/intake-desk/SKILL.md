---
name: intake-desk
description: >-
  Run the intake-desk — the generic front door of the process desk and the first of four desks in the pipeline (intake-desk → worker-desk → pr-review-desk → verify-desk). Ingests ALL inbound — open GitHub issues, the intake register (I-INCOMING entries), and any incoming request or idea — and converts each into one of five tracked exits: spec/brief · bug/issue · finding · decision-needed · rejected/watching. Scans issues into placeholders, triages raw intake entries through their four disposition exits, files human-decision issues, and closes out resolved issues. Use when starting or resuming the dedicated intake window, or when asked to "run the intake desk / work the front door / triage inbound / triage the front door / work the incoming / intake / run the issue loop / work the issue queue / watch the inbound queue". Role window, no persona; the human decides and merges.
---

# Intake Desk (the generic front door)

> **Home, as of this port.** This file is the portable core of the intake-desk skill — the
> front door of the desk pipeline. A project adopting Assay pairs it with its own project-local
> mechanics: an issue-scanning tool, a token-minting helper for whichever write identity it
> chooses, and its own owned-repo list. Those pieces are project config, not part of this
> portable core, and are shown here only as illustrative examples a project's own wrapper fills
> in with real values.

The **intake-desk** is the generic front door of the process-desk pipeline — the first of the four
desks in the pipeline (`intake-desk → worker-desk → pr-review-desk → verify-desk`). Where
pr-review-desk watches work *leaving* the system (PRs → ready), this desk watches work *arriving*
from **any** source: GitHub issues filed by anyone, intake-register entries (raw ideas), and any
incoming request. Its job is to convert each inbound item into exactly one of **five exits**:

| Exit | What it becomes | Route |
|------|-----------------|-------|
| **spec/brief** | A stream brief (via the author-brief flow) | `scoped → <stream>` |
| **bug/issue** | A GitHub issue (label `bug` when bug-shaped) | `scoped → issue #NN` |
| **finding** | An F-NN finding entry | `docs/streams/findings/` |
| **decision-needed** | A `needs-decision` issue (the single human-decision queue) | `decision-needed` |
| **rejected/watching** | An explicit rejection or watch entry | `rejected — <why>` / `watching` |

- **intake-desk** (this skill, its own window) — turns inbound issues + intake into tracked work.
- **worker-desk** (a separate window) — dispatches workers against the Next-up batch. **This
  includes the issue-placeholders this desk emits: fanning workers out against issue placeholders
  belongs to worker-desk, not here.** This desk's issue-lane output is the placeholder (the
  scanner PR); it never fans workers out itself. The shared noclobber **claims lock**
  (`<repo>--issue-<NN>.claim`) still keeps any two dispatchers disjoint.
- **pr-review-desk** (a separate window) — **reviews this desk's PRs** and flips them ready. Every
  write this desk makes leaves as a draft PR (scan commits, tooling, brief authoring, close-out
  carriers); the review-desk is the check on all of it, exactly as it is for worker PRs. This desk
  never flips its own PRs ready and never merges.
- **verify-desk** (a separate window) — drains `implemented → verified`.
- **The human driver** decides the `needs-decision` issues and merges. Both are always the
  human's.

**This runs as its own standing desk window**, peer to pr-review-desk, owning **both lanes**
(issues + intake) rather than being distributed across other desks' cadences — the inbound loop
gets its own attention precisely because it is where new work first becomes visible.

Run this in a **dedicated window** so its per-minute monitor churn does not fragment the coordinator
desk (`the-desk`). **Only this window runs the inbound monitor** — a second monitor double-files
placeholders and double-triages. This is a role-window with no persona (the persona convention, if a
project uses one, belongs to `the-desk` only); the register/evidence discipline of `the-desk` applies
(read it if not already booted).

## Model requirement — run this desk on a SMART model

**This desk's core work is judgment, so it MUST run on a strong/smart tier — not an economy tier.**
Unlike a worker (which executes a spec someone else already scoped) or a mechanical scan, the
inbound loop *decides*: whether an inbound thing is work or an idea (the routing test), which of the
five exits an entry takes, what a `needs-decision` issue's Situation/Options actually are,
and how to scope an idea toward a brief. Those are the same class of design-tier calls the
`author-brief` model-tier gate protects — errors here compound downstream through every worker the
placeholder or brief spawns.

- **If you are a cheap/economy-tier session** (haiku-class or equivalent): do **mechanical work
  only** — run the scanner, keep the board current, post already-formed placeholders — and **do not
  triage-classify, author decision issues, or scope ideas**. Say which model you are and ask for a
  strong-tier session to take the judgment work. Do not triage anyway "just to keep the queue moving."
- **A downgrade mid-session hard-gates the judgment work**, not the mechanical work (see the push
  policy's probe-vs-assertion rule). The scanner and board reads keep running; classification stops.
- **Trust gate:** inbound issues/PRs/comments not authored by a blessed identity or the desk's own
  identities are ignored unless a blessed identity has commented on them — they sit
  quarantined-visible in the board's EXTERNAL/UNBLESSED lane awaiting blessing; never triage,
  placeholder, or route them. (The blessing authority is project configuration — see the
  roster/trust primitive in the adoption runbook.)

This is why the desk exists as its own smart window rather than a cron: the front door needs a mind,
not a trigger.

## Boot sequence

0. **Prune stale worktrees** (bounded growth; the sandbox and any write-guarding tooling depend on
   it — worktree sprawl can trip shell resource limits and false-positive alarms): run the
   project's own worktree-prune tool if it has one. One-shot at boot; a steady-state timer is the
   natural longer-lived form (launchd / k8s pod / cron).

0b. **Lock your session worktree** (if this session booted into one via a session-boot flow):
   `git worktree lock --reason "intake-desk live session" <worktree-path>` — the cooperative half
   of the prune liveness guard: prune never touches locked trees; unlock is automatic when the
   worktree is removed at session end.

0c. **Operating-envelope preflight — run BEFORE anything is claimed:**
   `deskroster preflight --role intake-loop --root <repo-root>` (installed binary at
   /opt/desk-tools/bin/). Five checks, each three-state (checked-clean / checked-failed /
   could-not-check) with a NAMED remediation: a token mint cold from a fresh shell,
   the App's granted scopes vs this role's rostered duties, a READ-ONLY probe of the
   landing path, the commit email carrying the **bot USER id, not the App id**,
   and the sibling checkouts the queued briefs declare.
   **A red preflight is `could-not-run` for the WHOLE pass:** report the ONE summary line
   and STOP — do not claim work, do not burn a pass, and do NOT file an issue about the
   desk's own envelope (each failing check already names the issue that owns it). A probe
   REJECTION is a STOP: never retry it under another identity (AGENTS.md, "Scope
   rejections"). `deskroster preflight --help` exits 0, so a build that lacks the verb is
   distinguishable (an unknown subcommand exits 5).


1. `cd` to the project's primary desk-tooling checkout. Confirm `gh auth status` and the
   **owned-repo set** — the desk owns the front door for every repo the project has declared into
   that set (a project-local list, not necessarily every repo the organization owns). Keep that
   list in sync with the scanner's own compiled-in repo coverage (see "The board tool" below)
   whenever a repo is added or removed — a monitor or scanner covering only part of the set is a
   recurring failure mode, so audit for it periodically. A repo the token cannot see degrades to a
   per-repo NOTICE and is skipped, never a crash. Keep the `needs-decision` label present in every
   owned repo — a repo added to the set after the label was first created can lack it, so
   `gh label create needs-decision --repo <slug>` before filing a decision issue there.
2. **Run the board** (no bundled Go tool yet — see "The board tool" below; MVP is two commands):
   ```bash
   statusgen --root . --scan-issues --dry-run   # issue lane: placeholders to create / retire
   statusgen --root . --lint 2>&1 | grep -iE 'intake debt|untriaged|needs-decision'   # intake lane: the front-door alarm
   ```
   The first prints one ACTION per open issue {CREATE-PLACEHOLDER, CLOSE-ON-FIX (fixing PR merged),
   RETIRE (issue closed), AWAIT (worker parked), NONE}. The second surfaces the intake-debt line:
   untriaged count, how many are over the 3-day threshold, and the oldest. Together these are your
   worklist.
3. **Arm the persistent monitor** (once — check for an existing one first; never arm a second one): a
   monitor polling `gh issue list --state open` numbers + `updated_at` **across EVERY owned repo in
   the step-1 set** (a monitor that only covers part of the set is a recurring failure mode —
   coverage gaps here mean issues on the uncovered repos sit unwatched until someone notices by
   hand), keyed `<slug>#<num> <updatedAt>`, pre-seeded with current open issues so it fires only on
   genuinely new issues / new comments (a comment = a parked worker resuming, or a human answering a
   `needs-decision`). Cadence ~60s, same as pr-review-desk's PR monitor. One monitor covering all
   owned repos — never one per repo, never a second monitor.
   **Do not hand-roll the poll body — arm the `Monitor` on the durable
   `plugins/assay/scripts/inbound-monitor.sh` (pass an `INBOUND_MONITOR_STATE_DIR` under this
   session's scratchpad).** A hand-rolled `gh` loop keeps re-acquiring the two ways this monitor
   went SILENTLY BLIND in practice: (A) armed from a shell that `export`ed a role
   `GH_TOKEN`, it polls as the App — whose install token 404s the whole `medici-finance/*` set, a
   read failure byte-identical to "no open issues" — so the script UNSETs `GH_TOKEN`/`GITHUB_TOKEN`
   and polls as the keyring account; (B) a global "too few records" floor cannot protect a repo set
   one source dominates, so the script keeps state PER REPO and, when a repo's read fails or returns
   zero where it had issues, RETAINS its previous baseline and emits `MONITOR-DEGRADED: <slug> …`
   (exit 2) instead of rewriting the baseline empty and reporting the recovered issues as a phantom
   flood. It emits `MONITOR-ARMED: N` on seed, one `INBOUND: <slug>#<num> <updatedAt>` per genuinely
   new/updated issue, and collapses a mass update to one `INBOUND-BURST: N over 25 — listing
   suppressed`. Treat a `MONITOR-DEGRADED` line as blind-not-idle: re-check identity/auth, never a
   silent all-clear.
4. Announce "Intake desk up — N issues on the board, M intake untriaged (K over 3d)" with the board,
   then work it.

## The board tool

There is **no bundled Go board yet** (the pr-review-desk analog `deskboard.go` does not have an
issue-side twin). MVP is the two `statusgen` commands in boot step 2. A dedicated read-only
`issueboard.go` — one ACTION per open issue and per untriaged intake entry, computed from `gh` +
the registers — is the natural next brief (a future issue-loop board tool); author it the same way
`deskboard.go` was: stdlib-only, `go run`, read-only over `gh`. Until it exists, the two commands
above are the board.

**The board must cover every owned repo, not just the primary one — but the scanner does not do
that yet.** `--scan-issues` is already multi-repo *in shape*: it sweeps its compiled-in repo list in
a SINGLE run (never one invocation per repo), printing ACTIONs across every slug in the list, with
foreign-repo issues carrying a `<repo-name>-issue-NN` placeholder stem so numbers never collide, and
a repo the token can't see degraded to a NOTICE + skip rather than a crash. What it can lack is
*coverage*: the compiled-in list may trail the project's own owned-repo set. When that is true, the
board is one `--scan-issues` run for the covered repos **plus a manual `gh issue list --repo <slug>
--state open` for each remaining owned repo**. Widening scanner coverage is tracked as separate
upstream tooling work and reaches an adopting repo only via a `statusgen` release + `.assay-versions`
pin bump — at which point the manual sweep drops from this section. Keep the scanner's compiled-in
list and the boot-step-1 owned-repo set in sync (they are meant to be one authority) whenever a repo
is added or the triage conventions change.

## The loop — issue lane (driven off the board / monitor events)

The **GitHub issue body IS the spec** (stream convention). A placeholder never duplicates it; it
points (`issue: <NN>`, `repo: <owner/name>`) and carries only the scheduling metadata Next-up needs.

1. **CREATE-PLACEHOLDER** → a new open issue with no placeholder. Run the scanner for real and let
   it drop the `issue-<NN>` placeholder brief; it flows through Next-up like any brief. **The
   issue-loop stream lives in the desk-tooling repo**, so the scan runs rooted at that checkout and
   its commits land there — placeholders for issues on other owned repos included; only the stream
   directory moves between repos, not the remit. **Never push to main** — the scan commits land on a
   branch → draft PR (path-specific `git add docs/streams/issue-loop/`), same flow as any change:
   ```bash
   cd <desk-tooling-checkout>
   git checkout -b chore/issue-loop-scan-$(date +%Y-%m-%d)
   statusgen --root . --scan-issues
   git add docs/streams/issue-loop/ && git commit -m "chore(issue-loop): scan — N created, M retired"
   git push -u origin HEAD && GH_TOKEN=$(cat <token-path>) \
     gh pr create --draft --title "chore(issue-loop): scan $(date +%Y-%m-%d)" \
     --body "Automated inbound scan. Review created/retired placeholders."
   ```
   (Mint the token first, from the checkout where this skill's own tooling lives — see the identity
   rule in §Escalation below.) Coalesce: if a scan branch from this session is still open, push to
   it instead of opening a new one; close it and start fresh once merged. The scanner is cheap (reads
   `gh`, writes local files) — run it whenever the monitor fires.
2. **Dispatch belongs to worker-desk, not here.** This desk's issue-lane output is the placeholder
   from step 1 (the scanner PR) and nothing more — it never fans workers out from this window.
   Routing-test triage (worker-legible vs `question`/scope) still happens here at placeholder time;
   picking the placeholder up and running a worker against it is worker-desk's job, using the same
   canonical claim key (`<repo>--issue-<NN>.claim`) any lane computes for that issue, so a claim by
   either lane is visible to the other. **Only worker-legible issues are worth dispatching at all.**
   If the issue fails the routing test (thin / ambiguous), do NOT mark it ready to hand off — label
   it `question` with what's missing, or scope it. An under-specified issue wastes whoever picks it
   up and the review that bounces it; triage is the gate that keeps the fan-out productive. This
   desk never touches the claim bookkeeping or the worker's own worktree — that is worker-desk's,
   once it picks the placeholder up.
3. **AWAIT / unblock** → a placeholder whose worker parked a question on the issue. When the monitor
   sees a new human comment, the item is unblocked; the worker resumes on its own PR. This desk's job
   is only to keep the parked set visible (they are WAITING-ON-INPUT, not orphans for a fan-out
   sweep — same class as `question`-labelled items).
4. **DECISION issue** → an issue (or a brief) that hits a human gate — a call only the human driver
   can make. File (or confirm) a `needs-decision` issue that is self-contained: Situation, 2–4
   Options with pros/cons, what happens on each answer, and links to the source material. The
   decider is the human; **closing the issue with the chosen option stated is the decision
   record**, and only a verified human account is honored. This is the SINGLE decision queue —
   the intake lane routes into it too (below), never a second one.
5. **CLOSE-ON-FIX-LANDED — actively close the issue when its fix reaches main.** This desk does not
   wait for someone else to close a fixed issue. (The recurring lesson: a fix merged with a
   cross-reference marker instead of a closing one never auto-closes the issue, so it sits open
   despite being fixed — this desk actively checks rather than trusting auto-close.) For each open
   `issue-<NN>` placeholder, check the issue's timeline for a fixing PR and act on its state:
   - **PR MERGED** → **close the issue** as this desk's own write identity (mint first — see
     §Escalation):
     `GH_TOKEN=$(cat <token-path>) gh issue close <NN> -R <repo> --reason completed --comment "Fixed by #<PR> (merged <date>). Closing."`,
     then the scanner retires the placeholder (step 6). **The MERGE is the authorization** — the
     review loop approved it and it is on main, so this desk MAY close it. This is the one case that
     overrides "the coordinator never closes issues directly": a review-approved, *merged* fix. Close
     on the issue's STATED scope; if the merged PR itself flagged a genuine out-of-scope follow-up,
     **file that as a NEW issue** rather than leave the original open.
   - **PR approved / flipped ready but NOT yet merged** → do NOT close (the merge is the human
     driver's and may not happen). OPTIONALLY comment "fix ready in #<PR>, awaiting merge" so the
     issue reflects state; close on the eventual merge. (Trigger = **merge**, not approval —
     approved ≠ on-main.)
   - No merged fixing PR → leave open.
   Never close on a draft or mention-only PR. `needs-decision` issues still close ONLY on the human
   driver's recorded decision (step 4), never on a PR merge.
6. **RETIRE / close-out** → the scanner also retires placeholders whose issues are now closed
   (`status: done` + `resolved: issue-close`); a closed issue that reopens re-activates on the next
   scan. Without this the board fills with ghosts.
7. **System-emitted labels are excluded** from scanning — `verify-gate`, `live-verify`,
   `needs-decision` issues are closeable *states*, not work; a placeholder for them is noise.

## The loop — intake lane (the front door for raw ideas)

Intake entries are **in-git register files** (`docs/streams/intake/*`), not GitHub objects —
lint-checked, typed-linked, tombstoned, portable with the repo. An entry is IDEA-shaped: it needs
judgment before it is work. Triage is the exit.

**Routing test (the load-bearing distinction):** *if you could hand it to a worker as-is, it's an
issue; if it needs judgment first, it's intake.* Intake routes INTO the issue lane
(`disposition: issue`), never the reverse — the two lanes are sequential stages of one funnel
(ideas → work), not duplicates.

1. **Untriaged-age alarm** → the `--lint` intake-debt line NOTICEs entries past **3 days** in
   `disposition: new`. Draining that list is this desk's standing job — an untriaged front door is
   exactly the invisibility this loop exists to kill. Work oldest-first.
2. **The four triage exits** (every entry leaves `new` as exactly ONE; a reason is never optional):
   - **`scoped → <stream>`** — becomes a brief. **Tier gate: triage only QUEUES authoring.** Brief
     authoring is design-tier work (the author-brief model-tier gate); a cheap-tier triage session
     **never authors inline** — it marks the entry `scoped → <stream>` and the strong-tier author
     picks it up. If *this* window is strong-tier and the human driver wants it, author then;
     otherwise queue.
   - **`scoped → issue #NN`** — operational / bug-shaped work → file a GitHub issue (label `bug`
     when bug-shaped, per the project's own bug-labeling convention); record the issue number. It
     then enters the issue lane above like any inbound issue.
   - **`decision-needed`** — a call that is the human driver's. Requires filing (or already having) a
     `needs-decision` issue with the same Situation/Options/What-happens-on-each-answer/Links shape
     as above, recorded in the entry's `decision-issue: <NN>` field. The intake view renders these
     at the top as "waiting on a human" — but that section is a **pointer** into the issue lane's
     decision queue, never a second queue.
   - **`rejected — <why>`** / **`watching`** — existing vocabulary; an explicit reason is required.
     A withdrawn intake entry is tombstoned in-place (`disposition: rejected`), never deleted
     (deleting trips `--lint`).
3. **Brainstorming ends in an INTAKE entry, never an ad-hoc docs/ dir.** If the human driver
   brainstorms in this window, the exit is a `disposition: new` intake file for the next triage pass.

## Escalation labels & insight-routing (shared desk rules)

- **Escalation labels:** any desk/loop may label an issue `question` (needs an answer from the
  human driver or a stronger-tier model to PROCEED — the item PARKS) or `help wanted` (hit a
  capability/authority edge). A bare label is unanswerable — **comment what you need and from whom**
  when labelling; whoever answers removes the label with their response. A `question` that matures
  into a formal decision fork promotes to `needs-decision`. Labelled items are WAITING-ON-INPUT —
  they join the human/escalation queue, they are NOT orphans.
- **Insight-routing:** a systemic/process insight produced in passing MUST be filed as an issue in
  the project's own toolkit/methodology repo — commentary is not a register. Repo-specific defects
  go to that repo's own tracker (label `bug`).
- **Main-red → file-first (pointer only).** A `main` / post-merge CI gate going RED is a discovery
  this desk FILES (a `bug` + a draft fixing PR when mechanical), not a stop-and-ask — escalate only
  for a genuine fork, and even then file the issue with a recommended default. Check for an existing
  claiming PR/branch first. The full rule and its CI-native half (a workflow that auto-files a
  failure issue for a silent post-merge red) live in `assay:the-desk`; this line only points at them.
- **Desk write identity.** Mint FIRST via the project's own token-minting tool for this desk's write
  identity (pass the target `<owner>/<repo>` or `<org>`; a token 404-ing on a repo it should cover
  means the wrong installation, never "App not installed"). The tool should print the token path on
  its last line; ALL desk writes — issue create/comment/close, labels, scan-PR creation — run with
  `GH_TOKEN=$(cat <token-path>)` so they render as the desk's own App identity, not a shared
  human/agent login (a shared identity makes authorship ambiguous). Intake-entry PRs use the same
  desk App identity per the project's own role-to-App config. **Fallback:** the App genuinely not
  installed on a target repo → work as the shared human/operator account WITH a logged note in the
  artifact (PR body / issue comment / register entry), never silently.

## Git push policy

- **NEVER push to `main`, merge, close a human-decision issue, or trigger workflows / mutating
  `kubectl` without the human driver's go.** Deciding a `needs-decision` issue is the human's — this
  desk FILES and SURFACES them, it never answers them.
- **Branch push + draft PR is standing-authorized** — scan commits and any tooling change (e.g.
  the future board tool) go `git push -u origin <branch>` + `gh pr create --draft`, into the same
  PR→review→merge flow. This desk does not flip PRs ready (that is pr-review-desk) and does not
  commit Evidence (that is verify-desk / post-merge).
- **Filing issues is allowed** (as this desk's own App identity — mint first, §Escalation identity
  rule; the shared human/operator account only as the logged-note fallback): new inbound issues from
  intake triage, `bug` issues, `needs-decision` issues, `question`/`help wanted` labels + the
  required comment. **Closing** an issue is gated — a `bug`/work issue closes when its fix merges
  (the scanner retires the placeholder); a `needs-decision` issue closes only when the human driver
  records the decision.
- Never `git restore`/`clean` a shared checkout; isolate in your own temp worktree.
- No attribution lines anywhere (no Co-Authored-By, no generated-with).
- **Model-tier awareness:** if downgraded mid-session, stop judgment work (triage classification,
  decision-issue authoring, brief scoping) and fall back to mechanical work (running the scanner,
  posting already-formed placeholders, keeping the board current). Probe vs assertion: the human
  driver ASKING what model you are is a probe — answer with the env model line verbatim + ask for
  confirmation, keep mechanical work moving; only their ASSERTION of a downgrade hard-gates judgment
  work.
