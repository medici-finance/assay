---
brief: assay:assay:forge-neutral:36
title: Importable fact reader SDK and first shared read
why: "Consumers currently need a subprocess or duplicate forge handling to reuse read behavior. A narrow SDK lets commands and applications share the implementation without acquiring custody or write authority."
wave: 1
depends: []
unblocks: ["forge-neutral/18"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Consumers currently need a subprocess or duplicate forge handling to reuse read behavior. A narrow SDK lets commands and applications share the implementation without acquiring custody or write authority."
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
  - "forgeread/, go.work, tools/desk/go.mod, statusgen/go.mod: follow-up forge-neutral/36"
  - "tools/desk/internal/deskkit/forge_github.go, tools/desk/internal/deskkit/forge_gitlab.go, tools/desk/cmd/deskread: follow-up forge-neutral/36"
  - "statusgen/forgeread.go: follow-up forge-neutral/36"
  - "remaining statusgen forge reads: follow-up forge-neutral/18"
  - "CI identity admission: follow-up forge-neutral/34"
  - "ruling/sign-off resolvers: follow-up forge-neutral/35"
  - "tools/desk/scripts/forge-ban.sh: follow-up desktools-v2/08"
id: 41aca5ee-e627-4547-817d-932c899e766e
---

# Brief 36 — Importable fact reader SDK and first shared read

## Context

files: `forgeread/go.mod` (new), `forgeread/reader.go` (planned), `forgeread/offline.go` (planned),
`forgeread/envelope.go` (planned), `forgeread/adapters/` and tests (new), `go.work`,
`tools/desk/go.mod`, `tools/desk/internal/deskkit/forge_github.go`,
`tools/desk/internal/deskkit/forge_gitlab.go`, their tests,
`tools/desk/cmd/deskread/`, `statusgen/go.mod`, `statusgen/forgeread.go` and tests,
`statusgen/forgeread_sdk_test.go` (new), `tools/desk/internal/arch/`,
`.github/workflows/forge-surface-control.yml`, `.github/workflows/assay-statusgen.yml`,
`docs/library-first.md`, `docs/streams/forge-gitlab/inventory.md`,
`changelog/fact-reader-sdk.md` (new).

facts (403b8ec8c26b21b85d88b5faea4cff619550e602, 2026-10-08): statusgen has an offline
reader and a deskread process adapter; tools/desk's Forge interface and implementations are
internal. The SDK module does not exist. OpenIssues already has GitHub and GitLab behavior;
/33 has implemented additional operations, which this first slice does not reimplement.
/34 owns CI workflow-token admission and /35 owns the human-ruling control migrations.

layering: a narrow consumer read API plus separate effectful read adapters; credential
composition remains at the existing trusted boundary. Task 1–3 and Verify 2–4 enforce this.
design-fit:
  owner: forgeread (read implementation extracted from existing forge adapters)
  contract: "none — typed read transport; S-identity stays owned by existing custody"
  retires: [duplicated OpenIssues transport implementation, duplicated statusgen read envelope types]
  weight: "verbs 0, flags 0, refusals 0; one module and typed API"
  why-add: "An independent module avoids importing all of deskkit and replaces duplicated read code; exporting deskkit would expose unrelated capabilities."
single-point-of-failure: SDK types do not contain caller authority. Existing provider scopes
and the credential-holding boundary remain independent of the consumer; a read-only method
set alone is not a sandbox for an overprivileged token.

Read first: `docs/library-first.md`, `docs/streams/desktools-v2/spec.md`,
`docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md` and /35.

## Human decision

The implementation shares read behavior between commands and library callers. It must not
move a role credential into a previously credential-free process or broaden private-data access.
The review decides whether the demonstrated package and runtime boundaries preserve that rule.

Options:
1. **Accept the bounded extraction** — reuse the typed reader while retaining existing credential
   placement and compatibility transport wherever direct access is not authorized.
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
2. Move the existing OpenIssues read behavior for both providers into separate read-adapter
   packages. Existing deskkit methods delegate to it; delete duplicate implementations. Keep
   existing credential resolution, host/repo validation and activation at their owning boundary.
   Consumer API packages expose no raw transport/token/query or write methods and have no
   transitive custody/write dependency. Effectful adapters accept only explicitly supplied,
   already-admitted access at trusted composition; they never discover credentials themselves.
3. Wire deskread's issues kind and statusgen's OpenIssues seam to the shared API/types. Direct
   library access is used for offline/frozen inputs and at trusted read compositions. Keep the
   existing deskread bridge as a named compatibility adapter for role-session online reads:
   removing that bridge is not permission to copy credentials into statusgen. Exercise direct
   SDK and CLI production paths against the same fake-provider observations. Do not change /34's
   CI opt-in or /35's ruling rules. Add imports/flow checks with negative controls, including
   a client bypass that still cannot use another repository's credential scope at the lower
   boundary. If that property cannot be demonstrated, retain the isolated transport.
4. Pin dependencies, ensure desk-tools and statusgen release builds resolve the new module with
   GOWORK=off, and build an external consumer using a temporary local module proxy without relative
   replace or network. Version Go and JSON contracts separately; preserve command output/exit
   compatibility. Extend existing CI scans/path triggers to the new module, including the forge
   launch scan; moving code must not remove audit coverage. Update docs and changelog.

## Verify

Named tests are implementation deliverables; use fake providers, no live credentials. Each
case retains a fail-first mutation and exercises a production path, not a mirror implementation.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKCLIParity " "$result")` | exit 0; named PASS; Both provider fixtures give equal SDK/deskread/statusgen results; one deliberate mapping change fails. |
| 2 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKUnavailable$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKUnavailable " "$result")` | exit 0; named PASS; Known-empty, partial, all-unavailable, truncated and unsupported-schema fixtures remain distinct; empty-success substitution fails. |
| 3 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKAuthorityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKAuthorityBoundary " "$result")` | exit 0; named PASS; No ambient fallback or transitive custody/write access; upper-check bypass cannot broaden lower host/repo scope; allowed control succeeds. |
| 4 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKExternalConsumer " "$result")` | exit 0; named PASS; Release build and external consumer resolve with GOWORK=off and local proxy only; omitted module publication fails. |

## Pre-mortem

Duplicate backend mapping drifts (row 1); partial reads become empty queues (row 2); an SDK
silently inherits ambient or excessive access (row 3); a workspace-only dependency ships
unbuildable consumers (row 4). Package selection and independence remain human review judgments.

## Evidence

<!-- Independent verifier records subject revision, results and retained negative controls. -->

## Review

Gate: human. Library-first does not authorize new credentials, reduced checks or live rollout.
