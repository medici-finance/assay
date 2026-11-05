package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

type perfSize struct {
	Streams         int
	BriefsPerStream int
}

func perfWorkRoot(t *testing.T, size perfSize) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	// Retain only the reusable non-stream fixture inputs.
	if err := os.RemoveAll(filepath.Join(root, "docs/streams/alpha")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/perfworkload/brief.md")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < size.Streams; i++ {
		name := fmt.Sprintf("work-%02d", i)
		dir := filepath.Join(root, "docs/streams", name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		var board strings.Builder
		fmt.Fprintf(&board, "---\nstream: %s\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# Workload\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n|---|---|---|---|---|---|\n", name)
		for j := 1; j <= size.BriefsPerStream; j++ {
			num := fmt.Sprintf("%02d", j)
			state := "todo"
			if j%3 == 0 {
				state = "implemented"
			}
			fmt.Fprintf(&board, "| %s | [Unit](brief-%s.md) | 0 | %s | — | — |\n", num, num, state)
			text := strings.NewReplacer("{{stream}}", name, "{{num}}", num).Replace(string(raw))
			writeTestFile(t, filepath.Join(dir, "brief-"+num+".md"), text)
		}
		writeTestFile(t, filepath.Join(dir, "README.md"), board.String())
	}
	return root
}

// capturePerfOutput uses files rather than bounded pipes, because the large
// corpus deliberately retains every diagnostic from the real source lint.
func capturePerfOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	diag, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	defer diag.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	os.Stdout, os.Stderr = out, diag
	fn()
	a, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(diag.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(a), string(b)
}

type perfWorkSample struct {
	Wall                float64
	CPU                 *int64
	FileReads, DirReads int
	Parses              int64
}

func perfWorkRun(t *testing.T, root, mode string) (snapshotResult, perfWorkSample) {
	t.Helper()
	f, d := perfMemoCounts()
	p := briefParseCount.Load()
	cpu, available := perfrec.CPUMillis()
	start := time.Now()
	result := snapshotRun(t, root, mode)
	sample := perfWorkSample{Wall: float64(time.Since(start)) / float64(time.Millisecond)}
	if end, ok := perfrec.CPUMillis(); available && ok {
		delta := end - cpu
		sample.CPU = &delta
	}
	ff, dd := perfMemoCounts()
	sample.FileReads, sample.DirReads, sample.Parses = ff-f, dd-d, briefParseCount.Load()-p
	return result, sample
}

func perfQuantile(samples []float64, q float64) float64 {
	sort.Float64s(samples)
	return samples[int(math.Ceil(q*float64(len(samples))))-1]
}

func perfMetric(samples []perfWorkSample, get func(perfWorkSample) float64) string {
	values := make([]float64, len(samples))
	for i, s := range samples {
		values[i] = get(s)
	}
	return fmt.Sprintf("%.3f / %.3f", perfQuantile(values, .5), perfQuantile(values, .95))
}

