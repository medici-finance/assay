package deskkit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

// attemptRecord builds a record's top-level fields: a fixed window and row count around the
// given attempts JSON ("" leaves the key out).
func attemptRecord(t *testing.T, outcome, attempts string) map[string]json.RawMessage {
	t.Helper()
	raw := `{"brief":"example-stream/02","ts":"2026-10-22T12:00:00Z","outcome":"` + outcome + `",` +
		`"started":"2026-10-22T09:00:00Z","finished":"2026-10-22T11:00:00Z","rows_total":9`
	if attempts != "" {
		raw += `,"attempts":` + attempts
	}
	raw += `}`
	var f map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("fixture record: %v", err)
	}
	return f
}

const (
	attUncredited = `{"attempt":1,"disposition":"uncredited","reason":"local-build","started":"2026-10-22T09:00:00Z","finished":"2026-10-22T09:20:00Z","rows_run":9}`
	attDiscarded  = `{"attempt":2,"disposition":"discarded","reason":"env-fault","cause_ref":"example-org/example#12","started":"2026-10-22T09:30:00Z","finished":"2026-10-22T09:50:00Z","rows_run":5}`
	attCredited   = `{"attempt":3,"disposition":"credited","started":"2026-10-22T10:00:00Z","finished":"2026-10-22T10:30:00Z","rows_run":9}`
)

// regression: #4163
// TestCheckAttempts is Verify row 3: each rule refuses naming the attempt and the rule, the
// issue's own three-attempt shape passes, and a record with no attempts passes.
func TestCheckAttempts(t *testing.T) {
	valid := "[" + attUncredited + "," + attDiscarded + "," + attCredited + "]"
	t.Run("graph-execution shape passes", func(t *testing.T) {
		got, err := CheckAttempts(attemptRecord(t, "blocked", valid))
		if err != nil || len(got) != 3 || got[2].Disposition != AttemptCredited || got[1].CauseRef != "example-org/example#12" {
			t.Fatalf("valid attempts refused or misparsed: %v %+v", err, got)
		}
		if _, err := CheckAttempts(attemptRecord(t, "verified", valid)); err != nil {
			t.Fatalf("verified record ending in credited refused: %v", err)
		}
	})
	t.Run("no attempts passes", func(t *testing.T) {
		got, err := CheckAttempts(attemptRecord(t, "verified", ""))
		if err != nil || got != nil {
			t.Fatalf("record without attempts: %v %+v", err, got)
		}
	})

	att := func(n int, body string) string {
		return `{"attempt":` + itoa(n) + `,` + body + `}`
	}
	win := func(a, b string) string {
		return `"started":"2026-10-22T` + a + `Z","finished":"2026-10-22T` + b + `Z","rows_run":3`
	}
	cases := []struct {
		name, outcome, attempts, attempt, rule string
	}{
		{"gap in numbering", "blocked",
			"[" + att(1, `"disposition":"uncredited","reason":"local-build",`+win("09:00:00", "09:10:00")) + "," +
				att(3, `"disposition":"credited",`+win("09:20:00", "09:30:00")) + "]", "attempt 2", "rule numbering"},
		{"credited not last", "blocked",
			"[" + att(1, `"disposition":"credited",`+win("09:00:00", "09:10:00")) + "," +
				att(2, `"disposition":"uncredited","reason":"not-credited",`+win("09:20:00", "09:30:00")) + "]", "attempt 1", "rule credited-last"},
		{"two credited", "blocked",
			"[" + att(1, `"disposition":"credited",`+win("09:00:00", "09:10:00")) + "," +
				att(2, `"disposition":"credited",`+win("09:20:00", "09:30:00")) + "]", "attempt 2", "rule single-credited"},
		{"verified ending discarded", "verified",
			"[" + att(1, `"disposition":"uncredited","reason":"credit-unchecked",`+win("09:00:00", "09:10:00")) + "," +
				att(2, `"disposition":"discarded","reason":"superseded",`+win("09:20:00", "09:30:00")) + "]", "attempt 2", "rule verified-ends-credited"},
		{"env-fault without cause_ref", "blocked",
			"[" + att(1, `"disposition":"discarded","reason":"env-fault",`+win("09:00:00", "09:10:00")) + "]", "attempt 1", "rule cause-ref-required"},
		{"credited with a reason", "verified",
			"[" + att(1, `"disposition":"credited","reason":"local-build",`+win("09:00:00", "09:10:00")) + "]", "attempt 1", "rule credited-no-reason"},
		{"overlapping windows", "blocked",
			"[" + att(1, `"disposition":"uncredited","reason":"local-build",`+win("09:00:00", "09:30:00")) + "," +
				att(2, `"disposition":"credited",`+win("09:20:00", "09:40:00")) + "]", "attempt 2", "rule overlap"},
		{"attempt outside the record window", "blocked",
			"[" + att(1, `"disposition":"credited",`+win("08:00:00", "09:10:00")) + "]", "attempt 1", "rule record-window"},
		{"rows_run above rows_total", "blocked",
			"[" + att(1, `"disposition":"credited","started":"2026-10-22T09:00:00Z","finished":"2026-10-22T09:10:00Z","rows_run":10`) + "]", "attempt 1", "rule rows-run"},
		{"seventeen entries", "blocked", seventeen(), "attempt 17", "rule count"},
		{"unknown key tokens", "blocked",
			"[" + att(1, `"disposition":"credited","tokens":12000,`+win("09:00:00", "09:10:00")) + "]", "attempt 1", "rule unknown-key"},
		{"case-variant key Disposition", "blocked",
			"[" + att(1, `"Disposition":"credited",`+win("09:00:00", "09:10:00")) + "]", "attempt 1", "rule key-case"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CheckAttempts(attemptRecord(t, c.outcome, c.attempts))
			if err == nil {
				t.Fatalf("accepted; want refusal naming %q and %q", c.attempt, c.rule)
			}
			if ExitCodeOf(err) != ExitRefused {
				t.Fatalf("exit %d, want %d (refused): %v", ExitCodeOf(err), ExitRefused, err)
			}
			if msg := err.Error(); !strings.Contains(msg, c.attempt+":") || !strings.Contains(msg, c.rule+":") {
				t.Fatalf("refusal %q does not name %q and %q", msg, c.attempt, c.rule)
			}
		})
	}

	t.Run("writer path refuses through ParseJoinInput", func(t *testing.T) {
		bad := `{"brief":"example-stream/02","ts":"2026-10-22T12:00:00Z","outcome":"blocked","started":"2026-10-22T09:00:00Z",` +
			`"finished":"2026-10-22T11:00:00Z","rows_total":9,"attempts":[{"attempt":2,"disposition":"credited",` + win("09:00:00", "09:10:00") + `}]}`
		if _, err := ParseJoinInput([]byte(bad)); err == nil || !strings.Contains(err.Error(), "rule numbering") {
			t.Fatalf("ParseJoinInput did not run CheckAttempts: %v", err)
		}
		if _, err := ParseJoinInput([]byte(`{"brief":"example-stream/02","ts":"2026-10-22T12:00:00Z","Attempts":[]}`)); err == nil ||
			!strings.Contains(err.Error(), `must be spelled "attempts"`) {
			t.Fatalf("a top-level case-variant Attempts key was not refused: %v", err)
		}
	})
}

