package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

func TestCadenceSummaryRequiresCorrectCompletion(t *testing.T) {
	good := "tick role=worker-desk outcome=noop swept=0 acted=0 filed=0 duration=1"
	for _, tc := range []struct {
		name, output string
		ok           bool
	}{
		{"valid", "progress\n" + good + "\n", true},
		{"windows", good + "\r\n", true},
		{"missing", "model exited normally", false},
		{"wrong-role", strings.Replace(good, "worker-desk", "verify-desk", 1), false},
		{"blind-noop", strings.Replace(good, "swept=0", "swept=-", 1), false},
		{"after-summary", good + "\nmore work", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := cadenceSummary(tc.output, "worker-desk")
			if (err == nil) != tc.ok {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestCadenceStopAndRoleOwnership(t *testing.T) {
	c := &Cell{Name: "sample", Dir: t.TempDir(), Config: t.TempDir(), Kind: "house", Roles: []string{"worker-desk"}}
	d := c.cadenceDir("worker-desk")
	l, err := cellcadence.Acquire(d)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, err := cellcadence.Acquire(d); !errors.Is(err, cellcadence.ErrBusy) {
		t.Fatalf("second role owner got %v", err)
	}
	if err := c.cadenceGuard("worker-desk"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(d, "STOP"), filepath.Join(c.Config, "STOP"), filepath.Join(c.Config, "STOP.worker-desk"), filepath.Join(c.Config, "DISABLED")} {
		if err := os.WriteFile(p, []byte("stop"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := c.cadenceGuard("worker-desk"); err == nil {
			t.Fatalf("ignored %s", p)
		}
		os.Remove(p)
	}
}

func TestCadenceConfiguration(t *testing.T) {
	if resolveCadence("house", "", "") != nil {
		t.Fatal("legacy launch enabled cadence")
	}
	c := resolveCadence("house", "30m", "")
	if c.Interval != 30*time.Minute || c.Budget != 20*time.Minute {
		t.Fatalf("%+v", c)
	}
	for _, tc := range [][3]string{{"house", "-1s", ""}, {"house", "1ms", ""}, {"house", "1s", "1s"}, {"house", "", "20m"}, {"container", "30m", "20m"}, {"scrubbed", "30m", "20m"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted %v", tc)
				}
			}()
			resolveCadence(tc[0], tc[1], tc[2])
		}()
	}
}

func TestCadenceTailCaptureBounded(t *testing.T) {
	w := &tailWriter{limit: 4}
	w.Write([]byte("0123456"))
	w.Write([]byte("ab"))
	if w.String() != "56ab" {
		t.Fatal(w.String())
	}
}

func TestCadenceCockpitCommandCarriesOverrides(t *testing.T) {
	c := &Cell{Name: "sample"}
	for _, cockpit := range []string{"herdr", "orca"} {
		line := c.roleCmd("worker-desk", "", upOverrides{Model: "chosen", Harness: "cursor", Cockpit: cockpit, Cadence: "30m", TickBudget: "20m"})
		for _, want := range []string{"--model 'chosen'", "--harness 'cursor'", "--cockpit '" + cockpit + "'", "--cadence '30m'", "--tick-budget '20m'"} {
			if !strings.Contains(line, want) {
				t.Fatalf("missing %s: %s", want, line)
			}
		}
	}
}

func TestCadenceOffIgnoresSavedBudget(t *testing.T) {
	if resolveCadence("house", "off", "20m") != nil {
		t.Fatal("off did not disable saved cadence")
	}
}

func TestCockpitQuoteApostrophe(t *testing.T) {
	if got := cockpitQuote("work's folder"); got != "'work'\\''s folder'" {
		t.Fatalf("unsafe quote: %s", got)
	}
}

// A compiled harness stand-in proves the Go clock actually runs subsequent
// processes, rather than only testing the scheduler callback in isolation.
func init() {
	fixture := false
	for _, a := range os.Args[1:] {
		if a == "--cellctl-cadence-fixture" {
			fixture = true
		}
	}
	if !fixture {
		return
	}
	if os.Getenv("DESK_LOOP") != "worker-desk" || os.Getenv("DESK_ROOTS") != "example/repo=/a path" || os.Getenv("ASSAY_TICK") != "1" || os.Getenv("ASSAY_TICK_DEADLINE") != "1" || os.Getenv("ASSAY_COCKPIT") != "herdr" {
		os.Exit(23)
	}
	cwd, _ := os.Getwd()
	want, _ := filepath.EvalSymlinks(os.Getenv("FIXTURE_CWD"))
	if cwd != want {
		os.Exit(24)
	}
	final := "tick role=worker-desk outcome=noop swept=2 acted=0 filed=0 duration=1\n"
	if os.Getenv("FIXTURE_BAD_SUMMARY") == "1" {
		final = "completed without evidence\n"
	}
	if record := os.Getenv("FIXTURE_RECORD"); record != "" {
		f, err := os.OpenFile(record, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(25)
		}
		fmt.Fprintln(f, "pass")
		f.Close()
	}
	for i, a := range os.Args[1:] {
		if a == "--output-last-message" && i+2 < len(os.Args) {
			if os.WriteFile(os.Args[i+2], []byte(final), 0600) != nil {
				os.Exit(26)
			}
			fmt.Println("progress after final file; not completion evidence")
			os.Exit(0)
		}
	}
	fmt.Print(final)
	os.Exit(0)
}

func TestCadenceExecutesMultipleHarnessPasses(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, harness := range harnessValues {
		t.Run(harness, func(t *testing.T) {
			c := &Cell{Name: "sample", Dir: t.TempDir(), Config: t.TempDir(), Kind: "house"}
			wt := filepath.Join(t.TempDir(), "work space's 雪")
			if err := os.Mkdir(wt, 0700); err != nil {
				t.Fatal(err)
			}
			l, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			record := filepath.Join(t.TempDir(), "passes")
			env := []string{"DESK_LOOP=worker-desk", "DESK_ROOTS=example/repo=/a path", "ASSAY_COCKPIT=herdr", "FIXTURE_CWD=" + wt, "FIXTURE_RECORD=" + record}
			for _, k := range []string{"SYSTEMROOT", "SystemRoot"} {
				if v := os.Getenv(k); v != "" {
					env = append(env, k+"="+v)
				}
			}
			args, env, err := prepareTickLaunch(harness, []string{exe, "--cellctl-cadence-fixture", "--model", "chosen", "Invoke skill"}, env, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			stop := errors.New("two passes completed")
			cfg := cellcadence.Config{Cell: c.Name, Role: "worker-desk", Interval: 5 * time.Millisecond, Budget: 2 * time.Second, Heartbeat: 5 * time.Millisecond, Guard: func() error {
				s, e := cellcadence.Read(c.cadenceDir("worker-desk"))
				if e == nil && s.Sequence == 2 && !s.Running {
					return stop
				}
				return nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = l.Run(ctx, cfg, func(ctx context.Context) cellcadence.Result {
				return c.executeCadencePass(ctx, "worker-desk", harness, args, env, wt)
			})
			if !errors.Is(err, stop) {
				t.Fatalf("clock: %v", err)
			}
			state, err := cellcadence.Read(c.cadenceDir("worker-desk"))
			if err != nil || state.Outcome != "noop" {
				t.Fatalf("completion: %+v %v", state, err)
			}
			b, err := os.ReadFile(record)
			if err != nil || string(b) != "pass\npass\n" {
				t.Fatalf("model passes: %q %v", b, err)
			}
			result := c.executeCadencePass(ctx, "worker-desk", harness, args, append(env, "FIXTURE_BAD_SUMMARY=1"), wt)
			if result.Err == nil || result.Outcome != "could-not-check" {
				t.Fatalf("false completion: %+v", result)
			}
		})
	}
}

func TestCadenceColdCockpitFreezesClockAndKind(t *testing.T) {
	c := &Cell{Name: "sample", Dir: filepath.Join(t.TempDir(), "sample"), KindOverride: "house", Cadence: &cadenceOptions{Interval: 5 * time.Minute, Budget: 10 * time.Minute}}
	line := c.roleCmd("worker-desk", "", upOverrides{Harness: "cursor", Cockpit: "orca"})
	for _, want := range []string{"--cells-root " + cockpitQuote(filepath.Dir(c.Dir)), "--kind 'house'", "--cadence '5m0s'", "--tick-budget '10m0s'"} {
		if !strings.Contains(line, want) {
			t.Fatalf("missing %s: %s", want, line)
		}
	}
}
func TestGlobalCellRegistryKeepsHookDeadline(t *testing.T) {
	for _, args := range [][]string{{"model-policy", "hook"}, {"--cells-root", "/some/path", "model-policy", "hook"}} {
		got := commandArgs(args)
		if len(got) == 0 || got[0] != "model-policy" {
			t.Fatalf("hook deadline bypass: %v", args)
		}
	}
}
