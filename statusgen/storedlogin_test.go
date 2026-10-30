package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Inside an admitted verifyrun no forge read falls back to a stored login
// (forgecredential.go). Each call site of the ruling-read client is driven
// twice against a faked forge reached through http.DefaultTransport, with a
// stored login present (a stub `gh` on PATH that answers `auth token` with an
// obvious placeholder) and no forge token variable set:
//
//   - admitted: the environment an admitted Verify row runs in
//     (admittedRowEnv over this process's own) — no request is made and the
//     site reports could-not-check;
//   - control: the same with no admission signal — the fallback still works
//     there, and the placeholder reaches the forge.

// slStoredToken is what the stub stored login answers. It is not a credential.
const slStoredToken = "placeholder-stored-login"

// slStubGH puts a `gh` on PATH that answers `auth token` with slStoredToken
// and logs every invocation to the returned file.
func slStubGH(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the stub gh is a #!/bin/sh script on PATH")
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\n" +
		"if [ \"$1 $2\" = \"auth token\" ]; then echo " + slStoredToken + "; exit 0; fi\necho '{}'\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// slUnset removes each variable for the rest of the test.
func slUnset(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
}

// slAdmittedRow makes this process's environment the one an admitted Verify
// row runs in: admittedRowEnv over the current environment, with no forge
// token variable, no verify loop and a working directory outside any
// repository, so the row's environment is the only admission signal. A
// stored login is on PATH. It returns the stub's call log.
func slAdmittedRow(t *testing.T) string {
	t.Helper()
	log := slStubGH(t)
	t.Chdir(t.TempDir())
	t.Setenv("DESK_LOOP", "")
	slUnset(t, append(forgeTokenVars, "CELLCTL_GH_AMBIENT")...)
	keep := map[string]bool{}
	for _, kv := range admittedRowEnv(os.Environ()) {
		k, v, _ := strings.Cut(kv, "=")
		keep[k] = true
		t.Setenv(k, v)
	}
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); k != "" && !keep[k] {
			slUnset(t, k)
		}
	}
	return log
}

// slOutside is the control: the same stored login and no token variable, but
// no admission signal at all.
func slOutside(t *testing.T) string {
	t.Helper()
	log := slStubGH(t)
	t.Chdir(t.TempDir())
	t.Setenv("DESK_LOOP", "")
	slUnset(t, append(forgeTokenVars, "CELLCTL_GH_AMBIENT")...)
	for _, kv := range admittedRowEnv(nil) {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "GIT_CONFIG_") {
			slUnset(t, k)
		}
	}
	return log
}

// slTransport routes every default-transport request to the faked forge and
// records the Authorization header each one carried.
type slTransport struct {
	srv   *url.URL
	next  http.RoundTripper
	mu    sync.Mutex
	auths []string
}

func (s *slTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.auths = append(s.auths, r.Header.Get("Authorization"))
	s.mu.Unlock()
	r2 := r.Clone(r.Context())
	r2.URL.Scheme, r2.URL.Host, r2.Host = s.srv.Scheme, s.srv.Host, ""
	return s.next.RoundTrip(r2)
}

// slForge installs f behind http.DefaultTransport, which the production
// ruling-read client uses.
func slForge(t *testing.T, f *rlForge) *slTransport {
	t.Helper()
	c := f.client(t)
	u, err := url.Parse(c.base)
	if err != nil {
		t.Fatal(err)
	}
	rt := &slTransport{srv: u, next: c.doer.(*http.Client).Transport}
	old := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = old })
	return rt
}

// check asserts what the forge saw: nothing at all when admitted, the stored
// login's placeholder on the control.
func (s *slTransport) check(t *testing.T, admitted bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	sent := false
	for _, a := range s.auths {
		sent = sent || a == "Bearer "+slStoredToken
	}
	if admitted && len(s.auths) > 0 {
		t.Errorf("admitted: %d forge request(s) made (stored login sent: %v), want none", len(s.auths), sent)
	}
	if !admitted && !sent {
		t.Errorf("control: the stored login never reached the forge (%q), so the fallback no longer works outside admission", s.auths)
	}
}

// slModes runs body once as an admitted row and once as the control. Each body
// builds its fixture before it switches the environment.
func slModes(t *testing.T, body func(t *testing.T, admitted bool)) {
	t.Helper()
	for _, admitted := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admitted], func(t *testing.T) {
			body(t, admitted)
		})
	}
}

