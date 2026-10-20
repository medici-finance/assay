package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"testing"
)

// regression: class issue-projection
func TestPerfEntrypoint(t *testing.T) {
	raw := os.Getenv("ASSAY_PERF_TEST_ARGS")
	if raw == "" {
		t.Skip("entrypoint subprocess only")
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", os.Getenv("ASSAY_PERF_TEST_HOME"))
	deskkit.ReloadConfig()
	if os.Getenv("ASSAY_PERF_TEST_WORK") == "1" {
		withForge(t, map[string]*fakeForge{"example-org/tracker": {issues: []deskkit.IssueSummary{{Number: 1, Title: "private-title", Labels: []string{"urgent"}}}}})
	}
	os.Args = append([]string{"deskinbox"}, args...)
	main()
}
