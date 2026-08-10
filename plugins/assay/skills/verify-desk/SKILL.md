---
name: verify-desk
description: Run the post-merge VERIFY role of the process desk — the standing window that drains the "Awaiting verification / review" queue. Merging a brief-PR does NOT complete it; a NON-implementer must run the brief's Verify table on merged main, fill Evidence, and advance implemented-to-verified-to-done. Use when starting/resuming the verify window, when asked to "run the verifier loop / drain the awaiting queue / verify merged briefs / turn merged into done", or when the coordinator delegates the verify half. Mirrors pr-review-desk but POST-merge. Role window, no persona (Bob belongs to the-desk only); driver human:<name>.
---

# Verify Desk

The **post-merge verify half** of the four-role process-desk pipeline:

- **batch-fanout** → workers implement briefs, open draft PRs.
- **pr-review-desk** → pre-merge review; the reviewer App approves; desk flips ready; **human:<name> merges**.
- **verify-desk** (this skill, its own window) → post-merge: a NON-implementer runs each merged brief's
  Verify table on merged main, fills Evidence, advances `implemented → verified → done`.
- **the-desk** → coordinates.

**Why this role exists:** merging is `deployment frequency`; it is NOT completion. A merged brief sits at
`implemented`/`verified` until verified, and *an unwatched Awaiting queue is how briefs rot at
implemented* (the methodology's own warning). In DORA terms this loop is what fixes **Change Lead Time**
(implemented→done) and it doubles as the **Change Failure Rate** sensor — a Verify run on merged main that
FAILS is a change failure that review missed. Run it in its **own window**; it does NOT run the PR monitor
(that's pr-review-desk's).

**Single home.** This `medici-finance/assay-toolkit` copy (`.claude/skills/verify-desk/SKILL.md`) is the
**only** verify-desk skill body — its canonical home since assay-selfcontain/08 (the
oit copy was removed and it became a consumer). The user-level
`~/.claude/skills/verify-desk` is a thin pointer carrying deltas only; the earlier divergent fork (the
#541 afternoon — it contradicted this copy on whether the desk may land its own work) was **removed
2026-07-16** (brief-22 completion). This assay-toolkit file is authoritative, full stop. (This desk
resolves the skill from whichever checkout it boots into; the canonical text is here.)

> **TODO(assay-selfcontain/09) — the Go payload still lives in oit.** The
> `go run .claude/skills/verify-desk/mint-verifier-token.go` command below references a Go tool that
> this brief (assay-selfcontain/08) deliberately LEFT in the `oit`
> repo — it carries App/install-ID + `~/.config/adopter/*.pem` coupling (C4/C5 class) that
> assay-selfcontain/09 relocates and de-couples. Until then, run it from an oit
> checkout, where `.claude/skills/verify-desk/mint-verifier-token.go` resolves.

## Autonomy — this desk runs unattended: FILE, don't ask; DRAIN, don't wait

The whole point of this window is to turn merged → done **without hand-holding.** Two standing rules,
both of which OVERRIDE any "ask first" instinct:

- **FILE, don't ask.** When a verify run turns up anything worth an issue — a VERIFY FAIL, a defect, a
  stale fact, an out-of-scope discovery — **file the issue yourself, immediately, and keep going.**
  Filing a GitHub issue is durable, reversible, and low-cost; it **never needs human:<name>'s permission.** Asking
  "should I file this?" is the anti-pattern — it strands the finding in session context and halts the
  drain. Route by type WITHOUT stopping: a defect/failure → a `bug` issue (CLAUDE.md rule); a
  brief-invalidating fact → a FINDINGS entry; a systemic/process insight → an `assay-toolkit` issue
  (insight-routing); a call that is genuinely human:<name>'s → a `needs-decision`/`question` issue with the
  context. Then move to the next brief.
- **DRAIN, don't wait.** The Awaiting queue **is** your direction — you do not need fresh instruction
  between items. Work it to empty. The only things this desk legitimately cannot close are the
  **`irreversible` / `gate: human` sign-offs**, and those are handled **asynchronously** (the checkpoint
  PR below, or a labeled issue) — which does **not** halt the drain: open the PR / file the issue, move
  it to the "waiting on human:<name>" line of your report, and **pick up the next brief.** Never stop the whole
  loop to wait for one answer.

You escalate to **human:<name>**, never to a bigger model — but an escalation is a **filed artifact you keep
moving past** (an issue, a checkpoint PR, a report line), never a blocked prompt you sit on.
## HARD GATE — never claim "idle / caught up" on the Awaiting queue without a fresh statusgen sweep (#79)

**An idle claim is a claim about the Awaiting verification queue, and the only evidence about the queue is a fresh `statusgen` sweep.** Before the desk EVER reports "idle", "caught up", "Awaiting queue empty", or "nothing awaiting verification", it is a HARD PRECONDITION that it has *just* run `statusgen --root .` and confirmed the Awaiting verification / review table reports **zero rows — i.e. `awaiting == 0`** (the `awaiting` count from `debtCounts` in `statusgen/emit.go`). No fresh sweep → no idle claim. Full stop.

**"My dispatched verifiers finished" is NOT evidence the queue is empty.** They are different facts: a verifier completing tells you about the brief you already dispatched; it says NOTHING about briefs that were merged since your last sweep. Reporting idle from your own in-flight-verifier state — without a fresh statusgen sweep — is the exact failure mode of the #79 incident: it turned a silent monitor outage into a false "all clear." A subagent finishing re-invokes you; that is a cue to **sweep the Awaiting queue and advance the next item**, never a licence to report caught-up.

**A sweep that failed or errored is `could-not-check` — blind, not idle.** The instrument could not be read; re-sweep before making any idle claim.

This is the same defect class as desk-hardening/01: an instrument (the "Awaiting" count) reporting a state it never checked. The drive rule is in §Autonomy — DRAIN, don't wait, work the queue to empty — this section adds the gate: the desk may not claim idle without a fresh sweep confirming `awaiting == 0`. If you are waiting on a human sign-off (irreversible checkpoint), that is a documented wait state, not idle — report it and move to the next item. **Liveness backstop (M1):** the fixed-cadence `Monitor` that would run `statusgen --root .` on a timer (mirroring pr-review-desk step 4) is deferred to the observability service (oit #627/#651, `docs/streams/observability/`) — the desk is event-driven until it is live.


## Boot sequence

0. **Set the loop identity (brief 08):** `export DESK_LOOP=verify-desk` — the stop-flag
   system uses this to honour per-loop `STOP.verify-desk` flags. Run this once at boot and
   before every iteration.

0b. **Prune stale worktrees** (bounded growth; the bash sandbox + writeguard depend on it —
   worktree sprawl trips E2BIG and the #742 false-positives): `deskwt prune`
   (installed binary at /opt/desk-tools/bin/). It only removes tracked-clean, fully-merged
   worktrees; unmerged/dirty/unpushed (active work) are always left. One-shot at boot; the
   steady-state timer is the `deskwt prune --interval 30m` supervisor (launchd / k8s pod).

0c. **Register in the roster (desk-tools/09):** `DESK_SESSION=${CLAUDE_SESSION_ID:-verify-desk}
   deskroster set --role "verify-desk"` — self-declares this session so
   `deskroster list` can answer "who owns the verify loop" (out-of-git,
   `~/.claude/desk-tools/roster/`). Run once at boot. The roster keys one beacon per session
   name, so the identity must be per-session: prefer the real `$CLAUDE_SESSION_ID`, falling back
   to this role's own name — never `bob`, which belongs to the-desk (see the persona rule above).

1. **Set up a dedicated current-main worktree — never the shared checkout.** Verification runs against
   *merged main*, and main moves as PRs land, so the verify desk works from its own isolated checkout
   kept in sync with `origin/main`:
   ```
   git -C <repo> fetch origin
   git -C <repo> worktree add ../verify-desk-main origin/main --detach   # or the harness EnterWorktree
   git -C <repo> worktree lock --reason "verify-desk live session" ../verify-desk-main
   ```
   Enter it and treat it as the desk's working root for this session. The lock is the cooperative
   half of the prune liveness guard — prune never touches locked trees; unlock is automatic when
   the worktree is removed at session end.
2. **Keep it current with main — resync before every verification wave** (and whenever a merge lands),
   so you never verify stale main. **Scope the resync to your worktree with `git -C` — NEVER bare git:**
   ```
   git -C <verify-worktree> fetch origin && git -C <verify-worktree> reset --hard origin/main
   ```
   A **bare** `git reset --hard origin/main` runs against whatever the shell's cwd resolves to — and if
   that cwd is the shared checkout (or any session-homed worktree that isn't yours), it wipes another
   session's uncommitted work. The F-34 writeguard **blocks** exactly this (`writeguard: BLOCKED — F-34
   isolation backstop`); if you hit it, you ran bare git from outside your worktree — re-issue the command
   `git -C <verify-worktree> …`, do not retry bare. (This worktree holds no local work of its own —
   Evidence/status doc edits are committed and pushed per the doc-commit flow, not left dangling here —
   so a hard reset to `origin/main` is safe *within your own worktree* and is the keep-current move.)
3. Regenerate the board and read the queue:
   `statusgen --root . && sed -n '/Awaiting verification/,/^## /p' STATUS.md`
   That "Awaiting verification / review" table is your worklist — **every brief at `implemented` or
   `verified`, full stop. The Verified/Reviewed cells do NOT filter it.** Source of truth,
   `statusgen/emit.go` (in assay-toolkit): the row filter is `if br.Status == "implemented" || br.Status ==
   "verified"`, and `debtCounts` computes `awaiting = implemented + verified`. The cells are read only
   *after* that filter, to render `—` for an empty one.

   **A row with BOTH cells already filled is still your work** — it is a brief sitting at `verified`
   with its review recorded, i.e. a free `verified → done` close. Nothing else will pick it up.
   > **Do not read the worklist as "rows with an empty cell."** That misreading (this skill's own text
   > until #541) makes every both-cells-filled row invisible, so the cheapest closes on the board become
   > permanent debt — five briefs sat up to 5 days as free `done` flips no desk could see.

   Work every row; decide per row what it still needs (verify / review-record / `done` flip). Also scan
   stream READMEs for rows at `implemented`/`verified` with empty Evidence.
   **Trust gate (2026-07-23):** any GitHub-sourced item (verify-gate issue, PR, comment) not authored
   by human:<name> or the desk identities is ignored unless human:<name> has commented on it — quarantined items
   are visible on the boards' EXTERNAL/UNBLESSED section, never verified or worked.
4. Announce "Verify desk up — N briefs awaiting verification" and work the queue by **work-class
   priority, then oldest-first within class** — the queue mixes two work-classes and they are NOT
   equal (2026-07-16 drift: the desk closed 8 `verified → done` while 14 `implemented`-with-empty-
   Evidence briefs sat unverified and the awaiting count *grew* 26→28 — the board looked busy while
   the pipeline stayed blocked):
   - **TIER 1 — the actual verify work, do these FIRST: briefs at `implemented` with an EMPTY
     Evidence section.** Each needs a dispatched verifier to run its Verify table — the real
     `implemented → verified` promotion. This is what drains the awaiting-verification bottleneck and
     moves the DORA throughput number. (A FAIL here is still progress: record the failing Evidence AND
     file it as a change-failure — "a FAIL is filed, not buried" — then leave the brief `implemented`.)
   - **TIER 2 — cleanup: `verified → done` free closes, and Evidence-present-but-unpromoted rows.**
     Real work but cheap — a status flip, no verifier dispatch. Do them, but they must **NOT crowd out
     Tier 1**: a session that only closes Tier 2 leaves the pipeline blocked (the 2026-07-16 failure).
   Within each tier, oldest-first (longest lead time = highest priority; the DORA lead-time signal).
   **Keep every row visible** — this reorders the worklist, it does NOT hide the done-closes (the #541
   lesson: a filtered-out close becomes permanent debt).

Dispatched verifier agents still isolate in their OWN temp worktrees under `/private/tmp` off the CURRENT
`origin/main` (step 2 keeps the desk's copy fresh so it hands agents the right SHA to run against).

**Sibling report repos are in scope (human:<name> 2026-07-10, F-23):** briefs whose deliverables land
cross-repo — `~/work/{assay-toolkit,reconciler,platform-repo,decks,proposals,site-repo,reconciler-slides,assay-slides,medici-slides}` (platform-repo added assay-toolkit#81; assay-toolkit is the
survivor of the assay-tools merge) — are verified THERE: resync the sibling to its `origin/main`
(or its recorded deliverable SHA) before running its Verify rows, run the rows against the sibling
checkout, and record the sibling repo + SHA in Evidence alongside the in-repo row. Cross-repo PRs
in those repos pair with their in-repo status-flip PR — verify the pair, not just the flip.

### Stop-flag check (brief 08) — run at every iteration boundary

Before each loop cycle, check for active stop flags:

```bash
[ -f "$HOME/.claude/desk-tools/STOP" ] && echo "STOP flag active — exiting loop" && exit 0
[ -n "$DESK_LOOP" ] && [ -f "$HOME/.claude/desk-tools/STOP.$DESK_LOOP" ] && echo "STOP.$DESK_LOOP active — exiting loop" && exit 0
```

`DESK_LOOP` is set at step 0 of the boot sequence. A hit means exit cleanly (the loop can be
restarted by `rm <flag>` + re-arm). Never halt mid-action; a started outward write always completes.
Precedence: `DISABLED` (C-6) > `STOP` > `STOP.<name>`. The tool layer (`deskkit.Guard()`)
independently enforces these flags — a loop that skips its own check is defanged: every outward verb
will refuse.

### Hourly hygiene tick (ENFILE incident 2026-07-23)

At most once per hour during the loop, run
`../assay-toolkit/tools/prune-worktrees.sh --apply --include-scratch --min-idle 2h <oit-repo-root>`
— and the same for `../assay-toolkit` and `../reconciler` if present. Safe while other windows are
live: the tool HOLDs locked / recently-active / unmerged / dirty worktrees (sprawl exhausted the
system open-file table, 2026-07-23). Script missing (assay-toolkit#133 not yet merged/pulled) →
skip silently — never hand-delete worktrees.

## The loop (per awaiting brief)

1. **Pick a brief at `implemented` (needs verify) or `verified`-without-review (needs the review gate).**
2. **Dispatch a NON-implementer verifier — ALWAYS dispatch, NEVER verify inline.** An inline verdict
   lives in session context and dies with it: the desk holds a correct PASS, parks, and the work is
   unrecoverable (#541 — a full afternoon of real verification, zero artifacts, `Agent` calls: 0). A
   dispatched verifier returns a **written** verdict that outlives your context even if you never act
   on it. "It's only a one-liner, I'll just run it" is the failure mode, not an exception to it.
   Its prompt MUST carry:
   - the brief path + its **Verify table** (the exact commands) and the merged-main SHA to run against;
   - **isolation**: its own temp worktree under `/private/tmp` off `origin/main` (NEVER the shared
     checkout) — include VERBATIM the hard line "Your home worktree is <path> — every file operation
     stays under it" (the ~105 writeguard F-34 blocks were dispatched agents reaching outside their
     home); any PR a dispatched agent opens (e.g. a fix PR) goes via `deskpr`, and replies on
     its own PR via `deskreply`, not raw `gh` (checkpoint PRs are NOT a dispatched-agent artifact —
     the desk opens those itself under the App, per the irreversible carve-out below); for live-dev Verify rows, the `medici-admin` debug pod probes are allowlisted
     (`kubectl exec -n medici-dev-app deploy/medici-admin -- probe-canton.sh` etc.) and read-only
     `curl https://canton.dev.demo.example/...` GETs;
   - run EVERY Verify row and record **command → exit code → key output line** — real observed output,
     never a claim. A row that cannot run (no cluster / no toolchain) is recorded as explicitly unrun,
     not silently skipped or assumed-pass;
   - **Name-and-derive the risk-bearing value — ENUMERATE, then rank, then derive.** A green Verify table
     proves the code matches the pinned value; it does NOT prove the value is *right* — **green test ≠
     correct constant.** The trigger is **fail-safe** — it fires on ANY of:
     - `risk.irreversible: yes`; **OR**
     - the brief has **no `risk:` frontmatter at all** (no frontmatter, or frontmatter without a `risk:`
       block). **An ABSENT field is not a `no`** — reading it as one is the F-28 failure shape itself. 38
       of the 298 briefs under `docs/streams/*/brief-*.md` are pre-schema legacy with no frontmatter, and
       they are not dormant: all five `implemented` privacy-hardening briefs (04-08) are in this set and
       are auth/identity work — brief-07 removes `readAsAnyParty: true` from `k8s/{dev,prod}/identity.yaml`.
       One-line check: `awk '/^---$/{n++;next} n==1' <brief> | grep -q '^risk:' || echo TRIGGER-ON`; **OR**
     - the **risk-class diff fallback**: a diff touching `daml/`, ledger-service `auth`/`api`,
       `k8s/*/identity.yaml`, `k8s/*/canton/`; **OR**
     - the diff changes a value **CLAUDE.md pins as a hard constraint, wherever it lives** — e.g. the
       `settlementDelaySeconds` ∈ [30 floor, 60s interval) bound, which lives in agents/frontend and so
       sits outside every path above. The fallback is a *path* proxy for risk; this is the *content* proxy
       for the cases the paths miss.

     Recovering the diff for an ALREADY-MERGED brief (the fallback branch has no open PR): take the PR
     number from the stream-README notes cell or the brief's Evidence → `gh pr diff <N>`. No PR number
     recorded → `git log --oneline -- <brief path>` for the commits that touched it, then
     `git show <sha> --stat`.

     **Paste the block in "§ The risk-bearing value" below into the verifier's prompt VERBATIM** — it is
     the procedure, not a summary of one. The verdict comes back carrying a `RISK-VALUE:` line, and
     step 4 gates on which one;
   - **it must NOT be the brief's implementer** (brief-16 attribution: verifier ≠ author). Fresh agent.
   - report back the Evidence rows + a clear **VERIFY: PASS | FAIL** (+ FAIL detail).
   - **Tier: dispatch on the LOCAL SESSION MODEL. Never opus, never an external/paid tier** (human:<name>,
     2026-07-15 — a standing rule that overrides any `opus+` default still written in an older skill
     copy). The verify desk does not escalate to a bigger model; it escalates to **human:<name>**.
   - **The risk-keyed floor still applies, but its upper rung is a HUMAN, not a stronger model**
     (methodology/19 + the standing rule above): a **risk-clear** brief (all four risk answers `no`,
     gate `model`) is verified by a dispatched local-model verifier — that is the normal path, and it
     is the majority of the queue. A **risk-flagged** brief (`gate: human` OR any risk answer `yes`)
     may still have its Verify table *run* by a dispatched verifier for the Evidence, but it **cannot
     be signed off by a model** — route it to human:<name> (`irreversible: yes` → the checkpoint-PR path below).
     Check the brief's risk frontmatter; never default the whole queue to one treatment.
     > `statusgen` enforces this (`brieffile.go`, risk-keyed verifier floor): a risk-flagged brief at
     > `verified`/`done` needs a human or a non-cheap-tier runner in **Verified** (`brieffile.go:616`
     > reads `row.Verified`, not Reviewed — the irreversible band at L601-603 checks Reviewed, but the
     > risk-flagged non-irreversible band here checks Verified). Since this desk dispatches only the
     > local model, **for a risk-flagged brief that means a human — full stop.**
3. **On VERIFY: FAIL → this is a Change Failure.** The brief does NOT advance. **File a `bug` issue
   immediately — no permission needed (Autonomy: file, don't ask)** — with the failing command + output,
   then **continue the drain** (a FINDINGS entry or follow-up brief instead when that's the better route).
   The failure rate is a metric, don't bury it, and don't stop the loop to report it — the filed issue
   IS the report. A failed verify on already-merged code is exactly what the
   pre-merge review missed; note the class for the retro.
4. **On VERIFY: PASS → fill Evidence + advance the row — LAND IT AS ITS VERIFIER RETURNS**
   (methodology-metrics/17, #282). The desk (or a doc-committing agent) writes the
   Evidence section (one row per Verify item, `YYYY-MM-DD <runner>`, runner-attributed, NOT bare ✓) and
   flips the README status `implemented → verified`, Verified cell dated+attributed. This is a statusgen
   source edit — commit it per brief (per stream-group at most; small desk doc commit, retry the push
   race loop; never commit STATUS.md on a branch). **Anti-pattern: accumulating a wave's flips in a
   scratch buffer and landing them at wave-end** — a PASS in hand and not on main within one landing
   cycle is a defect: the board shows phantom verification debt, other sessions re-report "stuck"
   briefs, and merged→verified lead time inflates by pure reporting latency (land-as-you-go was
   already human:<name>'s 2026-07-09 operating direction). The Awaiting board's Age column measures exactly
   this latency — keep it honest.
   **EXCEPTION — `irreversible: yes` briefs do NOT get the status flip here; see "Irreversible briefs" below.**
   Check `risk.irreversible` in the brief frontmatter BEFORE flipping any status (a one-line `grep`).
   **ALSO check the verdict's `RISK-VALUE:` line before flipping.** A `NAMED, NOT DERIVED` verdict needs
   its `question` issue filed and linked FIRST — see "§ The risk-bearing value → Desk-side routing".
   `DERIVED` / `N/A` need no issue: record and proceed.
5. **`verified → done`** needs the recorded strong-model **review gate** (the pr-review-desk App-approval
   already covers the pre-merge review; for `done`, confirm the review verdict is recorded in the Reviewed
   cell — dated + attributed, `human:<name>` for `gate: human` briefs per brief-03). Only then `done`.
6. Repeat as a CONTINUOUS drain until the Awaiting queue is empty — results land per step 4 as they
   arrive (never held for an end-of-run landing); report progress incrementally + any Change Failures found.

## The risk-bearing value — enumerate → rank → derive → flag

**Why the enumeration is mandatory (the realised miss).** On `daml-hardening/05` a verifier voluntarily did
what a *"name the specific constant"* rule asks, and still missed the number. It named three risk-bearing
values (`docs/streams/daml-hardening/brief-05-settlement-invariants.md:92-93`): *"conservation (P+N payout ≤
collateral), notional-equality, and the `notional > 2` min-attestation threshold."* **None is
`conservationTolerance`** — the value F-28 exists because nobody interrogated
(`daml/OptionIndex/Core.daml:1808-1809`, guarding at `:437`, `:617`, `:722`). And the first entry *is the
property that the tolerance parameterises*: the verifier named the **guard** and stopped one level above the
**constant inside it** — and a property name reads exactly like a risk-bearing value in Evidence. That PASS
then survived an `opus-verifier` re-verify and **two human touches** (`2026-07-16 human:ian; accepted
2026-07-18 human:ian`). So this is **not** a cheap-tier gap: singular "name the constant" wording is
satisfiable without ever reaching a number, by anyone. Enumerate-then-rank surfaces the tolerance
mechanically, because it is a deliverable of the brief's own Task 2 — which removes the selection guess.

**Paste the following into the dispatched verifier's prompt VERBATIM.**

> **Risk-bearing value — do this BEFORE you return PASS.**
>
> 1. **ENUMERATE — do not pick.** List **every** literal constant, bound, threshold, tolerance, ratio,
>    timeout, limit, and authority binding that this brief's diff **introduces or changes**, plus every one
>    named in the brief's own Deliverables. This is mechanical and cheap; it is not a judgement call, and
>    you do not get to stop at the first one you find.
> 2. **Every entry must be a LITERAL with a source line**, in the form `<identifier> = <literal>` @
>    `<file>:<line>`. **Naming a property, invariant, guard, or check is NOT naming a risk-bearing value —
>    it is pointing at one from one level too high.** If what you wrote is a property ("conservation: P+N
>    payout ≤ collateral"), open the guard and name the **number inside it**
>    (`conservationTolerance = 0.0000000001 * (1.0 + abs notional)` @ `daml/OptionIndex/Core.daml:1809`).
>    An entry you cannot quote as a literal at a `file:line` is not an entry — resolve it down to the
>    literal, or drop it.
> 3. **RANK by irreversibility.** One line each: what breaks if it is wrong, and whether the breakage can
>    be undone by an edit + redeploy. Reversible operational knobs (WIP limits, alarm thresholds, retry
>    counts, log levels, poll intervals) rank last and need no derivation — *that is what `irreversible`
>    means*, and it is why they are out of scope by design rather than by oversight.
> 4. **DERIVE the top-ranked entries** — from first principles, the spec, the paper, or CLAUDE.md's stated
>    constraint. **"A test that exercises it passes" is not a derivation.** Record it in Evidence: the
>    value, its `file:line`, and *why that is the right value*.
> 5. **Return one verdict line per top-ranked entry**, in Evidence and in your report — exactly one of:
>    - `RISK-VALUE: DERIVED — <id> = <literal> @ <file>:<line> — <why it is right>`
>    - `RISK-VALUE: NAMED, NOT DERIVED — <id> = <literal> @ <file>:<line> — <what derivation is missing, and why you could not do it>`
>    - `RISK-VALUE: N/A — enumeration over <the diff scope you covered> found no literal; the irreversible
>      act here is <the org transfer / the spend / the domain purchase / the publish / …>`
>      — **N/A is valid ONLY when step 1 came back EMPTY.** A substantial minority of the 34
>      `irreversible: yes` briefs genuinely have no constant — at least these 8, checked:
>      `repo-migration-lending/02` is a GitHub org transfer; `assay-launch/06` and `assay-product/04` are
>      domain/USPTO spend; `methodology/09`, `/10`, `/11`, `/13`, `/34` are article publishes. This exit
>      exists so you never have to invent a value or write an unsanctioned
>      line. It is a **claim with the same standing as a derivation** — it is wrong if any literal exists,
>      so state what you enumerated over and make it checkable.
>
> **Calibration — this step is "enumerate and name", not "prove".** Enumeration is cheap and mandatory;
> full derivation is not always affordable — `daml-hardening/09` is an M-effort `gate: human` brief whose
> *entire* purpose is deriving one rounding bound, and it produced a 233-line analysis and 7 Verify rows.
> Nobody expects that as one bullet among five. So: **never skip step 1 to save effort, and never fabricate
> a derivation in step 4 to look complete.** `NAMED, NOT DERIVED` is an honest, sanctioned, useful verdict —
> it gets routed, not buried.

### Desk-side routing — a flag needs somewhere to land

"Routed to human:<name>" is not an artifact; **a question in the transcript is not durable, a question on GitHub is.**
On a verdict carrying `RISK-VALUE: NAMED, NOT DERIVED`, the desk acts **before any status flip, on BOTH
branches**:

- **`irreversible: yes`** → the value, its `file:line`, and the missing derivation go in the **checkpoint PR
  body** (below). The checkpoint PR IS the artifact; no separate issue.
- **`gate: model, irreversible: no`** — the fallback branch, e.g. `daml-hardening/12`: a DAML brief that
  fires the risk-class trigger and would otherwise flip to `verified` on main with no human in the loop →
  **file a `question`-labelled issue under the verifier App FIRST**, naming the brief, the value, its
  `file:line`, and the derivation needed; **link it in the Evidence row; then flip.** The brief still
  advances — the push policy stands and you never invent a gate the brief does not carry — but the
  un-interrogated number now carries a live, durable pointer instead of vanishing into a green row.
  **A flag with no artifact is the F-28 failure shape wearing the fix's clothes.**

`DERIVED` and `N/A` verdicts need no issue: record them in Evidence and proceed.

## Irreversible briefs (`risk.irreversible: yes`) — model verify records Evidence, a HUMAN flips the status

The broad irreversible gate (statusgen `brieffile.go`, needs-fixing-day2 / PR #159) blocks any
`irreversible: yes` brief from `verified`/`done` unless the Reviewed cell names a `human:<name>`. Because
this desk commits status **straight to main**, flipping such a brief to `verified` on a model verify would
fail `statusgen --lint` and **redden main CI directly** — the exact thing this desk must never do. So for an
`irreversible: yes` brief the model path STOPS short of the status flip:

1. **Still dispatch the non-implementer verifier and run the full Verify table** — the mechanical
   verification is real and worth recording. **Write the Evidence rows on the brief, but leave the status at
   `implemented`.** (Evidence on an `implemented` brief is allowed; the gate only bars the `verified` cell.)
2. **Do NOT flip the README status to `verified`, and do NOT stamp a model runner in Reviewed.** A model
   verifier cannot close — or pre-close — an unfixable change; only human:<name> can.
3. **Open a human-review checkpoint PR** (mirror #166 for on-ledger / #167 for the methodology batch): a
   small PR that flips the row to `verified` with `2026-… human:ian` in Reviewed, whose body is the review
   packet — what changed, the recorded Evidence, **the verifier's full `RISK-VALUE:` verdict line(s)** (per
   "§ The risk-bearing value" above: the enumerated literals with their `file:line`, the irreversible value
   human:<name> is signing off on, and why it is correct), and what to scrutinise. **A `NAMED, NOT DERIVED` verdict
   is carried into this body verbatim, called out as the open question** — it is what human:<name> is being asked to
   settle, and burying it here is the F-28 miss. human:<name>'s approve+merge IS the review and the status flip.
   Surface it in the Awaiting report so it's tracked, not lost.
4. A brief that is `gate: human` is a hint, but the authoritative trigger is **`irreversible: yes`**
   (the gate fires on it alone — human:<name>'s broad-scope call). `regulatory`/`customer` do not matter.

This keeps the desk's commit-straight-to-main speed for the reversible majority while never letting a model
sign-off close an unfixable change. Incident: daml-hardening/01 was marked `verified` by a glm verifier with
no human reviewer (needs-fixing-day2); the gate + this carve-out prevent the recurrence.

## Post as the App, always — `assay-verifier-app[bot]` (attribution, not authorization)

The verify desk has a **dedicated GitHub App** in the six-App **assay** desk-App family
(canonical record `../assay-toolkit/docs/streams/desk-apps/README.md`; provisioned 2026-07-18):

- **App**: `assay-verifier-app[bot]` · `VERIFIER_APP_ID=4331323` · installs `147393958`
  (the-org — the 3 core repos) / `147393973` (medici-finance — the report repos) · key
  `~/.config/adopter/verifier-app.pem` · source `~/.config/adopter/apps.env`.

The App is a **distinct, auditable actor** from the shared `the-org` account (which authors
every PR and which human:<name> also drives from a CLI). A verify-desk post under `the-org` is
attributable only to that shared account — not to this desk — which undermines the Change-Failure-
Rate / evidence audit trail this desk exists to produce (`apps.env`: the verifier App *"commits
Evidence, files verification-failure issues (may land main via ruleset)"*). This is **attribution
with an auditable trail, NOT an enforcement guarantee** — anyone who can read the PEM can mint the
token; the value is the trail (mirrors the reviewer App, assay-toolkit#37/#38). Incident that
prompted this rule: 2026-07-20 the desk filed verification-failure issue #900 via plain `gh`
(defaulted to the shared `the-org` keyring) — re-filed as #909 under the App, #900 closed with a
pointer (issue #910).

**Mint first, then post/commit.** `desktoken`/`deskkit` may not be on PATH; mint via the bundled
Go tool (parity with pr-review-desk's `mint-reviewer-token.go`):

```
go run .claude/skills/verify-desk/mint-verifier-token.go                 # the-org install  -> ~/.config/adopter/verifier-token
go run .claude/skills/verify-desk/mint-verifier-token.go medici-finance  # medici-finance org (arg 1) -> verifier-token-147393973
```

- **ALL posts go out under the App, NEVER plain `gh`/`the-org`.** Issues (verification-failure
  `bug`s, FINDINGS, `question`/`help wanted`), checkpoint PRs (irreversible sign-offs), PR/issue
  comments, and review replies — every one. Prefix **every** `gh` call with the App token; a bare
  `gh` defaults to the shared `the-org` keyring:
  ```
  GH_TOKEN="$(cat ~/.config/adopter/verifier-token)"           gh issue create -R example-org/<repo>      --body-file f.md
  GH_TOKEN="$(cat ~/.config/adopter/verifier-token-147393973)" gh issue comment <N> -R medici-finance/<repo> --body-file f.md
  ```
  If minting fails (missing key / revoked install), say so **in the artifact you were about to
  post** rather than silently falling back to a `the-org` post.
- **The Evidence/status commits to main are ALSO the App** (human:<name> 2026-07-20: "use the verifier app
  whenever possible"; `apps.env`: the verifier "commits Evidence… lands main via ruleset"). Author
  the commit as the App noreply identity and push over HTTPS with the App token:
  ```
  git -c user.name="assay-verifier-app[bot]" \
      -c user.email="4331323+assay-verifier-app[bot]@users.noreply.github.com" \
      commit -m "verify(<stream>/<NN>): <brief> Evidence + status"
  GH_TOKEN="$(cat ~/.config/adopter/verifier-token)" \
      git push https://x-access-token:$(cat ~/.config/adopter/verifier-token)@github.com/example-org/oit.git HEAD:main
  ```
  **Fall back to the the-org SSH push ONLY if a ruleset blocks the App push** — then the commit
  still authors as the App noreply email; only the transport changes. (The push race loop —
  `commit → pull --rebase → push` — is unchanged; only the identity/transport is App-scoped.)

## Rules (inherited)

- **Insight-routing (assay-toolkit#13):** a systemic/process insight produced in passing (drain notes,
  Evidence asides, "recurring enough to be worth a structural fix" observations) MUST also be filed as
  an issue in **medici-finance/assay-toolkit** — commentary is not a register. Include the triggering
  evidence and affected loops. Repo-specific defects still go to the repo's own tracker (issue-loop/05).
- **Escalation labels (human:<name> 2026-07-12): any desk/loop may label a PR or issue `question` (needs an
  answer from human:<name> or a stronger-tier model to proceed — the item PARKS awaiting input) or
  `help wanted` (the desk hit its capability/authority edge — a human or higher-tier model should
  weigh in on the work itself).** Both are GitHub default labels — they exist in every repo, no
  setup. Discipline: a bare label is unanswerable — the labeler MUST comment what it needs and from
  whom when labeling; whoever answers removes the label with their response. A `question` that
  matures into a formal decision fork promotes to `needs-decision` (issue-loop/06) with the
  pros/cons template — these two stay lightweight. Labeled items are WAITING-ON-INPUT: they join
  the human/escalation queue (I-28 panel, mm/11 ordering) and are NOT orphans for the
  batch-fanout sweep.
  **Labeling is not a substitute for landing** — see the write-first rule below: land what you have,
  *then* label what you need.
- **Evidence-not-claims, applied hardest here** — the verifier's report is itself a claim; the value is the
  recorded command output, and (brief-16) the runner must be attributable and ≠ author. A verifier that
  self-verifies its own implementation is void.
- Non-implementer isolation: own temp worktree; never mutate the shared checkout; never `git restore`/`clean`.
- No STATUS.md on a branch (single-writer is main's CI). No attribution lines in commits/PRs/issues.
- **Git push policy — THE DESK LANDS ITS OWN WORK. No push-go is needed and none should be waited for.**
  (Reconciled 2026-07-10; human:<name>'s direction 2026-07-09. If another copy of this skill says "pushing is
  gated" or "never push without human:<name>'s go", **that copy is wrong** — see the assay-toolkit Single home
  note, #541.) Concretely:
  - Evidence + status flips are committed **straight to main**, as they arrive, by this desk —
    **authored + pushed as the verifier App** (see "Post as the App, always" above), never plain `gh`/`the-org`.
  - The operating pattern is the **push race loop**: `commit → pull --rebase → push`, retry on race.
  - Never commit STATUS.md on a branch (single-writer = main's CI). Never trigger workflows or run
    mutating `kubectl`. **Merge is never yours**; branch push is not used here (this desk works on main).
  - The only landing this desk does NOT do itself is an `irreversible: yes` status flip — that is a
    checkpoint PR for human:<name> (see below). **Everything else, the desk lands.** `gate: model,
    irreversible: no` means *the desk flips it, no human involved* — read the brief's own fields and
    act on them; never invent a gate the brief does not carry.

- **WRITE FIRST; a "question" is a filed GitHub artifact, not a halt (see Autonomy — asking is the
  rare exception, and it never stops the drain). A parked question must never strand completed work.**
  If you have a verdict in hand, **land the artifact — Evidence, status flip, filed issue, or a written
  note — BEFORE you ask anything.** This is not a style preference; it is what makes a question survivable:
  - The artifact is what makes the question *answerable later*. A question asked over unwritten work
    means that if the answer never comes (human:<name>'s asleep, the window closes, context ends), the work is
    gone — not delayed, **gone**. #541: four parked "Want me to (a)… (b)…?" questions, an afternoon of
    correct verification, `Edit`/`Write`/`commit` calls: **0**. Nothing was recoverable from git
    because nothing was ever written.
  - Uncertain which of two things to do? **Do the reversible one and say so.** A landed Evidence row
    you later amend costs one commit; an unlanded verdict costs the whole afternoon.
  - Genuinely blocked and unable to land anything? Then the artifact **is** the question: label it
    `question` / `help wanted` per the escalation rule below, with the evidence in the comment. A
    question in the transcript is not a durable artifact; a question on GitHub is.
- Does NOT run the PR monitor (that's pr-review-desk). This window's signal is the Awaiting queue, polled
  each boot / after each merge wave.

## Metrics this loop feeds (DORA, for the retro)

Each pass produces retro-grade data — log it (candidate: `statusgen --trend`): **lead time** (merged→verified
age per brief), **change failure rate** (VERIFY: FAIL ÷ verified), **rework rate** (follow-up briefs/bugs
spawned by a failed verify). These are the stability half DORA says we're currently blind to; the verify
desk is where they get measured.
