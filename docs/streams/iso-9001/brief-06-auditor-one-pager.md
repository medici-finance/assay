---
brief: assay:assay:iso-9001:06
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
schema: brief-v2
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
  - "2026-10-26 pickup @ 8663d6a: docs/iso9001-mapping.md §8.4 / 8.6 and .github/workflows/release.yml now describe SLSA/Sigstore provenance for published assets; pre-change releases remain sha256-only. Correct the obsolete absence below without strengthening the claim or changing any Verify command."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — `docs/what-assay-is-and-is-not.md` (planned) does not exist; the honest framing lives only inside `docs/evidence-bundle.md`."
version: 3
id: 92e336c2-c5a9-408f-bc82-3c32b1472b18
---

# Brief 06 — the auditor one-pager

## Context

single-point-of-failure: this page becomes the quotable summary of every claim boundary in
the corpus, which means a single over-worded sentence in it outweighs a dozen careful ones
elsewhere. The countermeasure is that it may not introduce a claim: every statement is either
a restatement of one already on a shipped page, or a statement of absence. A sentence in this
brief's deliverable that cannot be traced to an existing page is the review finding.

files:
- `docs/what-assay-is-and-is-not.md` (planned) — the deliverable; one page, written to be handed over without a preamble.
- `docs/evidence-bundle.md` and `docs/iso9001-mapping.md` — cross-links only; the substance stays where it is.

facts:
- **Extraction, not authorship.** The sentences exist. The work is selection, ordering and
  making the page findable — and resisting the pull to improve the claims while moving them.
  A page that says more than its sources is the failure mode.
- **Lead with the no.** The first section answers "does adopting this make us compliant" with
  a plain no and a one-line reason, before anything about what the tooling does. A reader who
  stops after the first section must not be able to leave with the wrong impression.
