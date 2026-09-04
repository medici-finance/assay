package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// initStreamPlaceholder is the token every scaffold template uses where the
// starter stream's identity belongs. runInit substitutes it once, so the
// identity is chosen in ONE place and cannot drift between the stream
// directory, its frontmatter, the brief id and the register examples.
const initStreamPlaceholder = "{{stream}}"

// initCIFilePlaceholder is the token the next-steps text uses where the scaffolded
// CI file's path belongs. runInitForge substitutes it with whichever half it
// actually wrote (the GitHub workflow or `.gitlab-ci.yml`), so the closing advice
// never tells a GitLab adopter to commit a file the scaffold did not create
// (#349).
const initCIFilePlaceholder = "{{cifile}}"

// initFallbackStream is the identity used only when the target directory's name
// sanitises to nothing (e.g. a root named "/" or "---"). It is deliberately the
// historical literal: a name we cannot derive is better than one we invent.
const initFallbackStream = "example"

// initStreamName derives the starter stream's identity from the TARGET REPO's
// directory name, sanitised to the lowercase [a-z0-9-] identity shape statusgen
// accepts (the stream name must equal its directory name, and brief ids are
// `<stream>/NN`).
//
// Why derive rather than hardcode: scaffolding every repo with the same literal
// identity makes any two freshly-init'd repos COLLIDE by construction, and a
// duplicate stream identity across roots is a hard error in a multi-root run —
// so the out-of-the-box scaffold would break the very feature it is scaffolding
// for. Deriving from the directory name is not a uniqueness proof (two repos can
// share a basename), but it removes the guaranteed collision, and the multi-root
// pre-pass still catches the residue.
//
// The reserved register names are avoided because a stream directory called
// `findings`/`intake` is skipped by stream discovery — the scaffold would then
// resolve to zero streams and fail lint.
func initStreamName(root string) string {
	base := root
	if abs, err := filepath.Abs(root); err == nil {
		base = abs
	}
	var b strings.Builder
	for _, r := range strings.ToLower(filepath.Base(base)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		// Collapse any run of unacceptable runes into a single separator.
		if s := b.String(); s != "" && !strings.HasSuffix(s, "-") {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return initFallbackStream
	}
	if reservedRegisterNames[name] {
		return name + "-stream"
	}
	return name
}

// initCITemplate names the CI file `init` scaffolds for a given forge, and the
// template body to write there. The scaffold writes exactly ONE CI half — the one
// that matches the target's forge — because a GitHub workflow on a GitLab project
// is inert (no pipeline runs it), which is exactly the silent-half-install
// #349 reports.
type initCITemplate struct {
	path string
	body string
}

// ciTemplateFor resolves the CI half for a forge. forgeUnknown (no remote, or a
// self-hosted host naming neither forge) falls back to the historical GitHub
// half: it is the established default and keeps every no-remote scaffold (a fresh
// `t.TempDir()`, a repo whose origin is not yet set) byte-identical to before.
func ciTemplateFor(forge forgeKind) initCITemplate {
	if forge == forgeGitLab {
		return initCITemplate{path: ".gitlab-ci.yml", body: initGitlabCI}
	}
	return initCITemplate{path: ".github/workflows/assay-statusgen.yml", body: initWorkflow}
}

// runInit scaffolds the streams structure a repo needs to adopt the methodology:
// the registers, a streams README, one starter stream + brief, a CI workflow, and
// the day-one agent-instruction files (CLAUDE.md + AGENTS.md) carrying the ten
// universal invariants and the adopter CI recipe.
// It NEVER overwrites an existing file — each target is created only if absent, so
// running it in a partially-set-up repo fills the gaps without clobbering work.
// After scaffolding it prints next steps. `statusgen init --root DIR` targets DIR.
//
// The CI half is chosen from the target's `origin` forge (see runInitForge): a
// GitHub remote (or none) gets the GitHub workflow, a GitLab remote gets a
// `.gitlab-ci.yml` running the same two halves — because a GitHub workflow on a
// GitLab project is inert and leaves the board with no single writer (#349).
func runInit(root string) int {
	return runInitForge(root, detectForge(root))
}

// runInitForge is runInit with the forge already resolved — from the `--forge`
// flag when the operator gave one, else auto-detected. Kept separate so main()
// can honour an explicit flag while the bare runInit (used across the tests and
// as the default entry) still auto-detects.
//
// The starter stream is named after the target directory (see initStreamName), so
// two freshly-init'd repos do not collide when a later run boards them together.
func runInitForge(root string, forge forgeKind) int {
	stream := initStreamName(root)
	streamDir := "docs/streams/" + stream
	ci := ciTemplateFor(forge)
	files := []struct{ path, body string }{
		{"docs/streams/README.md", initStreamsReadme},
		{"docs/streams/FINDINGS.md", initFindings},
		{"docs/streams/INTAKE.md", initIntake},
		{"docs/streams/RETRO.md", initRetro},
		{streamDir + "/README.md", initExampleStream},
		{streamDir + "/brief-01-first-brief.md", initExampleBrief},
		{".assay-versions", initAssayVersions},
		{ci.path, ci.body},
		{"CLAUDE.md", initClaudeMd},
		{"AGENTS.md", initAgentsMd},
	}
	var created, skipped []string
	for _, f := range files {
		abs := filepath.Join(root, filepath.FromSlash(f.path))
		if _, err := os.Stat(abs); err == nil {
			skipped = append(skipped, f.path)
			continue
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "statusgen init:", err)
			return 1
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen init:", err)
			return 1
		}
		body := strings.ReplaceAll(f.body, initStreamPlaceholder, stream)
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen init:", err)
			return 1
		}
		created = append(created, f.path)
	}

	for _, p := range created {
		fmt.Printf("  created  %s\n", p)
	}
	for _, p := range skipped {
		fmt.Printf("  exists   %s (unchanged)\n", p)
	}
	if len(created) == 0 {
		fmt.Println("\nNothing to create — the streams structure is already in place.")
		return 0
	}
	next := strings.ReplaceAll(initNextSteps, initStreamPlaceholder, stream)
	next = strings.ReplaceAll(next, initCIFilePlaceholder, ci.path)
	fmt.Print(next)
	return 0
}

