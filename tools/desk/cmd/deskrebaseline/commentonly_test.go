package main

import "testing"

// TestGatherRowFactsDoesNotProbeACommentOnlyCommand — the behaviour probe runs the
// row's command and reads a zero exit as "the row still passes". A command that is
// only a shell comment exits 0 without looking at anything (#2684), so it is not
// probed; a real command still is (positive control).
// regression: #2684
func TestGatherRowFactsDoesNotProbeACommentOnlyCommand(t *testing.T) {
	var probed []string
	orig := runCommand
	runCommand = func(_ string, command string) (bool, int) {
		probed = append(probed, command)
		return true, 0
	}
	t.Cleanup(func() { runCommand = orig })

	root := t.TempDir()
	f := gatherRowFacts(root, verifyRow{Num: 1, Class: "check", Command: "## Coverage"}, false, "")
	if f.CommandProbed || len(probed) != 0 {
		t.Errorf("a comment-only command must not be probed, got probed=%v CommandProbed=%v", probed, f.CommandProbed)
	}
	f = gatherRowFacts(root, verifyRow{Num: 2, Class: "check", Command: "true"}, false, "")
	if !f.CommandProbed || len(probed) != 1 || probed[0] != "true" {
		t.Errorf("a real command must still be probed, got probed=%v CommandProbed=%v", probed, f.CommandProbed)
	}
}
