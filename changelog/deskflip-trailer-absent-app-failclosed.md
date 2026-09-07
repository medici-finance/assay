### Fixed
- **A trailer-less App-authored PR now fails closed at the flip gate.** A PR carrying no
  `Brief:` / `Issue:` link trailer used to be risk-classed by its path, surface-label and
  visibility terms alone, so a risk-bearing change with no trailer could be marked FLIP with no
  `Security-Review` verdict. `deskpr` makes the trailer MANDATORY for App-authored PRs, so a
  trailer-less PR whose author is a role App (worker / desk / verifier / reviewer, per the
  roster) is an anomaly by construction. `deskflip` and `deskboard` now treat it as
  RiskClassed + Unverifiable: the flip REFUSES until a `Security-Review: pass` stands at head,
  and the board row says why (`trailer absent on App-authored PR`). A trailer-less
  HUMAN-authored PR keeps today's path / label / visibility behaviour — no new cost on
  maintainer PRs. The author test reuses the roster's own App-slug resolution (never a
  hard-coded login). (#587)
