# Example: a task-board filter

This is an illustrative authoring example, not a recorded product run. Adapt the
workflow to the system being demonstrated; review, merge and verification are
appropriate here because the scenario demonstrates a software-delivery process.

Audience: a technical lead who wants a small feature delivered with evidence.
Promise: show how one request becomes a filter whose behavior the audience can check.
Starting state: a board mixes open and completed tasks.
Finished result: Open hides completed tasks; All restores them without editing tasks.

## Main path

1. **Problem.** Show the mixed task board. Evidence for a recorded version: a
   screenshot or application-state capture of the starting build.
2. **Request.** Ask for an Open filter. Retain the same work-item identity throughout.
3. **Acceptance.** State the visible behavior: Open hides completed tasks; All brings
   them back; task contents remain unchanged.
4. **Implementation.** Show the initial filter and the changed product, with the
   implementation artifact linked in the evidence ledger.
5. **Review.** Reproduce a concrete defect: returning to All still hides completed
   tasks. This is an invented finding in this example. A recorded version must use
   a finding actually observed in its run, or keep this scene labelled illustrative.
6. **Repair.** Show the corrected transition and its targeted regression output.
   A model's claim that tests passed is not the test output.
7. **Approval.** Show who approves the change and who merges it, according to the
   demonstrated system's actual authority. Keep these acts distinct.
8. **Verification.** Exercise the delivered build against the original acceptance
   example. Include build identity and the verifier's observation.
9. **Payoff.** Show the actual board switching Open → All → Open. Link the evidence
   and offer one relevant next action.

Model-selection explanations or recovery episodes can become optional chapters
when they answer a specific audience question. They need not extend the main path.

## Scene and evidence contract

For every scene, specify the audience question, benefit caption, state, focal
change, evidence mode and references. In a mixed story, labels change with the
scene. Captured test output does not turn an invented UI preview into a recording.

A recorded evidence entry should identify:

- the scene and local capture file;
- the demonstrated build or source revision and capture date;
- what was redacted or time-compressed; and
- the narrow claim the capture supports.

Use explicit relative paths inside the delivered bundle. Check each one resolves.
Treat local-only evidence separately from material cleared for publication. Exported
frames must retain evidence labels and cannot rely solely on a companion ledger.

## Rehearsal

Jump straight to Verification, then backwards to Acceptance. The displayed state
must be correct in either order. For a player, pause mid-playback, seek elsewhere,
and resume: old timers must not advance the newly selected scene. Check keyboard
navigation and reduced motion, and inspect a narrow viewport for unreadable text.
The stage stays a constant size across scenes — fixed aspect-ratio box,
fixed-height caption and evidence-label bands, controls at a constant position —
so seeking never moves the controls or reflows the page. Its backdrop contrasts
with the host page's background in the host's own design tokens, so the player
reads as an embedded presentation viewport while the surrounding page chrome is
unchanged.

When the demonstrated system numbers its own phases, rehearse the two numbering
systems side by side so no beat reads as its host phase; check that any `00` cover
beat frames the story before the promise; and for a comparative story, confirm that
switching path focus changes emphasis without changing the scene or the work-item
identity.

A storyboard-only delivery explains how each scene will be captured. A rendered
delivery also checks playback and reports any browser or capture limitations.
