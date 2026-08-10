---
brief: methodology/23
title: INTAKE + FINDINGS become directories of per-entry files with a statusgen-generated view
wave: 0
depends: ["methodology/16"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by opus (author-brief, from INTAKE [I-21](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-intake-findings-become-directories-with-a-statusgen-generate.md))
sources: ["INTAKE [I-21](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-intake-findings-become-directories-with-a-statusgen-generate.md)", "issue #89 (register sequence-integrity origin)", "issue #156 (sibling: append-only shared surface + sequential IDs)", "methodology/16 (non-self-writable lifecycle — the integrity mechanism this extends)"]
---

# Brief 23 — INTAKE + FINDINGS become directories of per-entry files with a generated view

## Context
files:
- `../assay-toolkit/statusgen/registers.go` — `registerSequenceProblems` / `registerFileProblems` (the gap+dup check on `INTAKE.md` / `FINDINGS.md`; replaced here).
- `../assay-toolkit/statusgen/load.go` — `loadStreams` (iterates `docs/streams/*` dirs, each treated as a stream needing a README) and `parseFindings` (reads `FINDINGS.md` directly into `[]Finding`).
- `../assay-toolkit/statusgen/registers_test.go`, `../assay-toolkit/statusgen/load_test.go` — table-driven tests to mirror.
- `../assay-toolkit/statusgen/gitinfo.go` — existing git shell-out helper (pattern for the git-history integrity read).
- `../assay-toolkit/statusgen/main.go` — `run()` mode wiring (`write` / `check` / `lint` / `record`); where `registerSequenceProblems` is invoked and where the generated views must be produced/compared.
- `../oit/.github/workflows/status-regen.yml` — main-only single-writer CI; must also regenerate the register views.
- docs/streams/INTAKE.md, docs/streams/FINDINGS.md — today's hand-appended single-file registers (source data to migrate).
- `CLAUDE.md` — "Raw ideas land in docs/streams/INTAKE.md" / "New knowledge … goes in docs/streams/FINDINGS.md" instructions (§ Status tracking).

facts:
- register-head grammar: `registerHeadRe = ^## ([FI])-(\d+)\b` — one regex serves both registers via the `F`/`I` prefix.
- `Finding` fields parsed today: `ID, Date, Title, Affects[], Ack, Resolved` (`model.go`); `Affects:` / `Ack:` / `Resolved:` are line-prefixed keys in the body.
- INTAKE entries carry a `Disposition: new | watching | scoped → <stream> | rejected — <why>` line; no struct today (INTAKE is not parsed into Go, only sequence-checked).
- anti-falsification guarantee ([F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md)/[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) lineage, issue #89): a **gap** in `I-NN`/`F-NN` is how `--lint` catches a *silently deleted* entry. Dropping contiguous numbering REMOVES this unless the guarantee is re-homed.
- single-writer pattern already exists for `STATUS.md` (methodology/15): PRs run `--lint` (sources only, never read/write the artifact); main's CI regenerates and commits it. Mirror it exactly for the register views.
- current register state to migrate: INTAKE `I-01…I-23`, FINDINGS `F-01…F-<max>` (read the live files at implementation time for the exact F max).
- **Register-link backfill** (methodology/33): after any future register renumber or slug rename, re-run `go run ./tools/statusgen --register-links` to update all brief file references — the script is idempotent, reads entry `id:` frontmatter (never guesses slugs), and rewrites bare `F-NN`/`I-NN` tokens to linked form.

consumers of the register file paths / format (rule 6 — enumerate + route):
- `tools/statusgen` `parseFindings` + `registerSequenceProblems` + `applyFindings`/`check` (findings staleness) — **fixed in this brief** (read per-entry files; new integrity check).
- `tools/statusgen` `loadStreams` stream discovery — **fixed in this brief** (must NOT treat the register dirs as streams).
- `../oit/.github/workflows/status-regen.yml` — **fixed in this brief** (regen + commit the new views alongside STATUS.md).
- `CLAUDE.md` § Status tracking wording — **fixed in this brief**.
- `../oit/.claude/skills/author-brief/SKILL.md` (in-repo) closing-steps that say "append a `## F-NN` to FINDINGS.md / `## I-NN` to INTAKE.md" — **fixed in this brief** (point at the new add-an-entry-file flow).
- user-level desk skills (`the-desk`, `batch-fanout`, `verify-desk`, user-level `author-brief`) that instruct appending to these registers — **out of scope (follow-up)**: they live outside this repo; note the drift in the brief's Evidence and file an INTAKE follow-up so the desk updates them. Do not edit `~/.claude` from this brief.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or the generated `INTAKE.md` / `FINDINGS.md` views on a branch — they are single-writer artifacts (main CI only). `--lint` must not read or write them.
- Preserve every existing `I-NN` / `F-NN` reference elsewhere in the tree — migration keeps ids stable (see Task step 2).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
One design, applied to BOTH registers (INTAKE and FINDINGS share the machinery). Implement in this order; the phases are separable if the brief is split at pickup.

1. **Per-entry storage.** Introduce reserved register directories `docs/streams/intake/` and `docs/streams/findings/`, one file per entry (`<YYYY-MM-DD>-<slug>.md`), each with YAML frontmatter carrying the entry's data:
   - intake: `id`, `date`, `title`, `disposition` (`new|watching|scoped|rejected`), optional `scoped-to`/`why`; body = the prose paragraph.
   - findings: `id`, `date`, `title`, `affects: []`, `ack`, `resolved`; body = the finding text.
   `id` retains the historical `I-NN`/`F-NN` for migrated entries (keeps all existing cross-refs valid); NEW entries may use a collision-free token (e.g. author/session-prefixed) instead of racing a global counter — uniqueness, not contiguity, is required.
2. **Migrate existing entries.** Split today's `INTAKE.md` (`I-01…I-23`) and `FINDINGS.md` (`F-01…F-max`) into per-entry files, preserving each `id`, all fields, and body text verbatim. No entry dropped or altered in meaning.
3. **Parse from the directories.** Rewrite `parseFindings` to read `docs/streams/findings/*.md` (frontmatter → `Finding`), and add an intake parser. Sort deterministically (by `id`/date) so the generated view is stable.
4. **Exclude register dirs from stream discovery.** `loadStreams` must skip the reserved names `intake`/`findings` (they are registers, not streams) — otherwise it errors "no README.md" or mis-parses them as streams. Add a reserved-name set; cover it with a test.
5. **Generate the views.** `statusgen` (write mode + main CI) renders docs/streams/INTAKE.md and docs/streams/FINDINGS.md from the per-entry files — byte-deterministic, single-writer, same discipline as `STATUS.md`. `--check` verifies byte-match; `--lint` builds them in memory but never reads/writes the on-disk artifact. Wire `status-regen.yml` to regenerate + commit them.
6. **Re-home the anti-falsification guarantee (the load-bearing part).** Replace `registerSequenceProblems` (gap/dup on numbering) with a **file-presence-vs-git-history integrity check**: enumerate every register entry file that has ever existed on `main` (via `git log --diff-filter=A --name-only` or equivalent, reusing the `gitinfo.go` shell-out pattern) and flag any that is absent from the working tree — a deleted entry file is the tombstone-not-delete violation the old gap check caught. Withdrawal stays in-place (`disposition: rejected` / `resolved: yes`), never a file deletion. Keep a duplicate-`id` check (two files claiming one `id`).
7. **Update instructions.** `CLAUDE.md` § Status tracking and `../oit/.claude/skills/author-brief/SKILL.md` closing steps: replace "append a heading to INTAKE.md/FINDINGS.md" with "add an entry file under `docs/streams/{intake,findings}/`; the `.md` view is generated." Note the user-level desk-skill drift in Evidence + an INTAKE follow-up (do not edit `~/.claude`).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test -count=1 ./tools/statusgen/...` | exit 0 (new register-parse, view-gen, discovery-skip, and integrity tests pass) |
| 2 | `go vet ./tools/statusgen/...` | exit 0; no output |
| 3 | `statusgen --root . --lint` | exit 0; migrated tree is clean; NO `PROBLEM:`; does not read/write `STATUS.md` or the register views |
| 4 | `statusgen --root . && git status --porcelain -- docs/streams/INTAKE.md docs/streams/FINDINGS.md` | write-mode regenerates both views; a fresh regen leaves them byte-identical to committed (no diff) |
| 5 | `(rc=0; for t in Register Intake Finding; do out=$(go test ./statusgen/... -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` (exit 0, prints nothing — all three named test groups EXIST and pass) then inspect: a test deletes a migrated entry file from a fixture tree and asserts `--lint`/check reports a "register entry removed (tombstone-not-delete)" problem | exit 0 — the integrity check FIRES on a deleted entry file (the re-homed anti-falsification guarantee). Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite |
| 6 | `statusgen --root . && grep -c '^## I-' docs/streams/INTAKE.md && grep -c '^## F-' docs/streams/FINDINGS.md` | regenerated views contain every migrated `I-`/`F-` entry (counts ≥ pre-migration: I ≥ 23, F ≥ prior max) — round-trip loses nothing |
| 7 | `statusgen --root . --lint` after adding a throwaway second `docs/streams/findings/*.md` reusing an existing `id` | exit 1; duplicate-`id` problem reported |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Also record: (a) the exact F-NN max migrated, (b) confirmation that no
     STATUS.md / view artifact was committed on the branch, (c) the user-level
     desk-skill drift note + the INTAKE follow-up id filed for it.
     "verified" status in the stream README requires this section filled by
     someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test -count=1 ./tools/statusgen/...` | 0 | ok | 2026-07-10 | opus-verifier |
| 2 | `go vet ./tools/statusgen/...` | 0 | clean | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | 0 PROBLEM; views not written | 2026-07-10 | opus-verifier |
| 4 | write-mode + `git status` on views | 0 | INTAKE.md/FINDINGS.md byte-identical (no diff) | 2026-07-10 | opus-verifier |
| 5 | delete a tracked entry file → lint | 1 | `PROBLEM: register entry removed (tombstone-not-delete)` fires | 2026-07-10 | opus-verifier |
| 6 | regen + count headings | 0 | `## I-`=31 (≥23), `## F-`=22 | 2026-07-10 | opus-verifier |
| 7 | dup-id findings file → lint | 1 | `PROBLEM: findings register: duplicate id F-02` fires | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — registers are per-entry directories; tombstone-not-delete + dup-id guards fire. (All manual mutations reverted, tree clean.)

## Review
Gate: model (all four risk answers no — internal tooling + docs, fully revertible via git; no
regulatory/customer/irreversible/sensitive-data dimension). Reviewer must confirm the re-homed
integrity check (Verify #5) actually fires on a deleted entry file — that guarantee is the whole
reason the old contiguous-numbering rule existed ([F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md)/[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) lineage), and losing it silently is the
one way this brief can do harm. Also confirm the views stayed single-writer (no artifact committed on
the branch) and that migration dropped no entry (Verify #6).

