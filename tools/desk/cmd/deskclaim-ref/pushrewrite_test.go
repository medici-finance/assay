package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// regression: #3281
func TestClaimOriginPushRewrite(t *testing.T) {
	regression.IsolateGit(t)
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	const target = "https://gitlab.example.invalid/team/project.git"
	const raw = "[core]\nrepositoryformatversion = 0\n" +
		"[remote \"origin\"]\nurl = " + target + "\n" +
		"[url \"https://push.example.invalid/\"]\npushInsteadOf = unmatched:\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "nested")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, subdir} {
		if got := gogitOriginURL(path); got != target {
			t.Fatalf("gogitOriginURL = %q; want %q", got, target)
		}
	}
}
