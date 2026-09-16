package main

import (
	"regexp"
	"testing"
)

// TestInitAssayVersions_BareStatusgenLine — the scaffolded pin file carries the
// bare `statusgen <tag> <sha256>` line the desk tools read first, ALONGSIDE the
// per-platform lines CI selects by (`grep '^statusgen-<platform> '`), so a fresh
// adopter's file satisfies both readers. Both stay placeholders: no live digest.
func TestInitAssayVersions_BareStatusgenLine(t *testing.T) {
	bare := regexp.MustCompile(`(?m)^statusgen[ \t]+REPLACE_WITH_TAG[ \t]+REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS[ \t]*$`)
	if !bare.MatchString(initAssayVersions) {
		t.Errorf("init pin file lacks the bare statusgen placeholder line:\n%s", initAssayVersions)
	}
	for _, plat := range []string{"statusgen-darwin-arm64", "statusgen-darwin-amd64", "statusgen-linux-amd64"} {
		if !regexp.MustCompile(`(?m)^` + plat + `[ \t]+REPLACE_WITH_TAG`).MatchString(initAssayVersions) {
			t.Errorf("init pin file lost the %s platform line", plat)
		}
	}
	if regexp.MustCompile(`[0-9a-f]{64}`).MatchString(initAssayVersions) {
		t.Error("init pin file must not ship a live digest")
	}
}
