package deskkit

import (
	"strings"
	"testing"
)

// TestRedactPreflightOutputPublishesNamesAndStatesOnly pins the alarm's publication
// boundary: from a red roster line whose details carry an ambient login, an absolute path
// and a helper command, only the tally and the closed-vocabulary name=state pairs survive.
// A verdict-shaped token whose NAME is outside the known check set (here smuggled inside a
// detail) is counted and withheld, never echoed.
func TestRedactPreflightOutputPublishesNamesAndStatesOnly(t *testing.T) {
	out := "preflight role=verifier RED 3/6 checked-clean · " + CheckAmbientID + "=checked-failed: " +
		"the ambient gh login is mallory → fix: gh auth login [#1527] · " + CheckWriteTransport +
		"=could-not-check: helper !cat /home/someone/app-token · leaked-secret=checked-failed → fix: none · " +
		CheckAppScopes + "=not-applicable: gitlab"
	r := RedactPreflightOutput(out)
	if r.Tally != "RED 3/6 checked-clean" {
		t.Errorf("tally = %q", r.Tally)
	}
	want := []PreflightVerdict{
		{CheckAmbientID, "checked-failed"},
		{CheckWriteTransport, "could-not-check"},
		{CheckAppScopes, "not-applicable"},
	}
	if len(r.Verdicts) != len(want) {
		t.Fatalf("verdicts = %v, want %v", r.Verdicts, want)
	}
	for i := range want {
		if r.Verdicts[i] != want[i] {
			t.Errorf("verdict %d = %v, want %v", i, r.Verdicts[i], want[i])
		}
	}
	if r.Unrecognised != 1 {
		t.Errorf("unrecognised = %d, want 1 (the out-of-vocabulary name)", r.Unrecognised)
	}

	body := PreflightAlarmBody("verifier", out)
	for _, banned := range []string{"mallory", "/home/someone", "app-token", "leaked-secret", "fix:", "gh auth login"} {
		if strings.Contains(body, banned) {
			t.Errorf("alarm body publishes %q:\n%s", banned, body)
		}
	}
	if !strings.Contains(body, "### Evidence") || !strings.Contains(body, "```\ndeskroster preflight --role verifier") {
		t.Errorf("alarm body lacks the Evidence fence opening with the producing command:\n%s", body)
	}
}

// An unrecognisable roster output (a REFUSED config line, a changed format) still yields a
// body that says so, rather than an empty fence that reads as "nothing failed".
func TestPreflightAlarmBodyUnrecognisedOutputSaysSo(t *testing.T) {
	body := PreflightAlarmBody("desk", "assay-config: REFUSED /some/local/path is not readable")
	if strings.Contains(body, "/some/local/path") {
		t.Errorf("body echoes the unrecognised output:\n%s", body)
	}
	if !strings.Contains(body, "no preflight summary line was recognised") {
		t.Errorf("body does not say the output was unrecognised:\n%s", body)
	}
}
