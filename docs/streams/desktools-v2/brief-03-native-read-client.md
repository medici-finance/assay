---
brief: assay:assay:desktools-v2:03
title: native read-path installation-token client — retire gh shell-out in the read path (#1223 pilot)
why: >-
  The reads that still shell gh (or a gh-shim, or a script that shells gh) are where three
  credential bugs live: a HOME override hides the ambient credential (#1145), a token attaches
  only to a child literally named gh (#1146), and an inherited GH_TOKEN forces one installation
  across repos (#628). All three vanish structurally once a read runs a native in-process
  client handed an explicitly-minted installation token — because none of those vectors exists
  without a gh subprocess. This is the stream's first concrete migration and the client every
  later write migration reuses.
wave: 2
depends: ["desktools-v2/01"]
unblocks: ["desktools-v2/04", "desktools-v2/06", "desktools-v2/08"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 1 (CUSTODY) — the framing this brief's contract codifies: explicit minted-token only, key-presence is the custody boundary, desktop-as-locked-container"
  - "docs/streams/desktools-v2/spec.md §5 — the audit reframe: desk verbs ~mostly migrated (5 token-custody exceptions); statusgen's read path is desktools-v2/08"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the exact desk read sites and the 5 gh exceptions this brief dispositions"
  - "tools/desk/internal/deskkit/forge_github.go:16-40,68-72 — GitHubForge already runs on go-gh with an explicitly-minted token and REFUSES an empty one; this is the client foundation the reads route onto"
  - "tools/desk/internal/deskkit/forge.go — the seam op each migrated read should call"
  - "docs/streams/forge-neutral/README.md — forge-neutral owns token MINTING/custody (forge-neutral/01); this brief consumes a minted token and does not mint one"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — forge_github.go's restClient refuses an empty token; the read reach-arounds enumerated by desktools-v2/01 are NOT yet routed through it"
gate-why: >-
  This brief changes HOW a desk read obtains and scopes its GitHub credential — from an
  ambient gh-CLI identity to an explicitly-minted installation token handed to an in-process
  client. A subtle error here (a client that silently falls back to the ambient keyring, a
  token minted for the wrong installation, a read that reaches the wrong repo's installation)
  writes/reads as an identity nobody chose and survives every happy-path test. The human is
  confirming two things: that the client REFUSES an unminted/empty token rather than resolving
  an ambient one, and that the installation the token is minted for is derived from the repo
  being read, never inherited from the environment (#628).
exec-tier: strong
exec-tier-why: >-
  question (c) — credential/identity plumbing where a subtle error (an ambient-fallback path, a
  wrong-installation token) survives the brief's own happy-path tests; and (a) — the fallback-
  refusal boundary is a design decision the facts do not fully pre-specify.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: follow-up desktools-v2/03 (this brief; the native read client + its refuse-if-unminted contract — flips to fixed-here when the implementation lands)"
  - "the read verbs named by the inventory as gh-shelling reads: follow-up desktools-v2/03 (this brief; each routed onto the client and its gh reach-around removed in the same change)"
  - "token minting / installation resolution: out-of-scope (forge-neutral/01 owns minting and custody; this brief consumes a minted token per forge.go's existing contract)"
---

# Brief 03 — native read-path installation-token client (#1223 pilot)

## Context

files:
- `tools/desk/internal/deskkit/forge_github.go` — the existing go-gh-seated GitHub backend;
  the native read client is built here or beside it, reusing its refuse-if-empty contract.
- The read verbs the inventory (`desktools-v2/01`) names as gh-shelling reads — each routed
  onto the native client, its gh reach-around DELETED in the same change (never left dormant).
- NEW `..._test.go` siblings — including the negative-path tests below.

single-point-of-failure: the ONE control this design depends on is the client's
refuse-if-unminted contract — if the client ever resolves an ambient credential, a read runs
as an identity nobody chose. It is backed by a SECOND independent layer that fails for a
different reason in a different component: the installation is derived from the repo argument,
not from the environment, so even a present ambient token cannot redirect the read to the
wrong installation. Refusal (transport floor) and repo-derived installation (identity floor)
catch two different faults — an unminted call and a wrong-installation call — and a negative-
path Verify row exercises each with the other bypassed.

facts:
- This brief codifies **Principle 1 (CUSTODY)** as the client's CONTRACT: (a) explicit
  minted-token only, refuse-if-unminted, never resolve an ambient credential; (b) the
  credential the environment holds and re-mints from is the App PEM + installation id, so
  key presence — not token presence — is the custody boundary the client assumes; (c) the
  client behaves identically on the desktop and in a container (no ambient fallback either
  place), which is what makes the desktop "behave like a locked container".
- The desk read foundation already EXISTS: `forge_github.go` is native and `forgeban` reddens
  a new desk `gh` shell-out. So this brief's desk-side work is (i) codifying the custody
  contract on the native read client and (ii) dispositioning the five sanctioned desk `gh`
  exceptions (each a token-custody decision, listed in the inventory), NOT a wholesale desk
  read rewrite. The higher-value read migration — statusgen — is `desktools-v2/08`, gated on
  the importable library `desktools-v2/07`; this brief does not touch statusgen.
- `forge_github.go`'s `restClient()` constructs the go-gh client with an explicit Host,
  AuthToken and Transport set, which makes go-gh's `optionsNeedResolution` false — it never
  consults gh's ambient keyring/config — and REFUSES an empty token. The native read client
  preserves this exact posture; it does not introduce a new auth path.
- The client is handed an ALREADY-MINTED token (App installation token or PAT). Minting is
  forge-neutral/01's identity layer, deliberately outside this seam (`forge.go` header). This
  brief consumes a minted token; it does not mint one.
- #628: an inherited `GH_TOKEN` must NOT determine the installation — the installation is
  resolved from the repo being read. This brief's client takes the repo coordinate and the
  minted token as explicit inputs; it reads neither from the ambient environment.
- Scope is the READ path only (idempotent, provable against golden outputs, no write risk).
  Write migrations reuse this client but are separate briefs.
- Out of scope: token minting; any write verb; editing forge-neutral's resolver.

## Human decision
This brief changes how a desk read obtains and scopes its GitHub credential: from an ambient
gh-CLI identity to an explicitly-minted installation token handed to an in-process client.
The risk is a read that silently runs as an identity nobody chose — an ambient-keyring
fallback, or a token minted for the wrong installation because an inherited environment token
decided it.

What the human is confirming:
1. The native read client REFUSES an unminted or empty token — it never resolves an ambient
   credential as a fallback.
2. The installation the token is scoped to is derived from the repository being read, never
   inherited from the environment.

Options:
1. **Approve as scoped (read-path only, refuse-if-unminted, repo-derived installation)** —
   the migration proceeds read verb by read verb, each reach-around removed in the same change.
2. **Approve, but fold the client into forge-neutral** — if the approver judges the native
   client belongs with token minting rather than as a v2 consumer of a minted token; the brief
   is then re-homed to forge-neutral and this row closes.
3. **Hold** — the credential-custody model needs more design before any read migrates.

Default if no answer: none — blocks until answered (a credential-custody change does not
proceed on silence).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires removing/weakening the refuse-if-unminted contract or its CI assertion:
  STOP and escalate (needs-decision) — that contract is the security control this brief exists
  to strengthen.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the native read client on `forge_github.go`'s go-gh foundation: it takes a repo
   coordinate and an already-minted token, and reads through `deskkit.Forge` ops. It REFUSES
   an empty/unminted token (mirrors `restClient()`), never resolving an ambient credential.
2. Resolve the installation from the repo coordinate, not from `GH_TOKEN` / the environment.
3. Migrate each read verb the inventory names as a gh-shelling read onto the client, and
   DELETE its gh reach-around in the same change (no dormant fallback).
4. Add negative-path tests, named exactly so the Verify rows target them:
   `TestNativeReadClientRefusesUnmintedToken` (an empty/unminted token is refused, not
   resolved to an ambient identity) and `TestNativeReadClientInstallationFromRepoNotEnv` (an
   ambient `GH_TOKEN`/`HOME` in the environment does not change the installation the read
   targets).
5. Record in the PR body which inventory read rows were migrated and which remain.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/deskkit/` | exit 0; native read-client + negative-path tests pass |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNativeReadClientRefusesUnmintedToken -v` | exit 0; the named test runs (`--- PASS`) and proves an empty/unminted token is REFUSED (not resolved to an ambient identity) — the negative-path row for the transport-floor layer |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestNativeReadClientInstallationFromRepoNotEnv -v` | exit 0; the named test runs (`--- PASS`) and proves an ambient GH_TOKEN/HOME does not redirect the read's installation — the negative-path row for the identity-floor layer |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb3.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb3.txt` | exit 0; prints a count STRICTLY LOWER than brief 02's recorded baseline (the migrated reads' gh reach-arounds are gone — the dereferencing check that the old path was removed, not left dormant) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data: yes — the brief changes how a desk read obtains and scopes its
GitHub credential). MANDATORY human sign-off per the risk answer; the human confirms the two
points in `## Human decision`. Rows 3 and 4 are the two independent negative-path layers
(refuse-if-unminted; repo-derived installation); row 5 dereferences that the reach-arounds
were removed, not fenced. Reviewer records verdict + date in the stream README table.
