package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/statusgen/contexthandoff"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWithdrawalExecutionDriftRemainsVisible(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "withdrawn"})
	for _, c := range []BriefCell{{ID: "alpha/01", Cell: "implemented", Witness: "merged PR"}, {ID: "tc:tr:alpha:01", Cell: "verified", Witness: "independent evidence"}, {ID: "alpha/01", Cell: "unknown", MergedPR: "PR #7"}, {ID: "alpha/01", Cell: "in-progress", Witness: "open PR"}} {
		if got := assertedVsDerivedNotices(s, []BriefCell{c}); len(got) != 1 {
			t.Errorf("withdrawal hid execution witness %+v: %v", c, got)
		}
	}
}
func TestWithdrawalSuccessorCyclesAndUnknownCoverage(t *testing.T) {
	fm := func(n, refs string) string {
		return withdrawnFM(n, 0, "", "withdrawn-date: 2026-10-18\nwithdrawn-reason: replanned\nsuperseded-by: ["+refs+"]\n")
	}
	_, p, n := withdrawnTree(t, []wdRow{{num: "01", status: "withdrawn", file: fm("01", "alpha/02")}, {num: "02", status: "withdrawn", file: fm("02", "tc:tr:alpha:01")}, {num: "03", status: "withdrawn", file: fm("03", "remote:other:beta:01")}})
	if !hasProblem(p, "superseded-by", "cycle") {
		t.Errorf("cycle not detected: %v", p)
	}
	if !hasProblem(n, "remote:other:beta:01", "could-not-check") {
		t.Errorf("unconfigured coverage silently accepted: %v", n)
	}
}
func TestWithdrawalConsumerAndArchiveReaders(t *testing.T) {
	streams, _, _ := withdrawnTree(t, []wdRow{{num: "01", status: "withdrawn", file: withdrawnFM("01", 0, "", "withdrawn-date: 2026-10-18\nwithdrawn-reason: retired\nsuperseded-by: []\n") + "\nChanges a shared value across all consumers.\n"}})
	s := streams[0]
	_, n := consumersCheck(s.Root, streams)
	if hasProblem(n, "enumerates no consumers") {
		t.Errorf("closed history needs no new consumer list: %v", n)
	}
	s.Briefs[0].Status = "todo"
	_, liveNotices := consumersCheck(s.Root, streams)
	if !hasProblem(liveNotices, "enumerates no consumers") {
		t.Fatalf("consumer test did not exercise the open-work prompt: %v", liveNotices)
	}
	s.Briefs[0].Status = "withdrawn"
	if err := os.Mkdir(filepath.Join(s.Dir, "done"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(s.Dir, "brief-01-x.md"), filepath.Join(s.Dir, "done", "brief-01-x.md")); err != nil {
		t.Fatal(err)
	}
	p, _ := checkArchivedPlaceholders(streams)
	if len(p) != 0 {
		t.Fatalf("withdrawn history must be archivable: %v", p)
	}
	s.Briefs[0].Status = "todo"
	p, _ = checkArchivedPlaceholders(streams)
	if !hasProblem(p, "only `status: done`") {
		t.Fatalf("live work became archivable: %v", p)
	}
}
func TestWithdrawalReportDenominators(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "done", Wave: 2}, Brief{Num: "02", Status: "withdrawn", Wave: 1})
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	streams := []*Stream{s}
	if got := computeBottleneck(streams, nil, now); got.TotalBriefs != 1 {
		t.Errorf("bottleneck includes retired work: %+v", got)
	}
	if got := nextWave(s); got != 0 {
		t.Errorf("withdrawal creates a next wave: %d", got)
	}
	row := roadmapStreamRow{Stream: s, StageCounts: stageCounts(s), HealthColor: "green"}
	portfolio := renderRoadmap(streams, []roadmapStreamRow{row}, nil, nil, false, "", NextUp{}, LadderReport{}, nil, "abcdef1", now)
	if !strings.Contains(portfolio, "1 total briefs across 1 streams") {
		t.Error("portfolio denominator includes withdrawal")
	}
	page := renderStreamPage(row, nil, nil, nil, nil, nil, "abcdef1", now, now)
	if !strings.Contains(page, "1 done / 1 total briefs") {
		t.Error("stream denominator includes withdrawal")
	}
	if got := normalizeStatus("withdrawn"); got != "withdrawn" {
		t.Errorf("telemetry loses withdrawal: %q", got)
	}
}
func TestWithdrawalLegacyWaveAndTypedDependencyDiffer(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Wave: 0, Status: "withdrawn"}, Brief{Num: "02", Wave: 1, Status: "todo"})
	if got := briefFrontierState([]*Stream{s}, s, s.Briefs[1], nil, nil); got != fsReady {
		t.Errorf("legacy wave held by retired work: %q", got)
	}
	s.Briefs[1].Schema = "brief-v3"
	s.Briefs[1].Depends = []string{"alpha/01"}
	if got := briefFrontierState([]*Stream{s}, s, s.Briefs[1], nil, nil); got != fsBlockedItem {
		t.Errorf("typed dependency silently satisfied by retirement: %q", got)
	}
}

