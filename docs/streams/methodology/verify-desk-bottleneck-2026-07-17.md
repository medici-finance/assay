# Verify-desk bottleneck — diagnosis & recommendation package (2026-07-17)

**Author:** adversarial multi-agent review (8-dimension methodology review + 4-dimension verify-desk
diagnosis, each finding adversarially re-checked against repo/gh ground truth).
**Driver:** human:<name> — "figure out WTF is wrong with our verify desk, and how we can improve it. It's
currently the bottleneck."
**Status of this doc:** analysis + routing (precedent: `red-team-2026-07-09.md`,
`assay-growth-2026-07-09.md`). Actionable items are routed to existing briefs or filed as INTAKE
entries at the bottom; this doc does not itself change any gate.

---

## TL;DR

The verify desk **is** the pipeline's slowest stage — but not because verification is slow. Per-brief
service time is seconds-to-minutes; **queue residence is a median of ~37h and a tail of 5–8 days** —
roughly **13–14× the median PR-review latency** (adversarial re-check trimmed an earlier "30×" headline:
the review stage has its own long merge tail, so the *tails* are comparable; the median gap is the real
story). The gap is almost entirely **wait time**, and the wait has three distinct causes that the single
"N briefs awaiting" number hides:

1. **The desk only exists when a human boots it.** Every other lifecycle stage (status regen, review
   dispatch, verify-gate issues, daily harvest) is automated server-side; verification alone waits for
   human:<name> to open a window. Drains are therefore bursty — the queue climbs +5–9/day between boots and gets
   emptied in one-day spikes.
2. **The majority of the queue is not desk-actionable.** Paused-stream rows, risk/irreversible briefs
   that *already* have PASS Evidence and are waiting on human:<name>'s sign-off, and FAIL-awaiting-rework rows all
   count as "verify-desk debt" at full weight. The oldest rows are almost all in these classes.
3. **The lifecycle needs 2–3 touches per brief**, and the later touches (`verified → done`) are
   mechanically derivable from data already on GitHub — yet they batch for days.

The fix is **not** faster verifiers. It is: put the trigger server-side, teach the board to separate
"desk debt" from "waiting on human:<name>" from "paused", and automate the mechanically-derivable touches into
CI using the pattern the repo has **already proven** in `verify-gate-close.yml`.

The loop's *output* is good and must be preserved: Evidence is real command→exit→output, and post-merge
verification catches change failures pre-merge review missed (≥12 recorded `VERIFY: FAIL` cases across 8
streams — F-32 is far from unique). We are optimizing cadence and queue hygiene, not lowering the bar.

---

## The numbers (queue forensics, reconstructed from 306 STATUS.md revisions + gh)

| Metric | Value | Source |
|---|---|---|
| PR review latency (open→merge) | median **~2.8h** (full 07-15..17 window; a 1-day-narrower sample read 1.1h), max ~143h | gh |
| Awaiting-queue full residence (implemented→done), n=108–114 | median **~37h**, p90 ~86h, max **167h** | STATUS.md git time series |
| impl→verified flip alone, n=105 | median 8.1h, p90 19.3h, max 139h | STATUS.md git time series |
| Verify stage vs review stage | **~13–14× slower at the median** (tails are *comparable*, ~167h vs ~143h) | derived |
| Queue depth trend | 5 (07-08) → 69 peak (07-12) → 27 trough (07-13 drain) → **40 (07-17)** | STATUS.md git time series |
| Net deficit, steady state | **~+3/day** (arrivals ~9/day, departures bursty) | STATUS.md git time series |
| Desk duty cycle | **~15%** — 13 sessions / ~30 active hours over 8 days; idle gaps to 51.5h | 49 `verify(desk)` commits |
| Burst throughput when running | **~4–5 briefs/hr** (one commit flipped 47 verified→done) | commit `df7ea6b4` |

**Read:** a desk that ran ~2h/day at observed throughput would hold the queue near zero. The pipeline
does not need faster verification — **it needs the verification window to exist daily.**

