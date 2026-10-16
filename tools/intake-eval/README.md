# intake-eval — intake-routing scorer

`intake-eval` measures how well an intake desk routes inbound items. It reads a **gold set**
(inbound items whose correct exit a human has confirmed) and a **prediction set** (the exit a
routing run chose for each item). For each of the five intake exits it reports precision,
recall, F1 and support. Given a threshold config it also returns a PASS/FAIL verdict, which
sets the process exit code.

The five exits are the classes: `spec/brief` · `bug/issue` · `finding` · `needs-decision` ·
`rejected/watching`.

The tool reports **per exit** and never uses a single blended accuracy as its verdict. Intake
traffic is imbalanced: a router that sends every rare `needs-decision` item to the majority exit
can still score 95% accuracy. Per-exit recall drops to 0 for that exit, and that is the signal.
Accuracy is printed, labelled informational.

## Consumption contract

- **Pinned binary, consumer-supplied data.** A consumer runs a released, version-pinned build
  and passes its own gold set, predictions and thresholds as arguments. The tool bundles no
  corpus and has no default corpus path, so a private corpus is never needed to build or test it.
- **Headless.** No network, no prompts, no state. Exit codes are the contract:

  | Code | Meaning |
  |------|---------|
  | 0 | Scored. The verdict is PASS, there is no `--thresholds`, or the config is `advisory`. |
  | 1 | Verdict FAIL (non-advisory). |
  | 2 | Usage error, unreadable input, a gold set that fails preflight lint, or an invalid threshold config. |

- **Own fixtures are synthetic.** Everything under `testdata/` is invented for this tool's tests.
  Each file carries a `# SYNTHETIC` header and a `syn-` id, and a test enforces both.

## Flags

| Flag | Required | Meaning |
|------|----------|---------|
| `--gold <dir>` | yes | Gold-set directory: one `*.yaml` / `*.yml` file per fixture, read non-recursively. Other files (a README, say) are ignored. |
| `--pred <file>` | yes | Predictions: a YAML or JSON mapping of fixture `id` to predicted exit. |
| `--thresholds <file>` | no | Threshold config (JSON, schema below). Turns on the PASS/FAIL verdict. |
| `--baseline <report.json>` | no | An earlier `--json` report. Its per-exit F1 values become the regression baseline and replace any `baseline` in the threshold config. Requires `--thresholds`. |
| `--methodology-version <v>` | no | The corpus's current methodology version. Fixtures behind it are flagged STALE. Defaults to the newest version on any fixture. |
| `--json` | no | Print the machine-readable report instead of the table. |

```sh
intake-eval --gold <gold-dir> --pred <predictions.json> --thresholds <thresholds.json> --json
```

## Gold-set fixture schema

One YAML file per fixture:

```yaml
id: syn-001                       # unique across the set
inbound: "…"                      # the inbound text (not read by the scorer)
expected_exit: spec/brief         # one of the five exits, verbatim
methodology_version: "2026-01-01" # the methodology the label was settled under
scoring: scored                   # scored | excluded
provenance:                       # required: the source that confirms the label
  type: …
  tier: …
  ratification: confirmed         # confirmed | pending-human
  ref: …
  confirmed_by: …
  date: "…"
  quote: "…"
notes: "…"                        # optional
```

Only `scoring: scored` fixtures are scored. `excluded` fixtures are listed as awaiting human
confirmation. Predictions for them are ignored.

### Preflight lint

Before scoring, every fixture is checked. Any violation fails the run with exit 2 and lists
**every** problem found. Invalid fixtures are never skipped silently. The checks:

1. `provenance` is present and all seven sub-fields are non-empty. `ratification` is
   `confirmed` or `pending-human`.
2. `expected_exit` is one of the five exits.
3. Every exit appears on at least one fixture file, scored or excluded.
4. `methodology_version` is present. Fixtures behind the current version are reported as
   **STALE** (see `--methodology-version`). They are still scored, and the report surfaces them.
   Versions compare as strings, so use a lexically sortable form such as an ISO date.
5. `scoring` is `scored` or `excluded`. `scored` requires `ratification: confirmed`, and
   `excluded` requires that it is not `confirmed`.
6. `id` is present and unique.

