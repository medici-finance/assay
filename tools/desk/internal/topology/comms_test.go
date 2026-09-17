package topology

import (
	"strings"
	"testing"
)

// comms_test.go — the `comms:` key half of the loader contract: the
// 2026-09-17 human ruling on this key's decision-gate issue chose Option 2,
// "Interim rung first", over the recorded full-enable target.
//
// THE KEY DECISIONS THIS FILE PINS, mirroring cellmodel_test.go's own structure
// for the sibling `relationship:` key:
//
//	absent comms            => CommsDisabled (zero value). Never a default-on
//	                            guess — mirrors every other fail-closed zero in
//	                            this package (VisibilityUnknown,
//	                            RelationshipUpstream).
//	unrecognised comms       => parse ERROR naming the line. There is no
//	                            best-effort reading of a typo.
//	`interim` / `full`       => the only two live modes, parsed exactly.
//
// THIS KEY IS INERT BY ITSELF. Nothing in this package (or in this test file)
// wires CommsMode into commsgw/commsloop's own execution — that independence
// is the point (CommsMode's doc, docs/acp-cell-comms-spec.md §8/§9): a defect
// in this loader can misparse the topology file, but it can never, by itself,
// cause a message to autonomously fire a session. That would need a SECOND,
// independent defect in the ASSAY_COMMS_* env gate or the deployment step.

// commsFixture is a minimal but COMPLETE topology-v1 document carrying an
// optional `comms:` line, mirroring cellmodel_test.go's cellFixture shape
// (Parse refuses a source missing `repos:`/`labels.system_state`, so the
// fixture stays complete even when the field under test is absent).
func commsFixture(commsLine string) string {
	return "schema: topology-v1\n" +
		"cell: platform\n" +
		commsLine +
		"repos:\n" +
		"  - slug: example-org/tracker\n" +
		"    visibility: public\n" +
		"labels:\n" +
		"  system_state:\n" +
		"    - name: verify-gate\n" +
		"      why: a closeable state the machinery emits\n" +
		"  decision_owed:\n" +
		"    - name: question\n" +
		"      why: the escalation label\n"
}

func TestTopologyComms(t *testing.T) {
	t.Run("absent comms reads as disabled, not a parse error", func(t *testing.T) {
		got, err := Parse([]byte(commsFixture("")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: a source omitting `comms:` failed to parse: %v", err)
		}
		if got.Comms != CommsDisabled {
			t.Errorf("Comms: got %s, want disabled — absence must never read as enabled", got.Comms)
		}
		if got.CommsEnabled() {
			t.Errorf("CommsEnabled(): got true for an absent key, want false")
		}
	})

	t.Run("a pre-comms-key topology-v1 file still loads (additive, no version bump)", func(t *testing.T) {
		// The shape of every file written before this key existed: no `comms:`
		// anywhere. Additive fields must not break old files, the same claim
		// cellmodel_test.go's own equivalent subtest makes for `cell`/`relationship`.
		got, err := Parse([]byte(cellFixture("", "")))
		if err != nil {
			t.Fatalf("a topology-v1 file predating the comms key no longer parses: %v", err)
		}
		if got.Comms != CommsDisabled {
			t.Errorf("Comms: got %s on a file that never mentions comms, want disabled", got.Comms)
		}
	})

	t.Run("comms: interim parses to CommsInterim and reports enabled", func(t *testing.T) {
		got, err := Parse([]byte(commsFixture("comms: interim\n")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: `comms: interim` failed to parse: %v", err)
		}
		if got.Comms != CommsInterim {
			t.Errorf("Comms: got %s, want interim", got.Comms)
		}
		if !got.CommsEnabled() {
			t.Errorf("CommsEnabled(): got false for comms: interim, want true")
		}
		if got.Comms.String() != "interim" {
			t.Errorf("Comms.String(): got %q, want %q", got.Comms.String(), "interim")
		}
	})

	t.Run("comms: full parses to CommsFull and reports enabled", func(t *testing.T) {
		got, err := Parse([]byte(commsFixture("comms: full\n")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: `comms: full` failed to parse: %v", err)
		}
		if got.Comms != CommsFull {
			t.Errorf("Comms: got %s, want full", got.Comms)
		}
		if !got.CommsEnabled() {
			t.Errorf("CommsEnabled(): got false for comms: full, want true")
		}
	})

	t.Run("an unrecognised comms value is an error naming the line", func(t *testing.T) {
		_, err := Parse([]byte(commsFixture("comms: enabled\n")))
		requireParseError(t, err, "unrecognised comms value", "enabled", "neither `interim` nor `full`", "line 3")
	})

	t.Run("an empty comms value is an error, not a silent default", func(t *testing.T) {
		_, err := Parse([]byte(commsFixture("comms: \"\"\n")))
		requireParseError(t, err, "empty comms value", "neither `interim` nor `full`", "line 3")
	})

	t.Run("this tree's own source states comms: interim, not the full-enable target", func(t *testing.T) {
		// The fixtures above prove the LOADER; this proves the FILE — the same
		// split cellmodel_test.go's own "this tree's own source" subtest draws.
		src := loadSourceOrFail(t)
		if src.Comms != CommsInterim {
			t.Errorf("%s states comms: %s, want interim — the ruled decision was Option 2 "+
				"(interim rung), never full enablement", SourceFile, src.Comms)
		}
	})
}

// TestTopologyCommsPositiveControl proves the comms checks can fail, mirroring
// TestTopologyCellModelPositiveControl's split: parse refusals that discriminate,
// and a drift comparison that actually looks at the field.
func TestTopologyCommsPositiveControl(t *testing.T) {
	t.Run("the refusals discriminate — the same fixtures parse when made valid", func(t *testing.T) {
		valid := []struct {
			name string
			src  string
		}{
			{"no comms at all", commsFixture("")},
			{"comms: interim", commsFixture("comms: interim\n")},
			{"comms: full", commsFixture("comms: full\n")},
		}
		for _, v := range valid {
			if _, err := Parse([]byte(v.src)); err != nil {
				t.Errorf("POSITIVE CONTROL FAILED: %s should PARSE, got: %v", v.name, err)
			}
		}
	})

	t.Run("the drift comparison catches a bent comms mode", func(t *testing.T) {
		src := loadSourceOrFail(t)
		bent := Compiled()
		if bent.Comms == CommsFull {
			bent.Comms = CommsInterim
		} else {
			bent.Comms = CommsFull
		}
		diffs := topologyDiffs(src, bent)
		if len(diffs) == 0 {
			t.Fatal("POSITIVE CONTROL FAILED: bending the derivation's comms mode produced NO diff " +
				"against the declared source — a derivation could silently claim `full` while the " +
				"source states `interim` (or vice versa), which is exactly the drift this exists to catch.")
		}
		if !strings.Contains(strings.Join(diffs, "\n"), "comms:") {
			t.Errorf("POSITIVE CONTROL FAILED: the diff fired but did not name `comms`: %v", diffs)
		}
	})
}
