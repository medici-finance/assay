# Motion language — choose, test, refine

A design-side reference. Staging supplies semantic intent through
[motion roles](motion-roles.md); design chooses how movement serves that intent.
Use [creative development](creative-development.md) before detailed production.

Begin with a provisional language, test it in a rough sequence, then record the
selected treatment in `motion-spec.md`. Selection gives the film coherence, not
an irreversible lock. Revise when playback exposes a better explanation; record
why and recheck affected scenes and boundaries. Do not silently drift or report
an unaudited intention as the delivered behavior.

## Decisions to record

| Area | Useful questions |
|---|---|
| Visual premise | What carries the explanation? What does the viewer discover? |
| Behavior and easing | What should feel deliberate, uncertain, continuous, abrupt or responsive? Choose curves for those meanings; declare them in implementation. |
| Timing and holds | Where does anticipation help? Where does a reveal need reading time? Which holds absorb a longer take without slowing every action? |
| Framing and continuity | What does a close-up, wide shot, rotation or change of perspective reveal? Where should the eye land across a cut? |
| Attention | What is primary at each beat? Can several moving parts read as one meaningful relationship, or do they compete? |
| Sound and accessibility | What role does sound or silence play? What survives reduced motion? What remains pending on the actual voice/captioned take? |

Record durations and travel in usable units (frame-relative distances are useful
across resolutions). There is no universal number of curves, durations, transitions,
figures or peaks. Anticipation, overshoot, rotation, repetition and exaggeration are
available tools when they clarify the subject and remain readable. Do not distort
quantitative geometry, imply unobserved execution, or present an illustrative
transformation as a measured event. Respect the project's brand and supplied assets.

## Optional preset: restrained technical explainer

When precision and quiet inspection suit the subject, start with clean cuts,
gentle ease-out entrances/ease-in exits, a roughly 0.4-second base move, small
supporting travel and generous reading holds. Avoid bounce where it makes a
serious state change look casual; favor one primary focus. These are starting
choices, not hard limits or a required house look. Select a different language
when the explanation benefits, and test the result rather than counting effects.

## Audit the delivered sequence

Record what helped, what competed and what changed, with scene/time evidence.
Retain actual FAIL → FIXED or unresolved findings; do not invent cuts or failures
to fill a template. A restraint pass can name what stays still and why, alongside
where richer motion earns its place.

Watch for front-loaded choreography followed by accidental dead time, multiple
unrelated focal points, repetitive framing, a reveal arriving before its meaning,
and an attractive move that implies the wrong relationship. Deliberate silence,
stillness and busy coherent action can each be right. Check the actual voice take
for phrase alignment; proportionally scaling estimates is not proof of sync.

All implementation safeguards still apply: self-contained assets, deterministic
seeking/freezing, measured-duration handling, readable type/captions, reduced
motion, all declared seams verified by the supported gate, and render-and-look.
A creative exception never waives a failed mechanical test. For a transition the
current seam verifier cannot assess, retain supported boundaries in the delivery
until appropriate verification exists; see [seam gate](seam-gate.md).
