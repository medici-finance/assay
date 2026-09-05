### Added
- **`commsgw`** — the per-cell message gateway: the one chokepoint every
  inbound cell message crosses. A deterministic pre-check pipeline (mTLS peer
  accept, envelope parse-or-refuse, signed-assertion verify, lane ACL incl. the
  cross-cell pair + verb allow-set, `Claim()` dedupe, per-sender rate/budget,
  kill switch) runs identically for cross-cell traffic (an A2A JSON-RPC server
  on the pinned `a2a-go` SDK, mTLS-fronted) and within-cell traffic (a loopback
  Unix socket matching `deskcomms`'s existing client wire shape). Config-off by
  default: every `ASSAY_COMMS_*` enable key is required, any one absent refuses
  to serve. Accepted messages are durably queued (`internal/commsqueue`) for
  `commsloop` to drain. On every accepted cross-cell message the gateway emits
  one deskd inbox item of kind `cross-cell`; an emission failure quarantines
  the message rather than dropping it.
- **`commsloop`** — the paired drain consumer: the fifth implementation of the
  frozen `loopengine.Loop` contract. Report-class messages (`status`,
  `metrics`, `help-offered`) land done+journaled with no session ever fired;
  everything else quarantines (held mailbox + a filed issue) pending the
  prose router. An independent, second lane-ACL check at the routing boundary
  (a different file from the gateway's own check) catches a message that
  somehow bypassed the gateway's precheck.
- `internal/comms/laneacl.yaml`'s `# OPEN DECISION` marker is replaced with a
  public-safe citation of the cross-cell verb ruling (date + bare decision
  record number only).

### Notes
- `isReportClass` (which verbs land immediately vs. await the prose router) is
  a documented, reviewable judgment call pending that router's own arrival —
  see `cmd/commsloop/routing.go`.
