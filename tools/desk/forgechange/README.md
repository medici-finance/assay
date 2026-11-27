# Forge change library

Import `github.com/medici-finance/assay/tools/desk/forgechange` to build a GitHub PR or GitLab MR tool. Construct a client with the forge name, canonical HTTPS project URL, repository slug and credential. The project URL binds GitHub to github.com and GitLab to its configured HTTPS installation. Use `OpenChanges`, `OpenChangeRows`, `Comments`, `Change` and `ChangedFiles` for reads. `Change` reports the change author and draft flag. `OpenChangeRows` returns the open changes newest first with number, author, draft flag, state, head, base and URL in one listing read, with no per-change or pipeline reads; it reports when the listing stopped at its cap. `ChangesUpdatedSince` returns changes updated at or after a bound, with draft flags. Its coverage says whether the read was complete. An incomplete read leaves `ObservedUpTo` zero, so keep the previous watermark. `MintGitHubToken` exchanges an App private key for an installation token restricted to one repository.

`PublishComment` posts an ordinary conversation comment. Supply the reviewed head, the operator-authorized repository visibility and any publication scan inputs. The library checks current visibility and head before writing or accepting an already-posted receipt. Callers own credential caching, retries and the publication receipt store. The library trims whitespace and normalizes case in publication lists. For private repositories, it checks both full slugs and short references such as `name#4`. Use the `PublicationNotice` callback to collect sanitized scan notices, including when no withheld identifiers are supplied.

`ChangedFiles` brackets the file list with change reads and returns the observed head. Compare it with any earlier head before using their data together.

The GitLab conversation list carries comment text and identifiers. For command discovery, fetch the individual MR note using GitLab's notes API for its visibility, author type and unrounded edit timestamps.

Build against a pinned Assay source checkout. Add your module and Assay's `tools/desk` module to a temporary Go workspace. The workspace uses the Assay module's sibling replacements.

```sh
go work init /path/to/your/module /path/to/assay/tools/desk
go -C /path/to/your/module build .
```

At runtime the tool uses the supplied credentials and policy. It can run with an empty home directory.
