---
brief: assay:assay:harness-portability:10
title: SpecMem portable-memory spike — one stream's registers across Claude Code and a second harness
why: >-
  The desks' memory and specs are welded to Claude Code's formats (CLAUDE.md, the per-session
  memory dir, the in-git registers). That lock-in is precisely what makes switching harnesses
  expensive — the ceiling this stream removes. SpecMem claims a portable, MCP-exposed memory layer
  usable across agents. If it can hold ONE stream's briefs/registers portably across Claude Code
  AND a second harness, harness-switching gets cheap; if it can't, we learn the real cost before
  betting a migration on it. This spike INFORMS — it does not gate — the HP/03 harness-target
  ruling (a proposal of the authoring session, not a dependency Ian set).
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-16 by intake-desk authoring session
sources:
  - "authoring dispatch (Ian, 2026-08-16): also evaluate SpecMem — two spikes, one per question; this is spike (b)"
  - "SpecMem: SuperagenticAI/specmem (Apache-2.0, built at Kiroween 2025) — github.com/SuperagenticAI/specmem. Claims: unified, embeddable, MCP-exposed cognitive-memory layer over Spec-Driven-Development metadata. Note: upstream is Kiro-native (it indexes `.kiro/specs/` as the agent memory) and asserts portability of that spec store ACROSS agents — whether it serves faithfully to a NON-Kiro harness is exactly the claim this spike tests, not an assumed freedom from `.kiro`/CLAUDE.md/.cursorrules"
  - "freshness-checked 2026-08-16: no docs/research/specmem-* file exists"
exec-tier: strong
exec-tier-why: >-
  (b) correctness depends on cross-harness reasoning — whether the SAME specs/registers serve
  faithfully to two different agents is exactly the portability claim under test, not a demo.
version: 1
id: 148d23f1-9a9b-4b3f-8797-fc24fb467156
---

# Brief 10 — SpecMem portable-memory spike

