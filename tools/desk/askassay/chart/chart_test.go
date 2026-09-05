package chart

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

func stamp() askassay.Stamp {
	return askassay.Stamp{At: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), Ref: "deadbeef"}
}

// declared is a question with a complete source, so Computed will attach a
// value to it.
func declared(id string) askassay.Question {
	return askassay.Question{
		ID:    id,
		Text:  "how many " + id + "?",
		Class: askassay.ClassBoard,
		Source: askassay.Source{
			Cmd:    "statusgen --root <root> --json",
			Probe:  "the " + id + " field of the emitted object",
			Window: "the working tree at <sha>",
			Limit:  "none on rows — the emitted object is read whole",
		},
	}
}

func measured(id string, v int64) askassay.Answer {
	return askassay.Computed(declared(id), v, stamp())
}

func unmeasured(id, why string) askassay.Answer {
	return askassay.Unavailable(declared(id), why, stamp())
}

func baseChart() Chart {
	return Chart{
		Title: "Rows by status",
		AsOf:  stamp(),
		Series: Series{
			Name: "brief rows",
			Unit: "rows",
			Points: []Point{
				{Label: "todo", Answer: measured("todo", 40)},
				{Label: "implemented", Answer: measured("implemented", 12)},
				{Label: "verified", Answer: unmeasured("verified", "the corroboration probe did not run in this pass")},
				{Label: "done", Answer: measured("done", 8)},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Determinism
// ---------------------------------------------------------------------------

// TestRenderIsByteIdenticalAcrossCalls is the chart half of the brief's
// determinism claim. Two renders of the same chart must be the same bytes;
// the classic breakers (a clock read, a ranged map) would show up here.
func TestRenderIsByteIdenticalAcrossCalls(t *testing.T) {
	c := baseChart()
	var prev string
	for i := 0; i < 8; i++ {
		got, err := c.Render()
		if err != nil {
			t.Fatalf("render %d: %v", i, err)
		}
		if i > 0 && got != prev {
			t.Fatalf("render %d differs from render %d — the render is not deterministic", i, i-1)
		}
		prev = got
	}
	if !strings.Contains(prev, "as-of "+stamp().String()) {
		t.Error("the render does not carry its as-of stamp")
	}
}

// TestRenderReadsNoClock reads this package's own source. A chart that
// consults the clock is not a function of the index.
func TestRenderReadsNoClock(t *testing.T) {
	for name, src := range packageSources(t) {
		for _, forbidden := range []string{"time.Now", "rand.", "os.Getenv"} {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s calls %s — the render must be a pure function of the chart", name, forbidden)
			}
		}
	}
}

// TestRenderRangesNoMap: ranging a map is the other classic source of a
// non-deterministic render, and it fails intermittently rather than always,
// which is why a source scan is worth more here than a repeated render.
func TestRenderRangesNoMap(t *testing.T) {
	rangeOverMap := regexp.MustCompile(`range\s+\w*[Mm]ap\b`)
	for name, src := range packageSources(t) {
		if rangeOverMap.MatchString(src) {
			t.Errorf("%s ranges a map", name)
		}
		if strings.Contains(src, "map[") && strings.Contains(src, "for ") && name != "chart_test.go" {
			// Not a failure on its own — esc() builds a replacer, not a map
			// iteration — but worth the explicit note that no map declared in
			// this package is iterated.
			t.Logf("%s declares a map type; TestRenderIsByteIdenticalAcrossCalls covers the iteration risk", name)
		}
	}
}

// ---------------------------------------------------------------------------
// The could-not-check render — the property this package exists to hold
// ---------------------------------------------------------------------------

// TestCouldNotCheckDrawsNoMagnitude asserts the headline rule: an
// unmeasurable slot draws a hatched band carrying the literal token, and NOT
// a bar of any derived height.
func TestCouldNotCheckDrawsNoMagnitude(t *testing.T) {
	svg, err := baseChart().Render()
	if err != nil {
		t.Fatal(err)
	}
	slot := extractGroup(t, svg, `class="slot could-not-check"`)
	if !strings.Contains(slot, CouldNotCheckToken) {
		t.Errorf("the could-not-check slot does not print the literal token %q:\n%s", CouldNotCheckToken, slot)
	}
	if !strings.Contains(slot, `url(#cnc-hatch)`) {
		t.Errorf("the could-not-check slot is not hatched — a solid or absent slot is a magnitude:\n%s", slot)
	}
	if !strings.Contains(slot, ">?<") {
		t.Error("the could-not-check slot carries no glyph — §7.6 forbids colour-only encoding")
	}
	if strings.Contains(slot, `height="0"`) {
		t.Errorf("the could-not-check slot drew a zero-height rect, which is a drawn claim that the value is 0:\n%s", slot)
	}
	if !strings.Contains(slot, "<title>") {
		t.Error("the could-not-check slot has no tooltip, so its reason is unreachable")
	}
	if !strings.Contains(slot, "the corroboration probe did not run") {
		t.Error("the tooltip does not carry the reason")
	}
}

// TestZeroAndCouldNotCheckDrawDifferently is the positive control for the
// rule above. A genuinely measured 0 must still render as 0 — the guarantee
// is not "never draw a zero", it is "never draw a zero you did not measure".
func TestZeroAndCouldNotCheckDrawDifferently(t *testing.T) {
	c := baseChart()
	c.Series.Points = []Point{
		{Label: "measured-zero", Answer: measured("measured-zero", 0)},
		{Label: "unmeasurable", Answer: unmeasured("unmeasurable", "the probe was refused by the read-only guard")},
		{Label: "positive", Answer: measured("positive", 5)},
	}
	svg, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	zero := extractGroup(t, svg, `class="slot checked"`)
	if !strings.Contains(zero, ">0<") {
		t.Errorf("a measured zero did not print the numeral 0 — the numbers rule must not swallow real zeroes:\n%s", zero)
	}
	if strings.Contains(zero, CouldNotCheckToken) {
		t.Error("a measured zero rendered as could-not-check")
	}
	cnc := extractGroup(t, svg, `class="slot could-not-check"`)
	if strings.Contains(cnc, ">0<") {
		t.Errorf("the could-not-check slot printed a 0:\n%s", cnc)
	}
	if zero == cnc {
		t.Error("a measured zero and a could-not-check render identically")
	}
}

// TestCouldNotCheckIsOutOfDomain: an unmeasurable slot must not move the axis
// the measured slots are read against.
func TestCouldNotCheckIsOutOfDomain(t *testing.T) {
	with := baseChart()
	without := baseChart()
	without.Series.Points = []Point{
		without.Series.Points[0], without.Series.Points[1], without.Series.Points[3],
	}
	maxWith, nWith := with.domain()
	maxWithout, nWithout := without.domain()
	if maxWith != maxWithout {
		t.Errorf("the axis maximum changed when an unmeasurable slot was removed (%d vs %d) — an unmeasured point is participating in the scale", maxWith, maxWithout)
	}
	// The maximum alone is a WEAK assertion, and a mutation run proved it: an
	// unmeasurable point folded into the domain as 0 never raises a maximum,
	// so the mutant survived its own test and was killed by someone else's.
	// The count of measured points is what actually distinguishes the arms.
	if nWith != nWithout {
		t.Errorf("the domain counted %d measured points with the unmeasurable slot present and %d without it — an unmeasured point is being counted as measured", nWith, nWithout)
	}
	if nWith != 3 {
		t.Errorf("the domain reports %d measured points; the fixture has exactly 3", nWith)
	}
}

// TestAllUnmeasurableRendersPanelNotAxis: with nothing measurable, the chart
// must not draw an axis. A plot frame with a baseline and no bars is a drawn
// row of zeroes.
func TestAllUnmeasurableRendersPanelNotAxis(t *testing.T) {
	c := baseChart()
	for i := range c.Series.Points {
		c.Series.Points[i].Answer = unmeasured(c.Series.Points[i].Label, "the token was rate-limited: an empty result is BLIND, not idle")
	}
	svg, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(svg, `class="axis"`) {
		t.Error("an all-unmeasurable chart drew an axis, and its baseline reads as a zero line")
	}
	// Positive control for the assertion above: the same assertion must FIND
	// an axis on a chart that does have measured values, or it would pass for
	// the wrong reason.
	ok, err := baseChart().Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ok, `class="axis"`) {
		t.Fatal("the axis marker is absent from a measurable chart too — this test would pass vacuously")
	}
	if !strings.Contains(svg, `class="chart-could-not-check"`) {
		t.Error("an all-unmeasurable chart did not render the could-not-check panel")
	}
	if !strings.Contains(svg, "BLIND, not idle") {
		t.Error("the panel does not carry the reason")
	}
}

// TestUnreachableSourceRendersCouldNotCheck is the required RED RUN against a
// positive control: a source that genuinely cannot be reached, proven to
// render could-not-check rather than an empty chart.
//
// The unreachability is real, not simulated: PATH is emptied so a permitted
// binary cannot be executed, the real subprocess path runs, and the real
// classifier decides. No seam is stubbed.
func TestUnreachableSourceRendersCouldNotCheck(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	q := declared("open-issue-count")
	q.Source.Cmd = "statusgen --root <root> --json"
	out, runErr := askassay.Runner{Dir: empty, Timeout: 10 * time.Second}.
		Run(context.Background(), []string{"statusgen", "--root", ".", "--json"})
	if runErr == nil {
		t.Fatalf("the positive control did not fail: the probe reached something with PATH=%s (output %d bytes). This test is only meaningful if the source is genuinely unreachable", empty, len(out))
	}
	t.Logf("positive control: the probe genuinely failed with %v", runErr)

	state, reason := askassay.Classify(q, out, runErr)
	if state != askassay.CouldNotCheck {
		t.Fatalf("an unreachable source classified as %s, not %s", state, askassay.CouldNotCheck)
	}
	ans := askassay.Unavailable(q, reason, stamp())

	c := baseChart()
	c.Series.Points = []Point{
		{Label: "open issues", Answer: ans},
		{Label: "known", Answer: measured("known", 7)},
	}
	svg, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	slot := extractGroup(t, svg, `class="slot could-not-check"`)
	if !strings.Contains(slot, CouldNotCheckToken) {
		t.Error("an unreachable source did not render the could-not-check token")
	}
	if strings.Contains(slot, ">0<") {
		t.Error("an unreachable source rendered a 0")
	}
	table := c.Table()
	if !strings.Contains(table, CouldNotCheckToken) {
		t.Error("the table view lost the could-not-check state")
	}
	if strings.Contains(table, "| 0 |") {
		t.Errorf("the table view rendered an unreachable source as 0:\n%s", table)
	}
}

// ---------------------------------------------------------------------------
// §7.6 colour and accessibility rules
// ---------------------------------------------------------------------------

// TestTheCVDFailingHuesAreNotCopied is the brief's own Verify item 3, run
// against this package rather than a directory that does not yet exist.
func TestTheCVDFailingHuesAreNotCopied(t *testing.T) {
	banned := []string{"#3366FF", "#3366ff", "#7a5cff", "#7A5CFF"}
	for name, src := range packageSources(t) {
		for _, b := range banned {
			if strings.Contains(src, b) {
				t.Errorf("%s contains %s — the CVD-failing lifecycle bar is being copied", name, b)
			}
		}
	}
	for i, c := range Ramp {
		for _, b := range banned {
			if strings.EqualFold(c, b) {
				t.Errorf("ramp step %d is %s", i, b)
			}
		}
	}
}

// TestPaletteMutations is the mutation test over ValidatePalette. Every arm
// must redden, and each mutant uses a DISTINCT fixture so two mutants cannot
// be covered by the same failure — the trap that let a mutation pass silently.
func TestPaletteMutations(t *testing.T) {
	if err := ValidatePalette(Ramp, GateGold); err != nil {
		t.Fatalf("the shipped palette does not validate: %v", err)
	}
	cases := []struct {
		name string
		ramp []string
		gold string
		want string
	}{
		{"gold used as a series colour",
			[]string{"#0e2233", "#173a56", GateGold}, GateGold, "reserved gate gold"},
		{"a second hue smuggled in",
			[]string{"#0e2233", "#173a56", "#7a1f1f"}, GateGold, "off the base hue"},
		{"two steps at the same lightness",
			[]string{"#0e2233", "#0e2334", "#22557c"}, GateGold, "not meaningfully above"},
		{"lightness runs backwards",
			[]string{"#4f97cb", "#22557c", "#0e2233"}, GateGold, "not meaningfully above"},
		{"a one-step ramp",
			[]string{"#22557c"}, GateGold, "at least two steps"},
		{"a malformed colour",
			[]string{"#0e2233", "not-a-colour"}, GateGold, "hex triplet"},
	}
	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePalette(tc.ramp, tc.gold)
			if err == nil {
				t.Fatalf("the mutant did not redden: ValidatePalette accepted %v", tc.ramp)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("the mutant reddened for the wrong reason: got %q, wanted a message containing %q", err, tc.want)
			}
			// Distinct fixtures: if two mutants produce the same error from
			// the same input, one of them is not being tested.
			key := strings.Join(tc.ramp, ",")
			if prev, dup := seen[key]; dup {
				t.Fatalf("this mutant shares a fixture with %q — one of them is not exercising its own arm", prev)
			}
			seen[key] = tc.name
		})
	}
}

