---
id: REQ-coverage-boundary
date: "2026-09-04"
title: "A reader can tell which lifecycle phases the method covers and which they still have to bring"
impact: major
asked-by: "project driver, relaying a first-visit review by a reader with no prior knowledge of the project (2026-09-04)"
acceptance:
  - "A published page lists the lifecycle phases end to end and marks each one covered, partly covered, or not covered."
  - "Each not-covered phase says, in one line, what the adopter brings instead."
  - "No page describes a phase as covered while the project's own board says the work that would cover it is not done."
  - "The page states the date it was last reconciled against that board."
status: proposed
---

The method covers build, review, verification and release well, and does not cover
requirements, design, deployment or incident handling. The site does not say so, and its
framing invites a comparison against tools that claim the whole lifecycle. A reader who
discovers the gap themselves reads it as an overclaim; a reader who is told the boundary
reads the same fact as a scope decision.

The constraint behind the ask is that the boundary must be derived from the project's own
tracked state rather than written once and left. A coverage page that drifts from the board
is worse than none: it converts a stated boundary back into an overclaim, silently, on the
day some other work slips.

Not being asked for: closing the uncovered phases. Stating the boundary and closing it are
different pieces of work, and stating it is the one that can be done honestly today.

Status is `proposed`. This register records what was asked and, later, what was claimed to
satisfy it; it does not observe that the product meets the ask
(`../../../spec/registers-v1.md` §6.4).
