### Added
- `opmetrics` now emits an **attention-class breakdown** (`operator.attention_families`) alongside the relay families — counts of route / status / toil / correction / decision / idea / ack / other for each non-empty operator turn (classifier `opmetrics-relay/2`, day-file schema `opmetrics/2`). Every `opmetrics/1` key is unchanged, so existing readers keep working and the relay-ratio trend line is unbroken.
- `opmetrics --transcripts` is now **repeatable** and defaults to **every `~/.claude*/projects`** profile on the machine, deduping a session synced across profiles so it is counted once. Zero readable roots is `could-not-check`, never a silent zero.
- `statusgen --lint` gains an **`opmetrics-stale` NOTICE** when the newest operator-load day-file under a root is older than three days — a root that has never carried one stays silent.

### Changed
- `statusgen`'s ladder/autonomy day-file reader tolerates the additive `opmetrics/2` schema with no metric change (proven by a v2-fixture schema-tolerance test).
