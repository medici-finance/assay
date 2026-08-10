# Assay growth design — from single-operator to multi-person SME use

**Provenance:** human:<name>'s direction 2026-07-09 ("we would like others to use it, and grow Assay into a
multi-person system used in a small/medium enterprise"), answered by the assay-review-1 Fable
session in the same conversation as [the operations review](https://github.com/example-org/oit/blob/main/docs/assay-review-1/README.md) and
landed here on human:<name>'s "add this to the docs, and intake." Tracked as **INTAKE I-19**. This is a
**scoping input** for a future `assay-adoption` stream (author-brief at activation, bootstrap-brief
convention applies) — nothing in it is enacted by this document.

**Relationship to existing artifacts:** extends I-02 (methodology-as-product) with the
multi-person/adoption design; builds on brief-07 (toolkit extraction — assay-toolkit exists),
brief-13 (the name), brief-16/17 (non-self-writable gates — the pattern §1 generalizes), brief-22
(desk skills into the repo — step one of §5), I-06 (toolkit init), I-13 (measured effort), I-18
(docs-only merge lane), and the metrics stream (§7's instrument). The red-team's A5 scope-honesty
(the machinery currently serves an agent fleet with ONE accountable human) is the starting fact.

---

## Thesis

Assay today is a **single-cell organism**: one accountable human, one repo, one machine, one git
identity, agents as the workforce. Its two deepest weaknesses — trust is probabilistic
(agent-writable sensors, F-08) and policy is prose (the rule-mass ratchet, review U-03) — are
tolerable at n=1 because the one human personally compensates for both. At n>1 they are the first
things that kill it. The growth path is not "add users"; it is **two inversions, then scale**:
identity makes trust structural, and configuration makes policy enforceable.

## 1. Identity first — trust goes from probabilistic to structural

Everything the red-team disallowed claiming ("measured, not self-reported") becomes claimable the
moment actors have distinct identities. The pattern is already proven once: the reviewer App made
verdict-forgery impossible, not discouraged (brief-17, PR #125 class dead).

- Every agent role gets its own bot identity (GitHub Apps or per-agent signed commits); humans
  keep their own. The single shared git author (`shared-agent`) is the common root under F-13, un-attributable
  Verified cells, and forgeable Evidence rows — one fix retires all three.
- Verified/Reviewed cells stop being free-text strings and become references to attributable acts
  (an App-posted check, a signed commit). statusgen validates *who*, not just format.
  `verifier ≠ implementer ≠ approver` becomes a mechanical check instead of a convention.
- This is also simply what a second human requires on day one: nobody can be onboarded into a
  system where everyone is the same actor.

## 2. Policy-as-config, not prose (assay.yaml)

The rule-mass ratchet (11 CLAUDE.md commits in a day; the one-change budget routed around on day
one) does not scale to two people, let alone a customer. The fix is the move statusgen already
made for status: **rules become data that lint enforces**. Tier floors, risk vectors, gate
assignments, WIP caps, merge lanes, batch sizes — one versioned config file per repo/org
(working name *assay.yaml*, planned). Prose remains only for what cannot be checked. Three
payoffs: the ratchet dies (a rule either becomes config or does not ship), adopters get **knobs
instead of a culture transplant**, and policy changes become reviewable diffs — the R-01 budget,
mechanized.

## 3. Accountability that scales without diffusing (DRI-per-gate)

Assay's sharpest property is that every gate has exactly one accountable human. The naive
multi-person move — everyone can approve everything — destroys that, and diffuse accountability
is *worse* than a single point of failure. The right move:

- Stream frontmatter gains `owner:`; gate authority maps to owners (CODEOWNERS-style).
- `gate: human` means *the accountable human for that stream*, not "the founder, always." The
  founder stops being every gate and becomes the gate for what they own.
- Gate-age becomes a standing alarm (a `gate: human` pending N days pages its owner — the same
  ISA-18.2 standing-alarm logic already used for findings).

## 4. Split Assay-core from house rules, and version the core

Adopters need stability; this repo needs to keep evolving daily. Those coexist only via a split:
**core** (brief schema, lifecycle semantics, register rules, lint behavior — semver'd in
assay-toolkit, with migrations) vs **profile** (this org's tiering, cadences, gate config — the
§2 config layer). Conformance = lint green against a core version. Toolkit `init` (I-06)
scaffolds core + a chosen profile. Without this split every adopter forks a dialect and the
methodology dissolves — the failure mode of every Scrum.

## 5. Ship the operators, not just the file format

The toolkit currently exports the *nouns* (statusgen, schema, registers). The *verbs* — dispatch,
review, verify, coordinate — live in one user's home directory (review U-06). The desk roles must
become portable, parameterized role definitions in the toolkit: runtime-agnostic specs with a
Claude Code reference implementation. A methodology whose operators cannot be installed is not
adoptable. brief-22 (skills into this repo) is step one; toolkit extraction of the roles is step
two.

## 6. A human interface for people who don't poll markdown

One person reads STATUS.md; a five-person team will not. Minimum viable surfaces, all
**generated, read-only, from git** (never a second source of truth):

- Per-person Next-up: "my queue," "gates waiting on me."
- A small hosted board + notification digests (Slack/email) for gate-age and alarms.
- The non-engineer bridge — the real adoption cliff for git-native systems: a form/Slack-bot that
  opens INTAKE-appending PRs, and gate approvals surfaced as GitHub checks so a PM can approve
  without a shell.
- The commercial wedge hides here: a GitHub App that runs the lint, posts the board, **assigns
  register IDs at merge** (killing the F-NN/I-NN collision class permanently), and renders alarms
  — value added while the source of truth stays in the customer's git.

## 7. The ROI instrument is the sales story

An SME buyer asks one question: *what did the agents actually finish, what did it cost, and who
checked?* The metrics stream is already building exactly that instrument (DORA + cost/tier ledger
via I-13, defect-escape via the verify desk). Ship it as the buyer-facing dashboard, and keep
F-12's discipline as a differentiator: Assay refuses to show a leverage number it cannot
recompute. In a market of unsourced "10x with agents" claims, audited honesty is a moat.

## What NOT to do

- **Don't move the source of truth off git.** Work-state in a database is a worse Jira.
  Git-native + CI-enforced + agent-writable is the entire moat.
- **Don't chase human-only teams.** Red-team A5 is right: for them Assay duplicates code review +
  CI + a tracker. The honest segment is teams where agents do a large share of implementation and
  "done" has stopped being trustworthy. Be early and scoped rather than broad and refuted.
- **Don't add estimation theater.** Measured token-size/challenge (I-13) instead of planning
  poker is a feature; guard it.
- **Don't fork dialects.** Profiles + conformance lint (§4) exist precisely so teams customize
  without diverging.
- **Don't publish multi-person claims before multi-person evidence** — the articles' own rule,
  extended to marketing.

## Sequence, with falsifiable exits

1. **Now (filed in assay-review-1):** verification floor, queue drain, dep-precise scheduling.
   *Exit: the board can schedule its own top-severity work; risk-flagged briefs can't be
   cheap-verified.*
2. **Identity + authority (§1, §3):** per-actor identities, identity-checked gates, stream
   owners. *Exit: a forged Verified cell cannot lint green; a real brief passes
   verifier≠implementer mechanically; at least one agent role acts as its own GitHub actor.*
3. **Config layer (§2):** assay.yaml v1. *Exit: ≥5 prose rules deleted with enforcement
   preserved; CLAUDE.md shrinks (brief-14) with zero rule loss.*
4. **Second human — dogfood before selling (§3, §6):** one collaborator owns one stream
   end-to-end. *Exit: a brief reaches evidence-backed `done` with the founder touching nothing
   but merges he owns; handoff friction logged to RETRO.*
5. **Multi-repo board (§6):** the three medici repos as testbed. *Exit: one board; a cross-repo
   `depends:` resolves.*
6. **Toolkit + design partners (§4, §5):** `init`, versioned core, portable roles, then 1–2
   friendly external teams. *Exit: an external repo reaches its first evidence-backed done
   without hand-holding beyond docs; partners still active after 4 weeks.*

## Positioning

**"GitOps for work-state — agent-native delivery governance where status is generated, *done*
requires evidence from someone who didn't do the work, and every gate is keyed to risk and model
tier."** Beachhead: engineering-led, agent-heavy SMEs, with regulated ones (fintech/health) the
sharpest fit — audit-trail-by-construction, and the dogfood story (a regulated-finance DAML
build) is exactly their shape.
