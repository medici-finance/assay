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

## Review

Gate: human. The reviewer answers two questions in the verdict: (1) what is the single control
now standing between a desk tool and a public write, and are the layers named in the Context's
single-point-of-failure note independent of it — do they fail on different signals, in
different components? (2) does any Verify row prove a LOWER layer catches the fault with the
UPPER layer bypassed — specifically, does row 5 prove the live read still refuses when the
roster entry (the new upper layer) says pass? A table on which every case is a listed, live-
public repo has verified the pass and nothing else.
