package deskkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type opaqueCommentGuard struct {
	Forge
	calls int
}

func (g *opaqueCommentGuard) ListComments(ForgeRepo, int) ([]Comment, error) {
	g.calls++
	return nil, errors.New("read guard denied")
}

func TestChangeCommentTimesScope(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{}`) }))
	defer srv.Close()
	raw := &GitHubForge{Token: "fixture", BaseURL: srv.URL, onlyRepo: "example/repo"}
	f := ReadOnly(OutboundChecked(raw, "worker"))
	for _, ns := range [][]int{nil, {1, 1}, {0}, {-1}, {1, 2, 3, 4, 5, 6}, {int(1 << 31)}} {
		if _, err := ReadChangeCommentTimes(f, ForgeRepo{Owner: "example", Name: "repo"}, ns); err == nil {
			t.Fatal("invalid batch accepted", ns)
		}
	}
	if _, err := ReadChangeCommentTimes(f, ForgeRepo{Owner: "other", Name: "repo"}, []int{1, 2}); err == nil || calls != 0 {
		t.Fatal("scope/size guard sent request", err, calls)
	}
	opaque := &opaqueCommentGuard{Forge: f}
	if CanBatchChangeCommentTimes(opaque) {
		t.Fatal("opaque decorator inherited batch capability")
	}
	rows, err := ReadChangeCommentTimes(opaque, ForgeRepo{Owner: "example", Name: "repo"}, []int{1, 2})
	if err != nil || opaque.calls != 2 || rows[1].Err == nil || rows[2].Err == nil || calls != 0 {
		t.Fatal("opaque read guards bypassed", err, opaque.calls, calls)
	}
}

func TestChangeCommentTimesRows(t *testing.T) {
	for _, mode := range []string{"ok", "bot", "ghost", "null-row", "wrong-number", "missing-comments", "null-nodes", "partial-error", "root-error", "unexpected-error", "http-error", "cut-body", "oversize", "hundred"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/graphql" || r.Header.Get("Authorization") == "" {
					t.Error("custody/operation changed")
				}
				var in struct {
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					t.Error(err)
					return
				}
				if strings.Contains(in.Query, "mutation") || !strings.Contains(in.Query, "comments(first:100)") || strings.Contains(in.Query, "after:") {
					t.Error("window/operation changed")
				}
				if in.Variables["owner"] != "example" || in.Variables["name"] != "repo" || in.Variables["n0"] != float64(7) || in.Variables["n1"] != float64(8) || in.Variables["use2"] != false {
					t.Error("batch scope changed", in.Variables)
				}
				if mode == "http-error" {
					http.Error(w, "denied", 403)
					return
				}
				if mode == "cut-body" {
					fmt.Fprint(w, `{"data":`)
					return
				}
				actor := any(map[string]any{"login": "ada", "__typename": "User", "databaseId": 17})
				if mode == "bot" {
					actor = map[string]any{"login": "example-worker", "__typename": "Bot", "databaseId": 18}
				}
				if mode == "ghost" {
					actor = nil
				}
				nodes := []any{map[string]any{"createdAt": "2026-08-01T00:00:00Z", "author": actor}}
				if mode == "hundred" {
					for len(nodes) < 100 {
						nodes = append(nodes, nodes[0])
					}
				}
				if mode == "oversize" {
					for len(nodes) <= 100 {
						nodes = append(nodes, nodes[0])
					}
				}
				row := map[string]any{"number": 7, "comments": map[string]any{"nodes": nodes}}
				switch mode {
				case "wrong-number":
					row["number"] = 8
				case "missing-comments":
					delete(row, "comments")
				case "null-nodes":
					row["comments"] = map[string]any{"nodes": nil}
				}
				a0 := any(row)
				if mode == "null-row" {
					a0 = nil
				}
				payload := map[string]any{"data": map[string]any{"repository": map[string]any{"a0": a0, "a1": map[string]any{"number": 8, "comments": map[string]any{"nodes": []any{}}}}}}
				if mode == "partial-error" {
					payload["errors"] = []any{map[string]any{"message": "unavailable", "path": []any{"repository", "a0", "comments"}}}
				}
				if mode == "root-error" {
					payload["errors"] = []any{map[string]any{"message": "unavailable"}}
				}
				if mode == "unexpected-error" {
					payload["errors"] = []any{map[string]any{"message": "unavailable", "path": []any{"repository", "unrequested"}}}
				}
				json.NewEncoder(w).Encode(payload)
			}))
			defer srv.Close()
			f := ReadOnly(OutboundChecked(&GitHubForge{Token: "fixture", BaseURL: srv.URL}, "worker"))
			got, err := ReadChangeCommentTimes(f, ForgeRepo{Owner: "example", Name: "repo"}, []int{7, 8})
			if calls != 1 {
				t.Fatal("batch retry/fallback", calls)
			}
			if mode == "http-error" || mode == "cut-body" {
				if err == nil {
					t.Fatal("transport failure accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "ok" || mode == "bot" || mode == "ghost" || mode == "hundred" {
				wantCount := 1
				if mode == "hundred" {
					wantCount = 100
				}
				if got[7].Err != nil || len(got[7].Comments) != wantCount {
					t.Fatal("good activity rejected", got)
				}
				want := "ada"
				if mode == "bot" {
					want = "example-worker[bot]"
				}
				if mode == "ghost" {
					want = ""
				}
				if got[7].Comments[0].Author.Login != want {
					t.Fatal("attribution changed", got)
				}
			} else if got[7].Err == nil {
				t.Fatal("unverifiable row accepted", mode)
			}
			if mode == "root-error" || mode == "unexpected-error" {
				if got[8].Err == nil {
					t.Fatal("unlocated failure certified a row")
				}
			} else if got[8].Err != nil {
				t.Fatal("row-local failure hid another row")
			}
		})
	}
}
