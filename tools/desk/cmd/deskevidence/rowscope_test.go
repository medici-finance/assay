package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		_, err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(staleCommit))
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
		sha, err := rowScopeWriteTimeCheck(f, repo, target, "main", []string{"01"}, []byte(agreeingCommit))
		if err != nil {
			t.Fatalf("expected the write op to accept matching foreign rows, got: %v", err)
		}
		if sha == "" {
			t.Fatal("an accepted write-time check returned no content id for the write's ExpectedSHA precondition")
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

// --- Review round 1 (PR #1685): the guard's refusal branches, each pinned by a test that goes
// red when the branch is removed (mutations.json carries each removal as a mutation). ---

const exampleReadmePath = "docs/streams/example-stream/README.md"

// landRow runs one row-scoped landing of local over remote, naming rows, and returns the exit
// code — the run()-level shape every test below uses.
func landRow(t *testing.T, f *fakeForge, remote, local string, rows ...string) int {
	t.Helper()
	f.setFile(exampleReadmePath, remote)
	root := rootWithFile(t, exampleReadmePath, local)
	args := []string{"example-org/tracker", "main", "--evidence-file", exampleReadmePath, "--root", root}
	for _, r := range rows {
		args = append(args, "--row", r)
	}
	return run(args)
}

// TestRowScopeMarkersMatchStatusgen pins tableMarkerBegin / tableMarkerEnd byte-for-byte to
// statusgen's own constants, read from the repo tree (statusgen is a separate Go module, so the
// literals cannot be imported). A drift would make every README look table-less; this test is
// what makes that drift loud instead of silent.
//
// The source is found by walking up from the package directory to the checkout root (the first
// ancestor holding .git). A checkout root without statusgen/readmetable.go FAILS — a moved file
// must re-point this pin, never silently skip it. Only a tree with no checkout root at all — an
// isolated copy of tools/desk, such as a `muhar -j >1` workspace — skips, since the statusgen
// module is simply not in it; the full checkout and CI always run the pin.
func TestRowScopeMarkersMatchStatusgen(t *testing.T) {
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, serr := os.Stat(filepath.Join(dir, ".git")); serr == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("no checkout root above this package (an isolated copy of tools/desk) — statusgen's source is not in this tree")
		}
		dir = parent
	}
	src, err := os.ReadFile(filepath.Join(dir, "statusgen", "readmetable.go"))
	if err != nil {
		t.Fatalf("cannot read statusgen/readmetable.go under the checkout root %s: %v", dir, err)
	}
	for name, want := range map[string]string{"briefsMarkerBegin": tableMarkerBegin, "briefsMarkerEnd": tableMarkerEnd} {
		m := regexp.MustCompile(`(?m)^\s*` + name + `\s*=\s*"([^"]*)"`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("statusgen/readmetable.go no longer declares %s — re-point this pin", name)
		}
		if string(m[1]) != want {
			t.Fatalf("marker drift: statusgen %s = %q, deskevidence has %q", name, m[1], want)
		}
	}
}

// A README carrying a marker literal whose region does not parse the way statusgen and this
// tool agree on (here: the begin marker shares its line with prose) is REFUSED, never landed as
// a whole-file write the guard does not see.
func TestReadmeLandingMalformedMarkersFailClosed(t *testing.T) {
	f, _ := setupFake(t)
	remote := strings.Replace(
		exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "implemented"}, "\n"),
		tableMarkerBegin+"\n", "board: "+tableMarkerBegin+"\n", 1)
	local := strings.Replace(remote, "| 02 | [Brief 02](brief-02.md) | 0 | S | implemented |", "| 02 | [Brief 02](brief-02.md) | 0 | S | todo |", 1)
	if code := landRow(t, f, remote, local); code != deskkit.ExitRefused {
		t.Fatalf("malformed-marker README exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("malformed-marker README was written %d time(s)", f.putCalls)
	}
}

