package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The verify-runner image's witness entrypoint (images/verify-runner/witness.sh)
// is the container half of the bracket: given a <stream>/<NN> key it runs
// `statusgen verifyrun --sandbox container-netns` behind the netns helper, and
// it must hand verifyrun the host's receipt and the image digest, or verifyrun
// refuses and no row runs.

const witnessEntrypoint = "../images/verify-runner/witness.sh"

const witnessFixtureBrief = "docs/streams/fx/brief-01-fixture.md"

type witnessFixture struct {
	home, admit, scratch, marker string
}

// newWitnessFixture builds a committed home holding one brief whose single
// Verify row touches a marker outside the home, and an admission directory
// holding the receipt the host's open step would write for it.
func newWitnessFixture(t *testing.T) witnessFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the entrypoint is a bash script")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("SKIP (could-not-check, not a pass): no bash on this host")
	}
	outside := t.TempDir()
	fx := witnessFixture{
		home:    t.TempDir(),
		admit:   filepath.Join(outside, "admission"),
		scratch: filepath.Join(outside, "scratch"),
		marker:  filepath.Join(outside, "ran"),
	}
	for _, d := range []string{fx.admit, fx.scratch, filepath.Join(fx.home, "docs", "streams", "fx")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitInit(t, fx.home, "fixture", "fixture@example.invalid")
	tick := "`"
	putFile(t, filepath.Join(fx.home, witnessFixtureBrief), "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | "+tick+"touch "+fx.marker+tick+" | exit 0 |\n\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-m", "fixture")
	cf := containerFixture{home: fx.home, receipt: filepath.Join(fx.admit, "receipt.json")}
	cf.writeBracketReceipt(t, checkJSON(1, fixtureRun, fixtureSource, witnessFixtureBrief, "m"))
	putFile(t, filepath.Join(fx.admit, "image-digest"), fixtureImage+"\n")
	return fx
}

// run starts the entrypoint at script with key, with bin first on PATH and
// none of the caller's CI or desk variables.
func (fx witnessFixture) run(t *testing.T, script, bin, key string) (int, string, string) {
	t.Helper()
	cmd := exec.Command("bash", script, key)
	cmd.Env = []string{
		"PATH=" + bin + string(os.PathListSeparator) + "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + fx.scratch,
		"VR_WORK_DIR=" + fx.home,
		"VR_ADMISSION_DIR=" + fx.admit,
		"VR_SCRATCH_DIR=" + fx.scratch,
	}
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("start %s: %v", script, err)
	}
	return code, stdout.String(), stderr.String()
}

// recordingStatusgen is a statusgen stand-in that records that it was started.
func recordingStatusgen(t *testing.T) (bin, log string) {
	t.Helper()
	bin = t.TempDir()
	log = filepath.Join(t.TempDir(), "calls")
	putFile(t, filepath.Join(bin, "statusgen"), "#!/bin/sh\necho \"$*\" >> '"+log+"'\nexit 0\n")
	if err := os.Chmod(filepath.Join(bin, "statusgen"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

// regression: #4264
// TestWitnessKeyRefusals: a key run with no receipt, no image digest, a
// malformed digest, or a symlink on the way to the brief exits 2 before
// statusgen is started, so no row runs. Portable: every refusal is decided
// before the script reads its network namespace.
func TestWitnessKeyRefusals(t *testing.T) {
	fx := newWitnessFixture(t)
	bin, log := recordingStatusgen(t)
	receipt := filepath.Join(fx.admit, "receipt.json")
	digest := filepath.Join(fx.admit, "image-digest")
	streams := filepath.Join(fx.home, "docs", "streams")
	cases := []struct {
		name, why       string
		mutate, restore func()
	}{
		{name: "no receipt", why: "no admission receipt",
			mutate:  func() { mustRename(t, receipt, receipt+".off") },
			restore: func() { mustRename(t, receipt+".off", receipt) }},
		{name: "no digest", why: "no image digest",
			mutate:  func() { mustRename(t, digest, digest+".off") },
			restore: func() { mustRename(t, digest+".off", digest) }},
		{name: "malformed digest", why: "not sha256:<64 hex>",
			mutate:  func() { putFile(t, digest, "sha256:XYZ\n") },
			restore: func() { putFile(t, digest, fixtureImage+"\n") }},
		{name: "two-line digest", why: "not sha256:<64 hex>",
			mutate:  func() { putFile(t, digest, fixtureImage+"\n"+fixtureImage+"\n") },
			restore: func() { putFile(t, digest, fixtureImage+"\n") }},
		{name: "symlinked stream directory", why: "is a symlink",
			mutate: func() {
				real := filepath.Join(t.TempDir(), "fx")
				mustRename(t, filepath.Join(streams, "fx"), real)
				if err := os.Symlink(real, filepath.Join(streams, "fx")); err != nil {
					t.Fatal(err)
				}
			},
			restore: func() {
				link := filepath.Join(streams, "fx")
				real, err := os.Readlink(link)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				mustRename(t, real, link)
			}},
	}
	for _, c := range cases {
		c.mutate()
		code, _, stderr := fx.run(t, witnessEntrypoint, bin, "fx/01")
		c.restore()
		if code != 2 || !strings.Contains(stderr, c.why) {
			t.Fatalf("%s: exit %d stderr %q, want exit 2 naming %q", c.name, code, stderr, c.why)
		}
		if calls := getFile(t, log); calls != "" {
			t.Fatalf("%s: statusgen was started: %q", c.name, calls)
		}
		if _, err := os.Stat(fx.marker); err == nil {
			t.Fatalf("%s: a Verify row ran", c.name)
		}
	}
}

func mustRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// regression: #4264
// TestWitnessKeyAdmittedRun: the entrypoint, the built statusgen and the
// kernel's namespaces together. A key run with the host's receipt and the
// image digest executes the brief's row inside the empty namespace and prints
// the witness, base64 on the last `vr-witness ` line, carrying the image
// digest; with the receipt removed, the same run executes nothing.
func TestWitnessKeyAdmittedRun(t *testing.T) {
	requireSandbox(t)
	fx := newWitnessFixture(t)
	bin := t.TempDir()
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bin, "statusgen"), ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	code, stdout, stderr := fx.run(t, witnessEntrypoint, bin, "fx/01")
	if code != 0 {
		t.Fatalf("admitted run: exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	if _, err := os.Stat(fx.marker); err != nil {
		t.Fatalf("admitted run: the Verify row did not run\nstdout %s\nstderr %s", stdout, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	last := lines[len(lines)-1]
	enc, ok := strings.CutPrefix(last, "vr-witness ")
	if !ok {
		t.Fatalf("the last line is not the witness hand-off: %q", last)
	}
	witness, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("the witness hand-off is not base64: %v", err)
	}
	if !strings.Contains(string(witness), fixtureImage) {
		t.Fatalf("the witness does not carry the image digest:\n%s", witness)
	}

	// Control: the same run without the receipt runs nothing.
	if err := os.Remove(fx.marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fx.scratch, "vr-witness.md")); err != nil {
		t.Fatal(err)
	}
	mustRename(t, filepath.Join(fx.admit, "receipt.json"), filepath.Join(fx.admit, "receipt.off"))
	code, _, stderr = fx.run(t, witnessEntrypoint, bin, "fx/01")
	if code == 0 || !strings.Contains(stderr, "no admission receipt") {
		t.Fatalf("control: exit %d stderr %q, want the missing-receipt refusal", code, stderr)
	}
	if _, err := os.Stat(fx.marker); err == nil {
		t.Fatal("control: a Verify row ran without a receipt")
	}
}
