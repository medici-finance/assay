### Added
- New `deskinbox` verb (`tools/desk/cmd/deskinbox`): a Go port of `assay-inbox.sh`'s
  `table` (default) and `walk` renderings — the `ask-decision` skill's actual entry point
  — with no `bash`/`jq`/`make` dependency, so it builds and runs on native Windows. The
  five-part decision format builder is byte-parity tested against the oracle's own jq
  program, extracted verbatim at test time.
- `plugins/assay/commands/inbox.md` and `plugins/assay/skills/ask-decision/SKILL.md` now
  invoke `deskinbox`/`deskinbox walk` for those two renderings, with the bash oracle kept
  as the documented fallback and as the current renderer for `--html`/`--flow` (split to
  windows-port/15).

### Changed
- `docs/streams/windows-port/brief-13-inbox-verb-port.md` narrowed to the table+walk scope
  actually delivered here, and corrected two factual errors found at pickup: the oracle
  script invokes `make` zero times (not four, as the brief's earlier draft claimed), and
  the skill body that names the script lives at `plugins/assay/commands/inbox.md`, not the
  nonexistent `plugins/assay/skills/inbox/SKILL.md` the earlier draft cited.
- New follow-up brief `docs/streams/windows-port/brief-15-inbox-html-flow-port.md` carries
  the `--html` and `--flow` renderings split off from windows-port/13; windows-port/14 now
  depends on windows-port/15 (not /13) for the `deskinbox flow` step its Windows leg runs.
