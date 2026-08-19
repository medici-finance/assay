package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- test scaffolding ---------------------------------------------------------

// withDrives sets the activeDriveSet package var for the duration of a test and
// restores it afterward (mirrors withSpan). The zero value is the inert default.
func withDrives(t *testing.T, ds DriveSet) {
	t.Helper()
	old := activeDriveSet
	activeDriveSet = ds
	t.Cleanup(func() { activeDriveSet = old })
}

// driveTestNow is a fixed board-day used across the drive tests so the wall-clock
// window is deterministic.
var driveTestNow = time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)

// writeDrive writes a manifest YAML under root/docs/roadmap/drives/<slug>.yaml and
// returns root. Repeated calls with the same root add sibling manifests.
func writeDrive(t *testing.T, root, slug, body string) string {
	t.Helper()
	dir := filepath.Join(root, "docs", "roadmap", "drives")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// liveWindow is a starts/expires pair straddling driveTestNow, inside the horizon.
const liveWindow = "starts: \"2026-08-14\"\nexpires: \"2026-08-20\"\n"

func driveBriefStream(name string, nBriefs int) *Stream {
	var briefs []Brief
	for i := 0; i < nBriefs; i++ {
		briefs = append(briefs, Brief{Num: pad2(i + 1), Wave: 0, Status: "todo"})
	}
	s := mkStream(name, "active", "P1", briefs...)
	s.LastTouch = day(0)
	return s
}

func pad2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// --- Verify row 1 -------------------------------------------------------------

// TestDriveAbsentIsInert (brief-44 Verify row 1): with NO manifest under
// docs/roadmap/drives/, the loaded DriveSet is inert and the generated board is
// BYTE-IDENTICAL to the no-drive baseline — zero score-input change.
func TestDriveAbsentIsInert(t *testing.T) {
	hot := driveBriefStream("hot", 3)
	cool := driveBriefStream("cool", 2)
	streams := []*Stream{hot, cool}

	// Baseline: the pre-drives generator (activeDriveSet forced to the zero value).
	withDrives(t, DriveSet{})
	baseNu := nextUp(streams, ClaimView{}, nil)
	baseline := emit(streams, nil, baseNu, nil, nil, IntakeAlarmResult{}, nil, "")

	// Now exercise the REAL loader against a root that has docs/streams but NO
	// docs/roadmap/drives directory — the absent-manifest path.
	root := t.TempDir()
	makeStreamsDir(t, root)
	ds := loadDrives(root, streams, driveTestNow)
	if ds.applied() {
		t.Fatalf("absent manifest must yield no active drive, got %d", len(ds.Active))
	}
	if ds.NotApplied || len(ds.Warnings) != 0 {
		t.Fatalf("absent manifest must be SILENT (no NotApplied, no warnings): NotApplied=%v warnings=%v", ds.NotApplied, ds.Warnings)
	}
	withDrives(t, ds)
	gotNu := nextUp(streams, ClaimView{}, nil)
	got := emit(streams, nil, gotNu, nil, nil, IntakeAlarmResult{}, nil, "")

	if got != baseline {
		t.Fatalf("absent-manifest board is NOT byte-identical to the no-drive baseline.\n--- baseline ---\n%s\n--- with-loader ---\n%s", baseline, got)
	}
	// And no pick carries a drive term.
	for _, p := range gotNu.Picks {
		if p.DriveTerm != 0 || p.DriveSlug != "" {
			t.Errorf("pick %s/%s carries a drive term (%d, %q) with no manifest", p.Stream.Name, p.Brief.Num, p.DriveTerm, p.DriveSlug)
		}
	}

	// An EMPTY drives directory (exists, no manifests) is equally inert.
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, "docs", "roadmap", "drives"), 0o755); err != nil {
		t.Fatal(err)
	}
	ds2 := loadDrives(root2, streams, driveTestNow)
	if ds2.applied() || ds2.NotApplied || len(ds2.Warnings) != 0 {
		t.Fatalf("empty drives dir must be inert: %+v", ds2)
	}
	withDrives(t, ds2)
	got2 := emit(streams, nil, nextUp(streams, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	if got2 != baseline {
		t.Fatalf("empty-drives-dir board is not byte-identical to baseline")
	}
}

// --- Verify row 2 -------------------------------------------------------------

// TestDriveMalformedFailNeutral (brief-44 Verify row 2): a malformed /
// unresolvable / expired / over-concurrent manifest yields ZERO boost, the board
// STILL generates, a DRIVE NOT APPLIED banner plus a WARN is emitted — and it is
// NEVER an rc≠0 PROBLEM. The loader can only return a DriveSet, never an error.
func TestDriveMalformedFailNeutral(t *testing.T) {
	streams := []*Stream{driveBriefStream("hot", 3)}

	cases := []struct {
		name string
		body string
	}{
		{"broken-yaml", "declared-by: ian\nintensity: [not, a, scalar\n:::"},
		{"bad-intensity", "declared-by: ian\n" + liveWindow + "intensity: ludicrous\nstate: active\n"},
		{"missing-required", "intensity: focus\nstate: active\n"},
		{"expired", "declared-by: ian\nstarts: \"2026-07-01\"\nexpires: \"2026-07-10\"\nintensity: push\nstate: active\n"},
		{"over-horizon", "declared-by: ian\nstarts: \"2026-08-14\"\nexpires: \"2026-09-30\"\nintensity: push\nstate: active\n"},
		{"bare-issue-ref", "declared-by: ian\n" + liveWindow + "intensity: focus\nstate: active\nitems:\n  - issue: \"#42\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			makeStreamsDir(t, root)
			writeDrive(t, root, "campaign", tc.body)

			// The loader NEVER errors — that is the whole safety property.
			ds := loadDrives(root, streams, driveTestNow)
			if ds.applied() {
				t.Fatalf("%s: a rejected manifest must apply ZERO boost, got %d active", tc.name, len(ds.Active))
			}
			if !ds.NotApplied {
				t.Fatalf("%s: a rejected manifest must set NotApplied for the banner", tc.name)
			}
			if len(ds.Warnings) == 0 {
				t.Fatalf("%s: a rejected manifest must emit a WARN line", tc.name)
			}
			for _, wn := range ds.Warnings {
				if !strings.Contains(wn, "DRIVE NOT APPLIED (WARN)") {
					t.Errorf("%s: warning is not the fail-neutral shape: %q", tc.name, wn)
				}
				if strings.Contains(wn, "PROBLEM") {
					t.Errorf("%s: fail-neutral warning must NEVER be a PROBLEM: %q", tc.name, wn)
				}
			}

			withDrives(t, ds)
			nu := nextUp(streams, ClaimView{}, nil)
			// Zero boost: every pick scores its base, exactly as with no drive.
			for _, p := range nu.Picks {
				if p.DriveTerm != 0 {
					t.Errorf("%s: pick %s carries boost %d — a rejected manifest must not steer the board", tc.name, p.Brief.Num, p.DriveTerm)
				}
			}
			// The board STILL generates and shows the DRIVE NOT APPLIED banner.
			if nu.DriveNotApplied == "" {
				t.Fatalf("%s: NextUp must carry the DRIVE NOT APPLIED banner", tc.name)
			}
			out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
			if !strings.Contains(out, "DRIVE NOT APPLIED") {
				t.Fatalf("%s: STATUS board missing the DRIVE NOT APPLIED banner:\n%s", tc.name, out)
			}
			// The only drive PROBLEM path (banner honesty) must NOT fire: there is
			// no boosted pick to gate, so a malformed manifest is never rc≠0.
			if p := driveBannerProblems(nu); len(p) != 0 {
				t.Fatalf("%s: fail-neutral manifest produced a PROBLEM (rc≠0): %v", tc.name, p)
			}
		})
	}

	// Over-concurrency (>2 active) is fail-neutral too: apply none.
	root := t.TempDir()
	makeStreamsDir(t, root)
	for _, slug := range []string{"a", "b", "c"} {
		writeDrive(t, root, slug, "declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nitems:\n  - stream: hot\n")
	}
	ds := loadDrives(root, streams, driveTestNow)
	if ds.applied() {
		t.Fatalf("3 active drives exceed the ≤2 cap — must apply zero, got %d active", len(ds.Active))
	}
	if !ds.NotApplied || len(ds.Warnings) == 0 {
		t.Fatalf("over-concurrency must be a fail-neutral WARN + banner: %+v", ds)
	}
}

