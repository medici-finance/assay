# Consuming `tools/desk` as a Go module

The desk tooling ships its reusable Go packages under
`github.com/medici-finance/assay/tools/desk`. If you import one of those
packages from a **different** Go module — for example the ask-pane numbers-rule
layer at `github.com/medici-finance/assay/tools/desk/askassay` — read this before
you write the `require` line. Getting the version form wrong produces an error
that reads like a network or typo problem but is neither.

## `tools/desk` is a submodule, not the repository root

This repository is a Go **multi-module** repository. `tools/desk/go.mod` declares
its own module:

```
module github.com/medici-finance/assay/tools/desk
```

so `tools/desk` is versioned **independently of the repository's own release
tags**. That distinction is the whole of what this page exists to explain.

## A bare repository tag does NOT resolve

The repository's releases are tagged in the bare `vX.Y.Z` form (`v0.27.0`,
`v0.26.0`, …). Those tags version the repository, not the `tools/desk`
submodule. Go gives a subdirectory module a semantic version only from a tag
spelled `<module-subdir>/vX.Y.Z` — here that would be `tools/desk/vX.Y.Z` — and
**no such tags exist**. So a consumer that pins the bare repository tag:

```
go get github.com/medici-finance/assay/tools/desk@v0.27.0
```

does not get a wrong-but-working build. It gets:

```
invalid version: unknown revision tools/desk/v0.27.0
```

Go looked for the tag `tools/desk/v0.27.0` — the only form that could version
this submodule — did not find it, and refused. Adding `tools/desk/*` tags is a
deliberate release decision, not something a consumer can assume; until such a
decision is made and recorded, the bare-tag form above will keep failing exactly
like this.

## The form that resolves: a commit pseudo-version

Pin a **merged commit** instead. Go derives a *pseudo-version* from it — a
synthetic `v0.0.0-<timestamp>-<short-sha>` — which is a legitimate module
version:

```
go get github.com/medici-finance/assay/tools/desk@<merged-commit-sha>
```

This writes a line like

```
require github.com/medici-finance/assay/tools/desk v0.0.0-20260905010510-41d799390e8a
```

into your `go.mod`. Use a commit that is on the default branch (`main`); a
pseudo-version is derived from a real commit, so the SHA must be one the module
proxy can see.

The repository is **public**, so it resolves through the public Go module proxy
with no `GOPRIVATE` configuration on the consumer's side.

## The integrity record is your `go.sum`

A pseudo-version is content-addressed. When you first add the dependency, Go
records a cryptographic checksum of the module's contents at that commit in your
own module's `go.sum`. Every later build re-verifies the downloaded module
against that recorded checksum, so a substituted or mutated module fails
verification in **your** build — independent of anything this repository does.
That makes a pseudo-version pin a stronger integrity record than a tag, which can
in principle be re-pointed. Commit `go.sum` alongside `go.mod`.

## In short

| You want to pin… | Write | Result |
|---|---|---|
| The `tools/desk` submodule | `@<merged-commit-sha>` (a pseudo-version) | resolves; checksummed in your `go.sum` |
| The bare repository tag | `@vX.Y.Z` | `invalid version: unknown revision tools/desk/vX.Y.Z` |
