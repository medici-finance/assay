package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDriveLintReusesHydratedBoardAndClaimRead(t *testing.T) {
	root, _ := driveManyRowBoard(t)
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	prev := listRemoteBranches
	t.Cleanup(func() { listRemoteBranches = prev })
	reads := 0
	listRemoteBranches = func(string) ([]string, error) {
		reads++
		return nil, nil
	}
	stubMergedClosedBranches(t, nil, nil)
	ctx := &driveLintCtx{root: root, loaded: true, streams: streams}
	first, rc := ctx.section("d1", driveTestNow)
	if rc != 0 {
		t.Fatalf("first render failed: %d", rc)
	}
	second, rc := ctx.section("d1", driveTestNow)
	if rc != 0 || first != second || reads != 1 {
		t.Fatalf("shared render changed or reread claims: rc=%d reads=%d", rc, reads)
	}
	wantProblems, wantNotices := driveRegionLintProblems(root, streams, resolveClaims, driveTestNow)
	if len(wantProblems) != 0 {
		t.Fatalf("hydrated fixture must lint clean: %v", wantProblems)
	}
	// The source board has already passed hydration. Removing its fixture
	// READMEs proves the renderer consumes those rows, rather than reloading.
	for _, s := range streams {
		if err := os.Remove(filepath.Join(s.Dir, "README.md")); err != nil {
			t.Fatal(err)
		}
	}
	gotProblems, gotNotices := driveRegionLintProblems(root, streams, resolveClaims, driveTestNow)
	if !reflect.DeepEqual(gotProblems, wantProblems) || !reflect.DeepEqual(gotNotices, wantNotices) {
		t.Fatalf("validated board was reloaded: problems=%v notices=%v; want %v %v", gotProblems, gotNotices, wantProblems, wantNotices)
	}
}

// scopedDriveRunFixture is a two-product tree whose drive snapshot depends on
// full hydration. Stream pb (product-b) carries pb/02, a brief-v1 row whose
// depends: names the unfinished pb/01 in the SAME wave: hydrated, it renders
// blocked; read from its README row alone (no frontmatter attached), the legacy
// wave walk renders it ready. The plan region is spliced from a fully hydrated
// render, so any arm that renders from a partially hydrated or empty board
// disagrees with it.
func scopedDriveRunFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, st := range []struct{ name, serves string }{{"pa", "product-a"}, {"pb", "product-b"}} {
		dir := filepath.Join(root, "docs", "streams", st.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, filepath.Join(dir, "README.md"), "---\n"+
			"stream: "+st.name+"\nstatus: active\npriority: P1\nserves: "+st.serves+"\n---\n\n# "+st.name+"\n\n"+
			"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n"+
			"|---|-------|------|--------|--------|----------|----------|\n"+
			"| 01 | [Base](./brief-01-base.md) | 0 | M | todo | — | — |\n"+
			"| 02 | [Next](./brief-02-next.md) | 0 | M | todo | — | — |\n")
		writeFixtureFile(t, filepath.Join(dir, "brief-01-base.md"), pausedDepBrief(st.name+"/01", "Base", 0, nil))
		writeFixtureFile(t, filepath.Join(dir, "brief-02-next.md"), pausedDepBrief(st.name+"/02", "Next", 0, []string{st.name + "/01"}))
	}
	writeDrive(t, root, "d1", "declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nwhy: fixture\nitems:\n  - stream: pb\n")
	fresh, rc := driveSnapshotSection(root, "d1", driveTestNow)
	if rc != 0 {
		t.Fatalf("fixture drive d1 must render (rc %d)", rc)
	}
	if strings.Contains(fresh, "pb/02") {
		t.Fatalf("fixture drift: hydrated, pb/02 is blocked and off the frontier:\n%s", fresh)
	}
	writePlanMD(t, root, "d1", "# Drive d1\n\n"+driveSnapshotBeginPrefix+"d1 @deadbee 2026-08-15"+driveSnapshotMarkerSuffix+"\n"+fresh+driveSnapshotEndMarker+"\n")
	return root
}

