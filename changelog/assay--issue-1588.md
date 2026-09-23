### Added
- New `gate: human` brief `desk-supervision/23`: Evidence-only changes land on a PR-required main without a PR. A validator admits only Evidence rows and outcome-log appends, a dedicated lander App is the only identity that may skip the PR rule, the verifier App keeps no write to main, and a rejected landing falls back to the batch Evidence PR.
