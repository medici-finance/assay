package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// snapshotResult includes the complete observable mode result and the coverage
// adapter's JSON. Resetting the two existing memos selects the fresh reference;
// it does not substitute a second parser or evaluator.
type snapshotResult struct {
	Out, Err     string
	Code         int
	Files        map[string]string
	Coverage     string
	CoverageCode int
}

func snapshotRun(t *testing.T, root, mode string) snapshotResult {
	t.Helper()
	r := snapshotResult{Files: map[string]string{}}
	r.Out, r.Err = capturePerfOutput(t, func() {
		switch mode {
		case "lint":
			r.Code = run(root, "lint", nil, nil, "")
		case "regen":
			r.Code = run(root, "write", nil, nil, "")
		case "gate-scores":
			r.Code = runGateScores(root)
		case "next-up":
			r.Code = runNextUp(root)
		default:
			t.Fatalf("unknown mode %s", mode)
		}
	})
	for _, p := range []string{"STATUS.md", "docs/streams/FINDINGS.md", "docs/streams/INTAKE.md"} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			r.Files[p] = string(b)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	// Include held/unavailable results rather than making coverage success a
	// prerequisite for measuring the operational mode.
	var ce string
	r.Coverage, ce = capturePerfOutput(t, func() { r.CoverageCode = runCoverage(root, true) })
	r.Coverage += ce
	return r
}

func resetPerfInputs() { resetReadMemo(); resetBriefParseMemo() }

func snapshotSetup(t *testing.T) string {
	t.Helper()
	// Keep the invoking toolchain's prepared caches when isolating HOME. In CI,
	// their defaults are derived from HOME; changing it must not force downloads.
	cacheCmd := exec.Command("go", "env", "GOCACHE", "GOMODCACHE")
	cacheOut, cacheErr := goquiet.Cmd(cacheCmd).CombinedOutput()
	caches := strings.Split(strings.TrimSpace(string(cacheOut)), "\n")
	if cacheErr != nil || len(caches) != 2 {
		t.Fatalf("resolve prepared Go caches: %v\n%s", cacheErr, cacheOut)
	}
	t.Setenv("GOCACHE", strings.TrimSpace(caches[0]))
	t.Setenv("GOMODCACHE", strings.TrimSpace(caches[1]))
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASSAY_TELEMETRY", "")
	t.Setenv("DESK_LOOP", "worker-desk")
	// These tests use the normal offline claim source, never a deskread child.
	prevReader := forgeReaderForRun
	forgeReaderForRun = newOfflineReader()
	t.Cleanup(func() { forgeReaderForRun = prevReader; resetPerfInputs() })
	prevNow := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = prevNow })
	return home
}

func perfInputFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/perfworkload/brief.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"01", "02"} {
		text := strings.NewReplacer("{{stream}}", "alpha", "{{num}}", n).Replace(string(raw))
		writeTestFile(t, filepath.Join(root, "docs/streams/alpha/brief-"+n+".md"), text)
	}
	return root
}

