---
brief: methodology/01
title: statusgen v1.1 — brief-file schema + pre-flight validation (opt-in by schema marker)
wave: 0
depends: []
unblocks: ["methodology/02", "methodology/03", "methodology/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §11 adopt-4 (pre-flight validation)", "spec §13 (structured islands)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)"]
---

# Brief 01 — statusgen v1.1: brief-file schema + pre-flight validation

## Context
files: tools/statusgen/{parse.go,checks.go,load.go,model.go} + new brieffile.go/brieffile_test.go
facts:
- statusgen v1 parses ONLY stream READMEs; brief files are never opened.
- brief-v1 frontmatter shape: the template in `~/.claude/skills/author-brief/SKILL.md` (brief/title/wave/depends/unblocks/effort/gate/risk/issues/schema/authored/sources).
- CRITICAL CONSTRAINT: ~55 legacy briefs across the other 11 streams have NO frontmatter. Validation is OPT-IN: only files whose first line is `---` and whose frontmatter contains `schema: brief-v1` are validated; all others are exempt (log nothing).
- Checker rule: opt-in is fine, environment-conditional is not (final-review lesson).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Parse `docs/streams/<stream>/brief-*.md` frontmatter for files carrying `schema: brief-v1`.
2. Validate per file: all required fields present; `brief:` ID matches `<dirname>/<NN>` of its filename; `wave` int; `depends`/`unblocks` are lists of typed IDs that RESOLVE (target stream + brief row exist); `gate` consistent with `risk` (any yes → human); `sources` non-empty; `effort` in {S,M,L}.
3. Cross-check against the stream README table: the row for `<NN>` exists and its `Wave` matches the frontmatter `wave`.
4. Emit failures as PROBLEM lines through the existing `check()` path (exit 1).
5. TDD throughout; fixtures under testdata/ for: valid brief, missing field, gate/risk mismatch, unresolvable depends, wave mismatch, legacy (no frontmatter → exempt).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestBriefFile -v` | exit 0; ≥6 subtests PASS incl. one named for legacy exemption |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --check` | exit 0 (all methodology briefs validate; legacy briefs exempt) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestBriefFile -v` | 0 | 13 `TestBriefFile*` PASS incl. `TestBriefFileLegacyExempt` | 2026-07-08 | implementer (Opus 4.8) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-08 | implementer (Opus 4.8) |
| 3 | `go run ./tools/statusgen --root . --check` | 0 | 11 methodology briefs validate; 54 legacy briefs exempt; STATUS.md current | 2026-07-08 | implementer (Opus 4.8) |

### Independent re-run

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestBriefFile -v` | 0 | 22 `TestBriefFile*` subtests PASS incl. `TestBriefFileLegacyExempt` | 2026-07-08 | sonnet verifier (non-implementer) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok (41 tests, 0 fail); vet clean | 2026-07-08 | sonnet verifier (non-implementer) |
| 3 | `go run ./tools/statusgen --root . --check` | 0 | no PROBLEM lines; check passes | 2026-07-08 | sonnet verifier (non-implementer) |

Post-merge re-review of the landed implementation (`../assay-toolkit/statusgen/brieffile.go` +
`brieffile_test.go`) against the brief's Task section: required-field/type validation,
`brief:` ID vs filename match, `wave` int check, `depends`/`unblocks` typed-ID
resolution against `byName`/README rows, gate↔risk consistency (any risk `yes` ⇒
`gate: human`), non-empty `sources`, README row + wave cross-check, and PROBLEM output
wired through `run()` (checkBriefFiles appended alongside `check()`/`linkProblems`) —
all present and match the brief. Fixtures extend past the minimum six (missing field,
gate/risk mismatch, unresolvable depends, wave mismatch, legacy exempt, valid) to also
cover bad effort, bad gate, empty sources, ID mismatch, unresolvable unblocks, bad refs,
and a malformed filename (`brief-zz-bad-name.md`) — treated as a strength, not scope
creep. Two gaps flagged in PR #77 review (unterminated-frontmatter exemption edge case,
four canonical risk keys) were adjudicated by the reviewer and routed to
methodology/02 via the amendment in commit `5073b07a` — out of scope here by design,
not counted against this brief.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
