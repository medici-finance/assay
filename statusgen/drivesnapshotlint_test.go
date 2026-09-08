package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if problems, _ := driveRegionLintProblems(root, driveTestNow); hasProblemContaining(problems, "drive-region-drift") {
		t.Fatalf("identical region must not PROBLEM: %v", problems)
	}

	// Drift: one hand-edited cell inside the fence PROBLEMs.
	drifted := strings.Replace(plan, "sign the release", "sign the RELEASE", 1)
	if drifted == plan {
		t.Fatal("mutation target not found")
	}
	writePlanMD(t, root, "d1", drifted)
	problems, _ := driveRegionLintProblems(root, driveTestNow)
	if !hasProblemContaining(problems, "drive-region-drift") {
		t.Fatalf("a hand-edited region must PROBLEM drive-region-drift: %v", problems)
	}
	if !hasProblemContaining(problems, "d1.md") {
		t.Errorf("the PROBLEM must name the drifted plan file: %v", problems)
	}

	// Absent docs/roadmap/drives ⇒ inert.
	empty := t.TempDir()
	if p, n := driveRegionLintProblems(empty, driveTestNow); len(p) != 0 || len(n) != 0 {
		t.Errorf("no drives dir must be inert: problems=%v notices=%v", p, n)
	}
}

// TestLintDriveWithoutManifest (Verify row 7a): --lint PROBLEMs a drive-plan .md
// with no <slug>.yaml manifest beside it.
func TestLintDriveWithoutManifest(t *testing.T) {
	root := t.TempDir()
	makeStreamsDir(t, root)
	writePlanMD(t, root, "orphan", "# Orphan drive\n\nno manifest beside me\n")

	problems, _ := driveRegionLintProblems(root, driveTestNow)
	if !hasProblemContaining(problems, "drive-without-manifest") {
		t.Fatalf("a plan file with no manifest must PROBLEM drive-without-manifest: %v", problems)
	}
	if !hasProblemContaining(problems, "orphan") {
		t.Errorf("the PROBLEM must name the orphan plan file: %v", problems)
	}

	// With the manifest beside it, the drive-without-manifest PROBLEM clears.
	writeDrive(t, root, "orphan", activeActOnlyManifest)
	problems2, _ := driveRegionLintProblems(root, driveTestNow)
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
		problems, notices := driveRegionLintProblems(root, driveTestNow)
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

		problems, notices := driveRegionLintProblems(root, driveTestNow)
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
		problems, _ := driveRegionLintProblems(root, driveTestNow)
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
		problems, _ := driveRegionLintProblems(root, driveTestNow)
		if hasProblemContaining(problems, "symlink") {
			t.Errorf("a real in-tree plan file must not PROBLEM a symlink: %v", problems)
		}
		if hasProblemContaining(problems, "drive-region-drift") {
			t.Errorf("an identical real plan file must not drift: %v", problems)
		}
	})
}
