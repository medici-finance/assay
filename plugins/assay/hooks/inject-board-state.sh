#!/bin/bash
# SessionStart hook — surface the assay board's front-door state into EVERY session, so the
# flow comes to the session instead of requiring a dedicated window. Without an ambient nudge,
# the front-door flow tends to go unused because acting on it depends on someone booting a
# dedicated window; surfacing the untriaged count in every session removes that tax.
#
# HARD CONSTRAINTS (this runs on every single session start):
#   - LOCAL-ONLY: no gh, no network, no statusgen invocation. Pure file reads on the cwd repo.
#   - FAST + FAIL-CLOSED: any missing tool, missing file, or error -> emit nothing, exit 0.
#     A hook that slows or blocks session start is worse than no hook.
#   - QUIET WHEN EMPTY: say nothing unless there is actually work to surface.
# It is informational only — it never enforces (the "direct work exits through the flow"
# two-mode rule is standing policy that waits on the human's sign-off of the issue-flow
# rulings; this hook does not pre-empt that decision).

set -uo pipefail   # NOT -e: grep -c exits 1 on zero matches, which is a normal outcome here.

# Only speak inside an assay-tracked repo (a docs/streams register present in the session cwd).
[ -f "docs/streams/INTAKE.md" ] || exit 0
command -v jq >/dev/null 2>&1 || exit 0   # no jq -> stay silent rather than emit malformed output.

# Cheap local signal: untriaged intake entries. Matches bare `Disposition: new` AND the
# `Disposition: new — proposed …` ratify-me limbo (both are genuinely untriaged; the latter is
# exactly the class the statusgen age-alarm tends to under-count).
untriaged="$(grep -cE '^Disposition:[[:space:]]*new' docs/streams/INTAKE.md 2>/dev/null || true)"
[ -z "$untriaged" ] && untriaged=0

# Nothing untriaged -> the front door is clear; stay silent.
[ "$untriaged" -eq 0 ] 2>/dev/null && exit 0

entries="entries"
[ "$untriaged" -eq 1 ] 2>/dev/null && entries="entry"

printf 'ASSAY FRONT DOOR: %s intake %s untriaged (docs/streams/INTAKE.md, disposition: new). The board has work waiting on judgment. Options: run the intake-desk skill to triage the front door, or triage inline before you finish this session. (Ambient board nudge — assay plugin SessionStart hook.)' \
  "$untriaged" "$entries" | jq -Rs '{systemMessage: .}'
