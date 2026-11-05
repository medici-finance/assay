package credential

// sidecar_test.go — desktools-v2/58 review advisory A3: the `.owner` and `.perms` sidecars
// beside a token cache are written through the same atomic writer as the token itself.
// Internal test package: both writers are unexported.

import (
	"os"
	"path/filepath"
	"testing"
)

// regression: desktools-v2/58 — the owner and perms sidecars were written in place with
// os.WriteFile, so a link planted at the sidecar path was written through (the target file
// was overwritten) and a pre-existing wider mode was kept. Each write must replace the path
// with a fresh 0600 regular file and leave any link target untouched.
func TestSidecarWritesAreAtomic(t *testing.T) {
	cases := []struct {
		name  string
		path  func(tokenPath string) string
		write func(tokenPath string)
		want  string
	}{
		{"owner", OwnerSidecarPath, func(tp string) { writeOwnerSidecar(tp, "app", "acct") }, "app acct\n"},
		{"perms", PermsPath, func(tp string) { writePerms(tp, map[string]string{"issues": "write"}) }, `{"issues":"write"}`},
	}
	for _, c := range cases {
		t.Run(c.name+"/link", func(t *testing.T) {
			dir := t.TempDir()
			tokenPath := filepath.Join(dir, "worker-token")
			victim := filepath.Join(dir, "victim")
			if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(victim, c.path(tokenPath)); err != nil {
				t.Fatal(err)
			}
			c.write(tokenPath)
			if b, _ := os.ReadFile(victim); string(b) != "keep" {
				t.Fatalf("link target overwritten: %q", b)
			}
			checkSidecar(t, c.path(tokenPath), c.want)
		})
		t.Run(c.name+"/mode", func(t *testing.T) {
			dir := t.TempDir()
			tokenPath := filepath.Join(dir, "worker-token")
			if err := os.WriteFile(c.path(tokenPath), []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
			c.write(tokenPath)
			checkSidecar(t, c.path(tokenPath), c.want)
		})
	}
}

func checkSidecar(t *testing.T, path, want string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 {
		t.Fatalf("sidecar %s: mode %v, want a 0600 regular file", path, fi.Mode())
	}
	if b, _ := os.ReadFile(path); string(b) != want {
		t.Fatalf("sidecar %s = %q, want %q", path, b, want)
	}
}
