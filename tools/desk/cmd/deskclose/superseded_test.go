package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// superseded_test.go — the two-role superseded lane.
//
// Four properties, each with its positive control:
//
//	1. a WORKER token PROPOSES and never closes — whatever flags it is handed;
//	2. a REVIEWER token CONFIRMS: closes, with the verdict on the item and the
//	   back-reference on the target — but only a proposal made by a DIFFERENT actor,
//	   naming the SAME target, whose target is MERGED;
//	3. a REVIEWER token DISPUTES: posts the reason, applies needs-decision, never closes
//	   — and from then on every close is refused;
//	4. the role comes from the TOKEN's roster binding — a login bound to neither role
//	   is refused, an unreadable login is could-not-check, a roster binding both roles
//	   to one App is refused — and the manifest lane, authorized by a human digest,
//	   is NOT role-keyed.
//
// The identities are the fixture roster's App slugs (rosterfixture_test.go): the
// `worker=` and `reviewer=` bindings, rendered as the forge renders them.

const (
	workerLogin   = "assay-worker-app[bot]"
	reviewerLogin = "assay-reviewer-app[bot]"
	deskAppLogin  = "assay-desk-app[bot]"
)

// plantProposal appends a worker-shaped proposal comment to the item's thread, with
// the given forge-attested author.
func (s *stubRemote) plantProposal(repo string, n int, author, target string) {
	body := proposalMarker + "\nSuperseded-By: " + target + "\nProposed-By: " + author + "\nProposed-At: 2026-08-30\n\nproposal text\n"
	s.plantComment(repo, n, author, body)
}

func (s *stubRemote) plantComment(repo string, n int, author, body string) {
	entry, _ := json.Marshal(map[string]any{"body": body, "user": map[string]string{"login": author}})
	key := repo + "#" + fmt.Sprint(n)
	s.threads[key] = append(s.threads[key], string(entry))
}

// execErr runs the dispatcher and returns the ERROR, so a test can assert on WHICH
// refusal fired rather than only on the exit code. Two refusals sharing an exit code are
// indistinguishable to execCLI, and the mutation harness needs the difference.
func execErr(args ...string) error {
	var out strings.Builder
	return dispatch(args, &out)
}

// prWorld is a PR subject (#90, open) with its disposition record naming the merged PR
// #40, and no proposal yet — the state a worker is in when it runs propose.
func prWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[testRepo+"#90"] = prIssueJSON(90, "open")
	s.pulls[testRepo+"#90"] = pullJSON(90, "open", false)
	s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#40")
	return s, rul
}

func onlyWrites(s *stubRemote, verb string) [][]string {
	var out [][]string
	for _, w := range s.writes() {
		if w[1] == verb {
			out = append(out, w)
		}
	}
	return out
}

func hasWrite(s *stubRemote, want ...string) bool {
	for _, w := range s.writes() {
		all := true
		for _, tok := range want {
			if !contains(w, tok) {
				all = false
			}
		}
		if all {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- 1. worker proposes

func TestSupersededWorkerProposes(t *testing.T) {
	nowFunc = func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = time.Now })

	t.Run("propose labels and comments, and does NOT close", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("a WORKER token closed its own item: %v", s.writes())
		}
		if !hasWrite(s, "pr", "edit", "--add-label", labelProposed) {
			t.Fatalf("the proposal must apply %q: %v", labelProposed, s.writes())
		}
		body := commentBody(t, s)
		for _, want := range []string{proposalMarker, "Superseded-By: " + testRepo + "#40", "Proposed-By: " + workerLogin, "Proposed-At: 2026-08-30", verdictSuperseded} {
			if !strings.Contains(body, want) {
				t.Fatalf("the proposal comment must carry %q:\n%s", want, body)
			}
		}
		if !strings.Contains(out, "proposed") || !strings.Contains(out, "did not close") {
			t.Fatalf("the operator must be told this was a proposal, not a close:\n%s", out)
		}
		// Label first, comment second — the index before the record.
		w := s.writes()
		var iEdit, iComment int = -1, -1
		for i, c := range w {
			if c[1] == "edit" && iEdit < 0 {
				iEdit = i
			}
			if c[1] == "comment" && iComment < 0 {
				iComment = i
			}
		}
		if iEdit < 0 || iComment < 0 || iEdit > iComment {
			t.Fatalf("the label must land before the proposal comment: %v", w)
		}
	})

	t.Run("a standing proposal for the same target is an idempotent no-op", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		s.items[testRepo+"#90"] = `{"number":90,"title":"stub pr","state":"open","body":"","labels":[{"name":"` + labelProposed + `"}],"pull_request":{"merged_at":null}}`
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("a re-run over a standing proposal wrote: %v", got)
		}
		if !strings.Contains(out, "noop") {
			t.Fatalf("want a noop line:\n%s", out)
		}
	})

	t.Run("the finding comes first: a PR with no disposition record is refused", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedClean, "", "")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("proposed with no finding recorded: %v", got)
		}
	})

	t.Run("a target that is closed-unmerged cannot be proposed", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#41")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#41", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("proposed a supersession by work that never landed: %v", got)
		}
	})

	t.Run("an OPEN target may be proposed (the close waits for its merge)", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		s.items[testRepo+"#77"] = prIssueJSON(77, "open")
		s.pulls[testRepo+"#77"] = pullJSON(77, "open", false)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#77")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#77", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatal("a proposal closed something")
		}
	})

	t.Run("--dispute under a worker token is refused with no write", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = workerLogin
		err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "not mine to say", "--rulings", rul)
		if err == nil || !deskkit.IsRefused(err) {
			t.Fatalf("want a refusal, got %v", err)
		}
		if !strings.Contains(err.Error(), "only PROPOSES") {
			t.Fatalf("the refusal must name the role boundary: %v", err)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("a worker's dispute wrote: %v", got)
		}
	})

	t.Run("an issue subject is proposable too (issue edit, not pr edit)", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.viewer = workerLogin
		s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] = issueJSON(subjectIssue, "open", nil, "")
		delete(s.threads, testRepo+"#"+fmt.Sprint(subjectIssue))
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue), "--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if !hasWrite(s, "issue", "edit", "--add-label", labelProposed) || s.closes() != 0 {
			t.Fatalf("want an issue label write and no close: %v", s.writes())
		}
	})
}

