- Skip desk communication polling when the project's recorded state explicitly keeps the lane
  pre-cutover. Desk skills continue their work-queue sweeps using the existing pre-cutover
  hand-off path; enabled-lane failures still stop the pass. Unknown or conflicting comms state
  now stops only comms (hand-offs go through the tracker, reported once) and never halts the
  sweep; the guardrail names where the record is read and that an absent `comms:` key reads as disabled.
