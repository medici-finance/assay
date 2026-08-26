# The scheduler is not the model's job: draining agent work without stalling

**Assay · by Medici** — August 2026

We run software work through fleets of agents. A desk role reads a queue of items, holds
several workers in flight, dispatches each to an agent, lands the result, and refills the
slot the moment one finishes. For months the outer loop of that role lived in the operator
model's attention: the same model that judged each item also tracked which slots were full,
which items were claimed, when to refill, and when the queue was empty. It stalled, and the
way it stalled is a general finding about agent systems, not a quirk of ours. This article
is about the finding, the fix, and a small piece of code you can clone and run in your own
stack. The Medici internals are the case study. The pattern is the takeaway.

## The failure: orchestration collapses into attention

Give one model two jobs — decide the next item, and keep the whole schedule in your head —
and under load the schedule is what it drops. Attention is a fixed budget. Every slot of it
spent remembering "worker three is still running, worker one landed, refill it, don't
re-dispatch item nine, the queue had eleven left" is a slot not spent on the judgment the
item actually needs. As the pool grows the bookkeeping grows with it, the model's attention
does not, and the loop degrades in the way a single-threaded process degrades when you pile
concurrent state onto it. It forgets a refill and the pool runs half-empty. It re-reads the
queue and re-dispatches an item already in flight. It declares the queue drained while three
workers are still out. None of these is a reasoning error inside an item. They are scheduling
errors, and the model made them because we asked one attention budget to carry both the
reasoning and the schedule.

We measured the two sharpest versions. The same brief was implemented twice, eighty-one
minutes apart, because the "don't re-dispatch a claimed item" rule lived as prose the model
was supposed to hold, and under a full pool it did not hold it. Separately, a loop reported
itself idle while work was still in flight, because "is the queue empty" and "are the workers
done" are two different facts and the model was tracking only one. Both are scheduling facts.
Neither is judgment. Both belong in code.

## The fix: move the scheduler out of the model and into deterministic code

The correction is a separation of concerns, and it is almost boring once stated. The
scheduler is a deterministic state machine: read the queue, hold a pool of N, claim before
dispatch, land each result as it returns, refill, poll for idle. None of that is a judgment
call. All of it is exactly the kind of thing code is good at and attention is bad at. So we
wrote the scheduler as code — a small Go conductor — and left the model exactly one job per
item: the judgment inside the dispatched agent.

The division is clean. The scheduler decides *which dispatch to make and when*; the agent
decides *what the answer is*. The scheduler never reasons about an item's content, and the
model never tracks a slot, a claim, or a refill. In our interim form the conductor does not
spawn the agent itself — there is no in-process binding to the agent runtime yet — so it
emits an exact dispatch instruction, the operator executes that one instruction verbatim,
and feeds a structured result back. Even then, zero scheduling state lives in the model's
attention. One iteration reads: the engine says exactly which dispatch to make; make it;
hand the result back. The whole win — the outer loop leaving the model's attention — is
already bought, whether the spawn is a human-relayed instruction today or a native child
process later. That upgrade swaps one method and touches nothing else, because the scheduler
never knew or cared how the dispatch was carried out.

## The contract: six methods, and why it stays small

A scheduler that is going to serve more than one desk role needs a contract, and the
contract is where this kind of project usually goes wrong — every new role wants "just one
more hook," and a year later the engine is a second place where per-role policy lives. We
froze the contract at six methods and made adding a seventh a design review rather than a
patch. An adapter implements:

- **select** — produce the queue of typed items to consider. The engine calls it; it does
  not know where items come from.
- **claim** — take an item before dispatching it, so no two dispatchers can pick up the same
  item concurrently. Claiming is the single chokepoint the dedupe guarantee rides on; a
  dispatcher that skips it is outside the guarantee by construction.
