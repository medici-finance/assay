package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2414 — the new classifier input turns CREATE-PLACEHOLDER into AWAIT and
// changes no other outcome.
func TestClassifyLabelHistoryHold(t *testing.T) {
	cases := []struct {
		name string
		in   issueClassifyInput
		want string
	}{
		{"un-briefed, history hold -> await (was create)", issueClassifyInput{open: true, labelHistoryHold: true}, actAwait},
		{"un-briefed, no history hold -> create", issueClassifyInput{open: true}, actCreatePlaceholder},
		{"excluded label -> none", issueClassifyInput{open: true, excludedLabel: true, labelHistoryHold: true}, actNone},
		{"closed, no placeholder -> none", issueClassifyInput{open: false, labelHistoryHold: true}, actNone},
		{"placeholder, unblocked -> none", issueClassifyInput{open: true, hasPlaceholder: true, labelHistoryHold: true}, actNone},
		{"placeholder, blocked -> await", issueClassifyInput{open: true, hasPlaceholder: true, blocked: true, labelHistoryHold: true}, actAwait},
		{"placeholder, closed -> retire", issueClassifyInput{open: false, hasPlaceholder: true, labelHistoryHold: true}, actRetire},
		{"placeholder, closed, done -> none", issueClassifyInput{open: false, hasPlaceholder: true, placeholderDone: true, labelHistoryHold: true}, actNone},
		{"decision-owed fresh -> await", issueClassifyInput{open: true, decisionOwed: true, labelHistoryHold: true}, actAwait},
		{"decision-owed aged -> escalate", issueClassifyInput{open: true, decisionOwed: true, agedPastSLA: true, labelHistoryHold: true}, actEscalate},
		{"addressed -> addressed", issueClassifyInput{open: true, addressed: true, labelHistoryHold: true}, actAddressed},
		{"addressed, aged, unanswered -> escalate", issueClassifyInput{open: true, addressed: true, agedPastSLA: true, labelHistoryHold: true}, actEscalate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyIssue(c.in); got != c.want {
				t.Errorf("classifyIssue(%+v) = %s, want %s", c.in, got, c.want)
			}
			// Every outcome but the one named above is the same with the input off.
			off := c.in
			off.labelHistoryHold = false
			if c.want != actAwait || c.in.hasPlaceholder || c.in.decisionOwed {
				if got := classifyIssue(off); got != c.want {
					t.Errorf("input off: classifyIssue(%+v) = %s, want %s unchanged", off, got, c.want)
				}
			}
		})
	}
}

// regression: #2414 — the superset test itself.
func TestLabelHistoryHoldSuperset(t *testing.T) {
	ev := func(name string, removed bool) deskkit.LabelEvent {
		return deskkit.LabelEvent{Name: name, AppliedBy: "someone", AppliedByID: 5, Removed: removed, CreatedAt: "2026-09-01T00:00:00Z"}
	}
	cases := []struct {
		name         string
		h            *deskkit.LabelHistory
		held, unread bool
	}{
		{"nil history is could-not-check", nil, true, true},
		{"incomplete is could-not-check", &deskkit.LabelHistory{Complete: false}, true, true},
		{"empty complete history", &deskkit.LabelHistory{Complete: true}, false, false},
		{"only an ordinary label", &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{ev("bug", false), ev("bug", true)}}, false, false},
		{"excluded label applied", &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{ev("needs-decision", false)}}, true, false},
		{"excluded label removed", &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{ev("needs-human", false), ev("needs-human", true)}}, true, false},
		{"excluded label, other case", &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{ev("Verify-Gate", true)}}, true, false},
		{"unnamed event", &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{ev("", true)}}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			held, unread, reason := labelHistoryHold(c.h)
			if held != c.held || unread != c.unread {
				t.Fatalf("held=%t unverifiable=%t, want %t/%t", held, unread, c.held, c.unread)
			}
			if held && !strings.Contains(reason, "label history") {
				t.Errorf("reason %q should say it is about the label history", reason)
			}
			if unread && !strings.Contains(reason, "could-not-check") {
				t.Errorf("reason %q should carry the could-not-check form", reason)
			}
		})
	}
}

