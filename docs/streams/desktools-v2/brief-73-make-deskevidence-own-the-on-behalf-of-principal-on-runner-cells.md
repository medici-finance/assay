---
brief: assay:assay:desktools-v2:73
title: Make deskevidence own the on-behalf-of principal on Runner cells
why: "deskevidence skips any Runner cell that already mentions on-behalf-of, so a hand-typed annotation that names no roster human reaches the branch, and verifiers who lint their home see red rows the landing would have stamped."
wave: 1
depends: []
unblocks: []
effort: M
gate: human
decision-issue: 4070
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes how the tool writes human attribution into Evidence: the stamped subject is a login on a target the roster states private, and the on-behalf-of annotation is an attribution control the segregation-of-duties checks trust. Human acceptance of the restamp-or-refuse rule is required at done."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-22 by statusgen newbrief
sources:
  - 'desktools-v2: Make deskevidence own the on-behalf-of principal on Runner cells'
  - "driver ruling, 2026-10-22, for ratification at merge (its public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22): the verifier adds the principal through the existing tool, no new verb; deskevidence restamps or refuses any annotation that is not a roster principal; the skill says leave Runner cells unannotated and pre-check with deskevidence --dry-run"
  - docs/on-behalf-of.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md (independent verification; attributed result)
  - freshness-checked 2026-10-22 @ 9a2c6ba2bc85558d77c4227984d93a728a7f7e94 (origin/main)
exec-tier: strong
exec-tier-why: "Rewriting or refusing an attribution annotation is a security-relevant write path; the parse must match the lint exactly and must never overwrite a recognised human."
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/deskevidence/principalstamp.go and its tests: follow-up desktools-v2/73"
  - "plugins/assay/skills/verify-desk/SKILL.md and the generated Cursor mirror: follow-up desktools-v2/73"
  - "statusgen attribution lint (principalAttributionProblems): out-of-scope (unchanged; it is the oracle the parity test compares against)"
  - "Evidence rows already on a branch: out-of-scope (never rewritten, as today)"
---

# Brief 73 — Make deskevidence own the on-behalf-of principal on Runner cells

## Context

files: tools/desk/cmd/deskevidence/principalstamp.go, tools/desk/cmd/deskevidence/principalstamp_test.go, tools/desk/cmd/deskevidence/deskevidence.go, tools/desk/cmd/deskevidence/lintdiff.go; tools/desk/internal/deskkit/principal.go (read only); statusgen/attribution.go, statusgen/principal.go (read only); plugins/assay/skills/verify-desk/SKILL.md

facts:
- `deskevidence` stamps the principal at ingest. The subject comes only from the roster, through `deskkit.ResolvePrincipal` (`principalstamp.go:60`-`:72`, `deskkit/principal.go:320`). It is the neutral name on a public target and the login on a target the roster states private (`deskkit/principal.go:145`). Only rows this landing adds are touched (`stampEvidencePrincipal`, `principalstamp.go:78`). Stamping runs before the merge (`deskevidence.go:435`-`:464`).
- The stamper skips any App Runner cell that contains the bare text `on-behalf-of` (`onBehalfOfCellMarker`, `principalstamp.go:51`; `needsPrincipalStamp`, `:171`-`:183`, skip at `:175`-`:177`). The file comment makes this deliberate: "a cell that already carries an annotation — even one the lint will reject — is left for the lint to judge, never rewritten" (`:22`-`:24`).
- The lint reads a narrower marker. `onBehalfOfPrincipalOf` (`statusgen/principal.go:174`) recognises only `on-behalf-of human:<principal>` and takes the last occurrence. So a cell such as `… on-behalf-of <desk role>` is skipped by the stamper but reads as unannotated to the lint. That is a PROBLEM for rows dated on or after the `2026-09-17` cutover (`statusgen/attribution.go:708`, `:750`-`:762`). A `human:` annotation naming someone outside the roster is a PROBLEM only when the roster config is present (`cfg.Configured()`, `attribution.go:765`-`:775`; `onBehalfOfPrincipalRecognised`, `principal.go:147`). Without that config, a wrong principal passes the lint.
- Before every write, `deskevidence` runs the statusgen PROBLEM-diff on the stamped content (`deskevidence.go:636`; `lintDiffAt`, `lintdiff.go:68`). `--dry-run` stops only after every refusing gate, the lint diff included (`deskevidence.go:677`-`:695`). So a clean dry-run already shows exactly what will land, while linting the home shows the rows before the stamp.
- The commit-trailer path already strips planted annotations. `AppendOnBehalfOf` removes any caller-supplied On-behalf-of line before appending the resolved one, "so … a caller cannot pre-seed or shadow the annotation" (`deskkit/principal.go:400`-`:413`). The Runner-cell path has no equivalent.
- Brief 71 brackets container witnesses with a `witness=` digest over the Evidence span. That digest drops only the exact `(on-behalf-of human:<subject>)` parenthetical from Runner cells and collapses the whitespace around it, so restamping one exact-form annotation to another never breaks a bracket, provided nothing else in the row changes. A non-exact annotation (for example a desk-role form) stays inside the hash, so restamping it breaks that bracket and the row is not credited, which fails closed.
- verify-desk has no on-behalf-of guidance (`grep -n on-behalf-of plugins/assay/skills/verify-desk/SKILL.md` is empty at the freshness SHA). Verifiers who lint their home see App rows as unannotated PROBLEMs and hand-annotate them.

