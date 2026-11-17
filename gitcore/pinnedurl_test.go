package gitcore

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

const (
	pinnedShared = "https://example.com/example-org/example-repo.git"
	pinnedURL    = "https://example.com/example-org/example-pinned.git"
)

// pinnedFixture is a repository whose origin is pinned per worktree by config.worktree.
func pinnedFixture(t *testing.T, worktreeConfig string) string {
	t.Helper()
	f := gittest.NewFixture(t)
	for _, args := range [][]string{
		{"remote", "add", "origin", pinnedShared},
		{"config", "extensions.worktreeConfig", "true"},
	} {
		if _, err := f.Git(args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.Dir, ".git", "config.worktree"), []byte(worktreeConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return f.Dir
}

func pinnedRead(t *testing.T, dir string) ([]string, string, error) {
	t.Helper()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo.PinnedRemoteURLs("origin")
}

// regression: #6736
func TestPinnedURLBareKeyRefused(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"bare", "[remote \"origin\"]\n\turl\n\turl = " + pinnedURL + "\n"},
		{"bare-comment", "[remote \"origin\"]\n\turl ; no value\n\turl = " + pinnedURL + "\n"},
		{"bare-hash-comment", "[remote \"origin\"]\n\turl # no value\n\turl = " + pinnedURL + "\n"},
		{"bare-upper", "[remote \"origin\"]\n\tURL\n\turl = " + pinnedURL + "\n"},
		{"bare-section-case", "[REMOTE \"origin\"]\n\turl\n\turl = " + pinnedURL + "\n"},
		{"bare-crlf", "[remote \"origin\"]\r\n\turl\r\n\turl = " + pinnedURL + "\r\n"},
		{"bare-split-sections", "[remote \"origin\"]\n\turl\n[remote \"origin\"]\n\turl = " + pinnedURL + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := pinnedFixture(t, tc.body)
			// Native control: git does not read a valueless url as the reset.
			if out, err := exec.Command("git", "-C", dir, "remote", "get-url", "--all", "origin").CombinedOutput(); err == nil && strings.TrimSpace(string(out)) == pinnedURL {
				t.Fatalf("native git resolved the bare-key shape to the pinned URL alone: %q", out)
			}
			local, pinned, err := pinnedRead(t, dir)
			if !errors.Is(err, ErrRemoteScopeUnmerged) {
				t.Fatalf("bare url key accepted as the reset: local=%q pinned=%q err=%v", local, pinned, err)
			}
			if !strings.Contains(err.Error(), "has no value") {
				t.Fatalf("refusal does not name the valueless shape: %v", err)
			}
			if pinned != "" || local != nil || strings.Contains(err.Error(), "example.com") {
				t.Fatalf("refusal carried a value: local=%q pinned=%q err=%v", local, pinned, err)
			}
		})
	}
}

// regression: #6736
func TestPinnedURLResetAccepted(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"reset", "[remote \"origin\"]\n\turl =\n\turl = " + pinnedURL + "\n"},
		{"reset-quoted", "[remote \"origin\"]\n\turl = \"\"\n\turl = " + pinnedURL + "\n"},
		{"reset-comment", "[remote \"origin\"]\n\turl = ; reset\n\turl = " + pinnedURL + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := pinnedFixture(t, tc.body)
			out, err := exec.Command("git", "-C", dir, "remote", "get-url", "--all", "origin").Output()
			if err != nil || strings.TrimSpace(string(out)) != pinnedURL {
				t.Fatalf("native control: git resolved %q err=%v", out, err)
			}
			local, pinned, err := pinnedRead(t, dir)
			if err != nil {
				t.Fatalf("explicit reset refused: %v", err)
			}
			if pinned != pinnedURL || len(local) != 1 || local[0] != pinnedShared {
				t.Fatalf("local=%q pinned=%q", local, pinned)
			}
		})
	}
}
