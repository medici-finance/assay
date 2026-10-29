package gittest

import (
	"testing"
)

// The three worked goldens below are the templates migration briefs copy: one per
// broad category (a read, a diff, a commit). Each asserts OUTCOME — refs, tree/blob
// contents, returned value, error class — never argv.

func TestGoldenReadHead(t *testing.T) {
	Record(t, "read-head", func(f *Fixture) (string, error) {
		return f.Git("rev-parse", "HEAD")
	})
}

func TestGoldenDiffAfterChange(t *testing.T) {
	Record(t, "diff-after-change", func(f *Fixture) (string, error) {
		f.CommitFile(t, "added.txt", "new line\n", "add a file")
		return f.Git("diff", "--name-only", "HEAD~1", "HEAD")
	})
}

func TestGoldenCommitAdvancesHead(t *testing.T) {
	Record(t, "commit-advances-head", func(f *Fixture) (string, error) {
		return f.CommitFile(t, "second.txt", "second\n", "second commit"), nil
	})
}
