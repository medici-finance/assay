package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests for the decision-gate hold (decisiongatehold.go, lifecycle-v1 §4.5).
//
// Every fixture is a throwaway git repository: the base board is committed and
// pinned as refs/remotes/origin/main, and the change under test is written to
// the working tree, exactly as a pull request's checkout or a landing tool's
// staged write presents it. Layer two runs against the faked forge of
// decisionruling_test.go (rlForge) — no test reaches the network — with the
// example-* human-login map passed in through the seams.
//
// The rows named "Verify N" are the brief's Verify table rows; the rows named
// "amendment" are the REFUSED rows its Tasks section requires under option 1.

const (
	dhStream  = "sdlc"
	dhBoardID = "sdlc/18"
	dhMarker  = "<!-- decision-gate: " + dhBoardID + " -->"
	dhPermID  = "0d4c1f3e-example-perm-id"
)

var dhRuling = rlURL(rlRepo, rlIssue, rlComment)

// dhFM is a brief-18 frontmatter with the given gate line and extra lines.
func dhFM(gateLine, extra string) string {
	fm := "---\nbrief: " + dhBoardID + "\ntitle: Brief 18\nwave: 1\neffort: M\n"
	if gateLine != "" {
		fm += gateLine + "\n"
	}
	fm += extra
	return fm + "---\n\n# Brief 18\n"
}

func dhRuledLines() string {
	return "decision-issue: 41\nruling: " + dhRuling + "\n"
}

func dhReadme(rows ...string) string {
	s := "---\nstream: " + dhStream + "\nstatus: active\npriority: P1\n---\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n"
	for _, r := range rows {
		s += r + "\n"
	}
	return s + "\n"
}

func dhRow(num, status string) string {
	return "| " + num + " | [brief-" + num + "](brief-" + num + ".md) | 1 | M | " + status + " | — | — |"
}

// dhFixture commits files (repo-relative path → content) as the base, pins it
// as origin/main, and leaves a feature branch checked out at the same commit.
func dhFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	dhWrite(t, root, files)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base board")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "feature")
	return root
}

// dhWrite writes the change; an empty content deletes the file.
func dhWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if content == "" {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			continue
		}
		mustMkdirAll(t, filepath.Dir(p))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const (
	dhReadmePath = "docs/streams/" + dhStream + "/README.md"
	dhBriefPath  = "docs/streams/" + dhStream + "/brief-18.md"
)

// dhForge is the all-conditions-hold forge for brief sdlc/18: a mapped human's
// unedited comment naming the board id, on decision issue #41 whose body
// carries the brief's marker.
func dhForge(commentText string) *rlForge {
	return &rlForge{
		commentStatus: 200,
		commentBody:   rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, commentText, rlPosted, rlPosted),
		issueStatus:   200,
		issueBody:     rlIssueJSON("A human decision is needed.\n\n" + dhMarker + "\n"),
	}
}

// dhSeams points layer two at forge (nil = no forge contact allowed) and the
// example login map, and the repository at rlRepo.
func dhSeams(t *testing.T, forge *rlForge) {
	t.Helper()
	oc, ol, or := decisionGateClientFn, decisionGateLoginsFn, repoFromOriginAtFn
	t.Cleanup(func() { decisionGateClientFn, decisionGateLoginsFn, repoFromOriginAtFn = oc, ol, or })
	if forge == nil {
		decisionGateClientFn = func() *ghClient {
			t.Errorf("the forge was contacted, but no fault passed layer one")
			return nil
		}
	} else {
		c := forge.client(t)
		decisionGateClientFn = func() *ghClient { return c }
	}
	decisionGateLoginsFn = func() map[string]string { return rlLogins }
	repoFromOriginAtFn = func(string) string { return rlRepo }
}

