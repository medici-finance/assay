package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTelemetryEndpointEmptyByDefault is the ship-safety guard: every shipped
// build must carry an EMPTY endpoint so a default run never dials. If someone
// bakes a receiver URL into the source (rather than stamping it at build time
// for a build that deliberately ships telemetry), this fails.
func TestTelemetryEndpointEmptyByDefault(t *testing.T) {
	if telemetryEndpoint != "" {
		t.Fatalf("telemetryEndpoint must be empty in-source (got %q) — a receiver URL must never be committed; nothing may leave a machine by default", telemetryEndpoint)
	}
	if err := sendTelemetry(TelemetryPayload{Schema: telemetrySchemaVersion}); err != errTelemetryNoEndpoint {
		t.Fatalf("with an empty endpoint sendTelemetry must be a no-op returning errTelemetryNoEndpoint, got %v", err)
	}
}

// TestTelemetryDefaultOff proves the double opt-in: telemetry is armed ONLY when
// the flag is set AND the env var is exactly "1". Every other combination is OFF.
func TestTelemetryDefaultOff(t *testing.T) {
	cases := []struct {
		name   string
		flag   bool
		env    string // "" means unset
		wantOn bool
	}{
		{"nothing set", false, "", false},
		{"flag only", true, "", false},
		{"env only", false, "1", false},
		{"env set to non-1", true, "0", false},
		{"env set to true-word", true, "true", false},
		{"both set", true, "1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.env == "" {
				os.Unsetenv(telemetryEnvVar)
			} else {
				t.Setenv(telemetryEnvVar, c.env)
			}
			if got := telemetryArmed(c.flag); got != c.wantOn {
				t.Fatalf("telemetryArmed(flag=%v, %s=%q) = %v, want %v", c.flag, telemetryEnvVar, c.env, got, c.wantOn)
			}
		})
	}
}

// TestClassifyLintProblemNeverEchoes proves the classifier returns only its own
// category constants and never any substring of the message it was handed — so a
// lint message embedding a repo/stream/brief identifier or a path cannot leak
// through a category label.
func TestClassifyLintProblemNeverEchoes(t *testing.T) {
	const secret = "ZZSECRETIDENTIFIER"
	msgs := []string{
		secret + "/brief-01: status verified requires a Verified entry",
		secret + ": invalid stream status \"weird\"",
		secret + ": done streams must be moved to docs/archive/",
		secret + ": duplicate brief number \"01\"",
		secret + ": max-concurrent 9 out of range — must be 1..4",
		secret + ": malformed Ack \"nope\"",
		secret + ": Affects references unknown stream \"" + secret + "\"",
		secret + "/brief-02: unresolved F-x (desk-acked) — demote to todo (re-gate) or resolve the finding",
		secret + ": some entirely novel problem shape nobody has seen",
	}
	valid := map[string]bool{
		lintCatFrontmatter: true, lintCatArchiveHygiene: true, lintCatConcurrencyConfig: true,
		lintCatBriefNumbering: true, lintCatBriefStatus: true, lintCatFindingAck: true,
		lintCatFindingReference: true, lintCatUnresolvedFinding: true, lintCatWordBudget: true,
		lintCatLinkCheck: true, lintCatUncategorized: true,
	}
	for _, m := range msgs {
		cat := classifyLintProblem(m)
		if !valid[cat] {
			t.Errorf("classifyLintProblem(%q) = %q, not a known category constant", m, cat)
		}
		if strings.Contains(cat, secret) {
			t.Errorf("category %q for message %q leaked the identifier", cat, m)
		}
	}
	// The novel shape must fall through to uncategorized (a count), never echo.
	if got := classifyLintProblem(msgs[len(msgs)-1]); got != lintCatUncategorized {
		t.Errorf("novel message classified as %q, want %q", got, lintCatUncategorized)
	}
}

// TestNormalizeStatusFixedVocabulary proves an unexpected status string is
// collapsed to a generic label rather than travelling verbatim.
func TestNormalizeStatusFixedVocabulary(t *testing.T) {
	if got := normalizeStatus(""); got != "none" {
		t.Errorf("normalizeStatus(\"\") = %q, want none", got)
	}
	if got := normalizeStatus("implemented"); got != "implemented" {
		t.Errorf("normalizeStatus(implemented) = %q, want implemented", got)
	}
	if got := normalizeStatus("ZZWEIRDSTATUS"); got != "other" {
		t.Errorf("normalizeStatus(ZZWEIRDSTATUS) = %q, want other — an unknown status must not travel verbatim", got)
	}
}

