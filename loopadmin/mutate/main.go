// Command mutate is the module's mutation gate. For each entry in a mutation
// map it copies the module to a scratch directory, applies the single textual
// replacement (the old text must match exactly once), and runs the named test
// there. The test must FAIL: a mutation that leaves it green means the test
// does not guard the behaviour it names.
//
//	cd loopadmin && go run ./mutate -map testdata/mutations.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

type mutation struct {
	Test string `json:"test"`
	Why  string `json:"why"`
	File string `json:"file"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

func main() {
	mapPath := flag.String("map", "testdata/mutations.json", "mutation map")
	only := flag.String("test", "", "run only the mutation for this test name")
	flag.Parse()
	if err := run(*mapPath, *only); err != nil {
		fmt.Fprintln(os.Stderr, "mutate:", err)
		os.Exit(1)
	}
}

func run(mapPath, only string) error {
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		return err
	}
	var ms []mutation
	if err := json.Unmarshal(raw, &ms); err != nil {
		return fmt.Errorf("%s: %w", mapPath, err)
	}
	if len(ms) == 0 {
		return fmt.Errorf("%s: no mutations", mapPath)
	}
	// Quarantine the Go build cache for this run. Every mutant is a full
	// scratch copy of the module, so each `go test` injects never-reused
	// entries into whatever GOCACHE it inherits; in a long-lived cell that
	// cache grows without bound (Go's own 5-day trim horizon is a
	// compile-time constant, not a knob). One throwaway cache for the WHOLE
	// run — mutants share most build artifacts, so per-run keeps the
	// intra-run speedup — removed when run() returns, before main's os.Exit.
	// GOMODCACHE is left alone: module downloads are immutable and
	// shared-safe.
	gocache, err := os.MkdirTemp("", "assay-mutant-gocache-*")
	if err != nil {
		return fmt.Errorf("create per-run GOCACHE: %w", err)
	}
	defer func() { _ = os.RemoveAll(gocache) }()
	// An interrupt mid-run must not strand the cache either.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		_ = os.RemoveAll(gocache)
		os.Exit(130)
	}()
	bad := 0
	ran := 0
	for _, m := range ms {
		if only != "" && m.Test != only {
			continue
		}
		ran++
		killed, out, err := apply(m, gocache)
		switch {
		case err != nil:
			fmt.Printf("ERROR    %s: %v\n", m.Test, err)
			bad++
		case killed:
			fmt.Printf("KILLED   %s  (%s)\n", m.Test, m.Why)
			if os.Getenv("MUTATE_VERBOSE") != "" {
				fmt.Print(out)
			}
		default:
			fmt.Printf("SURVIVED %s  (%s)\n%s", m.Test, m.Why, out)
			bad++
		}
	}
	if ran == 0 {
		return fmt.Errorf("no mutation matched -test %q", only)
	}
	if bad > 0 {
		return fmt.Errorf("%d mutation(s) not killed", bad)
	}
	return nil
}

func apply(m mutation, gocache string) (killed bool, out string, err error) {
	dir, err := os.MkdirTemp("", "loopadmin-mutate-")
	if err != nil {
		return false, "", err
	}
	defer os.RemoveAll(dir)
	if err := copyTree(".", dir); err != nil {
		return false, "", err
	}
	target, err := confine(dir, m.File)
	if err != nil {
		return false, "", err
	}
	src, err := os.ReadFile(target)
	if err != nil {
		return false, "", err
	}
	if n := strings.Count(string(src), m.Old); n != 1 {
		return false, "", fmt.Errorf("%s: old text matches %d times, want exactly 1", m.File, n)
	}
	mutated := strings.Replace(string(src), m.Old, m.New, 1)
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		return false, "", err
	}
	cmd := exec.Command("go", "test", "-count=1", "-timeout=120s", "-run", "^"+m.Test+"$", "./...")
	cmd.Dir = dir
	cmd.Env = append(envWithGOCACHE(os.Environ(), gocache), "GOWORK=off")
	b, runErr := cmd.CombinedOutput()
	out = string(b)
	if runErr == nil {
		return false, out, nil
	}
	// A compile failure is not a kill: the mutation must build and fail the test.
	if strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]") {
		return false, out, fmt.Errorf("mutation does not build:\n%s", out)
	}
	if !strings.Contains(out, "--- FAIL: "+m.Test) && !strings.Contains(out, "FAIL\t") {
		return false, out, fmt.Errorf("go test failed without a test failure:\n%s", out)
	}
	return true, out, nil
}

// envWithGOCACHE returns env with GOCACHE pinned to dir, overriding any
// inherited value: the quarantine only holds if the mutant's build artifacts
// land in the throwaway per-run cache even when the environment already names
// a GOCACHE. GOMODCACHE is never touched — module downloads are immutable and
// shared-safe, and re-downloading them would cost network per run.
func envWithGOCACHE(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "GOCACHE=") {
			out = append(out, e)
		}
	}
	return append(out, "GOCACHE="+dir)
}

// confine resolves a map entry's file inside the scratch copy. An absolute
// path, or one that climbs out of the copy, is refused: a mutation only ever
// edits the module under test.
func confine(dir, file string) (string, error) {
	rel := filepath.Clean(filepath.FromSlash(file))
	if file == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" ||
		rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file %q is not a path inside the module", file)
	}
	return filepath.Join(dir, rel), nil
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}