// dhGate runs `statusgen --decision-gate --decision-gate-base origin/main` on
// root and returns its exit code and report.
func dhGate(t *testing.T, root string) (int, string) {
	t.Helper()
	var code int
	out := captureStdout(t, func() { code = runDecisionGate(root, "", "refs/remotes/origin/main") })
	return code, out
}

func dhLayerOne(t *testing.T, root string) []string {
	t.Helper()
	problems, notices := decisionGateHoldProblems(root)
	if len(notices) != 0 {
		t.Fatalf("layer one emitted notices %v, want none — the fixture's base resolves", notices)
	}
	return problems
}

func wantContains(t *testing.T, what, got string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(got, s) {
			t.Errorf("%s does not contain %q:\n%s", what, s, got)
		}
	}
}

// unruledBase is the Verify 1 fixture: a gate: human brief at todo whose
// decision issue #41 is recorded but carries no ruling.
func unruledBase() map[string]string {
	return map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", "decision-issue: 41\n"),
	}
}

// Verify 1: a move to implemented with the decision issue unruled is refused by
// both layers, and the refusal names the issue and what is missing.
func TestDecisionGateVerify1UnruledMoveRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", "decision-issue: 41\nstatus: implemented\n"),
	})

	problems := dhLayerOne(t, root)
	if len(problems) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(problems), problems)
	}
	wantContains(t, "layer-one refusal", problems[0],
		dhReadmePath, "decision-gate hold", dhBoardID, "moves it to implemented (it was todo at the base)",
		"decision issue #41 has no recorded ruling", decisionGateLayerOneScope)

	code, out := dhGate(t, root)
	if code == 0 {
		t.Fatalf("--decision-gate exit 0, want non-zero:\n%s", out)
	}
	wantContains(t, "--decision-gate report", out, dhBoardID+" REFUSED", "#41", "no recorded ruling")
}

// Verify 2: the same move once the ruling is recorded is allowed by both layers.
func TestDecisionGateVerify2RuledMoveAllowed(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+", as written."))
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("layer one refused a recorded ruling: %v", p)
	}
	code, out := dhGate(t, root)
	if code != 0 {
		t.Fatalf("--decision-gate exit %d, want 0:\n%s", code, out)
	}
	wantContains(t, "--decision-gate report", out, dhBoardID+" RULED — ruled by "+rlHuman)
}

// Any ruling lifts the hold — one that holds the brief too. The check confirms
// THAT a human decided, not what.
func TestDecisionGateHoldingRulingAlsoLifts(t *testing.T) {
	dhSeams(t, dhForge(dhBoardID+": 4 — hold."))
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("a holding ruling did not lift the hold (exit %d):\n%s", code, out)
	}
}

// Verify 3: no decision issue at all is a refusal, not a pass.
func TestDecisionGateVerify3NoDecisionIssueRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", ""),
	})
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	problems := dhLayerOne(t, root)
	if len(problems) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(problems), problems)
	}
	wantContains(t, "layer-one refusal", problems[0], "no decision issue is recorded", "not a pass")
	code, out := dhGate(t, root)
	if code == 0 {
		t.Fatalf("--decision-gate exit 0, want non-zero:\n%s", out)
	}
	wantContains(t, "--decision-gate report", out, dhBoardID+" REFUSED", "no decision issue is recorded")
}

// Verify 4 and the amendment's BOT row: a well-formed link to a bot's relay of
// the ruling satisfies layer one, and layer two refuses it — the lower layer
// catching the fault with the upper layer satisfied. Perturbing the relay into
// the human's own comment turns the same fixture green.
func TestDecisionGateVerify4RelayRefusedHumanPasses(t *testing.T) {
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("layer one refused a well-formed link (it cannot see authors): %v", p)
	}

	relay := dhForge("Relaying the ruling: " + dhBoardID + " option 1.")
	relay.commentBody = rlCommentJSONWith("example-desk[bot]", "Bot", rlRepo, rlIssue, "Relaying the ruling: "+dhBoardID+" option 1.", rlPosted, rlPosted)
	dhSeams(t, relay)
	code, out := dhGate(t, root)
	if code == 0 {
		t.Fatalf("a bot relay passed layer two:\n%s", out)
	}
	wantContains(t, "--decision-gate report", out, dhBoardID+" REFUSED", string(rulingBotAuthor))

	dhSeams(t, dhForge("Ruling: "+dhBoardID+" option 1."))
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("the human's own comment did not pass (exit %d):\n%s", code, out)
	}
}

