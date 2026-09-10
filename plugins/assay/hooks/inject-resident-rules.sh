#!/bin/bash
# SessionStart hook — injects the portable resident operating rules into the session context.
# These are the project-agnostic rules the desk skills rely on; they were carried in CLAUDE.md
# residency before the plugin absorbed them. Keep injection SMALL: rules, not rationale.
# Pointers for narrative: see the assay:* skill bodies for context.

set -euo pipefail

# The payload text — including the version banner — is GENERATED from the single
# source (plugins/assay/resident-rules.md) by `go run ./tools/harnessgen resident`;
# do not hand-edit resident-rules.payload.txt or re-embed the rules text here.
# Read it relative to THIS script, not $CLAUDE_PLUGIN_ROOT or the caller's cwd, so
# the hook works both as the plugin invokes it and as this file's own README
# documents running it directly (`bash hooks/inject-resident-rules.sh`).
# assay#730: a hand-embedded copy here is exactly how the banner drifted from the
# plugin manifest's version (and, discovered alongside it, from the payload's own
# rule 8 wording) — a single generated file removes the second copy to drift from.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAYLOAD="${SCRIPT_DIR}/resident-rules.payload.txt"

# Emit resident rules as a JSON systemMessage.
# Uses jq -Rs to read the entire file as a single raw string and JSON-encode it.
jq -Rs '{systemMessage: .}' < "$PAYLOAD"