const initStreamsReadme = `# streams/

Each initiative is a **stream** — a directory under ` + "`docs/streams/<stream>/`" + ` with a
README (frontmatter + a brief table) and self-contained ` + "`brief-NN-*.md`" + ` files.
Append-only registers live alongside: ` + "`FINDINGS.md`" + ` (knowledge that invalidates a
brief) and ` + "`INTAKE.md`" + ` (raw ideas).

` + "`statusgen`" + ` reads these to generate the ` + "`STATUS.md`" + ` board and to lint the set in CI.
See the ` + "`{{stream}}/`" + ` stream for the shape; delete it once you have your own.
`

const initFindings = `# Findings

Append-only log of knowledge that invalidates or updates existing briefs. Sequence
contiguity is enforced by statusgen; withdrawal is a tombstone entry, never deletion.

## F-01 — 2026-01-01 — Example resolved finding

Delete this example. A real finding records what was learned and which briefs it affects.

Affects: {{stream}}/brief-01
Resolved: yes
`

const initIntake = `# Intake

Front door for raw ideas. Same append-only + sequence rules as FINDINGS.

## I-01 — 2026-01-01 — Example intake entry

Delete this example. A real entry captures an idea and its disposition.

Disposition: scoped → {{stream}}
`

const initRetro = `# Retro — cadence retrospective

Append-only cadence log (weekly to start). Inputs are generated/logged only — no
prose status narrative: a retro that reads its own narrative measures the
narrator, not the system. ` + "`statusgen`" + ` does NOT parse this file — it is the human
cadence log alongside the ` + "`statusgen`" + `-consumed FINDINGS + INTAKE registers.

Each retro walks: STATUS totals delta since last entry; streams untouched since
last retro (the staleness / rabbit-hole list); gate yield (what reviewers caught);
FINDINGS age; INTAKE entries still new; open bugs; branch/worktree hygiene; knob
tuning. ONE process change max per retro, recorded as an intake/finding/brief and
never enacted inline. Numbering is gap-free; withdraw with a tombstone (keep the
number, note the reversal), never by deleting a heading.

## R-01 — 2026-01-01 — Example retro

Delete this example. A real entry records the checklist walked (with numbers) and
the one process change (or "none"), with links.
`

const initExampleStream = `---
stream: {{stream}}
status: active
priority: P2
track: platform
---

# {{stream}}

A starter stream so the board renders and lint passes. Replace it with your own work,
then delete this directory.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Your first brief](./brief-01-first-brief.md) | 0 | S | todo | — | — |

## Critical path
Brief 01 is standalone.

## Dependency waves
- Wave 0: 01
`

