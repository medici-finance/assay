---
brief: assay:assay:desktools-v2:72
title: Land verifier outcome records from a dispatcher-named path outside the home
why: "Verifiers write their outcome record inside their own home, where it refuses the verifier admission check, because nothing names the path outside the home that the landing already accepts."
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-22 by statusgen newbrief
sources:
  - 'desktools-v2: Land verifier outcome records from a dispatcher-named path outside the home'
  - "driver ruling, 2026-10-22, for ratification at merge (its public record is the driver's merge of the brief PR that adds this brief, #4069, merged 2026-10-22): fix in the tooling; the dispatcher allocates a per-run scratch path outside the home and names it in the verifier prompt; deskevidence lands it through the existing binding; no widening of admission; record format unchanged"
  - docs/proposals/non-ci-architecture/13-migration-and-decisions.md (§The verifier transaction, end to end)
  - freshness-checked 2026-10-22 @ 9a2c6ba2bc85558d77c4227984d93a728a7f7e94 (origin/main)
exec-tier: any
exec-tier-why: "A path allocation, a prompt line and one input guard, with clear precedents (packet path, fragment guard); admission itself is untouched."
domain: clear
outcome: none
consumers:
  - "tools/desk/cmd/deskdispatch/ (prompt.go, packet.go): follow-up desktools-v2/72"
  - "tools/desk/cmd/deskevidence/ (deskevidence.go, outcomerecord.go): follow-up desktools-v2/72"
  - "plugins/assay/skills/verify-desk/SKILL.md and the generated Cursor mirror: follow-up desktools-v2/72"
  - "verifier admission (verifierSourceClosure, CheckEvidenceTarget): out-of-scope (unchanged; nothing new is admitted)"
  - "verify-wake-v1 record format: out-of-scope (the ruling keeps the record bytes unchanged, so readers of verify-outcomes need no migration)"
---

# Brief 72 — Land verifier outcome records from a dispatcher-named path outside the home

## Context

files: tools/desk/cmd/deskdispatch/prompt.go, tools/desk/cmd/deskdispatch/packet.go, tools/desk/cmd/deskdispatch/packet_verifier.go; tools/desk/cmd/deskevidence/deskevidence.go, tools/desk/cmd/deskevidence/outcomerecord.go; tools/desk/internal/deskkit/verifiersource.go, tools/desk/internal/deskkit/verifierattestation.go, tools/desk/internal/deskkit/verifyoutcomes.go (read only); plugins/assay/skills/verify-desk/SKILL.md

facts:
- `deskdispatch --check-verifier` reaches `verifierLocalCheck` (`deskkit/verifierattestation.go:163`). Its source closure (`deskkit/verifiersource.go:339`, `verifierSourceClosure`) refuses any untracked file: `Refused("unattested worktree file: "+rel)` (`:380`-`:381`). The check's own comment says "Other outputs belong outside this home" (`verifierattestation.go:220`-`:225`).
- The landing target for a record is already admitted. `CheckEvidenceTarget` (`verifierattestation.go:650`-`:668`) accepts the brief, its README or `OutcomeRecordsDir/<stream>/<NN>-*`, where `OutcomeRecordsDir = "docs/streams/verify-outcomes"` (`deskkit/verifyoutcomes.go:47`). Admission does not need to change. The problem is only where the verifier writes the file before landing it.
- `deskevidence --outcome-record` (`deskevidence.go:226`) goes to `cmdOutcomeRecordWrite` (`:274`-`:278`). That function rebases a relative path against `--root`, the verifier home (`outcomerecord.go:62`-`:66`). So the relative spelling reads a file inside the home, the same shape that refuses admission. The Evidence-fragment path already has the guard this lacks: with `--root` set, an absolute `--evidence-file` must be a `--brief-path` fragment outside `--root` (`deskevidence.go:344`-`:348`, using `pathWithin` at `:147`).
- The verifier prompt names no output path. `writeVerifierAssignment` (`deskdispatch/prompt.go:504`) prints the Pre-work admission paragraph (`:511`) and the verify instructions with no scratch location. The precedent for one is `packetPathFor` (`deskdispatch/packet.go:170`): beside `--prompt-file` when given, else `<UserCacheDir>/assay/packets/<claim key>.packet.md`, never in the worktree and never in the configuration directory, pruned after a week. `packet_verifier.go:45`-`:47` says the same thing for packets: "Outside the verifier home, always".
- verify-desk tells the verifier to write the record (`SKILL.md:564`-`:572`) and lists the admitted landing form `--outcome-record <absolute file outside the home>` (`:635`). It gives no path, so each verifier invents one, and a path under `docs/streams/verify-outcomes/` inside the home is the obvious wrong guess.

