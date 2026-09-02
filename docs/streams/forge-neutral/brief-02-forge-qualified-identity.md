---
brief: forge-neutral/02
title: Forge-qualified identity — roster entries, bot renderings, review corroboration
why: >-
  On the 2026-09-02 GitLab pilot the Evidence was committed by a distinct, non-implementing
  verifier account and the lint still reported "0 row(s) are backed" — a correctly verified row
  was indistinguishable from a self-attested one. The trust roster can only spell a GitHub App:
  a slug, a numeric App/user id, and the two renderings `<slug>[bot]` and `app/<slug>`. Until an
  entry can say WHICH FORGE an identity belongs to, every check that asks "who acted?" answers
  could-not-check on a non-GitHub deployment — and a check that cannot recognise the acting
  identity fails open rather than loudly.
wave: 2
depends: ["forge-neutral/01"]
unblocks: ["forge-neutral/07", "forge-neutral/08", "forge-neutral/09", "forge-neutral/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-gitlab/pilot-report.md D-3 — a correctly-verified GitLab row reads as self-attested; D-9 — the witness names a host-derived handle, not the acting account"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver whose per-repo forge this roster format must agree with"
  - "docs/streams/forge-gitlab/spec.md §2 (identity model)"
  - "freshness-checked 2026-09-02 @ deae247 — rosterconfig.go:783-800 parses `[role=]slug[:id]` and hardcodes the two GitHub login renderings; preflight.go:730,752-754 requires the GitHub noreply commit address; no roster entry can carry a forge"
exec-tier: strong
exec-tier-why: "one shared value (the roster entry format) is read by the desk verbs, the preflight, the Evidence-actor lint and the auto-flip corroboration in two separate binaries; a format that is right at one site and wrong at another fails the whole trust gate open (questions b and c)."
gate-why: >-
  This brief changes what the fleet accepts as a trusted identity — the roster is the single
  source for "may this actor's write be believed?". A format that is too permissive (a
  forge-unqualified entry matching a same-named account on another forge) or an equality rule
  that is too loose widens the trust gate silently. The human is confirming the entry grammar,
  the backward-compatibility rule for existing unqualified entries, and that an entry whose
  forge does not match the repo's resolved forge is refused rather than ignored.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/rosterconfig.go: fixed-here"
  - "tools/desk/internal/deskkit/preflight.go: fixed-here (commit-identity check per forge)"
  - "tools/desk/cmd/deskwt/roleinit.go: fixed-here (bot commit identity is built from the roster entry)"
  - "statusgen/rosterconfig.go, statusgen/evidenceactor.go: follow-up forge-neutral/07 (statusgen parses the same roster and must accept the same grammar)"
  - "statusgen/autoflip.go: follow-up forge-neutral/08"
  - "plugins/assay/skills/install/SKILL.md, plugins/assay/skills/adopt/SKILL.md: follow-up forge-neutral/11 (the two-principals prerequisite is stated from this grammar)"
  - "tools/cellctl/cellctl: follow-up forge-neutral/09"
---

# Brief 02 — Forge-qualified identity

## Context
files:
- `tools/desk/internal/deskkit/rosterconfig.go` — the entry parser and the accepted-login set.
- `tools/desk/internal/deskkit/preflight.go` — the commit-identity check.
- `tools/desk/cmd/deskwt/roleinit.go` — builds the bot commit identity from the roster entry.
- `docs/streams/forge-neutral/identity.md` (planned) — the grammar, the per-forge rendering
  table, and the corroboration rule, written once so brief 07, 08, 09 and 11 cite rather than
  restate it.

single-point-of-failure: the roster is the one source of "which identities may be believed",
so a permissive grammar is a single control failing open. Two independent layers back it: the
parser refuses a malformed or forge-mismatched entry at load (fail-closed, in `deskkit`), and
the commit-identity preflight independently re-derives the expected commit address from the
entry and compares it against what the forge actually reports (a different check, in a
different component, tripping on a different signal — a wrong entry that parses still fails
the preflight).

facts:
- Today's grammar is `[role=]slug[:id]` with a positive numeric id
  (`tools/desk/internal/deskkit/rosterconfig.go:783-793`); the parser then registers exactly
  two accepted logins per entry, `<slug>[bot]` (REST) and `app/<slug>` (the `gh` JSON
  rendering) — `rosterconfig.go:796-800`, with the comment *"BOTH GitHub renderings are
  accepted; the bare slug never is."*
- The commit-identity check requires `<bot-user-id>+<slug>[bot]@users.noreply.github.com`
  (`tools/desk/internal/deskkit/preflight.go:730,752-754`) and compares the login as
  `<slug>[bot]` (`preflight.go:781`).
- `deskwt` builds the same address for role worktrees and requires the roster entry
  `role=<app-slug>:<bot-user-id>` (`tools/desk/cmd/deskwt/roleinit.go:44,143-144`).
- A GitLab service account, as measured on the pilot: a numeric user id, `"bot": true` on
  `GET /user`, and a commit address of the form
  `service_account_group_<group-id>_<n>@noreply.gitlab.com`
  (`docs/streams/forge-gitlab/pilot-report.md` §0 and §3 row 13).
- Review corroboration differs per forge: on GitHub a review carries a state and a
  `commit_id`; on GitLab CE approvals do not reset on push (`reset_approvals_on_push` is
  Premium and read `false` on the pilot), so the head pin lives in the note body — the CE
  posture recorded in `../forge-gitlab/edition-matrix.md` row A7 and walked in
  `docs/streams/forge-gitlab/pilot-report.md` steps A9 and B4.
- The roster fails closed on an unrecognised key (`rosterconfig.go:728,762`) and on a schema
  version it does not understand (`rosterconfig.go:775-780`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Existing unqualified entries must keep working on a GitHub repo. Breaking every deployed
  roster is not an acceptable way to add a field.

## Task
1. **Grammar.** Extend the entry to `[role=]<forge>:<slug-or-login>[:<numeric id>]`, e.g.
   `reviewer=gitlab:assay-reviewer-bot:41987965` and
   `reviewer=github:assay-reviewer-app:300000004`. An entry with no `<forge>` segment is
   read as `github` — the backward-compatibility rule — and that default is recorded in the
   parse result so a caller can tell an explicit `github` from an inferred one.
2. **Per-forge renderings.** Replace the hardcoded pair with a per-forge rendering set:
   GitHub keeps `<slug>[bot]` and `app/<slug>`; GitLab registers the account's username and
   its numeric id. The bare slug remains unaccepted on every forge. Record the table in
   `identity.md`.
3. **Per-forge commit address.** `preflight.go` and `tools/desk/cmd/deskwt/roleinit.go` derive the expected
   commit address from the entry's forge: the GitHub noreply form as today, the GitLab
   service-account noreply form for a `gitlab:` entry. Neither may fall back to the other
   forge's shape.
4. **Forge agreement is enforced, not assumed.** An entry whose forge does not match the
   forge `ForgeFor` resolves for the repo being acted on is a `Refused` at load naming both
   values. Silently ignoring a mismatched entry is how a same-named account on the wrong
   forge becomes trusted.
5. **Corroboration rule.** Specify, in `identity.md`, what counts as a review verdict at head
   per forge: on GitHub, a review whose `commit_id` equals the head; on GitLab, an approval by
   an accepted reviewer identity PLUS a note by that identity pinning the head SHA — the CE
   posture the pilot walked. State plainly that the GitLab form is weaker where approvals do
   not reset on push, and that the head pin is what carries the at-head property there.
   No code in this brief consumes the rule; 07 and 08 do.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeQualifiedEntry -count=1 -v` | exit 0; parses `reviewer=gitlab:assay-reviewer-bot:41987965` and `reviewer=github:assay-reviewer-app:300000004`, and records the forge on each |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterUnqualifiedEntryDefaultsToGitHub -count=1 -v` | exit 0; a legacy `reviewer=assay-reviewer-app:300000004` still parses, resolves to `github`, and is flagged as INFERRED rather than explicit |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeMismatchRefuses -count=1 -v` | **negative path**: a `gitlab:` entry against a repo whose resolved forge is `github` yields a refusal naming both forges; the entry is NOT silently dropped and NOT accepted |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterBareSlugStillRejected -count=1 -v` | **negative path**: a bare slug with no rendering qualifier is rejected on BOTH forges — the pre-existing property must survive the new grammar |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityPerForge -count=1 -v` | exit 0; a `github:` entry expects `<id>+<slug>[bot]@users.noreply.github.com`, a `gitlab:` entry expects the service-account noreply form, and neither accepts the other's |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityCrossForgeRejected -count=1 -v` | **negative path**: a commit authored with the GitHub noreply address fails the preflight for a `gitlab:` entry, and vice versa |
| 8 | `cd tools/desk && go test ./cmd/deskwt/... ./cmd/deskflip/... ./cmd/deskboard/... ./cmd/deskclose/... -count=1` | exit 0 — every suite carrying a roster fixture stays green with the legacy unqualified fixtures unmodified |
| 9 | `grep -c '^[\|] ' docs/streams/forge-neutral/identity.md` | ≥ 2 — the per-forge rendering table and the corroboration table are present as tables, not prose |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1` | exit 0 — no new unregistered `ASSAY_*` key; the grammar change rides the existing key so no deployment fails closed on an unknown name |
| 11 | `statusgen --root . --consumers --brief forge-neutral/02` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The new grammar breaks every deployed roster and the whole fleet fails closed | rows 3 + 8 (legacy fixtures unmodified across four verb suites) |
| A `gitlab:` entry is accepted for a GitHub repo, so a same-named account on the wrong forge becomes trusted | row 4 |
| The bare-slug rejection is lost while rewriting the rendering set | row 5 |
| The commit-identity check keeps the GitHub address shape and simply skips it for GitLab entries — a check that no longer fires | rows 6 + 7 (7 asserts the cross-forge case FAILS, so a skipped check cannot pass it) |
| A new env key is introduced and an existing deployment that sets it fails closed | row 10 |
| The corroboration rule is written but the GitLab weakening is glossed as parity | **no row** — review-only. The Review gate reads `identity.md` §corroboration against the pilot's D-3 and the CE posture in the edition matrix; honesty about a weaker mechanism is a judgement, not a check |
| `identity.md` ships as prose so 07/08/09/11 each re-derive the grammar and drift | row 9 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between an unaccepted identity and a believed write? (The
   roster parser.) Is it acceptable alone? (No — hence the independent commit-identity
   preflight, which re-derives the address and compares it against what the forge reports.)
2. Does any Verify row prove the LOWER layer catches the fault with the UPPER bypassed? (Row
   7: an entry that parses cleanly still fails the preflight when the commit address belongs
   to the other forge.)
