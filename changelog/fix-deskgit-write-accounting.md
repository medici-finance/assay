### Fixed

- Fetch audit records no longer spend the push write budget, and budget refusals no longer charge themselves as uncertain writes. Existing write caps and circuit breakers remain unchanged.