---

## Diagnosis — what's actually wrong

### D1. No merge-side trigger (root cause). *Cross-corroborated by 3 of 4 investigators.*
Verification starts only when a human opens a desk window. Landing commits/day were 5, 7, 6, 10, 1, 0,
14 against arrivals of 19, 31, 18, 17, 7, 3, 5 — **4 of the last 7 days recorded zero departures**,
including the #541 day where a full afternoon of verification produced zero durable artifacts.
Meanwhile `status-regen.yml`, `verify-gate-open.yml`, and `daily-harvest.yml` prove every *other*
stage is already automated on the self-hosted runners. #627 (mac-sleep blindness) is why the trigger
must be **server-side**, not a laptop-resident monitor.

### D2. The queue conflates three owners' work. *Corroborated by queue-forensics + redesign-options + verification-integrity.*
Of the current ~40 rows: **7** are the *paused* example-poc stream (paused 07-16, commit `9f03dbe7`);
**~8–10** are risk/irreversible briefs with **PASS Evidence already recorded**, waiting on human:<name>'s
sign-off, not the desk (every 5–6-day-old row is in this class — e.g. daml-hardening/03/04/05 verified
+ `human:ian` since 07-16, still not flipped); **~4** are `VERIFY: FAIL` awaiting implementer rework;
**7** example-poc rows also collide with the `[exec:strong]` tier vs glm-5.2-only-verifier
contradiction (unresolved **F-16**). Only **~5** rows are genuinely unverified risk-clear briefs, most
< 2h old. **The headline "28–40 awaiting" systematically overstates desk failure** and hides that a
large majority of the aging tail is human:<name>'s gate and workers' rework, not desk throughput. *(Adversarial
re-check: the three-bucket decomposition and the paused/human-gate/rework classes are all confirmed on
origin/main; the exact per-bucket counts move a little snapshot-to-snapshot, so treat them as ±2.)*

### D3. The `verified → done` touch is pure bookkeeping that batches for days. *CONFIRMED (one sub-claim weakened).*
Commit `df7ea6b4` closed **47 briefs** `verified → done` in one commit purely by reading
`reviewer-app[bot]` approvals **already on GitHub** at merge time. For `gate:model` briefs the
Reviewed cell is derivable the moment the brief is verified, yet lh/14 waited 7 days and frontend/01 9
days for this mechanical flip. (Adversarial check knocked down the related claim that briefs get
*re-verified* at the done-flip as a duplication — that specific case, `2bee4849`, was a legitimate
re-run after an earlier FAIL. The core "second touch is derivable & batches" claim stands.)

### D4. A large minority of Verify rows are unrunnable by the dispatched verifier — and one environment defect blocks all of them.
Census of **968 Verify rows** on origin/main: **82 (~8.5%)** need live dev/kubectl/curl/devnet; **117
(12%)** need a sibling `../repo` checkout at a pinned SHA; **24** name a human. The dev ledger has had
**zero active contracts since ≥07-15** (broken governance bootstrap: CircuitBreaker schema drift +
HS256 submit block), so **every live row is currently unrunnable** — lh/23 burned 3 attempts across 3
days to NEEDS_CONTEXT. These briefs can never leave `implemented` and the desk keeps re-attempting them.

### D5. The risk-bearing row is systematically the one recorded UNRUN — and a `done` brief with UNRUN rows is board-indistinguishable from a fully-verified one. *(verification-integrity, HIGH)*
16 of 17 `done` ledger-hardening briefs — including the unauth read/write security trio and the
money-path idempotency brief — closed with their **live rows never run**. `statusgen` renders
`verified*` for unbacked rows but has **no** flag for UNRUN Evidence. This is the single most important
*integrity* (not throughput) finding: we are fastest at closing exactly the checks that matter least.

