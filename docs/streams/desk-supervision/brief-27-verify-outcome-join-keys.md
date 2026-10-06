---
brief: assay:assay:desk-supervision:27
title: Verify-outcome records carry join keys — delivering PRs, verifier tier and timing, per-row results — and every failure a blocker kind
why: >-
  A verify outcome records which brief failed and at which commit, but not which pull request
  delivered the work, so no one can connect a verify failure to the review that approved it or
  the CI run that tested it. "Do PRs that needed many review rounds fail verify more often?" and
  "did review approve something verify then caught?" cannot be answered today; in this repo's
  register zero records carry a PR reference. Separately, 18 failure records (16 verify-fail,
  2 blocked) carry no blocker kind, so they can never be routed to the right next actor or
  counted by cause. Recording both at write time costs one lookup the verifier already makes;
  reconstructing them later from PR-body trailers is lossy for multi-PR, cross-repo and
  re-homed briefs. The same schema change adds the verifier tier and run timing (cost and
  latency per verification), an optional per-row result list (today `rows_passed` cannot tell a
  failed row from one that could not run), and an optional dispatch reference that joins the
  record to the desk audit log. These are the join keys every later analysis of the pipeline needs.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (research on decision models and pipeline analytics)
exec-tier: strong
exec-tier-why: >-
  (b) correctness spans the record schema doc, the deskkit receipt type, the deskevidence writer's
  refusal path, and the verify-desk skill that composes the record; a writer that accepts a PR
  reference it never dereferenced survives a happy-path test.
domain: complicated
outcome: none
sources:
  - "docs/streams/desk-supervision/verify-wake-v1.md — the receipt schema this brief extends (fields table; blocker kinds closed set)"
  - "docs/streams/desk-supervision/brief-24-per-file-verify-outcomes.md — the per-file register and the writer (`deskevidence --outcome-record`) this brief extends"
  - "register read 2026-10-06 @ 1fbf1153f: 289 records (legacy log + per-file register); 0 carry any PR key; verify-fail records: 71 with a verify-wake-v1 receipt, 16 legacy-shape with no blocker_kind; blocked: 150 with a receipt, 2 legacy-shape"
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models and statistical learning to the pipeline concluded the verify register cannot yet be joined to review or CI data, and that join keys must be captured at write time before any model is worth fitting"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): no `prs`/`pr`/`merge_sha` key in deskkit.WakeReceipt (tools/desk/internal/deskkit/verifywake.go:70-95); `--outcome-record` validates receipts only when wake_schema is present (tools/desk/cmd/deskevidence/outcomerecord.go:151-162), so a legacy-shape verify-fail lands unvalidated"
consumers:
  - "docs/streams/desk-supervision/verify-wake-v1.md (fields table, Migration section): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/verifywake.go (WakeReceipt gains the `prs` field): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskevidence/outcomerecord.go and receiptvalidate.go (the refusals and the forge dereference): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the outcome-record composition paragraphs): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (deskevidence section): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/verify-outcomes/README.md (the register README's 'optional receipt' wording): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/verifyoutcomes.go (statusgen's own reader copy): out-of-scope (it decodes only `brief`/`ts`/`outcome`/`sha` and ignores additive keys, so a record carrying `prs` reads exactly as before — Verify row 6 proves it)"
  - "tools/desk/cmd/verifyloop (wake evaluation): out-of-scope (the wake evaluator reads inputs, predicate and blocker; `prs` is not a wake input, and the stricter failure requirement only removes legacy-shape records it already treats as unclassified)"
  - "existing records in the register: out-of-scope (records are immutable by design; no backfill — older records stay legacy-shape and read as they do today)"
---

# Brief 27 — Verify-outcome records carry join keys — delivering PRs, verifier tier and timing, per-row results — and every failure a blocker kind

## Context
files:
- **edit** `docs/streams/desk-supervision/verify-wake-v1.md` — add the `prs` field row and the
  failure-record requirement; note in Migration that older records are not backfilled.
- **edit** `tools/desk/internal/deskkit/verifywake.go` — `WakeReceipt` gains
  `PRs []DeliveredPR \`json:"prs,omitempty"\`` with `DeliveredPR{Repo string; Number int; MergeSHA string}`.
