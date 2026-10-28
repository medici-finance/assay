package deskkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func historySHA(n int) string { return fmt.Sprintf("%040x", n+1) }
func historyChunk(start, end int) []map[string]any {
	out := []map[string]any{}
	for i := start; i < end; i++ {
		out = append(out, map[string]any{"sha": historySHA(i + 1), "parents": []map[string]string{{"sha": historySHA(i)}}})
	}
	return out
}

func TestBoundedComparePagination(t *testing.T) {
	for _, n := range []int{0, 1, 99, 100, 250, 251, 999, 1000, 1001} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("per_page") != "100" {
					t.Error("page size not explicit")
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				start := (page - 1) * 100
				end := start + 100
				if end > n {
					end = n
				}
				if end < n {
					w.Header().Set("Link", `<https://untrusted.invalid/not-followed>; rel="next"`)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"total_commits": n, "status": "ahead", "ahead_by": n, "commits": historyChunk(start, end), "files": []map[string]string{{"filename": "aggregate-only"}}})
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			c, err := g.CompareRefs(ForgeRepo{Owner: "o", Name: "r"}, historySHA(0), historySHA(n))
			if err != nil {
				t.Fatal(err)
			}
			if c.CommitsComplete != (n <= 1000) {
				t.Fatalf("count=%d complete=%v", n, c.CommitsComplete)
			}
			want := n
			if want > 1000 {
				want = 100
			}
			if len(c.Commits) != want {
				t.Fatalf("commits=%d want %d", len(c.Commits), want)
			}
			wantCalls := (n + 99) / 100
			if wantCalls < 1 {
				wantCalls = 1
			}
			if wantCalls > 10 {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d want %d", calls, wantCalls)
			}
			if len(c.Files) != 1 {
				t.Fatalf("aggregate files appended from later pages: %+v", c.Files)
			}
			if n > 0 && (len(c.Commits[0].Parents) != 1 || c.Commits[0].Parents[0] != historySHA(0)) {
				t.Fatal("parents lost")
			}
		})
	}
}

