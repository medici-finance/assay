package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/gitcore/testenv"
)

// installStatusgenFixture is called at the test's entry, before constructing
// repositories or planting deliberate environment controls. Constructors must
// not call it again: doing so could erase a negative control installed by a
// caller after fixture setup.
func installStatusgenFixture(t *testing.T, overlays ...testenv.Options) *testenv.Fixture {
	t.Helper()
	opts := testenv.Options{Tools: map[string]string{}, Vars: map[string]string{}}
	for _, name := range []string{"git", "go", "sh", "bash", "cat", "grep", "touch", "env", "tr", "sleep"} {
		path, err := exec.LookPath(name)
		if err != nil {
			if name == "git" || name == "go" {
				t.Fatalf("resolve fixture tool %s: %v", name, err)
			}
			continue // A test that needs this optional platform tool still fails.
		}
		path, err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		opts.Tools[name] = path
	}
	// TestMain resolved these before replacing HOME. Do not let a fresh HOME
	// select an empty dependency cache or make a test fetch dependencies.
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		if value := os.Getenv(key); value != "" {
			opts.Vars[key] = value
		}
	}
	for _, overlay := range overlays {
		for name, path := range overlay.Tools {
			opts.Tools[name] = path
		}
		for key, value := range overlay.Vars {
			opts.Vars[key] = value
		}
	}
	// Cleanup runs in reverse order: reload only after Install restores the
	// previous process environment and its config home.
	t.Cleanup(scanReloadConfig)
	f := testenv.Install(t, opts)
	config := filepath.Join(f.Home, ".config", "assay")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "roster.env"), []byte(fixtureRoster+darFixtureExtra()), 0600); err != nil {
		t.Fatal(err)
	}
	scanReloadConfig()
	return f
}

// The outer process supplies the same injected Git profile that exposed the
// class. Actual fixture consumers must execute and pass; a cleaned-up toy
// environment or a skipped named test does not satisfy this regression.
func TestStatusgenFixtureHostileParent(t *testing.T) {
	f := installStatusgenFixture(t)
	victim := filepath.Join(t.TempDir(), "caller-global-config")
	want := []byte("[fixture]\n\tcaller = unchanged\n")
	if err := os.WriteFile(victim, want, 0600); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hostile := f.Env("GIT_CONFIG_GLOBAL="+victim, "GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default",
		"GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false",
		"GIT_NO_REPLACE_OBJECTS=1")
	cmd := exec.Command(self, "-test.count=1", "-test.v", "-test.run=^(TestTransitionChain|TestCoverageGitReadLifetime|TestFlowCard)$")
	cmd.Env = hostile
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture consumers under hostile parent: %v\n%s", err, out)
	}
	for _, name := range []string{"TestTransitionChain", "TestCoverageGitReadLifetime", "TestFlowCard"} {
		if !bytes.Contains(out, []byte("--- PASS: "+name+" ")) || bytes.Contains(out, []byte("--- SKIP: "+name)) {
			t.Fatalf("named fixture consumer did not execute: %s\n%s", name, out)
		}
	}
	// The unchanged inherited-environment control must expose the defect in
	// this very launcher, rather than letting a harmless parent make it green.
	control := exec.Command(self, "-test.count=1", "-test.v", "-test.run=^TestStatusgenFixtureInheritedControl$")
	control.Env = append(hostile, "STATUSGEN_FIXTURE_INHERIT_CONTROL=1")
	controlOut, err := control.CombinedOutput()
	if err == nil || !bytes.Contains(controlOut, []byte("inherited Git profile prevented a fixture source revision")) {
		t.Fatalf("inherited-environment control did not expose the defect: %v\n%s", err, controlOut)
	}
	got, err := os.ReadFile(victim)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("caller global config changed: err=%v", err)
	}
}

func TestStatusgenFixtureInheritedControl(t *testing.T) {
	if os.Getenv("STATUSGEN_FIXTURE_INHERIT_CONTROL") != "1" {
		t.Skip("negative control is launched only by TestStatusgenFixtureHostileParent")
	}
	// Deliberately omit Install here. Child Git setup strips injection, while
	// the in-process reader still sees the hostile caller's profile.
	root, _ := vfFixture(t, vfDefaults())
	brief, ok, err := parseBriefFile(vfBrief(root))
	if err != nil || !ok {
		t.Fatalf("parse inherited control fixture: %v", err)
	}
	if makeGateCard(root, brief, cardAcceptance).SourceRevision == "" {
		t.Fatal("inherited Git profile prevented a fixture source revision")
	}
}

// Fixture isolation changes only test setup. The real authority owners must
// still reject explicitly injected configuration applied after installation.
func TestFixtureAuthorityControls(t *testing.T) {
	installStatusgenFixture(t)
	root, _ := vfFixture(t, vfDefaults())
	runGit(t, root, "remote", "add", "origin", "https://github.com/"+rlRepo+".git")
	if _, err := gitcore.OriginAuthority(root); err != nil {
		t.Fatalf("owned fixture lacks its positive authority control: %v", err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/foreign/project.git")
	if gitCoreFor(root).native == "" {
		t.Fatal("injected configuration no longer selects the unsupported native profile")
	}
	if _, err := gitcore.OriginAuthority(root); err == nil || !strings.Contains(err.Error(), "injected configuration") {
		t.Fatalf("injected origin configuration was not rejected: %v", err)
	}
}
