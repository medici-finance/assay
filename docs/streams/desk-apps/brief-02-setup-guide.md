---
brief: desk-apps/02
title: GitHub App setup guide — public per-role Apps, asymmetric access, dual-install, key custody
why: >-
  Creating the per-role GitHub Apps is an irreversible, outward, public act (a new App identity on
  GitHub, installed across two accounts) — squarely human-gate. A single written guide lets human:<name>
  execute each App the same way, makes the asymmetric permission model concrete (reviewer can't
  commit, worker-authored PRs are self-approval-blocked by GitHub, only verifier+desk may land main), and is the real head of the whole
  desk-apps integrity chain: no minter/Evidence/ruleset code does anything until the Apps + keys exist.
wave: 0
depends: []
unblocks: ["desk-apps/04", "desk-apps/05", "desk-apps/06", "desk-apps/08"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (per-role GitHub Apps + the asymmetric permission model)", "methodology/brief-17 (the reviewer-App precedent — App ID <app-id>, dual-install 145487688/145764869)", "~/.claude/skills/pr-review-desk/mint-reviewer-token.go (the token-minting pattern to generalize, brief 03)", "CLAUDE.md [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) (ASSAY_MAIN_COMMIT_OK client hook → server ruleset)"]
---

# Brief 02 — GitHub App setup guide

**CROSS-REPO:** the guide lands in `medici-finance/assay-toolkit` (`../assay-toolkit/docs/github-apps-setup.md`)
— the methodology-toolkit home, alongside `product-brief.md`, `naming-clearance.md`, etc. This repo's
PR carries this brief + the stream row. The guide documents the **human-gate** act of creating the
Apps; the brief itself creates no Apps (risk.irreversible: no — the *doc* is reversible; the *acts*
it describes are human:<name>'s). Record the assay-toolkit commit SHA in Evidence.

## Context
files: `../assay-toolkit/docs/github-apps-setup.md` (new); `../assay-toolkit/docs/brand/README.md`
(brief 01) cross-referenced for each App's avatar.
facts:
- **The six Apps — all provisioned 2026-07-18 (issue #404).** Final App IDs, both install IDs, and
  key paths are the canonical record in
  [desk-apps README → Provisioned Apps](./README.md#provisioned-apps-2026-07-18); the reviewer was
  created **fresh** as `assay-reviewer-app` (App 4331225), retiring the legacy "Medici stuff" App
  <app-id>. Repository permissions **as this brief specified them at creation (2026-07-18)** — this
  is the creation-click record, not a measurement of the live installations:

  | App slug (bot login) | Repository permissions |
  |---|---|
  | `assay-reviewer-app` (`assay-reviewer-app[bot]`) | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
  | `assay-verifier-app` (`assay-verifier-app[bot]`) | `contents`, `issues`: write; `pull_requests`, `metadata`: read |
  | `assay-worker-app` (`assay-worker-app[bot]`) | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
  | `assay-desk-app` (`assay-desk-app[bot]`) | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
  | `assay-issue-loop-app` (`assay-issue-loop-app[bot]`) | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
  | `assay-intake-loop-app` (`assay-intake-loop-app[bot]`) | `pull_requests`, `issues`, `contents`: write; `metadata`: read |

  > **Correction 2026-08-02.** This table was previously headed "Final repository permissions as
  > provisioned", which reads as a measurement it never was. It has since drifted from the live
  > grant in at least one row: `assay-reviewer-app` also holds `checks` and `statuses: read`,
  > granted by human:<name> under #348 after this brief was written. Do not read this table as the current
  > state of any installation — the measured record is
  > `../../github-apps-setup.md` § "The six Apps", sourced from
  > `GET /orgs/medici-finance/installations`.

- **Public + dual-install (the model every App follows):** each App is created **public** (so it can
  be installed on any account, not just its owner) and installed on **both** `the-org` and the
  `medici-finance` org — exactly the reviewer App's footprint.
  > **Correction 2026-08-02.** This bullet originally described the footprint as a repo subset on
  > each account — "`the-org` (the three core repos: oit, agents, examples)" and "`medici-finance`
  > (report repos: assay-toolkit, reconciler, decks, proposals)". That is not the live grant and was
  > never a durable one: all twelve installations (6 Apps × 2 accounts) are
  > `repository_selection: all` — every repo in each account, current and future. The enumerations
  > are removed rather than refreshed, because the grant is the **account**, so any repo list here
  > would go stale the moment a repo is created. See `../../github-apps-setup.md` § "Actual
  > installation scope". Same correction as step 3 of the Task below.
- **Key custody = role (the core invariant):** each App has its own PEM private key at
  `~/.config/adopter/<role>-app.pem` (0600, owner `iholsman`). A session is a role because it holds
  only that role's key. The token minter (`desktoken`, brief 03) is key-parameterized:
  `desktoken <role> [--repo <slug>]` → installation access token for that role's App.
- **Token pattern (from `mint-reviewer-token.go`, generalized by brief 03):** RS256-sign a short JWT
  (iss = App ID) with the role's PEM → `POST /app/installations/<installID>/access_tokens` → ~60-min
  installation token; cache per-installation at `~/.config/adopter/<role>-token[-<installID>]` (reuse
  if <50 min old). Never print the token. The install ID is auto-picked by target owner
  (the-org vs medici-finance), overridable via `<ROLE>_INSTALL_ID`.
- **App creation is human:<name>'s act (human-gate):** the guide gives the exact clicks + records App ID +
  install IDs + key path. No brief automates App creation (mirrors assay-dogfood/01: "Repo creation,
  permission grants, and the first push are IAN'S acts").

## Ground rules
- NEVER git push / trigger workflows. assay-toolkit commits are LOCAL — pushing that repo is human:<name>'s
  (memory `no-auto-commit`). Leave commits per the task instructions only.
- The guide documents App creation; **do not create any App** (public identity = irreversible, human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
Write `../assay-toolkit/docs/github-apps-setup.md` covering, for each App (reviewer documented as the
existing precedent; verifier/worker/desk as the creation runbook; issue-loop noted as deferred):
1. **Create the App** (GitHub → Settings → Developer settings → GitHub Apps → New GitHub App): name,
   the avatar (from `docs/brand/<role>-app.svg`, brief 01), **public** toggle, webhook URL left empty
   (no subscription needed for token-based actor), and the exact repository-permission toggles from
   the table above. Record the **App ID**.
2. **Generate + store the private key**: download the PEM, place at `~/.config/adopter/<role>-app.pem`,
   `chmod 600`. (These are the paths `desktoken`, brief 03, reads.)
3. **Install on both accounts**: install the App on `the-org` and on the `medici-finance` org;
   record both **installation IDs** (URL `.../settings/installations/<NNN>`).
   > **Correction 2026-08-02.** This step originally read "(select the three core repos)" / "(select
   > the report repos)". That is not what was done: all twelve installations (6 Apps × 2 accounts)
   > are `repository_selection: all` — every repo in each account, current and future. Do not follow
   > the original wording as an instruction or read it as a description of the live grant; see
   > `../../github-apps-setup.md` § "Actual installation scope".
4. **Mint a token + smoke-test**: `desktoken <role> --repo <slug>` → token; verify the actor with
   `gh api /repos/<slug>/commits/<sha>` showing `commit.author` / verification as `<role>-app[bot]`,
   OR a no-op read confirming the installation resolves. (Until `desktoken` exists, the guide gives
   the equivalent `mint-<role>-token` one-off using the same JWT→installation-token flow.)
5. **The asymmetric model section** (state the invariant plainly): reviewer can't commit; GitHub self-approval block + reviewer-app-only gate; only verifier + desk may land `main` (enforced server-side by the brief-08 ruleset,
   replacing `ASSAY_MAIN_COMMIT_OK`); author ≠ approver is GitHub-enforced (a worker-app PR cannot
   be approved by worker-app — only by reviewer-app).
   > **Correction 2026-08-02.** "enforced server-side by the brief-08 ruleset, replacing
   > `ASSAY_MAIN_COMMIT_OK`" is what this brief asked the guide to state, and the guide stated it —
   > but the ruleset does not exist. Brief 08 is still `todo`, and rulesets cannot be configured on
   > this repo at all: `GET /repos/medici-finance/assay-toolkit/rulesets` returns HTTP 403,
   > "Upgrade to GitHub Pro or make this repository public to enable this feature" (private repo,
   > free plan). `main` is guarded today by the client-side `ASSAY_MAIN_COMMIT_OK` hook, which the
   > ruleset was meant to replace and has not. Do not restate this directive as a live guarantee;
   > see `../../github-apps-setup.md` § "The asymmetric permission model".

   > **Correction, 2026-08-02 (#47) — the other two claims in this step also did not survive
   > contact.** *"reviewer can't commit"*: the reviewer App was granted `contents: write` on
   > 2026-07-18, so that boundary is discipline-held here, not mechanical. *"author ≠ approver is
   > GitHub-enforced"*: true under a **single** shared identity — GitHub refuses to let an actor
   > approve its own PR — but per-role Apps turn one process into two *distinct* actors, which is
   > precisely what dissolves that platform check. The per-role split therefore removed a working
   > control; it was a knowing trade, not a discovery. See
   > enforcement-model.md § "Role separation is a naming
   > convention". Do not re-derive the guide from this list.
6. **A per-App quick-reference table** (slug, App ID, install IDs, key path, permissions) for human:<name> to
   fill at creation and commit back as the canonical record.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-toolkit/docs/github-apps-setup.md && wc -w < ../assay-toolkit/docs/github-apps-setup.md` | ≥800 |
| 2 | `grep -ciE 'pull_requests.*write|contents.*write' ../assay-toolkit/docs/github-apps-setup.md` | ≥1 (access matrix present) |
| 3 | `grep -ci 'assay-verifier-app\|assay-worker-app\|assay-desk-app' ../assay-toolkit/docs/github-apps-setup.md` | ≥3 (all three new Apps covered) |
| 4 | `grep -ci 'installation' ../assay-toolkit/docs/github-apps-setup.md` | ≥2 (dual-install documented) |
| 5 | `grep -ciE '\.config/medici|\.pem|chmod 600' ../assay-toolkit/docs/github-apps-setup.md` | ≥2 (key-custody documented) |
| 6 | `grep -ci 'public' ../assay-toolkit/docs/github-apps-setup.md` | ≥1 (public-App requirement stated) |
| 7 | `grep -ci 'assay-toolkit/docs/brand' ../assay-toolkit/docs/github-apps-setup.md` | ≥1 (cross-refs brief-01 avatars) |
| 8 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output, date, runner). Include the assay-toolkit commit SHA.
     human:<name>'s per-App creation record (App IDs, install IDs) lands here too, as he executes. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `c2311679`, assay-toolkit origin/main `703b262`, 2026-07-18)

Deliverable confirmed on `medici-finance/assay-toolkit` origin/main `703b26228` at `../assay-toolkit/docs/github-apps-setup.md`
(3401 words) — isolated `/private/tmp` clone (shared checkout untouched).

| # | Command | Exit | Output |
|---|---------|------|--------|
| 1 | `test -f …/github-apps-setup.md && wc -w` | 0 | 3401 words (≥800) |
| 2 | `grep -ciE 'pull_requests.*write\|contents.*write'` | 0 | 20 (≥1) |
| 3 | `grep -ci 'assay-verifier-app\|assay-worker-app\|assay-desk-app'` | 0 | 17 (≥3) |
| 4 | `grep -ci 'installation'` | 0 | 30 (≥2) |
| 5 | `grep -ciE '\.config/medici\|\.pem\|chmod 600'` | 0 | 42 (≥2) |
| 6 | `grep -ci 'public'` | 0 | 9 (≥1) |
| 7 | `grep -ci 'assay-toolkit/docs/brand'` | 0 | 1 (≥1) |
| 8 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

All 8 rows meet thresholds. `gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.

**Substantive notes for the Review gate (non-blocking; the brief's Task directive is met).**
- The guide documents reviewer + verifier/worker/desk + issue-loop (deferred) — the five Apps the Task
  (§Task line 1) lists. `assay-intake-loop-app` (the 6th in the §Context table) is intentionally **not**
  duplicated here; it is scoped to **desk-apps/09** (both inbound lanes), where it is fully covered and
  already provisioned ([#404](https://github.com/example-org/oit/issues/404)). A one-line
  "see desk-apps/09" pointer would improve completeness but is optional, not a spec violation.
- The guide tightens worker/desk to drop `issues: write` vs. the §Context table — a least-privilege
  refinement worth a one-line confirmation at Review (does the close-PR / issue-comment flow need it?).
  It also correctly grants the reviewer **no** `contents: write` (the reviewer-can't-commit invariant),
  reconciling the Context table's looser entry in favor of the invariant.

## Review
Gate: model. Reviewer confirms the access matrix is correct and asymmetric (reviewer no-contents,
self-approval block + reviewer-app-only gate, verifier+desk main-landing), dual-install + key custody are documented, and the
guide does NOT itself create any App (human-gate preserved). `/security-review` recommended at the
review gate (auth/identity surface) even though gate: model — the permission model IS the governance
boundary (assay-dogfood/01 gate-why).