- **State the remaining absences and the shipped provenance boundary.** There is no
  defect-notification path for a defect found in a shipped release that could have produced
  wrong past results, and no stated support lifetime or end-of-support policy for a released
  version. The release workflow now attests published assets with SLSA build provenance and
  publishes Sigstore bundles; pre-change releases remain sha256-only. A pinned checksum
  detects substitution but does not identify the builder. Provenance identifies the build
  workflow; it does not review source, establish intended-use validation, or replace human
  release authorization. Extract this boundary from `docs/iso9001-mapping.md`, not the
  obsolete 2026-08-25 absence.
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
4. **List what is not provided**: a delivered-defect notification path, a stated support
   lifetime, and validation for the adopter's intended use. State the shipped provenance
   boundary alongside what can be cited: SLSA/Sigstore identifies the build workflow, does
   not review source or establish intended-use validation, and does not cover pre-change
   releases that remain sha256-only.
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
| 1 | `test -f docs/what-assay-is-and-is-not.md && git grep -nF '# What Assay is and is not' -- docs/what-assay-is-and-is-not.md` | exit 0 — **DEREFERENCE, inverts**: the page does not exist at authoring (2026-08-25 @ `6871a3b`) |
| 2 | `git grep -nF 'audit opinion' -- docs/what-assay-is-and-is-not.md && git grep -nF 'not an audit opinion' -- docs/evidence-bundle.md` | exit 0 — **DEREFERENCE**: the source sentence this page extracts still stands where it was, so the extraction did not silently relocate the only copy |
| 3 | `git grep -cE -e 'certif' -e 'audit opinion' -- docs/what-assay-is-and-is-not.md` | exit 0; a non-zero count — the plain no is on the page, in those words |
| 4 | `git grep -cE -e 'attestation' -e 'sha256' -- docs/what-assay-is-and-is-not.md` | exit 0; a non-zero count — the provenance and checksum boundary is stated explicitly rather than left for a reader to infer |
| 5 | `git grep -nF 'not measured from ground truth' -- docs/what-assay-is-and-is-not.md` | exit 0 — the determinism and derivation position is stated in the corpus's own existing words |
| 6 | `set -- $(git grep -l 'what-assay-is-and-is-not' -- README.md docs/evidence-bundle.md docs/iso9001-mapping.md) && printf '%s\n' "$@" && test $# -ge 2` | exit 0, at least two hits — the page is reachable from the surfaces a reader actually starts on |
| 7 | `git grep -nF '](records-and-retention.md)' -- docs/what-assay-is-and-is-not.md && test -f docs/records-and-retention.md` | exit 0 — **DEREFERENCE**: iso-9001/05 has landed, so the can-cite list is not naming an artifact that does not exist. If this row is red, stop: the dependency did not land |
| 8 | `git grep -nF 'Tool-validation pack' -- docs/what-assay-is-and-is-not.md && git grep -n 'tool-validation' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: iso-9001/01 has landed, so the tool-validation pack the page tells an adopter to ask for is really emitted |
| 9 | `git grep -nF 'authorized-by' -- docs/what-assay-is-and-is-not.md && git grep -n 'authorized-by' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: iso-9001/04 has landed, so the authorizer record the page cites really reaches the release |
| 10 | `test -f docs/what-assay-is-and-is-not.md && cd statusgen && go run . --root .. --lint --acceptance-forge` | exit 0 — the tree lints clean, including the link check over every path this page cites; a can-cite entry naming a file that does not exist fails here |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `git grep -nF 'not an audit opinion' -- docs/evidence-bundle.md` | pass exit=0 | sha256:ae9d996ea068 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `git grep -cE -e 'certif' -e 'audit opinion' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:f3fefcd6e9a7 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git grep -cE -e 'attestation' -e 'sha256' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:4549819f5a07 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `git grep -nF 'not measured from ground truth' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:632a40d9f231 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `git grep -n 'what-assay-is-and-is-not' -- README.md docs/evidence-bundle.md docs/iso9001-mapping.md` | pass exit=0 | sha256:726a5c66e08a | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `test -f docs/records-and-retention.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git grep -n 'tool-validation' -- .github/workflows/release.yml` | pass exit=0 | sha256:da50aaea04ca | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 9 | `git grep -n 'authorized-by' -- .github/workflows/release.yml` | pass exit=0 | sha256:2689115220cb | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd statusgen && go run . --root .. --lint` | fail exit=1 | sha256:1ac92fcb19cc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4835 run=dc28a8b38555417758895542dae5207990225598c44b3a70c9e91d71f778a5e0 source=9e61aee3f8e946198214101938c0420ca37d7f6a brief=docs/streams/iso-9001/brief-06-auditor-one-pager.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — 9/10 rows pass at source 9e61aee3f8e9; row 10 is could-not-check, not an observed failure of this brief's obligation.

Row 10 disposition: the admitted command exited 1 with `LINT: FAIL 1 problem(s)`, and that one problem is the lint's own could-not-check on an unrelated brief, quoted from the row's output: `PROBLEM: desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`. The row's command carries no forge-read flag, so on any host it cannot reach exit 0 at this source while that protected-table transition stands; the repository's own CI lint step passes that flag. No link-check or other problem names this brief, its page, or any path the page cites. Needed: a reviewed definition repair of row 10 (or a lint change that reports a forge-only could-not-check without failing an offline run), then a fresh dispatch and admission.

RISK-VALUE: N/A — enumeration over the delivering change (README contents row, changelog fragment, two cross-link lines, and the new handover page; prose only) found no literal constant, bound, threshold, timeout, limit or authority binding; frontmatter risk is present and all-no, irreversible: no, and the irreversible act here is none (a documentation page, reversible by an edit).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `git grep -nF 'not an audit opinion' -- docs/evidence-bundle.md` | pass exit=0 | sha256:ae9d996ea068 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `git grep -cE -e 'certif' -e 'audit opinion' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:f3fefcd6e9a7 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git grep -cE -e 'attestation' -e 'sha256' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:4549819f5a07 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `git grep -nF 'not measured from ground truth' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:632a40d9f231 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `git grep -n 'what-assay-is-and-is-not' -- README.md docs/evidence-bundle.md docs/iso9001-mapping.md` | pass exit=0 | sha256:726a5c66e08a | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `test -f docs/records-and-retention.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git grep -n 'tool-validation' -- .github/workflows/release.yml` | pass exit=0 | sha256:da50aaea04ca | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `git grep -n 'authorized-by' -- .github/workflows/release.yml` | pass exit=0 | sha256:2689115220cb | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd statusgen && go run . --root .. --lint` | fail exit=1 | sha256:062137ce9674 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5070 run=f7c92db3ae7576e90f74aea59de5dfe051757f3f8172081b118a5bd989cc9c31 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/iso-9001/brief-06-auditor-one-pager.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — 9/10 rows pass at source 011f14965ef2; row 10 is could-not-check, not an observed failure of this brief's obligation.

