# `deskapps` — design of record (2026-09-05)

The installer for Assay's GitHub App identities. One command, one browser sitting. The program
does everything GitHub allows a program to do; the person does only the three things GitHub
insists a signed-in person do: click **Create**, click **Install**, drop in the avatar.

This document is the source every brief in the stream points at. It records the decisions; the
briefs carry the work. Where a brief and this document disagree, the brief is wrong and this
document is amended by the brief's PR.

## 1. Five rules

1. **The human only consents.** Create, Install and the avatar drop are GitHub's consent gates.
   Everything else — naming, permissions, key storage, record-keeping, verification — is the
   program's. If a step can be done by API, the person never sees it. The page always says whose
   consent it is asking for.
2. **Keys stay home.** The manifest conversion returns the private key, so whoever calls it holds
   the key. The call is made by the CLI on the adopter's machine, never by a hosted page. The PEM is
   written 0600 and is never printed, logged or rendered.
3. **Resumable by construction.** GitHub throttles App creation per account after roughly four
   Apps. Every App carries its own state record, so a throttled run is a pause, not a restart.
4. **Tier first, Apps second.** The first screen asks what the adopter is doing, not how many Apps
   they want. Solo needs none. A team needs two. The full suite is offered only where per-role key
   custody is real.
5. **It proves itself.** The run is done when every role has minted a token, the granted scopes
   match the role's duties, and the roster is written — not when the last click lands.

## 2. Screens

The CLI serves the page at `http://127.0.0.1:<port>/`. Step chips on every screen:
**Tier · Details · Create · Install · Prove**.

### Screen 0 — Tier (`/tier`)

A segmented control with three choices; the recommendation is pinned to the `--tier` flag the
CLI was started with (default `team`). The left column shows the copy for the tier under
consideration (purpose, description, *Runs on its own*, *You are responsible for*); the right
column is a "What this choice means" panel:

| Row | Solo | Read + Act | Full suite |
|---|---|---|---|
| Bot identities | 0 | 2 | 6 |
| Clicks to set up | 0 | 2 create · 2 install per org | 6 create · 6 install per org |
| Sittings | 0 | 1 | 2 (throttle met once) |
| Runs while you are away | no | yes | yes |
| Review verdicts posted by | you, as comments | `<prefix>-act[bot]`, as comments | reviewer App, formal review |
| Approves a PR | you | you | the reviewer App |
| Merges | you | you | you |
| Author and approver on GitHub | same login | same bot | distinct actors |
| Keys on this machine | none | 2 | 6 — hold them apart |
| Audit trail shows | your login | `<prefix>-read[bot]` · `<prefix>-act[bot]` | one login per role |

Switching the chip redraws both columns. All three stay visible because comparing is this
screen's job. Solo ends the run here (see §7).

### Screen 1 — Details (`/setup`)

- **Identity strip** on this and every later screen: avatar, login and email from `gh api user`,
  the orgs that login owns (from the memberships endpoint, role `admin`), and the sentence *"Your
  browser must be signed in to this same GitHub account. We check after Create."*
- **Owner**: org-owned (default; requires org owner) or personal-owned, with the custody trade
  stated: *an App created under a personal account leaves with that account*.
- **App names**: `<prefix>-read`, `<prefix>-act` (or the six role slugs). GitHub App names are
  globally unique; a collision gets a suffix suggestion, never a silent rename.
- **Permissions**, shown but not editable: fixed at creation by the manifest; any later widening
  needs GitHub's UI plus re-consent on every installation.
- **Avatars**, generated for this org (see §5), previewed.
- Footer: *Nothing is sent to GitHub until you click Create on GitHub's own page.*

### Screen 2 — Run board (`/run`)

One row per App, columns **Create · Key · Avatar · Install · Verify**, each cell a state chip.
Rows are ordered so a throttled first sitting still leaves a working review loop: reviewer,
worker, verifier, then desk, issue-loop, intake-loop (Read + Act: read, then act).

