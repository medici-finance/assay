package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	gh "github.com/medici-finance/assay/forgeread/adapters/github"
	gl "github.com/medici-finance/assay/forgeread/adapters/gitlab"
)

var updateBound = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

type incrementalAccess struct {
	provider string
	kind     string
	pages    []string
	queries  []url.Values
	errPage  int
	endless  bool
}

func (a *incrementalAccess) GetPage(ctx context.Context, repo, rel string, q url.Values) (adapters.Page, error) {
	if err := ctx.Err(); err != nil {
		return adapters.Page{}, err
	}
	if repo != "o/r" {
		return adapters.Page{}, fmt.Errorf("wrong repository")
	}
	wantRoute := "issues"
	if a.kind == "changes" {
		wantRoute = "pulls"
		if a.provider == "gitlab" {
			wantRoute = "merge_requests"
		}
	}
	if rel != wantRoute {
		return adapters.Page{}, fmt.Errorf("wrong route %q", rel)
	}
	a.queries = append(a.queries, q)
	p, err := strconv.Atoi(q.Get("page"))
	if err != nil || p != len(a.queries) {
		return adapters.Page{}, fmt.Errorf("invalid page sequence")
	}
	if p == a.errPage {
		return adapters.Page{}, errors.New("provider permission denied")
	}
	if a.endless {
		return adapters.Page{Body: []byte("[]"), More: true}, nil
	}
	if p > len(a.pages) {
		return adapters.Page{}, fmt.Errorf("read beyond fixture")
	}
	return adapters.Page{Body: []byte(a.pages[p-1]), More: p < len(a.pages)}, nil
}

func updateRow(provider, kind string, n int, stamp string) string {
	if provider == "github" {
		extra := ""
		if kind == "changes" {
			extra = `,"head":{"sha":"head","ref":"topic","repo":{"full_name":"fork/r"}},"base":{"sha":"base","ref":"main","repo":{"full_name":"o/r"}},"requested_reviewers":[{"login":"reviewer","id":8,"type":"User"}]`
		}
		return fmt.Sprintf(`{"number":%d,"title":"record","body":"body","state":"open","updated_at":%q,"user":{"login":"author","id":7,"type":"Bot"},"labels":[{"name":"label"}]%s}`, n, stamp, extra)
	}
	extra := ""
	if kind == "changes" {
		extra = `,"sha":"head","source_branch":"topic","target_branch":"main","diff_refs":{"base_sha":"base"},"source_project_id":42,"reviewers":[{"username":"reviewer","id":8,"bot":false}]`
	}
	return fmt.Sprintf(`{"iid":%d,"title":"record","description":"body","state":"opened","updated_at":%q,"author":{"username":"author","id":7,"bot":true},"labels":["label"]%s}`, n, stamp, extra)
}

func readUpdates(ctx context.Context, provider, kind string, a adapters.Access, q adapters.UpdateQuery) (adapters.UpdateCoverage, []adapters.IssueUpdate, []adapters.ChangeUpdate, error) {
	if provider == "github" {
		reader := gh.New(a, nil)
		if kind == "issues" {
			r, e := reader.IssuesUpdatedSince(ctx, "o/r", q)
			return r.UpdateCoverage, r.Records, nil, e
		}
		r, e := reader.ChangesUpdatedSince(ctx, "o/r", q)
		return r.UpdateCoverage, nil, r.Records, e
	}
	reader := gl.New(a, nil)
	if kind == "issues" {
		r, e := reader.IssuesUpdatedSince(ctx, "o/r", q)
		return r.UpdateCoverage, r.Records, nil, e
	}
	r, e := reader.ChangesUpdatedSince(ctx, "o/r", q)
	return r.UpdateCoverage, nil, r.Records, e
}

