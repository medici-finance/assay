# Test policy

Assay's per-brief Verify table probes the **delta** a brief introduces — the behaviour that
change is supposed to add or fix. That is necessary and it is not sufficient: the same identity
that wrote a change chooses what its Verify rows assert, so nothing in the Verify table alone
guards the **baseline** the change did not touch, states what a test is for, or says what a
flaky result means. This policy fills that gap. It is methodology, not a house convention: it
binds every repo that adopts Assay, and it is deliberately independent of whoever wrote the
last brief.

It covers five things and no more: the test **tiers**, the **regression floor**, **flake
classification**, the **standing truth suite**, and **plan-in-PR**. It changes nothing about
the Verify row `Class` vocabulary itself (`docs/brief-template.md`) — it maps onto that
vocabulary rather than extending it — and it introduces no new instrument state: every check
here reports the three states of `docs/three-state-instrument-rule.md`
(checked-clean / checked-failed / could-not-check) and never a fourth.

## Tiers

Four tiers, each named for what it is FOR and who runs it. Each maps onto the existing Verify
row `Class` vocabulary (`docs/brief-template.md`) — this policy invents no parallel classes.

| Tier | For | Who runs it | Verify row `Class` |
|------|-----|-------------|--------------------|
| **unit** | Prove one component's logic in isolation. Hermetic — no network, no clock, no filesystem beyond the checkout. | CI, on every push and PR. | `check:ci` (CI re-executes it network-off and refuses the verdict on mismatch). |
| **integration** | Prove two components agree across the boundary between them, with the far side real rather than mocked where that is what the assertion is about. | CI when the far side is still hermetic (a second in-tree module, a fixture process); a runner when it is not. | `check:ci` when hermetic; `check` when it needs a real external component CI cannot provision. |
| **live** | Prove behaviour that only exists against an environment CI does not have — a real queue, a live credential, a deployed endpoint. | The verifier, on their own machine or in the target environment; its verdict rests on the verifier's authorship, not on a CI re-run. | `check` — this is exactly the `check` vs `check:ci` split the Verify row classes already encode. |
| **drill** | Prove a *failure* path by rehearsing it — kill a dependency, revoke a token, trip a breaker — on a cadence, not on every diff. A drill that has never been run is a control that has never been shown to fire. | A scheduled runner (the standing truth suite below, or a named cadence owner), never a per-diff gate. | `check` when expressed as a Verify row; judgment outcomes use `gate:model` / `gate:human`. |

The tier is a property of what the test needs, not a label an author picks for convenience. A
test that reaches the network is not a unit test however it is filed, and CI treats it
accordingly: a `check:ci` row that turns out to need the network fails hermetic re-execution
rather than passing quietly. A tier CI cannot run — a `live` or `drill` row on an environment
CI does not have — is reported **could-not-check**, never rounded up to pass: an un-run tier
has cleared nothing, and the board must show that it did not look rather than imply that it did.

## Regression floor

The floor is a **property**, stated as an invariant a merged change must preserve:

> A merged change may not reduce the set of behaviours the standing truth suite asserts.

A change may add behaviours the suite guards; it may not remove one without replacing it with
an assertion at least as strong, and a deletion that is genuinely correct (a behaviour that no
longer exists) is stated in the PR and reviewed as such, never absorbed silently.

**A coverage percentage is deliberately *not* the floor.** A line- or branch-coverage
percentage is gameable by exactly the population this policy governs: an agent that writes both
the code and the tests can raise a coverage number with assertion-free tests that execute lines
without checking their results, and the number climbs while the guarantee does not. Percentage
measures which lines ran, never whether a wrong result would have been caught, so it is *not
the floor* — the floor is the set of asserted behaviours, which shrinks only when someone
removes an assertion, and that removal is a diff a reviewer can see.

## Flake classification

A test that sometimes passes and sometimes fails on the same tree is not a fourth outcome
alongside pass and fail — it is an instrument that has not yet been diagnosed. There is no
"known-flaky, ignore" bucket, because a muted test is an un-run test and an un-run test reports
**could-not-check**, never pass (`docs/three-state-instrument-rule.md`). Every flake is placed
in exactly one of three classes, each with a required action:

