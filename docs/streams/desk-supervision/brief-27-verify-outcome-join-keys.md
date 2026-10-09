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
- **edit** `docs/streams/desk-supervision/verify-wake-v1.md` — the new fields, labelling `prs` and
  `dispatch_ref` verifier-asserted, the failure-record requirement, what "complete" means for a
  verify-wake-v1 `verified` record (Task 2), the redaction rules for private and non-private
  registers including the history caveat (Task 4), the GitLab ancestry fallback; Migration notes
  older records are not backfilled.
- **edit** `tools/desk/internal/deskkit/verifywake.go` — `WakeReceipt` gains `PRs []DeliveredPR`
  (`DeliveredPR{Repo string; Number int; MergeSHA string; Withheld string}`, all `omitempty`),
  `PRsUnknownReason`, `VerifierTier`, `Started`, `Finished`, `RowsPassed`, `RowsTotal`
  (`rows_passed`/`rows_total`, today read by no Go code), `RowResults`, `DispatchRef`,
  `DispatchRefSHA256`. NEW code beside `Complete()`: a helper that returns the name of the FIRST
  field `Complete()` would fail on, in `Complete()`'s own check order (Task 2's refusal names it;
  `Complete()` itself returns only a bool and stays unchanged).
- **edit** `tools/desk/cmd/deskevidence/outcomerecord.go`, `receiptvalidate.go` + tests — the
  input-shape refusals, the reordered admission and forge handle, the forge dereference, the
  redaction (Tasks 2–5).
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
  `wake_predicate`, or a predicate-specific field (`blocker_ref` is that field only for
  `referenced-action-done`), and returns only a bool. `blocker_kind` is a closed set. The writer
  does not call it today. Separately, the writer's existing `validateReceipt` (`receiptvalidate.go`,
  `validateReceiptBlockerRef`) refuses an EMPTY `blocker_ref` on EVERY verify-wake-v1 record,
  whatever its `outcome`, and dereferences a non-empty one on the forge. verify-wake-v1.md (Where it
  lives) says the schema governs failed and blocked retries only.
- `deskkit.PullRequest` (`forge.go`, `type PullRequest struct`) already carries `HeadSHA`, `BaseRef`,
  `MergedAt`, `Merged` and `MergeCommitSHA` (set only for a merged change; GitLab fills it from
  `merge_commit_sha`, else `squash_commit_sha`, and a fast-forward merge reports neither, so it stays
  EMPTY — empty is could-not-check, never "no merge commit"). It also carries `URL`, the change's
  own page as the forge reports it (GitHub `html_url`, GitLab `web_url`, both already mapped by
  `GetPullRequest`), whose path is the forge's own spelling of the repository (`<owner>/<name>`
  before `/pull/<number>` on GitHub, before `/-/merge_requests/<number>` on GitLab); Task 4 takes
  the canonical repository name from it, so no adapter change is needed. `Forge.RepoVisibility`,
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
  key can carry a tracking repo's alias and issue number, so brief 28's Visibility rule (authoritative
  for this brief) keeps the clear ref out of any non-private surface AND out of any file committed to
  git: a consumer that must express the join there writes at most a sha256 digest of the FULL ref.
  Every register file is committed to git, so a clear `dispatch_ref` is never written to a register
  of any visibility (Task 4).
- Go modules: this repo has no root `go.mod`/`go.work`; `tools/desk` and `statusgen` are separate
  modules, so every Go Verify row `cd`s into its module first (the sibling briefs 24-26 form).
- Records are immutable; a correction is a new record. Every outcome-record reader decodes with
  plain `json.Unmarshal` into a partial struct, so additive keys are backward compatible.
- Overlap: #2239 has merged. `--outcome-record` calls `admitVerifierEvidence`
  (`outcomerecord.go`), which returns a `deskkit.VerifierReceipt` whose `Binding` carries `Tier`
  (and `Model`, `Run`, `Repo`, `Source`, `Brief` and three digests — NO `<claim_key>@<timestamp>.<nonce>`
  value). So only `verifier_tier` can be cross-checked against it: when the admitted binding carries a
  `Tier`, a self-reported `verifier_tier` that differs is refused (exit 5). `dispatch_ref` has no
  attested source; it stays self-reported, and its shape is checked against brief 28's grammar
  (Task 2).
