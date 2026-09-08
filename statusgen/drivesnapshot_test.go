package main

import (
	"strings"
	"testing"
)

// --- Verify row 1 -------------------------------------------------------------

// snapshotFixture builds an active drive (a resolvable stream item + two
// operator acts so the WAITING-ON-YOU table renders) loaded through the REAL
// loadDrives + driveStatuses path, and returns the rendered snapshot section.
func snapshotFixture(t *testing.T) (streams []*Stream, ds DriveSet, section string) {
	t.Helper()
	streams = []*Stream{driveBriefStream("demo", 4)}
	root := t.TempDir()
	writeDrive(t, root, "demo-drive",
		"declared-by: ian\n"+liveWindow+"intensity: focus\nstate: active\nwhy: prove the snapshot renders\n"+
			"items:\n"+
			"  - stream: demo\n"+
			"  - owner: operator\n    unblocks: sign the release\n    since: \"2026-08-13\"\n"+
			"  - owner: operator\n    unblocks: grant the token\n    since: \"2026-08-15\"\n")
	ds = loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("fixture drive must apply: %+v", ds)
	}
	const heartbeat = "abc1234 2026-08-15T00:00:00+00:00"
	sec, ok := driveSnapshotRender(ds, streams, nil, "demo-drive", heartbeat, driveTestNow)
	if !ok {
		t.Fatalf("driveSnapshotRender found no drive %q", "demo-drive")
	}
	return streams, ds, sec
}

// TestDriveSnapshot (Verify row 1): `--drive-snapshot <slug>` renders EXACTLY the
// `## Drive: <slug>` dashboard section drivedash writes into STATUS.md — the
// operator slice FIRST (state banner + heartbeat, then WAITING-ON-YOU), then
// progress, in-flight, blocked-on-review, frontier-next.
func TestDriveSnapshot(t *testing.T) {
	_, _, sec := snapshotFixture(t)

	order := []string{
		"## Drive: `demo-drive`",
		"**State:**",
		"_last regen: abc1234 2026-08-15T00:00:00+00:00_",
		"**Waiting on you:**",
		"| Act | Unblocks | Age | Issue |",
		"**In-flight:**",
		"**Blocked on review:**",
		"**Frontier next:**",
	}
	last := -1
	for _, want := range order {
		idx := strings.Index(sec, want)
		if idx < 0 {
			t.Fatalf("snapshot lacks %q:\n%s", want, sec)
		}
		if idx <= last {
			t.Errorf("%q out of dashboard order (idx %d <= %d):\n%s", want, idx, last, sec)
		}
		last = idx
	}
	// Oldest operator act first, with its age; the fresh act references the
	// tracking issue.
	if !strings.Contains(sec, "| act 1 | sign the release | 2 days |") {
		t.Errorf("oldest act row (2 days) missing:\n%s", sec)
	}
	if !strings.Contains(sec, "| act 2 | grant the token | today | tracking issue |") {
		t.Errorf("younger act row (today) missing:\n%s", sec)
	}
	// The render is driveDashboardSection VERBATIM — no snapshot-only banner is
	// added to it (the "add nothing to the render" invariant).
	if strings.Contains(sec, "DRIVE NOT APPLIED") {
		t.Errorf("snapshot must never carry a NOT APPLIED banner for an applied drive:\n%s", sec)
	}
	// Absent-is-inert: an unknown slug renders nothing.
	if _, ok := driveSnapshotRender(loadDrives(t.TempDir(), nil, driveTestNow), nil, nil, "no-such", "hb", driveTestNow); ok {
		t.Errorf("an unknown slug must not render")
	}
}

// --- Verify row 2 (check + mutation) -----------------------------------------

// planFileWith wraps a rendered section in the fenced drive-snapshot region,
// with surrounding narrative that --check must never touch or compare.
func planFileWith(slug, provenance, section string) string {
	return "# Drive: demo\n\nNarrative prose above the fence — never compared.\n\n" +
		driveSnapshotBeginPrefix + slug + " " + provenance + driveSnapshotMarkerSuffix + "\n" +
		section +
		driveSnapshotEndMarker + "\n\n_Narrative below the fence — never compared._\n"
}

// TestDriveSnapshotCheckDetectsEdit (Verify row 2): `--check` exits 0 on an
// identical region, 1 when one cell of the region is hand-edited, 2 when the
// file carries no region. The heartbeat provenance line is NOT counted as drift.
func TestDriveSnapshotCheckDetectsEdit(t *testing.T) {
	_, _, section := snapshotFixture(t)
	const slug = "demo-drive"

	// 0 — identical region spliced verbatim.
	identical := planFileWith(slug, "@abc1234 2026-08-15", section)
	if v := driveRegionVerdict(section, identical, slug); v != 0 {
		t.Fatalf("identical region: verdict %d, want 0", v)
	}

	// 1 — one cell of the WAITING-ON-YOU table hand-edited inside the fence.
	edited := strings.Replace(identical, "grant the token", "grant the TOKEN", 1)
	if edited == identical {
		t.Fatal("mutation did not change the fixture — the edit target moved")
	}
	if v := driveRegionVerdict(section, edited, slug); v != 1 {
		t.Fatalf("hand-edited cell: verdict %d, want 1 (drift)", v)
	}

	// 1 — an edit OUTSIDE the fence must NOT read as drift (only the region is
	// compared).
	outside := strings.Replace(identical, "Narrative prose above the fence — never compared.", "Rewritten narrative.", 1)
	if v := driveRegionVerdict(section, outside, slug); v != 0 {
		t.Errorf("an edit outside the region must not be drift: verdict %d, want 0", v)
	}

	// 0 — a DIFFERENT heartbeat in the region is provenance churn, not drift.
	churned := strings.Replace(identical,
		"_last regen: abc1234 2026-08-15T00:00:00+00:00_",
		"_last regen: 9f9f9f9 2026-09-08T12:00:00+00:00_", 1)
	if churned == identical {
		t.Fatal("heartbeat line not found to churn")
	}
	if v := driveRegionVerdict(section, churned, slug); v != 0 {
		t.Errorf("a heartbeat-only difference must not be drift: verdict %d, want 0", v)
	}

	// 2 — a file with no region for this slug is could-not-check.
	noRegion := "# Drive: demo\n\nJust prose, no fence.\n"
	if v := driveRegionVerdict(section, noRegion, slug); v != 2 {
		t.Errorf("file without a region: verdict %d, want 2 (could-not-check)", v)
	}
	// 2 — a region for a DIFFERENT slug is not this drive's region.
	otherSlug := planFileWith("other-drive", "@x 2026-08-15", section)
	if v := driveRegionVerdict(section, otherSlug, slug); v != 2 {
		t.Errorf("region for another slug: verdict %d, want 2", v)
	}
}
