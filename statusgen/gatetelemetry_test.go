package main

import (
	"strings"
	"testing"
)

// gtWindow writes a one-window data directory and returns its path. Every file
// is optional — omitting one exercises that surface's could-not-check path.
func gtWindow(t *testing.T, verdicts, findings, gates, audit string) string {
	t.Helper()
	root := t.TempDir()
	if verdicts != "" {
		writeTemp(t, root, "pr-verdicts.json", verdicts)
	}
	if findings != "" {
		writeTemp(t, root, "defect-findings.json", findings)
	}
	if gates != "" {
		writeTemp(t, root, "gates.json", gates)
	}
	if audit != "" {
		writeTemp(t, root, "audit.jsonl", audit)
	}
	return root
}

// gtDeskLine renders one line in the real deskkit audit-entry schema — the only
// shape any producer emits.
func gtDeskLine(tool, verb string, pr int, result string) string {
	return `{"ts":"2026-08-01T00:00:00Z","tool":"` + tool + `","verb":"` + verb +
		`","argsDigest":"","repo":"r","pr":` + itoa(pr) + `,"headSHA":null,"result":"` + result +
		`","detail":"","sourceSHA":"u","builtAt":"u","sessionTag":"unknown"}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

const (
	gtAuditSourcedGates = `[{"class":"deskpost-refusal","mutationTested":false,"auditSourced":true,"fires":[]}]`
	gtOneApprovedMerged = `[{"number":201,"appVerdict":"APPROVED","outcome":"merged"}]`
)

// --- the four fixtures pinned by brief-01's Verify table -------------------

// TestGateTelemetryOverrideOne pins Verify row 1: override-rate (a) computes
// 1/2 naming PR #101 (App-approved, then human-rejected); exit 0.
func TestGateTelemetryOverrideOne(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = runGateTelemetry("testdata/gatetelemetry/override-one")
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "override-rate") {
		t.Errorf("missing override-rate token:\n%s", out)
	}
	if !strings.Contains(out, "1/2") {
		t.Errorf("missing numerator 1 (of 2 approved):\n%s", out)
	}
	if !strings.Contains(out, "#101") {
		t.Errorf("missing PR #101:\n%s", out)
	}
}

// TestGateTelemetryZeroFireUntested pins Verify row 2: a positive control — 0
// fires, no mutation-test marker — must alarm ceremonial-or-untested.
func TestGateTelemetryZeroFireUntested(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = runGateTelemetry("testdata/gatetelemetry/zero-fire-untested")
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("missing ceremonial-or-untested for a 0-fire, non-mutation-tested gate:\n%s", out)
	}
}

// TestGateTelemetryZeroFireTested pins Verify row 3: a mutation-tested 0-fire
// gate must print proven-able-to-fire and must NEVER be flagged
// ceremonial-or-untested — "legitimately zero" is not an alarm.
func TestGateTelemetryZeroFireTested(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = runGateTelemetry("testdata/gatetelemetry/zero-fire-tested")
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0\noutput:\n%s", code, out)
	}
	if strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("mutation-tested zero-fire gate must NOT be flagged ceremonial-or-untested:\n%s", out)
	}
	if !strings.Contains(out, "proven-able-to-fire") {
		t.Errorf("missing proven-able-to-fire for a mutation-tested 0-fire gate:\n%s", out)
	}
}

// TestGateTelemetryMissingAudit pins Verify row 4: audit.jsonl absent — the
// audit-sourced metrics (override-rate (c), the auditSourced gate class) must
// report could-not-check, never 0, and the process must exit 3.
func TestGateTelemetryMissingAudit(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = runGateTelemetry("testdata/gatetelemetry/missing-audit")
	})
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d (gtExitCouldNotCheck)\noutput:\n%s", code, gtExitCouldNotCheck, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("missing could-not-check for the audit-sourced metrics:\n%s", out)
	}
	if strings.Contains(out, "human-gate-flip-reversal: override-rate 0") {
		t.Errorf("audit-sourced override-rate (c) rendered as a 0 instead of could-not-check:\n%s", out)
	}
	if strings.Contains(out, "gate-class deskpost-refusal: fires=0") {
		t.Errorf("auditSourced gate class rendered a fire count instead of could-not-check:\n%s", out)
	}
}

// TestGateTelemetryDeterministic pins Verify row 6: two runs over the same
// fixture must be byte-identical — no wall-clock or map-iteration leakage.
func TestGateTelemetryDeterministic(t *testing.T) {
	a := captureStdout(t, func() { runGateTelemetry("testdata/gatetelemetry/override-one") })
	b := captureStdout(t, func() { runGateTelemetry("testdata/gatetelemetry/override-one") })
	if a != b {
		t.Errorf("non-deterministic output:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", a, b)
	}
}

// --- an empty source must not read as a clean zero ---------------------------

// TestGateTelemetryEmptyAuditLogIsCouldNotCheck pins review finding 1. An
// append-only log that exists with zero lines is a collection failure (log
// rotation, a collector that created the file then died, a truncating write),
// not evidence of a quiet window.
//
// Regression direction: before the fix this printed
// "override-rate 0/0" and "gate-class deskpost-refusal: fires=0 ...
// ceremonial-or-untested" and exited 0 — the instrument affirmatively accusing
// a live gate of being ceremonial, with a clean exit so nobody would look.
func TestGateTelemetryEmptyAuditLogIsCouldNotCheck(t *testing.T) {
	root := gtWindow(t, gtOneApprovedMerged, "[]", gtAuditSourcedGates, "\n")

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d — an empty audit log is could-not-check\noutput:\n%s",
			code, gtExitCouldNotCheck, out)
	}
	if strings.Contains(out, "human-gate-flip-reversal: override-rate") {
		t.Errorf("empty audit log rendered a measured override rate:\n%s", out)
	}
	if strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("empty audit log made a gate read as ceremonial — the exact accusation finding 1 names:\n%s", out)
	}
	if !strings.Contains(out, "present but empty") {
		t.Errorf("could-not-check reason does not distinguish empty from absent:\n%s", out)
	}
}

// TestGateTelemetryEmptyAndAbsentAuditGiveDistinctReasons pins the other half
// of finding 1: the two shapes are both could-not-check, but a reader must be
// able to tell a deleted file from a truncated one.
func TestGateTelemetryEmptyAndAbsentAuditGiveDistinctReasons(t *testing.T) {
	empty := captureStdout(t, func() {
		runGateTelemetry(gtWindow(t, gtOneApprovedMerged, "[]", gtAuditSourcedGates, "\n"))
	})
	absent := captureStdout(t, func() {
		runGateTelemetry(gtWindow(t, gtOneApprovedMerged, "[]", gtAuditSourcedGates, ""))
	})
	if !strings.Contains(absent, "audit.jsonl missing") {
		t.Errorf("absent audit log does not say so:\n%s", absent)
	}
	if strings.Contains(empty, "audit.jsonl missing") {
		t.Errorf("empty audit log reported as missing — the two shapes must be distinguishable:\n%s", empty)
	}
}

// --- the reader must track the real producer ---------------------------------

// TestGateTelemetryReadsRealDeskkitAuditSchema pins that the audit reader
// consumes the schema the deskkit audit entry actually writes.
//
// Regression direction: the reader previously expected an invented
// {"event","gate","blockedDefect"} shape that no producer has ever emitted.
// Fed a deskpost audit log in the producer's actual schema, carrying genuine
// deskpost refusals, it reported "fires=0 ... ceremonial-or-untested" and
// exited 0.
func TestGateTelemetryReadsRealDeskkitAuditSchema(t *testing.T) {
	audit := strings.Join([]string{
		gtDeskLine("deskpost", "comment", 401, "refused"),
		gtDeskLine("deskpost", "comment", 402, "refused"),
		gtDeskLine("deskboard", "nextup", 0, "ok"),
	}, "\n") + "\n"
	root := gtWindow(t, gtOneApprovedMerged, "[]", gtAuditSourcedGates, audit)

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "gate-class deskpost-refusal: fires=2") {
		t.Errorf("real deskkit refusal records not counted as gate fires:\n%s", out)
	}
	if strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("a gate that demonstrably fired twice was flagged ceremonial:\n%s", out)
	}
}

// TestGateTelemetryUnrecognizedAuditSchemaIsCouldNotCheck pins the general
// form of finding 8: producer/reader schema drift must surface as
// could-not-check, never as an absence of events. The payload below is the
// tool's OWN previous invented schema — the concrete drift that shipped.
func TestGateTelemetryUnrecognizedAuditSchemaIsCouldNotCheck(t *testing.T) {
	audit := `{"event":"gate-fire","gate":"deskpost-refusal","pr":1,"blockedDefect":true}` + "\n" +
		`{"event":"ready-flip-reversal","pr":2,"blockedDefect":false}` + "\n"
	root := gtWindow(t, gtOneApprovedMerged, "[]", gtAuditSourcedGates, audit)

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d for an unrecognized audit schema\noutput:\n%s",
			code, gtExitCouldNotCheck, out)
	}
	if !strings.Contains(out, "schema mismatch") {
		t.Errorf("unrecognized audit schema not named as such:\n%s", out)
	}
	if strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("schema drift made a gate read as ceremonial:\n%s", out)
	}
}

// TestGateTelemetryUnrecognizedVerdictRowsAreCouldNotCheck applies the same
// rule to the JSON surfaces: rows present but none in a recognized shape is
// drift, not a window with no PRs. A well-formed `[]` stays a measured zero.
func TestGateTelemetryUnrecognizedVerdictRowsAreCouldNotCheck(t *testing.T) {
	root := gtWindow(t, `[{"id":"pr-1","verdict":"clean"}]`, "[]", gtAuditSourcedGates,
		gtDeskLine("deskpost", "ready", 1, "ok")+"\n")

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d for unrecognized verdict rows\noutput:\n%s",
			code, gtExitCouldNotCheck, out)
	}
	if !strings.Contains(out, "0 recognized") {
		t.Errorf("unrecognized verdict rows not named as schema drift:\n%s", out)
	}
	if strings.Contains(out, "app-approved-then-human-reversed: override-rate") {
		t.Errorf("unrecognized rows produced a measured override rate:\n%s", out)
	}
}

// TestGateTelemetryUnknownAuditGateClassIsCouldNotCheck pins the selector
// guard: an audit-sourced class with no selector matches nothing, and matching
// nothing must never render as "never fired" — that is the ceremonial alarm.
func TestGateTelemetryUnknownAuditGateClassIsCouldNotCheck(t *testing.T) {
	gates := `[{"class":"gate-nobody-wired","mutationTested":false,"auditSourced":true,"fires":[]}]`
	root := gtWindow(t, gtOneApprovedMerged, "[]", gates,
		gtDeskLine("deskpost", "ready", 1, "ok")+"\n")

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d for an unselectable audit gate class\noutput:\n%s",
			code, gtExitCouldNotCheck, out)
	}
	if strings.Contains(out, "ceremonial-or-untested") {
		t.Errorf("a gate the tool never looked for was accused of being ceremonial:\n%s", out)
	}
}

// --- (b)'s denominator: approved-and-merged only -----------------------------

// TestGateTelemetryOverrideBCountsApprovedMergesOnly pins finding 3. The (b)
// denominator is App-APPROVED merges, matching the family definition and the
// numerator's own wording — not every merge.
//
// Regression direction: three merged PRs (one APPROVED, one CHANGES_REQUESTED,
// one with no verdict) and one defect finding naming the APPROVED one used to
// print 1/3, diluting the rate 3x — always in the direction of making the App
// gate look better than it is.
func TestGateTelemetryOverrideBCountsApprovedMergesOnly(t *testing.T) {
	verdicts := `[{"number":301,"appVerdict":"APPROVED","outcome":"merged"},
		{"number":302,"appVerdict":"CHANGES_REQUESTED","outcome":"merged"},
		{"number":303,"appVerdict":"","outcome":"merged"}]`
	root := gtWindow(t, verdicts, `[{"pr":301}]`, `[]`,
		gtDeskLine("deskpost", "ready", 301, "ok")+"\n")

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "merged-PR-named-by-defect-finding: override-rate 1/1") {
		t.Errorf("(b) denominator is not scoped to APPROVED merges:\n%s", out)
	}
	if strings.Contains(out, "merged-PR-named-by-defect-finding: override-rate 1/3") {
		t.Errorf("(b) still counts non-APPROVED merges in its denominator:\n%s", out)
	}
}

// TestGateTelemetryOverrideBReportsOutOfWindowFindings pins the second half of
// finding 3: a finding naming a PR absent from pr-verdicts.json used to be
// dropped from the numerator with no output at all, so an incomplete verdict
// list shrank the numerator silently.
func TestGateTelemetryOverrideBReportsOutOfWindowFindings(t *testing.T) {
	root := gtWindow(t, gtOneApprovedMerged, `[{"pr":999}]`, `[]`,
		gtDeskLine("deskpost", "ready", 201, "ok")+"\n")

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "named PRs outside this window") {
		t.Errorf("a defect finding naming an out-of-window PR vanished silently:\n%s", out)
	}
}

// --- (c)'s denominator and numerator -----------------------------------------

// TestGateTelemetryFlipReversalIsLogShapeIndependent pins finding 4. One real
// event — a ready-flip that was later reversed — must produce the same rate
// whether the reversal is appended alongside the original flip or logged in
// its place.
//
// Regression direction: the two shapes used to print 1/2 and 1/1 for the same
// underlying world, because the denominator counted log rows instead of
// distinct flipped PRs.
func TestGateTelemetryFlipReversalIsLogShapeIndependent(t *testing.T) {
	appended := gtDeskLine("deskpost", "ready", 301, "ok") + "\n" +
		gtDeskLine("deskpost", gtVerbReadyReversal, 301, "ok") + "\n"
	replaced := gtDeskLine("deskpost", gtVerbReadyReversal, 301, "ok") + "\n"

	a := captureStdout(t, func() {
		runGateTelemetry(gtWindow(t, gtOneApprovedMerged, "[]", `[]`, appended))
	})
	b := captureStdout(t, func() {
		runGateTelemetry(gtWindow(t, gtOneApprovedMerged, "[]", `[]`, replaced))
	})
	const want = "human-gate-flip-reversal: override-rate 1/1"
	if !strings.Contains(a, want) {
		t.Errorf("append-only log shape: want %q\n%s", want, a)
	}
	if !strings.Contains(b, want) {
		t.Errorf("reversal-only log shape: want %q\n%s", want, b)
	}
}

// TestGateTelemetryFlipReversalUnobservableWithoutProducer pins the other half
// of finding 4: no desk tool emits a ready-flip reversal record, so "zero
// reversals seen" cannot be distinguished from "reversals are invisible on this
// surface". Reporting 0 would be the tool asserting a clean trust metric it
// never measured.
func TestGateTelemetryFlipReversalUnobservableWithoutProducer(t *testing.T) {
	audit := gtDeskLine("deskpost", "ready", 201, "ok") + "\n"
	root := gtWindow(t, gtOneApprovedMerged, "[]", `[]`, audit)

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d when the reversal channel is unobservable\noutput:\n%s",
			code, gtExitCouldNotCheck, out)
	}
	if strings.Contains(out, "human-gate-flip-reversal: override-rate 0/") {
		t.Errorf("an unobservable reversal numerator rendered as a measured zero:\n%s", out)
	}
	if !strings.Contains(out, "numerator unobservable") {
		t.Errorf("could-not-check reason does not name the missing producer:\n%s", out)
	}
}

// --- operator-assertion, dual-ceremony, and small-N markers ------------------

// TestGateTelemetryProvenAbleToFireDeclaresOperatorAssertion pins finding 5:
// mutationTested is read straight out of gates.json with nothing corroborating
// it, so the line that lets a gate escape the ceremonial alarm must say where
// the claim came from. A reader must not take proven-able-to-fire as proof.
func TestGateTelemetryProvenAbleToFireDeclaresOperatorAssertion(t *testing.T) {
	gates := `[{"class":"app-review","mutationTested":true,"auditSourced":false,"fires":[]}]`
	root := gtWindow(t, "[]", "[]", gates,
		gtDeskLine("deskpost", gtVerbReadyReversal, 1, "ok")+"\n")

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "proven-able-to-fire") {
		t.Fatalf("missing proven-able-to-fire:\n%s", out)
	}
	if !strings.Contains(out, "operator-asserted") {
		t.Errorf("proven-able-to-fire does not disclose that it rests on an uncorroborated flag:\n%s", out)
	}
}

// TestGateTelemetryFiresWithoutCatching pins finding 6: a gate that fires
// repeatedly and blocks nothing is the second, more expensive shape of ceremony
// — it spends the fleet's attention every time — and was previously invisible
// because detection only looked at fires == 0.
func TestGateTelemetryFiresWithoutCatching(t *testing.T) {
	gates := `[{"class":"app-review","mutationTested":true,"auditSourced":false,"fires":[
		{"pr":1,"blockedDefect":false},{"pr":2,"blockedDefect":false},
		{"pr":3,"blockedDefect":false},{"pr":4,"blockedDefect":false}]}]`
	root := gtWindow(t, "[]", "[]", gates,
		gtDeskLine("deskpost", gtVerbReadyReversal, 1, "ok")+"\n")

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "fires-without-catching") {
		t.Errorf("a gate that fired 4 times and blocked nothing was not flagged:\n%s", out)
	}
}

// TestGateTelemetryDenominatorMarkers pins finding 7: the report prints raw
// fractions, never percentages, but any consumer that divides them
// reintroduces the "0% over zero samples" failure. A zero or small denominator
// says so on the line itself.
func TestGateTelemetryDenominatorMarkers(t *testing.T) {
	root := gtWindow(t, "[]", "[]", `[]`,
		gtDeskLine("deskpost", gtVerbReadyReversal, 1, "ok")+"\n")

	out := captureStdout(t, func() { runGateTelemetry(root) })
	if !strings.Contains(out, "n=0 (no data") {
		t.Errorf("a 0/0 line is not marked as no-data:\n%s", out)
	}
	if !strings.Contains(out, "small-n") {
		t.Errorf("a small denominator is not marked:\n%s", out)
	}
}

// TestGateTelemetryNoTrailingWhitespace pins the review's cosmetic finding: the
// doc sells byte-identical diff output, and trailing whitespace is the first
// thing an editor or lint hook silently rewrites.
func TestGateTelemetryNoTrailingWhitespace(t *testing.T) {
	for _, fixture := range []string{
		"testdata/gatetelemetry/override-one",
		"testdata/gatetelemetry/zero-fire-tested",
		"testdata/gatetelemetry/zero-fire-untested",
		"testdata/gatetelemetry/missing-audit",
	} {
		out := captureStdout(t, func() { runGateTelemetry(fixture) })
		for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if line != strings.TrimRight(line, " \t") {
				t.Errorf("%s line %d has trailing whitespace: %q", fixture, i+1, line)
			}
		}
	}
}

// --- unchanged coverage ----------------------------------------------------

// TestGtPRVerdictReversed exercises the override sub-metric (a) predicate
// directly: only an APPROVED verdict paired with a human reversal outcome
// counts; a non-APPROVED verdict or a clean merge never does.
func TestGtPRVerdictReversed(t *testing.T) {
	cases := []struct {
		v    gtPRVerdict
		want bool
	}{
		{gtPRVerdict{AppVerdict: "APPROVED", Outcome: "human-rejected"}, true},
		{gtPRVerdict{AppVerdict: "APPROVED", Outcome: "reworked"}, true},
		{gtPRVerdict{AppVerdict: "APPROVED", Outcome: "closed-unmerged"}, true},
		{gtPRVerdict{AppVerdict: "APPROVED", Outcome: "merged"}, false},
		{gtPRVerdict{AppVerdict: "CHANGES_REQUESTED", Outcome: "human-rejected"}, false},
	}
	for _, c := range cases {
		if got := c.v.reversed(); got != c.want {
			t.Errorf("reversed(%+v) = %v, want %v", c.v, got, c.want)
		}
	}
}

// TestGateTelemetryMissingGatesFile confirms robustness beyond the four pinned
// fixtures: an absent gates.json also renders could-not-check rather than a
// silent empty gate-class list, and still exits 3.
func TestGateTelemetryMissingGatesFile(t *testing.T) {
	root := gtWindow(t, "[]", "[]", "",
		gtDeskLine("deskpost", gtVerbReadyReversal, 1, "ok")+"\n")

	var code int
	out := captureStdout(t, func() { code = runGateTelemetry(root) })
	if code != gtExitCouldNotCheck {
		t.Fatalf("exit = %d, want %d\noutput:\n%s", code, gtExitCouldNotCheck, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("missing could-not-check for absent gates.json:\n%s", out)
	}
}

// TestGateTelemetryMalformedJSON confirms a source that exists but fails to
// parse is a hard checked-failed error (exit 1), distinct from both a clean
// run and could-not-check.
func TestGateTelemetryMalformedJSON(t *testing.T) {
	root := gtWindow(t, "{not valid json", "", "", "")

	code := runGateTelemetry(root)
	if code != gtExitCheckedFailed {
		t.Fatalf("exit = %d, want %d (gtExitCheckedFailed)", code, gtExitCheckedFailed)
	}
}

// TestGateTelemetryMalformedAuditLine confirms the same for the audit log: a
// present-but-unparseable line is checked-failed, not could-not-check.
func TestGateTelemetryMalformedAuditLine(t *testing.T) {
	root := gtWindow(t, "[]", "[]", `[]`, "{not valid json\n")

	code := runGateTelemetry(root)
	if code != gtExitCheckedFailed {
		t.Fatalf("exit = %d, want %d (gtExitCheckedFailed)", code, gtExitCheckedFailed)
	}
}
