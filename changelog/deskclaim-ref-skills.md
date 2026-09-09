### Changed
- The desk skill bodies now name **`deskclaim-ref`** as the default dispatch-claim tool —
  installed with desk-tools, verbs `acquire` / `progress` / `release` / `steal` / `show` /
  `list`, deskkit exit codes 0/5/6 — with "a repo may ship its own `tools/dispatch-claim.sh`,
  which `deskdispatch` prefers when the resolved root carries it" as the documented override.
  `worker-desk`, its `dispatch-runbook` reference and `pr-shepherd` previously described only
  the consumer script, which a green-field or native-Windows adopter never has.
- Those bodies also now state that a claim read must run BOTH listings: the claim tool acquires
  and lists in `refs/dispatch/*`, while `git ls-remote origin 'refs/heads/dispatch/*'` lists the
  branch refs the Go claim readers use — a known, unresolved divergence a single read can miss.
