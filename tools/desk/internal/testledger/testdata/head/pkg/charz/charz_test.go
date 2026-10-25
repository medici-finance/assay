package charz

import (
	"strings"
	"testing"
)

// TestFixtureTriaged records what Fields does with a run of spaces. Its oracle lists it.
// characterization: pkg/charz docs/investigations/2026-01-20-pkg-charz-oracle.md
func TestFixtureTriaged(t *testing.T) {
	if got := len(strings.Fields("a  b")); got != 2 {
		t.Fatalf("fields: %d", got)
	}
}

// TestFixtureUntriaged records what Title does with an empty string. Its oracle names it
// only outside the triage table, so it has no row and the report lists it.
// characterization: pkg/charz docs/investigations/2026-01-20-pkg-charz-oracle.md
func TestFixtureUntriaged(t *testing.T) {
	if got := strings.ToUpper(""); got != "" {
		t.Fatalf("upper: %q", got)
	}
}

// TestFixtureProse is not a characterization test: the line below wraps prose, it is no tag.
// characterization: of the trimming behaviour, written before the oracle existed
func TestFixtureProse(t *testing.T) {
	if strings.TrimSpace(" p ") != "p" {
		t.Fatal("prose")
	}
}
