---
id: DR-issue-projections
date: "2026-10-17"
title: "Select issue populations before enrichment while preserving their distinct coverage"
consequence: minor
decided-by: "human:<name>"
alternatives:
  - "Continue broad enrichment for every caller: retained as the compatibility/full-board path, but not the selected optimization because worker, intake and inbox decisions require different populations."
  - "Add a cache or another service first: deferred by the approved specification until duplicate collection is removed and a residual bottleneck and validity/permission rules are measured."
  - "Copy policy into each caller: rejected in favor of shared pure selection with trust, identity and effects retained in their existing owners."
accepted:
  - "Each projection declares its population, freshness and completeness; an empty worker projection cannot establish intake or inbox idle."
  - "Trust identity/edit checks, comment pagination, escalation, addressed responses and positive closure evidence remain required for the selected decision. Unsupported batching uses the existing bounded fallback."
  - "Source qualification does not approve operational caller activation, executor changes, retirement, credential/custody changes, releases, live pin updates or collection. Existing delivery gates remain separate."
---

This record transcribes the existing approval of the
[decision-oriented reads specification](../../desk-workflow-efficiency-spec.md),
limited to its issue projection design in section 4 and acceptance A5. It grants
no new approval and does not claim the implementation is correct.

The maintainer merged [PR #3322](https://github.com/medici-finance/assay/pull/3322)
on 17 October 2026 at 20:45:29 UTC. The approved head was
`7504e6dbbaaba3faab6bf64b1c987fb756b4a980` and its merge commit was
`067c7f02bd39997d719d0f64d8576efbc6f55bcc`. The specification's subsequently
merged approval header records the maintainer's explicit confirmation that the
merge was approval and authorization to author the citing brief amendments.
[PR #3326](https://github.com/medici-finance/assay/pull/3326), merged by the same
maintainer at 21:49:47 UTC, routed those briefs. The forge merge metadata and the
approved specification corroborate this provenance. No issue-comment ruling was
found on either PR; this record therefore supplies no invented `ruling:` link.

The accepted design selects worker-ready, intake and addressed-inbox candidates
from shared open-issue observations before expensive history enrichment. Human
escalation-label inbox discovery remains a distinct population. Pure selection
stays inside the desk module; canonical trust and credentialed collection retain
their current owners. Caller migration requires compatible versions and rollback,
with ordinary full-board users inventoried before any retirement.

`consequence: minor` is a reversible recording default declared in this PR, not a
severity stated by the approving maintainer. It reflects a source-only read-path
change with retained compatibility; the maintainer may amend it.

This record does not certify desk-tools/38, complete desk-tools/42, approve the
separate perf-v3 implementation, or supply its eventual qualification. Independent
verification and all source-spec delivery/activation gates remain in force.
