package deskkit

import (
	"encoding/json"
	"github.com/medici-finance/assay/perfrec"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// Build provider observations from a record issued by the production fixture,
// not a parallel reimplementation of the binding/stamp predicates.
func verifierBatchNode(base *verifierForge, n, stage int) map[string]any {
	page := func(nodes any) map[string]any {
		return map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": nodes}
	}
	actor := func(login string) map[string]any {
		return map[string]any{"login": strings.TrimSuffix(login, "[bot]"), "__typename": "Bot", "databaseId": 1}
	}
	switch stage {
	case 0, 2:
		labels := []any{}
		for _, l := range base.issue.Labels {
			labels = append(labels, map[string]any{"name": l})
		}
		return map[string]any{"number": n, "title": base.issue.Title, "body": base.issue.Body, "state": "CLOSED", "url": base.issue.URL, "author": actor(base.issue.Author.Login), "labels": page(labels)}
	case 1:
		return map[string]any{"lastEditedAt": nil, "comments": page([]any{})}
	default:
		nodes := []any{}
		for _, l := range base.events {
			kind := "LabeledEvent"
			if l.Removed {
				kind = "UnlabeledEvent"
			}
			nodes = append(nodes, map[string]any{"__typename": kind, "createdAt": "2026-10-16T10:00:00Z", "actor": actor(l.AppliedBy), "label": map[string]any{"name": l.Name}})
		}
		return map[string]any{"timelineItems": page(nodes)}
	}
}

var verifierBatchAliases = regexp.MustCompile(`a([0-9]+):issue\(number:\$n([0-9]+)\)`)

func TestVerifierBatchPreservesTrustAndObservationOrder(t *testing.T) {
	root, base := verifierFixture(t, "gpt-6.1-sol")
	want, err := IssueVerifierAttestation(root, base)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		change     func(int, map[string]any)
		aliasError bool
	}{
		{name: "valid"},
		{name: "wrong-target", change: func(s int, v map[string]any) {
			if s == 0 {
				v["number"] = 999
			}
		}},
		{name: "missing-issue", change: func(s int, v map[string]any) {
			if s == 0 {
				delete(v, "number")
			}
		}},
		{name: "untrusted-author", change: func(s int, v map[string]any) {
			if s == 0 {
				v["author"] = map[string]any{"login": "stranger", "__typename": "User", "databaseId": 999}
			}
		}},
		{name: "edited-body", change: func(s int, v map[string]any) {
			if s == 1 {
				v["lastEditedAt"] = "2026-10-16T11:00:00Z"
			}
		}},
		{name: "missing-history", change: func(s int, v map[string]any) {
			if s == 1 {
				delete(v, "lastEditedAt")
			}
		}},
		{name: "omitted-comments", change: func(s int, v map[string]any) {
			if s == 1 {
				delete(v, "comments")
			}
		}},
		{name: "unknown-comment-pagination", change: func(s int, v map[string]any) {
			if s == 1 {
				delete(v["comments"].(map[string]any), "pageInfo")
			}
		}},
		{name: "incomplete-comments", change: func(s int, v map[string]any) {
			if s == 1 {
				v["comments"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
			}
		}},
		{name: "body-changes-after-history", change: func(s int, v map[string]any) {
			if s == 2 {
				v["body"] = v["body"].(string) + "edited"
			}
		}},
		{name: "title-changes-after-history", change: func(s int, v map[string]any) {
			if s == 2 {
				v["title"] = "different"
			}
		}},
		{name: "author-changes-after-history", change: func(s int, v map[string]any) {
			if s == 2 {
				v["author"] = nil
			}
		}},
		{name: "reopened-after-history", change: func(s int, v map[string]any) {
			if s == 2 {
				v["state"] = "OPEN"
			}
		}},
		{name: "unknown-state", change: func(s int, v map[string]any) {
			if s == 2 {
				v["state"] = "UNKNOWN"
			}
		}},
		{name: "labels-move-after-history", change: func(s int, v map[string]any) {
			if s == 2 {
				v["labels"].(map[string]any)["nodes"] = []any{}
			}
		}},
		{name: "missing-current-labels", change: func(s int, v map[string]any) {
			if s == 2 {
				delete(v, "labels")
			}
		}},
		{name: "omitted-label-history", change: func(s int, v map[string]any) {
			if s == 3 {
				delete(v, "timelineItems")
			}
		}},
		{name: "unknown-label-event", change: func(s int, v map[string]any) {
			if s == 3 {
				v["timelineItems"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["__typename"] = "OtherEvent"
			}
		}},
		{name: "deleted-label", change: func(s int, v map[string]any) {
			if s == 3 {
				v["timelineItems"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["label"] = nil
			}
		}},
		{name: "foreign-reapplication", change: func(s int, v map[string]any) {
			if s == 3 {
				for _, n := range v["timelineItems"].(map[string]any)["nodes"].([]any) {
					n.(map[string]any)["actor"] = map[string]any{"login": "stranger", "__typename": "User", "databaseId": 99}
				}
			}
		}},
		{name: "alias-error", aliasError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/graphql" || r.Method != "POST" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(500)
					return
				}
				var in struct {
					Query     string
					Variables map[string]any
				}
				if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
					t.Error(e)
				}
				stage := calls
				calls++
				if stage >= 4 {
					t.Error("unbounded batch")
					w.WriteHeader(500)
					return
				}
				selections := []string{verifierIssueSelection, verifierHistorySelection, verifierIssueSelection, verifierLabelsSelection}
				if !strings.Contains(in.Query, selections[stage]) {
					t.Errorf("stage %d lost observation order", stage)
				}
				data := map[string]any{}
				for _, m := range verifierBatchAliases.FindAllStringSubmatch(in.Query, -1) {
					if in.Variables["use"+m[1]] != true {
						continue
					}
					n := int(in.Variables["n"+m[1]].(float64))
					v := verifierBatchNode(base, n, stage)
					if n == 41 && tc.change != nil {
						tc.change(stage, v)
					}
					data["a"+m[1]] = v
				}
				result := map[string]any{"data": map[string]any{"repository": data}}
				if tc.aliasError && stage == 1 {
					result["errors"] = []any{map[string]any{"path": []any{"repository", "a0", "lastEditedAt"}, "message": "untrusted provider text"}}
				}
				json.NewEncoder(w).Encode(result)
			}))
			defer server.Close()
			g := &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
			got := VerifierAttestationBatchReader(OutboundChecked(g, "verifier"))(ForgeRepo{"example-org", "one"}, []int{42, 41, 41})
			if calls != 4 || len(got) != 2 {
				t.Fatalf("calls/results = %d/%d", calls, len(got))
			}
			if e := got[42].Err; e != nil {
				t.Fatalf("another alias contaminated healthy target: %v", e)
			}
			if (got[41].Err == nil) != (tc.name == "valid") {
				t.Fatalf("admission = %+v", got[41])
			}
			if tc.name == "valid" && got[41].Receipt != want {
				t.Fatal("batch changed the admitted binding")
			}
		})
	}
}