// This is an explicitly requested benchmark, skipped during ordinary CI. The
// public script supplies the report path, sample count and source revision.
// regression: statusgen/26
func TestPerfWorkloadSamples(t *testing.T) {
	reportPath := os.Getenv("ASSAY_PERF_WORKLOAD_REPORT")
	if reportPath == "" {
		t.Skip("run bench/perfworkload.sh to request local workload measurements")
	}
	snapshotSetup(t)
	samples, err := strconv.Atoi(os.Getenv("ASSAY_PERF_WORKLOAD_SAMPLES"))
	if err != nil || samples < 2 || samples > 100 {
		t.Fatal("sample count must be 2..100")
	}
	raw, err := os.ReadFile("testdata/perfworkload/sizes.json")
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[string]perfSize{}
	if err := json.Unmarshal(raw, &sizes); err != nil {
		t.Fatal(err)
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# Statusgen synthetic workload\n\nSource: `%s`. Environment: `%s`, `%s/%s`. Samples per row: %d.\n\n", os.Getenv("ASSAY_PERF_WORKLOAD_SOURCE"), runtime.Version(), runtime.GOOS, runtime.GOARCH, samples)
	report.WriteString("Cold means cleared invocation-local read/parse memos; warm is the immediately following replay over the same inputs in the same process. OS caches are uncontrolled. No persistent cache or deployment speedup is measured. The measured boundary includes the operational owner, coverage projection, output capture and generated-file comparison. CPU is process user+system, without child CPU. File/directory counts cover only readFileMemo/readDirMemo disk attempts (including errors); other file reads are unmeasured. Parses count canonical brief parses. Every measured replay matches a memo-cleared reference byte-for-byte for stdout, stderr, exit, generated files and coverage. Each operational mode and coverage projection must exit zero, and cold parsing must cover every declared brief while warm parsing performs zero new parses.\n\n")
	report.WriteString("| Corpus | Mode | State | Wall ms p50 / p95 | CPU ms p50 / p95 | Memo file reads p50 / p95 | Memo dir reads p50 / p95 | Brief parses p50 / p95 |\n|---|---|---|---|---|---|---|---|\n")
	lab := newPerfLab(t)
	var cliRows strings.Builder
	cliRows.WriteString("\n| Corpus | Compiled CLI mode | Process wall ms p50 / p95 | Process CPU ms p50 / p95 | Actual deskread starts |\n|---|---|---|---|---|\n")
	for _, name := range []string{"small", "large"} {
		root := perfWorkRoot(t, sizes[name])
		// Real settle interval, once per corpus: no clock manipulation in measurements.
		time.Sleep(readMemoSettleWindow + 100*time.Millisecond)
		for _, mode := range []string{"lint", "regen", "gate-scores", "next-up"} {
			cold, warm := []perfWorkSample{}, []perfWorkSample{}
			for i := 0; i < samples; i++ {
				resetPerfInputs()
				reference := snapshotRun(t, root, mode)
				if reference.Code != 0 || reference.CoverageCode != 0 {
					t.Fatalf("%s/%s fixture failed: exit=%d coverage=%d\n%s", name, mode, reference.Code, reference.CoverageCode, reference.Err)
				}
				resetPerfInputs()
				got, c := perfWorkRun(t, root, mode)
				replay, w := perfWorkRun(t, root, mode)
				if !reflect.DeepEqual(reference, got) || !reflect.DeepEqual(reference, replay) {
					t.Fatalf("%s/%s sample %d changed output/exit/coverage", name, mode, i)
				}
				if c.Parses != int64(sizes[name].Streams*sizes[name].BriefsPerStream) || w.Parses != 0 {
					t.Fatalf("%s/%s did not parse the declared corpus once then reuse it: cold=%d warm=%d", name, mode, c.Parses, w.Parses)
				}
				cold = append(cold, c)
				warm = append(warm, w)
			}
			var walls, cpus []float64
			args := []string{"--root", root}
			if mode != "regen" {
				args = append(args, "--"+mode)
			}
			for i := 0; i < samples; i++ {
				off := lab.run(t, []string{"DESK_LOOP=worker-desk"}, args...)
				artifacts := perfWorkFiles(t, root)
				on, line := lab.one(t, []string{"ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=tool", "DESK_LOOP=worker-desk"}, args...)
				if !reflect.DeepEqual(off, on) || !reflect.DeepEqual(artifacts, perfWorkFiles(t, root)) {
					t.Fatalf("compiled %s/%s parity changed", name, mode)
				}
				rec := decodeV3(t, line)
				if phaseNames(rec)["deskread"].N != 0 {
					t.Fatal("offline workload started deskread")
				}
				walls = append(walls, float64(rec.WallMs))
				if rec.CPUMs != nil {
					cpus = append(cpus, float64(*rec.CPUMs))
				}
			}
			cpuMetric := "unavailable"
			if len(cpus) == samples {
				cpuMetric = fmt.Sprintf("%.3f / %.3f", perfQuantile(cpus, .5), perfQuantile(cpus, .95))
			}
			fmt.Fprintf(&cliRows, "| %s | %s | %.3f / %.3f | %s | 0 |\n", name, mode, perfQuantile(walls, .5), perfQuantile(walls, .95), cpuMetric)
			for i, state := range []string{"cold", "warm"} {
				set := [][]perfWorkSample{cold, warm}[i]
				cpu := "unavailable"
				if set[0].CPU != nil {
					cpu = perfMetric(set, func(s perfWorkSample) float64 { return float64(*s.CPU) })
				}
				fmt.Fprintf(&report, "| %s (%d × %d) | %s | %s | %s | %s | %s | %s | %s |\n", name, sizes[name].Streams, sizes[name].BriefsPerStream, mode, state,
					perfMetric(set, func(s perfWorkSample) float64 { return s.Wall }), cpu,
					perfMetric(set, func(s perfWorkSample) float64 { return float64(s.FileReads) }), perfMetric(set, func(s perfWorkSample) float64 { return float64(s.DirReads) }), perfMetric(set, func(s perfWorkSample) float64 { return float64(s.Parses) }))
			}
		}
	}
	// The existing lab compiles the real main/flag/exit path, uses an isolated
	// HOME, and places credential-free fake children before every online command.
	report.WriteString("\nThe compiled CLI rows below are separate fresh processes using the shared selected-profile record. Every sample has a recording-off reference and generated-file parity check. They measure the process-init boundary, including the real flag parser and exit recorder, after Go runtime startup. Warm OS state is uncontrolled; invocation memos cannot cross this boundary. Read/parse counts for these process records are unavailable; the owner-local replay rows above describe those counters separately.\n")
	report.WriteString(cliRows.String())
	var childWall, childCPU, childStarts []float64
	for i := 0; i < samples; i++ {
		before := len(lab.children(t))
		baseline := lab.run(t, []string{"DESK_LOOP=worker-desk"}, "--lint", "--forge", "--root", ".")
		on, line := lab.one(t, []string{"ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=tool", "DESK_LOOP=worker-desk"}, "--lint", "--forge", "--root", ".")
		if !reflect.DeepEqual(baseline, on) {
			t.Fatal("fake deskread recording changed output/exit")
		}
		rec := decodeV3(t, line)
		childWall = append(childWall, float64(rec.WallMs))
		if rec.CPUMs != nil {
			childCPU = append(childCPU, float64(*rec.CPUMs))
		}
		count := len(lab.children(t)) - before
		if count != 2 || phaseNames(rec)["deskread"].N != 1 {
			t.Fatalf("fake child starts=%d phase=%+v", count, rec.Phases)
		}
		childStarts = append(childStarts, float64(count/2))
	}
	fmt.Fprintf(&report, "\nCompiled fake-deskread parent (%d recorded samples; each checked against a recording-off run): process wall ms p50/p95 %.3f / %.3f; actual child starts per run %.0f / %.0f. The fake child returns unavailable (exit 6), preserved as unavailable; it reports no request costs. Help probes are excluded from actual child starts and child phases.\n", samples, perfQuantile(childWall, .5), perfQuantile(childWall, .95), perfQuantile(childStarts, .5), perfQuantile(childStarts, .95))
	if len(childCPU) == samples {
		fmt.Fprintf(&report, "Parent process CPU ms p50/p95 %.3f / %.3f.\n", perfQuantile(childCPU, .5), perfQuantile(childCPU, .95))
	} else {
		report.WriteString("Parent CPU unavailable for one or more samples.\n")
	}
	report.WriteString("\nCoverage limits: synthetic local corpora, no providers or live permissions, no controlled OS-cold disk, no installed fleet baseline. Context discovery does not run inside these four modes: no context index reuse or savings is claimed.\n")
	if os.Getenv("ASSAY_SNAPSHOT_MUTANT") == "" {
		snapshotMutant(t, "recording-output", "perfrecord.go", "func recordPerf(exit int) {", `func recordPerf(exit int) {
            if perfrec.Enabled() { os.Stdout.WriteString("planted-recording-output\n") }`, "TestPerfWorkloadSamples")
	}
	if err := os.WriteFile(reportPath, []byte(report.String()), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d workload rows; output/exit/coverage parity checked", 16)
}

func perfWorkFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, p := range []string{"STATUS.md", "docs/streams/FINDINGS.md", "docs/streams/INTAKE.md"} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			files[p] = string(b)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	return files
}
