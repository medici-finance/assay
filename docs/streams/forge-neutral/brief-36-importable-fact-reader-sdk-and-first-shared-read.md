---
brief: assay:assay:forge-neutral:36
title: Importable fact reader SDK and first shared read
why: "Consumers currently need a subprocess or duplicate forge handling to reuse read behavior. A narrow SDK lets commands and applications share the implementation without acquiring custody or write authority."
wave: 1
depends: []
unblocks: ["forge-neutral/37", "forge-neutral/46", "forge-neutral/47"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-neutral-36
decision-issue: 3662
ruling: "https://github.com/medici-finance/assay/issues/3662#issuecomment-6561248130"
gate-why: "sensitive-data is yes because the slice moves the code that reads private issue data under a minted credential into a new module other processes can link. The human confirms three things: no credential moves into a previously credential-free process; the forge-CLI scan, the ambient-token rule and the CI path trigger follow the moved code (rows 5 to 8); and the lower boundary refuses an out-of-scope repository with the consumer-side check bypassed (row 9)."
issues: []
schema: brief-v2
version: 2
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

Default if no answer: none — blocks until answered.

**Superseded direction (note added 2026-10-25).** `DR-forge-neutral-36`, the design this brief
cites, is superseded by `DR-forge-library` (the driver's ruling of 2026-10-25 on #2395): one
shared forge library for reads and writes, linked by statusgen, holding the role-and-repo
credential. Option 1's clause that statusgen's online reads stay on the deskread process adapter,
and Task 3's matching clause, are superseded going forward by desktools-v2/77, which replaces
that adapter with the library. This brief's delivered module, Verify table and Evidence are
unchanged and stand as recorded; the `forgeread` root package stays dependency-free (row 11).

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
| 10 | check +dereference +mutation | `test -d forgeread && test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: no non-test, non-comment statusgen line quotes an import path below the module's root package, whatever the subpackage is called. It is forge-neutral/18 row 21, copied here so the ruled limit binds on the change that creates the link. **+mutation**: a planted statusgen import of `.../forgeread/adapters/github`, or of any other subpackage, counts `1` |
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKCLIParity " "$result")` | pass exit=0 sandbox=container-netns | sha256:71288a36da37 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKUnavailable$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKUnavailable " "$result")` | pass exit=0 sandbox=container-netns | sha256:e6fb56e373d9 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKAuthorityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKAuthorityBoundary " "$result")` | pass exit=0 sandbox=container-netns | sha256:56ef03eb7fa7 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKExternalConsumer " "$result")` | pass exit=0 sandbox=container-netns | sha256:eca1b3ab59bc | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -d forgeread && { sh tools/desk/scripts/forge-ban.sh > "${TMPDIR:-/tmp}/b36-r5.out" 2>&1; rc=$?; grep -F 'forgeread sites: 0' "${TMPDIR:-/tmp}/b36-r5.out" && test "$rc" -eq 0; }` | pass exit=0 sandbox=container-netns | sha256:42249b361d60 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 6 | `sh -c 'test -d forgeread \|\| exit 1; mkdir -p forgeread/zz_banprobe && printf "package zz\nimport \"os/exec\"\nvar _ = exec.CommandContext(nil, \"gh\", \"api\")\n" > forgeread/zz_banprobe/probe.go; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -rf forgeread/zz_banprobe; echo "rc=$rc"; test "$rc" -ne 0'` | pass exit=0 sandbox=container-netns | sha256:91d957f8f274 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | pass exit=0 sandbox=container-netns | sha256:7bbc0faaa7b1 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c -F '"forgeread/**"' .github/workflows/forge-surface-control.yml` | pass exit=0 sandbox=container-netns | sha256:53c234e5e847 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestSharedReadLowerScope$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestSharedReadLowerScope " "$result")` | pass exit=0 sandbox=container-netns | sha256:9f7359a63e43 | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |
| 11 | `test -f forgeread/go.mod && (cd forgeread && GOWORK=off go list -deps . > "${TMPDIR:-/tmp}/b36-r11.out") && { grep -E '^(net/http\|os/exec)$\|(^\|/)forgeread/' "${TMPDIR:-/tmp}/b36-r11.out" \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-20 | assay-verifier-app[bot] @ 331aba8f343c (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier run — 2026-10-20 claude-opus-5-5 verifier at 331aba8f343c (11 of 11 rows pass; gate:human sign-off pending)

Verification-Attestation: medici-finance/assay#3663 run=ae22e400a9eaecb1cd48d55c6aed6f5e8925906fb6e7615c2d7946409a5b8f4f source=331aba8f343c828763a1fae23c4cba26abff7560 brief=docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md model=claude-opus-5-5 tier=strong

Context: run by a fresh verifier, not the implementer, from a home worktree detached at merged main 331aba8f343c828763a1fae23c4cba26abff7560. Admission came first: `deskdispatch --check-verifier` exited 0 and bound run ae22e400a9ea; it was repeated after the rows. The witness table above was written by `statusgen verifyrun --sandbox container-netns` in a local docker container (image assay-verify:go1.27, built FROM golang:1.27.2-trixie, git 2.47, `--network none`, default seccomp profile), cloned from this home at its source commit; verifyrun exited 0 with 11 pass, 0 fail, 0 could-not-run. This runner follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Every row was also run by hand (go1.27.2 darwin/arm64, git 2.56.0) in a scratch clone at the same commit outside the home, with `KUBECONFIG=/dev/null` and a TMPDIR outside the home; a table-escaped pipe was run as a plain pipe (rows 6, 10 and 11). By hand rows 1 to 8, 10 and 11 met Expect as authored. Row 8 printed 2, and rows 10 and 11 each printed 0; the witness output hashes for those rows match those exact outputs (row 8 matches "2", rows 10 and 11 match "0"). Row 6 printed rc=1 and removed its probe directory. Row 5 reported "forgeread sites: 0" on its own line, separate from the statusgen count.

Prior Evidence: none. This brief's Evidence section held only its placeholder comment on origin/main, so this is the first Evidence pass. The work was delivered by 01e6a7f44 (medici-finance/assay#2536), and the workflow promotion that row 8 reads landed in 251e94739 (medici-finance/assay#2540). Since then, eight more commits touch the shared reader module, the statusgen seam, deskread, the counter or the workflow. They include medici-finance/assay#3329, medici-finance/assay#2944 and medici-finance/assay#2978. All 11 rows still hold at the current head.

Row 9, by-hand diagnosis (classification: environment-only, not a deliverable defect). In the first by-hand clone, origin pointed at a local path. There TestSharedReadLowerScope failed at sharedread_test.go:94, because ReadOnlyForgeForCIToken resolves the forge kind before it builds the backend: "cannot resolve which forge serves example-org/own: no ASSAY_REPO_FORGES entry names it, and the origin remote's host is absent ...". With origin set to the github.com URL, which matches the real checkout and the witness clone, the row exits 0 with the named PASS. The row asserts the lower-layer scope refusal, not anything about the environment, so this is a test hermeticity gap and not a defect in the boundary.

Fail-first mutations, applied only in the scratch clone and restored afterwards (git status clean after each):
- Row 9: disabling the `s.only` scope comparison in scopedAccess.GetPage (tools/desk/internal/deskkit/sharedread.go:45) makes the test FAIL. All three out-of-scope repositories return 1 issue, and "3 request(s) reached the provider for out-of-scope repositories, want 0". The lower layer catches the fault with the upper check bypassed, and no upper-layer check stands in for it.
- Row 10: a planted non-test statusgen file importing the github adapter subpackage counts 1.
- Row 11: a planted root-package file importing net/http counts 1, and that mutant was run in the rendered form, where each table-escaped pipe is a plain pipe. The lint notice [ere-literal-pipe] on this row reads the source-escaped text. Run as raw source, that text is a shell syntax error (exit 2), so the witness's exit 0 with output "0" comes from the rendered command, where the grep -E pattern is a real alternation.

Risk-bearing values (sensitive-data: yes). Enumeration covered the delivery diff 01e6a7f44 over the forgeread module and the deskkit, deskread and statusgen files it changed:
- RISK-VALUE: DERIVED — onlyRepo = jobRepo @ tools/desk/internal/deskkit/forgeresolve.go:834 (checked case-insensitively by scopedAccess.GetPage @ tools/desk/internal/deskkit/sharedread.go:45). The CI workflow token is admitted for the job's own repository only, and the constructor already refuses any other requested repository at forgeresolve.go:821. So the lower layer is bound to exactly that one coordinate, through separate code at the read boundary. A wider value would let a consumer-side bypass read another repository's private issues; the row 9 mutant shows the binding alone stops that.
- RISK-VALUE: DERIVED — ciTokenPrefix = "ghs_" @ tools/desk/internal/deskkit/forgeresolve.go:796. This value was not changed by the diff. It is the GitHub App installation-token prefix, so personal, OAuth and user-to-server tokens are refused before any request.
- RISK-VALUE: DERIVED — EnvelopeSchema = 1 @ forgeread/reader.go:35. It is the first wire version, and any other declared value is refused (ErrUnsupportedSchema @ forgeread/envelope.go:40) rather than parsed best-effort. Row 2 covers this as its unsupported-schema fixture.
- Reversible knobs rank last and need no derivation. They are carried unchanged from the deleted deskkit constants: GitHub PerPage = 100 and MaxPages = 100 @ forgeread/adapters/github/github.go:19 and :21 (previously forgeIssuePerPage = 100 and forgeMaxIssuePages = 100), and GitLab PerPage = 100 and MaxPages = 25 @ forgeread/adapters/gitlab/gitlab.go:20 and :23 (previously gitlabPerPage = 100 and gitlabMaxIssuePage = 25). A walk past MaxPages becomes TruncatedError, which is could-not-check and never an empty result. APIVersion = "1.0.0" @ forgeread/reader.go:31 is a version label.

Gate is human, with sensitive-data: yes and every other risk answer no. A model does not sign this item off. Every row passes on merged main, the scope, import-closure and counter mutants are each caught, and the human confirmations named in gate-why (no credential moves into a credential-free process; rows 5 to 8; row 9) remain the reviewer's to give.

**VERIFY: PASS** — 11 of 11 rows pass at 331aba8f343c in the container witness and by hand; first stamp, gate:human sign-off pending.

### Non-implementer verifier run — 2026-11-07 claude-opus-5-5 verifier at 5142af8c66f4 (re-run after the row 10 amendment)

Context: a fresh verifier, not the implementer, ran this pass from a home worktree detached at merged main 5142af8c66f4671d7cb3c575d5b65d5eadd6275b. Admission came first: `deskdispatch --check-verifier` exited 0 and bound run a308e12b10b2 (desk-tools v1.0.78, darwin-arm64), and it was repeated after the Evidence edit. The witness below is from `statusgen verifyrun --sandbox=container-netns --fail-first --base 323d7f080cb3` inside an admission bracket. The bracket was opened on the host with `--open-bracket`. The run used the pinned verify-runner image v1.0.78 (linux/arm64 manifest sha256:3cb0014a71fa, statusgen v1.0.78, go1.27.3, git 2.56.0, `--network none`). The bracket was then closed from this home with `--close-bracket`. `--bracket-digest` reports verdict=match, with one Tree-Digest, one Container-Image and one attestation line in the span. verifyrun exited 0 with 11 pass, 0 fail and 0 not run at head. At the base all 11 rows were red, so every row discriminates.

Container setup, disclosed because it departs from a bare mount of the home:
- The container root is a fresh clone of this home at its source commit, not the home itself. The home is a linked worktree whose git directory lives outside it, so git inside the container could not open it. Row 9 then fails for an environment reason: the forge kind for its test repository cannot be resolved from origin. The clone's origin is the same github.com URL the home uses. A tree comparison excluding .git showed no difference, and the tree digest inside the container equals the receipt's.
- The image's module cache lacks three modules that the tools/desk tests import, and the network is off. So the host Go module cache was mounted read-only at the container's module path, with GOPROXY=off.
- git's safe.directory is set for the mounted root through the GIT_CONFIG_COUNT/KEY/VALUE environment, because the container runs as uid 0 over a host-owned tree.
- Two preflight probes ran before the credited run: one deskkit test without the module cache, and row 9's test before the clone. They are setup diagnostics, not attempts, and none of their results is used here.

Prior Evidence: the 2026-10-20 pass above stands as written. Since then, commit 606bc41d amended row 10 to open with `test -d forgeread`. Without that guard the row printed 0 at a base where forgeread/ did not exist, so it could not tell the delivered state from the state before delivery. With the guard, row 10 is red at base 323d7f080cb3 (rc=1) and prints 0 at head. The earlier non-discrimination report (medici-finance/assay#5685) does not reproduce at this source.

Fail-first: base 323d7f080cb3 is the parent of the delivering squash commit 01e6a7f44 (medici-finance/assay#2536). The workflow trigger that row 8 reads landed in 251e94739 (medici-finance/assay#2540). For rows 1 to 11, verifyrun's Base column records red rc=1 at base and a pass at head. The output hashes match the Expect values: row 8's is the sha256 of "2\n", and rows 10 and 11 are the sha256 of "0\n".

Scope mutation (row 9), applied in the scratch clone only and restored afterwards (git status clean): switching off the `s.only` comparison in scopedAccess.GetPage (tools/desk/internal/deskkit/sharedread.go:45) makes TestSharedReadLowerScope fail in the same image. Each of the three out-of-scope repositories returns 1 issue, and the test reports "3 request(s) reached the provider for out-of-scope repositories, want 0". The lower layer is what catches the bypass.

Risk-bearing values (sensitive-data: yes), enumerated over the delivery diff and the current forgeread module, ranked by blast radius:
- RISK-VALUE: DERIVED — onlyRepo = jobRepo @ tools/desk/internal/deskkit/forgeresolve.go:838, compared case-insensitively at tools/desk/internal/deskkit/sharedread.go:45. The CI workflow token is valid for the job's own repository only, and the constructor already refuses any other requested repository at forgeresolve.go:825. The lower layer is therefore bound to that single coordinate. Any wider value would let a consumer that skips its own check read another repository's private issues, and the row 9 mutation shows this binding alone stops that.
- RISK-VALUE: DERIVED — ciTokenPrefix = "ghs_" @ tools/desk/internal/deskkit/forgeresolve.go:800, unchanged by the delivery. It is the GitHub App installation-token prefix, so personal and user-to-server tokens are refused before any request.
- RISK-VALUE: DERIVED — `[ "$FORGEREAD_TOTAL" -gt 0 ]` @ tools/desk/scripts/forge-ban.sh:260. Zero is the only safe bound for forge-CLI call sites in a module that reads private data under a minted credential. Row 5 shows the count at 0 passes, and row 6 shows one planted site fails.
- RISK-VALUE: DERIVED — EnvelopeSchema = 1 @ forgeread/reader.go:35. It is the only wire version. Any other declared value is refused with ErrUnsupportedSchema (forgeread/envelope.go:14 and :42) rather than parsed best-effort.
- Reversible knobs rank last and need no derivation. GitHub PerPage = 100 and MaxPages = 100 @ forgeread/adapters/github/github.go:19 and :21, and GitLab PerPage = 100 and MaxPages = 25 @ forgeread/adapters/gitlab/gitlab.go:20 and :23, are carried over from the deleted deskkit constants. APIVersion = "1.2.0" @ forgeread/reader.go:31 is a version label.

Gate: human. sensitive-data is yes, and every other risk answer is no. Every row passes and discriminates at merged main. A model does not sign this item off: the confirmations named in gate-why stay with the human gate (decision issue medici-finance/assay#3662).

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKCLIParity " "$result")` | pass exit=0 sandbox=container-netns | sha256:98c1d5f50491 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKUnavailable$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKUnavailable " "$result")` | pass exit=0 sandbox=container-netns | sha256:e6fb56e373d9 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKAuthorityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKAuthorityBoundary " "$result")` | pass exit=0 sandbox=container-netns | sha256:342bb21003c2 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderSDKExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestReaderSDKExternalConsumer " "$result")` | pass exit=0 sandbox=container-netns | sha256:7f2c46335fc3 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 5 | `test -d forgeread && { sh tools/desk/scripts/forge-ban.sh > "${TMPDIR:-/tmp}/b36-r5.out" 2>&1; rc=$?; grep -F 'forgeread sites: 0' "${TMPDIR:-/tmp}/b36-r5.out" && test "$rc" -eq 0; }` | pass exit=0 sandbox=container-netns | sha256:42249b361d60 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 6 | `sh -c 'test -d forgeread \|\| exit 1; mkdir -p forgeread/zz_banprobe && printf "package zz\nimport \"os/exec\"\nvar _ = exec.CommandContext(nil, \"gh\", \"api\")\n" > forgeread/zz_banprobe/probe.go; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -rf forgeread/zz_banprobe; echo "rc=$rc"; test "$rc" -ne 0'` | pass exit=0 sandbox=container-netns | sha256:91d957f8f274 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 7 | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | pass exit=0 sandbox=container-netns | sha256:a31778a7b3cc | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 8 | `grep -c -F '"forgeread/**"' .github/workflows/forge-surface-control.yml` | pass exit=0 sandbox=container-netns | sha256:53c234e5e847 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 9 | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && go test -count=1 -v -run "^TestSharedReadLowerScope$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestSharedReadLowerScope " "$result")` | pass exit=0 sandbox=container-netns | sha256:9f7359a63e43 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 10 | `test -d forgeread && test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
| 11 | `test -f forgeread/go.mod && (cd forgeread && GOWORK=off go list -deps . > "${TMPDIR:-/tmp}/b36-r11.out") && { grep -E '^(net/http\|os/exec)$\|(^\|/)forgeread/' "${TMPDIR:-/tmp}/b36-r11.out" \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=323d7f080cb3 red rc=1 · head=5142af8c66f4 |
Tree-Digest: sha256:a45db11d4bffc3f40f29c060e503274a3912aafbbb7aaded88ea20c181f1c3cf
Container-Image: sha256:3cb0014a71faf50db03e13189af8e2505a310473e41111c24c559e5dee2a519b
Verification-Attestation: medici-finance/assay#5700 run=a308e12b10b25d6c80e420920a1b545400e3d0231ac7ffeae48bc4eb9acb9a4a source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=a308e12b10b25d6c80e420920a1b545400e3d0231ac7ffeae48bc4eb9acb9a4a source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b tree=sha256:a45db11d4bffc3f40f29c060e503274a3912aafbbb7aaded88ea20c181f1c3cf witness=sha256:073c65e238e9e87bc413d1deca764a3043c564bc80b8c4f1482b5459bcb7dfb2 opened=2026-11-07T23:44:12Z closed=2026-11-07T23:49:38Z before=receipt after=check-verifier

#### Correction — 2026-11-08 rework

medici-finance/assay#3662 is the brief's pre-implementation design decision (the option-1 ruling recorded on 2026-11-01). It is closed, and it does not track the post-verification gate:human sign-off. That sign-off is tracked in the open issue medici-finance/assay#5721, which is the outcome record's blocker_ref.

**VERIFY: PASS** — 11 of 11 rows pass at 5142af8c66f4 and all 11 are red at base 323d7f080cb3 in the container-netns fail-first witness; gate:human sign-off remains with the human.

## Review

Gate: human. Library-first does not authorize new credentials, reduced checks or live rollout.

The [November 1 ruling on #3662](https://github.com/medici-finance/assay/issues/3662#issuecomment-6561248130)
selects option 1 for the delivered bounded extraction. The delivery is [#2536](https://github.com/medici-finance/assay/pull/2536),
with the workflow promotion in [#2540](https://github.com/medici-finance/assay/pull/2540).
The independent first stamp at `331aba8f343c` recorded 11 of 11 passing rows
in [#3670](https://github.com/medici-finance/assay/pull/3670). Its
[retained outcome](../verify-outcomes/forge-neutral/36-20261020T122723Z-5ef47573dd26.json)
recorded the human gate as pending at the time of that run. Verify-desk owns
assessment of that evidence for any lifecycle transition. The delivered design
history and the later `DR-forge-library` direction remain recorded above.
