---
brief: assay:assay:desk-supervision:32
title: Human decision record — options, recommended default, the pick and the latency
why: >-
  When the driver rules on a decision issue, nothing records which options were offered,
  which one the desk recommended, which one was picked, or how long the decision waited. So
  no one can ask "how often is the recommended default taken?" or "how long does a human gate
  hold work up, per kind of decision?". The close tool already reads the issue and the ruling;
  writing a small structured record at that moment costs almost nothing, while rebuilding it
  later from free-text threads is guesswork.
wave: 0
depends: []
unblocks: ["desk-supervision/35"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (pipeline analysis records)
exec-tier: strong
exec-tier-why: >-
  (b) correctness spans the shared Options parse (lifted out of deskinbox, which has a byte-parity
  test against the bash oracle), a new forge field on two backends, deskclose's human-decided close
  lane, and the ask-decision skill's relay shape; a record that silently mis-letters the pick or
  carries a login survives a happy-path test.
domain: complicated
outcome: none
sources:
  - "docs/streams/desk-supervision/brief-27-verify-outcome-join-keys.md — sibling record brief; same join-key and must-not-collect contract (the desk-supervision/28–35 analysis-records set)"
  - "plugins/assay/skills/ask-decision/SKILL.md — §The format (Options lettered, recommended default FIRST, never more than four), §Recording a ruling (relay `Asked as: … Answer: <letter>`), §Ratification"
  - "research note (driver-held, 2026-10-06): an independent review of pipeline analytics concluded human-gate latency and default-acceptance cannot be measured from today's records"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): deskclose's human-decided triage close posts a prose comment and an audit line only — no structured decision record (tools/desk/cmd/deskclose/triage.go:175-240, :390-404); deskkit.Issue carries no creation time (tools/desk/internal/deskkit/forge.go:286-314); the Options parse exists only inside cmd/deskinbox/format.go:277-345 and the bash oracle plugins/assay/scripts/assay-inbox.sh:695-715"
consumers:
  - "tools/desk/internal/deskkit/decisionrecord.go (planned — record type, Options parse, pick parse, validator, local append): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go (Issue gains CreatedAt): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskclose/triage.go and authority.go (record composed in the human-decided lane; ghComment gains CreatedAt): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskinbox/format.go (Options parse now calls the deskkit one): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/desk-supervision/human-decision-v1.md (planned schema doc): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/ask-decision/SKILL.md (one paragraph: close through deskclose's human-decided lane so the record is written): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (deskclose section): follow-up desk-supervision/32 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/scripts/assay-inbox.sh (the bash oracle's own Options parse): out-of-scope (unchanged; it stays the parity reference that Verify row 7 holds the lifted Go parse to)"
  - "deskclose's other lanes (manifest, duplicate, superseded, self-withdraw, not-planned): out-of-scope (none closes a single ruled decision issue; the manifest batch lane's per-row record is deferred to an unauthored brief)"
  - "decision issues closed by hand outside deskclose: out-of-scope (no writer runs, so no record exists; the records page in desk-supervision/35 must name this gap)"
  - "docs/records-and-retention.md: out-of-scope (listed there by desk-supervision/35, which this brief unblocks)"
---

# Brief 32 — Human decision record — options, recommended default, the pick and the latency

## Context
files:
- **add** `tools/desk/internal/deskkit/decisionrecord.go` (planned) + `decisionrecord_test.go` (planned)
  — `HumanDecisionRecord`, `ParseDecisionOptions`, `ParseRulingPick`, `ValidateDecisionRecord`,
  `AppendDecisionRecord`.
- **edit** `tools/desk/internal/deskkit/forge.go` — `Issue` gains `CreatedAt string \`json:",omitempty"\``
  (omitempty keeps the forge golden corpus byte-identical); populate it in `forge_github.go`
  (`GetIssue`, `return &Issue{` at :545) and `forge_gitlab.go` (:622, :632).
- **edit** `tools/desk/cmd/deskclose/triage.go` (+ `triage_test.go`) and `authority.go` (`ghComment`
  gains `CreatedAt`, filled at :195 from `deskkit.Comment.CreatedAt`).
- **edit** `tools/desk/cmd/deskinbox/format.go` — replace the inline Options parse (:277-345) with a
  call to `deskkit.ParseDecisionOptions`; rendered output unchanged.
