---
brief: assay:assay:measured-status:06
title: "model-floor: derive dispatch authority from the dispatch stamp, and give a first-class re-stamp path — stop refusing verdicts by guessing at a label actor's login"
why: >-
  The model-floor decides whether a PR's dispatched-* label counts as attestation by looking at
  who APPLIED the label. A label applied by a trusted LOGIN rather than a trusted dispatcher
  slug reads Indeterminate and the floor refuses to post a verdict, silently stranding real,
  legitimate draft PRs. Authority should be DERIVED from a verifiable stamp the dispatcher
  applies as its own bound identity — and where a legacy label lacks that stamp, there should be
  a sanctioned re-stamp path, not a reader's guess about a login.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [336]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
design: DR-independence-gate
gate-why: >-
  This changes what the model-floor accepts as dispatch authority — a trust-boundary control. A
  human confirms the derivation (authority read from the stamp, could-not-derive fails closed)
  and rules on whether a trusted-login label is honored at all or only re-stamped through the
  sanctioned path, since widening accepted authority is a security-relevant decision.
decision-trigger: creation
sources:
  - "#336 — model-floor refuses verdicts on PRs whose dispatched-* labels were applied by a trusted login, not a trusted dispatcher slug"
  - "tools/desk/internal/deskkit/modelstamp.go — DispatcherRoles / IsDispatcherLogin: the roster-derived set of actors whose stamp counts; an unbound actor reads Indeterminate (fail-closed)"
  - "tools/desk/internal/deskkit/modelstampactor_test.go — the actor-check tests"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — IsDispatcherLogin returns false for a trusted login that is not a dispatcher slug, so a legacy label reads Indeterminate->refuse"
exec-tier: strong
exec-tier-why: changes the trust-boundary the reviewer verdict lane depends on; too loose admits an unattested tier, too strict keeps the backlog stranded
domain: complicated
value: high
version: 1
id: 59e98b33-3177-47e6-94e9-70ca852738a4
---

# Brief 06 — Derive dispatch authority from the stamp, plus a re-stamp path

## Context
files:
- `tools/desk/internal/deskkit/modelstamp.go` — `DispatcherRoles`, `IsDispatcherLogin`,
  `AttestedModelStampOf` and the Indeterminate->refuse actor check.
- `tools/desk/internal/deskkit/modelstampactor_test.go` — the actor-check tests.
facts:
- today the floor trusts a stamp only when the label ACTOR's login is a bound dispatcher slug
  (`example-worker-app`, `example-reviewer-app`); a trusted human LOGIN (`example-dispatch-login`)
  applying the same label reads Indeterminate and the verdict is refused.
- the durable fix is a first-class re-stamp/migration verb that re-applies the label under a
  bound dispatcher slug (keeping the floor's derivation intact), NOT relaxing the floor to trust
  any login — honoring a login is at most an interim, human-authorized operator action.
- fail-closed is the invariant: an authority that cannot be derived from a verifiable stamp
  reads could-not-verify and is refused, never rounded up to trust (an unconfigured roster
  vouches for nobody).
- `tools/desk` is its own Go module; tests run from `tools/desk/`.

## Human decision
When work is dispatched to an automated reviewer, the system attaches a label recording which
model tier was chosen. A safety check later refuses to accept a review verdict unless it can
confirm that label came through the sanctioned dispatch path — and today it confirms this by
checking whether the account that attached the label is one of the system's own dispatcher
identities. Some real, legitimate items in the backlog had their label attached by a trusted
human account instead, which the check cannot recognize, so their verdicts are refused and they
cannot progress — even though the work is genuine. This asks how to let those items through
without weakening the check for everyone.

Options:
1. **Add a sanctioned re-stamp path (recommended)** — provide a first-class way to re-attach the
   label under a proper dispatcher identity, so the safety check keeps deriving authority from a
   verifiable stamp exactly as before, and the backlog is cleared by re-stamping rather than by
   trusting a human account. Consequence: a one-time operation on the affected items; the check
   itself is unchanged and stays strict.
2. **Also accept labels attached by a trusted human account** — widen what the check counts as
   authority to include trusted human accounts, not only dispatcher identities. Consequence:
   the backlog clears with no per-item action, but the set of accounts that can vouch for a tier
   grows, which is a security-relevant loosening. **`design: DR-independence-gate`'s second
   alternative already weighs and rules this out as the primary control** — the record calls
   honoring a trusted login "at most an interim operator action a human authorizes, recorded,
   not a code default." Choosing this option as a standing code default AMENDS that record and
   must update `DR-independence-gate` in the same motion, not leave the two disagreeing.
3. **Both** — clear the current backlog with a one-time, explicitly human-authorized acceptance,
   and add the re-stamp path as the durable mechanism so the loosening is not permanent. **This
   is the shape `DR-independence-gate` itself describes as sanctioned** — an interim,
   human-authorized, RECORDED operator action, never a silent code default — so choosing it is
   consistent with the record as written; no amendment needed as long as the acceptance is a
   recorded, one-time act rather than a standing widened check.

Default if no answer: none — blocks until answered (this changes a trust boundary and must not be defaulted).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the ruling. For the recommended path: add a re-stamp verb/function that re-applies
   a `dispatched-*` label under a bound dispatcher slug, and keep `IsDispatcherLogin` /
   the actor check deriving authority from the stamp unchanged. If the ruling widens accepted
   authority, gate that behind an explicit, roster-configured allowance — never a silent default.
2. Preserve fail-closed: an authority that cannot be derived reads could-not-verify and is
   refused; an unconfigured roster vouches for nobody.
3. Cover both cases in `modelstampactor_test.go`: a properly-stamped label is accepted, and an
   un-derivable actor is refused (and, if a re-stamp verb is added, a re-stamped label is then accepted).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp' -count=1` | exit 0; output contains "ok" |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp.*Actor' -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (the accepted-stamp and refused-actor cases both ran and passed) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -count=1` | exit 0 (the whole deskkit suite is green) |
| 4 | `cd tools/desk && go vet ./internal/deskkit/` | exit 0 |

## Evidence
<!-- appended at implementation time by a non-implementer -->

## Review
Gate: human (from frontmatter). Human gate is MANDATORY — this changes a trust boundary.
Reviewer records verdict + date in the stream README table.
