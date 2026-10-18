package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2972
func TestListPartialOutput(t *testing.T) {
	home := deskHome(t)
	dir := filepath.Join(home, ".config", "assay", "claims")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"good.claim": `{"item":"good","owner":"reader"}`, "bad.claim": `{broken`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	out := captureStdout(t, func() { err = cmdList(nil) })
	if !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "bad.claim") {
		t.Fatalf("partial listing reported clean: %v", err)
	}
	if !strings.Contains(out, "good") || !strings.Contains(out, "reader") || strings.Contains(out, "(no claims)") {
		t.Fatalf("readable rows lost or falsely empty: %q", out)
	}
	if detail := lastAuditDetail(t, home, "list"); !strings.Contains(detail, "bad.claim") {
		t.Fatalf("partial audit omitted cause: %q", detail)
	}
}
