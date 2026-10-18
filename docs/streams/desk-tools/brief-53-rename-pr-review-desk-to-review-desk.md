---
brief: assay:assay:desk-tools:53
title: "Rename pr-review-desk to review-desk without breaking live windows or adopters"
why: "The review role is the only desk role still named after its mechanism. Renaming it to review-desk must not break live windows, held stop flags, claims, labels or adopter installs. So the old name stays an accepted alias forever, as with the two earlier role renames, and a lint stops new uses of it."
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "An adopter-facing public role and skill name changes. A held stop flag that a rename orphans fails silently, so a human must confirm the alias coverage, the label-value correction and the deferred container image name."
decision-trigger: start
issues: []
schema: brief-v2
authored: 2026-10-18 by statusgen newbrief (driver-requested rename)
sources:
  - "docs/streams/desk-tools/README.md"
  - "tools/desk/internal/deskkit/loopnames.go (canonicalLoopNames rename precedent)"
  - "docs/streams/harness-portability/brief-02-drift-debt-authority-flip.md and brief-06-codex-packaging.md (batch-fanout to worker-desk rename precedent)"
  - freshness-checked 2026-10-18 @ 6dfb616ef (origin/main)
exec-tier: strong
exec-tier-why: "The change crosses about 45 Go source files, two binaries' on-disk state, three spec closed sets, a skill bundle and its mirrors. If any reader misses the alias, a stop flag, claim or inbox silently stops matching, and that failure shows up as a quiet pass, not an error."
domain: complicated
outcome: none
version: 1
id: 9a74aa56-1059-4e4e-bd53-d59b62b0f0f9
consumers:
  - "tools/desk/internal/deskkit/loopnames.go: follow-up desk-tools/53 (this brief: review-desk canonical, pr-review-desk retired alias)"
  - "tools/desk/internal/deskkit/roletoken.go: follow-up desk-tools/53 (this brief: TokenRoleForLoop folds the alias class)"
  - "tools/desk/internal/deskkit/addressto.go: follow-up desk-tools/53 (this brief: reader-side to: fold)"
  - "tools/desk/internal/deskkit/width.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/internal/deskkit/reviewdeskrename_test.go: follow-up desk-tools/53 (planned test contract)"
  - "tools/desk/cmd/issueboard/board.go: follow-up desk-tools/53 (this brief: --to inbox fold)"
  - "tools/desk/cmd/fanoutloop/adapter.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/cellctl/cell.go: follow-up desk-tools/53 (this brief: knownRoles + argument normalizer)"
  - "tools/desk/cmd/cellctl/policy.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/cellctl/up.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/cellctl/desk.go: follow-up desk-tools/53 (this brief: old worktree, model pin and session reuse)"
  - "tools/desk/cmd/cellctl/testdata/cli-nonhelp-golden.json: follow-up desk-tools/53 (this brief)"
  - "tools/cellctl/testdata/cellctl-shell-oracle.sh: follow-up desk-tools/53 (this brief: bash oracle parity)"
  - "tools/cellctl/tests/review-desk-alias.test.sh: follow-up desk-tools/53 (planned oracle test contract)"
  - "tools/desk/cmd/commsloop/roleprofile.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/internal/comms/laneacl.go: follow-up desk-tools/53 (this brief: lane ACL folds the alias)"
  - "tools/desk/internal/comms/laneacl.yaml: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/deskautolane/lane.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/deskflip/flip.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/deskwt/roleinit.go: follow-up desk-tools/53 (this brief: old-prefix worktree/branch recognition)"
  - "tools/desk/cmd/deskclaim-ref/claim.go: follow-up desk-tools/53 (this brief: renamed-session holder equivalence)"
  - "tools/desk/cmd/reviewloop/main.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/cmd/scanloop/exitrecord.go: follow-up desk-tools/53 (this brief)"
  - "statusgen/intakeexits.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/internal/cellcontainer/config.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/internal/tick/grammar.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/internal/weight/weight.go: follow-up desk-tools/53 (this brief: skill path)"
  - "tools/desk/cmd/deskboard/throughput.go: follow-up desk-tools/53 (this brief)"
  - "tools/desk/README.md: follow-up desk-tools/53 (this brief: loop-name registry bullet)"
  - "plugins/assay/skills/review-desk/SKILL.md: follow-up desk-tools/53 (this brief: renamed skill home)"
  - "plugins/assay/skills/review-desk/component.yaml: follow-up desk-tools/53 (this brief)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/53 (this brief: deprecated redirect stub)"
  - "components/KEYS.md: follow-up desk-tools/53 (this brief)"
  - "plugins/assay/.claude-plugin/plugin.json: follow-up desk-tools/53 (this brief)"
  - "plugins/assay/.codex-plugin/plugin.json: follow-up desk-tools/53 (this brief)"
  - ".cursor/assay-managed.json: follow-up desk-tools/53 (this brief: regenerated mirror hashes)"
  - "spec/intake-exit-v1.md: follow-up desk-tools/53 (this brief)"
  - "spec/registers-v1.md: follow-up desk-tools/53 (this brief)"
  - "spec/workflow-patterns/implementation-v1.yaml: follow-up desk-tools/53 (this brief)"
  - "containers/pr-review-desk/Dockerfile: follow-up desk-tools/53 (this brief: ASSAY_DESK and default command only; path and image name unchanged)"
  - "docs/skill-naming.md: follow-up desk-tools/53 (this brief)"
  - "tools/skillslint/retiredname.go: follow-up desk-tools/53 (planned retired-name gate)"
  - "tools/skillslint/retiredname_test.go: follow-up desk-tools/53 (planned test contract)"
  - "changelog/review-desk-rename.md: follow-up desk-tools/53 (this brief)"
  - "tools/release/images_contract.py: out-of-scope (the published image name is part of the release-images-v1 nine-image contract; renaming it is the deferred decision in Human decision)"
  - "CHANGELOG.md: out-of-scope (historical release record; never rewritten)"
  - "docs/quality/QUALITY.md: out-of-scope (generated from historical metrics; regenerated, never hand-edited)"
