---
brief: assay:assay:desk-tools:18
title: "Public-repo write gate — an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check"
why: >-
  The public-repo write gate asks for a human `+1` reaction on a referenced issue or PR before
  ANY outward write to a public repository. That is unsatisfiable for the write that matters
  most: opening the first pull request. `deskpr create` has no PR number yet and passes the
  trailer's issue number, which is 0 for a brief-carrying PR — so the gate refuses, exit 6,
  every time, and no desk can ever open a PR on a public repo without a human first
  hand-creating a sentinel file. The check also costs an API call per write and expresses the
  authorization in the wrong unit: the human decision is "this repository is a place the desk
  may write", made once, not "this item is a thing the desk may write about", re-made forever.
  A draft PR is inert until a human merges it, so the per-item ceremony buys nothing the
  repository-level decision and the merge gate do not already buy.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief REMOVES a per-write human-in-the-loop control on public repositories and replaces
  it with a standing, repository-scoped authorization. After it, a desk tool holding a role
  App token can post to a listed public repo with no fresh human act at all. The replacement
  is a recorded maintainer ruling (2026-09-06) and it is narrower than it sounds — the write
  set is configured out-of-band, an unlisted public repo still refuses, and merge remains the
  human's — but the human signing this off is confirming exactly that: that an UNLISTED public
  repo still refuses (Verify row 4), that a live-public repo whose roster entry claims private
  refuses rather than passing on the stale claim (row 5), and that nothing on the private path
  changed (row 6).
design: DR-desk-tools-18
issues: []
schema: brief-v2
authored: 2026-09-06 by an authoring session, from a maintainer ruling recorded 2026-09-06
sources:
  - "Maintainer ruling, 2026-09-06: a public repo listed in the roster's allowed-repos set with the public tag passes ALL outward writes for trusted identities; an unlisted public repo refuses; no per-item reaction check. A draft PR is inert until a human merges it."
  - "freshness-checked 2026-09-06 @ 0af8093 (origin/main) — `tools/desk/internal/deskkit/repovis.go` § `PublicRepoGate` still fetches reactions and requires a `+1` from the blessing authority; the `issueNumber <= 0` arm still refuses outright; `tools/desk/cmd/deskpr/deskpr.go` still passes `trailerIssue` (0 for a brief-carrying create). The defect is live."
  - "The standing per-repo sentinel this brief retires: tools/desk/internal/deskkit/publicbless.go (deleted by this brief) and its documentation in `tools/desk/README.md`."
  - "The configured write set and its `:public` / `:private` tokens: `tools/desk/internal/deskkit/rosterconfig.go` § allowed repos, and `config.go` § `allowedRepos` / `RepoVisibility` / `VisibilityDrift`."
exec-tier: strong
exec-tier-why: >-
  (c) this is the choke point every write-capable desk verb calls — an error that opens it
  passes every happy-path test, because the happy path is "the write succeeded"; and (b) the
  change is correct only in relation to a second component's data (the configured write set)
  and a third's live read (the forge's visibility), which must be made to disagree on purpose
  before the design can be believed.
consumers:
  - "tools/desk/cmd/deskpost/{comment,review,ready}.go, tools/desk/cmd/deskpr/{deskpr,edit}.go, tools/desk/cmd/deskreply/exec.go, tools/desk/cmd/deskevidence/deskevidence.go, tools/desk/cmd/deskrelease/cut.go: call sites of the changed signature — each drops the now-removed issue-number argument and is otherwise unchanged. Behaviour reaches them through the single choke point, not through per-site edits: fixed-here."
  - "~/.config/assay/public-app-ok (the standing-bless sentinel): retired by this brief — the reader is removed, so the file stops having any effect. Operators move each listed repo into the allowed-repos set with the `:public` token. The file itself is the operator's and is never written or deleted by any tool. (Non-conforming routing token, deliberately: this is neither `fixed-here` nor a follow-up brief — it is a retirement, and saying so truthfully outranks fitting the grammar.)"
  - "tools/desk/README.md § allowed repos, § standing per-repo authorization: follow-up in this brief's own Task step 5."
version: 1
id: 9d346af4-6df6-45cc-9f3e-4e983ee5ad0e
---

# Brief 18 — Public-repo write gate: allowed-repos `:public` replaces the per-item `+1`

