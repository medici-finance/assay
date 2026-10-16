package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Reader tests for the decision-gate hold (lifecycle-v1 §4.5): the base is read
// from the git tree and the change from the working tree, and both must read a
// board record by ONE rule. A board record that is not a plain file or
// directory in the tree — the board's own readers follow it, the git tree does
// not — is refused, never taken as carrying no gate. Each route below is two
// changes; the first lands on the base, the second is judged against it.

// dhSymlinks skips on a platform whose checkout does not keep symbolic links.
func dhSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links in a git checkout are not portable to Windows")
	}
}

// dhLink replaces rel (repo-relative) with a symbolic link to target.
func dhLink(t *testing.T, root, rel, target string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.RemoveAll(p); err != nil {
		t.Fatal(err)
	}
	mustMkdirAll(t, filepath.Dir(p))
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}

// dhLand commits the working tree and makes it the base the next change is
// judged against — a first change that has landed.
func dhLand(t *testing.T, root string) {
	t.Helper()
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "landed change")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
}

const dhLinkTarget = "docs/streams/" + dhStream + "/brief-18.txt"

// dhLinkedBrief is the first change of each route: the unruled gate: human
// brief's file becomes a symbolic link to a plain file holding the same
// frontmatter, so the board still reads the same brief.
func dhLinkedBrief(t *testing.T, root string) {
	t.Helper()
	dhWrite(t, root, map[string]string{dhLinkTarget: dhFM("gate: human", "decision-issue: 41\n")})
	dhLink(t, root, dhBriefPath, "brief-18.txt")
}

// The first change alone is refused: it leaves a board record the hold cannot
// read the way the board reads it.
func TestDGLinkedBriefRefused(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhLinkedBrief(t, root)
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "cannot read the way the board reads it")
}

// Two changes, relabel and move: after the link has landed, the linked-to file
// is relabelled away from human and the row moved to implemented, unruled.
// Refused — the base's linked record is not read as carrying no gate.
func TestDGLinkedBriefRelabelMoveRefused(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhLinkedBrief(t, root)
	dhLand(t, root)
	dhWrite(t, root, map[string]string{
		dhLinkTarget: dhFM("gate: model", "decision-issue: 41\n"),
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
	})
	dhRefusedBoth(t, root, dhBoardID)
}

// Two changes, drop: after the link has landed, the brief's row, its link and
// the linked-to file are removed, unruled. Refused as a drop.
func TestDGLinkedBriefDropRefused(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhLinkedBrief(t, root)
	dhLand(t, root)
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(), dhBriefPath: "", dhLinkTarget: ""})
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "drops it")
}

// Matched control: replacing the landed link with a plain file holding the
// same unruled gate: human record, row unchanged, is not refused — the base's
// unreadable record reads conservatively, not as a fault in itself.
func TestDGLinkedBriefRepairedQuiet(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhLinkedBrief(t, root)
	dhLand(t, root)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(dhBriefPath))); err != nil {
		t.Fatal(err)
	}
	dhWrite(t, root, map[string]string{dhBriefPath: dhFM("gate: human", "decision-issue: 41\n"), dhLinkTarget: ""})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("repairing a linked brief in place was refused: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("repairing a linked brief in place turned the network lane red (exit %d):\n%s", code, out)
	}
}

