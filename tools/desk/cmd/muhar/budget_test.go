package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The fixture is a REAL suite subprocess. Its shared counter is independently
// OS-locked, so the assertion measures work rather than the scheduler's claims.
type budgetCount struct{ Active, Peak, Calls int }

func fixtureCount(dir string, delta int) budgetCount {
	f, err := os.OpenFile(filepath.Join(dir, "count.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	end := time.Now().Add(5 * time.Second)
	for deskkit.TryLockExclusive(f) != nil {
		if time.Now().After(end) {
			panic("fixture count lock timeout")
		}
		time.Sleep(time.Millisecond)
	}
	defer deskkit.UnlockFile(f)
	p := filepath.Join(dir, "count.json")
	var c budgetCount
	b, _ := os.ReadFile(p)
	_ = json.Unmarshal(b, &c)
	c.Active += delta
	if delta > 0 {
		c.Calls++
	}
	if c.Active > c.Peak {
		c.Peak = c.Active
	}
	b, _ = json.Marshal(c)
	if err := os.WriteFile(p, b, 0600); err != nil {
		panic(err)
	}
	return c
}

func TestBudgetWork(t *testing.T) {
	dir := os.Getenv("MUHAR_FIXTURE")
	if dir == "" {
		return
	}
	sentinel, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("suite-%d.lock", os.Getpid())), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer sentinel.Close()
	if err := deskkit.TryLockExclusive(sentinel); err != nil {
		t.Fatal(err)
	}
	defer deskkit.UnlockFile(sentinel)
	fixtureCount(dir, 1)
	time.Sleep(45 * time.Millisecond)
	fixtureCount(dir, -1)
	if os.Getenv("MUHAR_BUILD") != "" {
		os.Exit(0)
	}
	b, err := os.ReadFile("value.txt")
	if err != nil {
		os.Exit(3)
	}
	if string(b) != "original" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestBudgetChild(t *testing.T) {
	if os.Getenv("MUHAR_CHILD") != "1" {
		return
	}
	os.Exit(run([]string{"-spec", os.Getenv("MUHAR_SPEC"), "-j", os.Getenv("MUHAR_JOBS")}, os.Stdout, os.Stderr))
}

func budgetFixture(t *testing.T, dir, jobs string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "value.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	command := shellQuote(os.Args[0]) + " -test.run=^TestBudgetWork$"
	m := Mutation{Name: "control", File: "value.txt", Old: "original", New: "mutant"}
	s := spec{Root: root, Test: command, Build: "MUHAR_BUILD=1 " + command, Control: m}
	for i := 0; i < 8; i++ {
		m.Name = fmt.Sprintf("mutation-%d", i)
		s.Mutations = append(s.Mutations, m)
	}
	raw, _ := json.Marshal(s)
	p := filepath.Join(root, "spec.json")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBudgetChild$")
	cmd.Env = append(envWithout(os.Environ(), "MUHAR_CHILD", "MUHAR_SPEC", "MUHAR_JOBS", "MUHAR_FIXTURE", "MUHAR_LEASE_DIR", "MUHAR_TOTAL_SLOTS"),
		"MUHAR_CHILD=1", "MUHAR_SPEC="+p, "MUHAR_JOBS="+jobs, "MUHAR_FIXTURE="+dir, "MUHAR_LEASE_DIR="+dir, "MUHAR_TOTAL_SLOTS=2")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	return cmd, &stdout, &stderr
}

// regression: F-muhar-budget
func TestBudgetBoundsTotal(t *testing.T) {
	dir := t.TempDir()
	a, ao, ae := budgetFixture(t, dir, "7")
	b, bo, be := budgetFixture(t, dir, "0")
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		_ = a.Process.Kill()
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- a.Wait() }()
	go func() { done <- b.Wait() }()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("child: %v; %s; %s", err, ae, be)
			}
		case <-timer.C:
			_ = a.Process.Kill()
			_ = b.Process.Kill()
			t.Fatal("over-demand processes did not both complete within 15s")
		}
	}
	c := fixtureCount(dir, 0)
	if c.Peak != 2 || c.Active != 0 || c.Calls != 38 {
		t.Fatalf("work counters=%+v; want peak=2 active=0 calls=38", c)
	}
	for _, out := range []*bytes.Buffer{ao, bo} {
		if !strings.Contains(out.String(), "Totals: 8 caught, 0 NOT CAUGHT, 0 could-not-mutate.") {
			t.Fatalf("complete spec lost: %s", out)
		}
	}
	if !strings.Contains(ae.String(), "clamping 7 workers to MUHAR_TOTAL_SLOTS=2") {
		t.Fatalf("clamp not echoed: %s", ae)
	}
	if !strings.Contains(ae.String()+be.String(), "queue-wait=") {
		t.Fatalf("no measured queue wait: %s %s", ae, be)
	}
	t.Logf("two over-demand processes completed within 15s; actual peak=%d commands=%d", c.Peak, c.Calls)
}

