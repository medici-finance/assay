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
  (b) correctness spans the record schema doc, the deskkit receipt type, the deskevidence writer's
  forge reads, refusal path and redaction, and the verify-desk skill that composes the record; a writer that accepts a PR
  reference it never dereferenced, or writes a private reference into a non-private register, survives a happy-path test (c).
domain: complicated
outcome: none
sources:
  - "docs/streams/desk-supervision/verify-wake-v1.md — the receipt schema this brief extends (fields table; blocker kinds closed set)"
  - "docs/streams/desk-supervision/brief-24-per-file-verify-outcomes.md — the per-file register and the writer (`deskevidence --outcome-record`) this brief extends"
  - "register read 2026-10-06 @ 1fbf1153f: 289 records (legacy log + per-file register); 0 carry any PR key; verify-fail records: 71 with a verify-wake-v1 receipt, 16 legacy-shape with no blocker_kind; blocked: 150 with a receipt, 2 legacy-shape"
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models and statistical learning to the pipeline concluded the verify register cannot yet be joined to review or CI data, and that join keys must be captured at write time before any model is worth fitting"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): no `prs`/`pr`/`merge_sha` key in deskkit.WakeReceipt (tools/desk/internal/deskkit/verifywake.go:70-95); `--outcome-record` validates receipts only when wake_schema is present (tools/desk/cmd/deskevidence/outcomerecord.go:151-162), so a legacy-shape verify-fail lands unvalidated"
  - "freshness-checked 2026-10-08 @ a0b70eb87 (origin/main): `deskkit.PullRequest.MergeCommitSHA` already exists, populated by both adapters with goldens (tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go — it landed after this brief was first authored), so this brief adds no forge-adapter change; `Forge.RepoDefaultBranch` exists; the dispatcher-owned verifier attestation (#2239) has merged and `--outcome-record` now calls `admitVerifierEvidence` (outcomerecord.go); `WakeReceipt` still has no `prs`/`verifier_tier`/`started`/`finished`/`dispatch_ref` key"
consumers:
  - "docs/streams/desk-supervision/verify-wake-v1.md (fields table, Migration section): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/verifywake.go (WakeReceipt gains the `prs` field): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskevidence/outcomerecord.go and receiptvalidate.go (the refusals and the forge dereference): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the outcome-record composition paragraphs): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (deskevidence section): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/verify-outcomes/README.md (the register README's 'optional receipt' wording): follow-up desk-supervision/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/verifyoutcomes.go (statusgen's own reader copy): out-of-scope (it decodes only `brief`/`ts`/`outcome`/`sha` and ignores additive keys, so a record carrying `prs` reads exactly as before — Verify row 6 proves it)"
  - "tools/desk/cmd/verifyloop (wake evaluation): out-of-scope (the wake evaluator reads inputs, predicate and blocker; `prs` is not a wake input, and the stricter failure requirement only removes legacy-shape records it already treats as unclassified)"
  - "docs/streams/quality/brief-11-dora-join.md (already joins on PR number + merge SHA + stream/task id): out-of-scope (a later reader of these keys; it needs no change for them to land)"
  - "tools/desk/internal/deskkit/verifierattestation.go (merged #2239, the dispatcher-owned verifier attestation): out-of-scope (read-only here: the writer cross-checks `verifier_tier` against the admitted binding's `Tier`; `dispatch_ref` stays self-reported — see facts)"
  - "existing records in the register: out-of-scope (records are immutable by design; no backfill — older records stay legacy-shape and read as they do today)"
---

# Brief 27 — Verify-outcome records carry join keys — delivering PRs, verifier tier and timing, per-row results — and every failure a blocker kind

## Context
files:
- **edit** `docs/streams/desk-supervision/verify-wake-v1.md` — the new fields, the failure-record
  requirement, the non-private-register redaction rule, the GitLab ancestry fallback; Migration notes older records are not backfilled.
- **edit** `tools/desk/internal/deskkit/verifywake.go` — `WakeReceipt` gains `PRs []DeliveredPR`
  (`DeliveredPR{Repo string; Number int; MergeSHA string; Withheld string}`, all `omitempty`),
  `PRsUnknownReason`, `VerifierTier`, `Started`, `Finished`, `RowsPassed`, `RowsTotal`
  (`rows_passed`/`rows_total`, today read by no Go code), `RowResults`, `DispatchRef`.
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
  It already performs a repo-visibility read for the public-repo gate (`publicRepoGateFn` over
  `deskkit.ForgeRepoInfoFetcher`) and the withheld-identifier outbound scan (`evidenceOutboundCheck`).
