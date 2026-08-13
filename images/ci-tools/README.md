# ci-tools

A tiny, public **carry image** of two pinned CI binaries:

- **`gh`** — the GitHub CLI (`github.com/cli/cli`)
- **`crane`** — the OCI registry tool (`github.com/google/go-containerregistry`)

Published at **`ghcr.io/medici-finance/ci-tools`**, **public / anonymously pullable**
(no registry auth or `imagePullSecret` needed).

## Why it exists

Some build environments have **no egress to the GitHub release-asset CDN**
(`objects.githubusercontent.com`), so a Dockerfile that `curl`s a prebuilt `gh` or
`crane` straight from a GitHub release fails there. This image bakes those tools at
stable absolute paths so a downstream build pulls them from the **registry** (which is
reachable) instead of the release CDN (which may not be):

```dockerfile
COPY --from=ghcr.io/medici-finance/ci-tools:gh-2.97.0 /usr/local/bin/gh    /usr/local/bin/gh
COPY --from=ghcr.io/medici-finance/ci-tools:gh-2.97.0 /usr/local/bin/crane /usr/local/bin/crane
```

Or run a tool directly where a Docker daemon is available:

```sh
docker run --rm ghcr.io/medici-finance/ci-tools:gh-2.97.0 gh --version
```

## How it is pinned

Both tools are **built from source** with `go install <module>@vX.Y.Z`. The Go
toolchain fetches module source through the module proxy (`proxy.golang.org`) and
verifies it against the checksum database (`sum.golang.org`). That module + checksum
verification **is** the pin — there is no release-CDN download and no tarball sha256 to
carry. The proxy and checksum DB are plain public HTTPS.

The recipe is fully **reproducible on any machine** with Docker and Go-proxy access:

```sh
docker build -t ci-tools images/ci-tools
```

To bump a tool, edit its `ARG *_VERSION` in the [`Dockerfile`](./Dockerfile); the
checksum verification gates the fetch and fails the build on any tampered source.

## Provenance & pinning for consumers

- The image carries OCI labels: `org.opencontainers.image.source` (this recipe),
  `.revision` (the recipe commit), `.version` (the tool pins), and `.licenses`
  (`MIT AND Apache-2.0` — `gh` is MIT, `crane` is Apache-2.0).
- **Pin by an immutable tag or digest**, never `:latest` — e.g.
  `ci-tools:gh-2.97.0`, `ci-tools:sha-<short>`, or `ci-tools@sha256:…`.

> The image is built from this public, reproducible recipe. The build itself runs on
> the maintainers' own CI infrastructure; the reproducibility of the recipe — not the
> builder — is the trust anchor. Rebuild it yourself with the command above to verify.
