---
brief: spec-routing/01
title: Enforce the §8 spec/scoping-doc lifecycle — the linter and the authoring-owed emitter
why: >-
  A specification can be ruled as the plan of record and then sit indefinitely with no brief ever
  authored against it — the edge from approval to authored work lives only in human memory. The
  lifecycle spec already gives that edge a machine-readable header, but a header nothing parses is a
  header nothing watches. This brief makes the approved-but-unrouted condition enforceable and, when
  it holds, files exactly one work-ready issue so the owed authoring cannot be silently forgotten.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
exec-tier: strong
exec-tier-why: >-
  Two design calls that must hold across three artifacts (the header parser, the owed-detector, and
  the emit-mode). Where the `**Status:**`/`**Routes-to:**` parser lives relative to the existing
  frontmatter reader, and how the §8.5 citation dereference is computed against authored `sources:`
  provenance without re-implementing the brief loader, are the central choices; getting either wrong
  produces a detector that is confidently right on the happy path and silently wrong on legacy docs.
issues: []
schema: brief-v1
authored: 2026-08-30 (authored for the spec-routing board)
sources:
  - "`spec/lifecycle-v1.md` §8 — the normative convention this brief enforces. §8.1 the `**Status:**` header grammar (first token exactly one of `draft`/`approved`/`routed`, optional ` — <prose>` behind an em-dash delimiter; a first token outside the set leaves the document unclassified/legacy and MUST be ignored). §8.3 an `approved` or `routed` document MUST carry a `**Routes-to:**` line and a linter MUST flag one that lacks it; a `draft` MUST NOT be required to carry it. §8.5 the citation rule: a document is cited when at least one brief's `sources:` frontmatter contains the document's repo-relative path — a prose title mention does NOT count — and an `approved` document that no brief cites has brief-authoring **owed**; only `approved` documents are owed-candidates. §8.6 a backfill whose correct state is undeterminable MUST default to `draft` (the failure that costs least)."
  - "The marker-deduped issue-emitter family already in `statusgen/decisionissues.go`: a hidden `<!-- … -->` per-item idempotency marker rendered as the first body line, an existing-markers set loaded from open issues (tolerant of raw `gh issue list --json body` output), and a JSON payload array a workflow feeds straight to `gh issue create`. The authoring-owed emitter is a new member of this family — reuse the marker/existing-markers/JSON-payload shape; do not invent a second dedup convention."
  - "The stable rule-tag bracket convention on emitted lint lines that `statusgen/lintaudit.go` extracts into a firing audit — the new PROBLEM/NOTICE lines key on it so the rules are auditable rather than unattributed."
  - "First instantiation: the convention and the emitter already run against an approved-spec corpus in a private sibling tree; this brief de-houses the adopter-generic device, not that instance, and reproduces none of its internals."
  - "freshness-checked 2026-08-30 @ `814c0cb` (origin/main): `git grep -niE 'routes-to' -- statusgen/` returns nothing (no header enforcement in the lint today), `git grep -niE 'authoring.?owed|owed-issues' -- statusgen/` returns nothing (no owed-detector), and `spec/lifecycle-v1.md` §8 is present on main (the authority exists)."
consumers:
  - "The routed edge's own downstream board-side consumption is an adopter-configured follow-on, not code in this brief: follow-up spec-routing/01"
---

# Brief 01 — Enforce the §8 lifecycle: the linter and the authoring-owed emitter

## Context

`spec/lifecycle-v1.md` §8 already ships the *convention*; this brief builds the *enforcement*. It is
two coupled devices over one header:

1. a **linter** that parses the `**Status:**`/`**Routes-to:**` header, validates the §8.1 grammar,
   requires `**Routes-to:**` on `approved`/`routed`, and computes the §8.5 owed condition; and
2. an **authoring-owed emitter** — a marker-deduped main-push watcher that files one work-ready
   issue per approved-but-uncited document, shipped as a `statusgen` emit-mode plus a
   workflow-template.

files:
- `statusgen/` (implementation home) — a new reader for the spec/scoping-doc header block
  (`**Status:** <state>` and `**Routes-to:** <dest>` per §8.1/§8.3), the §8.5 owed-detector that
  dereferences the document's repo-relative path against every brief's `sources:` frontmatter, the
  lint rules those two feed, and the `--owed-issues` emit-mode. Plus tests and their fixtures.
- The emitter's **workflow-template** — a main-push watcher, shipped under this repo's
  workflow-template surface, that on a push to the default branch lists the currently-open owed
  issues (for their markers), runs the emit-mode, and files one issue per new payload. It is a
  template an adopter installs into its own CI; it is not this repo's own live wiring.
- `statusgen/lintaudit.go` — read for the stable rule-tag bracket convention the new PROBLEM/NOTICE
  lines must carry; extend the rule set, do not fork the convention.
- `statusgen/decisionissues.go` — read as the reuse anchor for the marker/existing-markers/JSON
  emit shape. **Extend the family; do not duplicate the mechanism.**

