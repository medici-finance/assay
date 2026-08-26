package main

// clusterrow_test.go — PROOF THESE CHECKS CAN FAIL.
//
// Per the fleet's positive-control rule (docs/mistake-proofing.md §3 D1: a
// control that has never been seen red is either unnecessary or broken, and
// without an injected failure you cannot tell which), the guards and derivations
// exercised here ship with a re-runnable mutation spec, clusterrow-mutations.json,
// consumed by the desk mutation harness (tools/desk/cmd/muhar):
//
//	cd statusgen && muhar -spec clusterrow-mutations.json
//
// The captured red run (baseline GREEN, control CAUGHT, 4 mutations CAUGHT / 0
// survivors) shows each assertion observed red before it was trusted green:
//   - CONTROL  probe resolver first-vs-last  → TestCluster_ProbeExtraction red
//     (clusterProbe("run a.sh then b.sh") = "a.sh", want "b.sh").
//   - lint short-circuited to nil            → TestCluster_LintProbeThreeState +
//     TestCluster_LintNoRegistryIsProblem red (got [], want one PROBLEM).
//   - runWitnesses cluster branch disabled   → TestCluster_VerifyrunRecordsCouldNotCheckMarker
//     red (state "fail", want "could-not-run"; marker absent from the table).
//   - queue drops the VERIFY:FAIL exclusion / the all-parked gate →
//     TestCluster_PendingQueueDerivation red (a FAIL / an unparked brief queues).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// All tests here share the `Cluster` substring so a single `go test -run Cluster`
// (or the brief's `-run 'RowClass|Cluster'`) exercises the whole cluster-row
// surface: resolver, probe extraction, marker, registry, lint and queue.

