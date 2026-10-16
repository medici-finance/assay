### Fixed
- `cellctl` perf records (with `ASSAY_TELEMETRY=1`) now carry the verb's real name (`desk`, `cache`, `show`, `model-policy`, ...) instead of `other`, including when a leading `--cells-root` selector is given. The roster is derived from the command tree, so a new verb is recorded under its own name automatically.
- A `cellctl desk` launch now writes `step:launch-*` perf rows for its pre-handover phases (role lease, storage admission, plugin enable, main fetch, worktree), before the session starts, so a slow launch shows up in `deskperf boot`.