// regression: #4274
// TestStoredLoginRulingRecord: rulingrecord.go's client
// (rulingRecordClientFn, `--corroborate-record`).
func TestStoredLoginRulingRecord(t *testing.T) {
	slModes(t, func(t *testing.T, admitted bool) {
		root := rrSetup(t, rrBoundForge())
		rulingRecordClientFn = rulingForgeClient
		rt := slForge(t, rrBoundForge())
		if admitted {
			slAdmittedRow(t)
		} else {
			slOutside(t)
		}
		got := runCorroborateRecord(root, rlFile, rrBriefRel)
		rt.check(t, admitted)
		if want := map[bool]int{true: 2, false: 0}[admitted]; got != want {
			t.Errorf("exit %d, want %d (2 = could-not-check)", got, want)
		}
	})
}

// regression: #4274
// TestStoredLoginDecisionGate: decisiongatehold.go's client
// (decisionGateClientFn, `--decision-gate`).
func TestStoredLoginDecisionGate(t *testing.T) {
	slModes(t, func(t *testing.T, admitted bool) {
		forge := dhForge("Option 1 for " + dhBoardID + ", as written.")
		dhSeams(t, forge)
		decisionGateClientFn = rulingForgeClient
		root := dhFixture(t, unruledBase())
		dhWrite(t, root, map[string]string{
			dhReadmePath: dhReadme(dhRow("18", "implemented")),
			dhBriefPath:  dhFM("gate: human", dhRuledLines()),
		})
		rt := slForge(t, forge)
		if admitted {
			slAdmittedRow(t)
		} else {
			slOutside(t)
		}
		code, out := dhGate(t, root)
		rt.check(t, admitted)
		if admitted && (code == 0 || !strings.Contains(out, "could-not-check")) {
			t.Errorf("admitted: exit %d, want a could-not-check refusal:\n%s", code, out)
		}
		if !admitted && code != 0 {
			t.Errorf("control: exit %d, want 0:\n%s", code, out)
		}
	})
}

// regression: #4274
// TestStoredLoginAcceptance: acceptancetransition.go's client (the
// acceptance-approval read behind --acceptance-forge).
func TestStoredLoginAcceptance(t *testing.T) {
	slModes(t, func(t *testing.T, admitted bool) {
		f := transitionFixtureAt(t)
		f.evidence(t, nil)
		transitionClient = nil
		rt := slForge(t, f.forge)
		if admitted {
			slAdmittedRow(t)
		} else {
			slOutside(t)
		}
		verify, evidence, err := briefSections(f.path)
		if err != nil {
			t.Fatal(err)
		}
		findings, why := closureWitnesses(verify, evidence, f.path)
		pass := why == "" && len(findings) == 3
		for _, v := range findings {
			pass = pass && closureSatisfied(v)
		}
		rt.check(t, admitted)
		if pass == admitted {
			t.Errorf("closure accepted=%v, want %v: %s %+v", pass, !admitted, why, findings)
		}
		if admitted && !strings.Contains(why, "could-not-check") {
			t.Errorf("admitted: refusal %q does not say could-not-check", why)
		}
	})
}

// regression: #4274
// TestStoredLoginCorroborate: corroborate.go's ruling lane (`--corroborate`
// on a pull request that adds a decision record).
func TestStoredLoginCorroborate(t *testing.T) {
	slModes(t, func(t *testing.T, admitted bool) {
		rlWithRoster(t)
		root := t.TempDir()
		gitInit(t, root, "fixture", "fixture@example.invalid")
		rlWriteRecord(t, root, rlFile, rlRecordID)
		rt := slForge(t, defaultRLForge())
		if admitted {
			slAdmittedRow(t)
		} else {
			slOutside(t)
		}
		st := &corroborateStub{repo: rlRepo, files: []ghPRFile{{Filename: rlFile, Patch: "@@ -0,0 +1 @@\n+id: " + rlRecordID}}}
		var code int
		out := captureStdout(t, func() { code = runCorroborateOn(t, root, st) })
		rt.check(t, admitted)
		if admitted && (code == 0 || !strings.Contains(out, "could-not-check")) {
			t.Errorf("admitted: exit %d, want a could-not-check result:\n%s", code, out)
		}
		if !admitted && strings.Contains(out, "could-not-check") {
			t.Errorf("control: the ruling was not resolved:\n%s", out)
		}
	})
}

