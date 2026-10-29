package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/statusgen/verifyqueue"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// flowQueueRoot is the shared fixture root for the verification-queue contract
// (desk-tools/45). The deskboard and verifyloop tests in tools/desk read the
// golden file beside it, so a drift in the classification is red in every
// consumer, not only here.
const (
	flowQueueRoot   = "testdata/flowqueue"
	flowQueueGolden = "testdata/flowqueue/gate-scores.golden.json"
)

// flowQueueRow is the projection of one --gate-scores row the golden pins: the
// identity and every queue* field, never the score (a time-derived number the
// contract does not own).
type flowQueueRow struct {
	Brief             string `json:"brief"`
	Status            string `json:"status"`
	QueueVersion      int    `json:"queueVersion"`
	QueueClass        string `json:"queueClass"`
	QueueBucket       string `json:"queueBucket,omitempty"`
	QueueOwner        string `json:"queueOwner,omitempty"`
	QueueNextAct      string `json:"queueNextAct,omitempty"`
	QueueGate         string `json:"queueGate,omitempty"`
	QueueRisk         bool   `json:"queueRisk"`
	QueueIrreversible bool   `json:"queueIrreversible"`
	QueueValid        bool   `json:"queueValid"`
	QueueWhy          string `json:"queueWhy,omitempty"`
}

func flowQueueRows(t *testing.T, root string) (string, []flowQueueRow) {
	t.Helper()
	out := captureStdout(t, func() {
		if code := runGateScores(root); code != 0 {
			t.Fatalf("runGateScores(%s) exit %d", root, code)
		}
	})
	var rows []flowQueueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("gate-scores is not JSON: %v\n%q", err, out)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Brief < rows[j].Brief })
	return out, rows
}