const initExampleBrief = `---
brief: {{stream}}/01
title: Your first brief
why: A worked example so the board renders and lint passes. Replace it with a real brief
  whose motivation a non-engineer could justify in one to three lines.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-01-01 by statusgen init
sources: ["{{stream}}"]
---

# Brief 01 — Your first brief

## Context
files: (list the files this brief touches)
facts:
- Replace this brief with your own. Keep it self-contained: scope, rules, task, Verify.

## Ground rules
- NEVER git push / trigger workflows. Commit per the task instructions only.
- Stop at ` + "`implemented`" + ` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Replace this with the actual work.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`go test ./...`" + ` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer. -->

## Review
Gate: model (from frontmatter).
`

// initAssayVersions is a PLACEHOLDER pin file in the channel-E shape
// (statusgen-<platform> <tag> <sha256>). It is deliberately not a live pin: init
// is baked into the statusgen binary, so a hardcoded digest here would go stale
// every release and show stale in the very tool that emits it. The adopter fills
// a real tag + per-platform digest once, consciously, from a release's
// checksums.txt — the install step below refuses rather than guesses on a
// placeholder/absent line, so a fresh repo cannot silently install an unpinned or
// wrong binary. Shape matches medici-finance/assay docs/adopting-assay.md
// § install-statusgen.
const initAssayVersions = `# statusgen pin for this repo's lint/regen CI — channel E, the sha256-pinned
# release binary (medici-finance/assay docs/adopting-assay.md § install-statusgen).
# One line per platform you install on, in the form:
#
#     statusgen-<platform>  <tag>  <sha256>
#
# PLACEHOLDER — fill in a REAL release tag + per-platform sha256 before the first
# CI run. Pick the statusgen release you want to pin, download its checksums.txt
# from the medici-finance/assay releases, and replace each REPLACE_WITH_* token
# with the tag and the digest for that artifact. Match the FULL platform (os AND
# arch). Re-pin (never edit in place) on an upgrade so the bump shows in a diff.
# The install step refuses rather than guesses on an absent or placeholder line.
statusgen-darwin-arm64  REPLACE_WITH_TAG  REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS
statusgen-darwin-amd64  REPLACE_WITH_TAG  REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS
statusgen-linux-amd64   REPLACE_WITH_TAG  REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS
`

const initWorkflow = `# statusgen CI — the two-half single-writer shape (medici-finance/assay
# docs/adopting-assay.md, section: add-statusgen-ci). The PR half runs --lint
# only; the push-to-main half regenerates STATUS.md and commits it. STATUS.md is
# generated by this workflow on main and has a SINGLE writer: never commit
# STATUS.md on a branch.
#
# statusgen comes from ONE place — the sha256-pinned release binary named in
# .assay-versions (channel E), installed and hash-verified below. Vendoring the
# source is retired: a vendored copy is an unpinned fork that rots silently. Fill
# .assay-versions with a real tag + per-platform digest before the first run; the
# install step refuses rather than guesses on a placeholder/absent pin line.
name: statusgen
on:
  pull_request:
  push:
    branches: [main]
permissions:
  contents: write