// --- Verify row 3 -------------------------------------------------------------

// TestDriveBoostedPickRequiresBanner (brief-44 Verify row 3): a boosted Next-up
// pick emitted WITHOUT the active-drive banner is a PROBLEM; the decomposed
// display (base + named drive term) is asserted, never a merged number.
func TestDriveBoostedPickRequiresBanner(t *testing.T) {
	// filler keeps the drive's coverage of the eligible board under the anti-
	// Goodhart tax threshold, so the term stands at its full push weight.
	streams := []*Stream{driveBriefStream("hot", 3), driveBriefStream("filler", 5)}

	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "focus-hot", "declared-by: ian\n"+liveWindow+"intensity: push\nwhy: ship it\nstate: active\nitems:\n  - stream: hot\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("a valid live manifest must apply: %+v", ds)
	}
	withDrives(t, ds)

	nu := nextUp(streams, ClaimView{}, nil)
	boosted := false
	for _, p := range nu.Picks {
		if p.DriveTerm == drivePushWeight && p.DriveSlug == "focus-hot" {
			boosted = true
		}
	}
	if !boosted {
		t.Fatalf("hot picks must carry the +%d push term attributed to focus-hot: %+v", drivePushWeight, nu.Picks)
	}
	// A correct board carries the banner, and driveBannerProblems is silent.
	if nu.DriveBanner == "" {
		t.Fatal("a board with boosted picks MUST set the active-drive banner")
	}
	if p := driveBannerProblems(nu); len(p) != 0 {
		t.Fatalf("a banner-bearing board must not be a PROBLEM: %v", p)
	}
	// The rendered score is DECOMPOSED: `base + term (drive:slug)`, never merged.
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	wantScore := "2000 + 1500 (drive:focus-hot)"
	if !strings.Contains(out, wantScore) {
		t.Fatalf("Next-up score must be decomposed as %q, not merged:\n%s", wantScore, out)
	}
	if strings.Contains(out, "| 3500 |") {
		t.Fatalf("score must never be shown as a merged number (3500):\n%s", out)
	}
	if !strings.Contains(out, "ACTIVE DRIVE") {
		t.Fatalf("board with a boost must render the ACTIVE DRIVE banner:\n%s", out)
	}

	// The gate itself: a boosted pick with the banner STRIPPED is a PROBLEM.
	stripped := nu
	stripped.DriveBanner = ""
	probs := driveBannerProblems(stripped)
	if len(probs) == 0 {
		t.Fatal("a boosted pick shown WITHOUT the active-drive banner must be a PROBLEM")
	}
	if !strings.Contains(probs[0], "without the active-drive banner") {
		t.Errorf("problem must name the honesty violation: %q", probs[0])
	}
}