Row 10 disposition (could-not-run for acceptance): the admitted command exited 1 with `LINT: FAIL 1 problem(s)`. That one problem belongs to another brief and is the lint's own could-not-check, quoted from the row's output: `PROBLEM: desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`. No PROBLEM line names this brief, its page, or any path the page cites. The same blocker recurs from the 2026-10-29 run at source 9e61aee3f8e9. It is tracked at medici-finance/assay#4823, and medici-finance/assay#4904 records the same row class. Needed: resolve that transition, or decide that offline lint reports a forge-only could-not-check without failing, or review a definition repair of row 10; then a fresh dispatch and admission. Next actor: the owner of medici-finance/assay#4823.

Supplemental, no acceptance credit, 2026-10-30, assay-verifier-app[bot] @ 011f14965ef2: from the statusgen directory, the same lint with the forge read permitted (`go run . --root .. --lint --acceptance-forge`) exited 0 with `LINT: PASS`. A separate resolution of all eleven relative links on the page found every target file and all three anchors present. Neither discharges row 10.

RISK-VALUE: N/A — enumeration over the delivering change medici-finance/assay#4534 (merge c2946d88fac8: README contents row, changelog fragment, two cross-link lines, brief edits and the new handover page; prose only) found no literal constant, bound, threshold, timeout, limit or authority binding; frontmatter risk is present and all-no, irreversible: no, and the irreversible act here is none (a documentation page, reversible by an edit).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/what-assay-is-and-is-not.md && git grep -nF '# What Assay is and is not' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:cc7a9a17ab01 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 2 | `git grep -nF 'audit opinion' -- docs/what-assay-is-and-is-not.md && git grep -nF 'not an audit opinion' -- docs/evidence-bundle.md` | pass exit=0 | sha256:cad6eb39b07c | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 3 | `git grep -cE -e 'certif' -e 'audit opinion' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:f3fefcd6e9a7 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git grep -cE -e 'attestation' -e 'sha256' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:4549819f5a07 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 5 | `git grep -nF 'not measured from ground truth' -- docs/what-assay-is-and-is-not.md` | pass exit=0 | sha256:632a40d9f231 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 6 | `set -- $(git grep -l 'what-assay-is-and-is-not' -- README.md docs/evidence-bundle.md docs/iso9001-mapping.md) && printf '%s\n' "$@" && test $# -ge 2` | pass exit=0 | sha256:e72102d5357b | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 7 | `git grep -nF '](records-and-retention.md)' -- docs/what-assay-is-and-is-not.md && test -f docs/records-and-retention.md` | pass exit=0 | sha256:27064301cc40 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git grep -nF 'Tool-validation pack' -- docs/what-assay-is-and-is-not.md && git grep -n 'tool-validation' -- .github/workflows/release.yml` | pass exit=0 | sha256:6d5db3ed5dac | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 9 | `git grep -nF 'authorized-by' -- docs/what-assay-is-and-is-not.md && git grep -n 'authorized-by' -- .github/workflows/release.yml` | pass exit=0 | sha256:fb0cf20e41cc | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
| 10 | `test -f docs/what-assay-is-and-is-not.md && cd statusgen && go run . --root .. --lint --acceptance-forge` | fail exit=1 | sha256:fb9350b41ab6 | 2026-11-06 | assay-verifier-app[bot] @ 4cf9795597f6 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5653 run=5c29c5a0cff6118ad62e53fb71d9909c3c2bb22aa1be18cb1759c52f939d60a5 source=4cf9795597f6b88517eb503758b9c4de8a5fb9be brief=docs/streams/iso-9001/brief-06-auditor-one-pager.md model=claude-opus-5-5 tier=strong



