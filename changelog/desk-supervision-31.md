### Added
- The desk tools now keep a CI check history: one `ci-check-v1` line per finished check run or terminal commit status they read (repo, head SHA, PR, check name, attempt, conclusion, duration), so flake rate and check duration can be measured later. Check output, URLs and who posted a check are never recorded.
- The history lives in the local desk state directory as `ci-checks.jsonl`, rotated daily, never committed and never deleted by the tools; `deskkit.LoadCIChecks` reads it back de-duplicated.
- Recording costs no extra forge reads: the Forge decorator records from the `ChecksAtHead` and rollup results the tools already fetch, and a recorder failure never changes what the read returns.