// --- Verify row 4 -------------------------------------------------------------

// TestDriveTermExcludedFromMetrics (brief-44 Verify row 4): the drive term is
// absent from the gate-score and from every exported metric surface — it re-ranks
// the Next-up board only. A steer, not a value claim.
func TestDriveTermExcludedFromMetrics(t *testing.T) {
	// One stream carries an ELIGIBLE todo (Next-up) and an AWAITING brief
	// (gate-score) — a live drive covers the whole stream.
	s := mkStream("hot", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},        // eligible → Next-up
		Brief{Num: "02", Wave: 0, Status: "implemented"}, // awaiting → gate-score
	)
	s.LastTouch = day(0)
	// filler keeps the drive's coverage of the eligible board under the tax
	// threshold, so the surge term stands full — isolating the metrics-exclusion
	// assertion from the coverage self-tax.
	streams := []*Stream{s, driveBriefStream("filler", 3)}

	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "surge-hot", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: hot\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("live manifest must apply: %+v", ds)
	}
	withDrives(t, ds)

	// Next-up (the ONLY surface the steer touches): hot/01 is boosted.
	nu := nextUp(streams, ClaimView{}, nil)
	var pick01 *Pick
	for i := range nu.Picks {
		if nu.Picks[i].Stream.Name == "hot" && nu.Picks[i].Brief.Num == "01" {
			pick01 = &nu.Picks[i]
		}
	}
	if pick01 == nil || pick01.DriveTerm != driveSurgeWeight {
		t.Fatalf("hot/01 must carry the +%d surge term on the Next-up board: %+v", driveSurgeWeight, nu.Picks)
	}

	// GATE-SCORE: the same stream's awaiting brief is scored WITHOUT any drive
	// term — the score is the base formula only.
	gates := gateScores(streams, nil)
	if len(gates) != 1 || gates[0].Brief.Num != "02" {
		t.Fatalf("want exactly the awaiting hot/02 in the gate queue, got %+v", gates)
	}
	base := priorityWeight(s.Priority) + valueWeight(gates[0].Brief.Value) // staleness 0, blockedCount 0
	if gates[0].Score != base {
		t.Errorf("gate-score carries a drive term (%d) — the steer must be EXCLUDED from the gate-score; want base %d", gates[0].Score, base)
	}
	if gates[0].Score >= base+driveFocusWeight {
		t.Errorf("gate-score %d is drive-inflated — the term leaked into mm/11", gates[0].Score)
	}

	// Structural exclusion from metrics: gateScores takes no DriveSet, and the
	// metric emitters (dora/bottleneck/code/export) are computed from the same
	// gate/stream data with no drive input. Proven here by the gate-score equality
	// above; nothing in this package feeds activeDriveSet into a metric path.
}