- `deskkit.WakeReceipt.Complete()` (`tools/desk/internal/deskkit/verifywake.go`) is the
  existing completeness test; it fails on a missing `wake_schema`, `receipt_id`, `blocker_kind`,
  `wake_predicate`, or a predicate-specific field. `blocker_kind` is a closed set.
- `deskkit.PullRequest` (`forge.go`, `type PullRequest struct`) already carries `HeadSHA`, `BaseRef`,
  `MergedAt`, `Merged` and `MergeCommitSHA` (set only for a merged change; GitLab fills it from
  `merge_commit_sha`, else `squash_commit_sha`, and a fast-forward merge reports neither, so it stays
  EMPTY — empty is could-not-check, never "no merge commit"). `Forge.RepoVisibility`,
  `Forge.RepoDefaultBranch` and `Forge.CompareRefs` exist, with one caveat: `GitLabForge.CompareRefs`
  (`forge_gitlab.go`, the doc comment above `func (g *GitLabForge) CompareRefs`) ALWAYS returns
  could-not-check ("deferred to the forge-gitlab compare brief"), so ancestry cannot be read through
  it on GitLab; Task 3 defines the GitLab behaviour. `PublicRepoGate` and `RepoVisibility`
  normalisation (`repovis.go`, the comment above `PublicRepoGate`) are an allow-list on the
  lowercased, trimmed value because a `== "public"` denylist was bypassed by `PUBLIC`, `public ` and
  `internal`; Task 4 uses the same shape.
- The brief → PR convention: a delivering PR carries a `Brief: <stream>/<NN>` trailer; statusgen
  resolves it via `PRsNamingBrief` (`statusgen/autoflip.go`). The verifier
  uses it to find candidate PRs; the writer only checks what the record names.
- `dispatch_ref` is defined by desk-supervision/28 (merged; `brief-28-dispatch-record.md`) as the per-run id
  `<claim_key>@<YYYYMMDDTHHMMSSZ>.<12 lowercase hex>` (a 48-bit random nonce after the timestamp),
  readable as `git config --worktree assay.dispatchRef`. A claim
  key can carry a tracking repo's alias and issue number, so it is never written in clear to a
  non-private register.
- Go modules: this repo has no root `go.mod`/`go.work`; `tools/desk` and `statusgen` are separate
  modules, so every Go Verify row `cd`s into its module first (the sibling briefs 24-26 form).
- Records are immutable; a correction is a new record. Every outcome-record reader decodes with
  plain `json.Unmarshal` into a partial struct, so additive keys are backward compatible.
- Overlap: #2239 has merged. `--outcome-record` calls `admitVerifierEvidence`
  (`outcomerecord.go`), which returns a `deskkit.VerifierReceipt` whose `Binding` carries `Tier`
  (and `Model`, `Run`, `Repo`, `Source`, `Brief` and three digests — NO `<claim_key>@<timestamp>.<nonce>`
  value). So only `verifier_tier` can be cross-checked against it: when the admitted binding carries a
  `Tier`, a self-reported `verifier_tier` that differs is refused (exit 5). `dispatch_ref` has no
  attested source; it stays self-reported and shape-checked.
- Redaction order: today `cmdOutcomeRecordWrite` runs, in this order, `RecordName` (the file name,
  from the raw input), `admitVerifierEvidence`, `CanonicalBytes` and the size cap,
  `publishIdentityGate`, the token mint and the forge handle (`forgeForFn`), the immutability
  compare, the withheld-identifier outbound scan, the visibility read (`publicRepoGateFn`), and only
  then `bodyDig`. Task 4's forge reads are driven by the record, so they need the token and the
  forge handle that today come after `RecordName`: Task 4 moves redaction BEFORE all of those, which
  means the identity gate, the token mint and the forge handle move ahead of `RecordName` too. Every
  later step (name digest, size cap, immutability compare, outbound scan, `bodyDig`,
  `landOutcomeRecordAsChange`) then sees the redacted record.
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
   - `dispatch_ref` (optional; written only to a register whose live visibility is `private`) — Task 4.
   `withheld` (inside a `prs` entry) is a form only the WRITER produces (Task 4); it is never valid input.
   Free text is never a value of any of these fields. verify-wake-v1.md labels `prs` and
   `dispatch_ref` as verifier-asserted: the writer checks that a `prs` entry exists, is merged and
   carries that merge commit, but not that it delivered THIS brief, and `dispatch_ref` has no attested
   source, so a consumer must not treat either join as authenticated.
