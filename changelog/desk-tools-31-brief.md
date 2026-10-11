- desk-tools/31 authored: the correctness review lane gains a coverage contract — a named
  bug-class register (14 classes) every class of which the verdict body must account for as
  a finding, a checked-clean naming what was examined, an n/a with a reason, or a
  could-not-check. Silence stops passing as review, and examined-and-sound never hides
  inside n/a.
