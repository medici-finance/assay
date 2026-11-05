package deskkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	for _, n := range []int{0, 1, 99, 100, 299, 300, 301, 2999, 3000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("per_page") != "100" {
					t.Error("file page size not explicit")
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				start := (page - 1) * 100
				end := start + 100
				if end > n {
					end = n
				}
				var files []string
				for i := start; i < end; i++ {
					files = append(files, fmt.Sprintf(`{"filename":"f%d.go","previous_filename":"old%d.go"}`, i, i))
				}
				if end < n {
					w.Header().Set("Link", `<https://untrusted.invalid/ignored>; rel="next"`)
				}
				fmt.Fprintf(w, `{"sha":"%s","parents":[{"sha":"base"}],"files":[%s]}`, strings.Repeat("a", 40), strings.Join(files, ","))
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			c, err := g.GetCommit(ForgeRepo{Owner: "o", Name: "r"}, strings.Repeat("a", 40))
			if err != nil {
				t.Fatal(err)
			}
			if c.FilesComplete != (n < 3000) {
				t.Fatalf("files=%d complete=%v", n, c.FilesComplete)
			}
			if len(c.Files) != n {
				t.Fatalf("file count=%d want %d", len(c.Files), n)
			}
			if n > 0 && c.Files[0].PreviousFilename != "old0.go" {
				t.Fatal("rename source lost")
			}
			want := n/100 + 1
			if want > 30 {
				want = 30
			}
			if calls != want {
				t.Fatalf("calls=%d want %d", calls, want)
			}
		})
	}
}
