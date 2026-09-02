# Records control and retention

Documents say what to do; records prove it was done. This page states, once, which
artifacts in this repo are records rather than documents, who may write each, how an
unintended alteration in one would be detected, what enforces that detection, and how
long each is kept. Every claim below names the artifact that enforces it — so a reader
can check any line in one command — rather than asking for trust. See
[`docs/registers.md`](registers.md) for the append-only register conventions this page
draws on, and [`docs/evidence-bundle.md`](evidence-bundle.md) for the export mechanism
referenced in the bundle row below.

## Record classes

| Class | Where it lives | Who may write it | How an alteration is detected | Enforced by | Retention |
|---|---|---|---|---|---|
| Register entries (findings, intake) | `docs/streams/findings/*.md`, `docs/streams/intake/*.md` | Any contributor, one new file per entry, via pull request | An entry file that has ever existed on `main` and is absent from the working tree is flagged (tombstone check against history); two files claiming the same ID are flagged (duplicate-id detection) | `registerIntegrityProblems` in [`statusgen/registers.go`](../statusgen/registers.go), run by `--lint` | Retained for the life of the repository. Withdrawal is a tombstone (flip the disposition/resolution field, keep the file) — never a deletion, and none is planned |
| Briefs and their `## Evidence` sections | `docs/streams/<stream>/brief-<NN>[-slug].md` | The brief body by its author, via pull request; the `## Evidence` section appended at implementation/verify time — a `verified` status requires it filled by someone who did not implement | Any edit is a normal git diff subject to pull-request review; a brief whose own recorded witness contradicts a `verified`/`done` claim is refused that status | Pull-request review gated by the brief's own `gate:` field; `witnessgate` in [`statusgen/witnessgate.go`](../statusgen/witnessgate.go) | Retained for the life of the repository in git history; a superseded brief is not deleted, only re-gated or marked stale |
| Stream READMEs — `Verified` / `Reviewed` cells | `docs/streams/<stream>/README.md` | Whoever flips the cell (reviewer for `Reviewed`, verifier for `Verified`), via commit or pull request | Normal git history; register references from a README row must resolve, and the generated views built from these tables are checked against their sources | `registerrefs.go` and `viewlinks.go` in `statusgen/`, run by `--lint` | Retained for the life of the repository; every prior state of a cell is visible in git history |
| Generated views (`STATUS.md`, reports) | `STATUS.md` at the repo root; generated report output | **Single writer**: main's CI only, on every push that touches a source. Branches never commit it, and a pull request whose diff touches `STATUS.md` is blocked | Any branch attempting to commit `STATUS.md` fails PR CI outright — a generated view cannot be hand-edited into agreement with a claim | [`.github/workflows/assay-statusgen.yml`](../.github/workflows/assay-statusgen.yml); see [`docs/lifecycle.md`](lifecycle.md) §"STATUS.md — a single-writer generated artifact" | The current file is regenerated on every qualifying push; every prior generated state is retained in git history, for the life of the repository |
| Execution witnesses | Inside a brief's `## Evidence` section, one per `## Verify` row: command, exit code, sha256 of combined output, date, runner identity, tree SHA | `statusgen verifyrun` only — never hand-authored. Runner identity is derived from the executing process and can never be supplied (no `--runner` flag; passing one is a usage error) | A witness recording a failed run blocks a `verified`/`done` claim on the same brief; witness *absence* is a lower severity (a NOTICE, not a block), because the entire pre-witness corpus lacks witnesses by construction | [`statusgen/verifyrun.go`](../statusgen/verifyrun.go) writes it; [`statusgen/witnessgate.go`](../statusgen/witnessgate.go) enforces it | Retained with the brief, for the life of the repository, in git history |
| Released artifacts and their checksums | GitHub Releases (statusgen, desk-tools binaries); a per-platform sha256 pin committed by each consumer (see [`examples/adopter-scaffold/.assay-versions`](../examples/adopter-scaffold/.assay-versions) for the format) | The release workflow only, on a pushed or dispatched semver tag | A moved or re-published tag is a silent substitution; the workflow's `guard` job refuses to let a tag move, and a consumer's committed sha256 pin catches a substitution at fetch/install time regardless | [`.github/workflows/release.yml`](../.github/workflows/release.yml) (`guard` job, tag-immutability) plus each consumer's own pin-and-verify step | Releases and their checksums are retained on the forge for the life of the repository; no disposition process removes a published release |
| Exported evidence bundles | Produced on demand by `statusgen --export-evidence <from> <to>`; not stored in this repo | Whoever runs the export command, at export time | The bundle is not itself a living record to detect alteration in — but a recipient can check any file inside it against the manifest's per-file sha256, and the export is deterministic (byte-identical for the same inputs) so two independently produced bundles for the same range are directly comparable | [`docs/evidence-bundle.md`](evidence-bundle.md) — bundle format, `manifest.json`, determinism | This repo sets no retention for an exported bundle; it is a **copy** of a date range of the record above, not the record itself, and whoever holds the copy owns its retention |

## The retention rule, as it stands

The rule already in force, stated plainly: **records here are retained for the life of
the repository.** Register entries are never removed — a full git history is kept, and
withdrawal is a tombstone (the file stays, its disposition changes), never a deletion.
There is no deletion process for any record class above, and none is planned. This is a
real retention rule; it has simply never been written down before this page.

This is a description of current practice, **not an approved retention policy** with a
stated period, a review cadence, or a disposition step at the end of one. This page does
not invent any of those. Doing so in a documentation brief — asserting a period, a
cadence, or a disposition process that no mechanism here actually runs — would be a
claim this repo cannot honour, the same failure mode as describing a company nobody
works at. If an approved retention policy with a stated period is ever adopted, it
replaces this section; until then, "kept for the life of the repository, deletion
undone by tombstone" is the honest statement of what happens today.

## What is visible versus what is impossible

Every detection mechanism named in the table above makes an alteration **visible** in a
pull request or a lint run: a missing entry is flagged, a contradicted witness blocks a
status, a moved tag is refused, a changed file breaks its checksum. None of them make an
alteration **impossible**. A history rewrite by an actor with the authority to
force-push is outside every mechanism this page names — it is a scope limit on git
itself, not a gap specific to this repo's tooling. Detection here means "a reviewer
reading the next pull request, or a lint run, will see it," not "it cannot be done."

## What this page does not supply

This page states this repository's own practice for the record classes it produces. It
does not supply an adopter's retention obligation. If a regulatory, contractual, or
legal-hold requirement sets a specific retention period, a specific disposition action
at the end of that period, or a specific hold procedure, that obligation is the
adopter's own to state and to implement — this page is not that statement, in the same
way [`docs/telemetry.md`](telemetry.md) §Retention states the telemetry posture that
exists today rather than a policy that does not yet exist. An adopter who needs a
retention policy with a period and a disposition rule should write one; the record
classes and enforcement mechanisms in the table above are what such a policy would need
to reference.
