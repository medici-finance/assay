### Fixed
- windows-port/13 Verify row 7 now runs the `consumers:` gate against the commit that implemented the brief and passes only on a `CORROBORATED` line (one or more corroborated, 0 disproved). The old row ignored its own brief argument, ran on merged main where there is nothing to judge, and passed on a trailing `echo $?`, so it also passed with a DISPROVED claim.
