### Fixed
- The GitHub forge's file read now reports a path as a regular file only when its tree entry's mode is a regular-file mode (100644 or 100755) and the entry's blob is the one served; a symbolic link whose target text equals its own bytes is now reported as a link, and the file write's pre-read refuses it (#3149).
