package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// inbox_test.go — the addressed-inbox lane: an open issue addressed `to:worker` with no
// placeholder becomes a dispatch offer in the leading addressed lane, and every issue that
// fails a gate (trust, decision, dedupe, could-not-check) is listed and NOT offered.

const inboxTestRepo = "example-org/example"

// inboxLoop builds a hermetic loop over the given inbox source, one plain fresh board row, a
// complete PR window holding prs, and a claim reader that reports the claimed issue numbers.
func inboxLoop(t *testing.T, inbox func() ([]InboxIssue, error), prs []deskkit.PRRef, claimed ...int) *FanoutLoop {
	t.Helper()
	claims := map[int]bool{}
	for _, n := range claimed {
		claims[n] = true
	}
	return &FanoutLoop{
		Root:           t.TempDir(),
		TargetSHA:      "sha",
		DryRun:         true,
		issueBoardRepo: inboxTestRepo,
		Board:          func() ([]BoardRow, error) { return []BoardRow{briefRow("alpha", "01", "M", "", "model", false)}, nil },
		Rework:         noRework,
		Inbox:          inbox,
		issueIndex: &deskkit.IssuePRIndex{List: func(string) ([]deskkit.PRRef, bool, error) {
			return prs, false, nil
		}},
		IssueClaim: func(repo string, n int) (bool, error) { return claims[n], nil },
	}
}

func inboxIssue(n int, trusted bool, labels ...string) InboxIssue {
	return InboxIssue{Repo: inboxTestRepo, Number: n, Title: fmt.Sprintf("issue %d", n),
		Labels: append([]string{"to:worker"}, labels...), Author: "someone", Trusted: trusted}
}

func itemIDs(items []loopengine.Item) []string {
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func holdFor(f *FanoutLoop, n int) string {
	holds, _ := f.inboxHoldRows()
	ref := fmt.Sprintf("%s#%d", inboxTestRepo, n)
	for _, h := range holds {
		if h.ref == ref {
			return h.reason
		}
	}
	return ""
}

// TestInboxIssueOffered: a trusted, undecided, unrepresented to:worker issue is offered, in the
// leading addressed lane ahead of the fresh board row, as an inbox item keyed by its claim key.
func TestInboxIssueOffered(t *testing.T) {
	f := inboxLoop(t, func() ([]InboxIssue, error) { return []InboxIssue{inboxIssue(42, true)}, nil }, nil)
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	want := inboxItemID(inboxTestRepo, 42)
	if len(items) != 2 || items[0].ID != want || items[1].ID != "alpha/01" {
		t.Fatalf("queue = %v, want [%s alpha/01]", itemIDs(items), want)
	}
	p := items[0].Payload
	if p["kind"] != kindInbox || p["repo"] != inboxTestRepo || p["issue"] != "42" || p["to"] != "worker" {
		t.Errorf("inbox item payload = %v", p)
	}
	if classOf(items[0]) != "fresh" {
		t.Errorf("inbox item class = %q, want fresh", classOf(items[0]))
	}
	prompt := renderDispatchPrompt(items[0], loopengine.TierCheap)
	if !strings.Contains(prompt, "IMPLEMENT ISSUE "+inboxTestRepo+"#42") || !strings.Contains(prompt, "Issue: #42") {
		t.Errorf("inbox dispatch prompt does not name the issue as the task:\n%s", prompt)
	}
}

// TestInboxIssueNotOffered: untrusted, decision-owed, help wanted, PR-represented, claimed and
// placeholder-carried issues are each listed with why, and none is offered.
func TestInboxIssueNotOffered(t *testing.T) {
	issues := []InboxIssue{
		inboxIssue(1, false),
		inboxIssue(2, true, "needs-decision"),
		inboxIssue(3, true, "question"),
		inboxIssue(4, true, "help wanted"),
		inboxIssue(5, true),
		inboxIssue(6, true),
		inboxIssue(7, true),
		inboxIssue(8, true),
		{Repo: inboxTestRepo, Number: 9, Labels: []string{"to:worker"}, Author: "someone", TrustUnknown: "trust events unreadable"},
	}
	prs := []deskkit.PRRef{{Number: 50, State: "OPEN", Body: "Issue: #5"}, {Number: 60, State: "MERGED", Body: "Issue: #6"}}
	f := inboxLoop(t, func() ([]InboxIssue, error) { return issues, nil }, prs, 7)
	dir := filepath.Join(f.Root, "docs", "streams", "issue-loop")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ph := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-8\nrepo: " + inboxTestRepo + "\nissue: 8\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "issue-8.md"), []byte(ph), 0o600); err != nil {
		t.Fatal(err)
	}

	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	for _, it := range items {
		if it.Payload["kind"] == kindInbox {
			t.Errorf("inbox item %s was offered; every fixture issue fails a gate", it.ID)
		}
	}
	for n, want := range map[int]string{
		1: "QUARANTINED",
		2: "decision gate",
		3: "decision gate",
		4: "help wanted",
		5: "open PR",
		6: "merged PR",
		7: "live dispatch claim",
		8: "placeholder issue-loop/issue-8",
		9: "trust could-not-check",
	} {
		if got := holdFor(f, n); !strings.Contains(got, want) {
			t.Errorf("issue #%d hold = %q, want it to contain %q", n, got, want)
		}
	}

	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "INBOX NOT OFFERED: 9 issue(s)") {
		t.Errorf("plan does not list the inbox issues it did not offer:\n%s", got)
	}
}