---

# Brief 53 — Rename pr-review-desk to review-desk without breaking live windows or adopters

## Context

files: tools/desk/internal/deskkit/loopnames.go, tools/desk/internal/deskkit/roletoken.go, tools/desk/internal/deskkit/addressto.go, tools/desk/cmd/issueboard/board.go, tools/desk/cmd/cellctl/cell.go, tools/desk/cmd/cellctl/desk.go, tools/desk/cmd/deskwt/roleinit.go, tools/desk/cmd/deskclaim-ref/claim.go, tools/desk/internal/comms/laneacl.go, statusgen/intakeexits.go, plugins/assay/skills/pr-review-desk/SKILL.md, tools/skillslint/
facts:
- Driver request (2026-10-18): rename the desk role `pr-review-desk` to `review-desk` everywhere. The other phase roles are `worker-desk`, `verify-desk` and `intake-desk`. `pr-review-desk` is the one left that names a mechanism (pull requests) instead of the phase it owns.
- Measured at 6dfb616ef: `git grep -n pr-review-desk` gives 4507 lines in 340 files. 409 lines in 113 files are Go, and 60 lines in 45 files are non-test Go. 3265 lines are in two generated quality files (`docs/quality/`). Historical records (stream briefs, findings, verify-outcomes, `CHANGELOG.md`) hold about 490 more. The surface this brief actually edits is about 1240 lines.
- Precedent to follow exactly: `tools/desk/internal/deskkit/loopnames.go:77` `canonicalLoopNames` already carries `"worker-desk": {"batch-fanout"}` and `"intake-desk": {"issue-loop"}`. Retired names are kept as aliases forever, and the successor is registered BEFORE anything emits it. `LoopFlagNames` returns the whole equivalence class, so a `STOP.<retired>` flag still halts the renamed loop, and an unknown `DESK_LOOP` is exit 6, never "no stop held". Today `pr-review-desk` is canonical with no aliases (`tools/desk/internal/deskkit/loopnames.go:81`).
- Non-test Go sites that hard-code the name: `tools/desk/cmd/cellctl/cell.go:31` (`knownRoles`), `tools/desk/cmd/cellctl/policy.go:35`, `tools/desk/cmd/cellctl/up.go:529` (tmux window name), `tools/desk/cmd/commsloop/roleprofile.go:41,64,88`, `tools/desk/cmd/deskautolane/lane.go:28`, `tools/desk/cmd/deskflip/flip.go:102`, `tools/desk/cmd/deskwt/roleinit.go:93`, `tools/desk/cmd/reviewloop/main.go:43`, `tools/desk/cmd/scanloop/exitrecord.go:72` (+`:198`, `tools/desk/cmd/scanloop/landverb.go:33` clip widths), `statusgen/intakeexits.go:61` (+`:91`), `tools/desk/internal/cellcontainer/config.go:44`, `tools/desk/internal/comms/laneacl.go:55`, `tools/desk/internal/deskkit/roletoken.go:44`, `tools/desk/internal/deskkit/width.go:226`, `tools/desk/internal/tick/grammar.go:38`, `tools/desk/internal/weight/weight.go` (skill path), `tools/desk/cmd/deskboard/throughput.go:158`. All other non-test Go hits are help text and comments.
- `deskkit.TokenRoleForLoop` (`tools/desk/internal/deskkit/roletoken.go:61`) is an exact map lookup with NO alias fold. `SessionTokenRole` folds through `LoopFlagNames` first, but direct callers do not (`tools/desk/cmd/deskautolane/forge.go`, `tools/desk/cmd/deskflip/flip.go`, `tools/desk/cmd/fanoutloop/adapter.go`, `tools/desk/internal/commsqueue/quarantine.go`). Renaming the map key without a fold would strand any window still exporting the old name.
- Labels: `to:` and `raised-by:` carry roster ROLE names (`reviewer`), never skill or loop names. `deskkit.boundRole` (`tools/desk/internal/deskkit/raisedby.go:152`) is shared by both writers, and `tools/desk/internal/deskkit/addressto_test.go:29` pins that `AddressedToLabel("pr-review-desk")` is REFUSED. The forge's measured label set (2026-10-18) has `to:reviewer` and no `to:pr-review-desk`. The reader `tools/desk/cmd/issueboard/board.go:764` compares `AddressedToOf` against `--to` with `EqualFold`, so a hand-applied `to:pr-review-desk` or `to:review-desk` label currently reaches no inbox. This brief corrects the request's premise: the label VALUES do not change. The alias is added on the reader side, which also closes that gap.
- The skill's own shell stop check bypasses the alias class: `plugins/assay/skills/pr-review-desk/SKILL.md:189` tests only `STOP.$DESK_LOOP`. After the rename, a human holding `STOP.pr-review-desk` against a window that exports `DESK_LOOP=review-desk` would be ignored by that layer. The compiled `deskkit` guard would still catch it, but only at the next outward verb.
- cellctl launches `/assay:<role>` and exports `DESK_LOOP=<role>`. Role-keyed on-disk cell state must stay readable: `worktrees/<role>`, `memory/<role>/MEMORY.md`, `DESK_MODEL_<role_underscored>` in `cell.env`, role-context keys, `CELL_ROLES`, and session names (`<cell>-pr-review`, or `<cell>-pr-review-desk-<stamp>` on house or scrubbed kinds, `tools/desk/cmd/cellctl/desk.go:158`). `knownRoles` is ITERATED by cadence, comms init and model reset, so adding the alias to that list would launch the role twice. The alias belongs in an argument normalizer instead.
- Claims: `deskclaim-ref` stamps the owner from `DESK_SESSION` (`tools/desk/cmd/deskclaim-ref/args.go:124`) and refuses `progress` from any other owner (`tools/desk/cmd/deskclaim-ref/claim.go:248`). On a stable-named cell, a relaunched review window presents `<cell>-review` and would be refused on its own in-flight `<cell>-pr-review` claims until TTL.
- The existing retired-name lint `tools/desk/internal/deskkit/retiredskillnames_test.go` (`TestSkillBodiesCarryNoRetiredDeskName`) reads `.claude/skills`, which is absent in this tree, so it SKIPs (verified by `go test`, `--- SKIP`). The new gate must live where it runs and fails. `tools/skillslint` is a gating CI job (`.github/workflows/ci.yml:179`, exit 0/1/2).
- Closed role sets in published specs: `spec/intake-exit-v1.md:97`, `spec/registers-v1.md:271` (`triaged-by`) and `spec/workflow-patterns/implementation-v1.yaml:5`. Records already written with `pr-review-desk` must keep validating.
- Container: `containers/pr-review-desk/Dockerfile` sets `ASSAY_DESK=pr-review-desk`. Its published image name belongs to the release-images-v1 nine-image contract (`tools/release/images_contract.py:5`, `docs/release-images.md:59`).
- Maintainers' house layer: the house's roster, its cellctl house cell, its project instructions and its labels on the house tracker are a SEPARATE brief on the maintainers' house repository. They are not part of this PR. The alias in this brief is what keeps the house working until that follow-up lands.

