<!-- GENERATED from plugins/assay/references/intake-lane-triage.md by deskinstall --harness cursor. Do not edit. -->
# Intake lane triage — layout, stamp and the four exits

<!-- assay:harnesslint non-matrix-reference — harness-neutral triage procedure for raw intake-register entries, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

This is the procedure the intake-desk applies to each raw entry in `docs/streams/intake/*`. The
`intake-desk` skill keeps the routing test (work or idea) resident and cites this file for the
rest; every rule below binds exactly as it did when it was stated in the skill body.

**Split-layout repos (issue-loop/15, opt-in per repo):** once a repo has migrated
`docs/streams/intake/` into the five disposition subdirs (`new/`, `decision-needed/`, `watching/`,
`completed/`, `rejected/` — `ls intake/new/` IS the triage board), a triage verb below is **one
commit that does both**: the frontmatter `disposition:` update AND a `git mv` into the matching
subdir. Never one without the other — a mismatch is what `--lint` catches. New entries file under
`intake/new/`; flat-layout repos (an append-only `INTAKE.md` too) parse unchanged.

**Stamp the triage in the same commit.** Alongside `disposition:`, set `triaged:` (the date, or
RFC3339), `triaged-by:` (the running loop's canonical name, or `driver` when a human triages by
hand — never a person's login; anything outside that closed set is a `--lint` PROBLEM) and
`triager-tier:` (`any` or `strong`). `statusgen --intake-exits --json` exports only stamped entries;
an older entry without a stamp is counted, never backfilled.

1. **Untriaged-age alarm** (issue-loop/07) → the intake-debt line NOTICEs entries past **3 days** in
   `disposition: new`. Draining that list is this desk's standing job — an untriaged front door is
   exactly the invisibility this loop exists to kill. **Work oldest-first.**
2. **The four triage exits** (issue-loop/08 — every entry leaves `new` as exactly ONE; a reason is
   never optional):
   - **`scoped → <stream>`** — becomes a brief. **Tier gate: triage only QUEUES authoring.** Brief
     authoring is design-tier work (author-brief model-tier gate); a cheap-tier triage session
     **never authors inline** — it marks the entry and the strong-tier author picks it up. If *this*
     window is strong-tier, author NOW — a brief is a draft PR, the yes-case of the reversibility test;
     notify ("proceeded, filed as `<repo>#<N>`"). Otherwise queue.
     - **Opening a NEW stream (not just a brief in an existing one):** scaffold a stream ONLY from a
       scoping doc whose header is `**Status:** approved` (spec/lifecycle-v1.md §8.1) — never from a
       raw idea, an issue thread, or a `draft`. The stream README carries a `spec:` pointing at that
       doc; the `stream-source` lint decides — you carry the pointer, not the judgement.
     - **At the active-stream cap** (`ASSAY_STREAM_CAP`, the `stream-cap` lint): there are **no net new
       active streams past the cap**. Scaffold the new stream `status: parked` (its briefs are kept but
       it is out of dispatch), and name in the PR body which active stream the desk proposes to park or
       archive to make room. A human decides the swap; the parked stream re-activates by a README flip.
   - **`scoped → issue #NN`** — operational / bug-shaped work → file a GitHub issue (label `bug`
     when bug-shaped, per the project's own convention); record the issue number. It then enters the
     issue lane above. Before cross-boundary filing or attaching a comment, apply R7 to
     internal locators in the title, body and evidence; use opaque role+number refs.
   - **`decision-needed`** — a call that is human:<name>'s BECAUSE it is one-way. A reversible fork
     instead exits `scoped` on its best-guess default, with the `needs-decision` issue linked as a
     NOTIFICATION. Requires filing (or already having) a
     `needs-decision` issue per brief-06's template, recorded in the entry's `decision-issue: <NN>`
     field. The intake view renders these at the top as "waiting on a human" — a **pointer** into the
     issue lane's decision queue, never a second queue.
   - **`rejected — <why>`** / **`watching`** — an explicit reason is required. A withdrawn entry is
     tombstoned in-place (`disposition: rejected`), never deleted (deleting trips `--lint`).
3. **Brainstorming ends in an INTAKE entry, never an ad-hoc docs/ dir.** If human:<name>
   brainstorms in this window, the exit is a `disposition: new` intake file for the next triage pass.
