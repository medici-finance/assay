package gitcore

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestObjectFormatParity(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := &gittest.Fixture{Dir: t.TempDir()}
			if _, err := f.Git("init", "-q", "-b", "main", "--object-format="+format); err != nil {
				t.Fatal(err)
			}
			id := f.CommitFile(t, "f", "raw\x00bytes\r\n", "one")
			got, err := ObjectFormat(f.Dir)
			if err != nil || got != format {
				t.Fatalf("object format %q %v, want %s", got, err, format)
			}
			native, err := exec.Command("git", "--no-replace-objects", "-C", f.Dir, "cat-file", "commit", id).Output()
			if err != nil || len(native) == 0 {
				t.Fatal("native format fixture unreadable", err)
			}
			r, openErr := Open(f.Dir)
			if format == BuildObjectFormat() {
				if openErr != nil {
					t.Fatal(openErr)
				}
				b, err := r.RawCommit(id)
				if err != nil || !bytes.Equal(b, native) {
					t.Fatal("supported format read differs", err)
				}
			} else {
				if openErr == nil {
					if _, err := r.RawCommit(id); err == nil {
						t.Fatal("fixed-width reader accepted incompatible object id")
					}
				}
			}
			wt := filepath.Join(t.TempDir(), "linked")
			if _, err := f.Git("worktree", "add", "-q", "--detach", wt, "HEAD"); err != nil {
				t.Fatal(err)
			}
			got, err = ObjectFormat(wt)
			if err != nil || got != format {
				t.Fatalf("linked format %q %v", got, err)
			}
			if _, err := f.Git("config", "extensions.objectformat", "unknown"); err != nil {
				t.Fatal(err)
			}
			if _, err := ObjectFormat(f.Dir); err == nil || !strings.Contains(err.Error(), "unsupported object format") {
				t.Fatalf("unknown format not refused: %v", err)
			}
		})
	}
}
