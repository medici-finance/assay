package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// exitrecord_test.go — the intake-exit-v1 record: validation, the schema-document pin, and the
// drain writing it end to end.

// isolatedDeskHome points HOME at a fresh directory with the fixture roster planted, so a test's
// record file and audit log are its own. The first cleanup registered is the last to run: it
// reloads the config once HOME is back to the package's fixture home.
func isolatedDeskHome(t *testing.T) string {
	t.Helper()
	t.Cleanup(deskkit.ReloadConfig)
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	return filepath.Join(home, ".config", "assay")
}

func validMechanical() IntakeExitRecord {
	return IntakeExitRecord{
		Schema:      intakeExitSchema,
		Source:      "issue",
		Item:        "medici-finance/assay#77",
		Repo:        "medici-finance/assay",
		Exit:        ExitPlaceholder,
		Artifact:    "scan-pr:medici-finance/assay",
		DecidedBy:   decidedMechanical,
		TriagerRole: "intake-desk",
		TriagerTier: tierNone,
		Opened:      "2026-08-24T11:50:00Z",
		Triaged:     "2026-08-24T12:00:00Z",
		Kind:        kindNewIssue,
		Trust:       string(AdmissionAdmitted),
		SessionTag:  "sess-1",
	}
}

func validJudgment() IntakeExitRecord {
	r := validMechanical()
	r.Exit = ExitBug
	r.Artifact = "medici-finance/assay#900"
	r.DecidedBy = decidedJudgment
	r.TriagerTier = tierStrong
	r.Kind = kindUpdate
	r.DispatchRef = "assay--issue-77@20261006T141502Z.3fa9c01b7d2e"
	return r
}

func TestExitRecord_Validate(t *testing.T) {
	refused := []struct {
		name string
		edit func(*IntakeExitRecord)
	}{
		{"unknown exit", func(r *IntakeExitRecord) { r.Exit = "escalated" }},
		{"unrouted is not an exit", func(r *IntakeExitRecord) { r.Exit = ExitUnrouted }},
		{"missing artifact on bug", func(r *IntakeExitRecord) { r.Exit = ExitBug; r.Artifact = "" }},
		{"free-text artifact", func(r *IntakeExitRecord) { r.Artifact = "an issue in the decision queue" }},
		{"bare repo artifact", func(r *IntakeExitRecord) { r.Artifact = "medici-finance/assay" }},
		{"bare stream artifact", func(r *IntakeExitRecord) { r.Artifact = "desk-supervision" }},
		{"vendor model as tier", func(r *IntakeExitRecord) { r.DecidedBy = decidedJudgment; r.TriagerTier = "claude-opus" }},
		{"mechanical with a tier", func(r *IntakeExitRecord) { r.TriagerTier = tierStrong }},
		{"judgment with tier none", func(r *IntakeExitRecord) { r.DecidedBy = decidedJudgment }},
		{"unknown decided_by", func(r *IntakeExitRecord) { r.DecidedBy = "model" }},
		{"login-shaped role alice-dev", func(r *IntakeExitRecord) { r.TriagerRole = "alice-dev" }},
		{"retired loop name as role", func(r *IntakeExitRecord) { r.TriagerRole = "issue-loop" }},
		{"empty role", func(r *IntakeExitRecord) { r.TriagerRole = "" }},
		{"kind outside classifier", func(r *IntakeExitRecord) { r.Kind = "looks like a bug" }},
		{"scan-batch is not a kind", func(r *IntakeExitRecord) { r.Kind = "scan-batch" }},
		{"bad dispatch_ref", func(r *IntakeExitRecord) { r.DispatchRef = "assay--issue-77" }},
		{"dispatch_ref upper hex", func(r *IntakeExitRecord) { r.DispatchRef = "assay--issue-77@20261006T141502Z.3FA9C01B7D2E" }},
		{"dispatch_ref no claim sep", func(r *IntakeExitRecord) { r.DispatchRef = "assay@20261006T141502Z.3fa9c01b7d2e" }},
		{"detail on bug", func(r *IntakeExitRecord) {
			r.Exit = ExitBug
			r.Artifact = "#12"
			r.Detail = "rejected"
		}},
		{"rejected-watching no detail", func(r *IntakeExitRecord) { r.Exit = ExitRejectedWatching; r.Artifact = "" }},
		{"opened after triaged", func(r *IntakeExitRecord) { r.Opened = "2026-08-24T12:00:01Z" }},
		{"triaged not RFC3339", func(r *IntakeExitRecord) { r.Triaged = "2026-08-24" }},
		{"wrong schema", func(r *IntakeExitRecord) { r.Schema = "intake-exit-v2" }},
		{"unknown source", func(r *IntakeExitRecord) { r.Source = "email" }},
		{"repo not the item's", func(r *IntakeExitRecord) { r.Repo = "example-org/tracker" }},
		{"bad trust state", func(r *IntakeExitRecord) { r.Trust = "shared-agent" }},
		{"intake with kind", func(r *IntakeExitRecord) {
			r.Source = "intake"
			r.Item = "I-some-idea"
			r.Trust = ""
		}},
		{"session tag with spaces", func(r *IntakeExitRecord) { r.SessionTag = "a session tag" }},
	}
	for _, tc := range refused {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			r := validMechanical()
			tc.edit(&r)
			err := r.Validate()
			if err == nil {
				t.Fatalf("accepted %+v", r)
			}
			if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
				t.Fatalf("exit code %d, want %d (refused): %v", code, deskkit.ExitRefused, err)
			}
		})
	}

	t.Run("refusal does not echo a long value", func(t *testing.T) {
		r := validMechanical()
		r.TriagerRole = strings.Repeat("x", 200)
		err := r.Validate()
		if err == nil || strings.Contains(err.Error(), strings.Repeat("x", 40)) {
			t.Fatalf("the refusal echoed the pasted value back: %v", err)
		}
	})

	for _, role := range intakeExitRoles {
		t.Run("accepted/role "+role, func(t *testing.T) {
			r := validJudgment()
			r.TriagerRole = role
			if err := r.Validate(); err != nil {
				t.Fatalf("closed-set member %q refused: %v", role, err)
			}
		})
	}
	accepted := map[string]IntakeExitRecord{
		"valid mechanical": validMechanical(),
		"valid judgment":   validJudgment(),
	}
	rw := validJudgment()
	rw.Exit, rw.Artifact, rw.Detail = ExitRejectedWatching, "", "watching"
	accepted["rejected-watching without artifact"] = rw
	noOpened := validMechanical()
	noOpened.Opened = ""
	accepted["opened unknown"] = noOpened
	intake := validJudgment()
	intake.Source, intake.Item, intake.Kind, intake.Trust, intake.DispatchRef = "intake", "I-some-idea", "", "", ""
	intake.TriagerRole = "driver"
	intake.Exit, intake.Artifact = ExitFinding, "F-desk-emits-briefs"
	accepted["intake record"] = intake
	for _, ref := range []string{"desk-supervision/33", "example-org/tracker#12", "#12", "F-desk-emits-briefs", "scan-pr:medici-finance/assay"} {
		r := validJudgment()
		r.Artifact = ref
		accepted["artifact "+ref] = r
	}
	for name, r := range accepted {
		t.Run("accepted/"+name, func(t *testing.T) {
			if err := r.Validate(); err != nil {
				t.Fatalf("refused a valid record: %v", err)
			}
		})
	}
}

