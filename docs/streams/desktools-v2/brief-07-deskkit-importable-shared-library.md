---
brief: assay:assay:desktools-v2:07
title: promote deskkit's Forge to an importable shared library (the built-properly enabler)
why: >-
  statusgen — the highest-value read-path target — cannot consume the Forge seam because
  deskkit lives under tools/desk/internal/ in a SEPARATE Go module, and an internal package of
  another module is unimportable by construction. So "retire gh in the read path" is blocked
  on packaging, not on transport: the shared library has to become importable before statusgen
  (or any second module) can route through it. This is the "built properly" foundation the
  refined scope names — expose the existing, golden-pinned Forge interface + backends as a
  shared library, changing no wire behaviour, so the migrations that follow have something to
  import.
wave: 2
depends: ["desktools-v2/01"]
unblocks: ["desktools-v2/08", "desktools-v2/09"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 2 (read path covers statusgen) and §3 (commitment 3) — why importability is a prerequisite, not a nicety"
  - "docs/streams/desktools-v2/spec.md §7 open question 2 — the module-topology choice this brief proposes and defers to the approver"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go — the interface + two backends being exposed; the golden corpus (forge_github_golden_test.go) pins wire behaviour unchanged"
  - "tools/desk/internal/deskkit — the current home: an `internal/` package of module github.com/medici-finance/assay/tools/desk"
  - "statusgen/go.mod (module github.com/medici-finance/assay/statusgen) — the second module that must be able to import the result; separate module from tools/desk today"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — deskkit is under tools/desk/internal/ (unimportable cross-module); statusgen is a distinct module with no dependency on tools/desk; the go-gh-seated Forge already refuses an empty token"
consumers:
  - "tools/desk/internal/deskkit (the Forge interface + backends relocated to an importable path): follow-up desktools-v2/07 (this brief; flips to fixed-here when the relocation lands)"
  - "tools/desk cmd packages that import deskkit's Forge today: follow-up desktools-v2/07 (this brief; their import paths update in the same change — a shared-value/module-path change)"
  - "statusgen (the first external consumer): out-of-scope (this brief only makes the library importable; statusgen imports and consumes it in desktools-v2/08, which touches no path this brief edits)"
  - "token minting / identity layer: out-of-scope (stays in forge-neutral; this brief moves no minting code, only the Forge interface + backends)"
exec-tier: strong
exec-tier-why: >-
  question (b) — correctness depends on cross-module reasoning: the relocation must keep every
  existing tools/desk importer building AND make the package importable by a second module,
  with the golden wire behaviour byte-identical; a subtle packaging error (a leaked internal
  dependency, a moved symbol that changes an exported surface) survives a single-module test.
domain: complicated
version: 1
id: 097a1652-5156-4ff9-bbb3-5dda837c986c
---

# Brief 07 — promote deskkit's Forge to an importable shared library

## Context

files:
- `tools/desk/internal/deskkit/forge.go`, `forge_github.go`, `forge_gitlab.go` (+ the golden
  corpus `forge_github_golden_test.go`) — the interface + two backends relocated to an
  IMPORTABLE package (out of `internal/`), the minimal Forge surface a second module needs.
- The `tools/desk` cmd packages that import the Forge today — their import paths update in the
  same change.
- NEW a tiny second-module import smoke test (or the statusgen go.mod require edit staged for
  `desktools-v2/08`) proving the package is importable from OUTSIDE `tools/desk`.

single-point-of-failure: none of the credential kind — this is a packaging move with no
identity or wire change. The control that matters is that the move is BEHAVIOUR-NEUTRAL: the
golden corpus (which pins request method/path/query/pagination/error-mapping per operation)
is the independent layer that fails, in a different component than a build error, if the
relocation changed anything observable at the wire. Build-green proves it compiles; the
goldens prove it did not silently change behaviour — two different signals.

facts:
- deskkit's Forge is at `tools/desk/internal/deskkit/`. `internal/` means only
  `tools/desk/...` may import it; a separate module (statusgen) cannot, at all. That is the
  whole blocker — not transport.
- The minimal importable surface is the `Forge` interface, the two backend constructors, and
  the value types the interface signatures name (`ForgeRepo`, `PullRequest`, `Issue`, …).
  Token minting, budgets, secret-scan and rate wrappers stay where they are (identity layer /
  tool-side); they are NOT part of the exported library (spec §2 header contract).
- This brief changes NO wire behaviour: the go-gh-seated GitHub backend and the GitLab backend
  move as-is, pinned by the golden corpus. It is a relocation + import-path update, not a
  rewrite.
- Module topology is the approver's call (spec §7 q2): a new shared module
  (e.g. `.../forgekit`) vs a shared non-internal package in a restructured layout. This brief
  proposes the minimal-surface option and records the decision; it does not pre-empt the
  ruling.
- Out of scope: migrating any consumer's calls onto new ops (that is 08/09); moving minting;
  changing any query.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state (especially the module-topology decision):
  report NEEDS_CONTEXT, don't guess.

## Task
1. Relocate the `Forge` interface + the two backends + their signature value types out of
   `tools/desk/internal/deskkit/` to an importable package (topology per the approver's
   ruling; propose the minimal-surface shared module in the PR body).
2. Update every `tools/desk` cmd/package import of the Forge to the new path in the same change.
3. Keep the golden corpus with the backends and prove it still passes byte-identical.
4. Add a second-module import smoke test (a throwaway `package main` under a distinct module,
   or the staged statusgen require) proving the package imports from outside `tools/desk`.
5. Record the chosen topology and the moved symbol set in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 (every existing importer still builds after the relocation) |
| 2 | `cd tools/desk && go test ./... -run Golden` | exit 0; the forge golden corpus passes byte-identical — the relocation changed nothing at the wire |
| 3 | `bash -c 'grep -rlE "deskkit\".*Forge|/deskkit\"" tools/desk/cmd | head -1 >/dev/null; echo checked'` | exit 0; prints `checked` (the importer sweep ran; the PR body lists the updated import paths) |
| 4 | `cd tools/desk && go test ./... -run TestForgeImportableFromSecondModule -v` | exit 0; the named smoke test runs (`--- PASS`) proving the Forge package is importable from OUTSIDE tools/desk — the dereferencing row for "made importable" |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb7.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb7.txt` | exit 0; count EQUAL to brief 02's baseline (a pure relocation adds and removes no reach-past site — the neutrality check) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a behaviour-neutral packaging relocation pinned by the
golden corpus; no identity binding, no capability removed, no wire change). Row 2 is the
neutrality proof (goldens byte-identical); row 4 dereferences the actual deliverable
("importable from a second module"); row 5 confirms the ban-lint count is unchanged. The
module-topology decision is surfaced to the approver (spec §7 q2), not decided here. Reviewer
records verdict + date in the stream README table.
