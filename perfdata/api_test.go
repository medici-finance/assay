package perfdata_test

// api_test.go — the published collector input and acknowledgment contract. Each case in
// testdata/api is a batch (<case>.batch.json, the exact bytes a sender transmits) and a
// case file (<case>.case.json: the collector-side context and the expected receipt).
// No server exists here; a database, server or sender implementation is conformant when
// it returns the same receipt for every case. The expectations below are written by
// hand, so a fixture regenerated with -update must still match them.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

// apiContext is the serialized collector-side state of one case.
type apiContext struct {
	AuthenticatedSource string             `json:"authenticatedSource"`
	ConsentRevision     string             `json:"consentRevision"`
	AllowedProfiles     []perfdata.Profile `json:"allowedProfiles"`
	Now                 string             `json:"now"`
	ReplayHorizonHours  int                `json:"replayHorizonHours"`
	Known               map[string]string  `json:"known"`
	Commit              string             `json:"commit"` // "ok" or "fail"
}

type apiCase struct {
	Description string           `json:"description"`
	Context     apiContext       `json:"context"`
	Receipt     perfdata.Receipt `json:"receipt"`
}

// want is the hand-written expectation for one case.
type want struct {
	status, reason string
	committed      bool
	results        []string // per-event status, in batch order
	classes        map[int]perfdata.RefusalClass
	commitCalls    int
	committedIdx   []int // which batch events the store receives
}

const otherSource = "src-bbbbbbbbbbbbbbbbbbbbbbbbbb"

func apiEvent(t *testing.T, seq uint64, wall int64, recorded time.Time, src string, p perfdata.Profile) []byte {
	t.Helper()
	_, b, err := perfdata.Seal(perfdata.Draft{Kind: perfdata.KindTool, Payload: toolPayload("statusgen", wall, 1)},
		perfdata.Identity{Source: src, Epoch: 1, Seq: seq}, p, "r1", testBuild, &perfdata.Context{Invocation: fixedAlias(byte('f' + seq))}, recorded)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// batchBytes renders a batch with one event per line; event bytes are kept verbatim.
func batchBytes(src, consent string, p perfdata.Profile, events ...[]byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `{"schema":%q,"source":%q,"consentRevision":%q,"profile":%q,"events":[`, perfdata.BatchSchema, src, consent, p)
	for i, e := range events {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
		buf.Write(e)
	}
	buf.WriteString("\n]}\n")
	return buf.Bytes()
}

