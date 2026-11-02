package local

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

func quantiles(d []time.Duration) (p50, p95, mx time.Duration) {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	at := func(q float64) time.Duration { return s[min(len(s)-1, int(q*float64(len(s))))] }
	return at(0.50), at(0.95), s[len(s)-1]
}

// The 1 ms append target is a property of a reference host, not of whatever machine runs
// the suite. The gate therefore takes the best of several rounds (a transient stall
// spoils one round, not the result) and measures, in each round, the bare floor this
// filesystem sets for the same work: open, append one line, close, with no store logic.
// A floor this slow means the host is loaded or slow, so the target cannot be judged on
// it; the gate says so and skips rather than failing or passing on noise.
const (
	appendTarget   = time.Millisecond
	appendRounds   = 5
	appendPerRound = 100
	slowHostFloor  = 100 * time.Microsecond
)

// fsFloor is the median cost of an open/append/close of one line in dir.
func fsFloor(t *testing.T, dir string, line []byte, n int) time.Duration {
	t.Helper()
	p := filepath.Join(dir, "floor")
	lat := make([]time.Duration, n)
	for i := range lat {
		start := time.Now()
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(line); err != nil {
			t.Fatal(err)
		}
		f.Close()
		lat[i] = time.Since(start)
	}
	p50, _, _ := quantiles(lat)
	return p50
}

