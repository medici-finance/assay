package main

// lanes_test.go — the two identity+structure lanes (forge-neutral brief 16).
//
// Every refusal here ships with the positive control beside it, and every positive control
// asserts the EXACT write sequence on the recording stub — so "the lane refused" is the
// absence of a write, and "the lane ran" is the presence of exactly the writes the brief
// names, in the order it names them, against the one item it names.
//
// The two "ignores unsigned ruling" tests are the structural claim of the brief: neither
// lane calls gateFor. They run with the R-1 sign-off comment DELETED from the stub and the
// rulings path pointing at nothing, and assert that the per-process grant cache is still
// nil afterwards — the gate was never consulted, not merely satisfied.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	draftNum      = 77 // the worker App's own open DRAFT
	cardNum       = 88 // a CLOSED issue carrying verify-gate
	workerBotID   = 300000006
	verifierBotID = 300000005
	refireReason  = "re-running the verify cycle after the Evidence row was corrected"
)

// draftPullJSON renders a pulls-endpoint fixture carrying the fields the self-withdraw lane
// reads in its ONE change read: draft flag, author (login + numeric id) and labels.
func draftPullJSON(n int, state string, draft bool, login string, id int64, labels ...string) string {
	var ls []string
	for _, l := range labels {
		ls = append(ls, fmt.Sprintf(`{"name":%q}`, l))
	}
	return fmt.Sprintf(`{"number":%d,"title":"stub draft","state":%q,"merged":false,"draft":%t,`+
		`"user":{"login":%q,"id":%d,"type":"Bot"},"labels":[%s]}`, n, state, draft, login, id, strings.Join(ls, ","))
}

// selfWithdrawWorld is baseWorld plus the worker App's own open draft, with the session
// acting as the WORKER role — the author.
func selfWithdrawWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, draftNum)] = prIssueJSON(draftNum, "open")
	s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "open", true, workerLogin, workerBotID)
	s.viewer = workerLogin
	return s, rul
}

// refireWorld is baseWorld plus a CLOSED verify-gate card, with the session acting as the
// VERIFIER role.
func refireWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, cardNum)] = issueJSON(cardNum, "closed", []string{deskkit.VerifyGateLabel}, "sign-off card")
	s.viewer = verifierLogin
	return s, rul
}

// assertOnlyItem fails if ANY recorded call — read or write — addressed an item other than n.
func assertOnlyItem(t *testing.T, s *stubRemote, n int) {
	t.Helper()
	want := fmt.Sprint(n)
	for _, c := range s.calls {
		if len(c) >= 2 && c[0] == "api" {
			if !strings.HasSuffix(c[1], "/"+want) && !strings.HasSuffix(c[1], "/"+want+"/comments") {
				t.Fatalf("a call reached another item: %v", c)
			}
			continue
		}
		if len(c) >= 3 && c[2] != want {
			t.Fatalf("a write reached another item: %v", c)
		}
	}
}

func assertNoWrites(t *testing.T, s *stubRemote) {
	t.Helper()
	if w := s.writes(); len(w) != 0 {
		t.Fatalf("a refusal path performed writes: %v", w)
	}
}

// ---------------------------------------------------------------- self-withdraw

