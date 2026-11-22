# Video editorial review

Read for new videos, substantive script revisions and standalone excerpts,
before visual authoring. This is the shared editorial procedure; genre
playbooks choose the story shape. It owns comprehension and visual-quality
review through production; a working renderer does not establish either.

## 1. State the viewer contract

In `staging.yaml`, identify the audience, assumed knowledge and concepts that
need explanation (the `audience:` block). Industry experience or a senior job
title does not imply knowledge of our product, vocabulary or specialist topic.
Then record the rest in `viewer_contract:` — the motivating problem (`problem`),
its consequence for this viewer (`stakes`), the opening's promise
(`opening_promise`), the question
this film answers (`question`), one takeaway (`takeaway`), one useful next
action (`next_action`), the concrete example or case that carries the
explanation (`carrying_example`), what belongs in a companion article
(`companion_scope`), and the target runtime (`runtime_seconds`). Use the
brief's answers; state reasonable assumptions instead of interviewing again.
If an existing film keeps its contract in `editorial-review.md` instead, the
same answers must be present there. Identify likely mistaken inferences in the
review record: what might this viewer reasonably hear us promising that the
film does not establish?

For a task tutorial, also read [teach-workshop](genres/teach-workshop.md) and carry
its learning contract. The takeaway is a testable capability; `next_action` may be
the transfer task. Do not replace demonstration, practice and feedback with a CTA.

A viewer arriving from search must understand the subject and promised benefit
without reading the article first. A cold open may do this in one sentence;
it does not mean “omit context.” For a roughly three-minute practitioner film,
try 15–25 seconds of orientation including the motivating situation. This is a
starting budget, not a universal retention rule or a request for a logo sting.
The teaser answers **what this is about and why I should care** promptly, with
an honest promise of what the viewer will understand or decide. A product name,
slogan, historical analogy or intriguing incident alone does not answer both.
Orient before evaluating an unfamiliar mechanism: practical problem → subject
in plain language → useful behavior or outcome. A specialist film can start
further along only when its declared audience knowledge supports that choice.

When borrowing an external system's design, establish **the pressure that led to
it** before the carrying example. Explain whose problem it solves and why the
existing approach struggled. If our experience motivates the adaptation, give our
actual pain and the material difference in workload too. A product name, large
number or unexplained incident does not establish this causal background. Keep
reported behavior, our verified experience and a proposed adaptation distinct.

Write for the ear. Keep the decision, practical action and consequential limit in
speech; put audit detail such as exact query windows, test receipts, hashes and API
fields in supporting evidence unless the viewer needs them for the promised task.
Use readable rounding with units and period, preserving exact sources and arithmetic
in the evidence record. Keep exact values when a threshold or task depends on them;
never round across a decision boundary. Prefer a heading that says what happens over
an editorial label such as “the hedge.”

If instrumentation supports the story, say what we can now measure separately from
what we have measured. Verify implementation and actual enablement, explain coverage
gaps, and identify remaining collection needs. Adding a logger is not a measured
speedup; a partial counter's zero cannot establish that nothing happened.

## 2. Develop and review the script with rough visuals

Use [creative development](creative-development.md) to explore distinct treatments
and test an animatic alongside the script. Stage audience/evidence first; resolve
script-level findings before detailed animation, without requiring visual ideas
to wait for a fully locked script. Record alternatives and the selected visual
discovery here rather than adding a new approval or document.

