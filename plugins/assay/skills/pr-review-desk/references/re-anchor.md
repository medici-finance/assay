# Re-anchoring a reviewed PR whose head has moved

A reviewed PR whose head has moved is this desk's most repetitive job and its most expensive
mistake: each instance gets re-derived from scratch, and three of those derivations are known to
cost a full cycle. This file is the state table for it — five states, each with the SIGNAL that
raised it, the PROBE that confirms it from a primary read, the ACT, and the STOP that makes it a
filed issue or a hand-off instead of a desk action. Read the row, run the probe, act; the probe is
never a remembered value and never the board's cached summary.

**Every verdict is posted at the FULL 40-character head SHA.** A shortened SHA is accepted by the
shell and silently fails to register the verdict: the flip gate then reads no verdict at head, and
the whole re-anchor is lost with nothing red to show for it. Take the head from a primary read at
post time; never paste one out of a transcript or a board row. Posting mechanics belong to the
review-posting verb (`deskpost`) and are specified in `verdict-format.md`; which verdicts count at
which head is ruled in `SKILL.md` § What stays ORDERED and § the security lane. This file re-homes
neither — it says which state you are in and what the next step is.

## State 1 — the head moved under a standing approval or a flip

| SIGNAL | PROBE | ACT | STOP |
|---|---|---|---|
| The current head is not the head the standing verdict names — a keep-current merge, a board resync, a re-push — and no finding was answered. | Three-dot diff reviewed-head→current-head restricted to the PR's own paths, with every ref spelled in full; then confirm the reviewed tip is an ANCESTOR of the current head. Own paths byte-identical plus ancestry intact = the standing review still describes this tree. | Re-post BOTH lanes' verdicts at the current full-length head — correctness and security, per the body schema each lane already has — then run `deskflip` and honour its refusal. A re-anchor re-posts the same verdict at the new head; it does not re-open the review. | Own paths differ, or the reviewed tip is NOT an ancestor (a force-push, a rebase, a conflict resolution): that is authored work — dispatch a real RE-REVIEW, never a re-anchor. Ancestry or diff unreadable is could-not-check, which routes to RE-REVIEW, never to a flip. |

## State 2 — a standing CHANGES_REQUESTED at the SAME head

| SIGNAL | PROBE | ACT | STOP |
|---|---|---|---|
| A CHANGES_REQUESTED verdict stands and the head has not moved since it was posted. | Re-read the standing review at head from the forge, not from the board, and classify the CAUSE: (a) the finding is demonstrably not about this tree, (b) the finding needs a code change, (c) the finding is disputed on a question the desk has no authority to settle. | (a) dismissal, with the reason recorded on the PR; (b) hand back to the worker — the answer is a real commit that ADVANCES the head, followed by a re-review at the new head; (c) relay to a human as a filed issue carrying the escalation label and what is needed from whom. The cause selects the act; nothing else does. | Never answer a standing CHANGES_REQUESTED by re-posting an approval at the same head: `SKILL.md` § What stays ORDERED rules that a same-head approval is not re-verification, and § the security lane records what a same-head cross-lane approval launders. A SUSPECT approval is answered by advancing the head with a real commit, never by re-posting at the same one. |

## State 3 — the board says CONFLICTING while a merge gate is still pending

| SIGNAL | PROBE | ACT | STOP |
|---|---|---|---|
| A board row reports CONFLICTING on a PR whose merge or CI gate has not settled. | Re-probe the flip verb's own view before routing anyone at it: `deskflip` re-reads every condition itself and names the one that failed. A mergeability state computed while a gate is pending is frequently racy and clears on its own between two reads. | Second read still CONFLICTING → hand the resync to the worker (merge main, resolve, push). The resolution touches the PR's own files, so it returns as a mandatory RE-REVIEW, not a re-anchor. Second read clear → no action; the first read was the race. | Do not dispatch a worker on one board read — that is the read that sent a worker at nothing. Do not resolve the conflict from the desk. Two reads that disagree with nothing to settle them is could-not-check: hold the PR, record it, do not flip. |

## State 4 — a red check at head

| SIGNAL | PROBE | ACT | STOP |
|---|---|---|---|
| A required check is red at the current head, on the board or in the rollup. | Open the failing RUN and read its log before treating it as a failure. A run CANCELLED by a later push in a burst, a toolchain-download or infrastructure flake, and a real assertion failure present IDENTICALLY at the check-name level — the rollup cannot tell them apart and neither can you without the log. | Real failure → the worker owns it; a red check is a work item, never a wait state. Cancelled by a push burst → re-read at the current head; the superseding run is the one that counts. Flake → re-run, and record that a re-run is what cleared it rather than letting it read as a first-pass green. | A run whose log cannot be read is could-not-check: no flip, no worker dispatch on a guess — file it. If greening the check would delete, disable or weaken a security control or its assertion, that is a human decision: file it with the escalation label and stop. |

## State 5 — a non-commit fix at an unchanged head

| SIGNAL | PROBE | ACT | STOP |
|---|---|---|---|
| The fix was a label change or a PR body/title edit, and the board row or the monitor still shows the pre-fix state. | Read the PR's current labels, body and the check at head DIRECTLY. The head SHA is unchanged, so `pr-monitor` never fires on it and no CI re-runs: nothing downstream of the monitor will ever show you this change. | Re-evaluate the gate the non-commit change was meant to satisfy, against that direct read at the unchanged head, then take the flip or the hand-back that read supports — and say the read was direct, so the record does not imply the monitor saw it. | Do not wait for the monitor to re-fire; it cannot, and the wait is indistinguishable from a queue that is simply empty. If the gate needs a signal only a fresh CI run can produce, this is not a non-commit fix: hand it back for a head-advancing commit. |
