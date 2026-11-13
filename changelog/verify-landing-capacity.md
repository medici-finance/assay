### Added
- `deskroster capacity --role LOOP --repo OWNER/NAME[,…]` reports a loop's remaining landing capacity read from the gate its landing write actually calls (per-PR, per-repo, new-PR, tool-wide and breaker meters), reserving in-flight items and pricing the proposed depth per item (two writes for a verify item). Every reading is dated and scoped to this host's ledger; provider and cross-host quota print as unknown, and an unreadable meter is could-not-check (exit 6), never spare capacity.

### Changed
- `deskboard throughput` carries a `landing` reading per stage and bounds its widen advice by it, beside the roster maximum and the actionable depth: a blocked or unread landing budget never yields a `deskroster set --width` command. The rate-limit gates and the capacity read now share one meter plan, so the report cannot drift from what enforcement refuses.
