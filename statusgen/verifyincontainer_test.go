package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A concrete, well-formed digest used across these tests — 64 lowercase hex.
const testHarnessDigest = "sha256:" +
	"1111111111111111111111111111111111111111111111111111111111111111"

func testPin() harnessPin {
	return harnessPin{
		Image:  "ghcr.io/medici-finance/assay/desk-tools",
		Tag:    "v1.0.6",
		Digest: testHarnessDigest,
	}
}

// ---------------------------------------------------------------------------
// validate — fail-closed on anything that is not a concrete digest pin
// ---------------------------------------------------------------------------

func TestHarnessPinValidate(t *testing.T) {
	cases := []struct {
		name    string
		pin     harnessPin
		wantErr bool
	}{
		{"good digest pin", testPin(), false},
		{"empty image", harnessPin{Image: "", Digest: testHarnessDigest}, true},
		{"latest tag on image", harnessPin{Image: "ghcr.io/x/desk-tools:latest", Digest: testHarnessDigest}, true},
		{"no digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: ""}, true},
		{"placeholder digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "PENDING-HARVEST-x"}, true},
		{"truncated digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "sha256:abc123"}, true},
		{"uppercased digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "sha256:" + strings.Repeat("A", 64)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pin.validate()
			if tc.wantErr && err == nil {
				t.Fatalf("validate() = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validate() = %v, want nil", err)
			}
		})
	}
}

func TestHarnessPinRefIsDigestPinned(t *testing.T) {
	got := testPin().ref()
	want := "ghcr.io/medici-finance/assay/desk-tools@" + testHarnessDigest
	if got != want {
		t.Fatalf("ref() = %q, want %q", got, want)
	}
	if strings.Contains(got, ":latest") {
		t.Fatalf("ref() must never carry a floating tag: %q", got)
	}
}

// ---------------------------------------------------------------------------
// composeDockerArgs — the pure argv the wrapper would hand docker
// ---------------------------------------------------------------------------

func TestComposeDockerArgsPOSIX(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/home/me/checkout",
		briefRel: "docs/streams/windows-port/brief-10.md",
		envFile:  "/tmp/role.env",
		inner:    buildInnerCommand("docs/streams/windows-port/brief-10.md", false, false, false, ""),
		uid:      1000,
		gid:      1000,
	}
	argv := composeDockerArgs(inv)
	joined := strings.Join(argv, " ")

	// Digest-pinned image ref, never latest.
	if !strings.Contains(joined, testPin().ref()) {
		t.Errorf("argv missing digest-pinned image ref: %q", joined)
	}
	if strings.Contains(joined, ":latest") {
		t.Errorf("argv must never reference :latest: %q", joined)
	}
	// Bind mount + workdir.
	if !argvHasPair(argv, "-v", "/home/me/checkout:/work") {
		t.Errorf("argv missing bind mount -v /home/me/checkout:/work: %q", joined)
	}
	if !argvHasPair(argv, "-w", "/work") {
		t.Errorf("argv missing -w /work: %q", joined)
	}
	// --user maps container writes to the host user so Evidence lands host-owned.
	if !argvHasPair(argv, "--user", "1000:1000") {
		t.Errorf("argv missing --user 1000:1000: %q", joined)
	}
	// Credentials: only the env-file PATH is passed through.
	if !argvHasPair(argv, "--env-file", "/tmp/role.env") {
		t.Errorf("argv missing --env-file passthrough: %q", joined)
	}
	// The inner command runs verifyrun WITHOUT --in-container (no recursion) and
	// addresses the brief under /work.
	if !strings.Contains(joined, "statusgen verifyrun --brief docs/streams/windows-port/brief-10.md") {
		t.Errorf("argv missing inner verifyrun invocation: %q", joined)
	}
	if strings.Contains(joined, "--in-container") {
		t.Errorf("inner command must NOT re-pass --in-container (would recurse): %q", joined)
	}
	// The image ref must come BEFORE the inner command (docker run <ref> <cmd…>).
	refIdx, innerIdx := indexOf(argv, testPin().ref()), indexOf(argv, "statusgen")
	if refIdx < 0 || innerIdx < 0 || refIdx > innerIdx {
		t.Errorf("image ref must precede the inner command: ref@%d inner@%d in %q", refIdx, innerIdx, joined)
	}
}

func TestComposeDockerArgsWindowsOmitsUser(t *testing.T) {
	// On Windows there is no POSIX uid (os.Getuid() == -1); --user is omitted and
	// Docker Desktop maps ownership to the host user itself.
	inv := containerInvocation{
		pin:      testPin(),
		root:     `C:\src\checkout`,
		briefRel: "docs/streams/windows-port/brief-10.md",
		inner:    buildInnerCommand("docs/streams/windows-port/brief-10.md", false, false, false, ""),
		uid:      -1,
		gid:      -1,
	}
	argv := composeDockerArgs(inv)
	if indexOf(argv, "--user") >= 0 {
		t.Errorf("argv must omit --user when there is no POSIX uid: %q", strings.Join(argv, " "))
	}
}

func TestComposeDockerArgsOmitsEnvFileWhenEmpty(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/r",
		briefRel: "b.md",
		envFile:  "",
		inner:    buildInnerCommand("b.md", false, false, false, ""),
		uid:      1000, gid: 1000,
	}
	if indexOf(composeDockerArgs(inv), "--env-file") >= 0 {
		t.Errorf("argv must omit --env-file when none is supplied")
	}
}