func TestWithdrawalContextProvenance(t *testing.T) {
	const record = `{"schema":"context-handoff-v2","declared_by":"tc:tr:alpha:99","target":{"state":"approved","revision":"abcdef1","approval":{"revision":"abcdef1","reference":"https://example.invalid/approval"},"intent":"Retire obsolete scope","contracts":[],"transition":null,"acceptance_source":"spec/example.md"},"discovery":{"sources":[],"queries":[],"dependency_slice":[],"coverage":{"state":"complete","enumerated":[],"unavailable":[]}},"impacts":[{"subject":"tc:tr:alpha:01","lifecycle":"todo","evidence":null,"relationship":{"basis":"declared","edge":"replaces","locator":"spec/example.md"},"disposition":"retire","reason":"No longer needed"}],"amendments":[{"id":"retire-old","subject":"tc:tr:alpha:01","action":"retire","changes":[{"aspect":"scope","before":"old scope","after":"retired"}],"replaced_by":[],"candidate":null}],"history":[],"active_work":[],"readiness":{"unresolved":[],"deciding_role":"human","next_act":"publish","prerequisites":[],"facts":{"source_available":true,"proposal_approved":true,"deployment_qualified":false}}}`
	for _, tc := range []struct {
		name, change, want string
		edit               func(*BriefFile)
	}{
		{name: "valid shared provenance"},
		{name: "digest drift", want: "digest does not match", edit: func(b *BriefFile) { b.Withdrawal.Context.Digest = "sha256:" + strings.Repeat("0", 64) }},
		{name: "successor drift", want: "successors do not match", edit: func(b *BriefFile) { b.Withdrawal.SupersededBy = []string{"alpha/02"} }},
		{name: "history required", want: "preserved-history", edit: func(b *BriefFile) { b.Evidence = "A partial implementation run." }},
		{name: "wrong subject", change: "subject", want: "no retire/replace/split"},
		{name: "unacknowledged active claim", change: "claim", want: "not acknowledged"},
		{name: "missing file", want: "withdrawal-context", edit: func(b *BriefFile) { b.Withdrawal.Context.Path = "missing.json" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(record)
			if tc.change == "subject" {
				raw = []byte(strings.ReplaceAll(string(raw), "tc:tr:alpha:01", "tc:tr:alpha:02"))
			}
			if tc.change == "claim" {
				raw = []byte(strings.Replace(string(raw), `"active_work":[]`, `"active_work":[{"subject":"tc:tr:alpha:01","claim":"claim:alpha-01","brief_revision":"abcdef1","observed_at":"2026-10-18T00:00:00Z","owner":"worker","checkpoint":"partial work retained","treatment":"cancel","acknowledged":false}]`, 1))
			}
			h, err := contexthandoff.DecodeHandoff(raw)
			if err != nil {
				t.Fatal(err)
			}
			streams, _, _ := withdrawnTree(t, []wdRow{{num: "01", status: "withdrawn", file: withdrawnFM("01", 0, "", "withdrawn-date: 2026-10-18\nwithdrawn-reason: retired\nsuperseded-by: []\n")}})
			s := streams[0]
			if err := os.WriteFile(filepath.Join(s.Root, "handoff.json"), raw, 0644); err != nil {
				t.Fatal(err)
			}
			bf := &BriefFile{Brief: "tc:tr:alpha:01", Withdrawal: &Withdrawal{Context: &WithdrawalContext{Path: "handoff.json", Digest: h.Digest()}}}
			if tc.edit != nil {
				tc.edit(bf)
			}
			var p []string
			checkWithdrawalContext(func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }, "brief-01", bf, s, newWithdrawalResolver(streams))
			if tc.want == "" && len(p) != 0 {
				t.Fatalf("valid provenance refused: %v", p)
			}
			if tc.want != "" && !hasProblem(p, tc.want) {
				t.Fatalf("want %s, got %v", tc.want, p)
			}
		})
	}
}

