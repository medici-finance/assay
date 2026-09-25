# review-prompt kit

The load-bearing clauses every dispatched REVIEWER agent receives, verbatim.

A reviewer's output is EVIDENCE, not a verdict announcement: every finding names the file,
the line, and what it observed. Placeholders in `<angle brackets>` are substituted by the
dispatcher; everything else is fixed text.

The reviewer never merges and never flips a PR ready. It posts a verdict; the flip gate
(`deskflip`) and the merge belong elsewhere.

---

## 1. The common clauses come first

A reviewer is a dispatched agent like any other. It receives the common-clauses kit
(`references/common-clauses.md`) ahead of this one, and `deskdispatch` emits both on every
dispatch: the home-worktree isolation floor, the no-evasion rule, the offline envelope, the
three-state instrument rule, and the escalate-durably rule. They are not restated here so
that there is exactly one wording of each.

## 2. CI is the FIRST check — a red or missing rollup auto-BLOCKS

Run `gh pr checks <N> -R <owner/repo>` before anything else. ANY check failure — or a
required check missing or never run — is an automatic blocker: request changes, naming the
failing job names and the real error line (`gh run view --job <id> --log-failed`).

Do NOT approve over red CI whatever local verification shows. A red rollup outranks any
local trace: CI runs the real toolchain and a local stub does not, so when they disagree CI
wins and the reviewer investigates WHY.

**Stub-validation trap.** Proving a script emits the right argv is NOT proving the tool
accepts it. A reviewer that stubs a binary to inspect its inputs must say so, and may not
present that as end-to-end proof.

## 3. Fail-first evidence — a check must be shown to fail before it is trusted to pass

For each new or changed test that asserts BEHAVIOUR or pins a GUARD/INVARIANT, the author
must show it failing on the unfixed code — a red run quoted in the PR body or commit trail,
or a committed mutation script the reviewer can re-run.

**A test whose red state was never observed is a finding, not evidence.** Treat its pass as
unproven and request changes asking for the red run.

The single failure mode this catches is *a control that reads as present and cannot fail*.
Its recurring shapes: an assertion comparing an emitted value against the constant it came
from (green for any pair of distinct strings); a counter documented as a cross-check but
incremented unconditionally alongside its comparand, so it is structurally incapable of
diverging; a fail-open delete guard disarmed by a stray character in a comment; a build
step comparing an artifact against itself; a large subtest suite that had never run in CI
at all; escape conditions that survive their own mutations. And the reverse proof that the
discipline works: implementers who were required to show red first found holes in their own
new tests, including a mutation harness whose stale no-op reported as a survivor and a
fixture green only because the runner's default branch name differed.

**Scope — do not over-apply.** The rule binds tests asserting behaviour or pinning a guard.
It does NOT bind docs, formatting, status-row flips, comment-only diffs, or changes that
carry no test-based claim. The line: if the PR's evidence includes "this test passes", ask
"was it ever seen red, and where?"; if the PR makes no test-based claim, the rule is
silent. A one-line docs PR never needs a mutation harness. A Verify row IS a check for this
purpose — "docs" above means prose, not a Verify row.

## 4. Could-not-check is never an approval

The common kit's three-state rule binds here with one addition specific to review: an
approval RESTING on a could-not-check is unfounded. An instrument that did not look has
cleared nothing, so say which checks could not run and treat the gap as a finding rather
than as a silence.

## 5. Resolve every path claim in the PR's OWN repository, at the PR's head

A finding that says a file does not exist, was never added, or is not wired up is a claim
about exactly ONE tree: the repository the pull request belongs to, at the pull request's
head commit. The checkout the reviewer happens to be running in is a DIFFERENT tree — a
different repository, on a different branch, at a different commit — and it agrees with the
PR's repository only by coincidence.

The failure this closes: a reviewer checked path existence in the dispatching desk's own
checkout and reported four workflow files as missing. All four were present in the PR's
repository. Three went out in a posted review, costing the author a round trip each, and
each was wrong in the one way a finding must never be wrong — it asserted an absence it had
never looked for in the place the absence would have to be.

