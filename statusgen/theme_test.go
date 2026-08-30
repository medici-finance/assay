package main

import (
	"strings"
	"testing"
	"time"
)

// Unmapped-theme rule (statusgen/13): a stream README `theme:` value with no
// mapped render style renders as a VISIBLE marker rather than being dropped;
// absent or mapped themes contribute no marker.

func themeStream(name, theme string) *Stream {
	s := mkStream(name, "active", "P1")
	s.Theme = theme
	return s
}

func TestThemeUnmappedMarkersComputed(t *testing.T) {
	streams := []*Stream{
		themeStream("s1", "nonesuch"),  // unmapped → marker
		themeStream("s2", "highlight"), // mapped → no marker
		themeStream("s3", ""),          // absent → no marker
		themeStream("s4", "default"),   // mapped → no marker
		themeStream("s5", "another-unmapped"),
	}
	got := unmappedThemes(streams)
	if len(got) != 2 {
		t.Fatalf("unmappedThemes = %v, want 2 markers (s1, s5)", got)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "s1: unmapped theme: nonesuch") {
		t.Errorf("missing marker for s1: %v", got)
	}
	if !strings.Contains(joined, "s5: unmapped theme: another-unmapped") {
		t.Errorf("missing marker for s5: %v", got)
	}
	if strings.Contains(joined, "highlight") || strings.Contains(joined, "default") {
		t.Errorf("mapped themes must not produce a marker: %v", got)
	}
}

func TestThemeUnmappedRendersVisibly(t *testing.T) {
	// With a cadence overlay carrying an unmapped-theme marker, the rendered deck
	// must contain a VISIBLE "unmapped theme: <value>" string.
	saved := activeCadence
	defer func() { activeCadence = saved }()
	activeCadence = &cadenceView{
		Cadence: "weekly",
		Label:   "2026-W34",
		Start:   time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Brand:   neutralBrand(),
		Themes:  []string{"s1: unmapped theme: nonesuch"},
	}
	now := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	html := renderRoadmap(nil, nil, nil, nil, false, "", NextUp{}, LadderReport{}, nil, "abc1234", now)
	if !strings.Contains(html, "unmapped theme: nonesuch") {
		t.Fatalf("rendered deck must surface the unmapped-theme marker visibly; not found")
	}
	if !strings.Contains(html, "Reporting Window") {
		t.Errorf("cadence deck must render the Reporting Window section")
	}
}

func TestThemeMappedNoMarkerInRender(t *testing.T) {
	// A cadence overlay with no unmapped markers must NOT emit the Theme Markers
	// section (a mapped/absent theme is silent).
	saved := activeCadence
	defer func() { activeCadence = saved }()
	activeCadence = &cadenceView{
		Cadence: "monthly",
		Label:   "2026-07",
		Start:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Brand:   neutralBrand(),
		Themes:  nil,
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	html := renderRoadmap(nil, nil, nil, nil, false, "", NextUp{}, LadderReport{}, nil, "abc1234", now)
	if strings.Contains(html, "unmapped theme:") {
		t.Fatalf("no unmapped themes → no marker text expected")
	}
	if strings.Contains(html, "Theme Markers") {
		t.Errorf("no unmapped themes → Theme Markers section must be absent")
	}
}