layering: Extend the existing alias owner (`deskkit` loop-name registry) and route every reader through it. Do not add a second alias table. The only literal spellings of the retired name in source are the registry entry, closed-set value lists that must accept old records, the stub skill, and normalizers that read old on-disk state.

design-fit:
  owner: "tools/desk/internal/deskkit loop-name registry (canonicalLoopNames)"
  contract: none. This adds one equivalence class to the existing registry contract and does not create a new semantic owner.
  retires: ["pr-review-desk as a canonical emitted name (kept forever as an accepted alias)", "the skipped .claude/skills retired-name check as the only guard against regressions"]
  weight: "No new binary or flag. Adds one registry alias, one reader-side label fold, one session-holder equivalence, one skillslint check with an allowlist, and one stub skill. CLI help, refusal text and golden output change wording only. The implementer records exact deltas."
  why-add: "A rename that orphans a held stop flag, an in-flight claim or an inbox fails silently. Registering the alias first and routing readers through the single registry is the same move that made both earlier role renames non-breaking."

single-point-of-failure: The compiled deskkit alias class (a held STOP.pr-review-desk halts a DESK_LOOP=review-desk window). Behind it, two independent layers: the review-desk skill's own shell stop check tests both spellings at every iteration boundary without calling deskkit, and the DISABLED/global stop flags halt every loop whatever its name.

