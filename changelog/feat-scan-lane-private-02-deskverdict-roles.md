### Added
- `deskverdict sign`/`verify` gain a `--key verifier|issue-loop` role selector: it picks
  WHICH role's key is used, never where a key comes from. Both roles' public keys stay
  repo/Actions VARIABLES (`ASSAY_VERIFIER_PUBKEY` / `ASSAY_ISSUE_LOOP_PUBKEY`) — no key
  material of any kind is committed to the tree for either role. The signed block now
  declares its signing role, and `verify` refuses a block whose declared role differs
  from `--key`, before any signature arithmetic runs.
- `statusgen --transcribe-scan-delta`: the R-7 clause-4 cross-repo scan-delta verify path.
  It sweeps open issues on the home repo for a payload signed with the issue-loop role key,
  behind the same R-7 enactment gate as `--transcribe-scan`, and checks container-author
  identity, the role-declared RS256 signature, body-unedited timeline, a per-entry API
  re-check where readable, and same-repo-entry refusal — each layer naming the clause it
  refuses under. Ships INERT; adds no new arming path.
