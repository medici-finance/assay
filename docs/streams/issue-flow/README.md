# Issue-flow registers

This directory is a register, not a stream — stream discovery skips it. It holds the
rulings register (`rulings.md`): one section per recorded ruling, each carrying its text
above a `**Sign-off:**` line that names the ratifying artifact.

The register is machine-read. The auto-approve lane's enactment gate (`deskautolane`,
R-8 in `rulings.md`) reads the R-8 section's Sign-off line through the forge at this
repository's default branch, so a Sign-off line in that file is load-bearing tool input,
never prose alone. The lane's own never-admit set (`docs/streams/**/rulings.md`) keeps
any lane merge from editing the line that enacts the lane.

A ruling section's text above its Sign-off line is stable once signed: the enactment
gate counts a later edit to it as superseding an earlier acceptance. Record a new ruling
or a change to one as a new commit; never reword a signed section.
