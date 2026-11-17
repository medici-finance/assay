### Fixed
- `statusgen verifyrun` no longer writes a read-set file through a symlink: the state directory under the git dir must be a real directory, a link at the file's name is refused, and the file is written to a fresh temp file and renamed into place. Each write prunes the state directory to files younger than 30 days and at most the newest 256.
