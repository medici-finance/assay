package verifycmd

import (
	"reflect"
	"testing"
)

// TestSplitRowKeepsEscapedPipe — an escaped `\|` is cell content; outer pipes are dropped
// once from each end; an unclosed row keeps its last cell.
func TestSplitRowKeepsEscapedPipe(t *testing.T) {
	for line, want := range map[string][]string{
		"| 1 | `a \\| b` | exit 0 |": {" 1 ", " `a \\| b` ", " exit 0 "},
		"| 1 |  | x |":               {" 1 ", "  ", " x "},
		"| 1 | x":                    {" 1 ", " x"},
		"|a\\|\\|b|":                 {"a\\|\\|b"},
		"||":                         {""},
	} {
		if got := SplitRow(line); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitRow(%q) = %q, want %q", line, got, want)
		}
	}
	if got := UnescapePipes("a \\| b \\|\\| c"); got != "a | b || c" {
		t.Errorf("UnescapePipes = %q", got)
	}
	if got := EscapePipes("a | b"); got != "a \\| b" {
		t.Errorf("EscapePipes = %q", got)
	}
}
