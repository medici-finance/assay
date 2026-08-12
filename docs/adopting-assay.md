# Adopting Assay — install runbook for an LLM agent

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
   `install-desk-plugin`, `configure-roster`, `install-main-guard`, `first-board`,
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

## Human-gate quick reference (never autonomous)

| Act | Why it's human-gated | Where |
|-----|----------------------|-------|
| Reviewer GitHub App creation + install | Mints an irreversible public identity + grants repo perms; the bot login is what makes a review **attributable to an identity the author cannot post as** — attribution, not authorization (§1a) | CORE `setup-reviewer-app`, §1a |
| Choosing the roster VALUES — blessing authority, trusted logins, allowed repos | Names who the tools obey; `ASSAY_BLESS_LOGIN` is the authorisation half of the trust gate and must be a human. Writing the file is autonomous; picking the identities is not | CORE `configure-roster` |
| Repo creation + admin/collaborator grants | Irreversible public act; permission model is the distribution boundary | Scenario 3 §3a |
| Merge to `main` / `git push origin main` / release tag / first ready-flip | The deploy/merge gate; agents open **draft** PRs only | CORE §4, every scenario |
| git history rewrite (`filter-repo`) for a carve-out | Destructive + irreversible; a squash loses the audit trail | Scenario 3 §3a |
| Private-repo CI auth (`GOPRIVATE`, cross-repo checkout token) | Credential provisioning | Scenario 2 §3 |

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
public bundle ships the two portable, domain-neutral skills (`assay:adopt`, `assay:author-brief`); an
adopting team runs the remaining loop roles as its own project-local skills (see `install-desk-plugin`). There
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
the advisory merge gate into an enforced one. None of these is done. Until they are, the honest
sentence above is the whole of the claim.

## 2. Component inventory

| Component | What it is | Comes from (`assay`) | Lands in target repo |
|---|---|---|---|
| **statusgen** | Go module (stdlib + `yaml.v3`): generates `STATUS.md`, computes Next-up, lints sources | a **release binary** built from `statusgen/` | **no source in your repo** — a `.assay-versions` pin plus the installed binary on `PATH` |
| **statusgen CI** | Two-half workflow: PR-side `--lint`, main-side regenerate-and-commit | **not shipped** — house-specific CI you author yourself to the shape in §3 `add-statusgen-ci` | `.github/workflows/statusgen.yml` (you write it) |
| **streams layout + templates** | `docs/streams/<stream>/README.md` (frontmatter + brief table) and brief-v1 files | `docs/brief-template.md`, `docs/brief-rules.md`, `examples/adopter-scaffold/` | `docs/streams/<stream>/` |
| **registers** | Append-only FINDINGS / INTAKE / RETRO logs | `docs/registers.md`; scaffold's `FINDINGS.md` / `INTAKE.md` | `docs/streams/{FINDINGS,INTAKE,RETRO}.md` |
| **methodology skills / plugin** | The two portable methodology skills (`assay:adopt`, `assay:author-brief`), namespaced `assay:<name>` | `.claude-plugin/marketplace.json`, `plugins/assay/` — see `install-desk-plugin` | installed via `/plugin`, cached under `~/.claude` |
| **reviewer GitHub App** | The separate review identity (§1a) — attribution, not authorization; `pull_requests: write` with **no** `contents: write` is the *recommendation* (§3 `setup-reviewer-app`) | CORE `setup-reviewer-app` (runbook) | GitHub org/account settings — **not a repo file** |
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
| **C** | **Published Go module** — `go run github.com/medici-finance/assay/statusgen@<ver> --root .` | Rebuilds from source per run, so it verifies nothing by hash. |
| **D** | **CI fetch-and-run at a pinned ref** — check out `assay` at a pinned ref and `go run ./statusgen` | Was the recommendation for suites. Superseded by **E**: a pinned ref pins *source*, not the artifact, so the thing that runs is rebuilt each time and never hash-checked. Keep only where a runner cannot download release assets. |

Whichever you pick, **record it** — the per-repo invocation differs by where the tool is rooted,
and a suite that mixes channels is the inversion this doc exists to stop.

