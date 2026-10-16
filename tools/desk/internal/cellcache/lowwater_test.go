//go:build darwin || linux

package cellcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveRecord holds a consumer record for the test's length, so every pass
// below runs on the trim path, never the whole-root removal.
func liveRecord(t *testing.T, p Policy) {
	t.Helper()
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Finish(true) })
}

// Over the high-water mark, or under pressure, the trim takes compiler-cache
// entries oldest first and stops once the cache is at or under the low-water
// mark: newer entries the target did not need stay warm.
func TestCacheLowWaterTrimOldestFirst(t *testing.T) {
	for _, c := range []struct {
		name    string
		budget  int64
		trigger string
		gone    []string
		kept    []string
		after   int64
	}{
		// 800 bytes: over 750 (high water), under 1000 (no pressure);
		// target 600, met by the oldest entry alone.
		{"high-water", 1000, "high-water", []string{"build/00/e1-a"}, []string{"build/01/e2-d", "build/02/e3-a", "build/03/e4-d"}, 500},
		// 800 bytes over a 700 budget: pressure; target 420 needs two.
		{"pressure", 700, "pressure", []string{"build/00/e1-a", "build/01/e2-d"}, []string{"build/02/e3-a", "build/03/e4-d"}, 300},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := fixture(t)
			p.Budget = c.budget
			liveRecord(t, p)
			for _, e := range []struct {
				name string
				size int
				age  time.Duration
			}{
				{"build/00/e1-a", 300, 10 * time.Hour},
				{"build/01/e2-d", 200, 8 * time.Hour},
				{"build/02/e3-a", 200, 6 * time.Hour},
				{"build/03/e4-d", 100, 4 * time.Hour},
			} {
				material(t, p, e.name, e.size)
				age(t, p, e.name, e.age)
			}
			// A dry run trims nothing and reports no trim.
			if r, _ := Check(p, true); r.TrimmedBuild != 0 || r.LowWater != nil {
				t.Fatalf("dry run trimmed: %+v", r)
			}
			r, err := Check(p, false)
			if err != nil || r.Outcome != "checked-clean" {
				t.Fatalf("trim did not leave a clean pass: %+v %v", r, err)
			}
			lw := r.LowWater
			if lw == nil || lw.Trigger != c.trigger || !lw.Reached || lw.TargetBytes != p.LowWater() || lw.ShortfallBytes != 0 {
				t.Fatalf("low-water report: %+v", lw)
			}
			if *r.After.LogicalBytes != c.after || *r.After.LogicalBytes > p.LowWater() || r.TrimmedBuild != len(c.gone) {
				t.Fatalf("after %d bytes, %d trimmed; want %d bytes, %d trimmed", *r.After.LogicalBytes, r.TrimmedBuild, c.after, len(c.gone))
			}
			for _, name := range c.gone {
				if exists(p, name) {
					t.Errorf("%s survived: the oldest entries go first", name)
				}
			}
			for _, name := range c.kept {
				if !exists(p, name) {
					t.Errorf("%s removed after the target was met", name)
				}
			}
		})
	}
}

// The safety floor binds even when it leaves the cache above its target: the
// trim stops, keeps every young entry, and records the shortfall and why.
func TestCacheLowWaterFloorShortfall(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/old-a", 100)
	age(t, p, "build/00/old-a", 5*time.Hour)
	material(t, p, "build/01/young-d", 400)
	age(t, p, "build/01/young-d", 90*time.Minute)
	material(t, p, "build/02/younger-a", 400)
	age(t, p, "build/02/younger-a", 10*time.Minute)
	r, err := Check(p, false)
	if err != nil || r.Outcome != "checked-clean" {
		t.Fatalf("under budget after the trim: %+v %v", r, err)
	}
	if exists(p, "build/00/old-a") || !exists(p, "build/01/young-d") || !exists(p, "build/02/younger-a") {
		t.Fatal("the trim crossed the safety floor, or skipped the entry past it")
	}
	lw := r.LowWater
	if lw == nil || lw.Reached || lw.ShortfallBytes != 200 || lw.MinAgeSeconds != 7200 || !strings.Contains(lw.Reason, "safety floor") {
		t.Fatalf("shortfall not reported: %+v", lw)
	}
	b, err := os.ReadFile(p.ReportPath())
	if err != nil || !strings.Contains(string(b), `"low_water_reached": false`) || !strings.Contains(string(b), `"shortfall_bytes": 200`) {
		t.Fatalf("persisted report lacks the shortfall: %s %v", b, err)
	}
}