// A non-README file that merely QUOTES the marker literals (a brief documenting them) is not a
// statusgen table target and lands exactly as before.
func TestNonReadmeQuotingMarkersUnaffected(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/example-stream/brief-04.md"
	remote := "# Brief\n\nmarkers: `" + tableMarkerBegin + "` / `" + tableMarkerEnd + "`\n"
	f.setFile(target, remote)
	root := rootWithFile(t, target, remote+"\n## Evidence\n| 1 | ok |\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("non-README quoting the markers exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
}

// F-rowscope-duplicate-key: a key that appears twice in either table makes "which row" a guess
// — refused, nothing written, whichever side carries the duplicate.
func TestReadmeLandingRefusesDuplicateKey(t *testing.T) {
	dup := func(base string) string {
		return strings.Replace(base, "| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |\n",
			"| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |\n| 04 | [Other 04](other-04.md) | 0 | S | todo | — | — |\n", 1)
	}
	one := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	landed := strings.Replace(one, "| implemented | — | — |", "| verified | 2026-09-25 | x |", 1)
	for name, pair := range map[string][2]string{
		"remote duplicate": {dup(one), landed},
		"local duplicate":  {one, dup(one)},
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			if code := landRow(t, f, pair[0], pair[1], "04"); code != deskkit.ExitRefused {
				t.Fatalf("duplicate key exit = %d, want %d (stderr: %s)", code, deskkit.ExitRefused, errBuf.String())
			}
			if f.putCalls != 0 {
				t.Fatalf("duplicate key still wrote %d time(s)", f.putCalls)
			}
			if !strings.Contains(errBuf.String(), "more than once") {
				t.Fatalf("refusal does not name the duplicate: %s", errBuf.String())
			}
		})
	}
}

// The pre-check layer ALONE refuses a remote duplicate key (the run()-level test above can also
// be satisfied by the write-time layer, which re-parses the commit and sees the same duplicate —
// so this pins the first layer independently of the second).
func TestRebaseNamedRowsRefusesRemoteDuplicate(t *testing.T) {
	one := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	remote := strings.Replace(one, "| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |\n",
		"| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |\n| 04 | [Other 04](other-04.md) | 0 | S | todo | — | — |\n", 1)
	_, _, _, err := rebaseNamedRows([]byte(remote), []byte(one), []string{"04"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "remote table carries row key(s) 04 more than once") {
		t.Fatalf("pre-check on a remote duplicate: want a refusal naming 04, got %v", err)
	}
}

// Contract item 4, both sides: a named row absent from the remote table, or from the local
// file, is refused — never indexed as line 0 (which would overwrite the README's first line).
func TestReadmeLandingNamedRowAbsentRefused(t *testing.T) {
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo"}, "\n")
	withRow := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "04": "verified"}, "\n")
	for name, pair := range map[string][2]string{
		"absent from remote": {remote, withRow},
		"absent from local":  {withRow, remote},
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			if code := landRow(t, f, pair[0], pair[1], "04"); code != deskkit.ExitRefused {
				t.Fatalf("absent named row exit = %d, want %d (stderr: %s)", code, deskkit.ExitRefused, errBuf.String())
			}
			if f.putCalls != 0 {
				t.Fatalf("absent named row still wrote %d time(s):\n%s", f.putCalls, f.putContent)
			}
			if !strings.Contains(errBuf.String(), "--row 04 names a row absent") {
				t.Fatalf("refusal does not name the absent row: %s", errBuf.String())
			}
		})
	}
}

// A-malformed-named-row / S2: a named local line with the wrong cell count, or a bare CR
// mid-line, is refused rather than landed verbatim.
func TestReadmeLandingMalformedNamedRowRefused(t *testing.T) {
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	row := "| 04 | [Brief 04](brief-04.md) | 0 | S | implemented | — | — |"
	for name, bad := range map[string]string{
		"short row":        "| 04 | verified |",
		"mid-line CR":      "| 04 | [Brief 04](brief-04.md) | 0 | S | verified\r19 | done | x |",
		"extra cell count": "| 04 | [Brief 04](brief-04.md) | 0 | S | verified | x | y | z |",
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			if code := landRow(t, f, remote, strings.Replace(remote, row, bad, 1), "04"); code != deskkit.ExitRefused {
				t.Fatalf("%s exit = %d, want %d (stderr: %s)", name, code, deskkit.ExitRefused, errBuf.String())
			}
			if f.putCalls != 0 {
				t.Fatalf("%s still wrote:\n%s", name, f.putContent)
			}
		})
	}
}

