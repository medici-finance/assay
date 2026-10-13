### Fixed
- `deskcomms send` no longer reports could-not-submit for a message the cell gateway accepted after a slow prose-gate consult: the submit deadline now sits above the gateway's consult bound.
- The comms prose gate and inbound router journal the decider's own reason (the reply line naming its answer) instead of whatever line a runner CLI printed first.
- Held-message and lane-sweep issue filings now use an argv `deskfile new` accepts, and a held message whose issue fails to file says so on stderr instead of staying silent.
