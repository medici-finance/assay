# Adopting Assay — install runbook for an LLM agent

There are two ways to install Assay, easiest first:

1. **The turnkey `assay:install` skill (recommended — right below).** Add the plugin, invoke one
   skill, and it self-installs the whole setup, stopping only at the human-gated steps. Most
   adopters should start here.
2. **The manual runbook (further down).** The full step-by-step PRIMITIVEs and three adoption
   scenarios that same skill wraps. Reach for it directly for a carve-out, a multi-repo suite, or
   any non-standard boot — and it is the ground truth the turnkey path delegates to.

This runbook is GitHub-shaped throughout (Apps, rulesets, `gh`). Running the fleet on GitLab
Enterprise instead — service accounts in place of Apps, protected-branch push-access lists in
place of ruleset bypass — is a separate profile: see
[`docs/adopting-assay-gitlab.md`](adopting-assay-gitlab.md).

## Fastest path — the turnkey `assay:install` skill

For a straight install (most adopters), the fastest coherent boot is three steps: add the plugin
from the marketplace, install it, then invoke the installer skill.

```text
/plugin marketplace add medici-finance/assay
/plugin install assay@assay
```

Then invoke **`assay:install`** (the `install` skill, surfaced namespaced once the plugin is
installed). It self-installs the whole setup, driving each step itself and stopping only at the
never-autonomous escalation points below.

**Prerequisites.**

- **Claude Code**, with the plugin installed (the two `/plugin` commands above).
- An **authenticated `gh`** — the skill downloads the pinned statusgen release with
  `gh release download`.
- **macOS or Linux.** The statusgen binary acquisition is Unix-first; **Windows** is a named
  fast-follow, not yet in scope — on a Windows host the skill stops at that step rather than
  guessing.

**What the skill does** — it DELEGATES every PRIMITIVE and every human-gate to the manual runbook
below; the two are **one story with one mechanism**, not two implementations:

- **Detects + confirms the target repo** — names the absolute target path and detected `os-arch`
  back to you, and writes nothing until you confirm.
- **Scaffolds** via `statusgen init` — emits a lint-clean `docs/streams/` tree, the registers, and
  a bootstrap-safe CI workflow.
- **Acquires a version-PINNED, sha256-verified statusgen binary** — the tag resolved from the
  plugin↔statusgen pairing in `plugins/assay/paired-versions.yaml`, **never** `latest`; a hash
  mismatch is a hard **REFUSE**, not a warning, so no unverified bytes are ever installed.
- **Wires CI + the main-guard** — confirms the two-half single-writer workflow and the client-side
  commit guard rather than re-authoring them.
- **PROVES the install** — `statusgen --root <target> --lint` exits **0** and `statusgen --version`
  prints the **pinned tag**; if either check fails it reports the install **not proven** and stops,
  never a fabricated success.
- It is **idempotent**, REFUSES-not-clobbers an already-adopted repo, and opens **draft** PRs only.

**Three human-escalation points the skill stops at** — it hands you the exact values and waits,
never fabricating the outcome:

- **Private-repo CI auth** — the release-download token / module-privacy env a private repo's CI
  needs.
- **The reviewer GitHub App** — creation + installation of the separate review identity a worker
  session cannot post as (attribution, not authorization — see **§1a** below).
- **Any write to `main`** — merge, `git push origin main`, the release tag, and the first
  ready-flip are the human's; the skill only opens draft PRs.

**When to skip the turnkey path** and walk the manual runbook below instead: a carve-out, a
multi-repo suite, or any non-standard boot. `assay:install` is the turnkey wrapper over the
runbook, not a replacement for it — the runbook is the ground truth it delegates to.

---

# The manual runbook

**Audience: an AI coding agent (Opus-class) that will EXECUTE these steps.** Human-readable
second. The guide is a runbook, not an essay: numbered imperative steps, exact paths and
commands, a **Verify:** check under each step, and explicit **ESCALATE** callouts where a step
crosses into human-gated territory (App creation, repo permissions, merge/push, history rewrite).

Three adoption scenarios, each composing the same install PRIMITIVEs defined in **CORE**:

1. **Green-field** — a brand-new repo (or small set of new repos) with no history to preserve.
2. **Existing suite** — Assay across two or more repos that already have code, history, and CI.
3. **Carve-out** — extract part of an existing project into its own Assay-tracked unit.

## How to use this guide (agent)

1. Read **CORE** fully — it defines what Assay is, the component inventory, and the named
   PRIMITIVEs (`install-statusgen`, `scaffold-streams`, `scaffold-registers`, `add-statusgen-ci`,
   `install-desk-plugin`, `configure-roster`, `create-labels`, `install-main-guard`, `first-board`,
   `setup-reviewer-app`) that the scenarios reference rather than re-explain.

   > **Renamed** (`vendor-statusgen` → `install-statusgen`). The old name encoded
   > the wrong default. Vendoring the source is no longer recommended in any scenario.
2. Pick the ONE scenario that matches the situation and follow it top to bottom.
3. Never skip a **Verify:** — each proves a PRIMITIVE landed before the next depends on it.
4. At every **ESCALATE** callout, stop and hand the named values to a human; never fabricate the
   outcome (a faked reviewer App or a self-granted permission defeats the mechanism it installs).
5. Carry the honest framing (CORE §1): the board is *derived from agent-authored artifacts with
   linting and independent re-verification*, not *measured from ground truth*. Claim the weaker
   true thing.

## Prerequisite: two GitHub accounts — the bot's, and yours

Before any step below, make sure **two distinct GitHub identities** exist and that you can act as the second:

1. **The automation account** — the identity the fleet *runs as*: it authors PRs, pushes branches, runs CI, mints the reviewer-App token, and posts App reviews. In a solo setup this is one machine account plus the reviewer GitHub App it owns; in a larger setup, the set of role Apps. Everything an agent does, it does as this identity.
2. **Your human account** — a *separate* personal GitHub account that is **you**, and whose credentials the automation never holds. It does the things only a human may: it is the `ASSAY_BLESS_LOGIN` (the authorization half of the trust gate — `configure-roster`), it posts the sign-off on a `gate:human` brief, it adds the 👍 reaction that clears a public-repo write, and it clicks **merge**.

**Why this is not optional.** Assay's gates separate *proposing* work (the agents) from *authorizing* it (a human). That separation is only real if the two are different GitHub principals. If the fleet runs as the same account that blesses, approves, and merges, the automation can bless, approve, and merge its *own* work — every human gate becomes self-satisfiable and the model is theater. Under one shared identity, bot-vs-human is undecidable: a verdict, a 👍, or a merge attributed to that account attributes to *both* (§1a). The human account is precisely what the automation must be unable to act as — that is what makes "a human approved this" a claim the fleet cannot forge.

**What it does *not* buy you** (the honest weaker claim, §1a): two accounts give you *attribution*, not enforcement — on a free/private plan the merge gate stays advisory until branch protection is on (public repos unlock it). Two accounts are the necessary floor, not the whole control.

> The reviewer **GitHub App** (`setup-reviewer-app`) is part of identity (1), the automation side — it makes a *review* attributable. It is **not** your human account and does not replace it: an App reviews, a human authorizes. Keep both.

## Human-gate quick reference (never autonomous)

| Act | Why it's human-gated | Where |
|-----|----------------------|-------|
| Reviewer GitHub App creation + install | Mints an irreversible public identity + grants repo perms; the bot login is what makes a review **attributable to an identity the author cannot post as** — attribution, not authorization (§1a) | CORE `setup-reviewer-app`, §1a |
| Choosing the roster VALUES — blessing authority, trusted logins, allowed repos | Names who the tools obey; `ASSAY_BLESS_LOGIN` is the authorisation half of the trust gate and must be a human. Writing the file is autonomous; picking the identities is not | CORE `configure-roster` |
| Repo creation + admin/collaborator grants | Irreversible public act; permission model is the distribution boundary | Scenario 3 §3a |
| Merge to `main` / `git push origin main` / release tag / first ready-flip | The deploy/merge gate; agents open **draft** PRs only | CORE §4, every scenario |
| git history rewrite (`filter-repo`) for a carve-out | Destructive + irreversible; a squash loses the audit trail | Scenario 3 §3a |
| Private-repo CI auth (`GOPRIVATE`, cross-repo checkout token) | Credential provisioning | Scenario 2 §3 |

## Human post-install checklist — what you must do after `assay:install`

The skill does the autonomous parts — it scaffolds streams + registers, acquires the pinned
statusgen binary, wires CI, installs the plugin, and opens **draft** PRs — but it **STOPS at every
act that mints an identity, grants a permission, or authorizes a merge**. Those are yours, and the
skill cannot fake them without defeating the mechanism it installs. Here is the ordered list, so a
finished `assay:install` run hands you a checklist rather than the impression that nothing is left:

1. **Provision the two GitHub accounts.** The automation account the fleet runs as, and your own
   **separate human account** — the two are the trust boundary the whole model rests on. See the
   **two-accounts prerequisite** (§2 `automation identity`); this checklist does not re-explain it.
2. **Create + install the GitHub Apps** (creation, key-gen, and install are repo-admin-only):
   the **reviewer App** (`pull_requests: write`, `contents: read-only` — it must not be able to
   commit — plus `checks`/`statuses`/`actions: read` so it can read CI to gate on it; if you run the
   full desk pipeline, grant those same three read scopes to the worker + desk Apps, never to a
   verifier / inbound-lane App); the **board-writer App** *only if you turn on branch protection* (`contents: write`
   only, added to the ruleset bypass — §3 `add-statusgen-ci`); and the **automation identity** the
   fleet runs as. For **each** App: generate the private key (PEM), store it at the config-home
   (`~/.config/assay/`, mode `0600`) — **never in the repo tree or a committed env file** — install
   the App, and record its App ID + install IDs where the token minter reads them. See the §2 App
   inventory rows and `setup-reviewer-app` for the per-App detail.
3. **Choose the roster VALUES** — naming who the tools obey is a human act, never autonomous: the
   **bless login** (must be your human account), the trusted logins, the allowed repos, and the
   risk-path triggers. Write them to the config-home roster file **and** set the org/repo Actions
   variables for the CI reporting half. The desk binaries read the config-home file **only**, never
   the environment (§3 `configure-roster`, failure mode 1).
4. **Set the repo/org variables + secrets.** Actions **variables** = the roster values for the
   reporting half; Actions **secrets** = the App private key(s) the workflows mint tokens from, plus
   any private-repo CI auth. **Never put a PEM or a token in the repo tree** — a committed key is a
   published key.
5. **Branch protection (recommended-but-optional) + board-writer bypass.** If you enable protection
   on `main`, add the board-writer App to the protect-`main` ruleset bypass **and** — separately —
   to any required-check ruleset (e.g. a leak-sweep); bypassing one ruleset does not bypass another.
   Then **confirm the ruleset actually lists the App** — a workflow can pass YAML review yet be
   rejected at runtime by a ruleset that never got the bypass (§3 `add-statusgen-ci`).
