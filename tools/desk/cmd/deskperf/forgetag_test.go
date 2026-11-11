package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// taggedLine is one perf-v5 day-file line — a perf-v2 invocation from a release build, which
// also names the release tag it was stamped with. It is built as JSON text, not through the
// codec, so the history is the wire a release binary writes.
func taggedLine(t *testing.T, hm, sha, tag string, http int64) string {
	t.Helper()
	line := untaggedLine(t, hm, sha, http)
	line = strings.Replace(line, `"schema":"perf-v2"`, `"schema":"perf-v5"`, 1)
	return strings.TrimSuffix(line, "}") + `,"releaseTag":"` + tag + `"}`
}

// untaggedLine is one perf-v2 day-file line: a dev or unstamped build, which names no tag.
func untaggedLine(t *testing.T, hm, sha string, http int64) string {
	t.Helper()
	counters := map[perfrec.Counter]int64{}
	var un []perfrec.Counter
	for _, c := range perfrec.Counters() {
		if c == perfrec.CounterHTTP {
			counters[c] = http
		} else {
			un = append(un, c)
		}
	}
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	b, err := json.Marshal(perfrec.Record{Schema: perfrec.SchemaV2, TS: day + "T" + hm + ":00Z", Tool: "deskpr", Verb: "other",
		Role: "worker-desk", Boundary: perfrec.BoundaryProcess, WallMs: 100, CPU: perfrec.CPUUnsupported,
		Result: perfrec.ResultForExit(0), Install: strings.Repeat("b", 32), SourceSHA: sha, Counters: counters,
		Unavailable: un, Phases: []perfrec.PhaseStat{}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// legacyLine is one perf-v1 invocation line, which predates any release tag.
func legacyLine(t *testing.T, sha string, forge int64) string {
	t.Helper()
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	b, err := json.Marshal(perfrec.Record{Schema: perfrec.Schema, TS: day + "T09:00:00Z", Tool: "deskpost", Verb: "create",
		Role: "worker-desk", WallMs: 100, ForgeCalls: forge, Result: perfrec.ResultForExit(0), Install: strings.Repeat("a", 32), SourceSHA: sha})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// forgeTagReport seeds one day of history and returns the report's text forge section and its
// JSON release rows, each decoded as a raw object so absent and present keys are both visible.
func forgeTagReport(t *testing.T, lines ...string) (string, []map[string]any) {
	t.Helper()
	_, perfDir := seedHome(t)
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if err := os.WriteFile(filepath.Join(perfDir, day+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, out, errOut := runDeskperf("report")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	section := squash(out[max(strings.Index(out, "forge calls —"), 0):])
	rc, js, errOut := runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report --json rc %d stderr %q", rc, errOut)
	}
	var rep struct {
		Forge struct {
			Releases []map[string]any `json:"releases"`
		} `json:"forge"`
	}
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatalf("report --json unparseable: %v\n%s", err, js)
	}
	return section, rep.Forge.Releases
}

func releaseRow(rows []map[string]any, sha string) map[string]any {
	for _, r := range rows {
		if r["key"] == sha {
			return r
		}
	}
	return nil
}

// TestForgeReleaseTagLabel: a release build's row reads "<tag> (<sha>)", keyed on the sha as
// before; a perf-v1 row beside it, which carries no tag, reads as the sha alone.
// regression: #6141
func TestForgeReleaseTagLabel(t *testing.T) {
	section, rows := forgeTagReport(t, taggedLine(t, "10:00", "1a43034839", "v1.0.82", 5), legacyLine(t, "aaaa111", 2))
	for _, want := range []string{"v1.0.82 (1a43034839) 1 0 5 5 5 5 71.4", "\naaaa111 1 0 2 2 2 2 28.6"} {
		if !strings.Contains(section, want) {
			t.Errorf("forge section lacks %q:\n%s", want, section)
		}
	}
	if r := releaseRow(rows, "1a43034839"); r == nil || r["releaseTag"] != "v1.0.82" {
		t.Errorf("JSON release row for 1a43034839 = %v; want key 1a43034839 with releaseTag v1.0.82", r)
	}
}

// TestForgeLegacyShaAlone: records without a tag — perf-v1, and perf-v2 from a dev or
// unstamped build — render the sha alone, and their JSON rows carry no releaseTag key.
func TestForgeLegacyShaAlone(t *testing.T) {
	section, rows := forgeTagReport(t, legacyLine(t, "aaaa111", 2), untaggedLine(t, "10:00", "bbbb222", 3))
	for _, want := range []string{"\nbbbb222 1 0 3", "\naaaa111 1 0 2"} {
		if !strings.Contains(section, want) {
			t.Errorf("forge section lacks %q:\n%s", want, section)
		}
	}
	if strings.Contains(section, "(") && strings.Contains(section, "(bbbb222)") {
		t.Errorf("an untagged row was given a tag label:\n%s", section)
	}
	for _, sha := range []string{"aaaa111", "bbbb222"} {
		r := releaseRow(rows, sha)
		if r == nil {
			t.Fatalf("no JSON release row %s in %v", sha, rows)
		}
		if _, has := r["releaseTag"]; has {
			t.Errorf("untagged row %s carries releaseTag: %v", sha, r)
		}
		if _, has := r["releaseTagConflict"]; has {
			t.Errorf("untagged row %s carries releaseTagConflict: %v", sha, r)
		}
	}
}

// TestForgeSameTagTwoShas: two builds stamped with the same tag stay two rows, one per sha —
// the tag labels a row, it never merges builds.
// regression: #6141
func TestForgeSameTagTwoShas(t *testing.T) {
	section, rows := forgeTagReport(t, taggedLine(t, "10:00", "1a43034839", "v1.0.82", 5),
		taggedLine(t, "11:00", "2b54145940", "v1.0.82", 3))
	for _, want := range []string{"v1.0.82 (1a43034839) 1 0 5", "v1.0.82 (2b54145940) 1 0 3"} {
		if !strings.Contains(section, want) {
			t.Errorf("forge section lacks %q:\n%s", want, section)
		}
	}
	if len(rows) != 2 || releaseRow(rows, "1a43034839")["releaseTag"] != "v1.0.82" ||
		releaseRow(rows, "2b54145940")["releaseTag"] != "v1.0.82" {
		t.Errorf("JSON release rows = %v; want two rows, each keyed on its sha with releaseTag v1.0.82", rows)
	}
}

// TestForgeTagConflictShaOnly: records for one sha that name different tags render the sha
// alone, flagged, and name neither tag: the report never picks one. An untagged record beside
// a tagged one is no disagreement.
// regression: #6141
func TestForgeTagConflictShaOnly(t *testing.T) {
	section, rows := forgeTagReport(t, taggedLine(t, "10:00", "1a43034839", "v1.0.82", 5),
		taggedLine(t, "11:00", "1a43034839", "v1.0.83", 3), untaggedLine(t, "12:00", "1a43034839", 1),
		taggedLine(t, "10:30", "2b54145940", "v1.0.84", 2), untaggedLine(t, "11:30", "2b54145940", 1))
	if !strings.Contains(section, "\n1a43034839 (tag conflict) 3 0 9") {
		t.Errorf("forge section lacks the flagged sha-only row:\n%s", section)
	}
	if strings.Contains(section, "v1.0.82") || strings.Contains(section, "v1.0.83") {
		t.Errorf("a conflicting row names a tag:\n%s", section)
	}
	if !strings.Contains(section, "v1.0.84 (2b54145940) 2 0 3") {
		t.Errorf("an untagged record beside a tagged one dropped the tag:\n%s", section)
	}
	r := releaseRow(rows, "1a43034839")
	if _, has := r["releaseTag"]; has || r["releaseTagConflict"] != true {
		t.Errorf("JSON conflicting row = %v; want no releaseTag and releaseTagConflict true", r)
	}
}

// TestForgeTagJSONFields: --json adds releaseTag beside the existing key and keeps every
// field the release rows already carried, under the same names.
// regression: #6141
func TestForgeTagJSONFields(t *testing.T) {
	_, rows := forgeTagReport(t, taggedLine(t, "10:00", "1a43034839", "v1.0.82", 5))
	r := releaseRow(rows, "1a43034839")
	if r == nil {
		t.Fatalf("no JSON release row 1a43034839 in %v", rows)
	}
	for _, k := range []string{"key", "runs", "measured", "unavailable", "calls", "p50PerRun", "p90PerRun", "maxPerRun",
		"sharePct", "peakHour", "peakHourCalls", "peakHourBudgetPct", "p90RunBudgetPct", "releaseTag"} {
		if _, has := r[k]; !has {
			t.Errorf("JSON release row lacks %q: %v", k, r)
		}
	}
	if r["key"] != "1a43034839" || r["calls"] != float64(5) || r["releaseTag"] != "v1.0.82" {
		t.Errorf("JSON release row = %v; want key 1a43034839, 5 calls, releaseTag v1.0.82", r)
	}
}
