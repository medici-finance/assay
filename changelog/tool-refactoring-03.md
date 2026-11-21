### Changed
- Verify-row and Context `files:` reading now lives in one place, `statusgen/briefdoc`: row identity, the Class cell, the `cmd:` marker, prose-led and shell-comment refusal, and the strict declared-input grammar. statusgen's verify code and the `tools/desk` executors and declared-scope projection call it and keep no lexical copy.
- Retired the ported copy of the prose-led rule in `tools/desk/internal/verifycmd` and the AST-diff drift test that guarded it. A census-and-planted-parser class guard (`TestVerificationDefinitionClassGuard`) replaces it.
