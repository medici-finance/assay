package main

// kind_test.go — deskclose on a project that numbers issues and changes SEPARATELY.
//
// THE DEFECT THIS FILE PINS. `deskclose superseded` took a bare `<N>` and had no way to say
// which kind of object that number named. On a forge with one number sequence there is
// nothing to say; on one with two, `#3` and `!3` are different objects, the untyped read
// refuses (rightly — picking one would route a verdict at the wrong object), and the refusal
// pointed the caller at "the typed operation for the kind you mean", which deskclose did not
// expose. The supersession lane was therefore unreachable in BOTH roles: the worker could not
// post the proposal and the reviewer could not confirm it.
//
// Three properties, and the third is the one that would have been a WRONG WRITE rather than a
// failed one:
//
//  1. a bare number on a collision still fails closed — and the refusal now names the typed
//     forms that exist, instead of an operation that did not;
//  2. the typed forms reach each kind: `!N` and `--kind mr` address the change, `--kind issue`
//     addresses the issue, and every read and write in the lane follows the kind that was
//     stated rather than resolving the number again;
//  3. confirming a superseded CHANGE closes the change. The untyped close addresses the ISSUE
//     sequence, so on a project carrying both it would have closed the issue that merely
//     shares the number — and left the change open.
//
// The single-sequence forge is covered too: a bare number must keep working exactly as it
// did, through the UNTYPED read, or the fix would have broken every existing caller.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The collision world's numbers. `collideNum` is the number that names two objects.
const (
	collideNum = 3 // issue #3 AND merge request !3 — different objects
	supersedes = 4 // merge request !4, merged, the one that supersedes
)

// changeKey is the fixture key a change occupies on a project with separate sequences.
func changeKey(repo string, n int) string { return fmt.Sprintf("%s!%d", repo, n) }

// plantProposalOn appends a worker-shaped proposal to the thread registered under ONE fixture
// key, so a test can put a proposal on the change at a number and leave the issue at the same
// number without one (and the reverse).
func (s *stubRemote) plantProposalOn(key, author, target string) {
	body := proposalMarker + "\nSuperseded-By: " + target + "\nProposed-By: " + author +
		"\nProposed-At: 2026-08-30\n\nproposal text\n"
	entry, _ := json.Marshal(map[string]any{"body": body, "user": map[string]string{"login": author}})
	s.threads[key] = append(s.threads[key], string(entry))
}

// collisionWorld is a project whose issue and change sequences are SEPARATE:
//
//	#3  an OPEN ISSUE
//	!3  an OPEN MERGE REQUEST — a different object that happens to share the number
//	!4  a MERGED merge request, the one that supersedes
//
// Every other fixture is baseWorld's, so the ruling gate and the roster are unchanged and the
// only thing these tests vary is the numbering.
func collisionWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, collideNum)] = issueJSON(collideNum, "open", nil, "")
	s.items[changeKey(testRepo, collideNum)] = prIssueJSON(collideNum, "open")
	s.pulls[changeKey(testRepo, collideNum)] = pullJSON(collideNum, "open", false)
	s.items[changeKey(testRepo, supersedes)] = prIssueJSON(supersedes, "closed")
	s.pulls[changeKey(testRepo, supersedes)] = pullJSON(supersedes, "closed", true)
	// The change at the colliding number is a PULL REQUEST, so the lane requires its
	// disposition record before a worker may propose anything about it.
	s.disps[fmt.Sprintf("%s#%d", testRepo, collideNum)] =
		dispJSON(dispCheckedFailed, verdictSuperseded, fmt.Sprintf("!%d", supersedes))
	return s, rul
}

// TestCollisionBareNumberStillFailsClosedAndNamesTheTypedForms is the reported failure,
// restated as a check. The refusal itself is correct and must stay: resolving a bare number to
// the wrong kind would route a close at the wrong object. What was missing is the second half
// — the refusal has to name an operation that EXISTS.
func TestCollisionBareNumberStillFailsClosedAndNamesTheTypedForms(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = workerLogin

	err := execErr(modeSuperseded, "-R", testRepo, fmt.Sprint(collideNum),
		"--by", fmt.Sprintf("!%d", supersedes), "--rulings", rul)
	if err == nil {
		t.Fatal("a bare number naming two different objects must stay a refusal — " +
			"resolving it would act on whichever object the backend happened to answer with")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("want could-not-check (exit 6), got %v (exit %d)", err, deskkit.ExitCodeOf(err))
	}
	msg := err.Error()
	for _, want := range []string{
		"carries BOTH issue", // the forge's own diagnosis survives
		"!N",                 // and the refusal now names a form that exists
		"--kind issue",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the refusal does not mention %q — an operator reading it still cannot act on it:\n%s",
				want, msg)
		}
	}
	if got := s.writes(); len(got) != 0 {
		t.Fatalf("a refusal wrote to the forge: %v", got)
	}
}