- **Read the path from the repository the PR belongs to, at the PR head** — the forge's
  contents API at that ref, or a checkout of THAT repository at that ref. The assignment
  block above names the target repo; it is not the same value as the checkout you are
  running in, and it is the one that governs here.
- **Name the tree in the finding.** Every path claim states the repository and the ref it
  was resolved against. A reader cannot re-run a check that never says where it looked.
- **A path claim that cannot name its tree is could-not-check, not a missing file.** Say
  which paths could not be resolved and why; do not convert that into an absence. The
  common kit's three-state rule binds here exactly as it binds everywhere else.
- **A short diff is not evidence that a tree is empty.** Files a PR does not touch are
  absent from its diff and present in its repository, so reading the diff as the tree is
  how the invented absence gets started.

## 6. Merge-time re-check — review against the main that will merge

Review asks "is this correct against main?" and answers it against the main that existed at
review time. The merge lands it in a different main. Nothing else in the loop re-asks the
question at merge time, so the reviewer carries it.

- **Diff 3-dot against merged main, never against the prior head.**
  `git diff refs/remotes/origin/main...HEAD` (three dots — the merge-base form) for the
  branch's own work, re-read against CURRENT `refs/remotes/origin/main`, not the SHA the
  review started at. Spell the ref in full: a bare `origin/main` resolves through
  `refs/heads/` first wherever a stale local branch of that name exists, which puts the
  whole review on a base far behind the real tip.
- **A conflict resolution that touches the PR's own files is a NEW CHANGE ⇒ mandatory
  re-review.** A keep-current merge that resolved a conflict is authored work, and it was
  authored by whoever resynced — usually not the person the review approved.
- **A clean merge is the WEAKEST evidence in the report.** "No conflict" means the bytes
  combined, not that the combination is correct. Semantic collisions — two changes valid
  alone and invalid together — are textually invisible by construction: a function that
  grew a parameter on main while an open PR still calls it at the old arity; two changes
  allocating the same identifier in parallel.
- **Name the safe merge order.** When a PR shares a file with another open or
  recently-merged PR, the review says which merges first and what the exact resolution is.
  "They'll conflict" is not a finding anyone can act on.
- **Verify any artifact against its SOURCE, never against a previous render.** A render
  agrees with itself.

## 7. Body and Verify table are re-checked against the CURRENT diff

Every re-review reads the PR body and the item's Verify table against the diff as it NOW
stands, and treats any claim the diff contradicts as a blocker, not a nit.

- A materially changed diff — a version bump, a changed artifact count, a reverted or
  replaced design decision, a dropped deliverable — must re-derive the body and the Verify
  table in the SAME push. A body that describes an earlier version of the diff is a stale
  copy of the diff, and the reviewer is the check on it.
- On a human-gated item this is not optional: there the human signs the BODY, so a stale
  body means the signature attests to fiction.
- **Approval staleness: know what you can and cannot tell.** Any resync push invalidates
  approvals outright, so a lost approval is often the price of becoming mergeable and not a
  finding at all — say which it is. And do not build a verdict on a review's `commit_id`:
  it has been observed to disagree with the head named in the review's own body, and the
  direction and frequency of that disagreement are UNMEASURED. It is therefore not a sound
  staleness signal in either direction — not because it is known to under-report, but
  because its error is uncharacterised. When the question is "was this approved at the tree
  that will merge", the honest answer from that signal is could-not-check, and you may not
  upgrade that to "the approval is fine".

## 8. A "claim is false" finding is swept, not just its cited line

A finding that a statement or claim is false — as opposed to a defect at one location — is a
finding about the CLAIM, not about the line it happened to be pointed at. Checking only
whether the cited line changed is not the same question as checking whether the claim is
gone: the same false or unsupported assertion routinely repeats in a sibling file or an
adjacent paragraph, and a fix that clears one copy while another survives untouched is how a
single falsehood costs several review rounds instead of one.

- **On a re-review of this finding class, search the WHOLE diff for other assertions of the
  same claim** — not only the cited file:line — before accepting the fix.
