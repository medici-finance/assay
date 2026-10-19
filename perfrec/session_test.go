package perfrec

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// fakeProbe is an injected clock and CPU counter the test advances by hand.
type fakeProbe struct {
	now    time.Time
	cpu    int64
	cpuOK  bool
	noCPU  bool
	failAt int // the Nth CPU read (1-based) fails; 0 = never
	reads  int
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{now: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC), cpu: 100, cpuOK: true}
}

func (f *fakeProbe) advance(wall time.Duration, cpuMs int64) { f.now = f.now.Add(wall); f.cpu += cpuMs }

func (f *fakeProbe) probe() Probe {
	p := Probe{Now: func() time.Time { return f.now }}
	if !f.noCPU {
		p.CPU = func() (int64, bool) {
			f.reads++
			if f.failAt == f.reads {
				return 0, false
			}
			return f.cpu, f.cpuOK
		}
	}
	return p
}

// boundaryCovers is the parity oracle: the record's wall AND cpu must both include the
// setup interval that ran between the declared boundary and the work.
func boundaryCovers(r Record, wallMs, cpuMs int64) error {
	if r.WallMs != wallMs {
		return errors.New("wall does not start at the boundary")
	}
	if r.CPU != CPUMeasured || r.CPUMs == nil || *r.CPUMs != cpuMs {
		return errors.New("cpu does not start at the boundary")
	}
	return nil
}

// TestInvocationBoundaryAndCPUParity: wall and CPU deltas start at ONE mark and end in
// Finish, so an injected setup delay (configuration/activation before the work) lands in both;
// absent or failed CPU is a coverage state, never zero; and a planted late start (the mark
// taken after setup, the pre-v2 PerfRun shape) or a planted process-lifetime CPU reading fails
// the parity oracle.
// regression: desktools-v2/61
func TestInvocationBoundaryAndCPUParity(t *testing.T) {
	// Production order: mark, setup (40 ms wall / 25 ms CPU), work (60 / 30), finish.
	f := newFakeProbe()
	p := f.probe()
	start := p.Mark()
	f.advance(40*time.Millisecond, 25) // configuration + activation
	s := Begin(p, start, nil)
	f.advance(60*time.Millisecond, 30) // the verb's own work
	rec := s.Finish("deskpr", "create", "worker-desk", 0, "abcdef1")
	if err := boundaryCovers(rec, 100, 55); err != nil {
		t.Fatalf("one boundary: %v (wall %d, cpu %v %v)", err, rec.WallMs, rec.CPU, rec.CPUMs)
	}
	if rec.Boundary != BoundaryCaller || rec.Schema != SchemaV2 {
		t.Fatalf("boundary/schema = %s/%s", rec.Boundary, rec.Schema)
	}
	// Writer cost is outside: time spent after Finish never changes the finished record.
	f.advance(time.Second, 500)
	if err := newWriter().Write(t.TempDir(), rec); err != nil || rec.WallMs != 100 || *rec.CPUMs != 55 {
		t.Fatalf("post-finish time leaked into the record: %v %+v", err, rec)
	}

	// Planted late start: the mark is taken AFTER setup — the oracle must reject it.
	f = newFakeProbe()
	p = f.probe()
	f.advance(40*time.Millisecond, 25)
	late := Begin(p, p.Mark(), nil)
	f.advance(60*time.Millisecond, 30)
	if err := boundaryCovers(late.Finish("deskpr", "create", "", 0, ""), 100, 55); err == nil {
		t.Fatal("a late start passed the boundary oracle")
	}
	// Planted process-lifetime CPU (the perf-v1 reading): wall from the mark, CPU from zero.
	f = newFakeProbe()
	p = f.probe()
	lifetime := p.Mark()
	lifetime.cpuMs = 0
	f.advance(100*time.Millisecond, 55)
	if err := boundaryCovers(Begin(p, lifetime, nil).Finish("deskpr", "create", "", 0, ""), 100, 55); err == nil {
		t.Fatal("process-lifetime CPU passed the boundary oracle")
	}

	// Absent CPU: unsupported build, failed start read, failed end read, a counter that went
	// backwards, and a mark with no CPU sample. None carries cpuMs; none reads as zero.
	for name, tc := range map[string]struct {
		setup func(*fakeProbe)
		mark  func(Probe) Mark
		want  string
	}{
		"unsupported": {func(f *fakeProbe) { f.noCPU = true }, Probe.Mark, CPUUnsupported},
		"start fails": {func(f *fakeProbe) { f.failAt = 1 }, Probe.Mark, CPUFailed},
		"end fails":   {func(f *fakeProbe) { f.failAt = 2 }, Probe.Mark, CPUFailed},
		"backwards":   {func(f *fakeProbe) {}, func(p Probe) Mark { m := p.Mark(); m.cpuMs = 1 << 40; return m }, CPUFailed},
		"missing":     {func(f *fakeProbe) {}, func(p Probe) Mark { return Mark{boundary: BoundaryProcess, at: p.now(), cpu: CPUMissing} }, CPUMissing},
	} {
		f := newFakeProbe()
		tc.setup(f)
		p := f.probe()
		m := tc.mark(p)
		f.advance(10*time.Millisecond, 5)
		r := Begin(p, m, nil).Finish("deskpr", "create", "", 0, "")
		b, err := json.Marshal(r)
		if r.CPU != tc.want || r.CPUMs != nil || err != nil || bytes.Contains(b, []byte("cpuMs")) {
			t.Errorf("%s: cpu %q cpuMs %v (%v) %s; want %q and no cpuMs", name, r.CPU, r.CPUMs, err, b, tc.want)
		}
		if r.WallMs != 10 {
			t.Errorf("%s: wall %d, want 10 — CPU coverage must not disturb wall", name, r.WallMs)
		}
	}

	// The process-init mark precedes every test (and every importer's main).
	entry := ProcessEntry()
	if entry.Boundary() != BoundaryProcess || !entry.At().Before(time.Now()) {
		t.Fatalf("process entry mark = %s at %v", entry.Boundary(), entry.At())
	}
	switch entry.CPU() {
	case CPUMeasured, CPUMissing, CPUUnsupported:
	default:
		t.Fatalf("process entry cpu state %q", entry.CPU())
	}

	// The real probe: a spin measured from a fresh mark is bounded by process-lifetime CPU.
	sp := SystemProbe()
	sess := Begin(sp, sp.Mark(), nil)
	for deadline := time.Now().Add(30 * time.Millisecond); time.Now().Before(deadline); {
	}
	real := sess.Finish("deskpr", "create", "", 0, "")
	if runtime.GOOS == "windows" {
		if real.CPU != CPUUnsupported {
			t.Fatalf("windows cpu = %q, want unsupported", real.CPU)
		}
	} else if life, ok := CPUMillis(); real.CPU != CPUMeasured || !ok || *real.CPUMs > life || real.WallMs < 30 {
		t.Fatalf("system probe: cpu %q %v (lifetime %d), wall %d", real.CPU, real.CPUMs, life, real.WallMs)
	}
}

