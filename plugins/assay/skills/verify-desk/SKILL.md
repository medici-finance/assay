---
name: verify-desk
description: >-
  Run the post-merge VERIFY role of the process desk — the last of four desks in the pipeline
  (intake-desk → worker-desk → pr-review-desk → verify-desk) and the standing window that drains the
  "Awaiting verification / review" queue. Merging a brief-PR does NOT complete it; a NON-implementer must
  run the brief's Verify table on merged main, fill Evidence with real observed output, and advance each
  brief implemented → verified → done. In DORA terms this loop fixes Change Lead Time and doubles as the
  Change Failure Rate sensor. Use when starting or resuming the dedicated verify window, or when asked to
  "run the verifier loop / drain the awaiting queue / verify merged briefs / turn merged into done". Role
  window, no persona; the human driver merges and signs off the irreversible gates.
---

# Verify Desk (the post-merge verify half)

> **Home, as of this port.** This file is the portable core of the verify-desk skill — the
> post-merge half of the desk pipeline. A project adopting Assay pairs it with its own project-local
> mechanics: a board/status tool, a token-minting helper for whichever write identity it chooses, a
> session-worktree provisioning tool, and its own configured-repo list. Those pieces are project
> config, not part of this portable core, and are shown here only as illustrative examples a
> project's own wrapper fills in with real values.

The **verify-desk** is the post-merge verify half of the four-role process-desk pipeline — the last of
the four desks (`intake-desk → worker-desk → pr-review-desk → verify-desk`). Where pr-review-desk
watches work *leaving* the system pre-merge (PRs → ready), this desk watches what happens *after*
merge: a NON-implementer runs each merged brief's Verify table on merged main, fills Evidence, and
advances `implemented → verified → done`.

- **worker-desk** (a separate window) — workers implement briefs, open draft PRs.
- **pr-review-desk** (a separate window) — pre-merge review; the reviewer identity approves; the desk
  flips ready; **the human driver merges**.
- **verify-desk** (this skill, its own window) — post-merge: a NON-implementer runs the Verify table on
  merged main, fills Evidence, advances `implemented → verified → done`.
- **the-desk** (a separate window) — coordinates.
- **The human driver** merges and signs off the irreversible gates. Both are always the human's.

