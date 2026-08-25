---
brief: mistake-proofing/04
title: Derive the authoring guidance's enforcement-status claims from the lint itself
why: >-
  The document that shapes every brief in the fleet tells authors, in its own words, which of its
  rules are actually enforced and which are only conventions. Those claims are hand-typed and they
  drift: at least one of them is wrong today, telling authors that a rule is decorative when a
  shipped check makes one class of its violations a hard failure. That is worse than an unenforced
  rule — it manufactures deliberate non-conformance, because a conscientious author reads "nothing
  enforces this" and writes the non-conforming entry on purpose. This exact error class has already
  been closed twice with the same move: declare the source, generate the copy, diff it in CI. Doing
  it here retires the whole class for the document with the widest blast radius.
wave: 1
depends: ["mistake-proofing/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
exec-tier: strong
exec-tier-why: >-
  Questions (a) and (b). The facts do not pre-specify what the generator's SOURCE is: the enforcement
  status must come from the lint's own rule registry, and today that registry is reconstructed by
  scraping bracket tags out of a lint run rather than declared anywhere. Choosing between making the
  registry explicit and deriving it from a run is the central design call, and it must hold across
  three artifacts — the lint, the generated block, and the CI diff that binds them.
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the mistake-proofing board)
sources:
  - "`docs/mistake-proofing.md` §4 B9: 'Any statement in authoring guidance about what is and is not enforced (\"no lint checks this yet\") MUST be generated from the enforcement source, or carry a check that fails when it drifts. A guidance document that tells authors a live gate is decorative manufactures deliberate non-conformance; hand-maintained second copies of normative sources are the documented error class here, and derivation is the closed fix.'"
  - "Same spec §3 D6 — honesty about non-coverage is itself a device, which is why the generated block carries three status values and not two."
  - "The device inventory behind this stream (2026-08-25) — 'the authoring guidance is itself a stale second copy, and that is the highest-value fix', cost S–M. Names the live contradiction: the guidance states twice that a named optional frontmatter field is enforced by no lint, instructs authors to leave two cases deliberately non-conforming on that belief, and the check for that field ships and is wired on pull_request, with one class of claim fatal."
  - "The same error class, previously closed: a shared guidance block maintained as a hand-typed second copy of a normative source drifted from it; the fix was a declared source, a generator that writes the copies, and a CI byte-diff that fails when a copy is hand-edited. That is the mechanism this brief copies."
  - "The second in-tree precedent: the desk tree's compiled topology registry, where a declared YAML source is compiled to Go and a test that IS the diff keeps the two from disagreeing."
  - "freshness-checked 2026-08-25 @ 657cab1 (origin/main) — the stale claim is live in `plugins/assay/skills/author-brief/SKILL.md`, the check that contradicts it ships at `statusgen/consumers.go`, and no generator or byte-diff exists between them."
---

# Brief 04 — Derived enforcement status for authoring guidance

## Context

files:
- `statusgen/` (implementation home) — a new emitter that prints the lint's rules and each one's
  enforcement status in a stable machine-readable form, plus whatever change to the lint's own rule
  bookkeeping that emitter needs, plus tests.
- `plugins/assay/skills/author-brief/SKILL.md` — the guidance document. The enforcement-status
  claims inside it become a generated block; the rest of the document stays hand-authored.
- `.github/workflows/ci.yml` — the byte-diff gate. Regenerate and compare; a difference fails the
  job.
- `tools/skillslint/` — the existing generator-and-byte-diff tool for the shared guardrail blocks.
  **Read for reference; extend rather than duplicate if it fits.**

facts:
- The precedent mechanism is already in the tree and already closed this error class once: a
  declared single source, a generator that writes the copies, and a byte-diff that fails when a copy
  is edited by hand. It delimits its blocks with a heading of the form `## guardrail: <id>` and
  deliberately anchors on the first line of the block rather than on an injected marker comment.
  Reuse that shape; do not invent a third delimiter convention.
- The lint already carries a **stable rule-tag bracket token** on emitted problem and notice lines,
  and a firing-audit mode already extracts it. Untagged lines fall into an explicit unattributed
  bucket. That tag is the natural key for the generated table — but the audit **reconstructs its
  rule set by running the lint and scraping output**, not from a declared registry. Deciding whether
  to make the registry explicit or to keep deriving it from a run is this brief's central call.
- The generated block must carry, per rule: the rule's identity, what it checks in one line, and its
  enforcement status as one of exactly three values — **fatal**, **advisory**, or **not enforced**.
  Three values, not two: "not enforced" is a real state and hiding it produces the honesty failure
  the spec's own non-coverage rule (D6) exists to prevent. A generated list of what the devices do
  NOT check is itself a device.
- **The document currently contains at least one live contradiction**, and fixing it is part of this
  brief: the guidance states that a particular optional frontmatter field is an authoring convention
  that no lint enforces, and further instructs authors to leave certain entries deliberately
  non-conforming because "nothing enforces this yet, so a non-conforming entry costs nothing". A
  shipped check now makes one class of claims in that field a hard failure. Regenerating the block
  corrects the status; the surrounding hand-authored guidance that tells authors to write
  non-conforming entries must be corrected in the same change, or the document will still be telling
  authors to do the thing the gate now rejects.
