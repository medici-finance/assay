---
brief: assay:assay:desktools-v2:08
title: migrate statusgen's forge reads onto the shared Forge under minted-token custody
why: >-
  statusgen shells gh directly across its scan, auto-flip and autonomy reads with its own
  ambient-custody forge access and no native client — the scanloop-blind root: scanloop runs
  `statusgen --scan-issues`, which shells gh, so in a container with no ambient gh credential
  it reads an empty queue and cannot tell empty from blind (#628). Routing statusgen's reads
  through the shared Forge library (desktools-v2/07) under an explicitly-minted installation
  token retires the ambient custody, fixes the container break, and makes statusgen obey the
  same custody invariant as the desk verbs. This is the higher-value half of "retire gh in the
  read path".
wave: 3
depends: ["desktools-v2/07", "desktools-v2/03"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 1 (CUSTODY) and Principle 2 (read path covers statusgen) — the framing this brief lands"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the statusgen read sites this brief migrates"
  - "desk audit on the stream PR (2026-09-16) — statusgen scanissues.go:114,:870 / issues.go:577 / autoflip.go:535,:615,:656,:666 / autonomy.go:451,:479, all `exec.Command(\"gh\"`; the scanloop-blind root"
  - "the shared Forge library (desktools-v2/07) — what statusgen imports; the native read client + custody contract (desktools-v2/03) — the posture statusgen adopts"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the nine named statusgen gh read sites are present on origin/main and statusgen is NOT under forgeban; verified statusgen is a separate module from tools/desk"
gate-why: >-
  This brief changes statusgen's forge credential custody from an ambient gh CLI identity to an
  explicitly-minted installation token, on the path scanloop depends on. A subtle error — a
  read that silently keeps an ambient fallback, a token minted for the wrong installation, a
  scan that reads a different repo's issues than it reports — makes statusgen report a board
  that is not the one it authenticated against, and it survives every ambient-credential-present
  happy-path test. The human is confirming that statusgen's reads REFUSE an unminted token and
  never fall back to an ambient credential, and that the installation is derived from the repo
  being scanned, on every migrated read.
exec-tier: strong
exec-tier-why: >-
  question (c) — credential/identity plumbing on the scanloop-critical path where a subtle
  ambient-fallback survives happy-path tests; and (b) — cross-module correctness (statusgen
  importing and consuming the shared library correctly across a module boundary).
domain: complicated
consumers:
  - "statusgen/scanissues.go, statusgen/issues.go, statusgen/autoflip.go, statusgen/autonomy.go: follow-up desktools-v2/08 (this brief; each gh read replaced by a shared-Forge call under minted custody and the gh shell-out DELETED — flips to fixed-here when the implementation lands)"
  - "statusgen/go.mod (adds the shared Forge library as a require): follow-up desktools-v2/08 (this brief; the module dependency is added here)"
  - "scanloop (the consumer that broke in a container): out-of-scope (this brief fixes the root in statusgen; scanloop needs no change once statusgen reads via minted token)"
  - "token minting: out-of-scope (forge-neutral owns minting; statusgen consumes a minted token, does not mint)"
version: 1
id: f3b3a317-1951-49e3-a20c-71ce5679de59
---

# Brief 08 — migrate statusgen's forge reads onto the shared Forge

## Context

files:
- `statusgen/scanissues.go` (`:114`, `:870`), `statusgen/issues.go` (`:577`),
  `statusgen/autoflip.go` (`:535,:615,:656,:666`), `statusgen/autonomy.go` (`:451,:479`) —
  the `gh` read sites replaced by shared-Forge calls under minted custody; each `gh` shell-out
  DELETED in the same change.
- `statusgen/go.mod` — adds the shared Forge library (`desktools-v2/07`) as a require.
- NEW/extended `_test.go` — the negative-path custody test below.
- A full sweep of `statusgen/**` for any remaining `exec.Command("gh"` read beyond the nine
  named sites (the inventory lists them); a residual write/custody-gated site routes to a
  follow-up, a residual READ is in scope here.

single-point-of-failure: the control is statusgen's reads refusing an unminted token and never
falling back to ambient — inherited from `desktools-v2/03`'s custody contract. The independent
backing layer that fails on a different signal in a different component: the ban-lint
(`desktools-v2/02`), which — extended to `statusgen/**` by that brief — reddens CI if a `gh`
read is re-introduced, catching in the build what a runtime test might miss. Two signals: a
runtime custody test and a CI grep.

facts:
- Principle 2: statusgen is a separate module; it consumes the Forge only because
  `desktools-v2/07` made it importable. This brief is a PORT (statusgen gains a forge client it
  never had), not a call-site swap.
- Principle 1: statusgen's reads adopt `desktools-v2/03`'s custody contract — explicit
  minted-token, refuse-if-unminted, installation derived from the repo scanned, no ambient
  fallback. This is what fixes the container break: a container with no ambient `gh` credential
  but with the role's minting key reads correctly instead of reading empty.
- #628 is the concrete failure: `scanloop` → `statusgen --scan-issues` → `gh` → empty in a
  container. The fix is in statusgen; `scanloop` needs no change.
- The nine named sites are the read path; `deskmerge`-style writes are not in this brief.
- Out of scope: token minting; changing what the scan REPORTS (behaviour-neutral except for
  custody); the access-pattern query layer (that is `desktools-v2/09`, though statusgen's scan
  is a prime later consumer of it).

## Human decision
This brief changes statusgen's forge credential custody from an ambient `gh` CLI identity to an
explicitly-minted installation token, on the `scanloop`-critical read path. The risk is a
statusgen that reports a board it did not authenticate against — an ambient fallback left on
one read, or a token scoped to the wrong installation — which passes every test where an
ambient credential happens to be present.

What the human is confirming:
1. Every migrated statusgen read REFUSES an unminted/empty token and never falls back to an
   ambient credential (the same contract as `desktools-v2/03`).
2. The installation each read authenticates as is derived from the repository being scanned,
   never inherited from the environment.

Options:
1. **Approve as scoped (all nine named reads + any residual read, minted custody, gh deleted)**
   — statusgen reads via the shared Forge; `scanloop` unblocks in a container.
2. **Approve the scan/issues path only (`--scan-issues`), defer auto-flip/autonomy** — if the
   approver wants the container-break fix landed first and the flip/merge reads reviewed
   separately; those sites re-route to a follow-up.
3. **Hold** — the shared-library import or the custody model needs more design first.

Default if no answer: none — blocks until answered (a credential-custody change does not
proceed on silence).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires keeping an ambient-credential fallback or weakening the
  refuse-if-unminted contract: STOP and escalate (needs-decision) — that contract is the point.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the shared Forge library (`desktools-v2/07`) to `statusgen/go.mod` and construct a
   Forge in statusgen handed an explicitly-minted installation token (refuse-if-unminted).
2. Replace each of the nine named `gh` reads (and any residual `statusgen/**` gh READ the
   inventory names) with a shared-Forge call; DELETE the `gh` shell-out in the same change.
3. Derive the installation from the repo being scanned, never from an ambient `GH_TOKEN`.
4. Add a negative-path test `TestStatusgenScanRefusesAmbientRunsMinted` proving the scan reads
   under a minted token and refuses when unminted, with no ambient fallback.
5. Record in the PR body the before/after gh-read count in `statusgen/**` and confirm
   `scanloop`'s container path now reads a non-empty queue with only a minting key present.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go build ./... && go vet ./...` | exit 0 (statusgen builds importing the shared Forge library) |
| 2 | `cd statusgen && go test ./...` | exit 0; the custody + scan tests pass |
| 3 | `grep -n 'exec.Command("gh"' statusgen/scanissues.go statusgen/issues.go statusgen/autoflip.go statusgen/autonomy.go` | exit 1 (no match) — the nine named `gh` read shell-outs are GONE from statusgen's read path (the removal, not just the addition) |
| 4 | `cd statusgen && go test ./... -run TestStatusgenScanRefusesAmbientRunsMinted -v` | exit 0; the named test runs (`--- PASS`) proving the scan reads under a minted token and refuses unminted with no ambient fallback — the negative-path custody row |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb8.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb8.txt` | exit 0; count STRICTLY LOWER than the pre-brief value in the PR body — the statusgen reach-past sites (now covered by the ban-lint per brief 02) are gone |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data: yes — statusgen's forge credential custody changes from ambient to
minted on the scanloop-critical path). MANDATORY human sign-off; the human confirms the two
points in `## Human decision`. Row 3 is the removal check (gh gone from the named read path);
row 4 is the negative-path custody row; row 5 confirms the ban-lint (extended to statusgen)
count dropped. Reviewer records verdict + date in the stream README table.
