package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pfHead = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

var pfCodeFiles = []prFile{{Filename: "internal/widget.go", Status: "modified"}}

func pfClient(t *testing.T, body string, files []prFile, status int) *ghClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/30":
			json.NewEncoder(w).Encode(map[string]any{"number": 30, "body": body, "head": map[string]string{"sha": pfHead}})
		case "/repos/o/r/pulls/30/files":
			if status != 200 {
				w.WriteHeader(status)
				return
			}
			json.NewEncoder(w).Encode(files)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return &ghClient{base: srv.URL, doer: srv.Client()}
}
func pfRun(t *testing.T, root, body string, files []prFile, status int) (lifecyclePreflightResult, int) {
	t.Helper()
	return checkLifecyclePreflight(root, "o/r", 30, pfHead, pfClient(t, body, files, status), nil)
}
func TestLifecyclePreflightMissingDesign(t *testing.T) {
	root := copyDesignGateFixture(t)
	before := readFixtureReadme(t, root)
	res, code := pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
	if code != 1 || res.Passed || !res.LookedAt || len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "no design: record") {
		t.Fatalf("%d %+v", code, res)
	}
	if readFixtureReadme(t, root) != before {
		t.Fatal("board written")
	}
	_, held, err := planReconcileWrites(root, []BriefCell{{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "test"}})
	if err != nil || len(held) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), strings.Join(held[0].Problems, "\n")) {
		t.Fatalf("reconcile parity: %v %+v vs %+v", err, held, res)
	}
}
func TestLifecyclePreflightControls(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		files        []prFile
		status, code int
		class        string
	}{
		{"model", "Brief: dg/03", pfCodeFiles, 200, 0, "delivery"}, {"authors", "Authors: dg/05", nil, 500, 0, ""}, {"issue", "Issue: #42", nil, 500, 0, ""}, {"fenced", "```\nBrief: dg/05\n```", nil, 500, 0, ""}, {"authoring", "Brief: dg/05", []prFile{{Filename: "docs/streams/dg/brief-05-todo.md", Status: "added"}}, 200, 0, "authoring-or-evidence"}, {"files-fail", "Brief: dg/05", nil, 500, 3, ""}, {"missing-row", "Brief: dg/99", pfCodeFiles, 200, 3, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			res, code := pfRun(t, copyDesignGateFixture(t), tc.body, tc.files, tc.status)
			if code != tc.code {
				t.Fatalf("%d want%d %+v", code, tc.code, res)
			}
			if tc.class != "" && (len(res.Briefs) != 1 || res.Briefs[0].Classification != tc.class) {
				t.Fatalf("%+v", res)
			}
		})
	}
}
func TestLifecyclePreflightApprovedAndInvalidRecord(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "approved"
		if invalid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			p := filepath.Join(root, "docs/streams/dg/brief-05-todo.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			s := strings.Replace(string(raw), "gate: human", "gate: model", 1)
			s = strings.Replace(s, "sources:", "design: DR-fixture-approved\nsources:", 1)
			if err = os.WriteFile(p, []byte(s), 0644); err != nil {
				t.Fatal(err)
			}
			if invalid {
				entries, err := parseDecisionsDir(root)
				if err != nil {
					t.Fatal(err)
				}
				p = filepath.Join(root, "docs/streams/decisions", entries[0].File)
				raw, err = os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(p, []byte(strings.Replace(string(raw), "human:ada", "model:robot", 1)), 0644); err != nil {
					t.Fatal(err)
				}
			}
			res, code := pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
			want := 0
			if invalid {
				want = 1
			}
			if code != want {
				t.Fatalf("%d want%d %+v", code, want, res)
			}
		})
	}
}
func TestLifecyclePreflightHeadBinding(t *testing.T) {
	res, code := checkLifecyclePreflight(copyDesignGateFixture(t), "o/r", 30, strings.Repeat("a", 40), pfClient(t, "Brief: dg/03", pfCodeFiles, 200), nil)
	if code != 3 || res.Passed || !strings.Contains(res.Reason, "head changed") {
		t.Fatalf("%d %+v", code, res)
	}
}
func TestLifecyclePreflightPartial(t *testing.T) {
	for _, ref := range []string{"fx#30", "unknown#30"} {
		t.Run(ref, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			declareDelivery(t, root, "05", `{in: "`+ref+`", covers: partial, note: "slice"}`)
			res, code := pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
			want := 0
			if strings.HasPrefix(ref, "unknown") {
				want = 3
			}
			if code != want {
				t.Fatalf("%d want%d %+v", code, want, res)
			}
			if want == 0 && (len(res.Briefs) != 1 || res.Briefs[0].Classification != "partial") {
				t.Fatalf("%+v", res)
			}
		})
	}
}
func TestLifecyclePreflightUnruledHuman(t *testing.T) {
	root := copyDesignGateFixture(t)
	p := filepath.Join(root, "docs/streams/dg/brief-05-todo.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte(strings.Replace(string(raw), "sources:", "design: DR-fixture-approved\nsources:", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	res, code := pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
	if code != 1 || len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "decision") {
		t.Fatalf("%d %+v", code, res)
	}
}
func TestLifecyclePreflightMalformedBriefAndEmptyFiles(t *testing.T) {
	root := copyDesignGateFixture(t)
	res, code := pfRun(t, root, "Brief: dg/03", nil, 200)
	if code != 3 || res.Passed {
		t.Fatalf("emptyfiles %d %+v", code, res)
	}
	p := filepath.Join(root, "docs/streams/dg/brief-03-model-scoped.md")
	if err := os.WriteFile(p, []byte("---\nschema: brief-v1\nbrief: [\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	res, code = pfRun(t, root, "Brief: dg/03", pfCodeFiles, 200)
	if code != 3 || res.Passed {
		t.Fatalf("malformedbrief %d %+v", code, res)
	}
}

type pfRulingDoer struct {
	base   httpDoer
	status int
}

func (d pfRulingDoer) Do(r *http.Request) (*http.Response, error) {
	var body string
	if strings.HasSuffix(r.URL.Path, "/issues/comments/9001") {
		body = rlCommentJSONWith("example-human", "User", "o/r", 41, "Approved dg/05", rlPosted, rlPosted)
	} else if strings.HasSuffix(r.URL.Path, "/issues/41") {
		body = rlIssueJSON("<!-- decision-gate: dg/05 -->")
	} else {
		return d.base.Do(r)
	}
	return &http.Response{StatusCode: d.status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}
func TestLifecyclePreflightHumanRuling(t *testing.T) {
	for _, status := range []int{200, 404, 500} {
		name := http.StatusText(status)
		t.Run(name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			p := filepath.Join(root, "docs/streams/dg/brief-05-todo.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			s := strings.Replace(string(raw), "sources:", "design: DR-fixture-approved\ndecision-issue: 41\nruling: https://github.com/o/r/issues/41#issuecomment-9001\nsources:", 1)
			if err = os.WriteFile(p, []byte(s), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := scanEffectiveConfig()
			old := cfg.HumanLogins
			scanCfgValue.HumanLogins = map[string]string{"example_human": "example-human"}
			t.Cleanup(func() { scanCfgValue.HumanLogins = old })
			c := pfClient(t, "Brief: dg/05", pfCodeFiles, 200)
			c.doer = pfRulingDoer{base: c.doer, status: status}
			res, code := checkLifecyclePreflight(root, "o/r", 30, pfHead, c, nil)
			if status == 200 && (code != 0 || !res.Passed) {
				t.Fatalf("valid human approval %d %+v", code, res)
			}
			if status != 200 && (code == 0 || res.Passed) {
				t.Fatalf("invalid/unreadable human approval passed %d %+v", code, res)
			}
		})
	}
}

func TestLifecyclePreflightMalformedDeclarations(t *testing.T) {
	for _, body := range []string{"Brief:", "Brief:   ", "Brief: dg/03\nBrief: dg/05", "Brief: dg/03\nBrief: dg/03", "Brief: nonsense", "Brief: dg/03 trailing"} {
		t.Run(body, func(t *testing.T) {
			res, code := pfRun(t, copyDesignGateFixture(t), body, pfCodeFiles, 200)
			if code != 3 || res.Passed {
				t.Fatalf("%d %+v", code, res)
			}
		})
	}
}
func TestLifecyclePreflightPartialRequiresReadableFiles(t *testing.T) {
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "05", `{in: "fx#30", covers: partial, note: "slice"}`)
	res, code := pfRun(t, root, "Brief: dg/05", nil, 500)
	if code != 3 || res.Passed {
		t.Fatalf("unreadable partial %d %+v", code, res)
	}
	res, code = pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
	if code != 0 || len(res.Briefs) != 1 || res.Briefs[0].To != res.Briefs[0].From {
		t.Fatalf("partial must propose no status change %d %+v", code, res)
	}
}
func TestLifecyclePreflightExistingDesignViolation(t *testing.T) {
	root := copyDesignGateFixture(t)
	res, code := pfRun(t, root, "Brief: dg/02", pfCodeFiles, 200)
	if code != 1 || len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "no design: record") {
		t.Fatalf("existing in-progress design violation must fail %d %+v", code, res)
	}
}
func TestLifecyclePreflightPreservesAdvancedStatus(t *testing.T) {
	root := copyDesignGateFixture(t)
	p := filepath.Join(root, "docs/streams/dg/README.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// dg/03 is model-scoped; an additional delivery cannot demote its done row.
	lines := strings.Split(string(raw), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "| 03 |") {
			lines[i] = strings.Replace(l, "| in-progress |", "| done |", 1)
		}
	}
	if err = os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	res, code := pfRun(t, root, "Brief: dg/03", pfCodeFiles, 200)
	if code != 0 || len(res.Briefs) != 1 || res.Briefs[0].From != "done" || res.Briefs[0].To != "done" {
		t.Fatalf("advanced status demoted %d %+v", code, res)
	}
}

// Admission must preserve every status-sensitive rule in reconcile's registry,
// including future gates; checking only today's design fields is insufficient.
func TestLifecyclePreflightHoldRegistry(t *testing.T) {
	old := statusKeyedLintRules
	statusKeyedLintRules = append(append([]statusKeyedLintRule{}, old...), statusKeyedLintRule{name: "preflight-registry-control", problems: func(e *lintEnv) []string {
		if row := e.findRow("dg", "03"); row != nil && row.Status == "implemented" {
			return []string{"registry admission control"}
		}
		return nil
	}})
	t.Cleanup(func() { statusKeyedLintRules = old })
	res, code := pfRun(t, copyDesignGateFixture(t), "Brief: dg/03", pfCodeFiles, 200)
	if code != 1 || len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "registry admission control") {
		t.Fatalf("reconcile hold bypassed %d %+v", code, res)
	}
}
func TestLifecyclePreflightExistingDanglingDesign(t *testing.T) {
	root := copyDesignGateFixture(t)
	// This problem's text is independent of Status, so reconcile's added-problem
	// diff cannot detect it. The direct affected-brief gate must still refuse it.
	res, code := pfRun(t, root, "Brief: dg/06", pfCodeFiles, 200)
	if code != 1 || len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "dereferences to no record") {
		t.Fatalf("baseline dangling design passed %d %+v", code, res)
	}
}
