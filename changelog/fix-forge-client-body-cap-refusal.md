### Fixed
- The `deskfleet` forge client and the label transport `cellctl` uses now refuse an answer larger than their 4 MiB bound as could-not-check (exit 6), naming only the method and path. Before, they parsed the first 4 MiB as if it were the whole answer, so a list cut off after `[]` could read as complete and empty.
- `deskapps`' manifest conversion read does the same over 1 MiB. It no longer parses the first 1 MiB of an oversized answer.
- The `deskevidence` decision-gate check now fits its post-kill pipe drain inside each child's half of the write-window bound. Before, a child holding its pipes could run 2s past that half, so the before and after pair could hold the preparation lock past the bound.
