---
id: DR-perf-v3-prop
date: "2026-10-19"
title: "Accept bounded local performance context across tool and launch boundaries"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/3537#issuecomment-6281511664"
alternatives:
  - "Hold for correction of a field, lifecycle or failure boundary: this was the second offered option; the human selected acceptance instead."
  - "Forward arbitrary inherited workflow metadata through launches: excluded by the accepted validated-context and consent boundary."
  - "Activate remote collection with propagation: excluded from this local decision and retained behind separate activation controls."
accepted:
  - "Explicit workflow consent permits bounded, validated local work and agent linkage to propagate through the documented tool and launch boundaries."
  - "Opt-out and tool-only use cannot retain that linkage; invalid optional metadata cannot change the underlying operation's result."
  - "Invocation and parent identities describe recorded work only and grant no custody, credential, dispatch or lifecycle authority."
  - "The implemented local disclosure and isolation boundary is accepted while independent review, verification and remote activation controls remain separate."
---

This record transcribes the human's option 1 comment on the closed decision issue.
It records an existing acceptance and makes no new review or verification claim.

https://github.com/medici-finance/assay/issues/3537#issuecomment-6281511664

The bounded propagation contract is specified in
[performance attribution](../../performance-attribution-spec.md) and implemented
by desktools-v2/64 using the shared recorder and existing launch adapters. The
accepted boundary validates optional identity before forwarding it and preserves
the distinction between operational role authority and diagnostic context.

The optional handoff's expiration is checked, while physical pruning is
opportunistic on a subsequent write. A final handoff file can remain on disk
beyond its expiration; this record preserves that documented limitation and
does not claim a hard-deletion guarantee.

`consequence: major` records the local privacy consequence of incorrectly
propagating identifiable work linkage. This acceptance does not authorize uploads,
credential changes, weakened controls or bypasses of CI and independent review.
