### Changed
- The Compose desk services, `desk-run.sh` and the documented `docker run` recipes now set `GODEBUG=multipathtcp=0` themselves, so an older image tag or an operator env-file cannot leave Go MPTCP listeners open (CVE-2026-98432 stopgap; a patched node kernel remains the real fix).
- Tests now hold that setting on every Compose service and refuse a strategic-merge directive or a non-literal `GODEBUG` entry in a kustomize patch, and the `GODEBUG` matcher reads the list exactly as the Go runtime does (no whitespace trimming).