// briefV1WithVerifyEvidence writes a minimal, structurally-valid brief-v1 file
// with the given `## Verify` and `## Evidence` bodies, and returns its path.
func briefV1WithVerifyEvidence(t *testing.T, dir, num, verifyBody, evidenceBody string) string {
	t.Helper()
	fm := "---\n" +
		"brief: t/" + num + "\n" +
		"title: fixture\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"authored: 2026-08-26 by test\n" +
		"sources: [\"fixture\"]\n" +
		"schema: brief-v1\n" +
		"---\n\n# Fixture\n\n## Verify\n" + verifyBody +
		"\n\n## Evidence\n" + evidenceBody + "\n\n## Review\nGate: model.\n"
	path := filepath.Join(dir, "brief-"+num+".md")
	if err := os.WriteFile(path, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writePodProbes writes the documented-probe registry under root.
func writePodProbes(t *testing.T, root string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(podProbesRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCluster_ResolveIsKnownFifthClass(t *testing.T) {
	// The resolver recognises check:cluster as a KNOWN class (three-state:
	// recognised cluster / recognised non-cluster / unknown).
	cases := []struct {
		in    string
		want  string
		known bool
	}{
		{"check:cluster", classCheckCluster, true},   // recognised cluster row
		{"`check:cluster`", classCheckCluster, true}, // code span stripped
		{"CHECK:CLUSTER", classCheckCluster, true},   // case-folded
		{"check", classCheck, true},                  // recognised, not cluster
		{"check:clstr", "check:clstr", false},        // could-not-resolve — a typo, unknown
	}
	for _, c := range cases {
		got, known := resolveRowClass(c.in)
		if got != c.want || known != c.known {
			t.Errorf("resolveRowClass(%q) = (%q,%v), want (%q,%v)", c.in, got, known, c.want, c.known)
		}
	}
	if !knownRowClasses[classCheckCluster] {
		t.Error("classCheckCluster missing from knownRowClasses")
	}
}

func TestCluster_TableColumnResolvesClusterRow(t *testing.T) {
	section := strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check | `true` | exit 0 |",
		"| 2 | check:cluster | probe-participant.sh | ok |",
	}, "\n")
	var got []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 2 {
		t.Fatalf("parsed %d rows, want 2", len(got))
	}
	if got[1].class() != classCheckCluster {
		t.Errorf("row 2 class()=%q, want %q", got[1].class(), classCheckCluster)
	}
}

func TestCluster_ProbeExtraction(t *testing.T) {
	cases := map[string]string{
		"probe-participant.sh":                   "probe-participant.sh", // bare
		"kubectl exec pod -- probe-validator.sh": "probe-validator.sh",   // inside an invocation → last token
		"scripts/pod/check-widget.sh":            "check-widget.sh",      // path → basename
		"run a.sh then b.sh":                     "b.sh",                 // LAST *.sh wins
		"no probe here":                          "",                     // none
		"prefix probe-x.sh --flag":               "probe-x.sh",           // trailing args
	}
	for in, want := range cases {
		if got := clusterProbe(in); got != want {
			t.Errorf("clusterProbe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCluster_MarkerRoundTrip(t *testing.T) {
	m := clusterPendingMarker("probe-participant.sh")
	if !strings.Contains(m, "pod-runner-pending") || !strings.Contains(m, "probe=probe-participant.sh") {
		t.Fatalf("marker %q missing stable, greppable fields", m)
	}
	// Distinct from an ordinary env-bound skip.
	if m == verifyEnvBoundNote {
		t.Error("cluster marker must be distinct from the env-bound skip note")
	}
	evidence := "some prose\n" + m + "\nand a second: " + clusterPendingMarker("probe-validator.sh")
	got := clusterMarkerProbes(evidence)
	if !got["probe-participant.sh"] || !got["probe-validator.sh"] || len(got) != 2 {
		t.Errorf("clusterMarkerProbes = %v, want the two parked probes", got)
	}
	if len(clusterMarkerProbes("no marker here")) != 0 {
		t.Error("clusterMarkerProbes on marker-free evidence must be empty")
	}
}

func TestCluster_LoadPodProbes(t *testing.T) {
	root := t.TempDir()
	// Absent registry → exists=false, no error.
	if set, exists, err := loadPodProbes(root); err != nil || exists || set != nil {
		t.Fatalf("absent registry: got (%v,%v,%v), want (nil,false,nil)", set, exists, err)
	}
	writePodProbes(t, root,
		"# documented probes",
		"probe-participant.sh",
		"",
		"  check-widget.sh  ", // whitespace trimmed
	)
	set, exists, err := loadPodProbes(root)
	if err != nil || !exists {
		t.Fatalf("present registry: exists=%v err=%v", exists, err)
	}
	if !set["probe-participant.sh"] || !set["check-widget.sh"] || len(set) != 2 {
		t.Errorf("registry set = %v, want the two documented probes", set)
	}
}

func TestCluster_LintProbeThreeState(t *testing.T) {
	// A stream with a documented-probe registry naming one probe.
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "t")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePodProbes(t, root, "probe-participant.sh")
	s := &Stream{Name: "t", Dir: streamDir, Root: root}

	clusterRow := func(command string) string {
		return strings.Join([]string{
			"| # | Class | Command | Expect |",
			"|---|-------|---------|--------|",
			"| 1 | check:cluster | " + command + " | ok |",
		}, "\n")
	}

	// (i) a documented probe → clean.
	briefV1WithVerifyEvidence(t, streamDir, "01", clusterRow("probe-participant.sh"), "")
	if probs := clusterRowProblems([]*Stream{s}); len(probs) != 0 {
		t.Fatalf("documented probe: got %v, want none", probs)
	}

	// (ii) an UNDOCUMENTED probe → PROBLEM naming it.
	os.Remove(filepath.Join(streamDir, "brief-01.md"))
	briefV1WithVerifyEvidence(t, streamDir, "01", clusterRow("probe-ghost.sh"), "")
	probs := clusterRowProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], "probe-ghost.sh") {
		t.Fatalf("undocumented probe: got %v, want one PROBLEM naming probe-ghost.sh", probs)
	}

	// (iii) a cluster row naming NO probe → PROBLEM.
	os.Remove(filepath.Join(streamDir, "brief-01.md"))
	briefV1WithVerifyEvidence(t, streamDir, "01", clusterRow("nothing to run here"), "")
	probs = clusterRowProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], "names no probe") {
		t.Fatalf("no-probe row: got %v, want one names-no-probe PROBLEM", probs)
	}
}

func TestCluster_LintNoRegistryIsProblem(t *testing.T) {
	// A cluster row present but NO documented-probe registry → PROBLEM (can't
	// validate against a list that does not exist).
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "t")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	briefV1WithVerifyEvidence(t, streamDir, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:cluster | probe-participant.sh | ok |",
	}, "\n"), "")
	s := &Stream{Name: "t", Dir: streamDir, Root: root}
	probs := clusterRowProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], podProbesRelPath) {
		t.Fatalf("no registry: got %v, want one PROBLEM naming the registry path", probs)
	}
}