// --- Verify row 7 -------------------------------------------------------------

// TestDriveOverlapAndCoverage (brief-44 Verify row 7): at most 2 concurrent
// drives; overlapping drives take MAX not sum; coverage over 40% of the eligible
// briefs emits a NOTICE plus a concave scale-down.
func TestDriveOverlapAndCoverage(t *testing.T) {
	t.Run("overlap-max-not-sum", func(t *testing.T) {
		// Two live drives both cover stream x (one focus, one push). Enough OTHER
		// eligible briefs keep each drive's coverage under the tax threshold.
		x := driveBriefStream("x", 1)
		filler := driveBriefStream("filler", 4)
		streams := []*Stream{x, filler}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "a-focus", "declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nitems:\n  - stream: x\n")
		writeDrive(t, root, "b-push", "declared-by: ian\n"+liveWindow+"intensity: push\nstate: active\nitems:\n  - stream: x\n")
		ds := loadDrives(root, streams, driveTestNow)
		if len(ds.Active) != 2 {
			t.Fatalf("want exactly 2 active drives, got %d", len(ds.Active))
		}
		withDrives(t, ds)

		nu := nextUp(streams, ClaimView{}, nil)
		var px *Pick
		for i := range nu.Picks {
			if nu.Picks[i].Stream.Name == "x" {
				px = &nu.Picks[i]
			}
		}
		if px == nil {
			t.Fatal("x/01 must be on the board")
		}
		if px.DriveTerm != drivePushWeight {
			t.Fatalf("overlapping drives must take MAX (%d), not sum (%d): got %d", drivePushWeight, driveFocusWeight+drivePushWeight, px.DriveTerm)
		}
		if px.DriveSlug != "b-push" {
			t.Errorf("the max term must be attributed to the winning drive b-push, got %q", px.DriveSlug)
		}
	})

	t.Run("coverage-over-40pct-taxes-and-notices", func(t *testing.T) {
		// A single drive covering the WHOLE eligible board (100% > 40%): concave
		// self-tax scales the focus term down and a NOTICE fires.
		big := driveBriefStream("big", 5)
		streams := []*Stream{big}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "blanket", "declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nitems:\n  - stream: big\n")
		ds := loadDrives(root, streams, driveTestNow)
		withDrives(t, ds)

		nu := nextUp(streams, ClaimView{}, nil)
		if len(nu.DriveCoverageNotices) == 0 {
			t.Fatal("coverage > 40% must emit a NOTICE")
		}
		if !strings.Contains(nu.DriveCoverageNotices[0], "self-taxed") {
			t.Errorf("coverage NOTICE must name the self-tax: %q", nu.DriveCoverageNotices[0])
		}
		// 100% coverage → scale 0.4/1.0 → focus 800 * 0.4 = 320.
		wantTaxed := scaleForCoverage(driveFocusWeight, 1.0)
		if wantTaxed >= driveFocusWeight {
			t.Fatalf("test premise broken: scaled term %d not below full %d", wantTaxed, driveFocusWeight)
		}
		for _, p := range nu.Picks {
			if p.DriveTerm != wantTaxed {
				t.Errorf("pick %s: concave scale-down expected %d (< full %d), got %d", p.Brief.Num, wantTaxed, driveFocusWeight, p.DriveTerm)
			}
		}
	})

	t.Run("concave-monotonic", func(t *testing.T) {
		// The self-tax is concave/self-limiting: as coverage rises past the
		// threshold the term is non-increasing, and the full term stands at/below it.
		if scaleForCoverage(1000, 0.40) != 1000 {
			t.Error("at the threshold the full term must stand (no tax)")
		}
		if scaleForCoverage(1000, 0.30) != 1000 {
			t.Error("below the threshold the full term must stand")
		}
		prev := scaleForCoverage(1000, 0.41)
		for _, frac := range []float64{0.5, 0.6, 0.8, 1.0} {
			cur := scaleForCoverage(1000, frac)
			if cur > prev {
				t.Errorf("self-tax must be non-increasing in coverage: %.2f gave %d > previous %d", frac, cur, prev)
			}
			prev = cur
		}
	})

	t.Run("at-most-2-concurrent", func(t *testing.T) {
		// A third active drive tips the board over the ≤2 cap → fail-neutral.
		streams := []*Stream{driveBriefStream("x", 2)}
		root := t.TempDir()
		makeStreamsDir(t, root)
		for _, slug := range []string{"d1", "d2", "d3"} {
			writeDrive(t, root, slug, "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: x\n")
		}
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() {
			t.Fatalf("≤2 concurrent: 3 active drives must apply ZERO, got %d", len(ds.Active))
		}
		if !ds.NotApplied {
			t.Fatal("over-cap must set the DRIVE NOT APPLIED state")
		}
		withDrives(t, ds)
		nu := nextUp(streams, ClaimView{}, nil)
		for _, p := range nu.Picks {
			if p.DriveTerm != 0 {
				t.Errorf("over-cap board must carry zero boost, pick %s has %d", p.Brief.Num, p.DriveTerm)
			}
		}
	})
}

