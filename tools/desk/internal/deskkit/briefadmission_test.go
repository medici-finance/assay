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

// TestBriefAdmissionMultipleDocuments pins that a human gate in a second YAML
// document inside the frontmatter fence still gates the brief.
func TestBriefAdmissionMultipleDocuments(t *testing.T) {
	for _, content := range []string{
		"---\neffort: S\n...\ngate: human\n---\nbody",
		"---\ngate: model\n--- {gate: human}\n---\nbody",
		"---\ngate: model\n---\tgate: human\n---\nbody",
		"---\ngate: model\n\ufeff--- {gate: human}\n---\nbody",
		"---\ngate: model\n\u2800--- {gate: human}\n---\nbody",
		"---\ngate: model\nGate: human\n---\nbody",
	} {
		if b := ParseBriefAdmission(content); !b.GatesHuman() || b.ExecTier != "strong" {
			t.Errorf("second-document human gate dropped: %q read as %+v", content, b)
		}
	}
}

// TestBriefAdmissionContinuationLines pins that a gate, exec-tier or decision
// issue line the shared parser reads as text inside another value makes the
// admission read human and strong, never model and any.
func TestBriefAdmissionContinuationLines(t *testing.T) {
	for _, hidden := range continuationLines {
		content := "---\nbrief: s/07\n" + hidden + "\n---\nbody"
		if b := ParseBriefAdmission(content); !b.GatesHuman() || b.ExecTier != "strong" {
			t.Errorf("continuation line %q read as %+v", hidden, b)
		}
	}
	b := ParseBriefAdmission("---\nbrief: s/07\ngate: human\nmeta: {a: 1,\ndecision-issue: 41\n}\n---\nbody")
	if !b.GatesHuman() || b.ExecTier != "strong" || b.ID != "s/07" || b.DecisionIssue != 41 {
		t.Errorf("unreadable admission dropped its decision issue: %+v", b)
	}
	if b := ParseBriefAdmission("---\n---\nbody"); b != (BriefAdmission{}) {
		t.Errorf("empty frontmatter read as %+v", b)
	}
}

// continuationLines hide a column-0 policy line inside a multi-line value
// opened on an earlier line: the shared parser reads the line as text in that
// value, while a first-match line reader reads it as the key.
var continuationLines = []string{
	"meta: {a: 1,\ngate: human\n}\ngate: model",
	"meta: {a:1,\ngate: human\n}",
	"tags: [a,\ngate: human\n]\ngate: model",
	"tags: [a,\nexec-tier: strong]\nexec-tier: any",
	"title: \"first\ngate: human\n  end\"\ngate: model",
	"title: 'first\nexec-tier: strong\n  end'",
	"meta: {a: 1,\ndecision-issue: 12\n}",
}
