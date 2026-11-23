### Fixed

- `deskwt remove` and `deskwt prune --reap-dead-sessions` now read a dispatched home's claim in the repository that took it. A cross-repository verifier's home, cut from the deliverable checkout while its claim lives in the tracking repository, is removed once that claim is released and the tree is clean and pushed. A held claim, an unreadable listing, or a dirty or unpushed tree still refuses.
- A dispatched home recorded before claim namespaces were recorded still holds, and the refusal now names the sanctioned path: `deskwt remove <home> --claim-repo <owner/name>` states the repository its dispatch claimed in. The flag must agree with any namespace the home records and relaxes no other gate.
