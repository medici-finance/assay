### Fixed
- `cellctl` refuses to boot when its fetch fails, instead of silently falling back to a previous boot's `main`. The refusal names the usual cause, a credential helper answering with an expired token, and releases the fetch lock.
