package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLedger_CreatesFileAndDir(t *testing.T) {
	root := t.TempDir()
	if err := AppendLedger(root, LedgerEntry{
		Component: "assay/labels",
		Version:   "0.28.0",
		Step:      "label-set",
		Kind:      "label",
		ID:        "review-request",
		By:        "assay-desk-app",
	}); err != nil {
		t.Fatalf("AppendLedger: %v", err)
	}
	path := LedgerPath(root)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ledger file not created: %v", err)
	}
	entries, err := ReadLedger(root)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Component != "assay/labels" || e.Kind != "label" || e.ID != "review-request" {
		t.Errorf("entry mismatch: %+v", e)
	}
	if e.Created == "" {
		t.Errorf("Created was not filled in")
	}
}

func TestAppendLedger_AppendOnly(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := AppendLedger(root, LedgerEntry{
			Component: "assay/roster",
			Version:   "0.28.0",
			Step:      "trust-surface",
			Kind:      "roster-vars",
			ID:        "ASSAY_BLESS_LOGIN",
			By:        "human:ian",
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	raw, err := os.ReadFile(LedgerPath(root))
	if err != nil {
		t.Fatalf("reading ledger: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (append-only, never rewritten)", len(lines))
	}
	entries, err := ReadLedger(root)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
}

func TestAppendLedger_MissingFieldRefuses(t *testing.T) {
	root := t.TempDir()
	cases := []LedgerEntry{
		{Version: "0.28.0", Step: "s", Kind: "k", ID: "i"},              // missing component
		{Component: "assay/x", Version: "0.28.0", Kind: "k", ID: "i"},   // missing step
		{Component: "assay/x", Version: "0.28.0", Step: "s", ID: "i"},   // missing kind
		{Component: "assay/x", Version: "0.28.0", Step: "s", Kind: "k"}, // missing id
	}
	for i, e := range cases {
		if err := AppendLedger(root, e); err == nil {
			t.Errorf("case %d: expected refusal for incomplete entry %+v", i, e)
		}
	}
	entries, err := ReadLedger(root)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("a refused write must not land a line; got %d entries", len(entries))
	}
}

func TestReadLedger_MissingFileIsZeroEntriesNotError(t *testing.T) {
	root := t.TempDir()
	entries, err := ReadLedger(root)
	if err != nil {
		t.Fatalf("ReadLedger on a fresh repo must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestReadLedger_MalformedLineErrors(t *testing.T) {
	root := t.TempDir()
	path := LedgerPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLedger(root); err == nil {
		t.Errorf("expected an error reading a malformed ledger line")
	}
}

func TestLedgerForComponent_FiltersAndPreservesOrder(t *testing.T) {
	root := t.TempDir()
	ids := []string{"a", "b", "a", "c", "a"}
	for i, comp := range ids {
		if err := AppendLedger(root, LedgerEntry{
			Component: "assay/" + comp,
			Version:   "0.1.0",
			Step:      "s",
			Kind:      "k",
			ID:        strings.Repeat("x", i+1),
		}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	filtered := LedgerForComponent(entries, "assay/a")
	if len(filtered) != 3 {
		t.Fatalf("got %d entries for assay/a, want 3", len(filtered))
	}
	wantIDs := []string{"x", "xxx", "xxxxx"}
	for i, e := range filtered {
		if e.ID != wantIDs[i] {
			t.Errorf("order not preserved: entry %d = %q, want %q", i, e.ID, wantIDs[i])
		}
	}
}
