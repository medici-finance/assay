# Hooks — assay plugin v0.1.0

This directory holds the plugin's hook configuration.

## SessionStart hook

`hooks.json` + `inject-resident-rules.sh` — fires on every session start,
injecting the portable resident operating rules into the session context.
These rules (evidence-not-claims, isolation, neutral-dispatch wording,
out-of-repo protocol pointer, etc.) were previously carried in CLAUDE.md
residency; the hook is the structural fix (the single home that supersedes
loose `~/.claude` rule files — methodology/22).

### Scope — this fires in EVERY session

The hook registers `SessionStart` with `"matcher": "*"`. Once the plugin is
installed there is no per-project or per-skill narrowing: **every session you
start, in every project, receives the injected rules** — not just sessions that
invoke an `assay:*` skill. The payload is currently 2432 characters of
`systemMessage` (2444 bytes UTF-8 — the body carries six em-dashes), added to
the context of each of those sessions. Measure it the same way to get the same
number:

```sh
bash hooks/inject-resident-rules.sh | jq -r '.systemMessage | length'        # 2432 characters
bash hooks/inject-resident-rules.sh | jq -j  .systemMessage | wc -c          # 2444 bytes
```

If you want the rules only in desk sessions, do not install the plugin
globally — install it per-project, or drop `hooks/` from your copy and rely on
the `assay:*` skill bodies alone.

### Requirements

- **`jq`** must be on `PATH`. `inject-resident-rules.sh` uses `jq -Rs` to
  JSON-encode the rule text, and it is the hook's only external dependency.
  There is no manifest field for system binaries — the plugin manifest's
  `dependencies` key declares *plugin* dependencies, not executables — so this
  README is where the requirement is recorded.
- The script runs under `bash` with `set -euo pipefail`. If `jq` is missing the
  script exits non-zero and emits nothing, so the session starts without the
  rules rather than with a malformed `systemMessage`. It fails closed.
