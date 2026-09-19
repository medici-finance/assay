package main

import (
	"strings"
	"testing"
)

// queuetruth_realbrief_test.go — FAIL-FIRST coverage for the queue-truthfulness follow-up: the
// deferred (longitudinal) and awaiting-online-lane buckets must fire on the SHAPE real briefs
// carry, not only on the synthetic bare `## Verify` fixtures the earlier pass exercised.
//
// The load-bearing difference from queuetruth_part2_test.go: a real brief's Verify heading is not
// the bare `## Verify` — it carries a qualifier the author appends, e.g.
//
//	## Verify (executable — no prose-only DoD items)
//
// The content-signal derivation only runs on the `## Verify` section body, so a heading matcher
// that demands an EXACT `## Verify` line reads an empty section for every real brief and derives
// nothing — the deferred/online-lane buckets silently never fire and the items over-report as
// DISPATCH. These fixtures reproduce that exact shape (public-safe, neutral wording, no house
// stream/namespace names) and assert the correct bucket.

// realVerifyHeading is the qualifier-carrying Verify heading a real brief uses. Using it in the
// fixtures is what makes each gap test RED before the extractor is taught to match the heading as
// a prefix, and GREEN after.
const realVerifyHeading = "## Verify (executable — no prose-only DoD items)"

// fixtureBrief assembles a brief with the given frontmatter tail, Verify body, and empty Evidence.
func fixtureBrief(fmTail, verifyRows string) string {
	return "---\nbrief: x\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n" +
		fmTail + "---\n\n# Brief\n\n" +
		realVerifyHeading + "\n\n| # | Command | Expect |\n|---|---------|--------|\n" +
		verifyRows + "\n\n## Evidence\n<!-- appended at verification time -->\n"
}

const oneRowTable = "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
	"|---|-------|------|--------|--------|----------|----------|\n" +
	"| 01 | x | 0 | S | implemented | — | — |\n"

func classifyFixture(t *testing.T, brief string) (disposition, string) {
	t.Helper()
	root := t.TempDir()
	writeFixtureStream(t, root, "example-stream", oneRowTable, map[string]string{"01": brief})
	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	return classifyItem(it, tier)
}

// TestRealHeading_LongitudinalWindowStaysDispatchWhenOnlyExpectNamesIt (#1309 item 4): the
// Verify EXPECT prose mentions "the shadow window's dated capture trees", but the COMMAND is
// `git ls-files … | wc -l`, which runs offline today. Derivation reads the Command cell only,
// so this brief is DISPATCH — it was the live false positive that deferred a runnable brief on
// one row's expectation wording. (A row whose COMMAND names the window still defers: see
// TestGap1_LongitudinalWindowDefersWithoutMarker.)
func TestRealHeading_LongitudinalWindowStaysDispatchWhenOnlyExpectNamesIt(t *testing.T) {
	brief := fixtureBrief("",
		"| 1 | `git ls-files 'reports/daily/*' \\| wc -l` | >= 14 — the shadow window's dated capture trees exist on main |")
	disp, reason := classifyFixture(t, brief)
	if disp != dispDispatch {
		t.Fatalf("offline-command brief whose Expect names a window classified %v (%q); want dispatch", disp, reason)
	}
}

// TestRealHeading_ClusterKubectlBuckets: gap 2 on the real heading shape, live-cluster row. The
// Verify command is a literal `kubectl ... -n <ns> ...` against a live cluster, no `verify-lane:`
// marker, real qualifier heading. Before the fix it over-reports as DISPATCH; after, it buckets
// awaiting-online-lane. The namespace is a neutral placeholder — never a house-internal name.
func TestRealHeading_ClusterKubectlBuckets(t *testing.T) {
	brief := fixtureBrief("",
		"| 1 | `kubectl get cronjob -n example-ns some-job -o jsonpath='{.spec.schedule}'` | matches the declared schedule |")
	disp, reason := classifyFixture(t, brief)
	if disp != dispAwaitingOnlineLane {
		t.Fatalf("live-cluster kubectl row (real heading, no marker) classified %v; want awaiting-online-lane", disp)
	}
	if reason == "" {
		t.Fatalf("awaiting-online-lane member should carry a lane reason")
	}
}