6. **The ongoing human gates — not one-time.** The ready-flip is the desk's, but the **merge**, the
   **push to `main`**, the **release tag**, any **history rewrite**, and the **👍 that clears a
   public write** are yours *every time*, not just at install. The quick-reference table above is
   the standing list.
7. **Optionally acquire the desk-tools.** If you run the automated desk pipeline, install the pinned
   desk-tools binaries — see the **desk-tools acquisition** (`install-desk-tools` in §3). Without
   them the desk skills fall back to raw `gh`/`git`; with them you get the guards, write-budgets,
   and roster/trust gates.

---

# CORE

*The foundation. The three scenario sections compose the named **PRIMITIVE**s defined here —
install them once, reference them by name.*

## 1. What Assay is (read before installing)

Assay is an operating model for running fleet-of-agents software work behind machine-checkable
gates. Four load-bearing parts: **briefs** (self-contained scope-and-DoD units with typed
dependencies, a risk-derived review gate, and an executable Verify table — `docs/brief-template.md`);
**registers** (append-only FINDINGS / INTAKE / RETRO logs with tombstone-not-deletion and enforced
sequence-contiguity — `docs/registers.md`); a **lifecycle** (`todo → in-progress → implemented →
verified → done`, where `verified` is a distinct step a *non-implementer* runs — `docs/lifecycle.md`);
and **`statusgen`**, a repo-agnostic Go tool that is the **single writer** of the generated
`STATUS.md` board and computes a Next-up work queue. Around it sit the methodology's **loop roles** —
brief authoring, fan-out dispatch, PR review, post-merge verification, and desk coordination. The
public bundle ships two portable, domain-neutral skills (`assay:adopt`, `assay:author-brief`) plus a
growing set of desk-role skills carrying the *current house* methodology for those loop roles
(`assay:the-desk`, `assay:intake-desk`, `assay:batch-fanout`, `assay:pr-review-desk`,
`assay:verify-desk` — see `install-desk-plugin`); an adopting team may run those as-is, fork them, or
author its own project-local equivalents instead. There
is also a **separate review identity** — a GitHub App whose bot login an author cannot post as
(attribution, not authorization; `setup-reviewer-app` in §3). Read **§1a** before you describe that
property to anyone; it is narrower than it sounds.

**The honest framing you must carry (do not overclaim):** the board is *derived from agent-authored
artifacts with consistency linting and independent re-verification* — **not** *measured from ground
truth*. `statusgen` parses markdown written by the same agents whose work it reports; it checks
internal consistency (sequence gaps, missing evidence, unresolved findings, malformed gates) and is
backstopped by adversarial spot-verification. The value is that drift, missing evidence, and register
tampering become *machine-visible* — not that agents become trustworthy. When you describe what you
installed, claim the weaker true thing.

## 1a. What the review gate actually gives you — say this, not more

This guide used to call the gates **tamper-evident**. They are not, so that
word is retired here — soften the
language and publish the honest weaker claim. Retire these phrases — *"tamper-evident"*,
*"cannot be forged"*, *"impossible to self-certify"* are all out; they overclaim.

**Say this:** *approval is posted by a separate identity the author cannot impersonate.*

That is a claim about GitHub App attribution, and it is true. Stronger claims are not. Three
recorded reasons:

- **A worker can mint the App token.** If an implementer session can mint the reviewer App's
  token, it can drive approve → ready-flip → merge on its own PR. The identity is genuine; the
  separation is not. Attribution is a property of the credential, and whoever holds the
  credential holds the property.
- **The merge gate is advisory, not enforced.** On a plan or setup without server-side branch
  protection, nothing on the server requires a merge to wait for the verdict. The App approval
  is always **attribution**, never **authorization**, and conflating the two is the actual
  defect.
- **Bot-vs-human is undecidable under one shared identity.** When agents and a human act
  through the same account, no downstream check can tell which one acted. A verdict attributed
  to that account attributes to *both*.

**What survives all three, and is the thing worth installing:** a review posted by the App is
*attributable* — it names a distinct identity that is answerable for the verdict, and a plain
worker session cannot post as it. That is strictly more than a self-written checkmark, which is
what most fleets have. It is not proof the review happened, was thorough, or was independent.

**When you install this, report it in those terms** — a stronger sentence in a handover note is
the failure mode the ruling exists to stop.

**Hardening once the repo is public.** The gate gets stronger by *other* means than better
adjectives, and those are the items to schedule rather than assert: make the shared machine
account **read-only** so it cannot merge or push; add **CODEOWNERS** so review by
the owning identity is required by the server rather than by convention; and note that going
public is itself what *unlocks* branch protection on a free plan, turning
the advisory merge gate into an enforced one. Branch protection is **recommended but optional**;
if you turn it on, the push-to-main board regen can no longer commit `STATUS.md` directly and
needs a dedicated **board-writer App** to push past protection — see §3 `add-statusgen-ci`. None
of these is done. Until they are, the honest sentence above is the whole of the claim.

## 2. Component inventory

| Component | What it is | Comes from (`assay`) | Lands in target repo |
|---|---|---|---|
| **statusgen** | Go module (stdlib + `yaml.v3`): generates `STATUS.md`, computes Next-up, lints sources | a **release binary** built from `statusgen/` | **no source in your repo** — a `.assay-versions` pin plus the installed binary on `PATH` |
| **statusgen CI** | Two-half workflow: PR-side `--lint`, main-side regenerate-and-commit | **not shipped** — house-specific CI you author yourself to the shape in §3 `add-statusgen-ci` | `.github/workflows/statusgen.yml` (you write it) |
| **streams layout + templates** | `docs/streams/<stream>/README.md` (frontmatter + brief table) and brief-v1 files | `docs/brief-template.md`, `docs/brief-rules.md`, `examples/adopter-scaffold/` | `docs/streams/<stream>/` |
| **registers** | Append-only FINDINGS / INTAKE / RETRO logs | `docs/registers.md`; scaffold's `FINDINGS.md` / `INTAKE.md` | `docs/streams/{FINDINGS,INTAKE,RETRO}.md` |
| **mistake-proofing discipline** | Normative rules for the *devices* — how a check/gate/guard/scaffold is classified and kept honest (D1-D7), plus the brief-authoring rules (B1-B10) | `docs/mistake-proofing.md` — adopt it incrementally in the value-per-cost order of its **§5 adoption ladder**, which is the on-ramp | reference only — no file lands; the rules bind the checks you write |
| **methodology skills / plugin** | The two portable methodology skills (`assay:adopt`, `assay:author-brief`) plus the desk-role skills for the five-desk pipeline (`assay:the-desk`, `assay:intake-desk`, `assay:batch-fanout`, `assay:pr-review-desk`, `assay:verify-desk`), namespaced `assay:<name>` | `.claude-plugin/marketplace.json`, `plugins/assay/` — see `install-desk-plugin` | installed via `/plugin`, cached under `~/.claude` |
| **desk-tools** | The desk-role **binaries** (`deskboard`, `deskpr`, `deskevidence`, `deskfile`, `deskpost`, …) the five desk-role skills drive as their **primary** path — they carry the guards, write-budgets, and roster + trust gates. **Optional-but-recommended**: without them the desk skills fall back to raw `gh`/`git` (works, but loses the guards). Acquired as a pinned, sha256-verified tarball — the **same mechanism as statusgen** | a **release tarball** (`desk-tools-<platform>.tar.gz`) from the same release as statusgen — see `install-desk-tools` | **no source in your repo** — a `.assay-versions` pin plus the installed binaries on `PATH`; config at the config-home (`~/.config/assay/`) |
| **reviewer GitHub App** | The separate review identity (§1a) — attribution, not authorization; `pull_requests: write` with **no** `contents: write`, plus **`checks`/`statuses`/`actions: read`** so it can read CI to gate on it, is the *recommendation* (§3 `setup-reviewer-app`) | CORE `setup-reviewer-app` (runbook) | GitHub org/account settings — **not a repo file** |
| **automation identity** | The account (and/or role Apps) the fleet **runs as** — authors PRs, pushes branches, runs CI, mints tokens. Distinct from the human account (the two-accounts prerequisite). Minimum = one machine account plus the reviewer App it owns; larger fleets *optionally* split it into role Apps (worker / verifier / desk / loop) — a decomposition, **not** a requirement | operator-provisioned (GitHub org/account) | GitHub org/account settings — **not a repo file** |
| **board-writer GitHub App** | Needed **only** if `main` is branch-protected (§3 `add-statusgen-ci`): a dedicated App with **`contents: write` only**, added to the branch's ruleset bypass so the push-to-main statusgen regen can commit `STATUS.md` past protection. Not needed when protection is off | CORE `add-statusgen-ci` (when protection is on) | GitHub org/account settings + the branch's ruleset bypass — **not a repo file** |
| **.githooks main-guard** | `pre-commit` refusing `main` commits without `ASSAY_MAIN_COMMIT_OK` (worktree-isolation backstop) | parent-project hardening (not shipped in the toolkit) | `.githooks/pre-commit` + `core.hooksPath` |
| **trust roster** | The configured trust/authority surface the tools read: who is trusted, which repos are writable, which paths force a security review — held outside every ref, so a PR cannot widen its own gate. Governs repos the tools may **act** on, not the multi-repo board's own root-to-path map (separate, compiled-in — see `configure-roster`'s board caveat) | CORE `configure-roster` (mechanism) | **not a repo file** — org/repo Actions variables plus `~/.config/assay/roster.env` per operator (§3 `configure-roster`) |
| **adopter-scaffold example** | Minimal worked example: `STATUS.md` + two streams + registers | `examples/adopter-scaffold/` | reference only — copy the shape, don't vendor |

## 3. Core install primitives

Each primitive is idempotent (safe to re-run) and ends with a **Verify:** the agent runs to confirm
success. Scenarios compose these by name. `TARGET` = the repo you install into (absolute path);
`SRC` = a checkout of `assay`.

### PRIMITIVE: install-statusgen
Put the generator where the target can run it. **There is one recommended channel**, and it is
the one an adopting team should actually run:

- **(E) Pinned release binary, sha256-verified — DEFAULT for every scenario.** Commit a
  `.assay-versions` file at the target repo root with a line per platform you install on:
  `statusgen-<platform> <tag> <sha256>` (e.g. `statusgen-darwin-arm64 statusgen/v0.6.0 5985…`).
  Install by: detect platform → `grep "^statusgen-$platform " .assay-versions` → **refuse if
  absent** → `gh release download "$tag" --repo medici-finance/assay --pattern
  "statusgen-$platform"` → `shasum -a 256` → compare to the pinned digest → refuse on mismatch
  → `install -m 0755 … <bindir>/statusgen`. The pin file is one line per platform in the form
  above; the load-bearing rules are: match the **full** platform (os *and* arch, per the Verify
  below), refuse rather than guess when the line is absent, keep each pinned artifact name distinct
  from any CI-job name, and re-pin (never edit in place) on an upgrade so the bump shows in a diff.