- **edit** `tools/desk/cmd/deskevidence/outcomerecord.go`, `receiptvalidate.go` + tests — the two
  refusals and the forge dereference (Task 2–3).
- **edit** `plugins/assay/skills/verify-desk/SKILL.md` — the outcome-record paragraphs (around the
  "Since #882, `deskevidence --outcome-record` REFUSES" paragraph and the FAIL-path paragraph).
- **edit** `tools/desk/README.md` — the deskevidence section.
- **edit** `docs/streams/verify-outcomes/README.md` — the register README calls the receipt fields "optional"; amend to "required on verify-fail / blocked records" and point to verify-wake-v1.md for the field list instead of restating it.
- **add** `changelog/desk-supervision-27.md` (planned).

facts:
- Writer: `deskevidence <owner/repo> <branch> --outcome-record <file>`, `cmdOutcomeRecordWrite`
  in `tools/desk/cmd/deskevidence/outcomerecord.go`. It runs `validateReceipt` ONLY when
  `wake_schema == "verify-wake-v1"` (lines 151-162 @ 1fbf1153f); a record without it lands unvalidated.
- `deskkit.WakeReceipt.Complete()` (`tools/desk/internal/deskkit/verifywake.go:123`) is the existing
  completeness test; `blocker_kind` is one of a closed set (`implementation`, `check-definition`,
  `human-action`, `environment`, `unknown`).
- The brief → PR mapping already exists as a convention: a delivering PR carries a
  `Brief: <stream>/<NN>` trailer in its body; statusgen resolves it via `PRsNamingBrief`
  (`statusgen/autoflip.go:218-222`, used by `resolveByTrailer` at :825). The verifier uses the same
  convention to find candidate PRs; the writer does not search, it only checks what the record names.
- Records are immutable; a correction is a new record. Readers decode only the keys they know, so
  an additive key is backward compatible (verify-wake-v1.md §"Where it lives").
- Refusal convention: `deskkit.Refused(...)` → exit 5, naming the field.
- single-point-of-failure: the writer's forge dereference (Task 3) — backed by the review gate on
  the Evidence PR, which shows the record's `prs` beside the brief's `Brief:`-trailer PRs.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Schema.** Add to verify-wake-v1.md's fields table: `prs` — list of `{repo, number, merge_sha}`,
   the merged PRs that delivered the brief's deliverables (an Evidence-only PR is never one). Add the
   matching `DeliveredPR` type and `PRs` field to `WakeReceipt`. The field is additive; no existing
   reader changes.
2. **Failure records must be complete receipts.** In `cmdOutcomeRecordWrite`, refuse (exit 5) a
   record whose `outcome` is `verify-fail` or `blocked` unless it carries
   `wake_schema: verify-wake-v1` and `WakeReceipt.Complete()` is true. The refusal names the missing
   field (`wake_schema` or `blocker_kind`). `verified` records keep today's behaviour.
3. **`prs` is required on every new verify-wake-v1 record, and dereferenced.** In
   `validateReceipt`, refuse a record whose `prs` is empty, or whose entries fail any of: the PR
   exists on the configured forge; it is merged; its merge commit equals `merge_sha`; `merge_sha` is
   an ancestor of (or equal to) the record's `sha`. Use the forge reads the validator already holds;
   a forge read error is `Unverifiable` (exit 6), never a pass. An explicit empty list is allowed
   only with a non-empty `prs_unknown_reason` string (for a brief delivered outside any PR); record
   that key in the schema too.
4. **Skill.** In verify-desk's outcome-record paragraphs, tell the verifier to fill `prs` from the
   brief's `Brief:`-trailer PRs (excluding Evidence-only PRs), each with its merge commit, and that a
   FAIL or BLOCKED outcome always writes a full verify-wake-v1 receipt.
