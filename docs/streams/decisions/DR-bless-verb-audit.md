---
id: DR-bless-verb-audit
date: "2026-09-12"
title: "Blessing becomes a structured, tool-written comment with a fixed marker and a reason, replacing free-text recognition"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Keep parsing free text (any comment by the blessing authority admits the item) — ruled out: the admitting act is currently indistinguishable from any other remark the authority makes, so 'thanks, I'll look at this' admits an item into automation exactly as a considered approval does. The gate cannot tell a decision from a pleasantry, and the record afterwards cannot say what was decided or why."
  - "Use a LABEL as the blessing token instead of a comment — ruled out on its own: a label carries no reason and no author in its rendered form, its history is a timeline entry rather than content, and the bless-then-edit rule has nothing to compare against because a label has no timestamp relative to the item's content. A label may accompany the comment for queue legibility; it may not be the record."
  - "Accept a reaction (a thumbs-up by the authority) as the blessing — ruled out: a reaction carries no scope, no reason and no tier, and the existing per-write reaction gate is already the control this stream is trying to give shape to, not a shape to copy."
  - "Let the verb also PERFORM the tier promotion implied by a bless — ruled out: it collapses two decisions of very different consequence into one keystroke. Admitting one item is routine; granting a standing tier is not, and a verb that does both will do the second by accident."
accepted:
  - "The marker is a machine contract. Once the gate keys on it, a maintainer who types a blessing by hand without it does NOT admit the item — the act silently does nothing. Mitigating that is the verb's job (it is the sanctioned path) and the published policy's (it says how admission happens); the alternative, accepting both shapes, restores the ambiguity the change exists to remove."
  - "The bless-then-edit rule is preserved exactly: content added or edited after the blessing comment voids it. The structured form does not weaken it and must be shown not to."
  - "A blessing carries a scope (this item only) and a reason. The reason is free text and therefore unverifiable by any tool; its value is to the human reading the audit row later, not to a check."
  - "Every blessing writes an audit row. On a public repository the comment itself is already public, so the row adds no disclosure; it adds the ability to answer 'what has been admitted, by whom, and why' without reading every thread."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder; a human rules on the
brief's decision issue and the ruling is recorded here in the same motion.

The blessing is the single act that moves an inbound item from quarantined to actionable. It
is the narrowest and highest-consequence control in the inbound path, and today it is
recognised by *who wrote a comment*, not by *what the comment says*. Every property one wants
of an authorization — that it was deliberate, that it names its scope, that it states a
reason, that it can be listed afterwards — is absent, and the failure direction is toward
admitting.

The decision is therefore to make the blessing an explicit, tool-written artifact: a verb the
maintainer runs, which posts a comment carrying a fixed machine-readable marker, the scope,
and the reason, and which the trust gate reads in place of free text. The comment remains a
comment — human-legible, in the thread, visible to the contributor — so nothing about the
existing transparency is lost.

**What this record does not decide.** It does not fix the marker's syntax, it does not decide
whether an item may be un-blessed (a revocation shape is left to the brief's Task and the
review gate), and it does not grant the verb any authority over tiers — a bless admits one
item and implies no standing grant.
