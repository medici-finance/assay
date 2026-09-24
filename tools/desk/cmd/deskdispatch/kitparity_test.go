package main

// kitparity_test.go — the defect-class clause is ONE wording across both implementer kits.
//
// worker-prompt-objective.md promises to carry every load-bearing clause of the procedural
// worker kit verbatim; nothing checked that promise, so a clause strengthened in one kit
// could silently stay weak in the other. This pins the bug-fix defect-class clause: its body
// must be byte-identical in the worker kit and the worker-objective kit, and must still carry
// the three obligations (name the class, guard the class, fail-first against a planted second
// instance).

import (
	"strings"
	"testing"
)

const defectClassHeading = "A bug fix closes the defect CLASS, not the one instance"

// clauseBody returns the text under the heading that ends with title, up to the next markdown
// heading or horizontal rule, trimmed. The heading's own level and numbering are excluded, so
// a numbered "## 14." and an unnumbered "###" compare on their bodies alone. "" = not found.
func clauseBody(kit, title string) string {
	lines := strings.Split(kit, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "#") && strings.HasSuffix(strings.TrimSpace(l), title) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") || strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func TestDefectClassClauseIsOneWordingAcrossImplementerKits(t *testing.T) {
	bodies := map[string]string{}
	for _, kit := range []string{"worker", "worker-objective"} {
		text, err := kitText(kit)
		if err != nil {
			t.Fatalf("kit %q: %v", kit, err)
		}
		body := clauseBody(text, defectClassHeading)
		if body == "" {
			t.Fatalf("kit %q carries no %q clause — a worker dispatched on it fixes the instance and "+
				"leaves the class open", kit, defectClassHeading)
		}
		for _, must := range []string{"## Defect class", "ALLOW-LIST", "PLANTED SECOND", "## Fail-first"} {
			if !strings.Contains(body, must) {
				t.Errorf("kit %q defect-class clause lost %q — one of its three obligations is gone", kit, must)
			}
		}
		bodies[kit] = body
	}
	if bodies["worker"] != bodies["worker-objective"] {
		t.Errorf("the defect-class clause differs between the worker and worker-objective kits — the " +
			"objective kit promises the procedural kit's clauses verbatim, so edit both in the same change")
	}
}

// The extractor is the instrument; prove it finds a clause, stops at the next heading, and
// reports absence as "" rather than returning the rest of the file.
func TestClauseBodyExtractorIsLive(t *testing.T) {
	kit := "# kit\n\n## 3. Example clause\n\nbody line\n\n## 4. Next\n\nother\n"
	if got := clauseBody(kit, "Example clause"); got != "body line" {
		t.Errorf("clauseBody = %q, want %q", got, "body line")
	}
	if got := clauseBody(kit, "Missing clause"); got != "" {
		t.Errorf("clauseBody on an absent heading = %q, want empty", got)
	}
}
