### Fixed
- GitLab `ChecksAtHead` now publishes the head pipeline as the required `pipeline` status
  context even when the commit document's `last_pipeline` is empty or stamped with a
  different SHA — the ordinary `merge_request_event` shape. It falls back to the same by-SHA
  read (`pipelines?sha=`) the board already uses, still reconciling on the exact head SHA, so
  `deskflip` checks-green no longer refuses a genuinely green MR pipeline. A head with no
  pipeline from either source stays could-not-check, never a pass.
