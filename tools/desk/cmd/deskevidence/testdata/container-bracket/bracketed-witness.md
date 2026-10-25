# Brief 01

## Verify

| # | Command | Expect |
|---|---|---|
| 1 | `go test -run '^TestA$\|^TestA2$' -count=1 .` | exit 0 |
| 2 | `go test -run '^TestB$' -count=1 .` | exit 0 |

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go test -run '^TestA$\|^TestA2$' -count=1 .` | pass exit=0 sandbox=container-netns | sha256:0a1b2c3d4e5f | 2026-10-22 | example-verifier-app[bot] @ 0123abc (forge-identity) |
| 2 | `go test -run '^TestB$' -count=1 .` | pass exit=0 sandbox=container-netns | sha256:5f4e3d2c1b0a | 2026-10-22 | example-verifier-app[bot] @ 0123abc (git-config) |
Tree-Digest: sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
Container-Image: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
Verification-Attestation: example-org/example-repo#7 run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa source=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb brief=docs/streams/example/brief-01-stamped.md model=example-model tier=strong
Admission-Bracket: run=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa source=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb tree=sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd witness=sha256:28a9fac4fa3b4f6df3a80fcd396ad0803c029705f9970e640c3c4693e8becc37 opened=2026-10-22T10:00:00Z closed=2026-10-22T10:05:00Z before=receipt after=check-verifier