// TestTypedChangeReferenceReachesTheChange: `!N` addresses the merge request, and every read
// and write in the worker's half follows that kind. The proposal must land on the CHANGE — a
// proposal posted on the issue sharing its number is invisible to the reviewer looking at the
// change, which is the same unreachable lane wearing a different hat.
func TestTypedChangeReferenceReachesTheChange(t *testing.T) {
	for _, form := range []struct {
		name string
		args []string
	}{
		{"sigil", []string{fmt.Sprintf("!%d", collideNum)}},
		{"kind flag", []string{fmt.Sprint(collideNum), "--kind", "mr"}},
		{"pr alias", []string{fmt.Sprint(collideNum), "--kind", "pr"}},
	} {
		t.Run(form.name, func(t *testing.T) {
			s, rul := collisionWorld(t)
			s.viewer = workerLogin

			args := append([]string{modeSuperseded, "-R", testRepo}, form.args...)
			args = append(args, "--by", fmt.Sprintf("!%d", supersedes), "--rulings", rul)
			if err := execErr(args...); err != nil {
				t.Fatalf("the typed form must resolve where the bare number cannot: %v", err)
			}
			wantKind := fmt.Sprintf("%s#%d:%s", testRepo, collideNum, deskkit.TargetChange)
			assertContains(t, "typed read", s.typedGets, wantKind)
			assertContains(t, "typed comment", s.typedComments, wantKind)
			assertContains(t, "typed thread read", s.typedThreads, wantKind)
			// The label is the index the review queue filters on. It has to be on the object
			// the proposal is about.
			if got := onlyWrites(s, "edit"); len(got) != 1 || got[0][0] != "pr" {
				t.Fatalf("the superseded? label went somewhere other than the change: %v", got)
			}
		})
	}
}

// TestTypedIssueReferenceReachesTheIssue is the other half: with `--kind issue` the same
// number addresses the ISSUE, and the thread that is read is the issue's own. Reading the
// change's thread here would find the change's proposal and answer a question about a
// different object.
func TestTypedIssueReferenceReachesTheIssue(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = workerLogin
	// Only the CHANGE at this number carries a proposal. If the issue's thread read were
	// routed at the change, the run would report "a proposal already stands" instead of
	// posting one.
	s.plantProposalOn(changeKey(testRepo, collideNum), workerLogin, fmt.Sprintf("!%d", supersedes))

	if err := execErr(modeSuperseded, "-R", testRepo, fmt.Sprint(collideNum), "--kind", "issue",
		"--by", fmt.Sprintf("!%d", supersedes), "--rulings", rul); err != nil {
		t.Fatalf("--kind issue must resolve the issue at a colliding number: %v", err)
	}
	wantKind := fmt.Sprintf("%s#%d:%s", testRepo, collideNum, deskkit.TargetIssue)
	assertContains(t, "typed read", s.typedGets, wantKind)
	assertContains(t, "typed thread read", s.typedThreads, wantKind)
	if got := onlyWrites(s, "edit"); len(got) != 1 || got[0][0] != "issue" {
		t.Fatalf("the superseded? label went somewhere other than the issue: %v", got)
	}
}

