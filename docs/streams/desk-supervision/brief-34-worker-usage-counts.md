---
brief: assay:assay:desk-supervision:34
title: Worker usage counts at claim release — counts only, never transcripts
why: >-
  Nobody can say today what a worker run costs: how many tokens it used and how long it took,
  per brief and per model tier. Without that, "is the strong tier worth it for this kind of
  brief?" and "did the effort estimate match the real work?" are guesses. The figure already
  exists — the agent harness reports it to the desk when each worker finishes — and is thrown
  away. Keeping just those few numbers, in a place the operator chooses and never in the public
  repo, makes cost analysable without ever reading what a worker said or did.
wave: 1
depends: ["desk-supervision/28"]
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
  (b) the record joins to desk-supervision/28's dispatch record by `dispatch_ref`, reads the
  tier vocabulary from the model-stamp set and a new roster key that must stay in sync across
  two binaries and a coupling vector; (c) the "never a transcript" property is a privacy guard
  whose regression (a free-text field, a stdin read) survives a happy-path test.
domain: complicated
outcome: none
sources:
  - 'desk-supervision: Worker usage counts at claim release — counts only, never transcripts'
  - "driver ruling 2026-10-06 (pipeline analysis records set, desk-supervision/28-35): capture token counts (in/out), wall time and tier at claim release from the usage figure the agent run already reports; counts only, aggregated per brief/tier, stored where the adopter configures, never the public repo; never read session transcripts, prompts or tool output"
  - "desk-supervision/13 (brief-13-worker-operations-vitals.md) — the self-reported resource block and its three-state VitalField type, reused here rather than re-invented"
  - "desk-supervision/28 — defines `dispatch_ref`, the join key this record carries"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): no `deskusage` command under tools/desk/cmd/; no usage/token record is written at or after claim release by deskclaim-ref (claim.go:261 cmdRelease deletes the ref only), deskdispatch (dispatch.go:1120 releaseClaim) or the worker prompt's release line (prompt.go:453 writeReleaseClaim); the only token field anywhere is desk-supervision/13's per-session beacon `resource.tokens` (vitals.go:67-73), which is a live session vital, not a per-dispatch record"
consumers:
  - "tools/desk/cmd/deskusage/ (planned — new verb `record` + `summary`): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/usage.go (planned — record type, store resolution, strict reader): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/rosterconfig.go (knownRosterKeys gains ASSAY_USAGE_DIR): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/rosterconfig.go and statusgen/testdata/roster_coupling.json (statusgen's twin of the roster key set): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/worker-desk/SKILL.md (the refill-on-completion step records usage): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (deskusage row + the ASSAY_USAGE_DIR key): follow-up desk-supervision/34 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/records-and-retention.md (lists this record class): out-of-scope (desk-supervision/35 owns that page and lists every analysis record; this brief only supplies the location, writer and schema it will cite)"
  - "desksupervise reclaim and deskdispatch's lifecycle-failure release (dispatch.go:379/418/495): out-of-scope (no agent run completed, so no harness usage figure exists to record; a reclaimed run's cost stays unrecorded rather than estimated)"
  - "review and verify agent dispatches (pr-review-desk, verify-desk): out-of-scope (deferred to an unauthored follow-up — the same verb serves them, but this brief scopes the worker pool only, per the driver's ruling)"
  - "opmetrics (tools/desk/cmd/opmetrics): out-of-scope (it reads session transcripts by design for its own aggregates; this record must not reuse or depend on it — Task 1 keeps the two input paths disjoint)"
---

# Brief 34 — Worker usage counts at claim release — counts only, never transcripts

## Context
files:
- **add** `tools/desk/internal/deskkit/usage.go` (planned) — `UsageRecord` type (schema
  `usage-v1`), store-directory resolution, append writer, strict reader.
- **add** `tools/desk/internal/deskkit/usage_test.go` (planned).
- **add** `tools/desk/cmd/deskusage/main.go` (planned), `tools/desk/cmd/deskusage/record.go` (planned),
  `tools/desk/cmd/deskusage/summary.go` (planned) and their tests — the verb.
- **edit** `tools/desk/internal/deskkit/rosterconfig.go` — `EnvUsageDir = "ASSAY_USAGE_DIR"` and
  its entry in `knownRosterKeys()` (line 536).
