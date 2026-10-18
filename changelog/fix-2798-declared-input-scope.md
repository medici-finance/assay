### Fixed
- Verification wake receipts parse declared bare and inline paths, expand directories, globs and finite braces, and preserve membership/absence so input changes cannot silently remain held. Unsupported declarations and incomplete historical scopes are explicit could-not-check/refusal states; original observation binding and historical receipts remain intact.