// TestInboxIssueNotAddressedToRole: an issue addressed to a different desk is not this lane's row.
func TestInboxIssueNotAddressedToRole(t *testing.T) {
	other := InboxIssue{Repo: inboxTestRepo, Number: 11, Labels: []string{"to:verifier"}, Trusted: true}
	f := inboxLoop(t, func() ([]InboxIssue, error) { return []InboxIssue{other}, nil }, nil)
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != "alpha/01" {
		t.Fatalf("queue = %v, want only the board row", itemIDs(items))
	}
}

// TestInboxSourceErrorHoldsLane: an unreadable inbox holds the whole lane (stated in the plan)
// and never fails the plan; the board lane still flows.
func TestInboxSourceErrorHoldsLane(t *testing.T) {
	f := inboxLoop(t, func() ([]InboxIssue, error) { return nil, errors.New("listing refused") }, nil)
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != "alpha/01" {
		t.Fatalf("queue = %v, want only the board row", itemIDs(items))
	}
	if _, held := f.inboxHoldRows(); !strings.Contains(held, "listing refused") {
		t.Errorf("lane hold = %q, want the source error", held)
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "INBOX LANE HELD: could not read the addressed inbox") {
		t.Errorf("plan does not state the held lane:\n%s", out.String())
	}
}

// TestInboxWithoutPRIndexHoldsLane: with no PR index to dedupe against, no issue is offered unchecked.
func TestInboxWithoutPRIndexHoldsLane(t *testing.T) {
	f := inboxLoop(t, func() ([]InboxIssue, error) { return []InboxIssue{inboxIssue(42, true)}, nil }, nil)
	f.issueIndex = nil
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != "alpha/01" {
		t.Fatalf("queue = %v, want only the board row", itemIDs(items))
	}
	if _, held := f.inboxHoldRows(); !strings.Contains(held, "no PR index") {
		t.Errorf("lane hold = %q, want the missing PR index", held)
	}
}

// TestInboxClaimErrorHoldsIssue: a could-not-check claim read holds that issue, never offers it.
func TestInboxClaimErrorHoldsIssue(t *testing.T) {
	f := inboxLoop(t, func() ([]InboxIssue, error) { return []InboxIssue{inboxIssue(42, true)}, nil }, nil)
	f.IssueClaim = func(string, int) (bool, error) { return false, errors.New("refs unreadable") }
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("queue = %v, want only the board row", itemIDs(items))
	}
	if got := holdFor(f, 42); !strings.Contains(got, "dispatch claim could-not-check") {
		t.Errorf("hold = %q", got)
	}
}

// TestCarriedHoldRereadEveryTick: a carried decision hold is re-read on each SelectQueue. While
// its decision issue is open the held item is withheld; once it reads closed with human-decided
// the hold drops and the item is offered again, with the lift stated in the plan.
func TestCarriedHoldRereadEveryTick(t *testing.T) {
	f := inboxLoop(t, func() ([]InboxIssue, error) { return nil, nil }, nil)
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "example"}
	f.DecisionHolds = []CarriedHold{{Item: "alpha/01", Repo: fr, Issue: 77}}
	state := deskkit.DecisionHoldLive
	reads := 0
	f.ReadDecisionHold = func(r deskkit.ForgeRepo, n int) deskkit.DecisionHold {
		reads++
		return deskkit.DecisionHold{Repo: r, Issue: n, State: state, Why: "fixture state " + state.String()}
	}

	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("held item offered while its hold is live: %v", itemIDs(items))
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "HOLD LIVE: alpha/01") {
		t.Errorf("plan does not state the live hold:\n%s", out.String())
	}

	state = deskkit.DecisionHoldLifted
	items, err = f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != "alpha/01" {
		t.Fatalf("lifted hold still withholds the item: %v", itemIDs(items))
	}
	out.Reset()
	if err := renderPlan(f, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DROPPED HOLD: alpha/01 (lift met:") {
		t.Errorf("plan does not state the dropped hold:\n%s", out.String())
	}
	if reads < 3 {
		t.Errorf("decision issue read %d time(s); every SelectQueue must re-read it", reads)
	}

	state = deskkit.DecisionHoldUnknown
	items, err = f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("could-not-check lifted the hold: %v", itemIDs(items))
	}
}

func TestParseCarriedHold(t *testing.T) {
	h, err := parseCarriedHold("alpha/01=example-org/example#77")
	if err != nil {
		t.Fatal(err)
	}
	if h.Item != "alpha/01" || h.Repo.Slug() != inboxTestRepo || h.Issue != 77 {
		t.Errorf("parsed %+v", h)
	}
	for _, bad := range []string{"", "alpha/01", "=example-org/example#77", "alpha/01=#77", "alpha/01=example-org/example#0", "alpha/01=example-org/example#x"} {
		if _, err := parseCarriedHold(bad); err == nil {
			t.Errorf("parseCarriedHold(%q) accepted", bad)
		}
	}
}
