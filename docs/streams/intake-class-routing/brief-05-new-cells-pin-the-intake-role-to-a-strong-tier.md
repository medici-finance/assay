---
brief: assay:assay:intake-class-routing:05
title: New cells pin the intake role to a strong tier
why: >-
  The intake skill reserves its judgment work for a strong model tier, but a newly scaffolded
  cell starts the intake role on the mid tier, and an intake session there is told to do
  mechanical work only. So a new cell judges nothing, records no handoffs and groups no
  classes until an operator notices. The mid tier is set in two places, the seeded provider
  defaults and the scaffolded cell file, and the first overrides the second. Changing both
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
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): every scaffold site writes only the default pin and the coordinator pin; the provider-defaults seed sets intake-desk to tier mid for all four providers"
consumers:
  - "tools/desk/cmd/cellctl/providers.json: follow-up intake-class-routing/05 (this brief; the provider-defaults seed, which overrides cell pins on house and k8s cells)"
  - "docs/cellctl-model-policy.md: follow-up intake-class-routing/05 (this brief; the seeded desk tiers)"
  - "tools/desk/cmd/cellctl/new.go: follow-up intake-class-routing/05 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/cellctl/testdata/cellctl-shell-oracle.sh: follow-up intake-class-routing/05 (this brief; the parity oracle scaffolds the same lines)"
  - "docs/cellctl.md: follow-up intake-class-routing/05 (this brief; the Pinned models section)"
  - "tools/desk/cmd/cellctl/model.go: out-of-scope (resolution order and the role tier map are read, not changed)"
  - "tools/desk/cmd/cellctl/modelreset.go: out-of-scope (an explicit per-role pin is already spared by the reset; row 2 proves it for the new line)"
---

# Brief 05 — New cells pin the intake role to a strong tier

## Context

files: tools/desk/cmd/cellctl/providers.json, tools/desk/cmd/cellctl/new.go, tools/desk/cmd/cellctl/new_intake_pin_test.go, docs/cellctl-model-policy.md, tools/desk/cmd/cellctl/help/concepts.txt, tools/cellctl/testdata/cellctl-shell-oracle.sh, docs/cellctl.md, changelog/cell-intake-strong-pin.md

facts:
- Read at `0665bfb77` (2026-11-12).
- Two mechanisms decide a role's model, and the first wins outright. On a house or k8s cell, a model policy or the provider defaults (`CELLS_ROOT/providers.json`, with an optional per-cell override) supersede the `DESK_MODEL_*` pins entirely: provider, model and effort all come from that file (`tools/desk/cmd/cellctl/desk.go:79`, `cellModelPolicy` in `provider_defaults.go:162`). A container or scrubbed cell never takes provider defaults, so there the cell file's pins decide.
- `cellctl providers init` creates `CELLS_ROOT/providers.json` from the embedded seed `tools/desk/cmd/cellctl/providers.json` and never overwrites an existing file. The seed gives `intake-desk` tier `mid` under every provider's `desks` (`grep -n -A2 '"intake-desk"' tools/desk/cmd/cellctl/providers.json`); `pr-review-desk` and `verify-desk` are already `strong` there.
- A window launched under a policy pins that policy's SHA-256, and its hook refuses once the file changes ("cell policy changed since this window launched … restart the window", `policy_enforce.go:203`). Editing an existing provider-defaults file therefore requires restarting that cell's running windows.
- Observed 2026-11-12 on one installation: a per-role pin set with `cellctl set` had no effect on a house cell, because that cell resolves through provider defaults.
- `cellctl new` writes the model pins at three scaffold sites in `tools/desk/cmd/cellctl/new.go`: the shared `pinnedBlock` constant (line 221), the container cell body (line 319) and the scrubbed cell body (line 543). Each writes exactly `DESK_MODEL_DEFAULT=sonnet` and `DESK_MODEL_the_desk=fable` (`grep -n 'DESK_MODEL_' tools/desk/cmd/cellctl/new.go`).
- A role's model resolves in this order: its own `DESK_MODEL_<role>` pin, then `DESK_MODEL_DEFAULT`, then the tier map (`resolveRoleModel`, `tools/desk/cmd/cellctl/model.go:153`). Because the scaffold sets the default, the tier map is never reached for intake on a new cell; the fix must be an explicit per-role line.
- The role key for intake is `DESK_MODEL_intake_desk` (`underscore(role)`, `model.go:176`).
- Model aliases map to tiers as fable=top, opus=strong, sonnet=mid, haiku=fast (`tools/desk/cmd/cellctl/policy.go:32`).
- The shell oracle `tools/cellctl/testdata/cellctl-shell-oracle.sh` scaffolds the same lines and a parity test compares the two.
- The periodic model reset spares an explicit per-role pin (`TestModelResetSparesExplicitPin`, `tools/desk/cmd/cellctl/modelreset_test.go`).
- The intake skill's model requirement: a session below a strong tier does mechanical work only and does not triage, author decisions or scope.
- Scope limit: the cell-file pin is the Claude-harness namespace only; the Codex and Cursor pin namespaces are not changed. No existing cell file and no existing provider-defaults file is rewritten. An operator moves an existing installation by setting `intake-desk` to tier `strong` in its provider-defaults file where one exists, or with `cellctl set <cell> DESK_MODEL_intake_desk=opus` where none does.
- This is a default change, not a defect fix with a reusable shape, so no class-guard row applies. Row 1 covers every seeded provider and row 2 every scaffold site.

