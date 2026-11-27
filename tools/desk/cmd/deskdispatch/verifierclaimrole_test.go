package main

// verifierclaimrole_test.go — #7815. A verify-desk window dispatched verifiers with the claim
// written as the desk App, so its own progress and release of that claim needed a cross-role
// desk mint. A session permission layer refused that mint, and verifier dispatch stopped. These
// pin that a verifier dispatch from verify-desk takes, advances and releases its claim under the
// verifier App with no cross-role mint, and that the-desk's claim path is unchanged.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The claim role per loop and kit: only a verifier dispatch from a loop that acts as verifier
// moves off the stamp role. Every other pair, including an unset or unknown loop, keeps the
// role it claimed under before.
// regression: #7815
func TestClaimRoleForKit(t *testing.T) {
	for _, tc := range []struct {
		loop, kit, want string
	}{
		{"verify-desk", "verifier", "verifier"},
		{"verify-desk", "Verifier", "verifier"},
		{"the-desk", "verifier", "desk"},
		{"worker-desk", "verifier", "desk"},
		{"pr-review-desk", "verifier", "desk"},
		{"intake-desk", "verifier", "desk"},
		{"", "verifier", "desk"},
		{"no-such-loop", "verifier", "desk"},
		{"verify-desk", "worker", "desk"},
		{"verify-desk", "review", "reviewer"},
		{"the-desk", "worker", "desk"},
		{"the-desk", "review", "reviewer"},
		{"worker-desk", "worker", "desk"},
		{"pr-review-desk", "review", "reviewer"},
	} {
		t.Run(tc.loop+"/"+tc.kit, func(t *testing.T) {
			t.Setenv("DESK_LOOP", tc.loop)
			if got := claimRoleForKit(tc.kit); got != tc.want {
				t.Errorf("claimRoleForKit(%q) under DESK_LOOP=%q = %q, want %q", tc.kit, tc.loop, got, tc.want)
			}
			// The model stamp is a separate attestation and never follows the loop.
			if got, want := stampRoleForKit(tc.kit), map[bool]string{true: "reviewer", false: "desk"}[reviewKit(tc.kit)]; got != want {
				t.Errorf("stampRoleForKit(%q) = %q, want %q", tc.kit, got, want)
			}
		})
	}
}

// The claim credential is minted for the dispatching loop's claim role: verifier from
// verify-desk, desk from the-desk. Exactly one mint, so no second identity is borrowed.
// regression: #7815
func TestClaimAuthMintsLoopRole(t *testing.T) {
	for _, tc := range []struct{ loop, want string }{
		{"verify-desk", "verifier"},
		{"the-desk", "desk"},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			t.Setenv("DESK_LOOP", tc.loop)
			t.Setenv("GH_TOKEN", "")
			mints := stubMint(t, stubMintedToken, nil)
			o := dispatchOpts{kit: "verifier", item: "stream/05"}
			auth, err := resolveClaimAuth(o, allowedRepo, deskkit.ForgeGitHub, false)
			if err != nil {
				t.Fatalf("resolveClaimAuth: %v", err)
			}
			if len(*mints) != 1 || (*mints)[0].role != tc.want || (*mints)[0].repo != allowedRepo {
				t.Fatalf("mints = %+v, want exactly one %s mint for %s", *mints, tc.want, allowedRepo)
			}
			if !strings.HasPrefix(auth.source, "the "+tc.want+" App token") {
				t.Errorf("claim source = %q, want the %s App token", auth.source, tc.want)
			}
			if len(auth.args) != 2 || auth.args[0] != "--token-file" {
				t.Errorf("claim args = %q, want --token-file <path>", auth.args)
			}
		})
	}
}

