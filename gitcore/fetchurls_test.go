package gitcore

import (
	"reflect"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #3207
func TestFetchURLs(t *testing.T) {
	const base = "https://example.invalid/"
	for _, remote := range []string{"origin", "upstream"} {
		for _, tc := range []struct {
			name  string
			fetch []string
			push  []string
			want  []string
		}{
			{"split", []string{"alias:first"}, []string{base + "other.git"}, []string{base + "first"}},
			{"same", []string{"alias:first"}, []string{"alias:first"}, []string{base + "first"}},
			{"multi-fetch", []string{"alias:first", "alias:second"}, []string{base + "other.git"}, []string{base + "first", base + "second"}},
			{"push-only", nil, []string{base + "other.git"}, nil},
			{"no-urls", nil, nil, nil},
		} {
			t.Run(remote+"/"+tc.name, func(t *testing.T) {
				f := gittest.NewFixture(t)
				config := func(args ...string) {
					t.Helper()
					if _, err := f.Git(append([]string{"config"}, args...)...); err != nil {
						t.Fatal(err)
					}
				}
				config("remote."+remote+".fetch", "+refs/heads/*:refs/remotes/"+remote+"/*")
				config("url."+base+".insteadOf", "alias:")
				// Write push first: file order must not turn it into a fetch destination.
				for _, u := range tc.push {
					config("--add", "remote."+remote+".pushurl", u)
				}
				for _, u := range tc.fetch {
					config("--add", "remote."+remote+".url", u)
				}
				before, err := f.Git("config", "--local", "--list")
				if err != nil {
					t.Fatal(err)
				}
				repo, err := Open(f.Dir)
				if err != nil {
					t.Fatal(err)
				}
				got, err := repo.RemoteURLs(remote)
				if tc.want == nil {
					if err == nil {
						t.Fatalf("RemoteURLs(%s) = %v, want no-fetch-URL error", remote, got)
					}
				} else if err != nil || !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("RemoteURLs(%s) = %v, %v, want %v", remote, got, err, tc.want)
				}
				after, err := f.Git("config", "--local", "--list")
				if err != nil || after != before {
					t.Fatalf("config changed: %v", err)
				}
			})
		}
	}
}

// regression: #3281
func TestFetchURLs_PushInsteadOf(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fetch     []string
		insteadOf map[string]string // base -> prefix
		pushInst  map[string]string // base -> prefix
		want      []string
	}{
		{
			name:     "pushInsteadOf-only does not rewrite fetch URL",
			fetch:    []string{"https://github.com/example/project.git"},
			pushInst: map[string]string{"https://github.com/": "https://github.com/"},
			want:     []string{"https://github.com/example/project.git"},
		},
		{
			name:      "insteadOf rewrites while pushInsteadOf does not corrupt non-matching fetch URL",
			fetch:     []string{"alias:first.git", "https://github.com/second.git"},
			insteadOf: map[string]string{"https://github.com/": "alias:"},
			pushInst:  map[string]string{"https://github.com/": "https://github.com/"},
			want:      []string{"https://github.com/first.git", "https://github.com/second.git"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := gittest.NewFixture(t)
			config := func(args ...string) {
				t.Helper()
				if _, err := f.Git(append([]string{"config"}, args...)...); err != nil {
					t.Fatal(err)
				}
			}
			for base, prefix := range tc.insteadOf {
				config("url."+base+".insteadOf", prefix)
			}
			for base, prefix := range tc.pushInst {
				config("url."+base+".pushInsteadOf", prefix)
			}
			for _, u := range tc.fetch {
				config("--add", "remote.origin.url", u)
			}
			repo, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := repo.RemoteURLs("origin")
			if err != nil {
				t.Fatalf("RemoteURLs(origin) = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RemoteURLs(origin) = %v, want %v", got, tc.want)
			}
		})
	}
}
