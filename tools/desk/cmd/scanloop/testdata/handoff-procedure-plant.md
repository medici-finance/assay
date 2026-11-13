# Plant for TestHandoffExamplesFlagPlant

Three examples that each disagree with the shipped assessment. The test passes only
when every one is reported.

A direct repair that is labelled as a duplicate:

expect: observed duplicate

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#1",
  "opened": "2026-11-01T09:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "delivered": false,
  "route": "repair",
  "successor": "example-org/example#1",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "Names the failing check."
}
```

A record with a field the schema does not define:

expect: observed repair direct

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#2",
  "opened": "2026-11-01T09:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "route": "repair",
  "successor": "example-org/example#2",
  "owner": "someone",
  "evidence": "Names the failing check."
}
```

A repair whose successor is another issue, labelled as a direct repair:

expect: observed repair direct

```json
{
  "schema": "intake-handoff-v1",
  "item": "example-org/example#3",
  "opened": "2026-11-01T09:30:00Z",
  "revision": 1,
  "deciding_role": "intake-desk",
  "disposition": "repair",
  "delivered": false,
  "route": "repair",
  "successor": "example-org/example#99",
  "author_tier": "",
  "external_role": "",
  "required_action": "",
  "wake_condition": "",
  "evidence": "Names the failing check."
}
```
