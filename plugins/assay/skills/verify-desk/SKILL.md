---
name: verify-desk
description: Run the post-merge VERIFY role of the process desk — the standing window that drains the "Awaiting verification / review" queue. Merging a brief-PR does NOT complete it; a NON-implementer must run the brief's Verify table on merged main, fill Evidence, and advance implemented-to-verified-to-done. Use when starting/resuming the verify window, when asked to "run the verifier loop / drain the awaiting queue / verify merged briefs / turn merged into done", or when the coordinator delegates the verify half. Mirrors pr-review-desk but POST-merge. Role window, no persona (Bob belongs to the-desk only); driver human:<name>.
---

# Verify Desk

The **post-merge verify half** of the four-role pipeline: worker-desk implements behind draft PRs →
pr-review-desk reviews and flips ready → **human:<name> merges** → **verify-desk** (this skill, its own
window) runs each merged brief's Verify table on merged main as a NON-implementer, fills Evidence,
advances `implemented → verified → done`. Merging is deployment frequency, not completion, and an
unwatched Awaiting queue is how briefs rot at `implemented`; this loop is the **Change Lead Time** fix
and the **Change Failure Rate** sensor (`verify-outcomes.jsonl` is its input). It does not run the PR
monitor — that is pr-review-desk's.

**The stream board is a derived, generated surface** (`docs/streams/derived-board/spec.md`) — this
desk's deliverable is the witness (the Evidence row, the verifyrun log, the App approval); it never
hand-edits a board cell, and the board follows the witness.

> Bindings for your harness — which mechanism each `capability:*` names — are in
> `../../references/<harness>.md`.

**House rules live in the repo's own house-rules doc (`CLAUDE.md`)** — git/PR discipline, identity and
posting, trust gate, filing and escalation, refresh-don't-remember, board hygiene, the console
noise-floor pointer, and worktree-sprawl ownership (the `deskwt` prune supervisor). This skill points at
them; it never restates them.

**Autonomy.** FILE, don't ask: a FAIL, a defect, a stale fact, an out-of-scope discovery is filed
immediately and the drain continues. DRAIN, don't wait: the Awaiting queue IS your direction; the only
things this desk cannot close (`irreversible` / `gate: human`) are async — land the Evidence, surface the
wait, take the next brief. An escalation is a filed artifact you keep moving past, and it goes to
**human:<name>**, never to a bigger model.

## HARD GATE — never claim "idle / caught up" without a fresh sweep

An idle claim is a claim about the Awaiting queue, and the only evidence about that queue is a fresh
sweep: before reporting "idle", "caught up", or "nothing awaiting", it is a HARD PRECONDITION that
`verifyloop plan` has *just* run and printed an empty queue. **"My dispatched verifiers finished" is
NOT evidence** — a verifier completing speaks only to the brief you already dispatched, never to briefs
merged since; that conflation is a silent outage reported as an all-clear. **A `plan` run stopped
by a red preflight (exit 6) is `could-not-check` — blind, NEVER an empty queue**, as is any errored
run: fix the check it names, re-run, then claim. An open verify-gate wait is a wait state, not idle.

## Boot

1. `WT=$(deskwt role-init --role verifier)` — session-scoped locked worktree on a branch tracking
   `origin/main`, with the worktree-scoped verifier-App bot USER identity. Idempotent: reuses
   your own valid tree instead of registering another. Never hand-roll `git worktree add`; `deskwt`
   missing is a preflight-class failure.
2. `deskboot verify-desk`, from `$WT` — loop identity, prune, lock, roster registration, the five-check
   operating-envelope preflight, the token mint, and the read-only board fetch in one verb. **A red step
   is `could-not-run` for the WHOLE pass**: report the one summary line and STOP — claim nothing, and
   file no issue about the desk's own envelope (each failing check names the issue that owns it). A
   probe REJECTION is a STOP; never retry under another identity.
3. Resync before every wave, scoped to your worktree, identity guard FIRST — a mismatched origin is a
   hard STOP, not a re-point, and a **bare** `git reset --hard` from outside your own tree wipes another
   session's work (what the F-34 writeguard blocks; re-issue with `git -C "$WT"`, never bare):
   ```
   [ "$(git -C "$WT" remote get-url origin)" = "$(git -C <repo> remote get-url origin)" ] || { echo STOP; exit 1; }
   git -C "$WT" fetch origin && git -C "$WT" reset --hard origin/main
   ```

## The loop

1. **`verifyloop plan --root <repo>`** from `$WT`: the deterministic scheduler prints the Awaiting
   queue, each item's tier, and the exact dispatch instruction (or human-route note). It spawns nothing
   and writes nothing. **Do not hand-compute the queue** — the plan IS the worklist.