// ---------------------------------------------------------------- 2. reviewer confirms

func TestSupersededReviewerConfirms(t *testing.T) {
	t.Run("confirm closes, with the verdict on the item and the back-reference on the target", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertConfirmedClose(t, s, reasonNotPlanned)
		comments := onlyWrites(s, "comment")
		verdict, backref := comments[0][len(comments[0])-1], comments[1][len(comments[1])-1]
		for _, want := range []string{verdictMarker, "Superseded-Verdict: " + verdictConfirmed, "Proposed-By: " + workerLogin, "Verdict-By: " + reviewerLogin, rulingID} {
			if !strings.Contains(verdict, want) {
				t.Fatalf("the verdict comment must carry %q:\n%s", want, verdict)
			}
		}
		if comments[1][2] != "40" {
			t.Fatalf("the back-reference must land on the target #40: %v", comments[1])
		}
		if !strings.Contains(backref, "Supersedes "+testRepo+"#90") || !strings.Contains(backref, verdictConfirmed) {
			t.Fatalf("the back-reference must name the superseded item and the verdict:\n%s", backref)
		}
	})

	t.Run("no standing proposal: nothing to confirm, no write", func(t *testing.T) {
		s, rul := prWorld(t)
		err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if err == nil || !deskkit.IsRefused(err) {
			t.Fatalf("want a refusal, got %v", err)
		}
		if !strings.Contains(err.Error(), "no standing proposal") {
			t.Fatalf("the refusal must say there is nothing to confirm: %v", err)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("a reviewer originated a supersession: %v", got)
		}
	})

	t.Run("the same actor cannot propose AND confirm", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, reviewerLogin, testRepo+"#40")
		err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if err == nil || !deskkit.IsRefused(err) {
			t.Fatalf("want a refusal, got %v", err)
		}
		if !strings.Contains(err.Error(), "same identity") {
			t.Fatalf("the refusal must name the single-actor shape: %v", err)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("one identity proposed and confirmed: %v", got)
		}
	})

	t.Run("the forge-attested author decides, not the self-reported Proposed-By line", func(t *testing.T) {
		s, rul := prWorld(t)
		// The comment CLAIMS the worker wrote it; the forge says the reviewer did.
		body := proposalMarker + "\nSuperseded-By: " + testRepo + "#40\nProposed-By: " + workerLogin + "\nProposed-At: 2026-08-30\n"
		s.plantComment(testRepo, 90, reviewerLogin, body)
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("a self-reported author laundered a single-actor close")
		}
	})

	t.Run("the proposal's target and the caller's --by must agree", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#999")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("deskclose picked a winner between the proposal and the caller: %v", got)
		}
	})

	t.Run("the last proposal wins: a re-proposal against the right target confirms", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#999")
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertConfirmedClose(t, s, reasonNotPlanned)
	})

	t.Run("a proposal quoted inside a code fence is documentation, not a proposal", func(t *testing.T) {
		s, rul := prWorld(t)
		body := "here is the schema:\n```\n" + proposalMarker + "\nSuperseded-By: " + testRepo + "#40\n```\n"
		s.plantComment(testRepo, 90, workerLogin, body)
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused || s.closes() != 0 {
			t.Fatalf("a fenced example was read as a live proposal (exit %d, closes %d)", code, s.closes())
		}
	})

	t.Run("an unreadable thread is could-not-check, never 'no proposal'", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		inner := runGH
		runGH = func(args ...string) (string, error) {
			if args[0] == "api" && strings.Contains(args[len(args)-1], "/comments?") {
				s.calls = append(s.calls, args)
				return "", fmt.Errorf("HTTP 502")
			}
			return inner(args...)
		}
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("wrote on an unread thread: %v", got)
		}
	})

	t.Run("the target must be MERGED to confirm (closed-unmerged never satisfies)", func(t *testing.T) {
		s, rul := prWorld(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#41")
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#41")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#41", "--rulings", rul)
		if code != deskkit.ExitRefused || s.closes() != 0 {
			t.Fatalf("confirmed a supersession by work that never landed (exit %d, closes %d)", code, s.closes())
		}
	})

	t.Run("an OPEN target refuses the confirm — the proposal parks until it merges", func(t *testing.T) {
		s, rul := prWorld(t)
		s.items[testRepo+"#77"] = prIssueJSON(77, "open")
		s.pulls[testRepo+"#77"] = pullJSON(77, "open", false)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#77")
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#77")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#77", "--rulings", rul)
		if code != deskkit.ExitRefused || s.closes() != 0 {
			t.Fatalf("closed in favour of a PR that has not landed (exit %d, closes %d)", code, s.closes())
		}
	})

	t.Run("the ruling gate still binds the confirm's close", func(t *testing.T) {
		s, _ := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		rul := signedRulings(t, "_(empty)_")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused || len(s.writes()) != 0 {
			t.Fatalf("a reviewer confirm closed without the fetched human authorization (exit %d, writes %v)", code, s.writes())
		}
	})
}

