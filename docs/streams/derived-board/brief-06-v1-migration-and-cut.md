---
brief: assay:assay:derived-board:06
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
schema: brief-v2
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
version: 1
id: 0786889c-9b51-4b97-b02d-333c757cca9a
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

### Implementer note — v1.0.2 supersedes v1.0.1 as the upgrade target — 2026-09-10 opus-5[1m]-implementer

Umbrella **v1.0.2** was published on 2026-09-10 and is now the latest. It stands in exactly the
relation to v1.0.1 that v1.0.1 stood in to v1.0.0: it supersedes it as the version an adopter is
carried TO, and it does not supersede the flag day. This brief's deliverables are unchanged by it.
Measured rather than inferred, on this tree:

- **The migration file is untouched.**
  `examples/adopter-scaffold/migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` still spans
  v0.28.0 → v1.0.0 and still carries the whole brief-v1 → brief-v2 rewrite. v1.0.2 is a patch over
  the SAME brief-v2 contract, so there is nothing for a further migration to do.
- **An adopter on v1.0.0 or v1.0.1 has no migration to run**, only a re-pin. Measured:
  `deskmigrate --from v1.0.1 --to v1.0.2 --dry-run` → exit 0, `no migrations for v1.0.1 -> v1.0.2
  (clean no-op)`; `--from v1.0.0 --to v1.0.2 --dry-run` → exit 0, same clean no-op.
- **An adopter on v0.28.0 still gets the flag day**, running it on the way through rather than
  being skipped past it. Measured: `deskmigrate --from v0.28.0 --to v1.0.2 --dry-run` → exit 0,
  `1 migration(s)`, selecting `0001-v0.28.0-to-v1.0.0-derived-board`, planning the same 3 files;
  `git status --porcelain examples/` shows the dry-runs wrote nothing.
- **The pins move, the composition manifests accumulate.** `plugins/assay/paired-versions.yaml`
  re-pins both artifacts to v1.0.2 with all ten digests re-harvested from the v1.0.2 release's own
  checksum manifest (12 entries compared field-for-field, 0 mismatches, counting the two in the
  new scaffold manifest). `examples/adopter-scaffold/releases/v1.0.2.yaml` is ADDED rather than
  replacing `v1.0.1.yaml` or `v1.0.0.yaml`: both older manifests still have to resolve for a tree
  pinned at either, and the migration span still ends at v1.0.0. `plugin: "1.0.0"` is unchanged —
  it names the plugin version and is checked against `plugins/assay/.claude-plugin/plugin.json`,
  not against the umbrella tag. `check-paired-versions.sh` → exit 0 (pairing 1.0.0 == 1.0.0;
  single tag v1.0.2 across 10 pin lines; 10 digests 64-lowercase-hex).
- **End-to-end digest confirmation.** The published `statusgen-darwin-arm64` asset was downloaded
  and hashed locally; the hash equals the release's checksum-manifest entry AND the committed pin
  line (`316e1479…da018`), and the binary self-reports `v1.0.2`.
- **`qualgen` is deliberately still unpinned.** The release publishes five `qualgen` assets, as
  v1.0.1 did; neither manifest has ever carried a `qualgen` section, and adding one is a separate
  decision about the adopter front door rather than part of a re-pin. Recorded so the omission
  reads as a choice.

**Row 10, re-measured.** With the PUBLISHED, sha256-verified v1.0.2 `statusgen`:
`statusgen --root . --lint` → **exit 0, `LINT: PASS`**, NOTICEs only and no `PROBLEM` line. The
row's own "after migration" clause still belongs to this repo's flag-day migration, which remains
a separate change.

### Implementer note — v1.0.3 supersedes v1.0.2 as the upgrade target — 2026-09-10 opus-4.8[1m]-implementer

