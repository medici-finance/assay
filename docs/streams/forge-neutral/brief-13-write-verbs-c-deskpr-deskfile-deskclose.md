---
brief: assay:assay:forge-neutral:13
title: Write verbs C — deskpr, deskfile and deskclose onto the resolver
why: >-
  These three carry the last of the fleet's ambient-credential outward writes — opening a
  change, filing an issue, and closing one. Brief 04 was ruled down to its deskevidence slice
  (#509) precisely because these three keep `gh` calls with no enumerated forge op, so routing
  them through the resolver is a two-part move no single earlier brief could make: first add
  the four operations they still lack, then re-seat each verb onto the resolver — which changes
  WHICH identity performs the write. This is where the identity-class blocker in the permit
  register is answered instead of moved, and it is the last step before a brief can round-trip
  on GitLab with zero hand-built API calls.
wave: 3
depends: ["forge-neutral/04"]
unblocks: ["forge-neutral/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-neutral-13
issues: [274, 395, 509, 687, 691, 775]
schema: brief-v2
authored: 2026-09-10 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-04-write-verbs-issues-and-evidence.md — the amendment (#509) that rescoped these three OUT of 04 and named this follow-on, and the ForgeFor + SetGitHubCustodyMinter custody precedent this brief reuses"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver (ForgeFor / ResolveForge) and its refusal contract"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op table (32 rows on current main); this brief appends rows 33–36"
  - "tools/desk/internal/forgeban/allowlist.go:64 — allowedInvocationCeiling = 12 at authoring base 58b87de; the deskclose/deskfile/deskpr permit rows this brief retires"
  - "freshness-checked 2026-09-10 @ 58b87de — deskpr shells gh through runCmd (exec.go:87; create at deskpr.go, existing-PR/mergeable reads via `gh pr list`/`gh pr view`, `deskpr edit` body replace at edit.go:251); deskfile shells gh (exec.go:50; dedupe `gh search issues` in matcher.go, `gh label list` probe at deskfile.go:878, `gh issue create`/`gh issue comment`/`gh issue view`); deskclose runs gh under the AMBIENT identity (exec.go:48; authority `gh api` reads, `gh issue comment`, `gh pr close`/`gh issue close`, `gh api graphql viewer{login}` whoami, comment pagination, supersession `label list`/`label create`/`--add-label`)"
  - "#691 (MERGED) — deskfile's interim named-refusal on a GitLab repo (ForgeKindFor / requireSupportedForge) and the three gaps it names as blocking the full port: no text-search-issues op, no list-labels op, and GetIssue carrying no URL"
exec-tier: strong
exec-tier-why: "it changes WHICH identity performs three writes that today run under an ambient credential, and it re-homes deskfile from a no-token-ever design onto App custody — a subtle error here writes as the wrong actor, or leaves a `--as-app=false` fallback that the token-refusing backends silently cannot serve. Four backend ops with negative-path contracts on both forges, plus a custody re-seat, is not a mechanical port."
gate-why: >-
  `deskclose`, `deskfile` and `deskpr` reach the forge under an ambient CLI credential by
  documented design — `deskfile` states it mints no App token on any path, and `deskclose`'s
  exec.go states it "gates WHETHER and WHAT, never WHO". Routing them through the resolver
  changes WHO performs the write, which the permit register explicitly calls a token-custody
  decision and not a transport change. The human confirms, per verb, the acting identity this
  brief proposes: deskpr → the session-role worker App (its `--as-app=true` default path,
  ambient fallback retired); deskclose → the session-role App (DESK_LOOP-selected); deskfile →
  the session-role App, with `--raised-by` staying a body/label attribution as it is today (the
  alternative — minting the raised-by role's own App — is named in Task 4 for the human to rule
  on). #509 authorized this follow-on to perform the migration; this gate confirms the identity
  each verb assumes, which is a security judgment no model self-certifies.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/forge.go: fixed-here (four ops added to the frozen seam — branch→change lookup, change body/title edit, issue text-search, list-labels — both backends)"
  - "tools/desk/cmd/deskpr: fixed-here (routed onto ForgeFor; `--as-app=false` ambient fallback retired)"
  - "tools/desk/cmd/deskfile: fixed-here (routed onto ForgeFor; the interim GitLab named-refusal from #691 superseded now the backend serves GitLab; could-not-check refusal on an unresolvable forge retained)"
  - "tools/desk/cmd/deskclose: fixed-here (routed onto ForgeFor; the viewer-login whoami read replaced by the minted role's known login)"
  - "docs/streams/forge-gitlab/inventory.md: fixed-here (rows 33–36)"
  - "tools/desk/cmd/deskpushguard: out-of-scope (its `fetchPR` branch→change lookup can adopt the op this brief adds, but its identity/no-op permit row is a separate follow-up not owned by this brief and is NOT retired here)"
  - "docs/streams/forge-gitlab/brief-05-live-pilot-parity-walk.md: out-of-scope (downstream beneficiary — the live pilot's verbs-only round trip, which pilot-report §2 records running on hand-built curl for want of a verb, needs these three verbs' GitLab backends; the typed in-stream edge that proves the property first is the unblocks on forge-neutral/10)"
version: 1
id: 20be4cba-88b9-4210-8715-588b25f983f8
---

# Brief 13 — Write verbs C: deskpr, deskfile, deskclose onto the resolver

## Why this is brief 13 and not "04b"

Brief 04's amendment named this follow-on `04b`. It is filed here as the next free integer in
the stream (**13**; briefs 01–12 are taken) because a sharded id like `04b` is rejected by the
PR-trailer parser that reads a brief's `brief:` line. The on-disk id is `13`; the *role* it
plays is the `04b` that brief 04's `consumers:` and the stream README's ratchet ledger point at.

## Context

This is the code-aware follow-on that brief 04 was ruled down to leave undone. Brief 04
(`#509`, Option C) delivered only its `deskevidence` slice and established that the migration's
premise — that it lowers the forge-CLI ratchet — was **false in scope for these three verbs**,
because each keeps `gh` calls with no enumerated `Forge` method. The ruling: the
`deskpr`/`deskfile`/`deskclose` migration becomes this brief, which **first adds the enumerated
ops each still lacks — a branch→change lookup, PR body/title fields, an issue-search op, a
label-list op — then re-seats the three verbs**, and only then retires their permit rows.

files:
- `tools/desk/cmd/deskpr/exec.go`, `tools/desk/cmd/deskpr/deskpr.go`, `tools/desk/cmd/deskpr/edit.go`
  — draft-change creation, the existing-PR-for-branch check, the mergeable read, and `deskpr edit`'s
  body replace.
- `tools/desk/cmd/deskfile/exec.go`, `tools/desk/cmd/deskfile/deskfile.go`,
  `tools/desk/cmd/deskfile/matcher.go` — the dedupe search, the label-existence probe, the create,
  the attach comment, and the interim `requireSupportedForge` / `ForgeKindFor` gate from `#691`.
- `tools/desk/cmd/deskclose/exec.go`, `tools/desk/cmd/deskclose/authority.go`,
  `tools/desk/cmd/deskclose/github.go`, `tools/desk/cmd/deskclose/superseded.go` — the authority
  reads, the close, the supersession comment scan, the supersession label ensure, and the
  `viewer{login}` whoami.
- `tools/desk/internal/deskkit/forge.go`, `tools/desk/internal/deskkit/forgeresolve.go` — the
  frozen seam (four ops added) and `ForgeFor` + `SetGitHubCustodyMinter`.
- `tools/desk/internal/forgeban/allowlist.go` — three permit rows removed, ceiling lowered by 3.
- `docs/streams/forge-gitlab/inventory.md` — the frozen op table, rows 33–36 appended.

single-point-of-failure: for all three verbs the single control is WHICH token the resolver's
custody binding hands the backend — get it wrong and the write lands as an identity nobody
chose. Two independent layers stand behind it: the backends refuse an unminted token outright
(`forge_github.go`, `forge_gitlab.go`), and the commit-identity / actor preflight independently
compares the resulting actor against the roster entry — a credential fault and an actor fault
trip different checks in different components. The ambient-fallback flags are the decoy the
lower layers must catch: with the custody binding yielding nothing, the correct outcome is a
refusal, never a fall-through to whatever credential happens to be active.

facts:
- The permit register's identity class covers exactly these three rows:
  `deskclose/exec.go::runGH::gh`, `deskfile/exec.go::gh::gh`, `deskpr/exec.go::gh::gh`
  (`allowlist.go`). `allowedInvocationCeiling = 12` at authoring base `58b87de`.
- **`deskclose` needs NO new op.** Its authority reads map to `GetIssue` (op 2, which carries the
  author account the blessing-authority check compares), its close to `CloseIssue` (op 13), its
  comment to `PostComment` (op 9), its supersession comment scan to `ListComments` (op 17), and
  its supersession label ensure (`label list` → `label create` → `--add-label`) to `ApplyLabels`
  (op 19, whose ensure step folds exactly that list-then-create pattern). Its one read with no
  enumerated op — `gh api graphql { viewer { login } }` — is a whoami: on the App path the acting
  login is the minted role's known login (`RoleAppLogin(role)`, the `deskevidence` precedent), so
  the read is **replaced by the identity layer, not a new forge op**. deskclose's blocker is
  therefore purely identity, exactly as its permit row states.
- **`deskpr` needs two new ops.** `gh pr create --draft` already maps to `CreateDraftChange`
  (op 8) and `gh pr view <number>` to `GetPullRequest` (op 1). What has no op: resolving an OPEN
  change from a source-branch NAME (deskpr's `gh pr list` existing-PR check and `warnIfConflicting`'s
  `gh pr view <branch>` — every interface read is keyed by number), and replacing a change's own
  body/title text (`deskpr edit`'s `gh pr edit --body`). `deskpr` DOES mint a worker token on its
  `--as-app=true` default path but ships a documented `--as-app=false` ambient fallback the
  token-refusing backends cannot serve; retiring that flag is the behaviour change its callers see.
- **`deskfile` needs two new ops** and is the biggest identity change. It mints no App token on any
  path today — a stated, load-bearing property (`#691`). `gh issue create` maps to `FileIssue`
  (op 12), `gh issue comment` to `PostComment` (op 9), `gh issue view` to `GetIssue` (op 2). What
  has no op: the dedupe **text-search** over issues (`gh search issues`, `matcher.go`) and the
  **label-existence probe** (`gh label list`, `deskfile.go`), which must report whether a label
  exists WITHOUT creating it — the deliberate opposite of `ApplyLabels`, because a missing label
  makes `deskfile` file UNSTAMPED rather than mint a label. `#691` also records that `GetIssue`
  carries no URL, which the dedupe/attach path needs.
- `#691` shipped the **interim** answer for `deskfile` on GitLab: `ForgeKindFor(repo)` resolves the
  forge KIND without taking on token custody, and `requireSupportedForge` refuses (exit 5, before
  any `gh` call) with a NAMED message on a repo affirmatively resolved to a non-GitHub forge —
  replacing GitHub's misleading "Could not resolve to a Repository" error. It explicitly deferred
  the two real fixes (route the ops through the backend; make the label probe forge-aware) for two
  reasons a worker "should not resolve unilaterally": the identity model (ambient → minted App is a
  human call) and the interface gaps above. This brief closes both, so the GitLab **refusal is
  superseded** — the backend now serves GitLab; the could-not-check refusal on an *unresolvable*
  forge is retained.
- `#274` reports that `forge-gitlab/07`'s call-site migration for these three verbs did not land,
  and that its grep-based Verify row passed vacuously (it grepped `exec.Command("gh")` while the
  tools shell `gh` through a `runCmd`/`runGH` wrapper). The ban test and Verify row 8 below are
  written against the wrapper form, closing that gap.
- The four new ops sit inside the frozen surface: none is a generic/passthrough or endpoint-taking
  method, and each is consumed by a call site **in this same change** (the §6 freeze rule is
  amended by this brief, not bypassed — the `forge-neutral/04` precedent).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. Do not touch `.github/workflows/*`.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green **unmodified**, except gh-argv / ambient-fallback
  assertions replaced by their forge-op equivalents (the `/03` precedent); new test files are
  expected.
- The surface stays closed: no generic/passthrough method, no caller-supplied endpoint, no new
  `gh`/`glab` shell-out. `forge-surface-control.yml`'s three controls stay green.
- Retiring `deskpr`'s `--as-app=false` and superseding `deskfile`'s GitLab refusal are the two
  behaviour changes in scope; both must be STATED in the PR body. Widening any escape hatch is not
  in scope.
- Refusal, never fallback: any path that cannot be served on the configured forge returns a
  `deskkit.Unverifiable` could-not-check naming the gap — never a silent GitHub call or a raw
  request.

## Task
1. **Add four ops to `Forge`, both backends, each consumed by a verb in this change.**
   - `OpenChangeForBranch(repo, branch)` — resolve the single OPEN change whose source branch is
     `branch`, or none. GitHub `GET /repos/{o}/{r}/pulls?head=…&state=open`; GitLab
     `GET /projects/:id/merge_requests?source_branch=…&state=opened`. More than one open change on
     one source branch is a could-not-check REFUSAL (ambiguous), not a silent first-match. Consumed
     by `deskpr` (existing-PR check + `warnIfConflicting`).
   - `EditChange(repo, number, EditChangeInput{Title, Body})` — replace a change's own title/body.
     GitHub `PATCH /repos/{o}/{r}/pulls/{n}`; GitLab `PUT /projects/:id/merge_requests/:iid`.
     Consumed by `deskpr edit`.
   - `SearchIssues(repo, SearchIssuesInput{Query})` — free-text dedupe search over a repo's issues,
     returning number, title, state, labels, and **URL**. GitHub `GET /search/issues?q=repo:o/r …`;
     GitLab `GET /projects/:id/issues?search=…`. Consumed by `deskfile` dedupe. Add the `URL` field
     to the shared issue result shape so `GetIssue` (op 2) carries it too (`#691`).
   - `ListLabels(repo)` — the repo's labels by name; **reads only, never creates** (the opposite of
     `ApplyLabels`). GitHub `GET /repos/{o}/{r}/labels`; GitLab `GET /projects/:id/labels`. Consumed
     by `deskfile`'s label-existence probe.
   Record all four in `docs/streams/forge-gitlab/inventory.md` (rows 33–36) with each row's GitLab
   mapping, and give each a both-backend golden contract case.
2. **Migrate `deskpr` onto the resolver.** Route create → `CreateDraftChange`, the existing-PR
   check + mergeable read → `OpenChangeForBranch` / `GetPullRequest`, and `deskpr edit`'s body
   replace → `EditChange`, all under `ForgeFor(fr, role)` with the `SetGitHubCustodyMinter` hook
   (the `/03` / PR #498 / `deskevidence` precedent). **Retire the `--as-app=false` ambient
   fallback**: the backends refuse an unminted token, so the fallback cannot be served and its
   flag/branch is removed (not merely defaulted off). Delete the `runCmd("gh", …)` path in
   `tools/desk/cmd/deskpr/exec.go`.
3. **Migrate `deskclose` onto the resolver.** Route the authority reads → `GetIssue` /
   `GetPullRequest`, the comment → `PostComment`, the close → `CloseIssue`, the supersession comment
   scan → `ListComments`, and the supersession label ensure → `ApplyLabels`, under
   `ForgeFor(fr, role)`. **Replace the `viewer{login}` whoami** with the minted role's known login
   from the identity layer (no new op). Delete the `runGH`/ambient-`gh` path in `tools/desk/cmd/deskclose/exec.go`.
   The blessing-authority model is unchanged — it is read from the item, not the acting identity;
   only WHO closes changes (ambient → App), which is the point.
4. **Migrate `deskfile` onto the resolver, and supersede the interim GitLab refusal.** Route the
   dedupe → `SearchIssues`, the label probe → `ListLabels`, the create → `FileIssue`, the attach →
   `PostComment`, the view → `GetIssue`, under `ForgeFor(fr, role)`. Because the backend now serves
   GitLab, `requireSupportedForge`'s **GitLab refusal is removed**; the could-not-check refusal on an
   *unresolvable* forge (no `ASSAY_REPO_FORGES` entry and an absent/unmapped origin) is **retained**.
   **Identity (the human-gate question):** the proposed acting identity is the **session-role App**
   (DESK_LOOP-selected, worker by default) minted via `desktoken`, with `--raised-by` remaining the
   body/label attribution it is today. The named alternative the human may choose instead: mint the
   `--raised-by` role's own App (author = the raising role), which requires that role's PEM to be
   reachable from the session. The brief SHIP-nothing here silently: whichever the human confirms,
   the label-missing behaviour (file UNSTAMPED, never mint the label) is preserved.
5. **Retire exactly three permit rows and lower the ceiling by 3.** Remove
   `cmd/deskclose/exec.go::runGH::gh`, `cmd/deskfile/exec.go::gh::gh`, `cmd/deskpr/exec.go::gh::gh`
   from `tools/desk/internal/forgeban/allowlist.go`, and lower `allowedInvocationCeiling` by 3 (**12 → 9** at authoring
   base `58b87de`; if the base has moved, the invariant is −3 from the base's value, not the literal
   9). No OTHER permit row changes. `deskpushguard`'s `fetchPR` may adopt `OpenChangeForBranch` but
   its row stays — its identity/no-op status is a separate brief.

## Verify (executable — no prose-only DoD items)

**Brief 13 is this stream's first `Class`-column Verify table** (desk ruling): a new brief follows
the convention of record at authoring time, and statusgen v1.0.3's mistake-proofing/03 Class-token
obligation is that convention now — `+dereference` marks a row that resolves a claim rather than
counting its presence, `+flow` a row that exercises the cross-component path (a write verb →
`ForgeFor` → the backend) end to end. Siblings 01–12 predate the convention and are grandfathered
(uplifted only when next touched), so this table carries the column and they do not — by ruling,
not oversight. These names below are this brief's planned test deliverables, created by the
implementer:
`TestDeskprRefusesWithoutMintedToken` (planned),
`TestDeskfileFilesOnGitLabThroughBackend` (planned),
`TestDeskfileRefusesWithoutMintedToken` (planned),
`TestDeskcloseClosesThroughBackend` (planned),
`TestDeskcloseActingLoginFromRoster` (planned),
`TestOpenChangeForBranchAmbiguousRefuses` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskpr/... ./cmd/deskfile/... ./cmd/deskclose/... -count=1` | exit 0 — the three migrated suites are green |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 — the seam grows four ops and stays closed (no generic/endpoint method, no extra exported backend method) |
| 4 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | exit 0 — `open_change_for_branch` / `edit_change` / `search_issues` / `list_labels` golden cases pin both backends' wire, and coverage reconciles the seam against the inventory (rows 33–36) |
| 5 | check | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows a value **3 lower than the base** (= 9 at `58b87de`) |
| 6 | check | `grep -c -e 'cmd/deskclose/exec.go::runGH::gh' -e 'cmd/deskfile/exec.go::gh::gh' -e 'cmd/deskpr/exec.go::gh::gh' tools/desk/internal/forgeban/allowlist.go` | prints `0` — all three permit rows are gone |
| 7 | check:ci | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 — the ratchet passes at the lowered ceiling |
| 8 | check | `grep -rn -e 'runCmd("gh"' -e 'runGH(' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — no verb shells `gh` through its wrapper any more (the `#274` grep-form gap, closed against the wrapper) |
| 9 | check | `grep -rn -e '--as-app=false' -e 'asApp' tools/desk/cmd/deskpr --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — the ambient fallback flag and its branch are removed, not merely defaulted off |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskpr/... -run TestDeskprRefusesWithoutMintedToken -count=1 -v` | **negative path**: with the custody binding yielding no token, `deskpr` REFUSES (non-zero) and performs no forge write — asserted by the recording fake forge showing zero calls; there is no ambient fall-through |
| 11 | check:ci +flow | `cd tools/desk && go test ./cmd/deskfile/... -run TestDeskfileFilesOnGitLabThroughBackend -count=1 -v && go test ./cmd/deskfile/... -run TestDeskfileRefusesWithoutMintedToken -count=1 -v` | exit 0 — POSITIVE: on a GitLab-configured repo `deskfile` FILES via the backend (the `#691` refusal is superseded); NEGATIVE: with no minted token the backend refuses, no ambient fallback |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestDeskcloseClosesThroughBackend -count=1 -v && go test ./cmd/deskclose/... -run TestDeskcloseActingLoginFromRoster -count=1 -v` | exit 0 — deskclose closes through `CloseIssue`, and its acting login comes from the minted role (not a `viewer{login}` forge read) |
| 13 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestOpenChangeForBranchAmbiguousRefuses' -count=1 -v` | **negative path**: two open changes on one source branch → a could-not-check REFUSAL naming the ambiguity, not a silent first-match |
| 14 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/13` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |
| 15 | check +mutation | **Mutation demonstration for the deskclose two-role close-authority gate this brief carries onto the resolver.** In `runSupersededLane` (`tools/desk/cmd/deskclose/superseded.go`, the `if who.role == roleWorker {` guard) disable the worker-half check — change it to `if false && who.role == roleWorker {`, the pre-guard shape in which a worker token drives the single-actor close reserved for the reviewer half — then `cd tools/desk && go test ./cmd/deskclose/... -count=1`; restore the file and re-run | exit **1** on the mutant: the deskclose suite fails (a worker token falls through to the reviewer half, collapsing the propose≠confirm two-role separation the re-seat must preserve), exit **0** again after restoring. Proves the close-authority control reddens when broken rather than passing because nothing exercises it. The same `./cmd/deskclose/...` suite exercises the `authority.go` `IsBlessAuthorityIDStrict` id-pin this brief extends (`ListComments` now carries the author's numeric id for it) |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A new op is added with no consuming call site, violating the freeze rule | row 4 + inventory rows 33–36 + the three verb suites (row 2) |
| The seam grows a generic/passthrough or endpoint-taking method behind the four ops | row 3 (`TestForgeNoPassthrough`) |
| A backend's wire behaviour drifts | row 4 (goldens pin the four ops per backend) |
| A verb still shells `gh` after the migration (the `#274` vacuous-grep gap) | row 8, written against the `runCmd`/`runGH` wrapper form, not `exec.Command` |
| `deskpr`'s `--as-app=false` is defaulted off but left reachable, so an ambient identity can still write | row 9 asserts the flag and its branch are GONE; row 10 asserts a no-token run REFUSES with zero forge calls |
| `deskfile` keeps refusing on GitLab instead of filing through the backend | row 11 POSITIVE asserts a real filing on a GitLab-configured repo |
| `deskfile` loses its could-not-check refusal on a genuinely unresolvable forge and silently calls GitHub | row 11 NEGATIVE + the freeze/refusal contract; a filing on an unresolvable forge would surface as a forge write the fake records |
| `deskclose` re-introduces a `viewer{login}` forge read instead of using the minted role login | row 12 (`TestDeskcloseActingLoginFromRoster` (planned)) |
| `OpenChangeForBranch` silently returns the first of several open changes on a branch | row 13 asserts a refusal on ambiguity |
| The ratchet is moved by the wrong amount, or the wrong rows are pulled | rows 5 + 6 + 7 (ceiling −3, the three named rows gone, forgeban green) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `48b978bb08c468fec52c015d280285698fc362bd` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer. gate: human + `sensitive-data: yes` — table RUN for Evidence; no model sign-off; status stays `implemented`.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | 0 | whole module green (deskkit 34.2s, forgeban, all cmds ok) | PASS |
| 2 | `go test ./cmd/deskpr/... ./cmd/deskfile/... ./cmd/deskclose/...` | 0 | all three migrated suites ok | PASS |
| 3 | `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | 0 | seam grows 4 ops, stays closed | PASS |
| 4 (+deref) | `TestForgeGithubGolden` + `TestForgeGitlabGolden` + `TestForgeGitlabCoverage` | 0 | all 4 op tokens (open_change_for_branch/edit_change/search_issues/list_labels) in both golden corpora; coverage reconciles inventory rows 33-36 | PASS |
| 5 | `grep -n allowedInvocationCeiling tools/desk/internal/forgeban/allowlist.go` | — | `= 9` @ :64 (base 12 − 3) | PASS |
| 6 | `grep -c` the three permit rows (deskclose/deskfile/deskpr exec `gh`) | — | `0` — all three gone | PASS |
| 7 | `go test ./internal/forgeban/...` | 0 | ratchet passes at the lowered ceiling | PASS |
| 8 | `grep -rn -e 'runCmd("gh"' -e 'runGH(' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose non-test \| wc -l` | — | `0` — no verb shells gh | PASS |
| 9 | `grep -rn -e '--as-app=false' -e 'asApp' tools/desk/cmd/deskpr non-test \| wc -l` | — | ACTUAL `5` (expect 0) — all 5 are retirement-documenting COMMENTS (main.go doc block, edit.go/exec.go/github.go) + one refusal error-string; NO `flag.Bool`/`Var(` for as-app, NO live `asApp` identifier. The flag + branch are genuinely removed; intent met (independently proven by row 10). **Flagged for the human: literal-vs-intent divergence, not a silent pass.** | PASS (intent) |
| 10 (neg) | `TestDeskprRefusesWithoutMintedToken -v` | 0 | refuses ("ForgeFor never falls back to an ambient gh-CLI identity"); create/open/get calls == 0, no push | PASS |
| 11 (+flow) | `TestDeskfileFilesOnGitLabThroughBackend` + `TestDeskfileRefusesWithoutMintedToken` | 0 | POSITIVE: files via backend on a GitLab repo (no "GitHub only"); NEGATIVE: refuses, filed=nil, search==0 | PASS |
| 12 | `TestDeskcloseClosesThroughBackend` + `TestDeskcloseActingLoginFromRoster` | 0 | closes via `CloseIssue` (closes()==1); acting login from `RoleAppLogin`, NO `api graphql viewer` whoami | PASS |
| 13 (neg) | `TestOpenChangeForBranchAmbiguousRefuses -v` | 0 | github+gitlab: 2 open changes → `ExitUnverifiable` refusal, nil result, message names the branch; control subtest proves a single change resolves | PASS |
| 14 (+deref) | `statusgen --root . --consumers --brief forge-neutral/13` | — | COULD-NOT-CHECK — offline verifier shared-home writeguard + statusgen v1.0.6 brief-v2 gap. Consumers hand-corroborated: all three verbs route `forgeForFn → deskkit.ForgeFor(fr, mintedRole)`; deskfile RETAINS its unresolvable-forge could-not-check while the interim "GitHub only" GitLab refusal is GONE; inventory rows 33-36 present | COULD-NOT-CHECK |
| 15 (mutation) | disable the deskclose worker-half guard → `go test ./cmd/deskclose/...`; restore; re-run | 1 → 0 | MUTANT: `TestSupersededWorkerProposes` reddens (worker token falls through to the reviewer half, collapsing propose≠confirm); RESTORED: 0; worktree clean | PASS |

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):**
- `RISK-VALUE: DERIVED — allowedInvocationCeiling = 9 @ tools/desk/internal/forgeban/allowlist.go:64.` RANK #1 (security ratchet). Derived as base(12) − 3; the three named permit rows (`cmd/deskclose/exec.go::runGH::gh`, `cmd/deskfile/exec.go::gh::gh`, `cmd/deskpr/exec.go::gh::gh`) confirmed gone (row 6 = 0); ratchet green (row 7). No NAMED-NOT-DERIVED literal — the ceiling is derived, not a magic number.
- Secondary risk-bearing values: the two-role close-authority separation (row 15 mutant, live) and the removed ambient fallback (rows 9/10).

**Sensitive-data defense (gate: human) — independent layers:** (1) ambient fallback GONE, not defaulted (row 9 — no flag/branch; row 10 — a no-token run refuses with zero forge calls, no push, no ambient fall-through); (2) the two-role close authority is a LIVE control (row 15 — disabling the worker-half guard reddens the deskclose suite); (3) custody-mint is the sole token path (rows 10/11-neg — deskpr and deskfile both refuse when the mint yields no token; deskfile additionally fails closed on an unresolvable forge before any op). Layers trip on different signals in different components (backend unminted-token refusal vs `resolveCaller` roster-binding checks).

**Scope-traceability:** no work observed outside the brief's Verify rows / consumers; the extra deskclose guards map to the pre-mortem detection map. Row 9's literal grep divergence (5 vs 0) is a documentation artifact (comments/error-strings naming the retired path), not a live path.

**VERDICT: PASS** on all 14 runnable rows; row 14 COULD-NOT-CHECK (writeguard + brief-v2 gap; consumers hand-corroborated) — **HELD at `implemented` (human sign-off owed via the verify-gate).** For the human: (a) confirm the per-verb acting identity (deskpr/deskclose → session-role App; deskfile → session-role App with `--raised-by` as body attribution vs the named alternative of minting the raised-by role's own App); (b) note the row-9 literal-vs-intent divergence (5 documenting mentions, no live flag). No open NAMED-NOT-DERIVED value.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the acting identity of three write
verbs changes). Reviewer records verdict + date in the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a write and the wrong identity performing it? (The
   resolver's custody binding — WHICH token it hands the backend.) Is it acceptable alone? (No —
   the backends' unminted-token refusal and the commit-identity/actor preflight are the two
   independent layers behind it; a credential fault and an actor fault trip different checks in
   different components.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 10 and
   row 11-NEGATIVE: with the custody binding yielding nothing, the backend itself REFUSES rather
   than falling through to the decoy ambient credential — the `--as-app=false` / no-mint path is
   gone, not merely defaulted off.)
3. Is the acting identity this brief proposes for each verb correct? (deskpr → session-role worker
   App; deskclose → session-role App; deskfile → session-role App with `--raised-by` as body
   attribution — or the named alternative, the raised-by role's own App.) This is the token-custody
   decision the permit register reserves for a human.
