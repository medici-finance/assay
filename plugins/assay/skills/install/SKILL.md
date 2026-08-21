---
name: install
description: >-
  Turnkey installer for the Assay methodology — invoke it and it self-installs the whole project
  setup into the target repo. Use right after adding the plugin from the marketplace, when the ask
  is "install Assay here", "set Assay up in this repo", "run the turnkey install", "bootstrap the
  board/desks", or a cold adopter's first boot. It DETECTS the target repo, runs `statusgen init`
  for the scaffold, acquires + sha256-verifies a version-PINNED statusgen binary from the umbrella
  releases (never floating/`latest`), wires CI + the main-guard, and PROVES the install
  (`--lint` == 0, `--version` prints the pinned tag). It is idempotent and REFUSES-not-clobbers an
  already-adopted repo, opens DRAFT PRs only, and escalates every never-autonomous step (reviewer
  App, repo/permission grants, merge/push/tag, private-repo CI auth) to a human. Unix-first
  (mac/linux); Windows binary acquisition is a named fast-follow. For the step-by-step PRIMITIVE
  detail and the scenario routing it delegates to the `adopt` skill + docs/adopting-assay.md.
---

# Install Assay — turnkey installer

You are the **turnkey** entry to Assay. Where `assay:adopt` is the *runbook* (it routes you to a
scenario and holds the PRIMITIVEs and human-gates so a human or agent can hand-walk the install),
`assay:install` is the *installer*: invoke it and it self-installs the whole project setup, driving
each step itself and stopping only at the never-autonomous escalation points.

The orchestration logic here is **Claude-Code-driven and OS-agnostic**. The single OS-specific
piece is the statusgen *binary acquisition* in step 3 — Unix (mac/linux) today, Windows a named
fast-follow (see **Scope**). Everything else runs identically on every platform.

This skill does **not** fork the install steps. It DELEGATES the PRIMITIVE detail — exact commands,
per-step Verify, the failure modes — to **`assay:adopt`** and the full runbook at
**`docs/adopting-assay.md`** in your `assay` checkout. Read those for the ground truth; this skill
is the turnkey orchestration over them.

## Before anything: is this even installable here?

Run the **refuse-not-clobber** check FIRST, before writing a single file. Re-invoking the installer
must be safe, and it must never overwrite a live adoption. Classify the target:

- **Already adopted** — a valid streams tree (`docs/streams/` with at least one stream README) AND
  a pinned `.assay-versions` at the repo root already exist. **REFUSE.** Do not scaffold, do not
  re-pin, do not touch the board or the CI workflow. Report the existing state (the streams found,
  the pinned statusgen tag) and stop. This refusal is a **first-class outcome, not an error** — say
  "this repo is already adopted; nothing to install" and name what you saw, cleanly.
- **Partially installed** — some but not all of the setup is present (e.g. a scaffold but no pin, a
  pin but no CI workflow). This is the **idempotent** path: advance ONLY the unmet steps below,
  leaving every already-correct artifact untouched. Never re-scaffold over existing streams; never
  re-pin an already-correct `.assay-versions` line in place.
- **Fresh** — none of the above. Run the full flow.

When in doubt about whether an artifact is "already correct", treat it as present and leave it
alone — the installer's bias is to refuse and report, never to clobber and hope.

## The turnkey flow

### 1. Detect + confirm the target repo
Identify the repo the adopter is standing Assay up in (the current repo root unless the human names
another). **Confirm with the human before writing anything** — name the absolute target path and the
detected platform (`os-arch`, e.g. `darwin-arm64`) back to them. Do not scaffold a repo you only
inferred.

### 2. Scaffold — use `statusgen init`, never reinvent it
Run **`statusgen init --root <target>`** (the umbrella `statusgen` subcommand). It already emits a
**lint-clean tree** — `docs/streams/` plus the registers — AND a bootstrap-safe CI workflow. The
installer USES this; it does not hand-author a second copy of the scaffold. If `statusgen init` is
not yet available (an older pinned binary predates the subcommand), fall back to the `adopt`
runbook's `scaffold-streams` / `scaffold-registers` PRIMITIVEs and say you did so — do not fake the
scaffold.

