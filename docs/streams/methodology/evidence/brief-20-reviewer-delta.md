# Brief 20 — reviewer-side delta (DEFERRED to methodology/22 or the human)

This is a **written delta**, not a live edit. The target is the **user-level** pr-review-desk skill
(`~/.claude/skills/pr-review-desk/SKILL.md`), which is out of this repo. Per the out-of-repo boundary
(CLAUDE.md, issue #221) and brief-20 Task 3, brief-20 does NOT touch `~/.claude/` — applying this
delta is methodology/22's move (single-home the operating rules into the repo) or the human's,
whichever lands first.

## The delta

Add to the pr-review-desk **dispatch template** — the list of questions every fix-PR reviewer prompt
must carry — two mandatory questions, so that class-sweep and real-path-test discipline is checked at
the reviewer half as well as the authoring half:

1. **Class-sweep:** "Did this fix sweep the pattern to every sibling site? Quote the literal grep/
   search that enumerates the defect class, list every matched site, and confirm each is routed
   (fixed-here / follow-up brief / out-of-scope-with-reason). An unlisted sibling is how #147 and
   #104 shipped a fix that left the class live."

2. **Real-path test:** "Does the new/changed test exercise the **real path**, not a mock that would
   pass even with the bug present? Name the path under test and why the mock (if any) does not
   short-circuit the very code the fix touches."

## Why the reviewer half is needed

The authoring rule (SKILL.md § Class-sweep rule + this stream's README convention) puts the burden on
the brief author. The reviewer questions are the backstop for the case the author missed the class
entirely — which is precisely the #147/#104 failure mode, where the fix brief itself was the thing
that under-swept. Presence of the grep in the PR is cheap for the reviewer to re-run.

## Apply-when

- methodology/22 lands the desk skills into the repo → fold these two questions into the in-repo copy
  of the pr-review-desk dispatch template in the same change, OR
- the human applies them directly to `~/.claude/skills/pr-review-desk/SKILL.md` before then.