func fixtureLines(t *testing.T, name string) [][]byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][]byte
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := bytes.TrimSpace(sc.Bytes()); len(line) > 0 {
			out = append(out, append([]byte(nil), line...))
		}
	}
	return out
}

func keysOf(t *testing.T, line []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatal(err)
	}
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

const (
	v1Keys = "childProcs,childWallMs,exit,forgeCalls,install,result,role,schema,sourceSHA,tool,ts,verb,wallMs"
	v2Keys = "boundary,counters,cpu,exit,install,phaseOverlap,phases,result,role,schema,sourceSHA,tool,ts,unavailable,verb,wallMs"
)

func withoutCPUMs(keys string) string {
	return strings.Replace(strings.Replace(keys, "cpuMs,", "", 1), ",cpuMs", "", 1)
}

// TestPerformanceSchemaAndPrivacyCompatibility: committed perf-v1 and perf-v2 fixtures decode
// with their own meanings and re-encode byte for byte; a record carrying another schema's
// fields is refused in both directions (silent schema reuse); hostile labels never serialize;
// the switch off writes nothing; and a failing sink leaves the finished result untouched.
// regression: desktools-v2/61
func TestPerformanceSchemaAndPrivacyCompatibility(t *testing.T) {
	if SchemaV2 == Schema {
		t.Fatal("perf-v2 reuses the perf-v1 schema tag")
	}
	// Old fixtures keep their meanings: legacy counters, process-lifetime cpuMs, no coverage.
	for _, line := range fixtureLines(t, "perf-v1.jsonl") {
		r, err := Decode(line)
		if err != nil || r.Schema != Schema || r.hasV2Fields() {
			t.Fatalf("legacy line %s: %+v %v", line, r, err)
		}
		if withoutCPUMs(keysOf(t, line)) != v1Keys {
			t.Fatalf("legacy fixture key set drifted: %s", keysOf(t, line))
		}
		if b, err := json.Marshal(r); err != nil || !bytes.Equal(b, line) {
			t.Fatalf("legacy line did not round-trip:\n got  %s\n want %s (%v)", b, line, err)
		}
	}
	legacy, _ := Decode(fixtureLines(t, "perf-v1.jsonl")[0])
	if legacy.ChildProcs != 2 || legacy.ForgeCalls != 3 || legacy.CPUMs == nil || *legacy.CPUMs != 180 {
		t.Fatalf("legacy meanings lost: %+v", legacy)
	}
	// New fixtures: coverage partitions the roster; measured zero differs from unavailable.
	v2 := fixtureLines(t, "perf-v2.jsonl")
	for _, line := range v2 {
		r, err := Decode(line)
		if err != nil || r.Schema != SchemaV2 || r.hasLegacyCounters() {
			t.Fatalf("v2 line %s: %+v %v", line, r, err)
		}
		if withoutCPUMs(keysOf(t, line)) != v2Keys {
			t.Fatalf("v2 fixture key set drifted: %s", keysOf(t, line))
		}
		if b, err := json.Marshal(r); err != nil || !bytes.Equal(b, line) {
			t.Fatalf("v2 line did not round-trip:\n got  %s\n want %s (%v)", b, line, err)
		}
		if len(r.Counters)+len(r.Unavailable) != len(Counters()) {
			t.Fatalf("v2 coverage does not partition the roster: %+v", r)
		}
	}
	measured, _ := Decode(v2[0])
	zeroes, _ := Decode(v2[1])
	if _, ok := measured.Counters[CounterCacheHit]; ok || zeroes.Counters[CounterCacheHit] != 0 || len(zeroes.Unavailable) != 0 {
		t.Fatalf("unavailable and measured-zero cache hits are not distinguishable: %+v / %+v", measured, zeroes)
	}

	// Silent schema reuse fails, in every direction.
	state := t.TempDir()
	reuse := map[string]Record{
		"v1 tag with v2 field":    {Schema: Schema, Boundary: BoundaryProcess},
		"v1 tag with coverage":    {Schema: Schema, Unavailable: []Counter{CounterHTTP}},
		"v2 tag with v1 counters": func() Record { r := measured; r.ChildProcs = 2; return r }(),
		"unknown tag":             {Schema: "perf-v3"},
		"empty tag":               {},
	}
	for name, r := range reuse {
		if _, err := json.Marshal(r); err == nil {
			t.Errorf("%s: marshalled", name)
		}
		if err := newWriter().Write(state, r); err == nil {
			t.Errorf("%s: written", name)
		}
	}
	if day, _ := os.ReadFile(filepath.Join(Dir(state), dayName(0))); len(day) != 0 {
		t.Fatalf("a refused record reached the day file: %s", day)
	}
	for name, line := range map[string]string{
		"v1 line carrying v2 keys":  strings.Replace(string(fixtureLines(t, "perf-v1.jsonl")[0]), `"exit"`, `"boundary":"process-init","exit"`, 1),
		"v2 line carrying v1 keys":  strings.Replace(string(v2[0]), `"exit"`, `"childProcs":2,"exit"`, 1),
		"v2 line, no coverage":      strings.Replace(string(v2[0]), `"unavailable":["cacheLookup",`, `"unavailable":[`, 1),
		"v2 line, both lists":       strings.Replace(string(v2[0]), `"unavailable":["cacheLookup",`, `"unavailable":["http","cacheLookup",`, 1),
		"v2 line, cpuMs unmeasured": strings.Replace(string(v2[1]), `"cpu":"unsupported"`, `"cpu":"unsupported","cpuMs":0`, 1),
		"v2 line, unknown boundary": strings.Replace(string(v2[0]), `"process-init"`, `"main"`, 1),
		"v2 line, free-text phase":  strings.Replace(string(v2[0]), `"fetch"`, `"/home/x/repo"`, 1),
		"v1 tag on a v2 line":       strings.Replace(string(v2[0]), `"perf-v2"`, `"perf-v1"`, 1),
		"v2 tag on a v1 line":       strings.Replace(string(fixtureLines(t, "perf-v1.jsonl")[0]), `"perf-v1"`, `"perf-v2"`, 1),
	} {
		if r, err := Decode([]byte(line)); err == nil {
			t.Errorf("%s: decoded as %+v", name, r)
		}
	}

	// Hostile labels never serialize: tool, verb, role, phases and counters.
	hostile := []string{"/Users/example/private", "example-org/tracker", "ghp_" + strings.Repeat("a", 36), "PR 77",
		"--token=abc", "feat/branch", "Bad Label", "x" + strings.Repeat("y", 70)}
	f := newFakeProbe()
	p := f.probe()
	s := Begin(p, p.Mark(), append([]string{"fetch"}, hostile...))
	for _, h := range hostile {
		s.Phase(h)()
		s.Add(Counter(h), 7)
	}
	s.Phase("fetch")()
	for _, h := range hostile {
		r := s.Finish(h, h, h, 0, h)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("hostile %q: %v", h, err)
		}
		for _, needle := range hostile {
			if bytes.Contains(b, []byte(needle)) {
				t.Fatalf("hostile label %q serialized: %s", needle, b)
			}
		}
		if r.Tool != Other || r.Verb != Other || r.Role != Other || r.SourceSHA != Unpinned {
			t.Fatalf("hostile labels did not map to constants: %+v", r)
		}
		if len(r.Phases) != 2 || r.Phases[0].Name != "fetch" || r.Phases[1].Name != Other || r.Phases[1].N != int64(len(hostile)) {
			t.Fatalf("hostile phases not bucketed to other: %+v", r.Phases)
		}
		if len(r.Counters) != 0 {
			t.Fatalf("hostile counters measured: %+v", r.Counters)
		}
	}
	// Bounds: at most MaxPhases named phases; nil session costs nothing.
	var roster []string
	for i := 0; i < MaxPhases+8; i++ {
		roster = append(roster, "phase-"+string(rune('a'+i)))
	}
	if b := Begin(p, p.Mark(), roster); len(b.phases) != MaxPhases {
		t.Fatalf("phase roster not bounded: %d", len(b.phases))
	}
	var off *Session
	if allocs := testing.AllocsPerRun(100, func() { off.Phase("fetch")(); off.Add(CounterHTTP, 1); off.Observe(CounterHTTP) }); allocs != 0 {
		t.Fatalf("disabled session allocates %v per call", allocs)
	}

	// Disabled writes nothing, through the package Append.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, v := range []string{"", "0", "true"} {
		t.Setenv(EnvVar, v)
		Append(measured)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Fatalf("switch off, but the home gained %v", entries)
	}

	// Sink failure preserves the result: the perf path is a file, the write fails, and the
	// finished record (exit/result) is what it was.
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "perf"), []byte("squat"), 0o600); err != nil {
		t.Fatal(err)
	}
	f = newFakeProbe()
	p = f.probe()
	done := Begin(p, p.Mark(), nil).Finish("deskpr", "create", "", 5, "")
	before, _ := json.Marshal(done)
	if err := newWriter().Write(broken, done); err == nil {
		t.Fatal("the forced sink failure did not happen")
	}
	newWriter().Append(broken, done)
	after, _ := json.Marshal(done)
	if !bytes.Equal(before, after) || done.Exit != 5 || done.Result != ResultRefused {
		t.Fatalf("sink failure changed the result: %s -> %s", before, after)
	}

	// A real v2 write lands with restrictive modes and decodes as v2.
	t.Setenv(EnvVar, "1")
	Append(measured)
	dir := filepath.Join(home, ".config", "assay", "perf")
	st, err := os.Stat(dir)
	if err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o700) {
		t.Fatalf("perf dir mode: %v %v", st, err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("want one day file, got %v", files)
	}
	fi, _ := os.Stat(files[0])
	raw, _ := os.ReadFile(files[0])
	got, err := Decode(bytes.TrimSpace(raw))
	if err != nil || got.Schema != SchemaV2 || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("v2 append: %+v %v mode %v", got, err, fi.Mode())
	}
}
