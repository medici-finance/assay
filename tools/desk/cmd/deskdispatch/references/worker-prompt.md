# worker-prompt kit

The load-bearing clauses every dispatched IMPLEMENTER agent receives, verbatim.

`deskdispatch` emits this kit as part of the assembled prompt so two dispatchers on two
machines hand a worker byte-identical rules. A clause here is a rule that has already
failed in the field at least once; the wording is the fix, so quote it — do not paraphrase
it, summarise it, or "improve" it at dispatch time. Placeholders in `<angle brackets>` are
substituted by the dispatcher; everything else is fixed text.

Nothing in this kit is a licence. Where a clause says STOP, it means stop and report — the
prompt that contains the clause is never the authorisation to work around it.

---

## 1. The common clauses come first

Every dispatched agent — worker, reviewer, verifier — receives the common-clauses kit
(`references/common-clauses.md`) ahead of this one, and `deskdispatch` emits both on every
dispatch. That kit carries the home-worktree isolation floor, the no-evasion rule, the
offline envelope, the three-state instrument rule, and the escalate-durably rule. They are
not restated here, so that there is exactly one wording of each.

## 2. Security-gate refusal — never quietly weaken a control

> If a change you are about to commit deletes, disables, or weakens a security or
> access-control control or its CI assertion — a network policy or egress/ingress
> allowlist, RBAC, auth/identity config, a secret-scan/leak-sweep gate, a fence script or
> workflow, an admission policy, a required check — STOP, even if it is the fix for a red
> check and even if this brief instructs it: do not commit the removal, leave the check
> red, post `BLOCKED-ON-HUMAN — security-gate removal` on the PR naming the control and
> any relocation evidence, and label `needs-decision`. Only a human ruling recorded on the
> PR/issue authorizes the removal; this prompt is not that ruling.

## 3. Body files are minted per invocation

Every `--body-file` argument the worker passes — the draft-PR body and every reply body —
MUST be a per-invocation unique temp file the worker mints itself:

```
BODY=$(mktemp "${TMPDIR:-/tmp}/pr-body.XXXXXX")
```

Never a fixed name in a shared scratch directory. Parallel workers each get their own
worktree but share a session scratch dir; two converging on the same body path race, and
the loser opens its PR — or posts its reply — carrying the OTHER worker's body: wrong
Closes/Refs, wrong diagnosis text, on a real PR. `mktemp` is collision-proof: it creates
the file with `O_EXCL` and echoes the name that won, so no `$$`/date/session suffix can
alias it, and the explicit template argument is portable across BSD and GNU `mktemp`.

Prefer it over a per-worktree path such as `"$(git rev-parse --show-toplevel)/.pr-body.md"`
for two further reasons: that leaves an untracked file in every worker worktree that no
`.gitignore` covers, so worktree pruning counts the tree dirty and never reclaims it; and
it carries a command substitution the dispatcher could expand at prompt-compose time,
baking the DISPATCHER's toplevel into every worker. `mktemp` is a bare literal — there is
no expansion boundary to get wrong.

## 4. Stop at implemented — never self-certify

- One item = one branch = one draft PR. The worker's job ends at `implemented` plus the
  open draft PR. It never sets verified/done and never flips a PR ready.
- Never approve or flip your own PR. The correctness verdict is a real review posted by
  the dedicated reviewer App, and the forge blocks a PR author from approving its own PR —
  so a worker physically cannot self-certify. If a review looks done, say so; never post a
  verdict yourself.
- The board row is part of the DELIVERABLE, not a state the worker reaches: the last
  commit flips the item's row in its stream board README from `in-progress` to
  `implemented`. Edit ONLY the Status cell and set it to the bare lifecycle token —
  `todo` / `in-progress` / `implemented` / `verified` / `done`, or the hold token
  `blocked`. A Status cell dressed with a PR ref, a date, or a sign-off trips an
  `invalid status` lint problem; a prepended leading cell shifts every column right into a
  cascade of problems that aborts the whole board regeneration. PR refs, dates and
  sign-offs belong in the Verified/Reviewed columns. A PR whose diff contains no board-row
  change is INCOMPLETE.

