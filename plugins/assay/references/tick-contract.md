# The tick contract — one bounded pass, when the caller says so

<!-- assay:harnesslint non-matrix-reference — harness-neutral invocation contract for the desk-role loops, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

A desk-role skill is a STANDING loop. Its liveness contract arms a self-scheduled loop before
the first sweep and keeps it ticking for the life of the window, and that is right for a window
a person is sitting in front of. It is wrong for a SCHEDULED run.

A scheduled run — a container job, a cron-fired invocation, anything whose whole execution is
one non-interactive call under an outer time limit — has three properties a standing loop
cannot survive:

- **It is one-shot.** There is no second turn. Whatever the skill is doing when the limit
  expires, it is doing when the process dies.
- **It usually prints at completion, not as it goes.** A killed run then emits nothing at all —
  not a partial transcript, not the work it actually did. The log is empty.
- **It is nested inside harder limits it can only lose a race with** — the job's own deadline,
  and the lifetime of whatever credential it minted at boot.

So a standing loop under a scheduled call has exactly one possible ending: the kill, with no
record of what it did. Raising the time limit does not help — a loop with no terminating
condition spends whatever budget it is given.

This file is the missing mode. It states, once, what a role does when its caller asks for a
single bounded pass instead of a standing window. Every desk-role body carries a short
`## Tick mode` section that cites this file; none of them restates it.

**House values never appear in this file.** The role's sweep instrument, its width, its queue,
the time limit a scheduled caller sets, and the schedule itself are all named by the role's own
body or by the operator — this reference states the mechanism and the shape, never a concrete
value.

## The trigger — two spellings, one predicate

A run is a **tick** when either of these holds. They are an OR with no precedence:

| Spelling | Who can set it | Rule |
|---|---|---|
| the literal argument `--tick` in the skill's argument string | a person at a terminal, or any launcher that can add an argument | present ⇒ tick |
| the environment variable `ASSAY_TICK` | a scheduled job or container definition, which can set environment but often cannot add an argument | compared EXACTLY to `1` ⇒ tick. Any other value — `true`, `yes`, `0`, the empty string — is NOT a tick |

Both exist because the two callers are genuinely different, and a launcher in between may only
be able to do one of them. The exact-match rule on the environment half is the important one: a
loose truthiness test lets an inherited variable convert a live operator window into a one-pass
run, silently, with nobody having asked for it.

Absent both, the run is a **window** and the role behaves exactly as its body already says.
That default is what makes the contract safe to adopt before any caller passes the flag:
until something does, nothing changes.

## The bounded pass

Once, in this order:

1. **Boot.** Identical to the window case. Boot is not what a tick shortens.
2. **Read the budget.** `ASSAY_TICK_DEADLINE`, in whole seconds, when present. When absent the
   tick still runs exactly one pass and simply performs no reserve arithmetic.
3. **ONE fresh sweep** of this role's own queue, with the same named instrument its own
   hard-gate section already requires. Exactly one. A tick never re-sweeps.
4. **Act** on what that sweep made actionable, up to the role's own declared width. No sweep
   follows the acting.
5. **Wait, bounded**, for what this pass dispatched, with the bound taken from the remaining
   budget. Anything still running when the bound expires is reported, not waited on.
6. **Print the summary line** as the last line of output, and **exit**.

### What a tick never does

The negative half, and the enforceable one. In tick mode a role:

- never arms `capability:durable-monitor`, or any other durable cross-turn wake;
- never schedules a wake-up, and never sleeps for a cadence interval;
- never re-sweeps for a second cycle, and never refills a slot a first-pass dispatch vacated;
- never prompts a person or waits in line for an answer — an escalation is a FILED issue and
  the pass continues, which is the file-and-exit contract a scheduled run is already bound by;
- never claims **idle** or **caught up**. One fresh sweep satisfies the hard gate for the pass
  that ran; it licenses no standing claim about the queue. A tick that found nothing reports
  `noop`, which says only "this pass found nothing actionable".

### What is unchanged

Everything else: gates, budgets, stop flags and their precedence, identity rules, filing verbs,
width, and every escalation obligation. **A tick narrows the LOOP, never a GATE.** Under budget
pressure a tick drops WORK — it stops dispatching — and never drops a CHECK.

### What a tick skips at boot

Only the skill's OWN first-boot residue: arming the durable wake, registering the window as the
exclusive holder of a watcher, and any step whose sole purpose is continuity across turns that a
one-shot run does not have. The boot ceremony itself is untouched and runs identically in both
modes.

## Budget arithmetic

Let `D` be `ASSAY_TICK_DEADLINE`, `E` the seconds elapsed since the pass began, `R` the **exit
reserve**, and `W` the role's expected cost of one more unit of work.

- The tick **stops dispatching new work** once `D − E < R + W`.
- `R` covers only the exit path — printing the summary line, updating whatever continuity record
  the role keeps, and releasing any claim this pass took. It defaults to **60 seconds**.
- Each subagent this pass dispatches is given a deadline of `D − E − R`, so nothing this pass
  started outlives the pass that owns it.

