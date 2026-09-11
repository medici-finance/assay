### Fixed
- `deskevidence --brief-path` now secret-scans only the Evidence bytes it is adding, not the whole merged brief — a secret-shaped run already on the branch (a frontmatter `id:`, a Verify row's literal command, a fingerprint in prose) could permanently block every future Evidence append to that file (assay-toolkit#2447, #2449, #2452).
