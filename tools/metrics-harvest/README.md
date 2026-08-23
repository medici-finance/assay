# metrics-harvest

A multi-repository, point-in-time metrics reducer. Given one CI run's per-grouping
snapshots, it rolls them up into a committed daily record — open PRs by state and open
issues by label, per grouping and summed across products.

The set of repositories, how they group, and the grouping order are **not** compiled into
this tool. They are declared in an external config file that is read at runtime — see
[`domains.example.yaml`](./domains.example.yaml) for the schema. The tool ships with the
example only; provision your own roster as `domains.yaml` (git-ignored here) or point
`--config` at it.

Two halves, deliberately split:

| Half | What it is | Where it runs |
|---|---|---|
| **collector** | a `gh`+`jq` step set — no Go | one leg per grouping in a scheduled workflow |
| **reducer** | this Go module — no `gh`/`git` reads at all | once per run |

The collector shells out to `gh` only for the counts that are genuinely **point-in-time**
(an API query cannot reconstruct what was open on a past date the way it can replay commit
history). Re-derivable data — commit history, merged-PR lead times — is deliberately **not**
snapshotted: replaying queryable data out of a nightly commit is storage, not measurement.

## What the collector captures

For every repo in a selected grouping, per day:

| Field | Source |
|---|---|
| `prsOpenByState` | `gh pr list --state open --json isDraft` reduced to `{draft, ready}` counts |
| `issuesOpenByLabel` | `gh issue list --state open --json labels` reduced to a per-label count map |
| `issuesOpenUnlabeled` | count of open issues with zero labels |

On a failed `gh` read the collector writes the field as JSON **null**, uploads the snapshot
anyway, and exits non-zero. That null is load-bearing: it means *checked, and the check
failed*, which is not the same fact as a missing key, and the reducer keeps the two apart.
Each grouping's snapshot is uploaded as a CI artifact named
`metrics-snapshot-<grouping>-<run-id>`. No raw snapshot tree is committed.

## Cross-domain reducer (`aggregate`)

- **Input**: the SAME run's `metrics-snapshot-<grouping>-<run-id>` artifacts. There is no
  committed raw tree and no cross-run Actions-API read, so artifact retention cannot silently
  change what a past roll-up was computed from.
- **Committed output**: the daily roll-up ONLY — one small `rollup.json` per grouping plus a
  top-level `rollup.json` and its human-readable `rollup.md`.
- **Schema**: `prsOpenByState`, `issuesOpenByLabel`, `issuesOpenUnlabeled`.
  `prsOpenByReviewDecision` is emitted **`null`** until the collector half (landing separately) lands —
  never zero.

```
reports/daily/<date>/<grouping>/rollup.json   # one per grouping
reports/daily/<date>/rollup.json              # the cross-domain roll-up
reports/daily/<date>/rollup.md                # human-readable companion to the above
```

```bash
cd tools/metrics-harvest
go build -o /tmp/metrics-harvest .
/tmp/metrics-harvest aggregate --config domains.yaml --date 2026-08-13 \
    --snapshots ./snapshots --run-id 12345
```

(Build it rather than `go run` it when the exit code matters: `go run` reports 1 for any
non-zero exit of the program it runs, which erases the 0/2/3 distinction below.)

| Flag | Default | What |
|---|---|---|
| `--date` | yesterday (UTC) | `YYYY-MM-DD` — the day being rolled up |
| `--snapshots` | `./snapshots` | directory holding this run's downloaded `metrics-snapshot-*.json` |
| `--run-id` | none | the CI run id these artifacts came from, recorded in the roll-up |
| `--config` | `domains.yaml` alongside the source | the grouping config (see the schema below) |
| `--root` | derived from `--config` (three directories up) | repo root the `reports/daily/` output is written under |

| Exit | Meaning |
|---|---|
| `0` | published, and every figure was measured |
| `2` | **refused** — bad config or ambiguous input. Nothing is written. |
| `3` | **published, with a gap** — at least one figure is `withheld` / `could-not-check` / `not-configured`, or the prior day's roll-up failed to parse. The roll-up still lands (the markers are the day's evidence); the CI job then goes red. |
| `1` | IO/internal failure |

## Config schema

The config file (`domains.yaml` by default; the shipped
[`domains.example.yaml`](./domains.example.yaml) documents the shape) declares the entire
taxonomy:

```yaml
products:            # ordered list of product grouping names; each is summed
  - product-a        # into the all-products total, in this order. Required.
  - product-b

org: org-wide        # optional single org-wide grouping — reported separately,
                     # NEVER summed into the product total. Omit if unused.

groupings:           # every declared grouping name -> its "owner/repo" roster.
  product-a:         # An empty list is allowed (renders not-configured); the
    - example-org/alpha   # key itself must be present, so a dropped list is caught.
    - example-org/beta
  product-b:
    - example-net/gamma
  org-wide:
    - example-org/orgrepo
```

Grouping names must match `[a-z0-9][a-z0-9-]*` — they appear verbatim in the snapshot
artifact filenames and as the per-grouping output directory names. A grouping may span more
than one owner; provision one token per owner at runtime and select it per-repo in your
harvest workflow.

