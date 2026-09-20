### Added
- `deskevidence` gains `--dry-run`: it mints the verifier App token, resolves the forge,
  fetches the remote content, merges/scans it and runs the statusgen PROBLEM-diff guard —
  every gate that can refuse a landing still runs — then prints the commits-API landing plan
  (create/update, target path, branch, sha256, row delta) and stops before the write-rate-limit
  spend and the write itself. There is no local-git fallback anywhere in this tool: an
  unmintable verifier App token refuses at the mint step, `--dry-run` or not.

### Changed
- `statusgen`'s Evidence-actor check (desk-apps/07, F-verify-self-attest) is now merge-base
  scoped, the same shape `unrunGateChecks` and `witnessGate` already use: a `verified`/`done`
  brief whose Evidence section is not backed by the roster's verifier role is still a NOTICE
  when the closure predates `merge-base(HEAD, origin/main)` (the pre-cutover backlog), but is
  now a PROBLEM naming the actual rejected identity when the closure is one this branch newly
  made. The deliberate-spoof/tamper subclass (an Evidence commit dressed as the verifier —
  right name, wrong-or-absent account id) gets the SAME new-vs-backlog scoping and the stronger
  disposition: a new-closure impostor is a build-blocking PROBLEM naming the TAMPER signal, not
  the NOTICE it previously always was, while a backlog impostor stays a NOTICE. A shallow/grafted
  clone or an unresolvable merge-base still renders as could-not-check, never as either a pass or
  a failure.
- `spec/lifecycle-v1.md` §7.1 clause 2 names the new identity check and its exact scope (a
  post-cutover Evidence commit, evaluated per-transition against `closedAtBase`) so the spec
  never claims more independence than the lint enforces; the Verified cell and any Evidence
  commit outside that scope remain attribution-on-text, not identity, as before.