jobs:
  lint:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install pinned statusgen (channel E — sha256-verified release binary)
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          set -euo pipefail
          plat="$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')"
          line="$(grep "^statusgen-$plat " .assay-versions || true)"
          if [ -z "$line" ]; then
            echo "::error::no .assay-versions pin for platform $plat — refusing rather than guessing"
            exit 1
          fi
          tag="$(printf '%s' "$line" | awk '{print $2}')"
          sha="$(printf '%s' "$line" | awk '{print $3}')"
          gh release download "$tag" --repo medici-finance/assay --pattern "statusgen-$plat" -O /tmp/statusgen
          echo "${sha}  /tmp/statusgen" | shasum -a 256 -c -
          sudo install -m 0755 /tmp/statusgen /usr/local/bin/statusgen
      # A freshly-init'd tree ships an example stream, so docs/streams/ is never
      # empty on day one and --lint is green WITHOUT --allow-empty-root. If you
      # delete the example before authoring your own stream, add --allow-empty-root
      # to the line below for that transitional window ONLY — do not leave it on, or
      # the empty-root PROBLEM can never fire for a genuine regression.
      - name: statusgen --lint
        run: statusgen --lint
  regen:
    # STATUS.md is generated on main only, never on a branch (single writer).
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install pinned statusgen (channel E — sha256-verified release binary)
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          set -euo pipefail
          plat="$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')"
          line="$(grep "^statusgen-$plat " .assay-versions || true)"
          if [ -z "$line" ]; then
            echo "::error::no .assay-versions pin for platform $plat — refusing rather than guessing"
            exit 1
          fi
          tag="$(printf '%s' "$line" | awk '{print $2}')"
          sha="$(printf '%s' "$line" | awk '{print $3}')"
          gh release download "$tag" --repo medici-finance/assay --pattern "statusgen-$plat" -O /tmp/statusgen
          echo "${sha}  /tmp/statusgen" | shasum -a 256 -c -
          sudo install -m 0755 /tmp/statusgen /usr/local/bin/statusgen
      - name: Regenerate STATUS.md
        run: statusgen --root .
      - name: Commit STATUS.md if it changed
        run: |
          set -euo pipefail
          git config user.name  'statusgen'
          git config user.email 'statusgen@users.noreply.github.com'
          git add STATUS.md
          # BOOTSTRAP-SAFE GUARD: guard on ` + "`git status --porcelain -- STATUS.md`" + `, NOT
          # ` + "`git diff --quiet -- STATUS.md`" + `. git diff sees only TRACKED files, so on a
          # repo with no STATUS.md yet the diff guard mis-fires ("nothing to commit")
          # and the first board is NEVER created; porcelain reports the new/staged file.
          if [ -n "$(git status --porcelain -- STATUS.md)" ]; then
            git commit -m 'chore(status): regenerate [skip-status-regen]'
            git push
          fi
`

// initGitlabCI is the GitLab equivalent of initWorkflow, scaffolded when the
// target's `origin` is a GitLab remote (#349). It runs the SAME two halves —
// --lint on merge requests, regen-and-commit on the default branch — so a GitLab
// adopter's board has a single writer, exactly as the GitHub half gives a GitHub
// adopter. Three GitLab-specific facts are load-bearing:
//   - The pinned statusgen release assets live on the medici-finance/assay GitHub
//     releases even when the project is on GitLab, so the install step fetches
//     them over plain HTTPS and sha256-verifies, rather than shelling `gh`.
//   - GitLab's default CI job token cannot push back to the repo. The regen job
//     therefore uses a project/group access token the adopter sets as the masked
//     CI/CD variable STATUSGEN_PUSH_TOKEN, and STOPS with a clear message rather
//     than pushing when it is unset — the same refuse-don't-guess shape as the
//     pin line.
//   - A push by the regen job would itself trigger a pipeline; the [skip-status-regen]
//     commit marker is matched by a `when: never` rule so the board write does not
//     loop.
const initGitlabCI = `# statusgen CI — the two-half single-writer shape on GitLab, mirroring the GitHub
# workflow (medici-finance/assay docs/adopting-assay.md, section: add-statusgen-ci).
# The merge-request half runs --lint only; the default-branch half regenerates
# STATUS.md and commits it. STATUS.md is generated by this pipeline on the default
# branch and has a SINGLE writer: never commit STATUS.md on a branch.
#
# statusgen comes from ONE place — the sha256-pinned release binary named in
# .assay-versions (channel E). Its release assets live on the medici-finance/assay
# GitHub releases even when THIS project is on GitLab, so the install step fetches
# them over HTTPS and hash-verifies below (no ` + "`gh`" + ` needed). Fill .assay-versions
# with a real tag + per-platform digest before the first run; the install step
# refuses rather than guesses on a placeholder/absent pin line.
#
# The regen job pushes STATUS.md back to the default branch. GitLab's default CI
# job token cannot push, so create a project (or group) access token with the
# write_repository scope and set it as a MASKED CI/CD variable named
# STATUSGEN_PUSH_TOKEN. Until it is set the regen job stops with a clear message
# rather than pushing.
stages: [statusgen]

.statusgen-install: &statusgen-install
  - |
    set -euo pipefail
    plat="$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')"
    line="$(grep "^statusgen-$plat " .assay-versions || true)"
    if [ -z "$line" ]; then
      echo "no .assay-versions pin for platform $plat — refusing rather than guessing"
      exit 1
    fi
    tag="$(printf '%s' "$line" | awk '{print $2}')"
    sha="$(printf '%s' "$line" | awk '{print $3}')"
    curl -fsSL "https://github.com/medici-finance/assay/releases/download/$tag/statusgen-$plat" -o /tmp/statusgen
    echo "${sha}  /tmp/statusgen" | sha256sum -c -
    install -m 0755 /tmp/statusgen /usr/local/bin/statusgen

