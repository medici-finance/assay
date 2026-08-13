# statusgen

**Provenance**: a **history-free copy** — no upstream git history carried over.

Consumers do not vendor it — they install the pinned release binary and verify
its sha256 against their `.assay-versions` pin file.

Module path: `github.com/medici-finance/assay/statusgen`. The module path
intentionally differs from the repo slug.

## What it does

`statusgen` generates a `STATUS.md` from a repo's `docs/streams/*/README.md`
brief tables plus two registers, `docs/streams/FINDINGS.md` and
`docs/streams/INTAKE.md`. Each stream directory's `README.md` frontmatter and
brief-row table are parsed, findings are cross-applied to the streams they
affect (flagging affected briefs until resolved), and a single aggregated
view — including a priority/staleness-ranked "Next-up" batch — is emitted.
It supports three modes: default (write `STATUS.md`), `--lint` (run every
source check and build the view, but never read or write `STATUS.md` — the
PR-side gate), and `--check` (verify the committed `STATUS.md` byte-matches a
fresh regen — the drift gate). The model is **single-writer**: only one CI
job (typically triggered on push to the default branch) ever commits
`STATUS.md`; every other context (PRs, local runs) only reads or lints.

Both gate modes always end stdout with exactly one machine-parseable verdict
line — `LINT: PASS` / `LINT: FAIL <n> problem(s)` for `--lint`, `CHECK: PASS`
/ `CHECK: DRIFT <n>` for `--check` — so a caller reads the outcome from the
output itself instead of interrogating the exit code in a separate shell.

## Layout

This package is its own Go module, rooted at `statusgen/` (not at the repo
root) — see "Standalone layout" below for why.

```
statusgen/
├── go.mod, go.sum       # module github.com/medici-finance/assay/statusgen
├── *.go, *_test.go      # single `main` package
└── testdata/            # fixture repos used by the test suite
```

## Invocation

Run against any target repo root that follows the `docs/streams/` convention
(the target repo need not be this one — pass `--root` to point elsewhere):

```bash
cd statusgen

# Regenerate STATUS.md at the target repo root (default mode).
go run . --root /path/to/target-repo

# PR gate: run every check (streams, brief schema, findings, links) and build
# the view, but never touch STATUS.md.
go run . --root /path/to/target-repo --lint

# Drift gate: fail if the committed STATUS.md doesn't match a fresh regen.
go run . --root /path/to/target-repo --check

# consumers: gate — corroborate the routing claims of the briefs THIS BRANCH
# touches against its own three-dot diff (brief-rules.md rule 9).
go run . --root /path/to/target-repo --consumers --base origin/main
```

`--consumers` has three exit codes, and the third is the point of it:
**0** nothing was disproved · **1** a routing claim is contradicted by the diff ·
**2** COULD-NOT-CHECK — the diff could not be taken, or `--brief` named a brief this
diff carries no evidence about (the post-merge re-run). "Could not check" never
renders as "checked clean". Only entries this branch itself wrote are judged: one
unchanged since the merge-base was claimed by an earlier branch and reads
`UNCHECKED`, so touching a merged brief for any other reason cannot red-gate it.
To re-corroborate a merged brief, hand it the diff that made the claims —
`git checkout M^2 && … --base $(git merge-base M^1 M^2)` for its merge commit `M`.

Omit `--root` to default to `.` (run from the target repo's root instead of
passing the flag).

