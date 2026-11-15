# Plant for the class-pass example reader

This file is read by the reader's own test, never by a session. The group below is
quarantined, so its item must have no record. A repair record follows the expect line
anyway, and the reader must report it.

## Group: a quarantined item that was routed

outcome: quarantined

expect: none

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#141",
  "opened": "2026-01-08T08:00:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "route": "repair",
  "successor": "example-org/example#141",
  "evidence": "example-org/example#141"
}
```