func TestComparePaginationUncertainty(t *testing.T) {
	for _, mode := range []string{"missing count", "missing commits", "negative count", "short terminal", "duplicate", "missing identity", "count drift", "status drift", "ahead drift", "behind drift", "overrun", "empty next", "http error", "malformed", "cut body", "mutable refs", "missing next full page", "short next page", "extra next at count", "cap continuation", "oversized page"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				commits := historyChunk(0, 100)
				total := 101
				status := "ahead"
				ahead := 101
				behind := 0
				if page == 2 {
					commits = historyChunk(100, 101)
				}
				if page == 1 {
					w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
				}
				switch mode {
				case "missing count":
					fmt.Fprint(w, `{"commits":[]}`)
					return
				case "missing commits":
					fmt.Fprint(w, `{"total_commits":0}`)
					return
				case "negative count":
					total = -1
				case "short terminal":
					w.Header().Del("Link")
					commits = historyChunk(0, 1)
				case "duplicate":
					if page == 2 {
						commits = historyChunk(0, 1)
					}
				case "missing identity":
					if page == 2 {
						commits[0]["sha"] = ""
					}
				case "count drift":
					if page == 2 {
						total = 102
					}
				case "status drift":
					if page == 2 {
						status = "diverged"
					}
				case "ahead drift":
					if page == 2 {
						ahead = 102
					}
				case "behind drift":
					if page == 2 {
						behind = 1
					}
				case "overrun":
					if page == 2 {
						commits = historyChunk(100, 102)
					}
				case "empty next":
					if page == 2 {
						commits = historyChunk(0, 0)
						w.Header().Set("Link", `<ignored>; rel="next"`)
					}
				case "http error":
					if page == 2 {
						w.WriteHeader(502)
						return
					}
				case "malformed":
					if page == 2 {
						fmt.Fprint(w, `{"total_commits":`)
						return
					}
				case "cut body":
					if page == 2 {
						w.Header().Set("Content-Length", "500")
						fmt.Fprint(w, `{}`)
						return
					}
				case "cap continuation":
					total = 1000
					ahead = 1000
					commits = historyChunk((page-1)*100, page*100)
					w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
				case "extra next at count":
					total = 100
				case "oversized page":
					// A server ignoring per_page: page 1 alone overruns the page size.
					w.Header().Del("Link")
					total, ahead = 150, 150
					commits = historyChunk(0, 150)
					if page > 1 {
						commits = historyChunk(0, 0)
					}
				case "missing next full page":
					w.Header().Del("Link")
				case "short next page":
					if page == 1 {
						commits = historyChunk(0, 1)
					} else {
						commits = historyChunk(1, 101)
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"total_commits": total, "status": status, "ahead_by": ahead, "behind_by": behind, "commits": commits})
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			base, head := historySHA(0), historySHA(101)
			if mode == "cap continuation" {
				head = historySHA(1000)
			}
			if mode == "mutable refs" {
				head = "topic"
			}
			c, err := g.CompareRefs(ForgeRepo{Owner: "o", Name: "r"}, base, head)
			success := mode == "missing next full page" || mode == "short next page"
			if success {
				if err != nil || !c.CommitsComplete || calls != 2 {
					t.Fatalf("complete continuation failed: %+v %v calls=%d", c, err, calls)
				}
			} else if err == nil && c.CommitsComplete {
				t.Fatalf("uncertainty blessed: %+v", c)
			}
			if (mode != "cap continuation" && calls > 2) || (mode == "cap continuation" && calls != 10) {
				t.Fatalf("unexpected extra reads: %d", calls)
			}
			if mode == "oversized page" && (err != nil || calls != 1) {
				t.Fatalf("oversized page was not stopped at page 1: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestCommitPaginationUncertainty(t *testing.T) {
	for _, mode := range []string{"missing files", "missing later files", "duplicate", "missing name", "identity drift", "parent drift", "empty next", "http error", "malformed", "cut body", "unresolved sha", "requested mismatch", "branch pinned", "short next", "oversized page"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page > 1 && mode == "branch pinned" && !strings.HasSuffix(r.URL.Path, historySHA(1)) {
					t.Error("mutable branch was read again")
				}
				files := []map[string]string{}
				start, end := 0, 100
				if page > 1 {
					start, end = 100, 101
				}
				for i := start; i < end; i++ {
					files = append(files, map[string]string{"filename": fmt.Sprintf("f%d", i), "previous_filename": fmt.Sprintf("old%d", i)})
				}
				sha := historySHA(1)
				parents := []map[string]string{{"sha": historySHA(0)}}
				if page == 1 {
					w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
				}
				switch mode {
				case "missing files":
					fmt.Fprintf(w, `{"sha":"%s"}`, sha)
					return
				case "missing later files":
					if page > 1 {
						fmt.Fprintf(w, `{"sha":"%s","parents":[{"sha":"%s"}]}`, sha, historySHA(0))
						return
					}
				case "duplicate":
					if page > 1 {
						files[0]["filename"] = "f0"
					}
				case "missing name":
					if page > 1 {
						files[0]["filename"] = ""
					}
				case "identity drift":
					if page > 1 {
						sha = historySHA(2)
					}
				case "parent drift":
					if page > 1 {
						parents = nil
					}
				case "empty next":
					if page > 1 {
						files = []map[string]string{}
						w.Header().Set("Link", `<ignored>; rel="next"`)
					}
				case "http error":
					if page > 1 {
						w.WriteHeader(502)
						return
					}
				case "malformed":
					if page > 1 {
						fmt.Fprint(w, `{"files":`)
						return
					}
				case "cut body":
					if page > 1 {
						w.Header().Set("Content-Length", "500")
						fmt.Fprint(w, `{}`)
						return
					}
				case "requested mismatch":
					sha = historySHA(4)
				case "unresolved sha":
					sha = "short"
				case "oversized page":
					// A server ignoring per_page: page 1 alone overruns the page size.
					w.Header().Del("Link")
					files = files[:0]
					if page == 1 {
						for i := 0; i < 150; i++ {
							files = append(files, map[string]string{"filename": fmt.Sprintf("f%d", i)})
						}
					}
				case "short next":
					if page == 1 {
						files = files[:1]
					} else {
						files = files[0:1]
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"sha": sha, "parents": parents, "files": files})
			}))
			defer srv.Close()
			ref := historySHA(1)
			if mode == "branch pinned" {
				ref = "topic"
			}
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			c, err := g.GetCommit(ForgeRepo{Owner: "o", Name: "r"}, ref)
			success := mode == "branch pinned" || mode == "short next"
			if success {
				if err != nil || !c.FilesComplete || calls != 2 {
					t.Fatalf("continuation failed: %+v %v calls=%d", c, err, calls)
				}
			} else if err == nil && c.FilesComplete {
				t.Fatalf("uncertainty blessed: %+v", c)
			}
			if calls > 2 {
				t.Fatalf("unexpected extra reads: %d", calls)
			}
			if mode == "oversized page" && (err != nil || calls != 1) {
				t.Fatalf("oversized page was not stopped at page 1: err=%v calls=%d", err, calls)
			}
		})
	}
}

