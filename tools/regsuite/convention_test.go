package main

import (
	"os"
	"strings"
	"testing"
)

// TestConvention_DocMatchesConstant is Verify row 9: a drift guard between
// docs/test-policy.md's "## Regression suite" section and this tool's own
// constants. If either changes without the other, this test — not a human
// re-reading two files — catches it.
func TestConvention_DocMatchesConstant(t *testing.T) {
	const docPath = "../../docs/test-policy.md"
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docPath, err)
	}
	doc := string(raw)

	if !strings.Contains(doc, "## Regression suite") {
		t.Fatalf("%s does not carry a \"## Regression suite\" section", docPath)
	}
	if !strings.Contains(doc, RegressionTestPrefix) {
		t.Errorf("%s does not state the prefix constant %q", docPath, RegressionTestPrefix)
	}
	if !strings.Contains(doc, regressionNameShapePattern) {
		t.Errorf("%s does not state the shape regex %q byte-for-byte", docPath, regressionNameShapePattern)
	}
}
