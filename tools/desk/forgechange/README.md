# Forge change library

Import `github.com/medici-finance/assay/tools/desk/forgechange` to build a GitHub PR or GitLab MR tool. Construct a client with the forge name, canonical HTTPS project URL, repository slug and credential. Use `OpenChanges`, `Comments`, `Change` and `ChangedFiles` for reads. `MintGitHubToken` exchanges an App private key for an installation token restricted to one repository.

`PublishComment` posts an ordinary conversation comment. Supply the reviewed head, the operator-authorized repository visibility and any publication scan inputs. The library checks current visibility and head before writing or accepting an already-posted receipt. Callers own credential caching, retries and the publication receipt store. Use the `PublicationNotice` callback to collect the diagnostic when the publication policy supplies no withheld identifiers.

`ChangedFiles` brackets the file list with change reads and returns the observed head. Compare it with any earlier head before using their data together.

The GitLab conversation list carries comment text and identifiers. For command discovery, fetch the individual MR note using GitLab's notes API for its visibility, author type and unrounded edit timestamps.

Build against a pinned Assay source checkout. Add your module and Assay's `tools/desk` module to a temporary Go workspace. The workspace uses the Assay module's sibling replacements.

```sh
go work init /path/to/your/module /path/to/assay/tools/desk
go -C /path/to/your/module build .
```

At runtime the tool uses the supplied credentials and policy. It can run with an empty home directory.
