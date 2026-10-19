### Changed
- `clusterguard` classifies the talosctl `services` read-only entry on what follows the subcommand, not on the first token alone: the bare form stays on the read-only tier, and a form with a positional argument after it is classified like an unlisted verb (refused under `ASSAY_ALLOW_CLUSTER=1`, exit 5, with a refusal that says the entry is read-only only in its bare form).