// Verify 5: the hold is scoped to gate: human — a gate: model brief moves freely.
func TestDecisionGateVerify5ModelGateAllowed(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: model", ""),
	})
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("layer one refused a gate: model move: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("--decision-gate exit %d on a gate: model move:\n%s", code, out)
	}
}

// Verify 9: a bare status-cell flip — the README row only, frontmatter untouched
// — is refused.
func TestDecisionGateVerify9BareCellFlipRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	if p := dhLayerOne(t, root); len(p) != 1 {
		t.Fatalf("layer one: %d problems on a bare cell flip, want 1: %v", len(p), p)
	}
	if code, out := dhGate(t, root); code == 0 {
		t.Fatalf("a bare status-cell flip passed:\n%s", out)
	}
}

// A row already at or past its new status at the base is left alone, and so is
// a move to a status the hold does not cover.
func TestDecisionGateLeavesLandedAndEarlyStatusesAlone(t *testing.T) {
	dhSeams(t, nil)
	for _, tc := range []struct{ from, to string }{
		{"implemented", "implemented"},
		{"done", "verified"}, // a demotion is not a move into a later status
		{"todo", "in-progress"},
	} {
		root := dhFixture(t, map[string]string{
			dhReadmePath: dhReadme(dhRow("18", tc.from)),
			dhBriefPath:  dhFM("gate: human", ""),
		})
		dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", tc.to))})
		if p := dhLayerOne(t, root); len(p) != 0 {
			t.Errorf("%s → %s: layer one refused: %v", tc.from, tc.to, p)
		}
	}
}

// A row that first appears already advanced is a move from "not on the board",
// and a move straight to done is in scope.
func TestDecisionGateNewRowAndStraightToDone(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{dhReadmePath: dhReadme()})
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "done")),
		dhBriefPath:  dhFM("gate: human", "decision-issue: 41\n"),
	})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "moves it to done (it was not on the board at the base)")
}

// Amendment: the gate relabelled away from human in the same change as the move
// is judged as gate: human.
func TestDecisionGateAmendRelabelWithMove(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: model", "decision-issue: 41\n"),
	})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "moves it to implemented", `relabels its gate from human to "model"`)
	if code, out := dhGate(t, root); code == 0 {
		t.Fatalf("relabel + move passed:\n%s", out)
	}
}

// Amendment: the relabel alone, in a first change, is refused; were it let
// through, the second change (the move) would see gate: model on both sides.
func TestDecisionGateAmendRelabelThenMove(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	for _, gate := range []string{"gate: model", ""} { // relabel, and deleting the key
		dhWrite(t, root, map[string]string{dhBriefPath: dhFM(gate, "decision-issue: 41\n")})
		p := dhLayerOne(t, root)
		if len(p) != 1 {
			t.Fatalf("relabel %q alone: %d problems, want 1: %v", gate, len(p), p)
		}
		if code, out := dhGate(t, root); code == 0 {
			t.Fatalf("relabel %q alone passed:\n%s", gate, out)
		}
	}
	// The second change, had the first landed: the gate is model on both sides,
	// which is exactly why the first change must be refused.
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "relabel landed")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("the later move with the gate absent on both sides is out of scope by design, got %v", p)
	}
}

// An unparseable frontmatter after the change cannot show the gate is still
// human, so it is judged as a relabel.
func TestDecisionGateUnparseableFrontmatterIsRelabel(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhBriefPath: "---\ngate: [human\n---\n"})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "frontmatter unparseable")
}