Draft `SCRIPT.md` to the chosen arc using
[video-author's pipeline contract](../../video-author/SKILL.md#2-the-shared-script-and-html-preview-contracts)
even before visual authoring: its parser-facing table must be consumable at handoff.
Read it aloud when audio is available;
otherwise do an explicit prose read-through and label it that way. Estimate
runtime from actual word counts and the declared speaking rate; include a
10% slower take and room to inspect evidence and diagrams. Count the current
text, including additions made during review; old scene estimates are not a
word budget. Cut scope or agree a longer film before accelerating delivery.

Write a continuous spoken story, not a series of independent scene explanations.
Carry the same situation, question or work item through its changing state: why this
step is needed, what happens, and why that result leads to the next step. A technical
film can use a causal argument or diagnosis rather than inventing characters or an
incident. Keep actors, objects and terms consistent across the sequence.

At each scene boundary, read the last sentence and the next opening aloud (or mark
an explicit prose read-through). The opening must give the viewer a reason to follow
the next visual; “next, caching” or a spoken slide heading is not a story transition.
Introduce the new input, actor or question before exposing an unexplained diagram.
Plan where that lead-in lands and which visual is revealed afterwards. An authored
pause or scene-duration scale alone does not prove alignment to the eventual voice.

Then read the whole script without looking at the slides. It must make sense as a
connected explanation, with the example or argument developing rather than restarting
at every cut. Repair an abrupt catalogue at staging, before animation; do not hide
it with motion, add filler bridges, or invent causation to make a story sound smoother.
Use speech-friendly terminology and expand unfamiliar abbreviations when the viewer
needs their meaning; retain project-specific pronunciation/spelling choices in the
film's contract instead of making them universal vocabulary bans.

Write `editorial-review.md` beside the film. For every criterion below, record
**pass / revise / not-yet-checked**, the scene or sentence that supports the
judgment, and the required fix. A record written with other labels maps each
one to these three states and says so (for example `PARTIAL` = revise). Do not replace findings with a total score.
A pass requires evidence; “looks good” and successful playback are not evidence
for comprehension. Resolve script-level revisions before detailed production; rough shots may help
find the revision.
Visual/voice criteria may remain explicitly pending at this stage.

| Criterion | Review question and failure signal |
|---|---|
| Opening promise | Can a new viewer say what this is about, who it helps, why to care and what the film will explain after the opening? An unexplained incident, acronym, product name or delayed metaphor fails. |
| Problem and background | Before the carrying example, can the viewer explain the pressure that led to the design, who faced it and its consequence? For an adaptation, is our distinct workload and pain explained? Cite the case or evidence establishing the problem; an assertion that something is broken does not establish it. |
| Audience knowledge | Can someone with the declared industry knowledge follow without knowing this topic or our system? Identify every needed prerequisite outside `assumed_knowledge`, and where it is introduced. |
| One takeaway | Does every scene advance the same central judgment or learner capability? Necessary supporting mechanisms may share that outcome. Name what moved to the companion material. A compressed list of the article's headings fails. |
| Progression and example | Does the whole spoken script tell a connected story, and does each boundary explain why the next step follows before introducing its visual? Carry an example, decision or causal sequence from input through behavior to useful outcome; do not abandon it for a catalogue of components. Label illustrative cases as illustrative. |
| Load and language | Is reviewer-only detail in supporting evidence, precision useful to this decision, unfamiliar terminology explained before use, sentences speakable, and runtime plausible from current word counts at the declared pace? Numerical claims need units and a comprehensible denominator. Preserve time to inspect what is shown. |
| Visual explanation | For each planned visual, name the narration clause and relationship it will explain. Decoration or a full diagram revealed before its explanation needs revision. The finished version is checked again while playing. |
| Visual appeal | Does the visual treatment earn attention through composition, hierarchy, typography, meaningful imagery and clear flows? A succession of prose cards or a default block diagram is not a finished treatment merely because it animates. Inspect stills and playback; name the strongest discovery, weakest passage, repetitive framing and what the image adds beyond the voice. |
| Evidence | Distinguish measured results, testimony, illustrations and proposed settings. Put limitations needed to interpret a claim in speech as well as on screen. Do not turn one observational case into a causal or universal claim. |
| Interpretation and limits | What might a reasonable viewer wrongly conclude? Explain consequential conditions and boundaries beside the benefit: a protected identity does not establish a good test, a risk score is not automatically a calibrated probability, and a designed transaction is not demonstrated market liquidity. |
| Ending | Does the final action answer the opening promise and fit this audience? “Learn more” alone is insufficient when a concrete useful step is available. |
| Distribution fit | Does the cut complete its argument in the declared destination and use that surface's brand authority? A native LinkedIn clip cannot rely on a click, title-only runway, the parent film's opening/payoff, or product styling as a substitute for the declared personal-executive system. A YouTube companion needs a distinct depth/reference job, not merely a different upload. |
| Accessibility | Keep short on-screen labels separate from full-speech captions. Plan readable contrast/type, enough inspection time and a text alternative. Check caption coverage, segmentation and synchronization on the final take. |

For tutorials, include the playbook's demonstration, attempt/feedback and transfer
gates in this record. Useful repetition can support retrieval, comparison or
consolidation; reject it only when it adds no such work. Keep the same case or
visual object where continuity helps the viewer follow a changing relationship.

Add three retell sentences: the problem, the principle demonstrated by the
example, and the next action. This is the author's coherence check. It is
**not** evidence that an audience has understood the film.
Name the weakest criterion even when the review finds no required revision.

## 3. Run a fresh-context teach-back

Use an independent reviewer context supplied only the script, planned visual
sequence and viewer contract, including assumed knowledge. Keep the author
conversation, repository material and companion article out of this read; do
not retrieve missing subject knowledge. Check factual correctness separately
against sources. For a completed film, repeat using the film and contract;
a script-only read cannot clear visual, voice or caption criteria.

Ask the reviewer to explain, in their own words:

1. The problem, who faces it and why it matters.
2. How the mechanism works, following the carried example to its outcome.
3. The consequential limit or tradeoff, and the useful next action.
4. For instructional promises, the predicted outcome of a changed input or boundary
   case not already solved in the script, with reasoning grounded in what the film
   taught. For a tutorial, attempt its transfer task without the solution key and
   compare with the key afterwards. Do not supply extra knowledge to fill a gap.
5. The most tempting wrong inference and the line or visible relationship that
   prevents it. For a news/update film, use this evidence-scope check instead of
   inventing a hands-on exercise.
   When the film recommends a decision, also identify the strongest plausible
   alternative and the evidence that would change the recommendation. Cite the
   film's support or record a missing decision boundary.

Each answer cites the scene, phrase or visible event that supplies it. Mark
missing terms and inference steps instead of completing them from the
reviewer's own expertise. The audience criterion needs revision when the
promised understanding requires an unexplained prerequisite. Retain the
teach-back and gaps in `editorial-review.md`; recheck affected answers after
revision. An author retell is not this independent read. If a fresh context or
the relevant medium is unavailable, record that part as not-yet-checked.

This is an agent simulation of comprehension, not measured human comprehension
or factual verification. Keep its status separate from actual audience testing.

## 4. Carry the story into production

Hand off the contract, script and review to `video-author`. Map visual changes
to narration clauses. Word-count timing is provisional; scene-length scaling
does not prove that a reveal matches a particular spoken phrase. Check the
actual voice take. Re-review affected criteria after any substantive edit.
Review visual appeal as well as correctness, using
[video-author's screen grammar](../../video-author/references/screen-grammar.md).
The design pass owns the visual treatment; staging owns what it must explain.
Keep concrete layout, palette, chart and effect choices in the design section
of `editorial-review.md`, never in `staging.yaml`. At staging handoff, visual
criteria can carry their semantic purpose with treatment and playback pending.
Use a designed diagram flow, comparison, artifact close-up or illustrative
scene when it helps. Animation is one means, not the quality criterion. A
well-composed still can pass; motion cannot rescue an unreadable or generic
composition. An explanatory figure may show the same relationship the voice
describes; complementing speech is not duplication by itself.

The default SCRIPT table has no `subtitle` column: the current engine then
uses full narration for captions. Its current generator emits one cue per
scene. Before upload, split long cues into readable segments and align them
to the approved speech. A summary subtitle or one paragraph on screen for
25 seconds is not an adequate substitute. Do not assert that this skill has
implemented forced alignment or caption segmentation in the engine.

Use separate statuses in the same review record:

- **Script:** reviewed text, word count, estimates, findings/fixes.
- **Visual:** alternatives and selected discovery, metaphor limits and rough-cut
  findings, then inspected stills and playback
  findings for both explanation and appeal.
- **Voice and captions:** pending until the actual take is heard and captions checked.
- **Render:** exact runtime/adapter tests performed and their limitations.
- **Agent teach-back:** medium reviewed, answers with scene evidence and gaps,
  or not-yet-checked with the reason.
- **Human audience comprehension:** not tested, or the method and actual responses.

Bind each status to the artifact actually inspected: path and revision/checksum,
review date, reviewer role, medium, scene/timecode evidence and findings. For final
production, identify the delivered video, voice take and captions together. A
script hash does not identify a rendered take. For a pending check, name the next
responsible role and the evidence it must collect; do not invent a completed review
or a human commitment. Changed script, timings, visuals, audio or captions invalidate
the affected statuses and require rechecking that version before claiming readiness.

Before calling a film production-ready, review its actual voice/captioned
version and clear applicable editorial findings; pending checks remain visible
at the authoring handoff. Successful encoding or a private upload does not
clear them. This is a review procedure, not a new automatic media gate.

For each standalone clip, recheck its opening, necessary definitions, carried
example, payoff and qualifications using only what survives the cut. A parent
film's pass does not transfer to an excerpt. Preview the intended aspect ratio:
recompose authored material or choose a recording layout that preserves the
diagram, screen region and readable labels. Tightening pauses must preserve
the time needed to understand evidence and visual relationships.

A useful optional audience check is to show the film to someone representative
of its audience and ask what it helps them decide, what the example established,
and what they would do next. Record their answers, not a guessed score. Do not
invent a participant or hold routine work waiting for one. Publication approval
continues to follow the user's existing instructions; this procedure creates
no extra permission gate.

## Calibrate production decisions

When revising these rules or reviewing a new genre, compare a house specimen with
at least two external examples matched to audience, job and complexity. Record
the technique, an exact passage/frame/timecode, and what understanding it enables.
Separate defects in truth, learning and production from differences in house taste.
Do not turn a sample's length, scene count, pacing or presentation style into a
universal target. Check the inspected medium: transcripts support script judgments,
stills support composition judgments, and audio/playback is needed for delivery,
timing, caption sync and audiovisual quality. An unavailable medium stays unchecked.

Starting references from the October 2026 benchmark:

- [Vox, AI art explained](https://www.youtube.com/watch?v=SVcsDDABEkM): the
  6:21–8:27 explanation distinguishes retrieval from generation and corrects an
  inadequate single-feature model. Compare how a misconception is exposed and repaired.
- [IBM, A2A vs. MCP](https://www.youtube.com/watch?v=BMDFPOyezH4): the late carried
  case and 10:37/10:52 diagram states support comparison of relationships on a
  persistent subject. Do not copy the late case placement as a default opening.
- [3Blue1Brown, a short LLM explanation](https://www.3blue1brown.com/lessons/mini-llm/):
  the word/river example makes an abstract representation inspectable. Compare
  the learner's inference, not the brand's animation surface.

That benchmark inspected transcripts/adapted text and selected frames; it did not
perform complete audio/playback review. Reinspect the relevant medium before using
these references to claim production parity. Validate a proposed rule on both a
strong passage and a plausible weak one, then retain the false positives and
before/after specimen. Post-publication retention and clicks can locate questions;
they do not establish comprehension or persuasion without evidence of those outcomes.

## Basis and limits

These are house editorial choices informed by the following sources, reviewed
2026-09-27. They are not a validated universal scoring scale or proof that
three minutes is optimal.

The audience teach-back adapts the house article-craft procedure. The
2026-10-12 production review adds explicit problem/background, industry-versus-
topic knowledge, opening promise, mistaken-inference and visual-appeal checks.
These are production direction, not claims of measured audience effectiveness.

- [Common Craft's explainer tips](https://www.commoncraft.com/blog-categories/explainer-tip): establish context and explain relationships before details.
- [Wistia's scripting guidance](https://wistia.com/blog/wistias-scripting-tips): audience, takeaway and writing for speech.
- [Brame, Effective Educational Videos](https://pmc.ncbi.nlm.nih.gov/articles/PMC5132380/): signaling, segmentation, reducing extraneous load, engagement and active learning. Educational research informs the design; it does not establish a marketing-video retention target.
- [YouTube's audience-retention guidance](https://support.google.com/youtube/answer/9314415): inspect the first 30 seconds against title/thumbnail expectations and inspect later drop-offs after publication. Retention measures viewing behavior, not comprehension.
