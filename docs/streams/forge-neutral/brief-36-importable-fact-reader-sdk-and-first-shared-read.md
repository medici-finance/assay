---
brief: assay:assay:forge-neutral:36
title: Importable fact reader SDK and first shared read
why: "Consumers currently need a subprocess or duplicate forge handling to reuse read behavior. A narrow SDK lets commands and applications share the implementation without acquiring custody or write authority."
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-neutral-36
gate-why: "sensitive-data is yes because the slice moves the code that reads private issue data under a minted credential into a new module other processes can link. The human confirms three things: no credential moves into a previously credential-free process; the forge-CLI scan, the ambient-token rule and the CI path trigger follow the moved code (rows 5 to 8); and the lower boundary refuses an out-of-scope repository with the consumer-side check bypassed (row 9)."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-08 by statusgen newbrief
sources:
  - "#2395 — library-first API planning"
  - docs/library-first.md
  - "freshness-checked 2026-10-08 @ 403b8ec8c26b21b85d88b5faea4cff619550e602"
exec-tier: strong
exec-tier-why: Cross-module extraction must preserve identity constraints, partial-result semantics and both forge mappings.
domain: complicated
decision-trigger: start
consumers:
  - "forgeread/, tools/desk/go.mod, statusgen/go.mod: follow-up forge-neutral/36 (no root go.work: loopadmin/runner/request_test.go fails if one exists)"
  - "tools/desk/internal/deskkit/forge_github.go, tools/desk/internal/deskkit/forge_gitlab.go, tools/desk/cmd/deskread: follow-up forge-neutral/36"
  - "statusgen/forgeread.go: follow-up forge-neutral/36"
  - "remaining statusgen forge reads: follow-up forge-neutral/18"
  - "CI identity admission: follow-up forge-neutral/34"
  - "ruling/sign-off resolvers: follow-up forge-neutral/35"
  - "tools/desk/scripts/forge-ban.sh, tools/desk/internal/deskkit/ambienttoken_guard_test.go, .github/workflows/forge-surface-control.yml: follow-up forge-neutral/36 (this brief is the single owner of their coverage of forgeread/; desktools-v2/08 still owns the statusgen half)"
id: 41aca5ee-e627-4547-817d-932c899e766e
---

# Brief 36 — Importable fact reader SDK and first shared read

## Context

files: `forgeread/go.mod` (new), `forgeread/reader.go` (planned), `forgeread/offline.go` (planned),
`forgeread/envelope.go` (planned), `forgeread/adapters/` and tests (new),
`tools/desk/go.mod`, `tools/desk/internal/deskkit/forge_github.go`,
`tools/desk/internal/deskkit/forge_gitlab.go`, their tests,
`tools/desk/cmd/deskread/`, `tools/desk/scripts/forge-ban.sh`,
`tools/desk/internal/deskkit/ambienttoken_guard_test.go`, `statusgen/go.mod`, `statusgen/forgeread.go` and tests,
`statusgen/forgeread_sdk_test.go` (new), `tools/desk/internal/arch/`,
`.github/workflows/forge-surface-control.yml`, `.github/workflows/assay-statusgen.yml`,
`docs/library-first.md`, `docs/streams/forge-gitlab/inventory.md`,
`changelog/fact-reader-sdk.md` (new).

layout: the module's root package (`reader.go`, `offline.go`, `envelope.go`) is its offline
and frozen surface and the only package a credential-free consumer such as statusgen may link.
Every authenticated read adapter lives in a subpackage (`forgeread/adapters/`). Rows 10 and 11
hold that split where the link is made.

facts (403b8ec8c26b21b85d88b5faea4cff619550e602, 2026-10-08): statusgen has an offline
reader and a deskread process adapter; tools/desk's Forge interface and implementations are
internal. The SDK module does not exist. OpenIssues already has GitHub and GitLab behavior;
/33 has implemented additional operations, which this first slice does not reimplement.
/34 owns CI workflow-token admission and /35 owns the human-ruling control migrations.

layering: a narrow consumer read API plus separate effectful read adapters; credential
composition remains at the existing trusted boundary. Tasks 1–3 and 5 and Verify 2–11 enforce this.
design-fit:
  owner: forgeread (read implementation extracted from existing forge adapters)
  contract: "none — typed read transport; S-identity stays owned by existing custody"
  retires: [duplicated OpenIssues transport implementation, duplicated statusgen read envelope types]
  weight: "verbs 0, flags 0, refusals 0; one module and typed API"
  why-add: "An independent module avoids importing all of deskkit and replaces duplicated read code; exporting deskkit would expose unrelated capabilities."
