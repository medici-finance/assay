# Changelog

All notable changes to this repository — the canonical, releasing home for the
shared Assay tools (statusgen, desk-tools, drainloop) and the `assay` plugin —
are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this repo versions the
whole umbrella with a plain `vX.Y.Z` tag (see `.github/workflows/release.yml`),
so one section covers every shipped tool at that version.

Every notable change records itself as one small **fragment** file under
`changelog/` (`changelog/<slug>.md`) BEFORE it merges — one file per PR, so
concurrent PRs never collide on a shared section (the `changelog-check` CI leg
enforces this; a genuinely non-notable PR carries the `changelog:skip` label). At
release time the release workflow AGGREGATES the fragments (sorted, deduped) into
a dated `## vX.Y.Z — <date>` heading here and into the published release notes,
then clears `changelog/` — descriptive highlights, never a raw commit list. See
`changelog/README.md`.

## Unreleased

Pending notable changes are recorded as one-file-per-PR fragments under
`changelog/` (see `changelog/README.md`), aggregated into a dated section
here at release time. This section is written only by the release workflow;
do not add highlight bullets to it directly.

### Added
- `statusgen enforcement-status` renders the live authoring-guidance rules the lint actually enforces — derived from the lint registry and reported three-state (enforced / not-enforced / could-not-check) so the coverage boundary is explicit — and a new `skillslint` `ENFORCEMENT-BLOCK` check compares that fresh render against the committed enforcement block in the authoring-guidance skill, failing closed when the two drift, so documented guidance can no longer silently diverge from what the lint enforces (mistake-proofing/04).

## v0.23.0 — 2026-09-02

