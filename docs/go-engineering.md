# Go engineering and library choices

Repository engineering policy, 11 October 2026. Applies to new and changed Go
code in Assay; it is not a requirement on projects adopting the Assay methodology.
The catalogue selects defaults, not installed versions or completed migrations.

## Worker rule

Before implementing a common capability, read this catalogue, the relevant module's
`go.mod`, and its existing adapter. Reuse the canonical implementation. When none fits,
use the standard library or the selected upstream package; do not build a competing
parser, pool, router, logger, migration engine, retry mechanism or protocol client merely
because it looks small. Evaluate established OSS before choosing a custom implementation.
A missing dependency is a dependency-review task, not a reason to rebuild its functionality.

A selected package is the default for its stated capability, not a requirement to import
it into every program. Plain `net/http` is enough for a simple endpoint; use the selected
router when routing/middleware composition needs it. Libraries receive typed configuration;
Cobra and Viper stay at application entrypoints. Use small adapters for project policy,
redaction, credentials and test seams. Do not wrap every upstream function or grow a second
framework around a library.

An exception belongs in the implementation PR: capability needed; selected/existing option
considered; concrete mismatch with a reproducer, measurement or API limitation; smallest
alternative; owner and migration/removal consequence. A reviewer judges the exception.
“I can write it faster” and “avoid a dependency” alone are not sufficient. A stdlib-only
package boundary, unsupported platform, licence restriction or measured correctness/performance
gap can be sufficient. Existing human and security gates still apply.

## Catalogue

Choices are per capability. Reuse an existing adapter before importing its upstream directly.
“Selected” below means preferred for new work after normal dependency admission, not present
in `go.mod`. Package major versions identify API families; exact versions live in module
manifests or pinned tool definitions, never floating `@latest` in builds.