// TestBuildPayloadNoLeak_InMemory is the strong, isolated leak invariant: feed
// buildTelemetryPayload streams/problems/history saturated with sentinel
// identifiers and assert none survive into the marshaled JSON. Counts still land.
func TestBuildPayloadNoLeak_InMemory(t *testing.T) {
	const (
		streamName = "ZZSTREAMSENTINEL"
		briefTitle = "ZZTITLESENTINEL"
		briefID    = "ZZBRIEFSENTINEL"
		sha        = "ZZSHASENTINEL"
	)
	streams := []*Stream{{
		Name: streamName,
		Briefs: []Brief{
			{Num: "01", Title: briefTitle, Status: "verified"},
			{Num: "02", Title: briefTitle + "-two", Status: "done"},
		},
	}}
	problems := []string{
		streamName + "/brief-01: status verified requires a Verified entry",
		streamName + ": some totally novel problem containing " + briefTitle,
	}
	history := []HistoryEntry{
		{Ts: "2026-07-01T00:00:00Z", Brief: briefID, From: "implemented", To: "verified", SHA: sha},
		{Ts: "2026-07-02T00:00:00Z", Brief: briefID, From: "verified", To: "done", SHA: sha},
	}
	p := buildTelemetryPayload(streams, problems, history)

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{streamName, briefTitle, briefID, sha} {
		if bytes.Contains(raw, []byte(sentinel)) {
			t.Fatalf("payload leaked sentinel %q:\n%s", sentinel, raw)
		}
	}
	// Counts must still be populated — an all-empty payload would pass the leak
	// check vacuously.
	if p.StreamCount != 1 || p.BriefCount != 2 {
		t.Fatalf("counts wrong: streams=%d briefs=%d, want 1/2", p.StreamCount, p.BriefCount)
	}
	if p.LintFailureCategories[lintCatBriefStatus] != 1 {
		t.Errorf("expected one brief-status lint category, got %v", p.LintFailureCategories)
	}
	if p.LintFailureCategories[lintCatUncategorized] != 1 {
		t.Errorf("expected the novel problem to be uncategorized, got %v", p.LintFailureCategories)
	}
	if p.LifecycleTransitions["implemented->verified"] != 1 || p.LifecycleTransitions["verified->done"] != 1 {
		t.Errorf("transitions wrong: %v", p.LifecycleTransitions)
	}
	if p.BriefStatusCounts["verified"] != 1 || p.BriefStatusCounts["done"] != 1 {
		t.Errorf("status counts wrong: %v", p.BriefStatusCounts)
	}
}

// telemetryFixtureREADME builds a stream README whose stream name and brief
// title both carry sentinels, and whose brief status (verified with no Verified
// entry) provokes a real lint failure — so the end-to-end collect path exercises
// a non-empty problems set.
func telemetryFixtureREADME(stream, title string) string {
	return "---\nstream: " + stream + "\nstatus: active\npriority: P1\n---\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | " + title + " | 0 | M | verified | — | — |\n"
}

// TestCollectTelemetryPayload_FixtureTreeNoLeak drives the real load path
// (collectTelemetryPayload → loadStreams + check + LoadHistory) over a fixture
// tree on disk whose directory name, stream name, brief title, and history log
// all carry sentinels, and asserts none reach the marshaled payload.
func TestCollectTelemetryPayload_FixtureTreeNoLeak(t *testing.T) {
	const (
		streamName = "zzstreamsentinel"
		briefTitle = "ZZTITLESENTINEL"
		sha        = "zzshasentinel"
	)
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", streamName)
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdir, "README.md"), []byte(telemetryFixtureREADME(streamName, briefTitle)), 0o644); err != nil {
		t.Fatal(err)
	}
	// Append-only history log with a sentinel brief id + sha.
	hist := `{"ts":"2026-07-01T00:00:00Z","brief":"` + streamName + `/01","from":"implemented","to":"verified","sha":"` + sha + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(historyRelPath)), []byte(hist), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := collectTelemetryPayload(root)
	if err != nil {
		t.Fatalf("collectTelemetryPayload: %v", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{streamName, briefTitle, sha, "ZZTITLESENTINEL"} {
		if bytes.Contains(raw, []byte(sentinel)) {
			t.Fatalf("fixture-tree payload leaked sentinel %q:\n%s", sentinel, raw)
		}
	}
	if p.StreamCount != 1 || p.BriefCount != 1 {
		t.Fatalf("counts wrong: streams=%d briefs=%d, want 1/1", p.StreamCount, p.BriefCount)
	}
	if p.LifecycleTransitions["implemented->verified"] != 1 {
		t.Errorf("expected an implemented->verified transition, got %v", p.LifecycleTransitions)
	}
	if len(p.LintFailureCategories) == 0 {
		t.Errorf("expected the verified-without-entry brief to yield a lint category, got none")
	}
}

// TestPrintTelemetryPayloadDeterministic proves the printed payload is stable
// (sorted keys) and carries the schema banner.
func TestPrintTelemetryPayloadDeterministic(t *testing.T) {
	p := TelemetryPayload{
		Schema:                telemetrySchemaVersion,
		StatusgenVersion:      statusgenVersion,
		StreamCount:           1,
		BriefCount:            3,
		BriefStatusCounts:     map[string]int{"done": 2, "todo": 1},
		LintFailureCategories: map[string]int{lintCatBriefStatus: 1},
		LifecycleTransitions:  map[string]int{"todo->done": 1},
	}
	var a, b bytes.Buffer
	if err := printTelemetryPayload(&a, p); err != nil {
		t.Fatal(err)
	}
	if err := printTelemetryPayload(&b, p); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("printTelemetryPayload is not deterministic across runs")
	}
	if !strings.Contains(a.String(), telemetrySchemaVersion) {
		t.Errorf("printed payload missing schema banner:\n%s", a.String())
	}
	if !strings.Contains(a.String(), "\"stream_count\": 1") {
		t.Errorf("printed payload missing expected field:\n%s", a.String())
	}
}
