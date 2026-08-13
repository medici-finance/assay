package deskkit

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionUnpinned(t *testing.T) {
	oldS, oldB := SourceSHA, BuiltAt
	t.Cleanup(func() { SourceSHA, BuiltAt = oldS, oldB })

	SourceSHA, BuiltAt = "", ""
	s, b := Version()
	if s != "unpinned" || b != "unpinned" {
		t.Fatalf("Version() = %q,%q, want unpinned,unpinned", s, b)
	}
	if IsPinned() {
		t.Fatalf("IsPinned() = true for empty stamp")
	}
	var buf bytes.Buffer
	WarnIfUnpinned(&buf)
	if !strings.Contains(buf.String(), "UNPINNED") {
		t.Fatalf("WarnIfUnpinned did not warn: %q", buf.String())
	}
}

func TestVersionPinned(t *testing.T) {
	oldS, oldB := SourceSHA, BuiltAt
	t.Cleanup(func() { SourceSHA, BuiltAt = oldS, oldB })

	SourceSHA, BuiltAt = "abc1234", "2026-07-10T00:00:00Z"
	s, b := Version()
	if s != "abc1234" || b != "2026-07-10T00:00:00Z" {
		t.Fatalf("Version() = %q,%q, want the stamped values", s, b)
	}
	if !IsPinned() {
		t.Fatalf("IsPinned() = false for stamped binary")
	}
	var buf bytes.Buffer
	WarnIfUnpinned(&buf)
	if buf.Len() != 0 {
		t.Fatalf("WarnIfUnpinned wrote %q for a pinned binary", buf.String())
	}
}