**Verify:** the pin line for the **fully detected platform** exists — match os *and* arch, not the
os family, or a `darwin-amd64`-only pin file passes on a `darwin-arm64` host while the install
correctly refuses:

```bash
plat="$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')"
grep -q "^statusgen-$plat " "$TARGET/.assay-versions"   # trailing space: see the pin-file rules
```

Then `statusgen --version` prints the pinned tag; and re-running the install with a deliberately
corrupted digest **fails** (proves the hash check is live, not decorative).

#### Channels that are NOT the default (and why)

| | Channel | Status |
|---|---|---|
| **A** | **Vendor the source** — `cp -R "$SRC/statusgen/." "$TARGET/statusgen/"` | **RETIRED as a recommendation.** A vendored copy is a fork and forks rot silently: a real vendored `statusgen` fork **frozen** at an old commit, with no pin, missing `--budget` / `--changed` / multi-root — still gates a downstream repo's PRs. Use only if you genuinely cannot reach the release (fully air-gapped), and then record the fork and its base tag explicitly. |
| **B** | **Git submodule** — add `assay` as a submodule and build `statusgen` from it | **Never recommended, and not sanctioned.** Listed only so the A–E labels stay stable against the single-sourcing design that names the same four alternatives (submodule / published module / CI fetch-and-run / copy). It pins *source* like D, adds submodule-update failure modes to every clone and CI checkout, and still hash-checks nothing. |
| **C** | **Published Go module** — `go run github.com/medici-finance/assay/statusgen@<ver> --root .` | **Not sanctioned.** Rebuilds from source per run, so it verifies nothing by hash — a version resolves, but no artifact is ever checked, which is the same guarantee gap as B and D without D's excuse. |
| **D** | **CI fetch-and-run at a pinned ref** — check out `assay` at a pinned ref and `go run ./statusgen` | Was the recommendation for suites. Superseded by **E**: a pinned ref pins *source*, not the artifact, so the thing that runs is rebuilt each time and never hash-checked. Keep only where a runner cannot download release assets. |

Whichever you pick, **record it** — the per-repo invocation differs by where the tool is rooted,
and a suite that mixes channels is the inversion this doc exists to stop.

### PRIMITIVE: install-desk-tools — OPTIONAL
Acquire the desk-role **binaries** — the tools the five desk-role skills drive as their primary path
(they carry the guards, write-budgets, and roster + trust gates). **Optional**: install this only if
you run the automated desk pipeline; the desk skills otherwise fall back to raw `gh`/`git`, which
works but loses the guards. The mechanism is **channel-E, identical to `install-statusgen`** — a
`.assay-versions` pin plus a sha256-verified download — differing only in that the artifact is a
`.tar.gz` of binaries, not a single file:

- resolve the paired tag + per-platform sha256 from `plugins/assay/paired-versions.yaml`'s
  `desk-tools:` section (the desk-tools and statusgen are cut from the **same release**, so they pin
  the same tag) — **refuse if the pin line for the detected platform is absent**, never guess;
- `gh release download "$tag" --repo medici-finance/assay --pattern "desk-tools-$platform.tar.gz"`;
- `shasum -a 256` the tarball → compare to the pinned digest → **refuse on mismatch**;
- extract and install the binaries to a `bindir` on `PATH`.

Configuration is at the **config-home only** (`~/.config/assay/`) — the same roster file the acting
tools read everywhere (§3 `configure-roster`, failure mode 1); the environment is never a transport
for a desk binary.

**Verify:** the desk-tools pin line for the **fully detected platform** exists in `.assay-versions`;
`deskboard --version` prints (the binaries are on `PATH`) and its `assay-config:` echo shows the
roster present; a deliberately corrupted digest makes the install **fail** (proves the hash check is
live).

### PRIMITIVE: scaffold-streams
Create the streams tree and one stream README with the required frontmatter + brief table (copy the
shape from `examples/adopter-scaffold/docs/streams/example-service/README.md`). Author the first
brief by copying `docs/brief-template.md` into `docs/streams/<stream>/brief-01-<slug>.md` and filling
**every** frontmatter field — empty `sources:` or a missing `risk:` answer is a gap, not a shortcut.
`gate` is *derived* (any `risk: yes` ⇒ `human`), never chosen.

`statusgen init` also writes the day-one agent-instruction files at the target root — a `CLAUDE.md`
carrying the ten invariants + the CI recipe + an unanswered bindings checklist, and an `AGENTS.md`
pointing at it (see *Your own instruction file* below). Both are skipped if already present: `init`
never overwrites, so an adopter that already has one keeps it byte-for-byte.

**Verify:** `statusgen --root "$TARGET" --lint` → exit 0.

### PRIMITIVE: scaffold-registers
Create the three append-only logs (`docs/streams/{FINDINGS,INTAKE,RETRO}.md`; formats in
`docs/registers.md`). `statusgen` consumes FINDINGS + INTAKE (findings hold affected briefs out of
Next-up until resolved); RETRO is the cadence log. Numbering must be gap-free; withdraw with a
tombstone (keep the number, flip disposition), never by deleting a heading.

**Verify:** `--lint` exits 0; then delete a middle entry (`## I-02` between I-01/I-03) and re-run —
expect NON-zero (proves sequence-contiguity is live). Restore it.

