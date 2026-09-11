---
id: DR-forge-neutral-09
date: "2026-09-11"
title: "Express the leak gate's verdict per-forge with an absent-is-could-not-check contract, and make cellctl forge-aware without acquiring GitLab credentials"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Make the GitHub leak-gate verdict ADVISORY so the two forges look symmetric (GitLab CE cannot express a blocking external verdict) — ruled out: weakening the layer a forge CAN enforce to match one it cannot is a disclosure-control regression. The answer to a forge that cannot express a blocking external verdict is to add the pipeline-side layer for THAT forge, not to demote the layer the other forge blocks on. The brief's Ground rules forbid it and the human gate reads for it."
  - "Specify only the external verdict and leave the pipeline-side sweep to the adopter — ruled out: on GitLab CE the blocking external-status-check surface is tier-gated (HTTP 401 on the pilot), so the external verdict alone leaves CE with NO gate at all. The free-tier compensator (a sweep job in the change's own pipeline) is required rather than optional, and templated into the CI half so an adopter does not have to remember it."
  - "Let `cellctl` acquire GitLab credentials on the adopter's behalf to make the GitLab path easier — ruled out: the custody hand-steps are a deliberate design (a human moves credential material knowingly). `new` mints nothing on either forge; the GitLab role token store is provisioned by hand exactly as the GitHub App PEM symlinks are."
accepted:
  - "On GitLab CE the merge blocker is the pipeline-side sweep JOB, not an external status check: adopters must make the `leaksweep` job a REQUIRED pipeline step. On GitLab Ultimate the external status check blocks and the sweep job is the independent second layer; on GitHub the `leak-sweep` commit status blocks and the in-tree controls run in CI as the second layer."
  - "`cellctl` gains a forge dimension: `--forge github|gitlab`, and `--deskd-app-pem`/`--orgs` become required on the github path only. A GitLab cell requires a hand-provisioned role token store (`gitlab-<role>.token`, 0600) and forge-qualified roster entries (`role=gitlab:<slug>:<id>`, per forge-neutral/02); the hardcoded `api.github.com` host is replaced by the cell's configured forge endpoint."
  - "A MISSING leak-gate verdict is could-not-check on the ready-flip decision, never a pass: `deskflip` now cross-checks that every branch-protection-required context (the `leak-sweep` status among them) is PRESENT in an otherwise-green rollup and refuses could-not-check when one is absent. This extends the empty-rollup three-state contract to a rollup that is non-empty but incomplete."
---

The driver (`human:<name>`) ratified `forge-neutral/09` — expressing the leak gate's verdict
per forge with an absent-is-could-not-check contract, adding the free-tier pipeline-side sweep
compensator, and making `cellctl new`/`deskd`/`check` forge-aware — at the brief's own human
decision gate.

The ruling was recorded on the brief's decision-gate issue,
[issue #816](https://github.com/medici-finance/assay/issues/816), which the driver closed
`approve-as-briefed` at 2026-09-11T02:15Z. Per the decision-gate template the issue offered
approve as-briefed, approve with changes, or hold/reject; the recorded close is **option 1 —
approve as briefed** — confirming the two controls the brief's `gate-why` puts to the human:

- the per-forge **custody shape** `cellctl` provisions (a GitHub cell mints installation tokens
  from an App PEM; a GitLab cell reads a hand-provisioned role token store and mints nothing);
  and
- that the GitLab leak gate's **absence reads as could-not-check**, never as clean — enforced
  both on the ready-flip decision (`deskflip`) and, on CE, by the required pipeline-side sweep
  job.

This closes the human gate the brief's frontmatter declares (`gate: human`, `risk:
{sensitive-data: yes}`). The constraint behind the decision is that both halves are controls
that fail in the quiet direction: a cell standing up with a credential nobody scoped, or a gate
whose verdict lands nowhere and reads as absent-therefore-fine. The design's answer is two
independent layers per half — for the leak gate, the external verdict AND the pipeline-side
sweep, catching a change on different signals in different components; for `cellctl`, the
custody hand-steps AND `check`, which independently re-reads each precondition per forge rather
than trusting that `new` performed it.

**What this record does not decide.** It does not weaken the GitHub-side control to make the
forges symmetric — the alternative above is explicitly ruled out. It does not, by itself, prove
the approver is a different identity from the brief's author — the same attribution-not-identity
limit `lifecycle-v1.md` §7.1.2 declares for verification, and `registers-v1.md` §7.4 for this
register. And it does not attest the chosen design is correct: that the alternatives were
weighed is recorded here; whether the per-forge verdict surfaces and the custody shape are right
is the review gate's judgement, then the change's own validation after it lands.

**Grandfathering note.** `forge-neutral/09` was authored 2026-09-02, on or before the
design-approval-gate cutover (2026-09-05, `spec/lifecycle-v1.md` §4.4), so the gate does not bind
it and this record is not required to clear `statusgen --lint`. It is authored anyway, to make
the granted decision durable and citable on the public artifact rather than living only on the
now-closed issue.
