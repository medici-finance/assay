package deskkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

type guardedPositiveStateForge struct {
	*GitHubForge
	guardReads int
}

func (f *guardedPositiveStateForge) GetIssue(ForgeRepo, int) (*Issue, error) {
	f.guardReads++
	return nil, Refused("unknown decorator's original read guard")
}

// regression: #4360
// regression: class issue-positive-state-batch
func TestPositiveIssueStatesScopeAndObserver(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASSAY_TELEMETRY", "1")
	session := flowSession(perfrec.ObservationProvider)
	attempts := 0
	g := &GitHubForge{Token: "fixture", onlyRepo: "example/repo", Observer: RequestObserver{Sink: session, Origin: perfrec.OriginDirect}, Client: &http.Client{Transport: observeRT(func(r *http.Request) (*http.Response, error) {
		attempts++
		if r.Method != "POST" || r.URL.Path != "/graphql" || r.Header.Get("Authorization") == "" {
			t.Fatal(r.Method, r.URL.Path, "missing authenticated transport")
		}
		var in struct {
			Query     string
			Variables map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in.Variables["owner"] != "example" || in.Variables["name"] != "repo" {
			t.Fatal(in.Variables)
		}
		rows := map[string]any{}
		for slot := 0; slot < PositiveIssueStateBatchSize; slot++ {
			if in.Variables[fmt.Sprintf("use%d", slot)] != true {
				continue
			}
			n := int(in.Variables[fmt.Sprintf("n%d", slot)].(float64))
			rows[fmt.Sprintf("a%d", slot)] = map[string]any{"number": n, "state": "CLOSED", "title": fmt.Sprintf("title-%d", n)}
		}
		raw, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": rows}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(raw))), Request: r}, nil
	})}}
	f := ReadOnly(OutboundChecked(g, "worker"))
	got, err := ReadPositiveIssueStates(f, ForgeRepo{Owner: "example", Name: "repo"}, []int{7, 8})
	if err != nil || len(got) != 2 || got[0].Number != 7 || got[1].Number != 8 || attempts != 1 {
		t.Fatal(got, err, attempts)
	}
	for _, repo := range []ForgeRepo{{Owner: "other", Name: "repo"}, {Owner: "example", Name: "repo/nested"}, {Owner: "bad owner", Name: "repo"}} {
		if out, err := ReadPositiveIssueStates(f, repo, []int{7, 8}); err == nil || out != nil || attempts != 1 {
			t.Fatal("scope was not refused before send", out, err, attempts)
		}
	}
	for _, ns := range [][]int{{0, 8}, {7, 7}, {7, 1 << 31}} {
		if out, err := ReadPositiveIssueStates(f, ForgeRepo{Owner: "example", Name: "repo"}, ns); err == nil || out != nil || attempts != 1 {
			t.Fatal(ns, out, err, attempts)
		}
	}
	record := session.Finish("issueboard", "issues", "", 0, "")
	var observed int64
	for _, request := range record.V3.Requests {
		observed += request.Attempts
	}
	if observed != int64(attempts) {
		t.Fatal("actual native attempts were not observed", observed, attempts)
	}
	unknown := &guardedPositiveStateForge{GitHubForge: g}
	if out, err := ReadPositiveIssueStates(unknown, ForgeRepo{Owner: "example", Name: "repo"}, []int{7, 8}); err == nil || out != nil || unknown.guardReads != 1 || attempts != 1 {
		t.Fatal("promoted optional method bypassed unknown decorator", out, err, unknown.guardReads, attempts)
	}
}
