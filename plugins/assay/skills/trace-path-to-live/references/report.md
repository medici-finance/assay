# Report presentation

### Plain-language titles and counts

Use these visible headings: **Progress over the last month**, **Work still to finish**,
**What to do next**, **Longest sequence of dependent work**, **Work dependencies**, and
**What was checked and what is unknown**. Technical terms may appear in explanatory details,
not unexplained titles, chart legends or headline cards. In particular, do not label cards
"combined closure", "full-service" or "path points". Name each confirmed outcome in ordinary
language; use brief ids as secondary references. Define a brief once as a tracked piece of work.

Distinguish **Total work** (all distinct briefs in scope, including completed work) from
**All work remaining** (every brief whose status is not `done`). A `verified` brief is
**Awaiting final approval** and still counts as remaining. Do not equate dependency satisfaction
or a zero path weight with full completion. Show weighted-path scores only in an optional
method explanation as relative work estimates, explicitly not days or brief counts.

### Progress chart — first substantive content

Immediately below the objective and its plain-language scope, before summary cards or graphs,
show one chart with **six lines**, a labelled legend and an accessible table of the same values:

| Line | Count |
|------|-------|
| Total work | all briefs present in the selected scope at that reading, including `done` |
| All work remaining | all present briefs not marked `done` |
| Not started | `todo` |
| Being implemented | `in-progress` |
| Awaiting verification | `implemented` |
| Awaiting final approval | `verified` |

Use the report's stated timezone and a 28-day window: three preceding seven-day blocks,
oldest first, then the latest seven calendar days individually. This gives ten readings.
For each older block, show the count at its last day's end, label the full date range, and
say "week-end count", never imply it is a sum or average. Daily readings use day-end counts;
today uses the captured source revision and is labelled "so far". Separate the weekly and
daily portions visually. Use real date spacing or clearly labelled period slots, not an
unlabelled change of time scale. All six values must be inspectable at every reading.

For history, follow the **same current confirmed scope**: the deduplicated prerequisite set
computed in SKILL.md steps 3–4. Do not retroactively re-resolve a different objective for each day. Read each
root's default-branch state from historian data only when its cutoff, source revision and
status semantics are recorded. Otherwise, pin the fetched default-branch tip and walk
`git rev-list --first-parent <tip>` in ancestry order; select the first commit whose committer
timestamp (`git show -s --format=%ct <sha>`) is before the reading's exclusive cutoff. Use the
next local midnight as an exclusive cutoff for completed days, converted to a UTC instant;
today uses the captured tip. Compare numeric timestamps, not author dates; do not assume
committer dates are monotonic. This is a reproducible Git-history approximation, not proof of
when a commit reached the remote branch; disclose that limitation.

Materialise each selected SHA with `git archive` into separate scratch trees as in SKILL.md
step 2, preserving sibling repo basenames. Never check out historical commits or regenerate
files in a shared checkout. For each scoped brief, run the same source-pinned
`statusgen brief --root <snapshot-dir> --json <stream>/<NN>` reader, one key per call, and
read its returned `row.status`. Record the reader version/source, captured tip, selected SHA,
cutoff and timezone for every root and reading. A missing row, failed reader, ambiguous
identity or unreadable snapshot is unknown. Count a brief as not yet present only when its
absence is established in a fully readable tree; a failed lookup alone does not establish
absence. Match stable brief identity across moves/renames when available.
Distinguish a brief not yet present in a fully readable historical tree from an unreadable or
unmatched record. The former contributes to neither total nor remaining; the latter is unknown,
never zero or an invented flat line. Mark incomplete readings and leave affected series as gaps.
Explain that these are historical counts for today's selected work; newly authored scope can
make both total and remaining rise. This is not a historical reconstruction of the entire plan.
Disclose other lifecycle states; include them in remaining when not `done`, and do not silently
assign them to one of the four named stages. No projection or ideal burn line is required.

### Progress toward each outcome

After the monthly chart, show an **Outcome progress** view for each confirmed target and
any useful intermediate outcome. Use operational milestone records and their exact versions
when supported. Otherwise use clearly labelled **proposed outcome** packages derived from
named target briefs; record their membership/source and never imply that the proposals are
operational milestones. Packages may overlap: do not add their counts.

Make the main outcome view **one time-series line per outcome**, labelled **Briefs remaining**.
Use exactly the same ten weekly/daily readings, timezone, source revisions and canonical status
reader as the monthly chart above. For each outcome, hold today's prerequisite membership
fixed across those readings. Count present briefs not `done`; retain `verified` as remaining.
Proven-not-yet-present briefs contribute to neither remaining nor total. Unknown or unmatched
records make the affected outcome's reading unknown: draw a gap, never zero or a line through
the gap. Do not re-resolve a different historical membership or backfill absent work as todo.

Show a labelled, toggleable legend with each outcome's **current remaining/total** count,
an axis labelled in briefs and an accessible table of all ten readings for every outcome.
The table exposes period totals, remaining, unknown and not-yet-present counts. Separate the
three weekly and seven daily slots visibly, as in the monthly chart. Explain that the six-line
chart shows the entire selected plan's lifecycle mix; the outcome lines show the same data
split into overlapping prerequisite packages. Their counts are not additive. A line can rise
as today's scoped briefs first appear in recorded history; only `done` leaves remaining.
Neither a passing run nor a merged reconciliation PR moves a point without a recorded status.

