### Changed
- `statusgen --scan-issues` now derives `gate: human` on a new issue placeholder when the issue already carries the `risk:high` label (`risk:med` and `risk:low` do not gate). The intake-desk skill's scored-triage section says the gate is derived once at first write, and what a triage session does when the placeholder came first.
