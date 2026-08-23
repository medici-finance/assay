package main

import (
	"fmt"
	"strings"
	"testing"
)

// withFindings sets the activeFindings package var for the duration of a test and
// restores it afterward (mirrors withDrives). The zero value (nil) is inert.
func withFindings(t *testing.T, findings []Finding) {
	t.Helper()
	old := activeFindings
	activeFindings = findings
	t.Cleanup(func() { activeFindings = old })
}

// withStampAuthorities installs a ratified critical-stamp authority allowlist for
// the duration of a test and restores the placeholder afterward. The production
// default is EMPTY (the security arm is inert until a human ratifies), so a test that
// exercises the mechanism must opt into an authority explicitly.
func withStampAuthorities(t *testing.T, auths ...string) {
	t.Helper()
	old := criticalStampAuthorities
	m := map[string]bool{}
	for _, a := range auths {
		m[a] = true
	}
	criticalStampAuthorities = m
	t.Cleanup(func() { criticalStampAuthorities = old })
}

// --- Verify row 5 -------------------------------------------------------------

// TestDriveCriticalTierNeverBuried (brief-44 Verify row 5): the lexicographic
// (criticalTier, score) order ranks main-red / stamped-security / high-unblocks
// (blockedCount≥3) / reviewer-finding rows ABOVE all scores; no intensity — surge
// included — can pass the critical tier; membership is machine-derived / stamped,
// never self-declared.
func TestDriveCriticalTierNeverBuried(t *testing.T) {
	// The critical tier is applied only when a drive is active. A SURGE drive covers
	// the routine stream — the strongest intensity — and must STILL sit below a
	// critical fire on the board. That is the whole never-buried property.

	t.Run("high-unblocks-outranks-surge", func(t *testing.T) {
		// fire/01 is a P2 brief (base 1000 + 3×unblocks 1500 = 2500) that blocks three
		// others → high-unblocks critical. routine/01 is a P1 brief (base 2000) carried
		// by a SURGE drive (+2500 = 4500 total). Without the tier the surge total buries
		// the fire; with it the fire ranks first.
		fire := mkStream("fire", "active", "P2",
			Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1"},
		)
		fire.LastTouch = day(0)
		// Three downstream briefs depend on fire/01 → blockedCount(fire/01)=3.
		blocked := mkStream("blocked", "active", "P2",
			Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
			Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
			Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
		)
		blocked.LastTouch = day(0)
		routine := mkStream("routine", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
		routine.LastTouch = day(0)
		filler := driveBriefStream("filler", 4)
		streams := []*Stream{fire, blocked, routine, filler}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-routine", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: routine\n")
		ds := loadDrives(root, streams, driveTestNow)
		if !ds.applied() {
			t.Fatalf("the surge drive must apply: %+v", ds)
		}
		withDrives(t, ds)
		withFindings(t, nil)

		nu := nextUp(streams, ClaimView{}, nil)
		iFire, pFire := pickIndex(nu, "fire")
		iRoutine, pRoutine := pickIndex(nu, "routine")
		if pFire == nil || pRoutine == nil {
			t.Fatalf("both fire and routine must be on the board: %+v", nu.Picks)
		}
		if !pFire.CriticalTier || pFire.CriticalArm != "high-unblocks" {
			t.Fatalf("fire/01 (blockedCount 3) must be critical via high-unblocks, got critical=%v arm=%q", pFire.CriticalTier, pFire.CriticalArm)
		}
		if pRoutine.CriticalTier {
			t.Fatalf("routine/01 is not a fire and must NOT be in the critical tier")
		}
		// The surge total genuinely exceeds the fire's total — proving it is the tier,
		// not the score, that keeps the fire on top.
		if pRoutine.Total() <= pFire.Total() {
			t.Fatalf("test premise broken: surge total %d must exceed fire total %d", pRoutine.Total(), pFire.Total())
		}
		if iFire >= iRoutine {
			t.Fatalf("critical fire (idx %d) must rank ABOVE the surge-boosted routine (idx %d) — no intensity may bury a live fire", iFire, iRoutine)
		}
	})

	t.Run("stamped-security-authorized-only", func(t *testing.T) {
		// A brief carrying a security/critical stamp is critical ONLY when the stamp's
		// authority is ratified. With the placeholder allowlist empty the arm is inert.
		b := Brief{Num: "01", Wave: 0, Status: "todo", Reviewed: "2026-08-15 critical-security(security-desk)"}

		// Placeholder (empty allowlist): NOT authorized → not critical.
		if arm := criticalTierArm(b, "sec", 0, nil); arm != "" {
			t.Fatalf("with the empty placeholder allowlist a security stamp must grant nothing, got arm %q", arm)
		}
		// Ratified: authority allowlisted → security arm fires.
		withStampAuthorities(t, "security-desk")
		if arm := criticalTierArm(b, "sec", 0, nil); arm != "security" {
			t.Fatalf("a ratified-authority security stamp must qualify via the security arm, got %q", arm)
		}
		// A stamp from an UN-ratified authority still grants nothing.
		other := Brief{Num: "02", Status: "todo", Reviewed: "critical-security(random-actor)"}
		if arm := criticalTierArm(other, "sec", 0, nil); arm != "" {
			t.Fatalf("an un-ratified authority must not qualify, got %q", arm)
		}
	})

	t.Run("reviewer-finding-arm", func(t *testing.T) {
		// An unresolved reviewer finding naming the brief qualifies it (machine-derived).
		findings := []Finding{{ID: "F-leak-01", Affects: []string{"sec/01"}, Resolved: false}}
		if arm := criticalTierArm(Brief{Num: "01", Status: "todo"}, "sec", 0, findings); arm != "reviewer-finding" {
			t.Fatalf("a brief named by an unresolved finding must qualify via reviewer-finding, got %q", arm)
		}
		// A RESOLVED finding does not.
		resolved := []Finding{{ID: "F-leak-01", Affects: []string{"sec/01"}, Resolved: true}}
		if arm := criticalTierArm(Brief{Num: "01", Status: "todo"}, "sec", 0, resolved); arm != "" {
			t.Fatalf("a resolved finding must not qualify, got %q", arm)
		}
		// A bare-STREAM affects entry is NOT broadcast to every brief (anti-broadcast).
		streamLevel := []Finding{{ID: "F-x", Affects: []string{"sec"}, Resolved: false}}
		if arm := criticalTierArm(Brief{Num: "01", Status: "todo"}, "sec", 0, streamLevel); arm != "" {
			t.Fatalf("a bare-stream finding must NOT mark a brief critical (anti-broadcast), got %q", arm)
		}
		// The brief-<NN> spelling of an affects entry resolves the same.
		dashed := []Finding{{ID: "F-y", Affects: []string{"sec/brief-01"}, Resolved: false}}
		if arm := criticalTierArm(Brief{Num: "01", Status: "todo"}, "sec", 0, dashed); arm != "reviewer-finding" {
			t.Fatalf("a stream/brief-NN affects entry must resolve, got %q", arm)
		}
	})

	t.Run("no-intensity-passes-the-tier", func(t *testing.T) {
		// Even the strongest intensity (surge, +2500) does not set CriticalTier: the
		// drive term is a score input, the tier is an orthogonal ordering key. A pure
		// surge-boosted routine brief with no fire property is never critical.
		routine := Brief{Num: "01", Status: "todo"}
		for _, bc := range []int{0, 1, 2} { // below the high-unblocks threshold
			if arm := criticalTierArm(routine, "routine", bc, nil); arm != "" {
				t.Fatalf("a routine brief (blockedCount %d, no stamp, no finding) must not be critical, got %q", bc, arm)
			}
		}
	})

	t.Run("main-red-arm-deferred", func(t *testing.T) {
		// The main-red arm is a documented seam: it must always report false (statusgen
		// cannot poll live CI offline). This pins the deferral so it cannot silently
		// grow a network read.
		if mainRedCritical(Brief{Num: "01", Status: "todo"}, "any") {
			t.Fatal("the main-red arm is DEFERRED and must always report false pending an in-tree machine-derived signal")
		}
	})
}