// regression: statusgen/26
func TestInvocationSnapshotReuseParity(t *testing.T) {
	snapshotSetup(t)
	// Move only the read-memo clock to make the fast path observable without a
	// sleep. The mutations below still change ctime/content; clocks grant no reuse.
	prev := readMemoNow
	readMemoNow = func() time.Time { return time.Now().Add(time.Hour) }
	t.Cleanup(func() { readMemoNow = prev })
	for _, mode := range []string{"lint", "regen", "gate-scores", "next-up"} {
		t.Run(mode, func(t *testing.T) {
			root := perfInputFixture(t)
			localGit := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
				cmd.Env = append(perfTestEnv(os.Getenv("HOME"), filepath.Dir(root)), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
				out, err := goquiet.Cmd(cmd).CombinedOutput()
				if err != nil {
					t.Fatalf("local git %v: %v\n%s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			localGit("init", "-q", "-b", "main")
			localGit("add", "docs")
			localGit("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture")
			head := localGit("rev-parse", "HEAD")
			resetPerfInputs()
			first := snapshotRun(t, root, mode)
			parses := briefParseCount.Load()
			warm := snapshotRun(t, root, mode)
			if !reflect.DeepEqual(first, warm) {
				t.Fatal("same input changed output/exit/coverage")
			}
			if parses == 0 || briefParseCount.Load() != parses {
				t.Fatalf("same snapshot did not share parsing: %d -> %d", parses, briefParseCount.Load())
			}
			dir := filepath.Join(root, "docs/streams/alpha")
			file := filepath.Join(dir, "brief-02.md")
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// Dirty content changes under an unchanged repository identity. Same size
			// and restored mtime rule out cheap metadata substitutes for the content key.
			changes := []struct {
				name  string
				apply func()
			}{
				{"changed-content", func() {
					st, err := os.Stat(file)
					if err != nil {
						t.Fatal(err)
					}
					writeTestFile(t, file, strings.Replace(string(raw), "gate: model", "gate: human", 1))
					if err := os.Chtimes(file, st.ModTime(), st.ModTime()); err != nil {
						t.Fatal(err)
					}
				}},
				{"new-unlisted-consumer", func() {
					writeTestFile(t, filepath.Join(dir, "brief-03.md"), strings.NewReplacer("alpha/02", "alpha/03", "Synthetic workload brief 02", "Synthetic workload brief 03").Replace(string(raw)))
				}},
				{"deleted-consumer", func() {
					if err := os.Remove(file); err != nil {
						t.Fatal(err)
					}
				}},
				{"new-stream", func() {
					other := filepath.Join(root, "docs/streams/beta")
					if err := os.MkdirAll(other, 0700); err != nil {
						t.Fatal(err)
					}
					b, err := os.ReadFile(filepath.Join(dir, "README.md"))
					if err != nil {
						t.Fatal(err)
					}
					writeTestFile(t, filepath.Join(other, "README.md"), strings.ReplaceAll(string(b), "alpha", "beta"))
				}},
			}
			for _, c := range changes {
				t.Run(c.name, func(t *testing.T) {
					c.apply()
					if localGit("rev-parse", "HEAD") != head {
						t.Fatal("fixture HEAD moved")
					}
					got := snapshotRun(t, root, mode)
					resetPerfInputs()
					want := snapshotRun(t, root, mode)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s: memoized output/exit/coverage differs from fresh reference", c.name)
					}
				})
			}
		})
	}
	// Exercise the production memo owner below its callers too: consumers need
	// not be present in a README or in a prior set of parsed paths to be discovered.
	t.Run("membership", func(t *testing.T) {
		root := perfInputFixture(t)
		resetPerfInputs()
		s := &Stream{Dir: filepath.Join(root, "docs/streams/alpha")}
		before := briefFilePaths(s)
		writeTestFile(t, filepath.Join(s.Dir, "brief-99.md"), "# Untracked consumer\n")
		if got := briefFilePaths(s); len(got) != len(before)+1 {
			t.Fatalf("new untracked consumer omitted: %v", got)
		}
	})
	if os.Getenv("ASSAY_SNAPSHOT_MUTANT") == "" {
		snapshotMutant(t, "head-only", "brieffile.go", `return fmt.Sprintf("%s|%x", path, sum)`, `return fmt.Sprintf("%s|%x", path, sum[:0])`)
		t.Run("stamp-mutation", func(t *testing.T) {
			if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
				t.Skip("no ctime-based fast path on this platform")
			}
			snapshotMutant(t, "prior-readset", "readmemo.go", `!ok || !e.stamp.equal(now) || !e.stamp.settledBefore(e.began)`, `!ok`)
		})
	}
}

// snapshotMutant compiles an overlay of the real lower owner and demands this
// exact behavior test fail. No checkout is changed, and build errors are not kills.
func snapshotMutant(t *testing.T, label, file, old, replacement string, target ...string) {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), old) != 1 {
		t.Fatalf("mutation %s lost its unique anchor", label)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, file)
	if err := os.WriteFile(src, []byte(strings.Replace(string(raw), old, replacement, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{actual: src}})
	if err != nil {
		t.Fatal(err)
	}
	ov := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(ov, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	testName := "TestInvocationSnapshotReuseParity"
	if len(target) > 0 {
		testName = target[0]
	}
	cmd := exec.Command("go", "test", "-count=1", "-timeout=90s", "-overlay", ov, "-run=^"+testName+"$", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "ASSAY_SNAPSHOT_MUTANT="+label, "GOFLAGS=-overlay="+ov, "ASSAY_PERF_WORKLOAD_SAMPLES=2")
	out, runErr := cmd.CombinedOutput()
	text := string(out)
	if runErr == nil || !strings.Contains(text, "--- FAIL: "+testName) || strings.Contains(text, "[build failed]") || strings.Contains(text, "[setup failed]") {
		t.Fatalf("mutation %s did not fail its behavior test: %v\n%s", label, runErr, text)
	}
	t.Logf("KILLED %s: production owner fails %s", label, testName)
}

func perfMemoCounts() (files, dirs int) {
	readMemoMu.Lock()
	defer readMemoMu.Unlock()
	return readMemoDiskFiles, readMemoDiskDirs
}

// Keep the count split honest: these are owner-local disk attempts, including
// errors, not a claim to count every file read in statusgen.
// regression: statusgen/26
func TestPerfReadCountCoverage(t *testing.T) {
	resetReadMemo()
	t.Cleanup(resetReadMemo)
	dir := t.TempDir()
	if _, err := readFileMemo(filepath.Join(dir, "absent")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := readDirMemo(dir); err != nil {
		t.Fatal(err)
	}
	f, d := perfMemoCounts()
	if f != 1 || d != 1 || f+d != readMemoDiskReadCount() {
		t.Fatalf("owner reads: file=%d dir=%d total=%d", f, d, readMemoDiskReadCount())
	}
	if os.Getenv("ASSAY_SNAPSHOT_MUTANT") == "" {
		snapshotMutant(t, "file-count", "readmemo.go", "readMemoDiskFiles++", "readMemoDiskFiles += 0", "TestPerfReadCountCoverage")
		snapshotMutant(t, "dir-count", "readmemo.go", "readMemoDiskDirs++", "readMemoDiskDirs += 0", "TestPerfReadCountCoverage")
	}
}
