package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// withdrawnFM renders a brief-v3 frontmatter for stream alpha with the given
// number, wave, depends list and extra (withdrawn) keys.
func withdrawnFM(num string, wave int, depends, extra string) string {
	return schemaFM("brief-v3", num, wave, depends, extra)
}

// schemaFM renders a frontmatter of the given brief schema (v1, or the v2
// family with the hierarchical tc:tr:alpha:NN id).
func schemaFM(schema, num string, wave int, depends, extra string) string {
	if depends == "" {
		depends = "[]"
	}
	id := "alpha/" + num
	v2 := ""
	if schema != "brief-v1" {
		id = "tc:tr:alpha:" + num
		v2 = "version: 1\n"
	}
	return "---\n" +
		"schema: " + schema + "\n" +
		"brief: " + id + "\n" + v2 +
		"title: t" + num + "\n" +
		"wave: " + string(rune('0'+wave)) + "\n" +
		"depends: " + depends + "\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"why: a real rationale that clears the substance floor for this fixture\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"authored: 2026-07-08 by t\n" +
		"sources: [\"s\"]\n" +
		extra +
		"---\n# Brief " + num + "\n"
}

const validWithdrawal = "withdrawn-date: \"2026-10-18\"\n" +
	"superseded-by: [\"alpha/02\"]\n" +
	"withdrawn-reason: \"Retired by a later decision; the scope moved to alpha/02.\"\n"

type wdRow struct {
	num, status string
	wave        int
	file        string // "" = no brief file (legacy row)
}

