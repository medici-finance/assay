package gitcore

import (
	"reflect"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #3281
func TestFetchRewriteScope(t *testing.T) {
	// These synthetic public-forge remotes are rewrite keys only; no host is contacted.
	// Canonical forge hosts are admitted by the existing disclosure-shape policy.
	const target = "https://github.com/example/project.git"
	for _, tc := range []struct {
		name, raw, want string
		rules           [][]string
	}{
		{"push-only", target, target, [][]string{{"url.https://github.com/.pushInsteadOf", "https://github.com/"}}},
		{"unmatched-push", target, target, [][]string{{"url.https://elsewhere.invalid/.pushInsteadOf", "unmatched:"}}},
		{"longest-fetch", "github.com:example-team/project.git", target, [][]string{
			{"url.https://elsewhere.invalid/.insteadOf", "github.com:"},
			{"url.https://github.com/example/.insteadOf", "github.com:example-team/"},
			{"url.https://push.invalid/.pushInsteadOf", "github.com:example-team/"},
		}},
		{"both-options", "github.com:project.git", target, [][]string{
			{"url.https://github.com/example/.insteadOf", "github.com:"},
			{"url.https://github.com/example/.pushInsteadOf", "other:"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := gittest.NewFixture(t)
			for _, remote := range []string{"origin", "upstream"} {
				if _, err := f.Git("config", "remote."+remote+".url", tc.raw); err != nil {
					t.Fatal(err)
				}
			}
			for _, rule := range tc.rules {
				if _, err := f.Git(append([]string{"config"}, rule...)...); err != nil {
					t.Fatal(err)
				}
			}
			before, err := f.Git("config", "--local", "--list")
			if err != nil {
				t.Fatal(err)
			}
			repo, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, remote := range []string{"origin", "upstream"} {
				got, err := repo.RemoteURLs(remote)
				if err != nil || !reflect.DeepEqual(got, []string{tc.want}) {
					t.Errorf("RemoteURLs(%s) = %q, %v; want %q", remote, got, err, tc.want)
				}
				first, err := repo.RemoteURL(remote)
				if err != nil || first != tc.want {
					t.Errorf("RemoteURL(%s) = %q, %v; want %q", remote, first, err, tc.want)
				}
			}
			after, err := f.Git("config", "--local", "--list")
			if err != nil || after != before {
				t.Fatalf("URL read changed config: %v", err)
			}
		})
	}
}
