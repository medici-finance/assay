### Fixed
- **`.gitattributes` pins the board's inputs to LF, so the Windows CI leg matches Linux.**
  `statusgen --lint` compares `STATUS.md`, `CLAUDE.md` and everything under `docs/streams/**`
  byte-for-byte against a fresh regeneration. A default Windows checkout rewrote those files
  with CRLF, so the `windows-latest` leg (windows-port/04) reddened on `statusgen --lint`
  while Linux passed on the identical tree. A repo `.gitattributes` now forces LF on checkout
  for the board's inputs (`*.md`, `docs/streams/**`) while keeping `.ps1`/`.psm1` on CRLF.
  (#584)
