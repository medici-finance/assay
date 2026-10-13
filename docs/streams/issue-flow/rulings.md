# Rulings register

One section per recorded ruling: the ruling's text, then its `**Sign-off:**` line naming
the ratifying artifact. A section's text above its Sign-off line is byte-stable once
signed — `deskautolane`'s enactment gate reads R-8's Sign-off line through the forge at
this repository's default branch and counts any later edit to the text above it as
superseding an earlier acceptance. The Sign-off line of a section that is not enacted
ships EMPTY: an empty line reads as deliberately unsigned and grants nothing.

## R-8 — the auto-approve lane's enactment gate (ships unsigned)

The auto-approve lane (`deskautolane`) ships inert, and this section is the instrument
that would enact it: every write the lane can make is refused until this section's
Sign-off line names one comment — on the one configured sign-off thread, authored by the
roster-pinned blessing authority, whose body opens with the bare line `Enact: R-8` and
carries no word from the rejection lexicon. The Sign-off line below ships EMPTY: the
lane is enacted for no area, and enacting it remains the driver's own act under the
gate's own discipline (a recorded text change above this line supersedes an earlier
acceptance; the newest acceptance on the thread is the one that counts).

Recorded under this gate: the driver's ruling on the scheduled board reconcile's pull
request class — ruled 2026-10-11 (desk-relayed on #2688), ratified by the driver's own
comment on #2688 at 2026-10-12T01:50:46Z. The class — pull requests titled
`chore(board): reconcile` opened by app/assay-board-writer, carrying Status-cell flips
each witnessed by a merged pull request bearing the brief's `Brief:` trailer, plus
re-rendering of the generated tables — is generated output, not reviewable content; it
belongs in the auto-approve lane or on a direct-CI single-writer path, with the mechanism
delegated to the implementing brief. The implementing brief (desk-tools/36) chose the
direct-CI single-writer mechanism: the class lands on main the way STATUS.md
regeneration does — the workflow pushes, no pull request — and so the class does not
enter the lane. That choice enacts nothing here: this section's Sign-off line stays
empty, the lane stays closed for every area, and the class's own guards (the reconcile
tool's would-apply self-hold, the merged-PR witness discipline, and branch protection on
main) are untouched by this entry.

**Sign-off:**
