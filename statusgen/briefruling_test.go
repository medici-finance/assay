package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the brief ruling-link lane (briefruling.go): a pull request that
// adds or changes a brief's frontmatter ruling: link has that link resolved on
// the forge, whatever the brief's gate or status, and an unresolvable or
// refused link fails the run. The forge is the faked one of
// decisionruling_test.go; the board fixtures are decisiongatehold_test.go's.

// brPR commits base as the PR merge-base, writes head on top as the PR's
// commit, and runs the PR lane against that base. The forge is wired through
// the decision-gate seams (nil = the forge must not be contacted).
func brPR(t *testing.T, forge *rlForge, base, head map[string]string) []decisionGateResult {
	t.Helper()
	dhSeams(t, forge)
	root := dhFixture(t, base)
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	mb := strings.TrimSpace(string(out))
	dhWrite(t, root, head)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "the change")
	var files []ghPRFile
	for rel := range head {
		files = append(files, ghPRFile{Filename: rel})
	}
	return decisionGatePRLane(root, rlRepo, files, mb, nil)
}

// brBase is a brief at todo (no gate: human, no status move in the change)
// whose decision issue #41 is recorded with no ruling yet.
func brBase() map[string]string {
	return map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("", "decision-issue: 41\n"),
	}
}

// brRuled is the change under test: the same brief, the ruling link added.
func brRuled() map[string]string {
	return map[string]string{dhBriefPath: dhFM("", dhRuledLines())}
}

const brRulingText = "Option 1 for " + dhBoardID + ", as written."

// brWantRefused asserts the lane refused the brief's ruling with reason.
func brWantRefused(t *testing.T, rs []decisionGateResult, reason rulingReason) {
	t.Helper()
	for _, r := range rs {
		if r.Refused && strings.Contains(r.Detail, string(reason)) {
			return
		}
	}
	t.Fatalf("no refusal naming %q for the added ruling link: %+v", reason, rs)
}

// regression: #2595
func TestBriefRulingBotAuthor(t *testing.T) {
	f := dhForge(brRulingText)
	f.commentBody = rlCommentJSONWith("example-bot[bot]", "Bot", rlRepo, rlIssue, brRulingText, rlPosted, rlPosted)
	brWantRefused(t, brPR(t, f, brBase(), brRuled()), rulingBotAuthor)
}

// regression: #2595
func TestBriefRulingEdited(t *testing.T) {
	f := dhForge(brRulingText)
	f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, brRulingText, rlPosted, "2026-09-02T10:00:00Z")
	brWantRefused(t, brPR(t, f, brBase(), brRuled()), rulingEditedComment)
}

// regression: #2595
func TestBriefRulingMissing(t *testing.T) {
	f := dhForge(brRulingText)
	f.commentStatus = 404
	f.commentBody = `{"message":"Not Found"}`
	brWantRefused(t, brPR(t, f, brBase(), brRuled()), rulingDeleted)
}

// regression: #2595
func TestBriefRulingNonDriver(t *testing.T) {
	f := dhForge(brRulingText)
	f.commentBody = rlCommentJSONWith("example-stranger", "User", rlRepo, rlIssue, brRulingText, rlPosted, rlPosted)
	brWantRefused(t, brPR(t, f, brBase(), brRuled()), rulingWrongAuthor)
}

// regression: #2595
func TestBriefRulingNotUser(t *testing.T) {
	f := dhForge(brRulingText)
	f.commentBody = rlCommentJSONWith(rlHuman, "Organization", rlRepo, rlIssue, brRulingText, rlPosted, rlPosted)
	brWantRefused(t, brPR(t, f, brBase(), brRuled()), rulingBotAuthor)
}

// A ruling link to a comment on an issue other than the recorded decision issue
// is refused before the forge is read.
func TestBriefRulingOtherIssue(t *testing.T) {
	head := map[string]string{dhBriefPath: dhFM("", "decision-issue: 42\nruling: "+dhRuling+"\n")}
	rs := brPR(t, nil, brBase(), head)
	if !decisionGateRefused(rs) {
		t.Fatalf("a ruling link off the decision issue was not refused: %+v", rs)
	}
}

// A ruling link with no decision issue recorded is refused: there is no issue
// to bind it to.
func TestBriefRulingNoIssue(t *testing.T) {
	base := map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("", ""),
	}
	head := map[string]string{dhBriefPath: dhFM("", "ruling: "+dhRuling+"\n")}
	rs := brPR(t, nil, base, head)
	if !decisionGateRefused(rs) {
		t.Fatalf("a ruling link with no decision issue was not refused: %+v", rs)
	}
	for _, r := range rs {
		if r.Refused && strings.Contains(r.Detail, "no decision issue is recorded") {
			return
		}
	}
	t.Fatalf("the refusal does not name the missing decision issue: %+v", rs)
}

