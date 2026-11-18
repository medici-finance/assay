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
  for that one role. An operator who pins the role by hand on a cell where the pin is not
  read is told nothing today; the same change makes the tool say so and name the file to edit.
wave: 0
depends: []
unblocks: []
effort: M
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
  - "tools/desk/cmd/cellctl/set.go: follow-up intake-class-routing/05 (this brief; the notice when a written model pin is not read)"
  - "tools/desk/cmd/cellctl/provider_defaults.go: out-of-scope (the shared file and the per-cell override merge are read, not changed)"
  - "tools/desk/cmd/cellctl/model.go: out-of-scope (resolution order and the role tier map are read, not changed)"
  - "tools/desk/cmd/cellctl/modelreset.go: out-of-scope (an explicit per-role pin is already spared by the reset; row 2 proves it for the new line)"
---

# Brief 05 — New cells pin the intake role to a strong tier

## Context

files: tools/desk/cmd/cellctl/providers.json, tools/desk/cmd/cellctl/new.go, tools/desk/cmd/cellctl/set.go, tools/desk/cmd/cellctl/new_intake_pin_test.go, docs/cellctl-model-policy.md, tools/desk/cmd/cellctl/help/concepts.txt, tools/cellctl/testdata/cellctl-shell-oracle.sh, docs/cellctl.md, changelog/cell-intake-strong-pin.md

facts:
- Read at `0665bfb77` (2026-11-12).
- Two mechanisms decide a role's model, and the first wins outright. On a house or k8s cell, a model policy or the provider defaults (`CELLS_ROOT/providers.json`, with an optional per-cell override) supersede the `DESK_MODEL_*` pins entirely: provider, model and effort all come from that file (`tools/desk/cmd/cellctl/desk.go:79`, `cellModelPolicy` in `provider_defaults.go:162`). A container or scrubbed cell never takes provider defaults, so there the cell file's pins decide.
- `cellctl providers init` creates `CELLS_ROOT/providers.json` from the embedded seed `tools/desk/cmd/cellctl/providers.json` and never overwrites an existing file. The seed gives `intake-desk` tier `mid` under every provider's `desks` (`grep -n -A2 '"intake-desk"' tools/desk/cmd/cellctl/providers.json`); `pr-review-desk` and `verify-desk` are already `strong` there.
- A window launched under a policy pins that policy's SHA-256, and its hook refuses once the file changes ("cell policy changed since this window launched … restart the window", `policy_enforce.go:203`). Editing an existing provider-defaults file therefore requires restarting that cell's running windows.
- A cell can already differ from the shared provider defaults: a `providers.json` in the cell's own directory, or the file `CELL_PROVIDER_OVERRIDES` names, is merged over the shared file key by key (`mergeProviderOverrides`, `provider_defaults.go:119`; the cell-local case is covered in `provider_defaults_test.go`). One desk's tier can be changed for one cell with a file holding only that key. A cell carrying `CELL_MODEL_POLICY` takes its whole policy from that file instead and is out of this brief's scope: nothing here rewrites a policy file.
- Observed 2026-11-12 on one installation: a per-role pin set with `cellctl set <cell> DESK_MODEL_intake_desk=opus` printed a normal success line and had no effect on a house cell, because that cell resolves through provider defaults. `cellctl desk --set` already refuses in the same situation ("--set with a model policy is ambiguous; edit the policy or provider defaults instead", `desk.go:94`); the `KEY=VALUE` form of `cellctl set` does not consult the policy at all (`validateEnvKey`, `set.go:29`). The same silence applies when the cell's harness reads a different pin namespace.
- `cellctl new` writes the model pins at three scaffold sites in `tools/desk/cmd/cellctl/new.go`: the shared `pinnedBlock` constant (line 221), the container cell body (line 319) and the scrubbed cell body (line 543). Each writes exactly `DESK_MODEL_DEFAULT=sonnet` and `DESK_MODEL_the_desk=fable` (`grep -n 'DESK_MODEL_' tools/desk/cmd/cellctl/new.go`). The comment a few lines below in `pinnedBlock` says "the two DESK_MODEL_* lines above are Claude-only" (line 228) and goes stale when a third line is added.
- A role's model resolves in this order: its own `DESK_MODEL_<role>` pin, then `DESK_MODEL_DEFAULT`, then the tier map (`resolveRoleModel`, `tools/desk/cmd/cellctl/model.go:153`). Because the scaffold sets the default, the tier map is never reached for intake on a new cell; the fix must be an explicit per-role line.
- The role key for intake is `DESK_MODEL_intake_desk` (`underscore(role)`, `model.go:176`).
- Model aliases map to tiers as fable=top, opus=strong, sonnet=mid, haiku=fast (`tools/desk/cmd/cellctl/policy.go:32`).
- The shell oracle `tools/cellctl/testdata/cellctl-shell-oracle.sh` scaffolds the same lines and a parity test compares the two. `grep -n 'DESK_MODEL_the_desk=fable' tools/cellctl/testdata/cellctl-shell-oracle.sh` prints its cell-file sites: lines 3380, 3438, 3495, 3537 and 3618 at this commit.
- The periodic model reset spares an explicit per-role pin (`TestModelResetSparesExplicitPin`, `tools/desk/cmd/cellctl/modelreset_test.go`).
- The intake skill's model requirement: a session below a strong tier does mechanical work only and does not triage, author decisions or scope.
- Scope limit: the cell-file pin is the Claude-harness namespace only; the Codex and Cursor pin namespaces are not changed. No existing cell file and no existing provider-defaults file is rewritten. An operator moves an existing installation in one of three ways: for every cell, by setting `intake-desk` to tier `strong` in the shared provider-defaults file; for one cell, by putting that one key in the cell's own `providers.json`; or, on a cell that resolves through neither a policy nor provider defaults, with `cellctl set <cell> DESK_MODEL_intake_desk=opus`.
- The seed and scaffold changes are a default change with no reusable defect shape. The silent pin is a defect with one: a setting accepted by a verb and read by nothing. Row 7 guards it for model pins in both cases named in Task 5, each with its own subtest and its own no-notice control.