**Comparing `--lint` output across two tree states (e.g. "is this failure
pre-existing, or did my branch cause it?") — use a real worktree, not `git
archive`.** `git archive HEAD | tar -x` is the natural-looking way to get "the
tree at commit X", but the extraction has no `.git` directory at all, and every
check that reads git history (register-ID grandfathering, the register
tombstone/field-gutting guards, the human-stamp gate, claim filtering) then
runs degraded or is skipped outright — visibly, via a `NOTICE: git metadata
unavailable …` line and per-check NOTICEs, but the set of checks that ran is
still not the same set CI runs, so the PROBLEM/NOTICE count is not comparable.
Use a real worktree instead, which keeps `.git` and resolves
`origin/main` normally:

```bash
git worktree add /tmp/at-<sha> <commit-or-branch>
go run . --root /tmp/at-<sha> --lint
git worktree remove /tmp/at-<sha>   # when done
```

`statusgen --version` prints the release tag the binary was built from
(`statusgen/vX.Y.Z`), or `dev` for an unstamped local build. Consumers pin
statusgen by tag + sha256; this is how they can CHECK the pin rather than assume
it — a stale install otherwise produces a stale board with no signal. The tag is
stamped at link time by `release-statusgen.yml`
(`-ldflags "-X main.statusgenVersion=<tag>"`).

## Multi-root (a board that spans repos)

`--root` is **repeatable**. Give it more than once and statusgen emits **one
`STATUS.md` per root**, each filtered to that root's own streams:

```bash
cd statusgen

# One board per repo, each covering only its own streams.
go run . --root /path/to/repo-a --root /path/to/repo-b

# Lint both roots in one run — every check runs against every root.
go run . --root /path/to/repo-a --root /path/to/repo-b --lint

# Committed synthetic fixture, if you want to see it work without a second repo:
go run . --root .. --root testdata/stubroot
```

Rules, all of them load-bearing:

- **A stream lives in exactly one repo.** The same stream name under two roots
  is an authoring error and a **hard error** — never a silent merge. A silent
  merge would let one repo's rows overwrite another's, and a stream quietly
  vanishing from Next-up is invisible by construction.
- **Everything derived is per-root.** Findings, intake alarms, awaiting ages,
  per-brief staleness, Next-up counts, register integrity, the word-budget
  check and the link/register-ref lint all read *that root's* registers,
  historian and doc tree. Nothing is computed from the first root and applied
  to the rest.
- **Single-root is unchanged.** A 1-arg (or no-arg) run takes the original code
  path and produces byte-identical output; the multi-root driver only wraps it.
- **Sub-commands refuse multi-root.** `--dora`, `--trend`, `--code`,
  `--alarms`, `--bottleneck`, `--roadmap`, `--launch`, `--gate-scores`,
  `--verify-issues`, `--decision-issues`, `--scan-issues`, `--close-verify`,
  `--consumers`, `--register-links` and `init` are one-repo tools; with several `--root`s they
  exit 2 rather than quietly using the first. Run them once per root.
- **One verdict line.** A multi-root `--lint`/`--check` still ends with exactly
  one `LINT:`/`CHECK:` line, summing every root; each root's block on stderr is
  introduced by a `statusgen: === root <path> ===` banner.
- **Coverage is stated, not inferred.** The run ends with
  `statusgen: N/M root(s) completed without error` on stderr. The per-root
  banners are printed *before* each root is attempted, so on their own they
  enumerate configured roots, not successful ones — this line is how a
  partially-failed run is legible at a glance. It counts roots that exited 0, and
  does not distinguish "could not load" from "read fine, found problems".
- **Multi-root writes are not transactional.** Ownership problems (a duplicate
  stream name across roots) are caught in a pre-pass and write *nothing*. Any
  other failure is per-root: with a good first root and a failing second, the
  first root's `STATUS.md` has already been rewritten when the run exits
  non-zero. That is intended — root A's board is correct for root A — but it
  means a failed multi-root run can leave updated boards behind. Re-run after
  fixing; do not assume a non-zero exit means nothing was written.
- **A root that resolves to zero streams is a hard PROBLEM, not a silent
  pass.** A `docs/streams` that exists and reads cleanly but has
  no stream subdirectories (or only `findings`/`intake` registers) looks
  identical to a typo'd, renamed, or mid-restructure path — exactly the
  "invisible by construction" failure mode above — so it fails closed the
  same way a missing or unreadable `docs/streams` already does. Pass
  `--allow-empty-root` for a root that has genuinely adopted the methodology
  but not authored a stream yet; that downgrades the diagnostic to a NOTICE
  so the state stays visible instead of reading as clean.

`--version` is recognised only as the **sole** argument. `statusgen --version`
prints the tag; `statusgen --root . --version` is a usage error (exit 2) — a
version query combined with real work has no defined meaning, and the pin-check
contract only ever invokes it alone.

### `repo:` stream frontmatter

A stream README may declare the repo it belongs to:

```yaml
---
stream: my-stream
status: active
priority: P1
repo: medici-finance/assay
---
```

Optional — a single-repo house never needs it. When present it is **live**, not
decoration:

- validated for form (`<owner>/<name>`);
- every stream under one root must agree (one root is one repo);
- two roots may not declare the same repo;
- it is rendered as a banner under the `# Project Status` heading, and carried
  as a `repo` field on every `--gate-scores` JSON row so a cross-repo aggregator
  attributes a brief from the data rather than from whichever root it invoked.

A malformed or conflicting declaration is a hard `--lint` PROBLEM.

### The `testdata/stubroot/` fixture

`statusgen/testdata/stubroot/` is a committed, **populated, synthetic** second
root — one stream (`multiroot-fixture`), three briefs across three lifecycle
states, plus its own findings and intake registers. It is synthetic on purpose:
duplicate stream names across roots are exactly what multi-root rejects, so a
fixture copied from a live stream would demand the thing the tool forbids. Its
generated boards are gitignored, not committed.

## Standalone layout

The module lives **at `statusgen/`**, not at this repo's root. There is
(deliberately) no root `go.mod` or `go.work` in this extraction — that's a
decision for whoever assembles the rest of the assay repo (brief-v1
template, register-convention docs, etc.) around this package. Until then,
the standard way to build/test/vet this package standalone is to `cd` into
it first:

```bash
cd statusgen
go test ./...
go vet ./...
gofmt -l .   # expect no output
```

Running `go test ./statusgen/...` from a parent directory that has no
enclosing module will fail with "directory prefix statusgen does not contain
main module" — that's expected for a standalone nested module; `cd statusgen`
first (as the example CI workflow does).

## Cutting a release

Releases are built by `.github/workflows/release-statusgen.yml`, which has **two
entry paths**. `release-desk.yml` and `release-daily-harvest.yml` have the same
two, with the same `version` and `dry_run` inputs.

### `workflow_dispatch` — name a version in the Actions UI

Run the workflow from `main` with `version: v0.6.0` (the `statusgen/` prefix is
added for you). The tag is cut **inside the same run** that builds and
publishes, under GitHub's own auth: no local git credentials are involved, and
the run records who dispatched it.

**Set `dry_run` first.** It resolves the version, proves the tag is still free,
and runs the guard — then stops. Nothing is built, nothing is tagged, nothing is
published. Run it before committing to a version, because a tag that has
produced a release is **not recoverable**: `deskrelease` has no delete, force,
or move verb and never re-points an existing tag, and the rule is to fix
**forward** with the next patch version (`deskrelease --help`). A wrong cut
costs that version number permanently.

On a real dispatch the tag is pushed at the **last possible moment** — after the
guard has passed and the artifacts are built and checksummed — so a guard
refusal or a build failure leaves no tag anywhere and burns no version.

Why the tag is cut inside this workflow rather than by a separate "cut a tag"
one: a tag pushed by a workflow using the default `GITHUB_TOKEN` does **not**
trigger other workflows (GitHub's recursion guard). The separate-workflow design
therefore fails *silently* — the tag appears, nothing builds, nothing errors.

### Tag push — the original path, unchanged

**Tag main's head, from a checkout you have just fetched:**

```bash
git fetch origin
git tag statusgen/v0.6.0 origin/main
git push origin statusgen/v0.6.0
```

This remains the path for a release that needs an annotated tag of your own
composition — a `Release-Override:` waiver (below) can only be written this way.
The dispatch path composes its own annotation
(`statusgen <version> (dispatched by <actor>)`), so a dispatched tag can carry no
trailers.

### The guard, and what a refusal leaves behind

Before anything is built, the `guard` job runs
`tools/releaseguard`, which refuses a tag that is not a
sane release point — not on `main`, behind `main`, sorting below an existing
release, or shipping a `statusgen/` tree identical to the previous release's.
A refusal produces **no artifacts and no release**.

On the dispatch path there is nothing to clean up: the tag was never pushed.
Correct the version and dispatch again.

On the tag-push path the tag exists but published nothing, so it is free to
move:

```bash
git push origin :refs/tags/statusgen/v0.6.0   # drop it
git fetch origin && git tag statusgen/v0.6.0 origin/main && git push origin statusgen/v0.6.0
```

That is the *only* case in which a tag may be dropped and re-cut at the same
number. Once a tag has published a release, gap 1 below applies and the number
is spent.

**Dispatched tags are annotated.** The dispatch path creates the tag with
`git tag -a`, so `statusgen/v0.8.0` and later report `object.type == "tag"`
where `statusgen/v0.7.0` and earlier — pushed as lightweight refs — report
`commit`. One consequence worth knowing before you reach for a re-cut: a tag
created with `git tag -a` is an annotated object, and a re-cut guard reads the
ref and treats anything that is not a commit object as **unverifiable**. So a
re-cut aimed at an already-annotated tag fails loudly rather than nooping
quietly — nothing is written, and the tag does not move.

A deliberate release off an older line (a patch for a pinned consumer, cut from
a commit that is on `main` but no longer its head) waives the checks it trips —
explicitly, in an **annotated** tag, one trailer per check:

```bash
git tag -a statusgen/v0.4.1 <sha-on-main> -m "0.4.x patch
Release-Override: behind-main patch for consumers pinned to the 0.4 line
Release-Override: version-order 0.5.0 is already out on the main line"
```

Overrides are read only from an annotated tag object (`git tag -a`), never from
a commit message, and every waiver is echoed into the job log, the job summary,
and this repo's release history as a warning. `on-main` cannot be waived.

The **durable** record of a waiver is the tag object itself, not the job log —
Actions logs age out with the run, the tag does not:

```bash
git cat-file tag statusgen/v0.4.1     # the reason, as the releaser wrote it
```

If the releases API is unreachable when the guard runs, the job ERRORs and
nothing is published. That is deliberate — the guard fails closed rather than
publishing on a view it could not verify — and the recovery is to re-run the
workflow once the API is back. The release job is
idempotent (it creates the release if absent, otherwise re-uploads with
`--clobber`), so a re-run after any `ERROR` is safe and needs no cleanup.

### What the guard does not cover

It is **accident prevention, not an authorization boundary**. Four of the five
checks are satisfiable or waivable by the person cutting the tag, alone:

- `behind-main`, `version-order` and `tree-unchanged` are waivable outright, by
  a trailer that person writes. The 8-character minimum reason is a speed bump,
  not an approval — nobody else is asked.
- `version-order` and `tree-unchanged` are movable **even without a waiver**:
  both are measured against the tags on the remote, and anyone who can push a
  tag can delete one. Deleting the tag that sets the bar lowers the bar.
- `on-main` is the only check a releaser cannot satisfy unilaterally — it needs
  the commit merged to `main` through a PR. It is also unwaivable. That is the
  one real constraint here.

Three specific gaps worth knowing:

1. **Burned version numbers are re-cuttable.** `version-order` compares against
   what exists on the remote *right now* — remote tags unioned with published
   releases. A version whose tag **and** release have both been deleted leaves
   no trace in either, so the guard cannot see it and will pass a re-cut of the
   same number at a different commit. This is the same property that makes the
   legitimate `v0.2.1` re-cut work, and it is load-bearing; the cost is that
   burning the *highest* tag (the usual case — you burn the release you just
   cut) and re-using its number is not detected. Detecting it needs a durable
   burn record, which this tool does not have. **Re-cut with a fresh number,
   the way `v0.2.1` did — never re-use a burned one.**
2. **On the tag-push path, a tag cut at a commit that predates the guard skips
   it entirely.** For `on: push: tags`, GitHub resolves the workflow file from
   the *pushed ref's own tree*. Tagging a commit from before this guard merged
   runs that commit's version of `release-statusgen.yml`, which has no `guard`
   job, and publishes. This is inherent to Actions and cannot be fixed in-repo —
   only a tag protection rule or an org-level required check closes it. It bites
   exactly the population the guard targets, since a stale checkout is the
   likeliest thing to be sitting on a pre-guard commit.

   **The `workflow_dispatch` path does not have this gap.** A dispatched run
   resolves the workflow file from the branch it was started from (`main`), and
   the tag does not exist when the guard runs — it is materialised locally for
   the guard to judge and pushed only after it passes. So the guard that runs is
   always `main`'s current one, whatever commit is being released. Preferring
   dispatch is the practical mitigation for this gap.
3. **Consumer-side pinning is out of scope.** A release cut correctly from
   main's head, that simply does not yet contain a fix someone is waiting on,
   is a *sane release point* and the guard passes it — correctly. Whether a
   consumer should pin it is a question about the consumer's `.assay-versions`,
   and no release-time check can see it.

## CI

`.github/workflows/statusgen.yml` (repo root) is an example workflow for
adopters — and also serves as this repo's own CI. It runs `--lint` on every
pull request, and on push to `main` regenerates and commits `STATUS.md`
(guarded by a `[skip-status-regen]` commit-message check to avoid a
self-triggering loop). Adopters wiring this into their own repo should copy
the workflow and adjust the `docs/streams/**` path filters if their stream
docs live elsewhere.