// --- Blocker 1 (review 4944957583): the re-ranking itself is pinned -----------

// TestDriveReRanksTheBoard pins the feature's single load-bearing behaviour: an
// active drive actually CHANGES the emitted board ORDER. A low-base brief carrying
// a surge steer must outrank a higher-base UNsteered pick on the board — the exact
// mutation the reviewer showed passing when the comparator is reverted from Total
// back to base Score. The value-pinning rows (term value, attributed slug,
// decomposed rendering, banner) never assert order; this one does, so reverting
// the comparator to `all[i].Score > all[j].Score` turns it red.
func TestDriveReRanksTheBoard(t *testing.T) {
	// low: P2 → base 1000. high: P1 → base 2000. With NO drive, high outranks low.
	low := mkStream("low", "active", "P2", Brief{Num: "01", Wave: 0, Status: "todo"})
	low.LastTouch = day(0)
	high := mkStream("high", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	high.LastTouch = day(0)
	// filler keeps the drive's coverage under the 40% self-tax threshold so the
	// surge term stands at full weight (isolates ordering from the coverage tax).
	filler := driveBriefStream("filler", 3)
	streams := []*Stream{low, high, filler}

	root := t.TempDir()
	makeStreamsDir(t, root)
	// A surge drive covering ONLY the low-base stream.
	writeDrive(t, root, "lift-low", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: low\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("a valid live manifest must apply: %+v", ds)
	}
	withDrives(t, ds)

	nu := nextUp(streams, ClaimView{}, nil)

	idx := func(stream string) (int, *Pick) {
		for i := range nu.Picks {
			if nu.Picks[i].Stream.Name == stream {
				return i, &nu.Picks[i]
			}
		}
		return -1, nil
	}
	iLow, pLow := idx("low")
	iHigh, pHigh := idx("high")
	if pLow == nil || pHigh == nil {
		t.Fatalf("both low and high must be on the board: %+v", nu.Picks)
	}

	// The steer must be the ONLY thing that flips the order: base scores put high
	// above low; totals put low above high.
	if pLow.Score >= pHigh.Score {
		t.Fatalf("test premise broken: low base %d must be BELOW high base %d", pLow.Score, pHigh.Score)
	}
	if pLow.DriveTerm != driveSurgeWeight {
		t.Fatalf("low must carry the full +%d surge term (coverage under the self-tax threshold), got %d", driveSurgeWeight, pLow.DriveTerm)
	}
	if pHigh.DriveTerm != 0 {
		t.Fatalf("high is uncovered and must carry zero drive term, got %d", pHigh.DriveTerm)
	}
	if pLow.Total() <= pHigh.Total() {
		t.Fatalf("boosted low total %d must exceed unboosted high total %d", pLow.Total(), pHigh.Total())
	}

	// THE assertion: on the emitted board the boosted low-base pick OUTRANKS the
	// higher-base unboosted pick. Revert the comparator to base Score (the
	// reviewer's mutation) and this fails — low(1000) falls below high(2000).
	if iLow >= iHigh {
		t.Fatalf("drive re-ranking not applied to the board: boosted low at index %d, unboosted high at %d — a surge-steered low-base brief MUST outrank a higher-base unboosted one", iLow, iHigh)
	}

	// And the emitted STATUS board renders low's row before high's row.
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	lowRow := strings.Index(out, "| low |")
	highRow := strings.Index(out, "| high |")
	if lowRow < 0 || highRow < 0 || lowRow >= highRow {
		t.Fatalf("emitted board must render the boosted low row before the unboosted high row (low=%d high=%d)", lowRow, highRow)
	}
}

// TestDriveUnresolvableIsFailNeutral (Blocker 2, review 4944957583): a manifest
// whose items resolve to nothing — a mistyped stream, a nonexistent brief, an
// un-zero-padded ref — must be fail-neutral (zero boost, WARN + banner), NOT
// silently inert. The un-padded ref is the likeliest real mistake.
func TestDriveUnresolvableIsFailNeutral(t *testing.T) {
	streams := []*Stream{driveBriefStream("education", 8)} // education/01..08 exist
	cases := []struct{ name, item string }{
		{"mistyped-stream", "  - stream: eduction\n"},
		{"nonexistent-brief", "  - brief: methodology/9999\n"},
		{"un-zero-padded-brief", "  - brief: education/7\n"}, // education/07 exists, /7 does not
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			makeStreamsDir(t, root)
			writeDrive(t, root, "campaign", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n"+tc.item)
			ds := loadDrives(root, streams, driveTestNow)
			if ds.applied() {
				t.Fatalf("%s: an unresolvable manifest must apply ZERO boost, got %d active", tc.name, len(ds.Active))
			}
			if !ds.NotApplied {
				t.Fatalf("%s: an unresolvable manifest must set the DRIVE NOT APPLIED banner", tc.name)
			}
			if len(ds.Warnings) == 0 {
				t.Fatalf("%s: an unresolvable manifest must emit a WARN — silence is the bug", tc.name)
			}
		})
	}
	// Positive control: the zero-padded ref DOES resolve and boost.
	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "campaign", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - brief: education/07\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("education/07 is a real brief and must resolve+apply: %+v", ds)
	}
}

