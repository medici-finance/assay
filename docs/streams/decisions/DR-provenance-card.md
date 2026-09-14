---
id: DR-provenance-card
date: "2026-09-12"
title: "A provenance card states mechanical signals about an unknown pull-request author as neutral facts, and renders no verdict and no score"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Emit a composite trust SCORE (a number or a red/amber/green badge) — ruled out: a score is a verdict wearing a fact's clothes. It compresses signals with different meanings and different error rates into one figure that a reviewer will read as an authority, and it is the shape most likely to be experienced as an accusation by a genuine contributor whose account happens to be new. Facts can be argued with; a number cannot."
  - "Gather the signals but keep the card private to the maintainer (no pull-request comment) — ruled out: a control the contributor cannot see is a control they cannot correct. The observed pattern's benign explanation (a new account, a batch of genuine small fixes) is one the author can supply in a sentence, and only a visible card invites that sentence. A private card also loses the audit trail that makes a later promotion or demotion reviewable."
  - "Gather no signals; keep the existing per-item bless as the whole bar — ruled out: this is the measured status quo, and it is what the 2026-09-12 arrivals overran. The maintainer makes the same judgement every time with no instrument, so the judgement's quality tracks how much attention the maintainer had that hour."
  - "Include identity-adjacent signals (profile text, avatar presence, follower counts, named employer, geography) — ruled out: these do not predict whether a diff is correct, they do correlate with attributes the project must not sort contributors by, and the resulting card would read as a dossier on a person rather than a description of a submission."
accepted:
  - "A card is posted publicly on the pull request of an author the roster does not know, and it is visible to that author. Some genuine first-time contributors will read it as unwelcoming. The mitigation is wording — the card says what was measured and explicitly states that none of it is a judgement of the change — not suppression."
  - "Every signal is a cheap proxy with a benign explanation, and the card states the benign explanation alongside the measurement rather than leaving the reader to supply it."
  - "The card is a three-state instrument: a signal that could not be gathered is rendered as could-not-check and is never rendered as an absence of concern."
  - "The card is recomputed and replaced rather than accumulated, so a pull request never grows a column of stale cards; the ledger, not the card, is where history lives."
---

**PROPOSED — no ruling is recorded.** This record captures the design as authored so the
design-approval gate has something to dereference; `decided-by:` is a placeholder until a
human rules on the brief's decision issue, and the ruling is recorded here in the same motion.

The decision is what a mechanical provenance instrument is allowed to say. The constraint
behind it is that every signal available from public metadata — how old an account is against
when it first acted, how long elapsed between a fork and its pull request, how many pull
requests an author opened across repositories in a day, how their prior submissions were
closed, how similar their bodies are to each other, whether commits are signed, and whether
the diff touches continuous-integration configuration, dependency lockfiles, install scripts
or container definitions — is a **correlate of automated bulk submission, not a measure of
correctness or of intent**. Each has an ordinary, innocent explanation. A design that lets the
instrument conclude therefore converts a set of weak correlations into a strong-looking
statement about a person, which is both unsound and, on a public repository, harmful.

So the instrument reports and stops. The reviewer, who can read the diff, concludes.

**What this record does not decide.** It does not fix the exact signal list (that is the
brief's Task and the review gate's judgement), it does not decide which tier any card leads
to, and it does not license the card as an input to any automatic tier change — promotion and
demotion are recorded human acts under the tier record.
