# Brief rules

The rules a `brief-v1` file must satisfy, each stated with its reason. The template is in
`brief-template.md`; this is why it looks the way it does. A validator (`statusgen --lint`)
enforces the machine-checkable ones; the rest are review-gated.

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

