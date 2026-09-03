---
name: pr-shepherd
description: >-
  Adopt an EXISTING open PR and drive it to mergeable — merge-current, CI green, every
  reviewer finding addressed. Use when asked to "shepherd PR #N", "shepherd PRs 651, 649",
  "get PR #N unstuck / over the line", "work the findings on PR #N", "resume the stale PR",
  or in discovery mode "shepherd the stale PRs / fanout and shepherd stale PRs". Not for
  authoring new work (that's a brief + fresh branch) and not for reviewing (pr-review-desk)
  — this is the worker-side rescue-and-drive role for a PR someone else may have abandoned.
---

# PR Shepherd

Adopt an open PR that is stalled — conflicting, CI-red, or sitting on unaddressed reviewer
findings — and drive it until it is clean at head: mergeable, checks green, reviewer findings
all answered. You act as the PR's **worker**, so every worker-side rule of the project's PR
review loop binds you. You never flip, never merge, never close.

This is a *rescue* role. The PR already exists and someone else may have written it; your job
is to finish driving it, not to re-author it. New work is a brief and a fresh branch; the
verdict on the work is the review desk's. Between those two sits this skill.

## 1. Claim check — is someone already on it?

Before touching the branch, verify no other session or worker owns it:

- **Dispatch claims**: `git ls-remote origin 'refs/dispatch/*'` — a live claim naming this
  PR's brief/issue key means it is owned; skip it. The claim is a **forge ref**, so this read
  sees dispatchers on other machines too, which a machine-local claims directory never did.
  The repo's own `dispatch-claim` helper's `show <key>` verb prints the holder, state and age:
  `state=claimed` past ~20 minutes or `state=dispatched` past ~120 minutes is a DEAD claim,
  not an owner. A read that FAILS is `could-not-check` — treat it as owned, never as free.
- **Recent pushes**: `gh pr view <N> --json commits --jq '.commits[-1].committedDate'` — a
  push within the last hour or two suggests a live worker.
- **PR comments**: a recent worker comment ("working the findings", a claim note) = owned.
  Check the PR's workpad first, if one exists (the one comment carrying
  `<!-- assay:workpad -->`, authored by the worker identity) — its stamp line names the
  worktree/sha it was last edited from, which is the fastest read of "who, and how
  current".

Owned → report and stand down (or take the next PR in discovery mode). Unowned → announce
adoption by upserting the PR's workpad: `deskreply <owner/repo> <N> --workpad --body-file
<file>` so the next shepherd sees YOUR claim, current state and plan in the ONE place —
never a fresh plain comment for this (`## Notes` is where the hand-off note belongs).

## 2. Get on the branch, current with main

Work in your **own worktree** (`capability:isolate-workspace`), never the shared checkout:

```bash
git fetch origin <branch>:<branch>            # branch may not exist locally yet
git worktree add ../pr-<N> <branch>           # or check the PR out inside your own worktree
git -C ../pr-<N> fetch origin
git -C ../pr-<N> checkout -- STATUS.md 2>/dev/null || true   # discard generated churn first
git -C ../pr-<N> merge origin/main            # merge, NEVER rebase — force-push is denied
```