// regression: #4274
// TestStoredLoginClaimDecay: claimdecay_github.go's credential
// (githubDecayToken), for github.com and an enterprise host alike.
func TestStoredLoginClaimDecay(t *testing.T) {
	for _, c := range []struct {
		endpoint string
		derived  bool
	}{{"https://api.github.com/graphql", false}, {"https://github.example.invalid/api/graphql", true}} {
		t.Run(githubCredentialHost(c.endpoint), func(t *testing.T) {
			slModes(t, func(t *testing.T, admitted bool) {
				log := slOutside(t)
				if admitted {
					log = slAdmittedRow(t)
				}
				tok, source, _ := githubDecayToken(c.endpoint, c.derived)
				_, statErr := os.Stat(log)
				if admitted && (tok != "" || statErr == nil) {
					t.Errorf("admitted %s: token %q from %q (stored login started: %v), want none", c.endpoint, tok, source, statErr == nil)
				}
				if !admitted && tok != slStoredToken {
					t.Errorf("control %s: token %q from %q, want the stored login", c.endpoint, tok, source)
				}
			})
		})
	}
}

// regression: #4274
// TestStoredLoginVerifyrunState: the verifyrun process's own reads. Admission
// is established by the attestation file in the home's git directory, with the
// working directory elsewhere and no verify loop, so only the run's in-process
// admission state can refuse the stored login for its acceptance-approval
// read. The control is the same run without the attestation file.
func TestStoredLoginVerifyrunState(t *testing.T) {
	slModes(t, func(t *testing.T, admitted bool) {
		f := transitionFixtureAt(t)
		vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
		vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
		descriptor := filepath.Join(t.TempDir(), "transition.txt")
		if err := os.WriteFile(descriptor, []byte(f.descriptor), 0o600); err != nil {
			t.Fatal(err)
		}
		transitionClient = nil
		rt := slForge(t, f.forge)
		slOutside(t)
		if admitted {
			gitPath, err := exec.Command("git", "-C", f.root, "rev-parse", "--path-format=absolute", "--git-path", "assay-verifier-attestation.json").Output()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(strings.TrimSpace(string(gitPath)), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			receipt := `{"Issue":7,"Binding":{"Version":1,"Run":"r","Repo":"example-org/one","Source":"s","Brief":"brief.md","Model":"m","Tier":"strong"}}`
			if err := os.WriteFile(filepath.Join(bin, "deskdispatch"), []byte("#!/bin/sh\necho '"+receipt+"'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
		resetReadMemo()
		res, out := captureVerifyrunOut(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge", "--rows", "2,3", "--dry-run"})
		rt.check(t, admitted)
		if admitted && (res.code == verifyrunExitPass || !strings.Contains(out, "could-not-check") || !strings.Contains(out, "inside an admitted verifyrun")) {
			t.Errorf("admitted: exit %d, want a could-not-check refusal:\n%s", res.code, out)
		}
		if !admitted && res.code != verifyrunExitPass {
			t.Errorf("control: exit %d, want pass:\n%s", res.code, out)
		}
	})
}

// regression: #4274
// TestStoredLoginDecider pins each signal storedLoginRefusal reads, one at a
// time, against the no-signal baseline.
func TestStoredLoginDecider(t *testing.T) {
	cases := []struct {
		name   string
		set    func(t *testing.T)
		refuse bool
	}{
		{"outside", func(t *testing.T) {}, false},
		{"pending", func(t *testing.T) { slState(t, verifyrunPending) }, true},
		{"admitted", func(t *testing.T) { slState(t, verifyrunAdmitted) }, true},
		{"unadmitted", func(t *testing.T) { slState(t, verifyrunUnadmitted) }, false},
		{"row-env", func(t *testing.T) { slAdmittedRow(t) }, true},
		{"verify-loop", func(t *testing.T) { t.Setenv("DESK_LOOP", "verify-desk") }, true},
		{"other-loop", func(t *testing.T) { t.Setenv("DESK_LOOP", "worker-desk") }, false},
		{"attested-cwd", func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "fixture", "fixture@example.invalid")
			if err := os.WriteFile(filepath.Join(root, ".git", "assay-verifier-attestation.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
		}, true},
		{"no-git", func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			slOutside(t)
			c.set(t)
			if got := storedLoginRefusal() != ""; got != c.refuse {
				t.Fatalf("refused=%v, want %v (%q)", got, c.refuse, storedLoginRefusal())
			}
		})
	}
}

func slState(t *testing.T, s int32) {
	prev := verifyrunAdmission.Swap(s)
	t.Cleanup(func() { verifyrunAdmission.Store(prev) })
}

// regression: #4274
// TestStoredLoginGhGuard: what a forge CLI launch receives. Refused with no
// token variable, the command never starts; refused with one, it runs without
// CELLCTL_GH_AMBIENT and with the unset family closed by a placeholder;
// allowed, it is untouched.
func TestStoredLoginGhGuard(t *testing.T) {
	t.Run("refused-no-token", func(t *testing.T) {
		log := slAdmittedRow(t)
		out, err := ghGuard(exec.Command("gh", "auth", "token")).Output()
		if err == nil || !strings.Contains(err.Error(), "could-not-check") {
			t.Fatalf("err = %v (output %q), want the could-not-check refusal", err, out)
		}
		if _, statErr := os.Stat(log); statErr == nil {
			t.Fatal("the stub gh was started")
		}
	})
	t.Run("refused-with-token", func(t *testing.T) {
		slAdmittedRow(t)
		t.Setenv("GH_TOKEN", "fixture-gh_token")
		t.Setenv("CELLCTL_GH_AMBIENT", "fixture-ambient")
		cmd := ghGuard(exec.Command("gh", "auth", "token"))
		env := strings.Join(cmd.Env, "\n")
		if cmd.Err != nil || strings.Contains(env, "CELLCTL_GH_AMBIENT=") || !strings.Contains(env, "GH_ENTERPRISE_TOKEN="+storedLoginPlaceholder) || !strings.Contains(env, "GH_TOKEN=fixture-gh_token") {
			t.Fatalf("err %v env:\n%s", cmd.Err, env)
		}
	})
	t.Run("allowed", func(t *testing.T) {
		slOutside(t)
		t.Setenv("CELLCTL_GH_AMBIENT", "fixture-ambient")
		if cmd := ghGuard(exec.Command("gh", "auth", "token")); cmd.Err != nil || cmd.Env != nil {
			t.Fatalf("an allowed launch was changed: err %v env %v", cmd.Err, cmd.Env)
		}
	})
}

// ghLaunchesUnguarded returns "file:line" for every exec.Command or
// exec.CommandContext naming "gh" that is not the direct argument of ghGuard.
func ghLaunchesUnguarded(t *testing.T, files map[string]string) []string {
	t.Helper()
	var stray []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		guarded := map[ast.Node]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "ghGuard" && len(c.Args) == 1 {
					guarded[c.Args[0]] = true
				}
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || guarded[c] {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
				return true
			}
			at := map[string]int{"Command": 0, "CommandContext": 1}
			i, ok := at[sel.Sel.Name]
			if !ok || len(c.Args) <= i {
				return true
			}
			if lit, ok := c.Args[i].(*ast.BasicLit); ok {
				if v, _ := strconv.Unquote(lit.Value); v == "gh" {
					stray = append(stray, fset.Position(c.Pos()).String())
				}
			}
			return true
		})
	}
	return stray
}

// regression: #4274
// TestStoredLoginGhLaunches: every forge CLI launch in statusgen receives the
// decision. A planted unguarded launch proves the scan can fail.
func TestStoredLoginGhLaunches(t *testing.T) {
	planted := map[string]string{"planted.go": "package main\nimport \"os/exec\"\nfunc a() { exec.Command(\"gh\", \"auth\", \"token\").Run() }\nfunc b() { ghGuard(exec.Command(\"gh\", \"api\")).Run() }\n"}
	if got := ghLaunchesUnguarded(t, planted); len(got) != 1 || !strings.HasPrefix(got[0], "planted.go:3:") {
		t.Fatalf("planted control: %v, want exactly planted.go:3", got)
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		files[n] = string(b)
	}
	if got := ghLaunchesUnguarded(t, files); len(got) != 0 {
		t.Fatalf("forge CLI launches not passed through ghGuard:\n%s", strings.Join(got, "\n"))
	}
}
