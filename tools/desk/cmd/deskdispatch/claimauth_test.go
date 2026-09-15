package main

// claimauth_test.go — issue 1151. The claim-acquire step ran the claim tool as a child with
// NO credential handed over, so both tools read only the ambient one: bare `gh api` in the
// legacy script (nothing in a sandboxed desk window → 401; a human login with no write on the
// target → 404), GH_TOKEN/--token-file in the Go binary. Every other write verb minted its own
// role token and succeeded; the claim child was the one write path left on ambient auth, and a
// review window could not dispatch a single reviewer for an evening.
//
// These tests drive a REAL fake claim child — a shell script that records its argv and the
// credential it can see — through the exec seam, so "the token reached the child" is asserted
// on what the child observed, not on a variable nothing read. The fake lives on a temp PATH as
// deskclaim-ref (the Go-binary shape) and in the temp root as tools/dispatch-claim.sh (the
// script shape). Nothing here reaches a forge.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimChildScript is the fake claim tool. For every invocation it appends one record to the
// file named by CLAIM_CHILD_RECORD: the argv it received, the GH_TOKEN it could see, and — when
// handed --token-file — the file's content. It exits 0 for every verb.
const claimChildScript = `#!/bin/sh
tf=""
set -- "$@"
argv="$*"
while [ $# -gt 0 ]; do
  if [ "$1" = "--token-file" ]; then tf="$2"; fi
  shift
done
{
  printf 'argv=%s\n' "$argv"
  printf 'GH_TOKEN=%s\n' "${GH_TOKEN-}"
  if [ -n "$tf" ]; then
    printf 'token-file=%s\n' "$tf"
    printf 'token-file-content=%s\n' "$(cat "$tf")"
  fi
  printf -- '--\n'
} >> "$CLAIM_CHILD_RECORD"
exit 0
`

// claimChildRecord is what the fake child observed on ONE invocation.
type claimChildRecord struct {
	argv             string
	ghToken          string
	tokenFile        string
	tokenFileContent string
}

// installClaimChild wraps the harness's exec seam so the claim tool (and only the claim tool)
// really runs: a call whose argv[0] is the Go binary's bare name or the planted script path
// starts the real fake child; everything else (git, deskwt, deskroster, gh) keeps the harness's
// canned reply. It returns the record path the child appends to.
func installClaimChild(t *testing.T, s *stub) (record string) {
	t.Helper()
	record = filepath.Join(t.TempDir(), "claim-child.record")
	t.Setenv("CLAIM_CHILD_RECORD", record)
	prev := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == goClaimBinary || strings.HasSuffix(name, filepath.FromSlash(claimScriptRel)) {
			s.calls = append(s.calls, append([]string{name}, args...))
			return exec.Command(name, args...)
		}
		return prev(name, args...)
	}
	t.Cleanup(func() { execCommand = prev })
	return record
}

// plantGoClaimChild puts the fake child on a temp PATH under the Go binary's bare name and
// restores the REAL PATH lookup, so resolution goes through exec.LookPath exactly as
// production does.
func plantGoClaimChild(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, goClaimBinary), []byte(claimChildScript), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := lookPath
	lookPath = exec.LookPath
	t.Cleanup(func() { lookPath = old })
}

// plantScriptClaimChild plants the fake child as the resolved root's tools/dispatch-claim.sh
// (plus an inert decision script, which the verb only stats).
func plantScriptClaimChild(t *testing.T, root string) {
	t.Helper()
	plantScripts(t, root)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(claimScriptRel)), []byte(claimChildScript), 0o700); err != nil {
		t.Fatal(err)
	}
}

