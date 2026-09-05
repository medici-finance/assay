---
stream: apps-installer
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# apps-installer Stream — one guided sitting instead of eight App runbooks

The single largest adoption hurdle Assay has is the GitHub App setup: an adopter following
[`docs/adopting-assay.md`](../../adopting-assay.md) today creates one App per desk role by hand —
name, permission toggles, key generation, download, rename, record the App ID, install on the org,
record the installation ID — and repeats it six times, against a GitHub account-level creation
throttle that pauses the account after roughly four Apps. Nothing in that runbook is work only a
human can do except three clicks GitHub reserves for a signed-in person: **Create**, **Install**,
and the avatar upload.

This stream ships `deskapps`, a desk-tools verb that serves a one-page web app on the loopback
interface, drives GitHub's App Manifest flow, and reduces the whole runbook to those three clicks
per App. It is resumable across the throttle, it proves the result with the same
`app-scopes-vs-duties` check the desk preflight already runs, and it offers the adopter a
**tier** rather than a count of Apps. The design is in [`design.md`](./design.md) — read it before
any brief here.

## The three tiers, and what each one changes

| Tier | Bot identities | What runs without the adopter | What stays the adopter's |
|---|---|---|---|
| **Solo** | 0 — every loop acts as the operator's own login | board and status regen, the desk skills drafting reviews, evidence and briefs, the inbox query | starting every loop by hand; posting every verdict as themselves (GitHub refuses a login approving its own PR, so reviews land as comments); merging; every decision |
| **Read + Act** | 2 — `<prefix>-read` polls and indexes, `<prefix>-act` writes | review verdicts on every new PR head, workers opening draft PRs, issues filed at discovery, merged briefs verified and evidenced, approved-green PRs flipped ready | approving and merging (the act App authors PRs, so its verdicts on those are comments); answering needs-decision; installing the two Apps on each new repo or org |
| **Full suite** | 6 — one App per desk role | everything above, plus a reviewer that can formally approve a worker's PR, a verifier a ruleset can single out, a desk that dispatches as its own actor, inbound lanes that triage themselves | merging and every ruling; holding six keys apart; two install sittings (the throttle); re-consent on every permission change |

The **Read + Act** tier is the recommended default and is what `deskapps` provisions unless told
otherwise. Solo is the pilot ramp; Full suite is for adopters whose key custody is real (one key
per Secret or per user). The tiers change **who does the work**, never the method.

## Why the tool, and not just a better runbook

- **The manifest flow is the only programmatic surface GitHub offers for App creation.** There is
  no REST call that creates an App or installs one; both need a signed-in click. The manifest flow
  turns each into exactly one click and hands the App ID and private key back to a program through a
  one-hour code exchange (`POST /app-manifests/{code}/conversions`).
- **The conversion returns the private key, so whoever calls it holds the key.** The call is made
  by `deskapps` on the adopter's machine. A hosted page performing the exchange would hold every
  adopter's key; that is the hosted-pilot model, and it is out of scope here by design.
- **The throttle is real and undocumented.** A tool that is not resumable turns it into a restart;
  one that is turns it into a wait. Every App carries its own state record.
- **Two identities are in play.** The CLI acts as the `gh auth` login; the browser creates the App
  as whoever github.com is signed in to. The page names the login on every screen and checks the
  owner after each conversion, because that mismatch is otherwise invisible until it is too late.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Role→App indirection — six roles on N Apps without symlinks](./brief-01-role-app-indirection.md) | 0 | M | todo | — | — |
| 02 | [`deskapps init` — loopback page, tier manifests, the manifest→code→conversion flow, key + record writes](./brief-02-deskapps-manifest-flow.md) | 1 | L | todo | — | — |
| 03 | [`deskapps` install + prove — installation poll, fresh mint, scopes-vs-duties, roster write](./brief-03-deskapps-install-prove.md) | 2 | M | todo | — | — |
| 04 | [`deskapps resume` / `status` — per-App state machine, throttle pause, expired-code re-arm, page verbs](./brief-04-deskapps-resume-status.md) | 2 | M | todo | — | — |
| 05 | [`deskavatar` — deterministic per-adopter App avatars with a 20 px legibility proof](./brief-05-deskavatar-generator.md) | 0 | M | todo | — | — |
| 06 | [Avatar step in the run board — generated PNG beside the settings page, confirmed not uploaded](./brief-06-avatar-step-wiring.md) | 3 | S | todo | — | — |
| 07 | [Install skill + adoption runbook cutover — `deskapps` replaces the hand runbook, tiers documented](./brief-07-install-skill-runbook-cutover.md) | 4 | M | todo | — | — |
| 08 | [Solo identity mode — spec + decision: the desk verbs on the operator's own token](./brief-08-solo-identity-spec.md) | 0 | S | todo | — | — |

## Critical path

```
01 (role→App indirection) → 02 (manifest flow) → 03 (install + prove) → 07 (skill + runbook cutover)
                                              └→ 04 (resume/status) ─┘
05 (avatar generator) ──────────────────────────→ 06 (avatar step) ──→ 07
08 (solo spec, human decision) — independent; its implementation brief is authored AFTER the ruling
```

**Head = 01, and it is the head for a reason the tool itself hides.** `desktoken` resolves a role's
credentials by role name: `<role>-app.pem` on the credential search path, `<ROLE>_APP_ID` and
`<ROLE>_INSTALL_ID` from the environment or `apps.env`. The Read + Act tier puts six roles on two
Apps. Without an indirection from role to App name, the only way to run that tier is six copies or
six symlinks of two keys — which works, silently, and is exactly the kind of custody folklore this
stream exists to retire. 02 must write one record per App and one binding per role, so 01 is what
02 writes into. Verified 2026-09-05 against `tools/desk/cmd/desktoken/desktoken.go` (the
`validRoles` map and the `<ROLE>_APP_ID` / `<ROLE>_INSTALL_ID` resolution) at `38e96f7`.

**Smallest unblocking move:** land 01. It is small, it changes nothing for an adopter already on
one-App-per-role (the default binding is the role's own name), and it is the seam 02, 03 and 07 all
rely on.

**Tempting-but-wrong first step:** start with 02 and have it write `<role>-app.pem` six times for
the two-App tier. It demos in a day and leaves every Read + Act adopter with a credential layout no
document describes.

## Dependency waves

```
Wave 0: [01, 05, 08]
Wave 1: [02] ← 01
Wave 2: [03, 04] ← 02
Wave 3: [06] ← 02, 05
Wave 4: [07] ← 03, 04, 06
```

## Shared conventions

- **Loopback only.** The page binds `127.0.0.1`. Every callback and every page verb carries a
  per-run `state` nonce, checked before anything is acted on. The page can invoke the CLI's fixed
  verbs only — resume, retry, confirm-avatar, cancel — never arbitrary arguments.
- **The private key is written once, mode 0600, and never printed, logged or rendered.** Tests
  assert the absence of key material in stdout, the audit log and the served HTML.
- **Every state change is one line on the console that launched the CLI.** A Claude Code session
  driving the install skill sees the same progress the page shows.
- **Nothing here asks for a password or a token of its own.** Identity comes from `gh auth` once,
  at start; installation tokens come from `desktoken`.
- **Exit codes follow the desk-tools contract** (0 ok · 3 disabled · 4 rate-limited · 5 refused ·
  6 could-not-check). A throttle pause is not an error and exits 0 with the resume line printed.
- **Public tree.** Briefs and code here are self-contained: no private repo names, no private
  issue references, bare `#N` only for issues in this repo.
