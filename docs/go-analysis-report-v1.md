# Go analysis report v1

The lint runner writes one JSON object to `report.json` in its selected cache
(default `.go-analysis/`). This is the common input to the CI gate and the
qualgen integration owned by desk-tools/35. It reports upstream analyzer output;
it does not infer confirmed defects or a composite quality score.

Schema identifier: `go-analysis/v1`. The corresponding reviewed debt file uses
`go-analysis-baseline/v1`. Treat unknown schema versions as unsupported, not clean.

## Scan envelope

| Field | Meaning |
|---|---|
| `schema` | Exact schema identifier. |
| `repository` | Repository identity, supplied by CI through `GITHUB_REPOSITORY`. Local runs default to this repository; that default is not authenticated provenance. |
| `source_sha` | Actual analyzed checkout HEAD, as a full commit SHA. A PR head and a synthetic merge commit are different scans. |
| `base_sha` | Recorded trusted main SHA used for the ratchet. |
| `event_sha`, `pr_head_sha`, `pr_base_sha` | CI event and optional PR head/base identities. Neither substitutes for `source_sha`. |
| `run_id`, `attempt` | CI run and attempt identifiers. A local run uses `local` / `1`, which must not be accepted as forge evidence. |
| `scanned_at` | UTC observation time in ISO 8601 format. This is not the date a defect was introduced. |
| `family` | `lint` for this implementation. Security families require their declared extension before ingestion. |
| `contract` | SHA-256 over the exact analyzer config bytes, pin bytes and fingerprint version, joined with NUL separators in that order. Changed contracts are not silently comparable. |
| `fingerprint` | `source-window/v2`; a change requires a reviewed contract migration. |
| `tool` | Acquired-tool receipt: pins digest, analyzer binary digest, analyzer version, build-Go version and analysis-Go version. Absent if qualification could not complete. |
| `module_count`, `expected_scans` | Discovered tracked modules and planned module/configuration scans. Absent when discovery itself failed. |
| `coverage` | Every attempted module/configuration scan and its outcome. |
| `baseline` | Candidate and trusted baseline digests, plus the trusted baseline revision. In bootstrap, the revision is the human-approved immutable checkout. Absent if baseline loading failed. |
| `findings` | Normalized findings and, after a valid comparison, their dispositions. |
| `errors` | Controlled failure categories. Raw diagnostics and exception payloads do not belong in public reports. |
| `state` | `complete` or `could-not-check`. Completeness is separate from whether findings are accepted. |
| `gate` | `pass` or `fail`. A failed scan always fails the gate. |
| `exit_code` | `0`: complete with no regressions; `1`: complete with new/worsened findings; `2`: scan/contract/ratchet failure. |
| `totals` | Counts by disposition: `existing`, `new`, `worsened`, `resolved`. Authoritative only for a complete, validated comparison. |

The report is best effort on failure: a discovery, analyzer or baseline error
must still produce a failing report where the process can write one. Acquisition,
process termination and filesystem failures may prevent any report. The workflow
must surface missing publication, and the consumer must record could-not-measure;
an absent artifact is never an empty scan.

## Coverage and findings

Each coverage row has `module`, `configuration`, `state` and `findings`.
Configuration identifies GOOS/GOARCH, build tags, test inclusion and CGO policy.
A completed invocation also records `exit_code` and, when available, a `raw`
reference containing a cache-relative path and SHA-256. A failed invocation may
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
not identity; the source prefix distinguishes repeated calls on one line. Ambiguous identical anchors fail comparison instead of sharing an
allowance. A changed diagnostic at the same identity is `worsened`; changing a
source anchor conservatively creates a new finding. A source edit must not gain
an allowance by moving an old record to a new identity.

| Disposition | Interpretation |
|---|---|
| `existing` | Exact accepted finding and diagnostic; still outstanding debt. |
| `new` | No matching candidate allowance. This includes a removed allowance that is reintroduced later. |
| `worsened` | Matching identity but changed diagnostic hash; requires review/fix rather than automatic admission. |
| `resolved` | Present in the trusted baseline but absent from a complete current scan, with its candidate allowance removed. |

Accepted findings carry `exception` metadata: `first_seen`, `owner`, `work` and
`reason`. Resolution rows retain the corresponding baseline metadata. New
findings have no accepted exception. The initial baseline binds a measured source
revision and contract; normal PRs can remove entries, not expand or rewrite them.
Tool/config/fingerprint migrations require explicit reviewed admission. The current
composite action refuses post-bootstrap contract migrations; desk-tools/35 owns
the guarded migration input and its before/after evidence before enabling gosec.

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
