### Fixed
- On GitLab, `deskevidence` no longer refuses every landing (`--dry-run` included) with exit 6 as "an entry the forge did not type". The GitLab backend now types a file read itself: content that hashes to the served `blob_id`, with a regular or executable tree entry for that blob, is a plain file. Symbolic links and submodules still refuse, and so does a read the backend cannot type.
