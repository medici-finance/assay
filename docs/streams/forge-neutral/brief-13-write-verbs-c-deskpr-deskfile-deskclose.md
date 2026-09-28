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
unblocks: ["forge-neutral/10", "forge-neutral/16"]
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
### Verify-lane hygiene correction — 2026-09-17 sonnet-5-verifier (verify-desk)

The 2026-09-10 non-implementer verifier pass above genuinely PASSED 14 of 15 rows (row 14 could-not-check, a known tooling gap — `#822`, `statusgen --consumers --brief` rejects the `<stream>/<NN>` id form). That pass wrote its verdict as `**VERDICT: PASS**`, not the canonical bold token `**VERIFY: PASS**` that `statusgen --verify-issues` matches literally (`hasVerifyPass`, exact substring, fail-closed by design — the same marker-text near-miss also showed up on `windows-port/03`). As a result no human sign-off card has ever been filed for this brief, though it is `gate: human` + `sensitive-data: yes` with a genuinely earned pass.

This note does not re-verify — it restates the existing, unchanged verdict using the canonical marker so the sign-off card mechanism can fire on the next `--verify-issues` run:

**VERIFY: PASS** on 14 of 15 rows (row 14 could-not-check, tooling gap `#822`, hand-corroborated in the original pass). No new Evidence gathered, no re-run performed, no model sign-off — per `gate: human` + `sensitive-data: yes`, the human still closes the sign-off card; this note only unblocks that card from being filed at all.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskpr/... ./cmd/deskfile/... ./cmd/deskclose/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | pass exit=0 | sha256:e7e1daaf83f6 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c -e 'cmd/deskclose/exec.go::runGH::gh' -e 'cmd/deskfile/exec.go::gh::gh' -e 'cmd/deskpr/exec.go::gh::gh' tools/desk/internal/forgeban/allowlist.go` | fail exit=1 | sha256:9a271f2a916b | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -rn -e 'runCmd("gh"' -e 'runGH(' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn -e '--as-app=false' -e 'asApp' tools/desk/cmd/deskpr --include='*.go' \| grep -v _test.go \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskpr/... -run TestDeskprRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/... -run TestDeskfileFilesOnGitLabThroughBackend -count=1 -v && go test ./cmd/deskfile/... -run TestDeskfileRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskclose/... -run TestDeskcloseClosesThroughBackend -count=1 -v && go test ./cmd/deskclose/... -run TestDeskcloseActingLoginFromRoster -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestOpenChangeForBranchAmbiguousRefuses' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief forge-neutral/13` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 15 | `runSupersededLane` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:622fa03be176 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |

**Non-implementer verifier re-run — 2026-09-28 opus-5.5[1m]-verifier (verify-desk) — notes under the witness table above.** Runner is not the implementer. Own detached temp worktree cut off origin/main at merged head c50a38fc12518a4eec4db37e8dd847d49e79149a, equal to the live remote main head at run time (read with the verifier App token). The brief was last edited on main at 0ea52ec3, so nothing was reverted. Implementation landed in b7a82c025 (PR #783, issue #775), with a mutation-spec refresh in #793. Toolchain: host go1.27.1 darwin/arm64, and the pinned statusgen v1.0.27 binary run directly, not through a wrapper. Its sha256 matches the pin. KUBECONFIG=/dev/null. The only GitHub credential was the verifier App token. gate: human + `sensitive-data: yes`: this run adds Evidence only, and status stays `implemented`.

Witness profile: no Verify row's first code span mints a credential, calls a live forge, or mutates live state. The witness therefore ran unsandboxed and executed only the plain `check` rows (5, 6, 8, 9) plus row 15's first span. The direct runs below used a network-denied macOS sandbox profile, `(deny network*)`, with a loopback-only allowance for httptest listeners, and a probe confirmed that external name resolution failed inside it. They also used GOFLAGS=-count=1, a fresh GOCACHE, GOPROXY=off and GOTOOLCHAIN=local. These direct runs are targeted package tests. They support the witness table above but do not replace it.

Per-row key output:
- Rows 1-4, 7, 10-14 (check:ci): witness could-not-run, because the darwin host has no `unshare --net` network-off sandbox (#1800).
- Row 1: direct `go build ./...` exit 0. The whole-module `go test ./...` was NOT run directly. This pass used targeted package tests only, covering the three verb packages plus deskkit and forgeban (rows 2-4, 7, 10-13).
- Row 2: direct exit 0; deskpr ok 77.5s, deskfile ok 38.8s, deskclose ok 38.0s.
- Row 3: direct exit 0 for both `-run` names (ok, no "no tests to run").
- Row 4: direct exit 0 for all three `-run` names (github golden, gitlab golden, gitlab coverage). Inventory rows 33-36 are present on main.
- Row 5: the witness pass checks the exit status only, and its output decodes to `const allowedInvocationCeiling = 6` at allowlist.go:84. The Expect cell pins 9, which is 3 below the authoring base. At b7a82c025 the value went from 12 to 9 at allowlist.go:64, and the three rows were removed in the same commit. Later migrations named in the allowlist header lowered it to 6. The intent held at merge, and the literal Expect is stale-shaped against current main.
- Row 6: the witness recorded fail exit=1, and its output decodes to `0`, which is the expected value. `grep -c` exits 1 whenever the count is zero, so this is how the check was written, not a defect.
- Row 7: direct exit 0; forgeban ok, with the ratchet green at ceiling 6.
- Row 8: the witness recorded fail exit=1, and its output decodes to `0` (BSD `wc -l` padding), which is the expected value. The exit 1 comes from pipefail carrying the first grep's no-match. This is also how the check was written.
- Row 9: the witness pass is VACUOUS because the check reads the exit status only. The output decodes to `5`, but the Expect cell pins `0`. The five hits are four retirement-documenting comments (exec.go:59, main.go:87, github.go:11, edit.go:391) and one refusal error string (github.go:38). No `"as-app"` flag definition or live `asApp` identifier remains. The flag and its branch are gone, and row 10 proves that independently. This is the same literal-versus-intent gap the 2026-09-10 pass flagged, and it comes from how the check was written.
- Row 10: direct exit 0, and the deskpr refuses-without-minted-token test reports `--- PASS`.
- Row 11: direct exit 0 on both, with `--- PASS` for deskfile's files-on-GitLab-through-backend test and its refuses-without-minted-token test.
- Row 12: direct exit 0 on both, with `--- PASS` for deskclose's closes-through-backend test and its acting-login-from-roster test.
- Row 13: direct exit 0; the ambiguity-refusal subtests github, gitlab and github_single_resolves all `--- PASS`.
- Row 14: direct run of the pinned statusgen with `--consumers --brief forge-neutral/13` exited 0, but the result is VACUOUS. All 7 consumers read UNCHECKED ("unchanged since the merge-base"), because merged main has no branch diff to check against. The claims were checked by hand against b7a82c025 instead:
  - forge.go gained the four ops (OpenChangeForBranch, SearchIssues, ListLabels and EditChange, at forge.go:1342/1349/1356/1617).
  - cmd/deskpr, cmd/deskfile and cmd/deskclose all changed. deskpr and deskclose call `deskkit.ForgeFor(fr, mintedRole)`, and deskfile goes through `forgeForFn` to the resolver.
  - The interim GitHub-only refusal is gone from deskfile.
  - The out-of-scope deskpushguard `fetchPR` permit row is still present, and the live-pilot brief is untouched.
  - The slash id form was accepted on v1.0.27.
- Row 15: the witness gets exit 127 because the row is written as prose and the witness executes its first code span. The mutation was run directly as the row describes, in this run's own worktree:
  - superseded.go:271 was changed to `if false && who.role == roleWorker {`.
  - `go test ./cmd/deskclose/...` then exited 1, failing TestSupersededWorkerProposes plus the two typed-reference tests.
  - The file was restored (path-specific checkout), and the re-run exited 0 (ok 44.1s).
- No direct run wrote artifacts into the source tree: the untracked-file listing was empty after every run.

Risk-bearing value enumeration (diff scope: b7a82c025 non-test code plus the brief's Deliverables):
- `allowedInvocationCeiling = 9` @ tools/desk/internal/forgeban/allowlist.go:64 at b7a82c025; it is now `6` @ :84 after later retirements.
- `role := "worker"` @ tools/desk/cmd/deskfile/github.go:74, with `var mintedRole = "worker"` @ :33. This is deskfile's acting-role default, introduced here. Before this change deskfile minted no token.
- `var mintedRole = "worker"` @ tools/desk/cmd/deskpr/exec.go:65, introduced here. The `role := "worker"` fallback @ :149 predates this change (#396).
- `deskkit.SessionTokenRole("deskclose")` @ tools/desk/cmd/deskclose/forge.go:61. It has NO default, and an unresolved role refuses (:62-66).
- The ambiguity threshold in OpenChangeForBranch, `switch len(w)` with case 0, case 1 and a default that refuses @ tools/desk/internal/deskkit/forge_github.go:576; the GitLab twin is at forge_gitlab.go:696.
- `forgeSearchPerPage = 200` @ tools/desk/internal/deskkit/forge_github.go:1797. It predates this brief, but this diff reuses it for SearchIssues, which replaces deskfile's prior dedupe bound `searchLimit = "20"` (matcher.go:47 at the parent commit).
- `forgeFilePerPage = 100` and `forgeMaxFilePages = 40` @ forge_github.go:459-460, and `gitlabPerPage = 100` and `gitlabMaxFilePage = 40` @ forge_gitlab.go:432-433. These predate the brief and are reused here for the branch lookup and ListLabels pagination. The GitLab SearchIssues call sends no per_page (forge_gitlab.go:738).

Ranked by irreversibility:
1. The three acting-identity bindings rank first. A write under the wrong App leaves a permanent authorship record on a public forge: an issue or change can be closed, but its author cannot be re-attributed.
2. The ratchet ceiling ranks next. A wrong value lets a forge-CLI call site land without anyone seeing it.
3. The ambiguity threshold ranks third. It fails toward refusing.
4. The page sizes are reversible operational knobs and rank last.

RISK-VALUE: DERIVED — `allowedInvocationCeiling = 9` @ tools/desk/internal/forgeban/allowlist.go:64 (at b7a82c025) — the parent commit's value was 12, and exactly the three named identity-class rows came off in the same commit (row 6 prints 0), so the correct value is 12 - 3 = 9. Current main's 6 comes from later briefs' retirements, and the ratchet is green (row 7).
RISK-VALUE: DERIVED — `switch len(w)` default refuses @ tools/desk/internal/deskkit/forge_github.go:576 (gitlab twin forge_gitlab.go:696) — the op's contract is "the single OPEN change for a source branch". Two or more matches have no single correct answer, so refusing is the only outcome that does not guess (row 13, both backends).
RISK-VALUE: NAMED, NOT DERIVED — `role := "worker"` @ tools/desk/cmd/deskfile/github.go:74 — deskfile now files under the session-role App, which is the worker App when no loop role resolves, with `--raised-by` kept as body/label attribution. The brief reserves the choice between this and the named alternative (mint the `--raised-by` role's own App) for the human (gate-why, Task 4). The code implements the proposal, but a model cannot self-certify a token-custody decision. OPEN QUESTION for the human.
RISK-VALUE: NAMED, NOT DERIVED — `var mintedRole = "worker"` @ tools/desk/cmd/deskpr/exec.go:65 — deskpr acts as the session-role App, which is the worker App by default, and the ambient fallback is retired (rows 9, 10). The missing piece is the human's confirmation of that acting identity (gate-why). OPEN QUESTION for the human.
RISK-VALUE: NAMED, NOT DERIVED — `deskkit.SessionTokenRole("deskclose")` @ tools/desk/cmd/deskclose/forge.go:61 — deskclose acts as the DESK_LOOP-selected role App, with no worker default, and refuses when no role resolves. The missing piece is the human's confirmation of that acting identity (gate-why). OPEN QUESTION for the human.

Sensitive-data layers, as observed in this run:
- Layer 1: with no minted token, deskpr and deskfile refuse and make zero forge calls (rows 10 and 11), so there is no ambient fall-through.
- Layer 2: the two-role close authority is a live control, because disabling the worker-half guard turns the deskclose suite red (row 15).
- These layers trip on different signals in different components: the resolver/backend custody refusal versus the deskclose caller-role binding.

Observation, outside this brief's scope: `forgeSearchPerPage = 200` @ tools/desk/internal/deskkit/forge_github.go:1797 exceeds the per-page maximum of 100 that GitHub documents for its search endpoint. This could not be checked offline, so treat it as NAMED, NOT DERIVED. For deskfile dedupe a silent cap at 100 is harmless, because it is still more candidates than the prior bound of 20. The same constant also drives the truncation flag at :1983, which then could never fire. That is forge-neutral/12's surface. It is a reversible knob.

VERIFY: BLOCKED — 0 of 15 rows are witness-clean. Row 15 is green only by a direct mutation run.
- Rows 6 and 8 are witness fail, and rows 5 and 9 are witness passes on exit status alone. For all four, the expected value was confirmed by reading the output, and each gap comes from how the check was written or from later changes on main, not from a defect.
- Rows 1-4, 7 and 10-14 need a Linux network-off witness (#1800). Each one is green by direct targeted package test, except row 1's whole-module test, which was not run directly, and row 14, whose output is vacuous on merged main.
- No defect was found in the shipped change. Status stays at `implemented`, and the human sign-off (#1302) owes the three NAMED, NOT DERIVED acting-identity rulings above.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskpr/... ./cmd/deskfile/... ./cmd/deskclose/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | pass exit=0 | sha256:e7e1daaf83f6 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c -e 'cmd/deskclose/exec.go::runGH::gh' -e 'cmd/deskfile/exec.go::gh::gh' -e 'cmd/deskpr/exec.go::gh::gh' tools/desk/internal/forgeban/allowlist.go` | fail exit=1 | sha256:9a271f2a916b | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -rn -e 'runCmd("gh"' -e 'runGH(' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn -e '--as-app=false' -e 'asApp' tools/desk/cmd/deskpr --include='*.go' \| grep -v _test.go \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskpr/... -run TestDeskprRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/... -run TestDeskfileFilesOnGitLabThroughBackend -count=1 -v && go test ./cmd/deskfile/... -run TestDeskfileRefusesWithoutMintedToken -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskclose/... -run TestDeskcloseClosesThroughBackend -count=1 -v && go test ./cmd/deskclose/... -run TestDeskcloseActingLoginFromRoster -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestOpenChangeForBranchAmbiguousRefuses' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --root . --consumers --brief forge-neutral/13` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 15 | `runSupersededLane` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:5676635b3dab | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-09-28 at batch tree e1afb99a after main changed `tools/desk/internal/deskkit/forgeresolve.go` — non-implementer verifier (verify-desk, model claude-opus-5-5).** The witness table directly above ran at e1afb99aca990bbdb14ef44eb65fceb0610b18e8, which is merged main 02a2f75fb532af2bf7bc6284e3f5f4c6e9337f48 plus this batch's Evidence commit (main re-read with the verifier App token: still 02a2f75fb). Envelope: the pinned statusgen v1.0.27 binary run directly (sha256 matches the pin), `env -i` with the real HOME for the Runner cell, PATH reduced to the system dirs plus a scratch dir holding only `go` and `statusgen`, KUBECONFIG=/dev/null, GOFLAGS=-count=1, a fresh empty GOCACHE, GOPROXY=off, GOTOOLCHAIN=local, under macOS `sandbox-exec` with the network denied except loopback (the previous run's witness ran unsandboxed). No forge credential was in the environment. gate: human + `sensitive-data: yes`: Evidence only, status stays `implemented`.

Drift assessment: forgeresolve.go changed in exactly one main commit since the previous run (c50a38fc), 31d2ad536 (#1650, issue #1631). It ADDS one function, `GitHubTokenIdentityForRepo` (an identity probe for an inherited token), and the same commit adds `tokenidentity.go` to the deskkit package with an unexported `viewerIdentity` method on the GitHub backend. The only caller is `cmd/deskdispatch`. No existing function in forgeresolve.go changed, nothing under `cmd/deskpr`, `cmd/deskfile`, `cmd/deskclose` or `internal/forgeban` changed, and the four ops this brief added are untouched. The change does not touch this brief's behaviour. The seam tests (rows 3, 4, 13) run over the changed package and stay green, including "neither backend exports a method outside the interface".

Per row, vs the previous run (c50a38fc):
- Rows 1-4, 7, 10-14: witness could-not-run again, check:ci on a darwin host (#1800). Targeted package tests, run directly in the same envelope (the whole module `go test ./...` was NOT run):
  - Row 1: `go build ./...` exit 0.
  - Row 2: exit 0; deskpr, deskfile and deskclose ok (slower than before under host load, no failures).
  - Row 3: both `-run` names exit 0; all five no-passthrough subtests PASS.
  - Row 4: all three exit 0; `open_change_for_branch` (plus `_none`, `_ambiguous_refuses`), `edit_change`, `search_issues` and `list_labels` golden cases PASS on both backends (plus gitlab `search_issues_forbidden`); coverage "reconciles: 54 operations, all covered" (unchanged).
  - Row 7: exit 0; forgeban ok at ceiling 6.
  - Row 10: exit 0, the deskpr refuses-without-minted-token test PASS.
  - Row 11: both exit 0, deskfile files-on-GitLab and refuses-without-minted-token tests PASS.
  - Row 12: both exit 0, deskclose closes-through-backend and acting-login-from-roster tests PASS.
  - Row 13: exit 0; subtests github, gitlab and github_single_resolves PASS.
  - Row 14: pinned `statusgen --root . --consumers --brief forge-neutral/13` exit 0 but vacuous: base resolves to 02a2f75fb (the batch commit's parent), "0 corroborated, 0 disproved, 7 unchecked" (#1281). The previous run's hand check against b7a82c025 still stands; the drift touches none of the seven claimed paths.
- Row 5: witness pass exit=0, hash e7e1daaf83f6 unchanged: `const allowedInvocationCeiling = 6` at allowlist.go:84. Vacuous (exit status only); the Expect cell pins 9, stale-shaped against later retirements, as before.
- Row 6: witness fail exit=1, hash 9a271f2a916b unchanged, decodes to `0`, the expected value (`grep -c` exits 1 on a zero count).
- Row 8: witness fail exit=1, hash 4eff2db4bada unchanged, the padded `0` the Expect wants (pipefail carries the first grep's no-match).
- Row 9: witness pass exit=0, hash 30179a803d48 unchanged, decodes to `5` against an Expect of `0`. Vacuous; the same five retirement comments and one refusal string, no live flag or identifier.
- Row 15: witness could-not-run exit=127 again, prose cell whose first code span is `runSupersededLane` (#1805). The output hash moved (622fa03be176 to 5676635b3dab), most likely because the not-found message names the shell. The mutation was run directly in this run's own worktree: superseded.go:271 changed to `if false && who.role == roleWorker {`, `go test ./cmd/deskclose/... -count=1` exit 1 (the worker-proposes test and the two typed-reference tests FAIL), file restored by path-specific checkout, re-run exit 0. Same as before.
- No run left artifacts in the tree: `git status` showed only this brief modified.

Unchanged from the previous run: the RISK-VALUE lines and the three NAMED, NOT DERIVED acting-identity questions above. The drift adds no literal in this brief's diff scope.

VERIFY: BLOCKED — at e1afb99a no row is witness-clean (0/15). Rows 6 and 8 are witness fails and rows 5 and 9 are exit-status-only passes; each output was read and each gap comes from how the check was written or from later ratchet moves, not from a defect. Rows 1-4, 7 and 10-14 need a Linux network-off witness (#1800), row 14 is vacuous on merged main (#1281), and row 15's cell cannot be witnessed as written (#1805). Every direct targeted test and the direct mutation behaved as the Verify table expects.

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
