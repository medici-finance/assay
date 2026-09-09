### Added
- `statusgen --dora-json` emits the frozen full-DORA publish feed — `change_lead_time`,
  `change_failure_rate` and `time_to_restore` — as byte-stable JSON on stdout, ready for a
  metrics publish pipeline to wrap verbatim. Deployment frequency is deliberately absent:
  it is sourced by a separate delivery-metrics platform, and two emitters writing one
  frozen key is a collision the shape cannot resolve.
- `--dora-limit` (default 500) caps each recorded timing series the feed aggregates. The cap
  is declared in the feed's own `caps` block, so a capped number is never published as though
  it covered the whole window, and a capped metric reports itself `partial` rather than
  `measured`.

### Changed
- A DORA metric with no recorded history renders `{"computed": false, "state":
  "could-not-check", "value": null}` with `needs` naming what is missing — never a fabricated
  `0`. The lead-time and restore metrics read the recorded timing log; the change-failure
  number is an explicitly unlinked proxy over completed work, recorded reversions and open
  defect records, and is always `partial` because it lacks the change-to-incident linkage the
  canonical metric is defined over. A window with no completed work has no denominator and is
  `could-not-check`, not a 0% failure rate.
- The retained `--dora` / `--trend` grouped back-compat aliases are untouched; `--dora-json`
  is a new output mode over the same retained computation, not a revival of the removed
  standalone DORA CLI.
