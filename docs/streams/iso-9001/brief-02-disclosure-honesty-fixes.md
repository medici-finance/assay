---
brief: iso-9001/02
title: Align three shipped disclosures with the code they describe
why: >-
  Three surfaces this repo ships publicly now describe a weaker system than the one it has.
  The evidence bundle says no command is run and no exit code or output hash is recorded for a
  Verify row, when verifyrun records the command, the exit code, a sha256 of the output, the
  date, the runner and the tree SHA, and witnessgate refuses a verified brief that contradicts
  its own witness. The same page calls segregation of duties a string check that does not
  corroborate against commit authorship, when corroborate cross-checks a human stamp against
  the pull request's actual reviews and exits 1 on disproof. And the README and the FINDINGS
  register both claim contiguous numbering is enforced, which the registers page explicitly
  forbids claiming. Two of the three understate the system and one overstates it; the
  overstatement is the dangerous one, and the understatements are expensive in a different
  way — the corpus a reader uses to judge the methodology describes something weaker than what
  ships.
wave: 0
depends: []
unblocks: ["iso-9001/05"]
effort: S
exec-tier: strong
exec-tier-why: >-
  Every edit here is a claim about what the code enforces, and the failure mode is asymmetric:
  correcting a pessimistic disclosure into an optimistic overstatement is worse than leaving
  it stale. Each new sentence must be read back against the source before it is written.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`docs/mistake-proofing.md` §4 B9: 'Any statement in authoring guidance about what is and is not enforced MUST be generated from the enforcement source.' B9 is the standing rule that would have caught all three of these and it is unimplemented; this brief is the manual correction, not the mechanism."
  - "`docs/evidence-bundle.md` §'SOC2 change-management mapping' — the Testing row ('Nothing witnesses execution: no command is run, no exit code or output hash is recorded') and the Segregation-of-duties row ('Enforced as a string check, advisory as a control… it does not corroborate them against commit authorship or a signed identity')."
  - "`statusgen/verifyrun.go` — the execution witness: command, exit code, sha256 of combined output, date, runner identity and tree SHA, with runner attribution DERIVED and never supplied (no `--runner` flag; passing one is a dedicated usage error), and three dispositions on three exit codes."
  - "`statusgen/witnessgate.go` — a brief whose Evidence records a FAILED witness may not read verified or done; a lower severity for witness absence."
  - "`statusgen/corroborate.go` — `--corroborate --pr` cross-checks a human stamp against the pull request's actual reviews and comments, exit 1 on disproof."
  - "`docs/registers.md` §'Shared conventions': 'Slug IDs — contiguity is NOT enforced… Slugs deliberately carry no contiguity guarantee — there is no sequence to have a gap in… Do not claim gap detection.' `README.md` line 14 and `docs/streams/FINDINGS.md` line 3 both claim the opposite."
  - "`docs/registers.md` header — the page ships publicly under a SUPERSEDED banner with the retired dialect still shown below the shared-conventions bullets. In scope here only to the extent of the contiguity claim; the full rewrite is not this brief."
  - "The standard-side reading: an unstated or misstated limitation becomes the adopter's own nonconformity, and a records-control statement built on a false enforcement claim is the most expensive place for one — which is what the typed unblocks: edge to iso-9001/05 encodes."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — all three stale statements were read on that commit and are present verbatim."
---

# Brief 02 — align the disclosures with the code

## Context

single-point-of-failure: this brief is a hand correction of hand-written claims, which is
exactly the shape B9 says must not be the mechanism. It is deliberately a one-time repair
rather than the derivation, and it must say so where it lands, so the next stale claim is
caught by a rule rather than by another reader noticing. Do not attempt the derivation here —
generating enforcement-status text from the lint's rule registry is a separate, larger piece
of work with its own design decisions, and folding it into a three-file text correction would
make both un-reviewable.

files:
- `docs/evidence-bundle.md` — the Testing row and the Segregation-of-duties row of the SOC2
  table, plus the matching bullets in the "What the bundle does not claim" list.
- `README.md` — the Registers bullet's contiguous-numbering claim.
- `docs/streams/FINDINGS.md` — the header sentence making the same claim.

facts:
- **Correct toward what the code does, never toward what it could do.** `verifyrun` is a
  witness, not an attestation, and its own header says so: the environment is not a
  cryptographic anchor, and the witness is evidence for a reviewer reading a diff. The new
  Testing row must carry that limit in the same breath as the correction. Two residuals stay
  true and must survive the edit: witness **absence** is a lower severity than contradiction,
  so an unwitnessed `verified` can still land; and because `verifyrun` executes
  author-written markdown as shell it is not invoked from `--lint`.
- **The segregation correction is a narrowing, not a promotion.** `corroborate` disproves a
  false **human** stamp against the forge. It does not corroborate a **model** runner token,
  which remains self-written text. The corrected row says what is now machine-disprovable and
  says plainly what is not. Re-establish: read `statusgen/corroborate.go` and
  `statusgen/attribution.go`.
