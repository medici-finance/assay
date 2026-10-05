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
issues: [834, 857]
schema: brief-v2
authored: 2026-09-11 by forge-gitlab authoring session (custody design worker)
sources:
  - "#857 — this brief's own decision-gate issue, CLOSED `human-decided` 2026-09-11 with the driver's ruling: option A/1 (the dedicated read-only `auditor` identity) approved as proposed, PLUS one addition — the adopter documentation and the public website are updated alongside, so every page enumerating `desktoken` roles or an App permission set gains the auditor row and its minimal grant. `DR-forge-gitlab-11` is flipped to APPROVED citing that comment; this brief's deliverables and Verify rows carry the docs half."
  - "#834 — the verify FAIL on forge-gitlab/08 row 3 and the driver's ruling (option B, 2026-09-11, ratified in-thread): closure-to-zero stands; the residual `gh` sites are open work needing a custody design"
  - "#835 — the fg/08 Evidence PR recording the FAIL (row 3 returns 4: three real invocations plus one comment literal)"
  - "#841 — open at authoring (draft, reviewer-App APPROVED @ 887c6ce): moves the two `deskroster` sites onto the EXISTING `GetPullRequest` / `ListOpenChanges` ops under the session-role token and lowers the forgeban ceiling 9 → 7; proposed keeping `repohardenguard` as CLI, which the ruling rejects"
  - "docs/streams/forge-gitlab/spec.md §3 (a constrained typed surface is the stronger side of the parity table), §5 (custody: minted tokens, rotate-on-mint, file custody 0600), §6 (freeze rule — an op lands with its consuming tool in the same change)"
  - "docs/streams/forge-gitlab/inventory.md — the frozen 39-op table this brief adds op 40 to; its `Residual forge-CLI call sites` section classes `repohardenguard` as `not a forge op at all` and `deskroster` as `identity`; deltas D2 (minting stays outside the interface) and D3 (hardening reads are not frozen forge ops — until a consumer exists)"
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
  - "tools/desk/internal/deskkit/forge.go + forge_github.go + forge_gitlab.go + their golden tests: fixed-here (op 40 `RepoHardeningRead` with its kind validator, the GitHub kinds, the GitLab named refusal)"
  - "tools/desk/cmd/repohardenguard/*.go: fixed-here (the fetcher moves from `ghGet` onto the typed op under the `auditor` identity; `Row.Endpoint()` becomes `Row.Kind()` over the closed vocabulary; error→status mapping reads `ForgeAPIError`)"
  - "tools/desk/cmd/desktoken/desktoken.go (`validRoles`) + internal/deskkit/preflight.go remediation text: fixed-here (the `auditor` role; GitHub App mint and GitLab token-file read both keyed on the role name, no new code path)"
  - "tools/desk/internal/forgeban/allowlist.go + forgeban.go: fixed-here (the `repohardenguard` permit row is removed and the ceiling lowered; the comment literal at forgeban.go:173 is reworded so the whole-tree grep reads 0)"
  - "docs/streams/forge-gitlab/inventory.md: fixed-here (op 40's row and delta paragraph land WITH the method: `TestForgeNoPassthrough` reflects the interface against this table, so the row cannot precede the code)"
  - "docs/adopting-assay.md + docs/adopting-assay-gitlab.md: fixed-here (the #857 ruling's docs half, a DELIVERABLE with its own Verify rows: provisioning the auditor identity at every place these two pages ENUMERATE roles or permission sets — the GitHub App inventory + the provisioning checklist step 2 (`Metadata: read`, `Contents: read`, `Administration: read`, nothing writable), the GitLab role→service-account table (`read_api`, Reporter) and the per-role `gitlab-<role>.token` list, plus the `<config>/auditor-app.pem` / `gitlab-auditor.token` custody files and the preflight remediation text)"
  - "the public website's adoption + apps pages: out-of-scope (the #857 ruling's website half; it lives in a different repo and lands as a COMPANION change tracked separately, so this brief neither edits nor gates on it — the wording it mirrors is the two adopter pages above)"
  - "tools/desk/internal/deskkit/echocoverage_test.go (`exemptFromRoster` reason for repohardenguard): fixed-here (the guard now reads the roster's forge map, still never the write-authorisation set; the exemption's reason is rewritten to say so)"
  - "tools/desk/cmd/deskroster/*.go: out-of-scope (delivered by #841, open at authoring; absorbed into this brief only if #841 closes unmerged — the custody answer for those two reads, the session's own role token, is decided here either way)"
  - "the adopter's hardening checklist document (its Read cells): out-of-scope (it lives outside this tree; its `gh api <endpoint>` cells move to the `read <kind>` vocabulary when the adopter re-pins — the parser refuses the old form by name, never silently)"
  - "GitLab hardening kinds (protected branches, protected tags, push rules, approvals) + per-forge checklist rows: follow-up forge-gitlab/12"
version: 2
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
| `repohardenguard` `ghRun`/`ghGet` | arbitrary GET `gh api <endpoint>` parsed out of a checklist document; preflight `repos/<repo>`; `/user` for the identity line | ambient `gh` login, usually a human admin's | NONE today — a new enumerated op over a closed kind set (op 40, below); file-presence rows via the existing `ReadFile` (op 22) | a dedicated READ-ONLY `auditor` role with its own App / service account, minted and read through the existing per-role custody paths |

**Why not the other custody shapes.** Passing the ambient credential into the Forge client is
what the seam was built to retire: both backends refuse an unminted token by design, the audit
trail must name a role rather than "whoever was logged in", and on GitLab there is no `gh auth
token` to pass — only a `glab` shell-out the ban forbids. Running the guard as an existing role
(desk, reviewer, worker) hands a GET-only tool a credential that can post, file, flip and push;
the guard's own header says every setting it reads "belongs to the human", so its identity must
not be one that could apply them. A per-purpose identity with the narrowest grant is the same
model the write-issues App follows (inventory delta D2). The options, alternatives and accepted
consequences are the design record `DR-forge-gitlab-11`.