Move current stage bars into a **collapsed details** section labelled **Current outcome stage
breakdown**. Within it, use one stacked bar per outcome with mutually exclusive recorded
stages: **Not started**, **Being implemented**, **Delivered / awaiting verification**,
**Verified / awaiting closure**, and **Done**. Render blocked, withdrawn or unknown states
explicitly. Provide exact counts, a labelled legend and a text/table equivalent. Widths count
briefs, not effort or time. Do not call an outcome accepted merely because all its contributing
briefs are done: show exact outcome acceptance separately; without that observation say
**Acceptance not established** (or **No operational milestone record** for proposals).

Keep supporting evidence visible beside the outcome chart, independently of the collapsed bars. It shows observations that the lifecycle alone hides. Examples are
**Merged delivery; board reconciliation pending**, **Checks passed; closure tooling blocked**,
and **Approval recorded; reconciliation PR open**. Every badge links to its source and states
whether the evidence is on captured main or only on an open PR's captured head. Preserve
recorded status in the bar and monthly chart: do not move a `todo` into delivered based on an
inference, count an open reconciliation PR as merged, or promote a passing execution to
verified. Delivery observations must identify the actual delivering change/retained artifact;
a record-only reconciliation is not a second implementation. Historical failing runs remain
history when a sourced later repair or result supersedes them; an open tracker alone does
not prove its original failure still exists.

### What changed since the previous report

Show this section near outcome progress, before the detailed action list. Name the exact
baseline report/capture, root revisions, comparison scope and observation limits. Compare
stable work identities. Show **scope added/removed** separately from changes among shared
briefs; do not infer progress by subtracting totals for different scopes.

Distinguish **newly observed merged deliveries**, **verification/execution results**,
**recorded lifecycle changes**, **approval/reconciliation activity**, and **dependencies now
ready or newly held**. Link each event to its brief and immutable source, PR head or ruling.
Readiness changes require comparable canonical verdicts, not downstream-reach scores. PR and
commit counts describe activity, never completed briefs. Do not call a newly discovered old
delivery new work: show its landing date and label it newly observed. Open-branch evidence
belongs in an in-flight lane, not the merged-delivery lane. A green CI rerun is not independent
verification. If the baseline lacks a fact or reader coverage, show **Not comparable** and
explain the gap; no baseline means this is the first measured capture, not zero changes.

### What is holding progress

Add an evidence-backed blocker view with **Implementation**, **Verification tooling**,
**Decisions**, and **Review** categories. Keep prerequisite waiting and unknown coverage
visible too; neither means that unstarted code is defective. Classify concrete obligations,
not all unfinished briefs. One brief can have multiple blockers: deduplicate each category
by brief for display, say category totals overlap, and never sum them as remaining work.
Every entry names the affected brief/outcome, cause, owning role, source and next act.

Keep product/test failures distinct from runner/admission/closure-tool failures. A missing
approval must identify its actual subject; an already recorded design approval is not a new
acceptance request. Use the latest relevant current-head review evidence; a fixed prerequisite
may await review-desk reassessment without becoming a fresh code defect. Unsupported or
unreadable causes stay **Unknown**. Use concise text labels alongside colors, and make the
category view filterable when useful. These views explain the evidence; they do not dispatch,
review, accept or change a lifecycle.

### Remaining-work graph and coverage

Write ONE self-contained HTML page: inline CSS and JS, no external fetch, no CDN. In addition
to the progress chart and plain-language counts above, it contains:

- the objective and the confirmed targets;
- outcome progress, a comparable previous-report change view and classified blockers as above;
- the critical path, overall and per target;
- the ranked moves;
- the graph, rendered from only nodes whose status is **not `done`**:
  - each node shows its status, gate, eligibility, PR and claim badges;
  - the critical path is highlighted;
  - exclude `done` nodes and their edges from rendering, search, click-isolation and layout;
    keep them in the underlying analysis, historical counts and provenance;
  - keep `verified` nodes visible as awaiting final approval, including any sign-off action;
  - calculate dependency readiness and weighted paths on the complete graph before filtering;
    never invent a direct dependency across a hidden completed node;
  - clicking a node isolates its remaining prerequisites and dependents; its detail panel may
    separately list completed prerequisites as text;
- a coverage block. It lists each root with its SHA, every `could-not-check` root, hold or reader,
  every fallback used, and the cross-root dependents.

Treat all brief, PR, search and detail text as untrusted data. Render it with text nodes or
`textContent`, never raw HTML or `innerHTML`. When embedding JSON in an inline script,
use a serializer that escapes `<`, `>`, `&`, U+2028 and U+2029 (including any closing
script sequence); do not concatenate source strings into markup or executable code.
Validate link schemes before creating links; allow only intended local links and HTTP(S),
never executable schemes.

The page names private boards when a root is private, so write it **outside any repository** in
the operator's scratch location and never commit it. Hand the operator its absolute path.
