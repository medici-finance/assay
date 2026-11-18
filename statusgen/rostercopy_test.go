package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lintRowFixture rewrites the container fixture's brief so its one Verify row
// runs a built statusgen --lint on a copy of testdata/goodrepo, and commits it.
func lintRowFixture(t *testing.T, fx containerFixture) string {
	t.Helper()
	bin := buildStatusgenBinary(t)
	lintRoot := t.TempDir()
	if err := os.CopyFS(lintRoot, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "lint.log")
	tick := "`"
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + bin + " --root " + lintRoot + " --lint > " + log + " 2>&1" + tick + " | exit 0 |\n\n## Evidence\n\n## Review\n\nnone\n"
	putFile(t, fx.brief, body)
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-m", "lint row")
	return log
}

// useHome points HOME at home for this test and reloads the roster cache.
func useHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)
}

// regression: #6904
// TestContainerLintRoster: a container-netns row that runs statusgen --lint
// refuses its configuration with no config-home roster. With --roster-copy
// the open step writes only statusgen's keys to a new 0600 file, records its
// digest and prints the read-only mount; the container run refuses a
// read-write or different copy before any row, and otherwise gives each row's
// fresh HOME the copy, so the --lint row passes.
func TestContainerLintRoster(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	log := lintRowFixture(t, fx)
	putFile(t, fx.check, validCheck)

	// The host roster: the fixture's, plus a key statusgen does not read and
	// a non-schema key, neither of which may travel.
	hostHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hostHome, ".config", "assay"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostHome, ".config", "assay", "roster.env"), []byte(fixtureRoster+darFixtureExtra()+"ASSAY_USAGE_DIR=/host/usage\nOTHER_TOKEN=do-not-copy\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Without a roster the container's --lint row refuses its configuration.
	useHome(t, t.TempDir())
	fx.writeBracketReceipt(t, validCheck)
	stubMountRO(t, true)
	if res, out := captureVerifyrunOut(t, fx.runArgs()); res.code == verifyrunExitPass {
		t.Fatalf("a --lint row with no config-home roster passed:\n%s", out)
	}
	os.Remove(fx.witness)

	// Open on the host with --roster-copy.
	useHome(t, hostHome)
	copyPath := filepath.Join(t.TempDir(), "roster.env")
	os.Remove(fx.receipt)
	res, out := captureVerifyrunOut(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--roster-copy", copyPath})
	if res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d: %s", res.code, out)
	}
	raw, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(copyPath); fi.Mode().Perm() != 0o600 {
		t.Fatalf("roster copy mode %04o, want 0600", fi.Mode().Perm())
	}
	if strings.Contains(string(raw), "ASSAY_USAGE_DIR") || strings.Contains(string(raw), "do-not-copy") || !strings.Contains(string(raw), "ASSAY_TRUSTED_LOGINS=") {
		t.Fatalf("the roster copy carries a key statusgen does not read, or lacks the roster:\n%s", raw)
	}
	var receipt bracketReceipt
	if err := json.Unmarshal([]byte(getFile(t, fx.receipt)), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Roster == nil || receipt.Roster.Digest != rosterDigest(raw) {
		t.Fatalf("receipt roster %+v, want digest %s", receipt.Roster, rosterDigest(raw))
	}
	if !strings.Contains(out, "--mount type=bind,source="+copyPath+",target=/run/assay-roster/roster.env,readonly") {
		t.Fatalf("the open step does not print the read-only roster mount:\n%s", out)
	}

	// Container: the copy is "mounted" at the target.
	target := filepath.Join(t.TempDir(), "roster.env")
	orig := containerRosterTarget
	containerRosterTarget = target
	t.Cleanup(func() { containerRosterTarget = orig })
	putFile(t, target, string(raw))
	// The container's own HOME has no roster: each row's fresh HOME holds the
	// copy, and nothing is written outside the rows' scratch.
	containerHome := t.TempDir()
	useHome(t, containerHome)
	installed := filepath.Join(containerHome, ".config", "assay", "roster.env")

	stubMountRO(t, false)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code == verifyrunExitPass || !strings.Contains(stderr, "mounted read-write") {
		t.Fatalf("read-write roster mount: exit %d stderr %q", res.code, stderr)
	}
	stubMountRO(t, true)
	putFile(t, target, string(raw)+"ASSAY_STREAM_CAP=9\n")
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code == verifyrunExitPass || !strings.Contains(stderr, "not the "+receipt.Roster.Digest) {
		t.Fatalf("a different roster copy: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.witness); err == nil {
		t.Fatal("a refused run wrote a witness")
	}
	putFile(t, target, string(raw))
	if res, out := captureVerifyrunOut(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container --lint row with the roster copy: exit %d:\n%s\nrow output:\n%s", res.code, out, getFile(t, log))
	}
	if !strings.Contains(getFile(t, log), "LINT: PASS") {
		t.Fatalf("the --lint row did not load the roster copy:\n%s", getFile(t, log))
	}
	if _, err := os.Stat(installed); err == nil {
		t.Fatal("the run wrote a roster into the container's own HOME")
	}
}
