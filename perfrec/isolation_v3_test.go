package perfrec

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

// regression: class perf-v3-drop-coverage
func TestV3DroppedCoverage(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	s := BeginSelected(Probe{}, Probe{}.Mark(), nil, v3Context("tool"))
	if s.ObserveRequest(RequestObservation{}) {
		t.Fatal("invalid request accepted")
	}
	s.ObserveRequests()
	r := s.Finish("consumer", "run", "", 0, "")
	if r.V3.RequestCoverage != CoverageFailed || r.V3.RequestDrops != 1 {
		t.Fatal("later seam observation erased drops")
	}
	r.V3.RequestCoverage = CoverageMeasured
	if (&Writer{}).Write(t.TempDir(), r) == nil {
		t.Fatal("writer accepted complete dropped coverage")
	}
}

// regression: class perf-v3-optional-isolation
func TestPerfV3IsolationAndOverhead(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	self, e := filepath.Abs(".")
	if e != nil {
		t.Fatal(e)
	}
	consumer := consumerModule(t, self, "")
	main := `package main
import("os";"github.com/medici-finance/assay/perfrec")
func main(){c:=perfrec.ResolveContext(perfrec.ContextInput{});s:=perfrec.BeginSelected(perfrec.SystemProbe(),perfrec.ProcessEntry(),nil,c);if perfrec.Enabled(){perfrec.Append(s.Finish("consumer","run","",0,""))};os.Stdout.WriteString("result\n")}`
	if e = os.WriteFile(filepath.Join(consumer, "main.go"), []byte(main), 0600); e != nil {
		t.Fatal(e)
	}
	if bad := closureViolations(t, consumer); len(bad) != 0 {
		t.Fatal(bad)
	}
	bin := filepath.Join(t.TempDir(), "consumer")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, e := goCmd(t, consumer, "build", "-o", bin, "."); e != nil {
		t.Fatalf("external build %v %s", e, out)
	}
	run := func(home, armed string) ([]byte, error) {
		cmd := exec.Command(bin)
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, EnvVar+"="+armed, "ASSAY_PERF_PROFILE=workflow", "ASSAY_PERF_AGENT_ID=invalid")
		return cmd.CombinedOutput()
	}
	offHome, onHome := t.TempDir(), t.TempDir()
	off, e := run(offHome, "")
	if e != nil {
		t.Fatal(e)
	}
	on, e := run(onHome, "1")
	if e != nil || !bytes.Equal(off, on) {
		t.Fatal("measurement changed output", e)
	}
	if files, _ := os.ReadDir(offHome); len(files) != 0 {
		t.Fatal("off created state")
	}
	files, _ := filepath.Glob(filepath.Join(onHome, ".config", "assay", "perf", "*.jsonl"))
	if len(files) != 1 {
		t.Fatal("v3 missing")
	}
	b, _ := os.ReadFile(files[0])
	r, e := Decode(bytes.TrimSpace(b))
	if e != nil || r.Schema != SchemaV3 || r.V3.Attribution.Coverage[FieldAgent] != CoverageInvalid {
		t.Fatal("external context", e)
	}
	failed := t.TempDir()
	if e = os.WriteFile(filepath.Join(failed, ".config"), []byte("blocked"), 0600); e != nil {
		t.Fatal(e)
	}
	got, e := run(failed, "1")
	if e != nil || !bytes.Equal(got, off) {
		t.Fatal("failed optional sink changed operation", e)
	}
	// Matched workloads and alternating enabled/disabled runs; append is included in overhead.
	state := t.TempDir()
	w := Writer{}
	var offNS, onNS []int64
	sample := v3Record(v3Context("tool"))
	sample.Install = string(bytes.Repeat([]byte("0"), 32))
	encoded, _ := json.Marshal(sample)
	payloadBytes := len(encoded) + 1
	for i := 0; i < 100; i++ {
		for _, armed := range []bool{false, true} {
			start := time.Now()
			n := 0
			for j := 0; j < 1000; j++ {
				n += j
			}
			if n == 0 {
				t.Fatal("workload")
			}
			if armed {
				r := v3Record(v3Context("tool"))
				if e = w.Write(state, r); e != nil {
					t.Fatal(e)
				}
			}
			elapsed := time.Since(start).Nanoseconds()
			if armed {
				onNS = append(onNS, elapsed)
			} else {
				offNS = append(offNS, elapsed)
			}
		}
	}
	sort.Slice(offNS, func(i, j int) bool { return offNS[i] < offNS[j] })
	sort.Slice(onNS, func(i, j int) bool { return onNS[i] < onNS[j] })
	t.Logf("matched workload n=100 concurrency=1 bytes/record=%d drops=0 off ns p50=%d p95=%d max=%d; on ns p50=%d p95=%d max=%d; differences ns p50=%d p95=%d max=%d", payloadBytes, offNS[49], offNS[94], offNS[99], onNS[49], onNS[94], onNS[99], onNS[49]-offNS[49], onNS[94]-offNS[94], onNS[99]-offNS[99])
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				if e := w.Write(state, v3Record(v3Context("workflow"))); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
	day := filepath.Join(Dir(state), time.Now().UTC().Format("2006-01-02")+".jsonl")
	data, e := os.ReadFile(day)
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if _, e := Decode(line); e != nil {
			t.Fatal("concurrent line corrupted", e)
		}
	}
	t.Logf("concurrency=4 records=%d bytes=%d drops=0", len(bytes.Split(bytes.TrimSpace(data), []byte("\n"))), len(data))
}