5. **Run metadata and per-row results.** Add to the schema and `WakeReceipt`:
   - `verifier_tier` (REQUIRED on new verify-wake-v1 records) — the tier slug of the model that ran
     the verification (`any` / `strong`, the brief-frontmatter vocabulary), never a vendor model name;
   - `started`, `finished` (REQUIRED, RFC3339; `started` <= `finished` <= `ts`, else refused);
   - `row_results` (OPTIONAL) — list of `{row, result}` with `result` in the closed set
     `pass` / `fail` / `could-not-run` / `deferred`; when present, refuse a list whose `pass` count
     disagrees with `rows_passed` or whose length disagrees with `rows_total`;
   - `dispatch_ref` (OPTIONAL) — the dispatch claim id or session tag the verification ran under,
     the key the desk audit log already carries.
   Free text is never a value of any of these fields.
6. **Docs + changelog.** Update `tools/desk/README.md`'s deskevidence section; amend `docs/streams/verify-outcomes/README.md` (receipt required on failure records; link verify-wake-v1.md for fields rather than listing them); add the changelog
   fragment (bullets: the new keys, the stricter failure record).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test ./tools/desk/cmd/deskevidence/... ./tools/desk/internal/deskkit/...` | exit 0 | check:ci |
| 2 | `go test ./tools/desk/cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v > "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_FailWithoutReceipt.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt' "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_FailWithoutReceipt.out"` | exit 0; output contains `refused` and `blocker_kind` (a legacy-shape `verify-fail` record is refused, naming the field) | check:ci +mutation |
| 3 | `go test ./tools/desk/cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v > "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_PRsDereference.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_PRsDereference' "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_PRsDereference.out"` | exit 0; subtests cover: PR not found → refused; PR open (unmerged) → refused; `merge_sha` mismatch → refused; `merge_sha` not an ancestor of `sha` → refused; forge read error → exit 6 unverifiable; valid PR → accepted | check:ci +mutation |
| 4 | `go test ./tools/desk/cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedLegacyUnchanged$' -v > "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_VerifiedLegacyUnchanged.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_VerifiedLegacyUnchanged' "${TMPDIR:-/tmp}/b27-TestOutcomeRecord_VerifiedLegacyUnchanged.out"` | exit 0 (a legacy-shape `verified` record still lands — neighbouring behaviour untouched) | check:ci +neighbour |
| 5 | `grep -n 'prs_unknown_reason' docs/streams/desk-supervision/verify-wake-v1.md tools/desk/internal/deskkit/verifywake.go plugins/assay/skills/verify-desk/SKILL.md` | exit 0; one or more hits in each of the three files | check |
| 6 | `go test ./statusgen/ -run '^TestVerifyOutcomes_AdditivePRsKey$' -v > "${TMPDIR:-/tmp}/b27-sg.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditivePRsKey' "${TMPDIR:-/tmp}/b27-sg.out"` | exit 0; the test reads a record that carries `prs` and deriving the same latest outcome as without it (statusgen's reader ignores the additive key) | check:ci +flow |
| 7 | `go test ./tools/desk/cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v > "${TMPDIR:-/tmp}/b27-meta.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_RunMetadata' "${TMPDIR:-/tmp}/b27-meta.out"` | exit 0; subtests: missing `verifier_tier` → refused; a vendor model name as `verifier_tier` → refused; `started` after `finished` → refused; `row_results` pass count disagreeing with `rows_passed` → refused; unknown `result` value → refused; valid record with and without the optional fields → accepted | check:ci +mutation |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |
| 9 | `jq -r 'select(.prs) \| .prs[] \| "\(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json` (on main, after the first post-merge FAIL or BLOCKED record lands) | at least one line; for one printed `<number> <sha>` pair, `gh api repos/medici-finance/assay/pulls/<number> --jq .merge_commit_sha` prints that same `<sha>` and `.merged` is `true` | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Run metadata accepted as free text or inconsistent with the row counts | row 7 |
| Writer accepts any `prs` value without checking the forge (presence-only) | row 3 (each dereference case refuses) |
| Stricter failure rule also blocks `verified` records | row 4 |
| A reader somewhere chokes on the new key | row 6 (statusgen reader); deskkit reader is row 1 |
| Verifier skill never told to fill `prs`, so every fail is refused in production | row 5 (skill text) + row 9 (a real landed record) |
| Real records name the wrong PR (e.g. the Evidence PR) | row 9 dereferences one; Evidence-only exclusion stays review-only (needs a diff-shape judgement) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
