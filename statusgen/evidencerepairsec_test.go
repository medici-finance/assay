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
	// An old cell with no leading date: its first token is an identity, and a
	// split there must refuse (F-repair-scope-isolation).
	undated := strings.ReplaceAll(before, "| 2026-10-16 verifier-app[bot] (model verifier) |", "| verifier-app[bot] (model verifier) |")
	after := strings.ReplaceAll(repairSplitAt(before, "x", "y"), "| x | y |", "| verifier-app[bot] | (model verifier) |")
	if why := evidenceRepairEditorial(undated, after); !strings.Contains(why, "Date / Runner text") {
		t.Errorf("non-date: wanted a Date / Runner refusal, got %q", why)
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
		// Each half of the thread rule alone (F-repair-scope-isolation).
		{"wrong-thread", login, "For PR #78 only, I authorize separating Date and Runner in the vf/01 Evidence.", "does not authorize"},
		{"no-thread", login, "I authorize separating Date and Runner in the vf/01 Evidence.", "does not authorize"},
		{"no-authorize", login, "For PR #77: separating Date and Runner in the vf/01 Evidence looks fine.", "does not authorize"},
		// A negated or conditional sentence is not an approval (SR-A1).
		{"negated", login, "For PR #77 only, I do NOT authorize separating Date and Runner in the vf/01 Evidence.", "does not authorize"},
		{"contracted", login, "For PR #77 only, I don't authorize separating Date and Runner in the vf/01 Evidence.", "does not authorize"},
		{"conditional", login, "If CI passes on PR #77, I authorize separating Date and Runner in the vf/01 Evidence.", "does not authorize"},
		{"unauthorized", login, "For PR #77: separating Date and Runner in the vf/01 Evidence is unauthorized.", "does not authorize"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, repair := repairChain(t, repairSplit, repairMessage)
			f.extra[repairComment] = repairApproval(c.author, "User", c.text, rlPosted)
			f.withRepair(repair)
			f.refuse(t, c.want, false)
		})
	}
	// Admitted: the fixture approval, and the sanctioned approval's own wording
	// in fixture coordinates, whose closing scope limit ("not a new
	// verification ... claim") sits outside the authorizing sentence.
	for name, text := range map[string]string{"plain": repairApprovalText, "ruling-wording": repairRulingText} {
		t.Run("admit-"+name, func(t *testing.T) {
			f, repair := repairChain(t, repairSplit, repairMessage)
			f.extra[repairComment] = repairApproval(login, "User", text, rlPosted)
			f.withRepair(repair)
			if _, why := f.plan(t); why != "" {
				t.Fatalf("the driver's approval was refused: %s", why)
			}
		})
	}
}

// repairRulingText is the sanctioned approval's wording with its thread,
// brief and principal moved to fixture coordinates.
const repairRulingText = "For PR #77 only, I authorize the worker to separate Date and Runner and " +
	"transcribe `(on-behalf-of human:alex)` into the 14 retained vf/01 Evidence Runner cells " +
	"from the original verifier publication PR #12. Preserve every original command, " +
	"expectation, observation, date, verifier identity, attestation, source revision and " +
	"result. This is a one-item editorial exception to tool-stamped ownership, not a new " +
	"verification, execution or acceptance claim."

