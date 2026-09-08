---
name: alpha
description: A neutral fixture skill body.
---

# Alpha

Dispatch one worker agent per task (`capability:dispatch-worker`); to resume a
running worker with its prior context, message it (`capability:message-agent`).
Each worker runs in its own isolated workspace (`capability:isolate-workspace`),
NEVER the shared checkout.

Bindings for your harness: see `../../references/<harness>.md`.
