### Added

- `execution-profile/v1`, an extension of the shared runner protocol: an adapter declares qualified harness, model and backend profiles, each bound to its qualification evidence, with resume and restart preservation stated per dimension (files, conversation, process memory, descendants, pending interactions) and unknown kept apart from unsupported. A request pins one profile by digest and names what it needs; anything the profile does not promise is refused before Start, by the client and again by the adapter. The adapter's execution report is checked at acceptance, so a substituted backend or a partial restoration is refused. Spec `spec/loop-admin-execution-profile-v1.md`, schema `schemas/loop-admin-execution-profile-v1.json`, reference implementation and fixtures in `loopadmin`, and two new conformance kit cases (refusal before Start, and the adapter's own admission with the client's check bypassed).

### Changed

- The runner client now checks every extension value an adapter reports on a launch receipt or an observation, not only the execution report: credential material anywhere is refused, so is a value for an extension the request did not carry or the adapter does not declare, a refused observation is neither returned nor recorded, and the invocation can no longer be accepted.
- A launch request, and an extension value decoded on its own, longer than 1 MiB is refused before it is parsed.
- A qualified profile's digest hashes strings as written, with no HTML escaping of `&`, `<` or `>`, and a profile with a missing list no longer validates, so the digest has one byte sequence a non-Go encoder can reproduce; a fixed vector in the tests pins it.
