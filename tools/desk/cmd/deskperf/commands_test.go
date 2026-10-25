package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #4346
func TestCommandOverviewExposesExpensiveVerb(t *testing.T) {
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	base := mixedRecords(t)[5]
	var rs []perfrec.Record
	add := func(tool, verb string, wall int64) {
		r := base
		r.Tool, r.Verb, r.WallMs = tool, verb, wall
		rs = append(rs, r)
	}
	add("statusgen", "verifyrun", 100000)
	add("statusgen", "verifyrun", 300000)
	add("statusgen", "verifyrun", 900000)
	for i := 0; i < 20; i++ {
		add("statusgen", "--lint", 10)
	}
	add("issueboard", "issues", 200000)
	add("issueboard", "intake", 5)
	var lines []byte
	for _, r := range rs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(append(lines, b...), '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".jsonl"), lines, 0600); err != nil {
		t.Fatal(err)
	}
	rc, out, errOut := runDeskperf("report", "--top", "0")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc %d: %s", rc, errOut)
	}
	if !strings.HasPrefix(out, "command performance") {
		t.Fatalf("command overview must lead the report:\n%s", out)
	}
	if !strings.Contains(out, "last 7 UTC day(s), including the current partial UTC day") {
		t.Fatalf("overview has no window: %s", out)
	}
	if !strings.Contains(out, "21m40s") || !strings.Contains(out, "5m00s") || !strings.Contains(out, "15m00s") {
		t.Fatalf("overview durations are not readable: %s", out)
	}
	if !strings.Contains(out, "verifyrun") || !strings.Contains(out, "--lint") || !strings.Contains(out, "intake") {
		t.Fatalf("missing command costs:\n%s", out)
	}
	rc, out, errOut = runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc %d: %s", rc, errOut)
	}
	var rep struct {
		Commands struct {
			Rows []struct {
				Tool, Verb              string
				Count                   int
				SumWallMs, P50Ms, P90Ms int64
			}
		}
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Commands.Rows) != 4 {
		t.Fatalf("commands = %+v; want four command rows", rep.Commands.Rows)
	}
	r := rep.Commands.Rows[0]
	if r.Tool != "statusgen" || r.Verb != "verifyrun" || r.Count != 3 || r.SumWallMs != 1300000 || r.P50Ms != 300000 || r.P90Ms != 900000 {
		t.Fatalf("expensive command diluted by cheap lint: %+v", r)
	}
}

// commandProcess uses the actual recorder's selection/codec shape, not an invented
// v3 wire format. The timing fields are fixed below for a hand-computable oracle.
func commandProcess(t *testing.T, session string) perfrec.Record {
	t.Helper()
	in := perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }}
	if session != "" {
		in.Profile = perfrec.Value{Value: "workflow", Present: true}
		in.SessionID = perfrec.Value{Value: session, Present: true}
	}
	p := perfrec.Probe{}
	s := perfrec.BeginSelected(p, p.Mark(), nil, perfrec.ResolveContext(in))
	s.Add(perfrec.CounterHTTP, 0)
	r := s.Finish("statusgen", "verifyrun", "", 0, "")
	r.Install = strings.Repeat("a", 32)
	r.TS = time.Now().UTC().Format(time.RFC3339)
	r.WallMs = 100
	return r
}

