//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reconcileCell is a house cell whose role directories hold what a crashed session leaves:
// an unfinished checkpoint and the lease's surviving owner record.
func reconcileCell(t *testing.T) *Cell {
	t.Helper()
	orphanGrace = time.Second
	root := t.TempDir()
	return &Cell{Name: "example", Dir: filepath.Join(root, "example"), Config: t.TempDir(), Kind: "house"}
}

// deadPID is the pid of a process that has exited and been reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// liveChild is a running process that is NOT marked as the cell's: it must never be signalled.
func liveChild(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func writeStale(t *testing.T, c *Cell, role string, pid int, start string, childPID int, childStart string) {
	t.Helper()
	dir := c.cadenceDir(role)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s := cellcadence.State{Schema: cellcadence.Schema, Mode: "interactive", Cell: c.Name, Role: role, PID: pid, Running: true, Sequence: 1, NextDue: now, Heartbeat: now, LastStart: now, Outcome: "running"}
	b, _ := json.Marshal(s)
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	h, _ := json.Marshal(deskkit.LockHolder{Schema: "deskkit-lock-holder-v1", PID: pid, Start: start, Command: "cellctl", Lock: "role lease", Cell: c.Name, Role: role, Acquired: now, ChildPID: childPID, ChildStart: childStart})
	if err := os.WriteFile(filepath.Join(dir, "owner.lock.holder"), h, 0o600); err != nil {
		t.Fatal(err)
	}
}

func init() {
	if os.Getenv("CELLCTL_RECONCILE_WINDOW") == "1" {
		// The "window" a sleeper opens: it only sleeps. It never watches a trigger, so it can
		// never spawn further processes of its own.
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	}
	if os.Getenv("CELLCTL_RECONCILE_SLEEPER") != "1" {
		return
	}
	// When its trigger file appears, the sleeper opens a "window" beneath itself: a child whose
	// readable environment carries no cell marker, as another cell's role child or any unrelated
	// program would. It reports the child's pid, or why it could not start one.
	if out := os.Getenv("CELLCTL_RECONCILE_CHILD_OUT"); out != "" {
		go func() {
			for {
				if _, err := os.Stat(out + ".trigger"); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			report := "error: "
			exe, err := os.Executable()
			if err == nil {
				child := exec.Command(exe)
				for _, kv := range os.Environ() {
					if !strings.HasPrefix(kv, "ASSAY_CELL") && !strings.HasPrefix(kv, "CELLCTL_RECONCILE_") {
						child.Env = append(child.Env, kv)
					}
				}
				child.Env = append(child.Env, "CELLCTL_RECONCILE_WINDOW=1")
				if err = child.Start(); err == nil {
					report = strconv.Itoa(child.Process.Pid)
				}
			}
			if err != nil {
				report += err.Error()
			}
			os.WriteFile(out, []byte(report), 0o600)
		}()
	}
	time.Sleep(5 * time.Minute)
	os.Exit(0)
}

// startOrphan leaves a marked process re-parented to pid 1, as a background loop of a closed
// window is. It skips when this host re-parents to a subreaper instead. The orphan is this test
// binary, not a system binary: macOS withholds the environment of its platform binaries from
// other processes, and a process whose marker cannot be read is never a candidate.
func startOrphan(t *testing.T, c *Cell, role string, extraEnv ...string) int {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", cockpitQuote(exe)+" </dev/null >/dev/null 2>&1 & echo $!")
	cmd.Env = c.stampCellMarker(append([]string{"PATH=/usr/bin:/bin", "CELLCTL_RECONCILE_SLEEPER=1"}, extraEnv...), role)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	for deadline := time.Now().Add(3 * time.Second); ; {
		ps, _ := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
		if strings.TrimSpace(string(ps)) == "1" {
			return pid
		}
		if time.Now().After(deadline) {
			t.Skipf("orphan fixture pid %d was re-parented to %q, not pid 1, on this host", pid, strings.TrimSpace(string(ps)))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// regression: #2880
func TestReconcileHandClosedWindows(t *testing.T) {
	c := reconcileCell(t)
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", deadPID(t), "2.000000")
	writeStale(t, c, "verify-desk", deadPID(t), "3.000000", 0, "")
	orphan := startOrphan(t, c, "worker-desk")
	roles := []string{"worker-desk", "verify-desk"}
	var out bytes.Buffer
	err := c.reconcile(roles, nil, reconcileIO{TTY: true, In: strings.NewReader("y\n"), Out: &out})
	if err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out.String())
	}
	if n := strings.Count(out.String(), "[y/N]"); n != 1 {
		t.Fatalf("want exactly one prompt, got %d:\n%s", n, out.String())
	}
	for _, want := range []string{"worker-desk class=dead", "verify-desk class=dead", "orphan pid=" + strconv.Itoa(orphan), "Clear stale state and stop 1 orphaned processes?"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q:\n%s", want, out.String())
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for alive(orphan) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if alive(orphan) {
		t.Fatalf("orphan %d still running", orphan)
	}
	for _, r := range roles {
		if _, err := cellcadence.Read(c.cadenceDir(r)); !errors.Is(err, cellcadence.ErrNoState) {
			t.Fatalf("%s checkpoint not recovered: %v", r, err)
		}
	}
	// The next start finds nothing: no prompt, no output, no refusal.
	out.Reset()
	if err := c.reconcile(roles, nil, reconcileIO{Out: &out}); err != nil || out.Len() != 0 {
		t.Fatalf("second start: %v %q", err, out.String())
	}
}

// regression: #2880
func TestReconcileLiveRoleChildRefuses(t *testing.T) {
	c := reconcileCell(t)
	child := liveChild(t)
	start, gone, err := deskkit.ProcessStart(child.Process.Pid)
	if err != nil || gone {
		t.Fatalf("child start: %v %v", gone, err)
	}
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", child.Process.Pid, start)
	var out bytes.Buffer
	err = c.reconcile([]string{"worker-desk"}, nil, reconcileIO{TTY: true, In: strings.NewReader("y\n"), Out: &out})
	if !errors.Is(err, errReconcileRefused) {
		t.Fatalf("live role child accepted: %v\n%s", err, out.String())
	}
	want := "role child pid " + strconv.Itoa(child.Process.Pid) + " is still running"
	if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "class=alive") || strings.Contains(out.String(), "[y/N]") {
		t.Fatalf("refusal must name the live child and never ask:\n%s", out.String())
	}
	if !alive(child.Process.Pid) {
		t.Fatal("live role child was signalled")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); err != nil {
		t.Fatalf("checkpoint cleared under a live child: %v", err)
	}
}

// regression: #2880
func TestReconcileReusedPidNeverKilled(t *testing.T) {
	c := reconcileCell(t)
	reused := liveChild(t)
	// Same pid, different start time: the recorded supervisor is gone and the pid was reused.
	writeStale(t, c, "worker-desk", reused.Process.Pid, "1.000000", 0, "")
	var out bytes.Buffer
	if err := c.reconcile([]string{"worker-desk"}, nil, reconcileIO{TTY: true, In: strings.NewReader("y\n"), Out: &out}); err != nil {
		t.Fatalf("reconcile: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "worker-desk class=dead") || !strings.Contains(out.String(), "pid reuse") {
		t.Fatalf("reused pid not classed dead:\n%s", out.String())
	}
	if !alive(reused.Process.Pid) {
		t.Fatal("the process now holding the reused pid was killed")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); !errors.Is(err, cellcadence.ErrNoState) {
		t.Fatalf("dead record not recovered: %v", err)
	}
}

// regression: #2880
func TestReconcileNonTTYRefusesWithCommands(t *testing.T) {
	c := reconcileCell(t)
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", 0, "")
	writeStale(t, c, "intake-desk", deadPID(t), "1.000000", 0, "")
	cfg := &deskCommsConfig{Socket: filepath.Join(t.TempDir(), "gw.sock")}
	if err := os.MkdirAll(c.commsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	b, _ := json.Marshal(cellcadence.State{Schema: cellcadence.Schema, Mode: "interactive", Cell: c.Name, Role: "comms", PID: deadPID(t), Running: true, Sequence: 1, NextDue: now, Heartbeat: now, LastStart: now, Outcome: "running"})
	if err := os.WriteFile(filepath.Join(c.commsDir(), "checkpoint.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rio := range []reconcileIO{{TTY: false}, {TTY: true, In: strings.NewReader("n\n")}, {DryRun: true, TTY: true, In: strings.NewReader("y\n")}} {
		var out bytes.Buffer
		rio.Out = &out
		err := c.reconcile([]string{"worker-desk", "intake-desk"}, cfg, rio)
		if rio.DryRun {
			if err != nil || !strings.Contains(out.String(), "dry run: nothing was changed") || strings.Contains(out.String(), "[y/N]") {
				t.Fatalf("dry run: %v\n%s", err, out.String())
			}
		} else {
			if !errors.Is(err, errReconcileRefused) {
				t.Fatalf("%+v: refused nothing: %v\n%s", rio, err, out.String())
			}
			if !rio.TTY && strings.Contains(out.String(), "[y/N]") {
				t.Fatalf("asked without a terminal:\n%s", out.String())
			}
			root := cockpitQuote(filepath.Dir(c.Dir))
			for _, want := range []string{
				"cellctl --cells-root " + root + " cadence 'example' recover worker-desk --confirm-stopped",
				"cellctl --cells-root " + root + " cadence 'example' recover intake-desk --confirm-stopped",
				"cellctl --cells-root " + root + " comms 'example' recover --confirm-stopped",
				"comms class=dead record=unfinished",
			} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("%+v: missing %q:\n%s", rio, want, out.String())
				}
			}
		}
		for _, r := range []string{"worker-desk", "intake-desk"} {
			if _, err := cellcadence.Read(c.cadenceDir(r)); err != nil {
				t.Fatalf("%+v: %s changed without confirmation: %v", rio, r, err)
			}
		}
		if _, err := c.commsState(); err != nil {
			t.Fatalf("%+v: comms changed without confirmation: %v", rio, err)
		}
	}
}

// regression: #2880
func TestReconcileLeavesOwnedRoleAlone(t *testing.T) {
	c := reconcileCell(t)
	l, err := cellcadence.AcquireRole(c.cadenceDir("worker-desk"), cellcadence.RoleLock{Cell: c.Name, Role: "worker-desk"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	writeStale(t, c, "worker-desk", os.Getpid(), "", 0, "")
	var out bytes.Buffer
	if err := c.reconcile([]string{"worker-desk"}, nil, reconcileIO{Out: &out}); err != nil || out.Len() != 0 {
		t.Fatalf("a running window is not stale state: %v %q", err, out.String())
	}
}

func TestStampCellMarkerReplacesInherited(t *testing.T) {
	c := &Cell{Name: "example", Dir: "/cells/example", Env: envWith(map[string]string{})}
	env := c.stampCellMarker([]string{"ASSAY_CELL=other", "ASSAY_CELL_ROLE=the-desk", "KEEP=1"}, "worker-desk")
	got := strings.Join(env, "\n")
	for _, want := range []string{"ASSAY_CELL=example", "ASSAY_CELL_DIR=/cells/example", "ASSAY_CELL_ROLE=worker-desk", "KEEP=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %q", want, got)
		}
	}
	if strings.Contains(got, "the-desk") || strings.Contains(got, "ASSAY_CELL=other") {
		t.Fatalf("inherited marker survived: %q", got)
	}
	comms := strings.Join(c.commsServiceEnv(&deskCommsConfig{}), "\n")
	if !strings.Contains(comms, "ASSAY_CELL_ROLE=comms") || !strings.Contains(comms, "ASSAY_CELL=example") {
		t.Fatalf("comms service is not marked: %q", comms)
	}
}

// regression: #2880 (security S1) — a cockpit process cellctl starts never inherits a cell
// marker, even when cellctl itself was started from inside a role window.
func TestCockpitLaunchCarriesNoMarker(t *testing.T) {
	f := newRepairFixture(t)
	f.writeCellEnv(t, "ROLES=the-desk\n")
	bin := t.TempDir()
	out := filepath.Join(bin, "env-seen")
	script := "#!/bin/sh\nenv | grep '^ASSAY_CELL' >> " + cockpitQuote(out) + "\necho \"$1\" >> " + cockpitQuote(out+".calls") + "\ncase \"$1\" in has-session) exit 1;; esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, []string{"PATH=" + bin + ":/usr/bin:/bin", "ASSAY_CELL=example", "ASSAY_CELL_DIR=" + f.cellDir, "ASSAY_CELL_ROLE=the-desk"},
		"up", "example", "--cockpit", "tmux", "--no-attach", "--lease-wait", "1s") // the-desk window: up must reach tmux

	// The stub opens a window but never starts the role, so up reports the role failed (it never
	// took its lease) — what this test pins is the launch environment, not the role.
	if r.code != 1 || !strings.Contains(r.stdout, "failed") {
		t.Fatalf("up: %+v", r)
	}
	if calls, _ := os.ReadFile(out + ".calls"); !strings.Contains(string(calls), "new-session") {
		t.Fatalf("fixture tmux never opened a session: %q", calls)
	}
	if seen, _ := os.ReadFile(out); len(seen) != 0 {
		t.Fatalf("cockpit launch inherited the cell marker:\n%s", seen)
	}
}

// regression: #2880 (correctness F1) — a FINISHED comms checkpoint with a failed outcome (the
// supervisor was signal-stopped and reaped its own child) is not stale custody: up launches comms
// exactly as before, even when the recorded pid has since been reused.
func TestReconcileFinishedCommsNotStale(t *testing.T) {
	c := reconcileCell(t)
	reused := liveChild(t)
	if err := os.MkdirAll(c.commsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	b, _ := json.Marshal(cellcadence.State{Schema: cellcadence.Schema, Mode: "interactive", Cell: c.Name, Role: "comms", PID: reused.Process.Pid, Sequence: 1, NextDue: now, Heartbeat: now, LastStart: now, LastFinish: now, Outcome: "could-not-check"})
	if err := os.WriteFile(filepath.Join(c.commsDir(), "checkpoint.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := c.reconcile(nil, &deskCommsConfig{Socket: filepath.Join(t.TempDir(), "gw.sock")}, reconcileIO{Out: &out}); err != nil || out.Len() != 0 {
		t.Fatalf("finished comms record blocked or printed: %v %q", err, out.String())
	}
}

// regression: #2880 (correctness A1) — a live process left in the dead role child's process
// group without a readable marker keeps the record alive: it is named, never signalled, and the
// checkpoint is not cleared over it.
func TestReconcileUnattributedGroupMember(t *testing.T) {
	c := reconcileCell(t)
	leader := exec.Command("/bin/sh", "-c", "/bin/sleep 300 </dev/null >/dev/null 2>&1 & echo $!")
	leader.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := leader.Output()
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(member, syscall.SIGKILL) })
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", leader.Process.Pid, "2.000000")
	var buf bytes.Buffer
	err = c.reconcile([]string{"worker-desk"}, nil, reconcileIO{TTY: true, In: strings.NewReader("y\n"), Out: &buf})
	if !errors.Is(err, errReconcileRefused) {
		t.Fatalf("cleared over a live group member: %v\n%s", err, buf.String())
	}
	want := "pid " + strconv.Itoa(member) + " ("
	if !strings.Contains(buf.String(), want) || !strings.Contains(buf.String(), "process group") || strings.Contains(buf.String(), "[y/N]") {
		t.Fatalf("group member not named, or asked anyway:\n%s", buf.String())
	}
	if !alive(member) {
		t.Fatal("unattributed group member was signalled")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); err != nil {
		t.Fatalf("checkpoint cleared: %v", err)
	}
}

// windowOpener is the prompt answer that, before answering, has a live window open beneath
// the listed orphan — the human-time gap between listing and signalling.
type windowOpener struct {
	t      *testing.T
	orphan int
	out    string
	done   bool
	answer []byte
}

func (w *windowOpener) Read(b []byte) (int, error) {
	if !w.done {
		w.done = true
		if err := os.WriteFile(w.out+".trigger", nil, 0o600); err != nil {
			w.t.Fatal(err)
		}
		for deadline := time.Now().Add(15 * time.Second); ; {
			if raw, err := os.ReadFile(w.out); err == nil && len(raw) > 0 {
				child, err := strconv.Atoi(string(raw))
				if err != nil {
					w.t.Fatalf("window beneath the orphan: %s", raw)
				}
				w.t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL) })
				// the child must be visible to the process listing before the answer lands
				time.Sleep(300 * time.Millisecond)
				break
			}
			if time.Now().After(deadline) {
				w.t.Fatal("window beneath the orphan never opened")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	if w.answer == nil {
		w.answer = []byte("y\n")
	}
	if len(w.answer) == 0 {
		return 0, io.EOF
	}
	n := copy(b, w.answer)
	w.answer = w.answer[n:]
	return n, nil
}

// regression: #2880 (security round 2, S2) — a window that opens beneath a listed orphan while
// the prompt waits shields it: the shield is recomputed immediately before the signal, the
// orphan is never signalled, and nothing is cleared.
func TestReconcileShieldReprovedAfterPrompt(t *testing.T) {
	c := reconcileCell(t)
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", 0, "")
	out := filepath.Join(t.TempDir(), "child")
	orphan := startOrphan(t, c, "worker-desk", "CELLCTL_RECONCILE_CHILD_OUT="+out)
	var buf bytes.Buffer
	err := c.reconcile([]string{"worker-desk"}, nil, reconcileIO{TTY: true, In: &windowOpener{t: t, orphan: orphan, out: out}, Out: &buf})
	if !errors.Is(err, errReconcileRefused) || !strings.Contains(buf.String(), "no longer a provable orphan") {
		t.Fatalf("signalled after a window opened beneath it: %v\n%s", err, buf.String())
	}
	time.Sleep(200 * time.Millisecond)
	if !alive(orphan) {
		t.Fatal("orphan hosting a newly opened window was signalled")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); err != nil {
		t.Fatalf("checkpoint cleared: %v", err)
	}
	// The fixture leaves exactly the one window beneath the orphan, and nothing beneath that.
	time.Sleep(300 * time.Millisecond)
	under := childrenOf(t, orphan)
	if len(under) != 1 {
		t.Fatalf("want exactly one window under the orphan, got %v", under)
	}
	if more := childrenOf(t, under[0]); len(more) != 0 {
		t.Fatalf("the window spawned further processes: %v", more)
	}
}

// childrenOf lists the live direct children of pid.
func childrenOf(t *testing.T, pid int) []int {
	t.Helper()
	raw, err := exec.Command("ps", "-axo", "pid=,ppid=,stat=").Output()
	if err != nil {
		t.Fatal(err)
	}
	var out []int
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || strings.HasPrefix(f[2], "Z") {
			continue
		}
		if parent, _ := strconv.Atoi(f[1]); parent == pid {
			child, _ := strconv.Atoi(f[0])
			out = append(out, child)
		}
	}
	return out
}

// regression: #2880 (correctness round 2, F2) — a marked process of the stale role that is
// alive but cannot be signalled (here shielded as part of the invoking process's tree; on a
// Linux subreaper host an orphan looks the same) keeps the record alive: it is named, never
// signalled, and the checkpoint is not cleared over it.
func TestReconcileShieldedMarkedKeepsRecordAlive(t *testing.T) {
	c := reconcileCell(t)
	writeStale(t, c, "worker-desk", deadPID(t), "1.000000", 0, "")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = c.stampCellMarker([]string{"PATH=/usr/bin:/bin", "CELLCTL_RECONCILE_SLEEPER=1"}, "worker-desk")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	time.Sleep(300 * time.Millisecond) // past exec, so its environment is the stamped one
	var buf bytes.Buffer
	err = c.reconcile([]string{"worker-desk"}, nil, reconcileIO{TTY: true, In: strings.NewReader("y\n"), Out: &buf})
	if !errors.Is(err, errReconcileRefused) {
		t.Fatalf("cleared while a marked process of the role still runs: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "marked process pid "+strconv.Itoa(cmd.Process.Pid)) || !strings.Contains(buf.String(), "worker-desk class=alive") {
		t.Fatalf("live marked process not named:\n%s", buf.String())
	}
	if !alive(cmd.Process.Pid) {
		t.Fatal("shielded marked process was signalled")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); err != nil {
		t.Fatalf("checkpoint cleared: %v", err)
	}
}