// Class guard: for a committed tree with nothing changed, the base reader (git
// tree) and the working-tree reader produce the same record set for every key —
// one reading rule — at every board position a link or a nested repository
// can sit, with a plain control record beside each planted one.
func TestDGReadersAgree(t *testing.T) {
	dhSymlinks(t)
	type plant struct {
		name string
		do   func(t *testing.T, root string)
	}
	link := func(rel, target string) func(*testing.T, string) {
		return func(t *testing.T, root string) { dhLink(t, root, rel, target) }
	}
	elsewhere := map[string]string{
		"elsewhere/brief-18.md":       dhFM("gate: human", "decision-issue: 41\n"),
		"elsewhere/README.md":         dhReadme(dhRow("18", "todo")),
		"elsewhere/done/brief-19.md":  dhBrief19(""),
		"elsewhere/sdlc/README.md":    dhReadme(dhRow("18", "todo")),
		"elsewhere/sdlc/brief-18.md":  dhFM("gate: human", "decision-issue: 41\n"),
		"elsewhere/tree/sdlc/x.md":    "x\n",
		"elsewhere/docs/streams/a.md": "x\n",
	}
	plants := []plant{
		{"brief file", link(dhBriefPath, "../../../elsewhere/brief-18.md")},
		{"stream index", link(dhReadmePath, "../../../elsewhere/README.md")},
		{"done folder", link("docs/streams/"+dhStream+"/done", "../../../elsewhere/done")},
		{"brief in done", link("docs/streams/"+dhStream+"/done/brief-19.md", "../../../../elsewhere/done/brief-19.md")},
		{"stream directory", link("docs/streams/linked", "../../elsewhere/sdlc")},
		{"archive tree", link("docs/archive", "../elsewhere/tree")},
		{"streams tree", func(t *testing.T, root string) {
			// docs/streams itself a link, with the board's files behind it.
			if err := os.Rename(filepath.Join(root, "docs", "streams"), filepath.Join(root, "elsewhere", "streams")); err != nil {
				t.Fatal(err)
			}
			dhLink(t, root, "docs/streams", "../elsewhere/streams")
		}},
		{"nested repository", func(t *testing.T, root string) {
			sub := filepath.Join(root, "docs", "streams", "nested")
			mustMkdirAll(t, sub)
			gitRun(t, sub, "init", "-q")
			dhWrite(t, root, map[string]string{"docs/streams/nested/README.md": dhReadme(dhRow("1", "todo"))})
			gitRun(t, sub, "add", "-A")
			gitRun(t, sub, "commit", "-q", "-m", "nested")
		}},
	}
	for _, pl := range plants {
		t.Run(pl.name, func(t *testing.T) {
			files := unruledBase()
			files["docs/streams/"+dhStream+"/done/brief-19.md"] = dhBrief19("")
			files["docs/streams/control/README.md"] = dhReadme(dhRow("1", "todo"))
			for k, v := range elsewhere {
				files[k] = v
			}
			root := dhFixture(t, files)
			pl.do(t, root)
			dhLand(t, root)
			base, head := mustGateAtRev(t, root), gateSnapshotOnDisk(root)
			keys := map[string]bool{}
			for k := range base {
				keys[k] = true
			}
			for k := range head {
				keys[k] = true
			}
			for k := range keys {
				var bf, hf string
				if base[k] != nil {
					bf = base[k].fingerprint()
				}
				if head[k] != nil {
					hf = head[k].fingerprint()
				}
				if bf != hf {
					t.Errorf("%s: the two readers disagree on %s:\n  base: %q\n  head: %q", pl.name, k, bf, hf)
				}
			}
			unreadable := false
			for _, b := range base {
				if strings.Contains(b.fingerprint(), "unreadable ") {
					unreadable = true
				}
			}
			if !unreadable {
				t.Errorf("%s: the planted record is not read as unreadable", pl.name)
			}
			if c := base["control/1"]; pl.name != "streams tree" && (c == nil || strings.Contains(c.fingerprint(), "unreadable")) {
				t.Errorf("%s: the plain control stream is not read plainly: %+v", pl.name, c)
			}
			dhSeams(t, nil)
			if p := dhLayerOne(t, root); len(p) == 0 {
				t.Errorf("%s: layer one does not refuse a board holding the planted record", pl.name)
			}
		})
	}
}

// A scope the base could not enumerate: after a done/ folder that is a link
// (holding an unruled gate: human brief) has landed, the change removes the
// link and what it pointed at. Nothing after the change stands for the base's
// unreadable done/ folder, so the change is refused, not judged as dropping
// nothing.
func TestDGLinkedDoneFolderRemovedRefused(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:       dhReadme(dhRow("18", "todo"), dhRow("19", "todo")),
		dhBriefPath:        dhFM("gate: human", "decision-issue: 41\n"),
		"held/brief-19.md": dhBrief19(""),
	})
	dhLink(t, root, "docs/streams/"+dhStream+"/done", "../../../held")
	dhLand(t, root)
	dhWrite(t, root, map[string]string{"held/brief-19.md": ""})
	if err := os.Remove(filepath.Join(root, "docs", "streams", dhStream, "done")); err != nil {
		t.Fatal(err)
	}
	p := dhLayerOne(t, root)
	wantContains(t, "layer-one refusals", strings.Join(p, "\n"), "docs/streams/"+dhStream+"/done", "could not read the way the board reads it")
	if code, out := dhGate(t, root); code == 0 {
		t.Errorf("--decision-gate exit 0, want a refusal of the unreadable base done/ folder:\n%s", out)
	}
}
