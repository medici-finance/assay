package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// descendantdiff_test.go — #6610: ReadDescendantDiff answers only "which files differ
// between an ancestor and its descendant" on GitLab, from the merge base (ancestry) and an
// un-timed-out compare. Every other shape is could-not-check, and a non-GitLab backend is
// never served.

// regression: #6610
func TestGitLabDescendantDiff(t *testing.T) {
	anc, desc := strings.Repeat("ab", 20), strings.Repeat("cd", 20)
	repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
	type served struct {
		mergeBase string
		timeout   bool
		diffs     []map[string]any
	}
	serve := func(t *testing.T, s served) (Forge, *[]string) {
		t.Helper()
		var seen []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			seen = append(seen, r.URL.Path+"?"+r.URL.RawQuery)
			switch {
			case strings.HasSuffix(r.URL.Path, "/repository/merge_base"):
				_ = json.NewEncoder(w).Encode(map[string]any{"id": s.mergeBase})
			case strings.HasSuffix(r.URL.Path, "/repository/compare"):
				_ = json.NewEncoder(w).Encode(map[string]any{"commits": []any{}, "diffs": s.diffs, "compare_timeout": s.timeout})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)
		return OutboundChecked(&GitLabForge{Token: "fixture-token", BaseURL: srv.URL}, "verifier"), &seen
	}
	d := func(o, n string, added, renamed, deleted bool) map[string]any {
		return map[string]any{"old_path": o, "new_path": n, "new_file": added, "renamed_file": renamed, "deleted_file": deleted}
	}
	t.Run("a descendant's files", func(t *testing.T) {
		f, seen := serve(t, served{mergeBase: anc, diffs: []map[string]any{
			d("a.md", "a.md", false, false, false), d("b.md", "b.md", true, false, false),
			d("c.md", "c.md", false, false, true), d("d.md", "e.md", false, true, false),
		}})
		got, err := ReadDescendantDiff(f, repo, anc, desc)
		if err != nil {
			t.Fatal(err)
		}
		want := []ChangedFile{{Filename: "a.md", Status: "modified"}, {Filename: "b.md", Status: "added"}, {Filename: "c.md", Status: "removed"}, {Filename: "e.md", PreviousFilename: "d.md", Status: "renamed"}}
		if len(got) != len(want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("entry %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
		q := strings.Join(*seen, "\n")
		if !strings.Contains(q, "refs%5B%5D="+anc) || !strings.Contains(q, "refs%5B%5D="+desc) || !strings.Contains(q, "straight=true") {
			t.Fatalf("requests %q; want a merge base of both ids and a straight compare", q)
		}
	})
	for _, c := range []struct {
		name, anc, desc, want string
		s                     served
	}{
		{"not an ancestor", anc, desc, "not an ancestor", served{mergeBase: strings.Repeat("ef", 20)}},
		{"no merge base", anc, desc, "not an ancestor", served{}},
		{"a timed-out compare", anc, desc, "compare_timeout", served{mergeBase: anc, timeout: true}},
		{"an entry with no path", anc, desc, "no path", served{mergeBase: anc, diffs: []map[string]any{d("", "", false, false, false)}}},
		{"an abbreviated ancestor", anc[:7], desc, "full commit ids", served{mergeBase: anc}},
		{"a branch name", anc, "main", "full commit ids", served{mergeBase: anc}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := serve(t, c.s)
			got, err := ReadDescendantDiff(f, repo, c.anc, c.desc)
			if err == nil || !IsUnverifiable(err) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %+v, %v; want could-not-check naming %q", got, err, c.want)
			}
		})
	}
	t.Run("another backend is not served", func(t *testing.T) {
		gh := OutboundChecked(&GitHubForge{Token: "fixture-token"}, "verifier")
		if DescendantDiffServed(gh) {
			t.Fatal("DescendantDiffServed(GitHub) = true")
		}
		if _, err := ReadDescendantDiff(gh, repo, anc, desc); err == nil || !IsUnverifiable(err) {
			t.Fatalf("got %v; want could-not-check", err)
		}
	})
}