// TestSelfWithdrawSucceedsOnOwnDraft — Verify row 3. The authoring App closes its own draft:
// exactly one comment, then one close, and nothing against any other item. No disposition
// record is consulted (the disposition seam records zero calls).
func TestSelfWithdrawSucceedsOnOwnDraft(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string // substrings the close comment must carry
	}{
		{"abandoned", []string{"--because", becauseAbandoned}, []string{"Because: abandoned"}},
		{"superseded --by", []string{"--because", becauseSuperseded, "--by", "#40"},
			[]string{"Because: superseded", "Replaced by: " + testRepo + "#40", "not verified merged"}},
		{"superseded --by typed change", []string{"--because", becauseSuperseded, "--by", "!40"},
			[]string{"Replaced by: " + testRepo + "!40"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := selfWithdrawWorld(t)
			args := append([]string{modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--rulings", rul}, tc.args...)
			code, out := execCLI(args...)
			if code != deskkit.ExitOK {
				t.Fatalf("want exit 0, got %d\n%s", code, out)
			}
			w := s.writes()
			if len(w) != 2 || w[0][0] != "issue" || w[0][1] != "comment" || w[1][0] != "pr" || w[1][1] != "close" {
				t.Fatalf("want exactly [comment, close] on the draft, got %v", w)
			}
			assertOnlyItem(t, s, draftNum)
			if len(s.dispCalls) != 0 {
				t.Fatalf("self-withdraw consulted the disposition record %v — an author's own withdrawal needs no finding", s.dispCalls)
			}
			body := w[0][6]
			for _, want := range append(tc.want, "no ruling artifact is cited", "Withdrawn by: "+workerLogin) {
				if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
					t.Fatalf("close comment lacks %q:\n%s", want, body)
				}
			}
			if !strings.Contains(out, "closed\t"+testRepo+"#77\tself-withdraw") {
				t.Fatalf("stdout should report the close: %q", out)
			}
		})
	}

	t.Run("already closed is an idempotent no-op", func(t *testing.T) {
		s, rul := selfWithdrawWorld(t)
		s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "closed", true, workerLogin, workerBotID)
		code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
		if code != deskkit.ExitOK || !strings.Contains(out, "noop") {
			t.Fatalf("want exit 0 noop, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})

	t.Run("dry-run writes nothing", func(t *testing.T) {
		s, rul := selfWithdrawWorld(t)
		code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--dry-run", "--rulings", rul)
		if code != deskkit.ExitOK || !strings.Contains(out, "dry-run") {
			t.Fatalf("want exit 0 dry-run, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestSelfWithdrawRefusesNonAuthor — Verify row 4. The same draft, a DIFFERENT minted role
// (the reviewer App): refused, naming the actual author and the acting role, zero writes.
func TestSelfWithdrawRefusesNonAuthor(t *testing.T) {
	s, rul := selfWithdrawWorld(t)
	s.viewer = reviewerLogin
	code, _ := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d", code)
	}
	assertNoWrites(t, s)
	if reopens := countVerb(s, "reopen"); reopens != 0 {
		t.Fatalf("a refused self-withdraw reopened something: %d", reopens)
	}

	t.Run("id matches, login does not", func(t *testing.T) {
		// The login half is its own gate, not a consequence of the id half: an author whose
		// numeric id equals the acting App's but whose login does not is refused by the login
		// check, before the id is compared.
		s, rul := selfWithdrawWorld(t)
		s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "open", true, "someone-else", workerBotID)
		var stderr strings.Builder
		code := runCapturingStderr(t, &stderr, modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if !strings.Contains(stderr.String(), "was authored by someone-else[bot]") {
			t.Fatalf("the refusal must name the actual author, got:\n%s", stderr.String())
		}
		assertNoWrites(t, s)
	})

	// Positive control: the author's own session closes it.
	s2, rul2 := selfWithdrawWorld(t)
	if code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul2); code != deskkit.ExitOK {
		t.Fatalf("control: want exit 0, got %d\n%s", code, out)
	}
	if s2.closes() != 1 {
		t.Fatalf("control: want 1 close, got %d", s2.closes())
	}
}

// TestSelfWithdrawRefusesNonDraft — Verify row 5. The authoring App's OWN pull request, out
// of draft: refused, zero writes.
func TestSelfWithdrawRefusesNonDraft(t *testing.T) {
	s, rul := selfWithdrawWorld(t)
	s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "open", false, workerLogin, workerBotID)
	code, _ := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d", code)
	}
	assertNoWrites(t, s)
}

// TestSelfWithdrawRefusesDecisionLabelled — Verify row 6. The author's own draft carrying
// needs-decision: refused by refuseDecisionItem, BEFORE the draft and authorship checks run
// (the fixture also breaks both of those, and the refusal is still the decision-label one).
func TestSelfWithdrawRefusesDecisionLabelled(t *testing.T) {
	s, rul := selfWithdrawWorld(t)
	s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "open", false, "someone-else", 1, labelNeedsDecision)
	var stderr strings.Builder
	code := runCapturingStderr(t, &stderr, modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d", code)
	}
	if !strings.Contains(stderr.String(), `"needs-decision" label`) {
		t.Fatalf("the refusal must be the decision-label gate's, got:\n%s", stderr.String())
	}
	assertNoWrites(t, s)
}

// TestSelfWithdrawIDPinRejectsLoginOnlyMatch — Verify row 7. The pin is login AND id: a
// login match with the wrong numeric id is refused, and an UNPINNED roster id (0) is a
// could-not-check refusal rather than a login-only pass.
func TestSelfWithdrawIDPinRejectsLoginOnlyMatch(t *testing.T) {
	t.Run("login matches, id does not", func(t *testing.T) {
		s, rul := selfWithdrawWorld(t)
		s.pulls[fmt.Sprintf("%s#%d", testRepo, draftNum)] = draftPullJSON(draftNum, "open", true, workerLogin, 999)
		var stderr strings.Builder
		code := runCapturingStderr(t, &stderr, modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if !strings.Contains(stderr.String(), "login AND id") {
			t.Fatalf("the refusal must name the id half of the pin, got:\n%s", stderr.String())
		}
		assertNoWrites(t, s)
	})

	t.Run("roster pins no id for the role", func(t *testing.T) {
		s, rul := selfWithdrawWorld(t)
		plantRosterVariant(t, strings.Replace(fixtureRoster, "worker=assay-worker-app:300000006", "worker=assay-worker-app", 1))
		code, _ := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 (could-not-check, never a login-only pass), got %d", code)
		}
		assertNoWrites(t, s)
	})

	// Positive control: login AND id match → closes.
	s, rul := selfWithdrawWorld(t)
	if code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned, "--rulings", rul); code != deskkit.ExitOK {
		t.Fatalf("control: want exit 0, got %d\n%s", code, out)
	}
	if s.closes() != 1 {
		t.Fatalf("control: want 1 close, got %d", s.closes())
	}
}

// TestSelfWithdrawIgnoresUnsignedRuling — Verify row 8. R-1 unsigned (sign-off comment gone,
// rulings path absent): a genuine self-withdraw still succeeds, and the grant cache is still
// nil — gateFor was never called.
func TestSelfWithdrawIgnoresUnsignedRuling(t *testing.T) {
	s, _ := selfWithdrawWorld(t)
	delete(s.comment, signOffCID)
	code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--because", becauseAbandoned,
		"--rulings", filepath.Join(t.TempDir(), "absent-rulings.md"))
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 with R-1 unsigned, got %d\n%s", code, out)
	}
	if s.closes() != 1 {
		t.Fatalf("want 1 close, got %d", s.closes())
	}
	if cachedGrant != nil {
		t.Fatal("the ruling gate was consulted by self-withdraw — it must never be")
	}
}

