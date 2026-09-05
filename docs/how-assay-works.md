# How Assay Works

You do not need Assay installed, a repo cloned, or an agent running to read this page.
It teaches one mental model — how a fleet of coding agents can be supervised so their
claims are checkable instead of merely trusted — and every other document assumes you
have it. If you already know why "the agent said it's done" is not evidence, skip to
[the desk loop](#the-desk-loop) for the diagram.

Frame it plainly: **you are learning to run a desk.** Not "how to prompt an agent
better" — how to structure delegation so that when an agent reports success, that
report means something.

## The problem: self-reported "done" is not evidence

Three independent measurements, on developers who already use coding agents daily, say
the same thing three different ways:

- **METR's randomized controlled trial** found experienced developers were **19% slower
  with AI tools while believing they were 20% faster.** Self-perception of productivity,
  including your own, is unreliable — which means it cannot be the thing you check.
- **Stack Overflow's 2025 developer survey** found **66% of developers name "almost right"
  solutions** — code that looks correct but isn't — as their single biggest frustration
  with AI tools. An almost right answer is the failure mode that slips past a glance and
  an optimistic skim.
- A large-scale study of real coding-agent sessions (arXiv:2605.29442, 20,574 sessions)
  found that **~91% of agent misalignments require explicit human correction** — agents
  overwhelmingly do not notice and self-correct their own errors. Supervision is not
  hygiene layered on top of the workflow; it is the workflow.

Put together: an agent asked whether its own work is complete will say yes, confidently,
and that confidence is not correlated with being right. This is not a claim that agents
are unusually dishonest — it is a structural fact about self-report. The actor who did
the work is the weakest possible source for whether the work is correct, because it
shares every blind spot the work itself has. A checker drawn from the same fleet doesn't
fix this either — correlated failure modes ride along. The fix has to change who is
allowed to say "done," not how earnestly they say it.

## Four claims, and the reason for each

Assay is a small set of structural answers to that one problem. Each claim below is
load-bearing — remove it and a specific, named failure comes back.

**1. A brief is a delegation contract, not a task description.** A *brief* is a
self-contained document: enough context to be picked up by a worker who has never seen
the conversation that created it, a scope, typed dependencies on other briefs (never a
prose sentence like "depends on the auth work" — a sentence that stays readable after
the thing it refers to is renamed, without ever becoming true again), and an executable
**Verify table**: concrete commands and the outcomes they should produce. The Verify
table is what "done" means for that brief, written down *before* the work exists, so
nobody negotiates the definition after the fact. Fleet workers are ephemeral — the
session that implements a brief will not exist tomorrow to clarify what it meant, so the
brief has to mean the same thing without them.

**2. The verifier is never the implementer.** This is the direct repair for the
self-report problem above. Every brief moves through a lifecycle:

```
todo → in-progress → implemented → verified → done
```

An implementer stops at `implemented` — it records its own evidence run and does not
grade itself further. A **non-implementer** re-runs the brief's Verify table, on the
merged code, and only that independent run can move a brief to `verified`. Merging a
pull request is not verification: a merged brief sits in an "awaiting verification"
queue until someone distinct from its author actually executes the checks. The
separation is structural — who is allowed to run the check — not a request for people
(or agents) to try harder.

**3. Review checks something different than the Verify table does, and neither
substitutes for the other.** The Verify table proves the work *functions* — does it do
what it claims. A separate review pass judges whether it is *well-built* — is this the
right shape, does it create a mess elsewhere. A change can pass every check and still be
badly designed, and the reverse also happens. `done` requires both: a verified Verify
table and a recorded review verdict, attributed and dated, from an identity the
implementer cannot write on its own behalf.

What the Verify table proves is bounded: it probes the *delta* a brief introduced —
the behaviour that change was supposed to add — chosen by the same author who wrote the
change. The *baseline* the change did not touch is guarded separately, by a standing
truth suite that CI owns and the change's author does not write, under a test policy
that also fixes the tiers, the regression floor, and what a flaky result means
([`test-policy.md`](test-policy.md)). A green Verify table is a claim about the delta,
not about everything the change left alone.

**4. Every instrument reports in three states, never two.** A check that can only say
`pass` or `fail` is forced to lie whenever it can't actually look — a network timeout, a
missing prerequisite, a truncated page of results. Every desk instrument in Assay instead
reports **checked-clean**, **checked-failed**, or **could-not-check**, and the third
state is treated as seriously as a failure: an "I didn't look" reported as "pass" is a
silent hole in the supervision, and it's exactly the kind of hole that lets an
almost-right result through unchecked.

## The desk loop

One diagram, referenced everywhere else rather than redrawn: how an item moves from a
raw idea to done, and who is allowed to do what at each step.

```
  intake ──► next-up ──► fan-out ──► review ──► HUMAN MERGE ──► verify
    │            │           │          │             │            │
    └──────────────────── arbitrated by the-desk ───────────────────┘
```

- **intake** — a raw idea, finding, or request enters a register with a disposition. It
  is not yet work; it is a candidate for work.
- **next-up** — a computed queue picks what to work on next, weighing priority and
  staleness across every stream at once, so effort doesn't collapse onto "whatever is
  closest" for one worker.
- **fan-out** — one or more workers are dispatched, each in its own isolated workspace,
  each owning exactly one brief end to end.
- **review** — a non-author pass judges quality (claim 3 above) before anything merges.
- **HUMAN MERGE** — a human presses merge. Nothing in the loop merges itself; this is a
  deliberate human checkpoint, not an automation gap.
- **verify** — after merge, a non-implementer re-runs the Verify table on the merged
  result (claim 2) and the brief only now becomes `verified`, then `done` once a review
  verdict is also recorded.

**the-desk** arbitrates the whole loop: it watches the queue, dispatches and checks work
across every stream rather than owning just one, and is itself held to the same standard
it enforces — a desk's claims about its own state are never trusted over its own
instruments.

A worked, fictional walk through one turn of the loop: an idea about `example-app`
enters intake, becomes a brief, `human:alex` merges the resulting pull request, and a
different session — never the one that wrote the code — runs the Verify table before the
brief is marked done. Nothing about that shape depends on which product `example-app`
actually is.

### A note on vocabulary (dual-track)

Everything above is described in **capability terms** — what a step *does* — on purpose:
`dispatch-worker` (fan-out sends work to an isolated worker), `isolate-workspace` (each
worker gets its own workspace, never a shared one), `message-agent` (the desk and its
workers exchange messages), `invoke-skill` (a worker loads the instructions for the task
in front of it), and `session-notifications` (the desk learns when dispatched work
finishes). These terms are what stay stable. The concrete mechanism behind each one
depends on which coding-agent harness you're driving — this document names none of them
in the prose above so nothing here goes stale when a harness changes its tool names.
For readers who want the concrete mapping today:

| Capability | Claude Code | Codex | Cursor |
|---|---|---|---|
| `dispatch-worker` | a backgrounded subagent | a spawned agent process | a background agent in its own git worktree |
| `message-agent` | a message sent to that subagent | a message sent to that process | a follow-up sent to that agent |
| `isolate-workspace` | its own git worktree, never the shared checkout | its own sandboxed workspace | its own git worktree, never the shared checkout |
| `invoke-skill` | a `SKILL.md` loaded by description match | an equivalent on-demand instruction file | a `SKILL.md` loaded by description match (same open standard) |
| `session-notifications` | a completion notification when the subagent finishes | an equivalent completion signal | a structured completion signal when the agent finishes |

No column is the method; the capability row is. The table started with one column and
now has three — each new harness is a new column, and the loop above is unchanged.

## Where to next

This page is the mental model. It deliberately does not walk you through an install or
a first command — that belongs to the hands-on tutorial, which teaches the same loop by
doing rather than by reading. If the tutorial has not shipped yet where you're reading
this, the reference docs cover the same ground in more depth and can be read standalone:

- the brief lifecycle and the generated status board, in full
- the append-only registers (findings, intake, retro) and why deletions are
  machine-visible
- the three-state instrument rule this page's claim 4 draws from
- the adopter install runbook, if you're ready to bring Assay into a repo

None of them require you to have read this page first, but this page is the fastest way
to make the rest of them make sense on the first pass.
