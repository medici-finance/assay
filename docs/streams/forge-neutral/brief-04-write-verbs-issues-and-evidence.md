---
brief: assay:assay:forge-neutral:04
title: Write verbs B — deskpr, deskfile, deskclose and deskevidence onto the resolver
why: >-
  These four carry the rest of the fleet's outward writes — opening a change, filing an issue,
  closing one, and landing an Evidence row. Three of them shell `gh` under the caller's ambient
  credential by documented design, and deskevidence performs a GitHub App JWT exchange and a
  Contents-API write end to end. Routing them through the resolver is what makes "the desk
  verbs are the only sanctioned write path" true rather than aspirational, and it is where the
  identity-class blocker in the permit register actually gets answered instead of moved.
wave: 2
depends: ["forge-neutral/01"]
unblocks: ["forge-neutral/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [274]
schema: brief-v2
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver and its custody binding"
  - "tools/desk/internal/forgeban/allowlist.go:21-31 — the (identity) blocker class these three rows carry, and why retiring them is a custody decision"
  - "docs/streams/forge-gitlab/pilot-report.md D-8 — on GitLab the verifier's Evidence row has no direct-main lane and must travel as a merge request"
  - "freshness-checked 2026-09-02 @ deae247 — deskpr shells gh (exec.go:87, create at deskpr.go:293); deskfile at exec.go:50 (create at deskfile.go:473); deskclose at exec.go:48 (close at github.go:193-195); deskevidence builds net/http against deskkit.GitHubAPIBase (github.go:29), mints an installation token (:220) and writes via the Contents API (:358)"
exec-tier: strong
exec-tier-why: "it changes WHICH identity performs three writes that currently run under an ambient credential, and it re-homes the Evidence landing — a subtle error here writes as the wrong actor or lands Evidence the Evidence-actor check will then wrongly believe (questions a and c)."
gate-why: >-
  `deskfile`, `deskclose` and `deskpr` reach the forge under the caller's ambient CLI
  credential by documented design — `deskfile` states it "NEVER mints an App installation
  token; there is no desktoken call on any path". Routing them through the resolver changes WHO
  performs the write, which the permit register explicitly calls a token-custody decision and
  not a transport change. The human is confirming the new acting identity for each of the
  three, and separately confirming the Evidence-landing lane: on a forge with no direct-default-
  branch push, the verifier's Evidence row must travel as a change with a reviewer verdict, and
  that structural difference must be a stated design, not a discovered one.
domain: complicated
consumers:
  - "tools/desk/cmd/deskevidence: fixed-here"
  - "tools/desk/internal/deskkit/forge.go: fixed-here (WriteFile + ReadFile added to the frozen seam, both backends)"
  - "docs/streams/forge-gitlab/inventory.md: fixed-here (both ops inventoried, rows 21–22)"
  - "tools/desk/cmd/deskpr: follow-on forge-neutral/04b (the gh-migration for deskpr/deskfile/deskclose; #509 ruled it a code-aware rescope that first adds the enumerated ops each still lacks, not a ratchet-number correction)"
  - "tools/desk/cmd/deskfile: follow-on forge-neutral/04b"
  - "tools/desk/cmd/deskclose: follow-on forge-neutral/04b"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up forge-neutral/10 (the Evidence-landing lane gains a hop on a forge with no direct-default-branch push; the conformance round trip is where the loop shape is proved before the skill text is changed)"
version: 2
id: 929d765d-2ce6-4907-90c6-52e613f197cb
---

# Brief 04 — Write verbs B: deskpr, deskfile, deskclose, deskevidence

## Amendment (#509 — slice C, delivered here)

The four-verb scope below was NOT implementable as one change: five worker rounds established
against the code that the migration's premise — that it lowers the forge-CLI ratchet — is false
in scope, because `deskpr`, `deskfile` and `deskclose` each keep un-migratable `gh` calls with no
enumerated forge op, so their permit rows must stay exactly as `deskclose`'s did. The human gate
(#509) ruled **Option C**: this brief delivers ONLY the `deskevidence` slice, and the
`deskpr`/`deskfile`/`deskclose` gh-migration becomes a code-aware follow-on brief
(`forge-neutral/04b`) that first adds the enumerated ops each still lacks (a branch→change
lookup, PR body/title fields, an issue-search op, a label-list op). The ratchet is **untouched at
16** — the forge-method count is not the ratchet, and no `deskevidence` permit row exists to
remove (it reaches the forge over `net/http`, never a forge CLI).

The delivered slice adds **two** ops to the seam, not one: a fat `WriteFile` AND a companion
`ReadFile`. The `--brief-path` Evidence landing is a genuine read → transform → write (read the
remote brief, merge its `## Evidence` section, write the result), and the transform cannot be
folded into a backend-agnostic write, so the read is its own op. Both are consumed by
`deskevidence` in this same change (the §6 freeze rule is amended by this brief, not bypassed).
`WriteFile` folds the idempotency-noop read (a `Changed` flag), the append-only shrink guard (the
constraint is passed in and the backend refuses post-fetch), the default-branch writability probe
(a `DefaultBranchNotWritable` sentinel), and the branch-creation fallback (GitLab `start_branch`
inline; GitHub's default branch is directly writable by the verifier App — no new `CreateRef` op).

The Task and Verify sections below are rewritten to this slice; the original four-verb text is
preserved in the git history and in `docs/streams/forge-gitlab/inventory.md` rows 21–22.

## Context
files:
- `tools/desk/cmd/deskpr/exec.go`, `tools/desk/cmd/deskpr/deskpr.go` — draft-change creation.
- `tools/desk/cmd/deskfile/exec.go`, `tools/desk/cmd/deskfile/deskfile.go` — issue filing.
- `tools/desk/cmd/deskclose/exec.go`, `tools/desk/cmd/deskclose/github.go`,
  `tools/desk/cmd/deskclose/authority.go` — issue/change closing and the authority read.
- `tools/desk/cmd/deskevidence/github.go`, `tools/desk/cmd/deskevidence/deskevidence.go` —
  the App mint and the Evidence write.
- `tools/desk/internal/forgeban/allowlist.go` — three permit rows removed, ceiling lowered.

single-point-of-failure: for the three ambient-credential verbs the single control is WHICH
token the resolver hands the backend — get it wrong and the write lands as an identity nobody
chose. Two independent layers: the backends refuse an unminted token outright
(`forge_github.go:68-72`, `forge_gitlab.go:108-112`), and the commit-identity preflight
independently compares the resulting actor against the roster entry (`preflight.go:752-781`) —
a credential fault and an actor fault trip different checks in different components.

facts:
- The permit register's identity class covers exactly these rows: `deskclose/exec.go::runGH`
  (`allowlist.go:74`), `deskfile/exec.go::gh` (`:96`), `deskpr/exec.go::gh` (`:143`). Its
  header states each tool *"gates WHETHER and WHAT, never WHO"* and mints no token on any
  path, so routing it through the seam *"changes WHO performs the write, which is a
  token-custody decision"* (`allowlist.go:22-31`).
- `deskpr` is the exception among the three: it DOES mint a worker installation token
  (`tools/desk/cmd/deskpr/exec.go:30,90`) but retains a documented `--as-app=false` ambient fallback
  (`tools/desk/cmd/deskpr/deskpr.go:218`).
- `deskevidence` is fully GitHub-App-shaped: `RoleAppLogin("verifier")`
  (`tools/desk/cmd/deskevidence/github.go:44-46`), installation lookup (`:109`), JWT→installation-token
  exchange (`:220`), and the Evidence write itself via `PUT /repos/…/contents/…` (`:358`),
  with the bot commit identity built at `:455-456`.
- `Forge` already carries `CreateDraftChange` (`forge.go:196`), `FileIssue` (`:205`) and
  `CloseIssue` (`:207`). The Contents-API write `deskevidence` performs is **not** an
  enumerated operation and has no `Forge` method today.
- On the pilot, the default branch was push = No one for every identity including the owner,
  so the GitHub carve-out that lets the verifier commit an Evidence row directly has no
  GitLab equivalent; the Evidence row travelled as a merge request with a reviewer verdict
  (`docs/streams/forge-gitlab/pilot-report.md` D-8).
- `#274` reports that `forge-gitlab/07`'s call-site migration for `deskpr`/`deskfile`/
  `deskclose` did not land, and that its Verify row 3 passed vacuously because it grepped for
  `exec.Command("gh")` while the tools shell `gh` through a `runCmd` wrapper. This brief
  closes that gap for those verbs; the ban test and row 7 below are written against the
  wrapper form.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green **unmodified**.
- Do not remove or weaken the `--as-app=false` refusal semantics without saying so in the PR;
  narrowing an escape hatch is in scope, widening one is not.

## Task (slice C — delivered)
1. **Add TWO ops to `Forge`, both backends, both consumed by `deskevidence` in this change.**
   `WriteFile(repo, WriteFileInput)` writes a file's whole content on a branch (GitHub Contents
   API ↔ GitLab Repository Files API), folding: the idempotency-noop read (returns a `Changed`
   flag, writes nothing on byte-identical content); the append-only shrink guard (`AppendOnly`
   passed in, the backend refuses post-fetch below the branch's current row count); the
   default-branch writability probe (a `DefaultBranchNotWritable` sentinel — GitLab's protected
   default; GitHub's is directly writable by the verifier App); and the branch-creation fallback
   (`StartBranch` cuts the side branch inline — no new `CreateRef` op). `ReadFile(repo,
   ReadFileInput)` reads a file's content at a ref, consumed by the `--brief-path` Evidence merge
   (read the remote brief → merge the `## Evidence` section → write via `WriteFile`). Record both
   in `docs/streams/forge-gitlab/inventory.md` (rows 21–22) and give each a both-backend contract
   case.
2. **Migrate `deskevidence` onto the resolver.** Route its Evidence write through `WriteFile` and
   its brief read through `ReadFile`, under `ForgeFor(fr, "verifier")` with a
   `SetGitHubCustodyMinter` hook (the /03 / PR #498 precedent). Delete its `apiBaseURL` and its
   hand-rolled JWT/installation exchange — the mint moves to the identity layer
   (`desktoken verifier --repo`), reached through a `mintVerifierToken` helper.
3. **The Evidence lane on a forge with no direct-default-branch push.** When `WriteFile` reports
   the `DefaultBranchNotWritable` sentinel, `deskevidence` lands the row on a side branch (via
   `WriteFile` with `StartBranch`) and opens a draft change — and says so on stdout. It does NOT
   attempt the direct write and report success, and it does NOT skip the row.
4. **Ratchet untouched at 16.** No `allowedInvocationCeiling` change and no forgeban permit-row
   change: the forge-method count is not the ratchet, and `deskevidence` has no permit row.
   `deskpr`/`deskfile`/`deskclose` are rescoped OUT to the follow-on brief `forge-neutral/04b`.
5. **Amend this brief in the same PR** (done above): enumerate `ReadFile` as in-scope for the
   slice, and name the `deskpr`/`deskfile`/`deskclose` gh-migration as the follow-on brief. The
   /03-style test rewrites `deskevidence` needs are permitted (amended row 3 below).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/... -count=1` | exit 0 — the migrated suite is green |
| 3 | *(amended, #509)* — no EXISTING assertion in a migrated test suite is weakened or deleted except gh-argv / hand-rolled-transport assertions replaced by their forge-op equivalents (the /03 precedent); new test files are expected. `deskevidence`'s install-id/JWT tests are gone WITH the code they pinned — the custody question they were about is now the resolver's (`tools/desk/internal/deskkit/forgeresolve_test.go`). | reviewer reads the diff |
| 4 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows `= 16` — **untouched** (the migration removes no permit row) |
| 5 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 — the ratchet passes at 16 |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestNoForgeCLIShellout\|TestForgeNoPassthrough' -count=1` | exit 0 — the seam grows two ops and stays closed (no generic/endpoint method, no extra exported backend method) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestForgeGithubGolden\|TestForgeGitlabGolden\|TestForgeGitlabCoverage' -count=1` | exit 0 — `read_file` / `write_file*` golden cases pin both backends' wire, and coverage reconciles the seam against the inventory |
| 8 | `{ grep -rn -e 'apiBaseURL' -e 'access_tokens' tools/desk/cmd/deskevidence --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` — the hardcoded host and the hand-rolled installation exchange are gone, not merely unused. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestWriteFileOpBothBackends -count=1 -v` | exit 0 — the new file ops run the same scenario names (including a `ReadFile` case) against both backends' recorded fixtures |
| 10 | `cd tools/desk && go test ./cmd/deskevidence/... -run TestEvidenceLandsAsChangeWhenDefaultBranchClosed -count=1 -v` | **negative path**: with the resolved forge reporting the default branch not directly writable, the run opens a draft change and performs NO direct write to that branch — asserted by the recording fake forge showing zero writes to the default branch — and exits 0 with the change named on stdout |
| 11 | `statusgen --root . --consumers --brief forge-neutral/04` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A file op is added with no consuming call site, violating the freeze rule | row 9 + rows 21–22 of the inventory + `deskevidence`'s own suite (row 2) |
| The seam grows a generic/passthrough op behind the two file ops | row 6 (`TestForgeNoPassthrough`: no generic-verb method, no endpoint parameter, no extra exported backend method) |
| The extraction changes a backend's wire behaviour | row 7 (the golden corpora pin `read_file` / `write_file*` per backend) |
| On a forge with no direct-default-branch push, `deskevidence` reports success having written nothing | row 10 asserts a change was opened AND zero direct writes to the default branch occurred |
| The Evidence row is skipped entirely rather than re-routed, so a verified brief has no Evidence | row 10 asserts exit 0 with the change named — a skip would produce neither |
| `deskevidence`'s bot commit identity is lost when the JWT exchange moves to the resolver | row 2 (its suite covers the attribution three-state via the author `WriteFile` reports) |
| The ratchet is silently moved when it should not be | rows 4 + 5 (ceiling stays 16, no permit-row change) |
| The verify-desk skill text still describes a direct-to-default-branch Evidence landing | **no row** — deliberately deferred to forge-neutral/10, which proves the loop shape before the prose changes; recorded in `consumers:` as a follow-up rather than left implicit |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `13814ff8d84bab20bf2081f4e08a686bc90e2ac6` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`, go1.26.5) in an isolated worktree; runner ≠ implementer (fn/04 = PR #532, merge `e032f6f0`). gate: human + `sensitive-data: yes` — table RUN for Evidence; no model sign-off; status stays `implemented`.

| # | Command (in `tools/desk`) | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | 0 | build clean; every package `ok` (deskkit 37.6s, forgeban, deskevidence) | PASS |
| 2 | `go test ./cmd/deskevidence/... -count=1` | 0 | `ok cmd/deskevidence 4.190s` — migrated suite green | PASS |
| 3 | reviewer-reads-diff / no-weakening | — | JWT + install-id tests removed WITH the code they pinned (custody moved to `tools/desk/internal/deskkit/forgeresolve_test.go`); no assertion weakened beyond gh/transport→forge-op swaps (the /03 precedent) | PASS |
| 4 | `grep -n allowedInvocationCeiling internal/forgeban/allowlist.go` | 0 | `= 9` (ACTUAL; the brief's literal "16" is stale). fn/04 (#532) does NOT touch allowlist.go — the 16→9 ratchet came from a LATER brief (04b, #775/#783). The row's INTENT — this brief changes no permit row — HOLDS | PASS |
| 5 | `go test ./internal/forgeban/...` | 0 | `ok` — ratchet passes at the measured ceiling (9) | PASS |
| 6 | `go test -run 'TestNoForgeCLIShellout\|TestForgeNoPassthrough'` | 0 | both PASS; frozen surface = 37 ops; no generic method, no endpoint arg, neither backend exports a method outside the interface | PASS |
| 7 | `go test -run 'TestForgeGithubGolden\|TestForgeGitlabGolden\|TestForgeGitlabCoverage'` | 0 | `ok` — read_file/write_file* golden wire pinned both backends; coverage reconciles seam vs inventory | PASS |
| 8 | `grep -rn -e apiBaseURL -e access_tokens cmd/deskevidence --include='*.go' \| grep -v _test.go \| wc -l` | — | `0` — hardcoded host + hand-rolled installation exchange gone from non-test files (`forgeAPIBase` is test-only, empty in production) | PASS |
| 9 | `go test -run TestWriteFileOpBothBackends -v` | 0 | 5 scenarios incl. `read_file_returns_content_and_id`, each ×github+gitlab, all PASS | PASS |
| 10 | `go test -run TestEvidenceLandsAsChangeWhenDefaultBranchClosed -v` | 0 | PASS (negative path — see defense note) | PASS |
| 11 | `statusgen --root . --consumers --brief forge-neutral/04` | — | COULD-NOT-CHECK — offline verifier shared-home writeguard (#1035) + statusgen v1.0.6 brief-v2 gap. Corroborated manually: deskevidence fixed-here (migrated, JWT gone, uses WriteFile/ReadFile); forge.go fixed-here (both backends, 37-op surface); inventory rows 21-22 present; deskpr/deskfile/deskclose correctly NOT migrated (deferred to 04b); verify-desk/SKILL.md unchanged (deferred to forge-neutral/10) | COULD-NOT-CHECK |

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):** No `NAMED, NOT DERIVED` literal owed to the human.
- Enumerated: (a) ceiling `9` — NOT this brief's literal (allowlist.go untouched); (b) default-branch-closed detection — **no magic constant**: the GitLab backend reads the project's live `DefaultBranch` from the API and compares `in.Branch == p.DefaultBranch` (`forge_gitlab.go:2241`); (c) side-branch naming `"evidence/" + base + "-" + dig[:8]` (`deskevidence.go:311`) — cosmetic/reversible; (d) token-file mode — not touched (mint moved to `desktoken verifier`).
- Ranked by irreversibility: the write-identity and write-location decisions are the irreversible surface, but identity is minted by desktoken (no literal) and location is derived from the live-API default-branch value (no literal). Only reversible cosmetic branch-naming literals were introduced.
- `RISK-VALUE: DERIVED (N/A-for-human) — the sensitive crux (never force-write a protected default branch) uses NO pinned literal; it reads the live default-branch and re-routes. The one literal introduced, dig[:8], is a reversible uniqueness slice backstopped by idempotency-noop + append-only shrink guard + immediate draft change.`

**Sensitive-data defense (gate: human) — two independent layers:** (1) the GitLab backend PROBES before mutating — `GetProject` → `in.Branch == p.DefaultBranch` → returns the `DefaultBranchNotWritable` sentinel BEFORE any write (a pre-write probe, not a caught failure). (2) On the sentinel, deskevidence re-routes: writes a side branch `evidence/<base>-<dig>` (StartBranch=base), opens a draft change (head=side, base=default), prints the draft on stdout, and never retries a direct write. Row 10 asserts this against a recording fake: ZERO writes with `Branch=="main"`, exactly 1 side-branch write, exactly 1 draft change head=side/base=main, stdout contains "draft". GitHub's default is directly writable by the verifier App (a stated per-forge carve-out, `forge_github.go:1703`).

**Scope-traceability:** clean — all shipped work maps to Verify rows/Task items; deskpr/deskfile/deskclose correctly left on gh (04b); ceiling untouched by this brief; the #509 amendment (two-op WriteFile+ReadFile scope) accurately reflects delivery.

**VERDICT: PASS** on rows 1–10; row 11 COULD-NOT-CHECK (tooling/isolation, corroborated manually) — **HELD at `implemented` (human sign-off owed via the verify-gate).** No open NAMED-NOT-DERIVED value; the human confirms (1) the custody/acting-identity decision and (2) the Evidence-lane side-branch+draft-change design.

### Verify re-run — 2026-09-27, non-implementer dispatched verifier (opus-5.5[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `main` @ `d034d39fe1c8f0a0a93ba1177f208c48b068a1d0` (confirmed against the forge with the verifier App token). Implementing change: PR #532 (merge `e032f6f0a`); runner is not the implementer. Witness: the pinned statusgen v1.0.27 darwin-arm64 binary (sha256 matches the pin), run directly (not through a wrapper), non-dry, under a network-denied macOS sandbox with loopback-only allowance, `PATH` restricted to system dirs plus `go` and the pinned statusgen, `GOFLAGS=-count=1`, `GOPROXY=off`, `GOTOOLCHAIN=local` (go1.27.1), `KUBECONFIG=/dev/null`, no forge credential in the environment. No row's first code span mints a credential or reaches a live forge; the sandbox is belt-and-braces. No row is `check:ci`-classed, so the darwin network-off gap (#1800) does not apply.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:308db05e6c9c | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/... -count=1` | pass exit=0 | sha256:16c2d1944bed | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `deskevidence` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:db9b13da93d9 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | pass exit=0 | sha256:e7e1daaf83f6 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 | sha256:55af7a016367 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestNoForgeCLIShellout\|TestForgeNoPassthrough' -count=1` | pass exit=0 | sha256:3dd607cfabb6 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestForgeGithubGolden\|TestForgeGitlabGolden\|TestForgeGitlabCoverage' -count=1` | pass exit=0 | sha256:98d30b1e5587 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -rn -e 'apiBaseURL' -e 'access_tokens' tools/desk/cmd/deskevidence --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestWriteFileOpBothBackends -count=1 -v` | pass exit=0 | sha256:58e4450abbd5 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskevidence/... -run TestEvidenceLandsAsChangeWhenDefaultBranchClosed -count=1 -v` | pass exit=0 | sha256:6c32b829e582 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/04` | fail exit=2 | sha256:577a71dc7037 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |

**Per-row observed output (witness above; notes only, no second table):**
- Row 1 — witness `fail exit=1`. Targeted package tests, package by package (88 packages, same sandbox and env), isolate three failing tests, all in packages PR #532 did not touch (cellctl, commsgw, avatar); every package this brief touched (cmd/deskevidence, internal/deskkit, internal/forgeban) is `ok`. Each failure is environment-shaped, shown by re-running just that test: (a) cellctl `TestBashQuoteMatchesBash` — `bashQuote("~lead") = "\\~lead", bash says "~lead"` under the system bash 3.2 that the restricted PATH selects; `ok` with the Homebrew bash on PATH. (b) commsgw `TestBypassClientPreflight…` and `TestRefusalJournalSocket…` — `bind: operation not permitted` on a unix socket; the sandbox's network deny covers unix sockets; `ok` once unix sockets are allowed (still no IP reach beyond loopback). (c) avatar `TestGolden20px` — PNG strip differs from the golden under go1.27.1; `ok` under the CI-pinned go1.25.0 toolchain (GO_VERSION 1.25.0 in the workflows). The main CI `build-test` check at this SHA reports success. No failure is in this brief's scope; row 1 is recorded as the witness saw it.
- Row 2 — `ok cmd/deskevidence` — migrated suite green.
- Row 3 — prose, reviewer-read row: the witness executed its first code span (`deskevidence`, not on the restricted PATH) → could-not-run exit 127, which proves nothing. Read by this verifier instead: the PR's removed deskevidence tests are the JWT, installation-id, key-search-path and HTTP-client tests, gone with the code they pinned (custody now under the resolver's own suite). The one non-transport removal, the visibility-error fail-closed gate test, is replaced by a seam stub in deskevidence plus the fail-closed cases in the deskkit repovis suite. No surviving assertion was weakened. Reviewer-read: holds.
- Row 4 — witness `pass exit=0` is VACUOUS: the grep prints `const allowedInvocationCeiling = 6`, not the `= 16` the Expect cell names. Stale-shaped: PR #532 does not touch the allowlist; later briefs (the 04b / write-verbs-C line) lowered the ratchet 16→9→6. The row's intent (this brief moves no permit row) holds.
- Row 5 — `ok internal/forgeban` — ratchet passes at the current ceiling (6).
- Row 6 — `ok internal/deskkit` — both no-shellout and no-passthrough tests pass.
- Row 7 — `ok internal/deskkit` — GitHub and GitLab golden wire and GitLab coverage pass.
- Row 8 — witness `fail exit=1`, but the pipeline PRINTS `0` (the expected output). The exit is the first grep's no-match status surfaced by the witness's pipefail shell, so the row is authored in a way that cannot pass under pipefail when its own expectation holds. Same class as #1699. Stale-shaped (check definition), not a defect: `apiBaseURL` and `access_tokens` are absent from deskevidence non-test sources.
- Row 9 — `--- PASS: TestWriteFileOpBothBackends`, 7 scenarios incl. a `read_file…` content-and-id case and the `write_file…` default-branch probe, each across both backends.
- Row 10 — `--- PASS` on the negative path (default branch closed → side-branch write + draft change, zero direct writes to the default branch).
- Row 11 — COULD-NOT-CHECK on merged main by design: on a clean tree statusgen says `COULD-NOT-CHECK: assay:assay:forge-neutral:04 is not in the diff against d034d39fe1c8…, so this run carries no evidence about its claims`. Manual corroboration against PR #532's own diff: deskevidence, forge.go (plus both backends) and the gitlab inventory are in the diff (fixed-here holds). deskpr, deskfile, deskclose and the verify-desk skill are NOT in the diff, which matches their follow-on / follow-up routing. Inventory rows 21–22 are present and marked implemented.

**Risk-bearing value (sensitive-data: yes) — ENUMERATE → RANK → DERIVE.** Enumerated over PR #532's non-test diff (deskevidence main + github files, deskkit forge.go and both backends) plus the Deliverables:
- E1 `ForgeFor(fr, "verifier")` @ tools/desk/cmd/deskevidence/github.go:70 — resolver custody role.
- E2 `execCommand("desktoken", "verifier", "--repo", repoSlug)` @ tools/desk/cmd/deskevidence/github.go:118 — mint role.
- E3 `RoleAppLogin("verifier")` @ tools/desk/cmd/deskevidence/github.go:146 (also :175) — the attribution check's expected actor.
- E4 GitHub backend `DefaultBranchNotWritable = false` on every path (never assigned) @ tools/desk/internal/deskkit/forge_github.go:2445 (carve-out stated :2441-2444).
- E5 GitLab probe `p.DefaultBranch != "" && in.Branch == p.DefaultBranch` → `DefaultBranchNotWritable = true` @ tools/desk/internal/deskkit/forge_gitlab.go:3880-3881.
- E6 shrink threshold `res.Rows < res.PriorRows` (row = non-blank line, forge.go:687) @ tools/desk/internal/deskkit/forge_github.go:2480 and forge_gitlab.go:3922.
- E7 side branch `"evidence/" + … + "-" + dig[:8]` @ tools/desk/cmd/deskevidence/deskevidence.go:612; fallback component `"evidence"` and the commit-message literals — cosmetic.
- Out of diff, listed for completeness: `allowedInvocationCeiling = 6` @ tools/desk/internal/forgeban/allowlist.go:84 (not introduced or changed by this brief).

Ranked by irreversibility: E1–E3 first (an Evidence commit on a default branch under the wrong actor cannot be undone without a history rewrite, and the Evidence-actor check would believe it). Then E4 (a wrong value produces a server-side refusal, not a wrong write: reversible). Then E5 (a wrong value turns a direct write into a draft change: reversible). Then E6 (a same-count clobber passes, but git history keeps the prior content: reversible). E7 last (cosmetic).

- `RISK-VALUE: DERIVED — E1–E3 role = "verifier" @ tools/desk/cmd/deskevidence/github.go:70,118,146 — one role key at the mint, the resolver custody hook and the actor check. It equals the pre-migration binding (the removed code already checked the verifier App login), so this brief does not change deskevidence's acting identity. The methodology assigns Evidence authorship to the non-implementer verifier, and a mismatch trips the independent attribution check (wrong author → refusal; missing author → could-not-check).`
- `RISK-VALUE: NAMED, NOT DERIVED — E4 GitHub DefaultBranchNotWritable = false (never set) @ tools/desk/internal/deskkit/forge_github.go:2445 — whether the verifier App may write a GitHub default branch directly is a per-repo live branch-ruleset fact; the backend hard-codes "writable" with no probe. On this public repo the default branch is PR-required (the Evidence-by-PR lane was ruled on #742), so here the carve-out does not hold. A direct deskevidence write is refused server-side (fails loud, no wrong write) and is NOT rerouted to the draft-change lane the GitLab arm gets. Not derivable offline (reading live rulesets is out of envelope), and the correct per-repo value is a custody/lane decision.`
- `RISK-VALUE: DERIVED — E5 GitLab default branch treated as closed @ tools/desk/internal/deskkit/forge_gitlab.go:3880 — from the GitLab pilot report D-8 (default branch push = No one for every identity, owner included). Erring toward the reviewable lane costs only an extra draft change. Edge: a project reporting an empty default branch falls through to a direct write.`
- `RISK-VALUE: DERIVED — E6 shrink threshold strict "<" @ tools/desk/internal/deskkit/forge_github.go:2480 / forge_gitlab.go:3922 — append-only means the row count must not fall, so any reduction is refused and an equal count is allowed. Both backends share one row definition (forge.go:687), and git history retains any same-count overwrite.`
- E7 is reversible and cosmetic; no derivation owed.

**Open question for the human (carried verbatim from the NAMED, NOT DERIVED line):** E4 GitHub `DefaultBranchNotWritable = false` (never set) @ tools/desk/internal/deskkit/forge_github.go:2445. Should the GitHub backend probe branch protection / rulesets and return the sentinel, as the GitLab arm does, so deskevidence reroutes automatically on a PR-required GitHub default branch? Or is the operator-level Evidence-by-PR lane (#742) the intended design, with the hard-coded carve-out kept? The two gate-why confirmations also stay open: (1) the acting identity and custody, and (2) the side-branch + draft-change Evidence lane.

**Observations:** (a) a commsloop test writes an untracked `mailbox/cell-a/worker-desk/x.json` into the source tree under tools/desk/cmd/commsloop during row 1: test pollution outside this brief. (b) The avatar 20px golden depends on the Go toolchain version: it passes on the pinned 1.25.0 and differs on 1.27.1, so it will go red when CI's Go pin moves. Neither is in this brief's scope.

**VERIFY: BLOCKED** — no implementation defect found. Rows 2, 5, 6, 7, 9 and 10 are genuinely proven by the witness. Row 1 fails for environment reasons outside this brief's packages. Row 4 is a vacuous pass on a stale literal (16 vs the actual 6). Row 8 fails under pipefail while printing the expected `0`. Row 11 could not check on merged main by design and was corroborated manually. Row 3 is a reviewer-read row that holds. gate: human with `sensitive-data: yes`: Evidence only, status stays `implemented`, and human sign-off is owed, including the E4 open question above.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a write and the wrong identity performing it? (The
   resolver's custody binding.) Is it acceptable alone? (No — the backends' unminted-token
   refusal and the commit-identity preflight are the two independent layers behind it.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 11:
   with the custody binding yielding nothing, the backend itself refuses rather than falling
   through to the decoy ambient credential.)
