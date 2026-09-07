---
id: REQ-glossary-first
date: "2026-09-04"
title: "A newcomer meets no term on the front page that has not been defined by the time they meet it"
impact: minor
asked-by: "project driver, relaying a first-visit review by a reader with no prior knowledge of the project (2026-09-04)"
acceptance:
  - "Every project-specific term used on the entry page resolves to a one-line definition reachable in one click."
  - "A published reading order names, in sequence, the pages a first-time reader should take."
  - "A reader who follows that order encounters no term whose definition appears only later in the sequence."
status: proposed
---

The first-visit review reported jargon arriving before definitions: the entry page uses
project-specific vocabulary — the names of the desks, the register types, the gate
taxonomy — as though the reader already had them. The reader could still work the meaning
out by reading further, which is why this is `minor` and not `major`: the route to the
goal exists, and the cost is friction rather than a stop.

The constraint behind the ask is that a definition is not a substitute for the boundary
statement in `REQ-coverage-boundary`. Explaining the vocabulary of a phase does not make
the phase covered, and a glossary that reads as coverage would defeat both requirements at
once.

Not being asked for: a rewrite of the terminology itself. The terms are load-bearing and
consistent with the specifications; the gap is that they are introduced without
definitions, not that they are wrong.

Status is `proposed`. Whether this ask is taken on, and by what work, is not recorded here
yet — and a later `satisfied` status would be a claim, settled only by the cited work's own
evidence (`../../../spec/registers-v1.md` §6.4).