// Amendment: renumbered AND given a new permanent id, with a relabel and a move,
// in one change — both keys changed, so the base brief is dropped.
func TestDecisionGateAmendRenumberNewIDRelabelMove(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", "id: "+dhPermID+"\ndecision-issue: 41\n"),
	})
	dhWrite(t, root, map[string]string{
		dhBriefPath:  "",
		dhReadmePath: dhReadme(dhRow("19", "implemented")),
		"docs/streams/" + dhStream + "/brief-19.md": strings.Replace(dhFM("gate: model", "id: fresh-example-id\n"), dhBoardID, "sdlc/19", 1),
	})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], dhBoardID, "drops it", "nor its permanent id:")
	if code, out := dhGate(t, root); code == 0 {
		t.Fatalf("renumber + new id + relabel + move passed:\n%s", out)
	}
}

// Amendment: a renumber of a brief that has no permanent id drops it.
func TestDecisionGateAmendRenumberWithoutPermID(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhBriefPath:  "",
		dhReadmePath: dhReadme(dhRow("19", "todo")),
		"docs/streams/" + dhStream + "/brief-19.md": strings.Replace(dhFM("gate: human", "decision-issue: 41\n"), dhBoardID, "sdlc/19", 1),
	})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "drops it", "it has no permanent id:")
}

// A renumber that keeps the permanent id keeps the match, so the base's gate
// and status follow the brief — and a ruling that named the OLD board id still
// stands for it.
func TestDecisionGateRenumberKeepingPermIDKeepsRuling(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", "id: "+dhPermID+"\n"+dhRuledLines()),
	})
	dhWrite(t, root, map[string]string{
		dhBriefPath:  "",
		dhReadmePath: dhReadme(dhRow("19", "implemented")),
		"docs/streams/" + dhStream + "/brief-19.md": strings.Replace(dhFM("gate: model", "id: "+dhPermID+"\n"+dhRuledLines()), dhBoardID, "sdlc/19", 1),
	})
	faults := judgeDecisionGate(mustGateAtRev(t, root), gateSnapshotOnDisk(root))
	if len(faults) != 1 || !faults[0].Move || !faults[0].Relabel || faults[0].Drop {
		t.Fatalf("want one move+relabel fault matched by the permanent id, got %+v", faults)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("a ruling naming the old board id did not stand for the renumbered brief (exit %d):\n%s", code, out)
	}
}

// Amendment: deleting the brief drops it; deleting only its file leaves a row
// with no gate behind it, a relabel.
func TestDecisionGateAmendDeletion(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhBriefPath: "", dhReadmePath: dhReadme()})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("deletion: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "drops it")

	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "todo"))})
	p = dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("file-only deletion: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "removes its brief file")
}

// Archiving a stream, or moving a brief into done/, is not a drop.
func TestDecisionGateArchiveIsNotDrop(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "done")),
		dhBriefPath:  dhFM("gate: human", ""),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath: "", dhBriefPath: "",
		"docs/archive/" + dhStream + "/README.md":        dhReadme(dhRow("18", "done")),
		"docs/archive/" + dhStream + "/done/brief-18.md": dhFM("gate: human", ""),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("archiving a done brief was refused: %v", p)
	}
}

// Amendment: the relabel alone on a spec-trigger brief with no decision issue
// and no Human decision section yet is refused, and the refusal names the
// missing decision issue.
func TestDecisionGateAmendSpecTriggerRelabelNoIssue(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", "decision-trigger: spec\n"),
	})
	dhWrite(t, root, map[string]string{dhBriefPath: dhFM("gate: model", "decision-trigger: spec\n")})
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems, want 1: %v", len(p), p)
	}
	wantContains(t, "layer-one refusal", p[0], "relabels its gate", "no decision issue is recorded")
}

