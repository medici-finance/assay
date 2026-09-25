---
brief: assay:assay:desk-containers:08
title: "A tick contract: one bounded pass when the harness says `--tick`, so a loop pod can finish"
why: >-
  The five desk-role skills are STANDING loops by design. Every one of their bodies says the
  same thing in its liveness contract — start the standing self-scheduled loop BEFORE the
  first sweep and "keep it ticking for the life of the window" — and none of them has any
  other mode. That is correct for a live window and fatal for a pod. A cell's loop image runs
  a role skill as a ONE-SHOT harness call under an outer `timeout`: the harness is given the
  role skill and its arguments, prints nothing until the turn completes, and the whole
  process is killed when the deadline expires. A standing loop under a one-shot call has only
  one possible ending — the kill. It was measured on 2026-09-14: the review loop's first
  successful boot on the fixed image ran the full 480 s tick deadline and was killed with no
  output at all, because the one-shot print mode emits only at completion and the completion
  never came. No review tick has ever finished. The loop is not slow and the image is not
  broken; the skill has no mode that terminates. This brief adds the missing mode as a
  CONTRACT stated once and cited by all five bodies: when the harness says `--tick` (or the
  environment says `ASSAY_TICK=1`), the role runs ONE bounded pass — boot, one fresh sweep,
  act up to its width, wait bounded for what it dispatched, print a fixed-grammar summary
  line, exit — and arms no durable wake, schedules no cadence, and asks no human anything.
  The standing-window behaviour is untouched.
