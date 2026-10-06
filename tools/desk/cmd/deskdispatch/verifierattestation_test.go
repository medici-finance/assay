package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The helper is a separate checker process, like statusgen's installed-tool
// boundary. Only forge transport/custody is replaced; the shared reader is real.
func TestVerifierCheckHelper(t *testing.T) {
	if os.Getenv("VERIFIER_CHECK_FIXTURE") != "1" {
		return
	}
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (string, string, error) {
		return ghStampToken, os.Getenv("VERIFIER_CHECK_API"), nil
	})
	if err := cmdVerifierAttestation("--check-verifier", []string{"--root", os.Getenv("VERIFIER_CHECK_ROOT"), "--brief", filepath.Join(os.Getenv("VERIFIER_CHECK_ROOT"), "spec.md")}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	os.Exit(0)
}

func verifierGHFixture(t *testing.T, gh *ghStampServer) {
	t.Helper()
	var title, body, state string
	gh.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gh.mu.Lock()
		defer gh.mu.Unlock()
		enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		issue := func() map[string]any {
			labels := []map[string]any{}
			for _, l := range gh.labels {
				labels = append(labels, map[string]any{"name": l})
			}
			return map[string]any{"number": 77, "title": title, "body": body, "state": state, "labels": labels, "html_url": "https://example.invalid/attestation/77", "user": map[string]any{"login": "assay-desk-app[bot]", "id": 300000001}}
		}
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/issues"):
			var in struct{ Title, Body string }
			json.NewDecoder(r.Body).Decode(&in)
			title, body, state = in.Title, in.Body, "open"
			w.WriteHeader(201)
			enc(issue())
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/77"):
			if gh.failAfter && gh.writes > 0 {
				w.WriteHeader(500)
				return
			}
			enc(issue())
		case r.Method == "PATCH" && strings.HasSuffix(r.URL.Path, "/issues/77"):
			state = "closed"
			enc(issue())
		case strings.HasSuffix(r.URL.Path, "/graphql"):
			enc(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{"lastEditedAt": nil, "comments": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}}}}}})
		default:
			gh.handle(w, r)
		}
	})
}

func TestVerifierDispatchAdmissionEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX two-row executor fixture; shared admission reader and native adapter are portable tests")
	}
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol"} {
		t.Run(model, func(t *testing.T) {
			cache, err := exec.Command("go", "env", "GOMODCACHE").Output()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GOMODCACHE", strings.TrimSpace(string(cache)))
			s := &stub{}
			deskHome, root := s.install(t)
			plantScripts(t, root)
			// Forge selection belongs to the fixture, not the enclosing checkout's origin.
			pinFixtureForge(t, deskHome, allowedRepo)
			t.Setenv("DESK_LOOP", "verify-desk") // the real standing caller, same verifier App as its worker
			t.Setenv("DESK_SESSION", "fixture-shared-claim-owner")
			attestVerifierDispatchFn = attestVerifierDispatch
			home := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				c := exec.Command("git", append([]string{"-C", home}, args...)...)
				if out, err := c.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %s %v", args, out, err)
				}
			}
			git("init")
			git("remote", "add", "origin", "https://github.com/"+allowedRepo+".git")
			git("config", "user.name", "fixture")
			git("config", "user.email", "fixture@example.invalid")
			outputHome := t.TempDir()
			t.Setenv("VERIFY_OUTPUT", outputHome)
			body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + string(rune(96)) + "touch \"$VERIFY_OUTPUT/row-one\"" + string(rune(96)) + " | exit 0 |\n| 2 | " + string(rune(96)) + "touch \"$VERIFY_OUTPUT/row-two\"" + string(rune(96)) + " | exit 0 |\n\n## Evidence\n\nPending.\n"
			os.WriteFile(filepath.Join(home, "spec.md"), []byte(body), 0600)
			git("add", "spec.md")
			git("commit", "-m", "fixture")
			git("update-ref", "refs/remotes/origin/main", "HEAD")
			git("checkout", "--detach")
			s.replies = happyReplies(home)
			gh := installGHStamp(t)
			verifierGHFixture(t, gh)
			var claimRoles []string
			mintTokenFn = func(role, repo string) (string, string, error) {
				claimRoles = append(claimRoles, role)
				return stubMintedToken, stubMintedTokenPath(t, t.TempDir()), nil
			}
			prompt := filepath.Join(t.TempDir(), "prompt.md")
			if rc := run([]string{"example/01", "--root", root, "--repo", allowedRepo, "--kit", "verifier", "--model", model, "--tier", "strong", "--brief", "spec.md", "--prompt-file", prompt}); rc != 0 {
				t.Fatalf("real verifier-desk dispatch rc=%d", rc)
			}
			if len(claimRoles) != 1 || claimRoles[0] != deskkit.DispatcherRole {
				t.Fatalf("claim custody %v", claimRoles)
			}
			for _, mint := range gh.minted {
				if mint.role != deskkit.DispatcherRole {
					t.Fatalf("stamp did not use existing desk custody: %v", gh.minted)
				}
			}
			receipt, err := deskkit.CheckVerifierAttestation(home, "spec.md")
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Binding.Model != model || receipt.Binding.Tier != "strong" {
				t.Fatalf("wrong actual selection %+v", receipt)
			}
			p, _ := os.ReadFile(prompt)
			if strings.Contains(string(p), "stamp PENDING") {
				t.Fatal("pending verifier emitted")
			}
			// Run the actual separate-module Verify executor, with its checker subprocess
			// pointed at the same offline forge. Neither Verify command can run first.
			bin := filepath.Join(t.TempDir(), "statusgen")
			build := exec.Command("go", "build", "-o", bin, ".")
			build.Dir = "../../../../statusgen"
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build statusgen: %s %v", out, err)
			}
			helperDir := t.TempDir()
			helper := filepath.Join(helperDir, "deskdispatch")
			script := "#!/bin/sh\nexec '" + strings.ReplaceAll(os.Args[0], "'", "'\\''") + "' -test.run=TestVerifierCheckHelper\n"
			if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("VERIFIER_CHECK_FIXTURE", "1")
			t.Setenv("VERIFIER_CHECK_API", gh.srv.URL)
			t.Setenv("VERIFIER_CHECK_ROOT", home)
			execute := func() ([]byte, error) {
				c := exec.Command(bin, "verifyrun", "--root", home, "--brief", filepath.Join(home, "spec.md"))
				c.Dir = home
				return c.CombinedOutput()
			}
			gh.mu.Lock()
			saved := gh.labels
			gh.labels = nil
			gh.mu.Unlock()
			if out, err := execute(); err == nil {
				t.Fatalf("missing stamp ran Verify: %s", out)
			}
			for _, name := range []string{"row-one", "row-two"} {
				if _, err := os.Stat(filepath.Join(outputHome, name)); err == nil {
					t.Fatalf("row ran without admission: %s", name)
				}
			}
			gh.mu.Lock()
			gh.labels = saved
			gh.mu.Unlock()
			if out, err := execute(); err != nil {
				t.Fatalf("admitted Verify failed: %s %v", out, err)
			}
			for _, name := range []string{"row-one", "row-two"} {
				if _, err := os.Stat(filepath.Join(outputHome, name)); err != nil {
					t.Fatalf("admitted row did not run: %s", name)
				}
			}
			after, _ := os.ReadFile(filepath.Join(home, "spec.md"))
			if !strings.Contains(string(after), receipt.EvidenceBinding()) {
				t.Fatalf("Evidence lost exact run/source/model binding: %s", after)
			}
			if _, err := deskkit.CheckVerifierEvidence(home, allowedRepo, "spec.md"); err != nil {
				t.Fatalf("normal Evidence preparation deadlocked: %v", err)
			}
			if err := receipt.CheckEvidenceContent("spec.md", after, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifierDispatchFailedStampEmitsNoPromptAndReleasesClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	attestVerifierDispatchFn = func(dispatchOpts, string, string) (string, error) {
		return "", deskkit.Unverifiable("fixture dropped stamp", nil)
	}
	s.replies = happyReplies(t.TempDir())
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	if rc := run([]string{"example/01", "--root", root, "--repo", allowedRepo, "--kit", "verifier", "--model", "gpt-6-astra", "--brief", "spec.md", "--prompt-file", prompt}); rc == 0 {
		t.Fatal("failed stamp reported success")
	}
	if _, err := os.Stat(prompt); err == nil {
		t.Fatal("failed stamp emitted verifier prompt")
	}
	released := false
	for _, call := range s.calls {
		if strings.Contains(strings.Join(call, " "), "dispatch-claim.sh release") {
			released = true
		}
	}
	if !released {
		t.Fatal("failed pre-work attestation leaked claim")
	}
}

func TestWorkerDeskPostOpenCoordinatorHandoff(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(t.TempDir())
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("DESK_SESSION", "same-inherited-claim-owner")
	gh := installGHStamp(t)
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	if rc := run([]string{"example/01", "--root", root, "--repo", allowedRepo, "--model", "gpt-6.1-sol", "--tier", "strong", "--prompt-file", prompt}); rc != 0 {
		t.Fatal(rc)
	}
	p, _ := os.ReadFile(prompt)
	if !strings.Contains(string(p), "coordinator") {
		t.Fatal("worker-desk was not given a reachable coordinator handoff")
	}
	// Both original worker-desk and its child share this role/session: neither may
	// turn that inherited claim identity into stamp authority.
	for _, caller := range []string{"worker dispatcher", "dispatched child"} {
		if rc := run(onlyArgs("gpt-6.1-sol")); rc != 5 {
			t.Fatalf("%s self-stamped: %d", caller, rc)
		}
	}
	if gh.writes != 0 {
		t.Fatal("worker role wrote a stamp")
	}
	// A separate coordinator session receives the exact real dispatch selection.
	t.Setenv("DESK_LOOP", "the-desk")
	t.Setenv("DESK_SESSION", "fixture-coordinator")
	if rc := run(onlyArgs("gpt-6.1-sol")); rc != 0 {
		t.Fatal(rc)
	}
}

// pinFixtureForge names the fixture repository's forge in the fixture roster,
// so admission never falls back to the forge of the enclosing checkout's origin.
func pinFixtureForge(t *testing.T, home, repo string) {
	t.Helper()
	path := filepath.Join(home, ".config", "assay", "roster.env")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, []byte("\nASSAY_REPO_FORGES="+repo+"=github\n")...)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
