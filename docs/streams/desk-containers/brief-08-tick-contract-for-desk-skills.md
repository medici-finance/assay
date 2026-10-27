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
- `plugins/assay/skills/the-desk/SKILL.md`, `plugins/assay/skills/intake-desk/SKILL.md`, `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/pr-review-desk/SKILL.md`, `plugins/assay/skills/verify-desk/SKILL.md` — one added `## Tick mode` section each, derived from the declared block; no other line of any of the five changes.
- `plugins/assay/scripts/tick-summary.sh` (planned), `plugins/assay/scripts/tick-summary.test.sh` (planned) (both new) — the summary-line grammar as one executable, and its hermetic case suite.
- `docs/streams/desk-containers/README.md` — the board row, the narrative and the wave entry.
- `changelog/tick-contract-dc08-09141911.md`, `changelog/tick-mode-slice1-dc08-09141911.md` — the fragment (the names the delivering change used).

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

### Non-implementer verifier re-run — VERIFY: BLOCKED — offline rows pass, 0 fail; check:ci witness and online lane owed — 2026-09-27 claude-opus-5-5-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `GUARDRAILS:` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | fail exit=1 | sha256:7e839fabe3b3 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:4bf739e2b4fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 | sha256:165f6e5d57e5 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 11 | `ASSAY_TICK_DEADLINE=60` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 12 | `--tick` | fail exit=2 | sha256:e2aee07e1822 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

Per-row notes for the witness table above (re-verify 2026-09-27, merged main 9585b4b6cc2e, darwin, offline with KUBECONFIG=/dev/null). Every row was also run directly by hand; the direct runs below are supporting evidence for the check:ci rows, which the hermetic witness cannot execute on this host.

What changed since the prior run (Sept 23): none of the brief's own deliverables moved (the reference, the checker and its suite hash identically); only the five desk bodies and the guardrail source changed, through later unrelated commits, and the tick-mode copies still byte-match. New in this run: the mutation halves of rows 2, 3, 5 and 7 were exercised (all four redden as the brief predicts), and row 14 was corroborated through the tool's own merged-brief recipe instead of stopping at the merged-tree exit 2.

- Row 1 — direct run exit 0. SKILLSLINT, HIDDEN-CHARS, HOUSE-VALUES (39 markdown files under plugins/, no proper name in a driver position), GUARDRAILS, ENFORCEMENT-BLOCK and POSIX-TOKEN all PASS. Witness: could-not-run, because check:ci needs the Linux network-off sandbox.
- Row 2 — clean: GUARDRAILS PASS, 33 copies byte-match, and all five desk bodies carry exactly one tick-mode section. Mutation, run on a scratch export of the tree outside the worktree: changing one word in verify-desk's copy gives "GUARDRAILS: FAIL — 1 copy/copies drifted (33 compared)", exit 1. Deleting the-desk's copy gives "could-not-check: guardrail tick-mode — its anchor line is not present", with "GUARDRAILS: COULD-NOT-CHECK — 32 compared", exit 1. Because the count drops from 33 to 32, the five tick-mode sites are among the pairs compared. PASS. The witness's could-not-run (exit 127) comes from parsing the Expect prose; the Command cell holds no command.
- Row 3 — direct run exit 0, "checked-clean: bodies — no violations". Mutation: swapping capability:durable-monitor for the harness's own backticked wake-tool name in the-desk's Tick mode section gives "banned harness token", with "checked-failed: bodies — 1 violation(s)", exit 1. PASS on the direct run. The witness could not run it: check:ci on darwin.
- Row 4 — exit 1, as the row expects. The delta was taken on current main with and without tick-contract.md in a scratch copy of plugins/assay: the only difference is +1 line, "skipped (declared non-matrix-reference): …/tick-contract.md", and +0 violations. The one remaining violation is claude-code.md missing a system-demo degradation cell, which this brief did not cause. The brief names standing-note.md as the pre-existing red; that file has since been declared non-matrix, so the pre-existing red now sits in a different file. That wording is stale, not a defect. PASS. The witness records "fail exit=1" only because it assumes exit 0 is expected.
- Row 5 — a manual citation check over the five bodies, so it has no witness command (the witness exit 127 is a parse artifact). Each body has exactly one link to ../../references/tick-contract.md, the path resolves from the body's directory, and none of them restates the summary-line grammar. Mutations: pasting the grammar line into worker-desk makes the check exit 1, and so does deleting the reference (every path fails to resolve) or re-pointing pr-review-desk's link to a wrong relative path. PASS.
- Row 6 — direct run exit 0, "71 passed, 0 failed". The suite includes the no-network / no-forge-tool case. The witness could not run it: check:ci on darwin.
- Row 7 — witness pass, exit 0. Directly: rejects-zero-for-unknown 2/0 and rejects-unknown-outcome 3/0. Mutations on a scratch copy of the checker: widening the outcome alternation in GRAMMAR to any lowercase word gives rejects-unknown-outcome "2 passed, 1 failed", exit 1. Dropping the could-not-check swept="-" requirement gives rejects-zero-for-unknown "0 passed, 2 failed", exit 1. PASS.
- Row 8 — the literal diff against refs/remotes/origin/main is empty (0 lines) because HEAD equals origin/main. Run against the implementing change (squash commit 21a3e1474, parent ef7bfc2292b5), each of the five bodies gets 26 lines added and 0 removed, in one hunk. Every hunk sits between the body's Boot heading (Boot sequence in intake-desk) and the heading that followed it before the change. No Liveness-contract, loop or boot line changed. PASS on the direct run. The witness could not run it: check:ci on darwin.
- Rows 9 to 12 — ONLINE LANE, so they cannot run in this repo: there is no offline runner for a skill body, and they need a cell running the loop image. They stay could-not-check, with the hand-off named in the brief. The witness results for these rows are Expect-prose parse artifacts and are not evidence. In particular, row 11's "pass exit=0" executed only the bare variable assignment ASSAY_TICK_DEADLINE=60 and proves nothing about the reserve, and row 12's "fail exit=2" executed only the bare string --tick. Neither is a pass or a fail of the online property.
- Row 13 — direct run exit 0, "LINT: PASS" (statusgen v1.0.27, 0 PROBLEM lines). No NOTICE names desk-containers/08. The witness could not run it: check:ci on darwin.
- Row 14 — the literal command exits 2, "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against 9585b4b6…", which is the sanctioned merged-tree exit. The tool's printed recipe was then run on a scratch clone at the implementing commit with the parent as base: exit 0, "4 corroborated, 0 disproved, 5 unchecked". All four deliverable entries are CORROBORATED on the follow-up desk-containers/08 disposition. The five UNCHECKED entries are the out-of-scope reviewer-judgement entries. PASS on substance.

Risk-bearing values — enumeration over the implementing change's shipped files (tick-contract.md, tick-summary.sh, the tick-mode guardrail block, and the five copies of that block):
- R (exit reserve default) = 60 seconds @ plugins/assay/references/tick-contract.md:99
- ASSAY_TICK trigger literal = "1", exact match @ plugins/assay/references/tick-contract.md:40 (restated in the tick-mode block, .claude/guardrails/GUARDRAILS.md:224)
- OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68
- COUNT = '([0-9]+|-)', where "-" is the unknown-count sentinel @ plugins/assay/scripts/tick-summary.sh:67
- ROLES = "the-desk intake-desk worker-desk pr-review-desk verify-desk" @ plugins/assay/scripts/tick-summary.sh:56
- VERSION = "1" (checker version) @ plugins/assay/scripts/tick-summary.sh:52
- Formulae with no scalar of their own: stop dispatching once D − E < R + W, and give each subagent the deadline D − E − R @ plugins/assay/references/tick-contract.md:96-100
- Not set by this change: the 480 s / 540 s / 1 h deadline facts. They are operator values, and the brief sets no number in any manifest.

Ranking by irreversibility: every entry can be undone by an edit and a redeploy, and nothing in the change moves funds, publishes, or deletes. Ranked by how badly a wrong value would hurt before anyone noticed: (1) ASSAY_TICK = "1" — a looser test would let an inherited variable silently turn a live window into a one-pass run; (2) COUNT and OUTCOMES — a wider grammar would let a blind pass report health; (3) R = 60 — if it is too small, the summary line misses the deadline and the pod logs nothing. ROLES and VERSION are identifiers and rank last.

RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — an exact string match to "1" is the only rule under which no inherited truthy value (true, yes, 0, the empty string) can arm tick mode. It copies the repo's own precedent, statusgen/telemetry.go:65 (os.Getenv(...) == "1"). Only online row 12 can prove it at runtime; that row is could-not-check here.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59 — these apply the three-state instrument rule: checked-clean (noop, ok), checked-failed (refused), and could-not-check. "-" is the only way to write an unknown count, so an unfinished sweep never claims a 0. The set is closed at four, so nothing outside the three states can be spelled. The row 7 mutations show both literals are load-bearing: widening either one reddens the suite.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — R has to cover the exit steps that run before the final summary line: updating the continuity record and releasing the claim, which are forge writes. So whether 60 s is enough depends on forge-write latency on the cell's network, which cannot be measured offline. Online row 11 (ASSAY_TICK_DEADLINE=60) is what would test it. This is an open question for the online lane, not a defect. It is a reversible operator default, and a too-small R costs one empty pod log, not an irreversible act. (The prior run, dated Sept 23, marked this DERIVED. It is downgraded here because that derivation rested on "conservative" judgement, not on a measured exit-path latency.)

Observations: (a) The witness's rows 2, 5, 9, 10, 11 and 12 are parse artifacts of Command cells that hold prose, and row 4's "fail" comes from assuming exit 0 when the row expects exit 1. Both are check-definition gaps in the Verify table, not implementation defects. (b) Row 4's named pre-existing red has moved from standing-note.md to claude-code.md (system-demo); the Expect text is stale, the delta assertion holds.

VERIFY: BLOCKED — every offline row passes on substance (rows 1-8, 13 and 14, with the mutation halves of rows 2, 3, 5 and 7 now run), and 0 rows fail. Rows 1, 3, 6, 8, 13 and 14 (check:ci) still need the hermetic Linux network-off witness; the direct runs above only support them. Rows 9-12 need a cell (online lane). The status stays implemented.

### Non-implementer verifier re-run: 2026-10-02T21:46:36Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

