package forgeread

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// regression: #2395
func TestOfflineReaderNeverAnswersEmpty(t *testing.T) {
	res, err := OfflineReader{}.OpenIssues(context.Background(), []string{"o/a", "o/b", "o/a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("offline reader carries data entries %v — an unread repo must have NO entry", res.Issues)
	}
	if len(res.Unavailable) != 2 {
		t.Fatalf("unavailable = %v, want each distinct repo once", res.Unavailable)
	}
	for _, u := range res.Unavailable {
		if u.Reason == "" || u.Class != ClassOffline {
			t.Errorf("%+v: want a reason and ClassOffline", u)
		}
	}
}

// regression: #2395
func TestFrozenReaderReconcilesAgainstTheRequest(t *testing.T) {
	snap := NewResult()
	snap.Issues["o/a"] = []Issue{{Number: 1, State: "open"}}
	snap.Issues["o/empty"] = nil
	snap.Unavailable = []Unavailable{{Repo: "o/down", Reason: "503"}}
	fr := NewFrozenReader(snap)
	// The caller mutating its own value afterwards must not change the snapshot.
	snap.Issues["o/a"][0].Number = 99
	res, err := fr.OpenIssues(context.Background(), []string{"o/a", "o/empty", "o/down", "o/never"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Issues["o/a"]; len(got) != 1 || got[0].Number != 1 {
		t.Errorf("o/a = %+v, want the frozen value", got)
	}
	if v, ok := res.Issues["o/empty"]; !ok || v == nil || len(v) != 0 {
		t.Errorf("o/empty = %#v (present %v), want known-empty non-nil slice", v, ok)
	}
	classes := map[string]Class{}
	for _, u := range res.Unavailable {
		classes[u.Repo] = u.Class
	}
	if classes["o/down"] != ClassUnreadable || classes["o/never"] != ClassMissing || len(classes) != 2 {
		t.Errorf("unavailable classes = %v", classes)
	}
}

func TestReconcileEveryRepoExactlyOnce(t *testing.T) {
	in := NewResult()
	in.Issues["o/both"] = []Issue{{Number: 1}}
	in.Issues["o/stranger"] = []Issue{{Number: 2}}
	in.Unavailable = []Unavailable{
		{Repo: "o/both", Reason: "first"}, {Repo: "o/both", Reason: "second"},
		{Repo: "o/stranger2", Reason: "x"},
	}
	res := Reconcile([]string{"o/both", "o/ok", "o/lost"}, in, "nothing")
	if len(res.Issues)+len(res.Unavailable) != 3 {
		t.Fatalf("result covers %d repos, want 3: %+v", len(res.Issues)+len(res.Unavailable), res)
	}
	if _, bad := res.Issues["o/both"]; bad {
		t.Error("a repo reported as both read and unavailable must be unavailable, never data")
	}
	for _, u := range res.Unavailable {
		if u.Repo == "o/both" && u.Reason != "first" {
			t.Errorf("a repeated unavailable entry must keep the first reason, got %q", u.Reason)
		}
	}
	if _, ok := res.Issues["o/stranger"]; ok {
		t.Error("an entry for a repo nobody asked for must be dropped")
	}
}

func envelope(schema int, kind string, body string) []byte {
	return []byte(`{"schema":` + strconv.Itoa(schema) + `,"kind":"` + kind + `",` + body + `}`)
}

// regression: #2395
func TestParseIssuesEnvelopeFailsClosedOnUnknownSchema(t *testing.T) {
	for _, schema := range []int{0, 2, -1} {
		_, err := ParseIssuesEnvelope(envelope(schema, "issues", `"repos":[],"partial":[]`), []string{"o/a"})
		if !errors.Is(err, ErrUnsupportedSchema) {
			t.Errorf("schema %d: err = %v, want ErrUnsupportedSchema", schema, err)
		}
	}
	if _, err := ParseIssuesEnvelope(envelope(EnvelopeSchema, "trust", `"repos":[],"partial":[]`), []string{"o/a"}); err == nil {
		t.Error("an envelope of another kind must be refused")
	}
	if _, err := ParseIssuesEnvelope([]byte(`not json`), nil); err == nil {
		t.Error("a malformed document must be an error")
	}
}

func TestParseIssuesEnvelopeStates(t *testing.T) {
	doc := envelope(EnvelopeSchema, "issues", `"repos":[
		{"repo":"o/full","issues":[{"number":3,"state":"open","createdAt":"2026-01-01T00:00:00Z"}]},
		{"repo":"o/empty","issues":[]},
		{"repo":"o/dup","issues":[]},{"repo":"o/dup","issues":[]}
	],"partial":[{"repo":"o/down","reason":"503"},{"repo":"o/cut","reason":"ceiling","class":"truncated"}],"extra":"ignored"`)
	res, err := ParseIssuesEnvelope(doc, []string{"o/full", "o/empty", "o/down", "o/cut", "o/dup", "o/silent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues["o/full"]) != 1 || res.Issues["o/full"][0].Created().IsZero() {
		t.Errorf("o/full = %+v", res.Issues["o/full"])
	}
	if v, ok := res.Issues["o/empty"]; !ok || len(v) != 0 {
		t.Errorf("known-empty o/empty lost: %v %v", v, ok)
	}
	got := map[string]Class{}
	for _, u := range res.Unavailable {
		got[u.Repo] = u.Class
	}
	want := map[string]Class{"o/down": ClassUnreadable, "o/cut": ClassTruncated, "o/dup": ClassUnreadable, "o/silent": ClassMissing}
	if len(got) != len(want) {
		t.Fatalf("unavailable = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s class = %q, want %q", k, got[k], v)
		}
	}
	if len(res.Issues)+len(res.Unavailable) != 6 {
		t.Errorf("each of the 6 requested repos must appear exactly once")
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	res := NewResult()
	res.Issues["o/a"] = []Issue{{Number: 1, State: "open", Title: "t", AuthorLogin: "u", AuthorID: 5, Labels: []string{"x"}}}
	res.Issues["o/e"] = []Issue{}
	res.Unavailable = []Unavailable{{Repo: "o/c", Reason: "cut", Class: ClassTruncated}}
	req := []string{"o/a", "o/e", "o/c"}
	raw, err := json.Marshal(NewIssuesEnvelope(req, res))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"issues":[]`) {
		t.Errorf("a known-empty repo must keep an explicit empty issues member: %s", raw)
	}
	back, err := ParseIssuesEnvelope(raw, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Issues) != 2 || len(back.Unavailable) != 1 || back.Unavailable[0].Class != ClassTruncated {
		t.Errorf("round trip changed the result: %+v", back)
	}
}

func TestVersionsAreSeparateContracts(t *testing.T) {
	if EnvelopeSchema != 1 || !strings.HasPrefix(APIVersion, "1.") {
		t.Errorf("contract constants moved: schema %d api %s — bump deliberately, and update the docs", EnvelopeSchema, APIVersion)
	}
}

// TestRootImportsNothingOnline pins the offline/frozen surface: the root package's own sources
// import no subpackage, no net/http and no os/exec.
func TestRootImportsNothingOnline(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	if len(files) == 0 {
		t.Fatal("no sources found")
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p := strings.Trim(im.Path.Value, `"`)
			if p == "net/http" || p == "os/exec" || strings.Contains(p, "forgeread/") {
				t.Errorf("%s imports %q — the root package is the offline/frozen surface", f, p)
			}
		}
	}
}