// Amendment: a human's comment that does not name the brief is refused, and a
// bare option number is refused with it.
func TestDecisionGateAmendCommentNotNamingBriefRefused(t *testing.T) {
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	for _, text := range []string{"1", "Ratified.", "Option 1 for sdlc/180."} {
		dhSeams(t, dhForge(text))
		code, out := dhGate(t, root)
		if code == 0 {
			t.Fatalf("comment %q passed:\n%s", text, out)
		}
		wantContains(t, "--decision-gate report", out, string(rulingRecordNotNamed))
	}
}

// Layer one: a malformed link, and a link to a comment on another issue.
func TestDecisionGateLayerOneLinkShape(t *testing.T) {
	dhSeams(t, nil)
	for _, tc := range []struct{ ruling, want string }{
		{"https://example.com/not-a-comment", "is not an issue-comment URL"},
		{rlURL(rlRepo, 42, rlComment), "links a comment on #42, not on the recorded decision issue #41"},
	} {
		root := dhFixture(t, unruledBase())
		dhWrite(t, root, map[string]string{
			dhReadmePath: dhReadme(dhRow("18", "implemented")),
			dhBriefPath:  dhFM("gate: human", "decision-issue: 41\nruling: "+tc.ruling+"\n"),
		})
		p := dhLayerOne(t, root)
		if len(p) != 1 {
			t.Fatalf("ruling %q: %d problems, want 1: %v", tc.ruling, len(p), p)
		}
		wantContains(t, "layer-one refusal", p[0], tc.want, decisionGateLayerOneScope)
	}
}

// Layer one says what it did NOT check, on every refusal.
func TestDecisionGateLayerOneSaysWellFormedOnly(t *testing.T) {
	if !strings.Contains(decisionGateLayerOneScope, "looked only for a well-formed ruling link") {
		t.Fatalf("the layer-one scope sentence no longer says it checked only for a well-formed link: %q", decisionGateLayerOneScope)
	}
}

// The decision issue's marker: the board id, the frontmatter brief: value
// verbatim (a house alias in its repository part), or a colon-form id naming
// this repository; a colon-form id naming another repository is refused.
func TestDecisionGateBriefIssueMarker(t *testing.T) {
	names := []string{dhBoardID}
	for _, tc := range []struct {
		body    string
		fm      []string
		matches bool
	}{
		{dhMarker, nil, true},
		{"<!-- decision-gate: example-repo:sdlc:18 -->", nil, true},
		{"<!-- decision-gate: example-repo:sdlc:018 -->", nil, true},
		{"<!-- decision-gate: other-repo:sdlc:18 -->", nil, false},
		{"<!-- decision-gate: house:ex:sdlc:18 -->", []string{"house:ex:sdlc:18"}, true},
		{"<!-- decision-gate: sdlc/19 -->", nil, false},
		{"no marker", nil, false},
	} {
		if got := briefIssueRulesOn(tc.body, rlRepo, names, tc.fm); got != tc.matches {
			t.Errorf("briefIssueRulesOn(%q, fm %v) = %v, want %v", tc.body, tc.fm, got, tc.matches)
		}
	}
}

// Amendment: the colon-form marker the brief declares verbatim passes layer two.
func TestDecisionGateColonFormMarkerAccepted(t *testing.T) {
	forge := dhForge("Option 1 for " + dhBoardID + ".")
	forge.issueBody = rlIssueJSON("decide\n\n<!-- decision-gate: house:ex:sdlc:18 -->\n")
	dhSeams(t, forge)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  strings.Replace(dhFM("gate: human", dhRuledLines()), "brief: "+dhBoardID, "brief: house:ex:sdlc:18", 1),
	})
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("colon-form marker refused (exit %d):\n%s", code, out)
	}
}