statusgen-lint:
  stage: statusgen
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
  script:
    - *statusgen-install
    # A freshly-init'd tree ships an example stream, so docs/streams/ is never
    # empty on day one and --lint is green WITHOUT --allow-empty-root. If you
    # delete the example before authoring your own stream, add --allow-empty-root
    # to the line below for that transitional window ONLY — do not leave it on, or
    # the empty-root PROBLEM can never fire for a genuine regression.
    - statusgen --lint

statusgen-regen:
  # STATUS.md is generated on the default branch only, never on a branch (single
  # writer). The first rule stops the job re-firing on its OWN regen commit: the
  # push carries the [skip-status-regen] marker, so this pipeline does not loop.
  stage: statusgen
  rules:
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH && $CI_PIPELINE_SOURCE == "push" && $CI_COMMIT_MESSAGE =~ /\[skip-status-regen\]/'
      when: never
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH && $CI_PIPELINE_SOURCE == "push"'
  script:
    - *statusgen-install
    - |
      if [ -z "${STATUSGEN_PUSH_TOKEN:-}" ]; then
        echo "STATUSGEN_PUSH_TOKEN is not set — cannot push the regenerated board."
        echo "Create a project access token with the write_repository scope and set it as a masked CI/CD variable named STATUSGEN_PUSH_TOKEN. Refusing rather than guessing."
        exit 1
      fi
    - statusgen --root .
    - |
      set -euo pipefail
      git config user.name  'statusgen'
      git config user.email 'statusgen@users.noreply.gitlab.com'
      git add STATUS.md
      # BOOTSTRAP-SAFE GUARD: guard on ` + "`git status --porcelain -- STATUS.md`" + `, NOT
      # ` + "`git diff --quiet -- STATUS.md`" + `. git diff sees only TRACKED files, so on a
      # repo with no STATUS.md yet the diff guard mis-fires ("nothing to commit")
      # and the first board is NEVER created; porcelain reports the new/staged file.
      if [ -n "$(git status --porcelain -- STATUS.md)" ]; then
        git commit -m 'chore(status): regenerate [skip-status-regen]'
        git push "https://oauth2:${STATUSGEN_PUSH_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" "HEAD:${CI_DEFAULT_BRANCH}"
      fi
`

// initClaudeMd is the day-one agent-instruction file. It carries exactly two
// things a session cannot infer and every adopter needs on day one: the ten
// methodology INVARIANTS (universal — true of every repo running the method) and
// the adopter CI recipe (matching docs/adopting-assay.md § install-statusgen and
// § add-statusgen-ci). Everything below that is a CHECKLIST of the repo-local
// bindings the adopter fills in — the categories, not the values, exactly as the
// adopting guide's "your own instruction file" section names them.
//
// Deliberately NOT here: any organisation-specific machinery (identities, tools,
// rosters, paths). Those are bindings, and bindings are per-repo. The file is the
// adopter's the moment it lands — init never overwrites it on a re-run, so an
// edited copy always survives.
//
// The content is link-check-safe by construction: statusgen link-checks CLAUDE.md
// (linkcheck.go: docFiles + backtickPathScope), so this template carries no
// backticked slash-path with a checked extension and no markdown link to a file
// the scaffold does not create. Adding one that does not exist would make a
// freshly-init'd repo fail its own --lint.
const initClaudeMd = `# CLAUDE.md

Scaffolded by ` + "`statusgen init`" + ` — this file is yours to edit: keep the invariants,
replace everything else with your own repo's bindings.

## The ten invariants

These hold in every repo running the methodology, whatever the language, harness
or team. They are the floor, not the whole manual.

1. Evidence or it didn't happen — a claim of done/verified without a recorded command + exit code + real output is an unverified claim, never a fact.
2. Non-implementer verify — whoever verifies a piece of work is not who implemented it; a second person or a second session.
3. Derived, not declared — board/register state is computed from evidence; silence reads as unverified, never as success; never edit generated state by hand.
4. Draft-PR-first; the human merges — all work lands on a branch behind a draft PR; direct pushes to main and self-merges are out.
5. One logical change = one branch = one PR — merged or closed = done; follow-up work is a new branch.
6. Merge, never rebase, never force-push a branch anyone else may have.
7. A red check is a work item, never a wait state — reproduce, fix, push; "flake" needs evidence posted on the PR.
8. A blocked action is a stop signal — never route around a guard, gate, or refusal; escalate to the human instead.
9. Removing a security control is a human decision, always — leave the gate red, escalate with evidence.
10. No attribution lines in commits, PRs, or comments.

