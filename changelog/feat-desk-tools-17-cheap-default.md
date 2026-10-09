### Added
- `cellctl` records who owns each model pin (default or explicit) and repins default pins older than the TTL (weekly by default) to the mid-tier model at boot and on `cellctl models reset`; explicit pins are never touched.