- Step order today: `cmdOutcomeRecordWrite` runs, in this order, `RecordName` (the file name,
  from the raw input), `admitVerifierEvidence`, `CanonicalBytes` and the size cap,
  `publishIdentityGate`, the token mint and the forge handle (`forgeForFn`), the immutability
  compare, the withheld-identifier outbound scan, the visibility read (`publicRepoGateFn`), and only
  then `bodyDig`. Admission takes the target path but reads only its `<stream>/<NN>` part
  (`VerifierReceipt.CheckEvidenceTarget`), which redaction never changes, so it does not need the
  final (redacted) file name. Task 4's forge reads are driven by the record, so they need the token
  and the forge handle that today come after `RecordName`. The order this brief requires, stated once
  here and referenced by Tasks 2–4:
  1. parse and the future-`ts` check (as today), then every input-shape refusal of Task 2 — pure
     checks, exit 5, before ANY forge read (admission's own reads included);
  2. admission (`admitVerifierEvidence`), on the input record's `<stream>/<NN>` target, BEFORE any
     record-driven forge read: a caller that is not admitted triggers no forge read the record
     names (no PR, ancestry, default-branch, `blocker_ref` or visibility read);
  3. the identity gate, the token mint and the forge handle (moved ahead of `RecordName`);
  4. the record-driven forge reads — Task 3's dereference and Task 4's visibility reads — then
     Task 4's redaction;
  5. `RecordName` on the REDACTED record and its path-prefix guard, then everything that follows
     today (size cap, immutability compare, outbound scan, the public-repo gate, `bodyDig`, the
     closure gates, `validateReceipt`, the write or `landOutcomeRecordAsChange`), all of which see
     the redacted record.
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
   - `dispatch_ref` (optional INPUT) — the run's per-run id from desk-supervision/28, shape-checked
     by Task 2. The writer never commits it in clear, on any register (Task 4).
   - `dispatch_ref_sha256` — WRITER output only: the lowercase-hex sha256 of the FULL input
     `dispatch_ref`, written only to a private register (Task 4).
   `withheld` (inside a `prs` entry) and `dispatch_ref_sha256` are forms only the WRITER produces
   (Task 4); neither is ever valid input. Free text is never a value of any of these fields.
   verify-wake-v1.md labels `prs` and `dispatch_ref` (with its digest) as verifier-asserted: the
   writer checks that a `prs` entry exists, is merged and carries that merge commit, but not that it
   delivered THIS brief, and `dispatch_ref` has no attested source (its shape is checked, its claim
   key is not compared with the admitted run), so a consumer must not treat either join as
   authenticated.
2. **Which records must carry what.**
   - **Input shape, checked first.** These are pure checks on the parsed input, run right after the
     parse and the future-`ts` check (step 1 of the facts' step order): before admission, before any
     forge read and before any hashing. Each failure is exit 5 naming the field, on every register
     kind (private or not) and every record shape (verify-wake-v1 or legacy-shape):
     - each `prs` entry's `repo` is `<owner>/<name>`: exactly one `/`, each part 1–100 bytes from
       `[A-Za-z0-9._-]` and neither part `.` or `..`;
     - its `number` is a JSON integer >= 1 (zero, negative, fractional or a string is refused);
     - its `merge_sha` is a full object name: exactly 40 or exactly 64 lowercase hex characters;
     - a `dispatch_ref`, when present, matches desk-supervision/28's suffix grammar with a bounded
       prefix: `^[\x21-\x3F\x41-\x7E]{1,226}@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` — a prefix of 1–226
       printable-ASCII bytes with no `@`, whitespace or control byte, then `@`, a basic-form UTC
       timestamp, `.` and 12 lowercase hex characters; 226 + the 30-byte suffix keeps the whole value
       within desk-supervision/28's 256-byte string cap;
     - an input carrying `dispatch_ref_sha256`, or a `withheld` key in any `prs` entry (below), is
       refused: both are writer output only.
   - A NEW `verify-fail` or `blocked` record must carry `wake_schema: verify-wake-v1` and be
     complete; else refuse (exit 5) naming the FIRST failing field (the new `files:` helper, in
     `Complete()`'s own check order). "Complete", for EVERY verify-wake-v1 record whatever its
     `outcome` (`verified` included), means exactly what the writer requires today and nothing
     less: `Complete()` true plus the existing `validateReceipt` checks, unchanged — `blocker_ref`
     included, so a verify-wake-v1 `verified` record still needs a non-empty `blocker_ref` that
     dereferences. This brief adds no exemption and changes no existing check.
   - Every NEW verify-wake-v1 record, whatever its `outcome` (including `verified`), must carry
     `prs` (or `prs: []` + `prs_unknown_reason`), `verifier_tier` (validated against
     `deskkit.DispatchTiers()`, and equal to the admitted binding's `Tier` when that carries one),
     `started`, `finished`; else refuse.
   - An INPUT record that carries a `withheld` key in any `prs` entry is refused (exit 5): `withheld`
     is writer output only, so a presence-only `{withheld: <any hex>}` can never satisfy "`prs`
     required" without a forge read. This holds for every record, whatever its `wake_schema`.
   - A legacy-shape `verified` record (no `wake_schema`) has no NEW required field, so it still lands
     without a receipt. That is the only thing it keeps: it is still subject to the input-shape
     checks and the `withheld`-input refusal above, to Task 4's redaction and `dispatch_ref` handling
     (digest-only on a private register, dropped otherwise) (a legacy-shape record is the usual
     `verified` record, and may carry `prs` and `dispatch_ref` like any other), and, when it carries
     `prs`, to the Task 3 dereference (presence is never accepted unchecked, whatever `wake_schema`
     says).
3. **Dereference every `prs` entry** (each of `exists`, `merged`, `merge commit`, `ancestry`
   refuses with its own message; any forge read error, and an EMPTY `MergeCommitSHA` on a merged
   change, is exit 6 — could-not-check, never a pass and never a refusal of the PR itself; a read the
   register's token cannot make on another repo is the same exit 6). These reads run at step 4 of
   the facts' step order — after the input-shape refusals and after admission — so neither a
   malformed entry nor a record from a caller that is not admitted causes any of them:
   - the PR exists on the entry's `repo` and is merged (`GetPullRequest`); the entry's CANONICAL
     repository name is the `<owner>/<name>` the forge returns in that read's `PullRequest.URL`
     (GitHub names are case-insensitive, so the input spelling is not canonical); a URL that does not
     yield one is exit 6;
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
4. **Register redaction (the writer decides; the verifier never has to).** Task 4
   applies to EVERY record, whatever its `wake_schema` or `outcome` — a legacy-shape `verified`
   record included. It is not part of the verify-wake-v1 validation branch: it must run for a record
   that never reaches `validateReceipt`. It runs at step 4 of the facts' step order, so `RecordName`
   and every later step see only its output. Redaction is an ALLOW-LIST on live visibility, never a
   `== "public"` test. Each read is `Forge.RepoVisibility`
   on the repo in question, normalised by lowercasing and trimming; ANY read error is exit 6.
   - The register is `private` only when its normalised live visibility is exactly `private`. On
     any other value (`public`, `internal`, an unrecognised spelling) the register is NON-PRIVATE.
   - Every `prs` entry the writer commits names the entry's CANONICAL repository (Task 3), never
     the input spelling, so one repository always yields one join key.
   - On a private register, `prs` entries are written in clear. `dispatch_ref` is NOT: the writer
     computes `dispatch_ref_sha256` = sha256 of the FULL input `dispatch_ref`
     (`<claim_key>@<ts>.<nonce>`) itself, from the clear value, writes only that, and drops the clear
     value, for every record shape. A register file is committed to git, which desk-supervision/28's
     Visibility rule names among the surfaces that never carry the clear ref, and that rule allows
     at most a sha256 digest of the full ref there. verify-wake-v1.md also states that what a
     private register holds in clear (the `prs` entries) stays in its git history if the repository
     is later made public or internal: the writer reads visibility at write time only.
   - On a non-private register, a `prs` entry is written in clear only when the ENTRY repo's own
     normalised live visibility is exactly `public`; every other entry is dereferenced (Task 3) and then
     written ONLY as `{withheld: sha256("<canonical repo>#<number>@<merge_sha>")}` — the operator can
     join it on their side by hashing their own value with the canonical name; the clear
     `repo`/`number`/`merge_sha` never land in the file. The hash input contains the merge commit of a
     non-public repository, so it cannot be enumerated.
   - On a non-private register `dispatch_ref` is OMITTED: the writer drops it (a record that arrives
     carrying one is rewritten, never refused, so the verifier's composition is identical on every
     register). It is not hashed there either — no `dispatch_ref_sha256` is written: the 48-bit
     nonce makes the digest hard to enumerate from a guessed alias and the record's clear `started`,
     but it is still a stable identifier derived from the claim key and not a secret, and the PR keys
     already join the record. Dropping it costs no join — desk-supervision/28 defines the fallback
     join by PR keys.
   A `{withheld}` entry the WRITER produced is accepted as satisfying "`prs` required".
5. **Skill.** In verify-desk's outcome-record paragraphs: fill `prs` from the brief's
   `Brief:`-trailer PRs (excluding Evidence-only PRs) with each merge commit; set `verifier_tier`,
   `started`, `finished`, and `dispatch_ref` from `git config --worktree assay.dispatchRef` when set;
   a FAIL or BLOCKED outcome always writes a full verify-wake-v1 receipt; the writer redacts (it
   replaces `dispatch_ref` with its digest on a private register and drops it on a non-private one,
   and withholds non-public `prs` entries on a non-private one), so write clear values and never
   write `withheld` or `dispatch_ref_sha256` yourself.
6. **Docs + changelog.** Update `tools/desk/README.md`'s deskevidence section; amend
   `docs/streams/verify-outcomes/README.md` as listed in `files:`; add the changelog fragment
   (bullets: the new keys, the stricter records, the input-shape refusals, the register redaction
   including the `dispatch_ref` digest on a private register).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskevidence/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v > "${TMPDIR:-/tmp}/b27-r2.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/legacy_verify_fail_names_wake_schema' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_blocker_kind_names_blocker_kind' -e '--- PASS: TestOutcomeRecord_FailWithoutReceipt/missing_receipt_id_names_receipt_id' "${TMPDIR:-/tmp}/b27-r2.out" > "${TMPDIR:-/tmp}/b27-r2.hits" && test "$(wc -l < "${TMPDIR:-/tmp}/b27-r2.hits")" -eq 3` | exit 0 (each subtest asserts the refusal message names that first failing field) | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v > "${TMPDIR:-/tmp}/b27-r3.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_PRsDereference/not_found_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/unmerged_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/merge_sha_mismatch_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/same_repo_not_ancestor_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/cross_repo_checked_against_default_tip' -e '--- PASS: TestOutcomeRecord_PRsDereference/forge_error_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/empty_merge_commit_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_uses_base_ref_and_merged_at' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_wrong_base_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_compare_error_not_downgraded' -e '--- PASS: TestOutcomeRecord_PRsDereference/legacy_verified_with_prs_dereferenced' -e '--- PASS: TestOutcomeRecord_PRsDereference/input_withheld_entry_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/malformed_entry_no_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/not_admitted_no_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/valid_accepted' "${TMPDIR:-/tmp}/b27-r3.out")" -eq 15` | exit 0 (the command asserts the count 15) (`malformed_entry_no_read` feeds a `repo` that is not `<owner>/<name>`, a `number` of 0 and of -1, and a short and an uppercase `merge_sha`, on a private and a public register, and expects exit 5 naming the field with the fake forge recording ZERO reads; `not_admitted_no_read` feeds a well-formed record whose admission fails and expects the admission refusal with no `GetPullRequest`, `CompareRefs`, `RepoDefaultBranch`, `blocker_ref` or `RepoVisibility` read; `gitlab_*` run against a GitLab-backed fake whose `CompareRefs` returns could-not-check; `gitlab_compare_error_not_downgraded` shows a GitHub-kind forge's `CompareRefs` error stays exit 6 rather than falling to the GitLab rule; `input_withheld_entry_refused` feeds `prs:[{"withheld":"<64 hex>"}]` with no forge read, once on a verify-wake-v1 record and once on a legacy-shape `verified` record, and expects exit 5 both times) | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v > "${TMPDIR:-/tmp}/b27-r4.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_VerifiedRules/legacy_verified_no_new_requirement' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_without_prs_refused' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_complete_accepted' "${TMPDIR:-/tmp}/b27-r4.out")" -eq 3` | exit 0 (the command asserts the count 3; `legacy_verified_no_new_requirement` asserts that a legacy-shape `verified` record with no `prs` and no `dispatch_ref` still lands without a receipt, and says nothing about its bytes passing through unchanged) | check:ci +neighbour |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_NonPrivateRedaction$' -v > "${TMPDIR:-/tmp}/b27-r5.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_pr_on_public_register_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/internal_register_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/unrecognised_visibility_spelling_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/padded_uppercase_private_register_keeps_clear' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_repo_internal_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/register_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/dispatch_ref_dropped_and_not_hashed' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_keeps_clear_prs' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_dispatch_digest_only' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/withheld_uses_canonical_repo' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/written_bytes_contain_no_clear_private_repo' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/land_as_change_bytes_redacted' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/redacted_before_outbound_scan' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_non_public_pr_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_dispatch_ref_dropped' "${TMPDIR:-/tmp}/b27-r5.out")" -eq 16` | exit 0 (the command asserts the count 16). `internal_register_redacts` is the allow-list case (an `internal` register must redact exactly as a `public` one); `dispatch_ref_dropped_and_not_hashed` computes the unkeyed `sha256` of the fixture's clear `dispatch_ref` (a ref in the nonce form, e.g. `assay--x--1@20261006T141502Z.3fa9c01b7d2e`) and asserts neither it nor the clear value appears in the written bytes, and that no `dispatch_ref_sha256` key is written; `private_dispatch_digest_only` runs on a private register once with a verify-wake-v1 record and once with a legacy-shape `verified` record, and asserts the committed bytes hold `dispatch_ref_sha256` equal to the sha256 the test computes from the fixture's clear ref, and do NOT hold the clear value; `private_keeps_clear_prs` asserts a private register keeps `prs` entries in clear; `withheld_uses_canonical_repo` feeds an entry whose `repo` differs only in case from the name in the fake forge's `PullRequest.URL` and asserts the `withheld` digest is the one computed from the forge's spelling; `written_bytes_contain_no_clear_private_repo` greps the committed bytes; `land_as_change_bytes_redacted` does the same on the GitLab land-as-change path; `redacted_before_outbound_scan` uses an operator token map naming the private repo and expects the record rewritten and landed, not refused; the two `legacy_verified_*` subtests use a legacy-shape `verified` fixture (no `wake_schema`) on a non-private register — one carries a non-public `prs` entry, the other carries `dispatch_ref` and no `prs` — and each asserts the committed bytes hold neither the clear value nor, for `dispatch_ref`, its unkeyed `sha256` | check:ci +mutation |
| 6 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b27-sg.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b27-sg.out"` | exit 0; the test reads a record carrying every new key and derives the same latest outcome as without them | check:ci +flow |
| 7 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v > "${TMPDIR:-/tmp}/b27-r7.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_RunMetadata/missing_verifier_tier_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/vendor_model_name_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/tier_disagreeing_with_admitted_binding_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/started_after_finished_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/row_results_count_mismatch_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/unknown_result_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/dispatch_ref_bad_shape_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/input_dispatch_digest_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/valid_with_and_without_optionals' "${TMPDIR:-/tmp}/b27-r7.out")" -eq 9` | exit 0 (the command asserts the count 9). `dispatch_ref_bad_shape_refused` feeds refs with no nonce, an 11-character and an uppercase nonce, an extended-form timestamp, a prefix containing `@`, a space and a control byte, and a 227-byte prefix, each on a private and a non-private register and on a verify-wake-v1 and a legacy-shape record, and expects exit 5 naming `dispatch_ref` with no forge read and no record written; a 226-byte prefix is accepted. `input_dispatch_digest_refused` feeds an input carrying `dispatch_ref_sha256` and expects exit 5 | check:ci +mutation |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |
| 9 | `jq -r 'select(.prs) \| .prs[] \| select(.repo) \| "\(.repo) \(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json` (on main, after the first post-merge record lands whose `prs` has a CLEAR entry — verified records carry `prs` too, and on this public register an entry that is only `{withheld}` prints nothing, so a record whose only PR is non-public does not satisfy the precondition; run it again after the next one) | at least one line; for one printed `<repo> <number> <sha>` triple, `gh api repos/<repo>/pulls/<number> --jq '.merged, .merge_commit_sha'` prints `true` and that `<sha>` | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A private repo name or PR number lands in a public OR internal register (a `== "public"` redaction test) | row 5 `internal_register_redacts`, `unrecognised_visibility_spelling_redacts`, and the committed-bytes grep |
| Redaction runs only inside the verify-wake-v1 branch, so the usual legacy-shape `verified` record commits a clear private PR reference to a non-private register | row 5 `legacy_verified_non_public_pr_withheld` |
| A claim key with a private alias lands in a non-private register via `dispatch_ref` (clear or unkeyed hash) | row 5 `dispatch_ref_dropped_and_not_hashed`, and, for a legacy-shape `verified` record, `legacy_verified_dispatch_ref_dropped` |
| A clear `dispatch_ref` is committed to a PRIVATE register — a file in git, against desk-supervision/28's Visibility rule — for either record shape | row 5 `private_dispatch_digest_only` (both shapes) |
| A caller supplies its own `dispatch_ref_sha256`, so the committed digest is not the writer's | row 7 `input_dispatch_digest_refused` |
| A malformed `dispatch_ref` (no nonce, whitespace, an embedded `@`, an oversize prefix) is hashed or dropped instead of refused, or is checked on one register kind or record shape only | row 7 `dispatch_ref_bad_shape_refused` |
| A malformed `repo`, a non-positive `number` or a short `merge_sha` reaches an authenticated forge read path | row 3 `malformed_entry_no_read` |
| A record from a caller that is not admitted still drives forge reads, because the reorder put record-driven reads ahead of admission | row 3 `not_admitted_no_read` |
| A case variant of a repository name gives a different `withheld` digest and breaks the operator's join | row 5 `withheld_uses_canonical_repo` |
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