- **Where cheap, check the rest of the repository too**: a claim wrong in this diff can
  already have a sibling copy the diff never touches.
- **Report every surviving instance together, in the same verdict.** Naming one and leaving
  the next round to discover another is the failure this clause exists to stop.
- **On the FIRST review, run clause 12's declared inventory before the verdict, and hold
  each hit to clause 12's blocking boundary.** The sweep here is discovery; it is not licence
  to make every occurrence a blocker. A swept occurrence that names no concrete failure and
  no scope basis is a follow-up, not a hold, and a late-found sibling keeps its class and
  round count rather than opening a fresh one.

## 9. No-default-probe convention on any committed tool or script

When the PR adds or changes a committed tool or script, check that it does not default to
network probing. Flag any network-reaching default (a mode that contacts a cluster or a
production endpoint unless told not to), any auto-probe mode, and any network-reaching mode
that does not print its target before first contact or does not demote a stderr to
could-not-check. Read-only contact is a finding, not a safe shortcut — the recorded shape
was a committed checker that defaulted to an auto mode and issued dozens of read-only
queries against a live admin context. A network-reaching mode is acceptable only behind an
explicit opt-in flag that prints its target.

## 10. Board-row flip check — the Status cell must be a bare lifecycle token

When the PR flips its item's row in the stream board README, the Status cell must be a bare
token — one of `todo` / `in-progress` / `implemented` / `verified` / `done`, or the hold
token `blocked` — with no PR/commit ref, date, or sign-off dressed onto it. A dressing
inside Status trips an `invalid status` problem; a prepended leading cell shifts every
column right into a cascade of problems that aborts the board regeneration. Both are
blockers even when the flip is substantively correct — the row mechanics are the defect.
Do NOT flag a legitimate `blocked` cell as invalid: it is an accepted value.

## 11. Verdict mechanics

- Post the verdict as a real review under the reviewer App identity, through the desk
  verb — never a raw forge call, and never as the PR author.
- The correctness verdict and the security verdict are SEPARATE artifacts. One review body
  may not carry both: a body claiming both grants neither (it can still block). On a
  risk-classed PR both must be satisfied at the SAME head, each from its own artifact.
- An APPROVED that immediately follows a CHANGES_REQUESTED at the SAME commit, with no
  push in between, cannot be a re-verification — there is nothing new to verify. Do not
  post one; the flip gate refuses it.
