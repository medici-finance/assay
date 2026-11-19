# Five-role Kubernetes compatibility qualification

From a source checkout on Linux or macOS, run the lifecycle milestone with Python 3,
the repository Go toolchain and a populated module cache.

```sh
sh containers/k8s/compatibility/qualify.sh --offline --suite lifecycle
```

The runner builds the ordinary `cellctl` CLI. Its existing Kubernetes fixture
provides a temporary TLS API on loopback and a disposable cell profile. The
test checks the offline plan before any API call, starts every role, repeats
start, and attaches twice from separate client processes. It checks terminal
input and output and compares each role's execution identity. It then confirms
a controlled stop, reads the receipt for that same execution and verifies that
the explicit next start gets a new Pod and container under the same workload.

The JSON result names each role that passed. A failed command, skipped role or
missing test leaves the milestone incomplete. Image digests and process identities
in this run belong to the fixture. Keep the result alongside its source revision.

The five-role useful-work and resize exercise, container-level restart evidence,
and witnessed operator run remain open in
[desk-containers/32](streams/desk-containers/brief-32-qualify-the-compatibility-kubernetes-desk-cell.md).
The hosted run also needs scoped operator bindings, accepted transcript custody
and a published image cohort. The lifecycle command only supports offline use.
