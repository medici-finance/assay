# Teach a task: workshop and video tutorial

**Mandate:** leave a learner able to perform a bounded task and recognize whether
it succeeded. Agreement, recall and watching the presenter succeed do not establish
that capability. This playbook is an initial production contract; real learner
attempts should refine its pacing and recovery guidance.

## Audience and routing

Record what the learner already knows, their starting environment, the task they
need to perform and the mistake they are likely to make. Access and setup are
prerequisites, not evidence of expertise. A senior engineer may still be new to
this particular interface or workflow.

Choose this genre for hands-on workshops, screen walkthroughs, diagnostic lessons
and task-based videos. Choose `practitioner-explainer` when the outcome is deciding
what to try; choose `technical-deep-dive` when the outcome is judging a mechanism's
credibility. A product tour without an attempt and feedback may be useful, but it
does not demonstrate learning. State the routing choice and two rejected genres.

Use one observable capability per lesson. Necessary supporting mechanisms belong
when they explain the same task. Split independent tasks into chapters or lessons
with their own entry state and success check. Do not substitute a persuasive
thesis for the capability simply because staging calls the field `belief`.

## Learning contract

Add `learning_contract: learning-contract.md` to `staging.yaml`. Write that file
beside the script/deck. It carries task semantics; screen layout belongs to design.
Include:

- **Outcome and success:** given which starting state, the learner will do what,
  producing which observable result. Include a check that distinguishes success
  from a plausible wrong result.
- **Prerequisites:** versions, supplied fixture/data, access, assumed concepts,
  setup check and reset/cleanup. Use a safe local or sandbox fixture where possible.
  Label synthetic examples; never present staged failures as observed incidents.
- **Demonstration:** ordered actions, the reason for each consequential choice,
  expected state after each checkpoint, and one likely failure with recovery.
  Execute the path in the stated environment; record actual commands/UI state and
  results. Unavailable execution is not-yet-checked, not a successful demonstration.
- **Practice and feedback:** for each practice slot, its ID, starting state,
  exact prompt, attempt time or pause/resume instruction, success check, and the
  following feedback slot. Explain why the result is right and how to recover
  from the likely wrong result. Do not reveal the solution before the attempt.
- **Transfer:** change an input, constraint or failure mode so copying the worked
  example is insufficient. Record the expected result and reasoning separately
  from the learner-facing prompt. All necessary rules must have been taught.
- **Timing and evidence:** presentation/speech, demonstration/inspection and
  learner-work budgets; versioned execution evidence and review status. Distinguish
  an agent attempt from a human learner attempt and both from a presenter demo.

An exercise must be answerable using the lesson and declared prerequisites.
If the same person authored and solved it, call that an author check. Use the
shared fresh-context review to find missing prerequisites and unsupported inferences;
it does not certify population-level teaching effectiveness.

## Arc, slot sequence and transitions

Start with **outcome → ready state → worked example → attempt → feedback →
transfer → usable next task**. Adapt the number of checkpoints to the task.

| Beat | Functions | Transition that earns the next beat |
|---|---|---|
| Outcome | `cover`, optionally `statement` | Show the useful end state and what the learner will be able to check. |
| Ready state | `intro` after the cover, or `mechanism` when setup is part of the task | Confirm the starting state before the first dependent action. |
| Worked example | `mechanism`, `evidence` | Demonstrate the rule and its observable result; ask what happens with a changed input. |
| Attempt | `practice` | Give the prompt and stop for an attempt before any solution appears. |
| Feedback | `evidence` or `mechanism` | Compare result to success criteria, explain the cause, then repair the likely error. |
| Transfer | `practice` followed by `evidence`/`mechanism` | Apply the same principle to a new case and explain why the result differs or stays the same. |
| Next task | optional `ask`, then `end` | Consolidate the decision rule and point to a feasible independent use and reference. |

The same example and task state persist through explanation and checking. A
practice/feedback pair may interrupt a mechanism at a recoverable checkpoint.
Do not interleave unrelated material. No `pivot`, purchase decision or named/dated
sales ask is required. A synthesis is useful when it lets the learner retrieve the
rule or compare cases; a list of everything already said is not sufficient.

## Bold moment

Choose one observable payoff: the task succeeds, the deliberate mistake is detected,
or a changed input exposes the governing rule. Place it where the explanation has
equipped the learner to recognize its significance, commonly at worked-example
verification or transfer feedback. Name the event and its supporting evidence.