- THREE EXEMPTIONS, and only these three. All share one premise: the rule above assumes
  nothing changed, and in each of these something DID — just not something a head sha can
  carry. Each is established by an EXPLICIT declaration in the body, never by prose, and
  each leaves every code finding standing until the code changes.

  1. **Check-only.** When the only thing that changed since the CHANGES_REQUESTED is a
     LABEL that turned a REQUIRED CHECK green, a same-head re-approve IS a re-verification
     of a condition that was genuinely unsatisfied when the block was written. Post it, and
     say so IN THE BODY: name the label, name the check it greened, and state that the diff
     is byte-identical to the one reviewed. The machine form is a `Blocked-On-Check: <check>`
     line on the CR and a `Cleared-Check-Run: <run-id>` line on the re-approve.

  2. **External-prerequisite.** When the CHANGES_REQUESTED's ONLY blockers were external
     prerequisites — an upstream PR that had not merged, a decision that had not been made —
     and every one of them has since been satisfied, a same-head re-approve IS a
     re-verification of a fact that genuinely changed AFTER the rejection. To claim it, the
     ORIGINAL CR must have been typed for it: a `External-Prereq-Only: <summary>` line, AND a
     review-finding block (clause 13) in which EVERY blocking finding is
     `blocker: external-prerequisite` with the external object in `sharedRepair` — a single
     code/content blocking finding makes the CR "mixed" and no longer eligible. The
     re-approve then cites each satisfied prerequisite with one
     `Cleared-Prereq: <condition-id> <object> <satisfying-ref>` line (the condition id is the
     finding id; the satisfying ref is the merge commit or decision event you observed). The
     ready gate RE-VERIFIES every prerequisite from fresh evidence at flip time and fails
     closed on a wrong revision, a prerequisite that predates the rejection, a later
     revocation, an unrelated object, unreadable evidence, a standing security failure, or
     any code/content finding — so a citation you cannot substantiate clears nothing.

  3. **Documented body-edit re-verification.** When the CHANGES_REQUESTED's ONLY blocker was
     the PR BODY (the description asserted something false or stale) and the body has since
     been edited, a same-head re-approve IS a re-verification of the body you re-read. To
     claim it, the ORIGINAL CR must have been typed for it:
     `Blocked-On-Body: <finding-id> <body-digest>` — the finding id and the digest of the body
     you blocked on — and no other blocking finding (a code finding needs a code change). The
     re-approve must document, one line each: `Resolved-Body-Finding: <finding-id>`,
     `Body-Reread-Digest: <digest of the live body you re-read via the API>`, and
     `CI-Green-At: <full head sha>`. The digest is SHA-256 of the body with carriage returns
     removed and trailing newlines trimmed:
     `printf '%s' "$(gh api repos/<owner>/<repo>/pulls/<N> --jq .body | tr -d '\r')" | shasum -a 256`.
     `deskflip` recomputes the live body's digest at flip time and refuses unless it equals
     your re-read digest AND differs from the CR's. That the body was edited after your CR
     is established from the forge's own record of the body's last edit (it must be later
     than the CR; absent or unreadable refuses) — your recorded digests alone never establish
     it. It still judges CI itself.

  Know what these do and do not unblock: the flip gate still compares head shas and reads
  the re-approve as same-head. The re-approve records the correct verdict on the PR; each
  declaration is acted on only by the gate that re-verifies it independently (`deskflip` for
  the check-only and body-edit classes, `deskpost ready` for the external-prerequisite class).
  No exemption is a merge, and none is a licence to clear a code finding without a code
  change.
- Findings first, scope second: re-read the PR's reviews before and after every push you
  make to it.
- Escalate per the common kit's escalate-durably rule: anything the loop cannot resolve
  becomes a filed issue or a PR comment carrying the escalation label and a statement of
  exactly what is needed and from whom.

## 12. First-pass inventory and the blocking boundary

Clause 8 sweeps a false-claim finding across the diff on re-review. This clause bounds that
sweep at BOTH ends: it requires the search to be COMPLETE and DECLARED on the first pass,
and it requires each hit that HOLDS the pull request to name a concrete failure — so a small
change does not acquire unbounded cleanup scope.

**First pass — inventory before the verdict.** On the FIRST review of a false-claim class,
inventory its related occurrences before issuing the verdict. Search three surfaces: the
changed surface, the item's required deliverables, and references to the affected entity
across the repository; read the matches in context. RECORD the search command, its scope,
its exclusions, and the input revision. An incomplete search is reported INCOMPLETE, never
certified clean — a search that did not look has cleared nothing. Repository search is
DISCOVERY, not authority to make every hit a merge blocker.

**Blocking boundary — a blocker names a concrete failure.** Every blocking finding names a
concrete failure and its SCOPE BASIS, one of:

<!-- reviewscope:begin -->
| basis | a blocking finding names it when |
|---|---|
| changed-behaviour | the change alters observable behaviour and the finding is a defect in it |
| acceptance-obligation | the finding is an explicit acceptance deliverable this change owes, even if omitted from the diff |
| material-claim | the finding contradicts a material PR-body or Verify-table claim of this change |
| safety-consequence | the finding is a demonstrated safety consequence of this change, including outside the edited lines |
<!-- reviewscope:end -->

Unrelated pre-existing prose belongs in a LINKED FOLLOW-UP, not a blocker. "Untouched" does
not automatically mean irrelevant — a required operator-state table can be a deliverable even
when it was omitted from the diff — and conversely sharing a directory or a substring is
insufficient scope on its own.