// TestDriveSilentIgnoreClassWarns (Blocker 2, review 4944957583): files the
// loader does not read as a manifest — a .yml typo, a manifest in a subdirectory,
// a manifest with no state key — must WARN rather than vanish silently.
func TestDriveSilentIgnoreClassWarns(t *testing.T) {
	streams := []*Stream{driveBriefStream("hot", 3)}
	drivesDir := func(root string) string { return filepath.Join(root, "docs", "roadmap", "drives") }
	body := "declared-by: ian\n" + liveWindow + "intensity: surge\nstate: active\nitems:\n  - stream: hot\n"

	t.Run("yml-extension", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		if err := os.MkdirAll(drivesDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(drivesDir(root), "campaign.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() {
			t.Fatalf(".yml must not apply (only *.yaml is read), got %d active", len(ds.Active))
		}
		if len(ds.Warnings) == 0 || !ds.NotApplied {
			t.Fatalf(".yml typo must WARN + set the banner, not vanish: %+v", ds)
		}
	})

	t.Run("subdirectory", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		sub := filepath.Join(drivesDir(root), "archive")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "campaign.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() {
			t.Fatalf("a manifest in a subdirectory must not apply, got %d active", len(ds.Active))
		}
		if len(ds.Warnings) == 0 || !ds.NotApplied {
			t.Fatalf("a subdirectory manifest must WARN + set the banner: %+v", ds)
		}
	})

	t.Run("no-state-key", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		// No state: key at all — a typo, not a decision.
		writeDrive(t, root, "campaign", "declared-by: ian\n"+liveWindow+"intensity: surge\nitems:\n  - stream: hot\n")
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() {
			t.Fatalf("a manifest with no state key must not apply, got %d active", len(ds.Active))
		}
		if len(ds.Warnings) == 0 || !ds.NotApplied {
			t.Fatalf("a missing state key must WARN + set the banner (typo, not a decision): %+v", ds)
		}
	})

	t.Run("labelled-inactive-stays-silent", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		// A deliberately labelled inactive drive is a decision — silent, no warn.
		writeDrive(t, root, "campaign", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: scheduled\nitems:\n  - stream: hot\n")
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() || ds.NotApplied || len(ds.Warnings) != 0 {
			t.Fatalf("a labelled-inactive (state: scheduled) drive must be SILENT: %+v", ds)
		}
	})

	// symlink-escape-refused PINS escapesRoot (review 4945002046 second ask): a
	// manifest committed as a symlink pointing OUT of the tree is otherwise
	// followed and applied — invisible in a diff. The containment check must
	// refuse it, fail-neutral. This subtest is the mutation witness: stub
	// escapesRoot to always return false and the escape is followed, the
	// out-of-tree target applies, ds.applied() flips true, and this goes red.
	t.Run("symlink-escape-refused", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		if err := os.MkdirAll(drivesDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		// A VALID, active, in-window manifest covering "hot", living OUTSIDE the
		// repo root: on its own it WOULD apply and boost — so the only thing that
		// can keep it out is the containment check.
		outside := t.TempDir()
		target := filepath.Join(outside, "escape.yaml")
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(drivesDir(root), "campaign.yaml")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}
		ds := loadDrives(root, streams, driveTestNow)
		if ds.applied() {
			t.Fatalf("a manifest that is a symlink escaping the repo root must NOT apply, got %d active", len(ds.Active))
		}
		if !ds.NotApplied {
			t.Fatal("a symlink escape must set the DRIVE NOT APPLIED banner (fail-neutral)")
		}
		found := false
		for _, w := range ds.Warnings {
			if strings.Contains(w, "symlink escape") {
				found = true
			}
		}
		if !found {
			t.Fatalf("a symlink escape must emit a WARN naming the escape: %+v", ds.Warnings)
		}
	})
}

