package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2026
// TestDeskevidenceLandsReadSetWithRow — the read-set introduction binding,
// landing half (#2026): a row naming a
// read-set file lands in ONE commit with that file, taken from the run's state
// directory `<git dir>/assay-read-sets/`, never from the worktree. The file is
// copied before the write, so a worktree removed while the landing is in
// flight cannot take it away. A file absent from the state directory, or one
// with another digest, refuses before any write; a could-not-check token names
// no file and lands alone.
func TestDeskevidenceLandsReadSetWithRow(t *testing.T) {
	const evidencePath = "docs/streams/x/brief.md"
	body := []byte("assay-read-set v1\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	readsPath := "docs/streams/x/reads/" + digest
	named := "(reads sha256:" + digest + ")"
	row := func(token string) string {
		return "# Brief\n\n## Evidence\n| 1 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | x @ 0123456789ab " + token + " |\n"
	}
	cases := []struct {
		name     string
		token    string
		state    []byte // nil: no file in the state directory
		worktree []byte // nil: no copy beside the brief
		noGit    bool   // the root is not a checkout, so it has no state directory
		landed   bool
		why      string
	}{
		{name: "lands with its row", token: named, state: body, worktree: []byte("a worktree copy is never read\n"), landed: true},
		{name: "absent from the state directory", token: named, worktree: body, why: "assay-read-sets/" + digest + " is absent"},
		{name: "another digest", token: named, state: []byte("other\n"), why: "hashes to "},
		{name: "no state directory", token: named, state: body, noGit: true, why: "state directory cannot be located"},
		{name: "could-not-check token", token: "(reads could-not-check: recorder is Linux-only (darwin))", landed: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, _ := setupFake(t)
			root := rootWithFile(t, evidencePath, row(c.token))
			put := func(p string, b []byte) {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stateDir := filepath.Join(root, ".git", "assay-read-sets")
			if !c.noGit {
				if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
					t.Fatalf("git init: %v %s", err, out)
				}
				got, err := readSetStateDir(root)
				if err != nil {
					t.Fatal(err)
				}
				stateDir = got
			}
			if c.state != nil {
				put(filepath.Join(stateDir, digest), c.state)
			}
			if c.worktree != nil {
				put(filepath.Join(root, filepath.FromSlash(readsPath)), c.worktree)
			}
			// Order: the landing copies the file before any worktree cleanup.
			// Remove the state directory, as `git worktree remove` would, at
			// the landing's first forge read: the landing must already hold
			// the file, so it copied it before any forge call or write.
			removed := false
			f.onRead = func() {
				if !removed {
					removed = true
					_ = os.RemoveAll(stateDir)
				}
			}
			f.setFile(evidencePath, "# Brief\n\n## Evidence\n")
			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
			if c.landed {
				if code != deskkit.ExitOK || f.putCalls != 1 || len(f.writes) != 1 {
					t.Fatalf("exit %d with %d write(s), want one commit", code, f.putCalls)
				}
				w := f.writes[0]
				if !strings.Contains(string(w.Content), c.token) {
					t.Fatalf("the commit does not carry the row: %s", w.Content)
				}
				if c.state == nil {
					if len(w.Companions) != 0 {
						t.Fatalf("a could-not-check row carried files %v", w.Companions)
					}
					return
				}
				if len(w.Companions) != 1 || w.Companions[0].File != readsPath || string(w.Companions[0].Content) != string(body) {
					t.Fatalf("the commit should carry %s from the state directory; carried %+v", readsPath, w.Companions)
				}
				if _, err := os.Stat(stateDir); !removed || !os.IsNotExist(err) {
					t.Fatalf("the state directory outlived the write, so the copy-before-cleanup order went unexercised")
				}
				return
			}
			if code != deskkit.ExitRefused {
				t.Fatalf("exit %d, want %d", code, deskkit.ExitRefused)
			}
			if f.putCalls != 0 || len(f.writes) != 0 {
				t.Fatalf("a refused read-set landing wrote: %d write(s)", f.putCalls)
			}
			for _, h := range f.hits {
				if strings.HasPrefix(h, "PUT ") {
					t.Fatalf("a refused read-set landing reached the write: %v", f.hits)
				}
			}
			if d := lastAudit(t).Detail; !strings.Contains(d, c.why) || !strings.Contains(d, "before the worktree is removed") {
				t.Fatalf("refusal %q lacks %q or the cleanup order", d, c.why)
			}
		})
	}
}

// TestDeskevidenceReadSetAlreadyLanded — a digest the branch already names was
// introduced by an earlier commit: its state file may be long gone, and a new
// row beside it still lands, carrying no file for it.
func TestDeskevidenceReadSetAlreadyLanded(t *testing.T) {
	const evidencePath = "docs/streams/x/brief.md"
	digest := strings.Repeat("a", 64)
	old := "# Brief\n\n## Evidence\n| 1 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | x @ 0123456789ab (reads sha256:" + digest + ") |\n"
	f, _ := setupFake(t)
	root := rootWithFile(t, evidencePath, old+"| 2 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | x @ 0123456789ab |\n")
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	f.setFile(evidencePath, old)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK || len(f.writes) != 1 || len(f.writes[0].Companions) != 0 {
		t.Fatalf("exit %d, writes %+v; want the new row alone", code, f.writes)
	}
}