single-point-of-failure: the credential composition inside desk-tools — `deskkit`'s resolver,
reached through `deskread`, is the one place a token meets a read adapter. Behind it: the
provider installation's own repository scope, which the forge enforces whatever the client
does, and rows 3, 6, 7, 9, 10 and 11 (no ambient fallback, scans over the moved code, a
lower-layer refusal with the upper check bypassed, and statusgen linking only the root package,
whose dependency closure reaches no network client). SDK types carry no caller authority, and a
read-only method set alone is not a sandbox for an overprivileged token.

Read first: `docs/library-first.md`, `docs/streams/desktools-v2/spec.md`,
`docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md` and /35.

## Human decision

The implementation shares read behavior between commands and library callers. It must not
move a role credential into a previously credential-free process or broaden private-data access.
The review decides whether the demonstrated package and runtime boundaries preserve that rule.

Options:
1. **Accept the bounded extraction** — reuse the typed reader while retaining existing credential
   placement; statusgen's online reads stay on the deskread process adapter.
2. **Hold the extraction** — retain current reads until the implementation supplies adequate
   evidence of equivalent scope and independent enforcement.

Default if no answer: none — rollout remains held.

## Ground rules

Offline fixtures/fake providers only. No production query, credential provisioning, release,
merge or activation. No generic query/URL/exec port. Do not reopen the frozen Forge operation set.
Effort L covers one complete OpenIssues slice, two existing backends and consumer packaging;
all remaining read kinds stay with their existing migration owners.

## Task

1. Create the narrow public Reader API for OpenIssues over an explicit repository set and
   context; reuse the existing three-state envelope and keep unknown schema fail-closed.
   Each requested repo appears exactly once as data or unavailable, including mixed failures,
   pagination truncation and known-empty success. Supply offline and frozen-result adapters.
   All of this is the module's root package (see `layout:`); it imports no subpackage of the
   module, no HTTP client and no process launch (row 11).
2. Move the existing OpenIssues read behavior for both providers into separate read-adapter
   packages under `forgeread/adapters/`, never into the root package. Existing deskkit methods delegate to it; delete duplicate implementations. Keep
   existing credential resolution, host/repo validation and activation at their owning boundary.
   Consumer API packages expose no raw transport/token/query or write methods and have no
   transitive custody/write dependency. Effectful adapters accept only explicitly supplied,
   already-admitted access at trusted composition; they never discover credentials themselves.
3. Wire deskread's issues kind and statusgen's OpenIssues seam to the shared API/types. Direct
   library access is used for offline and frozen inputs, and inside `deskread` itself, where
   `deskkit`'s resolver composes the credential (the trusted read composition). statusgen's
   online reads, role-session and CI alike, keep the deskread process adapter. Retiring that
   bridge for any online read is not part of this brief: it needs its own human-gated brief and
   a driver ruling, and none is authored. Removing it is never permission to copy credentials
   into statusgen. Exercise direct
   SDK and CLI production paths against the same fake-provider observations. Do not change /34's
   CI opt-in or /35's ruling rules. Add imports/flow checks with negative controls, including
   a client bypass that still cannot use another repository's credential scope at the lower
   boundary. If that property cannot be demonstrated, retain the isolated transport.
4. Pin dependencies, ensure desk-tools and statusgen release builds resolve the new module with
   GOWORK=off, and build an external consumer using a temporary local module proxy without relative
   replace or network. Version Go and JSON contracts separately; preserve command output/exit
   compatibility. Do not create a root `go.work`. Update docs and changelog.
5. Own the control coverage of the new module, landing with the extraction; moving code must
   not remove audit coverage. `forge-ban.sh` counts `forgeread/` as its own tree, reported
   separately, and exits non-zero when that count is above zero from the start (rows 5–6). The
   ambient-token-read rule covers `forgeread/` (row 7). The forge-surface-control workflow's
   `pull_request` and `push` path lists include `forgeread/**` (row 8). Add the lower-layer
   scope test in the desk-tools module, where the boundary lives (row 9).

## Verify

