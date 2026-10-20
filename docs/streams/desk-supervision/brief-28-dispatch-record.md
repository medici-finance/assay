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
   `session_tag` outside its identifier grammar (for `repo`, every forge-name slug up to 256 bytes
   in all, since the dispatcher admits a repo by roster membership alone, and a longer rostered
   repo loses its line with a WARNING because `repo` is not nullable; for `item` and `branch`, the
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/... ./cmd/deskclaim-ref/... ./internal/deskkit/...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDispatchRecord_Refusals$' -v > "${TMPDIR:-/tmp}/b28-refusals.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecord_Refusals' "${TMPDIR:-/tmp}/b28-refusals.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordWrittenAtModelStamp$' -v > "${TMPDIR:-/tmp}/b28-write.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordWrittenAtModelStamp' "${TMPDIR:-/tmp}/b28-write.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRefRecordedInWorktree$' -v > "${TMPDIR:-/tmp}/b28-wt.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRefRecordedInWorktree' "${TMPDIR:-/tmp}/b28-wt.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run '^TestReleaseWritesReleasedRecord$' -v > "${TMPDIR:-/tmp}/b28-rel.out" 2>&1 && grep -F -e '--- PASS: TestReleaseWritesReleasedRecord' "${TMPDIR:-/tmp}/b28-rel.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordFailureNeverFailsDispatch$' -v > "${TMPDIR:-/tmp}/b28-nonfatal.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordFailureNeverFailsDispatch' "${TMPDIR:-/tmp}/b28-nonfatal.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordsRunKey$' -v > "${TMPDIR:-/tmp}/b28-runkey.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordsRunKey' "${TMPDIR:-/tmp}/b28-runkey.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
| 9 | `L=$(tail -n 1 "$HOME/.config/assay/dispatch-records.jsonl") && K=$(printf '%s' "$L" \| jq -r .claim_key) && R=$(printf '%s' "$L" \| jq -r .repo) && printf '%s' "$L" \| jq -e --arg k "$K" '.event == "dispatched" and (.dispatch_ref \| startswith($k + "@"))' && deskclaim-ref show "$K" --repo "$R"` | fail exit=1 | sha256:dedb70ddf05e | 2026-10-11 | assay-verifier-app[bot] @ f68adb19ca4e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2624 run=524387fc0c1ac5137d86a7b7ff00ee49a231ee4907f8f81813fca9a78f07effe source=f68adb19ca4eecc307b04ebab90b437e39fd47a4 brief=docs/streams/desk-supervision/brief-28-dispatch-record.md model=claude-opus-5-5 tier=any

### Verifier pass — 2026-10-11, assay-verifier-app[bot] (verify-desk, non-implementer), merged main f68adb19c
Delivering change: 8164deac5 (the commit on main carrying the `Brief: desk-supervision/28` trailer; single-parent squash onto 40b45182b). Host: darwin, go1.27.1, git 2.56.0, statusgen and desk-tools v1.0.39. Admission: deskdispatch --check-verifier exit 0 before the first row and again before this write (binding above).
The witness table above records rows 1-8 (check:ci) as could-not-run: its network-off sandbox is `unshare --net`, which this darwin host lacks. Each of those rows was run by hand at f68adb19c: once as written, once as a diagnostic with GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 and -count=1, and rows 1-7 a third time under a macOS sandbox-exec profile denying outbound network except loopback (an external HTTPS probe under the same profile failed to connect). Test output files went to TMPDIR outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1 | exit 0 | exit 0; `ok` for deskdispatch (177.1s), deskclaim-ref (3.4s), deskkit (240.9s), deskkit/untrustcorpus. Isolated git config: exit 0, same four `ok`. Network denied: exit 0, same four `ok` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 | exit 0; every listed refusal subtest refuses; valid records accepted | exit 0; grep prints `--- PASS: TestDispatchRecord_Refusals`; 66 PASS lines, 0 FAIL, including tier_is_a_model_name, brief_exec_tier_fast, ref_of_another_claim_key, ref_without_@, ref_with_no_nonce, nonce_too_short/too_long/uppercase_hex/non-hex_char, effort_XL, field_holds_a_newline, unknown_event, brief_is_prose/link/markup, brief_has_RLO/ZWSP/LSEP, item/branch/repo/session prose, kit_outside_set, valid_dispatched_accepted, valid_released_accepted. Isolated git config: exit 0, 66 PASS, 0 FAIL. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 | exit 0; one dispatched line, claim_key equals acquired key, frontmatter fields match; dry run writes zero | exit 0; `--- PASS: TestDispatchRecordWrittenAtModelStamp`; subtests real_dispatch_writes_one_line and dry_run_writes_nothing PASS (8 PASS lines, 0 FAIL). Isolated git config: exit 0, 8 PASS. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 | exit 0; worktree ref equals record ref; two same-second dispatches give two refs, attempt_local 1 then 2; failing entropy gives null ref + WARNING + exit 0 | exit 0; `--- PASS: TestDispatchRefRecordedInWorktree`; subtests two_dispatches_two_refs and failing_entropy_leaves_null PASS. Test body read: clock fixed to 2026-10-06T14:15:02Z for both dispatches, counting entropy reader. Isolated git config: exit 0, 3 PASS. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 | exit 0; matching ref carried, non-matching null, no-op writes nothing, unwritable store keeps exit 0 | exit 0; `--- PASS: TestReleaseWritesReleasedRecord`; subtests matching_ref_is_carried, other_item's_ref_is_null, malformed_matching_ref_is_null, no-op_release_writes_nothing, unwritable_store_keeps_exit_0 PASS. Isolated git config: exit 0, 6 PASS. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 | exit 0; unwritable store: dispatch exits 0, WARNING printed, claim kept | exit 0; `--- PASS: TestDispatchRecordFailureNeverFailsDispatch`. Isolated git config: exit 0, 1 PASS. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 | exit 0 | exit 0; `--- PASS: TestDispatchRecordsRunKey`. Isolated git config: exit 0, 1 PASS. Network denied: PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 | exit 0, every consumers routing corroborated | exit 0 as written, but vacuous on merged main: `consumers: no brief files in the diff against f68adb19c — nothing to corroborate`. Diagnostic with --base 40b45182b --brief desk-supervision/28: exit 0; `summary: 5 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing` (the 5 fixed-here entries CORROBORATED; the 2 follow-up and 3 out-of-scope entries UNCHECKED, their files absent from the diff, which matches each stated routing) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 | exit 0; jq prints true; deskclaim-ref show prints HELD with the same key | could-not-check. As written: exit 1, `tail: ...dispatch-records.jsonl: No such file or directory` (witness row 9 agrees: fail exit=1). Cause: the installed desk-tools are v1.0.39 built from 08d194b1, which is not a descendant of 8164deac5, so no installed binary writes dispatch records yet; this verifier's own dispatch (by v1.0.39) left no line. A live-claim read of this dispatch (deskclaim-ref show assay--desk-supervision--28) prints `HELD assay--desk-supervision--28`, but no recorded claim_key exists to compare it to. The row needs a dispatch by a desk-tools release that contains 8164deac5 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Mutation probes (rows 2, 4 and 6; run on a scratch copy of tools/desk and forgeread at f68adb19c, never in this home; unmutated baseline passed first): (M1) dispatch_ref nonce made optional in the suffix grammar: row 2 test FAILS at ref_with_no_nonce; (M2) tier check replaced by accept-anything: row 2 FAILS at tier_is_a_model_name and brief_exec_tier_fast; (M3) the record-write WARNING replaced by panic(err): row 6 test FAILS; (M4) nonce read into a discarded buffer, so every nonce is zero: row 4 FAILS at two_dispatches_two_refs. All four mutants killed; sources restored and diffed clean.

Risk-bearing value: frontmatter risk is present, all four answers no, irreversible: no; but the trigger fires on path, because statusgen --lint NOTICEs (risk-files-crossread) that the brief declares tools/desk/internal/deskkit/dispatchrecord.go, which matches the security-path trigger tools/desk/internal/deskkit/ for medici-finance/assay. Enumerated over the delivering diff's deskkit record code:
- RISK-VALUE: DERIVED — dispatchRefNonceBytes = 6 @ tools/desk/internal/deskkit/dispatchrecord.go:77 — 6 bytes = 48 bits = 12 lowercase hex, exactly the brief's definition; a collision needs the same claim key, the same UTC second and the same 48 random bits, and at most a few dispatches share one key-second, so the collision chance is about n^2/2^49, which is negligible. The 48 bits also make a sha256 digest of the full ref infeasible to recover by enumerating keys and seconds, which is the visibility rule's reason for the nonce.
- RISK-VALUE: DERIVED — claimKeyRe bound {1,226} @ tools/desk/internal/deskkit/dispatchrecord.go:85 — 226 = 256 − (1 "@" + 16 timestamp chars + 1 "." + 12 nonce chars), so any ref minted from an accepted key fits the 256-byte cap of dispatchRecordMaxBytes = 256 @ tools/desk/internal/deskkit/dispatchrecord.go:74 (the brief's per-field cap). The range excludes 0x40 "@" and space, which keeps the key/suffix split unambiguous.
- RISK-VALUE: DERIVED — record file mode 0o600 @ tools/desk/internal/deskkit/dispatchrecord.go:370 — the same mode the audit log uses (deskkit audit.go:341); the ref is local state under the visibility rule, so owner-only is the right mode.
- Ranked last, not derived (reversible operational knobs): scanner max line 1024*1024 @ tools/desk/internal/deskkit/dispatchrecord.go:396; dispatchRefTimeLayout = "20060102T150405Z" @ tools/desk/internal/deskkit/dispatchrecord.go:80 is the brief's literal ISO-8601 basic form, not a tunable.

**VERIFY: BLOCKED** — rows 1-8 meet Expect by hand at f68adb19c (row 8 only in its --base form; as written it is vacuous on merged main); row 9 (gate:model +dereference) is could-not-check because no installed desk-tools release contains 8164deac5 yet, so no dispatch record exists to dereference. Status stays implemented; re-run row 9 after a release containing 8164deac5 is installed and one real dispatch is made.

### Verifier re-run — 2026-10-15, assay-verifier-app[bot] (non-implementer), merged main 8efe5579d
Window: 2026-10-15T03:52:21Z to 2026-10-15T04:02:30Z (rows ran 03:53:06Z to 03:59:15Z). Delivering change: #2427, squash commit 8164deac5 (single parent 40b45182b). Host: darwin, go1.27.2, git 2.56.0; installed desk-tools and statusgen v1.0.42 built from 0b59c5c6, which contains 8164deac5 (git merge-base --is-ancestor exit 0) and is an ancestor of 8efe5579d. Admission: deskdispatch --check-verifier exit 0 before the first row and again before this write (binding below). Rows 1-7 ran as written with HOME, XDG_CONFIG_HOME, GOCACHE and TMPDIR pointed at a scratch dir outside the home, GOPROXY=off, KUBECONFIG=/dev/null. Row 9 read the dispatcher's local record store and made one read-only forge read of the claim ref; nothing was written to the forge.

Expected, derived from the brief before reading the tests: rows 1-7 exit 0 with the named PASS lines; row 8 exit 0 with every fixed-here routing corroborated; row 9 the newest dispatched record for this run is a `dispatched` line whose ref starts with its own claim key and `@`, and that key is HELD on the forge.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1, as written | exit 0 | exit 0; `ok` for deskdispatch (223.0s), deskclaim-ref (2.9s), deskkit (242.5s), deskkit/untrustcorpus (0.1s) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | Verify row 2, as written | exit 0; each listed refusal refused; valid records and every dispatcher identifier shape accepted | exit 0; grep prints `--- PASS: TestDispatchRecord_Refusals (0.00s)`; 66 PASS lines, 0 FAIL. Refusal subtests include tier_is_a_model_name, brief_exec_tier_fast, ref_of_another_claim_key, ref_without_@, ref_with_no_nonce, nonce_too_short, nonce_too_long, nonce_uppercase_hex, nonce_non-hex_char, effort_XL, field_holds_a_newline, unknown_event, brief_is_prose, brief_is_a_link, brief_is_markup, brief_has_RLO, brief_has_ZWSP, brief_has_LSEP, item_is_prose, branch_is_prose, repo_is_prose, session_is_prose, kit_outside_set; acceptance subtests include valid_dispatched_accepted, valid_released_accepted, brief_v1_id_accepted, brief_v2_id_accepted, pr_item_key_accepted, uuid_session_accepted | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | Verify row 3, as written | exit 0; one dispatched line, claim_key equals the acquired key, ref prefixed by key and @, frontmatter fields match; dry run writes zero | exit 0; grep prints the row's PASS line; 8 PASS, 0 FAIL, subtests real_dispatch_writes_one_line and dry_run_writes_nothing among them (plus tier case canonicalisation, invalid UTF-8 brief id null, free text never recorded) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | Verify row 4, as written | exit 0; worktree ref equals record ref; two same-second dispatches give two refs, same claim_key, attempt_local 1 then 2; failing entropy gives null ref, WARNING, exit 0 | exit 0; grep prints the row's PASS line; subtests two_dispatches_two_refs and failing_entropy_leaves_null PASS (3 PASS, 0 FAIL). Test body read: clock fixed to 2026-10-06T14:15:02Z for both dispatches, a counting entropy reader, refs compared unequal, attempt_local checked as i+1, mint WARNING asserted on stderr | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | Verify row 5, as written | exit 0; matching ref carried, non-matching null, no-op writes nothing, unwritable store keeps exit 0 | exit 0; grep prints `--- PASS: TestReleaseWritesReleasedRecord (0.45s)`; subtests matching_ref_is_carried, other_item's_ref_is_null, malformed_matching_ref_is_null, no-op_release_writes_nothing, unwritable_store_keeps_exit_0 PASS (6 PASS, 0 FAIL) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | Verify row 6, as written | exit 0; unwritable store: dispatch exits 0, WARNING printed, claim kept | exit 0; grep prints the row's PASS line (1 PASS, 0 FAIL). Test body read: the record path is made a directory, the record-write WARNING is asserted on stderr, and a claim release is asserted absent | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | Verify row 7, as written | exit 0 | exit 0; grep prints `--- PASS: TestDispatchRecordsRunKey (1.67s)` | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | Verify row 8, as written | exit 0; every consumers routing corroborated | exit 0 as written, but vacuous on merged main: `consumers: no brief files in the diff against 8efe5579d9253813835db650d45180eea76cf33e — nothing to corroborate`. Diagnostic with --base 40b45182b --brief desk-supervision/28: exit 0; `summary: 5 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing`; all 5 fixed-here entries CORROBORATED; the 2 follow-up and 3 out-of-scope entries UNCHECKED because their files are absent from the diff, which matches each stated routing | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | Verify row 9, as written, right after this run's own dispatch | exit 0; jq prints true; deskclaim-ref show prints HELD and the same key | Met through a diagnostic keyed to this run. As written at 03:53:06Z the last line was this run's `dispatched` record (claim_key assay--desk-supervision--28, repo medici-finance/assay, kit verifier, brief_exec_tier strong, brief_effort M, model_stamp applied) and jq printed `true`, but deskclaim-ref show exited 6 because no forge token was exported in that shell. Re-run as written at 03:53:23Z and 03:53:36Z with a read token: jq printed `false`, exit 1, because another desk session's `released` line for a different claim key had been appended in between. Diagnostic at 03:53:36Z selecting the record whose dispatch_ref equals the home's assay.dispatchRef (1 matching line; sha256 of the two values equal): jq `true`, then `dispatch-claim: HELD assay--desk-supervision--28 … state=dispatched`, exit 0. The record's claim_key is the live claim on the forge | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Row 9 check-definition note: the store is shared by every desk session on the host, and any session's dispatch or release appends to it, so `tail -n 1` names this run's record only when no other write lands between the dispatch and the read. The property the row tests holds; the command needs to select the record by the worktree's assay.dispatchRef (or by claim_key and event) rather than by file position.

Risk-bearing value: frontmatter risk is present with all four answers no, irreversible: no, but the trigger fires on path (the delivering diff touches tools/desk/internal/deskkit/, a security-path trigger for this repo). Enumerated over the delivering diff's record code at 8efe5579d: dispatchRecordMaxBytes = 256 (line 74), dispatchRefNonceBytes = 6 (line 77), dispatchRefTimeLayout = "20060102T150405Z" (line 80), claimKeyRe bound {1,226} (line 85), dispatchRefSuffixRe nonce width {12} (line 88), field regex bounds at lines 103-108, kit vocabulary at line 112, effort set S/M/L at line 290, file mode 0o600 at line 370, scanner max line 1024*1024 at line 396; the production entropy source is rand.Reader from crypto/rand @ tools/desk/cmd/deskdispatch/dispatch.go:173.
- RISK-VALUE: DERIVED — dispatchRefNonceBytes = 6 @ tools/desk/internal/deskkit/dispatchrecord.go:77 — 6 bytes = 48 bits = 12 lowercase hex characters, the brief's definition exactly; a collision needs the same claim key, the same UTC second and the same 48 random bits, so for the few dispatches that can share one key-second the chance is about n^2/2^49; the 48 bits also keep a sha256 digest of the full ref from being recovered by enumerating keys and seconds, which is why the visibility rule needs the nonce. The source is crypto/rand (dispatch.go:173), and a read failure yields a null ref, never a weaker nonce (row 4 failing_entropy_leaves_null).
- RISK-VALUE: DERIVED — claimKeyRe bound {1,226} @ tools/desk/internal/deskkit/dispatchrecord.go:85 — 226 = 256 − (1 "@" + 16 timestamp characters + 1 "." + 12 nonce characters), so every ref minted from an accepted key fits dispatchRecordMaxBytes = 256 @ tools/desk/internal/deskkit/dispatchrecord.go:74 (the brief's 256-byte per-field cap); the byte range excludes 0x40 "@" and space, so the key/suffix split is unambiguous.
- RISK-VALUE: DERIVED — record file mode 0o600 @ tools/desk/internal/deskkit/dispatchrecord.go:370 — the same owner-only mode as the audit log (audit.go:341); the ref is local state under the visibility rule, so owner-only is right. The live store on this host reads -rw------- .
- Ranked last, not derived (reversible): scanner max line 1024*1024 at line 396 and the field regex bounds at lines 103-108 (a too-tight bound drops or nulls a field, fixed by an edit and a release). dispatchRefTimeLayout at line 80 is the brief's literal ISO-8601 basic form, not a tunable.

**VERIFY: PASS** — 9/9 rows meet Expect at 8efe5579d: rows 1-7 as written; row 8 exit 0 as written and corroborated in its --base form; row 9 through the diagnostic keyed to this run's dispatch_ref, because the as-written tail -n 1 raced a concurrent append (check-definition note above).
RISK-VALUE: DERIVED — dispatchRefNonceBytes = 6 @ tools/desk/internal/deskkit/dispatchrecord.go:77 — 48 bits from crypto/rand, per the brief's definition.
Verification-Attestation: medici-finance/assay#3005 run=c01f9cf22dc5bf321e07f81f2a3047986ef90cfd0a5f8abbf5c3424f110969e1 source=8efe5579d9253813835db650d45180eea76cf33e brief=docs/streams/desk-supervision/brief-28-dispatch-record.md model=claude-opus-5-5 tier=any

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/... ./cmd/deskclaim-ref/... ./internal/deskkit/...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDispatchRecord_Refusals$' -v > "${TMPDIR:-/tmp}/b28-refusals.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecord_Refusals' "${TMPDIR:-/tmp}/b28-refusals.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordWrittenAtModelStamp$' -v > "${TMPDIR:-/tmp}/b28-write.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordWrittenAtModelStamp' "${TMPDIR:-/tmp}/b28-write.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRefRecordedInWorktree$' -v > "${TMPDIR:-/tmp}/b28-wt.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRefRecordedInWorktree' "${TMPDIR:-/tmp}/b28-wt.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run '^TestReleaseWritesReleasedRecord$' -v > "${TMPDIR:-/tmp}/b28-rel.out" 2>&1 && grep -F -e '--- PASS: TestReleaseWritesReleasedRecord' "${TMPDIR:-/tmp}/b28-rel.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordFailureNeverFailsDispatch$' -v > "${TMPDIR:-/tmp}/b28-nonfatal.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordFailureNeverFailsDispatch' "${TMPDIR:-/tmp}/b28-nonfatal.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDispatchRecordsRunKey$' -v > "${TMPDIR:-/tmp}/b28-runkey.out" 2>&1 && grep -F -e '--- PASS: TestDispatchRecordsRunKey' "${TMPDIR:-/tmp}/b28-runkey.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --consumers --root .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
| 9 | `L=$(tail -n 1 "$HOME/.config/assay/dispatch-records.jsonl") && K=$(printf '%s' "$L" \| jq -r .claim_key) && R=$(printf '%s' "$L" \| jq -r .repo) && printf '%s' "$L" \| jq -e --arg k "$K" '.event == "dispatched" and (.dispatch_ref \| startswith($k + "@"))' && deskclaim-ref show "$K" --repo "$R"` | fail exit=1 | sha256:2ed27c1421e6 | 2026-10-18 | assay-verifier-app[bot] @ 35c5ff08163b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3379 run=9ad2d1b80733ff089a7c31f28fb41ff87e38a36969124ccfe6f17c9b5bbbad72 source=35c5ff08163bd01a03cc907f6b3b5b5926e9795a brief=docs/streams/desk-supervision/brief-28-dispatch-record.md model=claude-opus-5-5 tier=strong

### Verifier re-run — 2026-10-18, assay-verifier-app[bot] (non-implementer), merged main 35c5ff081
Window: 2026-10-18T19:10:31Z to 2026-10-18T19:30Z. Delivering change: #2427, squash commit 8164deac5 (single parent 40b45182b), an ancestor of 35c5ff081. Host: darwin, go1.27.2, git 2.56.0, statusgen v1.0.49. Admission: deskdispatch --check-verifier exit 0 before the first row (binding below). The witness table above records rows 1-8 (check:ci) as could-not-run because its network-off sandbox is `unshare --net`, which darwin lacks; rows 1-7 were then run by hand, as written, under a macOS sandbox-exec profile denying outbound network except loopback (an external HTTPS probe under the same profile failed to connect), with GOFLAGS=-count=1, GOPROXY=off, KUBECONFIG=/dev/null and TMPDIR outside the home. Row 9 read the local record store and made one read-only forge read of the claim ref.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1, as written | exit 0 | exit 1, twice (19:11-19:18Z and 19:24-19:28Z). deskdispatch `ok`, deskkit `ok`, deskkit/untrustcorpus `ok`; deskclaim-ref `FAIL`: `--- FAIL: TestClaimReaderCompatibility/ordinary-global` and `/linked-global`, each `git -C: error: could not lock config file /dev/null: Operation not permitted`. Same failure with the network sandbox off. The failing test is not one of this brief's tests: the same test passes at 1afd2485c and fails from fa0403ed8 (#3286), which makes the shared git test fixture force GIT_CONFIG_GLOBAL to /dev/null so the test's own `config --global` step cannot write; CI build-test is red on main from fa0403ed8 on. A first run also failed TestWorkerShepherdPacketShowsOutsideValuesOnlyInCodeSpans in deskdispatch; that test passes with a short TMPDIR and fails with a long one at 1afd2485c too, so it depends on the length of the temp path and was not counted against this row | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2, as written | exit 0; each listed refusal refused; valid records and every dispatcher identifier shape accepted | exit 0; grep prints `--- PASS: TestDispatchRecord_Refusals (0.00s)`; 66 PASS lines, 0 FAIL. Refusal subtests include tier_is_a_model_name, brief_exec_tier_fast, ref_of_another_claim_key, ref_without_@, ref_with_no_nonce, nonce_too_short, nonce_too_long, nonce_uppercase_hex, nonce_non-hex_char, effort_XL, field_holds_a_newline, unknown_event, brief_is_prose, brief_is_a_link, brief_is_markup, brief_has_RLO, brief_has_ZWSP, brief_has_LSEP, item_is_prose, branch_is_prose, repo_is_prose, session_is_prose, kit_outside_set; acceptance subtests include valid_dispatched_accepted, valid_released_accepted, brief_v1_id_accepted, brief_v2_id_accepted, pr_item_key_accepted, uuid_session_accepted | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3, as written | exit 0; one dispatched line, claim_key equals the acquired key, ref prefixed by key and @, frontmatter fields match; dry run writes zero | exit 0; grep prints `--- PASS: TestDispatchRecordWrittenAtModelStamp (0.38s)`; 8 PASS, 0 FAIL, including real_dispatch_writes_one_line and dry_run_writes_nothing | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4, as written | exit 0; worktree ref equals record ref; two same-second dispatches give two refs, same claim_key, attempt_local 1 then 2; failing entropy gives null ref, WARNING, exit 0 | exit 0; grep prints `--- PASS: TestDispatchRefRecordedInWorktree (0.27s)`; subtests two_dispatches_two_refs and failing_entropy_leaves_null PASS (3 PASS, 0 FAIL). Test body read: one fixed instant for both dispatches, refs compared unequal, attempt_local checked as i+1, mint WARNING asserted on stderr | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5, as written | exit 0; matching ref carried, non-matching null, no-op writes nothing, unwritable store keeps exit 0 | exit 0; grep prints `--- PASS: TestReleaseWritesReleasedRecord (0.14s)`; subtests matching_ref_is_carried, other_item's_ref_is_null, malformed_matching_ref_is_null, no-op_release_writes_nothing, unwritable_store_keeps_exit_0 PASS (6 PASS, 0 FAIL) | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6, as written | exit 0; unwritable store: dispatch exits 0, WARNING printed, claim kept | exit 0; grep prints `--- PASS: TestDispatchRecordFailureNeverFailsDispatch (0.09s)` (1 PASS, 0 FAIL) | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7, as written | exit 0 | exit 0; grep prints `--- PASS: TestDispatchRecordsRunKey (0.16s)` | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8, as written | exit 0; every consumers routing corroborated | exit 0 as written, but it judged only this run's uncommitted Evidence edit: `summary: 0 corroborated, 0 disproved, 10 unchecked`, each entry `unchanged since the merge-base`. Diagnostic with --base 40b45182b --brief desk-supervision/28: exit 0; `summary: 5 corroborated, 0 disproved, 5 unchecked, 0 brief(s) claiming nothing`; all 5 fixed-here entries CORROBORATED; the 2 follow-up and 3 out-of-scope entries UNCHECKED because their files are absent from the diff, which matches each stated routing | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9, as written, right after this run's own dispatch | exit 0; jq prints true; deskclaim-ref show prints HELD and the same key | Met through a diagnostic keyed to this run. As written at 19:11:12Z: jq printed `false`, exit 1, because the last line was another desk session's `released` record for a different claim key (the witness row 9 agrees: fail exit=1). Diagnostic at 19:11:24Z selecting the record whose dispatch_ref equals the home's assay.dispatchRef: 1 matching line (event dispatched, claim_key assay--desk-supervision--28, repo medici-finance/assay, kit verifier, brief_exec_tier strong, brief_effort M, model_stamp applied; sha256 of the two refs equal), jq `true`, then `dispatch-claim: HELD assay--desk-supervision--28 … state=dispatched`, exit 0 | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Row 9 check-definition note (unchanged from the 2026-10-15 pass): the store is shared by every desk session on the host, so `tail -n 1` names this run's record only when no other write lands between the dispatch and the read; the command should select the record by the worktree's assay.dispatchRef.

Risk-bearing value: frontmatter risk is present with all four answers no, irreversible: no, but the trigger fires on path (statusgen --lint risk-files-crossread: the brief declares a file under tools/desk/internal/deskkit/, a security-path trigger). Enumerated over the delivering diff's record code at 35c5ff081 (only change to that file since the last pass: eded7ba6e, a process-runner wrapper in WorktreeDispatchRef, no literal changed): dispatchRecordMaxBytes = 256 (line 74), dispatchRefNonceBytes = 6 (line 77), dispatchRefTimeLayout = "20060102T150405Z" (line 80), claimKeyRe bound {1,226} (line 85), dispatchRefSuffixRe nonce width {12} (line 88), effort set S/M/L (line 290), file mode 0o600 (line 370), scanner max line 1024*1024 (line 396); production entropy rand.Reader from crypto/rand @ tools/desk/cmd/deskdispatch/dispatch.go:184.
- RISK-VALUE: DERIVED — dispatchRefNonceBytes = 6 @ tools/desk/internal/deskkit/dispatchrecord.go:77 — 6 bytes = 48 bits = 12 lowercase hex, the brief's definition; a collision needs the same claim key, the same UTC second and the same 48 random bits (about n^2/2^49 for the few dispatches sharing one key-second), and the 48 bits keep a sha256 digest of the full ref from being recovered by enumerating keys and seconds.
- RISK-VALUE: DERIVED — claimKeyRe bound {1,226} @ tools/desk/internal/deskkit/dispatchrecord.go:85 — 226 = 256 − (1 "@" + 16 timestamp characters + 1 "." + 12 nonce characters), so every ref minted from an accepted key fits dispatchRecordMaxBytes = 256 @ tools/desk/internal/deskkit/dispatchrecord.go:74; the byte range excludes "@" and space, so the key/suffix split is unambiguous.
- RISK-VALUE: DERIVED — record file mode 0o600 @ tools/desk/internal/deskkit/dispatchrecord.go:370 — owner-only, as the audit log; the ref is local state under the visibility rule. The live store reads -rw------- .
- Ranked last, not derived (reversible): scanner max line at line 396; dispatchRefTimeLayout at line 80 is the brief's literal ISO-8601 basic form.

**VERIFY: FAIL** — row 1: exit 1 at 35c5ff081 because TestClaimReaderCompatibility (deskclaim-ref, not one of this brief's tests) fails from fa0403ed8 (#3286) on; rows 2-7 meet Expect as written, row 8 in its --base form, row 9 through the diagnostic keyed to this run's dispatch_ref. Status stays implemented; re-run after the claim-reader fixture regression is fixed on main.
Verification-Attestation: medici-finance/assay#3379 run=9ad2d1b80733ff089a7c31f28fb41ff87e38a36969124ccfe6f17c9b5bbbad72 source=35c5ff08163bd01a03cc907f6b3b5b5926e9795a brief=docs/streams/desk-supervision/brief-28-dispatch-record.md model=claude-opus-5-5 tier=strong


## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
