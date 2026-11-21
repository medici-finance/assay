### Fixed
- The outbound body and diff secret scan no longer refuses a Go build-info `DefaultGODEBUG` row, or a `GODEBUG=` environment assignment, as a high-entropy run. It admits the list only when every comma-joined item is a lowercase `name=<digits>` setting, so a token written behind the same key is still refused (#6821).