// An entry a build refreshes between the listing and the removal is never
// removed: the floor is judged again on the entry's mtime at removal, and the
// trim takes the next-oldest entry instead.
func TestCacheLowWaterSparesTouchedEntry(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/touched-a", 300)
	age(t, p, "build/00/touched-a", 10*time.Hour)
	material(t, p, "build/01/next-d", 300)
	age(t, p, "build/01/next-d", 9*time.Hour)
	material(t, p, "build/02/young-a", 200)
	age(t, p, "build/02/young-a", 3*time.Hour)
	old := afterTrimListing
	t.Cleanup(func() { afterTrimListing = old })
	afterTrimListing = func() {
		now := time.Now()
		if err := os.Chtimes(filepath.Join(p.Root, "build/00/touched-a"), now, now); err != nil {
			t.Error(err)
		}
	}
	r, err := Check(p, false)
	if err != nil || !exists(p, "build/00/touched-a") {
		t.Fatalf("a refreshed entry was removed: %+v %v", r, err)
	}
	if exists(p, "build/01/next-d") || r.TrimmedBuild != 1 || r.LowWater == nil || !r.LowWater.Reached {
		t.Fatalf("the trim did not move on to the next-oldest entry: %+v", r)
	}
}

// The low-water mark must sit under the high-water mark and the floor above
// Go's hourly use stamp, whether set through the environment or recorded; a
// policy that sets neither serializes as it did before both existed.
func TestCacheLowWaterPolicy(t *testing.T) {
	p := fixture(t)
	if p.LowWater() != p.Budget/100*60 || p.MinAge() != 2*time.Hour || p.HighWater() != p.Budget/4*3 {
		t.Fatalf("defaults: low %d min-age %v high %d", p.LowWater(), p.MinAge(), p.HighWater())
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "low_water") || strings.Contains(string(b), "trim_min_age") {
		t.Fatalf("a default policy's serialization changed: %s", b)
	}
	get := func(key, v string) func(string) string {
		return func(k string) string {
			switch k {
			case "CELL_GO_CACHE":
				return "on"
			case key:
				return v
			}
			return ""
		}
	}
	cell := filepath.Dir(p.Root)
	for _, bad := range []string{"75", "80", "0", "-5", "60.5", "most"} {
		if _, err := Resolve(cell, get("CELL_GO_CACHE_LOW_WATER_PERCENT", bad)); err == nil {
			t.Errorf("accepted CELL_GO_CACHE_LOW_WATER_PERCENT=%s", bad)
		}
	}
	for _, bad := range []string{"1h", "61m", "119m59s", "30m", "0", "-3h", "150m0.5s", "soon"} {
		if _, err := Resolve(cell, get("CELL_GO_CACHE_TRIM_MIN_AGE", bad)); err == nil {
			t.Errorf("accepted CELL_GO_CACHE_TRIM_MIN_AGE=%s", bad)
		}
	}
	q, err := Resolve(cell, get("CELL_GO_CACHE_LOW_WATER_PERCENT", "50"))
	if err != nil || q.LowWater() != q.Budget/100*50 {
		t.Fatalf("50%%: %v %v", q, err)
	}
	// The minimum itself is accepted: the floor is "at least 2h", like the
	// trim age, never "above" it.
	q, err = Resolve(cell, get("CELL_GO_CACHE_TRIM_MIN_AGE", "2h"))
	if err != nil || q.MinAge() != 2*time.Hour {
		t.Fatalf("2h: %v %v", q, err)
	}
	q, err = Resolve(cell, get("CELL_GO_CACHE_TRIM_MIN_AGE", "150m"))
	if err != nil || q.MinAge() != 150*time.Minute {
		t.Fatalf("150m: %v %v", q, err)
	}
	// A set value round-trips through the consumer environment intact.
	if got, err := FromEnv(q.Env()); err != nil || *got != *q {
		t.Fatalf("round trip: %v %v", got, err)
	}
	recorded := p
	recorded.LowWaterPercent = HighWaterPercent
	if recorded.Validate() == nil {
		t.Fatal("recorded low-water mark at the high-water mark accepted")
	}
	recorded = p
	recorded.TrimMinAge = int64((2*time.Hour - time.Second) / time.Second)
	if recorded.Validate() == nil {
		t.Fatal("recorded safety floor under two hours accepted")
	}
	recorded.TrimMinAge = int64(2 * time.Hour / time.Second)
	if err := recorded.Validate(); err != nil {
		t.Fatalf("recorded safety floor of exactly two hours refused: %v", err)
	}
}

