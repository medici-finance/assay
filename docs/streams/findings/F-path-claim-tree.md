---
id: F-path-claim-tree
date: "2026-09-02"
title: "A reviewer reported four present files as missing because it checked them in its own checkout, not the PR's repository"
affects: ["build-less-brittle/06"]
class: one-off
resolved: true
---

**What was found.** A dispatched reviewer checked whether the files a pull request relied on
existed, and ran that check in the dispatching desk's own checkout — a different repository, on
a different branch, at a different commit from the pull request. It reported four workflow files
as missing. All four were present in the pull request's repository. Three of the four went out in
a posted review, and each cost the author a round trip. Each was wrong in the one way a finding
must never be wrong: it asserted an absence it had never looked for in the place the absence
would have to be.

**Fix.** The review kit's path-claim clause ("Resolve every path claim in the PR's OWN
repository, at the PR's head", in `tools/desk/cmd/deskdispatch/references/review-prompt.md`)
requires every path claim to be resolved in the pull request's repository at its head, to name
the tree it was resolved against, and to be reported as could-not-check when it cannot be.

**Why it is recorded here.** The kit carried this narrative inline until the design-fit review
stage (`build-less-brittle/06`) tightened the kit to stay inside its line budget. The rule stays in
the kit; the incident that motivated it lives here, and the kit points to the findings register
in one line (it names no path, because the kit is generic and its public-tree guard refuses
internal document paths). It is resolved: that clause is the fix.