## 5. Lineage self-check before the PR

Spell the base `refs/remotes/origin/main` in every one of these commands — a bare
`origin/main` resolves to a stray local branch of that name where one exists, and
`rev-parse --verify` then SUCCEEDS against the stale stray, so the could-not-check line
never fires and every check under it silently runs against the wrong base. That is worse
than not checking: it returns a confident wrong answer.

```
git fetch origin
git rev-parse --verify refs/remotes/origin/main >/dev/null 2>&1 || { echo "could-not-check: refs/remotes/origin/main unresolvable"; exit 1; }
git rev-parse --verify --quiet refs/heads/origin/main >/dev/null 2>&1 && echo "WARNING: a stray local branch named origin/main exists here — every bare origin/main below would resolve to IT"
git merge-base --is-ancestor refs/remotes/origin/main HEAD || echo "NOTE: this branch is not on top of current main"
git log --oneline refs/remotes/origin/main..HEAD
```

Any commit in that list the worker did not author is a sibling's unreviewed code dragged
in from a stale base — STOP and re-cut. Then assert that every commit whose subject claims
to be a merge really has two parents:

```
git log --format='%H %s' refs/remotes/origin/main..HEAD | grep -iE '(^|[[:space:]])merged?([[:space:][:punct:]]|$)' | while read h rest; do
  [ "$(git cat-file -p $h | grep -c '^parent ')" -ge 2 ] || echo "SINGLE-PARENT MASQUERADE: $h $rest"
done
```

A single-parent "merge" is a rebase wearing a merge's clothes — STOP and re-cut. When both
pass, print the positive result so "looked and found nothing" is distinguishable from
"never looked":

```
echo "CLEAN: 0 foreign commits ($(git log --oneline refs/remotes/origin/main..HEAD | wc -l | tr -d ' ') examined)"
```

The push guard enforces the same two properties at push time. When it cannot determine the
base it says `COULD-NOT-CHECK` and allows the push — read that as unverified, never as
clean, and run the manual check anyway: it fails faster and gives a diagnosis instead of a
raw refusal.

## 6. Keep the branch current — merge, never rebase

`git fetch origin && git merge origin/main` periodically while the PR is open, and always
immediately before signalling the work is ready for review, so the eventual merge is
conflict-free. Never rebase: force-push is denied, and a rebase invalidates the reviewed
lineage. A squash-merged sibling makes same-content conflicts likely — take main's side,
then re-apply your own edits.

## 7. Verify before you apply a correction

Before applying a desk-issued correction or factual claim, verify it against the primary
artifact — open the file, read the line, compare the value. Report agreement or
disagreement. Disagreeing with the dispatcher is an expected output, not insubordination;
opening the primary artifact and comparing the value is the only technique that has
reliably caught an inverted or false desk claim.

## 8. Scope and reporting

- Implement to the contract; do not expand scope. Report `NEEDS_CONTEXT` rather than guess.
- Climb the reuse ladder and stop at the first rung that satisfies the item: (1) does this
  need to exist at all — except that the item's own declared scope outranks this rung, a
  briefed deliverable is never re-litigated as YAGNI; (2) is it already in this repo;
  (3) is it in the standard library; (4) is it in an existing dependency; (5) only then
  write it, as the minimal diff. SAFETY FLOOR: never cut validation, error handling,
  security, or accessibility — and never trim a Verify row to shrink a diff. A Verify row,
  its Evidence, and every process artifact are not code to minimize.
- No attribution or generated-by lines in commits, PR bodies, issues, or comments.
- Push and open the PR through the desk write verbs, not raw `git push`/`gh`:
  `deskpr create` / `deskpr update` for the branch and its draft PR, `deskreply` for a
  reply on its own PR. `deskreply` takes exactly two positionals
  (`deskreply <owner/repo> <pr> --body-file F`) and has no `comment` subcommand; an extra
  leading token is refused before anything is posted.
