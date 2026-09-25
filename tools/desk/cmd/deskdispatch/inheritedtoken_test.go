package main

// inheritedtoken_test.go — issue 1631. A cell shim exported the operator's ambient `gh` login as
// GH_TOKEN, and this verb read ANY GH_TOKEN as a deliberate operator override: no role mint, no
// --token-file, and the claim went out under the human's credential. An inherited token now wins
// only once it is shown to BE the dispatching role's App; anything else is ignored with a NOTICE
// and the role token minted; a token whose identity cannot be read refuses before any claim.
//
// These drive the same REAL fake claim child claimauth_test.go uses, so "the claim ran under the
// role token" is asserted on what the child observed.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// dispatcherAppIdentity is the account the fixture roster binds to the WORKER-kit dispatching
// role (deskkit.DispatcherRole): its App's login and pinned bot USER id.
var dispatcherAppIdentity = deskkit.TokenIdentity{Login: "assay-desk-app[bot]", ID: 300000001}

// stubTokenIdentity binds the inherited-token probe to answer id (or err) and records every
// token it was asked about. It also fails the test unless the probe was scoped to the TARGET
// repo and the origin read from --root: the resolver binds the token's destination host to
// those two, so a probe handed anything else could offer the token to the wrong host.
func stubTokenIdentity(t *testing.T, id deskkit.TokenIdentity, err error) *[]string {
	t.Helper()
	var asked []string
	old := tokenIdentityFn
	tokenIdentityFn = func(repo deskkit.ForgeRepo, originURL, tok string) (deskkit.TokenIdentity, error) {
		if repo.Slug() != allowedRepo {
			t.Errorf("the probe was scoped to repo %q, want the dispatch target %q", repo.Slug(), allowedRepo)
		}
		if !strings.Contains(originURL, "github.com") {
			t.Errorf("the probe was handed origin %q, want the target checkout's origin remote", originURL)
		}
		asked = append(asked, tok)
		return id, err
	}
	t.Cleanup(func() { tokenIdentityFn = old })
	return &asked
}

