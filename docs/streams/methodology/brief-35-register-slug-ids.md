---
brief: methodology/35
title: 'Register IDs become letter-prefixed slugs (F-<slug>, I-<slug>, 10-20 chars) — no counter, no collisions'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> decision on [I-32](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-do-registers-still-need-sequential-numbers-merge-time-id-assig.md)'s question)
sources: ["human:<name> 2026-07-10: give IDs a length of 10-20 prefixed by a letter; the 10-20 is a slug/random word, prefer slug", "INTAKE [I-32](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-do-registers-still-need-sequential-numbers-merge-time-id-assig.md) (the analysis + today's evidence: 6-PR renumber cascade, 3 brief-number collisions)", "methodology/23 (per-entry registers — the substrate)", "methodology/33 (register links — the reference convention this composes with)", "freshness-checked 2026-07-10 @ post-#235 main"]
why: >-
  Sequential register IDs are the last contended resource after methodology/23: author-side
  allocation is a global mutex across parallel sessions, paid three recorded times plus
  today's full-chain renumber cascade. A letter-prefixed slug is unique by meaning instead
  of by coordination — two sessions filing different findings can never collide, and the ID
  says what the entry is.
---

# Brief 35 — Register slug IDs

## Context
files: `../assay-toolkit/statusgen/` (ID validation, uniqueness lint, Affects/reference matching),
`docs/streams/{intake,findings}/` conventions text (the generated-view headers),
`../oit/.claude/skills/author-brief/SKILL.md` (in-repo wrapper — rule 2 examples)
out-of-repo files: `~/.claude/skills/author-brief/SKILL.md` (core rule 2 examples — per
issue #221 protocol)
facts:
- **Format (human:<name>, verbatim intent):** `<letter>-<slug>` — letter = register type (`F`
  findings, `I` intake), slug = 10–20 chars, `[a-z0-9-]`, starts/ends alphanumeric,
  DERIVED FROM THE TITLE (prefer slug; a random word is the fallback only when the title
  yields nothing usable). Examples: `F-ws-token-expiry`, `I-model-mix-tiers`,
  `F-oracle-topology`. The entry's filename stays the existing `YYYY-MM-DD-<long-slug>.md`
  convention; the ID is the frontmatter `id:` and the citation handle.
- **Uniqueness is lint-enforced, not counter-enforced:** statusgen PROBLEMs on any two
  entry files sharing an `id:` within a register. Cross-session collision now requires two
  sessions to file entries with near-identical titles simultaneously — and the fix is a
  one-word slug tweak, not a cascade renumber.
- **Legacy IDs freeze:** existing numeric IDs ([F-01](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-11-prod-rollout-blocker-text-was-stale.md)…, [I-01](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-frontend-ui-issues-fix.md)…) remain valid forever — 250+
  references and the entire history keep meaning. The legacy grammar INCLUDES the
  collision-suffix form `F-NN-a` (`^[FI]-\d+(-[a-z])?$`): [F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md)'s 2026-07-10 double-filing
  resolved by suffixing the sibling to F-22-a (human:<name>'s call), and that form is grandfathered
  wherever legacy IDs are. NEW entries in either legacy form (numeric or suffixed) are a
  lint PROBLEM (prevents regression to the counter).
- **Code-verified state (2026-07-10, desk read of tools/statusgen):** the contiguity check
  is ALREADY GONE — methodology/23's implementation replaced it with duplicate-ID detection
  + a git-history deletion check (registers.go `registerIntegrityProblems`,
  `deletedRegisterFiles` — tombstone-not-delete is machine-enforced, stronger than
  contiguity was). There is currently NO id-format validation at all (`id: F-banana` loads
  silently today) — this brief ADDS the first format rules rather than changing existing
  ones. The Affects/staleness machinery reads frontmatter ids via `parseFindingsDir`
  (string-typed — already slug-compatible); `findingHeadRe`'s numeric-only pattern is the
  LEGACY single-file parser, untouched. Net scope: format validation + legacy-freeze
  rules + the ordering fix below; no contiguity work, no Affects work.
- **View ordering:** entries sort lexicographically by id today (migrate.go) — correct for
  zero-padded numerics and for F-22-a, but slug ids would order alphabetically after the
  numeric block. Change the generated-view ordering to date-then-id so mixed-era registers
  read chronologically.
- **Tamper-wire honesty ([F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) lineage):** contiguity detected deletions; slugs have no
  sequence. The replacement wires, stated plainly where the contiguity rule was
  documented: entry-file deletion is PR-diff-visible, the generated view is single-writer
  on main (a deletion shows as a view diff in the regen commit), and the tombstone rule
  (amend disposition to rejected, never delete the file) stays. State this trade in the
  register headers; do not silently drop the old claim.
- **Reference matching updates everywhere IDs are parsed:** the Affects/stale-flag
  machinery, methodology/33's link-text pattern (becomes `^[FI]-[a-z0-9-]+$`), the
  verify-gate/board renderers, grep guidance in the author-brief rule-2 text. Implementer
  greps statusgen for the old `[FI]-\d+` patterns and updates each — enumerate the sites
  in the PR description (shared-value consumer rule).
- **Scope:** registers only. RETRO entries (R-NN) are a dated cadence log — out of scope.
  BRIEF numbers share the collision disease ([I-32](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-do-registers-still-need-sequential-numbers-merge-time-id-assig.md) notes 3 hits today) but have harder
  constraints (typed `depends:` in-PR); explicitly out of scope here — [I-32](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-do-registers-still-need-sequential-numbers-merge-time-id-assig.md)'s scoping owns
  that question.
- [I-32](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-do-registers-still-need-sequential-numbers-merge-time-id-assig.md)'s merge-time-assignment proposal is SUPERSEDED by this decision — its entry gets
  the decision note + disposition flip in PR #252 (already amended there), not here.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo skill edit per issue #221's protocol (declared above; apply last; diff in PR
  body).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. statusgen: new-form ID validation (format, length, charset) + per-register uniqueness
   PROBLEM + numeric-form-for-new-entries PROBLEM + legacy freeze; update every ID-parsing
   site (enumerate in PR description).
2. Register headers (the generated-view preamble text): new ID convention + the tamper-wire
   trade note per facts.
3. Both author-brief homes: rule-2 examples gain the slug form.
4. Tests: valid slug id passes; 9-char and 21-char slugs fail; duplicate id across two
   fixture entries fails; new numeric id fails; legacy numeric set passes untouched;
   Affects matching works for both forms.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes every Task-4 case |
| 2 | fixture entry `id: F-slug-id-demo-x` → `--lint` | 0 (new form accepted) |
| 3 | fixture entry `id: F-33` (new file, numeric) → `--lint` | non-zero, names the numeric-regression rule |
| 4 | `grep -c "10–20\|10-20" docs/streams/findings/README* docs/streams/FINDINGS.md 2>/dev/null \| awk -F: '{t+=$2} END{print (t>0)?1:0}'` | 1 (convention documented in the register surface) |
| 5 | PR body contains the out-of-repo diff (#221) + the ID-parsing site enumeration | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `abc64aec`, impl PR #590 + flip PR #652, 2026-07-18)

Verifier ran isolated (own worktree). **Verdict question — are all register IDs in canonical slug form on main? No, by
design:** numeric IDs (`F-01`..`F-41`, `I-01`..`I-41`; `F-32`/`I-31` absent) are frozen legacy, grandfathered by
`grandfatheredIDs()` (all existed at the merge-base). New entries use slug form; the tool produces NO diffs on main;
`--lint` is green. Migration is complete by design — the brief explicitly does NOT rename existing numeric IDs.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen`; `idvalidate_test.go` covers slug/legacy/invalid regex, 10/20-char boundaries (9 fail, 10/20 pass, 21 fail), new-numeric-in-fixture (`F-33` → PROBLEM), duplicate-slug, mixed-form, intake entries, view ordering |
| 2 | fixture `id: F-slug-id-demo-x` → `--lint` | 0 | new slug accepted; exit 0, no PROBLEM; fixture removed, tree clean |
| 3 | fixture `id: F-33` (numeric, git-less) → `--lint` | non-zero (expected) | PASS — backed by `TestIDFormatProblemsNewNumericInFixture` (`idvalidate_test.go:122-141`), which exercises the exact `idFormatProblems` path `--lint` calls. **Caveat:** the isolated worktree was reaped mid-run (by an external `git worktree prune`) before a fresh end-to-end run; the contract is proven by the Row-1 unit test instead. (On real main literal `F-33` already exists → would also fire duplicate-id; clean numeric equiv `F-50` covered by `TestIDFormatProblemsMixedFormats`.) |
| 4 | convention text "10–20 chars" in register views | present | `docs/streams/FINDINGS.md:11`, `INTAKE.md:13`/`:724` carry it (generated views; no per-dir README) |
| 5 | PR #590 body: out-of-repo diff (#221) + ID-parsing site enumeration | present | full `~/.claude/skills/author-brief/SKILL.md` diff (rule-2 + template, committed stopgap `1272f6e`); 9-site enumeration (`model.go:46`, `registers.go` ×4, `migrate.go` ×2, `alarms.go:21-22`, `idvalidate.go`, `idvalidate_test.go`) + intentionally-not-updated legacy parsers. Out-of-repo edit confirmed LIVE in the skill (lines 91, 147-154) |
| 6 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) |

`gate: model`, all four risks `no` → model flip permitted → `implemented → verified`. Legacy-freeze claim holds:
80 numeric IDs grandfathered, lint green, no broken refs (spot-grep `F-05`/`F-13`/`F-22`/`F-31`/`I-32` all resolve + link).

## Review
Gate: model. Reviewer confirms (a) legacy IDs genuinely freeze (no reference breaks — spot
grep three old F-NN refs), (b) the numeric-regression PROBLEM fires, (c) the tamper-wire
trade is documented where contiguity was, not silently dropped, (d) every enumerated
parsing site actually handles both forms.
