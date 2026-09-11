# The leak gate's verdict surface, per forge

The leak gate is a **disclosure control**: it decides whether a change may land by
asserting that no withheld content is present in the change's tree. Its verdict is a
_merge blocker_, and the load-bearing property is not "does the gate pass" but "what does
a **missing** verdict mean". A control whose absence is indistinguishable from a pass is
not a control — a change lands ungated whenever the gate never ran. This document fixes,
per forge, **where the verdict lands**, **whether it blocks**, and the **three-state
contract** every consumer of the verdict must honour.

The gate has two independent layers, by design (see the brief's `single-point-of-failure`
note). This document specifies both:

1. **The external verdict** — a status posted against the change's head by the
   control-based sweep, which runs outside the change's own pipeline. It is the strong
   layer, and where the forge can express a _blocking_ status it is the merge blocker.
2. **The pipeline-side sweep** — a job in the change's own CI that runs the in-tree
   disclosure controls and **fails the pipeline** when they trip. This is the free-tier
   compensator, and it is the layer that still fires when the external verdict is
   unavailable (a tier that cannot express a blocking external status, an outage of the
   out-of-band sweep). Its shape lives in [`gitlab-ci-half.md`](gitlab-ci-half.md), which
   [`forge-neutral/08`](brief-08-statusgen-forge-aware.md) templates into the GitLab CI
   half so an adopter does not have to remember it.

The two layers catch a change on **different signals in different components** — the
external verdict on the head, the pipeline job in the change's own CI — so a single
component failing does not open the gate.

## Verdict surface, per forge

| Forge / tier | External verdict surface | Blocks the merge? | Pipeline-side layer |
|---|---|---|---|
| **GitHub** | the `leak-sweep` **commit status** on the change's head (`leaksweep-pattern.yml` posts the pattern half; the strong control-based sweep posts the verdict out of band) | **yes** — a required status check in branch protection; a red or absent `leak-sweep` status blocks the merge | the in-tree controls run in the change's CI (`leaksweep-control.yml`) and fail the run |
| **GitLab — Ultimate** | an **external status check** on the merge request, plus the head-pipeline **commit status** | **yes** — the external status check is a merge-request approval rule the tier can enforce as blocking | the sweep **job** in the MR's own pipeline fails the pipeline (`gitlab-ci-half.md`) |
| **GitLab — CE / free tier** | the head-pipeline **commit status** only (the blocking external-status-check surface is tier-gated and returned `HTTP 401` on the pilot — `../forge-gitlab/pilot-report.md` §3 row 4) | **advisory** — CE cannot express a _blocking_ external verdict, so the commit status alone does NOT block | **the merge blocker on CE**: the sweep job runs in the MR's own pipeline and fails it, and a failing required pipeline is what blocks the merge here (`gitlab-ci-half.md`) |

**The GitHub side is never made advisory to match GitLab CE.** Where a forge cannot
express a blocking external verdict, the answer is to add the pipeline-side layer for that
forge — not to weaken the layer the other forge _can_ enforce. On CE the pipeline-side job
_is_ the blocking layer; on GitHub and GitLab Ultimate the external verdict blocks and the
pipeline-side job is the second, independent layer behind it.

## The three-state contract

Every consumer that decides a change is mergeable — a ready-flip gate, a branch-protection
rule, a human reading the queue — reads the verdict as **one of three states**, and the
third (a verdict that could not be established) is reported AS ITSELF: never rounded up to
a pass, never rounded down to a failure that was not observed.

| Verdict state | What the consumer observes | How it MUST be read |
|---|---|---|
| **ran and passed** | the `leak-sweep` status is present at the change's head and green (GitHub commit status `success`; GitLab MR external check `passed` / a green sweep job) | **clear** — this condition is satisfied |
| **ran and failed** | the status is present and red (GitHub `failure`/`error`; GitLab external check `failed` / a failing sweep job) | **blocked** — the change may not land; the withheld content must be scrubbed |
| **could not run** | the status is **absent** from the head — no `leak-sweep` context in the rollup, no MR external check reported, no sweep job in the pipeline | **could-not-check** — treated exactly as _not cleared_, NEVER as a pass. A missing verdict is "the gate did not run", which is never "no objection". |

The whole point of the contract is the third row. On the ready-flip decision this is
enforced in `tools/desk/cmd/deskflip` (`checks-green`): a rollup that is otherwise green
but is **missing** a required verdict such as `leak-sweep` is could-not-check, not a pass —
the flip cross-checks that every branch-protection-required context is _present_ in the
rollup and refuses when one is absent (`TestMissingLeakGateIsCouldNotCheck`). Absence of
the verdict is the exact failure this contract exists to prevent: a change landing because
nothing ever posted an objection.

## Why the verdict is posted out of band

The strong, control-based sweep needs the private withheld-token map, so it **cannot run in
the public CI** (`leaksweep-pattern.yml:1-27`). It runs privately, against the repo's change
heads, and posts its verdict back as the `leak-sweep` status. That out-of-band posting is
exactly why the "could not run" state is not hypothetical: a head the private sweep has not
yet reached carries a green rollup of every check that _did_ run, with the disclosure
verdict simply not there. The pipeline-side layer exists so that head is still caught by its
own CI, and the three-state contract exists so the ready-flip decision does not mistake the
gap for a pass.
