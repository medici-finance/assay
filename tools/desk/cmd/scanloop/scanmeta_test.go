package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// scanmeta_test.go — the lane's own files never land inside the scan worktree.
//
// The incident: every pass ended `dispatch-error: git worktree remove … contains modified or
// untracked files`, exit 6, with the scan worktree left behind. The lane wrote the derived title
// and body INTO the worktree, and git refuses to remove a worktree holding untracked files. The
// class: any file the lane itself writes inside the worktree makes the final removal fail.

// worktreeGit models the one git semantic the incident turned on, against the REAL filesystem:
// `git worktree remove` refuses a worktree that holds any untracked file. The scanner and commit
// are modelled as leaving the tree clean (their output is committed), so the only way an entry is
// left in the worktree is a file the lane wrote there itself.
type worktreeGit struct {
	bodies map[string]string // deskpr verb -> the body file's content at the time it ran
}

func (g *worktreeGit) exec(_ string, name string, args ...string) (string, error) {
	switch name {
	case "git":
		if len(args) >= 3 && args[0] == "worktree" && args[1] == "add" {
			return "", os.MkdirAll(args[2], 0o700)
		}
		if len(args) >= 3 && args[0] == "worktree" && args[1] == "remove" {
			entries, err := os.ReadDir(args[2])
			if err != nil {
				return "", err
			}
			if len(entries) > 0 {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				return "", fmt.Errorf("fatal: '%s' contains modified or untracked files, use --force to delete it (%s)",
					args[2], strings.Join(names, ", "))
			}
			return "", os.Remove(args[2])
		}
	case "deskscanbody":
		if args[0] == "emit" {
			return "intake scan: 3 created, 0 retired", nil
		}
	case "deskpr":
		if g.bodies == nil {
			g.bodies = map[string]string{}
		}
		for i, a := range args {
			if a == "--body-file" && i+1 < len(args) {
				b, err := os.ReadFile(args[i+1])
				if err != nil {
					return "", fmt.Errorf("deskpr %s: the body file is not readable: %w", args[0], err)
				}
				g.bodies[args[0]] = string(b)
			}
		}
	}
	return "", nil
}

// leftovers lists every entry under dir, so a test can assert the run put everything back.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	sort.Strings(got)
	return got
}

// regression: scan worktree removal failed on the lane's own files. A real (non-dry-run) scan
// carry must end with the isolated worktree REMOVED, on both the fresh and the coalesce path, and
// leave nothing of its own behind.
func TestScanMeta_WorktreeRemovable(t *testing.T) {
	for _, tc := range []struct {
		name string
		open *OpenScanPR
		verb string
	}{
		{"fresh", nil, "create"},
		{"coalesce", openPR(3 * time.Minute), "edit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &worktreeGit{}
			req := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), tc.open)
			req.DryRun = false
			if tc.open != nil {
				req.Branch = tc.open.Branch
			}
			out, err := scanCarrierPRLane{exec: g.exec, write: writeTextFile}.Execute(req)
			if err != nil {
				t.Fatalf("the scan carry failed at the end: %v\n%s", err, strings.Join(out.Steps, "\n"))
			}
			if _, serr := os.Stat(req.Worktree); !os.IsNotExist(serr) {
				t.Fatalf("the scan worktree %s was left behind (stat: %v)", req.Worktree, serr)
			}
			if left := leftovers(t, filepath.Dir(req.Worktree)); len(left) != 0 {
				t.Fatalf("the run left %q behind beside the worktree", left)
			}
			if got := g.bodies[tc.verb]; got != "intake scan: 3 created, 0 retired" {
				t.Fatalf("deskpr %s read body %q — the derived body must still be in the file it is handed", tc.verb, got)
			}
		})
	}
}

// TestScanMeta_NeverInside is the class invariant, independent of git's behaviour: no
// file the lane writes may sit inside the scan worktree. A future write added there would make the
// final `git worktree remove` fail again, and this test names it before git does.
func TestScanMeta_NeverInside(t *testing.T) {
	for _, open := range []*OpenScanPR{nil, openPR(3 * time.Minute)} {
		req := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), open)
		req.DryRun = false
		g := &worktreeGit{}
		var written []string
		write := func(path, content string) error {
			written = append(written, path)
			return writeTextFile(path, content)
		}
		_, err := (scanCarrierPRLane{exec: g.exec, write: write}).Execute(req)
		// The invariant is checked FIRST, so it names the offending write itself rather than the
		// downstream git refusal it causes.
		if len(written) == 0 {
			t.Fatal("the lane wrote no files — this invariant would be vacuous")
		}
		inside := filepath.Clean(req.Worktree) + string(filepath.Separator)
		for _, p := range written {
			if strings.HasPrefix(filepath.Clean(p), inside) {
				t.Fatalf("the lane wrote %s INSIDE the scan worktree — `git worktree remove` refuses an untracked file", p)
			}
		}
		if err != nil {
			t.Fatalf("lane errored: %v", err)
		}
	}
}

