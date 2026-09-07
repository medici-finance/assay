package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCanonicalToolKey pins the resolver: exact keys, the variant spellings the live audit
// log carried, the whole-token boundary (so a substring is not a match), the synthetic key's
// distinctness, and the two unknown shapes.
func TestCanonicalToolKey(t *testing.T) {
	cases := []struct {
		raw   string
		want  string
		known bool
	}{
		// Exact canonical keys — a caller's constant or a correctly named binary.
		{"deskpost", "deskpost", true},
		{"deskpr", "deskpr", true},
		{"deskpreflight", "deskpreflight", true}, // NOT deskpr — "deskpr" is a substring, not a token
		// Variant spellings observed in the live log — all collapse to their tool.
		{"deskpost.test", "deskpost", true},
		{"vfy713-deskpost", "deskpost", true},
		{"deskpr-322", "deskpr", true},
		{"deskpr-bin", "deskpr", true},
		{"deskreply-bin", "deskreply", true},
		{"vd-dt07-deskreply", "deskreply", true},
		{"deskboard-v4test", "deskboard", true},
		{"deskwt.test", "deskwt", true},
		{"desktoken-test", "desktoken", true},
		{"desktoken-bin", "desktoken", true},
		// The synthetic verdict-issue key is its OWN bucket, matched only exactly — it must
		// never collapse into "verifyloop" (a deliberately separate budget, ratelimit.go).
		{VerdictIssueTool, VerdictIssueTool, true},
		// Unknown: an abbreviation that is no tool's token, and empty.
		{"dt", "", false},
		{"", "", false},
		// Ambiguous: two DIFFERENT canonical tokens is not a guess.
		{"deskpost-deskpr", "", false},
	}
	for _, c := range cases {
		got, known := CanonicalToolKey(c.raw)
		if got != c.want || known != c.known {
			t.Errorf("CanonicalToolKey(%q) = (%q,%v), want (%q,%v)", c.raw, got, known, c.want, c.known)
		}
	}
}

// TestVariantKeysShareOneBudget is the load-bearing fail-first test for hole A: entries
// written under a VARIANT spelling of a tool count against that tool's ONE budget, so a
// renamed/copied/test-built binary can no longer earn a fresh, uncounted budget.
//
// Before the fix, pointsFor matched `e.Tool == tool` exactly, so the variant lines below
// were invisible to "deskpost"'s meter and the write was admitted.
func TestVariantKeysShareOneBudget(t *testing.T) {
	dir := setup(t)
	// Fill the whole per-PR budget using ONLY the variant spelling "deskpost.test".
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost.test", Verb: "comment", Result: ResultOK})
	}
	// The canonical tool must now be OVER budget: the variant lines are its own.
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Fatalf("variant-key writes did not count against the canonical budget: meter=%q (want pr-budget)", got)
	}
	// And symmetrically, a caller passing the variant spelling is metered against the same
	// canonical history — it cannot buy headroom by renaming itself either.
	if got := meterOf(AllowWrite("vfy713-deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Fatalf("a variant CALLER escaped the canonical budget: meter=%q (want pr-budget)", got)
	}
}

// TestUnregisteredKeyRefusedNotBucketed is the fail-first test for the other half of hole A:
// a key that attributes to no known tool is a LOUD Unverifiable at the write gate, never a
// silent fresh budget. Before the fix, "dt" simply got its own empty bucket and was admitted.
func TestUnregisteredKeyRefusedNotBucketed(t *testing.T) {
	setup(t)
	err := AllowWrite("dt", testRepo, testPR)
	if !IsUnverifiable(err) {
		t.Fatalf("unregistered tool key was not refused: err=%v (want Unverifiable)", err)
	}
	if err := AllowWriteRepoWide("dt", testRepo); !IsUnverifiable(err) {
		t.Fatalf("unregistered tool key not refused on the repo-wide gate: err=%v", err)
	}
}

// TestRegistryCoversCmdBinaries is the drift guard: every binary under tools/desk/cmd/ must be
// a registered canonical key, or that tool would earn a loud Unverifiable at its first write.
// Compiled-in roster + this diff is the loopnames.go pattern — a new tool that forgets to
// register here goes red rather than breaking in production.
func TestRegistryCoversCmdBinaries(t *testing.T) {
	// This test file sits in tools/desk/internal/deskkit; cmd/ is two levels up.
	entries, err := os.ReadDir(filepath.Join("..", "..", "cmd"))
	if err != nil {
		t.Fatalf("read cmd dir: %v", err)
	}
	known := map[string]struct{}{}
	for _, k := range KnownToolKeys() {
		known[k] = struct{}{}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := known[e.Name()]; !ok {
			t.Errorf("cmd binary %q is not registered in canonicalToolKeys (audittoolkey.go) — its writes would be a loud Unverifiable", e.Name())
		}
	}
}
