package contexthandoff

import (
	"fmt"
	"testing"
)

// regression: #2763 SEC-1
func TestDeliveryPaths(t *testing.T) {
	base := fixtureTree(t, pairedManifestPath)
	schema := loadSchema(t, manifestSchemaPath)
	cases := []struct {
		path  string
		valid bool
	}{
		{"docs/demo.md", true}, {"./docs/demo.md", true}, {".config/demo", true},
		{"docs/a..b", true}, {"docs/...", true}, {"docs/\u00e9.md", true},
		{"docs/~demo", true}, {"docs//demo.md", true}, {"docs/", true},
		{"", false}, {"/demo", false}, {"../demo", false}, {"..", false},
		{"docs/../demo", false}, {"docs/..", false}, {"docs\\demo", false},
		{"C:demo", false}, {"C:/demo", false}, {"C:\\demo", false},
		{"\\\\server\\share", false}, {"//server/share", false},
		{"~/demo", false}, {"~example/demo", false}, {"file:demo", false},
		{"https://example.invalid/demo", false}, {"git+demo:asset", false},
	}
	// Exercise every C0/C1 control and every bidi formatting control, not only
	// the reported ESC and RLO samples. Values stay JSON data; no path is opened.
	for r := rune(0); r <= 0x9f; r++ {
		if r < 0x20 || r >= 0x7f {
			cases = append(cases, struct {
				path  string
				valid bool
			}{"docs/" + string(r) + "demo", false})
		}
	}
	for _, r := range []rune{0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069} {
		cases = append(cases, struct {
			path  string
			valid bool
		}{"docs/" + string(r) + "demo", false})
	}
	for i, c := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			// A second entry and a second array element close the whole field class.
			for _, at := range []string{"entries.0.delivery_paths", "entries.1.delivery_paths"} {
				data := marshal(t, set(base, at, []any{"docs/control.md", c.path}))
				_, err := DecodeManifest(data)
				if (err == nil) != c.valid {
					t.Errorf("decoder %s %q: %v; valid=%t", at, c.path, err, c.valid)
				}
				if !c.valid && RuleOf(err) != RuleBadValue {
					t.Errorf("rule=%q want bad-value", RuleOf(err))
				}
				if err := schema.Accepts(data); (err == nil) != c.valid {
					t.Errorf("schema %s %q: %v; valid=%t", at, c.path, err, c.valid)
				}
			}
		})
	}
}

// regression: #2763 SEC-A2
func TestWhitespaceParity(t *testing.T) {
	// ECMA-262 whitespace, plus NEL (Unicode White_Space). Explicit values
	// keep the oracle independent of either layer's regular expression.
	spaces := []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff}
	for _, kind := range []string{"handoff", "manifest"} {
		path, sp, locator, txt := pairedHandoffPath, handoffSchemaPath, "target.acceptance_source", "target.intent"
		if kind == "manifest" {
			path, sp, locator, txt = pairedManifestPath, manifestSchemaPath, "entries.0.contract_version", "entries.0.active_work"
		}
		base, schema := fixtureTree(t, path), loadSchema(t, sp)
		for _, r := range spaces {
			for _, at := range []string{locator, txt} {
				value := string(r)
				if at == locator {
					value = "docs/" + value + "demo"
				}
				data := marshal(t, set(base, at, value))
				if err := decodeKind(t, kind, data); RuleOf(err) != RuleBadValue {
					t.Errorf("%s %s U+%04X decoder=%v", kind, at, r, err)
				}
				if err := schema.Accepts(data); err == nil {
					t.Errorf("%s %s U+%04X schema accepted whitespace", kind, at, r)
				}
			}
		}
	}
}

// regression: #2763 SEC-1
func TestSchemaNegation(t *testing.T) {
	s := &testSchema{}
	for _, tc := range []struct {
		value any
		valid bool
	}{{"demo", false}, {nil, true}, {true, true}} {
		node := map[string]any{"not": map[string]any{"type": "string"}}
		if err := s.validate(tc.value, node, "$"); (err == nil) != tc.valid {
			t.Errorf("not string on %v: %v", tc.value, err)
		}
	}
}
