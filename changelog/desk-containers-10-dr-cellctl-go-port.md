### Added
- Design-decision record `DR-cellctl-go-port` (`docs/streams/decisions/`): the operator's
  ruling that `cellctl` is ported from bash to Go, with the bash kept as the oracle until the
  Go port reaches parity. Brief desk-containers/10 now cites it through `design:`, which is what
  the design-approval gate reads before that brief can move to `in-progress`. The record does
  not decide the cutover (whether and when the release tarball ships the Go binary), which
  stays the brief's own human sign-off.