func TestBuildInnerCommandPassesFlags(t *testing.T) {
	inner := buildInnerCommand("b.md", true, true, true, "30s")
	joined := strings.Join(inner, " ")
	for _, want := range []string{"--check", "--dry-run", "--ci", "--timeout 30s"} {
		if !strings.Contains(joined, want) {
			t.Errorf("inner command missing %q: %q", want, joined)
		}
	}
}

// ---------------------------------------------------------------------------
// readHarnessPin — reads the harness: block from paired-versions.yaml
// ---------------------------------------------------------------------------

func TestReadHarnessPin(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	pin, err := readHarnessPin(root)
	if err != nil {
		t.Fatalf("readHarnessPin: %v", err)
	}
	if pin.Image != "ghcr.io/medici-finance/assay/desk-tools" || pin.Tag != "v1.0.6" || pin.Digest != testHarnessDigest {
		t.Fatalf("readHarnessPin = %+v", pin)
	}
}

func TestReadHarnessPinMissingBlockIsError(t *testing.T) {
	root := t.TempDir()
	// A paired-versions.yaml with no harness: block at all.
	dir := filepath.Join(root, "plugins", "assay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "paired-versions.yaml"), []byte("plugin: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHarnessPin(root); err == nil {
		t.Fatalf("readHarnessPin should error on a missing harness: block (could-not-check is a failure, not a default)")
	}
}

// ---------------------------------------------------------------------------
// End-to-end with a FAKE docker on PATH — no real container ever runs
// ---------------------------------------------------------------------------

func TestRunInContainerInvokesDockerWithComposedArgs(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	briefRel := filepath.Join("docs", "streams", "windows-port", "brief-10.md")
	writeFileWP10(t, filepath.Join(root, briefRel), "# brief\n")

	// A fake `docker` that records its argv and exits 0 — the wrapper's argv is the
	// unit under test, not a real container run (the offline envelope forbids one).
	argvFile := filepath.Join(root, "docker-argv.txt")
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+argvFile+"'\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(root, briefRel), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code != verifyrunExitPass {
		t.Fatalf("runInContainer exit = %d, want %d", code, verifyrunExitPass)
	}
	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("fake docker did not record its argv: %v", err)
	}
	recorded := string(got)
	for _, want := range []string{
		"run", "--rm", "-w", "/work",
		testPin().ref(),
		"statusgen", "verifyrun", "--brief", filepath.ToSlash(briefRel),
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("fake docker argv missing %q:\n%s", want, recorded)
		}
	}
	// The bind mount names this run's absolute root.
	absRoot, _ := filepath.Abs(root)
	if !strings.Contains(recorded, absRoot+":/work") {
		t.Errorf("fake docker argv missing bind mount %s:/work:\n%s", absRoot, recorded)
	}
}

func TestRunInContainerRefusesPlaceholderDigest(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: PENDING-HARVEST-x\n")
	writeFileWP10(t, filepath.Join(root, "b.md"), "# b\n")
	// A fake docker that WOULD succeed if it were ever called — proving the refusal
	// happens before any docker invocation.
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(root, "b.md"), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code == verifyrunExitPass {
		t.Fatalf("runInContainer must REFUSE a placeholder digest, got pass")
	}
}

func TestRunInContainerRefusesBriefOutsideRoot(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	other := t.TempDir()
	writeFileWP10(t, filepath.Join(other, "b.md"), "# b\n")
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(other, "b.md"), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code == verifyrunExitPass {
		t.Fatalf("a brief outside the bind-mounted root must be refused, got pass")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writePairedVersions(t *testing.T, root, digestLine string) {
	t.Helper()
	body := "plugin: \"1.0.0\"\n" +
		"harness:\n" +
		"  release_home: medici-finance/assay\n" +
		"  image: ghcr.io/medici-finance/assay/desk-tools\n" +
		"  tag: v1.0.6\n" +
		digestLine
	writeFileWP10(t, filepath.Join(root, "plugins", "assay", "paired-versions.yaml"), body)
}

func writeFileWP10(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeStubBinWP10(t *testing.T, dir, name, script string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func argvHasPair(argv []string, flag, val string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == val {
			return true
		}
	}
	return false
}

func indexOf(argv []string, s string) int {
	for i, a := range argv {
		if a == s {
			return i
		}
	}
	return -1
}