wave: 3
depends: []
unblocks: ["desk-containers/12"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-14 by a desk-containers authoring session, from a live-cell measurement taken
  the same day and a same-day read of the five desk bodies and the loop-shaped references below
sources:
  - "Live-cell measurement, 2026-09-14: the review loop's first successful boot on the fixed image ran to the tick deadline and was killed; outcome recorded as `filed`, stdout empty. The harness's one-shot print mode emits only at turn completion, so a killed turn produces no output whatsoever — the run is indistinguishable from a hung one in the pod log."
  - "The harness contract, as given: the loop image invokes the role skill as a single non-interactive call with a text output format, an optional model pin and an optional tool allowlist, wrapped in `timeout $ASSAY_TICK_DEADLINE`. In the shipped CronJob that deadline is 480 s, under the pod's `activeDeadlineSeconds` of 540 s, under the App token's 1 h life. Those three numbers are taken as facts here; the image's own change to PASS `--tick` is follow-up in the image's repository and is deliberately not in this brief."
  - "freshness-checked 2026-09-14 @ `86e3a434` (origin/main) — every claim below re-read against the tree at that commit."
  - "The standing-loop text this brief does NOT change: `plugins/assay/skills/the-desk/SKILL.md` § 'Liveness contract (binding)' (line 318) and its byte-identical twins in `intake-desk` (434), `pr-review-desk` (641) and `verify-desk` (486), plus `worker-desk`'s near-twin (637). All five say the loop starts before the first sweep and keeps ticking for the life of the window. There is no second mode anywhere in the five files."
  - "The wake vocabulary the tick must NOT arm: `plugins/assay/references/claude-code.md` § `capability:durable-monitor` — 'a re-arming poll that survives across turns and re-invokes the session on a new event or a fixed cadence … best-effort by construction — NOT the sole wake signal'. The five bodies reference it by capability name, never by harness tool name, which is the vocabulary this brief keeps."
  - "The half of the contract that already exists: `plugins/assay/skills/pr-review-desk/references/out-of-scope-filing.md` § 'File-and-exit, never block — the pod-loop contract' — 'a pod CronJob run terminates; a live session window yields to the next PR in the queue … a loop that blocks in-run is undebuggable in a pod'. It already binds the ESCALATION path to terminate. Nothing binds the ordinary path, which is the gap: a tick with no escalation at all still never ends."
  - "The same clause, already scoped to bounded runs, in `plugins/assay/skills/worker-desk/SKILL.md` (line 544): 'scoped to POD / CronJob runs. In a bounded execution (a pod or scheduled job whose run must terminate) … the run exits; it never holds a bounded run open waiting. In a live role window this clause does not license an exit'. The body therefore already distinguishes a bounded run from a window — but only for the escalation branch, and with no way for the skill to KNOW which it is in. The trigger predicate this brief adds is what makes that existing distinction decidable."
  - "The reference-file convention to mirror: `plugins/assay/references/desk-shell.md` and `standing-note.md`, each cited from the bodies as a single block-quote line of the form ``> … are in [`../../references/<name>.md`](../../references/<name>.md).`` — one neutral statement of the mechanism, house values resolved elsewhere. `desk-shell.md` line 3 also carries the `<!-- assay:harnesslint non-matrix-reference — <reason> -->` declaration that keeps a harness-NEUTRAL reference out of `harnesslint`'s per-harness binding matrix; `standing-note.md` omits it, which is why `harnesslint bindings` is already red on that file today."
  - "The parity mechanism this brief USES rather than reinvents: `.claude/guardrails/GUARDRAILS.md` — five declared rule blocks, each with an exhaustive `- site:` list and a ```text fence holding the canonical text; `tools/skillslint` § `guardrail.go` locates each copy by its FIRST LINE (which must be unique in the site file) and byte-diffs the rest, three-state and never fail-open; `make guardrail-sync` regenerates a located copy. `default-forward-reversibility` (source line 147) is the existing block whose site list is exactly these five bodies. This is the gating plugin-tree lint on pull requests."
  - "What has NO check, and therefore needs a purpose-written one: nothing in `tools/` verifies that a file under `plugins/assay/references/` is cited by any skill body. `skillslint`'s house-value walk reads every markdown under `plugins/` for CONTENT, not linkage; its hidden-character scan does not read the references directory at all. An orphaned reference is currently invisible."
  - "The hermetic shell-suite precedent the grammar checker follows: `plugins/assay/scripts/pr-monitor.sh` with `plugins/assay/scripts/pr-monitor.test.sh` — 'No network and no token … the suite is hermetic and runs in CI', globbed and run by the `plugin-shell-suites` CI job over `plugins/assay/scripts/*.test.sh`. It is the only place in this repo where a shipped script's behaviour is asserted offline."
  - "What does NOT exist here, stated so no Verify row pretends otherwise: there is no offline runner for a SKILL.md body anywhere in this repo. `tools/skillbench` is explicitly 'a pure reducer over committed session artifacts, not an agent runner'; `tools/desk/internal/fleetharness` exercises the desk TOOLS, not skill prose. Any row that requires a real tick PASS is therefore offline-barred here and runs against the loop image, in the image's own repository."
  - "The fresh-sweep gate a tick's single sweep must satisfy: `plugins/assay/skills/worker-desk/SKILL.md` § 'HARD GATE — never claim \"pool empty / nothing to dispatch\" without a fresh sweep' (132), `pr-review-desk` § 'HARD GATE — no idle claim without a fresh board sweep' (111), `verify-desk` § 'HARD GATE — never claim \"idle / caught up\" without a fresh sweep' (40)."
  - "The three-state instrument rule the `could-not-check` outcome implements: the common-clauses kit's C4 — checked-clean / checked-failed / could-not-check, the third reported AS ITSELF and never rounded up to green."
  - "The exact-match env precedent: `statusgen/telemetry.go` § `telemetryArmed` compares its environment variable EXACTLY to \"1\", so no inherited truthy value can arm it silently. `ASSAY_TICK` follows it."
  - "The stop flags a tick must still honour: the stop-flag block in `plugins/assay/skills/worker-desk/SKILL.md` § 'Stop-flag check — run at every iteration boundary' — precedence `DISABLED` > `STOP` > `STOP.<name>`, enforced independently at the tool layer."
  - "The launch surface that will eventually pass the flag: `docs/cellctl.md` § '`cellctl desk`, `up`, `down`' and § 'Pinned models' — the per-run model override and harness selection already exist; the tick flag joins the same argument path."
exec-tier: strong
exec-tier-why: >-
  (c) the deliverable is a NEGATIVE contract, and a negative contract is the shape that passes
  every happy-path test while being entirely absent. "The tick never arms a durable wake" and
  "the tick never asks a human" are satisfied by a body that merely omits to mention them; only
  a transcript assertion over a real tick pass distinguishes a contract that binds from one that
  is written down. The second hazard is the one that produced this brief: a mode that *almost*
  terminates — it prints its summary after the deadline, or it prints `noop` when it was blind —
  is worse than no mode at all, because the loop image then reports health it never observed. The
  `noop` / `could-not-check` split and the reserve arithmetic each need a fixture that forces the
  bad branch, and neither is reachable by reading the diff.
consumers:
  - "`plugins/assay/references/tick-contract.md` (planned): follow-up desk-containers/08 — the DEFERRED disposition, because the brief-authoring PR declares this path and the implementing change creates it; it flips to fixed-here there. New: the single statement of the contract; every other file cites it and none restates it."
  - "`.claude/guardrails/GUARDRAILS.md`: follow-up desk-containers/08 — deferred for the same reason, flipped to fixed-here by the implementing change. A sixth declared block, `tick-mode`, whose site list is exactly the five desk bodies — the same shape as the existing `default-forward-reversibility` block, so parity is enforced by the gating lint rather than by a new bespoke check."
  - "`plugins/assay/skills/the-desk/SKILL.md`, `plugins/assay/skills/intake-desk/SKILL.md`, `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/pr-review-desk/SKILL.md`, `plugins/assay/skills/verify-desk/SKILL.md`: follow-up desk-containers/08 — deferred, flipped to fixed-here by the implementing change. Each gains ONE short `## Tick mode` section — a copy of the declared block — immediately after its Boot section, citing the reference; no other line of any of the five changes."
  - "`plugins/assay/scripts/tick-summary.sh` (planned) and `plugins/assay/scripts/tick-summary.test.sh` (planned): follow-up desk-containers/08 — deferred, flipped to fixed-here by the implementing change. New: the published summary-line grammar as an executable checker plus its hermetic case suite, in the shape of the existing monitor script and its suite, so the grammar has one implementation that a parser and a test both read."
  - "The `## Liveness contract (binding)` block in all five bodies: out-of-scope (read, NOT changed — the standing-window behaviour is exactly what it is today, and Verify row 8 asserts the block is byte-identical to its pre-change form)."
  - "`plugins/assay/skills/pr-shepherd/SKILL.md` and every non-desk-role skill: out-of-scope (a worker role is already invoked as a bounded job with a definite end; it has no standing loop to bound, so it gains no tick mode and is unchanged)."
  - "The loop image's own invocation — passing `--tick` or setting `ASSAY_TICK=1`: out-of-scope (the image lives in another repository and is changed there; named here as the operator knob and implemented nowhere in this repo. Until it lands the contract is inert by construction, because absent both spellings every run is a window — which is what makes landing the contract first the safe order rather than a gap)."
  - "`ASSAY_TICK_DEADLINE`, the CronJob schedule, and the per-role width: out-of-scope as VALUES (operator settings). This brief fixes only how a tick READS the budget and what reserve arithmetic it applies; it sets no number in any manifest."
  - "`tools/desk` and `deskboot`: out-of-scope (read, NOT changed — boot is identical in both modes; what a tick skips is the skill's OWN first-boot residue, not anything a desk verb does)."
version: 1
id: 09e5e77a-2e96-4bbc-8694-f45f1151af8b
---

# Brief 08 — a tick contract: one bounded pass when the harness says `--tick`

## Dependencies

None. This is deliberate and worth stating, because the obvious guess is wrong: brief 03's
per-desk image matrix is NOT a prerequisite. The deliverables here are one new file under
`plugins/assay/references/` and one added section in each of five `SKILL.md` bodies — content
every desk image already carries, because the base image bakes `plugins/assay/` wholesale into
the plugin directory the harness is pointed at. The dependency runs the other way: the image
needs the contract before passing `--tick` can mean anything, which is why the image's own
change is named as follow-up rather than taken here. Wave 3 places it beside the other
launch-surface briefs (04/05/06) because that is when it becomes operationally load-bearing,
not because anything in wave 1 or 2 blocks it.

## Context

files:
- `plugins/assay/references/tick-contract.md` (planned) (new) — the single statement of the contract.
- `.claude/guardrails/GUARDRAILS.md` — a sixth declared block, `tick-mode`, with five sites.
- `plugins/assay/skills/the-desk/SKILL.md`, `plugins/assay/skills/intake-desk/SKILL.md`,
  `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/pr-review-desk/SKILL.md`,
  `plugins/assay/skills/verify-desk/SKILL.md` — one added `## Tick mode` section each, derived
  from the declared block; no other line of any of the five changes.
- `plugins/assay/scripts/tick-summary.sh` (planned), `plugins/assay/scripts/tick-summary.test.sh` (planned) (both
  new) — the summary-line grammar as one executable, and its hermetic case suite.
- `docs/streams/desk-containers/README.md` — the board row, the narrative and the wave entry.
- `changelog/<slug>.md` — the fragment.

Nothing under `.github/workflows/`, `containers/`, `tools/` or any manifest is touched: the new
suite is picked up by the existing shell-suite CI job through its `*.test.sh` glob, so the change
adds a gate without adding CI wiring.

single-point-of-failure: **the trigger predicate — the ONE place that decides whether this run
is a tick or a window** — with three independent layers behind it. (1) The outer `timeout` the
harness already wraps the call in is enforced by the operating system, in a different component,
and binds whatever the skill believes about its own mode: a mis-fired predicate cannot run past
it. (2) The negative half of the contract is stated as ABSENCES a transcript grep can see — no
durable wake armed, no cadence sleep, no human prompt — so a run that armed one is detectable
after the fact regardless of which branch the predicate took (row 10). (3) The summary line's
ABSENCE is itself a signal with its own reader: a loop image that receives no `tick …` line knows
the pass did not complete, which is precisely the distinction the shipped image cannot make today
(an empty log currently means "killed" and "healthy but silent" equally). Three components — the
skill body, the kernel's `timeout`, the image's log parser — tripping on three different signals
at three different time scales.

### The problem, stated once

A role skill is invoked by the loop image as a single non-interactive harness call, with a text
output format, wrapped in `timeout $ASSAY_TICK_DEADLINE`. Three properties of that shape matter:

- **It is one-shot.** The call has no second turn. Whatever the skill is doing when the deadline
  expires, it is doing when the process dies.
- **It prints only at completion.** The one-shot text output mode buffers the turn and emits it
  when the turn ends. A killed turn emits nothing — not a partial transcript, not the work it
  actually did. The pod log for a 480 s kill is empty.
- **It is nested inside two harder deadlines.** 480 s under an `activeDeadlineSeconds` of 540 s
  under a token life of 1 h. The skill can only ever lose a race with the innermost.

Against that, every desk body says the same thing: arm the standing self-scheduled loop before
the first sweep and keep it ticking for the life of the window. There is no second mode. So the
loop boots, sweeps, arms its wake, and settles into the cadence it was designed for — and the
`timeout` kills it mid-cadence with nothing printed. Measured 2026-09-14: the review loop's
first successful boot on the fixed image did exactly this, and no review tick has ever completed.

The existing pod-loop contract covers only the escalation branch — *file and exit rather than
block* — and even there the body cannot act on it, because nothing tells the skill which kind of
run it is in. `worker-desk` already spells the distinction out ("scoped to POD / CronJob runs …
In a live role window this clause does not license an exit") and has no way to decide it. The
trigger predicate is the missing half of a rule that is already written.

### What is NOT the problem

- **Not the deadline.** Raising 480 s to 900 s buys a standing loop nothing: a loop with no
  terminating condition runs out whatever budget it is given, and the log is empty either way.
- **Not the boot cost.** Boot completes; the measured run got past it.
- **Not the image.** The image change (passing the flag) is one line and is follow-up; the image
  cannot pass a flag to a mode that does not exist.
- **Not a new loop.** Nothing here writes a second scheduler, a second sweep, or a second wake.
  A tick reuses the role's existing boot, its existing named sweep instrument, its existing
  dispatch width and its existing filing verbs, and stops after one pass of them.

### The contract, in shape

Stated once in `plugins/assay/references/tick-contract.md` (planned) and cited — never restated —
by all five bodies.

**Trigger.** A run is a TICK when either holds, and the two are an OR with no precedence:

| Spelling | Who sets it | Rule |
|---|---|---|
| the literal argument `--tick` in the skill's argument string | a human, or any launcher that can add an argument | present ⇒ tick |
| the environment variable `ASSAY_TICK` | a CronJob / container env, which cannot add an argument | compared EXACTLY to `1` ⇒ tick; any other value, including `true`, `yes` and the empty string, is NOT a tick |

Both are supported because the two callers are genuinely different: a CronJob sets environment,
a human types an argument, and a launcher in between may only be able to do one. The exact-match
rule on the environment half follows `statusgen`'s telemetry switch, for the same reason: no
inherited truthy value may silently convert a live window into a one-pass run.

Absent both, the run is a WINDOW and behaves exactly as it does today. That default is what makes
this change safe to land before the image change: with nothing passing the flag, nothing changes.

**The bounded pass**, in order, once:

1. **Boot** — `deskboot` and the role's own boot section, unchanged. Both modes boot identically.
2. **Read the budget** — `ASSAY_TICK_DEADLINE` in whole seconds if present; if absent, the tick
   still runs exactly one pass and simply performs no reserve arithmetic.
3. **ONE fresh sweep** of this role's own queue, using the same named instrument its HARD-GATE
   section already names. Exactly one. A tick never re-sweeps.
4. **Act** on what that sweep made actionable, up to the role's own declared width — the same
   width its pool section already states. No sweep follows the acting.
5. **Wait, bounded**, for what this pass dispatched, with the bound derived from the remaining
   budget (below). A subagent still running when the bound expires is reported, not waited on.
6. **Print the summary line** as the last line of output, and **exit**.

**What a tick never does.** The negative half, and the enforceable one:

- never arms `capability:durable-monitor` or any other durable cross-turn wake;
- never schedules a wake-up, and never sleeps for a cadence interval;
- never re-sweeps for a second cycle, and never refills a slot a first-pass dispatch vacated;
- never prompts a human or waits in-line for an answer — an escalation is a FILED issue and the
  pass continues, which is the file-and-exit pod-loop contract already in force;
- never claims **idle** or **caught up**. One fresh sweep satisfies the HARD GATE for the pass it
  ran; it does not license a standing-state claim about the queue. A tick that found nothing
  reports `noop`, which says only "this pass found nothing actionable".

Everything else the role does is unchanged: its gates, its budgets, its stop flags (`DISABLED` >
`STOP` > `STOP.<name>`, checked before the pass and honoured as a `refused`), its identity rules,
its filing verbs, its width. **The tick narrows the LOOP, never a GATE.** A tick that is running
short of budget drops WORK, never a control: it stops dispatching, it does not stop checking.

**Budget arithmetic.** Let `D` = `ASSAY_TICK_DEADLINE`, `E` = elapsed seconds, `R` = the exit
reserve. The tick stops dispatching new work once `D − E < R + W`, where `W` is the role's
expected cost for one more unit of work. `R` covers only the exit path — the summary line, a
workpad or standing-note update, and releasing any claim this pass took — and defaults to **60 s**.
Each subagent this pass dispatches is given a deadline of `D − E − R`, so a dispatched agent
cannot outlive the pass that owns it.

The operator consequence, stated as arithmetic rather than a preference: with `D = 480` and
`R = 60`, a pass has `420 − boot` seconds of working budget. One PR review on a strong model does
not reliably fit in that, so **480 s is a floor for a `noop` pass, not a target for a working
one** — an operator who wants a review tick to actually review something sets `D` to at least
boot + one review + 60, and raises the pod's `activeDeadlineSeconds` above it in the same edit,
since the pod deadline must stay the outer bound. This brief sets no number in any manifest; it
fixes only how the tick reads the one it is given.

**The summary line.** Exactly one line, the LAST line of standard output:

```
tick role=<role> outcome=<outcome> swept=<n> acted=<n> filed=<n> duration=<s>
```

| Field | Value |
|---|---|
| leading token | the literal `tick`, so a log grep needs no context |
| `role` | one of the five public desk names, exactly as the skill is named |
| `outcome` | `ok` \| `noop` \| `refused` \| `could-not-check` |
| `swept` | items the one fresh sweep enumerated |
| `acted` | items this pass dispatched, landed or flipped |
| `filed` | issues filed or attached this pass |
| `duration` | whole seconds from process start to this line |

Fields appear in that fixed order, single-space separated, `key=value`, unquoted, and no value
may contain a space. A count that is genuinely unknown is written `-`, **never `0`** — a zero is a
measurement claim and an unfinished sweep has not made one.

`outcome` is decided as:

| Outcome | When | Then, necessarily |
|---|---|---|
| `ok` | the sweep completed and at least one act landed | `swept` numeric, `acted` numeric and ≥ 1 |
| `noop` | the sweep completed and found nothing actionable | `swept` numeric, `acted` = 0 |
| `refused` | the pass declined to run at all — a stop flag, a kill switch, a tripped budget breaker, an unmet precondition the role refuses on | `swept` = 0, `acted` = 0 |
| `could-not-check` | the sweep did not complete, or completed blind — an instrument exited non-zero, a board could not be read, a token could not be minted, or the reserve cut the pass short mid-sweep | `swept` = `-` |

**`could-not-check` is never rounded to `noop`.** They are the two outcomes a reader will most
want to confuse, and they mean opposite things: `noop` says the queue is empty, `could-not-check`
says the instrument did not look. Rounding one to the other turns a blind loop into a health
report, which is the C4 three-state rule applied to this line.

**The right-hand column is what makes that mechanical rather than aspirational.** A shape-only
grammar accepts `outcome=noop swept=-` — a blind pass wearing a healthy pass's clothes — and any
parser written against such a grammar will eventually report health nobody observed. Because
`could-not-check` REQUIRES `swept=-` and `noop` FORBIDS it, a pass that could not read its queue
cannot produce a well-formed `noop` line at all. The distinction stops being a rule a producer
has to remember and becomes one the grammar will not let it break.

**Why a line and not structured output.** The one-shot text mode emits one blob at completion; a
single grep-able last line survives a truncated or interleaved pod log, is parseable by `awk`
without a dependency, and is readable by a human running `kubectl logs` at three in the morning.
The line is the sole machine-readable verdict — the tick does not encode its outcome in a process
exit status, because the harness owns that and exits 0 for any completed turn regardless.

**What the tick skips at boot.** Only the skill's OWN first-boot residue: arming the durable
wake, registering the window as the exclusive holder of a watcher, and any "read the register /
re-read the standing note from the last window" step whose only purpose is continuity across
turns a tick does not have. `deskboot` itself is unchanged and runs identically in both modes —
the boot a tick performs is the boot a window performs.

## Ground rules

- **The contract is stated ONCE.** `plugins/assay/references/tick-contract.md` (planned) is the only place
  the mechanism is described. A body that restates a rule instead of citing the reference is a
  finding, not a convenience — five copies drift, and this file exists because five copies of the
  standing-loop rule already had to be kept in step by hand.
- **The five sections are byte-identical.** Not "equivalent", not "adapted per role". Any
  role-specific value (the name of the sweep instrument, the width) is named by the role's own
  existing sections, which the tick reuses; the Tick mode section itself carries no role-specific
  text at all.
- **Capability vocabulary, never harness tool names, in the bodies.** The bodies say
  `capability:durable-monitor`; a harness's tool name appears only in a test that greps a
  transcript, never in a body.
- **The window path is untouched.** Not one line of the `## Liveness contract (binding)` block,
  the loop sections, the HARD GATEs, or the boot sections changes. The Tick mode section is
  purely additive and sits after Boot.
- **The tick narrows the loop, never a gate.** No gate, budget, stop flag, identity rule or
  escalation obligation is relaxed in tick mode. A budget-short tick drops work, never a check.
- **Nothing in this repo passes the flag.** The image's invocation change is follow-up. Until it
  lands the contract is inert by construction, because absent both spellings every run is a
  window.
- **No number lands in a manifest.** `ASSAY_TICK_DEADLINE`, the schedule and the widths are
  operator values; this brief fixes the arithmetic, not the settings.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **The reference — `plugins/assay/references/tick-contract.md` (planned).** One neutral file in
   the house reference shape (mechanism stated, values deferred), covering: the trigger predicate
   and its two spellings with the exact-match rule; the six-step bounded pass; the negative
   contract as an explicit list; the budget arithmetic with `R` and the derived subagent deadline;
   the summary-line grammar with the field table, the outcome table and the `-`-not-`0` rule; the
   `noop` / `could-not-check` distinction stated as its own paragraph; and the "what a tick skips
   at boot" boundary. It states no role-specific value and no operator number.
   **It carries the `<!-- assay:harnesslint non-matrix-reference — <reason> -->` declaration on
   line 3**, in `desk-shell.md`'s exact form: it is harness-NEUTRAL, so without the declaration
   `harnesslint bindings` would demand it resolve all seven capabilities and carry a degradation
   cell for every skill, which is a matrix it is not and must not become.
2. **The section, DERIVED not copied — a sixth guardrail block.** The `## Tick mode` text is
   authored ONCE as `## guardrail: tick-mode` in `.claude/guardrails/GUARDRAILS.md`, with a `- site:`
   line for each of the five bodies and the canonical text in the ```text fence, exactly as
   `default-forward-reversibility` already does for the same five files. The copy is then placed in
   each body immediately after its Boot section and before the heading that currently follows it —
   `the-desk` (after `## Boot`), `intake-desk` (after `## Boot sequence`), `worker-desk` (after
   `## Boot`), `pr-review-desk` (after `## Boot`), `verify-desk` (after `## Boot`) — and from that
   point on it is regenerated by `make guardrail-sync` and byte-diffed by the gating lint, never
   hand-maintained. The block's FIRST LINE is its anchor and must be unique in each site file, which
   `## Tick mode` is. Content: the trigger, the one-pass rule, the negative contract in a sentence,
   the obligation to print the summary line, and the citation in the established block-quote form.
   Constraints the lint imposes on the text: no banned harness token (the capability name
   `capability:durable-monitor` is the only vocabulary for the wake, and a harness's own tool name
   never appears in a body), and no proper name in a driver position — `human:<name>` only.
   **Placement caution:** the copy must not land inside any existing generated guardrail region.
3. **The grammar, as one executable — `plugins/assay/scripts/tick-summary.sh` (planned).** The
   published grammar has exactly one implementation: a small POSIX-shell checker with a `validate`
   verb that reads a candidate line and exits 0 only if it satisfies the grammar, so a parser, a
   test and a reader are all looking at the same rule. It is the artifact the loop image's own
   parser is written against.
4. **Its hermetic case suite — `plugins/assay/scripts/tick-summary.test.sh` (planned).** In the
   shape of the existing monitor-script suite (no network, no token, named cases, picked up by the
   `plugin-shell-suites` CI job through its `*.test.sh` glob): valid lines for each of the four
   outcomes; and the invalid ones, each its own named case — wrong field order, a space inside a
   value, an outcome outside the closed set, a missing leading `tick` token, a `0` where the case
   marks the count unknown, and a summary line that is not the last line of its input.
5. **Changelog fragment** under `changelog/` (one `### Added` bullet).
6. **Board row** — `08` added to `docs/streams/desk-containers/README.md` (this brief's own PR
   sets it `todo`; the implementation PR moves it to `in-progress`), plus the narrative and wave
   entries.
7. **Nothing else.** No image change, no manifest change, no new desk verb, no second scheduler,
   no change to `deskboot`, no change to any non-desk-role skill, and no edit to the standing-loop
   text. In particular **no new offline agent runner**: this repo has none, and writing one to
   green a Verify row would be a far larger piece of work than the contract it was meant to check.

## Definition of done

- A run with neither `--tick` nor `ASSAY_TICK=1` behaves exactly as it does today, proven by the
  `## Liveness contract (binding)` block and the loop/boot sections being byte-identical to their
  pre-change form (row 8).
- The `## Tick mode` block is DERIVED from one declared source and compared at all five sites by
  the gating lint — `GUARDRAILS: checked-clean` with five pairs compared, not zero (row 2).
- Each body cites the reference exactly once by a path that resolves, and no body restates the
  grammar (row 5). The reference itself is declared non-matrix, so `harnesslint bindings` skips it
  rather than demanding a capability matrix of it (row 4).
- The grammar has exactly one implementation, it is hermetically tested, and its suite is picked up
  by existing CI with no new wiring (rows 6, 7). A line carrying `0` for an unknown count or an
  outcome outside the closed four is REJECTED.
- `cd tools/skillslint && go run . --root ../..` exits 0 on all five verdict lines;
  `harnesslint bodies` is `checked-clean`; `statusgen --root . --lint` reports `LINT: PASS`.
- The four online-lane properties — one pass inside the deadline, no durable wake armed, the
  reserve honoured, and the trigger predicate exact — are recorded as could-not-check with the
  named hand-off, NOT as passes, until a cell runs rows 9–12.
- The board row is `implemented` and the changelog fragment is present.

## Verify

Rows 1–8 and 13–14 run offline in this repo. Rows 9–12 need a real tick PASS, and **this repo has
no runner for a skill body** — `skillbench` is a reducer over committed artifacts, not an agent
runner, and the fleet harness exercises the desk tools rather than skill prose. Those four are
therefore ONLINE-LANE rows in exactly the sense `verify-desk` § "Cluster rows — the offline→online
hand-off" already defines: they are run on a cell against the loop image, and until they are they
are reported as **could-not-check, never as a pass**. Writing a skill-body runner to green them
here would be a larger piece of work than the contract they check, and is not in this brief.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/skillslint && go run . --root ../..` | exit 0 — all five verdict lines clean with the five changed bodies and the new reference in the tree. `HOUSE-VALUES` walks every markdown under `plugins/`, so the new reference is read: a proper name in a driver position anywhere in it REDDENS this row (the driver is `human:<name>`) |
| 2 | check +mutation | the same command, reading the `GUARDRAILS:` verdict specifically | exit 0 — `checked-clean`, with the `tick-mode` block compared at all FIVE sites (a run that compared zero pairs proved nothing and is reported as a failure, not a pass). Mutation: change one word in ONE body's copy → `GUARDRAILS: FAIL`; delete one copy → `could-not-check: anchor line is not present`. This is the parity row, and it is mechanical because the block is DERIVED from the declared source, not hand-copied |
| 3 | check:ci | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | exit 0 — `checked-clean`. The added section introduces no banned harness token and no `capability:` outside the closed seven. Mutation: naming a harness's own wake TOOL in a body instead of `capability:durable-monitor` REDDENS it |
| 4 | check | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | the run prints `skipped (declared non-matrix-reference): …/tick-contract.md`, and the new file contributes ZERO violations. **Expect the mode's overall exit to remain 1**, unchanged from the base commit, because `standing-note.md` is already red there for a pre-existing missing declaration; the assertion is the DELTA (violation count and file list identical to the base run apart from the new skip line), not a green mode. Fixing `standing-note.md` is out of scope and is named, not silently absorbed |
| 5 | check +mutation | a citation check over the five bodies: exactly one link to `../../references/tick-contract.md` each, the path resolving from that body's directory, and the summary-line GRAMMAR LINE itself absent from every body (a body may name an outcome in prose; it may not carry a second copy of the line shape) | exit 0 — five citations, five resolving paths, zero restatements. This row exists because **nothing in `tools/` checks that a reference is cited at all** — an orphaned reference file is currently invisible to every lint. Mutation: inlining the grammar into one body, or citing a path that does not resolve, REDDENS it |
| 6 | check:ci | `bash plugins/assay/scripts/tick-summary.test.sh` | exit 0 — every named case passes, hermetically (no network, no token). Picked up by the existing `plugin-shell-suites` CI job through its `*.test.sh` glob, so the grammar is gated on every future PR without new CI wiring |
| 7 | check +mutation | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | exit 0 — a line carrying `0` where the case marks the count unknown is REJECTED, and an `outcome` outside the closed four is REJECTED. Mutation: widening the checker to accept any non-negative integer in an unknown field, or to accept an arbitrary outcome word, REDDENS it. The grammar's whole job is to make `could-not-check` un-spellable as `noop` |
| 8 | check:ci | `git diff refs/remotes/origin/main -- plugins/assay/skills/` restricted to the `## Liveness contract (binding)` blocks, the loop sections and the boot sections of all five bodies | exit 0 / empty — the window path is untouched. The regression row: it is what makes "purely additive" checkable rather than asserted, and it is the one row a reviewer can run without reading the whole diff |
| 9 | check +flow | **ONLINE LANE** — on a cell: one `pr-review-desk` tick pass against a forced-actionable queue, under the image's own `timeout` | the pass completes strictly INSIDE the deadline; its LAST stdout line satisfies the row-6 grammar; `outcome=ok` with `acted` ≥ 1. The cross-component row — trigger → boot → one sweep → act → summary, end to end. **Offline-barred in this repo**; could-not-check until a cell runs it |
| 10 | check +mutation | **ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt | zero hits. Mutation: arming the durable wake before the sweep — i.e. taking the window path — REDDENS it. This is the NEGATIVE contract's only mechanical enforcement anywhere, which is why it is named as an online row rather than dropped for being inconvenient |
| 11 | check | **ONLINE LANE** — row 9 repeated with `ASSAY_TICK_DEADLINE=60` | the summary line is printed strictly before 60 s elapse; no new work is dispatched once fewer than `R` + one work-unit seconds remain; and any subagent the pass dispatched carried a deadline no later than the pass's own. Mutation: removing the reserve subtraction so the pass dispatches against the raw deadline REDDENS it |
| 12 | check | **ONLINE LANE** — row 9 repeated with, in turn: no trigger; `--tick` only; `ASSAY_TICK=1` only; both; and `ASSAY_TICK` set to each of `true`, `yes`, `0` and the empty string | the first four spellings tick; the last four do NOT and take the window path. Mutation: relaxing the environment comparison from exact `1` to a truthiness test REDDENS it — the failure mode is an inherited variable silently converting a live operator window into a one-pass run |
| 13 | check:ci | `statusgen --root . --lint; echo $?` | 0 — `LINT: PASS`; the new brief, its board row, its wave entry and its frontmatter are all well-formed |
| 14 | check:ci +dereference | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | 0 — every deliverable path is routed `follow-up desk-containers/08` (the DEFERRED disposition: the brief declares the path, and the gate corroborates a follow-up claim against the brief rather than against a diff). A `fixed-here` entry naming a path this change does not add is DISPROVED and exit 1 — that is the failure this row exists to catch, and it is NOT satisfiable by running `--lint` alone. **Measured caveat, 2026-09-14 (statusgen v1.0.6 and the in-tree source, same verdict):** `fixed-here` was also DISPROVED for four paths that were tracked, present on disk, and carried in the very same commit as the brief, with the reason `resolves to nothing in the tree and appears nowhere in the diff` — which is verifiably false of those paths. Until that is diagnosed, `follow-up` is the only disposition this gate will corroborate for these entries, and the flip-to-`fixed-here` step the tool suggests inline cannot be exercised. Reported separately; the row passes on the deferred disposition and this caveat is recorded rather than worked around. Exit 2 is COULD-NOT-CHECK (no diff to take, e.g. on a fully merged tree) and is reported AS ITSELF, never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The tick mode is documented but the bodies never actually stop — the standing loop still arms its wake | row 10 (transcript absence-grep) + row 9 (the pass completes inside the deadline) — both online-lane, and deliberately named as such rather than replaced with a cheaper row that would not see this |
| An inherited environment variable silently converts a live operator window into a one-pass run | row 12 (the four non-`1` spellings) |
| A blind pass reports `noop`, so the loop image records health nobody observed | row 7, and its mutation — the grammar makes the wrong word un-spellable before any runtime gets the chance to spell it |
| An unknown count is written `0`, which reads as a measurement the sweep never made | rows 6 and 7 |
| The summary line is printed, but after the deadline — so the pod still logs nothing and the fix changed nothing | row 11 (printed strictly before 60 s) |
| A dispatched subagent outlives the pass that owns it and is killed mid-write | row 11 (the derived subagent deadline) |
| The five sections drift apart the first time one of them is edited | row 2 — and structurally, because the block is derived from one declared source rather than copied |
| The grammar is restated in a body and the two copies diverge | row 5 (exactly one citation, zero restatements) |
| A parser is written against a grammar that accepts more than it should | rows 6 and 7 (the named rejection cases) |
| The window path regresses while nobody is looking, because the diff "only added a section" | row 8 (byte-comparison against the base commit) |
| The new reference file is orphaned — added, cited by nothing, and quietly stale | row 5. Nothing in `tools/` would otherwise see this, which is why the row is purpose-written rather than delegated to a lint |
| The new reference is treated as a per-harness binding matrix and demands seven capability rows it should not have | row 4 (the declared-skip line) |
| A pre-existing red in `harnesslint bindings` is absorbed into this change and read as caused by it — or, worse, silenced | row 4 asserts the DELTA against the base run, and names the pre-existing red rather than fixing it inside this brief |
| A budget-short tick drops a CHECK rather than dropping WORK | review-only — the reviewer confirms the reserve arithmetic gates dispatch and nothing else; a gate never reached in a run cannot be asserted absent by a row |
| The contract is landed and the image never passes the flag, so nothing changes and nobody notices | review-only — the follow-up is named in `consumers:`, and the inert-by-default property is what makes landing first the safe order rather than a gap |

## Evidence
<!-- appended at implementation time by a NON-implementer: one witness row per Verify item
     (command, exit code, observed output, date, runner). -->

| # | Exit | Key observed output | Date | Runner |
|---|------|---------------------|------|--------|
| — | — | not yet run — this brief is authored, not implemented | — | — |

### Non-implementer verifier run — VERIFY: BLOCKED — 1/14 pass, 13 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Ran against merged origin/main 39866201ce48acdce1f9b14d1cae38eb2b7eff38 on darwin, offline
(KUBECONFIG=/dev/null). Every row the brief's `## Verify` table classes `check:ci` (rows 1, 3,
6, 8, 13, 14) is COULD-NOT-CHECK on this darwin desk: the hermetic `statusgen verifyrun` witness
needs a Linux `unshare --net` sandbox, so the direct non-hermetic run is recorded supporting-only,
never as a pass (#1491 is the `--in-container` witness path). Rows 2, 5 and 7 are `+mutation` rows
whose reddening half was not exercised in this offline fixer pass, so they too are could-not-check.
Rows 9-12 are ONLINE-LANE, offline-barred; row 14 is the sanctioned merged-tree could-not-check.
Only row 4 (a plain offline `check`, delta assertion) is a clean pass.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/skillslint && go run . --root ../.. | exit 0 — all five verdict lines clean with the changed bodies and new reference in tree | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; SKILLSLINT/HIDDEN-CHARS/HOUSE-VALUES/GUARDRAILS/ENFORCEMENT-BLOCK all PASS; HOUSE-VALUES read 38 markdown files under plugins/, no proper name in a driver position | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | cd tools/skillslint && go run . --root ../.. | exit 0 — checked-clean with the tick-mode block compared at all FIVE sites, not zero | COULD-NOT-CHECK — the +mutation half was not exercised in this offline fixer pass: the clean run passed (GUARDRAILS: PASS, 32 guardrail copies byte-match; tick-mode block compared at all five sites) but the reddening (one word changed in ONE body's copy → GUARDRAILS: FAIL) was not run | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills | exit 0 — checked-clean; no banned harness token, no capability outside the closed seven | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; checked-clean: bodies — no violations | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references | tick-contract.md prints skipped (declared non-matrix-reference) and contributes ZERO violations; mode exit stays 1 (a pre-existing unrelated red), assertion is the DELTA | PASS — exit 1 (pre-existing unrelated red; the assertion is the DELTA, met): tick-contract.md printed "skipped (declared non-matrix-reference)", zero violations from it; the one remaining violation is claude-code.md missing a system-demo degradation cell, which is pre-existing (absent already at the impl commit's parent) and untouched by this brief; delta attributable to this brief is +1 skip line, +0 violations | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | (manual row; no shell command) | five citations to ../../references/tick-contract.md, five resolving paths, zero grammar-line restatements | COULD-NOT-CHECK — manual citation check; the clean check passed (each body citations=1, path resolves, grammar-restatements=0) but the +mutation reddening (inlining the summary-line grammar into one body) was not exercised in this offline fixer pass | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | bash plugins/assay/scripts/tick-summary.test.sh | exit 0 — every named case passes hermetically (no network, no token) | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 71 passed, 0 failed; suite runs offline and includes bash-3.2 portability + no-network/forge-tool cases | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome | exit 0 — a 0 where the count is unknown is REJECTED and an outcome outside the closed four is REJECTED | COULD-NOT-CHECK — the +mutation half was not exercised in this offline fixer pass: the clean run passed (rejects-zero-for-unknown: 2 passed; rejects-unknown-outcome: 3 passed) but the reddening (widening the checker to accept an out-of-set outcome or an unknown-count 0) was not run | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | git diff refs/remotes/origin/main -- plugins/assay/skills/ | exit 0 / empty — the Liveness-contract/loop/boot window of all five bodies is untouched | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): the brief's literal `git diff refs/remotes/origin/main -- plugins/assay/skills/` is empty on the merged tree (HEAD == origin/main); the meaningful diff, run against the impl commit's parent, shows 130 insertions / 0 deletions across the five bodies (26 added lines each = the Tick mode section only), no Liveness-contract/loop/boot line changed | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | (manual row; no shell command) | ONLINE LANE — on a cell, one pr-review-desk tick pass against a forced-actionable queue under the image timeout: completes inside the deadline; last stdout line satisfies the grammar; outcome=ok, acted >= 1 | COULD-NOT-CHECK — offline-barred in this repo (no offline runner for a skill body: skillbench is a reducer over committed artifacts, the fleet harness exercises desk tools not skill prose); runs on a cell against the loop image, hand-off named | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | (manual row; no shell command) | ONLINE LANE — the row-9 transcript grepped for the arming of any durable wake, scheduled wake-up, cadence sleep and human prompt: zero hits | COULD-NOT-CHECK — offline-barred in this repo; requires the row-9 transcript from a cell run, hand-off named | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | (manual row; no shell command) | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60: summary line printed strictly before 60 s; no new work dispatched inside reserve+one-unit; every dispatched subagent carried a deadline no later than the pass | COULD-NOT-CHECK — offline-barred in this repo; requires a cell run, hand-off named | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | (manual row; no shell command) | ONLINE LANE — row 9 repeated with, in turn: no trigger; --tick only; ASSAY_TICK=1 only; both; and ASSAY_TICK set to each of true, yes, 0, empty; first four tick, last four take the window path | COULD-NOT-CHECK — offline-barred in this repo; requires a cell run. The exact-match trigger rule is asserted offline at grammar level and in prose (tick-contract.md line 40), but its RUNTIME behavior needs a cell, hand-off named | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | statusgen --root . --lint; echo $? | 0 — LINT: PASS; the brief, board row, wave entry and frontmatter well-formed | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; LINT: PASS (statusgen v1.0.26). NOTICEs printed are for other streams (statusgen, windows-port witness debt; verified-runner-attribution on unrelated briefs), none names desk-containers/08 | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $? | 0 — every deliverable routed follow-up desk-containers/08; exit 2 is could-not-check and reported as itself | COULD-NOT-CHECK — exit 2 on a fully merged tree, reported AS ITSELF: "assay:assay:desk-containers:08 is not in the diff against 39866201ce48..., so this run carries no evidence about its claims — no entry was corroborated and none was disproved." Exactly the sanctioned exit-2 on a fully merged tree that the row defines; not a pass, not a fail | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE lines (kit section 4 — enumerate then rank then derive):