func seventeen() string {
	var parts []string
	for i := 1; i <= 17; i++ {
		parts = append(parts, `{"attempt":`+itoa(i)+`,"disposition":"discarded","reason":"superseded","started":"2026-10-22T09:00:00Z","finished":"2026-10-22T09:00:00Z","rows_run":1}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// TestRollupAttempts counts every record of a brief: attempts by disposition and reason, a
// record without attempts as one unrecorded run, elapsed per disposition from the windows.
func TestRollupAttempts(t *testing.T) {
	withAttempts := `{"brief":"example-stream/02","ts":"2026-10-22T12:00:00Z","outcome":"blocked","started":"2026-10-22T09:00:00Z",` +
		`"finished":"2026-10-22T11:00:00Z","rows_total":9,"attempts":[` + attUncredited + "," + attDiscarded + "," + attCredited + `]}`
	legacy := `{"brief":"example-stream/02","ts":"2026-10-20T12:00:00Z","outcome":"verify-fail"}`
	var recs []OutcomeRecord
	for _, line := range []string{withAttempts, legacy} {
		r, err := ParseRecord([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		recs = append(recs, r)
	}
	r := RollupAttempts("example-stream/02", recs)
	if r.Runs != 4 || r.ByDisposition[AttemptUncredited] != 1 || r.ByDisposition[AttemptDiscarded] != 1 ||
		r.ByDisposition[AttemptCredited] != 1 || r.ByDisposition[AttemptUnrecorded] != 1 {
		t.Fatalf("counts: %+v", r)
	}
	if r.ByReason[ReasonLocalBuild] != 1 || r.ByReason[ReasonEnvFault] != 1 {
		t.Fatalf("reasons: %+v", r.ByReason)
	}
	if r.Elapsed[AttemptUncredited].Minutes() != 20 || r.Elapsed[AttemptCredited].Minutes() != 30 || r.ElapsedUnknown[AttemptUnrecorded] != 1 {
		t.Fatalf("elapsed: %+v unknown %+v", r.Elapsed, r.ElapsedUnknown)
	}
	if r.Attempts[0].Disposition != AttemptUnrecorded {
		t.Fatalf("runs are not in record ts order: %+v", r.Attempts)
	}
}

// regression: #4163
// TestPerfBriefRef is Verify row 12's mapping half: the frontmatter id maps to the qualified
// perf work ref with leading zeros dropped, short keys and number 0 are refused, and every
// mapped value resolves through perfrec's own context resolver with work coverage measured.
func TestPerfBriefRef(t *testing.T) {
	for in, want := range map[string]string{
		"assay:assay:desk-supervision:37": "assay:assay:desk-supervision:37",
		"assay:assay:graph-execution:02":  "assay:assay:graph-execution:2",
	} {
		got, err := PerfBriefRef(in)
		if err != nil || got != want {
			t.Fatalf("PerfBriefRef(%q) = %q, %v; want %q", in, got, err, want)
		}
		work := `{"kind":"brief","repo":"medici-finance/assay","brief":"` + got + `"}`
		c := perfrec.ResolveContext(perfrec.ContextInput{
			Profile: perfrec.Value{Value: "workflow", Present: true},
			Role:    perfrec.Value{Value: "verify", Present: true},
			Work:    []string{work}, WorkPresent: true,
			LookupEnv: func(k string) (string, bool) {
				if k == perfrec.EnvVar {
					return "1", true
				}
				return "", false
			},
		})
		d := c.Data()
		if d.Coverage[perfrec.FieldWork] != perfrec.CoverageMeasured || len(d.Work) != 1 || d.Work[0].Brief != got {
			t.Fatalf("%q as a perf work ref: coverage %q work %+v", got, d.Coverage[perfrec.FieldWork], d.Work)
		}
	}
	for _, bad := range []string{"example-stream/37", "a:b:c:0", "assay:assay:desk-supervision:", ""} {
		_, err := PerfBriefRef(bad)
		if err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), `"`+bad+`"`) {
			t.Fatalf("PerfBriefRef(%q) not refused naming the value: %v", bad, err)
		}
	}
}