// TestTextTokensPassContrast: §7.6 sets body text at >= 4.5:1 and restricts
// the dimmer token to decorative use.
func TestTextTokensPassContrast(t *testing.T) {
	for _, tc := range []struct {
		name, fg string
		min      float64
	}{
		{"InkText", InkText, 4.5},
		{"InkDim", InkDim, 4.5},
	} {
		r, err := ContrastRatio(tc.fg, Surface)
		if err != nil {
			t.Fatal(err)
		}
		if r < tc.min {
			t.Errorf("%s is %.2f:1 on the surface, below the %.1f:1 floor", tc.name, r, tc.min)
		}
		t.Logf("%s: %.2f:1 on %s", tc.name, r, Surface)
	}
}

// TestValuesWearTextTokens: §7.6 — values wear text tokens, never the series
// colour. A number encoded in the colour of the thing it labels is a number
// a colour-blind reader cannot bind to its bar.
func TestValuesWearTextTokens(t *testing.T) {
	svg, err := baseChart().Render()
	if err != nil {
		t.Fatal(err)
	}
	textFill := regexp.MustCompile(`<text[^>]*fill="([^"]+)"`)
	for _, m := range textFill.FindAllStringSubmatch(svg, -1) {
		fill := m[1]
		for _, c := range Ramp {
			if strings.EqualFold(fill, c) {
				t.Errorf("a <text> element is filled with the series colour %s", c)
			}
		}
		if fill != InkText && fill != InkDim {
			t.Errorf("a <text> element uses %s, which is not a text token", fill)
		}
	}
}

