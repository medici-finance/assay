---
stream: spec-routing
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# spec-routing Stream — enforce the spec/scoping-doc lifecycle so an approved plan cannot go unrouted

[`spec/lifecycle-v1.md` §8](../../../spec/lifecycle-v1.md) already defines the *convention*: a
spec-shaped or scoping document opts into a machine-readable three-state lifecycle —
`draft` → `approved` → `routed` — with a `**Routes-to:**` destination and a citation rule that
says a document is `routed` exactly when a brief's `sources:` frontmatter dereferences the
document's repo-relative path. That section exists so the **approved-but-unrouted** condition is
*detectable rather than remembered*: an idea ruled as the plan of record, with no brief ever
authored against it, is the gap the header was designed to close.

This stream implements the *enforcement* half. The convention on its own is inert — a header
nothing parses is a header nothing watches. Two adopter-generic devices close the loop:

- a **linter** that validates the §8 header grammar, requires `**Routes-to:**` on any
  `approved`/`routed` document, and computes the §8.5 **authoring-owed** condition against
  authored brief provenance; and
- an **authoring-owed emitter** — a marker-deduped main-push watcher that files exactly **one**
  work-ready issue per approved-but-uncited document, shipped as reusable tooling plus a
  workflow-template an adopter drops into its own CI.

Both are repo-agnostic and OSS: the linter is a `statusgen` capability and the emitter is a
`statusgen` emit-mode plus a workflow-template. The house wiring — which App files the issue, and
the adopter's own downstream consumption of the routed edge — is *configuration* supplied against
these devices, not code in this stream.

## The convention this stream enforces, in one line per state

| §8 state | Machine meaning | The device that watches it |
|---|---|---|
| `draft` | not yet ruled; the plan of record for nothing | nothing watches a `draft` (by design) |
| `approved` | ruled and merged; brief-authoring is **owed** | the linter (Routes-to present?) + the owed-detector + the emitter |
| `routed` | at least one brief's `sources:` dereferences this path | the linter closes the owed condition; the emitter files nothing |

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Enforce the §8 lifecycle — the linter and the authoring-owed emitter](./brief-01-enforce-lifecycle-and-owed-emitter.md) | 0 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #410 @ 7bb86b33d1a81d605b1ef799f1d3352ef9572752) |

## Critical path

```
01 (linter + authoring-owed emitter)
```

**Critical path: `01`.** The head is **01**, and it is the whole stream today. The convention it
enforces is already on `main` (`spec/lifecycle-v1.md` §8), so this stream has **no external
prerequisite**: the authority exists, and the enforcement is the only open edge. If §8 is
materially reshaped, re-scope this brief rather than implementing against a superseded grammar.

## Dependency waves

```
Wave 0: [01]
```

## Shared conventions (inherited by every brief)

- **Plan-only authoring PR.** The brief in this stream describes work to be IMPLEMENTED later. The
  authoring pull request that files it touches no enforcement code, cuts no tag, and changes no
  lint. The brief's `todo` status is honest: the plan is filed; the enforcement is not yet built.
- **One home.** The linter and the emit-mode land under `statusgen/`; the emitter's
  workflow-template lands under the repo's workflow-template surface. Consumers of the published
  binary pick each change up on the next release and version-pin bump.
- **Verify rows run from the repo root**, are written repo-relative, and are executed from the root
  of a checkout of this repo. A row that cannot resolve its target is **could-not-check**, not a
  pass — record it that way in Evidence rather than marking the row green.
- **Verify rows were dereferenced at authoring time.** The brief carries rows that grep the CURRENT
  source for the ABSENCE of the enforcement it proposes; each was run against `origin/main` @
  `814c0cb` on 2026-08-30. Those rows are expected to **invert** at implementation — a row that
  still reports absence after the work lands means the work did not land.
- **Three states, always.** Every new check reports PASS / PROBLEM / could-not-check. A check that
  cannot establish its condition — no git dir, an unreadable tree, an unparseable header — says so
  and refuses; it never waves through. Fail-open under error is a warning wearing a control's label.
- **Rule-tag every emitted message.** New PROBLEM/NOTICE lines carry a stable `[rule-tag]` bracket
  token, the convention `statusgen/lintaudit.go` already extracts, so the rule is visible to the
  firing audit.

## Sources

- [`spec/lifecycle-v1.md` §8](../../../spec/lifecycle-v1.md) — the normative convention: the
  `**Status:**` header grammar (§8.1), state meanings (§8.2), `**Routes-to:**` (§8.3), the flip
  rules (§8.4), the citation rule and the owed condition (§8.5), and the safe default (§8.6). This
  is on `main`, so the authority this stream enforces already exists.
- The marker-deduped issue-emitter family already in `statusgen/` (`decisionissues.go` and the
  drive-issues mode): a per-item hidden `<!-- … -->` idempotency marker, an existing-markers file
  read from open issues, and a JSON payload a workflow feeds to `gh issue create`. The
  authoring-owed emitter is a new member of this family, not a new mechanism.
- The design has a **first instantiation** already running against an approved-spec corpus in a
  private sibling tree; this stream de-houses the adopter-generic device, not that instance, and
  names none of its internals.
- Freshness: `origin/main` read 2026-08-30 @ `814c0cb`. Every seam named in this stream, and every
  DEREFERENCE row, was checked against that commit.
