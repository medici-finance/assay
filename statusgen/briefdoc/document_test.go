package briefdoc

import (
	"fmt"
	"regexp"
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
	"gate: model\n...\nnotes: [unclosed",
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
	// The date refusal is its own check, not only the line backstop's.
	if _, e := ParseFields("implementer: 2026-11-01"); e == nil || !strings.Contains(e.Error(), "not a date") {
		t.Errorf("strict key date not refused as a date: %v", e)
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

// formatCharacterKeys are frontmatter blocks whose extra key, or `---` line,
// contains a non-printing character. On screen each reads as a risk
// answer or a human gate; a lower-case-and-trim key match would not see it.
var formatCharacterKeys = []string{
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n\u200bcustomer: yes",
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n\ufeffcustomer: yes",
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n\u2060regulatory: yes",
	"gate: model\nmeta:\n  \u200bcustomer: yes",
	"gate: model\n\ufeff--- {customer: yes}",
	"gate: model\n\ufeff--- {gate: human}",
	"gate: model\n\ufeff---\ncustomer: yes",
	"gate: model\nmeta:\n  - cust\u00adomer: yes",
	"gate: model\nmeta:\n  \"customer\ufe0f\": yes",
	"gate: model\nrisk: {\"\u200bcustomer\": yes}",
	"gate: model\nmeta:\n  1: one\n  \u200bcustomer: yes",
	"gate: model\nmeta:\n  \ue000customer: yes",
}

// TestFormatCharacterKeysRefused pins that a mapping key, at any depth, that
// contains a non-printing character makes the frontmatter unreadable, so every
// desk reader treats the brief as risk-bearing.
func TestFormatCharacterKeysRefused(t *testing.T) {
	for _, input := range formatCharacterKeys {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("format-character-led key accepted: %q read as %+v", input, f)
		}
	}
	for _, input := range []string{"gate: \u200bhuman", "risk: {customer: \"\u200byes\"}"} {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("format character in a gate or risk value accepted: %q read as %+v", input, f)
		}
	}
	// Controls: printable non-ASCII keys and a format character inside a
	// free-text value stay readable.
	for _, input := range []string{"gate: model\ntitle: caf\u00e9 notes", "gate: model\nowner-name: J\u00f6rg", "gate: model\ntitle: a\u200bb", "gate: model\nmeta:\n  r\u00e9sum\u00e9: none"} {
		if _, e := ParseFields(input); e != nil {
			t.Errorf("printable frontmatter refused: %q: %v", input, e)
		}
	}
}

// blankDecoratedRiskKeys are frontmatter blocks whose extra key, or `---`
// line, carries a risk answer behind a printing rune that renders blank, a
// leading combining mark, a non-ASCII space, or a case-fold variant letter.
// The earlier "<risk question>: yes" classifier reads each as risk-bearing.
var blankDecoratedRiskKeys = []string{
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n\u2800customer: yes",
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n\u2800regulatory: yes",
	"gate: model\nmeta:\n  \u2800customer: yes",
	"gate: model\nmeta:\n  - \u2800customer: yes",
	"gate: model\nmeta: {\u2800customer: yes}",
	"gate: model\n\u2800--- {customer: yes}",
	"gate: model\n\u0301customer: yes",
	"gate: model\nmeta:\n  \u0301customer: yes",
	"gate: model\n\u00a0--- {customer: yes}",
	"gate: model\n\u3000--- {customer: yes}",
	"gate: model\n\u2003--- {customer: yes}",
	"gate: model\n\u202f--- {customer: yes}",
	"gate: model\n\u1680--- {customer: yes}",
	"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\ncu\u017ftomer: yes",
	"gate: model\nmeta:\n  cu\u017ftomer: yes",
}

// TestBlankDecoratedRiskKeysRefused pins that each blank-decorated risk answer
// makes the frontmatter unreadable, so every desk reader treats the brief as
// risk-bearing.
func TestBlankDecoratedRiskKeysRefused(t *testing.T) {
	for _, input := range blankDecoratedRiskKeys {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("blank-decorated risk key accepted: %q read as %+v", input, f)
		}
	}
}

