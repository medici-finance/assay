---
brief: harness-portability/07
title: Adoption docs, freshness registration, live Codex smoke protocol + first run
why: >-
  Everything upstream is structure; this brief is the claim. "Assay runs natively on
  Codex" is only true when a real Codex session installs the bundle, receives the
  resident rules without pasting, invokes the skills, and degrades exactly as ruled —
  and CI cannot run that session. The honest closure is a scripted smoke protocol
  executed on the live harness with evidence pasted back, plus the docs and freshness
  leashes that keep the second harness true after this stream ends.
wave: 4
depends: ["harness-portability/05", "harness-portability/06"]
unblocks: []
effort: M
gate: human
gate-why: >-
  The acceptance evidence is a live session on the second harness — an act outside CI,
  on an environment (OpenAI account, Codex install) only Ian provides or sanctions.
  The human is confirming the smoke run happened on a real Codex, with its transcript
  evidence pasted, and that the degradations observed match the ruled matrix — the one
  claim no in-repo command can corroborate.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07): say honestly how a Codex-targeted skill gets tested, and mark rows blocked rather than writing ones that pass vacuously", "the harness-target ruling (HP/03): the degradation matrix the smoke run is judged against", "harness-portability/05 + /06 deliverables (the artifacts under test)", "freshness.yaml + tools/freshness (the existing staleness instrument the binding files get registered in)", "docs/adopting-assay.md (the adoption runbook gaining the Codex path)", "freshness-checked 2026-08-07 (adopting-assay.md's harness mention is a single generic line; no Codex path exists)"]
consumers: ["docs/adopting-assay.md: fixed-here (Codex adoption path)", "plugins/assay/PARITY.md + RELEASE-NOTES.md: fixed-here (bundle version bump recording the second harness)", "freshness.yaml: fixed-here (references/codex.md + references/claude-code.md + the capability matrix registered)", "the publication review: out-of-scope (whether/when the Codex-ready bundle reaches the public copy is the publication manifest's call)"]
---

# Brief 07 — Adoption docs, freshness leashes, live smoke

> **Note (re-home):** the codex smoke protocol doc, the bundle's PARITY.md, and its
> RELEASE-NOTES.md are method-text-layer/bundle artifacts that arrive with the sequenced
> tool/method-text de-house (see the stream README's re-home note). `docs/adopting-assay.md`,
> `freshness.yaml`, `tools/freshness`, and the two `plugins/assay/references/*.md` bindings are
> already public. The design record is re-homed here.

## Context

files:
- **amend** `docs/adopting-assay.md` — the Codex adoption path (install, fragment,
  config flag, degradation expectations)
- **create** `docs/codex-smoke-protocol.md` (planned) — the scripted live-harness checklist
- **amend** `freshness.yaml` — register `plugins/assay/references/codex.md`,
  `plugins/assay/references/claude-code.md` (binding files rot against vendor
  behaviour) — 45-day leash, same rationale as 01's matrix registration
- **amend** the bundle's PARITY / RELEASE-NOTES — version bump + second-harness record

facts:
- **CI cannot run Codex.** The verification split is: structural truth in CI
  (04/05/06's checks), behavioural truth in a scripted, evidence-producing live run.
  The protocol makes the live run repeatable and its evidence comparable across
  re-runs (each Codex release, each bundle version bump).
- Protocol shape: numbered steps, each with a literal action and an `Expect:` line —
  minimum coverage: (1) fresh install per the adopt path; (2) session start → resident
  rules present WITHOUT manual pasting (probe: ask the session to state rule 3's
  neutral-dispatch wording); (3) invoke each of the seven skills by name → body loads;
  (4) auto-trigger probe per 01's `auto-trigger` verdict; (5) dispatch probe →
  `runs`/`degrades`/`refuses` observed per the ruled matrix, degradation STATED by the
  session, not silent; (6) isolation probe → refusal fires where ruled; (7) evidence
  discipline probe → a Verify row executed and recorded. Each step's evidence is a
  transcript excerpt pasted into the protocol's run log.
- The first executed run log is committed under
  `docs/codex-smoke-runs/<date>-<codex-version>.md` — the run log is the artifact the
  human gate signs.
- A failed step routes to a GitHub issue against the owning brief's surface; the smoke
  protocol never hot-fixes.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The live run happens on an environment Ian provides/sanctions — never self-procured.
- A protocol step with no `Expect:` line is not a step; a run log entry with no
  transcript excerpt is not evidence.

## Task

1. Write `docs/codex-smoke-protocol.md` (planned) (shape + minimum coverage per facts).
2. Write the Codex path in `docs/adopting-assay.md` (from 06's adopt scenario, expanded
   to runbook form, including the `multi_agent` config step and the degradation
   expectations table copied — generated or referenced, not re-typed — from
   the codex binding file).
3. Register the two binding files in `freshness.yaml` (45-day leash).
4. Bump the bundle version; record the second harness in the bundle's PARITY + RELEASE-NOTES.
5. Coordinate the first live run (Ian or a sanctioned session with the environment);
   commit the run log; file issues for any failed step.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/codex-smoke-protocol.md; echo $?` | `0` |
| 2 | Every step has an Expect: `s=$(grep -cE '^#### Step [0-9]+' docs/codex-smoke-protocol.md); e=$(grep -cE '^Expect:' docs/codex-smoke-protocol.md); echo "$s $e"; test "$s" -ge 7 -a "$s" -eq "$e"; echo $?` | `0` — at least the seven minimum steps, and step-count equals Expect-count (a step without an Expect breaks the equality) |
| 3 | `rm -f /tmp/hp07r3.out; grep -qF 'references/codex.md' freshness.yaml \|\| { echo "NOT REGISTERED codex.md"; exit 1; }; grep -qF 'references/claude-code.md' freshness.yaml \|\| { echo "NOT REGISTERED claude-code.md"; exit 1; }; go run ./tools/freshness > /tmp/hp07r3.out 2>&1; grep -cE -e '^FRESH +plugins/assay/references/codex\.md' -e '^FRESH +plugins/assay/references/claude-code\.md' /tmp/hp07r3.out` | `2` — both binding files registered and EACH reports its own `FRESH` line (the tool's overall exit code is not load-bearing here: an unrelated stale artifact elsewhere in the repo reddens the whole run regardless of these two files, so this row checks each file's own line, matching the shape used on brief-01 row 4 / brief-02 row 1). **Guarded against a stale-file false pass**: the prior `grep -qF ... && grep -qF ... && go run ...` form silently skipped the `go run` when either registration grep failed and fell through to grepping whatever `/tmp/hp07r3.out` already held from an earlier invocation. This form deletes the file first and fails fast naming the missing registration when either grep misses, so there is nothing stale left to read (control: same planted-stale-file setup now exits `1` with the `NOT REGISTERED` message instead of a false `2`) |
| 3a | **Mutation — the leash can fail**: `go run ./tools/freshness --as-of 2027-06-01 > /tmp/hp07r3a.out 2>&1; grep -cE -e '^STALE +plugins/assay/references/codex\.md' -e '^STALE +plugins/assay/references/claude-code\.md' /tmp/hp07r3a.out` | `2` — force-aged past the 45-day leash, BOTH binding files' own lines specifically flip to `STALE`, proving row 3's `FRESH` match is a real check and not a no-op (`freshness` prints one line per registered artifact whether FRESH or STALE, so a bare `grep -c 'references/'` path-token count is invariant under force-aging and never discriminates — matching the shape used on brief-01 row 4a; the exit code is NOT load-bearing, may already be non-zero unforced from an unrelated repo-wide stale artifact) |
| 4 | `grep -qiF 'codex' docs/adopting-assay.md && grep -qF 'multi_agent' docs/adopting-assay.md; echo $?` | `0` — the adoption path exists and carries the config step (two independent greps ANDed) |
| 4a | **Positive control for row 4** — `grep -qF 'multi_agent_no_such_flag' docs/adopting-assay.md; echo $?` | `1` |
| 5 | Version bumped past what `origin/main` already carried when this brief started: `base=$(git show origin/main:plugins/assay/.claude-plugin/plugin.json \| jq -r .version); cur=$(jq -r .version plugins/assay/.claude-plugin/plugin.json); test "$cur" != "$base"; echo $?` | `0` — moved off the branch-point value on `origin/main`, NOT the literal pre-stream `0.1.0`: brief-02 (wave 0) bumps the version first, so pinning to `0.1.0` would already read `!=` off 02's bump alone and never discriminate 07's own |
| 6 | The CURRENT version's own RELEASE-NOTES section names the second harness: `V=$(jq -r .version plugins/assay/.claude-plugin/plugin.json); awk -v v="$V" '/^## v/{p = ($0 ~ ("v" v))} p' plugins/assay/RELEASE-NOTES.md \| grep -qiF 'codex'; echo $?` | `0` — scoped to the section headed by the CURRENT version, not a whole-file grep for the version string: brief-02's own bump-and-record entry already puts that version string somewhere in the file without naming Codex, so an unscoped grep would pass on 02's entry alone |
| 7 | **BLOCKED (needs live Codex + Ian)** — the first run log exists and is complete: `ls docs/codex-smoke-runs/ && f=$(ls docs/codex-smoke-runs/ \| head -1) && grep -cE -e '^Result: PASS' -e '^Result: FAIL' -e '^Result: BLOCKED' "docs/codex-smoke-runs/$f"` | count equals the protocol's step count (separate `-e` patterns), every step carries a Result, and FAIL steps cite a filed issue number. Until the environment exists this row is BLOCKED — it is the stream's acceptance row and is never greened from the protocol text alone |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Row 7 is the stream's acceptance evidence: the run log path, the Codex version,
     and who ran it. BLOCKED until the live environment exists.
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review

Gate: **human** (from frontmatter). The human signs the run log: a real Codex, the
listed version, transcript excerpts present per step, degradations observed matching
the ruled matrix, failures routed to issues. Rows 1–6 green with row 7 BLOCKED means
the stream is NOT done — that state is "ready for the live run", and saying otherwise
is the vacuous-green failure this stream's bar forbids.
