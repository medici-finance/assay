### Fixed
- Bind existing-change source refresh and cross-root board claim reads to the dispatching session's role credential, with complete-read checks and no ambient Git fallback in desk mode.
- Run the role-bound claim transport regressions explicitly in PR CI through the shared offline mutation/test invocation.