Host: darwin/arm64, offline envelope (KUBECONFIG=/dev/null, GOPROXY=off), go 1.27.1, statusgen v1.0.31. Every row was run by hand exactly as authored, from a detached worktree at the merged head. Go commands ran under a throwaway HOME. Mutation halves ran on a scratch export of the tree outside the worktree; the shell suite's temp directory was redirected into the verifier's own scratch directory. The worktree was clean before and after.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | cd tools/skillslint && go run . --root ../.. | exit 0, all verdict lines clean | exit 0 by hand. SKILLSLINT: PASS (14 skill files); HIDDEN-CHARS: PASS; "HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/, no proper-name-shaped token in a driver position"; GUARDRAILS: PASS; ENFORCEMENT-BLOCK: PASS; POSIX-TOKEN: PASS. check:ci row: the hermetic network-off witness could not execute on this host (see witness section), so the hand run is supporting evidence only | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | the row-1 command, reading the GUARDRAILS verdict | exit 0, checked-clean, tick-mode compared at all five sites; mutation reddens | exit 0, "GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"; each of the five desk bodies carries exactly one Tick mode heading. Mutation A (one word changed in verify-desk's copy): exit 1, "guardrail tick-mode has drifted" and "GUARDRAILS: FAIL — 1 copy/copies drifted … (33 compared)". Mutation B (the-desk's copy deleted): exit 1, "could-not-check: guardrail tick-mode — its anchor line is not present" and "GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared". PASS | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills | exit 0, checked-clean; mutation reddens | exit 0 by hand, "checked-clean: bodies — no violations". Mutation (the capability name in the-desk's Tick mode section replaced by the harness's own backticked durable-watch tool name): exit 1, "banned harness token" and "checked-failed: bodies — 1 violation(s)". check:ci row: the hermetic witness could not execute on this host, so the hand run is supporting evidence only | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references | skip line for tick-contract.md, zero violations from it; mode exit expected to remain 1; the assertion is the DELTA | exit 0, "skipped (declared non-matrix-reference): …/tick-contract.md" and "checked-clean: bindings — no violations". Delta taken on the scratch export with and without the reference: the only difference is that one skip line, +0 violations, exit 0 both ways. The exit differs from the authored "remain 1" because the unrelated pre-existing red (a missing system-demo degradation cell in the Claude Code binding) was fixed on main by #1817; the Expect text is stale, the delta assertion holds. PASS on the delta | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | citation check over the five bodies (manual row; a purpose-written script in scratch) | five citations, five resolving paths, zero grammar restatements; mutation reddens | exit 0: every one of the-desk, intake-desk, worker-desk, pr-review-desk, verify-desk reports "citations=1 other-path-citations=0 resolves=yes grammar-restatements=0". Mutation A (grammar line appended to worker-desk): exit 1, grammar-restatements=1. Mutation B (pr-review-desk's link re-pointed to a wrong relative path): exit 1, citations=0 other-path-citations=1. PASS | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | bash plugins/assay/scripts/tick-summary.test.sh | exit 0, every named case passes hermetically | exit 0 by hand, "71 passed, 0 failed", including "ok   the checker shells out to no network or forge tool". check:ci row: the hermetic witness could not execute on this host, so the hand run is supporting evidence only | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome | exit 0; both rejections hold; mutation reddens | exit 0: "2 passed, 0 failed" then "3 passed, 0 failed". Mutation A (outcome alternation in the checker's grammar widened to any lowercase word): rejects-unknown-outcome exit 1, "1 passed, 2 failed". Mutation B (the could-not-check requires swept=- test disabled): rejects-zero-for-unknown exit 1, "0 passed, 2 failed", "ACCEPTED a line the contract forbids: tick role=the-desk outcome=could-not-check swept=7 …". PASS (witness also pass, exit 0) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | git diff refs/remotes/origin/main -- plugins/assay/skills/ (Liveness contract, loop and boot sections of the five bodies) | exit 0 / empty | exit 0, 0 lines by hand (HEAD equals origin/main on the merged tree, so the literal diff is empty by construction). Against the implementing change (squash 21a3e1474 vs its parent): 5 files changed, 130 insertions, 0 removed lines, one hunk of 26 added lines per body; the Liveness contract block hashes identically before and after that commit in all five bodies. Later unrelated commits have since edited those blocks; that is outside this brief. check:ci row: the hermetic witness could not execute on this host, so the hand run is supporting evidence only | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | ONLINE LANE — one pr-review-desk tick pass on a cell against a forced-actionable queue, under the image's own timeout | completes inside the deadline; last stdout line satisfies the grammar; outcome=ok, acted at least 1 | UNRUN — could-not-check. Needs a cell running the loop image; this repo has no offline runner for a skill body and the verifier's envelope bars live infrastructure. Not a pass, not a fail | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | ONLINE LANE — the row-9 transcript grepped for any durable-wake arming, scheduled wake-up, cadence sleep or human prompt | zero hits | UNRUN — could-not-check. Needs the row-9 transcript from a cell run, which does not exist. Not a pass, not a fail | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60 | summary line printed strictly before 60 s; reserve honoured; subagent deadlines bounded | UNRUN — could-not-check. Needs a cell run. The witness token for this row reads pass, but it executed only the bare variable assignment and is vacuous; it is not evidence of the online property | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | ONLINE LANE — row 9 repeated across the eight trigger spellings | first four tick, last four take the window path | UNRUN — could-not-check. Needs a cell run. The witness token for this row reads fail exit 2, but it executed only the bare string --tick; it is a parse artifact, not a failure of the online property | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | statusgen --root . --lint; echo $? | 0, LINT: PASS | exit 0 by hand, "LINT: PASS", zero PROBLEM lines (statusgen v1.0.31). One advisory NOTICE names this brief: an ordering-gate note on the stream README's wave prose; advisory, does not affect exit. check:ci row: the hermetic witness could not execute on this host, so the hand run is supporting evidence only | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $? | 0, every deliverable routed follow-up desk-containers/08; exit 2 is could-not-check, reported as itself | exit 2 on the literal command, reported AS ITSELF: "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against cf31c32418ba…, so this run carries no evidence about its claims". The tool's own merged-brief recipe, run on a scratch clone at the implementing commit with its parent as base: exit 0, "summary: 4 corroborated, 0 disproved, 5 unchecked" — all four deliverable entries CORROBORATED on the follow-up disposition; the five unchecked entries are the out-of-scope reviewer-judgement entries. Passes on substance via the recipe; the literal row is could-not-check; check:ci hermetic witness could not execute on this host | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness** — statusgen verifyrun --brief (this brief) --dry-run, statusgen v1.0.31, exit 2, nothing written: 3 of 14 rows read pass, 1 reads fail, 10 could-not-run.

- Rows 1, 3, 6, 8, 13, 14: could-not-run, exit "-" — "check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses unshare --net, a Linux facility, and this host is darwin".
- Rows 2, 9, 10: could-not-run, exit 127 — the Command cell holds prose, not a command.
- Row 5: could-not-run — "prose-led-command: not executed; the first span … is a .md file name, not a command. Mark the command with a cmd: code span" (new wording since the prior run; same substance).
- Row 4: pass, exit 0 (previously "fail exit=1"; changed because the mode now exits 0, see row 4).
- Row 7: pass, exit 0.
- Row 11: pass, exit 0 — vacuous (bare variable assignment). Row 12: fail, exit 2 — parse artifact (bare --tick). Neither is evidence.

**What changed since the 2026-09-27 verdict.** The brief, the guardrail source, the reference, the checker and its suite hash identically to the prior record. The five desk bodies and the stream README changed through later unrelated commits; the tick-mode copies still byte-match the source (row 2, 33 compared). The only row whose observed result moved is row 4: exit 1 became exit 0 after #1817 added the missing system-demo degradation cell. The six check:ci rows are in the same state as before: they pass by direct run and still lack the hermetic witness on this host. The cited blocker #1491 is still OPEN (last updated 2026-10-01). #2038 changed the Linux network-off sandbox (loopback inside it); it does not change what a darwin host can execute. Rows 9 to 12 are still owed to a cell.

**Findings.**
1. No implementation defect observed: 10 of 14 rows pass on substance by hand (rows 1 to 8, 13, 14), 0 fail, 4 are UNRUN online-lane rows.
2. Check-definition gaps in the Verify table, unchanged from the prior run: rows 2, 5, 9, 10, 11, 12 carry prose in the Command cell, so the witness cannot execute them (rows 11 and 12 yield a vacuous pass and an artifact fail). The witness now names the remedy for row 5 itself (a cmd: code span).
3. Row 4's Expect ("the mode's overall exit to remain 1") is stale: the named pre-existing red no longer exists on main. The delta assertion the row actually makes still holds.
4. Row 14's literal command can only ever exit 2 on a merged tree; corroboration needs the merged-brief recipe.

**Risk-bearing values** — enumeration over the implementing change's shipped files (the reference, the checker, the tick-mode guardrail block and its five copies); all literals re-located at the merged head, files unchanged since the prior run:
- R (exit reserve default) = 60 seconds @ plugins/assay/references/tick-contract.md:99
- ASSAY_TICK trigger literal = "1", exact match @ plugins/assay/references/tick-contract.md:40
- OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68
- COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67
- ROLES = "the-desk intake-desk worker-desk pr-review-desk verify-desk" @ plugins/assay/scripts/tick-summary.sh:56
- VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52
- Formulae with no scalar of their own (stop dispatching once D − E < R + W; subagent deadline D − E − R) @ plugins/assay/references/tick-contract.md:96-100
- Not set by this change: the 480 s / 540 s / 1 h deadline facts (operator values; the brief sets no number in any manifest).

Ranking by irreversibility: every entry is undone by an edit and a redeploy; the brief is irreversible: no, gate: model, and the change moves no funds, publishes nothing and deletes nothing. By harm before detection: (1) ASSAY_TICK = "1"; (2) COUNT and OUTCOMES; (3) R = 60. ROLES and VERSION are identifiers and rank last.

RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact string equality with "1" is the only rule under which no inherited truthy value (true, yes, 0, the empty string) arms tick mode; it follows the repo's own precedent at statusgen/telemetry.go:65 (os.Getenv(...) == "1"). Runtime proof is online row 12, UNRUN here.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59 — the three-state instrument rule made un-spellable otherwise: "-" is the only spelling of an unknown count, so an unfinished sweep cannot claim 0, and the closed four admit nothing outside checked-clean / checked-failed / could-not-check. Both row-7 mutations redden, so both literals are load-bearing.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — R must cover the exit steps (continuity-record update, claim release, the summary line), which include forge writes; whether 60 s suffices depends on write latency on the cell's network, which cannot be measured offline. Online row 11 is the test. Reversible operator default.

