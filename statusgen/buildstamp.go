package main

// statusgenVersion is the release tag this binary was built from, stamped at
// link time by the release workflow:
//
//	go build -ldflags "-X main.statusgenVersion=statusgen/v0.3.0"
//
// It defaults to "dev" for a local `go build`/`go run`, which is the honest
// answer: an unstamped binary is not a pinned release.
//
// It lives in its own file so the hotfix ineligible list
// (tools/desk/internal/deskkit/version.go) can name it: the layer-2 self-check
// reads it, so a statusgen hotfix may never rewrite it.
//
// This exists so a consumer can CHECK the pin rather than assume it. Consumers
// pin statusgen by tag + sha256 in `.assay-versions`; until now nothing could
// ask a running binary which release it is, so a stale install produced a stale
// board with no signal.
var statusgenVersion = "dev"
