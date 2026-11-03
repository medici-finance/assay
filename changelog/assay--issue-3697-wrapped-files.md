### Fixed

- The brief Context `files:` parser now reads a bullet whose description wraps onto deeper-indented lines, such as a continuation that starts with "(". It reads it the same way as the bullet joined onto one line. A continuation line that reads as declarations still declares, and every other wrapped shape is still refused (#3697).