// The close to done lands with no pull request, so --close-verify runs both
// layers on the README it would write.
func TestDecisionGateCloseVerifyHold(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "verified")),
		dhBriefPath:  dhFM("gate: human", "decision-issue: 41\n"),
	})
	err := closeVerifyDecisionGate(root, filepath.Join(root, filepath.FromSlash(dhReadmePath)), []byte(dhReadme(dhRow("18", "done"))))
	if err == nil {
		t.Fatal("the close to done of an unruled gate: human brief was not refused")
	}
	wantContains(t, "close-verify refusal", err.Error(), "refusing: decision-gate hold", dhBoardID+" REFUSED", "#41")

	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	dhWrite(t, root, map[string]string{dhBriefPath: dhFM("gate: human", dhRuledLines())})
	if err := closeVerifyDecisionGate(root, dhReadmePath, []byte(dhReadme(dhRow("18", "done")))); err != nil {
		t.Fatalf("a ruled close was refused: %v", err)
	}
}

// The PR lane with no resolvable merge-base fails closed when the PR touches the
// board, or when a listing naming no board file cannot be shown complete; a
// complete listing that leaves the board alone is inert.
func TestDecisionGatePRLaneUnresolvedBase(t *testing.T) {
	count := func(n int, err error) func() (int, error) { return func() (int, error) { return n, err } }
	board := []ghPRFile{{Filename: dhReadmePath}}
	other := []ghPRFile{{Filename: "cmd/x.go"}}

	if rs := decisionGatePRLane(".", rlRepo, board, "", count(1, nil)); !decisionGateRefused(rs) {
		t.Errorf("board touched, base unresolved: not refused: %+v", rs)
	}
	if rs := decisionGatePRLane(".", rlRepo, []ghPRFile{{Filename: "x", PreviousFilename: "docs/archive/s/README.md"}}, "", count(1, nil)); !decisionGateRefused(rs) {
		t.Errorf("board renamed away, base unresolved: not refused: %+v", rs)
	}
	if rs := decisionGatePRLane(".", rlRepo, other, "", count(3, nil)); !decisionGateRefused(rs) {
		t.Errorf("truncated listing: not refused: %+v", rs)
	}
	if rs := decisionGatePRLane(".", rlRepo, other, "", count(0, errors.New("boom"))); !decisionGateRefused(rs) {
		t.Errorf("unreadable count: not refused: %+v", rs)
	}
	if rs := decisionGatePRLane(".", rlRepo, other, "", count(1, nil)); len(rs) != 0 {
		t.Errorf("complete listing, board untouched: got %+v, want nothing", rs)
	}
}

// The PR lane with a resolved merge-base judges the checkout against it.
func TestDecisionGatePRLaneJudgesAgainstMergeBase(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	baseOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	gitRun(t, root, "commit", "-q", "-am", "flip")
	rs := decisionGatePRLane(root, rlRepo, []ghPRFile{{Filename: dhReadmePath}}, base, nil)
	if !decisionGateRefused(rs) {
		t.Fatalf("PR lane did not refuse the unruled move: %+v", rs)
	}
}

func TestDecisionGateUsage(t *testing.T) {
	for _, tc := range []struct{ prs, base string }{{"", ""}, {"1", "HEAD"}} {
		if code := runDecisionGate(".", tc.prs, tc.base); code != 2 {
			t.Errorf("runDecisionGate(prs=%q, base=%q) = %d, want 2", tc.prs, tc.base, code)
		}
	}
}

// An unresolvable base revision fails closed.
func TestDecisionGateBadBaseFailsClosed(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	var code int
	out := captureStdout(t, func() { code = runDecisionGate(root, "", "refs/no/such") })
	if code == 0 {
		t.Fatalf("an unresolvable base passed:\n%s", out)
	}
}

