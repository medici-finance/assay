# deskboard CLI compatibility

The command tree replaces the argument loop. Domain handlers, output rendering, roster
custody, scope, risk admission, GitHub/GitLab readers, delta snapshots and audit writers
retain their existing owners. The already-admitted shared cli adapter owns Cobra/Viper;
no dependency was added or promoted.

## Settings matrix

The pre-port matrix is committed as `tools/desk/cmd/deskboard/testdata/cli-matrix.json`.
No CLI setting reads a config file. Only the quota floor has an environment source.

| Settings | Sources and default | Empty and repeat behavior |
|---|---|---|
| table, json, delta, quiet | flags; false | scalar last wins |
| out | flag; empty | empty means stdout; last wins |
| min-age-hours | flag; 48 | empty/zero/negative refused; last wins |
| merge-now-threshold, unreviewed-threshold | flags; 20m, 30m | empty/malformed duration refused; last wins |
| tier | flag; no filter | explicit empty/unknown refused; last wins; canonical vocabulary unchanged |
| rate-floor | flag > trimmed DESKBOARD_RATE_FLOOR > 500 | explicit empty flag refused; empty/whitespace env falls through; 0 disables; last flag wins |
| throughput | hidden legacy flag alias; false | selects throughput without a verb |

Every invocation uses fresh declared settings and a fresh Viper instance. The environment
fallback has its own binding so present-empty retains the existing trim/fallback rule;
it cannot silently inherit Viper's present-empty override rule. Domain configuration
(DESK_ROOTS, STATUSGEN_BIN, roster and fixture-only gate-score readers) stays in its
existing domain reader, with no new sources or precedence.

## Consumer inventory and replay

Pickup used a repository-wide search for deskboard invocations, including scripts,
Go sources, reference docs, and every skill. The registry's six starting consumers
remain valid. No consumer uses single-dash long flags; those remain refused.

| Caller family | Invocations kept and exercised |
|---|---|
| worker-desk and its dispatch runbook | dispatch; stalled --min-age-hours N; health; throughput; queue; nextup warning |
| pr-review-desk | actions; actions --delta --quiet; actions --out path; prs --out path |
| verify-desk and coordinator | awaiting; nextup; queue; health; throughput |
| deskboot, cellctl launcher and shell parity fixtures | prs; actions; --version; dispatch |
| reviewloop board reactor and askassay readers | actions; prs; queue; files repo pr [path]; diff repo pr; reviews repo pr |
| verifyloop adapter | awaiting |
| assay-inbox and deskinbox flow | throughput --json |
| desk shell reference, tool README, adoption docs | prs --table; queue; health; --version; global output flags |
| quality report and historical inventories | source references, no executable help quotation changed |

`TestCLILegacyForms` replays every verb/alias and long flag form, scalar repeats,
termination, Windows/POSIX paths, and exact JSON/table renderings. Existing integration
tests continue to run the real board handlers against their recorded/stubbed forges.
The tier domain check is also exercised with the CLI bypassed.

## Observable changes

Generated root and verb help, `help <verb>`, and version read no roster or credential and
start no child or telemetry write, even with hostile configuration. Completion stays
refused. Long equals forms now work as required by the shared contract. Unknown flags,
malformed values and positional counts fail before execution. Parse refusals retain
legacy exit 5 (the same compatibility mapping as issueboard); domain exit codes stay
unchanged. Extra positionals that the old loop ignored are now usage refusals. Existing
consumers do not use those forms. Error prose is generated or value-redacted; board
stdout is unchanged. Incompatible output mode combinations refuse before the board read.

The old README suggestion to use version for a roster dump is replaced with
`ASSAY_CONFIG_ECHO=full deskboard prs --table`, an execution that retains the roster echo.