// ---------------------------------------------------------------- 3. reviewer disputes

func TestSupersededReviewerDisputes(t *testing.T) {
	t.Run("dispute posts the reason, applies needs-decision, and does NOT close", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40",
			"--dispute", "the target lacks the migration this PR carried", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("a dispute closed the item: %v", s.writes())
		}
		if !hasWrite(s, "pr", "edit", "--add-label", labelNeedsDecision) {
			t.Fatalf("the dispute must apply %q: %v", labelNeedsDecision, s.writes())
		}
		body := commentBody(t, s)
		for _, want := range []string{verdictMarker, "Superseded-Verdict: " + verdictDisputed, "Reason: the target lacks the migration", "Verdict-By: " + reviewerLogin, labelNeedsDecision} {
			if !strings.Contains(body, want) {
				t.Fatalf("the dispute comment must carry %q:\n%s", want, body)
			}
		}
		// Comment first, label second — a bare escalation label is unanswerable.
		w := s.writes()
		if w[0][1] != "comment" || w[len(w)-1][1] != "edit" {
			t.Fatalf("the reason must land before the label: %v", w)
		}
		if !strings.Contains(out, "human-only close") {
			t.Fatalf("the operator must be told the item is the human's now:\n%s", out)
		}
	})

	t.Run("after a dispute, a confirm is refused — the decision label gate holds", func(t *testing.T) {
		s, rul := prWorld(t)
		s.items[testRepo+"#90"] = `{"number":90,"title":"stub pr","state":"open","body":"","labels":[{"name":"` + labelProposed + `"},{"name":"` + labelNeedsDecision + `"}],"pull_request":{"merged_at":null}}`
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused || len(s.writes()) != 0 {
			t.Fatalf("a disputed item was closed past the decision label (exit %d, writes %v)", code, s.writes())
		}
	})

	t.Run("a disputed item is a no-op to re-dispute", func(t *testing.T) {
		s, rul := prWorld(t)
		s.items[testRepo+"#90"] = `{"number":90,"title":"stub pr","state":"open","body":"","labels":[{"name":"` + labelNeedsDecision + `"}],"pull_request":{"merged_at":null}}`
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "again", "--rulings", rul)
		if code != deskkit.ExitOK || len(s.writes()) != 0 || !strings.Contains(out, "noop") {
			t.Fatalf("re-disputing must be a silent no-op (exit %d, writes %v)\n%s", code, s.writes(), out)
		}
	})

	t.Run("nothing to dispute without a standing proposal", func(t *testing.T) {
		s, rul := prWorld(t)
		err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "x", "--rulings", rul)
		if err == nil || !deskkit.IsRefused(err) || !strings.Contains(err.Error(), "no standing proposal") {
			t.Fatalf("want the no-proposal refusal, got %v", err)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("disputed nothing, and wrote: %v", got)
		}
	})

	t.Run("a dispute cannot reopen a closed item", func(t *testing.T) {
		s, rul := prWorld(t)
		s.items[testRepo+"#90"] = prIssueJSON(90, "closed")
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "x", "--rulings", rul)
		if code != deskkit.ExitRefused || len(s.writes()) != 0 {
			t.Fatalf("want a refusal and no write (exit %d, writes %v)", code, s.writes())
		}
	})

	t.Run("a failed label write is reported as could-not-check, not swallowed", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		inner := runGH
		runGH = func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == "edit" {
				s.calls = append(s.calls, args)
				return "", fmt.Errorf("could not add label: 'needs-decision' not found")
			}
			return inner(args...)
		}
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "x", "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
	})

	t.Run("dry-run disputes nothing", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--dispute", "x", "--rulings", rul, "--dry-run")
		if code != deskkit.ExitOK || len(s.writes()) != 0 || !strings.Contains(out, "dry-run") {
			t.Fatalf("dry-run wrote or did not say so (exit %d, writes %v)\n%s", code, s.writes(), out)
		}
	})
}

