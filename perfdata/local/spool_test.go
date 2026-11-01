package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

var day1 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

var armed = perfdata.Consent{Armed: true, Profile: perfdata.ProfileTool}

func testPolicy(t testing.TB, extra string) perfdata.Policy {
	t.Helper()
	p, err := perfdata.DecodePolicy([]byte(`{"schema":"performance-policy-v1","revision":"r1","profiles":["tool","workflow"],` +
		`"tools":["statusgen","deskboard"],"ageCapDays":7,"byteCap":16777216` + extra + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func toolDraft(startedAt time.Time, wall int64) perfdata.Draft {
	tc := perfdata.ToolCompletion{Tool: "statusgen", Verb: "lint", Outcome: perfdata.OutcomeOK, StartedAt: perfdata.FormatTime(startedAt),
		Wall: perfdata.Measured(wall), CPU: perfdata.CPU{Own: perfdata.Measured(1), Descendant: perfdata.Unavailable(), Inclusive: perfdata.Unavailable()}}
	b, _ := json.Marshal(tc)
	return perfdata.Draft{Kind: perfdata.KindTool, Ctx: &perfdata.RawContext{Invocation: "run-" + strconv.FormatInt(invSeq.Add(1), 10)}, Payload: b}
}

var invSeq atomic.Int64

func fixed(t time.Time) func() time.Time { return func() time.Time { return t } }

func mustOpen(t testing.TB, dir string, pol perfdata.Policy, o Options) *Store {
	t.Helper()
	st, err := Open(dir, armed, pol, o)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func mustWrite(t testing.TB, st *Store, n int, at time.Time) []perfdata.Event {
	t.Helper()
	ds := make([]perfdata.Draft, n)
	for i := range ds {
		ds[i] = toolDraft(at, int64(10+i))
	}
	res, err := st.Write(ds, false)
	if err != nil || len(res.Refused) > 0 {
		t.Fatalf("write: %v %+v", err, res.Refused)
	}
	out := make([]perfdata.Event, len(res.Lines))
	for i, l := range res.Lines {
		e, err := perfdata.DecodeEvent(l)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = e
	}
	return out
}

func mustRead(t testing.TB, dir string) (ReadResult, perfdata.SourceCoverage) {
	t.Helper()
	rr, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Coverage) != 1 {
		t.Fatalf("want one source, got %d: %+v", len(rr.Coverage), rr.Coverage)
	}
	return rr, rr.Coverage[0]
}

// helperEnv drives TestSpoolHelperProcess in a child process.
const helperEnv = "PERFDATA_SPOOL_HELPER"

// TestSpoolHelperProcess is the child side of the cross-process cases. It does nothing
// unless started by them.
func TestSpoolHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		t.Skip("helper process only")
	}
	dir := os.Getenv(helperEnv + "_DIR")
	n, _ := strconv.Atoi(os.Getenv(helperEnv + "_N"))
	st, err := Open(dir, armed, testPolicy(t, ""), Options{Now: fixed(day1)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	for i := 0; i < n; i++ {
		if _, err := st.Write([]perfdata.Draft{toolDraft(day1, 5)}, false); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	}
	if mode == "crash" {
		os.Exit(1) // no Close: the live marker is left behind, as after a kill
	}
	st.Close()
	os.Exit(0)
}

func helper(t *testing.T, mode, dir string, n int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestSpoolHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), helperEnv+"="+mode, helperEnv+"_DIR="+dir, helperEnv+"_N="+strconv.Itoa(n))
	return cmd
}

func TestSpoolRecovery(t *testing.T) {
	t.Run("control: a clean write and close reads complete", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, st, 3, day1)
		st.Close()
		rr, c := mustRead(t, dir)
		if c.State != perfdata.CoverageComplete || len(rr.Events) != 3 || c.LastSeq != 3 {
			t.Fatalf("clean store: %+v", c)
		}
	})

	t.Run("concurrent appends from goroutines and processes never interleave or reuse a sequence", func(t *testing.T) {
		dir := t.TempDir()
		pol := testPolicy(t, `,"segmentBytes":32768`) // force segment rollover under contention
		boot := mustOpen(t, dir, pol, Options{Now: fixed(day1)})
		boot.Close()
		const goroutines, perG, procs, perP = 6, 25, 2, 40
		var cmds []*exec.Cmd
		for i := 0; i < procs; i++ {
			c := helper(t, "clean", dir, perP)
			if err := c.Start(); err != nil {
				t.Fatal(err)
			}
			cmds = append(cmds, c)
		}
		var wg sync.WaitGroup
		errs := make(chan error, goroutines)
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				st, err := Open(dir, armed, pol, Options{Now: fixed(day1), LockWait: 10 * time.Second})
				if err != nil {
					errs <- err
					return
				}
				defer st.Close()
				for i := 0; i < perG; i++ {
					if _, err := st.Write([]perfdata.Draft{toolDraft(day1, 1), toolDraft(day1, 2)}, false); err != nil {
						errs <- err
						return
					}
				}
			}()
		}
		wg.Wait()
		for _, c := range cmds {
			if err := c.Wait(); err != nil {
				t.Fatalf("helper: %v", err)
			}
		}
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		rr, c := mustRead(t, dir)
		want := goroutines*perG*2 + procs*perP
		if len(rr.Events) != want || c.LastSeq != uint64(want) || c.State != perfdata.CoverageComplete || rr.Conflicts != 0 || rr.Duplicates != 0 {
			t.Fatalf("want %d events seq 1..%d complete, got %d events last %d %s conflicts=%d gaps=%v",
				want, want, len(rr.Events), c.LastSeq, c.State, rr.Conflicts, c.UnexplainedGaps)
		}
		if segs := listSegments(Root(dir)); len(segs) < 2 {
			t.Fatalf("expected segment rollover, got %d segments", len(segs))
		}
	})

	t.Run("a partial tail is reported and never extended", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, st, 2, day1)
		seg := filepath.Join(Root(dir), "segments", listSegments(Root(dir))[0].name)
		f, _ := os.OpenFile(seg, os.O_WRONLY|os.O_APPEND, 0)
		f.WriteString(`{"schema":"performance-event-v1","id":"tor`) // a torn final append
		f.Close()
		mustWrite(t, st, 1, day1)
		st.Close()
		rr, c := mustRead(t, dir)
		if c.PartialTails != 1 || c.State != perfdata.CoveragePartial || len(rr.Events) != 3 || c.CorruptLines != 0 {
			t.Fatalf("partial tail: %+v events=%d", c, len(rr.Events))
		}
		if n := len(listSegments(Root(dir))); n != 2 {
			t.Fatalf("the write after a torn tail must start a new segment, got %d segments", n)
		}
	})

	t.Run("a crashed writer reads as uncertain and the source survives restart", func(t *testing.T) {
		dir := t.TempDir()
		if err := helper(t, "crash", dir, 3).Run(); err == nil {
			t.Fatal("crash helper exited cleanly")
		}
		_, before := mustRead(t, dir)
		if before.LiveWriters != 0 || before.UncleanTerminations != 1 || before.State != perfdata.CoveragePartial {
			t.Fatalf("a crashed writer's orphaned marker must read as an unclean end: %+v", before)
		}
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		evs := mustWrite(t, st, 1, day1)
		st.Close()
		rr, c := mustRead(t, dir)
		if c.UncleanTerminations != 1 || c.State != perfdata.CoveragePartial || c.LiveWriters != 0 {
			t.Fatalf("unclean end not reported: %+v", c)
		}
		if evs[0].Source != rr.Events[0].Source || evs[0].Seq != 4 || evs[0].Epoch != rr.Events[0].Epoch {
			t.Fatalf("source identity must survive restart: first %s/%d/%d, after %s/%d/%d",
				rr.Events[0].Source, rr.Events[0].Epoch, rr.Events[0].Seq, evs[0].Source, evs[0].Epoch, evs[0].Seq)
		}
	})

	t.Run("lost metadata moves the epoch forward and never reuses a sequence", func(t *testing.T) {
		dir := t.TempDir()
		pol := testPolicy(t, "")
		st := mustOpen(t, dir, pol, Options{Now: fixed(day1)})
		first := mustWrite(t, st, 2, day1)
		st.Close()
		os.Remove(filepath.Join(Root(dir), "source.json"))
		st = mustOpen(t, dir, pol, Options{Now: fixed(day1)})
		next := mustWrite(t, st, 1, day1)
		st.Close()
		if next[0].Source != first[0].Source || next[0].Seq != 3 || next[0].Epoch != first[0].Epoch+1 {
			t.Fatalf("recovered identity: %+v after %+v", next[0], first[1])
		}
		_, c := mustRead(t, dir)
		if len(c.EpochResets) != 1 || c.EpochResets[0].Reason != perfdata.ResetSourceLost || c.State != perfdata.CoveragePartial {
			t.Fatalf("epoch reset not reported: %+v", c)
		}
	})

	t.Run("a full disk is counted, explained and never reaches the caller", func(t *testing.T) {
		dir := t.TempDir()
		var full atomic.Bool
		o := Options{Now: fixed(day1), fault: func(op string) error {
			if full.Load() && op != "meta" {
				return syscall.ENOSPC
			}
			return nil
		}}
		pol := testPolicy(t, "")
		sink, err := OpenSink(dir, armed, pol, o)
		if err != nil {
			t.Fatal(err)
		}
		sink.Record(toolDraft(day1, 1))
		waitFor(t, func() bool { return countEvents(t, dir) == 1 })
		full.Store(true)
		sink.Record(toolDraft(day1, 2))
		waitFor(t, func() bool { return sink.Health().WriteFailed == 1 })
		v, err := Observe(sink, func() (int, error) { return 42, nil }, func(int, error, time.Duration) perfdata.Draft { return toolDraft(day1, 3) })
		if v != 42 || err != nil {
			t.Fatalf("a full disk changed the wrapped result: %d %v", v, err)
		}
		_, err = Observe(sink, func() (int, error) { return 0, errSentinel }, func(int, error, time.Duration) perfdata.Draft { return toolDraft(day1, 4) })
		if !errors.Is(err, errSentinel) {
			t.Fatalf("the wrapped error must pass through unchanged: %v", err)
		}
		if !sink.Close() {
			t.Fatal("a fast failing disk is not a flush timeout")
		}
		full.Store(false)
		rr, c := mustRead(t, dir)
		if len(rr.Events) != 1 || c.Drops.WriteFailed < 3 || c.State != perfdata.CoveragePartial {
			t.Fatalf("full disk coverage: events=%d %+v", len(rr.Events), c)
		}
		for _, g := range c.ExplainedGaps {
			if g.Reason != perfdata.GapWriteFailed {
				t.Fatalf("unexpected gap reason %+v", g)
			}
		}
		if len(c.ExplainedGaps) == 0 || len(c.UnexplainedGaps) != 0 {
			t.Fatalf("failed writes must be explained gaps: %+v / %+v", c.ExplainedGaps, c.UnexplainedGaps)
		}
	})

	t.Run("expired data is retired as unsent and the gap is explained", func(t *testing.T) {
		dir := t.TempDir()
		pol := testPolicy(t, "")
		st := mustOpen(t, dir, pol, Options{Now: fixed(day1)})
		first := mustWrite(t, st, 4, day1)
		st.Close()
		later := day1.AddDate(0, 0, pol.AgeCapDays+1)
		st = mustOpen(t, dir, pol, Options{Now: fixed(later)})
		next := mustWrite(t, st, 1, later)
		st.Close()
		rr, c := mustRead(t, dir)
		if len(rr.Events) != 1 || next[0].Seq != 5 || next[0].Source != first[0].Source {
			t.Fatalf("after retirement: %d events, next %+v", len(rr.Events), next[0])
		}
		if len(c.Retired) != 1 || c.Retired[0].Reason != RetireAge || !c.Retired[0].Unsent || c.Retired[0].Events != 4 {
			t.Fatalf("retirement record: %+v", c.Retired)
		}
		if len(c.ExplainedGaps) != 1 || c.ExplainedGaps[0].To != 4 || len(c.UnexplainedGaps) != 0 || c.State != perfdata.CoverageComplete {
			t.Fatalf("retired range must be an explained gap: %+v", c)
		}
	})

	t.Run("a missing store is unknown, not empty", func(t *testing.T) {
		rr, err := Read(t.TempDir())
		if err != nil || rr.Present || rr.Coverage[0].State != perfdata.CoverageMissing {
			t.Fatalf("missing store: %+v %v", rr, err)
		}
	})
}

var errSentinel = errors.New("wrapped operation failed")

func countEvents(t testing.TB, dir string) int {
	rr, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(rr.Events)
}

func waitFor(t testing.TB, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached within 5s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