// regression: #4346
func TestCommandOverviewTimingCoverageAndOutcomes(t *testing.T) {
	base := mixedRecords(t)[5]
	base.Tool, base.Verb, base.WallMs = "issueboard", "issues", 1000
	base.Phases = []perfrec.PhaseStat{{Name: "fetch", N: 1, Ms: 500000}}
	base.Counters[perfrec.CounterSubprocessMs] = 900000
	zero := int64(0)
	base.CPUMs = &zero
	base.Counters[perfrec.CounterHTTP] = 0
	absent := base
	absent.CPU = perfrec.CPUFailed
	absent.CPUMs = nil
	absent.Counters = map[perfrec.Counter]int64{}
	absent.Unavailable = perfrec.Counters()
	absent.Result = deskkit.ResultRefused
	caller := base
	caller.Boundary = perfrec.BoundaryCaller
	caller.WallMs = 5000
	legacy := mixedRecords(t)[0]
	legacy.Tool, legacy.Verb, legacy.WallMs = "statusgen", "verifyrun", 10
	step := legacy
	step.Verb = "step:token-mint"
	step.WallMs = 999999
	process := commandProcess(t, "")
	process.WallMs = 20000
	client := buildV3(t, v3Spec{ts: process.TS, wall: 1000000 * time.Millisecond, http: 500})
	handler := client
	handler.V3 = cloneCommandV3(t, client.V3)
	handler.V3.Attribution.InvocationID = strings.Repeat("d", 32)
	handler.V3.Kind = perfrec.ObservationHandler
	provider := client
	provider.V3 = cloneCommandV3(t, client.V3)
	provider.V3.Attribution.InvocationID = strings.Repeat("e", 32)
	provider.V3.Kind = perfrec.ObservationProvider
	got, err := deskkit.SummarisePerfCommands([]perfrec.Record{base, absent, caller, legacy, step, process, client, handler, provider}, "", perfrec.V3Filter{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.NonProcess != 3 || len(got.Rows) != 4 {
		t.Fatalf("unexpected invocation population: %+v", got)
	}
	// Global cost ranking keeps the expensive current record ahead of cheap legacy.
	if got.Rows[0].Schema != perfrec.SchemaV3 || got.Rows[0].SumWallMs != 20000 {
		t.Fatalf("largest command hidden behind source family: %+v", got.Rows)
	}
	var row *deskkit.PerfCommandStats
	for i := range got.Rows {
		r := &got.Rows[i]
		if r.Schema == perfrec.SchemaV2 && r.Boundary == perfrec.BoundaryProcess {
			row = r
		}
	}
	if row == nil || row.Count != 2 || row.SumWallMs != 2000 || row.P50Ms != 1000 || row.P90Ms != 1000 || row.CohortSharePct != 100 {
		t.Fatalf("inclusive times or boundaries contaminated process row: %+v", row)
	}
	if row.CPUCount != 1 || row.CPUUnavailable != 1 || row.SumCPUMs != 0 || row.HTTPMeasured != 1 || row.HTTPUnavailable != 1 || row.HTTPAttempts != 0 {
		t.Fatalf("missing observations became zero measurements: %+v", row)
	}
	if row.Outcomes[deskkit.ResultOK] != 1 || row.Outcomes[deskkit.ResultRefused] != 1 {
		t.Fatalf("outcomes collapsed: %+v", row.Outcomes)
	}
	for _, r := range got.Rows {
		if r.Schema == perfrec.Schema && (r.CPUCount != 0 || r.HTTPMeasured != 0 || r.HTTPUnavailable != 1) {
			t.Fatalf("legacy statusgen unobserved counters became measured: %+v", r)
		}
	}
}

func cloneCommandV3(t *testing.T, v *perfrec.V3Data) *perfrec.V3Data {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out perfrec.V3Data
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

// regression: #4346
func TestCommandOverviewSelectsBeforeFilters(t *testing.T) {
	a := commandProcess(t, "")
	conflict := a
	conflict.Tool = "issueboard"
	conflict.TS = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	good := commandProcess(t, "")
	good.WallMs = 700
	invalid := commandProcess(t, "")
	invalid.V3 = nil
	s, err := deskkit.SummarisePerfCommands([]perfrec.Record{a, a, conflict, good, invalid}, "", perfrec.V3Filter{}, "statusgen", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.V3Duplicates != 1 || s.V3Conflicts != 1 || s.V3Invalid != 1 || len(s.Rows) != 1 || s.Rows[0].Count != 1 || s.Rows[0].SumWallMs != 700 {
		t.Fatalf("filter hid a conflict or included invalid/replay record: %+v", s)
	}
	// CLI --tool must exclude the same conflicting invocation from ALL sections.
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a, conflict, good})
	rc, out, errOut := runDeskperf("report", "--tool", "statusgen", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("rc %d: %s", rc, errOut)
	}
	var rep reportJSON
	if err = json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Commands.Rows) != 1 || rep.Commands.Rows[0].Count != 1 || rep.PerfV3 == nil || rep.PerfV3.Records != 1 || rep.PerfV3.Conflicts != 1 {
		t.Fatalf("CLI hid cross-tool conflict: %+v", rep)
	}
}

// regression: #4346
func TestCommandOverviewWorkflowAndSafeLabels(t *testing.T) {
	session := strings.Repeat("b", 32)
	selected := commandProcess(t, session)
	other := commandProcess(t, strings.Repeat("c", 32))
	old := mixedRecords(t)[0]
	rs := []perfrec.Record{selected, other, old}
	if _, err := deskkit.SummarisePerfCommands(rs, "", perfrec.V3Filter{SessionID: session}, "", ""); err == nil {
		t.Fatal("workflow filter bypassed permission")
	}
	got, err := deskkit.SummarisePerfCommands(rs, perfrec.ProfileWorkflow, perfrec.V3Filter{SessionID: session}, "", "")
	if err != nil || len(got.Rows) != 1 || got.Rows[0].Count != 1 || got.Unattributed != 1 {
		t.Fatalf("workflow population wrong: %+v %v", got, err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), session) || strings.Contains(string(b), other.V3.Attribution.SessionID) {
		t.Fatal("command overview exposed workflow identity")
	}
	unsafe := mixedRecords(t)[0]
	unsafe.Tool = "/tmp/example\nunsafe"
	unsafe.Verb = "https://example.invalid/unsafe"
	unsafe.Result = "free-text-result"
	got, err = deskkit.SummarisePerfCommands([]perfrec.Record{unsafe}, "", perfrec.V3Filter{}, "", "")
	if err != nil || len(got.Rows) != 1 || got.Rows[0].Tool != "other" || got.Rows[0].Verb != "other" || got.Rows[0].Outcomes["other"] != 1 {
		t.Fatalf("raw old labels escaped safe vocabulary: %+v %v", got, err)
	}
}

// regression: #4346
func TestCommandOverviewSaturatesTotalsAndKeepsTiesStable(t *testing.T) {
	a := mixedRecords(t)[5]
	a.WallMs = math.MaxInt64
	a.CPUMs = &a.WallMs
	a.Counters = map[perfrec.Counter]int64{perfrec.CounterHTTP: math.MaxInt64}
	b := a
	b.Tool = "issueboard"
	got, err := deskkit.SummarisePerfCommands([]perfrec.Record{a, a, b}, "", perfrec.V3Filter{}, "", "")
	if err != nil || len(got.Rows) != 2 {
		t.Fatalf("summary: %+v %v", got, err)
	}
	for _, r := range got.Rows {
		if r.SumWallMs != math.MaxInt64 || r.SumCPUMs != math.MaxInt64 || r.HTTPAttempts != math.MaxInt64 {
			t.Fatalf("overflow wrapped costs: %+v", r)
		}
	}
	if got.Rows[0].Tool != "deskpr" || got.Rows[1].Tool != "issueboard" {
		t.Fatalf("unstable tied ranking: %+v", got.Rows)
	}
}

// regression: #4346
func TestCommandOverviewConflictOnlyExplainsExclusion(t *testing.T) {
	a := commandProcess(t, "")
	b := a
	b.Tool = "issueboard"
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a, b})
	rc, out, errOut := runDeskperf("report", "--tool", "statusgen")
	if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "excluded 1 conflicting identities") {
		t.Fatalf("exclusion was hidden behind generic no-match: rc%d stdout%q stderr%q", rc, out, errOut)
	}
}

