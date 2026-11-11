### Fixed

- `deskpost review` and `deskpost finish review`: a request-changes verdict on a GitLab merge request that has no merge-hold thread, such as one a person opened, now opens the thread, so the merge is held. Before, the verdict landed, the hold step exited 6, and the merge gate stayed open. An approve on such a change still opens nothing. A change a desk verb opened is handled as before.
