package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// truncatedcompare_test.go — the decision-gate board read when the forge's compare cannot
// list every changed file (#5913). The compare's file list stops at the forge's cap and a
// long interval leaves its commit list unread; neither is evidence of a board change. The
// board's directories are read at both commits instead, and only that read failing holds.

// capFiles is a compare file list at the forge's cap, every entry outside the board: the
// shape a busy branch produces once it moves past the cap.
func capFiles() []deskkit.ChangedFile {
	out := make([]deskkit.ChangedFile, 0, decisionGateCompareFileCap)
	for i := 0; i < decisionGateCompareFileCap; i++ {
		out = append(out, deskkit.ChangedFile{Filename: fmt.Sprintf("tools/pkg%03d/x.go", i), Status: "modified"})
	}
	return out
}

// boardTrees is a fake board listing: per commit, per board directory, the entries the
// directory holds. A commit not named serves the "*" listing.
type boardTrees map[string]map[string][]deskkit.TreeEntry

// serveTrees binds the tree-listing seam to trees (or to err when it is set) for the
// test, and returns the "<commit>:<dir>" reads it was asked for.
func serveTrees(t *testing.T, trees boardTrees, err error) *[]string {
	t.Helper()
	old := decisionGateTreeFn
	var calls []string
	decisionGateTreeFn = func(_ deskkit.Forge, _ deskkit.ForgeRepo, commit, dir string) ([]deskkit.TreeEntry, error) {
		calls = append(calls, commit+":"+dir)
		if err != nil {
			return nil, err
		}
		at, ok := trees[commit]
		if !ok {
			at = trees["*"]
		}
		return at[dir], nil
	}
	t.Cleanup(func() { decisionGateTreeFn = old })
	return &calls
}

func blob(p, oid string) deskkit.TreeEntry {
	return deskkit.TreeEntry{Path: p, Mode: "100644", OID: oid}
}

