### Added
- desk-containers/15 brief: `cellctl desk --force` diagnoses a failed desk boot and applies the matching repair. Safe repairs run at once; destructive ones are confirmed first (`-y` confirms all) and always preserve what they displace. A live session is never ended. Authoring only; nothing is implemented.