2. **Dispatch what it prints**, one verifier per item, via `deskdispatch <item-key> --kit verifier`
   (claim → worktree → roster → decision gate → model stamp → prompt). Tier 1 (`implemented`, empty
   Evidence) before Tier 2 (`verified → done` closes), oldest-first within a tier; Tier 2 is never
   hidden — a filtered-out free close becomes permanent debt.
3. **Land each verdict as it returns** via `deskevidence` (below) — never a wave buffered to the end.
   **How many verifiers may be in flight at once is `deskroster width --role verify-desk`, re-read
   every tick.** This desk's declared default is a SEQUENTIAL drain (width 1), which is what it has
   always done; the width exists so the coordinator can widen it when `deskboard throughput` names
   verify as the bottleneck, without this body carrying a number that could drift from the tools.
   Narrowing never stops a verifier mid-pass — stop dispatching and let the pool converge as
   verdicts land. A width that cannot be read is could-not-check: hold at the last-read number.
4. Repeat as a CONTINUOUS drain, reporting incrementally. `verifyloop verdict` is the
   deterministic-runner half; filing its signed payload is the autonomous cutover, `gate: human`.

**Sibling repos are in scope** (human:<name>, 2026-07-10, F-23): a brief whose deliverables land
cross-repo is verified in the sibling checkout — read the set from `deskroster repos`, never a
hardcoded list; an uncloned repo is **could-not-check** for that row, never a fail. Resync the
sibling, run its rows there, record sibling repo + SHA in Evidence beside the in-repo row.

## The verifier dispatch — the moat

- **ALWAYS dispatch; NEVER verify inline; never the implementer.** An inline verdict lives in session
  context and dies with it (an afternoon of correct verification, `capability:dispatch-worker`
  calls **0**, zero artifacts). A dispatched verifier returns a **written** verdict that outlives your
  context even if you never act on it. "It's only a one-liner, I'll just run it" is the failure mode,
  not an exception to it. Verifier ≠ author — a fresh agent, always (brief-16 attribution).
- **Isolation** (`capability:isolate-workspace`): its own TEMPORARY worktree under `/private/tmp` off
  `origin/main` at the merged head, never the shared checkout, removed when the pass ends. PRs via
  `deskpr`, replies via `deskreply` — never a raw forge call.
- **Evidence is `command → exit code → real observed output`**, one row per Verify item, dated and
  runner-attributed, never a bare ✓ and never a claim. A row that cannot run is recorded EXPLICITLY
  unrun with its reason — never silently skipped, never assumed-pass.
- **Tier: the LOCAL SESSION MODEL. Never a stronger external/paid tier** (human:<name>, 2026-07-15 —
  overrides any `opus+` default in an older copy). A risk-clear brief (gate `model`, all risk answers
  `no`) is the normal path and most of the queue. A **risk-flagged** brief (`gate: human` or any `yes`)
  may have its Verify table RUN for the Evidence but **cannot be signed off by a model** — route it to
  the human gate. Read each brief's own frontmatter; never default the queue to one treatment.
- **The prompt is a KIT, not prose written here.** `deskdispatch --kit verifier` emits the common-clauses
  kit (isolation floor, no-evasion, offline envelope, three-state instruments, escalate-durably) plus the
  verifier kit — Evidence format, run-every-row, and the **risk-bearing value ENUMERATE → rank → derive**
  procedure, verbatim. Declared source, in this package's tool home and embedded in the binary
  (`deskdispatch --kits`): `tools/desk/cmd/deskdispatch/references/verifier-prompt.md` and
  `common-clauses.md`. Never paraphrase a kit clause at dispatch time — the wording IS the fix.

## Risk-bearing value — desk-side routing

