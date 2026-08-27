# Forge portability: GitLab v0.1-design — the Assay fleet on GitLab Enterprise

**Version:** v0.1-design
**Status:** design draft for review — no implementation exists. MUST/SHOULD language
expresses design intent for an eventual normative profile, not a conformance claim.
**Scope note:** this is the design specification only. Every implementation
deliverable — forge-adapter code, provisioning script, site pages — lives alongside
the reference implementation and the site, not in this document.

## 1. Scope and motivation

Assay's identity model, gates, and tooling are currently GitHub-shaped: eight GitHub
Apps, installation tokens minted per use, a `workflows` permission class guarding CI,
branch rulesets with a single bypass identity. This document designs the **GitLab
profile**: how a team on GitLab (self-managed EE or gitlab.com, Premium/Ultimate — 
"Enterprise" throughout) runs the same methodology with equivalent — and where they
are weaker, *disclosed* — guarantees.

Two audiences: adopters who live on GitLab and will never move; and the portability
claim itself — a methodology that only runs on one forge is a product feature, not a
methodology. GitLab is the first non-GitHub profile because it is the largest
enterprise population and because its primitive set is *almost* sufficient — the
gaps are instructive and each has a compensating control (§4, §5).

## 2. Non-goals

- **Not a migration tool.** Nothing here moves repos, issues, or history between
  forges.
- **Not GitLab CE/Free parity.** The profile targets Premium as the floor with
  Ultimate for two named capabilities (§4.3); a Free-tier degradation is described
  (§4.4) but its weaker guarantees make it a disclosed-limits mode, not a
  recommendation.
- **Not a second methodology.** Briefs, registers, lifecycle, the board, and
  statusgen are forge-agnostic already (git + markdown) and are unchanged. Only the
  identity/permission/CI layer and the forge-speaking desk tools are in scope.
- **Other forges** (Bitbucket, Gitea, Forgejo) are out of scope; the adapter
  interface (§7) is designed so they become profiles, not rewrites.

## 3. Identity model — the eight-role fleet on GitLab

GitLab has no GitHub-Apps equivalent (no manifest flow, no per-resource permission
matrix, no JWT→installation-token mint). The fleet maps to **service accounts**:
seatless bot users (Premium+) owned by the top-level group, one per role, each
authenticating with its own personal access token.

| Assay role | GitLab identity | Access level / mechanism |
|---|---|---|
| reviewer | service account `<prefix>-assay-reviewer` | Developer; MR notes + approvals. Ultimate: custom role without push (§4.3) |
| worker | service account `-worker` | Developer; branches + Draft MRs |
| verifier | service account `-verifier` | Developer (commits Evidence); excluded from MR approval eligibility (§5.2) |
| desk | service account `-desk` | Developer; coordination via MRs |
| issue-loop | service account `-issue-loop` | Reporter (issues need no code access) |
| intake-loop | service account `-intake-loop` | Reporter |
| board-writer | service account `-board-writer` | Developer + **allowed-to-push entry on the protected `main`** — the direct analog of the ruleset bypass |
| promote | **usually not an identity at all** — see §5. Where a team wants the identity anyway: service account `-promote`, Maintainer on the ci-config project only |

Attribution holds: notes, approvals, and commits display the service-account
username, an identity the author's token cannot produce — the same separation claim
as the GitHub Apps, with the same honest limit (separation of attribution, not proof
of diligence; `spec/README.md` normative preamble applies verbatim).

Provisioning: no manifest buttons. The whole fleet is REST-creatable (service
accounts, memberships, tokens), so the deliverable is one idempotent
`create-fleet-gitlab.sh` (home: `assay` repo) replacing the eight manifest clicks;
the site page (§8) documents it plus the by-hand path.

## 4. Permissions — the coarse-role problem and its ladder

### 4.1 The gap