// TestIncrementalPagination verifies the actual request shape and a
// three-page walk with an inclusive boundary. Ending on the first short page,
// dropping equality, filtering only open records or inventing a second read all
// disagree with the independent expected records/requests.
// regression: F-incremental-source
func TestIncrementalPagination(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		for _, kind := range []string{"issues", "changes"} {
			t.Run(provider+"/"+kind, func(t *testing.T) {
				rows := []string{updateRow(provider, kind, 3, "2026-10-01T12:00:02Z"), updateRow(provider, kind, 2, "2026-10-01T12:00:00Z"), updateRow(provider, kind, 1, "2026-10-01T11:59:59Z")}
				a := &incrementalAccess{provider: provider, kind: kind, pages: []string{"[" + rows[0] + "]", "[" + rows[1] + "]", "[" + rows[2] + "]"}}
				coverage, issues, changes, err := readUpdates(context.Background(), provider, kind, a, adapters.UpdateQuery{Since: updateBound})
				if err != nil {
					t.Fatal(err)
				}
				if coverage.Incomplete || coverage.ConditionalSupported || coverage.PageCount != 3 || len(a.queries) != 3 || !coverage.ObservedUpTo.Equal(updateBound.Add(2*time.Second)) {
					t.Fatalf("coverage/requests: %+v %v", coverage, a.queries)
				}
				var records []adapters.IssueUpdate
				if kind == "issues" {
					records = issues
				} else {
					for _, c := range changes {
						records = append(records, c.IssueUpdate)
						if c.HeadSHA != "head" || c.BaseSHA != "base" || c.Kind == "" || len(c.RequestedReviewers) != 1 {
							t.Fatalf("change mapping: %+v", c)
						}
					}
				}
				if len(records) != 2 || records[0].Number != 3 || records[1].Number != 2 {
					t.Fatalf("inclusive results: %+v", records)
				}
				for _, r := range records {
					if r.Repository != "o/r" || r.Provider != provider || r.Author.ID != 7 || r.Author.Type != "Bot" || r.Body != "body" || !r.BodyKnown || !reflect.DeepEqual(r.Labels, []string{"label"}) {
						t.Fatalf("identity/content: %+v", r)
					}
				}
				for _, q := range a.queries {
					if q.Get("state") != "all" || q.Get("per_page") != "100" {
						t.Fatalf("unbounded/lifecycle query: %v", q)
					}
					if provider == "github" {
						if q.Get("sort") != "updated" || q.Get("direction") != "desc" {
							t.Fatalf("ordering: %v", q)
						}
						if kind == "issues" && q.Get("since") != updateBound.Add(-time.Second).Format(time.RFC3339Nano) {
							t.Fatalf("bound: %v", q)
						}
						if kind == "changes" && q.Has("since") {
							t.Fatalf("unsupported pulls bound: %v", q)
						}
					} else {
						if q.Get("updated_after") != updateBound.Format(time.RFC3339Nano) || q.Get("order_by") != "updated_at" || q.Get("sort") != "desc" {
							t.Fatalf("bound/order: %v", q)
						}
					}
				}
			})
		}
	}
}

