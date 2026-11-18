package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestOutcomeProtocolFlow follows ONE outcome word through the producer and the guards that
// read it, to pin that they share a single meaning (statusgen/evaluation/outcome.go):
//
//   - the writer lands only the exact canonical spellings, so a record no reader would classify
//     the same way never reaches the sidecar, and a record with no outcome never lands;
//   - the closure guard and the sidecar gate treat every spelling a READER counts as a pass
//     ("Verified", " verified ") as a pass, so a case or whitespace variant cannot walk past the
//     closure requirement;
//   - a pass that holds or fails rows is refused at the writer, not left for a reader to find.
//
// regression: F-outcome-protocol-copies
func TestOutcomeProtocolFlow(t *testing.T) {
	const brief = "x/01"
	record := func(outcome string, extra string) string {
		return fmt.Sprintf(`{"brief":%q,"ts":"2026-10-06T00:00:00Z","outcome":%q%s}`, brief, outcome, extra)
	}

	land := func(t *testing.T, body string) (code int, puts int, stderr string) {
		t.Helper()
		f, errBuf := setupFake(t)
		mintTokenFn = func(string) (deskkit.RoleCredential, error) {
			ghToken = fakeVerifierToken
			return deskkit.RoleCredential{Token: ghToken, Resolution: deskkit.ForgeResolution{Kind: deskkit.ForgeGitHub}}, nil
		}
		verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
			return stubAdmittedReceipt(target), nil
		}
		home := rootWithFile(t, "docs/streams/x/brief.md", "# Brief\n")
		rec := filepath.Join(t.TempDir(), "outcomes", "record.json")
		writeFixtureFile(t, rec, body)
		code = run([]string{fixtureRepo, "main", "--root", home, "--outcome-record", rec})
		return code, f.putCalls, errBuf.String()
	}

	t.Run("the writer lands the canonical spelling", func(t *testing.T) {
		code, puts, errs := land(t, record("verified", ""))
		if code != deskkit.ExitOK || puts != 1 {
			t.Fatalf("exit=%d puts=%d: %s", code, puts, errs)
		}
	})

	t.Run("the writer refuses a spelling a reader would read differently", func(t *testing.T) {
		for _, bad := range []string{"Verified", " verified", "verified ", "VERIFIED", "pass", "ok", "done", "Fail"} {
			code, puts, errs := land(t, record(bad, ""))
			if code != deskkit.ExitRefused || puts != 0 {
				t.Errorf("outcome %q: exit=%d puts=%d, want a refusal that lands nothing: %s", bad, code, puts, errs)
				continue
			}
			if !strings.Contains(errs, "outcome") {
				t.Errorf("outcome %q: refusal does not name the outcome field: %s", bad, errs)
			}
		}
	})

	t.Run("the writer refuses a record that names no outcome", func(t *testing.T) {
		body := fmt.Sprintf(`{"brief":%q,"ts":"2026-10-06T00:00:00Z","verdict":"verified"}`, brief)
		code, puts, errs := land(t, body)
		if code != deskkit.ExitRefused || puts != 0 || !strings.Contains(errs, "outcome") {
			t.Fatalf("exit=%d puts=%d: %s", code, puts, errs)
		}
	})

	t.Run("the writer refuses a pass that holds or fails rows", func(t *testing.T) {
		for name, extra := range map[string]string{
			"held rows":  `,"rows":[2],"rows_total":2,"rows_passed":1`,
			"failed row": `,"rows_total":2,"rows_passed":1,"row_results":[{"row":1,"result":"pass"},{"row":2,"result":"fail"}]`,
		} {
			code, puts, errs := land(t, record("verified", extra))
			if code != deskkit.ExitRefused || puts != 0 {
				t.Errorf("%s: exit=%d puts=%d, want a refusal that lands nothing: %s", name, code, puts, errs)
			}
		}
	})

	t.Run("the closure guard counts every reader-visible pass spelling", func(t *testing.T) {
		for _, spelling := range []string{"verified", "Verified", " verified ", "VERIFIED"} {
			var asked []string
			oldCheck := closureCheckFn
			t.Cleanup(func() { closureCheckFn = oldCheck })
			closureCheckFn = func(_, key string, _ childOptions) (string, error) {
				asked = append(asked, key)
				return "", deskkit.Refused("not verified")
			}
			after := record(spelling, "") + "\n"
			err := guardVerifiedOutcomes(t.TempDir(), t.TempDir(), "docs/streams/verify-outcomes-2026-09.jsonl",
				nil, []byte(after), nil, deskkit.ForgeRepo{}, "main", childOptions{})
			if err == nil || len(asked) != 1 || asked[0] != brief {
				t.Errorf("outcome %q slipped past the closure guard: err=%v asked=%v", spelling, err, asked)
			}
			if got := verifiedBriefsAdded(nil, []byte(after), false); len(got) != 1 || got[0] != brief {
				t.Errorf("outcome %q slipped past the sidecar gate: %v", spelling, got)
			}
		}
	})

	t.Run("a fail, a hold and an unrecognised word never ask for closure", func(t *testing.T) {
		for _, word := range []string{"verify-fail", "fail", "blocked", "needs-context", "ok", "pass", ""} {
			oldCheck := closureCheckFn
			t.Cleanup(func() { closureCheckFn = oldCheck })
			closureCheckFn = func(string, string, childOptions) (string, error) {
				t.Errorf("outcome %q asked for a closure check", word)
				return "", nil
			}
			after := record(word, "") + "\n"
			if err := guardVerifiedOutcomes(t.TempDir(), t.TempDir(), "docs/streams/verify-outcomes-2026-09.jsonl",
				nil, []byte(after), nil, deskkit.ForgeRepo{}, "main", childOptions{}); err != nil {
				t.Errorf("outcome %q: %v", word, err)
			}
			if got := verifiedBriefsAdded(nil, []byte(after), false); len(got) != 0 {
				t.Errorf("outcome %q counted as a pass: %v", word, got)
			}
		}
	})
}
