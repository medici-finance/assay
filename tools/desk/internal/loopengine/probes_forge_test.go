package loopengine

import (
	"os"
	"strings"
	"testing"
)

// probes_forge_test.go — the offline tests behind #1197 for the production BranchLister:
// houseBranchLister must list AUTHENTICATED against the forge the resolver names, never an
// anonymous read against a hardcoded github.com (which 404s on a private repo, so on a
// private board root a dispatched worker's liveness was seen only through the PR probe).

// TestHouseBranchListerNeverHardcodesGitHubHost is the source-level guard: probes.go must
// not carry a `https://github.com/` literal in a git URL. The host is the forge resolver's
// answer (deskkit.ForgeKindFromSlugAndHost), the same way cmd/deskclaim-ref's newForgeStore
// builds its URL — a compiled-in SaaS host is the self-hosted-adopter failure #727 retired.
func TestHouseBranchListerNeverHardcodesGitHubHost(t *testing.T) {
	src, err := os.ReadFile("probes.go")
	if err != nil {
		t.Fatalf("read probes.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `"https://github.com/`) {
			t.Fatalf("probes.go:%d hardcodes the GitHub host in a git URL: %s\n"+
				"— the host must come from the forge resolver (deskkit.ForgeKindFromSlugAndHost), never a literal", i+1, strings.TrimSpace(line))
		}
	}
}
