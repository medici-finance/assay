// Package briefdoc owns offline brief syntax; callers retain admission policy.
package briefdoc

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func Split(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("no frontmatter: first line must be ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", fmt.Errorf("unterminated frontmatter")
}

// Frontmatter returns the normalized fenced YAML, or empty for unreadable input.
func Frontmatter(content string) string {
	v, _, e := Split(content)
	if e != nil {
		return ""
	}
	return v
}

type Risk struct{ Regulatory, Customer, Irreversible, SensitiveData bool }

func UnreadableRisk() Risk { return Risk{true, true, true, true} }

// riskKeys are the four risk questions in canonical order. They are the one
// source every reader derives the risk-answer key set from.
var riskKeys = [...]string{"regulatory", "customer", "irreversible", "sensitive-data"}

// RiskKeys returns the four risk questions in canonical order.
func RiskKeys() []string { return append([]string(nil), riskKeys[:]...) }

// answers maps each risk question to its field, in riskKeys order.
func (r *Risk) answers() map[string]*bool {
	fields := [len(riskKeys)]*bool{&r.Regulatory, &r.Customer, &r.Irreversible, &r.SensitiveData}
	m := make(map[string]*bool, len(riskKeys))
	for i, k := range riskKeys {
		m[k] = fields[i]
	}
	return m
}

// keyLetters keeps only the letters, digits and hyphens of a key, without
// leading or trailing hyphens, so a policy key spelling decorated with spaces,
// marks, symbols or a "---" prefix compares equal to the plain spelling.
func keyLetters(k string) string {
	var b strings.Builder
	for _, r := range k {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

// isRiskKey reports whether a mapping key names a risk question once its
// decoration is dropped (keyLetters), under Unicode case folding, so a
// near-miss spelling cannot slip past as an ordinary key.
func isRiskKey(k string) bool {
	k = keyLetters(k)
	for _, r := range riskKeys {
		if strings.EqualFold(k, r) {
			return true
		}
	}
	return false
}

// policyKeys are the top-level keys that route a brief to a human or a strong
// implementer. A key that reads as one of them but is not spelled exactly
// like it is refused rather than ignored.
var policyKeys = [...]string{"gate", "exec-tier"}

// decoratedPolicyKey reports a top-level key that reads as a policy key but is
// not spelled exactly like it.
func decoratedPolicyKey(k string) bool {
	for _, p := range policyKeys {
		if k != p && strings.EqualFold(keyLetters(k), p) {
			return true
		}
	}
	return false
}

// textRiskAnswer is the risk classifier the desk readers used before this
// parser: a case-insensitive "<risk question>: yes" anywhere in the block.
var textRiskAnswer = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(riskKeys))
	for _, k := range riskKeys {
		m[k] = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(k) + `\s*:\s*yes`)
	}
	return m
}()

// textRiskBackstop refuses a block whose text the earlier classifier would
// read as answering a risk question yes when the parsed answer is not yes, so
// the parser never reads a brief as less risky than that classifier did.
func textRiskBackstop(block string, r Risk) error {
	answers := r.answers()
	for _, k := range riskKeys {
		if textRiskAnswer[k].MatchString(block) && !*answers[k] {
			return fmt.Errorf("frontmatter text answers %s yes outside the risk mapping", k)
		}
	}
	return nil
}

// misplacedRiskKey returns the first risk-answer key found at any depth in v,
// which is a value outside the top-level risk mapping.
func misplacedRiskKey(v interface{}) (string, bool) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, child := range x {
			if isRiskKey(k) {
				return k, true
			}
			if found, ok := misplacedRiskKey(child); ok {
				return found, true
			}
		}
	case map[interface{}]interface{}:
		for k, child := range x {
			if s, ok := k.(string); ok && isRiskKey(s) {
				return s, true
			}
			if found, ok := misplacedRiskKey(child); ok {
				return found, true
			}
		}
	case []interface{}:
		for _, child := range x {
			if found, ok := misplacedRiskKey(child); ok {
				return found, true
			}
		}
	}
	return "", false
}

