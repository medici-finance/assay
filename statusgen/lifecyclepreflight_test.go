package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// pfBase commits root's current tree as the judged base (once) and returns it.
// A test that edits the fixture AFTER pfBase models a same-PR change; one that
// edits before it models a base that already carried the edit.
func pfBase(t *testing.T, root string) string {
	t.Helper()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		git("init", "-q")
		git("add", "-A")
		git("commit", "-q", "-m", "base")
		// The lint hold's decision-gate rule judges against origin/main, as the
		// workflow's fetch-depth: 0 merge checkout provides.
		git("update-ref", "refs/remotes/origin/main", "HEAD")
	}
	return git("rev-parse", "HEAD")
}
func pfRunWith(t *testing.T, root string, c *ghClient) (lifecyclePreflightResult, int) {
	t.Helper()
	return checkLifecyclePreflight(root, "o/r", 30, pfHead, pfBase(t, root), c, nil)
}
func pfRun(t *testing.T, root, body string, files []prFile, status int) (lifecyclePreflightResult, int) {
	t.Helper()
	return checkLifecyclePreflight(root, "o/r", 30, pfHead, pfBase(t, root), pfClient(t, body, files, status), nil)
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
	root := copyDesignGateFixture(t)
	res, code := checkLifecyclePreflight(root, "o/r", 30, strings.Repeat("a", 40), pfBase(t, root), pfClient(t, "Brief: dg/03", pfCodeFiles, 200), nil)
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
	typ    string // comment author type; "" is a human User
	marker string // decision-gate marker id; "" is dg/05
	text   string // comment body; "" is "Approved dg/05"
}

