# derived-board — spec

**Status**: adopted for authoring 2026-08-22; the briefs in this stream implement it.
Open questions are listed at the end and each is owned by a brief.

## 1. The claim

A stream board's lifecycle cell is a **claim about the world** ("this brief's deliverable
is on main", "its Verify rows were run by someone else and passed"). Every such claim
already has a durable witness somewhere — a merged PR, a `verifyrun` log, an App review at
head, a human ruling in an Evidence row — and `docs/brief-rules.md` rule 30 already says a
claim with a machine-readable witness must not stay hand-asserted. Today that principle
covers `verified`/`done` only. This spec extends it to every state and removes the
hand-edited cell, so the board cannot disagree with its own witnesses.

## 2. The derivation

| Cell | Derived from | Witness the instrument must have looked at |
|---|---|---|
| `todo` | nothing else applies, **and the instrument looked** | PR search ran; no open/merged PR carries this brief's trailer; no witness |
| `in-progress` | an OPEN PR carries `Brief: <stream>/<NN>` | the PR (number, head SHA, draft/ready) |
| `implemented` | a MERGED PR carries the trailer | the merge commit SHA on the default branch |
| `verified` | rule 30 unchanged: `statusgen verifyrun --check` exit 0 by a non-implementer | the witness log |
| `done` | `verified` + (`gate: model` → App approval at head, the existing auto-flip; `gate: human` → a `human:<login>` Evidence row) | the review id / the Evidence row |
| `blocked` | the linked issue carries `question` / `needs-decision` / `help wanted`, or `blocked-by: env` | the issue + label |
| `unknown` | the instrument **could not look** (no network, API error, rate-limited, no trailer convention yet) | — the board prints WHY, per cell |

Rules that fall out:

- **`unknown` is a first-class cell.** A derived negative (`todo`) is legal only when the
  search ran and returned nothing. An offline `statusgen --lint` on a branch renders every
  PR-derived cell as `unknown (offline)` — it never renders `todo`. This is the three-state
  instrument invariant applied to the board itself.
- **Demotion is automatic, promotion is witnessed.** A PR re-opened after a revert, a red
  witness, a dismissed approval: the cell falls back to the highest state still witnessed.
- **The trailer is the only PR→brief edge.** No title parsing, no branch-name heuristics.
  A merged PR with no trailer is a lint finding (NOTICE during backfill, PROBLEM after),
  never a guess.
- **Hand edits to a generated table are a PROBLEM**, the same as hand-editing `STATUS.md`.

## 3. What stays hand-written

Brief frontmatter, unchanged in spirit: the authoring facts (`brief`, `title`, `why`,
`wave`, `depends`, `unblocks`, `effort`, `gate`, `risk`, `gate-why`, `issues`, `sources`,
`exec-tier`, `domain`, `parallel-streams`, `consumers`, `blocked-by`, `measures`) plus the
graph keys reserved by §5. The brief body (Context / Task / Verify / Evidence / Review).
Human rulings, as today, as `human:<login>` Evidence rows the human commits.

There is **no `status:` field in frontmatter and no sidecar `brief-NN.yaml`**. Both would
be a cache of GitHub state committed to git; a cache is what the phantom rows were.

## 4. Where derivation runs

- **Online** — the repo's `statusgen` regen workflow (push to main + schedule). It has a
  read-only token, runs `statusgen reconcile`, regenerates `STATUS.md` and every stream
  README's generated table, and commits (single writer, existing loop guards). On a
  schedule it opens a PR instead of pushing when the diff touches a stream README, so a
  human sees state change on the board, not only in `STATUS.md`.
- **Offline** — `statusgen --lint` on a branch: tree-only, PR-derived cells `unknown`,
  everything else (schema, edges, witnesses, hand-edit detection) as today.

## 5. brief-v2 — one flag-day, not two

**Provenance note.** Every design choice in this section is the authoring session's
proposal. None is a ruling until the human ratifies it — by merging this stream (the
merge is the ratification record for the spec as a whole) or by a `human:<login>`
Evidence row / linked decision issue for any item they want ruled separately. A bot
commit cannot stamp a ruling; this document does not try to.

The dependency-graph design note (private; re-staged by brief 01 as `docs/dependency-graph-design.md` (planned)) §3.6 left open whether its new keys (`gates:`,
`feathers:`, the ref grammar, `docs/streams/graph-repos.yaml` (planned)) land as optional brief-v1
keys or as brief-v2, because a v2 bump is a fleet-wide flag-day. This stream is that
flag-day regardless (a hand-edited surface becomes generated), so brief-v2 **reserves
those keys now**: parsed, type-checked, and lint-validated, gating behaviour deferred to
the graph stream. Reserving them costs one parser stanza; adding them later costs a
brief-v3 and a second migration. Concretely, brief-v2 =

