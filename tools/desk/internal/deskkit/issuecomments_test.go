package deskkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

type commentWire struct {
	t        *testing.T
	mode     string
	attempts int
	seen     map[int]int
	points   bool
	lastBody string
}

func (w *commentWire) RoundTrip(r *http.Request) (*http.Response, error) {
	w.attempts++
	if r.Method != "POST" || r.URL.Path != "/graphql" || r.Header.Get("Authorization") == "" {
		w.t.Fatal("history escaped admitted authenticated query")
	}
	if w.mode == "cancel" {
		return nil, context.Canceled
	}
	var in struct {
		Query     string
		Variables map[string]any
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.t.Fatal(err)
	}
	if strings.Contains(in.Query, "mutation") || in.Variables["owner"] != "example" || in.Variables["name"] != "repo" {
		w.t.Fatal("wrong history operation or repository")
	}
	w.points = strings.Contains(in.Query, "_assayPerf:rateLimit{cost}")
	rows := map[string]any{}
	add := func(alias string, number int, after string) {
		w.seen[number]++
		page := 1
		if after != "" {
			prefix := fmt.Sprintf("%d-page-", number)
			if !strings.HasPrefix(after, prefix) {
				w.t.Fatalf("cross-thread cursor %d %q", number, after)
			}
			var err error
			page, err = strconv.Atoi(strings.TrimPrefix(after, prefix))
			if err != nil || page < 2 {
				w.t.Fatal("wrong page cursor", after)
			}
		}
		more := false
		if w.mode == "near-limit" || w.mode == "overflow" && number == 7 {
			more = page < forgeMaxEventPages || w.mode == "overflow"
		} else if w.mode == "pages" || w.mode == "late-null" {
			more = number%2 == 1 && page == 1
		}
		cursor := ""
		if more {
			cursor = fmt.Sprintf("%d-page-%d", number, page+1)
		}
		if w.mode == "no-cursor" && number == 7 {
			more, cursor = true, ""
		}
		nodes := []any{}
		count := 1
		if w.mode == "near-limit" {
			count = 100
		}
		for i := 0; i < count; i++ {
			var actor any = map[string]any{"login": "reader", "__typename": "User", "databaseId": number * 10}
			if w.mode == "deleted" && number == 8 {
				actor = nil
			}
			nodes = append(nodes, map[string]any{"createdAt": fmt.Sprintf("2026-01-%02dT00:00:00Z", page), "author": actor})
		}
		comments := map[string]any{"pageInfo": map[string]any{"hasNextPage": more, "endCursor": cursor}, "nodes": nodes}
		row := map[string]any{"number": number, "comments": comments}
		if number == 8 {
			switch w.mode {
			case "wrong-number", "wrong-before-meta":
				row["number"] = 7
			case "missing-page":
				delete(comments, "pageInfo")
			case "missing-nodes":
				delete(comments, "nodes")
			case "null":
				rows[alias] = nil
				return
			case "missing":
				return
			}
		}
		if w.mode == "late-null" && number == 7 && page == 2 {
			rows[alias] = nil
			return
		}
		rows[alias] = row
	}
	if n, ok := in.Variables["number"].(float64); ok {
		after, _ := in.Variables["after"].(string)
		add("issue", int(n), after)
	} else {
		for slot := 0; slot < IssueCommentBatchSize; slot++ {
			if in.Variables[fmt.Sprintf("use%d", slot)] == true {
				n := int(in.Variables[fmt.Sprintf("n%d", slot)].(float64))
				after, _ := in.Variables[fmt.Sprintf("after%d", slot)].(string)
				add(fmt.Sprintf("a%d", slot), n, after)
			}
		}
	}
	env := map[string]any{"data": map[string]any{"repository": rows, "_assayPerf": map[string]any{"cost": 2}}}
	if w.mode == "data-errors" {
		env["errors"] = []any{map[string]any{"message": "second issue unavailable", "path": []any{"repository", "a1"}}}
	}
	raw, _ := json.Marshal(env)
	text := string(raw)
	if w.mode == "trailing" {
		text += `{}`
	}
	if w.mode == "truncated" {
		text = text[:len(text)-1]
	}
	if w.mode == "syntax-before-meta" {
		text = `{"data":{"repository":!,"_assayPerf":{"cost":2}}}`
	}
	if w.mode == "syntax-after-meta" {
		text = `{"data":{"_assayPerf":{"cost":2},"repository":!}}`
	}
	if w.mode == "wrong-before-meta" {
		row, _ := json.Marshal(rows)
		text = `{"data":{"repository":` + string(row) + `,"_assayPerf":{"cost":2}}}`
	}
	if w.mode == "data-null" {
		text = `{"data":null}`
	}
	if w.mode == "data-array" {
		text = `{"data":[]}`
	}
	if w.mode == "root-null" {
		text = `null`
	}
	if w.mode == "root-array" {
		text = `[]`
	}
	if w.mode == "repo-null" {
		text = `{"data":{"repository":null,"_assayPerf":{"cost":2}}}`
	}
	if w.mode == "alias-array" {
		text = `{"data":{"repository":{"a0":[],"a1":null},"_assayPerf":{"cost":2}}}`
	}
	if w.mode == "duplicate" {
		row, _ := json.Marshal(rows["a0"])
		text = strings.Replace(text, `"a0":`, `"a0":`+string(row)+`,"a0":`, 1)
	}
	var body io.Reader = strings.NewReader(text)
	if w.mode == "body-cancel" {
		body = io.MultiReader(body, commentTailError{})
	}
	w.lastBody = text
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(body), Request: r}, nil
}