// TestRunDriveLintScopeArms pins run()'s drive-lint arm selection: unscoped
// lint reuses the board run() hydrated, product-scoped lint (pb out of scope,
// so its rows were never hydrated) re-hydrates the full board. Both must
// compare the region clean, and both must still catch a hand-edited region.
// Red under either mutation of main.go: scoped lint rendering from the scoped
// board (pb/02 turns ready: drift PROBLEM), or unscoped lint handed no board
// (the drive no longer resolves: could-not-check NOTICE).
// regression: #2861
func TestRunDriveLintScopeArms(t *testing.T) {
	pinNow(t, driveTestNow)
	stubRemoteBranches(t, nil, nil)
	stubMergedClosedBranches(t, nil, nil)
	root := scopedDriveRunFixture(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	unhydrated, _ := resolveClaims(root, streams)
	ds := loadDrives(root, streams, driveTestNow)
	stale, ok := driveSnapshotRender(ds, streams, unhydrated, "d1", "", driveTestNow)
	fresh, _ := driveSnapshotSection(root, "d1", driveTestNow)
	if !ok || !strings.Contains(stale, "pb/02") || normalizeDriveSnapshot(stale) == normalizeDriveSnapshot(fresh) {
		t.Fatalf("fixture drift: unhydrated, pb/02 must reach the frontier (ok=%v):\n%s", ok, stale)
	}
	// A plain --lint reads no claim set (#3323), so the fresh region's todo briefs
	// are assumed unclaimed: the one drive-region line allowed is that
	// could-not-check, naming exactly them. Any other drive-region line — drift,
	// or a drive that no longer renders — is the comparison going wrong.
	const assumedFree = "NOTICE: drive-region-drift: docs/roadmap/drives/d1.md matches a fresh render only if pb/01, pb/02 are not claimed by an open branch;"
	for _, scope := range []string{"", "product-a"} {
		stderr := captureStderr(t, func() { run(root, "lint", nil, nil, scope) })
		sawAssumed := false
		for _, line := range strings.Split(stderr, "\n") {
			if strings.HasPrefix(line, assumedFree) {
				sawAssumed = true
				continue
			}
			if strings.Contains(line, "drive-region-drift") || strings.Contains(line, "drive-without-manifest") {
				t.Errorf("scope=%q: the fresh region must compare clean, got: %s", scope, line)
			}
		}
		if !sawAssumed {
			t.Errorf("scope=%q: offline, the fresh region must read could-not-check on its unread claims (%q); stderr:\n%s", scope, assumedFree, stderr)
		}
	}
	// Positive control: a hand-edited region is drift under both arms, so the
	// clean verdict above came from a comparison that actually ran.
	writePlanMD(t, root, "d1", "# Drive d1\n\n"+driveSnapshotBeginPrefix+"d1 @deadbee 2026-08-15"+driveSnapshotMarkerSuffix+"\n"+fresh+"hand edit\n"+driveSnapshotEndMarker+"\n")
	for _, scope := range []string{"", "product-a"} {
		stderr := captureStderr(t, func() { run(root, "lint", nil, nil, scope) })
		if !strings.Contains(stderr, "PROBLEM: drive-region-drift: docs/roadmap/drives/d1.md") {
			t.Errorf("scope=%q: a hand-edited region must be drift; stderr:\n%s", scope, stderr)
		}
	}
}

// activeActOnlyManifest is an operator-act-only active drive: it applies and
// renders (WAITING-ON-YOU) without naming any stream, so the lint fixtures need
// no on-disk docs/streams to resolve against.
const activeActOnlyManifest = "declared-by: ian\n" + liveWindow +
	"intensity: focus\nstate: active\nwhy: lint fixture\n" +
	"items:\n  - owner: operator\n    unblocks: sign the release\n    since: \"2026-08-13\"\n"

func writePlanMD(t *testing.T, root, slug, content string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "roadmap", "drives")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasProblemContaining(problems []string, sub string) bool {
	for _, p := range problems {
		if strings.Contains(p, sub) {
			return true
		}
	}
	return false
}

// TestLintDriveRegionDrift (Verify row 7a, mutation): --lint PROBLEMs a
// drive-plan .md whose snapshot region has been hand-edited away from a fresh
// render; an identical region is clean.
func TestLintDriveRegionDrift(t *testing.T) {
	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "d1", activeActOnlyManifest)

	fresh, rc := driveSnapshotSection(root, "d1", driveTestNow)
	if rc != 0 {
		t.Fatalf("fixture drive d1 must render (rc %d)", rc)
	}
	region := driveSnapshotBeginPrefix + "d1 @deadbee 2026-08-15" + driveSnapshotMarkerSuffix + "\n" +
		fresh + driveSnapshotEndMarker + "\n"
	plan := "# Drive d1\n\nprose\n\n" + region

	// Clean: an identical region raises no drift PROBLEM.
	writePlanMD(t, root, "d1", plan)
	if problems, _ := driveRegionLintProblems(root, nil, nil, driveTestNow); hasProblemContaining(problems, "drive-region-drift") {
		t.Fatalf("identical region must not PROBLEM: %v", problems)
	}

	// Drift: one hand-edited cell inside the fence PROBLEMs.
	drifted := strings.Replace(plan, "sign the release", "sign the RELEASE", 1)
	if drifted == plan {
		t.Fatal("mutation target not found")
	}
	writePlanMD(t, root, "d1", drifted)
	problems, _ := driveRegionLintProblems(root, nil, nil, driveTestNow)
	if !hasProblemContaining(problems, "drive-region-drift") {
		t.Fatalf("a hand-edited region must PROBLEM drive-region-drift: %v", problems)
	}
	if !hasProblemContaining(problems, "d1.md") {
		t.Errorf("the PROBLEM must name the drifted plan file: %v", problems)
	}

	// Absent docs/roadmap/drives ⇒ inert.
	empty := t.TempDir()
	if p, n := driveRegionLintProblems(empty, nil, nil, driveTestNow); len(p) != 0 || len(n) != 0 {
		t.Errorf("no drives dir must be inert: problems=%v notices=%v", p, n)
	}
}