### 3. Acquire + pin + verify statusgen (Unix mechanism)
Version-PINNED, **sha256-verified**, never floating and never `latest`. This is the channel-E
`.assay-versions` mechanism the `adopt` runbook's `install-statusgen` PRIMITIVE defines; the
installer wraps it:

1. **Resolve the paired tag.** Read the running plugin's version from
   `plugins/assay/.claude-plugin/plugin.json`, then resolve the statusgen tag it was built and
   tested against from the shipped pairing manifest **`plugins/assay/paired-versions.yaml`** (see
   **The plugin↔statusgen pairing** below). The tag comes from the manifest — you do NOT pick
   "the newest release" and you do NOT resolve `latest`.
2. **Detect the full platform** — os *and* arch: `plat="$(uname -s | tr A-Z a-z)-$(uname -m | sed
   's/^x86_64$/amd64/; s/^aarch64$/arm64/')"`. Match the FULL platform; a `darwin-amd64` pin must
   not satisfy a `darwin-arm64` host.
3. **Write/confirm the pin line** in the target's root `.assay-versions`, in channel-E form
   `statusgen-<platform> <tag> <sha256>`, taking the tag and the per-platform sha256 from the
   pairing manifest. Re-pin (never edit in place silently) so an upgrade shows in a diff. If the
   line is already present and correct, leave it.
4. **Download the pinned tag**: `gh release download <tag> --repo <umbrella-releases> --pattern
   "statusgen-<platform>"`. The `<umbrella-releases>` value is the release home named in
   `paired-versions.yaml` — resolve it from there, do not hardcode it in prose.
5. **Verify the hash**: `shasum -a 256 statusgen-<platform>` and compare to the pinned digest.
   **REFUSE on mismatch** — a hash mismatch is a hard stop, not a warning; do not install a binary
   whose bytes do not match the pin. A pinned sha256 is the one thing a re-tagged release cannot
   silently swap out.
6. **Install**: `install -m 0755 statusgen-<platform> <bindir>/statusgen`.

If the pin line for the fully detected platform is **absent**, REFUSE rather than guess a platform.

### 4. Wire CI — confirm, don't re-author
`statusgen init` already emitted the CI workflow (a `lint`-on-PR half and a regenerate-on-main
half). The installer **confirms it is present and correct** rather than writing a second copy. Two
load-bearing properties to confirm:

- **Bootstrap-safe first board.** The regen half must *stage* `STATUS.md` before it decides whether
  to commit — the init-emitted workflow does `git add STATUS.md` then `git diff --cached --quiet ||
  git commit`, which correctly sees the untracked first board and commits it. A workflow that
  instead guards on a *working-tree* `git diff --quiet -- STATUS.md` (no prior `git add`) can't see
  the untracked first board, so a fresh repo would never generate one — that shape is a defect to
  report, not to silently patch around.
- **Acquisition channel matches this install.** The emitted workflow assumes a `statusgen/` *source
  tree* (`cd statusgen && go run .`). An adopter installing the pinned *release binary* (the normal
  case, step 3) swaps those `go run` steps for `statusgen --root .` against the binary named in
  `.assay-versions`, exactly as the workflow's own header comment instructs. Confirm the running
  channel matches; do not leave a `go run` workflow on a repo with no `statusgen/` source.

While `docs/streams/` is still legitimately empty (e.g. the scaffolded `example` stream has been
removed and no real stream authored yet), the PR/lint half needs `--lint --allow-empty-root` so
day-one CI is green; drop that flag the moment the first real stream lands.

### 5. Install the desk plugin + main-guard (scenario-appropriate)
Apply `install-desk-plugin` and `install-main-guard` per the `adopt` runbook, as the scenario calls
for. The plugin surfaces the skills namespaced (`assay:<name>`); the main-guard is optional-but-
recommended client-side hardening that refuses un-flagged `main` commits.

