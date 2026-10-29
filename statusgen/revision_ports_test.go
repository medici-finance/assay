package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// revisionNativeGuard blocks only revision/prefix construction calls after all
// native fixture/oracle work. Other deliberately native owners (show-ref,
// symbolic-ref, archive, history, merge-base) retain their real implementation.
func revisionNativeGuard(t *testing.T) string {
	t.Helper()
	native, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "revision-starts")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nfor arg do\nif [ \"$arg\" = rev-parse ]; then echo native-revision >>" + quote(marker) + "; exit 91; fi\ndone\nexec " + quote(native) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}
func revisionNoStarts(t *testing.T, marker string) {
	t.Helper()
	if raw, err := os.ReadFile(marker); !os.IsNotExist(err) {
		t.Errorf("qualified caller launched native revision query: %q %v", raw, err)
	}
}
func revisionFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	mustWrite(t, filepath.Join(root, "check.go"), "package check\n")
	runGit(t, root, "add", "check.go")
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-03T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-03T12:00:00Z"}, "commit", "-qm", "repair")
	head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	runGit(t, root, "update-ref", "refs/remotes/origin/main", head)
	return root, head
}

// regression: #2395
func TestRevisionCallerFlow(t *testing.T) {
	t.Run("exact-ref-peel", func(t *testing.T) {
		root, head := revisionFixture(t)
		marker := revisionNativeGuard(t)
		if got := exactRefCommit(root, "refs/remotes/origin/main"); got != head {
			t.Errorf("exact ref=%q want %q", got, head)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("integration-commit", func(t *testing.T) {
		root, head := revisionFixture(t)
		marker := revisionNativeGuard(t)
		if got, err := integrationTip(root); err != nil || got != head {
			t.Errorf("integration=(%q,%v)", got, err)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("ordinary-root-prefix", func(t *testing.T) {
		root, _ := revisionFixture(t)
		marker := revisionNativeGuard(t)
		if why := verifyrunRootBelowToplevel(root); why != "" {
			t.Errorf("root was refused: %s", why)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("decision-commit", func(t *testing.T) {
		root := dhFixture(t, unruledBase())
		dhSeams(t, nil)
		marker := revisionNativeGuard(t)
		var code int
		captureStdout(t, func() { code = runDecisionGate(root, "", "HEAD") })
		if code != 0 {
			t.Errorf("unchanged board decision got %d", code)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("delivery-commit-and-parent", func(t *testing.T) {
		root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here", "web/excluded.go: out-of-scope (unchanged reader contract)"})
		marker := revisionNativeGuard(t)
		var code int
		captureStdout(t, func() { code = runDeliveryConsumers(root, base, "alpha/01", delivery) })
		if code != 0 {
			t.Errorf("delivery got %d", code)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("stale-fail-ref", func(t *testing.T) {
		root, head := revisionFixture(t)
		marker := revisionNativeGuard(t)
		got, ok := newestFailFix(root, &BriefFile{DeclaredEntriesRaw: []string{"check.go"}}, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
		if !ok || !strings.HasPrefix(head, got) || got == "" {
			t.Errorf("repair=(%q,%v), head=%s", got, ok, head)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("absent-evidence-commit", func(t *testing.T) {
		root, _ := revisionFixture(t)
		marker := revisionNativeGuard(t)
		if got, err := flipEvidenceAt(root, "HEAD", "absent.md"); err != nil || got != flipAbsent {
			t.Errorf("absent evidence=(%q,%v)", got, err)
		}
		revisionNoStarts(t, marker)
	})
}