### PRIMITIVE: add-statusgen-ci
Author a CI workflow at `$TARGET/.github/workflows/statusgen.yml` — this workflow is house-specific
and is **not** shipped in the toolkit, so write your own to the shape described here (adjust
`paths:` if `docs/streams/` isn't at the root). Two halves enforce single-writer: the **PR half**
runs `--lint` only and blocks any PR whose diff touches `STATUS.md`; the **push-to-main half**
regenerates and commits `STATUS.md`.

**The workflow set — statusgen is not the only one, be honest about which apply to you.** "Wire CI"
is more than one workflow, and conflating them leaves an adopter either missing the security gate or
copying release plumbing they have no use for. Enumerated:

- **`assay-statusgen`** (**required**) — the lint + regen halves above; this is the one this
  primitive installs.
- **`leaksweep-control` + `leaksweep-pattern`** (**recommended, especially for a public repo**) —
  the automated layered-defense gate that scans a diff for secrets before it can land. On a
  branch-protected repo this is a **required check**, which means it is one the **board-writer App
  must bypass** (see the board-writer note above) — the App's push-to-main board regen has to clear
  it, so the App needs the bypass on **that** ruleset too, not only the protect-`main` one.
- **`ci` / `release` / `docker-publish`** — these belong to the repo that **builds and releases the
  tools** (the release home), **not** to a consuming adopter. An adopter pins sha-verified binaries;
  it does not cut releases. Add these **only** if you host your own tool releases; a plain adopter
  does not, and copying them in is release plumbing with nothing to release.

**Branch protection changes how the regen half lands (recommended-but-optional).** Branch
protection on `main` is *recommended* — it turns the advisory merge gate into a server-enforced
one — but it is **optional**. Enabling it has a consequence the regen half must account for: the
push-to-main half commits the regenerated `STATUS.md` **directly** to `main`, and a protected
branch (a PR-required ruleset, required status checks, or classic protection) **rejects that
push**. So if you enable branch protection you also need a **board-writer GitHub App**:

- a dedicated App with **`contents: write` only** — least-privilege, NOT `github-actions[bot]`
  (whose broad Actions identity would let any workflow push to `main`);
- added to the branch's **ruleset bypass list** (identity-based, "Always") so its push is admitted
  while every other actor still goes through PRs. If a **required check** also guards the branch
  (e.g. a leak-sweep), the App needs the bypass on **that** ruleset too — bypassing one ruleset
  does not bypass another;
- the regen job mints the App's token and pushes as it. Validate two things *separately*: that the
  workflow YAML parses, AND that the target branch's ruleset actually lists the App — a job can
  pass YAML review yet be rejected at runtime by a ruleset that never got the bypass.

Without branch protection the regen half pushes as the automation account directly and no
board-writer App is needed. This is the one place the recommended hardening adds a *required
identity* — plan for it before you turn protection on, not after the board silently stops updating.

> **BOOTSTRAP-SAFE GUARD — required.** The regen step must guard on `git status --porcelain -- STATUS.md`,
> **not** `git diff --quiet -- STATUS.md`. `git diff` only sees *tracked* files, so on a repo with no
> `STATUS.md` yet the guard mis-fires ("nothing to commit") and the first board is **never created**.
> Make sure the workflow you author uses the `git status --porcelain` form here, **not**
> `git diff --quiet -- STATUS.md`.

> **BOOTSTRAP-SAFE GUARD (the `--lint` half) — required on day one.** A fresh adopter's
> `docs/streams/` is legitimately empty until the first stream is authored, and a bare
> `statusgen --lint` treats that empty root as a PROBLEM (*"docs/streams: exists and is
> readable but resolves to 0 streams — pass --allow-empty-root…"*), so the PR half reddens
> your very first PR's CI. While no streams exist yet, run the PR half as
> **`statusgen --lint --allow-empty-root`** so day-one CI is green. **Drop the flag the moment
> your first real stream lands** — that same empty-root diagnostic is what later catches a typo
> or rename that accidentally WIPES `docs/streams/`, so keeping the flag permanently would
> silence that guard.

**Verify:** `grep -q 'skip-status-regen' …/statusgen.yml && grep -q 'STATUS.md is generated' …/statusgen.yml`; and `grep -F 'git status --porcelain -- STATUS.md' …/statusgen.yml` matches (bootstrap-safe). After first push to main, `STATUS.md` appears in one `[skip-status-regen]` commit; a PR editing `STATUS.md` fails lint.

### PRIMITIVE: install-desk-plugin
Install the methodology plugin so the skills surface namespaced (`assay:<name>`):
`/plugin marketplace add medici-finance/assay` then `/plugin install assay@assay`.

> **What the public bundle ships — two portable skills plus the desk-role skills.**
> `plugins/assay/skills/` ships the two portable, domain-neutral methodology skills — **`adopt`**
> (this install runbook, as a skill) and **`author-brief`** (the brief-authoring methodology) — and
> the five desk-role skills that carry the taught five-desk pipeline whole: **`the-desk`** (coordination),
> **`intake-desk`** (the front door), **`worker-desk`** (dispatch), **`pr-review-desk`** (review),
> and **`verify-desk`** (post-merge verification). `plugins/assay/skills/README.md` names the current
> set — check it rather than this paragraph, which will drift; `ls plugins/assay/skills/*/SKILL.md`
> in a fresh checkout enumerates all seven.
>
> **The desk-role skills carry the current house methodology, published self-contained.**
> All five desk-role skills — `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk` and
> `verify-desk` — ship scrubbed of internal repo slugs, issue numbers and names, the same as
> `adopt`/`author-brief`, and each reads as a self-contained account of its role that you can
> follow in another project. An adopting team may run them as-is, fork and rewrite them, or author
> its own project-local equivalents in the naming convention at `docs/skill-naming.md` instead —
> the method is in this guide and the shipped skill bodies, not a copied skill body you must use
> verbatim.

**Verify:** `/plugin` lists `assay` installed; `ls plugins/assay/skills/` enumerates the set
`plugins/assay/skills/README.md` currently names; and `assay:adopt` / `assay:author-brief` each
resolve in a fresh session. This primitive also installs a `SessionStart` hook whose blast radius is
wider than the repo you are adopting into — read **§3a** before you install it globally.

### PRIMITIVE: configure-roster

**Applicability — probe, don't assume.** The roster machinery landed in a recent release; this primitive applies from the
first `desk-tools` / `statusgen` release cut that carries it. On an older pin there is no roster
and nothing here has an effect. The build tells you which you have — every roster-reading tool
echoes its effective configuration to **stderr** on every run, including `--version`:

```bash
deskboard --version 2>&1 | grep -q '^assay-config:' && echo "roster present" || echo "pre-roster pin — skip"
```

At the toolkit versions current as this is written it prints `skip`.

Who is trusted, which repos the tools may write to, and which paths force a security review are
**adopter configuration**, deliberately held outside every ref the tools evaluate — so the pull
request being judged cannot widen its own gate. Formats, level (org vs repo), and what UNSET means
are called out with each surface as it appears below; this primitive is only what you
**do**, plus the two ways adopters get it wrong.

**ESCALATE the values, not the file.** Writing the file is autonomous. *Choosing* the blessing
authority is not — `ASSAY_BLESS_LOGIN` is the authorisation half of the trust gate and takes a
single **human** identity with a mandatory numeric id. Hand the operator the list of surfaces and
ask for the logins and ids (`gh api users/<login> --jq .id`; an App's is its **bot user**,
`gh api 'users/<slug>%5Bbot%5D' --jq .id`). Never invent one, and never point it at a shared agent
account: the loader refuses `[bot]`-shaped logins, but a plain shared login is *accepted*, so that
constraint is yours to hold, not the code's.

#### Failure mode 1 — the two transports are not interchangeable

The transport is chosen by what a tool **does**, not where it runs:

| Tools | Transport |
|---|---|
| Every `desk-tools` binary (they all act — post, flip, dispatch, write a work item), and `statusgen --scan-issues` | the config-home file **only** — never the environment, in CI as well as locally |
| `statusgen` reporting modes (`--lint`, `--check`, `--record`) | the Actions variables when genuinely inside a runner; the same config-home **file** the acting tools use everywhere else, including a local `--lint` run |

So **setting the repository/organization variables does nothing for the desk binaries**, and
exporting `ASSAY_*` in a shell does nothing for them either — an acting tool driven by an agent
reading untrusted text is one injected sentence away from an env override, so that route is closed
unconditionally. Every operator needs the local file; a workflow that runs an acting tool has to
*write* that file in a setup step.

Two consequences worth stating before you debug them:

- Your CI `env:` block covers only the **five** variables the reporting modes use.
  `ASSAY_RISK_PATH_TRIGGERS_EXTRA` and `ASSAY_ROSTER_SCHEMA` exist but are not passed there.
- A stray unrecognised `ASSAY_*` key — a one-character typo — collapses the **whole**
  configuration to unconfigured and every tool refuses. Keys outside the `ASSAY_` namespace are
  echoed and ignored, so a co-tenant tool may share the file.
- `ASSAY_HUMAN_LOGIN_MAP` is load-bearing for `statusgen --lint` itself — an absent or
  wrong-shaped roster fails lint (`LINT: FAIL`), not just repo-scoped acting commands. Sequencing
  matters: this primitive lands at Scenario 1 step 6, but step 4 (`add-statusgen-ci`) and step 9
  (`first-board`) both Verify on `--lint` exiting 0. Land the CI `env:` passthrough
  in the same change that adds the roster, or an adopter's CI turns
  red on briefs the roster change never touched.

#### Failure mode 2 — a PARTIAL roster looks like a normal board, not an empty one

An absent roster fails loudly: a `REFUSED` line in the echo, and repo-scoped *acting* commands
(post, flip, dispatch) exit 5. A **partial** one does not — but not because it silently empties the
board. `deskboard nextup`'s board content comes from its configured-roots resolver, which reads
the compiled-in default root-to-path map whenever `DESK_ROOTS` is unset; `ASSAY_ALLOWED_REPOS`
never enters that path. The board is
**byte-identical** whether the roster is full, partial with `ASSAY_ALLOWED_REPOS` blank, or absent
altogether — it looks entirely normal. `ASSAY_ALLOWED_REPOS` gates *acting* commands only (the
exit-5 refusal above), never what the board lists.

That makes this worse than an empty board, not equivalent to one: an adopter told to watch for a
clean empty board will see a populated one and conclude the roster is fine. The configured-check
tests only that there are no validation problems, a blessing authority, and a non-empty login set —
it does **not** consider the repo set — so a roster carrying `ASSAY_BLESS_LOGIN` and
`ASSAY_TRUSTED_LOGINS` but no `ASSAY_ALLOWED_REPOS` reports `configured=true` and prints no
refusal. The gap only surfaces later, when a repo-scoped acting command runs against a repo
`ASSAY_ALLOWED_REPOS` never named. Fill every surface, and read the `assay-config:` echo rather
than either the exit code or the board's contents.

**The board's root set is separate configuration, not roster-derived.** `deskboard nextup`'s
default roots (`defaultRoots`, same file) are compiled to a fixed default set of repos. Configuring the
roster correctly for your own repo does not add it to that map — you also need a `DESK_ROOTS`
entry for it (each entry's repo is validated against your `ASSAY_ALLOWED_REPOS`, but never
populated from it — see below for why one entry is not enough). Skip this and `deskboard nextup`
fails on a fresh adopter checkout with `exit=6`, `root .../assay for repo
medici-finance/assay is not readable` — naming a repo that is not yours. **This
primitive's own Verify below does not use `deskboard nextup` and does not need `DESK_ROOTS` at
all** — it uses `deskboard --version`, which emits the identical `assay-config:` echo. Read on
only once you are ready to stand up the actual cross-repo board.

`DESK_ROOTS` is a **comma-separated list that replaces the defaults outright, not a single
override** — and `deskboard nextup` unconditionally
requires the primary repo among the resolved roots,
because the statusgen version pin (`.assay-versions`) it reads before building anything else lives
there. A `DESK_ROOTS` naming only your
own repo refuses with `exit=5`, `the primary repo is not among the configured
roots` — a **different, unrelated** exit-5, not failure mode 2's roster refusal. This board is
currently exercised only from inside the primary suite (the reference-consumer scenario); true
external self-serve is not yet shipped (the "Target architecture"): today's `DESK_ROOTS` always needs
both entries, e.g. `DESK_ROOTS="<primary>=<path-to-primary>,<owner>/<repo>=<path-to-your-checkout>"`.
And because each `DESK_ROOTS` entry is validated against **your own** `ASSAY_ALLOWED_REPOS` (never
the compiled default set), naming both repos in `DESK_ROOTS` only resolves if
the primary repo is *also* in your `ASSAY_ALLOWED_REPOS` — a real widening of
the same surface that gates what your desk tools may act on, purely to satisfy a version-pin
lookup. **Do not do that just to make `deskboard nextup` run.** This is a product-level gap; the roster template below
deliberately names only your own repo, and this primitive's Verify deliberately does not exercise
`deskboard nextup` for that reason.

**Do this.** Values from the escalation above; one line per surface, no blanks:

```bash
install -d -m 700 ~/.config/assay
umask 077
cat > ~/.config/assay/roster.env <<'EOF'
ASSAY_BLESS_LOGIN=<login>:<numeric-id>
ASSAY_TRUSTED_LOGINS=<login>:<id>
ASSAY_TRUSTED_BOT_SLUGS=reviewer=<slug>:<bot-user-id>
ASSAY_ALLOWED_REPOS=<owner>/<name>:ci:private
ASSAY_HUMAN_LOGIN_MAP=<name>:<login>
EOF
chmod 600 ~/.config/assay/roster.env
```

The mode is enforced, not advisory: a group- or world-writable file **or directory**, or one owned
by another user, is refused with the mode printed. Anything that can write that file names the
accounts the tools trust. Then set the Actions variables for the reporting half, and add the `env:`
passthrough to your `statusgen --lint` step.

**Verify:** every surface is present — the configured-check covers only two of them, so assert all
five yourself:

```bash
for k in ASSAY_BLESS_LOGIN ASSAY_TRUSTED_LOGINS ASSAY_TRUSTED_BOT_SLUGS \
         ASSAY_ALLOWED_REPOS ASSAY_HUMAN_LOGIN_MAP; do
  grep -q "^$k=." ~/.config/assay/roster.env || echo "MISSING: $k"
done
```

No output. Then the tools agree —
`deskboard --version 2>&1 >/dev/null | grep -E 'configured=|role-bindings=|ASSAY_ALLOWED_REPOS='`
shows `configured=true`, a **non-empty** `ASSAY_ALLOWED_REPOS=`, and every `role=` you rely on in
`role-bindings=` (an unbound role refuses the check it exists for). This deliberately uses
`--version`, not `nextup`: it needs no `DESK_ROOTS`, and it does not tempt you into widening your
own `ASSAY_ALLOWED_REPOS` with the maintainer's repo just to get a command to exit 0 (see the caveat
above).

Finally, prove failure mode 2 is live rather than decorative — drop one surface and watch the tools
stay quiet:

```bash
# unset DESK_ROOTS for this demo — it must exercise the COMPILED DEFAULT roots, which is the
# path this failure mode is about. With DESK_ROOTS set, dropping ASSAY_ALLOWED_REPOS instead
# fails DESK_ROOTS' own repo check (exit=5, "outside the fixed set") before reaching the board
# at all — a different, unrelated refusal that does NOT reproduce the silent state below. This
# demo therefore only reproduces from inside the primary suite (sibling primary + adopter
# checkouts, run from the primary root) — see the DESK_ROOTS caveat above.
unset DESK_ROOTS
cp ~/.config/assay/roster.env ~/.config/assay/roster.env.full
grep -v '^ASSAY_ALLOWED_REPOS=' ~/.config/assay/roster.env.full > ~/.config/assay/roster.env
deskboard nextup 2>&1 >/dev/null | grep -E 'configured=|ASSAY_ALLOWED_REPOS='   # configured=true, repos EMPTY
deskboard nextup >/dev/null 2>&1; echo "exit=$?"                                # exit=0 — the silent state
cp ~/.config/assay/roster.env.full ~/.config/assay/roster.env && rm ~/.config/assay/roster.env.full
```

If that printed `configured=true`, `ASSAY_ALLOWED_REPOS=` empty, and `exit=0`, you have seen how
quiet a half-filled roster is: neither the echo's exit code nor `deskboard nextup`'s own output —
which never lists repos in the first place — tells you a surface is missing. The gap only shows up
later, when a repo-scoped acting command runs. If you have `DESK_ROOTS` set for your own day-to-day
board usage outside this primitive, restore it now and re-run the five-surface check before moving
on. (This demo only reproduces from inside the primary suite, per the comment above. On a
fresh, non-maintainer checkout the same steps do not reach this failure mode at all: with `DESK_ROOTS`
unset — as the demo instructs — `deskboard nextup` refuses first with `exit=6`, `root
.../assay for repo medici-finance/assay is not readable`, because the compiled
default roots are the maintainer's paths, not yours — measured, not `exit=5`. Pointing `DESK_ROOTS` at
your own checkout instead hits a *different* refusal the moment `ASSAY_ALLOWED_REPOS` is dropped —
`exit=5`, `... is outside the fixed set` — because every `DESK_ROOTS` entry re-validates against
`ASSAY_ALLOWED_REPOS` (the caveat above). Neither outcome reproduces this failure mode, and neither
is a substitute for reading the `assay-config:` echo.)

### PRIMITIVE: create-labels

The desk tools and the operating skills apply a small set of GitHub **labels** the repo does not
have by default. A label that is absent when a tool reaches for it is not an error the tool
raises — it degrades silently, and the information the label carried is lost (a provenance stamp
is dropped; a `gh pr edit --add-label` fails). So the labels are a **one-off `gh label create` per
repo**, run once at adoption (this primitive is the single home for that list; the operating
skills point here rather than re-listing it). Idempotent: `gh label create` on an existing label
is a harmless "already exists" error you can ignore, or pass `--force` to reconcile the
color/description.

**Three label families:**

- **`review-request`** — the review-dispatch token. The coordinator desk files a `review-request`
  issue when a review is needed and stops; a review session picks it up (the intake work-scanner
  *excludes* `review-request` issues, so they are dispatch tokens, not work items). If the label
  is missing when the desk files, the dispatch token is indistinguishable from a work item.
- **`raised-by:<role>`** — the provenance stamp. `deskfile new --raised-by <role>` records **which
  loop noticed** an issue, feeding the by-desk issue metric. `deskfile` **probes for the label
  first and, if it is absent, files the issue anyway UNSTAMPED with a NOTICE** — the provenance is
  silently dropped, so the metric goes blind unless the labels are pre-created. The roles are the
  roster's role-bindings (`role=` prefixes on `ASSAY_TRUSTED_BOT_SLUGS`), and `deskfile`
  **refuses** any role the roster does not bind, so the label set and the vocabulary are the same:
  `desk`, `worker`, `reviewer`, `verifier`, `issue-loop`, `intake-loop`. An omitted `--raised-by`,
  a missing label, or an unanswered probe all leave the issue **UNKNOWN** — never read UNKNOWN as
  "a human raised it".
- **PR-state pair (`authorization-needed` / `approval-needed`)** — the review-loop's
  who-is-the-PR-waiting-on marker (`pr-review-desk` skill, §PR-state labels). Exactly ONE of the
  two sits on a PR under review, sequentially: `authorization-needed` while the review lane has
  not yet approved at the current head (open findings or no approving verdict); swapped to
  `approval-needed` at the ready-flip (reviewer approved at head + findings met + checks green —
  now waiting on the HUMAN's merge approval); cleared when the PR merges. If the pair is missing,
  `gh pr edit --add-label` fails and the queue is readable only by opening every PR.

```bash
# Review-dispatch token
gh label create review-request --repo <owner/repo> --color d4c5f9 \
  --description "dispatch token: a review session picks this up, runs the skill, posts the verdict"

# Provenance stamps — one per roster role-binding (deskfile new --raised-by <role> applies them)
gh label create raised-by:desk        --repo <owner/repo> --color BFDADC --description "filed by the process desk (the-desk)"
gh label create raised-by:worker      --repo <owner/repo> --color BFDADC --description "filed by a worker (worker-desk)"
gh label create raised-by:reviewer    --repo <owner/repo> --color BFDADC --description "filed by the reviewer desk (pr-review-desk)"
gh label create raised-by:verifier    --repo <owner/repo> --color BFDADC --description "filed by the verify desk (verify-desk)"
gh label create raised-by:issue-loop  --repo <owner/repo> --color BFDADC --description "filed by the intake/issue loop (intake-desk)"
gh label create raised-by:intake-loop --repo <owner/repo> --color BFDADC --description "filed by the intake loop (roster-bound; no skill stamps it yet)"

# PR-state pair — the review-loop keeps exactly ONE of these on each PR it is driving
gh label create authorization-needed --repo <owner/repo> --color FBCA04 \
  --description "review lane has not approved at head — waiting on a reviewer verdict / open findings"
gh label create approval-needed --repo <owner/repo> --color 5319E7 \
  --description "review lane fully approved; flipped ready — waiting on the human's merge approval"
```

> The `raised-by:*` labels track the roster, not this list: if `configure-roster` binds a role
> beyond those above, add the matching `raised-by:<role>` label here for that suite.

**Verify:** `gh label list --repo <owner/repo> --json name --jq '.[].name' | sort` includes
`review-request`, all six `raised-by:*` labels, and the PR-state pair `authorization-needed` +
`approval-needed`; `deskfile new … --raised-by worker` on a test filing reports
`raised-by=raised-by:worker` (stamped), **not** `UNSTAMPED:label-missing`.

### PRIMITIVE: install-main-guard
`cp <parent>/.githooks/pre-commit "$TARGET/.githooks/pre-commit" && chmod +x … && git -C "$TARGET"
config core.hooksPath .githooks`. The hook refuses any `main` commit unless `ASSAY_MAIN_COMMIT_OK=1`
is exported (sanctioned main-writers set it per shell; workers isolate into a worktree + branch). This
is optional-but-recommended client-side hardening; a server-side ruleset is the stronger future form.

**Verify:** on `main`, `git commit --allow-empty -m test` is refused with "refusing commit to 'main'";
with `ASSAY_MAIN_COMMIT_OK=1` it proceeds.

### PRIMITIVE: first-board
`statusgen --root "$TARGET" --lint && statusgen --root "$TARGET"`. Read `STATUS.md` —
roll-up, Next-up, awaiting-verification, unresolved-findings should reflect your streams/registers
(compare `examples/adopter-scaffold/STATUS.md`). Never commit this local `STATUS.md` on a branch.

**Verify:** `test -f "$TARGET/STATUS.md" && grep -q "Next up" "$TARGET/STATUS.md"`; `--lint` exited 0.

### PRIMITIVE: setup-reviewer-app — HUMAN-GATED
Prepare the exact App name + permission toggles (`pull_requests: write`, `contents: read-only` — the
reviewer must not be able to commit — plus **`checks: read`, `statuses: read`, `actions: read`** so
the reviewer can read CI to gate on it), then **escalate**: App creation,
key generation, and installation are the GitHub-admin's. The App's bot login is what makes a verdict
**attributable to an identity the PR author cannot post as** — a placeholder or self-minted stand-in
defeats the entire point. Wait for the recorded App ID / install IDs before wiring anything that
depends on them.

**What the bot login buys, precisely.** No party without the App's private key can make a review
appear to come from that bot — that much holds on **GitHub's side**, and it is real. It does **not**
hold on *yours*: any process that can read the App's PEM can mint its token and post as it, so the
reviewer identity is **attribution plus an audit trail, not authorization**. Keep those two claims
separate in your own docs; conflating them is the single most common way an adopter ends up believing
it has a control it does not have. **§1a** gives the
sentence to use, and why this guide retires the stronger word — do not describe the install in
stronger terms when you report it.

**Adopter caution — `contents: read-only` is the recommendation.** The
read-only reviewer is what keeps *author ≠ approver* mechanically true. A deployment whose reviewer App
carries `contents: write` (so it can flip its own draft PRs) makes
that boundary discipline-held rather than mechanical. Take the read-only default unless you have a reason, and record
the trade if you don't.

**CI-read scopes are separate from `contents` — grant them even on a read-only reviewer.** The
reviewer reads CI check status to gate on it (it never approves or flips a PR over red CI), which
means the App needs **`checks: read`, `statuses: read`, and `actions: read`** — `checks`/`statuses`
for the `commits/<sha>/check-runs` + `statusCheckRollup` reads, `actions` for the failing run's
logs. These are **read-only** and orthogonal to the `contents: read-only` recommendation: a reviewer
that cannot commit still must read CI. Missing any of them surfaces not as a clean error but as an
opaque **HTTP 403** on the CI-rollup read, after which the gate silently degrades. **If you run the
full desk pipeline** (a separate **worker** and/or **desk** App, via `install-desk-tools`), grant the
same three read scopes to those Apps too — they read CI to shepherd red PRs and to detect main-red.
Do **not** grant them to a verifier / inbound-lane App: those roles do not read CI. Like App
creation, the grant is the **GitHub-admin's** act — set the toggles in the App's *Permissions &
events* and re-consent the install; a tool cannot self-grant.

**Verify:** the App appears in the repo's installed-Apps list; a probe PR receives a review authored
by the reviewer-App **bot**, not a user account; and — **for an install that took the `contents:
read-only` default** — the App has no `contents: write`. If you recorded a deviation above, that
last clause does not apply to you: check instead that the deviation is written down where the next
reader of your setup will meet it.

## 3a. What the bundle delivers by itself — and what you still have to write

The install leaves you with working skills and **two things that behave very differently**. Read
both before you tell anyone the adoption is finished.

### The `SessionStart` hook — the METHOD, delivered automatically, in every session

`install-desk-plugin` also installs `plugins/assay/hooks/`, which registers a `SessionStart` hook
injecting the method's portable resident rules — isolation, evidence-not-claims, neutral-dispatch
wording, push policy, attribution, and the rest — as a `systemMessage`. **Do not copy those rules
into your own instruction file.** The hook is their single home; a duplicated copy is the overlap
the precedence rule below calls a bug.

**Scope — this fires in EVERY session, in EVERY project.** `hooks/hooks.json` registers with
`"matcher": "*"`. There is no per-project or per-skill narrowing: once the plugin is installed,
every session you start receives the payload, not only sessions that invoke an `assay:*` skill.
Measured on `main` as this is written: **10 numbered rules, 2432 characters (2444 bytes UTF-8)**
of `systemMessage` added to each of those sessions' context. `jq` must be on `PATH` — the script
fails closed (session starts with no rules) if it is missing. The opt-outs are recorded once, in
[`plugins/assay/hooks/README.md`](../plugins/assay/hooks/README.md): install per-project rather
than globally, or drop `hooks/` from your copy and rely on the `assay:*` skill bodies alone.
Adopt that trade deliberately; do not discover it.

**Verify:** `bash plugins/assay/hooks/inject-resident-rules.sh | jq -r '.systemMessage | length'`
prints the character count above (`| jq -j .systemMessage | wc -c` gives the byte count), and the
numbered rules in that output are the complete set your sessions receive.

### Your own instruction file — the LOCAL BINDINGS, which nothing can deliver for you

The hook carries method. It cannot carry **your repo's bindings**, because it does not know them.
That half goes in your repo's own agent instruction file (`AGENTS.md`, `CLAUDE.md`, or whatever
your harness reads) — a file that is **repo-local**: it documents how to work in *that* repo, and
adopters never clone this one.

#### Running Assay on Cursor — a second first-class harness

Assay is the method, not the harness: the CLI tooling, the `SKILL.md` skills, and the `AGENTS.md`
instructions run on any agent that reads them — the capability table in
[how-assay-works.md](./how-assay-works.md) gives the concrete mapping per harness. **Cursor** is a
supported end-user harness, targeted **headless-first**: the headless `cursor-agent` CLI is the
primary surface (it runs the CLI tooling and desk automation the way any terminal does), the
in-editor agent the secondary one. Because Cursor reads `AGENTS.md` natively and reads the same
`SKILL.md` open standard the skills are written in, most of Assay arrives with no per-harness
translation:

- **Skills** — place the skills tree where Cursor discovers it (`.cursor/skills/` or
  `.agents/skills/`); the same tree Claude Code and Codex use, no per-harness copy.
- **Resident rules + repo bindings** — Cursor reads `AGENTS.md` natively, so the invariants and
  your repo's bindings arrive through the same `AGENTS.md` this section already describes; a
  `.cursor/rules/*.mdc` always-apply rule is the equivalent Cursor-native channel if you prefer it.
- **Isolation** — Cursor's background agents run in isolated git worktrees, so parallel workers
  isolate natively there; on the headless CLI, worker isolation follows the same `git worktree`
  discipline every harness uses, and where the sandbox cannot create a worktree the worker
  **refuses** rather than working in a shared checkout — the isolation floor never degrades.

The one acceptance step is a live smoke run on a Cursor install, the same posture every harness
target holds until it has been exercised end-to-end.

**You start with a stub, not a blank page.** `scaffold-streams` (`statusgen init`) writes a
starting `CLAUDE.md` plus an `AGENTS.md` that points at it. The stub carries the **ten invariants**
and the **CI recipe** — the universal floor, true of every adopter, and the part that must not be
paraphrased repo by repo — and then the bindings **checklist below, unanswered**. It is yours from
the moment it lands: `init` never overwrites an existing file, so an edited copy always survives a
re-run, and a repo that already has an instruction file keeps it untouched. The checklist is the
work; filling it in is step 6 of the post-`init` next-steps.

**Precedence, so drift doesn't decide it**: *the bundled skill is
authoritative on the METHOD; your repo file on THAT REPO's mechanics. An overlap is itself the
bug — resolve it by deleting one side, not by ranking them.*

Write down the bindings a session cannot infer, and nothing else:

- **Streams** — which streams exist, what each owns, and where `docs/streams/` is rooted if not
  the repo root.
- **Pinned tools** — which tools are pinned, in which pin file, and the install/upgrade command
  for this repo (`statusgen` at minimum; a suite pins one tag across every repo).
- **The human gate** — *who*, by account name, and which acts require them here: merge, the
  ready-flip, release tags, App creation. A session must be able to tell a real gate-holder from
  an agent claiming to be one.
- **The risk-path set** — the concrete file and directory globs in *this* repo that force
  `gate: human` when a diff touches them. `risk:` is answered per brief; the paths that make the
  answer `yes` are repo knowledge.
- **Generated / single-writer artifacts** — what agents must never hand-edit or commit on a
  branch (`STATUS.md` at minimum), and which job is the single writer of each.
- **Isolation mechanics here** — whether the checkout is shared, where worktrees go, the branch
  naming convention. The hook says *isolate*; only you can say *where*.
- **The review identity** — the reviewer App's bot login as it appears in this repo, so a worker
  can recognise a genuine verdict from a relayed or forged one.

**What does NOT belong there:** anything true of Assay in general. If you find yourself writing a
rule the hook already injects, that is the overlap — delete it and point at the skill instead.

**Verify:** every bullet above is either answered in your instruction file or explicitly recorded
as not-applicable; and a grep of that file for the hook's own rule text returns nothing.

> *The scaffolded stub is deliberately thin, and the line is drawn where the precedence rule draws
> it. What it ships is the part that is **identical in every adopter** — the ten invariants and the
> CI recipe — so it cannot drift by being re-worded repo by repo. What it does **not** ship is a
> single binding VALUE: the checklist names the categories, you supply the values, and a guide that
> guessed them would be inventing your repo's mechanics. If you run the plugin's `SessionStart`
> hook, its resident rules stay the single home for everything beyond that floor; the stub exists so
> a repo or harness WITHOUT the plugin is not left with a blank page on day one.*

## 4. Decision points: escalate vs. autonomous

**Autonomous** (reversible file/config the agent verifies before moving on): installing the pinned
statusgen release, scaffolding streams + registers, authoring briefs + READMEs, copying the CI workflow,
installing the plugin locally, installing the `.githooks` guard, writing the roster file once the values are in hand, running every
`--lint` / local board gen. **Escalate** (see the quick-reference table above): reviewer App creation,
**choosing the roster values** (blessing authority, trusted logins/Apps, allowed repos), repo
permission model, merge/push/release/ready-flip, git history rewrite, private-repo CI auth.

## 5. Multi-cell topology contract — one `topology.yaml` **per cell**

Past one team, an enterprise running Assay is **10–15 independent cells**: a lead plus its agent
fleet, each accountable for its own repos. This section is the contract that keeps them
independent. It is read before Scenario 2, because the multi-repo mechanics below assume it.

**One topology file per cell — never an enterprise registry.** `topology.yaml` is one cell's
statement about the repos *that cell* deals with. It is not a roster of every cell. A single
central file would route every cell's roster change through a PR against one repo, restoring the
exact coordination point cells exist to remove and making one repo's reviewers the gate on every
other cell's org chart. An adopting cell **copies the shipped `topology.yaml` worked example into
its own tree and edits it in place**; it does not send a PR to the toolkit.

**`relationship: owned | upstream`, stated per repo.** `owned` means this cell is the repo's owner.
`upstream` means this cell reads it and may **file issues** into it, but never modifies it and never
runs jobs against it. Absent reads as `upstream` — least authority, so a file that says nothing
grants nothing. An unrecognised value is a **parse error naming the line**, never a default: there
is no third legal value, so a present-but-unknown one is a typo, and a typo that quietly defaulted
would be indistinguishable from a stated fact.

The load-bearing case is the toolkit itself. Exactly **one** cell states `medici-finance/assay:
owned`; every other cell states it `upstream`. That is why the first edit an adopting cell makes to
the example is flipping that one line.

**Relationship is stated topology. It is NOT write authorisation.** It gates nothing, and nothing
should ever be built that reads it to decide whether a write is permitted — that would move the
write boundary into a tracked file. Write authorisation stays roster configuration
(`ASSAY_ALLOWED_REPOS`), which is already per-cell-shaped: each cell's deployment sets its own.
`deskroster repos` prints the two as **distinct sets** (write-authorisation, intake-read-scope,
stated-topology) precisely so nobody collapses them. A repo can be `owned` and outside the write
boundary; a repo can be inside the write boundary and `upstream`.

**One owner: a mechanism in-file, a convention across cells.** A repo is `owned` by at most one
cell. *Within* a file that is enforced — a duplicate slug is a loader error, so a repo appears once
and carries one relationship — and a file stating `owned` while naming no `cell:` is refused
outright, because an ownership claim with no claimant cannot be audited. *Across* cells it is an
**audit convention**: the toolkit cell reads other cells' files and reports a collision after the
fact. It cannot prevent one, and it is not a gate. Do not design as if it were.

**No global cross-cell registry — an explicit non-goal.** Cross-cell repo references ride GitHub's
already-global `owner/name`, so a registry would add nothing but a coordination point and a second
copy of facts each cell already states. Cross-org work is never fully clean; the contract is
tolerance of that, not a scheme to eliminate it.

**A human in N cells is N independent grants, not N copies of one fact.** Each cell decides its own
membership, so the same person appearing in several cells is several decisions that happen to agree
— derive-or-diff is not violated and there is nothing to single-source. If identity drift ever
needs checking, diff against **GitHub org membership**, never against a central identity file: a
tracked identity file is an org chart in a tree that gets published, which is what
`ASSAY_HUMAN_LOGIN_MAP` being operator configuration already prevents.

**What holds this together mechanically.** The schema is additive and stays `topology-v1`, and each
consuming module's compiled derivation is CI-diffed against the source. In this tree that derivation
is `statusgen/topologyvalues.go`, bound to `topology.yaml` by `TestTopologyValuesMatchSource` — so a
schema change reddens until the derivation demonstrates it too. A module that ships its own
derivation adds its own drift test on the same source; a derivation with no such test is the defect.

### What config alone does NOT cover

Everything above says a cell is *data*. Measured against the tree, that is true of the **schema**
and false of the **runtime**, and an adopting cell needs both halves before it starts.

**True, and executed.** A second cell is expressible as a different instance of this file and
nothing else: a different `cell:`, its own `owned` repos, `medici-finance/assay` flipped to
`upstream`. The schema carries every field a second cell needs, and no code path is added to make
that work — the shipped `topology.yaml` exercises each field at least once so a second instance has
a complete worked reference to copy.

**False for the shipped binaries, and this is the part prose kept quiet.** The tools ship as pinned
standalone binaries that run from an arbitrary working directory, so they do not read
`topology.yaml` at run time — deliberately, because a gate that needs the filesystem fails open
when the filesystem is not the repo. They read a compiled derivation instead. So editing your
`topology.yaml` changes nothing any installed binary does until the derivation is edited and the
binaries are rebuilt.

These are the surfaces frozen at build time — what your cell cannot change with config alone:

| Frozen at build time in | What your cell cannot change with config alone |
|---|---|
| `statusgen/topologyvalues.go` | the system-state and decision-owed label sets, and the default release repo, that `statusgen` reasons about |
| `tools/desk/cmd/issueboard/board.go` | the system-state and decision-owed label sets the board excludes and escalates on |
| `tools/desk/cmd/deskroster/sets.go` | the cell name, per-repo relationship and App roles `deskroster repos` prints |
| `tools/desk/cmd/deskrelease/cut.go` | the default repo `deskrelease` cuts a release from |
| `tools/desk/internal/deskkit/riskpath.go` | per-repo visibility and risk-path triggers, which decide a diff's risk class |
| `tools/desk/internal/deskkit/roots.go` | the repo → local checkout root map the multi-repo board walks |

Every one of those runs against an arbitrary working directory or `--root`, which is why none of
them can read the file at run time.

**So an adopting cell's topology change is four steps, not one.** Edit `topology.yaml`; mirror it
into each consuming module's compiled derivation — in this tree, `statusgen/topologyvalues.go` (the
drift test names the field when you miss one); rebuild; re-pin your own binaries. Only the first is
config. If that is more than you want to carry, the alternative is to accept the toolkit cell's
compiled values for the surfaces above and use `topology.yaml` as documentation — which is a
legitimate choice, but make it knowingly rather than discovering it when an edit has no effect.