// TestKeyChecksIndependent pins each key check on a shape that only it
// refuses: none of these carries a "<risk question>: yes" text, so the text
// backstop does not fire.
func TestKeyChecksIndependent(t *testing.T) {
	for _, input := range []string{
		// non-printing runes in an ordinary key, at each depth and container
		"gate: model\nmeta:\n  ti\u200btle: x",
		"gate: model\nmeta:\n  ti\ue000tle: x",
		"gate: model\nmeta:\n  title\ufe0f: x",
		"gate: model\nmeta:\n  ti\u2800tle: x",
		"gate: model\nmeta:\n  ti\u3000tle: x",
		"gate: model\nmeta:\n  \u0301title: x",
		"gate: model\nmeta:\n  inner:\n    ti\u200btle: x",
		"gate: model\nmeta:\n  1: one\n  ti\u200btle: x",
		"gate: model\nmeta:\n  - ti\u200btle: x",
		// decorated or case-variant risk keys answered no
		"gate: model\nmeta:\n  \"[customer]\": no",
		"gate: model\nmeta:\n  CUSTOMER: no",
		"gate: model\nmeta:\n  cu\u017ftomer: no",
		"gate: model\nmeta:\n  - customer: no",
		"gate: model\ncustomer: no",
		// a top-level key that reads as a policy key but is not spelled as one
		"gate: model\nGate: human",
		"gate: model\n(gate): human",
		"exec-tier: any\nExec-Tier: strong",
	} {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("key check did not refuse: %q read as %+v", input, f)
		}
	}
}

// TestTextRiskBackstop pins that text the earlier "<risk question>: yes"
// classifier reads as a yes, when the parsed risk answer is not yes, makes the
// frontmatter unreadable; and that a parsed yes, or text answering no, does
// not.
func TestTextRiskBackstop(t *testing.T) {
	for _, input := range []string{
		"gate: model\nnotes: |\n  customer: yes",
		"gate: model\ntitle: \"the sensitive-data: yes case\"",
		"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nwhy: >-\n  a sibling with Irreversible:YES is held",
	} {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("text risk answer read as clean: %q read as %+v", input, f)
		}
	}
	for _, input := range []string{
		"gate: model\nrisk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}\nwhy: \"customer: yes is the point\"",
		"gate: model\ntitle: \"customer: no\"",
		"gate: model\nrisk:\n  customer: YES",
	} {
		if _, e := ParseFields(input); e != nil {
			t.Errorf("readable frontmatter refused: %q: %v", input, e)
		}
	}
}

// lineShapes wrap a column-0 line in a multi-line YAML value opened on an
// earlier line, so the parser reads the line as text inside that value and not
// as a top-level key, while a first-match line reader still reads it as one.
var lineShapes = map[string]string{
	"flow mapping":           "meta: {a: 1,\n%s\n}",
	"flow mapping, no space": "meta: {a:1,\n%s\n}",
	"flow sequence":          "tags: [a,\n%s\n]",
	"flow sequence, closing": "tags: [a,\n%s]",
	"double-quoted scalar":   "title: \"first\n%s\n  end\"",
	"single-quoted scalar":   "title: 'first\n%s\n  end'",
}

// lineKeys pairs each line reader key with the line a shape hides and a later
// top-level line that sets the key to something else.
var lineKeys = []struct{ hidden, later string }{
	{"gate: human", "gate: model"},
	{"gate:\n  human", "gate: model"},
	{"exec-tier: strong", "exec-tier: any"},
	{"exec-tier:\n  strong", "exec-tier: any"},
	{"decision-issue: 12", "decision-issue: 3"},
	{"effort: L", "effort: S"},
	{"effort:\n  L", "effort: S"},
	{"implementer: one", "implementer: two"},
	{"blocked-until: 2026-11-01", "blocked-until: 2026-12-01"},
	{"verify-lane: one", "verify-lane: two"},
	{"in-repair: one", "in-repair: two"},
	{"brief: s/07", "brief: s/08"},
	{"deliverable_repo: a/b", "deliverable_repo: a/c"},
	{"homed-in: a/b", "homed-in: a/c"},
}

