package deskkit

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

// setReleaseTag stamps the binary's release tag for one test, as release.yml's -X does.
func setReleaseTag(t *testing.T, tag string) {
	t.Helper()
	old := ReleaseTag
	ReleaseTag = tag
	t.Cleanup(func() { ReleaseTag = old })
}

// TestPerfRunNamesReleaseTag: a release build's invocation record names the tag the binary
// was stamped with (perf-v5), beside its commit stamp, and reads back in the perf-v2 group.
// regression: #6141
func TestPerfRunNamesReleaseTag(t *testing.T) {
	state := perfFixture(t)
	setReleaseTag(t, "v1.0.82")
	PerfRun("deskpr", []string{"create"}, []string{"create"}, func([]string) int { return 0 })
	recs, raw := perfLines(t, state)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d:\n%s", len(recs), raw)
	}
	if recs[0]["schema"] != PerfSchemaV5 || recs[0]["releaseTag"] != "v1.0.82" {
		t.Errorf("record = %v; want %s with releaseTag v1.0.82", recs[0], PerfSchemaV5)
	}
	if _, ok := recs[0]["sourceSHA"]; !ok {
		t.Errorf("record dropped sourceSHA: %v", recs[0])
	}
	r, err := perfrec.Decode([]byte(strings.TrimSpace(string(raw))))
	if err != nil || !IsPerfV2(r) || IsPerfStep(r) || IsPerfLegacyInvocation(r) {
		t.Errorf("perf-v5 line reads as %+v, %v; want one perf-v2-group invocation", r, err)
	}
}

// TestPerfRunUntaggedStaysV2: a dev or unstamped build, or a stamp that is not a release tag,
// writes the unchanged perf-v2 record with no releaseTag — no tag is guessed, and a hostile
// stamp never reaches the line.
func TestPerfRunUntaggedStaysV2(t *testing.T) {
	for _, tag := range []string{"", "dev", "1a43034839", "https://forge.invalid/rel?token=fake-sentinel-stamp"} {
		t.Run(tag, func(t *testing.T) {
			state := perfFixture(t)
			setReleaseTag(t, tag)
			PerfRun("deskpr", []string{"create"}, []string{"create"}, func([]string) int { return 0 })
			recs, raw := perfLines(t, state)
			if len(recs) != 1 || recs[0]["schema"] != PerfSchemaV2 {
				t.Fatalf("records = %v; want one %s record", recs, PerfSchemaV2)
			}
			if _, has := recs[0]["releaseTag"]; has {
				t.Errorf("an untagged build wrote releaseTag: %s", raw)
			}
			if strings.Contains(string(raw), "sentinel") || strings.Contains(string(raw), "forge.invalid") {
				t.Errorf("the stamp reached the record: %s", raw)
			}
		})
	}
}
