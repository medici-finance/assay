### Fixed
- The worker-desk skill no longer tells its standing loop to touch the `HEARTBEAT` dead-man lease on every tick. Renewal stays a human act, as the desk tools' runtime-state contract states, and a stale-lease refusal now means quote it, print one loud line, and stand down.