### D6. Dispatch overhead is ≥95% of an S-brief verification. *(work-anatomy, MEDIUM)*
`statusgen --lint` = 0.48s, `go test ./tools/statusgen` = 2.9s, worktree add < 1s. ~75% of all 968 rows
are seconds-cheap commands. Yet each brief gets a fresh verifier agent: temp worktree + full CLAUDE.md
(~2,600 words) re-read + written report + Evidence commit + push-race loop. The post-#541 "ALWAYS
dispatch" rule (correct for durability) applies this ~10–20-min ceremony even to 2-command briefs.

### D7. The skill is a 234-line accreted incident log. *(process-structure, redesign-options H)*
~50 lines explain the skill's own divergent user-level fork (#541, brief-22 stub authored but never
applied). It was patched 5 times in ~30 hours (07-15/16) because agents keep misexecuting it —
including a worklist misread that made every both-cells-filled row invisible for 5 days. A glm-5.2 boot
should read *rules*, not *history*.

---

## Recommendation package

Ordered by leverage-per-effort. Each maps to an existing brief/intake where one exists, or a **new**
INTAKE entry (filed below). Nothing here lowers the Evidence standard.

### This week (quick wins — agent-doable except where noted)

- **R1 — Segment the Awaiting board by blocker owner.** Split into `desk-actionable /
  awaiting-human-gate / awaiting-implementer-rework / paused-stream / env-blocked`, alarm each lane
  separately, and exclude paused + human-gated-with-PASS from the desk's oldest-first drain order. This
  single change makes the "40 awaiting" number mean something and stops the desk burning boot-time
  re-triaging rows it cannot move. *S statusgen change → **I-board-segment**.*
- **R2 — Auto-flip `verified → done` for `gate:model` briefs** by resolving the merge-PR's
  `reviewer-app[bot]` APPROVED review, recording PR# + head SHA in the Reviewed cell. Deletes the
  model-path second touch entirely; the machinery half-exists in `verify-gate-close.yml`. *→ **I-auto-flip-done**.*