// TestFlowQueue pins the exported queue contract: every base bucket, the
// paused/parked placements and the unknown (could-not-check) state map to one
// class; the legacy fields stay first and unchanged; and corrupt input is an
// explicit invalid row, never an empty queue.
// regression: F-queue-contract
func TestFlowQueue(t *testing.T) {
	raw, rows := flowQueueRows(t, flowQueueRoot)

	t.Run("classes", func(t *testing.T) {
		want := map[string]struct{ class, bucket, owner string }{
			"flowq/01":      {queueClassHuman, "human gate", ownerDriver},
			"flowq/02":      {queueClassContinue, "implementer rework", ownerWorker},
			"flowq/03":      {queueClassHuman, "environment-blocked", ownerOperator},
			"flowq/04":      {queueClassContinue, "runner-pending", ownerCIAutoFlip},
			"flowq/05":      {queueClassRunner, "runner-pending", ownerVerifyRunner},
			"flowq/06":      {queueClassJudgment, "desk-actionable", ownerVerifyDesk},
			"flowq/07":      {queueClassJudgment, "desk-actionable", ownerVerifyDesk},
			"flowq/08":      {queueClassUnreadable, "could-not-check", ownerVerifyDesk},
			"flowq/09":      {queueClassUnreadable, "could-not-check", ownerVerifyDesk},
			"flowq/10":      {queueClassHuman, "human gate", ownerDriver},
			"flowpaused/01": {queueClassPaused, "", ""},
			"flowparked/01": {queueClassParked, "", ""},
		}
		if len(rows) != len(want) {
			t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
		}
		for _, r := range rows {
			if !verifyqueue.Supported(r.QueueVersion, r.QueueClass) {
				t.Errorf("producer emitted unsupported contract: %+v", r)
			}
			w, ok := want[r.Brief]
			if !ok {
				t.Errorf("unexpected row %s", r.Brief)
				continue
			}
			if r.QueueVersion != queueContractVersion {
				t.Errorf("%s: queueVersion %d, want %d", r.Brief, r.QueueVersion, queueContractVersion)
			}
			if r.QueueClass != w.class || r.QueueBucket != w.bucket || r.QueueOwner != w.owner {
				t.Errorf("%s: class/bucket/owner = %q/%q/%q, want %q/%q/%q", r.Brief,
					r.QueueClass, r.QueueBucket, r.QueueOwner, w.class, w.bucket, w.owner)
			}
			if wantValid := w.class != queueClassUnreadable; r.QueueValid != wantValid {
				t.Errorf("%s: queueValid %v, want %v", r.Brief, r.QueueValid, wantValid)
			}
			if !r.QueueValid && r.QueueWhy == "" {
				t.Errorf("%s: an invalid classification must say why", r.Brief)
			}
		}
	})

	t.Run("risk and gate facts", func(t *testing.T) {
		byBrief := map[string]flowQueueRow{}
		for _, r := range rows {
			byBrief[r.Brief] = r
		}
		if g := byBrief["flowq/01"]; g.QueueGate != "human" || g.QueueRisk {
			t.Errorf("flowq/01 gate/risk = %q/%v, want human/false", g.QueueGate, g.QueueRisk)
		}
		if g := byBrief["flowq/10"]; !g.QueueRisk || !g.QueueIrreversible || g.QueueGate != "model" {
			t.Errorf("flowq/10 (irreversible, gate model) = %+v", g)
		}
	})

	t.Run("one classifier", func(t *testing.T) {
		// queueFor must be bucketAwaiting's own answer (via placeAwaiting), not a
		// second inference: compare it to placeAwaiting for every awaiting row.
		streams, _, err := loadHydratedStreams(flowQueueRoot)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, s := range streams {
			for i := range s.Briefs {
				br := &s.Briefs[i]
				if br.Status != "implemented" && br.Status != "verified" {
					continue
				}
				n++
				p, q := placeAwaiting(s, br), queueFor(s, br)
				switch {
				case p.paused:
					if q.Class != queueClassPaused {
						t.Errorf("%s/%s: paused placement exported as %q", s.Name, br.Num, q.Class)
					}
				case p.parked:
					if q.Class != queueClassParked {
						t.Errorf("%s/%s: parked placement exported as %q", s.Name, br.Num, q.Class)
					}
				default:
					if q.Bucket != p.bucket.String() || q.Owner != p.owner || q.NextAct != p.nextAct {
						t.Errorf("%s/%s: queueFor %q/%q/%q disagrees with placeAwaiting %q/%q/%q",
							s.Name, br.Num, q.Bucket, q.Owner, q.NextAct, p.bucket, p.owner, p.nextAct)
					}
				}
			}
		}
		if n != len(rows) {
			t.Errorf("walked %d awaiting briefs, --gate-scores emitted %d", n, len(rows))
		}
	})

	t.Run("legacy fields compatible", func(t *testing.T) {
		// The legacy fields keep their names and come FIRST, in order, so a
		// reader that scans the row prefix or decodes the old struct is unmoved.
		var generic []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &generic); err != nil || len(generic) == 0 {
			t.Fatalf("raw rows: %v", err)
		}
		line := string(generic[0])
		last := -1
		for _, k := range []string{`"brief"`, `"score"`, `"blockedCount"`, `"stream"`, `"status"`, `"queueVersion"`, `"queueClass"`} {
			at := strings.Index(line, k)
			if at < 0 || at < last {
				t.Fatalf("field %s missing or out of order in %s", k, line)
			}
			last = at
		}
		var legacy []gateScoreRow
		if err := json.Unmarshal([]byte(raw), &legacy); err != nil || len(legacy) != len(rows) {
			t.Fatalf("legacy decode: %v (%d rows)", err, len(legacy))
		}
		for i := 1; i < len(legacy); i++ {
			if legacy[i-1].Score < legacy[i].Score {
				t.Errorf("rows are no longer score-descending at %d: %d < %d", i, legacy[i-1].Score, legacy[i].Score)
			}
		}
	})

	t.Run("corrupt input is not an empty queue", func(t *testing.T) {
		// An unparseable outcome store turns EVERY non-paused/parked row
		// unreadable: the queue stays full of explicit unknowns.
		tmp := t.TempDir()
		copyTree(t, flowQueueRoot, tmp)
		writeOutcomeRecord(t, tmp, "flowq", "zz-20261001T000000Z-bbbbbbbbbbbb.json", `{not json`)
		_, bad := flowQueueRows(t, tmp)
		if len(bad) != len(rows) {
			t.Fatalf("corrupt store changed the row count: %d -> %d (a queue must not shrink to empty)", len(rows), len(bad))
		}
		for _, r := range bad {
			if strings.HasPrefix(r.Brief, "flowq/") && (r.QueueClass != queueClassUnreadable || r.QueueValid) {
				t.Errorf("%s: unparseable store left class %q valid=%v", r.Brief, r.QueueClass, r.QueueValid)
			}
		}
	})

	t.Run("golden", func(t *testing.T) {
		got, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, '\n')
		if os.Getenv("FLOWQUEUE_UPDATE") == "1" {
			if err := os.WriteFile(flowQueueGolden, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(filepath.FromSlash(flowQueueGolden))
		if err != nil {
			t.Fatalf("golden unreadable (FLOWQUEUE_UPDATE=1 regenerates it): %v", err)
		}
		if string(want) != string(got) {
			t.Errorf("gate-scores queue golden drifted (FLOWQUEUE_UPDATE=1 regenerates it)\n got: %s\nwant: %s", got, want)
		}
	})
}