// TestLineReadersAgree pins that a block where the first-match line readers
// the desk tools used before this parser read a key differently from the
// parsed mapping is refused, with or without a later top-level line for that
// key and with or without a risk mapping; and that ordinary spellings those
// readers and the parser read alike are not.
func TestLineReadersAgree(t *testing.T) {
	const riskNo = "risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n"
	for name, shape := range lineShapes {
		for _, k := range lineKeys {
			hidden := fmt.Sprintf(shape, k.hidden)
			for _, input := range []string{hidden, hidden + "\n" + k.later, riskNo + hidden, riskNo + hidden + "\n" + k.later} {
				if _, err := decodeSingle(input); err != nil {
					t.Fatalf("%s: fixture is not YAML: %q: %v", name, input, err)
				}
				if f, e := ParseFields(input); e == nil {
					t.Errorf("%s: line read differently was accepted: %q read as %+v", name, input, f)
				}
			}
		}
	}
	for _, input := range []string{
		"exec-tier:\n  strong",
		"effort:\n  L",
	} {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("value the line readers read differently was accepted: %q read as %+v", input, f)
		}
	}
	for _, input := range []string{
		"",
		"gate: human # note\nexec-tier: strong\ndecision-issue: \"#12\"\nbrief: s/07\neffort: M",
		"gate: Human\nexec-tier: STRONG",
		"gate:\n  human",
		"deliverable_repo: a/b\nhomed-in: 'a/b'\nimplementer: one\nverify-lane: two\nin-repair: three",
		"blocked-until: 2026-11-01\ntitle: \"first\n  gate: human\n  end\"\ngate: model",
		riskNo + "gate: model\nnotes: |\n  exec-tier: strong",
	} {
		if _, e := ParseFields(input); e != nil {
			t.Errorf("readable frontmatter refused: %q: %v", input, e)
		}
	}
}

// loopScalar is the column-0 reader both work loops used for every key before
// this parser: the first `key:` line, the gap "\s*", the inline comment kept,
// then surrounding space and quotes trimmed.
func loopScalar(block, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*)$`)
	if m := re.FindStringSubmatch(block); m != nil {
		return strings.Trim(strings.TrimSpace(m[1]), `"'`)
	}
	return ""
}

// TestCommentKeepingKeysReadAsBefore pins that every key whose replaced
// readers all kept an inline comment in the value reads exactly as those
// readers did, comment and all, and that the keys whose replaced readers cut
// the comment still read without it.
func TestCommentKeepingKeysReadAsBefore(t *testing.T) {
	for _, key := range []string{"implementer", "blocked-until", "verify-lane", "in-repair"} {
		for _, value := range []string{"no # x", `"" # x`, "' ' # x", "no", `""`, "online", "'pipeline #3'", `"a # b" # c`} {
			input := key + ": " + value
			f, e := ParseFields(input)
			if want := loopScalar(input, key); e != nil || f.Values[key] != want {
				t.Errorf("%q: got %q, %v; the loop reader read %q", input, f.Values[key], e, want)
			}
		}
	}
	for _, marker := range []string{"blocked-until", "in-repair"} {
		input := marker + ": 2026-11-01 # x"
		f, e := ParseFields(input)
		if want := loopScalar(input, marker); e != nil || f.Values[marker] != want {
			t.Errorf("%q: got %q, %v; the loop reader read %q", input, f.Values[marker], e, want)
		}
	}
	// A first match the parser reads as text inside an earlier value is not
	// the key's own line, so its reading is not taken over the key's own line.
	for _, input := range []string{
		"title: \"first\nin-repair: no # x\n  end\"\nin-repair: no # x",
		"meta: {a: 1,\nblocked-until: soon # x\n}\nblocked-until: soon # x",
		"title: \"first\nverify-lane: online # x\n  end\"",
	} {
		if f, e := ParseFields(input); e == nil {
			t.Errorf("hidden comment-keeping line was read as the key's own line: %q read as %+v", input, f)
		}
	}
	for input, want := range map[string]string{
		"gate: human # note":        "human",
		"exec-tier: strong # x":     "strong",
		"effort: M # x":             "M",
		"brief: s/07 # x":           "s/07",
		"decision-issue: 12 # x":    "12",
		"deliverable_repo: a/b # x": "a/b",
		"homed-in: a/b # x":         "a/b",
	} {
		key := strings.SplitN(input, ":", 2)[0]
		if f, e := ParseFields(input); e != nil || f.Values[key] != want {
			t.Errorf("%q: got %q, %v; want %q", input, f.Values[key], e, want)
		}
	}
}
