# skillbench

An **AI-free reducer** that measures what one of our own skills or prompt overlays does to a
coding session — adapting a same-agent-with-and-without-overlay benchmark method to the house.

It is a **pure reducer over committed session artifacts, not an agent runner.** It never
shells out to an agent, never reads GitHub or git, and makes no network call. Its input is a
directory laid out as two arms; its output is one markdown report of per-metric deltas. The
house pattern for such collectors is AI-free, three-state discipline included.

## The one rule

**A could-not-check must never become a measured value.** A run missing its usage log renders
`tokens`/`cost` as `could-not-check`, never a zero; a delta is emitted only when *both* arms
measured that metric over at least one run. The state is rendered in the cell, so a gap can
never be read as a number. This is the same discipline the house AI-free collectors enforce,
and it is the whole reason the tool exists rather than a shell one-liner.

## Input: the two-arm artifact layout

The `--arms <dir>` directory holds exactly two arm subdirectories, and each holds one
subdirectory per run:

```
<arms>/
  with-overlay/
    run-1/  run-2/  …          # ≥1 run; the arm with the skill/overlay applied
  without-overlay/
    run-1/  run-2/  …          # ≥1 run; the same tasks, overlay OFF
```

Each **run** directory holds up to three artifact files — the whole input contract:

| File | Required? | Feeds | Shape |
|---|---|---|---|
| `diff.patch` | yes (for diff metrics) | `diff_lines`, `files_touched` | the run's `git diff` (unified) |
| `usage.json` | **optional** | `tokens`, `cost_usd` | `{"tokens": 12000, "cost_usd": 0.12}` |
| `run.json` | yes (for time/check) | `wall_seconds`, `check_pass_rate` | `{"wall_seconds": 120, "check": "pass"}` |

- `diff_lines` = added + removed content lines; `files_touched` = count of `diff --git`
  headers. File-header lines (`+++`, `---`) are not counted as content.
- `usage.json` is **optional by design**: it is the could-not-check path. A run without it, or
  with a `usage.json` missing a field, renders that metric `could-not-check` — never a measured
  zero. This is the positive control the fixtures and tests exercise.
- `run.json`'s `check` is `"pass"` or `"fail"` (the task's own check result — the safety
  floor). Anything else is `could-not-check`, not a silent fail.
- A file absent, unparseable, or missing a field is `could-not-check` with a distinguishing
  note; only a present, readable value is `measured`.

## Metric set

| Metric | Direction | Source |
|---|---|---|
| `diff_lines` (added+removed) | lower is better | `diff.patch` |
| `files_touched` | lower is better | `diff.patch` |
| `tokens` | lower is better | `usage.json` (optional) |
| `cost_usd` | lower is better | `usage.json` (optional) |
| `wall_seconds` | lower is better | `run.json` |
| `check_pass_rate` | higher is better — the **safety floor** | `run.json` |

Each arm's figure is the **mean over the runs that carried it**, and the report shows that
count as `n`. A cost win at a fallen `check_pass_rate` is a regression, not a win — the report
states the safety floor on its own line so a consuming adoption brief can read adopt/hold unambiguously.

## Output: the report

```
skillbench --arms <dir> --out reports/skillbench/<date>-<overlay-slug>.md
```

- `--arms` (required) — the two-arm directory above.
- `--out` — report path. Default: `reports/skillbench/<date>-<overlay-slug>.md` under the repo
  root.
- `--overlay-slug` — label for the header/filename. Default: the base name of `--arms`.
- `--date` — `YYYY-MM-DD`. Default: today (UTC).

Exit codes are coarse **on purpose** — the three-state discipline lives in the report cells,
not the process exit. `0` = a report was written (even a degraded one, with its gaps on the
page); `1` = a usage error; `2` = the arms directory itself could not be read. A missing metric
never changes the exit code.

The report carries, per metric, a `with-overlay` line, a `without-overlay` line, and a `delta:`
line (each with its `n`), then a verdict block that states the safety floor and the cost-side
movement as an **input to an adoption decision, not an adoption**. The harness draws no conclusion of
its own.

## Producing the runs (runbook — OUTSIDE this tool)

The harness reduces artifacts; it never produces them. Producing the two arms is a desk
dispatch step, and **overlay on/off is a dispatch-prompt difference, nothing else**:

1. Pick a small set of fixture tasks (real, checkable repo tasks with a pass/fail check).
2. Dispatch **N workers per arm** on the *same* tasks — the `with-overlay` arm with the
   skill/overlay in the dispatch prompt, the `without-overlay` arm with it removed and
   *nothing else changed* (same model, same tier, same tasks).
3. For each run, collect its artifacts into `<arm>/run-<k>/`:
   - `git diff` of the run's changes → `diff.patch`;
   - the run's token/cost usage, if the harness recorded one → `usage.json` (omit it if there
     is none — do **not** write zeros);
   - `{"wall_seconds": …, "check": "pass"|"fail"}` → `run.json`.
4. Run `skillbench --arms <dir> --out reports/skillbench/<date>-<overlay-slug>.md` and commit
   the report. The report is the artifact a consuming adoption brief (and every future overlay
   brief) gates adoption on.

Because the tool is a pure reducer, the runbook and the measurement are cleanly separable: the
same committed arms directory always reduces to the same report.

## Fixtures

- [`fixtures/complete/`](./fixtures/) — a full two-arm fixture (2 runs per arm, every artifact
  present); the golden report is [`testdata/complete.golden.md`](./testdata/).
- [`fixtures/degraded/`](./fixtures/) — the `with-overlay` arm's runs carry no `usage.json`, so
  `tokens`/`cost` reduce to `could-not-check` (the positive control: a missing usage log never
  renders as a measured zero).

## Running the tests

```bash
cd tools/skillbench
go build ./... && go vet ./... && go test ./...
```

Regenerate the golden after an intended report-format change with
`go test . -run TestReportGolden -update`. The repository's `ci.yml` auto-discovers every Go
module in the tree (one `go.mod` per module) and runs `go build` + `go vet` over each, so this
module is covered without editing the workflow; run the full `go test` suite locally.
