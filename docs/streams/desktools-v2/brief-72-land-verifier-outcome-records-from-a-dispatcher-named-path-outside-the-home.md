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
| 4 | check | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pre-merge, on the implementation branch: output is `0` (admission and record format untouched). On merged main the diff against `origin/main` is empty by construction, so a post-merge run cannot discriminate |
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:9b6dbfac0366 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 | sha256:c4e1b21a9fde | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5064 run=35c59a17128fc1a6d55030735317a94c096a2090bd4eb5125067f903ad9e10d0 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=any

### Postmerge verification, 2026-10-30, source 011f14965e

Verifier: claude-opus-5-5, tier any, non-implementer, against merged main source 011f14965ef2 (attestation medici-finance/assay#5064), desk-tools and statusgen v1.0.67, darwin arm64 host. Delivery: commit da4c268ac2 through merge 6cdcf0c06a (PR #4073). The admitted-home check (deskdispatch --check-verifier) exited 0 before the bracket opened, again before host rows 4, 6 and 7, and again on the finished Evidence.

The table above is the host verifyrun witness for rows 4, 6 and 7. Rows 1, 2, 3 and 5 (check:ci) ran once through the admitted container-netns lane: verifyrun --open-bracket on the host (opened 2026-10-30T23:09:35Z), the container run (the arm64 manifest of the v1.0.67 runner release index sha256:f073d208c69e, native platform, --network none, home and receipt mounted read-only, --rows 1,2,3,5, --timeout 40m), verifyrun --close-bracket on the host (exit 0) and verifyrun --bracket-digest (verdict=match). No attempt was discarded. The closed container witness follows verbatim.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:77f865563b4f | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:574171971f67 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:eacdb8f51a4c | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d
Container-Image: sha256:5fa242b9d02a5f769fe9d541f5a8ad33ae291e418a735617faf4812ef21e8629
Verification-Attestation: medici-finance/assay#5064 run=35c59a17128fc1a6d55030735317a94c096a2090bd4eb5125067f903ad9e10d0 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=any
Admission-Bracket: run=35c59a17128fc1a6d55030735317a94c096a2090bd4eb5125067f903ad9e10d0 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae tree=sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d witness=sha256:fecce9746fdd87c883dd1376c94332ef1545091cec8807268132e545e0b93826 opened=2026-10-30T23:09:35Z closed=2026-10-30T23:17:14Z before=receipt after=check-verifier

Row notes:

- Row 1: pass in the container lane, exit 0, the named test passes. Mutation (supplemental, exported copy of the source, GOPROXY=off, no acceptance credit): replacing the claim-key refusal with a sanitizing rewrite fails the subtest for traversal-shaped claim keys on both branches, a/b among them; the unmutated copy passes.
- Row 2: pass in the container lane, exit 0, the named test passes. Mutation (supplemental, no credit): removing the in-home containment arm of the outcome-record guard fails the subtests for an absolute path inside the home and for a symlink alias into the home; the unmutated copy passes.
- Row 3: could-not-check. The container lane observed exit 1 and the evidence places the cause in the lane, not in this delivery. No acceptance credit rests on the probes below. Same image, same command, network off, supplemental: on the admitted read-only home, deskevidence passes and 29 top-level tests fail across deskdispatch and deskkit (forge resolution with no origin remote, ASSAY_REPO_FORGES unset, and the credential-chain tests that drive real git). On a standalone clone at the source with a git directory and a forge origin, deskevidence and deskdispatch pass and deskkit has one failure, TestReviewCallerRollback, whose only failing assertion is the real-unwritable-directory case of the pr-monitor suite. That lane runs as uid 0, which ignores the directory's mode. The same suite as uid 1000 on the same clone reports 129 passed, 0 failed. None of these tests is in the delivery diff. The earlier go test timeouts did not recur on the native platform (deskkit 271s). Public CI check-runs at 011f14965e are green for build-test and its desk shards (supplemental, no credit). The suite needs a lane with a git directory, a forge origin (#4327, #4322) and a non-root user before it can be witnessed.
- Row 4: pass, exit 0, output 0.
- Row 5: pass in the container lane, exit 0.
- Row 6: could-not-check. Exit 2 on merged main: COULD-NOT-CHECK, the brief is not in the diff against the source. The row's Expect is pre-merge, on the implementation branch. The tool's merged-brief recipe cannot help: the implementation head 8d38487a98 has no file for this brief, which landed separately. A rerun after this Evidence edit exits 0 with 0 corroborated, 0 disproved and 5 unchecked, all marked unchanged since the merge-base; that is no corroboration. Independent read (no credit): the delivery diff of commit da4c268ac2 touches the three declared follow-up areas: the deskdispatch directory (prompt.go changed; the allocator is the new outcomepath.go and packet.go is unchanged, its constant reused), deskevidence (deskevidence.go, outcomerecord.go) and the verify-desk skill with its Cursor mirror. Row 4 covers the admission out-of-scope judgment; the record-format judgment stays reviewer-owned.
- Row 7: could-not-check. Exit 1, LINT: FAIL 1 problem(s), on the clean source and with this Evidence applied. The single PROBLEM is "desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required", tracked in #4823. No PROBLEM line names desktools-v2/72, which is the part of the Expect this brief owns. Exit 0 cannot be shown until #4823 is resolved.

Accounting: 7 rows; 4 pass (rows 1, 2, 4, 5); 3 could-not-check (rows 3, 6, 7); 0 failed obligations of this brief.

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory.
RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only, and 0o700 grants the owner alone; this run's own outcome directory reads drwx------.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits, so a base another user can write to, where that user could swap the per-claim leaf, is refused.
RISK-VALUE: DERIVED — packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154 — Task 1 asks for the same week-based pruning as packets; the allocator reuses this constant, so the two cannot drift.

**VERIFY: BLOCKED** — 4/7 rows pass at source 011f14965ef2; rows 3, 6 and 7 could-not-check (row 3 needs a lane with a git directory, a forge origin and a non-root user, #4327, #4322; row 6 is pre-merge only; row 7's lint is red on desktools-v2/65, #4823); no obligation of this brief observed failing.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:7212cc619ded | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 | sha256:56609f1e4154 | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5887 run=87a5e71551349cf9395975ead2df56ab9f495a56336be92d6547d416a80b06f2 source=b1f0161efb6121b8ba9e7089011a7d6499c12ab7 brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=any

### Postmerge verification, 2026-11-09, source b1f0161efb

Verifier: claude-opus-5-5, tier any, non-implementer, against merged main source b1f0161efb61 (attestation medici-finance/assay#5887), desk-tools and statusgen v1.0.80, darwin arm64 host. Delivery: commit da4c268ac2 through merge 6cdcf0c06a (PR #4073). deskdispatch --check-verifier exited 0 before the bracket opened and again on the finished Evidence.

The table above is the host verifyrun witness for rows 4, 6 and 7, run after the container bracket closed. Rows 1, 2, 3 and 5 (check:ci) ran once through the admitted container-netns lane. verifyrun --open-bracket ran on the host with --module-cache on a new empty directory, opened 2026-11-09T14:27:39Z. Its module download for all 13 listed roots succeeded and it printed the container flags. The container run used the linux/arm64 manifest sha256:7ed3c93a77e6 of the pinned v1.0.80 runner index sha256:cdb261e28942, on the native platform, with exactly those flags: --network none, --read-only, the cache mounted read-only at /go/pkg/mod, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly. It also mounted the home and the receipt read-only, used a /tmp tmpfs, ran as uid 1000, and passed --rows 1,2,3,5 and --timeout 40m. verifyrun --close-bracket on the host exited 0, and verifyrun --bracket-digest reported verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. No attempt was discarded. The container rows now run under v1.0.80: the module-cache step is no longer refused (medici-finance/assay#4683, medici-finance/assay#5882). The closed container witness follows verbatim.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:8ea5dab65432 | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:76429ac47f9e | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:3a4de9d24997 | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-11-09 | assay-verifier-app[bot] @ b1f0161efb61 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8785a3e34ef6c8419a553b145d27516f08fb3057973130adb2fec2cae60bae08
Container-Image: sha256:7ed3c93a77e66707c6668f423b604f0b53ec66cf654a8c33a385136fd6005fcd
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#5887 run=87a5e71551349cf9395975ead2df56ab9f495a56336be92d6547d416a80b06f2 source=b1f0161efb6121b8ba9e7089011a7d6499c12ab7 brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=any
Admission-Bracket: run=87a5e71551349cf9395975ead2df56ab9f495a56336be92d6547d416a80b06f2 source=b1f0161efb6121b8ba9e7089011a7d6499c12ab7 tree=sha256:8785a3e34ef6c8419a553b145d27516f08fb3057973130adb2fec2cae60bae08 witness=sha256:b3ebde07a4149f395008bf71c35af067e2e9aff60acd8d3ddb890abfb7ef72fd opened=2026-11-09T14:27:39Z closed=2026-11-09T14:31:11Z before=receipt after=check-verifier

Row notes:

- Row 1: pass in the container lane, exit 0, the named test passes. The Expect's mutation clause was not re-run in this pass, because this dispatch runs every test command through verifyrun only. The 2026-10-29 and 2026-10-30 passes record that mutation failing as wanted at earlier sources.
- Row 2: pass in the container lane, exit 0, the named test passes. The mutation clause was not re-run in this pass, for the same reason. deskevidence.go and outcomerecord.go changed after source 011f14965ef2, so the earlier mutation result is from an older version of the guard.
- Row 3: exit 1 in the container lane, so the row is not credited. The lane gap that the 2026-10-29 and 2026-10-30 passes traced still applies. The home is a linked worktree whose git directory lies outside the mounted tree (#4327), and the container has no forge origin (#4322). This pass ran as uid 1000, a non-root user, which removes the third cause the 2026-10-30 pass named. verifyrun keeps no row output, and this pass ran no test command by hand, so it does not name the failing tests. It does not separate a lane failure from a code failure, and it attributes no failure to this delivery.
- Row 4: pass, exit 0. The Expect discriminates only pre-merge on the implementation branch. On merged main the diff against origin/main is empty by construction, so this pass does not discriminate on its own.
- Row 5: pass in the container lane, exit 0.
- Row 6: exit 2 on merged main; verifyrun --check reads it as "fail — the witness records a failure". The Expect is pre-merge only, on the implementation branch, so no post-merge pass is credited for it.
- Row 7: exit 1; verifyrun --check reads it as "fail — the witness records a failure". Under the ruling on medici-finance/assay#4823 (https://github.com/medici-finance/assay/issues/4823#issuecomment-6725046323), a clean repo-wide lint row needs statusgen --root . --lint --acceptance-forge clean at the source in the host lane. An admitted run makes no forge read, so this row is not accepted by --check (medici-finance/assay#4700).

Fail-first: the brief's risk block is all no, and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag. No row is risk-bearing, so no fail-first witness is owed.

Accounting: 7 rows. Rows 1, 2, 4 and 5 pass (row 4 is pass by construction on merged main). Rows 3, 6 and 7 are not credited: row 3 in the lane, #4327 and #4322; row 6 is pre-merge only; row 7 needs a forge read, #4700. verifyrun --check: 4 pass, 3 fail, 0 not run or missing, of 7.

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory.
RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only, and 0o700 grants the owner alone.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits, so a base another user can write to, where that user could swap the per-claim leaf, is refused.
RISK-VALUE: DERIVED — packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154 — Task 1 asks for the same week-based pruning as packets; the allocator reuses this constant, so the two cannot drift.

**VERIFY: BLOCKED** — 4/7 rows pass at source b1f0161efb61, and the container rows now run under v1.0.80. Rows 3, 6 and 7 are not credited: row 3 exited 1 in a lane with no git directory or forge origin (#4327, #4322); row 6 is pre-merge only; row 7 needs the forge read the #4823 ruling requires (#4700). No obligation of this brief was observed failing because of this delivery.
### Postmerge verification, 2026-11-13, source 4b7942836f

Verifier: claude-opus-5-5, tier strong, non-implementer, against merged main source 4b7942836f5d809de264179926543417401fdc06 (attestation medici-finance/assay#6361), desk-tools and statusgen v1.0.83, darwin arm64 host, git 2.56.1. Delivery: commit da4c268ac2 through merge 6cdcf0c06a (PR #4073), parents 924cd4cfb6fd and 8d38487a98de. deskdispatch --check-verifier exited 0 before the first bracket opened, again before the second, and again on the finished Evidence. deskpathguard rederive reported 7 rows, 0 author-added. This Evidence carries no Verification-Table marker: the brief file is absent at the delivery's merge base 9a2c6ba2bc85 and at its second parent (the brief was authored on main while the delivery was in flight), so no base table exists for the marker to name.

Rows 1, 2, 3 and 5 (check:ci) ran in the container admission bracket, selected with --rows 1,2,3,5: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache on a new empty directory outside the home; it downloaded the 14 listed module roots against their go.sum and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned image ghcr.io/medici-finance/verify-runner:v1.0.83@sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4 on its native linux/arm64 entry (go1.27.3, git 2.56.0, statusgen v1.0.83), with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly. The home and its git directory were mounted read-only, the receipt read-only, the Go build cache and HOME on a tmpfs, uid 501. statusgen verifyrun --bracket-digest on the closed witness reported verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:020c588e5dca | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:b48769c9812e | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:c9a2688ae9bd | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:ad62f7f21902 | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:25e3722044ba1d021c97840158a052db5772d8480f65ff4c6c31f8efc00bd5b7
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6361 run=78b9879fb6f7c7634d4dedcab57bec181c6d60276b1013f8dbbdda2e1077b01d source=4b7942836f5d809de264179926543417401fdc06 brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=78b9879fb6f7c7634d4dedcab57bec181c6d60276b1013f8dbbdda2e1077b01d source=4b7942836f5d809de264179926543417401fdc06 tree=sha256:25e3722044ba1d021c97840158a052db5772d8480f65ff4c6c31f8efc00bd5b7 witness=sha256:b778bd7ba3b71fd3bcf69b0af71b8539f686eb917ca89e9e63fa6c244af4b07c opened=2026-11-13T03:22:47Z closed=2026-11-13T03:26:31Z before=receipt after=check-verifier

Rows 4, 6 and 7 (check) ran on the darwin host after the bracket closed, on the unedited home, with statusgen verifyrun --rows 4,6,7 --dry-run under a minimal PATH. It printed the table and attestation line below, reproduced unchanged; nothing was written to the home.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:525c66834e8c | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 | sha256:5fa4ede572cd | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6361 run=78b9879fb6f7c7634d4dedcab57bec181c6d60276b1013f8dbbdda2e1077b01d source=4b7942836f5d809de264179926543417401fdc06 brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong

Row notes:

- Row 1: pass in the container lane, exit 0, the named test passes. Its subtests cover the absolute outside path and landing command in the verifier prompt, refusal of a directory inside the home or root (also through a symlink), distinct directories for two claims, and no outcome line in non-verifier prompts. Supplemental mutation probe, no acceptance credit, on a scratch copy of tools/desk outside the home: when outcomeClaimDir rewrites the claim key through sanitizeSegment instead of refusing it, TestVerifierOutcomePath fails as the Expect wants ("a/b" is created as outcomes/a-b on both branches, "want a refusal", and "refused keys still created 7 entr(ies)"). Hyphenated keys such as repo--example-stream--07 are accepted on both branches, and itemKeyRe accepts a-b.
- Row 2: pass in the container lane, exit 0, the named test passes. Supplemental mutation probe, no acceptance credit, same scratch copy: dropping the pathWithin(root, localFile) clause at tools/desk/cmd/deskevidence/outcomerecord.go:76 makes TestOutcomeRecordOutsideRoot fail on the "absolute path inside the home" and "symlink alias into the home" subtests (exit 0, want 5); the relative-path and outside-path subtests still pass. Both files were restored and compared byte-identical to the home.
- Row 3: exit 1 in the container lane, so the row is not credited. This pass names the failing test. The only failure is TestLandedApprovalContext in tools/desk/cmd/deskevidence (landed_test.go:537, "closure docs/streams/verify-outcomes/vf/01-test.json lost repository authority: origin=\"\" err=gitcore: remote-url origin: no URL configured", and the same for verify-outcomes.jsonl); deskdispatch and deskkit are ok. Supplemental, no credit: the same three packages run by hand in the same image and mounts, outside verifyrun, all pass (deskevidence 91s, deskdispatch 34s, deskkit 186s). On the darwin host, the test passes in a plain environment and fails with only the admitted row variables added (GIT_CONFIG_NOSYSTEM=1 and GIT_CONFIG_COUNT=2 pinning grep.patternType and grep.extendedRegexp, from admittedRowEnv in statusgen/verifieradmission.go). The fixture's origin does not survive an inherited GIT_CONFIG_COUNT. That is the open class medici-finance/assay#2715, which already records this test (https://github.com/medici-finance/assay/issues/2715#issuecomment-6835989112). The test came with the closure-snapshot fix (4b4fa4744), not with this delivery, so no failure is attributed to this delivery. The lane gaps earlier passes named (#4327, #4322) no longer show here: the git directory is mounted and deskdispatch passes.
- Row 4: pass, exit 0. The Expect discriminates only pre-merge on the implementation branch. On merged main the diff against origin/main is empty by construction, so this pass does not discriminate on its own.
- Row 5: pass in the container lane, exit 0.
- Row 6: exit 2 on merged main, "COULD-NOT-CHECK: assay:assay:desktools-v2:72 is not in the diff against 4b7942836f5d…". The Expect is pre-merge only, on the implementation branch, so no post-merge pass is credited. The tool's suggested post-merge form (check out the merge's second parent, base at the parents' merge base) cannot run as written, because the brief file is absent at that head.
- Row 7: exit 1, LINT: FAIL 3 problem(s): desk-tools/37, desktools-v2/65 and forge-neutral/54, each "protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required". No PROBLEM line names desktools-v2/72, which is the part of the Expect this brief owns. Under the ruling on medici-finance/assay#4823 (https://github.com/medici-finance/assay/issues/4823#issuecomment-6725046323), a clean repo-wide lint row needs statusgen --root . --lint --acceptance-forge clean at the source in the host lane; an admitted run makes no forge read, so this row is not accepted (medici-finance/assay#4700). Supplemental, no credit: in a scratch clone at this source, statusgen --root . --lint --acceptance-forge exited 0 with LINT: PASS, both before and after this Evidence was appended, and statusgen verifyrun --check --acceptance-forge then read 4 pass, 3 fail, 0 could-not-run or missing, of 7.

Attempts: two in this dispatch. Attempt 1, 2026-11-13T03:12:36Z to 2026-11-13T03:16:38Z, ran rows 1, 2, 3 and 5 in a closed bracket (verdict=match) with the same results: rows 1, 2 and 5 passed and row 3 failed through the same test. It was discarded as superseded: its run envelope also set safe.directory through GIT_CONFIG_COUNT=1, which was first suspected as the cause, so the bracket was re-run with that setting moved to a mounted global config. The re-run reproduced the failure, which places the cause in the admitted row variables rather than the envelope. Attempt 2, 2026-11-13T03:22:32Z to 2026-11-13T03:35:44Z, is the credited run: the bracket above and the host rows.

Fail-first: the brief's risk block is all no, and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag. No row is risk-bearing, so no fail-first witness is owed.

Accounting: 7 rows. Rows 1, 2, 4 and 5 pass (row 4 is pass by construction on merged main). Rows 3, 6 and 7 are not credited: row 3 under the admitted row environment, medici-finance/assay#2715; row 6 is pre-merge only; row 7 needs a forge read, medici-finance/assay#4700.

Risk-bearing values, re-read at this source; the delivered lines did not move since the 2026-11-09 pass:

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory.
RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only, and 0o700 grants the owner alone.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits, so a base another user can write to, where that user could swap the per-claim leaf, is refused.
RISK-VALUE: DERIVED — packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154, used by the outcome prune @ tools/desk/cmd/deskdispatch/outcomepath.go:128 — Task 1 asks for the same week-based pruning as packets; the allocator reuses this constant, so the two cannot drift.

**VERIFY: BLOCKED** — 4/7 rows pass at source 4b7942836f5d. Rows 3, 6 and 7 are not credited: row 3 fails only in TestLandedApprovalContext, a closure-snapshot test that does not survive verifyrun's admitted git variables (medici-finance/assay#2715); row 6 is pre-merge only; row 7 needs the forge read the #4823 ruling requires (medici-finance/assay#4700). No obligation of this brief was observed failing because of this delivery.

### Postmerge verification, 2026-11-19, source 44a02f3dc7

Verifier: claude-opus-5-5, tier strong, non-implementer, against merged main source 44a02f3dc7ab850eec2143981f060769ca9b492c (attestation medici-finance/assay#7185), desk-tools and statusgen v1.0.88, darwin arm64 host, git 2.56.1. Delivery: PR #4073, merge 6cdcf0c06afa (parents 924cd4cfb6fd and 8d38487a98de). deskdispatch --check-verifier exited 0 before the first row, again at each bracket open and close, and again on the finished Evidence. deskpathguard rederive reported 7 rows, 0 author-added, base 44a02f3dc7ab. Every step ran with the kit's verifier Git environment (GIT_* removed, GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0) and KUBECONFIG=/dev/null.

Rows 4, 6 and 7 (check) ran first on the darwin host, on the unedited home, with statusgen verifyrun --rows 4,6,7 --acceptance-forge. The table below is what it appended, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:b83c61952f14 | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 | sha256:7cd77f646bba | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7185 run=3b5d578e1a7c84cff85b25b40b7dea1aa60114870e982d6bf05b4e827c74ef76 source=44a02f3dc7ab850eec2143981f060769ca9b492c brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong

Rows 1, 2, 3 and 5 (check:ci) ran after that in the container admission bracket, selected with --rows 1,2,3,5: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache on a new empty directory outside the home and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned image ghcr.io/medici-finance/verify-runner:v1.0.88@sha256:dd758003b77fc718ec5b9b3e3a34529a422eb87316a605207bdad2560ca169bc (the digest the v1.0.88 release names), linux/arm64, with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly. The home and its git directory were mounted read-only, the receipt read-only, /tmp and HOME on a tmpfs, uid 501. statusgen verifyrun --bracket-digest on the closed witness reported verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:a37d9a08cadb | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:f79ae1a47a73 | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:faab55c2ae43 | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:ce12c636026d | 2026-11-19 | assay-verifier-app[bot] @ 44a02f3dc7ab (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:5dd081c32718fb7125c7bc3a1719e0457d25b39639603371ed27ca74f3b2c434
Container-Image: sha256:dd758003b77fc718ec5b9b3e3a34529a422eb87316a605207bdad2560ca169bc
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7185 run=3b5d578e1a7c84cff85b25b40b7dea1aa60114870e982d6bf05b4e827c74ef76 source=44a02f3dc7ab850eec2143981f060769ca9b492c brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=3b5d578e1a7c84cff85b25b40b7dea1aa60114870e982d6bf05b4e827c74ef76 source=44a02f3dc7ab850eec2143981f060769ca9b492c tree=sha256:5dd081c32718fb7125c7bc3a1719e0457d25b39639603371ed27ca74f3b2c434 witness=sha256:b0409fb1b4a88de476b04f27403ac0d77c52b5aa515f0a15d9992c8654b91921 opened=2026-11-19T07:14:10Z closed=2026-11-19T07:18:06Z before=receipt after=check-verifier

Row notes:

- Row 1: pass in the container lane, exit 0, and the named test passes.
- Row 2: pass in the container lane, exit 0, and the named test passes. The guard it covers is still at tools/desk/cmd/deskevidence/outcomerecord.go:77 (relative path, or pathWithin(root, localFile), refused with --root).
- Row 3: exit 1 in the container lane, so the row is not credited. Supplemental, no credit: the same three packages in the same image and mounts, run by hand outside verifyrun. With verifyrun's admitted row variables added (GIT_NO_REPLACE_OBJECTS=1, GIT_ATTR_NOSYSTEM=1, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=2 pinning grep.patternType and grep.extendedRegexp, from admittedRowEnv at statusgen/verifieradmission.go:181-186), deskevidence passes and 11 tests fail. Eight are in deskdispatch: TestResumeRealAllocator, the four TestRework* home tests, TestCrossVerifierClaimsTracking, TestCrossRepoDispatchExecution and TestCrossRepoClaimRootOrigin. Two are in deskkit: TestOriginSlugPinAccepted and TestOriginSlugGrammar. Each fails with "remote URL could be changed by config the repository-local reader does not merge: command-priority config (GIT_CONFIG_COUNT, GIT_CONFIG_PARAMETERS or GIT_CONFIG) is set" or "cannot read the tracking checkout's fetch origin". That is the open class medici-finance/assay#2715. The eleventh, deskkit TestOverrideIdentityResolvesRosterLogin, panics with a nil dereference at scanoverride_identity_test.go:105. user.Current() fails for uid 501, which has no passwd entry in the image (CGO is enabled, so USER is never consulted), and the skip message then reads u.Username off a nil u. That is the open medici-finance/assay#4329. Without the admitted variables, deskevidence and deskdispatch pass and only TestOverrideIdentityResolvesRosterLogin fails. None of these tests came with this delivery. TestLandedApprovalContext, the failure the 2026-11-13 pass named, now passes under the admitted variables.
- Row 4: pass, exit 0. The Expect discriminates only pre-merge, on the implementation branch. On merged main the diff against origin/main is empty by construction, so this pass does not discriminate.
- Row 5: pass in the container lane, exit 0.
- Row 6: exit 2 on merged main. The Expect is pre-merge only, on the implementation branch: git merge-base origin/main HEAD is HEAD itself, so the brief is not in the diff. No post-merge pass is credited. A by-hand run after the host table was appended exited 0 only because the uncommitted Evidence edit put the brief in the diff, and every consumer line was UNCHECKED "unchanged since the merge-base". That is not discrimination either.
- Row 7: exit 1, LINT: FAIL 3 problem(s): desk-tools/37, desktools-v2/65 and forge-neutral/54, each "protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required". No PROBLEM line names desktools-v2/72, which is the part of the Expect this brief owns. The row's literal command makes no forge read, so under the ruling on medici-finance/assay#4823 (https://github.com/medici-finance/assay/issues/4823#issuecomment-6725046323) a clean repo-wide lint stays with the forge-reading lane (medici-finance/assay#4700), and this row is not accepted.

Attempts: three in this dispatch, in this order. Attempt 1, 2026-11-19T06:55:23Z to 2026-11-19T06:57:24Z: the host rows 4, 6 and 7, recorded above, uncredited (credit-unchecked). Attempt 2, 2026-11-19T07:00:34Z to 2026-11-19T07:06:09Z: rows 1, 2, 3 and 5 in a closed bracket (verdict=match), with the same results as below (rows 1, 2 and 5 pass, row 3 exit 1). It was discarded as superseded and its witness is not landed: USER was unset in its container envelope and was suspected as the cause of the deskkit panic, so the bracket was re-run with USER set. The re-run reproduced the panic, which places the cause in the image's missing passwd entry, not the envelope. Attempt 3, 2026-11-19T07:13:59Z to 2026-11-19T07:18:06Z, is the bracket above.

Fail-first: the brief's risk block is all no, and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag. No row is risk-bearing, so no fail-first witness is owed.

Accounting: 7 rows. Rows 1, 2, 4 and 5 pass (row 4 is pass by construction on merged main). Rows 3, 6 and 7 are not credited: row 3 fails under the admitted row environment (medici-finance/assay#2715) and in an image without a passwd entry for the run uid (medici-finance/assay#4329). Row 6 is pre-merge only. Row 7 needs a forge read (medici-finance/assay#4700).

Risk-bearing values, re-read at this source. The delivered lines have not moved since the 2026-11-13 pass:

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory. This dispatch's own outcome directory is that cache form.
RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only, and 0o700 grants the owner alone.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits. A base that another user can write to, where that user could swap the per-claim leaf, is refused.
RISK-VALUE: DERIVED — packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154, used by the outcome prune @ tools/desk/cmd/deskdispatch/outcomepath.go:128 — Task 1 asks for the same week-based pruning as packets. The allocator reuses this constant, so the two cannot drift.

**VERIFY: BLOCKED** — 4/7 rows pass at source 44a02f3dc7ab. Rows 3, 6 and 7 are not credited: row 3 exits 1, and a supplemental by-hand run in the same lane names 11 failing tests, none from this delivery: 10 refuse origin reads under verifyrun's admitted GIT_CONFIG_COUNT (medici-finance/assay#2715), and 1 nil-dereferences when the image has no passwd entry for uid 501 (medici-finance/assay#4329). Row 6 is pre-merge only. Row 7 needs the forge read that the #4823 ruling requires (medici-finance/assay#4700). No obligation of this brief was observed failing because of this delivery.

### Postmerge verification, 2026-11-25, source 77ff0f2222

Verifier: claude-opus-5-5, tier strong, non-implementer, against merged main source 77ff0f2222a28df926c38ad30fea4a368173712e (attestation medici-finance/assay#7703, a closed dispatcher record), desk-tools and statusgen v1.0.94, darwin arm64 host, git 2.56.1. Delivery: PR #4073, merge 6cdcf0c06afa (parents 924cd4cfb6fd and 8d38487a98de). deskdispatch --check-verifier exited 0 before the first row, at the bracket open and close, and again on the finished Evidence. deskpathguard rederive reported 7 rows, 0 author-added, base 77ff0f2222a2. Every step ran with the kit's verifier Git environment (GIT_* removed, GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0) and KUBECONFIG=/dev/null.

Rows 4, 6 and 7 (check) ran first on the darwin host, on the unedited home, with statusgen verifyrun --rows 4,6,7 --acceptance-forge --rule51. Row 7 is in the rule-51 class (a bare repo-wide statusgen --root . --lint with a clean-exit Expect), so the run also ran the pinned v1.0.94 lint with --acceptance-forge under its own verifier read credential and stamped the row. The table below is what it appended, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `git diff origin/main -- tools/desk/internal/deskkit/verifiersource.go tools/desk/internal/deskkit/verifierattestation.go tools/desk/internal/deskkit/verifyoutcomes.go \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers --brief desktools-v2/72 --base $(git merge-base origin/main HEAD)` | fail exit=2 | sha256:9b2795eda736 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --lint` | fail exit=1 rule51=pass forge-rc=0 forge-out=sha256:9f2a71c1ddab pin=v1.0.94 cred=verifier-run-read cred-digest=sha256:c41cf9ddd4d9 stamp=sha256:ce85b91a46b888d0f4af376b1a60b3a4dba81de163343fc3f5d404abada08e45 | sha256:f0d0315654e9 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7703 run=ec58978804fae4fb2233e948e6b38f30687a6f8ce2d633cb4dff38ec03013279 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong

Rows 1, 2, 3 and 5 (check:ci) ran after that in the container admission bracket, selected with --rows 1,2,3,5: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache on a new empty directory outside the home and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned image by digest only, ghcr.io/medici-finance/verify-runner@sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a (the v1.0.94 reference in the house runner lock), with no platform flag; it resolved to linux/arm64 (go1.27.4, git 2.56.1, statusgen v1.0.94). It used the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly. The home and its git directory were mounted read-only at their host paths, the receipt read-only, /tmp (Go build cache and HOME) on a tmpfs, uid 501, and the run took --timeout 40m. statusgen verifyrun --close-bracket exited 0, and statusgen verifyrun --bracket-digest on the closed witness reported verdict=match with span-lines tree-digest:1, container-image:1, attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestVerifierOutcomePath$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:51e61dc601fd | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordOutsideRoot$' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:2e7005a045c5 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ ./cmd/deskdispatch/ ./internal/deskkit/ -count=1` | pass exit=0 sandbox=container-netns | sha256:80cddcb4676f | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:588556806cec | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:375e170d062d7719fceacddf3d75d209322c4d7e4849efdd1302389e71353d20
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7703 run=ec58978804fae4fb2233e948e6b38f30687a6f8ce2d633cb4dff38ec03013279 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desktools-v2/brief-72-land-verifier-outcome-records-from-a-dispatcher-named-path-outside-the-home.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ec58978804fae4fb2233e948e6b38f30687a6f8ce2d633cb4dff38ec03013279 source=77ff0f2222a28df926c38ad30fea4a368173712e tree=sha256:375e170d062d7719fceacddf3d75d209322c4d7e4849efdd1302389e71353d20 witness=sha256:b6bd08a2553564ed529ca87123eb6896fbcee2ec3f2729d20c70cee0b3701154 opened=2026-11-25T05:01:27Z closed=2026-11-25T05:06:25Z before=receipt after=check-verifier

Row notes:

- Row 1: pass in the container lane, exit 0, and the named test passes. The mutation clause was not re-run in this pass; earlier passes record it failing as wanted. The allocator file has changed since the 2026-11-13 source, so that earlier mutation result is not carried to this source.
- Row 2: pass in the container lane, exit 0, and the named test passes. The guard it covers is at tools/desk/cmd/deskevidence/outcomerecord.go:76 (a relative path, or pathWithin(root, localFile), refused with --root). The mutation clause was not re-run in this pass.
- Row 3: pass in the container lane, exit 0, the first time this row has passed in an admitted run. All three packages pass under verifyrun's admitted row environment. This pass did not trace the cause. Two changes since the 2026-11-19 source plausibly explain it: commit 38c3be720f made the override-identity test hermetic (the image still has no passwd entry for uid 501, so the nil-dereference medici-finance/assay#4329 described no longer fires there), and the admitted-row environment work in statusgen verifyrun (commits a034f1abaf, f71602140e and later, around medici-finance/assay#7007) changed which git config variables reach a row.
- Row 4: pass, exit 0, output 0. The Expect discriminates only pre-merge, on the implementation branch. On merged main the diff against origin/main is empty by construction, so this pass does not discriminate.
- Row 5: pass in the container lane, exit 0.
- Row 6: exit 2 on merged main; the row is not credited. The Expect is pre-merge, on the implementation branch: on the home, git merge-base origin/main HEAD is HEAD itself, so the brief is not in the diff. Supplemental, no credit: a by-hand run after the host table was appended exited 0 only because the uncommitted Evidence edit put the brief in the diff, with 0 corroborated, 0 disproved and 5 UNCHECKED, every entry "unchanged since the merge-base"; that judges nothing. As written, with a base of git merge-base origin/main HEAD and no delivery pin, the row has no post-merge form that can give a verdict. That is the open check-definition class medici-finance/assay#6099 (see also medici-finance/assay#1281). A row-definition repair (an explicit delivery and base, or a pre-merge-only marking the closure reader accepts) needs a reviewed amendment and a fresh dispatch.
- Row 7: offline exit 1, LINT: FAIL 3 problem(s): desk-tools/37, desktools-v2/65 and forge-neutral/54, each "protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required". No PROBLEM line names desktools-v2/72. The rule-51 lane stamped the witness rule51=pass forge-rc=0 pin=v1.0.94: the pinned lint with --acceptance-forge under the run's verifier read credential exited 0, the forge-reading lane the medici-finance/assay#4823 ruling names.

Attempts: two in this dispatch, in this order. Attempt 1, 2026-11-25T04:58:23Z to 2026-11-25T05:00:49Z: the host rows 4, 6 and 7, recorded above. Attempt 2, 2026-11-25T05:01:10Z to 2026-11-25T05:06:25Z: rows 1, 2, 3 and 5 in the closed bracket above. No attempt was discarded.

Fail-first: the brief's risk block is all no, and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag. No row is risk-bearing, so no fail-first witness is owed.

Accounting: 7 rows. Rows 1, 2, 3, 4 and 5 pass (row 4 by construction on merged main). Row 7 passes through the rule-51 forge lane. statusgen verifyrun --check --acceptance-forge on the finished Evidence reads 6 pass, 1 fail (row 6), 0 could-not-run, 0 author-added; row 7 reads "rule-51 lane: offline lint red; --acceptance-forge clean at v1.0.94 @ 77ff0f2222a2". Row 6 is not credited: it exits 2 on merged main and has no admissible post-merge form (medici-finance/assay#6099). No obligation of this brief was observed failing because of this delivery.

Risk-bearing values, re-read at this source:

RISK-VALUE: DERIVED — outcomeDirName = "outcomes" @ tools/desk/cmd/deskdispatch/outcomepath.go:25, with cache subpath assay/outcomes @ :60 — Task 1 names exactly these two forms: outcomes/{claim key} beside --prompt-file, else assay/outcomes/{claim key} under the user cache directory. This dispatch's own outcome directory is that cache form.
RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141 — Task 1 creates the directory owner-only, and 0o700 grants the owner alone; this dispatch's outcome directory reads drwx------.
RISK-VALUE: DERIVED — refusal mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:164 — the group-write and other-write bits. A base that another user can write to, where that user could swap the per-claim leaf, is refused.
RISK-VALUE: DERIVED — packetMaxAge = 7 * 24 * time.Hour @ tools/desk/cmd/deskdispatch/packet.go:154, used by the outcome prune @ tools/desk/cmd/deskdispatch/outcomepath.go:128 — Task 1 asks for the same week-based pruning as packets. The allocator reuses this constant, so the two cannot drift.

**VERIFY: BLOCKED** — 6/7 rows pass at source 77ff0f2222a2, row 3 for the first time in an admitted run and row 7 through the rule-51 forge lane. Row 6 is not credited: it exits 2 on merged main and its Expect is pre-merge only, with no admissible post-merge form (check-definition, medici-finance/assay#6099). No obligation of this brief was observed failing because of this delivery.

## Review

Gate: model. Independent correctness review of the exact head.
