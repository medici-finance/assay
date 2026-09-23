---
name: install
description: >-
  Turnkey installer for the Assay methodology — invoke it and it self-installs the whole project
  setup into the target repo. Use right after adding the plugin from the marketplace, when the ask
  is "install Assay here", "set Assay up in this repo", "run the turnkey install", "bootstrap the
  board/desks", or a cold adopter's first boot. It DETECTS the target repo, acquires +
  sha256-verifies a version-PINNED statusgen binary from the umbrella releases over plain HTTPS
  (never floating/`latest`, no forge CLI needed — GitHub or GitLab), runs `statusgen init` for the
  scaffold, wires CI + the main-guard, and PROVES the install
  (`--lint` == 0, `--version` prints the pinned tag). It is idempotent and REFUSES-not-clobbers an
  already-adopted repo, opens DRAFT PRs only, and escalates every never-autonomous step (reviewer
  identity, repo/permission grants, merge/push/tag, private-repo CI auth) to a human. Unix-first
  (mac/linux), with a native-Windows acquisition arm (PowerShell bootstrap + Go-native
  `deskinstall`, same sha256-verify-or-refuse). For the step-by-step PRIMITIVE detail and the
  scenario routing it delegates to the `adopt` skill + docs/adopting-assay.md.
---

# Install Assay — turnkey installer

You are the **turnkey** entry to Assay. Where `assay:adopt` is the *runbook* (it routes you to a
scenario and holds the PRIMITIVEs and human-gates so a human or agent can hand-walk the install),
`assay:install` is the *installer*: invoke it and it self-installs the whole project setup, driving
each step itself and stopping only at the never-autonomous escalation points.

The orchestration logic here is **Claude-Code-driven, OS-agnostic and forge-neutral**. The single
OS-specific piece is the statusgen *binary acquisition* in step 2 — Unix (mac/linux) via a plain
HTTPS fetch verified against the pin file (`scripts/assay-install.sh acquire`, shipped in this
plugin), and a native-Windows arm via the PowerShell bootstrap + Go-native `deskinstall` (see
**Scope**). Everything else runs identically on every platform, and on GitHub or GitLab — the
per-forge differences are named in **Per-forge prerequisites** and **CORE primitives per forge**
below.

This skill does **not** fork the install steps. It DELEGATES the PRIMITIVE detail — exact commands,
per-step Verify, the failure modes — to **`assay:adopt`** and the full runbook at
**`docs/adopting-assay.md`** in your `assay` checkout. Read those for the ground truth; this skill
is the turnkey orchestration over them.

The helper script is `<bundle>/scripts/assay-install.sh`, where `<bundle>` is this plugin's root
(the directory holding `skills/` and `paired-versions.yaml`). Its subcommands — `classify`, `pin`,
`acquire`, `rehearse` — are the executable form of the steps below; `--help` prints the contract.

## Per-forge prerequisites — two distinct principals

The invariant is **two distinct principals**: a *human identity* that authorizes (blesses, signs
off a `gate:human` brief, merges) and an *automation identity* the fleet runs as. They must never
be the same principal, on any forge — under one shared identity every human gate is
self-satisfiable. How each principal is provisioned differs per forge; the invariant does not:

| Principal | GitHub mechanism | GitLab mechanism | Roster entry it produces |
|---|---|---|---|
| Human identity | a personal account the automation never holds credentials for | a personal user the automation never holds credentials for | its login in `ASSAY_BLESS_LOGIN` / `ASSAY_TRUSTED_LOGINS` |
| Automation identity (implementer and the other roles) | a GitHub App per role, created and installed by a human | a group service account per role, provisioned by a human running `tools/create-fleet-gitlab.sh` | `<role>=github:<slug>[:<id>]` in `ASSAY_TRUSTED_BOT_SLUGS` |
| Reviewer identity (a separate automation principal) | a separate reviewer GitHub App | a separate reviewer service account | `reviewer=gitlab:<username>[:<id>]` (GitLab) or `reviewer=github:<slug>[:<id>]` (GitHub) |

