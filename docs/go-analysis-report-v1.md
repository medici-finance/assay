# Go analysis report v1

The lint runner writes one JSON object to `report.json` in its selected cache
(default `.go-analysis/`). This is the common input to the CI gate and the
qualgen integration owned by desk-tools/35. It reports upstream analyzer output;
it does not infer confirmed defects or a composite quality score.

Schema identifier: `go-analysis/v1`. The retained debt register, read only by the
debt report, uses `go-analysis-baseline/v1`. Treat unknown schema versions as unsupported, not clean.

## Scan envelope

| Field | Meaning |
|---|---|
| `schema` | Exact schema identifier. |
| `mode` | `gate` (merge base compared with head) or `debt-report` (every finding at the analyzed revision; never gates on findings). |
| `repository` | Repository identity, supplied by CI through `GITHUB_REPOSITORY`. Local runs default to this repository; that default is not authenticated provenance. |
| `source_sha` | Actual analyzed checkout HEAD, as a full commit SHA. A PR head and a synthetic merge commit are different scans. |
| `base_sha` | Trusted main SHA the merge base was computed against. Empty in `debt-report` mode. |
| `merge_base_sha` | `git merge-base base_sha source_sha`: the revision the head is compared with. Equal to `source_sha` for a push to main, when one scan serves both sides. Gate mode only; absent if it could not be resolved. |
| `event_sha`, `pr_head_sha`, `pr_base_sha` | CI event and optional PR head/base identities. Neither substitutes for `source_sha`. |
| `run_id`, `attempt` | CI run and attempt identifiers. A local run uses `local` / `1`, which must not be accepted as forge evidence. |
| `scanned_at` | UTC observation time in ISO 8601 format. This is not the date a defect was introduced. |
| `family` | `lint` for this implementation. Security families require their declared extension before ingestion. |
| `contract` | SHA-256 over the exact analyzer config bytes, pin bytes and fingerprint version, joined with NUL separators in that order. Changed contracts are not silently comparable. |
| `fingerprint` | `source-window/v3` (v3 drops staticcheck related-information notes, which annotate a primary finding); a change requires a reviewed contract migration. |
| `tool` | Acquired-tool receipt: pins digest, analyzer binary digest, analyzer version, build-Go version and analysis-Go version. Absent if qualification could not complete. |
| `module_count`, `expected_scans` | Discovered tracked modules and planned module/configuration scans. Absent when discovery itself failed. |
| `coverage` | Every attempted module/configuration scan of the head and its outcome. |
| `base_coverage` | Gate mode: every attempted scan of the merge base and its outcome. |
| `baseline` | Debt-report mode only: the retained register's digest and `role: reporting-only`. Absent when no register exists. The gate never reads it. |
| `debt` | Debt-report mode only: `recorded` and `unrecorded` finding counts against the register. |
| `findings` | Normalized findings and, after a valid comparison, their dispositions. |
| `errors` | Controlled failure categories. Raw diagnostics and exception payloads do not belong in public reports. |
| `state` | `complete` or `could-not-check`. In gate mode both the head and the merge-base scans must be complete. |
| `gate` | `pass` or `fail`. A failed scan always fails the gate. A complete debt report always passes. |
| `exit_code` | `0`: complete with no regressions (or any complete debt report); `1`: gate mode, complete with new/worsened findings; `2`: scan, contract or comparison failure. |
| `totals` | Counts by disposition: `existing`, `new`, `worsened`, `resolved`. Authoritative only for a complete, validated comparison. |

The report is best effort on failure: a discovery, analyzer, merge-base or comparison error
must still produce a failing report where the process can write one. Acquisition,
process termination and filesystem failures may prevent any report. The workflow
must surface missing publication, and the consumer must record could-not-measure;
an absent artifact is never an empty scan.

## Coverage and findings

Each coverage row has `module`, `configuration`, `state` and `findings`.
Configuration identifies GOOS/GOARCH, build tags, test inclusion and CGO policy.
A completed invocation also records `exit_code` and, when available, a `raw`
reference containing a cache-relative path (`raw/head/N.json` or
`raw/merge-base/N.json`) and SHA-256. A failed invocation may
carry an `error` category; its finding count remains unknown. Raw references are
provenance, not instructions to fetch or execute content. Raw files are not
uploaded by the lint workflow hunk.

A finding records `analyzer`, `rule`, `module`, `configuration`, repository-relative
`path`, hashed source `anchor`, stable `id`, hashed `diagnostic`, and display
`line`/`column`. Source snippets, diagnostic text and absolute filesystem paths
are not public report fields.

The identity is the SHA-256 of canonical JSON over analyzer, rule, module,
configuration, path and anchor. Canonical JSON sorts keys with compact separators.
The anchor hashes the enclosing top-level declaration's symbol, the smallest
distinguishing neighborhood of trimmed source lines inside that declaration (up to
64 significant lines on each side), plus the trimmed source prefix
before the diagnostic column. Blank lines and standalone `//` comments outside
the diagnostic line are excluded. Line/column numbers are display coordinates,
not identity; the source prefix distinguishes repeated calls on one line. Ambiguous identical anchors fail comparison on either side instead of
being matched. A changed diagnostic at the same identity is `worsened`; changing a
source anchor conservatively creates a new finding. A source edit must not pass
by moving a merge-base finding to a new identity.

| Disposition | Interpretation |
|---|---|
| `existing` | Same identity and diagnostic at the merge base and the head; still outstanding debt. In a debt report, every finding. |
| `new` | Identity absent from the merge base. Fails the gate. |
| `worsened` | Identity present at the merge base with a changed diagnostic hash. Fails the gate. |
| `resolved` | Present at the merge base, absent from a complete head scan. Needs no file edit. Never produced by an incomplete scan. |

In gate mode every finding's `exception` is null: what passes is decided by the
merge-base scan, not by recorded allowances. In a debt report, a finding with a
record in the retained register carries `exception` metadata (`first_seen`,
`owner`, `work`, `reason`); an unrecorded one has null. Tool/config/fingerprint
migrations require explicit reviewed admission: the gate refuses a merge base
whose analysis policy differs from the runner's. desk-tools/35 owns the guarded
migration input and its before/after evidence before enabling gosec.

## Consumer rules

Qualgen must validate the envelope, dispositions and coverage before rendering
measured totals. The gate's verdict and the ingestion result are separate: a
complete scan with regressions has valid measurements and a failing gate; a
partial scan has a failing gate and unknown complete totals. Partial findings may
be retained as observations but cannot clear outstanding debt. Preserve the last
complete snapshot as historical context, visibly dated, rather than making it
look like the failed current scan.

A workflow consuming an artifact must bind it to the expected repository,
workflow, run/attempt and actual source SHA using forge metadata. Self-asserted
JSON fields do not authenticate an artifact. Do not run code from the report or
from an untrusted checkout in a privileged reporting job. PR renders are previews;
only the existing main writer may append durable history and write QUALITY.md.

Deduplicate an exact retry by scan identity and content. Reject the same identity
with different content; preserve distinct attempts and source commits. Compare
trends only across complete scans with compatible tool/configuration and coverage.
Never add overlapping linter diagnoses into a claimed defect count or invented
quality score. Missing, stale, malformed, wrong-head or incomplete artifacts render
could-not-measure with a reason, never measured zero.

Desk-tools/35 extends this contract with vulnerability IDs, affected versions,
symbol reachability, database identity/freshness and dated exception review
metadata. That extension must specify comparability and validate real scanner
output before it is treated as implemented.