func (d pfRulingDoer) Do(r *http.Request) (*http.Response, error) {
	var body string
	if strings.HasSuffix(r.URL.Path, "/issues/comments/9001") {
		typ := d.typ
		if typ == "" {
			typ = "User"
		}
		text := d.text
		if text == "" {
			text = "Approved dg/05"
		}
		body = rlCommentJSONWith("example-human", typ, "o/r", 41, text, rlPosted, rlPosted)
	} else if strings.HasSuffix(r.URL.Path, "/issues/41") {
		marker := d.marker
		if marker == "" {
			marker = "dg/05"
		}
		body = rlIssueJSON("<!-- decision-gate: " + marker + " -->")
	} else {
		return d.base.Do(r)
	}
	return &http.Response{StatusCode: d.status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}
func TestLifecyclePreflightHumanRuling(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		typ    string
	}{{"OK", 200, ""}, {"Not Found", 404, ""}, {"Internal Server Error", 500, ""}, {"bot-author", 200, "Bot"}} {
		status := tc.status
		t.Run(tc.name, func(t *testing.T) {
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
			c.doer = pfRulingDoer{base: c.doer, status: status, typ: tc.typ}
			res, code := checkLifecyclePreflight(root, "o/r", 30, pfHead, pfBase(t, root), c, nil)
			if status == 200 && tc.typ == "" && (code != 0 || !res.Passed) {
				t.Fatalf("valid human approval %d %+v", code, res)
			}
			if (status != 200 || tc.typ != "") && (code == 0 || res.Passed) {
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

// pfHumanRulingFixture declares dg/05's design record and a cited human ruling
// on decision issue 41, with the roster's human login configured.
func pfHumanRulingFixture(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, "docs/streams/dg/brief-05-todo.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(raw), "sources:", "design: DR-fixture-approved\ndecision-issue: 41\nruling: https://github.com/o/r/issues/41#issuecomment-9001\nsources:", 1)
	if err = os.WriteFile(p, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
	old := scanEffectiveConfig().HumanLogins
	scanCfgValue.HumanLogins = map[string]string{"example_human": "example-human"}
	t.Cleanup(func() { scanCfgValue.HumanLogins = old })
}

// S2: the alias registry rules at the judged base. A delivery PR that adds an
// alias for its own repository cannot make an alias-form marker rule on its own
// gate: human brief; the same registry already at the base can.
func TestLifecyclePreflightAliasesReadAtBase(t *testing.T) {
	const registry = "schema: graph-repos-v1\ncell: example\nrepos:\n  wide: {cell: example, repo: o/r}\n"
	for _, inBase := range []bool{true, false} {
		name := "added-in-pr"
		if inBase {
			name = "registered-at-base"
		}
		t.Run(name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			pfHumanRulingFixture(t, root)
			write := func() {
				if err := os.WriteFile(filepath.Join(root, "docs/streams/graph-repos.yaml"), []byte(registry), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if inBase {
				write()
			}
			base := pfBase(t, root)
			if !inBase {
				write()
			}
			c := pfClient(t, "Brief: dg/05", pfCodeFiles, 200)
			c.doer = pfRulingDoer{base: c.doer, status: 200, marker: "wide:dg:05"}
			res, code := checkLifecyclePreflight(root, "o/r", 30, pfHead, base, c, nil)
			if inBase && (code != 0 || !res.Passed) {
				t.Fatalf("base-registered alias must rule %d %+v", code, res)
			}
			if !inBase && (code == 0 || res.Passed) {
				t.Fatalf("alias added in the judged PR ruled on its own brief %d %+v", code, res)
			}
		})
	}
}

// S3: the decision gate is read at the judged base, so a delivery PR that
// downgrades its own brief from gate: human to gate: model is still held.
func TestLifecyclePreflightSamePRGateDowngrade(t *testing.T) {
	root := copyDesignGateFixture(t)
	base := pfBase(t, root)
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
	res, code := checkLifecyclePreflight(root, "o/r", 30, pfHead, base, pfClient(t, "Brief: dg/05", pfCodeFiles, 200), nil)
	// In a merge checkout the lint hold's own decision-gate rule also sees the
	// relabel, so either a refusal or could-not-check is acceptable; a pass is not.
	if code == 0 || res.Passed {
		t.Fatalf("same-PR gate downgrade admitted %d %+v", code, res)
	}
}

// S4/F5: deskflip treats Cross-repo-brief: as a delivery; this check must not
// answer it with a success that evaluated nothing.
func TestLifecyclePreflightCrossRepoDeliveryCouldNotCheck(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{{"Cross-repo-brief: other/repo#dg/05", 3}, {"```\nCross-repo-brief: other/repo#dg/05\n```", 0}, {"Cross-repo-brief:", 0}} {
		t.Run(tc.body, func(t *testing.T) {
			res, code := pfRun(t, copyDesignGateFixture(t), tc.body, nil, 500)
			if code != tc.code {
				t.Fatalf("%d want%d %+v", code, tc.code, res)
			}
		})
	}
}

// S5: an unparseable authored: date cannot skip validation of a cited design record.
func TestLifecyclePreflightUnparseableAuthoredWithDesign(t *testing.T) {
	root := copyDesignGateFixture(t)
	p := filepath.Join(root, "docs/streams/dg/brief-05-todo.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(raw), "gate: human", "gate: model", 1)
	s = strings.Replace(s, "sources:", "design: DR-fixture-approved\nsources:", 1)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "authored:") {
			lines[i] = "authored: someday"
		}
	}
	if err = os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	res, code := pfRun(t, root, "Brief: dg/05", pfCodeFiles, 200)
	if code != 3 || res.Passed || !strings.Contains(res.Reason, "authored") {
		t.Fatalf("unparseable authored skipped design validation %d %+v", code, res)
	}
}

// The judged base comes from an explicit --base-rev or from a PR merge checkout
// whose second parent is the expected head; anything else is could-not-check.
func TestLifecyclePreflightBaseRev(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	base := git("rev-parse", "HEAD")
	if _, err := preflightBaseRev(root, pfHead, ""); err == nil {
		t.Fatal("non-merge checkout without --base-rev must be could-not-check")
	}
	if got, err := preflightBaseRev(root, pfHead, base); err != nil || got != base {
		t.Fatalf("explicit base %q %v", got, err)
	}
	if _, err := preflightBaseRev(root, pfHead, "no-such-rev"); err == nil {
		t.Fatal("unresolvable --base-rev accepted")
	}
	git("checkout", "-q", "-b", "pr")
	git("commit", "-q", "--allow-empty", "-m", "pr")
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	git("commit", "-q", "--allow-empty", "-m", "main moved")
	parent := git("rev-parse", "HEAD")
	git("merge", "-q", "--no-ff", "-m", "merge", "pr")
	if got, err := preflightBaseRev(root, head, ""); err != nil || got != parent {
		t.Fatalf("merge checkout base %q %v want %s", got, err, parent)
	}
	if _, err := preflightBaseRev(root, pfHead, ""); err == nil || !strings.Contains(err.Error(), "not the expected head") {
		t.Fatalf("merge of another head accepted: %v", err)
	}
}

// F4: a design record's own ruling: URL is corroborated. An unresolvable ruling
// is could-not-check (exit 3), never a soft pass; a readable ruling that does not
// stand is a lifecycle refusal naming the resolver's reason (exit 1). The brief
// is gate: model so only the record-ruling guards decide the outcome.
func TestLifecyclePreflightRecordRuling(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		typ    string
		code   int
		reason rulingReason
	}{{"human", 200, "", 0, rulingOK}, {"unresolvable", 500, "", 3, rulingUnresolvable}, {"bot-author", 200, "Bot", 1, rulingBotAuthor}, {"deleted", 404, "", 1, rulingDeleted}} {
		t.Run(tc.name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			edit := func(rel, old, new string) {
				p := filepath.Join(root, rel)
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(raw), old) {
					t.Fatalf("%s lacks %q", rel, old)
				}
				if err = os.WriteFile(p, []byte(strings.Replace(string(raw), old, new, 1)), 0644); err != nil {
					t.Fatal(err)
				}
			}
			edit("docs/streams/dg/brief-05-todo.md", "gate: human", "gate: model")
			edit("docs/streams/dg/brief-05-todo.md", "sources:", "design: DR-fixture-approved\nsources:")
			edit("docs/streams/decisions/DR-fixture-approved.md", "consequence: major", "consequence: major\nruling: https://github.com/o/r/issues/41#issuecomment-9001")
			old := scanEffectiveConfig().HumanLogins
			scanCfgValue.HumanLogins = map[string]string{"ada": "example-human"}
			t.Cleanup(func() { scanCfgValue.HumanLogins = old })
			c := pfClient(t, "Brief: dg/05", pfCodeFiles, 200)
			c.doer = pfRulingDoer{base: c.doer, status: tc.status, typ: tc.typ, text: "Approved DR-fixture-approved for dg/05"}
			res, code := pfRunWith(t, root, c)
			if code != tc.code {
				t.Fatalf("%d want%d %+v", code, tc.code, res)
			}
			switch tc.code {
			case 0:
				if !res.Passed {
					t.Fatalf("%+v", res)
				}
			case 3:
				if res.Passed || !strings.Contains(res.Reason, "DR-fixture-approved") {
					t.Fatalf("%+v", res)
				}
			case 1:
				if len(res.Briefs) != 1 || !strings.Contains(strings.Join(res.Briefs[0].Problems, "\n"), "DR-fixture-approved: ruling "+string(tc.reason)+":") {
					t.Fatalf("%+v", res)
				}
			}
		})
	}
}
