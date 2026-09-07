---
brief: forge-neutral/04
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
schema: brief-v1
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
| 8 | `grep -rn -e 'apiBaseURL' -e 'access_tokens' tools/desk/cmd/deskevidence --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — the hardcoded host and the hand-rolled installation exchange are gone, not merely unused |
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