// TestGoldIsReservedAndAlwaysGlyphed: gold appears only on the gate marker,
// and never without its glyph and a word.
func TestGoldIsReservedAndAlwaysGlyphed(t *testing.T) {
	plain, err := baseChart().Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, GateGold) {
		t.Error("gold appears on a chart with no human gate")
	}

	c := baseChart()
	c.Gate = &GateMarker{AtLabel: "verified", Label: "decide"}
	svg, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, GateGold) {
		t.Fatal("the gate marker did not draw")
	}
	if !strings.Contains(svg, GateGlyph+" decide") {
		t.Error("gold rendered without its glyph and word")
	}
	if !strings.Contains(c.Table(), GateGlyph) {
		t.Error("the table view drops the gate glyph, so the gate is colour-only there")
	}

	c.Gate = &GateMarker{AtLabel: "verified", Label: ""}
	if _, err := c.Render(); err == nil {
		t.Error("a gate marker with no word was accepted — gold never ships without its label")
	}
	c.Gate = &GateMarker{AtLabel: "no-such-slot", Label: "decide"}
	if _, err := c.Render(); err == nil {
		t.Error("a gate marker pointing at a slot that is not on the chart was accepted")
	}
}

// TestTypeFloor: §7.6 sets an 11px floor for anything that must be read, and
// the wireframes' 8.5-9.5px chips violate it.
func TestTypeFloor(t *testing.T) {
	svg, err := baseChart().Render()
	if err != nil {
		t.Fatal(err)
	}
	sizes := regexp.MustCompile(`font-size="(\d+)"`)
	found := 0
	for _, m := range sizes.FindAllStringSubmatch(svg, -1) {
		found++
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		if n < MinTypePx {
			t.Errorf("font-size %d is below the %dpx floor", n, MinTypePx)
		}
	}
	if found == 0 {
		t.Fatal("no font-size found — this scan would pass vacuously")
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

// TestChartRefusals is the mutation test over the render's fail-closed arms.
// Each mutant is a distinct field, and each must redden with its own message.
func TestChartRefusals(t *testing.T) {
	if _, err := baseChart().Render(); err != nil {
		t.Fatalf("the baseline chart does not render: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*Chart)
		want string
	}{
		{"no title", func(c *Chart) { c.Title = "" }, "no title"},
		{"no series name", func(c *Chart) { c.Series.Name = "" }, "no name"},
		{"no unit", func(c *Chart) { c.Series.Unit = "" }, "no unit"},
		{"no points", func(c *Chart) { c.Series.Points = nil }, "no points"},
		{"no as-of stamp", func(c *Chart) { c.AsOf = askassay.Stamp{} }, "no as-of stamp"},
		{"an unlabelled point", func(c *Chart) { c.Series.Points[1].Label = "  " }, "has no label"},
	}
	msgs := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := baseChart()
			tc.mut(&c)
			out, err := c.Render()
			if err == nil {
				t.Fatalf("the mutant did not redden: it rendered %d bytes", len(out))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("reddened for the wrong reason: %v (wanted %q)", err, tc.want)
			}
			if prev, dup := msgs[err.Error()]; dup {
				t.Fatalf("this mutant produces the same error as %q — one of the two arms is not covered", prev)
			}
			msgs[err.Error()] = tc.name
		})
	}
}

