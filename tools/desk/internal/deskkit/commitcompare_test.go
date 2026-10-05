package deskkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompareHistoryEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		complete      bool
	}{
		{"complete", `{"total_commits":1,"commits":[{"sha":"fix","parents":[{"sha":"base"}]}]}`, true},
		{"short", `{"total_commits":2,"commits":[{"sha":"fix","parents":[{"sha":"base"}]}]}`, false},
		{"missing count", `{"commits":[]}`, false},
		{"empty", `{"total_commits":0,"commits":[]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.payload) }))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			c, err := g.CompareRefs(ForgeRepo{Owner: "o", Name: "r"}, "base", "head")
			if err != nil {
				t.Fatal(err)
			}
			if c.CommitsComplete != tc.complete {
				t.Fatalf("complete=%v want %v", c.CommitsComplete, tc.complete)
			}
			if len(c.Commits) > 0 && (c.Commits[0].SHA != "fix" || len(c.Commits[0].Parents) != 1) {
				t.Fatalf("lost history: %+v", c)
			}
		})
	}
}
func TestCommitFileEvidence(t *testing.T) {
	for _, n := range []int{0, 1, 299, 300} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			files := make([]string, n)
			for i := range files {
				files[i] = `{"filename":"new.go","previous_filename":"old.go"}`
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"sha":"fix","parents":[{"sha":"base"}],"files":[%s]}`, strings.Join(files, ","))
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			c, err := g.GetCommit(ForgeRepo{Owner: "o", Name: "r"}, "fix")
			if err != nil {
				t.Fatal(err)
			}
			if c.FilesComplete != (n < 300) {
				t.Fatalf("file window %d complete=%v", n, c.FilesComplete)
			}
			if len(c.Files) != n {
				t.Fatalf("file count=%d want %d", len(c.Files), n)
			}
			if n > 0 && c.Files[0].PreviousFilename != "old.go" {
				t.Fatal("rename source lost")
			}
		})
	}
}
