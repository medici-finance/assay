---
brief: apps-installer/06
title: Avatar step in the run board — generated PNG beside the settings page, confirmed not uploaded
why: >-
  GitHub has no API to set an App's avatar, so the drop stays a human click; without a generated
  file beside it the person hunts for an image and most Apps ship with the default identicon. The
  step costs one drag if the file is already on screen, and remembering whether it happened is what
  keeps status honest.
wave: 3
depends: ["apps-installer/02", "apps-installer/05"]
unblocks: ["apps-installer/07"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §2 Screen 1 (avatars previewed), Screen 2 (Avatar cell), §4 (`keyed → avatar_ok`; unconfirmed does not block install), §6 (`deskapps avatar --regen`), §8 (avatar never dropped)."
  - "GitHub App avatars are set only in the App's settings page (Display information); the manifest schema has no avatar field and no REST endpoint uploads one."
  - "apps-installer/05 — `internal/avatar.Generate` and the file layout `~/.config/assay/avatars/<app>.png`."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — nothing in the tree references an App avatar."
exec-tier: any
---

# Brief 06 — The avatar step

## Context
files:
- `tools/desk/cmd/deskapps/avatar.go` (new), `page/` (Screen 1 preview, Screen 2 Avatar cell).
- `docs/desk-tools/deskapps.md` (planned) § Avatar.

facts:
- Settings page URL for an App: org-owned `https://github.com/organizations/<org>/settings/apps/<slug>`;
  personal `https://github.com/settings/apps/<slug>`.
- Files: `~/.config/assay/avatars/<app>.png` + `.svg`, generated at `init` (Screen 1 preview) via
  `internal/avatar.Generate`; `deskapps avatar --regen` regenerates and resets every row's
  `avatar_confirmed` to false.
- The Avatar cell opens the settings page and shows the PNG with the instruction *"Drag this file
  onto the avatar area, save, then click Confirm here."* Confirm is the `confirm-avatar` page verb.
- An unconfirmed avatar never blocks Install or Verify; `status` and Screen 3 show a warn chip
  until confirmed.
- The proof metric of brief 05 runs at `init`; a failing pair is a refusal (exit 5) before any
  form is served — a bad palette never reaches an upload.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Generate at `init`; preview on Screen 1; write files.
2. Avatar cell: settings link, image, instruction, Confirm → `avatar_confirmed=true`, console line.
3. `deskapps avatar --regen`.
4. Warn chip on `status` and Screen 3 for unconfirmed rows.
5. Docs § Avatar.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -run 'Avatar' -count=1` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestInitWritesAvatars' -count=1 -v 2>&1 \| grep -cE -e 'avatars/example-read\.png' -e 'avatars/example-act\.png'` | 2 |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestAvatarCellLinksSettings' -count=1 -v 2>&1 \| grep -cE -e 'organizations/example/settings/apps/example-read' -e 'Drag this file'` | ≥ 2 |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestUnconfirmedAvatarDoesNotBlockInstall' -count=1` | exit 0 |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRegenResetsConfirmed' -count=1` | exit 0 |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestInitRefusesFailingProof' -count=1 -v 2>&1 \| grep -cE -e 'exit 5' -e 'no form served'` | ≥ 1 |
| 7 | `grep -cE -e '^## Avatar' docs/desk-tools/deskapps.md` | 1 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