**The ruling, and the docs half it added (#857, 2026-09-11).** The decision-gate issue is CLOSED
`human-decided`: the driver ruled **option 1 — the dedicated read-only `auditor` identity —
approved as proposed**, with one addition: *the documentation and the website are updated
alongside*. So provisioning the auditor is not a side note in this brief; it is a DELIVERABLE with
its own Verify rows. Every page that enumerates the `desktoken` roles or an App's permission set
gains the auditor and its minimal grant — in this repo that is `docs/adopting-assay.md` (the App
inventory and the provisioning checklist) and `docs/adopting-assay-gitlab.md` (the role→service-
account table and the per-role `gitlab-<role>.token` list) — and a drift test pins code against
those pages so a role can never again exist only in `validRoles`. The reason is the same one the
guard's own three-state rule encodes: a role a fleet must provision but no page names is a
provisioning step an adopter meets first as a Refused exit, with nothing to read that says what to
create. `DR-forge-gitlab-11` records the addition as an accepted consequence. The WEBSITE half —
the public site's adoption and apps pages, which mirror these two documents — lives in a different
repo and lands as a COMPANION change tracked separately; this brief neither edits it nor gates on
it, and no Verify row here dereferences it.

**Op 40 — `RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error)`.**
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
- `tools/desk/internal/deskkit/forge.go` — op 40 `RepoHardeningRead` + `HardeningReadKind` +
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
- `tools/desk/cmd/desktoken/adopterdocs_test.go` (new) — `TestAdopterDocsEnumerateEveryRole` (planned): the
  docs-drift guard for the #857 ruling's docs half, the `secondcell_test.go` `adopterContract`
  shape. It walks `validRoles` and fails naming any role absent from `docs/adopting-assay.md` or
  from `docs/adopting-assay-gitlab.md`'s role table and token-file list, so a SEVENTH role added
  in code with no page naming it reddens CI instead of reaching an adopter as a Refused exit. The
  same test asserts the auditor's documented GitHub grant carries no `: write`.
- `tools/desk/internal/forgeban/allowlist.go` — remove `cmd/repohardenguard/check.go::ghRun::gh`;
  lower `allowedInvocationCeiling` by exactly the rows removed. `forgeban.go:173` — reword the
  comment so it no longer carries the literal the whole-tree grep counts.
- `tools/desk/internal/deskkit/echocoverage_test.go` — `exemptFromRoster["repohardenguard"]`
  reason updated (reads the forge map, never the write-authorisation set).
- `docs/streams/forge-gitlab/inventory.md` — op 40 row + delta paragraph (consumer:
  `repohardenguard`), the residual-call-sites table updated, the ceiling narrative.
- `docs/adopting-assay.md`, `docs/adopting-assay-gitlab.md` — provisioning the auditor identity,
  at EVERY place these pages enumerate a role or a permission set (the #857 ruling's docs half):
  GitHub — the §2 App inventory row and the §4 provisioning-checklist step that lists the Apps to
  create, both naming an App with `Metadata: read`, `Contents: read`, `Administration: read` and
  NO write permission of any kind; GitLab — the role→service-account table row (`auditor` |
  service account | Reporter (20) | `read_api` | GET-only hardening reads) and the per-role
  `gitlab-<role>.token` link/copy list, which must include `auditor`. Both pages state the
  accepted consequence in the adopter's own terms: the admin-gated rows report could-not-check
  under this identity and are re-run by a human administrator, and the grant is NOT widened to
  make them readable. The auditor is NOT added to the `--raised-by` attribution roles — it never
  files.
- `CHANGELOG.md`, the v1.0.8 section — the fragment (changelog/forge-gitlab-11-guard-read-custody.md) was consumed by the v1.0.8 changelog roll, so the fragment file no longer exists and the section is its record.
- Conditional: `tools/desk/cmd/deskroster/roster.go`, `forge.go`, tests — ONLY if #841 has
  closed unmerged at pickup; then re-land its diff here unchanged (same ops, same session-role
  custody).

single-point-of-failure: the FORGE-SIDE GRANT on the auditor identity is the one control between a
leaked or mis-provisioned guard token and a settings write — our code is GET-only, but code is not
a boundary a stolen token respects. It is backed by two independent layers that fail for different
reasons in different components: (1) the enumerated surface — op 40 takes a closed kind, not a
path, `TestForgeNoPassthrough` forbids any method that does, and the forgeban ratchet forbids a
shell-out around it, so the tool's own code path cannot be steered at a write endpoint; (2) the
forge's permission model — the auditor App / PAT carries no write scope, so a write attempted with
its token (bypassing our code entirely) is refused by the forge with a 403. Verify row 8 proves
layer 2 with layer 1 bypassed. A third, out-of-band signal already exists: the forgeban ceiling
and the inventory reflection redden CI on any new shell-out or method.

facts:
- Spec §6 freeze rule: op 40 lands WITH `repohardenguard` converted in the same change; the
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
- The human decision above is RECORDED: issue #857 is closed `human-decided` with **option 1**
  (the dedicated read-only `auditor` identity), approved as proposed, plus the documentation-and-
  website addition carried by Task 2 and Verify rows 11–13. `DR-forge-gitlab-11` is APPROVED and
  cites the ruling comment. Re-read the issue before starting: if what it records is not option 1,
  report NEEDS_CONTEXT — the Task below implements option 1, and the design record is amended,
  never silently re-targeted.
- The WEBSITE half of the ruling is a COMPANION change in the site repo, out of scope here. Do not
  edit it from this brief's branch and do not gate this brief on it.

## Task
1. **Custody.** Add `auditor` to `desktoken`'s `validRoles`; confirm the GitHub mint and the
   GitLab token-file path both resolve it by name with no further change (`desktoken --version`
   echoes `auditor=auditor-app`). Add the preflight remediation text for a missing/insecure
   `auditor` custody file.
2. **Adopter docs (the #857 ruling's docs half — a deliverable, not a follow-up).** Add the
   auditor at EVERY place the two adopter pages enumerate a role or a permission set, so no page
   an adopter reads is silent about a role they must provision:
   - `docs/adopting-assay.md` — the §2 App inventory row and the provisioning-checklist step that
     lists the Apps to create: an App with `Metadata: read`, `Contents: read`,
     `Administration: read` and **no write permission of any kind**, installed on the account,
     PEM at the config-home `0600` as `auditor-app.pem`. State the accepted consequence in the
     adopter's terms — the admin-gated rows (`security_and_analysis.*`, ruleset `bypass_actors`)
     report could-not-check under this identity and are re-run by a human administrator; the grant
     is NOT widened to make them readable. Do NOT add `auditor` to the `--raised-by` attribution
     roles: it never files.
   - `docs/adopting-assay-gitlab.md` — the role→service-account table gains
     `| auditor | service account | Reporter (20) | \`read_api\` | GET-only hardening reads for
     \`repohardenguard\`; no write scope |`, and the per-role `gitlab-<role>.token` link/copy list
     gains `auditor`.
   - `tools/desk/cmd/desktoken/adopterdocs_test.go` (new) — `TestAdopterDocsEnumerateEveryRole` (planned)
     reconciles `validRoles` against both pages and fails naming any role no page enumerates, and
     asserts the auditor's documented GitHub grant carries no `: write`. This is the guard that
     keeps the docs half from rotting on the next role.
   The WEBSITE half (the public site's adoption + apps pages) is a COMPANION change in the site
   repo — out of scope here, not edited from this branch, not gated on.
3. **Op 40.** Add `RepoHardeningRead` + `HardeningReadKind` + `ValidateHardeningReadKind` to
   `forge.go`; implement the six GitHub kinds (fixed literals; `rulesets` walks list→detail
   and returns the detail array); implement the GitLab named refusal; goldens for every kind,
   the zero-request unknown-kind refusal, and the GitLab coverage case. Update `inventory.md`
   (row 40, delta paragraph, residual-sites table) in the same change.
4. **Guard migration.** Replace `ghRun`/`ghGet` with the resolver seam (`forge.go`, fixed role
   `auditor`, roster-map resolution only); `Row.Endpoint()` → `Row.Kind()` over `read <kind>` /
   `read file <path>` (a `gh api` cell is a parse REFUSAL naming the vocabulary); file rows via
   `ReadFile`; `httpStatus` from `ForgeAPIError`; identity line from the role's known login;
   preflight `read repo`. Re-plumb the tests from the fake `gh` binary onto a stub `Forge`;
   every existing verdict (admin-null, 403, 404-public, 404-admin, wrong-repo scope, row census)
   unchanged. Add `TestChecklistRefusesGhApiCell` (planned).
5. **Closure.** Remove the `repohardenguard` permit row, lower the ceiling by the rows removed,
   reword the `forgeban.go:173` comment. If #841 is not on main at pickup, re-land its
   `deskroster` diff here first (rows and ceiling accordingly). fg/08 row 3 must read 0.
6. **Consumers.** Flip every `follow-up forge-gitlab/11` routing above to `fixed-here` in this
   change (the public-site entry stays `out-of-scope` — the companion change is not delivered
   here); run `statusgen --root . --consumers` before pushing.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | check:ci |
| 2 | `test -d tools/desk && { grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` — fg/08 row 3, verbatim, now closed. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. The `test -d` leg covers BSD grep, which stays silent on an absent directory under `--include`. | check +neighbour |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -v` | exit 0; output contains `PASS` — the ratchet reconciles at the lowered ceiling, op 40 passes the name and no-endpoint-argument checks, the inventory reflects 40 methods, every kind has a golden | check:ci |
| 4 | `grep -cE -e 'Key: +"cmd/repohardenguard/' -e 'Key: +"cmd/deskroster/' tools/desk/internal/forgeban/allowlist.go; test "$(grep -oE 'allowedInvocationCeiling = [0-9]+' tools/desk/internal/forgeban/allowlist.go \| grep -oE '[0-9]+$')" -le 6` | first line `0`; exit 0 — no permit row for either tool, ceiling at or below 6 | check |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden/hardening_read_unknown_kind -v` | exit 0; output contains `PASS` (`hardening_read_unknown_kind` (planned)) and the golden records ZERO requests — the kind validator refuses before a request exists | check +mutation |
| 6 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestChecklistRefusesGhApiCell -v && go test ./cmd/repohardenguard/ -run TestAdminNullIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestForbiddenIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestPublicNotFoundIsAbsent -v` | exit 0; output contains `PASS` — the old Read grammar is refused by name; the three-state verdicts survive the seam (`TestChecklistRefusesGhApiCell` (planned), `TestAdminNullIsCouldNotCheck` (planned), `TestForbiddenIsCouldNotCheck` (planned), `TestPublicNotFoundIsAbsent` (planned) — names the re-plumbed suite creates; the four verdict classes are the contract) | check +flow |
| 7 | `repohardenguard --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" --json \| jq -e '(.identity \| test("\\[bot\\]$")) and ([.rows[] \| select(.state=="could-not-check" and .gated!="admin")] \| length == 0)'` | exit 0 — the run names the auditor App as its identity (never a human login), and every could-not-check row is an admin-gated one (a public row never goes could-not-check under the auditor) | gate:human +dereference |
| 8 | `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer $(cat "$(desktoken auditor --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/[^/]+?(\.git)?$#\1#')")")" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" -d '{}'` | `403` — a settings WRITE attempted with the guard's token, bypassing the guard entirely, is refused by the FORGE (an empty PATCH changes nothing even where it would succeed, so the probe is side-effect-free) | gate:human +mutation |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | `1` — the role resolves through the existing binding echo | check |
| 10 | `statusgen --root . --consumers` | exit 0 — every `follow-up forge-gitlab/11` routing has flipped to `fixed-here` and the diff corroborates it | check |
| 11 | `cd tools/desk && go test ./cmd/desktoken/ -run TestAdopterDocsEnumerateEveryRole -v -timeout 60s` | exit 0; output contains `PASS` — every `validRoles` entry, `auditor` included, is enumerated by BOTH adopter pages, and the auditor's documented GitHub grant carries no `: write` (`TestAdopterDocsEnumerateEveryRole` (planned)). This is the anti-drift guard the #857 ruling's docs half needs: deleting the auditor from either page reddens it | check +mutation |
| 12 | `test "$(grep -c 'Administration: read' docs/adopting-assay.md)" -ge 1 && grep -qE '^[\|] *auditor *[\|].*read_api' docs/adopting-assay-gitlab.md && sed -n 's/.*for r in \(.*\); do.*/\1/p' docs/adopting-assay-gitlab.md \| grep -qw auditor` | exit 0 — the GitHub page names the auditor App's read-only grant, the GitLab role table carries the `auditor` / Reporter (20) / `read_api` row, and the per-role `gitlab-<role>.token` list includes `auditor`. An adopter who reads only the pages can provision the identity | check |
| 13 | `grep -hniE auditor docs/adopting-assay.md docs/adopting-assay-gitlab.md \| grep -viE -e 'no write' -e read-only -e 'never writes' -e could-not-check \| grep -E -e ': write' -e write_repository -e '`api`' -e 'Developer \(30\)' -e Maintainer; test $? -eq 1` | exit 0 — NEGATIVE control on the docs half: NO line that names the auditor also documents a write permission or a write-capable scope for it (`grep` exits 1 on no match, which is the pass). A documented grant an adopter copies is the grant the forge ends up enforcing, so row 8's runtime `403` is only as good as this row | check +mutation |

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
| The adopter docs describe a permission set the forge does not offer (a permission NAME that does not exist) | **no row** — review-only (dereferenced by the human who provisions the App in row 7's run; a wrong set shows as unexpected could-not-check rows) |
| The auditor is added to `validRoles` but no adopter page names it, so the first an adopter hears of the role is a Refused exit | row 11 (the drift guard walks `validRoles` against both pages) + row 12 |
| The docs tell the adopter to grant the auditor a write permission or a write-capable scope ("so it can see `bypass_actors`") | row 13 (the docs-side negative control) + row 8 (the runtime one, on the identity actually provisioned) |
| The GitLab role table gains the auditor but the per-role token-file list does not, so `gitlab-auditor.token` is never created | row 12 (all three greps must pass) |
| The website half is quietly treated as delivered because the docs half landed | **no row** — out of scope by the brief's Ground rules; the companion change is tracked separately and this brief asserts nothing about it |

### Dispatch checklist
```
[x] 1. Rows DISCRIMINATE — rows 8 and 13 are the negative controls (runtime and documented grant); rows 2/4/5 fail on a plausible half-migration; rows 11/12 fail on a half-done docs half.
[x] 2. Facts dated and checkable — sha 8953d38d, the row-3 count, the ceiling, the docs quotes all dated 2026-09-11; the ruling is #857, closed human-decided 2026-09-11.
[x] 3. Self-contained — the kind table, the custody shape, the docs surfaces and the row-3 literal are in this file.
[x] 4. Risk answers match files: — desktoken + forge backends + adopter provisioning = sensitive-data yes; nothing regulatory/customer/irreversible.
[x] 5. gate-why names the wire — a new identity's scope, and the admin-gated trade-off. RULED: #857, option 1 approved as proposed + the docs/website addition.
[x] 6. Effort honest — one op (six fixed reads), one tool, one role, two adopter pages + their drift guard; #841 carries the other tool; the website half is a companion change. M.
[x] 7. Shared value → consumers: enumerated; row 6 is the flow row (checklist → kind → backend → status → verdict).
[x] 8. Pre-mortem run; the two row-less failures are recorded as review-only / out-of-scope.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 7 and 8 need a
     provisioned auditor identity and are run by the human gate. -->
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — gate: human, risk.sensitive-data: yes — Evidence only, HELD at `implemented`

Runner ≠ implementer. Own detached temp worktree off `medici-finance/assay` origin/main at `57509073b9b7c989b850c7e5d251f7443ef3794c`.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go test ./...` | exit 0 | exit 0, all ~55 packages ok | 2026-09-17 | sonnet-5-verifier |
| 2 | grep for gh/glab exec.Command shell-outs | `0` | `0` | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestNoForgeCLIShellout`, `TestForgeNoPassthrough`, `TestForgeGitlabCoverage`, `TestForgeGithubGolden` | exit 0 | exit 0 all — ratchet ceiling 5, frozen surface 46 ops, coverage reconciles | 2026-09-17 | sonnet-5-verifier |
| 4 | permit-row grep + ceiling check | first line `0`, ceiling ≤6 | `0`; `allowedInvocationCeiling = 5` | 2026-09-17 | sonnet-5-verifier |
| 5 | `TestForgeGithubGolden/hardening_read_unknown_kind` | exit 0, zero requests | exit 0 PASS; golden fixture directly confirms `"requests": []` | 2026-09-17 | sonnet-5-verifier |
| 6 | 4 named repohardenguard tests | exit 0 | exit 0, all 4 PASS | 2026-09-17 | sonnet-5-verifier |
| 7 | live `repohardenguard --json` run (needs auditor token) | exit 0 | **could-not-check: `auditor` App not provisioned in this environment** (see finding below) | 2026-09-17 | sonnet-5-verifier |
| 8 | live PATCH probe with auditor token | `403` | **could-not-check: same reason as row 7** — `desktoken auditor` fails to mint (no `AUDITOR_APP_ID`, no `auditor-app.pem`) | 2026-09-17 | sonnet-5-verifier |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | `1` | `1` — binding exists even though the App itself isn't provisioned | 2026-09-17 | sonnet-5-verifier |
| 10 | `statusgen --root . --consumers` | exit 0 | exit 0 — no pending diff (post-merge); frontmatter itself already shows every consumer `fixed-here` | 2026-09-17 | sonnet-5-verifier |
| 11 | `TestAdopterDocsEnumerateEveryRole` | exit 0 | exit 0 PASS | 2026-09-17 | sonnet-5-verifier |
| 12 | 3-part adopter-docs grep | exit 0 | exit 0 — GitHub page names the read-only grant; GitLab role table has the auditor/Reporter(20)/read_api row; per-role token list includes auditor | 2026-09-17 | sonnet-5-verifier |
| 13 | negative control — no auditor+write line | exit 1 (no match) | **exit 1 expected, got a match (exit 0)** — `docs/adopting-assay.md:329` names `auditor` on the same line as a `: write` reference, but on inspection the write belongs to the UNRELATED `cell-issues` App ("a narrower... `write-issues` identity... the model **the auditor role above** follows") — a control-precision false-positive, not an actual write grant to auditor. Flagged for human judgment, not resolved by this verifier | 2026-09-17 | sonnet-5-verifier |

**Finding, filed separately (tracked internally — not resolvable from this repo):** the `auditor` App is not provisioned in this house environment. This means rows 7/8 — the actual runtime proof of the security boundary this brief exists to establish — cannot be independently verified by ANY verify-desk session until the App is provisioned, not just this one.

**RISK-VALUE — layered, one layer unverified this cycle:**
- Layer 1 (code-level containment): op 40 takes a closed kind, not a path; zero-request refusal on an unknown kind. VERIFIED (rows 3, 5).
- Layer 2 (forge-level containment — the actual point of the human gate): the live 403 refusal of a write attempt with the auditor's own token. **NOT VERIFIED this cycle** — could-not-check per the provisioning gap above, not a pass by default.
- GitHub grant documented (`docs/adopting-assay.md:328`): Metadata:read, Contents:read, Administration:read, explicitly no write. GitLab grant documented (`docs/adopting-assay-gitlab.md:121`, §5a): Reporter(20) for reads, Maintainer(40) for protected-branches/tags/approvals/push-rules reads — Maintainer is not inherently read-only on GitLab in general; the docs assert the write boundary comes from the PAT's `read_api` scope, server-enforced regardless of role. Architecturally consistent with the brief's own design principle (scope, not role, is the boundary), but not independently re-verified against a live GitLab instance this cycle (no credential available, and out of this brief's own scope).

**Net:** code-level containment solidly verified; forge-level containment — the actual human-gate substance — unverified this cycle for GitHub due to missing App provisioning, and row 13's negative control did not cleanly pass on the literal command (false-positive match, flagged not resolved). Both are directly relevant to the sign-off this item asks for.

**Gate statement:** `gate: human` + `sensitive-data: yes` — Evidence only, no verdict/flip made or attempted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE -e 'Key: +"cmd/repohardenguard/' -e 'Key: +"cmd/deskroster/' tools/desk/internal/forgeban/allowlist.go; test "$(grep -oE 'allowedInvocationCeiling = [0-9]+' tools/desk/internal/forgeban/allowlist.go \| grep -oE '[0-9]+$')" -le 6` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden/hardening_read_unknown_kind -v` | pass exit=0 | sha256:693497cf08a1 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestChecklistRefusesGhApiCell -v && go test ./cmd/repohardenguard/ -run TestAdminNullIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestForbiddenIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestPublicNotFoundIsAbsent -v` | pass exit=0 | sha256:186f7c45d8c3 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `repohardenguard --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" --json \| jq -e '(.identity \| test("\\[bot\\]$")) and ([.rows[] \| select(.state=="could-not-check" and .gated!="admin")] \| length == 0)'` | fail exit=4 | sha256:0087b96b601c | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer $(cat "$(desktoken auditor --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/[^/]+?(\.git)?$#\1#')")")" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" -d '{}'` | fail exit=6 | sha256:9a07e914d95b | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | fail exit=1 | sha256:6acc4612d178 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers` | pass exit=0 | sha256:1156598a8008 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/desktoken/ -run TestAdopterDocsEnumerateEveryRole -v -timeout 60s` | pass exit=0 | sha256:3d2ce09219a2 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 12 | `test "$(grep -c 'Administration: read' docs/adopting-assay.md)" -ge 1 && grep -qE '^[\|] *auditor *[\|].*read_api' docs/adopting-assay-gitlab.md && sed -n 's/.*for r in \(.*\); do.*/\1/p' docs/adopting-assay-gitlab.md \| grep -qw auditor` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -hniE auditor docs/adopting-assay.md docs/adopting-assay-gitlab.md \| grep -viE -e 'no write' -e read-only -e 'never writes' -e could-not-check \| grep -E -e ': write' -e write_repository -e '` | fail exit=2 | sha256:3b4acfef4ecb | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run — 2026-09-27 claude-opus-5-5-verifier (verify-desk dispatch) — gate: human, risk.sensitive-data: yes — Evidence only, HELD at `implemented`

Runner is not the implementer (implementing change: PR #1097, merge commit 7288a6d198329ae6ab2d57f2bbb3588a42631a99).
Own detached temp worktree at merged main e70bc86474f94b6e241d12857a10dcbd8136d556 (matches the forge's
reported main head). The witness table above was written by the PINNED statusgen v1.0.27 (sha256 matches the
release pin), run OFFLINE: under `sandbox-exec` with all network denied, a stripped environment (no forge
credential exported), and PATH limited to the system dirs plus `go` and the pinned `statusgen`. `desktoken`,
`repohardenguard` and every credential file were deliberately kept out of reach, because rows 7 and 8 mint an
auditor token and call the live GitHub API (row 8 is a PATCH). Rows 7, 8 and 9 in the table therefore record
what that envelope allowed, not a verdict on the code.

**Per-row notes (real output):**
- Row 1 (check:ci) — could-not-run on this darwin host, #1800. Out of witness, same sha: `go build ./...` in
  tools/desk exit 0; targeted `go test -timeout 300s` of the four touched packages (internal/deskkit,
  internal/forgeban, cmd/repohardenguard, cmd/desktoken) exit 0, all `ok`. The full-module test run was not
  executed here (dispatch rule: targeted tests only); it clears on a Linux runner.
- Row 2 — witness `fail exit=1` is a check-definition artifact, not a regression: the witness runs under
  `pipefail`, and a zero-match `grep -rnE` exits 1. The output hash 4eff2db4bada is the BSD `wc -l` padded
  count `       0` (reproduced: same hash, exit 1 under `bash -o pipefail`). The count the row asks for is 0.
- Row 3 (check:ci) — could-not-run, #1800. Out of witness, same sha: all four tests `--- PASS`
  (`TestNoForgeCLIShellout`, `TestForgeNoPassthrough`, `TestForgeGitlabCoverage`, `TestForgeGithubGolden`
  plus its count test, which reports 62 golden operations incl. seven `hardening_read_*` cases).
- Row 4 — pass: permit-row count `0`; `allowedInvocationCeiling = 6` (the ratchet equals the permit-list
  length; this brief moved it 7 → 6, a later change took it to 5 and another back to 6).
- Row 5 — pass. The golden fixture for `hardening_read_unknown_kind` records `"requests": []` and the refusal
  `could-not-check: "not-a-real-kind" is not a known hardening-read kind — …`.
- Row 6 — pass: all four repohardenguard verdict tests `--- PASS`.
- Row 7 — could-not-check (live forge; needs a provisioned auditor App, and no auditor custody material
  exists on this host). Witness `fail exit=4` is the envelope artifact: `repohardenguard` absent from the
  sandbox PATH, then `jq -e` on empty input exits 4. Exact probe for the human gate, as authored:
  `repohardenguard --repo "<owner>/<repo>" --json | jq -e '(.identity | test("\\[bot\\]$")) and ([.rows[] | select(.state=="could-not-check" and .gated!="admin")] | length == 0)'`.
- Row 8 — could-not-check (live forge write probe). Witness `fail exit=6` is `curl: (6) Could not resolve
  host` under the network deny. Exact probe for the human gate: the row-8 cell with the auditor token, i.e.
  `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer <auditor token>" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/<owner>/<repo>" -d '{}'`, expect `403`.
- Rows 7 and 8, check-definition defect found while reproducing: the slug derivation
  `sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#'` is not portable. macOS/BSD `sed` rejects it
  (`RE error: repetition-operator operand invalid`, empty slug). GNU `sed` accepts it but POSIX ERE has no
  lazy `+?`, so for an origin URL ending in `.git` it yields `<owner>/<repo>.git`, not `<owner>/<repo>`.
  Whoever runs rows 7/8 should pass the slug explicitly; otherwise row 8 can return a non-403 for the wrong
  reason.
- Row 9 — witness `fail exit=1` is the envelope artifact (`desktoken` kept off the sandbox PATH because row 8
  mints with it). Out of witness, offline: `desktoken --version` from a build of this sha prints
  `auditor=auditor-app`, count `1`; the pinned v1.0.27 `desktoken` also counts `1`.
- Row 10 — witness pass is VACUOUS on merged main (`consumers: no brief files in the diff … nothing to
  corroborate`). Supplement: the same pinned binary with `--base` at the implementing commit's parent reports
  this brief's seven `fixed-here` consumers CORROBORATED and none disproved. The other five entries are
  out-of-scope or `follow-up forge-gitlab/12`, UNCHECKED by design. No `follow-up forge-gitlab/11` routing
  remains anywhere under docs/.
  - Correction (2026-09-27 re-run): the count above is wrong. `statusgen --root . --consumers --base 7288a6d1^1`,
    pinned v1.0.27, run at 7288a6d198329ae6ab2d57f2bbb3588a42631a99 (an isolated clone checked out at that
    commit, network denied), exits 0 with `summary: 7 corroborated, 0 disproved, 4 unchecked`. The four
    UNCHECKED entries are the three out-of-scope rows (website pages, deskroster, the adopter's checklist
    document) and the one `follow-up forge-gitlab/12` row: four, not five.
- Row 11 — pass: `TestAdopterDocs…EveryRole` `--- PASS`.
- Row 12 — pass (exit 0, empty output).
- Row 13 — witness `fail exit=2` is a TRUNCATED command cell: the cell's own backticks around `api` end the
  code span early, so the witness ran a fragment with an open quote (shell syntax error). The full command,
  run by hand at the same sha, is a real FAIL: exit 1, because the final grep matches
  docs/adopting-assay.md line 359. That line is the **cell-issues** App's row (`issues: write`) and names
  the auditor only as a cross-reference ("the model the auditor role above follows"). Nothing grants the
  auditor a write permission. The auditor's own rows (adopting-assay.md line 358: `Metadata: read`,
  `Contents: read`, `Administration: read`, "no write permission of any kind"; adopting-assay-gitlab.md line
  142: `read_api`, "no write scope") carry no write grant. So this is a false positive in the negative
  control. The implementing change added that line, so the row has never passed as written. Same finding
  as the 2026-09-17 run. It needs a row fix or a human ruling.

**Risk-bearing value enumeration** (literals and authority bindings this item's diff introduces or changes,
plus those its Deliverables name). Ranked by irreversibility, most sensitive first:
1. Documented GitHub grant for the auditor App: `Metadata: read`, `Contents: read`, `Administration: read`,
   no write @ docs/adopting-assay.md:358. This is the single forge-side control (the SPOF line). An adopter
   copies it, and the forge then enforces it for as long as the App exists. Undoing a mis-provisioned App is
   an out-of-band human act, not a redeploy.
2. Documented GitLab grant for the auditor PAT: scope `read_api`, role `Reporter (20)` @
   docs/adopting-assay-gitlab.md:142. This item introduced `Reporter (20)`. A later item (forge-gitlab/12)
   added `Maintainer (40)` to the same row for four reads, which is out of this diff but rests on the same
   `read_api` boundary claim.
3. Authority binding `validRoles["auditor"] = true` @ tools/desk/cmd/desktoken/desktoken.go:42.
4. Fixed acting role `"auditor"` @ tools/desk/cmd/repohardenguard/forge.go:73 (resolve) and :92
   (`desktoken auditor --repo` mint). The identity line, `AppBinding("auditor") + "[bot]"`, is at
   forge.go:120.
5. The six fixed GitHub GET literals @ tools/desk/internal/deskkit/forge_github.go:2289-2295 (`/repos/%s/%s`,
   `…/actions/permissions/workflow`, `…/fork-pr-contributor-approval`, `…/fork-pr-workflows-private-repos`,
   `…/private-vulnerability-reporting`) and :2341/:2350 (`…/rulesets`, `…/rulesets/%d`).
6. `allowedInvocationCeiling = 6` @ tools/desk/internal/forgeban/allowlist.go:84. This is a reversible
   ratchet, ranked last.

RISK-VALUE: NAMED, NOT DERIVED — auditor GitHub grant = `Metadata: read`, `Contents: read`, `Administration: read`, no write @ docs/adopting-assay.md:358 — two derivations are missing. (a) That this set is SUFFICIENT for the six kinds plus ReadFile, with each endpoint's permission requirement checked against GitHub's own permission reference. (b) That no member of it grants a settings write, checked on a provisioned App (row 8's 403). Neither can be done offline: no auditor App is provisioned and live forge calls are outside this envelope. **Open question for the human.**
RISK-VALUE: NAMED, NOT DERIVED — auditor GitLab grant = scope `read_api`, role `Reporter (20)` @ docs/adopting-assay-gitlab.md:142 — missing: a live check that a `read_api` PAT is refused on a write whatever the project role (it matters more now that the same row lists `Maintainer (40)`). No GitLab instance or credential is in this envelope. **Open question for the human.**
RISK-VALUE: DERIVED — `validRoles["auditor"] = true` @ tools/desk/cmd/desktoken/desktoken.go:42 — the #857 ruling (option 1, a dedicated read-only auditor identity) requires exactly one new role, keyed by name so the existing role-parameterised mint and token-file lookups resolve it. Row 9 (out of witness) and row 11 confirm that both adopter pages enumerate it.
RISK-VALUE: DERIVED — acting role `"auditor"` @ tools/desk/cmd/repohardenguard/forge.go:73 and :92 — the design fixes the role instead of reading the loop variable (the guard runs outside a desk window). The custody hook refuses when no auditor token was minted (forge.go:45-56): "never falls back to an ambient forge identity". This matches the ruling and the "never ambient" resolver contract.
RISK-VALUE: DERIVED — the six GitHub GET literals @ tools/desk/internal/deskkit/forge_github.go:2289-2295, 2341, 2350 — each one matches the brief's kind table verbatim. The map holds one literal per kind, with no caller-supplied path segment, and only `http.MethodGet` is used.
RISK-VALUE: DERIVED — `allowedInvocationCeiling = 6` @ tools/desk/internal/forgeban/allowlist.go:84 — forgeban_test.go:330 requires it to equal `len(AllowedInvocations)`. At the implementing commit, six named-`gh` permit rows remained, as the brief's facts predicted. Reversible.

**Observations for the human gate:**
- Row 7's identity assertion (`.identity` ends in `[bot]`) does not discriminate: `identity()` is a display
  string built from the auditor's App binding, not a whoami, so any completed run satisfies it. The
  no-ambient guarantee comes from the custody hook's refusal, which this verifier confirmed by reading the
  code, not by a live run.
- The rulesets walk GETs the list without pagination (GitHub's default page size). On a repo with more
  rulesets than one page, a named ruleset beyond page one would read as absent: a false could-not-check or
  wrong, never a false pass.
- A darwin human running rows 7/8 exactly as authored hits the BSD `sed` error above.

VERIFY: BLOCKED — offline rows 4, 5, 6, 11 and 12 genuinely pass, and rows 2, 3 and 9 pass on substance
outside the witness. Row 13 FAILS as authored (a negative-control false positive: check-definition, not a
real write grant). Rows 7 and 8, the forge-side proof this gate exists for, are could-not-check pending a
provisioned auditor App and a human-run probe with an explicit repo slug. Rows 1 and 3 are check:ci
could-not-run on darwin (#1800). Evidence only; status stays `implemented`; no flip made or attempted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE -e 'Key: +"cmd/repohardenguard/' -e 'Key: +"cmd/deskroster/' tools/desk/internal/forgeban/allowlist.go; test "$(grep -oE 'allowedInvocationCeiling = [0-9]+' tools/desk/internal/forgeban/allowlist.go \| grep -oE '[0-9]+$')" -le 6` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden/hardening_read_unknown_kind -v` | fail exit=1 | sha256:b356dfa9eded | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestChecklistRefusesGhApiCell -v && go test ./cmd/repohardenguard/ -run TestAdminNullIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestForbiddenIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestPublicNotFoundIsAbsent -v` | pass exit=0 | sha256:186f7c45d8c3 | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `repohardenguard --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" --json \| jq -e '(.identity \| test("\\[bot\\]$")) and ([.rows[] \| select(.state=="could-not-check" and .gated!="admin")] \| length == 0)'` | fail exit=4 | sha256:0087b96b601c | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer $(cat "$(desktoken auditor --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/[^/]+?(\.git)?$#\1#')")")" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" -d '{}'` | fail exit=6 | sha256:9a07e914d95b | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | fail exit=1 | sha256:6acc4612d178 | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers` | pass exit=0 | sha256:3b363364f3f0 | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/desktoken/ -run TestAdopterDocsEnumerateEveryRole -v -timeout 60s` | pass exit=0 | sha256:3d2ce09219a2 | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 12 | `test "$(grep -c 'Administration: read' docs/adopting-assay.md)" -ge 1 && grep -qE '^[\|] *auditor *[\|].*read_api' docs/adopting-assay-gitlab.md && sed -n 's/.*for r in \(.*\); do.*/\1/p' docs/adopting-assay-gitlab.md \| grep -qw auditor` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -hniE auditor docs/adopting-assay.md docs/adopting-assay-gitlab.md \| grep -viE -e 'no write' -e read-only -e 'never writes' -e could-not-check \| grep -E -e ': write' -e write_repository -e '` | fail exit=2 | sha256:3b4acfef4ecb | 2026-09-27 | assay-verifier-app[bot] @ b857f792d2d3 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-09-27 at the #1801 merge tree b857f792, after #1684 changed forgeban/allowlist.go**
(claude-opus-5-5-verifier, verify-desk dispatch; runner is not the implementer). The witness table
directly above ran at b857f792d2d3b185f9cb7798d07df74ffa917c1b, a local merge of PR #1801's head
0ad91ca527f40803d4c743540f75e4d4ca29c02a with main 46af8d389d5e02eb9a30c1c723709199d4b43fe5 (the
forge's main head at run time, read with the verifier App token). The envelope matches the first run:
pinned statusgen v1.0.27 invoked directly, `sandbox-exec` denying all network, `env -i`,
KUBECONFIG=/dev/null, and PATH limited to the system dirs plus `go` and the pinned `statusgen`, so
`desktoken`, `repohardenguard` and `gh` do not resolve. What #1684 changed:
it added one entry to the separate `UnresolvedArgv` list in tools/desk/internal/forgeban/allowlist.go
(the deskinbox flow reader seam, which runs statusgen or deskboard). `AllowedInvocations` and the
ceiling are untouched, and nothing else in the four touched packages (internal/deskkit,
internal/forgeban, cmd/repohardenguard, cmd/desktoken) differs from e70bc864.

Per row, against the first run:
- Row 1: unchanged, check:ci could-not-run on darwin (medici-finance/assay#1800). Out of witness, same
  tree, network denied except loopback: `go build ./...` in tools/desk exit 0; targeted package tests
  (`go test -count=1 -timeout 300s` of internal/forgeban, cmd/repohardenguard, cmd/desktoken,
  internal/deskkit) exit 0, all four `ok`. The whole-module `go test ./...` was not run here.
- Row 2: unchanged, `fail exit=1` with the same hash 4eff2db4bada (the padded count `0` under
  pipefail). The count is 0.
- Row 3: unchanged, check:ci could-not-run (#1800). Out of witness, same envelope as row 1:
  `go test -count=1 -timeout 300s ./internal/deskkit/ -run` over the four named tests, exit 0, each
  `--- PASS`, golden corpus 62 operations.
- Row 4: unchanged, pass, same hash 9a271f2a916b (output `0`). `allowedInvocationCeiling = 6` @
  tools/desk/internal/forgeban/allowlist.go:84 equals the six `AllowedInvocations` permit rows, and the
  forgeban ratchet test passes in the targeted run above. #1684's new entry sits in `UnresolvedArgv`,
  which the ceiling does not count.
- Row 5: CHANGED. The witness shows `fail exit=1` (hash b356dfa9eded). This is an envelope artifact,
  not a regression. The golden server is an `httptest` listener on loopback, and the all-network deny
  also refuses a loopback bind (`panic: httptest: failed to listen on a port: listen tcp6 [::1]:0:
  bind: operation not permitted`). With loopback allowed and all other network still denied, the same
  command exits 0 with `--- PASS: TestForgeGithubGolden/hardening_read_unknown_kind`. The first run's
  row-5 `pass` was a Go test-cache replay: its hash 693497cf08a1 is byte-for-byte the `ok … (cached)`
  output left by a loopback-capable run of the same package inputs (reproduced), so it was not an
  execution inside the envelope. Row 5 is not witness-proven in either run. Its substance is proven
  only out of witness.
- Row 6: unchanged, pass, same hash 186f7c45d8c3, which is again a cache replay (reproduced). Forced
  uncached (`GOFLAGS=-count=1`) in the same strict envelope, all four tests `--- PASS`, exit 0. These
  tests do not listen on loopback.
- Rows 7 and 8: unchanged, same exits (4, 6) and hashes. Both are could-not-check: they need the live
  forge, and no auditor App is provisioned. Nothing was minted, and no request left the host.
- Row 9: unchanged envelope artifact (`desktoken` kept off PATH). Out of witness, a build of this tree
  prints `bindings=auditor=auditor-app …`, count `1`.
- Row 10: new hash 3b363364f3f0, still VACUOUS. Against main, the diff holds only the three Evidence
  edits, so every entry reads UNCHECKED "unchanged since the merge-base" (`summary: 0 corroborated,
  0 disproved, 16 unchecked, 1 brief(s) claiming nothing` across the three briefs in the diff). The
  real count at the implementing commit is in the Correction under the first run's row-10 note:
  7 corroborated, 0 disproved, 4 unchecked.
- Rows 11 and 12: unchanged, pass, same hashes. Row 11 forced uncached in the strict envelope also
  shows `--- PASS`, exit 0.
- Row 13: unchanged, `fail exit=2` (the truncated command cell). The negative-control false-positive
  finding above still stands.

Witness-proven this run: rows 4, 6, 11 and 12. Held: rows 1, 2, 3, 5, 7, 8, 9, 10 and 13.
Risk-bearing values: the enumeration above is unchanged. #1684 introduces no literal in this item's
scope, and the ceiling stays `allowedInvocationCeiling = 6` @ tools/desk/internal/forgeban/allowlist.go:84
(reversible ratchet). The two RISK-VALUE: NAMED, NOT DERIVED lines (the auditor GitHub and GitLab
grants) remain open questions for the human.

VERIFY: BLOCKED (unchanged in substance). No implementation regression from #1684. Row 5 lost its
witness pass to the loopback deny and still passes out of witness. Rows 7 and 8 still await a
provisioned auditor App and a human-run probe, and row 13 still needs a row fix or a human ruling.
Evidence only; status stays `implemented`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE -e 'Key: +"cmd/repohardenguard/' -e 'Key: +"cmd/deskroster/' tools/desk/internal/forgeban/allowlist.go; test "$(grep -oE 'allowedInvocationCeiling = [0-9]+' tools/desk/internal/forgeban/allowlist.go \| grep -oE '[0-9]+$')" -le 6` | pass exit=0 | sha256:9a271f2a916b | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGithubGolden/hardening_read_unknown_kind -v` | fail exit=1 | sha256:ce391305ecdf | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestChecklistRefusesGhApiCell -v && go test ./cmd/repohardenguard/ -run TestAdminNullIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestForbiddenIsCouldNotCheck -v && go test ./cmd/repohardenguard/ -run TestPublicNotFoundIsAbsent -v` | pass exit=0 | sha256:e9581cfbef81 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `repohardenguard --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" --json \| jq -e '(.identity \| test("\\[bot\\]$")) and ([.rows[] \| select(.state=="could-not-check" and .gated!="admin")] \| length == 0)'` | fail exit=4 | sha256:0087b96b601c | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `curl -sS -o /dev/null -w '%{http_code}' -X PATCH -H "Authorization: Bearer $(cat "$(desktoken auditor --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/[^/]+?(\.git)?$#\1#')")")" -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')" -d '{}'` | fail exit=6 | sha256:9a07e914d95b | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 9 | `desktoken --version \| grep -c 'auditor=auditor-app'` | fail exit=1 | sha256:6acc4612d178 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers` | pass exit=0 | sha256:c53bb5f05568 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/desktoken/ -run TestAdopterDocsEnumerateEveryRole -v -timeout 60s` | pass exit=0 | sha256:286a22766366 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 12 | `test "$(grep -c 'Administration: read' docs/adopting-assay.md)" -ge 1 && grep -qE '^[\|] *auditor *[\|].*read_api' docs/adopting-assay-gitlab.md && sed -n 's/.*for r in \(.*\); do.*/\1/p' docs/adopting-assay-gitlab.md \| grep -qw auditor` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -hniE auditor docs/adopting-assay.md docs/adopting-assay-gitlab.md \| grep -viE -e 'no write' -e read-only -e 'never writes' -e could-not-check \| grep -E -e ': write' -e write_repository -e '` | fail exit=2 | sha256:3b4acfef4ecb | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |

**Re-run (round 2) 2026-09-27 at the #1801 merge tree 25fb2a4b, after #1685 added the ExpectedSHA conditional-write precondition to forge.go / forge_github.go / forge_gitlab.go**
(claude-opus-5-5-verifier, verify-desk dispatch; runner is not the implementer). The witness table
directly above ran at 25fb2a4b3ea34c8b3a5c7acbe540262f8e76309c, a local merge of PR #1801's head
2f1e152f3baa93f0d06d9ccdcde4690c2b9dec16 with main c50a38fc12518a4eec4db37e8dd847d49e79149a (the
forge's main head at run time, read with the verifier App token). Its rows are dated 2026-09-28
because the witness stamps UTC. Envelope, as in round 1: pinned statusgen v1.0.27 invoked directly
(sha256 matches the darwin-arm64 pin), `sandbox-exec` with a STRICT all-network deny (no loopback
allowance), `env -i`, KUBECONFIG=/dev/null, GOFLAGS=-count=1, a fresh empty GOCACHE, GOPROXY=off
over a pre-populated module cache, GOTOOLCHAIN=local, and PATH limited to the system dirs plus `go`
and the pinned `statusgen`. `desktoken`, `repohardenguard`, `gh` and `glab` do not resolve
(checked before the run); `curl` inside the sandbox cannot resolve a host. HOME pointed at a scratch
dir holding only the roster config the Runner stamp reads. Two earlier attempts this session also
set TMPDIR to a scratch path, a departure from round 1; their tables were discarded (path-specific
restore) and both briefs re-run without it. Every row had the same exit in all three attempts.

What #1685 changed in this item's inputs: a new `WriteFileInput.ExpectedSHA` field and a
backend-neutral `expectedSHAPrecondition` helper in tools/desk/internal/deskkit/forge.go (+30), and
one call to it inside `GitHubForge.WriteFile` (forge_github.go +8) and `GitLabForge.WriteFile`
(forge_gitlab.go +5), right after each backend's own pre-write fetch. It is on the WRITE path only.
`RepoHardeningRead`, the kind table, the kind-to-forge map, the per-kind paths and the goldens are
untouched; tools/desk/internal/forgeban/allowlist.go is unchanged since round 1. The +30 lines sit
above the hardening-read block in forge.go, so line cites there moved down by 30 (the kind map
`hardeningKindForge` is now forge.go:1053); forge.go:45 / 73 / 120, forge_github.go:2289 and
allowlist.go:84 are unchanged.

Per row, against round 1:
- Row 1: unchanged, check:ci could-not-run on darwin (medici-finance/assay#1800). Out of witness,
  same tree, network denied except loopback: `go build ./...` in tools/desk exit 0 (also exit 0
  under the strict deny); targeted package tests `go test -count=1 -timeout 480s` of
  internal/deskkit, internal/forgeban, cmd/repohardenguard and cmd/desktoken exit 0, all four
  `ok`. The whole-module `go test ./...` was NOT run here. #1685's own test
  (`TestWriteFileOpBothBackends`, in internal/deskkit) passes in that run.
- Row 2: unchanged, `fail exit=1`, same hash 4eff2db4bada. The printed count is `0`; the non-zero
  exit is the `grep -v` stage selecting nothing under pipefail.
- Row 3: unchanged, check:ci could-not-run (#1800). Out of witness, same loopback envelope: the four
  named tests each `--- PASS`, exit 0; golden corpus 62 operations; the frozen surface is 54
  operations and the committed inventory reconciles.
- Row 4: unchanged, pass, same hash 9a271f2a916b (output `0`); ceiling still 6 @ allowlist.go:84.
- Row 5: unchanged in substance, `fail exit=1` under the strict deny: the golden server cannot bind
  loopback (`panic: httptest: failed to listen on a port … bind: operation not permitted`). The hash
  (now ce391305ecdf) differs run to run because the panic trace carries addresses. With loopback
  allowed and all other network denied, uncached, the same command exits 0 with
  `--- PASS: TestForgeGithubGolden/hardening_read_unknown_kind`; the golden still records
  `"requests": []` and a could-not-check refusal naming the kind. Not witness-proven.
- Row 6: pass, NEW hash e9581cfbef81. Round 1's hash was a Go test-cache replay; this run is
  uncached (fresh GOCACHE, -count=1) inside the strict deny, all four tests `--- PASS`. This row
  is now genuinely witness-proven by execution. The hardening-read result did not move.
- Rows 7 and 8: unchanged, same exits (4, 6) and hashes. Could-not-check: they need the live forge
  and a provisioned auditor App. Nothing was minted, and no request left the host.
- Row 9: unchanged envelope artifact (`desktoken` kept off PATH), same exit and hash.
- Row 10: pass, new hash c53bb5f05568, still VACUOUS. The base is now c50a38fc and the diff holds
  only Evidence edits, so every entry reads UNCHECKED "unchanged since the merge-base" (`summary: 0
  corroborated, 0 disproved, 16 unchecked, 1 brief(s) claiming nothing`), as in round 1.
- Row 11: pass, NEW hash 286a22766366, now uncached inside the strict deny (`--- PASS`, exit 0).
  Round 1's witness hash was a cache replay.
- Row 12: unchanged, pass, same hash.
- Row 13: unchanged, `fail exit=2` (the truncated command cell); the negative-control finding
  above still stands.

Witness-proven this run: rows 4, 6, 11 and 12. Held: rows 1, 2, 3, 5, 7, 8, 9, 10 and 13.
No hardening-read row's result moved, and no implementation regression from #1685. Risk-bearing
values are unchanged; the two RISK-VALUE: NAMED, NOT DERIVED lines (the auditor GitHub and GitLab
grants) remain open questions for the human.

VERIFY: BLOCKED (unchanged in substance). Rows 7 and 8 still await a provisioned auditor App and a
human-run probe, row 5 passes only with loopback allowed, and row 13 still needs a row fix or a
human ruling. Evidence only; status stays `implemented`.

## Review
Gate: human (from frontmatter — a risk answer is yes). Human gate is MANDATORY. Reviewer records
verdict + date in the stream README table. The reviewer answers the two core-system questions:
(1) the single control between a leaked guard token and a settings write is the forge-side grant
on the auditor identity — is a read-only App / `read_api` PAT an acceptable single control given
the enumerated surface and the ratchet behind it? (2) does row 8 prove the LOWER layer (the forge's
permission model) refuses with the UPPER layer (our GET-only code, the kind validator) bypassed —
i.e. is the probe genuinely outside our code path? A Security-Review verdict is required on the
implementing PR (custody change on a public repo). The reviewer also reads the #857 ruling's docs
half as a control, not as prose: does what the two adopter pages tell an adopter to PROVISION match
the grant the design accepted — no write permission, no write-capable scope, the admin-gated rows
left as could-not-check — and would a permission name that the forge does not actually offer be
caught anywhere but here? (It is the one pre-mortem row with no Verify row.)
