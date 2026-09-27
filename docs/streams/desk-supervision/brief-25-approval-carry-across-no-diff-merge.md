---
brief: assay:assay:desk-supervision:25
title: Carry a correctness approval across a merge of main that leaves the PR's diff byte-identical
why: >-
  Every time a draft PR merges main to stay current, its head moves past its approval and it waits
  for a full model re-review, even when the change it asks to land is byte-for-byte what the reviewer
  already approved. On a busy queue that re-review is pure cost: the verify desk's Evidence PRs pay it
  after every sibling landing, and #1706 paid it on 2026-09-27. Carrying the correctness approval,
  when tools can prove the diff is identical and the only new commits are driver-free clean merges of
  main, removes that round. It does not re-check what main's own changes mean for the PR (CI at the
  new head is the only check on that, exactly as for any PR that merges without updating), and it
  never carries a security verdict.
wave: 1
depends: ["desk-supervision/24"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: >-
  This relaxes a review gate: a PR whose head moved can be flipped and merged on a correctness
  approval a model gave at an earlier head, with no model reading the new head. A wrong carry lets
  content no reviewer read reach a public default branch, where disclosure cannot be recalled and
  force-push is denied (irreversible). The human confirms that the carry is acceptable at all, which
  definition of "identical" to use, and that the safeguards and the one residual single-layer path
  named below are acceptable.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
id: 573b35fd-05f0-430a-a520-16a6fdab348e
authored: 2026-09-27 by worker-desk authoring session (#882, the approval carry option)
exec-tier: strong
exec-tier-why: >-
  (a) the identity predicate and its disqualifiers are a design, not a lookup; (b) the verdict is
  written by one tool, re-derived by two flip verbs and bounded by the forge, and all must agree;
  (c) review-gate plumbing where a subtle carry bug survives a happy-path test.
domain: complicated
sources:
  - "medici-finance/assay#882 — the re-review churn after each keep-current merge of an Evidence PR"
  - "option name: this brief carries 'approval carry' (carry an approval across a no-diff merge of main); its siblings are 'per-file outcomes' (desk-supervision/24) and 'batched landing' (desk-supervision/26) — three alternatives for the conflict class #882 tracks"
  - "docs/streams/desk-supervision/review-finding-v1.md:85 — the rule this brief amends"
  - "tools/desk/cmd/deskflip/flip.go — checkReviewerApproved (line 771) and reduceSecurityVerdict (line 1412)"
  - "tools/desk/cmd/deskpost/ready.go — the second ready-flip verb, gate (b)"
  - "tools/desk/cmd/deskboard/board.go:1233 — the MERGE-CURR classification"
  - ".github/workflows/evidence-automerge.yml — enables auto-merge on a ready, approved verifier-App PR"
  - "plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md — approval staleness and the unreliability of a review's commit_id"
  - "live reads 2026-09-27: the default branch's rules, and the timeline, reviews and commits of #1706 (facts)"
  - "freshness-checked 2026-09-27 @ b227b4076 (origin/main): no carry exists; both flip verbs require the verdict at the current head"
consumers:
  # Authoring PR: routed to this brief (rule 6); each flips to fixed-here in the implementation
  # commit that edits the path.
  - "tools/desk/internal/deskkit/approvalcarry.go (planned; both carry predicates): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation adds it)"
  - "tools/desk/cmd/deskpost (the carry verb, and `deskpost ready` gate (b) re-deriving a carry): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/cmd/deskflip (accepts a carry verdict only after its own re-derivation): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/cmd/deskboard (MERGE-CURR becomes CARRY on the same predicate; a carry verdict on a non-draft PR is an anomaly row): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "docs/streams/desk-supervision/review-finding-v1.md (line 85): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation amends it)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md and references/merge-time-recheck.md: follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits them)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the Evidence-PR state table of the PR lane): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/README.md (deskpost, deskflip, deskboard sections): follow-up desk-supervision/25 (this brief; flips to fixed-here when the implementation edits it)"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (it enables auto-merge only on a ready PR, at or after the flip. The carry verb refuses a PR that is ready or has an auto-merge request (C8), so a carry verdict reaches this lane only after a flip verb has re-derived it; a later merge push is dismissed by the forge and needs a model review. Left unchanged; Verify row 12 proves the refusal)"
  - "the default branch's rulesets: out-of-scope (unchanged by design; layer 3 depends on them staying as they are, and Verify row 8 reads them back)"
