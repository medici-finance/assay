package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// outbound_test.go — the push-path incident fixture (desktools-v2/10, Verify row 5).
//
// Before the outbound-write check, deskpr's push scan ran the credential arms and the
// impersonation guard only, so a comment in a test file naming a PRIVATE repository was
// pushed to a public one and only an out-of-band sweep caught it after review. The check
// now runs over the pushed range's ADDED lines (kind `file`) before any push.

const obPrivateRepo = "example-org/example-internal"

// plantPublicTargetRoster re-plants the fixture roster with obPrivateRepo stated private,
// then points the fixture checkout's origin at the fixture's PUBLIC repository.
func plantPublicTargetRoster(t *testing.T, work string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	roster := strings.Replace(fixtureRoster, "ASSAY_ALLOWED_REPOS=",
		"ASSAY_ALLOWED_REPOS="+obPrivateRepo+":ci:private,", 1)
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	mustGit(t, work, "remote", "set-url", "origin", "https://github.com/example-org/example-k8s.git")
}

func TestPushRefusesWithheldNameInAddedTestComment(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	plantPublicTargetRoster(t, work)

	writeFile(t, filepath.Join(work, "widget_test.go"),
		"package widget\n\n// Mirrors the fixture kept in "+obPrivateRepo+" so both stay in step.\nfunc TestWidget() {}\n")
	mustGit(t, work, "add", "widget_test.go")
	mustGit(t, work, "commit", "-m", "add widget test")

	stderr := withStderrCapture(t)
	rc := run([]string{"create", "--title", "add widget test", "--body-min", "adds a test\nBrief: fixture/01"})

	var pushes int
	for _, c := range *calls {
		if len(c) > 0 && c[0] == pushMark {
			pushes++
		}
	}
	if pushes != 0 {
		t.Fatalf("the branch was PUSHED (%d push call(s)) with a private repository name on an added "+
			"line, to a public target — rc=%d\n%s", pushes, rc, stderr.String())
	}
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)\n%s", rc, deskkit.ExitRefused, stderr.String())
	}
	if curForge.createCalls != 0 {
		t.Fatal("a draft change was opened although the push was refused")
	}
}
