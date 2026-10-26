package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// briefadmission_test.go — the two brief-side holds deskdispatch takes BEFORE the claim
// (medici-finance/assay#2744): an exec-tier stricter than --tier, and a gate:human brief whose
// decision issue is open and unruled. Each refusal must leave no child process behind: the
// claim is the first child a dispatch runs, so zero calls means no claim ref exists.

// fakeDecisionForge serves one repo's issues for deskkit.BriefDecisionWait.
type fakeDecisionForge struct {
	issues map[int]*deskkit.Issue
	events map[int][]deskkit.LabelEvent
	trust  map[int]*deskkit.TrustPayload
	search map[string][]deskkit.IssueSearchResult
	reads  []string
}

func (f *fakeDecisionForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.reads = append(f.reads, "issue")
	if it, ok := f.issues[n]; ok {
		return it, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeDecisionForge) SearchIssues(_ deskkit.ForgeRepo, in deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	f.reads = append(f.reads, "search")
	key := in.Query
	if in.DecisionGateMarkers {
		key = fmt.Sprintf("%q in:body", deskkit.DecisionGateMarkerPrefix+": "+in.DecisionGateBrief)
	}
	hits := append([]deskkit.IssueSearchResult(nil), f.search[key]...)
	for i := range hits {
		if item := f.issues[hits[i].Number]; item != nil {
			hits[i].Body = item.Body
		}
	}
	return hits, nil
}

func (f *fakeDecisionForge) ListIssueLabelEvents(_ deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	return f.events[n], nil
}

func (f *fakeDecisionForge) IssueTrustEvents(_ deskkit.ForgeRepo, n int) (*deskkit.TrustPayload, error) {
	if t, ok := f.trust[n]; ok {
		return t, nil
	}
	return &deskkit.TrustPayload{Complete: true}, nil
}

// openDecision is an open decision issue labelled at 10:00, with the given comments after it.
func openDecision(n int, body string, by ...string) *fakeDecisionForge {
	f := &fakeDecisionForge{
		issues: map[int]*deskkit.Issue{n: {Number: n, State: "open", Labels: []string{deskkit.DecisionLabel}, Body: body}},
		events: map[int][]deskkit.LabelEvent{n: {{Name: deskkit.DecisionLabel, CreatedAt: "2026-10-01T10:00:00Z"}}},
		trust:  map[int]*deskkit.TrustPayload{n: {Complete: true}},
		search: map[string][]deskkit.IssueSearchResult{},
	}
	for _, who := range by {
		f.trust[n].Events = append(f.trust[n].Events, deskkit.ContentEvent{Author: who, CreatedAt: at("2026-10-02T10:00:00Z")})
	}
	return f
}

func useDecisionForge(t *testing.T, f *fakeDecisionForge) {
	t.Helper()
	old := decisionWaitForgeFn
	decisionWaitForgeFn = func(repo string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
		fr, err := forgeRepoOf(repo)
		return f, fr, err
	}
	t.Cleanup(func() { decisionWaitForgeFn = old })
}

func plantBrief(t *testing.T, root, front string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte("---\n"+front+"---\n\n# Brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// regression: medici-finance/assay#2744
func TestExecTierStrongRefusesAny(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	plantBrief(t, root, "brief: example-stream/07\ngate: model\nexec-tier: strong\n")
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--brief", "b.md", "--tier", "any", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("an exec-tier strong brief dispatched at --tier any: err=%v, want a refusal (exit 5)", err)
	}
	for _, want := range []string{"exec-tier: strong", "--tier any"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, err.Error())
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es) — a claim may exist: %v", len(s.calls), s.calls)
	}
}

// regression: medici-finance/assay#2744
func TestUnruledDecisionHolds(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	plantBrief(t, root, "brief: example-stream/07\ngate: human\ndecision-issue: 41\n")
	useRatifier(t, true)
	useDecisionForge(t, openDecision(41, "", "someone-else"))
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--brief", "b.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a gate:human brief with an open, unruled decision was dispatched: err=%v", err)
	}
	if !strings.Contains(err.Error(), allowedRepo+"#41") {
		t.Errorf("the hold does not name the decision issue:\n%s", err.Error())
	}
	if len(s.calls) != 0 {
		t.Fatalf("the hold came after %d child process(es) — a claim may exist: %v", len(s.calls), s.calls)
	}
}

// The control: the same brief, with the ruling recorded, is accepted and claimed.
func TestRuledDecisionAccepted(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	plantBrief(t, root, "brief: example-stream/07\ngate: human\ndecision-issue: 41\n")
	useRatifier(t, true)
	useDecisionForge(t, openDecision(41, "", gateRatifier))
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: "decision-issue.sh ensure", stdout: "exists: #41 (OPEN)"})

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--brief", "b.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err != nil {
		t.Fatalf("a gate:human brief whose ruling is recorded was not accepted: %v", err)
	}
	if !s.ran("dispatch-claim.sh acquire") {
		t.Error("the accepted dispatch took no claim")
	}
}

