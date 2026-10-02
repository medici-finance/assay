package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type aliasForge struct {
	deskkit.Forge
	refs []string
	err  error
}

func (f *aliasForge) MatchingRefs(deskkit.ForgeRepo, string) ([]string, error) {
	return f.refs, f.err
}

func TestAliasClaimReaders(t *testing.T) {
	for _, lane := range []string{"--correctness", "--security"} {
		for _, adapter := range []string{"github", "forge"} {
			t.Run(adapter+lane, func(t *testing.T) {
				f, _ := setupFake(t)
				path := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(b, []byte("\nASSAY_REPO_ALIASES=tracker=trk:\n")...), 0600); err != nil {
					t.Fatal(err)
				}
				deskkit.ReloadConfig()
				f.matchingRefs = []string{"refs/dispatch/tracker--pr-547" + lane}
				var got deskkit.ClaimLiveness
				if adapter == "github" {
					cl, err := newGHClient("example-org", "tracker")
					if err != nil {
						t.Fatal(err)
					}
					got = cl.claimLiveness(exampleRepo, 547)
				} else {
					cl := &forgeBackend{fg: &aliasForge{refs: f.matchingRefs}, repo: deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}}
					got = cl.claimLiveness(exampleRepo, 547)
				}
				if got != deskkit.ClaimHeld {
					t.Fatalf("accepted basename claim with alias reads %v, want held", got)
				}
			})
		}
	}
}

func setClaimAlias(t *testing.T) {
	t.Helper()
	path := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(b, []byte("\nASSAY_REPO_ALIASES=tracker=trk:\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
}

func TestAliasClaimReaderStates(t *testing.T) {
	for _, adapter := range []string{"github", "forge"} {
		for _, tc := range []struct {
			name       string
			refs       []string
			unreadable bool
			want       deskkit.ClaimLiveness
		}{
			{"alias", []string{"refs/dispatch/trk--pr-547--security"}, false, deskkit.ClaimHeld},
			{"empty", []string{}, false, deskkit.ClaimReleased},
			{"unrelated", []string{"refs/dispatch/tracker--pr-5479--security", "refs/dispatch/foreign--pr-547"}, false, deskkit.ClaimReleased},
			{"unreadable", []string{}, true, deskkit.ClaimLivenessUnknown},
		} {
			t.Run(adapter+"/"+tc.name, func(t *testing.T) {
				f, _ := setupFake(t)
				setClaimAlias(t)
				f.matchingRefs = tc.refs
				var readErr error
				if tc.unreadable {
					f.matchingRefsStatus = 503
					readErr = fmt.Errorf("offline unavailable")
				}
				var got deskkit.ClaimLiveness
				if adapter == "github" {
					cl, err := newGHClient("example-org", "tracker")
					if err != nil {
						t.Fatal(err)
					}
					got = cl.claimLiveness(exampleRepo, 547)
				} else {
					cl := &forgeBackend{fg: &aliasForge{refs: tc.refs, err: readErr}, repo: deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}}
					got = cl.claimLiveness(exampleRepo, 547)
				}
				if got != tc.want {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			})
		}
	}
}

func TestAliasClaimFloorConsumers(t *testing.T) {
	for _, verb := range []string{"review", "ready"} {
		for _, state := range []string{"held", "released", "unknown"} {
			t.Run(verb+"/"+state, func(t *testing.T) {
				f, errBuf := setupFake(t)
				setClaimAlias(t)
				f.stamp(strongStampBy(deskDispatcherLogin(t))...)
				f.matchingRefs = []string{"refs/dispatch/tracker--pr-1--security"}
				if state == "released" {
					f.matchingRefs = []string{}
				}
				if state == "unknown" {
					f.matchingRefsStatus = 503
				}
				var code int
				if verb == "review" {
					f.files = riskyFiles()
					bf := writeBody(t, "rev.md", okReviewBody)
					code = run(reviewArgs(exampleRepo, "1", "approve", testHead, bf))
				} else {
					f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
					f.status = greenStatus()
					code = run(readyArgs(exampleRepo))
				}
				want := 0
				if verb == "review" && state == "released" {
					want = deskkit.ExitRefused
				}
				if code != want {
					t.Fatalf("exit=%d want=%d: %s", code, want, errBuf.String())
				}
				if state == "released" {
					for _, part := range []string{"refs/dispatch/trk--pr-1", "refs/dispatch/tracker--pr-1", "Unconfigured historical aliases"} {
						if !strings.Contains(errBuf.String(), part) {
							t.Errorf("missing release diagnostic %q: %s", part, errBuf.String())
						}
					}
					if verb == "review" && f.postedReview != 0 {
						t.Fatal("released risk verdict posted")
					}
				} else if strings.Contains(errBuf.String(), "No live claim matched") {
					t.Fatalf("incorrect release claim: %s", errBuf.String())
				}
			})
		}
	}
}

// GitLab cannot enumerate custom claim refs. Its concrete backend must remain
// Unknown through the shared adapter, rather than invent a release for either family.
func TestAliasClaimGitLabUnknown(t *testing.T) {
	setupFake(t)
	setClaimAlias(t)
	cl := &forgeBackend{fg: &deskkit.GitLabForge{}, repo: deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}}
	if got := cl.claimLiveness(exampleRepo, 547); got != deskkit.ClaimLivenessUnknown {
		t.Fatalf("GitLab liveness=%v, want unknown", got)
	}
}
