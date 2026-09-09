---
brief: derived-board/06
title: "v1.0.0 — deskmigrate statusgen-regen op, the v0.28.0→v1.0.0 migration, paired-versions bump, same-tag pin lint, brief-reading tools refuse v2 below v1"
why: >-
  A hand-edited surface becoming generated is the first contract-breaking change the
  tooling has shipped; it has to be a major version with a migration an adopter can
  dry-run, not a release note. The same cut closes the bundle gap — statusgen and
  desk-tools pinned at different tags reading one tree — with a lint and a refusal
  instead of a version matrix nobody maintains.
wave: 3
depends: ["derived-board/04"]
unblocks: ["derived-board/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: >-
  The release cut is irreversible under immutable releases (v0.9.0 is a permanent
  tombstone for exactly this reason), and v1.0.0 is the version adopters will read as
  "stable". The human confirms the migration's dry-run output on a real adopter tree and
  the release-note prose, then pushes the tag; the brief prepares everything up to the tag.
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §6 (bundle versioning), §7 (migration)"
  - "tools/desk/internal/deskkit/migrate.go — migration format; only op today is ensure-line"
  - "tools/desk/cmd/deskmigrate/main.go, tools/desk/cmd/upgrade-assay/main.go, plugins/assay/skills/upgrade-assay/SKILL.md — the runner and the flow"
  - "plugins/assay/paired-versions.yaml (plugin 0.3.0 ↔ statusgen v0.13.0); examples/adopter-scaffold/.assay-versions"
  - "freshness-checked 2026-08-22 @ f78ea24 — latest release v0.13.0; the only migration in the tree is a test fixture"
exec-tier: strong
exec-tier-why: release mechanics under immutable releases; a wrong sequence burns a tag permanently
consumers:
  - "plugins/assay/.claude-plugin/plugin.json (version 1.0.0): fixed-here"
  - "plugins/assay/paired-versions.yaml: fixed-here"
  - "examples/adopter-scaffold/.assay-versions + its briefs (schema: brief-v2): fixed-here"
  - ".assay-versions of each consumer repo: follow-up derived-board/07"
---

# Brief 06 — v1.0.0 migration + cut preparation

## Context
files:
- `tools/desk/internal/deskkit/migrate.go` — new op `statusgen-regen: {verb: migrate,
  args: [brief-v1-to-v2]}`: runs the PINNED statusgen (resolved from `.assay-versions`,
  never `$PATH`) with `--root`; dry-run prints the statusgen dry-run; idempotent.
- `statusgen/migrate.go` (new) — verb `migrate brief-v1-to-v2 [--dry-run]`: rewrite
  `schema: brief-v1` → `brief-v2` on every brief; rewrite `brief: <stream>/<NN>` →
  `brief: <cell>:<repo>:<stream>:<NN>` using the tree's `graph-repos.yaml` (refuse, exit 5, if
  absent — the adopter writes the registry first; the release note says so); add `version: 1`
  where absent; mint `id:` (uuid v4) on every brief that lacks one (spec §8 — minted at
  migration, once); wrap each stream README's Briefs table in
  the markers and add `board: generated`; refuse (exit 5) if any README has no
  recognisable table. Prints a per-file plan under `--dry-run`.