func TestVerifierBatchScopeFencesAndBounds(t *testing.T) {
	root, base := verifierFixture(t, "gpt-6.1-sol")
	if _, err := IssueVerifierAttestation(root, base); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Query     string
			Variables map[string]any
		}
		json.NewDecoder(r.Body).Decode(&in)
		stage := calls % 4
		calls++
		matches := verifierBatchAliases.FindAllStringSubmatch(in.Query, -1)
		if len(matches) != verifierAttestationBatchCap {
			t.Error("batch exceeded bound")
		}
		data := map[string]any{}
		for _, m := range matches {
			if in.Variables["use"+m[1]] != true {
				continue
			}
			n := int(in.Variables["n"+m[1]].(float64))
			data["a"+m[1]] = verifierBatchNode(base, n, stage)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": data}})
	}))
	defer server.Close()
	g := &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client(), onlyRepo: "example-org/one"}
	for _, f := range []Forge{ReadOnly(g), &GitLabForge{}, &verifierForge{}} {
		if VerifierAttestationBatchReader(f) != nil {
			t.Fatal("unsupported/read-only wrapper exposed batch capability")
		}
	}
	batch := VerifierAttestationBatchReader(g)
	large := int(^uint(0) >> 1)
	if int64(large) > 1<<31-1 {
		if r := batch(ForgeRepo{"example-org", "one"}, []int{large}); r[large].Err == nil || calls != 0 {
			t.Fatal("invalid GraphQL Int reached provider")
		}
	}
	if r := batch(ForgeRepo{"example-org", "two"}, []int{41}); r[41].Err == nil || calls != 0 {
		t.Fatal("scope mismatch reached provider")
	}
	// Check slug validation independently of the credential's repository fence.
	unscoped := &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
	if r := VerifierAttestationBatchReader(unscoped)(ForgeRepo{"bad owner", "one"}, []int{41}); !IsRefused(r[41].Err) || calls != 0 {
		t.Fatal("invalid repository slug reached provider")
	}
	var ns []int
	for n := 1; n <= 65; n++ {
		ns = append(ns, n)
	}
	results := batch(ForgeRepo{"example-org", "one"}, append(ns, 0, -1, 1))
	if calls != 12 || len(results) != 67 || results[0].Err == nil || results[-1].Err == nil {
		t.Fatalf("bounds calls/results = %d/%d", calls, len(results))
	}
	for _, n := range ns {
		if results[n].Err != nil {
			t.Fatal(results[n].Err)
		}
	}
}