Enumeration of every literal this diff introduces or changes:
- R (exit reserve default) = 60 seconds @ plugins/assay/references/tick-contract.md:99
- ASSAY_TICK exact-match trigger literal = "1" @ plugins/assay/references/tick-contract.md:40
- closed outcome set = { ok, noop, refused, could-not-check } @ plugins/assay/references/tick-contract.md:123 (a definitional set, not a scalar)
- budget breaker inequality D minus E less-than R plus W, and derived subagent deadline D minus E minus R @ plugins/assay/references/tick-contract.md:94-99 (formulae, no scalar of their own)
- The deadline facts named in the brief (480 s tick / 540 s activeDeadlineSeconds / 1 h token) are explicitly NOT set by this change — the brief sets no number in any manifest — so they are out of scope operator values.

Ranking by irreversibility: every enumerated entry is REVERSIBLE (edit-and-redeploy). The brief is
irreversible:no, gate:model, all four risk answers no; the diff touches only docs and POSIX shell
scripts (no risk-classed path) and changes no hard-pinned repo constraint. No irreversible entry
exists, so nothing routes to the human gate on risk grounds.

RISK-VALUE: DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — the exit reserve. Derived from its stated job: R covers only the exit path (print the one summary line, update the workpad or standing-note, release any claim this pass took). 60 s is a conservative fixed budget for three cheap local operations, sized so the exit completes before the outer OS timeout kills the process yet small enough not to starve the working budget (with D=480, working budget = 420 minus boot). It is a reversible operational default, not a hard-pinned constraint.
RISK-VALUE: DERIVED — ASSAY_TICK exact-match literal = "1" @ plugins/assay/references/tick-contract.md:40 — the environment trigger value. Derived from the anti-silent-arming requirement: an EXACT string match to "1" (mirroring statusgen/telemetry.go telemetryArmed) so no inherited truthy value (true, yes, non-empty) can silently convert a live operator window into a one-pass run. "1" is the conventional armed value; reversible. Enforced offline at grammar level (row 7) and, at runtime, only by online-lane row 12 (could-not-check here).

