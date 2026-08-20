package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runInit scaffolds the streams structure a repo needs to adopt the methodology:
// the registers, a streams README, one example stream + brief, and a CI workflow.
// It NEVER overwrites an existing file — each target is created only if absent, so
// running it in a partially-set-up repo fills the gaps without clobbering work.
// After scaffolding it prints next steps. `statusgen init --root DIR` targets DIR.
func runInit(root string) int {
	files := []struct{ path, body string }{
		{"docs/streams/README.md", initStreamsReadme},
		{"docs/streams/FINDINGS.md", initFindings},
		{"docs/streams/INTAKE.md", initIntake},
		{"docs/streams/RETRO.md", initRetro},
		{"docs/streams/example/README.md", initExampleStream},
		{"docs/streams/example/brief-01-first-brief.md", initExampleBrief},
		{".assay-versions", initAssayVersions},
		{".github/workflows/assay-statusgen.yml", initWorkflow},
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
		if err := os.WriteFile(abs, []byte(f.body), 0o644); err != nil {
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
	fmt.Print(initNextSteps)
	return 0
}

const initStreamsReadme = `# streams/

Each initiative is a **stream** — a directory under ` + "`docs/streams/<stream>/`" + ` with a
README (frontmatter + a brief table) and self-contained ` + "`brief-NN-*.md`" + ` files.
Append-only registers live alongside: ` + "`FINDINGS.md`" + ` (knowledge that invalidates a
brief) and ` + "`INTAKE.md`" + ` (raw ideas).

` + "`statusgen`" + ` reads these to generate the ` + "`STATUS.md`" + ` board and to lint the set in CI.
See the ` + "`example/`" + ` stream for the shape; delete it once you have your own.
`

const initFindings = `# Findings

Append-only log of knowledge that invalidates or updates existing briefs. Sequence
contiguity is enforced by statusgen; withdrawal is a tombstone entry, never deletion.

## F-01 — 2026-01-01 — Example resolved finding

Delete this example. A real finding records what was learned and which briefs it affects.

Affects: example/brief-01
Resolved: yes
`

const initIntake = `# Intake

Front door for raw ideas. Same append-only + sequence rules as FINDINGS.

## I-01 — 2026-01-01 — Example intake entry

Delete this example. A real entry captures an idea and its disposition.

Disposition: scoped → example
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
stream: example
status: active
priority: P2
track: platform
---

# Example Stream

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
brief: example/01
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
sources: ["example"]
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

const initNextSteps = `
Scaffolded the streams structure. Next:

  1. Fill .assay-versions: replace the REPLACE_WITH_TAG / REPLACE_WITH_SHA256 tokens
     with a real statusgen release tag + per-platform sha256 (from that release's
     checksums.txt on medici-finance/assay). CI installs the pinned binary and
     refuses on a placeholder line, so this is the one required bootstrap step.
  2. Lint the set:         statusgen --root . --lint
  3. Generate the board:   statusgen --root .          writes STATUS.md at the repo root
  4. Replace docs/streams/example/ with your own stream, then delete it.
  5. Commit .github/workflows/assay-statusgen.yml — the two-half single-writer CI
     is already bootstrap-safe (porcelain STATUS.md guard, [skip-status-regen] marker).

STATUS.md has a SINGLE writer (main's CI): regenerate it locally freely to preview,
but never commit it on a branch. The scaffolded example stream keeps docs/streams/
non-empty, so --lint is green from the first commit — no --allow-empty-root needed.
`
