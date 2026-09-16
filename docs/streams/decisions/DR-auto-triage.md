---
id: DR-auto-triage
date: "2026-09-16"
title: "Automate the RESPONSE to an all-stop CI gate — the automation may file issues, open draft PRs, and route/escalate autonomously, but never merges and never weakens a gate"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Scope the whole-tree gate down to the PR's own diff so ambient churn stops reddening every open PR — ruled out, and ruled out FIRST, because it treats the all-stop as the defect. The all-stop is the desired signal: a whole-tree gate reddening is how the system says something somewhere is wrong, and narrowing it to per-PR scope would silence exactly the class of ambient defect (#611, #1119, #612) this stream exists to surface and act on. The problem was never that the gate is too broad; it is that the RESPONSE to the gate is manual."
  - "Leave the response manual — keep hand-triaging each red gate as today — ruled out: it is the toil this stream removes. The public record (#611, #1119, #612, #880, #536, and the two same-day main-reds scoped alongside this doc) shows the identical read-name-file-fix-or-route pattern recurring, each instance costing a human or a desk a hand pass. A recurring, identically-shaped manual response is the definition of automatable toil."
  - "Automate the response AND let the automation merge its own mechanical fixes — ruled out: merge authority stays human. The automation opens DRAFT PRs only; a wrong mechanical fix that a human never sees before merge converts a triage bug into a shipped bug, and the human merge gate is the backstop that makes an autonomous mis-fire cheap (a closed draft) rather than expensive (a bad merge on main)."
  - "Alert only — file/notify on a red gate but never open a fixing PR — ruled out as the WHOLE answer, though retained as one LAYER: alerting without action still leaves the mechanical toil (someone must still write the gofmt fix), and the mechanical class is precisely the part where the fix is deterministic enough to draft automatically. Alerting-without-action survives only as the never-invisible watchdog (brief 04), the fail-safe for when no responder acted."
  - "Let the identifier attempt a fix for every red, including opaque ones like leak-sweep, by guessing the culprit from the diff — ruled out: leak-sweep withholds the matching token and file:line by design, and guessing from the diff has already re-planted the same withheld token while 'fixing' the artifact. An opaque red is never auto-fixed; it always routes to judgement, and the automation never guesses a withheld value."
accepted:
  - "The automation takes autonomous OUTBOUND actions — files an issue, opens a draft PR, posts a routing default, escalates — without a fresh human act per action. This is a real widening of standing authority: once armed, it responds to every future red gate on its own. It is bounded on four sides — draft PRs only (never merge, never ready-flip, never self-approve), the mechanical class only for auto-fix, judgement-and-opaque reds routed to a human default, and no gate ever narrowed, silenced, or edited to make a red go green."
  - "A classifier bug can open a wrong draft PR or file a spurious issue. The cost is bounded to a human closing an artifact, not a bad merge, because the human merge gate stands unchanged behind every mechanical fix. The classifier is deliberately asymmetric: genuine ambiguity classifies as judgement (ask a human), never mechanical (act) — a false 'mechanical' is the expensive error, a false 'judgement' merely asks."
  - "The identifier cannot see what CI withholds. leak-sweep's matching token and file:line are private by design, so a leak-sweep red is named but never mechanically fixed — it always routes to a human, and the automation never reconstructs a withheld token from the diff. This is a permanent limit of step 1, not a gap to close."
  - "A red gate that no responder acted on within a bounded window is escalated by the watchdog (brief 04) rather than left silent. Accepted cost: the watchdog itself files autonomously, and a watchdog false-positive is a spurious escalation — tuned by widening N before it is tightened, because a missed silent red (the failure mode it guards) is worse than an early ping."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
this record. Nothing in this stream's `gate: human` briefs may advance past `todo` to
`in-progress` until this record is approved (the design-approval gate,
`spec/lifecycle-v1.md` §4.4; the briefs cite it via their `design: DR-auto-triage` key).

## The decision, and the constraint behind it

The signal is kept; the response is automated. A whole-tree gate reddening on ambient churn
is a DESIRED all-stop — it is the system telling the operators that something is wrong
somewhere, and the value of an all-stop is precisely that it cannot be ignored. What is
being widened is not the gate (untouched) but the AUTOMATION'S AUTHORITY to respond to the
gate on its own: to name the culprit, file the bug, draft the mechanical fix, route the
judgement call, and escalate a silent red — the work a human or a desk does by hand today.

The constraint that makes this acceptable is that every autonomous action is **reversible
by a human at zero cost to `main`**: a draft PR is not a merge, a filed issue is not a
ruling, a routing default is a recommendation a human overrides, and no gate is ever
touched to make a red pass. The one control the whole design leans on is the human MERGE
gate, and it is unchanged — which is why arming autonomous *authoring* is a major, not a
critical, consequence.

## Defense in depth — the layers behind the single control

The single control that would otherwise stand alone is the classifier's mechanical/judgement
decision (brief 01). Behind it, three independent layers, each failing for a different reason
in a different component:

1. **Draft-only + human merge (at the forge).** Every mechanical fix lands as a DRAFT PR a
   human must review and merge. This layer holds even if the classifier is wholly wrong,
   because it trips on a different actor (the human reviewer) at a different time (before
   merge) than the classifier.
2. **Conservative classification (in the identifier).** Genuine ambiguity, and every opaque
   red (leak-sweep), routes to judgement, never to auto-fix. This layer holds when the
   auto-fixer would otherwise act, because it withholds the mechanical path on a different
   signal (classification confidence / opacity) than the merge gate.
3. **The never-invisible watchdog (out of band).** A red gate with no responder action in N
   minutes is escalated regardless of why the responders were silent — a crashed responder,
   an unclassifiable red, a bug in step 1. It trips on elapsed-time-with-no-action, a signal
   independent of both the classifier and the merge gate, in a separate component.

## What this record does NOT decide

It does not fix the trigger mechanism (workflow event vs poll), the binary-vs-workflow split,
the value of N for the watchdog, or which forge-write identity the automation posts under —
those are the briefs' and their reviewers' to settle. It does not authorize any merge,
ready-flip, or self-approval. It does not narrow, silence, or edit any gate. And it does not
claim the draft-only bound is a sandbox: a human acting outside the tools can still merge a
bad fix — the bound raises the cost of the mistake and gives it a review surface, it is not
an enforcement the platform provides.