// End to end: a cache Check trimmed down to its low-water mark is one the
// next admission clears lock-free, without taking the lock.
func TestCacheLowWaterThenAdmitPrecheck(t *testing.T) {
	p := fixture(t)
	p.Budget = admitBudget
	liveRecord(t, p)
	// 900,000 bytes: over the 786,432 high-water mark Admit's pre-check
	// refuses above, under the budget.
	material(t, p, "build/00/old-a", 600000)
	age(t, p, "build/00/old-a", 5*time.Hour)
	material(t, p, "build/01/warm-d", 300000)
	age(t, p, "build/01/warm-d", 3*time.Hour)
	quiet(t, p)
	r, err := Check(p, false)
	if err != nil || r.LowWater == nil || !r.LowWater.Reached || *r.After.LogicalBytes != 300000 {
		t.Fatalf("trim to low water: %+v %v", r, err)
	}
	calls := 0
	r, err = admit(p, false, envWith(bigFree, &calls))
	if err != nil || r.Admission != "precheck" || calls != 0 || *r.After.LogicalBytes != 300000 {
		t.Fatalf("trimmed cache not admitted lock-free: %+v %v (locked calls %d)", r, err, calls)
	}
}

// Under the high-water mark and the budget, with no wipe asked, a pass has no
// cause to trim: old-but-not-aged entries stay and no trim is reported.
func TestCacheLowWaterQuietBelowHighWater(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/old-a", 700)
	age(t, p, "build/00/old-a", 10*time.Hour)
	r, err := Check(p, false)
	if err != nil || r.TrimmedBuild != 0 || r.LowWater != nil || !exists(p, "build/00/old-a") {
		t.Fatalf("a cache under the high-water mark was trimmed: %+v %v", r, err)
	}
}

// The removal-time re-check binds at the SAFETY FLOOR, not at the trim's own
// start time: an entry a build refreshed after the listing to an mtime that is
// already in the past (30 minutes, inside the 2h floor) is still spared.
// Re-checked against "now" instead, it would go.
func TestCacheLowWaterRecheckCutoffIsTheFloor(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/touched-a", 300)
	age(t, p, "build/00/touched-a", 10*time.Hour)
	material(t, p, "build/01/next-d", 300)
	age(t, p, "build/01/next-d", 9*time.Hour)
	material(t, p, "build/02/young-a", 200)
	age(t, p, "build/02/young-a", 3*time.Hour)
	old := afterTrimListing
	t.Cleanup(func() { afterTrimListing = old })
	afterTrimListing = func() {
		inside := time.Now().Add(-30 * time.Minute)
		if err := os.Chtimes(filepath.Join(p.Root, "build/00/touched-a"), inside, inside); err != nil {
			t.Error(err)
		}
	}
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	if !exists(p, "build/00/touched-a") {
		t.Fatal("an entry refreshed to inside the safety floor after the listing was removed")
	}
	if exists(p, "build/01/next-d") {
		t.Fatal("the trim did not move on to the next-oldest entry")
	}
}

