package evaluation

import (
	"reflect"
	"strings"
	"testing"
)

// witness_gap_test.go — the parser guards the mutation spec pins
// (witnessv2-mutations.json): the list form of `inputs:`, an unclosed
// backtick, and the clean-path rule.

// regression: #4409 review F2 — the list form (`- ` / `* ` items) is read;
// anything else ends the field.
func TestWitnessInputsListForm(t *testing.T) {
	cases := []struct {
		name, ctx string
		want      []string
		found     bool
		problem   string
	}{
		{"dash items", "inputs:\n- `tools/x/pkg/`\n- tools/x/go.mod\n\nfacts: none", []string{"tools/x/go.mod", "tools/x/pkg/"}, true, ""},
		{"star items", "inputs:\n* tools/x/pkg/\n* tools/x/go.mod\n", []string{"tools/x/go.mod", "tools/x/pkg/"}, true, ""},
		{"a prose line ends the list", "inputs:\n- tools/x/pkg/\nfacts: none\n- tools/x/go.mod\n", []string{"tools/x/pkg/"}, true, ""},
		{"a dash-less line right after the label names nothing", "inputs:\ntools/x/pkg/\n", nil, false, "names no entry"},
		{"an indented continuation joins the item", "inputs:\n- tools/x/pkg/\n  tools/x/go.mod\n", []string{"tools/x/go.mod", "tools/x/pkg/"}, true, ""},
		{"inline value and list items", "inputs: tools/x/pkg/\n- tools/x/go.mod\n", nil, false, "both an inline value and list items"},
	}
	for _, tc := range cases {
		got, found, problem := ParseInputsDeclaration(tc.ctx)
		if found != tc.found || !reflect.DeepEqual(got, tc.want) || !strings.Contains(problem, tc.problem) || (tc.problem == "" && problem != "") {
			t.Errorf("%s: got %v found=%v problem=%q, want %v found=%v problem~%q", tc.name, got, found, problem, tc.want, tc.found, tc.problem)
		}
	}
}

// regression: #4409 — an unclosed backtick is a named refusal, not a token
// that merely fails the path test.
func TestWitnessInputsUnclosedBacktick(t *testing.T) {
	for _, ctx := range []string{"inputs: `tools/x/pkg/\n", "inputs: tools/x/pkg/ `tools/x/go.mod\n", "inputs:\n- `tools/x/pkg/\n"} {
		got, found, problem := ParseInputsDeclaration(ctx)
		if found || len(got) != 0 || !strings.Contains(problem, "unclosed backtick") {
			t.Errorf("%q: got %v found=%v problem=%q, want an unclosed-backtick refusal", ctx, got, found, problem)
		}
	}
}

// regression: #4409 — a witness path must be clean: `.`, `./a`, `a//b`,
// `a/../b`, a trailing slash and a leading `../` are refused.
func TestWitnessPathClean(t *testing.T) {
	for _, p := range []string{"a/../b", "./a", "a//b", ".", "../x", "a/", "..", "a/./b", "/abs", "", `a\\b`} {
		if witnessPathOK(p) == nil {
			t.Errorf("witnessPathOK(%q) accepted a non-canonical path", p)
		}
	}
	for _, p := range []string{"a", "a/b.go", "tools/x/go.mod", ".github/x"} {
		if err := witnessPathOK(p); err != nil {
			t.Errorf("witnessPathOK(%q) = %v, want accepted", p, err)
		}
	}
	// And through the record parser: a record keyed by a non-clean path is refused.
	fx := newV2fx(t)
	rec := fx.defaultRecord(fx.row)
	canonical, _, err := EncodeWitnessRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"tools/x/pkg/../pkg/a.go", "./tools/x/pkg/a.go", "tools/x//pkg/a.go"} {
		mangled := strings.Replace(string(canonical), `"tools/x/pkg/a.go"`, `"`+bad+`"`, 1)
		if _, _, err := ParseWitnessRecord([]byte(mangled)); err == nil {
			t.Errorf("a record keyed by %q parsed", bad)
		}
	}
}
