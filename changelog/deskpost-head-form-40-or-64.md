### Changed
- `deskpost review --head` now states the accepted SHA form as "40- (or 64-) character
  lowercase-hex" in its usage error and in `tools/desk/README.md`, matching what
  `isFullSHA` actually accepts (40 for SHA-1, 64 for the SHA-256 object format) rather
  than only naming 40. Behaviour is unchanged; only the guidance text is now accurate.
