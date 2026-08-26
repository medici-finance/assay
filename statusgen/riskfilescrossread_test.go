package main

import (
	"strings"
	"testing"
)

// briefWith builds a minimal valid brief-v1 file body around a risk block and a
// `## Context` section, for the cross-read tests. riskAllNo controls the four
// answers; ctx is the raw `## Context` body (its `files:` line, if any).
func crossReadBrief(id, gate string, riskAllNo bool, ctx string) string {
	risk := "{regulatory: no, customer: no, irreversible: no, sensitive-data: no}"
	if !riskAllNo {
		risk = "{regulatory: no, customer: yes, irreversible: no, sensitive-data: no}"
	}
	return "---\n" +
		"brief: " + id + "\n" +
		"title: cross-read fixture " + id + "\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: " + gate + "\n" +
		"risk: " + risk + "\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-08-26 fixture\n" +
		"sources: [\"fixture\"]\n" +
		"---\n\n# Brief\n\n## Context\n\n" + ctx + "\n"
}

// runCrossRead writes the given brief files into a temp stream dir and runs the
// check against a stream at the given repo. Returns the notices (the check lands
// advisory, so hits and could-not-checks are notices while riskFilesCrossReadFatal
// is false).
func runCrossRead(t *testing.T, repo string, briefs map[string]string) (problems, notices []string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range briefs {
		writeTemp(t, dir, name, content)
	}
	s := &Stream{Name: "crossread", Dir: dir, Repo: repo}
	return riskFilesCrossRead([]*Stream{s})
}

