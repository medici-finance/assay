package local

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

// regression: a Record racing or following Close was dropped without being counted.
func TestRecordAfterCloseCounted(t *testing.T) {
	t.Run("after a clean close", func(t *testing.T) {
		sink, err := OpenSink(t.TempDir(), armed, testPolicy(t, ""), Options{Now: fixed(day1)})
		if err != nil {
			t.Fatal(err)
		}
		if !sink.Close() {
			t.Fatal("an idle sink must close cleanly")
		}
		sink.Record(toolDraft(day1, 1))
		if h := sink.Health(); h.FlushTimeout != 1 {
			t.Fatalf("a Record after Close must be counted as a flush drop: %+v", h)
		}
	})
	t.Run("after an abandoned close", func(t *testing.T) {
		release := make(chan struct{})
		sink, _ := slowSink(t, release)
		sink.Close()
		before := sink.Health().FlushTimeout
		sink.Record(toolDraft(day1, 2))
		close(release)
		<-sink.done
		if h := sink.Health(); h.FlushTimeout != before+1 {
			t.Fatalf("a Record after an abandoned Close must be counted: before=%d now %+v", before, h)
		}
	})
}

// regression: an abandoned close never closed the store's lock file.
func TestAbandonClosesStoreLock(t *testing.T) {
	release := make(chan struct{})
	sink, _ := slowSink(t, release)
	if sink.Close() {
		t.Fatal("the close must be abandoned while the writer is stalled")
	}
	close(release)
	<-sink.done
	deadline := time.Now().Add(2 * time.Second)
	for {
		sink.st.mu.Lock()
		open := sink.st.lockF != nil
		sink.st.mu.Unlock()
		if !open {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the store lock file is still open after the abandoned writer finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// slowSink opens a sink whose writer stalls inside its first append until release is
// closed, and returns once that writer is stalled.
func slowSink(t *testing.T, release chan struct{}) (*Sink, string) {
	t.Helper()
	dir := t.TempDir()
	entered := make(chan struct{})
	var once sync.Once
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
	sink, err := OpenSink(dir, armed, testPolicy(t, `,"queue":4,"flushBudgetMs":20`), Options{Now: fixed(day1), fault: slow})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		<-sink.done
	})
	sink.Record(toolDraft(day1, 1))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never reached its append")
	}
	return sink, dir
}

// regression: the default 1 MiB segment outgrew a smaller byte cap and was never retired.
func TestSegmentFitsByteCap(t *testing.T) {
	const byteCap = 65536
	p := testPolicy(t, "")
	p.ByteCap = byteCap
	p.SegmentBytes = 0 // as a policy that names only its byte cap
	dp, err := perfdata.DecodePolicy([]byte(`{"schema":"performance-policy-v1","revision":"r1","profiles":["tool"],` +
		`"tools":["statusgen"],"ageCapDays":7,"byteCap":65536}`))
	if err != nil {
		t.Fatal(err)
	}
	if dp.SegmentBytes > byteCap {
		t.Fatalf("a decoded policy defaults its segment past its byte cap: %d > %d", dp.SegmentBytes, byteCap)
	}
	dir := t.TempDir()
	st := mustOpen(t, dir, p, Options{Now: fixed(day1)})
	defer st.Close()
	mustWrite(t, st, 300, day1)
	total := int64(0)
	for _, sg := range listSegments(Root(dir)) {
		size, _ := regularSize(filepath.Join(Root(dir), "segments", sg.name))
		total += size
	}
	// Retirement runs when a segment rolls over, so the bound is the cap plus the one
	// open segment, and that segment is never larger than the cap.
	if total > 2*byteCap {
		t.Fatalf("segments hold %d bytes against a %d-byte cap", total, byteCap)
	}
}

// regression: coverage.json was decoded loosely and its source names reached the report.
func TestCoverageRefusesBadSource(t *testing.T) {
	cases := map[string]string{
		"control text as a source": `{"schema":"performance-coverage-v1","retired":[],"lost":[{"source":"\u001b[2Jx","range":{"from":1,"to":2}}],"drops":{},"unclean":0,"epochResets":[]}`,
		"an unknown field":         `{"schema":"performance-coverage-v1","retired":[],"lost":[],"drops":{},"unclean":0,"epochResets":[],"extra":1}`,
		"trailing data":            `{"schema":"performance-coverage-v1","retired":[],"lost":[],"drops":{},"unclean":0,"epochResets":[]} {}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
			mustWrite(t, st, 1, day1)
			st.Close()
			if err := os.WriteFile(filepath.Join(Root(dir), "coverage.json"), []byte(doc), 0o600); err != nil {
				t.Fatal(err)
			}
			rr, err := Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range rr.Coverage {
				if strings.ContainsRune(c.Source, '\x1b') {
					t.Fatalf("a control-text source reached the report: %q", c.Source)
				}
			}
			if rr.Coverage[0].CorruptMetadata == 0 {
				t.Fatalf("a malformed coverage file must be reported corrupt: %+v", rr.Coverage[0])
			}
		})
	}
}

// regression: a new live marker was visible, unlocked, before its writer locked it, so a
// concurrent Open could reap it as an unclean end.
func TestLiveMarkerBornLocked(t *testing.T) {
	if !liveMarkersSupported {
		t.Skip("live markers are not lock-checked on this platform")
	}
	dir := t.TempDir()
	openOnce(t, dir)
	run := filepath.Join(Root(dir), "run")
	var reapable []string
	// At the moment between creating and locking its marker, do what a concurrent
	// Open's reaper does: any live- marker whose lock is free would be removed.
	probe := func(op string) error {
		if op != "live" {
			return nil
		}
		ents, _ := os.ReadDir(run)
		for _, e := range ents {
			if !strings.HasPrefix(e.Name(), "live-") {
				continue
			}
			f, err := os.OpenFile(filepath.Join(run, e.Name()), os.O_RDWR, 0)
			if err != nil {
				continue
			}
			if tryLockFile(f) {
				reapable = append(reapable, e.Name())
				unlockFile(f)
			}
			f.Close()
		}
		return nil
	}
	st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1), fault: probe})
	st.Close()
	if len(reapable) != 0 {
		t.Fatalf("a concurrent reaper could take the new writer's unlocked marker: %v", reapable)
	}
	rr, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c := rr.Coverage[0]; c.UncleanTerminations != 0 {
		t.Fatalf("a clean open and close must not read as unclean: %+v", c)
	}
}
