package deskkit

import "testing"

// TierCovers is the one tier comparison deskdispatch and deskboard share (medici-finance/assay#2744).
func TestTierCoversOrder(t *testing.T) {
	cases := []struct {
		dispatch, brief string
		want            bool
	}{
		{"any", "", true},
		{"any", "any", true},
		{"any", "strong", false},
		{"strong", "strong", true},
		{"strong", "any", true},
		{"strong", `"strong"`, true},
	}
	for _, c := range cases {
		got, err := TierCovers(c.dispatch, c.brief)
		if err != nil || got != c.want {
			t.Errorf("TierCovers(%q, %q) = %v, %v; want %v", c.dispatch, c.brief, got, err, c.want)
		}
	}
	if _, err := TierCovers("any", "fast"); err == nil {
		t.Error("an exec-tier outside the vocabulary compared without an error")
	}
}

func TestParseBriefAdmission(t *testing.T) {
	b := ParseBriefAdmission("---\nbrief: s/07\ngate: Human\nexec-tier: strong # why\ndecision-issue: \"#41\"\n---\n\nbody\nexec-tier: any\n")
	if b.ID != "s/07" || !b.GatesHuman() || b.ExecTier != "strong" || b.DecisionIssue != 41 || !b.Frontmatter {
		t.Errorf("parsed %+v", b)
	}
	if got := ParseBriefAdmission("no fence\nexec-tier: strong\n"); got.ExecTier != "" || got.Frontmatter {
		t.Errorf("a body line was read as frontmatter: %+v", got)
	}
}
