### Changed
- `TestForgeLocalAuthorityCustody` failure messages now carry the failing step's error value and, once the producer exists, its fixture stage and operator read-back. An intermittent failure of the `private-volume-leak` control's stages can then be read from one log line. The step text each message starts with is unchanged, so the mutation driver's witness match is unaffected.
