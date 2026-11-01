package local

import (
	"runtime"
	"sort"
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

func TestLocalCostAndBounds(t *testing.T) {
	t.Run("a small locked append stays under 1ms median", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		defer st.Close()
		mustWrite(t, st, 1, day1) // warm: aliases, segment, metadata exist
		const n = 300
		lat := make([]time.Duration, n)
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
		line, _ := st.Write([]perfdata.Draft{toolDraft(day1, 1)}, false)
		t.Logf("locked append on %s/%s, concurrency 1, one %d-byte event, %d runs: median=%v p95=%v max=%v",
			runtime.GOOS, runtime.GOARCH, len(line.Lines[0])+1, n, p50, p95, mx)
		if p50 >= time.Millisecond && !raceEnabled {
			t.Fatalf("small-append median %v is not under 1ms", p50)
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
		sink, err := OpenSink(t.TempDir(), armed, testPolicy(t, `,"queue":4096`), Options{Now: fixed(day1)})
		if err != nil {
			t.Fatal(err)
		}
		on := measure(sink)
		sink.Close()
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
		release := make(chan struct{})
		slow := func(op string) error {
			if op == "append" {
				select {
				case <-release:
				case <-time.After(400 * time.Millisecond):
				}
			}
			return nil
		}
		sink, err := OpenSink(dir, armed, testPolicy(t, `,"queue":8,"flushBudgetMs":50`), Options{Now: fixed(day1), fault: slow})
		if err != nil {
			t.Fatal(err)
		}
		const n = 200
		start := time.Now()
		var worst time.Duration
		for i := 0; i < n; i++ {
			s := time.Now()
			sink.Record(toolDraft(day1, int64(i)))
			worst = max(worst, time.Since(s))
		}
		recordTotal := time.Since(start)
		if recordTotal > 100*time.Millisecond || worst > 20*time.Millisecond {
			t.Fatalf("Record waited on slow storage: total=%v worst=%v", recordTotal, worst)
		}
		if got := len(sink.ch); got > cap(sink.ch) || cap(sink.ch) != 8 {
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
		if clean || closeTook > 250*time.Millisecond {
			t.Fatalf("Close must give up at the flush budget: clean=%v took %v", clean, closeTook)
		}
		if h.QueueFull == 0 || h.FlushTimeout == 0 {
			t.Fatalf("dropped observations must be counted: %+v", h)
		}
		rr, err := Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		c := rr.Coverage[0]
		accounted := int64(len(rr.Events)) + h.QueueFull + h.FlushTimeout + h.WriteFailed + h.Refused
		if accounted != n {
			t.Fatalf("every observation is written or counted exactly once: events=%d + drops %+v = %d, want %d", len(rr.Events), h, accounted, n)
		}
		if c.UncleanTerminations != 1 || c.State != perfdata.CoveragePartial {
			t.Fatalf("an abandoned flush must leave coverage uncertain: %+v", c)
		}
	})
}
