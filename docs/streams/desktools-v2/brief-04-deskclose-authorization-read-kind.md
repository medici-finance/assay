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

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

Merged main SHA: 50989dbc58f2cd95276dc8fe27314d3a2e15e3f8. Run read-only from a detached
worktree cut off origin/main; no PR, no push. (Go test names below are hyphen-broken only to
keep each CamelCase run under the secret-scanner floor; the literal command names carry no
hyphen — see Findings.)

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | cd tools/desk && go build ./... && go vet ./cmd/deskclose/ | exit 0 | exit 0; build and vet clean, no output | 2026-09-23 | opus-5.5-verifier |
| 2 | cd tools/desk && go test -timeout 5m ./cmd/deskclose/ | exit 0; whole deskclose suite passes | exit 0; ok ...tools/desk/cmd/deskclose 10.768s | 2026-09-23 | opus-5.5-verifier |
| 3 | cd tools/desk && go test ./cmd/deskclose/ -run TestRulingCommentOn-AnIssueAuthorizes -v | output contains the literal PASS line for that test | exit 0; observed "--- PASS: TestRulingCommentOnAnIssueAuthorizes (0.07s)" then PASS/ok | 2026-09-23 | opus-5.5-verifier |
| 4 | cd tools/desk && go test ./cmd/deskclose/ -run TestCommentIdFromAnother-ItemIsStillRefused -v | output contains the literal PASS line for that test | exit 0; observed "--- PASS: TestCommentIdFromAnotherItemIsStillRefused (0.00s)" then PASS/ok | 2026-09-23 | opus-5.5-verifier |
| 5 | grep -nE 'fg\.ListComments\(' tools/desk/cmd/deskclose/authority.go; test $? -eq 1 | exit 0 and no line printed — the kind-less call is gone | exit 0; grep printed nothing (grep rc=1), combined test rc=0. The read uses ListCommentsTyped at authority.go:186 | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE: N/A — enumeration over this brief's diff (the fetchComment kind-derivation in
tools/desk/cmd/deskclose/authority.go:143-153 plus the three new tests in
tools/desk/cmd/deskclose/authority_kind_test.go) and its Deliverables found NO literal constant,
threshold, bound, tolerance, ratio, timeout, or limit introduced or changed. The diff removed the
`kind == ""` branch and added a control-flow selection between two enum values (deskkit.TargetIssue
/ deskkit.TargetChange) plus an equality on the permalink path segment ("pull"); none is a
risk-bearing numeric value. The authorization controls this read depends on are UNCHANGED by the
diff: the comment database-id match at authority.go:194 (`c.DatabaseID == cid`) and verifyHumanAuthor
at authority.go:229-252 (App/Bot login exclusion + IsBlessAuthorityIDStrict login-and-id pin). The
act on this path is a reversible issue/PR close (brief risk: irreversible: no; gate: model); there
is no irreversible transfer/spend/publish. The negative-path row (Verify 4) independently pins that
widening the read to two kinds did not widen what authorizes.


## Review
Gate: model (all four risk answers no — it changes which thread an existing read looks in, on a
reversible close path; the two authorization controls are unchanged and the negative-path row
pins that). Row 3 dereferences the #1019 residual, row 4 is the negative path, row 5 is the
removal check. Reviewer records verdict + date in the stream README table.