// regression: the append gate failed on a loaded host and the slow-I/O counts at -cpu 1.
func TestLocalCostAndBounds(t *testing.T) {
	t.Run("a small locked append stays under 1ms median", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		defer st.Close()
		mustWrite(t, st, 1, day1) // warm: aliases, segment, metadata exist
		line, _ := st.Write([]perfdata.Draft{toolDraft(day1, 1)}, false)
		size := len(line.Lines[0]) + 1
		var best, bestP95, bestMax, bestFloor time.Duration
		for r := 0; r < appendRounds; r++ {
			lat := make([]time.Duration, appendPerRound)
			for i := range lat {
				d := toolDraft(day1, int64(i))
				d.Ctx.Invocation = "run-bench" // one invocation, as within one process run: no alias minting
				start := time.Now()
				if _, err := st.Write([]perfdata.Draft{d}, false); err != nil {
					t.Fatal(err)
				}
				lat[i] = time.Since(start)
			}
			p50, p95, mx := quantiles(lat)
			floor := fsFloor(t, t.TempDir(), append(line.Lines[0], '\n'), appendPerRound)
			t.Logf("round %d: locked append median=%v p95=%v max=%v | filesystem floor median=%v", r+1, p50, p95, mx, floor)
			if r == 0 || p50 < best {
				best, bestP95, bestMax, bestFloor = p50, p95, mx, floor
			}
		}
		t.Logf("locked append on %s/%s, concurrency 1, one %d-byte event, best of %d rounds of %d: median=%v p95=%v max=%v (floor %v)",
			runtime.GOOS, runtime.GOARCH, size, appendRounds, appendPerRound, best, bestP95, bestMax, bestFloor)
		switch {
		case best < appendTarget:
		case raceEnabled:
			t.Logf("race detector on: the %v target is reported, not enforced", appendTarget)
		case bestFloor >= slowHostFloor:
			t.Skipf("not a reference host: the bare open/append/close floor is %v (>= %v), so the host is loaded or slow and the %v target cannot be judged here",
				bestFloor, slowHostFloor, appendTarget)
		default:
			t.Fatalf("small-append median %v is not under %v on a host whose filesystem floor is %v", best, appendTarget, bestFloor)
		}
	})

	t.Run("recording on vs off adds bounded producer overhead", func(t *testing.T) {
		const n = 2000
		work := func() (int, error) { return 1, nil }
		mk := func(int, error, time.Duration) perfdata.Draft { return toolDraft(day1, 1) }
		measure := func(s *Sink) []time.Duration {
			out := make([]time.Duration, n)
			for i := range out {
				start := time.Now()
				_, _ = Observe(s, work, mk)
				out[i] = time.Since(start)
			}
			return out
		}
		off := measure(&Sink{})
		// The flush budget is the policy maximum so Close drains the queue on a slow host;
		// waiting on done keeps the writer from outliving the test's directory.
		sink, err := OpenSink(t.TempDir(), armed, testPolicy(t, `,"queue":4096,"flushBudgetMs":10000`), Options{Now: fixed(day1)})
		if err != nil {
			t.Fatal(err)
		}
		on := measure(sink)
		clean := sink.Close()
		<-sink.done
		if !clean {
			t.Fatalf("Close did not drain %d observations within the maximum budget: %+v", n, sink.Health())
		}
		o50, o95, omax := quantiles(off)
		n50, n95, nmax := quantiles(on)
		t.Logf("producer cost per observation (%d runs): off median=%v p95=%v max=%v | on median=%v p95=%v max=%v",
			n, o50, o95, omax, n50, n95, nmax)
		if n50 >= time.Millisecond && !raceEnabled {
			t.Fatalf("recording-on median producer cost %v is not under 1ms", n50)
		}
	})

	t.Run("slow I/O: the queue stays bounded and Close keeps its flush budget", func(t *testing.T) {
		dir := t.TempDir()
		// The writer is held inside its first append until released, so where every
		// observation ends up does not depend on how the scheduler interleaves it with
		// the producer: one is in the stalled write, the queue holds its capacity, and
		// the rest are refused at the queue.
		entered := make(chan struct{})
		var once sync.Once
		release := make(chan struct{})
		slow := func(op string) error {
			if op == "append" {
				once.Do(func() { close(entered) })
				select {
				case <-release:
				case <-time.After(10 * time.Second):
				}
			}
			return nil
		}
		const queue, budget = 8, 50 * time.Millisecond
		sink, err := OpenSink(dir, armed, testPolicy(t, `,"queue":8,"flushBudgetMs":50`), Options{Now: fixed(day1), fault: slow})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { // never leave the writer running into TempDir cleanup
			select {
			case <-release:
			default:
				close(release)
			}
			<-sink.done
		}()
		const n = 200
		start := time.Now()
		var worst time.Duration
		record := func(i int) {
			s := time.Now()
			sink.Record(toolDraft(day1, int64(i)))
			worst = max(worst, time.Since(s))
		}
		record(0)
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("the writer never reached its append")
		}
		for i := 1; i < n; i++ {
			record(i)
		}
		recordTotal := time.Since(start)
		if worst > 20*time.Millisecond {
			t.Fatalf("Record waited on slow storage: worst=%v", worst)
		}
		if got := len(sink.ch); got != queue || cap(sink.ch) != queue {
			t.Fatalf("queue not bounded by policy: len=%d cap=%d", got, cap(sink.ch))
		}
		closeStart := time.Now()
		clean := sink.Close()
		closeTook := time.Since(closeStart)
		close(release)
		select {
		case <-sink.done:
		case <-time.After(5 * time.Second):
			t.Fatal("writer goroutine did not finish")
		}
		h := sink.Health()
		t.Logf("slow I/O: record total=%v worst=%v close=%v health=%+v", recordTotal, worst, closeTook, h)
		if clean || closeTook > budget+200*time.Millisecond {
			t.Fatalf("Close must give up at the flush budget: clean=%v took %v", clean, closeTook)
		}
		want := perfdata.Drops{QueueFull: n - 1 - queue, FlushTimeout: queue}
		if h != want {
			t.Fatalf("dropped observations must be counted where they were dropped: got %+v, want %+v", h, want)
		}
		rr, err := Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		c := rr.Coverage[0]
		accounted := int64(len(rr.Events)) + h.QueueFull + h.FlushTimeout + h.WriteFailed + h.Refused
		if len(rr.Events) != 1 || accounted != n {
			t.Fatalf("every observation is written or counted exactly once: events=%d + drops %+v = %d, want 1 event and %d", len(rr.Events), h, accounted, n)
		}
		if c.UncleanTerminations != 1 || c.State != perfdata.CoveragePartial {
			t.Fatalf("an abandoned flush must leave coverage uncertain: %+v", c)
		}
	})
}
