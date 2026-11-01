package local

// sink.go — the best-effort recording contract. A producer hands observations to a
// bounded queue and never waits on storage: a full queue drops the observation and
// counts it, a failed write (a full disk) is counted and never returned to the caller,
// and Close waits at most the policy's flush budget. A clean Close writes one health
// event with the drop counters and write latency; a Close that runs out of budget
// leaves its live marker behind, so the next reader reports uncertain coverage.

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

// maxSinkBatch bounds one locked append from the sink.
const maxSinkBatch = 64

// Sink is a bounded asynchronous recorder. The zero value and nil are disabled sinks:
// Record is a no-op and Close returns immediately.
type Sink struct {
	st     *Store
	ch     chan perfdata.Draft
	done   chan struct{}
	budget time.Duration

	queueFull, writeFailed, flushTimeout, refused atomic.Int64
	abandoned                                     atomic.Bool
	closeOnce                                     sync.Once

	latMu sync.Mutex
	lat   perfdata.Distribution
}

// OpenSink opens a sink. When collection is off it returns a disabled sink and creates
// nothing; when the store cannot be opened it returns a disabled sink and the error, so
// a caller can ignore the error without changing its own behavior.
func OpenSink(stateDir string, c perfdata.Consent, p perfdata.Policy, o Options) (*Sink, error) {
	if !c.Armed {
		return &Sink{}, nil
	}
	st, err := Open(stateDir, c, p, o)
	if err != nil {
		return &Sink{}, err
	}
	q := p.Queue
	if q <= 0 {
		q = perfdata.DefaultQueue
	}
	b := p.FlushBudgetMs
	if b <= 0 {
		b = perfdata.DefaultFlushBudgetMs
	}
	s := &Sink{st: st, ch: make(chan perfdata.Draft, q), done: make(chan struct{}), budget: time.Duration(b) * time.Millisecond, lat: perfdata.NewDistribution()}
	go s.run()
	return s, nil
}

// Enabled reports whether the sink records anything.
func (s *Sink) Enabled() bool { return s != nil && s.st != nil }

// Record queues one observation without blocking.
func (s *Sink) Record(d perfdata.Draft) {
	if !s.Enabled() || s.abandoned.Load() {
		return
	}
	defer func() { _ = recover() }() // a Record racing Close is dropped, never a panic
	select {
	case s.ch <- d:
	default:
		s.queueFull.Add(1)
	}
}

func (s *Sink) run() {
	defer close(s.done)
	batch := make([]perfdata.Draft, 0, maxSinkBatch)
	for d := range s.ch {
		batch = append(batch[:0], d)
	fill:
		for len(batch) < maxSinkBatch {
			select {
			case d2, ok := <-s.ch:
				if !ok {
					break fill
				}
				batch = append(batch, d2)
			default:
				break fill
			}
		}
		if s.abandoned.Load() {
			s.flushTimeout.Add(int64(len(batch)))
			continue
		}
		start := time.Now()
		res, err := s.st.Write(batch, true)
		ms := time.Since(start).Milliseconds()
		s.latMu.Lock()
		s.lat.Add(ms)
		if len(s.lat.Samples) > perfdata.MaxEventSamples {
			s.lat.Samples = nil // keep bins only; memory stays bounded
		}
		s.latMu.Unlock()
		s.refused.Add(int64(len(res.Refused)))
		if err != nil {
			s.writeFailed.Add(int64(len(batch) - len(res.Refused)))
		}
	}
}

// Health returns the current drop counters.
func (s *Sink) Health() perfdata.Drops {
	if s == nil {
		return perfdata.Drops{}
	}
	return perfdata.Drops{QueueFull: s.queueFull.Load(), WriteFailed: s.writeFailed.Load(), FlushTimeout: s.flushTimeout.Load(), Refused: s.refused.Load()}
}

// Close flushes within the flush budget. It reports whether the flush completed.
func (s *Sink) Close() (clean bool) {
	if !s.Enabled() {
		return true
	}
	clean = true
	s.closeOnce.Do(func() {
		close(s.ch)
		timer := time.NewTimer(s.budget)
		defer timer.Stop()
		select {
		case <-s.done:
		case <-timer.C:
			s.abandoned.Store(true)
			// Drain what is still queued; the writer goroutine counts any it takes
			// itself, so each queued observation is counted exactly once.
			for range s.ch {
				s.flushTimeout.Add(1)
			}
			clean = false
		}
		if !clean {
			// The writer may still hold the store; record what can be recorded
			// without waiting on it, and leave the live marker so coverage reads
			// as uncertain.
			s.st.abandon()
			return
		}
		s.latMu.Lock()
		lat := s.lat
		s.latMu.Unlock()
		if !lat.Exact() {
			lat.Samples = nil
		}
		h := s.Health()
		payload, _ := json.Marshal(perfdata.Health{QueueFull: h.QueueFull, WriteFailed: h.WriteFailed, FlushTimeout: h.FlushTimeout, Refused: h.Refused, WriteLatency: lat})
		if _, err := s.st.Write([]perfdata.Draft{{Kind: perfdata.KindHealth, Payload: payload}}, true); err != nil {
			s.st.mu.Lock()
			s.st.addDropsBestEffort(h)
			s.st.mu.Unlock()
		}
		s.st.Close()
	})
	return clean
}

// addDropsBestEffort records drops in coverage when the health event could not be
// written. Caller holds s.mu. It waits only briefly for the store lock.
func (s *Store) addDropsBestEffort(d perfdata.Drops) {
	if s.lockF == nil || lockFile(s.lockF, 20*time.Millisecond) != nil {
		return
	}
	defer unlockFile(s.lockF)
	cov := s.loadCoverage()
	cov.Drops.Add(d)
	_ = s.saveCoverage(cov)
}

// Observe runs fn, records the draft mk builds from its result and elapsed time, and
// returns fn's result unchanged — recording can never alter, delay beyond the queue
// hand-off, or fail the wrapped operation.
func Observe[T any](s *Sink, fn func() (T, error), mk func(T, error, time.Duration) perfdata.Draft) (T, error) {
	start := time.Now()
	v, err := fn()
	if s.Enabled() {
		func() {
			defer func() { _ = recover() }()
			s.Record(mk(v, err, time.Since(start)))
		}()
	}
	return v, err
}