**The operator consequence, as arithmetic rather than a preference.** A pass has `D − boot − R`
seconds of working budget. A tick whose unit of work is small (reading a board, filing an issue)
fits in a short `D`; a tick whose unit of work is a full review on a slow model does not. So a
`D` sized for a `noop` pass is a FLOOR, not a target: an operator who wants a tick to actually
do its role's work sets `D` to at least boot + one unit of work + `R`, and raises the job's own
outer deadline above it in the same edit, since that deadline must stay the outer bound. This
file fixes the arithmetic; it sets no number.

## The summary line

Exactly one line, the **last** line of standard output:

```
tick role=<role> outcome=<outcome> swept=<n> acted=<n> filed=<n> duration=<s>
```

| Field | Value |
|---|---|
| leading token | the literal `tick`, so a log grep needs no surrounding context |
| `role` | the desk role, spelled exactly as the skill is named |
| `outcome` | one of `ok`, `noop`, `refused`, `could-not-check` — a closed set |
| `swept` | items the one fresh sweep enumerated, or `-` if unknown |
| `acted` | items this pass dispatched, landed or flipped, or `-` if unknown |
| `filed` | issues filed or attached this pass, or `-` if unknown |
| `duration` | whole seconds from the start of the pass to this line |

Fields appear in that fixed order, single-space separated, `key=value`, unquoted, and no value
may contain a space. **A count that is genuinely unknown is written `-`, never `0`** — a zero is
a measurement claim, and a pass that did not finish looking has not made one.

### The outcomes, and the cross-field rules that keep them honest

| Outcome | When | Then, necessarily |
|---|---|---|
| `ok` | the sweep completed and at least one act landed | `swept` numeric, `acted` numeric and ≥ 1 |
| `noop` | the sweep completed and found nothing actionable | `swept` numeric, `acted` = 0 |
| `refused` | the pass declined to run at all — a stop flag, a kill switch, a tripped budget breaker, an unmet precondition the role refuses on | `swept` = 0, `acted` = 0 |
| `could-not-check` | the sweep did not complete, or completed blind — an instrument exited non-zero, a queue could not be read, a credential could not be minted, or the reserve cut the pass short mid-sweep | `swept` = `-` |

The right-hand column is not decoration: it is what makes the distinction mechanical. `noop` and
`could-not-check` are the two outcomes a reader will most want to confuse, and they mean opposite
things — `noop` says the queue is empty, `could-not-check` says the instrument did not look.
Because a `could-not-check` line must carry `swept=-` and a `noop` line must carry a numeric
`swept`, a blind pass cannot produce a well-formed `noop` line at all. Rounding one to the other
turns a blind loop into a health report; that is the three-state instrument rule
(checked-clean / checked-failed / could-not-check, the third reported as itself) applied to a
single line of text.

### Why a line, and not structured output

A one-shot call emits its output as one blob at completion. A single grep-able last line
survives a truncated or interleaved log, parses with `awk` and no dependency, and is readable by
a person reading the job's log at three in the morning. The line is the **sole machine-readable
verdict**: a tick does not encode its outcome in a process exit status, because the harness owns
that and typically exits 0 for any completed turn regardless of what the turn concluded.

The absence of the line is itself a signal, and the one that matters most: a caller that
receives no `tick …` line knows the pass did not complete. That is precisely the distinction an
empty log cannot otherwise make, since "killed at the deadline" and "healthy but silent" look
identical.

### One implementation of the grammar

The grammar has exactly one executable form, `../scripts/tick-summary.sh`, so a producer, a
parser and a test all read the same rule:

```
tick-summary.sh regexp              # print the published extended regular expression
tick-summary.sh validate '<line>'   # exit 0 iff that one line satisfies the grammar
tick-summary.sh check < <output>    # exit 0 iff the LAST line of the input satisfies it
```

`tick-summary.test.sh` beside it is its hermetic case suite — no network, no credential.

## Boundary

This file is not a scheduler, and adopting it adds none. A tick reuses the role's existing boot,
its existing named sweep, its existing width and its existing filing verbs, and stops after one
pass of them. Nothing here changes how a window behaves, and nothing here passes the trigger:
the caller does that, and until some caller does, every run is a window.

It restates no rule it does not own, either. The file-and-exit contract, the hard-gate sweep
requirement, the stop-flag precedence and the three-state instrument rule all live where they
already lived; this file names them and defers to them. Its own scope is exactly four things:
the trigger predicate, the bounded pass, the budget arithmetic, and the summary line.

Unlike the per-harness binding files beside it, this one carries no freshness entry, because it
tracks no upstream: it describes a contract this bundle defines, not a capability some other
tool's release notes can move.

## Worked example

A review role, woken by a scheduled job with a 900-second limit, finding two pull requests that
need a reviewer and dispatching both:

```
tick role=pr-review-desk outcome=ok swept=14 acted=2 filed=0 duration=612
```

The same role on a quiet queue:

```
tick role=pr-review-desk outcome=noop swept=14 acted=0 filed=0 duration=47
```

The same role when the board could not be read at all — note `swept=-`, which is what makes this
line un-confusable with the one above:

```
tick role=pr-review-desk outcome=could-not-check swept=- acted=0 filed=1 duration=39
```

And the same role with a stop flag armed, which is a refusal rather than a failure:

```
tick role=pr-review-desk outcome=refused swept=0 acted=0 filed=0 duration=8
```