// A NON-APP inherited token — the human login a cell shim put there, or ANOTHER role's App — is
// not an override: the role token is minted and handed to the claim child exactly as with nothing
// exported, the inherited value reaches no child, and a NOTICE names what was ignored.
func TestInheritedNonRoleTokenIsIgnoredAndTheRoleTokenMinted(t *testing.T) {
	for _, who := range []struct {
		name string
		id   deskkit.TokenIdentity
	}{
		{"human-login", deskkit.TokenIdentity{Login: "ada", ID: 2001}},
		{"another-roles-app", deskkit.TokenIdentity{Login: "assay-worker-app[bot]", ID: 300000006}},
	} {
		for _, tool := range []string{"go-binary", "legacy-script"} {
			t.Run(who.name+"/"+tool, func(t *testing.T) {
				s := &stub{}
				_, root := s.install(t)
				if tool == "go-binary" {
					plantGoClaimChild(t)
				} else {
					plantScriptClaimChild(t, root)
				}
				record := installClaimChild(t, s)
				s.replies = happyReplies("/private/tmp/worker-home")
				t.Setenv("GH_TOKEN", "example-inherited-human-token")
				asked := stubTokenIdentity(t, who.id, nil)
				mints := stubMint(t, stubMintedToken, nil)

				rc, stderr := runCapturingStderr(t, []string{"example--stream--07", "--root", root, "--repo", allowedRepo,
					"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
				if rc != deskkit.ExitOK {
					t.Fatalf("dispatch rc = %d, want 0:\n%s", rc, stderr)
				}
				if len(*asked) != 1 || (*asked)[0] != "example-inherited-human-token" {
					t.Errorf("the probe was asked about %v, want exactly the inherited token once", *asked)
				}
				if len(*mints) != 1 {
					t.Fatalf("the role token was minted %d time(s), want 1 — an ignored export must not stop the mint", len(*mints))
				}
				r := acquireRecord(t, record)
				if tool == "go-binary" {
					if r.tokenFile != "/tmp/example-token-path" { // stubMint's token file
						t.Errorf("the Go claim binary was handed --token-file %q, want the minted role token file (argv %s)", r.tokenFile, r.argv)
					}
					if r.ghToken != "" {
						t.Errorf("the Go claim binary still inherited GH_TOKEN=%q — the ignored token must reach no child", r.ghToken)
					}
				} else if r.ghToken != stubMintedToken {
					t.Errorf("the legacy script saw GH_TOKEN=%q, want the minted role token", r.ghToken)
				}
				if os.Getenv("GH_TOKEN") != "" {
					t.Errorf("the ignored token is still in the dispatcher's environment, where every later child inherits it")
				}
				for _, want := range []string{"NOTICE", "IGNORED", who.id.Login, "assay-desk-app[bot]"} {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr does not carry %q:\n%s", want, stderr)
					}
				}
				if strings.Contains(stderr, "example-inherited-human-token") {
					t.Errorf("the inherited token VALUE was printed:\n%s", stderr)
				}
			})
		}
	}
}

// FAIL CLOSED: an inherited token whose identity cannot be read (a failed probe — transport, a
// 401) is neither honoured nor silently swapped: the dispatch refuses exit 6 naming the step, and
// NO claim child runs.
func TestInheritedTokenWithUnreadableIdentityRefusesBeforeAnyClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")
	t.Setenv("GH_TOKEN", "example-inherited-human-token")
	stubTokenIdentity(t, deskkit.TokenIdentity{}, errors.New("example probe failure: HTTP 401 Bad credentials"))
	mints := stubMint(t, stubMintedToken, nil)

	err := cmdDispatch([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("an inherited token of unknown identity was used — the claim would have run under it")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d: %v", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable, err)
	}
	for _, want := range []string{stepClaimAcquire, "example probe failure", "NO claim", "Unset GH_TOKEN"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not carry %q: %s", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "example-inherited-human-token") {
		t.Errorf("the refusal carries the token VALUE: %s", err.Error())
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatal("the claim child RAN although the inherited token's identity could not be read")
	}
	if len(*mints) != 0 {
		t.Errorf("minted %d time(s) — an unverifiable export is a refusal, never a silent swap", len(*mints))
	}
}

// FAIL CLOSED: a roster that binds no App identity to the dispatching role leaves nothing to check
// the export against — a named refusal (exit 5), not an assumption either way.
func TestInheritedTokenWithUnboundRoleRefuses(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")
	unbound := strings.Replace(fixtureRoster, "desk=assay-desk-app:300000001,", "", 1)
	if unbound == fixtureRoster {
		t.Fatal("fixture drift: the desk binding was not found to remove")
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(unbound), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Setenv("GH_TOKEN", "example-inherited-token")
	stubTokenIdentity(t, dispatcherAppIdentity, nil)

	err := cmdDispatch([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal (exit %d), got %v", deskkit.ExitRefused, err)
	}
	if !strings.Contains(err.Error(), "binds no App identity") {
		t.Errorf("the refusal does not name the missing binding: %v", err)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatal("the claim child RAN with no binding to check the export against")
	}
}

// REGRESSION FLOOR: the deliberate override still works — an exported token that IS the
// dispatching role's App is honoured with no mint and no NOTICE, and the report says it was
// verified.
func TestInheritedRoleAppTokenIsHonouredWithoutANotice(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	s.replies = happyReplies("/private/tmp/worker-home")
	t.Setenv("GH_TOKEN", "example-explicit-export")
	stubTokenIdentity(t, dispatcherAppIdentity, nil)
	mints := stubMint(t, stubMintedToken, nil)

	rc, stderr := runCapturingStderr(t, []string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0:\n%s", rc, stderr)
	}
	r := acquireRecord(t, record)
	if r.ghToken != "example-explicit-export" || r.tokenFile != "" {
		t.Errorf("the child saw GH_TOKEN=%q --token-file=%q, want the verified export and no token file", r.ghToken, r.tokenFile)
	}
	if len(*mints) != 0 {
		t.Errorf("minted %d time(s) although the export is the role's own App token", len(*mints))
	}
	if strings.Contains(stderr, "IGNORED") {
		t.Errorf("a verified export earned the ignored-token NOTICE:\n%s", stderr)
	}
	if !strings.Contains(stderr, "verified: it acts as assay-desk-app[bot]") {
		t.Errorf("the claim-acquire line does not say the export was verified:\n%s", stderr)
	}
}