The entry grammar, the per-forge renderings and the corroboration rule are defined ONCE in
`docs/streams/forge-neutral/identity.md` (in your `assay` checkout) — read them there; this table
does not restate them. The GitHub half is walked in `docs/adopting-assay.md` (*Prerequisite: two
distinct principals*), the GitLab half in `docs/adopting-assay-gitlab.md` §1–§2.

## Optional forge CLIs — never required

`gh` (GitHub) and `glab` (GitLab) are **optional on their own forge, and only for convenience
reads a human might do** — listing labels, eyeballing a pipeline, reading a release page. **No
install step requires either one.** Acquisition is a plain HTTPS fetch; every forge write is made
by a desk verb or is a human act on the forge's own settings pages. An install that stops
because a forge CLI is absent is a defect in this skill — report it; never install a CLI to route
around it.

## CORE primitives per forge

The eight CORE primitives (`adopt` §2) plus `create-labels`, each marked forge-neutral or not. Where
one is not, the forge-specific act is named per forge together with the desk verb that makes or
reads it back — never a forge CLI. A cell that says *human* is on the NEVER-autonomous list below.
Every file the flow writes lands on a branch and reaches `main` by a draft PR/MR opened with
`deskpr create`, which resolves the forge itself.

| Primitive | Forge-neutral? | GitHub | GitLab | Desk verb / tool |
|---|---|---|---|---|
| `install-statusgen` | yes | HTTPS fetch + pin-file sha256 (step 2) | identical — the release assets are public; no GitLab account or CLI is involved | `assay-install.sh acquire` (not a forge operation) |
| `scaffold-streams` | yes | files written by `statusgen init` | identical | `statusgen init` |
| `scaffold-registers` | yes | files written by `statusgen init` | identical | `statusgen init` |
| `add-statusgen-ci` | no — delegated | `init` writes the GitHub workflow | `init` writes `.gitlab-ci.yml`; the runner, `STATUSGEN_PUSH_TOKEN` and `STATUSGEN_ROSTER_ENV` are *human* (`docs/adopting-assay-gitlab.md` §2a, §2c, §2e) | `statusgen init` (forge from `origin`, or `--forge`) — never re-authored here |
| `install-desk-plugin` | yes (harness-specific, not forge-specific) | Claude Code `/plugin`; Cursor / Codex copy the skills | identical | none |
| `install-main-guard` | the client hook, yes; its server-side counterpart, no | `.githooks/pre-commit` + `core.hooksPath`; server side: a ruleset / branch protection on `main` — *human* | the same hook; server side: a protected branch with a push-access list — *human*, set by `tools/create-fleet-gitlab.sh` | the hook is plain git; `repohardenguard` reads the server-side setting back on either forge (a per-forge checklist) |
| `first-board` | yes | `statusgen --root <target> --lint` then `statusgen --root <target>` | identical | `statusgen` |
| `setup-reviewer-app` (the reviewer grant) | no | a separate reviewer GitHub App with the three write duties, the CI reads and `administration: read` — *human* | a separate reviewer service account at the role in `docs/adopting-assay-gitlab.md` §1 — *human*, via `tools/create-fleet-gitlab.sh` | `deskroster preflight` (run by `deskboot`) reads the grant back — `app-scopes-vs-duties` on GitHub, the token-custody check on GitLab |
| `create-labels` | no | a one-off at the repo's label settings — *human/admin*; the list is `docs/adopting-assay.md` §3 `create-labels` | created by `tools/create-fleet-gitlab.sh` (same names, colours and descriptions) — *human* | `deskflip` creates the PR-state pair on first use on either forge; `deskfile` only PROBES `raised-by:*` and files unstamped when one is missing. No desk verb provisions the whole set yet (#1559) |

`tools/create-fleet-gitlab.sh` and the two runbooks live in your `assay` checkout. Nothing in the
table is performed by `gh` or `glab`.

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
inferred. Name the target's **forge** too (from its `origin` remote — a GitHub or GitLab host), and
confirm the adopter has the **two distinct principals** of **Per-forge prerequisites** above — a
human identity separate from the automation identity; do not proceed under one shared identity.

