package deskkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// Fixed provider population shared by normal-reader, batch and busy-queue tests.
// It is local only; no credentials or GitHub state enter the fixture.
func pendingChangeFixture(t testing.TB, files int, delay time.Duration, edit func(int, int, map[string]any), envelope func(int, map[string]any)) (*GitHubForge, *atomic.Int64) {
	t.Helper()
	calls := new(atomic.Int64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		metadata := func(n int) map[string]any {
			return map[string]any{"number": n, "state": "OPEN", "merged": false, "headRefOid": strings.Repeat("a", 40), "baseRefName": "main", "changedFiles": files, "author": map[string]any{"login": "example-verifier", "__typename": "Bot", "databaseId": 300000005}, "headRepository": map[string]any{"nameWithOwner": "example-org/one"}, "baseRepository": map[string]any{"nameWithOwner": "example-org/one"}}
		}
		if r.URL.Path == "/graphql" {
			var in struct {
				Query     string
				Variables map[string]any
			}
			json.NewDecoder(r.Body).Decode(&in)
			stage := 0
			if strings.Contains(in.Query, "files(first:100)") {
				stage = 1
			}
			values := map[string]any{}
			for slot := range verifierAttestationBatchCap {
				if in.Variables[fmt.Sprintf("use%d", slot)] != true {
					continue
				}
				n := int(in.Variables[fmt.Sprintf("n%d", slot)].(float64))
				node := metadata(n)
				if stage == 1 {
					var nodes []any
					for i := 0; i < min(files, 100); i++ {
						nodes = append(nodes, map[string]any{"path": fmt.Sprintf("docs/brief-%d-%d.md", n, i), "changeType": "MODIFIED"})
					}
					if nodes == nil {
						nodes = []any{}
					}
					node["files"] = map[string]any{"totalCount": files, "pageInfo": map[string]any{"hasNextPage": files > 100}, "nodes": nodes}
				}
				if edit != nil {
					edit(stage, n, node)
				}
				values[fmt.Sprintf("a%d", slot)] = node
			}
			response := map[string]any{"data": map[string]any{"repository": values, "_assayPerf": map[string]any{"cost": 1}}}
			if envelope != nil {
				envelope(stage, response)
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		var n int
		fmt.Sscanf(r.URL.Path, "/repos/example-org/one/pulls/%d", &n)
		if strings.HasSuffix(r.URL.Path, "/files") {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			var out []any
			for i := (page - 1) * 100; i < min(page*100, files); i++ {
				out = append(out, map[string]any{"filename": fmt.Sprintf("docs/brief-%d-%d.md", n, i), "status": "modified"})
			}
			if out == nil {
				out = []any{}
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"number": n, "state": "open", "merged": false, "changed_files": files, "user": map[string]any{"login": "example-verifier[bot]", "id": 300000005}, "head": map[string]any{"sha": strings.Repeat("a", 40), "repo": map[string]any{"full_name": "example-org/one"}}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": "example-org/one"}}})
	}))
	t.Cleanup(server.Close)
	return &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client(), onlyRepo: "example-org/one"}, calls
}

func TestPendingChangeBatchEquivalentAndBounded(t *testing.T) {
	repo := ForgeRepo{"example-org", "one"}
	for _, population := range []int{0, 1, 32, 65} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			g, calls := pendingChangeFixture(t, 2, 0, nil, nil)
			var numbers []int
			for n := 1; n <= population; n++ {
				numbers = append(numbers, n)
			}
			got := PendingChangeBatchReader(g)(repo, numbers)
			if calls.Load() != int64(2*((population+31)/32)) {
				t.Fatalf("unbounded request count %d", calls.Load())
			}
			for _, n := range numbers {
				pr, files, err := CompleteChangedFilesOf(g, repo, n)
				ob := got[n]
				if err != nil || ob.Err != nil || !reflect.DeepEqual(pr, ob.PR) || !reflect.DeepEqual(files, ob.Files) {
					t.Fatalf("changed native meaning: n=%d %v %v\n%+v\n%+v", n, err, ob.Err, pr, ob.PR)
				}
			}
		})
	}
}
func TestPendingChangeBatchUncertainty(t *testing.T) {
	cases := []struct {
		name string
		edit func(int, int, map[string]any)
	}{
		{"missing-number", func(s, n int, v map[string]any) { delete(v, "number") }},
		{"missing-merged", func(s, n int, v map[string]any) { delete(v, "merged") }},
		{"wrong-number", func(s, n int, v map[string]any) { v["number"] = 999 }},
		{"missing-count", func(s, n int, v map[string]any) { delete(v, "changedFiles") }},
		{"moved-head", func(s, n int, v map[string]any) {
			if s == 1 {
				v["headRefOid"] = strings.Repeat("b", 40)
			}
		}},
		{"count-drift", func(s, n int, v map[string]any) {
			if s == 1 {
				v["changedFiles"] = 3
			}
		}},
		{"connection-count", func(s, n int, v map[string]any) {
			if s == 1 {
				v["files"].(map[string]any)["totalCount"] = 3
			}
		}},
		{"missing-page-proof", func(s, n int, v map[string]any) {
			if s == 1 {
				delete(v["files"].(map[string]any), "pageInfo")
			}
		}},
		{"null-nodes", func(s, n int, v map[string]any) {
			if s == 1 {
				v["files"].(map[string]any)["nodes"] = nil
			}
		}},
		{"missing-file", func(s, n int, v map[string]any) {
			if s == 1 {
				v["files"].(map[string]any)["nodes"] = []any{}
			}
		}},
		{"duplicate-file", func(s, n int, v map[string]any) {
			if s == 1 {
				x := v["files"].(map[string]any)["nodes"].([]any)
				x[1] = x[0]
			}
		}},
		{"unknown-kind", func(s, n int, v map[string]any) {
			if s == 1 {
				x := v["files"].(map[string]any)["nodes"].([]any)
				x[0].(map[string]any)["changeType"] = "UNKNOWN"
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := pendingChangeFixture(t, 2, 0, func(s, n int, v map[string]any) {
				if n == 1 {
					tc.edit(s, n, v)
				}
			}, nil)
			got := PendingChangeBatchReader(g)(ForgeRepo{"example-org", "one"}, []int{1, 2})
			if got[1].Err == nil || got[2].Err != nil {
				t.Fatalf("target uncertainty lost: %+v", got)
			}
		})
	}
}
func TestPendingChangeBatchErrorsAndScope(t *testing.T) {
	repo := ForgeRepo{"example-org", "one"}
	for _, kind := range []string{"alias", "missing", "global", "foreign-alias"} {
		t.Run(kind, func(t *testing.T) {
			g, calls := pendingChangeFixture(t, 2, 0, nil, func(s int, v map[string]any) {
				if s != 0 {
					return
				}
				switch kind {
				case "alias":
					v["errors"] = []any{map[string]any{"path": []any{"repository", "a0", "author"}}}
				case "missing":
					delete(v["data"].(map[string]any)["repository"].(map[string]any), "a0")
				case "global":
					v["errors"] = []any{map[string]any{}}
				case "foreign-alias":
					v["errors"] = []any{map[string]any{"path": []any{"repository", "other"}}}
				}
			})
			got := PendingChangeBatchReader(g)(repo, []int{1, 2})
			if got[1].Err == nil {
				t.Fatal("provider error became absence")
			}
			if kind == "alias" || kind == "missing" {
				if got[2].Err != nil {
					t.Fatal(got[2].Err)
				}
			} else if got[2].Err == nil || calls.Load() != 1 {
				t.Fatal("unattributed error did not stop requests")
			}
		})
	}
	g, calls := pendingChangeFixture(t, 2, 0, nil, nil)
	for _, f := range []Forge{ReadOnly(g), &GitLabForge{}, &verifierForge{}} {
		if PendingChangeBatchReader(f) != nil {
			t.Fatal("unknown/read-only forge exposed batch")
		}
	}
	for _, repo := range []ForgeRepo{{"example-org", "other"}, {"bad owner", "one"}} {
		if PendingChangeBatchReader(g)(repo, []int{1})[1].Err == nil {
			t.Fatal("scope admitted")
		}
	}
	for _, n := range []int{0, -1} {
		if PendingChangeBatchReader(g)(repo, []int{n})[n].Err == nil {
			t.Fatal("invalid number admitted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("scope/number refusal contacted provider")
	}
}
func TestPendingChangeBatchOverflowAndRemoved(t *testing.T) {
	repo := ForgeRepo{"example-org", "one"}
	g, calls := pendingChangeFixture(t, 101, 0, nil, nil)
	got := PendingChangeBatchReader(g)(repo, []int{1, 2})[1]
	if got.Err != nil || len(got.Files) != 101 || calls.Load() != 8 {
		t.Fatalf("incomplete native fallback %+v calls=%d", got, calls.Load())
	}
	g, _ = pendingChangeFixture(t, 2, 0, func(s, n int, v map[string]any) {
		if s == 1 {
			v["files"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["changeType"] = "DELETED"
		}
	}, nil)
	got = PendingChangeBatchReader(g)(repo, []int{1, 2})[1]
	if got.Err != nil || got.Files[0].Status != "removed" {
		t.Fatal("removed file became a candidate")
	}
}
func TestPendingChangeBatchPerfCost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	session := flowSession(perfrec.ObservationProvider)
	g, _ := pendingChangeFixture(t, 2, 0, nil, nil)
	base := g.Client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	// Keep the public schema/cost contract while routing the fixture locally.
	url := g.BaseURL
	g.BaseURL = ""
	g.Observer = RequestObserver{Sink: session, Origin: perfrec.OriginDirect}
	g.Client = &http.Client{Transport: observeRT(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(url, "http://")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "_assayPerf:rateLimit{cost}") {
			t.Error("cost not observed in same request")
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		return base.RoundTrip(r)
	})}
	got := PendingChangeBatchReader(g)(ForgeRepo{"example-org", "one"}, []int{1, 2})
	if got[1].Err != nil || got[2].Err != nil {
		t.Fatal(got)
	}
	var attempts, points int64
	for _, r := range writeForgeRecord(t, session).Requests {
		attempts += r.Attempts
		if r.Points.Value == nil {
			t.Fatal("points unavailable")
		}
		points += *r.Points.Value
	}
	if attempts != 2 || points != 2 {
		t.Fatalf("attempts/points %d/%d", attempts, points)
	}
}

// BenchmarkPendingChangeBusyQueue is repeatable, offline and sends through the
// production native transport/decoders/count proof. Injected 1ms service delay
// models round-trip scaling, not a promised GitHub network latency.
func BenchmarkPendingChangeBusyQueue(b *testing.B) {
	for _, population := range []int{1, 32, 65} {
		for _, batched := range []bool{false, true} {
			name := fmt.Sprintf("prs=%d/batch=%t", population, batched)
			b.Run(name, func(b *testing.B) {
				g, calls := pendingChangeFixture(b, 2, time.Millisecond, nil, nil)
				repo := ForgeRepo{"example-org", "one"}
				ns := make([]int, population)
				for i := range ns {
					ns[i] = i + 1
				}
				var want = 2 * population
				if batched {
					want = 2 * ((population + 31) / 32)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					before := calls.Load()
					if batched {
						for _, r := range PendingChangeBatchReader(g)(repo, ns) {
							if r.Err != nil {
								b.Fatal(r.Err)
							}
						}
					} else {
						for _, n := range ns {
							if _, _, e := CompleteChangedFilesOf(g, repo, n); e != nil {
								b.Fatal(e)
							}
						}
					}
					if calls.Load()-before != int64(want) {
						b.Fatal("request budget regressed")
					}
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "requests/op")
			})
		}
	}
}

func TestPendingChangeBatchStopsAndBindsFallback(t *testing.T) {
	repo := ForgeRepo{"example-org", "one"}
	t.Run("transport-refusal", func(t *testing.T) {
		g, _ := pendingChangeFixture(t, 2, 0, nil, nil)
		calls := 0
		g.Client = &http.Client{Transport: observeRT(func(request *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 403, Request: request, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		})}
		var ns []int
		for n := 1; n <= 65; n++ {
			ns = append(ns, n)
		}
		got := PendingChangeBatchReader(g)(repo, ns)
		if calls != 1 || len(got) != 65 {
			t.Fatalf("refusal continued: %d requests / %d targets", calls, len(got))
		}
		for _, r := range got {
			if r.Err == nil {
				t.Fatal("unobserved target became absence")
			}
		}
	})
	t.Run("fallback-head-moved", func(t *testing.T) {
		g, _ := pendingChangeFixture(t, 101, 0, nil, nil)
		base := g.Client.Transport
		g.Client = &http.Client{Transport: observeRT(func(r *http.Request) (*http.Response, error) {
			response, err := base.RoundTrip(r)
			if err == nil && r.Method == http.MethodGet && !strings.HasSuffix(r.URL.Path, "/files") {
				var obj map[string]any
				json.NewDecoder(response.Body).Decode(&obj)
				response.Body.Close()
				obj["head"].(map[string]any)["sha"] = strings.Repeat("b", 40)
				raw, _ := json.Marshal(obj)
				response.Body = io.NopCloser(strings.NewReader(string(raw)))
				response.ContentLength = int64(len(raw))
			}
			return response, err
		})}
		got := PendingChangeBatchReader(g)(repo, []int{1, 2})
		if got[1].Err == nil || got[2].Err == nil {
			t.Fatal("overflow fallback admitted a moved head")
		}
	})
	t.Run("metadata-target", func(t *testing.T) {
		g, _ := pendingChangeFixture(t, 2, 0, func(stage, n int, v map[string]any) {
			if stage == 0 && n == 1 {
				v["number"] = 999
			}
		}, nil)
		if PendingChangeBatchReader(g)(repo, []int{1, 2})[1].Err == nil {
			t.Fatal("foreign metadata alias admitted")
		}
	})
	t.Run("files-target", func(t *testing.T) {
		g, _ := pendingChangeFixture(t, 2, 0, func(stage, n int, v map[string]any) {
			if stage == 1 && n == 1 {
				v["number"] = 999
			}
		}, nil)
		if PendingChangeBatchReader(g)(repo, []int{1, 2})[1].Err == nil {
			t.Fatal("foreign files alias admitted")
		}
	})
}
