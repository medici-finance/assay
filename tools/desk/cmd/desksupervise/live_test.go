package main

import (
	"os"
	"strings"
	"testing"
)

// live_test.go — the offline tests behind #1197: readLiveClaims' ref listing must be
// AUTHENTICATED and FORGE-RESOLVED, never an anonymous read against a hardcoded github.com
// (which 404s on every private board root as "authentication required: Repository not
// found" and makes the whole stop/blocked-timeout/reclaim instrument could-not-check).

// TestLiveClaimListNeverHardcodesGitHubHost is the source-level guard: live.go must not
// carry a `https://github.com/` literal. The host is the forge resolver's answer
// (deskkit.ForgeKindFromSlugAndHost, fed the --root checkout's origin host), the same way
// cmd/deskclaim-ref's newForgeStore builds its URL — a compiled-in SaaS host is exactly the
// self-hosted-adopter failure #727 retired from the claim layer.
func TestLiveClaimListNeverHardcodesGitHubHost(t *testing.T) {
	src, err := os.ReadFile("live.go")
	if err != nil {
		t.Fatalf("read live.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `"https://github.com/`) {
			t.Fatalf("live.go:%d hardcodes the GitHub host in a git URL: %s\n"+
				"— the host must come from the forge resolver (deskkit.ForgeKindFromSlugAndHost), never a literal", i+1, strings.TrimSpace(line))
		}
	}
}