func TestAPIContractFixtures(t *testing.T) {
	now := t0.Add(24 * time.Hour)
	e1 := apiEvent(t, 1, 10, t0, testSource, perfdata.ProfileTool)
	e2 := apiEvent(t, 2, 20, t0, testSource, perfdata.ProfileTool)
	e2conflict := apiEvent(t, 2, 999, t0, testSource, perfdata.ProfileTool) // same identity, different record
	old := apiEvent(t, 3, 30, t0.Add(-10*24*time.Hour), testSource, perfdata.ProfileTool)
	foreign := apiEvent(t, 4, 40, t0, otherSource, perfdata.ProfileTool)
	tampered := bytes.Replace(apiEvent(t, 5, 50, t0, testSource, perfdata.ProfileTool), []byte(`"seq":5`), []byte(`"seq":6`), 1)
	id := func(seq uint64) string { return perfdata.EventID(testSource, 1, seq) }

	base := apiContext{AuthenticatedSource: testSource, ConsentRevision: "r1", AllowedProfiles: []perfdata.Profile{perfdata.ProfileTool},
		Now: perfdata.FormatTime(now), ReplayHorizonHours: 7 * 24, Known: map[string]string{}, Commit: "ok"}
	with := func(f func(c *apiContext)) apiContext {
		c := base
		c.Known = map[string]string{}
		f(&c)
		return c
	}
	tool := perfdata.ProfileTool

	cases := []struct {
		name, description string
		ctx               apiContext
		batch             []byte
		want              want
	}{
		{"committed", "Every event is new; the store commits them and the receipt acknowledges each one.",
			base, batchBytes(testSource, "r1", tool, e1, e2),
			want{status: perfdata.ReceiptCommitted, committed: true, results: []string{"acknowledged", "acknowledged"}, commitCalls: 1, committedIdx: []int{0, 1}}},
		{"identical-duplicate", "A resend after a lost receipt: an already-committed identical event is a duplicate (success, not stored again).",
			with(func(c *apiContext) { c.Known[id(1)] = perfdata.Digest(e1) }), batchBytes(testSource, "r1", tool, e1, e2),
			want{status: perfdata.ReceiptCommitted, committed: true, results: []string{"duplicate", "acknowledged"}, commitCalls: 1, committedIdx: []int{1}}},
		{"replay-all-duplicate", "A full replay of a committed batch commits nothing new and still reports committed.",
			with(func(c *apiContext) { c.Known[id(1)], c.Known[id(2)] = perfdata.Digest(e1), perfdata.Digest(e2) }), batchBytes(testSource, "r1", tool, e1, e2),
			want{status: perfdata.ReceiptCommitted, committed: true, results: []string{"duplicate", "duplicate"}}},
		{"conflicting-identity", "The same event identity with a different record is a conflict, never an overwrite; the rest commits (partial).",
			with(func(c *apiContext) { c.Known[id(2)] = perfdata.Digest(e2conflict) }), batchBytes(testSource, "r1", tool, e1, e2),
			want{status: perfdata.ReceiptPartial, committed: true, results: []string{"acknowledged", "conflicting"}, commitCalls: 1, committedIdx: []int{0}}},
		{"conflict-within-batch", "Two records with one identity inside one batch: the first is admitted, the second conflicts.",
			base, batchBytes(testSource, "r1", tool, e2, e2conflict),
			want{status: perfdata.ReceiptPartial, committed: true, results: []string{"acknowledged", "conflicting"}, commitCalls: 1, committedIdx: []int{0}}},
		{"partial-refused-event", "A malformed (non-canonical) event and an event claiming another source are refused by class; the valid event commits.",
			base, batchBytes(testSource, "r1", tool, e1, tampered, foreign),
			want{status: perfdata.ReceiptPartial, committed: true, results: []string{"acknowledged", "rejected", "rejected"},
				classes: map[int]perfdata.RefusalClass{2: perfdata.RefusePrivacy}, commitCalls: 1, committedIdx: []int{0}}},
		{"outside-replay-horizon", "An event recorded before now minus the replay horizon is not admitted; the sender treats it as expired loss.",
			base, batchBytes(testSource, "r1", tool, e1, old),
			want{status: perfdata.ReceiptPartial, committed: true, results: []string{"acknowledged", "outside-horizon"}, commitCalls: 1, committedIdx: []int{0}}},
		{"none-admitted", "When no event is admitted the batch is rejected and nothing is committed.",
			base, batchBytes(testSource, "r1", tool, old),
			want{status: perfdata.ReceiptRejected, reason: perfdata.ReasonNoneAdmitted, results: []string{"outside-horizon"}}},
		{"commit-failed", "Acknowledged means committed: when the store fails, nothing is acknowledged and the sender resends unchanged.",
			with(func(c *apiContext) { c.Commit = "fail"; c.Known[id(1)] = perfdata.Digest(e1) }), batchBytes(testSource, "r1", tool, e1, e2),
			want{status: perfdata.ReceiptRetry, reason: perfdata.ReasonCommitFailed, results: []string{"duplicate", "retry"}, commitCalls: 1}},
		{"source-not-authenticated", "The batch's source must equal the source bound to the caller's credential.",
			base, batchBytes(otherSource, "r1", tool, foreign),
			want{status: perfdata.ReceiptRejected, reason: perfdata.ReasonSourceBinding, results: []string{}}},
		{"consent-revision-mismatch", "A batch recorded under another consent revision is rejected whole.",
			base, batchBytes(testSource, "r0", tool, e1),
			want{status: perfdata.ReceiptRejected, reason: perfdata.ReasonConsentRevision, results: []string{}}},
		{"profile-not-allowed", "A profile the collector policy does not admit is rejected whole.",
			with(func(c *apiContext) { c.AllowedProfiles = []perfdata.Profile{perfdata.ProfileWorkflow} }), batchBytes(testSource, "r1", tool, e1),
			want{status: perfdata.ReceiptRejected, reason: perfdata.ReasonProfile, results: []string{}}},
		{"unknown-field", "Unknown batch fields are refused, never ignored.",
			base, []byte(`{"schema":"performance-batch-v1","source":"` + testSource + `","consentRevision":"r1","profile":"tool","events":[],"note":"x"}` + "\n"),
			want{status: perfdata.ReceiptRejected, reason: perfdata.ReasonSchema, results: []string{}}},
	}

	dir := filepath.Join("testdata", "api")
	if *update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		seen[tc.name] = true
		t.Run(tc.name, func(t *testing.T) {
			batchPath, casePath := filepath.Join(dir, tc.name+".batch.json"), filepath.Join(dir, tc.name+".case.json")
			if *update {
				if err := os.WriteFile(batchPath, tc.batch, 0o644); err != nil {
					t.Fatal(err)
				}
				ac, _ := admit(t, tc.batch, tc.ctx)
				b, _ := json.MarshalIndent(apiCase{Description: tc.description, Context: tc.ctx, Receipt: ac}, "", "  ")
				if err := os.WriteFile(casePath, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(batchPath)
			if err != nil {
				t.Fatal(err)
			}
			cb, err := os.ReadFile(casePath)
			if err != nil {
				t.Fatal(err)
			}
			var fc apiCase
			dec := json.NewDecoder(bytes.NewReader(cb))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&fc); err != nil {
				t.Fatal(err)
			}
			got, stored := admit(t, raw, fc.Context)
			if !reflect.DeepEqual(got, fc.Receipt) {
				t.Fatalf("receipt drifted from the published fixture:\n got  %+v\n want %+v", got, fc.Receipt)
			}
			checkWant(t, got, stored, raw, tc.want)
		})
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		name := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".batch.json"), ".case.json")
		if !seen[name] {
			t.Errorf("unexpected fixture %s has no expectation", e.Name())
		}
	}

	t.Run("planted: a receipt that acknowledges before commit is caught", func(t *testing.T) {
		got, stored := admit(t, cases[8].batch, cases[8].ctx)
		got.Status, got.Committed = perfdata.ReceiptCommitted, true
		got.Results[1].Status = perfdata.ResultAcknowledged
		if !wantFails(func(tb testing.TB) { checkWant(tb, got, stored, cases[8].batch, cases[8].want) }) {
			t.Fatal("control: a pre-commit acknowledgment passed the expectation check")
		}
	})
}