- **tier** — map an item to a dispatch destination (which class of runner, or "not
  dispatchable, route to a human"). Tier is a policy hook, so per-role routing lives here and
  not in the engine.
- **dispatch** — carry out one dispatch and return a handle. This is the one method that
  differs between "emit an instruction" today and "spawn a process" later.
- **land** — record one finished result: write the evidence, release the claim, and let the
  drain continue. All per-role heterogeneity that is not tiering lives in land.
- **idle** — decide what "the queue is empty" means for this role and what to do about it
  (poll again, or stop).

Everything that varies between roles is pushed into **tier** and **land**. Everything else —
the pool arithmetic, the claim/refill dance, the drain-continues-on-failure discipline — is
the engine's, identical for every role, written once. The contract stays small because the
moment it grows a hook to accommodate one role, that hook propagates to every consumer, and
the engine stops being a scheduler and starts being a junk drawer of policy. Keeping it at
six is not minimalism for its own sake; it is the property that lets a second and third role
adopt the engine without the first role's decisions leaking into theirs.

Two disciplines the contract enforces rather than requests. A dispatch that fails does not
abort the drain — the engine lands it as blocked, releases the claim, and refills the slot,
so one bad item cannot freeze the pool. And failure has three outcomes, not two: retry it,
refuse it, or *cannot classify it*. A retry/no-retry pair has to guess on a failure it does
not recognise, and both guesses are defects — loop forever on an unrecognised refusal, or
silently drop a transient one. The engine instead lands an unrecognised failure as
needs-context and routes it to a human, which is the same three-state honesty the rest of our
tooling uses: checked-clean, checked-failed, could-not-check.

## Case study: draining a Canton delivery pipeline

The loops this engine drives are not abstract. One family of them runs our Canton pipeline —
the path a Daml model takes from a merged change to a converged, verified ledger. A change
lands, a model compiles to a DAR, the DAR ships, the ledger has to accept it, and a GitOps
reconciler has to converge the cluster onto the new state. Each of those is an item in a
queue, and each item's judgment — did the ledger actually accept this package, did the
reconciler actually reach the declared state, or is it reporting progress it has not made —
is real work an agent does per item.

Here the separation earns itself twice over. Verifying a Canton deployment is exactly the
kind of task where the tempting shortcut is to trust the report: the pipeline says converged,
so mark it converged. The scheduler-in-code arrangement makes that shortcut unrepresentable
for the drain, because the only way an item leaves the queue is through dispatch to a verifier
that queries the ledger and the cluster directly. There is no inline "the queue-runner
verified it in passing" path, because the queue-runner is code that cannot verify anything —
it can only dispatch. A Canton convergence check that would have been a plausible thing for a
distracted orchestrator to wave through is now a dispatched item with a recorded command, an
exit code, and the ledger response it was derived from. The Canton work is a minority of what
the fleet does — most items are ordinary software briefs — but it is the part where "the
scheduler cannot cut corners because the scheduler cannot judge" pays the largest dividend.

## What the separation buys, and the limit of the claim

Pulling the scheduler into code buys three concrete things, and it is worth being exact about
each because the fourth thing people assume it buys, it does not.

It buys **attribution**: every scheduling decision — claim, dispatch, retry, land, release —
is journalled, so the record of what was dispatched and what came back is derived from the
run, not narrated by the model afterward. It buys **evidence**: a result is rendered from the
dispatched runner's structured output — command, exit code, key output — not from free text a
model wrote about its own work. And it buys **structural separation**: the entity that
schedules cannot be the entity that judges, and by a typed guard the author of an item cannot
be the runner that verifies it, enforced with its own distinct exit code rather than a
convention.

What it does not buy is cryptographic un-forgeability. A session holding the right signing
material can still forge a result; nothing in the scheduler prevents that. The separation is
structural and the attribution is real, but the true close — the author of a change being
unable to also approve it — is a property of distinct signing identities and branch
protection, which sits outside the scheduler entirely. We state this because the failure
mode of a piece like this is to let "the scheduler cannot cut corners" quietly inflate into
"the results cannot be faked," and those are different claims. The engine measures and
attributes; it does not make forgery impossible, and it should not be sold as if it does.

## The takeaway: a harness you can clone

The mechanism generalises past our stack. Any team running agent loops hits the same
collapse — orchestration eating the attention that should be doing the work — and the same
fix applies regardless of what your items are or where your workers run. So the takeaway of
this article is not a description; it is a working, framework-agnostic harness you can clone
and run, in [`drainloop/`](./drainloop/).

It implements the same six-method contract against stand-in adapters — an in-memory queue, a
file-based claim, an echoing dispatcher — so you can watch a constant-N pool drain a set of
fake items, claim before dispatch, land each result as it returns, refill, and idle, with
nothing from our infrastructure attached. Swap the adapters for your queue, your claim store,
and your real dispatch, and you have a non-stalling drain in your own stack. The engine loop
in the middle — the part that used to live in a model's attention — you get to keep as is. The
harness README ([`drainloop/README.md`](./drainloop/README.md)) has the quickstart; it assumes
nothing about how we work.

The move is small and it is not novel: take the deterministic part of a job out of the place
that is bad at determinism. It is worth writing down only because, with agents, the place
that is bad at determinism is the same place that is good at judgment, and the temptation to
let it do both is strong right up until the pool is full and the schedule quietly falls on the
floor.
