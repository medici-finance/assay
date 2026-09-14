# The GitLab CI half — the pipeline-side leak-sweep job

This file holds the **`.gitlab-ci.yml` shape** an assay adopter templates into a
GitLab-hosted repo. [`forge-neutral/08`](brief-08-statusgen-forge-aware.md) owns the broader
CI scaffold (init's CI-config generation, the parity jobs); this section, owned by
[`forge-neutral/09`](brief-09-substrate-leakgate-and-cellctl.md), is the **pipeline-side
leak-sweep job** — the free-tier compensator the live pilot found missing
(`../forge-gitlab/pilot-report.md` §3 row 8: no `.gitlab-ci.yml`, no pipelines,
`secret_push_protection_enabled: false`). It is templated in alongside the rest of the CI
half, **not** left as a step an adopter must remember, so a repo that gets the scaffold gets
the compensator.

See [`leak-gate-shape.md`](leak-gate-shape.md) for where this job sits in the two-layer
design: on GitLab CE it is the **blocking** layer (CE cannot express a blocking external
verdict), and on GitLab Ultimate it is the independent second layer behind the external
status check.

## The job

The job runs the **in-tree disclosure controls** — the pattern half of the sweep, the part
that needs no private token map — over the change's own tree, in the change's own pipeline,
and **fails the pipeline** when they trip. A failing required pipeline is what blocks the
merge on CE.

```yaml
# --- leak-sweep: the pipeline-side disclosure compensator ---------------------
# Runs the in-tree (pattern-half) controls in the change's OWN pipeline and fails
# it on a hit. This is the free-tier layer: it needs no licence and no private
# token map, only building. It does NOT replace the out-of-band control-based
# sweep that posts the external `leak-sweep` verdict — it is the independent
# layer that still fires when that verdict is unavailable (CE, or a sweep outage).
leaksweep:
  stage: test
  rules:
    # Run on merge-request pipelines and on the default branch, so a change is
    # swept before it merges AND after, and an absent run is never mistaken for a
    # clean one.
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
  script:
    # The in-tree controls only. `leaksweep` is the same binary the house gate
    # builds; here it runs the pattern engine over the checked-out tree with NO
    # private token map, so it is safe to run in a public/free-tier pipeline.
    - leaksweep run --tree "$CI_PROJECT_DIR" --engines legacy
  # A non-zero exit fails the job, which fails the pipeline. Make the job a
  # REQUIRED pipeline step in the project's merge-request settings so a failing
  # sweep blocks the merge on CE.
  allow_failure: false
```

## The three-state property in CI

The job encodes the same three-state contract the external verdict does
([`leak-gate-shape.md`](leak-gate-shape.md)):

- **passed** — the job ran and exited 0: the pattern controls found nothing.
- **failed** — the job ran and exited non-zero: a control tripped; the pipeline is red and
  (as a required step) the merge is blocked.
- **could not run** — the job is **absent** from the pipeline (removed, `rules` excluded it,
  the pipeline never ran). A required pipeline that produced no `leaksweep` job is
  could-not-check, never a pass — which is why the job's `rules` fire on both the
  merge-request pipeline and the default branch, and why the job is a **required** step: an
  absent run then shows as a missing required pipeline rather than as a silent pass.

The job is deliberately the pattern half only. The strong, control-based sweep needs the
private withheld-token map and cannot run in an adopter's CI; it posts its verdict out of
band as the external `leak-sweep` status. This job is the layer that does not depend on that
external posting having happened.
