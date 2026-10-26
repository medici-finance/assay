package briefdoc

import (
	"strings"
	"testing"
)

func TestQuotedRiskAndMalformed(t *testing.T) {
	for _, input := range []string{`risk: {regulatory: yes}`, `risk: {regulatory: "yes"}`} {
		f, e := ParseFields(input)
		if e != nil || !f.Risk.Regulatory {
			t.Fatal(input, f, e)
		}
	}
	for _, input := range []string{"risk: [yes]", "gate: unsupported", "exec-tier: unsupported", "risk: {typo: yes}", "risk: {regulatory: maybe}", "gate: model\ngate: human", "risk: {regulatory: no, regulatory: yes}"} {
		if _, e := ParseFields(input); e == nil {
			t.Fatalf("malformed frontmatter accepted: %s", input)
		}
	}
}

// TestRiskAnswerOutsideRiskMapping pins the fail-closed direction: a risk-answer
// key anywhere outside the top-level risk mapping makes the frontmatter
// unreadable, so every caller treats the brief as risk-bearing.
func TestRiskAnswerOutsideRiskMapping(t *testing.T) {
	for _, input := range []string{
		"risks:\n  regulatory: yes",
		"meta:\n  customer: yes",
		"meta:\n  nested:\n    irreversible: yes",
		"notes:\n  - sensitive-data: yes",
		"meta:\n  Regulatory: yes",
		"Customer: yes",
		"risk:\n  regulatory: no\nextra:\n  customer: no",
	} {
		if _, e := ParseFields(input); e == nil {
			t.Errorf("risk answer outside the risk mapping accepted: %q", input)
		}
	}
	// Controls: a legitimate top-level risk mapping still parses, and an
	// unrelated mapping key is left alone.
	f, e := ParseFields("gate: human\nrisk:\n  regulatory: yes\n  customer: no\n  irreversible: no\n  sensitive-data: yes\nmeta:\n  owner: someone\n  tags: [a, b]")
	if e != nil || !f.Risk.Regulatory || f.Risk.Customer || f.Risk.Irreversible || !f.Risk.SensitiveData || f.Values["gate"] != "human" {
		t.Fatalf("legitimate frontmatter misread: %+v %v", f, e)
	}
	if _, e := ParseFields("meta:\n  owner: someone\n  regulation-notes: none"); e != nil {
		t.Fatalf("unrelated mapping refused: %v", e)
	}
}

// multiDocument are frontmatter blocks whose second YAML document carries a
// risk answer or a human gate. A single-document decode would drop it.
var multiDocument = []string{
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n...\nregulatory: yes",
	"effort: S\n...\ngate: human",
	"gate: model\n--- {customer: yes}",
	"gate: model\n\n# a second document follows\n--- # note\nrisk: {customer: yes}",
	"gate: model\n---\tgate: human",
	"gate: model\n...\n%YAML 1.2\n--- {customer: yes}",
	"gate: model\n---\ngate: human",
}

// TestMultipleDocumentsRefused pins that a frontmatter block holding more than
// one YAML document is an error, so every desk reader treats it as unreadable
// and risk-bearing instead of reading only the first document.
func TestMultipleDocumentsRefused(t *testing.T) {
	for _, input := range multiDocument {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("second YAML document ignored: %q read as %+v", input, f)
		}
	}
	// Controls: a lone document-end marker, text containing dots or dashes, and
	// a block scalar holding a dashed line are one document.
	for _, input := range []string{
		"gate: model\n...",
		"gate: model\n...\n# trailing comment",
		"title: wait...\ngate: model",
		"notes: |\n  ---\n  not a document\ngate: model",
		"",
	} {
		if _, e := ParseFields(input); e != nil {
			t.Errorf("single document refused: %q: %v", input, e)
		}
	}
}

// TestUnquotedDateMarkers pins that an unquoted YAML date or timestamp for the
// free-text markers is read as its source text, so a deferral marker defers
// instead of making the brief unreadable. Other strict keys keep refusing a
// non-text value.
func TestUnquotedDateMarkers(t *testing.T) {
	for input, want := range map[string]string{
		"blocked-until: 2026-11-01":           "2026-11-01",
		"blocked-until: 2026-11-01T10:00:00Z": "2026-11-01T10:00:00Z",
		"blocked-until: '2026-11-01'":         "2026-11-01",
		"in-repair: 2026-11-01":               "2026-11-01",
	} {
		f, e := ParseFields(input)
		key := strings.SplitN(input, ":", 2)[0]
		if e != nil || f.Values[key] != want {
			t.Errorf("%q: got %q, %v; want %q", input, f.Values[key], e, want)
		}
	}
	for _, input := range []string{"gate: 2026-11-01", "exec-tier: 2026-11-01", "effort: 2026-11-01", "implementer: 2026-11-01", "verify-lane: 2026-11-01", "blocked-until: [2026-11-01]", "blocked-until: {at: 2026-11-01}"} {
		if _, e := ParseFields(input); e == nil {
			t.Errorf("strict key accepted a non-text value: %q", input)
		}
	}
}

func TestSectionConsumers(t *testing.T) {
	for _, heading := range []string{"## Verify", " ## Verify (executable)", "##\tVerify (executable)", "##   Verify"} {
		body := heading + "\r\nCOMMAND\r\n## Evidence\r\nOBSERVED\r\n"
		if strings.TrimSpace(Section(body, "Verify", true)) != "COMMAND" || strings.TrimSpace(Section(body, "Evidence", false)) != "OBSERVED" {
			t.Fatal(heading)
		}
	}
}

func TestCommentContentBoundary(t *testing.T) {
	if HasContent("<!-- unfinished\nnot visible") {
		t.Fatal("unterminated comment counted as content")
	}
	if !HasContent("<!-- closed -->\nvisible") {
		t.Fatal("visible content lost")
	}
}
