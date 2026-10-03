### Added
- Design-decision record `DR-cellctl-go-port` (`docs/streams/decisions/`): the operator's
  ruling that `cellctl` is ported from bash to Go, with the bash kept as the oracle until the
  Go port reaches parity. Brief desk-containers/10 now cites it through `design:`, which is what
  the design-approval gate reads before that brief can move to `in-progress`. The record
  describes the tree as it stands (the Go `cellctl` already ships in the release tarball since
  #1377) and does not decide the brief's cutover sign-off, which stays its own human gate.
