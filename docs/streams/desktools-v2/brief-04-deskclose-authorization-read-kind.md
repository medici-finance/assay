---
brief: assay:assay:desktools-v2:04
title: deskclose reads an authorizing comment by its stated kind — retire the kind-less default (#1019)
why: >-
  #1019 reported that deskclose could not work on an issue because the comment read assumed a
  pull request. Most of that is already fixed at head — the superseded lane and the triage lane
  state the target kind. What remains is the authorization read itself: the ruling gate and the
  manifest gate still fetch their authorizing comment through the kind-less ListComments, which
  reads a CHANGE's thread on both backends. A human ruling recorded on an ISSUE therefore comes
  back as could-not-check every time, and deskclose closes nothing. The permalink already says
  which kind it names; the read should use it. Small, self-contained, and it lets #1019 close.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1019]
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session; re-derived at head 2026-09-17
sources:
  - "docs/streams/desktools-v2/spec.md §1 (#1019 row), §3 (commitment 5) — migrate-and-remove"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the deskclose rows"
  - "#1019 — its reported root cause is GitHubForge.ListComments hardcoding the pullRequest selection; its reported symptom is `deskclose superseded` failing on an issue"
  - "tools/desk/internal/deskkit/forge_github.go:1251-1268 — ListComments is now ListCommentsTyped with TargetChange; ListCommentsTyped picks the issue or the pullRequest selection by kind"
  - "tools/desk/cmd/deskclose/authority.go:131,:139,:146,:169-173 — fetchComment passes kind \"\" and takes the kind-less branch; fetchCommentTyped exists and is used by the triage lane; :248 (authorize) and :276 (authorizeManifest) are the two callers still on the kind-less path"
  - "tools/desk/cmd/deskclose/authority.go:63-66 — commentURLRe already captures the permalink's `issues` or `pull` path segment"
  - "freshness-checked 2026-09-17 @ 57509073 — neither authority.go nor triage.go constructs any query (the word pullRequest appears there only in two explanatory comments, authority.go:135 and triage.go:344); the earlier draft of this brief said a query literal lived in those files, and that was wrong. kind_test.go:151 TestTypedIssueReferenceReachesTheIssue already pins the typed issue path"
consumers:
  - "tools/desk/cmd/deskclose/authority.go: follow-up desktools-v2/04 (this brief; the two authorization reads state a kind and the kind-less branch is DELETED — flips to fixed-here when the implementation lands)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (ListCommentsTyped already exists with both backends; this brief consumes it and adds no operation)"
  - "deskboard, deskprovenance, deskdisposition, deskreply (the other four kind-less ListComments callers): out-of-scope (each reads a thread it already knows is a change — a PR's comments — so the default is correct there; they are recorded in the inventory, not changed here)"
exec-tier: strong
exec-tier-why: >-
  question (c) — this is the read that establishes a human AUTHORIZED a close. A subtle error
  (accepting a comment from a different item that shares the number, or treating a
  could-not-check as a pass) survives a happy-path test.
domain: complicated
version: 1
id: b71f42a9-71c1-414b-a326-a9d5b2bdea8f
---

# Brief 04 — deskclose reads an authorizing comment by its stated kind

## Context

files:
- `tools/desk/cmd/deskclose/authority.go` — `fetchComment` / `fetchCommentKinded`, and the two
  callers `authorize` and `authorizeManifest`.
- `tools/desk/cmd/deskclose/authority_kind_test.go` (planned) — the tests below.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

single-point-of-failure: the control that matters is "the authorizing comment is ON the item
the permalink names" — deskclose lists that item's comments and matches the comment's database
id, so a comment id from elsewhere is refused. That property must survive this change
untouched. The second, independent layer is `verifyHumanAuthor`, which runs on the fetched
comment afterwards and refuses a bot or App author whatever thread it came from. One fails on
WHERE the comment is, the other on WHO wrote it.

facts:
- On GitHub an issue and a pull request share one number sequence but not a GraphQL selection.
  `ListCommentsTyped` returns could-not-check when the number names the other kind — it never
  returns an empty thread for it.
- A comment permalink's path is `/issues/<N>` or `/pull/<N>`. A pull request's comment can
  legitimately be linked under `/issues/<N>` (the forge redirects it), so `/issues/` does not
  PROVE the item is an issue; `/pull/` does prove it is a change.
- Therefore: `/pull/` → read as `TargetChange`. `/issues/` → read as `TargetIssue`; if that
  answers could-not-check because the number names a change, read as `TargetChange`. No third
  path, and never the kind-less default. Trying the second kind is safe because the id match
  above is what authorizes, not the kind.
- A could-not-check from BOTH reads stays could-not-check: deskclose refuses and closes nothing.
- Out of scope: the other four kind-less callers; any query construction; any credential change.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires loosening the id match or `verifyHumanAuthor`: STOP and escalate
  (needs-decision) — those are the authorization controls.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. First, reproduce the residual at head and record it in the PR body: with a recording fake
   forge whose ISSUE thread holds the ruling comment, `authorize` returns could-not-check.
2. Derive the kind from the permalink per `facts:` and read through `ListCommentsTyped`.
3. DELETE the `kind == ""` branch of `fetchCommentKinded` and the kind-less `fetchComment`
   wrapper, so no deskclose path can reach the default again.
4. Add `TestRulingCommentOnAnIssueAuthorizes`, `TestIssuesPathNamingAChangeStillResolves` and
   `TestCommentIdFromAnotherItemIsStillRefused` (the negative-path row: same number, other
   kind, different comment id → refused).
5. State in the PR body whether #1019 can close, and if not, exactly what remains.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskclose/` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 5m ./cmd/deskclose/` | exit 0; the whole deskclose suite passes, so no existing authorization assertion was weakened |
| 3 | `cd tools/desk && go test ./cmd/deskclose/ -run TestRulingCommentOnAnIssueAuthorizes -v` | output contains the literal line `--- PASS: TestRulingCommentOnAnIssueAuthorizes` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0). Fail-first: on the unfixed code this test is RED with could-not-check, quoted in the PR body |
| 4 | `cd tools/desk && go test ./cmd/deskclose/ -run TestCommentIdFromAnotherItemIsStillRefused -v` | output contains the literal line `--- PASS: TestCommentIdFromAnotherItemIsStillRefused` — the negative-path row: widening the read to two kinds did not widen what authorizes |
| 5 | `grep -nE 'fg\.ListComments\(' tools/desk/cmd/deskclose/authority.go; test $? -eq 1` | exit 0 and no line printed — the kind-less call is GONE from the authorization read (a call, not a word in a comment, is what is matched) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskclose/` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -timeout 5m ./cmd/deskclose/` | pass exit=0 | sha256:4e018f0ac11d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskclose/ -run TestRulingCommentOnAnIssueAuthorizes -v` | pass exit=0 | sha256:1877e74b790f | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskclose/ -run TestCommentIdFromAnotherItemIsStillRefused -v` | pass exit=0 | sha256:491cd69dca70 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -nE 'fg\.ListComments\(' tools/desk/cmd/deskclose/authority.go; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier notes — 2026-09-27 opus-5.5-verifier

