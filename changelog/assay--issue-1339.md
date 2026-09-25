### Added
- `deskpr` accepts a third link trailer, `Authors: <stream>/<NN>[, …]`, for a PR that only authors briefs. It names the briefs the PR writes without claiming to deliver them, so the dispatcher, `fanoutloop plan` and the derived board no longer treat a merged authoring PR as the brief's delivery. Every entry must resolve to a brief file under `--root`.

### Changed
- `deskpr create` refuses a `Brief:` line on a branch that only authors that brief: the branch adds the brief's file and touches only stream board READMEs, brief files and changelog fragments. The refusal names the `Authors:` line to use. A PR that authors a brief and also delivers work keeps `Brief:`.
- `deskpr create` also refuses the mirror case: an `Authors:` line on a branch whose diff is NOT provably authoring-only for every listed id (including a rename, which the writer-side gate cannot classify and so refuses rather than trusts). `deskflip`'s security lane carries the binding half of the same check (`deskkit.AuthorsRiskFromBody`), so an `Authors:` PR that actually delivers code or a document for a `gate: human` / `risk: yes` brief is still risk-classed at flip time even if the writer-side gate was bypassed or predates it.

### Fixed
- `fanoutloop plan` no longer lists a `todo` brief as LANDED-UNRECONCILED because the merged docs-only PR that wrote it carried `Brief:`. `deskdispatch` already applied this check. The planner now reads the changed files of each PR naming a queued brief and ignores a PR that only authored it. A PR whose file list cannot be read still counts as delivering the brief.