Named tests are implementation deliverables; use fake providers, no live credentials. Each
case retains a fail-first mutation and exercises a production path, not a mirror implementation.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKCLIParity " "$result")` | exit 0; named PASS; Both provider fixtures give equal SDK/deskread/statusgen results; one deliberate mapping change fails. |
| 2 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKUnavailable$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKUnavailable " "$result")` | exit 0; named PASS; Known-empty, partial, all-unavailable, truncated and unsupported-schema fixtures remain distinct; empty-success substitution fails. |
| 3 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKAuthorityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKAuthorityBoundary " "$result")` | exit 0; named PASS; No ambient fallback or transitive custody/write access from statusgen's side; allowed control succeeds. The lower-layer scope proof is row 9, in the module that holds the boundary. |
| 4 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKExternalConsumer " "$result")` | exit 0; named PASS; Release build and external consumer resolve with GOWORK=off and local proxy only; omitted module publication fails. |
| 5 | check +dereference | `test -d forgeread && { sh tools/desk/scripts/forge-ban.sh > "${TMPDIR:-/tmp}/b36-r5.out" 2>&1; rc=$?; grep -F 'forgeread sites: 0' "${TMPDIR:-/tmp}/b36-r5.out" && test "$rc" -eq 0; }` | exit 0: the counter reports the `forgeread/` tree separately, at `0`, and exits 0 on the clean tree. Fails if the tree is missing or not counted |
| 6 | check +mutation | `sh -c 'test -d forgeread \|\| exit 1; mkdir -p forgeread/zz_banprobe && printf "package zz\nimport \"os/exec\"\nvar _ = exec.CommandContext(nil, \"gh\", \"api\")\n" > forgeread/zz_banprobe/probe.go; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -rf forgeread/zz_banprobe; echo "rc=$rc"; test "$rc" -ne 0'` | exit 0; prints a non-zero `rc=`. **Negative control**: a forge-CLI launch planted under `forgeread/` makes the counter FAIL, and the probe directory is removed whatever the result |
| 7 | check:ci +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | exit 0; named PASS. `TestAmbientTokenGuardCoversForgeread` (planned) runs the ambient-token-read rule over `forgeread/` and requires zero findings there with no permit; on a temporary copy with a planted `os.Getenv("GH_TOKEN")` in an adapter it requires exactly one finding |
| 8 | check +dereference | `grep -c -F '"forgeread/**"' .github/workflows/forge-surface-control.yml` | prints `2`: both path lists trigger the forge-surface control on a `forgeread/` change. Measured at this brief's authoring: `0` |
| 9 | check:ci +flow +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestSharedReadLowerScope$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestSharedReadLowerScope " "$result")` | exit 0; named PASS. `TestSharedReadLowerScope` (planned) bypasses the consumer-side repository check and hands the shared adapter, as composed by `deskkit`, a repository outside the composed credential's scope: it is refused before any request reaches the fake provider (zero recorded requests), and an in-scope control repository reads. **+mutation**: removing the composition's scope check makes the out-of-scope request reach the fake provider and fails the test |
| 10 | check +dereference +mutation | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: no non-test, non-comment statusgen line quotes an import path below the module's root package, whatever the subpackage is called. It is forge-neutral/18 row 21, copied here so the ruled limit binds on the change that creates the link. **+mutation**: a planted statusgen import of `.../forgeread/adapters/github`, or of any other subpackage, counts `1` |
| 11 | check +dereference +mutation | `test -f forgeread/go.mod && (cd forgeread && GOWORK=off go list -deps . > "${TMPDIR:-/tmp}/b36-r11.out") && { grep -E '^(net/http\|os/exec)$\|(^\|/)forgeread/' "${TMPDIR:-/tmp}/b36-r11.out" \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: the root package's whole dependency closure carries no HTTP client, no process launch and no subpackage of the module, so an authenticated adapter cannot sit in, or be reached through, the one package statusgen may link. A missing module prints nothing and fails the row. **+mutation**: a root-package file importing `net/http`, or importing `forgeread/adapters/...`, counts `1` or more |

## Pre-mortem

Duplicate backend mapping drifts (row 1); partial reads become empty queues (row 2); an SDK
silently inherits ambient or excessive access (row 3); a workspace-only dependency ships
unbuildable consumers (row 4); the moved adapters fall outside the forge-CLI scan, the
ambient-token rule or the CI trigger (rows 5–8); a consumer-side bypass reaches another
repository through the shared adapter (row 9); statusgen links an authenticated adapter, under
any package name, or an adapter is placed in the root package it may link (rows 10 and 11). Package selection and independence remain
human review judgments.

## Evidence

<!-- Independent verifier records subject revision, results and retained negative controls. -->

## Review

Gate: human. Library-first does not authorize new credentials, reduced checks or live rollout.
