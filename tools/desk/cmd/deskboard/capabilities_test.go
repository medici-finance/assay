package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #4531
func TestDeskCapsPreflight(t *testing.T) {
	r := cliRecord(t)
	for _, form := range []string{"--version", "version"} {
		n, out, err := r.exec(form, "--json")
		if n != 0 || err != "" {
			t.Fatalf("offline discovery: %d %s", n, err)
		}
		var v struct {
			ReaderCapabilities readercaps.Advertisement `json:"readerCapabilities"`
		}
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatal(err)
		}
		if err := v.ReaderCapabilities.Validate(); err != nil {
			t.Fatal(err)
		}
		if r.starts+r.guards+r.calls+r.reads != 0 {
			t.Fatalf("discovery reached execution: %+v", r)
		}
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sample")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("---\nstream: sample\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: dispatch, schemas: [], semantics: [milestone-v1]}]}\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Bypass the CLI/startup/outer guard. The real consuming boundary must
	// still refuse before boardDispatch (and its claim/forge/output effects).
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	var stdout, stderr bytes.Buffer
	if code := runBoard("dispatch", nil, nil, boardOptions{out: filepath.Join(root, "effect.json")}, &stdout, &stderr); code != 5 || r.calls != 0 {
		t.Fatalf("lower boundary admitted effects: code=%d calls=%d %s", code, r.calls, &stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "effect.json")); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote output: %v", err)
	}
	// A clean CWD cannot clear incompatible configured roots. Bypass startup
	// entirely and enter the actual root consumer before reader resolution.
	plantFixtureRoster(t, os.Getenv("HOME"))
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+root)
	if _, err := resolveRootsOnce(); err == nil || !deskkit.IsRefused(err) || !strings.Contains(err.Error(), "unsupported required") {
		t.Fatalf("configured root bypassed negotiation: %v", err)
	}
}