// admit runs Admit with the case context and returns the receipt and the events the
// store received, by batch index.
func admit(t *testing.T, raw []byte, c apiContext) (perfdata.Receipt, [][]perfdata.Event) {
	t.Helper()
	now, ok := perfdata.ParseTime(c.Now)
	if !ok {
		t.Fatalf("bad now %q", c.Now)
	}
	var stored [][]perfdata.Event
	ac := perfdata.AdmitContext{AuthenticatedSource: c.AuthenticatedSource, ConsentRevision: c.ConsentRevision,
		AllowedProfiles: c.AllowedProfiles, Now: now, ReplayHorizon: time.Duration(c.ReplayHorizonHours) * time.Hour, Known: c.Known,
		Commit: func(evs []perfdata.Event) error {
			stored = append(stored, evs)
			if c.Commit == "fail" {
				return errors.New("store unavailable")
			}
			return nil
		}}
	return perfdata.Admit(raw, ac), stored
}

func checkWant(t testing.TB, got perfdata.Receipt, stored [][]perfdata.Event, raw []byte, w want) {
	t.Helper()
	if got.Schema != perfdata.ReceiptSchema || got.BatchDigest != perfdata.Digest(raw) {
		t.Fatalf("receipt must name its schema and the digest of the exact batch bytes: %+v", got)
	}
	if got.Status != w.status || got.Reason != w.reason || got.Committed != w.committed {
		t.Fatalf("receipt: status=%q reason=%q committed=%v, want %q %q %v", got.Status, got.Reason, got.Committed, w.status, w.reason, w.committed)
	}
	var statuses []string
	for i, r := range got.Results {
		if r.Index != i {
			t.Fatalf("results must be in batch order: %+v", got.Results)
		}
		statuses = append(statuses, r.Status)
		if c, ok := w.classes[i]; ok && r.Class != c {
			t.Fatalf("result %d class %q, want %q", i, r.Class, c)
		}
		if r.Status == perfdata.ResultRejected && r.Class == "" {
			t.Fatalf("a refused event names its class: %+v", r)
		}
	}
	if fmt.Sprint(statuses) != fmt.Sprint(w.results) && !(len(statuses) == 0 && len(w.results) == 0) {
		t.Fatalf("per-event statuses %v, want %v", statuses, w.results)
	}
	if len(stored) != w.commitCalls {
		t.Fatalf("store received %d commits, want %d", len(stored), w.commitCalls)
	}
	if w.committed && len(stored) == 1 {
		var ids []string
		for _, e := range stored[0] {
			ids = append(ids, e.ID)
		}
		var wantIDs []string
		for _, i := range w.committedIdx {
			wantIDs = append(wantIDs, got.Results[i].ID)
		}
		if fmt.Sprint(ids) != fmt.Sprint(wantIDs) {
			t.Fatalf("store received %v, want %v (duplicates and conflicts are never re-stored)", ids, wantIDs)
		}
	}
}

// wantFails reports whether f fails its testing.TB.
func wantFails(f func(tb testing.TB)) bool {
	ft := &failTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(ft)
	}()
	<-done
	return ft.failed
}

type failTB struct {
	testing.TB
	failed bool
}

func (f *failTB) Helper()                      {}
func (f *failTB) Fatalf(string, ...any)        { f.failed = true; runtime.Goexit() }
func (f *failTB) Errorf(string, ...any)        { f.failed = true }
func (f *failTB) Fatal(...any)                 { f.failed = true; runtime.Goexit() }
func (f *failTB) Logf(format string, a ...any) {}
