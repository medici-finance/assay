package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeOutcomeRecord commits one verify-outcome record under root at the path
// deskkit.RecordName derives from line's bytes — the per-file layout #882 introduces.
func writeOutcomeRecord(t *testing.T, root, line string) {
	t.Helper()
	name, err := deskkit.RecordName([]byte(line))
	if err != nil {
		t.Fatalf("RecordName(%q): %v", line, err)
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFileAt(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestOutcomeRecords is Verify row 2: one fixture tree's outcome data, read three ways (legacy
// log only, per-file records only, both together), must classify identically — same stuck-flip
// bucket, same wake state, for every brief. #882 retires the position-dependent single-file
// read; this pins that rewiring WHERE the data lives changed nothing about WHAT it means.
func TestOutcomeRecords(t *testing.T) {
	clear := planBrief("model", "no", "no", "no", "no")
	briefs := map[string]string{
		"01": filledEvidence(clear), // outcome verified + Evidence filled -> stuck-flip
		"02": filledEvidence(clear), // outcome verify-fail -> stays dispatchable
	}
	line01 := `{"ts":"2026-09-08T02:00:00Z","brief":"example-stream/01","outcome":"verified","sha":"0000002"}`
	line02 := `{"ts":"2026-09-08T03:00:00Z","brief":"example-stream/02","outcome":"verify-fail","sha":"0000003"}`

	runVariant := func(t *testing.T, place func(root string)) string {
		root := planFixtureRoot(t, briefs)
		place(root)
		var perr error
		out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
		if perr != nil {
			t.Fatalf("cmdPlan: %v", perr)
		}
		return out
	}

	var logOnly, recordsOnly, both string
	t.Run("log only", func(t *testing.T) {
		logOnly = runVariant(t, func(root string) {
			writeFileAt(t, root, "docs/streams/verify-outcomes.jsonl", line01+"\n"+line02+"\n")
		})
	})
	t.Run("records only", func(t *testing.T) {
		recordsOnly = runVariant(t, func(root string) {
			writeOutcomeRecord(t, root, line01)
			writeOutcomeRecord(t, root, line02)
		})
	})
	t.Run("both", func(t *testing.T) {
		both = runVariant(t, func(root string) {
			writeFileAt(t, root, "docs/streams/verify-outcomes.jsonl", line01+"\n"+line02+"\n")
			writeOutcomeRecord(t, root, line01)
			writeOutcomeRecord(t, root, line02)
		})
	})

	if logOnly != recordsOnly {
		t.Fatalf("log-only and records-only plans differ:\n--- log-only ---\n%s\n--- records-only ---\n%s", logOnly, recordsOnly)
	}
	if logOnly != both {
		t.Fatalf("log-only and both-layouts plans differ (a record present in both layouts must count once):\n--- log-only ---\n%s\n--- both ---\n%s", logOnly, both)
	}
}