facts:
- **The convention is on `main` and this brief does not touch it.** `spec/lifecycle-v1.md` §8 defines
  every rule enforced here. This brief adds no spec text; if §8 is reshaped, re-scope rather than
  enforce a superseded grammar.
- **The header grammar is exact (§8.1).** A document opts in with a header line `**Status:** <state>`
  where `<state>` is the FIRST token and is exactly one of `draft`, `approved`, `routed`, optionally
  followed by ` — <free prose>`. A `**Status:**` line whose first token is not one of the three
  leaves the document **unclassified (legacy)**: the detector MUST ignore it and MAY warn that it
  carries an unparseable state. It MUST NOT be rounded up to any real state.
- **`**Routes-to:**` is required on `approved`/`routed` only (§8.3).** The linter flags an
  `approved`/`routed` document with no `**Routes-to:**` line as a PROBLEM; it MUST NOT require one on
  a `draft`. The destination is a repo-relative path (for example a `docs/streams/<stream>/`
  directory); the check is presence-and-shape, not that the destination is any good — presence is the
  control, adequacy stays review.
- **The owed condition is a dereference, not an assertion (§8.5).** A document is **cited** when at
  least one brief's `sources:` frontmatter *contains the document's repo-relative path*. A prose
  mention of the title does NOT count, and neither does a citation of the title without its path. An
  `approved` document that no brief cites has brief-authoring **owed**. Only `approved` documents are
  owed-candidates: a `draft` watches nothing, and a `routed` document is by definition already cited.
  The detector reuses the existing brief loader's `sources:` parse rather than re-reading frontmatter
  by hand — that is the design call `exec-tier-why` names.
- **Safe default under uncertainty (§8.6).** When a legacy document's correct state cannot be
  determined, treat it as `draft`. A wrongly-`draft` document is silent status quo; a
  wrongly-`approved` document makes owed-detection emit noise, so the cheaper failure is preferred.
  This is also why the linter must not red this very repo's own specs: a spec doc that carries no
  `**Status:**` header is unclassified/legacy and is IGNORED, never defaulted up to `approved`.
- **The emitter is a member of an existing family.** `statusgen/decisionissues.go` already emits
  marker-deduped JSON issue payloads (`--decision-issues`) that a workflow feeds to `gh issue
  create`, keyed on a hidden `<!-- needs-decision: <id> -->` first-line marker and de-duplicated
  against an existing-markers set. `--owed-issues` copies this shape exactly: a per-document hidden
  marker (keyed on the document's repo-relative path), an existing-markers input read from the open
  owed issues, and one JSON payload per NEW approved-but-uncited document. Re-running the emitter
  when the issue is already open emits nothing — exactly one open issue per owed document.
- **The emitter files as a trusted App identity the adopter's roster binds.** The workflow-template
  does not name any specific identity: it files the work-ready issue as the trusted App identity the
  adopter's own roster binds for this purpose. Which App that is, is the adopter's configuration, not
  part of this device.
- **Three states, and rule-tags.** Every new check reports PASS / PROBLEM / could-not-check; an
  unreadable tree or an unparseable header is could-not-check, never a silent pass. Every new
  PROBLEM/NOTICE line carries a stable `[rule-tag]` bracket token so the firing audit can see it.
- **Scope discipline.** This brief enforces the §8 lifecycle and files the owed issue. It does NOT
  judge whether a routed destination is the *right* stream, does NOT auto-author the owed brief, and
  does NOT flip any document's state — the §8.4 forward flips ride the PRs that make each true and
  are out of scope here.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done, and you never flip a PR ready.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI). Run
  `statusgen --root . --lint` (read-only) to check; never `--root` in write mode on the branch.
- If a change would delete, disable, or weaken a security/access-control control or its CI
  assertion, STOP and escalate — this brief instructs none such.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Parse the header (§8.1/§8.3).** Add a reader that, given a spec/scoping document, extracts the
   `**Status:**` state (first token; the ` — <prose>` tail discarded for the machine value) and the
   `**Routes-to:**` destination. A first token outside `{draft, approved, routed}` yields
   **unclassified**, not a real state. Locate the reader relative to the existing frontmatter/markdown
   reading in `statusgen/` and state the placement decision in the source header.
2. **Add the grammar + Routes-to lint rules.** An `approved`/`routed` document with no
   `**Routes-to:**` line is a PROBLEM; a `draft` is exempt. An unparseable `**Status:**` first token
   is at most a NOTICE (unclassified). Each line carries its stable `[rule-tag]`.
3. **Compute the owed condition (§8.5).** For each `approved` document, dereference its repo-relative
   path against every brief's `sources:` frontmatter (reusing the existing brief loader, not a fresh
   frontmatter scrape). Cited ⇒ not owed; uncited ⇒ authoring **owed**. `draft` and `routed`
   documents are never owed. Emit the owed condition as an advisory NOTICE in the lint.
