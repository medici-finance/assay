# Issue discovery projections

The issue reader selects a population from the complete typed open-issue listing
before paying trust, comment and label-history reads. Collection stays in the
existing role-bound forge adapter; pure selection lives in
`tools/desk/internal/issueflow`. No cache, service, new credential or pool is added.

| Caller | Producer | Population and required enrichment |
|---|---|---|
| Worker skill and dispatch runbook | `issueboard issues --projection work-ready` | Unrepresented candidates without excluded/decision/addressed/help-wanted labels. Selected external authors still need current blessing and numeric identity; selected work still needs complete label history. Missing active placeholders retain positive state reads to detect a partial listing. A history hold is AWAIT, never dispatchable. |
| Intake skill | `issueboard issues --projection intake` | All open issues and active closed placeholders, with original trust/quarantine, escalation, addressed-response, label-history and positive closure behavior. `issueboard intake` separately reads local raw intake. |
| Addressed desk inbox | `issueboard issues --projection inbox --to ROLE` | Only the named addressee, selected before trust and comment enrichment; retains complete escalation histories and conservative overflow. Role aliases remain the existing addressee owner's responsibility. |
| Human decision inbox script | `deskinbox discovery REPOS...` | Exact case-sensitive union of urgent, needs-decision, question and help wanted, ordered urgency then creation age. This is distinct from the addressed-role population. Selection precedes detail; the script retains its existing detail, positive ruling screen, cards, flow and partial-result exit semantics. |

Projection output declares population, current invocation cutoff, enumeration
completeness and row-level history degradation. It is an observation set, not an
atomic provider snapshot, and has no validity beyond the collecting invocation.
An empty worker result cannot establish intake/inbox idle. Failed complete-list
reads fail the whole issueboard projection; existing row-level history overflow
still degrades conservatively. The human inbox preserves partial output plus exit 2.
Neither observation grants write, claim, admission or executor authority.

## Compatibility and rollback

| Plugin/script and producer | Supported path |
|---|---|
| Older plugin with new producer | Legacy full/default/`issues`/`intake`/`--to` forms remain available, with unchanged tables and exit behavior. |
| New plugin with matching producer | Check `issueboard --version` for `issue-projection-v1` before selecting projected forms. New source is qualified together in this PR; a released pin must contain this capability. |
| New plugin with older producer | Use the documented legacy spellings before discovery, preserving manual readiness checks. Do not send an unsupported selector or retry after a partial projected read. |
| Inbox script with compatible producer | Auto mode checks `deskinbox --version` for `issue-discovery-v1` before any discovery. It runs exactly one typed listing per requested repository; no legacy label queries run in that arm. `ASSAY_DESKINBOX` selects a pinned executable. |
| Inbox script with absent/older producer | Auto selects legacy before discovery. `ASSAY_INBOX_READER=typed` instead refuses before any discovery; use it to qualify the migrated arm without broad reads. |
| Explicit rollback | `ASSAY_INBOX_READER=legacy` restores shell label discovery. Restore the previous plugin and producer pins together for worker/intake. No live pin, installation or executor switch is performed here. |

Never fall back after a compatible producer starts: permission errors, malformed
output and page-cap overflow remain incomplete observations. The existing typed
reader's bounded serial fallback is used; no unsupported batching API is invented.
Its per-process caps do not coordinate independent processes or guarantee fleet quota.

## Remaining broad readers and acceptance holds

The default full board, `tools/desk/README.md` operator recipe and independently
selected legacy compatibility paths remain supported broad readers. Deskboard's
intake-depth note correctly names the separate local `issueboard intake` read;
dispatch/stalled/health remain their own board populations and are not switched to
a narrow worker result. Independent watcher health, all trust/edit checks, audit,
budget/breaker/admission controls and consequential rechecks remain required.

Both tools use the canonical `deskkit.PerfRun` / perfrec recorder. Deskinbox now
selects compiled operation names after parsing, including discovery; private argv
never becomes a label. At this source revision that shared recorder emits perf-v2.
Shared perf-v3 schema/context and provider timing/points are still owned by
desktools-v2/63–65; this draft holds completion until those canonical producers
are delivered and both issue callers are integrated and qualified against them.
No local perf-v3 schema, writer or fabricated point value is introduced. The
separate desk-tools/38 verification prerequisite is also retained; source tests
here do not certify it or authorize activation.
