---
brief: assay:assay:cell-api:08
title: Build the deskd binary and image from public source through the Makefile
why: "The deskd source is now a standalone public module, but nothing builds it into a reviewable artifact. A local, unprivileged, digest-pinned binary and image build through the root Makefile lets the service package be inspected before any profile, pin or release depends on it."
wave: 0
depends: []
unblocks: ["cell-api/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "All four risk answers are no. The deliverable builds a binary and a local image from already-public source; it pushes, tags, publishes, deploys and provisions nothing, and handles no runtime data or credentials. Adding deskd to the release image set is cell-api/05's, publication remains the release workflow's separately approved act, and every deployment, profile and exposure decision stays behind cell-api/05's human gate."
issues: []
schema: brief-v2
version: 1
id: 1d91db5a-9be2-4caa-bc02-78b143fb3cd0
authored: 2026-10-27 by the-desk
exec-tier: strong
exec-tier-why: Supply-chain pins, credential exclusion and a cross-platform target-parity rule must hold together, and a subtle Dockerfile or recipe error (an unpinned base, a stray push, a copied credential) survives a passing build unless the static contract test and its mutation controls are designed independently.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/streams/cell-api/brief-05-package-the-deskd-image-and-generic-kubernetes-service-profile.md
  - deskd/README.md
  - docs/go-engineering.md
  - docs/release-images.md
  - containers/secrets.md
  - tools/winparity/README.md
  - https://github.com/medici-finance/assay/pull/3957
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-10-27 @ dda6c8c67d3bbdb185c16428609e6e11678ca63a
consumers:
  - "images/deskd/Dockerfile: follow-up cell-api/08"
  - "images/deskd/Dockerfile.dockerignore: follow-up cell-api/08"
  - "Makefile: follow-up cell-api/08"
  - "scripts/build-windows.ps1: follow-up cell-api/08"
  - "deskd/.gitignore: follow-up cell-api/08"
  - "deskd/README.md: follow-up cell-api/08"
  - "tools/desk/internal/cellapi/deskdimage_test.go: follow-up cell-api/08"
  - "tools/desk/internal/cellapi/testdata/deskdimage/: follow-up cell-api/08"
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up cell-api/08"
  - "changelog/deskd-image-build.md: follow-up cell-api/08"
  - "image artifact pin and supported service/client matrix: follow-up cell-api/05"
  - "release image set (tools/release/images.py, its contract and docs/release-images.md): follow-up cell-api/05 (adds deskd to the release producer; publishing stays the release's separately approved act; this brief adds no image to the release set)"
---

# Brief 08 — Build the deskd binary and image from public source through the Makefile

## Context

files: `images/deskd/Dockerfile` (planned), `images/deskd/Dockerfile.dockerignore` (planned), `Makefile`, `scripts/build-windows.ps1`, `deskd/.gitignore` (planned), `deskd/README.md`, `tools/desk/internal/cellapi/deskdimage_test.go` (planned), `tools/desk/internal/cellapi/testdata/deskdimage/` (planned), `tools/desk/internal/deskkit/citrigger_test.go`, `changelog/deskd-image-build.md` (planned)

facts:
- pin: public main dda6c8c67, inspected 2026-10-27. `deskd/` is a standalone Apache-2.0 Go module (`github.com/medici-finance/assay/deskd`, `go 1.27.0`), landed by [PR #3957](https://github.com/medici-finance/assay/pull/3957). From `deskd/`, `GOWORK=off go build ./cmd/deskd` builds the service; a CGO-off `linux/amd64` build and a `windows/amd64` build of `./cmd/deskd` both compiled at this pin. `images/deskd/` does not exist.
- inputs: `deskd/go.mod` requires the public `tools/desk` module by pseudo-version plus public upstream modules, all checksummed in `deskd/go.sum`; no private module is required. The SQLite driver is pure Go, so the binary is static with CGO off. Its web assets are embedded (`deskd/web/web.go`).
- runtime: the terminal backends resolve executables (`docker`, local shells) from PATH at run time. The service image ships none of them; lifecycle routing through the shared runner is cell-api/06's, not this image's.
- conventions: the repo already pins `golang:1.27.3-bookworm@sha256:972b4110…` (`images/verify-runner/Dockerfile`, `images/assay-harness/Dockerfile`) and a digest-pinned `# syntax=` frontend (`containers/base/Dockerfile`). `containers/scripts/layer-secret-scan.sh <image-ref>` is the existing per-image credential scan; `containers/secrets.md` states the no-secret-in-a-layer rule.
- parity: the root Makefile's `.PHONY` line and the `MAKEFILE-PARITY TARGETS` block in `scripts/build-windows.ps1` must be equal sets, and the script must stay ASCII-only with no `>>>`. `tools/winparity` enforces both and reddens on drift.
- registry: a `tools/desk` test that reads files outside the `tools/desk` module (here `Makefile`, `scripts/build-windows.ps1`, `images/deskd/`) is registered in `ciCrossModuleRegistry` (`tools/desk/internal/deskkit/citrigger_test.go`); `TestCrossModuleReaderRegistryIsNotSilentlyStale` flags an unclassified reader.
- package: the contract test lives in `tools/desk/internal/cellapi/` (planned), the package cell-api/05 also plans for its service tests. Whichever brief lands first creates it; neither owns the other's files in it.

layering: This brief carves the image-build duty out of cell-api/05 Task 1 so it can proceed on the already-public source while 05's profile, database and qualification dependencies stay open. It packages the existing deskd closure unchanged. It adds no runtime behaviour, configuration or authority, and it neither pins nor publishes an artifact.

single-point-of-failure: the static contract test (Verify 1) is the control that keeps an unpinned base, a credential or out-of-tree COPY, a widened default bind or a push out of the build definition. The built-image check (Verify 5) runs the existing layer-secret scan and inspects the actual image's user and contents on a different signal, A local image is never a pin: only the release job, once cell-api/05 adds deskd to its image set, produces a pinnable registry reference, under its own approval, pre-push checks and layer-secret gate.

design-fit:
  owner: R7 service host packaging (cell-api); the root Makefile owns local build entrypoints and `tools/winparity` owns their Windows parity
  contract: none — packaging only; the service, wire and authorization contracts stay with cell-api/01–03 and are not changed
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0 (two Makefile targets mirrored in the Windows script; no new CLI verb, flag or runtime refusal)
  why-add: n/a

All planned paths and the named test are implementation deliverables. The source pin
proves the starting point, not that these deliverables exist or were tested.

## Read first

`deskd/README.md`, the [stream README](README.md) (ownership, refactor map and code,
configuration and credential boundaries), [brief 05](brief-05-package-the-deskd-image-and-generic-kubernetes-service-profile.md)
Task 1, `containers/secrets.md`, `docs/release-images.md`, `tools/winparity/README.md`,
`docs/go-engineering.md`, and the existing images (`images/verify-runner/Dockerfile`,
`images/assay-harness/Dockerfile`, `containers/base/Dockerfile`) for pin, user and
comment conventions. `tools/desk/internal/deskkit/basepins_test.go` and
`tools/desk/cmd/deskpushguard/hookentrypoints_test.go` show the existing static
Dockerfile and Makefile-recipe test patterns.

## Ground rules

- Work in an owned branch; no merge, ready flip, image push, registry login, release tag, installation or cluster contact. Running `make deskd-image` locally is permitted; publishing its result is not.
- Public source and fixtures must be self-contained and synthetic. Never copy private build history, locators, configuration, host paths or credentials into the tree or the image.
- Package the existing deskd closure as it is. Do not change service behaviour, flags, defaults or dependencies here; a needed source change is NEEDS_CONTEXT for its owning brief.
- Stop at implemented; independent verification remains distinct.

## Task

1. **Dockerfile.** Create `images/deskd/Dockerfile`, multi-stage, with a header comment
   in the style of the existing images stating what is inside, what is deliberately
   absent, and the pins.
   - Pin the `# syntax=` frontend and every pulled `FROM` by tag and `@sha256:` digest,
     reusing the repo's existing Go builder reference rather than admitting a new one.
     The final stage is `FROM scratch`, the only permitted unpinned `FROM`, because it
     pulls nothing.
   - The build context is `deskd/` alone, so nothing outside the public module can be
     copied. `images/deskd/Dockerfile.dockerignore` excludes `dist/` and other local
     build output from that context.
   - The build stage runs with `GOWORK=off`, `CGO_ENABLED=0`, `GOTOOLCHAIN=local` and
     `GOFLAGS=-mod=readonly`, downloads modules checked against `deskd/go.sum`, and
     builds only `./cmd/deskd` with `-trimpath`. BuildKit cache mounts are allowed;
     secret and SSH mounts are not.
   - Leave module checksum verification on. No stage sets `GOSUMDB=off`, `GONOSUMDB`,
     `GONOSUMCHECK`, `GOPRIVATE` or `GOINSECURE`, and no `GOFLAGS` value or build flag
     carries `-mod=mod` or `-insecure`.
   - The final stage carries only the static binary, the CA trust bundle from the
     pinned build stage, and passwd/group entries for one numeric non-root user. It ends
     with `USER <uid>:<gid>` with a non-zero uid and gid. It has no shell, package
     manager, container CLI or terminal backend executable.
   - `ENTRYPOINT` is the binary. Bake no `CMD` or flag that names a config, index,
     token or host path, or that widens deskd's default loopback bind. Runtime
     configuration is cell-api/05's profile.
   - Set OCI labels for the source (`https://github.com/medici-finance/assay`), the
     revision (from a build argument) and the licence (`Apache-2.0`).
   - Do not COPY or ADD a credential-shaped path, a URL or anything into the final
     stage other than the items above. Set no ENV or ARG whose name or default carries
     a token, secret, password or key.

2. **Makefile targets.** Add both targets to the root `Makefile`'s `.PHONY` line and
   document them in its header block, where `desk-build` and `desk-install` are
   documented:
   - `deskd-build`: an unprivileged local build. It runs
     `cd deskd && GOWORK=off go build -trimpath -o dist/deskd ./cmd/deskd`, writes
     nothing outside `deskd/dist/`, and is safe for agents and CI. Add
     `deskd/.gitignore` ignoring `/dist/`.
   - `deskd-image`: a local image build only. It runs `docker build -f
     images/deskd/Dockerfile` with the context `deskd`. It tags the image
     `assay-deskd:<full source SHA>`, a local name with no registry host, and adds a
     `-dirty` suffix when `git status --porcelain -- deskd images/deskd` is non-empty.
     It passes the same SHA into the revision label.
   - The tag is a local build name, never an artifact pin. The build is not specified as
     reproducible, so two builds of one SHA need not share an image ID, and an image that
     was never pushed has no registry digest. cell-api/05 pins only the reference a
     published release records.
   - After the build, `deskd-image` runs `containers/scripts/layer-secret-scan.sh` on
     the built image. A finding or a could-not-check result fails the target.
   - If `docker` is absent or its daemon is unreachable, `deskd-image` prints a named
     could-not-check reason and exits non-zero. It never reports success without an
     image.
   - The recipe contains no `push`, `--push`, `--output type=registry`, `login`, `tag`
     to a registry host, or `buildx` exporter that leaves the local image store.
     Building for the builder's default platform is enough. The published platform
     matrix is cell-api/05's declaration and the release's act.

3. **Windows parity.** Add `deskd-build` and `deskd-image` to the
   `MAKEFILE-PARITY TARGETS` block and the dispatch table in `scripts/build-windows.ps1`.
   - `deskd-build` builds `deskd\dist\deskd.exe` the same way, unprivileged.
   - `deskd-image` either performs the equivalent local build or refuses with a named,
     non-zero "not supported on Windows" exit. It is never a silent no-op success.
   - Keep the script ASCII-only and free of `>>>`, and run `tools/winparity`.

4. **Static contract test.** Deliver `TestDeskdImageBuildContract` (planned) in
   `tools/desk/internal/cellapi/deskdimage_test.go` (planned). It parses the real
   `images/deskd/Dockerfile`, its ignore file and the real `Makefile`, and holds every
   property in Tasks 1–2:
   - every pulled `FROM` and the frontend are digest-pinned;
   - the final stage's last `USER` is numeric and non-root;
   - the build runs with `GOWORK=off`, CGO off and readonly modules, and builds only
     `./cmd/deskd`;
   - no checksum-disabling setting (`GOSUMDB=off`, `GONOSUMDB`, `GONOSUMCHECK`,
     `GOPRIVATE`, `GOINSECURE`, `-mod=mod`, `-insecure`) appears in any stage;
   - nothing credential-shaped, URL-sourced or out-of-context is copied, and the final
     stage has no extra COPY;
   - no secret-named ENV or ARG, and no baked config path or non-loopback bind;
   - both targets are on `.PHONY`, `deskd-image` uses the `deskd` context and runs the
     layer-secret scan, and `deskd-image` contains no push, login or registry exporter;
   - `deskd-build` writes only under `deskd/dist/`.

   Run the same checker over one synthetic negative fixture per property under
   `testdata/deskdimage/` (each fixture plants exactly one violation, such as an
   unpinned `FROM`, `USER root`, a missing `GOWORK=off`, `ENV GOSUMDB=off`,
   `ENV GOFLAGS=-mod=mod`, `COPY id_rsa`, `ENV GH_TOKEN=`,
   `--addr 0.0.0.0`, `docker push`, `--push` or a repo-root context). Every fixture must
   be rejected for its named reason. Zero fixtures read or a passing negative fixture
   fails the test. Register the test in `ciCrossModuleRegistry` with the paths it reads.

5. **Documentation.** Update `deskd/README.md`'s build paragraph to name `make
   deskd-build` and `make deskd-image`. State that the image is local-only until a
   separate release act publishes it. Add `changelog/deskd-image-build.md` (planned).

## Interface and acceptance contract

The image packages the existing deskd closure and runtime contract unchanged. Its
consumers are cell-api/05, which adds deskd to the release image set and declares the
artifact pin (a published release record's reference), the supported service/client
matrix and the deployment profile, and the release act that publishes it. This brief
pushes, tags remotely, publishes and deploys nothing, and adds no entry to
`release-images.json` or the release workflow. Explicitly out of scope, all owned by
cell-api/05 or later briefs:

- adding the image to the release image set, and publishing, pushing or release-tagging it;
- the artifact pin and supported service/client matrix;
- the Kubernetes service profile and network policy;
- the service database role and grants;
- health and restart qualification;
- the `deskd-deploy` target;
- real-profile qualification.

## Verify

The named test must be delivered and run, with a recorded failing control before the
restored pass. Zero selected tests or a skipped fixture is failure. Row 5 needs a
working container builder. Without one it is could-not-check, never pass, and the
offline rows do not substitute for it.

For row 7, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdImageBuildContract$' ./internal/cellapi/ > "$o" && grep -F -- '--- PASS: TestDeskdImageBuildContract ' "$o")` | exit 0; named PASS over the real `images/deskd/Dockerfile`, its ignore file and `Makefile`. Each property from Tasks 1–2 has a negative fixture that the same checker rejects by name: unpinned `FROM` or frontend, root or non-numeric final `USER`, missing `GOWORK=off`/CGO-off/readonly modules, each checksum-disabling setting (`GOSUMDB=off`, `GONOSUMDB`, `GONOSUMCHECK`, `GOPRIVATE`, `GOINSECURE`, `-mod=mod`, `-insecure`), credential-shaped or out-of-context COPY, extra final-stage COPY, secret-named ENV/ARG, baked config path or non-loopback bind, missing target, repo-root context, missing layer-secret scan, and `push`/`--push`/`login`/registry exporter in `deskd-image`. Zero fixtures read fails. |
| 2 | check:ci +neighbour | `(cd tools/winparity && GOWORK=off go run . --root ../..)` | exit 0; prints `winparity: OK` for target parity, and the target list includes `deskd-build` and `deskd-image`. Also prints the 5.1-clean line. A target on one side only, or a non-ASCII or `>>>` script, fails. |
| 3 | check:ci +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 120s -count=1 -v -run '^(TestCrossModuleTestsAreTriggeredByWhatTheyRead\|TestCrossModuleReaderRegistryIsNotSilentlyStale)$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCrossModuleTestsAreTriggeredByWhatTheyRead ' "$o" && grep -F -- '--- PASS: TestCrossModuleReaderRegistryIsNotSilentlyStale ' "$o")` | exit 0; both named PASS. The new test is registered with every path it reads, and the CI job that runs `tools/desk` reaches it on pull_request and push. |
| 4 | check +flow | `make deskd-build && test -x deskd/dist/deskd && deskd/dist/deskd -h 2>&1 \| grep -F -- '-human-login'` | exit 0; the unprivileged target builds the real service binary from `deskd/` with `GOWORK=off`, and its usage lists the existing `-human-login` flag. Nothing is written outside `deskd/dist/`, and `git status --porcelain` shows no tracked change. |
| 5 | check +flow +mutation | `command -v docker >/dev/null && docker info >/dev/null 2>&1 && make deskd-image && img="assay-deskd:$(git rev-parse HEAD)" && u=$(docker image inspect --format '{{.Config.User}}' "$img") && printf '%s\n' "$u" \| grep -Eqx '[1-9][0-9]*:[1-9][0-9]*' && test "$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$img")" = "$(git rev-parse HEAD)" && t=$(mktemp) && { docker run --rm --network none --entrypoint /bin/sh "$img" -c true > "$t" 2>&1; test $? -eq 127; } && grep -F -- '/bin/sh' "$t" && docker run --rm --network none "$img" -h 2>&1 \| grep -F -- '-human-login' && sh containers/scripts/layer-secret-scan.sh "$img"` | exit 0 on a clean checkout with a working builder. The local image exists under the source SHA, with no registry host. Its configured user is `<uid>:<gid>`, both numeric and non-zero, and its revision label equals the SHA. No `/bin/sh` exists in it: running it as the entrypoint exits 127 with an error naming `/bin/sh`, and any other failure (a missing image, a daemon error) fails the row instead of passing it. The binary runs offline and prints its usage. The layer-secret scan is clean. A missing builder or daemon, a scan could-not-check, or a missing image is non-zero and recorded as could-not-check, never pass. The recorded control plants `USER root` or a credential file in a scratch copy of the Dockerfile and shows the inspection or scan fail. |
| 6 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |
| 7 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/08 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent. Record exact corroborated, disproved and unchecked counts, with 0 disproved. Unchecked consumers stay held, and the two non-path entries (the cell-api/05 pin and cell-api/05's release-set addition) are review obligations. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| A re-pushed upstream tag changes what is built (unpinned base or frontend) | row 1 |
| A credential, private file or host path reaches a layer, ENV, ARG or build history | rows 1 and 5 (static COPY/ENV rules, then the existing layer-secret scan on the actual image) |
| The image runs as root or ships a shell or terminal backend that widens what a compromised service can do | rows 1 and 5 |
| The image bakes a non-loopback bind or config path that turns a local build into an exposure | row 1; exposure stays behind cell-api/05's human gate |
| `deskd-image` pushes or exports to a registry, turning a local build into an unapproved publication | row 1; publication is also gated by the release workflow's own approval |
| A local or `-dirty` build is pinned or deployed as if it were a published artifact | No row here: this brief produces no pin. cell-api/05's `deskd-deploy` accepts only a published release record's reference (its row 8) |
| The Windows script drifts from the Makefile, or the new test is not run by CI | rows 2 and 3 |
| An absent builder reads as a passing image build | row 5 is could-not-check without a builder, never pass |
| A Go module substituted on the proxy | row 1: `GOFLAGS=-mod=readonly` with `deskd/go.sum` and the default checksum database refuse it in the build stage, and a negative fixture for each checksum-disabling setting proves the Dockerfile cannot turn either off |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation result is asserted. -->
### Independent verification — 2026-10-29

Verifier: claude-opus-5-5, tier strong, non-implementer. Subject: merged main 5729430698d44ffd874bb25275d056f5a5404dc6. Delivery: PR #4755, merge 48ca54f64ffdd05956741619418a98d61eaf6a54, first parent 8f2d6865c6b290efd874fc30a21023aa817b6368. Tool: statusgen v1.0.65. Container rows (1, 2, 3, 7) ran network-off in verify-runner v1.0.65 (linux/amd64) inside a closed admission bracket; --bracket-digest verdict=match. Host rows (4, 5, 6) ran on a darwin arm64 host with docker 29.8.2 and buildx v0.38.0, through statusgen verifyrun --rows 4,5,6 --dry-run after the bracket closed.

Binding (check-verifier rc 0 before the rows and again before this write):

Verification-Attestation: medici-finance/assay#4813 run=95786f6ec14b2cc23f0870b7ae6c2dee0594b989ed3d8c8446a45fba7da185ca source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/cell-api/brief-08-build-the-deskd-binary-and-image-from-public-source-through-the-makefile.md model=claude-opus-5-5 tier=strong

Container witness (closed bracket, bytes as emitted):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestDeskdImageBuildContract$' ./internal/cellapi/ > "$o" && grep -F -- '--- PASS: TestDeskdImageBuildContract ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e6e250b2b8ef | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/winparity && GOWORK=off go run . --root ../..)` | pass exit=0 sandbox=container-netns | sha256:7c18b0f9767a | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 120s -count=1 -v -run '^(TestCrossModuleTestsAreTriggeredByWhatTheyRead\|TestCrossModuleReaderRegistryIsNotSilentlyStale)$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCrossModuleTestsAreTriggeredByWhatTheyRead ' "$o" && grep -F -- '--- PASS: TestCrossModuleReaderRegistryIsNotSilentlyStale ' "$o")` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/08 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | pass exit=0 sandbox=container-netns | sha256:c7063fb6e4f3 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4813 run=95786f6ec14b2cc23f0870b7ae6c2dee0594b989ed3d8c8446a45fba7da185ca source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/cell-api/brief-08-build-the-deskd-binary-and-image-from-public-source-through-the-makefile.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=95786f6ec14b2cc23f0870b7ae6c2dee0594b989ed3d8c8446a45fba7da185ca source=5729430698d44ffd874bb25275d056f5a5404dc6 tree=sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f witness=sha256:5df6883123fffae6906049205e9935af88dacedf51f046583f47398d7c3d34d0 opened=2026-10-29T17:03:49Z closed=2026-10-29T17:07:50Z before=receipt after=check-verifier