---

# Brief 25 — Carry a correctness approval across a no-diff merge of main

## Context

files:
- **add** `tools/desk/internal/deskkit/approvalcarry.go` (planned) + `approvalcarry_test.go` (planned)
  — two pure predicates, `CarryByPatch` and `CarryByBlobs`, and the verdict body grammar.
- **edit** `tools/desk/cmd/deskpost/` — a `carry` verb + tests, and gate (b) of `ready.go`.
- **edit** `tools/desk/cmd/deskflip/flip.go` + tests — `checkReviewerApproved`.
- **edit** `tools/desk/cmd/deskboard/board.go` + tests — the MERGE-CURR branch of `classify`, and the
  carry-on-ready anomaly.
- **edit** `docs/streams/desk-supervision/review-finding-v1.md` (line 85).
- **edit** `plugins/assay/skills/pr-review-desk/SKILL.md`,
  `plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md`,
  `plugins/assay/skills/verify-desk/SKILL.md`, `tools/desk/README.md`.
- **add** `changelog/<branch>.md` — the fragment this repo enforces.

single-point-of-failure: the carry predicate is the ONE control between "the head moved" and "the old correctness approval counts". Layers, counted per path to a merge. Flip via `deskflip` or via `deskpost ready` (the only way a draft PR becomes mergeable): (1) the carry verb derives the predicate from local git objects and refuses a PR that is ready or has an auto-merge request (C8); (2) both flip verbs re-derive the predicate from forge data by a different method, never trusting the verdict, so a carry posted by any path, including one that skipped layer 1, cannot flip a PR whose content changed; (3) the forge dismisses a correctness APPROVED on any later push, so a carry never outlives its head; (4) the review board lists any carry verdict standing at the head of a non-draft PR as an anomaly for a human. Four layers. A PR that is already ready, or has auto-merge enabled: the carry verb refuses it (layer 1), and the forge dismissed its previous approval on the merge push (layer 3), so the merge waits for a new approval. A carry verdict placed on such a PR by a path that skipped the verb would meet no flip gate; that path has layer 3 for any later push and layer 4 after the fact, and nothing before the merge except the reviewer identity's own key custody. That residual path is stated in the Human decision. The security lane is never carried, so layer 3's limit to APPROVED reviews (a security pass is a COMMENT review the forge never dismisses) does not apply.

facts (2026-09-27 @ b227b4076; re-establish from the named files and commands at pickup):
- **The rule being amended.** `review-finding-v1.md:85`: "A finding keeps its id and evidence when
  the head advances; … An approval recorded at an old head is **not** carried across the change."
- **Two flip verbs.** `deskflip`'s `checkReviewerApproved` (flip.go:771) accepts only the reviewer
  App's decisive correctness verdict AT the current head; an older head is STALE, naming both
  commits. `deskpost ready` gate (b) (ready.go) performs the same ready mutation and accepts "the
  reviewer App's latest verdict is APPROVED, submitted at the CURRENT head". Both must re-derive a
  carry, or one of them is the ungated path.
- **Security verdicts.** `reduceSecurityVerdict` (flip.go:1412): a `Security-Review: fail` STANDS
  across any head move; a `pass` grants only at the current head. A security pass is posted as a
  COMMENT-event review (`TestSecurityReviewPassPostsACommentEventReview`,
  `tools/desk/cmd/deskpost/securitylane_test.go`), which the forge's dismiss-on-push never dismisses.
  So this brief carries the correctness lane only; a risk-classed PR still needs a fresh security
  verdict at every new head.