### 6. Prove the install
The install is not done until it is PROVEN:

- **`statusgen --root <target> --lint`** exits **0** (the corpus is lint-clean).
- **`statusgen --version`** prints the **pinned tag** (the binary you installed is the one you
  pinned, not a stale one already on `PATH`).

If either check fails, the install is **not proven** — say so plainly and stop; do not report
success.

### 7. Report what ACTUALLY installed — claim the weaker true thing
Report only what you VERIFIED. Name every step you skipped, every artifact you left untouched
because it was already correct, and every point you escalated to a human. Never a fabricated
success. Carry the honest framing: the board is **derived** from agent-authored artifacts with
linting and independent re-verification — it is **not measured from ground truth**, and it is
re-verified rather than trusted. Report the weaker, true thing. Do not describe the install as
tamper-proof, atomic, or a stronger guarantee than the mechanism delivers.

## The plugin↔statusgen pairing

An adopter on plugin **vX** must get the statusgen **vY** that plugin was built and tested against —
never a mismatched or floating tool. The pairing is shipped in the plugin itself, at
**`plugins/assay/paired-versions.yaml`**. It carries the paired statusgen release **tag** and the
per-platform **sha256** lines (channel-E form), so the installer can BOTH resolve the tag AND verify
the hash from the plugin's own shipped record. The resolution is:

> plugin version (`plugin.json`) → paired statusgen tag (`paired-versions.yaml`) → pinned,
> hash-verified download.

Resolve the pairing from the manifest at install time; never assume a tag, never hardcode one in
prose, and never fall through to `latest`.

## NEVER autonomous — STOP and escalate to a human

The installer authors branches and opens **DRAFT PRs only**. It never performs any of these — it
hands the human the exact values and waits:

- **Reviewer GitHub App** creation / installation — the identity that posts approvals, which a plain
  worker session cannot post as. A placeholder or self-minted stand-in defeats the whole mechanism.
  Claim only attribution-plus-audit-trail, **not** tamper-evidence.
- **Repo creation** + admin / permission grants.
- **Merge to main / pushing to the main branch / release tag / the first ready-flip.**
- **git history rewrite** (for a carve-out).
- **Private-repo CI auth** (module-privacy env / cross-repo checkout token).

Hand the human the exact values (App name + permissions, repo slug + module path, etc.), wait for
confirmation, and never fabricate the outcome.

## Prove-the-machinery loop (optional, after install)
To prove not just the tool but the whole pipeline, walk ONE trivial seed brief through the full
lifecycle — `todo → in-progress → implemented → verified → done` — so the desks, the board, the
reviewer App, and the human-merge gate each fire once (the `adopt` runbook's "hello-world loop").

## Scope

**Unix-first (mac/linux).** The statusgen binary acquisition in step 3 is the only OS-specific arm,
and it is implemented for mac and linux today.

**Windows is a deferred fast-follow — NOT in this skill's scope yet.** The Windows arm (the
`statusgen-windows-amd64.exe` asset, a cross-platform hash-verify, and the `.exe` install path) is a
future, not-yet-authored follow-up. Because the orchestration logic above is already OS-agnostic,
the fast-follow adds a *platform arm to step 3* — it is not a second skill. The platform detection
in step 3 is written so a Windows branch slots in beside the Unix one without reshaping the flow. Do
NOT implement Windows acquisition here; when a Windows host is detected, say the acquisition arm is
not yet available on this platform and stop, rather than guessing.

## Delegation
- **`assay:adopt`** — the scenario router (green-field / existing-suite / carve-out) and the
  PRIMITIVE + human-gate reference this installer wraps.
- **`docs/adopting-assay.md`** — the full step-by-step runbook with exact commands and a per-step
  Verify. When any step above needs more detail, that guide is the ground truth.