Design-fit: proposal 13's verifier transaction keeps admission, execution and
publication separate, and proposal 11 keeps the runner without publisher credentials
while a separately authorized publisher lands the result. A scratch path outside the home
is part of that boundary: the home stays exactly the admitted source, and the outcome
record crosses to the publisher (`deskevidence`) as an explicit input, the same way an
Evidence fragment does today. The record stays verify-wake-v1. Proposal 29's closed core
is untouched because the record's bytes do not change. This adds no budget or knob. The
path is per run, not per session.

## Ground rules

Normal worker admission, one isolated worktree and one draft PR. Tests use temporary
directories and the existing fakes, with no forge access. Do not change
`verifierSourceClosure`, `CheckEvidenceTarget`, `RecordName` or the record parser. Stop
at implemented.

## Task

1. In `deskdispatch`, add an outcome-scratch allocator modeled on `packetPathFor`. It
   uses `<prompt dir>/outcomes/<claim key>/` (beside `--prompt-file`) when given, else
   `<UserCacheDir>/assay/outcomes/<claim key>/`, so runs that share a prompt directory
   never share an outcome directory. The claim key becomes a path segment on both
   branches and is used verbatim: it is refused, never rewritten. This is stricter than
   `packetPathFor`'s cache-branch rule (`deskdispatch/packet.go:185`-`:188`), which first
   rewrites the key with `sanitizeSegment` (`deskdispatch/dispatch.go:2204`-`:2208`, which
   trims `/` and replaces `/` and `..` with `-`), so `a/b` would pass as `a-b` and collide
   with a real `a-b` key. On both branches a key is refused when `sanitizeSegment` would
   change it in any way, when it contains `/`, `\` or `..`, or when it does not match
   `itemKeyRe`. It creates the directory owner-only and
   refuses (does not fall back) when the resolved path lies inside the verifier home or
   the dispatcher root. Use the same week-based pruning as packets.
2. `writeVerifierAssignment` names that directory in the verifier prompt as an absolute
   path, with the exact landing command:
   `deskevidence <owner/repo> <branch> --outcome-record <dir>/<file>.json --root <home>`.
   It also says never to write the record inside the home. The line is emitted only for
   the verifier class.
3. `deskevidence --outcome-record` with `--root` mirrors the fragment guard. A relative
   path, or an absolute path that `pathWithin(root, …)` places inside the home, is refused
   with a message naming the outside-the-home rule. Without `--root`, behaviour is
   unchanged.
4. verify-desk: replace the invented-path guidance with "write the record to the
   dispatcher-named outcome path from your prompt" and keep the `:635` landing form.
   Regenerate the Cursor mirror.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | exit 0 and named PASS; the verifier prompt contains one absolute outcome path outside the home and the `--outcome-record … --root` command; an allocation that resolves inside the home or root is refused; two claims dispatched with prompt files in one directory get different outcome directories; a claim key containing `/`, `\` or `..` (for example `a/b`) is refused on both the prompt-file and the cache branch, with no directory created, while `a-b` is accepted; non-verifier prompts carry no such line; mutation: rewrite the claim key with `sanitizeSegment` instead of refusing it — the `a/b` case must fail |
| 2 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | exit 0 and named PASS; with `--root`, a relative path and an in-home absolute path (including through a symlink alias) are refused before any forge read; an outside path proceeds to admission; mutation: drop the `pathWithin(root, …)` check — the in-home absolute and symlink-alias cases must fail |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | exit 0; existing outcome-record, packet and admission tests unchanged |
| 4 | check | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | `0` (admission and record format untouched) |
| 5 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift reported |
| 6 | check | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |
| 7 | check | `statusgen --root . --lint` | exit 0; no PROBLEM line names this brief |

Pre-mortem: a prompt path that lands inside the home on some layout (for example a
prompt file kept in the home) would repeat the refusal. Row 1's inside-home case and the
refuse-not-fallback rule catch that. A guard that only checks the literal prefix and
misses a symlinked alias is caught by row 2. Widening admission instead of moving the file
is caught by row 4. Mutation checks: rewriting the claim key instead of refusing it turns
row 1's `a/b` case red, and dropping the `pathWithin` check turns row 2's in-home cases red.

The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:329a7e463e0a | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 | sha256:d7509bd91bad | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4833 run=33ccbc138fca53da67ddc8c20ccc6c79b5b8e44db8fcadf9dd3d359d383d684b source=9e61aee3f8e946198214101938c0420ca37d7f6a brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong

### Postmerge verification, 2026-10-29, source 9e61aee3f8

Verifier: claude-opus-5-5, tier strong, non-implementer, against merged main source 9e61aee3f8e9 (attestation medici-finance/assay#4833), desk-tools / statusgen v1.0.65, darwin arm64 host. Delivery: commit da4c268ac2 through merge 6cdcf0c06a (PR #4073). The admitted-home check (deskdispatch --check-verifier) exited 0 before any row and again before this Evidence was written.

The table above is the host verifyrun witness for rows 4, 6 and 7. Rows 1, 2, 3 and 5 (check:ci) ran once through the admitted container-netns lane: verifyrun --open-bracket on the host (opened 2026-10-29T17:50:40Z), the container run (image ghcr.io/medici-finance/verify-runner:v1.0.65, platform linux/amd64 under emulation, --network none, home and receipt mounted read-only, --rows 1,2,3,5), verifyrun --close-bracket on the host (exit 0) and verifyrun --bracket-digest (verdict=match). One earlier container attempt refused at container admission before any row executed: the mounted tree digested to the empty-input sha256 because the bind did not reach the host path. Nothing was written; the run was repeated with the home bound at a fixed container path. The closed container witness follows verbatim.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:b67191ae193d | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:c28b5b1ddfc2 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:991155681d55 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:45a7cf09742c66449d44c13be973ed4b287736e750b4de6f320474fc5a2c821b
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4833 run=33ccbc138fca53da67ddc8c20ccc6c79b5b8e44db8fcadf9dd3d359d383d684b source=9e61aee3f8e946198214101938c0420ca37d7f6a brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=33ccbc138fca53da67ddc8c20ccc6c79b5b8e44db8fcadf9dd3d359d383d684b source=9e61aee3f8e946198214101938c0420ca37d7f6a tree=sha256:45a7cf09742c66449d44c13be973ed4b287736e750b4de6f320474fc5a2c821b witness=sha256:241053e21c69c34f9d67abda479adb5a210a01f25241257a97f81a86839ca004 opened=2026-10-29T17:50:40Z closed=2026-10-29T18:05:54Z before=receipt after=check-verifier

Row notes:

- Row 1: pass in the container lane, exit 0, the named test passes. Mutation (supplemental, scratch clone at the source, GOPROXY=off, no acceptance credit): replacing the claim-key refusal with a sanitizing rewrite fails the subtest for traversal-shaped claim keys on both branches; restored, it passes.
- Row 2: pass in the container lane, exit 0, the named test passes. Mutation (supplemental, no credit): removing the in-home containment arm of the deskevidence outcome-record guard fails the subtests for an absolute path inside the home and for a symlink alias into the home (exit 0 where 5 is wanted); restored, it passes.
- Row 3: could-not-check. The container lane observed exit 1, but the failures sit outside this brief's diff and come from the lane's environment. The deskevidence package passes. In deskdispatch, the two review-dispatch packet tests fail because no forge can be resolved (the mounted tree has no origin remote host), and the package hits go test's 10m default timeout under amd64 emulation. In deskkit, the three credential-chain tests that drive real git fail because the mounted tree's gitfile points outside the mount, the review caller-flow test fails, and the package hits the same 10m timeout. None of these tests is in the delivery diff. Public CI check-runs at 9e61aee3f8 are green for build-test and its desk shards (supplemental, no credit). This suite cannot be witnessed in this lane until the lane supplies a git directory (the linked-worktree gap tracked in #4327), a forge origin and a longer timeout.
- Row 4: pass, exit 0, output 0.
- Row 5: pass in the container lane, exit 0.
- Row 6: could-not-check. Exit 2 on merged main: COULD-NOT-CHECK, assay:assay:desktools-v2:72 is not in the diff against the source. The row's Expect is pre-merge, on the implementation branch. A supplemental probe at PR head 8d38487a98 with base 9a2c6ba2bc also returned COULD-NOT-CHECK (no brief file for desktools-v2/72; the brief is absent on the implementation branch), so neither spelling could corroborate.
- Row 7: could-not-check. Exit 1, LINT: FAIL 1 problem(s). The single PROBLEM is "desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required", an offline could-not-check on another brief, tracked in #4823. No PROBLEM line names desktools-v2/72, which is the part of the Expect this brief owns. Exit 0 cannot be shown offline.

Accounting: 7 rows; 4 pass (rows 1, 2, 4, 5); 3 could-not-check (rows 3, 6, 7); 0 failed obligations of this brief.

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory.
RISK-VALUE: DERIVED — directory mode 0o700 via os.MkdirAll and os.Chmod @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits; an outcome base directory other users can write to is refused, so Task 1's owner-only directory never sits under a directory someone else controls.
RISK-VALUE: DERIVED — prune age packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154 — the brief asks for the same week-based pruning as packets; the allocator reuses the packet constant, so the two cannot drift.

**VERIFY: BLOCKED** — 4/7 rows pass at source 9e61aee3f8e9; rows 3, 6 and 7 could-not-check (row 3's suites cannot be witnessed in the container lane, #4327; row 6 is pre-merge only; row 7's lint is red offline on desktools-v2/65, #4823); no obligation of this brief observed failing.

## Review

Gate: model. Independent correctness review of the exact head.
