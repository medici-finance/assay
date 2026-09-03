package filer

import (
	"errors"
	"strings"
	"testing"
)

func advItem() RefactorItem {
	return NewAdvisoryItem("t", "body about qualgen/hotspot.go", "qualgen/hotspot.go", "refactor")
}

// TestGitHubFiler_DryRunFilesNothing: with ForceDryRun (and no Post wired) the
// adapter composes the item and files nothing — the first-class dry-run mode.
func TestGitHubFiler_DryRunFilesNothing(t *testing.T) {
	g := &GitHubFiler{Owner: "o", Repo: "r", ForceDryRun: true}
	res, err := g.File(advItem(), false)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Filed {
		t.Fatalf("ForceDryRun should file nothing, got Filed=true")
	}
	if !res.DryRun {
		t.Fatalf("want DryRun=true")
	}
	if res.Ref != "" {
		t.Fatalf("dry-run must carry no tracker ref, got %q", res.Ref)
	}
	if res.Item.TargetPath != "qualgen/hotspot.go" {
		t.Fatalf("composed item lost its target: %q", res.Item.TargetPath)
	}
}

// TestGitHubFiler_NilPostIsDryRunOnly: the zero-value/ unwired adapter never
// contacts the network — a nil Post degrades every File to a dry-run.
func TestGitHubFiler_NilPostIsDryRunOnly(t *testing.T) {
	g := &GitHubFiler{Owner: "o", Repo: "r"} // Post nil
	res, err := g.File(advItem(), false)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Filed || !res.DryRun {
		t.Fatalf("nil Post must dry-run: Filed=%v DryRun=%v", res.Filed, res.DryRun)
	}
}

// TestGitHubFiler_FilesThroughPost: with a Post hook wired and dryRun=false the
// adapter files exactly once and returns the tracker ref.
func TestGitHubFiler_FilesThroughPost(t *testing.T) {
	calls := 0
	g := &GitHubFiler{
		Owner: "o", Repo: "r",
		Post: func(owner, repo string, item RefactorItem) (string, error) {
			calls++
			if owner != "o" || repo != "r" {
				t.Fatalf("wrong target: %s/%s", owner, repo)
			}
			return "https://example/issues/1", nil
		},
	}
	res, err := g.File(advItem(), false)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !res.Filed || res.DryRun {
		t.Fatalf("want a real file: Filed=%v DryRun=%v", res.Filed, res.DryRun)
	}
	if res.Ref != "https://example/issues/1" {
		t.Fatalf("ref not propagated: %q", res.Ref)
	}
	if calls != 1 {
		t.Fatalf("Post called %d times, want 1", calls)
	}
}

// TestGitHubFiler_CallerDryRunWinsOverWiredPost: even with Post wired, a
// caller-forced dryRun (the over-budget path) files nothing.
func TestGitHubFiler_CallerDryRunWinsOverWiredPost(t *testing.T) {
	g := &GitHubFiler{
		Owner: "o", Repo: "r",
		Post: func(string, string, RefactorItem) (string, error) {
			t.Fatalf("Post must not be called when the caller forces dry-run")
			return "", nil
		},
	}
	res, err := g.File(advItem(), true)
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Filed || !res.DryRun {
		t.Fatalf("caller dry-run must win: Filed=%v DryRun=%v", res.Filed, res.DryRun)
	}
}

// TestGitHubFiler_RefusesNonAdvisory: a bare non-advisory item is refused, so
// the never-self-dispatch posture can't be bypassed by hand-crafting an item.
func TestGitHubFiler_RefusesNonAdvisory(t *testing.T) {
	g := &GitHubFiler{Owner: "o", Repo: "r", ForceDryRun: true}
	bad := RefactorItem{Title: "t", TargetPath: "p", Advisory: false}
	_, err := g.File(bad, false)
	if !errors.Is(err, ErrNotAdvisory) {
		t.Fatalf("want ErrNotAdvisory, got %v", err)
	}
}

// TestGitHubFiler_RefusesNoTarget: an item with no target path (the dedup key)
// is refused.
func TestGitHubFiler_RefusesNoTarget(t *testing.T) {
	g := &GitHubFiler{Owner: "o", Repo: "r", ForceDryRun: true}
	bad := RefactorItem{Title: "t", TargetPath: "  ", Advisory: true}
	_, err := g.File(bad, false)
	if err == nil || !strings.Contains(err.Error(), "no target path") {
		t.Fatalf("want no-target error, got %v", err)
	}
}

// TestNewAdvisoryItem_AlwaysAdvisory: the composer guarantees the advisory flag
// and a single advisory label, deduping extras.
func TestNewAdvisoryItem_AlwaysAdvisory(t *testing.T) {
	it := NewAdvisoryItem("t", "b", "p", "refactor", "advisory", "refactor")
	if !it.Advisory {
		t.Fatalf("advisory flag not set")
	}
	adv := 0
	for _, l := range it.Labels {
		if l == AdvisoryLabel {
			adv++
		}
	}
	if adv != 1 {
		t.Fatalf("advisory label present %d times, want exactly 1: %v", adv, it.Labels)
	}
}