- **edit** `statusgen/rosterconfig.go` (`scanKnownRosterKeys`, line 457) and
  `statusgen/testdata/roster_coupling.json` — the twin of the new key (the file's own
  KEEP-IN-SYNC rule).
- **edit** `plugins/assay/skills/worker-desk/SKILL.md` — the "Fill to N, refill on completion"
  bullet (line 127) gains the record step.
- **edit** `tools/desk/README.md` — a `deskusage` row in the tool table and the
  `ASSAY_USAGE_DIR` key.
- **add** `changelog/desk-supervision-34.md` (planned).

facts:
- **Who releases the claim, and who sees the usage figure (read 2026-10-06 @ 1fbf1153f).** The
  claim is `refs/dispatch/<id>` (`tools/desk/cmd/deskclaim-ref/main.go:11-13`); `release` only
  deletes it (`claim.go:261-275`). A WORKER releases its own claim as its last step — the
  dispatch prompt tells it to (`tools/desk/cmd/deskdispatch/prompt.go:453-463`). The worker
  cannot read its own run's usage figure; the harness reports it to the DISPATCHING desk session
  in the worker's completion notification, which is the event the worker-desk skill already acts
  on ("Fill to N, refill on completion", `plugins/assay/skills/worker-desk/SKILL.md:127`). So
  "at claim release" is implemented as: the desk records usage at that completion event, which
  follows the worker's own release. The claim tools are NOT changed.
- **`dispatch_ref`** is as defined by desk-supervision/28: a per-RUN id `<claim_key>@<YYYYMMDDTHHMMSSZ>`
  minted at claim acquire (the claim key is `claimKeyFor`, `tools/desk/cmd/deskdispatch/dispatch.go:1829-1834`,
  per ITEM; the acquire timestamp makes it per run) and readable in the worker worktree as
  `git config --worktree assay.dispatchRef`. A usage record therefore joins to exactly one
  dispatch record by `dispatch_ref` alone.
- **Tier vocabulary** is `deskkit.DispatchTiers()` = {`any`, `strong`}
  (`tools/desk/internal/deskkit/modelstamp.go:123`), the same set `deskdispatch --tier` validates
  against (`dispatch.go:1781`). The record stores the tier slug only, never a model name. (The
  forge stamp label `dispatched-<model>` already exposes a model slug on the PR; this record does
  not add one.)
- **Three-state counts reuse desk-supervision/13's type.** `deskkit.VitalField` with
  `MeasuredInt`, `CouldNotCheckVital` and nil-as-null (`tools/desk/internal/deskkit/vitals.go:30-62`)
  — a count the notification did not carry is null, never 0; a measured 0 stays 0. Reuse it; no
  second three-state type.
- **The harness figure may be a total only.** Some harnesses report one total-token count, a
  duration and a tool-use count rather than an in/out split. The record therefore carries
  `tokens_in`, `tokens_out` AND `tokens_total`, each three-state; whichever the notification
  does not report is null. No field is derived or estimated from another.
- **Storage.** The desk's local state directory is `deskkit.StateDir()` (`~/.config/assay` by
  default, `tools/desk/internal/deskkit/killswitch.go:65-90`); the audit log is `audit.jsonl`
  there (`audit.go:88-94`). Usage records go to `<ASSAY_USAGE_DIR or StateDir()/usage>/usage.jsonl`,
  append-only JSONL, mode 0600 — never committed per event, never under a git working tree.
- **Roster keys are a closed, synced set.** A new `ASSAY_*` key must be added to
  `knownRosterKeys()` (`tools/desk/internal/deskkit/rosterconfig.go:536`), statusgen's
  `scanKnownRosterKeys()` (`statusgen/rosterconfig.go:457`) and the coupling vector
  `statusgen/testdata/roster_coupling.json`, or an adopter roster carrying it fail-closes.
- **What already reads transcripts, and must stay separate.** `opmetrics` reads session
  transcripts by design (`tools/desk/cmd/opmetrics/main.go:8-17`). This record has NO transcript
  input path: the verb reads only its own flags.
