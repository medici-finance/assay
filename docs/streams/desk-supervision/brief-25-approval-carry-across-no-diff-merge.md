---
brief: assay:assay:desk-supervision:25
title: Carry an approval across a merge of main that leaves the PR's diff byte-identical
why: >-
  Every time a PR merges main to stay current, its head moves past its approval and it waits for a
  full model re-review, even when the change it asks to land is byte-for-byte what the reviewer
  already approved. On a busy queue that re-review is pure cost: the verify desk's Evidence PRs pay
  it after every sibling landing, and #1706 paid it on 2026-09-27. Carrying the approval, when tools
  can prove the diff is identical and the only new commits are clean merges of main, removes that
  round without removing a review of any new content.
wave: 0
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: >-
  This relaxes a review gate: a PR whose head moved can be flipped and merged on an approval a model
  gave at an earlier head, with no model reading the new head. A wrong carry lets content no reviewer
  read reach a public default branch, where disclosure cannot be recalled and force-push is denied
  (irreversible). The human confirms that the carry is acceptable at all, which definition of
  "identical" to use, and that the safeguards below are the ones wanted.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
id: 573b35fd-05f0-430a-a520-16a6fdab348e
authored: 2026-09-27 by worker-desk authoring session (#882 ruling, option 2)
exec-tier: strong
exec-tier-why: >-
  (a) the identity predicate and its disqualifiers are a design, not a lookup; (b) the verdict is
  written by one tool, re-derived by another and enforced by the forge, and the three must agree;
  (c) review-gate plumbing where a subtle carry bug survives a happy-path test.
domain: complicated
sources:
  - "medici-finance/assay#882 — the 2026-09-27 driver ruling that selects this option (option 2 of 3, with desk-supervision/24 and desk-supervision/26)"
  - "docs/streams/desk-supervision/review-finding-v1.md:85 — the rule this brief amends"
  - "tools/desk/cmd/deskflip/flip.go — checkReviewerApproved (line 771) and reduceSecurityVerdict (line 1412)"
  - "tools/desk/cmd/deskboard/board.go:1233 — the MERGE-CURR classification"
  - "plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md — approval staleness and the unreliability of a review's commit_id"
  - "live reads 2026-09-27: the default branch's rules, and the timeline, reviews and commits of #1706 (facts)"
  - "freshness-checked 2026-09-27 @ b227b4076 (origin/main): no carry exists; deskflip requires the verdict at the current head"
consumers:
  # Authoring PR: routed to this brief (rule 6); each flips to fixed-here in the implementation
  # commit that edits the path.
  - "tools/desk/internal/deskkit/approvalcarry.go (planned; both carry predicates): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation adds it)"
  - "tools/desk/cmd/deskpost (the carry verb): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/cmd/deskflip (accepts a carry verdict only after its own re-derivation): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/cmd/deskboard (MERGE-CURR becomes CARRY on the same predicate): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "docs/streams/desk-supervision/review-finding-v1.md (line 85): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation amends it)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md and references/merge-time-recheck.md: follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits them)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the just-in-time merge step of the PR lane): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/README.md (deskpost, deskflip, deskboard sections): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (it merges on the forge's own review decision; a carry verdict is a real approval at the current head by the reviewer identity, which that decision already counts, so the lane is unchanged)"
  - "the default branch's rulesets: out-of-scope (unchanged by design; the server layer below depends on them staying as they are, and Verify row 8 reads them back)"
---

# Brief 25 — Carry an approval across a no-diff merge of main

## Context

files:
- **add** `tools/desk/internal/deskkit/approvalcarry.go` (planned) + `approvalcarry_test.go` (planned)
  — two pure predicates, `CarryByPatch` and `CarryByBlobs`, and the verdict body grammar.
- **edit** `tools/desk/cmd/deskpost/` — a `carry` verb + tests.
- **edit** `tools/desk/cmd/deskflip/flip.go` + tests — `checkReviewerApproved` and the security lane.
- **edit** `tools/desk/cmd/deskboard/board.go` + tests — the MERGE-CURR branch of `classify`.
- **edit** `docs/streams/desk-supervision/review-finding-v1.md` (line 85).
- **edit** `plugins/assay/skills/pr-review-desk/SKILL.md`,
  `plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md`,
  `plugins/assay/skills/verify-desk/SKILL.md`, `tools/desk/README.md`.
- **add** `changelog/<branch>.md` — the fragment this repo enforces.

single-point-of-failure: the carry predicate is the ONE control between "the head moved" and "the old approval counts" — layers behind it: (1) the verb that posts a carry verdict derives it from local git objects (patch identity plus a driver-free recompute of every new merge) and refuses on any disqualifier; (2) the flip gate never trusts that verdict: it re-derives eligibility from forge data by a different method (file set and blob identity, plus the merge bases' changed files), so a verdict posted by any path, including one that skipped layer 1, cannot flip a PR whose content changed; (3) the forge's own ruleset (dismiss stale approvals on push, approval of the last push by a non-pusher) dismisses a carry verdict on any later push, so a carry never outlives the head it was derived at. Layers 1 and 2 fail on different signals in different components; layer 3 is server-side and unchanged by this brief.

facts (2026-09-27 @ b227b4076; re-establish from the named files and commands at pickup):
- **The rule being amended.** `review-finding-v1.md:85`: "A finding keeps its id and evidence when
  the head advances; … An approval recorded at an old head is **not** carried across the change."
- **deskflip today.** `checkReviewerApproved` (flip.go:771) accepts only the reviewer App's decisive
  correctness verdict AT the current head; a verdict at an older head is refused as STALE, naming
  both commits. `reduceSecurityVerdict` (flip.go:1412): a `Security-Review: fail` STANDS across any
  head move; a `pass` grants only at the current head.
- **deskboard disagrees with deskflip today.** `classify` (board.go:1233) marks a PR MERGE-CURR, "no
  re-review", when the PR's own files are unchanged since the reviewed sha; deskflip then refuses the
  same PR as STALE. For an Evidence PR the shared outcomes log is one of the PR's own files, so a
  merge of main always classifies RE-REVIEW instead.
- **The server dismisses on push.** Live read of the default branch's rules (2026-09-27): the
  PR-review ruleset carries `dismiss_stale_reviews_on_push: true`,
  `require_last_push_approval: true`, one required approval. #1706: APPROVED at 135f3a344; the
  verifier pushed a merge of main, 86f8b67ee (2026-09-27 16:02:58Z); the timeline records
  `review_dismissed` by the pushing identity at 16:04:03Z; a fresh APPROVED at 86f8b67ee followed
  at 16:13:03Z. So a carried approval must be a NEW review at the new head: the forge will not
  honour the old one, whatever the desk's tools decide.
- **Identity under the shared log, measured on #1706.** `git diff <main>...<head> | git patch-id
  --verbatim` differs between 135f3a344 and 86f8b67ee (default context: the log's context lines
  moved as main appended), and is equal with `-U0`. So a default-context carry never applies to an
  Evidence PR while outcomes share one appended file; once each outcome is its own new file
  (desk-supervision/24) the outcome's diff has no context from main. <!-- graph: not-a-gate -->
  Neither brief depends on the other; this one helps ordinary PRs' keep-current merges on its own.
- **A review's `commit_id` is not a trusted head.** `merge-time-recheck.md`: it has been observed to
  disagree with the head named in the review's own body, and the error is uncharacterised.
- **`git patch-id --verbatim`** hashes the diff without stripping whitespace (plain `--stable`
  ignores whitespace, which would let a whitespace-only change carry); line numbers are ignored.

**"Identical" — the predicate (decided here, option 1 below; option 2 changes only C3's context).**
Inputs: the approved head `A` (the source verdict's head), the current head `H`, the fetched tip of
the base branch `M`. A carry holds only if ALL of these hold; any unreadable input is could-not-check,
which is no carry:

- **C1 — no rewrite.** `A` is an ancestor of `H`. A force-push or any history rewrite fails this.
- **C2 — only merges of main.** Every commit reachable from `H` but not from `A` or `M` has exactly
  two parents, and its second parent is an ancestor of `M`. Any non-merge commit, an octopus merge,
  or a merge of any branch other than the base disqualifies.
- **C3 — the diff is byte-identical.** `git diff --no-ext-diff --no-textconv --no-renames --binary
  M...A | git patch-id --verbatim` equals the same for `M...H` (default context, 3 lines), and the
  two name-status lists are equal. Any change to the diff, including whitespace and file mode,
  disqualifies.
- **C4 — no merge carries content.** For every commit in C2, its tree equals
  `git -c core.attributesFile=/dev/null --attr-source=<empty tree> merge-tree --write-tree` of its two
  parents, and that recompute is clean (exit 0). A conflict resolution, a merge driver's
  resolution, or any content the merge adds of its own disqualifies.
- **C5 — the source is a real, current verdict.** The source is the reviewer identity's decisive
  verdict for the lane (correctness APPROVED; for the security lane a `Security-Review: pass`), it
  is an original review and not itself a carry (a carry always names the original `A`), the head
  its body names equals its `commit_id` equals `A`, and no CHANGES_REQUESTED by the reviewer follows
  it.
- **C6 — the body did not change.** The PR body was not edited after the source verdict. On a
  `gate: human` brief the human signs the body, so a body edit needs a real re-review.
- **C7 — a fail is never carried and never cleared.** A standing `Security-Review: fail` blocks a
  carry of either lane and keeps blocking the flip exactly as today.

## Human decision
<!-- decision-trigger: creation — the options are enumerable now; filed as the brief lands. -->
When a pull request merges the latest default branch into itself to stay current, its newest commit
changes, and today every approval it had stops counting. A model reviewer must read it again before it
can be marked ready, even when what the pull request would change is exactly what was already
approved. The proposal: when tools can prove that the only new commits are clean merges of the default
branch and that the pull request's change is byte-for-byte what was approved, the review desk posts
a mechanical approval at the new commit, which names the approved commit, and no model re-reads it.
The ready gate re-checks that proof itself, by a different method, before it accepts the approval.
The hosting platform still discards any approval the moment another commit is pushed, so a carried
approval never outlives the commit it was checked at. A security-review failure is never carried.
It keeps blocking exactly as it does today.

This needs a human because it relaxes a review gate: content can be marked ready and merged to a
public default branch on an approval a model gave to an earlier commit. The cost of a wrong carry is
content no reviewer read becoming public, which cannot be recalled.

What is being decided: whether to adopt the carry, and how strict "byte-identical" is.

Options:
1. **Adopt, strict identity.** The change must be identical including three lines of surrounding
   context. If the default branch changed anything within three lines of the pull request's own
   changes, there is no carry and a model re-reviews. Recommended: a nearby change is exactly where
   a semantic collision hides. The trade: pull requests whose change sits at the end of one shared,
   appended log file never qualify, because every other landing changes the lines around theirs.
   They qualify once that log is split into one file per record, which is planned separately.
2. **Adopt, changed-lines-only identity.** Only the changed lines themselves must be identical;
   changes the default branch made next to them are ignored. This carries today's appended-log pull
   requests as well, and it misses a nearby change that alters what the pull request's lines mean.
3. **Do not adopt.** Every commit change needs a model re-review, as today.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. The deliverable is a draft PR
  opened by the desk verbs.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- Never change a ruleset or branch protection. The server layer is relied on, not edited; if a live
  row finds it different, that row fails and is reported, never worked around.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Two predicates in deskkit, two methods.**
   - `CarryByPatch(facts)`: C1-C7 over local git facts (ancestry, commit parents, the two patch-ids
     and name-status lists, each merge's tree and its driver-free recompute), gathered by a thin
     adapter that runs the git commands above in a checkout.
   - `CarryByBlobs(facts)`: the same verdict from forge data, without a checkout:
     - (i) the forge's compare of `A...H` reports `H` ahead of `A` and not behind;
     - (ii) the pull request's own file list, with each file's status and head blob id, is equal at
       `A` and at `H`;
     - (iii) the compare between the merge base of `A` with `M` and the merge base of `H` with `M`
       touches none of those files;
     - (iv) every commit between `A` and `H` that is not on `M` has two parents;
     - (v) C5, C6 and C7 as above.

   Both return carry, no-carry (naming the first failed condition), or could-not-check. With
   option 2 ruled, C3 uses `-U0`. `CarryByBlobs` is then weaker than `CarryByPatch` on context, and
   the PR body says so.
2. **The carry verb.** `deskpost carry <owner/repo> <N> --lane correctness|security --from <A>
   --root <checkout>`. It runs `CarryByPatch` at the PR's current head. On carry it posts, as the
   reviewer identity, a correctness APPROVED at `H`, or a security pass at `H` in the existing
   security-lane shape. Either body carries `Approval-Carried-From: <A>`,
   `Carry-Patch-Id: <id>` and `Carry-Merges: <shas>`. On no-carry it exits 5 naming the
   condition; on could-not-check it exits 6. Nothing is posted in either case.
3. **The flip gate.** In `checkReviewerApproved` and the security lane, a verdict at `H` whose body
   carries `Approval-Carried-From:` counts only when `CarryByBlobs` from the cited `A` to `H` returns
   carry at flip time, re-read with the head-stable re-read that already exists. Otherwise the
   refusal names the carry and the failed condition. C7 is unchanged code: the standing-fail
   reduction is not touched.
4. **The board.** `classify`'s MERGE-CURR branch uses `CarryByBlobs`. An eligible PR gets a new
   CARRY action ("post a carry verdict per lane"); anything else is RE-REVIEW.
5. **Amend `review-finding-v1.md:85`** to read, in substance: "An approval recorded at an old head is
   **not** carried across the change, with one exception. When the only new commits are clean merges
   of the base branch and the PR's diff against the base is byte-identical, a carry verdict posted at
   the new head stands in for a re-review. The verdict names the original approval, and the flip gate
   re-derives it independently. A finding is never cleared by a carry, and a security fail is never
   carried." Findings keep their ids and rounds as today.
6. **Skills.**
   - `plugins/assay/skills/pr-review-desk/SKILL.md`: the MERGE-CURR bullet becomes CARRY (run `deskpost carry` for each
     lane that approved, then flip). The Evidence-PR bullet points at the carry for a just-in-time
     merge that qualifies.
   - `plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md`: its approval-staleness paragraph gains the carry
     exception.
   - `plugins/assay/skills/verify-desk/SKILL.md` step 4 of the PR lane: after its just-in-time merge, expect a carry
     rather than a re-review when the merge qualifies.
   - Keep every skill body neutral: no values, no names.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCarryByPatch$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r1-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryByPatch' "${TMPDIR:-/tmp}/b25-r1-1.out"` | exit 0; output contains `--- PASS: TestCarryByPatch`. carry: a clean two-parent merge of main with an identical diff. no-carry, one fixture each, naming the condition: a non-merge commit after `A` (C2); a merge of a non-base branch (C2); an octopus merge (C2); `A` not an ancestor of `H` after a rewrite (C1); main changed a line inside a PR hunk's context (C3); a whitespace-only edit to a PR file (C3, which plain `--stable` would miss); a file-mode change (C3); a conflict resolution in a PR file (C4); a union-driver resolution (C4); a merge that adds a file neither parent has (C4); a CHANGES_REQUESTED after the source (C5); a source whose body head differs from its `commit_id` (C5); a source that is itself a carry (C5, resolved to the original); a body edited after the source (C6); a standing security fail (C7). A missing object is could-not-check | check:ci +mutation |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCarryByBlobs$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r2-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryByBlobs' "${TMPDIR:-/tmp}/b25-r2-1.out"` | exit 0; output contains `--- PASS: TestCarryByBlobs`. The row 1 scenarios expressed as forge data give the same verdicts. `TestCarryPredicatesAgree` (planned) runs both predicates over every shared scenario and fails on any disagreement | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskpost/ -run '^TestCarryVerb$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r3-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryVerb' "${TMPDIR:-/tmp}/b25-r3-1.out"` | exit 0; output contains `--- PASS: TestCarryVerb`. Each disqualifier exits 5 and the stub forge records zero posts; could-not-check exits 6 with zero posts; an eligible PR gets exactly one review at `H` carrying the three body lines; the security lane posts in the existing security shape and refuses when a fail stands | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestFlipCarriedVerdict$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r4-1.out" 2>&1 && grep -F -e '--- PASS: TestFlipCarriedVerdict' "${TMPDIR:-/tmp}/b25-r4-1.out"` | exit 0; output contains `--- PASS: TestFlipCarriedVerdict`. LOWER LAYER WITH THE UPPER BYPASSED: a carry APPROVED at `H` placed directly in the fixture's reviews, never produced by the verb, citing an `A` whose file blobs differ at `H` (a merge that edited a PR file), is refused naming the carry and the failed condition. A valid carry flips. A security pass carried over a standing `Security-Review: fail` at an older head is refused at the security condition. A carry citing a source whose body head differs from its `commit_id` is refused | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestClassifyCarry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r5-1.out" 2>&1 && grep -F -e '--- PASS: TestClassifyCarry' "${TMPDIR:-/tmp}/b25-r5-1.out"` | exit 0; output contains `--- PASS: TestClassifyCarry`. A keep-current merge whose PR files are unchanged but whose merge resolved a PR file with a driver is RE-REVIEW, not CARRY; an eligible one is CARRY; one whose inputs cannot be read degrades to RE-REVIEW | check:ci +neighbour |
| 6 | Live, on a canary PR in this repository (number exported as `CANARY_PR`) after a carry verdict at `H`: push a further merge of main, then `gh api "repos/medici-finance/assay/issues/$CANARY_PR/timeline" --jq '.[] \| select(.event=="review_dismissed") \| .created_at' && deskflip "$CANARY_PR"` | the timeline shows the carry review dismissed after the push, and deskflip refuses until a new carry is posted: a carry never outlives its head | gate:human +mutation |
| 7 | Live: a real PR approved at `A` merges main to an eligible `H`; the review desk runs `deskpost carry`; then `deskflip <N>`; then read the PR's reviews | one carry review at `H` naming `A` and the patch-id, no model review between `A` and the flip, and the flip succeeds | gate:human +flow |
| 8 | `gh api repos/medici-finance/assay/rules/branches/main --jq '[.[] \| select(.type=="pull_request") \| .parameters \| {dismiss_stale_reviews_on_push, require_last_push_approval}]'` | at least one entry reads `{"dismiss_stale_reviews_on_push":true,"require_last_push_approval":true}` — the server layer this design relies on is still in place | check +dereference |
| 9 | `grep -n 'is \*\*not\*\* carried across the change' docs/streams/desk-supervision/review-finding-v1.md && cd tools/desk && go test ./internal/deskkit/ -run '^TestDeriveLedgerCarry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r9-1.out" 2>&1 && grep -F -e '--- PASS: TestDeriveLedgerCarry' "${TMPDIR:-/tmp}/b25-r9-1.out"` | output contains `--- PASS: TestDeriveLedgerCarry`; the amended sentence is present with its exception clause; the ledger test shows a carry verdict clears no finding and a finding opened after `A` blocks the carry | check:ci +dereference |
| 10 | `cd tools/skillslint && go run . --root ../..` | exit 0 | check:ci |
| 11 | `statusgen --consumers --root .` | exit 0 — every routing token above corroborated against the branch diff | check:ci +dereference |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| A merge carries an edit of its own (an "evil merge") and the approval carries over it | row 1 (C4), row 4 (the flip gate's blob check with the verb bypassed) |
| A whitespace or mode change rides a carry | row 1 (C3 with `--verbatim`) |
| History rewritten and an approval carried onto different code | row 1 (C1) |
| The verb's predicate has a bug and posts a bad carry | row 4 (independent re-derivation at flip), row 2 (differential agreement) |
| A carry clears or launders a security fail | rows 1, 3, 4 (C7) |
| A carry outlives a later push | row 6 (server dismissal), row 4 (head-stable re-read) |
| The forge ruleset this rests on is later relaxed by hand | row 8 |
| Board and flip gate disagree again | row 5 (both use `CarryByBlobs`) |
| A carried approval hides a body edit on a human-gated brief | row 1 (C6) |
| A wrong `commit_id` makes the wrong commit the source | row 1 and row 4 (body head must equal `commit_id`) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 6-7 need a human-run canary;
     until then they are could-not-check, never pass. -->

## Review
Gate: human (from frontmatter: irreversible is yes). The human gate is MANDATORY. The reviewer
answers BOTH, in the verdict:
1. What is the single control standing between the fault and the damage, and is that acceptable?
   (The SPOF line names the carry predicate; confirm layers 1-3 are present and independent.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Row 4
   places a carry verdict the verb never produced; row 6 proves the server dismisses a carry on a
   later push.)

Reviewer records verdict + date in the stream README table.