// TestBranchBoardTruncatedCompare pins the board read past the compare's cap: an
// unchanged board lands with no edits; a board file added, modified, removed or
// re-typed on the branch is returned exactly as a complete compare would have listed it;
// a listing that cannot be read, or a home that is not an ancestor of the tip, still
// holds.
// regression: #5913
func TestBranchBoardTruncatedCompare(t *testing.T) {
	const target = "docs/streams/sdlc/README.md"
	const brief = "docs/streams/sdlc/brief-18.md"
	const finding = "docs/streams/findings/2026-11-23-x.md"
	root, head := freshHome(t)
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	homeBoard := []deskkit.TreeEntry{blob(target, "a1"), blob(brief, "b1"), blob(finding, "f1")}
	type want struct {
		path   string
		body   string
		delete bool
	}
	cases := []struct {
		name    string
		mutate  func(f *fakeForge)
		tip     []deskkit.TreeEntry // docs/streams at the tip
		archive []deskkit.TreeEntry // docs/archive at the tip
		edits   []want
		wantErr string
	}{
		{"no board change past the cap", func(f *fakeForge) { f.cmpFiles = capFiles() },
			// The target and a register moved on the branch; neither is a board edit.
			[]deskkit.TreeEntry{blob(target, "a2"), blob(brief, "b1"), blob(finding, "f2")}, nil, nil, ""},
		{"a brief changed past the cap", func(f *fakeForge) {
			f.cmpFiles = capFiles()
			f.setFile(brief, "branch brief\n")
		}, []deskkit.TreeEntry{blob(target, "a1"), blob(brief, "b2"), blob(finding, "f1")}, nil,
			[]want{{path: brief, body: "branch brief\n"}}, ""},
		{"a brief added past the cap", func(f *fakeForge) {
			f.cmpFiles = capFiles()
			f.setFile("docs/streams/sdlc/brief-19.md", "new brief\n")
		}, append(append([]deskkit.TreeEntry{}, homeBoard...), blob("docs/streams/sdlc/brief-19.md", "c1")), nil,
			[]want{{path: "docs/streams/sdlc/brief-19.md", body: "new brief\n"}}, ""},
		{"a brief removed past the cap", func(f *fakeForge) { f.cmpFiles = capFiles() },
			[]deskkit.TreeEntry{blob(target, "a1"), blob(finding, "f1")}, nil,
			[]want{{path: brief, delete: true}}, ""},
		{"a brief archived past the cap", func(f *fakeForge) {
			f.cmpFiles = capFiles()
			f.setFile("docs/archive/sdlc/brief-18.md", "archived\n")
		}, []deskkit.TreeEntry{blob(target, "a1"), blob(finding, "f1")},
			[]deskkit.TreeEntry{blob("docs/archive/sdlc/brief-18.md", "b1")},
			[]want{{path: "docs/archive/sdlc/brief-18.md", body: "archived\n"}, {path: brief, delete: true}}, ""},
		{"a brief made a link past the cap", func(f *fakeForge) {
			f.cmpFiles = capFiles()
			f.setFile(brief, "the link target's bytes\n")
			f.fileKinds = map[string]string{brief: deskkit.FileKindSymlink}
		}, []deskkit.TreeEntry{blob(target, "a1"), {Path: brief, Mode: "120000", OID: "b1"}, blob(finding, "f1")}, nil,
			nil, "not a plain file"},
		{"the commit list is incomplete", func(f *fakeForge) {
			f.cmpIncomplete = true
			f.setFile(brief, "branch brief\n")
		}, []deskkit.TreeEntry{blob(target, "a1"), blob(brief, "b2"), blob(finding, "f1")}, nil,
			[]want{{path: brief, body: "branch brief\n"}}, ""},
		{"a changed brief cannot be read at the tip", func(f *fakeForge) { f.cmpFiles = capFiles() },
			[]deskkit.TreeEntry{blob(target, "a1"), blob(brief, "b2"), blob(finding, "f1")}, nil, nil, "cannot read"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeForge{commitSHAs: map[string]string{"main": "c0ffee1"}}
			c.mutate(f)
			calls := serveTrees(t, boardTrees{
				head:      {"docs/streams": homeBoard},
				"c0ffee1": {"docs/streams": c.tip, "docs/archive": c.archive},
			}, nil)
			edits, err := decisionGateBranchBoard(f, fr, root, "refs/heads/main", target)
			if c.wantErr != "" {
				if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got %v, want could-not-check containing %q", err, c.wantErr)
				}
			} else if err != nil {
				t.Fatalf("got %v, want no error (a truncated compare is read past, not held on)", err)
			}
			if len(edits) != len(c.edits) {
				t.Fatalf("edits = %+v, want %+v", edits, c.edits)
			}
			for i, w := range c.edits {
				if edits[i].Path != w.path || edits[i].Delete != w.delete || string(edits[i].Content) != w.body {
					t.Fatalf("edit %d = %+v, want %+v", i, edits[i], w)
				}
			}
			wantCalls := []string{head + ":docs/streams", "c0ffee1:docs/streams", head + ":docs/archive", "c0ffee1:docs/archive"}
			if c.wantErr == "" && strings.Join(*calls, ",") != strings.Join(wantCalls, ",") {
				t.Fatalf("tree reads = %v, want %v (both board directories at the home HEAD and the resolved tip)", *calls, wantCalls)
			}
			for _, r := range f.reads {
				if r.Ref != "c0ffee1" {
					t.Fatalf("read %s at %q, want the resolved tip c0ffee1", r.File, r.Ref)
				}
			}
		})
	}
	t.Run("a board listing cannot be read", func(t *testing.T) {
		f := &fakeForge{commitSHAs: map[string]string{"main": "c0ffee1"}, cmpFiles: capFiles()}
		serveTrees(t, nil, deskkit.Unverifiable("listing truncated", nil))
		edits, err := decisionGateBranchBoard(f, fr, root, "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "cannot be shown complete") || len(edits) != 0 {
			t.Fatalf("got edits %+v, err %v; want could-not-check (the fallback read is itself incomplete)", edits, err)
		}
	})
	t.Run("the production listing on a non-GitHub forge holds", func(t *testing.T) {
		// No seam: the fake is not a GitHub backend, so the listing is could-not-check
		// and the truncated compare holds as before.
		f := &fakeForge{commitSHAs: map[string]string{"main": "c0ffee1"}, cmpFiles: capFiles()}
		_, err := decisionGateBranchBoard(f, fr, root, "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "cannot be shown complete") {
			t.Fatalf("got %v, want could-not-check", err)
		}
	})
	t.Run("truncated and not an ancestor", func(t *testing.T) {
		f := &fakeForge{commitSHAs: map[string]string{"main": "c0ffee1"}, cmpFiles: capFiles(), cmpBehind: 1,
			compareStatus: map[string]string{"example-org/tracker:" + head + "...c0ffee1": "diverged"}}
		calls := serveTrees(t, boardTrees{"*": {"docs/streams": homeBoard}}, nil)
		_, err := decisionGateBranchBoard(f, fr, root, "main", target)
		if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "not an ancestor") {
			t.Fatalf("got %v, want could-not-check naming the ancestry", err)
		}
		if len(*calls) != 0 {
			t.Fatalf("tree reads %v made for a home that is not an ancestor of the tip", *calls)
		}
	})
}

