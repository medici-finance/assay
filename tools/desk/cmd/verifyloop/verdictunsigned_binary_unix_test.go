//go:build unix

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnsignedOutBinaryActivation runs the BUILT verifyloop binary, so the run passes through
// main() and its activation check — the in-process rows call runVerdict and never see it.
// It pins two facts the host contract (H6) states:
//   - in a checkout with no usable roster, `verdict --unsigned-out` exits 6, writes no
//     payload and prints no digest (H2: that means do not sign);
//   - with only a roster in the fence's HOME and no key anywhere, the same command exits 0,
//     writes the payload and prints one digest line — and the signed path in that same
//     environment still finds no key, so the keyless success is not a key read.
//
// regression: #2419
func TestUnsignedOutBinaryActivation(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "verifyloop")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building verifyloop: %v\n%s", err, out)
	}

	// A checkout the binary's activation check can discover: a .git entry and a component
	// manifest that provides the desk verbs and requires the roster trust key — the shape of
	// the tree's tools/desk/component.yaml, kept minimal so an unrelated manifest change
	// cannot redden this test.
	root := demoRoot(t)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "component: assay/desk-tools\nprovides:\n  - assay.desk.verbs\ninject:\n  required:\n    - key: assay.roster.trust\n"
	if err := os.MkdirAll(filepath.Join(root, "tools", "desk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", "desk", "component.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// home builds a fence HOME: a roster copy when withRoster, never a key.
	home := func(withRoster bool) string {
		h := t.TempDir()
		if withRoster {
			dir := filepath.Join(h, ".config", "assay")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			roster := "ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\n"
			if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return h
	}
	run := func(h string, args ...string) (int, string, string) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		cmd.Env = []string{"HOME=" + h, "PATH=" + os.Getenv("PATH"), "VERIFIER_PEM=", "ASSAY_CONFIG_HOME="}
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("running %s: %v", bin, err)
		}
		return code, so.String(), se.String()
	}

	t.Run("no-roster-refuses", func(t *testing.T) {
		payload := filepath.Join(privDir(t), "payload.json")
		code, stdout, stderr := run(home(false), "verdict", "--root", ".", "--repo", exRepo, "--sha", exHead, "--unsigned-out", payload)
		if code != 6 {
			t.Fatalf("exit = %d, want 6\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed") {
			t.Fatalf("stderr lacks the activation refusal:\n%s", stderr)
		}
		if strings.Contains(stdout, "sha256=") {
			t.Fatalf("a refused run printed a digest:\n%s", stdout)
		}
		if _, err := os.Lstat(payload); !os.IsNotExist(err) {
			t.Fatalf("a refused run left a payload behind (lstat err %v)", err)
		}
	})

	t.Run("roster-only-composes", func(t *testing.T) {
		h := home(true)
		payload := filepath.Join(privDir(t), "payload.json")
		code, stdout, stderr := run(h, "verdict", "--root", ".", "--repo", exRepo, "--sha", exHead, "--unsigned-out", payload)
		if code != 0 {
			t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if n := len(sha256Line.FindAllString(stdout, -1)); n != 1 {
			t.Fatalf("stdout carries %d sha256= values, want exactly 1:\n%s", n, stdout)
		}
		if _, err := os.Stat(payload); err != nil {
			t.Fatalf("payload not written: %v", err)
		}
		if _, err := os.Stat(filepath.Join(h, ".config", "assay", "verifier-app.pem")); !os.IsNotExist(err) {
			t.Fatalf("the fixture HOME holds a key (stat err %v) — the keyless success proves nothing", err)
		}

		// CONTROL: the signed path in the same environment finds no key.
		code, _, stderr = run(h, "verdict", "--root", ".", "--repo", exRepo, "--sha", exHead, "--dry-run")
		if code != 6 || !strings.Contains(stderr, "cannot find the verifier private key") {
			t.Fatalf("control: signed path exit = %d, want 6 with 'cannot find the verifier private key'\nstderr: %s", code, stderr)
		}
	})
}
