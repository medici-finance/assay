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
retain their board/version-stamp invocations. Roster-echo consumers require the migration
below. No consumer uses single-dash long flags; those remain refused.

| Caller family | Invocations kept and exercised |
|---|---|
| worker-desk and its dispatch runbook | dispatch; stalled --min-age-hours N; health; throughput; queue; nextup warning |
| pr-review-desk | actions; actions --delta --quiet; actions --out path; prs --out path |
| verify-desk and coordinator | awaiting; nextup; queue; health; throughput |
| deskboot, cellctl launcher and shell parity fixtures | prs; actions; --version; dispatch |
| reviewloop board reactor and askassay readers | actions; prs; queue; files repo pr [path]; diff repo pr; reviews repo pr |
| verifyloop adapter | awaiting |
| assay-inbox and deskinbox flow | throughput --json |
| desk shell reference and tool README | prs --table; queue; health; --version for build stamps; global output flags |
| adoption guide and tool README roster inspection | migrated from version echo to ASSAY_CONFIG_ECHO=full deskroster repos --scope write; no board roots or forge required |
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

The adoption guide's applicability probe, verification command and explanations, and the
README's roster-dump example now use `ASSAY_CONFIG_ECHO=full deskroster repos --scope write`.
This existing local reader owns effective write-scope inspection; no new configuration
source or authority is introduced. The guide retains the reader's exit code and requires
both a successful inspection and the intended effective repos/roles. Missing or malformed
configuration and empty write scope exit 6 with diagnostics; `configured=true` alone does
not establish complete scope. Initial inspection needs no configured board roots.

The replacement guide commands were exercised against the built tools with a hermetic
config home, no board roots, and only the inspection binary and required shell utilities
on PATH. A valid roster reports its scope
and exits 0; missing, malformed and empty-repo rosters exit 6 with their configuration
diagnostics. Version continues to exit 0 with zero stderr in all four cases. The old
applicability pipeline misclassified the valid roster as pre-roster; the old verification
grep exited 1. These consumer checks supplement the parser replay above; parser replay
alone did not exercise the former roster echo.

The caller sweep also classified install and Docker version checks, launcher/performance
fixtures and README source-SHA comparisons as build-stamp consumers that remain valid.
Recorded verifier Evidence and old incident reports are historical evidence, not current
roster-inspection instructions, and are retained unchanged.
