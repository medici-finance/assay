### Added
- `assay-inbox.sh --walk` classifies every item before asking, and puts only `genuine`
  items to the driver. The other classes are entered only on positive evidence from a
  trusted identity. `already-ruled`: after the newest ask (the last comment by a roster
  App, else the body's own Options), the ratifying identity (`ASSAY_BLESS_LOGIN`, and no
  other human) has posted an unedited ruling that names an offered option or ratifies. A
  question, a refusal, a "hold", or a bare desk relay is never a ruling. `no-fork`: the
  issue has a trusted author and its Options section parses to exactly one entry (with no
  Options section, a fork-test block with one `option:` line counts). `reversible-default`:
  the issue has a trusted author and carries a live `caught-by:`, a `default:`, no
  `class:` line and no one-way term.
- "Question k of n" now counts genuine items only, and nothing screened out is silently
  dropped. Every `--walk` question prints a tail line of counts. `--walk --screened` lists
  every screened item with its evidence: the ruling's date and author, or the parsed
  default and its gate. The table and `--html` renderings carry a `class` on every row and
  hide nothing. `--no-screen` turns the screen off in one flag and restores the pre-screen
  output.
- The screen's roster comes from the environment, else from the owner-only
  `${ASSAY_CONFIG_HOME:-~/.config/assay}/roster.env`, and never from the current
  directory. Lists split and compare like the Go roster reader. With no roster, no class is
  entered and a NOTICE names the classes that were switched off. An item the screen cannot
  read is always genuine.
- The table and `--html` now make one `gh issue view` per item, because the screen must
  read an item to classify it; a failed detail fetch makes them exit `2`. `--no-screen`
  restores the old cost.
- The `ask-decision` skill states the four classes and what the desk does with each before
  its five-part format, which now applies to every GENUINE item. The skill walks with the
  bash oracle, the only renderer that runs the screen. Under `deskinbox walk`, which does
  not classify yet, the skill's two floors ("never ask twice", "never ask what the desk can
  answer") stay manual checks.
