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

## Review

Gate: model. Independent correctness review of the exact head.