design-fit:
  owner: tools/desk/cmd/cellctl (the cell scaffold)
  contract: none — a scaffolded default; no registered meaning changes
  retires: []
  weight: "verbs 0, flags 0, refusals 0, notices +1 (a written model pin the cell will not read), seeded tier values changed 4, scaffolded cell.env lines +1 per site, rule-text lines 0"
  why-add: "The requirement already exists in the intake skill and nothing in a new cell satisfies it. Removal considered: changing the intake role's tier in the tier map instead, which has no effect while the scaffold sets a default pin. Changing only the cell file was also considered and rejected: provider defaults override it on house and k8s cells. The notice on `cellctl set` is added because the override that already exists was not discoverable from the tool; refusing the write, as `cellctl desk --set` does, was considered and not chosen, since a pin written ahead of removing a provider-defaults file is legitimate."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not change `resolveRoleModel`, `roleTier`, the coordinator's pin or its Opus refusal, or any existing cell's files.

## Task

1. Re-check the facts. If any scaffold site already writes an intake pin, or the seed already gives intake a tier above `mid`, report NEEDS_CONTEXT.
2. In the embedded seed `tools/desk/cmd/cellctl/providers.json`, set `intake-desk` to tier `strong` under every provider's `desks`. Change no other desk, tier or model. If the shell oracle or any fixture carries a copy of the seed, change it identically.
3. At each of the three scaffold sites, write `DESK_MODEL_intake_desk=opus` directly after the `DESK_MODEL_the_desk=fable` line. In `pinnedBlock` and the scrubbed body, add one comment line above it saying intake's judgment work needs a strong tier and that removing the line returns the role to the default. Reword the `pinnedBlock` comment that counts "the two DESK_MODEL_* lines" so it no longer states a number.
4. Make the same cell-file change in the shell oracle so the parity test compares equal: at every site the grep in the facts prints that scaffolds a cell file, and in the oracle's copy of the reworded comment.
5. In `cellctl set`, when a `KEY=VALUE` call writes a model pin the cell's current resolution will not read, print one notice on stderr after the normal success line. Two cases: the cell resolves through a model policy or provider defaults, or the key belongs to a pin namespace the cell's harness does not read. The notice names the key, says it is not read, and names what decides instead: the policy file, or the shared provider-defaults file together with the per-cell override path, or the namespace the harness reads. The write still happens and the exit code is unchanged. Use `cellModelPolicy` to decide; add no second resolver.
6. In `docs/cellctl.md`'s Pinned models section, `docs/cellctl-model-policy.md` and `help/concepts.txt`, state the new seeded tier and scaffolded line, why they exist, which of the two applies on which cell kind, the three ways an operator moves an existing installation including the per-cell override file, and that changing a provider-defaults file a running cell reads requires restarting that cell's windows.
7. Add `TestSeedIntakeTierStrong` (planned): the embedded seed parses as a valid policy; under every provider the intake desk resolves to that provider's `strong` tier model, and `worker-desk` still resolves to `mid`.
8. Add `TestNewPinsIntakeStrong` (planned): for each cell kind the scaffold supports in a test, a freshly scaffolded `cell.env` contains the line exactly once and the intake role resolves to `opus` from `DESK_MODEL_intake_desk`, while another non-coordinator role still resolves to the default.
9. Add `TestIntakePinSurvivesReset` (planned): the model reset run over a freshly scaffolded cell leaves the intake line unchanged.
10. Add `TestSetShadowedPinNotice` (planned): on a house cell with provider defaults in force, `cellctl set <cell> DESK_MODEL_intake_desk=opus` writes the line, exits 0 and prints a notice naming the key and both files; with a per-cell override giving intake tier `strong`, the role then resolves to the strong tier model. On a cell with no policy and no provider defaults, the same call prints no notice. A second subtest covers the other case in Task 5: on a cell whose harness is `codex`, with no policy and no provider defaults, `cellctl set <cell> DESK_MODEL_intake_desk=opus` writes the line, exits 0 and prints a notice naming the key and the namespace that harness reads, and `cellctl set <cell> CODEX_MODEL_intake_desk=<a model id>` on the same cell prints no notice.
11. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSeedIntakeTierStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestSeedIntakeTierStrong ' "$o")` | exit 0 and the named PASS; under every seeded provider intake resolves to the strong tier and the worker desk stays on mid |
| 2 | check:ci +dereference +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewPinsIntakeStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestNewPinsIntakeStrong ' "$o")` | exit 0 and the named PASS; every scaffolded cell kind resolves intake to the strong pin and leaves other roles on the default |
| 3 | check:ci +neighbour | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakePinSurvivesReset$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestIntakePinSurvivesReset ' "$o")` | exit 0 and the named PASS; the model reset does not undo the scaffolded pin |
| 4 | check:ci | tree | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl)` | exit 0; output contains "ok" |
| 5 | check:ci | tree | `sh tools/cellctl/tests/parity.test.sh` | exit 0; the Go scaffold and the shell oracle still agree |
| 6 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:05` | exit 0; output contains "0 disproved" |
| 7 | check:ci +flow +mutation | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSetShadowedPinNotice$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestSetShadowedPinNotice ' "$o")` | exit 0 and the named PASS; a pin the cell will not read is written with a notice naming what decides, in both cases: under provider defaults, and in a pin namespace the cell's harness does not read. A per-cell override moves the role, and a pin the cell does read gets no notice in either case |

