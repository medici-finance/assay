### Added
- `statusgen brief <stream/NN>` resolves a brief item key to its file path, parsed frontmatter, and board-row status as JSON (or `--text`) — read-only, reusing the same parsers `--lint` runs. Handles not-found and ambiguous keys explicitly (a duplicate or missing `brief-NN-*` exits non-zero and names what it found, with no JSON body), and accepts multiple keys in one call.
