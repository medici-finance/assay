package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// intakeexits_test.go — desk-supervision/33: the intake register's half of the intake-exit-v1
// record, its triage-stamp lint, and the schema-document pin shared with scanloop.

const stampOK = "triaged: \"2026-08-25\"\ntriaged-by: intake-desk\ntriager-tier: any\n"

func intakeFile(id, date, disposition, extra string) string {
	return "---\nid: " + id + "\ndate: \"" + date + "\"\ntitle: A title that must never be exported\n" +
		"disposition: " + disposition + "\n" + extra + "---\n\nA body that must never be exported.\n"
}

// intakeRoot writes a flat per-entry intake register holding files, keyed by name.
func intakeRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, dir)
	for name, body := range files {
		writeTemp(t, dir, name, body)
	}
	return root
}

func problemsFor(root, id string) []string {
	var out []string
	for _, p := range registerIntegrityProblems(root) {
		if strings.Contains(p, id+":") {
			out = append(out, p)
		}
	}
	return out
}

// exportIntakeExits runs the --json export and splits it into records and the summary.
func exportIntakeExits(t *testing.T, root string) ([]intakeExitRecord, intakeExitSummary, []string) {
	t.Helper()
	var out, errw bytes.Buffer
	if code := writeIntakeExits(root, true, &out, &errw); code != 0 {
		t.Fatalf("export exit %d: %s", code, errw.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("the export printed nothing; it must always end with the summary line")
	}
	var sum intakeExitSummary
	dec := json.NewDecoder(strings.NewReader(lines[len(lines)-1]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sum); err != nil {
		t.Fatalf("last line is not the summary object: %v\n%s", err, lines[len(lines)-1])
	}
	var recs []intakeExitRecord
	for _, l := range lines[:len(lines)-1] {
		var r intakeExitRecord
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("record line does not parse: %v\n%s", err, l)
		}
		recs = append(recs, r)
	}
	return recs, sum, lines
}

func TestIntakeExits_MappingAndStamp(t *testing.T) {
	type want struct{ exit, detail, artifact string }
	mapped := map[string]struct {
		disposition, extra string
		want               want
	}{
		"scoped to a stream":         {"scoped", "scoped-to: desk-supervision\n", want{"placeholder", "", "desk-supervision"}},
		"scoped to a brief":          {"scoped", "scoped-to: desk-supervision/33\n", want{"placeholder", "", "desk-supervision/33"}},
		"scoped to issue #NN":        {"scoped", "scoped-to: \"issue #42\"\n", want{"bug", "", "#42"}},
		"scoped to a finding":        {"scoped", "scoped-to: F-desk-emits-briefs\n", want{"finding", "", "F-desk-emits-briefs"}},
		"decision-needed with issue": {"decision-needed", "decision-issue: \"77\"\n", want{"needs-decision", "", "#77"}},
		"watching":                   {"watching", "", want{"rejected-watching", "watching", ""}},
		"rejected":                   {"rejected", "why: out of scope\n", want{"rejected-watching", "rejected", ""}},
		"legacy adopted":             {"adopted", "scoped-to: desk-tools\n", want{"placeholder", "", "desk-tools"}},
		"legacy inline scoped":       {"\"scoped → desk-tools/07\"", "", want{"placeholder", "", "desk-tools/07"}},
		"legacy inline decision":     {"\"decision-needed → issue #9\"", "", want{"needs-decision", "", "#9"}},
		"legacy inline rejected":     {"\"rejected — superseded\"", "", want{"rejected-watching", "rejected", ""}},
	}
	for name, tc := range mapped {
		t.Run("maps/"+name, func(t *testing.T) {
			root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-mapped-entry", "2026-08-24", tc.disposition, tc.extra+stampOK)})
			recs, sum, lines := exportIntakeExits(t, root)
			if sum != (intakeExitSummary{Stamped: 1}) || len(recs) != 1 {
				t.Fatalf("summary %+v, %d records; want exactly one stamped record", sum, len(recs))
			}
			r := recs[0]
			got := want{r.Exit, r.Detail, r.Artifact}
			if got != tc.want {
				t.Fatalf("mapped to %+v, want %+v", got, tc.want)
			}
			if r.Schema != intakeExitSchema || r.Source != "intake" || r.Item != "I-mapped-entry" ||
				r.DecidedBy != "judgment" || r.TriagerRole != "intake-desk" || r.TriagerTier != "any" ||
				r.Triaged != "2026-08-25T00:00:00Z" || r.Opened != "2026-08-24T00:00:00Z" ||
				r.Kind != "" || r.Trust != "" || r.SessionTag != "" || r.DispatchRef != "" {
				t.Fatalf("record = %+v", r)
			}
			for _, l := range lines {
				if strings.Contains(l, "must never be exported") {
					t.Fatalf("a title or body reached the export: %s", l)
				}
			}
		})
	}

	unmapped := map[string]string{
		"decision-needed without issue": intakeFile("I-unmapped-a", "2026-08-24", "decision-needed", stampOK),
		"value outside vocabulary":      intakeFile("I-unmapped-b", "2026-08-24", "parked", stampOK),
		"scoped to free text":           intakeFile("I-unmapped-c", "2026-08-24", "scoped", "scoped-to: the console work\n"+stampOK),
		"scoped with no target":         intakeFile("I-unmapped-d", "2026-08-24", "scoped", ""),
	}
	for name, body := range unmapped {
		t.Run("unmapped/"+name, func(t *testing.T) {
			recs, sum, _ := exportIntakeExits(t, intakeRoot(t, map[string]string{"e.md": body}))
			if sum != (intakeExitSummary{Unmapped: 1}) || len(recs) != 0 {
				t.Fatalf("summary %+v, %d records; want one unmapped, nothing exported", sum, len(recs))
			}
		})
	}

	t.Run("unstamped and partial stamps are counted not exported", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{
			"a.md": intakeFile("I-no-stamp", "2026-08-24", "watching", ""),
			"b.md": intakeFile("I-partial-stamp", "2026-08-24", "watching", "triaged-by: intake-desk\n"),
			"c.md": intakeFile("I-bad-stamp", "2026-08-24", "watching", "triaged: \"2026-08-25\"\ntriaged-by: alice-dev\ntriager-tier: any\n"),
			"d.md": intakeFile("I-stamped", "2026-08-24", "watching", stampOK),
			"e.md": intakeFile("I-untriaged", "2026-08-24", "new", stampOK),
		})
		recs, sum, _ := exportIntakeExits(t, root)
		if sum != (intakeExitSummary{Stamped: 1, Unstamped: 3}) {
			t.Fatalf("summary %+v, want 1 stamped + 3 unstamped (and `new` in no count)", sum)
		}
		if len(recs) != 1 || recs[0].Item != "I-stamped" {
			t.Fatalf("records = %+v", recs)
		}
	})

	t.Run("bad triager-tier is a lint PROBLEM", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-bad-tier", "2026-08-24", "watching",
			"triaged: \"2026-08-25\"\ntriaged-by: intake-desk\ntriager-tier: claude-opus\n")})
		ps := problemsFor(root, "I-bad-tier")
		if len(ps) != 1 || !strings.Contains(ps[0], "triager-tier") || !strings.Contains(ps[0], "claude-opus") {
			t.Fatalf("problems = %v", ps)
		}
	})

	t.Run("bad triaged is a lint PROBLEM", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-bad-time", "2026-08-24", "watching",
			"triaged: last tuesday\n")})
		if ps := problemsFor(root, "I-bad-time"); len(ps) != 1 || !strings.Contains(ps[0], "triaged") {
			t.Fatalf("problems = %v", ps)
		}
	})

	t.Run("login-shaped triaged-by alice-dev is a lint PROBLEM", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-login-role", "2026-08-24", "watching",
			"triaged-by: alice-dev\n")})
		ps := problemsFor(root, "I-login-role")
		if len(ps) != 1 || !strings.Contains(ps[0], "triaged-by") || !strings.Contains(ps[0], "closed role set") {
			t.Fatalf("problems = %v", ps)
		}
	})

	t.Run("a long triaged-by is not echoed back", func(t *testing.T) {
		long := strings.Repeat("pasted-text-", 20)
		root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-long-role", "2026-08-24", "watching",
			"triaged-by: "+long+"\n")})
		ps := problemsFor(root, "I-long-role")
		if len(ps) != 1 || strings.Contains(ps[0], long[:len("pr-review-desk")+1]) {
			t.Fatalf("the PROBLEM echoed more than the closed set's length: %v", ps)
		}
	})

	for _, role := range intakeExitRoles {
		t.Run("closed-set member lints clean/"+role, func(t *testing.T) {
			root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-good-role", "2026-08-24", "watching",
				"triaged: \"2026-08-25T10:00:00Z\"\ntriaged-by: "+role+"\ntriager-tier: strong\n")})
			if ps := problemsFor(root, "I-good-role"); len(ps) != 0 {
				t.Fatalf("closed-set member %q is a PROBLEM: %v", role, ps)
			}
		})
	}

	t.Run("absent keys lint clean", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{"e.md": intakeFile("I-no-keys", "2026-08-24", "watching", "")})
		if ps := problemsFor(root, "I-no-keys"); len(ps) != 0 {
			t.Fatalf("an entry with none of the new keys is a PROBLEM: %v", ps)
		}
	})

	// Neighbour: the three keys are additive. intake-debt reads only `disposition`, so an entry
	// with none of them counts as untriaged / triaged exactly as before, and adding a stamp to a
	// `new` entry does not make it triaged.
	t.Run("intake-debt neighbour unchanged", func(t *testing.T) {
		root := intakeRoot(t, map[string]string{
			"a.md": intakeFile("I-debt-new", "2026-08-01", "new", ""),
			"b.md": intakeFile("I-debt-new-stamped", "2026-08-01", "new", stampOK),
			"c.md": intakeFile("I-debt-scoped", "2026-08-01", "scoped", "scoped-to: desk-tools\n"),
			"d.md": intakeFile("I-debt-watching", "2026-08-01", "watching", stampOK),
		})
		entries, err := loadIntake(root)
		if err != nil {
			t.Fatal(err)
		}
		res := intakeAlarm(entries, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))
		if res.Untriaged != 2 {
			t.Fatalf("untriaged = %d, want 2 (both `new` entries, stamped or not)", res.Untriaged)
		}
	})

	t.Run("no register at all is could-not-check", func(t *testing.T) {
		var out, errw bytes.Buffer
		if code := writeIntakeExits(t.TempDir(), true, &out, &errw); code != 6 {
			t.Fatalf("exit %d, want 6 (could-not-check)", code)
		}
		if out.Len() != 0 {
			t.Fatalf("a could-not-check export printed records: %s", out.String())
		}
	})

	t.Run("index with no entries is a zero summary", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "streams")
		mustMkdirAll(t, dir)
		writeTemp(t, dir, "INTAKE.md", "# Intake\n\nNo intake entries yet.\n")
		recs, sum, _ := exportIntakeExits(t, root)
		if len(recs) != 0 || sum != (intakeExitSummary{}) {
			t.Fatalf("records %v, summary %+v; want none and all-zero", recs, sum)
		}
	})
}