- **Scope discipline.** This brief generates the ENFORCEMENT-STATUS claims only. It does not
  generate the guidance's methodology prose, its worked examples, or its rationale — those are
  judgment and belong to their authors (spec §3 D7). A generator that swallows the whole document is
  how a teaching text becomes a reference table nobody reads.
- The tool that does the byte-diff is present in the tree but **is not referenced by any workflow**,
  so wiring the gate is part of this brief and not an assumed given.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Settle the source of truth and record the decision in the emitter's source header.** Either
   make the lint's rule set an explicit registry that the checks themselves register into, or keep
   deriving it from a lint run and state what that costs (an untagged rule is invisible; the
   generated table is then a floor, not a total). Whichever is chosen, an untagged or unregistered
   rule must be **visible as such** in the output, never silently absent.
2. **Add the emitter.** A mode that prints every rule with its identity, its one-line description,
   and its enforcement status from the three-value set, in a stable order, in a form a generator can
   render. Stable order matters: an unstable order turns every unrelated change into a diff.
3. **Generate the block into the guidance document**, delimited in the style the existing generator
   already uses, containing only the enforcement-status claims. Leave the surrounding prose
   hand-authored.
4. **Wire the byte-diff gate in CI.** Regenerate, compare, fail on difference, with a failure message
   that tells the author to regenerate rather than to hand-edit. Verify the gate is reached by the
   workflow that actually runs on pull requests — a gate in a file no workflow calls is not a gate.
5. **Correct the live contradiction in the same change.** Regenerating fixes the status line. Also
   correct the surrounding hand-authored guidance that instructs authors to write deliberately
   non-conforming entries on the strength of the now-false claim. Describe the correction in the
   pull-request body by what it is — a guidance statement that a rule is unenforced, contradicted by
   a shipped check — and cite the spec rule.
6. **Positive control.** A test that hand-edits the generated block and asserts the gate fails; and
   its inverse, a freshly generated block asserting the gate passes. Additionally, a test that
   flips one rule's enforcement status in the source and asserts the generated block changes — which
   is what proves the block is derived rather than merely present.
7. **State the coverage boundary in the generated block's own header**: that it reports what the lint
   enforces, not what the methodology requires, and that a rule enforced by a tool outside the lint
   (a CI workflow, a desk-side guard) will read as not-enforced here unless it is registered.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -n 'No lint enforces this yet' plugins/assay/skills/author-brief/SKILL.md` | exit 0; one hit — **DEREFERENCE, true at authoring (2026-08-25 @ `657cab1`)**: the stale claim is live in the guidance today. Must be gone (or generated-and-correct) at implementation |
| 2 | `test -f statusgen/consumers.go` | exit 0 — **DEREFERENCE**: the shipped check that contradicts the claim in row 1 really exists, so the contradiction is real and not asserted |
| 3 | `git grep -n 'guardrail: ' -- tools/skillslint/guardrail.go` | exit 0 — **DEREFERENCE**: the block-delimiter convention this brief reuses ships today |
| 4 | `git grep -c 'skillslint' -- .github/workflows/` | exit 1, no output — **DEREFERENCE**: no workflow calls the byte-diff tool today, so wiring the gate is genuinely part of this brief. Inverts at implementation |
| 5 | `git grep -n 'ruleTagFor' -- statusgen/lintaudit.go` | exit 0 — **DEREFERENCE**: the stable rule-tag convention the generated table keys on really exists |
| 6 | `git grep -n 'unattributed:' -- statusgen/lintaudit.go` | exit 0 — **DEREFERENCE**: untagged rules really do fall into an unattributed bucket, which is why task step 1 must make them visible |
| 7 | `go test ./statusgen/ -run 'EnforcementStatus' -count=1` | exit 0 — the emitter's tests pass, including the three-value status set and the stable ordering |
| 8 | `go test ./tools/skillslint/ -count=1` | exit 0 — the generator and its byte-diff pass, with the new block included |
| 9 | `go test ./tools/skillslint/ -run 'GeneratedBlockDiffFailsOnHandEdit' -count=1` | exit 0 — **positive control**: a hand-edited generated block reddens the gate; a freshly generated one does not |
| 10 | `go test ./statusgen/ -run 'EnforcementStatusTracksTheLint' -count=1` | exit 0 — flipping one rule's enforcement status in the source changes the generated block: the claim is derived, not merely present |
| 11 | `grep -c 'nothing enforces this yet' plugins/assay/skills/author-brief/SKILL.md` | exit 1 (zero hits) at implementation — the instruction to write deliberately non-conforming entries on a now-false premise is gone. One hit today (2026-08-25 @ `657cab1`), which is the defect |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) is the generated block genuinely
derived — is there a test that changes the source and observes the block change, not merely a test
that the block exists? (2) is the three-value status set preserved, with "not enforced" visible
rather than elided? (3) is the byte-diff gate reached by a workflow that runs on pull requests?
(4) is the generator's scope held to enforcement claims, leaving the methodology prose hand-authored?