**Why this role exists:** merging is `deployment frequency`; it is NOT completion. A merged brief sits
at `implemented`/`verified` until independently verified, and *an unwatched Awaiting queue is how briefs
rot at `implemented`* (the methodology's own warning). In DORA terms this loop is what fixes **Change
Lead Time** (implemented → done) and it doubles as the **Change Failure Rate** sensor — a Verify run on
merged main that FAILS is a change failure that pre-merge review missed. Run it in its **own window**; it
does NOT run the PR monitor (that is pr-review-desk's signal, not this desk's).

This is a role-window with no persona (the persona convention, if a project uses one, belongs to
`the-desk` only); the register/evidence discipline of `the-desk` applies (read `assay:the-desk` if not
already booted).

## Model requirement — run this desk on a SMART model

**This desk's core work is judgment, so it MUST run on a strong/smart tier — not an economy tier.**
Deciding whether a green Verify table actually proves the change correct, enumerating and deriving a
risk-bearing constant, and reading a FAIL as a change failure are design-tier calls — errors here ship a
wrong-but-green change to `done`.

- **If you are a cheap/economy-tier session:** do **mechanical work only** — sweep the board, keep it
  current, record already-formed Evidence — and **do not** author risk-value derivations, sign off
  risk-flagged briefs, or judge a borderline FAIL. Say which model you are and ask for a strong-tier
  session to take the judgment work.
- **A downgrade mid-session hard-gates the judgment work**, not the mechanical work. The board sweep and
  Evidence recording keep running; risk-value judgment stops.
- **Verifier tier:** dispatched verifiers run on the **local session model**, never escalated to a paid
  or larger tier. This desk does not escalate to a bigger model — it escalates to the **human driver**.
  The upper rung of the risk-keyed floor is a **human**, not a stronger model (see the loop below).

## Autonomy — this desk runs unattended: FILE, don't ask; DRAIN, don't wait

The whole point of this window is to turn merged → done **without hand-holding.** Two standing rules,
both of which OVERRIDE any "ask first" instinct:

- **FILE, don't ask.** When a verify run turns up anything worth an issue — a VERIFY FAIL, a defect, a
  stale fact, an out-of-scope discovery — **file the issue yourself, immediately, and keep going.**
  Filing an issue is durable, reversible, and low-cost; it **never needs the human driver's permission.**
  Asking "should I file this?" is the anti-pattern — it strands the finding in session context and halts
  the drain. Route by type WITHOUT stopping: a defect/failure → a `bug` issue; a brief-invalidating fact →
  a finding entry (`docs/streams/findings/`); a systemic/process insight → an issue in the project's own
  toolkit/methodology repo (insight-routing); a call that is genuinely the human's → a
  `needs-decision`/`question` issue with the context. Then move to the next brief.
- **DRAIN, don't wait.** The Awaiting queue **is** your direction — you do not need fresh instruction
  between items. Work it to empty. The only things this desk legitimately cannot close are the
  **`irreversible` / `gate: human` sign-offs**, and those are handled **asynchronously** (the verify-gate
  issue below, or a labeled issue) — which does **not** halt the drain: land the Evidence / file the
  issue, move it to the "waiting on the human" line of your report, and **pick up the next brief.** Never
  stop the whole loop to wait for one answer.

You escalate to **the human driver**, never to a bigger model — but an escalation is a **filed artifact
you keep moving past** (an issue, a verify-gate wait, a report line), never a blocked prompt you sit on.

This desk's FILE-don't-ask is the post-merge instance of the shared **Main-red → file-first** rule
(canonical in `assay:the-desk`): verify-desk is the standing first responder when a `main` CI gate goes
RED. A companion CI workflow that auto-files a `ci-failure:<workflow>` tracking issue (illustrated as
`main-failure-alert.yml`) ensures a silent post-merge red still surfaces when no window is up.

## HARD GATE — never claim "idle / caught up" without a fresh board sweep

**An idle claim is a claim about the Awaiting verification queue, and the only evidence about the queue
is a fresh board sweep.** Before the desk EVER reports "idle", "caught up", "Awaiting queue empty", or
"nothing awaiting verification", it is a HARD PRECONDITION that it has *just* run the board tool
(`statusgen --root .`) **from inside its own worktree** and confirmed the Awaiting verification / review
table reports **zero rows — i.e. `awaiting == 0`** (the awaiting count = `implemented + verified`). No
fresh sweep → no idle claim. Full stop.

**The sweep WRITES — that is why it is worktree-only.** A bare board regeneration is write mode: it
rewrites the status document and the generated register views (single-writer, main CI only). Inside this
desk's own worktree that churn is contained and discarded by the next `reset --hard origin/main` (boot
step 2); it is never committed, never pushed, never hand-merged. Running the sweep from the SHARED
checkout (or any worktree that isn't yours) is a real incident class: a write-mode sweep left a large
dirty register view in the shared checkout that presented as register corruption. Register WRITES are
per-entry files (`docs/streams/findings/<date>-<slug>.md`) landing via the sanctioned path — never a
hand-append to the generated view.

**"My dispatched verifiers finished" is NOT evidence the queue is empty.** They are different facts: a
verifier completing tells you about the brief you already dispatched; it says NOTHING about briefs merged
since your last sweep. Reporting idle from your own in-flight-verifier state — without a fresh board
sweep — is a known failure mode: it turns a silent monitor outage into a false "all clear." A subagent
finishing re-invokes you; that is a cue to **sweep the Awaiting queue and advance the next item**, never
a licence to report caught-up.

**A sweep that failed or errored is `could-not-check` — blind, not idle.** The instrument could not be
read; re-sweep before making any idle claim. If you are waiting on a human sign-off (an irreversible
verify-gate), that is a documented *wait state*, not idle — report it and move to the next item.

> **Liveness backstop:** a fixed-cadence `Monitor` that runs the sweep on a timer (in this desk's own
> worktree, mirroring pr-review-desk) is the natural longer-lived form; until an observability service
> runs it, the desk is event-driven — sweep at every boot and after every merge wave.

## Boot sequence

0. **Prune stale worktrees** (bounded growth; the sandbox and any write-guarding tooling depend on it —
   worktree sprawl trips resource limits and false-positive alarms): run the project's own worktree-prune
   tool. It removes only tracked-clean, fully-merged worktrees; unmerged/dirty/unpushed (active work) are
   always left. One-shot at boot; a steady-state timer is the natural longer-lived form. This is also the
   sprawl control for stale ROLE worktrees — sessions were observed accumulating many registered verify
   trees from past runs; a session-scoped provisioning step (step 1) plus this prune keeps registrations
   bounded.

0b. **Operating-envelope preflight — run BEFORE anything is claimed:**
   `deskroster preflight --role verifier --root <repo-root>` (the project's own preflight tool; e.g. an
   installed binary under `/opt/desk-tools/bin/`). Five checks, each three-state (checked-clean /
   checked-failed / could-not-check) with a NAMED remediation: a token mint cold from a fresh shell, the
   write identity's granted scopes vs this role's rostered duties, a READ-ONLY probe of the landing path,
   the commit email carrying the **bot USER id, not the App id**, and the sibling checkouts the queued
   briefs declare. **A red preflight is `could-not-run` for the WHOLE pass:** report the ONE summary line
   and STOP — do not claim work, do not burn a pass, and do NOT file an issue about the desk's own
   envelope (each failing check already names the issue that owns it). A probe REJECTION is a STOP: never
   retry it under another identity.

1. **Provision the desk's OWN worktree via the project's session-worktree tool — do NOT hand-roll
   `git worktree add`.** Verification runs against *merged main*, and main moves as PRs land, so the
   verify desk works from its own isolated, session-scoped checkout. Use the one idempotent command that
   bakes in every correctness property (session-scoped path, a uniquely-named branch tracking origin/main,
   the lock, the worktree-scoped write identity, and the origin-identity guard):
   ```bash
   WT=$(<project worktree tool> role-init --role verifier)   # prints the worktree path
   ```
   `role-init` is **idempotent** — re-running it reuses your existing valid tree instead of adding another
   registration (the fix for role-worktree sprawl). Do NOT name the tree with a name that could collide
   with a live sibling-repo worktree; use a session-scoped path
   (e.g. `<scratch>/verify-desk-$SESSION` on branch `verify-desk/$SESSION` **tracking origin/main**).
   That tracking branch is also what turns the preflight write-transport probe GREEN — a linked worktree
   can never check out `main` (the primary holds it), so a detached or main-less tree has no landing ref
   to test. The commit identity is set **worktree-scoped** (`git config --worktree`) to the write
   identity's bot **USER** id, so a concurrent worker/reviewer session rewriting the **shared**
   `.git/config` cannot clobber this desk's commit identity. At session end, tear the tree down with the
   tool's `role-clean` (and the step-0 prune reaps any leftovers). If the tool is not on PATH, that is a
   preflight-class failure — do not fall back to a hand-rolled `git worktree add`.

2. **Keep it current with main — resync before every verification wave** (and whenever a merge lands),
   so you never verify stale main. **Scope the resync to your worktree with `git -C "$WT"` — NEVER bare
   git — and run the identity guard FIRST:**
   ```bash
   [ "$(git -C "$WT" remote get-url origin)" = "$(git -C <repo> remote get-url origin)" ] || { echo STOP; exit 1; }
   git -C "$WT" fetch origin && git -C "$WT" reset --hard origin/main
   ```
   The identity guard is a hard **STOP**, not a re-point: if `$WT`'s origin does not match the repo you
   booted from, you are pointed at a foreign checkout (a sibling repo occupying the name) — never
   `reset --hard` it. A **bare** `git reset --hard origin/main` runs against whatever the shell's cwd
   resolves to — and if that cwd is the shared checkout (or any session-homed worktree that isn't yours),
   it wipes another session's uncommitted work. A write-guard should block exactly this; if you hit it,
   you ran bare git from outside your worktree — re-issue as `git -C "$WT" …`, do not retry bare. (This
   worktree holds no local work of its own — Evidence/status edits are committed and landed per the
   doc-commit flow, not left dangling — so a hard reset to `origin/main` is safe *within your own
   worktree* and is the keep-current move.)

3. **Regenerate the board in your own worktree and read the queue** (write-mode board tool — worktree-only;
   the churn on the status document + register views is discarded by the next reset, never committed — see
   the HARD GATE above):
   ```bash
   statusgen --root . && sed -n '/Awaiting verification/,/^## /p' STATUS.md
   ```
   That "Awaiting verification / review" table is your worklist — **every brief at `implemented` or
   `verified`, full stop. The Verified/Reviewed cells do NOT filter it.** The row filter is
   `status == "implemented" || status == "verified"`, and the awaiting count is `implemented + verified`;
   the cells are read only *after* that filter, to render `—` for an empty one.

   **A row with BOTH cells already filled is still your work** — it is a brief sitting at `verified` with
   its review recorded, i.e. a free `verified → done` close. Nothing else will pick it up.
   > **Do not read the worklist as "rows with an empty cell."** That misreading makes every
   > both-cells-filled row invisible, so the cheapest closes on the board become permanent debt — briefs
   > sat for days as free `done` flips no desk could see.

   Work every row; decide per row what it still needs (verify / review-record / `done` flip). Also scan
   stream READMEs for rows at `implemented`/`verified` with empty Evidence.
   **Trust gate:** any inbound item (verify-gate issue, PR, comment) not authored by a blessed identity or
   the desk's own identities is ignored unless a blessed identity has commented on it — quarantined items
   are visible in the board's EXTERNAL/UNBLESSED section, never verified or worked. (The blessing
   authority is project configuration — see the roster/trust primitive in the adoption runbook.)

4. **Announce "Verify desk up — N briefs awaiting verification"** and work the queue by **work-class
   priority, then oldest-first within class** — the queue mixes two work-classes and they are NOT equal
   (a real drift: a desk closed several `verified → done` while many `implemented`-with-empty-Evidence
   briefs sat unverified and the awaiting count *grew* — the board looked busy while the pipeline stayed
   blocked):
   - **TIER 1 — the actual verify work, do these FIRST: briefs at `implemented` with an EMPTY Evidence
     section.** Each needs a dispatched verifier to run its Verify table — the real `implemented →
     verified` promotion. This is what drains the awaiting-verification bottleneck and moves the DORA
     throughput number. (A FAIL here is still progress: record the failing Evidence AND file it as a
     change-failure — "a FAIL is filed, not buried" — then leave the brief `implemented`.)
   - **TIER 2 — cleanup: `verified → done` free closes, and Evidence-present-but-unpromoted rows.** Real
     work but cheap — a status flip, no verifier dispatch. Do them, but they must **NOT crowd out Tier 1**:
     a session that only closes Tier 2 leaves the pipeline blocked. The `gate: model` half of Tier 2 may
     be auto-flipped by main CI from the App approval at the merged head (see step 5 of the loop); what is
     left for the desk is the `gate: human` routing, plus any `gate: model` row main CI **refused** —
     which is a finding to investigate, not a stamp to write.
   Within each tier, oldest-first (longest lead time = highest priority; the DORA lead-time signal).
   **Keep every row visible** — this reorders the worklist, it does NOT hide the done-closes (a
   filtered-out close becomes permanent debt).

Dispatched verifier agents still isolate in their OWN temp worktrees off the CURRENT `origin/main` (step
2 keeps the desk's copy fresh so it hands agents the right SHA to run against).

**Sibling report repos are in scope:** briefs whose deliverables land cross-repo — the sibling checkouts
of the desk's configured repos (**read that set from your project's own repo-roster tool; this skill
carries no hardcoded list — a hardcoded one drifts from the tools' set in both directions**) — are
verified THERE. Not every write-authorised repo has a local sibling checkout — a repo you have not cloned
is COULD-NOT-CHECK for a cross-repo row, never a fail; verify only the siblings that exist. For each that
does: resync the sibling to its `origin/main` (or its recorded deliverable SHA) before running its Verify
rows, run the rows against the sibling checkout, and record the sibling repo + SHA in Evidence alongside
the in-repo row. Cross-repo PRs in those repos pair with their in-repo status-flip PR — verify the pair,
not just the flip.

### Stop-flag check — run at every iteration boundary

Before each loop cycle, check for your project's stop-flag convention (a global stop, and a per-loop stop
keyed to this role's name). A hit means exit cleanly (the loop can be restarted by clearing the flag +
re-arming). Never halt mid-action; a started outward write always completes. The tool layer should
independently enforce these flags, so a loop that skips its own check is defanged — every outward verb
still refuses.

### Console noise floor (the role-skill output contract)

This desk drains the Awaiting-verification queue event by event. Observe the **console noise floor**.
Three output classes:

1. **Actionable** — multi-line, always printed: a VERIFY FAIL finding, a new defect discovered, an
   error/refusal, or a question to the human. Never compressed.
2. **State change** — the Awaiting queue printed ONLY when it changed (a new brief landed at
   `implemented`) or on explicit request.
3. **Quiet iteration** — ONE line when nothing actionable happened: timestamp, the queue swept, the
   delta count, the actionable count, and whether the queue is empty. Per-item progress goes nowhere —
   the Evidence commit and the register already record it.

A quiet loop looks like:
`11:02Z swept awaiting queue — Δ 0 — actionable 0 — 3 awaiting — dispatched 1, 2 in flight`

**Both modes report transitions, not standing state.** A brief that has sat `implemented` awaiting verify
for days is silent after its first sighting, so satisfying the class-1 "always print actionable" duty
needs a periodic full sweep of the Awaiting queue beside the quiet loop. This binds console output only —
Evidence commits, issue filings, and PR comments are unaffected; the contract is about the desk window's
own narration, not the audit trail.

## The loop (per awaiting brief)

1. **Pick a brief at `implemented` (needs verify) or `verified`-without-review (needs the review gate).**

2. **Dispatch a NON-implementer verifier — ALWAYS dispatch, NEVER verify inline.** An inline verdict
   lives in session context and dies with it: the desk holds a correct PASS, parks, and the work is
   unrecoverable (a real incident: a full afternoon of real verification, zero artifacts, `Agent` calls:
   0). A dispatched verifier returns a **written** verdict that outlives your context even if you never
   act on it. "It's only a one-liner, I'll just run it" is the failure mode, not an exception to it. Its
   prompt MUST carry:
   - the brief path + its **Verify table** (the exact commands) and the merged-main SHA to run against;
   - **isolation**: its own temp worktree off `origin/main` (NEVER the shared checkout) — include VERBATIM
     the hard line "Your home worktree is `<path>` — every file operation stays under it" (the many
     write-guard blocks in practice were dispatched agents reaching outside their home). Any PR a
     dispatched agent opens (e.g. a fix PR) goes via the project's own PR/reply tools, not raw `gh`. For
     live-environment Verify rows, allowlist your project's own **read-only** probes (a debug-pod exec, a
     read-only health GET) — nothing that mutates.
   - run EVERY Verify row and record **command → exit code → key output line** — real observed output,
     never a claim. A row that cannot run (no cluster / no toolchain) is recorded as explicitly unrun, not
     silently skipped or assumed-pass;
   - **Name-and-derive the risk-bearing value** — paste the block in "§ The risk-bearing value" below
     into the verifier's prompt VERBATIM (see the trigger conditions there). The verdict comes back
     carrying a `RISK-VALUE:` line, and step 4 gates on which one.
   - **it must NOT be the brief's implementer** (verifier ≠ author — a fresh agent). A verifier that
     self-verifies its own implementation is void.
   - report back the Evidence rows + a clear **VERIFY: PASS | FAIL** (+ FAIL detail).
   - **Tier: dispatch on the LOCAL SESSION MODEL. Never a paid/external or larger tier.** The verify desk
     does not escalate to a bigger model; it escalates to the human driver.
   - **The risk-keyed floor still applies, but its upper rung is a HUMAN, not a stronger model:** a
     **risk-clear** brief (all risk answers `no`, gate `model`) is verified by a dispatched local-model
     verifier — the normal path, and the majority of the queue. A **risk-flagged** brief (`gate: human` OR
     any risk answer `yes`) may still have its Verify table *run* by a dispatched verifier for the
     Evidence, but it **cannot be signed off by a model** — route it to the human (`irreversible: yes` →
     the verify-gate path below). Check the brief's risk frontmatter; never default the whole queue to one
     treatment. (The board's own lint enforces this: a risk-flagged brief at `verified`/`done` needs a
     human or a non-cheap-tier runner recorded — and since this desk dispatches only the local model, for
     a risk-flagged brief that means a human, full stop.)