| Class | What it is | Required action |
|-------|-----------|-----------------|
| **environmental** | The test is correct; something outside the tree (a slow runner, a shared port, a rate limit) makes it intermittent. | **Quarantine, bounded.** Shelve it with the findings register's parking shape — `parked-until` (a hard expiry date), `parked-by` (a `human:<name>` authority), and `parked-reason`, all three required together — cited from the findings register, not re-invented. A park is a snooze, not a mute: on expiry it re-annunciates. **While parked the test reports `could-not-check`, never pass** — it is guarding nothing until it runs again. |
| **non-deterministic-under-test** | The test races or depends on unpinned order/time *in the code or the test itself*: the same tree genuinely produces different results. | **File a finding**, `class: recurring`. This is a real defect in the code or the assertion, and it stays visible in the findings register until a permanent fix lands, not parked away. |
| **wrongly-asserted** | The test asserts something that was never actually invariant — an incidental ordering, a timestamp, a map iteration order. | **Fix the assertion** so it pins the real invariant and nothing more. The flake was the test's fault; there is nothing to quarantine and nothing to file. |

No fourth class exists on purpose. "Ignore it, it's flaky" is the exact move this section
forecloses: it converts a checked-failed or a could-not-check into a silent pass, which is the
two-state lie the three-state rule exists to stop.

## Standing truth suite

The standing truth suite is the **baseline** assertions that CI owns — the corpus of tests plus
the full set of mutation gates the release already runs, including the deskmerge sweep (the
highest-risk guard, since deskmerge is the only desk tool that can rewrite the head of somebody
else's branch) — executed independently of whoever wrote the last brief. Release shards the
deskmerge sweep across three legs because it is the long pole in a per-PR-blocking suite; the
standing suite runs push+daily rather than gating a PR, so it runs the same sweep unsharded in a
single leg — same coverage, no per-PR wall-time pressure to shard against. It runs on push to the
default branch and on a daily schedule (`.github/workflows/truth-suite.yml`), so a regression is
caught by a suite the change's author did not write and cannot narrow.

**What it is not:** it is not a replacement for per-brief Verify rows. The two guard different
things and neither substitutes for the other:

- a **Verify row** probes the **delta** — did *this* brief do what it claims — and is written
  by the brief's author before the work exists;
- the **standing truth suite** guards the **baseline** — did the merged change break a
  behaviour it never mentioned — and is owned by CI, written against the existing corpus, and
  reddens on a regression the delta's author never considered.

This is the second, independent layer the single-point-of-failure would otherwise lack: it
fails for a different reason (a baseline assertion, not the delta's own row), in a different
component (CI, not the author's table), so a change that is green on its own Verify table can
still be caught. The suite reports three-state — a leg it could not run is `could-not-check`,
never a green tick over an un-run baseline.

## Plan-in-PR

Before the first commit on a brief of effort **M** or **L**, the draft PR body carries a
`## Plan` block: a short statement of the approach the author intends to take before any code
lands. The review then includes a **diff-vs-plan pass** — the reviewer reads the final diff
against the stated plan — and a divergence between the two is **stated**, not silently absorbed:
if the implementation went a different way than the plan said, the PR says so and why.

**What this buys:** a rework signal that is visible *before* review rather than after. A plan
that the diff has quietly abandoned is the cheapest possible early warning that the approach
changed under the author's hands, and it surfaces at the point where redirecting costs one
comment instead of a re-review round-trip.

**What it does not:** it is **not a gate**. No check parses a PR body for a `## Plan` block and
no merge is blocked for its absence. It is an argument addressed to a reviewer — a discipline
that makes divergence legible — and a reviewer can note its absence on an M/L brief the same
way they note any other missing context. Effort-S briefs are exempt: the plan and the diff are
the same size, and demanding a separate plan block there is ceremony, not signal.