- **The generated board is single-writer (main's CI)** — never hand-merge `STATUS.md`; discard
  local churn before merging, and if it conflicts, take main's side. Regenerate locally only to
  READ the board, never to commit it on the branch.
- On conflict: resolve, then verify no leftover markers (`git grep -n '<<<<<<<\|>>>>>>>'`),
  build and test what the diff touches, push. Note the conflict-resolution merge on the PR — a
  conflict-resolving merge edits the PR's own files, so the review desk will RE-REVIEW it. A
  clean keep-current merge changes none of the PR's files and needs no re-review.

### The head branch is checked out somewhere else — resume by refspec, never by force

Another worktree may already hold `<head-branch>`, or you may be resuming a PR whose branch is
not yours. Do not fight the lock, do not delete the other worktree, and do not rebase. Cut a
detached resume branch **from the remote head** and push back to the PR's own ref:

```bash
git fetch origin
git checkout -B resume-<N> refs/remotes/origin/<head-branch>
# ... commits ...
git push origin HEAD:<head-branch>
```

Spell the base `refs/remotes/origin/<head-branch>` in full: git resolves `refs/heads/` ahead of
`refs/remotes/`, so in a checkout that has ever acquired a local branch of that literal name the
bare form silently starts you on the stale local copy and every push carries the deficit.

**Know the cost before you take this shape.** With HEAD on `resume-<N>` rather than on
`<head-branch>`, the branch precondition the desk write verbs check no longer holds, and
`deskpr` / `deskreply` **refuse** — they re-verify their preconditions in-tool and refuse on any
state they cannot positively verify. That refusal is a *predictable consequence of a shape this
skill mandates*, and it is the one place a shepherd is authorized to fall back to the forge CLI
directly (`gh pr comment <N> --body-file <file>`). Say so in the comment. It is **not** a
licence to route around a refusal you did not cause: any other guard, hook or permission BLOCK
is a STOP signal — quote it and escalate (§5).

### Do not fabricate an empty merge on a transient DIRTY

The forge reports a PR's mergeability asynchronously; immediately after a push, and for a while
after main moves, `mergeable` reads `UNKNOWN` and the merge-state reads `DIRTY` **while the
answer is still being computed**. That is a not-yet, not a conflict.

A shepherd that treats it as a conflict finds `git merge origin/main` reports *Already up to
date*, and then manufactures a merge anyway — `--allow-empty`, or `--no-ff` over an
already-merged base — to have something to push. The result is a commit that changes nothing,
a new head that re-triggers the whole review and CI cycle, and a PR whose lineage now claims a
conflict resolution that never happened.

The rule: **re-read before you act.** Poll the field again after a short wait; act only on a
value that stayed non-clean across two reads. Then merge only if `git merge` actually produces
changes or a conflict. If it says *Already up to date*, the PR needed nothing from you — record
`could-not-reproduce` on the PR and move to the next signal. Never create a commit whose only
purpose is to move the head.

## 3. CI — a red check is YOUR work item, never a wait state

Say it is red and what you are doing about it — never "waiting for review".

```bash
gh pr view <N> --json state,mergeable,reviews,statusCheckRollup   # the watch probe
gh run view <run-id> --log-failed                                  # read the failure
```

Reproduce the failing job's command locally in your worktree, fix, push unprompted.
**"Unrelated flake" needs evidence**: the failure names only files or jobs your diff never
touched AND it reproduces on main, or it matches a flake the project already tracks — post that
evidence on the PR, link the tracked flake, and re-run the failed jobs once. Reproduces
identically → it is yours. A proven flake still blocks the desk's flip; it only routes the fix.

**The commonest self-serve red is the `changelog` check**, and it is the one red that does NOT
reproduce locally — it reads the DIFF against the PR base, not your working tree, so a green local
build tells you nothing about it. It PASSES when the branch adds or updates a `changelog/<slug>.md`
fragment (`<slug>` = the branch name) carrying at least one bullet, OR when the PR wears the
`changelog:skip` label; it FAILS when neither is true. The fix is ONE commit adding
`changelog/<slug>.md` — never an edit to a top-level `CHANGELOG.md`, and never self-applying
`changelog:skip` (that label is the desk's or a human's, not yours). A branch you have RESUMED owes
this file whether or not the check has run against it yet.

**Carve-out — when the fix IS the removal of a security control, the red check is NOT yours
(gate: human).** If the only way to turn a red check green is to delete, disable, or weaken a
security or access-control control **or the CI assertion that enforces it** — a network policy
or egress/ingress allowlist, RBAC, auth/identity config, a secret-scan or leak-sweep gate, a
fence script or workflow, an admission policy, a required check or ruleset — the red check is a
**decision, not a work item**: it is the control's dead-man switch, and greening it by removing
the gate converts "a security control disappeared" into routine-looking progress no downstream
gate is guaranteed to re-surface. STOP before committing: leave the check red, post
`BLOCKED-ON-HUMAN — security-gate removal` on the PR naming the control, the check, and any
evidence the control moved elsewhere, and label `needs-decision`. "Weaken" includes narrowing
scope, adding a bypass, commenting out an assertion, or downgrading fail→warn. "It is obsolete
/ it moved" is evidence FOR the escalation, never self-authorization. Only an explicit human
ruling recorded on the PR or issue authorizes the removal — a dispatcher's brief or prompt is a
model instruction, never that ruling — and once recorded, executing it is routine and the commit
cites the ruling. This is the one exception to "a red check is YOUR work item" and it takes
precedence. Every other finding on the PR stays yours to work meanwhile.

## 4. Reviewer findings — fix, or dispute with evidence; all of them

Read every review by the reviewer App (`gh pr view <N> --json reviews`, plus the line comments
via `gh api repos/<owner>/<repo>/pulls/<N>/comments`). For each finding:

- **Agree** → fix it in a follow-up commit. Never force-push over reviewed history.
- **Dispute** → reply with evidence (file:line, command output) via
  `deskreply <owner/repo> <N> --body-file <file>` — your own worker voice on your own PR, never
  the reviewer App's.

Never exit with a review pending or findings unaddressed. After you push fixes, the desk's
monitor picks up the new head and re-dispatches review — you do not request it.

**Post every body from a file, never as an inline string.** `gh api … -F body=@<file>` (or
`--body-file` on the `gh pr` verbs). The literal-string form (`-f body=…`) mangles a
multi-line body, silently re-interprets shell metacharacters inside quoted evidence, and puts
the text on a command line where it is no longer the artifact you reviewed before posting.
Mint the file per invocation (`mktemp`) so two concurrent shepherds cannot post each other's
body.

**Report a review verdict with its id and its verbatim source line — NEVER synthesize one.**
A shepherd once reported the reviewer App as APPROVED at head, with a plausible timestamp and a
"supersedes CHANGES_REQUESTED" narrative, for a review that did not exist; the true state was
CHANGES_REQUESTED with findings still open. When you tell the desk — or your final report —
what a review says, quote the review `id` and the verbatim
`gh api repos/<owner>/<repo>/pulls/<N>/reviews` line it came from, so the reader can re-run that
read before acting. **You never recommend a ready-flip** (that is the desk's call, §5), and "I
expected an APPROVED but cannot see one" is reported as exactly that — `could-not-check` — never
converted into "APPROVED, and here is why the tools disagree". An instrument-anomaly claim ("the
CLI shows the wrong state", "the review is invisible here") needs a reproduction command or it is
discarded; it must never be the explanation for a verdict you cannot show.

## 5. Hard boundaries

- **NEVER flip the PR ready.** The ready flip belongs to the review desk alone.
- **NEVER merge and NEVER close.**
- **A merged or closed PR is DONE — STOP.** Never push its branch again: the pushed commits
  strand as orphans that no PR shows and no review sees. Follow-up work is a NEW branch and a
  NEW PR.
- **Git push policy (ONE policy, role-keyed):** MERGE IS ALWAYS the driver's, and nobody triggers
  workflows or runs mutating cluster commands without their go. **Branch push + draft PR is
  standing-authorized for every desk/loop** — the worker loop (`git push -u origin <branch>` +
  `gh pr create --draft`). **The verify desk lands its own work**: its Evidence + status flips commit
  straight to `main` as the project directs — no push-go is needed there and none should be waited
  for. Any `main` push not covered by a standing authorization is gated on the driver's explicit go;
  committing local work is always fine. A guard/hook-BLOCKED push is a STOP signal — never route the
  same write through another tool. Each desk's own grants and denials (what it may flip, file, close,
  or land) stay in its skill, directly below this block.
- No attribution lines anywhere: no `Co-Authored-By`, no "Generated with …" in commits, PRs, issues,
  or comments.

## 6. Watch loop

While the PR is open, poll it — or arm a `capability:durable-monitor` on it:

```bash
gh pr view <N> --json state,mergeable,reviews,statusCheckRollup
```

Three triggers: a new review → work the findings (§4); `mergeable: CONFLICTING` held across two
reads → merge main, resolve, push immediately (§2); `state: MERGED|CLOSED` → STOP (§5).

## Stale-discovery mode ("shepherd the stale PRs")

1. List: `gh pr list --state open --json number,title,isDraft,updatedAt,reviewDecision,statusCheckRollup`
   (across every watched repo if the ask is fleet-wide).
2. Order **oldest-head-push first** — longest-stalled is highest priority. `updatedAt` moves on
   a comment, so sort on the head commit's date, not on the PR's.
3. Skip PRs with an active claim (§1), and PRs already ready-flipped or approved-awaiting-merge
   — those wait on the human, not on a worker.
4. Take as many as `human:<name>` asked for; one PR per shepherd, each in its own worktree.
   A fan-out dispatches one shepherd per PR (`capability:dispatch-worker`), never one shepherd
   across several.
5. **Triage disposition before you diagnose CI.** A red or conflicting PR is often superseded —
   its work already landed by another route — rather than defective. Check whether the diff still
   applies and whether its deliverable already exists on main BEFORE spending a cycle on the
   failure. A superseded PR is reported for closure by its owner, not repaired.