## Human decision

The review role is being renamed from "pr-review-desk" to "review-desk". The old name will be accepted forever as an alias: in stop flags, the loop variable, cell launcher arguments, the comms lane list, closed role sets in specs, claims and worktrees. For at least one release, a short deprecated skill under the old name will redirect to the new one. Decide three points.

First, the canonical rename itself, with the permanent alias.

Second, a correction to the request. Issue labels never carried the skill name. The inbox label for this role is "to:reviewer", and "raised-by:reviewer" is the provenance stamp. Those values stay. Readers will also treat "to:review-desk" and "to:pr-review-desk" as addressed to the reviewer, which fixes the current state where such a label reaches no inbox. Writers still refuse to emit loop names as labels.

Third, deferring the published container image name. The image keeps its current registry name in this change, because that name belongs to a fixed nine-image release contract. Only the role it starts changes. Renaming the image is a separate release decision.

Options:
1. Approve all three as described.
2. Approve the rename and alias, but require the container image to be renamed in this same change, adding a tenth image as the new name and keeping the old one as an alias for at least one release.
3. Hold. Keep "pr-review-desk" as the canonical name.

Recommendation: option 1. It changes nothing that live windows or adopters depend on, and it leaves the image decision to whoever owns the release contract.

Default if no answer: hold.

## Task