// TestRecordArtifact_TypesTheScanPR — the lane's free-text "new draft scan PR" artifact becomes the
// typed scan-pr ref; anything else passes through for Validate to judge.
func TestRecordArtifact_TypesTheScanPR(t *testing.T) {
	cases := map[string]string{
		"medici-finance/assay (new draft scan PR)": "scan-pr:medici-finance/assay",
		"medici-finance/assay#12":                  "medici-finance/assay#12",
		"an issue in the decision queue":           "an issue in the decision queue",
	}
	for in, want := range cases {
		if got := recordArtifact(in); got != want {
			t.Errorf("recordArtifact(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestClassifierKinds_AreTheRecordKinds — every reason classify can return is a kind the record
// accepts, so a new reason cannot silently fail every drain that meets it.
func TestClassifierKinds_AreTheRecordKinds(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir()}
	a := Admission{Item: inbound("medici-finance/assay", 5), State: AdmissionAdmitted}
	seen := map[string]bool{}
	for _, target := range []string{"", "someone-else/elsewhere", "medici-finance/assay"} {
		s.ScanTarget = target
		_, kind := s.classify(a)
		seen[kind] = true
	}
	for k := range seen {
		if !memberOf(k, classifierKinds) {
			t.Errorf("classify returned %q, which the record refuses", k)
		}
	}
}

// schemaDocPath is the schema document, relative to this package directory.
const schemaDocPath = "../../../../docs/streams/desk-supervision/intake-exit-v1.md"

// docTableFirstColumn returns the backticked first-column values of the table under the "## "
// heading named section.
func docTableFirstColumn(t *testing.T, doc, section string) []string {
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

func jsonKeys(v any) []string {
	var out []string
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	return out
}

// TestIntakeExitSchema_MatchesDoc pins this writer to the schema document: its JSON keys (set AND
// order) and its closed role set. statusgen carries the twin of this test, so the two writers
// cannot drift apart without one of them going red.
func TestIntakeExitSchema_MatchesDoc(t *testing.T) {
	raw, err := os.ReadFile(schemaDocPath)
	if err != nil {
		t.Fatalf("reading the schema document: %v", err)
	}
	doc := string(raw)

	docKeys := docTableFirstColumn(t, doc, "Fields")
	if got := jsonKeys(IntakeExitRecord{}); !reflect.DeepEqual(got, docKeys) {
		t.Errorf("record JSON keys differ from the schema document\n record: %v\n    doc: %v", got, docKeys)
	}

	docRoles := docTableFirstColumn(t, doc, "Closed role set")
	a := append([]string(nil), intakeExitRoles...)
	b := append([]string(nil), docRoles...)
	sort.Strings(a)
	sort.Strings(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("closed role set differs from the schema document\n writer: %v\n    doc: %v", a, b)
	}
	// The five desk loop names in the set are exactly the canonical loop registry's names.
	var loops []string
	for _, r := range intakeExitRoles {
		if r != "driver" {
			loops = append(loops, r)
		}
	}
	known := deskkit.KnownLoopNames()
	for _, l := range loops {
		if c, ok := deskkit.CanonicalLoopName(l); !ok || c != l {
			t.Errorf("role %q is not a canonical loop name (registry: %v)", l, known)
		}
	}
}

func readRecordLines(t *testing.T, stateDir string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(stateDir, intakeExitsFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			out = append(out, sc.Text())
		}
	}
	return out
}

func decodeRecord(t *testing.T, line string) (IntakeExitRecord, map[string]any) {
	t.Helper()
	var r IntakeExitRecord
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		t.Fatalf("record line does not parse: %v\n%s", err, line)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatal(err)
	}
	return r, m
}

// TestDrainPass_WritesExitRecords is the flow: an offline pass over a fixture inbound file (one new
// issue, one update) writes one mechanical record per batch member, then `land` on the parked
// update writes the judgment record, and no line carries a title, an author or a body.
func TestDrainPass_WritesExitRecords(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	t.Setenv("DESK_LOOP", "intake-desk")

	g := newFakeGit()
	loop := batchLoop(t,
		"INBOUND: medici-finance/assay#170 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#171 2026-08-24T11:51:00Z\n", g)
	const author = "shared-agent" // a trusted fixture login: it must never reach a record
	loop.Probe = probeReturning(author, time.Time{}, nil, true, nil)
	dir := filepath.Join(loop.Root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-171.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s", err, sb.String())
	}
	if got := loop.Parked(); len(got) != 1 || got[0] != "medici-finance/assay#171" {
		t.Fatalf("parked = %v, want the update parked for judgment", got)
	}

	lines := readRecordLines(t, stateDir)
	if len(lines) != 1 {
		t.Fatalf("after the pass the record file has %d lines, want 1 (one per batch member):\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	mech, _ := decodeRecord(t, lines[0])
	if mech.Item != "medici-finance/assay#170" || mech.DecidedBy != decidedMechanical ||
		mech.TriagerTier != tierNone || mech.Exit != ExitPlaceholder || mech.Kind != kindNewIssue ||
		mech.Artifact != "scan-pr:medici-finance/assay" || mech.Trust != string(AdmissionAdmitted) ||
		mech.TriagerRole != LoopName || mech.Opened != "2026-08-24T11:50:00Z" {
		t.Fatalf("mechanical record = %+v", mech)
	}

	var out strings.Builder
	err := cmdLand([]string{"--item", "medici-finance/assay#171", "--exit", "needs-decision",
		"--artifact", "medici-finance/assay#901", "--tier", "strong", "--kind", "update"}, &out)
	if err != nil {
		t.Fatalf("land on the parked update: %v", err)
	}
	lines = readRecordLines(t, stateDir)
	if len(lines) != 2 {
		t.Fatalf("after land the record file has %d lines, want 2", len(lines))
	}
	judged, _ := decodeRecord(t, lines[1])
	if judged.Item != "medici-finance/assay#171" || judged.DecidedBy != decidedJudgment ||
		judged.TriagerTier != tierStrong || judged.Exit != ExitNeedsDecision || judged.TriagerRole != "intake-desk" {
		t.Fatalf("judgment record = %+v", judged)
	}

	wantKeys := jsonKeys(IntakeExitRecord{})
	for i, line := range lines {
		_, m := decodeRecord(t, line)
		for _, banned := range []string{"title", "author", "body"} {
			if _, ok := m[banned]; ok {
				t.Errorf("line %d carries the key %q: %s", i+1, banned, line)
			}
		}
		if len(m) != len(wantKeys) {
			t.Errorf("line %d has %d keys, want exactly the schema's %d: %s", i+1, len(m), len(wantKeys), line)
		}
		if strings.Contains(line, author) {
			t.Errorf("line %d carries the author login: %s", i+1, line)
		}
	}
}

// TestLand_DryRunWritesNoRecord — a dry-run pass prints the ledger and writes nothing.
func TestLand_DryRunWritesNoRecord(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	loop := passLoop(t, "INBOUND: medici-finance/assay#172 2026-08-24T11:50:00Z\n")
	if err := drainPass(passConfig(t), loop, &strings.Builder{}); err != nil {
		t.Fatalf("pass errored: %v", err)
	}
	if len(loop.Ledger().Records()) != 1 {
		t.Fatalf("ledger = %v", loop.Ledger().Records())
	}
	if lines := readRecordLines(t, stateDir); len(lines) != 0 {
		t.Fatalf("a dry-run wrote %d record line(s)", len(lines))
	}
}