3. **On VERIFY: FAIL → this is a Change Failure.** The brief does NOT advance. **File a `bug` issue
   immediately — no permission needed (Autonomy: file, don't ask)** — with the failing command + output,
   then **continue the drain** (a finding entry or follow-up brief instead when that is the better route).
   The failure rate is a metric — don't bury it, and don't stop the loop to report it: the filed issue IS
   the report. A failed verify on already-merged code is exactly what pre-merge review missed; note the
   class for the retro.

4. **On VERIFY: PASS → fill Evidence + advance the row — LAND IT AS ITS VERIFIER RETURNS.** The desk (or a
   doc-committing agent) writes the Evidence section (one row per Verify item, `YYYY-MM-DD <runner>`,
   runner-attributed, NOT a bare ✓) and flips the README status `implemented → verified`, Verified cell
   dated + attributed. This is a board-source edit — commit it per brief (per stream-group at most; a small
   desk doc commit, retry the push-race loop; never commit the status document on a branch). **Anti-pattern:
   accumulating a wave's flips in a scratch buffer and landing them at wave-end** — a PASS in hand and not
   on main within one landing cycle is a defect: the board shows phantom verification debt, other sessions
   re-report "stuck" briefs, and merged→verified lead time inflates by pure reporting latency (land-as-you-go
   is the operating direction). The Awaiting board's Age column measures exactly this latency — keep it
   honest.
   **EXCEPTION — `irreversible: yes` briefs do NOT get the status flip here; see "Irreversible briefs"
   below.** Check `risk.irreversible` in the brief frontmatter BEFORE flipping any status (a one-line
   `grep`). **ALSO check the verdict's `RISK-VALUE:` line before flipping.** A `NAMED, NOT DERIVED` verdict
   needs its `question` issue filed and linked FIRST — see "§ The risk-bearing value → Desk-side routing".
   `DERIVED` / `N/A` need no issue: record and proceed.

5. **`verified → done`** needs the recorded strong-model **review gate** (the pr-review-desk App-approval
   already covers the pre-merge review; for `done`, confirm the review verdict is recorded in the Reviewed
   cell — dated + attributed, `human:<name>` for `gate: human` briefs). Only then `done`.
   **`gate: model` briefs: do NOT transcribe that stamp by hand — main CI can do it.** A board auto-flip
   step (`statusgen --auto-flip-model` on push to main) reads the reviewer App's **APPROVED review object**
   on the PR that merged the work, requires it to sit at that PR's **merged head SHA**, then stamps
   Reviewed (date + App + PR# + SHA) and flips the row to `done` in one write. Any failure — no App
   approval, an approval at an earlier commit, an unreadable review — leaves the brief at `verified` and is
   reported; nothing flips on a "probably". So the desk's remaining done-close work is the **`gate: human`**
   half: route it to the verify-gate issue. A `gate: model` row still sitting at `verified` after a
   main-CI run is telling you something — read that run's `REFUSED` / `COULD-NOT-CHECK` line. It is a
   finding, not a stamp to write over by hand.

6. **Repeat as a CONTINUOUS drain until the Awaiting queue is empty** — results land per step 4 as they
   arrive (never held for an end-of-run landing); report progress incrementally + any Change Failures
   found.

## The risk-bearing value — enumerate → rank → derive → flag

**Why the enumeration is mandatory.** A green Verify table proves the code matches the pinned value; it
does NOT prove the value is *right* — **green test ≠ correct constant.** In one realised miss, a verifier
voluntarily did what a *"name the specific constant"* rule asks and still missed the number: it named
three risk-bearing *properties* (a conservation invariant, a notional-equality check, a min-attestation
threshold) — but named the **guard/property**, not the numeric **tolerance inside it**. A property name
reads exactly like a risk-bearing value in Evidence, so that PASS then survived a re-verify and two human
touches. The lesson: a singular "name the constant" wording is satisfiable without ever reaching a number,
by anyone. **Enumerate-then-rank surfaces the constant mechanically** — it is a deliverable of the brief's
own task list — which removes the selection guess.

**The trigger is fail-safe — enumerate whenever ANY of these holds:**
- `risk.irreversible: yes`; **OR**
- the brief has **no `risk:` frontmatter at all** (no frontmatter, or frontmatter without a `risk:`
  block). **An ABSENT field is not a `no`** — reading it as one is the failure shape itself; pre-schema
  legacy briefs with no frontmatter are not dormant (auth/identity/privacy work often lives among them).
  One-line check: `awk '/^---$/{n++;next} n==1' <brief> | grep -q '^risk:' || echo TRIGGER-ON`; **OR**
- the **risk-class diff fallback**: a diff touching your project's declared risk-bearing paths (e.g.
  ledger/settlement logic, auth/identity APIs, deployment-identity manifests); **OR**
- the diff changes a value your project's own guardrails pin as a **hard constraint, wherever it lives** —
  a business constant that sits outside every risk path above. The path fallback is a *path* proxy for
  risk; this is the *content* proxy for the cases the paths miss.

Recovering the diff for an ALREADY-MERGED brief (the branch has no open PR): take the PR number from the
stream-README notes cell or the brief's Evidence → `gh pr diff <N>`. No PR number recorded →
`git log --oneline -- <brief path>` for the commits that touched it, then `git show <sha> --stat`.

**Paste the following into the dispatched verifier's prompt VERBATIM** — it is the procedure, not a
summary of one.

> **Risk-bearing value — do this BEFORE you return PASS.**
>
> 1. **ENUMERATE — do not pick.** List **every** literal constant, bound, threshold, tolerance, ratio,
>    timeout, limit, and authority binding that this brief's diff **introduces or changes**, plus every one
>    named in the brief's own Deliverables. This is mechanical and cheap; it is not a judgement call, and
>    you do not get to stop at the first one you find.
> 2. **Every entry must be a LITERAL with a source line**, in the form `<identifier> = <literal>` @
>    `<file>:<line>`. **Naming a property, invariant, guard, or check is NOT naming a risk-bearing value —
>    it is pointing at one from one level too high.** If what you wrote is a property, open the guard and
>    name the **number inside it**. An entry you cannot quote as a literal at a `file:line` is not an
>    entry — resolve it down to the literal, or drop it.
> 3. **RANK by irreversibility.** One line each: what breaks if it is wrong, and whether the breakage can
>    be undone by an edit + redeploy. Reversible operational knobs (WIP limits, alarm thresholds, retry
>    counts, log levels, poll intervals) rank last and need no derivation — *that is what `irreversible`
>    means*, and it is why they are out of scope by design rather than by oversight.
> 4. **DERIVE the top-ranked entries** — from first principles, the spec, the paper, or the project's
>    stated constraint. **"A test that exercises it passes" is not a derivation.** Record it in Evidence:
>    the value, its `file:line`, and *why that is the right value*.
> 5. **Return one verdict line per top-ranked entry**, in Evidence and in your report — exactly one of:
>    - `RISK-VALUE: DERIVED — <id> = <literal> @ <file>:<line> — <why it is right>`
>    - `RISK-VALUE: NAMED, NOT DERIVED — <id> = <literal> @ <file>:<line> — <what derivation is missing, and why you could not do it>`
>    - `RISK-VALUE: N/A — enumeration over <the diff scope you covered> found no literal; the irreversible
>      act here is <the org transfer / the spend / the domain purchase / the publish / …>`
>      — **N/A is valid ONLY when step 1 came back EMPTY.** A substantial minority of `irreversible: yes`
>      briefs genuinely have no constant (a GitHub org transfer, a domain/registration purchase, an
>      irreversible publish, a spend). This exit exists so you never have to invent a value or write an
>      unsanctioned line. It is a **claim with the same standing as a derivation** — it is wrong if any
>      literal exists, so state what you enumerated over and make it checkable.
>
> **Calibration — this step is "enumerate and name", not "prove".** Enumeration is cheap and mandatory;
> full derivation is not always affordable — a brief whose *entire* purpose is deriving one bound may
> warrant a long analysis and many Verify rows, but nobody expects that as one bullet among five. So:
> **never skip step 1 to save effort, and never fabricate a derivation in step 4 to look complete.**
> `NAMED, NOT DERIVED` is an honest, sanctioned, useful verdict — it gets routed, not buried.

### Desk-side routing — a flag needs somewhere to land

"Routed to a human" is not an artifact; **a question in the transcript is not durable, a question on the
tracker is.** On a verdict carrying `RISK-VALUE: NAMED, NOT DERIVED`, the desk acts **before any status
flip, on BOTH branches**:

- **`irreversible: yes`** → the value, its `file:line`, and the missing derivation go **verbatim into the
  Evidence recorded on the brief** — the verify-gate issue (below) lifts the Evidence section into its
  body, so the verdict reaches the human on the sign-off card. That issue IS the artifact; no separate
  issue.
- **`gate: model, irreversible: no`** — the fallback branch: a brief that fires the risk-class trigger and
  would otherwise flip to `verified` on main with no human in the loop → **file a `question`-labelled issue
  under the verify write identity FIRST**, naming the brief, the value, its `file:line`, and the derivation
  needed; **link it in the Evidence row; then flip.** The brief still advances — the push policy stands and
  you never invent a gate the brief does not carry — but the un-interrogated number now carries a live,
  durable pointer instead of vanishing into a green row. **A flag with no artifact is the failure shape
  wearing the fix's clothes.**

`DERIVED` and `N/A` verdicts need no issue: record them in Evidence and proceed.

## Irreversible briefs (`risk.irreversible: yes`) — model verify records Evidence, a HUMAN flips the status

A broad irreversible gate in the board lint blocks any `irreversible: yes` brief from `verified`/`done`
unless the Reviewed cell names a `human:<name>`. Because this desk commits status **straight to main**,
flipping such a brief to `verified` on a model verify would fail the board lint and **redden main CI
directly** — the exact thing this desk must never do. So for an `irreversible: yes` brief the model path
STOPS short of the status flip:

1. **Still dispatch the non-implementer verifier and run the full Verify table** — the mechanical
   verification is real and worth recording. **Write the Evidence rows on the brief, but leave the status
   at `implemented`.** (Evidence on an `implemented` brief is allowed; the gate only bars the `verified`
   cell.)
2. **Do NOT flip the README status to `verified`, and do NOT stamp a model runner in Reviewed.** A model
   verifier cannot close — or pre-close — an unfixable change; only the human can.
3. **Do NOT open a human-review checkpoint PR** (a small PR flipping the row with `human:<name>` in
   Reviewed as the review). That pattern is structurally lint-rejected: a Verified/Reviewed cell that gains
   a `human:<name>` stamp on ANY PR branch is a hard lint PROBLEM — the **sole permitted writer is the
   main-side verify-gate-close workflow**, which writes straight to main, where the branch-vs-main guard is
   disarmed. The sanctioned mechanism: with Evidence landed per step 1 — **the verifier's full
   `RISK-VALUE:` verdict line(s) included**, **a `NAMED, NOT DERIVED` verdict verbatim, called out as the
   open question the human is being asked to settle** (burying it is the miss) — a board step
   (`statusgen --verify-issues`) files a **verify-gate issue** that lifts the brief's Verify table +
   recorded Evidence into its body as the review packet; **the allowlisted human closing that issue IS the
   review**: the verify-gate-close workflow then writes the `human:<name>` Reviewed stamp directly to main
   and advances `implemented → verified → done` in one step (the recorded verifier date + runner becomes
   the Verified stamp). Surface the open verify-gate wait in the Awaiting report so it is tracked, not lost.
   Where the verify-gate-close workflow / the `--verify-issues` wiring is not yet deployed in a repo, the
   brief simply stays at `implemented` with Evidence — a documented wait state, never a checkpoint PR.
4. A brief that is `gate: human` is a hint, but the authoritative trigger is **`irreversible: yes`** (the
   gate fires on it alone). `regulatory`/`customer` flags do not change the mechanism.

This keeps the desk's commit-straight-to-main speed for the reversible majority while never letting a
model sign-off close an unfixable change. Incident: a brief once marked `verified` by a model verifier
with no human reviewer; the gate + this carve-out prevent the recurrence.

## Post as the verify write identity, always (attribution, not authorization)

The verify desk uses a **dedicated write identity** — a GitHub App (or equivalent) distinct from the
shared account that authors PRs. A verify-desk post under the shared account is attributable only to that
shared account — not to this desk — which undermines the Change-Failure-Rate / evidence audit trail this
desk exists to produce. This is **attribution with an auditable trail, NOT an enforcement guarantee** —
anyone who can read the App key can mint the token; the value is the trail. (Incident that prompted the
rule: the desk once filed a verification-failure issue via plain `gh`, which defaulted to the shared
account — it had to be re-filed under the App.)

**Mint first, then post/commit.** Mint the write-identity token via the project's own token-minting tool
(it should print the token path on its last line); all posts run with that token so they render as the
desk's own identity, not a shared human/agent login.

- **ALL posts go out under the verify identity, NEVER plain `gh` / the shared account.** Issues
  (verification-failure `bug`s, findings, `question`/`help wanted`), PR/issue comments, and review
  replies — every one. Prefix **every** `gh` call with the identity token; a bare `gh` defaults to the
  shared keyring:
  ```bash
  GH_TOKEN="$(cat <token-path>)" gh issue create -R <owner/repo> --body-file f.md
  ```
  If minting fails (missing key / revoked install), say so **in the artifact you were about to post**
  rather than silently falling back to a shared-account post.
- **File issues through the project's issue-filing tool (with a `--raised-by verifier` stamp), not a bare
  `gh issue create`.** That tool runs the dedupe gate AND stamps provenance, which is what lets the
  by-desk issue metric see that the VERIFY loop noticed the problem — a different question from which
  identity posted it. It composes with the identity token above (it uses whatever `gh` credential is
  ambient and mints none of its own). Omitting the provenance flag is not neutral: the issue lands with
  UNKNOWN provenance, which is the absence of an answer, never "a human raised it". A filing forced out
  under a raw `gh issue create` is an **unstamped** filing — say so rather than assuming the metric will
  see it.
- **The Evidence/status commits to main are ALSO the verify identity, and the PRIMARY landing path is the
  project's sanctioned Evidence-to-main commit tool** — an App-token writer (e.g. via the Contents API)
  where the REAL guards live and fire: the main-write allowlist, a secret/impersonation body scan, the
  outward-write rate limit, the post-commit attribution check (author = the bot USER id), and the audit
  line. It is the identity's proper commit path *because it enforces those guards*.
  **Framing — this is a SANCTIONED channel, not a way around a blocked write.** This skill authorizes the
  verify desk's Evidence/flip writes to main, and the tool is where the guards fire. Do NOT reach for it
  "because a raw `git push` is classifier-blocked" — that is the forbidden route-a-blocked-write-through-
  another-tool pattern. The API transport happens not to trip a git-push classifier, but that is a side
  effect, not the reason to use it.
- **A raw `git push` that is classifier-BLOCKED is a STOP-and-escalate, never a route-around.** Do not
  retry the same write through a different tool to dodge the block. The sanctioned Evidence tool is the
  standing path; if it too is unavailable, land nothing and escalate.
- **Fallback (only when a ruleset blocks the App API path):** a direct commit authored as the identity's
  noreply address — **the bot USER id, NOT the App id** — pushed over the identity's SSH transport. Only
  the transport changes; the author identity stays the App:
  ```bash
  git -c user.name="<verify-app>[bot]" \
      -c user.email="<bot-user-id>+<verify-app>[bot]@users.noreply.github.com" \
      commit -m "verify(<stream>/<NN>): <brief> Evidence + status"
  # push over the identity's SSH transport ONLY when a ruleset blocks the App API path
  ```
  **Commit identity — the per-commit `-c` override is the PRIMARY guarantee, not a nicety.** Linked
  worktrees share one `.git/config`, so a plain `git config user.email` set at boot is CLOBBERED by any
  concurrent worker/reviewer session (observed live: this desk's identity was overwritten to another
  desk's). Supplying the identity **inline on the commit** with `-c user.name=… -c user.email=…` is immune
  to that race by construction — it cannot be overwritten, because it is not persisted. Always pass it on
  this local-commit path. The session worktree ALSO carries a worktree-SCOPED identity
  (`git config --worktree`, immune to the shared-config race) as **defense in depth** for any commit path
  that forgets the inline override — but the inline `-c` is what you rely on. (The push-race loop —
  `commit → pull --rebase → push`, retry on race — is unchanged; only the identity/transport is
  identity-scoped.)

## Rules (inherited)

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
  **Labeling is not a substitute for landing** — see the write-first rule: land what you have, *then*
  label what you need.
- **Evidence-not-claims, applied hardest here** — the verifier's report is itself a claim; the value is
  the recorded command output, and the runner must be attributable and ≠ author. A verifier that
  self-verifies its own implementation is void.
- **Non-implementer isolation:** own temp worktree; never mutate the shared checkout; never `git
  restore`/`clean` a shared checkout.
- **No status document on a branch** (single-writer is main's CI).
- No attribution lines anywhere: no `Co-Authored-By`, no "Generated with …" in commits, PRs, issues,
  or comments.
- **Git push policy (ONE policy, role-keyed):** MERGE IS ALWAYS the driver's, and nobody triggers
  workflows or runs mutating cluster commands without their go. **Branch push + draft PR is
  standing-authorized for every desk/loop** — the worker loop (`git push -u origin <branch>` +
  `gh pr create --draft`). **The verify desk lands its own work**: its Evidence + status flips commit
  straight to `main` as the project directs — no push-go is needed there and none should be waited
  for. Any `main` push not covered by a standing authorization is gated on the driver's explicit go;
  committing local work is always fine. A guard/hook-BLOCKED push is a STOP signal — never route the
  same write through another tool. Each desk's own grants and denials (what it may flip, file, close,
  or land) stay in its skill, directly below this block.
  - Desk-specific: Evidence + status flips are committed **straight to main**, as they arrive, by this
    desk — **authored + pushed as the verify identity** (see "Post as the verify write identity" above),
    never plain `gh` / the shared account.
  - The operating pattern is the **push-race loop**: `commit → pull --rebase → push`, retry on race.
  - Branch push is not used here (this desk works on main); never commit the status document on a branch
    (single-writer = main's CI).
  - The only landing this desk does NOT do itself is an `irreversible: yes` status flip — that waits on the
    verify-gate issue the human closes; the verify-gate-close workflow is the sole `human:<name>` writer
    (see §Irreversible briefs). **Everything else, the desk lands.** `gate: model, irreversible: no` means
    *the desk flips it, no human involved* — read the brief's own fields and act on them; never invent a
    gate the brief does not carry.
- **WRITE FIRST; a "question" is a filed tracker artifact, not a halt** (see Autonomy — asking is the rare
  exception, and it never stops the drain). A parked question must never strand completed work. If you have
  a verdict in hand, **land the artifact — Evidence, status flip, filed issue, or a written note — BEFORE
  you ask anything.** This is what makes a question survivable:
  - The artifact is what makes the question *answerable later*. A question asked over unwritten work means
    that if the answer never comes (the human's asleep, the window closes, context ends), the work is gone
    — not delayed, **gone**. In one incident: parked "Want me to (a)… (b)…?" questions, an afternoon of
    correct verification, `Edit`/`Write`/`commit` calls: **0**. Nothing was recoverable from git because
    nothing was ever written.
  - Uncertain which of two things to do? **Do the reversible one and say so.** A landed Evidence row you
    later amend costs one commit; an unlanded verdict costs the whole afternoon.
  - Genuinely blocked and unable to land anything? Then the artifact **is** the question: label it
    `question` / `help wanted` per the escalation rule, with the evidence in the comment. A question in the
    transcript is not durable; a question on the tracker is.
- **Does NOT run the PR monitor** (that is pr-review-desk). This window's signal is the Awaiting queue,
  polled each boot / after each merge wave.

## Metrics this loop feeds (DORA, for the retro)

Each pass produces retro-grade data — log it: **lead time** (merged→verified age per brief), **change
failure rate** (VERIFY: FAIL ÷ verified), **rework rate** (follow-up briefs/bugs spawned by a failed
verify). These are the stability half DORA says most teams are blind to; the verify desk is where they
get measured.
