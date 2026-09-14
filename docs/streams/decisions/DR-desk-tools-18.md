---
id: DR-desk-tools-18
date: "2026-09-11"
title: "Public-repo write gate: an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep the existing per-item `+1` reaction gate (a human reacts on a referenced issue/PR before each outward write) — ruled out: it is unsatisfiable for the write that matters most. `deskpr create` has no PR number yet and passes the trailer's issue number, which is 0 for a brief-carrying PR, so the gate refuses (exit 6) every time — no desk can open the first PR on a public repo without a human first hand-creating a sentinel file. It also re-makes, per item, a decision that is really scoped to the repository ('this repo is a place the desk may write'), and it costs an API call per write."
  - "Keep the standing per-repo sentinel file (`~/.config/assay/public-app-ok`, read by `publicbless.go`) as the durable bypass — ruled out: it is an un-audited, operator-local flag with no record of who set it or why, coexisting with the per-item gate as a second undocumented authorization path. Retiring the READER (not deleting operators' files, which stay theirs) collapses authorization to one path: the configured allowed-repos entry."
  - "Add a flag, environment read, or CLI argument that can admit a repo at call time, instead of reading only the out-of-band configured set — ruled out (brief's own Ground rules): the set must stay configured outside every ref the tools evaluate, so that no pull request can add its own repository to the write set. A call-time knob on a security gate is a waiver, not a control."
accepted:
  - "`PublicRepoGate(fetcher, owner, repo)` drops the issue-number parameter entirely (not defaulted) so the unsatisfiable 'issue 0' state cannot recur at any call site — the compiler finds every caller of the old signature."
  - "A `public`/`internal` live-visibility read passes ONLY when the repo has an EXPLICIT allowed-repos entry whose configured visibility is `:public`; absent, pattern-matched-only, `:private`-tagged, or untagged entries all refuse (exit 5) naming the repo, what was read live, what the set says, and the remedy."
  - "The write gate reads LIVE visibility (not just the configured claim), so a repo flipped to public after the roster was written is still gated — the configured value and the live read must agree before a write is authorized."
  - "The sentinel file's reader is retired (not the file itself, which is the operator's); the per-item `+1` reaction check and the `Reaction`/`ReactionUser` types are removed from the fetcher interface entirely."
  - "The authorization is repository-scoped, covers every write-capable verb through the single gate choke point, and merge remains the human's — a draft PR opened under this gate is inert until a human merges it."
---

**Ruling recorded (2026-09-11): APPROVED — approve as briefed.** The driver (`human:<name>`)
recorded the ruling on the brief's decision-gate
[issue #809](https://github.com/medici-finance/assay/issues/809) — the
[ruling comment](https://github.com/medici-finance/assay/issues/809#issuecomment-5636096251)
(2026-09-11T14:37:18Z) — as **"1"**, approving the design as briefed: a public repo listed in
the roster's allowed-repos set with the `:public` tag passes all outward writes for trusted
identities; an unlisted public repo still refuses; the per-item `+1` reaction check is retired.
This record transcribes that ruling into the register; it does not mint a new one — the human
act was the driver's "1" on #809, not this file.

**The decision.** `docs/streams/desk-tools/brief-18-allowed-public-repos-write-gate.md` replaces
the public-repo write gate's per-item `+1` reaction requirement with a standing,
repository-scoped authorization read from the allowed-repos configuration's `:public` /
`:private` tags. The brief's own `sources:` records the original maintainer ruling
(2026-09-06) that this design implements; issue #809 is the decision-gate's formal closure of
that same ruling for the lifecycle's design-approval-gate requirement (brief-18 was authored
after the design-approval-gate cutover and needs a cited `DR-<slug>`, which did not exist until
this record).

**The constraint behind it.** After this change the ONE control standing between a desk tool
and an outward write to a public repository is the configured allowed-repos entry carrying
`:public` (brief's own Context, `single-point-of-failure`). Four layers behind it, each failing
on a different signal in a different component: (1) the LIVE visibility read — a repo whose live
visibility disagrees with, or cannot confirm, the roster's claim refuses regardless of what the
roster says; (2) the item-level author-trust / blessing gate in `deskpost`, an identity signal
unrelated to which repo is targeted; (3) branch protection and human merge — every PR this gate
lets a tool open is a draft, inert until a human merges it; (4) the append-only audit log, so an
authorization that should not have been granted is discoverable after the fact.

**What this record does not decide.** It does not attest the design is correct — that the
alternatives were weighed is recorded here; whether the gate's shape is right is the review
gate's judgement (brief-18's own Review section), then the change's own Verify table after it
lands. It does not touch the item-level author-trust gate in `deskpost`, a different control on
a different signal, left as-is by the brief.
