package deskkit

import (
	"strings"
	"testing"
)

// TestWithoutOnBehalfOf: for every body AppendOnBehalfOf produces, removing the on-behalf-of
// lines from the posted body and from the caller's body gives the same text — and a change
// to anything else still shows.
func TestWithoutOnBehalfOf(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	t.Setenv("DESK_SESSION", "sess-"+t.Name())
	const repo = privateFixtureRepo
	cases := []struct {
		name string
		body string
	}{
		{"plain", "A verdict.\n\nNo blockers.\n"},
		{"no trailing newline", "A verdict."},
		{"many trailing newlines", "A verdict.\n\n\n"},
		{"a planted line in the middle", "First.\n" + OnBehalfOfPrefix + " human:somebody-else\nLast.\n"},
		{"a planted line at the end", "First.\n\n" + OnBehalfOfPrefix + " human:somebody-else\n"},
		{"an indented planted line", "First.\n   " + OnBehalfOfPrefix + " human:somebody-else\nLast."},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			posted, err := AppendOnBehalfOf([]byte(c.body), "", repo)
			if err != nil {
				t.Fatalf("AppendOnBehalfOf: %v", err)
			}
			if !strings.Contains(string(posted), OnBehalfOfPrefix) {
				t.Fatal("fixture defect: the posted body carries no on-behalf-of line")
			}
			got, want := WithoutOnBehalfOf(string(posted)), WithoutOnBehalfOf(c.body)
			if got != want {
				t.Fatalf("posted and caller bodies differ once the lines are removed:\n posted=%q\n caller=%q", got, want)
			}
			if strings.Contains(got, OnBehalfOfPrefix) {
				t.Fatalf("an on-behalf-of line survived: %q", got)
			}
			if c.body != "" && WithoutOnBehalfOf(string(posted)+"x") == want {
				t.Fatal("a change to the text did not change the result")
			}
		})
	}
}
