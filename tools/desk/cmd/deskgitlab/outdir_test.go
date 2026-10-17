package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFleetScenarioTokenFileReplacedNotFollowed — a token file is created exclusively and then
// put in place: an entry already sitting at the token path is replaced as a whole, so what it
// pointed at is left untouched and the path ends up a regular 0600 file holding the minted
// value.
func TestFleetScenarioTokenFileReplacedNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: creating a symbolic link needs a privilege Windows test runners lack")
	}
	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	out := filepath.Join(t.TempDir(), "tokens")
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(out, fleetUsername(fakePrefix, "reviewer")+".token")
	if err := os.Symlink(elsewhere, file); err != nil {
		t.Fatal(err)
	}

	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--project", fakeProject, "--out-dir", out, "--no-avatars")
	if r.code != 0 {
		t.Fatalf("run exited %d:\n%s", r.code, r.out)
	}
	if b, _ := os.ReadFile(elsewhere); string(b) != "untouched" {
		t.Errorf("the file the old entry pointed at now holds %q, want it untouched", b)
	}
	fi, err := os.Lstat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 {
		t.Errorf("token path is %v, want a regular 0600 file", fi.Mode())
	}
	if b, _ := os.ReadFile(file); string(b) != fakeMinted {
		t.Errorf("token file holds %q, want the minted value", b)
	}
	left, _ := filepath.Glob(filepath.Join(out, ".*.token.*"))
	if len(left) > 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

// TestFleetScenarioSharedOutDirRefused — an existing --out-dir that other users can write to is
// refused before any request is sent; an owner-only one is used as given.
func TestFleetScenarioSharedOutDirRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: Windows permissions are not mode bits")
	}
	for _, mode := range []os.FileMode{0o777, 0o770, 0o722} {
		f := newFakeGitLab(t)
		out := filepath.Join(t.TempDir(), "shared")
		if err := os.Mkdir(out, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(out, mode); err != nil {
			t.Fatal(err)
		}
		r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", out, "--no-avatars")
		if r.code == 0 || !strings.Contains(r.out, "writable by other users") {
			t.Errorf("out-dir mode %v: exit %d, want a refusal naming the shared directory:\n%s", mode, r.code, r.out)
		}
		if n := f.calls(); n != 0 {
			t.Errorf("out-dir mode %v: %d request(s) reached GitLab before the refusal, want none", mode, n)
		}
		if ents, _ := os.ReadDir(out); len(ents) != 0 {
			t.Errorf("out-dir mode %v: the refused run wrote %d file(s)", mode, len(ents))
		}
	}

	f := newFakeGitLab(t)
	f.respond = mainProtectedAsIntended
	out := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(out, 0o755); err != nil {
		t.Fatal(err)
	}
	r := runAgainst(t, f, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--project", fakeProject, "--out-dir", out, "--no-avatars")
	if r.code != 0 {
		t.Fatalf("an owner-only-writable out-dir (0755) was refused: exit %d:\n%s", r.code, r.out)
	}
}
