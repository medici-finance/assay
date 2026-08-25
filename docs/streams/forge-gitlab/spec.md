# forge-gitlab — the Assay fleet on GitLab Enterprise (stream spec)

**Status:** design accepted 2026-08-24 (v0.1); this stream implements it.
**Governing requirement:** the GitLab profile MUST be **at least as secure as the
existing GitHub controls**, even where the mechanism is completely different
(project ruling, 2026-08-24). "Weaker but disclosed" is non-conforming.

## 1. Scope

Assay's briefs, registers, lifecycle, board, and statusgen are forge-agnostic
(git + markdown) and unchanged. What is GitHub-shaped today is the identity /
permission / CI layer and the desk tools that speak the forge API. This stream
delivers the GitLab profile: self-managed EE or gitlab.com, **Premium floor,
Ultimate required for public/risk-classed work**. GitLab Free/CE cannot meet the
parity requirement and is declared non-conforming for this profile.

Out of scope: migration tooling between forges; other forges (Bitbucket, Gitea)
— the forge interface (brief 01) is their door, not their delivery.

## 2. Identity model

GitLab has no GitHub-Apps analog (no manifest flow, no per-resource permission
matrix, no JWT-minted installation tokens). The role fleet maps to **service
accounts** — seatless bot users owned by the top-level group, one per role, each
with its own personal access token:

| Assay role | GitLab identity | Mechanism |
|---|---|---|
| reviewer | service account | Developer; MR notes + approvals; Ultimate: custom role without push |
| worker | service account | Developer; branches + `Draft:` MRs |
| verifier | service account | Developer (commits Evidence); excluded from approval eligibility by approval rules |
| desk | service account | Developer; coordination via MRs |
| issue-loop / intake-loop | service accounts | Reporter |
| board-writer | service account | Developer + allowed-to-push entry on protected `main` (the ruleset-bypass analog) |
| promote | usually **no identity at all** — see §4 |

Attribution separation holds exactly as on GitHub: notes/approvals/commits carry
the service-account identity, which the author's token cannot produce — and
carries the same honest limit (separation of attribution, not proof of
diligence).

## 3. Security parity — the per-control table

Parity is assessed control-by-control and **verified per deployment** (the pilot
walks this table against live group settings and records it as Evidence):

| GitHub control | GitLab mechanism | Verdict |
|---|---|---|
| Per-resource App permissions | Premium: role + token-scope narrowing + protected branches; Ultimate: custom roles | parity at Ultimate; Premium parity on protected lines only → risk-classed work REQUIRES Ultimate |
| Ruleset bypass = single board-writer | Protected-branch allowed-to-push list = exactly board-writer | parity |
| No self-approval | Approval rules: prevent-author + prevent-committers | parity |
| Required CI checks before merge | "Pipelines must succeed" + required approvals; Ultimate: external status checks | parity |
| `workflows` permission guarding CI | Locked **ci-config project** + external CI config path; Ultimate: pipeline execution policy pins it | **stronger** — CI definition lives outside the writable repo |
| Human-gated workflow promotion | Human-merged MR into the ci-config project | parity, fewer moving parts |
| Short-lived minted tokens | **Rotate-on-mint** + short expiry policy (§5) | parity; single-valid-credential property is stronger, TTL shape differs |
| Secret push protection | Push rules secret checks (Premium) + Secret Detection (Ultimate); the house leak sweep runs in CI regardless | parity |
| Immutable release integrity | Protected tags + audit events + sha256 pins in `.assay-versions` | parity via the pin discipline |
| Merge is always the human's | Allowed-to-merge = humans only | parity |

## 4. CI isolation

GitLab has no `workflows` permission class — `.gitlab-ci.yml` is an ordinary
file — and does not need one: a project's CI configuration can point at a file
in a **different project** (external CI config path, all tiers), and Ultimate
groups can **enforce** an injected pipeline by policy. The profile therefore
uses one locked ci-config project per group (humans-only Maintainer, protected
main, approval rules on); fleet projects set their CI config path there. Bot
identities are simply never members of the ci-config project. Workflow
promotion collapses to an ordinary human-merged MR — structural isolation
rather than procedural, and the one place this profile is *stronger* than the
GitHub controls it must match.

## 5. Token custody — rotate-on-mint

PATs are long-lived; naive handling would be a custody downgrade, which §3
forbids. The profile closes the gap:

- **Rotate-on-mint**: every token mint calls the rotation API, which returns a
  fresh token and atomically invalidates the old one — at most ONE valid
  credential per role at any moment, and any captured token dies at the next
  mint. Roles are single-window by convention already; parallel actors get
  per-actor service accounts, never shared tokens.
- **Expiry backstop**: group/instance token-lifetime policy set short (7 days
  RECOMMENDED) so an idle fleet leaves no live credential.
- **File custody unchanged**: `0600` token files, path-only printing, never in
  env or argv.
- **Audit events** (Premium+) reviewed for rotation/use anomalies.

## 6. Tooling — the forge interface

The desk tools reach the forge through one seam: a `Forge` interface in deskkit
of roughly twelve operations (create draft change, comment, approve, flip
draft, read checks at head, read reviews at head, reaction/award read, file
issue, close issue, push-transport hints). The `github` implementation is an
extraction of current behavior pinned by goldens; the `gitlab` implementation
is REST v4. Concept mapping: draft PR ↔ `Draft:` MR; review approval ↔ MR
approval; required check ↔ pipeline status (+ external status check at
Ultimate); reaction admission gate ↔ award emoji; `Fixes #N` ↔ `Closes #N`.
Budgets, breakers, and body checks are forge-agnostic and wrap the interface.
The interface is FROZEN at the operations a shipping tool consumes — additions
require a consuming tool in the same change.

## 7. Deliverable homes

Everything in this stream lands in this repository (OSS), except the public
site page describing the GitLab profile, which lands with the site and is
tracked outside this stream. The live pilot (brief 05) is the conformance
gate: no published claim of GitLab support before one brief has round-tripped
todo→done on a real GitLab group with the §3 table walked and recorded.