## Dependencies
None. Independent of desk-tools/17, which changes an AUTHOR-trust predicate on the read/
classify path; this changes the OUTWARD-WRITE gate. They touch different files and neither
reads the other's decision.

## Context

files:
- `tools/desk/internal/deskkit/repovis.go` (`PublicRepoGate` rewritten; `RepoInfoFetcher`
  narrowed; `Reaction` / `ReactionUser` and the reactions HTTP call removed)
- tools/desk/internal/deskkit/publicbless.go (retired — deleted by this brief) and its test
- `tools/desk/internal/deskkit/repovis_test.go` (or the package's gate tests)
- the call sites listed in `consumers:` (signature update only)
- `tools/desk/README.md` (§ allowed repos, § standing per-repo authorization)

facts (all read at `0af8093`, 2026-09-06):
- `PublicRepoGate(fetcher, owner, repo, issueNumber)` today: reads LIVE visibility → `private`
  returns nil → `public` / `internal` fall through → a sentinel-file bless returns nil with a
  stderr NOTICE → `issueNumber <= 0` refuses (exit 6) → otherwise fetches reactions and
  requires a `+1` whose author is the blessing authority by login AND pinned numeric id →
  no match refuses (exit 5).
- The gate is the SINGLE choke point: every write-capable verb (create, edit, comment, review,
  ready, reply, evidence, release cut) calls it. That is why the change belongs here and not
  at eight call sites.
- `deskpr create` passes `trailerIssue`, which is the `Issue: #<N>` number or 0 for a
  brief-carrying trailer. On a public repo with no sentinel, a brief-carrying create can
  therefore never pass — the first PR on a public repo is unopenable by the tool.
- The allowed-repos set is `owner/name[:ci|:no-ci][:public|:private]`, configured from outside
  every ref the tools evaluate. `RepoVisibility(repo)` reads the CONFIGURED value and answers
  `VisibilityUnknown` for a repo that is absent, or present with no visibility token, or
  matched only by an `owner/*` PATTERN entry (patterns carry no policy — they widen
  `IsAllowedRepo` alone). An unconfigured set answers false/unknown for everything.
- `FetchRepoVisibility`'s doc already states the deliberate split the new gate depends on:
  risk-classing reads the CONFIGURED value so it cannot fail open when the forge is
  unreachable; the write gate reads the LIVE value so a repo flipped to public after the set
  was written is still gated. Both halves are now load-bearing at once — the new gate requires
  the live read AND the configured claim to agree.
- Exit-code vocabulary in this package: `Unverifiable(...)` → exit 6 (could not check),
  `Refused(...)` → exit 5 (refused by constraint).
- The blessing authority's `+1` on an item, and the sentinel file, are the two things this
  brief removes. The item-level AUTHOR-trust gate (`trustGate` / `prTrustGate` in `deskpost`)
  is a DIFFERENT control on a different signal and is not touched.

single-point-of-failure: after this change the ONE control standing between a desk tool and an
outward write to a public repository is the configured allowed-repos entry carrying `:public`.
Layers behind it, each failing on a different signal in a different component: (1) the LIVE
visibility read — a repo whose live visibility cannot be read, or is unrecognised, refuses
whatever the roster says, so a stale roster cannot authorize a write to a repo the forge
disagrees about; (2) the item-level author-trust / blessing gate in `deskpost`, which still
refuses a verdict on an unvetted third-party PR regardless of which repo it is on — an
identity signal, not a repository signal; (3) branch protection and human merge — every PR
this gate lets a tool open is a DRAFT and is inert until a human merges it; (4) the append-only
audit log, which records every write that passed, so an authorization that should not have been
granted is discoverable after the fact rather than invisible. The roster is configured from
outside every ref the tools evaluate, so no pull request can add its own repository to the set.

## Ground rules
- NEVER git push, trigger workflows, or contact the forge from a test or a Verify row. The
  gate's tests use the package's stub fetcher.
- Stop at `implemented` — you do not set verified/done.
- Do NOT widen `IsAllowedRepo`, and do NOT add any flag, environment read, or CLI argument
  that can admit a repo at call time. The set stays configured out-of-band; a knob here would
  be a waiver on a security gate.
- Do NOT delete or rewrite any operator's sentinel file from code. Retiring the mechanism
  means removing the READER, nothing else.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Narrow the interface.** Remove `IssueReactions` from `RepoInfoFetcher`, delete the
   `Reaction` / `ReactionUser` types, the `HTTPRepoInfoFetcher.IssueReactions` method and the
   stub's implementation. `RepoInfoFetcher` keeps `RepoVisibility` alone.
2. **Rewrite the gate** as `PublicRepoGate(fetcher RepoInfoFetcher, owner, repo string) error`
   — the issue-number parameter is REMOVED, not defaulted, so the compiler finds every call
   site and the unsatisfiable "issue 0" state cannot recur:
   - live visibility read fails → `Unverifiable` (exit 6), message unchanged in spirit;
   - `private` (trimmed, case-insensitive) → nil;
   - `public` / `internal` → the repo MUST have an EXPLICIT allowed-repos entry whose
     configured visibility is public (`RepoVisibility(owner+"/"+repo) == VisibilityPublic`).
     If it does, return nil. If it does not — absent, pattern-matched only, tagged `:private`,
     or carrying no visibility token — return `Refused` (exit 5) with a message naming the
     repo, what was read live, what the set says, and the exact remedy (add
     `owner/name:public` to the allowed-repos configuration);
   - anything else → `Unverifiable` (exit 6), unchanged.
   Keep the allowlist-not-denylist shape and the comment explaining why `internal` is gated
   like `public`.
3. **Retire the sentinel.** Delete `publicbless.go`, its test, and the notice writer, once
   `grep -rn 'publicRepoBlessed\|PublicBlessSentinelName\|public-app-ok' tools/` is empty.
4. **Call sites.** Update the eight call sites to the new signature. No other change at any
   of them; in `deskpr` the `trailerIssue` value stays where it is used for the trailer and is
   simply no longer passed to the gate.
5. **Docs.** `tools/desk/README.md`: replace the `+1` bullet and the whole standing-per-repo-
   authorization block with the new rule, including the migration sentence for an operator who
   has a sentinel file today (move each line into the allowed-repos configuration as
   `owner/name:public`). State plainly that the authorization is repository-scoped and covers
   every write verb, and that merge remains the human's.
6. **Tests** (stub fetcher, no network) — a table over (live visibility × configured entry):
   - listed public repo, live `public` → PASS, for a create-shaped call (no issue number
     exists in the signature any more), a comment-shaped call and a verdict-shaped call;
   - listed public repo, live `internal` → PASS;
   - UNLISTED public repo → refused, exit 5, message names the remedy;
   - pattern-only (`owner/*`) match, live `public` → refused, exit 5;
   - listed `:private` entry but live `public` (roster drift) → refused, exit 5;
   - listed `:public` entry but live `private` → PASS (the private arm, unchanged);
   - live visibility unreadable → exit 6; live visibility unrecognised → exit 6;
   - unconfigured roster, live `public` → refused (nothing is listed);
   - a `deskpr` test proving a brief-carrying create on a listed public repo passes the gate
     seam, which is the defect this brief closes.
7. **Changelog fragment** under `changelog/` (`### Changed`, plus one line under `### Removed`
   for the sentinel).
8. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — every call site compiles against the narrowed signature |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateListedPublicPasses$' -count=1` | exit 0 — a listed `:public` repo passes for create-, comment- and verdict-shaped calls |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskpr/ -run '^TestBriefCarryingCreateOnListedPublicRepoPassesGate$' -count=1` | exit 0 — the create path no longer refuses for want of an issue number |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateUnlistedPublicRefuses$' -count=1` | exit 0 — the NEGATIVE control: unlisted, pattern-only and unconfigured all refuse with exit 5 |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateRosterDriftRefuses$' -count=1` | exit 0 — live `public` against a `:private` entry refuses; the stale claim never authorizes |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGatePrivateAndUnreadable$' -count=1` | exit 0 — private passes, unreadable and unrecognised visibility exit 6 |
| 7 | check:ci | `grep -rn -e 'publicRepoBlessed' -e 'PublicBlessSentinelName' -e 'public-app-ok' -e 'IssueReactions' tools/ > /tmp/b18-residual.out; test ! -s /tmp/b18-residual.out` | exit 0 — the retired sentinel and the reactions probe leave no reference behind |
| 8 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole suite, including every call site's own tests |
| 9 | check:ci | `gofmt -l tools/desk/internal/deskkit tools/desk/cmd > /tmp/b18-fmt.out; test ! -s /tmp/b18-fmt.out` | exit 0 |
| 10 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 11 | check +mutation | **Mutation demonstration for the public-repo write gate this brief adds.** In `PublicRepoGate` (`tools/desk/internal/deskkit/repovis.go`) change the public/internal arm's authorization check `if RepoVisibility(owner+"/"+repo) == VisibilityPublic {` to `if true {` — the fail-open shape in which every live-public/internal repo passes whether or not the allowed-repos set lists it — then `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateUnlistedPublicRefuses$' -count=1`; restore the file and re-run | exit **1** on the mutant: the negative control fails on all three subtests (unlisted, pattern-only, unconfigured each return nil instead of Refused/exit 5), exit **0** again after restoring. This is the row that proves the single control the design rests on reddens when the guarded thing is broken, rather than passing because nothing exercises it |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The gate is removed rather than replaced — every public repo passes | row 4, the negative control |
| The configured value is consulted INSTEAD of the live read, so a repo flipped to public after the set was written passes on a stale claim | row 5 |
| The live read is consulted but its failure is treated as "not public" and passes | row 6 |
| A pattern entry (`owner/*`) is read as carrying `:public` policy and silently authorizes a whole owner | row 4 (pattern case) |
| The private path regresses while the public path is rewritten | row 6 + row 8 |
| The issue-number parameter is kept and defaulted to 0, so the unsatisfiable state survives in some caller | row 1 (the parameter is gone; a caller passing it does not compile) + row 3 |
| The sentinel reader is left in place, so two authorization paths coexist and only one is documented | row 7 |
| The README still promises a `+1` that no longer runs | row 7 covers code references; prose adequacy stays review-only |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

Non-implementer pass against merged main 893cd6114b0382a1f71e6ef763c6601a5e19d270 (implementing merge d739495fc, #812). Execution witness first (statusgen verifyrun, rows verbatim), then the direct re-run of every row on the same tree.
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateListedPublicPasses$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskpr/ -run '^TestBriefCarryingCreateOnListedPublicRepoPassesGate$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateUnlistedPublicRefuses$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateRosterDriftRefuses$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGatePrivateAndUnreadable$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -rn -e 'publicRepoBlessed' -e 'PublicBlessSentinelName' -e 'public-app-ok' -e 'IssueReactions' tools/ > /tmp/b18-residual.out; test ! -s /tmp/b18-residual.out` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 9 | `gofmt -l tools/desk/internal/deskkit tools/desk/cmd > /tmp/b18-fmt.out; test ! -s /tmp/b18-fmt.out` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 11 | `PublicRepoGate` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:dbb1360f5a1e | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |

The witness above is the tool's own record: every check:ci row is could-not-run because the hermetic runner needs Linux `unshare --net` and this host is darwin; its row 11 took the first backticked span of a prose mutation row (the identifier PublicRepoGate) as the command, hence exit 127. Every row was therefore also executed directly, non-hermetic (network not isolated), at the same head:

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/desk && go build ./... && go vet ./... | exit 0 | exit 0, no output — every call site compiles against the three-argument gate | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateListedPublicPasses$' -count=1 -v | exit 0 | exit 0 — --- PASS: TestPublicRepoGateListedPublicPasses, subtests public_create, public_comment, public_verdict, internal_listed_public_passes, public_recased_still_passes | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | cd tools/desk && go test ./cmd/deskpr/ -run '^TestBriefCarryingCreateOnListedPublicRepoPassesGate$' -count=1 -v | exit 0 | exit 0 — --- PASS: TestBriefCarryingCreateOnListedPublicRepoPassesGate (5.56s) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateUnlistedPublicRefuses$' -count=1 -v | exit 0 | exit 0 — --- PASS: TestPublicRepoGateUnlistedPublicRefuses, subtests unlisted, pattern_only, unconfigured_set | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateRosterDriftRefuses$' -count=1 -v | exit 0 | exit 0 — --- PASS: TestPublicRepoGateRosterDriftRefuses (configured :private, live public, Refused exit 5) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGatePrivateAndUnreadable$' -count=1 -v | exit 0 | exit 0 — --- PASS: TestPublicRepoGatePrivateAndUnreadable, subtests private_passes, private_case_and_whitespace, visibility_read_error_exit6, visibility_empty_exit6, visibility_unrecognised_exit6 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | grep -rn -e 'publicRepoBlessed' -e 'PublicBlessSentinelName' -e 'public-app-ok' -e 'IssueReactions' tools/ > /tmp/b18-residual.out; test ! -s /tmp/b18-residual.out | exit 0 | FAIL exit 1 — 17 residual lines, all IssueReactions (sentinel names: zero hits). Two are a residual of the retired reactions probe itself: tools/desk/cmd/deskpost/forgeclient.go:28 still documents the embedded fetcher as "RepoVisibility / IssueReactions — the public-repo +1 gate's surface", and forgeclient.go:323 keeps a forgeBackend IssueReactions method with no non-test caller. Both were present at the implementing merge d739495fc. The other 15 are the Forge interface's IssueReactions (forge.go:1412, forge_github.go, forge_gitlab.go and their tests), which the implementing commit message retains on purpose as the separate admission surface; the row's grep is wider than Task 1's scope there | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | cd tools/desk && go test ./... -count=1 | exit 0 | FAIL exit 1 — 83 packages ok, 2 FAIL: cmd/commsloop (TestRunDoesNotBusySpinOnEmptyQueue: loopengine.Run did not stop within deadline) and internal/loopengine (6 tests, engine did not stop within deadline). Both re-run in isolation: go test ./cmd/commsloop/ ./internal/loopengine/ -count=1 exit 0. Load-induced timing class, #612; neither package is touched by this brief. Every brief-18 package is ok: internal/deskkit, cmd/deskpr, cmd/deskpost, cmd/deskreply, cmd/deskevidence, cmd/deskrelease | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | gofmt -l tools/desk/internal/deskkit tools/desk/cmd > /tmp/b18-fmt.out; test ! -s /tmp/b18-fmt.out | exit 0 | FAIL exit 1 — 7 files listed (deskkit forge_writefile_test.go and repairobligation.go; cmd deskboard classdegrade_test.go and inventory_test.go, deskclose authority.go, deskmigrate deskmigrate_test.go, deskrebaseline main.go), none touched by the implementing merge; gofmt -l over the brief's own surviving Go files exits 0 with no output. The row was already red at d739495fc (8 unformatted files under the same dirs), so it scopes directories, not the brief's diff | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | cd statusgen && go run . --root .. --lint; echo $? | 0 | exit 0 — LINT: PASS then 0; zero lines begin with PROBLEM | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | mutate repovis.go:123 to if true {, then cd tools/desk && go test ./internal/deskkit/ -run '^TestPublicRepoGateUnlistedPublicRefuses$' -count=1 -v; restore and re-run | exit 1 on the mutant, exit 0 restored | mutant exit 1 — all three subtests FAIL: unlisted, pattern_only, unconfigured_set each "got <nil>, want Refused/exit 5"; restored (git diff clean) exit 0, ok | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Tally: 8 of 11 rows pass on the direct run (1, 2, 3, 4, 5, 6, 10, 11); rows 7, 8, 9 fail. Row 7 is partly an implementation residual (the deskpost forgeBackend IssueReactions method and its "+1 gate" comment) and partly a check that is wider than Task 1; rows 8 and 9 fail outside this brief's code (#612 flake, gofmt drift across the whole directory).

Review questions (gate: human): (1) The single control is the explicit allowed-repos entry tagged :public, compared at repovis.go:123. The layers behind it fail on other signals: the live visibility read (forge, per call), deskpost's author-trust gate (identity), draft plus human merge (branch protection), and the audit log. (2) Yes, on the fixed tests: row 6's visibility_read_error_exit6 and visibility_unrecognised_exit6 run against example-org/pubrepo, which the test roster lists as ci:public, and still return exit 6. The lower layer (the live read) refuses while the upper layer (the roster) says pass. Row 5 proves the opposite direction: the roster says :private, the live read says public, and the gate refuses.

RISK-VALUE (kit §4 — enumerated over the non-test Go of merge d739495fc: repovis.go, deskpr.go and edit.go, deskrelease cut.go, deskpost comment/ready/review/github.go, deskreply, deskevidence; plus the Deliverables' named config and exit codes). Entries: the authorization comparand RepoVisibility(owner+"/"+repo) == VisibilityPublic @ tools/desk/internal/deskkit/repovis.go:123; the skip literal case "private" @ repovis.go:112; the gated set case "public", "internal" @ repovis.go:114; the roster token case "public" → pol.Visibility = VisibilityPublic @ tools/desk/internal/deskkit/rosterconfig.go:1405, with pattern entries recorded in RepoPatterns only @ rosterconfig.go:1389-1392 and RepoVisibility reading the explicit table only @ tools/desk/internal/deskkit/config.go:174-179; ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:27; ExitUnverifiable = 6 @ exitcodes.go:31; EnvAllowedRepos = "ASSAY_ALLOWED_REPOS" @ rosterconfig.go:91. Removed by the diff, and checked absent by row 7: "+1", User.Type == "User", per_page=100, issueNumber <= 0, and the sentinel filename. Ranked by irreversibility: a public comment, review or PR is world-readable once posted, so a wrong authorization value cannot be undone by a redeploy. That puts the comparand, the private skip, the gated set and the token parse first. The exit codes and the env name are fixed vocabulary, reversible, and last.

- RISK-VALUE: DERIVED — RepoVisibility(owner+"/"+repo) == VisibilityPublic @ tools/desk/internal/deskkit/repovis.go:123 — the 2026-09-06 ruling says "listed with the public tag passes, unlisted refuses". RepoVisibility is a lookup in the explicit Repos table (config.go:174-179), and it answers VisibilityUnknown for an absent repo, a pattern-only repo or an untagged repo, so equality with VisibilityPublic is exactly "explicitly listed :public". Row 11 shows the tests catch the fail-open replacement.
- RISK-VALUE: DERIVED — case "private" (trimmed, lower-cased) @ repovis.go:112 — the only forge visibility whose content is not readable outside the repo's own grants. The allowlist shape makes every other string, including re-cased, padded and unknown ones, either need the entry or exit 6. Row 6 covers it.
- RISK-VALUE: DERIVED — case "public" → VisibilityPublic @ rosterconfig.go:1405, with pattern entries carrying no policy (rosterconfig.go:1389-1392) — the configured token is an exact, case-insensitive, trimmed match, and an owner/* pattern never enters the policy table. That matches the brief's fact that patterns widen IsAllowedRepo alone. Row 4's pattern_only covers it.
- RISK-VALUE: NAMED, NOT DERIVED — case "public", "internal" @ tools/desk/internal/deskkit/repovis.go:114, authorized through == VisibilityPublic @ repovis.go:123 — the 2026-09-06 maintainer ruling in the brief's sources speaks only of public repos. The rule that a live-internal repo is authorized by a :public entry comes from the implementer-authored DR-desk-tools-18 and the brief's Task 2, not from the ruling. It also conflicts with the drift check: VisibilityDrift (config.go:214-249) parses only "public" and "private", so every internal repo tagged :public to authorize writes is reported as unrecognised-visibility drift on every run. OPEN QUESTION for the human gate: should a live-internal repository be write-authorized by an allowed-repos :public entry (current behaviour), refuse until it carries a token of its own, or keep :public with the drift check taught that internal-under-:public is expected?
- RISK-VALUE: DERIVED — ExitRefused = 5 @ exitcodes.go:27 and ExitUnverifiable = 6 @ exitcodes.go:31 — the package's fixed vocabulary, which the brief's facts restate: an unlisted repo is a constraint refusal (5), and an unreadable live read is could-not-check (6). This is reversible and ranked last.

VERIFY: FAIL — rows 7, 8, 9 on the direct run. Status stays implemented (gate: human; Evidence only, no flip).

## Review

Gate: human. The reviewer answers two questions in the verdict: (1) what is the single control
now standing between a desk tool and a public write, and are the layers named in the Context's
single-point-of-failure note independent of it — do they fail on different signals, in
different components? (2) does any Verify row prove a LOWER layer catches the fault with the
UPPER layer bypassed — specifically, does row 5 prove the live read still refuses when the
roster entry (the new upper layer) says pass? A table on which every case is a listed, live-
public repo has verified the pass and nothing else.
