---
brief: assay:assay:desk-supervision:27
title: Verify-outcome records carry join keys — delivering PRs, verifier tier and timing, per-row results — and every failure a blocker kind
why: >-
  A verify outcome records which brief failed and at which commit, but not which pull request
  delivered the work, so no one can connect a verify failure to the review that approved it or
  the CI run that tested it. "Do PRs that needed many review rounds fail verify more often?" and
  "did review approve something verify then caught?" cannot be answered today; in this repo's
  register zero records carry a PR key. Separately, 18 failure records (16 verify-fail,
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
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (research on decision models and pipeline analytics)
exec-tier: strong
exec-tier-why: >-
  (b) correctness spans the forge adapters (a new merge-commit field), the record schema doc, the deskkit receipt type, the deskevidence writer's
  refusal path, and the verify-desk skill that composes the record; a writer that accepts a PR
  reference it never dereferenced, or writes a private reference into a public register, survives a happy-path test (c).
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
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go and their golden fixtures (PullRequest gains MergeCommitSHA): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/quality/brief-11-dora-join.md (already joins on PR number + merge SHA + stream/task id): out-of-scope (a later reader of these keys; it needs no change for them to land)"
  - "open draft PR #2239 (dispatcher-owned verifier attestation, also edits outcomerecord.go): out-of-scope (merge order: #2239 first; when merged, this brief sources verifier_tier and dispatch_ref from it — see facts)"
  - "existing records in the register: out-of-scope (records are immutable by design; no backfill — older records stay legacy-shape and read as they do today)"
---

# Brief 27 — Verify-outcome records carry join keys — delivering PRs, verifier tier and timing, per-row results — and every failure a blocker kind

## Context
files:
- **edit** `docs/streams/desk-supervision/verify-wake-v1.md` — the new fields, the failure-record
  requirement, the public-register redaction rule; Migration notes older records are not backfilled.
- **edit** `tools/desk/internal/deskkit/verifywake.go` — `WakeReceipt` gains `PRs []DeliveredPR`
  (`DeliveredPR{Repo string; Number int; MergeSHA string; Withheld string}`, all `omitempty`),
  `PRsUnknownReason`, `VerifierTier`, `Started`, `Finished`, `RowsPassed`, `RowsTotal`
  (`rows_passed`/`rows_total`, today read by no Go code), `RowResults`, `DispatchRef`,
  `DispatchRefDigest`.
- **edit** `tools/desk/internal/deskkit/forge.go` — `PullRequest` gains `MergeCommitSHA string`
  (`json:",omitempty"`, so existing golden fixtures stay byte-identical where it is empty).
- **edit** `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` — populate
  `MergeCommitSHA` in `GetPullRequest` (GitHub `merge_commit_sha`; GitLab `merge_commit_sha`, or
  `squash_commit_sha` when the change was squash-merged); refresh the affected fixtures under
  `tools/desk/internal/deskkit/testdata/forge_golden/` and `testdata/forge_gitlab_golden/`.
- **edit** `tools/desk/cmd/deskevidence/outcomerecord.go`, `receiptvalidate.go` + tests — the
  refusals, the forge dereference, the redaction (Tasks 2–5).
- **edit** `plugins/assay/skills/verify-desk/SKILL.md` — the outcome-record paragraphs (around the
  "Since #882, `deskevidence --outcome-record` REFUSES" paragraph and the FAIL-path paragraph).
- **edit** `tools/desk/README.md` — the deskevidence section.
- **edit** `docs/streams/verify-outcomes/README.md` — its "optional" receipt wording becomes
  "required on new records" and it links verify-wake-v1.md for fields instead of listing them.
- **add** `changelog/desk-supervision-27.md` (planned).

facts:
- Writer: `deskevidence <owner/repo> <branch> --outcome-record <file>`, `cmdOutcomeRecordWrite`
  in `tools/desk/cmd/deskevidence/outcomerecord.go`. It runs `validateReceipt` ONLY when
  `wake_schema == "verify-wake-v1"` (lines 151-162 @ 1fbf1153f); a record without it lands unvalidated.
  It already performs a repo-visibility read for the public-repo gate
  (`deskkit.ForgeRepoInfoFetcher`, lines 117-118) and the withheld-identifier outbound scan (line 112).
- `deskkit.WakeReceipt.Complete()` (`tools/desk/internal/deskkit/verifywake.go:123-141`) is the
  existing completeness test; it fails on a missing `wake_schema`, `receipt_id`, `blocker_kind`,
  `wake_predicate`, or a predicate-specific field. `blocker_kind` is a closed set.
- `deskkit.PullRequest` (`forge.go`, `type PullRequest struct`) has `HeadSHA`, `MergedAt`,
  `Merged`, but NO merge-commit field; `Forge.RepoVisibility` (`forge.go:1538-1539`) and
  `Forge.CompareRefs` exist.
- The brief → PR convention: a delivering PR carries a `Brief: <stream>/<NN>` trailer; statusgen
  resolves it via `PRsNamingBrief` (`statusgen/autoflip.go:218-222`, used at :825). The verifier
  uses it to find candidate PRs; the writer only checks what the record names.
- `dispatch_ref` is defined by desk-supervision/28 (separate draft PR) as the per-run id
  `<claim_key>@<YYYYMMDDTHHMMSSZ>`, readable as `git config --worktree assay.dispatchRef`. A claim
  key can carry a tracking repo's alias and issue number, so it is never written in clear to a
  public register.
- Go modules: this repo has no root `go.mod`/`go.work`; `tools/desk` and `statusgen` are separate
  modules, so every Go Verify row `cd`s into its module first (the sibling briefs 24-26 form).
- Records are immutable; a correction is a new record. Every outcome-record reader decodes with
  plain `json.Unmarshal` into a partial struct, so additive keys are backward compatible.
- Overlap: open draft PR #2239 adds a dispatcher-owned verifier attestation (selected tier and run)
  and edits `outcomerecord.go`. If it has merged when this brief is picked up, `verifier_tier` and
  `dispatch_ref` are taken from that attestation and a mismatching self-reported value is refused;
  if not, they are self-reported and validated only for shape.
- Refusal convention: `deskkit.Refused(...)` → exit 5 naming the field; a forge read failure is
  `deskkit.Unverifiable` → exit 6, never a pass.
- single-point-of-failure: the writer's validation (Tasks 2–5) — backed by the withheld-identifier
  outbound scan the writer already runs (independent, per-operator token map) and the review gate on
  the Evidence PR.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Schema.** Document every new field in verify-wake-v1.md's fields table and add it to
   `WakeReceipt`:
   - `prs` — list of `{repo, number, merge_sha}`: the merged PRs that delivered the brief's
     deliverables (an Evidence-only PR is never one); or, for a withheld entry, `{withheld}` (Task 4).
   - `prs_unknown_reason` — a short code from a closed set (`no-pr-delivery`), required exactly when
     `prs` is an explicit empty list.
   - `verifier_tier` — `any` / `strong` (the brief-frontmatter vocabulary), never a vendor model name.
   - `started`, `finished` — RFC3339; `started` <= `finished` <= `ts`.
   - `rows_passed`, `rows_total` — the existing keys, now typed in `WakeReceipt`.
   - `row_results` (optional) — list of `{row, result}`, `result` in `pass` / `fail` /
     `could-not-run` / `deferred`; it always covers ALL `rows_total` rows of the brief (including on
     a partial receipt with non-empty `rows`), so its length equals `rows_total` and its `pass`
     count equals `rows_passed`.
   - `dispatch_ref` (optional, private registers only) and `dispatch_ref_digest` (optional) — Task 4.
   Free text is never a value of any of these fields.
2. **Which records must carry what.**
   - A NEW `verify-fail` or `blocked` record must carry `wake_schema: verify-wake-v1` with
     `WakeReceipt.Complete()` true; else refuse (exit 5) naming the FIRST failing field.
   - Every NEW verify-wake-v1 record, whatever its `outcome` (including `verified`), must carry
     `prs` (or `prs: []` + `prs_unknown_reason`), `verifier_tier`, `started`, `finished`; else refuse.
   - A legacy-shape `verified` record (no `wake_schema`) still lands as today — UNLESS it carries
     `prs`, in which case `prs` is dereferenced exactly as in Task 3 (presence is never accepted
     unchecked, whatever `wake_schema` says).
3. **Dereference every clear `prs` entry** (each of `exists`, `merged`, `merge commit`, `ancestry`
   refuses with its own message; any forge read error is exit 6):
   - the PR exists on the entry's `repo` and is merged (`GetPullRequest`);
   - its `MergeCommitSHA` equals `merge_sha` (the new `PullRequest` field);
   - ancestry: when `repo` is the register's own repo, `merge_sha` is an ancestor of (or equal to) the
     record's `sha`; when `repo` is another repo, `merge_sha` is an ancestor of (or equal to) that
     repo's default-branch tip at write time (`CompareRefs`).
4. **Public-register redaction (the writer decides; the verifier never has to).** Read the
   register repo's visibility (`RepoVisibility`, the read the writer already makes). When the
   register is public:
   - a `prs` entry whose `repo` is not public is dereferenced (Task 3) and then written ONLY as
     `{withheld: sha256("<repo>#<number>@<merge_sha>")}` — the operator can join it on their side by
     hashing their own value; the clear `repo`/`number`/`merge_sha` never land in the file;
   - `dispatch_ref` is never written in clear: the writer replaces it with `dispatch_ref_digest`
     (`sha256` of the clear value); a record that arrives carrying only a clear `dispatch_ref` is
     rewritten, never refused, so the verifier's composition is identical on every register.
   On a private register both are written in clear. A `{withheld}` entry is accepted as satisfying
   "`prs` required".
5. **Skill.** In verify-desk's outcome-record paragraphs: fill `prs` from the brief's
   `Brief:`-trailer PRs (excluding Evidence-only PRs) with each merge commit; set `verifier_tier`,
   `started`, `finished`, and `dispatch_ref` from `git config --worktree assay.dispatchRef` when set;
   a FAIL or BLOCKED outcome always writes a full verify-wake-v1 receipt; the writer redacts for
   public registers, so write clear values.
6. **Docs + changelog.** Update `tools/desk/README.md`'s deskevidence section; amend
   `docs/streams/verify-outcomes/README.md` as listed in `files:`; add the changelog fragment
   (bullets: the new keys, the stricter records, the public-register redaction).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskevidence/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v > "${TMPDIR:-/tmp}/b27-r2.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/legacy_verify_fail_names_wake_schema' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_blocker_kind_names_blocker_kind' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_receipt_id_names_receipt_id' "${TMPDIR:-/tmp}/b27-r2.out" > "${TMPDIR:-/tmp}/b27-r2.hits" && test "$(wc -l < "${TMPDIR:-/tmp}/b27-r2.hits")" -eq 3` | exit 0 (each subtest asserts the refusal message names that first failing field) | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v > "${TMPDIR:-/tmp}/b27-r3.out" 2>&1 && grep -c -F -e '--- PASS: TestOutcomeRecord_PRsDereference/not_found_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/unmerged_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/merge_sha_mismatch_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/same_repo_not_ancestor_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/cross_repo_checked_against_default_tip' -e '--- PASS: TestOutcomeRecord_PRsDereference/forge_error_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/legacy_verified_with_prs_dereferenced' -e '--- PASS: TestOutcomeRecord_PRsDereference/valid_accepted' "${TMPDIR:-/tmp}/b27-r3.out"` | exit 0; prints `8` | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v > "${TMPDIR:-/tmp}/b27-r4.out" 2>&1 && grep -c -F -e '--- PASS: TestOutcomeRecord_VerifiedRules/legacy_verified_lands_unchanged' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_without_prs_refused' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_complete_accepted' "${TMPDIR:-/tmp}/b27-r4.out"` | exit 0; prints `3` | check:ci +neighbour |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PublicRedaction$' -v > "${TMPDIR:-/tmp}/b27-r5.out" 2>&1 && grep -c -F -e '--- PASS: TestOutcomeRecord_PublicRedaction/private_pr_on_public_register_withheld' -e '--- PASS: TestOutcomeRecord_PublicRedaction/clear_dispatch_ref_on_public_register_digested' -e '--- PASS: TestOutcomeRecord_PublicRedaction/private_register_keeps_clear_values' -e '--- PASS: TestOutcomeRecord_PublicRedaction/written_bytes_contain_no_clear_private_repo' "${TMPDIR:-/tmp}/b27-r5.out"` | exit 0; prints `4` (the last subtest greps the committed bytes for the private repo name and clear `dispatch_ref`) | check:ci +mutation |
| 6 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b27-sg.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b27-sg.out"` | exit 0; the test reads a record carrying every new key and derives the same latest outcome as without them | check:ci +flow |
| 7 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v > "${TMPDIR:-/tmp}/b27-r7.out" 2>&1 && grep -c -F -e '--- PASS: TestOutcomeRecord_RunMetadata/missing_verifier_tier_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/vendor_model_name_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/started_after_finished_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/row_results_count_mismatch_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/unknown_result_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/valid_with_and_without_optionals' "${TMPDIR:-/tmp}/b27-r7.out"` | exit 0; prints `6` | check:ci +mutation |
| 8 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestForgeGit.*Golden$' -v > "${TMPDIR:-/tmp}/b27-r8.out" 2>&1 && grep -c -F -e '--- PASS: TestForgeGithubGolden' -e '--- PASS: TestForgeGitlabGolden' "${TMPDIR:-/tmp}/b27-r8.out"` | exit 0; prints `2` (GitHub and GitLab goldens include `MergeCommitSHA` for a merged change and stay byte-identical elsewhere) | check:ci +neighbour |
| 9 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |
| 10 | `jq -r 'select(.prs) \| .prs[] \| select(.repo) \| "\(.repo) \(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json` (on main, after the first post-merge FAIL or BLOCKED record lands) | at least one line; for one printed `<repo> <number> <sha>` triple, `gh api repos/<repo>/pulls/<number> --jq '.merged, .merge_commit_sha'` prints `true` and that `<sha>` | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A private repo name or PR number lands in a public register | row 5 (the committed bytes are grepped) |
| A claim key with a private alias lands in a public register via `dispatch_ref` | row 5 |
| Writer accepts any `prs` value without checking the forge (presence-only) | row 3 (one subtest per refusal) |
| Cross-repo entries are always refused by a same-repo ancestry rule | row 3 `cross_repo_checked_against_default_tip` |
| The merge-commit check can't be done with existing reads | row 8 (adapter field) + row 3 `merge_sha_mismatch_refused` |
| Stricter rules break legacy `verified` records, or wake-v1 `verified` slip through without `prs` | row 4 |
| Run metadata accepted as free text or inconsistent with the row counts | row 7 |
| A reader somewhere chokes on the new keys | row 6 (statusgen); deskkit reader is row 1 |
| Real records name the wrong PR (e.g. the Evidence PR) | row 10 dereferences one; Evidence-only exclusion stays review-only (needs a diff-shape judgement) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no; `sensitive-data: no` rests on Task 4's
redaction, which row 5 proves). Reviewer records verdict + date in the stream README table.