4. **Add the `--owed-issues` emit-mode.** Following `statusgen/decisionissues.go`: emit a JSON array
   of work-ready issue payloads, one per approved-but-uncited document, each carrying a hidden
   per-document marker (keyed on the repo-relative path) as the first body line, and skip any
   document whose marker is already present in the existing-markers input. A missing/empty
   existing-markers input means nothing is filed yet.
5. **Ship the emitter workflow-template.** A main-push watcher template that, on a push to the
   default branch, loads the open owed issues' markers, runs `--owed-issues`, and files one issue per
   new payload as the trusted App identity the adopter's roster binds. It is a template an adopter
   installs; do not wire it as this repo's own live workflow.
6. **Positive controls (make the detector prove it is derived, not merely present).** Tests that: an
   `approved` document with no citing brief is owed; adding a brief whose `sources:` contains that
   document's path flips it to **not owed** (the dereference is real, not a title match); a `routed`
   and a `draft` document are never owed; an `approved`/`routed` document missing `**Routes-to:**`
   reddens the lint while a `draft` does not; and a document whose owed-marker is already in the
   existing-markers set emits NO payload (idempotency — exactly one open issue).
7. **Fail-first evidence.** For each test that asserts the enforcement behaviour or pins the
   idempotency/derivation guards, show it failing on the pre-fix tree (a red run quoted in the PR
   body under `## Fail-first`, or a committed mutation entry the reviewer can re-run). A red that
   legitimately cannot be produced is reported as such, never engineered by weakening the assertion.
8. **Do not red this repo's own tree.** `statusgen --root . --lint` stays green: this repo's specs
   that carry no `**Status:**` header are unclassified/legacy and are ignored (§8.6), not defaulted
   up to `approved`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -niE 'routes-to' -- statusgen/` | exit 1, no output — **DEREFERENCE, true at authoring (2026-08-30 @ `814c0cb`)**: no `**Routes-to:**`/header enforcement exists in the lint today. Inverts at implementation |
| 2 | `grep -n '^## 8. Spec and scoping-doc lifecycle' spec/lifecycle-v1.md` | exit 0; one hit — **DEREFERENCE**: the normative convention this brief enforces is on `main`, so the authority exists and is not asserted |
| 3 | `git grep -niE 'authoring.?owed\|owed-issues' -- statusgen/` | exit 1, no output — **DEREFERENCE**: no owed-detector or `--owed-issues` mode exists today, so building it is genuinely this brief. Inverts at implementation |
| 4 | `git grep -n 'decisionMarker(' statusgen/decisionissues.go` | exit 0 — **DEREFERENCE**: the marker-deduped issue-emitter family the emit-mode extends really ships, so `--owed-issues` reuses a mechanism rather than inventing one |
| 5 | `git grep -n 'unattributed:' statusgen/lintaudit.go` | exit 0 — **DEREFERENCE**: untagged lint lines really fall into an unattributed bucket, which is why the new PROBLEM/NOTICE lines must carry a `[rule-tag]` |
| 6 | `go test ./statusgen/ -run 'SpecStatusHeader' -count=1` | exit 0 — header parser tests pass: first-token classification, the ` — <prose>` tail discarded for the machine value, and a non-set first token yielding **unclassified** rather than a real state |
| 7 | `go test ./statusgen/ -run 'RoutesToRequired' -count=1` | exit 0 — an `approved`/`routed` document missing `**Routes-to:**` reddens the lint; a `draft` does not |
| 8 | `go test ./statusgen/ -run 'AuthoringOwed' -count=1` | exit 0 — **positive control**: an `approved`+uncited document is owed; adding a brief whose `sources:` contains its path flips it to not-owed; `draft`/`routed` are never owed |
| 9 | `go test ./statusgen/ -run 'OwedIssueMarkerDedup' -count=1` | exit 0 — **positive control**: a document whose owed-marker is already in the existing-markers set emits NO payload (exactly one open issue per owed document) |
| 10 | `statusgen --root . --lint; echo rc=$?` | `rc=0` — the new checks do not red this repo's own tree: unclassified/legacy specs are ignored, never defaulted up to `approved` (§8.6) |
| 11 | `statusgen --root . --consumers --brief spec-routing/01` | exit 0; `1 corroborated, 0 disproved` — the brief's single self-routed `consumers:` claim is corroborated against the diff, not asserted; the routed edge's downstream consumption is a `follow-up <self>`, so no external consumer is enumerated |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) is the owed condition genuinely a
*dereference* against authored `sources:` provenance — is there a test that adds a citing brief and
observes owed flip to not-owed, not merely a test that the detector runs? (2) is the §8.6 safe
default honoured — does an unclassified/legacy document stay ignored rather than defaulting up to
`approved`, and is the repo's own tree proven green? (3) is `--owed-issues` idempotent against its
existing-markers input, so a re-run over an already-open issue files nothing? (4) does the
workflow-template file as *the trusted App identity the adopter's roster binds*, naming no specific
identity, and is it a template rather than this repo's own live wiring?