2. **Which records must carry what.**
   - A NEW `verify-fail` or `blocked` record must carry `wake_schema: verify-wake-v1` with
     `WakeReceipt.Complete()` true; else refuse (exit 5) naming the FIRST failing field.
   - Every NEW verify-wake-v1 record, whatever its `outcome` (including `verified`), must carry
     `prs` (or `prs: []` + `prs_unknown_reason`), `verifier_tier` (validated against
     `deskkit.DispatchTiers()`, and equal to the admitted binding's `Tier` when that carries one),
     `started`, `finished`; else refuse.
   - An INPUT record that carries a `withheld` key in any `prs` entry is refused (exit 5): `withheld`
     is writer output only, so a presence-only `{withheld: <any hex>}` can never satisfy "`prs`
     required" without a forge read. This holds for every record, whatever its `wake_schema`.
   - A legacy-shape `verified` record (no `wake_schema`) has no NEW required field, so it still lands
     without a receipt. That is the only thing it keeps: it is still subject to the `withheld`-input
     refusal above, to Task 4's redaction and `dispatch_ref` drop (a legacy-shape record is the usual
     `verified` record, and may carry `prs` and `dispatch_ref` like any other), and, when it carries
     `prs`, to the Task 3 dereference (presence is never accepted unchecked, whatever `wake_schema`
     says).