Then run the refuse-not-clobber check mechanically: `bash <bundle>/scripts/assay-install.sh
classify --root <target>` prints `fresh`, or `partial` with what is present, or REFUSES on
`adopted` (exit 5 — the first-class "already adopted; nothing to install" outcome above).

**Rehearse first — the dry run.** `bash <bundle>/scripts/assay-install.sh rehearse --target
<target>` runs steps 2, 3 and 6 end to end against a SCRATCH COPY of the target (kept under the
target's own name): classify → pin → acquire + verify → `statusgen init` → prove. It does not
confirm CI (step 4) or install the plugin and main-guard (step 5). It prints whether a forge CLI is on
`PATH` (it needs neither), writes nothing to the real target, pushes nothing and opens no PR. A
rehearsal that does not end `rehearsal PROVEN` is a reason to stop before touching the target.

### 2. Acquire + pin + verify statusgen (Unix mechanism)
Version-PINNED, **sha256-verified**, never floating and never `latest`. This is the channel-E
`.assay-versions` mechanism the `adopt` runbook's `install-statusgen` PRIMITIVE defines; the
installer wraps it. It runs BEFORE the scaffold, because the scaffold is `statusgen init` and needs
the verified binary. **No forge CLI is involved, on either forge**: the release assets are public
and fetched over plain HTTPS.

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
   pairing manifest: `assay-install.sh pin --manifest <bundle>/paired-versions.yaml --pins
   <target>/.assay-versions`. An identical line is left alone; EVERY scaffold placeholder line
   (`statusgen init` writes one per platform plus the bare `statusgen` line) is filled from the
   manifest, or the step refuses naming the ones it cannot fill; a DIFFERENT real line is refused
   — a re-pin is a reviewed change, never a silent in-place edit. A refusal leaves the file
   untouched.
4. **Fetch and verify**: `assay-install.sh acquire --pins <target>/.assay-versions --dest <bindir>
   --release-home <release_home>` — `<release_home>` is the one named in `paired-versions.yaml`
   (resolve it from there, do not hardcode it in prose). It fetches
   `https://github.com/<release_home>/releases/download/<tag>/statusgen-<platform>` with `curl`,
   computes the sha256, and compares it to the digest **in the pin file**. The download is
   **HTTPS-only on the initial URL and on every redirect hop**: a non-HTTPS URL or a redirect to
   one is REFUSED with nothing written (cross-host HTTPS redirects, which GitHub's asset links
   use, are followed). That narrows the transport; it never replaces the digest check. The pin
   file is the single source of the expected value: nothing fetched from the release home — its
   `checksums.txt` included — is ever the comparison's source, or a substituted asset could vouch
   for itself.
5. **REFUSE on mismatch, and on an unreadable digest.** A hash mismatch is a hard stop, not a
   warning (exit 5, nothing installed). So is a digest that **cannot be read** — no pin line for
   the platform, a placeholder, a malformed value, two competing lines (exit 5, nothing
   installed). A failed fetch, or a host with no sha256 tool, is could-not-check (exit 6, nothing
   installed). There is no "verification unavailable, continuing" path: a pinned sha256 is the one
   thing a re-tagged release cannot silently swap out.
6. **Install** happens only after the digest matched: the verified bytes are placed at
   `<bindir>/statusgen`, and nowhere before that.

If the pin line for the fully detected platform is **absent**, REFUSE rather than guess a platform.

### 2b. (Optional) Acquire the desk-tools — same mechanism
**Only if the adopter runs the automated desk pipeline.** The desk-role binaries (`deskboard`,
`deskpr`, `deskevidence`, `deskfile`, `deskpost`, …) are the desk skills' primary path — they carry
the guards, write-budgets, and roster + trust gates, and they are how the forge writes in
**CORE primitives per forge** are made on either forge. This is a **skippable** step, not part of
the minimal install.

Acquisition is **channel-E, identical to step 2** — resolve the tag + per-platform sha256 from the
`desk-tools:` section of `paired-versions.yaml` (same tag as statusgen — cut from the same release),
then `assay-install.sh pin --kind desk-tools …` and `assay-install.sh acquire --kind desk-tools …`:
the same HTTPS fetch, the same pin-file comparison, **REFUSE on a mismatch or an unreadable
digest**, and only then extract and install the binaries to `PATH`. The only shape difference is
the artifact is a `.tar.gz` of binaries, not a single file. Config is at the config-home
(`../../references/desk-shell.md` §Config home) only, never the environment. See `install-desk-tools` in the `adopt` runbook.

**Verify:** `deskboard --version` prints and its `assay-config:` echo shows the roster present.

### 3. Scaffold — use `statusgen init`, never reinvent it
Run **`statusgen init --root <target>`** (the umbrella `statusgen` subcommand, from the binary
step 2 verified). It already emits a
**lint-clean tree** — `docs/streams/` plus the registers — a bootstrap-safe CI workflow, AND the
day-one agent-instruction files at the target root (`CLAUDE.md` with the ten invariants + the CI
recipe + an unanswered bindings checklist, and an `AGENTS.md` pointing at it). Every target is
skipped if it already exists, so an adopter's own `CLAUDE.md` is never clobbered. The
installer USES this; it does not hand-author a second copy of the scaffold. `init` picks the CI
half from the target's forge — a GitHub workflow for a GitHub `origin`, a `.gitlab-ci.yml` for a
GitLab one, and neither (with a note saying why) when the host names neither forge; `--forge
github|gitlab` states it explicitly. It never overwrites a file that exists, so the
`.assay-versions` step 2 wrote is kept. If `statusgen init` is not yet available (an older pinned
binary predates the subcommand), fall back to the `adopt` runbook's `scaffold-streams` /
`scaffold-registers` PRIMITIVEs and say you did so — do not fake the scaffold.

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
  case, step 2) swaps those `go run` steps for `statusgen --root .` against the binary named in
  `.assay-versions`, exactly as the workflow's own header comment instructs. Confirm the running
  channel matches; do not leave a `go run` workflow on a repo with no `statusgen/` source.