func TestVerifierBatchStopsAfterTransportRefusal(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer s.Close()
	g := &GitHubForge{Token: "fixture", BaseURL: s.URL, Client: s.Client()}
	ns := []int{}
	for n := 1; n <= 65; n++ {
		ns = append(ns, n)
	}
	got := VerifierAttestationBatchReader(g)(ForgeRepo{"example-org", "one"}, ns)
	if calls != 1 || len(got) != 65 {
		t.Fatalf("continued after refusal: %d/%d", calls, len(got))
	}
	for _, n := range ns {
		if got[n].Err == nil {
			t.Fatalf("target %d became empty instead of unavailable", n)
		}
	}
}

func TestVerifierBatchMissingAliasesAndGlobalErrors(t *testing.T) {
	root, base := verifierFixture(t, "gpt-6.1-sol")
	if _, err := IssueVerifierAttestation(root, base); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing-alias", "null-alias", "missing-repository", "global-error", "unknown-error-alias"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				stage := calls
				calls++
				data := map[string]any{"a0": verifierBatchNode(base, 41, stage), "a1": verifierBatchNode(base, 42, stage)}
				env := map[string]any{"data": map[string]any{"repository": data}}
				if stage == 0 {
					switch name {
					case "missing-alias":
						delete(data, "a0")
					case "null-alias":
						data["a0"] = nil
					case "missing-repository":
						env["data"] = map[string]any{}
					case "global-error":
						env["errors"] = []any{map[string]any{"message": "untrusted-provider-text"}}
					case "unknown-error-alias":
						env["errors"] = []any{map[string]any{"path": []any{"repository", "a99"}}}
					}
				}
				json.NewEncoder(w).Encode(env)
			}))
			defer server.Close()
			g := &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
			got := VerifierAttestationBatchReader(g)(ForgeRepo{"example-org", "one"}, []int{41, 42})
			if got[41].Err == nil || strings.Contains(got[41].Err.Error(), "untrusted-provider-text") {
				t.Fatal("missing/error observation admitted or provider text leaked")
			}
			global := name != "missing-alias" && name != "null-alias"
			if global && (got[42].Err == nil || calls != 1) {
				t.Fatal("unattributable error continued or admitted another target")
			}
			if !global && (got[42].Err != nil || calls != 4) {
				t.Fatal("missing alias contaminated independent target")
			}
		})
	}
}

