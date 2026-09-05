---
id: REQ-evidence-visible
date: "2026-09-04"
title: "A stranger can verify a published claim without running anything or asking anyone"
impact: critical
asked-by: "project driver, relaying a first-visit review by a reader with no prior knowledge of the project (2026-09-04)"
acceptance:
  - "A reader with JavaScript disabled sees the current metric values on the metrics page, not an empty shell."
  - "At least one operated board is published at a stable URL, with its real dwell times, including the unflattering ones."
  - "Every operational number published anywhere on the site names the fleet it came from, in the same view as the number."
  - "A reader can reach, from the front page in one step, an artifact they can check themselves without credentials."
status: proposed
---

The project's central claim is that its evidence is checkable by someone who does not
trust the party making the claim. A first-visit review found that a reader has nothing to
check: the strongest proof on the site — a live, same-day metrics feed that refuses to
publish a productivity multiplier because no baseline exists — renders client-side only,
so no reader without JavaScript, no crawler and no assistant ever sees a number. The
refusal to publish uncheckable numbers is the differentiator; a differentiator nobody can
observe is indistinguishable from an ordinary marketing page.

The constraint behind the ask is the project's own honesty rule: no number without a
baseline, and a measurement that could not be computed is never rendered as a zero.
Meeting this requirement must not be done by publishing a number that clears the bar only
because it is unlabelled.

Not being asked for: an external customer reference. There are none yet, and inventing one
would spend the asset this requirement exists to protect. A single honest self-reference,
labelled as such, satisfies the fourth criterion.

Status is `proposed`, not `accepted`: this entry records the ask as received. Recording an
ask is not agreeing to it, and this register does not observe that any of it was met — see
`../../../spec/registers-v1.md` §6.4.