// schemaDocPath is the schema document, relative to this package directory.
const schemaDocPath = "../spec/intake-exit-v1.md"

// schemaDocColumn returns the backticked first-column values of the table under the "## " heading
// named section.
func schemaDocColumn(t *testing.T, doc, section string) []string {
	t.Helper()
	var out []string
	in := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == section
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		first := strings.TrimSpace(cells[1])
		if strings.HasPrefix(first, "`") && strings.HasSuffix(first, "`") && len(first) > 2 {
			out = append(out, strings.Trim(first, "`"))
		}
	}
	if len(out) == 0 {
		t.Fatalf("the schema document has no %q table", section)
	}
	return out
}

// TestIntakeExitSchema_MatchesDoc pins this writer to the schema document: its JSON keys (set AND
// order) and its closed role set. scanloop carries the twin of this test.
func TestIntakeExitSchema_MatchesDoc(t *testing.T) {
	raw, err := os.ReadFile(schemaDocPath)
	if err != nil {
		t.Fatalf("reading the schema document: %v", err)
	}
	doc := string(raw)

	var keys []string
	rt := reflect.TypeOf(intakeExitRecord{})
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		keys = append(keys, name)
	}
	if docKeys := schemaDocColumn(t, doc, "Fields"); !reflect.DeepEqual(keys, docKeys) {
		t.Errorf("record JSON keys differ from the schema document\n record: %v\n    doc: %v", keys, docKeys)
	}

	a := append([]string(nil), intakeExitRoles...)
	b := schemaDocColumn(t, doc, "Closed role set")
	sort.Strings(a)
	sort.Strings(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("closed role set differs from the schema document\n writer: %v\n    doc: %v", a, b)
	}
}
