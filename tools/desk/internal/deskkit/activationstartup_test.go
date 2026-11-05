package deskkit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/gitcore/gitstub"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// regression: F-activation-snapshot
func TestCompiledStartupDiscoveryWorkload(t *testing.T) {
	if startupBuildCacheErr != nil {
		t.Fatal(startupBuildCacheErr)
	}
	// Build the actual command packages, with a test-only observer overlay for work
	// accounting. Elapsed timings use separately built uninstrumented executables.
	desk, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	source := filepath.Join(desk, "internal", "deskkit", "activationindex.go")
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	observer := `var activationWork = func(event string) {
 if path := os.Getenv("ACTIVATION_TEST_TRACE"); path != "" {
  f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
  if err == nil { _, _ = f.WriteString(event+"\n"); _ = f.Close() }
 }
}`
	needle := "var activationWork = func(event string) {}"
	if strings.Count(string(body), needle) != 1 {
		t.Fatal("observer seam missing")
	}
	replacement := filepath.Join(scratch, "activationindex.go")
	snapshotFile(t, replacement, strings.Replace(string(body), needle, observer, 1))
	overlay := filepath.Join(scratch, "overlay.json")
	encoded, _ := json.Marshal(map[string]any{"Replace": map[string]string{source: replacement}})
	snapshotFile(t, overlay, string(encoded))
	build := func(tool string, observed bool) string {
		t.Helper()
		name := tool
		if observed {
			name += "-observed"
		}
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		bin := filepath.Join(scratch, name)
		args := []string{"build", "-o", bin}
		if observed {
			args = append(args, "-overlay", overlay)
		}
		args = append(args, "./cmd/"+tool)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = desk
		cmd.Env = []string{"GOWORK=off", "GOPROXY=off", "GOENV=off", "GOTOOLCHAIN=local",
			"GOMODCACHE=" + startupBuildCaches["GOMODCACHE"], "GOCACHE=" + startupBuildCaches["GOCACHE"]}
		for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TMPDIR", "TMP", "TEMP"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
		if out, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v %s", tool, err, out)
		}
		return bin
	}
	run := func(bin, root, home, telemetry, trace string, args ...string) (time.Duration, int, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = root
		cmd.Env = []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + os.Getenv("PATH"), "KUBECONFIG=/dev/null", "ASSAY_TELEMETRY=" + telemetry, "ASSAY_CONFIG_ECHO=summary", "ACTIVATION_TEST_TRACE=" + trace}
		start := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		code := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				code = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		return elapsed, code, string(out)
	}
	tools := []struct {
		name string
		args []string
	}{{"deskboard", []string{"prs"}}, {"deskdispatch", []string{"fixture-1", "--dry-run"}}, {"desktoken", []string{"worker"}}}
	for _, tool := range tools {
		normal := build(tool.name, false)
		observed := build(tool.name, true)
		for _, size := range []int{4, 256} {
			root := filepath.Join(scratch, fmt.Sprintf("%s-%d", tool.name, size))
			gitstub.Repo(t, root)
			for i := 0; i < size; i++ {
				snapshotFile(t, filepath.Join(root, fmt.Sprintf("d%04d", i), "leaf.txt"), "fixture\n")
			}
			writeManifest(t, root, "test/verbs", []string{"assay.desk.verbs"}, []string{"test.absent"}, nil)
			home := t.TempDir()
			trace := filepath.Join(scratch, fmt.Sprintf("%s-%d.trace", tool.name, size))
			_, code, out := run(observed, root, home, "0", trace, tool.args...)
			if code != 6 || !strings.Contains(out, "inactive") {
				t.Fatalf("%s direct refusal: %d %s", tool.name, code, out)
			}
			work, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(work), "enumeration\n") != 1 || strings.Count(string(work), "parse\n") != 1 || strings.Count(string(work), "reference-directory\n") != size+3 || strings.Count(string(work), "validation-directory\n") != 0 || strings.Count(string(work), "validation-read\n") != 0 {
				t.Fatalf("%s unaccounted startup work: %s", tool.name, work)
			}
			t.Logf("work %s dirs=%d reference=1 parse=1 validation-dirs=%d validation-read=0 reuse=0", tool.name, size+3, strings.Count(string(work), "validation-directory\n"))
			helpTrace := trace + "-help"
			_, helpCode, _ := run(observed, root, home, "0", helpTrace, "--help")
			if tool.name != "desktoken" {
				if helpCode != 0 {
					t.Fatalf("%s help exit %d", tool.name, helpCode)
				}
				if data, _ := os.ReadFile(helpTrace); len(data) != 0 {
					t.Fatalf("%s help scanned before execution: %s", tool.name, data)
				}
			} else if helpCode != 6 {
				t.Fatal("token pre-parser activation ownership changed")
			}
			// Time the same inactive execution through real startup, without observer I/O.
			for _, telemetry := range []string{"0", "1"} {
				for _, state := range []string{"fresh-home", "reused-home"} {
					var samples []time.Duration
					for i := 0; i < 6; i++ {
						sampleHome := home
						if state == "fresh-home" {
							sampleHome = t.TempDir()
						}
						d, c, out := run(normal, root, sampleHome, telemetry, "", tool.args...)
						if c != 6 || !strings.Contains(out, "inactive") {
							t.Fatalf("%s inactive execution: %d %s", tool.name, c, out)
						}
						samples = append(samples, d)
					}
					sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
					t.Logf("elapsed %s dirs=%d manifests=1 state=%s telemetry=%s n=6 p50=%s p95=%s", tool.name, size+3, state, telemetry, samples[2], samples[5])
				}
			}
		}
	}
	t.Logf("environment go=%s os=%s arch=%s; external elapsed, normal builds; OS caches uncontrolled; fresh process cannot reuse in-process slot; inactive synthetic workloads; telemetry switch overhead only (refusal may precede recorder), no successful forge/custody operation", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
