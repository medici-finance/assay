### Fixed
- The desk secret scan and the diagnostic redactor now recognise GitHub App user-to-server tokens (`ghu_`) and refresh tokens (`ghr_`), the two documented GitHub token prefixes they previously missed. A pasted token of either kind is now refused in outbound text and redacted from `DESK_TRACE` output, like the other four prefixes.
