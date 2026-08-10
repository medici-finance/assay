---
brief: assay-product/04
title: Domain wiring for Plumb + Assay; formal TM search deferred
wave: 0
depends: []
unblocks: ["assay-product/05"]
effort: S
gate: human
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
decision-issue: 724
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["methodology/13 (the Assay decision + its explicit note: formal USPTO/EUIPO clearance is a pre-publication step, NOT done)", "../reconciler/docs/naming.md (the sweep-and-decide precedent, incl. the banked 2026-07-08 sweep)", "freshness-checked 2026-07-10 @ 78200803"]
gate-why: >-
  regulatory: a trademark clearance opinion has legal consequence — a wrong "clear" verdict
  followed by public launch invites a dispute; the go/no-go on the mark and any filing or
  spend is human:<name>'s. irreversible: domain purchases/DNS cutover and any USPTO/EUIPO filing are
  outward, paid, and public acts. The agent prepares the search record and the wiring plan;
  human:<name> executes every purchase/filing/publication step.
---

# Brief 04 — Domain wiring for Plumb + Assay; formal TM search deferred

## Context
files: ../assay-toolkit/docs/naming-clearance.md (new — the search record);
../reconciler/docs/naming.md (read-only precedent)
facts:
- methodology/13 decided the name (**Assay**, "Assay by Medici" variant open, home domain
  **assay.guide**) and explicitly deferred formal clearance: informal sweeps only; "Assay"
  is a common English word — clearance risk concentrates in software/SaaS classes (Nice 9,
  42) and in existing marks like assay-adjacent lab/biotech software.
- Deliverable is a SEARCH RECORD + recommendation, not a legal opinion: USPTO TESS + EUIPO
  eSearch sweeps for "assay" in classes 9/42 (word + stylized), plus common-law sweep
  (products named Assay in dev-tools/PM/AI space), each hit assessed
  likely-confusable / distinguishable / dead.
- Domain state to verify and wire: is assay.guide actually registered/owned (methodology/13
  says decided, not necessarily purchased)? Enumerate: registrar, DNS host, where the
  brief-05 site will be served (pages host TBD in 05) — prepare the DNS plan; human:<name> executes.
- Sibling-name pattern: Plumb.finance is owned (reconciler) — "held true" family branding
  note for the deck (06).

## Amendment (2026-07-14 — human:<name>'s rescope: formal search deferred, domain wiring proceeds)

**USPTO/EUIPO formal register clearance is DEFERRED by human:<name> 2026-07-14.** It stays a counsel-gated,
non-urgent legal step with no dependency on the domain work. The **domain-wiring half proceeds now** and
is extended to the newly-registered **Plumb** product domains (`plumb.finance`, `plumb.guide`) plus the
`hello@plumb.finance` email plan.

- **Clearance rows moved to a follow-up stub.** The exact deferred sweep spec (USPTO TESS + EUIPO
  eSearch for `ASSAY`, word + stylized, Nice 9/42; refreshed common-law sweep; per-hit
  likely-confusable/distinguishable/dead + overall clear/clear-with-constraints/rename verdict; gate:
  human) is recorded verbatim in the deliverable at `../assay-toolkit/docs/naming-clearance.md` **§9.6**,
  so the follow-up is dispatchable later. §§1–6 of that doc remain the 2026-07-10 *indicative* record.
- **Domain-wiring rows remain** and are what this brief now verifies. The Verify table below is adjusted
  accordingly: the clearance-presence rows (USPTO/TESS, EUIPO, recommendation) are replaced by
  domain-wiring rows (Plumb domain state, `hello@plumb.finance` email plan, deferred-search stub +
  human-gate stop-points). The `assay.guide` row and the statusgen-lint row are unchanged.
- **Prior Evidence** (§ below) pertains to the *pre-rescope* Verify table (the 2026-07-10 indicative
  record) and is retained as history; a non-implementer verifier re-runs the amended table on merged main.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. NO purchases, filings, DNS changes, or public claims — prepare and
  STOP; report BLOCKED-ON-IAN at the documented stop-point.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Run the TESS/EUIPO/common-law sweeps per facts; write
   `../assay-toolkit/docs/naming-clearance.md` with the hit table, per-hit assessment,
   overall recommendation (clear-to-use / clear-with-constraints / rename), and the open
   question list for counsel if any hit is close.
