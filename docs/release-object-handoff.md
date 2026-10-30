# Checked release object handoff

The release producer builds the published binaries and suites once. In-cluster jobs use an explicitly configured S3-compatible store for those exact prepared files. The hosted native ARM64 job continues downloading GitHub artifacts and returns its checked native image through GitHub artifacts. The publisher imports that exact checked image; a source revision alone is never substituted for its native proof.

## Trust and membership

`tools/release/transport.py` creates a deterministic bounded archive containing the complete seven-group GitHub upload membership: five complete platform groups, the ARM64 statusgen group and the common files. The optional source bundle is required exactly when prepared source differs from the dispatch base. The manifest binds repository, run, attempt, base, prepared source, version, each filename, byte size and SHA256. Its name roster is checked against the GitHub upload contract by offline tests.

The producer records bundle SHA256, manifest SHA256 and byte length as GitHub job outputs only after the configured upload and object-length readback succeed. The object key includes run, attempt, prepared source and content digest. The consumer takes expected values from those job outputs, not from an object-side checksum file. It checks object length, requests the bounded byte range, verifies downloaded archive and manifest hashes, rejects missing/duplicate/extra members, links, unsafe names and malformed records, and verifies every file before adding any prepared input to the workspace. Existing build-inputs/checksums, native image receipt/config/image-ID admission, image tests, approval, provenance and publication ordering remain required.

A missing configuration, failed operation or proof mismatch stops the workflow. There is no automatic GitHub artifact fallback for an in-cluster failure. Hosted native ARM64 retains its existing independent GitHub path and receives no object credentials.

## Operator prerequisites

The two object-enabled release pool labels are additive. Existing pools and network fences are unchanged. Operator-controlled TLS routing, narrow bucket/prefix policies and protected environment credentials must be provisioned and qualified before activating the new pools. No deployment, credential provisioning, storage probe or release is performed by this source change.

The producer uses environment `release-object-store-producer`; the publisher retains environment `release`. Both entry paths, tag push and workflow dispatch, must be covered by the operator's protected-ref policy. This source does not claim an exclusive selected-workflow runner-group boundary.

Each environment supplies these transport-step secrets:

- `RELEASE_OBJECT_ENDPOINT`: explicit HTTPS endpoint, including an explicit port when needed.
- `RELEASE_OBJECT_BUCKET` and `RELEASE_OBJECT_REGION`: the chosen store coordinates.
- `RELEASE_OBJECT_ACCESS_KEY_ID`, `RELEASE_OBJECT_SECRET_ACCESS_KEY`, and optional `RELEASE_OBJECT_SESSION_TOKEN`: distinct scoped producer and consumer credentials.
- Optional `RELEASE_OBJECT_CA_BUNDLE`: an operator-provisioned readable absolute regular-file path, not inline PEM. Symlink and relative paths are refused. TLS verification is always enabled.

The producer needs scoped PutObject and GetObject for HEAD confirmation; multipart cancellation can require AbortMultipartUpload. The publisher needs scoped GetObject. HEAD is not a separate IAM permission. No root/admin, bucket listing, ACL, arbitrary deletion or registry credential is requested by this adapter. The operator qualifies actual SDK multipart, range, checksum and TLS compatibility on a non-production fixture before activation.

Only the explicit transport-step inputs enter the client environment. Ambient AWS profiles/shared configuration, instance metadata, container credentials and web-identity credentials are not inherited. Raw client errors and private endpoint/bucket values are not printed; progress identifies the logical content object.

## Runtime client and qualification

The Linux amd64 AWS CLI installer is pinned to 2.37.8 and SHA256 `6a2f98fee0901ff623ff9abb5c80f4d67c40e5c0ac5e2ad7afa2ba756a7d5088`, verified before extraction/execution. The [upstream release](https://github.com/aws/aws-cli/releases/tag/2.37.8) date is 2026-10-01; it was 28 days old at the 2026-10-29 assessment. The command-line installer does not auto-update. It uses only the job's temporary directory and no sudo/global installation. Updates require a reviewed version/digest change and a new dependency assessment; no vulnerability-free claim is made.

Run offline contracts with `python3 -B -m unittest discover -s tools/release -p '*_test.py' -v`. Run the separate canonical mutation controls with `python3 -B -m tools.release.transport_mutation --out <new-evidence-directory>`. The mutation harness requires a healthy baseline, a distinct caught control and every selected mutation caught with no survivor or unapplied target. These local controls do not claim additional CI-matrix enforcement.

The transport change needs a real workflow run for production latency qualification. A small local compression fixture and historical step durations do not establish a whole-release speedup.
