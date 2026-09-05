package askassay

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func testStamp() Stamp {
	return Stamp{At: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), Ref: "deadbee"}
}

func wellFormed() Question {
	return Question{
		ID:    "test-question",
		Text:  "a well-formed question",
		Class: ClassBoard,
		Source: Source{
			Cmd:    "statusgen --root <root> --json",
			Probe:  "count of rows",
			Window: "the tree at <sha>",
			Limit:  "none on rows — the file set is walked whole",
		},
		Caveats: []string{CaveatBoardLies},
	}
}

// figureRe extracts the figure field from a rendered answer. The pane keys on
// this field, so the test asserts on the same thing the pane reads.
var figureRe = regexp.MustCompile(`figure=([^ ]+)`)

func figureOf(t *testing.T, a Answer) string {
	t.Helper()
	m := figureRe.FindStringSubmatch(a.Render())
	if m == nil {
		t.Fatalf("rendered answer has no figure field:\n%s", a.Render())
	}
	return m[1]
}

// TestUncomputableNumberNeverRendersAsZero is THE test of this brief. Every
// way a number can fail to exist must reach the same rendered token, and that
// token must not be a digit.
func TestUncomputableNumberNeverRendersAsZero(t *testing.T) {
	digits := regexp.MustCompile(`^-?[0-9]+$`)

	noSource := wellFormed()
	noSource.Source.Limit = "" // a source that cannot state its cap

	noProbe := wellFormed()
	noProbe.Source.Probe = ""

	cases := []struct {
		name string
		a    Answer
	}{
		{"probe did not run", Unavailable(wellFormed(), "the probe was rate-limited", testStamp())},
		{"no declared limit", Computed(noSource, 42, testStamp())},
		{"no declared probe", Computed(noProbe, 42, testStamp())},
		{"no as-of stamp", Computed(wellFormed(), 42, Stamp{})},
		{"stamp with no ref", Computed(wellFormed(), 42, Stamp{At: time.Now()})},
		{"undeclared question", Unanswerable("no-such-question", "how many widgets?", testStamp())},
		{"negative count", Computed(wellFormed(), -1, testStamp())},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.a.State() != CouldNotCheck {
				t.Fatalf("state = %q, want %q — a figure that cannot be computed must be could-not-check", c.a.State(), CouldNotCheck)
			}
			if _, ok := c.a.Value(); ok {
				t.Errorf("Value() reported a usable number for a could-not-check answer")
			}
			got := figureOf(t, c.a)
			if got != FigureField {
				t.Errorf("figure = %q, want %q", got, FigureField)
			}
			if digits.MatchString(got) {
				t.Errorf("figure rendered as the digit string %q — this is the exact falsehood the brief forbids", got)
			}
			if strings.TrimSpace(c.a.Reason()) == "" {
				t.Errorf("could-not-check with no reason: a could-not-check that does not say why is not a finding")
			}
		})
	}
}

// TestZeroIsRenderableWhenItWasActuallyMeasured is the other half. The rule is
// not "never render 0"; it is "never render 0 for a number you did not
// compute". A real measured zero must still get through, or the guard is
// useless in the direction that matters.
func TestZeroIsRenderableWhenItWasActuallyMeasured(t *testing.T) {
	a := Computed(wellFormed(), 0, testStamp())
	if a.State() != Checked {
		t.Fatalf("state = %q, want %q", a.State(), Checked)
	}
	v, ok := a.Value()
	if !ok || v != 0 {
		t.Fatalf("Value() = (%d, %v), want (0, true)", v, ok)
	}
	if got := figureOf(t, a); got != "0" {
		t.Errorf("figure = %q, want %q — a measured zero is an answer", got, "0")
	}
}