- `migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` (planned) — the first REAL migration (source umbrella v0.28.0, the latest at cut time — medici-finance/assay#453) — `apply:` =
  `statusgen-regen` + `ensure-line` in `docs/UPGRADING.txt`; body = the adopter-facing
  release note (what changes on their board, the trailer they must now write, the
  reconcile step they must add to their workflow — with the exact YAML).
- `statusgen/main.go` + brief-reading desk tools (`deskboard`, `deskpr`, `deskclaim`,
  `deskevidence`) — on `schema: brief-v2` in the tree with a binary `< v1.0.0`: exit 6
  "tree is brief-v2; this <tool> is vX; run assay:upgrade-assay".
- `statusgen` `--lint` — PROBLEM when `.assay-versions` artifact tags differ.
- `plugins/assay/.claude-plugin/plugin.json` 1.0.0; `paired-versions.yaml` plugin 1.0.0,
  its statusgen/desk-tools pin held at the last REAL umbrella release (one tag, one tree,
  `check-paired-versions.sh` green — no hand-invented `sha256`); the v1.0.0 re-pin + the
  `sha256` harvest is the cut-release skill's post-tag step, since a hash for an uncut tag
  is never typed by hand; `examples/adopter-scaffold/` migrated.
- `docs/release-notes/v1.0.0.md` (new) — same prose as the migration body.

facts:
- Umbrella tag is bare `vX.Y.Z`; release.yml is draft → upload → publish under immutable
  releases; the tag push is the human's.
- `deskmigrate` selects migrations by `[from,to]` span; `upgrade-assay` re-pins
  `.assay-versions` then runs them; dry-run first, always.
- The pinned-binary resolution inside the new op must use the SAME code path
  `desk-install` uses (sha256-verified), so a migration never runs an unverified tool.
- The refusal in brief-reading tools is version-gated by the build stamp (`-ldflags`),
  so a local unstamped build behaves as "latest".

## Ground rules
- NEVER git push / trigger workflows / push a tag. Commit on the feature branch only.
- Stop at `implemented`; the human runs the cut (cut-release skill) after review.
- No sha256 is ever typed by hand.

## Task
1. `statusgen migrate brief-v1-to-v2` with dry-run + idempotency tests on a fixture tree.
2. `statusgen-regen` op in deskkit with pinned-binary resolution + tests (dry-run writes
   nothing; unknown verb refused).
3. The real migration file + release note; `upgrade-assay` fixture updated to exercise it.
4. Refusals in brief-reading tools + the same-tag lint, each with a test.
5. Version bumps; adopter-scaffold migrated; `docs/UPGRADING.txt` convention documented.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd statusgen && go test . -run Migrate -count=1` | `ok` |
| 2 | check | `cd tools/desk && go test ./internal/deskkit/ -run 'StatusgenRegen' -count=1 && go test ./internal/deskkit/ -run 'Migrat' -count=1` | `ok` |
| 3 | check | `cd tools/desk && go run ./cmd/deskmigrate --from v0.28.0 --to v1.0.0 --root ../../examples/adopter-scaffold --dry-run; echo rc=$?` | `rc=0`; output lists `0001-v0.28.0-to-v1.0.0`; `git status --porcelain examples/ \| wc -l` → `0` (dry-run wrote nothing) |
| 4 | check | `rm -rf "$TMPDIR/adopt" && cp -r examples/adopter-scaffold "$TMPDIR/adopt" && cd tools/desk && go run ./cmd/deskmigrate --from v0.28.0 --to v1.0.0 --root "$TMPDIR/adopt" && go run ./cmd/deskmigrate --from v0.28.0 --to v1.0.0 --root "$TMPDIR/adopt"; echo rc=$?; grep -c 'schema: brief-v2' "$TMPDIR"/adopt/docs/streams/example-service/*.md; grep -c -E '^brief: [a-z0-9-]+:[a-z0-9-]+:example-service:0[12]$' "$TMPDIR"/adopt/docs/streams/example-service/*.md; grep -c '^version: 1' "$TMPDIR"/adopt/docs/streams/example-service/*.md` | `rc=0` twice (idempotent); every brief `1` on all three greps |
| 4b | check | `rm -rf "$TMPDIR/adopt2" && cp -r examples/adopter-scaffold "$TMPDIR/adopt2" && rm "$TMPDIR/adopt2/docs/streams/graph-repos.yaml" && cd tools/desk && go run ./cmd/deskmigrate --from v0.28.0 --to v1.0.0 --root "$TMPDIR/adopt2"; echo rc=$?` | `rc=5`; stderr names `graph-repos.yaml` (registry required before ids can be rewritten) |
| 5 | check +mutation | `printf 'statusgen v1.0.0 aaaa\ndesk-tools-linux-amd64 v0.13.0 bbbb\n' > "$TMPDIR/adopt/.assay-versions" && statusgen --root "$TMPDIR/adopt" --lint; echo rc=$?` | `rc=1`; output contains `artifact tags differ` (breaks the one-tag-one-tree guard in `statusgen/main.go`'s `sameTagPinLint`; proves it reddens) |
| 6 | check +mutation | `cd tools/desk && go build -ldflags '-X main.version=v0.13.0' -o "$TMPDIR/deskboard-old" ./cmd/deskboard && "$TMPDIR/deskboard-old" --root "$TMPDIR/adopt"; echo rc=$?` | `rc=6`; stderr contains `tree is brief-v2` (breaks the brief-reading version gate with a sub-v1.0.0 stamp; proves it reddens) |
| 7 | check | `python3 -c "import json,yaml;p=json.load(open('plugins/assay/.claude-plugin/plugin.json'))['version'];y=yaml.safe_load(open('plugins/assay/paired-versions.yaml'));assert p==y['plugin']=='1.0.0' and y['statusgen']['tag']==y['desk-tools']['tag'];print('ok')" && bash plugins/assay/scripts/check-paired-versions.sh >/dev/null && echo checked` | `ok` then `checked` — plugin bumped to 1.0.0; paired-versions holds the last real umbrella release tag (one tag, one tree, check green). The v1.0.0 re-pin + `sha256` harvest is cut-release's post-tag step (medici-finance/assay#453) — never hand-typed here |
| 8 | check | `! grep -n -E 'v1\.0\.0 [0-9a-f]{64}' plugins/assay/paired-versions.yaml` | exit 0 (no hand-typed hash for the unreleased tag) |
| 9 | check | `cd tools/desk && go run ./cmd/upgrade-assay --root "$TMPDIR/adopt" --to v1.0.0 --dry-run \| grep -c 'What changed'` | ≥ 1 (release-note prose surfaced before consent) |
| 10 | check | `statusgen --root . --lint` | exit 0 on this repo's own tree after migration |

> **Row 8's premise retired at the cut.** Row 8 asserts that no `v1.0.0 <sha256>` pin line
> exists, because at authoring time v1.0.0 was UNCUT and any hash for it could only have been
> invented. v1.0.0 is now published, so the guard it encodes ("never hand-type a hash for an
> unreleased tag") has done its job and its literal form is now expected to match. The
> post-release form of the same property is row 8's successor, run and recorded in the Evidence
> below: every pin line's sha256 equals the one the published release's own checksum manifest
> carries for that artifact. Row 8 is kept, not deleted, so the pre-cut history stays readable;
> a verifier reads it together with this note.

## Evidence
<!-- appended at implementation time -->

### Implementer run — post-cut re-pin — 2026-09-09 opus-5[1m]-implementer, branch off main @ `077a93b`

**What this run covers.** The umbrella v1.0.0 release is published, so the two pins the brief
deliberately deferred ("the v1.0.0 re-pin + the `sha256` harvest is the cut-release skill's
post-tag step, since a hash for an uncut tag is never typed by hand") are now made from real
evidence. This is the IMPLEMENTER run and is NOT a verdict: a non-implementer verify is
separate, and the status cell is not advanced here — the frontmatter gate is human and
`irreversible: yes`, so the flip is the verify-gate's.

**Environment.** Own worktree off `refs/remotes/origin/main`, offline (`KUBECONFIG=/dev/null`);
no cluster or production endpoint contacted. The only network reads are `gh release download`
against the public release home. Binaries built from this tree (`statusgen`, `deskmigrate`,
`deskversion`) plus, where the pinned oracle matters, the PUBLISHED `statusgen-darwin-arm64`
asset downloaded from the v1.0.0 release and sha256-verified before it was run.

**Diff under test.** Two files: `plugins/assay/paired-versions.yaml` (statusgen + desk-tools
`tag:` v0.26.0 → v1.0.0; all ten per-platform sha256 values re-harvested) and
`examples/adopter-scaffold/releases/v1.0.0.yaml` (fixture placeholder digests → the real
linux-amd64 ones, the platform the bare artifact lines in the scaffold pin file name — the same
convention `examples/adopter-scaffold/releases/v0.28.0.yaml` uses). `plugin: "1.0.0"` is
unchanged and still equals `plugins/assay/.claude-plugin/plugin.json`'s `version`.

| # | Command (as written in the Verify table) | Exit | Result |
|---|---|---|---|
| 1 | `cd statusgen && go test . -run Migrate -count=1` | 0 | `ok` |
| 2 | `go test ./internal/deskkit/ -run 'StatusgenRegen'` then `-run 'Migrat'` | 0, 0 | `ok`, `ok` |
| 3 | `deskmigrate --from v0.28.0 --to v1.0.0 --root examples/adopter-scaffold --dry-run` | 0 | lists `0001-v0.28.0-to-v1.0.0-derived-board`; plan names 3 files; `git status --porcelain examples/` → 0 lines (dry-run wrote nothing) |
| 4 | `deskmigrate` twice against a copied scaffold | 0, 0 | idempotent; each of the two brief files scores 1 on all three greps (the stream README is in the `*.md` glob and correctly scores 0 — it is not a brief) |
| 4b | `deskmigrate` against a copy with the alias registry removed | 5 | refusal names the missing registry file: "the brief-v2 id form … cannot be minted without the alias registry" |
| 5 | mutate the copied pin file to two different artifact tags, then `statusgen --lint` | 1 | `PROBLEM: … artifact tags differ across pinned artifacts` — the one-tag-one-tree guard reddens |
| 6 | `deskboard` built `-ldflags '-X main.version=v0.13.0'`, run on the migrated copy | 6 | `deskboard: tree is brief-v2; this deskboard is v0.13.0; run assay:upgrade-assay` |
| 7 | plugin/paired-versions agreement + `bash plugins/assay/scripts/check-paired-versions.sh` | 0 | `ok` then `checked`. Guard output: pairing plugin 1.0.0 == plugin.json 1.0.0; single tag v1.0.0 across 10 pin lines; 10 sha256 values 64-lowercase-hex |
| 8 | `! grep -nE 'v1\.0\.0 [0-9a-f]{64}' plugins/assay/paired-versions.yaml` | 1 | MATCHES-BY-DESIGN — 10 pin lines. Premise retired at the cut; see the note under the Verify table and row 8′ below |
| 8′ | harvest equality: every pin line's `<artifact> <tag> <sha256>` compared field-for-field against the published release's checksum manifest | 0 | 10 pin lines checked, 0 mismatches; every tag is v1.0.0 |
| 9 | `upgrade-assay --root <copy> --to v1.0.0 --dry-run \| grep -c 'What changed'` | 0 | count = 1 (≥ 1) — the release-note prose is surfaced before consent, above the artifact deltas and the migration list |
| 10 | `statusgen --root . --lint` on this repo's own tree | 1 | NOT SATISFIABLE BY THIS PR — see below |

**Row 10 — not satisfiable here, and why.** The row asks for exit 0 on this repo's own tree
*after migration*. This repo's own flag-day migration is a separate change; this PR re-pins
version manifests and does not migrate this tree, so the row cannot be claimed either way from
here. What was measured, so the number is not mistaken for a regression: the lint exits 1 with
exactly ONE `PROBLEM`, in `docs/streams/harness-portability/brief-15-ci-wiring-harnesslint.md`,
for a backticked changelog-fragment path that no longer exists. It is PRE-EXISTING at this
branch's base — the fragment was removed by the release aggregation commit that is main's head,
and this PR's diff touches neither that brief nor that directory. Confirmed twice: with an
unstamped build from this tree AND with the published, sha256-verified v1.0.0 `statusgen`
binary, which reports the identical single problem. It is reported here as itself: a
checked-failed on a row this PR does not own, not a pass and not a defect introduced here.

**Provenance of every hash — the point of the whole change.** No digest in this diff was typed,
recalled, or taken from a local build. All ten were read out of the checksum manifest downloaded
from the published v1.0.0 release, and rewritten into the manifest by a script that keyed each
pin line on its own artifact name, so a transposition could not survive authoring. End-to-end
confirmation for one platform: the published `statusgen-darwin-arm64` asset was downloaded and
hashed locally, and that hash equals the manifest entry AND the committed pin line
(`a749c2f1…f5c28`); the binary then self-reports `v1.0.0`. `linux-arm64` stays deliberately
unpinned in both sections — v1.0.0 publishes no such asset, and the acquisition REFUSES rather
than guesses when a detected platform has no pin line.

**Fail-first — the guards were watched failing before they were claimed to pass.** Against a
scratch copy of the committed manifest:

| Mutation | `check-paired-versions.sh` | Harvest-equality (row 8′) |
|---|---|---|
| baseline (as committed) | 0 — `check-paired-versions: OK` | 0 — 10 checked, 0 mismatches |
| M1: one pin line left on the old tag | 1 — `FAIL pins span 2 tags, must be exactly one: v0.28.0 v1.0.0` | (not run) |
| M2: one hash upper-cased | 1 — `FAIL sha256 is not 64 lowercase hex` | (not run) |
| M3: two REAL hashes swapped between platforms | **0 — cannot see this class** | 1 — both lines reported as mismatches |

M3 is the reason row 8′ exists rather than leaning on the shipped guard. `check-paired-versions.sh`
checks shape and tag agreement offline and says so in its own header; it cannot tell a correct
hash from a well-formed wrong one, which is exactly the failure a hand-copied re-pin produces. The
harvest comparison against the published manifest is the check that catches it, and it is the one
a re-pin author owes.

**Additional targeted proof (beyond the Verify rows).** `go test ./cmd/deskversion/` → `ok`;
`go test ./internal/deskkit/ -run 'Composition|VersionMarker|Pins'` → `ok`; the companion shell
suite `plugins/assay/scripts/check-paired-versions.test.sh` → 16 passed, 0 failed. And
`deskversion --root examples/adopter-scaffold --releases examples/adopter-scaffold/releases`
still reports `state: known`, `umbrella: v0.28.0` (made of desk-tools v0.28.0 + statusgen
v0.28.0) — the scaffold's own pin is untouched by this change; only its v1.0.0 upgrade TARGET
manifest gained real digests.

**Not claimed.** No status cell is advanced by this PR. Rows 3, 4, 4b and 9 were run with the
statusgen-binary override pointed at a build from this tree, because the `statusgen` installed on
the running machine is older than v1.0.0 and has no `migrate` subcommand; run without the
override those rows fail on the environment, not on the code, and the human's row-3/row-9 run on
a real adopter checkout should use an installed, sha256-verified v1.0.0. A second wording note
for whoever re-runs them: rows 4b and 5 spell their determinate exit codes (5, 1) as the
BINARY's, and `go run` collapses a non-zero child status to 1 while printing `exit status 5` —
run those two rows against a built binary, or read the printed status rather than `$?`.

### Implementer note — v1.0.1 supersedes v1.0.0 as the flag-day target — 2026-09-09 opus-5[1m]-implementer

Umbrella **v1.0.1** was published hours after v1.0.0 and is now the latest. It supersedes v1.0.0
as the version an adopter is carried TO; it does not supersede the flag day itself, and this
brief's deliverables are unchanged by it. The distinction matters, so it is recorded rather than
inferred:

- **The migration file is untouched.**
  `examples/adopter-scaffold/migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` still
  spans v0.28.0 → v1.0.0 and still carries the whole brief-v1 → brief-v2 rewrite. v1.0.1 is a
  patch over the SAME brief-v2 contract, so there is nothing for a second migration to do.
- **An adopter on v1.0.0 has no migration to run**, only a re-pin. Measured:
  `deskmigrate --from v1.0.0 --to v1.0.1 --dry-run` → exit 0, `no migrations for v1.0.0 -> v1.0.1
  (clean no-op)`.
- **An adopter on v0.28.0 still gets the flag day**, running it on the way through rather than
  being skipped past it. Measured: `deskmigrate --from v0.28.0 --to v1.0.1 --dry-run` → exit 0,
  `1 migration(s)`, selecting `0001-v0.28.0-to-v1.0.0-derived-board`, planning the same 3 files;
  `git status --porcelain examples/` shows the dry-run wrote nothing. End to end,
  `upgrade-assay --root <copy> --to v1.0.1 --dry-run` → exit 0, deltas
  `statusgen v0.28.0 -> v1.0.1` and `desk-tools v0.28.0 -> v1.0.1`, with the release-note prose
  surfaced before consent.
- **The pins move, the composition manifests accumulate.** `plugins/assay/paired-versions.yaml`
  re-pins both artifacts to v1.0.1 with all ten digests re-harvested from the v1.0.1 release's
  own checksum manifest (12 entries compared field-for-field, 0 mismatches, counting the two in
  the new scaffold manifest). `examples/adopter-scaffold/releases/v1.0.1.yaml` is ADDED rather
  than replacing `v1.0.0.yaml`: the older manifest still has to resolve for a tree pinned at
  v1.0.0 and for the migration span that ends there. `plugin: "1.0.0"` is unchanged — it names
  the plugin version and is checked against `plugins/assay/.claude-plugin/plugin.json`, not
  against the umbrella tag. `check-paired-versions.sh` → exit 0 (pairing 1.0.0 == 1.0.0; single
  tag v1.0.1 across 10 pin lines; 10 digests 64-lowercase-hex).
- **End-to-end digest confirmation.** The published `statusgen-darwin-arm64` asset was downloaded
  and hashed locally; the hash equals the release's checksum-manifest entry AND the committed pin
  line (`152c0699…b1de6`), and the binary self-reports `v1.0.1`.

**Row 10 update.** At the v1.0.0 re-pin this row was recorded as not satisfiable, with one
PRE-EXISTING `PROBLEM` (a brief's backticked changelog fragment, deleted by the release that
consumed it — the class is filed, and open, as #722). That problem is no longer present on main.
Re-measured here with the PUBLISHED, sha256-verified v1.0.1 `statusgen`:
`statusgen --root . --lint` → **exit 0, `LINT: PASS`** (NOTICEs only). The row's own "after
migration" clause still belongs to this repo's flag-day migration, which remains a separate
change; what is now true, and was not before, is that the lint is green on this tree.

## Review
Gate: human (from frontmatter). The human records the ruling after running rows 3 and 9
on a real adopter checkout and reading the release note; then cuts `v1.0.0` via the
cut-release skill.
