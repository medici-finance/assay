---
id: DR-contrib-disclosure
date: "2026-09-12"
title: "The contributor-facing documents state the trust bar plainly, including that automation profiles unknown authors, and ask for verification claims rather than assertions"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Say nothing publicly and run the controls quietly — ruled out: a contributor whose pull request is quarantined, labelled and carrying a provenance card is experiencing the controls whether or not they are documented, and the undocumented version reads as arbitrary hostility. Stating the bar is what makes it a policy rather than a mood."
  - "State only the rules that constrain the contributor (issue-first, concurrency, approval) and omit the profiling — ruled out: a publicly posted card describing an account's behaviour is the most surprising thing this stream does, and the page that omits it is the page that makes it feel like surveillance. Disclosure costs nothing that the card does not already disclose by existing."
  - "Publish the tier names and each identity's tier — ruled out: naming who holds which tier is the public reputation list the ledger deliberately avoids. The MODEL is published; the ROWS are not."
  - "Add a long template with many required fields — ruled out: a template heavy enough to deter a drive-by submission deters a careful first-time contributor identically, and the observed pattern would fill it in as fluently as it filled in the bodies. Two short, specific prompts that are cheap to answer honestly and awkward to answer falsely beat a long form."
accepted:
  - "Publishing the bar tells someone submitting in bulk exactly which signals are measured, and some of those signals are trivially adjustable once known (pacing a fork, varying body wording). This is accepted: the signals are weak by design and never conclude anything on their own, so the value lost to disclosure is small, and a control a contributor cannot see is a control they cannot comply with."
  - "The pull-request template asks the author to state, per claim, how they verified it, and to disclose whether the change was produced with automated assistance. Both are unverifiable by any tool. Their value is that an honest answer is cheap and a false one is a specific, checkable falsehood a reviewer can point at — which is what the review lane then does."
  - "Stating the bar honestly means stating the parts that are advisory as advisory, and not implying enforcement the repository does not perform."
  - "The wording is judged, not linted. Presence checks prove the sections exist; whether they read as welcoming is the review gate's call."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
the brief's decision issue.

The decision is how much of the trust machinery to publish, and what to ask contributors for
in return. The published guidance today is accurate about issue-first, concurrency, maintainer
merge and automation-ignores-strangers. It is silent on everything this stream adds.

The governing asymmetry: the *model* — tiers exist, this is what moves an identity between
them, this is what each unlocks, these are the signals an unknown author's submission is
measured on — costs almost nothing to publish and buys a contributor the ability to understand
and comply with their own treatment. The *rows* — who holds which tier — cost a person's
standing to publish and buy nobody anything. Publish the first, never the second.

The asking half follows from the incident in the scoping document: the one provably false
thing in the observed pattern was a body claim. A template that asks "which claims are you
making, and how did you check each one?" does not stop anyone from writing a false answer. It
does convert an atmosphere of confident assertion into a set of specific statements that a
reviewer can check one at a time, which is the whole mechanism of the review lane this stream
adds.

**What this record does not decide.** It does not flip any advisory guideline to enforcing, it
does not set the tier-promotion criteria a contributor might read as a promise, and it does
not decide how the automated-assistance disclosure is used beyond the tiering rule recorded
under the tier model.
