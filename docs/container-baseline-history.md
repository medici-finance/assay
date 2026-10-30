# Fail-first history for isolated verification

A linked worktree's `.git` file points to metadata outside its source mount.
Mounting only that home into a network-isolated container therefore does not
provide the baseline commits needed by `verifyrun --fail-first`.

Use the host admission step's optional `--history-out` to export a Git bundle:

```sh
statusgen verifyrun --open-bracket --root "$home" --brief "$brief" \
  --out "$receipt" --history-out "$history_bundle"
```

Both output paths must be outside the home and different. The bundle output
must be new. Export requires complete history and HEAD equal to the admitted
source. Only HEAD and its reachable objects are exported, without repository
configuration, hooks, remotes, credentials or unrelated branch refs.

Mount the home, receipt and bundle read-only into the existing digest-pinned
runner started with `--network none`. Keep witness output, module caches and
temporary baseline checkouts outside the mounted home. Add the mounted bundle
path to the ordinary container invocation:

```sh
statusgen verifyrun --root /work --brief /work/docs/streams/example/brief-01-example.md \
  --sandbox=container-netns --admission-receipt /input/receipt.json \
  --container-image-digest "$image_digest" --fail-first --base "$delivery_base" \
  --history-bundle /input/history.bundle --witness /output/witness.md
```

Use the actual admitted brief, immutable delivery base and image digest; these
examples supply no approval or verification authority. Existing `--rows`
selection remains available for disjoint host and container obligations.
Close the bracket on the host as before. Admission, unchanged-source digests,
network isolation, witness provenance, approval and independent verdict checks
still apply. A newer host binary alone is insufficient: the pinned container
runner must also support `--history-bundle`.

The container imports objects into a new private repository outside the home,
validates their connectivity and requires fetched HEAD to equal the receipt's
full source commit. Its baseline ancestry anchor is that source, never a
caller-supplied branch ref. The ordinary ancestry and brief-at-base checks still
run. Baseline worktrees use only this private metadata and are removed after
the run; head commands still execute on the original admitted mount. No writable
host Git directory or forge access is required in the container.

This option currently requires a home-relative brief. Cross-repository briefs
remain on their existing path. `--history-bundle` is refused outside a
container-netns fail-first execution. Missing, incomplete, corrupt or
wrong-source history refuses before any Verify row runs; it never produces a
synthetic baseline failure or verification credit. `--in-container` and the
acceptance-transition approval rules are unchanged.