// TestDriveCriticalArmDisplayedAttributed pins that a critical pick carries its
// arm attribution (so the board can show WHY a row jumped, never an unexplained
// reorder).
func TestDriveCriticalArmDisplayedAttributed(t *testing.T) {
	fire := mkStream("fire", "active", "P2",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1"},
	)
	fire.LastTouch = day(0)
	blocked := mkStream("blocked", "active", "P2",
		Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
	)
	blocked.LastTouch = day(0)
	streams := []*Stream{fire, blocked, driveBriefStream("filler", 3)}

	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "surge-filler", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: filler\n")
	ds := loadDrives(root, streams, driveTestNow)
	withDrives(t, ds)
	withFindings(t, nil)

	nu := nextUp(streams, ClaimView{}, nil)
	_, pFire := pickIndex(nu, "fire")
	if pFire == nil || !pFire.CriticalTier {
		t.Fatalf("fire/01 must be a critical pick: %+v", nu.Picks)
	}
	if !strings.Contains(pFire.CriticalArm, "high-unblocks") {
		t.Errorf("critical arm must be attributed as high-unblocks, got %q", pFire.CriticalArm)
	}
}

// pickIndex returns the index and pointer of the first pick from the given stream.
func pickIndex(nu NextUp, stream string) (int, *Pick) {
	for i := range nu.Picks {
		if nu.Picks[i].Stream.Name == stream {
			return i, &nu.Picks[i]
		}
	}
	return -1, nil
}