// regression: #4346 (review c4352-f1)
func TestCommandOverviewUnfilteredConflictOnlyKeepsDetail(t *testing.T) {
	a := commandProcess(t, "")
	b := a
	b.Tool = "issueboard"
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a, b})
	for _, asJSON := range []bool{false, true} {
		args := []string{"report"}
		if asJSON {
			args = append(args, "--json")
		}
		rc, out, errOut := runDeskperf(args...)
		if rc != 0 || errOut != "" {
			t.Errorf("json=%v: conflict-only detail lost: rc%d stdout%q stderr%q", asJSON, rc, out, errOut)
			continue
		}
		if asJSON {
			var got reportJSON
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if got.PerfV3 == nil || got.PerfV3.Records != 0 || got.PerfV3.Conflicts != 1 {
				t.Errorf("conflict-only perfV3 counters missing: %+v", got.PerfV3)
			}
		} else if !strings.Contains(out, "perf-v3 — 0 records; 0 duplicates; 1 conflicting identities excluded; 0 invalid") {
			t.Errorf("conflict-only detail missing: %q", out)
		}
	}
}

// regression: #4346 (review c4352-f1)
func TestCommandOverviewUnfilteredInvalidOnlyKeepsDetail(t *testing.T) {
	a := commandProcess(t, "")
	// The real loader retains a missing-install record as an invalid source.
	a.Install = ""
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a})
	for _, asJSON := range []bool{false, true} {
		args := []string{"report"}
		if asJSON {
			args = append(args, "--json")
		}
		rc, out, errOut := runDeskperf(args...)
		if rc != 0 || errOut != "" {
			t.Fatalf("json=%v: invalid-only detail lost: rc%d stdout%q stderr%q", asJSON, rc, out, errOut)
		}
		if asJSON {
			var got reportJSON
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if got.PerfV3 == nil || got.PerfV3.Records != 0 || got.PerfV3.Invalid != 1 {
				t.Errorf("invalid-only perfV3 counters missing: %+v", got.PerfV3)
			}
		} else if !strings.Contains(out, "perf-v3 — 0 records; 0 duplicates; 0 conflicting identities excluded; 1 invalid") {
			t.Errorf("invalid-only detail missing: %q", out)
		}
	}
}

