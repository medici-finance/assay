package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskcore/writepolicy"
	"github.com/medici-finance/assay/gitcore/gitstub"
	"github.com/medici-finance/assay/gitquiet/goquiet"
)

func buildHook(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "writeguard")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if out, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("build hook: %v\n%s", err, out)
	}
	return bin
}

func fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture git %v: %v\n%s", args, err, out)
	}
}

func hookBytes(t *testing.T, bin, dir string, env []string, input []byte) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), code
}

// Pin the caches used to compile this source package before fixture HOME changes.
// With GOPATH/GOMODCACHE unset, Go otherwise derives a new, empty module cache
// from that disposable home. Every child remains offline; a missing dependency
// is a setup failure, never evidence that a policy mutation was caught.
func pinHookBuildCaches(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "env", "-json", "GOMODCACHE", "GOCACHE")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	out, err := goquiet.Cmd(cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("resolve source build caches: %v\n%s", err, out)
	}
	var caches map[string]string
	if err := json.Unmarshal(out, &caches); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GOMODCACHE", "GOCACHE"} {
		p := caches[key]
		info, err := os.Stat(p)
		if !filepath.IsAbs(p) || err != nil || !info.IsDir() {
			t.Fatalf("source build cache %s is unavailable: %q (%v)", key, p, err)
		}
		t.Setenv(key, p)
	}
	for key, value := range map[string]string{
		"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local",
		"GOPROXY": "off", "GOSUMDB": "off", "GOVCS": "off",
		"GOPRIVATE": "", "GONOPROXY": "", "GONOSUMDB": "",
	} {
		t.Setenv(key, value)
	}
}