// TestDriveDepEdgeReciprocity (brief-44 Verify row R): the dependency-edge
// reciprocity lint flags a dangling / self-referential / one-sided depends edge
// (so blockedCount cannot be gamed into the high-unblocks arm), and passes a
// genuine reciprocated edge.
//
// TIER split (Ian's ruling, assay#92): the malformed edges — self-referential and
// dangling — stay hard PROBLEMs in checkRef (a self-loop / missing target is not a
// data-quality debt but a broken ref). The RECIPROCITY leg (a valid A→B that B
// does not reciprocate) ships at NOTICE, since ~104 legitimate older edges predate
// the two-sided convention; reciprocityNotices() therefore returns NOTICE-tier
// lines that checkBriefFiles routes into its `notices` channel (non-fatal, exit 0).
func TestDriveDepEdgeReciprocity(t *testing.T) {
	t.Run("self-referential-depends-is-a-problem", func(t *testing.T) {
		var problems []string
		add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
		checkRef(add, "x/brief-01.md", "depends", "x/01", "x/01", map[string]*Stream{
			"x": mkStream("x", "active", "P1", Brief{Num: "01"}),
		})
		if len(problems) != 1 || !strings.Contains(problems[0], "self-referential") {
			t.Fatalf("a self-referential depends edge must be a PROBLEM naming it: %v", problems)
		}
	})

	t.Run("self-referential-unblocks-is-a-problem", func(t *testing.T) {
		var problems []string
		add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
		checkRef(add, "x/brief-01.md", "unblocks", "x/01", "x/01", map[string]*Stream{
			"x": mkStream("x", "active", "P1", Brief{Num: "01"}),
		})
		if len(problems) != 1 || !strings.Contains(problems[0], "self-referential") {
			t.Fatalf("a self-referential unblocks edge must be a PROBLEM: %v", problems)
		}
	})

	t.Run("dangling-depends-is-a-problem", func(t *testing.T) {
		var problems []string
		add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
		checkRef(add, "x/brief-02.md", "depends", "x/99", "x/02", map[string]*Stream{
			"x": mkStream("x", "active", "P1", Brief{Num: "01"}, Brief{Num: "02"}),
		})
		if len(problems) != 1 || !strings.Contains(problems[0], "unknown brief") {
			t.Fatalf("a dangling depends edge must be a PROBLEM: %v", problems)
		}
	})

	t.Run("one-sided-depends-is-a-notice-not-a-problem", func(t *testing.T) {
		// A→B declared, but B does not reciprocate with unblocks: A. Spurious inbound
		// edge — flagged so blockedCount cannot be inflated, at NOTICE tier (assay#92).
		idx := newDepEdgeIndex()
		idx.dependsOf["a/01"] = []string{"b/01"}
		idx.unblocksOf["a/01"] = nil
		idx.dependsOf["b/01"] = nil
		idx.unblocksOf["b/01"] = nil // B does NOT unblock A → one-sided
		idx.knownV1["a/01"] = true
		idx.knownV1["b/01"] = true
		notices := idx.reciprocityNotices()
		if len(notices) != 1 || !strings.Contains(notices[0], "one-sided") {
			t.Fatalf("a one-sided depends edge must be a NOTICE naming it: %v", notices)
		}
	})

	t.Run("genuine-reciprocated-edge-passes", func(t *testing.T) {
		// A→B depends, B→A unblocks: a genuine two-sided dependency. No NOTICE.
		idx := newDepEdgeIndex()
		idx.dependsOf["a/01"] = []string{"b/01"}
		idx.unblocksOf["a/01"] = nil
		idx.dependsOf["b/01"] = nil
		idx.unblocksOf["b/01"] = []string{"a/01"} // B reciprocates
		idx.knownV1["a/01"] = true
		idx.knownV1["b/01"] = true
		if notices := idx.reciprocityNotices(); len(notices) != 0 {
			t.Fatalf("a genuine reciprocated edge must pass clean: %v", notices)
		}
	})

	t.Run("legacy-target-is-exempt", func(t *testing.T) {
		// A depends on a legacy (non-brief-v1) brief that declares no unblocks. Mixed
		// corpora must not be reddened for the pre-typed-edge convention.
		idx := newDepEdgeIndex()
		idx.dependsOf["a/01"] = []string{"legacy/01"}
		idx.knownV1["a/01"] = true
		// legacy/01 is NOT in knownV1.
		if notices := idx.reciprocityNotices(); len(notices) != 0 {
			t.Fatalf("a depends edge to a legacy (non-brief-v1) target must be exempt: %v", notices)
		}
	})
}