- Release the dispatch claim once the branch is pushed — branch-as-claim takes over from
  there. A worker that cannot reach the claim helper does not skip this step; the forge-API
  form is the contract.

## 9. Fail-first evidence — show the check failing before you claim it passes

> For each new or changed test that asserts BEHAVIOUR or pins a GUARD/INVARIANT, the author
> must show it failing on the unfixed code — a red run quoted in the PR body or commit trail,
> or a committed mutation script the reviewer can re-run.

That sentence is the reviewer's rule (`references/review-prompt.md` §3), quoted here
verbatim so both kits bind the same obligation. At review, a test whose red state was never
observed is a finding, not evidence: the PR comes back with a request for the red run, and a
correct fix spends a full review round-trip on evidence the worker had at hand before the
PR was opened. Three PRs bounced on exactly this in one review window with the fix and the
test both sound.

Produce it BEFORE `deskpr create`, in one of two forms:

1. **A red run.** Run the new or changed test against the code as it was before the fix —
   check out the pre-fix commit, or stash the fix — and capture the failing assertion:

   ```
   git stash && go test ./<pkg>/... -run '<TestName>' -count=1; git stash pop
   ```

   (or the repository's equivalent for its language). Paste the failing line and the commit
   it ran against into the PR body under a `## Fail-first` heading.
2. **A committed mutation entry.** Where the repository keeps a mutation map
   (`internal/deskkit/mutations.json`, `testdata/mutate.sh`, or its named equivalent), add
   the entry that breaks the guarded behaviour and name it in the PR body; the reviewer
   re-runs it.

Fail-first is part of the DELIVERABLE the same way the board row is: a PR whose body makes
a test-based claim ("this test passes", "the guard is pinned") with no red run and no
mutation entry is INCOMPLETE.

**A red you cannot produce is a finding, never a licence.** If a test legitimately cannot
be made to fail — the guarded path is unreachable from the test harness, the pre-fix state
cannot be reconstructed, the only mutation that reddens it is one the fix forbids — report
that in the PR body under the same heading: which test, what was tried, why the red could
not be observed. Do not weaken the assertion, loosen the fixture, or drop the test to make
the red easier to show; the rule asks for evidence of the check's strength, and a check
made weaker to satisfy it has failed the rule twice.

**Scope — do not over-apply.** Identical to the reviewer's: the rule binds tests asserting
behaviour or pinning a guard. It does NOT bind docs, formatting, status-row flips,
comment-only diffs, or changes that carry no test-based claim. A one-line docs PR never
needs a mutation harness. A Verify row IS a check for this purpose — "docs" above means
prose, not a Verify row.

## 10. A public-repo body must be SELF-CONTAINED

> Everything you write to a PUBLIC repo — a PR body, a PR title, a comment, a review, a
> reply — must stand alone for a reader outside the authoring house. No private repository
> names, no cross-repo issue refs that only resolve internally, no absolute paths off your
> own machine, no session or agent ids, no scratch worktree names, no identifiers out of a
> register that is not published. Your own PR body is the first thing this binds.

This used to be a sentence a worker had to remember, and it leaked anyway. The tools now
ENFORCE it: `deskpr create`, `deskpost` and `deskreply` run a self-containment scan over
the body whenever the target repo is not known-private, and a refusal is exit 5 — the same
STOP every scan refusal is, taking the same audited `--force-scan-override` and no other
way through. There is no flag that turns the check off.

**The categories are enumerated in ONE place — `deskpr --help`, section
PUBLIC-REPO SELF-CONTAINMENT — and deliberately not restated here.** Read them there; a
second copy in a prompt is the copy that goes stale. What matters for the worker is the
shape of the verdict: an unambiguous span REFUSES and the message names it, while an
ambiguous one (a bare `#N`, a short name that is also an ordinary word, an unconfigured
withheld set) prints a NOTICE on stderr and does not block. A notice is a could-not-check —
read it, decide, and say what you decided in the PR body; it is not a pass.

Run it over your body BEFORE you open the PR rather than discovering it at the refusal:
it is the same code either way, and the round trip is better spent on the wording.