Derive its position from the actual slot/timing plan, including demonstration
time; do not borrow another genre's percentage window. A correct result can be
quiet. No dramatic effect or kinetic peak is required merely because a tutorial
has a bold moment. The motion governor still applies to any chosen motion.

## Deck/workshop lane

Budget the whole session, including setup, attempts, recovery and discussion.
Example for a 25-minute workshop: outcome 3, ready state 2, model 5, attempt 5,
feedback 3, transfer with feedback 5, close 2. This is a starting allocation, not
a universal ratio. Reduce scope when setup or learner work needs more time.

State what learners do during each exercise, what the facilitator watches for,
and when/how feedback is released. Keep solutions in presenter notes or a later
surface until the attempt is complete. Provide a recoverable checkpoint if a
learner falls behind. A read-alone deck must include setup, prompts and reachable
answers; a live-only deck can use facilitator notes for the same material.

## Video lane

Use chapters at task boundaries with an identifiable starting state, exact actions
and a visible success check. Show the relevant screen/code/output at readable size;
keep the cursor, changed value and resulting state easy to follow. Introduce a
control before using it. Cut dead waiting only when the elapsed time and resulting
state remain honest. Do not simulate success or splice incompatible states.

Account for three clocks:

1. **Speech:** current word count / declared speaking rate, plus the usual 10%
   slower-take check.
2. **Encoded demonstration/inspection:** time needed to read or inspect the result,
   including silent holds if supported by the actual production path. Do not count
   a second twice when inspection intentionally overlaps intelligible speech.
3. **Learner work:** explicit pause-and-try time, outside encoded runtime, stated
   separately so a six-minute film is not advertised as a six-minute completion.

For example, a six-minute encoded budget might allocate 20/40/80/20/65/105/30
seconds to the seven beats above (transfer includes feedback). If 40 seconds are
reserved for non-overlapping inspection, 640 words at 140 wpm take about 274
seconds, or 302 seconds at 10% longer duration; adding 40 gives about 342 seconds.
The remaining time is margin. Two three-minute learner pauses add six minutes to
the learning experience, not the file. Recalculate from the actual script.

The current engine measures TTS and overwrites scene durations. A script estimate
or motion role cannot create a silent hold. Record any hold implementation and
its production owner; keep timing pending until checked on the measured take.
Where holds cannot be delivered, use explicit viewer-controlled pause cues or
revise scope; never report estimated silence as rendered. This does not authorize
running TTS/render beyond `video-author`'s existing boundary.

An attempt screen says what to do, how to check it and when to resume. Keep the
answer out of that screen and its captions; put it in the following feedback scene.
Provide copyable commands/fixtures in the companion when needed, plus captions
and a text alternative. Short on-screen label conventions do not justify hiding
the exact code or state the learner must inspect.

## Opening, closing and failure modes

Open on the attainable result, its use and the relevant starting assumption.
Avoid a logo introduction or unexplained screen activity. Close on the rule that
made the task work, a consequential boundary and a new task the learner can now
attempt. Avoid claiming mastery because the presenter completed the happy path.

Route back to scope or another genre when the task has no observable success
criterion, the prerequisites consume the available session, the learner cannot
access a safe environment, or a crucial dependency cannot be demonstrated honestly.
Do not quietly proceed as if the missing evidence were supplied. A conceptual
explanation can still be produced if its narrower promise is stated.

Common repairs: unexplained clicks → state the decision before acting; no recovery
path → show diagnosis and reset; solution visible during practice → move it to
feedback; same-input copying → add changed-case transfer; rapid result flash →
budget inspection; "try it" without a check → define observable success.

## Gates and handoff

Run staging's shared gates and the video editorial procedure when applicable.
Use the following tutorial-specific evidence, with pass/revise/not-yet-checked:

| Gate | Evidence | Returns to |
|---|---|---|
| Reproducible task | Recorded starting state, actual run and observable result; likely failure and recovery | Setup/demo |
| Attempt before answer | Prompt, attempt opportunity and adjacent feedback mapped by slot ID | Slot sequence |
| Transfer | Fresh case, expected reasoning and cited taught rule; independent attempt when available | Explanation/practice |
| Time to learn | Separate clocks, arithmetic from current words, inspected output and measurable final timing | Scope/timing |
| Completion honesty | Author, agent and human evidence distinguished; unrun/media checks pending with an owner | Review record |

Hand off `staging.yaml`, `learning-contract.md`, script or deck, and the existing
review record. Linter success is only its limited content checks; it does not
validate exercises, learner ability or actual media quality.