| Capability | Default | Current adoption and boundary |
|---|---|---|
| Command trees, flags, help | [Cobra](https://github.com/spf13/cobra), through `tools/desk/internal/cli` in the desk module | Existing direct dependency. Follow the [CLI contract](streams/desktools-v2/cli-contract.md); standalone modules cannot import another module's internal package. No parallel custom parser. |
| Application configuration | [Viper](https://github.com/spf13/viper), through the same adapter where importable | Existing direct dependency. Explicit keys/sources, fresh instance, typed validated options; preserve empty/unset and precedence. No global Viper, ambient config search or new credential reads. |
| Structured diagnostics | [`log/slog`](https://pkg.go.dev/log/slog) | Stdlib default for new structured logs. Existing `deskkit.ReportError`, `Scrub` and `DESK_TRACE` remain the current desk diagnostic contract. Migrate their owner once with output/redaction tests; do not bolt a parallel logger onto each command. |
| HTTP transport | [`net/http`](https://pkg.go.dev/net/http) | Explicit reusable clients, cancellation, timeouts, body limits and closing. Existing admitted forge transports remain the access path; a new client grants no authority. |
| HTTP router/middleware composition | [`github.com/go-chi/chi/v5`](https://github.com/go-chi/chi) | Selected when `http.ServeMux` is insufficient for the service's composition. Keep `net/http` handlers; no second router framework for the same service. |
| PostgreSQL access and pool | [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx), `pgxpool` | Selected for the planned PostgreSQL core. Do not build a pool or wire driver. `database/sql` compatibility uses pgx's stdlib adapter when an integration requires it. No new ORM by default. |
| SQL-to-Go query generation | [sqlc](https://sqlc.dev/) | Selected when the persistence module needs generated typed queries. SQL remains explicit; pin the generator and check regenerated output. A few straightforward pgx queries do not require a generator. |
| Database schema migrations | [`github.com/pressly/goose/v3`](https://github.com/pressly/goose) | Selected; SQL migrations, explicit execution owner, tested upgrade/recovery. Do not invent a migration ledger/runner. Library availability does not authorize automatic startup DDL or imply every migration is reversible. |
| Concurrent task groups / weighted limits | [`golang.org/x/sync/errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup), [`semaphore`](https://pkg.go.dev/golang.org/x/sync/semaphore) | x/sync is indirect in the desk module; direct use needs a direct requirement. Bound concurrency and cancellation. A local goroutine group is not a durable workflow executor or claim authority. |
| In-process rate limiting | [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) | x/time is indirect in the desk module. Use for token-bucket mechanics; distributed quotas and provider rate-limit meaning remain their owner's contract. |
| HTTP retry mechanics | [`github.com/hashicorp/go-retryablehttp`](https://github.com/hashicorp/go-retryablehttp) | Indirect in the desk module. Use only behind the admitted transport with explicit bounds, scrubbed logging and an operation-specific retry predicate. Never rely on generic retry defaults for writes or layer retries over an SDK's own retries. Existing loopengine retry classification still decides refusals, unknown outcomes and retry-after; do not replace that domain policy. |
| UUID generation/parsing | [`github.com/google/uuid`](https://github.com/google/uuid) | Indirect in the desk module. Prefer it when a contract requires UUIDs. Do not change existing identifier formats or use UUIDs as secret credentials. |
| JSON | [`encoding/json`](https://pkg.go.dev/encoding/json) | Stdlib default. Schemas, unknown-field policy and canonical hashing are contract-specific; do not swap codecs for a benchmark without compatibility evidence. |
| YAML | [YAML organization's go-yaml](https://github.com/yaml/go-yaml) | Existing modules pin `gopkg.in/yaml.v3`. The maintained upstream now directs new work to `go.yaml.in/yaml/v4`; select that for a new isolated YAML contract after version admission. Shared existing formats stay on their current codec until one coordinated migration proves tags, nodes, aliases, duplicate-key and scalar compatibility. Do not independently mix codecs across readers of one contract. |
| Filesystem notifications | [`github.com/fsnotify/fsnotify`](https://github.com/fsnotify/fsnotify) | Indirect in the desk module. Use for supported local filesystem events; overflow, missed events and unsupported filesystems require explicit reconciliation. Notifications are not a durable queue. |
| Traces and metrics | [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) | Selected for new service instrumentation. Libraries use APIs; applications configure SDK/exporters explicitly. No implicit exporter network access. Diagnostic telemetry does not replace audit or evidence records. |
| Tests and value diffs | `testing`, `httptest`, `t.TempDir`, fuzzing; [go-cmp](https://github.com/google/go-cmp) for structural diffs | Stdlib first for test control, go-cmp selected for rich value comparisons. Do not build another reflection-based comparison library. Production equality and admission never depend on go-cmp. |
| Git and forge protocols | Existing gitcore and admitted forge adapters over their qualified upstream packages | Preserve the current go-git, go-gh and GitLab SDK owners. Readers, credentials, compare-and-delete and bounded native-Git exceptions need their existing qualification; do not add a new Git/forge client because an API looks convenient. |

## Common behavior

- CLI syntax, configuration resolution and exit behavior have one home in the
  [CLI contract](streams/desktools-v2/cli-contract.md); this document does not redefine them.
- Results go to stdout; diagnostics go to stderr. Machine-readable output must remain
  parseable without stripping log lines. An existing output change is a compatibility change.
- New structured logs use stable keys for component, operation, outcome and duration, plus
  the applicable existing run/attempt/request correlation IDs. Preserve cause chains and
  redact before output, including nested attributes. Avoid credentials, arbitrary payloads
  and unbounded/high-cardinality metric labels. Do not invent new identity meanings here.
- Pass cancellation/deadlines through `context.Context`. Return inspectable errors; report
  them once at the command/service boundary. Libraries do not call `os.Exit` or configure a
  process-global logger. Cleanup runs on failure as well as success.
- [Library-first boundaries](library-first.md) still apply. Stdlib-only evaluators and
  credential isolation outrank convenience imports. Module dependencies are part of that
  boundary, including transitive dependencies.

## Dependency admission and migration

Before adding or upgrading a dependency/tool, follow the existing worker dependency-risk
procedure: record publisher/source, exact version and release date, permissions and
persistence in an issue; cite it from the PR. Preserve its minimum release-age and driver
blessing rule. Also check licence compatibility, maintenance/security status, Go/platform
support and transitive cost. A catalogue entry is not that version-specific assessment.
Adding a direct import of an indirect dependency still needs explicit manifest ownership.
This policy installs nothing and changes no credential or network permission.

Keep one implementation owner and align compatible versions across consuming modules.
Do not copy a package locally, vendor a private fork or introduce a second library for the
same capability without the exception record. Preserve truly different capability boundaries
(e.g. HTTP retry mechanics versus durable effect reconciliation).

For existing bespoke infrastructure: identify callers and behavioral tests, qualify the
selected replacement, move consumers, then remove the old mechanism. An active worker's
brief is checkpointed before its scope changes. Completed Evidence is preserved. The
catalogue is not permission for a repository-wide rewrite or for expanding an unrelated fix.
New dependency selection belongs in a brief's Context/design-fit or the issue-driven PR's
rationale, so the implementer does not make the choice afresh on every dispatch.

## Evidence and enforcement

Source inventory: Assay `40b45182b3c011cef823309c4e1c8d1aecbf6967`, all tracked `go.mod`
files, the shared CLI adapter/contract, and desk trace/retry owners. Upstream project
pages linked above were inspected on 11 October 2026. Installed versions and security
qualification must be refreshed at adoption; an indirect dependency is not proof of direct use.

The contributor entrypoint links this policy; both worker prompt kits require consulting the
project's catalogue and explaining exceptions. This is reviewed guidance, not a mechanical
ban on imports or a completed fleet rollout. Existing prompt/CLI/architecture checks continue
to enforce their current scopes. New binaries must be released and consumed before deployed
workers receive changed embedded prompts; already-running workers retain their old prompt.
