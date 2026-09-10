### Fixed
- **`statusgen`'s runner-attribution check no longer treats a `(non-implementer)`
  self-label as proof of independence.** `implementerAttributed` used to strip the
  literal `non-implementer` before its substring test, so an Evidence Runner cell
  reading `worker (non-implementer)` — or a Verified cell naming `non-implementer` —
  passed the independence check outright. A session could certify its own verification
  as independent just by writing the label. The predicate now matches the token
  `implementer` in any spelling (case-insensitive), including the self-labelled
  `(non-implementer)`; an honest independent verifier names the runner that ran the
  table (e.g. `opus-verifier`) — a token with no `implementer` substring — which still
  counts. The self-verification error message now states plainly that a self-labelled
  `(non-implementer)` is still a self-assertion.

### Added
- **A committer-identity cross-check on verified/done briefs.** The attribution check's
  author/verifier comparison reads free strings an agent writes at will; it now layers a
  second, independent signal from git — comparing the git identity of a brief's authoring
  commit against that of the commit that most recently touched it (the Evidence-adding /
  status-flip commit). Where a repository's brief history uses more than one identity, a
  brief whose authoring and Evidence commits sit under a single identity is surfaced (the
  Evidence was not independently committed); where the whole history is one identity, one
  aggregate notice says so rather than emitting a per-brief non-signal. The cross-check is
  a NOTICE, never a hard failure — it never over-rejects an honest verification whose
  distinct runners commit under a shared identity — and it degrades LOUDLY, never
  silently, when git cannot answer (an untracked brief under a `.git` repo). A `git
  archive` export with no `.git` is skipped silently, having nothing to check.