- **The contiguity claim is the one overstatement and it appears twice.** `docs/registers.md`
  is the normative side and already says the right thing; `README.md` and
  `docs/streams/FINDINGS.md` contradict it. Both must change, in this brief, or the
  correction is half-done and the next reader finds whichever copy was missed. What replaces
  it is the property that *is* enforced: a register entry that has ever existed on main and
  is absent from the working tree is a lint failure, plus duplicate-id detection — deletion is
  caught by a tombstone check against history, not by a gap.
- **`docs/registers.md`'s SUPERSEDED banner is out of scope beyond the contiguity line.** The
  page's legacy format sections are a real obsolete-document problem and a separate piece of
  work. Touching them here would turn a bounded correction into a rewrite.
- **Nothing in this brief changes behaviour.** No lint rule, no check, no tool. That is why it
  carries no mutation row: it adds no control. Its Verify table is therefore mostly
  dereferencing rows against the corrected text and the source it now describes, and that
  boundary is stated in Review below.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Rewrite the evidence bundle's Testing row.** State what `verifyrun` records (command,
   exit code, sha256 of combined output, date, runner identity, tree SHA), that runner
   attribution is derived and never supplied, and that `witnessgate` refuses a `verified` or
   `done` brief contradicted by its own witness — then, in the same row, the two residuals:
   witness absence is a lower severity than contradiction, and the witness is evidence for a
   reviewer, not an unforgeable attestation. Update the Enforcement cell to match the row.
2. **Rewrite the Segregation-of-duties row** to distinguish the two cases: a `human:<name>`
   stamp is cross-checkable against the pull request's actual reviews with `--corroborate
   --pr`, which exits 1 on disproof; a model runner token is still self-written text. Keep the
   honest half of the old sentence rather than deleting it.
3. **Update the matching bullets in "What the bundle does not claim"** so the list and the
   table agree. A corrected table above a stale bullet list is the same defect one paragraph
   lower.
4. **Replace the contiguous-numbering claim in `README.md` and in
   `docs/streams/FINDINGS.md`** with the property that is actually enforced — tombstone
   detection against history and duplicate-id detection — and say, once, that slug IDs carry
   no contiguity guarantee because there is no sequence to have a gap in.
5. **Leave a pointer, not a promise.** Where the corrections land, note that a statement about
   what is and is not enforced should be derived from the enforcement source rather than
   hand-maintained (B9), and that the derivation is not implemented. Do not claim it as
   scheduled work in a document that ships.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -n 'enforces contiguous numbering' -- README.md` | exit 1, no output — **DEREFERENCE, inverts**: the claim is present at authoring (2026-08-25 @ `6871a3b`, `README.md` line 14) and must be gone |
| 2 | `git grep -n 'contiguity is enforced by statusgen' -- docs/streams/FINDINGS.md` | exit 1, no output — **DEREFERENCE, inverts**: the second copy of the same claim, present at authoring |
| 3 | `git grep -nF 'no command is run' -- docs/evidence-bundle.md` | exit 1, no output — **DEREFERENCE, inverts**: the stale Testing row is present at authoring |
| 4 | `git grep -nF 'Do not claim gap detection' -- docs/registers.md` | exit 0 — **DEREFERENCE**: the normative statement the two corrections are made *toward* still stands, unedited. If this row goes red, the correction was made in the wrong direction |
| 5 | `git grep -nF 'sha256' -- docs/evidence-bundle.md` | exit 0 — the corrected Testing row names the output hash the witness actually records |
| 6 | `git grep -nF 'corroborate' -- docs/evidence-bundle.md` | exit 0 — the corrected segregation row names the mechanism that disproves a false human stamp |
| 7 | `git grep -n 'tombstone' -- README.md docs/streams/FINDINGS.md` | exit 0 — both corrected surfaces name the property that IS enforced, not merely the absence of the one that is not |
| 8 | `git grep -nF 'not an unforgeable attestation' -- docs/evidence-bundle.md` | exit 0 — the residual survived the correction; the row did not become an overstatement in the other direction |
| 9 | `git grep -n 'derived' -- statusgen/verifyrun.go` | exit 0 — **DEREFERENCE**: runner attribution really is derived in the source the corrected row now describes, so the new sentence is not a dangling claim |
| 10 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree lints clean after the edits, including the link check over every backticked path the corrected rows cite |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). **This table gates presence and
absence of specific sentences, not their adequacy** (`docs/brief-rules.md` rule 8): rows 5–8
prove the corrected text contains the right tokens and cannot prove it says a true thing well.
Whether each new sentence is a faithful description of the code is the reviewer's job, and it
is the whole job here. Reviewer questions specific to this brief: (1) read
`statusgen/verifyrun.go`, `witnessgate.go` and `corroborate.go` and confirm each new sentence
against the source rather than against this brief; (2) did any correction move a disclosure
from too-weak to too-strong — in particular, is the model-runner residual still stated?
(3) do the table and the "does not claim" list now agree? (4) is `docs/registers.md`
untouched except where the contiguity claim required it?