- Refusal convention: `deskkit.Refused(...)` → exit 5 (`exitcodes.go:168`); unverifiable → exit
  6; kill switch first; one audit line per invocation.
- single-point-of-failure: the writer's input surface — flags only, integer counts, slug fields
  matching closed grammars, no stdin/file read. The layer behind it is the reader: `deskusage
  summary` decodes every stored line strictly (unknown key or non-integer count → refused, line
  named), so a store polluted by a hand append or a future writer regression is reported, not
  aggregated. They fail for different reasons (input parsing vs stored-line decoding) at
  different times (write vs read).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Record type (`tools/desk/internal/deskkit/usage.go` (planned)).** `UsageRecord{Schema "usage-v1", TS (RFC3339 UTC),
   DispatchRef, Repo (owner/name), Item (`<stream>/<NN>` or `issue-<N>`), Tier, TokensIn,
   TokensOut, TokensTotal, WallSeconds}` — the four counts are `*VitalField`. NO other field: no
   session id, no person, no model name, no free text, no PR title, no path. Each string field
   is validated against a closed grammar (`dispatch_ref` = desk-supervision/28's `<claim_key>@<ts>` grammar; `repo` =
   owner/name; `item` = `<stream>/<NN>` or `issue-<N>`; `tier` ∈ `DispatchTiers()`).
2. **Store.** Resolve the directory: `ASSAY_USAGE_DIR` (one absolute path) when set, else
   `StateDir()/usage`. REFUSE (exit 5) a directory that is, or is inside, a git working tree
   (walk up looking for `.git`) — the counts never land in a repository. Register the key in the
   three roster sites listed in Context. Append one JSON line per record with an exclusive lock,
   file mode 0600.
3. **Verb `deskusage record <dispatch_ref> --repo O/R --item ITEM --tier any|strong
   [--tokens-in N] [--tokens-out N] [--tokens-total N] [--wall-seconds N]`.** Flags only. Each
   count flag takes a non-negative integer or the literal `could-not-check`; an omitted count is
   null. Anything else — a non-integer, a negative number, an unknown flag, a positional beyond
   `dispatch_ref` — is refused (exit 5) and nothing is written (the store file is byte-identical).
   The verb never reads stdin and takes no file argument.
4. **Verb `deskusage summary [--by item|tier] [--json]`.** Aggregates only: per group, the
   record count and, per count field, the number measured, the sum and the median of measured
   values, and the number null / could-not-check. It never prints `dispatch_ref` or a single
   record. Decode every stored line strictly (`DisallowUnknownFields`, count = integer or
   `could-not-check` or null); an unknown key or bad value is refused (exit 6, naming the line
   number), never skipped silently.
5. **Skill.** In worker-desk's "Fill to N, refill on completion" bullet: when a worker's
   completion notification arrives, before refilling the slot, run `deskusage record` with the
   claim key the dispatch used, the item, the dispatched tier, and only the counts the
   notification itself reports; omit any it does not carry. State plainly: never read, quote or
   pass anything from the worker's transcript, prompt or tool output; the figures are not a
   target for ranking agents or people. A refusal is logged and does not block the refill.
