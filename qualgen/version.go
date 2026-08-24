package main

// qualgenVersion is the release tag, stamped at link time by the release
// pipeline (the same pinned-binary mechanism statusgen uses, `.assay-versions`):
//
//	go build -ldflags "-X main.qualgenVersion=qualgen/v0.1.0"
//
// An UNSTAMPED build answers "dev" rather than inventing a release number, so a
// consumer pinning qualgen by tag can tell a stale install from a current one
// (a stale install must never be indistinguishable from a fresh one).
var qualgenVersion = "dev"