Design-fit: proposal 11's independent verification ends in "an attributed result" published
by a separately authorized publisher (`11-target-architecture.md:281`). Attribution is
the publisher's to write, never the runner's to claim. This brief makes the Runner cell
follow the rule the trailer path already follows: the tool resolves the principal and
any planted form is replaced or refused. Proposal 29 keeps attestations as a closed
authority core. The on-behalf-of subject is part of that core, so it must not be an
optional field that free text can fill. No new verb, flag, budget or knob is added.
`--dry-run` is the pre-check.

single-point-of-failure: deskevidence's roster resolution of the subject — backed by the PROBLEM-diff it runs on the stamped bytes before writing, and by the statusgen lint on the target branch in CI (where the roster map is set).

## Human decision

For ratification at merge: driver ruling, 2026-10-22, as in sources; its public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22. For acceptance at done, the
restamp-or-refuse split. Recommended: restamp when the added row's annotation names no
recognised roster human, or is not in the `human:` form. Leave it when it names the same
subject the roster resolves. Refuse (exit 5, nothing written) when it names a different
recognised human, because overwriting one human's name with another is never automatic.
Alternative: refuse in every mismatch case. That is stricter, but it forces verifiers back
into hand-editing cells.

Default if no answer: the recommended split (restamp a non-principal annotation, leave a matching one, refuse a different recognised human) after 2026-10-24.

The default above fixes only the design the implementer builds to at dispatch. It never
substitutes for the `gate: human` acceptance at done, which still decides the
restamp-or-refuse split.

## Ground rules

Normal worker admission, one isolated worktree and one draft PR. Tests use fixture rosters
and the existing seams (`principalSubjectFn`, `lintDiffFn`), with no forge access. Never
rewrite a row already on the target branch. Never take the subject from a flag, an
environment variable or the landed content. Do not change the statusgen lint. Stop at
implemented.

## Task

1. `needsPrincipalStamp` and its caller classify each added App row with the lint's own
   reading: the last `on-behalf-of human:<p>`, matched case-insensitively. Cases: no
   annotation means stamp, as today. Annotation equal to the resolved subject means leave
   it. Annotation naming the same roster human in the other form (the login where the
   target's form is the neutral name, or the reverse) is the same subject, not a different
   human: strip and restamp it in the target's form, because a login must never stay on a
   target the roster states public. Annotation present but not in the `human:` form, or naming no recognised roster
   human, means strip every on-behalf-of fragment from the cell and stamp the resolved one
   (the trailer path's strip-then-append rule). The stamped annotation is always written as
   one parenthetical in the Runner cell and the rest of the row is left byte-identical,
   which keeps brief 71's bracket digest stable. Annotation naming a different recognised
   human means refuse with exit 5, name the row, and write nothing.
2. Recognition uses the same roster data the subject resolution reads, so it does not
   depend on the target's lint config being present. Update the file comment to the new
   rule.
3. Add a parity test with a table of Runner-cell fixtures (bare App, App with the same
   subject, App with a desk-role annotation, App with an unknown `human:` name, App with a
   different known human, App with the same human in the other form, human runner,
   pre-cutover dates). The parity is one-directional: for every fixture where
   `stampEvidencePrincipal` succeeds, `statusgen --lint`, run with a configured fixture
   roster on the stamped content, reports no attribution PROBLEM for that row. The refusal
   of a different recognised human is deliberately stricter than the lint, which accepts
   any recognised human (`statusgen/attribution.go:765`-`:775`, `principal.go:147`); that
   fixture is asserted as a refusal with nothing written and sits outside the parity claim.
   The lint stays unchanged (Ground rules).
4. verify-desk: in the Evidence-landing passage, say to leave Runner cells unannotated,
   never type an on-behalf-of annotation, and pre-check with `deskevidence … --dry-run`,
   not by linting the home, where App rows read as unstamped. Regenerate the Cursor
   mirror.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestPrincipalStamp$' -count=1 -v` | exit 0 and named PASS; the desk-role annotation and the unknown `human:` name are restamped to the resolved subject; the same-subject cell is byte-identical; a different known human refuses with nothing written; rows already on the branch are untouched; mutation: revert `needsPrincipalStamp` to the bare `on-behalf-of` skip — the named test must fail |
| 2 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestPrincipalStampLintParity$' -count=1 -v` | exit 0 and named PASS; for every fixture where the stamp succeeds, the lint reports no attribution PROBLEM on the stamped row; the different-known-human fixture is refused with nothing written and is the one named case stricter than the lint; the other-form fixture is restamped in the target's form; mutation: revert `needsPrincipalStamp` to the bare `on-behalf-of` skip — the named test must fail |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestDryRunRestampsPlantedPrincipal$' -count=1 -v` | exit 0 and named PASS; a dry-run of a landing with a planted desk-role annotation shows the restamped row and runs the PROBLEM-diff before stopping |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ ./internal/deskkit/ -count=1` | exit 0; every package reports `ok` |
| 5 | check:ci | `grep -n 'on-behalf-of' plugins/assay/skills/verify-desk/SKILL.md` | at least one match, in the Evidence-landing passage, naming `--dry-run` |
| 6 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift reported |
| 7 | check | `statusgen --root . --consumers --brief desktools-v2/73 --base $(git merge-base origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |
| 8 | check | `statusgen --root . --lint` | exit 0; no PROBLEM line names this brief |

Pre-mortem: a parser that differs from the lint's (first rather than last occurrence, or
case-sensitive) lets a planted cell through. Row 2 catches that by comparing against the
lint itself. A restamp that overwrites a real human is caught by row 1's different-human
refusal. A change that reaches rows already on the branch is caught by row 1's untouched
case. Mutation check: revert `needsPrincipalStamp` to the bare `on-behalf-of` skip and
confirm rows 1 and 2 go red.

The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Gate: human (gate-why above). Independent correctness and security review of the exact head.
