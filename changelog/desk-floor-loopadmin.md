### Fixed
- The desk-tools compile job in CI now also runs when a PR changes `loopadmin/`, `statusgen/` or `workflow/`, the remaining modules `tools/desk/go.mod` pulls in through replace directives; before, a change to one of them alone skipped the compile.
