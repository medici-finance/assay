package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rowscope_test.go — the row-scoped README landing Verify suite, replaying the real defect shape
// (a stale whole-file README landing reverting an unrelated row) with neutral names, on a
// fixture `example-stream` README carrying statusgen's own generated-table markers.

// exampleStreamReadme builds a minimal `example-stream` README carrying the marker-wrapped
// Briefs table, one row per (num -> status) pair in rows, in ascending num order. prefix and
// suffix are extra prose placed outside the markers, so a test can prove that prose always
// comes from whichever copy (remote/local) the assertion cares about.
func exampleStreamReadme(prefix string, rows map[string]string, suffix string) string {
	nums := make([]string, 0, len(rows))
	for n := range rows {
		nums = append(nums, n)
	}
	sort.Strings(nums)
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(tableMarkerBegin + "\n")
	b.WriteString("| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n")
	b.WriteString("|---|-------|------|--------|--------|----------|----------|\n")
	for _, n := range nums {
		fmt.Fprintf(&b, "| %s | [Brief %s](brief-%s.md) | 0 | S | %s | — | — |\n", n, n, n, rows[n])
	}
	b.WriteString(tableMarkerEnd + "\n")
	b.WriteString(suffix)
	return b.String()
}

// --- Verify row 1: TestReadmeLandingKeepsForeignRow ---
//
// Remote row 02 stands at `implemented`. The local copy is one landing stale: it has already
// reverted row 02 to `todo` in its own (stale) view, and carries a fresh update to row 01. The
// landing names ONLY row 01. Expect: row 01 lands with the local content, row 02 stays
// `implemented` (the remote's), and one stale-local line names row 02.
func TestReadmeLandingKeepsForeignRow(t *testing.T) {
	f, errBuf := setupFake(t)
	target := "docs/streams/example-stream/README.md"
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "implemented"}, "\n")
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "todo"}, "\n")
	f.setFile(target, remote)
	root := rootWithFile(t, target, local)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root, "--row", "01"})
	if code != deskkit.ExitOK {
		t.Fatalf("row-scoped landing exit = %d, want 0 (stderr: %s)", code, errBuf.String())
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	committed := f.putContent
	if !strings.Contains(committed, "| 01 | [Brief 01](brief-01.md) | 0 | S | verified | — | — |") {
		t.Fatalf("named row 01 did not land with the local content:\n%s", committed)
	}
	if !strings.Contains(committed, "| 02 | [Brief 02](brief-02.md) | 0 | S | implemented | — | — |") {
		t.Fatalf("foreign row 02 was reverted — want it to STAY implemented (the remote's):\n%s", committed)
	}
	if strings.Contains(committed, "| 02 | [Brief 02](brief-02.md) | 0 | S | todo | — | — |") {
		t.Fatalf("foreign row 02 was overwritten with the stale local value:\n%s", committed)
	}
	if !strings.Contains(errBuf.String(), "stale-local: row 02 differed and was NOT written") {
		t.Fatalf("expected a stale-local notice for row 02, stderr:\n%s", errBuf.String())
	}
}

// --- Verify row 2: TestReadmeLandingWritesNamedRow ---
//
// The named row's three lifecycle cells (Status/Verified/Reviewed) land byte-exact from the
// local file.
func TestReadmeLandingWritesNamedRow(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/example-stream/README.md"
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	// Hand-craft the local row so all three lifecycle cells differ from the remote's, proving
	// the whole row (not just Status) lands byte-exact.
	local = strings.Replace(local,
		"| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |",
		"| 04 | [Brief 04](brief-04.md) | 0 | S | verified | 2026-09-25 | ada |", 1)
	f.setFile(target, remote)
	root := rootWithFile(t, target, local)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root, "--row", "04"})
	if code != deskkit.ExitOK {
		t.Fatalf("row-scoped landing exit = %d, want 0", code)
	}
	want := "| 04 | [Brief 04](brief-04.md) | 0 | S | verified | 2026-09-25 | ada |"
	if !strings.Contains(f.putContent, want) {
		t.Fatalf("named row did not land byte-exact, want line:\n%s\ngot:\n%s", want, f.putContent)
	}
}