- **`brief:` becomes hierarchical — `<cell>:<repo>:<stream>:<NN>`** (authoring proposal, 2026-08-22,
  pending ratification by the human's merge of this stream; the field keeps its name). Example: `assay:assay:derived-board:01` for this brief — cell
  `assay`, repo alias `assay` (the public repo), stream, number. The cell and repo alias come
  from `docs/streams/graph-repos.yaml` (planned), which becomes REQUIRED in a v2 tree (the repo
  segment is the registry ALIAS, never `owner/name` — ruled by the human, 2026-08-22:
  https://github.com/medici-finance/assay/pull/71#issuecomment-5381058997); `--lint`
  PROBLEMs a `brief:` whose cell/repo do not match the registry or whose stream/NN do not
  match the file path. References elsewhere (`depends`, `unblocks`, `gates.on`, the `Brief:`
  PR trailer) accept the full form and the elided forms (`<stream>:<NN>`, `<repo>:<stream>:<NN>`
  — each omitted prefix means "same as the declaring brief"); the file's own `brief:` is
  always the full form, so a brief is self-identifying when read outside its repo. The `/`
  separator of brief-v1 (`<stream>/<NN>`) is accepted on READ for the migration window and
  rewritten by the migration; v2 lint PROBLEMs it in a v2 file.
- **`version: <int>`** (authoring proposal, 2026-08-22, pending the same ratification) — the brief's own revision, `1` at authoring, bumped
  by every edit to Task or Verify after first dispatch (re-baselines bump it). Evidence and
  witness rows record the `version` they were run against, so a witness for version 2 of a
  brief whose Verify table is now version 3 renders as `unknown (witness for v2, brief is v3)`
  instead of `verified` — the stale-Verify-artifact class becomes visible on the board instead
  of in a verifier's could-not-check note. `--lint` PROBLEMs a Task/Verify diff on a branch
  whose `version` did not change.
- the rest of brief-v1's frontmatter, unchanged;
- `schema: brief-v2` required on every brief in a v2 tree (`--lint` PROBLEM otherwise —
  the fail-closed property the graph design wanted from a version bump: an old pinned
  statusgen REFUSES a v2 tree instead of silently ignoring it);
- reserved optional keys: `gates: [{on, type, reason}]`, `feathers: [...]`, with the
  §3.3 ref grammar (`<stream>/<NN>`, `<alias>:<stream>/<NN>`, `<alias>#<NNN>`, `#<NNN>`)
  and `docs/streams/graph-repos.yaml` (planned) (`schema: graph-repos-v1`) as the alias registry;
- the stream README's Briefs table is generated between markers; the README frontmatter
  gains `board: generated`;
- **reserved from an internal substrate design** (its §4.4 reaches the same conclusion —
  status is a fold over facts, not a field — and names the identity fields a lifecycle
  store needs): `id:` (a uuid minted once at authoring, never reused; the stable key a
  fact log or an executor can reference across renames and re-homes), `supersedes: []`
  (object lineage: split briefs, re-baselined briefs), and on each Verify row an optional
  `id` (`v1`, `v2`, …) and `target:` (the verify substrate, e.g. a sibling repo or a live
  cluster — the console-stream could-not-check-by-design class becomes a field instead of
  a NOTICE). All OPTIONAL under brief-v2; `--lint` validates shape only. `id:` is the one
  the author-brief skill starts minting immediately, because a uuid added later is a
  uuid with no history.

## 6. Versioning and the tooling bundle

statusgen and desk-tools ship from one umbrella tag; `.assay-versions` pins each artifact
separately, so an adopter *can* be on `statusgen v1.0.0` + `desk-tools v0.13.0`. The desk
tools that read briefs (`deskboard`, `deskpr`, `deskclaim`, `deskevidence`) would then
misread a v2 tree. Cheapest adequate control, in brief 06: (a) `--lint` PROBLEMs an
`.assay-versions` whose artifact tags differ; (b) every brief-reading desk tool checks the
tree's `schema:` and refuses `brief-v2` below `v1.0.0` with a one-line "upgrade" message.
No separate min-version matrix — one tag, one tree.

## 7. Migration (v0.13.0 → v1.0.0)

`deskmigrate` gains one declarative op, `statusgen-regen` (run the pinned statusgen's
`migrate brief-v1-to-v2 --root .`: rewrite `schema:` lines, wrap each README table in
generated markers, add `board: generated`), so the migration file stays declarative and
dry-runnable. The historical backfill — deriving `in-progress`/`implemented` from PR
history for rows that were hand-asserted — needs the GitHub API and runs as
`statusgen reconcile --backfill` in the regen workflow, producing a drift-report PR
(`hand-said` vs `derived`, per row) that a human reads before it lands. `upgrade-assay`
drives the first; the rollout brief drives the second, repo by repo.

## 8. Open questions → owners

| Q | Owner |
|---|---|
| Trailer form for issue-only work (no brief): `Issue: #N` vs `Closes #N` only | 02 |
| Whether `in-progress` requires a non-draft PR or any open PR | 03 (spec says any open PR; draft is the normal worker state) |
| Schedule cadence for the reconcile PR and its noise floor | 04 |
| Which desk tools are "brief-reading" and need the refusal | 06 |
| Whether the cross-repo feathering table is generated in this stream or the graph stream | deferred to the graph stream; brief-v2 only reserves the key |
| Whether cross-repo refs in `depends:` (now expressible) are GATING in v2 or reserved like `gates:` | 03 — ruled by the human, 2026-08-22 (https://github.com/medici-finance/assay/pull/71#issuecomment-5381058997): reserved — parsed, validated, reported; gating stays with the graph stream |
| Whether `id:` is minted for the 400+ existing briefs at migration time or lazily | 06 (proposed: minted by the migration op — a uuid added later is a uuid with no history) |