// A wipe that finds live records trims toward ZERO, not toward the low-water
// mark: every compiler-cache entry past the safety floor goes even when the
// cache is already under the low-water mark, and only entries younger than
// the floor stay.
func TestCacheWipeUnderRecordsTrimsToZero(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/past-floor-a", 100)
	age(t, p, "build/00/past-floor-a", 5*time.Hour)
	material(t, p, "build/01/past-floor-d", 100)
	age(t, p, "build/01/past-floor-d", 3*time.Hour)
	material(t, p, "build/02/young-a", 100)
	age(t, p, "build/02/young-a", 30*time.Minute)
	material(t, p, "mod/cache/keep", 50)
	r, err := Wipe(p, false)
	if err != nil && r.Outcome == "" {
		t.Fatal(err)
	}
	if exists(p, "build/00/past-floor-a") || exists(p, "build/01/past-floor-d") {
		t.Fatalf("a wipe under records left entries past the safety floor: %+v", r)
	}
	if !exists(p, "build/02/young-a") || !exists(p, "mod/cache/keep") {
		t.Fatal("a wipe under records crossed the safety floor or touched mod/")
	}
	if lw := r.LowWater; lw == nil || lw.Trigger != "wipe" || lw.TargetBytes != 0 || lw.Reached {
		t.Fatalf("wipe low-water report: %+v", lw)
	}
}

// A wipe whose measurement failed never reaches the trim: there is no size to
// count down from, and acting on a cache it could not measure is exactly what
// the failures gate exists to stop.
func TestCacheWipeSkipsTrimWhenMeasureFails(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/old-a", 100)
	age(t, p, "build/00/old-a", 5*time.Hour)
	material(t, p, "mod/cache/bad", 10)
	if err := os.Chmod(filepath.Join(p.Root, "mod/cache/bad"), 0o666); err != nil {
		t.Fatal(err)
	}
	r, _ := Wipe(p, false)
	if r.Outcome != "could-not-check" || r.Before.LogicalBytes != nil {
		t.Fatalf("an unmeasurable cache was not could-not-check: %+v", r)
	}
	if !exists(p, "build/00/old-a") || r.LowWater != nil || r.TrimmedBuild != 0 {
		t.Fatalf("the trim ran on a cache that could not be measured: %+v", r)
	}
}

// A trim that removed nothing but hit a refusal still counts as having acted,
// so the pass measures the cache again instead of reusing the first walk: a
// refused directory entry may have been partly removed.
func TestCacheTrimErrorRemeasures(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	liveRecord(t, p)
	material(t, p, "build/00/old-a", 900)
	age(t, p, "build/00/old-a", 10*time.Hour)
	old := afterTrimListing
	t.Cleanup(func() { afterTrimListing = old })
	afterTrimListing = func() {
		// Turns bad between measurement and removal: the trim refuses it.
		if err := os.Chmod(filepath.Join(p.Root, "build/00/old-a"), 0o666); err != nil {
			t.Error(err)
		}
	}
	r, _ := Check(p, false)
	if r.TrimmedBuild != 0 || !exists(p, "build/00/old-a") {
		t.Fatalf("the refused entry was removed: %+v", r)
	}
	remeasured := false
	for _, f := range r.Failures {
		if strings.HasPrefix(f, "measure build") {
			remeasured = true
		}
	}
	if !remeasured || r.After.LogicalBytes != nil {
		t.Fatalf("a trim that hit a refusal reused the first measurement: %+v", r)
	}
}

// When what a trim can never take (the module cache) alone holds the cache
// above the low-water mark, emptying build/ cannot reach the target, so the
// trim does not chase it: only entries past the trim age go, warm entries past
// the floor stay, and the shortfall names the untrimmable bytes.
func TestCacheLowWaterModAboveTargetAgeTrimOnly(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000 // high water 750, low water 600
	liveRecord(t, p)
	material(t, p, "mod/cache/big", 650)
	material(t, p, "build/00/ancient-a", 50)
	age(t, p, "build/00/ancient-a", 30*time.Hour)
	material(t, p, "build/01/warm-d", 100)
	age(t, p, "build/01/warm-d", 5*time.Hour)
	r, err := Check(p, false)
	if err != nil || r.Outcome != "checked-clean" {
		t.Fatalf("pass: %+v %v", r, err)
	}
	if exists(p, "build/00/ancient-a") {
		t.Fatal("the entry past the trim age survived")
	}
	if !exists(p, "build/01/warm-d") {
		t.Fatal("a warm entry was evicted chasing a target only the module cache keeps out of reach")
	}
	lw := r.LowWater
	if lw == nil || lw.Trigger != "high-water" || lw.Reached || lw.TargetBytes != p.LowWater() || !strings.Contains(lw.Reason, "not trimmable") {
		t.Fatalf("low-water report: %+v", lw)
	}
}
