package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// treelisting_test.go — ReadTreeListing answers with the whole listing of one directory at
// one commit, or could-not-check (the board read past a capped compare, #5913).

const (
	tlCommit = "1111111111111111111111111111111111111111"
	tlTree   = "2222222222222222222222222222222222222222"
)

// tlServer serves the GraphQL object read and the recursive git-trees read. commitObj and
// dirObj are the two GraphQL objects (nil = null); tree is the git-trees body.
func tlServer(t *testing.T, commitObj, dirObj map[string]any, gqlErrors bool, tree map[string]any) (*GitHubForge, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/graphql":
			var in struct {
				Variables map[string]string `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.Variables["rev"] != tlCommit || in.Variables["expr"] != tlCommit+":docs/streams" {
				t.Errorf("graphql variables = %v", in.Variables)
			}
			body := map[string]any{"data": map[string]any{"repository": map[string]any{"commit": commitObj, "dir": dirObj}}}
			if gqlErrors {
				body["errors"] = []map[string]any{{"message": "boom"}}
			}
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/o/r/git/trees/"):
			if r.URL.Query().Get("recursive") != "1" {
				t.Errorf("tree read not recursive: %s", r.URL.RequestURI())
			}
			_ = json.NewEncoder(w).Encode(tree)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &GitHubForge{Token: "stub", BaseURL: srv.URL}, &paths
}

func tlEntry(path, mode, typ, sha string) map[string]any {
	return map[string]any{"path": path, "mode": mode, "type": typ, "sha": sha}
}

func TestReadTreeListing(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "r"}
	commit := map[string]any{"__typename": "Commit", "oid": tlCommit}
	dir := map[string]any{"__typename": "Tree", "oid": tlTree}
	whole := map[string]any{"sha": tlTree, "truncated": false, "tree": []map[string]any{
		tlEntry("sdlc", "040000", "tree", strings.Repeat("3", 40)),
		tlEntry("sdlc/README.md", "100644", "blob", strings.Repeat("A", 40)),
		tlEntry("sdlc/link.md", "120000", "blob", strings.Repeat("5", 40)),
	}}
	t.Run("whole listing", func(t *testing.T) {
		g, paths := tlServer(t, commit, dir, false, whole)
		got, err := ReadTreeListing(g, repo, tlCommit, "docs/streams")
		if err != nil {
			t.Fatal(err)
		}
		want := []TreeEntry{
			{Path: "docs/streams/sdlc/README.md", Mode: "100644", OID: strings.Repeat("a", 40)},
			{Path: "docs/streams/sdlc/link.md", Mode: "120000", OID: strings.Repeat("5", 40)},
		}
		if len(got) != len(want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("entry %d = %+v, want %+v", i, got[i], want[i])
			}
		}
		if len(*paths) != 2 || !strings.Contains((*paths)[1], "/git/trees/"+tlTree+"?recursive=1") {
			t.Fatalf("requests = %v, want the object read then the recursive read of the resolved tree", *paths)
		}
	})
	t.Run("absent directory is empty", func(t *testing.T) {
		g, paths := tlServer(t, commit, nil, false, nil)
		got, err := ReadTreeListing(g, repo, tlCommit, "docs/streams")
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %+v, %v; want an empty listing", got, err)
		}
		if len(*paths) != 1 {
			t.Fatalf("requests = %v, want no tree read for an absent directory", *paths)
		}
	})
	bad := []struct {
		name      string
		commitObj map[string]any
		dirObj    map[string]any
		gqlErrors bool
		tree      map[string]any
	}{
		{"commit absent", nil, nil, false, nil},
		{"commit resolves elsewhere", map[string]any{"__typename": "Commit", "oid": strings.Repeat("9", 40)}, dir, false, whole},
		{"graphql errors", commit, dir, true, whole},
		{"dir is a blob", commit, map[string]any{"__typename": "Blob", "oid": tlTree}, false, whole},
		{"listing truncated", commit, dir, false, map[string]any{"sha": tlTree, "truncated": true, "tree": whole["tree"]}},
		{"truncated flag missing", commit, dir, false, map[string]any{"sha": tlTree, "tree": whole["tree"]}},
		{"tree list missing", commit, dir, false, map[string]any{"sha": tlTree, "truncated": false}},
		{"another tree served", commit, dir, false, map[string]any{"sha": strings.Repeat("4", 40), "truncated": false, "tree": whole["tree"]}},
		{"entry without an object id", commit, dir, false, map[string]any{"sha": tlTree, "truncated": false, "tree": []map[string]any{tlEntry("a.md", "100644", "blob", "")}}},
		{"path served twice", commit, dir, false, map[string]any{"sha": tlTree, "truncated": false, "tree": []map[string]any{
			tlEntry("a.md", "100644", "blob", strings.Repeat("1", 40)), tlEntry("a.md", "100644", "blob", strings.Repeat("2", 40))}}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			g, _ := tlServer(t, c.commitObj, c.dirObj, c.gqlErrors, c.tree)
			got, err := ReadTreeListing(g, repo, tlCommit, "docs/streams")
			if err == nil || !IsUnverifiable(err) || got != nil {
				t.Fatalf("got %+v, %v; want could-not-check", got, err)
			}
		})
	}
	t.Run("arguments refused before any read", func(t *testing.T) {
		g, paths := tlServer(t, commit, dir, false, whole)
		for _, a := range []struct{ commit, dir string }{
			{"main", "docs/streams"}, {tlCommit[:7], "docs/streams"}, {strings.Repeat("AB", 20), "docs/streams"},
			{tlCommit, ""}, {tlCommit, "/docs"}, {tlCommit, "docs/../x"}, {tlCommit, "."}, {tlCommit, "docs/streams/"},
		} {
			if _, err := ReadTreeListing(g, repo, a.commit, a.dir); err == nil || !IsUnverifiable(err) {
				t.Fatalf("%q %q: got %v, want could-not-check", a.commit, a.dir, err)
			}
		}
		if len(*paths) != 0 {
			t.Fatalf("requests = %v, want none for refused arguments", *paths)
		}
	})
	t.Run("a non-GitHub backend", func(t *testing.T) {
		if _, err := ReadTreeListing(&GitLabForge{}, repo, tlCommit, "docs/streams"); err == nil || !IsUnverifiable(err) {
			t.Fatalf("got %v, want could-not-check", err)
		}
	})
}

// A landing's forge is the backend inside the outbound check and the evidence-landing read
// scope (EvidenceLandingForge). A backend-only read made on it reaches the backend: the tree
// listing on GitHub, and the descendant read on GitLab, are served through both wrappers.
// regression: #5913
func TestBackendReadsThroughLanding(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "r"}
	const file = "docs/streams/s/README.md"
	t.Run("GitHub tree listing", func(t *testing.T) {
		g, _ := tlServer(t, map[string]any{"__typename": "Commit", "oid": tlCommit}, nil, false, nil)
		landing := EvidenceLandingForge(OutboundChecked(g, "verifier"), repo, file, "main")
		if _, ok := landing.(*outboundForge); !ok {
			t.Fatalf("landing forge is %T, want the outbound-checked landing scope", landing)
		}
		if got, err := ReadTreeListing(landing, repo, tlCommit, "docs/streams"); err != nil || got == nil {
			t.Fatalf("tree listing through the landing forge: %+v, %v; want the backend's (empty) listing", got, err)
		}
	})
	t.Run("GitLab descendant read", func(t *testing.T) {
		landing := EvidenceLandingForge(OutboundChecked(&GitLabForge{Token: "stub"}, "verifier"), repo, file, "main")
		if !DescendantDiffServed(landing) {
			t.Fatal("the descendant read is not served through the GitLab landing forge")
		}
	})
}
