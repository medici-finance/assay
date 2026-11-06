### Fixed

- `deskevidence` lands Evidence on a GitHub brief whose text holds a control-character escape such as a literal backslash-u-0003. The go-gh transport rewrites those escapes in the GraphQL `Blob.text` answer, so the snapshot now reads the bytes by blob id whenever `text` does not hash to that id, and still refuses bytes that cannot be proven against it. A failed blob read refuses as unverifiable, never as a missing file.
