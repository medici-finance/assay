package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fixtureSrc is the committed perf fixture the deskkit aggregate test hand-checks; this
// package renders the same files, so the two tests can never disagree about the data.
const fixtureSrc = "../../internal/deskkit/testdata/perf"

// seedHome points the desk state dir at a temp HOME and returns the perf dir path (not
// created). DESK_LOOP is cleared so the wrapped verbs record role "none".
func seedHome(t *testing.T) (stateDir, perfDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	t.Setenv("ASSAY_TELEMETRY", "")
	stateDir = filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return stateDir, filepath.Join(stateDir, "perf")
}

// seedFixture copies the committed fixture into perfDir, renaming each day file relative to
// the REAL clock: 2026-10-11 -> two days ago, 2026-10-12 -> yesterday, and 2026-10-05 ->
// eight days ago (outside every window, so its outlier must never render).
func seedFixture(t *testing.T, perfDir string) {
	t.Helper()
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC()
	for src, ago := range map[string]int{"2026-10-11.jsonl": 2, "2026-10-12.jsonl": 1, "2026-10-05.jsonl": 8} {
		b, err := os.ReadFile(filepath.Join(fixtureSrc, src))
		if err != nil {
			t.Fatal(err)
		}
		name := today.AddDate(0, 0, -ago).Format("2006-01-02") + ".jsonl"
		if err := os.WriteFile(filepath.Join(perfDir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func runDeskperf(args ...string) (rc int, stdout, stderr string) {
	var out, errb bytes.Buffer
	rc = run(args, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestReportAndBootRenderFixtureDirectory renders the committed fixture through both
// subcommands (table and JSON), and pins every refusal: an empty history names its reason
// (logging off vs on-but-empty) with nothing on stdout, a filter that matches nothing is
// refused, and a window past the 7 kept days is refused rather than silently capped.
func TestReportAndBootRenderFixtureDirectory(t *testing.T) {
	_, perfDir := seedHome(t)

	// Empty history, logging OFF: unverifiable, reason named, stdout empty.
	rc, out, errOut := runDeskperf("report")
	if rc != deskkit.ExitUnverifiable || out != "" ||
		!strings.Contains(errOut, "no perf history: local perf logging is off (ASSAY_TELEMETRY is not 1)") {
		t.Fatalf("empty history, logging off: rc %d stdout %q stderr %q", rc, out, errOut)
	}
	// Empty history, logging ON: a different reason.
	t.Setenv("ASSAY_TELEMETRY", "1")
	rc, out, errOut = runDeskperf("boot")
	if rc != deskkit.ExitUnverifiable || out != "" ||
		!strings.Contains(errOut, "no perf history: local perf logging is on (ASSAY_TELEMETRY=1) but no wrapped verb has recorded") {
		t.Fatalf("empty history, logging on: rc %d stdout %q stderr %q", rc, out, errOut)
	}
	if _, err := os.Stat(perfDir); !os.IsNotExist(err) {
		t.Fatalf("deskperf created the perf dir (stat err %v) — it must only read", err)
	}

	seedFixture(t, perfDir)

	rc, out, errOut = runDeskperf("report", "--top", "3")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	for _, want := range []string{
		"8 record(s) over the last 7 UTC day(s), total wall 3550 ms",
		"deskpr", "deskboot", "deskpost",
		"slowest 3 invocation(s):",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report table lacks %q:\n%s", want, out)
		}
	}
	// One row per tool, fields in column order (whitespace-normalised).
	rows := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 8 {
			rows[f[0]] = strings.Join(f[1:], " ")
		}
	}
	for tool, want := range map[string]string{
		"deskpr":   "4 200 1000 1000 1600 45.1 0.500",
		"deskboot": "1 1500 1500 1500 1500 42.3 0.000",
		"deskpost": "3 150 250 250 450 12.7 0.000",
	} {
		if rows[tool] != want {
			t.Errorf("report row %s = %q, want %q\n%s", tool, rows[tool], want, out)
		}
	}
	if strings.Contains(out, "99999") {
		t.Errorf("the out-of-window outlier rendered:\n%s", out)
	}
	if i, j := strings.Index(out, "\ndeskpr"), strings.Index(out, "\ndeskpost"); i < 0 || j < 0 || i > j {
		t.Errorf("tools are not sorted by total wall descending:\n%s", out)
	}

	rc, out, _ = runDeskperf("report", "--json", "--top", "3")
	var rep reportJSON
	if rc != deskkit.ExitOK || json.Unmarshal([]byte(out), &rep) != nil {
		t.Fatalf("report --json rc %d / unparseable: %s", rc, out)
	}
	if rep.Summary.TotalMs != 3550 || len(rep.Summary.Tools) != 3 || rep.Summary.Tools[0].RefusalRatio != 0.5 ||
		len(rep.Summary.Slowest) != 3 || rep.Summary.Slowest[0].WallMs != 1500 || rep.Days != 7 {
		t.Errorf("report --json drifted: %+v", rep)
	}

	rc, out, errOut = runDeskperf("boot")
	if rc != deskkit.ExitOK {
		t.Fatalf("boot rc %d stderr %q", rc, errOut)
	}
	for _, want := range []string{
		"3 step record(s) over the last 7 UTC day(s)",
		"role the-desk — summed boot cost 1000 ms",
		"role worker-desk — summed boot cost 200 ms",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("boot table lacks %q:\n%s", want, out)
		}
	}
	if i, j := strings.Index(out, "token-mint"), strings.Index(out, "board-fetch"); i < 0 || j < 0 || i > j {
		t.Errorf("boot steps are not in boot order:\n%s", out)
	}
	rc, out, _ = runDeskperf("boot", "--json", "--role", "the-desk")
	var boot bootJSON
	if rc != deskkit.ExitOK || json.Unmarshal([]byte(out), &boot) != nil {
		t.Fatalf("boot --json rc %d / unparseable: %s", rc, out)
	}
	if len(boot.Summary.Roles) != 1 || boot.Summary.Roles[0].TotalMs != 1000 || len(boot.Summary.Roles[0].Steps) != 2 {
		t.Errorf("boot --json --role the-desk drifted: %+v", boot)
	}

	// Refusals on a NON-empty history.
	for _, tc := range []struct {
		args []string
		rc   int
		want string
	}{
		{[]string{"report", "--tool", "deskwt"}, deskkit.ExitRefused, "perf history present but no record matches --tool deskwt"},
		{[]string{"boot", "--role", "verify-desk"}, deskkit.ExitRefused, "perf history present but no record matches"},
		{[]string{"report", "--days", "8"}, deskkit.ExitRefused, "keeps 7 UTC days"},
		{[]string{"boot", "--days", "0"}, deskkit.ExitRefused, "must be at least 1"},
		{[]string{"report", "stray"}, deskkit.ExitRefused, "unexpected arguments"},
		{[]string{"nope"}, deskkit.ExitRefused, "unknown subcommand"},
	} {
		rc, out, errOut := runDeskperf(tc.args...)
		if rc != tc.rc || out != "" || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: rc %d stdout %q stderr %q; want rc %d naming %q", tc.args, rc, out, errOut, tc.rc, tc.want)
		}
	}
}

// TestWrappedVerbToDeskperfEndToEnd is the cross-component path: a verb wrapped by
// deskkit.PerfRun runs with the switch armed, its record lands in today's day file through
// the substrate, and `deskperf report` reads that same record back. In the same run
// audit.jsonl's bytes and mtime are unchanged.
func TestWrappedVerbToDeskperfEndToEnd(t *testing.T) {
	stateDir, perfDir := seedHome(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("DESK_LOOP", "worker-desk")

	audit := filepath.Join(stateDir, "audit.jsonl")
	ledger := []byte(`{"ts":"2026-10-13T00:00:00Z","tool":"deskpr","verb":"create","result":"ok"}` + "\n")
	if err := os.WriteFile(audit, ledger, 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(audit, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	rc := deskkit.PerfRun("deskpr", []string{"create", "update"}, []string{"update", "--pr", "1"}, func([]string) int {
		time.Sleep(15 * time.Millisecond)
		return deskkit.ExitRefused
	})
	if rc != deskkit.ExitRefused {
		t.Fatalf("PerfRun rc = %d, want the wrapped verb's %d", rc, deskkit.ExitRefused)
	}
	day := filepath.Join(perfDir, time.Now().UTC().Format("2006-01-02")+".jsonl")
	if _, err := os.Stat(day); err != nil {
		t.Fatalf("the wrapped verb wrote no day file: %v", err)
	}

	rc, out, errOut := runDeskperf("report", "--json", "--tool", "deskpr", "--role", "worker-desk")
	if rc != deskkit.ExitOK {
		t.Fatalf("deskperf report rc %d stderr %q", rc, errOut)
	}
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report --json unparseable: %v\n%s", err, out)
	}
	// PerfRun writes perf-v2: it reads back in the perf-v2 group, and the legacy group is empty.
	v2 := rep.PerfV2
	if rep.Summary.Records != 0 || v2 == nil || v2.Records != 1 || len(v2.Tools) != 1 || len(v2.Slowest) != 1 {
		t.Fatalf("deskperf did not read back exactly the one wrapped record as perf-v2: legacy %+v perf-v2 %+v", rep.Summary, v2)
	}
	st, sl := v2.Tools[0], v2.Slowest[0]
	if st.Tool != "deskpr" || st.Refusals != 1 || st.RefusalRatio != 1 || sl.Verb != "update" || sl.WallMs < 15 ||
		v2.Boundaries["process-init"] != 1 {
		t.Fatalf("the record read back is not the one written: tool %+v slowest %+v boundaries %v", st, sl, v2.Boundaries)
	}

	got, err := os.ReadFile(audit)
	if err != nil || !bytes.Equal(got, ledger) {
		t.Fatalf("audit.jsonl bytes changed: %q, %v", got, err)
	}
	fi, err := os.Stat(audit)
	if err != nil || !fi.ModTime().Equal(stamp) {
		t.Fatalf("audit.jsonl mtime moved: %v, %v", fi.ModTime(), err)
	}
}