// TestLintDriveWithoutManifest (Verify row 7a): --lint PROBLEMs a drive-plan .md
// with no <slug>.yaml manifest beside it.
func TestLintDriveWithoutManifest(t *testing.T) {
	root := t.TempDir()
	makeStreamsDir(t, root)
	writePlanMD(t, root, "orphan", "# Orphan drive\n\nno manifest beside me\n")

	problems, _ := driveRegionLintProblems(root, nil, nil, driveTestNow)
	if !hasProblemContaining(problems, "drive-without-manifest") {
		t.Fatalf("a plan file with no manifest must PROBLEM drive-without-manifest: %v", problems)
	}
	if !hasProblemContaining(problems, "orphan") {
		t.Errorf("the PROBLEM must name the orphan plan file: %v", problems)
	}

	// With the manifest beside it, the drive-without-manifest PROBLEM clears.
	writeDrive(t, root, "orphan", activeActOnlyManifest)
	problems2, _ := driveRegionLintProblems(root, nil, nil, driveTestNow)
	if hasProblemContaining(problems2, "drive-without-manifest") {
		t.Errorf("a manifest beside the plan must clear drive-without-manifest: %v", problems2)
	}
}

// TestLintDrivePlanSymlinkRefused pins the symlink-escape containment (security
// review of PR #638): --lint runs automatically over an attacker-plantable PR
// tree on a public runner, so a plan .md that is a symlink pointing OUT of the
// tree must be REFUSED (a loud PROBLEM) and never followed/read — the same
// surface parseDrive already hardened for the sibling .yaml manifest. It also
// checks a symlinked manifest is refused, and a real in-tree file is still read.
func TestLintDrivePlanSymlinkRefused(t *testing.T) {
	// A secret living OUTSIDE the repo root; if the read followed the symlink its
	// bytes would enter the lint. The refusal must fire before any os.ReadFile.
	outside := t.TempDir()
	secret := filepath.Join(outside, "outside-secret.txt")
	const secretMarker = "TOP-SECRET-OUT-OF-TREE-BYTES"
	if err := os.WriteFile(secret, []byte(secretMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("plan-md-symlink-refused", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		// A valid manifest so the loop reaches the .md read (which is the vuln).
		writeDrive(t, root, "evil", activeActOnlyManifest)
		link := filepath.Join(root, "docs", "roadmap", "drives", "evil.md")
		if err := os.Symlink(secret, link); err != nil {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}
		problems, notices := driveRegionLintProblems(root, nil, nil, driveTestNow)
		if !hasProblemContaining(problems, "symlink escape") {
			t.Fatalf("a symlinked plan .md must PROBLEM a symlink escape: problems=%v", problems)
		}
		if !hasProblemContaining(problems, "evil.md") {
			t.Errorf("the PROBLEM must name the symlinked plan file: %v", problems)
		}
		// Fail-closed: the out-of-tree secret must never be read, so its bytes must
		// appear in NO problem and NO notice.
		for _, m := range append(append([]string{}, problems...), notices...) {
			if strings.Contains(m, secretMarker) {
				t.Fatalf("out-of-tree secret bytes leaked into lint output — symlink was followed: %q", m)
			}
		}
	})

	// ancestor-dir-symlink-refused is the FAIL-FIRST pin for the ancestor-symlink
	// gap the security re-review found: the leaf plan .md is a REGULAR file, but an ANCESTOR directory
	// (docs/roadmap) is a symlink pointing OUT of the tree at a dir that already
	// holds drives/<slug>.md + .yaml. The leaf Lstat cannot see the parent
	// symlink, and a sub-dir-rooted containment resolves both the root and the
	// plan path under the same out-of-tree location so the escape slips through —
	// os.ReadFile then follows out of tree. The containment MUST be rooted at the
	// repo root and reject the symlinked ancestor BEFORE any bytes are read. This
	// subtest goes red against the pre-fix (sub-dir-rooted) reader.
	t.Run("ancestor-dir-symlink-refused", func(t *testing.T) {
		// The out-of-tree drive tree the symlinked ancestor points at: a valid
		// manifest + a plan .md carrying the secret marker.
		otherRoadmap := filepath.Join(outside, "roadmap")
		otherDrives := filepath.Join(otherRoadmap, "drives")
		if err := os.MkdirAll(otherDrives, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(otherDrives, "evil.yaml"), []byte(activeActOnlyManifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(otherDrives, "evil.md"), []byte("# plan\n"+secretMarker+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		root := t.TempDir()
		makeStreamsDir(t, root)
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		// docs/roadmap is a SYMLINK to the out-of-tree roadmap dir. Its leaf
		// (evil.md) is a regular file at the target — only the ancestor is a link.
		if err := os.Symlink(otherRoadmap, filepath.Join(root, "docs", "roadmap")); err != nil {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}

		problems, notices := driveRegionLintProblems(root, nil, nil, driveTestNow)
		if !hasProblemContaining(problems, "symlinked ancestor") {
			t.Fatalf("a symlinked ANCESTOR dir must PROBLEM a symlink escape before any read: problems=%v", problems)
		}
		if !hasProblemContaining(problems, "evil.md") {
			t.Errorf("the PROBLEM must name the plan file reached through the symlinked ancestor: %v", problems)
		}
		// Fail-closed: the out-of-tree bytes must never be read on any path.
		for _, m := range append(append([]string{}, problems...), notices...) {
			if strings.Contains(m, secretMarker) {
				t.Fatalf("out-of-tree secret bytes leaked into lint output — a symlinked ancestor was followed: %q", m)
			}
		}
	})

	t.Run("manifest-symlink-refused", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		writePlanMD(t, root, "evilmf", "# plan\n")
		link := filepath.Join(root, "docs", "roadmap", "drives", "evilmf.yaml")
		if err := os.Symlink(secret, link); err != nil {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}
		problems, _ := driveRegionLintProblems(root, nil, nil, driveTestNow)
		if !hasProblemContaining(problems, "drive-manifest-symlink") {
			t.Fatalf("a symlinked manifest must PROBLEM drive-manifest-symlink: %v", problems)
		}
	})

	t.Run("real-plan-file-still-read", func(t *testing.T) {
		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "d1", activeActOnlyManifest)
		fresh, rc := driveSnapshotSection(root, "d1", driveTestNow)
		if rc != 0 {
			t.Fatalf("fixture drive d1 must render (rc %d)", rc)
		}
		region := driveSnapshotBeginPrefix + "d1 @deadbee 2026-08-15" + driveSnapshotMarkerSuffix + "\n" +
			fresh + driveSnapshotEndMarker + "\n"
		writePlanMD(t, root, "d1", "# Drive d1\n\nprose\n\n"+region)
		// A real, in-tree, identical-region plan file raises no symlink PROBLEM and
		// no drift PROBLEM: the containment guard never blocks legitimate files.
		problems, _ := driveRegionLintProblems(root, nil, nil, driveTestNow)
		if hasProblemContaining(problems, "symlink") {
			t.Errorf("a real in-tree plan file must not PROBLEM a symlink: %v", problems)
		}
		if hasProblemContaining(problems, "drive-region-drift") {
			t.Errorf("an identical real plan file must not drift: %v", problems)
		}
	})
}
