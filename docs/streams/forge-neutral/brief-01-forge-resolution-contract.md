---
brief: assay:assay:forge-neutral:01
title: Forge resolution contract — the forge comes from repo config, and refusal is the only fallback
why: >-
  Two complete forge backends exist and nothing in the fleet can obtain one: no constructor,
  no resolver, no config key. Until a verb can ask "which forge serves this repo?" and get an
  answer, every later brief has nothing to bind to. This delivers that one answer — sourced
  from repo configuration rather than from a flag a session can set — plus the custody binding
  that decides WHICH identity performs the write, and the refusal a verb gives when the
  configured forge cannot serve an operation.
wave: 1
depends: []
unblocks: ["forge-neutral/02", "forge-neutral/03", "forge-neutral/04", "forge-neutral/05", "forge-neutral/06", "forge-neutral/07", "forge-neutral/08", "forge-neutral/09", "forge-neutral/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/README.md — the measured matrix this brief's head finding comes from"
  - "docs/streams/forge-gitlab/pilot-report.md §2 and D-1 — every pilot write was a hand-built curl because no verb had a GitLab path"
  - "docs/streams/forge-gitlab/spec.md §6 (interface scope, freeze rule), §5 (token custody)"
  - "tools/desk/internal/forgeban/allowlist.go:21-40 — the two blocker shapes (identity, no-op) this contract has to answer"
  - "freshness-checked 2026-09-02 @ deae247 — `grep -rn 'GitHubForge{\\|GitLabForge{' tools/ --include='*.go' | grep -v _test.go` returns 0; no New*Forge/ForgeFor/ResolveForge constructor exists in deskkit; no ASSAY_FORGE anywhere; the only --forge flag is desktoken's custody switch (tools/desk/cmd/desktoken/desktoken.go:454)"
exec-tier: strong
exec-tier-why: "the deliverable decides which identity performs a write and what happens when a forge cannot serve an operation — a subtle error (a fallback that silently reaches GitHub, a resolver that accepts a session-supplied value) survives every happy-path test (questions a and c)."
gate-why: >-
  This brief binds token custody to forge selection: it decides which minted credential a verb
  hands to a backend, and it is the single place a wrong answer means a verb writes as an
  identity nobody chose. The human is confirming two things — that the forge value cannot be
  supplied by the session (only by repo configuration and the roster), and that the
  unsupported-operation path refuses rather than degrading to a raw request or to the other
  forge's behavior.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the resolver, the config key, the custody binding)"
  - "tools/desk/cmd/deskpost: fixed-here (the one verb wired end-to-end as proof the resolver is reachable)"
  - "tools/desk/cmd/deskpr, deskreply, deskflip: follow-up forge-neutral/03"
  - "tools/desk/cmd/deskfile, deskclose, deskevidence: follow-up forge-neutral/04"
  - "tools/desk/cmd/deskboard, issueboard, scanloop: follow-up forge-neutral/06"
  - "tools/desk/cmd/desktoken: fixed-here (its --forge flag becomes the custody path selector the resolver drives, not an independent switch)"
  - "statusgen: follow-up forge-neutral/08 (statusgen resolves its own forge; it does not import deskkit)"
version: 1
id: 38a45459-d392-4a7c-a84e-8bf599ed0f75
---

# Brief 01 — Forge resolution contract

## Context
files:
- `tools/desk/internal/deskkit/forgeresolve.go` (planned) — the resolver, the config key, and
  the refusal type.
- `tools/desk/internal/deskkit/forgeresolve_test.go` (planned) — including the negative-path
  tests below.
- `tools/desk/internal/deskkit/rosterconfig.go` — the new config key registers here (an
  unregistered `ASSAY_*` key fails the roster closed).
- `tools/desk/cmd/deskpost/github.go` — the one verb wired through the resolver in this brief.

single-point-of-failure: the resolver is the ONE place a backend is constructed, so the
control that matters is that no other construction site can exist. It is backed by a second,
independent layer that fails for a different reason in a different component — the existing
`forge-surface-control.yml` CI job, whose no-passthrough shape check and permit-register
ratchet already refuse a new forge-CLI call site or an exported raw-request method. A source
test that finds a stray `GitHubForge{}` literal and a CI control that finds a new shell-out
catch two different ways of going around the resolver.

facts:
- Both backends are complete and both refuse an empty token: `forge_github.go` `restClient()`
  at `tools/desk/internal/deskkit/forge_github.go:68-72`, `forge_gitlab.go` `client()` at
  `tools/desk/internal/deskkit/forge_gitlab.go:108-112`. Neither resolves an ambient CLI
  credential.
- Token minting is DELIBERATELY outside the interface: *"a `Forge` is handed an
  already-minted token; it never mints one"* — `tools/desk/internal/deskkit/forge.go:10-17`.
  So the resolver's job includes deciding which minted token file a verb hands over, per
  forge and per role.
- Custody paths differ per forge and both already exist: GitHub mints an App installation
  token (`tools/desk/cmd/desktoken/desktoken.go:147,252`); GitLab rotates a PAT in place
  (`tools/desk/cmd/desktoken/gitlab.go:83,163`), reading `GITLAB_API_BASE` with no fallback
  (`gitlab.go:54-55`).
- The interface is FROZEN at 15 operations; adding one requires a consuming tool in the same
  change (`tools/desk/internal/deskkit/forge.go:169-172`).
- `deskkit.Refused` and `deskkit.Unverifiable` are the existing refusal constructors; exit
  codes are `5 refused` / `6 unverifiable` (`tools/desk/internal/deskkit/exitcodes.go`).
- `GitLabForge.DeleteRef` is the reference shape for an unsupported operation: it returns
  `Unverifiable` naming the gap and stating the claim is *"NOT reported released"*
  (`tools/desk/internal/deskkit/forge_gitlab.go:1227-1240`).
- A new `ASSAY_*` key must register in the roster's known-set or a set value fails the fleet
  closed (`tools/desk/internal/deskkit/rosterconfig.go:728,762`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not add a generic/passthrough method, a new `gh`/`glab` shell-out, or a new interface
  operation. `.github/workflows/forge-surface-control.yml` must stay green unchanged.

## Task
1. **The resolver.** Add `deskkit.ForgeFor(repo ForgeRepo) (Forge, error)` in
   `forgeresolve.go`. It is the ONLY function in the tree that constructs a backend.
   Resolution order, each step recorded in the returned value so a caller can report it:
   a. the repo's configured forge, read from repo configuration (the roster's per-repo forge
      binding, registered as a new key in `rosterconfig.go`'s known-set);
   b. if absent, the origin remote's host, mapped to a forge only when the mapping is
      unambiguous;
   c. otherwise a `deskkit.Unverifiable` could-not-check naming the repo and saying which
      configuration would resolve it.
   **There is no parameter, flag, or environment variable by which a caller supplies the
   forge.** A session must not be able to answer this question; the repo's configuration
   answers it.
2. **The custody binding.** `ForgeFor` obtains the role's minted token for the resolved
   forge from the existing custody path (GitHub: the App installation token; GitLab: the
   rotated PAT file) and hands it to the backend. It never mints; it never falls back to an
   ambient credential; a missing or wrong-mode token file is a `Refused` naming the remedy,
   exactly as the existing custody refusals do. `desktoken`'s `--forge` stops being an
   independent switch and becomes the custody path the resolver names.
3. **Refusal semantics.** Specify and implement one rule: when the resolved forge cannot
   serve an operation, the verb returns could-not-check (`Unverifiable`, exit 6) naming the
   forge, the operation and the gap — never a silent success, never the other forge's
   behavior, never a raw request. Document the rule in `forgeresolve.go`'s header as the
   contract every later brief inherits.
4. **The single-construction-site control.** Add a test asserting that no non-test file
   outside `forgeresolve.go` constructs `GitHubForge` or `GitLabForge` (composite literal or
   `&`-address), and that no exported constructor for either type exists. This is what stops
   a later brief re-opening the seam by hand.
5. **Wire one verb.** Route `deskpost`'s forge operations through `ForgeFor` — it is the
   `net/http` verb with the fullest identity story (App installation mint at
   `tools/desk/cmd/deskpost/github.go:185`, reviewer login at `:52-57`), so it exercises custody as well
   as transport. Its existing tests stay green unmodified. `deskpost` has no `forgeban`
   permit row, so this step is a pure proof-of-reachability and moves the ratchet by zero —
   which is the point: the resolver must be provably live before any ratchet claim rests on
   it.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSingleConstructionSite -count=1 -v` | exit 0; output contains `PASS` — no backend literal outside the resolver |
| 3 | `grep -rn -e 'GitHubForge{' -e 'GitLabForge{' -e '&GitHubForge' -e '&GitLabForge' tools/desk --include='*.go' \| grep -v _test.go \| grep -cv 'forgeresolve.go' \|\| true` | prints `0` — independent cross-check of row 2 by a different instrument (the `\|\| true` neutralises grep's exit-1-on-no-match so the success path does not read as a failure) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForRejectsCallerSuppliedForge -count=1 -v` | exit 0; the test asserts `ForgeFor`'s signature takes no forge argument and that no exported symbol in `deskkit` accepts a forge name from a caller |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForUnconfiguredRepoRefuses -count=1 -v` | **negative path**: a repo with no configured forge and an unrecognisable remote yields `Unverifiable` (exit-code class 6) whose message names the repo and the configuration that would resolve it; it does NOT return a GitHub backend |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForMissingTokenRefuses -count=1 -v` | **negative path**: with the role's token file absent, and with it present at mode 0644, `ForgeFor` returns `Refused` naming the remedy and constructs no backend; no ambient credential is read |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestUnsupportedOperationIsCouldNotCheck -count=1 -v` | **negative path**: an operation the resolved backend cannot serve returns `Unverifiable` naming forge+operation+gap; the test asserts the call performed no request against the other forge and returned no zero-value success |
| 8 | `cd tools/desk && go test ./cmd/deskpost/... -count=1` | exit 0 — `deskpost`'s existing suite green unmodified with its forge ops on the resolver |
| 9 | `cd tools/desk && go test ./internal/forgeban/... -count=1 && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 — the ratchet still reads 24 and the surface is still closed; this brief adds no shell-out and no passthrough |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | exit 0 — the test sets the new forge key in a roster fixture and asserts the load succeeds; an unregistered key would fail the roster closed, so this row fails if the key was added without registering it |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| The resolver ships but nothing calls it — dead code, exactly the `#274` failure repeated | row 8 (a real verb's suite runs against it) + row 2 (the single-construction-site test would be vacuous if no site existed, so the test asserts the resolver's own site exists) |
| A `forge` parameter, env var, or flag creeps in "for testing", so a session can choose its forge | row 4 |
| An unconfigured repo quietly resolves to GitHub because that is the historical default | row 5 |
| The resolver falls back to an ambient `gh`/`glab` credential when the token file is missing | row 6 |
| An unsupported operation returns a zero value that reads as "no results" rather than refusing | row 7 |
| A later brief constructs a backend directly and bypasses custody | rows 2 + 3 (two instruments: a Go test and a grep) |
| The new config key is not registered, so setting it fails the whole fleet closed | row 10 |
| The migration re-opens the closed surface (a new shell-out or a passthrough method) | row 9 |
| The contract is written in the header but the header disagrees with the code | **no row** — review-only. Prose/code agreement is an adequacy judgement; the Review gate reads the header against rows 5–7 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: PASS (code contract rows 1-10); row 11 could-not-check — HELD at implemented (gate:human, sensitive-data:yes) — 2026-09-05 opus-4.8[1m]-verifier (verify-desk dispatch), medici-finance/assay merged main 55bb04c

Runner != implementer. Offline envelope (KUBECONFIG=/dev/null). gate: human; risk {regulatory:no, customer:no, irreversible:no, sensitive-data:yes} — human gate; a model records Evidence and holds.

| # | command | expected | observed (exit + key line) | date · runner |
|---|---------|----------|----------------------------|---------------|
| 1 | cd tools/desk; go build ./... and go test ./... | exit 0 | exit 0; two full runs, 0 FAIL (a single earlier run showed a transient deskpost-suite flake, non-reproducing across two clean re-runs) | 2026-09-05 · opus-4.8[1m]-verifier |
| 2 | go test ./internal/deskkit/ -run TestForgeSingleConstructionSite | exit 0 PASS | exit 0; PASS | 2026-09-05 · opus-4.8[1m]-verifier |
| 3 | grep GitHubForge/GitLabForge backend literals outside forgeresolve.go | prints 0 | prints 0 (no backend literal outside the resolver) | 2026-09-05 · opus-4.8[1m]-verifier |
| 4 | go test ...TestForgeForRejectsCallerSuppliedForge | exit 0, no forge arg | exit 0 PASS | 2026-09-05 · opus-4.8[1m]-verifier |
| 5 | go test ...TestForgeForUnconfiguredRepoRefuses | Unverifiable, names repo+config, no GitHub backend | exit 0 PASS | 2026-09-05 · opus-4.8[1m]-verifier |
| 6 | go test ...TestForgeForMissingTokenRefuses | Refused, no backend, no ambient cred | exit 0; PASS incl subtests file_absent, insecure_mode 0644, no_ambient_fallback | 2026-09-05 · opus-4.8[1m]-verifier |
| 7 | go test ...TestUnsupportedOperationIsCouldNotCheck | Unverifiable naming forge+op+gap | exit 0 PASS | 2026-09-05 · opus-4.8[1m]-verifier |
| 8 | go test ./cmd/deskpost/... | exit 0 unmodified | exit 0; ok deskpost 25.2s, ok bodycheck | 2026-09-05 · opus-4.8[1m]-verifier |
| 9 | forgeban tests + TestNoForgeCLIShellout + TestForgeNoPassthrough | exit 0, ratchet closed | exit 0; ok forgeban, ok deskkit | 2026-09-05 · opus-4.8[1m]-verifier |
| 10 | go test ...TestRosterKnownKeySet | exit 0 (new key registered) | exit 0 PASS | 2026-09-05 · opus-4.8[1m]-verifier |
| 11 | statusgen --root . --consumers --brief forge-neutral/01 | exit 0 | could-not-check — local statusgen v0.25.0 refuses ASSAY_REPO_FORGES as an unknown key (the brief registers it in deskkit rosterconfig, verified by row 10; statusgen forge-awareness is deferred to forge-neutral/08), AND the global scan aborts on the register dir docs/streams/requirements/ having no README (#471). Route to CI pinned statusgen. Not a FAIL; no diff defect | 2026-09-05 · opus-4.8[1m]-verifier |

**VERIFY: PASS (rows 1-10, the full code contract green); row 11 could-not-check — HELD at implemented.** gate:human + sensitive-data:yes: a model runs the table for Evidence but cannot sign off; the human gate owns the flip.

RISK-VALUE: DERIVED — verifyCustodyFileMode perm = 0o600 @ tools/desk/internal/deskkit/forgeresolve.go:284 — owner-only read/write is the correct secret-credential-file mode and matches the existing desktoken gitlab rotation path; row 6 insecure_mode subtest asserts a 0644 file is Refused. A looser mode would hand a group/other-readable token to a backend — the sensitive-data leak the human gate guards.
RISK-VALUE: DERIVED — wellKnownForgeHosts = {github.com:github, gitlab.com:gitlab} @ tools/desk/internal/deskkit/forgeresolve.go:79-80 — the canonical public hostnames, exact-match only; every self-hosted instance deliberately does not match and falls through to refusal rather than guessing, so the set cannot silently misroute an unrecognised host to the wrong forge/identity.
**Verify-table RUN 2026-09-10 — opus-4.8[1m]-verifier (non-implementer, verify-desk). NOT a sign-off.** Merged main `22f7645a1826ff3082836503ee518b552885b623`, offline (`KUBECONFIG=/dev/null`, go1.26.5). Frontmatter: `gate: human`, `risk {sensitive-data: yes, rest no}`. Forge resolution contract — the forge comes from repo config; refusal is the only fallback.

| # | command | exit | observed | discharges |
|---|---------|------|----------|------------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | 0 | build 0; test 0 on clean re-run (one transient non-reproducing deskflip harness flake, out of scope — passes in isolation and on a second full run) | Row 1 |
| 2 | `go test ./internal/deskkit/ -run TestForgeSingleConstructionSite -count=1` | 0 | PASS — no backend literal outside the resolver | Row 2 |
| 3 | grep forge-backend construction sites over `tools/desk` `*.go` (excl `_test.go`, `forgeresolve.go`) | 0 | `0` — only two sites, both inside the resolver (forgeresolve.go:395,397) | Row 3 |
| 4 | `go test ./internal/deskkit/ -run TestForgeForRejectsCallerSuppliedForge -count=1` | 0 | PASS — `ForgeFor` takes no forge argument; no exported symbol accepts a caller-supplied forge | Row 4 |
| 5 | `go test ./internal/deskkit/ -run TestForgeForUnconfiguredRepoRefuses -count=1` | 0 | PASS — unconfigured repo + unrecognisable remote yields Unverifiable naming the repo + the config that would resolve it; returns no backend | Row 5 |
| 6 | `go test ./internal/deskkit/ -run TestForgeForMissingTokenRefuses -count=1` | 0 | PASS incl. file_absent, insecure_mode (0644), no_ambient_fallback — Refused, constructs no backend, reads no ambient credential | Row 6 |
| 7 | `go test ./internal/deskkit/ -run TestUnsupportedOperationIsCouldNotCheck -count=1` | 0 | PASS — unsupported op returns Unverifiable naming forge+operation+gap; no request against the other forge, no zero-value success | Row 7 |
| 8 | `go test ./cmd/deskpost/... -count=1` | 0 | ok deskpost; ok bodycheck — existing suite green unmodified with forge ops on the resolver | Row 8 |
| 9 | `go test ./internal/forgeban/... && TestNoForgeCLIShellout && TestForgeNoPassthrough` | 0 | ok forgeban; ok deskkit — surface still closed, no shell-out, no passthrough added | Row 9 |
| 10 | `go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1` | 0 | PASS — the forge key is registered in the roster known-set; setting it loads | Row 10 |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | 2 | could-not-check — the installed statusgen recognises only brief-v1; this is a brief-v2 file, so it returns COULD-NOT-CHECK. statusgen schema/forge awareness is deferred to a later forge-neutral brief; route to CI-pinned statusgen. Not a FAIL; no diff defect | Row 11 (could-not-check) |

`RISK-VALUE: DERIVED — custody file mode must be 0600 (Refused if looser) @ tools/desk/internal/deskkit/custodyowner_unix.go:23` — owner-only read/write is correct for a secret credential file; a looser mode (0644, group/other-readable) is Refused rather than handed to a backend (row 6 insecure_mode subtest proves a 0644 file is Refused). Matches the repo's established secret-file convention. A looser value would leak a group/other-readable token — the sensitive-data failure the human gate guards.
`RISK-VALUE: DERIVED — well-known forge hosts = exact-match {github.com, gitlab.com} @ tools/desk/internal/deskkit/forgeresolve.go:78-80` — exact lowercased-hostname match on the two canonical public forge hostnames, no suffix/substring matching; every self-hosted instance deliberately fails the match and falls through to refusal, precisely the "refusal is the only fallback" default.

**VERDICT: PASS on rows 1-10 (code contract green); row 11 could-not-check (statusgen brief-v2). gate:human + sensitive-data:yes — a model does NOT sign off.** Evidence gathered; the flip is the human's via the verify-gate sign-off card. Status stays at implemented.

**Findings:** (1) Row 1 one transient non-reproducing deskflip test-harness flake under full-tree parallel run (passes in isolation + on a second full run) — a deskflip flake-tracking note, not a defect of this brief (deskkit resolver + deskpost only). (2) Line-number drift only vs the prior run: custody-mode enforcement refactored into `VerifyCustodyOwnerOnly` (custodyowner_unix.go:23), same `!= 0600` semantics; control intact. (3) Anti-gaming: row 2's single-construction-site test is corroborated by row 3's independent grep (zero sites elsewhere); negative-path rows 5/6/7 assert real refusal semantics, not happy-path passthrough.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between "a verb writes as the wrong identity" and the damage,
   and is that acceptable? (The resolver is that control; row 2/3 and the
   `forge-surface-control.yml` job are the two independent layers behind it.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed?
   (Row 3 greps the tree with the Go test assumed absent; row 9 proves the CI control fires
   independently of anything this brief adds.)
