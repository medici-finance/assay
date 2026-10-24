### Added
- the verify-runner image now ships the cluster backend's three fixed entrypoints,
`stage-source.sh`, `stage-cache.sh` and `witness.sh`, at `/usr/local/bin`, root-owned
and mode 0755, so an admission policy that pins each container's command to one
baked script path can admit the stage and witness Jobs. The image's ENTRYPOINT and
CMD are unchanged. Each takes a fixed argument shape: the stage scripts
`--repo=<owner/repo> --sha=<40-hex>` or `--probe`, the witness script one word
(`seed`, `seed-probe`, `probe` or a `<stream>/<NN>` brief key). A brief-key run is
the container half of the verification bracket: it needs the host's admission
receipt and the image digest under `/run/verify-admission`, passes both to
`statusgen verifyrun --sandbox container-netns`, and prints the witness on its last
log line; without them it exits 2 and no row runs. The stage scripts drop every
inherited `GIT_*`, proxy and shell-startup variable, and the cache stage reads each
`go.mod` with Go's own parser, with workspaces off, before any module request.
The release probe (`images/verify-runner/check.py`) now fails the build unless
`/usr/local/bin` and each script are owned by `root:root` and writable by neither
group nor others, each script is an executable regular file with no set-id bit,
and, executed directly with no argument as the non-root user, each prints its usage
and exits 2.
