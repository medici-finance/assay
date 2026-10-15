package cellop

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// world is the stub a session measures and changes: precondition values by key, and how many
// times each action ran.
type world struct {
	mu    sync.Mutex
	state map[string]string
	ran   map[string]int
	fail  map[string]bool
	// onRun runs inside Run, before the action is counted.
	onRun func(a Action)
}

func newWorld() *world {
	return &world{state: map[string]string{}, ran: map[string]int{}, fail: map[string]bool{}}
}

func (w *world) set(k, v string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state[k] = v
}

func (w *world) runs(id string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ran[id]
}

func (w *world) measure(a Action) ([]Precondition, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []Precondition
	for _, pc := range a.Preconditions {
		v, ok := w.state[pc.Key]
		if !ok {
			return nil, errors.New("cannot read " + pc.Key)
		}
		out = append(out, Precondition{Key: pc.Key, Value: v})
	}
	return out, nil
}

func (w *world) run(a Action) (string, error) {
	if w.onRun != nil {
		w.onRun(a)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ran[a.ID]++
	if w.fail[a.ID] {
		return "", errors.New("stub failure")
	}
	return "done", nil
}

// clock is a settable time source.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func newID(t *testing.T) string {
	t.Helper()
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// testPlan is a one-action drain plan on cell whose precondition "state" was measured as value.
func testPlan(t *testing.T, cell, value string) Plan {
	t.Helper()
	p, err := NewPlan(newID(t), "drain", []string{cell}, []Action{{
		ID: "drain-" + cell, Cell: cell, Summary: "drain " + cell,
		Preconditions: []Precondition{{Key: cell + "/state", Value: value}},
	}}, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func save(t *testing.T, s Store, p Plan) {
	t.Helper()
	if _, err := s.Save(p); err != nil {
		t.Fatal(err)
	}
}

// session is an interactive operator session on cell that types the right code.
func session(s Store, w *world, c *clock, cell, verb string) Session {
	return Session{
		Store: s, Cell: cell, Verb: verb, Interactive: true,
		Confirm: func(p Plan, _ []Plan) (string, error) { return p.ConfirmCode, nil },
		Measure: w.measure, Run: w.run, Now: c.Now,
	}
}

func expect(t *testing.T, r Result, o Outcome, reason string) {
	t.Helper()
	got := ""
	if r.Reason != nil {
		got = *r.Reason
	}
	if r.Outcome != o || got != reason {
		detail := ""
		if r.Detail != nil {
			detail = *r.Detail
		}
		t.Fatalf("outcome %s/%q (%s), want %s/%q", r.Outcome, got, detail, o, reason)
	}
	if r.ExitCode() != o.ExitCode() {
		t.Fatalf("exit %d for %s", r.ExitCode(), o)
	}
}

func consumed(t *testing.T, s Store, id string) bool {
	t.Helper()
	done, err := s.Consumed(id)
	if err != nil {
		t.Fatal(err)
	}
	return done
}

// The contract's exit codes are 0 ok, 1 failed/partial, 5 refused, 6 could-not-check, and an unknown outcome never exits 0 (desk-containers/15).
// regression: #2890
func TestOutcomeExitCodes(t *testing.T) {
	for o, want := range map[Outcome]int{OK: 0, Partial: 1, Failed: 1, Refused: 5, CouldNotCheck: 6, "bogus": 1} {
		if got := o.ExitCode(); got != want {
			t.Errorf("%s exits %d, want %d", o, got, want)
		}
	}
	r := NewResult("status", "example", t0)
	if r.Outcome != CouldNotCheck || r.Roles == nil || r.Actions == nil || r.Next == nil {
		t.Fatalf("a fresh result must be could-not-check with empty lists: %+v", r)
	}
	raw, _ := r.JSON()
	if !strings.Contains(string(raw), `"plan_id":null`) || !strings.Contains(string(raw), `"roles":[]`) {
		t.Fatalf("unmeasured values must be null and lists empty: %s", raw)
	}
}

// The digest must be sha256 over RFC 8785 bytes with digest and confirm_code omitted, and confirm_code its first 8 characters (desk-containers/15).
// regression: #2890
func TestPlanDigestIsJCS(t *testing.T) {
	p, err := NewPlan("0123456789abcdef01234567", "gc", []string{"b", "a", "b"}, []Action{{
		ID: "x", Cell: "a", Summary: "quote \" slash \\ tab \t nl \n ctl \u0001 <&> é €",
		Preconditions: []Precondition{{Key: "k", Value: "v"}},
	}}, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"actions":[{"cell":"a","id":"x","preconditions":[{"key":"k","value":"v"}],` +
		`"summary":"quote \" slash \\ tab \t nl \n ctl \u0001 <&> é €"}],"cells":["a","b"],"children":[],` +
		`"created":"2026-10-14T12:00:00Z","expires":"2026-10-14T12:10:00Z","id":"0123456789abcdef01234567",` +
		`"kind":"cell-op-plan","schema":"cell-op-v1","verb":"gc"}`
	got, err := p.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("canonical form\n got %s\nwant %s", got, want)
	}
	if p.ConfirmCode != p.Digest[:8] || len(p.Digest) != 64 {
		t.Fatalf("digest %q confirm %q", p.Digest, p.ConfirmCode)
	}
	q := p
	q.Actions = []Action{{ID: "x", Cell: "a", Summary: "other", Preconditions: []Precondition{{Key: "k", Value: "v"}}}}
	if err := q.Verify(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an edited plan must fail its digest, got %v", err)
	}
	if p.Expires != "2026-10-14T12:10:00Z" {
		t.Fatalf("expiry %s, want created + 10m", p.Expires)
	}
}

// A stored plan is 0600 in a 0700 directory, never overwritten, and decoded strictly (desk-containers/15).
// regression: #2890
func TestStoreModesAndStrictDecode(t *testing.T) {
	s := StoreFor(t.TempDir())
	p := testPlan(t, "example", "running")
	path, err := s.Save(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("plan mode %v, want 0600", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(s.Dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("plan dir mode %v, want 0700", fi.Mode().Perm())
	}
	if _, err := s.Save(p); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("a second save of the same id must not overwrite, got %v", err)
	}
	if got, err := s.Load(p.ID); err != nil || got.Digest != p.Digest {
		t.Fatalf("load: %v", err)
	}
	if _, err := s.Load("../../etc/passwd"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a path-like id must be refused, got %v", err)
	}
	if _, err := s.Load(newID(t)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing plan must be ErrNotFound, got %v", err)
	}
	raw, _ := json.Marshal(p)
	for name, bad := range map[string]string{
		"case-variant member": strings.Replace(string(raw), `"verb"`, `"Verb"`, 1),
		"unknown member":      strings.Replace(string(raw), `{"schema"`, `{"approval":"x","schema"`, 1),
		"null list":           strings.Replace(string(raw), `"children":[]`, `"children":null`, 1),
		"trailing data":       string(raw) + "{}",
		"edited summary":      strings.Replace(string(raw), "drain example", "drain elsewhere", 1),
	} {
		if _, err := DecodePlan([]byte(bad)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := DecodePlan(raw); err != nil {
		t.Fatalf("the plan itself must decode: %v", err)
	}
}

// Apply off a terminal, or with a desk-session marker set, must refuse before loading, confirming or consuming (desk-containers/15).
// regression: #2890
func TestApplyRefusesOffTTYAndDeskSession(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0}
	p := testPlan(t, "example", "running")
	save(t, s, p)
	w.set("example/state", "running")
	sess := session(s, w, c, "example", "drain")
	sess.Interactive = false
	expect(t, sess.Apply(p.ID), Refused, ReasonNotInteractive)
	sess.Interactive = true
	for _, k := range DeskSessionMarkers {
		markers := SetMarkers(func(name string) (string, bool) { return "", name == k })
		if len(markers) != 1 || markers[0] != k {
			t.Fatalf("SetMarkers(%s) = %v", k, markers)
		}
		sess.DeskMarkers = markers
		expect(t, sess.Apply(p.ID), Refused, ReasonDeskSession)
	}
	if w.runs("drain-example") != 0 || consumed(t, s, p.ID) {
		t.Fatal("a refused apply ran or consumed the plan")
	}
	sess.DeskMarkers = nil
	sess.Confirm = func(Plan, []Plan) (string, error) { return "00000000", nil }
	expect(t, sess.Apply(p.ID), Refused, ReasonConfirmMismatch)
	sess.Verb = "pause"
	expect(t, sess.Apply(p.ID), Refused, ReasonWrongVerb)
	if w.runs("drain-example") != 0 || consumed(t, s, p.ID) {
		t.Fatal("a refused apply ran or consumed the plan")
	}
}

// 16 concurrent applies of one plan must run its actions exactly once (consume before effect) (desk-containers/15).
// regression: #2890
func TestConcurrentApplyRunsOnce(t *testing.T) {
	const n = 16
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0}
	p := testPlan(t, "example", "running")
	save(t, s, p)
	w.set("example/state", "running")
	// Every apply waits at the confirm until all n are there, so all n are past the early
	// consumed check before any of them consumes: only the atomic consume can serialise them.
	var arrived sync.WaitGroup
	arrived.Add(n)
	sess := session(s, w, c, "example", "drain")
	sess.Confirm = func(p Plan, _ []Plan) (string, error) {
		arrived.Done()
		arrived.Wait()
		return p.ConfirmCode, nil
	}
	results := make([]Result, n)
	var done sync.WaitGroup
	for i := 0; i < n; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			results[i] = sess.Apply(p.ID)
		}(i)
	}
	done.Wait()
	oks, refusedConsumed := 0, 0
	for _, r := range results {
		switch {
		case r.Outcome == OK:
			oks++
		case r.Outcome == Refused && r.Reason != nil && *r.Reason == ReasonAlreadyConsumed:
			refusedConsumed++
		default:
			t.Errorf("unexpected result %s %v", r.Outcome, r.Reason)
		}
	}
	if oks != 1 || refusedConsumed != n-1 {
		t.Fatalf("%d applied and %d refused already-consumed, want 1 and %d", oks, refusedConsumed, n-1)
	}
	if got := w.runs("drain-example"); got != 1 {
		t.Fatalf("the action ran %d times, want exactly 1", got)
	}
}

// A plan whose precondition changed must be refused plan-stale and stay unconsumed (desk-containers/15).
// regression: #2890
func TestApplyReverifies(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0}
	p := testPlan(t, "example", "running")
	save(t, s, p)
	sess := session(s, w, c, "example", "drain")
	w.set("example/state", "paused")
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanStale)
	if w.runs("drain-example") != 0 || consumed(t, s, p.ID) {
		t.Fatal("a stale plan ran or was consumed")
	}
	// An unmeasurable precondition is could-not-check, also unconsumed.
	w.mu.Lock()
	delete(w.state, "example/state")
	w.mu.Unlock()
	expect(t, sess.Apply(p.ID), CouldNotCheck, ReasonUnmeasured)
	if consumed(t, s, p.ID) {
		t.Fatal("an unmeasured plan was consumed")
	}
	// The re-measure runs after the confirm: a change made while the operator reads the plan
	// is still caught.
	w.set("example/state", "running")
	sess.Confirm = func(p Plan, _ []Plan) (string, error) {
		w.set("example/state", "stopped")
		return p.ConfirmCode, nil
	}
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanStale)
	if w.runs("drain-example") != 0 || consumed(t, s, p.ID) {
		t.Fatal("a plan that went stale during the confirm ran or was consumed")
	}
	// Restored, the same plan applies: the refusals above did not consume it.
	w.set("example/state", "running")
	sess.Confirm = func(p Plan, _ []Plan) (string, error) { return p.ConfirmCode, nil }
	expect(t, sess.Apply(p.ID), OK, "")
	if w.runs("drain-example") != 1 || !consumed(t, s, p.ID) {
		t.Fatal("the restored plan did not apply exactly once")
	}
}

// Apply at or after the 10-minute expiry, and apply of a plan that does not list the invoking cell, must be refused (desk-containers/15).
// regression: #2890
func TestPlanExpiryAndForeignCell(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{}
	p := testPlan(t, "example", "running")
	save(t, s, p)
	w.set("example/state", "running")
	sess := session(s, w, c, "example", "drain")
	c.set(t0.Add(Expiry))
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanExpired)
	c.set(t0.Add(time.Hour))
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanExpired)
	// Expiry is checked again after the confirm: an operator who waits at the prompt past it
	// is refused.
	c.set(t0.Add(Expiry - time.Second))
	sess.Confirm = func(p Plan, _ []Plan) (string, error) {
		c.set(t0.Add(Expiry))
		return p.ConfirmCode, nil
	}
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanExpired)
	sess.Confirm = func(p Plan, _ []Plan) (string, error) { return p.ConfirmCode, nil }
	// Another cell's plan, stored where this cell would look for it, is refused.
	other := testPlan(t, "elsewhere", "running")
	save(t, s, other)
	w.set("elsewhere/state", "running")
	c.set(t0.Add(time.Minute))
	expect(t, sess.Apply(other.ID), Refused, ReasonForeignCell)
	if w.runs("drain-example")+w.runs("drain-elsewhere") != 0 || consumed(t, s, p.ID) || consumed(t, s, other.ID) {
		t.Fatal("a refused plan ran or was consumed")
	}
	// One second before expiry the same plan still applies.
	expect(t, sess.Apply(p.ID), OK, "")
}

// The stated bound — a deleted consumed marker allows another apply within expiry only if every precondition still matches, and none after expiry (desk-containers/15).
// regression: #2890
func TestMarkerResetBound(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
	p := testPlan(t, "example", "running")
	save(t, s, p)
	w.set("example/state", "running")
	sess := session(s, w, c, "example", "drain")
	expect(t, sess.Apply(p.ID), OK, "")
	expect(t, sess.Apply(p.ID), Refused, ReasonAlreadyConsumed)
	reset := func() {
		if err := os.Remove(s.MarkerPath(p.ID)); err != nil {
			t.Fatal(err)
		}
	}
	reset()
	// Within expiry, preconditions unchanged: the stated limit — it runs again.
	expect(t, sess.Apply(p.ID), OK, "")
	if w.runs("drain-example") != 2 {
		t.Fatalf("ran %d times, want 2 (the stated bound)", w.runs("drain-example"))
	}
	reset()
	// Within expiry, a precondition changed: the re-measure still refuses.
	w.set("example/state", "drained")
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanStale)
	// After expiry: refused whatever the preconditions say.
	w.set("example/state", "running")
	c.set(t0.Add(Expiry))
	expect(t, sess.Apply(p.ID), Refused, ReasonPlanExpired)
	if w.runs("drain-example") != 2 {
		t.Fatalf("ran %d times after the bound, want 2", w.runs("drain-example"))
	}
}

// batch builds a batch plan on cell "example" of n one-action children and stores them all.
func batch(t *testing.T, s Store, w *world, n int) (Plan, []Plan) {
	t.Helper()
	var children []Plan
	var refs []ChildRef
	for i := 0; i < n; i++ {
		key := "example/step" + string(rune('1'+i))
		ch, err := NewPlan(newID(t), "drain", []string{"example"}, []Action{{
			ID: "step" + string(rune('1'+i)), Cell: "example", Summary: "step",
			Preconditions: []Precondition{{Key: key, Value: "ready"}},
		}}, nil, t0)
		if err != nil {
			t.Fatal(err)
		}
		w.set(key, "ready")
		save(t, s, ch)
		children = append(children, ch)
		refs = append(refs, ChildRef{ID: ch.ID, Digest: ch.Digest})
	}
	b, err := NewPlan(newID(t), "restart", []string{"example"}, nil, refs, t0)
	if err != nil {
		t.Fatal(err)
	}
	save(t, s, b)
	return b, children
}

// A batch is consumed before its first child runs, and a child found already consumed stops the sequence instead of being skipped (desk-containers/15).
// regression: #2890
func TestBatchConsumedFirstAndStops(t *testing.T) {
	t.Run("concurrent applies: exactly one proceeds", func(t *testing.T) {
		s := StoreFor(t.TempDir())
		w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
		b, children := batch(t, s, w, 2)
		// Every first-child action asserts that the batch was already consumed when it ran.
		w.onRun = func(a Action) {
			if !consumed(t, s, b.ID) {
				t.Errorf("action %s ran before the batch was consumed", a.ID)
			}
		}
		var arrived, done sync.WaitGroup
		arrived.Add(2)
		sess := session(s, w, c, "example", "restart")
		sess.Confirm = func(p Plan, ch []Plan) (string, error) {
			if len(ch) != 2 {
				t.Errorf("the confirm must show both children, got %d", len(ch))
			}
			arrived.Done()
			arrived.Wait()
			return p.ConfirmCode, nil
		}
		var results [2]Result
		for i := range results {
			done.Add(1)
			go func(i int) {
				defer done.Done()
				results[i] = sess.Apply(b.ID)
			}(i)
		}
		done.Wait()
		oks := 0
		for _, r := range results {
			if r.Outcome == OK {
				oks++
			} else if r.Reason == nil || *r.Reason != ReasonAlreadyConsumed || len(nonNotRun(r)) != 0 {
				t.Errorf("the losing apply must be refused already-consumed having run nothing: %s %v", r.Outcome, r.Reason)
			}
		}
		if oks != 1 {
			t.Fatalf("%d batch applies proceeded, want exactly 1", oks)
		}
		for _, ch := range children {
			if w.runs(ch.Actions[0].ID) != 1 || !consumed(t, s, ch.ID) {
				t.Fatalf("child %s ran %d times, want once and consumed", ch.ID, w.runs(ch.Actions[0].ID))
			}
		}
	})
	t.Run("a consumed child stops the sequence", func(t *testing.T) {
		s := StoreFor(t.TempDir())
		w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
		b, children := batch(t, s, w, 3)
		if err := s.Consume(children[1].ID, c.Now()); err != nil {
			t.Fatal(err)
		}
		r := session(s, w, c, "example", "restart").Apply(b.ID)
		expect(t, r, Partial, ReasonAlreadyConsumed)
		if w.runs("step1") != 1 || w.runs("step2") != 0 || w.runs("step3") != 0 {
			t.Fatalf("runs step1=%d step2=%d step3=%d, want 1 0 0: the sequence must stop, never skip and continue",
				w.runs("step1"), w.runs("step2"), w.runs("step3"))
		}
		if consumed(t, s, children[2].ID) {
			t.Fatal("a step after the stop was consumed")
		}
		if got := r.Actions[2].Outcome; got != ActionNotRun {
			t.Fatalf("step3 recorded %s, want not-run", got)
		}
	})
	t.Run("a stale child stops the sequence", func(t *testing.T) {
		s := StoreFor(t.TempDir())
		w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
		b, _ := batch(t, s, w, 2)
		w.set("example/step1", "gone")
		expect(t, session(s, w, c, "example", "restart").Apply(b.ID), Refused, ReasonPlanStale)
		if w.runs("step1")+w.runs("step2") != 0 || !consumed(t, s, b.ID) {
			t.Fatal("a stale first step must stop the sequence with the batch consumed")
		}
	})
	t.Run("the batch expiry bounds the start of each step", func(t *testing.T) {
		s := StoreFor(t.TempDir())
		w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
		b, _ := batch(t, s, w, 2)
		w.onRun = func(a Action) { c.set(t0.Add(Expiry)) }
		r := session(s, w, c, "example", "restart").Apply(b.ID)
		expect(t, r, Partial, ReasonPlanExpired)
		if w.runs("step1") != 1 || w.runs("step2") != 0 {
			t.Fatal("a step must not start after the batch expired")
		}
	})
	t.Run("a child that no longer matches the batch is refused before anything", func(t *testing.T) {
		s := StoreFor(t.TempDir())
		w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
		b, children := batch(t, s, w, 2)
		if err := os.Remove(s.Path(children[1].ID)); err != nil {
			t.Fatal(err)
		}
		expect(t, session(s, w, c, "example", "restart").Apply(b.ID), Refused, ReasonPlanNotFound)
		if consumed(t, s, b.ID) || w.runs("step1") != 0 {
			t.Fatal("a batch with a missing child was consumed or ran")
		}
	})
}

func nonNotRun(r Result) []ActionResult {
	var out []ActionResult
	for _, a := range r.Actions {
		if a.Outcome != ActionNotRun {
			out = append(out, a)
		}
	}
	return out
}

// A failure after the consume leaves the plan consumed with a failed (or partial) result, never ok (desk-containers/15).
// regression: #2890
func TestFailureAfterConsume(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0.Add(time.Minute)}
	p, err := NewPlan(newID(t), "gc", []string{"example"}, []Action{
		{ID: "a1", Cell: "example", Summary: "one", Preconditions: []Precondition{}},
		{ID: "a2", Cell: "example", Summary: "two", Preconditions: []Precondition{}},
	}, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	save(t, s, p)
	w.fail["a2"] = true
	r := session(s, w, c, "example", "gc").Apply(p.ID)
	expect(t, r, Partial, ReasonActionFailed)
	if !consumed(t, s, p.ID) || r.Actions[0].Outcome != ActionOK || r.Actions[1].Outcome != ActionFailed {
		t.Fatalf("after a failure: consumed=%v actions=%+v", consumed(t, s, p.ID), r.Actions)
	}
	q, _ := NewPlan(newID(t), "gc", []string{"example"}, []Action{{ID: "a2", Cell: "example", Summary: "two", Preconditions: []Precondition{}}}, nil, t0)
	save(t, s, q)
	expect(t, session(s, w, c, "example", "gc").Apply(q.ID), Failed, ReasonActionFailed)
	if !consumed(t, s, q.ID) {
		t.Fatal("a plan that failed after its consume must stay consumed")
	}
}

// The confirm prompt shows plan, cells and code, states the limit, and reads exactly one line (desk-containers/15).
// regression: #2890
func TestReadConfirm(t *testing.T) {
	p := testPlan(t, "example", "running")
	var out bytes.Buffer
	in := strings.NewReader(p.ConfirmCode + "\r\nleftover\n")
	got, err := ReadConfirm(in, &out, Render(p, nil))
	if err != nil || got != p.ConfirmCode {
		t.Fatalf("read %q, %v", got, err)
	}
	rest := make([]byte, 64)
	n, _ := in.Read(rest)
	if string(rest[:n]) != "leftover\n" {
		t.Fatalf("ReadConfirm consumed past its line: %q left", rest[:n])
	}
	for _, want := range []string{p.ID, "example", p.ConfirmCode, "not an authorization control", "requires example/state"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the prompt does not show %q:\n%s", want, out.String())
		}
	}
	if _, err := ReadConfirm(strings.NewReader(""), &out, ""); err == nil {
		t.Fatal("an empty stdin must not read as a confirm")
	}
}
