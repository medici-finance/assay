package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// flaggedRun builds, at test time, a 40-char mixed-case run that the secret scan's
// high-entropy arm refuses. It is assembled here so no flaggable literal sits in source.
func flaggedRun() string {
	const pool = "QxLkZtNcYfRhVsJuXoAeGiWnDzBmPq"
	var b strings.Builder
	for i := 0; b.Len() < 40; i++ {
		b.WriteByte(pool[(i*7)%len(pool)])
		if i%3 == 0 {
			b.WriteByte(byte('2' + i%8))
		}
	}
	return b.String()[:40]
}

// publishedRange pushes one more commit to the branch, carrying the flagged run when
// flagged is set, so the destination tip is confirmed and no local commit lies beyond it.
func publishedRange(t *testing.T, flagged bool) (string, *[][]string, string) {
	t.Helper()
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	value := "clean"
	if flagged {
		value = flaggedRun()
	}
	writeFile(t, filepath.Join(work, "pushed.txt"), "value: "+value+"\n")
	mustGit(t, work, "add", "pushed.txt")
	mustGit(t, work, "commit", "-m", "add pushed value")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/feature/test-branch")
	mustGit(t, work, "update-ref", "refs/remotes/origin/feature/test-branch", "HEAD")
	return work, calls, mustGit(t, work, "rev-parse", "HEAD")
}

// regression: #6060
//
// The confirmed-tip narrowing covers the publish-identity check only. The secret scan
// deliberately judges the whole branch diff against the default branch, so a flagged
// string already on the destination is refused again by --check-remote (and by a live
// create), even with no commit beyond the confirmed tip. The audited override clears it.
// A flagged commit beyond a clean published tip is refused, and so is a flagged pushed
// range when the destination cannot be read at all.
func TestRemoteCheckScansWhole(t *testing.T) {
	for _, mode := range []string{"pushed only", "new commit", "new flagged", "unconfirmable", "override"} {
		t.Run(mode, func(t *testing.T) {
			work, calls, tip := publishedRange(t, mode != "new flagged")
			switch mode {
			case "new commit":
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "new.txt")
			case "new flagged":
				writeFile(t, filepath.Join(work, "new.txt"), "value: "+flaggedRun()+"\n")
				mustGit(t, work, "add", "new.txt")
				mustGit(t, work, "commit", "-m", "add new value")
			case "unconfirmable":
				mustGit(t, work, "remote", "set-url", "--push", "origin", "file://"+filepath.Join(t.TempDir(), "missing.git"))
			}
			args := []string{"create", "--title", "check published", "--body-min", "Issue: #6060\nCheck published range.", "--check-remote"}
			if mode == "override" {
				args = append(args, "--"+deskkit.ScanOverrideFlag, "fixture value already published")
			}
			var rc int
			stderr := captureStderr(t, func() { rc = run(args) })
			want := deskkit.ExitRefused
			if mode == "override" {
				want = deskkit.ExitOK
			}
			if rc != want {
				t.Fatalf("%s: rc=%d want %d\n%s", mode, rc, want, stderr)
			}
			if want == deskkit.ExitRefused && !strings.Contains(stderr, "branch diff vs") {
				t.Fatalf("%s: refusal is not the branch-diff secret scan\n%s", mode, stderr)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("preflight caused an outward write")
			}
			if got := remoteRef(t, filepath.Join(filepath.Dir(work), "origin.git"), "refs/heads/feature/test-branch"); got != tip {
				t.Fatal("preflight moved destination")
			}
		})
	}
}

// regression: #6060
//
// The help must scope the confirmed-tip narrowing to the identity check and say the
// secret scan still judges the whole branch diff, so a repeat refusal is not a surprise.
func TestHelpScopesTipNarrowing(t *testing.T) {
	help := strings.Join(strings.Fields(usage), " ")
	for _, want := range []string{
		"publish-identity (author/committer) check on only the commits beyond its confirmed tip",
		"the secret scan and the outbound check (here and in the pre-push hook) always judge the whole branch diff",
		"commits already on the destination included",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q", want)
		}
	}
	docs := map[string]string{"deskpr help": help}
	for _, path := range []string{"../../README.md", "../../../../docs/evidence-publishing-contract.md"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		docs[path] = strings.Join(strings.Fields(string(b)), " ")
	}
	for where, text := range docs {
		for _, unscoped := range unscopedTipClaims {
			if strings.Contains(text, unscoped) {
				t.Errorf("%s says %q without scoping it to the identity check", where, unscoped)
			}
		}
	}
	for _, path := range []string{"../../README.md", "../../../../docs/evidence-publishing-contract.md"} {
		if !strings.Contains(docs[path], "The confirmed tip narrows the identity check only.") {
			t.Errorf("%s lacks the identity-only scope sentence", path)
		}
	}
}

// unscopedTipClaims are the phrasings that read as "the tip narrows every check".
var unscopedTipClaims = []string{
	"checks only commits beyond its confirmed tip",
	"judges every commit actually new to that destination",
}