func TestVerifierBatchOverflowUsesCompleteNativeReads(t *testing.T) {
	root, base := verifierFixture(t, "gpt-6.1-sol")
	want, err := IssueVerifierAttestation(root, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "unavailable"}[fail], func(t *testing.T) {
			graph, rest := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/graphql" {
					stage := graph
					graph++
					v := verifierBatchNode(base, 41, stage)
					field := "labels"
					if stage == 3 {
						field = "timelineItems"
					}
					if stage != 1 {
						v[field].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"a0": v}}})
					return
				}
				rest++
				if fail {
					w.WriteHeader(500)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/timeline") {
					events := []any{}
					for _, ev := range base.events {
						kind := "labeled"
						if ev.Removed {
							kind = "unlabeled"
						}
						events = append(events, map[string]any{"event": kind, "actor": map[string]any{"login": ev.AppliedBy, "id": 1}, "label": map[string]any{"name": ev.Name}, "created_at": "2026-10-16T10:00:00Z"})
					}
					json.NewEncoder(w).Encode(events)
					return
				}
				if r.URL.Path != "/repos/example-org/one/issues/41" {
					t.Error("unexpected fallback path", r.URL)
				}
				labels := []any{}
				for _, l := range base.issue.Labels {
					labels = append(labels, map[string]any{"name": l})
				}
				json.NewEncoder(w).Encode(map[string]any{"number": 41, "title": base.issue.Title, "body": base.issue.Body, "state": "closed", "html_url": base.issue.URL, "user": map[string]any{"login": base.issue.Author.Login, "id": 1}, "labels": labels})
			}))
			defer server.Close()
			g := &GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
			got := VerifierAttestationBatchReader(g)(ForgeRepo{"example-org", "one"}, []int{41})[41]
			if (got.Err != nil) != fail {
				t.Fatalf("overflow admission %v", got.Err)
			}
			if !fail && (got.Receipt != want || graph != 4 || rest != 3) {
				t.Fatalf("overflow reads graph/rest %d/%d", graph, rest)
			}
		})
	}
}

func TestVerifierBatchPerfV3CountsAttemptsAndPoints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	session := flowSession(perfrec.ObservationProvider)
	observer := RequestObserver{Sink: session, Origin: perfrec.OriginDirect}
	leaf := observeRT(func(r *http.Request) (*http.Response, error) {
		var input struct{ Query string }
		json.NewDecoder(r.Body).Decode(&input)
		if !strings.Contains(input.Query, "_assayPerf:rateLimit{cost}") {
			t.Error("batch lost same-operation cost observation")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"repository":{"a0":null},"_assayPerf":{"cost":1}}}`))}, nil
	})
	g := &GitHubForge{Token: "fixture", Client: &http.Client{Transport: leaf}, Observer: observer}
	got := VerifierAttestationBatchReader(g)(ForgeRepo{"example-org", "one"}, []int{41})
	if got[41].Err == nil {
		t.Fatal("missing alias admitted")
	}
	summary := writeForgeRecord(t, session)
	var attempts, points int64
	for _, r := range summary.Requests {
		attempts += r.Attempts
		if r.Points.Value == nil || r.Points.Coverage != perfrec.CoverageMeasured {
			t.Fatal("batch points unavailable", r.Points)
		}
		points += *r.Points.Value
	}
	if attempts != 4 || points != 4 {
		t.Fatalf("batch attempts/points %d/%d", attempts, points)
	}
}

// A prepared reader has no full Forge to promote an unexpected method from.
// New validator dependencies must compile against this explicit read contract.
var _ verifierAttestationReader = (*verifierBatchForge)(nil)

func TestVerifierBatchAdapterReadBoundary(t *testing.T) {
	repo := ForgeRepo{"example-org", "one"}
	reader := &verifierBatchForge{repo: repo, number: 41, observation: &verifierBatchObservation{first: &Issue{Number: 41}, current: &Issue{Number: 41}}}
	if _, ok := any(reader).(Forge); ok {
		t.Fatal("prepared adapter exposed a full Forge")
	}
	for range 2 {
		if _, err := reader.GetIssueTyped(repo, 41, TargetIssue); err != nil {
			t.Fatal(err)
		}
	}
	if issue, err := reader.GetIssueTyped(repo, 41, TargetIssue); issue != nil || !IsUnverifiable(err) {
		t.Fatalf("third observation must be unavailable: %+v %v", issue, err)
	}
}
