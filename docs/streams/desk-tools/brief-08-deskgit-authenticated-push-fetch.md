---
brief: desk-tools/08
title: "`deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file"
why: >-
  Every desk role that pushes a branch or refreshes a private remote today re-types the same
  inline git credential recipe — a shell-function credential helper that reads the role's
  0600 token file — because the shared checkout's ambient helper shadows per-worktree config
  and a token-in-URL is refused by policy. A 24-hour sweep of fifteen desk-role and worker
  session transcripts found one dispatch session retyping that recipe 46 times, once before
  nearly every board refresh. A recipe retyped 46 times is a recipe that will one day be
  retyped wrong — with the token on the command line, in the audit ledger, or against the
  wrong remote. `deskgit` already owns the desk's one sanctioned fetch and already closes the
  transport-exec vectors; giving it the authenticated form (and the push it deliberately
  lacked) retires the recipe and puts the token path behind the same fixed-argv guard.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskgit/` has `fetch` only (`main.go` usage block; `transportexec.go` refuses `receive-pack` by name with the note that deskgit never pushes); no `--as`/token path exists; `deskpr` pushes with a bare `git push -u origin <branch>` (`tools/desk/cmd/deskpr/deskpr.go` push sites) and relies on ambient credentials."
  - "The token-supply pattern this reuses rather than reinvents: `tools/desk/cmd/deskadvisory/advisory.go` § fetchAdvisoryTree / writeAskpass — an ephemeral 0700 `GIT_ASKPASS` script in a private temp dir, the token passed to the CHILD only via one env var, `-c credential.helper=` to silence the ambient helper, deskgit-grade hardening pins."
  - "Role → token-file resolution: `tools/desk/internal/deskkit/roletoken.go` — `RoleTokenForRepo` (per-OWNER installation tokens) and `SessionTokenRole` / `RequireLoopIdentity` (a session may act only as the role its loop identity binds)."
  - "deskgit's existing guards that push must inherit unchanged: fixed argv, `--upload-pack` pin, `--refmap` pin, env allowlist (`exec.go`), effective-origin-URL gate with the local-roots allowlist (`deskgit.go` parseRepo), the named-refusal table in `transportexec.go`."
  - "The outward-write budget every mutating desk verb takes: `tools/desk/internal/deskkit/ratelimit.go` (`AllowWrite`); the pre-push guard that must keep running under the new verb: `tools/desk/cmd/deskpushguard/`."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
exec-tier: strong
exec-tier-why: "(c) auth and safety plumbing — a token that reaches argv, the audit line or a
  temp file that outlives the call, an ambient helper consulted after all, or a refspec that
  can be widened, each survives every happy-path test and leaves the verb looking safe."
---

# Brief 08 — `deskgit push` / `deskgit fetch --as <role>`: authenticated transport from the role's token file

## Dependencies
None. `deskgit`, the role-token resolver, the askpass pattern and the outward-write budget
are all on main today; this brief composes them.

## Context

single-point-of-failure: **the fixed argv** — every transport-exec vector deskgit closes is
closed because nothing the caller passes can reach git's flag surface. Behind it stand two
independent layers this brief keeps intact rather than adds: the **env allowlist** (a config
injected through the environment never reaches the child) and the **effective-URL gate** (a
rewritten remote is refused before any credential is offered). Independence: the argv guard
fails on a parser change, the env scrub on a new `GIT_*` variable, the URL gate on an
`insteadOf` rewrite — different reasons, different code paths.

risk note — this brief handles a live credential and answers `sensitive-data: no`. The answers
stand and here is why, so the reviewer checks the reasoning: the token is READ from the 0600
file the role already owns and PASSED to one child process through one environment variable
and an ephemeral askpass script, exactly as `deskadvisory` does today; it is never written to
stdout, argv, the audit ledger, a cache, or any file that outlives the call. The verb widens no
credential's reach — the same token already authenticates the same role's pushes through the
retyped recipe — it only removes the recipe. A reviewer who finds a path by which the token can
leave the child process flips the answer to yes and takes the human gate.

files:
- `tools/desk/cmd/deskgit/main.go` (usage, `push` dispatch)
- `tools/desk/cmd/deskgit/deskgit.go` (`cmdPush`, the `--as` resolution shared with `cmdFetch`)
- `tools/desk/cmd/deskgit/exec.go` (askpass supply on the scrubbed env — one added variable, added AFTER the scrub)
- `tools/desk/cmd/deskgit/transportexec.go` (the `receive-pack` entry's comment: deskgit now pins it rather than merely refusing it)
- `tools/desk/cmd/deskgit/*_test.go` (fixture bare repos as local-path remotes, as the existing fetch tests do)
- `tools/desk/internal/forgeban/allowlist.go` (register the push exec site if the ledger tracks it)
- `tools/desk/README.md` (contract: the two authenticated forms, what the token never touches)

facts:
- today: `deskgit fetch [--prune] | --pr N | --branch B` only; no push; no credential path; the
  child env is the allowlist in `exec.go` with `GIT_ASKPASS` deliberately dropped (checked
  2026-09-02, `exec.go` env comment).
- token resolution: `deskkit.RoleTokenForRepo(role, repo)` returns (token, path, err); it is
  per-OWNER (`roletoken.go`), so the repo the effective origin URL names is the one the token
  is minted for — never a `--repo` the caller types.
- identity binding: `deskkit.SessionTokenRole(verb)` reads the loop identity; `--as <role>` MUST
  equal the role that identity binds, else exit 5. A session cannot borrow another role's
  token by naming it.
- askpass supply (the `deskadvisory` pattern, `advisory.go` § writeAskpass): private
  `os.MkdirTemp` dir, a 0700 `/bin/sh` script answering `x-access-token` to the Username prompt
  and `$DESKGIT_TOKEN` to everything else; env = scrubbed allowlist + `GIT_ASKPASS=<script>` +
  `DESKGIT_TOKEN=<token>`; argv carries `-c credential.helper=` so the ambient helper (the
  shadowing observed in the sweep) is never consulted; the temp dir is removed before return
  on every path including error.
- push contract (fixed argv, no caller flags): remote `origin` only; refspec exactly
  `refs/heads/<B>:refs/heads/<B>` where `<B>` is the CURRENT branch read from `symbolic-ref
  HEAD`, validated by the same rule `--branch` uses, and never `main`/`master` in any case;
  `--receive-pack=git-receive-pack` pinned (the push-side twin of the upload-pack pin);
  no `--force`, no `--delete`, no `-o`, no tags. Anything not in that argv is not reachable.
- push is an OUTWARD WRITE: it takes `deskkit.AllowWrite("deskgit", repo, 0)` and the audit
  line, unlike fetch (which takes only audit + kill switch — `main.go` header). The pre-push
  hook (`deskpushguard`, via `core.hooksPath`) still runs; deskgit passes no `--no-verify`.
- `fetch --as <role>` is `cmdFetch` with the askpass supply added; every existing fetch guard
  and mode is unchanged. Without `--as`, fetch behaves exactly as today.
- audit: the line records role, repo slug, branch, mode — never the token, never the token
  path's contents; the existing userinfo redaction (`deskgit.go` § parseRepo) applies to any URL
  echoed in a refusal.
- tests use LOCAL bare fixtures under a directory the local-roots allowlist admits (the
  existing `rosterfixture_test.go` harness); the askpass path is exercised by a fixture whose
  `credential.helper` config writes a canary file — the canary must NEVER appear.

## Ground rules
- Never push to a real remote from a test or a Verify row; fixture bare repositories only.
- Never print, log, or persist the token; never place it in argv or a URL.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`--as <role>` resolution** shared by both verbs: role must equal the session's bound
   token role (`SessionTokenRole`) else exit 5; token via `RoleTokenForRepo(role, <effective
   origin slug>)` else exit 6 naming the path searched — never the contents.
2. **Askpass supply** in `exec.go`: a function that returns (env, cleanup) — scrubbed allowlist
   plus the two variables above — and the argv prefix `-c credential.helper=`. The cleanup
   removes the temp dir; it runs on every return path.
3. **`deskgit push --as <role>`**: current-branch push per the facts (fixed argv, pinned
   receive-pack, main/master refused, no force/delete), gated on the effective origin URL
   exactly as fetch is, charged to the outward-write budget, audited.
4. **`deskgit fetch --as <role>`**: the existing fetch modes with the askpass supply; nothing
   else changes.
5. **Tests** (fixture-only): push advances the bare remote's ref; push refuses on `main`; push
   refuses a detached HEAD (no branch to name, exit 5); `--as` for a role the session is not
   bound to refuses; a fixture `credential.helper` canary never fires under `--as`; the token
   never appears in argv, stdout, stderr, or the audit line (grep the captured surfaces for
   the fixture token value); the askpass temp dir is gone after success AND after a forced
   failure; `--force`, `--delete`, `--receive-pack=` and `--no-verify` are refused by name
   BEFORE the FlagSet (extend `transportexec.go`'s table and its test).
6. **README contract**: the two authenticated forms, the fixed push argv, what the token never
   touches, and the statement that `deskgit push` is the sanctioned replacement for the inline
   credential-helper recipe.
7. **Nothing else.** No change to `deskpr`'s push (a follow-up may route it through this verb
   once verified); no new fetch modes.

## Verify

`-run` compiles an RE2 pattern, so each row names ONE test and rows chain with `&&`.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestPushAdvancesFixtureRemote$' -count=1` | exit 0 — the bare fixture's `refs/heads/<B>` equals the worktree HEAD after `push --as`, and the argv the runner saw is exactly the fixed form |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestPushRefusesMainAndDetachedHead$' -count=1` | exit 0 — both exit 5; the remote ref is untouched |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestAsRoleMustMatchSessionIdentity$' -count=1` | exit 0 — a role the loop identity does not bind exits 5 before any token is read |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestAmbientCredentialHelperNeverConsulted$' -count=1` | exit 0 — the NEGATIVE control: a repo-config `credential.helper` that writes a canary file; the canary does not exist after a `--as` push |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestTokenNeverLeavesTheChild$' -count=1` | exit 0 — the fixture token value is absent from argv, stdout, stderr and the audit line; the askpass dir is absent after success and after an injected failure |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestPushOptionsRefusedByName$' -count=1` | exit 0 — `--force`, `--delete`, `--receive-pack=x`, `--no-verify` each refused with their OWN reason, before the FlagSet |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskgit/ -run '^TestFetchAsRoleKeepsEveryFetchGuard$' -count=1` | exit 0 — `fetch --as` still pins upload-pack and refmap and still refuses a rewritten origin |
| 9 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole module, including any exec-site ledger check |
| 10 | check:ci | `gofmt -l tools/desk/cmd/deskgit > /tmp/dg-fmt.out; test ! -s /tmp/dg-fmt.out` | exit 0 |
| 11 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Token lands in argv or the audit line | row 6 |
| Ambient helper still consulted because `-c credential.helper=` was placed after the verb | row 5 |
| Refspec or remote widened by a later flag | row 7 (named refusals) + row 2 (exact argv) |
| Push on `main` allowed by a case difference | row 3 |
| Askpass temp dir leaks on the error path | row 6 (injected failure) |
| A session pushes as a role it is not bound to | row 4 |
| Existing fetch guards weakened by the shared env change | row 8 |
| README says "sanctioned replacement" but a skill still carries the recipe | review-only — skills are retired in a follow-up once this verb is verified |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer records verdict + date in the stream README
table and confirms: (1) the single control is the fixed argv, and rows 5 and 8 prove the env
scrub and the URL gate hold independently of it; (2) row 5 is a genuine negative-path row — the
ambient helper is ARMED and proven unconsulted, not merely absent; (3) no test or Verify row
contacts a real remote; (4) the risk note's claim that the token never leaves the child is what
row 6 measures, on both the success and the failure path.