GitHub Apps grant per-resource (`issues: write`, `contents: read`). GitLab grants a
**role** (Reporter/Developer/Maintainer) whose permission bundle is fixed: a
reviewer bot that can comment can, at Developer, also push. Token *scopes* (`api`,
`read_repository`, `write_repository`) narrow the API surface per token but not
per-resource semantics.

### 4.2 Premium compensations (the floor)

- **Protected branches** with explicit allowed-to-push/allowed-to-merge lists —
  no bot pushes `main` except board-writer; merge stays allowed-to-merge = humans.
- **MR approval rules**: prevent-author-approval + prevent-committers-approval ON —
  encodes no-self-approval and keeps the verifier (an Evidence committer) out of
  the approval set structurally.
- **Push rules** (commit author/branch-name regex) for branch hygiene.
- **Token scopes**: reviewer/loop tokens carry `api` only (no `write_repository`);
  worker/verifier/board-writer carry `write_repository`. This is the real
  per-role narrowing lever at Premium and MUST be used.

### 4.3 Ultimate refinements (named, optional)

- **Custom roles**: compose a reviewer role with MR permissions but no push —
  restores most of the GitHub granularity where it matters most.
- **External status checks**: an outside service posts a required check on MRs —
  the natural carrier for Security-Review-style lane verdicts
  without granting the lane any repo write.

### 4.4 Free/CE — non-conforming for this profile

Project access tokens instead of service accounts, no approval-rule enforcement,
no custom roles. statusgen and the board still run (forge-agnostic), but the
security-parity requirement (§6a) cannot be met — so Free/CE is **not a
conforming deployment of this profile**, and the site MUST say so plainly rather
than offer a "limits disclosed" wink. An adopter may still use the methodology's
forge-agnostic parts there; they may not claim the profile.

## 5. CI isolation — where GitLab is actually cleaner

### 5.1 The gap that isn't