### Added
- A model-capability floor gates authority-bearing desk writes: `deskflip`, `deskpost ready`, and `deskpost` review verdicts now refuse a session whose dispatch is attested below the strong tier — keyed on the dispatcher-applied model+tier label stamp (self-applied stamps are worthless), failing closed. Unattested (human / pre-attestation) lanes proceed with a notice, and an incident-recovery override is logged loudly (#278).
- A peer-auth desk-comms backbone lands (`tools/desk/internal/comms/`): a `cellmsg-v1` envelope that parses-or-refuses, ed25519 sender-identity assertions (mint/verify, single-use, TTL-bounded), and a compiled lane ACL that is deny-by-default — cross-cell reach and human-gate verbs ship refused until a recorded ruling (#276).
- New `qualgen/dorajoin` package: the DORA join — a quality denominator (durable-change volume) and a traced-CFR refinement reported alongside incident-based CFR, joined to a pluggable `DeliveryMetricsSource` (a file-based reference adapter ships in-tree) on PR number / merge SHA / stream-task-ID, three-state throughout and never emitting a bare traced rate without its trace-rate and evidence-tier split.
- The changelog discipline is ACTIVE: the changelog-check PR leg gates merges and release.yml refuses an empty Unreleased section, lifting highlights into the release body (#266 activation, #269).
- `deskcomms` gives the desks their client surface onto the local cell gateway: `send` runs a fail-fast preflight (reserved-verb → identity → parse → lane-ACL → bodycheck → ratelimit → mint → submit) that CALLS the same `internal/comms` parse/ACL the gateway re-runs authoritatively, then signs and submits; `poll`/`ack` read this session's own per-role mailbox (ack moves, never deletes). Sender identity comes from session context, never a flag; enforcement stays the gateway's; there is no local-spool fallback, so an unreachable gateway fails closed rather than fabricating delivery (#299).
- `deskpost` attaches mechanical verdict-time triage labels to agent PRs — a `size:S/M/L` class over changed lines (generated files excluded) and a three-state `surface:core/std` tier read from a repo's `.assay-surfaces` globs — advisory only (nothing gates on them; an unreadable surface is could-not-check, never assumed) (#277).
- `qualgen check <paths>` screens named files for brittleness signals (stronger-tier, add-coverage, coupling-partner, reference-rot) as an always-advisory, exit-0 pass over the mined M1/M2 families — the per-file complement to the corpus-wide mine (#275).
- `qualgen pr <n>` emits a generic per-touched-file risk-feature feed (hotspot percentile, traced defect density with its trace-rate, ownership top-share, missing coupling partners) as JSON — no weighting or combined outcome of its own, so a consumer's own config decides what to do with the numbers.
- `qualgen/attribution` implements M3 stage attribution: it assembles a deterministic, content-addressed dossier per traced defect and names the stage the defect escaped at — `spec` / `brief` / `implementation`, or `untraceable` when the provenance chain is broken (never binned into a stage) — plus a `review-escape` overlay naming the lanes that approved the inducing change; the stage call is judgment-classified and spot-auditable against the fixed dossier, with a pluggable provenance-linkage adapter (a generic commit→issue reference adapter ships as the default) and an append-only per-stage defect ledger correctable only by tombstone amendment.
- `qualgen` mines the instruction-brittleness M1 family for real: instruction reference-validity and doc↔code staleness now render into `QUALITY.md`'s trend view behind a new `--instruction-globs` flag, replacing the family's placeholder — and an unconfigured run reports could-not-measure, never a silent zero (#271).
- `qualgen`'s M4 session-forensics join lands: a pluggable `TelemetrySource` interface plus a file-based reference adapter (`qualgen/telemetry`), and a read-only join over the M1/M2 corpus (`qualgen/m4`) correlating harness telemetry (retries, refusals, …) against churn and defect outcomes, with three-state coverage reported beside every correlation — code only, no telemetry source wired in (quality/13).
- `skillslint` also emits an advisory context-budget `NOTICE` for any instruction file over a word threshold (3,000 for `SKILL.md`, 5,000 for `CLAUDE.md`), flagging context-bloat candidates. The NOTICE is advisory only — it prints to stderr and never changes the exit code.
- `skillslint` now runs a byte-level invisible-character / Trojan-Source lint over the instruction surfaces (`plugins/assay/skills/**/*.md`, and `.claude/skills/**/*.md`, `plugins/assay/resident-rules.md`, `CLAUDE.md` where present). It rejects — by Unicode category, not an enumerated blacklist that would miss members — the whole `unicode.Cf` format category (bidi controls and directional marks incl. LRM/RLM/ALM, zero-width, invisible math operators, soft hyphen, the Unicode Tag block used for LLM ASCII smuggling, and non-leading U+FEFF), the variation selectors (U+FE00–U+FE0F, U+E0100–U+E01EF, and the Mongolian free variation selectors U+180B–U+180D/U+180F), a curated set of other invisibles that are neither Cf/VS/Cc (U+034F combining grapheme joiner, the Hangul fillers U+115F/U+1160/U+3164/U+FFA0, the Khmer inherent vowels U+17B4/U+17B5, U+2800 braille blank, and the U+2028/U+2029 line/paragraph separators), the assigned Unicode Default_Ignorable_Code_Point property as its own durable property-based branch (so every assigned-DI codepoint flags even if reclassified out of a category), any C0/C1 control outside tab/newline/carriage-return, and invalid UTF-8 — each reported with file, line, column and codepoint. Unassigned/reserved Default_Ignorable and visible Zs space separators (ordinary space, NBSP, …) are deliberately left legal. Printable non-ASCII (accented text, arrows, box drawing, an emoji whose base glyph carries its own presentation) stays legal — the check targets invisibility, not foreignness. This catches the exact payload class a human reviewing the rendered text cannot see.
- `statusgen conform` validates brief frontmatter against a versioned, machine-readable brief-v1 contract (`schemas/brief-v1.json`, JSON Schema draft 2020-12) embedded in the binary — required keys, field types, and closed value sets, reported three-state (checked-clean / checked-failed naming file+field / could-not-check, fail-closed) and distinct from `--lint`'s methodology rules; a `schema:` marker newer than the embedded contract is a version mismatch, not a field error. `conform --emit-schema` prints the embedded schema so the artifact is reproducible from any pinned binary, and a source-side coverage test derives the required-key and value sets from the reference validator's own tables so the schema and validator cannot drift without CI failing.
- `statusgen` gains a DECLARED, fail-closed fixture-corpus exclusion: a directory that drops a `.statusgen-fixtures` marker at its root opts its whole subtree out of both the `--lint` link check (dead-link / backticked-path / identifier-dereference / register-ref) and `--corroborate`'s `human:<name>` stamp scan, so eval/fixture corpora of captured run-outputs stop redding on their legitimate forward-references. The exclusion is DECLARED (marker-only, never inferred from a path name or `testdata`/`fixtures` convention) and FAIL-CLOSED (no marker on disk → the subtree is scanned exactly as a live brief); live briefs are untouched.
- `tools/create-fleet-gitlab.sh` idempotently provisions the Assay fleet's seven per-role GitLab service accounts, memberships, and PATs, plus a project's protected-`main` and approval settings; paired with `docs/adopting-assay-gitlab.md`, the GitLab-profile adopter walkthrough (ci-config-project runbook, token custody, tier ladder) cross-linked from `docs/adopting-assay.md` (forge-gitlab/04, #288).

### Fixed
- The desk's CI-rollup readers now evaluate the LATEST run per check NAME, mirroring branch protection's own "latest run per context" rule, so a superseded run — an older CANCELLED predecessor, or a stale QUEUED orphan left by a push + pull_request double-trigger — no longer counts against a PR whose current run for that name is green. This lands as one shared `deskkit.LatestRunPerName` reducer called by all three surfaces — `deskflip`'s ready-flip gate, `deskboard`'s CI-state render, and `deskkit.ReduceCIVerdict` — so the flip gate and the board can no longer diverge on the same double-triggered PR (one flipping it ready while the other still renders it CI-fail). The gate is not relaxed anywhere: a name whose current latest run is red, cancelled, or pending still reddens or blocks (#282, #289).
- `statusgen --record`'s DORA-timing recorder no longer fails silently when its authenticated `gh` reads (restore episodes, PR lead times) all fail — it emits a loud, distinct `DEGRADED` signal naming the failed read and the substrate path, instead of returning a no-op indistinguishable from a healthy quiet day (so a persistently token-less `--record` CI can no longer leave `.dora-timing.jsonl` silently never accruing); still fail-open, never fabricates (#279).

### Changed
- Changelog highlights are now recorded as per-PR fragment files under `changelog/` instead of shared `## Unreleased` edits: `changelog-check` greens on a fragment (or `changelog:skip`) and refuses a direct `## Unreleased` edit, and the release workflow aggregates fragments into the dated section and release Highlights, then clears `changelog/`.
- The `assay:verify-desk` skill body gains three neutral verification-quality controls — derive-from-base-branch grounding (derive what should exist before reading the work), per-row fan-out for large Verify tables (≥4 risk-bearing rows run as isolated per-row sub-verifications), and Evidence↔Verify-row scope-traceability (unmapped verified work is flagged as invented scope) — plus an anti-gaming rule to re-derive expected values from the brief rather than the work under test.

## v0.22.0 — 2026-08-31

### Added
- CI grows five control legs (#255): a forge-surface control sweep, a leak-sweep
  pattern sweep, per-plugin shell suites, a gating skillslint leg, and a
  QUALITY.md render check — each exercising a control that `go build`/`go vet`
  alone would leave un-run.
- A quality trend view: churn-vs-durable, hotspot and brittleness reporting land
  behind a single-writer `QUALITY.md` (quality/01–06: #245–#248, #252, #254).
- A per-loop pool-width knob for the desk loops (#226).
- A deterministic verdict runner (#242).
- Roster-from-deployment resolution (#256).
- A PR-body self-containment scan (#227).
- `inbox --flow` / `--walk` / `--html` views (#225, #233).
- A two-role superseded lane for `deskclose` (#232).
- B-SZZ inducing-commit tracing plus derived defect metrics land in `qualgen`
  (#261).
- Spec-routing §8 spec-lifecycle enforcement: a linter and an authoring-owed
  emitter (#267).
- The CHANGELOG discipline itself — a per-notable-PR `## Unreleased` highlight
  line, with the release-time roll and its CI enforcement staged (#266).

### Fixed
- The board archives cleanly: statusgen now resolves streams under
  `docs/archive/` as known depends / unblocks / affects targets (#259), so
  archiving a finished stream no longer reds valid references from still-active
  work.
- The verify queue stops lying: `verifyloop` defers `blocked-until` briefs,
  buckets online-lane and human-gated work out of DISPATCH, and reads
  qualifier-carrying `## Verify (…)` headings (#251, #253, #257).
- Board regeneration no longer races on concurrent pushes: regen-push is
  serialized (#221).
- A latent drift-registry test red is fixed (#262): `statusgen`'s
  `blockingIssueLabels` is registered as a declared exception, greening the
  release-only (desk-tools) test leg.
- The archive fallback extends to the markdown link/backtick check (#264), so
  references into `docs/archive/` stay green there too.
- `muhar -j 0` auto-parallelism is capped at 2 mutants in flight (#268) — the
  release test leg is memory-bounded by construction; every mutation still runs.

### Changed
- `pr-shepherd` is de-housed into the `assay` plugin so adopters get it too
  (#234).
- GitLab forge support closes out with a forge tier matrix (#222, #230, #231).

### Consumer action
- Pin `statusgen` at ≥ this release to lint boards that reference archived
  streams under `docs/archive/` (#259).