// TestConfirmClosesTheChangeNotTheIssueSharingItsNumber is the property whose absence was a
// WRONG WRITE. The untyped close addresses the issue sequence; with a change and an issue at
// one number it closes the issue and leaves the change open, and the audit trail records a
// successful close of the item the caller never named.
func TestConfirmClosesTheChangeNotTheIssueSharingItsNumber(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = reviewerLogin
	s.plantProposalOn(changeKey(testRepo, collideNum), workerLogin, fmt.Sprintf("!%d", supersedes))

	if err := execErr(modeSuperseded, "-R", testRepo, fmt.Sprintf("!%d", collideNum),
		"--by", fmt.Sprintf("!%d", supersedes), "--rulings", rul); err != nil {
		t.Fatalf("the reviewer's confirm must resolve the typed form: %v", err)
	}
	// The proof that matters is on the OTHER object: the issue at the same number must be
	// untouched. A close routed by number alone closes exactly this, reports success, and
	// leaves the change the caller named open — so this assertion comes FIRST, because it is
	// the one whose failure is a wrong write rather than an absent one.
	issue := s.items[fmt.Sprintf("%s#%d", testRepo, collideNum)]
	if !strings.Contains(issue, `"state":"open"`) {
		t.Fatalf("the ISSUE sharing the change's number was closed — the close was routed by number, "+
			"not by kind: %s", issue)
	}
	if !strings.Contains(s.items[changeKey(testRepo, collideNum)], `"state":"closed"`) {
		t.Fatalf("the change the caller named was not closed: %s", s.items[changeKey(testRepo, collideNum)])
	}
	assertContains(t, "typed close", s.typedCloses,
		fmt.Sprintf("%s#%d:%s", testRepo, collideNum, deskkit.TargetChange))
}

// TestBareNumberUnchangedOnASingleSequenceForge is the regression guard. Where a number names
// exactly one object there is nothing to state, and stating nothing must keep taking the
// UNTYPED read — otherwise every existing caller and every manifest row would now be asserting
// a kind it never said.
func TestBareNumberUnchangedOnASingleSequenceForge(t *testing.T) {
	s, rul := baseWorld(t)
	code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
		"--by", mergedPRRef, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertConfirmedClose(t, s, reasonNotPlanned)
	// The caller stated no kind, so the object is RESOLVED by the forge — not asserted to be
	// one kind on the caller's behalf. That untyped read is what keeps `#N` neutral and every
	// existing call site working unchanged.
	assertContains(t, "untyped read", s.untypedGets, fmt.Sprintf("%s#%d", testRepo, subjectIssue))
	// The WRITES are typed regardless, because by then the object has been READ and its kind
	// is a fact rather than a claim.
	assertContains(t, "typed close", s.typedCloses,
		fmt.Sprintf("%s#%d:%s", testRepo, subjectIssue, deskkit.TargetIssue))
}

// TestKindFlagAndSigilDisagreeIsRefused: two statements about one object, and deskclose picks
// neither. The same rule applies on the target side.
func TestKindFlagAndSigilDisagreeIsRefused(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"item", []string{modeSuperseded, "-R", testRepo, "!3", "--kind", "issue", "--by", "!4"}},
		{"target", []string{modeSuperseded, "-R", testRepo, "!3", "--by", "!4", "--by-kind", "issue"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, rul := collisionWorld(t)
			s.viewer = workerLogin
			err := execErr(append(c.args, "--rulings", rul)...)
			if err == nil || !deskkit.IsRefused(err) {
				t.Fatalf("a disagreeing sigil and kind flag must be refused, got %v", err)
			}
			if !strings.Contains(err.Error(), "disagree") {
				t.Fatalf("the refusal should say the two disagree: %v", err)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("a refusal wrote to the forge: %v", got)
			}
		})
	}
}

// TestUnknownKindWordIsRefused: the kind vocabulary is the shared one, so deskclose cannot
// accept a word its sibling verbs reject, and an unparseable kind never quietly becomes an
// issue.
func TestUnknownKindWordIsRefused(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = workerLogin
	err := execErr(modeSuperseded, "-R", testRepo, "3", "--kind", "banana", "--by", "!4", "--rulings", rul)
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("an unknown kind word must be refused, got %v", err)
	}
	if !strings.Contains(err.Error(), "--kind") {
		t.Fatalf("the refusal should name the flag it came from: %v", err)
	}
}

// TestItemNumberTakesNoRepoPrefix: the repository is named by -R, once. A second, possibly
// different, repository on the positional would be two answers to one question.
func TestItemNumberTakesNoRepoPrefix(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = workerLogin
	err := execErr(modeSuperseded, "-R", testRepo, "other-org/other!3", "--by", "!4", "--rulings", rul)
	if err == nil || !deskkit.IsRefused(err) {
		t.Fatalf("a repo-qualified item number must be refused, got %v", err)
	}
}

func assertContains(t *testing.T, what string, got []string, want string) {
	t.Helper()
	for _, g := range got {
		if g == want {
			return
		}
	}
	t.Fatalf("%s trail %v does not contain %q", what, got, want)
}
