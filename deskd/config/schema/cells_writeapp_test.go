package schema

import (
	"strings"
	"testing"
)

// writeAppDoc is validDoc's shape with a per-loop write_app on the review
// loop — the service contract read/write split expressed per loop.
const writeAppDoc = `
cells:
  - name: assay
    repos:
      - example-org/toolkit
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat, glm-5.2]
        app: assay-reviewer-app
        write_app: assay-cell-issues
      - role: metrics
        schedule: "0 * * * *"
        models: []
        app: assay-desk-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`

// Verify row 2 — a cell WITH write_app validates.
func TestParse_WriteAppPresentValidates(t *testing.T) {
	f, err := Parse([]byte(writeAppDoc))
	if err != nil {
		t.Fatalf("cell with write_app should validate, got: %v", err)
	}
	got := f.Cells[0].Loops[0].WriteAppOrDefault()
	if got != "assay-cell-issues" {
		t.Fatalf("review loop write identity: want assay-cell-issues, got %q", got)
	}
}

// Verify row 3 — a present-but-blank write_app: "" is rejected, mirroring the
// app-non-empty rule. A plain string could not distinguish this from an absent
// key; the *string field is what makes the check enforceable.
func TestParse_WriteAppPresentButEmptyRejected(t *testing.T) {
	doc := strings.Replace(writeAppDoc, "write_app: assay-cell-issues", `write_app: ""`, 1)
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal(`write_app: "" should be rejected as a config error, got nil`)
	}
	if !strings.Contains(err.Error(), "write_app") {
		t.Fatalf("error should name write_app, got: %v", err)
	}
}

// Verify row 4 — the default-to-app rule lives in exactly one resolver:
// WriteAppOrDefault() returns app when write_app is absent and write_app when
// present. This is the single control standing between an omitted field and a
// silent identity change (§Review core-system question 1).
func TestWriteAppOrDefault(t *testing.T) {
	// absent (nil) → the loop's own app, never empty, never a hardcoded slug.
	absent := Loop{App: "assay-reviewer-app"}
	if got := absent.WriteAppOrDefault(); got != "assay-reviewer-app" {
		t.Fatalf("absent write_app should resolve to app, want assay-reviewer-app, got %q", got)
	}
	// present → the named write App.
	w := "assay-cell-issues"
	present := Loop{App: "assay-reviewer-app", WriteApp: &w}
	if got := present.WriteAppOrDefault(); got != "assay-cell-issues" {
		t.Fatalf("present write_app should resolve to write_app, want assay-cell-issues, got %q", got)
	}
}

// A cell with NO write_app still validates and resolves its write identity to
// app — back-compat (Verify row 1's contract, at the unit level).
func TestParse_NoWriteAppUnchanged(t *testing.T) {
	f, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("cell with no write_app should validate: %v", err)
	}
	l := f.Cells[0].Loops[0]
	if l.WriteApp != nil {
		t.Fatalf("absent write_app should decode to nil, got %v", l.WriteApp)
	}
	if got := l.WriteAppOrDefault(); got != l.App {
		t.Fatalf("no write_app should resolve to app %q, got %q", l.App, got)
	}
}