The scorer checks that `type`, `tier` and `confirmed_by` are present but not their vocabularies.
Those values belong to the corpus owner, so the corpus owner's own lint checks them.

The predictions file is checked too. Every predicted exit must be one of the five, and every id
must exist in the gold set, so a wrong gold set fails loudly. A scored fixture with no prediction
counts as a miss on its expected exit and is listed under `missing`.

## Metric definitions

Over the scored fixtures, for each exit *e*:

- **support**: the number of fixtures whose expected exit is *e*.
- **TP**: fixtures expected *e* and predicted *e*. **FP**: predicted *e*, expected something
  else. **FN**: expected *e*, predicted something else or not predicted.
- **precision** = TP / (TP + FP). **Unmeasured** when nothing was predicted into *e*.
- **recall** = TP / (TP + FN). **Unmeasured** when *e* has no scored fixture.
- **F1** = 2TP / (2TP + FP + FN), the harmonic mean of precision and recall in count form.
  **Unmeasured** whenever recall is. Because of the count form, an exit that has fixtures but
  receives no predictions (a collapsed class) scores F1 = 0, not "unmeasured".
- **accuracy** = correct / scored. Informational only. The verdict never uses it.

An unmeasured metric is `null` in JSON and `unmeasured` in the table. It is never reported as 0
and never treated as passing. If a scored fixture is predicted into an exit that has no scored
fixtures, that exit's precision is 0: every such prediction is a misroute.

## Threshold config schema

```json
{
  "advisory": false,
  "unmeasured": "fail",
  "regression_delta": 0.05,
  "baseline": { "spec/brief": 0.92, "bug/issue": 0.88 },
  "exits": {
    "spec/brief":        { "precision": 0.85, "f1": 0.80 },
    "bug/issue":         { "precision": 0.85, "f1": 0.80 },
    "finding":           { "f1": 0.70 },
    "needs-decision":    { "f1": 0.70 },
    "rejected/watching": { "f1": 0.70 }
  }
}
```

| Key | Meaning |
|-----|---------|
| `exits.<exit>.precision` | Precision floor for that exit. Set a higher floor on the exits that spawn work than on the terminal ones to make a misroute into a work-spawning exit cost more. |
| `exits.<exit>.f1` | F1 floor for that exit. |
| `regression_delta` | No exit's F1 may fall more than this below its baseline. Requires a baseline. |
| `baseline` | Per-exit F1 baseline. `--baseline <report.json>` replaces it. A baseline value of `null` (unmeasured in the earlier run) skips that exit's regression check. |
| `advisory` | `true` reports the verdict but always exits 0. Use it before thresholds are ratified, or while the corpus is methodology-stale. |
| `unmeasured` | `"fail"` (default) or `"report"`. Decides a checked metric that is unmeasured for this data. |

Floors and `regression_delta` must lie within [0, 1]. The scorer refuses a config with unknown
keys, unknown exit names, a `regression_delta` with no baseline, or a baseline with no
`regression_delta`. A typo cannot silently drop a check. The file must hold one JSON value; content
after it is refused. A floor written as an explicit `null` is the same as omitting it: that check is
not run.

The verdict is FAIL when any check fails. Under `"unmeasured": "fail"`, an unmeasured value on a
checked metric also counts as a failure. The `--json` report lists every check with its floor,
value and status (`pass`, `fail` or `unmeasured`).

## Report (`--json`)

Top-level keys: `methodology_version`, `fixtures`, `scored`, `correct`, `accuracy`, `exits`
(per exit: `support`, `predicted`, `tp`, `fp`, `fn`, `precision`, `recall`, `f1`), `missing`,
`ignored_excluded`, `pending_human`, `stale` and `verdict` (`null` without `--thresholds`;
otherwise `result`, `advisory`, `unmeasured`, `failures` and `checks`).

## Development

```sh
cd tools/intake-eval
go build ./...
go test ./...
go run . --gold testdata/gold-perfect --pred testdata/pred-perfect --json
go run . --gold testdata/gold-imbalanced --pred testdata/pred-collapse
go run . --gold testdata/gold-perfect --pred testdata/pred-regressed --thresholds testdata/thresholds.json; echo $?
```
