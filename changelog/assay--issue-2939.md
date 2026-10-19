### Fixed
- `verifyloop plan` reports current-source Evidence in a trusted verifier's open PR as awaiting merge, retaining source/wake bindings and surfacing unreadable provenance of a verifier-authored PR as could-not-check. PRs by any other author are ignored and never hold a brief.
- `verifyloop plan --no-forge` now reports every queued brief as could-not-check, including deferred, human and stuck-flip items, because absence of pending Evidence cannot be shown without forge reads.