// regression: F-muhar-budget
func TestBudgetReleases(t *testing.T) {
	dir := t.TempDir()
	lookup := func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return "1", true
	}
	ctx, cancel := context.WithCancel(context.Background())
	b, err := loadBudget(ctx, lookup, nil)
	if err != nil {
		t.Fatal(err)
	}
	release, err := b.acquire()
	if err != nil {
		t.Fatal(err)
	}
	// A second independent process cannot take the same live slot.
	cmd := exec.Command(os.Args[0], "-test.run=^TestBudgetProbe$")
	cmd.Env = append(os.Environ(), "MUHAR_PROBE="+dir, "MUHAR_BUSY=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("busy probe: %v %s", err, out)
	}
	done := make(chan error, 1)
	go func() {
		r, e := b.acquire()
		if r != nil {
			r()
		}
		done <- e
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled admission succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not wake waiter")
	}
	release()
	release() // release is idempotent
	cmd = exec.Command(os.Args[0], "-test.run=^TestBudgetProbe$")
	cmd.Env = append(envWithout(os.Environ(), "MUHAR_BUSY"), "MUHAR_PROBE="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("release probe: %v %s", err, out)
	}
	if runtime.GOOS == "windows" {
		return
	} // Windows has no catchable console Signal API.
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		dir := t.TempDir()
		c, _, stderr := budgetFixture(t, dir, "4")
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for fixtureCount(dir, 0).Active == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if err := c.Process.Signal(signal); err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() { finished <- c.Wait() }()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			_ = c.Process.Kill()
			t.Fatal("signal did not finish")
		}
		if c.ProcessState.ExitCode() != 130 {
			t.Fatalf("interrupt status=%v stderr=%s", c.ProcessState, stderr)
		}
		probe := exec.Command(os.Args[0], "-test.run=^TestBudgetProbe$")
		probe.Env = append(os.Environ(), "MUHAR_PROBE="+dir, "MUHAR_PROBE_TOTAL=2")
		if out, err := probe.CombinedOutput(); err != nil {
			t.Fatalf("signal release: %v %s", err, out)
		}
		sentinels, err := filepath.Glob(filepath.Join(dir, "suite-*.lock"))
		if err != nil {
			t.Fatal(err)
		}
		if len(sentinels) == 0 {
			t.Fatal("no suite process was observed before signal")
		}
		for _, p := range sentinels {
			f, err := os.OpenFile(p, os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			err = deskkit.TryLockExclusive(f)
			if err == nil {
				_ = deskkit.UnlockFile(f)
			}
			_ = f.Close()
			if err != nil {
				t.Fatalf("suite child still running after %v: %v", signal, err)
			}
		}
	}
}

func TestBudgetProbe(t *testing.T) {
	dir := os.Getenv("MUHAR_PROBE")
	if dir == "" {
		return
	}
	total := 1
	if value := os.Getenv("MUHAR_PROBE_TOTAL"); value != "" {
		total, _ = strconv.Atoi(value)
	}
	for i := 0; i < total; i++ {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("slot-%d.lock", i)), os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		err = deskkit.TryLockExclusive(f)
		if (err != nil) != (os.Getenv("MUHAR_BUSY") == "1") {
			_ = f.Close()
			t.Fatalf("slot ownership=%v", err)
		}
		if err == nil {
			_ = deskkit.UnlockFile(f)
		}
		_ = f.Close()
	}
}