// TestTruncatedCompareWholeVerb drives the whole verb with the real statusgen and the
// real judge on a branch that moved past the compare's cap. With no board change on the
// branch the landing lands; with the brief relabelled gate: human on the branch the
// unruled move is refused exactly as it is under a complete compare.
// regression: #5913
func TestTruncatedCompareWholeVerb(t *testing.T) {
	const brief = "docs/streams/sdlc/brief-18.md"
	t.Run("no board change past the cap lands", func(t *testing.T) {
		g, f, args := landingStaleHome(t, "model", briefOf("model", ""))
		f.cmpFiles = capFiles()
		home := []deskkit.TreeEntry{blob(g.readme, "r1"), blob(brief, "b1")}
		serveTrees(t, boardTrees{"*": {"docs/streams": home}}, nil)
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want %d (a branch past the compare's cap with no board change must land); audit: %s", code, deskkit.ExitOK, auditDetail(t))
		}
		if f.putCalls != 1 {
			t.Fatalf("landing wrote %d time(s), want 1", f.putCalls)
		}
	})
	t.Run("a board change past the cap is still refused", func(t *testing.T) {
		g, f, args := landingStaleHome(t, "model", briefOf("human", ""))
		f.cmpFiles = capFiles()
		head := gitHead(t, g.root)
		serveTrees(t, boardTrees{
			head: {"docs/streams": {blob(g.readme, "r1"), blob(brief, "b1")}},
			"*":  {"docs/streams": {blob(g.readme, "r1"), blob(brief, "b2")}},
		}, nil)
		if code := run(args); code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (the branch relabelled the brief gate: human); audit: %s", code, deskkit.ExitRefused, auditDetail(t))
		}
		if f.putCalls != 0 {
			t.Fatalf("refused landing still wrote %d time(s)", f.putCalls)
		}
		if d := lastAudit(t).Detail; !strings.Contains(d, "sdlc/18"+decisionGateRefusedMark) {
			t.Fatalf("audit detail = %q, want the REFUSED line", d)
		}
		if b, _ := os.ReadFile(filepath.Join(g.root, brief)); string(b) != briefOf("model", "") {
			t.Fatalf("the home's brief was moved: %q", b)
		}
	})
}

// auditDetail is the last audit entry's detail, or "" when the verb wrote none.
func auditDetail(t *testing.T) string {
	t.Helper()
	if e := auditEntries(t); len(e) > 0 {
		return e[len(e)-1].Detail
	}
	return ""
}

// gitHead is root's HEAD commit.
func gitHead(t *testing.T, root string) string {
	t.Helper()
	h, err := landingHead(root)
	if err != nil || h == "" {
		t.Fatalf("landing HEAD of %s: %q, %v", root, h, err)
	}
	return h
}

// TestBoardDirsMatchBoardRe pins the selector built from decisionGateBoardDirs: it selects
// a board file under each listed directory and nothing beside them, and the registers it
// leaves out stay left out.
func TestBoardDirsMatchBoardRe(t *testing.T) {
	cases := map[string]bool{
		"docs/streams/sdlc/brief-18.md":         true,
		"docs/archive/sdlc/brief-18.md":         true,
		"docs/streams/README.md":                true,
		"docs/streamsx/a.md":                    false,
		"docs/stream/a.md":                      false,
		"docs/archives/a.md":                    false,
		"docs/other/a.md":                       false,
		"x/docs/streams/a.md":                   false,
		"docs/streams":                          false,
		"docsXstreams/a.md":                     false,
		"docs/streams/findings/2026-11-23-x.md": true,
	}
	for p, want := range cases {
		if got := decisionGateBoardRe.MatchString(p); got != want {
			t.Errorf("decisionGateBoardRe(%q) = %v, want %v", p, got, want)
		}
	}
	for _, p := range []string{"docs/streams/findings/x.md", "docs/archive/intake/x.md", "docs/streams/verify-outcomes/x.json"} {
		if !decisionGateRegisterRe.MatchString(p) {
			t.Errorf("register %q is not left out of the board", p)
		}
	}
	if decisionGateRegisterRe.MatchString("docs/streams/sdlc/findings.md") {
		t.Error("a brief named findings.md is read as a register")
	}
}
