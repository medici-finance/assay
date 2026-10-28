package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// admission_test.go — `deskboard dispatch` drops the rows deskdispatch refuses at
// claim-acquire (medici-finance/assay#2744), and keeps a row it could not check.

type boardDecisionForge struct {
	issues map[int]*deskkit.Issue
	ruled  map[int]bool
}

func (f *boardDecisionForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	if it, ok := f.issues[n]; ok {
		return it, nil
	}
	return nil, errors.New("not found")
}

func (f *boardDecisionForge) SearchIssues(deskkit.ForgeRepo, deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	return nil, nil
}

func (f *boardDecisionForge) ListIssueLabelEvents(deskkit.ForgeRepo, int) ([]deskkit.LabelEvent, error) {
	return []deskkit.LabelEvent{{Name: deskkit.DecisionLabel, CreatedAt: "2026-10-01T10:00:00Z"}}, nil
}

func (f *boardDecisionForge) IssueTrustEvents(_ deskkit.ForgeRepo, n int) (*deskkit.TrustPayload, error) {
	tp := &deskkit.TrustPayload{Complete: true}
	if f.ruled[n] {
		tp.Events = []deskkit.ContentEvent{{Author: "example-ratifier", CreatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}
	}
	return tp, nil
}

// admissionRoot plants one brief file per front matter under <root>/docs/streams/s/.
func admissionRoot(t *testing.T, briefs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for num, front := range briefs {
		body := "---\nbrief: s/" + num + "\n" + front + "---\n\n# Brief\n"
		if err := os.WriteFile(filepath.Join(dir, "brief-"+num+"-x.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func useBoardDecisions(t *testing.T, f *boardDecisionForge) {
	t.Helper()
	oldF, oldR := decisionForgeFor, decisionRatifierFn
	decisionForgeFor = func(repo string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return f, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	decisionRatifierFn = func() (func(string, int64) bool, bool) {
		return func(login string, _ int64) bool { return login == "example-ratifier" }, true
	}
	t.Cleanup(func() { decisionForgeFor, decisionRatifierFn = oldF, oldR })
}

func admissionReport(root string, nums ...string) *dispatchReport {
	rep := &dispatchReport{Eligible: len(nums)}
	for _, n := range nums {
		rep.Rows = append(rep.Rows, dispatchRow{Repo: "example-org/tracker", Root: root, Brief: "s/" + n, Stream: "s"})
	}
	return rep
}

func rowBriefs(rep *dispatchReport) string {
	var out []string
	for _, r := range rep.Rows {
		out = append(out, r.Brief)
	}
	return strings.Join(out, ",")
}

// regression: medici-finance/assay#2744
func TestBoardDropsStrongAtAny(t *testing.T) {
	root := admissionRoot(t, map[string]string{"01": "exec-tier: strong\n", "02": "exec-tier: any\n", "03": ""})
	useBoardDecisions(t, &boardDecisionForge{})

	rep := admissionReport(root, "01", "02", "03")
	applyBriefAdmission(rep, "any")
	if got := rowBriefs(rep); got != "s/02,s/03" || rep.HeldByExecTier != 1 {
		t.Fatalf("--tier any: rows %q heldByExecTier %d; want s/02,s/03 and 1", got, rep.HeldByExecTier)
	}
	if line := dispatchHeldBackLine(rep); !strings.Contains(line, "1 by exec-tier stricter than --tier any") {
		t.Errorf("the held-back line does not name the exec-tier hold: %s", line)
	}

	rep = admissionReport(root, "01", "02", "03")
	applyBriefAdmission(rep, "")
	if got := rowBriefs(rep); got != "s/01,s/02,s/03" || rep.Rows[0].ExecTier != "strong" {
		t.Fatalf("no --tier: rows %q (exec-tier %q); want every row kept and tagged", got, rep.Rows[0].ExecTier)
	}
}

// TestBoardHoldsContinuationLines pins that an exec-tier, gate or decision
// issue line the shared parser reads as text inside another value is held at
// the board, as the line readers held it, and never dispatched as model/any.
func TestBoardHoldsContinuationLines(t *testing.T) {
	root := admissionRoot(t, map[string]string{
		"01": "tags: [a,\nexec-tier: strong]\nexec-tier: any\n",
		"02": "gate: human\nmeta: {a: 1,\ndecision-issue: 41\n}\n",
		"03": "exec-tier: any\n",
	})
	useBoardDecisions(t, &boardDecisionForge{
		issues: map[int]*deskkit.Issue{41: {Number: 41, State: "open", Labels: []string{deskkit.DecisionLabel}}},
	})

	rep := admissionReport(root, "01", "02", "03")
	applyBriefAdmission(rep, "any")
	if got := rowBriefs(rep); got != "s/03" || rep.HeldByExecTier != 2 {
		t.Fatalf("--tier any: rows %q heldByExecTier %d; want s/03 and 2", got, rep.HeldByExecTier)
	}
	rep = admissionReport(root, "02", "03")
	applyBriefAdmission(rep, "strong")
	if got := rowBriefs(rep); got != "s/03" || rep.HeldByDecision != 1 {
		t.Fatalf("--tier strong: rows %q heldByDecision %d; want s/03 and 1", got, rep.HeldByDecision)
	}
}

// regression: medici-finance/assay#2744
func TestBoardDropsUnruledGate(t *testing.T) {
	root := admissionRoot(t, map[string]string{
		"01": "gate: human\ndecision-issue: 41\n",
		"02": "gate: human\ndecision-issue: 42\n",
		"03": "gate: human\ndecision-issue: 43\n", // unreadable: kept, could-not-check
		"04": "gate: human\n",                     // no decision issue yet: dispatch files it
	})
	open := func(n int) *deskkit.Issue {
		return &deskkit.Issue{Number: n, State: "open", Labels: []string{deskkit.DecisionLabel}}
	}
	useBoardDecisions(t, &boardDecisionForge{
		issues: map[int]*deskkit.Issue{41: open(41), 42: open(42)},
		ruled:  map[int]bool{42: true},
	})

	rep := admissionReport(root, "01", "02", "03", "04")
	applyBriefAdmission(rep, "strong")
	if got := rowBriefs(rep); got != "s/02,s/03,s/04" || rep.HeldByDecision != 1 {
		t.Fatalf("rows %q heldByDecision %d; want s/02,s/03,s/04 and 1", got, rep.HeldByDecision)
	}
	if len(rep.AdmissionHeld) != 1 || !strings.Contains(rep.AdmissionHeld[0].Reason, "example-org/tracker#41") {
		t.Errorf("the held row does not name its decision issue: %+v", rep.AdmissionHeld)
	}
	if len(rep.AdmissionUnknown) != 1 || !strings.Contains(rep.AdmissionUnknown[0].Reason, "s/03") {
		t.Errorf("the unreadable decision was not named could-not-check: %+v", rep.AdmissionUnknown)
	}
}

// The verb end to end: `dispatch --tier any` reads the tier off the command line and drops
// the strong row the merged queue carried.
// regression: medici-finance/assay#2744
func TestDispatchVerbTierAny(t *testing.T) {
	installFakeStatusgen(t)
	tracker, toolkit := twoRoots(t)
	useBoardDecisions(t, &boardDecisionForge{})
	for root, tier := range map[string]string{tracker: "strong", toolkit: "any"} {
		stream := filepath.Base(root) + "-stream"
		dir := filepath.Join(root, "docs", "streams", stream)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nbrief: " + stream + "/02\nexec-tier: " + tier + "\n---\n"
		if err := os.WriteFile(filepath.Join(dir, "brief-02-x.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var out, errb bytes.Buffer
	if code := run([]string{"dispatch", "--tier", "any"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("dispatch exited %d, stderr=%s", code, errb.String())
	}
	var rep dispatchReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("dispatch output is not JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) != 1 || rep.Rows[0].Repo != "medici-finance/assay" || rep.HeldByExecTier != 1 || rep.Shown != 1 {
		t.Fatalf("--tier any: rows %+v heldByExecTier %d shown %d; want only the any-tier row", rep.Rows, rep.HeldByExecTier, rep.Shown)
	}
}

func TestBoardKeepsMissingBrief(t *testing.T) {
	useBoardDecisions(t, &boardDecisionForge{})
	rep := admissionReport(t.TempDir(), "09")
	applyBriefAdmission(rep, "any")
	if rowBriefs(rep) != "s/09" || len(rep.AdmissionUnknown) != 1 {
		t.Fatalf("a row with no brief file was not kept as could-not-check: rows %q unknown %+v", rowBriefs(rep), rep.AdmissionUnknown)
	}
}

func TestDispatchTierArg(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
		ok   bool
	}{
		{nil, "", true},
		{[]string{"--tier", "any"}, "any", true},
		{[]string{"--tier=strong"}, "strong", true},
		{[]string{"--tier", "fast"}, "", false},
		{[]string{"--tier"}, "", false},
	} {
		r := cliRecord(t)
		code, _, _ := r.exec(append([]string{"dispatch"}, c.args...)...)
		got := r.tier
		var err error
		if code != 0 {
			err = deskkit.Refused("CLI refused")
		}
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("dispatchTierArg(%q) = %q, %v", c.args, got, err)
		}
		if err != nil && deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Errorf("dispatchTierArg(%q): exit %d, want a refusal", c.args, deskkit.ExitCodeOf(err))
		}
	}
}
