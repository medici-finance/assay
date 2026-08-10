---
stream: desk-apps
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# Desk-Apps Stream

Per-role **GitHub Apps as distinct enforcement actors** — one App identity per desk role
(reviewer, verifier, worker, desk, issue-loop, intake-loop), each with its own key and asymmetric
permissions.

**The stream's goal state** — author ≠ approver enforced by mechanism rather than discipline,
Evidence authorship machine-checkable, `main`-push authority a server-side ruleset rather than a
client-side hook. **None of the enforcement half has arrived.** What the provisioned Apps deliver
today is attribution + audit; author ≠ approver is discipline, and the main-push ruleset (brief 08)
is `todo` **and plan-blocked** — branch protection and rulesets both 403 on a free-plan private
repo. Read enforcement-model.md before treating any sentence in
this stream as a description of a live control.

**Origin:** INTAKE I-38 (2026-07-11, human:<name>). **Precedent:** the reviewer App
(now `assay-reviewer-app[bot]`, App ID 4331225 — created fresh 2026-07-18 to replace the retired
legacy `reviewer-app` App <app-id>, issue #404; methodology/brief-17 — verified, in daily
use) is the pattern every brief here extends. **Sibling:** desk-tools (the zero-prompt Go plumbing). Desk-apps
is the *identity* layer — the Go tools it spawns (`desktoken`, `deskevidence`) live in `../assay-toolkit/tools/desk/`
and inherit `deskkit` (desk-tools/01), but the brief home is here because the work also spans brand
assets, GitHub config, and a statusgen change that desk-tools' scoping explicitly excludes.

## Naming

