# Hooks — assay plugin

This directory holds the plugin's hook configuration.

## SessionStart hook

`hooks.json` + `inject-resident-rules.sh` — fires on every session start,
injecting the portable resident operating rules into the session context.
These rules (evidence-not-claims, isolation, neutral-dispatch wording,
out-of-repo protocol pointer, etc.) were previously carried in CLAUDE.md
residency; the hook is the structural fix (the single home that supersedes
loose `~/.claude` rule files).

The rules text — including the version the banner announces — is GENERATED
from the single source `plugins/assay/resident-rules.md` by
`go run ./tools/harnessgen resident` (see that file's own header for the
mechanism); `inject-resident-rules.sh` reads the generated
`resident-rules.payload.txt` rather than carrying its own copy, and the
banner's version is derived from `plugins/assay/.claude-plugin/plugin.json`,
never hand-typed. Do not restate a version number, or any rule text, directly
in this README or the script — that is the drift `harnessgen resident --check`
exists to catch (#730).

### Scope — this fires in EVERY session

The hook registers `SessionStart` with `"matcher": "*"`. Once the plugin is
installed there is no per-project or per-skill narrowing: **every session you
start, in every project, receives the injected rules** — not just sessions that
invoke an `assay:*` skill. This adds a `systemMessage` of a couple thousand
characters to the context of each of those sessions; the exact size moves with
the rule text and the plugin version, so measure it rather than trust a number
written down here:

```sh
bash hooks/inject-resident-rules.sh | jq -r '.systemMessage | length'        # characters
bash hooks/inject-resident-rules.sh | jq -j  .systemMessage | wc -c          # bytes UTF-8
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
