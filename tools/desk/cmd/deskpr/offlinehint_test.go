package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// offlinehint_test.go — brief 27's ergonomics half for deskpr: the two largest measured
// `deskpr` refusal classes (a body with no Brief:/Issue: trailer, and a missing --title)
// now name the offline rehearsal that would have caught them for free, and `deskpr create
// --check` IS that rehearsal — a gate run early, never a preview that can disagree with
// the real write path.

// TestSchemaRefusalNamesTheOfflineCheck pins the two measured top classes: both must keep
// their original diagnosis (a hint that replaced the explanation would trade one usability
// problem for a worse one) and both must now point at `deskpr … --check`.
func TestSchemaRefusalNamesTheOfflineCheck(t *testing.T) {
	t.Run("no trailer", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)

		err := cmdCreate([]string{"--title", "add feature", "--body-min", "does the thing"})
		if !deskkit.IsRefused(err) {
			t.Fatalf("err = %v, want exit-5 refusal", err)
		}
		got := err.Error()
		if !strings.Contains(got, "Brief: <stream>/<NN>") {
			t.Errorf("the original diagnosis is gone: %s", got)
		}
		if !strings.Contains(got, "deskpr") || !strings.Contains(got, "--check") {
			t.Errorf("the refusal does not name the offline check that would have caught it: %s", got)
		}
	})

	t.Run("missing --title", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)

		err := cmdCreate([]string{"--body-min", "x\nBrief: fixture/01"})
		if !deskkit.IsRefused(err) {
			t.Fatalf("err = %v, want exit-5 refusal", err)
		}
		got := err.Error()
		if !strings.Contains(got, "--title is required") {
			t.Errorf("the original diagnosis is gone: %s", got)
		}
		if !strings.Contains(got, "deskpr") || !strings.Contains(got, "--check") {
			t.Errorf("the refusal does not name the offline check that would have caught it: %s", got)
		}
		// Adding a hint is not a change of verdict.
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Errorf("exit = %d, want %d", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
		}
	})

	// THE NEGATIVE CONTROL. A well-formed create must still pass its LOCAL gates (it will
	// still refuse further down once the fake forge is asked something it has not been
	// primed for — the hint must never appear on an acceptance).
}

// k8sURL is a fixture origin resolving to a repo the fixture roster marks PUBLIC
// (example-org/example-k8s:ci:public — rosterfixture_test.go), so the public-repo
// self-containment scan (deskkit.SelfContainApplies) actually runs, unlike the
// known-private example-org/tracker fixture every other case in this package uses.
const k8sURL = "https://github.com/example-org/example-k8s.git"

// newPublicFixture is newBaseFixture's twin against a PUBLIC-repo origin — same shape
// (bare "origin" for the push target, a committed fixture brief on origin/main so
// `Brief: fixture/01` resolves, then one feature commit ahead).
func newPublicFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	bareURL := "file://" + bare

	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", k8sURL)
	mustGit(t, work, "remote", "set-url", "--push", "origin", bareURL)

	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")

	if merr := os.MkdirAll(filepath.Join(work, "docs", "streams", "fixture"), 0o755); merr != nil {
		t.Fatalf("mkdir fixture streams: %v", merr)
	}
	writeFile(t, filepath.Join(work, "docs", "streams", "fixture", "brief-01-test.md"),
		"---\nschema: brief-v1\nbrief: fixture/01\ntitle: fixture brief\n---\n\nFixture brief for deskpr tests.\n")
	mustGit(t, work, "add", "docs")
	mustGit(t, work, "commit", "-m", "fixture brief")
	mainSHA := mustGit(t, work, "rev-parse", "HEAD")

	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mainSHA)
	mustGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	writeFile(t, filepath.Join(work, "feature.txt"), "work\n")
	mustGit(t, work, "add", "feature.txt")
	mustGit(t, work, "commit", "-m", "feature work")
	return work
}

// TestCheckIsOfflineAndGatesRatherThanPreviews is Verify row 16: `deskpr create --check`
// against a forge stub that fails every call and a git stub that refuses any push.
func TestCheckIsOfflineAndGatesRatherThanPreviews(t *testing.T) {
	t.Run("well-formed body: no connection, nothing pushed, exit 0", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)
		// The forge stub: every method embedded via the nil deskkit.Forge PANICS if
		// called (forgefake_test.go's header states this is deliberate — deskpr must
		// never call a method the fake does not override, and a call would panic loudly
		// rather than pass silently). --check must never reach it.
		//
		// The git stub: execCommand already records every argv; asserting none of them
		// is "push" (and none is "desktoken", the token mint) is the "refuses any push"
		// stub in effect — a --check that reached either would fail this test, not
		// silently succeed.
		rc := run([]string{"create", "--title", "add feature", "--body-min", "x\nBrief: fixture/01", "--check"})
		if rc != deskkit.ExitOK {
			t.Fatalf("--check on a well-formed body rc = %d, want 0", rc)
		}
		for _, c := range *calls {
			if len(c) == 0 {
				continue
			}
			base := filepath.Base(c[0])
			if base == "desktoken" {
				t.Fatalf("--check minted a token: %v", c)
			}
			if base == "git" && anyCall([][]string{c}, "push") {
				t.Fatalf("--check pushed: %v", c)
			}
		}
		if curForge.openCalls != 0 || curForge.getCalls != 0 || curForge.createCalls != 0 ||
			curForge.visibilityCalls != 0 {
			t.Fatalf("--check reached the forge: open=%d get=%d create=%d visibility=%d",
				curForge.openCalls, curForge.getCalls, curForge.createCalls, curForge.visibilityCalls)
		}
	})

	t.Run("no trailer: same refusal text as the real write path", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)

		realErr := cmdCreate([]string{"--title", "add feature", "--body-min", "does the thing"})
		checkErr := cmdCreate([]string{"--title", "add feature", "--body-min", "does the thing", "--check"})
		if !deskkit.IsRefused(realErr) || !deskkit.IsRefused(checkErr) {
			t.Fatalf("real=%v check=%v, want both exit-5 refusals", realErr, checkErr)
		}
		if realErr.Error() != checkErr.Error() {
			t.Fatalf("--check disagreed with the real write path:\n real:  %s\ncheck:  %s",
				realErr.Error(), checkErr.Error())
		}
	})

	t.Run("bare #N is named as not checked, never passed silently", func(t *testing.T) {
		work := newPublicFixture(t)
		withEnv(t, work)

		var stderr strings.Builder
		old := os.Stderr
		r, w, perr := os.Pipe()
		if perr != nil {
			t.Fatalf("pipe: %v", perr)
		}
		os.Stderr = w
		done := make(chan struct{})
		go func() {
			buf := make([]byte, 1<<16)
			for {
				n, rerr := r.Read(buf)
				if n > 0 {
					stderr.Write(buf[:n])
				}
				if rerr != nil {
					break
				}
			}
			close(done)
		}()

		err := cmdCreate([]string{"--title", "add feature",
			"--body-min", "see #42 for background\nBrief: fixture/01", "--check"})

		_ = w.Close()
		os.Stderr = old
		<-done

		if err != nil {
			t.Fatalf("--check on an otherwise-clean public-repo body err = %v, want nil (exit 0)", err)
		}
		if !strings.Contains(stderr.String(), "NOT CHECKED") {
			t.Fatalf("stderr did not name the bare-#N category as not checked:\n%s", stderr.String())
		}
	})
}
