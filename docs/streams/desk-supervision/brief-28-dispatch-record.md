---
brief: assay:assay:desk-supervision:28
title: Dispatch record — one line per dispatch joining brief, PR, session and model tier
why: >-
  When the desk launches an agent onto a brief or a pull request, nothing durable says which brief,
  which pull request, which session and which model tier that launch was. The only trace is a label
  on the pull request (and only once one exists) plus a free-text audit line, so no later analysis
  can ask "how often does a strong-tier worker need a second dispatch?" or connect a verify failure
  back to the dispatch that produced the work. This brief writes one structured line per dispatch,
  locally beside the audit log, and defines `dispatch_ref` — the one key every other pipeline record
  uses to point at a dispatch.
wave: 0
depends: []
unblocks: ["desk-supervision/29", "desk-supervision/30", "desk-supervision/34", "desk-supervision/35"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (pipeline analysis records)
exec-tier: strong
exec-tier-why: >-
  (a) the dispatch_ref shape and the release-side pairing are design choices the facts constrain but
  do not fully pre-specify; (b) the key is minted in deskdispatch, persisted in worktree config, read
  back by deskclaim-ref at release and consumed by sibling records, so correctness is cross-component.
domain: complicated
outcome: none
sources:
  - "research note (driver-held, 2026-10-06): an independent review of applying decision models and statistical learning to the pipeline found the records cannot be joined; the only key surviving every stage is brief → PR → head SHA → dispatch claim/session → verify receipt"
  - "sibling verify-outcome join-keys brief (brief 27, merged in #2287) — adds an OPTIONAL `dispatch_ref` to verify-outcome records and defers its definition to a dispatch-side brief; this is that brief"
  - "code read 2026-10-06 @ 1fbf1153f: the claim key is per ITEM, not per dispatch (claimKeyFor, tools/desk/cmd/deskdispatch/dispatch.go:1800-1834); a re-review re-acquires the SAME key (plugins/assay/skills/pr-review-desk/SKILL.md:345-348), so the bare claim id cannot identify one dispatch"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main): no dispatch record exists; deskdispatch's only durable outputs are the forge claim ref, the `assay.runKey` worktree config (dispatch.go:464-478), the `dispatched-model:`/`dispatched-tier:` labels when --pr is known (dispatch.go:1494-1512; deskkit/modelstamp.go:106,110), and one audit line whose repo/pr/headSHA fields deskdispatch leaves empty (dispatch.go:1944-1967)"
consumers:
  - "tools/desk/cmd/deskdispatch/dispatch.go (mints dispatch_ref after claim-acquire, records assay.dispatchRef at worktree-create, writes the record at model-stamp): fixed-here (helpers in cmd/deskdispatch/dispatchrecord.go)"
  - "tools/desk/cmd/deskdispatch/brief.go (reads the brief's own `brief:`, `exec-tier:` and `effort:` frontmatter): fixed-here"
  - "tools/desk/internal/deskkit/dispatchrecord.go (the record type, validator and append writer): fixed-here"
  - "tools/desk/cmd/deskclaim-ref/claim.go (cmdRelease writes the `released` line): fixed-here"
  - "tools/desk/README.md (new `The dispatch record` section, covering deskdispatch and deskclaim-ref): fixed-here"
  - "worker usage counts at claim release (joins on dispatch_ref): follow-up desk-supervision/34"
  - "records-and-retention page (lists this record's location, writer, schema and retention): follow-up desk-supervision/35"
  - "verify-outcome records' optional dispatch_ref (sibling brief 27, merged in #2287): out-of-scope (that brief landed separately and reads this definition; its verifier reads `assay.dispatchRef` from its own dispatched worktree, which this brief writes)"
  - "tools/desk/internal/deskkit/killswitch.go (per-run stop reads assay.runKey): out-of-scope (assay.runKey is left byte-identical; the new assay.dispatchRef is a separate key — Verify row 7 proves the neighbour is untouched)"
  - "tools/dispatch-claim.sh (the legacy bash claim tool): out-of-scope (fallback for trees without the Go binary; it writes no released line, which this brief documents as a known gap rather than porting record-writing into shell)"
---

# Brief 28 — Dispatch record — one line per dispatch joining brief, PR, session and model tier