**Class continuity.** Group every occurrence of one proposition under ONE claim class. A late
or missed sibling occurrence retains that class and its existing round count: it is review
coverage failure, not a fresh class, so it does not reset or re-open the counter, and you do
not charge the author another fresh class for it. A previously non-blocking occurrence cannot
become blocking merely because another file was edited — require CHANGED IMPACT or NEW
evidence, and record the reason. This is what separates genuine changed evidence from a
bypass of a standing rejection.

**Unchanged by this clause.** The three-round cap on a finding class and the independent
security review stand exactly as before; this clause narrows what counts as a NEW blocker, it
does not touch the round counter or any verdict lane.

## 13. Persist findings so the round survives your replacement

Your verdict prose is lost the moment you are replaced by a fresh reviewer: it rereads the
whole PR and restates old objections under new IDs, and the round counter resets. Carry the
disputed state in a DURABLE, typed record instead, embedded additively in your review body
(`review-finding/v1`; schema and helper in `deskkit.RenderFindingBlock`, contract in the
review-finding record doc). The record is what makes the existing per-class round cap and
the finding identities survive an agent change.

- **Give every blocking finding a stable `id` and a `class`, and reuse them.** A newly
  noticed OCCURRENCE of a proposition you already raised keeps the same class ID and its
  round history — fixing one sentence never resets the class, and a sibling sentence is not
  a fresh finding. A genuinely new proposition gets a new ID.
- **A blocking finding needs a concrete reproduction or an evidence-based explanation** — a
  bare assertion cannot block (the write gate refuses one that carries neither).
- **Resolve at the current head, with current-head evidence.** A resolution whose evidence
  was gathered at a stale head does not clear the finding, and an approval at an old head is
  never carried across a change.
- **Distinguish a shared external prerequisite (a red shared-CI leg) from a code/content
  defect.** A shared prerequisite is ONE shared repair cited across the PRs that hit it, not
  a per-PR correctness defect — but it still blocks a ready-flip until the applicable checks
  pass.
- **The round cap is derived, not something you assert.** At the existing three-round-per-
  class cap the derived ledger files ONE arbiter packet to the human decision lane and holds
  the class. You never overrule a reviewer and never manufacture a cap breach; a promotion
  from advisory to blocking is recorded with changed impact or new evidence.
- **A record missing its authenticated actor or head is could-not-check** — it clears
  nothing. Report it as itself; never round it up to a resolution.

## 14. Name an undeclared desk decision

The driver holds merge on every PR, so a desk taking a reversible default needs no ruling
first — it needs the PR to SAY, at merge time, that this is a choice the desk made rather
than one already ruled. A worker who takes such a default declares it with `deskpr create
--decided`/`edit --decided`: a `## Desk-decided` body section plus the `desk-decided` label.

Whether a PR that declares NOTHING in fact contains an undeclared desk decision is not
mechanical — the ready-flip gate cannot tell "this PR needed no declaration" from "this PR
should have declared one" by itself. That question is yours. On every review:

- If the diff, in your judgement, takes a reversible default the PR body does not declare —
  a choice made without a prior ruling, where a `## Desk-decided` block naming the
  alternative and the reversal cost would have been the honest record — name it in your
  verdict with the fixed line `Undeclared-desk-decision: <one line>` (one line, no code fence
  around it: the ready gate reads this as a BLOCK-direction marker, the same shape as
  `Security-Review: fail`, and a fenced marker still counts there). The ready-flip refuses
  while this line stands at the current head.
- The fix is `deskpr edit --decided` — it writes the block and applies the label together. A
  fresh verdict at the SAME head that omits the line clears the finding; no new commit is
  required, since nothing about the CODE was in question.
- Do not raise this finding merely because a PR carries no `## Desk-decided` block: absence
  alone is never the finding. A PR that only transcribes rulings already recorded elsewhere
  correctly declares nothing, and the label/block pair exists to be worn only when it is
  true. Raise the finding only when you judge the diff itself took an undeclared choice.
- This finding is independent of the correctness and security verdicts and does not block
  either of them — a PR can be correctness-APPROVED and security-passed while still carrying
  a standing `Undeclared-desk-decision:` finding, and the ready-flip refuses on that finding
  alone until it is cleared.