- **deskboard disagrees with deskflip today.** `classify` (board.go:1233) marks a PR MERGE-CURR, "no
  re-review", when the PR's own files are unchanged since the reviewed sha; deskflip then refuses the
  same PR as STALE.
- **The server dismisses on push.** Live read of the default branch's rules (2026-09-27): the
  PR-review ruleset carries `dismiss_stale_reviews_on_push: true`,
  `require_last_push_approval: true`, one required approval. #1706: APPROVED at 135f3a344; the
  verifier pushed a merge of main, 86f8b67ee (2026-09-27 16:02:58Z); the timeline records
  `review_dismissed` by the pushing identity at 16:04:03Z; a fresh APPROVED at 86f8b67ee followed
  at 16:13:03Z. So a carried approval must be a NEW review at the new head.
- **The auto-merge lane.** `evidence-automerge.yml` enables auto-merge on a verifier-App PR that is
  ready (not a draft) and approved; the request survives later pushes. So a carry on a ready PR
  would be the approval that request waits for, with no flip gate in between — which is why C8
  refuses it.
- **Under the shared log no Evidence PR can carry.** Measured on #1706: the default-context patch-id
  of the 3-dot diff differs between 135f3a344 and 86f8b67ee (the log's context lines moved), and the
  `-U0` one is equal. That `-U0` equality does NOT make it carryable under either option: the merge
  resolved the log with the `merge=union` driver, a driver-free recompute of two end-of-file appends
  conflicts (exit 1; reproduced for all 19 CONFLICTING Evidence heads on 2026-09-27), so C4 refuses
  it, and `CarryByBlobs` (ii)/(iii) refuse it because the log's blob changed on both sides. **The
  carry reaches Evidence PRs only after desk-supervision/24 replaces the appended log with one new
  file per outcome** (no context from main, no driver needed); hence the dependency. Ordinary PRs'
  keep-current merges qualify on their own merits either way.
- **A review's `commit_id` is not a trusted head.** `merge-time-recheck.md`: it has been observed to
  disagree with the head named in the review's own body, and the error is uncharacterised. C5 fails
  closed on any disagreement.
- **`git patch-id --verbatim`** hashes the diff without stripping whitespace (plain `--stable`
  ignores whitespace); line numbers are ignored.
- **Forge list limits.** The compare API returns at most 300 files and a PR's file list at most
  3000 (GitHub REST documentation; re-check at pickup). A truncated list is could-not-check.

**"Identical" — the predicate.** As written it is the strict-identity option of the Human decision
below; the changed-lines-only option changes only C3's context. Inputs: the approved head `A` (the
source verdict's head), the current head `H`, the fetched tip of the base branch `M`. A carry holds
only if ALL of these hold; any unreadable or truncated input is could-not-check, which is no carry:

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
  parents, and that recompute is clean (exit 0). A conflict resolution, a merge driver's resolution,
  or any content the merge adds of its own disqualifies. The recompute runs only with
  `$GIT_DIR/info/attributes` empty or absent, checked first (non-empty is could-not-check), so no
  local attributes file can make it agree with a driver's resolution.
- **C5 — the source is a real, current verdict.** The source is the reviewer identity's decisive
  correctness APPROVED; it is an original review, not itself a carry (a carry always names the
  original `A`); its body names a head, and that head equals its `commit_id` equals `A` (a body that
  names no head is no-carry). Nothing after it blocks: no CHANGES_REQUESTED after it from the
  reviewer identity or from any trusted reviewer or maintainer, including one later dismissed, and
  no open review-finding record on the PR.
- **C6 — the body and title did not change.** The forge's record of the PR's last body edit and last
  title edit are both earlier than the source verdict; an absent or unreadable record is
  could-not-check, as the existing body-edit exemption treats it. On a `gate: human` brief the
  human signs the body, so an edit needs a real re-review.
- **C7 — security is never carried.** A standing `Security-Review: fail` blocks a carry; a security
  pass is never carried and never counts at a new head without a fresh security review there.
- **C8 — only a draft with no auto-merge request.** The PR is a draft and has no auto-merge request
  at the moment the carry is posted, so a flip verb, which re-derives it, must run before the PR can
  merge.

## Human decision
<!-- decision-trigger: creation — the options are enumerable now; filed as the brief lands. -->
When a draft pull request merges the latest default branch into itself to stay current, its newest
commit changes, and today every approval it had stops counting. A model reviewer must read it again
before it can be marked ready, even when what the pull request would change is exactly what was
already approved. The proposal: when tools can prove that the only new commits are clean merges of
the default branch that needed no merge driver, and that the pull request's change is byte-for-byte
what was approved, the review desk posts a mechanical correctness approval at the new commit that
names the approved commit, and no model re-reads it.

Safeguards: the approval is posted only while the pull request is still a draft with no automatic
merge requested; both commands that mark a pull request ready re-check the proof themselves, by a
different method, before accepting it; the hosting platform discards the approval the moment another
commit is pushed; the review board flags any such approval found on a pull request that is no longer
a draft. A security review is never carried: a pull request that needs one gets a fresh one at the
new commit. Only an approval placed by some route other than the posting command, on a pull request
already marked ready, would meet none of the ready checks; that route is guarded only by who holds
the reviewer identity's key, and it is flagged afterwards.

What the carry cannot see: a change the default branch made to some other file that alters what the
pull request means (a function it calls, a test helper, the CI configuration). The only check on that
is the continuous-integration run at the new commit, exactly as for a pull request that merges into
a newer default branch without updating first.

This needs a human because it relaxes a review gate: content can be marked ready and merged to a
public default branch on an approval a model gave to an earlier commit. The cost of a wrong carry is
content no reviewer read becoming public, which cannot be recalled.

What is being decided: whether to adopt the carry, and how strict "byte-identical" is.

Options:
1. **Adopt, strict identity.** The change must be identical including three lines of surrounding
   context. If the default branch changed anything within three lines of the pull request's own
   changes, there is no carry and a model re-reviews. Recommended: a nearby change is exactly where
   a semantic collision hides.
2. **Adopt, changed-lines-only identity.** Only the changed lines themselves must be identical. This
   additionally carries a keep-current merge where the default branch changed lines next to, but not
   inside, the pull request's own changes, provided the merge was clean without any merge driver. It
   misses a nearby change that alters what the pull request's lines mean.
3. **Do not adopt.** Every commit change needs a model re-review, as today.

Under either adopt option, no merge that needed a merge driver ever carries. That includes every
pull request that appends to the one shared, appended log file that verify outcomes use today, so
the carry is built only after that log is split into one file per record, which is planned
separately.

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
   - `CarryByPatch(facts)`: C1-C8 over local git facts (ancestry, commit parents, the two patch-ids
     and name-status lists, each merge's tree and its driver-free recompute) plus the forge facts C5,
     C6 and C8 need, gathered by a thin adapter that runs the git commands above in a checkout.
   - `CarryByBlobs(facts)`: the same verdict from forge data, without a checkout:
     - (i) the forge's compare of `A...H` reports `H` ahead of `A` and not behind;
     - (ii) the pull request's own file list, with each file's status and its tree entry (mode and
       blob id together), is equal at `A` and at `H`;
     - (iii) the compare between the merge base of `A` with `M` and the merge base of `H` with `M`
       touches none of those files;
     - (iv) every commit between `A` and `H` that is not on `M` has two parents, and its second
       parent is an ancestor of `M`;
     - (v) C5, C6 and C7 as above.

     A truncated compare or file list is could-not-check, never a shorter list that happens to
     match.

   Both return carry, no-carry (naming the first failed condition), or could-not-check. The two are
   deliberately not equivalent: under the strict option `CarryByPatch` carries a main edit to a PR
   file outside the 3-line context and `CarryByBlobs` refuses it (the blob changed); under the
   changed-lines-only option (C3 with `-U0`) `CarryByBlobs` is the STRICTER of the two for the same
   reason. Every asymmetry fails closed: the verb may post, the flip verbs refuse. Row 2 pins them.
2. **The carry verb.** `deskpost carry <owner/repo> <N> --from <A> --root <checkout>`, correctness
   lane only. It runs `CarryByPatch` at the PR's current head. On carry it posts, as the reviewer
   identity, a correctness APPROVED at `H` whose body carries `Approval-Carried-From: <A>`,
   `Carry-Patch-Id: <id>` and `Carry-Merges: <shas>`. On no-carry (including a PR that is ready or
   has an auto-merge request, C8) it exits 5 naming the condition; on could-not-check it exits 6.
   Nothing is posted in either case. There is no security-lane form.
3. **Both flip verbs.** In `deskflip`'s `checkReviewerApproved` and in `deskpost ready` gate (b), a
   verdict at `H` whose body carries `Approval-Carried-From:` counts only when `CarryByBlobs` from the
   cited `A` to `H` returns carry at flip time, re-read with the head-stable re-read `deskflip`
   already has (add the same re-read to `deskpost ready`). Otherwise the refusal names the carry and
   the failed condition. The security-lane reductions are not touched: a carry never satisfies them.
4. **The board.** `classify`'s MERGE-CURR branch uses `CarryByBlobs`. An eligible draft PR gets a new
   CARRY action ("post a correctness carry"; a risk-classed PR still needs its security review);
   anything else is RE-REVIEW. A carry verdict standing at the head of a non-draft PR, or of a PR
   with an auto-merge request, is a CARRY-ON-READY anomaly row that the review desk files for a human.
5. **Amend `review-finding-v1.md:85`** to read, in substance: "An approval recorded at an old head is
   **not** carried across the change, with one exception. When the PR is a draft, the only new
   commits are driver-free clean merges of the base branch, and the PR's diff against the base is
   byte-identical, a correctness carry verdict posted at the new head stands in for a re-review. The
   verdict names the original approval, and every flip verb re-derives it independently. A finding
   is never cleared by a carry, and a security verdict is never carried." Findings keep their ids
   and rounds as today.
6. **Skills.**
   - `plugins/assay/skills/pr-review-desk/SKILL.md`: the MERGE-CURR bullet becomes CARRY (run
     `deskpost carry` for the correctness lane, re-review the security lane if the PR is
     risk-classed, then flip). The Evidence-PR state table points at the carry for a merge that
     qualifies. CARRY-ON-READY is filed, never flipped over.
   - `plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md`: its approval-staleness
     paragraph gains the carry exception and its limits.
   - `plugins/assay/skills/verify-desk/SKILL.md`, the Evidence-PR state table of the PR lane: a
     merge-delta re-review may be answered by a carry when the merge qualifies.
   - Keep every skill body neutral: no values, no names.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCarryByPatch$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r1-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryByPatch' "${TMPDIR:-/tmp}/b25-r1-1.out"` | exit 0; output contains `--- PASS: TestCarryByPatch`. carry: a draft PR with a clean two-parent merge of main and an identical diff. no-carry, one fixture each, naming the condition: a non-merge commit after `A` (C2); a merge of a non-base branch (C2); an octopus merge (C2); `A` not an ancestor of `H` after a rewrite (C1); main changed a line inside a PR hunk's context (C3); a whitespace-only edit to a PR file (C3, which plain `--stable` would miss); a file-mode change (C3); a conflict resolution in a PR file (C4); a union-driver resolution of two appends (C4); a merge that adds a file neither parent has (C4); a non-empty `info/attributes` (C4, could-not-check); a CHANGES_REQUESTED after the source from the reviewer, from another trusted reviewer, and one later dismissed (C5); an open review-finding record (C5); a source whose body names no head, and one whose body head differs from its `commit_id` (C5); a source that is itself a carry (C5, resolved to the original); a body edit and a title edit after the source, and an unreadable edit record (C6); a standing security fail (C7); a ready PR and a draft with an auto-merge request (C8). A missing object is could-not-check | check:ci +mutation |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCarryByBlobs$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r2-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryByBlobs' "${TMPDIR:-/tmp}/b25-r2-1.out"` | exit 0; output contains `--- PASS: TestCarryByBlobs`. The row 1 scenarios expressed as forge data give the same verdicts, a mode-only change included (tree entries, not bare blob ids); a truncated compare (300 files) or file list is could-not-check; a merge whose second parent is not on `M` is no-carry. `TestCarryPredicatesAgree` (planned) runs both predicates over every shared scenario and fails on any disagreement EXCEPT the two named asymmetries, which it asserts explicitly: a main edit to a PR file outside the context window (patch carries, blobs refuse) and, under `-U0`, a main edit next to a PR hunk (patch carries, blobs refuse) | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskpost/ -run '^TestCarryVerb$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r3-1.out" 2>&1 && grep -F -e '--- PASS: TestCarryVerb' "${TMPDIR:-/tmp}/b25-r3-1.out"` | exit 0; output contains `--- PASS: TestCarryVerb`. Each disqualifier exits 5 and the stub forge records zero posts, a ready PR and an auto-merge-requested draft included; could-not-check exits 6 with zero posts; an eligible PR gets exactly one APPROVED at `H` carrying the three body lines; the verb has no security-lane form | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestFlipCarriedVerdict$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r4-1.out" 2>&1 && grep -F -e '--- PASS: TestFlipCarriedVerdict' "${TMPDIR:-/tmp}/b25-r4-1.out"` | exit 0; output contains `--- PASS: TestFlipCarriedVerdict`. LOWER LAYER WITH THE UPPER BYPASSED: a carry APPROVED at `H` placed directly in the fixture's reviews, never produced by the verb, citing an `A` whose file entries differ at `H` (a merge that edited a PR file), is refused naming the carry and the failed condition. A valid carry flips. A carry on a risk-classed PR with no fresh security pass at `H` is refused at the security condition. A carry citing a source whose body head differs from its `commit_id` is refused | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/deskpost/ -run '^TestReadyCarriedVerdict$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r5-1.out" 2>&1 && grep -F -e '--- PASS: TestReadyCarriedVerdict' "${TMPDIR:-/tmp}/b25-r5-1.out"` | exit 0; output contains `--- PASS: TestReadyCarriedVerdict`. The same LOWER-LAYER fixtures as row 4, run through `deskpost ready` gate (b): the verb-less carry over a content change is refused naming the carry; a valid carry flips; the head-stable re-read refuses when the head moves between read and mutation | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestClassifyCarry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r6-1.out" 2>&1 && grep -F -e '--- PASS: TestClassifyCarry' "${TMPDIR:-/tmp}/b25-r6-1.out"` | exit 0; output contains `--- PASS: TestClassifyCarry`. A keep-current merge whose PR files are unchanged but whose merge resolved a PR file with a driver is RE-REVIEW, not CARRY; an eligible draft is CARRY; unreadable inputs degrade to RE-REVIEW; a carry verdict at the head of a ready PR, and of a draft with an auto-merge request, is CARRY-ON-READY | check:ci +neighbour |
| 7 | Live, on a canary draft PR in this repository (number exported as `CANARY_PR`) after a carry verdict at `H`: push a further merge of main, then `gh api "repos/medici-finance/assay/issues/$CANARY_PR/timeline" --jq '.[] \| select(.event=="review_dismissed") \| .created_at' && deskflip "$CANARY_PR"` | the timeline shows the carry review dismissed after the push, and deskflip refuses until a new carry is posted: a carry never outlives its head. Use a merge whose diff is truly unchanged; if the forge moved the old review's `commit_id` to the new head instead, record that (C5 then fails closed) | gate:human +mutation |
| 8 | `gh api repos/medici-finance/assay/rules/branches/main --jq '[.[] \| select(.type=="pull_request") \| .parameters \| {dismiss_stale_reviews_on_push, require_last_push_approval}]'` | at least one entry reads `{"dismiss_stale_reviews_on_push":true,"require_last_push_approval":true}` — the server layer this design relies on is still in place | check +dereference |
| 9 | Live: a real draft PR approved at `A` merges main to an eligible `H`; the review desk runs `deskpost carry`; then `deskflip` on it; then read the PR's reviews | one carry review at `H` naming `A` and the patch-id, no model correctness review between `A` and the flip, and the flip succeeds | gate:human +flow |
| 10 | `grep -n 'is \*\*not\*\* carried across the change' docs/streams/desk-supervision/review-finding-v1.md && cd tools/desk && go test ./internal/deskkit/ -run '^TestDeriveLedgerCarry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b25-r10-1.out" 2>&1 && grep -F -e '--- PASS: TestDeriveLedgerCarry' "${TMPDIR:-/tmp}/b25-r10-1.out"` | output contains `--- PASS: TestDeriveLedgerCarry`; the amended sentence is present with its exception clause; the ledger test shows a carry verdict clears no finding and a finding opened after `A` blocks the carry | check:ci +dereference |
| 11 | `cd tools/skillslint && go run . --root ../..` | exit 0 | check:ci |
| 12 | Live: on a canary PR that is ready (or a draft with an auto-merge request), after a qualifying merge of main, run `deskpost carry` on it | exit 5 naming C8, and the PR's reviews show no new review: a carry never reaches the auto-merge lane without a flip verb | gate:human +mutation |
| 13 | `statusgen --consumers --root .` | exit 0 — every routing token above corroborated against the branch diff | check:ci +dereference |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| A merge carries an edit of its own (an "evil merge") and the approval carries over it | row 1 (C4), rows 4 and 5 (both flip verbs' entry check with the verb bypassed) |
| A whitespace or mode change rides a carry | row 1 (C3 with `--verbatim`), row 2 (tree entries) |
| History rewritten and an approval carried onto different code | row 1 (C1) |
| The verb's predicate has a bug and posts a bad carry | rows 4 and 5 (independent re-derivation at every flip verb), row 2 (agreement and named asymmetries) |
| One flip verb re-derives and the other does not | rows 4 and 5 |
| A carry reaches an auto-merge request, or a ready PR, with no flip gate | row 1 and row 3 (C8), row 12 (live), row 6 (CARRY-ON-READY) |
| A carried security pass is never dismissed | C7: no security carry exists (rows 1, 3, 4) |
| A carry outlives a later push | row 7 (server dismissal), rows 4 and 5 (head-stable re-read) |
| The forge ruleset this rests on is later relaxed by hand | row 8 |
| Board and flip verbs disagree again | row 6 (all use `CarryByBlobs`) |
| A carried approval hides a body or title edit on a human-gated brief | row 1 (C6) |
| A wrong `commit_id` makes the wrong commit the source | rows 1, 4 (body head must equal `commit_id`) |
| Main changes another file that alters the PR's meaning | no row. Review-only: stated in the Human decision; CI at the new head is the only check, as for any un-updated PR |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 7, 9 and 12 need a human-run
     canary; until then they are could-not-check, never pass. -->

## Review
Gate: human (from frontmatter: irreversible is yes). The human gate is MANDATORY. The reviewer
answers BOTH, in the verdict:
1. What is the single control standing between the fault and the damage, and is that acceptable?
   (The SPOF line names the carry predicate and counts the layers per path, including the one
   residual path; confirm each count.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Rows 4
   and 5 place a carry verdict the verb never produced in front of each flip verb; row 7 proves the
   server dismisses a carry on a later push; row 12 proves the verb keeps a carry off the auto-merge
   lane.)

Reviewer records verdict + date in the stream README table.