2. Verify domain ownership state; write the DNS/hosting wiring plan (records needed, host
   options deferred to brief 05's decision) into the same doc.
3. Update the stream-README row; report BLOCKED-ON-IAN for the human steps (filing/spend
   decisions, DNS execution).

## Verify (executable)
Amended 2026-07-14 (rescope): clearance-presence rows replaced by domain-wiring rows; the deferred
formal-search spec lives in the deliverable's §9.6 (row 3 checks its presence). **Deliverable ref:**
`../assay-toolkit/docs/naming-clearance.md` on branch `docs/naming-clearance-domain-wiring`
(PR medici-finance/assay-toolkit#40 @ `8c71007`).

Rows 1–3 are anchored to §9-specific strings that exist nowhere in the pre-rescope document:
on assay-toolkit `main` (232 lines, 0 §9 content) each returns 0 — the rows go red when the
work is absent.

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/naming-clearance.md && grep -c "plumb\.guide" docs/naming-clearance.md` | ≥1 (Plumb domain wiring — plumb.guide registration + redirect plan in §9.4) |
| 2 | `grep -c "hello@plumb\.finance" docs/naming-clearance.md` | ≥1 (hello@plumb.finance email plan in §9.3) |
| 3 | `grep -c "9\.6 Deferred formal search" docs/naming-clearance.md` | ≥1 (deferred-search stub + human-gate stop-points) |
| 4 | `grep -c "assay\.guide" docs/naming-clearance.md` | ≥1 (domain state + wiring plan recorded in §7) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     human:<name>'s go/no-go on the mark and the domain execution record land here too. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`): deliverable in assay-toolkit `317ca69` (local/unpushed — push is human:<name>'s).

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f docs/naming-clearance.md && grep -ciE "USPTO\|TESS"` | 0 | 10 (≥1) | 2026-07-10 | opus-verifier |
| 2 | `grep -ciE "EUIPO"` | 0 | 7 (≥1) | 2026-07-10 | opus-verifier |
| 3 | `grep -ciE "recommendation"` | 0 | 3 (≥1) | 2026-07-10 | opus-verifier |
| 4 | `grep -ciE "assay.guide"` | 0 | 11 (≥1) — domain state + DNS wiring plan recorded | 2026-07-10 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — naming-clearance search record (USPTO/TESS + EUIPO + common-law) + recommendation + assay.guide DNS wiring plan all present (assay-toolkit `317ca69`). Trademark filing / domain purchase / go-no-go remain human-gated, correctly BLOCKED-ON-IAN.

*Held at implemented: irreversible gate (risk.irreversible: yes) — requires human:<name> sign-off in Reviewed before the verified flip. Evidence recorded; flip deferred to human:<name>.*

### Re-verify 2026-07-13 (verify desk, non-implementer `opus-verifier`, merged main `a7f48277`)

Re-run because the prior run's deliverable was local/unpushed. It is now **on the assay-toolkit default
branch**: commit `5cf8d37` (232 lines), repo @ `faf363b`. All five rows re-executed, same results
(exits 0; counts 10 / 7 / 3 / 11; statusgen lint exit 0). The "local/unpushed `317ca69`" gap is closed.

**Two substance flags human:<name> should see BEFORE signing — the greps prove presence, not clearance:**

1. **The recommendation label understates the document's own finding.** §5 reads `clear-with-constraints`,
   but §4 assesses **two** hits as *"Likely-confusable"* — its own strongest category — same word, same
   aisle, same channel: `tryassay` (npm) is an *"AI code verification CLI"*, and `assay-cli` (crates.io)
   is a spec-driven agentic dev kit shipped as a **Claude Code plugin** — our exact product shape and
   distribution channel. Both were independently re-confirmed live by this verifier. The three offered
   constraints address registrability and differentiation; they do not neutralise a likelihood-of-confusion
   risk *in use* against two live in-channel products. On its own evidence the defensible label is nearer
   *"use-risk material — counsel before public use"*. The body is honest; the label is what gets skimmed.
2. **§3 (EUIPO) should be read as unsearched, not as a negative result.** Its three hits contain no
   "assay" mark and no class 9/42 software. Zero assay-containing EU marks is not evidence none exist —
   it suggests the register was not properly queried. The doc states plainly that live TESS/eSearch were
   NOT run (mirror indexes only), so the register-level sweep remains a counsel step.

Domain state independently confirmed as **fact**, not intention: RDAP shows `assay.guide` registered
`2026-07-09`, Cloudflare, no A/CNAME, nothing served — matching the doc field-for-field.

**Change-failure note (process, not code):** §4 states the common-law discovery "partially revises
methodology/13's characterisation and is flagged for the desk as FINDINGS-worthy" — **no such FINDINGS
entry was ever filed.** The premise it revises is still live and load-bearing (`../reconciler/docs/naming.md`
: *"nearest software uses are small and non-methodology"*; methodology/13 records *"Assay was the only
CLEAR verdict"* on that basis). Filed as F-assay-common-law by the desk.

### Re-verify 2026-07-17 (verify desk, non-implementer `glm-5.2-verifier`, merged main `42a741e2`)

Cross-repo verify: rows 1–4 run against an isolated assay-toolkit clone at `origin/main` `703b262`
(path substituted for `../assay-toolkit/...` per the verify-desk cross-repo rule); row 5 in this worktree.
All five executable rows PASS; deliverable present and rescoped as amended.

**PR provenance (all merged):** oit **#308** (original, merge `d89fe3ed`, 2026-07-11) →
**#508** (human:<name>'s 2026-07-14 rescope amendment, merge `0c51d17d`, 2026-07-15); assay-toolkit **#40** (sibling
deliverable, merge `e6250751`, 2026-07-16). assay-toolkit **issue #67 OPEN** tracks the deferred §9.6
formal USPTO/EUIPO search.

| # | Command (path substituted for `…/naming-clearance.md`) | Exit | Result | Expect |
|---|---------|------|--------|--------|
| 1 | `test -f … && grep -c "plumb\.guide" …` | 0 | **11** | ≥1 ✅ |
| 2 | `grep -c "hello@plumb\.finance" …` | 0 | **5** | ≥1 ✅ |
| 3 | `grep -c "9\.6 Deferred formal search" …` | 0 | **1** | ≥1 ✅ |
| 4 | `grep -c "assay\.guide" …` | 0 | **21** | ≥1 ✅ |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 0 ✅ |

**VERIFY: PASS on all five executable rows.** Domain-wiring stop-points confirmed intact — no purchase,
DNS record, filing, or public use of the mark performed (`assay.guide`/`plumb.finance`/`plumb.guide`
serve HTTP 000; DNS records BLOCKED-ON-IAN + gated on brief-05 host selection).

**Three substance flags for the human sign-off — advisory, not Verify-blockers — but flag #1 is now
materially STRONGER than the 2026-07-13 re-verify and contains a factual error in the deliverable:**

1. **§4 common-law field is MORE crowded than the doc records, and C3 is factually wrong.** Live
   registry/HTTP re-check (naming-clearance.md §4, lines 78–90):
   - **C1 `tryassay` (L88): LIVE** — npm `0.35.1` (2026-03-26), tryassay.ai HTTP 200. "Likely-confusable —
     strongest concern" correct and current.
   - **C2 `wollax/assay` (L89): exists but DORMANT** — 0 stars, null description, last push 2026-04-21.
     "Likely-confusable — near-identical category/channel" overstates this one repo's vitality.
   - **C3 `assay-core`/`assay-cli` crates.io (L90): DO NOT EXIST** — both return HTTP 200 with all-null
     fields (crate-not-found). The described "Rust policy-as-code engine / MCP firewall" is not real under
     those names; **C3 is factually wrong.** (Corrects the 2026-07-13 note: the Claude Code plugin is the
     GitHub repo wollax/assay (C2), not a crates.io crate.)
   - **Active `assay-*` Rust workspace §4 misses ENTIRELY:** `assay-evidence`, `assay-policy`,
     `assay-metrics`, `assay-runner-{schema,core,linux,spike}`, `assay-adapter-api`, `dep-assay` — workspace
     **v3.32.0 published 2026-07-04** (6 days before the doc's 2026-07-10 check), a coordinated
     compliance/evidence/policy framework in the exact verification-and-audit aisle.
   - **npm products §4 omits:** `@assay-ai/{core,cli,vitest,jest,ai-sdk}` 1.3.1-beta ("Assay LLM
     evaluation framework" org); `@aerofortress/assay` 0.2.0 (AVP — Acceptance Verification Protocol —
     reference impl; methodologically overlapping); npm `assay-cli` 1.0.0; `lockfile-assay` 1.2.0 (2026-07-15).
   **Net:** field is more crowded, not less. The §4 C-list is both partly wrong (crates.io C3 null) and
   materially incomplete (misses the Rust workspace + `@assay-ai/*` + `@aerofortress/assay`). §5
   `clear-with-constraints` **understates** in-channel confusion risk; "use-risk material — counsel before
   public use" is more defensible now than in July. Reinforces
   F-assay-common-law.
   **Filed [assay-toolkit#89](https://github.com/medici-finance/assay-toolkit/issues/89)** (the §4 common-law
   correction; distinct from #67's formal-register search).

2. **§3 (EUIPO) still unsearched, not a negative result** — flag stands unchanged. Its three hits contain
   no "assay" string and no class-9/42 software: mirror-index noise, not evidence of absence. Doc framing
   says this plainly; §3's header sentence could be misread as a finding by a skim-reader.

3. **§9.6 deferred-search stop-points adequate as written** — no purchase/DNS/filing/public-use observed;
   the label is the soft spot (its interaction with the rescope's "domain-wiring half proceeds now" framing
   could let a future brief cite `clear-with-constraints` as a green light to publish once the brief-05 host
   is picked). Written stop-points hold; the label needs the caveat (≈ flag #1).

**Verifier recommendation:** executable Verify PASSES, but the deliverable carries a factual error (§4 C3)
+ a material omission (the active `assay-*` Rust workspace + npm orgs). **Recommend correcting §4 in
assay-toolkit (#89) before the irreversible `verified` flip** — the clearance label should reflect the
fuller, corrected field. Brief stays `implemented`; the flip is human:<name>'s via checkpoint-PR (irreversible gate).

### Non-implementer re-verify — VERIFY: PASS (executable); held on changed circumstances — glm-5.2-verifier, assay-toolkit `99e8ac2f`, 2026-07-20

All 5 executable rows PASS. human:<name> signed off (#724, Option A, 2026-07-18). **But circumstances changed since the sign-off — flagging before any irreversible flip.**

| # | Command | Result |
|---|---|---|
| 1 | `grep -c "plumb\.guide" …/naming-clearance.md` | 11 PASS |
| 2 | `grep -c "hello@plumb\.finance" …` | 5 PASS |
| 3 | `grep -c "9\.6 Deferred formal search" …` | 1 PASS |
| 4 | `grep -c "assay\.guide" …` | 21 PASS |
| 5 | `go run ./tools/statusgen --root . --lint` | exit 0 PASS |

**Prior §4-defect hold (2026-07-17) largely WRONG.** The claim "C3 `assay-core`/`assay-cli` don't exist (HTTP 200 + nulls)" was a response-shape misread — non-existent crates return HTTP **404**, not 200+nulls. Both crates ARE real (v3.34.0, 2026-07-19, `github.com/Rul1an/assay`). C3 is accurate. (assay-toolkit #89's premise that C3 is "factually wrong" is itself wrong.)

**Advisory content defects (Review-gate, not Verify fails):**
1. §4 under-represents the active **24-crate `assay-*` Rust workspace** (v3.34.0, `github.com/Rul1an/assay`) — cites only `assay-core`/`assay-cli`. assay-toolkit #89 still OPEN.
2. **§9/§8 stop-points STALE** — all three domains now serve HTTP 200 on Cloudflare Pages: `assay.guide` ("Assay — work held true"), `plumb.finance` ("Plumb by Medici"), `plumb.guide` (301 → `plumb.finance`). The doc still lists these as future human:<name> steps.
3. §5 `clear-with-constraints` understates given the now-live site + fuller common-law field.

**⚠ Held despite the sign-off:** the clearance bar ROSE since the 2026-07-18 sign-off — `assay.guide` is now PUBLICLY SERVING content (the brief-05 site), which §5 says moves the framing toward "use-risk material — counsel before public use." The sign-off predates the public site. `risk.irreversible: yes` + `gate: human` → the irreversible flip is human:<name>'s to re-confirm given the now-public `assay.guide` (or to tell verify-desk to flip anyway).

## Review
Gate: human (gate-why above — human:<name> signs the clearance recommendation and executes
domain/filing steps). `/security-review` not required (no auth/DAML/funds surface).