## Context
files:
- **add** `tools/desk/internal/deskkit/dispatchrecord.go` (planned) + `dispatchrecord_test.go` (planned) —
  `DispatchRecord` type, `ValidateDispatchRecord`, `AppendDispatchRecord`, `MintDispatchRef`.
- **edit** `tools/desk/cmd/deskdispatch/dispatch.go` — mint the ref after step 1, record it at step 2,
  write the record at step 5.
- **edit** `tools/desk/cmd/deskdispatch/brief.go` — read `brief:`, `exec-tier:`, `effort:` with the
  existing regex extractor.
- **add** `tools/desk/cmd/deskdispatch/dispatchrecord_test.go` (planned).
- **edit** `tools/desk/cmd/deskclaim-ref/claim.go` + a test — the `released` line.
- **edit** `tools/desk/README.md` — deskdispatch and deskclaim-ref sections.
- **add** `changelog/desk-supervision-28.md` (planned).

facts (all read 2026-10-06 @ 1fbf1153f):
- **The claim id is per item, not per dispatch.** `claimKeyFor(item, repo)`
  (`tools/desk/cmd/deskdispatch/dispatch.go:1829-1834`): a key already containing `--` passes
  through; otherwise `deskkit.RepoShortLabel(repo) + "--" + item with "/" → "--"`, e.g.
  `assay--desk-supervision--28`. The claim lands at `refs/dispatch/<key>`
  (`deskclaim-ref/claim.go:32`; `deskkit/claimref.go:71` `DispatchClaimActiveRefsPrefix`) under the
  default forge-ref store; `deskkit.ResolveClaimStore` (`deskkit/claimstore.go:208`) may choose
  another store, but the key is the same in every store.
- **The same key recurs.** A released claim is re-acquired under the same key on re-dispatch; a
  stale claim is reclaimed under the same key (`cmdAcquire` → `cmdSteal`, `claim.go:217-221`); and a
  re-review runs "the SAME original-key ceremony … never change the key"
  (`plugins/assay/skills/pr-review-desk/SKILL.md:345-348`). There is no `--rr`/attempt flag in
  deskdispatch (`dispatch.go:149-171`, the full flag set). The claim tag's date is not stable either:
  `progress` rewrites the tag (`claim.go:244`).