// --- Verify row 3: TestNonTableTargetsUnchanged ---
//
// A brief-path merge and a .jsonl sidecar landing — neither carries the generated-table
// markers — behave exactly as they did before this brief: no --row required, no rebase.
func TestNonTableTargetsUnchanged(t *testing.T) {
	t.Run("brief-path merge", func(t *testing.T) {
		f, _ := setupFake(t)
		briefPath := "docs/streams/x/brief.md"
		f.setFile(briefPath, "# Brief\n\n## Evidence\n| 1 | a | b |\n")
		evidencePath := writeRepoFile(t, "row.md", "| 2 | c | d |\n")

		code := run([]string{"example-org/tracker", "main",
			"--evidence-file", evidencePath, "--brief-path", briefPath})
		if code != deskkit.ExitOK {
			t.Fatalf("brief-merge exit = %d, want 0", code)
		}
		if f.putCalls != 1 {
			t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
		}
		if !strings.Contains(f.putContent, "| 1 | a | b |") || !strings.Contains(f.putContent, "| 2 | c | d |") {
			t.Fatalf("merged content missing a row:\n%s", f.putContent)
		}
	})

	t.Run("jsonl sidecar", func(t *testing.T) {
		f, _ := setupFake(t)
		target := "docs/streams/x/rows.jsonl"
		root := rootWithFile(t, target, "{\"a\":1}\n{\"b\":2}\n")
		f.setFile(target, "{\"a\":1}\n")

		code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
		if code != deskkit.ExitOK {
			t.Fatalf("jsonl growth exit = %d, want 0", code)
		}
		if f.putCalls != 1 {
			t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
		}
		if f.putContent != "{\"a\":1}\n{\"b\":2}\n" {
			t.Fatalf("jsonl content changed unexpectedly: %q", f.putContent)
		}
	})
}

// --- Verify row 4: TestReadmeLandingOutsideMarkersFromRemote ---
//
// Prose outside the markers always comes from the remote, even when the local copy's outside
// prose differs (a stale local copy that also drifted in unrelated prose must not carry that
// drift onto the branch).
func TestReadmeLandingOutsideMarkersFromRemote(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/example-stream/README.md"
	remote := exampleStreamReadme("# example-stream\n\nREMOTE PROSE\n\n", map[string]string{"01": "todo"}, "\nREMOTE FOOTER\n")
	local := exampleStreamReadme("# example-stream\n\nLOCAL STALE PROSE\n\n", map[string]string{"01": "implemented"}, "\nLOCAL STALE FOOTER\n")
	f.setFile(target, remote)
	root := rootWithFile(t, target, local)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root, "--row", "01"})
	if code != deskkit.ExitOK {
		t.Fatalf("row-scoped landing exit = %d, want 0", code)
	}
	committed := f.putContent
	if !strings.Contains(committed, "REMOTE PROSE") || !strings.Contains(committed, "REMOTE FOOTER") {
		t.Fatalf("outside-marker prose did not come from the remote:\n%s", committed)
	}
	if strings.Contains(committed, "LOCAL STALE PROSE") || strings.Contains(committed, "LOCAL STALE FOOTER") {
		t.Fatalf("outside-marker prose leaked in from the stale local copy:\n%s", committed)
	}
	if !strings.Contains(committed, "| 01 | [Brief 01](brief-01.md) | 0 | S | implemented | — | — |") {
		t.Fatalf("named row did not land:\n%s", committed)
	}
}

// --- Verify row 5: TestWriteOpRowScopeSecondLayer ---
//
// The write op's OWN re-enforcement (rowScopeWriteTimeCheck), exercised directly against a
// fresh fetch that disagrees with the content about to be committed on a FOREIGN row — the
// pre-check that would ordinarily have caught this earlier is bypassed entirely, so a pass
// here can only be the write op's own, independent check.
func TestWriteOpRowScopeSecondLayer(t *testing.T) {
	target := "docs/streams/example-stream/README.md"
	freshAtWrite := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "verified"}, "\n")
	f := &fakeForge{}
	f.setFile(target, freshAtWrite)
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}

	t.Run("foreign row disagrees with the fresh fetch — refused", func(t *testing.T) {
		// commitContent still carries row 02 at its OLD (pre-check-time) value "implemented",
		// but the write-time fetch above now reports row 02 at "verified" — a table change in
		// the race window between the pre-check and the write.
		staleCommit := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "implemented"}, "\n")
		err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(staleCommit))
		if err == nil {
			t.Fatal("expected the write op to refuse a foreign-row mismatch against its own fetch, got nil")
		}
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (refused); err=%v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
		}
		if !strings.Contains(err.Error(), "02") {
			t.Fatalf("refusal does not name the offending row 02: %v", err)
		}
	})

	t.Run("foreign rows agree with the fresh fetch — accepted", func(t *testing.T) {
		agreeingCommit := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "verified"}, "\n")
		if err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(agreeingCommit)); err != nil {
			t.Fatalf("expected the write op to accept matching foreign rows, got: %v", err)
		}
	})
}

// --- Verify row 6: TestReadmeLandingRequiresRow ---
//
// A marker-carrying target with no --row exits 5, naming the flag.
func TestReadmeLandingRequiresRow(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/example-stream/README.md"
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo"}, "\n")
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "implemented"}, "\n")
	f.setFile(target, remote)
	root := rootWithFile(t, target, local)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("no --row on a table target exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("refusal still wrote %d time(s)", f.putCalls)
	}
}