// TestDisputeLabelIsCloseRefused pins the property the dispute path rests on: the label
// it applies is one the close gate refuses in every mode. If someone renames one side
// without the other, a disputed item silently becomes closeable again.
func TestDisputeLabelIsCloseRefused(t *testing.T) {
	if !contains(decisionLabels, labelNeedsDecision) {
		t.Fatalf("the dispute applies %q but the close gate refuses only %v — a disputed item would be closeable",
			labelNeedsDecision, decisionLabels)
	}
}

// ---------------------------------------------------------------- 4. role from the binding

func TestSupersededRoleFromTokenBinding(t *testing.T) {
	t.Run("a login bound to neither role is refused, no write", func(t *testing.T) {
		for _, login := range []string{sharedLogin, blessLogin, deskAppLogin, "app/assay-desk-app"} {
			s, rul := prWorld(t)
			s.viewer = login
			s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
			err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
			if err == nil || !deskkit.IsRefused(err) {
				t.Fatalf("%s: want a refusal, got %v", login, err)
			}
			if !strings.Contains(err.Error(), "neither") {
				t.Fatalf("%s: the refusal must say the login is bound to neither role: %v", login, err)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("%s: an unbound identity wrote: %v", login, got)
			}
		}
	})

	t.Run("an unreadable identity is could-not-check, never a role", func(t *testing.T) {
		s, rul := prWorld(t)
		s.failViewer = true
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("acted on an unread identity: %v", got)
		}
	})

	t.Run("an empty login from the forge is could-not-check", func(t *testing.T) {
		s, rul := prWorld(t)
		s.viewer = ""
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable || len(s.writes()) != 0 {
			t.Fatalf("an empty identity was resolved to a role (exit %d, writes %v)", code, s.writes())
		}
	})

	t.Run("the roster's REST and gh-CLI renderings of the App both resolve", func(t *testing.T) {
		for _, login := range []string{reviewerLogin, "app/assay-reviewer-app", "ASSAY-REVIEWER-APP[bot]"} {
			s, rul := prWorld(t)
			s.viewer = login
			s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
			code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
			if code != deskkit.ExitOK {
				t.Fatalf("%s: want exit 0, got %d\n%s", login, code, out)
			}
		}
	})

	t.Run("an unbound reviewer role is could-not-check", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		plantRoster(t, strings.Replace(fixtureRoster, "reviewer=assay-reviewer-app:300000004,", "", 1))
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable || len(s.writes()) != 0 {
			t.Fatalf("an unbound role was compared against (exit %d, writes %v)", code, s.writes())
		}
	})

	t.Run("a roster binding both roles to ONE App is refused — the lane cannot be two-role", func(t *testing.T) {
		s, rul := prWorld(t)
		s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
		plantRoster(t, strings.Replace(fixtureRoster, "worker=assay-worker-app:300000006", "worker=assay-reviewer-app:300000004", 1))
		err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if err == nil || !deskkit.IsRefused(err) || !strings.Contains(err.Error(), "same App") {
			t.Fatalf("want the one-App refusal, got %v", err)
		}
		if got := s.writes(); len(got) != 0 {
			t.Fatalf("a one-App roster ran the lane: %v", got)
		}
	})

	t.Run("the manifest lane is NOT role-keyed: a human digest authorizes, whatever the token", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.viewer = workerLogin
		mf := writeManifest(t, manifestURL, "", []string{
			fmt.Sprintf("  - issue: %d\n    mode: superseded\n    target: \"#%d\"", subjectIssue, mergedPRNum),
		})
		authorizeStub(t, s, mf)
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		for _, c := range s.calls {
			if c[0] == "api" && len(c) > 1 && c[1] == "graphql" {
				t.Fatal("the manifest lane consulted the token's identity — its authority is the human digest")
			}
		}
	})
}

// plantRoster installs a roster VARIANT for one test and restores the fixture after.
func plantRoster(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
