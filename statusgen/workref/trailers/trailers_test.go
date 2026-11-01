package trailers

import (
	"fmt"
	"testing"
)

func TestFencedExamples(t *testing.T) {
	for _, body := range []string{"~~~\nBrief: demo/01\n~~~\n", "````md\n```\nBrief: demo/01\n````\n", "~~~md\n```\nBrief: demo/01\n~~~\n", "```md\n~~~\nBrief: demo/01\n```\n", "~~~\nBrief: demo/01\n"} {
		if ts := Scan([]byte(body)); len(ts) != 0 {
			t.Fatalf("example became work: %q: %v", body, ts)
		}
	}
	ts, e := ParseStrict([]byte("~~~\nBrief: demo/99\n~~~\nBrief: demo/01\r\n"))
	if e != nil || len(ts) != 1 || ts[0].Value != "demo/01" || ts[0].Line != 4 {
		t.Fatal(ts, e)
	}
}
func TestTrailerProfiles(t *testing.T) {
	body := []byte("Authors: demo/01\nBrief: demo/02\n")
	if len(Scan(body)) != 2 {
		t.Fatal("historical facts lost")
	}
	if _, e := ParseStrict(body); e == nil {
		t.Fatal("new mixed work link admitted")
	}
	if ts, e := ParseStrict([]byte("Authors: demo/01\n")); e != nil || len(ts) != 1 || ts[0].Kind != TrailerAuthors {
		t.Fatal(ts, e)
	}
}

// regression: F-resolving-trailer
// TestResolvingIssueTrailer — `Fixes:`/`Closes:`/`Resolves:` is the same Issue edge as
// `Issue:`, flagged Resolves; `Issue:` itself never resolves; the line must be a real
// GitHub closing keyword (a `#` is required); and it is still THE one link line.
func TestResolvingIssueTrailer(t *testing.T) {
	for _, tc := range []struct {
		line, value, repo string
		resolves          bool
	}{
		{"Issue: #42", "42", "", false},
		{"Issue: 42", "42", "", false},
		{"Fixes: #42", "42", "", true},
		{"Closes: #42", "42", "", true},
		{"  Resolves:#42  ", "42", "", true},
		{"Fixes: example-org/example-repo#7", "7", "example-org/example-repo", true},
	} {
		ts, err := ParseStrict([]byte("body\n" + tc.line + "\n"))
		if err != nil || len(ts) != 1 {
			t.Fatalf("%q: got (%v, %v), want one trailer", tc.line, ts, err)
		}
		got := ts[0]
		if got.Kind != TrailerIssue || got.Value != tc.value || got.Repo != tc.repo || got.Resolves != tc.resolves || got.Line != 2 {
			t.Errorf("%q: got %+v", tc.line, got)
		}
	}
	// Not a trailer: no `#` (GitHub would not close), no colon (prose), lower case,
	// a trailing word, or inside a fence.
	for _, body := range []string{"Fixes: 42\n", "Fixes #42\n", "fixes: #42\n", "Fixes: #42 and more\n", "```\nFixes: #42\n```\n"} {
		if ts := Scan([]byte(body)); len(ts) != 0 {
			t.Errorf("%q became a link: %+v", body, ts)
		}
	}
	// Exactly one link, whichever form: the resolving form duplicates an Issue: line
	// and is refused next to a Brief: or an Authors: line.
	for body, want := range map[string]string{
		"Issue: #1\nFixes: #1\n":        "*trailers.ErrTrailerDuplicate",
		"Fixes: #1\nCloses: #2\n":       "*trailers.ErrTrailerDuplicate",
		"Brief: demo/01\nFixes: #1\n":   "*trailers.ErrTrailerBoth",
		"Authors: demo/01\nFixes: #1\n": "*trailers.ErrTrailerMixed",
	} {
		_, err := ParseStrict([]byte(body))
		if got := fmt.Sprintf("%T", err); got != want {
			t.Errorf("%q: error %s (%v), want %s", body, got, err, want)
		}
	}
}

// regression: F-resolving-trailer
// TestIssueLinkRendering — the comparable rendering edit's immutability check keys on.
func TestIssueLinkRendering(t *testing.T) {
	for line, want := range map[string]string{
		"Issue: #9":                     "Issue: #9",
		"Fixes: #9":                     "Fixes: #9",
		"Closes: #9":                    "Fixes: #9",
		"Resolves: example-org/other#9": "Fixes: example-org/other#9",
	} {
		ts, err := ParseStrict([]byte(line + "\n"))
		if err != nil || len(ts) != 1 {
			t.Fatalf("%q: %v %v", line, ts, err)
		}
		if got := ts[0].IssueLink(); got != want {
			t.Errorf("%q renders %q, want %q", line, got, want)
		}
	}
}
