### Fixed

- The desk self-containment scan no longer reads a JSON-escaped or Go-quoted regex character class (`\x00`, `\u001f`, `\U0000001f` escapes after a run of backslashes) as a UNC machine path, so a mutation spec or test fixture holding such an anchor no longer needs a human scan override.