**The workflow set is more than statusgen — say which apply here.** `assay-statusgen` (the two
halves above) is the **required** one this step installs. `leaksweep-control` + `leaksweep-pattern`
are **recommended, especially for a public repo** — the automated secret-scan gate; on a
branch-protected repo they are a **required check the board-writer App must also bypass** (step 4's
board-writer note). `ci` / `release` / `docker-publish` belong to the repo that **builds + releases
the tools**, NOT a consuming adopter — an adopter pins binaries and does not cut releases, so add
those only if the adopter hosts its own tool releases. Do not copy release plumbing into a plain
adopter.

While `docs/streams/` is still legitimately empty (e.g. the scaffolded `example` stream has been
removed and no real stream authored yet), the PR/lint half needs `--lint --allow-empty-root` so
day-one CI is green; drop that flag the moment the first real stream lands.

**If `main` is branch-protected, the regen half needs a board-writer App (recommended-but-optional).**
Branch protection on `main` is *recommended but optional*; when it is on, the push-to-main half can
no longer commit `STATUS.md` directly — a protected branch rejects that push. Confirm the regen job
mints and pushes as a dedicated **board-writer App** (`contents: write` only, NOT
`github-actions[bot]`) that is on the branch's **ruleset bypass list**; if a required check (e.g. a
leak-sweep) also guards the branch, the App needs the bypass on that ruleset too — bypassing one
ruleset does not bypass another. Validate the workflow YAML **and** that the branch's ruleset
actually lists the App — a job can pass YAML review yet be rejected at runtime by a ruleset that
never got the bypass. With protection off, the regen pushes as the automation account and no
board-writer App is needed. App creation + the ruleset-bypass edit are **human-gated** (see the
NEVER-autonomous list below). **On GitLab** the same role is a board-writer service account on the
protected branch's push-access list, and the regen job pushes with the `STATUSGEN_PUSH_TOKEN`
the scaffolded `.gitlab-ci.yml` names — both human-provisioned (`docs/adopting-assay-gitlab.md`
§2c).

