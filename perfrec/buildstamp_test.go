package perfrec

import (
	"runtime/debug"
	"strings"
	"testing"
)

// testRev is a full-width (40 hex) revision, built rather than spelled out.
var testRev = strings.Repeat("0123456789abcdef", 3)[:40]

func infoWith(settings ...debug.BuildSetting) BuildInfoFunc {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: settings}, true
	}
}

// TestRevisionFallback pins the resolution order: an explicit link-time stamp wins; an empty
// stamp falls back to the toolchain's vcs.revision (shortened); a binary with no usable VCS
// information resolves to "" so the record's own "unpinned" substitution still applies.
// regression: F-release-binary-unpinned
func TestRevisionFallback(t *testing.T) {
	vcs := infoWith(
		debug.BuildSetting{Key: "vcs", Value: "git"},
		debug.BuildSetting{Key: "vcs.revision", Value: testRev},
		debug.BuildSetting{Key: "vcs.time", Value: "2026-10-20T01:32:51Z"},
		debug.BuildSetting{Key: "vcs.modified", Value: "true"},
	)
	noInfo := func() (*debug.BuildInfo, bool) { return nil, false }
	for _, c := range []struct {
		name  string
		stamp string
		read  BuildInfoFunc
		want  string
	}{
		{"stamp wins over vcs", "abc1234", vcs, "abc1234"},
		{"malformed stamp still wins", "v1.0.52", vcs, "v1.0.52"},
		{"empty stamp uses vcs", "", vcs, testRev[:RevisionLen]},
		{"sha256 revision", "", infoWith(debug.BuildSetting{Key: "vcs.revision", Value: testRev + testRev[:24]}), testRev[:RevisionLen]},
		{"no build info", "", noInfo, ""},
		{"nil reader", "", nil, ""},
		{"no vcs settings", "", infoWith(debug.BuildSetting{Key: "-buildmode", Value: "exe"}), ""},
		{"non-hex revision", "", infoWith(debug.BuildSetting{Key: "vcs.revision", Value: strings.Repeat("z", 40)}), ""},
		{"short revision", "", infoWith(debug.BuildSetting{Key: "vcs.revision", Value: "abc1234"}), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := RevisionFrom(c.stamp, c.read); got != c.want {
				t.Errorf("RevisionFrom(%q) = %q, want %q", c.stamp, got, c.want)
			}
		})
	}
	// End to end through the record's own gate: an empty stamp on a VCS build records the
	// revision, and on a build with no VCS information it records "unpinned".
	if got := SourceSHA(RevisionFrom("", vcs)); got != testRev[:RevisionLen] {
		t.Errorf("SourceSHA(vcs fallback) = %q, want the short revision", got)
	}
	if got := SourceSHA(RevisionFrom("", noInfo)); got != Unpinned {
		t.Errorf("SourceSHA(no vcs) = %q, want %q", got, Unpinned)
	}
}

// TestBuildTimeFallback pins the same order for the build time: stamp, then vcs.time
// normalised to RFC 3339 UTC, then "".
// regression: F-release-binary-unpinned
func TestBuildTimeFallback(t *testing.T) {
	for _, c := range []struct {
		name  string
		stamp string
		read  BuildInfoFunc
		want  string
	}{
		{"stamp wins", "2026-07-10T00:00:00Z", infoWith(debug.BuildSetting{Key: "vcs.time", Value: "2026-10-20T01:32:51Z"}), "2026-07-10T00:00:00Z"},
		{"empty stamp uses vcs.time", "", infoWith(debug.BuildSetting{Key: "vcs.time", Value: "2026-10-20T03:32:51+02:00"}), "2026-10-20T01:32:51Z"},
		{"no vcs.time", "", infoWith(), ""},
		{"unparseable vcs.time", "", infoWith(debug.BuildSetting{Key: "vcs.time", Value: "yesterday"}), ""},
		{"no build info", "", func() (*debug.BuildInfo, bool) { return nil, false }, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := BuildTimeFrom(c.stamp, c.read); got != c.want {
				t.Errorf("BuildTimeFrom(%q) = %q, want %q", c.stamp, got, c.want)
			}
		})
	}
}