// The NOT LAUNCHED progress and release commands name the dispatching loop's own role and run
// as printed from that window: no --cross-role on either, for verify-desk (verifier) and for
// the-desk (desk, unchanged).
// regression: #7815
func TestNotLaunchedNamesLoopRole(t *testing.T) {
	for _, tc := range []struct{ loop, role string }{
		{"verify-desk", "verifier"},
		{"the-desk", "desk"},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			t.Setenv("DESK_LOOP", tc.loop)
			o := dispatchOpts{item: "stream/05", kit: "verifier"}
			plan := dispatchPlan{claimKey: "at--stream--05", claimReleaseHint: goClaimBinary}
			line := captureStderr(t, func() { o.notLaunched(plan, allowedRepo, "/tmp/home") })
			if !strings.Contains(line, "NOT LAUNCHED") {
				t.Fatalf("no NOT LAUNCHED line:\n%s", line)
			}
			if strings.Contains(line, "--cross-role") {
				t.Errorf("NOT LAUNCHED line from %s borrows another role:\n%s", tc.loop, line)
			}
			hints := hintMint.FindAllStringSubmatch(line, -1)
			if len(hints) != 2 {
				t.Fatalf("NOT LAUNCHED line carries %d desktoken mint(s), want 2 (progress, release):\n%s", len(hints), line)
			}
			for _, h := range hints {
				if h[1] != tc.role {
					t.Errorf("printed mint names role %q, want %q: %s", h[1], tc.role, h[0])
				}
				if ok, why := admittedAsPrinted(t, tc.loop, h[0]); !ok {
					t.Errorf("%s NOT LAUNCHED hint refused as printed (%s): %s", tc.loop, why, h[0])
				}
			}
			if want := credFor(tc.role); strings.Count(line, want) != 2 {
				t.Errorf("NOT LAUNCHED line does not carry %s on both commands:\n%s", want, line)
			}
		})
	}
}

// The agent's own release line agrees with the claim role: a verifier dispatched from
// verify-desk releases as verifier with no override, the same role its window claimed under.
// regression: #7815
func TestVerifyDeskAgentReleaseRole(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withGoClaimOnPath(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	s.replies = []reply{{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"}}
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte("---\ngate: model\n---\n# fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	if rc := run([]string{"example-stream--07", "--root", root, "--repo", allowedRepo, "--kit", "verifier",
		"--dry-run", "--prompt-file", promptFile, "--model", "gpt-6-astra", "--brief", "spec.md"}); rc != deskkit.ExitOK {
		t.Fatalf("verifier dry run rc = %d, want 0", rc)
	}
	body, err := os.ReadFile(emittedPathFor(promptFile))
	if err != nil {
		t.Fatal(err)
	}
	line := lineWith(string(body), goClaimBinary+" release")
	if line == "" {
		t.Fatalf("no %s release line in the verifier prompt", goClaimBinary)
	}
	if !strings.Contains(line, credFor(verifierClaimRole)) || strings.Contains(line, "--cross-role") {
		t.Errorf("verify-desk agent release line does not run as verifier without an override:\n%s", line)
	}
}

// A real verifier dispatch from verify-desk acquires its claim under the verifier App and
// prints follow-up commands that need no other identity. The stamp stays the desk's.
// regression: #7815
func TestVerifyDeskDispatchClaimsAsVerifier(t *testing.T) {
	var (
		rc             int
		stdout, stderr string
	)
	stderr = captureStderr(t, func() {
		stdout = captureStdoutConcurrent(t, func() {
			_, _, rc, _ = world50Try(t, attestVerifierDispatch)
		})
	})
	if rc != deskkit.ExitOK {
		t.Fatalf("verify-desk verifier dispatch rc = %d, want 0\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	acquired := lineWith(stderr, stepClaimAcquire+" OK:")
	if acquired == "" {
		t.Fatalf("no %s OK line:\n%s", stepClaimAcquire, stderr)
	}
	if !strings.Contains(acquired, "authenticated by the verifier App token") {
		t.Errorf("verify-desk claim was not acquired under the verifier App:\n%s", acquired)
	}
	if strings.Contains(stdout+stderr, "--cross-role") {
		t.Errorf("a verify-desk verifier dispatch printed a cross-role mint:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if lineWith(stderr, "NOT LAUNCHED") == "" {
		t.Errorf("no NOT LAUNCHED line:\n%s", stderr)
	}
}

// captureStdoutConcurrent drains stdout while fn runs, so a dispatch's full step report cannot
// fill the pipe and block.
func captureStdoutConcurrent(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	var (
		b  strings.Builder
		wg sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			b.Write(buf[:n])
			if rerr != nil {
				return
			}
		}
	}()
	fn()
	os.Stdout = old
	_ = w.Close()
	wg.Wait()
	return b.String()
}