GitLab has **no `workflows` permission class**: `.gitlab-ci.yml` is an ordinary
file, so any write-capable identity can edit CI. The GitHub answer (the promote App
holding the fleet's only `workflows` grant) has no analog — and doesn't need one,
because GitLab can move the CI definition out of the writable repo entirely:

- **External CI config path** (all tiers): a project's CI configuration can point
  at a file in a *different project* (`ci-config@group/ci-config:pipelines/x.yml`).
- **Compliance pipelines / pipeline execution policies** (Ultimate): the group
  *enforces* an injected pipeline regardless of what the repo's own file says.

### 5.2 The design

One locked **ci-config project** per group: only humans hold Maintainer, protected
`main`, approval rules on. Every fleet-managed project sets its CI config path
there (Ultimate groups additionally pin it by policy so a project cannot opt out).
The staged-promote pattern (the 3+1 reusable-workflow centralization) then collapses
to an ordinary flow: workers open MRs *against the ci-config project*; the human
merge IS the promotion. No promote identity, no staging directory, no dispatch
click — the isolation is structural rather than procedural, and this profile
SHOULD present it as the preferred shape even to GitHub-side readers evaluating
both. Worker bots are simply never granted membership in the ci-config project.

### 5.3 Rollout note

The reusable-workflow centralization already chosen for GitHub (3+1) is the same
move: GitLab `include:`-from-project is the analog of `workflow_call`, so the house
CI content ports as includes with per-project stubs — and on GitLab even the stubs
live outside the consumer repo.

## 6. Token custody — parity by rotate-on-mint

GitHub: short-lived installation tokens minted per use from a locally-held PEM.
GitLab has no mint-on-use — PATs are long-lived — so naive PAT handling would be a
custody *downgrade*, which §6a forbids. The profile closes the gap with a
different mechanism of equal effect:

- **Rotate-on-mint.** `desktoken --forge gitlab <role>` calls the token rotation
  API on every mint: the API returns a fresh token and **invalidates the old one
  atomically**. The live credential is therefore always the most recent mint —
  exposure of any captured token ends at the next mint, and at most one token per
  role is valid at any moment (a property the GitHub profile does not even have,
  where several installation tokens can be live inside their hour). Concurrency:
  role sessions already serialize on role identity (one window per role); where a
  role genuinely needs parallel actors, per-actor service accounts, not shared
  tokens.
- **Backstop expiry.** Instance/group token-lifetime policy set SHORT (7 days
  RECOMMENDED) so a fleet that stops minting leaves no long-lived credential
  behind; rotation refreshes the expiry on every mint.
- **File custody unchanged**: token files at `~/.config/<house>/gitlab-<role>.token`,
  0600, path-only printing, never env or argv — the existing desktoken discipline
  verbatim.
- **Audit events** (Premium+) streamed/reviewed for token rotation and use — the
  detection lane the GitHub profile gets from App-token audit logs.

Net custody verdict: **parity, by a different mechanism** — bounded-lifetime,
single-valid-credential, on-disk-custody — with the residual difference (rotation
window vs fixed one-hour TTL) stated in the conformance text, not hidden.

## 6a. Security parity — the governing requirement

**Governing requirement: the GitLab profile MUST be at least as secure as the
existing GitHub controls, even where the mechanism is completely different.**
Parity is assessed control-by-control; a control with no equal-or-stronger GitLab
mechanism at the deployment's tier makes that deployment non-conforming — weaker
plus disclosure is NOT acceptable; disclosure covers residual mechanics only.

| GitHub control (today) | GitLab mechanism (this profile) | Verdict |
|---|---|---|
| Per-resource App permissions | Premium: role + token-scope narrowing + protected branches; Ultimate: custom roles | parity at Ultimate; at Premium, parity **on protected lines** (main, release/*) with residual bot-push to unprotected branches — compensate with branch-name push rules; risk-classed repos REQUIRE Ultimate (open Q4 resolved in favor of the stricter floor) |
| Ruleset bypass = single board-writer identity | Protected branch allowed-to-push list containing exactly the board-writer | parity (direct analog) |
| No self-approval (reviewer ≠ author) | Approval rules: prevent-author + prevent-committers | parity |
| Required leak-sweep / CI checks before merge | "Pipelines must succeed" + required approvals; Ultimate: external status checks for lane verdicts | parity |
| `workflows` permission class guarding CI | Locked ci-config project + external CI config path; Ultimate: pipeline execution policy pins it | **stronger** (§5) — CI definition is outside the writable repo entirely |
| Staged-promote, human-dispatched | Human-merged MR into the ci-config project | parity, fewer moving parts |
| Short-lived minted tokens | Rotate-on-mint + short expiry policy (§6) | parity (single-valid-credential property is stronger; TTL shape differs) |
| Secret push protection / leak scanning | Push rules secret checks (Premium) + Secret Detection (Ultimate); the house leaksweep runs forge-agnostically in CI regardless | parity (house sweep carries the bar; forge scanning is additive) |
| Immutable release integrity | Protected tags + audit events; releases are deletable → compensate: release artifacts also pinned by sha256 in `.assay-versions` (already the house pattern, forge-agnostic) | parity via the existing pin discipline |
| Merge is always the human's | Allowed-to-merge = humans only on protected branches | parity |

Every wave-4 pilot MUST walk this table against the live group settings and record
the result as Evidence — parity is verified per deployment, not assumed from the
spec.

## 7. Tooling — the forge adapter

statusgen needs nothing (git + files). The desk tools that speak the forge
(deskpost, deskpr, deskreply, deskmerge probes, deskboard's PR/issue reads,
deskverdict, deskfile) currently shell `gh` / GitHub REST. The deliverable is a
**forge interface in deskkit** (home: `assay` repo):

- `Forge` interface: the ~12 operations the tools actually use (create draft
  PR/MR, comment, review/approve, flip draft, read checks, read reactions, file
  issue, close issue, read reviews at head, push transport hints).
- `github` implementation = extraction of what exists; `gitlab` implementation =
  GitLab REST v4 (MRs, notes, approvals, statuses). `glab` CLI is NOT a
  dependency — the tools call the API directly as they do today.
- Concept mapping the implementation must respect: draft PR ↔ `Draft:` MR;
  review-approval ↔ MR approval; required check ↔ pipeline status + (Ultimate)
  external status check; reaction-gate (public-repo +1) ↔ award emoji on the
  issue; `Fixes #N` ↔ `Closes #N` (GitLab auto-close syntax differs — the
  verdict-lane linkage survives).
- Budgets, breakers, bodycheck, and every writeguard-class control are
  forge-agnostic and wrap the interface, not the implementations.

## 8. Website and docs deliverables (all outside this repo)

- **Site** (assay-site): a `gitlab.html` sibling of `apps.html` — the role table
  of §3, the provisioning script, the tier ladder of §4, the ci-config-project
  pattern of §5, and an honest "what is weaker here" section (§6). The existing
  GitHub pages gain one line pointing at the GitLab profile. Same
  self-contained-content bar as every public page.
- **assay repo**: `create-fleet-gitlab.sh`, the deskkit forge interface + gitlab
  implementation, adopter doc (`docs/adopting-assay-gitlab.md`).

## 9. Honest-claims discipline

The governing claim is §6a's: **at-least-parity per control, verified per
deployment** — never "identical guarantees" (the mechanisms differ) and never
"weaker but disclosed" (non-conforming). The conformance statement carries the
residual-mechanics deltas: rotation-window custody rather than fixed-TTL mints
(§6); Ultimate floor for risk-classed repos (§6a); and the one upgrade — CI
definitions living outside the writable repo (§5) — which MAY be claimed as
stronger, because it is.

## 10. Phasing

| Wave | Deliverable | Home |
|---|---|---|
| 0 | Forge interface extraction in deskkit (github impl = current behavior, tests pin it) | assay |
| 1 | gitlab impl of the interface + `desktoken --forge gitlab` (token-path discipline) | assay |
| 2 | `create-fleet-gitlab.sh` + adopter doc + ci-config-project runbook | assay |
| 3 | Site `gitlab.html` + cross-links | assay-site |
| 4 | Live pilot: one real GitLab group (dogfood or a design partner), fleet booted, one brief driven todo→done end-to-end; Evidence = the pilot's MR/approval/board artifacts | assay + pilot group |
| 5 | Ultimate refinements: custom reviewer role, external status checks as the verdict lane | assay |

Wave 0 is pure refactor with zero behavior change and pays for itself even if
GitLab is never shipped (it is also the Bitbucket/Forgejo door). Wave 4 is the
conformance gate: nothing on the site claims GitLab support before a pilot brief
has actually round-tripped.

## 11. Open questions (for the review)

1. **Tier floor messaging.** Premium as stated floor (service accounts) — or also
   court Free-tier with the §4.4 disclosed-limits mode on the site? Marketing
   honesty vs reach.
2. **gitlab.com vs self-managed.** Same profile, but instance-level knobs (token
   lifetime policy, instance service accounts) differ — one page with callouts, or
   two profiles?
3. **Promote identity on GitLab.** §5 makes it unnecessary; keep an optional
   `-promote` service account purely for cross-forge symmetry of the docs, or
   present the ci-config project as the recommended shape and let the GitHub
   profile inherit the idea later?
4. **Verdict lane.** RESOLVED by the §6a parity ruling: Ultimate is the floor for
   public/risk-classed work (external status checks + custom roles); Premium
   suffices only for private, non-risk-classed groups where the approval+note
   fallback passes the parity table.
5. **Adapter scope creep.** The Forge interface invites "support everything"
   pressure; propose freezing it at the ~12 ops the tools use today and rejecting
   additions without a consuming tool.
6. **Naming.** `spec/forge-gitlab-v1.md` implies a `forge-*` family; confirm that
   namespace before a second profile exists.