// A-named-row-whole-line: only the named row's LIFECYCLE cells land from local; its authoring
// cells (title, wave, effort) stay the remote's, and the caller is told they differed.
func TestReadmeLandingNamedRowAuthoringFromRemote(t *testing.T) {
	f, errBuf := setupFake(t)
	remote := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "implemented"}, "\n")
	remote = strings.Replace(remote, "[Brief 04](brief-04.md) | 0 | S |", "[Renamed 04](brief-04.md) | 1 | M |", 1)
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"04": "verified"}, "\n")
	if code := landRow(t, f, remote, local, "04"); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errBuf.String())
	}
	want := "| 04 | [Renamed 04](brief-04.md) | 1 | M | verified | — | — |"
	if !strings.Contains(f.putContent, want) {
		t.Fatalf("named row did not keep the remote's authoring cells, want line:\n%s\ngot:\n%s", want, f.putContent)
	}
	if !strings.Contains(errBuf.String(), "stale-local: row 04 authoring cell(s) differed and were NOT written") {
		t.Fatalf("expected an authoring stale-local notice for row 04, stderr:\n%s", errBuf.String())
	}
}

// F-rowscope-unpinned M1, at run() level: the table changes between the pre-check's read and
// the write-time re-fetch. The write op's own layer — reached only through its call site in
// cmdEvidence — refuses; nothing is written.
func TestReadmeLandingRaceCaughtByWriteTimeLayer(t *testing.T) {
	f, errBuf := setupFake(t)
	atPrecheck := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "implemented"}, "\n")
	atWrite := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "verified"}, "\n")
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "implemented"}, "\n")
	f.readScript = map[string][]string{exampleReadmePath: {atPrecheck, atWrite}}
	root := rootWithFile(t, exampleReadmePath, local)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", exampleReadmePath, "--root", root, "--row", "01"})
	if code != deskkit.ExitRefused {
		t.Fatalf("race exit = %d, want %d (stderr: %s)", code, deskkit.ExitRefused, errBuf.String())
	}
	if f.putCalls != 0 {
		t.Fatalf("race landing still wrote — row 02 reverted to implemented:\n%s", f.putContent)
	}
	if !strings.Contains(errBuf.String(), "foreign row(s) 02") {
		t.Fatalf("refusal does not name row 02: %s", errBuf.String())
	}
}

// F-rowscope-race-claim, at run() level: the table changes AFTER the write-time re-check, just
// before the backend's own fetch. The write carries the re-check's content id as ExpectedSHA, so
// the backend refuses instead of overwriting.
func TestReadmeLandingRaceAfterRecheckRefusedByWrite(t *testing.T) {
	f, errBuf := setupFake(t)
	base := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "implemented"}, "\n")
	moved := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "todo", "02": "verified"}, "\n")
	local := exampleStreamReadme("# example-stream\n\n", map[string]string{"01": "verified", "02": "implemented"}, "\n")
	f.readScript = map[string][]string{exampleReadmePath: {base, base, moved}}
	root := rootWithFile(t, exampleReadmePath, local)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", exampleReadmePath, "--root", root, "--row", "01"})
	if code != deskkit.ExitRefused {
		t.Fatalf("post-recheck race exit = %d, want %d (stderr: %s)", code, deskkit.ExitRefused, errBuf.String())
	}
	if f.putCalls != 0 || f.expectedSHARefusals != 1 {
		t.Fatalf("want the write refused by its ExpectedSHA precondition (puts=%d, refusals=%d)", f.putCalls, f.expectedSHARefusals)
	}
}

// F-rowscope-unpinned M4 + A-prose-race: the write-time layer refuses a commit that carries a
// foreign row the fresh fetch lacks, and a commit whose prose outside the table moved.
func TestWriteOpRowScopeCommitOnlyRowAndProse(t *testing.T) {
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	fresh := exampleStreamReadme("# example-stream\n\nPROSE\n\n", map[string]string{"01": "todo"}, "\n")
	f := &fakeForge{}
	f.setFile(exampleReadmePath, fresh)

	extra := exampleStreamReadme("# example-stream\n\nPROSE\n\n", map[string]string{"01": "verified", "03": "todo"}, "\n")
	_, err := rowScopeWriteTimeCheck(f, repo, exampleReadmePath, "main", []string{"01"}, []byte(extra))
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "foreign row(s) 03") {
		t.Fatalf("commit-only foreign row 03: want a refusal naming 03, got %v", err)
	}

	prose := exampleStreamReadme("# example-stream\n\nSTALE PROSE\n\n", map[string]string{"01": "verified"}, "\n")
	_, err = rowScopeWriteTimeCheck(f, repo, exampleReadmePath, "main", []string{"01"}, []byte(prose))
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "outside the named row") {
		t.Fatalf("moved prose: want a refusal naming content outside the named row(s), got %v", err)
	}
}
