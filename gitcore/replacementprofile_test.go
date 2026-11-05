package gitcore

import (
	"github.com/medici-finance/assay/gitcore/gittest"
	"testing"
)

func TestReplacementReadReasonIncludesSymbolic(t *testing.T) {
	f := gittest.NewFixture(t)
	id := mustGitOutput(t, f, "rev-parse", "HEAD")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if why, err := r.NativeReplacementReadReason(); err != nil || why != "" {
		t.Fatalf("ordinary namespace: %q %v", why, err)
	}
	name := "refs/replace/" + id
	if _, err := f.Git("update-ref", name, id); err != nil {
		t.Fatal(err)
	}
	if why, err := r.NativeReplacementReadReason(); err != nil || why == "" {
		t.Fatalf("hash replacement absent: %q %v", why, err)
	}
	if _, err := f.Git("update-ref", "-d", name); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Git("symbolic-ref", name, mustGitOutput(t, f, "symbolic-ref", "HEAD")); err != nil {
		t.Fatal(err)
	}
	if why, err := r.NativeReplacementReadReason(); err != nil || why == "" {
		t.Fatalf("symbolic replacement absent: %q %v", why, err)
	}
}
