### Changed

- Review scope is now bounded by a declared first pass and an impact-based blocking
  boundary. A reviewer inventories the related occurrences of a false-claim class on
  the first pass — recording the search, its scope, its exclusions and the input
  revision — and an incomplete search is reported incomplete, never certified clean.
  A blocking finding must name a concrete failure and its scope basis (changed
  behaviour, an explicit acceptance obligation, a material PR-body/Verify claim, or a
  demonstrated safety consequence of the change); unrelated pre-existing prose is
  routed to a follow-up instead of holding the PR. A missed sibling occurrence keeps
  its original claim class and round count, and a previously non-blocking occurrence
  cannot become blocking merely because another file was edited — a promotion requires
  changed impact or new evidence, explicitly recorded. The existing three-round cap
  and the independent security review are unchanged.
