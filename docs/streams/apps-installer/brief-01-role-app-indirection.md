---
brief: assay:assay:apps-installer:01
title: Role→App indirection — six desk roles on N GitHub Apps without symlinks
why: >-
  desktoken resolves a role's credentials by the role's own name, so a deployment with fewer Apps
  than roles (the recommended two-App tier: one that reads, one that writes) has no supported
  layout — the only way to run it today is six copies or six symlinks of two private keys, a
  custody arrangement no document describes and no check can see. One optional binding from role to
  App name, defaulting to the role's own name, makes two-App and six-App deployments the same code
  path and is the seam the installer writes into.
wave: 0
depends: []
unblocks: ["apps-installer/02", "apps-installer/03", "apps-installer/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §6 (command surface: apps.env carries the role→App bindings) and §7 (Solo creates no App)."
  - "`tools/desk/cmd/desktoken/desktoken.go` — the `validRoles` map (six roles) and the comment at its head: a role's config is parameterised by the role name (`<role>-app.pem`, `<ROLE>_APP_ID`); App ID resolution reads env `<ROLE>_APP_ID` else `apps.env`; `<ROLE>_INSTALL_ID` override."
  - "`tools/desk/internal/deskkit/rosterconfig.go` — `ASSAY_TRUSTED_BOT_SLUGS` entries parse as `[role=]slug[:id]`; the `role=` prefix binds a desk role to a slug."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no role→App indirection exists; every credential lookup is keyed on the role name."
exec-tier: strong
exec-tier-why: >-
  Question (b) — cross-artifact correctness: the binding must resolve identically in desktoken, in
  deskkit's roster trust matching (which role a `<slug>[bot]` login IS), and in the preflight's
  scopes check, or a two-App deployment passes one and fails another. Question (a) — the record
  format for the binding in apps.env is a design decision the facts constrain but do not fix.
consumers:
  - "tools/desk/cmd/desktoken/desktoken.go: fixed-here (resolution reads the binding first)"
  - "tools/desk/internal/deskkit/rosterconfig.go: fixed-here (a slug may carry more than one `role=` binding; today's single-binding assumption is documented, not enforced — measure before changing)"
  - "tools/desk/internal/deskkit/preflight.go: fixed-here (the scopes check reads the grant of the App the role is BOUND to)"
  - "tools/desk/README.md § App credentials: fixed-here (the binding documented beside the search path)"
  - "docs/adopting-assay.md roster section: follow-up apps-installer/07"
version: 1
id: 383542d8-1dec-4385-a6b2-83d5b143dfd7
---

# Brief 01 — Role→App indirection

## Context
files:
- `tools/desk/cmd/desktoken/desktoken.go` — App ID / install ID / PEM resolution.
- `tools/desk/cmd/desktoken/desktoken_test.go`, `tools/desk/cmd/desktoken/mutations.json`.
- `tools/desk/internal/deskkit/rosterconfig.go` — `ASSAY_TRUSTED_BOT_SLUGS` parse; role bindings.
- `tools/desk/internal/deskkit/preflight.go` — `checkAppScopes` (reads the grant sidecar of the
  minted token).
- `tools/desk/README.md` — § "App credentials (`desktoken`, `deskpost`, `deskevidence`)".

single-point-of-failure: the binding lookup is the ONE control deciding which key a role mints
with. The second, independent layer already exists and is kept: the roster's `role=slug` binding
in `ASSAY_TRUSTED_BOT_SLUGS` decides which bot login the trust gate accepts FOR that role, and it
is read by a different package from a different file. A mis-bound key mints a token for an App
whose `[bot]` login the roster does not bind to that role, and the trust gate refuses the post.
Verify row 6 proves that layer with the upper one deliberately wrong.

facts:
- `validRoles` = {reviewer, verifier, worker, desk, issue-loop, intake-loop} (`desktoken.go:27`,
  2026-09-05 @ 38e96f7).
- Today: PEM `<role>-app.pem` on the credential search path; App ID from env `<ROLE>_APP_ID`
  else `apps.env`; install ID from `<ROLE>_INSTALL_ID` override else `apps.env`
  (`desktoken.go` ~L346–L560).
- Roster entry grammar: `[role=]slug[:id]` (`rosterconfig.go:81`, `:828`). Whether one slug may
  carry two `role=` prefixes is NOT stated in code — the implementer measures it (Task 1) before
  relying on it.
- Binding record (this brief decides the shape; the recommended one): in `apps.env`, one line
  per role, `<ROLE>_APP=<app-name>`, where `<app-name>` is the stem of `<app-name>.pem` and the
  prefix of `<APP_NAME>_APP_ID` / `<APP_NAME>_INSTALL_ID`. Absent → `<app-name>` = `<role>-app`,
  which is byte-identical to today's layout. Env `<ROLE>_APP` overrides the file, like the other
  fields.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never introduce a symlink, copy, or "duplicate the pem" fallback. If the binding is absent the
  default is the role's own name; nothing else.

## Task
1. **Measure the roster.** Write a test that parses `ASSAY_TRUSTED_BOT_SLUGS` with two `role=`
   prefixes on one slug (`reviewer=x-act:1,worker=x-act:1`). Record the result in the test's
   comment. If the parser refuses, extend it so a slug may be bound to several roles; the
   `role-bindings=` echo lists every binding. If it already accepts, keep the test as the
   regression guard.
2. **Add the binding to desktoken.** Resolve `<ROLE>_APP` (env, then `apps.env`) before the PEM,
   App ID and install ID lookups; use it as the stem for all three. Default `<role>-app`. The audit
   line gains an `app=<app-name>` field. The `--version` config echo prints the effective
   bindings on one `bindings=` line, `role=app-name` per role (this is what rows 2 and 3 read).
3. **Make the scopes check role-aware through the binding.** `checkAppScopes` already reads the
   `.perms` sidecar of the minted token; confirm that with two roles bound to one App the sidecar
   is shared and the check still runs once per role with the same grant (a shared grant that
   covers `requiredDuties` passes both). Add a test.
4. **Document.** `tools/desk/README.md` § App credentials: the binding line, the default, the
   override precedence, and the sentence: *a two-App deployment is two keys and six bindings, never
   six keys*.
5. **Mutation row.** Add a `mutations.json` entry that removes the binding lookup (falls back to
   `<role>-app` unconditionally) and assert Verify row 5 goes red on it.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/desktoken/ -run RoleApp -count=1 && go test ./internal/deskkit/ -run MultiRole -count=1` | exit 0 |
| 2 | `cd tools/desk && go build -o /tmp/desktoken ./cmd/desktoken && HOME=$(mktemp -d) sh -c 'mkdir -p $HOME/.config/assay; printf "REVIEWER_APP=x-act\nX_ACT_APP_ID=1\nX_ACT_INSTALL_ID=2\n" > $HOME/.config/assay/apps.env; /tmp/desktoken --version 2>&1'; echo $?` | output contains `bindings=` with `reviewer=x-act`; exit 0 |
| 3 | `cd tools/desk && HOME=$(mktemp -d) sh -c 'mkdir -p $HOME/.config/assay; printf "REVIEWER_APP_ID=1\nREVIEWER_INSTALL_ID=2\n" > $HOME/.config/assay/apps.env; /tmp/desktoken --version 2>&1'` | output contains `reviewer=reviewer-app` (absent binding = today's layout) |
| 4 | `cd tools/desk && ASSAY_TRUSTED_BOT_SLUGS='reviewer=x-act:1,worker=x-act:1' go test ./internal/deskkit/ -run 'MultiRole' -count=1 -v 2>&1 \| grep -cE -e 'role-bindings=.*reviewer=x-act' -e 'role-bindings=.*worker=x-act'` | ≥ 1 |
| 5 | `grep -cE -e 'REVIEWER_APP' -e 'ROLE>_APP' tools/desk/README.md` | ≥ 1 |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run UnboundRoleRefused -count=1` | exit 0 — a `[bot]` login whose slug is not bound to the posting role is refused even when a token for it exists (the independent lower layer) |
| 7 | `cd tools/desk && go test ./cmd/desktoken/ -run 'Mutation' -count=1` | exit 0 — the mutation corpus includes the removed-binding mutant and row 2's assertion catches it |
| 8 | `statusgen --root . --consumers --brief apps-installer/01` | exit 0 (routing claims corroborated against the diff) |

## Evidence

Implemented on branch `feat/apps-installer-01`. All eight Verify rows run locally (offline).

| # | Result |
|---|--------|
| 1 | PASS — `go build ./...` clean; `go test ./cmd/desktoken/ -run RoleApp` and `go test ./internal/deskkit/ -run MultiRole` both exit 0 |
| 2 | PASS — `desktoken --version` with `REVIEWER_APP=x-act` prints `bindings=… reviewer=x-act …`, exit 0 |
| 3 | PASS — `desktoken --version` with no binding prints `reviewer=reviewer-app` (absent binding = today's layout) |
| 4 | PASS — the `MultiRole` test logs `role-bindings=reviewer=x-act worker=x-act`; the row's grep counts 1 (≥1) |
| 5 | PASS — `tools/desk/README.md` § App credentials documents `<ROLE>_APP` / `REVIEWER_APP` (grep ≥1) |
| 6 | PASS — `TestUnboundRoleRefused`: a `[bot]` login the roster does not bind to the posting role is refused, and an unbound role resolves to no login (the independent lower layer) |
| 7 | PASS — `TestMutationRemovedBindingInCorpus`: `mutations.json` carries the removed-binding mutant, and the `--version` echo is the guard that reddens on it |
| 8 | PASS — `statusgen --root . --consumers --brief <this brief>` exits 0 (routing claims corroborated / inherited) |

**Fail-first (Task 5 / rule 9).** With the binding lookup removed (`appNameFor` returning
`role + "-app"` unconditionally — the `mutations.json` removed-binding mutant), the `--version`
bindings echo shows `reviewer=reviewer-app` instead of the bound `reviewer=x-act`, and
`TestRoleAppBindingVersionEcho`, `TestRoleAppBindingResolvesBoundAppID` and
`TestMutationRemovedBindingInCorpus` all fail. Reverting restores green — the guard observes the
mutant.

**Design decisions (frontmatter question (a) — the binding record format).**
- Record: one line per role, `<ROLE>_APP=<app-name>` (env, then `apps.env`), the App-name being
  the PEM stem and the App-ID / install-ID key prefix. Default `<role>-app`.
- App-name → env-prefix (`AppEnvPrefix`): upper-case with `-`→`_`, then strip a trailing `_APP`.
  This is the one rule that reconciles "the default is byte-identical to today"
  (`reviewer-app` → `REVIEWER` → `REVIEWER_APP_ID`) with "the App-name is the prefix of
  `<APP_NAME>_APP_ID`" (`x-act` → `X_ACT` → `X_ACT_APP_ID`): only the default's conventional
  `-app` suffix is trimmed, every real bound App-name uses the plain prefix.
- Task 1 measurement: the roster parser ALREADY accepts several `role=` prefixes on one slug
  (`RoleBots` is keyed on the role), so no parser widening was needed — the test is the
  regression guard, and `rosterconfig.go` documents the property.
- Task 3: `checkAppScopes` needed no logic change — the token path it reads is the bound App's
  mint, and `requiredDuties` is one fixed set, so a shared grant passes every bound role; a
  comment records this and `TestMultiRoleSharedGrantPassesEveryBoundRole` proves it.
- Scope boundary: the binding is confined to the mint path (`desktoken` + the deskkit
  `AppBinding`/`AppEnvPrefix`/`AppIDForApp` helpers) per the consumer list. `deskpost` /
  `deskevidence` / `desktoken coverage` still resolve role-keyed; extending the binding to their
  own JWT-signing paths is follow-up territory (they are not in this brief's consumer set).
### Non-implementer verifier run — 2026-09-11 sonnet-5-verifier (verify-desk dispatch), offline — **VERIFY: PASS — first non-implementer pass**

Pin: `medici-finance/assay` main `86c7d62c8081189147baf37424b602f907b139aa` (git rev-parse == gh api commits/main). Fresh clone. `gate: model`, `risk {all no}`, `irreversible: no`.

| # | Result |
|---|---|
| 1 | PASS — `go build ./...` clean; `go test ./cmd/desktoken/ -run RoleApp` and `go test ./internal/deskkit/ -run MultiRole` both exit 0 |
| 2 | PASS — `desktoken --version` with `REVIEWER_APP=x-act` prints `bindings=… reviewer=x-act …`, exit 0 |
| 3 | PASS — with no binding set, prints `reviewer=reviewer-app`, exit 0 |
| 4 | PASS — `MultiRole` test logs `role-bindings=reviewer=x-act worker=x-act`; grep count 1 (≥1) |
| 5 | PASS — README grep count 6 (≥1); documents `<ROLE>_APP`/`REVIEWER_APP`, default, precedence, the "two keys and six bindings" sentence |
| 6 | PASS — `TestUnboundRoleRefused` is a real independent lower-layer test (roster `role=slug` binding), not a stub |
| 7 | PASS as literally run, but the named test only checks corpus-entry-exists + unmutated-code-passes, not the mutant reddening itself — **independently hand-verified**: applied the exact `mutations.json` patch (`appNameFor` → `role + "-app"` unconditionally) by hand, confirmed `TestRoleAppBindingVersionEcho`, `TestRoleAppBindingResolvesBoundAppID`, and `TestMutationRemovedBindingInCorpus` all genuinely FAIL under the mutant; reverted, all green again, working tree clean |
| 8 | **FAIL as literally written** — `statusgen --root . --consumers --brief apps-installer/01` exits 2, `COULD-NOT-CHECK: no brief-v1 file`. Root cause: a later, unrelated repo-wide flag-day migration (`bb2079bd`) changed every brief's `brief:` frontmatter field from `<stream>/<NN>` to the colon form `assay:assay:<stream>:<NN>`, breaking the row's literal `--brief` argument for every brief in the repo, not just this one — already tracked at `assay#822` (open: "statusgen --consumers --brief rejects the `<stream>/<NN>` id... only the colon form works"). Even with the corrected id, the row is inherently pre-merge/diff-scoped by the tool's own design (documented in its source), so a fully-merged main with no open diff reports COULD-NOT-CHECK by construction — this is the same structural class as `assay-toolkit#2406`'s gap, not a defect in this brief's own work |

**Substance checks (traced code, not just names):**
1. `AppEnvPrefix` reconciliation rule — genuinely wired: `deskkit.AppEnvPrefix` (`appconfig.go:114-117`) trims `_APP` only when present, and `desktoken.go:670-672` calls it before PEM/App-ID/install-ID resolution. Traced both directions (`reviewer-app`→`REVIEWER`, `x-act`→`X_ACT` unchanged).
2. Fail-first mutation — empirically reproduced by hand (see row 7).
3. `checkAppScopes` role-agnostic claim — `requiredDuties` never branches on role (`preflight.go:776-860`); `TestMultiRoleSharedGrantPassesEveryBoundRole` is a real, non-trivial test of a shared grant passing both bound roles.

`RISK-VALUE: DERIVED` — `AppEnvPrefix`'s `_APP`-trim rule and the default `<role>-app` binding are both traced to source and empirically exercised both directions. `requiredDuties` (pre-existing, unaffected) confirmed role-agnostic. No unvetted magic numbers introduced.

**VERIFY: PASS.** Rows 1-7 pass genuinely (row 7's substance independently confirmed by hand-mutation, beyond what its own named test proves). Row 8 fails only for a documented, repo-wide, already-tracked tooling reason (`assay#822`) affecting every brief in this stream identically — not a defect in this brief's deliverable. `gate: model`, `risk: {all no}`, `irreversible: no` — flip-eligible.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