- **Exclusivity does NOT make (key, acquire time) unique.** While a claim is held, a second
  acquire is refused (exit 5), so at most one live dispatch exists per key at any INSTANT. That
  says nothing about two runs one after the other: a release followed by a re-acquire inside the
  same wall-clock second (deskdispatch's own abort-then-retry path, a fast stale reclaim) yields the
  same key at the same one-second timestamp. A timestamp alone therefore cannot make a per-run id;
  the definition below adds a random component.
- deskdispatch already records `assay.runKey = <claim key>` in the agent worktree's
  `git config --worktree` (`dispatch.go:464-478`); `deskkit/killswitch.go:38-39` reads it for the
  per-run stop. Item keys match `itemKeyRe` `^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`
  (`dispatch.go:100`), so a claim key never contains `@`.
- Step order is pinned: claim-acquire, worktree-create, roster-register, decision-gate, model-stamp,
  queue-label, prompt-emit (`dispatch.go:60-68`). `stepStamp` returns `SKIPPED` (no `--model`),
  `PENDING` (no `--pr`) or applies `dispatched-model:<slug>` + `dispatched-tier:<tier>`
  (`dispatch.go:1494-1512`; prefixes `deskkit/modelstamp.go:106,110`).
- Audit: `deskkit.Entry` (`deskkit/audit.go:66-87`) has `repo`, `pr`, `headSHA`, `sessionTag` but no
  brief; deskdispatch's `audit()` (`dispatch.go:1944-1967`) sets only tool/verb/result/detail/title.
  It is written to `<StateDir>/audit.jsonl` (`audit.go:340`), StateDir = `~/.config/assay`
  (`deskkit/killswitch.go:65-80`), mode 0600. `deskkit.SessionTag()` (`audit.go:270-281`) is the
  session tag source. Per-step timing exists only under `DESK_TRACE=1` (`deskdispatch/main.go:211`).
- `deskclaim-ref release <id>` (`claim.go:261-275`) deletes the claim and logs `released <id>`
  (or `(no claim — no-op)`); deskdispatch's abort path calls the same tool with cwd = `--root`
  (`releaseClaim`, `dispatch.go:1120-1121`).
- single-point-of-failure: the dispatch_ref minted once in deskdispatch — backed by the record
  validator (refuses a ref whose prefix is not the record's own `claim_key`, or whose suffix is not
  the timestamp-plus-random shape). Uniqueness rests on the random component, not on the claim
  ref: exclusivity only rules out two LIVE claims at one instant. A collision needs the same key,
  the same second and the same 48 random bits.

### `dispatch_ref` — the definition (authoritative for desk-supervision/29–35)
`dispatch_ref = <claim_key> "@" <acquired_at> "." <nonce>`, where `claim_key` is exactly
`claimKeyFor(item, repo)` (the `<id>` of `refs/dispatch/<id>`), `acquired_at` is the UTC time
deskdispatch read immediately after claim-acquire returned 0, in ISO-8601 basic form
`YYYYMMDDTHHMMSSZ`, and `nonce` is 12 lowercase hex characters (48 bits) drawn from a
cryptographically secure source (`crypto/rand`) at mint time. Full grammar:
`^<claim_key>@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$` for the record's own `claim_key`. Example:
`assay--desk-supervision--28@20261006T141502Z.3fa9c01b7d2e`. The claim id alone is NOT the key: it
names the item, and every re-dispatch, re-review and stale reclaim reuses it. The timestamp alone
is NOT enough either: one-second resolution lets two runs of the same item share it, which is why
the nonce is part of the definition. If the secure source fails, the mint returns an error, the
dispatch proceeds with `dispatch_ref` null and the WARNING of Task 3 — it never falls back to a
weaker source and never invents a ref.

**Visibility rule.** `dispatch_ref` embeds the claim key (a repo label and an item key), so it is
local state: it is written in clear only to the desk's local state directory and the agent
worktree's config, and NEVER in clear to any non-private surface — a forge comment, label or
review body, a public register, or a file committed to git. A consumer that must express the join
on such a surface writes at most a sha256 digest of the FULL ref (every consumer in
desk-supervision/29–35 and the sibling verify-outcome brief must follow this). The 48-bit nonce is
what makes that digest not recoverable by enumerating claim keys and timestamps; the digest is
still a stable identifier, not a secret, and a consumer must not rely on it for confidentiality of
anything beyond the claim key. A record that must point at all
dispatches of one item uses `claim_key` (the part before `@`). A record that cannot know
`dispatch_ref` joins by PR (`repo` + `pr` number + head SHA), or by `repo` + `branch` = the PR's head
branch for a fresh worker dispatch whose PR did not yet exist.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Record type (deskkit).** Add `DispatchRecord` with JSON keys: `schema` (const
   `dispatch-record-v1`), `event` (`dispatched` | `released`), `ts` (RFC3339), `dispatch_ref`
   (string or null), `claim_key`, `repo` (owner/name), `item` (the item key as given), `brief`
   (the brief's frontmatter `brief:` id, or null), `kit` (worker/review/verifier…), `branch`
   (null for a detached review/verifier dispatch), `pr` (int or null), `session_tag`
   (`deskkit.SessionTag()`), `tier` (`--tier`: `any`|`strong`), `brief_exec_tier` and
   `brief_effort` (frontmatter values or null), `model_stamp` (`applied`|`pending`|`skipped`),
   `attempt_local` (int — 1 + count of earlier `dispatched` lines with the same `claim_key` in THIS
   store; a local ordinal, documented as such). `MintDispatchRef(claimKey, t, rnd io.Reader)` builds the ref from an injected clock value and an
   injected entropy source (production passes `time.Now().UTC()` and `crypto/rand.Reader`), so a test
   can fix both.
   `ValidateDispatchRecord` refuses: unknown schema/event; `tier` or `brief_exec_tier` outside
   `any`/`strong` (so a vendor model name can never be a tier value); a `dispatch_ref` that does not
   parse as `<claim_key>@YYYYMMDDTHHMMSSZ.<12 lowercase hex>` for the record's OWN `claim_key`; `brief_effort` outside
   `S`/`M`/`L`; a `kit` outside the kit vocabulary; a `repo`, `item`, `brief`, `branch` or
   `session_tag` outside its identifier grammar (for `repo`, every slug a forge can host, since
   the dispatcher admits a repo by roster membership alone; for `item` and `branch`, the
   dispatcher's own rules within the 256-byte cap; `<stream>/<NN>` or
   `<cell>:<alias>:<stream>:<NN>` for `brief`; one token for `session_tag`); any string field over
   256 bytes or containing a control
   character. There is no free-text field, because the validator refuses one, and no model slug:
   the record carries tier only (the model slug stays on the PR's existing stamp label, joinable
   via `pr`). The writer drops what it cannot record rather than lose the line: an out-of-grammar
   `brief`, `brief_exec_tier` or `brief_effort` value and a branch over 256 bytes are recorded
   null, and an out-of-grammar session as `unknown`.
2. **Writer.** `AppendDispatchRecord` validates, then appends one line to
   `<StateDir>/dispatch-records.jsonl` (mode 0600, O_APPEND), beside `audit.jsonl`. Never committed
   to git. No rotation in this brief (low volume: one line per dispatch).
3. **deskdispatch.** Immediately after `stepClaim` returns nil (`dispatch.go:339`), mint
   `dispatch_ref` from `plan.claimKey`, the current UTC time and a fresh nonce. The clock and the
   entropy reader are fields on the dispatch plan so a test can inject them. At worktree-create, next to
   `assay.runKey`, record `git config --worktree assay.dispatchRef <dispatch_ref>`; a failure there
   is a WARNING, same as the runKey one. After the model-stamp step returns (step 5), write one
   `dispatched` record (`model_stamp` from the step's outcome). A record-write failure prints
   `deskdispatch: WARNING: could not write dispatch record: …` and NEVER fails a dispatch whose
   claim and worktree stand. `--dry-run` writes no record and mints no ref. Also add
   `dispatch_ref=<ref>` to the audit `detail` of a prepared dispatch (the audit log is local state,
   mode 0600, so the visibility rule allows it).
4. **Brief frontmatter.** In `brief.go`, read `brief:`, `exec-tier:` and `effort:` with the same
   extractor as `gate:`; unreadable or absent → null, never a guess.
5. **Release line (deskclaim-ref).** In `cmdRelease`, after a delete that removed an EXISTING
   claim, append a `released` record: `claim_key` = the id, `dispatch_ref` = the cwd worktree's
   `assay.dispatchRef` IF its prefix before `@` equals the id, else null; `repo` from `--repo`; all
   dispatch-only fields null. A no-op release writes nothing. A write failure warns on stderr and
   never changes the release's exit code. A `released` line with null `dispatch_ref` pairs with the
   latest earlier `dispatched` line for the same `claim_key` (exclusivity makes that unique);
   document this rule.
6. **Docs + changelog.** `tools/desk/README.md`: the record path, the `dispatch_ref` definition
   (copy the Context subsection), the pairing rule, the legacy bash claim tool gap, and what is never
   recorded (task 1's exclusions). Changelog fragment with highlight bullets.

Never recorded (state in the README too): prompt text, brief body, PR/issue text, tool output,
transcripts, vendor model names, any per-person metric. These records are for aggregate analysis
per brief / tier / kit, never for ranking people or agents.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/... ./cmd/deskclaim-ref/... ./internal/deskkit/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDispatchRecord_Refusals$' -v > "${TMPDIR:-/tmp}/b28-refusals.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecord_Refusals' "${TMPDIR:-/tmp}/b28-refusals.out"` | exit 0; subtests each refused: tier `opus-4.8`; brief_exec_tier `fast`; dispatch_ref `other--x--1@20261006T141502Z.3fa9c01b7d2e` on a record whose claim_key is `assay--x--1`; dispatch_ref without `@`; dispatch_ref with no nonce (`assay--x--1@20261006T141502Z`); a nonce of the wrong length, with uppercase hex or with a non-hex character; effort `XL`; a field holding a newline; unknown event; prose, a link, markup, U+202E, U+200B or U+2028 in `brief`; prose in `item`, `branch`, `repo` and `session_tag`; a `kit` outside the vocabulary. A valid `dispatched` and a valid `released` record are accepted, and so is each identifier shape the dispatcher writes | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordWrittenAtModelStamp$' -v > "${TMPDIR:-/tmp}/b28-write.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordWrittenAtModelStamp' "${TMPDIR:-/tmp}/b28-write.out"` | exit 0; a stubbed real dispatch with `--brief` writes exactly ONE `dispatched` line whose `claim_key` equals the key passed to the stubbed `acquire`, whose `dispatch_ref` starts with that key + `@`, and whose `brief`/`brief_exec_tier`/`brief_effort` match the fixture brief's frontmatter; a `--dry-run` writes zero lines | check:ci |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRefRecordedInWorktree$' -v > "${TMPDIR:-/tmp}/b28-wt.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRefRecordedInWorktree' "${TMPDIR:-/tmp}/b28-wt.out"` | exit 0; the worktree's `assay.dispatchRef` equals the record's `dispatch_ref`, and two sequential dispatches of the same item (release between) produce two DIFFERENT refs with the same `claim_key` and `attempt_local` 1 then 2 — run with the injected clock FIXED to one instant for both dispatches (the same-second case) and a seeded entropy reader that yields different bytes each call, so the result never depends on wall-clock timing; a second subtest with a failing entropy reader leaves `dispatch_ref` null, prints the WARNING and still exits 0 | check:ci +flow |
| 5 | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run '^TestReleaseWritesReleasedRecord$' -v > "${TMPDIR:-/tmp}/b28-rel.out" 2>&1 && grep -F -e '--- PASS: TestReleaseWritesReleasedRecord' "${TMPDIR:-/tmp}/b28-rel.out"` | exit 0; subtests: release of a held claim from a worktree carrying a matching `assay.dispatchRef` writes a `released` line with that same ref; a non-matching `assay.dispatchRef` yields `dispatch_ref: null`; a no-op release writes nothing; an unwritable record store leaves the release exit code 0 | check:ci +flow |
| 6 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordFailureNeverFailsDispatch$' -v > "${TMPDIR:-/tmp}/b28-nonfatal.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordFailureNeverFailsDispatch' "${TMPDIR:-/tmp}/b28-nonfatal.out"` | exit 0; with the record store unwritable the dispatch exits 0, prints the WARNING line, and the claim is NOT released | check:ci +mutation |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordsRunKey$' -v > "${TMPDIR:-/tmp}/b28-runkey.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordsRunKey' "${TMPDIR:-/tmp}/b28-runkey.out"` | exit 0 (the existing `assay.runKey` behaviour the per-run stop reads is unchanged) | check:ci +neighbour |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing above is corroborated by the implementation diff) | check:ci |
| 9 | `L=$(tail -n 1 "$HOME/.config/assay/dispatch-records.jsonl") && K=$(printf '%s' "$L" \| jq -r .claim_key) && R=$(printf '%s' "$L" \| jq -r .repo) && printf '%s' "$L" \| jq -e --arg k "$K" '.event == "dispatched" and (.dispatch_ref \| startswith($k + "@"))' && deskclaim-ref show "$K" --repo "$R"` (run right after one real dispatch, while its agent still holds the claim) | exit 0; jq prints `true`; `deskclaim-ref show` prints `HELD` followed by that same key — the recorded `claim_key` is the live claim on the forge, not a re-derivation | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| `dispatch_ref` defined as the bare claim id, so a re-review collides with the first review | row 4 (two dispatches, two refs) |
| Two runs of one item inside the same second share a ref, silently merging them in every join | row 4 (clock fixed to one instant, refs still differ), row 2 (a ref with no nonce is refused) |
| The ref is written in clear to a forge comment, a public register or a committed file | review-only on the consumers' diffs — the visibility rule is the definition; a mechanical check belongs to each consumer's own brief |
| Recorded `claim_key` is re-derived and drifts from the key actually acquired (e.g. alias config) | row 3 (equals the stubbed acquire's key) + row 9 (resolves against the live claim) |
| A vendor model name lands in `tier`, or free text slips into a field | row 2 |
| Record write failure breaks or aborts dispatches | row 6 |
| Release line never links back (worktree key not written, or read from the wrong cwd) | rows 4 and 5 |
| `assay.runKey` accidentally replaced, breaking the per-run stop | row 7 |
| `attempt_local` read as a global attempt count across machines | review-only — README wording; the field name says local |
| Legacy bash claim tool releases leave no `released` line | review-only — documented gap; the Go tool is the preferred path |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