Host witness (verifyrun --rows 4,5,6 --dry-run, bytes as emitted):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `make deskd-build && test -x deskd/dist/deskd && deskd/dist/deskd -h 2>&1 \| grep -F -- '-human-login'` | pass exit=0 | sha256:0137dfc409fd | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `command -v docker >/dev/null && docker info >/dev/null 2>&1 && make deskd-image && img="assay-deskd:$(git rev-parse HEAD)" && u=$(docker image inspect --format '{{.Config.User}}' "$img") && printf '%s\n' "$u" \| grep -Eqx '[1-9][0-9]*:[1-9][0-9]*' && test "$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$img")" = "$(git rev-parse HEAD)" && t=$(mktemp) && { docker run --rm --network none --entrypoint /bin/sh "$img" -c true > "$t" 2>&1; test $? -eq 127; } && grep -F -- '/bin/sh' "$t" && docker run --rm --network none "$img" -h 2>&1 \| grep -F -- '-human-login' && sh containers/scripts/layer-secret-scan.sh "$img"` | pass exit=0 | sha256:9edb88839924 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --lint` | fail exit=1 | sha256:80e3d15bb235 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |

Per-row accounting (observations only; the witness tables above are the execution record):

- Row 1, pass: --- PASS: TestDeskdImageBuildContract over the real Dockerfile, ignore file and Makefile; 80 fixture subtests PASS, 0 FAIL. Control: USER root planted in a scratch copy failed with final-user-root; an unpinned build-stage FROM failed with unpinned-from; the unmodified copy passed.
- Row 2, pass: winparity: OK — 11 targets in parity, list includes deskd-build and deskd-image; the PowerShell 5.1-clean line printed.
- Row 3, fail: Both named tests print --- SKIP (citrigger_test.go:143 and :2536: fixture .github/workflows/tools.yml not present in this tree). go test exits 0 and the named-PASS grep exits 1. The brief counts a skipped fixture as failure.
- Row 4, pass: make deskd-build produced deskd/dist/deskd; usage lists -human-login; git status --porcelain clean apart from the ignored dist directory, which the verifier removed after the run.
- Row 5, pass: Local image assay-deskd:5729430698d44ffd874bb25275d056f5a5404dc6 (id sha256:b4e95f56edf6), User 65532:65532, revision label equal to the source SHA, /bin/sh absent (exit 127), offline -h lists -human-login, layer-secret scan clean. Control: a scratch Dockerfile copy with USER root built and its inspected User root failed the row's numeric non-zero check (rc 1).
- Row 6, fail: LINT: FAIL 1 problem(s). The one PROBLEM names desktools-v2/65 (protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required). No PROBLEM names cell-api/08; the row's expectation (exit 0, no PROBLEM) is not met at this source.
- Row 7, pass: consumers corroboration for 48ca54f64ffd over 8f2d6865c6b2: 12 corroborated, 0 disproved, 0 unchecked. The two non-path entries (the cell-api/05 pin and release-set entries) are corroborated as review obligations and stay with cell-api/05.

Supplemental, no acceptance credit: with the row 3 skip guard repointed to the ci.yml workflow in a scratch archive, both registry tests FAIL with 81 subtest failures, the deskdimage_test.go entry among them. Causes: registry rows name workflow files this repository does not carry (tools.yml, statusgen.yml, leaksweep.yml), ci.yml has no on.pull_request.paths filter, and opt-out rows are stale. The registry is never exercised here, so the CI-reach obligation of row 3 is unproven.

RISK-VALUE: DERIVED — syntax frontend pin = docker/dockerfile:1.27.1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e @ images/deskd/Dockerfile:1 — byte-identical to the reviewed pin at containers/base/Dockerfile:1; tag plus digest, so a re-pushed tag cannot change it.
RISK-VALUE: DERIVED — build base pin = golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 @ images/deskd/Dockerfile:31 — byte-identical to images/verify-runner/Dockerfile:26 and images/assay-harness/Dockerfile:50.
RISK-VALUE: DERIVED — final user = 65532:65532 @ images/deskd/Dockerfile:66 — numeric, both non-zero, matching the passwd and group entries the build stage writes at lines 50-51 (the conventional distroless nonroot id); the built image reports the same User.
RISK-VALUE: DERIVED — licence label = Apache-2.0 @ images/deskd/Dockerfile:58 — matches deskd/LICENSE (Apache License, Version 2.0).
RISK-VALUE: DERIVED — revision default = unknown @ images/deskd/Dockerfile:55 — a placeholder that the Makefile always overrides with the source SHA (or SHA-dirty); row 5 shows the override took effect.
RISK-VALUE: DERIVED — could-not-check exit = 2 @ Makefile:148-149 — non-zero, distinct from build failure, so a missing docker or daemon never reads as pass.

**VERIFY: FAIL** — row 3: both named registry tests skip on an absent workflow fixture, which the brief defines as failure; row 6: lint red on one PROBLEM in an unrelated brief (desktools-v2/65, could-not-check human authority). Rows 1, 2, 4, 5 and 7 pass. The item does not advance.

## Review

Gate: model. Review that the build definition is pinned, credential-free and local-only,
that the Windows mirror and CI registry are complete, and that the negative fixtures fail
for their named reasons. A passing image build grants no publication, pin, deployment or
exposure. Those remain cell-api/05's human decision and the release's separate act.