- **Create** posts an auto-submitting form carrying the manifest to GitHub's new-App page
  (org-owned: `/organizations/<org>/settings/apps/new`; personal: `/settings/apps/new`). GitHub
  redirects to `redirect_url?code=…&state=…` on the loopback listener.
- **Key** converts the code immediately (`POST /app-manifests/{code}/conversions`, code valid one
  hour), writes the PEM 0600, and records App ID, slug, client id and webhook secret.
- **Avatar** opens the App's settings page with the generated PNG shown beside a one-line
  instruction; the program cannot upload it, so it asks for confirmation and remembers the answer.
- **Install** opens `/apps/<slug>/installations/new`, then polls the App's installations with the
  App JWT until one appears for the target org.
- **Verify** mints a fresh installation token and runs `app-scopes-vs-duties`.

**Throttle banner** (amber): *GitHub paused App creation for this account. This is GitHub's
throttle, not an error in your setup. N Apps are created and keyed. Wait a while, then resume from
here. Closing this window loses nothing: the console can resume too.* Buttons: **Resume creation**,
**Close for now**. Under the banner, a console strip mirrors the line the launching console
received (`page → resume requested · re-arming 3 rows (…)`).

### Screen 3 — Prove (`/proof`)

The only screen that carries the gold fineness mark, because it is the one that certifies. Tiles:
tokens minted (n/n, as which login), scopes vs duties (match / the missing grant), roster written
(roles bound to their Apps), installation scope (selected repos or all), files written (paths, mode),
avatars (n/n confirmed). Then the next commands.

## 3. Mechanics of one App

```
CLI (127.0.0.1)                     Browser                          GitHub
  │ open /setup, serve manifest form (state nonce) ─▶ │
  │                                 │ POST manifest ─────────────▶ │
  │                                 │                 [person clicks Create]
  │                                 │   throttled → error page, no redirect;
  │                                 │   listener times out (10 min) → row paused
  │                                 │ ◀── 302 redirect_url?code&state ─┤
  │ ◀── GET /callback (state checked) ─┤
  │ POST /app-manifests/{code}/conversions ───────────────────────▶ │
  │ ◀── { id, slug, pem, webhook_secret, client_id } ────────────────┤
  │ write <app>.pem 0600; apps.env + apps.state.json
  │ open settings page; show PNG ──▶ │  [person drops avatar, confirms]
  │ open /apps/<slug>/installations/new ─▶ │  [person clicks Install, picks repos]
  │ poll installations (App JWT) ─────────────────────────────────▶ │
  │ mint token (desktoken) → app-scopes-vs-duties → roster write
```

Every state change is also one line on the console that launched the CLI. A Claude Code session
driving the install skill sees the same progress as the page, and can issue the same verbs
(`deskapps resume`) when the browser is gone. Two surfaces, one state machine, one owner.

## 4. Per-App state machine

```
pending → posted → keyed → avatar_ok → installed → verified
```

| Edge | Cause |
|---|---|
| pending → posted | manifest form sent |
| posted → keyed | conversion succeeded, PEM written |
| posted → **paused** | no callback within 10 minutes (the throttle); back to pending on resume |
| posted → posted | conversion 404 after the hour: code expired, Create again |
| keyed → avatar_ok | person confirmed the drop (unconfirmed does not block install; it nags on every status) |
| avatar_ok/keyed → installed | installation poll sees the target org |
| installed → verified | fresh mint, scopes cover duties |
| verified → installed | scopes ≠ duties: banner names the missing grant and the GitHub page to fix it |

The record is `~/.config/assay/apps.state.json` (schema `deskapps-state-v1`, one object per App:
slug, tier, role bindings, state, timestamps, app id, installation ids, avatar confirmed). It is the
only thing `deskapps resume` reads.

## 5. Avatars

Generated by `deskavatar`, composed as SVG in Go and rasterised offline to a 512 px PNG (GitHub's
uploader takes raster). Rules:

