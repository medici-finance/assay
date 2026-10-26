### Fixed
- Unify YAML brief parsing, numeric-ID trust checks and fenced work-link interpretation across statusgen and desk tooling. Supplied bot IDs must match their pins, quoted risk answers remain risk-bearing, and fenced examples grant no delivery credit.
- A risk answer written anywhere outside the top-level `risk:` mapping (for example under a misspelled `risks:` key or nested under another mapping) now makes the brief frontmatter unreadable, so desk readers treat the brief as risk-bearing instead of reading it as risk-free.
- A brief frontmatter block that holds more than one YAML document (for example a `...` line or a `--- <content>` line inside the fence) is now unreadable to desk readers, so a risk answer or `gate: human` after the marker can no longer be dropped.
- An unquoted date or timestamp in `blocked-until:` or `in-repair:` is read as written, so a dated deferral marker defers the brief instead of making its frontmatter unreadable.
- An attestation binding whose brief path cleans to `..` or `.` is refused under every profile.

### Changed
- Move common trust, risk-path, verdict, attestation and closure rules into shared offline libraries while retaining explicit legacy, historical-read and attestation-capability profiles.
- statusgen verdict transcription now refuses a signed verdict block that declares a role other than `verifier`, matching the desk verifier's check. A block that declares no role is still read as the verifier role.
