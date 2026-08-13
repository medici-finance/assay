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
		{"docs/streams/example/README.md", initExampleStream},
		{"docs/streams/example/brief-01-first-brief.md", initExampleBrief},
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

const initWorkflow = `# statusgen CI: lint every PR, regenerate the board on main.
# statusgen is its own Go module, so it runs from INSIDE its directory
# (` + "`cd statusgen`" + `) with ` + "`--root ..`" + ` pointing back at the repo root. Adjust the
# ` + "`statusgen`" + ` directory below only if you are running from a source tree at a
# different depth. If you have no ` + "`statusgen/`" + ` source tree — the normal case —
# install the sha256-pinned release binary named in ` + "`.assay-versions`" + ` and run
# ` + "`statusgen --root .`" + ` instead of the ` + "`go run`" + ` steps below.
# Vendoring the source is retired: a vendored copy is an unpinned fork.
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
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: cd statusgen && go run . --root .. --lint
  regen:
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: cd statusgen && go run . --root ..
      - name: commit STATUS.md
        run: |
          git config user.name  'statusgen'
          git config user.email 'statusgen@users.noreply.github.com'
          git add STATUS.md
          git diff --cached --quiet || git commit -m 'chore(status): regenerate [skip ci]'
          git push
`

const initNextSteps = `
Scaffolded the streams structure. Next:

  1. Generate the board:   (cd statusgen && go run . --root ..)         writes STATUS.md at the repo root
  2. Lint the set:         (cd statusgen && go run . --root .. --lint)
  3. Replace docs/streams/example/ with your own stream, then delete it.
  4. Commit .github/workflows/assay-statusgen.yml. If you have no ./statusgen source
     tree, swap its go-run steps for the pinned release binary (statusgen --root .)
     named in .assay-versions -- vendoring the source is retired.

statusgen is its own Go module — run it from inside its directory (cd statusgen) with
--root pointing at the repo root. STATUS.md has a single writer (main's CI): regenerate
it locally freely, but never commit it on a branch.
`