// acquireRecord reads the record of the child's `acquire` invocation; it fails the test when
// the child never ran acquire, so no assertion below can pass vacuously.
func acquireRecord(t *testing.T, record string) claimChildRecord {
	t.Helper()
	b, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the fake claim child left no record — it never ran: %v", err)
	}
	for _, block := range strings.Split(string(b), "--\n") {
		var r claimChildRecord
		for _, ln := range strings.Split(block, "\n") {
			k, v, _ := strings.Cut(ln, "=")
			switch k {
			case "argv":
				r.argv = v
			case "GH_TOKEN":
				r.ghToken = v
			case "token-file":
				r.tokenFile = v
			case "token-file-content":
				r.tokenFileContent = v
			}
		}
		if strings.HasPrefix(r.argv, "acquire ") {
			return r
		}
	}
	t.Fatalf("the fake claim child recorded no acquire invocation:\n%s", b)
	return claimChildRecord{}
}

// FRESH DISPATCH, GO BINARY: no GH_TOKEN exported, no token minted before step 1. The claim
// step mints the dispatching role's token ITSELF (the stamp step's mint is four steps too
// late) and hands the binary `--token-file <path>` naming a 0600 file holding that token —
// never the token value on the command line, never GH_TOKEN injected for the binary.
func TestClaimChildReceivesTheMintedTokenAsTokenFileForTheGoBinary(t *testing.T) {
	s := &stub{}
	home, root := s.install(t) // clears GH_TOKEN, binds the default mint stub
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")
	dispatcherToken = "" // nothing minted before step 1 — the ordering the issue names
	t.Cleanup(func() { dispatcherToken = "" })

	rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	r := acquireRecord(t, record)
	want := stubMintedTokenPath(t, home)
	if r.tokenFile != want {
		t.Fatalf("the Go claim binary was handed --token-file %q, want the minted token file %q "+
			"(argv: %s) — on ambient auth this child would have run as whatever gh is logged in as",
			r.tokenFile, want, r.argv)
	}
	if strings.TrimSpace(r.tokenFileContent) != stubMintedToken {
		t.Errorf("the token file the child read holds %q, want the minted token", r.tokenFileContent)
	}
	fi, err := os.Stat(r.tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("--token-file %s has mode %o, want 0600 (deskclaim-ref refuses anything wider)", r.tokenFile, fi.Mode().Perm())
	}
	if strings.Contains(r.argv, stubMintedToken) {
		t.Errorf("the token VALUE appeared on the child's command line: %s", r.argv)
	}
	if r.ghToken != "" {
		t.Errorf("GH_TOKEN=%q was injected into the Go binary's environment; the binary takes --token-file", r.ghToken)
	}
}

// FRESH DISPATCH, LEGACY SCRIPT (Go binary absent): the script shells bare `gh api`, which
// reads GH_TOKEN — so the minted token is placed in the CHILD's environment, and only there:
// no --token-file (the script has no such flag), nothing on the command line.
func TestClaimChildReceivesTheMintedTokenAsGHTokenForTheLegacyScript(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // install's default lookPath finds no Go binary
	plantScriptClaimChild(t, root)
	record := installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")
	dispatcherToken = ""
	t.Cleanup(func() { dispatcherToken = "" })

	rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	r := acquireRecord(t, record)
	if r.ghToken != stubMintedToken {
		t.Fatalf("the legacy script saw GH_TOKEN=%q, want the minted token — its bare `gh api` calls "+
			"would have run on the ambient login (argv: %s)", r.ghToken, r.argv)
	}
	if r.tokenFile != "" || strings.Contains(r.argv, "--token-file") {
		t.Errorf("--token-file was passed to the legacy script, which has no such flag: %s", r.argv)
	}
	if strings.Contains(r.argv, stubMintedToken) {
		t.Errorf("the token VALUE appeared on the child's command line: %s", r.argv)
	}
	// The dispatcher's own environment is untouched: the token was added to the CHILD's
	// environment only, never exported into this process.
	if os.Getenv("GH_TOKEN") != "" {
		t.Errorf("GH_TOKEN leaked into the dispatcher's own environment: %q", os.Getenv("GH_TOKEN"))
	}
}

