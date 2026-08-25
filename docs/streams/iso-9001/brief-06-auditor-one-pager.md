---
brief: iso-9001/06
title: The auditor one-pager — what Assay is and is not
why: >-
  The first question a certified adopter's quality manager asks is "does using this make us
  compliant?", and the answer has to be a crisp no, in writing, on a page they can hand to
  someone. Today that answer exists only as sentences buried in the preamble of a format
  reference — the honest framing is there, and it is three scrolls into a document about
  tarball layout. An adopter who cannot find it writes their own summary instead, and a
  summary written by someone hoping the answer is yes becomes an unsupportable claim inside
  their management system. That is a defect this repo caused, not one it inherited.
wave: 2
depends: ["iso-9001/01", "iso-9001/03", "iso-9001/04", "iso-9001/05"]
unblocks: []
effort: S
exec-tier: strong
exec-tier-why: >-
  A claim-boundary page is the one document where the failure mode is the document itself.
  Every sentence has to be checked for what a motivated reader could quote out of it, and the
  drafting judgement is the entire deliverable.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`docs/evidence-bundle.md` preamble — the prose that already says it: the bundle evidences the RECORDED process, derived from authored artifacts; it is an INPUT TO a compliance review, not a compliance artifact in itself; it is NOT an audit opinion and does not attest ground truth. This brief extracts and sharpens; it does not invent."
  - "`docs/evidence-bundle.md` §'What the bundle does not claim' — the five-bullet list, and the Enforced/Advisory reading rule above it: 'An advisory row is evidence that something was CLAIMED, not that it was DONE.'"
  - "`docs/lifecycle.md` §'The honest claim about the board' — 'the sensors are agent-writable… Claim the weaker, true thing.' The single most important sentence for this page's register."
  - "`README.md` — 'The board is derived from agent-authored artifacts, checked by the linter and re-verified independently. It is not measured from ground truth, and Assay does not claim it is.'"
  - "`docs/iso9001-mapping.md` §'What Assay does not claim, and what the adopter must supply' — the adopter-obligation table this page summarises rather than duplicates."
  - "depends iso-9001/01, /03, /04, /05: each converts a 'not provided' into a 'provided'. A one-pager written before them states a what-you-can-cite list that this same stream falsifies three times over — and a stale claim-boundary page is the exact defect the page exists to prevent."
  - "The standard-side reading: a vendor's own certificate does not validate a tool for an adopter's intended use — it is one input to supplier evaluation and nothing more; and validation is intended-use-specific and therefore inherently the adopter's act, which nobody can perform for them. Both belong on this page in plain words."
  - "The standard-side reading, second half: what a certified adopter actually needs from a tool is a statement of intended use and limitations, immutable versioned releases whose version is reported into every record, a published defect process including notification of defects that could have produced wrong past results, a support and lifecycle policy, exportable human-readable records, and an explicit statement of non-determinism where a model is in the loop. This page states which of those exist here and which do not."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — `docs/what-assay-is-and-is-not.md` (planned) does not exist; the honest framing lives only inside `docs/evidence-bundle.md`."
---

# Brief 06 — the auditor one-pager

## Context

single-point-of-failure: this page becomes the quotable summary of every claim boundary in
the corpus, which means a single over-worded sentence in it outweighs a dozen careful ones
elsewhere. The countermeasure is that it may not introduce a claim: every statement is either
a restatement of one already on a shipped page, or a statement of absence. A sentence in this
brief's deliverable that cannot be traced to an existing page is the review finding.

files:
- `docs/what-assay-is-and-is-not.md` (planned) — the deliverable; one page, written to be
  handed over without a preamble.
- `docs/evidence-bundle.md` and `docs/iso9001-mapping.md` — cross-links only; the substance
  stays where it is.

facts:
- **Extraction, not authorship.** The sentences exist. The work is selection, ordering and
  making the page findable — and resisting the pull to improve the claims while moving them.
  A page that says more than its sources is the failure mode.
- **Lead with the no.** The first section answers "does adopting this make us compliant" with
  a plain no and a one-line reason, before anything about what the tooling does. A reader who
  stops after the first section must not be able to leave with the wrong impression.
- **State the three things this repo genuinely does not have**, because they are what an
  adopter's own auditor will reach for: there is no signing or provenance attestation anywhere
  in the release pipeline (integrity is sha256 pinning, which stops a silent substitution and
  establishes nothing about who built the artifact); there is no defect-notification path for
  a defect found in a shipped release that could have produced wrong past results; and there
  is no stated support lifetime or end-of-support policy for a released version.
