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

## Additional library choices

These decisions extend the catalogue for common capabilities. They select defaults where a
need exists; they do not install dependencies or authorize rewriting existing owners.

| Capability | Choice and rationale |
|---|---|
| Bounded in-memory caches | Select [golang-lru/v2](https://github.com/hashicorp/golang-lru) when needed. Prefer its typed API over building eviction machinery. Define size, freshness and invalidation; eviction must never lose authoritative claims, idempotency records or recovery state. No cache is added by this policy. |
| Bidirectional WebSockets | Select [coder/websocket](https://github.com/coder/websocket) when needed. Keep polling or SSE for simpler one-way updates. Require origin/auth checks, message limits, cancellation and backpressure; reconnect does not authorize replaying a mutation. Do not introduce both this and Gorilla for the same service. |
| Semantic-version validation/comparison | Select [golang.org/x/mod/semver](https://pkg.go.dev/golang.org/x/mod/semver) for new Go-style version contracts. It requires a `v` prefix and is not a general constraint-range solver. Validate before comparing; retain existing external-tool output parsers until compatibility is proven. A range-solving need requires a separate selection, not a homemade parser. |
| Prometheus scrape endpoints | Keep OpenTelemetry as the instrumentation API. Use its [Prometheus exporter](https://opentelemetry.io/docs/languages/go/exporters/#prometheus) when scraping is required; do not instrument each operation twice through a parallel direct-client stack. An existing direct Prometheus integration needs a coordinated migration, not blind replacement. |
| Non-HTTP backoff | Keep the selected HTTP retry mechanism and existing loopengine policy. [cenkalti/backoff](https://github.com/cenkalti/backoff) is a candidate only for a demonstrated non-HTTP scheduling gap. Its timer does not decide whether an operation is safe to repeat; unknown mutation outcomes still require reconciliation. |
| Deterministic concurrency/time tests | On Go 1.25+, evaluate [testing/synctest](https://pkg.go.dev/testing/synctest) before adding a clock/mock framework for in-process tests. It does not virtualize external I/O. Keep testing and go-cmp for test control and comparisons. |

Do not add zerolog, Gorilla mux, json-iterator, pkg/errors, go-multierror or an atomic wrapper
as competing defaults where slog, net/http/chi, encoding/json, errors.Is/As/Join and typed
sync/atomic satisfy the contract. Additional stack traces, performance or concurrency
semantics need concrete evidence, not package popularity. Testify is not selected as a
second assertion convention. Embedded stores are not selected as an alternative authority
to the planned PostgreSQL core.

## Go analysis and security tooling

Select three complementary checks. Tool versions are pinned and admitted at implementation;
these rows do not claim the checks are installed or running in CI.

| Check | Selected tool | What it covers |
|---|---|---|
| General correctness | [golangci-lint](https://golangci-lint.run/docs/linters/) | The errcheck, staticcheck, unused and govet baseline owned by [PR #2535](https://github.com/medici-finance/assay/pull/2535), which authors desk-tools/32. |
| Security-oriented source analysis | [gosec](https://github.com/securego/gosec), through golangci-lint's [gosec integration](https://golangci-lint.run/docs/linters/configuration/#gosec) | Suspicious source patterns such as unsafe subprocess/SQL construction, weak crypto and insecure transport. Triage findings; neither a warning nor a clean scan proves exploitability or its absence. Use the version bundled in the admitted golangci-lint release, not an independently floating second installation. |
| Known vulnerable code | [govulncheck](https://go.dev/doc/security/vuln/) | Go vulnerability-database checks with source call-graph reachability, including dependencies and stdlib/toolchain exposure. It complements gosec; it does not discover unknown application authorization or business-logic flaws. |

Keep #2535 open: it owns all-module discovery, correctness baseline measurement/remediation,
pinned acquisition and the exact human-applied CI hunk. This policy does not subsume its
brief or verification. The security implementation must checkpoint that owner's work and
extend the shared runner/config after its baseline, without silently expanding the active
brief. Until that implementation and its workflow land, gosec and govulncheck remain
selected, not deployed. The rollout must deliver:

- Discover every tracked `go.mod`, run from each module directory and report coverage per
  module; reuse desk-tools/32's discovery rather than maintain another list. Record Go
  version, build tags, GOOS/GOARCH and test inclusion. Cover supported build configurations;
  one Linux scan does not certify Windows-only code. Unscannable modules are failures or
  explicitly blocked work, never silently skipped passes.
- Pin tool versions and verify acquired artifacts. For gosec, record the bundled analyzer
  version/rules and measure the initial results. Fix findings or record narrowly scoped,
  reviewed exceptions with rule/path, reason, owner, linked work and review date. No blanket
  exclusions or security-policy weakening to get green; existing human gates still apply.
- Run govulncheck in text mode for the failing gate. Its [structured output modes](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
  can return zero despite findings: uploading JSON/SARIF is not enforcement. If structured
  output becomes the gate, first prove its result evaluator rejects reachable findings.
  Keep per-module failures through any loop or report pipeline; do not use `|| true` or
  `continue-on-error`. Record database source/freshness; database, package-loading and
  execution failures cannot count as clean. Updates to vulnerability data are expected,
  even while the tool itself is pinned.
- Run on PRs and periodically against main, since vulnerabilities can be disclosed without
  a code change. Explicitly configure database/dependency-network access; local offline
  scans need a prepared database and module cache, otherwise report could-not-check.
  This does not grant production access or credentials to untrusted PR code.
- Prove the checks discriminate: a controlled insecure-code fixture fails gosec and passes
  after repair; a separate vulnerable-dependency fixture fails the govulncheck gate and
  passes after upgrade; scan errors fail rather than disappear. Keep fixtures out of shipped
  modules. Record observed outcomes and prove the merged-main workflow actually runs both.
  Prepare exact workflow changes through the same human-applied runsheet route as #2535.

Existing secret/leak checks, tests and independent security review remain. CodeQL may add
interprocedural coverage later, but adding another platform is not required for this initial
Go baseline; no existing CodeQL control is removed by this choice.

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
Promoting an indirect dependency to direct and upgrading an existing dependency use the
same issue-based assessment; cite the prior assessment and record the changed version or
usage rather than treating graph presence as approval.
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
