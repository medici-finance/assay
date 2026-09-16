---
brief: assay:assay:desk-containers:11
title: "retire the out-of-tree bridge: migrate `CELL_KIND=local` registrations, remove the shell shim, one `cellctl` on PATH"
wave: 2
depends: ["desk-containers/10"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1193]
schema: brief-v2
authored: 2026-09-16 by the-desk dispatch (issue #1193)
sources:
  - "#1193 — 'What happened': a ~170-line out-of-tree bridge plus a shell function sourced from the operator's shell rc that shadows `cellctl`, routes two cells to the bridge and delegates the rest to the released binary; the operator disabled the rc edit and directed the fold-in. The bridge's registration is `CELL_KIND=local` + `CELL_HARNESS=codex`. 'Why a Go port': three divergent copies on one laptop — the pinned install, a user-local copy, a cell-registry copy"
  - "#1193 — 'What is needed' (c): the bridge retired and its registrations migrated"
  - "tools/cellctl/cellctl:224-236 — the kind switch; `local` currently falls to the `*)` arm and dies `is not a known kind (k8s|house|container)` with no next step named"
  - "tools/cellctl/cellctl:576-632 — `CELL_ENV_KNOWN_KEYS` (includes `CELL_KIND`) and `validate_env_key`: `cellctl set <cell> CELL_KIND=<v>` is already a legal write, so the one-command migration needs no new verb, only a value the loader accepts"
  - "tools/cellctl/cellctl:166-169 — `SELF`, the script's own resolved path; the copy-drift row compares it with `command -v cellctl`"
  - "tools/cellctl/cellctl:181-198 — `CELLCTL_VERSION` and the `--version` contract that makes a stale copy detectable; the docs section below tells the operator to USE it"
  - "docs/cellctl.md §Install — the two-line checkout/tarball install into `~/.local/bin` is exactly the user-local copy #1193 counts as one of the three; the section gains the one-copy rule"
  - "desk-containers/09 — the kind a bridge registration migrates INTO (`scrubbed`) and the `check` rows it must then pass; desk-containers/10 — the Go binary that is `cellctl` on PATH after the cutover, and the parity harness that keeps the bash oracle's message identical"
  - "freshness-checked 2026-09-16 @ 872ac03e — no `local` handling, no copy-drift row, no 'one cellctl on PATH' section exists on main"
why: >-
  The bridge is gone from the operator's shell rc but its registrations still say
  `CELL_KIND=local`, which every copy of `cellctl` refuses with a message that names no way
  out; and the reason the shim could shadow the real tool at all — three copies of `cellctl`
  on one machine — is still true after the port. Naming the migration in the refusal itself,
  making it one command, and making `check` say which copy is running closes the last two
  ways the bridge can come back.
version: 1
id: 148b494e-8ace-4c43-b25c-7ebd57a6073e
consumers:
  - "tools/desk/cmd/cellctl/cell.go: follow-up desk-containers/11 (this brief; the retired-alias refusal, flips to fixed-here when it lands)"
  - "tools/cellctl/cellctl: follow-up desk-containers/11 (this brief; the same refusal text in the bash oracle so the parity harness stays green — the ONE bash edit this chain makes after brief 10)"
  - "tools/cellctl/tests/parity.test.sh: follow-up desk-containers/11 (this brief; one new fixture cell `local/codex`)"
  - "docs/cellctl.md: follow-up desk-containers/11 (this brief; §'One `cellctl` on PATH' and the migration line)"
---

# Brief 11 — retire the out-of-tree bridge: migrate `CELL_KIND=local` registrations, remove the shell shim, one `cellctl` on PATH

## Context

files:
- `tools/desk/cmd/cellctl/cell.go` (planned) — created by desk-containers/10 — the kind loader gains a
  RETIRED-ALIAS arm: `local` is refused (exit 3, like any unknown kind) with the migration line.
- `tools/cellctl/cellctl` — the same arm, same text, in the bash oracle (`load_cell`, :224-236),
  so brief 10's parity harness stays green; no other bash change.
- `tools/cellctl/tests/parity.test.sh` (planned) — created by desk-containers/10 — one added fixture:
  a `CELL_KIND=local` cell, verb `check`, expected divergence 0 (both refuse identically).
- `tools/cellctl/tests/scrubbed-cell.test.sh` (planned) — created by desk-containers/09 — one added
  case `migrate-local`.
- `docs/cellctl.md` — new section *One `cellctl` on PATH* (after §Install) and a *Migrating a
  bridge-era registration* paragraph in §*Scrubbed cells*.
- `changelog/desk-containers-11-retire-bridge.md` (planned) — one `### Changed` bullet.

facts:
- **The refusal text is the migration.** On `CELL_KIND=local` the loader dies, exit 3, with
  exactly: `cell.env: CELL_KIND=local is the retired bridge kind — migrate it: cellctl set
  <cell> CELL_KIND=scrubbed, then cellctl check <cell>` (with the cell name substituted). Any
  other unknown value keeps the existing form, now listing four kinds:
  `is not a known kind (k8s|house|container|scrubbed)`. Both implementations emit the same
  bytes (parity row).
- **One command, because `set` already writes `CELL_KIND`:** `CELL_KIND` is in
  `CELL_ENV_KNOWN_KEYS` (`tools/cellctl/cellctl:576`), so `cellctl set <cell>
  CELL_KIND=scrubbed` is legal today and needs no `--force`. `set` must NOT itself validate the
  kind against the loader (a migration would otherwise be refused by the very check it is
  meant to satisfy); `check` is what proves the migrated cell — and a bridge registration
  lacks `CELL_REPO_SLUG`, its config home is wherever the bridge put it, so the expected first
  `check` after migration is a MISS list, not a pass. The docs paragraph says so and lists the
  scrubbed `check` rows as the to-do the operator works down.
- **The shell shim is not in this tree.** It lived in the operator's shell rc and is already
  disabled (#1193). What this repo can do is (a) never need it again — every bridge behaviour
  is now a `cellctl` verb (desk-containers/09) — and (b) tell the operator how to prove no
  shim remains: `type -a cellctl` (bash) / `whence -a cellctl` (zsh) must list exactly one
  entry, a file. That command is documented, not run by `check` (a non-interactive script
  cannot see an interactive shell's functions).
- **Three copies, one fix.** #1193 names the drift: the pinned install (desk-tools bindir),
  a user-local copy (docs §Install's `~/.local/bin` line), a cell-registry copy. The fix is
  the PIN — one install location, the tarball's sha256-pinned binary — never a shell rc
  shim that picks among copies. `check` gains one row: **running copy is the one on PATH** —
  the running executable's resolved path equals `command -v cellctl` resolved; a mismatch is
  `warn` (visible, non-fatal — `check` from a checkout is legitimate) naming both paths and
  both `--version` outputs. On the bash oracle the running path is `SELF`
  (`tools/cellctl/cellctl:169`); on the Go binary it is `os.Executable()` resolved through
  symlinks.
- **Docs §Install changes one sentence's meaning:** the `~/.local/bin` two-liner stays as
  the checkout/tarball fallback but is preceded by the rule that a machine with a pinned
  desk-tools install has exactly one `cellctl`, the pinned one, and that `cellctl check`'s
  copy row is how to see it.
- **No new Python, no new binary, no new verb.**

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- The bash edit is the retired-alias arm and the copy row ONLY; anything else in the oracle
  is NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Go loader (`cell.go`): add the `local` retired-alias refusal with the exact text in facts;
   widen the unknown-kind list to four.
2. Bash oracle (`load_cell`): the identical two changes.
3. `check` copy row in both implementations (`warn`, per facts).
4. `scrubbed-cell.test.sh --case migrate-local`: a `CELL_KIND=local` fixture → `check` exits
   3 with the migration line; run the printed `cellctl set` line verbatim (parsed out of the
   refusal, not retyped) → `check` now loads and reports MISS rows (exit 1), never exit 3.
5. Parity fixture `local/codex/check` added; harness green.
6. `docs/cellctl.md`: §*One `cellctl` on PATH*, the migration paragraph, the copy row in the
   `check` table.
7. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `d=$(mktemp -d); mkdir -p "$d/b"; printf 'CELL_KIND=local\nCELL_HARNESS=codex\nCELL_REPO=%s\n' "$PWD" > "$d/b/cell.env"; CELLS_ROOT="$d" tools/desk/cellctl check b 2>&1; test $? -eq 3` | exit 0 — exit 3 AND the output contains `retired bridge kind` and `cellctl set b CELL_KIND=scrubbed`. Red on the merge-base (after brief 10): exit 3 with `is not a known kind` and NO `cellctl set` line, so the row's grep half fails | check +dereference |
| 2 | `d=$(mktemp -d); mkdir -p "$d/b"; printf 'CELL_KIND=local\nCELL_HARNESS=codex\nCELL_REPO=%s\n' "$PWD" > "$d/b/cell.env"; line=$(CELLS_ROOT="$d" tools/desk/cellctl check b 2>&1 \| sed -n 's/.*migrate it: \(cellctl set [^,]*\),.*/\1/p'); CELLS_ROOT="$d" tools/desk/cellctl ${line#cellctl }; CELLS_ROOT="$d" tools/desk/cellctl check b; test $? -eq 1` | exit 0 — the migration line PRINTED by the refusal, run verbatim, turns exit 3 into exit 1 (loaded, MISS rows). Dereferencing: a docs-only migration line that drifts from the code cannot pass here because the row never reads the docs | check +dereference +flow |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case migrate-local` | exit 0 (the same flow as row 2, against the bash oracle via `CELLCTL`, plus the unknown-kind message now listing four kinds). Red on the merge-base: `--case migrate-local` is unknown to the suite (exit 2) | check:ci |
| 4 | `CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=tools/desk/cellctl PARITY_ONLY=local/codex/tmux/check bash tools/cellctl/tests/parity.test.sh` | exit 0; `parity: 1 cells, 0 divergent` — both implementations refuse `local` with the same bytes. Red on the merge-base: the fixture cell is unknown to the harness (exit 2) | check:ci +neighbour |
| 5 | `grep -cF 'is not a known kind (k8s\|house\|container\|scrubbed)' tools/cellctl/cellctl` | exit 0; prints `1` (the bash oracle's unknown-kind text lists four kinds; `-F` matches the pipes as literal text) | check |
| 6 | `grep -c '^## One .cellctl. on PATH' docs/cellctl.md` | exit 0; prints `1` | check |
| 7 | `doc=$(grep -o 'cellctl set <cell> CELL_KIND=scrubbed' docs/cellctl.md \| head -1); code=$(grep -o 'cellctl set [$][^ ]* CELL_KIND=scrubbed' tools/cellctl/cellctl \| head -1); test -n "$doc" && test -n "$code"` | exit 0 — the migration command appears in the docs (with `<cell>`) and in the oracle's refusal text (with the cell variable); a rename of the target kind on one side only goes red here | check +dereference |
| 8 | `bash tools/cellctl/tests/house-cell.test.sh && bash tools/cellctl/tests/container-cell.test.sh` | exit 0 — the two neighbouring kinds still pass (they pass on the merge-base too: regression guard) | check:ci +neighbour |
| 9 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | exit 0 | check:ci |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- A bridge-era registration is migrated by running the line `cellctl` itself prints, and the
  migrated cell is then judged by the scrubbed `check` rows (desk-containers/09).
- `cellctl check` names the running copy vs the copy on PATH; the docs say the pin is the
  only sanctioned copy and how to prove no shim remains.
- The bash oracle's diff in this brief is the alias arm and the copy row only.
- `docs/cellctl.md` regenerated (docs-regen item).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a refusal message, a one-command migration through an
existing verb, a `warn`-class check row and a docs section; no credential, no launch path, no
control weakened). Reviewer confirms row 2 runs the PRINTED line (not a retyped one) and that
the bash diff touches nothing outside the two named arms.
