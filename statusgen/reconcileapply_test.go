package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeApplyFixtureReadme lays a single-stream README (a plain hand-written
// briefs table — no generated markers, since writeStatusCell must work on
// either shape by locating the Status column by NAME) under a temp
// docs/streams/<stream>/README.md and returns its path.
func writeApplyFixtureReadme(t *testing.T, stream, body string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, path
}

const applyFixtureHeader = "---\n" +
	"stream: apstream\n" +
	"status: active\n" +
	"---\n\n# apstream\n\n" +
	"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
	"|---|-------|------|--------|--------|----------|----------|\n"

// Case 1: a `todo` row whose brief has a merged-PR witness (either a real
// trailer or the backfill branch/body match — both surface as Cell
// "implemented" with Source "pr"/"backfill") flips to `implemented` under
// --apply, and NOTHING else in that row changes.
func TestApplyReconcileWrites_TodoToImplemented(t *testing.T) {
	body := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | todo | — | — |\n"
	root, path := writeApplyFixtureReadme(t, "apstream", body)

	cells := []BriefCell{
		{ID: "apstream/01", Cell: "implemented", Source: "backfill", Witness: "PR #42 (merged abc1234) — backfill: branch/body match, no trailer"},
	}
	applied, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("want 1 applied row, got %d: %+v", len(applied), applied)
	}
	if applied[0].From != "todo" || applied[0].To != "implemented" || applied[0].ID != "apstream/01" {
		t.Fatalf("unexpected applied row: %+v", applied[0])
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | implemented | — | — |\n"
	if string(got) != want {
		t.Fatalf("row content changed beyond the Status cell:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Case 2: a row already at `done` (or `verified`) must NOT be touched by
// --apply, even when the cells fed in claim a fresh `implemented` witness —
// done/verified rows are immutable to this verb regardless of what a later
// merged PR might suggest.
func TestApplyReconcileWrites_DoneRowImmutable(t *testing.T) {
	for _, state := range []string{"done", "verified"} {
		t.Run(state, func(t *testing.T) {
			body := applyFixtureHeader +
				"| 01 | [first thing](brief-01-first.md) | 0 | M | " + state + " | 2026-01-02 verifier | 2026-01-03 reviewer |\n"
			root, path := writeApplyFixtureReadme(t, "apstream", body)

			cells := []BriefCell{
				{ID: "apstream/01", Cell: "implemented", Source: "backfill", Witness: "PR #99 (merged deadbee) — backfill: branch/body match, no trailer"},
			}
			applied, err := applyReconcileWrites(root, cells)
			if err != nil {
				t.Fatalf("applyReconcileWrites: %v", err)
			}
			if len(applied) != 0 {
				t.Fatalf("want zero applied rows for an already-%s row, got %+v", state, applied)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != body {
				t.Fatalf("a %s row must be byte-for-byte untouched:\ngot:\n%s\nwant:\n%s", state, got, body)
			}
		})
	}
}

// Case 3: a row that already carries a `human:<name>` stamp in its Reviewed
// cell must never have that stamp removed, added, or altered — under any
// circumstance, including a live todo->implemented flip on the SAME row.
func TestApplyReconcileWrites_NeverTouchesHumanStamp(t *testing.T) {
	body := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | todo | — | human:ian |\n"
	root, path := writeApplyFixtureReadme(t, "apstream", body)

	cells := []BriefCell{
		{ID: "apstream/01", Cell: "implemented", Source: "pr", Witness: "PR #7 (merged cafefee)"},
	}
	applied, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("want the Status cell to flip, got %+v", applied)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "human:ian") {
		t.Fatalf("the human:ian stamp in Reviewed must survive verbatim; got:\n%s", got)
	}
	want := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | implemented | — | human:ian |\n"
	if string(got) != want {
		t.Fatalf("only the Status cell may change:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Case 4: a row with no PR witness at all (the normal fold and the backfill
// both left it `todo`) stays `todo`, completely untouched by --apply — this
// is exactly what --report already lists for a human, never guessed at here.
func TestApplyReconcileWrites_NoWitnessLeftAlone(t *testing.T) {
	body := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | todo | — | — |\n"
	root, path := writeApplyFixtureReadme(t, "apstream", body)

	cells := []BriefCell{
		{ID: "apstream/01", Cell: "todo", Source: "pr", Reason: "PR search ran; no open or merged PR carries this brief's trailer"},
	}
	applied, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("want zero applied rows with no witness, got %+v", applied)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("a no-witness row must be untouched:\ngot:\n%s\nwant:\n%s", got, body)
	}
}

// TestApplyReconcileWrites_ExcludesBackfillHandSaidUnknown documents that the
// backfill's OTHER Source=="backfill" shape — a hand-asserted
// implemented/verified/done with no PR at all, which DeriveLifecycle/backfill
// render as Cell "unknown" — is never written by --apply: it is not
// Cell=="implemented", so witnessedImplemented excludes it by construction,
// and it stays exactly what --report already surfaces for a human.
func TestApplyReconcileWrites_ExcludesBackfillHandSaidUnknown(t *testing.T) {
	body := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | todo | — | — |\n"
	root, path := writeApplyFixtureReadme(t, "apstream", body)

	cells := []BriefCell{
		{ID: "apstream/01", Cell: "unknown", Source: "backfill", Reason: "no witness — hand-asserted implemented at abc1234"},
	}
	applied, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("want zero applied rows for a no-PR hand-said unknown, got %+v", applied)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("must be untouched:\ngot:\n%s\nwant:\n%s", got, body)
	}
}

// TestWriteStatusCell_InProgressToImplemented covers the second legal source
// state named by the brief: in-progress -> implemented.
func TestWriteStatusCell_InProgressToImplemented(t *testing.T) {
	body := applyFixtureHeader +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | in-progress | — | — |\n"
	_, path := writeApplyFixtureReadme(t, "apstream", body)

	wrote, from, err := writeStatusCell(path, "01", "implemented")
	if err != nil {
		t.Fatalf("writeStatusCell: %v", err)
	}
	if !wrote || from != "in-progress" {
		t.Fatalf("wrote=%v from=%q, want wrote=true from=\"in-progress\"", wrote, from)
	}
}

// TestWriteStatusCell_GeneratedMarkerTable proves the write also works
// correctly on a derived-board/04 generated (marker-wrapped) table, whose row
// shape is identical to a plain table's — only the Status cell inside the
// markers may change; prose outside the markers and the markers themselves
// must survive byte-for-byte.
func TestWriteStatusCell_GeneratedMarkerTable(t *testing.T) {
	body := "---\nstream: apstream\nstatus: active\nboard: generated\n---\n\n" +
		"Prose above.\n\n" +
		briefsMarkerBegin + "\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [first thing](brief-01-first.md) | 0 | M | todo | — | — |\n" +
		briefsMarkerEnd + "\n\n" +
		"Prose below.\n"
	_, path := writeApplyFixtureReadme(t, "apstream", body)

	wrote, from, err := writeStatusCell(path, "01", "implemented")
	if err != nil {
		t.Fatalf("writeStatusCell: %v", err)
	}
	if !wrote || from != "todo" {
		t.Fatalf("wrote=%v from=%q, want wrote=true from=\"todo\"", wrote, from)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), briefsMarkerBegin) || !strings.Contains(string(got), briefsMarkerEnd) {
		t.Fatalf("markers must survive:\n%s", got)
	}
	if !strings.Contains(string(got), "Prose above.") || !strings.Contains(string(got), "Prose below.") {
		t.Fatalf("prose outside the markers must survive:\n%s", got)
	}
	if !strings.Contains(string(got), "| 01 | [first thing](brief-01-first.md) | 0 | M | implemented | — | — |") {
		t.Fatalf("row was not rewritten as expected:\n%s", got)
	}
}

// TestWriteStatusCell_MissingReadmeIsNotAnError documents that --apply is
// idempotent/non-fatal when a witnessed brief's README cannot be found: not
// this write's failure to raise.
func TestWriteStatusCell_MissingReadmeIsNotAnError(t *testing.T) {
	root := t.TempDir()
	wrote, from, err := writeStatusCell(filepath.Join(root, "docs", "streams", "nostream", "README.md"), "01", "implemented")
	if err != nil {
		t.Fatalf("want no error for a missing README, got %v", err)
	}
	if wrote || from != "" {
		t.Fatalf("wrote=%v from=%q, want wrote=false from=\"\"", wrote, from)
	}
}
