package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// failclosed_test.go — source-level guards that survive the forge migration.
//
// The pre-migration guards in this file pinned the SHAPE of resolveInstallID (no fabricated
// installation id) and the httpClient timeout, and exercised the verifier-App PEM search path.
// All three concerns left this package with the JWT/installation exchange: the mint now lives in
// the identity layer (`desktoken verifier`), reached through mintVerifierToken, and the custody
// binding is the resolver's (internal/deskkit/forgeresolve.go and its tests). The one guard that
// still belongs here is the no-parallel rule, because this package's tests still mutate shared
// package-level state through setupFake.

// TestNoParallelTestsInPackage: this package's tests mutate package-level state (the forgeForFn
// / mintTokenFn / publicRepoGateFn seams, ghToken, stdout, stderr, lockWait) through setupFake,
// none of which is safe to share. Today no test calls t.Parallel, so those mutations are safe; a
// future one added without noticing would race them into an intermittently green suite — the
// worst failure mode for a package whose whole job is proving guards fire.
func TestNoParallelTestsInPackage(t *testing.T) {
	entries, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no _test.go files found — this guard is looking in the wrong directory")
	}
	// Assembled at run time so this guard's own source does not contain the literal it searches
	// for — otherwise it reports itself and can never pass.
	needle := "t." + "Parallel("
	for _, name := range entries {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, needle) {
				t.Errorf("%s:%d parallelises a test; this package's tests share mutable package state "+
					"(the forgeForFn / mintTokenFn / publicRepoGateFn seams, ghToken, stdout, stderr, lockWait "+
					"via setupFake). Make that state per-test before parallelising anything here.", name, i+1)
			}
		}
	}
}

// --- Custody backstops ---
//
// deskevidence reaches the forge ONLY as the verifier App, under a token it mints itself. Two
// custody guards enforce that, and both were previously unexercised because setupFake stubs the
// mint as always-succeed. These drive each guard directly.

// TestMintFailureAbortsBeforeForge exercises the mint-failure abort in cmdEvidence
// (`if merr := mintTokenFn(repoSlug); merr != nil { return merr }`). The mint is deliberately
// placed after the cheap/stateless refusals and BEFORE the forge is resolved, so a mint failure
// must propagate that error and never reach the forge — a doomed call must not act as any
// identity. Fail-first: with the mint forced to fail, the forge must record ZERO hits.
func TestMintFailureAbortsBeforeForge(t *testing.T) {
	f, _ := setupFake(t)

	mintErr := deskkit.Unverifiable(
		"desktoken verifier --repo example-org/tracker: mint refused", errors.New("mint boom"))
	mintTokenFn = func(string) error { return mintErr }

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | ... | row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("mint failure exit = %d, want %d (the mint error must propagate)", code, deskkit.ExitUnverifiable)
	}
	if len(f.hits) != 0 {
		t.Fatalf("mint failed but the forge was reached: %v — the abort must precede any forge call", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("mint failed but %d WriteFile call(s) were made", f.putCalls)
	}
}

// TestCustodyMintRefusesWithNoMintedToken exercises the empty-token refusal in the GitHub custody
// step deskevidence installs (githubCustodyMint, registered via deskkit.SetGitHubCustodyMinter).
// With no token minted the step must REFUSE — never fall back to an ambient forge identity — and
// yield no token. Fail-first: the empty-token call is asserted before the minted-token counterpart
// proves the refusal is the empty-token branch, not an unconditional failure.
func TestCustodyMintRefusesWithNoMintedToken(t *testing.T) {
	old := ghToken
	ghToken = ""
	t.Cleanup(func() { ghToken = old })

	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}

	tok, _, err := githubCustodyMint("verifier", repo)
	if err == nil {
		t.Fatal("custody step returned nil error with no minted token — it must refuse")
	}
	if tok != "" {
		t.Fatalf("custody step returned a token %q with none minted", tok)
	}
	if !strings.Contains(err.Error(), "no minted verifier token") {
		t.Fatalf("refusal message = %q, want it to name the missing minted verifier token", err.Error())
	}

	// Counterpart: once a token IS minted the same step hands it back, proving the refusal above
	// is the empty-token branch and not an unconditional failure.
	ghToken = "minted-verifier-token"
	tok, _, err = githubCustodyMint("verifier", repo)
	if err != nil {
		t.Fatalf("custody step refused a minted token: %v", err)
	}
	if tok != "minted-verifier-token" {
		t.Fatalf("custody step returned token %q, want the minted one", tok)
	}
}
