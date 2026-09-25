package main

// arm_test.go — scanloop arms the `deskmonitor inbound` VERB (example-stream/11), not a script
// through a fixed interpreter path, and the state dir it hands the verb is the one its arming read
// reads back.
//
// The poller here is a real process launched by the real exec path: a copy of this test binary
// named deskmonitor on PATH, which — started with fakeDeskmonitorEnv set — records the argv it was
// given and seeds one state file per repo it was asked to poll, the way a first real poll does.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	fakeDeskmonitorEnv  = "SCANLOOP_TEST_FAKE_DESKMONITOR"
	fakeDeskmonitorArgv = "SCANLOOP_TEST_FAKE_DESKMONITOR_ARGV"
)

// fakeDeskmonitorMain is the fake poller: argv[0] basename and every argument, one per line, to the
// argv file; one "<owner>__<name>.state" per repo argument into INBOUND_MONITOR_STATE_DIR; then the
// seeding cycle's own line.
func fakeDeskmonitorMain() int {
	var b strings.Builder
	b.WriteString(strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") + "\n")
	for _, a := range os.Args[1:] {
		b.WriteString(a + "\n")
	}
	if err := os.WriteFile(os.Getenv(fakeDeskmonitorArgv), []byte(b.String()), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := os.Getenv(EnvMonitorStateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	n := 0
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == monitorTokenFileFlag:
			i++
		case strings.Contains(args[i], "/"):
			if err := os.WriteFile(filepath.Join(dir, stateFileName(args[i])), []byte(args[i]+"#1 2026-09-01T00:00:00Z\n"), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			n++
		}
	}
	fmt.Printf("MONITOR-ARMED: %d\n", n)
	return 0
}

// installFakeDeskmonitor puts a copy of this test binary on PATH as deskmonitor and returns the
// file its argv will be recorded to.
func installFakeDeskmonitor(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	name := "deskmonitor"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	src, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(filepath.Join(bin, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	argv := filepath.Join(t.TempDir(), "argv")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeDeskmonitorEnv, "1")
	t.Setenv(fakeDeskmonitorArgv, argv)
	t.Setenv(EnvMonitorScript, "")
	return argv
}

func readArgv(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the poller was never launched (no argv recorded): %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// TestScanloopArmsDeskmonitor — the flow row. With no --monitor and no ASSAY_INBOUND_MONITOR,
// ResolvePoller picks the verb from PATH, execMonitor launches it with argv[0] = deskmonitor and
// argv[1] = inbound (then the token files and the scope), the state dir scanloop passes is the one
// the verb seeds, and ReadMonitorState reads that seeding back as arming evidence. The second half
// runs the same arming through `scanloop run` itself.
func TestScanloopArmsDeskmonitor(t *testing.T) {
	argvFile := installFakeDeskmonitor(t)
	scope := []string{"example-org/tracker", "medici-finance/assay"}

	poller, err := ResolvePoller(t.TempDir(), "")
	if err != nil {
		t.Fatalf("ResolvePoller with the verb on PATH: %v", err)
	}
	if poller.Parity() || !strings.HasPrefix(filepath.Base(poller.Path), "deskmonitor") {
		t.Fatalf("default mode must arm the deskmonitor verb, got %+v", poller)
	}

	stateDir := filepath.Join(t.TempDir(), "state")
	rep, err := RunMonitor(poller, stateDir, scope, map[string]string{"example-org": "/cache/example-token"}, nil)
	if err != nil {
		t.Fatalf("RunMonitor through the real exec path: %v", err)
	}
	argv := readArgv(t, argvFile)
	want := []string{"deskmonitor", "inbound", "--token-file", "example-org=/cache/example-token",
		"example-org/tracker", "medici-finance/assay"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("execMonitor argv = %q\nwant %q (argv[0] the verb, argv[1] its inbound poller)", argv, want)
	}
	if !rep.Armed || rep.ArmedTotal != 2 || rep.Blind() {
		t.Fatalf("the verb's seeding cycle was not parsed as armed: %+v", rep)
	}
	st, err := ReadMonitorState(stateDir, scope)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Armed || !reflect.DeepEqual(st.Seeded, scope) || len(st.Unseeded) != 0 {
		t.Fatalf("the state dir scanloop handed the verb does not read back as arming evidence: %+v", st)
	}

	// The same arming, end to end through `scanloop run`: no --monitor, so the verb is armed, and
	// the --state-dir given to run is the one the verb seeds.
	t.Run("through scanloop run", func(t *testing.T) {
		stubIdentity(t,
			func(string) (string, string, error) { return "", "", errors.New("no role in this test") },
			func(string, string) (string, string, error) {
				t.Fatal("a token was minted with no resolvable role")
				return "", "", nil
			})
		runState := filepath.Join(t.TempDir(), "run-state")
		var out strings.Builder
		err := cmdRun([]string{
			"--root", t.TempDir(),
			"--scan-target", "medici-finance/assay",
			"--worktree-base", t.TempDir(),
			"--state-dir", runState,
			"--now", "2026-08-24T12:00:00Z",
		}, &out)
		if err != nil {
			t.Fatalf("scanloop run: %v\n%s", err, out.String())
		}
		if !strings.Contains(out.String(), "poller: deskmonitor inbound — the desk verb at ") {
			t.Fatalf("run did not report arming the verb:\n%s", out.String())
		}
		if argv := readArgv(t, argvFile); argv[0] != "deskmonitor" || argv[1] != "inbound" {
			t.Fatalf("run launched %q, want deskmonitor inbound", argv)
		}
		st, err := ReadMonitorState(runState, deskkit.ScanRepos())
		if err != nil {
			t.Fatal(err)
		}
		if !st.Armed || len(st.Unseeded) != 0 {
			t.Fatalf("run's --state-dir was not the dir the verb seeded: %+v", st)
		}
	})
}

// TestResolvePoller_ModesAndRefusals — parity mode is the ONLY path that touches bash, and it
// resolves bash from PATH; neither mode falls back to the other.
func TestResolvePoller_ModesAndRefusals(t *testing.T) {
	script := filepath.Join(t.TempDir(), "inbound-monitor.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withLookPath := func(t *testing.T, found map[string]string) {
		old := lookPath
		lookPath = func(name string) (string, error) {
			if p, ok := found[name]; ok {
				return p, nil
			}
			return "", errors.New("executable file not found in $PATH")
		}
		t.Cleanup(func() { lookPath = old })
	}

	t.Run("default arms the verb and never looks for bash", func(t *testing.T) {
		t.Setenv(EnvMonitorScript, "")
		var asked []string
		old := lookPath
		lookPath = func(name string) (string, error) { asked = append(asked, name); return "/opt/bin/" + name, nil }
		t.Cleanup(func() { lookPath = old })
		p, err := ResolvePoller(t.TempDir(), "")
		if err != nil || p.Parity() || p.Path != "/opt/bin/deskmonitor" {
			t.Fatalf("got %+v, %v", p, err)
		}
		if !reflect.DeepEqual(asked, []string{"deskmonitor"}) {
			t.Fatalf("default mode looked up %q; it must resolve only the verb", asked)
		}
	})
	t.Run("default without the verb is unverifiable, never the script", func(t *testing.T) {
		t.Setenv(EnvMonitorScript, "")
		withLookPath(t, map[string]string{"bash": "/usr/local/bin/bash"})
		_, err := ResolvePoller(t.TempDir(), "")
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "deskmonitor") {
			t.Fatalf("want unverifiable naming deskmonitor, got %v", err)
		}
	})
	t.Run("--monitor is parity mode through bash from PATH", func(t *testing.T) {
		t.Setenv(EnvMonitorScript, "")
		withLookPath(t, map[string]string{"bash": "/usr/local/bin/bash"})
		p, err := ResolvePoller(t.TempDir(), script)
		if err != nil || !p.Parity() || p.Script != script || p.Path != "/usr/local/bin/bash" {
			t.Fatalf("got %+v, %v", p, err)
		}
	})
	t.Run("ASSAY_INBOUND_MONITOR is parity mode too", func(t *testing.T) {
		t.Setenv(EnvMonitorScript, script)
		withLookPath(t, map[string]string{"bash": "/usr/local/bin/bash"})
		p, err := ResolvePoller(t.TempDir(), "")
		if err != nil || !p.Parity() || p.Script != script {
			t.Fatalf("got %+v, %v", p, err)
		}
	})
	t.Run("parity mode without bash is unverifiable naming the mode", func(t *testing.T) {
		t.Setenv(EnvMonitorScript, "")
		withLookPath(t, map[string]string{"deskmonitor": "/opt/bin/deskmonitor"})
		_, err := ResolvePoller(t.TempDir(), script)
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "PARITY MODE") {
			t.Fatalf("want unverifiable naming PARITY MODE, got %v", err)
		}
	})
}
