package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStoreAppendIsAppendOnly proves Append never rewrites a prior line: two
// appends yield two lines, and the first line is byte-identical before and
// after the second append.
func TestStoreAppendIsAppendOnly(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	if err := s.Append(KindCommit, Commit{SHA: "aaa", Message: "one"}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	after1, err := os.ReadFile(filepath.Join(root, qualityDir, commitsTable))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if err := s.Append(KindCommit, Commit{SHA: "bbb", Message: "two"}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	after2, err := os.ReadFile(filepath.Join(root, qualityDir, commitsTable))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(after2) <= len(after1) {
		t.Fatal("second append did not grow the file")
	}
	if string(after2[:len(after1)]) != string(after1) {
		t.Fatal("second append rewrote the first line — not append-only")
	}

	commits, err := s.ReadCommits()
	if err != nil {
		t.Fatalf("read commits: %v", err)
	}
	if len(commits) != 2 || commits[0].SHA != "aaa" || commits[1].SHA != "bbb" {
		t.Fatalf("stream did not return records in order: %+v", commits)
	}
}

// TestStoreHeaderRoundTrip proves the header read/write is faithful, and that an
// unmined root reports no header (nil) rather than erroring.
func TestStoreHeaderRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	if h, err := s.ReadHeader(); err != nil || h != nil {
		t.Fatalf("unmined root should read (nil, nil), got (%v, %v)", h, err)
	}

	in := MineHeader{
		MinedAt:         time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC),
		TipSHA:          "tip123",
		Horizon:         "root000",
		Discontinuities: []Discontinuity{{Kind: "shallow-clone-floor", Detail: "floored"}},
		Coverage:        Coverage{Measured: 5, MeasuredZero: 2, CouldNotMeasure: 1},
		CommitCount:     7,
		DiffCount:       8,
	}
	if err := s.WriteHeader(in); err != nil {
		t.Fatalf("write header: %v", err)
	}
	out, err := s.ReadHeader()
	if err != nil || out == nil {
		t.Fatalf("read header: %v", err)
	}
	if out.TipSHA != in.TipSHA || out.Horizon != in.Horizon || out.CommitCount != in.CommitCount {
		t.Fatalf("header round-trip lost data: %+v", out)
	}
	if out.SchemaVersion != schemaVersion {
		t.Fatalf("header should stamp the schema version, got %q", out.SchemaVersion)
	}
	if out.Coverage.CouldNotMeasure != 1 {
		t.Fatalf("coverage lost: %+v", out.Coverage)
	}
}

// TestStoreReadRejectsCorruptLine proves a malformed table line fails loudly
// rather than being silently skipped.
func TestStoreReadRejectsCorruptLine(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, qualityDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, commitsTable), []byte("{not json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(root).ReadCommits(); err == nil {
		t.Fatal("expected a corrupt line to error, not be skipped")
	}
}