1. **Register before rename (deskkit).** In `canonicalLoopNames`, make `review-desk` canonical with `{"pr-review-desk"}` as its retired alias, and remove the `pr-review-desk` canonical key, following the `worker-desk` / `intake-desk` comment pattern. Re-key `loopTokenRoles` and `tools/desk/internal/deskkit/width.go` to `review-desk`, and make `TokenRoleForLoop` resolve through `CanonicalLoopName` first, so both spellings return `reviewer` and an unknown name stays `ok=false`. Update the `tools/desk/README.md` "Loop-name registry" bullet that `tools/desk/internal/deskkit/loopnames_test.go` diffs against.
2. **Every hard-coded site (Go).** Move each non-test site in Context to `review-desk`. Readers that compare a presented name (`deskflip`, `deskautolane`, `reviewloop`, `commsloop`, `tick`, `deskboard`) compare canonical forms, so the old spelling still matches. `tools/desk/cmd/cellctl/up.go`'s special case goes away, because `TrimSuffix("-desk")` already yields `review`. Clip widths that use `len("pr-review-desk")` keep measuring the longest accepted member, which is still the retired name.
3. **Closed role sets accept both names.** `statusgen/intakeexits.go`, `tools/desk/cmd/scanloop/exitrecord.go`, `spec/intake-exit-v1.md`, `spec/registers-v1.md` (`triaged-by`) and `spec/workflow-patterns/implementation-v1.yaml` list `review-desk`, plus `pr-review-desk` marked as a retired alias that stays valid for records already written. New records emit `review-desk`.
4. **Labels.** Writers do not change: `to:reviewer` and `raised-by:reviewer`, and `AddressedToLabel("pr-review-desk")` stays refused. Add a reader-side fold. `AddressedToOf`/`stampOf` (or a wrapper used by `issueboard --to` and `fanoutloop`) maps a `to:` value that names a loop through `CanonicalLoopName` and then `TokenRoleForLoop`. So `to:review-desk` and `to:pr-review-desk` mean `reviewer`, and `to:reviewer` plus `to:pr-review-desk` on one issue is ONE addressee, not indeterminate. Two genuinely different roles stay indeterminate.
5. **Comms lane ACL.** `laneacl.yaml` and its compiled mirror list `review-desk`. `Allows` canonicalizes `fromRole`/`toRole` through the registry before membership and self-message checks. So `pr-review-desk` to `worker-desk` is allowed, and `review-desk` to `pr-review-desk` is a self-message and refused. Do NOT list both spellings, because that would let a role message itself.
6. **cellctl (Go canonical, bash oracle in parity).** `knownRoles`/`policyKnownRoles` list `review-desk` only. Add one argument normalizer, applied where a role argument or `CELL_ROLES` entry is parsed (`desk`, the cobra role args, `comms`, `cursor`). It maps the alias to `review-desk` and de-duplicates, so `cellctl desk <cell> pr-review-desk` launches exactly one `review-desk` window. That window gets `/assay:review-desk` and `DESK_LOOP=review-desk`, and it reuses the existing `worktrees/pr-review-desk`, `memory/pr-review-desk/` and `DESK_MODEL_pr_review_desk` pin when no new-name state exists. New-name state wins when both exist. Mirror the normalizer in `tools/cellctl` and its shell oracle, and update `cli-nonhelp-golden.json`, providers and example JSON.
7. **Worktrees, sessions, claims.** `deskwt role-init` for `review-desk` reuses an existing `tracker-pr-review-desk-<session>` worktree and `pr-review-desk/<session>` branch, and `prune`/reap recognizes both prefixes as the review role. New ones use the `review-desk` prefix. `deskclaim-ref` treats a holder and owner as the same session when they differ only by the renamed role segment (`<cell>-pr-review` and `<cell>-review`, or `<cell>-pr-review-desk-<stamp>` and `<cell>-review-desk-<stamp>` with the same stamp). Any other mismatch, including another cell or another role, is still refused. Roster beacons and width pools already canonicalize (`tools/desk/cmd/deskroster/width.go`).
8. **Skill rename plus stub.** `git mv plugins/assay/skills/pr-review-desk plugins/assay/skills/review-desk`, and set `name: review-desk` and component `assay/review-desk` providing `assay.desk.role.review-desk`. The iteration-boundary shell stop check must test `$DESK_LOOP`, `review-desk` and `pr-review-desk`, without deskkit. Add a thin `plugins/assay/skills/pr-review-desk/` stub. Its `name: pr-review-desk` and its description start "Deprecated alias of review-desk". Keep the description narrow so it does not win routing. The body only says the role was renamed and tells the reader to run `/assay:review-desk`, plus the same both-names stop check. Its component still provides the old key `assay.desk.role.pr-review-desk`, so installs that require it keep resolving. Keep the stub for at least one release, and record the earliest removal release in the changelog fragment. Regenerate the mirrors (`.cursor/skills/`, `.cursor/assay-managed.json`, `.claude/guardrails/GUARDRAILS.md` via `skillslint --sync`, plugin manifests, weight testdata), and keep deskinstall upgrades from deleting a user's old-name install without putting the new one in place.
9. **Docs sweep.** Rename every live reference in `docs/`, `plugins/assay/references/`, `plugins/assay/scripts/`, `containers/` (README, entrypoint comment, Dockerfile `ASSAY_DESK` and default command), `components/KEYS.md` and `docs/skill-naming.md`, and record the rename in its table the way the earlier two were recorded. Do NOT rewrite historical records: `docs/streams/**` briefs, findings and verify-outcomes, `CHANGELOG.md`, release notes, `plugins/assay/PARITY.md`, `SOURCES.yaml` provenance notes and generated `docs/quality/`.
10. **Retired-name gate (tools/skillslint).** Add a check that walks the tracked tree and fails (exit 1, naming `file:line`) on the tokens `pr-review-desk` or `pr_review_desk` outside one declared allowlist. Every allowlist entry is a path with a one-line reason: the registry, the closed-set value lists, the stub skill and its mirror, normalizers, the container path pending the image decision, alias-asserting tests, and the historical-record paths above. An allowlist entry whose file no longer contains the token also fails (stale entry). The check must never SKIP. A missing root or unreadable tree is exit 2.
11. **Changelog fragment** `changelog/review-desk-rename.md` (planned) (`### Changed`). It names the alias guarantee, the label correction, the stub's earliest removal release and the deferred image name.
12. At implementation delivery, convert all 44 consumer entries that name this brief from `follow-up desk-tools/53` to `fixed-here`, keeping the exact sites and notes. The three `out-of-scope` entries stay. Every owed path must be in the delivering diff. Any exclusion or reroute needs a reviewed amendment to this brief.