The gate essence, so the desk can read a verdict: a green Verify table proves the code matches the pinned
value, not that the value is right — **green test ≠ correct constant** — and **naming a property,
invariant, or guard is NOT naming a risk-bearing value**; the entry is the literal *inside* the guard, at
a `file:line`. The trigger is **fail-safe**: `irreversible: yes`, OR **no `risk:` block at all** (an
ABSENT field is not a `no` — reading it as one is the F-28 failure shape itself), OR a risk-classed path
in the diff (the repo's own list — `auth/`, `billing/`, `deploy/` and the like), OR a value CLAUDE.md
pins as a hard constraint wherever it lives. Recover an already-merged diff from the PR number in the
stream-README notes cell or the Evidence (`gh pr diff <N>`); none recorded → `git log --oneline -- <brief
path>`, then `git show`. Every verdict returns a `RISK-VALUE:` line, and the desk acts on it **before any
status flip**:

- `NAMED, NOT DERIVED` + **`irreversible: yes`** → value, `file:line`, and the missing derivation go
  **verbatim into the Evidence on the brief**; the verify-gate issue lifts Evidence into its body, so the
  open question reaches the human on the sign-off card. That issue IS the artifact.
- `NAMED, NOT DERIVED` + **`gate: model, irreversible: no`** → **file a `question` issue under the
  verifier App FIRST** (brief, value, `file:line`, derivation needed), **link it in the Evidence row,
  then flip**. The brief still advances — never invent a gate it does not carry — but the un-interrogated
  number carries a durable pointer instead of vanishing into a green row. **A flag with no artifact is
  the F-28 failure shape wearing the fix's clothes.**
- `DERIVED` / `N/A` → record in Evidence and proceed; no issue.

## On VERIFY: FAIL — a Change Failure, filed not buried

The brief does NOT advance. File a `bug` immediately (`deskfile new -R <owner/repo> --raised-by verifier
--label bug`) with the failing command and its real output, then **continue the drain** — the filed issue
IS the report. **In addition** append one row to the append-only sidecar
`docs/streams/verify-outcomes.jsonl` (single-writer = this desk; the `VERIFY FAIL` commit-subject
convention is grep-fragile, so bounce-back rate is not computable from prose). On PASS append the same
row with `"outcome":"verified"`, so the denominator is complete.

```
{"ts":"<ISO8601Z>","brief":"<stream>/<NN>","outcome":"verify-fail","rows_passed":<n>,"rows_total":<N>,"sha":"<merged-head-sha>"}
```

## Landing — `deskevidence` is the SOLE main-push carve-out (narrow, dated)

**The whole fleet is branch + draft PR; push-to-main and merge are human-gated. The one exception, and it
is this desk's alone: `deskevidence` Evidence-row landings and the status flips that accompany them
commit straight to `main` as the verifier App — human:<name>'s authorization of 2026-07-09, restated
2026-08-24 (audit ruling 3).** Nothing else, nobody else: not another verb, not another file
class, not another desk, not a wider branch grant.

Interface: `deskevidence --help` — positional `<owner/repo> <branch>`, `--evidence-file` required (plus
`--brief-path` to merge a row into a brief's Evidence section), ONE file per invocation, and it **mints
its own verifier App token in-process and does NOT read `GH_TOKEN`** (prefixing it with a token `cat`
only misleads). Set `VERIFIER_MAIN_OK=1`.

**A sanctioned channel is not a way around a blocked write.** `deskevidence` is where the real guards
live and fire — `VERIFIER_MAIN_OK`, the repo allowlist, the BodyCheck secret/impersonation scan, the
outward-write rate limit, the post-commit attribution check (author = the App's bot USER id), the
audit line — so use it *because* it enforces those. A guard- or classifier-BLOCKED `git push` is a
STOP-and-escalate; never route the same write through another tool to get past a block.

**Land as each verdict arrives.** A PASS in hand and not on main within one landing cycle is a defect:
the board shows phantom verification debt, other sessions re-report "stuck" briefs, and merged→verified
lead time inflates by pure reporting latency. The Age column measures exactly that. Everything else this
desk posts goes out under the role App via the desk verbs (`deskpost` / `deskpr` / `deskreply` /
`deskfile --raised-by verifier`), minted with `desktoken verifier` — never a bare forge call.
**Standing-doctrine pointer (2026-08-17):** the verdict-transcription lane (`docs/streams/verdict-lane/`,
ruling R-6) would replace this path with signed verdict issues and a main-side sole writer; until R-6 is
SIGNED and the lane armed (cutover = verdict-lane/06), this section stands unchanged.

## Irreversible briefs (`risk.irreversible: yes`) — the model records Evidence, a HUMAN flips

The broad gate blocks any `irreversible: yes` brief from `verified`/`done` unless Reviewed names a
`human:<name>`. Because this desk lands status straight to main, flipping one on a model verify would
fail `--lint` and redden main CI directly. So the model path STOPS short of the flip:

1. **Still dispatch the non-implementer verifier and run the full Verify table** — the mechanical
   verification is real and worth recording. **Write the Evidence rows; leave the status at
   `implemented`** (allowed — the gate bars only the `verified` cell). Check `risk.irreversible` BEFORE
   flipping any status, and never stamp a model runner in Reviewed: a model cannot close, or pre-close,
   an unfixable change.
2. **Never open a human-review checkpoint PR.** A Verified/Reviewed cell gaining a `human:<name>` stamp
   on ANY branch is a hard `--lint` PROBLEM: the sole permitted writer is `verify-gate-close.yml`, which
   writes straight to main where the branch-vs-main guard is disarmed (methodology-metrics/43); four
   checkpoint PRs opened under the stale doctrine were dead on arrival.
3. **The sanctioned mechanism.** With Evidence landed per step 1 — including the verifier's full
   `RISK-VALUE:` line(s), a `NAMED, NOT DERIVED` verdict **verbatim and called out as the open question
   the human is being asked to settle** (burying it is the F-28 miss) — `statusgen --verify-issues` files
   a verify-gate issue lifting the Verify table + Evidence into its body as the review packet, and **the
   allowlisted human closing that issue IS the review**: `verify-gate-close.yml` writes the `human:<name>`
   stamp to main and advances `implemented → verified → done` in one step. Surface the open wait in the
   report. Where that wiring is not deployed, the brief stays at `implemented` with Evidence — a
   documented wait state, never a checkpoint PR.
4. `gate: human` is a hint; the authoritative trigger is **`irreversible: yes` alone** (human:<name>'s
   broad-scope call) — `regulatory` / `customer` do not change it.

## `gate: model` verified→done — CI owns the flip; this desk WATCHES

**The desk never flips a `gate: model` row** (audit ruling 4). Main CI does: the auto-flip job
on every push to main runs `statusgen --root . --auto-flip-model`, reads the reviewer App's APPROVED
review object on the PR that merged the work, requires it at that PR's **merged head SHA**, then stamps
Reviewed (date + App + PR# + SHA) and flips the row to `done` in one write.

A `gate: model` row still at `verified` after a main-CI run — or a verify-gate issue closed with no flip
landed — is a **STUCK row**, and stuck rows ARE this desk's work: read that run's `REFUSED` /
`COULD-NOT-CHECK` line and **file it**. Nothing flips on a "probably"; a stuck row is a finding to
investigate, never a stamp to write by hand over the automation. The desk's remaining done-close work is
the `gate: human` half — route it to the verify-gate issue.

## Cluster rows — the offline→online hand-off (a `check` row this desk hands off, never a dead end)

A **cluster row** — a `check` row that runs a documented probe script (`kubectl exec …`) against live
in-cluster infrastructure — has a runner that MUST be the privileged **admin pod** (the sole holder of the
cluster's admin credential; a raw `curl` / direct token chain is prohibited even read-only). This desk
runs OFFLINE by contract (the offline envelope, common-clauses kit, `KUBECONFIG=/dev/null`), so it **can
never run a cluster row itself**. Left as-is, a brief whose local rows all PASS but which carries one
cluster row rots at `implemented` — one live confirmation from `verified`, unreachable by any offline
verifier. A cluster row is therefore a **hand-off to the online/pod verify lane** (`docs/streams/verdict-lane/`,
brief `verdict-lane/07`, human:<name>'s Option A ruling), **not a permanent park.**

On a `gate: model` brief whose non-cluster Verify rows all PASS and whose only unrun rows are cluster rows
(the cluster row class, `verdict-lane/08`), the offline desk:

1. **Lands the passing-row Evidence** via `deskevidence` — the offline half of the verification is real
   and worth recording.
2. **Records each cluster row `could-not-check` with the EXACT probe command** to run — never a bare
   "cluster". That stable, greppable marker is what the online runner's worklist (the `verdict-lane/08`
   cluster-pending queue) keys on.
3. **Leaves the status at `implemented`.** The online/pod verify runner (`verdict-lane/09`), resident in
   the admin pod, owns the cluster-row Evidence **and** the completion flip.

This desk **NEVER runs the cluster row itself** (the offline envelope) and **NEVER flips a brief over a
could-not-check cluster row** — a could-not-check is not a pass (the three-state instrument), and signing
off a cluster row is the online lane's alone, never this desk's. This is already what a careful verifier
does; writing it as a contract keeps the two halves of the lane meeting instead of drifting.

## Live / billed / externally-authenticated probe rows — a Phase-0 implementer obligation, not a Verify gate

A **live / billed / externally-authenticated probe row** — a Verify row reproducible **only by the
implementer**, because it needs an external API key, meters real billed spend, or opens a paid/subscription
session (the triggering case: a live Anthropic ACP session — adapter negotiation, metered cost, negotiated
params). Unlike a cluster row it has **no online hand-off lane**: no second non-implementer runner holds the
credential or can be charged the spend. Left under the plain Verify contract (a non-implementer re-runs
every row) such a brief rots at `implemented` forever and needs a bespoke human ruling — the class that
stranded loop-engine/14 at `implemented` and recurs across desk-console-saas/04-05, desk-console-2/01,
desk-apps/04.

human:<name> ruled (2026-08-27) that this class is handled by **Option 2**:
the probe is a **Phase-0 implementer obligation**, recorded as Evidence **at implementation time** (adapter
version, exit code, metered cost, negotiated params) — it is **NOT** a non-implementer Verify-table gate.
The offline verify contract is unchanged for every OTHER row (the offline surface — build/test/grep + the
derived risk-bearing values — is re-run as always). For the probe row the non-implementer's job is narrowed:

1. **Never re-run the billed probe.** The offline envelope forbids it anyway (no external/billed endpoint,
   `KUBECONFIG=/dev/null`), and a second run would meter a **second real charge**. Re-running is not the
   verification here.
2. **Confirm the implementer's Phase-0 record EXISTS and is well-formed** — the recorded adapter version,
   exit code, metered cost and negotiated params are present, internally consistent, and pinned to the
   merged head. Record THAT confirmation as the row's Evidence (`command → what was read → PASS/FAIL on
   presence + well-formedness`), never a re-run and never a bare ✓. A missing or malformed Phase-0 record
   is a **FAIL**, filed like any Change Failure — it does not silently pass.
3. **The vouch-by-record boundary.** The implementer's recorded result is **single-sourced by construction**
   (only they can produce it), so this contract is a **vouch-by-record** — acceptable **only** for the
   **reversible, non-risk-bearing** billed surface. A **risk-bearing** live probe (a risk-classed path,
   `irreversible: yes`, or a value CLAUDE.md pins) still owes the full derivation and routes through the
   risk-bearing-value / verify-gate path above — a model never signs off a risk-bearing probe on the
   implementer's word alone.

**The author-side + tooling half lives in the public spec, not here.** For authors to mark such rows at
authoring time, and for statusgen/lint to distinguish a Phase-0-implementer row from a non-implementer
Verify row (a marker so lint does not demand non-implementer Evidence for it — cf. the `‡`/UNRUN
derivation), the brief-authoring contract (`assay/spec/brief-v1.md`) and `statusgen` — both de-housed to
public `medici-finance/assay` — carry the companion change; that half is tracked separately. This section is the
verify-side contract only. Until the marker lands, record the Phase-0-record confirmation as the row's
Evidence per step 2 and surface any brief that cannot advance without it — never a re-run, never a bespoke
human ruling re-derived from scratch each time.

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
- **File-and-exit, never block** (desk-hardening/13): after filing, the run does not hold — an open
  verify-gate wait is surfaced and the run moves past it. A loop that blocks in-run is undebuggable in a
  pod; its blocked state must be an at-rest filed issue anyone can inspect.
- **WRITE FIRST — a question is a filed artifact, not a halt.** With a verdict in hand, land it before
  asking: a question over unwritten work means that if the answer never comes the work is **gone**, not
  delayed (4 parked questions, write/commit calls **0**). Unsure between two actions? Do the
  reversible one and say so. Unable to land at all? Then the artifact IS the question.
- **Evidence-not-claims, applied hardest here** — the verifier's report is itself a claim; the value is
  the recorded output, and the runner must be attributable and ≠ author. A self-verify is void. Own temp
  worktree; never mutate the shared checkout; never `git restore`/`clean`; no STATUS.md on a branch.
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
  - Desk-specific: the grant is exactly the `deskevidence` carve-out above, nothing wider. The two
    landings this desk does NOT do are the `irreversible: yes` flip (the verify-gate issue the human
    closes) and the `gate: model` verified→done flip (CI's). Everything else it lands, as it arrives, via
    the push race loop (`commit → pull --rebase → push`, retry on race). It does not run the PR monitor.

### Stop-flag check — run at every iteration boundary

```bash
[ -f "$HOME/.config/assay/STOP" ] && echo "STOP flag active — exiting loop" && exit 0
[ -n "$DESK_LOOP" ] && [ -f "$HOME/.config/assay/STOP.$DESK_LOOP" ] && echo "STOP.$DESK_LOOP active — exiting loop" && exit 0
```

`DESK_LOOP` is set by `deskboot`. A hit means exit cleanly; never halt mid-action — a started outward
write always completes. Precedence: `DISABLED` (C-6) > `STOP` > `STOP.<name>`. `deskkit.Guard()` enforces
these independently: a loop that skips its own check is defanged — every outward verb refuses.

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
