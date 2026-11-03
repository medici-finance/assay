---
stream: vgc
status: active
priority: P0
track: platform
---

# VGC (verify-gate body-cap fixtures)

Kept apart from testdata/verifygate because the verify-issues selection tests
pin that tree's exact row set. The oversize test inflates each brief's
`<!-- oversize: … -->` line past the forge's issue-body limit at test time, so
no 64 KiB file is committed.

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Verified, oversize Verify + Evidence](./brief-01-verified-oversize.md) | 0 | M | verified | 2026-10-21 rev | — |
| 02 | [Implemented with pass, oversize Evidence](./brief-02-implemented-oversize.md) | 0 | M | implemented | — | — |