## Interface contract

- `deskkit.CanonicalLoopName("pr-review-desk") == ("review-desk", true)`. `LoopFlagNames` of either spelling returns `{review-desk, pr-review-desk}`. `TokenRoleForLoop` of either spelling returns `("reviewer", true)`.
- Label values do not change (`to:reviewer`, `raised-by:reviewer`). Readers fold the loop spellings of `to:` onto `reviewer`.
- `cellctl desk <cell> pr-review-desk` is equivalent to `cellctl desk <cell> review-desk`.
- Specs: `review-desk` is the emitted value, and `pr-review-desk` is accepted forever.
- Skill: `/assay:review-desk` is canonical, and `/assay:pr-review-desk` redirects for at least one release.

## Ground rules

- Isolated branch and draft PR. No merge, main push, workflow trigger, forge label edit or live infrastructure command in this brief.
- Never remove or narrow a stop-flag match, refusal or fail-closed path to make a test pass. A red caused by the alias is a design defect to fix, never something to weaken.
- Do not rewrite historical records or generated quality files, and do not touch the maintainers' house layer. That is the separate house-repository brief.
- The public tree carries no logins, bot ids, roster keys or private repository aliases.
- Stop at `implemented`. Independent verification owns the Evidence.

## Verify

New test names below are planned contracts. A named PASS assertion must fail if the selector matches no test. Run commands from the repository root. This brief adds a gate (row 8), so the gate's mutation control is row 9. Grep and presence rows gate presence only. The human review gate owns whether the alias coverage is adequate.