func crossReadNoticeHas(lines []string, subs ...string) bool {
	for _, l := range lines {
		ok := true
		for _, sub := range subs {
			if !strings.Contains(l, sub) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// TestCrossReadFiresOnDowngradedGate is the positive control (Verify row 8): a
// brief that declares a triggering path with four "no" answers makes the check
// fire, and its honest-"yes" inverse does not.
func TestCrossReadFiresOnDowngradedGate(t *testing.T) {
	// A base-list trigger (.github/workflows/) so the case is repo-independent, plus
	// a per-repo (medici-finance/assay) gate-code path. Both with all-risk-no.
	downgraded := crossReadBrief("crossread/01", "model", true,
		"files:\n- `.github/workflows/deploy.yml` — the deploy workflow this brief edits.")
	// The INVERSE: the same triggering path, but an honest "yes" answer forces
	// gate:human, so the derivation is not downgraded and the check must stay quiet.
	honest := crossReadBrief("crossread/02", "human", false,
		"files:\n- `.github/workflows/deploy.yml` — the deploy workflow this brief edits.")

	_, notices := runCrossRead(t, "medici-finance/assay", map[string]string{
		"brief-01-downgraded.md": downgraded,
		"brief-02-honest.md":     honest,
	})

	if !crossReadNoticeHas(notices, "[risk-files-crossread]", "brief crossread/01", ".github/workflows/deploy.yml") {
		t.Errorf("the check did not fire on the downgraded brief (triggering path + all-no). notices:\n%s",
			strings.Join(notices, "\n"))
	}
	// The honest-yes inverse must NOT be reported.
	if crossReadNoticeHas(notices, "brief crossread/02") {
		t.Errorf("the check fired on the honest-yes brief — a single risk 'yes' forces gate:human and is not a downgrade. notices:\n%s",
			strings.Join(notices, "\n"))
	}
	// The message must say it is about the INPUTS, not the derivation (reviewer Q).
	if !crossReadNoticeHas(notices, "INPUTS to the gate derivation") {
		t.Error("the finding does not state it is about the gate INPUTS, not the derivation")
	}
}

// TestCrossReadFiresOnPerRepoTrigger proves the per-repo topology triggers reach
// the check, not just the base list — and that they are keyed to the stream's repo.
func TestCrossReadFiresOnPerRepoTrigger(t *testing.T) {
	brief := crossReadBrief("crossread/01", "model", true,
		"files:\n- `tools/desk/cmd/writeguard/guard.go` — the write guard.")

	// On the toolkit repo the path is a per-repo trigger → fires.
	_, onAssay := runCrossRead(t, "medici-finance/assay", map[string]string{"brief-01-x.md": brief})
	if !crossReadNoticeHas(onAssay, "[risk-files-crossread]", "tools/desk/cmd/writeguard/guard.go") {
		t.Errorf("the per-repo trigger did not fire on medici-finance/assay. notices:\n%s", strings.Join(onAssay, "\n"))
	}

	// On an unrelated repo the same path matches nothing (base list only) → quiet.
	// This proves the trigger set is keyed to the stream's repo, not global.
	_, onOther := runCrossRead(t, "example-org/tracker", map[string]string{"brief-01-x.md": brief})
	if crossReadNoticeHas(onOther, "tools/desk/cmd/writeguard/guard.go") {
		t.Errorf("a toolkit-only trigger fired on an unrelated repo — the per-repo set is not repo-keyed. notices:\n%s",
			strings.Join(onOther, "\n"))
	}
}

// TestCrossReadCouldNotCheck is Verify row 9: an absent or unparseable
// declared-paths line reports could-not-check, never a pass.
func TestCrossReadCouldNotCheck(t *testing.T) {
	// No files: line at all, only prose.
	noLine := crossReadBrief("crossread/01", "model", true,
		"single-point-of-failure: this brief declares no files line.")

	_, notices := runCrossRead(t, "medici-finance/assay", map[string]string{"brief-01-nofiles.md": noLine})
	if !crossReadNoticeHas(notices, "[risk-files-crossread]", "brief crossread/01", "COULD-NOT-CHECK") {
		t.Errorf("a brief with no declared-paths line did not report could-not-check. notices:\n%s",
			strings.Join(notices, "\n"))
	}
	// It must NOT be silently absent (rounded up to a clean pass): the only line
	// about this brief is the could-not-check, never nothing.
	sawBrief := false
	for _, l := range notices {
		if strings.Contains(l, "brief crossread/01") {
			sawBrief = true
		}
	}
	if !sawBrief {
		t.Error("could-not-check was rounded up to a pass — the brief produced no finding at all")
	}
}

// TestRiskFilesCrossReadOrdinaryBriefQuiet proves the check does not class an
// ordinary brief (reviewer Q1): a brief that declares only non-trigger paths, all
// risk "no", produces no finding.
func TestRiskFilesCrossReadOrdinaryBriefQuiet(t *testing.T) {
	ordinary := crossReadBrief("crossread/01", "model", true,
		"files:\n- `docs/notes.md` — a plain doc.\n- `statusgen/model.go` — an ordinary source file.")

	_, notices := runCrossRead(t, "medici-finance/assay", map[string]string{"brief-01-ordinary.md": ordinary})
	if crossReadNoticeHas(notices, "brief crossread/01") {
		t.Errorf("the check classed an ordinary brief (only non-trigger paths). notices:\n%s",
			strings.Join(notices, "\n"))
	}
}

// TestExtractContextDeclaredPaths pins the declared-paths parser across the two
// authored forms and the could-not-check states.
func TestExtractContextDeclaredPaths(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantPaths []string
		wantFound bool
	}{
		{
			name: "bulleted with prose, continuation and read-only annotation",
			body: "## Context\n\nfiles:\n" +
				"- `statusgen/` (implementation home) — the lint tree.\n" +
				"- `tools/desk/internal/deskkit/riskclassifier.go` and\n" +
				"  `tools/desk/internal/deskkit/riskpath.go` — **read for reference only; not edited.**\n" +
				"- `topology.yaml` — the trigger declarations. Read, not edited.\n\n" +
				"facts:\n- something else\n",
			wantPaths: []string{"statusgen/", "tools/desk/internal/deskkit/riskclassifier.go", "tools/desk/internal/deskkit/riskpath.go", "topology.yaml"},
			wantFound: true,
		},
		{
			name:      "inline comma-separated form with backticks and a (new) marker",
			body:      "## Context\n\nfiles: `tools/desk/internal/deskkit/decide.go` (new) + `docs/streams/x/decide.md`\n\nsingle-point-of-failure: ...",
			wantPaths: []string{"tools/desk/internal/deskkit/decide.go", "docs/streams/x/decide.md"},
			wantFound: true,
		},
		{
			name:      "no files line — could-not-check",
			body:      "## Context\n\nsingle-point-of-failure: nothing here names files.\n",
			wantPaths: nil,
			wantFound: false,
		},
		{
			name:      "no Context section — could-not-check",
			body:      "# Brief\n\nSome prose but no Context heading.\n",
			wantPaths: nil,
			wantFound: false,
		},
		{
			name:      "backticked non-path token is not mistaken for a path",
			body:      "## Context\n\nfiles:\n- `RiskPathTriggersFor` is a function, `topology.yaml` is a path.\n",
			wantPaths: []string{"topology.yaml"},
			wantFound: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, found := extractContextDeclaredPaths(c.body)
			if found != c.wantFound {
				t.Fatalf("found = %t, want %t (paths %v)", found, c.wantFound, got)
			}
			if !equalStringSlice(got, c.wantPaths) {
				t.Errorf("paths = %v, want %v", got, c.wantPaths)
			}
		})
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