func TestCluster_LintInertWithoutClusterRows(t *testing.T) {
	// A board with no cluster rows and no registry stays clean — the whole lint
	// is inert where the pod/online lane does not exist.
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "t")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	briefV1WithVerifyEvidence(t, streamDir, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci | `true` | exit 0 |",
	}, "\n"), "")
	s := &Stream{Name: "t", Dir: streamDir, Root: root}
	if probs := clusterRowProblems([]*Stream{s}); len(probs) != 0 {
		t.Errorf("no cluster rows: got %v, want none (inert)", probs)
	}
}

func TestCluster_VerifyrunRecordsCouldNotCheckMarker(t *testing.T) {
	// The OFFLINE lane records a cluster row could-not-run with the greppable
	// marker — never a silent skip, never a false pass.
	rows := []verifyRow{
		{ID: "1", Command: "true", Expect: "exit 0", Class: classCheck, Classed: false},                                     // ordinary check → runs
		{ID: "2", Command: "kubectl exec p -- probe-participant.sh", Expect: "ok", Class: classCheckCluster, Classed: true}, // cluster → could-not-run
	}
	ws := runWitnesses(t.TempDir(), rows, "test", "0000", "2026-08-26", 30*time.Second, false)
	if ws[1].State != stateCouldNotRun {
		t.Errorf("cluster row: state %q, want %q", ws[1].State, stateCouldNotRun)
	}
	if ws[1].Note != clusterPendingMarker("probe-participant.sh") {
		t.Errorf("cluster row note = %q, want the cluster-pending marker", ws[1].Note)
	}
	if ws[0].State == stateCouldNotRun {
		t.Error("ordinary check row must not be treated as a cluster row")
	}
	// The marker lands in the witness table (could-not-run is recorded, unlike a
	// skip) so it is greppable in Evidence.
	if !strings.Contains(witnessTable(ws), "pod-runner-pending") {
		t.Error("cluster marker missing from the witness table — it must be greppable in Evidence")
	}
}

func TestCluster_PendingQueueDerivation(t *testing.T) {
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "t")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePodProbes(t, root, "probe-participant.sh")

	verify := strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci | `true` | exit 0 |",
		"| 2 | check:cluster | probe-participant.sh | ok |",
	}, "\n")
	parked := clusterPendingMarker("probe-participant.sh")

	// POSITIVE: implemented, non-cluster PASS, cluster row parked → in the queue.
	briefV1WithVerifyEvidence(t, streamDir, "01", verify, "VERIFY: PASS row 1\n"+parked)
	// NEGATIVE (a): a non-cluster FAIL → rework, NOT the pod's — excluded.
	briefV1WithVerifyEvidence(t, streamDir, "02", verify, "VERIFY: FAIL row 1\n"+parked)
	// NEGATIVE (b): cluster row NOT parked (no marker) → not derivable as pending.
	briefV1WithVerifyEvidence(t, streamDir, "03", verify, "VERIFY: PASS row 1")
	// NEGATIVE (c): a brief with no cluster row at all → excluded.
	briefV1WithVerifyEvidence(t, streamDir, "04", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci | `true` | exit 0 |",
	}, "\n"), "VERIFY: PASS row 1")

	s := &Stream{Name: "t", Dir: streamDir, Root: root, Briefs: []Brief{
		{Num: "01", Status: "implemented"},
		{Num: "02", Status: "implemented"},
		{Num: "03", Status: "implemented"},
		{Num: "04", Status: "implemented"},
	}}

	q := clusterPendingQueue([]*Stream{s})
	if len(q) != 1 {
		t.Fatalf("queue = %+v, want exactly brief 01", q)
	}
	if q[0].Brief != "t/01" {
		t.Errorf("queue brief = %q, want t/01", q[0].Brief)
	}
	if len(q[0].Probes) != 1 || q[0].Probes[0] != "probe-participant.sh" {
		t.Errorf("queue probes = %v, want [probe-participant.sh]", q[0].Probes)
	}

	// NEGATIVE (d): a VERIFIED brief is already past the parked state → excluded.
	s.Briefs[0].Status = "verified"
	if q := clusterPendingQueue([]*Stream{s}); len(q) != 0 {
		t.Errorf("verified brief still queued: %+v", q)
	}
}
