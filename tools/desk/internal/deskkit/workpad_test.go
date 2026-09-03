package deskkit

import (
	"strings"
	"testing"
)

func TestWorkpadRenderCarriesMarkerAsItsOwnLine(t *testing.T) {
	body := Render(Workpad{Stamp: "myworktree@abc1234", Plan: "- do the thing"})
	if !HasWorkpadMarker(body) {
		t.Fatalf("Render output does not carry the workpad marker as its own line:\n%s", body)
	}
	lines := strings.Split(body, "\n")
	if strings.TrimSpace(lines[0]) != WorkpadMarker {
		t.Fatalf("marker must be the FIRST line; got %q", lines[0])
	}
}

func TestWorkpadRenderIncludesEveryFixedSectionEvenWhenEmpty(t *testing.T) {
	body := Render(Workpad{Stamp: "w@abc1234"})
	for _, h := range []string{workpadPlanHeader, workpadAcceptanceHeader, workpadValidationHeader, workpadNotesHeader} {
		if !strings.Contains(body, h) {
			t.Fatalf("Render output missing section header %q:\n%s", h, body)
		}
	}
}

func TestWorkpadParseRoundTrip(t *testing.T) {
	w := Workpad{
		Stamp:      "assay-example-stream-06@a1b2c3d4",
		Plan:       "- [ ] step one\n- [ ] step two",
		Acceptance: "- [ ] verify row 1 passes",
		Validation: "`go test ./...` → ok",
		Notes:      "blocked on nothing",
	}
	body := Render(w)

	got, ok := Parse(body)
	if !ok {
		t.Fatalf("Parse reported no marker on a body Render just produced:\n%s", body)
	}
	if got.Stamp != w.Stamp {
		t.Errorf("Stamp = %q, want %q", got.Stamp, w.Stamp)
	}
	if got.Plan != w.Plan {
		t.Errorf("Plan = %q, want %q", got.Plan, w.Plan)
	}
	if got.Acceptance != w.Acceptance {
		t.Errorf("Acceptance = %q, want %q", got.Acceptance, w.Acceptance)
	}
	if got.Validation != w.Validation {
		t.Errorf("Validation = %q, want %q", got.Validation, w.Validation)
	}
	if got.Notes != w.Notes {
		t.Errorf("Notes = %q, want %q", got.Notes, w.Notes)
	}
}

func TestWorkpadParseEmptySectionsRoundTripAsEmpty(t *testing.T) {
	body := Render(Workpad{Stamp: "w@a1b2c3d4"})
	got, ok := Parse(body)
	if !ok {
		t.Fatal("Parse reported no marker")
	}
	if got.Plan != "" || got.Acceptance != "" || got.Validation != "" || got.Notes != "" {
		t.Fatalf("an unfilled Render should round-trip to empty sections, got %+v", got)
	}
}

func TestWorkpadParseNoMarkerReturnsFalse(t *testing.T) {
	if _, ok := Parse("just an ordinary PR comment with no marker at all"); ok {
		t.Fatal("Parse reported a marker on a body that has none")
	}
}

// TestWorkpadMarkerRequiresExactLine pins the load-bearing property the doc comment on
// HasWorkpadMarker/Parse claims: the marker must be its OWN line. Prose that merely
// mentions the marker mid-sentence, or on a line carrying other text, must not read as a
// real workpad — otherwise a skill doc or a PR body that quotes the marker to EXPLAIN the
// feature would itself be (mis)parsed as one.
func TestWorkpadMarkerRequiresExactLine(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"marker alone on its own line", WorkpadMarker + "\nsome text", true},
		{"marker with surrounding whitespace on its line", "  " + WorkpadMarker + "  \nsome text", true},
		{"marker mid-sentence", "the workpad marker is " + WorkpadMarker + " — see the docs", false},
		{"marker mentioned in prose about the feature", "This PR adds `" + WorkpadMarker + "` as a new marker.", false},
		{"no marker at all", "nothing to see here", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasWorkpadMarker(c.body); got != c.want {
				t.Errorf("HasWorkpadMarker(%q) = %v, want %v", c.body, got, c.want)
			}
			if _, ok := Parse(c.body); ok != c.want {
				t.Errorf("Parse(%q) ok = %v, want %v", c.body, ok, c.want)
			}
		})
	}
}

// TestWorkpadStampHasNoPath is Verify row 5's pinned test: the stamp must never leak an
// absolute worktree path onto a public PR — only the worktree's BASENAME, joined to the
// short sha. Every case here would carry a "/" (or, on a Windows-style input, a "\") if
// Stamp forwarded anything but the basename.
func TestWorkpadStampHasNoPath(t *testing.T) {
	cases := []struct {
		worktree, sha string
	}{
		{"/private/tmp/assay-example-stream-06", "abc1234def"},
		{"/Users/operator/work/assay-example-stream-06/", "1111111111111111111111111111111111111111"},
		{"relative/path/worktree", "abc1234"},
		{"C:\\Users\\operator\\worktree", "abc1234"},
		{"", "abc1234"},
		{"/", "abc1234"},
	}
	for _, c := range cases {
		got := Stamp(c.worktree, c.sha)
		if strings.ContainsAny(got, "/\\") {
			t.Errorf("Stamp(%q, %q) = %q, contains a path separator", c.worktree, c.sha, got)
		}
		if got == "" {
			t.Errorf("Stamp(%q, %q) returned empty", c.worktree, c.sha)
		}
	}
}

func TestWorkpadStampFormatIsBasenameAtShortSHA(t *testing.T) {
	got := Stamp("/private/tmp/assay-example-stream-06", "0123456789abcdef")
	want := "assay-example-stream-06@01234567"
	if got != want {
		t.Errorf("Stamp = %q, want %q", got, want)
	}
}

func TestStripWorkpadMarkerLineBlanksOnlyTheExactLine(t *testing.T) {
	body := WorkpadMarker + "\nw@abc1234\n\n## Plan\n" + workpadEmptySection
	stripped := StripWorkpadMarkerLine(body)
	if strings.Contains(stripped, WorkpadMarker) {
		t.Fatalf("StripWorkpadMarkerLine left the marker in place:\n%s", stripped)
	}
	if !strings.Contains(stripped, "w@abc1234") || !strings.Contains(stripped, "## Plan") {
		t.Fatalf("StripWorkpadMarkerLine touched more than the marker line:\n%s", stripped)
	}
	// A body that merely MENTIONS the marker mid-line is untouched — only an exact-match
	// LINE is blanked, never a substring.
	mention := "quoting the marker `" + WorkpadMarker + "` here"
	if StripWorkpadMarkerLine(mention) != mention {
		t.Fatalf("StripWorkpadMarkerLine altered a mid-sentence mention, want it untouched")
	}
}
