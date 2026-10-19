package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type perfRow struct {
	Tool   string `json:"tool"`
	Verb   string `json:"verb"`
	WallMs int64  `json:"wallMs"`
}

// perfRowsIn reads every perf-v1 row under a state directory, in file order.
func perfRowsIn(t *testing.T, state string) []perfRow {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(state, "perf", "????-??-??.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []perfRow
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var r perfRow
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				t.Fatalf("bad perf line %q: %v", line, err)
			}
			rows = append(rows, r)
		}
	}
	return rows
}

func verbsOf(rows []perfRow) []string {
	var v []string
	for _, r := range rows {
		v = append(v, r.Verb)
	}
	return v
}

// regression: #3099
func TestPerfRecordsRealVerbNames(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.plainCell(t)
	state := filepath.Join(f.cellDir, ".config", "assay")
	for _, args := range [][]string{{"version"}, {"ls"}, {"show", "example"}} {
		if r := f.run(t, []string{"ASSAY_TELEMETRY=1"}, args...); r.code != 0 {
			t.Fatalf("cellctl %v: %+v", args, r)
		}
	}
	// A leading --cells-root selector must not turn the verb into "other".
	if r := f.run(t, []string{"ASSAY_TELEMETRY=1"}, "--cells-root", f.cellsRoot, "version"); r.code != 0 {
		t.Fatalf("selector run: %+v", r)
	}
	got := verbsOf(perfRowsIn(t, state))
	want := []string{"version", "ls", "show", "version"}
	if !slices.Equal(got, want) {
		t.Fatalf("recorded verbs = %v, want %v (every verb recorded as other)", got, want)
	}
}

// regression: #3099
func TestPerfLaunchWritesPreHandoverSteps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub harness is a shell script")
	}
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	state := filepath.Join(f.cellDir, ".config", "assay")
	snap := filepath.Join(f.cellsRoot, "perf-at-handover.jsonl")
	// The stub harness copies the perf file the instant it starts: whatever the launch wrote
	// BEFORE handing over is what lands in the snapshot.
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo '2.1.278 (stub)'; exit 0;;\nplugin) exit 0;;\nesac\n" +
		"cat \"$PERF_STATE\"/perf/*.jsonl > \"$PERF_SNAPSHOT\" 2>/dev/null\nexit 0\n"
	must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0o700))
	r := f.run(t, []string{"CELLCTL_DESKWT=0", "ASSAY_TELEMETRY=1", "PERF_STATE=" + state, "PERF_SNAPSHOT=" + snap},
		"desk", "example", "worker-desk", "--cadence", "off")
	if r.code != 0 {
		t.Fatalf("launch exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
	}

	steps := []string{"step:launch-role-lease", "step:launch-storage-admission", "step:launch-plugin-enable",
		"step:launch-fetch-main", "step:launch-worktree"}
	raw, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("harness never saw a perf file: %v", err)
	}
	var atHandover []perfRow
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var row perfRow
		if line != "" && json.Unmarshal([]byte(line), &row) == nil {
			atHandover = append(atHandover, row)
		}
	}
	for _, s := range steps {
		if !slices.Contains(verbsOf(atHandover), s) {
			t.Errorf("step %q not on disk when the harness started; saw %v", s, verbsOf(atHandover))
		}
	}
	for _, row := range atHandover {
		if row.Tool != "cellctl" {
			t.Errorf("row %+v: want tool cellctl", row)
		}
	}
	// And the launch's own verb-level record lands afterwards under its real name.
	all := verbsOf(perfRowsIn(t, state))
	if len(all) == 0 || all[len(all)-1] != "desk" {
		t.Errorf("final record = %v, want the desk verb last", all)
	}
}