design-fit:
  owner: tools/desk/cmd/cellctl (the cell scaffold)
  contract: none — a scaffolded default; no registered meaning changes
  retires: []
  weight: "verbs 0, flags 0, refusals 0, seeded tier values changed 4, scaffolded cell.env lines +1 per site, rule-text lines 0"
  why-add: "The requirement already exists in the intake skill and nothing in a new cell satisfies it. Removal considered: changing the intake role's tier in the tier map instead, which has no effect while the scaffold sets a default pin. Changing only the cell file was also considered and rejected: provider defaults override it on house and k8s cells."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not change `resolveRoleModel`, `roleTier`, the coordinator's pin or its Opus refusal, or any existing cell's files.

## Task

1. Re-check the facts. If any scaffold site already writes an intake pin, or the seed already gives intake a tier above `mid`, report NEEDS_CONTEXT.
2. In the embedded seed `tools/desk/cmd/cellctl/providers.json`, set `intake-desk` to tier `strong` under every provider's `desks`. Change no other desk, tier or model. If the shell oracle or any fixture carries a copy of the seed, change it identically.
3. At each of the three scaffold sites, write `DESK_MODEL_intake_desk=opus` directly after the `DESK_MODEL_the_desk=fable` line. In `pinnedBlock` and the scrubbed body, add one comment line above it saying intake's judgment work needs a strong tier and that removing the line returns the role to the default.
4. Make the same cell-file change in the shell oracle so the parity test compares equal.
5. In `docs/cellctl.md`'s Pinned models section, `docs/cellctl-model-policy.md` and `help/concepts.txt`, state the new seeded tier and scaffolded line, why they exist, which of the two applies on which cell kind, how an operator moves an existing installation, and that changing an existing provider-defaults file requires restarting that cell's windows.
6. Add `TestSeedIntakeTierStrong` (planned): the embedded seed parses as a valid policy; under every provider the intake desk resolves to that provider's `strong` tier model, and `worker-desk` still resolves to `mid`.
7. Add `TestNewPinsIntakeStrong` (planned): for each cell kind the scaffold supports in a test, a freshly scaffolded `cell.env` contains the line exactly once and the intake role resolves to `opus` from `DESK_MODEL_intake_desk`, while another non-coordinator role still resolves to the default.
8. Add `TestIntakePinSurvivesReset` (planned): the model reset run over a freshly scaffolded cell leaves the intake line unchanged.
9. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSeedIntakeTierStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestSeedIntakeTierStrong ' "$o")` | exit 0 and the named PASS; under every seeded provider intake resolves to the strong tier and the worker desk stays on mid |
| 2 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewPinsIntakeStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestNewPinsIntakeStrong ' "$o")` | exit 0 and the named PASS; every scaffolded cell kind resolves intake to the strong pin and leaves other roles on the default |
| 3 | check:ci +neighbour | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakePinSurvivesReset$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestIntakePinSurvivesReset ' "$o")` | exit 0 and the named PASS; the model reset does not undo the scaffolded pin |
| 4 | check:ci | tree | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl)` | exit 0; output contains "ok" |
| 5 | check:ci | tree | `sh tools/cellctl/tests/parity.test.sh` | exit 0; the Go scaffold and the shell oracle still agree |
| 6 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:05` | exit 0; output contains "0 disproved" |

Failure modes mapped: the seed left on mid, so the cell-file pin is overridden on house and k8s cells → row 1; another desk's tier moved by accident → row 1; a scaffold site missed, or the pin written where resolution does not read it → row 2; the reset silently reverting it → row 3; the coordinator pin or refusal disturbed → row 4; the oracle left behind → row 5. Whether `opus` remains the right alias for the strong tier is not decided here; the alias table in `policy.go` owns it.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