// invisibleRune reports a rune that does not print: a format character
// (category Cf), another non-graphic rune, a variation selector, another
// default-ignorable code point, or a blank-rendering rune. A key holding one
// can look identical on screen to a key it does not equal.
func invisibleRune(r rune) bool {
	return unicode.Is(unicode.Cf, r) || !unicode.IsGraphic(r) ||
		unicode.In(r, unicode.Variation_Selector, unicode.Other_Default_Ignorable_Code_Point) ||
		blankRune(r)
}

// blankRune reports a printing rune that renders as empty space: a space
// separator other than the ASCII space, or a blank-rendering symbol.
func blankRune(r rune) bool {
	return r != ' ' && unicode.Is(unicode.Zs, r) || r == '\u2800'
}

// invisibleText reports a key that holds an invisible rune or starts with a
// combining mark, which has no base letter to attach to and so renders on
// whatever precedes it.
func invisibleText(k string) bool {
	if r, _ := utf8.DecodeRuneInString(k); unicode.Is(unicode.M, r) {
		return true
	}
	return strings.IndexFunc(k, invisibleRune) >= 0
}

// invisibleKey returns the first mapping key, at any depth in v, that contains
// an invisible rune.
func invisibleKey(v interface{}) (string, bool) {
	hasInvisible := invisibleText
	switch x := v.(type) {
	case map[string]interface{}:
		for k, child := range x {
			if hasInvisible(k) {
				return k, true
			}
			if found, ok := invisibleKey(child); ok {
				return found, true
			}
		}
	case map[interface{}]interface{}:
		for k, child := range x {
			if s, ok := k.(string); ok && hasInvisible(s) {
				return s, true
			}
			if found, ok := invisibleKey(child); ok {
				return found, true
			}
		}
	case []interface{}:
		for _, child := range x {
			if found, ok := invisibleKey(child); ok {
				return found, true
			}
		}
	}
	return "", false
}

// textMarkers are free-text frontmatter markers whose value is shown as
// written. An unquoted date or timestamp decodes as a YAML timestamp, so these
// keys take the scalar's source text instead of refusing it.
var textMarkers = map[string]bool{"blocked-until": true, "in-repair": true}

// scalarKeys must hold a scalar; a mapping or sequence value is an error.
var scalarKeys = map[string]bool{"gate": true, "effort": true, "exec-tier": true, "implementer": true, "verify-lane": true, "in-repair": true, "blocked-until": true}

// sourceScalar returns the source text of a top-level scalar value.
func sourceScalar(block, key string) (string, bool) {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(block), &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", false
	}
	m := doc.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key && m.Content[i+1].Kind == yaml.ScalarNode {
			return m.Content[i+1].Value, true
		}
	}
	return "", false
}

type Fields struct {
	Values map[string]string
	Risk   Risk
}

// decodeSingle decodes block as exactly one YAML document. A block holding a
// second document (after a "..." or "--- <content>" line) is an error: a
// single-document decode would silently drop whatever the later document says.
func decodeSingle(block string) (map[string]interface{}, error) {
	var values map[string]interface{}
	dec := yaml.NewDecoder(strings.NewReader(block))
	if err := dec.Decode(&values); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	} else if err != nil {
		return nil, nil
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("frontmatter holds more than one YAML document")
	}
	return values, nil
}