// TestScanMeta_RemovedOnFailure — a carry that fails after the title and body were written (here:
// the PR verb refuses) still removes them; a failed pass must not leave its own files behind.
func TestScanMeta_RemovedOnFailure(t *testing.T) {
	req := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), nil)
	req.DryRun = false
	g := &worktreeGit{}
	failing := func(dir, name string, args ...string) (string, error) {
		if name == "deskpr" {
			return "", fmt.Errorf("deskpr %s: refused", args[0])
		}
		return g.exec(dir, name, args...)
	}
	if _, err := (scanCarrierPRLane{exec: failing, write: writeTextFile}).Execute(req); err == nil {
		t.Fatal("the lane reported success although the PR verb refused")
	}
	// The worktree itself stays for inspection on a failed carry; what is checked is that the
	// lane's OWN files are gone — beside it and inside it.
	for _, name := range leftovers(t, filepath.Dir(req.Worktree)) {
		if filepath.Join(filepath.Dir(req.Worktree), name) == req.Worktree {
			continue
		}
		t.Fatalf("a failed carry left its own file %s beside the worktree", name)
	}
	if left := leftovers(t, req.Worktree); len(left) != 0 {
		t.Fatalf("a failed carry left %q inside the worktree", left)
	}
}

// regression: #6282
//
// TestScanMetaRefusesSymlink — the title and body files must not sit at a name another local
// account can predict in the shared tmp base. Before a carry, a symlink to a victim file is planted
// at the old body path beside the worktree and a file of someone else's at the old title path. The
// carry must write neither through the symlink nor over the planted file; it writes only inside a
// private 0700 directory of its own, away from the worktree's parent, and removes that directory.
func TestScanMetaRefusesSymlink(t *testing.T) {
	for _, tc := range []struct {
		name string
		open *OpenScanPR
	}{
		{"fresh", nil},
		{"coalesce", openPR(3 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), tc.open)
			req.DryRun = false
			if tc.open != nil {
				req.Branch = tc.open.Branch
			}
			victim := filepath.Join(t.TempDir(), "victim")
			if err := os.WriteFile(victim, []byte("victim\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			wt := filepath.Clean(req.Worktree)
			if err := os.Symlink(victim, wt+".scan-body"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(wt+".scan-title", []byte("planted\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			var dirs []string
			write := func(path, content string) error {
				dir := filepath.Dir(path)
				if dir == filepath.Dir(wt) {
					t.Errorf("the lane wrote %s in the worktree's parent, the shared tmp base", path)
				}
				fi, err := os.Lstat(dir)
				if err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
					t.Errorf("the lane wrote %s in %s, which is not a private 0700 directory (%v)", path, dir, err)
				}
				dirs = append(dirs, dir)
				return writeTextFile(path, content)
			}
			g := &worktreeGit{}
			out, err := (scanCarrierPRLane{exec: g.exec, write: write}).Execute(req)
			if err != nil {
				t.Fatalf("the scan carry failed: %v\n%s", err, strings.Join(out.Steps, "\n"))
			}
			if b, _ := os.ReadFile(victim); string(b) != "victim\n" {
				t.Fatalf("the carry wrote through the planted symlink: the victim file now reads %q", b)
			}
			if b, _ := os.ReadFile(wt + ".scan-title"); string(b) != "planted\n" {
				t.Fatalf("the carry overwrote the planted file: it now reads %q", b)
			}
			if len(dirs) == 0 {
				t.Fatal("the lane wrote no files — this test would be vacuous")
			}
			for _, d := range dirs {
				if _, serr := os.Stat(d); !os.IsNotExist(serr) {
					t.Fatalf("the private metadata directory %s was left behind (stat: %v)", d, serr)
				}
			}
		})
	}
}
