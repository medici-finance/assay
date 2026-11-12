---
brief: assay:assay:intake-class-routing:05
title: New cells pin the intake role to a strong tier
why: >-
  The intake skill reserves its judgment work for a strong model tier, but a newly scaffolded
  cell starts the intake role on the mid-tier default, and an intake session on that default
  is told to do mechanical work only. So a new cell judges nothing, records no handoffs and
  groups no classes until an operator notices and pins the role by hand. Scaffolding the pin
  makes a new cell's intake able to do its job from the first boot, at a higher model cost
  for that one role.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
schema: brief-v2
version: 1
id: 1d9db8d1-7cdc-4ae1-a550-15e932c381eb
authored: "2026-11-12 by a strong-tier authoring session (read-only review)"
domain: clear
sources:
  - "docs/streams/intake-class-routing/README.md §1.2, §3, §4.5, §8"
  - "plugins/assay/skills/intake-desk/SKILL.md — the model requirement section, unchanged by this brief"
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): every scaffold site writes only the default pin and the coordinator pin"
consumers:
  - "tools/desk/cmd/cellctl/new.go: follow-up intake-class-routing/05 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/cellctl/testdata/cellctl-shell-oracle.sh: follow-up intake-class-routing/05 (this brief; the parity oracle scaffolds the same lines)"
  - "docs/cellctl.md: follow-up intake-class-routing/05 (this brief; the Pinned models section)"
  - "tools/desk/cmd/cellctl/model.go: out-of-scope (resolution order and the role tier map are read, not changed)"
  - "tools/desk/cmd/cellctl/modelreset.go: out-of-scope (an explicit per-role pin is already spared by the reset; row 2 proves it for the new line)"
---

# Brief 05 — New cells pin the intake role to a strong tier

## Context

files: tools/desk/cmd/cellctl/new.go, tools/desk/cmd/cellctl/new_intake_pin_test.go, tools/desk/cmd/cellctl/help/concepts.txt, tools/cellctl/testdata/cellctl-shell-oracle.sh, docs/cellctl.md, changelog/cell-intake-strong-pin.md

facts:
- Read at `0665bfb77` (2026-11-12).
- `cellctl new` writes the model pins at three scaffold sites in `tools/desk/cmd/cellctl/new.go`: the shared `pinnedBlock` constant (line 221), the container cell body (line 319) and the scrubbed cell body (line 543). Each writes exactly `DESK_MODEL_DEFAULT=sonnet` and `DESK_MODEL_the_desk=fable` (`grep -n 'DESK_MODEL_' tools/desk/cmd/cellctl/new.go`).
- A role's model resolves in this order: its own `DESK_MODEL_<role>` pin, then `DESK_MODEL_DEFAULT`, then the tier map (`resolveRoleModel`, `tools/desk/cmd/cellctl/model.go:153`). Because the scaffold sets the default, the tier map is never reached for intake on a new cell; the fix must be an explicit per-role line.
- The role key for intake is `DESK_MODEL_intake_desk` (`underscore(role)`, `model.go:176`).
- Model aliases map to tiers as fable=top, opus=strong, sonnet=mid, haiku=fast (`tools/desk/cmd/cellctl/policy.go:32`).
- The shell oracle `tools/cellctl/testdata/cellctl-shell-oracle.sh` scaffolds the same lines and a parity test compares the two.
- The periodic model reset spares an explicit per-role pin (`TestModelResetSparesExplicitPin`, `tools/desk/cmd/cellctl/modelreset_test.go`).
- The intake skill's model requirement: a session below a strong tier does mechanical work only and does not triage, author decisions or scope.
- Scope limit: this is the Claude-harness pin namespace only. The Codex and Cursor namespaces are not changed, and an existing cell is not rewritten; an operator pins an existing cell with `cellctl set <cell> DESK_MODEL_intake_desk=opus`.
- This is a default change, not a defect fix with a reusable shape, so no class-guard row applies. The three scaffold sites are covered by one test each in row 1.

design-fit:
  owner: tools/desk/cmd/cellctl (the cell scaffold)
  contract: none — a scaffolded default; no registered meaning changes
  retires: []
  weight: "verbs 0, flags 0, refusals 0, scaffolded cell.env lines +1 per site, rule-text lines 0"
  why-add: "The requirement already exists in the intake skill and nothing in a new cell satisfies it. Removal considered: changing the intake role's tier in the tier map instead, which has no effect while the scaffold sets a default pin and would also change other harnesses."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not change `resolveRoleModel`, `roleTier`, the coordinator's pin or its Opus refusal, or any existing cell's files.

## Task

1. Re-check the facts. If any scaffold site already writes an intake pin, report NEEDS_CONTEXT.
2. At each of the three scaffold sites, write `DESK_MODEL_intake_desk=opus` directly after the `DESK_MODEL_the_desk=fable` line. In `pinnedBlock` and the scrubbed body, add one comment line above it saying intake's judgment work needs a strong tier and that removing the line returns the role to the default.
3. Make the same change in the shell oracle so the parity test compares equal.
4. In `docs/cellctl.md`'s Pinned models section and in `help/concepts.txt`, state the new scaffolded line, why it exists, and the one command that applies it to an existing cell.
5. Add `TestNewPinsIntakeStrong` (planned): for each cell kind the scaffold supports in a test, a freshly scaffolded `cell.env` contains the line exactly once and the intake role resolves to `opus` from `DESK_MODEL_intake_desk`, while another non-coordinator role still resolves to the default.
6. Add `TestIntakePinSurvivesReset` (planned): the model reset run over a freshly scaffolded cell leaves the intake line unchanged.
7. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewPinsIntakeStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestNewPinsIntakeStrong ' "$o")` | exit 0 and the named PASS; every scaffolded cell kind resolves intake to the strong pin and leaves other roles on the default |
| 2 | check:ci +neighbour | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakePinSurvivesReset$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestIntakePinSurvivesReset ' "$o")` | exit 0 and the named PASS; the model reset does not undo the scaffolded pin |
| 3 | check:ci | tree | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl)` | exit 0; output contains "ok" |
| 4 | check:ci | tree | `sh tools/cellctl/tests/parity.test.sh` | exit 0; the Go scaffold and the shell oracle still agree |
| 5 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:05` | exit 0; output contains "0 disproved" |

Failure modes mapped: a scaffold site missed → row 1; the pin written but not the one resolution reads → row 1; the reset silently reverting it → row 2; the coordinator pin or refusal disturbed → row 3; the oracle left behind → row 4. Whether `opus` remains the right alias for the strong tier is not decided here; the alias table in `policy.go` owns it.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