Per human:<name> (2026-07-12), the desk-App family is `assay-<role>-app[bot]` (Assay product, not Medici).
The five new bots are created as `assay-{verifier,worker,desk,issue-loop,intake-loop}-app[bot]`.
The **reviewer** App predated the three-brand split; on 2026-07-18 it was created **fresh** as
`assay-reviewer-app[bot]` (App ID 4331225) — NOT renamed in place — and the legacy
`reviewer-app` (App ID <app-id>) was retired after the first assay-reviewer review (issue
[#404](https://github.com/example-org/oit/issues/404), now implemented). All
six desk-App identities now follow the `assay-<role>-app[bot]` scheme; the canonical App/install-ID
record is the [Provisioned Apps](#provisioned-apps-2026-07-18) table below.

## The asymmetric permission model (the core design)

Role assignment provides **attribution** (which bot name appears on the artifact) plus an audit trail — not cryptographic authorization. Every agent session runs as the same OS user; all six PEMs live in one directory (`~/.config/adopter/`) owned by that user. Mode 0600 keeps keys from *other* users; it does nothing between *sessions of the same user*. A session that holds a role's key can mint as that role regardless of which loop it is meant to be running — `desktoken` takes the role as a positional argv string and performs no caller-identity check. Possession of a readable key file is the entire gate.

What this genuinely delivers, and it is exactly two things: tamper-evident attribution (the correct bot name appears on GitHub, and nobody without the key can fake it) and audit (one line per mint in the audit log). What it does not deliver, and cannot at this layer: authorization (which session was permitted to act as that role) — nor any `main`-push distinction between App identities, which would require the brief-08 ruleset that does not exist.

**Ruled 2026-08-02 (human:<name>, [#44](https://github.com/medici-finance/assay-toolkit/issues/44)): the boundary comes from sandboxed execution** — *"our plan for this is to use desk-console or docker/pods to run. these sandboxes will stop OS/access."* Not one OS user per role; the earlier research framing (and its shared-memory sub-problem) is superseded. The mechanism is `desk-console/04` — each role's App key as an RBAC-scoped Secret mounted only into that role's pods. See enforcement-model.md.

Final permissions as provisioned 2026-07-18 (values in the [Provisioned Apps](#provisioned-apps-2026-07-18)
table). The reviewer now carries `issues` **and** `contents: write` (previously PR-only) — a conscious
override of the original "no reviewer `contents`" design (see [Decisions](#decisions-2026-07-18-ian)).

The last column is **who is sanctioned to land `main`**, not who is prevented from it. Every App
below holds `contents: write` and could push `main` today; the distinction is convention plus the
client-side `ASSAY_MAIN_COMMIT_OK` hook, pending brief 08.

| App | Permissions | Role | Sanctioned `main`-lander? |
|---|---|---|---|
| `assay-reviewer-app[bot]` | `pull_requests`, `issues`, `contents`: write; `metadata`: read | reviews PRs; flips its own draft PRs; files governance issues | no (brief-08 ruleset will enforce) |
| `assay-worker-app[bot]` | `pull_requests`, `issues`, `contents`: write; `metadata`: read | authors PRs/commits; files discovery bugs | no |
| `assay-verifier-app[bot]` | `contents`, `issues`: write; `pull_requests`, `metadata`: read | Evidence authorship under a distinct bot identity — GitHub-side commit-actor linkage is **not** achieved today (`.author.login` is `null`, [oit#1356](https://github.com/example-org/oit/issues/1356); brief 06 Verify row 3 FAIL); files discovery bugs | **yes** |
| `assay-desk-app[bot]` | `pull_requests`, `issues`, `contents`: write; `metadata`: read | coordinator's main-landing identity (status-regen, brief-row updates, methodology calls); files governance issues | **yes** |
| `assay-issue-loop-app[bot]` | `pull_requests`, `issues`, `contents`: write; `metadata`: read | INBOUND **issues** lane; opens close-PRs | no |
| `assay-intake-loop-app[bot]` | `pull_requests`, `issues`, `contents`: write; `metadata`: read | INBOUND **intake** lane (idea-shaped → INTAKE entries); opens entry PRs | no |

Each App is **public**, installed on **both** accounts (`the-org` + the `medici-finance` org),
matching the reviewer App's dual-install. `{verifier-app, desk-app, human:<name>}` are the sanctioned
`main`-landers; brief 08's server-side ruleset is the intended replacement for the client-side
`ASSAY_MAIN_COMMIT_OK` hook (F-13) and has not landed — see the goal-state note at the top.

## Provisioned Apps (2026-07-18)

Canonical durable record of the six-App **assay** family as created on 2026-07-18 (issue #404).
App IDs and install IDs live **here and in `~/.config/adopter/apps.env`** — never baked into source
(tools resolve `<ROLE>_APP_ID` via env → `apps.env`, `deskkit.AppID`). Key custody stays under the
house namespace `~/.config/adopter/<role>-app.pem` (the `medici` here is the house directory,
deliberately NOT rebranded — do not "fix" it).

| App (bot login) | App ID | the-org install | medici-finance install | Key path | Repo permissions |
|---|---|---|---|---|---|
| `assay-reviewer-app[bot]` | 4331225 | 147391347 | 147391333 | `~/.config/adopter/reviewer-app.pem` | `pull_requests`, `issues`, `contents`: write; `checks`, `statuses`, `metadata`: read |
| `assay-worker-app[bot]` | 4331284 | 147393415 | 147393366 | `~/.config/adopter/worker-app.pem` | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
| `assay-verifier-app[bot]` | 4331323 | 147393958 | 147393973 | `~/.config/adopter/verifier-app.pem` | `contents`, `issues`: write; `pull_requests`, `metadata`: read |
| `assay-desk-app[bot]` | 4331346 | 147394557 | 147394574 | `~/.config/adopter/desk-app.pem` | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
| `assay-issue-loop-app[bot]` | 4331385 | 147395450 | 147395610 | `~/.config/adopter/issue-loop-app.pem` | `pull_requests`, `issues`, `contents`: write; `metadata`: read |
| `assay-intake-loop-app[bot]` | 4331405 | 147396055 | 147396073 | `~/.config/adopter/intake-loop-app.pem` | `pull_requests`, `issues`, `contents`: write; `metadata`: read |

**Permissions re-measured 2026-08-02** against `GET /orgs/medici-finance/installations` — this table
was already correct and is now also the source the setup guide's tables were corrected from (they
had drifted narrower). **Installation scope is `repository_selection: all` on both accounts** for
every row — the grant is the *account*, not any repo list.

Setup guide (App creation, permissions, dual-install, key custody): `../assay-toolkit/docs/github-apps-setup.md`.

## Decisions (2026-07-18, human:<name>)

- **(a) Reviewer gets `contents: write`** so it can flip its own draft PRs → ready as the App — a
  conscious override of issue #556's "no reviewer `contents`" recommendation. The cost: author ≠
  approver becomes **discipline-enforced, not GitHub-enforced**, for the reviewer. The residual
  guardrail *named at the time* was the brief-08 server-side main-push ruleset barring the reviewer
  App from `main`, holding the blast radius to PR-flip rather than history rewrite. **That ruleset
  has not landed** (`todo`, plan-blocked), so the reviewer App's `contents: write` is currently
  unbounded by mechanism and the guardrail is convention as well.
- **(b) All six Apps get `issues: write`.** Filing convention (assay-toolkit#13): worker + verifier
  file **at discovery** — bugs → the working repo, self-improvement / systemic insights →
  `medici-finance/assay-toolkit`; reviewer, desk, and the two inbound Apps file **governance**
  issues.
- **(c) issue-loop + intake-loop also get `pull_requests: write` + `contents: write`** so they can
  open close-PRs (issue-loop) and intake-entry PRs (intake-loop) — the inbound lanes act via PRs,
  not just issue writes.
- **(d) Reviewer created FRESH, not renamed.** A new `assay-reviewer-app` (4331225) was stood up;
  the legacy `reviewer-app` (<app-id>) is retired after the first assay-reviewer review. (A
  rename in place would have preserved the App ID; a fresh App changes it, hence the config-not-source
  App-ID resolution.)
- **(e) Key path stays `~/.config/adopter/`** — the house namespace directory. Deliberately NOT
  rebranded to `assay`; recorded here so nobody "fixes" it later.

## Loop → App mapping (decks/loops/deck.md)

The methodology runs eight loops; six are agent-actor loops that get an App, the rest are not:

| Loop | App | Why |
|---|---|---|
| REVIEW | `reviewer-app` | posts the verdict — a distinct actor so author ≠ approver |
| VERIFY | `verifier-app` | commits Evidence under a distinct bot identity — GitHub-side commit linkage pending [oit#1356](https://github.com/example-org/oit/issues/1356) |
| DISPATCH (workers) | `worker-app` | authors PRs/commits |
| COORDINATE | `desk-app` | the coordinator's main-landing identity |
| INBOUND — issues lane | `issue-loop-app` | files/routes work-shaped items; opens close-PRs (ACTIVE — provisioned 2026-07-18) |
| INBOUND — intake lane | `intake-loop-app` | files/routes idea-shaped items → INTAKE entries; opens entry PRs (ACTIVE — provisioned 2026-07-18) |
| METRICS | — (no App) | zero-AI deterministic generators — code, not an agent |
| ANALYSIS | — (future seat) | judgment → findings/reports; add an App only if analysts commit as a distinct actor |
| RETRO | — (no App) | human-driven; one process change per cadence |

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [desk-app brand icons — octagonal assay stamps (reviewer/verifier/worker/desk/issue-loop/intake-loop) + canonical assay-mark](./brief-01-brand-icons.md) | 0 | S | done | 2026-07-16 glm-5.2-verifier | 2026-07-14 reviewer-app[bot] |
| 02 | [GitHub App setup guide — access matrix, public Apps, dual-install, key custody, token cache](./brief-02-setup-guide.md) | 0 | S | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 03 | [desktoken — key-parameterized token minter (generalizes mint-reviewer-token.go)](./brief-03-desktoken.md) | 1 | M | done | 2026-07-16 glm-5.2-verifier | 2026-07-20 human:ian |
| 04 | [deskevidence — verify-desk commits Evidence as verifier-app[bot]](./brief-04-deskevidence.md) | 2 | M | implemented | — | — |
| 05 | [worker App + deskpr/deskreply App-identity cutover](./brief-05-worker-app-cutover.md) | 2 | M | verified | 2026-07-30 glm-5.2-verifier | — |
| 06 | [desk App cutover — coordinator main-landing identity (replaces example-org/client-hook)](./brief-06-desk-app-cutover.md) | 2 | M | implemented | — | — |
| 07 | [statusgen Evidence-actor check (tamper-evident verified rows, closes I-08)](./brief-07-statusgen-actor-check.md) | 3 | M | todo | — | — |
| 08 | [F-13 server-side main-push ruleset ({verifier-app, desk-app, human:<name>})](./brief-08-main-push-ruleset.md) | 3 | S | todo | — | — |
| 09 | [INBOUND Apps — issue-loop + intake-loop (the two inbound lanes)](./brief-09-inbound-apps.md) | 3 | S | done | 2026-07-19 glm-5.2-verifier | 2026-07-18 human:ian; accepted 2026-07-20 human:ian |

## Dependency waves

```
Wave 0: [01, 02]            ← leaf deliverables (model-gate, do first)
Wave 1: [03]                ← desk-tools/01 (deskkit)
Wave 2: [04, 05, 06]        ← 03 + [human: human:<name> creates the Apps via guide 02]
Wave 3: [07, 08, 09]        ← 07←04; 08←04+06; 09 deferred
```

**Critical path:** `01 → 02 →[human: Apps created]→ 04 → 07`.

**Real head of the integrity chain (not what it looks like):** the tempting first step past the
leaf briefs is `desktoken` (03) — but 03 is *inert* until the Apps exist on GitHub. The real head
of the chain is **the human act of human:<name> creating the verifier + desk + worker Apps** (documented in
guide 02). No amount of minter/Evidence/ruleset code does anything until those identities + keys
exist. Author 03 in parallel, but do not mistake it for the unblock — the same shape of failure as
the Phase 2.5 "Brief 8 → Brief 1" mis-head (the real head was BUGS #17, not in the path).

## Shared conventions (inherited by every brief)

- **Code home:** `../assay-toolkit/tools/desk/` + `../assay-toolkit/tools/desk/internal/deskkit/` (shared with desk-tools; desk-apps
  tools inherit deskkit's config/audit/kill-switch/rate-limit/version + the C-1…C-10 constraints
  from `docs/streams/desk-tools/scoping.md`). Install via `sudo make desk-install` (C-1: the sudo
  password IS the manual permission gate).
- **The reviewer-App precedent (methodology/brief-17):** the reviewer App is `assay-reviewer-app[bot]`
  (App ID 4331225), **public**, installed on `the-org` (install 147391347) + `medici-finance` org
  (install 147391333) — see [Provisioned Apps](#provisioned-apps-2026-07-18). (It replaced the retired
  legacy "Medici stuff" App <app-id> / `reviewer-app[bot]`, installs 145487688 / 145764869.)
  Token = RS256 JWT with the App private key → installation access token (~60 min TTL), per-install
  cache (<50 min reuse). Every new App replicates this; `desktoken` (03) generalizes
  `mint-reviewer-token.go` to be key-parameterized.
- **Cross-repo deliverables:** brand assets (01) + setup guide (02) land in `assay-toolkit` (the
  Assay brand home per INTAKE I-brand-system); briefs live here, deliverables there, SHA recorded
  in Evidence. A sibling draft PR opens in assay-toolkit, cross-referenced from this repo's PR
  (memory `cross-repo-brief-needs-sibling-pr`).

## Out of scope

- Per-session App-token enforcement (the harness has no per-session permissions — same residual as
  desk-tools TM-1).
- ~~Renaming the existing reviewer App's bot login~~ — **done (2026-07-18, issue #404):** the
  reviewer was created FRESH as `assay-reviewer-app[bot]` (App 4331225) and the legacy App <app-id>
  retired; see [Decisions](#decisions-2026-07-18-ian) (d).
- Merge/close/deploy authority — always human:<name>'s, never an App's.