// TestRenderStatesProbeWindowAndLimit — no silent caps: the four declared
// fields are in every rendered line, including a could-not-check line.
func TestRenderStatesProbeWindowAndLimit(t *testing.T) {
	for _, a := range []Answer{
		Computed(wellFormed(), 7, testStamp()),
		Unavailable(wellFormed(), "throttled", testStamp()),
	} {
		out := a.Render()
		for _, want := range []string{"source=", "probe=", "window=", "limit=", "as-of=", "state="} {
			if !strings.Contains(out, want) {
				t.Errorf("rendered answer omits %s:\n%s", want, out)
			}
		}
		if !strings.Contains(out, testStamp().String()) {
			t.Errorf("rendered answer omits the as-of stamp:\n%s", out)
		}
	}
}

// TestRenderNeverEmitsARawPipe — these lines are pasted into markdown Evidence
// tables, and a raw pipe shreds the table it lands in.
func TestRenderNeverEmitsARawPipe(t *testing.T) {
	for _, q := range Questions() {
		for _, a := range []Answer{
			Computed(q, 3, testStamp()),
			Unavailable(q, "probe unavailable", testStamp()),
		} {
			if strings.Contains(a.Render(), "|") {
				t.Errorf("%s: rendered answer contains a raw pipe, which shreds any markdown table it is pasted into:\n%s", q.ID, a.Render())
			}
		}
	}
}

// TestSaturatedCountIsRefused — a list read at its cap is not a count. This is
// the 500-against-958 failure, made unrepresentable.
func TestSaturatedCountIsRefused(t *testing.T) {
	q, ok := Lookup("open-issue-list")
	if !ok {
		t.Fatal("open-issue-list is not declared")
	}
	if q.SaturatesAt == 0 {
		t.Fatal("open-issue-list declares no saturation cap, so a truncated read would render as a total")
	}

	under := Computed(q, q.SaturatesAt-1, testStamp())
	if under.State() != Checked {
		t.Errorf("a count below the cap must be answerable, got %q", under.State())
	}

	for _, v := range []int64{q.SaturatesAt, q.SaturatesAt + 1} {
		at := Computed(q, v, testStamp())
		if at.State() != CouldNotCheck {
			t.Errorf("count %d at/over cap %d: state = %q, want %q", v, q.SaturatesAt, at.State(), CouldNotCheck)
		}
		if got := figureOf(t, at); got != FigureField {
			t.Errorf("count %d at/over cap: figure = %q, want %q", v, got, FigureField)
		}
		if !strings.Contains(at.Reason(), "cap") {
			t.Errorf("saturation refusal does not name the cap: %q", at.Reason())
		}
	}
}

// TestFailedCarriesItsNumber — checked-failed is a real measurement of a bad
// state, not a synonym for could-not-check.
func TestFailedCarriesItsNumber(t *testing.T) {
	a := Failed(wellFormed(), 4, "four checks are red", testStamp())
	if a.State() != CheckedFailed {
		t.Fatalf("state = %q, want %q", a.State(), CheckedFailed)
	}
	v, ok := a.Value()
	if !ok || v != 4 {
		t.Fatalf("Value() = (%d, %v), want (4, true)", v, ok)
	}
	// ...but a Failed built on an invalid source still degrades, rather than
	// smuggling a number in through the other door.
	bad := wellFormed()
	bad.Source.Cmd = ""
	if got := Failed(bad, 4, "red", testStamp()); got.State() != CouldNotCheck {
		t.Errorf("Failed with an undeclared source: state = %q, want %q", got.State(), CouldNotCheck)
	}
}

// TestClockOnlyStampNamesWhyThereIsNoSHA — "no SHA" must be a statement.
func TestClockOnlyStampNamesWhyThereIsNoSHA(t *testing.T) {
	s := ClockOnly(time.Now().UTC(), "live server-side count, no tree behind it")
	if s.Zero() {
		t.Fatal("a clock-only stamp must still be a usable stamp")
	}
	if !strings.Contains(s.String(), "clock-only:") {
		t.Errorf("clock-only stamp does not declare itself: %q", s.String())
	}
	if got := ClockOnly(time.Now().UTC(), "  "); !strings.Contains(got.String(), "unstated") {
		t.Errorf("a clock-only stamp with no reason must say so, got %q", got.String())
	}
}