// regression: #2414 — end to end with the package's fake forge.
func TestBoardLabelHistoryHold(t *testing.T) {
	removed := &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{
		{Name: "needs-decision", AppliedBy: "authority", AppliedByID: 100001, CreatedAt: "2026-09-01T00:00:00Z"},
		{Name: "needs-decision", AppliedBy: "example-app", AppliedByID: 300000001, CreatedAt: "2026-09-02T00:00:00Z", Removed: true},
	}}
	issues := []deskkit.IssueSummary{
		{Number: 1, Title: "removed by an app", Author: deskkit.Account{Login: "shared-agent"}},
		{Number: 2, Title: "plain unlabelled", Author: deskkit.Account{Login: "shared-agent"}},
		{Number: 3, Title: "history unfinished", Author: deskkit.Account{Login: "shared-agent"}},
		{Number: 4, Title: "has a placeholder", Author: deskkit.Account{Login: "shared-agent"}},
		{Number: 5, Title: "excluded label now", Author: deskkit.Account{Login: "shared-agent"}, Labels: []string{"verify-gate"}},
	}
	fx := func() *repoFixture {
		return &repoFixture{
			issues: issues,
			history: map[int]*deskkit.LabelHistory{
				1: removed,
				3: {Complete: false},
				4: removed, // never read: the issue has a placeholder
			},
		}
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, issueLoopDir, "issue-4.md"), placeholderFixture(homeRepo, "todo", ""))

	calls := installForge(t, map[string]*repoFixture{homeRepo: fx()})
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "issues"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb.String())
	}
	rows := map[string]string{}
	for _, line := range strings.Split(out.String(), "\n") {
		for _, id := range []string{"#1 ", "#2 ", "#3 ", "#4 ", "#5 "} {
			if strings.Contains(line, id) {
				rows[strings.TrimSpace(id)] = line
			}
		}
	}
	if l := rows["#1"]; !strings.HasPrefix(l, "AWAIT") || !strings.Contains(l, "[label history holds an event for") {
		t.Errorf("#1 should be AWAIT with the bracketed reason, got %q", l)
	}
	if l := rows["#2"]; !strings.HasPrefix(l, "CREATE-PLACEHOLDER") || strings.Contains(l, "label history") {
		t.Errorf("#2 should stay CREATE-PLACEHOLDER, got %q", l)
	}
	if l := rows["#3"]; !strings.HasPrefix(l, "AWAIT") || !strings.Contains(l, "label history could-not-check") {
		t.Errorf("#3 should be AWAIT with the could-not-check reason, got %q", l)
	}
	if l := rows["#4"]; strings.Contains(l, "label history") {
		t.Errorf("#4 has a placeholder and must not carry a history reason, got %q", l)
	}

	var read []int
	for _, c := range *calls {
		if c.op == "IssueLabelHistory" {
			read = append(read, c.num)
		}
	}
	for _, n := range read {
		if n == 4 || n == 5 {
			t.Errorf("a label-history read was spent on #%d, which is not a would-be CREATE-PLACEHOLDER", n)
		}
	}
	if len(read) != 3 {
		t.Errorf("history reads = %v, want exactly #1 #2 #3", read)
	}

	// A failing history read fails the whole board with exit 6, as the other per-issue reads do.
	f2 := fx()
	f2.historyErr = map[int]error{2: deskkit.Unverifiable("simulated history read failure", nil)}
	installForge(t, map[string]*repoFixture{homeRepo: f2})
	out.Reset()
	errb.Reset()
	if code := run([]string{"--root", root, "issues"}, &out, &errb); code != deskkit.ExitUnverifiable {
		t.Errorf("a failing history read: exit %d, want %d; stdout=%s", code, deskkit.ExitUnverifiable, out.String())
	}
}