// The second way a brief names its decision: no `decision-issue:` line, only the issue
// carrying its hidden decision-gate marker. A lower-numbered search hit whose body marker
// names a different brief is skipped.
// regression: medici-finance/assay#2744
func TestMarkerDecisionHolds(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	plantBrief(t, root, "brief: example-stream/07\ngate: human\n")
	useRatifier(t, true)
	f := openDecision(52, "text\n<!-- decision-gate: example-stream/07 -->\n")
	f.issues[50] = &deskkit.Issue{Number: 50, State: "open", Body: "<!-- decision-gate: example-stream/070 -->"}
	f.search[`"decision-gate: example-stream/07" in:body`] = []deskkit.IssueSearchResult{{Number: 52, State: "open"}, {Number: 50, State: "open"}}
	useDecisionForge(t, f)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--brief", "b.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil || !strings.Contains(err.Error(), allowedRepo+"#52") {
		t.Fatalf("the marker-named open decision did not hold the dispatch: %v", err)
	}
	if len(s.calls) != 0 {
		t.Fatalf("the hold came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// A decision read that fails never holds.
func TestDecisionReadFailProceeds(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	plantBrief(t, root, "brief: example-stream/07\ngate: human\ndecision-issue: 41\n")
	useRatifier(t, true)
	useDecisionForge(t, &fakeDecisionForge{}) // issue 41 cannot be read
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: "decision-issue.sh ensure", stdout: "exists: #41 (OPEN)"})

	var err error
	errOut := captureStderr(t, func() {
		err = cmdDispatch([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
			"--brief", "b.md", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	})
	if err != nil {
		t.Fatalf("an unreadable decision issue held the dispatch: %v", err)
	}
	if !strings.Contains(errOut, "decision hold:") {
		t.Errorf("the failed read was not noted on stderr:\n%s", errOut)
	}
}

// The main() wiring: the shipped binary reads the decision; nothing outside main() sets it.
func TestDecisionForgeWiredInMain(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\tdecisionWaitForgeFn = liveDecisionWaitForge\n") {
		t.Error("main() does not wire decisionWaitForgeFn = liveDecisionWaitForge: the shipped binary would never hold")
	}
	if decisionWaitForgeFn != nil {
		t.Error("decisionWaitForgeFn is set outside main(): tests that do not ask for it would read a forge")
	}
}

// TestBriefGateSecondDocument pins that a human gate in a second YAML document
// inside the frontmatter fence still fires the decision gate: the block is
// unreadable, and unreadable metadata gates human.
func TestBriefGateSecondDocument(t *testing.T) {
	root := t.TempDir()
	for i, content := range []string{
		"---\neffort: S\n...\ngate: human\n---\nbody\n",
		"---\ngate: model\n--- {gate: human}\n---\nbody\n",
		"---\ngate: model\n\ufeff--- {gate: human}\n---\nbody\n",
		"---\ngate: model\n\u2800--- {gate: human}\n---\nbody\n",
		"---\ngate: model\nGate: human\n---\nbody\n",
	} {
		name := fmt.Sprintf("brief-%02d.md", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if !briefGatesHuman(root, name) {
			t.Errorf("second-document human gate dropped: %q read as gate %q", content, briefFrontmatterGate(root, name))
		}
	}
}

// TestBriefGateContinuationLines pins that a gate or exec-tier line the shared
// parser reads as text inside another value makes the dispatcher read the
// brief as human-gated.
func TestBriefGateContinuationLines(t *testing.T) {
	root := t.TempDir()
	for i, hidden := range continuationLines {
		name := fmt.Sprintf("brief-%02d.md", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("---\n"+hidden+"\n---\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !briefGatesHuman(root, name) {
			t.Errorf("continuation line %q read as gate %q", hidden, briefFrontmatterGate(root, name))
		}
	}
}

// continuationLines hide a column-0 policy line inside a multi-line value
// opened on an earlier line: the shared parser reads the line as text in that
// value, while a first-match line reader reads it as the key.
var continuationLines = []string{
	"meta: {a: 1,\ngate: human\n}\ngate: model",
	"meta: {a:1,\ngate: human\n}",
	"tags: [a,\ngate: human\n]\ngate: model",
	"tags: [a,\nexec-tier: strong]\nexec-tier: any",
	"title: \"first\ngate: human\n  end\"\ngate: model",
	"title: 'first\nexec-tier: strong\n  end'",
	"meta: {a: 1,\ndecision-issue: 12\n}",
}

// TestDeliverableUnreadableFrontmatter pins that a brief whose frontmatter does
// not parse is a could-not-check for the deliverable declarations, never "no
// declaration": a refused block could still carry deliverable_repo, homed-in or
// a brief-v2 id.
func TestDeliverableUnreadableFrontmatter(t *testing.T) {
	root := t.TempDir()
	for i, content := range []string{
		"---\ngate: model\ndeliverable_repo: example-tool\nmeta:\n  customer: yes\n---\nbody\n",
		"---\ngate: model\n...\nhomed-in: example-org/console\n---\nbody\n",
	} {
		p := filepath.Join(root, fmt.Sprintf("brief-%02d.md", i))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		d, err := resolveDeliverable(dispatchOpts{brief: p, root: root})
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(fmt.Sprint(err), "frontmatter") {
			t.Errorf("unreadable frontmatter read as no declaration: %q gave %+v, exit %d: %v", content, d, deskkit.ExitCodeOf(err), err)
		}
	}
	// Control: a brief with readable frontmatter and no declaration stays inactive.
	p := filepath.Join(root, "brief-plain.md")
	if err := os.WriteFile(p, []byte("---\ngate: model\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d, err := resolveDeliverable(dispatchOpts{brief: p, root: root}); err != nil || d.active {
		t.Errorf("readable brief with no declaration: %+v, %v", d, err)
	}
}

// TestDeliverableIgnoresSiblingKeys pins that a quoted key which merely starts with
// "deliverable_repo:", "homed-in:" or "brief:" is not read as that declaration: a brief with only
// such keys declares nothing and stays inactive.
func TestDeliverableIgnoresSiblingKeys(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "brief-decoy.md")
	content := "---\ngate: model\n'deliverable_repo: example-tool': x\n'homed-in: example-org/console': x\n" +
		"'brief: c:other:s:01': x\n---\nbody\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if d, err := resolveDeliverable(dispatchOpts{brief: p, root: root}); err != nil || d.active {
		t.Errorf("sibling keys read as a declaration: %+v, %v", d, err)
	}
}
