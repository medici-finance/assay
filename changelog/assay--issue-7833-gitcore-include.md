### Fixed

- The library read path now reads a repository checked out by actions/checkout, which keeps the job's credential in a separate file pulled in through `includeIf` entries. Previously every delivering commit was refused in a workflow checkout, so verify-gate-close could not check any fail-first base (#7833). Only that credential-only include shape is accepted; any other include, key, condition or target is still refused.
