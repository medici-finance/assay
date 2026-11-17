package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// An admitted verifyrun reads the forge only with the verifier's own
// run-scoped read-only credential, minted by the run itself (#6746). A fake
// desktoken on PATH plays the minter. A decoy forge variable and a stored
// login are both present, and neither may reach the forge.

const (
	rcMinted = "example-minted-read"
	rcDecoy  = "example-ambient-decoy"
	rcRead   = `{"issues":"read","metadata":"read","pull_requests":"read"}`
)

// rcMinter puts a fake desktoken first on PATH and points the mint seam back
// at the real exec path. mode picks its behaviour: "ok" writes the
// credential with an all-read grant, "write" records a write grant, "fail"
// exits non-zero and "elsewhere" prints another path. It returns the
// file the fake logs its arguments and environment to.
func rcMinter(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the fake desktoken is a #!/bin/sh script on PATH")
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	perms := rcRead
	if mode == "write" {
		perms = `{"metadata":"read","pull_requests":"write"}`
	}
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\nenv >> '" + log + ".env'\n"
	switch mode {
	case "fail":
		script += "echo 'desktoken: refused' >&2\nexit 3\n"
	case "elsewhere":
		script += "echo /elsewhere\n"
	default:
		script += "umask 077\nprintf %s " + rcMinted + " > \"$5\"\nprintf '%s' '" + perms + "' > \"$5.perms\"\necho \"$5\"\n"
	}
	if err := os.WriteFile(filepath.Join(bin, "desktoken"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	prev := runReadCredentialMint
	runReadCredentialMint = execRunCredentialMint
	t.Cleanup(func() { runReadCredentialMint = prev })
	return log
}

// rcCalls records the method and path of every request, over slTransport.
type rcCalls struct {
	next http.RoundTripper
	mu   sync.Mutex
	seen []string
}

func (c *rcCalls) RoundTrip(r *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.seen = append(c.seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
	c.mu.Unlock()
	return c.next.RoundTrip(r)
}

func rcRecord(t *testing.T) *rcCalls {
	t.Helper()
	c := &rcCalls{next: http.DefaultTransport}
	http.DefaultTransport = c
	t.Cleanup(func() { http.DefaultTransport = c.next })
	return c
}

// rcAdmit makes the verifyrun of f admitted, as TestStoredLoginVerifyrunState
// does: an attestation file in the home and a deskdispatch that issues a
// version-1 receipt for example-org/one.
func rcAdmit(t *testing.T, f transitionFixture) {
	t.Helper()
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

// rcRun runs the acceptance-forge verifyrun of TestStoredLoginVerifyrunState
// with a decoy GH_TOKEN and a stored login present.
func rcRun(t *testing.T, admitted bool, mode string) (verifyrunResult, string, *slTransport, *rcCalls, string) {
	t.Helper()
	f := transitionFixtureAt(t)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	transitionClient = nil
	rt := slForge(t, f.forge)
	calls := rcRecord(t)
	slOutside(t)
	t.Setenv("GH_TOKEN", rcDecoy)
	log := ""
	if mode != "" {
		log = rcMinter(t, mode)
	}
	if admitted {
		rcAdmit(t, f)
	}
	resetReadMemo()
	res, out := captureVerifyrunOut(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge", "--rows", "2,3", "--dry-run"})
	return res, out, rt, calls, log
}

// regression: #6746
// TestRunCredMintedRead: (a) an admitted run reads the approval with the
// credential it minted for the verifier, and (c) the GH_TOKEN in its
// environment is never sent. The credential is revoked and its directory
// removed when the run ends, and its value is never printed. The unadmitted
// control is unchanged: it mints nothing and reads with GH_TOKEN as before.
func TestRunCredMintedRead(t *testing.T) {
	t.Run("admitted", func(t *testing.T) {
		res, out, rt, calls, log := rcRun(t, true, "ok")
		if res.code != verifyrunExitPass {
			t.Fatalf("exit %d, want pass through the minted credential:\n%s", res.code, out)
		}
		minted := false
		for _, a := range rt.auths {
			if a == "Bearer "+rcDecoy || a == "Bearer "+slStoredToken {
				t.Errorf("an ambient credential reached the forge: %q", a)
			}
			minted = minted || a == "Bearer "+rcMinted
		}
		if !minted {
			t.Errorf("the minted credential never reached the forge: %q", rt.auths)
		}
		args, err := os.ReadFile(log)
		if err != nil {
			t.Fatalf("the minter never ran: %v", err)
		}
		fields := strings.Fields(string(args))
		if len(fields) != 5 || fields[0] != "verifier" || fields[1] != "--repo" || fields[2] != "example-org/one" || fields[3] != "--read-only-out" || !filepath.IsAbs(fields[4]) {
			t.Fatalf("minter args = %q, want verifier --repo example-org/one --read-only-out <abs>", args)
		}
		if strings.Count(string(args), "\n") != 1 {
			t.Errorf("minted %d times, want once per run", strings.Count(string(args), "\n"))
		}
		if env, _ := os.ReadFile(log + ".env"); strings.Contains(string(env), rcDecoy) {
			t.Error("the minter was handed the ambient GH_TOKEN")
		}
		if _, err := os.Stat(filepath.Dir(fields[4])); !os.IsNotExist(err) {
			t.Errorf("the run's credential directory survived the run: %v", err)
		}
		revoked := false
		for _, s := range calls.seen {
			revoked = revoked || s == "DELETE /installation/token Bearer "+rcMinted
		}
		if !revoked {
			t.Errorf("the minted credential was not revoked at run end: %q", calls.seen)
		}
		if strings.Contains(out, rcMinted) {
			t.Error("the credential value reached the run's output")
		}
	})
	t.Run("control", func(t *testing.T) {
		res, out, rt, _, log := rcRun(t, false, "ok")
		if res.code != verifyrunExitPass {
			t.Fatalf("control: exit %d, want pass:\n%s", res.code, out)
		}
		if _, err := os.Stat(log); !os.IsNotExist(err) {
			t.Error("control: an unadmitted run minted a credential")
		}
		if len(rt.auths) == 0 || rt.auths[0] != "Bearer "+rcDecoy {
			t.Errorf("control: forge saw %q, want the unchanged GH_TOKEN read", rt.auths)
		}
	})
}

// regression: #6746
// TestRunCredUnavailable: (b) every way the minted credential can be missing
// is a could-not-check, never a pass, and no ambient credential is sent
// in its place.
func TestRunCredUnavailable(t *testing.T) {
	for _, mode := range []string{"fail", "write", "elsewhere", ""} {
		t.Run(map[string]string{"": "stubbed"}[mode]+mode, func(t *testing.T) {
			res, out, rt, calls, _ := rcRun(t, true, mode)
			if res.code == verifyrunExitPass || !strings.Contains(out, "could-not-check") || !strings.Contains(out, "inside an admitted verifyrun") {
				t.Errorf("exit %d, want a could-not-check refusal:\n%s", res.code, out)
			}
			if len(rt.auths) > 0 {
				t.Errorf("%d forge request(s) made without the minted credential: %q", len(rt.auths), calls.seen)
			}
			if strings.Contains(out, rcMinted) {
				t.Error("the credential value reached the run's output")
			}
		})
	}
}

// rcCredFile writes a credential file and its grant into a private dir.
func rcCredFile(t *testing.T, perms string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "read-credential")
	if err := os.WriteFile(p, []byte(rcMinted), 0o600); err != nil {
		t.Fatal(err)
	}
	if perms != "" {
		if err := os.WriteFile(p+".perms", []byte(perms), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// regression: #6746
// TestRunCredRowScoped: only a row that reads the forge is handed the
// credential's path. Other rows, and every row of an unadmitted run, see no
// new variable. No row ever sees the value. A forge-reading row whose
// credential cannot be had is could-not-run, never run.
func TestRunCredRowScoped(t *testing.T) {
	plan := resolveShellPlan()
	if !plan.ok {
		t.Skip("no runnable shell: " + plan.reason)
	}
	path := rcCredFile(t, rcRead)
	prev := runReadCredentialMint
	runReadCredentialMint = func(string, string) (string, error) { return "", nil }
	t.Cleanup(func() { runReadCredentialMint = prev })
	cred := &runReadCredential{repo: "example-org/one"}
	cred.once.Do(func() { cred.path, cred.secret = path, rcMinted })
	show := `printf 'cred=%s\n' "${` + runCredentialEnv + `:-none}"; env`
	run := func(p shellPlan, command string) runResult {
		return runVerifyCommandWith(t.TempDir(), command, 30*time.Second, nil, p, rowShellSh)
	}
	admitted := plan
	admitted.env = admittedRowEnv(os.Environ())
	admitted.readCred = cred
	for name, c := range map[string]struct {
		p       shellPlan
		command string
		want    string
	}{
		"forge row":            {admitted, show + " # --acceptance-forge", "cred=" + path},
		"other row":            {admitted, show, "cred=none"},
		"unadmitted forge row": {plan, show + " # --acceptance-forge", "cred=none"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(runCredentialEnv, "/inherited/elsewhere")
			c.p.env = admittedRowEnv(os.Environ())
			if c.p.readCred == nil {
				c.p.env = nil
				t.Setenv(runCredentialEnv, "")
				_ = os.Unsetenv(runCredentialEnv)
			}
			got := run(c.p, c.command)
			if got.couldNotRun || got.exit != 0 {
				t.Fatalf("row did not run: %+v", got)
			}
			if !strings.Contains(string(got.output), c.want+"\n") {
				t.Errorf("row saw %q, want %q", firstLine(string(got.output)), c.want)
			}
			if strings.Contains(string(got.output), rcMinted) {
				t.Error("the credential value reached a row's environment")
			}
		})
	}
	t.Run("unavailable", func(t *testing.T) {
		broken := &runReadCredential{why: "the attestation names no owner/name repository to narrow it to"}
		p := admitted
		p.readCred = broken
		got := run(p, "true # --acceptance-forge")
		if !got.couldNotRun || !strings.Contains(got.reason, "could-not-check") {
			t.Errorf("a forge row without its credential = %+v, want could-not-run with could-not-check", got)
		}
		if got := run(p, "true"); got.couldNotRun {
			t.Errorf("a row that reads no forge was refused for the credential: %+v", got)
		}
	})
}

// regression: #6746
// TestRunCredRowRead: a statusgen started inside an admitted Verify row reads
// the forge (here the ruling-record read, through the same rulingForgeClient
// as the acceptance read) with the credential file the run handed it, never
// GH_TOKEN. A row handed none, or a file that fails its checks, is a
// could-not-check with no request.
func TestRunCredRowRead(t *testing.T) {
	for name, perms := range map[string]string{"read": rcRead, "write": `{"pull_requests":"write"}`, "no grant": "", "none handed": "-"} {
		t.Run(name, func(t *testing.T) {
			root := rrSetup(t, rrBoundForge())
			rulingRecordClientFn = rulingForgeClient
			rt := slForge(t, rrBoundForge())
			path := ""
			if perms != "-" {
				path = rcCredFile(t, perms)
			}
			slAdmittedRow(t)
			t.Setenv("GH_TOKEN", rcDecoy)
			if path != "" {
				t.Setenv(runCredentialEnv, path)
			}
			got := runCorroborateRecord(root, rlFile, rrBriefRel)
			for _, a := range rt.auths {
				if a != "Bearer "+rcMinted {
					t.Errorf("forge saw %q, want only the run credential", a)
				}
			}
			if name == "read" && (got != 0 || len(rt.auths) == 0) {
				t.Errorf("exit %d with %d request(s), want 0 through the run credential", got, len(rt.auths))
			}
			if name != "read" && (got != 2 || len(rt.auths) > 0) {
				t.Errorf("exit %d with %d request(s), want 2 (could-not-check) and none", got, len(rt.auths))
			}
		})
	}
}

// TestRunCredFileChecks: what readRunCredentialFile refuses.
func TestRunCredFileChecks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	good := rcCredFile(t, rcRead)
	if got, err := readRunCredentialFile(good); err != nil || got != rcMinted {
		t.Fatalf("valid file: %v", err)
	}
	open := rcCredFile(t, rcRead)
	if err := os.Chmod(open, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	empty := rcCredFile(t, rcRead)
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{
		"relative":    "read-credential",
		"missing":     filepath.Join(t.TempDir(), "absent"),
		"open mode":   open,
		"symlink":     link,
		"empty":       empty,
		"no grant":    rcCredFile(t, ""),
		"write grant": rcCredFile(t, `{"issues":"read","contents":"write"}`),
		"empty grant": rcCredFile(t, `{}`),
		"admin grant": rcCredFile(t, `{"administration":"admin"}`),
		"garbled":     rcCredFile(t, `read`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readRunCredentialFile(p); err == nil {
				t.Error("accepted")
			}
		})
	}
}

// TestRunCredMintStubbed pins TestMain's guard: no test reaches a real
// minter unless it asks for the exec path with a fake on PATH.
// rcAdmittedWith puts this process in an admitted verifyrun whose run
// credential is already minted as rcMinted (or absent when minted is false),
// with a decoy GH_TOKEN in the environment.
func rcAdmittedWith(t *testing.T, minted bool) {
	t.Helper()
	t.Setenv("GH_TOKEN", rcDecoy)
	prevState, prevCred := verifyrunAdmission.Load(), currentRunCredential.Load()
	t.Cleanup(func() {
		verifyrunAdmission.Store(prevState)
		currentRunCredential.Store(prevCred)
	})
	verifyrunAdmission.Store(verifyrunAdmitted)
	currentRunCredential.Store(nil)
	if minted {
		c := &runReadCredential{repo: "example-org/one"}
		c.once.Do(func() { c.path, c.secret = "/example/read-credential", rcMinted })
		currentRunCredential.Store(c)
	}
}

// regression: #6746
// TestRunCredPlantedRest: the defect class is a forge reader in an
// admitted context built with an inherited credential. A planted new
// REST site, passing the decoy straight to newGHClient as an unreviewed
// caller would, still carries only the run credential, or refuses.
func TestRunCredPlantedRest(t *testing.T) {
	t.Run("minted", func(t *testing.T) {
		rcAdmittedWith(t, true)
		if c := newGHClient(os.Getenv("GH_TOKEN")); c.token != rcMinted {
			t.Fatalf("planted REST site carries %q, want the run credential", c.token)
		}
	})
	t.Run("none", func(t *testing.T) {
		rcAdmittedWith(t, false)
		c := newGHClient(os.Getenv("GH_TOKEN"))
		if _, refused := c.doer.(runCredentialDoer); !refused || c.token != "" {
			t.Fatalf("planted REST site was not refused (token %q)", c.token)
		}
	})
	t.Run("outside", func(t *testing.T) {
		t.Setenv("GH_TOKEN", rcDecoy)
		prev := verifyrunAdmission.Load()
		t.Cleanup(func() { verifyrunAdmission.Store(prev) })
		verifyrunAdmission.Store(verifyrunOutside)
		if c := newGHClient(rcDecoy); c.token != rcDecoy {
			t.Fatalf("outside an admitted context the caller's credential changed to %q", c.token)
		}
	})
}

// regression: #6746
// TestRunCredPlantedCli: the same class on the forge CLI path. A planted
// gh launch inside an admitted run sees only the run credential, and with
// none it never starts.
func TestRunCredPlantedCli(t *testing.T) {
	t.Run("minted", func(t *testing.T) {
		rcAdmittedWith(t, true)
		cmd := ghGuard(exec.Command("gh", "api", "example"))
		if cmd.Err != nil {
			t.Fatalf("refused with a run credential: %v", cmd.Err)
		}
		var got []string
		for _, kv := range cmd.Env {
			if strings.HasPrefix(kv, "GH_TOKEN=") || strings.HasPrefix(kv, "GITHUB_TOKEN=") {
				got = append(got, kv)
			}
		}
		if len(got) != 1 || got[0] != "GH_TOKEN="+rcMinted {
			t.Fatalf("planted CLI launch sees %q, want only the run credential", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		rcAdmittedWith(t, false)
		cmd := ghGuard(exec.Command("gh", "api", "example"))
		if cmd.Err == nil || !strings.Contains(cmd.Err.Error(), "could-not-check") {
			t.Fatalf("planted CLI launch was not refused: %v", cmd.Err)
		}
	})
}

func TestRunCredMintStubbed(t *testing.T) {
	out := filepath.Join(t.TempDir(), "read-credential")
	if _, err := runReadCredentialMint("example-org/one", out); err == nil {
		t.Fatal("the test binary's minter is not the refusing stub")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatal("the stub wrote a credential")
	}
}