// withdrawnTree writes a one-stream tree (stream alpha) and returns the loaded
// streams plus the checkBriefFiles problems and notices.
func withdrawnTree(t *testing.T, rows []wdRow) ([]*Stream, []string, []string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("---\nstream: alpha\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# Alpha\n\n")
	b.WriteString("| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n")
	for _, r := range rows {
		name := "brief-" + r.num + "-x.md"
		b.WriteString("| " + r.num + " | [T" + r.num + "](./" + name + ") | " + string(rune('0'+r.wave)) + " | S | " + r.status + " | — | — |\n")
		if r.file != "" {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(r.file), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: tc\nrepos:\n  tr: {cell: tc, repo: medici-finance/assay}\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	p, n := checkBriefFiles(streams, streams)
	return streams, p, n
}

func linesFor(lines []string, sub string) []string {
	var out []string
	for _, l := range lines {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return out
}

func TestWithdrawnLintRules(t *testing.T) {
	cases := []struct {
		name       string
		row        wdRow
		wantProb   []string // substrings of one PROBLEM about this brief ("" = none expected)
		wantNotice []string
	}{
		{name: "valid withdrawn", row: wdRow{num: "01", status: "withdrawn", file: withdrawnFM("01", 0, "", validWithdrawal)}},
		{name: "won't-fix empty successor list", row: wdRow{num: "03", status: "withdrawn", file: withdrawnFM("03", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: []\nwithdrawn-reason: \"No longer needed.\"\n")}},
		{name: "cross-repo global id, root not configured", row: wdRow{num: "04", status: "withdrawn", file: withdrawnFM("04", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"cell:other:beta:30\"]\nwithdrawn-reason: \"Moved to another repo.\"\n")}},
		{name: "unquoted date accepted", row: wdRow{num: "05", status: "withdrawn", file: withdrawnFM("05", 0, "",
			"withdrawn-date: 2026-10-18\nsuperseded-by: []\nwithdrawn-reason: \"No longer needed.\"\n")}},
		{name: "missing reason", row: wdRow{num: "10", status: "withdrawn", file: withdrawnFM("10", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: []\n")}, wantProb: []string{"brief-10", "withdrawn-reason"}},
		{name: "empty reason", row: wdRow{num: "11", status: "withdrawn", file: withdrawnFM("11", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: []\nwithdrawn-reason: \"  \"\n")}, wantProb: []string{"brief-11", "withdrawn-reason"}},
		{name: "missing date", row: wdRow{num: "12", status: "withdrawn", file: withdrawnFM("12", 0, "",
			"superseded-by: []\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-12", "no withdrawn-date"}},
		{name: "malformed date", row: wdRow{num: "13", status: "withdrawn", file: withdrawnFM("13", 0, "",
			"withdrawn-date: \"18 Oct\"\nsuperseded-by: []\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-13", "not YYYY-MM-DD"}},
		{name: "keys on a non-withdrawn brief", row: wdRow{num: "14", status: "todo", file: withdrawnFM("14", 0, "", validWithdrawal)},
			wantProb: []string{"brief-14", "withdrawn-date, withdrawn-reason, superseded-by", "\"todo\""}},
		{name: "one key on a non-withdrawn brief", row: wdRow{num: "15", status: "done", file: withdrawnFM("15", 0, "", "superseded-by: []\n")},
			wantProb: []string{"brief-15", "carries superseded-by"}},
		{name: "unresolved same-repo successor", row: wdRow{num: "16", status: "withdrawn", file: withdrawnFM("16", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"alpha/99\"]\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-16", "alpha/99", "does not resolve"}},
		{name: "unknown-stream successor", row: wdRow{num: "17", status: "withdrawn", file: withdrawnFM("17", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"nostream/01\"]\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-17", "nostream/01", "does not resolve"}},
		{name: "malformed cross-repo id", row: wdRow{num: "18", status: "withdrawn", file: withdrawnFM("18", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"other:beta:30\"]\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-18", "other:beta:30", "is not a brief id"}},
		{name: "self successor", row: wdRow{num: "19", status: "withdrawn", file: withdrawnFM("19", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"alpha/19\"]\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-19", "supersede itself"}},
		{name: "superseded-by wrong type", row: wdRow{num: "20", status: "withdrawn", file: withdrawnFM("20", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: {a: b}\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-20", "superseded-by"}},
		{name: "withdrawn legacy row without a typed file", row: wdRow{num: "21", status: "withdrawn"},
			wantProb: []string{"alpha/brief-21", "requires a schema brief-v3 brief file"}},
		{name: "global id naming this repo resolves", row: wdRow{num: "06", status: "withdrawn", file: withdrawnFM("06", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"tc:tr:alpha:02\"]\nwithdrawn-reason: \"x\"\n")}},
		{name: "global id naming this repo, unresolved", row: wdRow{num: "22", status: "withdrawn", file: withdrawnFM("22", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"tc:tr:alpha:98\"]\nwithdrawn-reason: \"x\"\n")}, wantProb: []string{"brief-22", "tc:tr:alpha:98", "does not resolve"}},
		{name: "withdrawn brief on schema brief-v2", row: wdRow{num: "23", status: "withdrawn", file: schemaFM("brief-v2", "23", 0, "", validWithdrawal)},
			wantProb: []string{"brief-23", "must be schema brief-v3"}},
		{name: "withdrawn brief on schema brief-v1", row: wdRow{num: "24", status: "withdrawn", file: schemaFM("brief-v1", "24", 0, "", "")},
			wantProb: []string{"brief-24", "must be schema brief-v3"}},
		// Backward compatibility: on brief-v1/brief-v2 the three keys are unknown
		// keys, tolerated as written — boards already carry a prose
		// `superseded-by:` string on blocked/implemented/done briefs.
		{name: "withdrawn keys on a brief-v1 live brief are ignored", row: wdRow{num: "25", status: "todo", file: schemaFM("brief-v1", "25", 0, "", validWithdrawal)}},
		{name: "v2 prose superseded-by string, status blocked", row: wdRow{num: "27", status: "blocked", file: schemaFM("brief-v2", "27", 0, "",
			"superseded-by: \"merged into the follow-up brief; see its README row\"\n")}},
		{name: "v2 prose superseded-by string, status done", row: wdRow{num: "28", status: "done", file: schemaFM("brief-v2", "28", 0, "",
			"superseded-by: \"merged into the follow-up brief; see its README row\"\n")}},
		{name: "v1 prose superseded-by string, status implemented", row: wdRow{num: "29", status: "implemented", file: schemaFM("brief-v1", "29", 0, "",
			"superseded-by: \"merged elsewhere\"\nwithdrawn-date: someday\n")}},
		{name: "v3 prose superseded-by string is a PROBLEM", row: wdRow{num: "32", status: "blocked", file: withdrawnFM("32", 0, "",
			"superseded-by: \"merged into the follow-up brief\"\n")}, wantProb: []string{"brief-32", "superseded-by"}},
		{name: "live brief-v3 brief without keys is valid", row: wdRow{num: "26", status: "todo", file: withdrawnFM("26", 0, "", "")}},
		{name: "depends on withdrawn brief", row: wdRow{num: "30", status: "todo", wave: 1, file: withdrawnFM("30", 1, "[\"alpha/01\"]", "")},
			wantNotice: []string{"brief-30", "depends on alpha/01, which is withdrawn", "successor alpha/02"}},
		{name: "depends on won't-fix brief", row: wdRow{num: "31", status: "todo", wave: 1, file: withdrawnFM("31", 1, "[\"alpha/03\"]", "")},
			wantNotice: []string{"brief-31", "alpha/03, which is withdrawn", "no successor"}},
	}
	// One tree holding every case, plus the successor brief alpha/02.
	rows := []wdRow{{num: "02", status: "todo", file: withdrawnFM("02", 0, "", "")}}
	for _, c := range cases {
		rows = append(rows, c.row)
	}
	_, problems, notices := withdrawnTree(t, rows)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key := "brief-" + c.row.num
			got := linesFor(problems, key)
			if len(c.wantProb) == 0 {
				var mine []string
				for _, p := range got {
					if strings.Contains(p, "withdrawn") || strings.Contains(p, "superseded") {
						mine = append(mine, p)
					}
				}
				if len(mine) > 0 {
					t.Errorf("want no withdrawn PROBLEM for %s; got:\n%s", key, strings.Join(mine, "\n"))
				}
			} else if !hasProblem(problems, c.wantProb...) {
				t.Errorf("want PROBLEM %q; got:\n%s", c.wantProb, strings.Join(problems, "\n"))
			}
			if len(c.wantNotice) > 0 && !hasProblem(notices, c.wantNotice...) {
				t.Errorf("want NOTICE %q; got:\n%s", c.wantNotice, strings.Join(notices, "\n"))
			}
		})
	}
	// The successor's own clean row raises no depends NOTICE.
	if n := linesFor(notices, "brief-02-x.md: depends on"); len(n) > 0 {
		t.Errorf("unexpected depends NOTICE on alpha/02: %v", n)
	}
}

// The README Status cell accepts the bare withdrawn token, and the
// invalid-status message lists it.
func TestWithdrawnStatusTokenValid(t *testing.T) {
	ok := []*Stream{mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "withdrawn"})}
	probs, _ := check(ok, nil)
	if hasProblem(probs, "invalid status") {
		t.Errorf("withdrawn must be a valid status; got %v", probs)
	}
	bad := []*Stream{mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "superseded"})}
	probs, _ = check(bad, nil)
	if !hasProblem(probs, "invalid status", "withdrawn") {
		t.Errorf("invalid-status message must list withdrawn; got %v", probs)
	}
	if validPlaceholderStatus[statusWithdrawn] {
		t.Error("placeholder-v1 must not accept withdrawn (placeholders retire by issue close)")
	}
}

// A withdrawn brief is out of the done-ratio denominator, Next-up, the
// incomplete list and the bottleneck WIP, and is shown in its own count.
func TestWithdrawnExcludedFromViews(t *testing.T) {
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done", Verified: "2026-10-01 v", Reviewed: "2026-10-01 r"},
		Brief{Num: "02", Wave: 0, Status: "withdrawn", Title: "Old plan", SupersededBy: []string{"alpha/03"}, WithdrawnDate: "2026-10-18"},
		Brief{Num: "03", Wave: 1, Status: "todo", Title: "New plan"},
		Brief{Num: "04", Wave: 0, Status: "withdrawn", Title: "Dropped"},
	)
	streams := []*Stream{s}

	if got := liveBriefCount(s); got != 2 {
		t.Errorf("liveBriefCount = %d, want 2", got)
	}
	if got := withdrawnCount(s); got != 2 {
		t.Errorf("withdrawnCount = %d, want 2", got)
	}

	nu := nextUp(streams, ClaimView{}, nil)
	var ids []string
	for _, p := range nu.Picks {
		ids = append(ids, p.Brief.Num)
	}
	if strings.Join(ids, ",") != "03" {
		t.Errorf("Next-up picks = %v, want only 03 (a lower-wave withdrawn brief never holds the wave)", ids)
	}
	if st := briefFrontierState(streams, s, s.Briefs[1], nil, nil); st != fsDone {
		t.Errorf("frontier state of a withdrawn brief = %q, want %q", st, fsDone)
	}

	rep := computeBottleneck(streams, nil, time.Now())
	if rep.TotalBriefs != 2 {
		t.Errorf("bottleneck TotalBriefs = %d, want 2", rep.TotalBriefs)
	}
	for _, st := range rep.Stages {
		if st.Stage == "withdrawn" {
			t.Errorf("bottleneck must not carry a withdrawn stage")
		}
	}

	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	for _, want := range []string{
		"| 1/2 |",
		"2 withdrawn",
		"## Withdrawn briefs",
		"- 02 Old plan — withdrawn 2026-10-18 — superseded by alpha/03",
		"- 04 Dropped — withdrawn — no successor",
		"**1/2** briefs done · **2** withdrawn",
		"### alpha (1 open)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("STATUS.md missing %q", want)
		}
	}

	// A tree with no withdrawn brief renders no Withdrawn section or clause.
	plain := mkStream("beta", "active", "P1", Brief{Num: "01", Status: "todo"})
	out = emit([]*Stream{plain}, nil, nextUp([]*Stream{plain}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out, "withdrawn") || strings.Contains(out, "Withdrawn") {
		t.Errorf("a board with no withdrawn brief must not mention withdrawn")
	}
}

// A stream whose every brief is done or withdrawn is an archive candidate, and
// a withdrawn brief may sit under done/.
func TestWithdrawnIsTerminalForArchive(t *testing.T) {
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "01", Status: "done", Schema: "brief-v1"},
		Brief{Num: "02", Status: "withdrawn", Schema: "brief-v1"},
	)
	if n := streamArchiveReadyNotices([]*Stream{s}); len(n) == 0 {
		t.Errorf("an all-terminal stream must be an archive candidate")
	}
}

// A never-started withdrawal does not conflict with an unstarted todo witness.
func TestWithdrawnUnstartedHasNoDriftNotice(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "withdrawn"})
	if n := assertedVsDerivedNotices(s, []BriefCell{{ID: "alpha/01", Cell: "todo"}}); len(n) != 0 {
		t.Errorf("want no drift NOTICE for a withdrawn row; got %v", n)
	}
}

// A brief schema newer than this binary is refused loudly — never exempted —
// naming the needed schema, the supported set, where the first supporting
// release is listed, and the statusgen this repo pins.
func TestUnknownBriefSchemaRefusedLoudly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte("assay v9.9.9\nstatusgen v9.9.9 abc  # linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs", "streams", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTemp(t, dir, "brief-01-x.md", schemaFM("brief-v4", "01", 0, "", ""))
	bf, ok, err := parseBriefFile(p)
	if bf != nil || ok || err == nil {
		t.Fatalf("an unknown brief schema must be refused, not exempted; got (%v,%v,%v)", bf, ok, err)
	}
	for _, want := range []string{"needs schema brief-v4", "brief-v1, brief-v2, brief-v3", "§Schema versions", "pins statusgen v9.9.9"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q: %v", want, err)
		}
	}
}

// brief-v3 is a recognized schema that carries every brief-v2 rule.
func TestBriefV3IsV2Family(t *testing.T) {
	if !recognizedBriefSchemas[briefSchemaV3] || !isBriefV2Family(briefSchemaV3) || !isBriefV2Family(briefSchemaV2) || isBriefV2Family(briefSchemaCurrent) {
		t.Fatal("brief-v3 must be recognized and in the brief-v2 family")
	}
	// A brief-v3 brief with a v1-shaped id fails the v2-family id rule.
	_, problems, _ := withdrawnTree(t, []wdRow{{num: "01", status: "todo",
		file: strings.Replace(withdrawnFM("01", 0, "", ""), "brief: tc:tr:alpha:01", "brief: alpha/01", 1)}})
	if !hasProblem(problems, "brief-01", "brief-v1 <stream>/<NN> form") {
		t.Errorf("brief-v3 must enforce the brief-v2 hierarchical id; got:\n%s", strings.Join(problems, "\n"))
	}
}

// Every recognized schema has a first-release row (the machine-readable
// supported-schema declaration printed by `--version --json`).
func TestBriefSchemaFirstReleaseCoversRecognized(t *testing.T) {
	for s := range recognizedBriefSchemas {
		if _, ok := briefSchemaFirstRelease[s]; !ok {
			t.Errorf("recognized schema %s has no briefSchemaFirstRelease row", s)
		}
	}
	for s := range briefSchemaFirstRelease {
		if !recognizedBriefSchemas[s] {
			t.Errorf("briefSchemaFirstRelease names %s, which this build does not recognize", s)
		}
	}
	if _, ok := embeddedBriefSchemaNames[briefSchemaV3]; !ok {
		t.Error("conform must embed the brief-v3 contract")
	}
}

// TestBriefV3SchemaCoverage holds schemas/brief-v3.json in the same lockstep as
// brief-v2: the shared surface and every brief-v2 rule, plus the three
// withdrawn keys typed as the validator parses them.
func TestBriefV3SchemaCoverage(t *testing.T) {
	schema, err := parseSchema(embeddedSchemaBytes(embeddedBriefV3SchemaName))
	if err != nil {
		t.Fatalf("embedded brief-v3 schema does not parse: %v", err)
	}
	problems := schemaCoverageProblems(schema.raw, briefSchemaV3)
	problems = append(problems, schemaV2CoverageProblems(schema.raw)...)
	props, _ := schema.raw["properties"].(map[string]any)
	for key, want := range map[string]string{"withdrawn-date": "string", "withdrawn-reason": "string"} {
		p, _ := props[key].(map[string]any)
		if p == nil || p["type"] != want {
			problems = append(problems, "properties."+key+" must be type "+want)
		}
	}
	if p, _ := props["superseded-by"].(map[string]any); p == nil {
		problems = append(problems, "properties.superseded-by is missing")
	}
	if len(problems) > 0 {
		t.Fatalf("committed brief-v3 schema drifted from the validated contract:\n  - %s", strings.Join(problems, "\n  - "))
	}
}

// Backward compatibility of the generated board: a brief-v1/brief-v2 file with a
// prose `superseded-by:` string still parses, so its generated README row is
// rendered unchanged and no "hand edit to a generated table" PROBLEM appears.
func TestWithdrawnKeysOnV2DoNotChangeGeneratedTable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	prose := "superseded-by: \"merged into the follow-up brief\"\n"
	for num, schema := range map[string]string{"01": "brief-v2", "02": "brief-v1"} {
		if err := os.WriteFile(filepath.Join(dir, "brief-"+num+"-x.md"), []byte(schemaFM(schema, num, 0, "", prose)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	readme := "---\nstream: alpha\nstatus: active\npriority: P1\ntrack: platform\nboard: generated\n---\n\n# Alpha\n\n" +
		briefsMarkerBegin + "\n" + briefsMarkerEnd + "\n"
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: tc\nrepos:\n  tr: {cell: tc, repo: medici-finance/assay}\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if rows := streamBriefRows(streams[0]); len(rows) != 2 {
		t.Fatalf("both prose-superseded-by briefs must render a generated row; got %d rows", len(rows))
	}
	if _, err := rewriteReadmeRegion(streams[0], path); err != nil {
		t.Fatal(err)
	}
	streams, _, err = loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := checkReadmeTables(streams); len(p) > 0 {
		t.Errorf("want no generated-table PROBLEM; got:\n%s", strings.Join(p, "\n"))
	}
	if p, _ := checkBriefFiles(streams, streams); hasProblem(p, "superseded-by") || hasProblem(p, "withdrawn") {
		t.Errorf("a v1/v2 prose superseded-by must raise no PROBLEM; got:\n%s", strings.Join(p, "\n"))
	}
}

// A brief depending on two withdrawn briefs later in its own stream (checked
// AFTER it, so their README rows are not yet wired) still names each one's
// cross-repo successors, never "no successor".
func TestDependsOnWithdrawnNamesSuccessorsInFileOrder(t *testing.T) {
	_, _, notices := withdrawnTree(t, []wdRow{
		{num: "11", status: "todo", wave: 1, file: withdrawnFM("11", 1, "[\"alpha/21\", \"alpha/22\"]", "")},
		{num: "21", status: "withdrawn", file: withdrawnFM("21", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"cell:other:composability:03\"]\nwithdrawn-reason: \"x\"\n")},
		{num: "22", status: "withdrawn", file: withdrawnFM("22", 0, "",
			"withdrawn-date: \"2026-10-18\"\nsuperseded-by: [\"cell:other:graph-execution:22\", \"cell:other:graph-execution:25\"]\nwithdrawn-reason: \"x\"\n")},
	})
	for _, want := range [][]string{
		{"brief-11", "depends on alpha/21", "successor cell:other:composability:03"},
		{"brief-11", "depends on alpha/22", "successor cell:other:graph-execution:22, cell:other:graph-execution:25"},
	} {
		if !hasProblem(notices, want...) {
			t.Errorf("want NOTICE %q; got:\n%s", want, strings.Join(linesFor(notices, "depends on"), "\n"))
		}
	}
	if n := linesFor(linesFor(notices, "brief-11"), "no successor"); len(n) > 0 {
		t.Errorf("withdrawn deps with successors must not read as won't-fix: %v", n)
	}
}

// briefSchemaFirstRelease and the normative table in spec/brief-v1.md §3.5
// name the same first release for every schema. A "" row (a schema not yet in
// any release) is exempt until the release cut stamps both.
func TestBriefSchemaFirstReleaseMatchesSpecTable(t *testing.T) {
	raw, err := os.ReadFile("../spec/brief-v1.md")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	row := regexp.MustCompile("(?m)^\\| `(brief-v[0-9]+)` \\|.*\\| ([^|]*?) \\|$")
	seen := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(string(raw), -1) {
		schema, cell := m[1], strings.Fields(m[2])
		seen[schema] = true
		want, ok := briefSchemaFirstRelease[schema]
		if !ok {
			t.Errorf("spec table names %s, which briefSchemaFirstRelease lacks", schema)
			continue
		}
		if want != "" && (len(cell) == 0 || cell[0] != want) {
			t.Errorf("%s: spec first release %q, briefSchemaFirstRelease %q", schema, m[2], want)
		}
	}
	for s := range briefSchemaFirstRelease {
		if !seen[s] {
			t.Errorf("briefSchemaFirstRelease names %s, which the spec table lacks", s)
		}
	}
}
