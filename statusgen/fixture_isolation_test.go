package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	opts := testenv.Options{Tools: map[string]string{}, Vars: map[string]string{
		"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOENV": "off",
	}}
	for _, name := range []string{"git", "go", "sh", "bash", "cat", "grep", "touch", "env", "tr", "sleep", "printenv", "sed", "awk", "wc", "mkdir", "mktemp", "rm", "date", "basename", "ls", "cut", "tee", "curl", "perl", "python3", "mkfifo", "jq"} {
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
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT", "SG_OLD_BIN"} {
		if value := os.Getenv(key); value != "" {
			opts.Vars[key] = value
		}
	}
	// Keep the existing missing-tool CI assertion; isolation must not turn
	// a required CI execution into a developer-only skip.
	if os.Getenv("CI") != "" {
		opts.Vars["CI"] = "true"
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

// These consumers failed before the remaining fixture entry points declared
// their environment. Keep the parent's repository/config independent of the
// fixtures, and prove the launcher also exposes an intentionally inherited
// forge marker rather than silently clearing the hostile input for every child.
func TestFixtureSiblingParentIsolation08(t *testing.T) {
	f := installStatusgenFixture(t)
	victim := t.TempDir()
	repo := filepath.Join(victim, "repo")
	global := filepath.Join(victim, "global")
	globalBody := []byte("[fixture]\n\tparent = unchanged\n")
	if err := os.WriteFile(global, globalBody, 0600); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("independent victim setup: %v: %s", err, out)
		}
	}
	run(f.Env(), "init", "-q", repo)
	run(f.Env(), "-C", repo, "remote", "add", "origin", "https://github.com/example-org/fixture.git")
	config := filepath.Join(repo, ".git", "config")
	configBody, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	selectors := []string{"GIT_DIR=" + filepath.Join(repo, ".git"), "GIT_WORK_TREE=" + repo,
		"GIT_INDEX_FILE=" + filepath.Join(repo, ".git", "index"), "GIT_CONFIG_GLOBAL=" + global}
	// A separately constructed unsanitized control really can reach both
	// victims; restore their original bytes before the isolated consumers run.
	run(f.Env(selectors...), "config", "fixture.control", "reached")
	run(f.Env(selectors...), "config", "--global", "fixture.control", "reached")
	for path, original := range map[string][]byte{config: configBody, global: globalBody} {
		body, err := os.ReadFile(path)
		if err != nil || bytes.Equal(body, original) || !bytes.Contains(body, []byte("reached")) {
			t.Fatal("unsanitized control did not reach the disposable victim")
		}
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := fixtureSiblingTree08(t, victim)
	pins := []string{"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default",
		"GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false", "GIT_NO_REPLACE_OBJECTS=1"}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"TestAdmittedRowsDropForgeTokens", "TestDecayRunsWithoutGhOnPath", "TestDerivedHistorySquash", "TestEvidenceRepairChain"}
	cmd := exec.Command(self, "-test.count=1", "-test.v", "-test.timeout=3m", "-test.run=^("+strings.Join(names, "|")+")$")
	cmd.Env = f.Env(append(selectors, pins...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("actual sibling consumers under hostile parent: %v\n%s", err, out)
	}
	for _, name := range names {
		if !bytes.Contains(out, []byte("--- PASS: "+name+" ")) || bytes.Contains(out, []byte("--- SKIP: "+name)) {
			t.Fatalf("required consumer did not execute: %s\n%s", name, out)
		}
	}
	control := exec.Command(self, "-test.count=1", "-test.v", "-test.run=^TestFixtureSiblingInheritedControl08$")
	control.Env = f.Env(append(pins, "STATUSGEN08_INHERIT_CONTROL=1")...)
	controlOut, controlErr := control.CombinedOutput()
	if controlErr == nil || !bytes.Contains(controlOut, []byte("inherited row marker prevented fixture GraphQL")) {
		t.Fatalf("inherited control did not expose the fixture defect: %v\n%s", controlErr, controlOut)
	}
	strict := exec.Command(self, "-test.count=1", "-test.v", "-test.run=^TestFixtureCIControl08$")
	strict.Env = f.Env("CI=true", "STATUSGEN08_CI_CONTROL=1")
	strictOut, strictErr := strict.CombinedOutput()
	if strictErr == nil || !bytes.Contains(strictOut, []byte("under CI this test must run, not skip")) || bytes.Contains(strictOut, []byte("--- SKIP: TestFixtureCIControl08")) {
		t.Fatalf("fixture setup lost CI missing-tool strictness: %v\n%s", strictErr, strictOut)
	}
	if _, err := gitcore.OriginAuthority(repo); err != nil {
		t.Fatalf("owned authority control: %v", err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/foreign/project.git")
	if _, err := gitcore.OriginAuthority(repo); err == nil || !strings.Contains(err.Error(), "injected configuration") {
		t.Fatalf("deliberate authority injection stopped being rejected: %v", err)
	}
	if !reflect.DeepEqual(before, fixtureSiblingTree08(t, victim)) {
		t.Fatal("isolated sibling consumers changed the independent parent repository or config")
	}
}

func TestFixtureSiblingInheritedControl08(t *testing.T) {
	if os.Getenv("STATUSGEN08_INHERIT_CONTROL") != "1" {
		t.Skip("negative control is launched only by the sibling parent regression")
	}
	// Deliberately omit Install. This is the original fake-forge fixture flow;
	// inherited admitted-row markers take precedence over its planted token.
	offlineGitHubEnv(t, "fixture-08-token")
	fakeGraphQL(t, func(int, map[string]any) (int, string) {
		return 200, branchPage(false, "", map[string]string{"fix/alpha-01-dead": "CLOSED"})
	})
	if _, reason := decayDeadClaims("/repo", []string{"fix/alpha-01-dead"}); reason != "" {
		t.Fatalf("inherited row marker prevented fixture GraphQL: %s", reason)
	}
}

func fixtureSiblingTree08(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var body []byte
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			body = []byte(target)
		} else if info.Mode().IsRegular() {
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("%v:%x", info.Mode(), sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// This probe is executed as an expected failure by the parent regression.
func TestFixtureCIControl08(t *testing.T) {
	if os.Getenv("STATUSGEN08_CI_CONTROL") != "1" {
		t.Skip("CI failure probe is launched only by the parent regression")
	}
	installStatusgenFixture(t)
	requireTool(t, "fixture-tool-intentionally-absent")
}