### 5. Install the desk plugin + main-guard (scenario-appropriate)
Apply `install-desk-plugin` and `install-main-guard` per the `adopt` runbook, as the scenario calls
for. The plugin surfaces the skills namespaced (`assay:<name>`); the main-guard is optional-but-
recommended client-side hardening that refuses un-flagged `main` commits — a plain git hook,
identical on either forge. Its server-side counterpart differs per forge and is a human act; see
**CORE primitives per forge**.

### 6. Prove the install
The install is not done until it is PROVEN:

- **`statusgen --root <target> --lint`** exits **0** (the corpus is lint-clean).
- **`statusgen --version`** prints the **pinned tag** (the binary you installed is the one you
  pinned, not a stale one already on `PATH`).

If either check fails, the install is **not proven** — say so plainly and stop; do not report
success.

**GitLab forge — the runner is part of the proof.** `statusgen init --forge gitlab` scaffolds
the `.gitlab-ci.yml` but registers **no runner** (that is instance-admin work, an explicit
non-goal). A GitLab pipeline can fire, parse the YAML, and still never start — an **untagged**
scaffold job sits in `pending` / `stuck_pending_no_matching_runners` when no runner takes
untagged jobs (`run_untagged = false` is a common default). So on a GitLab adopter, do **not**
call CI installed on the CI file alone: watch the first merge-request (or default-branch)
pipeline and require a job to **leave `pending`** — reach `running`, or a terminal **non-stuck**
result (an unset `STATUSGEN_PUSH_TOKEN` is a *later* red — the job ran). A job still `pending`
is **could-not-check**, never a pass. Hand the human the runner requirement (a Linux
Docker/Kubernetes executor with `run_untagged = true`, or the instance's tag(s) on each job's
commented `tags:` placeholder) — see `docs/adopting-assay-gitlab.md`, section
"Runners and job tags". Assay does not register runners.

### 7. Report what ACTUALLY installed — claim the weaker true thing
Report only what you VERIFIED. Name every step you skipped, every artifact you left untouched
because it was already correct, and every point you escalated to a human. Never a fabricated
success. Carry the honest framing: the board is **derived** from agent-authored artifacts with
linting and independent re-verification — it is **not measured from ground truth**, and it is
re-verified rather than trusted. Report the weaker, true thing. Do not describe the install as
tamper-proof, atomic, or a stronger guarantee than the mechanism delivers.

**Then hand the human their half — the install is not "done" until they know what is left.** The
skill did the autonomous parts; it **stopped at every act that mints an identity, grants a
permission, or authorizes a merge**, and those are the human's. Do not close the run on "here is what
I installed" alone — end with **"Install done. Here is what YOU must do now:"** and point at the
**Human post-install checklist** section of `docs/adopting-assay.md` (provision the two principals,
create the automation identities — GitHub Apps, or GitLab service accounts per
`docs/adopting-assay-gitlab.md` §2 — and store their credentials at the config-home, choose the
roster values, set the variables + secrets, branch protection + board-writer bypass, the standing
human gates, and optionally the desk-tools). Surface that remaining setup explicitly in the final report — a run that
lists only what the skill did, and leaves the human to discover the identity/permission/merge steps
on their own, has under-reported.

## The plugin↔statusgen pairing

An adopter on plugin **vX** must get the statusgen **vY** that plugin was built and tested against —
never a mismatched or floating tool. The pairing is shipped in the plugin itself, at
**`plugins/assay/paired-versions.yaml`**. It carries the paired statusgen release **tag** and the
per-platform **sha256** lines (channel-E form), so the installer can BOTH resolve the tag AND verify
the hash from the plugin's own shipped record. The resolution is:

> plugin version (`plugin.json`) → paired statusgen tag (`paired-versions.yaml`) → pinned,
> hash-verified download.

Resolve the pairing from the manifest at install time; never assume a tag, never hardcode one in
prose, and never fall back to `latest`.

## NEVER autonomous — STOP and escalate to a human