// regression: #4346 (review c4352-f1)
func TestCommandOverviewV3OnlyWorkflowMissKeepsReason(t *testing.T) {
	a := commandProcess(t, strings.Repeat("a", 32))
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a})
	for _, tool := range []string{"", a.Tool} {
		for _, byDay := range []bool{false, true} {
			for _, asJSON := range []bool{false, true} {
				args := []string{"report", "--perf-profile", "workflow", "--session-id", strings.Repeat("b", 32)}
				if tool != "" {
					args = append(args, "--tool", tool)
				}
				if byDay {
					args = append(args, "--by-day")
				}
				if asJSON {
					args = append(args, "--json")
				}
				rc, out, errOut := runDeskperf(args...)
				if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "no v3 record matches the workflow filter") {
					t.Errorf("tool=%q byDay=%v json=%v: workflow refusal changed: rc%d stdout%q stderr%q", tool, byDay, asJSON, rc, out, errOut)
				}
			}
		}
	}
	for _, byDay := range []bool{false, true} {
		args := []string{"report", "--perf-profile", "workflow", "--session-id", strings.Repeat("b", 32), "--tool", "other-tool"}
		if byDay {
			args = append(args, "--by-day")
		}
		rc, out, errOut := runDeskperf(args...)
		if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "no record matches --tool other-tool") {
			t.Errorf("byDay=%v: missing-tool refusal changed: rc%d stdout%q stderr%q", byDay, rc, out, errOut)
		}
		args[4] = strings.Repeat("a", 32)
		args[6] = a.Tool
		rc, out, errOut = runDeskperf(args...)
		if rc != 0 || out == "" || errOut != "" {
			t.Errorf("byDay=%v: matching workflow failed: rc%d stdout%q stderr%q", byDay, rc, out, errOut)
		}
	}
}

// regression: #4346 (review A1)
func TestCommandOverviewUnattributedUsesCommandFilters(t *testing.T) {
	session := strings.Repeat("a", 32)
	selected := commandProcess(t, session)
	old := mixedRecords(t)[0]
	old.Tool, old.Role = "statusgen", "worker-desk"
	otherTool := old
	otherTool.Tool = "issueboard"
	otherRole := old
	otherRole.Role = "reviewer"
	for _, role := range []string{"", "worker-desk"} {
		got, err := deskkit.SummarisePerfCommands([]perfrec.Record{selected, old, otherTool, otherRole}, perfrec.ProfileWorkflow, perfrec.V3Filter{SessionID: session}, "statusgen", role)
		want := 2
		if role != "" {
			want = 1
		}
		if err != nil || got.Unattributed != want {
			t.Errorf("role=%q: got %d unattributed, want %d: %v", role, got.Unattributed, want, err)
		}
	}
}

// regression: #4346
func TestCommandOverviewReadableDurations(t *testing.T) {
	for _, c := range []struct {
		ms   int64
		want string
	}{
		{0, "0ms"}, {999, "999ms"}, {1000, "1.000s"}, {59999, "59.999s"}, {60000, "1m00s"},
		{3599999, "59m59s"}, {3600000, "1h00m00s"}, {81317776, "22h35m17s"}, {math.MaxInt64, "2562047788015h12m55s"},
	} {
		if got := commandDuration(c.ms); got != c.want {
			t.Errorf("%dms: %s, want %s", c.ms, got, c.want)
		}
	}
}

