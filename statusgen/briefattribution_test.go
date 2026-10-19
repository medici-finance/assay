package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// regression: #3343 — strict opt-in shares actor policy, not corpus severity.
func TestBriefAttribution(t *testing.T) {
	for _, tc := range []struct {
		name, author string
		want         int
	}{
		{"verifier", vfMailVerifier, briefInfoExitOK},
		{"human", vfMailHuman, briefInfoExitOK},
		{"worker", vfMailWorker, briefInfoExitInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := vfDefaults()
			o.status = "verified"
			o.verified = vfStamp
			o.author = tc.author
			root, _ := vfFixture(t, o)
			// The baseline is already closed: strict acceptance is independent
			// of corpus lint's historical severity classification.
			vfGit(t, root, tc.author, "update-ref", "refs/remotes/origin/main", "HEAD")
			var out, errout bytes.Buffer
			code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout)
			if code != tc.want || (code != 0 && out.Len() != 0) {
				t.Fatalf("exit=%d out=%s err=%s", code, out.String(), errout.String())
			}
			out.Reset()
			errout.Reset()
			if code := runBriefInfo([]string{"--root", root, "--check-verified", "vf/01"}, &out, &errout); code != 0 {
				t.Fatalf("default structural check changed: %d %s", code, errout.String())
			}
			if tc.name == "verifier" {
				if err := os.RemoveAll(root + "/.git"); err != nil {
					t.Fatal(err)
				}
				out.Reset()
				errout.Reset()
				if code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
					t.Fatalf("missing history exit=%d out=%s err=%s", code, out.String(), errout.String())
				}
			}
		})
	}
	var out, errout bytes.Buffer
	if code := runBriefInfo([]string{"--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
		t.Fatalf("flag dependency exit=%d out=%s", code, out.String())
	}
}

// regression: #3343 — unavailable policy and grafted history are not rejections.
func TestBriefAttributionUnavailable(t *testing.T) {
	for _, mode := range []string{"policy", "shallow"} {
		t.Run(mode, func(t *testing.T) {
			o := vfDefaults()
			o.status = "verified"
			o.verified = vfStamp
			o.author = vfMailWorker
			root, _ := vfFixture(t, o)
			if mode == "policy" {
				t.Setenv("HOME", t.TempDir())
				scanReloadConfig()
				t.Cleanup(scanReloadConfig)
			} else {
				vfGit(t, root, vfMailWorker, "commit", "-q", "--allow-empty", "-m", "later")
				dest := t.TempDir()
				cmd := exec.Command("git", "clone", "-q", "--depth=1", "file://"+root, dest)
				if raw, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("shallow fixture: %v %s", err, raw)
				}
				root = dest
				if !isShallowRepository(root) {
					t.Fatal("fixture is not shallow")
				}
			}
			var out, errout bytes.Buffer
			if code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
				t.Fatalf("exit=%d out=%s err=%s", code, out.String(), errout.String())
			}
		})
	}
}
