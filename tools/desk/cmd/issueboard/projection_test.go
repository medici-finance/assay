package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func projectionFixture(t *testing.T) (string, *[]forgeCall) {
	t.Helper()
	calls := installForge(t, map[string]*repoFixture{homeRepo: {
		issues: []deskkit.IssueSummary{
			{Number: 1, Title: "ready-one", Author: deskkit.Account{Login: "shared-agent"}},
			{Number: 2, Title: "decision-two", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"question"}, CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 3, Title: "addressed-three", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"to:worker"}, CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 4, Title: "excluded-four", Author: deskkit.Account{Login: "external-user"}, Labels: []string{"review-request"}},
			{Number: 5, Title: "represented-five", Author: deskkit.Account{Login: "shared-agent"}},
			{Number: 6, Title: "help-six", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"help wanted"}},
			{Number: 7, Title: "external-seven", Author: deskkit.Account{Login: "external-user"}},
			{Number: 8, Title: "held-eight", Author: deskkit.Account{Login: "shared-agent"}},
		},
		trust: map[int]*deskkit.TrustPayload{
			2: {Complete: true}, 3: {Complete: true}, 4: {Complete: true},
			7: {Complete: true, Events: []deskkit.ContentEvent{comment("ada", 31337, "2026-01-02T00:00:00Z")}},
		},
		history: map[int]*deskkit.LabelHistory{8: {Complete: false}},
		titles:  map[int]string{9: "closed-nine"},
	}})
	root := t.TempDir()
	dir := filepath.Join(root, issueLoopDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{5, 9} {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("issue-%d.md", n)), []byte(placeholderFixture(homeRepo, "todo", "")), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root, calls
}

func projectionRun(t *testing.T, root string, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append([]string{"--root", root, "issues"}, args...), &out, &errb)
	return code, out.String() + errb.String()
}
func enrichment(calls []forgeCall) []string {
	var out []string
	for _, c := range calls {
		if c.repo == homeRepo && c.op != "ListOpenIssues" {
			out = append(out, fmt.Sprintf("%s/%d", c.op, c.num))
		}
	}
	sort.Strings(out)
	return out
}

// regression: class issue-projection
func TestProjectionJourney(t *testing.T) {
	t.Run("unknown addresses stay held", func(t *testing.T) {
		for _, labels := range [][]string{{"to:"}, {"to:worker", "to:intake"}} {
			for _, projection := range []string{"full", "intake", "work-ready"} {
				t.Run(strings.Join(labels, "+")+"/"+projection, func(t *testing.T) {
					calls := installForge(t, map[string]*repoFixture{homeRepo: {
						issues: []deskkit.IssueSummary{{Number: 81, Title: "unknown-address", Author: deskkit.Account{Login: "shared-agent"}, Labels: labels}},
					}})
					code, out := projectionRun(t, t.TempDir(), "--projection", projection)
					if code != 0 {
						t.Fatalf("exit=%d %s", code, out)
					}
					if projection == "work-ready" {
						if strings.Contains(out, "unknown-address") {
							t.Fatalf("unknown address offered for work: %s", out)
						}
					} else if !strings.Contains(out, "unknown-address") || !strings.Contains(out, "AWAIT") || strings.Contains(out, "CREATE-PLACEHOLDER") {
						t.Fatalf("unknown address not held: %s", out)
					}
					if got := enrichment(*calls); len(got) != 0 {
						t.Fatalf("unknown address paid unnecessary reads: %v", got)
					}
				})
			}
		}
	})
	for _, c := range []struct {
		p, to               string
		want, absent, reads []string
	}{
		{"full", "", []string{"ready-one", "decision-two", "addressed-three", "represented-five", "closed-nine", "external-seven"}, nil, []string{"GetIssue/9", "IssueContentEvents/2", "IssueContentEvents/3", "IssueLabelHistory/1", "IssueLabelHistory/6", "IssueLabelHistory/8", "IssueTrustEvents/4", "IssueTrustEvents/7"}},
		{"intake", "", []string{"ready-one", "decision-two", "addressed-three", "represented-five", "closed-nine", "external-seven"}, nil, []string{"GetIssue/9", "IssueContentEvents/2", "IssueContentEvents/3", "IssueLabelHistory/1", "IssueLabelHistory/6", "IssueLabelHistory/8", "IssueTrustEvents/4", "IssueTrustEvents/7"}},
		{"work-ready", "", []string{"external-seven", "empty says nothing about intake or inbox idle"}, []string{"ready-one", "held-eight", "decision-two", "addressed-three", "represented-five", "closed-nine", "excluded-four", "help-six"}, []string{"GetIssue/9", "IssueLabelHistory/1", "IssueLabelHistory/8", "IssueTrustEvents/7"}},
		{"inbox", "worker", []string{"addressed-three", "ESCALATE"}, []string{"decision-two", "ready-one", "closed-nine", "external-seven"}, []string{"IssueContentEvents/3"}},
	} {
		t.Run(c.p, func(t *testing.T) {
			root, calls := projectionFixture(t)
			args := []string{"--projection", c.p}
			if c.to != "" {
				args = append(args, "--to", c.to)
			}
			code, out := projectionRun(t, root, args...)
			if code != 0 {
				t.Fatalf("exit=%d %s", code, out)
			}
			for _, token := range c.want {
				if !strings.Contains(out, token) {
					t.Errorf("missing %q: %s", token, out)
				}
			}
			for _, token := range c.absent {
				if strings.Contains(out, token) {
					t.Errorf("unselected %q: %s", token, out)
				}
			}
			sort.Strings(c.reads)
			if got := enrichment(*calls); !reflect.DeepEqual(got, c.reads) {
				t.Errorf("enrichment got %v; want %v", got, c.reads)
			}
			if c.p != "full" && !strings.Contains(out, "enumeration=complete; freshness=this invocation") {
				t.Fatal("missing coverage")
			}
		})
	}
}

// regression: class issue-projection
func TestProjectionFailures(t *testing.T) {
	for _, args := range [][]string{{"--projection", "unknown"}, {"--projection", "scanner"}, {"--projection", "inbox"}, {"--projection", "work-ready", "--to", "worker"}} {
		root, calls := projectionFixture(t)
		code, _ := projectionRun(t, root, args...)
		if code != 5 || len(*calls) != 0 {
			t.Fatal("unsupported projection did not refuse before collection")
		}
	}
	t.Run("missing-open", func(t *testing.T) {
		root, _ := projectionFixture(t)
		previous := forgeFor
		forgeFor = func(repo string, _ ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
			f, fr, e := previous(repo)
			if repo == homeRepo {
				f.(*fakeForge).data.openUnlisted = map[int]string{9: "still-open"}
			}
			return f, fr, e
		}
		t.Cleanup(func() { forgeFor = previous })
		code, out := projectionRun(t, root, "--projection", "work-ready")
		if code != 6 || !strings.Contains(out, "listing for") || !strings.Contains(out, "partial") {
			t.Fatalf("partial listing became empty: %d %s", code, out)
		}
	})
}