type commentTailError struct{}

func (commentTailError) Read([]byte) (int, error) { return 0, context.Canceled }

type guardedCommentForge struct {
	*GitHubForge
	reads int
}

func (g *guardedCommentForge) IssueContentEvents(ForgeRepo, int) (*TrustPayload, error) {
	g.reads++
	return nil, Refused("unknown decorator read guard")
}

func makeCommentForge(t *testing.T, mode string) (*GitHubForge, *commentWire) {
	t.Helper()
	w := &commentWire{t: t, mode: mode, seen: map[int]int{}}
	return &GitHubForge{Token: "fixture", onlyRepo: "example/repo", Client: &http.Client{Transport: w}}, w
}

// regression: #4622
func TestCommentBatchScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	session := flowSession(perfrec.ObservationProvider)
	g, wire := makeCommentForge(t, "")
	g.Observer = RequestObserver{Sink: session, Origin: perfrec.OriginDirect}
	f := ReadOnly(OutboundChecked(g, "worker"))
	repo := ForgeRepo{Owner: "example", Name: "repo"}
	got, err := ReadIssueCommentEvents(f, repo, []int{7, 8})
	if err != nil || len(got) != 2 || !got[7].Complete || !got[8].Complete || wire.attempts != 1 || !wire.points {
		t.Fatal(got, err, wire.attempts, wire.points)
	}
	if got[7].Events[0].AuthorID != 70 || got[8].Events[0].AuthorID != 80 {
		t.Fatal("comment rows crossed issue identities", got)
	}
	for _, bad := range []ForgeRepo{{Owner: "other", Name: "repo"}, {Owner: "example", Name: "repo/nested"}, {Owner: "bad owner", Name: "repo"}} {
		if out, err := ReadIssueCommentEvents(f, bad, []int{7, 8}); err == nil || out != nil || wire.attempts != 1 {
			t.Fatal("scope guard did not precede the request", out, err, wire.attempts)
		}
	}
	for _, ns := range [][]int{{}, {0, 8}, {7, 7}, {7, 1 << 31}, {1, 2, 3, 4, 5, 6}} {
		if out, err := ReadIssueCommentEvents(f, repo, ns); err == nil || out != nil || wire.attempts != 1 {
			t.Fatal("identity/batch validation did not precede request", ns, out, err)
		}
	}
	unknown := &guardedCommentForge{GitHubForge: g}
	if out, err := ReadIssueCommentEvents(unknown, repo, []int{7, 8}); err == nil || out != nil || unknown.reads != 1 || wire.attempts != 1 {
		t.Fatal("optional method bypassed opaque decorator", out, err, unknown.reads, wire.attempts)
	}
	record := session.Finish("issueboard", "issues", "", 0, "")
	var attempts, points int64
	for _, request := range record.V3.Requests {
		attempts += request.Attempts
		if request.Points.Value != nil {
			points += *request.Points.Value
		}
	}
	if attempts != 1 || points != 2 {
		t.Fatal("same-query observation lost actual request/metadata", attempts, points)
	}
}

