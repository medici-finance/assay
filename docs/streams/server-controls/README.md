---
stream: server-controls
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/server-controls/spec.md
---

# server-controls Stream

**Status:** proposed — this stream is PARKED pending a human ruling on its scope
([`spec.md`](spec.md)). Parked reserves the `server-controls` namespace and holds every brief
below off Next-up; nothing here is dispatched until a human flips the scoping doc to `approved`
and this README to `status: active`. The briefs are authored so the plan is reviewable now, not
so it runs now.

Provision the fleet's SERVER-SIDE controls the only way a forge allows — and stop asking for the
way it does not. The recurring request, *"provision fine-grained privileges,"* is impossible:
GitHub and GitLab expose a fixed App/PAT permission menu, a fixed ruleset menu on the protected
ref, and required status checks — nothing arbitrary between them. This stream reframes that ask
into three tractable things ([`spec.md`](spec.md) §0):

1. **Set the coarse ruleset menu UNIFORMLY across every repo.** The menu is fixed; the only lever
   is setting it the same everywhere, so client tools stop compensating for the divergence.
2. **Standardize on rulesets, retire classic branch protection, so controls are READABLE (#1020).**
   #1020 is a read gap — `deskflip` cannot read classic protection (its App lacks
   `administration: read`) — closed by one readable API on every repo, or that grant as interim.
3. **Express any invariant FINER than the menu as a required STATUS CHECK reported by a runner the
   policed party cannot control.** The only fine-grained mechanism the forge gives, already in
   production here as the `leak-sweep` check — trustworthy ONLY under four conditions (non-author
   identity, base-repo execution, protected source, and — where the rule reads input data beyond
   the triggering event — custody of that data), or it degrades to self-attestation.

The honest anti-collusion position ([`spec.md`](spec.md) §1): on this repo author≠approver is
ALREADY enforced (`require_last_push_approval: true`), so #997's original premise is partly stale.
The genuine residual — an independent/second approver, and cross-operator collusion the forge
cannot see — is exactly a required-check job, designed in brief 05.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Uniform-ruleset audit — define the target menu, read each repo's current ruleset state against it](brief-01-uniform-ruleset-audit.md) | 0 | M | todo | — | — |
| 02 | [Standardize on rulesets and retire classic protection so controls are readable (#1020)](brief-02-standardize-rulesets-readable.md) | 1 | M | todo | — | — |
| 03 | [The required-check enforcement pattern — a fine invariant as a required status check reported by a non-author-controllable runner](brief-03-required-check-enforcement-pattern.md) | 1 | M | todo | — | — |
| 04 | [Decision-dependency note — the credential/identity rulings that gate the credential-contract work](brief-04-credential-identity-decision-dependencies.md) | 0 | S | todo | — | — |
| 05 | [Reference cross-operator / independent-approver check — the residual after require_last_push_approval, as a required status check](brief-05-cross-operator-anti-collusion-check.md) | 2 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

```
[human: approve scope + DR-server-controls]
        │
        ▼
   server-controls/01 (uniform-ruleset audit)
        ├────────────► server-controls/02 (standardize / readable, #1020)
        └────────────► server-controls/03 (required-check pattern, gate:human)
                              │
   server-controls/04 (credential/identity deps) ─┐
                              │                    │
                              ▼                    ▼
                       server-controls/05 (cross-operator check, gate:human)
```

**Smallest unblocking move:** the human ruling on this stream's scope and on
`DR-server-controls`. It is the REAL head, not brief 01 — the two `gate: human` design
briefs (03, 05) cannot move to `in-progress` until the design record they cite is `approved`
([`../decisions/README.md`](../decisions/README.md)), and the whole stream is parked until its
scope is approved. Brief 01 (a read-only audit) is the head of the STANDARDIZATION path and can
be done as soon as the stream is un-parked; but sequencing 03/05 behind brief 01 alone, without
the DR ruling, would be putting the wrong item at the head.

**Tempting-but-wrong first step:** authoring the `human-approved` / cross-operator workflow files
or editing a ruleset. Those are admin/human acts (out of scope here); doing them first would build
against a pattern (brief 03) and a residual scoping (brief 05) that have not been human-approved.

## Dependency waves

```
Wave 0: [01, 04]
Wave 1: [02 ←01, 03 ←01]
Wave 2: [05 ←02,03,04]
```

Critical path: `01 → 03 → 05` (and `01 → 02 → 05`), gated at the head by the scope + DR ruling.

## Before starting

- The stream is parked. Un-parking (scope approval) and approving `DR-server-controls` are
  the prerequisites for any `gate: human` brief here.
- Every brief is read/design/spec only. No brief applies a ruleset change, retires classic
  protection, grants a permission, authors a workflow file, or adds a required check to a ruleset —
  those are repo-admin / human acts the briefs END in as provisioning asks.

## Shared conventions

- Bare same-repo issue refs (`#997`, `#1020`, `#900`, `#903`, `#942`).
- The concrete repo set for the audit is read from the operator's configured repo set at run time;
  shipped docs use `example-*` placeholders and name no repo a reader cannot resolve.
