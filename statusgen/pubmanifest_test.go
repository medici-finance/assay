package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// privateStreamFixture builds a root containing a docs/streams/<name>-private/
// directory and (optionally) a docs/publication-manifest.yaml with the given
// body. It returns the root and the *-private Stream pointed at its real dir.
func privateStreamFixture(t *testing.T, name, manifest string) (string, *Stream) {
	t.Helper()
	root := t.TempDir()
	privDir := filepath.Join(root, "docs", "streams", name+"-private")
	if err := os.MkdirAll(privDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "publication-manifest.yaml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, &Stream{Name: name + "-private", Dir: privDir}
}

const wholesaleWithhold = `schema: publication-manifest-v1
rows:
  - path: docs/streams/
    kind: tree
    disposition: do-not-copy
    reason: the 2026-08-10 pivot — the whole operating-stream tree is withheld
`

// TestPrivateDirDoNotCopyWholesaleWithholdClean: a *-private dir covered by the
// wholesale docs/streams/ do-not-copy prefix resolves to do-not-copy — clean.
func TestPrivateDirDoNotCopyWholesaleWithholdClean(t *testing.T) {
	root, s := privateStreamFixture(t, "methodology", wholesaleWithhold)
	problems, notices := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 0 {
		t.Errorf("a *-private dir under the wholesale do-not-copy prefix must be clean; got %v", problems)
	}
	if len(notices) != 0 {
		t.Errorf("no could-not-check expected with a present, parseable manifest; got %v", notices)
	}
}

// TestPrivateDirDoNotCopyExactRowClean: an exact do-not-copy row on the
// *-private dir itself also resolves clean (exact match wins over a prefix).
func TestPrivateDirDoNotCopyExactRowClean(t *testing.T) {
	manifest := `schema: publication-manifest-v1
rows:
  - path: docs/streams/
    kind: tree
    disposition: copy
    reason: (hypothetical) the streams tree is public
  - path: docs/streams/methodology-private/
    kind: tree
    disposition: do-not-copy
    reason: this private sibling is withheld even though the tree above is copy
`
	root, s := privateStreamFixture(t, "methodology", manifest)
	problems, _ := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 0 {
		t.Errorf("an exact do-not-copy row (longest match) must win and be clean; got %v", problems)
	}
}

// TestPrivateDirDoNotCopyCopyDispositionReds: a *-private dir that resolves to a
// COPY disposition is a hard PROBLEM — the leak the invariant exists to prevent.
func TestPrivateDirDoNotCopyCopyDispositionReds(t *testing.T) {
	manifest := `schema: publication-manifest-v1
rows:
  - path: docs/streams/methodology-private/
    kind: tree
    disposition: copy
    reason: MIS-SET — a private stream marked copy, which must red
`
	root, s := privateStreamFixture(t, "methodology", manifest)
	problems, _ := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 1 {
		t.Fatalf("a *-private dir marked copy must produce exactly 1 problem; got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "methodology-private") || !strings.Contains(problems[0], "copy") {
		t.Errorf("problem must name the dir and its bad disposition; got %q", problems[0])
	}
}

// TestPrivateDirDoNotCopyUncoveredReds: a *-private dir that NO manifest row
// covers is a PROBLEM — the assertion is "covered by a do-not-copy
// disposition", so an unrowed private dir is not machine-guaranteed withheld.
func TestPrivateDirDoNotCopyUncoveredReds(t *testing.T) {
	manifest := `schema: publication-manifest-v1
rows:
  - path: README.md
    kind: file
    disposition: copy
    reason: unrelated row; nothing covers docs/streams/
`
	root, s := privateStreamFixture(t, "methodology", manifest)
	problems, _ := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 1 {
		t.Fatalf("an uncovered *-private dir must produce exactly 1 problem; got %d: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "covered by NO") {
		t.Errorf("problem must say the dir is covered by no disposition; got %q", problems[0])
	}
}

// TestPrivateDirDoNotCopyNoManifestIsCouldNotCheck: a *-private dir with no
// publication manifest present cannot be verified — a could-not-check NOTICE,
// never a silent pass.
func TestPrivateDirDoNotCopyNoManifestIsCouldNotCheck(t *testing.T) {
	root, s := privateStreamFixture(t, "methodology", "") // no manifest written
	problems, notices := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 0 {
		t.Errorf("a missing manifest must not be a hard PROBLEM; got %v", problems)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "could-not-check") {
		t.Errorf("want a could-not-check NOTICE when the manifest is absent; got %v", notices)
	}
}

// TestPrivateDirDoNotCopyUnparseableManifestIsCouldNotCheck: a present but
// malformed manifest is could-not-check, not a false pass or a false fail.
func TestPrivateDirDoNotCopyUnparseableManifestIsCouldNotCheck(t *testing.T) {
	root, s := privateStreamFixture(t, "methodology", "schema: x\nrows: [ this is : not valid yaml")
	problems, notices := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 0 {
		t.Errorf("an unparseable manifest must not be a hard PROBLEM; got %v", problems)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "could not be parsed") {
		t.Errorf("want a could-not-check NOTICE for the unparseable manifest; got %v", notices)
	}
}

// TestPrivateDirDoNotCopyInertWithoutPrivateStream: a repo with no *-private
// stream never loads the manifest and emits nothing (the public-repo posture).
func TestPrivateDirDoNotCopyInertWithoutPrivateStream(t *testing.T) {
	root := t.TempDir()
	s := &Stream{Name: "methodology", Dir: filepath.Join(root, "docs", "streams", "methodology")}
	problems, notices := privateStreamDoNotCopyProblems(root, []*Stream{s})
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("no *-private stream → the lint must be fully inert; got problems=%v notices=%v", problems, notices)
	}
}

// TestPrivateDirDoNotCopyResolvePrecedence: the resolver picks the longest (most
// specific) matching path — exact over prefix, longer prefix over shorter.
func TestPrivateDirDoNotCopyResolvePrecedence(t *testing.T) {
	m := &pubManifest{Rows: []pubManifestRow{
		{Path: "docs/", Disposition: "copy"},
		{Path: "docs/streams/", Disposition: "do-not-copy"},
		{Path: "docs/streams/methodology-private/", Disposition: "relocate"},
	}}
	row, ok := m.resolveDir("docs/streams/methodology-private/")
	if !ok || row.Path != "docs/streams/methodology-private/" {
		t.Errorf("exact/longest match must win; got ok=%v row=%+v", ok, row)
	}
	row, ok = m.resolveDir("docs/streams/other-private/")
	if !ok || row.Path != "docs/streams/" {
		t.Errorf("longest covering prefix must win; got ok=%v row=%+v", ok, row)
	}
}