- **State the non-determinism plainly.** A model is in the loop. The board is derived from
  agent-authored artifacts checked for internal consistency, not measured from ground truth,
  and identical inputs are not guaranteed to produce identical judgements. A tool whose
  verdicts vary for identical input cannot support a known-answer corpus on its own, and an
  adopter must know that before they build a procedure on it. The deterministic parts — the
  lint, the mutation gates, the exporter's byte-identical output — should be named as the
  parts that are deterministic, rather than letting the page imply the whole is.
- **Two sentences that must appear, in the adopter's interest and not this repo's.** A
  vendor's own certificate would not validate this tool for their intended use; it would be
  one input to their supplier evaluation. And validation is intended-use-specific, therefore
  inherently their act — nobody can perform it for them, and the most a tool can do is make it
  cheap.
- **Name what changed, once the stream lands.** Briefs 01, 04 and 05 each convert an absence
  into an artifact — a per-release tool-validation pack, an authorizer recorded in the release
  notes, a records and retention statement. The page lists what an adopter can cite, and the
  list is only true after those land, which is why this brief is last.
- **No new check, and no claim of enforcement.** This page is prose and is honoured by review.
  It carries no mutation row because it adds no control, and it must not describe itself as
  enforcing anything.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Write `docs/what-assay-is-and-is-not.md` (planned)**, opening with the plain no: adopting Assay
   does not confer, contribute to, or substitute for certification of any management system,
   and nothing it produces is an audit opinion.
2. **State what it is** in one short section: a delivery methodology and a toolchain that
   produce a recorded, exportable process — an input to a review, not a review.
3. **List what an adopter can cite**, each with what it establishes and what it does not, and
   each pointing at the page that carries the detail rather than restating it.
4. **List what is not provided**: signing and provenance attestation, a delivered-defect
   notification path, a stated support lifetime, and validation for the adopter's intended
   use.
5. **State the determinism position** — which parts are deterministic, that a model is in the
   loop, and that the board is derived from authored artifacts rather than measured.
6. **Add the two adopter-interest sentences** about a vendor certificate and about validation
   being the adopter's own act.
7. **Cross-link both ways** with `docs/evidence-bundle.md` and `docs/iso9001-mapping.md`, and
   link it from the repository README's contents table so it is reachable without knowing it
   exists.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/what-assay-is-and-is-not.md` | exit 0 — **DEREFERENCE, inverts**: the page does not exist at authoring (2026-08-25 @ `6871a3b`) |
| 2 | `git grep -nF 'not an audit opinion' -- docs/evidence-bundle.md` | exit 0 — **DEREFERENCE**: the source sentence this page extracts still stands where it was, so the extraction did not silently relocate the only copy |
| 3 | `git grep -cE -e 'certif' -e 'audit opinion' -- docs/what-assay-is-and-is-not.md` | exit 0; a non-zero count — the plain no is on the page, in those words |
| 4 | `git grep -cE -e 'attestation' -e 'sha256' -- docs/what-assay-is-and-is-not.md` | exit 0; a non-zero count — the no-signing position is stated rather than left as an absence a reader must notice |
| 5 | `git grep -nF 'not measured from ground truth' -- docs/what-assay-is-and-is-not.md` | exit 0 — the determinism and derivation position is stated in the corpus's own existing words |
| 6 | `git grep -n 'what-assay-is-and-is-not' -- README.md docs/evidence-bundle.md docs/iso9001-mapping.md` | exit 0, at least two hits — the page is reachable from the surfaces a reader actually starts on |
| 7 | `test -f docs/records-and-retention.md` | exit 0 — **DEREFERENCE**: iso-9001/05 has landed, so the can-cite list is not naming an artifact that does not exist. If this row is red, stop: the dependency did not land |
| 8 | `git grep -n 'tool-validation' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: iso-9001/01 has landed, so the tool-validation pack the page tells an adopter to ask for is really emitted |
| 9 | `git grep -n 'authorized-by' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: iso-9001/04 has landed, so the authorizer record the page cites really reaches the release |
| 10 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree lints clean, including the link check over every path this page cites; a can-cite entry naming a file that does not exist fails here |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). **The whole deliverable is
judgement and this table cannot reach it** (`docs/brief-rules.md` rule 8): rows 3–5 prove
particular tokens are present, rows 7–9 prove the artifacts the page cites exist, and nothing
mechanical can tell a careful claim boundary from a careless one. That is the review, and it
is the only control here. Reviewer records verdict + date in the stream README table.
Reviewer questions specific to this brief: (1) read the page as a hostile reader looking for a
sentence to quote in a sales deck — is there one? (2) can every statement be traced to an
already-shipped page, or does the extraction introduce a claim? (3) does a reader who stops
after the first section leave with the right impression? (4) are the four not-provided items
stated as absences rather than as roadmap? (5) does the determinism section separate the
deterministic parts from the model-in-the-loop parts, rather than implying either covers the
whole?