## CI recipe

1. Install the pinned statusgen release binary: detect the platform, read its line
   from ` + "`.assay-versions`" + ` (` + "`statusgen-<platform>  <tag>  <sha256>`" + `), download that
   release asset, and check its sha256 against the pinned digest. An absent pin
   line or a digest mismatch is a REFUSAL, never a guess — nothing unverified is
   ever installed. Re-pin (never edit in place) on an upgrade, so the bump shows
   in a diff.
2. Run ` + "`statusgen --lint`" + ` on every pull request. A red lint is a work item, not
   a wait state.
3. Regenerate the board on main only: the push-to-main half runs
   ` + "`statusgen --root .`" + ` and commits STATUS.md. STATUS.md has a SINGLE writer —
   never commit it on a branch.

The scaffolded workflow already does all three. If you rewrite it, keep those
three properties; they are what make the board derived rather than declared.

## This repo's bindings — fill these in

The invariants are universal; this section is not. Write down what a session
cannot infer about THIS repo, and nothing else. Answer each line or record it as
not applicable — an unanswered line is a guess waiting to happen.

- Streams — which streams exist, what each owns, and where the streams tree is
  rooted if not at the repo root.
- Pinned tools — which tools are pinned, in which pin file, and the install and
  upgrade commands for this repo.
- The human gate — who, by account name, and which acts require them here: merge,
  the ready-flip, release tags.
- Risk paths — the concrete file and directory globs in this repo that force a
  human gate when a diff touches them.
- Generated / single-writer artifacts — what must never be hand-edited or
  committed on a branch, and which job is the single writer of each.
- Isolation mechanics — whether the checkout is shared, where worktrees go, and
  the branch-naming convention.
- The review identity — the reviewing account as it appears in this repo, so a
  worker can tell a genuine verdict from a relayed or forged one.
`

// initAgentsMd is a POINTER, not a second copy. Harnesses read one of several
// instruction-file names; two files with the same rules in them is drift with a
// start date, and the overlap is itself the bug. So AGENTS.md names CLAUDE.md as
// the single home and tells an AGENTS.md-only harness to MOVE the content rather
// than duplicate it.
const initAgentsMd = `# AGENTS.md

Scaffolded by ` + "`statusgen init`" + ` — this file is yours to edit.

This repo keeps its agent instructions in CLAUDE.md at the repo root: the ten
methodology invariants, the CI recipe, and this repo's own local bindings. Read
that file first.

CLAUDE.md is the SINGLE home for those rules. Do not copy them here — two files
carrying the same rules is drift with a start date, and the overlap is itself the
bug. If your harness reads only AGENTS.md, MOVE the content across and delete
CLAUDE.md, so there is still exactly one instruction file.
`

const initNextSteps = `
Scaffolded the streams structure. Next:

  1. Fill .assay-versions: replace the REPLACE_WITH_TAG / REPLACE_WITH_SHA256 tokens
     with a real statusgen release tag + per-platform sha256 (from that release's
     checksums.txt on medici-finance/assay). CI installs the pinned binary and
     refuses on a placeholder line, so this is the one required bootstrap step.
  2. Lint the set:         statusgen --root . --lint
  3. Generate the board:   statusgen --root .          writes STATUS.md at the repo root
  4. Replace docs/streams/{{stream}}/ with your own stream, then delete it.
  5. Commit {{cifile}} — the two-half single-writer CI
     is already bootstrap-safe (porcelain STATUS.md guard, [skip-status-regen] marker).
  6. Fill in the "This repo's bindings" section of the scaffolded CLAUDE.md. The
     ten invariants above it are universal and stay as they are; the bindings are
     the half nothing can write for you. AGENTS.md points at that one file.

STATUS.md has a SINGLE writer (main's CI): regenerate it locally freely to preview,
but never commit it on a branch. The scaffolded example stream keeps docs/streams/
non-empty, so --lint is green from the first commit — no --allow-empty-root needed.
`
