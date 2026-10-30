# Fail-first history for isolated verification

A linked worktree's `.git` file points to metadata outside its source mount.
Mounting only that home into a network-isolated container therefore does not
provide the baseline commits needed by `verifyrun --fail-first`.

Use the host admission step's optional `--history-out` to export a Git object pack:

```sh
statusgen verifyrun --open-bracket --root "$home" --brief "$brief" \
  --out "$receipt" --history-out "$history_pack"
```

Both output paths must be outside the home and different. The pack output
must be new. Export requires complete history and HEAD equal to the admitted
source. Only objects reachable from HEAD are exported, without repository
configuration, hooks, remotes, credentials or unrelated branch refs.

Mount the home, receipt and pack read-only into the existing digest-pinned
runner started with `--network none`. Keep witness output, module caches and
temporary baseline checkouts outside the mounted home. Add the mounted pack
path to the ordinary container invocation:

```sh
statusgen verifyrun --root /work --brief /work/docs/streams/example/brief-01-example.md \
  --sandbox=container-netns --admission-receipt /input/receipt.json \
  --container-image-digest "$image_digest" --fail-first --base "$delivery_base" \
  --history-pack /input/history.pack --witness /output/witness.md
```

Use the actual admitted brief, immutable delivery base and image digest; these
examples supply no approval or verification authority. Existing `--rows`
selection remains available for disjoint host and container obligations.
The pack is owner-readable only. On POSIX hosts run the container as the host
UID/GID, as the existing container launcher does; keep the pack mount read-only
rather than widening its host permissions.
Close the bracket on the host as before. Admission, unchanged-source digests,
network isolation, witness provenance, approval and independent verdict checks
still apply. A newer host binary alone is insufficient: the pinned container
runner must also support `--history-pack`.

The container imports objects into a new private repository outside the home,
validates their connectivity and requires the receipt's full source commit and
all reachable objects, including blobs, to be present. Its baseline ancestry
anchor is that source, never a caller-supplied branch ref. For post-merge
verification this matches the host's merge-base anchor. For a branch-head home,
the container's early check accepts ancestors of the source, including commits
after the branch diverged; the closure/landing gate still checks the witness
against the real default-branch merge-base. The brief-at-base check still runs.
Baseline worktrees use only this private metadata and are removed after
the run; head commands still execute on the original admitted mount. No writable
host Git directory or forge access is required in the container.

This option currently requires a home-relative brief. Cross-repository briefs
remain on their existing path. `--history-pack` is refused outside a
container-netns fail-first execution. Missing, incomplete, corrupt or
wrong-source history refuses before any Verify row runs; it never produces a
synthetic baseline failure or verification credit. `--in-container` and the
acceptance-transition approval rules are unchanged.

This exports complete history, not a bounded delivery diff. Resource use and
pack size are not capped: a review benchmark on a loaded machine for a roughly
7,000-commit repository took about 13 minutes wall time, 1 GB peak RSS and a
160 MB pack; import took about 40 seconds. Export time counts against the
bracket window. Provision scratch space and memory accordingly; a narrower or
faster transport is future work, not a relaxed integrity check. If receipt
writing fails after export, the completed pack remains outside the home. Keep
it as diagnostic output and use a new output path for the next open attempt.