Run setup, 2026-11-06, assay-verifier-app[bot] @ 4cf9795597f6: fresh non-implementer dispatch (claude-opus-5-5, tier strong) on a darwin-arm64 host, detached home at merged main 4cf9795597f6, installed statusgen v1.0.75 through the cell shim (rows run with the cell HOME), no forge variable set by the verifier, KUBECONFIG=/dev/null. `deskdispatch --check-verifier` exited 0 before any row. The table carries no check:ci row, so no container bracket applies. Since the 2026-10-30 run, row 10 gained `--acceptance-forge` through the definition repair medici-finance/assay#5217 (merge 584fc03c86aa); the other nine rows were repaired in the same change to read the page, and all nine pass.

Row 10 disposition: the admitted command exited 1, against an Expect of exit 0. verifyrun records only the output digest, and this dispatch does not hand-run a forge-reading row, so the PROBLEM lines behind that exit were not observed in this run. No observation in this run ties that exit to this brief. `statusgen verifyrun --check` exited 1 with 9 pass, 1 fail, 0 could-not-run of 10.

Supplemental, no acceptance credit, 2026-11-06, assay-verifier-app[bot] @ 4cf9795597f6: from the statusgen directory, the same lint without the forge flag, run with HOME set to the cell home that verifyrun's rows use, exited 1 with `LINT: FAIL 2 problem(s)`. Both problems belong to other briefs' protected-table acceptance transitions: desk-tools/37 (approval medici-finance/assay#5157) and desktools-v2/65 (approval medici-finance/assay#4230). Each says the human-approval read needs `--acceptance-forge`. No PROBLEM line names iso-9001/06, its page, or any path the page cites, so the link check over the page's targets is clean in that run. The forge flag changes only that transition path. Row 10's exit 1 therefore most likely comes from the forge read of one or both of those approvals. Whether that read could not reach the forge or found an approval that does not match its payload was not observed. A second lint under the operator's own HOME stops earlier on the same two briefs. There, a user-level includeIf rule keyed to the operator's home pulls in a file with a url rewrite, and the origin-include observer refuses it as possibly affecting repository authority. That rule does not match under the cell HOME the rows use, so it did not shape the witness.

Needed: a forge-capable runner captures row 10's PROBLEM lines at this source. If they name only those two transitions, resolve them (medici-finance/assay#4823 tracks desktools-v2/65) or review a definition repair that scopes row 10 to this brief's own findings, then dispatch and admit afresh. If any line names this brief or its page, this item fails on row 10. Next actor: the verify desk, then the owner of medici-finance/assay#4823.

Fail-first: no row is risk-bearing. The risk metadata is present and every answer is no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag. `statusgen verifyrun --fail-first --base 27b0feb856e3 --dry-run`, where 27b0feb856e3 is the parent of the delivering merge c2946d88fac8, reported "no risk-bearing row in this table; every row is recorded not-selected at base" and wrote nothing.

RISK-VALUE: N/A — enumeration over the delivering changes medici-finance/assay#4534 (merge c2946d88fac8: README contents row, changelog fragment, two cross-link lines, brief edits and the new handover page) and medici-finance/assay#5217 (merge 584fc03c86aa: Verify command cells and a changelog fragment) found no literal constant, bound, threshold, timeout, limit or authority binding; the page's only digits are an anchor slug and the algorithm name sha256. Frontmatter risk is present and all-no, irreversible: no, and the irreversible act here is none (a documentation page, reversible by an edit).

**VERIFY: BLOCKED** — 9/10 rows pass at source 4cf9795597f6; row 10 exited 1, and the cause was not observed in this run (forge-read lint output not visible).

## Review
Gate: model (from frontmatter — all four risk answers no). **The whole deliverable is
judgement and this table cannot reach it** (`docs/brief-rules.md` rule 8): rows 3–5 prove
particular tokens are present, rows 7–9 prove the artifacts the page cites exist, and nothing
mechanical can tell a careful claim boundary from a careless one. That is the review, and it
is the only control here. Reviewer records verdict + date in the stream README table.
Reviewer questions specific to this brief: (1) read the page as a hostile reader looking for a
sentence to quote in a sales deck — is there one? (2) can every statement be traced to an
already-shipped page, or does the extraction introduce a claim? (3) does a reader who stops
after the first section leave with the right impression? (4) are the remaining not-provided items
stated as absences rather than as roadmap, and is provenance limited to the shipped boundary? (5) does the determinism section separate the
deterministic parts from the model-in-the-loop parts, rather than implying either covers the
whole?