The config is validated **before** anything is read, and a config the reducer cannot reduce
honestly is **refused** rather than reduced:

- a spec that is not exactly `owner/repo`;
- the same `owner/repo` listed twice, in one grouping or across two — it would inflate every
  figure it contributes to;
- two repos in one grouping sharing a base name (e.g. `example-org/alpha` and
  `example-net/alpha`) — an ambiguous label for two different repos.

Repos are keyed on the **full** `owner/repo` spec — collapsing to the base name is what let
one repo be counted twice while another was never read at all, with no gap reported.

> The real roster is a private, deployment-specific file. It is git-ignored in this tree
> (`/domains.yaml`) and provisioned to the operating cell at runtime; only the synthetic
> example is published here.

## The gap model — a could-not-check never renders as a measured value

An earlier cut of this reducer had eight defects, all of one family: an
unreadable input published `0` at exit 0; an empty-config grouping and an all-unreadable
grouping rendered byte-identically; a duplicate config entry inflated five figures and
reported clean. Each fix carries a named regression test in `aggregate_test.go`, demonstrated
fail-first.

**Every published figure is one of four states, rendered in the cell where the number would
be** — not only in a gaps section further down, which is where the old output hid it:

| Cell | Meaning |
|---|---|
| `<n>` | **measured** — every configured source for that figure contributed |
| `withheld (m/n sources)` | some sources failed, were missing, or were truncated. A count is **not** published from a partial source set: a smaller number is indistinguishable from a real fall |
| `could-not-check (0/n sources)` | sources are configured and **none** could be read. Not a zero |
| `not-configured` | no repos configured for that grouping at all. Also not a zero |
| trailing `~` | the source declared nothing about whether its listing hit the API `--limit` cap, so the count may be a floor — the reducer could not check (collector half, landing separately) |

Four rules produce those states:

1. **Per-figure, not per-repo, accounting.** Each figure carries its own
   `coverage.{configured,measured,failed,missing,truncated,truncationUndeclared}` set. One repo
   can be excluded from one figure and included in three, and the row says so (defect 3).
2. **Four states, visible at the number** (defects 1, 2).
3. **A gapped or truncated source refuses to publish a count** rather than publishing a smaller
   one (defects 1, 3, 8). The partial sum survives in JSON as `measuredSubtotal`,
   explicitly not the figure, and never appears in the markdown.
4. **Comparisons require comparable coverage.** A day-over-day delta is computed only when both
   days measured that figure over the **same** source set (defect 4); a prior roll-up
   that exists and fails to parse is reported as a parse failure, never as "no prior day"
   (defect 5).

### Per-grouping `rollup.json` shape

```jsonc
{
  "grouping": "product-a",
  "date": "2026-08-13",
  "runID": "12345",
  "source": "snapshot-artifact",        // or snapshot-unreadable / snapshot-absent / snapshot-date-mismatch
  "reposConfigured": ["example-org/alpha", "example-org/beta"],
  "prsOpenByState": {
    "draft": {
      "value": 35,                      // null for every state but "measured"
      "state": "measured",
      "note": "",
      "coverage": {
        "configured": ["…"], "measured": ["…"], "failed": [], "missing": [],
        "truncated": [], "truncationUndeclared": ["…"]
      }
    },
    "ready": { "…": "…" },
    "total": { "…": "…" }
  },
  "prsOpenByReviewDecision": null,      // until the collector lands — null, never zero
  "prsOpenByReviewDecisionNote": "No source declares prsOpenByReviewDecision yet (collector half, landing separately) …",
  "issuesOpenByLabel": {
    "labels": { "bug": 12, "P1": 4 },   // null unless measured; an OCCURRENCE count, never an issue count
    "state": "measured",
    "coverage": { "…": "…" }
  },
  "issuesOpenUnlabeled": { "…": "same measure shape" }
}
```

### Top-level `rollup.json` shape

The products **side by side**, an **all-products total**, and the org grouping as a
**distinct** section — org-wide activity is never summed into the product total:

```jsonc
{
  "date": "2026-08-13",
  "runID": "12345",
  "products": { "product-a": { "…": "…" }, "product-b": { "…": "…" } },
  "allProducts": { "…": "the same figure set, summed across the products" },
  "org": { "…": "a per-grouping rollup, NOT included in allProducts" },
  "trend": {
    "previousDate": "2026-08-12",
    "previousPath": "reports/daily/2026-08-12/rollup.json",
    "state": "computed",                // or no-prior-day / prior-unreadable
    "products": { "product-a": { "prsOpenTotal": { "value": -3, "state": "computed" } } }
  }
}
```

`allProducts` is `measured` only when **every** contributing grouping was — a grouping that
configured nothing enters the combined coverage as `notConfigured` and cannot vanish into a
complete-looking union.

## Auth

The reducer makes **no live read at all** — it consumes only the downloaded snapshot
artifacts. The collector half reads GitHub under a least-privilege, per-grouping token minted
at runtime (a token is scoped to a single account, so a grouping that spans two owners mints
one token per owner). See your harvest workflow for the concrete auth wiring.

## Running the tests

```bash
cd tools/metrics-harvest
go build ./... && go vet ./... && go test ./...
```