// regression: F-incremental-source
func TestIncrementalUncertainty(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		for _, kind := range []string{"issues", "changes"} {
			t.Run(provider+"/"+kind, func(t *testing.T) {
				empty := &incrementalAccess{provider: provider, kind: kind, pages: []string{"[]"}}
				ec, ei, ech, ee := readUpdates(context.Background(), provider, kind, empty, adapters.UpdateQuery{})
				if ee != nil || ec.Incomplete || ec.PageCount != 1 || len(ei)+len(ech) != 0 {
					t.Fatalf("known-empty: %+v %+v %+v %v", ec, ei, ech, ee)
				}
				for _, bad := range []string{"null", `{"message":"forbidden"}`, "[" + updateRow(provider, kind, 1, "not-a-time") + "]", "[" + updateRow(provider, kind, 1, "") + "]"} {
					a := &incrementalAccess{provider: provider, kind: kind, pages: []string{bad}}
					_, issues, changes, err := readUpdates(context.Background(), provider, kind, a, adapters.UpdateQuery{})
					if err == nil || issues != nil || changes != nil {
						t.Fatalf("malformed source became data: %q %+v %+v %v", bad, issues, changes, err)
					}
				}
				a := &incrementalAccess{provider: provider, kind: kind, pages: []string{"[" + updateRow(provider, kind, 1, "2026-10-01T12:00:00Z") + "]", "[]"}, errPage: 2}
				_, issues, changes, err := readUpdates(context.Background(), provider, kind, a, adapters.UpdateQuery{})
				if err == nil || issues != nil || changes != nil || len(a.queries) != 2 {
					t.Fatalf("page failure became a short success: %+v %+v %v", issues, changes, err)
				}
				a = &incrementalAccess{provider: provider, kind: kind, endless: true}
				coverage, _, _, err := readUpdates(context.Background(), provider, kind, a, adapters.UpdateQuery{})
				wantPages := gh.MaxPages
				if provider == "gitlab" {
					wantPages = gl.MaxPages
				}
				if err != nil || !coverage.Incomplete || coverage.PageCount != wantPages || len(a.queries) != wantPages {
					t.Fatalf("ceiling not explicit: %+v %v", coverage, err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				a = &incrementalAccess{provider: provider, kind: kind, pages: []string{"[]"}}
				_, _, _, err = readUpdates(ctx, provider, kind, a, adapters.UpdateQuery{})
				if !errors.Is(err, context.Canceled) || len(a.queries) != 0 {
					t.Fatalf("canceled read used access: %v %v", err, a.queries)
				}
			})
		}
	}
}

// regression: F-incremental-source
func TestIncrementalProviderKinds(t *testing.T) {
	ghPR := strings.TrimSuffix(updateRow("github", "changes", 7, "2026-10-01T12:00:00Z"), "}") + `,"merged_at":"2026-10-01T12:00:00Z"}`
	ghIssue := strings.Replace(updateRow("github", "issues", 8, "2026-10-01T12:00:00Z"), `"state":"open"`, `"state":"closed","state_reason":"not_planned"`, 1)
	a := &incrementalAccess{provider: "github", kind: "issues", pages: []string{"[" + ghIssue + "," + strings.TrimSuffix(ghPR, "}") + `,"pull_request":{}}]`}}
	r, err := gh.New(a, nil).IssuesUpdatedSince(context.Background(), "o/r", adapters.UpdateQuery{})
	if err != nil || len(r.Records) != 1 || r.Records[0].Number != 8 || r.Records[0].ClosureReason != "not_planned" {
		t.Fatalf("issue/PR kind confusion: %+v %v", r, err)
	}
	a = &incrementalAccess{provider: "github", kind: "changes", pages: []string{"[" + ghPR + "]"}}
	c, err := gh.New(a, nil).ChangesUpdatedSince(context.Background(), "o/r", adapters.UpdateQuery{})
	if err != nil || len(c.Records) != 1 || c.Records[0].State != "merged" {
		t.Fatalf("explicit merge: %+v %v", c, err)
	}
	wrongRepo := strings.Replace(updateRow("github", "changes", 7, "2026-10-01T12:00:00Z"), `"full_name":"o/r"`, `"full_name":"other/repo"`, 1)
	a = &incrementalAccess{provider: "github", kind: "changes", pages: []string{"[" + wrongRepo + "]"}}
	if _, err := gh.New(a, nil).ChangesUpdatedSince(context.Background(), "o/r", adapters.UpdateQuery{}); err == nil {
		t.Fatal("wrong target repository accepted")
	}
	locked := strings.Replace(updateRow("gitlab", "changes", 7, "2026-10-01T12:00:00Z"), `"state":"opened"`, `"state":"locked"`, 1)
	la := &incrementalAccess{provider: "gitlab", kind: "changes", pages: []string{"[" + locked + "]"}}
	lr, le := gl.New(la, nil).ChangesUpdatedSince(context.Background(), "o/r", adapters.UpdateQuery{})
	if le != nil || len(lr.Records) != 1 || lr.Records[0].State != "locked" || lr.Records[0].ClosureReason != "" {
		t.Fatalf("locked change lost its observed state: %+v %v", lr, le)
	}

	for _, provider := range []string{"github", "gitlab"} {
		var raw map[string]any
		if err := json.Unmarshal([]byte(updateRow(provider, "changes", 7, "2026-10-01T12:00:00Z")), &raw); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"body", "description", "labels", "requested_reviewers", "reviewers"} {
			delete(raw, field)
		}
		encoded, err := json.Marshal([]map[string]any{raw})
		if err != nil {
			t.Fatal(err)
		}
		a := &incrementalAccess{provider: provider, kind: "changes", pages: []string{string(encoded)}}
		_, _, changes, err := readUpdates(context.Background(), provider, "changes", a, adapters.UpdateQuery{})
		if err != nil || len(changes) != 1 || changes[0].BodyKnown || changes[0].Labels != nil || changes[0].RequestedReviewers != nil {
			t.Fatalf("omitted fields became known empty: %+v %v", changes, err)
		}
	}

	for _, provider := range []string{"github", "gitlab"} {
		row := updateRow(provider, "changes", 7, "2026-10-01T12:00:00Z")
		row = strings.Replace(row, `"state":"open"`, `"state":"closed"`, 1)
		row = strings.Replace(row, `"state":"opened"`, `"state":"closed"`, 1)
		a = &incrementalAccess{provider: provider, kind: "changes", pages: []string{"[" + row + "]"}}
		_, _, records, err := readUpdates(context.Background(), provider, "changes", a, adapters.UpdateQuery{})
		if err != nil || len(records) != 1 || records[0].State != "closed" || !records[0].MergedAt.IsZero() {
			t.Fatalf("closed inferred as merged: %+v %v", records, err)
		}
	}
}