VERIFY: BLOCKED — unchanged from 2026-09-27. Rows 1 to 8, 13 and 14 pass on substance by hand and 0 rows fail, but the six check:ci rows (1, 3, 6, 8, 13, 14) still lack the hermetic Linux network-off witness (#1491, open), and rows 9 to 12 are UNRUN online-lane rows that need a cell tick pass against the loop image. Status stays implemented.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Ran against merged main ade741f44372ccdc0986c2b0973f621745fbced7 from a detached worktree at that head, by assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian). Host: darwin/arm64, offline envelope (KUBECONFIG=/dev/null), go 1.27.1, statusgen v1.0.31. Every row was run by hand as authored. Mutation halves were applied inside the verifier's own worktree and reverted with git checkout after each run; the tree was clean apart from the verifier's untracked scratch directory before and after.

**Why this re-run.** The last receipt (blocked at cf31c3241) went stale because the guardrail source, the stream README and six skill bodies changed on main. None of those changes touches the tick-contract text this brief verifies. The tick-mode guardrail block is unchanged from cf31c3241. The Tick mode section hashes identically in all five desk bodies at cf31c3241 and at ade741f44. The guardrail diff is confined to the cross-desk hand-offs (comms) block. The README diff is one board-row flip for brief 10. The skill-body diffs are the comms/cellctl wiring (#2126, #2141), receipt recovery (#2059) and the pr-review-desk scan-refusal rule (#2062). The reference, the checker and its suite are byte-unchanged since the implementing commit 21a3e1474.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | cd tools/skillslint && go run . --root ../.. | exit 0, all verdict lines clean | exit 0. "SKILLSLINT: PASS — 14 skill file(s)"; "HIDDEN-CHARS: PASS — 24 instruction file(s)"; "HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/, no proper-name-shaped token in a driver position"; "GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"; ENFORCEMENT-BLOCK: PASS; POSIX-TOKEN: PASS. PASS by hand. The hermetic check:ci witness could not execute on this host (see witness), so this hand run is supporting evidence | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | the row-1 command, reading the GUARDRAILS verdict | exit 0, checked-clean, tick-mode compared at all five sites; the mutation reddens | exit 0, "GUARDRAILS: PASS — 33 … byte-match". The tick-mode block declares exactly five sites, one per desk body, and each body has one Tick mode heading. Mutation A: one word was changed in each body's copy in turn (all five, one at a time). Each run exited 1 with "GUARDRAILS: FAIL — 1 copy/copies drifted … (33 compared)". That proves all five sites are compared. Mutation B deleted verify-desk's copy: exit 2, "could-not-check: guardrail "tick-mode" — its anchor line is not present" and "GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared". PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills | exit 0, checked-clean; the mutation reddens | exit 0, "checked-clean: bodies — no violations". Mutation: in pr-review-desk's Tick mode section, the durable-monitor capability name was replaced with the harness's own backticked durable-watch tool name. Result: exit 1, "banned harness token … Claude Code durable-watch tool" and "checked-failed: bodies — 1 violation(s)". PASS by hand; the hermetic check:ci witness could not execute on this host | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references | skip line for tick-contract.md; it adds zero violations; the assertion is the DELTA against the base run | exit 0, "skipped (declared non-matrix-reference): ../../plugins/assay/references/tick-contract.md — harness-neutral invocation contract …" and "checked-clean: bindings — no violations". Base delta: the same binary over the references tree exported from 21a3e1474^ gives exit 1, "checked-failed: bindings — 11 violation(s)", standing-note.md among them. That red was pre-existing; standing-note.md has since been declared non-matrix (#1293). The new file adds only its skip line and +0 violations. The Expect's "overall exit remains 1" is stale because the pre-existing red is gone. The delta assertion holds. PASS on the delta (witness also pass, exit 0) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | citation check over the five bodies (manual row; a purpose-written script in the verifier's scratch directory) | five citations, five resolving paths, zero grammar restatements; the mutation reddens | exit 0, "row5: citations=5/5 resolving=5/5 restatements=0". The check counts exactly one markdown link to ../../references/tick-contract.md per body, tests that the path resolves from the body's own directory, and greps for the "tick role=… outcome=" grammar line. Mutation A appended the grammar line to worker-desk: exit 1, "FAIL worker-desk: grammar line restated". Mutation B pointed worker-desk's link at a misspelled path: exit 1, "FAIL worker-desk: 0 citations", citations=4/5. PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | bash plugins/assay/scripts/tick-summary.test.sh | exit 0, every named case passes hermetically | exit 0, "71 passed, 0 failed". PASS by hand; the hermetic check:ci witness could not execute on this host | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome | exit 0; both rejections hold; the mutation reddens | exit 0: "2 passed, 0 failed" then "3 passed, 0 failed". Mutation A disabled the "could-not-check requires swept=-" test at tick-summary.sh line 112: rejects-zero-for-unknown exits 1, "0 passed, 2 failed". Mutation B widened the outcome alternation in GRAMMAR (line 68) to any letter/hyphen word: rejects-unknown-outcome exits 1, "0 passed, 3 failed". PASS (witness also pass, exit 0) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | git diff refs/remotes/origin/main -- plugins/assay/skills/ (Liveness contract, loop and boot sections of the five bodies) | exit 0 / empty | exit 0 with 0 lines (the local origin/main ref is e0daa83a2; no skill body differs between it and ade741f44). Against the implementing squash 21a3e1474 vs its parent: five bodies with 26 insertions each, 0 deletions, one hunk per body. The Liveness contract (binding) and Boot / Boot sequence sections hash identically before and after that commit in all five bodies. Later, unrelated commits have since refactored the Liveness block into desk-common.md (#1597). Those blocks are identical at cf31c3241 and ade741f44, and that refactor is outside this brief. PASS by hand; the hermetic check:ci witness could not execute on this host | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | ONLINE LANE — one pr-review-desk tick pass on a cell against a forced-actionable queue, under the image's own timeout | completes inside the deadline; the last stdout line satisfies the grammar; outcome=ok, acted at least 1 | UNRUN — could-not-check. It needs a cell running the loop image. This repo has no offline runner for a skill body, and the offline envelope bars live infrastructure. Not a pass, not a fail | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | ONLINE LANE — the row-9 transcript grepped for durable-wake arming, scheduled wake-ups, cadence sleeps or human prompts | zero hits | UNRUN — could-not-check. There is no row-9 transcript, because no cell run exists. Not a pass, not a fail | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60 | summary line printed strictly before 60 s; the reserve is honoured; subagent deadlines are bounded | UNRUN — could-not-check; it needs a cell run. The witness token reads pass exit 0, but it executed only the bare variable assignment, so it is vacuous and not evidence | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | ONLINE LANE — row 9 repeated across the eight trigger spellings | the first four tick; the last four take the window path | UNRUN — could-not-check; it needs a cell run. The witness token reads fail exit 2, but it executed only the bare string --tick. That is a parse artifact, not a failure of the online property | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | statusgen --root . --lint; echo $? | 0, LINT: PASS | exit 0, "LINT: PASS" (statusgen v1.0.31). The only NOTICE line concerns windows-port briefs, not this brief. PASS by hand; the hermetic check:ci witness could not execute on this host | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $? | 0, every deliverable routed follow-up desk-containers/08; exit 2 is could-not-check, reported as itself | The literal command exits 2, reported AS ITSELF: "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against ade741f44…". The tool's merged-brief recipe was then run with the worktree detached at 21a3e1474 and --base 21a3e1474^. It exited 0, "summary: 4 corroborated, 0 disproved, 5 unchecked". All four deliverable entries are CORROBORATED on the follow-up disposition, and the five unchecked entries are the out-of-scope reviewer-judgement ones. The worktree was returned to ade741f44 afterwards. A wider --base 21a3e1474^ run against ade741f44 exits 1, DISPROVING the pr-shepherd out-of-scope entry. That only reflects later, unrelated pr-shepherd commits inside the wider range; it is not this change. Passes on substance via the recipe; the literal row is could-not-check | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness**: statusgen verifyrun --brief (this brief) --timeout 300s, statusgen v1.0.31, exit 2. The witness was written into the brief's Evidence section in the verifier's worktree, at a clean tree: the verifier's untracked scratch directory was excluded through a per-process core.excludesFile, so the runner reads @ ade741f44372 with no +dirty. The witness table as written:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 2 | `GUARDRAILS:` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | pass exit=0 | sha256:201f2e06be53 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=- — prose-led-command: not executed; the first span ../../references/tick-contract.md is a .md file name, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 | sha256:165f6e5d57e5 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 11 | `ASSAY_TICK_DEADLINE=60` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 12 | `--tick` | fail exit=2 | sha256:e2aee07e1822 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |

**statusgen verifyrun --check (this brief), exit 2:** "3 pass, 1 fail, 10 could-not-run/missing (of 14 Verify rows)". Rows 4 and 7 pass, and row 11 is a vacuous pass. Row 12 is an artifact fail. Rows 1, 3, 6, 8, 13 and 14 could not run because the check:ci network-off sandbox needs Linux unshare --net and this host is darwin. Rows 2, 9 and 10 could not run because the Command cell holds prose (exit 127). Row 5 could not run as a prose-led command. This is the same shape as the 2026-10-02 dry-run.

**Findings.**
1. No implementation defect was observed. Rows 1 to 8, 13 and 14 pass on substance by hand, including every mutation half. 0 rows fail. Rows 9 to 12 are UNRUN online-lane rows.
2. The Verify table has check-definition gaps, unchanged from the prior runs. Rows 2, 5, 9, 10, 11 and 12 carry prose in the Command cell, so the witness cannot execute them. Rows 11 and 12 produce a vacuous pass and an artifact fail.
3. Row 4's Expect ("overall exit remains 1") is stale; the delta assertion holds.
4. Row 14's literal command can only exit 2 on a merged tree.

**Risk-bearing values.** The enumeration covers the implementing change's shipped files: the reference, the checker, the tick-mode guardrail block and its five copies. Every literal was re-located at ade741f44, and these files are byte-unchanged since 21a3e1474.
- R (exit reserve default) = 60 seconds @ plugins/assay/references/tick-contract.md:99
- ASSAY_TICK trigger literal = "1", exact match @ plugins/assay/references/tick-contract.md:40
- OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68
- COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67
- ROLES = "the-desk intake-desk worker-desk pr-review-desk verify-desk" @ plugins/assay/scripts/tick-summary.sh:56
- VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52
- Not set by this change: the 480 s / 540 s / 1 h deadlines, which are operator values.

Ranking by harm before detection: every entry is undone by an edit and a redeploy (irreversible: no; gate: model). The order is (1) ASSAY_TICK = "1"; (2) COUNT and OUTCOMES; (3) R = 60. ROLES and VERSION are identifiers and rank last.

RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality with "1" is the only rule under which no inherited truthy value (true, yes, 0, the empty string) arms tick mode. It matches the repo precedent at statusgen/telemetry.go:65 (os.Getenv(...) == "1"). The runtime proof is online row 12, which is UNRUN.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59 — together they enforce the three-state rule. "-" is the only spelling of an unknown count, and the outcome set is closed. Both row-7 mutations redden, so both literals are load-bearing.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — R must cover the exit-path forge writes (claim release, continuity update, summary line). Whether 60 s is enough depends on cell network latency, which cannot be measured offline. Online row 11 is the test. This is a reversible operator default.

VERIFY: BLOCKED — the state is unchanged from 2026-10-02, and 0 rows fail. Rows 1 to 8, 13 and 14 pass on substance by hand. Two things are still owed. First, the six check:ci rows (1, 3, 6, 8, 13, 14) lack the hermetic Linux network-off witness: blocker medici-finance/assay#1800 is still OPEN (the darwin verify host has no `unshare --net`; read 2026-10-04). Second, rows 9 to 12 are UNRUN online-lane rows that need a cell tick pass against the loop image. Status stays implemented.

### Non-implementer verifier re-run — 2026-10-15, merged main 0bc69f95c6f8727513024b25f502d0009ca65518

Verification-Attestation: medici-finance/assay#2989 run=2de12032dbaaecc824ad96c994decc97f423654a5cbabeb1792167957972d495 source=0bc69f95c6f8727513024b25f502d0009ca65518 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=any

Run from a detached worktree at the attested source (admitted by the pre-work dispatcher check, exit 0). Host: darwin/arm64, offline envelope (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local), go 1.27.2, git 2.56.0, statusgen v1.0.42. statusgen ran under a throwaway HOME outside the worktree; the shell suite's TMPDIR pointed outside the worktree. Every mutation half ran on a git-archive export of the source tree outside the worktree, never in the worktree, and each mutated file was restored and the clean result re-confirmed. The worktree had zero porcelain lines before and after every row. Delivering change: medici-finance/assay#1072, squash commit 21a3e1474c617a0a87a38e8453364c792d0000d6 (parent ef7bfc2292b5).

Expected values were written down from the brief text before any row ran: a reference with the non-matrix declaration on line 3; a sixth guardrail block tick-mode with exactly five desk-body sites; one Tick mode section per desk body after its Boot section; a checker plus hermetic suite carrying the named rejection cases; a changelog fragment; the board row at implemented.

| # | Command | Expected | Observed (exit + key output line) | Date | Runner |
|---|---------|----------|-----------------------------------|------|--------|
| 1 | cd tools/skillslint && go run . --root ../.. | exit 0, all verdict lines clean | exit 0. "SKILLSLINT: PASS — 17 skill file(s)"; "HIDDEN-CHARS: PASS — 30 instruction file(s)"; "HOUSE-VALUES: PASS — 45 markdown file(s) under plugins/, no proper-name-shaped token in a driver position"; "GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"; ENFORCEMENT-BLOCK: PASS; POSIX-TOKEN: PASS. Advisory soft-budget NOTICE lines only. PASS by hand; the hermetic check:ci witness cannot execute on this darwin host (see witness note) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | the row-1 command, reading the GUARDRAILS verdict, plus mutations | exit 0, checked-clean with tick-mode compared at all five sites; one-word drift gives FAIL; a deleted copy gives could-not-check | exit 0, "GUARDRAILS: PASS — 33 … byte-match". The tick-mode block declares five sites (the-desk, worker-desk, pr-review-desk, verify-desk, intake-desk); each body has exactly one Tick mode heading. Mutation A, one word changed in each body's copy in turn (five runs): every run exit 1, "guardrail tick-mode has drifted" naming that body and "GUARDRAILS: FAIL — 1 copy/copies drifted … (33 compared)", so all five sites are compared. Mutation B, verify-desk's copy deleted: exit 1, "could-not-check: guardrail tick-mode — its anchor line is not present" and "GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared, 0 drifted". Restored: PASS, exit 0. PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills | exit 0, checked-clean; naming the harness wake tool reddens | exit 0, "checked-clean: bodies — no violations". Mutation: in pr-review-desk's Tick mode copy the durable-monitor capability name replaced by the harness's own backticked durable-watch tool name: exit 1, "banned harness token … Claude Code durable-watch tool" and "checked-failed: bodies — 1 violation(s)". Restored: exit 0. PASS by hand; hermetic check:ci witness owed on this host | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references | the non-matrix-reference line for tick-contract.md is printed; the new file adds zero violations; the assertion is the DELTA | exit 0, prints "(declared non-matrix-reference): ../../plugins/assay/references/tick-contract.md — harness-neutral invocation contract …" and "checked-clean: bindings — no violations". Delta 1 (current tree with and without the reference, plugin-shaped copy outside the worktree): exit 0 both ways, the only output difference is that one line. Delta 2 (same binary over plugins/assay exported at the implementing commit and at its parent): exit 1 both, "checked-failed: bindings — 24 violation(s)" both, outputs identical apart from the one added line. Expect text "overall exit remains 1" is stale on current main (the pre-existing red has since been fixed); the delta assertion holds. PASS on the delta (witness also pass, exit 0) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | citation check over the five bodies (a purpose-written script in the verifier's scratch directory outside the worktree) | five citations, five resolving paths, zero grammar restatements; mutation reddens | exit 0: every body reports "citations=1 all-tick-contract-links=1 resolves=yes grammar-restatements=0"; summary "row5: citations=5/5 resolving=5/5 restatements=0". The restatement grep was positively controlled: it matches the grammar line in the reference itself (line 116). Mutation A (grammar line appended to worker-desk): exit 1, restatements=1. Mutation B (pr-review-desk's link re-pointed to a wrong relative path): exit 1, citations=4/5. Restored: exit 0. PASS | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | bash plugins/assay/scripts/tick-summary.test.sh | exit 0, every named case passes hermetically | exit 0, "71 passed, 0 failed", including "ok   the checker shells out to no network or forge tool". Named cases present: accepts-four-outcomes, rejects-unknown-outcome, rejects-unknown-role, rejects-field-order, rejects-missing-token, rejects-missing-field, rejects-space-in-value, rejects-zero-for-unknown, rejects-unknown-for-completed, rejects-ok-without-act, rejects-refused-with-sweep, check-takes-last-line, check-refuses-empty, regexp-is-the-applied-one, contract-citation, portability. PASS by hand; hermetic check:ci witness owed on this host | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome | exit 0; both rejections hold; widening the checker reddens | exit 0: "2 passed, 0 failed" then "3 passed, 0 failed". Mutation A (outcome alternation in GRAMMAR widened to any lowercase/hyphen word, scratch copy): rejects-unknown-outcome exit 1, "2 passed, 1 failed", "ACCEPTED a line the contract forbids: … outcome=maybe …". Mutation B (the could-not-check requires swept=- test disabled): rejects-zero-for-unknown exit 1, "0 passed, 2 failed". Restored full suite: 71 passed, exit 0. PASS (witness also pass, exit 0) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | git diff refs/remotes/origin/main -- plugins/assay/skills/ (Liveness contract, loop and boot sections of the five bodies) | exit 0 / empty | exit 0, 0 lines (the local origin/main ref, 44054ca37, is one commit ahead of the source with no skill-body change). Against the implementing commit vs its parent: 5 files, 130 insertions, 0 deletions, one 26-line hunk per body. In every body the Liveness contract (binding) section and the Boot / Boot sequence section hash identically before and after that commit, and the new Tick mode heading sits directly after Boot and before the heading that previously followed it (the-desk: The loop; intake-desk: The board; worker-desk: The pool; pr-review-desk: the idle-claim HARD GATE; verify-desk: The loop). PASS by hand; hermetic check:ci witness owed on this host | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | ONLINE LANE — one pr-review-desk tick pass on a cell against a forced-actionable queue, under the image's own timeout | completes inside the deadline; last stdout line satisfies the grammar; outcome=ok, acted at least 1 | could-not-check — needs a cell running the loop image; this repo has no offline runner for a skill body and the verifier's offline envelope bars live infrastructure. Not a pass, not a fail. Re-run: on a cell, the image's tick invocation of pr-review-desk with the tick argument | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 10 | ONLINE LANE — the row-9 transcript grepped for durable-wake arming, scheduled wake-ups, cadence sleeps or human prompts | zero hits | could-not-check — no row-9 transcript exists. Not a pass, not a fail | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 11 | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60 | summary line printed strictly before 60 s; reserve honoured; subagent deadlines bounded | could-not-check — needs a cell run. The witness token reads pass exit 0 but executed only the bare variable assignment, so it is vacuous and is not evidence | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 12 | ONLINE LANE — row 9 repeated across the eight trigger spellings | first four tick; last four take the window path | could-not-check — needs a cell run. The witness token reads fail exit 2 but executed only the bare string --tick; a parse artifact, not a failure of the online property | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 13 | statusgen --root . --lint; echo $? | 0, LINT: PASS | exit 0, "LINT: PASS", 0 PROBLEM lines (statusgen v1.0.42). Two advisory NOTICEs name this brief: row 5's Command cell is prose-led (no cmd: span), and an ordering-gate note on the stream README's wave prose. Neither affects exit. PASS by hand; hermetic check:ci witness owed on this host | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 14 | statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $? | 0, every deliverable routed follow-up desk-containers/08; exit 2 is could-not-check, reported as itself | literal command: exit 2, reported AS ITSELF: "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against 0bc69f95c6f8…". The tool's own merged-brief recipe, on a scratch clone outside the worktree checked out at the implementing commit with --base set to its parent: exit 0, "summary: 4 corroborated, 0 disproved, 5 unchecked" — all four deliverable entries CORROBORATED on the follow-up disposition; the five unchecked entries are the out-of-scope reviewer-judgement entries. PASS on substance via the recipe; the literal row is could-not-check; hermetic check:ci witness owed on this host | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

**Execution witness** — statusgen verifyrun with this brief and --dry-run (nothing written), statusgen v1.0.42, exit 2: rows 4 and 7 pass exit 0; row 11 a vacuous pass; row 12 an artifact fail (exit 2 on the bare --tick); rows 1, 3, 6, 8, 13, 14 could-not-run because check:ci hermetic execution needs the Linux unshare --net network-off sandbox and this host is darwin; rows 2, 9, 10 could-not-run with exit 127 (prose in the Command cell); row 5 could-not-run as a prose-led command. Same shape as the 2026-10-02 and 2026-10-04 runs.

**Deliverable presence at the source head.** Reference present with the non-matrix declaration on line 3. Guardrail block tick-mode present with five sites, its text unchanged since the implementing commit apart from one trailing blank line that a later, unrelated block introduced. Checker, suite and reference byte-unchanged since the implementing commit. The two changelog fragments added by the implementing commit have since been folded into CHANGELOG.md, which carries the entry. Board row 08 reads implemented.

**Findings.** (1) No implementation defect observed: rows 1 to 8, 13 and 14 pass on substance by hand, every mutation half reddens as the brief predicts, 0 rows fail. (2) Check-definition gaps in the Verify table, unchanged: rows 2, 5, 9, 10, 11, 12 carry prose in the Command cell, so the witness cannot execute them (rows 11 and 12 produce a vacuous pass and an artifact fail). (3) Row 4's Expect ("overall exit remains 1") is stale; the delta holds. (4) Row 14's literal command can only exit 2 on a merged tree.

**Risk-bearing values.** Enumeration over the implementing change's shipped files (the reference, the checker, the tick-mode guardrail block and its five copies), every literal re-located at 0bc69f95c:
- R (exit reserve default) = 60 seconds @ plugins/assay/references/tick-contract.md:99
- ASSAY_TICK trigger literal = "1", exact match @ plugins/assay/references/tick-contract.md:40
- OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68
- COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67
- ROLES = "the-desk intake-desk worker-desk pr-review-desk verify-desk" @ plugins/assay/scripts/tick-summary.sh:56
- VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52
- Not set by this change: the 480 s / 540 s / 1 h deadlines (operator values; no manifest number).

Ranking: every entry is undone by an edit and a redeploy (irreversible: no; gate: model). By harm before detection: (1) ASSAY_TICK = "1"; (2) COUNT and OUTCOMES; (3) R = 60. ROLES and VERSION are identifiers and rank last.

RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality with "1" is the only rule under which no inherited truthy value (true, yes, 0, the empty string) arms tick mode; it follows the repo precedent at statusgen/telemetry.go:65 (os.Getenv(...) == "1"). Runtime proof is online row 12, could-not-check here.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59 — the three-state instrument rule made un-spellable otherwise: "-" is the only spelling of an unknown count and the outcome set is closed at four; both row-7 mutations redden, so both literals are load-bearing.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — R must cover the exit-path forge writes (claim release, continuity update) before the summary line; whether 60 s suffices depends on cell network write latency, which cannot be measured offline. Online row 11 is the test. Reversible operator default.

**VERIFY: BLOCKED** — 10 of 14 rows pass on substance by hand (1 to 8, 13, 14, all mutation halves run), 0 fail. Still owed: the hermetic Linux network-off witness for the six check:ci rows (1, 3, 6, 8, 13, 14), which this darwin host cannot execute; and online-lane rows 9 to 12, which need a cell tick pass against the loop image. Status stays implemented.

### Non-implementer verifier re-run — 2026-10-19, merged main 8eb028b49bc5e7d45e470cc25dc8b4f531605238

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | pass exit=0 sandbox=container-netns | sha256:841dcf5186ae | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `GUARDRAILS:` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | pass exit=0 sandbox=container-netns | sha256:38c6aee7f466 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | pass exit=0 sandbox=container-netns | sha256:201f2e06be53 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=- — prose-led-command: not executed; the first span ../../references/tick-contract.md is a .md file name, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | pass exit=0 sandbox=container-netns | sha256:a75223552e32 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 sandbox=container-netns | sha256:165f6e5d57e5 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 11 | `ASSAY_TICK_DEADLINE=60` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 12 | `--tick` | fail exit=2 sandbox=container-netns | sha256:e2aee07e1822 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:80dde6c3aa66 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | fail exit=0 sandbox=container-netns | sha256:a62423807e53 | 2026-10-19 | assay-verifier-app[bot] @ 8eb028b49bc5 (on-behalf-of human:ian) (forge-identity) |

Verification-Attestation: medici-finance/assay#3548 run=a2d6e34799adb75fdbd87ad2d85b8e01fa30c3286b01b0b1374f27d826575c84 source=8eb028b49bc5e7d45e470cc25dc8b4f531605238 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong

**Non-implementer verifier re-run — 2026-10-19, merged main 8eb028b49bc5e7d45e470cc25dc8b4f531605238.** The witness table directly above is the execution witness for this run: statusgen verifyrun, non-dry, in a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47), started with --network none, sandbox=container-netns, the engine's default seccomp profile, GOPROXY=off, GOTOOLCHAIN=local, statusgen and the desk tools built from the attested tree; verifyrun exit 2 (8 pass, 2 fail, 4 could-not-run). This runner follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). By-hand rows ran on darwin/arm64 (go 1.27.2, git 2.56.0, statusgen v1.0.51, offline envelope with KUBECONFIG=/dev/null) in a scratch clone at the same source outside this worktree; every mutation half ran in that clone and was restored, and the clone and this worktree had no porcelain lines apart from this brief afterwards. Admitted by the pre-work dispatcher check (exit 0) before the run, and re-checked after it.

Prior Evidence: the 2026-10-15 re-run at 0bc69f95c6f8 recorded VERIFY: BLOCKED (10 of 14 rows pass on substance by hand, 0 fail; check:ci witness and online rows 9 to 12 owed). Since then the checker gained a multiline-candidate rejection and a pipefail reader fix (medici-finance/assay#3271, medici-finance/assay#3040; the suite now runs 93 cases), row 14 was rewritten by medici-finance/assay#3428 and restored to its pre-amendment text by medici-finance/assay#3508; the Tick mode section hashes identically in all five bodies at both commits, and the reference and the tick-mode guardrail block are unchanged.

By-hand results at the source: row 1 exit 0, all six verdict lines PASS ("GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"). Row 2 exit 0; tick-mode declares exactly five sites, each body has one Tick mode heading; one word changed in each body's copy in turn gives exit 1 and "guardrail tick-mode has drifted … GUARDRAILS: FAIL — 1 copy/copies drifted (33 compared)" for all five; deleting verify-desk's copy gives exit 2, "its anchor line is not present … GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared, 0 drifted"; restored PASS. Row 3 exit 0 "checked-clean: bodies — no violations"; the harness's own backticked durable-watch tool name in pr-review-desk's copy gives exit 1 "checked-failed: bodies — 1 violation(s)". Row 4 exit 0, prints the declared non-matrix-reference skip line for tick-contract.md and "checked-clean: bindings — no violations"; with and without the reference in a plugin-shaped copy, both exit 0 and the only output difference is that skip line. Row 5, a purpose-written citation check: five citations, five resolving, zero grammar restatements, the grammar line positively controlled in the reference; inlining the grammar line into worker-desk or re-pointing pr-review-desk's link each gives exit 1. Row 6 exit 0 "93 passed, 0 failed". Row 7 exit 0 ("2 passed" then "3 passed"); widening the outcome alternation gives exit 1 "ACCEPTED a line the contract forbids … outcome=maybe"; disabling the could-not-check requires swept=- test gives exit 1 "0 passed, 2 failed". Row 8: the literal diff is empty (exit 0); against the implementing commit 21a3e1474c61 and its parent, 5 files, 130 insertions, 0 deletions, the Liveness contract (binding) and Boot sections hash identically before and after in all five bodies, and Tick mode sits directly after Boot. Row 13 exit 0 "LINT: PASS". Row 14: the literal statusgen call exits 2, "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against 8eb028b49bc5…", which the row's own Expect names could-not-check; row 14 is therefore could-not-check on merged main. Diagnostic only, a different command from the row and not the row passing: the tool's delivery-pinned recipe (delivery 21a3e1474c61, base ef7bfc2292b5) exits 0 with "summary: 4 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing", all four deliverable entries corroborated on the follow-up disposition.

Witness rows that did not pass, classified. Row 2 (could-not-run, exit 127): harness defect; the Command cell is prose and verifyrun executed the bare span GUARDRAILS:; passes by hand as above. Row 5 (could-not-run, prose-led): harness defect of the same class; passes by hand. Row 9 and row 10 (could-not-run, exit 127): online-lane rows; verifyrun executed a bare skill name and a prose span; the rows need a cell tick pass against the loop image and stay could-not-check. Row 11 (pass, exit 0): vacuous, verifyrun executed only a bare variable assignment, so it is not evidence; the row stays could-not-check online. Row 12 (fail, exit 2): harness artifact, verifyrun executed the bare string --tick; no deliverable defect; the row stays could-not-check online. Row 14 (fail, exit 0, "expected 1"): harness and tool defect; the command ends in "; echo $?", so its process exit is always 0, and verifyrun read "exit 1" out of the prose Expect cell (the DISPROVED case) as the expectation; by hand the literal statusgen call exits 2, which the row's own Expect says is COULD-NOT-CHECK, reported as itself and never as a pass. Row 14 is could-not-check on merged main, blocked under medici-finance/assay#1915; the delivery-pinned result above is a labelled diagnostic, not the row passing.

Risk-bearing values, re-located at the source: ASSAY_TICK = "1" (exact match) @ plugins/assay/references/tick-contract.md:40; R = 60 seconds @ plugins/assay/references/tick-contract.md:99; OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68; COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67; ROLES @ plugins/assay/scripts/tick-summary.sh:56; VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52. All are reversible by an edit and a redeploy.
RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality is the only rule under which no inherited truthy value arms tick mode; it follows the precedent at statusgen/telemetry.go:65.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES @ plugins/assay/scripts/tick-summary.sh:59 — "-" is the only spelling of an unknown count and the outcome set is closed at four; both row-7 mutations redden, so both literals bear load.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test.

**VERIFY: BLOCKED** — rows 1 to 8 and 13 pass on substance (7 in the witness: rows 1, 3, 4, 6, 7, 8 and 13; rows 2 and 5 by hand; row 11's witness pass is vacuous and not counted), 0 deliverable failures. Still owed: row 14, could-not-check on merged main (exit 2, reported as itself; blocked under medici-finance/assay#1915; the delivery-pinned diagnostic reads 4 corroborated, 0 disproved), and online-lane rows 9 to 12, which need a cell tick pass against the loop image and stay could-not-check. Status stays implemented.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | pass exit=0 sandbox=container-netns | sha256:39bf9a9c2529 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 2 | `GUARDRAILS:` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | pass exit=0 sandbox=container-netns | sha256:38c6aee7f466 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | pass exit=0 sandbox=container-netns | sha256:214a4116c64b | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=- — prose-led-command: not executed; the first span ../../references/tick-contract.md is a .md file name, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | pass exit=0 sandbox=container-netns | sha256:a75223552e32 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 sandbox=container-netns | sha256:165f6e5d57e5 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 11 | `ASSAY_TICK_DEADLINE=60` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 12 | `--tick` | fail exit=2 sandbox=container-netns | sha256:e2aee07e1822 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:2385e135dda9 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | fail exit=0 sandbox=container-netns | sha256:0b684da16855 | 2026-10-20 | assay-verifier-app[bot] @ 74f79b7af5fe (on-behalf-of human:ian) (forge-identity) |

Verification-Attestation: medici-finance/assay#3780 run=bec833763502030d953512f6ad705a39639642f91d5deb4b4c23cfcbaa489a0f source=74f79b7af5fe35f0845321c67cb1709f83147c43 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong

**Non-implementer verifier re-run — 2026-10-20, merged main 74f79b7af5fe35f0845321c67cb1709f83147c43.** The witness table directly above is the execution witness for this run: statusgen verifyrun, non-dry, in a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), started with --network none, sandbox=container-netns, the engine's default seccomp profile, GOPROXY=off, GOTOOLCHAIN=local, statusgen and the desk tools built from the attested tree; verifyrun exit 2 (8 pass, 2 fail, 4 could-not-run/missing of 14; 0 author-added). This runner follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). By-hand rows ran on darwin/arm64 (go 1.27.2, git 2.56.0, statusgen v1.0.52, offline envelope with KUBECONFIG=/dev/null) in a scratch clone at the same source outside this worktree; every mutation half ran in that clone and was restored, and the clone had no porcelain lines afterwards. Admitted by the pre-work dispatcher check (exit 0) before the run, and re-checked after it. This pass exists because medici-finance/assay#3685 repaired the Context `files:` declaration that the declared-input-scope parser refused (medici-finance/assay#3757); at this source the parser accepts the brief and returns its 12 declared paths.

Prior Evidence: the 2026-10-19 re-run at 8eb028b49bc5 recorded VERIFY: BLOCKED (rows 1 to 8 and 13 pass on substance, 0 fail; row 14 could-not-check; online rows 9 to 12 owed). Since then this brief's Context `files:` block was rejoined onto single lines (medici-finance/assay#3685) and unrelated skill-body edits landed; the Tick mode section hashes identically in all five bodies at both commits, and the reference, the checker and the tick-mode guardrail block are unchanged.

By-hand results at the source: row 1 exit 0, every verdict line PASS ("GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"; the conformance-budget line is an advisory NOTICE). Row 2 exit 0; tick-mode declares exactly five sites, each body has one Tick mode heading; one word changed in each body's copy in turn gives exit 1 and "guardrail tick-mode has drifted … GUARDRAILS: FAIL — 1 copy/copies drifted (33 compared)" for all five; deleting verify-desk's copy gives exit 2, "its anchor line is not present … GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared, 0 drifted"; restored PASS. Row 3 exit 0 "checked-clean: bodies — no violations"; the harness's own backticked durable-watch tool name in pr-review-desk's copy gives exit 1 "checked-failed: bodies — 1 violation(s)"; restored clean. Row 4 exit 0, prints the declared non-matrix-reference skip line for tick-contract.md and "checked-clean: bindings — no violations"; with and without the reference in a plugin-shaped copy, both exit 0 and the only output difference is that skip line (the Expect's "overall exit remains 1" no longer describes the tree; the delta is what the row tests). Row 5, a purpose-written citation check: five citations, five resolving, zero grammar restatements, the grammar line positively controlled in the reference; inlining the grammar line into worker-desk or re-pointing pr-review-desk's link each gives exit 1; restored exit 0. Row 6 exit 0 "93 passed, 0 failed". Row 7 exit 0 ("2 passed" then "3 passed"); widening the outcome alternation gives exit 1 "ACCEPTED a line the contract forbids … outcome=maybe"; disabling the could-not-check requires swept=- test gives exit 1 "0 passed, 2 failed"; restored 93 passed. Row 8: the literal diff is empty (exit 0); against the implementing commit 21a3e1474c61 and its parent ef7bfc2292b5, 5 files, 130 insertions, 0 deletions, the Liveness contract (binding) and Boot sections hash identically before and after in all five bodies, and Tick mode sits directly after Boot. Row 13 exit 0 "LINT: PASS", no PROBLEM line. Row 14: the literal statusgen call exits 2, "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against 74f79b7af5fe…, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery", which the row's own Expect names could-not-check; row 14 is therefore could-not-check on merged main. Diagnostic only, a different command from the row and not the row passing: the tool's delivery-pinned recipe (delivery 21a3e1474c61, base ef7bfc2292b5) exits 0 with "summary: 4 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing". The cause is the missing delivery marker; the marker that names the delivery, for the landing step to carry so row 14 can be re-run at the landing commit, is `Verification-Table: delivery=21a3e1474c617a0a87a38e8453364c792d0000d6 base=ef7bfc2292b5f9a6ddf438591bfd6b42d30f00b4 original=ef7bfc2292b5f9a6ddf438591bfd6b42d30f00b4` (quoted here, not asserted as a live marker, because a live marker beside a BLOCKED verdict makes the protected-table check refuse).

Witness rows that did not pass, classified. Row 2 (could-not-run, exit 127): harness defect; the Command cell is prose and verifyrun executed the bare span GUARDRAILS:; passes by hand as above. Row 5 (could-not-run, prose-led): harness defect of the same class; passes by hand. Row 9 and row 10 (could-not-run, exit 127): online-lane rows; verifyrun executed a bare skill name and a prose span; the rows need a cell tick pass of pr-review-desk against the loop image (a forced-actionable queue, then its transcript), which no offline runner can provide; they never ran and stay could-not-check. Row 11 (pass, exit 0): vacuous, verifyrun executed only a bare variable assignment, so it is not evidence; the row needs the same cell tick pass with ASSAY_TICK_DEADLINE=60 and stays could-not-check online. Row 12 (fail, exit 2): harness artifact, verifyrun executed the bare string --tick; no deliverable defect; the row needs the eight trigger spellings driven through a cell tick and stays could-not-check online. Row 14 (fail, exit 0): harness and tool defect; the command ends in "; echo $?", so its process exit is always 0, and verifyrun read "exit 1" out of the prose Expect cell (the DISPROVED case) as the expectation; by hand the literal statusgen call exits 2, which the row's own Expect says is COULD-NOT-CHECK, reported as itself and never as a pass.

Risk-bearing values, re-located at the source: ASSAY_TICK = "1" (exact match) @ plugins/assay/references/tick-contract.md:40; R = 60 seconds @ plugins/assay/references/tick-contract.md:99; OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68; COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67; ROLES @ plugins/assay/scripts/tick-summary.sh:56; VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52. All are reversible by an edit and a redeploy.
RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality is the only rule under which no inherited truthy value arms tick mode; it follows the precedent at statusgen/telemetry.go:65.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES @ plugins/assay/scripts/tick-summary.sh:59 — "-" is the only spelling of an unknown count and the outcome set is closed at four; both row-7 mutations redden, so both literals bear load.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test.

**VERIFY: BLOCKED** — rows 1 to 8 and 13 pass on substance (7 in the witness: rows 1, 3, 4, 6, 7, 8 and 13; rows 2 and 5 by hand; row 11's witness pass is vacuous and not counted), 0 deliverable failures; still owed are row 14, could-not-check on merged main (exit 2) until the delivery marker above is carried and the row re-run at the landing commit, and online-lane rows 9 to 12, which need a cell tick pass against the loop image and never ran. Status stays implemented.


### Non-implementer verifier re-run — 2026-10-21, merged main 610a78107977

Verification-Attestation: medici-finance/assay#3937 run=3e0ef667eded17266828c7b8770a0154646509fc4b323cc52265aa9ee51914b2 source=610a78107977b4b9569545b97ec17e11acfbecfe brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong

Admitted by the pre-work dispatcher check (exit 0) before any row ran. Every row ran in a no-hardlinks clone of the attested source in a fresh mktemp -d dir outside this worktree; this worktree was never written except for this Evidence section. The check:ci rows (1, 3, 6, 8, 13, 14) and rows 4 and 7 base runs ran network-off in a local docker container, image assay-verify:go1.27-py (id sha256:9951b77a257f; go 1.27.2 linux/arm64, git 2.47.3), started with --network none, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null, a copied local module cache, statusgen built inside the container from the attested tree, and a throwaway HOME carrying a copy of the roster file (the first row-13 attempt without a roster printed "LINT: REFUSED configuration" and is discarded as an environment fault, not a result). The check-class mutation halves (rows 2, 3, 5, 7) ran on darwin/arm64 (go 1.27.2) in a second clone at the same source, each mutated file restored with git checkout and the clean result re-confirmed; both clones showed zero porcelain lines afterwards. Delivering change: medici-finance/assay#1072, squash commit 21a3e1474c61 (parent ef7bfc2292b5).

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | exit 0, all verdict lines clean | exit 0 (network-off container); HIDDEN-CHARS, HOUSE-VALUES ("46 markdown file(s) under plugins/"), GUARDRAILS, ENFORCEMENT-BLOCK, ACT-BLOCK, AMBIENT-AUTH, POSIX-TOKEN all PASS; one advisory word-budget NOTICE; output sha256:a3b60acc4b41 | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/skillslint && go run . --root ../..` (reading the GUARDRAILS verdict) | exit 0, checked-clean with tick-mode compared at all five sites; one-word change in one copy gives FAIL; deleting one copy gives could-not-check | exit 0 "GUARDRAILS: PASS — 33 guardrail copy/copies byte-match"; the tick-mode block declares exactly 5 site lines and each body has one Tick mode heading. Mutation: "bounded pass" changed to "limited pass" in each body's copy in turn gives exit 1 and "guardrail tick-mode has drifted … GUARDRAILS: FAIL — 1 copy/copies drifted (33 compared)" for all five; deleting verify-desk's Tick mode section gives exit 1 with "could-not-check: guardrail tick-mode — its anchor line is not present" and "GUARDRAILS: COULD-NOT-CHECK — 1 site(s) unreadable or unlocatable, 32 compared, 0 drifted"; restored exit 0 PASS | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | exit 0 checked-clean; naming the harness wake tool reddens | exit 0 (network-off container) "checked-clean: bodies — no violations", output sha256:38c6aee7f466. Mutation: the capability name in pr-review-desk's copy replaced by the harness's own backticked durable-watch tool name gives exit 1 "checked-failed: bodies — 1 violation(s)"; restored | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | the declared non-matrix-reference line for tick-contract.md is printed and the new file contributes zero violations; the assertion is the delta against the run without it | exit 0 (network-off container) "checked-clean: bindings — no violations", the declared non-matrix-reference line printed for tick-contract.md, output sha256:214a4116c64b. Delta: the same command with tick-contract.md removed from a clone also exits 0, and the only output difference is that one line. The Expect's "overall exit remains 1" no longer describes the tree because standing-note.md now carries its declaration; the delta, which is what the row tests, holds | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `for b in the-desk intake-desk worker-desk pr-review-desk verify-desk; do f=plugins/assay/skills/$b/SKILL.md; if [ "$(grep -c 'references/tick-contract.md' $f)" = 1 ] && [ -f plugins/assay/skills/$b/../../references/tick-contract.md ] && ! grep -q 'tick role=<role> outcome=' $f; then echo "$b ok"; else echo "$b FAIL"; exit 1; fi; done` | exit 0, five citations, five resolving paths, zero restatements; inlining the grammar or a non-resolving path reddens | exit 0, "ok" for all five bodies; the grammar line is positively present in the reference (control). Mutation: inlining the grammar line into worker-desk gives exit 1 "worker-desk FAIL"; re-pointing pr-review-desk's link to a non-existent file name gives exit 1 (citation count 0); restored exit 0 | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | exit 0, every named case passes hermetically | exit 0 (network-off container) "93 passed, 0 failed", output sha256:a75223552e32 | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | exit 0; widening either rule reddens | exit 0 (network-off container) "2 passed, 0 failed" then "3 passed, 0 failed", output sha256:165f6e5d57e5. Mutation: the outcome alternation in GRAMMAR widened to any lowercase word gives exit 1 "2 passed, 1 failed"; the could-not-check requires swept=- guard disabled gives exit 1 "0 passed, 2 failed" with "ACCEPTED a line the contract forbids … outcome=could-not-check swept=0"; restored "93 passed, 0 failed" | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | exit 0 / empty, the window path untouched | exit 0, empty output (network-off container; output sha256:e3b0c44298fc, the empty-input hash). Substance at the delivery: git diff ef7bfc2292b5 21a3e1474c61 on plugins/assay/skills/ is 5 files, 130 insertions, 0 deletions; the Liveness contract (binding) and Boot sections hash identically before and after in all five bodies, and Tick mode sits directly after Boot in each | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | ONLINE LANE — on a cell: one pr-review-desk tick pass against a forced-actionable queue, under the image's own timeout | completes inside the deadline, last line satisfies the row-6 grammar, outcome=ok acted ≥ 1 | could-not-check: needs a cell tick pass against the loop image; this repo has no skill-body runner and the verifier runs offline | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | ONLINE LANE — the transcript of row 9, grepped for durable-wake arming, scheduled wake-ups, cadence sleeps and human prompts | zero hits | could-not-check: needs the row-9 transcript, which only a cell tick pass produces | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60 | summary printed before 60 s; reserve honoured; subagent deadline bounded | could-not-check: needs a cell tick pass against the loop image | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | ONLINE LANE — row 9 repeated with each of the eight trigger spellings | the first four tick; true, yes, 0 and empty take the window path | could-not-check: needs a cell tick pass against the loop image for each spelling | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `statusgen --root . --lint; echo $?` | 0 — LINT: PASS | "LINT: PASS" then "0" (network-off container, statusgen built from the attested tree), zero PROBLEM lines; output sha256:b4cfb62b26c5 | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | 0 with every deliverable on the follow-up disposition; exit 2 is COULD-NOT-CHECK, reported as itself | could-not-check: printed "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against 610a78107977…, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery" then "2" (network-off container), output sha256:01947c43a435. Diagnostic only, a different command and not the row passing: the delivery-pinned form with --delivery 21a3e1474c61 --base ef7bfc2292b5 (host statusgen v1.0.54) exits 0 with "summary: 4 corroborated, 0 disproved, 5 unchecked" | 2026-10-21 | assay-verifier-app[bot] @ 610a78107977b4b9569545b97ec17e11acfbecfe (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Prior Evidence: the 2026-10-20 re-run at 74f79b7af5fe recorded VERIFY: BLOCKED (rows 1 to 8 and 13 pass on substance, 0 fail; row 14 and online rows 9 to 12 owed). Results at this source are unchanged: the reference, the checker, its 93-case suite and the tick-mode guardrail block are byte-stable, and the five Tick mode copies still byte-match the declared source.

Risk-bearing values, enumerated over the delivering diff (reference, guardrail block, five body copies, checker, suite) and re-located at this source: ASSAY_TICK = "1" (exact match) @ plugins/assay/references/tick-contract.md:40; R = 60 seconds @ plugins/assay/references/tick-contract.md:99; OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59, enforced by GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68; COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67; ROLES @ plugins/assay/scripts/tick-summary.sh:56; VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52. All are reversible by an edit and a redeploy (risk: irreversible no).
RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality is the only rule under which no inherited truthy value arms tick mode; it follows the precedent at statusgen/telemetry.go:65.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES @ plugins/assay/scripts/tick-summary.sh:59 — "-" is the only spelling of an unknown count (could-not-check requires it) and the outcome set is closed at four; both row-7 mutations redden, so both literals bear load.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test; routed on medici-finance/assay#1916.

**VERIFY: BLOCKED** — 9 of 14 rows pass (rows 1 to 8 and 13; check:ci rows network-off), 0 fail, 5 could-not-check: row 14 (exit 2, no delivery marker on merged main; blocked under medici-finance/assay#1915) and online-lane rows 9 to 12, which need a cell tick pass against the loop image. Status stays implemented.


### Non-implementer verifier re-pass (citation scope) — 2026-10-21, supersedes the citation scope of the 2026-10-21 VERIFY block at merged main 610a78107977

Verification-Attestation: medici-finance/assay#4004 run=18b58c4e0c9ac7f2005db966929cb8f5ac486421165683636bb274fe1d634824 source=e1952464ea12e220e9c8cf8c02000e2b4b50f9f2 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong

Scope: this re-pass changes citations only. It supersedes the citation scope of the VERIFY block recorded above for merged main 610a78107977 (attestation medici-finance/assay#3937). Every observed result in that block stands, and no row was re-run. The issues read below show no recorded result to be wrong. Admitted by the pre-work dispatcher check (exit 0) before any reading. The admitted source e1952464ea12 differs from 610a78107977 only under statusgen/ (the contexthandoff work and its testdata). The brief, plugins/assay/ (the reference, the checker, its suite and the five skill bodies), tools/skillslint and tools/harnesslint are byte-identical at both commits, so the recorded rows still describe the admitted source. The home was left at its admitted source rather than moved to 610a78107977, because moving it would void the admission binding.

Citation corrections:

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | unchanged from the row above | unchanged: exit 0, and the delta holds. The stale "overall exit remains 1" clause in the Expect is tracked on medici-finance/assay#1920, which lists row 4 as expecting a stale exit 1. A verifier comment there (2026-10-13) also records exit 0 for the exact command | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | ONLINE LANE — on a cell: one pr-review-desk tick pass against a forced-actionable queue, under the image's own timeout | unchanged | unchanged: could-not-check. It needs a cell tick pass against the loop image. Tracked on medici-finance/assay#1920 (online rows 9 to 12 could-not-check in the network-off witness) | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | ONLINE LANE — the transcript of row 9, grepped for durable-wake arming, scheduled wake-ups, cadence sleeps and human prompts | unchanged | unchanged: could-not-check. It needs the row-9 transcript. Tracked on medici-finance/assay#1920 (online rows 9 to 12) | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | ONLINE LANE — row 9 repeated with ASSAY_TICK_DEADLINE=60 | unchanged | unchanged: could-not-check. It needs a cell tick pass. Tracked on medici-finance/assay#1920 (online rows 9 to 12; that issue also lists rows 11 and 12 as cell-split parse artifacts) | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | ONLINE LANE — row 9 repeated with each of the eight trigger spellings | unchanged | unchanged: could-not-check. It needs a cell tick pass for each spelling. Tracked on medici-finance/assay#1920 (online rows 9 to 12). The 2026-10-13 verifier comment there also records a definition defect: the Expect says "the first four spellings tick", but the first spelling is "no trigger". The reference, at plugins/assay/references/tick-contract.md line 46, says "Absent both, the run is a window", and that line is unchanged at this source. So the row needs re-authoring before an online run can discharge it | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | unchanged | unchanged: could-not-check, exit 2, with no Verification-Table marker naming the delivery. Still tracked on medici-finance/assay#1915. That issue lists desk-containers/08 rows 8 and 14, and its 2026-10-20 comment records this same no-marker exit-2 shape | 2026-10-21 | assay-verifier-app[bot] @ e1952464ea12 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test; routed on medici-finance/assay#1916. (Unchanged; the DERIVED lines above for ASSAY_TICK and COUNT/OUTCOMES also stand.)

**VERIFY: BLOCKED** — 9 of 14 rows pass (rows 1 to 8 and 13), 0 fail, 5 could-not-check: row 14 (exit 2, no delivery marker on merged main; blocked under medici-finance/assay#1915) and online-lane rows 9 to 12, which need a cell tick pass against the loop image (blocked under medici-finance/assay#1920; row 12 also needs its Expect re-authored, per that issue). Status stays implemented.

### Non-implementer verifier corrective re-pass (round 3) — 2026-10-22, admitted source e1952464ea12

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `GUARDRAILS:` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | pass exit=0 | sha256:214a4116c64b | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=- — prose-led-command: not executed; the first span ../../references/tick-contract.md is a .md file name, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 | sha256:165f6e5d57e5 | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `ASSAY_TICK_DEADLINE=60` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `--tick` | fail exit=2 | sha256:e2aee07e1822 | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ e1952464ea12+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4004 run=18b58c4e0c9ac7f2005db966929cb8f5ac486421165683636bb274fe1d634824 source=e1952464ea12e220e9c8cf8c02000e2b4b50f9f2 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong

**Corrective re-pass, round 3: 2026-10-22, admitted source e1952464ea12.** The witness table directly above is this pass's execution witness: `statusgen verifyrun`, non-dry, written by the tool itself, with its own Runner cells. It ran on the host, darwin/arm64, with go 1.27.2 and KUBECONFIG=/dev/null. statusgen, deskdispatch, deskpathguard and deskevidence were built from the admitted tree into a scratch directory outside this worktree (`go -C statusgen build -o <scratch>/statusgen .` and `go -C tools/desk build -o <scratch>/ ./cmd/deskdispatch ./cmd/deskpathguard ./cmd/deskevidence`), and that directory came first on PATH. verifyrun exited 2: 3 pass (rows 4, 7, 11), 1 fail (row 12), 10 could-not-run (rows 1, 2, 3, 5, 6, 8, 9, 10, 13, 14). The tool wrote "+dirty" in the Runner cells because this worktree's brief already carries the landed Evidence. The pre-work dispatcher check (exit 0) admitted the pass, and the same check ran again after it.

Why the host: a first attempt in a `--network none` container, with linux builds of the same tools and `--sandbox container-netns`, was refused at admission before any row ran: "statusgen verifyrun: pre-work verifier admission failed; no Verify rows executed: exit status 5: … cannot obtain the verifier GitHub App installation token for medici-finance/assay … executable file not found in $PATH". Admission reads the forge, and a network-off container cannot. On this darwin host the check:ci network sandbox (`unshare --net`) does not exist. So in this lane no check:ci row can get a network-off witness. No blocker issue tracks this lane limit yet.

Witness rows, classified:
- Rows 1, 3, 6, 8 and 13 are could-not-run because this host has no network-off sandbox (see above). This witness does not discharge them.
- Row 14 is could-not-run for the same reason. It is also blocked under medici-finance/assay#1915: there is no delivery marker, and the last run of the literal call exited 2.
- Rows 2 and 5 are could-not-run because verifyrun reads their prose Command cells as a bare `GUARDRAILS:` span and a prose-led `.md` span. This witness does not discharge them.
- Rows 4 and 7 pass.
- Rows 9 and 10 are online-lane rows and could-not-run, blocked under medici-finance/assay#1920.
- Row 11's pass is vacuous: verifyrun ran only a bare variable assignment, so it is not evidence. The row stays online-lane, under medici-finance/assay#1920.
- Row 12's fail (exit 2) is a parse artifact: verifyrun ran the bare string `--tick`, and no deliverable defect is involved. The row stays online-lane, under medici-finance/assay#1920.

Supersessions:
- F3, lines 764 to 794. The row table at lines 772 to 785 was transcribed by hand. verifyrun did not write it, so it is not an execution witness. It stands only as a by-hand record taken at 610a78107977. This pass's witness for the admitted source is the table directly above.
- F4, line 801. The sentence "differs from 610a78107977 only under statusgen/" is false. `git diff --stat 610a78107977 e1952464ea12` reports "152 files changed, 11302 insertions(+), 240 deletions(-)": 87 files under tools/desk, 29 under statusgen, 11 under containers, 9 under docs/streams, 8 other files under docs, 7 under changelog, and STATUS.md. The row inputs for rows 1 to 8 are byte-identical at both commits. `git diff --quiet 610a78107977 e1952464ea12 -- plugins/assay tools/skillslint tools/harnesslint docs/streams/harness-portability` exits 0. The `git rev-parse` object ids match at both commits for plugins/assay (739f1c6ea2f4), tools/skillslint (441d26fda1e5), tools/harnesslint (c76c54ff5ed9), docs/streams/harness-portability/README.md (f42a2a95661f) and this brief (41167dfd4e7f). So the by-hand results for rows 1 to 8 carry to the admitted source. Rows 13 and 14 do NOT carry by identity. Both run statusgen built from the tree, and both statusgen/ and docs/streams/ (which `--lint` reads) differ between the commits. Line 816's "9 of 14 rows pass (rows 1 to 8 and 13)" is therefore superseded: at e1952464ea12, rows 1 to 8 pass on substance and row 13 is owed.
- A2, line 811. Row 12 cites plugins/assay/references/tick-contract.md at line 46. The sentence "Absent both, the run is a window" is at line 47 (`grep -n 'Absent both' plugins/assay/references/tick-contract.md` prints 47 at both commits). The citation is corrected to line 47.
- A3, lines 807 to 812. The Runner cells there read "@ e1952464ea12", but that pass re-ran nothing at e1952464ea12. Those cells name the admitted source of a citation-only pass. The observations they carry come from earlier passes (rows 4 and 14 at 610a78107977), and online rows 9 to 12 have never run.

Risk-bearing values, re-located at the admitted source and unchanged: ASSAY_TICK = "1" (exact match) @ plugins/assay/references/tick-contract.md:40; R = 60 seconds @ plugins/assay/references/tick-contract.md:99; VERSION = "1" @ plugins/assay/scripts/tick-summary.sh:52; ROLES @ plugins/assay/scripts/tick-summary.sh:56; OUTCOMES = "ok noop refused could-not-check" @ plugins/assay/scripts/tick-summary.sh:59; COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67; GRAMMAR @ plugins/assay/scripts/tick-summary.sh:68. All of them can be reversed by an edit and a redeploy.
RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality is the only rule under which no inherited truthy value arms tick mode; it follows the precedent at statusgen/telemetry.go:65.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES @ plugins/assay/scripts/tick-summary.sh:59 — "-" is the only spelling of an unknown count and the outcome set is closed at four; both row-7 mutations redden, so both literals bear load.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test; routed on medici-finance/assay#1916.

**VERIFY: BLOCKED** — at e1952464ea12, 0 deliverable failures. Rows 1 to 8 pass on substance: they ran by hand at 610a78107977, and their inputs are byte-identical at this source. In this pass's witness only rows 4 and 7 pass. Still owed: the six check:ci rows have no network-off witness in this lane; row 13 has no admissible result at this source; row 14 is could-not-check (medici-finance/assay#1915); online-lane rows 9 to 12 have never run (medici-finance/assay#1920). Status stays implemented.

### Non-implementer verifier re-pass (citation scope) — 2026-10-22, cites the lane blocker for the round-3 addendum at admitted source e1952464ea12

Scope: citations only. No row was re-run in this pass, and every observation in the round-3 addendum directly above stands. The pre-work dispatcher check (exit 0) admitted the pass, and the same check ran again after it. This pass supersedes the round-3 sentence "No blocker issue tracks this lane limit yet". The lane limit is now tracked on medici-finance/assay#4010, cause 6 (issue comment 6330447531). That comment says: "`statusgen verifyrun` runs `deskdispatch --check-verifier` before any row. Inside the go1.27-py image that check refuses, so no check:ci row gets an admitted network-off witness." It adds: "A darwin host passes admission but has no `unshare --net`, so check:ci rows come back could-not-run there. Neither local lane can witness check:ci rows." It also lists this brief by name: "desk-containers/08 rows 1, 3, 6, 8 and 13 (Evidence #3941)."

Citations:
- Check:ci rows 1, 3, 6, 8 and 13 are could-not-run. They are blocked under medici-finance/assay#4010, cause 6, quoted above.
- Row 14 has two halves:
  - The lane half: like the other check:ci rows, it has no network-off witness. It is blocked under medici-finance/assay#4010, cause 6. The comment's general statement ("Neither local lane can witness check:ci rows") covers it, but its list of named rows does not include row 14.
  - The check-definition half (no delivery marker, exit 2): it stays blocked under medici-finance/assay#1915.
- Rows 9 to 12 are online-lane rows that have never run. They stay blocked under medici-finance/assay#1920.
- Rows 2 and 5 hit a witness-parse limit: their prose Command cells cannot be executed by verifyrun. A filed issue already covers this. The body of medici-finance/assay#1920 says "Rows 2 and 5: the Command cells are prose, which the witness cannot run". The older verifyrun issue for prose-cell execution, medici-finance/assay#1805, is closed.

**VERIFY: BLOCKED** — at e1952464ea12, 0 deliverable failures. Rows 1 to 8 pass on substance: they ran by hand at 610a78107977, and their inputs are byte-identical at this source. In the round-3 witness only rows 4 and 7 pass. Still owed:
- check:ci rows 1, 3, 6, 8 and 13 have no network-off witness (medici-finance/assay#4010, cause 6);
- row 13 also has no admissible result at this source;
- row 14 is could-not-check, its lane half under medici-finance/assay#4010 and its check-definition half under medici-finance/assay#1915;
- online-lane rows 9 to 12 have never run (medici-finance/assay#1920);
- rows 2 and 5 hit the prose-cell witness-parse limit (medici-finance/assay#1920).

Status stays implemented.
### Non-implementer verifier re-pass (container bracket) — 2026-10-27, admitted source b1d3c0a44a01

A fresh claude-opus-5-5 verifier (tier strong) ran this pass. It did not implement this brief. `deskdispatch --check-verifier` admitted it with exit 0, attestation medici-finance/assay#4643, before any row ran. The pass grounded in the brief and merged main. Between the round-3 admitted source e1952464ea12 and this source, the only deliverable-scope change is in `.claude/guardrails/GUARDRAILS.md` (+5/-1, outside the `tick-mode` block's text). `tick-contract.md`, `tick-summary.sh` and its suite, `tools/skillslint` and `tools/harnesslint` are byte-identical.

Execution witness for the check:ci rows (1, 3, 6, 8, 13, 14): the container admission bracket, which provides the network-off witness the round-3 pass lacked.

1. On the host, `statusgen verifyrun --open-bracket` recorded the home tree digest at 20:53:34Z, using statusgen v1.0.63 checksum-matched against the release.
2. In the container, the rows ran under these settings:
   - the published release runner image `ghcr.io/medici-finance/verify-runner:v1.0.63` (linux/amd64; its digest is the Container-Image line below);
   - `--network none`, a non-root uid, and read-only bind mounts for the home, the repository's git directory and a pre-warmed Go module cache;
   - a tmpfs over the git directory's worktrees folder, and an exec-enabled tmpfs at /tmp;
   - GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly.
3. On the host, `--close-bracket` closed the bracket at 20:57:08Z, and `--bracket-digest` then reported `verdict=match`.

The block directly below is the closed witness file, byte for byte.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/skillslint && go run . --root ../..` | pass exit=0 sandbox=container-netns | sha256:fce317ec668b | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills` | pass exit=0 sandbox=container-netns | sha256:38c6aee7f466 | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash plugins/assay/scripts/tick-summary.test.sh` | pass exit=0 sandbox=container-netns | sha256:a75223552e32 | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git diff refs/remotes/origin/main -- plugins/assay/skills/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 13 | `statusgen --root . --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:372c83f0ad4d | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief assay:assay:desk-containers:08; echo $?` | fail exit=0 sandbox=container-netns | sha256:a82e51c40f4c | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:5af4c9a39564d9634cb1089d5ad22ee79687d28690cd5905982c88f2b4a0b8ed
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4643 run=c19707bcaaf49f960c8541b56059ffe281660bff52f509bc49deb5c14a2135b8 source=b1d3c0a44a01a8fbf65f030c5e08a1e520037e07 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c19707bcaaf49f960c8541b56059ffe281660bff52f509bc49deb5c14a2135b8 source=b1d3c0a44a01a8fbf65f030c5e08a1e520037e07 tree=sha256:5af4c9a39564d9634cb1089d5ad22ee79687d28690cd5905982c88f2b4a0b8ed witness=sha256:3598bb0743b70e7c2085622ecebb593680c2e98dcf3a38cece7b9596e9d91ace opened=2026-10-27T20:53:34Z closed=2026-10-27T20:57:08Z before=receipt after=check-verifier

Host witness for the `check` rows (2, 4, 5, 7, 9, 10), from `statusgen verifyrun --rows 2,4,5,7,9,10` at the same source. The block directly below is the tool's appended witness, byte for byte.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 2 | `GUARDRAILS:` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:0d5053a5f84f | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references` | pass exit=0 | sha256:0114f3b44931 | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 5 | `../../references/tick-contract.md` | could-not-run exit=- — prose-led-command: not executed; the first span ../../references/tick-contract.md is a .md file name, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash plugins/assay/scripts/tick-summary.test.sh --case rejects-zero-for-unknown && bash plugins/assay/scripts/tick-summary.test.sh --case rejects-unknown-outcome` | pass exit=0 | sha256:165f6e5d57e5 | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 9 | `pr-review-desk` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:a3608250984e | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
| 10 | `**ONLINE LANE** — the transcript of row 9, grepped for the arming of any durable wake, any scheduled wake-up, any cadence sleep and any human prompt` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:06add9bcf8be | 2026-10-27 | assay-verifier-app[bot] @ b1d3c0a44a01 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4643 run=c19707bcaaf49f960c8541b56059ffe281660bff52f509bc49deb5c14a2135b8 source=b1d3c0a44a01a8fbf65f030c5e08a1e520037e07 brief=docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md model=claude-opus-5-5 tier=strong


Row-by-row accounting at this source:

- **Row 1** passes: exit 0 in the bracket, and the full lint printed `HOUSE-VALUES: PASS` over 49 markdown files under `plugins/` (supplemental read, no credit).
- **Row 2** cannot run. Its Command cell is prose, so the witness ran the first span `GUARDRAILS:` and got exit 127. Supplemental read, no credit: the row-1 command printed `GUARDRAILS: PASS — 33 guardrail copy/copies byte-match .claude/guardrails/GUARDRAILS.md`. The `tick-mode` guardrail declares five sites (the five desk bodies), and each of the five bodies carries exactly one `## Tick mode` section with the block's text. The row's mutation half was not run.
- **Row 3** passes: exit 0 in the bracket.
- **Row 4** is witnessed `pass exit=0`. The authored Expect still asks for an overall exit of 1, because `standing-note.md` was red at the base. That file now carries a declared non-matrix skip, so the mode exits 0 and the Expect is stale. Supplemental read, no credit: the run printed `skipped (declared non-matrix-reference): ../../plugins/assay/references/tick-contract.md — …` and `checked-clean: bindings — no violations`, so the new file adds zero violations, which is this row's DELTA assertion. The stale exit is already filed on medici-finance/assay#1920.
- **Row 5** cannot run, because its Command cell is prose. Supplemental read, no credit:
  - each of the five desk bodies carries exactly one `../../references/tick-contract.md` citation, and each resolves from that body's directory;
  - none of them contains the `tick role=` summary-line shape.
  The row's mutation half was not run.
- **Rows 6 and 7** pass: row 6 exits 0 in the bracket and row 7 exits 0 on the host.
- **Row 8** passes: exit 0, with empty output (the output hash is the empty-input hash).
- **Rows 9 to 12** are online-lane rows. A network-off verifier cannot run them, and none has ever run.
  - Row 9's and row 10's first spans exit 127 above.
  - Row 11's first span is a bare assignment, which would pass vacuously.
  - Row 12's first span parses as a flag.
  - Rows 11 and 12 were therefore not re-witnessed.
  - Row 12's Expect also contradicts the contract: it lists "no trigger" among the "first four spellings" that tick, but tick-contract.md says that with no trigger the run is a window.
  - All of this is filed on medici-finance/assay#1920.
- **Row 13** passes: `LINT: PASS`, exit 0, in the bracket.
- **Row 14** is COULD-NOT-CHECK, and its own Expect cell says to report that state as itself, never as a pass. The witness records `fail exit=0`, because the trailing `; echo $?` masks the process exit. The printed status is 2, with this message: "COULD-NOT-CHECK: assay:assay:desk-containers:08 is not in the diff against b1d3c0a44a01a8fbf65f030c5e08a1e520037e07, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery."
  - This brief is self-delivered: it first appears in its own delivering squash 21a3e1474c617a0a87a38e8453364c792d0000d6, whose parent ef7bfc2292b5f9a6ddf438591bfd6b42d30f00b4 does not hold it. That leaves no admissible `original` revision for a Verification-Table marker, so this pass records none.
  - Supplemental delivery-pinned run, in the same image outside the bracket and with no credit: `--delivery 21a3e1474c617a0a87a38e8453364c792d0000d6 --base ef7bfc2292b5f9a6ddf438591bfd6b42d30f00b4` gave `summary: 4 corroborated, 0 disproved, 5 unchecked`, exit 0.

Fail-first: no row is risk-bearing.
- The brief-wide risk answers are all no.
- No row carries a risk-bearing, live, mutating or end-to-end Class tag; the tags present are `+mutation`, `+flow` and `+dereference`.

So no fail-first witness is owed. For the record, `--fail-first --base ef7bfc2292b5f9a6ddf438591bfd6b42d30f00b4` refuses: "the base predates the brief: it does not hold docs/streams/desk-containers/brief-08-tick-contract-for-desk-skills.md, which the merge-base does". This is the same self-delivered shape.

Risk-bearing values. I enumerated the literals in tick-contract.md and tick-summary.sh again at this source. VERSION = "1" (:52) and ROLES (:56) are closed-set identifiers. ASSAY_TICK, COUNT/OUTCOMES and R carry load. All the line numbers below were re-read at b1d3c0a44a01.
RISK-VALUE: DERIVED — ASSAY_TICK = "1" @ plugins/assay/references/tick-contract.md:40 — exact equality is the only rule under which no inherited truthy value arms tick mode; it follows the precedent at statusgen/telemetry.go:65.
RISK-VALUE: DERIVED — COUNT = '([0-9]+|-)' @ plugins/assay/scripts/tick-summary.sh:67 and OUTCOMES @ plugins/assay/scripts/tick-summary.sh:59 — "-" is the only spelling of an unknown count and the outcome set is closed at four; row 7 exits 0 on both rejection cases, so both literals bear load.
RISK-VALUE: NAMED, NOT DERIVED — R = 60 @ plugins/assay/references/tick-contract.md:99 — whether 60 s covers the exit-path forge writes depends on cell write latency, which cannot be measured offline; online row 11 is the test; routed on medici-finance/assay#1916.

**VERIFY: BLOCKED** — at b1d3c0a44a01, 0 deliverable failures observed.
- check:ci rows 1, 3, 6, 8 and 13 now carry network-off container witnesses and pass.
- Rows 4 and 7 pass on the host. Row 4's Expect exit is stale (medici-finance/assay#1920).
- Still owed:
  - online-lane rows 9 to 12 have never run (medici-finance/assay#1920);
  - rows 2 and 5 are prose cells that the witness cannot execute (medici-finance/assay#1920);
  - row 14 is COULD-NOT-CHECK, because a self-delivered brief has no derivable delivery. The medici-finance/assay#1915 checkpoint routed this brief as a hold, and no open issue has owned it since medici-finance/assay#1915 closed. A blocker filing is pending.

Status stays implemented.

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