### PRIMITIVE: scaffold-streams
Create the streams tree and one stream README with the required frontmatter + brief table (copy the
shape from `examples/adopter-scaffold/docs/streams/example-service/README.md`). Author the first
brief by copying `docs/brief-template.md` into `docs/streams/<stream>/brief-01-<slug>.md` and filling
**every** frontmatter field — empty `sources:` or a missing `risk:` answer is a gap, not a shortcut.
`gate` is *derived* (any `risk: yes` ⇒ `human`), never chosen.

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

> **BOOTSTRAP-SAFE GUARD — required.** The regen step must guard on `git status --porcelain -- STATUS.md`,
> **not** `git diff --quiet -- STATUS.md`. `git diff` only sees *tracked* files, so on a repo with no
> `STATUS.md` yet the guard mis-fires ("nothing to commit") and the first board is **never created**.
> Make sure the workflow you author uses the `git status --porcelain` form here, **not**
> `git diff --quiet -- STATUS.md`.

**Verify:** `grep -q 'skip-status-regen' …/statusgen.yml && grep -q 'STATUS.md is generated' …/statusgen.yml`; and `grep -F 'git status --porcelain -- STATUS.md' …/statusgen.yml` matches (bootstrap-safe). After first push to main, `STATUS.md` appears in one `[skip-status-regen]` commit; a PR editing `STATUS.md` fails lint.

### PRIMITIVE: install-desk-plugin
Install the methodology plugin so the skills surface namespaced (`assay:<name>`):
`/plugin marketplace add medici-finance/assay` then `/plugin install assay@assay`.

> **What the public bundle ships — two skills.** `plugins/assay/skills/` ships the two portable,
> domain-neutral methodology skills: **`adopt`** (this install runbook, as a skill) and
> **`author-brief`** (the brief-authoring methodology). That is the authoritative set —
> `plugins/assay/skills/README.md` names exactly those two, and `ls plugins/assay/skills/*/SKILL.md`
> in a fresh checkout returns exactly them.
>
> **The loop roles are not shipped skills.** Fan-out dispatch, PR review, post-merge verification,
> and desk coordination are methodology *roles* the scenarios below describe — but an adopting team runs
> them as its **own project-local skills** in the repo's `.claude/skills/`, not from this bundle.
> Author them against the naming convention in `docs/skill-naming.md`; the method they follow is in
> this guide and the two shipped skills, not a copied skill body. Do **not** expect an
> `assay:pr-review-desk`, `assay:verify-desk`, `assay:batch-fanout`, or `assay:the-desk` to resolve
> from the public bundle — they are deliberately not in it.

**Verify:** `/plugin` lists `assay` installed; `ls plugins/assay/skills/` enumerates exactly
`adopt` and `author-brief`; and `assay:adopt` / `assay:author-brief` each resolve in a fresh
session. This primitive also installs a `SessionStart` hook whose blast radius is wider than the
repo you are adopting into — read **§3a** before you install it globally.

### PRIMITIVE: configure-roster

**Applicability — probe, don't assume.** The roster machinery landed in a recent release; this primitive applies from the
first `desk-tools` / `statusgen` release cut that carries it. On an older pin there is no roster
and nothing here has an effect. The build tells you which you have — every roster-reading tool
echoes its effective configuration to **stderr** on every run, including `--version`:

```bash
deskboard --version 2>&1 | grep -q '^assay-config:' && echo "roster present" || echo "pre-roster pin — skip"
```

At `desk-tools/v0.2.1` + `statusgen/v0.8.0` (the newest tags as this is written) it prints `skip`.

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
default roots (`defaultRoots`, same file) are compiled to the maintainer's own repos. Configuring the
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
reviewer must not be able to commit), then **escalate**: App creation,
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

> *This guide gives the shape rather than a copyable template on purpose. A shipped template would
> be a method-owned product artifact carrying concrete rule text into every adopter repo — which
> is the duplication the precedence rule forbids, plus a second thing to keep in sync. The
> checklist names the categories; you supply the values.*

## 4. Decision points: escalate vs. autonomous

**Autonomous** (reversible file/config the agent verifies before moving on): installing the pinned
statusgen release, scaffolding streams + registers, authoring briefs + READMEs, copying the CI workflow,
installing the plugin locally, installing the `.githooks` guard, writing the roster file once the values are in hand, running every
`--lint` / local board gen. **Escalate** (see the quick-reference table above): reviewer App creation,
**choosing the roster values** (blessing authority, trusted logins/Apps, allowed repos), repo
permission model, merge/push/release/ready-flip, git history rewrite, private-repo CI auth.

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