The installer authors branches and opens **DRAFT PRs only**. An install or re-pin PR delivers no
brief, so its body carries the `Issue: #<N>` link trailer — file a short tracking issue for the
install/bump and name it — not a `Brief:` line; both forms satisfy `deskpr` and `pr-review-desk`
(this repo's own front-door re-pin, PR #496, is the precedent). It never performs any of these — it
hands the human the exact values and waits:

- **Reviewer identity** creation / installation — a GitHub App, or a GitLab service account — the
  identity that posts approvals, which a plain worker session cannot post as. A placeholder or self-minted stand-in defeats the whole mechanism.
  Claim only attribution-plus-audit-trail, **not** tamper-evidence.
- **Board-writer identity** creation + the edit that admits it past protection — needed only
  when `main` is branch-protected (step 4). On GitHub a dedicated `contents: write`-only App, added
  to the branch's ruleset bypass so the push-to-main board regen can commit `STATUS.md` past
  protection; on GitLab a board-writer service account on the protected branch's push-access list,
  plus the `STATUSGEN_PUSH_TOKEN` CI/CD variable. These are repo-admin acts — hand the human the
  identity name + its single write permission + the branch/ruleset to add it to, and wait.
- **Repo creation** + admin / permission grants.
- **Merge to main / pushing to the main branch / release tag / the first ready-flip.**
- **git history rewrite** (for a carve-out).
- **Private-repo CI auth** (module-privacy env / cross-repo checkout token).
- **The GitLab runner** — registering or tagging a runner is instance-admin work (step 6).

Hand the human the exact values (identity name + permissions, repo slug + module path, etc.), wait for
confirmation, and never fabricate the outcome.

## Prove-the-machinery loop (optional, after install)
To prove not just the tool but the whole pipeline, walk ONE trivial seed brief across the full
lifecycle — `todo → in-progress → implemented → verified → done` — so the desks, the board, the
reviewer identity, and the human-merge gate each fire once (the `adopt` runbook's "hello-world loop").

## Scope

**Unix-first (mac/linux), with a real native-Windows arm.** The statusgen binary acquisition in
step 2 is the only OS-specific arm. It is implemented for mac and linux as a plain HTTPS fetch
verified against the pin file (`assay-install.sh acquire` — `curl`, plus `sha256sum`,
`shasum` or `openssl`; no forge CLI), and there is now a **native-Windows path** that slots in
beside the Unix one without reshaping the flow. Forge is not an axis here: the same arm serves a
GitHub and a GitLab adopter, because the release assets are fetched from their public release home
whatever forge the target lives on.

**Windows is supported — the acquisition arm is real, not deferred.** On a native Windows host,
step 2 acquires the pinned `statusgen-windows-<arch>.exe` (and `desk-tools-windows-<arch>.tar.gz`)
via a PowerShell first-install bootstrap (`scripts/bootstrap-windows.ps1`) plus the Go-native
`deskinstall` command, keeping the same **sha256-verify-or-refuse** control the Unix path uses
(a hash mismatch is a hard refuse — exit 5 — never a warn-and-continue). Two honesty caveats remain
and are stated in the runbook, not hidden: the harness's session-start resident-rules injection
channel (harness-portability/05; the mechanism your harness uses is named in
`../../references/<harness>.md`) needs a documented `bash`+`jq`
workaround (install Git-Bash, or WSL for local dev only — WSL is a fallback, not the native claim),
and the **native `windows/arm64` smoke is BLOCKED** pending an arm64 Windows runner (the arm64
asset still ships cross-compiled + checksummed). The full step-by-step Windows guide — install
command, `.assay-versions` pins, CI-proven status, and the documented-workaround surfaces — lives in
`docs/adopting-assay.md` § **Windows adopters**.

## Delegation
- **`assay:adopt`** — the scenario router (green-field / existing-suite / carve-out) and the
  PRIMITIVE + human-gate reference this installer wraps.
- **`docs/adopting-assay.md`** — the full step-by-step runbook with exact commands and a per-step
  Verify. When any step above needs more detail, that guide is the ground truth.