- **R3 — Batch the human-gate sign-off into one daily digest, and surface its age.** *(Corrected after
  adversarial re-check — the original "the workflow never fires for irreversible briefs" premise was
  REFUTED: `verify-gate-open.yml` **does** fire — open verify-gate issues #583/#584/#331 for
  daml-hardening 03/04/05 and #332 for assay-product/04 "irreversible — human sign-off" prove it, and
  `verifyissues.go` has no irreversible exclusion.)* The surface exists; the lever is that it's **N
  scattered per-brief issues human:<name> must hunt**, with no oldest-age signal — so PASS-Evidence-recorded
  briefs still age 5–6 days at the human gate. Add a **single daily digest** (one issue/PR listing every
  brief awaiting human:<name>'s sign-off, oldest-first, with the recorded Evidence link) and a per-stream
  oldest-implemented-age metric, so the human gate's queue is as visible as the model gates'. Lower
  confidence than R1/R2 — verify the current per-brief issue latency first. *S → **I-signoff-digest**.*
- **R4 — Fix the dev bootstrap once** (CircuitBreaker schema drift + submit-capable token) — fully
  diagnosed already in lh/23 Evidence — to unblock all 82 live rows at once. *Route to a
  ledger/deploy brief; not a methodology change.*
- **R5 — human:<name> applies the brief-22 user-level skill stub** to `~/.claude/skills/verify-desk/SKILL.md`
  (5 min), then a diet pass moves #541/#282 narrative to `docs/` so the resident skill is rules-only.
  *human:<name>-gated (out-of-repo edit).*
- **R6 — Schedule the already-authored mm/11 (gate-queue priority) + mm/20 (approved-idle alarm)** into
  the next fanout batch, and add one skill line: dispatch verifiers for the whole risk-clear head of the
  queue **in one wave**, land per return. *No new authoring.*

### Structural (needs sequencing; R9 needs human:<name>'s policy sign-off)

- **R7 — Verify-row runnability tags** `[ci]/[cluster]/[sibling]/[human]` + a statusgen lint (untagged
  ⇒ `[desk]`). Prerequisite for all automation: today runnability is only discoverable by LLM-parsing
  prose. Natural extension of `../assay-toolkit/statusgen/verifyrows.go` (#509). Overlaps existing **I-22**
  (`verify-when: deployed`) and **I-63** (external exit-code evidence) — fold them in. *→ **I-verify-row-ci-tags**.*
- **R8 — Merge-triggered CI verification of the `[ci]` subset.** A push-to-main workflow enumerates
  newly-implemented briefs (via a **new** statusgen enumeration mode — `--awaiting --json` does not exist
  yet, it is part of this work), runs their `[ci]`-tagged rows on the runner, and commits Evidence
  attributed to `ci:actions (run <id>)` (statusgen must also learn to accept the `ci:` runner
  attribution). **The plumbing is proven in-repo**: runners already run `go`, `dpm` (`daml-ci.yml`), and
  `gh`; `verify-gate-close.yml` already implements statusgen-as-brain + 5-attempt push-race + idempotency
  guards — so this generalizes a working pattern rather than inventing one. This *strengthens* the
  non-implementer guarantee — CI is outside the model fleet, which is exactly the correlated-failure
  concern in I-17. *M → depends on R7 (the tag contract) landing first.*
- **R9 — Collapse to a single-touch close for risk-clear `[ci]`-only briefs** (`implemented →
  verified → done` in one CI commit). **Changes lifecycle-integrity semantics → human-gate per standing
  policy; must land WITH I-17 sampling QC** (strong-tier re-runs of a random 2–3 model/CI-verified
  briefs weekly) as the Goodhart counterweight. Do not ship R9 without R8+I-17 proven.
- **R10 — UNRUN as a first-class board state** (from D5): statusgen flags `done`/`verified` rows whose
  Evidence contains UNRUN, and every UNRUN risk-bearing row must be closed live or routed to a named
  follow-up before `done`. This is the integrity backstop that keeps the throughput work honest. *→
  **I-unrun-board-state**.*

---

## What must NOT change (preserve)

- Evidence format = command → exit code → key output, with runner attribution. It is what makes
  re-verification and human sign-off cheap, and it demonstrably catches real bugs (mm/22→#575,
  issue-loop/10→#712, assay-product/04→assay-toolkit#89).
- The `FAIL → issue` discipline (post-merge verification is the working change-failure-rate sensor).
- Always-dispatch (durability) — but change the *unit* (batch S briefs sharing a command set into one
  worktree/run; Evidence still per-brief). See R6.
- Do **not** re-propose RICE/WSJF human scoring (rejected, research §5.5 / I-13) or generic
  "more desk discipline" — the data says uptime and hygiene, not effort.

---

## Cross-reference: this is a *symptom* of two methodology-wide gaps

The broader review surfaced two roots the verify desk merely *exposes*, worth their own tracking:

- **No measured catch rate anywhere.** The review gate and the verify gate both gate hundreds of PRs on
  verdicts whose true defect-detection rate has never been tested. External SOTA (commercial AI
  reviewers, AI-control research) benchmarks with **seeded-defect drills**. Highest-leverage new idea:
  periodically float a draft PR seeded with N defects from this repo's own bug taxonomy (readAs leaks
  #428, settlement-authority, verify-math off-by-one) through the blind pipeline and record per-category
  catch rate. *→ **I-seeded-defect-drill**.*
- **Model monoculture + visible-test hazard.** Implementer, reviewer, and verifier all run glm-5.2, and
  the verifier re-runs a table the implementer authored and could optimise against — the textbook
  reward-hacking setup. Cheapest transplant: for irreversible/funds/auth/DAML briefs, require the
  reviewing/verifying model family to differ from the implementer's (the `via-*` multi-backend wrapper
  already exists), and require one **held-out check** not in the brief's Verify table. *→ **I-model-diversity**.*
