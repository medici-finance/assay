### Changed
- `deskdispatch --kit review` emits the review kit cut for the dispatched lane. A claim key ending `--security` gets the security cut, in which a pass only the correctness lane runs (design fit, the board-row flip check, the same-head re-approve exemptions, the prompt-audit procedure) is replaced by a short note naming its owner; the bare PR key gets the correctness cut; any other key gets the whole kit. No rule is dropped from a lane it binds, and a test table pins each section to its lanes (#2437).

### Added
- The review kit gains two clauses on every lane. Clause 17 tells a reviewer to read a file whole and once, to send independent reads in one request, and to gather a PR's state in one command, because every request re-reads the whole conversation. Clause 18 tells a reviewer to read a prepared review packet first when the assignment carries a `Packet:` line; it is inert when no such line is present (#2437).
