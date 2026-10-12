### Fixed
- `deskpr create` can open a draft PR for an already-published Evidence branch while preserving its commit objects and checking every newly introduced commit. Destination proof is bound to the plain push; offline checks explain their remote-proof limitation.
- Keep the identity-check snapshot bound to the exact source commit published by create and update, including when local refs change during checking.