// TestTableStatesEveryLimit: no silent caps. Every slot's source, probe,
// window and limit are on the table view, and an undeclared one says so
// rather than rendering blank.
func TestTableStatesEveryLimit(t *testing.T) {
	table := baseChart().Table()
	for _, want := range []string{"| Slot |", "Limit", "none on rows", "statusgen --root <root> --json"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table view omits %q:\n%s", want, table)
		}
	}
	// A slot whose source is undeclared must say UNDECLARED, not blank.
	c := baseChart()
	c.Series.Points[0].Answer = askassay.Unanswerable("mystery", "how many?", stamp())
	if !strings.Contains(c.Table(), "UNDECLARED") {
		t.Error("an undeclared source rendered as a blank cell rather than UNDECLARED")
	}
	if strings.Contains(c.Table(), "||") {
		t.Error("the table has an empty cell where a source belongs")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func extractGroup(t *testing.T, svg, marker string) string {
	t.Helper()
	i := strings.Index(svg, marker)
	if i < 0 {
		t.Fatalf("no group matching %s in:\n%s", marker, svg)
	}
	start := strings.LastIndex(svg[:i], "<g")
	end := strings.Index(svg[i:], "</g>")
	if start < 0 || end < 0 {
		t.Fatalf("malformed group around %s", marker)
	}
	return svg[start : i+end+4]
}

func packageSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		out[n] = stripComments(string(b))
	}
	if len(out) == 0 {
		t.Fatal("no non-test source found — this scan would pass vacuously")
	}
	return out
}

// stripComments removes // and /* */ comments. The source scans above have to
// read code and not prose, because this package's own headers name the very
// tokens they forbid — a scan that counted comments would fail on its own
// documentation, and the obvious "fix" for that is to stop documenting.
func stripComments(src string) string {
	var b strings.Builder
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], "//") {
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				break
			}
			i += j
			continue
		}
		if strings.HasPrefix(src[i:], "/*") {
			j := strings.Index(src[i:], "*/")
			if j < 0 {
				break
			}
			i += j + 2
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
