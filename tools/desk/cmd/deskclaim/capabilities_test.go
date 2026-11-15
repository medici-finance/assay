package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #4947
func TestDeskCapsPreflight(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  string
		want int
	}{
		"malformed":   {"---\nschema: brief-v1\n", deskkit.ExitUnverifiable},
		"unsupported": {"---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n---\n", deskkit.ExitRefused},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			record := filepath.Join(root, "docs", "streams", "sample", "brief-01-example.md")
			if err := os.MkdirAll(filepath.Dir(record), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(record, []byte(tc.raw), 0644); err != nil {
				t.Fatal(err)
			}
			home := deskHome(t)
			capture, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			old := os.Stderr
			os.Stderr = capture
			t.Cleanup(func() { os.Stderr = old; capture.Close() })
			if code := run([]string{"acquire", "--kind", "dispatch", "--item", "sample/01", "--root", root}); code != tc.want {
				t.Fatalf("preflight exit=%d, want %d", code, tc.want)
			}
			raw, err := os.ReadFile(capture.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "reader capabilities:") || !strings.Contains(string(raw), record) {
				t.Fatalf("CLI bypassed capability refusal: %s", raw)
			}
			if _, err := os.Stat(claimFile(home, "sample/01")); !os.IsNotExist(err) {
				t.Fatalf("preflight created a claim: %v", err)
			}
		})
	}
}