- **add** `docs/streams/desk-supervision/human-decision-v1.md` (planned) — the schema, field table,
  closed sets, never-recorded list.
- **edit** `plugins/assay/skills/ask-decision/SKILL.md` — §Recording a ruling: one paragraph.
- **edit** `tools/desk/README.md` — deskclose section (around :3339).
- **add** `changelog/desk-supervision-32.md` (planned).

facts:
- The human-only close is `deskclose triage --disposition human-decided --decision <comment permalink>
  --tracker <ref>` (`tools/desk/cmd/deskclose/triage.go:13-55`). It refuses an item still labelled
  `needs-decision` (`refuseNeedsDecision`, :375) — the desk relabels to `human-decided` first — and
  authorizes only on the ruling comment ON that issue, author-verified as the roster's blessing
  authority (`humanDecidedAuthority`, :329; `verifyHumanAuthor`, `authority.go:229`).
- `applyTriage` (:175) order: read item → refuse PR → closed=no-op → authorize → compose comment →
  dry-run exit → post comment → close. The record is composed after authorize and before the comment.
- `fetchCommentKinded` (`authority.go:165`) already lists the issue's comments via
  `ListCommentsTyped`; `deskkit.Comment` carries `CreatedAt`, `Author`, `Body`, `Minimized`
  (`forge.go:552-560`). No new forge read is needed for comments; the issue's own creation time is
  the one missing field.
- Option format (ask-decision §The format): an `Options` heading, list lines lettered `A`–`D`
  (or `1`–`4`), at most four, the recommended one marked "recommended". The inbox walk RE-LETTERS
  so the recommended option becomes `A` (`assay-inbox.sh:697-711`, `deskinbox/format.go:299-339`),
  so a bare letter in a ruling can mean either the body's letter or the walk's.
- A ruling is a comment by the blessing authority whose first line names an offered letter
  ("B", "Option B — …") or ratifies ("ratified", "approved"); a ratification picks the letter in
  the newest roster-App relay before it (`Answer: <letter>`, ask-decision SKILL.md:168). Grammar
  reference: `assay-inbox.sh` `isruling`, :758-772.
- Decision issues filed by statusgen carry `<!-- needs-decision: <brief> -->` in the body
  (`statusgen/decisionissues.go:34-39`) — the brief join key when present.
- Local store: `deskDir()` = `~/.config/assay` (`deskkit/killswitch.go:65-73`); the audit log is
  `audit.jsonl` there (`deskkit/audit.go:88-93`). The record goes to `decision-records.jsonl`
  beside it.
- Join: this record cannot know `dispatch_ref` (as defined by desk-supervision/28); it joins by
  `repo` + `issue`, the `--tracker` ref (the brief/PR/issue carrying the work) and `brief` when the
  marker is present.
