### Added
- Verify rows can name their command explicitly: in a Command cell that mixes prose and code spans, a code span starting `cmd:` (for example `` `cmd: go test ./pkg/ -count=1` ``) is the command. `statusgen verifyrun`, the check:ci re-execution lane, `newbrief` and the row lint all take the first `cmd:` span. A cell without the marker still uses its first code span, so no existing row changes command or verdict.
- `statusgen --lint` NOTICEs a prose Command cell whose first code span is a mention rather than a command, such as a function name, a file, an `owner/repo`, or a word ahead of the real command (`prose-led-command`). It also NOTICEs a cell that carries two `cmd:` markers (`cmd-marker-ambiguous`). Both are advisory, so main stays green.

### Fixed
- The check:ci verdict re-execution lane now runs the command lifted from the Verify cell, not the raw cell text. Before, the shell read the cell's backticks as command substitution.