Consumer-diff provenance: obtain the full immutable `DELIVERY_HEAD` and `DELIVERY_BASE` from this brief's actual delivering PR, as described in `docs/brief-rules.md`. An absent binding or an empty diff is could-not-check, never acceptance.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run "^TestReviewDeskRenameAliasHolds$" -count=1 -v > "${TMPDIR:-/tmp}/b53-alias.log" 2>&1 && grep -F -e "--- PASS: TestReviewDeskRenameAliasHolds " "${TMPDIR:-/tmp}/b53-alias.log"` | exit 0, named PASS. Cases: with a temp HOME holding only `STOP.pr-review-desk`, the kill-switch guard for `DESK_LOOP=review-desk` reports halted, and the reverse (`STOP.review-desk` against `DESK_LOOP=pr-review-desk`) also halts. Both spellings canonicalize to `review-desk` and resolve token role `reviewer`. With no flag, it is not halted. An unknown name stays exit 6 | check:ci +flow +dereference |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run "^TestReviewDeskAliasRemovedTurnsRed$" -count=1 -v > "${TMPDIR:-/tmp}/b53-alias-neg.log" 2>&1 && grep -F -e "--- PASS: TestReviewDeskAliasRemovedTurnsRed " "${TMPDIR:-/tmp}/b53-alias-neg.log"` | exit 0, named PASS. Negative control: the row-1 assertions are run against a registry copy with the `pr-review-desk` alias deleted, and the held `STOP.pr-review-desk` must then fail to halt `DESK_LOOP=review-desk`. That proves row 1 depends on the alias, not on a coincidence | check:ci +mutation |
| 3 | `cd tools/skillslint && go test . -run "^TestReviewDeskSkillStopCheckHonoursRetiredName$" -count=1 -v > "${TMPDIR:-/tmp}/b53-shellstop.log" 2>&1 && grep -F -e "--- PASS: TestReviewDeskSkillStopCheckHonoursRetiredName " "${TMPDIR:-/tmp}/b53-shellstop.log"` | exit 0, named PASS. The test extracts the stop-check line from BOTH `plugins/assay/skills/review-desk/SKILL.md` (planned) and the stub, and runs it under `sh` with a temp HOME and `DESK_LOOP=review-desk`. With `STOP.pr-review-desk` present it exits 0 and prints the active-stop line. With no flag it falls through. This is the independent layer that does not use deskkit | check:ci +flow +dereference |
| 4 | `cd tools/desk && go test ./cmd/cellctl/ -run "^TestDeskRoleAliasResolvesToReviewDesk$" -count=1 -v > "${TMPDIR:-/tmp}/b53-cellctl.log" 2>&1 && grep -F -e "--- PASS: TestDeskRoleAliasResolvesToReviewDesk " "${TMPDIR:-/tmp}/b53-cellctl.log"` | exit 0, named PASS. Against a fixture cell, `DRY_RUN=1 cellctl desk <cell> pr-review-desk` exits 0 and prints role `review-desk`, `/assay:review-desk` and `DESK_LOOP=review-desk`, reusing the existing `worktrees/pr-review-desk` and `DESK_MODEL_pr_review_desk` pin. `CELL_ROLES` holding both spellings plans ONE review window. An unknown role is still refused | check:ci +flow |
| 5 | `bash tools/cellctl/tests/review-desk-alias.test.sh` | exit 0. The bash oracle, run against a fixture cell with stubbed `claude` and `deskwt` (the `house-cell.test.sh` pattern), treats `cellctl desk <cell> pr-review-desk` exactly like the Go port: one `review-desk` window, `DESK_LOOP=review-desk`, old worktree reused. An unknown role is still refused | check:ci +neighbour +flow |
| 6 | `cd tools/desk && go test ./cmd/issueboard/ -run "^TestToReviewerReachesReviewDeskSweep$" -count=1 -v > "${TMPDIR:-/tmp}/b53-inbox.log" 2>&1 && grep -F -e "--- PASS: TestToReviewerReachesReviewDeskSweep " "${TMPDIR:-/tmp}/b53-inbox.log"` | exit 0, named PASS. Fixture issues labelled `to:reviewer`, `to:pr-review-desk`, `to:review-desk`, and `to:reviewer` + `to:pr-review-desk` all appear in the reviewer inbox swept by a `DESK_LOOP=review-desk` session and by a `DESK_LOOP=pr-review-desk` session. `to:worker` is absent. `to:reviewer` + `to:worker` stays indeterminate. `AddressedToLabel("pr-review-desk")` is still refused | check:ci +flow +dereference |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run "^TestRoleInitReusesRetiredReviewWorktree$" -count=1 -v > "${TMPDIR:-/tmp}/b53-wt.log" 2>&1 && grep -F -e "--- PASS: TestRoleInitReusesRetiredReviewWorktree " "${TMPDIR:-/tmp}/b53-wt.log" && go test ./cmd/deskclaim-ref/ -run "^TestClaimHeldByRetiredReviewSessionAdvances$" -count=1 -v > "${TMPDIR:-/tmp}/b53-claim.log" 2>&1 && grep -F -e "--- PASS: TestClaimHeldByRetiredReviewSessionAdvances " "${TMPDIR:-/tmp}/b53-claim.log"` | exit 0 and both PASS lines. `role-init review-desk` reuses an existing `tracker-pr-review-desk-<session>` worktree and branch, and prune recognizes it as the review role. A claim held by `<cell>-pr-review` (and by `<cell>-pr-review-desk-<stamp>`) is advanced by `<cell>-review` (and by `<cell>-review-desk-<same stamp>`). A claim held by another cell's review session, by `<cell>-worker`, or under a different stamp is still refused (exit 5) | check:ci +flow |
| 8 | `cd tools/skillslint && go run . --root ../.. > "${TMPDIR:-/tmp}/b53-skillslint.log" 2>&1` | exit 0 on the delivered tree. The retired-name gate runs (its summary line is present in the log) and reports zero bare `pr-review-desk` / `pr_review_desk` outside the allowlist | check:ci |
| 9 | `cd tools/skillslint && go test . -run "^TestRetiredReviewNameGate$" -count=1 -v > "${TMPDIR:-/tmp}/b53-gate.log" 2>&1 && grep -F -e "--- PASS: TestRetiredReviewNameGate " "${TMPDIR:-/tmp}/b53-gate.log" && ! grep -F -e "--- SKIP" "${TMPDIR:-/tmp}/b53-gate.log"` | exit 0, named PASS, no SKIP. Positive control: in a temp tree, a planted bare `pr-review-desk` in a non-allowlisted doc makes the gate exit 1 and name that `file:line`. The same token in the stub, an allowlisted registry file or a historical-record path passes. A stale allowlist entry exits 1. A missing root exits 2 | check:ci +mutation +dereference |
| 10 | `cd tools/desk && go test ./internal/comms/ -run "^TestLaneACLFoldsRetiredReviewName$" -count=1 -v > "${TMPDIR:-/tmp}/b53-lane.log" 2>&1 && grep -F -e "--- PASS: TestLaneACLFoldsRetiredReviewName " "${TMPDIR:-/tmp}/b53-lane.log" && go test ./internal/comms/ -run "^TestACLCompiledMatchesSourceDiff$" -count=1 -v > "${TMPDIR:-/tmp}/b53-acl.log" 2>&1 && grep -F -e "--- PASS: TestACLCompiledMatchesSourceDiff " "${TMPDIR:-/tmp}/b53-acl.log" && go test ./cmd/scanloop/ -run "^TestIntakeExitAcceptsBothReviewNames$" -count=1 -v > "${TMPDIR:-/tmp}/b53-scan.log" 2>&1 && grep -F -e "--- PASS: TestIntakeExitAcceptsBothReviewNames " "${TMPDIR:-/tmp}/b53-scan.log" && cd ../../statusgen && go test . -run "^TestIntakeExitAcceptsBothReviewNames$" -count=1 -v > "${TMPDIR:-/tmp}/b53-sg.log" 2>&1 && grep -F -e "--- PASS: TestIntakeExitAcceptsBothReviewNames " "${TMPDIR:-/tmp}/b53-sg.log"` | exit 0 and every PASS line. Lane: `pr-review-desk` to `worker-desk` is allowed, and `review-desk` to `pr-review-desk` is a refused self-message. The compiled ACL still matches `laneacl.yaml`. Intake-exit records whose triager is `pr-review-desk` (already written) and `review-desk` (new) both validate in scanloop and statusgen, and an unlisted name such as `reviewer-desk` is refused | check:ci +neighbour +flow |
| 11 | `test -f plugins/assay/skills/review-desk/SKILL.md && grep -q -x -e "name: review-desk" plugins/assay/skills/review-desk/SKILL.md && grep -q -x -e "name: pr-review-desk" plugins/assay/skills/pr-review-desk/SKILL.md && grep -q -F -e "/assay:review-desk" plugins/assay/skills/pr-review-desk/SKILL.md && test "$(wc -l < plugins/assay/skills/pr-review-desk/SKILL.md)" -lt 60` | exit 0. The canonical skill home exists under the new name, and the stub exists under the old name, redirects to `/assay:review-desk` and stays thin. Presence only: the review gate judges the stub's wording and routing narrowness | check +dereference |
| 12 | `cd tools/desk && go test ./internal/deskkit/ ./internal/comms/ ./internal/cellcontainer/ ./internal/tick/ ./internal/weight/ ./cmd/cellctl/ ./cmd/deskwt/ ./cmd/deskclaim-ref/ ./cmd/issueboard/ ./cmd/fanoutloop/ ./cmd/deskflip/ ./cmd/deskautolane/ ./cmd/scanloop/ ./cmd/commsloop/ ./cmd/reviewloop/ ./cmd/deskboard/ -count=1 -timeout 900s && cd ../../statusgen && go test ./... -count=1` | exit 0. Neighbour suites stay green, including the existing `TestIssueLoopRenameAliasHolds` and `TestHeldFlagSurvivesTheBatchFanoutRename` | check:ci +neighbour |
| 13 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-53-rename-pr-review-desk-to-review-desk.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/53 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 44 corroborated, 0 disproved, 3 unchecked' "$o"` | exit 0. Exact delivering head/base, no deferred route left, and all 44 owed sites corroborated by the implementation diff. The 3 unchecked entries are the out-of-scope judgements (image contract, CHANGELOG, generated quality file). The reviewer confirms that none of them has a live reader the rename breaks | check +flow +dereference |

## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no implementation test executed. -->

## Failure map

- A held `STOP.pr-review-desk` stops matching after the rename. Rows 1 and 3 each catch it independently, and row 2 proves row 1 is not vacuous.
- `cellctl` double-launches the review role, or loses its worktree or model pin. Row 4 (one window, old state reused).
- An inbox loses `to:` items, or a mixed old/new label pair becomes indeterminate. Row 6.
- A relaunched window is refused on its own in-flight claims, or the fold lets another session steal one. Row 7, positive and negative cases.
- Old intake-exit or register records stop validating. Row 10.
- New bare uses creep back in, or the gate silently skips like the earlier `.claude/skills` check. Rows 8 and 9 (no-SKIP assertion).
- Known residual, not fixed here: the `worker-desk`, `verify-desk` and `intake-desk` skill bodies' shell stop checks also test only `STOP.$DESK_LOOP`, so they ignore their own retired names at that layer. The compiled guard still covers them.

## Review

Gate: human. Answer the core questions from the single-point-of-failure line. (1) Is the compiled alias class an acceptable primary control, given the skill-layer shell check and the global stop behind it? (2) Does a row prove a lower layer catches the fault with the upper bypassed? Row 3 exercises the shell layer without deskkit. Also confirm the label-value correction and the deferred image name against the Human decision. A passing presence row is not adequacy.