## Context
files:
- **create** `docs/research/specmem-portability-spike.md` (planned) — the measured findings + go/no-go
- **amend** `freshness.yaml` (planned) — register the new file (empirical facts rot)
out-of-repo files: none (SpecMem runs as an MCP server against a chosen stream's docs; no desk memory is migrated in this spike)
facts:
- the SECOND harness is whichever is available to pair with Claude Code — Codex (this stream's primary target) or jcode (HP/09). The claim under test is harness-INDEPENDENCE, so the pairing is the point, not the specific partner.
- pick a LOW-STAKES stream's briefs/registers for the trial — never a load-bearing register (the desk registers stay authoritative in-git during the spike).
- the test is faithfulness, not presence: does querying the SAME SpecMem store from two harnesses return the same specs/impact/context, or does one harness silently get a degraded view?

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- Do NOT move any load-bearing desk register into SpecMem; this is a read-mostly portability trial on a copy of one low-stakes stream.

## Task
1. Stand up SpecMem (Apache-2.0) as an MCP server over a COPY of one low-stakes stream's briefs/registers.
2. Query it from Claude Code AND from a second harness (Codex or jcode) via its MCP surface — specs lookup, impact analysis, optimized-context retrieval.
3. Fill the planned findings file: what served IDENTICALLY across both harnesses vs what still needed each harness's native format; and where SpecMem's spec model does / does not fit Assay's brief-v1 + register shapes.
4. Write the go/no-go read: does SpecMem meaningfully de-couple memory from the harness (making a Claude-Code↔other switch cheap), and at what adoption cost. Register the file in `freshness.yaml`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0 — the findings doc exists (planned deliverable) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | exit 0 — faithfulness verdicts present (separate `-e` patterns; `\|` in `-E` is a literal pipe) |
| 3 | (dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted — proving portability was exercised, not asserted | two harnesses' outputs for one query are both quoted and compared |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | exit 0 — the empirical file is registered |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer at implementation time -->
SpecMem portable-memory spike. Documentary/architectural spike (no code, no gating config — "informs, does not gate" HP/03). Verified in-tree at the source repo. SpecMem referenced as documentary upstream only (no local clone).

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | 0 | present | 2026-08-24 | opus-4.8[1m]-verifier |
| 2 | grep -qiE identical/degraded/portable/native-only in the doc | 0 | all four faithfulness terms present (§4 table + §5) | 2026-08-24 | opus-4.8[1m]-verifier |
| 3 | SAME query from BOTH harnesses, outputs compared | — | COULD-NOT-CHECK / BLOCKED by design — no live specmem-mcp server + no 2nd MCP harness offline; doc declares BLOCKED §7, gives verbatim reproduction protocol §6, does NOT fabricate output | 2026-08-24 | opus-4.8[1m]-verifier |
| 4 | `grep -q specmem-portability-spike freshness.yaml` | 0 | registered (last-reviewed 2026-08-24, max-age-days 45, upstreams []) | 2026-08-24 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — top value = `go/no-go verdict = NO-GO (watch-list)`, DERIVED in §5 from the §4 register-mapping (SpecMem ships no adapter for brief-v1/Verify/freshness/STATUS; only CLAUDE.md natively ingested; adoption would re-lock registers to Kiro's SDD triad) — internally sound, follows from documented upstream facts, not a bare assertion. `max-age-days=45`, `last-reviewed="2026-08-24"`/`upstreams:[]` — NAMED, house-consistent, reversible (empty-upstreams justified inline: SpecMem not a locally-tracked clone). No irreversible operational literal.

**VERIFY: PASS** — all 3 mechanically-runnable rows (1,2,4) pass; row 3 is a genuinely unrunnable live cross-harness comparison, honestly declared BLOCKED §7 with a positive-control reproduction protocol §6 and no fabricated output (matching the brief-01 house pattern). The go/no-go read does not hinge on the blocked row — it turns on the statically-assessable register-mapping. gate:model + all-risk-no → flipped `implemented → verified`.
### Non-implementer verifier re-run — VERIFY: FAIL (de-house re-home dropped the deliverable, already tracked) — sonnet-5-verifier (verify-desk dispatch), @ merged main `5fbf75834e1d2e5a80b44524649b4030f50e80f1`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0 | **FAIL — exit 1**, file absent on merged main | 2026-09-18 | sonnet-5-verifier |
| 2 | grep identical/degraded/portable/native-only in the doc | exit 0 | **FAIL — exit 2**, target file absent | 2026-09-18 | sonnet-5-verifier |
| 3 | dereference: same query run from both harnesses, quoted output | verdicts present | **could-not-check** — nothing to dereference, doc absent | 2026-09-18 | sonnet-5-verifier |
| 4 | `grep -q specmem-portability-spike freshness.yaml` | exit 0 | **FAIL — exit 1**, no specmem entry (case-insensitive re-check also confirms absence) | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all 4 rows map 1:1 to Verify rows; no invented scope.

**Finding — confirmed already tracked, same de-house-drop class as harness-portability/09.** The brief's own pre-existing Evidence table is real historical evidence from the house source tree, carried into the public repo by the re-home commit `527a938be` (2026-08-26) without the actual deliverable or freshness.yaml registration. The table's own text even says "Verified in-tree at the source repo." Already tracked at medici-finance/assay#393 (OPEN), which names harness-portability/10 explicitly with the identical root cause and identical row 1/2/4 failures. The README board status already correctly shows `implemented` with empty Verify/Review cells — the real tracked status was never corrupted, only the in-brief inline text is stale. No new issue filed.

RISK-VALUE: N/A — enumeration over this item's diff/deliverables on this repo found no literal; the one candidate irreversible act the brief guards against (treating SpecMem as authoritative over git) never happened — frontmatter risk.irreversible: no, and the brief states the desk registers stay authoritative in-git during the spike.

VERIFY: FAIL — held at implemented, matching what #393 already asks (route to worker-desk to land the deliverable, porting the real house-repo findings, before any re-verify). Recommend the coordinator also consider #393's Ask item 2: stripping/marking the brief's own inline Evidence table as detached/non-authoritative, since as written it misleadingly reads as a completed verified-here pass.

### Non-implementer verifier re-run — VERIFY: FAIL (deliverable still absent; tracked at #393) — opus-5.5[1m]-verifier (verify-desk dispatch), @ merged main `cf56ddeebc187e7923c4c6349bbdf95291bdfe05`, 2026-09-27

Runner is not the implementer. Own detached temp worktree off the fetched origin/main (HEAD equals the forge main SHA). Offline envelope observed (`KUBECONFIG=/dev/null`). No PR, no push, no status flip. Execution witness (statusgen verifyrun, non-dry):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | fail exit=2 | sha256:2e6d05164b27 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted — proving portability was exercised, not asserted` | fail exit=2 | sha256:2b9c659c2dc3 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |

Per-row notes (real output):
- Row 1: the findings doc is absent on merged main (exit 1, empty output). `git grep -i specmem` over freshness.yaml and docs/research returns nothing (exit 1).
- Row 2: grep exits 2, "No such file or directory" on the findings doc — the target file does not exist.
- Row 3: a dereferencing (prose) row; the witness's exit 2 is the shell refusing the prose as a command, not an observed result. Manually: could-not-check — there is no doc on this tree to dereference, so no two-harness quoted output exists to compare. A live two-harness SpecMem session is outside the offline envelope in any case.
- Row 4: freshness.yaml carries no specmem entry (exit 1).

Observation: harness-portability/14 (code de-house) did not re-home this brief's deliverable — brief 14 names no SpecMem path, and this item's deliverable is a research doc plus a freshness registration, not code. The findings doc and its freshness registration still exist only in the pre-re-home source tree. Same defect as the 2026-09-18 run, still OPEN at #393.

Risk-bearing value enumeration: this item's in-repo diff on merged main is the brief markdown only (commits 527a938be, bb2079bdc, 41530fd25); no deliverable file landed, so there is no introduced literal to enumerate. The Deliverables name a freshness registration whose values (max-age-days, last-reviewed) are absent here — nothing to quote at a file:line on this tree.

RISK-VALUE: N/A — enumeration over the item's merged-main diff (brief markdown only) and its named deliverables (research doc + freshness.yaml entry, both absent) found no literal on this tree; frontmatter risk is all-no, and the only irreversible-shaped act the brief guards against (making SpecMem authoritative over in-git registers) is excluded by its ground rules and never happened.

VERIFY: FAIL — rows 1, 2, 4 fail by exit code; row 3 could-not-check (nothing to dereference). A real defect (deliverable never ported to this repo), not stale-shaped: paths and idioms in the Verify table are correct, and they pass against the pre-re-home source tree. Held at implemented; tracked at #393.

### Non-implementer verifier re-run: 2026-10-02 (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 5a108baba705c5f9b458501065fc52a12557c969

Runner is not the implementer. Own detached temporary worktree at the merged head. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR, no push, no status flip. Every row was executed by hand as authored, then again by the execution witness.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0, the findings doc exists | PASS, exit 0. The doc is present (177 lines). | 2026-10-02 assay-verifier-app[bot] |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | exit 0, faithfulness verdicts present | PASS, exit 0. The §4 register-mapping table carries the portable and native-only verdicts; §5 and §6 use identical and degraded. | 2026-10-02 assay-verifier-app[bot] |
| 3 | (dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted | two harnesses' outputs for one query are both quoted and compared | FAIL, expectation unmet. Dereferenced by reading the whole doc: it quotes no returned output from either harness. Its header line says "the live cross-harness run is BLOCKED (see §7)", and its own §8 table marks item 3 "BLOCKED: no live SpecMem server + second harness offline (§7); protocol to discharge in §6 — no fabricated output". The live run itself needs a SpecMem MCP server and a second MCP-capable harness, both outside the offline envelope, so this verifier could not produce the missing record either. | 2026-10-02 assay-verifier-app[bot] |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | exit 0, the empirical file is registered | PASS, exit 0. Entry at freshness.yaml line 51: last-reviewed 2026-08-24, max-age-days 45, upstreams empty. | 2026-10-02 assay-verifier-app[bot] |

Execution witness (statusgen verifyrun v1.0.31, exit 1), 3 of 4 rows pass:

| # | Result | Output |
|---|--------|--------|
| 1 | pass exit=0 | sha256:e3b0c44298fc |
| 2 | pass exit=0 | sha256:e3b0c44298fc |
| 3 | fail exit=2 | sha256:2b9c659c2dc3 |
| 4 | pass exit=0 | sha256:e3b0c44298fc |

The witness's exit 2 on row 3 is the shell refusing a prose row as a command. It is not an observation about the doc; the by-hand dereference above is the observation.

Findings:

- **Verdict changed since the 2026-09-27 record (0 of 4): rows 1, 2 and 4 now pass.** Commit d73d7c996 (#1814, merged 2026-09-28) landed the findings doc and its freshness registration. That is the declared-input change: the freshness.yaml sha256 moved from 28d2645a7554 to 227d07cccc36, while the brief file is byte-identical to the last record (5421a5d0700a).
- **#393 is still OPEN, but its harness-portability/10 part is resolved** by #1814 (the doc and registration it named as missing are on main). It no longer blocks this brief; it stays open for its other items.
- **Row 3 is the only remaining failure and it is not an implementation defect in this repo.** It needs a human-run live two-harness session, or a recorded ruling that the row closes as blocked-by-design, or a re-baseline of the row by the brief author. That is tracked at #1834 (OPEN, help wanted), which names this row and lists those three end states.
- **Freshness clock.** The registration kept last-reviewed 2026-08-24 from the pre-re-home source tree although the doc was ported on 2026-09-27. With max-age-days 45 the entry goes stale on 2026-10-08, six days after this run. Not a Verify row; noted so the next freshness sweep is not a surprise.
- The 2026-08-24 Evidence block above reads row 3 as acceptable ("BLOCKED by design") and ends in a PASS. This run does not adopt that reading: the row's authored expectation is that both outputs are quoted and compared, and they are not.

Risk-bearing value enumeration (scope: the diff of d73d7c996 for this item, plus the brief's named deliverables):

- max-age-days = 45 @ freshness.yaml:53. If wrong, the doc is flagged stale too early or too late; undone by an edit.
- last-reviewed = "2026-08-24" @ freshness.yaml:52. If wrong, the staleness date shifts; undone by an edit.
- upstreams = [] @ freshness.yaml:54. If wrong, upstream changes do not invalidate the doc; undone by an edit. The inline comment gives the reason (the SpecMem upstream is not a locally tracked clone).

All three are reversible operational knobs and rank last; no entry is irreversible. Frontmatter risk is all-no. The doc's NO-GO read is a conclusion, not a literal, and the brief states it informs and does not gate.

RISK-VALUE: NAMED, NOT DERIVED — max-age-days = 45 @ freshness.yaml:53 — a reversible staleness knob matching the neighbouring entries' value; no first-principles derivation exists or is required for a reversible knob.

VERIFY: FAIL — 3 of 4 rows pass (1, 2, 4); row 3 is unmet because the doc quotes no two-harness output, and satisfying it needs a human-run live session or a recorded ruling (#1834, OPEN). Status stays implemented.

### Non-implementer verifier re-run: 2026-10-11 (UTC), assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main 63bd67f01a01d007d3e63f18977d4265294013ea

Runner is not the implementer. Own detached temporary worktree at the merged head; HEAD equals the forge main SHA read at run time. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR, no push, no status flip. Admitted by `deskdispatch --check-verifier` (exit 0, run 4b6a2faeeb26, brief sha256 41e578c66c9f). Every row was executed by hand as authored, then again by the execution witness.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0, the findings doc exists | PASS, exit 0. The doc is present, 177 lines, sha256 4cd77c28c8ac (unchanged since d73d7c996, #1814). | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | exit 0, faithfulness verdicts present | PASS, exit 0. Case-insensitive line counts per term: identical 8, degraded 5, portable 11, native-only 7. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | (dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted | two harnesses' outputs for one query are both quoted and compared | FAIL, expectation unmet. Dereferenced by reading the whole doc: no returned output from either harness is quoted. Its header says "the live cross-harness run is BLOCKED (see §7)"; its §8 table marks item 3 BLOCKED ("no live SpecMem server + second harness offline (§7); protocol to discharge in §6 — no fabricated output"). A live SpecMem MCP server plus a second MCP-capable harness is outside the offline envelope, so this verifier could not produce the record either. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | exit 0, the empirical file is registered | PASS, exit 0. Entry at freshness.yaml line 51: last-reviewed "2026-08-24", max-age-days 45, upstreams empty. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness (statusgen verifyrun v1.0.37, exit 1), 3 of 4 rows pass. No row is classed check:ci, so the Darwin network-off sandbox limit did not apply. The witness's exit 2 on row 3 is the shell refusing a prose row as a command, not an observation about the doc; the by-hand dereference above is the observation.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 63bd67f01a01 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 63bd67f01a01 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted — proving portability was exercised, not asserted` | fail exit=2 | sha256:2b9c659c2dc3 | 2026-10-11 | assay-verifier-app[bot] @ 63bd67f01a01 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 63bd67f01a01 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2604 run=4b6a2faeeb267bc4cb84d3197d7cbfc6f0cfef14dad35cd8a4c83d9f92319dd7 source=63bd67f01a01d007d3e63f18977d4265294013ea brief=docs/streams/harness-portability/brief-10-specmem-portable-memory-spike.md model=claude-opus-5-5 tier=any

Findings:

- **No change since the 2026-10-02 record.** Between 5a108baba705 and this head, the only commit touching the brief, the findings doc or freshness.yaml is 62b9c7efa (#2065), which changed the brief's Evidence text only. The doc (4cd77c28c8ac) and freshness.yaml (227d07cccc36) are byte-identical to that run's inputs.
- **Row 3 remains the sole failure and is not an implementation defect in this repo.** It needs a human-run live two-harness session, a recorded ruling that the row closes as blocked-by-design, or a re-baseline of the row by the brief author. Tracked at #1834 (OPEN, help wanted; read 2026-10-11, last updated 2026-10-11).
- **The freshness registration is now past its clock.** last-reviewed 2026-08-24 plus max-age-days 45 puts the entry stale from 2026-10-08; this run is 2026-10-11. The 2026-10-02 record forecast this. Not a Verify row; it needs a freshness review of the doc (re-check the upstream SpecMem claims and bump last-reviewed), which is a separate small item.
- #393 stays OPEN for its other items; its harness-portability/10 part remains resolved by #1814.

Risk-bearing value enumeration (scope: the diff of d73d7c996 for this item, plus the brief's named deliverables; nothing in scope changed since):

- max-age-days = 45 @ freshness.yaml:53. Wrong means the doc is flagged stale early or late; undone by an edit.
- last-reviewed = "2026-08-24" @ freshness.yaml:52. Wrong means the staleness date shifts; undone by an edit.
- upstreams = [] @ freshness.yaml:54. Wrong means upstream changes do not invalidate the doc; undone by an edit. The inline comment gives the reason (SpecMem upstream is not a locally tracked clone).

All three are reversible operational knobs and rank last; none is irreversible. Frontmatter risk is all-no. The doc's NO-GO read is a conclusion, not a literal, and the brief says it informs and does not gate.

RISK-VALUE: NAMED, NOT DERIVED — max-age-days = 45 @ freshness.yaml:53 — a reversible staleness knob; the delivering commit says it matches "the same 45-day leash as the other harness capability matrices", which is consistency, not a first-principles derivation, and none is needed for a reversible knob.

**VERIFY: FAIL** — 3 of 4 rows pass (1, 2, 4); row 3 is unmet because the doc quotes no two-harness output. Satisfying it needs a human-run live session or a recorded ruling (#1834, OPEN). Status stays implemented.

### Non-implementer verifier re-run: 2026-10-15 (UTC), assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk), merged main a1c0a9c1e8a3bbd4de6f223eabc10ed9a27a1e68

Runner is not the implementer. Own detached temporary worktree at the merged head a1c0a9c1e8a3. Offline envelope observed (`KUBECONFIG=/dev/null`); no third-party tool was downloaded, installed or executed. No PR, no push, no status flip. Admitted by `deskdispatch --check-verifier` (exit 0, run 20c90d53e848, brief sha256 8ba38ea4201c). Run window 2026-10-15 05:11Z to 05:15Z. Every row was executed by hand as authored, with no execution witness.

Grounding, written from the brief before reading any diff: merged main should hold the findings doc. The doc should carry faithfulness verdicts and quote the SAME SpecMem query issued from Claude Code and from a second harness (Codex or jcode), with both returned outputs shown and compared. It should also give a go/no-go read, and freshness.yaml should register the doc.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0, the findings doc exists | PASS, exit 0. The doc is present, 177 lines, sha256 4cd77c28c8ac. Discharges Verify row 1. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | exit 0, faithfulness verdicts present | PASS, exit 0. Case-insensitive line counts per term: identical 8, degraded 5, portable 11, native-only 7. Discharges Verify row 2. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | (dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted | two harnesses' outputs for one query are both quoted and compared | FAIL, checked by reading the whole doc; the expectation is unmet. The doc quotes no returned output from either harness. Its header line says "the live cross-harness run is BLOCKED (see §7)". Its §8 table marks item 3 BLOCKED: "no live SpecMem server + second harness offline (§7); protocol to discharge in §6 — no fabricated output". Its §6 gives only a protocol to run later. A live SpecMem MCP server plus a second MCP-capable harness is outside this offline envelope, so this verifier could not produce the record either. Discharges Verify row 3. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | exit 0, the empirical file is registered | PASS, exit 0. Entry at freshness.yaml line 51: last-reviewed "2026-08-24", max-age-days 45, upstreams empty. Discharges Verify row 4. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#3024 run=20c90d53e8487c4c1ff3cadc678334023f1f465c7fe861efe14f2e23995ccc99 source=a1c0a9c1e8a3bbd4de6f223eabc10ed9a27a1e68 brief=docs/streams/harness-portability/brief-10-specmem-portable-memory-spike.md model=claude-opus-5-5 tier=any

Findings:

- **No change in the deliverable since the 2026-10-11 record.** Between 63bd67f01a01 and this head, the only commit touching the brief, the findings doc or freshness.yaml is a0463071d (#2613). That commit landed the brief's Evidence text only. The doc (sha256 4cd77c28c8ac) and freshness.yaml (sha256 227d07cccc36) are byte-identical to that run's inputs. The delivering change is still d73d7c996 (#1814), the only commit that ever touched the doc.
- **Row 3 is the sole failure.** The deliverable does not meet the row: no two-harness output is quoted. The cause is environmental, not a coding defect, because the live run needs a SpecMem MCP server and a second MCP-capable harness. Closing it needs one of three things: a human-run live two-harness session following the doc's §6 protocol, a recorded ruling that the row closes as blocked-by-design, or a re-baseline of the row by the brief author. Tracked at #1834; its state was not read in this offline run.
- **The freshness registration is past its clock.** last-reviewed 2026-08-24 plus max-age-days 45 makes the entry stale from 2026-10-08, seven days before this run. This is not a Verify row. It needs a freshness review of the doc that re-checks the upstream SpecMem claims and bumps last-reviewed.

Risk-bearing value enumeration (scope: the diff of d73d7c996 for this item, meaning the findings doc and its freshness.yaml entry, plus the brief's named deliverables; nothing in scope changed since):

- max-age-days = 45 @ freshness.yaml:53. If wrong, the doc is flagged stale early or late; an edit undoes it.
- last-reviewed = "2026-08-24" @ freshness.yaml:52. If wrong, the staleness date shifts; an edit undoes it.
- upstreams = [] @ freshness.yaml:54. If wrong, upstream changes do not invalidate the doc; an edit undoes it. The inline comment gives the reason: the SpecMem upstream is not a locally tracked clone.

All three are reversible operational knobs and rank last; none is irreversible. The findings doc introduces no operational literal; its NO-GO read is a conclusion, and the brief says it informs and does not gate. Frontmatter risk is all-no.

RISK-VALUE: NAMED, NOT DERIVED — max-age-days = 45 @ freshness.yaml:53 — a reversible staleness knob. All 8 max-age-days entries in freshness.yaml are 45, and the comment block above the entry gives "45-day leash" as the shared policy for empirical third-party harness facts. That is consistency, not a first-principles derivation, and none is needed for a reversible knob.

**VERIFY: FAIL** — 3 of 4 rows pass (1, 2, 4). Row 3 is unmet because the doc quotes no two-harness output for a shared query. Satisfying it needs a human-run live session, a recorded ruling, or a row re-baseline (#1834). Status stays implemented.

### Non-implementer verifier re-run: 2026-10-22 (UTC), verify-desk dispatched verifier (claude-opus-5-5, strong tier), merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

Runner is not the implementer. Own detached temporary worktree at the merged head; HEAD equals the forge main SHA read at run time. Offline envelope observed (`KUBECONFIG=/dev/null`); no third-party tool was downloaded, installed or executed. No PR, no push, no status flip. Admitted by `deskdispatch --check-verifier` (exit 0, run 80898bdcd174, brief sha256 61a1a1e2ef10, tier strong) before any row ran. The execution witness below (statusgen verifyrun v1.0.54, write mode, exit 1) ran on a clean tree at the source sha; its rows and attestation line are exactly as the tool wrote them. No row is classed check:ci, so the network-off sandbox did not apply and every row ran under the witness.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted — proving portability was exercised, not asserted` | fail exit=2 | sha256:2b9c659c2dc3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4035 run=80898bdcd174e5cf271006746a833c9915438d143c90d55d046914bcbaef9150 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/harness-portability/brief-10-specmem-portable-memory-spike.md model=claude-opus-5-5 tier=strong

Row notes (what rests on the witness, and what on a hand read):

- Rows 1, 2 and 4 rest on the witness alone: pass, exit 0 each. A hand cross-check agrees: the findings doc is present (177 lines, sha256 4cd77c28c8ac); case-insensitive line counts per faithfulness term are identical 8, degraded 5, portable 11, native-only 7; freshness.yaml line 51 registers the doc with last-reviewed "2026-08-24", max-age-days 45, upstreams empty.
- Row 3 is a dereferencing (prose) row. The witness's exit 2 is the shell refusing the prose as a command, not an observation about the doc. The observation is a hand read of the whole doc: it quotes no returned output from either harness for any query. Its header says the live cross-harness run is BLOCKED (its §7); its §8 table marks item 3 BLOCKED ("no live SpecMem server + second harness offline (§7); protocol to discharge in §6 — no fabricated output"); its §6 is a protocol for a later run, not a record of one. The row tests for presence of two quoted outputs compared side by side, and their absence is observed, so the row fails. A live SpecMem MCP server and a second MCP-capable harness are outside this offline envelope, so this verifier could not produce the record either.

Findings:

- **No change in the deliverable since the 2026-10-15 record.** Between a1c0a9c1e8a3 and this head the only commit touching the brief, the findings doc or freshness.yaml is 7d681e984, which landed the brief's Evidence text only. The doc (sha256 4cd77c28c8ac) and freshness.yaml (sha256 227d07cccc36) are byte-identical to that run's inputs; the delivering change is still d73d7c996 (#1814).
- **Row 3 is the sole failure.** Closing it needs a human-run live two-harness session following the doc's §6 protocol, a recorded ruling that the row closes as blocked-by-design, or a re-baseline of the row by the brief author. Tracked at #1834 (OPEN, help wanted; read 2026-10-22).
- **The freshness registration remains past its clock.** last-reviewed 2026-08-24 plus max-age-days 45 makes the entry stale from 2026-10-08, fourteen days before this run. Not a Verify row; it needs a freshness review of the doc.

Risk-bearing value enumeration (scope: the diff of d73d7c996 for this item, meaning the findings doc and its freshness.yaml entry, plus the brief's named deliverables; nothing in scope changed since):

- max-age-days = 45 @ freshness.yaml:53. If wrong, the doc is flagged stale early or late; an edit undoes it.
- last-reviewed = "2026-08-24" @ freshness.yaml:52. If wrong, the staleness date shifts; an edit undoes it.
- upstreams = [] @ freshness.yaml:54. If wrong, upstream changes do not invalidate the doc; an edit undoes it. The inline comment gives the reason: the SpecMem upstream is not a locally tracked clone.

All three are reversible operational knobs and rank last; none is irreversible. The findings doc introduces no operational literal; its NO-GO read is a conclusion, and the brief says it informs and does not gate. Frontmatter risk is all-no.

RISK-VALUE: NAMED, NOT DERIVED — max-age-days = 45 @ freshness.yaml:53 — a reversible staleness knob shared by the neighbouring empirical harness entries; that is consistency, not a first-principles derivation, and none is needed for a reversible knob.

**VERIFY: FAIL** — 3 of 4 rows pass (1, 2, 4, all on the witness). Row 3 fails on a hand dereference: the doc quotes no two-harness output for a shared query. Satisfying it needs a human-run live session, a recorded ruling, or a row re-baseline (#1834, OPEN). Status stays implemented.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the harness-portability README table.
