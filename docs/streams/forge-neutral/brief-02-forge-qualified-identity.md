---
brief: assay:assay:forge-neutral:02
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
schema: brief-v2
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
version: 1
id: 9c18ec3c-2dc0-4735-b54e-a9d9ec40fec3
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

### Non-implementer verifier run — VERIFY: PASS (rows 1-10); row 11 BLOCKED (post-merge instrument) — HELD at implemented (gate:human, sensitive-data:yes) — 2026-09-05 opus-4.8[1m]-verifier (verify-desk dispatch), medici-finance/assay merged main 55bb04c

Runner != implementer. Offline envelope (KUBECONFIG=/dev/null). gate: human; risk {regulatory:no, customer:no, irreversible:no, sensitive-data:yes} — human gate.

| # | command | expected | observed (exit + key line) | date · runner |
|---|---------|----------|----------------------------|---------------|
| 1 | cd tools/desk; go build ./... and go test ./... | exit 0 | build 0; suite 0 on clean re-run (ok deskkit 26.5s); one non-reproducing loopengine timing flake on first run, passed 3/3 in isolation — not this brief's deskkit | 2026-09-05 · opus-4.8[1m]-verifier |
| 2 | go test ./internal/deskkit/ -run TestRosterForgeQualifiedEntry -v | exit 0; parses gitlab: and github: entries, records forge | PASS ok 0.24s | 2026-09-05 · opus-4.8[1m]-verifier |
| 3 | ...TestRosterUnqualifiedEntryDefaultsToGitHub -v | exit 0; legacy entry to github, flagged INFERRED | PASS ok 0.23s | 2026-09-05 · opus-4.8[1m]-verifier |
| 4 | ...TestRosterForgeMismatchRefuses -v | negative: gitlab entry on github repo refused naming both | PASS ok 0.24s | 2026-09-05 · opus-4.8[1m]-verifier |
| 5 | ...TestRosterBareSlugStillRejected -v | negative: bare slug rejected on both forges | PASS ok 0.24s | 2026-09-05 · opus-4.8[1m]-verifier |
| 6 | ...TestCommitIdentityPerForge -v | exit 0; github vs gitlab address forms, neither accepts the other | PASS ok 0.25s | 2026-09-05 · opus-4.8[1m]-verifier |
| 7 | ...TestCommitIdentityCrossForgeRejected -v | negative: cross-forge commit address fails preflight | PASS ok 0.25s | 2026-09-05 · opus-4.8[1m]-verifier |
| 8 | go test ./cmd/deskwt ./cmd/deskflip ./cmd/deskboard ./cmd/deskclose | exit 0, legacy fixtures unmodified | all four ok (deskboard 84.3s) | 2026-09-05 · opus-4.8[1m]-verifier |
| 9 | grep -c table rows in docs/streams/forge-neutral/identity.md | >= 2 | 6 (rendering + corroboration tables present) exit 0 | 2026-09-05 · opus-4.8[1m]-verifier |
| 10 | ...TestRosterKnownKeySet | exit 0; no new unregistered key | ok 0.29s | 2026-09-05 · opus-4.8[1m]-verifier |
| 11 | statusgen --root . --consumers --brief forge-neutral/02 | exit 0 | BLOCKED could-not-check — the consumers instrument refuses to judge a merged brief by design and mandates a M^2 checkout + merge-base base; that checkout is writeguard-blocked in this shared-homed session (a STOP), and the brief predates the branch merge-base so it never appears in the branch three-dot diff. Also the register dir docs/streams/requirements/ lacks a README (#471), aborting the global scan. Manual inspection: the three fixed-here consumers are present in the implementing commit; follow-up-routed paths untouched. Route to CI. Not a FAIL | 2026-09-05 · opus-4.8[1m]-verifier |

**VERIFY: PASS (rows 1-10 green, 0 disproved); row 11 BLOCKED (post-merge instrument + writeguard) — HELD at implemented.** gate:human + sensitive-data:yes — a model cannot sign off; the human gate owns the flip.

RISK-VALUE: DERIVED — defaultForge = ForgeGitHub with ForgeInferred:true @ tools/desk/internal/deskkit/forgeidentity.go:72 — an entry with no forge segment resolves to github, flagged inferred; github is the only forge any deployed roster addressed, so this is the exact backward-compat rule, and the inferred flag lets legacy entries be exempted without widening trust to a second forge.
RISK-VALUE: DERIVED — githubCommitAddr = the botUSERid+login noreply form @ forgeidentity.go:134 and gitlabServiceAccountRe anchored regexp @ forgeidentity.go:60 — the GitHub form matches GitHub's bot-USER-id convention (per the null-author rule), the GitLab shape matches the pilot-measured service-account address; the regexp is fully anchored and the two are mutually exclusive (row 7 rejects cross-forge).
RISK-VALUE: DERIVED — githubAcceptedLogins = {slug[bot], app/slug} (bare slug absent) @ forgeidentity.go:104; gitlabAcceptedLogins = {username} @ forgeidentity.go:102 — GitHub keeps the two decorated renderings and excludes the bare slug (spoof risk; row 5 guards); GitLab uses username-only because that is the identity the API attributes to.
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `a91bffd0ea73e49b85569549cb4a4521703e827d` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`, go1.26.5) in an isolated worktree; runner ≠ implementer (impl commit `0d4af087` + preflight follow-up `0276ce0a`). gate: human + `sensitive-data: yes` — the table is RUN for Evidence; no model sign-off; status stays `implemented`.

| # | Command (in `tools/desk`) | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | build 0 | build clean; brief surface (deskkit/preflight/all four cmd suites) green; only `internal/loopengine` FAILed as a non-reproducing timing flake (3/3 clean in isolation), off this brief's surface | PASS |
| 2 | `-run TestRosterForgeQualifiedEntry -v` | 0 | parses `gitlab:…` + `github:…` qualified entries, forge recorded explicit on each | PASS |
| 3 | `-run TestRosterUnqualifiedEntryDefaultsToGitHub -v` | 0 | legacy unqualified entry resolves github, `ForgeInferred=true` (INFERRED, not explicit) | PASS |
| 4 | `-run TestRosterForgeMismatchRefuses -v` | 0 | negative: gitlab entry vs github repo → `ExitRefused` naming BOTH forges; parsed-then-refused, not dropped | PASS |
| 5 | `-run TestRosterBareSlugStillRejected -v` | 0 | negative: bare slug + cross-forge-decorated slug rejected on BOTH forges; genuine renderings accepted | PASS |
| 6 | `-run TestCommitIdentityPerForge -v` | 0 | github entry accepts `<id>+<slug>[bot]@users.noreply.github.com` & rejects gitlab addr; gitlab entry accepts the SA shape & rejects github addr | PASS |
| 7 | `-run TestCommitIdentityCrossForgeRejected -v` | 0 | negative (crux): github noreply addr → CheckedFailed for a gitlab entry; gitlab SA addr → CheckedFailed for a github entry | PASS |
| 8 | `go test ./cmd/deskwt/... ./cmd/deskflip/... ./cmd/deskboard/... ./cmd/deskclose/...` | 0 | all four `ok` — legacy roster fixtures green | PASS |
| 9 | `grep -c '^[|] ' docs/streams/forge-neutral/identity.md` | 0 | `6` (≥2): per-forge rendering table + corroboration table both present as tables | PASS |
| 10 | `-run TestRosterKnownKeySet` | 0 | `ok` — no new unregistered `ASSAY_*` key; grammar rides the existing key | PASS |
| 11 | `statusgen --root . --consumers --brief forge-neutral/02` | 2 | COULD-NOT-CHECK — installed statusgen v1.0.6 cannot parse a `schema: brief-v2` brief (`no brief-v1 file`), plus the offline verifier's shared-home writeguard. Manual corroboration done instead: all three `fixed-here` consumers (`rosterconfig.go`, `preflight.go`, `tools/desk/cmd/deskwt/roleinit.go`) modified in impl commit `0d4af087`; follow-up-routed paths untouched as designed | COULD-NOT-CHECK |

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):** bot ids in tests (300000004, 41987965, 9619193) are TEST FIXTURES only — the real bindings live in the deployed roster, not this source, so not risk-bearing. Top-ranked = the two per-forge commit-address forms (the lower layer behind row 7):
- `RISK-VALUE: DERIVED — per-forge commit-addr forms @ tools/desk/internal/deskkit/forgeidentity.go:60,134.` GitHub form binds authorship to the bot USER id (an App id would land account-unlinked, #638); the GitLab SA form is matched by SHAPE (anchored regexp), not constructed, because group-id + per-account suffix are not derivable from a roster entry — so deskwt refuses to STAMP a gitlab identity (`Derivable:false`) rather than invent one. `Accepts` is exact-match (github) vs anchored-regexp (gitlab) → mutually exclusive → exactly what row 7 proves.
- `RISK-VALUE: DERIVED — defaultForge=github, ForgeInferred:true @ forgeidentity.go:72.` github was the only forge any deployed roster addressed → exact backward-compat rule; the inferred flag exempts legacy entries from forge-agreement without widening trust to a second forge.
- `RISK-VALUE: DERIVED — per-forge login sets @ forgeidentity.go:102,104.` github keeps two decorated forms and excludes the bare slug (username-squatting guard, row 5); gitlab uses username-only; github decorations are never minted for gitlab. No `NAMED, NOT DERIVED` value outstanding.

**Sensitive-data defense (gate: human) — two independent layers:** (1) at LOAD, `assertEntryForgeAgrees`/`AssertRoleForgeMatches` refuse (exit 5, naming both forges) when an explicit-forge entry disagrees with the repo's `ForgeFor`-resolved forge (row 4). (2) INDEPENDENTLY, the commit-identity preflight (`CommitEmailSpec.Accepts`) re-derives the expected address from the entry's forge and rejects a commit authored under the other forge's shape (row 7). Different component (parser vs preflight), different signal (roster forge field vs commit author email) — an entry that parses and passes forge-agreement STILL fails the preflight if its commit address is the wrong forge (lower layer catches with the upper bypassed).

**Scope-traceability:** all rows map to Verify items; both identity.md tables present. Off-surface `loopengine` flake noted (not attributable to this brief). Follow-up commit `0276ce0a` (GitLab session-vs-SA distinction, #643) is independently justified + fully tested (`TestCommitIdentityGitLabSessionEmail`), its own concern — flagged, not a defect. Row 11 is a statusgen v1.0.6 brief-v2 instrument gap, not a brief defect.

**VERDICT: PASS** on rows 1–10; row 11 COULD-NOT-CHECK (statusgen brief-v2 gap + writeguard; manual consumers corroboration passed) — **HELD at `implemented` (human sign-off owed via the verify-gate).** No open NAMED-NOT-DERIVED value for the human.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:07d898b22d5e | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeQualifiedEntry -count=1 -v` | pass exit=0 | sha256:99598c4c7854 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterUnqualifiedEntryDefaultsToGitHub -count=1 -v` | pass exit=0 | sha256:8983c4894597 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeMismatchRefuses -count=1 -v` | pass exit=0 | sha256:fe970de56707 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterBareSlugStillRejected -count=1 -v` | pass exit=0 | sha256:bcf2b36f4e1f | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityPerForge -count=1 -v` | pass exit=0 | sha256:1532a29e90dd | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityCrossForgeRejected -count=1 -v` | pass exit=0 | sha256:c8de1c077a05 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/... ./cmd/deskflip/... ./cmd/deskboard/... ./cmd/deskclose/... -count=1` | pass exit=0 | sha256:dd002fd91c53 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c '^[\|] ' docs/streams/forge-neutral/identity.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1` | pass exit=0 | sha256:a99001777650 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/02` | fail exit=2 | sha256:ba6de000fdeb | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |

Verifier notes (2026-09-27, assay-verifier-app[bot], non-implementer, dispatched verify-desk run on opus-5.5). Witness above run by the pinned statusgen v1.0.27 binary (sha256 matches the darwin-arm64 pin), not a shim, at merged main d034d39fe1c8f0a0a93ba1177f208c48b068a1d0, in a scrubbed environment: empty env, KUBECONFIG=/dev/null, PATH holding only go1.27.1 and statusgen plus system dirs, GOPROXY=off, a scratch HOME carrying only the trust roster, no forge credential. No Verify cell mints or rotates a credential, calls a live forge API, or mutates live state (all rows are go build/test, grep, and an offline git-diff read), so no network sandbox was required. Implementing commit 0d4af0879da01a6c4dc8e0d85f9ca877995bf700, PR #445 (merge b8fd1893e756e7d456f7fa4b33fd226a0a9df7ff); later touches on the same surface: 0276ce0a5 (#643 GitLab session emails) and dac811073 (#1058 per-forge role login).

Per-row key output:
- Row 1 (witness FAIL, exit 1) — environment-shaped, not a defect of this brief. A background re-run of the same full suite in the same scrubbed env failed in 7 packages, none of them this brief's identity code: cellctl (bash-quote test compares against /bin/bash 3.2), commsgw (two tests fail to bind a unix socket: the long scratch TMPDIR exceeds the socket-path limit), commsloop (wedge-deadline test under concurrent load), deskpreflight (read-only test finds the real statusgen on PATH), avatar (byte-exact PNG golden), deskkit (the skill-repo-list positive-control test reads the roster from HOME and trips on its withheld-identifier list — a test-hermeticity gap), loopengine (drain concurrency timing). Targeted package tests of each failing test with an empty HOME, a short TMPDIR and a bash-5 PATH: 6 of 7 ok; the avatar golden test fails under host go1.27.1 and passes under the CI-pinned go1.25.0 toolchain (byte-level PNG encoding drift, not a pixel change). The build step and the whole brief surface (deskkit identity tests, deskwt/deskflip/deskboard/deskclose) were green. Side effect noted: the commsloop tests leave an untracked mailbox directory inside that cmd package in the source tree.
- Rows 2-7 and 10 (pass exit 0) — each named test function exists exactly once (forgeidentity, preflight and forgeresolve test files); a targeted verbose re-run printed `--- PASS` for all seven, so no row is a vacuous "no tests to run".
- Row 8 (pass exit 0) — all four cmd suites ok with legacy unqualified roster fixtures.
- Row 9 (pass exit 0) — direct run prints 9 (Expect is 2 or more): rendering table, corroboration table and the statusgen-consumers table.
- Row 11 (witness FAIL, exit 2) — COULD-NOT-CHECK by design, tracked by open #1281: on a clean merged-main tree the instrument prints "COULD-NOT-CHECK: assay:assay:forge-neutral:02 is not in the diff against d034d39fe1c8…" and asks for the merged branch's own merge-base. Manual corroboration against PR #445's own diff (merge-base 0b5c75f9368f7b0911bddac2feccb55dc3efa098 to branch head 7dbb3ef25d31571e9a8cc6c4e88ca5765b629550): the three fixed-here consumers (deskkit rosterconfig.go and preflight.go, deskwt roleinit.go) are changed; none of the follow-up-routed paths (statusgen, install/adopt skills, cellctl) are touched.

Risk-bearing value (sensitive-data: yes). ENUMERATE over the non-comment code lines PR #445 changed in forgeidentity.go, rosterconfig.go, preflight.go, trust.go, forgeresolve.go and deskwt roleinit.go, plus the Deliverables:
1. `gitlabServiceAccountRe = ^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` @ tools/desk/internal/deskkit/forgeidentity.go:60 — the GitLab commit-identity acceptance shape.
2. `ForgeInferred` exemption `if !ok || ident.ForgeInferred { return nil }` @ forgeidentity.go:262 and :283 — an unqualified entry is never forge-agreement-checked, on any repo forge.
3. `defaultForge: BotIdentity{Forge: ForgeGitHub, ForgeInferred: true}` @ forgeidentity.go:72.
4. GitHub commit address `"%d+%s[bot]@users.noreply.github.com"` @ forgeidentity.go:158.
5. Accepted logins: GitHub `{slug[bot], app/slug}` @ forgeidentity.go:128; GitLab `{username}` @ forgeidentity.go:126.
6. Forge discriminator tokens `github` / `gitlab` in the first colon segment @ forgeidentity.go:75.
7. Refusal exit class `Refused` (exit 5) @ forgeidentity.go:266 — operational.
Test ids (300000004, 41987965, 9619193) are fixtures, not bindings. RANK: 1 and 2 decide which identities are believed (a wrong value silently widens the trust gate; reversible by edit and redeploy, but writes believed meanwhile are not un-believed); 4 and 5 same class, narrower; 3 and 6 grammar; 7 operational, last.

RISK-VALUE: NAMED, NOT DERIVED — gitlabServiceAccountRe = `^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` @ tools/desk/internal/deskkit/forgeidentity.go:60 — the group id and suffix are rightly left open (not derivable from a roster entry, per identity.md), but the host is left open too: any `@noreply.<any-host>` passes for a gitlab entry, although the repo's resolved forge host is known to the resolver. The pilot measured only `@noreply.gitlab.com`, and its suffix is redacted, so the `[0-9a-z]+` class cannot be checked against measured data offline. Missing: a derivation that an unbound host (and an unbound group id) is acceptable for the preflight layer, or a decision to bind them. OPEN QUESTION FOR THE HUMAN.
RISK-VALUE: NAMED, NOT DERIVED — ForgeInferred exemption = `ident.ForgeInferred → return nil` @ tools/desk/internal/deskkit/forgeidentity.go:262,283 — a legacy unqualified (inferred-github) entry is exempt from forge agreement even when the repo resolves to gitlab; the test that pins this (`TestForgeAgreement…InferredGitHub`, forgeidentity_test.go:162) states the carve-out is "the human gate is confirming". The brief's ground rule only requires legacy entries keep working "on a GitHub repo", and the ratified pre-work ruling (#437) says a forge-mismatched entry is refused, never ignored. Supporting but not conclusive: the inferred entry's renderings (`[bot]`, `app/`) are not spellable GitLab usernames. Missing: the human's confirmation that the exemption should extend to non-github repos rather than stop at github ones. OPEN QUESTION FOR THE HUMAN.
RISK-VALUE: DERIVED — defaultForge = ForgeGitHub, ForgeInferred = true @ forgeidentity.go:72 — brief Task 1 states it verbatim, and every roster deployed before this brief addressed GitHub only.
RISK-VALUE: DERIVED — GitHub commit address `<bot-user-id>+<slug>[bot]@users.noreply.github.com` @ forgeidentity.go:158 — the bot USER id form (#638), identical to the address the pre-brief preflight built, now reached through CommitEmailSpec; row 7 proves neither forge accepts the other's address.
RISK-VALUE: DERIVED — accepted logins GitHub {slug[bot], app/slug} @ forgeidentity.go:128, GitLab {username} @ forgeidentity.go:126 — GitHub keeps the two pre-brief renderings and omits the bare slug (row 5); GitLab attributes notes, approvals and commits to the username. Observation: brief Task 2 says GitLab "registers the account's username and its numeric id"; the id is stored on the parsed identity, not added as an accepted login — tighter, not looser.

Observations: #643 (0276ce0a5) adds ASSAY_GITLAB_SESSION_EMAILS, an exact-match allowlist that widens the GitLab commit-identity check; it is registered (row 10 green), unset by default, and never consulted for a GitHub identity. Defense in depth holds as briefed: the load-time forge-agreement refusal (row 4) and the independent commit-address preflight (rows 6-7) are different components tripping on different signals, and row 7 exercises the lower layer on its own — for EXPLICIT entries. For an inferred entry the upper layer is exempt by design (open question 2 above).

VERIFY: BLOCKED — rows 2-10 pass on the witness (9 of 11); row 1 witness FAIL is environment-shaped (targeted re-runs clean, avatar golden clean under the CI-pinned toolchain); row 11 COULD-NOT-CHECK by design (#1281), manual consumers corroboration clean. gate: human, sensitive-data: yes — HELD at implemented; two NAMED, NOT DERIVED values carried to the human above.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:636cf0a440f6 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeQualifiedEntry -count=1 -v` | pass exit=0 | sha256:9ef38c15a28d | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterUnqualifiedEntryDefaultsToGitHub -count=1 -v` | pass exit=0 | sha256:2bccba376c1c | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterForgeMismatchRefuses -count=1 -v` | pass exit=0 | sha256:5014b5b3761d | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterBareSlugStillRejected -count=1 -v` | pass exit=0 | sha256:b87c51a18e05 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityPerForge -count=1 -v` | pass exit=0 | sha256:f95c5e6bcc8a | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run TestCommitIdentityCrossForgeRejected -count=1 -v` | pass exit=0 | sha256:fc66e3be3806 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/... ./cmd/deskflip/... ./cmd/deskboard/... ./cmd/deskclose/... -count=1` | pass exit=0 | sha256:fd5425235847 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c '^[\|] ' docs/streams/forge-neutral/identity.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -run TestRosterKnownKeySet -count=1` | pass exit=0 | sha256:692d4dc15ba7 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/02` | pass exit=0 | sha256:cfff82922901 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |

**Re-witness notes (2026-09-28, clean tree).**
- Re-run at the batch tree (`e1afb99aca99`, porcelain empty before the run, no `+dirty`) because main changed a declared input after batch F; this table supersedes the batch-F witness above for these rows.
- Row 1 (whole-module `go test ./...`) is HELD, attributed to the run environment (from a same-tree reproduction of the row), not to this brief: `cmd/commsgw` unix-socket bind is denied by the network-off sandbox; `cmd/cellctl` bash-quoting test sees macOS bash 3.2; `internal/loopengine` and `cmd/commsloop` hit 5s deadlines under host load 21-38 and pass outside the sandbox; `internal/avatar` golden images differ on darwin/arm64 (go1.27.1 local), NOT confirmed against CI. None of these packages is a deliverable of forge-neutral/02.
- Rows 2-11 PASS (10 of 11), including row 11 `statusgen --consumers`, which exits 0 on this run.
- Test hygiene: a `cmd/commsloop` test leaves an untracked `mailbox/` directory in the source tree; it reappeared mid-run (stamp is taken before any row runs, so it stays clean) and was removed afterwards.

**VERIFY: BLOCKED** — rows 2-11 pass (10 of 11); row 1 held, environment-attributed above. gate: human, sensitive-data: yes — a model records Evidence and does not sign off; status stays `implemented`.

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
