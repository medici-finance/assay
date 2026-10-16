package main

// claimstore_test.go — the claim-store seam: step 1 asks deskkit.ResolveClaimStore where the claim
// is kept. A store that does not resolve refuses BEFORE anything durable happens — no child
// process, no worktree, no credential minted — and a resolved store is named on the step's
// report line. A valid store this build does not ship resolves to the legacy store under its
// own NOTICE (medici-finance/assay#2753).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// withClaimStoreKey rewrites the harness's fixture roster with one extra claim-store line.
func withClaimStoreKey(t *testing.T, home, line string) {
	t.Helper()
	p := filepath.Join(home, ".config", "assay", "roster.env")
	if err := os.WriteFile(p, []byte(fixtureRoster+line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// countMints wraps the claim step's mint seam so a test can assert no credential was minted.
func countMints(t *testing.T) *int {
	t.Helper()
	n := 0
	prev := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		n++
		return prev(role, repo)
	}
	t.Cleanup(func() { mintTokenFn = prev })
	return &n
}

func TestDispatchRefusesBeforeWorktreeOnStoreRefusal(t *testing.T) {
	for _, c := range []struct{ why, line, want string }{
		{"forge-ref selected explicitly", deskkit.EnvClaimStore + "=" + deskkit.ClaimStoreForgeRef, "cannot be selected"},
		{"an unknown store", deskkit.EnvClaimStore + "=nfs", "Valid values are file and service"},
		{"a malformed declaration", deskkit.EnvClaimSingleHost + "=true", "the only value is yes"},
	} {
		t.Run(c.why, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			withClaimStoreKey(t, home, c.line)
			mints := countMints(t)

			_, errOut, rc := ddRunCapture(t, []string{"item-1", "--root", root, "--repo", allowedRepo,
				"--kit", "worker"})
			if rc != deskkit.ExitUnverifiable {
				t.Fatalf("rc = %d, want %d (a store that does not resolve is exit 6); stderr:\n%s",
					rc, deskkit.ExitUnverifiable, errOut)
			}
			if len(s.calls) != 0 {
				t.Fatalf("%d child process(es) ran before the store refusal: %v — the refusal must come "+
					"before any claim, worktree or other child", len(s.calls), s.calls)
			}
			if s.ran("deskwt add") {
				t.Fatal("a worktree was cut for a dispatch whose claim store did not resolve")
			}
			if *mints != 0 {
				t.Fatalf("a credential was minted %d time(s) for a dispatch whose claim store did not resolve", *mints)
			}
			for _, f := range []string{stepClaimAcquire, "claim store", c.want} {
				if !strings.Contains(errOut, f) {
					t.Errorf("the refusal does not name %q:\n%s", f, errOut)
				}
			}
		})
	}
}

// TestDispatchReportsTheResolvedClaimStore — with the key unset, the claim goes to the legacy
// forge-ref store exactly as before, the step line names it, and the removal NOTICE is printed.
func TestDispatchReportsTheResolvedClaimStore(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	mints := countMints(t)

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	_, errOut, rc := ddRunCapture(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--kit", "worker", "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0; stderr:\n%s", rc, errOut)
	}
	if !s.ran("dispatch-claim.sh acquire example-stream--07") {
		t.Fatal("the legacy resolution did not take the claim through the claim tool as before")
	}
	if *mints != 1 {
		t.Fatalf("the forge-ref store needs the role credential: minted %d time(s), want 1", *mints)
	}
	if !strings.Contains(errOut, "claim-acquire OK: ") ||
		!strings.Contains(errOut, ", store forge-ref (legacy), authenticated by ") {
		t.Errorf("the claim-acquire line does not name the resolved store:\n%s", errOut)
	}
	if !strings.Contains(errOut, toolName+": "+deskkit.ClaimStoreLegacyNotice) {
		t.Errorf("the removal NOTICE was not printed:\n%s", errOut)
	}
}

// TestDispatchUnshippedFallback — ASSAY_CLAIM_STORE=file (or service) names a valid store
// this build does not ship: the dispatch no longer exits 6, the claim goes to the legacy
// forge-ref store exactly as an unset key's would, and the NOTICE names the configured store
// and the release that ships it.
// regression: #2753
func TestDispatchUnshippedFallback(t *testing.T) {
	for _, c := range []struct{ store, ships string }{
		{deskkit.ClaimStoreFile, "ships the file store"},
		{deskkit.ClaimStoreService, "ships the served store"},
	} {
		t.Run(c.store, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			withClaimStoreKey(t, home, deskkit.EnvClaimStore+"="+c.store+"\n"+deskkit.EnvClaimSingleHost+"=yes")
			mints := countMints(t)

			promptFile := filepath.Join(t.TempDir(), "prompt.md")
			_, errOut, rc := ddRunCapture(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
				"--kit", "worker", "--prompt-file", promptFile})
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0 (an unshipped store is ignored, never exit %d); stderr:\n%s",
					rc, deskkit.ExitUnverifiable, errOut)
			}
			if !s.ran("dispatch-claim.sh acquire example-stream--07") {
				t.Fatal("the unshipped store's fallback did not take the claim through the legacy claim tool")
			}
			if *mints != 1 {
				t.Fatalf("the forge-ref store needs the role credential: minted %d time(s), want 1", *mints)
			}
			if !strings.Contains(errOut, ", store forge-ref (legacy), authenticated by ") {
				t.Errorf("the claim-acquire line does not name the legacy store:\n%s", errOut)
			}
			if strings.Contains(errOut, deskkit.ClaimStoreLegacyNotice) {
				t.Errorf("the UNSET notice was printed for a key the operator did set:\n%s", errOut)
			}
			for _, f := range []string{toolName + ": NOTICE: " + deskkit.EnvClaimStore + "=" + c.store,
				"does not ship", c.ships} {
				if !strings.Contains(errOut, f) {
					t.Errorf("the unshipped-store NOTICE does not carry %q:\n%s", f, errOut)
				}
			}
		})
	}
}