// regression: #4360 (review F2)
func TestCommandOverviewV4ExclusionsVisible(t *testing.T) {
	for _, tc := range []struct {
		name    string
		summary deskkit.PerfCommandSummary
		want    string
	}{
		{"duplicates", deskkit.PerfCommandSummary{V4Duplicates: 2}, "excluded perf-v4: 2 duplicates; 0 conflicting identities; 0 invalid"},
		{"conflicts", deskkit.PerfCommandSummary{V4Conflicts: 3}, "excluded perf-v4: 0 duplicates; 3 conflicting identities; 0 invalid"},
		{"invalid", deskkit.PerfCommandSummary{V4Invalid: 1}, "excluded perf-v4: 0 duplicates; 0 conflicting identities; 1 invalid"},
		{"mixed", deskkit.PerfCommandSummary{V3Conflicts: 1, V4Duplicates: 2, V4Conflicts: 3, V4Invalid: 1}, "excluded perf-v4: 2 duplicates; 3 conflicting identities; 1 invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := commandTable(&out, tc.summary, 7); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("v4 exclusions missing: %q", out.String())
			}
		})
	}
}

// regression: #4360 (review 2 F2)
func TestCommandOverviewByDayV4ExclusionsVisible(t *testing.T) {
	a, err := perfrec.ToV4(commandProcess(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.Tool = "issueboard"
	c, err := perfrec.ToV4(commandProcess(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a, b, c})
	rc, out, errOut := runDeskperf("report", "--by-day")
	if rc != 0 || errOut != "" {
		t.Fatalf("by-day report failed: rc%d stdout%q stderr%q", rc, out, errOut)
	}
	if !strings.Contains(out, "perf-v4 — 1 record(s) kept; 0 duplicate(s); 1 conflicting identities excluded; 0 invalid (selected over the whole window before any day bucket)") {
		t.Errorf("window-wide v4 exclusion hidden in by-day text: %q", out)
	}
	if strings.Contains(out, "0 conflicting identities excluded") {
		t.Errorf("by-day text restates an exclusion as an affirmative per-bucket zero: %q", out)
	}
	if !strings.Contains(out, "perf-v4 — 1 records in this bucket (exclusions are counted once for the whole window, above)") {
		t.Errorf("per-day v4 bucket line missing: %q", out)
	}
	rc, out, errOut = runDeskperf("report", "--by-day", "--json")
	if rc != 0 || errOut != "" {
		t.Fatalf("by-day json failed: rc%d stdout%q stderr%q", rc, out, errOut)
	}
	var got reportDayJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.PerfV4 == nil || got.PerfV4.Records != 1 || got.PerfV4.Conflicts != 1 {
		t.Errorf("window-wide v4 exclusion missing from by-day json: %+v", got.PerfV4)
	}
}

// regression: #4360 (parent c4352-f1)
func TestCommandWorkflowV4(t *testing.T) {
	a, err := perfrec.ToV4(commandProcess(t, strings.Repeat("a", 32)))
	if err != nil {
		t.Fatal(err)
	}
	_, dir := seedHome(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writePerfDay(t, dir, time.Now().UTC().Format("2006-01-02"), []perfrec.Record{a})
	for _, byDay := range []bool{false, true} {
		for _, asJSON := range []bool{false, true} {
			for _, tool := range []string{a.Tool, "other-tool"} {
				args := []string{"report", "--perf-profile", "workflow", "--session-id", strings.Repeat("b", 32), "--tool", tool}
				if byDay {
					args = append(args, "--by-day")
				}
				if asJSON {
					args = append(args, "--json")
				}
				want := "no v3/v4 record matches the workflow filter"
				if tool != a.Tool {
					want = "no record matches --tool other-tool"
				}
				rc, out, diag := runDeskperf(args...)
				if rc != deskkit.ExitRefused || out != "" || !strings.Contains(diag, want) {
					t.Errorf("tool=%q day=%v json=%v: rc%d out%q diag%q; want %q", tool, byDay, asJSON, rc, out, diag, want)
				}
			}
		}
	}
}