// Layer one with no resolvable origin/main emits a NOTICE, never a pass that
// looks like a clean run.
func TestDecisionGateLayerOneUnresolvedBaseNotice(t *testing.T) {
	root := dhFixture(t, unruledBase())
	gitRun(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	problems, notices := decisionGateHoldProblems(root)
	if len(problems) != 0 || len(notices) != 1 || !strings.Contains(notices[0], "did not run") {
		t.Fatalf("got problems %v notices %v, want one did-not-run notice", problems, notices)
	}
}

func mustGateAtRev(t *testing.T, root string) gateSnapshot {
	t.Helper()
	s, err := gateSnapshotAtRev(root, "refs/remotes/origin/main")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The hold is wired into `statusgen --lint`: an unruled move turns the lint red
// and the lint names the hold.
func TestDecisionGateLintWiring(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	var code int
	stdout, stderr := captureBoth(t, func() { code = run(root, "lint", nil, nil, "") })
	if code == 0 {
		t.Fatalf("--lint passed an unruled move:\n%s\n%s", stdout, stderr)
	}
	wantContains(t, "--lint output", stdout+stderr, "decision-gate hold (lifecycle-v1 §4.5)", dhBoardID)
}

// The hold is wired into --close-verify: the close to done of an unruled
// gate: human brief is refused and nothing is written.
func TestDecisionGateCloseVerifyWiring(t *testing.T) {
	old := closeVerifyDecisionGateFn
	closeVerifyDecisionGateFn = closeVerifyDecisionGate
	t.Cleanup(func() { closeVerifyDecisionGateFn = old })
	dhSeams(t, nil)
	root, _ := loadVGStreams(t)
	err := closeVerify(root, "vg/01", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("close-verify closed an unruled gate: human brief to done")
	}
	wantContains(t, "close-verify refusal", err.Error(), "decision-gate hold", "vg/01 REFUSED", "no decision issue is recorded")
	if got := vgRowStatus(t, root, "01"); got != "verified" {
		t.Errorf("vg/01 = %q after a refused close, want verified (nothing written)", got)
	}
}

// The hold is wired into `statusgen --corroborate --pr`: a PR that moves an
// unruled gate: human brief fails the pass, and the report carries the lane.
func TestDecisionGateCorroborateWiring(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	gitRun(t, root, "commit", "-q", "-am", "flip")
	st := &corroborateStub{files: []ghPRFile{{Filename: dhReadmePath, Patch: "@@ -9 +9 @@\n-" + dhRow("18", "todo") + "\n+" + dhRow("18", "implemented")}}, mergeBase: originMain(t, root)}
	var rc int
	out := captureStdout(t, func() { rc = runCorroborateOn(t, root, st) })
	if rc != 1 {
		t.Fatalf("--corroborate passed an unruled move (rc=%d):\n%s", rc, out)
	}
	wantContains(t, "--corroborate report", out, "# decision-gate hold", "PR #7: "+dhBoardID+" REFUSED")
}

// verify-gate-close.yml keys its reopen-and-relay branch on the literal phrase
// every close-verify hold refusal carries; the two must not drift apart.
func TestDecisionGateCloseWorkflowKeyedPhrase(t *testing.T) {
	const phrase = "decision-gate hold"
	// The edit surface is the staged twin while it is pending promotion (no App may
	// push a .github/workflows/* change); once a maintainer promotes it and the twin
	// is gone, the live file carries the branch.
	wfPath := filepath.Join("..", "ci", "staged-workflows", "verify-gate-close.yml")
	wf, err := os.ReadFile(wfPath)
	if os.IsNotExist(err) {
		wfPath = filepath.Join("..", ".github", "workflows", "verify-gate-close.yml")
		wf, err = os.ReadFile(wfPath)
	}
	if err != nil {
		t.Skipf("workflow not present in this tree: %v", err)
	}
	if !strings.Contains(string(wf), "grep -q '"+phrase+"'") {
		t.Fatalf("%s no longer keys a branch on %q", wfPath, phrase)
	}
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	err = closeVerifyDecisionGate(root, dhReadmePath, []byte(dhReadme(dhRow("18", "done"))))
	if err == nil || !strings.Contains(err.Error(), phrase) {
		t.Fatalf("the close-verify refusal does not carry %q: %v", phrase, err)
	}
}
