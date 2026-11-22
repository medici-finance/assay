# Genre stubs — the five not yet written out

These five carry `id`, `mandate` and `pick_when` only. **A stub is not usable
for staging.** If CLASSIFY routes here, write the full playbook first — the
field list is in SKILL.md's HANDOFF schema and the two complete playbooks
(`technical-deep-dive.md`, `executive-explainer.md`) are the shape to follow:
audience block, pick/skip/instead, 5–9 beat arc, slot sequence with functions
and no visual field, bold moment with a placement rule, deck **and** video
time shapes, opening/closing moves each naming the failure they avoid,
transitions per boundary, kill conditions, anti-patterns, gates.

Writing one is roughly half a day and should be triggered by a real artifact,
not by a desire for a complete set. Playbooks written speculatively encode
nobody's actual audience.

---

```yaml
- id: investor-pitch
  mandate: >
    Make a funder believe this team can take this market at this moment — and
    give them the one number they will repeat to their partners.
  pick_when:
    - capital is the ask, from people whose job is allocating it
    - the audience will see fifty of these this month
    - the decision is a partnership meeting you will not be in
  known_open_question: >
    The conditional ordering branch — founder-led decks put the team early,
    product-led decks put it late — is the one real structural fork in this
    genre and the playbook must state the selection rule, not offer a menu.

- id: board-update
  mandate: >
    Let a governing body exercise oversight without reading the appendix —
    surface what changed, what is off-plan, and what needs them.
  pick_when:
    - a standing body meets on a cadence and holds fiduciary duty
    - most of the content is unchanged since last time
    - the real content is exceptions, not status
  known_open_question: >
    This genre's hardest rule is what to LEAVE OUT of the walk while keeping
    it in the pack. It is the strongest test of the appendix discipline.

- id: sales-proposal
  mandate: >
    Move a buying group — not a buyer — from interest to a specific next
    commitment, with the objections of the absent members pre-answered.
  pick_when:
    - a purchase is being progressed and several people must agree
    - the artifact will be forwarded to people who never meet you
    - a competitor is in the room whether or not they are named

- id: internal-decision
  mandate: >
    Get one specific decision made, by a named person, in this meeting.
  pick_when:
    - the decision-maker is present and holds the authority
    - the options are enumerable and the recommendation is real
    - not deciding has a stated cost
  known_open_question: >
    Recommendation-first or options-first is audience-dependent, not a house
    default: some decision-makers want the answer immediately, others read a
    recommendation-first deck as having been denied the alternatives. The
    playbook must make this a stated fork with a selection rule.

- id: conference-keynote
  mandate: >
    Leave a room of strangers holding one idea they will repeat to someone who
    was not there.
  pick_when:
    - large room, one-way, projected, hall lane by construction
    - the audience chose to be there and owes you nothing
    - success is propagation, not agreement
  known_open_question: >
    The only genre where the artifact legitimately does NOT stand alone
    without its narrator — which inverts the read-fallback rule that governs
    every other playbook here. That inversion needs stating explicitly, or
    the house default will quietly ruin the genre.

```
