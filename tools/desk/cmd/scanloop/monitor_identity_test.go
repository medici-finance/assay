package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// monitor_identity_test.go — the poller's explicit read identity (issue 1503).
//
// Under a replaced HOME (a desk cell's) the poller's keyring fallback resolves to no usable account
// and 401s every repo, so `run` reports the drain blind everywhere while `plan` shows it armed.
// RunMonitor now hands the poller the running role's already-minted installation token FILE per
// owner; these tests pin the hand-off shape, the resolution, and the end-to-end read.

// stubIdentity swaps the role and mint seams for the duration of a test.
func stubIdentity(t *testing.T, role func(string) (string, string, error), mint func(string, string) (string, string, error)) {
	t.Helper()
	oldRole, oldMint := sessionRoleFn, mintTokenFn
	sessionRoleFn, mintTokenFn = role, mint
	t.Cleanup(func() { sessionRoleFn, mintTokenFn = oldRole, oldMint })
}

// TestRunMonitor_HandsTheTokenFileByPath — the child gets `--token-file OWNER=PATH` for the owner
// that has one, then the whole scope in ONE invocation. The token VALUE is in neither the argv nor
// the environment the child is started with: only the path travels.
func TestRunMonitor_HandsTheTokenFileByPath(t *testing.T) {
	const tokenValue = "x-example-installation-token"
	t.Setenv("GH_TOKEN", "")
	var sawEnv, sawArgs []string
	_, err := RunMonitor("/x/inbound-monitor.sh", t.TempDir(),
		[]string{"example-org/tracker", "example-other/agents", "example-org/examples"},
		map[string]string{"example-org": "/cache/example-token", "example-unused": "/cache/unused-token"},
		func(_ string, env []string, args ...string) (string, int, error) {
			sawEnv, sawArgs = env, args
			return seededPoll, 0, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--token-file", "example-org=/cache/example-token",
		"example-org/tracker", "example-other/agents", "example-org/examples",
	}
	if !reflect.DeepEqual(sawArgs, want) {
		t.Fatalf("argv = %q\nwant   %q (a token file only for an owner IN scope, then the whole scope)", sawArgs, want)
	}
	for _, e := range append(append([]string{}, sawEnv...), sawArgs...) {
		if strings.Contains(e, tokenValue) {
			t.Fatalf("the token value reached the child: %q", e)
		}
		if strings.HasPrefix(e, "GH_TOKEN=") && e != "GH_TOKEN=" {
			t.Fatalf("RunMonitor set a GH_TOKEN for the child (%q) — the hand-off is by FILE PATH only", e)
		}
	}
}

// TestRunMonitor_NoTokenFilesKeepsTodaysArgv — absent an identity, the argv is exactly the scope,
// byte-for-byte what the poller was always called with.
func TestRunMonitor_NoTokenFilesKeepsTodaysArgv(t *testing.T) {
	scope := []string{"example-org/tracker", "example-other/agents"}
	for _, tf := range []map[string]string{nil, {}} {
		var sawArgs []string
		if _, err := RunMonitor("/x/inbound-monitor.sh", t.TempDir(), scope, tf,
			func(_ string, _ []string, args ...string) (string, int, error) {
				sawArgs = args
				return seededPoll, 0, nil
			}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sawArgs, scope) {
			t.Fatalf("tokenFiles=%v: argv = %q, want exactly the scope %q", tf, sawArgs, scope)
		}
	}
}

// TestResolveMonitorIdentity_OneMintPerOwnerAndNamedFallbacks — the role is the session's own, the
// mint is looked up once per OWNER (an installation token is per owner), only the path is kept, and
// an owner whose mint fails is a NAMED keyring fallback rather than a dropped one.
func TestResolveMonitorIdentity_OneMintPerOwnerAndNamedFallbacks(t *testing.T) {
	var mints []string
	stubIdentity(t,
		func(verb string) (string, string, error) {
			if verb != "scanloop" {
				t.Fatalf("role resolved for verb %q, want scanloop", verb)
			}
			return "intake-loop", "intake-desk", nil
		},
		func(role, repo string) (string, string, error) {
			mints = append(mints, role+" "+repo)
			switch {
			case strings.HasPrefix(repo, "example-org/"):
				return "x-example-installation-token", "/cache/example-org-token", nil
			case strings.HasPrefix(repo, "example-empty/"):
				return "x-example-installation-token", "", nil
			default:
				return "", "", errors.New("no installation on this owner")
			}
		})

	id := ResolveMonitorIdentity([]string{
		"example-org/tracker", "example-org/examples", "example-other/agents", "example-empty/r",
	})
	if id.Role != "intake-loop" {
		t.Fatalf("role = %q", id.Role)
	}
	if want := []string{"intake-loop example-org/tracker", "intake-loop example-other/agents", "intake-loop example-empty/r"}; !reflect.DeepEqual(mints, want) {
		t.Fatalf("mints = %q, want one per owner %q", mints, want)
	}
	if want := map[string]string{"example-org": "/cache/example-org-token"}; !reflect.DeepEqual(id.TokenFiles, want) {
		t.Fatalf("token files = %v, want %v", id.TokenFiles, want)
	}
	if !strings.Contains(id.Fallbacks["example-other"], "no installation") {
		t.Fatalf("the failed mint is not a named fallback: %v", id.Fallbacks)
	}
	if !strings.Contains(id.Fallbacks["example-empty"], "named no token file") {
		t.Fatalf("a mint with no file path is not a named fallback: %v", id.Fallbacks)
	}
	for o, why := range id.Fallbacks {
		if strings.Contains(why, "x-example-installation-token") {
			t.Fatalf("fallback reason for %s carries the token value", o)
		}
	}
}

// TestResolveMonitorIdentity_NoRoleIsAllKeyringNeverAGuess — with no resolvable role nothing is
// minted (a guessed role is the wrong-identity failure SessionTokenRole exists to prevent); every
// owner falls back to the keyring path, each with the reason.
func TestResolveMonitorIdentity_NoRoleIsAllKeyringNeverAGuess(t *testing.T) {
	stubIdentity(t,
		func(string) (string, string, error) { return "", "", errors.New("DESK_LOOP is not set") },
		func(string, string) (string, string, error) {
			t.Fatal("a token was minted with no resolvable role")
			return "", "", nil
		})
	id := ResolveMonitorIdentity([]string{"example-org/tracker", "example-other/agents"})
	if len(id.TokenFiles) != 0 {
		t.Fatalf("token files = %v, want none", id.TokenFiles)
	}
	for _, o := range []string{"example-org", "example-other"} {
		if !strings.Contains(id.Fallbacks[o], "DESK_LOOP is not set") {
			t.Fatalf("owner %s fallback = %q", o, id.Fallbacks[o])
		}
	}
}

// TestRunMonitor_KeyringlessHomeReadsCleanWithTokenFile — the acceptance row end to end: the REAL
// poller from the plugin tree, run by the real exec path, under a HOME with no gh config at all and
// a `gh` that 401s without a token exactly as a keyring-less gh does. With the role's token file it
// arms clean; without one the same read is DEGRADED, as before.
func TestRunMonitor_KeyringlessHomeReadsCleanWithTokenFile(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not on PATH — the poller refuses to run without it")
	}
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("/bin/bash absent — execMonitor runs the poller through it")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", monitorScriptRelPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("poller not found at %s: %v", script, err)
	}

	const tokenValue = "x-example-installation-token"
	bin := t.TempDir()
	stub := "#!/bin/sh\n" +
		"if [ -z \"${GH_TOKEN:-}\" ]; then echo 'HTTP 401: Requires authentication (https://api.github.com/graphql)' >&2; exit 1; fi\n" +
		"if [ \"$GH_TOKEN\" != '" + tokenValue + "' ]; then echo 'HTTP 401: Bad credentials' >&2; exit 1; fi\n" +
		"echo '[{\"number\":7,\"updatedAt\":\"2026-01-01T00:00:00Z\"},{\"number\":8,\"updatedAt\":\"2026-01-01T00:00:00Z\"}]'\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	tokFile := filepath.Join(t.TempDir(), "example-org-token")
	if err := os.WriteFile(tokFile, []byte(tokenValue+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir()) // no gh config, no keyring account
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "x-bogus-inherited-token") // the poller must never read as this
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	scope := []string{"example-org/tracker"}

	// With the role's token file: armed, nothing degraded.
	rep, err := RunMonitor(script, t.TempDir(), scope, map[string]string{"example-org": tokFile}, nil)
	if err != nil {
		t.Fatalf("keyring-less HOME + token file: %v", err)
	}
	if !rep.Armed || rep.ArmedTotal != 2 || rep.Blind() {
		t.Fatalf("keyring-less HOME + token file did not read clean: %+v", rep)
	}

	// Without one: the same read goes DEGRADED on the 401, exactly as before the hand-off.
	rep, err = RunMonitor(script, t.TempDir(), scope, nil, nil)
	if err != nil {
		t.Fatalf("keyring-less HOME, no token file: a degraded read must not be fatal: %v", err)
	}
	if len(rep.Degraded) != 1 || !strings.Contains(rep.Degraded[0], "401") || rep.Armed {
		t.Fatalf("keyring-less HOME without a token file: want one DEGRADED 401 line, got %+v", rep)
	}
}