// The approving sentence is a plain declarative "I authorize": a question, a
// refusal, another speaker, a struck or marked-up sentence, or a negation
// spelled to hide from a tokenizer does not approve, while a scope limit in
// another sentence does not refuse (SR-A1, F-repair-approval-hedge).
//
// regression: #6616
func TestRepairAuthorizes(t *testing.T) {
	const tail = " separating Date and Runner in the vf/01 Evidence for PR #77."
	refuse := map[string]string{
		"question":      "For PR #77, I authorize separating Date and Runner in the vf/01 Evidence?",
		"should-i":      "Should I authorize separating Date and Runner in the vf/01 Evidence for PR #77?",
		"refuse-to":     "For PR #77, I refuse to authorize" + tail,
		"decline-to":    "For PR #77, I decline to authorize" + tail,
		"later-reject":  "I authorize" + tail + " On reflection, I reject it.",
		"would-but":     "For PR #77, I would authorize the vf/01 repair, but I am rejecting it.",
		"nobody":        "Nobody should authorize the vf/01 repair on PR #77.",
		"who-can":       "For PR #77: who can authorize the vf/01 repair?",
		"others":        "Others authorize" + tail,
		"struck":        "~~I authorize" + tail + "~~",
		"modifier":      "I don\u02bct authorize" + tail,
		"backtick":      "I don`t authorize" + tail,
		"prime":         "I don\u2032t authorize" + tail,
		"emphasis":      "I do n*o*t authorize" + tail,
		"cyrillic":      "I do n\u043et authorize" + tail,
		"entity":        "I do n&#111;t authorize" + tail,
		"hedged-repeat": "I authorize" + tail + " I do not authorize the rest.",
		"split-para":    "If CI passes on PR #77,\n\nI authorize" + tail,
		// "I authorize" stays adjacent, so only the n't suffix refuses these.
		"contraction-after": "For PR #77 only, I authorize separating Date and Runner in the vf/01 Evidence, which shouldn't be applied yet.",
		"contraction-curly": "For PR #77 only, I authorize separating Date and Runner in the vf/01 Evidence, which shouldn\u2019t be applied yet.",
		// Only the bare present form counts; "authorized" and "authorizes" do not.
		"past-tense":   "For PR #77 only, I authorized" + tail,
		"third-person": "For PR #77 only, I authorizes" + tail,
	}
	for name, text := range refuse {
		if repairAuthorizes(text) {
			t.Errorf("%s: %q was read as an approval", name, text)
		}
	}
	for name, text := range map[string]string{
		"plain":          repairApprovalText,
		"ruling-wording": repairRulingText,
		"apostrophe":     "For PR #77, I authorize the driver\u2019s vf/01 repair.",
	} {
		if !repairAuthorizes(text) {
			t.Errorf("%s: %q was refused", name, text)
		}
	}
}

// An added link-reference or footnote definition is not rendered, and with an
// open title quote it hides the prose after it on the page (SR-A4).
//
// regression: #6616
func TestRepairNoLinkDef(t *testing.T) {
	before := "Historical result retained.\nPrior note.\n" + repairDelivered
	ok := repairSplit(before)
	for _, add := range []string{"[n]: /x \"\nPrior note.\n\"", "[^1]: note\nPrior note.", "   [x]: y\nPrior note."} {
		after := strings.Replace(ok, "Prior note.", add, 1)
		if why := evidenceRepairEditorial(before, after); !strings.Contains(why, "definition") {
			t.Errorf("%q: want a definition refusal, got %q", add, why)
		}
	}
	if why := evidenceRepairEditorial(before, strings.Replace(ok, "Prior note.", "See [n] below.\nPrior note.", 1)); why != "" {
		t.Fatalf("a bracket in prose was refused: %s", why)
	}
}

// A split table is split in every row: a header-only or partial split leaves
// rows that disagree on which column holds the runner. The column map resets
// at each table, so a later table cannot inherit an earlier table's split
// (SR-A3).
//
// regression: #6616
func TestRepairSplitComplete(t *testing.T) {
	before := "Historical result retained.\n" + repairDelivered
	header := strings.NewReplacer(
		"| Date / Runner |\n|---|---------|----------|------------------------------|---------------|",
		"| Date | Runner |\n|---|---------|----------|------------------------------|------|--------|",
	)
	row1 := strings.NewReplacer("| exit 0; ok | 2026-10-16 verifier-app[bot] (model verifier) |",
		"| exit 0; ok | 2026-10-16 | verifier-app[bot] (model verifier) |")
	hashed := strings.Replace(before, "| 11 |", "| # | a | b | c | d |\n| 11 |", 1)
	second := "\nOther table.\n\n| n | Command | Expected | Observed (exit + key output) | Date / Runner |\n" +
		"|---|---------|----------|------------------------------|---------------|\n" +
		"| 2 | go vet ./x | exit 0 | exit 0 | 2026-10-16 verifier-app[bot] (model verifier) |\n"
	for _, c := range []struct{ name, before, after, want string }{
		{"header-only", before, header.Replace(before), "every row"},
		{"partial", before, row1.Replace(header.Replace(before)), "every row"},
		{"later-table", before + second, repairSplit(before) + strings.NewReplacer(
			"| Date / Runner |\n|---|---------|----------|------------------------------|---------------|",
			"| Date | Runner |\n|---|---------|----------|------------------------------|------|--------|",
			"| 2026-10-16 verifier-app[bot] (model verifier) |", "| 2026-10-16 | verifier-app[bot] (model verifier) |",
		).Replace(second), "outside any table"},
		// A mid-table data row whose first cell is "#" is not a header (the
		// reader needs a separator after it), so it cannot reset the split.
		{"hash-row", hashed, strings.Replace(repairSplit(hashed), "| 11 | run x 2>&1 \\| grep -c y | 1 | printed 1 | 2026-10-16 | verifier-app[bot] (model verifier) (on-behalf-of human:alex) |",
			"| 11 | run x 2>&1 \\| grep -c y | 1 | printed 1 | 2026-10-16 verifier-app[bot] (model verifier) |", 1), "every row"},
	} {
		if why := evidenceRepairEditorial(c.before, c.after); !strings.Contains(why, c.want) {
			t.Errorf("%s: want %q, got %q", c.name, c.want, why)
		}
	}
	if why := evidenceRepairEditorial(before, repairSplit(before)); why != "" {
		t.Fatalf("the full split was refused: %s", why)
	}
}