// A metadata-only caller (author, committed date, parents, compare status) must keep
// what page 1 returned when a LATER page fails; only the completeness flag drops.
// Identity or metadata drift between pages still errors: the metadata is in doubt.
func TestLaterPageFailureKeepsMetadata(t *testing.T) {
	later := func(w http.ResponseWriter, mode string) bool {
		switch mode {
		case "http 502":
			w.WriteHeader(502)
		case "http 503":
			w.WriteHeader(503)
		case "malformed":
			fmt.Fprint(w, `{"files":`)
		case "cut body":
			w.Header().Set("Content-Length", "500")
			fmt.Fprint(w, `{}`)
		default:
			return false
		}
		return true
	}
	t.Run("commit", func(t *testing.T) {
		for _, mode := range []string{"http 502", "http 503", "malformed", "cut body", "duplicate", "missing name", "empty next", "identity drift"} {
			t.Run(mode, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					sha := historySHA(1)
					files := []map[string]string{}
					for i := 0; i < 100; i++ {
						files = append(files, map[string]string{"filename": fmt.Sprintf("f%d", i)})
					}
					if page == 1 {
						w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
					} else {
						if later(w, mode) {
							return
						}
						files = files[:1]
						switch mode {
						case "duplicate":
							files[0]["filename"] = "f0"
						case "missing name":
							files[0]["filename"] = ""
						case "empty next":
							files = files[:0]
							w.Header().Set("Link", `<ignored>; rel="next"`)
						case "identity drift":
							sha = historySHA(2)
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"sha": sha, "parents": []map[string]string{{"sha": historySHA(0)}}, "files": files,
						"author": map[string]string{"login": "alice"}, "committer": map[string]string{"login": "web-flow"},
						"commit": map[string]any{"committer": map[string]string{"date": "2026-10-01T00:00:00Z"}},
					})
				}))
				defer srv.Close()
				g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
				c, err := g.GetCommit(ForgeRepo{Owner: "o", Name: "r"}, historySHA(1))
				if mode == "identity drift" {
					if err == nil {
						t.Fatalf("identity drift between pages returned metadata: %+v", c)
					}
					return
				}
				if err != nil {
					t.Fatalf("later-page %s discarded page-1 metadata: %v", mode, err)
				}
				if c.FilesComplete {
					t.Fatalf("later-page %s blessed the file list", mode)
				}
				if c.SHA != historySHA(1) || c.AuthorLogin != "alice" || c.CommitterLogin != "web-flow" || c.CommittedDate != "2026-10-01T00:00:00Z" || len(c.Parents) != 1 || c.Parents[0] != historySHA(0) {
					t.Fatalf("page-1 metadata lost on later-page %s: %+v", mode, c)
				}
			})
		}
	})
	t.Run("compare", func(t *testing.T) {
		for _, mode := range []string{"http 502", "http 503", "malformed", "cut body", "duplicate", "missing identity", "overrun", "status drift"} {
			t.Run(mode, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					commits := historyChunk(0, 100)
					status := "ahead"
					if page == 1 {
						w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
					} else {
						if later(w, mode) {
							return
						}
						commits = historyChunk(100, 101)
						switch mode {
						case "duplicate":
							commits = historyChunk(0, 1)
						case "missing identity":
							commits[0]["sha"] = ""
						case "overrun":
							commits = historyChunk(100, 102)
						case "status drift":
							status = "diverged"
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"total_commits": 101, "status": status, "ahead_by": 101, "behind_by": 3, "commits": commits})
				}))
				defer srv.Close()
				g := &GitHubForge{Token: "stub", BaseURL: srv.URL}
				c, err := g.CompareRefs(ForgeRepo{Owner: "o", Name: "r"}, historySHA(0), historySHA(101))
				if mode == "status drift" {
					if err == nil {
						t.Fatalf("metadata drift between pages returned a comparison: %+v", c)
					}
					return
				}
				if err != nil {
					t.Fatalf("later-page %s discarded page-1 status: %v", mode, err)
				}
				if c.CommitsComplete {
					t.Fatalf("later-page %s blessed the commit list", mode)
				}
				if c.Status != "ahead" || c.AheadBy != 101 || c.BehindBy != 3 {
					t.Fatalf("page-1 status lost on later-page %s: %+v", mode, c)
				}
			})
		}
	})
}