- single-point-of-failure: deskclose is the only writer — backed by two copies written by
  independent paths (the record embedded in the close comment on the forge, and the local JSONL
  line); either alone survives the other's loss. A decision closed by hand outside deskclose has no
  record (named in `consumers:`; not backed).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Schema doc** `human-decision-v1.md`. One JSON object per ruled close, `schema:
   "human-decision-v1"`. Fields:
   - `repo`, `issue` (int), `tracker` (`owner/repo#N`), `brief` (from the needs-decision marker,
     else omitted);
   - `options` — list of `{id, text_sha256}`: `id` is the option's letter **as the anchor ask
     wrote it** (body or relay), `text_sha256` the digest of its cleaned text. Never the text.
   - `options_source` — closed set `body` | `relay` (which ask the options came from);
   - `recommended` — the id marked recommended, or `none` when no option carried the mark;
   - `picked` — an offered id, or one of the closed tokens `ambiguous` (the letter means a
     different option under the walk's re-lettering than under the source's) or `unparsed`
     (no letter resolvable); `picked_via` — `direct` | `ratified-relay`;
     `picked_is_recommended` (bool, false unless `picked` is an id equal to `recommended`);
   - `ruler` — the literal role string `driver`; never a login or account id;
   - `ruling_sha256` — digest of the ruling comment body;
   - `opened_at`, `asked_at`, `ruled_at`, `recorded_at` (RFC3339 UTC); `opened_to_ruled_s`,
     `asked_to_ruled_s` (int seconds, equal to the differences);
   - `tool_sha` (the build's source SHA, as the audit Entry carries).
   Add a "Never recorded" list: option text, ruling text, any comment body, any login or account
   id, any per-person aggregate. Add two rules to the schema doc:
   - **Visibility.** The forge copy lands on the close comment's own repo, whose visibility can
     differ from the `tracker` item's repo. `tracker` is already rendered in the close comment's
     source string (`deskclose/triage.go:273`) and the outbound check scans the whole comment body,
     so the block discloses nothing the comment did not already carry — the doc states that
     `tracker` is the only repo-bearing field, that it is exactly as visible as the close comment
     itself, and that the record holds no `dispatch_ref`, no option or ruling text and no login.
   - **Authenticity.** Any commenter can post a lookalike `human-decision-v1` block. A reader
     honours a block ONLY in the close comment authored by the closing role's own App identity
     (the roster-trusted App that ran `deskclose`), never in a comment by any other author, and
     prefers the local `decision-records.jsonl` line when both copies exist.
2. **deskkit.** In `decisionrecord.go`: `ParseDecisionOptions(body)` — the exact parse now inline
   in deskinbox (`section` + line regexes + recommended detection), returning source letters, the
   recommended index and cleaned texts; `ParseRulingPick(comments, anchorIdx, rulerComment)` —
   the first-line letter / ratify grammar above, resolving a ratification through the newest
   roster-trusted App comment (`deskkit.TrustedAuthorID`) carrying `Answer: <letter>`;
   `ValidateDecisionRecord` refusing: `ruler` ≠ `driver`, `picked` outside offered ids ∪
   {`ambiguous`, `unparsed`}, unknown `options_source`/`picked_via`, more than four options,
   latency fields that disagree with the timestamps or are negative, any field containing a
   newline; `AppendDecisionRecord` — one line to `decision-records.jsonl` under `deskDir()`
   (0600, O_APPEND).
3. **Forge.** Add `Issue.CreatedAt` and fill it on both backends.
4. **deskclose.** In `applyTriage`, for `--disposition human-decided` only, after `authorizeTriage`:
   compose the record from the fetched item (body, CreatedAt), the comment thread (anchor = newest
   roster-App comment that states options, else the body), and the verified ruling comment. A record
   that fails validation is NOT written; the close still proceeds and the audit line says
   `decision-record: invalid (<field>)`. Append a fenced hidden block
   `<!-- human-decision-v1 {…} -->` (the same JSON bytes) to the close comment, so the forge copy
   lands in the existing write; dry-run prints the record and writes nothing. After the close
   succeeds, append the local line; an append failure logs `decision-record: local-unwritten` to
   the audit log and stderr and does NOT fail the close (the forge copy holds it).
5. **deskinbox.** Switch `format.go` to `deskkit.ParseDecisionOptions`; `TestParityWalk` must stay green.
6. **Docs.** ask-decision §Recording a ruling: "close a ruled decision through `deskclose triage
   --disposition human-decided`, which writes the human-decision-v1 record; a hand close writes
   none." tools/desk/README.md deskclose section: the record and its two copies. Changelog fragment.

Must NOT collect (restate in the schema doc): option or ruling text; any login/id (the ruler is the
role `driver`); per-person aggregates — analysis groups by decision class, brief or stream, and the
records are never a target for ranking the driver, the desk or any agent. No model names appear in
this record at all.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskclose/... ./cmd/deskinbox/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskclose/ -run '^TestTriageHumanDecidedRecord$' -v > "${TMPDIR:-/tmp}/b32-rec.out" 2>&1 && grep -F -e '--- PASS: TestTriageHumanDecidedRecord' "${TMPDIR:-/tmp}/b32-rec.out"` | exit 0; fixture issue (options A/B/C, B marked recommended, opened T0) + ruling "B" at T1 → the close comment's `human-decision-v1` block and the local line are byte-identical JSON with options `[A,B,C]`, `recommended: B`, `picked: B`, `picked_is_recommended: true`, `opened_to_ruled_s` = T1−T0 | check:ci +flow |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestParseRulingPick$' -v > "${TMPDIR:-/tmp}/b32-pick.out" 2>&1 && grep -F -e '--- PASS: TestParseRulingPick' "${TMPDIR:-/tmp}/b32-pick.out"` | exit 0; subtests: direct letter; "Option C — …"; "ratified" resolves through a trusted App relay's `Answer: A`; the same relay by an untrusted author → `unparsed`; a letter that names different options under source vs walk lettering → `ambiguous`; a question or "hold" → `unparsed` | check:ci |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestValidateDecisionRecord$' -v > "${TMPDIR:-/tmp}/b32-val.out" 2>&1 && grep -F -e '--- PASS: TestValidateDecisionRecord' "${TMPDIR:-/tmp}/b32-val.out"` | exit 0; each refusal case in Task 2 is refused naming its field (ruler set to a login, picked outside the set, five options, latency mismatch, newline in a field); a valid record passes | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/deskclose/ -run '^TestTriageHumanDecidedRecordNeverCollects$' -v > "${TMPDIR:-/tmp}/b32-nc.out" 2>&1 && grep -F -e '--- PASS: TestTriageHumanDecidedRecordNeverCollects' "${TMPDIR:-/tmp}/b32-nc.out"` | exit 0; the encoded record contains none of the fixture's option texts, ruling text, ruler login or App login, and `ruler` is `driver` | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskclose/ -run '^TestTriageHumanDecidedRecordStoreUnwritable$' -v > "${TMPDIR:-/tmp}/b32-st.out" 2>&1 && grep -F -e '--- PASS: TestTriageHumanDecidedRecordStoreUnwritable' "${TMPDIR:-/tmp}/b32-st.out"` | exit 0; with the local store unwritable the issue still closes, the comment still carries the block, the audit detail names `local-unwritten`; with an invalid record the close proceeds and no block is posted | check:ci |
| 7 | `cd tools/desk && go test ./cmd/deskinbox/ -run '^TestParityWalk$' -v > "${TMPDIR:-/tmp}/b32-par.out" 2>&1 && grep -F -e '--- PASS: TestParityWalk' "${TMPDIR:-/tmp}/b32-par.out"` | exit 0 (the lifted Options parse renders byte-identically to the bash oracle) | check:ci +neighbour |
| 8 | `cd tools/desk && go test ./cmd/deskclose/ -run '^TestTriageDecisionLabelControl$' -v > "${TMPDIR:-/tmp}/b32-nb1.out" 2>&1 && grep -F -e '--- PASS: TestTriageDecisionLabelControl' "${TMPDIR:-/tmp}/b32-nb1.out" && go test ./cmd/deskclose/ -run '^TestTriageDryRunWritesNothing$' -v > "${TMPDIR:-/tmp}/b32-nb2.out" 2>&1 && grep -F -e '--- PASS: TestTriageDryRunWritesNothing' "${TMPDIR:-/tmp}/b32-nb2.out" && go test ./cmd/deskclose/ -run '^TestTriageNotPlannedCloses$' -v > "${TMPDIR:-/tmp}/b32-nb3.out" 2>&1 && grep -F -e '--- PASS: TestTriageNotPlannedCloses' "${TMPDIR:-/tmp}/b32-nb3.out"` | exit 0; the needs-decision refusal, dry-run (writes nothing, no local line) and the not-planned lane (no record written) are unchanged | check:ci +neighbour |
| 9 | `statusgen --consumers --root .` | exit 0 (every routing above corroborated by the implementation diff) | check:ci |
| 10 | `R="$(tail -n 1 "$HOME/.config/assay/decision-records.jsonl")"; gh issue view "$(jq -r .issue <<<"$R")" -R "$(jq -r .repo <<<"$R")" --json createdAt --jq .createdAt; jq -r .opened_at <<<"$R"` (on the desk host, after the first real human-decided close by the built tool) | the two printed timestamps are equal; and the issue's close comment, read with `gh issue view --comments`, carries a `human-decision-v1` block whose `picked` matches the first line of the cited ruling comment | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| Record names the walk's re-lettered letter as if it were the body's, so default-acceptance is wrong | row 3 (`ambiguous` case) |
| A login or option text leaks into a public close comment | row 5; row 4 refuses a login-shaped `ruler` |
| The record write blocks or breaks the close | row 6 |
| Lifting the Options parse changes what the inbox shows | row 7 |
| Not-planned or dry-run starts writing records | row 8 |
| Latency computed from the wrong timestamp (e.g. close time, not issue creation) | row 2 (fixture T1−T0) + row 10 (dereferenced against the forge) |
| A ratification picks from an untrusted relay | row 3 (untrusted relay → `unparsed`) |
| Desks keep closing decision issues by hand, so no records exist | review-only: row 10 proves one real record; adoption of the lane is the ask-decision text (Task 6) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