// An explicit GH_TOKEN already in the environment WINS: nothing is minted, the child sees the
// operator's export, and the Go binary is NOT handed --token-file (that flag would outrank the
// export inside the binary).
func TestExplicitGHTokenWinsAndNothingIsMinted(t *testing.T) {
	for _, tool := range []string{"go-binary", "legacy-script"} {
		t.Run(tool, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			if tool == "go-binary" {
				plantGoClaimChild(t)
			} else {
				plantScriptClaimChild(t, root)
			}
			record := installClaimChild(t, s)
			s.replies = happyReplies("/private/tmp/worker-home")
			t.Setenv("GH_TOKEN", "example-explicit-export")
			mints := stubMint(t, stubMintedToken, nil)

			rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0", rc)
			}
			r := acquireRecord(t, record)
			if r.ghToken != "example-explicit-export" {
				t.Errorf("the child saw GH_TOKEN=%q, want the explicit export", r.ghToken)
			}
			if r.tokenFile != "" {
				t.Errorf("--token-file %q was passed although GH_TOKEN was exported — inside the binary the "+
					"flag outranks the export, so the operator's choice would have been overridden", r.tokenFile)
			}
			if len(*mints) != 0 {
				t.Errorf("the role token was minted %d time(s) although an explicit GH_TOKEN was exported", len(*mints))
			}
		})
	}
}

// FAIL CLOSED: with no GH_TOKEN exported and the mint refusing, the dispatch exits 6 carrying
// the mint refusal, and NO claim child runs — the tool is never fallen back onto ambient auth,
// and no claim (which could not have been released) is placed.
func TestClaimStepFailsClosedOnMintRefusalWithoutRunningTheClaimTool(t *testing.T) {
	for _, tool := range []string{"go-binary", "legacy-script"} {
		t.Run(tool, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			if tool == "go-binary" {
				plantGoClaimChild(t)
			} else {
				plantScriptClaimChild(t, root)
			}
			record := installClaimChild(t, s)
			s.replies = happyReplies("/private/tmp/worker-home")
			stubMint(t, "", errors.New("example mint refusal: no App key for this role"))

			err := cmdDispatch([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if err == nil {
				t.Fatal("a dispatch whose role token could not be minted returned nil — the claim ran on ambient auth")
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
				t.Fatalf("rc = %d, want %d (unverifiable): %v", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable, err)
			}
			for _, want := range []string{stepClaimAcquire, "example mint refusal", "NO claim was attempted"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not carry %q: %s", want, err.Error())
				}
			}
			if _, statErr := os.Stat(record); statErr == nil {
				b, _ := os.ReadFile(record)
				t.Fatalf("the claim child RAN despite the mint refusal — that is the ambient-auth fallback:\n%s", b)
			}
			if s.ran("acquire") {
				t.Fatalf("a claim tool was invoked despite the mint refusal: %v", s.calls)
			}
		})
	}
}

// The claim-acquire report line NAMES the tool that ran and how it authenticated, without ever
// carrying the token value.
func TestClaimAcquireLineNamesTheToolAndTheCredentialSource(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantGoClaimChild(t)
	installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldErr := os.Stderr
	os.Stderr = w
	rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	w.Close()
	os.Stderr = oldErr
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, rerr := r.Read(buf)
		sb.Write(buf[:n])
		if rerr != nil {
			break
		}
	}
	out := sb.String()
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0:\n%s", rc, out)
	}
	line := ""
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, stepClaimAcquire+" OK:") {
			line = ln
		}
	}
	if line == "" {
		t.Fatalf("no %s OK line was printed:\n%s", stepClaimAcquire, out)
	}
	for _, want := range []string{"via " + goClaimBinary, "--token-file", stubMintedTokenPath(t, home)} {
		if !strings.Contains(line, want) {
			t.Errorf("the claim-acquire line does not carry %q: %s", want, line)
		}
	}
	if strings.Contains(out, stubMintedToken) {
		t.Errorf("the token VALUE was printed:\n%s", out)
	}
}