// TestWriteHookParity executes the actual main binary. It checks independent
// expected decisions first, then compares the complete legacy JSON envelope and
// stdout/exit contract with the production adapter's direct evaluator result.
// regression: #3752
func TestWriteHookParity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX callout fixtures; Windows lexical decisions run in deskcore")
	}
	pinHookBuildCaches(t)
	bin := buildHook(t)
	base := t.TempDir()
	shared := filepath.Join(base, "shared")
	owned := filepath.Join(base, "owned")
	if err := os.Mkdir(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, shared, "init", "-q")
	fixtureGit(t, shared, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture")
	fixtureGit(t, shared, "worktree", "add", "-q", "--detach", owned, "HEAD")
	shared, _ = filepath.EvalSymlinks(shared)
	owned, _ = filepath.EvalSymlinks(owned)
	home := filepath.Join(base, "home")
	configDir := filepath.Join(home, ".config")
	if err := os.MkdirAll(filepath.Join(configDir, "assay"), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(base, "claim")
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(shared, alias); err != nil {
		t.Fatal(err)
	}
	callout := writeCallout(t, "echo block fixture-refusal")
	allowCallout := writeCallout(t, "echo allow")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("WRITEGUARD_SHARED_OK_FILE", sentinel)
	t.Setenv("WRITEGUARD_SHARED_OK", "")
	t.Setenv("WRITEGUARD_SHARED_OK_TTL", "")
	var totalPaths, totalCallouts int
	started := time.Now()
	cases := []struct {
		name, tool, target, cwd, project, claim, callout string
		deny                                             bool
	}{
		{"shared", "Write", shared + "/new", owned, owned, "", "", true},
		{"owned", "Write", owned + "/new", owned, owned, "", "", false},
		{"symlink-new-descendant", "Write", alias + "/new/child", owned, owned, "", "", true},
		{"relative-shared", "Edit", "new", shared, owned, "", "", true},
		{"carveout", "Write", shared + "/.claude/worktrees/worker/new", owned, owned, "", "", false},
		{"shell-write", "Bash", "echo x > " + shared + "/new", owned, owned, "", "", true},
		{"leading-cd-owned", "Bash", "cd " + owned + " && echo x > new", shared, owned, "", "", false},
		{"literal", "Bash", "echo 'rm " + shared + "/new'", owned, owned, "", "", false},
		{"heredoc", "Bash", "cat <<'END'\nrm " + shared + "/new\nEND", owned, owned, "", "", false},
		{"unknown-head-legacy", "Bash", "echo x > $UNKNOWN/new", shared, owned, "", "", true},
		{"shared-exemption", "Write", shared + "/new", shared, shared, "1", "", false},
		{"sentinel-before-exemption", "Write", sentinel, shared, shared, "1", "", true},
		{"callout-refusal", "Bash", "frobnicate", shared, owned, "", callout, true},
		{"callout-cannot-clear-deny", "Bash", "echo x > " + shared + "/new", shared, owned, "", allowCallout, true},
		{"configured-missing-callout", "Bash", "frobnicate", shared, owned, "", base + "/missing", true},
		{"out-of-scope-callout", "Bash", "echo hello", owned, owned, "", callout, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAUDE_PROJECT_DIR", tc.project)
			t.Setenv("WRITEGUARD_SHARED_OK", tc.claim)
			roster := "ASSAY_BLESS_LOGIN=fixture:2001\nASSAY_TRUSTED_LOGINS=fixture:2001\n"
			if tc.callout != "" {
				roster += "ASSAY_WRITEGUARD_CALLOUT=" + tc.callout + "\n"
			}
			if err := os.WriteFile(filepath.Join(configDir, "assay", "roster.env"), []byte(roster), 0o600); err != nil {
				t.Fatal(err)
			}
			field := "file_path"
			if tc.tool == "Bash" {
				field = "command"
			}
			payload, _ := json.Marshal(map[string]any{"cwd": tc.cwd, "tool_name": tc.tool, "tool_input": map[string]string{field: tc.target}})
			stdout, stderr, code := hookBytes(t, bin, owned, os.Environ(), payload)
			if code != 0 {
				t.Fatalf("hook exit %d: %s", code, stderr)
			}
			if (len(stdout) > 0) != tc.deny {
				t.Fatalf("expected deny=%v, stdout=%s stderr=%s", tc.deny, stdout, stderr)
			}
			claim := observeClaim()
			cfg := Config{SharedRoot: shared, ProjectDir: tc.project, Cwd: tc.cwd, CwdTop: worktreeToplevel(tc.cwd), SentinelPath: sentinel, Callout: tc.callout, claim: &claim}
			v, out, counts := cfg.evaluate(tc.tool, tc.target)
			totalPaths += counts.Paths
			totalCallouts += counts.Callouts
			wantDecision := writepolicy.Allow
			if tc.deny {
				wantDecision = writepolicy.Deny
			}
			if v.Block != tc.deny || out.Decision != wantDecision {
				t.Fatalf("direct: %+v / %+v", v, out)
			}
			var want []byte
			if v.Block {
				var wire hookOutput
				wire.HookSpecificOutput.HookEventName = "PreToolUse"
				wire.HookSpecificOutput.PermissionDecision = "deny"
				wire.HookSpecificOutput.PermissionDecisionReason = v.Reason
				want, _ = json.Marshal(wire)
				want = append(want, '\n')
			}
			if !bytes.Equal(stdout, want) {
				t.Fatalf("binary differs from direct import:\n got %s\nwant %s", stdout, want)
			}
			// Optional local before/after receipt, never a substitute for the expected
			// outcomes above and never required from an unavailable published binary.
			if old := os.Getenv("WRITEPOLICY_BASELINE_BIN"); old != "" {
				oldOut, oldErr, oldCode := hookBytes(t, old, owned, os.Environ(), payload)
				if oldCode != code || !bytes.Equal(oldOut, stdout) {
					t.Fatalf("baseline differs: old exit=%d stdout=%s stderr=%s; new exit=%d stdout=%s", oldCode, oldOut, oldErr, code, stdout)
				}
			}
		})
	}
	t.Logf("fixture workload cases=%d direct path-observations=%d direct callouts=%d elapsed=%s (includes child process tests; not a performance target)", len(cases), totalPaths, totalCallouts, time.Since(started))
	t.Run("all-hook-tools-and-empty-input", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", owned)
		if err := os.WriteFile(filepath.Join(configDir, "assay", "roster.env"), []byte("ASSAY_BLESS_LOGIN=fixture:2001\nASSAY_TRUSTED_LOGINS=fixture:2001\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			tool, field, target string
			deny                bool
		}{
			{"Write", "file_path", shared + "/new", true}, {"Edit", "file_path", shared + "/new", true},
			{"MultiEdit", "file_path", shared + "/new", true}, {"NotebookEdit", "notebook_path", shared + "/new", true},
			{"Read", "file_path", shared + "/new", false}, {"Write", "file_path", "", false}, {"Bash", "command", "", false},
		} {
			payload, _ := json.Marshal(map[string]any{"cwd": owned, "tool_name": tc.tool, "tool_input": map[string]string{tc.field: tc.target}})
			out, stderr, code := hookBytes(t, bin, owned, os.Environ(), payload)
			if code != 0 || (len(out) > 0) != tc.deny {
				t.Fatalf("%s protocol: %d %s %s", tc.tool, code, out, stderr)
			}
			if old := os.Getenv("WRITEPOLICY_BASELINE_BIN"); old != "" {
				oldOut, _, oldCode := hookBytes(t, old, owned, os.Environ(), payload)
				if oldCode != code || !bytes.Equal(out, oldOut) {
					t.Fatalf("%s baseline protocol changed", tc.tool)
				}
			}
		}
	})
	t.Run("callout-config-source", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", owned)
		t.Setenv("ASSAY_WRITEGUARD_CALLOUT", callout)
		if err := os.WriteFile(filepath.Join(configDir, "assay", "roster.env"), []byte("ASSAY_BLESS_LOGIN=fixture:2001\nASSAY_TRUSTED_LOGINS=fixture:2001\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(map[string]any{"cwd": shared, "tool_name": "Bash", "tool_input": map[string]string{"command": "frobnicate"}})
		out, stderr, code := hookBytes(t, bin, owned, os.Environ(), payload)
		if code != 0 || len(out) != 0 {
			t.Fatalf("ambient callout entered roster authority: %d %s %s", code, out, stderr)
		}
	})
	t.Run("production-mutations", hookPolicyMutations)
	t.Run("git-failure-activation-fails-closed", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(base, ".git"), []byte("invalid fixture gitdir"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CLAUDE_PROJECT_DIR", base)
		stdout, stderr, code := hookBytes(t, bin, base, os.Environ(), []byte("{"))
		if code != 6 || len(stdout) != 0 || !strings.Contains(string(stderr), "could-not-check") {
			t.Fatalf("rejected .git must refuse activation, never proceed: %d %s %s", code, stdout, stderr)
		}
	})
	t.Run("malformed-input-no-decision", func(t *testing.T) {
		if err := os.Remove(filepath.Join(base, ".git")); err != nil {
			t.Fatal(err)
		}
		gitstub.Repo(t, base)
		t.Setenv("CLAUDE_PROJECT_DIR", base)
		for _, data := range []string{"{", `{"tool_name":"Write","tool_input":"malformed"}`, `{"tool_name":"Write","tool_input":{"file_path":"new"}}`} {
			stdout, stderr, code := hookBytes(t, bin, base, os.Environ(), []byte(data))
			if code != 0 || len(stdout) != 0 {
				t.Fatalf("no-decision changed: %d %s %s", code, stdout, stderr)
			}
		}
	})
	t.Run("activation-refuses-before-policy", func(t *testing.T) {
		manifest := []byte("component: fixture/guard\nprovides: [assay.desk.verbs]\ninject:\n  required:\n    - key: fixture.missing\n")
		p := filepath.Join(owned, "component.yaml")
		if err := os.WriteFile(p, manifest, 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := hookBytes(t, bin, owned, os.Environ(), []byte("{"))
		if code != 6 || len(stdout) != 0 || !strings.Contains(string(stderr), "inactive") {
			t.Fatalf("activation: %d %s %s", code, stdout, stderr)
		}
	})
}

// Each mutant changes the production evaluator used by the built child hook.
// The child selects only its named fixture, so it cannot recurse into this test.
func hookPolicyMutations(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	deny := "if v.Block {\n\t\tresult = outcome(Deny, v.Code)"
	for _, m := range []struct {
		name, old, replacement, test, diagnostic string
		core                                     bool
	}{
		{"skipped-deny", deny, "if false && v.Block {\n\t\tresult = outcome(Deny, v.Code)", "^TestWriteHookParity/shared$", "expected deny=true", false},
		{"exemption-before-sentinel", "v := c.CheckClaimSentinel(r.Tool, r.Target)", "c.SharedOK = c.claimed(in.Claim)\n v := verdict{}\n if !c.Exempt() { v = c.CheckClaimSentinel(r.Tool, r.Target) }", "^TestWriteHookParity/sentinel-before-exemption$", "expected deny=true", false},
		{"callout-clears-deny", deny, "if v.Block && in.Callout.State != CalloutUnconfigured { v = verdict{}; needCallout = in.Callout.State == CalloutPending }\n\t" + deny, "^TestWriteHookParity/callout-cannot-clear-deny$", "expected deny=true", false},
		{"managed-inherits-fallback", "if managed && !validManaged(in) {\n\t\treturn outcome(Unknown, ManagedHold)", "if managed && !validManaged(in) {\n\t\treturn outcome(Allow, NoObjection)", "^TestWritePolicyCases/managed-never-inherits-legacy-fallback$", "FAIL: TestWritePolicyCases/managed-never", true},
		{"managed-unknown-allows", "if managed && unresolved {\n\t\treturn outcome(Unknown, ManagedHold)", "if managed && unresolved {\n\t\treturn outcome(Allow, NoObjection)", "^TestWritePolicyCases/managed-never-inherits-legacy-fallback$", "unknown outside shared inherited allow", true},
		{"classification-borrows-environment", "lex.Env = env", "lex.Env = lex.Env", "^TestClassificationOwnsItsSnapshot$", "FAIL: TestClassificationOwnsItsSnapshot", true},
		// #4821 positive control: a matcher that never fires must fail the hook-level refusal fixture.
		{"ambient-token-unmatched", "return forgeTokenRead(c.request.Target)", "return false", "^TestAmbientTokenRefusedLoop$", "an ambient gh token read must be refused", false},
		// #5078 positive control: the same, for the gh output filter check.
		{"gh-filter-unmatched", "return ghFilterReads(c.request.Target)", "return false", "^TestHookGhFilterRefused$", "a gh filter reading outside its output must be refused", false},
	} {
		t.Run(m.name, func(t *testing.T) {
			dst := t.TempDir()
			core := filepath.Join(dst, "deskcore")
			if err := filepath.WalkDir(filepath.Join(root, "deskcore"), func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(root, "deskcore"), p)
				if err != nil {
					return err
				}
				target := filepath.Join(core, rel)
				if d.IsDir() {
					return os.MkdirAll(target, 0o755)
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				return os.WriteFile(target, b, 0o600)
			}); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(core, "writepolicy", "policy.go")
			b, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(b), m.old) != 1 {
				t.Fatal("mutation anchor drifted")
			}
			dir, pkg := filepath.Join(root, "tools", "desk"), "./cmd/writeguard"
			env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=", "WRITEPOLICY_BASELINE_BIN=")
			if m.core {
				dir, pkg = core, "./writepolicy"
			} else {
				// A mutation fixture may replace source. The independent consumer
				// test separately resolves a module zip with no replacement.
				mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
				if err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"comms", "forgeread", "perfrec", "gitquiet", "deskcore"} {
					target := filepath.Join(root, name)
					if name == "deskcore" {
						target = core
					}
					mod = bytes.ReplaceAll(mod, []byte("=> ../../"+name), []byte("=> "+strconv.Quote(target)))
				}
				modfile := filepath.Join(dst, "go.mod")
				if err := os.WriteFile(modfile, mod, 0o600); err != nil {
					t.Fatal(err)
				}
				sums, err := os.ReadFile(filepath.Join(dir, "go.sum"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dst, "go.sum"), sums, 0o600); err != nil {
					t.Fatal(err)
				}
				env = append(env, "GOFLAGS=-modfile="+modfile)
			}
			run := func(childEnv []string) ([]byte, error) {
				cmd := exec.Command("go", "test", "-v", "-count=1", "-timeout=45s", "-run", m.test, pkg)
				cmd.Dir, cmd.Env = dir, childEnv
				return goquiet.Cmd(cmd).CombinedOutput()
			}
			fixture := strings.Trim(m.test, "^$")
			out, err := run(env)
			if err != nil || !strings.Contains(string(out), "--- PASS: "+fixture+" ") {
				t.Fatalf("unmutated child fixture did not pass: %v\n%s", err, out)
			}
			t.Log("unmutated child passed the same named fixture offline")
			if m.name == "skipped-deny" {
				// Reproduce the CI failure with an empty cache, independently of
				// HOME. It must remain a setup error, not a successful mutation kill.
				cold, coldErr := run(append(env, "GOMODCACHE="+t.TempDir()))
				if coldErr == nil || !strings.Contains(string(cold), "module lookup disabled by GOPROXY=off") ||
					strings.Contains(string(cold), m.diagnostic) || strings.Contains(string(cold), "--- FAIL: "+fixture+" ") {
					t.Fatalf("empty-cache control was not a distinct setup failure: %v\n%s", coldErr, cold)
				}
				t.Log("empty-cache control failed in setup, without a policy-failure witness")
			}
			if err := os.WriteFile(source, []byte(strings.Replace(string(b), m.old, m.replacement, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err = run(env)
			if err == nil || !strings.Contains(string(out), m.diagnostic) || !strings.Contains(string(out), "--- FAIL: "+fixture+" ") {
				t.Fatalf("production mutant escaped or failed for another reason: %v\n%s", err, out)
			}
			t.Log("production mutant caught by independently specified fixture")
		})
	}
	t.Run("terminal-unknown-allows", hookUnknownOutcomeMutation)
}

// The overlay changes the production adapter file, not a fixture-only mapper.
// It is confined to this child build; the separate consumer proof uses no overlay.
func hookUnknownOutcomeMutation(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "tools", "desk", "cmd", "writeguard", "observe.go")
	b, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "case writepolicy.Allow:"
	if strings.Count(string(b), anchor) != 1 {
		t.Fatal("adapter mutation anchor drifted")
	}
	dir := t.TempDir()
	copyPath := filepath.Join(dir, "observe.go")
	if err := os.WriteFile(copyPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{source: copyPath}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() ([]byte, error) {
		cmd := exec.Command("go", "test", "-overlay", overlayPath, "-v", "-count=1", "-timeout=45s", "-run", "^TestHookOutcomeMapping$", "./cmd/writeguard")
		cmd.Dir = filepath.Join(root, "tools", "desk")
		cmd.Env = append(os.Environ(), "GOFLAGS=", "WRITEPOLICY_BASELINE_BIN=")
		return goquiet.Cmd(cmd).CombinedOutput()
	}
	if out, err := run(); err != nil || !strings.Contains(string(out), "--- PASS: TestHookOutcomeMapping ") {
		t.Fatalf("unmutated adapter fixture did not pass: %v\n%s", err, out)
	}
	t.Log("unmutated child passed the same named fixture offline")
	if err := os.WriteFile(copyPath, []byte(strings.Replace(string(b), anchor, "case writepolicy.Allow, writepolicy.Unknown:", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := run()
	if err == nil || !strings.Contains(string(out), "terminal unknown was allowed or lost its reason") ||
		!strings.Contains(string(out), "--- FAIL: TestHookOutcomeMapping/invalid-observation ") {
		t.Fatalf("adapter mutant escaped or failed for another reason: %v\n%s", err, out)
	}
	t.Log("production mutant caught by independently specified fixture")
}