// POSITIVE: the driver's own unedited comment, naming the brief, on its
// decision issue, passes and is reported as ruled.
func TestBriefRulingPositive(t *testing.T) {
	rs := brPR(t, dhForge(brRulingText), brBase(), brRuled())
	if decisionGateRefused(rs) {
		t.Fatalf("a good ruling link was refused: %+v", rs)
	}
	var ruled bool
	for _, r := range rs {
		ruled = ruled || strings.Contains(r.Detail, "ruled by "+rlHuman)
	}
	if !ruled {
		t.Fatalf("the added ruling link was not resolved: %+v", rs)
	}
}

// A ruling link the base already carries is not re-resolved: only an added or
// changed link is judged here (the move lanes judge the rest).
func TestBriefRulingUnchanged(t *testing.T) {
	base := map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("", dhRuledLines()),
	}
	head := map[string]string{dhBriefPath: dhFM("", dhRuledLines()+"wave-note: unrelated\n")}
	if rs := brPR(t, nil, base, head); len(rs) != 0 {
		t.Fatalf("an unchanged ruling link was judged: %+v", rs)
	}
}

// A brief file moved into done/ carries its ruling link with it; the link is
// not new.
func TestBriefRulingMovedFile(t *testing.T) {
	base := map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("", dhRuledLines()),
	}
	head := map[string]string{
		dhBriefPath: "",
		"docs/streams/" + dhStream + "/done/brief-18.md": dhFM("", dhRuledLines()),
	}
	if rs := brPR(t, nil, base, head); len(rs) != 0 {
		t.Fatalf("a moved brief's unchanged ruling link was judged: %+v", rs)
	}
}

// A changed ruling link is judged even when the base carried another one.
func TestBriefRulingChanged(t *testing.T) {
	base := map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("", "decision-issue: 41\nruling: "+rlURL(rlRepo, rlIssue, 9000)+"\n"),
	}
	f := dhForge(brRulingText)
	f.commentBody = rlCommentJSONWith("example-bot[bot]", "Bot", rlRepo, rlIssue, brRulingText, rlPosted, rlPosted)
	brWantRefused(t, brPR(t, f, base, brRuled()), rulingBotAuthor)
}

// ---- the staged workflow's corroborate step -------------------------------------

// brCorroborateStep is the corroborate job's --corroborate step from the
// staged workflow, exactly as the runner executes it.
func brCorroborateStep(t *testing.T) wfStep {
	t.Helper()
	for _, st := range stagedWorkflow(t).Jobs["corroborate"].Steps {
		if strings.Contains(st.Run, "--corroborate --pr") {
			return st
		}
	}
	t.Fatalf("%s: jobs.corroborate has no --corroborate step", reconcileWorkflowPath())
	return wfStep{}
}

// brRunStep runs the step's script in a repository whose HEAD^1 holds base and
// HEAD the change, with a stand-in guard binary and gh. It reports the exit
// code, the output, and whether the guard ran.
func brRunStep(t *testing.T, base, head map[string]string) (int, string, bool) {
	t.Helper()
	requireTool(t, "bash")
	root := dhFixture(t, base)
	dhWrite(t, root, head)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "--allow-empty", "-m", "the change")
	tmp := t.TempDir()
	ran := filepath.Join(tmp, "guard-ran")
	guard := "#!/bin/sh\necho ran > '" + ran + "'\n"
	if err := os.WriteFile(filepath.Join(tmp, "statusgen-base"), []byte(guard), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tmp, "bin")
	mustMkdirAll(t, bin)
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho main\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := brCorroborateStep(t)
	cmd := exec.Command("bash", scriptFile(t, st.Run))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "RUNNER_TEMP="+tmp, "PR_NUMBER=7", "BASE_REF=main",
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(ran)
	return code, string(out), statErr == nil
}

// regression: #2595
func TestStagedCorroborateFilter(t *testing.T) {
	dr := "docs/streams/decisions/DR-example-ruling-rec.md"
	for _, tc := range []struct {
		name       string
		base, head map[string]string
		want       bool
	}{
		{"decision record added", brBase(), map[string]string{dr: "---\nid: DR-example-ruling-rec\n---\n"}, true},
		{"brief ruling added", brBase(), brRuled(), true},
		{"archived brief ruling added",
			map[string]string{"docs/archive/s/done/brief-1.md": dhFM("", "decision-issue: 41\n")},
			map[string]string{"docs/archive/s/done/brief-1.md": dhFM("", dhRuledLines())}, true},
		{"findings entry", brBase(), map[string]string{"docs/streams/findings/F-example.md": "x\n"}, true},
		{"brief edit without a ruling", brBase(), map[string]string{dhBriefPath: dhFM("", "decision-issue: 41\nwave-note: x\n")}, false},
		{"unrelated file", brBase(), map[string]string{"README.md": "x\n"}, false},
	} {
		code, out, ran := brRunStep(t, tc.base, tc.head)
		if code != 0 {
			t.Errorf("%s: step exit %d:\n%s", tc.name, code, out)
		}
		if ran != tc.want {
			t.Errorf("%s: guard ran = %v, want %v:\n%s", tc.name, ran, tc.want, out)
		}
	}
}
