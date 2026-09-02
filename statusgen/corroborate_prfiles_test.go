package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestPRFilesToDiff_ReconstructsParseableDiff proves the diff string
// reconstructed from the paginated "List PR files" API entries is exactly what
// stampsInDiff parses. Three properties, all ASSERTED:
//   - a real status-cell "+human:<name>" stamp is found with the right Name+File;
//   - a PROSE "human:<name>" placeholder line (the form documentation references carry
//     — "human:" followed by "<", which breaks the token boundary) is NOT read as
//     a stamp, even though it rides on the same "+"-added file;
//   - a binary file (empty patch) contributes only the header and no stamp.
func TestPRFilesToDiff_ReconstructsParseableDiff(t *testing.T) {
	files := []ghPRFile{
		{
			// A normal file whose patch adds a status-cell row bearing a real
			// human:<name> stamp (the exact shape the existing stampsInDiff
			// tests treat as a stamp) AND a prose placeholder line that must
			// NOT be read as a stamp.
			Filename: "docs/streams/methodology/README.md",
			Patch: "@@ -50,9 +50,9 @@\n" +
				"-| 01 | Some brief | 0 | S | verified | 2026-07-10 opus-verifier | — |\n" +
				"+| 01 | Some brief | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 human:alex |\n" +
				"+corroborates only `human:<name>` sign-off stamps in the diff",
		},
		{
			// A binary/oversize file: the API returns no patch. It must
			// contribute only the header and no added lines.
			Filename: "docs/streams/methodology/diagram.png",
			Patch:    "",
		},
	}

	diff := prFilesToDiff(files)
	stamps := stampsInDiff("", diff)

	// (a) exactly the ONE real stamp — the prose `human:<name>` line adds none.
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want exactly 1 (prose human:<name> is not a stamp): %+v",
			len(stamps), stamps)
	}
	if stamps[0].Name != "alex" {
		t.Errorf("stamp Name = %q, want %q", stamps[0].Name, "alex")
	}
	if stamps[0].File != "docs/streams/methodology/README.md" {
		t.Errorf("stamp File = %q, want %q", stamps[0].File, "docs/streams/methodology/README.md")
	}
	// (b)+(c) neither the prose line's `<name>` nor the binary file sneaks in.
	for _, s := range stamps {
		if s.Name == "<name>" || s.Name == "name" {
			t.Errorf("prose human:<name> line was read as a stamp: %+v", s)
		}
		if s.File == "docs/streams/methodology/diagram.png" {
			t.Errorf("binary file produced a stamp: %+v", s)
		}
	}
}

// TestPRFilesToDiff_DeleteRenameBinaryNoStamp covers the file shapes that
// dominate a delete-heavy PR: a deleted file (patch is
// only "-"/context lines), a large deletion with a null patch, a renamed file
// (no content change, null patch), and a binary/oversize file (null patch).
// None may break prFilesToDiff or produce a spurious human:<name> stamp — a
// stamp only ever lives on a "+"-added line, which none of these has.
func TestPRFilesToDiff_DeleteRenameBinaryNoStamp(t *testing.T) {
	// A null JSON `patch` must decode to the empty string, not panic. Build
	// these entries by unmarshalling the exact API shape (patch: null / absent)
	// so the test proves the decode path, not just a hand-set "".
	raw := `[
	  {"filename":"statusgen/deleted.go",
	   "patch":"@@ -1,3 +0,0 @@\n-package main\n-\n-func gone() { human:alex }"},
	  {"filename":"statusgen/big_deleted.go","patch":null},
	  {"filename":"statusgen/renamed.go"},
	  {"filename":"docs/diagram.png","patch":null}
	]`
	var files []ghPRFile
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files, want 4", len(files))
	}
	// The null / absent patch fields must have decoded to "".
	for _, f := range files[1:] {
		if f.Patch != "" && f.Filename != "statusgen/deleted.go" {
			// (renamed.go, big_deleted.go, diagram.png)
			t.Errorf("%s: patch = %q, want empty", f.Filename, f.Patch)
		}
	}

	diff := prFilesToDiff(files) // must not panic on the null patches
	stamps := stampsInDiff("", diff)
	// The only "human:alex" text lives on a DELETED ("-") line, which is not an
	// added line, so no stamp is produced.
	if len(stamps) != 0 {
		t.Errorf("got %d stamps, want 0 (delete/rename/binary carry no +added stamp): %+v",
			len(stamps), stamps)
	}
}