func TestWithdrawalContextParsingAndRootContainment(t *testing.T) {
	for _, bad := range []string{"true", "{path: ../outside.json, digest: bad}", "{path: handoff.json, digest: 'sha256:" + strings.Repeat("0", 64) + "', extra: false}"} {
		_, p, _ := withdrawnTree(t, []wdRow{{num: "01", status: "withdrawn", file: withdrawnFM("01", 0, "", "withdrawn-date: 2026-10-18\nwithdrawn-reason: retired\nwithdrawal-context: "+bad+"\n")}})
		if !hasProblem(p, "withdrawal-context") {
			t.Errorf("bad context accepted: %s: %v", bad, p)
		}
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "handoff.json")); err != nil {
		t.Skip(err)
	}
	if _, err := readWithdrawalContext(root, "handoff.json"); err == nil {
		t.Fatal("context followed symlink outside repo")
	}
}

func TestWithdrawalAuditIncludesOpenEvidenceAndImplementationPRs(t *testing.T) {
	s := mkStream("alpha", "active", "P1", Brief{Num: "01", Status: "withdrawn", SupersededBy: []string{"alpha/02"}}, Brief{Num: "02", Status: "todo"})
	var prs []withdrawalAuditPR
	if err := json.Unmarshal([]byte(`[{"number":7,"headRefName":"feat/alpha-01","headRefOid":"abc","files":[]},{"number":8,"headRefOid":"def","files":[{"path":"docs/streams/alpha/evidence/01.md"}]},{"number":9,"headRefOid":"ghi","files":[{"path":"docs/streams/alpha/README.md"}]},{"number":10,"headRefOid":"jkl","files":[{"path":"docs/streams/alpha/brief-02-new.md"}]}]`), &prs); err != nil {
		t.Fatal(err)
	}
	report := auditWithdrawals([]*Stream{s}, prs)
	if len(report.Affected) != 3 {
		t.Fatalf("lost impacted PRs or selected live work: %+v", report)
	}
	for _, r := range report.Affected {
		if r.Brief != "alpha/01" || len(r.Successors) != 1 || !strings.Contains(r.Treatment, "never auto-close") || r.Head == "" {
			t.Errorf("lost treatment/provenance: %+v", r)
		}
	}
	if !strings.Contains(report.Coverage, "could-not-check") {
		t.Fatal("snapshot coverage falsely complete")
	}
}

func TestWithdrawalPinLookupStaysInOwningRoot(t *testing.T) {
	parent := t.TempDir()
	mustWrite(t, filepath.Join(parent, ".assay-versions"), "statusgen v99.0.0 sha\n")
	root := filepath.Join(parent, "child")
	brief := filepath.Join(root, "docs", "streams", "alpha", "brief-01-x.md")
	mustWrite(t, brief, "brief")
	if got := pinnedStatusgenFor(brief); got != "" {
		t.Fatalf("borrowed parent pin: %s", got)
	}
	mustWrite(t, filepath.Join(root, ".assay-versions"), "statusgen v1.2.3 sha\n")
	if got := pinnedStatusgenFor(brief); got != "v1.2.3" {
		t.Fatalf("lost own pin: %s", got)
	}
}

func TestWithdrawalHumanGateNotBypassed(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "withdrawn"))})
	if p := dhLayerOne(t, root); !hasProblem(p, "moves it to withdrawn", "no recorded ruling") {
		t.Fatalf("withdrawal bypassed human decision hold: %v", p)
	}
	if code, out := dhGate(t, root); code == 0 {
		t.Fatalf("withdrawal bypassed corroborated human decision hold: %s", out)
	}
}

func TestWithdrawalRecordedHumanRulingStillWorks(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+", as written."))
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "withdrawn")), dhBriefPath: dhFM("gate: human", dhRuledLines())})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("recorded ruling refused: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("recorded ruling refused: %s", out)
	}
}
