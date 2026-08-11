package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qualStreams copies the point-quality fixture tree into a temp root and
// loads its streams.
func qualStreams(t *testing.T) []*Stream {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/quality")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

func findBrief(streams []*Stream, num string) (*Stream, *Brief) {
	for _, s := range streams {
		for i := range s.Briefs {
			if s.Briefs[i].Num == num {
				return s, &s.Briefs[i]
			}
		}
	}
	return nil, nil
}

func TestPointQualityBackedDoneRendersPlain(t *testing.T) {
	s, br := findBrief(qualStreams(t), "01")
	if backed, reasons := rowIsBacked(s, br); !backed {
		t.Errorf("filled Evidence + dated/attributed Verified and Reviewed cells should be backed; reasons: %v", reasons)
	}
	if got := qualityToken(s, br); got != "done" {
		t.Errorf("qualityToken = %q, want %q", got, "done")
	}
}

func TestPointQualityEmptyEvidenceIsUnbacked(t *testing.T) {
	s, br := findBrief(qualStreams(t), "02")
	if backed, _ := rowIsBacked(s, br); backed {
		t.Error("a done row with empty Evidence should be unbacked")
	}
	if got := qualityToken(s, br); got != "done*" {
		t.Errorf("qualityToken = %q, want %q", got, "done*")
	}
}

func TestPointQualityBareVerifiedCellIsUnbacked(t *testing.T) {
	s, br := findBrief(qualStreams(t), "03")
	if backed, _ := rowIsBacked(s, br); backed {
		t.Error("a non-dated Verified cell should be unbacked even with filled Evidence")
	}
}

func TestPointQualityLegacyGrandfatheredIsUnbacked(t *testing.T) {
	// This is the exact gap I-08 targets: a legacy no-frontmatter done row
	// is exempt from the hard attribution/evidence checks (brief-16,
	// brief-02) but must still be visibly flagged here.
	s, br := findBrief(qualStreams(t), "04")
	if backed, _ := rowIsBacked(s, br); backed {
		t.Error("a legacy grandfathered done row (no Evidence, bare cells) should be unbacked")
	}
	if got := qualityToken(s, br); got != "done*" {
		t.Errorf("qualityToken = %q, want %q", got, "done*")
	}
}

func TestPointQualityVerifiedRowNeedsNoReviewedCell(t *testing.T) {
	s, br := findBrief(qualStreams(t), "05")
	if backed, reasons := rowIsBacked(s, br); !backed {
		t.Errorf("a verified row with filled Evidence and a dated Verified cell should be backed even with no Reviewed cell yet; reasons: %v", reasons)
	}
	if got := qualityToken(s, br); got != "verified" {
		t.Errorf("qualityToken = %q, want %q", got, "verified")
	}
}

func TestPointQualityVerifiedEmptyEvidenceIsUnbacked(t *testing.T) {
	s, br := findBrief(qualStreams(t), "06")
	if backed, _ := rowIsBacked(s, br); backed {
		t.Error("a verified row with empty Evidence should be unbacked")
	}
	if got := qualityToken(s, br); got != "verified*" {
		t.Errorf("qualityToken = %q, want %q", got, "verified*")
	}
}

func TestPointQualityOutOfScopeStatusNeverFlagged(t *testing.T) {
	s, br := findBrief(qualStreams(t), "07")
	if backed, _ := rowIsBacked(s, br); !backed {
		t.Error("a todo row is out of scope for point quality and must never be flagged unbacked")
	}
	if got := qualityToken(s, br); got != "todo" {
		t.Errorf("qualityToken = %q, want unchanged %q", got, "todo")
	}
}

func TestPointQualityInlineOutOfScopeAlwaysBacked(t *testing.T) {
	s := mkStream("x", "active", "P1", Brief{Num: "01", Wave: 0, Status: "in-progress"})
	if backed, _ := rowIsBacked(s, &s.Briefs[0]); !backed {
		t.Error("an in-progress row is out of scope and must never be flagged unbacked")
	}
}

// TestPointQualityImplementerOnlyEvidenceIsUnbacked exercises the "attributed
// per brief-16" fold: filled Evidence is not enough — an Evidence table whose
// only runner is the implementer has no independent backing, so the row is
// unbacked even though it renders identically to a real done today. This is the
// legacy blind spot (no brief-v1 opt-in → attributionProblems is exempt) the
// brief exists to surface.
func TestPointQualityImplementerOnlyEvidenceIsUnbacked(t *testing.T) {
	s, br := findBrief(qualStreams(t), "08")
	backed, reasons := rowIsBacked(s, br)
	if backed {
		t.Error("a done row whose Evidence has only an implementer-attributed runner row should be unbacked")
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "independent") {
			found = true
		}
	}
	if !found {
		t.Errorf("want an 'independent' Evidence reason; got %v", reasons)
	}
	if got := qualityToken(s, br); got != "done*" {
		t.Errorf("qualityToken = %q, want %q", got, "done*")
	}
}

// TestPointQualityBareHumanReviewerIsBacked guards the reviewedIsAttributed
// fold: a done row reviewed by a bare "human:<name>" token (no leading date)
// must count as backed — checkBriefFiles' human-gate accepts exactly this
// (hasHumanReviewer), so point quality must not contradict it by rendering
// `done*`. The pre-fold code required a dated Reviewed cell and would have
// wrongly flagged this row.
func TestPointQualityBareHumanReviewerIsBacked(t *testing.T) {
	s, br := findBrief(qualStreams(t), "09")
	if backed, reasons := rowIsBacked(s, br); !backed {
		t.Errorf("a done row reviewed by a bare human:<name> token should be backed; reasons: %v", reasons)
	}
	if got := qualityToken(s, br); got != "done" {
		t.Errorf("qualityToken = %q, want %q", got, "done")
	}
}