Every check:ci row is could-not-check (darwin hermetic witness owed, #1491); rows 2/5/7 mutation
halves un-run; rows 9-12 are the online-lane behavioural proofs (run by a cell against the loop
image), handed off; row 14 is the merged-tree could-not-check. Row 4 is the one clean offline pass.


## Review

Gate: model (all four risk answers no). The change is additive prose plus a shell checker and its
hermetic suite: it touches no credential, no access control, no merge or flip authority, and no
network path, and it is inert until a caller outside this repo passes the flag. Model-gated because
the parity hazard is fully mechanical (row 2, on the gating lint) and the grammar hazard is fully
offline (rows 6–7) — what remains is the online lane, which is a stated hand-off rather than an
unbounded judgment.

The reviewer confirms in the verdict:

1. That the `tick-mode` block is genuinely short and carries no role-specific text — a block that
   has grown a per-role clause is the first step of the drift the derive-or-diff mechanism exists
   to prevent, and the mechanism cannot catch text that was role-specific from the start.
2. That the negative contract in the reference is a CLOSED list, and that each item on it is either
   asserted by row 10 or explicitly recorded as review-only. An item on the list that no row and no
   reviewer covers is a promise, not a control.
3. That nothing in the diff relaxes a gate, a budget, a stop flag or an escalation obligation in
   tick mode. The tick may drop WORK under budget pressure; it must never drop a CHECK.
4. That the window path is untouched — by reading row 8's comparison, not the diff summary.
5. That rows 9–12 are recorded as could-not-check with the cell hand-off named, and that no
   offline row has been stretched to stand in for one of them. A cheap row that looks like it
   covers the negative contract, but runs against prose instead of a transcript, would be worse
   than the honest gap.