// TestDriveAppliedClearsNotAppliedBanner (finding, review 4944957583): with one
// valid and one rejected manifest present, the board must NOT show both the ACTIVE
// DRIVE and DRIVE NOT APPLIED headline banners. The applied drive is the headline;
// the rejected file keeps its own WARN line.
func TestDriveAppliedClearsNotAppliedBanner(t *testing.T) {
	streams := []*Stream{driveBriefStream("hot", 3)}
	root := t.TempDir()
	makeStreamsDir(t, root)
	// good applies; bad is unresolvable (rejected).
	writeDrive(t, root, "good", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: hot\n")
	writeDrive(t, root, "bad", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: nope\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("the good manifest must apply: %+v", ds)
	}
	if ds.NotApplied {
		t.Fatal("an applied drive must CLEAR the DRIVE NOT APPLIED banner — no contradictory banner pair")
	}
	if len(ds.Warnings) == 0 {
		t.Fatal("the rejected sibling must still keep its own WARN line")
	}
	withDrives(t, ds)
	nu := nextUp(streams, ClaimView{}, nil)
	if nu.DriveNotApplied != "" {
		t.Fatalf("board must not carry the DRIVE NOT APPLIED banner when a drive applied: %q", nu.DriveNotApplied)
	}
	if nu.DriveBanner == "" {
		t.Fatal("board must carry the ACTIVE DRIVE banner")
	}
}
