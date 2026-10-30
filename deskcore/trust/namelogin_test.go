package trust

import (
	"reflect"
	"strings"
	"testing"
)

// regression: #4978
func TestSplitNameLoginList(t *testing.T) {
	for raw, want := range map[string][]string{
		"":                               {},
		"ada:a,bob:b":                    {"ada:a", "bob:b"},
		"ada:a bob:b":                    {"ada:a", "bob:b"},
		"ada:a;bob:b\nc:d\r\ne:f":        {"ada:a", "bob:b", "c:d", "e:f"},
		" ada:a ,\tbob:b ":               {"ada:a", "bob:b"},
		"Ada Lovelace:a, bob:b":          {"Ada Lovelace:a", "bob:b"},
		"Ada  Byron\tLovelace:a":         {"Ada Byron Lovelace:a"},
		"bob:b Ada Lovelace:a":           {"bob:b", "Ada Lovelace:a"},
		"ada:a, lovelace":                {"ada:a", "lovelace"},
		"ada lovelace":                   {"ada", "lovelace"},
		"Ada Lovelace:a, Grace Hopper:g": {"Ada Lovelace:a", "Grace Hopper:g"},
	} {
		if got := SplitNameLoginList(raw); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitNameLoginList(%q) = %q, want %q", raw, got, want)
		}
	}
}

// Every value with no spaced name splits exactly as SplitList splits it.
//
// regression: #4978
func TestSplitNameLoginMatchesList(t *testing.T) {
	for _, raw := range []string{"ada:a,bob:b", "ada:a bob:b", "a:b;c:d\te:f", "ada:a, lovelace", "x", ""} {
		if got, want := SplitNameLoginList(raw), SplitList(raw); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitNameLoginList(%q) = %q, SplitList = %q", raw, got, want)
		}
	}
}

// regression: #4978
func TestSpacedNameNotice(t *testing.T) {
	n := SpacedNameNotice("ASSAY_HUMAN_LOGIN_MAP", "Ada Lovelace:a")
	for _, want := range []string{"ASSAY_HUMAN_LOGIN_MAP", `"Ada Lovelace:a"`, "NOT applied", "NO mapping"} {
		if !strings.Contains(n, want) {
			t.Errorf("notice %q lacks %q", n, want)
		}
	}
	if !NameHasWhitespace("ada lovelace") || !NameHasWhitespace("ada\tl") || NameHasWhitespace("ada-lovelace") {
		t.Error("NameHasWhitespace misclassifies")
	}
}
