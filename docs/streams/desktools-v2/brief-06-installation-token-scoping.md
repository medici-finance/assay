---
brief: assay:assay:desktools-v2:06
title: "installation-token scoping — one token per repo, no ambient-credential hiding (#628 / #1145 / #1146)"
why: >-
  Three credential-custody bugs share one root: the desk tools let the AMBIENT environment
  decide identity. An inherited GH_TOKEN forces one installation across every scan repo (#628);
  cellctl's HOME override hides gh's ambient credential from a shimmed verb (#1145); dispatch
  attaches the token only to a child literally named gh, not to a script that shells gh (#1146).
  Routing every credentialed operation through the native client (desktools-v2/03) with a token
  minted PER the repo it targets removes the ambient-decides-identity class structurally: there
  is no ambient credential to hide, inherit, or mis-scope once the token is an explicit,
  repo-scoped input.
wave: 4
depends: ["desktools-v2/02", "desktools-v2/03"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 1 (CUSTODY) — this brief IS the provisioning half of that principle: key presence is the custody boundary (b), one role's minting key per environment (c), and the desktop made to behave like a locked container"
  - "docs/streams/desktools-v2/spec.md §1 (#628/#1145/#1146 rows), §3 (commitment 4)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the ambient-credential rows (HOME override, GH_TOKEN inheritance, token-attaches-only-to-gh)"
  - "tools/desk/internal/deskkit/forge_github.go:16-40 — the client refuses an empty token and never consults the ambient keyring; this brief makes that the ONLY path"
  - "docs/streams/forge-neutral/README.md — forge-neutral/01 owns minting/custody; this brief scopes the minted token to the repo, it does not change how minting authenticates"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — #628/#1145/#1146 are open; the native client (desktools-v2/03) is the mechanism this brief scopes per-repo; no per-repo installation resolution exists in the shell/shim paths today"
gate-why: >-
  This brief decides which installation a desk operation authenticates as. A subtle error — a
  token minted for the wrong repo's installation, an ambient GH_TOKEN still consulted on one
  path, a shim that still inherits HOME — means an operation runs against a repo as an
  identity nobody scoped to it, and it survives every same-repo happy-path test. The human is
  confirming that identity is decided by the repo being operated on and the roster, NEVER by an
  ambient environment value, on EVERY path (Go, shim, and script), with no remaining fallback.
exec-tier: strong
exec-tier-why: >-
  question (c) — credential/identity plumbing across three surfaces (Go, cellctl shims, dispatch
  scripts) where a subtle miss on any one surface leaves an ambient-decides-identity hole that
  survives happy-path tests; and (b) — correctness depends on cross-artifact reasoning (the same
  no-ambient-identity invariant must hold on all three surfaces at once).
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: follow-up desktools-v2/06 (this brief; per-repo token scoping on the native client — flips to fixed-here when the implementation lands)"
  - "tools/cellctl (gen_shims HOME override, #1145): follow-up desktools-v2/06 (this brief; the shim path stops hiding the ambient credential — flips to fixed-here when the edit lands)"
  - "tools/desk dispatch path (token-attaches-only-to-gh, #1146): follow-up desktools-v2/06 (this brief; the token reaches a script that shells the client, not only a child named gh)"
  - "token minting / authentication: out-of-scope (forge-neutral/01 owns HOW a token is minted; this brief owns WHICH installation it is scoped to)"
version: 1
id: 2808921b-f587-499f-abe8-e87b63dddea6
---

# Brief 06 — installation-token scoping

## Context

files:
- `tools/desk/internal/deskkit/` — per-repo installation-token scoping on the native client
  (desktools-v2/03); the repo coordinate determines the installation, not the environment.
- `tools/cellctl/` — the `gen_shims` `HOME` override that hides the ambient credential (#1145).
- The dispatch path that attaches the token only to a child named `gh` (#1146) — the token
  reaches a script that shells the client, not only a literally-named `gh` child.
- NEW/extended `..._test.go` — including the three negative-path tests below.

single-point-of-failure: the ONE control is "identity is decided by the repo + roster, never
by ambient env". It is backed by TWO independent layers that fail for different reasons in
different components: (1) the native client REFUSES an unminted token and never consults the
ambient keyring (transport floor, desktools-v2/03); (2) the installation is DERIVED from the
repo coordinate, so an inherited `GH_TOKEN` cannot redirect it (identity floor). A third,
out-of-band layer is the ban-lint (desktools-v2/02), which reddens CI if a `gh`-shelling path
that could inherit ambient identity is re-introduced. Three surfaces, three failure signals.

facts:
- **Principle 1 (CUSTODY) made concrete here.** (b) The credential an environment holds and
  re-mints from is the App PEM + installation id, so KEY PRESENCE is the custody boundary —
  scoping decides which installation the PEM mints a token FOR, per operation. (c) The safety
  property is "each environment holds exactly one role's minting key and nothing else"; the
  desktop is the most confused environment (a human ambient token plus possibly several role
  PEMs), and the goal of this brief is to make it behave like a locked container: no path
  falls back to the ambient token, every path mints explicitly from the role's own key scoped
  to the target repo.
- #628: an inherited `GH_TOKEN` must not determine the installation — the installation is
  resolved from the repo being operated on. The token is minted per-repo, not inherited whole.
- #1145: cellctl's `gen_shims` `HOME` override hides `gh`'s ambient credential from a shimmed
  verb. With the native client there is no ambient credential to hide — the shim path hands the
  minted token explicitly, so the `HOME` override no longer changes which identity a verb uses.
- #1146: dispatch attaches the token only to a child literally named `gh`. With the native
  client the token is an explicit input to the operation, so a script that shells the client
  receives it — the "must be named `gh`" coupling is gone.
- Minting (HOW a token authenticates) is forge-neutral/01's; this brief owns WHICH installation
  the minted token is scoped to. It changes no authentication mechanism.
- Every ambient-credential fallback is REMOVED, not left dormant. The invariant "no path
  consults an ambient identity" must hold on all three surfaces (Go, shim, script).
- Out of scope: changing how a token is minted/authenticated; any write-verb behavior beyond
  scoping; editing forge-neutral's resolver.

## Human decision
This brief decides which installation a desk operation authenticates as, across three surfaces
(Go, cellctl shims, dispatch scripts). The risk is an operation that runs against a repo as an
identity nobody scoped to it — because an ambient `GH_TOKEN` was inherited, a `HOME` override
swapped the credential, or a token leaked to the wrong child.

What the human is confirming:
1. Identity is decided by the repository being operated on and the roster — NEVER by an
   ambient environment value — on every path (Go, shim, and script).
2. No ambient-credential fallback remains on any surface: every path hands an explicit,
   repo-scoped, minted token, and refuses when it has none.

Options:
1. **Approve as scoped (per-repo installation, no ambient fallback, all three surfaces)** —
   the migration lands the scoping on the native client and closes the three shim/script holes.
2. **Approve Go + client only, defer the cellctl/dispatch surfaces** — if the approver judges
   the shim/script surfaces (#1145/#1146) need separate review; those rows re-route to a
   follow-up brief and this brief narrows to #628 on the client.
3. **Hold** — the per-repo installation model needs more design before any surface changes.

Default if no answer: none — blocks until answered (a credential-scoping change does not
proceed on silence).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires re-introducing an ambient-credential path or weakening the
  refuse-if-unminted contract: STOP and escalate (needs-decision) — that is the control this
  brief exists to complete.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Scope the native client's token to the repo coordinate: the installation is derived from
   the repo being operated on, never from an inherited `GH_TOKEN` (#628).
2. Change the cellctl `gen_shims` path so the shim hands the minted token explicitly rather
   than relying on an ambient credential a `HOME` override then hides (#1145).
3. Change the dispatch path so the token reaches a script that shells the client, not only a
   child literally named `gh` (#1146).
4. REMOVE every ambient-credential fallback on all three surfaces (no dormant path).
5. Add negative-path tests, named so the Verify rows target them:
   `TestTokenInstallationFromRepoNotInheritedGHToken` (#628),
   the shell suite `tools/cellctl/tests/shim-explicit-token.test.sh` (planned) (#1145 — cellctl is
   a shell script, so its test is a shell suite that prints
   `PASS shim hands explicit token, not ambient` and exits non-zero on failure),
   `TestDispatchHandsTokenToScriptChild` in `tools/desk/cmd/deskdispatch/` (#1146).
6. Record in the PR body which surfaces were migrated and the ban-lint count before/after.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/deskkit/` | exit 0; the scoping + negative-path tests pass |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestTokenInstallationFromRepoNotInheritedGHToken -v` | output contains the literal line `--- PASS: TestTokenInstallationFromRepoNotInheritedGHToken` (assert on that line, not on the exit status — a `-run` selector matching nothing exits 0) — an inherited `GH_TOKEN` does not determine the installation (#628); the identity-floor negative-path row |
| 4 | `sh tools/cellctl/tests/shim-explicit-token.test.sh` (planned) | exit 0 AND output contains the literal line `PASS shim hands explicit token, not ambient`; `tools/cellctl/cellctl` is a POSIX shell script with no Go source, so its test lives beside the other `tools/cellctl/tests/*.test.sh` suites — the shim hands an explicit token, not an ambient credential a `HOME` override hides (#1145). A missing script is exit 127, not a pass |
| 5 | `cd tools/desk && go test ./cmd/deskdispatch/ -run TestDispatchHandsTokenToScriptChild -v` | output contains the literal line `--- PASS: TestDispatchHandsTokenToScriptChild` — assert on that line, NOT on the exit status (`go test -run` with no matching test exits 0); the test lives in `tools/desk/cmd/deskdispatch/`, the package that owns the dispatch path — the token reaches a script that shells the client, not only a child named `gh` (#1146) |
| 6 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb6.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb6.txt` | exit 0; count STRICTLY LOWER than the most recent line in `docs/streams/desktools-v2/forge-ban-baseline.txt` (written by `forge-ban.sh --baseline`, `desktools-v2/02`; the ambient-credential reach-arounds are gone — the removal check) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data: yes — the brief decides which installation a desk operation
authenticates as, across three surfaces). MANDATORY human sign-off per the risk answer; the
human confirms the two points in `## Human decision`. Rows 3-5 are the three independent
negative-path layers (one per surface / issue); row 6 dereferences that the ambient paths were
removed, not fenced. Reviewer records verdict + date in the stream README table.
