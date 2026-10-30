### Changed
- In-cluster release jobs hand prepared artifacts through an explicitly configured S3-compatible store, binding the complete file set to trusted job-output hashes, run, source and version; hosted native ARM64 checks retain GitHub artifacts. Dedicated operator-provisioned access is required, with no automatic fallback.
- Release preparation runs the complete bodycheck mutation harness independently and losslessly compresses the checked native ARM64 image during artifact transfer. Every mutation verdict, native image check and publication gate remains required.