// ParseFields decodes real YAML, rejects duplicate keys and invalid risk answers.
// A risk answer is read only from the top-level risk mapping; a risk-answer key
// anywhere else, a second YAML document in the block, or a mapping key at any
// depth that contains an invisible character, a top-level key that reads as
// gate or exec-tier without being spelled as one, or text the earlier
// "<risk question>: yes" classifier reads as a yes the parsed risk answer does
// not give, is an error, which every desk caller treats as risk-bearing.
func ParseFields(block string) (Fields, error) {
	f := Fields{Values: map[string]string{}}
	values, err := decodeSingle(block)
	if err != nil {
		return f, err
	}
	if found, ok := invisibleKey(map[string]interface{}(values)); ok {
		return f, fmt.Errorf("frontmatter key %q contains an invisible character", found)
	}
	for k, v := range values {
		if decoratedPolicyKey(k) {
			return f, fmt.Errorf("frontmatter key %q reads as a policy key but is not spelled as one", k)
		}
		if isRiskKey(k) {
			return f, fmt.Errorf("risk answer %s must be under the risk mapping", k)
		}
		if k == "risk" {
			continue
		}
		if found, ok := misplacedRiskKey(v); ok {
			return f, fmt.Errorf("risk answer %s under %s must be under the top-level risk mapping", found, k)
		}
		switch x := v.(type) {
		case string:
			f.Values[k] = x
		case int, bool, float64:
			f.Values[k] = fmt.Sprint(x)
		case time.Time:
			if src, ok := sourceScalar(block, k); ok && textMarkers[k] {
				f.Values[k] = src
			} else if scalarKeys[k] {
				return f, fmt.Errorf("%s must be text, not a date", k)
			}
		default:
			if scalarKeys[k] {
				return f, fmt.Errorf("%s must be a scalar", k)
			}
		}
	}
	for _, key := range []string{"gate", "exec-tier"} {
		if _, present := values[key]; present {
			v := strings.ToLower(strings.TrimSpace(f.Values[key]))
			valid := key == "gate" && (v == "model" || v == "human") || key == "exec-tier" && (v == "any" || v == "strong")
			if !valid {
				return f, fmt.Errorf("unsupported %s value", key)
			}
			f.Values[key] = v
		}
	}
	raw, ok := values["risk"]
	if !ok {
		return f, textRiskBackstop(block, f.Risk)
	}
	risk, ok := raw.(map[string]interface{})
	if !ok {
		return f, fmt.Errorf("risk must be a mapping")
	}
	answers := f.Risk.answers()
	for k := range risk {
		if _, known := answers[k]; !known {
			return f, fmt.Errorf("unknown risk answer %s", k)
		}
	}
	for k, target := range answers {
		v, present := risk[k]
		if !present {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return f, fmt.Errorf("risk %s must be yes or no", k)
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "yes":
			*target = true
		case "no":
		default:
			return f, fmt.Errorf("risk %s must be yes or no", k)
		}
	}
	return f, textRiskBackstop(block, f.Risk)
}
func Scalar(block, key string) string {
	f, e := ParseFields(block)
	if e != nil {
		return ""
	}
	return f.Values[key]
}
func Risks(block string) (Risk, error) { f, e := ParseFields(block); return f.Risk, e }

// Heading interprets an ATX heading independently of its consumer's projection.
func Heading(line string) (int, string) {
	t := strings.TrimSpace(line)
	rest := strings.TrimLeft(t, "#")
	n := len(t) - len(rest)
	if n < 1 || n > 6 {
		return 0, ""
	}
	if r, _ := utf8.DecodeRuneInString(rest); rest != "" && !unicode.IsSpace(r) {
		return 0, ""
	}
	return n, strings.TrimSpace(rest)
}
func IsSection(line string) bool { n, _ := Heading(line); return n == 2 }
func IsVerify(line string) bool {
	n, t := Heading(line)
	if n != 2 {
		return false
	}
	return strings.HasPrefix(t, "Verify") || strings.EqualFold(t, "Verify")
}
func IsEvidence(line string) bool { n, t := Heading(line); return n == 2 && t == "Evidence" }

// SectionBounds returns content offsets and preserves the original lines for writers.
func SectionBounds(lines []string, title string, prefix bool) (int, int, bool) {
	start := -1
	for i, line := range lines {
		n, t := Heading(line)
		if n != 2 {
			continue
		}
		match := t == title
		if title == "Verify" {
			match = IsVerify(line)
		} else if prefix {
			match = strings.HasPrefix(t, title)
		}
		if match {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if IsSection(lines[i]) {
			end = i
			break
		}
	}
	return start, end, true
}
func Section(body, title string, prefix bool) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	a, b, ok := SectionBounds(lines, title, prefix)
	if !ok {
		return ""
	}
	return strings.Join(lines[a:b], "\n")
}

var comments = regexp.MustCompile(`(?s)<!--.*?(?:-->|$)`)

func HasContent(section string) bool {
	return strings.TrimSpace(comments.ReplaceAllString(section, "")) != ""
}
