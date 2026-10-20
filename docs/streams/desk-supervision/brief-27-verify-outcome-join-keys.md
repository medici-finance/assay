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
version: 2
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
  verify-wake-v1 `verified` record (Task 2), the key-uniqueness input rule (Task 2), the redaction
  rules for private and non-private
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
  input-shape refusals (key uniqueness first), the reordered admission and forge handle, the forge
  dereference, the redaction behind a package-level function variable (as `forgeForFn` and
  `publicRepoGateFn` already are) so a test can substitute the rewrite, and step 5's TARGET-TIE
  check (Tasks 2–5).
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
  (`VerifierReceipt.CheckEvidenceTarget`). Today the record admitted, the record named and the
  bytes committed are one record. In the order below they are not: the future-`ts` check and
  admission run on the SUBMITTED record, while the file is named and written from the REWRITTEN
  one. Nothing about the rewrite is assumed to keep `brief` or `ts`: the record readers decode by
  struct (`ParseRecord`, `verifyoutcomes.go`), which matches a key in any letter case and keeps the
  last match, so a record carrying one key twice, or in two letter cases, can resolve to one value
  as submitted and another after a rewrite that round-trips through a key map. The TARGET-TIE
  rule below (steps 1 and 5) closes that. Task 4's forge reads are driven by the record, so they
  need the token and the forge handle that today come after `RecordName`. The order this brief
  requires, stated once here and referenced by Tasks 2–4:
  1. parse; then key uniqueness, the first half of the TARGET-TIE rule: a JSON object anywhere in
     the input (the top level, a `prs` entry, a `row_results` entry or any other nested object)
     whose keys repeat, or differ only in letter case, is refused naming the key, before the
     future-`ts` check reads `ts`; then the future-`ts` check (as today), then every other
     input-shape refusal of Task 2 — all pure checks, exit 5, before ANY forge read (admission's
     own reads included);
  2. admission (`admitVerifierEvidence`), on the input record's `<stream>/<NN>` target, BEFORE any
     record-driven forge read: a caller that is not admitted triggers no forge read the record
     names (no PR, ancestry, default-branch, `blocker_ref` or visibility read);
  3. the identity gate, the token mint and the forge handle (moved ahead of `RecordName`);
  4. the record-driven forge reads — Task 3's dereference and Task 4's visibility reads — then
     Task 4's redaction;
  5. `RecordName` on the REDACTED record and its path-prefix guard, then the TARGET-TIE check: the
     rewritten record's `brief` and `ts` values must equal, byte for byte, the ones steps 1–2
     checked, and `CheckEvidenceTarget` is run again, on the FINAL path, against the receipt step 2
     admitted. Any mismatch or refusal is exit 5 with nothing written and no change landed. Then
     everything that follows today (size cap, immutability compare, outbound scan, the public-repo
     gate, `bodyDig`, the closure gates, `validateReceipt`, the write or
     `landOutcomeRecordAsChange`), all of which see the redacted record.
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
   - **Input shape, checked first.** These are pure checks on the parsed input, run in step 1 of the
     facts' step order (key uniqueness right after the parse, the rest after the future-`ts` check):
     before admission, before any forge read and before any hashing. Each failure is exit 5 naming
     the field, on every register kind (private or not) and every record shape (verify-wake-v1 or
     legacy-shape):
     - no JSON object anywhere in the input repeats a key or carries two keys that differ only in
       letter case (the facts' step 1, the first half of the TARGET-TIE rule; it runs right after
       the parse, ahead of the future-`ts` check); the refusal names the key. This also stops a
       `prs` or `dispatch_ref` key in another letter case from being read by the struct decode and
       then missed by Task 4's redaction;
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
   and every later step see only its output, and that output is held to step 5's TARGET-TIE check
   (`brief` and `ts` unchanged from what steps 1–2 checked). Redaction is an ALLOW-LIST on live
   visibility, never a
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
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v > "${TMPDIR:-/tmp}/b27-r3.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_PRsDereference/not_found_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/unmerged_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/merge_sha_mismatch_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/same_repo_not_ancestor_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/cross_repo_checked_against_default_tip' -e '--- PASS: TestOutcomeRecord_PRsDereference/cross_repo_not_found_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/same_repo_makes_no_default_branch_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/forge_error_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/empty_merge_commit_exit_6' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_uses_base_ref_and_merged_at' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_wrong_base_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/gitlab_compare_error_not_downgraded' -e '--- PASS: TestOutcomeRecord_PRsDereference/legacy_verified_with_prs_dereferenced' -e '--- PASS: TestOutcomeRecord_PRsDereference/input_withheld_entry_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/malformed_entry_no_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/not_admitted_no_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/case_variant_key_no_read' -e '--- PASS: TestOutcomeRecord_PRsDereference/rewritten_target_refused' -e '--- PASS: TestOutcomeRecord_PRsDereference/valid_accepted' "${TMPDIR:-/tmp}/b27-r3.out")" -eq 19` | exit 0 (the command asserts the count 19) (`cross_repo_not_found_exit_6` has the fake answer not-found for an entry in another repository and expects exit 6 naming the entry, not the "does not exist" refusal, then the same answer for the register's own repository spelled in another letter case and expects the exit-5 refusal; `same_repo_makes_no_default_branch_read` expects no `RepoDefaultBranch` read for a same-repo GitHub entry; `malformed_entry_no_read` feeds a `repo` that is not `<owner>/<name>`, a `number` of 0 and of -1, and a short and an uppercase `merge_sha`, on a private and a public register, and expects exit 5 naming the field with the fake forge recording ZERO reads; `not_admitted_no_read` feeds a well-formed record whose admission fails and expects the admission refusal with no `GetPullRequest`, `CompareRefs`, `RepoDefaultBranch`, `blocker_ref` or `RepoVisibility` read; `case_variant_key_no_read` feeds records whose two `brief` keys differ only in letter case and name two different briefs (one of them the brief the run is admitted for), a record whose `ts` key repeats with a second, future value, and records carrying `prs` beside `PRS` and `dispatch_ref` beside `Dispatch_Ref`, on a private and a public register and on a verify-wake-v1 and a legacy-shape record, and expects exit 5 naming the key, with ZERO forge reads (admission's included) and no record written; `rewritten_target_refused` bypasses step 1 by substituting the step-4 rewrite (the test sets the redaction's function variable) with one that changes the admitted record's `brief` to another brief's key, and, as a second case, one that changes only its `ts`, and expects exit 5 from step 5's TARGET-TIE check with no record written and no change landed (both subtests run their cases inside the one subtest body, with no nested `t.Run`, so the count stays exact); `gitlab_*` run against a GitLab-backed fake whose `CompareRefs` returns could-not-check; `gitlab_compare_error_not_downgraded` shows a GitHub-kind forge's `CompareRefs` error stays exit 6 rather than falling to the GitLab rule; `input_withheld_entry_refused` feeds `prs:[{"withheld":"<64 hex>"}]` with no forge read, once on a verify-wake-v1 record and once on a legacy-shape `verified` record, and expects exit 5 both times) | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v > "${TMPDIR:-/tmp}/b27-r4.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_VerifiedRules/legacy_verified_no_new_requirement' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_without_prs_refused' -e '--- PASS: TestOutcomeRecord_VerifiedRules/wake_v1_verified_complete_accepted' "${TMPDIR:-/tmp}/b27-r4.out")" -eq 3` | exit 0 (the command asserts the count 3; `legacy_verified_no_new_requirement` asserts that a legacy-shape `verified` record with no `prs` and no `dispatch_ref` still lands without a receipt, and says nothing about its bytes passing through unchanged) | check:ci +neighbour |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_NonPrivateRedaction$' -v > "${TMPDIR:-/tmp}/b27-r5.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_pr_on_public_register_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/internal_register_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/unrecognised_visibility_spelling_redacts' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/padded_uppercase_private_register_keeps_clear' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_repo_internal_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/register_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/entry_visibility_read_error_exit_6' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/dispatch_ref_dropped_and_not_hashed' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_keeps_clear_prs' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/private_dispatch_digest_only' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/withheld_uses_canonical_repo' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/written_bytes_contain_no_clear_private_repo' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/land_as_change_bytes_redacted' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/redacted_before_outbound_scan' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_non_public_pr_withheld' -e '--- PASS: TestOutcomeRecord_NonPrivateRedaction/legacy_verified_dispatch_ref_dropped' "${TMPDIR:-/tmp}/b27-r5.out")" -eq 16` | exit 0 (the command asserts the count 16). `internal_register_redacts` is the allow-list case (an `internal` register must redact exactly as a `public` one); `dispatch_ref_dropped_and_not_hashed` computes the unkeyed `sha256` of the fixture's clear `dispatch_ref` (a ref in the nonce form, e.g. `assay--x--1@20261006T141502Z.3fa9c01b7d2e`) and asserts neither it nor the clear value appears in the written bytes, and that no `dispatch_ref_sha256` key is written; `private_dispatch_digest_only` runs on a private register once with a verify-wake-v1 record and once with a legacy-shape `verified` record, and asserts the committed bytes hold `dispatch_ref_sha256` equal to the sha256 the test computes from the fixture's clear ref, and do NOT hold the clear value; `private_keeps_clear_prs` asserts a private register keeps `prs` entries in clear; `withheld_uses_canonical_repo` feeds an entry whose `repo` differs only in case from the name in the fake forge's `PullRequest.URL` and asserts the `withheld` digest is the one computed from the forge's spelling; `written_bytes_contain_no_clear_private_repo` greps the committed bytes; `land_as_change_bytes_redacted` does the same on the GitLab land-as-change path; `redacted_before_outbound_scan` uses an operator token map naming the private repo and expects the record rewritten and landed, not refused; the two `legacy_verified_*` subtests use a legacy-shape `verified` fixture (no `wake_schema`) on a non-private register — one carries a non-public `prs` entry, the other carries `dispatch_ref` and no `prs` — and each asserts the committed bytes hold neither the clear value nor, for `dispatch_ref`, its unkeyed `sha256` | check:ci +mutation |
| 6 | `cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v > "${TMPDIR:-/tmp}/b27-sg.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes_AdditiveKeys' "${TMPDIR:-/tmp}/b27-sg.out"` | exit 0; the test reads a record carrying every new key and derives the same latest outcome as without them | check:ci +flow |
| 7 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v > "${TMPDIR:-/tmp}/b27-r7.out" 2>&1 && test "$(grep -c -F -e '--- PASS: TestOutcomeRecord_RunMetadata/missing_verifier_tier_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/vendor_model_name_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/tier_disagreeing_with_admitted_binding_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/started_after_finished_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/row_results_count_mismatch_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/unknown_result_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/dispatch_ref_bad_shape_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/input_dispatch_digest_refused' -e '--- PASS: TestOutcomeRecord_RunMetadata/valid_with_and_without_optionals' "${TMPDIR:-/tmp}/b27-r7.out")" -eq 9` | exit 0 (the command asserts the count 9). `dispatch_ref_bad_shape_refused` feeds refs with no nonce, an 11-character and an uppercase nonce, an extended-form timestamp, a prefix containing `@`, a space and a control byte, and a 227-byte prefix, each on a private and a non-private register and on a verify-wake-v1 and a legacy-shape record, and expects exit 5 naming `dispatch_ref` with no forge read and no record written; a 226-byte prefix is accepted. `input_dispatch_digest_refused` feeds an input carrying `dispatch_ref_sha256` and expects exit 5 | check:ci +mutation |
| 8 | `D=$(git rev-parse "6df93b442c5a^{commit}") && B=$(git rev-parse "4696d598e588^{commit}") && o=$(statusgen --root . --consumers --brief desk-supervision/27 --delivery "$D" --base "$B"); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 6 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |
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
| The record named and written is not the record admitted: a key repeated or in two letter cases resolves to one `brief` or `ts` as submitted and another after the rewrite, so an admitted run lands under another brief, or with a `ts` the future-`ts` check would have refused | row 3 `case_variant_key_no_read` (refused at step 1, before admission) and `rewritten_target_refused` (step 5's TARGET-TIE check with step 1 bypassed) |
| A `prs` or `dispatch_ref` key in another letter case is read by the struct decode but missed by the exact-key redaction, so its clear value is committed | row 3 `case_variant_key_no_read` |
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
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at c690023a864b (9/9 rows pass; row 1 needed a fast-git clone of the same commit, timing fixture tracked in medici-finance/assay#2776)

Verification-Attestation: medici-finance/assay#2920 run=96cd611dd15be504852fad6197bd49ac06faa918007c1e5bb23d6ef34eb04d79 source=c690023a864b1c1cfdd55c0a28e19d22e572048d brief=docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md model=claude-opus-5-5 tier=strong

Fresh non-implementer pass on merged main c690023a864b (implementing change: medici-finance/assay#2717, squash 6df93b442c5a). Pre-work admission (deskdispatch check-verifier) returned the binding above, exit 0. Every row ran in the attested detached home, with go1.27.2 and git 2.56.0, except as noted for row 1. In the home, row 1 exits 1 on one unrelated deskkit subtest, Test Guard Stop Flags / "HEARTBEAT exactly at boundary". That fixture leaves a 1-second margin. A single git call in the home's checkout takes about 1.3 s on this host, so the margin runs out. Re-running the identical row-1 command at the same commit, in a separate clone with fast git, exits 0. CI build-test is also green at c690023a864b. This is tracked as medici-finance/assay#2776. It does not touch any code this brief changed: every deskevidence package test passed in both runs. Row 8's corroboration tool exits 0 but judges nothing, because the brief's consumers lines predate the implementing diff. I checked each routing by hand against that diff (see row 8).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | cd tools/desk && go test ./cmd/deskevidence/... ./internal/deskkit/... | pass exit=0 | Run in a fast-git clone at c690023a864b: ok deskevidence 17.3s, ok deskkit 277.3s, ok deskkit/untrustcorpus. In the attested home the same command exits 1. The only failure is Test Guard Stop Flags / HEARTBEAT exactly at boundary: "HEARTBEAT stale (age 24h0m0s > 24h0m0s)". That is a 1 s timing fixture, tracked as medici-finance/assay#2776; deskevidence passed there too, ok 261.3s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 2 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v, then row 2's grep of the 3 named subtests and its wc -l -eq 3 gate | pass exit=0 | 3 hits: PASS legacy_verify_fail_names_wake_schema, PASS missing_blocker_kind_names_blocker_kind, PASS missing_receipt_id_names_receipt_id; ok deskevidence 4.3s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 3 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v, then row 3's grep -c gate over the 19 named subtests (-eq 19) | pass exit=0 | grep count 19, zero FAIL lines; subtests include not_admitted_no_read, case_variant_key_no_read, rewritten_target_refused, gitlab_compare_error_not_downgraded; ok deskevidence 39.5s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 4 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v, then row 4's grep -c gate (-eq 3) | pass exit=0 | PASS legacy_verified_no_new_requirement, PASS wake_v1_verified_without_prs_refused, PASS wake_v1_verified_complete_accepted; ok deskevidence 2.6s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 5 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_NonPrivateRedaction$' -v, then row 5's grep -c gate over the 16 named subtests (-eq 16) | pass exit=0 | grep count 16, zero FAIL lines; subtests include internal_register_redacts, private_dispatch_digest_only, redacted_before_outbound_scan, legacy_verified_non_public_pr_withheld; ok deskevidence 16.2s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 6 | cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v, then row 6's grep -F of its PASS line | pass exit=0 | PASS: Test Verify Outcomes Additive Keys (0.01s); ok statusgen 0.29s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 7 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v, then row 7's grep -c gate over the 9 named subtests (-eq 9) | pass exit=0 | grep count 9; subtests include tier_disagreeing_with_admitted_binding_refused, dispatch_ref_bad_shape_refused, input_dispatch_digest_refused; ok deskevidence 21.6s | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 8 | statusgen --consumers --root . | pass exit=0 | I used statusgen built from source at c690023a864b. The literal run exits 0 with "no brief files in the diff ... nothing to corroborate". Re-run on the implementing diff (head 6df93b442c5a, base 4696d598e588, --brief desk-supervision/27): exit 0, "0 corroborated, 0 disproved, 11 unchecked". All 11 are unchecked because the consumers lines predate that diff, so I checked them by hand against its file list. 6 follow-up paths are edited: verify-wake-v1.md, verifywake.go, outcomerecord.go, the verify-desk SKILL.md, tools/desk README.md and the verify-outcomes README.md. receiptvalidate.go is not edited; its logic landed in the new outcomejoin.go and verifyoutcomejoin.go. All 4 out-of-scope paths are untouched. | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 9 | jq -r 'select(.prs) \| .prs[] \| select(.repo) \| "\(.repo) \(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json; then gh api repos/medici-finance/assay/pulls/1999 --jq '.merged, .merge_commit_sha' | pass exit=0 | jq printed one line: medici-finance/assay 1999 2a6460e85c784b072e8a44ee1ba16e03756703bb, from the cellctl-windows/00 record of 2026-10-13. gh api printed true and 2a6460e85c784b072e8a44ee1ba16e03756703bb, exit 0. The precondition holds: a post-merge record with a clear prs entry exists. | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |

Risk-bearing value. The trigger does not fire on this brief: its risk frontmatter is present and all four answers are no, it is not irreversible, and nothing it changes is pinned as hard. I enumerated anyway, over the implementing diff's two new code files (the deskkit join file and the deskevidence join file) and the changed verifywake.go. Literals found: the dispatch_ref grammar, the repo-part, merge_sha and number lexeme bounds, the prs entry cap, the visibility allow-list words, the compare status words, the withheld digest input, and the closed sets. The top-ranked entries are the redaction allow-list and the dispatch_ref bound. A wrong value there could write a private identifier into a committed public register. That is reversible by edit and redeploy, but git history keeps it.

- RISK-VALUE: DERIVED — private-register test: normVisibility(regVis) == "private" @ tools/desk/cmd/deskevidence/outcomejoin.go:72, after lower-casing and trimming @ tools/desk/cmd/deskevidence/outcomejoin.go:49 — the register keeps clear prs only on an exact match with "private". Every other value (public, internal, any odd spelling) falls through to the redacting branch. That is the allow-list shape the brief's Task 4 requires, and the same normalisation as the existing PublicRepoGate.
- RISK-VALUE: DERIVED — entry clear test: normVisibility(v) == "public" @ tools/desk/cmd/deskevidence/outcomejoin.go:87 (and :79 for a same-repo entry, which reuses the register's visibility) — on a non-private register an entry stays clear only when its own repo reads exactly "public". Otherwise it is written only as withheld = sha256("<canonical repo>#<number>@<merge_sha>") @ tools/desk/cmd/deskevidence/outcomejoin.go:209, the exact hash input Task 4 specifies. The canonical repo name comes from the forge's change URL.
- RISK-VALUE: DERIVED — dispatchRefRe = `^[\x21-\x3F\x41-\x7E]{1,226}@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:32 — the suffix is 30 bytes: "@" + 8 + "T" + 6 + "Z" + "." + 12. That gives 226 + 30 = 256, desk-supervision/28's string cap. The 12 lowercase hex nonce matches brief 28's 48-bit nonce definition. The character class leaves out 0x40 ("@") and everything at or below 0x20, so the prefix can carry no "@", whitespace or control byte.
- RISK-VALUE: DERIVED — prShaRe = `^([0-9a-f]{40}|[0-9a-f]{64})$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:26 — these are the full object-name lengths for SHA-1 and SHA-256 repositories, which is Task 2's rule.
- RISK-VALUE: DERIVED — CompareRefs status accepted = "identical" or "ahead" @ tools/desk/cmd/deskevidence/outcomejoin.go:195, called as CompareRefs(repo, base=merge_sha, head) — GitHub's compare reports how head stands relative to base. "ahead" or "identical" means merge_sha is an ancestor of, or equal to, head (the record's sha, or the other repo's default-branch tip). That is the ancestry rule in Task 3.
- RISK-VALUE: NAMED, NOT DERIVED — prRepoPartRe = `^[A-Za-z0-9._-]{1,100}$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:25 — the brief pins 1–100. I did not check it against GitHub and GitLab's real owner and name length limits. It only shapes input; a too-tight bound refuses a record rather than leaking anything, and an edit fixes it.
- RISK-VALUE: NAMED, NOT DERIVED — intLexemeRe = `^(0|[1-9][0-9]{0,8})$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:27 (number at most 9 digits) and the prs cap of 64 entries @ tools/desk/internal/deskkit/verifyoutcomejoin.go:169 — the implementer added both; the brief states neither. Both only refuse input and can be undone by an edit. The brief has no derivation to check them against.

**VERIFY: PASS** — 9/9 rows pass at c690023a864b. Row 1 passed in a fast-git clone of the same commit. In the attested home it hits an environment-timing fixture unrelated to this brief (environment; medici-finance/assay#2776). Gate is model, so no human sign-off is needed.

### Non-implementer verifier run — 2026-10-16 claude-opus-5-5 verifier at 446ceaa76f9f (9/9 rows pass, all in the attested home)

Verification-Attestation: medici-finance/assay#3125 run=f6db4d63ddb83ee42615a0c41b1cd6a242e5ee2be7f3362b706732797eca769a source=446ceaa76f9fc6abcc6135b98743e00da8aa99be brief=docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md model=claude-opus-5-5 tier=any

Fresh non-implementer pass on merged main 446ceaa76f9f (implementing change: medici-finance/assay#2717, squash 6df93b442c5a). Pre-work admission (deskdispatch check-verifier) returned the binding above, exit 0, before any row and again before this block was handed back. Every row ran in the attested detached home with go1.27.2 and git 2.56.0, offline, with a throwaway HOME and TMPDIR and the real Go caches. Expected outcomes were taken from this brief's Verify table before any test output was read. This time row 1 passed in the home itself; the timing fixture noted in the earlier run did not trip. Row 8's literal run exits 0 but judges nothing (no brief file in the diff against main), so it was also run on the implementing diff and each routing checked by hand.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | cd tools/desk && go test ./cmd/deskevidence/... ./internal/deskkit/... | exit 0 | exit=0; ok deskevidence 26.8s, ok deskkit 268.1s, ok deskkit/untrustcorpus 0.1s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_FailWithoutReceipt$' -v, then row 2's grep of the 3 named subtests and its wc -l -eq 3 gate | exit 0, 3 hits | exit=0; 3 hits: PASS legacy_verify_fail_names_wake_schema, PASS missing_blocker_kind_names_blocker_kind, PASS missing_receipt_id_names_receipt_id; ok deskevidence 0.18s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_PRsDereference$' -v, then row 3's grep -c gate over the 19 named subtests (-eq 19) | exit 0, count 19 | exit=0; grep count 19 of 19 subtest PASS lines, zero FAIL lines; ok deskevidence 0.60s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_VerifiedRules$' -v, then row 4's grep -c gate (-eq 3) | exit 0, count 3 | exit=0; PASS legacy_verified_no_new_requirement, PASS wake_v1_verified_without_prs_refused, PASS wake_v1_verified_complete_accepted; ok deskevidence 0.19s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_NonPrivateRedaction$' -v, then row 5's grep -c gate over the 16 named subtests (-eq 16) | exit 0, count 16 | exit=0; grep count 16, zero FAIL lines; ok deskevidence 0.28s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | cd statusgen && go test . -run '^TestVerifyOutcomes_AdditiveKeys$' -v, then row 6's grep -F of its PASS line | exit 0 | exit=0; --- PASS: TestVerifyOutcomes_AdditiveKeys (0.01s); ok statusgen 0.29s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecord_RunMetadata$' -v, then row 7's grep -c gate over the 9 named subtests (-eq 9) | exit 0, count 9 | exit=0; grep count 9; ok deskevidence 0.29s | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | statusgen --consumers --root . | exit 0 | exit=0 with the installed statusgen v1.0.45 and with statusgen built from source at 446ceaa76f9f: "no brief files in the diff against 446ceaa76f9f ... nothing to corroborate". The source build, re-run in a scratch clone at 6df93b442c5a with base 4696d598e588 and brief desk-supervision/27: exit 0, "0 corroborated, 0 disproved, 11 unchecked" (each consumers line predates that diff). Hand check against that diff: the 6 follow-up paths verify-wake-v1.md, verifywake.go, outcomerecord.go, the verify-desk SKILL.md, the tools/desk README.md and the verify-outcomes README.md are edited; receiptvalidate.go is not, its new checks sit in the new outcomejoin.go and verifyoutcomejoin.go; the 4 out-of-scope code and doc paths are untouched, and no existing register record is changed | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | jq -r 'select(.prs) \| .prs[] \| select(.repo) \| "\(.repo) \(.number) \(.merge_sha)"' docs/streams/verify-outcomes/*/*.json; then gh api repos/medici-finance/assay/pulls/N --jq '.merged, .merge_commit_sha' for two printed triples | at least one line; one triple dereferences to true and its sha | jq exit=0, 16 lines. gh api for medici-finance/assay 2366 printed true and 0333e8e14adfc4c19639372c05c3f5b7296e8145, exit 0; for medici-finance/assay 2717 printed true and 6df93b442c5abeffcf57ea1576b8acb932c00556, exit 0. Both match the printed triples | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Risk-bearing value. The trigger does not fire: the risk frontmatter is present with all four answers no, the brief is not irreversible, and nothing it changes is pinned as hard. I enumerated anyway, over the two new join files and the changed verifywake.go at 446ceaa76f9f. Literals: the dispatch_ref grammar, the repo-part, merge_sha and number lexeme bounds, the sha256 hex shape, the prs entry cap of 64, the visibility allow-list words, the compare status words and the withheld digest input. The top-ranked entries are the redaction allow-list and the dispatch_ref bound, because a wrong value could commit a private identifier to a public register, which git history keeps even after an edit.

- RISK-VALUE: DERIVED — private-register test: normVisibility(regVis) == "private" @ tools/desk/cmd/deskevidence/outcomejoin.go:72, with normVisibility = lowercase of the trimmed value @ tools/desk/cmd/deskevidence/outcomejoin.go:49 — clear prs are kept only on an exact "private"; public, internal and any other spelling take the redacting branch. That is the allow-list Task 4 requires.
- RISK-VALUE: DERIVED — entry clear test: normVisibility(v) == "public" @ tools/desk/cmd/deskevidence/outcomejoin.go:87 (and :79 for a same-repo entry) — on a non-private register an entry stays clear only when its own repo reads exactly "public"; otherwise it is written as sha256(canonical repo + "#" + number + "@" + merge_sha) @ tools/desk/cmd/deskevidence/outcomejoin.go:209, the hash input Task 4 names.
- RISK-VALUE: DERIVED — dispatchRefRe = `^[\x21-\x3F\x41-\x7E]{1,226}@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:32 — the suffix is 1 + 8 + 1 + 6 + 1 + 1 + 12 = 30 bytes, so 226 + 30 = 256, desk-supervision/28's string cap; 12 lowercase hex is its 48-bit nonce; the class leaves out 0x40 and every byte at or below 0x20 and 0x7F, so the prefix holds no "@", whitespace or control byte.
- RISK-VALUE: DERIVED — prShaRe = `^([0-9a-f]{40}|[0-9a-f]{64})$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:26 — the full SHA-1 and SHA-256 object-name lengths, Task 2's rule.
- RISK-VALUE: DERIVED — compare status accepted = "identical" or "ahead" @ tools/desk/cmd/deskevidence/outcomejoin.go:195, from CompareRefs(repo, merge_sha, head) — the forge's compare reports head relative to base, so ahead or identical means merge_sha is an ancestor of or equal to head; that is Task 3's ancestry rule.
- RISK-VALUE: NAMED, NOT DERIVED — prRepoPartRe = `^[A-Za-z0-9._-]{1,100}$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:25 — the brief pins 1 to 100; I did not check it against the forges' real name limits. It only refuses input, and an edit undoes a wrong bound.
- RISK-VALUE: NAMED, NOT DERIVED — intLexemeRe = `^(0|[1-9][0-9]{0,8})$` @ tools/desk/internal/deskkit/verifyoutcomejoin.go:27 and the prs cap 64 @ tools/desk/internal/deskkit/verifyoutcomejoin.go:169 — the implementer added both and the brief states neither, so there is nothing to derive them from. Both only refuse input and are reversible by an edit.

**VERIFY: PASS** — 9/9 rows pass at 446ceaa76f9f, all in the attested home. Gate is model, so no human sign-off is needed.

## Review
Gate: model (from frontmatter — all four risk answers no; `sensitive-data: no` rests on Task 4's
redaction, which row 5 proves). Reviewer records verdict + date in the stream README table.