**What is genuinely per-cell with no rebuild** is the operator configuration these tools read from
the environment — `ASSAY_ALLOWED_REPOS`, `ASSAY_SCAN_REPOS`, `ASSAY_TRUSTED_LOGINS`,
`ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_HUMAN_LOGIN_MAP` (`tools/desk/internal/deskkit/rosterconfig.go`).
That is where the write boundary lives, and it was already per-cell-shaped. The split is worth
stating plainly: **authorisation is config; stated topology is compiled.**

---

# SCENARIO 1 — Green-field project(s)

**When to use.** The repo is brand-new (or a small set of brand-new repos) with no history worth
preserving. You are standing Assay up from zero and want the full loop firing on day one. If the repo
already carries streams, a legacy board, or in-flight work, use **Scenario 2** instead.

### Ordered steps (zero → exercised end-to-end)

1. **Initialize the repo.** `git init && git checkout -b main && git commit --allow-empty -m "chore: initial commit"`, then create the remote and push `main` (human does remote-create if org policy requires — see Step 7). **Verify:** `git rev-parse --abbrev-ref HEAD` = `main`; `git remote -v` shows `origin`.
2. **Run `install-statusgen`.** Green-field default: **channel E — the pinned release binary**. Commit `.assay-versions`, install, verify the sha256. The old "green-field default: vendor" advice is **retired**: a new repo has no update-propagation cost *on day one*, which is exactly how a fork starts, and every day after that it drifts (a frozen vendored fork is that story, still gating a repo's PRs). Starting pinned costs one file. **Verify:** `statusgen --version` prints the pinned tag; the pin line for your platform exists in `.assay-versions`.
3. **Run `scaffold-registers` + `scaffold-streams`.** Create `docs/streams/{FINDINGS,INTAKE,RETRO}.md` and **exactly one** stream dir. **Verify:** `ls docs/streams/` shows the three registers + one stream; `yq eval '.' docs/streams/<stream>/README.md >/dev/null`.
4. **Run `add-statusgen-ci`.** Confirm `paths:` match your streams and the bootstrap-safe guard is present. **Verify:** both a `lint` (pull_request) and a `regen` (push→main) job exist; the porcelain guard is present.
5. **Run `install-desk-plugin` + `install-main-guard`, then write the local bindings (§3a).** The plugin's `SessionStart` hook fires in every session in every project — take that trade knowingly, per **§3a**. Then fill your repo's own instruction file with the §3a checklist; the hook delivers the method, not your streams / pins / gate-holder / risk paths. **Verify:** `assay:adopt` and `assay:author-brief` resolve; `git config core.hooksPath` returns the hooks path; every §3a bullet except the review identity (which step 7 establishes) is answered or recorded as not-applicable.
6. **Run `configure-roster`** — skip it only if the primitive's probe says your pin predates the roster. **ESCALATE** the values (blessing authority, trusted logins/Apps, allowed repos), then write `~/.config/assay/roster.env` and set the Actions variables. **Verify:** all five surfaces present; `configured=true` with a **non-empty** `ASSAY_ALLOWED_REPOS=` and every `role=` you rely on bound — `configured=true` with an empty repo set is the silent half-configured state, not a pass.
7. **Run `setup-reviewer-app` — HUMAN-GATED.** **ESCALATE:** ask the operator to install the reviewer App on the new repo (and create the remote / grant access for a private repo). Report the exact repo + App name. Do not proceed to Step 10's ready-flip until the App can post a review.
8. **Seed the FIRST stream + first brief.** Copy `docs/brief-template.md` → `brief-01-<slug>.md`; fill every field; add the README table row. Keep it **effort: S, wave: 0, gate: model** — trivial on purpose. A good seed nails four things (per `docs/brief-rules.md`): **why** (`sources:` names a real origin), **gate** (derived from the four `risk` answers; all `no` ⇒ `model`), **shared-value handling** (none, deliberately), **Verify** (literal commands runnable by a non-implementer). **Verify:** `yq eval '.schema' …brief-01-*.md` = `brief-v1`; the README row matches the frontmatter.
9. **Run `first-board`.** **Verify:** `--lint` exits 0; local `STATUS.md` lists brief `01` in **Next up**; `git status` does NOT stage `STATUS.md` on your branch.
10. **Open the first draft PR through the review loop.** Branch from fresh `origin/main` in an owned worktree, commit streams + brief + statusgen + CI + hooks, push, open a **draft** PR. **Verify:** `gh pr view <N> --json state,statusCheckRollup,reviews` shows lint green, no `STATUS.md` in the diff (`gh pr diff <N> --name-only | grep -qx STATUS.md && echo LEAK || echo clean`), and a reviewer-App review present. **ESCALATE:** the ready-flip is the desk's call and **merge is human-gated** — do not self-merge.

### The "hello-world" loop (proves the install)
Walk brief `01` through the full lifecycle once so every part fires exactly once:
1. **todo → in-progress** — a worker claims it off Next-up (not "next in stream").
2. **in-progress → implemented** — the worker does the task, runs the Verify table, fills **Evidence** with its own run, sets `implemented`, and STOPS (an implementer never sets `verified`).
3. **PR review + merge** — the draft PR carries it; the reviewer App reviews; findings get follow-up commits; the desk flips ready; **the human merges** (merging does NOT advance status past `implemented`).
4. **implemented → verified** — a **non-implementer** re-runs the Verify table on merged `main`, fills a dated Verified/Evidence entry, sets `verified`.
5. **verified → done** — the recorded review verdict is attached (model sign-off for this `gate: model` seed; a `gate: human` brief needs a `human:<name>` entry).

**Verify (whole loop):** after main's CI regenerates, committed `STATUS.md` shows brief `01` `done` with dated Verified + Reviewed cells; the regen commit carries `[skip-status-regen]`; no branch ever committed `STATUS.md`. If all hold, the desks, board, reviewer App, and human-merge gate each fired once — install proven.

### Green-field-specific choices
- **Pin statusgen from day one** (channel E). Do not vendor "just for now" — the update-propagation cost you are deferring is what a frozen vendored fork becomes.
- **Start with ONE stream** (an empty stream is just staleness the board will nag about).
- **Don't add batch-fanout yet** — fan-out earns its keep only once Next-up regularly returns more than one session should carry. Day one has one brief; run it inline.

### Multi-new-repo note
Several green-field repos at once: run the steps **per repo** (each gets its own `.assay-versions` pin,
streams, CI, and human-gated reviewer-App install — one pinned release, N pins, never N copies of the source). Only the **cross-repo board + dispatch** differs —
forward-reference **Scenario 2's** multi-repo mechanics rather than inventing them here.

---

# SCENARIO 2 — Existing suite of projects (multi-repo)

**When to use.** Assay across **two or more repos that already have code, history, issues, and CI** —
a product suite dispatched, reviewed, and rolled up as one suite. Single fresh repo → Scenario 1.

The core problem is four asymmetries that appear once streams live in more than one repo: **per-repo
boards** (each repo generates its own Next-up; a repo with no board is invisible to dispatch);
**cross-repo dispatch** (the dispatcher must read *every* board-bearing repo, or half the fleet looks
staffed but is never picked up — a real defect where review was multi-repo but dispatch read one repo);
**one review identity fleet-wide** (a single reviewer App across all repos, not N per-repo bots); and
**an aggregate view** (one roll-up answering "what's next, anywhere"). **Do not** vendor N copies of the
tooling — one methodology source, one review identity, each repo contributing its own board.

### Steps

1. **Prove the single-repo loop on the highest-traffic repo first.** Run Scenario 1's primitives there end-to-end; confirm one brief goes `todo → … → done` with a real reviewer-App verdict **before** touching another repo. **Verify:** the primary repo has a `STATUS.md` on `main` with a populated Next-up; one brief reached `verified`/`done` with a bot review. **ESCALATE** if the board never appears on `main` — that's the bootstrap-guard bug; fix it before fanning out.
2. **Scaffold streams in each additional repo** (core primitives, per repo, in that repo's **own owned worktree** — isolation is per-repo). Run `scaffold-streams`, `install-main-guard`, `install-desk-plugin`. Do **not** run `install-statusgen` here (step 3 does it suite-wide), and do **not** re-run `configure-roster` per repo — the roster is **one file per operator**, not a per-repo artifact; adding a repo to the suite means adding it to `ASSAY_ALLOWED_REPOS`, and a repo missing from that value is refused by every desk tool. Write **each** repo's own local bindings (**§3a**): the method is shared suite-wide via one bundle and one hook, but streams, risk paths, single-writer artifacts and isolation mechanics are per-repo, and a session working in repo B cannot read repo A's instruction file. **Verify:** each repo has `docs/streams/` with a valid `brief-v1` stream README; `core.hooksPath` resolves; each repo's instruction file answers the §3a checklist.
3. **Install the pinned statusgen release into each repo (do not vendor N copies).** The tool comes from **one place — `assay/statusgen`** — so copies can't drift. Run `install-statusgen` per repo: **channel E**, a `.assay-versions` pin plus a sha256-verified release binary. Every repo in the suite should pin the **same tag**, and a suite-wide upgrade is then N one-line pin bumps you can see in a diff. Prior revisions of this doc recommended **D — CI fetch-and-run at a pinned ref**; that is now the fallback for a runner that cannot download release assets, because a pinned ref pins *source* and rebuilds it per run, so nothing is ever hash-checked.  Then run `add-statusgen-ci` per repo. **Verify:** `statusgen --root <adopter>` writes `<adopter>/STATUS.md`; every repo's `.assay-versions` names the same `statusgen` tag (`grep -h '^statusgen ' */.assay-versions | sort -u | wc -l` → 1); no repo carries a `statusgen/` source tree; after first merge, `STATUS.md` appears on `main`. **ESCALATE** private-repo CI auth (the release-download token) to the admin.
4. **Install ONE reviewer GitHub App across ALL repos** (`setup-reviewer-app`) — dual-installed on the account and the org with `repository_selection: all` so new repos are auto-covered; the token minter picks the install by the `owner/repo` slug. **Verify:** `gh api /app/installations` shows both installs with `all`; the App can post a review in a spot-checked repo per account; and — **for an install that took the `contents: read-only` default** — it has no `contents: write`. (Recorded deviations exempt the last clause — a reviewer deliberately granted `contents: write` will not pass it.) **ESCALATE** — App creation is the admin's alone.
5. **Enable multi-repo dispatch in `batch-fanout`**: define the board-bearing **repo set in ONE place** in the skill; loop it, regenerating each repo's board to scratch with **that repo's** statusgen command and extracting its Next-up (skip a non-dispatchable repo **with a logged note**); merge into one **repo-tagged** batch preserving each board's per-stream cap + ordering; make every worker-dispatch carry the target repo + "isolate in an owned worktree of that repo, open the draft PR in that repo"; reconcile the PR-scan set with the board set. **Verify:** the skill names the repo set + both statusgen forms; a dry-run surfaces a pick whose repo is **not** the primary; worker-dispatch carries per-repo isolation.
6. **Aggregate / roll-up view (direction).** End state = a master board across the suite. Today the aggregate is the merged dispatch batch (step 5); the standalone master aggregator is a later direction. Keep the board-bearing repo set canonical in one place so a future aggregator has a single source. Don't block adoption on it.
7. **Fan out to the rest of the suite** — repeat steps 2–4 per repo, adding each to the board-bearing set (one line) **as it becomes dispatchable**. **Verify:** every intended repo has a `STATUS.md` on `main`; the App is an available reviewer in each; a full dispatch surfaces picks from more than one repo.

### Migration realities
- **Don't fight existing CI** — add `statusgen.yml` as a *new*, additive, path-filtered workflow; it gates only STATUS.md-on-a-branch and brief-source errors, never the repo's build/test.
- **Existing issues/PRs are untouched** — Assay adds a board + review identity; it doesn't migrate or renumber prior issues.
- **The bootstrap-safe board guard is mandatory** in every adopter workflow (`git status --porcelain -- STATUS.md`), or a repo with no `STATUS.md` yet never generates its first board. **Verify:** `grep -F 'git status --porcelain -- STATUS.md' .github/workflows/statusgen.yml` matches in each repo.
- **ESCALATE** if a repo's statusgen/streams live only on an unmerged branch (as an in-flight extraction's can) — coordinate with that branch's owner; do not edit another repo's in-flight branch to force it dispatchable.

*Source note: the earlier A–D option set (default D) is **superseded** by channel E — the sha256-pinned release binary. E is the channel to run; A–D are kept in the table above only so a repo already on one of them knows what to migrate off.*

---

# SCENARIO 3 — Carve a subsystem out of an existing project

Extract one part of an existing project — a service, module, or package that has grown its own
lifecycle — into its own board-bearing unit: **its own repo** (a "product cell"), or **its own
stream-set in place** as a staging step toward a later repo split.

**This is the highest-risk path.** Two steps fail loudly if rushed and quietly if skipped:
**enumerating the subsystem's consumers before moving anything** (a wrong boundary strands callers),
and **preserving git history across the move** (a history-free copy discards the audit trail the
methodology exists to keep). Slow down on both; everything else is mechanical. Real precedent to read:
an example-service carve-out from a monorepo into its own repo (run as a
dependency-ordered extraction stream whose briefs *are* the extraction), and the toolkit's own
extraction (its README records a history-free copy as a **known imperfection**, not a template).

### 1. Decide whether to carve out at all
Extract only when the subsystem has **independent lifecycle** — its own release cadence, its own
owners, a backlog that keeps colliding with the parent's board. If it's just a big directory that
ships with everything else, give it a stream, not a repo. **When unsure, do 3b (own stream-set) first**
— a stream-set is reversible; a repo split with rewritten history is not.

### 2. Decide the boundary FIRST — enumerate consumers before touching a file
Read-only investigation, written down before any move:
1. **Enumerate the moving set** — every path that moves (`git ls-files -- <subsystem paths> > /tmp/carve-moving.txt`). Too wide drags shared code out; too narrow leaves it non-compiling.
2. **Enumerate consumers — who imports it.** Grep the whole repo (and sibling suite repos) more than one way: import paths, package names, symbols, k8s manifest refs, CI job names. For each hit decide **move-with / stay-and-consume-across-the-boundary / sever-first**. A consumer that stays needs a seam (an interface it calls instead of reaching in).
3. **Decide stay vs. move explicitly**, per path — the contract the move briefs execute against.

**Verify (boundary complete):** the moving-set + consumer lists account for **every** grep hit — no reference unclassified. Record both lists in the tracking brief (§6) before proceeding. An un-triaged import is a strand waiting to happen.

### 3. Stand up the target — pick one shape
**3a. Own repo (product-cell shape).**
1. **Create the repo + grant access — HUMAN-GATED. STOP and escalate.** Hand the human: the slug/org, the Go module path (match the slug so no import rename is needed), the license, and the admin-grant request. Do not run `gh repo create` or set permissions. Wait for the repo + access confirmation.
2. **Move the code preserving history — HUMAN-GATED rewrite; never destructive-unattended.** Use `git filter-repo` on a **throwaway fresh clone** (never the working checkout): one `--path` per moving-set entry, `--path-rename` to the destination layout. Push the rewritten history to the empty repo. **Do NOT delete the source from the parent yet** — deletion is a separate, later, dependency-gated brief. **Verify (history survived):** `git log --oneline -- <a moved path>` in the new repo shows real historical commits, not one "initial import" squash; `git log --follow` crosses the rename. One commit ⇒ history dropped; stop and redo.
3. **Run the core primitives** in the new repo (`scaffold-streams`, `install-statusgen`, `add-statusgen-ci`, `install-desk-plugin`, `setup-reviewer-app`, `install-main-guard`). **Verify:** `--lint` = 0; CI single-writes `STATUS.md`; the App is installed.
4. **Seed its streams from the real backlog** (§4).

**3b. Own stream-set in place (intermediate shape).** `docs/streams/<subsystem>/` with a stream README
via `scaffold-streams` — no new statusgen/CI/App (the parent already has them). Optionally carry an
`external:` frontmatter pointer to a reserved target repo (a carved-out stream does this). Treat as a
**staging state**, not the destination. **Verify:** the new stream appears in the parent's regenerated
`STATUS.md`; `--lint` stays green.

### 4. Backfill the registers — honestly
Capture reality: briefs at `todo`/`in-progress` for not-started/underway work; known defects as
`FINDINGS.md` entries (with `Affects:`); raw ideas as `INTAKE.md`. **Do NOT mark anything `verified`/
`done` that a non-implementer didn't independently verify on the carved-out unit** — backfilled status
is a claim, not evidence; carry it at `implemented` at most. **Verify:** every backfilled status matches
an artifact; `--lint` passes.

### 5. Wire it into the suite (own-repo shape)
Add the new repo to the multi-repo board-bearing set + shared reviewer App exactly as **Scenario 2**
describes. **Verify (defer to Scenario 2's checks):** the new repo is in the review desk's repo set; a
test PR draws a reviewer-App review.

### 6. Track the migration AS Assay work — the move is not exempt
Author the extraction as a stream of dependency-ordered move briefs run through the normal
branch → draft-PR → review → verify loop (structure: scaffold-seam → move-core → move-adapters → release-pipeline):
1. **Scaffold + seam** — land the destination layout + interface boundary (with a CI import-boundary guard) *before* any code moves.
2. **Move the core** — copy-and-**adapt** (imports/package names change, logic doesn't), gated by a **parity harness** (same fixtures → byte-identical output from moved vs. original); any needed behavior change is `NEEDS_CONTEXT`, never a silent fix.
3. **Move the platform/adapter layers** behind the seam.
4. **Release pipeline** — produce the versioned artifact the consumer depends on.
5. **Consume + delete** — cut parent consumers over to the released artifact, then a **later** brief that `depends:` on the release deletes the now-vendored source. Deletion is last and gated so a wrong boundary is recoverable up to that point.

**Verify:** each move brief has an executable Verify table (tests green behind the seam, parity byte-identical, CI green at the landed SHA); the deletion brief `depends:` on the release brief; `--lint` = 0 on both repos.

**The two things that go wrong:** a boundary that stranded an un-enumerated consumer (§2), and a squash that discarded history (§3a.2). Both are cheap to prevent up front and expensive to discover after deletion.

## 8. Upgrading after install — the `assay:upgrade-assay` skill

Install and upgrade are **one story with one mechanism**: install writes the `.assay-versions` pin
(the umbrella line plus the per-artifact tags), and upgrade moves it. The single supported upgrade
path is the **`assay:upgrade-assay`** skill (surfaced once the plugin is installed). Do **not**
hand-edit pins against a release page — that is precisely the drift the pin/umbrella model removes.
The distribution model this section implements is [`distribution.md`](distribution.md).

**What it does.** `assay:upgrade-assay` moves an adopter to the **latest stable** umbrella (the
highest published bare `vX.Y.Z` release) or to a **named** umbrella version, dry-run-first:

1. **Preview** — it asks the version marker (`deskversion`) what umbrella you are on, resolves the
   target, computes the artifact-version delta, selects the migrations that would run, and shows the
   **release notes** for the span — writing nothing.
2. **Apply on your confirmation** — it re-pins `.assay-versions` to the target umbrella and its
   artifact tags, and runs the migrations idempotently.
3. **Re-resolve** — it prints the `/plugin marketplace add …@<version>` + `/plugin update` commands
   for you to run; it never re-points the marketplace or edits the install cache itself.

**What it refuses** (each a distinct exit): an **undetermined** current version (exit 6 — it never
assumes latest), **inconsistent** records (exit 5 — it names the disagreeing pair), a
**per-artifact tag** where an umbrella version was required (exit 7 — the verb moves the whole
umbrella, e.g. `--to v0.13.0`, never `statusgen/v0.13.0`), and a target that names **no published
release** (exit 8 — never a nearest-match guess).

**There is no rollback.** The platform has no downgrade verb — `/plugin` updates but cannot
downgrade, and cached prior versions are pruned after about 14 days. Moving to an older named
version re-points and re-resolves; it is **not a rollback**, and an artifact older than roughly two
weeks may be unavailable. With no rollback to fall back on, `assay:upgrade-assay` refuses cleanly
rather than pretending otherwise.

**Verify:** after an upgrade, `deskversion --root <repo>` reports **known** at the new umbrella and
`deskpins --check` still passes.