Umbrella **v1.0.3** was published on 2026-09-10 and is now the latest. It stands in exactly the
relation to v1.0.2 that v1.0.2 stood in to v1.0.1: it supersedes it as the version an adopter is
carried TO, and it does not supersede the flag day. This brief's deliverables are unchanged by it.
Measured rather than inferred, on this tree:

- **The migration file is untouched.**
  `examples/adopter-scaffold/migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` still spans
  v0.28.0 → v1.0.0 and still carries the whole brief-v1 → brief-v2 rewrite. v1.0.3 is a patch over
  the SAME brief-v2 contract, so there is nothing for a further migration to do.
- **An adopter on v1.0.0, v1.0.1 or v1.0.2 has no migration to run**, only a re-pin. Measured
  against a brief-v2 tree (this repo's own root): `deskmigrate --from v1.0.2 --to v1.0.3 --root .
  --dry-run` → exit 0, `no migrations for v1.0.2 -> v1.0.3 (clean no-op)`; `--from v1.0.0 --to
  v1.0.3 --root . --dry-run` → exit 0, same clean no-op.
- **An adopter on v0.28.0 still gets the flag day**, running it on the way through rather than
  being skipped past it. Measured: `deskmigrate --from v0.28.0 --to v1.0.3 --root
  examples/adopter-scaffold --dry-run` → exit 0, selecting `0001-v0.28.0-to-v1.0.0-derived-board`,
  planning the same 3 files; `git status --porcelain examples/` after the dry-runs shows only this
  PR's own edits, so the dry-runs wrote nothing. (The scaffold root itself carries `schema:
  brief-v1` briefs as the flag-day SOURCE, so a patch-span run against IT correctly refuses rather
  than reading as a completed migration — which is why the patch-span no-op is measured against a
  brief-v2 tree.)
- **The pins move, the composition manifests accumulate.** `plugins/assay/paired-versions.yaml`
  re-pins both artifacts to v1.0.3 with all ten digests re-harvested from the v1.0.3 release's own
  checksum manifest. `examples/adopter-scaffold/releases/v1.0.3.yaml` is ADDED rather than
  replacing `v1.0.2.yaml`, `v1.0.1.yaml` or `v1.0.0.yaml`: each older manifest still has to resolve
  for a tree pinned at it, and the migration span still ends at v1.0.0. `plugin: "1.0.0"` is
  unchanged — it names the plugin version and is checked against
  `plugins/assay/.claude-plugin/plugin.json`, not against the umbrella tag.
  `check-paired-versions.sh` → exit 0 (pairing 1.0.0 == 1.0.0; single tag v1.0.3 across 10 pin
  lines; 10 digests 64-lowercase-hex); `check-paired-versions.test.sh` → 16 passed, 0 failed.
- **End-to-end digest confirmation.** The published `statusgen-darwin-arm64` asset was downloaded
  and hashed locally; the hash equals the release's checksum-manifest entry AND the committed pin
  line (`8843fcfa…3753f`), and the binary self-reports `v1.0.3`.
- **`qualgen` is deliberately still unpinned.** The release publishes five `qualgen` assets, as
  v1.0.2 did; neither manifest has ever carried a `qualgen` section, and adding one is a separate
  decision about the adopter front door rather than part of a re-pin. Recorded so the omission
  reads as a choice.

**Row 10, re-measured.** With the PUBLISHED, sha256-verified v1.0.3 `statusgen`:
`statusgen --root . --lint` → **exit 0, `LINT: PASS`**, NOTICEs only and no `PROBLEM`-prefixed line.
The row's own "after migration" clause still belongs to this repo's flag-day migration, which
remains a separate change.
**Verify-table RUN 2026-09-10 — opus-4.8[1m]-verifier (non-implementer, verify-desk). NOT a sign-off.** Merged main `8d799c6bc026f675817bc3cfbfa81cad11efe052` (re-pin umbrella v1.0.2→v1.0.3), offline (`KUBECONFIG=/dev/null`, go1.26.5). Binaries built from the merged tree (the installed statusgen predates v1.0.0 and lacks `migrate`, per the implementer note). Frontmatter: `gate: human`, `risk {irreversible: yes, rest no}`.

| # | command | exit | observed | discharges |
|---|---------|------|----------|------------|
| 1 | `cd statusgen && go test . -run Migrate -count=1` | 0 | `ok` | Task 1 (migrate + idempotency) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run 'StatusgenRegen'` then `-run 'Migrat'` | 0 | ok, ok | Task 2 (statusgen-regen op) |
| 3 | `deskmigrate --from v0.28.0 --to v1.0.0 --root examples/adopter-scaffold --dry-run` | 0 | selects `0001-v0.28.0-to-v1.0.0-derived-board`; plans 3 files; `git status --porcelain examples/` = 0 (dry-run wrote nothing) | Task 3; dry-run purity |
| 4 | `deskmigrate` twice on a fresh scaffold copy | 0 | idempotent; each brief scores 1 on `schema: brief-v2` + hierarchical `brief:` id + `version: 1`; the README scores 0 (not a brief) | Task 1/3 idempotency |
| 4b | `deskmigrate` on a copy with `docs/streams/graph-repos.yaml` removed | 5 | `graph-repos.yaml is absent — the brief-v2 id form cannot be minted without the alias registry` | registry-required refusal |
| 5 | mutate `.assay-versions` to two differing artifact tags, `statusgen --lint` | 1 | `PROBLEM: .assay-versions: artifact tags differ …`; `LINT: FAIL` | same-tag pin lint reddens |
| 6 | `deskboard` built `-ldflags -X main.version=v0.13.0`, run on the migrated copy | 6 | `tree is brief-v2; this deskboard is v0.13.0; run assay:upgrade-assay` | brief-reading version gate |
| 7 | plugin/paired agreement + `bash plugins/assay/scripts/check-paired-versions.sh` | 0 | ok; `check-paired-versions: OK` — plugin 1.0.0 == plugin.json 1.0.0; single tag v1.0.3 across 10 pin lines; 10 sha256 well-formed | Task 5 (version bump + paired guard) |
| 8 | `! grep -nE 'v1\.0\.0 [0-9a-f]{64}' plugins/assay/paired-versions.yaml` | 0 | no `v1.0.0 <sha256>` line (tree pinned at v1.0.3); negated-grep passes | no hand-typed hash for the uncut tag |
| 8' | harvest-equality: each pin line's `<artifact> <tag> <sha256>` vs the PUBLISHED v1.0.3 release checksums manifest | — | COULD-NOT-CHECK (offline) — requires a network read of the published release checksums; not run per the offline envelope | see RISK-VALUE |
| 9 | `upgrade-assay --root <clean scaffold copy> --to v1.0.0 --dry-run \| grep -c 'What changed'` | 0 | count 1 — release-note `## What changed` prose surfaced before consent, above the migration list | Task 3 (release-note pre-consent) |
| 10 | `statusgen --root . --lint` (this repo's merged tree) | 0 | `LINT: PASS`, 0 PROBLEM | repo tree lints clean |

All executable rows PASS. The only non-executed row (8') is could-not-check by the offline envelope.

`RISK-VALUE: NAMED, NOT DERIVED — the 10 pinned sha256 digests @ plugins/assay/paired-versions.yaml:37-41 (statusgen) and :62-66 (desk-tools), all at tag v1.0.3` — **THE crux and the open question for the human card.** Deriving them means comparing each digest field-for-field against the PUBLISHED v1.0.3 release checksums manifest, a network read forbidden by the offline envelope. `check-paired-versions.sh` (row 7) confirms only shape + single-tag; by its own header it CANNOT distinguish a correct hash from a well-formed wrong one (the implementer's M3 mutation showed two real hashes swapped between platforms passes that guard, caught only by the harvest comparison of row 8'). **A model verifier offline cannot confirm the digests are right — the human must run row 8' against the published v1.0.3 checksums** (or accept the implementer's recorded end-to-end darwin-arm64 confirmation, itself a network act). A wrong digest fails-safe (every adopter install refuses), so this is availability-risk, not silent-corruption risk.
`RISK-VALUE: DERIVED — plugin major version = 1.0.0 @ plugins/assay/.claude-plugin/plugin.json:5 and paired-versions.yaml:26` — a hand-edited surface becoming generated is the first contract-breaking change, so a semver MAJOR; plugin == plugin.json enforced by check-paired-versions.sh.
`RISK-VALUE: DERIVED — migration span from v0.28.0 to v1.0.0 @ examples/adopter-scaffold/migrations/0001-...:3-4` — v0.28.0 is the source umbrella (latest at cut), v1.0.0 the flag-day target; selectable + idempotent (rows 3/4).
`RISK-VALUE: DERIVED — brief-reading version-gate floor = v1.0.0 (RefuseIfTreeV2BelowV1, tools/desk/internal/deskkit/briefschemagate.go; sibling statusgen/versiongate.go)` — brief-v2 is the first contract-breaking schema, cut as v1.0.0, so a STAMPED build strictly below v1.0.0 reading a v2 tree refuses (exit 6) while v1.0.0 is not gated; observed row 6.

**VERDICT: PASS (Evidence recorded) — NOT signed off, NOT flipped.** `gate: human` + `irreversible: yes`: a model records Evidence but does NOT sign off or flip. Status stays `implemented`; the human closes the verify-gate card after running rows 3 and 9 on a real adopter checkout, reading the release note, and discharging the RISK-VALUE open question by confirming the 10 v1.0.3 sha256 digests against the published release checksums (row 8'), which cannot be done offline.

**Findings (for the human gate):** (1) Verify-table sequencing nit: row 5 mutates `$TMPDIR/adopt/.assay-versions` in place and row 9 as written reuses the same dir → run strictly top-to-bottom, row 9 yields 0 (upgrade-assay correctly refuses a tree with no determinable umbrella); row 9 passes (count 1) on a clean scaffold copy, the intended tree. Suggest the table spell a distinct fresh copy for row 9. Not a code defect. (2) Row 8' (harvest-equality) is the one check a model verifier structurally cannot discharge offline — exactly the risk check-paired-versions.sh cannot see; the human gate must not skip it. (3) `go run` collapses non-zero child exits to 1; rows 4b/5/6 were run against tree-built binaries to read the true exits (5/1/6).

### Implementer note — v1.0.4 supersedes v1.0.3 as the upgrade target — 2026-09-10 opus-4.8[1m]-implementer

Umbrella **v1.0.4** was published on 2026-09-10 and is now the latest. It stands in exactly the
relation to v1.0.3 that v1.0.3 stood in to v1.0.2: it supersedes it as the version an adopter is
carried TO, and it does not supersede the flag day. This brief's deliverables are unchanged by it.
Measured rather than inferred, on this tree:

- **The migration file is untouched.**
  `examples/adopter-scaffold/migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` still spans
  v0.28.0 → v1.0.0 and still carries the whole brief-v1 → brief-v2 rewrite. v1.0.4 is a patch over
  the SAME brief-v2 contract, so there is nothing for a further migration to do.
- **An adopter on v1.0.0, v1.0.1, v1.0.2 or v1.0.3 has no migration to run**, only a re-pin.
  Measured against a brief-v2 tree (this repo's own root): `deskmigrate --from v1.0.3 --to v1.0.4
  --root . --dry-run` → exit 0, `no migrations for v1.0.3 -> v1.0.4 (clean no-op)`; `--from v1.0.0
  --to v1.0.4 --root . --dry-run` → exit 0, same clean no-op.
- **An adopter on v0.28.0 still gets the flag day**, running it on the way through rather than
  being skipped past it. Measured: `deskmigrate --from v0.28.0 --to v1.0.4 --root
  examples/adopter-scaffold --dry-run` → exit 0, selecting `0001-v0.28.0-to-v1.0.0-derived-board`,
  planning the same 3 files; `git status --porcelain examples/` after the dry-runs shows only this
  PR's own edits, so the dry-runs wrote nothing. (The scaffold root itself carries `schema:
  brief-v1` briefs as the flag-day SOURCE, so a patch-span run against IT correctly refuses rather
  than reading as a completed migration — which is why the patch-span no-op is measured against a
  brief-v2 tree.)
- **The pins move, the composition manifests accumulate.** `plugins/assay/paired-versions.yaml`
  re-pins both artifacts to v1.0.4 with all ten digests re-harvested from the v1.0.4 release's own
  checksum manifest. `examples/adopter-scaffold/releases/v1.0.4.yaml` is ADDED rather than
  replacing `v1.0.3.yaml`, `v1.0.2.yaml`, `v1.0.1.yaml` or `v1.0.0.yaml`: each older manifest still
  has to resolve for a tree pinned at it, and the migration span still ends at v1.0.0. `plugin:
  "1.0.0"` is unchanged — it names the plugin version and is checked against
  `plugins/assay/.claude-plugin/plugin.json`, not against the umbrella tag.
  `check-paired-versions.sh` → exit 0 (pairing 1.0.0 == 1.0.0; single tag v1.0.4 across 10 pin
  lines; 10 digests 64-lowercase-hex); `check-paired-versions.test.sh` → 16 passed, 0 failed.
- **End-to-end digest confirmation.** The published `statusgen-darwin-arm64` asset was downloaded
  and hashed locally; the hash equals the release's checksum-manifest entry AND the committed pin
  line (`56c67b3f…374dcc`), and the binary self-reports `v1.0.4`.
- **`qualgen` is deliberately still unpinned.** The release publishes five `qualgen` assets, as
  v1.0.3 did; neither manifest has ever carried a `qualgen` section, and adding one is a separate
  decision about the adopter front door rather than part of a re-pin. Recorded so the omission
  reads as a choice.

**Row 10, re-measured.** With the PUBLISHED, sha256-verified v1.0.4 `statusgen`:
`statusgen --root . --lint` → **exit 0, `LINT: PASS`**, NOTICEs only and no `PROBLEM`-prefixed line.
The row's own "after migration" clause still belongs to this repo's flag-day migration, which
remains a separate change.

### Implementer note — v1.0.5 supersedes v1.0.4 as the upgrade target — 2026-09-10 opus-5[1m]-implementer

Umbrella **v1.0.5** was published 2026-09-10T16:07:43Z by release run 34498522079 (`success`);
`refs/tags/v1.0.5` is an ANNOTATED tag object peeling to commit `0c72024`, the commit that run
built. It carries the header-keyed base-cell fix for the `--corroborate` pre-existing-stamp
exemption (#785) plus its fail-closed regression test (#788), and stands to v1.0.4 as v1.0.4 stood
to v1.0.3 — superseding it as the version an adopter is carried TO, never superseding the flag day.
**MIGRATION STILL NOT RUN HERE — pin bump only.** Measured:

- **Nothing to migrate on the patch span.** `deskmigrate --from v1.0.4 --to v1.0.5 --root .
  --dry-run` → exit 0, `no migrations for v1.0.4 -> v1.0.5 (clean no-op)`; `--from v1.0.0` → exit 0,
  the same. An adopter on v0.28.0 still gets the flag day: `--from v0.28.0 --to v1.0.5 --root
  examples/adopter-scaffold --dry-run` → exit 0, selecting `0001-v0.28.0-to-v1.0.0-derived-board`,
  planning the same 3 files; `git status --porcelain examples/` after shows only this PR's edits.
- **The pins move, the composition manifests accumulate.** `plugins/assay/paired-versions.yaml`
  re-pins both artifacts to v1.0.5, all ten digests re-harvested from the v1.0.5 release's own
  checksum manifest; `examples/adopter-scaffold/releases/v1.0.5.yaml` is ADDED and v1.0.4 demoted
  to a patch step alongside the older manifests. `plugin: "1.0.0"` is unchanged — it names the
  plugin version, checked against `plugins/assay/.claude-plugin/plugin.json`, not the umbrella tag.
  `plugins/assay/scripts/check-paired-versions.sh` → exit 0; its tests → 16 passed, 0 failed.
  `qualgen` stays deliberately unpinned: five assets published, no section here, as with v1.0.4.
- **End-to-end digest confirmation, and row 10 re-measured.** The published
  `statusgen-darwin-arm64` was downloaded and hashed; its hash equals the checksum-manifest entry
  AND the committed pin line (`12d82b58…fad4b`), and it self-reports `v1.0.5`. With it,
  `statusgen --root . --lint` → **exit 0, `LINT: PASS`**, NOTICEs only, no `PROBLEM`-prefixed line.

### Implementer note — v1.0.6 supersedes v1.0.5 as the upgrade target — 2026-09-11 fable-5.1-implementer

Umbrella **v1.0.6** was published 2026-09-11T00:20:58Z by release run 34545350818 (`success`);
`refs/tags/v1.0.6` is an ANNOTATED tag object peeling to commit `f91b72f`, the commit that run
built. `git log --oneline v1.0.5..v1.0.6` carries, first, the same-tag pin-lint scoping fix (#794)
— an exempt or non-umbrella artifact line no longer makes a whole `.assay-versions` unlintable —
then: the `--corroborate` absent-column fail-closed test (#788); write verbs C onto the resolver
(#783); `gl_api` hardening in the GitLab fleet-creation script (#787); the v1.0.5 re-pin (#791);
the channel-D `desk-tools-source` pin shape + GitLab queue-label parity (#797); the GitLab reviewer
write-path brief (#796) and its PAT auth + verdict-write tests (#800); refreshed `deskclose` specs
(#793); one Evidence row (#799); the Orca fourth dispatch arm in the worker-desk body (#802). It
stands to v1.0.5 as v1.0.5 stood to v1.0.4 — superseding it as the version an adopter is carried
TO, never superseding the flag day. **MIGRATION STILL NOT RUN HERE — pin bump only.** Measured:

- **Nothing to migrate on the patch span.** `deskmigrate --from v1.0.5 --to v1.0.6 --root .
  --dry-run` → exit 0, `clean no-op`; `--from v1.0.0` → the same. `--from v0.28.0 --to v1.0.6 --root
  examples/adopter-scaffold --dry-run` → exit 0, selecting `0001-v0.28.0-to-v1.0.0-derived-board`,
  planning the same 3 files; `git status --porcelain examples/` after shows only this PR's edits.
- **The pins move, the manifests accumulate.** `paired-versions.yaml` re-pins both artifacts to
  v1.0.6, all ten digests from the v1.0.6 `checksums.txt`; `releases/v1.0.6.yaml` ADDED, v1.0.5
  demoted to a patch step. `plugin: "1.0.0"` unchanged. `check-paired-versions.sh` → exit 0; its
  tests → 16 passed, 0 failed. `qualgen` stays unpinned, as before.
- **Digest confirmed end-to-end, row 10 re-measured.** The published `statusgen-darwin-arm64`
  hashes to the manifest entry AND the pin line (`b2f926cd…04fa1`), self-reports `v1.0.6`, and
  `statusgen --root . --lint` with it → **exit 0, `LINT: PASS`**, NOTICEs only, no `PROBLEM` line.

## Review
Gate: human (from frontmatter). The human records the ruling after running rows 3 and 9
on a real adopter checkout and reading the release note; then cuts `v1.0.0` via the
cut-release skill.
