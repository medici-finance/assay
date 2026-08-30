# Brief rules

The rules a `brief-v1` file must satisfy, each stated with its reason. The template is in
`brief-template.md`; this is why it looks the way it does. A validator (`statusgen --lint`)
enforces the machine-checkable ones; the rest are review-gated.

The brief-authoring device rules **B1-B10** in `docs/mistake-proofing.md` bind alongside
these. They govern the authoring *act* rather than the file's contents — how the brief gets
written, and what proves the Verify table can fail — and are stated once there, not here.

## Structure

1. **Load-bearing facts live in islands, not prose.** Frontmatter, tables, and Verify rows
   are the record; prose motivates but never solely carries a fact a script or reviewer
   needs. Reason: prose drifts and can't be parsed; a renamed brief with a prose dependency
   silently breaks.
2. **References are typed IDs** (`stream/NN`, `F-NN`, `I-NN`) — never fuzzy names ("the
   pricing brief") or arrow notation (`←12a`). Reason: a typed ID survives renumbering and
   greps cleanly.
3. **A brief is self-contained.** If executing it needs knowledge from another brief, link
   it in `facts:`/Read-first or state it in `depends:`. Reason: any single agent must
   execute a brief without the whole plan in context.

## Dependencies and waves

4. **`depends`/`unblocks` are typed and inverse; same-wave deps are forbidden.**
   `wave` is derived: Wave 0 = no deps, Wave N = depends only on briefs in waves < N
   (strictly earlier — depWave must be less than briefWave). A brief whose `depends:`
   points to another brief in the same wave breaks strict wave-layering and miscomputes
   the critical path; `statusgen --lint` reports violations as NOTICEs (advisory; will promote to PROBLEM once adopter repos are migrated). Reason: the wave layout is a
   projection of the dependency graph; keeping them consistent lets the board schedule work
   and compose waves as true parallelism boundaries.
44. **Every ordering/behavioural gate is a typed EDGE, never README prose alone.**
   A prerequisite that says "no X before Y," "blocked on," "must land before," or a
   behavioural "no CronJob brief starts before the loop skills file-and-exit" is a
   dependency the same as any `depends:`. If it lives only in prose, statusgen's Next-up
   (computed purely from the `depends:` graph) cannot see it and the standing worker pool
   dispatches straight past it — a silent premature-dispatch, the most dangerous blocker
   class because it is invisible to every Next-up consumer (a prior issue). Encode it as
   `depends:`/`unblocks:` (in-repo — authoring an OWNING brief for a behavioural gate where
   none exists, per the `<stream>/NN` owning-brief pattern) or a desk feathering row (cross-repo);
   the prose then survives as a caption on a real edge. `statusgen --lint` flags gate-shaped
   prose with no matching edge as an `ordering-gate` NOTICE (design: `docs/dependency-graph-design.md`
   §3.4); a per-line `<!-- graph: not-a-gate -->` waiver silences a false positive.
5. **Data-first, roughly equal-sized pieces.** Split anything too big (a brief doing two
   distinct roles becomes two). Reason: uneven briefs stall a wave and hide the real
   critical path. Rough test: if the Task needs more than ~5 steps or touches >2 subsystems,
   split at authoring time.
6. **Find and verify the REAL head of the critical path.** The critical path is the chain
   that, if it slips, slips everything. The recurring failure is sequencing work behind a
   step that is itself blocked by something not even in the path. For each "smallest
   unblocking move," check the code/config/tracker that it actually works — don't assume. If
   the proposed first step depends on something unproven, that something is the real head;
   say so.

## Verify

These three rules say what a row must BE. Rules 25-29 ("Row-runner discipline",
below) say what its command must not DO — the shapes that report a verdict
nothing measured.

7. **Verify rows must be runnable by someone who didn't do the work.** A row with no literal
   command and no expected exit/output is not a DoD item — it's a hope.
8. **Prose deliverables get PRESENCE gates; quality is the human gate (the honesty rule).**
   For docs/articles, executable checks verify that required elements *exist* (a file, a
   section, a token) — `wc -w ≥ N` or `grep -c` passes N words of garbage with the right
   tokens. State in the Verify section that it gates presence and the human review gate owns
   quality. Reason: posing grep rows as quality DoD is checkmark-DoD — the exact anti-pattern
   the system exists to catch.
9. **A brief that changes a SHARED VALUE enumerates its consumers and verifies the FLOW.**
   A shared value is anything another component reads: a party/identity, an env-var name, a
   config key, a field's *meaning*, a wire/JSON format, a default. Grep for every reader,
   list each with a disposition (fixed-here / follow-up `stream/NN` / out-of-scope why), and
   add a flow-level Verify row (end-to-end completion), not only a site-local one. Reason
   (the cost that forced it): a change correct at its own site — "the agent submits as its
   own party" — silently broke a downstream fallback that assumed the old coupling; the
   whole end-to-end flow died while every local check stayed green.

   **The list is a structured field, and every entry is a claim that gets corroborated.**

   ```yaml
   consumers:
     - "statusgen/consumers.go: fixed-here (routing parse + the corroboration mode)"
     - "docs/reports/daily/opmetrics.json: follow-up <stream>/41"
     - "tools/statusgen/: out-of-scope (frozen tree; consumers read the pinned release binary)"
   ```

   A `consumers:` entry is written by the implementer *about their own work*, which is
   exactly the class of self-assessed claim that can reach `implemented` while still
   unproven. So nothing treats an entry as true because it is present. `statusgen --consumers`
   judges each entry against the branch's own diff and reports it in three states:

   | State | Meaning | Effect |
   |---|---|---|
   | `CORROBORATED` | every path the `fixed-here` site resolves to is in the diff; `follow-up <stream>/<NN>` exists **and** references back | — |
   | `DISPROVED` | the diff contradicts the claim: a path the site names is absent from the diff, the target brief does not exist, `out-of-scope` names a file this same diff edits, or the entry states no routing at all | **exit 1 — the gate** |
   | `UNCHECKED` | the evidence is not visible here: an out-of-repo path, a component named in prose, `out-of-scope` whose truth is a judgement no diff settles, or an entry **inherited unchanged from the merge-base** (some earlier branch made that claim) | printed and counted, never a pass |

   Site grammar, as the gate reads it:
   - a **directory** site (`statusgen/`) is touched when anything under it is — and because
     that makes a trailing slash the cheapest way to widen a claim, a corroborated directory
     site **names the paths it rests on and how many there are**. `docs/: fixed-here` resting
     on one file and `docs/brief-rules.md: fixed-here` naming that file are both honest, but
     they are no longer indistinguishable in the log: the reviewer can see how thin the
     evidence is and ask for a narrower site;
   - a **glob or brace** site names a SET and claims the whole set — `docs/streams/*/README.md:
     fixed-here` is disproved if one README is untouched. Fixed part of a set? Split the entry.
     (Passing on a single match made widening the site the cheapest way to green a claim.)
   - the routing token is matched case-insensitively and however it is spaced, so a doubled
     space or a capital letter cannot quietly demote a checkable claim to `UNCHECKED`;
   - an entry with no routing token is `DISPROVED` by the gate — that needs no diff to settle,
     and leaving it unchecked would make unreadability the cheapest way past the gate. Pointing
     at another repo is still fine, it just has to route: `"[other-repo] path/x.md: out-of-scope
     (tracked in other-repo#NN)"`.

   Three rules follow from that table:
   - **The brief carries a Verify row that runs the corroboration** —
     `statusgen --consumers --brief <stream>/<NN>` — so the claims are re-run by whoever
     verifies, not asserted once by whoever wrote them.
   - **The `UNCHECKED` residue belongs in that row's "Expect" column**, in words a reviewer
     can weigh ("the two out-of-scope exclusions are judgements — reviewer confirms the
     frozen tree really has no live reader"). It does not get restated as fact in the
     frontmatter, where it would read as settled.
   - **After the merge, the claims need the diff that made them.** `--brief` lifts the diff
     SCOPING, not the need for a diff: run on merged main it exits **2 (COULD-NOT-CHECK)**,
     because HEAD equals the base and nothing there is evidence either way. To re-run the
     claims, give it the branch: `git checkout M^2 && statusgen --consumers --brief <id>
     --base $(git merge-base M^1 M^2)` for the PR's merge commit `M`. Exit 2 is deliberate —
     "could not check" must never render as "checked clean".

   What the gate does **not** establish: that the list is COMPLETE. It judges the entries
   that were written; a consumer left off the list is invisible to it, and the run says so on
   every invocation. Completeness is still the author's and reviewer's job.

   `statusgen --lint` runs the offline half over the whole corpus: it hard-fails only on a
   claim the stream tables *disprove* (a `follow-up` naming a brief that does not exist),
   and NOTICEs everything a diff would be needed to settle — because a corpus-wide lint
   runs against briefs whose diff is long gone, and an instrument must not report an
   answer it cannot establish. A brief already merged is never rewritten to satisfy the
   gate: its entries are inherited, so editing it for any other reason cannot red-gate CI.

## Evidence

> Rule numbering in this file is **append-only**: rules are cited by number from other docs
> and skills, so a rule added to an earlier topic takes the next free number rather than
> renumbering everything after it. Hence 36–37 sitting under Verify.
>
> **These two were 25 and 26 until 2026-08-13, and the renumber is the repair of a
> numbering-space collision, not a style change.** Two briefs allocated out of this space in
> parallel; each diff was internally consistent, neither contained the other's number, and
> git merged them cleanly — so `^25.` and `^26.` each appeared twice in the merged file and
> "brief-rule 26" resolved to two different rules. The Row-runner block keeps 25–29 because
> that is where the live citations land — rule 7's own preamble, and `desk-hardening/01`
> citing "brief-rules 26/28" for the alternation and shredded-cell rules; the Evidence pair
> moved to the next free numbers above the maximum. `statusgen mergecheck` now detects this
> class before the merge and `statusgen --lint` NOTICEs it after (rule 40).

36. **The `## Evidence` section is a LOG OF RUNS, and each entry is an EXECUTION WITNESS.**
    `statusgen verifyrun --brief <path>` executes each Verify row's Command in a fresh
    subshell at the repo root and appends one witness row per Verify row:

    ```
    | # | Command | Result | Output | Date | Runner |
    |---|---------|--------|--------|------|--------|
    | 1 | `go test ./... > /tmp/out 2>&1; echo rc=$?` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | human:alex @ b988d1753038 |
    ```

    | Column | What it holds |
    |---|---|
    | `#` | the Verify row this witnesses, by its own `#` cell |
    | `Command` | the command **as authored in the Verify table** — code-spanned, pipes still `\|`-escaped, so a later run can prove the row has not been edited since |
    | `Result` | `<state> exit=<code>`, where state is `pass`, `fail` or `could-not-run`, and the code is `-` when nothing executed |
    | `Output` | `sha256:` + the first 12 hex of the sha256 of the combined stdout+stderr — a fingerprint two people can compare, **not** a tamper-proof seal |
    | `Date` | `YYYY-MM-DD` |
    | `Runner` | `<identity> @ <12-char HEAD SHA>`, the SHA suffixed `+dirty` when the working tree was modified and `+unknown` when cleanliness could not be determined |

    Rules that follow from it being a log:

    - **Runs APPEND; they never overwrite.** An implementer's run, an independent verifier's
      re-run, and a re-verify at a later SHA are three separate facts, and `statusgen` unions
      every table in the section. Rewriting the section in place would let a green re-run
      erase a red one — editing the recorded basis of a past sign-off, which is the
      falsification the whole mechanism exists to catch.
    - **The runner is DERIVED from the executing identity, never supplied.** `GITHUB_ACTOR`
      under GitHub Actions, otherwise the repo's git identity — a bot recorded verbatim
      (`assay-reviewer-app[bot]`), a person as `human:<name>`. There is deliberately no flag,
      and passing one is refused with its own message: *a witness you can caption is a witness
      you can forge.* When no identity can be derived, `verifyrun` writes nothing and exits 2
      rather than emitting an unattributed row.
    - **A witness is evidence, not an attestation.** Whoever controls the process controls the
      environment it reads. What makes it worth something is that it lands in a PR diff, next
      to the tree SHA it names, where a second person can re-run the command and compare
      hashes. It does not replace rule 7's independence requirement — that a `verified` stamp
      rests on rows re-run by someone who did not do the work — it makes it checkable.
    - `statusgen --lint` NOTICEs a `verified`/`done` brief whose Evidence has no witness for
      one or more Verify rows, rolled up one line per stream. **NOTICE, not a hard error**, for
      this phase: every brief closed before the mechanism existed lacks witnesses by
      construction, and hand-writing them in to green the gate would manufacture precisely the
      evidence the witness replaces. It flips to a hard problem once the active streams are
      backfilled — the same phased path `gate-why` took.

37. **The witness result is THREE-STATE, and `could-not-run` is never `pass`.** This is the
    three-state instrument invariant (§ *Three-state instrument invariant* below) applied at
    the row level:

    | State | Meaning |
    |---|---|
    | `pass` | the row ran and satisfied every constraint readable from its Expect cell |
    | `fail` | the row ran and contradicted one |
    | `could-not-run` | **no verdict was produced** — command not found or not executable, an unsubstituted `<placeholder>`, a timeout, or a row whose Expect declares a numeric floor the output carries no number to compare against |

    `verifyrun --check <path>` re-reads the section and exits **0** (every row witnessed,
    matching and passing), **1** (a witness records a failure), or **2** (a row has no witness,
    its witness records `could-not-run`, or the witness records a command the Verify row no
    longer carries). Three distinct codes, and 1 and 2 must not be collapsed by a consumer:
    1 says *the work is wrong*, 2 says *the instrument did not look*, and they prescribe
    different actions. Where both are present the run reports 2 — the weaker state of
    knowledge wins, because the failure count means nothing until the check is repaired.

    Two consequences worth stating outright:

    - A witness whose Command no longer matches the Verify row is `could-not-run`, not `pass`.
      Otherwise editing a row after running it would be the cheapest way to keep a green
      witness.
    - `could-not-run` contains the substring `not-run`, which is what the UNRUN derivation
      already reads. That interlock is deliberate: writing a `could-not-run` witness for a row
      **does not** clear that row's UNRUN state on the board. A row nobody could run cannot be
      laundered into coverage by recording that nobody could run it.

    What this does **not** establish: `could-not-run` is a LOWER BOUND on the class. A shell
    reports only 126/127 distinctly, so a row whose fixture is missing and whose command
    reports that as exit 1 is recorded `fail`. And a row whose Expect is pure prose is decided
    on its exit status alone — the run says so on that row, and the output hash is what a
    reviewer weighs for the rest. `verifyrun` narrows the unproven set; it does not close it.

## Derived status cells

Rules 36-37 say what the Evidence record IS. The rules here say what every lifecycle
cell on a stream board may claim on top of the witnesses that already exist, and who
repairs the board when a cell and its witness disagree. The lifecycle vocabulary is
exactly `todo`, `in-progress`, `implemented`, `verified`, `done`, `blocked`, and
`unknown`; an `unknown` cell carries its reason in parentheses on the board.

The derivation contract, one row per cell — rule 30 is the principle it generalizes,
rules 46-47 the edges it rests on:

| Cell | Derived from | Witness the instrument must have looked at |
|---|---|---|
| `todo` | nothing else applies, **and the instrument looked** | PR search ran; no open/merged PR carries this brief's trailer; no witness |
| `in-progress` | an OPEN PR carries `Brief: <stream>/<NN>` (rule 46) | the PR (number, head SHA, draft/ready) |
| `implemented` | a MERGED PR carries the trailer | the merge commit SHA on the default branch |
| `verified` | `statusgen verifyrun --check` exit 0, the rows re-run by a non-implementer (rule 30) | the witness log |
| `done` | `verified` + (`gate: model` → App approval at head; `gate: human` → a `human:<login>` Evidence row) | the review id / the Evidence row |
| `blocked` | the linked issue carries `question` / `needs-decision` / `help wanted`, or the brief carries `blocked-by: env` | the issue + label |
| `unknown` | the instrument **could not look** (no network, API error, rate-limited, no trailer convention yet) | — the board prints WHY, per cell |

**`unknown` is a first-class cell.** A derived negative (`todo`) is legal only when the
search ran and returned nothing. An offline `statusgen --lint` on a branch renders every
PR-derived cell `unknown (offline)` — it never renders `todo`. This is the three-state
instrument invariant (§ *Three-state instrument invariant*) applied to the board itself.
**Demotion is automatic, promotion is witnessed.** A PR re-opened after a revert, a red
witness, a dismissed approval: the cell falls back to the highest state still witnessed;
nothing advances a cell but the witness its row names.

30. **Every lifecycle Status cell is DERIVED from its witness, not asserted by whoever
    edited the table — and a cell whose witness goes red falls back to the highest state
    still witnessed** (the derivation table above; the `verified`/`done` mechanics in
    detail here). A board cell is a claim about the world, and every such claim already
    has a durable witness somewhere: an open or merged PR carrying the brief's `Brief:`
    trailer, a `verifyrun` log, an App approval at head, a human ruling in an Evidence
    row, a labelled blocking issue. A claim with a machine-readable witness must not
    stay hand-asserted, or the board can disagree with its own evidence and nothing
    notices. The `verified`/`done` rows have had their machine-readable form since rule
    36 — `statusgen verifyrun --check <brief>` exits 0 — and `verified` reverts to
    `implemented` when its witness goes red. Three states, three treatments:

    | `verifyrun --check` | What the cell may say | Enforced by |
    |---|---|---|
    | exit 0 — every row witnessed, matching, passing | `verified` / `done` | — |
    | exit 1 — a witness records a FAILURE | `implemented` (at most) | `statusgen --lint` PROBLEM on a closure the branch made; NOTICE when inherited |
    | exit 2 — no witness, stale witness, or `could-not-run` | `implemented` (at most) | `statusgen --lint` NOTICE, rolled up per stream (rule 36) |

    **Scoped to the transition, and phased on purpose.** A cell already `verified`/`done` at
    the merge-base was closed by an earlier branch, so it is grandfathered to a NOTICE — an
    unrelated PR must not inherit somebody else's red. And exit 2 is a NOTICE rather than a
    PROBLEM in this phase because it describes the inherited corpus, not a live act: measured
    2026-08-13, 319 of 320 brief files carried no witness for any row (the mechanism landed
    that day) while **zero** rows anywhere recorded `fail`. Absence is the backlog;
    contradiction is always something a run just wrote. Exit 2 promotes to a PROBLEM once the
    active streams are backfilled — the same phased path `gate-why` and the unfailable-row
    lint took.

    **The override is the demotion, and it is not a bypass.** A brief whose witness is red is
    never stranded: set the cell to `implemented` and the check releases in seconds, leaving
    an audit row in the diff. What changed is that the repo stopped making a claim it could
    not support. There is deliberately no flag, label, env var, or commit-message token that
    suppresses the check — a suppression a worker can apply to their own branch would make
    the cell asserted again, one level up.

    **What this does not settle.** A green witness is not independence: rule 36 still requires
    that a `verified` stamp rest on rows re-run by someone who did not do the work, and
    `verifyrun` records who ran them precisely so a reviewer can check that. And the
    `verified` → `done` step for a `gate: human` brief is a human sign-off, unchanged by this
    rule; a derived demotion can take a cell back to `implemented`, it never advances one.

31. **A PR that turns a neighbouring brief's witness red carries the re-baseline, or says in
    its body that it does not.** The brief whose table goes stale is usually *not* the brief
    the PR is about — a refactor moves a path some other brief's Verify command greps, and
    that brief's row was designed as a tripwire (#634) with nothing listening. So the
    obligation lands where the cause is: the PR that reddens a row either re-runs that brief
    and commits the fresh witness (`statusgen verifyrun --brief <path>` — runs APPEND, the
    red run stays on the log), or states in the PR body which brief it left red and why that
    is out of scope. Reason: without this, a red row becomes a post-merge archaeology issue
    weeks later, filed against whoever finds it rather than whoever caused it — which is the
    verifier desk's largest issue source (29 filed in the week to 2026-08-13).

    **The out-of-scope note is read by a reviewer, never by CI.** No workflow parses a PR body
    for an exemption token, and none should: a CI-honoured escape hatch a worker can type
    into their own PR is a self-service bypass of the gate, which is the shape rule 30's
    override paragraph rejects. The note is an argument addressed to a person, and the person
    can decline it.

    **Drain, don't file.** The correction for a red row on main lands as a re-baseline PR, not
    as an issue. A red witness is already a red check annotating the exact brief and row; a
    second copy in the issue tracker adds a queue entry and no information.

46. **The `Brief:` trailer is the only PR→brief edge.** A PR is linked to the brief it
    delivers by exactly one trailer line in its body — `Brief: <stream>/<NN>` (the
    hierarchical forms of brief-v2 are accepted on read: `<stream>:<NN>`,
    `<repo>:<stream>:<NN>`, `<cell>:<repo>:<stream>:<NN>`; issue-only work with no brief
    carries `Issue: #<N>` instead). No title parsing, no branch-name heuristics: a
    derivation that cannot find the trailer finds nothing, and a merged PR with no
    trailer is a lint finding (NOTICE during backfill, PROBLEM after), never a guess. A
    trailer inside a fenced code block is documentation, not a link. There is
    deliberately no bypass flag, env var, or commit-message token — an override a worker
    can type into their own PR makes the edge asserted again, one level up, which is the
    shape rule 30's override paragraph rejects.

47. **A generated board table has exactly ONE WRITER; a hand edit to it is a PROBLEM.**
    When a stream README's Briefs table is wrapped in generated markers (the README
    frontmatter gains `board: generated`), every lifecycle cell in it is written by the
    regen loop from the witnesses rule 30 names, and hand-editing the table is the same
    defect as hand-editing `STATUS.md` — `statusgen --lint` PROBLEMs a marker-wrapped
    table whose content differs from the derivation. Until a stream's table is wrapped,
    its cells stay hand-written and rule 35's diff-against-merge-history NOTICE is the
    check that applies; the wrapping is a per-stream migration, not a flag-day edit.

## Provenance and gating

10. **Provenance is required.** A brief with an empty `sources:` is untraceable — no one can
    tell why the work exists or whether it's still needed. Name a scoping doc, finding, or
    intake ID.
11. **`gate` is derived from risk, not chosen.** Record all four answers (regulatory,
    customer, irreversible, sensitive-data); if ANY is `yes`, `gate` must be `human`. Reason:
    the answers are what a reviewer audits; the gate is just their conclusion. A human gate
    at `done` needs a review entry naming a human (`human:<name>`), not a model sign-off.

## Execution tiering (effort-keyed)

12. **Authoring/decomposition is strong-tier work; implementation is cheap-tier behind
    strong gates.** Errors in authoring (risk gates, dependencies, critical-path head)
    compound through every downstream implementer; implementation errors get caught by the
    gates. So a fast/cheap-tier model must not author a brief set — a cheap draft anchors the
    strong model that reviews it.
13. **Effort keys the execution tier.** Effort S may run inline at the session's tier
    (dispatch overhead dominates). Effort M/L should be *planned* at your tier then
    **dispatched** to cheap-tier implementers behind the verify/review gates. Reason: running
    M/L inline at a strong tier is the cost leak the system is built to avoid. A session
    cannot downlevel itself; dispatch is the only way down.

## Mid-flight changes

14. **Route a mid-flight tweak by one test: does the brief's Verify table change?**
    - No → just do it. A brief is a contract, not a step list; small course-corrections
      inside the contract need no ceremony.
    - Yes → amend the brief in the same commit (and demote it if it was past `implemented`,
      so it re-gates).
    - No owning brief exists → a one-line intake entry or a new small brief; never an
      untracked drive-by change.
    Reason: the Verify table is the contract's observable surface — changing it changes what
    "done" means, which must be visible to the board and re-reviewed.
15. **Splitting a brief mid-execution is authoring.** Apply the authoring rules (data-first
    pieces, typed deps, README rows). The splitting session keeps ONLY the piece it was
    actively mid-implementing; every other piece returns to the board as an unclaimed `todo`.
    Reason: self-assigning the whole split set is the rabbit-hole reflex at brief
    granularity — invisible to the board and un-re-tiered.
## Three-state instrument invariant

Every desk instrument — any program, script, query, or check whose output a human or
another instrument acts on — reports in three states, never two: **checked-clean** /
**checked-failed** / **could-not-check**. The canonical rule is in
`docs/three-state-instrument-rule.md`. These two rules apply it to Verify tables:

16. **A brief that adds a CHECK must include a mutation-test Verify row.** Revert the fix
    or break the guarded thing, run the check, and confirm it goes RED. Reason (the cost
    that forced it): six of the eight measured instances — a deploy Job reporting a broken
    pipeline green, a CI rollup reading page 1 of N as "all passed", a board printing
    "worker must act" without checking a worker exists — would have been caught in
    minutes by a single row that proved the instrument fails when the guarded thing is
    broken. A green table that was never mutation-tested is the very defect this rule closes.

17. **A brief that touches a shared lister/flag/query must include a neighbour Verify row.**
    One row exercises the *adjacent consumer* of the shared code path — not the
    deliverable. Reason: every well-scoped Verify table is blind to collateral regression
    in a sibling feature sharing a code path, flag, or data source; the tighter the row,
    the blinder. A neighbour row proves the change didn't silently break the thing next
    to it that uses the same mechanism.

## Register & brief IDs

18. **Sequential-ID collisions are a known hazard under parallel authorship.** When
    assigning a brief number (e.g., ``01`` in ``<stream>/01``), check not only
    the README on fresh main but also brief files added by open PRs — the merged-main
    view is the wrong read surface because another open PR may have already claimed the
    same number. The durable fix is **slug-identified briefs** (``<stream>/brief-01-three-state``
    instead of ``brief-01``) — the findings and intake registers already use this
    pattern and have eliminated the collision class. Until slugs are adopted, the
    CI check in ``.github/workflows/statusgen.yml`` flags a brief-NN filename whose
    number duplicates one claimed by another open PR. Reason: three parallel sessions
    each picking next-free from merged main all claim the same number; at least one
    must renumber after the fact. Slug identity kills the counter entirely.
19. **A register with per-entry files uses slugs, not sequential numbers.** Findings
    (``docs/streams/findings/``) and intake (``docs/streams/intake/``) entries use
    letter-prefixed slugs (``F-<slug>``, ``I-<slug>``) — the counter is dead.
    New entries using numeric IDs trip a ``statusgen --lint`` PROBLEM.
    A brief number collision is the same class at brief granularity;
    briefs inherit the same durable fix when they move to slug identity.

## Dispatch discipline

Every dispatch from a desk to an implementer or reviewer is a contract. These rules
prevent the desk's output — confident, compressed, citation-free by the time a worker
sees it — from becoming unreviewed authority. Dispatch-template rules for the desk and
fanout tooling live with those skills; this section covers only rules that apply to brief text.

20. **A figure claimed as "measured" must name the artifact that measured it.**
    "Measured" without a named instrument is a claim, not a measurement — the claim
    "X is measured" is itself unmeasured. Reason: a real incident — a
    claimed "measured" figure was a crashed probe's cap, applied as ground truth by
    five workers across nine sites because it was presented as settled fact.

21. **A cap a run survived bounds only the quantity it caps.** It is not a measurement
    of consumption, and it says nothing about quantities that live outside the cap.
    A WASM heap cap, for example, bounds V8 old-space only — WASM linear memory and
    native buffers sit outside it, so the process working set is unbounded by the cap.
    Reason: the same incident — a probe's heap cap was quoted as the
    sync's full working-set cost, ignoring the WASM boundary; the figure propagated
    across nine sites because it carried no primary-source citation a worker could check.

22. **A dispatch names its authority envelope.** Every dispatch states what it grants —
    repos, paths/write-surfaces, tools, and any budget — and the envelope must be a
    subset of the dispatcher's own authority: a desk cannot delegate what it does not
    hold. The subset relation must be checkable from the two artifacts alone (the
    dispatcher's own grant and the dispatch it writes), not from memory or intent.
    Source: CIGAR's handoff mechanism intersects a child's capabilities with the
    issuer's own scope — a result never amplifies authority.

23. **No amplification.** A worker that needs more than its envelope reports the
    request; the dispatcher grants explicitly — a recorded, reviewable act — or
    bounces. A result produced using authority outside the envelope is a finding
    about the dispatch, not a deliverable to accept. Source: CIGAR records a
    requested-but-ungranted capability rather than silently absorbing the request
    or silently widening the grant.

24. **Declared exclusions.** Dispatch context states what was deliberately excluded —
    files, surfaces, or candidates considered and left out — so a scoped view is
    never mistaken for the whole. This promotes the existing "never truncate dispatch
    file lists" / "a scoped view is not evidence of absence" practice from lore to
    rule; CIGAR's selection manifests, which record rejected candidates and not just
    the selected ones, are the independent derivation.


## Row-runner discipline

Rules 7-9 say what a Verify row must BE. These say what it must not do — five
command shapes that make a row report a verdict it did not measure. Each is
decidable from the command text, so `statusgen --lint` flags it (NOTICE, tagged
with the rule name in brackets). The rules below were derived from a corpus
inventory of an existing brief set; run `statusgen --lint` over your own tree to
produce the equivalent inventory for it.

The shared shape is worth naming, because it is not carelessness: **in every
case the harness silently substitutes its own answer for the one under test,
and the row goes green either way.** None of these is discoverable from a
passing run — which is why they are lint rules and not review vigilance.

25. **Never assert a specific non-zero exit code through `go run`** (`gorun-exit`,
    #493). `go run` does not propagate the program's status: it prints
    `exit status 5` to stderr and itself exits **1**. Every non-zero code
    flattens to 1, so a row cannot tell 3 (disabled) from 4 (rate-limited) from
    5 (refused) from 6 (could-not-check) — the whole three-state contract. `go
    run` also exits 1 when the package does not COMPILE, so such a row passes on
    a tree whose code never built. Measured on PR #487: `go run` → 1, the built
    binary on the same world → 5.
    - Write: `go build -o /tmp/tool ./cmd/tool && /tmp/tool …; echo $?`
    - A row expecting exit **0** is unaffected and needs no change.

26. **A pipe in a grep pattern needs `-E`, and `-E` needs the pipe unescaped —
    so use neither** (`bre-alternation` / `ere-literal-pipe`, #262, #509). Without
    `-E` the pattern is a BASIC regex where `|` is an ordinary character, so
    `grep -c "alpha|beta|gamma"` searches for one 17-character literal — and in a
    brief the single line containing it is the Verify row itself, which returns 1
    and passes a `≥1` bar having measured nothing (#257's table returned
    1,1,1,1,1,1,1 against thresholds 3,3,1,3,2,3,3). With `-E`, `\|` is a literal
    pipe and matches nothing. The markdown escape makes the two indistinguishable
    on sight: GFM renders `\|` as `|`, so the source and the rendered page are
    different commands.
    - Write: `grep -cE -e alpha -e beta -e gamma f.md` — no pipe at all, so it
      reads identically in both.
    - For a genuinely literal pipe: `grep -F`, or a `[\|]` bracket class.

27. **RE2 selectors get one token or a chain, never an alternation**
    (`rE2-literal-pipe` / `shredded-cell`, #374). `go test -run` / `-bench`
    compiles RE2, where `\|` is a literal pipe: `-run 'Forged\|Sub\|Onboard'`
    matches zero tests, prints "no tests to run", and exits 0 — and the Evidence
    row records the vacuous command as though the tests ran (two live briefs did
    exactly this). Writing the pipe RAW does not fix it: a bare `|` is a
    table-cell delimiter wherever it sits, so the command is cut at the pipe, the
    Expect column becomes a fragment of the command, and every other row check
    goes blind past the cut. There is no spelling of an RE2 alternation that
    survives a table cell unambiguously.
    - Write: `-run Forged`, or `go test -run A ./... && go test -run B ./...`,
      or move the command to a fenced block outside the table.

28. **A comparison base must be a pinned SHA or a computed merge-base, never a
    branch** (`moving-ref`, #639). A row based on `origin/main` is a function of
    another branch's tip, not of the tree under test. Measured: the identical
    `statusgen --consumers --base origin/main` returned exit 1 and exit 2 on
    consecutive runs because background commits advanced main between them. A
    flappy gate is worse than a failing one — the green run and the red run are
    equally unreproducible, so re-running settles no disagreement.
    - Write: `--base $(git merge-base origin/main HEAD)`, an explicit SHA, or
      the PR's own `base.sha`.
    - `refs/remotes/origin/main` is the same moving ref by its long name.

29. **Rows run on macOS too — no GNU-isms** (`gnu-only`, #650). A Verify row is
    run by whoever verifies, on their own machine, and a row that only works on
    ubuntu produces a different verdict per platform with the platform invisible
    in the Evidence cell. The catalogued instance fails QUIETLY: under BSD `grep`
    a `<(…)` process substitution's `/dev/fd/N` reads as EMPTY, so
    desk-hardening/06 rows 1,4,5,6,7 returned count 0 with the content plainly
    present — an empty read is indistinguishable from a genuine absence, so it
    looks like a finding about the code rather than about the row.
    - `<(cmd)` → a pipe: `sed … | grep -cE …`
    - `grep -P` → `grep -E`
    - `sed -i 's/…/'` → `sed -i '' 's/…/'` (BSD needs the suffix argument)
    - `readlink -f`, `date -d`, `stat -c`, `xargs -r`, `sort -V`, `tac`,
      `mapfile` — all GNU-only; the lint names a portable substitute for each.

45. **A Verify table MAY declare each row's CLASS, and a scripted row must exist
    and be executable** (`<stream>/NN`; the row-classes spec lives with that
    stream's design docs in the authoring repo and is staged separately). Add an
    optional `Class` column
    right after `#` — `| # | Class | Command | Expect |` — so the verdict lane can
    route each row instead of re-interpreting prose+shell at run time:
    - `check:ci` — HERMETIC (tree-only). CI re-executes it **network-off** and
      refuses the verdict on mismatch; hermeticity is enforced at execution
      (`statusgen verifyrun` disables the network), never merely declared.
    - `check` — deterministic but ENV-BOUND (a live PEM, a real queue, a tool on
      PATH). A runner executes it; CI skips an explicitly-classed one (its verdict
      rests on the verifier's authorship+signature, not on a CI re-run).
    - `gate:model` / `gate:human` — JUDGMENT rows. `gate:human` stays on the
      verify-gate issue pair, outside the transcription lane.

    A table with **no** `Class` column is legacy: every row is treated as `check`,
    and the whole inherited corpus keeps its exact prior behaviour — the column is
    additive. A `check:ci`/`check` row may be a **reviewed script** instead of an
    inline command — `docs/streams/<stream>/verify.d/brief-NN/row-K.sh`
    (executable, exit 0 = PASS) — and then the row's Command cell IS that script
    path; the reviewer who approves the brief approves the script (the reviewer is
    the trust anchor, no freeze rule). `statusgen --lint` hard-errors (PROBLEM) on
    an **unknown class**, and on a `check:ci`/`check` scripted row whose script is
    **missing or not executable** on a brief the board has moved past `todo`; and
    it raises a conspicuous NOTICE naming every `verify.d/**` script a PR's diff
    touches, so a change to runner-executed code is reviewed as code.

    *(Numbering note: the authoring repo's copy of this file allocated this rule and
    the typed-edge rule in the Dependencies section the same number in parallel —
    the collision class rule 40's detector exists for. This file allocates cleanly:
    44 is the typed-edge rule, 45 is this one.)*

## Derived surfaces

A fact that appears on more than one surface has exactly one declared source; every other
occurrence is regenerated from it, or diffed against it by a check. A hand-maintained
second copy is the defect whatever it contains — it drifts, and the discipline that was
supposed to keep it current is the discipline that already failed (#592 → #627 → #685 is
the same defect filed three times).

The surfaces under this section so far: `STATUS.md` (regenerated from the stream
boards); the issue-loop scan PR's body counts (rule 34); and each stream README's
Briefs table, whose lifecycle cells derive from the witnesses rule 30 names — the table
becomes a generated surface with exactly one writer (rule 47) as each stream is wrapped
in markers, and is diffed against merge history until then (rule 35).

32. **A worker's terminal verdict on a PR is a DISPOSITION RECORD, not a prose comment.**
    A conclusion that only a human can read is one a sweep must re-derive. In one
    2026-08-12 batch-fanout cycle, 8 of 10 completed orphan dispatches re-derived a
    conclusion an earlier pass had already posted; one orphan was re-derived four times
    across three weeks, and one sweep cited a worker's own "this is dead" note as
    evidence of activity. Write the record with `deskdisposition set`, which emits both
    halves:

    - the **label** `disposition:<verdict>` — the index a sweep filters on in the
      `gh pr list --json number,labels` call it already makes; and
    - the **marker comment** — the record, carrying the evidence link a reader needs:

    ```
    <!-- desk-disposition v1 -->
    Disposition: SUPERSEDED
    Evidence: https://github.com/<owner>/<repo>/pull/223
    Recorded-By: <session or App>
    Recorded-At: 2026-08-13
    ```

    The vocabulary is CLOSED — an open one is a prose comment with extra steps:

    | Verdict | Meaning | Evidence | Still dispatchable? |
    |---|---|---|---|
    | `SUPERSEDED` | the work landed through a different branch/PR | required | no |
    | `RESOLVED-ELSEWHERE` | the outcome was reached another way (issue already closed, row already advanced on main) | required | no |
    | `NEEDS-REBASE` | live work, mechanically blocked | optional | yes |

    Evidence is required for the terminal verdicts because "superseded" with no link to
    what superseded it is the same unfalsifiable claim the prose comment was.

    **The record does not close anything.** Writing it is the worker's; closing the PR is
    a human-authorized event and belongs to `deskclose` (issue-flow/03), which consumes
    these records as its queue. Stated-but-unexecuted close intent is its own failure
    (one tracker item sat that way from 2026-08-09) — the record is what makes the close
    decidable without re-investigating. **And a `SUPERSEDED` record is answered by a
    second role before it closes:** under a worker token `deskclose superseded` only
    PROPOSES (`superseded?` label + proposal comment naming the target); the review desk,
    under the reviewer token, confirms (closes, with a back-reference on the target) or
    disputes (`needs-decision`, human-only close). The role is read from the token's roster
    binding, never from a flag — see `docs/streams/desk-tools/superseded-confirmation.md`.

33. **A check that reads a derived surface is three-state.** checked-clean (it read, and
    found nothing) / checked-failed (it read, and found something) / could-not-check (it
    could not read). A PR whose disposition could not be read is **not** dispatch-eligible
    — an instrument that could not look must never answer the question it was asked, and
    a sweep that rate-limits reports could-not-check for the whole repo rather than an
    empty queue.

34. **Counts on an emitted artifact are derived from the artifact they describe.** The
    issue-loop scan PR's created/retired counts come from
    `git diff <merge-base>..HEAD -- docs/streams/issue-loop/` at push time
    (`deskscanbody emit`), never from a body written once and pushed past. The
    belt-and-suspenders half is `deskscanbody check`, which refuses a title/body whose
    stated counts disagree with the diff — the class becomes a red gate instead of a
    reviewer catch, which is what #592 and #627 both relied on and both lost.

35. **A merged PR that names a brief must not leave that brief at `todo`.**
    `statusgen --lint` emits a NOTICE when a merge whose branch or subject names
    `<stream>/<NN>` sits against a README row still at `todo`/`in-progress`: the board is
    then offering work that already landed (desk-hardening/01 was offered at score 3500
    for days after PR #255 merged its deliverables). The cell stays hand-written — a
    status transition is a judgement, and a PR can land only part of a brief — but it is
    now DIFFED against the merge history instead of trusted. Severity is NOTICE until the
    standing backlog of drifted rows is reconciled; promotion to PROBLEM is a later
    ruling.

## Merge-time re-check

The review gate asks "is this correct against main?" and answers it against the main that
existed at review time. The merge lands it in a different main. These three rules occupy that
gap.

38. **Cite by EXPRESSION, not by line number.** A citation names the thing it points at — a
    function name, a rule's heading text, a section title, an identifier — optionally with a
    line number as a convenience, never instead of the name. Reason: a `file:line` citation
    is silently invalidated by any edit above it, and the failure is undetectable, because a
    stale line number still resolves to *a* line. The observed shape is a citation that after
    a rebase points into a code fence, reads as confirming something, and confirms nothing.
    An expression citation either resolves or visibly does not.

    This is also what makes a hand-maintained numbering space citable at all: rule 40's
    detector reports both allocations by their heading text, so a reader can tell which rule a
    "brief-rule 26" meant even while the number is ambiguous.

39. **A materially-changed diff re-derives the PR body and the Verify table in the SAME
    push.** Material means: a version bump, a changed part/artifact count, a reverted or
    replaced design decision, a dropped or added deliverable — anything that makes a sentence
    in the body no longer describe the diff. Mandatory on `gate: human` briefs, because there
    the human signs the BODY: a stale body means the signature attests to fiction, and the
    recorded instance is a PR body asserting a funds-protection property the code had already
    reverted. The reviewer half is rule 32's discipline applied to every delta re-review —
    read the body and the Verify table against the CURRENT diff, and treat any claim the diff
    contradicts as a blocker, not a nit.

    This is derive-or-diff with the body as the copy: the diff is the source, the body is a
    hand-written second copy of what the diff does, and a copy that is not regenerated must be
    checked against its source. It is deliberately a review rule and not a mechanised gate —
    deciding whether prose still describes a diff is a judgement, and a checker that guessed
    would either miss the interesting cases or block on rewording.

40. **A merge-time re-check runs against the MERGED tree, and reports four states, not two.**
    A semantic merge collision — two changes each valid alone, invalid together, textually
    non-conflicting — is invisible to every check that reads one branch's tree, including this
    repo's own lint. `statusgen mergecheck` computes the trial merge (writing no ref, checking
    nothing out, merging nothing) and runs its probes over that tree. Four states, because
    collapsing any pair of them sends someone to the wrong file:

    | State | Meaning | What it asks of the worker |
    |---|---|---|
    | `MERGE-INTRODUCED` | the probe passes on the branch and fails on the combination | fix it here; nobody else can see it |
    | `PRE-EXISTING` | it already fails on the branch alone | fix it here, but the merge is innocent |
    | `STALE-BASE` | the base carries a CI-invoked path the branch's tree lacks | resync; a failure here is currency, not a defect |
    | `could-not-check` | a probe did not run | repair the instrument — this is never a pass |

    The last row is the three-state invariant at merge time and it is the load-bearing one: a
    re-check that cannot reach its base and answers "current" certifies stale work with a
    green tick.

    Two things this rule deliberately does NOT do. It does not gate on merge-currency —
    measured 2026-08-13, 52 of 52 open PRs in this repo were behind main, and a hard gate
    there reds the whole queue on day one — so currency is reported and never fails. And it
    does not judge approval staleness: a GitHub review's `commit_id` has been observed to
    disagree with the head named in the review's own body, and **the direction and frequency
    of that disagreement are unmeasured**, so it is not a sound staleness signal in either
    direction. Approval currency is therefore reported as could-not-check.

    State that one at its evidential strength, because this rule is cited from other briefs.
    The disagreement is a SINGLE observation (`#881`); a 10-PR sweep taken afterwards (`#940`)
    failed to establish any direction — 4 of 5 sampled reviews had `commit_id` equal to head
    with no post-review push, consistent with the field being correct, and `#750`'s
    `commit_id` correctly lagged head. No review body in that sweep names a SHA, so no
    cross-check exists to measure a direction with. It is **not** established that the field
    under-reports staleness and **not** established that it fails open. The reason not to
    build on it is the unknown error direction itself: a signal that cannot be characterised
    cannot be trusted either way, and a merge-time claim resting on it inherits an unmeasured
    error. "We have not measured this" is the finding — do not upgrade it to a direction.

    **A check ships with the list of shapes it cannot see.** `mergecheck` prints its blind
    spots on every run, clean or red — compile-level collisions when nothing was compiled,
    behavioural collisions, namespaces other than this file's rule numbers, collisions with
    a still-open PR, and approval currency. A clean verdict that does not say what it did not
    look at is read as "nothing is wrong".

## Parallel work on one brief

A brief may declare that its work decomposes into concurrent shards
(`parallel-streams:`). These two rules bound that. Absence of the field is the default
and needs neither.

41. **A file partition is a precondition, not a safety argument.** Disjoint paths prevent
    exactly one collision class — two workers writing the same bytes — and that only when
    a check proves the disjointness against the real tree rather than reading an author's
    assertion. Three classes survive a path partition untouched, and all three have
    reddened this repo's main: a **semantic** collision (one shard changes a declaration
    another shard calls; no textual conflict, both shards green in isolation), a **shared
    space** (rule numbers, board rows, a generated artifact, a module graph, a pin set —
    where the contended resource is not the byte, which is how this very file came to
    carry rules 25 and 26 twice before that was reconciled, and why rule 40's detector
    exists), and a collision with a **different branch** entirely.
    A split therefore ships with a checker that names which classes it covers and reports
    every pair it could not reason about; the covered/uncovered boundary is stated, never
    implied by a green run. Reason: the failure this rule prevents is not a bad split, it
    is a split believed safe on the strength of a check that never looked at the thing
    that broke.

42. **A shard's verdict is not the brief's verdict, and an unread sibling is
    could-not-check.** Applying rule 33 to a split: a shard reporting success while any
    sibling shard's state could not be read reports **could-not-check**, and the brief
    stays in-progress with the missing shards named. The brief-level Verify table runs
    against the RECOMBINED tree, because N per-shard greens are evidence about N trees
    and none of them is the tree that merges. A partial split is reported as partial;
    it is never a silently half-applied brief.

## Verify row semantics: dereferencing vs. presence

43. **A brief whose deliverable makes checkable factual claims must include at least one
    DEREFERENCING Verify row — not only presence/formatting checks.** Two Verify rows can
    both go green without proving the same thing, and the format should make authors notice
    which one they are writing:
    - **Dereferencing checks** resolve something: fetch a link and inspect what it actually
      serves, run a documented command and compare its real output/exit code against the
      specific claim the deliverable makes, check a documented ID or property against the
      live system it describes. These CAN fail on a document that is factually wrong but
      well-formed.
    - **Presence/formatting checks** (`grep -c`, `wc -w`, "section X exists") prove the
      deliverable is well-formed — a required section is there, at the required length, with
      the required tokens. They CANNOT fail on a wrong-but-well-formed document: a confidently
      worded falsehood sitting in the right section, at the right word count, passes exactly
      like the truth would.

    This sharpens rule 8, it doesn't relax it: rule 8 says a presence-only table must be
    *disclosed* as gating presence, not quality; this rule says that when the deliverable
    carries checkable facts, the table must also carry at least one row *capable* of catching
    a wrong one, so the honest disclosure in rule 8 doesn't quietly become the whole floor.

    **This rule is a different axis from the row-runner rules (25-29) and from the
    unfailable-row lint** (`statusgen`'s `verifyrows.go`), and it is worth being precise
    about the difference, because the three are easy to mistake for one another:
    - Rules 25-29 catch commands whose TEXT betrays that the harness substituted its own
      answer for the one under test — a `go run` exit-code assertion, a BRE `\|` alternation,
      an escaped pipe under `-E`, an unpinned comparison base, a GNU-only flag. They are
      decidable from the command string, so `statusgen --lint` flags them.
    - The unfailable-row lint catches rows structurally incapable of failing for *any* input
      (a `grep -c` gated on an expected count of zero, an always-zero pipeline sink).
    - This rule catches a row that is well-formed, portable, and genuinely failable, and still
      measures nothing about the claim. `grep -c "Section 3" doc.md` returns 0 on a malformed
      doc, so it is failable and passes both of the above — and it fails only on a *missing*
      section, never on a wrong claim inside a present one.

    A brief can therefore clear every mechanical row check in this file and still ship a Verify
    table that cannot catch a false statement. That gap is what this rule closes, and it closes
    it at authoring time, by judgement, not by lint.

    Reason (the triggering evidence): a brief `<stream>/NN` (a GitHub App setup guide)
    shipped a Verify table with 8 rows, every one a grep-presence count ("section X appears ≥N
    times", wordcount ≥800, lint exit 0) — all 8 passed, and the guide was factually wrong in
    four places, one load-bearing: it asserted a GitHub enforcement property that GitHub does
    not actually provide. The same session had already hit the sibling shape once:
    another brief `<stream>/NN` (a market analysis) passed its model review gate with citation
    links present but never resolved, carrying an invented competitor name and a URL that
    serves a different vendor's docs. Neither defect was catchable by a table built entirely of
    presence counts — the row that would have caught either one had to fetch the URL or check
    the named claim against a live system, not count characters near it.

    Applies when the deliverable asserts something checkable about the world — a setup/config
    guide, a market or competitive analysis, a spec, anything carrying "X is true" claims a
    reader could act on. Does **not** apply to genuinely presence-only deliverables (a docs-only
    reformat, a template scaffold, a brief whose content makes no factual claim to dereference)
    — manufacturing a dereferencing row there checks something arbitrary just to satisfy the
    rule, which is its own kind of checkmark-DoD. Whether a deliverable "makes checkable factual
    claims" is a judgement call the author states, not a mechanical trigger — **no lint enforces
    this rule**, and that is a declared limitation, not an omission: deciding whether a row
    dereferences requires knowing what the deliverable claims, which is not in the command text.

    **Relation to a proposed link-resolution lint.** A proposed report-repo lint would resolve
    every link in a report-repo doc and flag a 404 or wrong host. If/when that lands, it is a partial,
    mechanical instance of *this* rule — link-resolution is one specific kind of dereferencing
    check, not the whole class (running a documented command and checking its real output, or
    checking a documented ID against a live API, dereference just as much and involve no link
    at all). Don't treat a passing link-resolution lint as satisfying this rule on its own if
    the brief's factual claims aren't primarily link-shaped; and don't skip authoring a
    dereferencing row on the assumption that a future lint will supply one — this rule is the
    authoring-time requirement, that proposed lint (if built) is a narrower automated assist layered
    on top.
