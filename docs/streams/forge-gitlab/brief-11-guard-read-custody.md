---
brief: assay:assay:forge-gitlab:11
title: Guard-read custody — the last gh shell-outs onto the Forge seam
why: >-
  Three `gh` shell-outs survive in the desk tree — two display reads in `deskroster`, one
  GET choke point in `repohardenguard` — and they are the reason forge-gitlab/08's closure-to-zero
  row still reads 4, not 0. They are not a cosmetic remainder: each runs under whatever credential
  `gh` happens to hold on the machine, and on GitLab none of them works at all, so a desk on GitLab
  cannot annotate its roster or check a project's hardening. The ruling on #834 (option B:
  closure-to-zero stands) makes them open work. What has blocked them is not transport but
  CUSTODY — whose token each read runs under once it can no longer borrow the operator's — and
  this brief settles that: the roster reads run as the session's own role, the hardening guard
  gets a read-only identity of its own, and its arbitrary `gh api <endpoint>` reads become one
  enumerated Forge operation over a closed kind set, so the surface stays shut on both forges.
wave: 4
depends: ["forge-gitlab/02", "forge-gitlab/03", "forge-gitlab/08"]
unblocks: ["forge-gitlab/12"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief decides WHICH identity two tools act as once they stop borrowing an ambient
  credential, and it mints a NEW forge identity (a read-only auditor App / service account) whose
  scope, if drawn too wide, is a standing credential that can change repository settings while
  living on a laptop next to a GET-only tool. The permit register calls exactly this a token-custody
  decision, not a transport change. The human confirms three things: that the roster's display
  reads run as the session's own role token; that the hardening guard runs as a dedicated
  READ-ONLY identity rather than any role that can write; and that the admin-gated rows a
  read-only identity cannot see stay could-not-check (re-run by a human admin) rather than buying
  visibility with a write-capable grant.
decision-trigger: creation
design: DR-forge-gitlab-11
issues: [834]
schema: brief-v2
authored: 2026-09-11 by forge-gitlab authoring session (custody design worker)
sources:
  - "#834 — the verify FAIL on forge-gitlab/08 row 3 and the driver's ruling (option B, 2026-09-11, ratified in-thread): closure-to-zero stands; the residual `gh` sites are open work needing a custody design"
  - "#835 — the fg/08 Evidence PR recording the FAIL (row 3 returns 4: three real invocations plus one comment literal)"
  - "#841 — open at authoring (draft, reviewer-App APPROVED @ 887c6ce): moves the two `deskroster` sites onto the EXISTING `GetPullRequest` / `ListOpenChanges` ops under the session-role token and lowers the forgeban ceiling 9 → 7; proposed keeping `repohardenguard` as CLI, which the ruling rejects"
  - "docs/streams/forge-gitlab/spec.md §3 (a constrained typed surface is the stronger side of the parity table), §5 (custody: minted tokens, rotate-on-mint, file custody 0600), §6 (freeze rule — an op lands with its consuming tool in the same change)"
  - "docs/streams/forge-gitlab/inventory.md — the frozen 37-op table this brief adds op 38 to; its `Residual forge-CLI call sites` section classes `repohardenguard` as `not a forge op at all` and `deskroster` as `identity`; deltas D2 (minting stays outside the interface) and D3 (hardening reads are not frozen forge ops — until a consumer exists)"
  - "docs/streams/forge-gitlab/brief-08-close-the-forge-surface.md — Verify row 3 (whole-tree grep, expect 0) and the no-passthrough test this brief must stay inside"
  - "docs/streams/forge-gitlab/brief-03-gitlab-token-custody.md — the per-role token-file contract (`<config>/gitlab-<role>.token`, 0600, rotate-on-mint) the new role inherits unchanged"
  - "tools/desk/internal/forgeban/allowlist.go — the permit register; its header names the blocker in its own words: both backends REFUSE a client without an explicitly minted token, so routing an ambient-credential tool through the seam changes WHO acts, which is a token-custody decision"
  - "tools/desk/internal/deskkit/forgeresolve.go — the resolver contract: `ForgeFor` never mints, never falls back to an ambient CLI credential, is the single construction site; GitHub custody via the installed minter hook → `desktoken`, GitLab custody by READING the role's token file"
  - "tools/desk/cmd/desktoken/desktoken.go (`validRoles`, the fixed six) + internal/deskkit/appconfig.go (`AppID`/`AppBinding`: every credential lookup is parameterised by role name — `<role>-app.pem`, `<ROLE>_APP_ID`, `<ROLE>_INSTALL_ID`, optional `<ROLE>_APP` binding)"
  - "tools/desk/cmd/repohardenguard/{main,check,checklist}.go — `ghRun`/`ghGet`, `Row.Endpoint()` parsing `gh api <endpoint>`, the three-state rule (#127 in its header), the no-roster posture, `identity()` reading `/user`"
  - "the custody ruling (an adopter-tracker ruling, cited here without its number): a tool's forge identity is minted for its role and scoped to what that role does — never the operator's ambient credential — and a per-purpose identity carries the narrowest grant that serves the purpose (the write-issues App model, D2)"
  - "#821's Security-Review verdicts — the reviewer bar on custody changes: fail-closed on every error path, purely additive on the green path, no existing refusal weakened, no secret in the diff"
  - "GitHub REST docs, read 2026-09-11: the `security_and_analysis` block on GET /repos requires admin permission on the repository; GET …/private-vulnerability-reporting requires admin READ access; a ruleset's `bypass_actors` is returned only to a caller with WRITE access to the ruleset"
  - "freshness-checked 2026-09-11 @ 8953d38d — fg/08 row 3 returns 4 on main (`cmd/repohardenguard/check.go:46`, `cmd/deskroster/roster.go:229`, `:246`, and the comment at `internal/forgeban/forgeban.go:173`); `allowedInvocationCeiling = 9`; the `Forge` interface has 37 methods, none of them a hardening read; `validRoles` is the fixed six"
exec-tier: strong
exec-tier-why: "it fixes the acting identity of two tools (question c: an over-scoped grant or a quiet ambient fallback survives every functional test) and it re-shapes an open-ended read (arbitrary endpoints from a document) into a closed enumerated op without losing the guard's three-state semantics (question b: correctness is the flow checklist → kind → backend → status → verdict, not any one site)."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit/forge.go + forge_github.go + forge_gitlab.go + their golden tests: follow-up forge-gitlab/11 (this brief — op 38 `RepoHardeningRead` with its kind validator, the GitHub kinds, the GitLab named refusal; flips to fixed-here when the implementation edits the paths)"
  - "tools/desk/cmd/repohardenguard/*.go: follow-up forge-gitlab/11 (this brief — the fetcher moves from `ghGet` onto the typed op under the `auditor` identity; `Row.Endpoint()` becomes `Row.Kind()` over the closed vocabulary; error→status mapping reads `ForgeAPIError`)"
  - "tools/desk/cmd/desktoken/desktoken.go (`validRoles`) + internal/deskkit/preflight.go remediation text: follow-up forge-gitlab/11 (this brief — the `auditor` role; GitHub App mint and GitLab token-file read both keyed on the role name, no new code path)"
  - "tools/desk/internal/forgeban/allowlist.go + forgeban.go: follow-up forge-gitlab/11 (this brief — the `repohardenguard` permit row is removed and the ceiling lowered; the comment literal at forgeban.go:173 is reworded so the whole-tree grep reads 0)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-gitlab/11 (this brief — op 38's row and delta paragraph land WITH the method: `TestForgeNoPassthrough` reflects the interface against this table, so the row cannot precede the code)"
  - "docs/adopting-assay.md + docs/adopting-assay-gitlab.md: follow-up forge-gitlab/11 (this brief — provisioning the auditor identity: the GitHub App's permission set, the GitLab service account's `read_api` scope, the `<config>/auditor-app.pem` / `gitlab-auditor.token` custody files)"
  - "tools/desk/internal/deskkit/echocoverage_test.go (`exemptFromRoster` reason for repohardenguard): follow-up forge-gitlab/11 (this brief — the guard now reads the roster's forge map, still never the write-authorisation set; the exemption's reason is rewritten to say so)"
  - "tools/desk/cmd/deskroster/*.go: out-of-scope (delivered by #841, open at authoring; absorbed into this brief only if #841 closes unmerged — the custody answer for those two reads, the session's own role token, is decided here either way)"
  - "the adopter's hardening checklist document (its Read cells): out-of-scope (it lives outside this tree; its `gh api <endpoint>` cells move to the `read <kind>` vocabulary when the adopter re-pins — the parser refuses the old form by name, never silently)"
  - "GitLab hardening kinds (protected branches, protected tags, push rules, approvals) + per-forge checklist rows: follow-up forge-gitlab/12"
version: 1
id: 20cb61a4-6f57-4722-8d52-812b6dd8c989
---

# Brief 11 — Guard-read custody: the last `gh` shell-outs onto the Forge seam

## Context

forge-gitlab/08 delivered a closed `Forge` surface (37 enumerated ops, no passthrough — its Verify
rows 4 and 5 PASS) but its shell-exec ban shipped as a RATCHET, not a closure: the permit register
in `internal/forgeban` allows nine call sites, and the brief's Verify row 3 — a whole-tree grep for
`exec.Command(…"gh"…)`, expected `0` — returns 4. The driver ruled on #834 that closure-to-zero
stands, because the three real sites are GitHub-only: on GitLab they do not degrade, they do not
run. The blocker, in the permit register's own words, is custody — "Both Forge backends REFUSE to
construct a client without an explicitly minted token … Routing these tools through the seam
therefore changes WHO performs the write, which is a token-custody decision, not a transport
change." This brief makes that decision and lands the three migrations behind it.

**The three sites, and what each needs.**

| Site | Reads | Runs today as | Forge op | Custody decided here |
|---|---|---|---|---|
| `deskroster` `ghViewPR` | `pr view --json state,isDraft,title` | ambient `gh` login | `GetPullRequest` (op 1; `Title` landed with `deskpr edit`) | the SESSION's own role token (`SessionTokenRole` from the loop identity) — #841's shape |
| `deskroster` `ghListOpenPRs` | `pr list --state open` | ambient `gh` login | `ListOpenChanges` (op 23) | same |
| `repohardenguard` `ghRun`/`ghGet` | arbitrary GET `gh api <endpoint>` parsed out of a checklist document; preflight `repos/<repo>`; `/user` for the identity line | ambient `gh` login, usually a human admin's | NONE today — a new enumerated op over a closed kind set (op 38, below); file-presence rows via the existing `ReadFile` (op 22) | a dedicated READ-ONLY `auditor` role with its own App / service account, minted and read through the existing per-role custody paths |

**Why not the other custody shapes.** Passing the ambient credential into the Forge client is
what the seam was built to retire: both backends refuse an unminted token by design, the audit
trail must name a role rather than "whoever was logged in", and on GitLab there is no `gh auth
token` to pass — only a `glab` shell-out the ban forbids. Running the guard as an existing role
(desk, reviewer, worker) hands a GET-only tool a credential that can post, file, flip and push;
the guard's own header says every setting it reads "belongs to the human", so its identity must
not be one that could apply them. A per-purpose identity with the narrowest grant is the same
model the write-issues App follows (inventory delta D2). The options, alternatives and accepted
consequences are the design record `DR-forge-gitlab-11`.

**Op 38 — `RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error)`.**
One method, a CLOSED kind vocabulary, one fixed endpoint literal per kind per backend. The
argument is a named enum validated BEFORE any request exists (`ValidateHardeningReadKind`, the
`DeleteRef`/`ValidateRefPath` shape), never a path — so the method passes `TestForgeNoPassthrough`
on both its name check and its no-endpoint-argument check, and a kind the backend does not serve
is a could-not-check REFUSAL naming the forge and the kind (resolver contract 3), never a guess
and never the other forge's document. The GitHub kinds and the document each returns:

| Kind | GitHub read (fixed literal) | Notes |
|---|---|---|
| `repo` | `GET /repos/{o}/{r}` | `.visibility`, `.security_and_analysis.*` (admin-visible only — a `null` here is could-not-check, exactly as today) |
| `rulesets` | `GET /repos/{o}/{r}/rulesets` then `GET …/rulesets/{id}` per entry; returns the ARRAY of detail documents | the guard's `[name=X].field` selector resolves inside the returned array — the two-hop moves into the backend; `bypass_actors` is present only for a caller with write access to the ruleset, so under a read-only identity those rows are could-not-check |
| `actions-workflow-permissions` | `GET /repos/{o}/{r}/actions/permissions/workflow` | admin-gated |
| `actions-fork-pr-approval` | `GET /repos/{o}/{r}/actions/permissions/fork-pr-contributor-approval` | admin-gated |
| `actions-private-fork-pr` | `GET /repos/{o}/{r}/actions/permissions/fork-pr-workflows-private-repos` | admin-gated |
| `vulnerability-reporting` | `GET /repos/{o}/{r}/private-vulnerability-reporting` | admin read |
| _(file presence)_ | not a kind — the checklist's `read file <path>` rows route through op 22 `ReadFile`; the Field cell resolves against the op's result rendered as JSON | a 404 from a repo the preflight proved readable is a real absence |

On the GitLab backend every kind above is a named could-not-check refusal in this brief; the
GitLab kinds (protected branches, protected tags, push rules, approvals) and the per-forge
checklist rows are forge-gitlab/12. The checklist's Read cell becomes `read <kind>` (or `read file
<path>`); a `gh api …` cell is refused at parse time naming the vocabulary. The property the
guard's tests pin is preserved and sharpened: the DOCUMENT is the source of every required value,
the TOOL is the source of the enumerated reads, and neither can widen the other.

files:
- `tools/desk/internal/deskkit/forge.go` — op 38 `RepoHardeningRead` + `HardeningReadKind` +
  `ValidateHardeningReadKind`; no other method, no field taking a path.
- `tools/desk/internal/deskkit/forge_github.go`, `forge_github_golden_test.go` — the six GitHub
  kinds, one golden per kind (the `rulesets` golden pins the list-then-detail walk) plus
  `hardening_read_unknown_kind` emitting ZERO requests.
- `tools/desk/internal/deskkit/forge_gitlab.go`, `forge_gitlab_test.go` — the named refusal
  (`could-not-check: gitlab serves no hardening read of kind %q — forge-gitlab/12`), one golden so
  `TestForgeGitlabCoverage` reconciles.
- `tools/desk/cmd/repohardenguard/check.go`, `checklist.go`, `main.go`, `repohardenguard_test.go`
  — `Checker.Get(endpoint)` → `Checker.Read(kind)`; `Row.Endpoint()` → `Row.Kind()`; `httpStatus`
  reads `*deskkit.ForgeAPIError.Status` / `IsForgeNotFound` instead of regexing `gh`'s stderr;
  the preflight becomes `read repo`; `identity()` reports the minted role's known login
  (`<slug>[bot]`, the mechanism `deskclose` already uses) instead of `/user`, which an App token
  cannot read; the fake-`gh` test fixture becomes a stub `Forge` keyed by kind, every existing
  verdict asserted unchanged.
- `tools/desk/cmd/repohardenguard/forge.go` (new) — the resolver seam: `deskkit.ForgeFor(fr,
  "auditor")` (a FIXED role, the `deskevidence`/`"verifier"` shape — the guard runs outside any
  desk window, so it must not read `$DESK_LOOP`); forge resolution by the roster's
  `ASSAY_REPO_FORGES` entry only, never the CWD's origin remote (the guard is routinely run from a
  checkout of a different repo); unresolvable → exit 6 naming the key.
- `tools/desk/cmd/desktoken/desktoken.go` — `validRoles` += `auditor`. Nothing else: the GitHub
  mint resolves `auditor-app.pem` / `AUDITOR_APP_ID` / `AUDITOR_INSTALL_ID` by the existing
  role-parameterised lookup; the GitLab path reads/rotates `gitlab-auditor.token` by the same
  brief-03 contract. `desktoken --version` echoes the new binding.
- `tools/desk/internal/forgeban/allowlist.go` — remove `cmd/repohardenguard/check.go::ghRun::gh`;
  lower `allowedInvocationCeiling` by exactly the rows removed. `forgeban.go:173` — reword the
  comment so it no longer carries the literal the whole-tree grep counts.
- `tools/desk/internal/deskkit/echocoverage_test.go` — `exemptFromRoster["repohardenguard"]`
  reason updated (reads the forge map, never the write-authorisation set).
- `docs/streams/forge-gitlab/inventory.md` — op 38 row + delta paragraph (consumer:
  `repohardenguard`), the residual-call-sites table updated, the ceiling narrative.
- `docs/adopting-assay.md`, `docs/adopting-assay-gitlab.md` — provisioning the auditor identity
  (GitHub: an App with `Metadata: read`, `Contents: read`, `Administration: read` and NO write
  permission of any kind, installed on the account; GitLab: a service account / bot with a
  `read_api`-scoped PAT and the Reporter role, or Maintainer only where forge-gitlab/12's kinds
  need it).
- `changelog/forge-gitlab-11-guard-read-custody.md` — the fragment.
- Conditional: `tools/desk/cmd/deskroster/roster.go`, `forge.go`, tests — ONLY if #841 has
  closed unmerged at pickup; then re-land its diff here unchanged (same ops, same session-role
  custody).

single-point-of-failure: the FORGE-SIDE GRANT on the auditor identity is the one control between a
leaked or mis-provisioned guard token and a settings write — our code is GET-only, but code is not
a boundary a stolen token respects. It is backed by two independent layers that fail for different
reasons in different components: (1) the enumerated surface — op 38 takes a closed kind, not a
path, `TestForgeNoPassthrough` forbids any method that does, and the forgeban ratchet forbids a
shell-out around it, so the tool's own code path cannot be steered at a write endpoint; (2) the
forge's permission model — the auditor App / PAT carries no write scope, so a write attempted with
its token (bypassing our code entirely) is refused by the forge with a 403. Verify row 8 proves
layer 2 with layer 1 bypassed. A third, out-of-band signal already exists: the forgeban ceiling
and the inventory reflection redden CI on any new shell-out or method.

facts:
- Spec §6 freeze rule: op 38 lands WITH `repohardenguard` converted in the same change; the
  inventory row lands with the method (the reflection test compares the two).
- fg/08 Verify row 3 is the closure target, verbatim: `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' | grep -v _test.go | wc -l` must read `0`. On main @ 8953d38d it reads 4. #841 takes it to 2 (repohardenguard + the comment); this brief takes it to 0.
- The permit register (2026-09-11): ceiling 9; #841 → 7; this brief → one fewer than whatever
  merged main reads at pickup (6 if #841 has merged, 6 if this brief absorbs deskroster). Six
  permit rows remain after both (deskadvisory, deskdigest, deskdispatch, deskdisposition,
  deskmerge, deskpushguard); they invoke `gh` through a wrapper the row-3 grep does not see, so
  row 3 reads 0 while the ratchet still fences them. They are NOT this brief's scope; the register's
  rows name what each needs.
- `ForgeFor` contract (forgeresolve.go): never mints, never ambient, single construction site;
  GitHub custody via the `GitHubCustodyMinter` hook a command installs in `init()` (`deskboard`'s
  `forge.go` is the template), GitLab custody by reading `gitlab-<role>.token` (0600, Refused
  otherwise). The guard installs the same hook and passes the fixed role `auditor`.
- Roster: an UNBOUND role is not refused by `ForgeFor` (forgeidentity.go: "an unbound role is
  not this check's concern") — the auditor never posts, so it needs no `ASSAY_TRUSTED_BOT_SLUGS`
  trust binding; a forge-qualified entry MAY be added for the echo, and if present must agree
  with the resolved forge (`assertEntryForgeAgrees`).
- Three-state rule (#127, the guard's header) is unchanged in meaning: 403 → could-not-check; 404
  on an admin-gated row → could-not-check; 404 on a public row from a repo the preflight proved
  readable → absent-therefore-wrong; a `null` admin field → could-not-check. Only the SOURCE of
  the status changes (`ForgeAPIError.Status` instead of `(HTTP \d{3})` regexed from stderr).
- What a read-only identity CANNOT see, by the forge's own rules (GitHub docs, 2026-09-11):
  `security_and_analysis` (admin), `bypass_actors` (write access to the ruleset). Under the
  auditor those rows report could-not-check with the existing "re-run as a repository admin"
  hint. This is the accepted consequence of least privilege, recorded in the design record; it
  is NOT bought back with an `Administration: write` grant.
- GitHub App installation tokens cannot read `/user` (403 for an integration) — the identity
  line must come from the role's known login, not a whoami.
- `repohardenguard` keeps reading NO write-authorisation set (`AllowedRepos`); the roster read
  it gains is the forge map (`ASSAY_REPO_FORGES`) for resolution only. The checklist's repos
  directive remains the scope authority.
- Public-tree hygiene: the checklist document itself is an adopter artifact and is not in this
  tree; the guard's tests carry their own fixture rows (o/r, o/r2) and those move to the
  `read <kind>` grammar in the same change.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
Two desk tools still reach the code forge by running the `gh` command with whatever login the
machine happens to hold. They have to stop: the fleet's forge client refuses to act without a
token minted for a named role, and on GitLab there is no such command at all. The question is
WHICH identity each tool acts as once it stops borrowing the operator's.

The roster tool only annotates a listing with each open change's state and title, inside a desk
window that already holds that window's role token — so it uses that token. No new identity.

The hardening guard is different. It is a GET-only checker that compares a repository's live
settings (visibility, secret scanning, rulesets and their bypass lists, workflow permissions)
against a checklist, and its own rule is that every one of those settings belongs to the human
to apply. Some of the fields it reads are only visible to an administrator. The choice:

Options:
1. **A dedicated read-only "auditor" identity (recommended)** — a new App (GitHub) / service
   account (GitLab) that can read repository settings and contents and holds NO write permission
   of any kind. The guard runs as it; a write attempted with its token is refused by the forge
   itself. Consequence accepted: the few fields the forge only shows to a write-capable caller
   (ruleset bypass lists, the security-and-analysis block) report "could not check" under this
   identity and are re-run by a human administrator — the same outcome the guard gives a
   non-admin today. One more identity to provision per account, on both forges.
2. **An existing desk role's token** (desk, reviewer or worker) — no new provisioning, and the
   fields above are still not visible (those roles are not repository admins either), so nothing
   is gained; what is lost is least privilege: a GET-only tool would carry a credential that can
   post, file, flip and push.
3. **A read-only identity WITH administrative write rights** — makes every admin-gated field
   readable, at the price of a standing settings-changing credential sitting next to a GET-only
   tool on an operator's laptop. Rejected by the guard's own rule that settings are the human's
   to apply.
4. **Keep passing the operator's own login through** — the status quo in a new coat; rejected:
   it is exactly the lane being retired, it cannot work on GitLab, and the evidence a
   hardening run produces is only worth something when it names the role that produced it.

Default if no answer: none — blocks until answered (the tools keep their current allow-listed
shell-outs; the closure-to-zero row stays red).

## Edition
Minimum GitLab tier: **free** (Community Edition). This brief adds no GitLab READ — every kind
above refuses by name on the GitLab backend until forge-gitlab/12 — so nothing here is tier-gated.
The custody half is CE-native: the `auditor` role's GitLab token is a service-account PAT under the
brief-03 rotate-on-mint contract (edition-matrix.md row C2, Free), and a `read_api`-scoped PAT is
the forge-enforced read-only boundary on any tier. What the GitLab side will and will not be able
to check — push rules (Premium), approval settings (Premium), secret push protection (Ultimate),
and the identity-level bypass list that CE expresses only at role level — is forge-gitlab/12's
`## Edition` to state; this brief names no new degradation (spec §1 forbids a third without a
recorded ruling).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The human decision above must be RECORDED (the decision issue closed with an option) before
  any code is written; if the recorded option is not 1, report NEEDS_CONTEXT — the Task below
  implements option 1 and the design record is amended, never silently re-targeted.

## Task
1. **Custody.** Add `auditor` to `desktoken`'s `validRoles`; confirm the GitHub mint and the
   GitLab token-file path both resolve it by name with no further change (`desktoken --version`
   echoes `auditor=auditor-app`). Document provisioning in both adopter docs: GitHub App
   permissions `Metadata: read`, `Contents: read`, `Administration: read`, nothing writable;
   GitLab service account with a `read_api` PAT, Reporter role. Add the preflight remediation
   text for a missing/insecure `auditor` custody file.
2. **Op 38.** Add `RepoHardeningRead` + `HardeningReadKind` + `ValidateHardeningReadKind` to
   `forge.go`; implement the six GitHub kinds (fixed literals; `rulesets` walks list→detail
   and returns the detail array); implement the GitLab named refusal; goldens for every kind,
   the zero-request unknown-kind refusal, and the GitLab coverage case. Update `inventory.md`
   (row 38, delta paragraph, residual-sites table) in the same change.
3. **Guard migration.** Replace `ghRun`/`ghGet` with the resolver seam (`forge.go`, fixed role
   `auditor`, roster-map resolution only); `Row.Endpoint()` → `Row.Kind()` over `read <kind>` /
   `read file <path>` (a `gh api` cell is a parse REFUSAL naming the vocabulary); file rows via
   `ReadFile`; `httpStatus` from `ForgeAPIError`; identity line from the role's known login;
   preflight `read repo`. Re-plumb the tests from the fake `gh` binary onto a stub `Forge`;
   every existing verdict (admin-null, 403, 404-public, 404-admin, wrong-repo scope, row census)
   unchanged. Add `TestChecklistRefusesGhApiCell` (planned).
4. **Closure.** Remove the `repohardenguard` permit row, lower the ceiling by the rows removed,
   reword the `forgeban.go:173` comment. If #841 is not on main at pickup, re-land its
   `deskroster` diff here first (rows and ceiling accordingly). fg/08 row 3 must read 0.
5. **Consumers.** Flip every `follow-up forge-gitlab/11` routing above to `fixed-here` in this
   change; run `statusgen --root . --consumers` before pushing.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | check:ci |
| 2 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | `0` — fg/08 row 3, verbatim, now closed | check +neighbour |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -v` | exit 0; output contains `PASS` — the ratchet reconciles at the lowered ceiling, op 38 passes the name and no-endpoint-argument checks, the inventory reflects 38 methods, every kind has a golden | check:ci |
| 4 | `grep -cE -e 'Key: +"cmd/repohardenguard/' -e 'Key: +"cmd/deskroster/' tools/desk/internal/forgeban/allowlist.go; test "$(grep -oE 'allowedInvocationCeiling = [0-9]+' tools/desk/internal/forgeban/allowlist.go \| grep -oE '[0-9]+$')" -le 6` | first line `0`; exit 0 — no permit row for either tool, ceiling at or below 6 | check |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden/hardening_read_unknown_kind -v` | exit 0; output contains `PASS` (`hardening_read_unknown_kind` (planned)) and the golden records ZERO requests — the kind validator refuses before a request exists | check +mutation |
| 6 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestChecklistRefusesGhApiCell -v && go test ./cmd/repohardenguard/ -run TestAdminNullIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestForbiddenIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestPublicNotFoundIsAbsent -v` | exit 0; output contains `PASS` — the old Read grammar is refused by name; the three-state verdicts survive the seam (`TestChecklistRefusesGhApiCell` (planned), `TestAdminNullIsCouldNotCheck` (planned), `TestForbiddenIsCouldNotCheck` (planned), `TestPublicNotFoundIsAbsent` (planned) — names the re-plumbed suite creates; the four verdict classes are the contract) | check +flow |
| 7 | `repohardenguard --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" --json \| jq -e '(.identity \| test("\\[bot\\]$")) and ([.rows[] \| select(.state=="could-not-check" and .gated!="admin")] \| length == 0)'` | exit 0 — the run names the auditor App as its identity (never a human login), and every could-not-check row is an admin-gated one (a public row never goes could-not-check under the auditor) | gate:human +dereference |
| 8 | `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer $(cat "$(desktoken auditor --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/[^/]+?(\.git)?$#\1#')")")" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" -d '{}'` | `403` — a settings WRITE attempted with the guard's token, bypassing the guard entirely, is refused by the FORGE (an empty PATCH changes nothing even where it would succeed, so the probe is side-effect-free) | gate:human +mutation |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | `1` — the role resolves through the existing binding echo | check |
| 10 | `statusgen --root . --consumers` | exit 0 — every `follow-up forge-gitlab/11` routing has flipped to `fixed-here` and the diff corroborates it | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The op takes a kind but a backend builds the path from a caller string (a passthrough in a coat) | row 3 (`TestForgeNoPassthrough` no-endpoint-argument walk) + row 5 (unknown kind → zero requests) |
| The permit row is removed but the ceiling is not lowered, so the gain is never locked in | row 3 (the ratchet fails when the list is SHORTER than the ceiling) + row 4 |
| The whole-tree grep still counts the comment literal, so fg/08 row 3 reads 1 | row 2 |
| The three-state semantics regress in the fetcher swap (a 403 read as absent-therefore-wrong) | row 6 |
| The guard silently falls back to an ambient identity when no auditor custody file exists | row 7 (identity must be a `[bot]` login) — plus `ForgeFor` refuses by contract; review-only for the refusal text |
| The auditor App is provisioned with a write permission "to see bypass_actors" | row 8 — the negative-path probe returns 2xx instead of 403 |
| The GitLab backend returns an empty document for a kind instead of refusing | row 3 (`TestForgeGitlabCoverage` needs a golden per method; the golden pins the refusal) — adequacy of the refusal text is review-only |
| #841 closes unmerged and deskroster is forgotten | row 2 reads 2 |
| The inventory row is written but the method count disagrees | row 3 (reflection against the table) |
| The adopter docs describe a permission set the forge does not offer | **no row** — review-only (dereferenced by the human who provisions the App in row 7's run; a wrong set shows as unexpected could-not-check rows) |

### Dispatch checklist
```
[x] 1. Rows DISCRIMINATE — row 8 is the negative control; rows 2/4/5 fail on a plausible half-migration.
[x] 2. Facts dated and checkable — sha 8953d38d, the row-3 count, the ceiling, the docs quotes all dated 2026-09-11.
[x] 3. Self-contained — the kind table, the custody shape and the row-3 literal are in this file.
[x] 4. Risk answers match files: — desktoken + forge backends + adopter provisioning = sensitive-data yes; nothing regulatory/customer/irreversible.
[x] 5. gate-why names the wire — a new identity's scope, and the admin-gated trade-off.
[x] 6. Effort honest — one op (six fixed reads), one tool, one role; #841 carries the other tool. M.
[x] 7. Shared value → consumers: enumerated; row 6 is the flow row (checklist → kind → backend → status → verdict).
[x] 8. Pre-mortem run; the one row-less failure is recorded as review-only.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 7 and 8 need a
     provisioned auditor identity and are run by the human gate. -->

## Review
Gate: human (from frontmatter — a risk answer is yes). Human gate is MANDATORY. Reviewer records
verdict + date in the stream README table. The reviewer answers the two core-system questions:
(1) the single control between a leaked guard token and a settings write is the forge-side grant
on the auditor identity — is a read-only App / `read_api` PAT an acceptable single control given
the enumerated surface and the ratchet behind it? (2) does row 8 prove the LOWER layer (the forge's
permission model) refuses with the UPPER layer (our GET-only code, the kind validator) bypassed —
i.e. is the probe genuinely outside our code path? A Security-Review verdict is required on the
implementing PR (custody change on a public repo).
