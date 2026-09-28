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
unblocks: ["forge-neutral/02", "forge-neutral/03", "forge-neutral/04", "forge-neutral/05", "forge-neutral/06", "forge-neutral/07", "forge-neutral/08", "forge-neutral/09", "forge-neutral/11", "forge-neutral/14", "forge-neutral/15", "forge-neutral/17"]
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
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `a91bffd0ea73e49b85569549cb4a4521703e827d` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`, go1.26.5) in an isolated worktree; runner ≠ implementer. gate: human + `sensitive-data: yes` — the table is RUN for Evidence; a model does not sign off, status stays `implemented`, the human closes the verify-gate.

| # | Command (in `tools/desk`) | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | 0 | build 0; every package `ok`, zero FAIL across the module (deskkit 35.4s, deskpost 58.6s, forgeban ok) | PASS |
| 2 | `go test ./internal/deskkit/ -run TestForgeSingleConstructionSite` | 0 | PASS — no backend literal outside the resolver | PASS |
| 3 | grep `GitHubForge{`/`GitLabForge{`/`&…` over `*.go` (excl `_test.go`+`forgeresolve.go`) | 0 | `0` — the only two live sites are `forgeresolve.go:444,446`, both inside the resolver | PASS |
| 4 | `go test …TestForgeForRejectsCallerSuppliedForge` | 0 | PASS — `ForgeFor` takes no forge arg; no exported symbol accepts a caller forge | PASS |
| 5 | `go test …TestForgeForUnconfiguredRepoRefuses` | 0 | PASS (real negative) — `err!=nil`, `f==nil`, exit `ExitUnverifiable`, message names the repo slug + `ASSAY_REPO_FORGES`; no GitHub backend | PASS |
| 6 | `go test …TestForgeForMissingTokenRefuses` | 0 | PASS — subtests `file_absent`, `insecure_mode` (0644→Refused, names "600"), `no_ambient_fallback` (failing mint→Refused, not a Forge) | PASS |
| 7 | `go test …TestUnsupportedOperationIsCouldNotCheck` | 0 | PASS (real negative) — unmapped ref → `ExitUnverifiable` naming forge+op+ref; `noRequestTransport` proves no request to the other forge; no zero-value success | PASS |
| 8 | `go test ./cmd/deskpost/... -count=1` | 0 | `ok deskpost 24.6s`; `ok bodycheck` — existing suite green on the resolver | PASS |
| 9 | forgeban tests + `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | 0 | `ok forgeban`; `ok deskkit` — surface still closed, no shell-out, no passthrough (ratchet note below) | PASS |
| 10 | `go test …TestRosterKnownKeySet` | 0 | PASS — `ASSAY_REPO_FORGES` registered in the roster known-set | PASS |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | — | COULD-NOT-CHECK — statusgen blocked by the offline verifier's shared-home writeguard backstop (writes STATUS.md; exemption human-only); consistent with both prior runs. Route to CI-pinned statusgen. | COULD-NOT-CHECK |

**Ratchet note (not a defect):** `allowedInvocationCeiling = 9` @ `tools/desk/internal/forgeban/allowlist.go:64`, not 24 as row-9 prose reads — later forge-neutral briefs (03/04/06) migrated the other verbs and ratcheted the ceiling down 24→9 (tightening as designed; a lower ceiling is a stronger ban). Row 9's test asserts ceiling == permit-list length whichever value, so it passes; "24" is a stale point-in-time descriptor.

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):**
- `RISK-VALUE: DERIVED — custody token file mode must be 0o600 else Refused @ tools/desk/internal/deskkit/custodyowner_unix.go:23.` Ranked #1 by irreversibility (a token at a group/other-readable mode is an irreversible secret leak — the exact sensitive-data harm this gate guards). Owner-only read/write is the correct mode for a secret credential file; row 6 `insecure_mode` proves a 0644 file is Refused and names the `chmod 600` remedy.
- `RISK-VALUE: DERIVED — well-known forge hosts = exact-match {github.com→github, gitlab.com→gitlab} @ tools/desk/internal/deskkit/forgeresolve.go:79-80.` Exact lowercased-hostname match only; every self-hosted host deliberately fails and falls through to `Unverifiable` refusal rather than misrouting to a wrong forge/identity — "refusal is the only fallback." No `NAMED, NOT DERIVED` value outstanding.

**Sensitive-data defense (gate: human):** the single control keeping an ambient credential from being read is that `ForgeFor` obtains the token ONLY from the custody minter path; a failing/absent mint surfaces as `Refused` rather than falling through to an ambient `gh`/`glab` read. Row 6 `no_ambient_fallback` proves it — a failing minter yields an error (not a Forge) naming the role/repo, only possible if the failure was surfaced not swallowed. Second independent layer: the `forge-surface-control.yml` no-passthrough CI ratchet + row 3's grep against any bypass construction site.

**Scope-traceability:** all observed work maps to Verify rows 1–11 (resolver, custody enforcement, roster key, the one wired verb deskpost). No invented scope; negative-path rows 5/6/7 genuinely assert refusal semantics.

**VERDICT: PASS** on rows 1–10; row 11 COULD-NOT-CHECK (statusgen environment limit, no diff defect) — **HELD at `implemented` (human sign-off owed via the verify-gate).** Open question for the human: none outstanding — the top-ranked risk-value (custody mode 0o600) is DERIVED and test-proven.
### Verify pass 2026-09-22 (non-implementer, VERIFY: PASS on the code contract — gate:human, routes to human gate)

Runner: `claude-opus-4-8[1m]` (non-implementer). Merged main `6204bb4f1eacc0229f2a86c8e0dce59edabdd22a`. Offline (`KUBECONFIG=/dev/null`). gate: human, risk {sensitive-data: yes}.

| # | Command | Expect | Observed (exit + key line) | Date | Runner |
|---|---------|--------|----------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | build 0; test 0 on clean re-run (one transient loopengine parallel flake did not reproduce; isolation PASS) | 2026-09-22 | opus-4.8-verifier |
| 2 | `go test ./internal/deskkit/ -run TestForgeSingleConstructionSite -v` | exit 0 | 0 — PASS; no backend literal outside the resolver | 2026-09-22 | opus-4.8-verifier |
| 3 | grep `GitHubForge{`/`GitLabForge{` over tools/desk *.go (excl _test, forgeresolve.go) | `0` | `0` — only two live sites, both in forgeresolve.go:477,479 (the resolver) | 2026-09-22 | opus-4.8-verifier |
| 4 | `TestForgeForRejectsCallerSuppliedForge -v` | exit 0, no forge arg | 0 PASS — ForgeFor takes no forge argument | 2026-09-22 | opus-4.8-verifier |
| 5 | `TestForgeForUnconfiguredRepoRefuses -v` | Unverifiable(6), names repo+config, no backend | 0 PASS — unconfigured repo → Unverifiable naming repo+config, no backend | 2026-09-22 | opus-4.8-verifier |
| 6 | `TestForgeForMissingTokenRefuses -v` | Refused, no ambient cred | 0 PASS incl subtests file_absent, insecure_mode (0o644 token rejected @ forgeresolve_test.go:166-172), no_ambient_fallback | 2026-09-22 | opus-4.8-verifier |
| 7 | `TestUnsupportedOperationIsCouldNotCheck -v` | Unverifiable naming forge+op+gap | 0 PASS — unsupported op → Unverifiable; no cross-forge request; no zero-value success | 2026-09-22 | opus-4.8-verifier |
| 8 | `go test ./cmd/deskpost/... -count=1` | exit 0 unmodified | 0 — ok deskpost/bodycheck/deskclose green with forge ops on the resolver | 2026-09-22 | opus-4.8-verifier |
| 9 | forgeban tests + TestNoForgeCLIShellout + TestForgeNoPassthrough | exit 0, surface closed | 0 all three — no shell-out, no passthrough | 2026-09-22 | opus-4.8-verifier |
| 10 | `TestRosterKnownKeySet -v` | exit 0 (forge key registered) | 0 PASS — ASSAY_REPO_FORGES registered in roster known-set | 2026-09-22 | opus-4.8-verifier |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | exit 0 | **could-not-check (exit 2)** — statusgen refuses a merged brief not in the diff vs current main; traced the implementing merge (PR #367) and the merge-base too, still c-n-c because the brief markdown predates the impl branch. Environment/timing limitation, not a diff defect; consistent with all prior runs; structural gap tracked #1281. | 2026-09-22 | opus-4.8-verifier |

Scope traceability: every Evidence row maps 1:1 to its Verify row; no invented scope.

RISK-VALUE: DERIVED — custody token file mode `!= 0o600` → error @ `tools/desk/internal/deskkit/custodyowner_unix.go:23` (ranked #1 by irreversibility for the sensitive-data gate) — owner-only `rw-------` is the correct mode for a secret credential file; the check is EXACT (`!= 0o600`), so any group/world-readable mode (0640/0644/0660) fails closed with a `chmod 600` remedy. Row 6 `insecure_mode` proves a 0o644 file is Refused. A looser value would hand a group/other-readable token to a backend — the exact sensitive-data leak this gate guards.
RISK-VALUE: DERIVED — well-known forge hosts = exact-match `{github.com→github, gitlab.com→gitlab}` @ `tools/desk/internal/deskkit/forgeresolve.go:79-80` — exact lowercased-hostname match only (no suffix/substring), so every self-hosted/unrecognised host fails the match and falls through to `Unverifiable` refusal rather than misrouting to a wrong forge/identity ("refusal is the only fallback"). Prevents a lookalike host resolving to a real identity.

**VERIFY: PASS on rows 1-10** (the full code contract green). Row 11 could-not-check is the structural `statusgen --consumers` merged-brief limitation tracked #1281 (same accepted condition sibling briefs landed under). gate: human + sensitive-data: yes → a model records Evidence but does NOT sign off; both top-ranked risk values are DERIVED and test-proven, so there is NO open question for the human. Routed to the human verify-gate for the `done` close.

### Witnessed verify run — 2026-09-27, non-implementer verifier (claude-opus-5-5, verify-desk dispatch) — gate: human, HELD at `implemented`

Merged main `46af8d389d5e02eb9a30c1c723709199d4b43fe5` (HEAD confirmed equal to the forge's main at start). Implementing change: PR #367 (merge `f723cc5c1aea05702625266880d1516a7d8a496e`, commit `359e12c5bb9208b9fa761ed537504f5b5ed28054`). Pinned statusgen v1.0.27 (sha256 matches the pinned darwin-arm64 digest) run directly, not through a shim. Offline envelope: `env -i`, KUBECONFIG=/dev/null, PATH = system dirs plus a scratch dir holding only `go` and `statusgen`, a scratch HOME holding only a copy of the non-secret roster file, no gh/glab on PATH, no forge token in the environment. Safety pre-read of every Verify cell's first code span: rows 1-10 are `go build`/`go test`/`grep`, row 11 is a local statusgen read — none mints or rotates a credential, calls a live forge API, or mutates live state, so the network-deny sandbox was not applied (it would also deny the loopback httptest servers the deskpost suite uses).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:987af78a4f53 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSingleConstructionSite -count=1 -v` | pass exit=0 | sha256:947e212550cc | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -rn -e 'GitHubForge{' -e 'GitLabForge{' -e '&GitHubForge' -e '&GitLabForge' tools/desk --include='*.go' \| grep -v _test.go \| grep -cv 'forgeresolve.go' \|\| true` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForRejectsCallerSuppliedForge -count=1 -v` | pass exit=0 | sha256:f8ab663eceac | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForUnconfiguredRepoRefuses -count=1 -v` | pass exit=0 | sha256:2afc1e07d351 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForMissingTokenRefuses -count=1 -v` | pass exit=0 | sha256:0f0bfdd8ed3d | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestUnsupportedOperationIsCouldNotCheck -count=1 -v` | pass exit=0 | sha256:dac65e33c738 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskpost/... -count=1` | pass exit=0 | sha256:f101e5274dc6 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/forgeban/... -count=1 && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 | sha256:2cbc77e012a5 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | pass exit=0 | sha256:32c90d2416f7 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | fail exit=2 | sha256:ae4a7d3fb169 | 2026-09-27 | assay-verifier-app[bot] @ 46af8d389d5e (on-behalf-of human:ian) (forge-identity) |

**Per-row notes (real output):**
- Row 1 — FAIL exit 1; the witness keeps only the output hash, so the failing package was not captured. Observed during the run: the deskwt package test binary was still executing at about 9.5 minutes against Go's default 10-minute per-package test timeout, while other verifier suites ran concurrently on the same host. The most likely cause is that timeout under host contention (environment-shaped), but this was NOT confirmed: a diagnostic re-run was not completed in this pass. Earlier runs at other main SHAs (2026-09-05, 2026-09-10, 2026-09-22) recorded row 1 green. Needs a re-run on a quiet host or CI before row 1 counts either way.
- Row 1 side effect: the full suite left an untracked `mailbox/` directory under the commsloop command package in the checkout (a test writing into the source tree). Not caused by this brief; noted for hygiene.
- Rows 2, 4, 5, 6, 7, 10 — targeted package tests, exit 0 (PASS). Row 4's test reflects over ForgeFor's parameters and AST-scans every exported deskkit func for a ForgeKind-typed or `forge`-named parameter, with a non-vacuity guard (fails if zero files were scanned).
- Row 3 — exit 0; output hash `9a271f2a916b` decodes to the single line `0`: no backend literal outside the resolver.
- Row 8 — `go test ./cmd/deskpost/... -count=1` exit 0.
- Row 9 — forgeban suite plus the shell-out and passthrough tests exit 0. Ratchet note as in the 2026-09-10 run: the permit ceiling is now 9 (ratcheted down from 24 by later briefs); the test asserts ceiling equals permit-list length, so it passes.
- Row 11 — could-not-check, exit 2: with a clean tree statusgen reports COULD-NOT-CHECK because the brief is not in the diff against merged main (the structural merged-brief limitation tracked in #1281). A later read, taken after this Evidence edit put the brief into the working-tree diff, returned exit 0 with `0 corroborated, 0 disproved, 7 unchecked` — every consumers entry, including `tools/desk/cmd/desktoken: fixed-here`, is UNCHECKED, so this is not a pass either.

**Observation — consumers claim vs merged code (for the human gate):** the brief marks `tools/desk/cmd/desktoken` as fixed-here ("its --forge flag becomes the custody path selector the resolver drives, not an independent switch"), and Task 2 says the same. The implementing commit did not touch desktoken. On merged main, desktoken's `--forge` is still an independent operator switch: an explicit `--forge github` forces the App mint even for a repo that resolves to GitLab, and an explicit `--forge gitlab` takes the PAT rotation path (tools/desk/cmd/desktoken/desktoken.go:697-725). Only the no-flag default consults the resolver (ForgeKindFor, via gitlab.go:236, added later). ForgeFor itself takes no forge input (row 4). Whether an operator-set custody switch in the mint tool is an acceptable reading of Task 2 is a question for the human; no Verify row covers it.

**Risk-bearing values — ENUMERATE (implementing diff plus deliverables) → RANK → DERIVE:**

Enumerated:
- E1 custody file permission `!= 0o600` → Refused @ tools/desk/internal/deskkit/custodyowner_unix.go:23 (introduced in the diff inline in forgeresolve.go, later moved behind the OS boundary)
- E2 `wellKnownForgeHosts = {"github.com": github, "gitlab.com": gitlab}` @ tools/desk/internal/deskkit/forgeresolve.go:79-80
- E3 GitLab API destination for the custody PAT: `gitlabAPIBaseOverride` = env `GITLAB_API_BASE` @ tools/desk/internal/deskkit/forgeresolve.go:647, else `GitLabAPIBase = "https://gitlab.com"` @ tools/desk/internal/deskkit/forge_gitlab.go:62
- E4 deskpost's custody role `"reviewer"` @ tools/desk/cmd/deskpost/comment.go:176 (also forgeclient.go:80)
- E5 GitLab custody file name `"gitlab-" + role + ".token"` @ tools/desk/internal/deskkit/forgeresolve.go:351
- E6 roster key `EnvRepoForges = "ASSAY_REPO_FORGES"` @ tools/desk/internal/deskkit/rosterconfig.go:164; value domain {github, gitlab} @ rosterconfig.go:1574; full owner/name slug required @ rosterconfig.go:1567
- E7 exit classes `ExitRefused = 5` @ tools/desk/internal/deskkit/exitcodes.go:51 and Unverifiable = 6 (existing, named by Task 3, not changed)
- Outside the diff (added later, not ranked here): `githubAppHost = "github.com"` @ forgeresolve.go:439

Ranked by irreversibility: E1 (a token at a loose mode is an exposed secret; needs rotation, not a redeploy) > E3 (a PAT sent to the wrong host is disclosed; needs rotation) > E2 (a wrong map routes a write through the wrong forge's identity) > E4 (a write under the wrong identity is public and misattributed) > E5, E6, E7 (a wrong value fails closed; an edit and redeploy fixes it).

- `RISK-VALUE: DERIVED — custody file perm = 0o600 (exact; anything else Refused) @ tools/desk/internal/deskkit/custodyowner_unix.go:23 — owner-only read/write is the minimal mode that still lets the owner process read the token; any group/other bit exposes the secret to another local principal. The check is exact inequality, so 0640/0644/0660 all fail closed with a chmod 600 remedy; row 6 insecure_mode proves a 0644 file is Refused and no backend is constructed.`
- `RISK-VALUE: NAMED, NOT DERIVED — GitLab PAT destination = env GITLAB_API_BASE, else "https://gitlab.com" @ tools/desk/internal/deskkit/forgeresolve.go:647 + forge_gitlab.go:62 — the host that receives the role's PAT comes from a process environment variable a session can set, and when unset defaults to gitlab.com even for a repo the roster binds to a self-hosted GitLab (ASSAY_REPO_FORGES names forge software, not an instance). The GitHub arm binds its token to github.com (forgeresolve.go:439) and the git-transport path refuses the gitlab.com default when GITLAB_API_BASE is unset (forgegit.go:137, the #727 class), but the ForgeFor API path does neither. The code comment argues this is the backend's own shipped default, not a new risk. I could not derive the right binding: it needs a decision on where a GitLab instance host should come from (roster, not env?) and whether an unset base should refuse. OPEN QUESTION for the human.`
- `RISK-VALUE: DERIVED — wellKnownForgeHosts = exact-match {github.com→github, gitlab.com→gitlab} @ tools/desk/internal/deskkit/forgeresolve.go:79-80 — these are the only two hostnames whose forge software is known by definition. Matching is exact on the lower-cased parsed host (no suffix/substring), so a lookalike or self-hosted host misses and falls through to the Unverifiable refusal (row 5) instead of guessing.`
- `RISK-VALUE: DERIVED — deskpost custody role = "reviewer" @ tools/desk/cmd/deskpost/comment.go:176 — deskpost already posted as the reviewer App before this brief (its expected login is RoleAppLogin("reviewer") @ tools/desk/cmd/deskpost/github.go:62, and its install/App-ID lookups use "reviewer"). Routing through ForgeFor keeps the same identity; row 8 proves the existing suite passes unchanged.`
- E5-E7 rank as reversible and fail-closed: E5 matches the file desktoken's rotation writes (tools/desk/cmd/desktoken/gitlab.go:119), and a mismatch is Refused; E6 is registered in the known-set (row 10) and a malformed entry deactivates the binding instead of widening it; E7 is the existing exit-class convention.

**Sensitive-data layers (gate: human):** the single control between "a verb writes as an identity nobody chose" and the damage is ForgeFor as the only construction site. Row 2 (AST test) and row 3 (an independent grep that does not rely on the Go test) are two instruments over that control. Row 9 shows the forgeban shell-out and passthrough layer still passes on its own.

**VERIFY: BLOCKED** — rows 2-10 PASS. Row 1 FAILED in the witness and the cause was not captured (most likely a host-contention package timeout; unconfirmed). Row 11 could-not-check (#1281). This is gate:human with sensitive-data:yes: a model records Evidence and does not sign off, so status stays `implemented`. Open question for the human: the NAMED, NOT DERIVED GitLab PAT destination (E3) above, plus the desktoken `--forge` consumers observation.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:00684f6ad3ac | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSingleConstructionSite -count=1 -v` | pass exit=0 | sha256:26134f2438a4 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -rn -e 'GitHubForge{' -e 'GitLabForge{' -e '&GitHubForge' -e '&GitLabForge' tools/desk --include='*.go' \| grep -v _test.go \| grep -cv 'forgeresolve.go' \|\| true` | pass exit=0 | sha256:9a271f2a916b | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForRejectsCallerSuppliedForge -count=1 -v` | pass exit=0 | sha256:af95c9e02523 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForUnconfiguredRepoRefuses -count=1 -v` | pass exit=0 | sha256:54dd0f92a044 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeForMissingTokenRefuses -count=1 -v` | pass exit=0 | sha256:c6d572ab2414 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestUnsupportedOperationIsCouldNotCheck -count=1 -v` | pass exit=0 | sha256:e61c2eb3f199 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskpost/... -count=1` | pass exit=0 | sha256:d3e6003827a3 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/forgeban/... -count=1 && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 | sha256:17bbab62c795 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1 -v` | pass exit=0 | sha256:219dd1d1d077 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/01` | pass exit=0 | sha256:ad07be4c20e4 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |

**Re-witness notes (2026-09-28, clean tree).**
- Re-run at the batch tree (`e1afb99aca99`, porcelain empty before the run, no `+dirty`) because main changed a declared input after batch F; this table supersedes the batch-F witness above for these rows.
- Row 1 (whole-module `go test ./...`) is HELD, attributed to the run environment (from a same-tree reproduction of the row), not to this brief: `cmd/commsgw` unix-socket bind is denied by the network-off sandbox; `cmd/cellctl` bash-quoting test sees macOS bash 3.2; `internal/loopengine` and `cmd/commsloop` hit 5s deadlines under host load 21-38 and pass outside the sandbox; `internal/avatar` golden images differ on darwin/arm64 (go1.27.1 local), NOT confirmed against CI. None of these packages is a deliverable of forge-neutral/01.
- Rows 2-11 PASS (10 of 11), including row 11 `statusgen --consumers`, which exits 0 on this run.
- Test hygiene: a `cmd/commsloop` test leaves an untracked `mailbox/` directory in the source tree; it reappeared mid-run (stamp is taken before any row runs, so it stays clean) and was removed afterwards.

**VERIFY: BLOCKED** — rows 2-11 pass (10 of 11); row 1 held, environment-attributed above. gate: human, sensitive-data: yes — a model records Evidence and does not sign off; status stays `implemented`.

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