// regression: #4622
func TestCommentBatchPages(t *testing.T) {
	repo := ForgeRepo{Owner: "example", Name: "repo"}
	for _, mode := range []string{"pages", "near-limit", "overflow", "no-cursor", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			g, wire := makeCommentForge(t, mode)
			numbers := []int{7, 8, 9, 10, 11}
			want := map[int]*TrustPayload{}
			for _, n := range numbers {
				var err error
				want[n], err = g.IssueContentEvents(repo, n)
				if err != nil {
					t.Fatal(err)
				}
			}
			wire.attempts, wire.seen = 0, map[int]int{}
			got, err := ReadIssueCommentEvents(ReadOnly(OutboundChecked(g, "worker")), repo, numbers)
			if err != nil || !reflect.DeepEqual(want, got) {
				t.Fatalf("individually valid histories changed in batch: %v", err)
			}
			if mode == "near-limit" {
				if wire.attempts != forgeMaxEventPages {
					t.Fatal("near-limit histories not batched", wire.attempts)
				}
				for _, n := range numbers {
					if !got[n].Complete || len(got[n].Events) != 100*forgeMaxEventPages {
						t.Fatal("new aggregate restriction on valid legacy history", n, got[n].Complete, len(got[n].Events))
					}
				}
			}
			if mode == "overflow" && (got[7].Complete || !got[8].Complete || wire.seen[7] != forgeMaxEventPages || wire.seen[8] != 1) {
				t.Fatal("overflow or completed sibling history changed", got, wire.seen)
			}
			if mode == "no-cursor" && (got[7].Complete || wire.seen[7] != 1) {
				t.Fatal("missing cursor became complete or retried", got, wire.seen)
			}
		})
	}
}

// regression: #4622
func TestCommentBatchFaults(t *testing.T) {
	for _, mode := range []string{"wrong-number", "missing-page", "missing-nodes", "null", "missing", "late-null", "data-errors", "duplicate", "trailing", "truncated", "cancel", "body-cancel"} {
		t.Run(mode, func(t *testing.T) {
			g, wire := makeCommentForge(t, mode)
			out, err := ReadIssueCommentEvents(ReadOnly(OutboundChecked(g, "worker")), ForgeRepo{Owner: "example", Name: "repo"}, []int{7, 8})
			wantAttempts := 1
			if mode == "late-null" {
				wantAttempts = 2
			}
			if err == nil || out != nil || wire.attempts != wantAttempts {
				t.Fatal("partial/failed history admitted or retried", out, err, wire.attempts)
			}
		})
	}
}

// regression: #4622
func TestCommentBatchPoints(t *testing.T) {
	for _, c := range []struct {
		mode     string
		coverage perfrec.Coverage
	}{
		{"trailing", perfrec.CoverageFailed}, {"truncated", perfrec.CoverageFailed},
		{"syntax-before-meta", perfrec.CoverageFailed}, {"syntax-after-meta", perfrec.CoverageFailed},
		{"data-errors", perfrec.CoverageMeasured}, {"wrong-number", perfrec.CoverageMeasured},
		{"wrong-before-meta", perfrec.CoverageMeasured}, {"null", perfrec.CoverageMeasured},
		{"data-null", perfrec.CoverageUnknown}, {"data-array", perfrec.CoverageFailed},
		{"root-null", perfrec.CoverageUnknown}, {"root-array", perfrec.CoverageFailed},
		{"repo-null", perfrec.CoverageMeasured}, {"alias-array", perfrec.CoverageMeasured},
		{"cancel", perfrec.CoverageUnknown}, {"body-cancel", perfrec.CoverageUnknown},
	} {
		t.Run(c.mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("ASSAY_TELEMETRY", "1")
			session := flowSession(perfrec.ObservationProvider)
			g, wire := makeCommentForge(t, c.mode)
			g.Observer = RequestObserver{Sink: session, Origin: perfrec.OriginDirect}
			out, err := ReadIssueCommentEvents(ReadOnly(OutboundChecked(g, "worker")), ForgeRepo{Owner: "example", Name: "repo"}, []int{7, 8})
			if err == nil || out != nil {
				t.Fatal("bad business observation admitted", out, err)
			}
			record := session.Finish("issueboard", "issues", "", 6, "")
			if len(record.V3.Requests) != 1 {
				t.Fatal("missing request observation", record.V3.Requests)
			}
			points := record.V3.Requests[0].Points
			if points.Coverage != c.coverage || points.Unit != perfrec.UnitGitHubPoints {
				t.Fatalf("wire cost coverage=%s want=%s; cost=%v", points.Coverage, c.coverage, points.Value)
			}
			if c.coverage == perfrec.CoverageMeasured && (points.Value == nil || *points.Value != 2) {
				t.Fatal("valid refused query lost actual cost", points)
			}
			if c.coverage == perfrec.CoverageFailed && points.Value != nil {
				t.Fatal("malformed wire acquired measured value", points)
			}
			if c.mode != "cancel" && c.mode != "body-cancel" {
				want := decodeRequestPoints([]byte(wire.lastBody), perfrec.UnitGitHubPoints)
				if !reflect.DeepEqual(want, points) {
					t.Fatal("coverage differs from canonical whole-response decoder", want, points)
				}
			}
		})
	}
}