// TestRealHeading_ExternalHandoffRowBuckets: gap 2 broadened-phrase coverage. A Verify row whose
// COMMAND is an offline→online hand-off ("console-external-verify hand-off to the online verify
// lane"), with NO kubectl and NO marker, must still bucket awaiting-online-lane — an offline
// verifier cannot produce the verdict for a row that names an external/online lane. The hand-off
// is named in the Command cell: since #1309 item 4 derivation reads the Command cell only, so a
// hand-off that appears only in the Expect prose no longer defers (that prose explains, it does
// not command).
func TestRealHeading_ExternalHandoffRowBuckets(t *testing.T) {
	brief := fixtureBrief("",
		"| 1 | console-external-verify — hand-off to the online verify lane: confirm the deployed config matches | matches |")
	disp, reason := classifyFixture(t, brief)
	if disp != dispAwaitingOnlineLane {
		t.Fatalf("external hand-off row (real heading, no marker) classified %v; want awaiting-online-lane", disp)
	}
	if reason == "" {
		t.Fatalf("awaiting-online-lane member should carry a lane reason")
	}
}

// TestRealHeading_ExplicitMarkerWinsOverDerived: precedence guard. A brief that carries BOTH an
// explicit `blocked-until:` marker AND a live-cluster kubectl Verify row under the real heading
// must surface the AUTHORED deferral (its exact reason), never a derived online-lane bucket —
// explicit author-set markers always win, derivation is only the fallback. Green before and after
// (the marker path is independent of the heading extractor); it guards the fix from inverting the
// precedence.
func TestRealHeading_ExplicitMarkerWinsOverDerived(t *testing.T) {
	brief := fixtureBrief("blocked-until: 2026-12-01 (authored condition)\n",
		"| 1 | `kubectl get pods -n example-ns` against the live cluster | pod Running |")
	disp, reason := classifyFixture(t, brief)
	if disp != dispDeferred {
		t.Fatalf("explicit blocked-until + cluster row classified %v; want deferred (marker wins)", disp)
	}
	if !strings.Contains(reason, "authored condition") {
		t.Fatalf("explicit marker reason should win, got %q", reason)
	}
}

// TestRealHeading_IncidentalKubectlStaysDispatch: false-positive guard. A genuinely-actionable
// brief may MENTION kubectl in its prose (Context/Task) while its Verify rows are entirely offline
// (git/grep). Because derivation reads ONLY the `## Verify` section, the incidental mention must not
// bucket it — it stays DISPATCH. This proves the derivation is scoped to the Verify rows and keeps
// the broadened phrase set from false-deferring real work.
func TestRealHeading_IncidentalKubectlStaysDispatch(t *testing.T) {
	brief := "---\nbrief: x\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n" +
		"# Brief\n\n## Context\n\nThe deploy is applied elsewhere with `kubectl apply`; this brief only edits the reference manifest in-repo.\n\n" +
		realVerifyHeading + "\n\n| # | Command | Expect |\n|---|---------|--------|\n" +
		"| 1 | `git diff --exit-code k8s/reference/some.yaml` | no drift from the committed reference |\n\n" +
		"## Evidence\n<!-- appended at verification time -->\n"
	disp, _ := classifyFixture(t, brief)
	if disp != dispDispatch {
		t.Fatalf("incidental kubectl in Context (offline Verify rows) classified %v; want dispatch", disp)
	}
}

// TestExtractVerify_MatchesQualifierHeading is the direct unit anchor for the root cause: the
// Verify-section extractor must return the body under a qualifier-carrying `## Verify (...)`
// heading, not only under a bare `## Verify`. Without it every real brief derives from an empty
// section.
func TestExtractVerify_MatchesQualifierHeading(t *testing.T) {
	content := "---\nbrief: x\n---\n\n# Brief\n\n" + realVerifyHeading +
		"\n\n| 1 | `kubectl get pods` | ok |\n\n## Evidence\n<!-- x -->\n"
	body := extractVerify(content)
	if !strings.Contains(body, "kubectl") {
		t.Fatalf("extractVerify returned %q; want the row body under the qualifier heading", body)
	}
	if strings.Contains(body, "## Evidence") {
		t.Fatalf("extractVerify leaked past the section boundary: %q", body)
	}
}
