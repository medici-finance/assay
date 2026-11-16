package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repairSplitAt splits each Date / Runner cell at a chosen place instead of
// after the date, keeping the joined text equal to the old cell plus the
// principal.
func repairSplitAt(body, date, runner string) string {
	r := strings.NewReplacer(
		"| Date / Runner |\n|---|---------|----------|------------------------------|---------------|",
		"| Date | Runner |\n|---|---------|----------|------------------------------|------|--------|",
		"| 2026-10-16 verifier-app[bot] (model verifier) |",
		"| "+date+" | "+runner+" |",
	)
	return r.Replace(body)
}

// regression: #6616
func TestRepairDateCellIsDate(t *testing.T) {
	before := "Historical result retained.\n" + repairDelivered
	for _, c := range []struct{ name, date, runner string }{
		{"identity-in-date", "2026-10-16 verifier-app[bot]", "(model verifier) (on-behalf-of human:alex)"},
		{"qualifier-in-date", "2026-10-16 verifier-app[bot] (model", "verifier) (on-behalf-of human:alex)"},
		{"runner-only", "", "2026-10-16 verifier-app[bot] (model verifier) (on-behalf-of human:alex)"},
	} {
		after := repairSplitAt(before, c.date, c.runner)
		if why := evidenceRepairEditorial(before, after); !strings.Contains(why, "Date / Runner text") {
			t.Errorf("%s: wanted a Date / Runner refusal, got %q", c.name, why)
		}
	}
	ok := repairSplitAt(before, "2026-10-16", "verifier-app[bot] (model verifier) (on-behalf-of human:alex)")
	if why := evidenceRepairEditorial(before, ok); why != "" {
		t.Fatalf("a split at the date was refused: %s", why)
	}
}

// regression: #6616
func TestRepairReaderView(t *testing.T) {
	quoted := "Example quotation:\n```\nVERIFY: PASS\n```\n"
	before := "Historical result retained.\n" + quoted + repairDelivered
	ok := repairSplit(before)
	if why := evidenceRepairEditorial(before, ok); why != "" {
		t.Fatalf("the plain repair was refused: %s", why)
	}
	table := "| # | Command | Expected | Observed (exit + key output) | Date | Runner |\n"
	for _, c := range []struct{ name, after string }{
		{"comment-marker", ok + "Verification-<!-- x -->Table: delivery=x\n"},
		{"comment-verdict", ok + "VERIFY<!-- x -->: PASS\n"},
		{"fence-parity", strings.Replace(ok, "Example quotation:\n", "Example quotation:\n```\n", 1)},
		{"comment-rows", strings.Replace(ok, table, "<!--\n"+table, 1) + "-->\n"},
		{"table-split", strings.Replace(ok, "| 11 | run x", "\nNote.\n| 11 | run x", 1)},
		{"inert-comment", ok + "<!-- note -->\n"},
		{"mid-line-marker", ok + "Note on Verification-Table: x\n"},
		{"fenced-verdict", strings.Replace(ok, "VERIFY: PASS\n", "VERIFY: PASS\nVERIFY: FAIL\n", 1)},
	} {
		if why := evidenceRepairEditorial(before, c.after); why == "" {
			t.Errorf("%s: an added line that changes what a reader reads was admitted", c.name)
		}
	}
}

// repairRoster installs the fixture roster with a second mapped human who is
// not the blessing authority.
func repairRoster(t *testing.T) {
	t.Helper()
	t.Cleanup(scanReloadConfig)
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := strings.Replace(fixtureRoster+darFixtureExtra(), "ASSAY_HUMAN_LOGIN_MAP=alex:ada",
		"ASSAY_HUMAN_LOGIN_MAP=alex:ada,example_other:"+rlOther, 1)
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	scanReloadConfig()
	if l, ok := HumanLogin("example_other"); !ok || l != rlOther {
		t.Fatalf("test roster not in effect: example_other -> %q, %v", l, ok)
	}
}

// regression: #6616
func TestRepairApprovalScope(t *testing.T) {
	repairRoster(t)
	login, _ := HumanLogin("alex")
	for _, c := range []struct{ name, author, text, want string }{
		{"other-human", rlOther, repairApprovalText, "is authored by"},
		{"path-suffix", login, "For PR #77 only, I authorize separating Date and Runner in the other/vf/01 Evidence.", "does not name the brief"},
		{"other-thread", login, "Noted: the vf/01 Evidence layout looks fine.", "does not authorize"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, repair := repairChain(t, repairSplit, repairMessage)
			f.extra[repairComment] = repairApproval(c.author, "User", c.text, rlPosted)
			f.withRepair(repair)
			f.refuse(t, c.want, false)
		})
	}
	f, repair := repairChain(t, repairSplit, repairMessage)
	f.withRepair(repair)
	if _, why := f.plan(t); why != "" {
		t.Fatalf("the driver's approval was refused: %s", why)
	}
}

// regression: #6616
func TestRepairWholeDocView(t *testing.T) {
	quoted := "Example quotation:\n```\nVERIFY: PASS\n```\n"
	before := "Historical result retained.\n" + quoted + repairDelivered
	ok := repairSplit(before)
	table := "| # | Command | Expected | Observed (exit + key output) | Date | Runner |\n"
	for _, c := range []struct{ name, after, want string }{
		{"marker", ok + "Verification-<!-- x -->Table: delivery=x\n", "add a marker"},
		{"verdict", ok + "VERIFY<!-- x -->: PASS\n", "add a verdict"},
		{"fence", strings.Replace(ok, "Example quotation:\n", "Example quotation:\n```\n", 1), "add a verdict"},
		{"rows", strings.Replace(ok, table, "<!--\n"+table, 1) + "-->\n", "Evidence row"},
		{"open", ok + "<!-- open\n", "unterminated comment"},
		{"split", strings.Replace(ok, "| 11 | run x", "\n| 11 | run x", 1), "split or join"},
	} {
		if why := evidenceRepairReaderView(before, c.after, nil); !strings.Contains(why, c.want) {
			t.Errorf("%s: want %q, got %q", c.name, c.want, why)
		}
	}
	if why := evidenceRepairReaderView(before, ok, nil); why != "" {
		t.Fatalf("the plain repair was refused: %s", why)
	}
}
