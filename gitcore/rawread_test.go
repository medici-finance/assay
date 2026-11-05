package gitcore

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestRawCommitBytes(t *testing.T) {
	f := gittest.NewFixture(t)
	tree := mustGitOutput(t, f, "rev-parse", "HEAD^{tree}")
	for name, headers := range map[string]string{
		"ordinary":  "author Alpha <alpha@example.invalid> 1 +0000\n",
		"duplicate": "author Alpha <alpha@example.invalid> 1 +0000\nauthor Beta <beta@example.invalid> 1 +0000\n",
		"encoding":  "author Andr\xe9 <alpha@example.invalid> 1 +0000\nencoding ISO-8859-1\n",
		"spacing":   "author  Alpha  < alpha@example.invalid > 1 +0000\n",
	} {
		t.Run(name, func(t *testing.T) {
			raw := []byte("tree " + tree + "\n" + headers + "committer Test <test@example.invalid> 1 +0000\n\nsubject\r\n\nbody\x00tail\n")
			cmd := exec.Command("git", "-C", f.Dir, "hash-object", "-t", "commit", "--literally", "-w", "--stdin")
			cmd.Stdin = bytes.NewReader(raw)
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			id := strings.TrimSpace(string(out))
			r, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.RawCommit(id)
			if err != nil {
				t.Fatal(err)
			}
			want, err := exec.Command("git", "--no-replace-objects", "-C", f.Dir, "cat-file", "commit", id).Output()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, raw) || !bytes.Equal(got, want) {
				t.Fatalf("raw commit bytes differ: got %q want %q", got, raw)
			}
			if _, err := r.RawCommit("HEAD"); err == nil {
				t.Fatal("accepted a mutable revision")
			}
			if _, err := r.RawCommit(strings.Repeat("f", 40)); err == nil {
				t.Fatal("missing commit reported readable")
			}
		})
	}
}