- **The octagon is constant.** It is the Assay stamp and the one thing that marks every Assay App.
- **At 20 px only two signals survive: tile colour and one bold silhouette.** GitHub renders App
  avatars at 20 px in timelines and about 40 px in review headers, cropped to a circle.
- **Full suite: hue belongs to the role and colours the whole tile** (frame and glyph). Six hues
  spaced for maximum separation on a near-black ground. Glyphs are solid, about sixty percent of the
  canvas, no stroke thinner than 7 units on a 64 grid, each with a distinct outline (ring, check,
  hammer, disc, star, ticket, funnel) so the set survives monochrome viewing.
- **Read + Act: inversion, not hue.** The adopter's hue (hashed from the org login) colours both
  tiles; read is a dark tile with a light glyph, act a filled tile with a dark glyph.
- **Adopter identity sits behind the glyph** — initials or a quantized org avatar as a low-contrast
  field, visible at 64 px and above, invisible at 20 px where the role must win.
- **The fineness mark leaves the uploaded avatar.** It stays on the large masters; at 20 px it is
  unreadable and steals a third of the height.
- **Every set is proofed at 20 px before it is written.** Pairwise colour distance and silhouette
  overlap; a pair under threshold fails the run and names the two roles.
- **Deterministic.** Same org, same tier, same bytes.

## 6. Command surface

```
deskapps init   --tier team|family [--org <login>] [--owner org|me] [--prefix assay] [--port 41873] [--no-browser]
deskapps resume
deskapps status
deskapps avatar --regen
deskavatar --org <login> --tier team|family --out <dir>     # also callable on its own
```

Every verb is safe to run twice. `--no-browser` prints the URL; only `127.0.0.1` is ever bound.
Files: `~/.config/assay/apps.env` (App and installation IDs per App, plus the role→App bindings of
brief 01), `apps.state.json`, `<app>.pem` (0600), `avatars/<app>.png` + `.svg`, and the existing
`tokens/` cache.

## 7. Solo

Solo creates no App and exits after Screen 0 having written a roster that names the operator's
login. Whether and how the desk verbs run on that login — a user token in place of a role token,
role as a label rather than an identity, GitHub's own refusal of self-approval as the restored
control — is a decision, not a default. Brief 08 writes that spec and files the decision; its
implementation is authored after the ruling.

## 8. Failure conditions the page must handle

| Condition | Detected by | The person sees |
|---|---|---|
| Creation throttled | no callback within 10 min | amber banner, count done, Resume button; rows paused, nothing retried on its own |
| Code expired | conversion 404 after > 1 h | row back to posted, "Create again on GitHub"; no key was written |
| Name taken | GitHub rejects the manifest name | suffix suggestion, accept or edit; never a silent rename |
| Browser signed in as someone else | personal-owned: conversion owner ≠ `gh` login; org-owned: GitHub refuses the form for a non-owner | red identity strip naming both accounts, a link to switch, Create re-armed; nothing written |
| Not an org owner | org-owned manifest URL forbidden | offer personal ownership with the custody trade stated, or hand the URL to an owner |
| Installed on the wrong account | installation poll sees another login | row shows where it landed with an uninstall link, keeps waiting for the right one |
| Scopes ≠ duties | proof step | red row naming the exact permission and the GitHub page; reminds that re-consent follows |
| Avatar never dropped | tab closed without confirming | install proceeds; warn chip on status and proof until confirmed |
| Port in use | bind fails | next free loopback port, said so; the manifest redirect follows the port actually bound |
| Cross-org cell | repo list spans two orgs | says one App still needs one install and one token per org; runs the install step per org |

## 9. Measured before building (brief 02 records the answers)

1. Does the throttle count conversions, or only the create form? Decides whether codes are
   converted the instant they arrive (the plan) or may be batched.
2. Does the org-owned manifest URL require org owner, or does an App-manager role suffice?
3. Is a redirect to a loopback URL accepted on GitHub Enterprise Server?
