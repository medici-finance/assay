---
brief: assay:assay:harness-portability:17
title: Skill frontmatter conformance lint — Codex / agentskills hard limits fail CI
why: >-
  Two shipped skills already exceed the 1024-character description limit that both the
  agentskills spec and Codex impose (`install` 1103, `pr-review-desk` 1187), and CI is green
  on both. On a current Codex CLI the description is silently cut at 1021 characters plus
  "..."; an older CLI refused to load the skill at all. Either way an adopter on Codex gets a
  skill whose trigger text has lost its tail, and nothing in this repo notices. The limits
  are already written down in the Codex capability matrix — nothing enforces them. This
  brief turns the per-skill hard limits into an exit-code-bearing skillslint rule, reports
  the soft budgets as NOTICEs, and brings the two offending descriptions under the limit so
  the rule lands green.
wave: 0
depends: []
unblocks: ["harness-portability/18"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-23 by intake-desk authoring dispatch
sources: ["freshness-checked 2026-09-23 @ e284ba9b8 — `tools/skillslint` at that SHA enforces a real YAML load, name==dir and a non-empty description (lint.go LintSkills), plus an advisory word-count NOTICE (hidden.go budgetThresholdSkillMd = 3000); no description length, name length/pattern, byte, line or bundle-budget bound exists. `skillslint --root ../..` exits 0 on the bundle at that SHA despite the two over-limit descriptions", "measured 2026-09-23 @ e284ba9b8 over plugins/assay/skills/*/SKILL.md (14 skills, YAML-loaded, Unicode code points): description chars install 1103, pr-review-desk 1187, all others <= 1002; bundle description total 10202; 12 of 14 bodies exceed 8000 bytes (11 exceed 8 KiB); 5 exceed 500 lines; 7 exceed 20000 bytes (~5000 tokens at 4 bytes/token)", "agentskills specification https://agentskills.io/specification read 2026-09-23: `name` 1-64 chars, lowercase a-z 0-9 and hyphens, no leading/trailing/consecutive hyphen, must match the parent directory; `description` 1-1024 chars; body `< 5000 tokens recommended`; `Keep your main SKILL.md under 500 lines`", "Codex skills docs https://learn.chatgpt.com/docs/build-skills read 2026-09-23: the initial skills list uses at most 2% of the model's context window, or 8,000 characters when the window is unknown; Codex shortens descriptions first, then may omit skills with a warning; the page says a selected skill's full SKILL.md is still read", "openai/codex @ 30fc6864cc1318121eca1843c217fe00ce1212f1 codex-rs/ext/skills/src/render.rs read 2026-09-23: MAX_CATALOG_SKILL_DESCRIPTION_CHARS = 1_024 (truncate_catalog_skill_description keeps 1021 chars + \"...\"), DEFAULT_SKILL_METADATA_CHAR_BUDGET = 8_000, SKILL_METADATA_CONTEXT_WINDOW_PERCENT = 2, APPROX_BYTES_PER_TOKEN = 4, MAX_SKILL_PROMPT_BYTES = 8_000 (truncate_main_prompt_contents)", "openai/codex @ 30fc6864 codex-rs/ext/skills/src/host_prompt.rs read 2026-09-23: load_skill_prompts truncates a body to MAX_SKILL_PROMPT_BYTES only when `is_agent_plugin_skill(skill)` — whether an `assay@assay` plugin install is classed that way is UNVERIFIED (Verify row 9)", "https://github.com/openai/codex/issues/13941 (closed): Codex CLI 0.111.0 refused a SKILL.md whose description exceeded 1024 characters — `invalid description: exceeds maximum length of 1024 characters`", "docs/research/codex-harness-capabilities.md line 77 @ e284ba9b8 already records `name` 1-64 / `description` 1-1024 as the Codex contract — documented, not enforced", ".github/workflows/ci.yml @ e284ba9b8: the `skillslint` job runs `cd tools/skillslint && go run . --root ../..`, so a new exit-1 rule inside skillslint gates PRs with no workflow edit"]
consumers: ["tools/skillslint (conformance.go, conformance_test.go, main.go, README.md, testdata/conformance/**): fixed-here (harness-portability/17 lands the conformance rule, its tests, fixtures and README section in this branch's diff)", "plugins/assay/skills/install/SKILL.md description: fixed-here (shortened to <= 1024 chars, trigger phrases first)", "plugins/assay/skills/pr-review-desk/SKILL.md description: fixed-here (shortened to <= 1024 chars, trigger phrases first)", "every harness that loads the bundle's skill descriptions (Claude Code, Codex, Cursor): out-of-scope (they read the same YAML field unchanged; a shorter description is a strict subset of what they accept, and no generated packaging embeds per-skill descriptions)", ".github/workflows/ci.yml: out-of-scope (the existing `skillslint` job already runs the tool; the new rule rides it with no workflow edit)"]
exec-tier: strong
exec-tier-why: >-
  (a): rewriting `pr-review-desk`'s and `install`'s descriptions is trigger-text judgement —
  which phrases a harness must still match on after the cut is a design call the facts do
  not pre-specify. The lint half alone would be `any`.
---

# Brief 17 — Skill frontmatter conformance lint

## Context
files: `tools/skillslint/conformance.go` (planned), `tools/skillslint/conformance_test.go` (planned),
`tools/skillslint/testdata/conformance/**` (planned), `tools/skillslint/main.go`,
`tools/skillslint/README.md`, `plugins/assay/skills/install/SKILL.md`,
`plugins/assay/skills/pr-review-desk/SKILL.md`,
`changelog/harness-portability-17-skill-frontmatter-conformance-lint.md` (planned)
facts:
- limits (dated 2026-09-23, see `sources:`): description <= 1024 Unicode chars (agentskills + Codex
  `MAX_CATALOG_SKILL_DESCRIPTION_CHARS`); name 1-64 chars, `^[a-z0-9]+(-[a-z0-9]+)*$`, equals
  its directory (agentskills). Soft: body < 5000 tokens, < 500 lines (agentskills); Codex
  truncates an agent-plugin skill body at 8000 bytes; Codex's skills-list budget is 2% of the
  context window in tokens, or 8000 chars when the window is unknown.
- current bundle @ e284ba9b8: `install` 1103 chars, `pr-review-desk` 1187; total 10202 chars
  over 14 skills. Re-measure: YAML-load each `plugins/assay/skills/*/SKILL.md` frontmatter and
  count code points of the trimmed `description`.
- skillslint today (`lint.go` LintSkills) already does a strict yaml.v3 load, name==dir and
  non-empty description; `--root` names a repo root and the glob `plugins/assay/skills/*/SKILL.md`
  is FIXED, so it cannot read an adopter's own skills directory. Exit codes: 0 clean,
  1 violation, 2 could-not-check. Advisory output is a `skillslint: NOTICE:` line on stderr.
- CI: the `skillslint` job in `.github/workflows/ci.yml` runs the tool from source on every PR.
- sibling: `harness-portability/16` (Codex long-context compaction cap) is a separate Codex-limit
  brief; no shared file.

**Budget ruling (recorded here, reversible).** The bundle-wide description budget is a
**NOTICE, not a failure**: the Codex 8000-char figure applies only when the context window is
unknown (a known window gets 2% of it in tokens — ~4000 tokens, ~16000 bytes, for a
200k window), and when it binds Codex
degrades by shortening descriptions, not by refusing. Cutting ~2200 chars from 14 trigger texts
to satisfy a fallback path would harm triggering on every harness. The hard failures are the
per-skill limits, where a harness truncates or refuses one skill.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Conformance rule (exit-code bearing)** in a new `conformance.go`, called for every skill
   file `LintSkills` reads, each finding an `Issue` (so exit 1):
   - description longer than 1024 chars, counted with `utf8.RuneCountInString` on the trimmed
     YAML-loaded value — never `len()` (bytes);
   - name longer than 64 chars, or not matching `^[a-z0-9]+(-[a-z0-9]+)*$`;
   - keep the existing name==dir and strict-YAML checks as they are (do not duplicate them).
   The message names the limit, the measured length and the source (`agentskills / Codex`).
2. **Advisory NOTICEs** (stderr, never move the exit code), at most one line per skill listing
   every threshold it crosses: body > 8000 bytes (Codex plugin-body truncation), > 500 lines,
   > 5000 approx tokens (bytes / 4, Codex's `APPROX_BYTES_PER_TOKEN`). One bundle line when the
   summed description chars exceed 8000 (the Codex unknown-window list budget), saying the
   rendered list line (name + locator) costs more than the description alone.
3. **Adopter reach**: add a repeatable `--skills-dir <dir>` flag. When given, skillslint runs
   ONLY the structural + conformance checks over `<dir>/*/SKILL.md` (the other halves check
   this repo's own tree and do not apply to an adopter's directory) and exits 0/1/2 as today;
   zero matched files is exit 2. Without the flag, behaviour over `--root` is unchanged.
4. **Shorten** the `install` and `pr-review-desk` descriptions to <= 1024 chars each. Keep the
   trigger phrases and quoted user asks FIRST (truncation cuts the tail); cut mechanism detail
   the body already carries. Keep the folded `>-` form. Do not touch other descriptions.
5. Fixtures under `testdata/conformance/`: `desc-1025` (ASCII, 1025 chars), `desc-1024-multibyte`
   (1024 chars, >= 2048 bytes), `name-mismatch`, `name-pattern` (`Bad--Name`), `budget-over`
   (9 valid skills, summed descriptions > 8000). Tests drive `LintSkills` / the flag path.
6. README: add the conformance rule to "What it checks" with the limits table and the budget
   ruling above. Changelog fragment with one `### Added` bullet.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/desc-1025` | exit 1; stderr contains `1025` and `1024` |
| 2 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/desc-1024-multibyte` | exit 0 (chars, not bytes, are counted) |
| 3 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/name-mismatch` | exit 1; stderr contains `!= directory` |
| 4 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/name-pattern` | exit 1; stderr names the name pattern |
| 5 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/budget-over` | exit 0; stderr contains `NOTICE` and `8000` (budget is advisory) |
| 6 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --root ../..` | exit 0 on the real bundle after Task 4; no `install` or `pr-review-desk` description Issue |
| 7 | `(cd tools/skillslint && go build -o /tmp/skillslint-hp17 .) && rm -rf /tmp/hp17-mut && mkdir -p /tmp/hp17-mut/install && git show e284ba9b8:plugins/assay/skills/install/SKILL.md > /tmp/hp17-mut/install/SKILL.md && /tmp/skillslint-hp17 --skills-dir /tmp/hp17-mut` | exit 1; stderr contains `1103` (mutation: the pre-fix description reddens the rule) |
| 8 | `cd tools/skillslint && go test ./... -count=1` | exit 0 |
| 9 | Live Codex probe, in a Codex CLI session with the `assay@assay` plugin installed: `codex exec 'Use the pr-review-desk skill. Quote verbatim the LAST level-2 heading of its SKILL.md, and say whether you were told the skill was truncated.'` | Evidence records which holds: the last heading is quoted (plugin body NOT truncated at 8000 bytes) or the reply reports the truncation warning (it IS). `BLOCKED (needs live Codex)` is a legitimate record; a truncation result is filed as a follow-up, not fixed here |
| 10 | `curl -fsSL -o /tmp/hp17-render.rs https://raw.githubusercontent.com/openai/codex/30fc6864cc1318121eca1843c217fe00ce1212f1/codex-rs/ext/skills/src/render.rs && grep -c -e 'MAX_CATALOG_SKILL_DESCRIPTION_CHARS: usize = 1_024' -e 'MAX_SKILL_PROMPT_BYTES: usize = 8_000' -e 'DEFAULT_SKILL_METADATA_CHAR_BUDGET: usize = 8_000' /tmp/hp17-render.rs` | exit 0; prints `3` (the cited limits resolve at the pinned SHA) |
| 11 | `gh api 'repos/medici-finance/assay/commits/main/check-runs?check_name=skillslint' --jq '.check_runs[0].conclusion'` (after merge) | prints `success` |
| 12 | `statusgen --consumers --brief harness-portability/17 --root . --base "$(git merge-base origin/main HEAD)"` | exit 0; no routing claim disproved (the implementation flips its own `follow-up` entries to `fixed-here`) |
| 13 | `statusgen --lint --root .` (built from this repo's `statusgen/`, not an older `PATH` binary) | exit 0; no PROBLEM line naming this brief |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Length counted in bytes, so a multi-byte description under 1024 chars fails | row 2 |
| Rule ships but never fires on the real bundle's pre-fix text | row 7 |
| Descriptions shortened by cutting the trigger phrases, not the mechanism tail | review-only — trigger adequacy is judgement; reviewer compares the first 300 chars before/after |
| Budget wired as a failure, reddening CI on every adopter bundle over 8000 | row 5 |
| `--skills-dir` silently passes on an empty directory | review-only — Task 3 states exit 2; the reviewer checks the zero-match branch has a test |
| Cited Codex limits were misread or moved upstream | row 10 |

## Evidence

<!-- appended at implementation/verification time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this section filled by someone who did NOT implement.
     Row 9 needs a live Codex install; record BLOCKED with the reason if none exists. -->

### Implementation-time run (implementer, sonnet-5-worker, 2026-09-24, darwin, non-hermetic)

Recorded at `implemented`; per-brief-rule this is NOT the verified run — a non-implementer
must re-run every row to flip verified.

| # | Command | Exit | Output |
|---|---------|------|--------|
| 1 | `.../skillslint-hp17 --skills-dir testdata/conformance/desc-1025` | 1 | `skillslint: skill/SKILL.md: description is 1025 characters, over the 1024-character hard limit (...)` |
| 2 | `.../skillslint-hp17 --skills-dir testdata/conformance/desc-1024-multibyte` | 0 | `SKILLSLINT: PASS — 1 skill file(s) under --skills-dir, structural + conformance checks clean` |
| 3 | `.../skillslint-hp17 --skills-dir testdata/conformance/name-mismatch` | 1 | `skillslint: the-desk/SKILL.md: frontmatter name "not-the-desk" != directory "the-desk" — ...` |
| 4 | `.../skillslint-hp17 --skills-dir testdata/conformance/name-pattern` | 1 | `skillslint: Bad--Name/SKILL.md: name "Bad--Name" does not match the agentskills name pattern ...` |
| 5 | `.../skillslint-hp17 --skills-dir testdata/conformance/budget-over` | 0 | `SKILLSLINT: PASS — 9 skill file(s) ...`; `skillslint: NOTICE: bundle: 9 skill(s), summed description characters 8100 exceeds the 8000-character budget ...` |
| 6 | `cd tools/skillslint && go run . --root ../..` (built binary) | 0 | `SKILLSLINT: PASS — 14 skill file(s) under ../.., ...`; no `install`/`pr-review-desk` description Issue in the FAIL lines; bundle NOTICE fires (9914 > 8000, expected/advisory) |
| 7 | mutation: `git show e284ba9b8:plugins/assay/skills/install/SKILL.md` copied to a scratch `--skills-dir` tree | 1 | `skillslint: install/SKILL.md: description is 1103 characters, over the 1024-character hard limit (...)` — matches the brief's own pre-fix measurement exactly |
| 8 | `cd tools/skillslint && go test ./... -count=1` | 0 | `ok  	github.com/medici-finance/assay/tools/skillslint	...` |
| 9 | Live Codex probe | — | **BLOCKED (needs live Codex)** — no Codex CLI / `assay@assay` plugin install available in this environment |
| 10 | `curl ... render.rs@30fc6864... \| grep -c ...` | 0 | `3` — all three cited Codex constants resolve at the pinned SHA |
| 11 | `gh api .../check-runs?check_name=skillslint` | — | Not yet applicable: no merge has happened; run post-merge |
| 12 | `statusgen --consumers --brief harness-portability/17 --root . --base $(git merge-base origin/main HEAD)` (built from this branch's `statusgen/`) | 0 | `summary: 3 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` — the three brief-authored entries CORROBORATED (fixed-here), 0 DISPROVED |
| 13 | `statusgen --lint --root .` (built from this branch's `statusgen/`) | 0 | `LINT: PASS`; 0 `PROBLEM` lines; none naming `harness-portability/17` |

Pre-mortem/detection-map failure modes (byte-vs-rune, rule-never-fires-on-real-bundle,
budget-wired-as-failure, empty-skills-dir) are each caught by the rows above (2, 7, 5)
and by `TestLintSkillsDir_EmptyDirFailsClosed` (unit-level, exit-2 fail-closed path).
### Verification re-run (non-implementer verifier, 2026-10-01, darwin/arm64, go1.27.1) — merged main @ 024c87b01aba8f6c7dd7ccd939e647a9b936be09

What moved since the last run: main advanced to 024c87b01aba (implementing squash commit aa1cc0e36, PR #1663, unchanged); this brief's Verify table is unchanged; blocker #1657 (self-referential `git merge-base origin/main HEAD` BASE in row 12) is still OPEN with no activity since 2026-09-30. Every row was re-run on current main; the result is again 11/13.

Every row was run from the repo root of a detached worktree at the merged sha. `statusgen` for rows 12 and 13 was built from this repo's `statusgen/` (the `--version` is `dev`, not an older PATH binary). Per the desk rule it was called with an absolute `--root` spelled as `"$(git rev-parse --show-toplevel)"`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/desc-1025` | Verify row 1: exit 1; stderr contains `1025` and `1024` | exit 1; stderr: `skillslint: skill/SKILL.md: description is 1025 characters, over the 1024-character hard limit (agentskills spec description; Codex MAX_CATALOG_SKILL_DESCRIPTION_CHARS ...)`, and `SKILLSLINT: FAIL — 1 issue(s) across 1 skill file(s)`. Re-run with stdout discarded: both tokens are on stderr | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/desc-1024-multibyte` | Verify row 2: exit 0 (chars, not bytes) | exit 0; `SKILLSLINT: PASS — 1 skill file(s) under --skills-dir, structural + conformance checks clean`. An independent YAML load of the fixture's description gave 1024 code points and 2732 UTF-8 bytes, so the check counts characters, not bytes | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/name-mismatch` | Verify row 3: exit 1; stderr contains `!= directory` | exit 1; stderr: `skillslint: the-desk/SKILL.md: frontmatter name "not-the-desk" != directory "the-desk" — a skill must declare the id it is invoked under` | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/name-pattern` | Verify row 4: exit 1; stderr names the name pattern | exit 1; stderr: `skillslint: Bad--Name/SKILL.md: name "Bad--Name" does not match the agentskills name pattern ^[a-z0-9]+(-[a-z0-9]+)*$ (lowercase letters/digits, hyphen-separated, no leading/trailing/consecutive hyphen)` | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --skills-dir testdata/conformance/budget-over` | Verify row 5: exit 0; stderr contains `NOTICE` and `8000` | exit 0; `SKILLSLINT: PASS — 9 skill file(s)`; stderr: `skillslint: NOTICE: bundle: 9 skill(s), summed description characters 8100 exceeds the 8000-character budget (Codex's skills-list budget when the model's context window is unknown) ...`, and `CONFORMANCE-BUDGET: NOTICE — ... (advisory: does not affect exit)` | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd tools/skillslint && go build -o /tmp/skillslint-hp17 . && /tmp/skillslint-hp17 --root ../..` | Verify row 6: exit 0 on real bundle; no `install` / `pr-review-desk` description Issue | exit 0; `SKILLSLINT: PASS — 14 skill file(s) under ../.., all frontmatter loads as a YAML mapping ...`. A grep for a `(install or pr-review-desk)/SKILL.md: description` line in the output returned no match (grep rc=1). Only advisory NOTICEs appeared: per-skill soft body budgets, and the bundle description sum of 9581 > 8000. the other skillslint sections (hidden-chars, guardrails, enforcement-block, posix-token, values) all PASS | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `(cd tools/skillslint && go build -o /tmp/skillslint-hp17 .) && rm -rf /tmp/hp17-mut && mkdir -p /tmp/hp17-mut/install && git show e284ba9b8:plugins/assay/skills/install/SKILL.md > /tmp/hp17-mut/install/SKILL.md && /tmp/skillslint-hp17 --skills-dir /tmp/hp17-mut` | Verify row 7: exit 1; stderr contains `1103` | exit 1; stderr: `skillslint: install/SKILL.md: description is 1103 characters, over the 1024-character hard limit ...`, and `SKILLSLINT: FAIL — 1 issue(s) across 1 skill file(s)`. The mutation turns the rule red | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `cd tools/skillslint && go test ./... -count=1` then `cd tools/skillslint && go test ./... -count=1 -v` | Verify row 8: exit 0 | exit 0; `ok  github.com/medici-finance/assay/tools/skillslint 0.726s`. The `-v` re-run also exited 0, with 116 `--- PASS`, 0 `--- SKIP` and 0 `--- FAIL` | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `codex exec 'Use the pr-review-desk skill. Quote verbatim the LAST level-2 heading of its SKILL.md, and say whether you were told the skill was truncated.'` | Verify row 9: record whether the plugin body is truncated at 8000 bytes; `BLOCKED (needs live Codex)` is a legitimate record | BLOCKED (needs live Codex). The offline-envelope filter excluded this row: `codex exec` is a live, billed call to an external model API, which this verifier pass may not make. A Codex CLI binary is present on the runner, so this is a policy exclusion, not a missing toolchain. The truncation question stays open (could-not-check) | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `curl -fsSL -o /tmp/hp17-render.rs https://raw.githubusercontent.com/openai/codex/30fc6864cc1318121eca1843c217fe00ce1212f1/codex-rs/ext/skills/src/render.rs && grep -c -e 'MAX_CATALOG_SKILL_DESCRIPTION_CHARS: usize = 1_024' -e 'MAX_SKILL_PROMPT_BYTES: usize = 8_000' -e 'DEFAULT_SKILL_METADATA_CHAR_BUDGET: usize = 8_000' /tmp/hp17-render.rs` | Verify row 10: exit 0; prints `3` | exit 0; printed `3`. All three cited Codex constants resolve at the pinned upstream SHA | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | `gh api 'repos/medici-finance/assay/commits/main/check-runs?check_name=skillslint' --jq '.check_runs[0].conclusion'` | Verify row 11: prints `success` | exit 0; printed `success`. That check run's head_sha is 024c87b01aba8f6c7dd7ccd939e647a9b936be09 (the merged sha under test), completed 2026-09-30T22:33:34Z. Read-only call | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12a | As authored, with the brief's `--root .` swapped for an absolute root: `go build -C statusgen -o /tmp/statusgen-hp17 . && /tmp/statusgen-hp17 --consumers --brief harness-portability/17 --root "$(git rev-parse --show-toplevel)" --base "$(git merge-base origin/main HEAD)"` | Verify row 12: exit 0; no routing claim disproved | exit 2; `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:harness-portability:17 is not in the diff against 024c87b01aba8f6c7dd7ccd939e647a9b936be09, so this run carries no evidence about its claims`. This is the self-referential BASE defect tracked in #1657: on merged main, `git merge-base origin/main HEAD` resolves to HEAD, so the diff is empty | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12b | Corrected, anchored to the implementing commit's own parent: `git checkout --detach aa1cc0e36 && /tmp/statusgen-hp17 --consumers --brief harness-portability/17 --root "$(git rev-parse --show-toplevel)" --base "$(git rev-parse aa1cc0e36^)" && git checkout --detach 024c87b01aba8f6c7dd7ccd939e647a9b936be09` | Verify row 12: exit 0; no routing claim disproved | exit 0; base be4380ce5a820c806d60d8ca311d85a99bd9dca9; `summary: 3 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. CORROBORATED: tools/skillslint, plugins/assay/skills/install/SKILL.md description, and plugins/assay/skills/pr-review-desk/SKILL.md description. UNCHECKED: the two out-of-scope entries. This passes only in corrected form; the as-authored row (12a) cannot run on merged main until #1657 fixes the row | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | `go build -C statusgen -o /tmp/statusgen-hp17 . && /tmp/statusgen-hp17 --lint --root "$(git rev-parse --show-toplevel)"` | Verify row 13: exit 0; no PROBLEM line naming this brief | exit 0; `LINT: PASS`. The output has 0 lines starting with `PROBLEM`, and 0 lines of any kind name harness-portability/17 or the brief-17 file | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Scope traceability.** Each row above discharges the Verify row with the same number; 12a and 12b both discharge Verify row 12. No verified work falls outside a Verify row. The only extra checks were the byte/char measurement in row 2, the stderr-only re-runs in rows 1 and 3–5, and the `-v` re-run in row 8. Each one strengthens its own row.

**Findings.**
- Verify row 12 as authored cannot be satisfied on merged main. It is one of the 13 briefs listed in #1657, which is still OPEN and has had no activity since 2026-09-30. The corrected anchor (`aa1cc0e36^`) shows the claims themselves hold: 3 corroborated, 0 disproved. The fix belongs in the brief's Verify row, per #1657.
- Verify row 9 needs a live Codex session with the plugin installed. A Codex CLI is present on this runner, but calling it is a billed external model call, which is outside this pass's offline envelope. A driver-run probe (or a sanctioned Codex run) is needed to close it.

**Risk-bearing value.** The brief's risk metadata is all `no` (`irreversible: no`, gate `model`) and the diff touches no risk-classed path. Enumeration still ran over every literal that `tools/skillslint/conformance.go` introduces (commit aa1cc0e36):
- `maxDescriptionChars = 1024` @ tools/skillslint/conformance.go:69 (hard, exit 1)
- `maxNameChars = 64` @ tools/skillslint/conformance.go:71 (hard)
- `namePattern = ^[a-z0-9]+(-[a-z0-9]+)*$` @ tools/skillslint/conformance.go:76 (hard)
- `bodyByteBudget = 8000` @ :83, `bodyLineBudget = 500` @ :85, `bodyTokenBudget = 5000` @ :88, `approxBytesPerToken = 4` @ :92, `bundleDescriptionBudget = 8000` @ :99 (all advisory NOTICE-only)

Ranking: every value is a lint threshold, so a wrong value is undone by an edit and a re-release. None of them is irreversible. The advisory budgets never move the exit code and rank last; they need no derivation. The top-ranked values are the hard limits, because a wrong limit either lets a truncated or refused skill ship green, or reddens CI on valid adopter skills.
- RISK-VALUE: DERIVED — maxDescriptionChars = 1024 @ tools/skillslint/conformance.go:69. The agentskills specification (fetched 2026-10-01) states the description is "Max 1024 characters". Codex's `MAX_CATALOG_SKILL_DESCRIPTION_CHARS: usize = 1_024` resolves at the pinned upstream SHA (row 10). The check counts code points with `utf8.RuneCountInString` (conformance.go:106), matching the spec's character unit (row 2).
- RISK-VALUE: DERIVED — maxNameChars = 64 @ tools/skillslint/conformance.go:71. The agentskills specification (fetched 2026-10-01) states "Max 64 characters" / "1-64 characters" for `name`. The name pattern at :76 encodes the same spec's rule: lowercase alphanumerics, hyphen-separated, no leading/trailing/consecutive hyphen (row 4).

rows_passed=11 rows_total=13 (row 9: could-not-check, excluded by the offline envelope; row 12: could-not-check as authored, #1657 — passes only in the corrected form 12b)

RISK-VALUE: DERIVED — maxDescriptionChars = 1024 @ tools/skillslint/conformance.go:69 and maxNameChars = 64 @ tools/skillslint/conformance.go:71 (agentskills spec + Codex constant at pinned SHA); advisory budgets rank last, no derivation needed

VERIFY: BLOCKED — blocker #1657 (Verify row 12's self-referential BASE); row 9 needs a live Codex session

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table and
answers: are the shortened descriptions' opening sentences still the trigger text a harness
matches on?