6. **Docs + changelog.** `tools/desk/README.md`: a `deskusage` row (local-only write, never a
   repo) and the `ASSAY_USAGE_DIR` key. Changelog fragment: the new record, what it never
   collects.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskusage/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskusage/ -run '^TestUsageRecord_RefusesTranscriptShapedInput$' -v > "${TMPDIR:-/tmp}/b34-transcript.out" 2>&1 && grep -F -e '--- PASS: TestUsageRecord_RefusesTranscriptShapedInput' "${TMPDIR:-/tmp}/b34-transcript.out"` | exit 0; subtests: a transcript-shaped JSON line (a `message` object with `content` text carrying a canary string) passed as `--tokens-in` → exit 5, store byte-identical; the same transcript piped on stdin with otherwise valid flags → record written, stdin never read, canary absent from the store; `--transcript`/`--prompt` flags → exit 5 | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskusage/ -run '^TestUsageSummary_RefusesPollutedStore$' -v > "${TMPDIR:-/tmp}/b34-polluted.out" 2>&1 && grep -F -e '--- PASS: TestUsageSummary_RefusesPollutedStore' "${TMPDIR:-/tmp}/b34-polluted.out"` | exit 0; a store with one hand-appended line carrying a `prompt` key → `summary` exits 6 naming that line number; a store of valid lines → aggregates printed with no `dispatch_ref` in the output (the reader layer, with the writer bypassed) | check:ci +mutation |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestUsageStore_RefusesGitWorkTree$' -v > "${TMPDIR:-/tmp}/b34-gitdir.out" 2>&1 && grep -F -e '--- PASS: TestUsageStore_RefusesGitWorkTree' "${TMPDIR:-/tmp}/b34-gitdir.out"` | exit 0; `ASSAY_USAGE_DIR` pointing inside a temp dir carrying `.git` → refused, nothing written; unset → resolves under the (test-overridden) state dir | check:ci +mutation |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestUsageRecord_ThreeState$' -v > "${TMPDIR:-/tmp}/b34-3state.out" 2>&1 && grep -F -e '--- PASS: TestUsageRecord_ThreeState' "${TMPDIR:-/tmp}/b34-3state.out"` | exit 0; omitted count → JSON `null`; `could-not-check` → that string; measured `0` → `0`; a vendor model name as `--tier` → refused | check:ci |
| 6 | `cd tools/desk && go test ./cmd/deskusage/ -run '^TestUsage_RecordToSummaryFlow$' -v > "${TMPDIR:-/tmp}/b34-flow.out" 2>&1 && grep -F -e '--- PASS: TestUsage_RecordToSummaryFlow' "${TMPDIR:-/tmp}/b34-flow.out"` | exit 0; roster key → store dir → three `record` calls (two items, two tiers) → `summary --by tier --json` shows the right counts, sums and measured/null tallies; each stored `dispatch_ref` equals what deskdispatch's claim-key derivation yields for the same item and repo | check:ci +flow |
| 7 | `(cd statusgen && go test . -run '^TestRosterKeySchemaCoupling$' -v > "${TMPDIR:-/tmp}/b34-sg-roster.out" 2>&1 && grep -F -e '--- PASS: TestRosterKeySchemaCoupling' "${TMPDIR:-/tmp}/b34-sg-roster.out") && (cd tools/desk && go test ./internal/deskkit/ -run '^TestRosterKeySchemaCoupling$' -v > "${TMPDIR:-/tmp}/b34-dk-roster.out" 2>&1 && grep -F -e '--- PASS: TestRosterKeySchemaCoupling' "${TMPDIR:-/tmp}/b34-dk-roster.out") && grep -F -e 'ASSAY_USAGE_DIR' statusgen/testdata/roster_coupling.json` | exit 0 (the new key is in the coupling vector and both binaries' key-schema coupling tests pass) | check:ci +neighbour |
| 8 | `grep -n 'deskusage record' plugins/assay/skills/worker-desk/SKILL.md && grep -n -i 'transcript' plugins/assay/skills/worker-desk/SKILL.md` | exit 0; the record step sits in the refill-on-completion bullet and the same passage forbids transcript content | check |
| 9 | `grep -n 'func cmdRelease' -A6 tools/desk/cmd/deskclaim-ref/claim.go && git diff --quiet 1fbf1153f -- tools/desk/cmd/deskclaim-ref/` | exit 0; release still only removes the ref and the claim tool is untouched (the claim of Context fact 1 dereferenced against code) | check +dereference |
| 10 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A free-text or transcript-carrying input path creeps in (stdin, a file flag, a string count) | row 2 |
| The writer guard regresses and a polluted line lands; summary aggregates it anyway | row 3 |
| Counts land inside a repo (operator points the dir at a checkout) | row 4 |
| An unreported count is stored as 0, skewing every average | row 5 |
| `dispatch_ref` does not match the dispatch record, so nothing joins | row 6 |
| The new roster key fail-closes an adopter's fleet because one binary does not know it | row 7 |
| The skill never tells the desk to record, so the store stays empty in production | row 8 |
| Someone "simplifies" by adding usage flags to the claim tool, which the worker runs blind | row 9 |
| The figures get used to rank agents or people | no row — review-only (a use-of-data norm stated in the skill and the summary's aggregate-only output; no mechanism can test intent) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
