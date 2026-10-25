package reviewtest_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func projectionTests(t *testing.T, bin, tool, pattern string) (string, error) {
	t.Helper()
	cmd := exec.Command(filepath.Join(bin, tool+".test"), "-test.v", "-test.timeout=45s", "-test.run=^("+pattern+")$")
	cmd.Env = isolatedEnv(t.TempDir())
	b, e := cmd.CombinedOutput()
	return string(b), e
}

// regression: class issue-projection
func TestIssueProjectionFlow(t *testing.T) {
	bin := build(t)
	for tool, pattern := range map[string]string{"issueboard": "TestProjectionJourney", "deskinbox": "TestDiscoveryJourney|TestFetchQueueRankAndAgeSort"} {
		log, e := projectionTests(t, bin, tool, pattern)
		if e != nil {
			t.Fatalf("%s production flow: %v\n%s", tool, e, log)
		}
		for _, name := range strings.Split(pattern, "|") {
			if !strings.Contains(log, "--- PASS: "+name+" ") {
				t.Fatal("missing production PASS", name, log)
			}
		}
	}
	inboxMutant := buildMutant(t, "deskinbox", "../../deskcore/issueflow/selection.go", "return best", "_ = best; return 99")
	if log, e := projectionTests(t, inboxMutant, "deskinbox", "TestDiscoveryJourney"); e == nil || !strings.Contains(log, "human inbox selection") {
		t.Fatalf("human inbox mutation survived: %v %s", e, log)
	}
	// Mutate the actual selector, not an expected-value comparator. The production
	// journey must reject extra histories/unrelated rows when narrowing is removed.
	mutant := buildMutant(t, "issueboard", "../../deskcore/issueflow/selection.go", "return !c.HasPlaceholder && !c.Excluded && !c.DecisionOwed && !c.Addressed && !c.AddressUnknown && !c.HelpWanted", "return true")
	if log, e := projectionTests(t, mutant, "issueboard", "TestProjectionJourney"); e == nil || !strings.Contains(log, "unselected") {
		t.Fatalf("removed selection survived: %v %s", e, log)
	}
}

// regression: class issue-projection
func TestIssueProjectionFaults(t *testing.T) {
	bin := build(t)
	pattern := "TestProjectionFailures|TestTrustGate_BlessThenEdit|TestTrustGate_AdaWrongID|TestTrustGate_CommentsUnreadable_Exit6|TestEscalate_LongThread_BoardRenders|TestPartialListing_NeverRetires|TestPositiveClosedRead_StillRetires|TestAddressedEscalate|TestBoardLabelHistoryHold"
	log, e := projectionTests(t, bin, "issueboard", pattern)
	if e != nil {
		t.Fatalf("production faults: %v\n%s", e, log)
	}
	for _, name := range strings.Split(pattern, "|") {
		if !strings.Contains(log, "--- PASS: "+name+" ") {
			t.Fatal("missing PASS", name, log)
		}
	}
	for _, m := range []struct{ old, new, pattern, diagnostic string }{
		{"return deskkit.Blessed(bodyEdited, events), nil", "_ = bodyEdited; _ = events; return true, nil", "TestTrustGate_BlessThenEdit|TestTrustGate_AdaWrongID", "FAIL"},
		{`if state == "open" {`, "_ = state; if false {", "TestProjectionFailures", "partial listing became empty"},
		{"if !complete {\n\t\t\t\t\t// The escalation read", "_ = complete; if false {\n\t\t\t\t\t// The escalation read", "TestEscalate_LongThread_BoardRenders", "FAIL"},
	} {
		mutant := buildMutant(t, "issueboard", "cmd/issueboard/board.go", m.old, m.new)
		if log, e := projectionTests(t, mutant, "issueboard", m.pattern); e == nil || !strings.Contains(log, m.diagnostic) {
			t.Fatalf("fault mutation survived: %v %s", e, log)
		}
	}
}

// regression: class issue-projection
func TestIssuePerfIntegration(t *testing.T) {
	bin := build(t)
	mutant := buildMutant(t, "deskinbox", "cmd/deskinbox/main.go", "deskkit.PerfCommand(perfVerbs, mode)", "")
	wrong, _, _, _ := perfEntry(t, mutant, "deskinbox", t.TempDir(), "1", []string{"example-org/tracker"}, "ASSAY_PERF_TEST_WORK=1")
	if wrong.Verb == "table" {
		t.Fatal("removed parser attribution survived")
	}
	for _, c := range []struct {
		tool, verb string
		args       []string
	}{
		{"issueboard", "issues", []string{"issues", "--projection", "work-ready", "--sla-days", "-1"}},
		{"deskinbox", "discovery", []string{"discovery", "example-org/tracker"}},
		{"deskinbox", "table", []string{"example-org/tracker"}},
	} {
		for _, consent := range []string{"0", "1"} {
			home := t.TempDir()
			rec, _, code, raw := perfEntry(t, bin, c.tool, home, consent, c.args, "ASSAY_PERF_TEST_WORK=1")
			if consent == "0" {
				if len(raw) != 0 {
					t.Fatal("consent off wrote record")
				}
				if _, e := os.Stat(filepath.Join(home, ".config", "assay", "perf")); !os.IsNotExist(e) {
					t.Fatal("consent off created sink")
				}
				continue
			}
			if rec.Schema != "perf-v2" || rec.Tool != c.tool || rec.Verb != c.verb || rec.Exit != code {
				t.Fatalf("wrong existing recorder attribution: %+v", rec)
			}
			for _, private := range []string{home, "private-title", "example-org/tracker", "example-perf-preservation"} {
				if bytes.Contains(raw, []byte(private)) {
					t.Fatalf("private input leaked into perf record: %s", raw)
				}
			}
		}
	}
}

// regression: class issue-projection-v3-composition
func TestIssueV3Composition(t *testing.T) {
	bin := build(t)
	for _, c := range []struct{ tool, pattern string }{
		{"issueboard", "TestProjectionV3ActualRecorder"},
		{"deskinbox", "TestDiscoveryV3ActualRecorder"},
	} {
		if log, e := projectionTests(t, bin, c.tool, c.pattern); e != nil {
			t.Fatalf("actual native->record/report %s: %v\n%s", c.tool, e, log)
		}
		for _, m := range []struct{ file, old, new string }{
			{"forge.go", "deskkit.ForgeFor(fr, role, observers...)", "deskkit.ForgeFor(fr, role)"},
			{"main.go", "session.SelectContext(c)", "_ = c"},
		} {
			mutant := buildMutant(t, c.tool, "cmd/"+c.tool+"/"+m.file, m.old, m.new)
			if log, e := projectionTests(t, mutant, c.tool, c.pattern); e == nil || !strings.Contains(log, "--- FAIL: "+c.pattern) {
				t.Fatalf("missing invocation composition survived %s/%s: %v\n%s", c.tool, m.file, e, log)
			}
		}
	}
	// Marking raw-detail modes complete would manufacture coverage across the
	// unchanged unobserved comments reader, despite correctly observed GetIssue.
	mutant := buildMutant(t, "deskinbox", "cmd/deskinbox/main.go", "session.SelectContext(c)", "session.SelectContext(c); session.ObserveRequests()")
	if log, e := projectionTests(t, mutant, "deskinbox", "TestDiscoveryV3ActualRecorder"); e == nil || !strings.Contains(log, "raw gap lost") {
		t.Fatalf("raw coverage mutation survived: %v\n%s", e, log)
	}
}
