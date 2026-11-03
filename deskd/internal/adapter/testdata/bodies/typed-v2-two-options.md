## Situation

The derived index is complete for escalations and incomplete for the roadmap
pane. Shipping now surfaces the queue a week earlier; holding ships both at
once.

### Options
<!-- assay:options v2 -->
#### `ship-partial` — Ship the partial index
Consequence: inbox live today; roadmap pane lags a week.
Pros: the ten invisible escalations become visible now.
Cons: two panes render could-not-check for a week.

#### `wait-for-fix` — Hold until the roadmap derivation lands
Consequence: nothing ships this week; no rework risk.
Pros: one release, one review.
Cons: the queue stays invisible for another week.
