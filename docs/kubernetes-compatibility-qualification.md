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

Run `python3 -I containers/k8s/compatibility/qualify.test.py` to replay the
runner's failure cases. The test feeds recorded Go events to the ordinary
qualification command and checks its exit code and diagnostic for each case.

## Offline suites for desk-containers/32

Use a checkout containing the candidate's recorded Git revision. Run
`sh containers/k8s/compatibility/qualify.sh --help` to see the suites present
in that checkout. Once all three implementations are present, run these commands
from the repository root.

```sh
sh containers/k8s/compatibility/qualify.sh --offline --suite interaction
sh containers/k8s/compatibility/qualify.sh --offline --suite controlled-restart --self-test
sh containers/k8s/compatibility/qualify.sh --offline --suite custody-target --self-test
```

| Suite | Additional local inputs | What the result covers |
|---|---|---|
| Interaction | Explicit local Unix Docker socket in `ASSAY_TEST_DOCKER_HOST` and a cached Linux/Python fixture image | Five compiled CLI role paths, terminal I/O and resize, stable execution and invocation counts across detach, reconnect, repeated up and operator-client loss. |
| Controlled restart | Same socket and cached fixture image | Controlled stop and explicit start, retained work file and rejection of planted restart/continuity defects. |
| Custody and target selection | Pinned Kustomize version from `containers/k8s/check.py` on `PATH` | Offline target/receipt refusals and rejected mutations to declared credential, terminal, log, network and container policy. |

The Docker suites use cached `python:3.13-slim` or a local immutable image ID
provided in `ASSAY_TEST_FIXTURE_IMAGE`. See the [interaction fixture setup](../containers/k8s/components/compatibility-cellctl/README.md#offline-interaction-suite)
for the runtime boundary. Retain the JSON output from each command separately.

Independent qualification, container-level restart evidence and the witnessed
operator run are tracked in
[desk-containers/32](streams/desk-containers/brief-32-qualify-the-compatibility-kubernetes-desk-cell.md).
The hosted run also needs scoped operator bindings, accepted transcript custody
and a published image cohort. The lifecycle command only supports offline use.