Failure modes mapped: the seed left on mid, so the cell-file pin is overridden on house and k8s cells → row 1; another desk's tier moved by accident → row 1; a scaffold site missed, or the pin written where resolution does not read it → row 2; the reset silently reverting it → row 3; the coordinator pin or refusal disturbed → row 4; the oracle left behind → row 5; an operator's hand-set pin silently ignored, or the notice firing on a cell that does read the pin → row 7. Whether `opus` remains the right alias for the strong tier is not decided here; the alias table in `policy.go` owns it.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSeedIntakeTierStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestSeedIntakeTierStrong ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestNewPinsIntakeStrong$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestNewPinsIntakeStrong ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakePinSurvivesReset$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestIntakePinSurvivesReset ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl)` | fail exit=1 sandbox=container-netns | sha256:23d806e724a3 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh tools/cellctl/tests/parity.test.sh` | fail exit=2 sandbox=container-netns | sha256:de5d53a24fff | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --consumers --brief assay:assay:intake-class-routing:05` | fail exit=2 sandbox=container-netns | sha256:f4f3df9c8d21 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestSetShadowedPinNotice$' ./cmd/cellctl > "$o" && grep -q -- '--- PASS: TestSetShadowedPinNotice ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:7fc8216b67e2dc1d9eac1305a2b70a311ca2d70f94c13ea341c3d73e24f2c5b3
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6829 run=e86134a8a908b1e69d62644448082d08aee7e91b15e51f82938ee0f96495dc81 source=185b7ee7ebdfefbcd5866c8dc2389a2ab82989e9 brief=docs/streams/intake-class-routing/brief-05-new-cells-pin-the-intake-role-to-a-strong-tier.md model=claude-opus-5-5 tier=any
Admission-Bracket: run=e86134a8a908b1e69d62644448082d08aee7e91b15e51f82938ee0f96495dc81 source=185b7ee7ebdfefbcd5866c8dc2389a2ab82989e9 tree=sha256:7fc8216b67e2dc1d9eac1305a2b70a311ca2d70f94c13ea341c3d73e24f2c5b3 witness=sha256:84df910e3a9aa01c679592be09fe34692c2e1e797fcf4f6cad7654a2958a0bee opened=2026-11-18T02:05:07Z closed=2026-11-18T02:07:01Z before=receipt after=check-verifier

**Verifier notes (claude-opus-5-5 non-implementer, 2026-11-18, merged main 185b7ee7ebdf, desk-tools v1.0.87).** Rows 1-7 ran in one closed container-netns admission bracket: verify-runner v1.0.87, --network none, a fresh read-only module cache, tmpfs /tmp of 32 GiB. bracket-digest reports verdict=match. With --credit it reports credit=could-not-check, because the witness file sits outside a work tree. The delivering merge is da01ede73a2b (PR 6310), an ancestor of the source. Rows 1, 2, 3 and 7 pass: every named PASS is present, including the four new intake-pin tests.

- **Row 4: exit 1 under the admitted row environment, from a test outside this delivery.** verifyrun's admitted row environment sets GIT_CONFIG_COUNT=2 (the grep.patternType and grep.extendedRegexp pins). Since PR 6689 (issue 6606), gitcore's scoped remote readers refuse whenever GIT_CONFIG_COUNT is nonzero. Under that refusal, TestDeskdCoordinateUsesCellOriginNotCallerDirectory (tools/desk/cmd/cellctl/helper_custody_test.go:88) panics in deskdRepo (tools/desk/cmd/cellctl/deskd.go:155), and the panic aborts the package. This is the open admitted-environment Git-fixture class medici-finance/assay#2715. Supplemental runs in the same image and mounts, no acceptance credit:
  - The plain verifier env (GIT_CONFIG_COUNT=0) gives ok cellctl 167.764s, rc 0.
  - The admitted env's row variables reproduce the single FAIL plus the panic, rc 1.
  - The admitted env with only that one test excluded by name gives ok cellctl 237.938s, rc 0.
  - On the host, that one test passes with GIT_CONFIG_COUNT unset or 0 and fails with the two grep pins.
- **Row 5: exit 2 is a check-definition defect at this brief's line 100.** The row runs a bash-only script with sh. The script's shebang is /usr/bin/env bash, line 38 is set -uo pipefail and line 404 uses process substitution, and it has been that way since 6f5597961 (2026-09-20). In the container, dash stops with "parity.test.sh: 38: set: Illegal option -o pipefail". On the host, sh stops at line 404 with "syntax error near unexpected token". Supplemental host run of bash tools/cellctl/tests/parity.test.sh: "parity: 200 cells, 0 divergent", rc 0, no acceptance credit. The repair is the reviewed row edit sh to bash, then a fresh dispatch.
- **Row 6: exit 2, COULD-NOT-CHECK, nothing disproved.** Observed: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:intake-class-routing:05 is not in the diff against 185b7ee7…, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery, and the history cannot be searched through the library read path (native discovery/config/shallow environment profile)". The source is statusgen/gitcore_read.go:98-100, which refuses the library read whenever GIT_CONFIG_COUNT is present (reached through statusgen/consumerdelivery.go:302). The admitted environment always sets it, and the declared verifier env sets it to 0. The same refusal reproduces on the host under the declared verifier Git env, rc 2. The remedy the tool names is a Verification-Table marker or --delivery/--base, which is a check-definition matter. No Git configuration was stripped to retry.
- Attempt 1 (01:52:47Z-01:58:18Z) was discarded as superseded. Its 8 GiB tmpfs sat below cellctl's 10 GiB free-space floor, which failed three more row 4 tests. It was re-run as attempt 2 with 32 GiB; the witness above is attempt 2.

RISK-VALUE: DERIVED — intake-desk tier = "strong" @ tools/desk/cmd/cellctl/providers.json:74 (also :137, :200, :263 for the glm, kimi and codex providers) — the intake skill does judgment work only at a strong tier and mechanical work below it (this brief's line 61). The seeded provider defaults override the cell-file pin, so only a strong seed lets a new cell's intake judge from its first boot. Worker-desk and the other desks are unchanged. The value is reversible by editing CELLS_ROOT/providers.json.
RISK-VALUE: DERIVED — DESK_MODEL_intake_desk = opus @ tools/desk/cmd/cellctl/new.go:225 (also :321 and :549) — the alias table maps opus to strong (tools/desk/cmd/cellctl/policy.go:32), so the cell-file pin lands on the same strong tier as the seed. The value is reversible by removing the line.

**VERIFY: BLOCKED** — 4/7 pass (1, 2, 3, 7). No delivered obligation was observed failing. Row 4 is blocked by the admitted-environment Git-fixture class medici-finance/assay#2715 (one test outside this delivery). Row 5 is a check-definition defect (sh runs a bash-only script, brief line 100). Row 6 is COULD-NOT-CHECK on the library read path under the admitted and declared Git env.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