// TestReviewedIsAttributedAcceptsDatedOrHumanTag covers boundary cases of
// the OR-condition on top of brief-09's integration-level coverage: a
// Reviewed cell is attributed either by the same dated-runner shape as
// Verified, or by a bare "human:<name>" token with no date.
func TestReviewedIsAttributedAcceptsDatedOrHumanTag(t *testing.T) {
	cases := []struct {
		reviewed string
		want     bool
	}{
		{"2026-07-08 model:sonnet", true},
		{"2026-07-08 human:alex", true},
		{"human:alex", true},
		{"grandfathered", false},
		{"", false},
	}
	for _, c := range cases {
		if got := reviewedIsAttributed(c.reviewed); got != c.want {
			t.Errorf("reviewedIsAttributed(%q) = %v, want %v", c.reviewed, got, c.want)
		}
	}
}

// TestPointQualityNoticeNamesSpecificReasons cross-checks the improvement
// ported from PR #148 (an independent second implementation of this brief,
// closed in favor of this one): a NOTICE must name WHICH criterion failed,
// not just assert "unbacked" generically.
func TestPointQualityNoticeNamesSpecificReasons(t *testing.T) {
	notices := qualityNotices(qualStreams(t))
	var brief02, brief03 string
	for _, n := range notices {
		if strings.Contains(n, "qual/brief-02") {
			brief02 = n
		}
		if strings.Contains(n, "qual/brief-03") {
			brief03 = n
		}
	}
	if !strings.Contains(brief02, "Evidence") {
		t.Errorf("brief-02 (empty Evidence) notice should name Evidence specifically; got %q", brief02)
	}
	if !strings.Contains(brief03, "Verified cell") {
		t.Errorf("brief-03 (bare Verified cell) notice should name the Verified cell specifically; got %q", brief03)
	}
	if strings.Contains(brief03, "Evidence") {
		t.Errorf("brief-03 has filled, independent Evidence — its notice should not also claim an Evidence problem; got %q", brief03)
	}
}

func TestPointQualityNoticesListsExactlyTheUnbackedRows(t *testing.T) {
	notices := qualityNotices(qualStreams(t))
	wantIn := []string{"qual/brief-02", "qual/brief-03", "qual/brief-04", "qual/brief-06", "qual/brief-08"}
	for _, want := range wantIn {
		found := false
		for _, n := range notices {
			if strings.Contains(n, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("qualityNotices missing %q; got:\n%s", want, strings.Join(notices, "\n"))
		}
	}
	wantOut := []string{"qual/brief-01", "qual/brief-05", "qual/brief-07", "qual/brief-09"}
	for _, notWant := range wantOut {
		for _, n := range notices {
			if strings.Contains(n, notWant) {
				t.Errorf("qualityNotices should not flag backed/out-of-scope row %q; got %q", notWant, n)
			}
		}
	}
	if len(notices) != len(wantIn) {
		t.Errorf("qualityNotices returned %d entries, want %d:\n%s", len(notices), len(wantIn), strings.Join(notices, "\n"))
	}
}

// TestPointQualityRendering exercises the full write pipeline (Verify #2):
// STATUS.md must render an unbacked done row as `done*` and a backed done
// row as plain `done`, with the legend present.
func TestPointQualityRendering(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/quality")); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write run exited %d, want 0", code)
	}
	out, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	status := string(out)

	if !strings.Contains(status, "## Done briefs") {
		t.Error("STATUS.md missing the Done briefs section")
	}
	if !strings.Contains(status, "I-08 point quality") && !strings.Contains(status, "unbacked") {
		t.Error("STATUS.md missing the done/done* legend")
	}
	if !strings.Contains(status, "01 Backed done — done (wave 0)") {
		t.Errorf("backed done row should render plain done; got:\n%s", status)
	}
	if !strings.Contains(status, "02 Unbacked done - empty evidence — done* (wave 0)") {
		t.Errorf("unbacked (empty Evidence) done row should render done*; got:\n%s", status)
	}
	if !strings.Contains(status, "03 Unbacked done - bare verified cell — done* (wave 0)") {
		t.Errorf("unbacked (bare Verified cell) done row should render done*; got:\n%s", status)
	}
	if !strings.Contains(status, "04 Legacy grandfathered done — done* (wave 0)") {
		t.Errorf("legacy grandfathered done row should render done*; got:\n%s", status)
	}
	// The verified rows show up in Awaiting verification / review with the
	// same suffix convention.
	if !strings.Contains(status, "| qual | 05 | verified |") {
		t.Errorf("backed verified row should render plain verified in Awaiting verification/review; got:\n%s", status)
	}
	if !strings.Contains(status, "| qual | 06 | verified* |") {
		t.Errorf("unbacked verified row should render verified* in Awaiting verification/review; got:\n%s", status)
	}
}

// TestPointQualityLintNoticeDoesNotChangeExitCode covers Verify #3: the
// NOTICE is visible (returned in a way a caller can inspect) but --lint's
// exit code is unaffected by unbacked rows — only hard problems change it.
func TestPointQualityLintNoticeDoesNotChangeExitCode(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/quality")); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Errorf("lint on a repo with only unbacked (not hard-invalid) rows exited %d, want 0", code)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(qualityNotices(streams)) == 0 {
		t.Fatal("sanity: fixture should produce at least one quality notice")
	}
}