3. **Dereference every `prs` entry** (each of `exists`, `merged`, `merge commit`, `ancestry`
   refuses with its own message; any forge read error, and an EMPTY `MergeCommitSHA` on a merged
   change, is exit 6 — could-not-check, never a pass and never a refusal of the PR itself; a read the
   register's token cannot make on another repo is the same exit 6):
   - the PR exists on the entry's `repo` and is merged (`GetPullRequest`);
   - its `MergeCommitSHA` equals `merge_sha`;
   - ancestry, on a forge whose `CompareRefs` is served (GitHub): when `repo` is the register's own
     repo, `merge_sha` is an ancestor of (or equal to) the record's `sha`; when `repo` is another repo,
     `merge_sha` is an ancestor of (or equal to) the tip of that repo's `RepoDefaultBranch` at write
     time (`CompareRefs` status `identical` or `ahead`);
   - ancestry on GitLab, chosen by the resolved forge KIND (`ForgeGitLab`), never by catching the
     `CompareRefs` could-not-check (which would turn an outage into a weaker check): the GitLab ancestry
     read is deferred, so the writer instead requires `PullRequest.BaseRef` to equal the entry repo's
     `RepoDefaultBranch` and `MergedAt` to be no later than the record's `ts`. This is weaker than
     ancestry (it does not show the record's `sha` contains the merge commit) and is stated as such in
     verify-wake-v1.md; when the forge-gitlab compare brief serves `CompareRefs`, the writer falls
     through to the ancestry rule above with no schema change.
4. **Non-private-register redaction (the writer decides; the verifier never has to).** Task 4
   applies to EVERY record, whatever its `wake_schema` or `outcome` — a legacy-shape `verified`
   record included. It is not part of the verify-wake-v1 validation branch: it must run for a record
   that never reaches `validateReceipt`. Redaction is an ALLOW-LIST on live visibility, never a
   `== "public"` test. Each read is `Forge.RepoVisibility`
   on the repo in question, normalised by lowercasing and trimming; ANY read error is exit 6.
   - The register is `private` only when its normalised live visibility is exactly `private`. On
     any other value (`public`, `internal`, an unrecognised spelling) the register is NON-PRIVATE.
   - On a private register both values below are written in clear.
   - On a non-private register, a `prs` entry is written in clear only when the ENTRY repo's own
     normalised live visibility is exactly `public`; every other entry is dereferenced (Task 3) and then
     written ONLY as `{withheld: sha256("<repo>#<number>@<merge_sha>")}` — the operator can join it on
     their side by hashing their own value; the clear `repo`/`number`/`merge_sha` never land in the
     file. The hash input contains the merge commit of a non-public repository, so it cannot be
     enumerated.
   - On a non-private register `dispatch_ref` is OMITTED: the writer drops it (a record that arrives
     carrying one is rewritten, never refused, so the verifier's composition is identical on every
     register). It is never hashed: desk-supervision/28 permits a sha256 digest of the FULL ref
     (`<claim_key>@<ts>.<nonce>`), and the 48-bit nonce makes that digest hard to enumerate from a
     guessed alias and the record's clear `started`, but it is still a stable identifier derived from
     the claim key and not a secret, and the PR keys already join the record. Dropping it costs no
     join — desk-supervision/28 defines the fallback join by PR keys.
   A `{withheld}` entry the WRITER produced is accepted as satisfying "`prs` required".
5. **Skill.** In verify-desk's outcome-record paragraphs: fill `prs` from the brief's
   `Brief:`-trailer PRs (excluding Evidence-only PRs) with each merge commit; set `verifier_tier`,
   `started`, `finished`, and `dispatch_ref` from `git config --worktree assay.dispatchRef` when set;
   a FAIL or BLOCKED outcome always writes a full verify-wake-v1 receipt; the writer redacts for
   non-private registers, so write clear values and never write `withheld` yourself.
6. **Docs + changelog.** Update `tools/desk/README.md`'s deskevidence section; amend
   `docs/streams/verify-outcomes/README.md` as listed in `files:`; add the changelog fragment
   (bullets: the new keys, the stricter records, the non-private-register redaction).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskevidence/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v > "${TMPDIR:-/tmp}/b27-r2.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/legacy_verify_fail_names_wake_schema' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_blocker_kind_names_blocker_kind' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_receipt_id_names_receipt_id' "${TMPDIR:-/tmp}/b27-r2.out" > "${TMPDIR:-/tmp}/b27-r2.hits" && test "$(wc -l < "${TMPDIR:-/tmp}/b27-r2.hits")" -eq 3` | exit 0 (each subtest asserts the refusal message names that first failing field) | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v > "${TMPDIR:-/tmp}/b27-r3.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_PRsDereference/not_found_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/unmerged_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/merge_sha_mismatch_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/same_repo_not_ancestor_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/cross_repo_checked_against_default_tip' -e '--- PASS: TestOutcomeRecord_PRsDereference/forge_error_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/empty_merge_commit_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_uses_base_ref_and_merged_at' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_wrong_base_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_compare_error_not_downgraded' -e '--- PASS: TestOutcomeRecord_PRsDereference/legacy_verified_with_prs_dereferenced' -e '--- PASS: TestOutcomeRecord_PRsDereference/input_withheld_entry_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/valid_accepted' "${TMPDIR:-/tmp}/b27-r3.out")" -eq 13` | exit 0 (the command asserts the count 13) (`gitlab_*` run against a GitLab-backed fake whose `CompareRefs` returns could-not-check; `gitlab_compare_error_not_downgraded` shows a GitHub-kind forge's `CompareRefs` error stays exit 6 rather than falling to the GitLab rule; `input_withheld_entry_refused` feeds `prs:[{"withheld":"<64 hex>"}]` with no forge read, once on a verify-wake-v1 record and once on a legacy-shape `verified` record, and expects exit 5 both times) | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v > "${TMPDIR:-/tmp}/b27-r4.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_VerifiedRules/legacy_verified_no_new_requirement' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_without_prs_refused' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_complete_accepted' "${TMPDIR:-/tmp}/b27-r4.out")" -eq 3` | exit 0 (the command asserts the count 3; `legacy_verified_no_new_requirement` asserts that a legacy-shape `verified` record with no `prs` and no `dispatch_ref` still lands without a receipt, and says nothing about its bytes passing through unchanged) | check:ci +neighbour |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_NonPrivateRedaction$' -v > "${TMPDIR:-/tmp}/b27-r5.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_pr_on_public_register_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/internal_register_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/unrecognised_visibility_spelling_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/padded_uppercase_private_register_keeps_clear' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_repo_internal_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/register_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/dispatch_ref_dropped_and_not_hashed' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_register_keeps_clear_values' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/written_bytes_contain_no_clear_private_repo' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/land_as_change_bytes_redacted' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/redacted_before_outbound_scan' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_non_public_pr_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_dispatch_ref_dropped' "${TMPDIR:-/tmp}/b27-r5.out")" -eq 14` | exit 0 (the command asserts the count 14). `internal_register_redacts` is the allow-list case (an `internal` register must redact exactly as a `public` one); `dispatch_ref_dropped_and_not_hashed` computes the unkeyed `sha256` of the fixture's clear `dispatch_ref` (a ref in the nonce form, e.g. `assay--x--1@20261006T141502Z.3fa9c01b7d2e`) and asserts neither it nor the clear value appears in the written bytes; `written_bytes_contain_no_clear_private_repo` greps the committed bytes; `land_as_change_bytes_redacted` does the same on the GitLab land-as-change path; `redacted_before_outbound_scan` uses an operator token map naming the private repo and expects the record rewritten and landed, not refused; the two `legacy_verified_*` subtests use a legacy-shape `verified` fixture (no `wake_schema`) on a non-private register — one carries a non-public `prs` entry, the other carries `dispatch_ref` and no `prs` — and each asserts the committed bytes hold neither the clear value nor, for `dispatch_ref`, its unkeyed `sha256` | check:ci +mutation |
| 6 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b27-sg.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b27-sg.out"` | exit 0; the test reads a record carrying every new key and derives the same latest outcome as without them | check:ci +flow |
| 7 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v > "${TMPDIR:-/tmp}/b27-r7.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_RunMetadata/missing_verifier_tier_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/vendor_model_name_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/tier_disagreeing_with_admitted_binding_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/started_after_finished_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/row_results_count_mismatch_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/unknown_result_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/valid_with_and_without_optionals' "${TMPDIR:-/tmp}/b27-r7.out")" -eq 7` | exit 0 (the command asserts the count 7) | check:ci +mutation |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |
| 9 | `jq -r 'select(.prs) \| .prs[] \| select(.repo) \| "\(.repo) \(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json` (on main, after the first post-merge record lands whose `prs` has a CLEAR entry — verified records carry `prs` too, and on this public register an entry that is only `{withheld}` prints nothing, so a record whose only PR is non-public does not satisfy the precondition; run it again after the next one) | at least one line; for one printed `<repo> <number> <sha>` triple, `gh api repos/<repo>/pulls/<number> --jq '.merged, .merge_commit_sha'` prints `true` and that `<sha>` | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A private repo name or PR number lands in a public OR internal register (a `== "public"` redaction test) | row 5 `internal_register_redacts`, `unrecognised_visibility_spelling_redacts`, and the committed-bytes grep |
| Redaction runs only inside the verify-wake-v1 branch, so the usual legacy-shape `verified` record commits a clear private PR reference to a non-private register | row 5 `legacy_verified_non_public_pr_withheld` |
| A claim key with a private alias lands in a non-private register via `dispatch_ref` (clear or unkeyed hash) | row 5 `dispatch_ref_dropped_and_not_hashed`, and, for a legacy-shape `verified` record, `legacy_verified_dispatch_ref_dropped` |
| Writer accepts any `prs` value without checking the forge (presence-only), including a writer-only `{withheld}` form supplied as input | row 3 (one subtest per refusal) and `input_withheld_entry_refused` |
| Cross-repo entries are always refused by a same-repo ancestry rule | row 3 `cross_repo_checked_against_default_tip` |
| GitLab records can no longer land because `CompareRefs` is a could-not-check there, or a forge outage is mistaken for the GitLab case | row 3 `gitlab_uses_base_ref_and_merged_at`, `gitlab_compare_error_not_downgraded` |
| A merged change with no merge commit (GitLab fast-forward) is read as "no PR" | row 3 `empty_merge_commit_exit_6` |
| A cross-repo entry the register's token cannot read looks like not-found on GitHub, so no outcome record (FAIL and BLOCKED included) lands for a brief whose delivering PR is in such a repository | stated, not asserted: the writer exits 6 (could-not-check) and never a pass |
| A visibility read fails and is treated as public or private | row 5 `register_visibility_read_error_exit_6`, `entry_visibility_read_error_exit_6` |
| Redaction runs after the outbound scan, so an operator whose token map names the private repo is refused instead of rewritten | row 5 `redacted_before_outbound_scan` |
| The merge-commit check relies on a field that is missing or empty | row 3 `merge_sha_mismatch_refused`, `empty_merge_commit_exit_6` (the field itself already exists, with goldens, on main) |
| Stricter rules break legacy `verified` records, or wake-v1 `verified` slip through without `prs` | row 4 |
| Run metadata accepted as free text, inconsistent with the row counts, or disagreeing with the admitted tier | row 7 |
| A reader somewhere chokes on the new keys | row 6 (statusgen); deskkit reader is row 1 |
| Real records name the wrong PR (e.g. the Evidence PR) | row 9 dereferences one; Evidence-only exclusion stays review-only (needs a diff-shape judgement) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no; `sensitive-data: no` rests on Task 4's
redaction, which row 5 proves). Reviewer records verdict + date in the stream README table.