// An added line may not start an HTML block: an unterminated processing
// instruction, CDATA section or declaration hides the rest of the rendered page,
// and <details> collapses it, while the machine readers still see the rows
// (SR-A4).
//
// regression: #6616
func TestRepairNoHTML(t *testing.T) {
	before := "Historical result retained.\n" + repairDelivered
	ok := repairSplit(before)
	for _, line := range []string{"<?x", "<![CDATA[", "<!X", "<pre>", "<details>", "</details>", "  <div>", "Note <b>x</b>"} {
		after := strings.Replace(ok, "Editorial repair only", line+"\nEditorial repair only", 1)
		if why := evidenceRepairEditorial(before, after); !strings.Contains(why, "HTML") {
			t.Errorf("%q: want an HTML refusal, got %q", line, why)
		}
	}
	if why := evidenceRepairEditorial(before, strings.Replace(ok, "Editorial repair only", "Note: 2 < 3.\nEditorial repair only", 1)); why != "" {
		t.Fatalf("a plain '<' in prose was refused: %s", why)
	}
}

// The whole-document verdict comparison is element-wise: the same number of
// verdicts with a different word, or a struck one unstruck, is refused even
// when no added line carries it (SR-A5).
//
// regression: #6616
func TestRepairVerdictOrder(t *testing.T) {
	for _, c := range []struct{ name, old, new string }{
		{"word", "VERIFY: FAIL\n", "VERIFY: PASS\n"},
		{"struck", "~~VERIFY: PASS~~\n", "VERIFY: PASS\n"},
	} {
		before := "Historical result retained.\n" + c.old + repairDelivered
		after := strings.Replace(repairSplit(before), c.old, c.new, 1)
		if why := evidenceRepairReaderView(before, after, nil); !strings.Contains(why, "add a verdict") {
			t.Errorf("%s: want a verdict refusal, got %q", c.name, why)
		}
	}
}

// With no driver resolvable from the blessing login, a repair approval is
// could-not-check before any forge read, named as such (SR-A5).
//
// regression: #6616
func TestRepairDriverUnset(t *testing.T) {
	t.Cleanup(scanReloadConfig)
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := strings.Replace(fixtureRoster+darFixtureExtra(), "ASSAY_BLESS_LOGIN=ada:100001", "ASSAY_BLESS_LOGIN=example-unmapped:100009", 1)
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	scanReloadConfig()
	link := rulingLink{Repo: rlRepo, Issue: repairPR, CommentID: repairComment}
	why := corroborateEvidenceRepairs(rlRepo, "docs/streams/vf/brief-01-x.md",
		[]evidenceRepair{{commit: strings.Repeat("a", 40), link: link}},
		func() *ghClient { t.Fatal("the driver guard read the forge"); return nil })
	if !strings.Contains(why, "could-not-check human authority: the driver's login is unset") {
		t.Fatalf("want the driver could-not-check, got %q", why)
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
