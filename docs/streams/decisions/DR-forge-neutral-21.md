---
id: DR-forge-neutral-21
date: "2026-09-23"
title: "Lift the dispatch-claim store behind one deskkit seam, resolved from the roster with no caller choice and refusal as the only fallback; keep forge-ref only as what an unset key means, for one release window"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Approve, but refuse immediately when nothing is configured (no notice period) — ruled out: put to the driver as option 2 on #1552 and not taken; the ruling selected option 1. Under option 2 every existing install would stop at its next upgrade until someone edited its configuration, where the brief keeps today's behaviour for one release window under a NOTICE that names the key, its two valid values and the removal release."
  - "Reject — claims stay on the platform only, and the reviewer keeps repository write — ruled out: put to the driver as option 3 on #1552 and not taken; the ruling selected option 1. The spec this brief implements (`docs/streams/forge-neutral/reviewer-write-boundary.md`) exists because a dispatch claim that can only live on the forge forces every dispatching role, the reviewer included, to hold repository write."
accepted:
  - "The store is chosen in ONE place, `deskkit.ResolveClaimStore(repo)`, from the roster key ASSAY_CLAIM_STORE; no command-line flag and no exported symbol accepts a store choice."
  - "A configured store whose preconditions do not hold is a refusal (exit 6) before any worktree is cut or credential minted; the resolver never moves on to another store."
  - "The two valid values are `file` and `service`. `forge-ref` is not a valid value: set explicitly it is refused like any unknown value. In this brief `file` and `service` parse as valid and resolve to a refusal naming the brief that ships them (forge-neutral/23, forge-neutral/24)."
  - "An UNSET key resolves to the forge-ref store exactly as today, for one release window, with a NOTICE on every run of a dispatching role naming ASSAY_CLAIM_STORE, the two valid values and the release in which the unset key stops resolving. That release is one constant, set when release N is cut; forge-neutral/30 records it and forge-neutral/32 targets it."
  - "Nothing else changes for current installs: an install with no key set keeps its claims on the forge and gains only the NOTICE."
---

**Ruling recorded (2026-09-23): option 1 — approve as specified.** The driver
(`human:<name>`) ruled on the brief's decision-gate issue,
[issue #1552](https://github.com/medici-finance/assay/issues/1552), in the
[ruling comment](https://github.com/medici-finance/assay/issues/1552#issuecomment-5800803636)
(2026-09-23T18:44:36Z, posted by the driver under the driver's own login, never a role App): *"Ruling: option 1 —
approve as specified, and I approve the reviewer-write-boundary spec."* The question was put
as the three options in the brief's `## Human decision`: (1) approve as specified; (2) approve,
but refuse immediately when nothing is configured; (3) reject. The recorded answer is **1**.
The same comment approves `docs/streams/forge-neutral/reviewer-write-boundary.md`, whose
`**Status:**` line reads `approved` from the pull request that lands this record. This record
transcribes that ruling into the register; it does not mint a new one — the human act is the
driver's comment on #1552 and the driver's merge of the pull request that lands this file,
not this file itself.

**The decision.** `docs/streams/forge-neutral/brief-21-claim-store-seam-and-resolver.md` lifts
the claim tool's storage interface into `deskkit` as `ClaimStore`, adds the roster keys
ASSAY_CLAIM_STORE, ASSAY_CLAIM_DIR and ASSAY_CLAIM_SINGLE_HOST with strict parsing, and adds
`ResolveClaimStore(repo)`, which `deskdispatch`'s claim step and the claim tool both ask. The
ruling confirms the rules the brief's `gate-why` puts to the human: the resolver takes no flag,
refuses on every unmet precondition instead of moving to another store, cannot be told to
select the forge store, and leaves existing installs exactly as they are apart from one notice.

**The constraint behind it.** The dispatch claim is the fleet's mutual exclusion. A resolver
that fell back to another store instead of refusing would double-dispatch without failing a
single happy-path test, which is why refusal is the only fallback and why a key set early —
before the store it names ships — fails loudly. The brief's `single-point-of-failure` line
names the resolver's order as the one control; behind it sit each store's own atomic create
and the mixed-store refusal of forge-neutral/23, which detects two live stores for one repo
from the forge side.

**What this record does not decide.** It does not ship the `file` store or the serve mode
(forge-neutral/23, forge-neutral/24), move any claim reader onto the seam (forge-neutral/22),
change any role's duties (forge-neutral/25), or name the removal release (forge-neutral/30
records it when release N is cut). It does not, by itself, prove the approver is a different
identity from the brief's author — the same attribution-not-identity limit `lifecycle-v1.md`
§7.1.2 declares for verification, and `registers-v1.md` §7.4 for this register. And it does
not attest the design is correct: that the alternatives were weighed is recorded here; whether
the resolver refuses where it must is the brief's Verify rows 2-6 and its mutation entry.