// regression: F-muhar-budget
func TestBudgetBadConfig(t *testing.T) {
	for _, jobs := range []int{1, 2, 7} {
		b := &hostBudget{total: 2}
		want := jobs
		if want > 2 {
			want = 2
		}
		if got := b.clamp(jobs); got != want {
			t.Errorf("clamp(%d)=%d want=%d", jobs, got, want)
		}
	}
	dir := t.TempDir()
	for _, values := range []map[string]string{
		{"MUHAR_LEASE_DIR": dir}, {"MUHAR_TOTAL_SLOTS": "1"}, {"MUHAR_LEASE_DIR": "", "MUHAR_TOTAL_SLOTS": "1"},
		{"MUHAR_LEASE_DIR": dir, "MUHAR_TOTAL_SLOTS": "0"}, {"MUHAR_LEASE_DIR": dir, "MUHAR_TOTAL_SLOTS": "-2"},
		{"MUHAR_LEASE_DIR": dir, "MUHAR_TOTAL_SLOTS": "x"}, {"MUHAR_LEASE_DIR": filepath.Join(dir, "missing"), "MUHAR_TOTAL_SLOTS": "1"},
		{"MUHAR_LEASE_DIR": "relative", "MUHAR_TOTAL_SLOTS": "1"},
	} {
		lookup := func(k string) (string, bool) { v, ok := values[k]; return v, ok }
		if _, err := loadBudget(context.Background(), lookup, nil); err == nil {
			t.Errorf("accepted bad configuration: %v", values)
		}
	}
	if b, err := loadBudget(context.Background(), func(string) (string, bool) { return "", false }, nil); b != nil || err != nil {
		t.Fatalf("unset: %v %v", b, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "slot-0.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := loadBudget(context.Background(), func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return strconv.Itoa(1), true
	}, nil)
	if err == nil {
		t.Fatal("unreadable slot silently accepted")
	}
}

// regression: F-muhar-budget
func TestBudgetSelector(t *testing.T) {
	dir := t.TempDir()
	budget, err := loadBudget(context.Background(), func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return "2", true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs := &coverSelector{budget: budget}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "value.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	// This is the actual command path used by go-list, coverage builds, test
	// discovery and concurrent single-test coverage, with eight competing calls.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestBudgetWork$")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "MUHAR_FIXTURE="+dir)
			_, err := cs.output(cmd, true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	c := fixtureCount(dir, 0)
	if c.Peak != 2 || c.Calls != 8 {
		t.Fatalf("selector work=%+v", c)
	}
	// A proof command that cannot run breaks Prepare; it cannot be mislabeled
	// as a red pristine set and silently fall back to the full command.
	m := Mutation{Name: "proof", File: "value.go", Old: "return 1", New: "return 2"}
	if err := os.WriteFile(filepath.Join(root, m.File), []byte("package p\nfunc F() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cs = &coverSelector{budget: budget, log: &bytes.Buffer{}, jobs: 2, muts: []Mutation{m}, cmd: goTestCmd{pkgs: []string{"."}},
		pkgs: []testPkg{{importPath: "p"}}, tests: []testRef{{pkg: 0, name: "TestF"}}, blocks: map[string][]cblock{"value.go": {{l1: 2, l2: 2, tests: []int{0}}}},
		timedTest: func(string, []string, time.Duration) CommandResult {
			return CommandResult{Err: errors.New("proof resource failure")}
		}}
	if err := cs.proveSets(root); err == nil {
		t.Fatal("proof resource failure became a definitive red")
	}
}

// regression: F-muhar-budget
func TestBudgetDrained(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	b, err := loadBudget(context.Background(), func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return "1", true
	}, &log)
	if err != nil {
		t.Fatal(err)
	}
	release, err := b.acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var pending atomic.Bool
	pending.Store(true)
	done := make(chan error, 1)
	go func() {
		r, err := b.acquireWhile(pending.Load)
		if r != nil {
			r()
		}
		done <- err
	}()
	// Still holding the only slot, drain the queue. The waiter must exit
	// without waiting for this lease's release or reserving another mutation.
	time.Sleep(35 * time.Millisecond)
	pending.Store(false)
	select {
	case err := <-done:
		if !errors.Is(err, errNoBudgetWork) {
			t.Fatalf("drained=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("drained worker waited for an unnecessary lease")
	}
}

// regression: F-muhar-budget
func TestBudgetError(t *testing.T) {
	dir := t.TempDir()
	b, err := loadBudget(context.Background(), func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return "1", true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Removing a slot is an operational error, never permission to switch pools.
	if err := os.Remove(filepath.Join(dir, "slot-0.lock")); err != nil {
		t.Fatal(err)
	}
	h := &Harness{Budget: b, RunTest: func(string, time.Duration) CommandResult { t.Fatal("unadmitted baseline ran"); return CommandResult{} }}
	if rep := h.Run(); !rep.Broken || len(rep.Results) != 0 {
		t.Fatalf("admission error produced verdicts: %+v", rep)
	}
	b, err = loadBudget(context.Background(), func(k string) (string, bool) {
		if k == "MUHAR_LEASE_DIR" {
			return dir, true
		}
		return "1", true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h = &Harness{Budget: b, RunTest: func(string, time.Duration) CommandResult { return CommandResult{Err: errors.New("resource failure")} }}
	if rep := h.Run(); !rep.Broken {
		t.Fatal("resource failure did not break baseline")
	}
	r, err := b.acquire()
	if err != nil {
		t.Fatal(err)
	}
	r()
}