Run against merged main 9585b4b6cc2ea8d35d367fb912e7c8216a765ba3 from an isolated worktree cut
detached at that head; implementing commit bbc826d59 (#1320). Each row was also run by hand
before the witness run above; the key real output per row:

- Row 1: build + vet exit 0, no output (the witness hash is the empty-output digest).
- Row 2: exit 0; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskclose` (whole suite).
- Row 3: the literal line `--- PASS: TestRulingCommentOnAnIssueAuthorizes` is present (checked on
  the line, not only the exit status the witness records).
- Row 4: the literal line `--- PASS: TestCommentIdFromAnotherItemIsStillRefused` is present.
- Row 5: grep printed nothing, test exit 0 — no kind-less `fg.ListComments(` call is left in
  the authorization read; the only read is `fg.ListCommentsTyped` (authority.go line 186).

Fail-first, re-checked independently: with the kind derivation reverted to the old change-only
read (the default kind in fetchComment set to TargetChange) in this verifier's own worktree,
row 3's test goes RED with `could-not-check: ... carries no pull request at number 298` and
row 4 stays green; the mutation was restored before the witness run (tree clean).

Row 2's "no assertion weakened": the only pre-existing test file the implementing commit
touched is the forge stub, and the change makes its typed comment read STRICTER (a number
that names the other kind now answers could-not-check, matching production) — it removes no
assertion. The two authorization controls (the database-id match in fetchCommentKinded and
verifyHumanAuthor) are unchanged by the diff.

Risk-bearing values — enumeration over the non-test lines of the implementing diff in
authority.go (the test stub, the new test file's fixture numbers, and the changelog fragment
are not behavior-governing):

1. `m[3] == "pull"` @ tools/desk/cmd/deskclose/authority.go:145 — which permalink path segment
   selects a direct change read.
2. `kind := deskkit.TargetIssue` @ tools/desk/cmd/deskclose/authority.go:144 (TargetIssue =
   "issue" @ tools/desk/internal/deskkit/forge.go:896) — the default kind for an `/issues/` link.
3. `kind == deskkit.TargetIssue && deskkit.IsUnverifiable(err)` → retry with
   `deskkit.TargetChange` @ tools/desk/cmd/deskclose/authority.go:149-150 (TargetChange =
   "change" @ tools/desk/internal/deskkit/forge.go:898) — the one fallback binding.

Rank: none is irreversible — a wrong value makes deskclose either refuse (could-not-check,
zero closes) or read another thread, and a close itself is reopenable; each is undone by an
edit and a rebuild. Entry 3 ranks first (it is the only one that widens where a read looks),
then 1, then 2.

RISK-VALUE: DERIVED — fallback binding (TargetIssue could-not-check → retry TargetChange) @ tools/desk/cmd/deskclose/authority.go:149 — the brief's facts establish that an `/issues/` permalink does not prove the object is an issue (the forge redirects a pull request's comment under it), so a second read is needed; it cannot widen what authorizes because fetchCommentKinded authorizes only on a database-id match on the permalink's own item, and a double could-not-check stays could-not-check (row 4 and the mutation above exercise both halves).
RISK-VALUE: DERIVED — `m[3] == "pull"` @ tools/desk/cmd/deskclose/authority.go:145 — commentURLRe (authority.go:66-67) captures owner, repo, (issues|pull), number, comment id as groups 1-5, so group 3 is the kind segment; GitHub never renders an issue's own comment under `/pull/`, so `/pull/` proves a change and a direct TargetChange read is correct with no fallback.
RISK-VALUE: DERIVED — default `kind := deskkit.TargetIssue` @ tools/desk/cmd/deskclose/authority.go:144 — every permalink that is not `/pull/` is `/issues/` (the regex admits only those two), and an `/issues/` link names an issue unless the retry proves otherwise; trying the issue thread first is what reaches the #1019 residual (row 3).

Observations (no defect): (a) Task 3 said to delete "the kind-less fetchComment wrapper"; the
name fetchComment survives, but it is no longer kind-less — it derives the kind and every
path reaches ListCommentsTyped with a stated kind, which is the brief's intent. (b) The
fallback fires on ANY could-not-check from the issue read (a transient read error too), not
only on a kind mismatch; the outcome stays fail-closed (the change read of a real issue
number also answers could-not-check), so the only cost is a second read.

VERIFY: PASS

## Review
Gate: model (all four risk answers no — it changes which thread an existing read looks in, on a
reversible close path; the two authorization controls are unchanged and the negative-path row
pins that). Row 3 dereferences the #1019 residual, row 4 is the negative path, row 5 is the
removal check. Reviewer records verdict + date in the stream README table.
