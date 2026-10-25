---
id: DR-cell-stop-flags
date: "2026-10-25"
title: "Adopt cell-scoped stop flags STOP.cell.<cell>[.<loop>], enforced by the Guard and by the loop's own iteration check"
consequence: major
decided-by: "human:ian"
alternatives:
  - "Adopt the loop check only (option 2): desks honour the cell flag in their own iteration-boundary check, but the tool-level Guard is unchanged. Ruled out: a desk that skips its own check keeps writing, so the stop has one layer instead of two."
  - "Reject (option 3): drain and pause keep using the machine-wide STOP and STOP.<loop> flags. Ruled out: a house cell's config home links to the operator's, so a pause aimed at one cell also stops every other cell's matching desks."
accepted:
  - "Two new flags in the existing stop-flag directory: STOP.cell.<cell> stops every loop of that cell, STOP.cell.<cell>.<loop> stops one loop of it. Precedence DISABLED > STOP > STOP.cell.<cell> > STOP.<loop> = STOP.cell.<cell>.<loop>; any one present is a stop."
  - "Cell flags only ever add stops: they never clear or outrank DISABLED, STOP or STOP.<loop>."
  - "Cell membership comes only from the launcher-set process marker ASSAY_CELL; a session with no marker is unaffected, and a marker failing the cell-name rule builds no path."
  - "Two independent layers: deskkit.Guard() before every outward verb, and the iteration-boundary check of the three loops that have one (pr-review-desk, verify-desk, worker-desk). For intake-desk and the-desk the Guard is the only layer."
  - "deskboard tags a malformed or unknown-loop cell flag INERT, and a flag naming a cell unregistered under its cells root INERT with that root named and a may-still-be-live reason, never silently honoured or ignored."
  - "Same-OS-account threat model: any desk can write another cell's flag or drop its own marker; neither is an escalation over today's machine-wide STOP."
  - "Interim: the runner lifecycle and ownership contract replaces this mechanism and must keep its two guarantees (a pause aimed at one cell never stops another; a stop only ever adds)."
---

This record captures the design adopted for desk-containers/16
([brief](../desk-containers/brief-16-cell-scoped-stop-flags.md)), per the driver's ruling on
decision issue medici-finance/assay#4401, option 1 (adopt):

https://github.com/medici-finance/assay/issues/4401#issuecomment-6399335268

The ruling comment names the brief's board id, not this record's id, so this record carries
no `ruling:` key: the ruling-link lane requires the linked comment to name the record id.
The `decided-by` stamp is corroborated instead by the driver's approval of the pull request
that adds this record, which is the merge.

The record is written after the fact. The implementation it describes was delivered by
medici-finance/assay#2971 (merged 2026-10-16) before the decision issue was answered; the
ruling adopts that implementation as it stands. This record transcribes the brief's
`## Human decision` options and the adopted option's guarantees from the brief's `facts:` and
`gate-why:`; it makes no independent verification claim. Running the brief's Verify table on
merged main remains the verifier's obligation.

`consequence: major` because this is the stop-flag check every desk verb consults: a cell flag
that silently matches nothing reads as "stopped" while desks keep writing.

Not decided here: the runner lifecycle contract that replaces this interim mechanism, and the
drain and pause verbs built on it (desk-containers/18 and /20).
