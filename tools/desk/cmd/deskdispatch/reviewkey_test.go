package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestReviewKeyPreclaim(t *testing.T) {
	for _, key := range []string{"unknown--pr-547", "unknown--pr-547--security", "tracker--pr-5479", "tracker--issue-547"} {
		t.Run(key, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "home"))
			rc := run([]string{key, "--root", root, "--repo", "example-org/tracker", "--kit", "review", "--pr", "547", "--dry-run"})
			if rc != deskkit.ExitRefused {
				t.Fatalf("unreadable review claim accepted: rc=%d key=%s", rc, key)
			}
			if s.ran("acquire") || s.ran("desktoken") || s.ran("deskwt") {
				t.Fatalf("invalid key reached durable effects: %v", s.calls)
			}
		})
	}
}

func TestReviewKeyPreserved(t *testing.T) {
	for _, alias := range []string{"", "tracker=trk:"} {
		for _, prefix := range []string{"tracker", "trk"} {
			if alias == "" && prefix == "trk" {
				continue
			}
			for _, lane := range []string{"", "--correctness", "--security"} {
				t.Run(alias+prefix+lane, func(t *testing.T) {
					s := &stub{}
					home, root := s.install(t)
					plantScripts(t, root)
					path := filepath.Join(home, ".config", "assay", "roster.env")
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(path, append(b, []byte("\nASSAY_REPO_ALIASES="+alias+"\n")...), 0600); err != nil {
						t.Fatal(err)
					}
					deskkit.ReloadConfig()
					s.replies = happyReplies(filepath.Join(t.TempDir(), "home"))
					key := prefix + "--pr-547" + lane
					plan, err := validateCallerPreconditions(dispatchOpts{item: key, root: root, repo: "example-org/tracker", kit: "review", pr: 547, tier: "any", dryRun: true})
					if err != nil {
						t.Fatal(err)
					}
					if plan.claimKey != key {
						t.Fatalf("key changed: %s -> %s", key, plan.claimKey)
					}
					got := deskkit.ReadReviewClaims(plan.repo, 547, func(family string) ([]string, error) {
						if strings.HasPrefix(deskkit.DispatchClaimActiveRefsPrefix+plan.claimKey, family) {
							return []string{deskkit.DispatchClaimActiveRefsPrefix + plan.claimKey}, nil
						}
						return nil, nil
					})
					if got != deskkit.ClaimHeld {
						t.Fatalf("accepted key not held by reader: %s %v", key, got)
					}
				})
			}
		}
	}
}
