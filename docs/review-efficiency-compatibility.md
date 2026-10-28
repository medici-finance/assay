# Review caller compatibility and retirement

Source qualification for desk-tools/43. No deployed pin, collection consent, executor,
credential, live adoption or release is changed here. Producer release precedes caller
activation; the latest released producer inspected at pickup is v1.0.59, source dff97978.

| Producer | Caller | Supported behavior |
|---|---|---|
| Pre-v1.0.59 supported tools | Prior caller or this caller in legacy mode | Existing event monitor plus classified actions cadence; actions+prs legacy planner; observation flags omitted |
| v1.0.59 | Prior caller | Legacy interfaces remain intact; no new observation requirement |
| v1.0.59 | This caller, classified review mode | One complete review-queue, offline review-selection-v1, advisory observation preparation; review-observation-v1 |
| This source /43 | This caller | Same classified path plus frozen coordinator/inbox review depth; new throughput flag is source-qualified, no future release number assigned |
| Unknown schema, unavailable tool/version, changed scope/policy or stale cutoff | Classified caller | Visible complete-cycle hold; invalidate reusable output, retain event baseline, no in-cycle legacy scan |

Harnesses use their existing durable-monitor and bounded wait bindings. There is one
routine owner per watch, due liveness independent of event changes, same-item continuation,
and fresh-new-item contexts. PR #3325 merged as 028b35e66be87d2d97f0714bf3df916eb7ee0fcd;
its canonical desk-common supervision procedure is reused. This prose is guidance, not
runtime enforcement. graph-execution/27 remains the later all-role binding and consumes
this planner; it must not copy selection or classification.

The launcher chooses classified mode explicitly with current policy, capacity and the
full requested repo set. Never derive priority approval from untrusted observation bytes.
The source adapters remain opt-in until installed tools and plugin are paired and the
existing caller/adopter gates accept them. The supported legacy recipe is preserved in
`plugins/assay/references/desk-common.md#review-observation-handoff`. Rollback selects it
on a later cycle after resolving live effects; old and new effects never run together.
One quarantined public PR holds the complete classified cycle. Recovery reads the full
population again; neither dropping the PR nor narrowing the scope constitutes recovery.

| Consumer | Routing / remaining obligation |
|---|---|
| pr-review-desk skill and generated Cursor copy | Qualified recipe uses the existing monitor's observation and returned policy order, passes it to deskdispatch; legacy recipe retained |
| pr-monitor.sh | Classified mode collects once; offline plan and PR events use that artifact; default legacy mode unchanged |
| deskboard throughput | Explicit snapshot mode validates current local policy, scope and cutoff; review depth from shared PlanObservation; legacy actions path retained |
| the-desk and assay-inbox flow | Forward applicable artifact to throughput; standalone legacy reads remain supported |
| desk-common and Codex harness binding | Existing one-watch-owner and bounded wait procedure retained; canonical handoff added once |
| trace-path-to-live and explicit diagnostic actions/prs/reviews users | Remain full-board/diagnostic consumers, not duplicate routine collection removed here |
| supported external pins, cells and operator recipes | Actual fleet not inspected or activated; release/adoption owner must qualify each pair before switching |
| desk-tools/44 | Complete cost and last-consumer census remains; no command/binary retirement claimed |

No deskboard, reviewloop or deskdispatch command is removed. Active parser work #3248/#3302
retains interfaces; generic scorecard ownership #3305 is untouched. No new scheduler,
checkpoint, service, credentials or cache is introduced. /42 owns issue projection policy,
/45 and /51 own verification queue and prose integration, desktools-v2 owns CLI/custody,
and drainloop owns effect lifecycle rather than a second priority algorithm.

Cost qualification uses fake provider data and disposable homes. For R one-page repos,
P changed PRs, the existing planning estimate is 6R+2P versus 5R+2P foreground requests.
A same-cutoff watcher handoff additionally removes the actual second collector cost C;
coordinator/inbox snapshot depth adds zero review-provider requests instead of C each.
Cold collection remains O(R+P), including history/page factors. K pure selections from
an applicable observation cost zero provider requests instead of K independent scans;
whole-cycle cost is C+fresh consequential checks, not zero. No CPU, elapsed, quota-unit
or live fleet saving is inferred from request counts; /44 owns broader qualification.

Measured matched local fixture (three PRs, one repo, same fake provider responses and
existing cache behavior): event inventory + actions + prs + independent review-depth
classification made **14 HTTP requests**; one shared queue plus offline planning and
reused depth made **7**. Both retain O(R+P) cold work. This 50% fixture reduction excludes
review dispatch and its fresh effect checks and is not a live fleet estimate. The
acceptance test records the actual HTTP transports and requires zero extra review-depth
requests; monitor invocations are also counted, with no legacy call in selected mode.
