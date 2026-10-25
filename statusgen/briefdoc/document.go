// Package briefdoc owns offline brief syntax; callers retain admission policy.
package briefdoc

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
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

type Fields struct {
	Values map[string]string
	Risk   Risk
}

// ParseFields decodes real YAML, rejects duplicate keys and invalid risk answers.
func ParseFields(block string) (Fields, error) {
	f := Fields{Values: map[string]string{}}
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(block), &values); err != nil {
		return f, err
	}
	for k, v := range values {
		if k == "regulatory" || k == "customer" || k == "irreversible" || k == "sensitive-data" {
			return f, fmt.Errorf("risk answer %s must be under the risk mapping", k)
		}
		if k == "risk" {
			continue
		}
		switch x := v.(type) {
		case string:
			f.Values[k] = x
		case int, bool, float64:
			f.Values[k] = fmt.Sprint(x)
		default:
			if k == "gate" || k == "effort" || k == "exec-tier" || k == "implementer" || k == "verify-lane" || k == "in-repair" || k == "blocked-until" {
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
		return f, nil
	}
	risk, ok := raw.(map[string]interface{})
	if !ok {
		return f, fmt.Errorf("risk must be a mapping")
	}
	answers := map[string]*bool{"regulatory": &f.Risk.Regulatory, "customer": &f.Risk.Customer, "irreversible": &f.Risk.Irreversible, "sensitive-data": &f.Risk.SensitiveData}
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
	return f, nil
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

// VerifiedStampRE is the canonical dated-runner grammar used by lint and explicit outcomes.
var VerifiedStampRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \S`)

func HasHumanReviewer(stamp string) bool {
	for _, tok := range strings.Fields(stamp) {
		if strings.HasPrefix(tok, "human:") {
			return true
		}
	}
	return false
}