// TestSelfWithdrawFlagGrammar pins the --because/--by contract and the pre-flight kind
// refusal: each is refused before any forge call.
func TestSelfWithdrawFlagGrammar(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"superseded without --by", []string{"--because", becauseSuperseded}},
		{"abandoned with --by", []string{"--because", becauseAbandoned, "--by", "#40"}},
		{"no --because", nil},
		{"unknown --because", []string{"--because", "bored"}},
		{"--kind issue", []string{"--because", becauseAbandoned, "--kind", "issue"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := selfWithdrawWorld(t)
			args := append([]string{modeSelfWithdraw, "-R", testRepo, fmt.Sprint(draftNum), "--rulings", rul}, tc.args...)
			code, _ := execCLI(args...)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d", code)
			}
			if len(s.calls) != 0 {
				t.Fatalf("a pre-flight refusal reached the forge: %v", s.calls)
			}
		})
	}
}

// ---------------------------------------------------------------- verify-gate-refire

// TestVerifyGateRefireSucceedsAsVerifier — Verify row 9. A verifier session on a CLOSED
// verify-gate card: reopen, then comment, then close, in that order, on the same item; the
// reason is in the comment under both the reopen and the re-close line; the close carries no
// state reason.
func TestVerifyGateRefireSucceedsAsVerifier(t *testing.T) {
	s, rul := refireWorld(t)
	code, out := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	w := s.writes()
	if len(w) != 3 || w[0][1] != "reopen" || w[1][1] != "comment" || w[2][1] != "close" {
		t.Fatalf("want exactly [reopen, comment, close] in that order, got %v", w)
	}
	for _, c := range w {
		if c[0] != "issue" || c[2] != fmt.Sprint(cardNum) {
			t.Fatalf("a write left the card: %v", c)
		}
	}
	assertOnlyItem(t, s, cardNum)
	if len(w[2]) != 5 {
		t.Fatalf("the re-close must carry NO state reason, got argv %v", w[2])
	}
	body := w[1][6]
	if strings.Count(body, refireReason) < 2 {
		t.Fatalf("the reason must be stated for both the reopen and the re-close:\n%s", body)
	}
	for _, want := range []string{"not the human sign-off", "verify-gate-close.yml", verifierLogin} {
		if !strings.Contains(body, want) {
			t.Fatalf("comment lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(out, "refired\t"+testRepo+"#88") {
		t.Fatalf("stdout should report the cycle: %q", out)
	}

	t.Run("already open is an idempotent no-op", func(t *testing.T) {
		s, rul := refireWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, cardNum)] = issueJSON(cardNum, "open", []string{deskkit.VerifyGateLabel}, "")
		code, out := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
		if code != deskkit.ExitOK || !strings.Contains(out, "noop") {
			t.Fatalf("want exit 0 noop, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})

	t.Run("dry-run writes nothing", func(t *testing.T) {
		s, rul := refireWorld(t)
		code, out := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--dry-run", "--rulings", rul)
		if code != deskkit.ExitOK || !strings.Contains(out, "dry-run") {
			t.Fatalf("want exit 0 dry-run, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestVerifyGateRefireRefusesNonVerifier — Verify row 10. Worker and reviewer roles are
// refused by name; an unresolved role is could-not-check. Zero reopen/close calls each time.
func TestVerifyGateRefireRefusesNonVerifier(t *testing.T) {
	for _, tc := range []struct {
		name   string
		viewer string
		want   int
	}{
		{"worker", workerLogin, deskkit.ExitRefused},
		{"reviewer", reviewerLogin, deskkit.ExitRefused},
		{"desk (neither)", appLogin, deskkit.ExitRefused},
		{"unresolved", "", deskkit.ExitUnverifiable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := refireWorld(t)
			s.viewer = tc.viewer
			var stderr strings.Builder
			code := runCapturingStderr(t, &stderr, modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
			if code != tc.want {
				t.Fatalf("want exit %d, got %d\n%s", tc.want, code, stderr.String())
			}
			assertNoWrites(t, s)
			if tc.want == deskkit.ExitRefused && !strings.Contains(stderr.String(), "not the verifier role") {
				t.Fatalf("the refusal must name the lane's role, got:\n%s", stderr.String())
			}
		})
	}
}

// TestVerifyGateRefireRefusesUnlabelledItem — Verify row 11. A verifier session, a closed
// issue WITHOUT verify-gate: refused naming the missing label; zero writes. A closed PULL
// REQUEST is refused too (a card is an issue), and a stated change kind is refused
// pre-flight with no forge call at all.
func TestVerifyGateRefireRefusesUnlabelledItem(t *testing.T) {
	s, rul := refireWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, cardNum)] = issueJSON(cardNum, "closed", []string{"bug"}, "")
	var stderr strings.Builder
	code := runCapturingStderr(t, &stderr, modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d", code)
	}
	if !strings.Contains(stderr.String(), `"verify-gate" label`) {
		t.Fatalf("the refusal must name the missing label, got:\n%s", stderr.String())
	}
	assertNoWrites(t, s)

	t.Run("a closed pull request is not a card", func(t *testing.T) {
		s, rul := refireWorld(t)
		// Even a closed pull request CARRYING the label is refused: the kind check is its own
		// gate, ahead of the label gate, not a consequence of it.
		s.items[fmt.Sprintf("%s#%d", testRepo, mergedPRNum)] = fmt.Sprintf(
			`{"number":%d,"title":"stub pr","state":"closed","body":"","labels":[{"name":%q}],"pull_request":{"merged_at":null}}`,
			mergedPRNum, deskkit.VerifyGateLabel)
		code, _ := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(mergedPRNum), "--reason", refireReason, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("a stated change kind is refused pre-flight", func(t *testing.T) {
		s, rul := refireWorld(t)
		code, _ := execCLI(modeVerifyGateRefire, "-R", testRepo, "!"+fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if len(s.calls) != 0 {
			t.Fatalf("a pre-flight refusal reached the forge: %v", s.calls)
		}
	})

	t.Run("a decision-labelled card is refused", func(t *testing.T) {
		s, rul := refireWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, cardNum)] = issueJSON(cardNum, "closed", []string{deskkit.VerifyGateLabel, labelNeedsDecision}, "")
		code, _ := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("--reason is mandatory", func(t *testing.T) {
		s, rul := refireWorld(t)
		code, _ := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if len(s.calls) != 0 {
			t.Fatalf("a pre-flight refusal reached the forge: %v", s.calls)
		}
	})
}

// TestVerifyGateRefireIgnoresUnsignedRuling — Verify row 12. R-1 unsigned: a genuine
// verifier re-fire still succeeds, and gateFor was never called.
func TestVerifyGateRefireIgnoresUnsignedRuling(t *testing.T) {
	s, _ := refireWorld(t)
	delete(s.comment, signOffCID)
	code, out := execCLI(modeVerifyGateRefire, "-R", testRepo, fmt.Sprint(cardNum), "--reason", refireReason,
		"--rulings", filepath.Join(t.TempDir(), "absent-rulings.md"))
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 with R-1 unsigned, got %d\n%s", code, out)
	}
	if s.closes() != 1 || countVerb(s, "reopen") != 1 {
		t.Fatalf("want 1 reopen and 1 close, got writes %v", s.writes())
	}
	if cachedGrant != nil {
		t.Fatal("the ruling gate was consulted by verify-gate-refire — it must never be")
	}
}

// ---------------------------------------------------------------- structure

// TestNewLanesAreNotManifestRowModes pins the exclusion by inspection: both lanes are in the
// closed mode set (the dispatcher's refusal names them) and NEITHER is a manifest row mode.
func TestNewLanesAreNotManifestRowModes(t *testing.T) {
	has := func(list []string, m string) bool {
		for _, x := range list {
			if x == m {
				return true
			}
		}
		return false
	}
	for _, m := range []string{modeSelfWithdraw, modeVerifyGateRefire} {
		if !has(modes(), m) {
			t.Fatalf("%s is missing from modes()", m)
		}
		if has(rowModes(), m) {
			t.Fatalf("%s must not be a manifest row mode — it is not a human-ruled batch primitive", m)
		}
	}
	for _, m := range []string{modeSelfWithdraw, modeVerifyGateRefire} {
		if !strings.Contains(usage, m) {
			t.Fatalf("usage text does not name %s", m)
		}
	}
	// A manifest row naming either lane is refused by the row-mode validator.
	s, rul := baseWorld(t)
	_ = s
	dir := t.TempDir()
	mf := filepath.Join(dir, "m.yaml")
	if err := os.WriteFile(mf, []byte("authorized-by: "+manifestURL+"\nrows:\n  - issue: 77\n    mode: "+modeSelfWithdraw+"\n    target: '#40'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _ := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("a manifest row carrying %s must be refused, got exit %d", modeSelfWithdraw, code)
	}
}

// ---------------------------------------------------------------- helpers

func countVerb(s *stubRemote, verb string) int {
	n := 0
	for _, c := range s.calls {
		if len(c) >= 2 && c[1] == verb {
			n++
		}
	}
	return n
}

// runCapturingStderr runs the CLI with os.Stderr redirected into sb, so a test can assert
// WHICH refusal fired rather than only that one did.
func runCapturingStderr(t *testing.T, sb *strings.Builder, args ...string) int {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = w
	code, _ := execCLI(args...)
	os.Stderr = prev
	w.Close()
	b, _ := io.ReadAll(r)
	r.Close()
	sb.Write(b)
	return code
}

// plantRosterVariant installs a roster variant under a private HOME for the rest of this
// test. The reload cleanup is registered BEFORE t.Setenv so it runs AFTER HOME is restored
// (LIFO), putting the fixture roster back for the next test.
func plantRosterVariant(t *testing.T, roster string) {
	t.Helper()
	t.Cleanup(deskkit.ReloadConfig)
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
}