// TestPRFilesToDiff_LargeNPastCap exercises the real failure mode: a PR with far
// more than 300 files. The old `gh pr diff` path returned HTTP 406 at this size;
// the pure prFilesToDiff → stampsInDiff path has no such cap. Across 400 files:
//   - exactly ONE carries a real "+human:<name>" status-cell stamp (past the 300
//     boundary), and it must be the one and only stamp found;
//   - another file carries a PROSE "human:<name>" placeholder line, which must NOT
//     be read as a stamp — the distinction that let us reject a workflow-level
//     skip (a large PR's diff has prose human: lines) must survive reconstruction;
//   - another is a delete-only ("-" lines) file, contributing no added stamp.
func TestPRFilesToDiff_LargeNPastCap(t *testing.T) {
	const n = 400
	const stampedIdx = 317 // real stamp, well past the 300-file boundary
	const proseIdx = 101   // prose human:<name> placeholder — NOT a stamp
	const deleteIdx = 210  // delete-only file — NOT a stamp
	files := make([]ghPRFile, 0, n)
	for i := 0; i < n; i++ {
		f := ghPRFile{Filename: fmt.Sprintf("docs/streams/methodology/brief-%03d.md", i)}
		switch i {
		case stampedIdx:
			f.Patch = "@@ -1,1 +1,1 @@\n" +
				"-| 01 | brief | 0 | S | verified | 2026-07-10 opus-verifier | — |\n" +
				"+| 01 | brief | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 human:alex |"
		case proseIdx:
			// Prose placeholder: "human:" followed by "<" is not a stamp.
			f.Patch = "@@ -1,1 +1,2 @@\n context\n+each brief needs a `human:<name>` sign-off before done"
		case deleteIdx:
			// Delete-only hunk: the human: text is on a "-" line, not added.
			f.Patch = "@@ -1,2 +0,0 @@\n-old row | 2026-07-01 human:sam |\n-second removed line"
		default:
			f.Patch = "@@ -1,1 +1,1 @@\n context line unchanged"
		}
		files = append(files, f)
	}

	stamps := stampsInDiff("", prFilesToDiff(files))
	// (a) exactly the ONE real stamp across >300 files; (b) prose excluded;
	// (c) delete-only file emits none.
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps across %d files, want exactly 1: %+v", len(stamps), n, stamps)
	}
	if stamps[0].Name != "alex" {
		t.Errorf("stamp Name = %q, want alex", stamps[0].Name)
	}
	wantFile := fmt.Sprintf("docs/streams/methodology/brief-%03d.md", stampedIdx)
	if stamps[0].File != wantFile {
		t.Errorf("stamp File = %q, want %q", stamps[0].File, wantFile)
	}
	// Belt-and-braces: neither the prose nor the delete file is present.
	for _, s := range stamps {
		if s.File == fmt.Sprintf("docs/streams/methodology/brief-%03d.md", proseIdx) {
			t.Errorf("prose human:<name> line read as a stamp: %+v", s)
		}
		if s.File == fmt.Sprintf("docs/streams/methodology/brief-%03d.md", deleteIdx) {
			t.Errorf("delete-only line read as a stamp: %+v", s)
		}
	}
}

// TestPRFilesToDiff_HeaderPerFile asserts each filename yields exactly one
// "+++ b/<name>" header line, so stampsInDiff attributes added lines to the
// right file even when a file has no patch.
func TestPRFilesToDiff_HeaderPerFile(t *testing.T) {
	files := []ghPRFile{
		{Filename: "a/one.md", Patch: "@@ -1 +1 @@\n+hello\n"},
		{Filename: "b/two.png", Patch: ""},
	}
	diff := prFilesToDiff(files)
	for _, f := range files {
		want := "+++ b/" + f.Filename
		if strings.Count(diff, want) != 1 {
			t.Errorf("diff has %d %q header lines, want 1:\n%s",
				strings.Count(diff, want), want, diff)
		}
	}
}
