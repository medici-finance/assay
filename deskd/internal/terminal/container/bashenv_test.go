package container

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// TestAttachPIDLineIgnoresBashEnv (the service contract security advisory
// A-sec-pidline-trust): the pid line Close later signals must be printed by
// deskd's own shell, never by a startup file the container's environment
// names. A non-interactive `bash -c` sources $BASH_ENV before it runs its
// script, so a container that sets BASH_ENV could print a different number
// first and steer Close's SIGHUP/SIGKILL to another in-container process.
//
// It runs the real attach argv under the host's bash (no container engine),
// with BASH_ENV pointing at a file that prints a decoy pid, and with the
// `bash` the preamble execs replaced by a stub that exits at once, so no
// login shell runs. The first line must be the shell's own pid.
func TestAttachPIDLineIgnoresBashEnv(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	dir := t.TempDir()
	decoy := filepath.Join(dir, "decoy.sh")
	if err := os.WriteFile(decoy, []byte("echo 31337\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubs := filepath.Join(dir, "bin")
	if err := os.Mkdir(stubs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stubs, "bash"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(realBash, attachCmd[1:]...)
	cmd.Env = []string{"PATH=" + stubs, "HOME=" + dir, "BASH_ENV=" + decoy}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	first, _ := bufio.NewReader(stdout).ReadString('\n')
	_ = cmd.Wait()

	want := strconv.Itoa(cmd.Process.Pid)
	if got := strings.TrimSpace(first); got != want {
		t.Fatalf("attach command's first line = %q, want the shell's own pid %s — a container-side $BASH_ENV printed the pid line", got, want)
	}
}

// TestDetachExecIgnoresBashEnv: Close's detach exec runs bash the same way,
// so a container-side $BASH_ENV runs nothing ahead of detachScript either.
// The fake engine records the argv; the posix-mode property itself is what
// TestAttachPIDLineIgnoresBashEnv proves against a real bash.
func TestDetachExecIgnoresBashEnv(t *testing.T) {
	log := detachFakeDocker(t, "live")
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}
	sess, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	readUntil(t, sess, "fake-shell-ready", 10*time.Second)
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	calls, _ := os.ReadFile(log)
	var detach string
	for _, line := range strings.Split(string(calls), "\n") {
		if strings.HasSuffix(line, " deskd-detach 4242") {
			detach = line
		}
	}
	if !strings.HasPrefix(detach, "exec c-review bash --posix -c ") {
		t.Fatalf("detach exec = %q, want it to start `exec c-review bash --posix -c ` (a non-interactive bash outside posix mode sources $BASH_ENV first)\ncalls:\n%s", detach, calls)
	}
}

// TestDetachScriptSignalsOnlyAnExecEnteredProcess (the service contract security
// advisory A-sec-pidline-trust, round 3): the pid line can still be forged by
// the container's own environment (an exported function, or xtrace on bash
// 5.x), so detachScript does not rely on the line alone. It signals only a
// process that `docker exec` entered: such a process has PPid 0 inside the
// container's pid namespace, and every process started inside the container
// has a parent there. Here the target is an ordinary host child of this test
// (PPid != 0), so the script must leave it running. On a host without /proc
// the script cannot confirm the parent and must also leave it running.
func TestDetachScriptSignalsOnlyAnExecEnteredProcess(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	victim := exec.Command("sleep", "30")
	if err := victim.Start(); err != nil {
		t.Fatal(err)
	}
	gone := make(chan struct{})
	go func() { _ = victim.Wait(); close(gone) }()
	defer func() { _ = victim.Process.Kill() }()

	argv := posixBash(detachScript, "deskd-detach", strconv.Itoa(victim.Process.Pid))
	if out, err := exec.Command(realBash, argv[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("detachScript: %v\n%s", err, out)
	}
	select {
	case <-gone:
		t.Fatalf("detachScript signalled pid %d, whose parent is inside the same pid namespace — only a docker-exec-entered process (PPid 0) may be signalled", victim.Process.Pid)
	case <-time.After(500 * time.Millisecond):
	}
}

// TestDetachScriptPPidGate pins both branches of detachScript's PPid-0 gate
// offline (review advisory A-cor-ppid-positive-offline): the real script, with
// only its `/proc/$1/status` path pointed at a status file this test writes,
// signals a target whose status reads `PPid: 0` (what `docker exec` gives the
// shell it enters) and leaves one with a parent alone. The third case exports
// a `read` function that would report PPid 0 for anything: the script unsets
// `read` and `kill` before it uses them (`unset` is a special builtin, which a
// function cannot shadow in posix mode), so that forgery signals nothing
// either (security advisory A-sec-pidline-trust). The fourth turns on xtrace
// with a PS4 that sets p=0 each time a traced line runs; the script's `set +x`
// ends the trace before the check. Bash 5.3 imports SHELLOPTS and PS4 for a
// non-root user; a bash that does not import them passes this case trivially.
func TestDetachScriptPPidGate(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	for _, c := range []struct {
		name, ppid string
		env        []string
		signalled  bool
	}{
		{"docker-exec-entered", "0", nil, true},
		{"has-a-parent", "7", nil, false},
		{"read-function-forgery", "7", []string{"BASH_FUNC_read%%=() { k=PPid:; v=0; }"}, false},
		{"xtrace-ps4-forgery", "7", []string{"SHELLOPTS=xtrace", "PS4=$((p=0))"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			victim := exec.Command("sleep", "30")
			if err := victim.Start(); err != nil {
				t.Fatal(err)
			}
			gone := make(chan struct{})
			go func() { _ = victim.Wait(); close(gone) }()
			defer func() { _ = victim.Process.Kill() }()

			root := t.TempDir()
			pid := strconv.Itoa(victim.Process.Pid)
			if err := os.MkdirAll(filepath.Join(root, pid), 0o755); err != nil {
				t.Fatal(err)
			}
			status := "Name:\tsleep\nState:\tS (sleeping)\nPid:\t" + pid + "\nPPid:\t" + c.ppid + "\n"
			if err := os.WriteFile(filepath.Join(root, pid, "status"), []byte(status), 0o644); err != nil {
				t.Fatal(err)
			}
			script := strings.Replace(detachScript, `"/proc/$1/status"`, `"`+root+`/$1/status"`, 1)
			if script == detachScript {
				t.Fatal("detachScript no longer reads /proc/$1/status — this test's path substitution missed")
			}
			argv := posixBash(script, "deskd-detach", pid)
			cmd := exec.Command(realBash, argv[1:]...)
			cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, c.env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("detachScript: %v\n%s", err, out)
			}
			select {
			case <-gone:
				if !c.signalled {
					t.Fatalf("detachScript signalled pid %s whose status reads PPid %s — only a docker-exec-entered process (PPid 0) may be signalled", pid, c.ppid)
				}
			case <-time.After(2 * killGrace):
				if c.signalled {
					t.Fatalf("detachScript left pid %s running although its status reads PPid 0 — detach would leak deskd's shell", pid)
				}
			}
		})
	}
}

// TestDetachScriptRechecksBeforeKill pins the second PPid-0 check, the one
// before SIGKILL (security review note on round 4): a target that ignores
// SIGHUP and whose status no longer reads PPid 0 by the end of the grace (its
// pid now names something else) must not be sent SIGKILL.
//
// Trap readiness is acknowledged before the target can be signalled. No
// timer decides the status-check ordering (review advisory A-cor-recheck-timing). The
// status "file" is a FIFO, so each check the script makes blocks until the
// test hands it one answer. The first answer is PPid 0. The target records
// the SIGHUP it then receives, which tells the test that the first check has
// read and closed the FIFO. Only then is the second answer, PPid 7, offered,
// and it reaches the next reader, which is the check before SIGKILL. A script
// with no second check never opens the FIFO again and goes on to SIGKILL.
func TestDetachScriptRechecksBeforeKill(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	root := t.TempDir()
	hupMark := filepath.Join(root, "hup")
	victim, gone := startHUPTarget(t, hupMark, false)

	pid := strconv.Itoa(victim.Process.Pid)
	statusPath := filepath.Join(root, pid, "status")
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	makeFIFO(t, statusPath)
	script := strings.Replace(detachScript, `"/proc/$1/status"`, `"`+root+`/$1/status"`, 1)
	argv := posixBash(script, "deskd-detach", pid)
	cmd := exec.Command(realBash, argv[1:]...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	defer func() { _ = cmd.Process.Kill() }()

	if !serveFIFO(t, statusPath, "PPid:\t0\n", exited) {
		t.Fatal("detachScript exited without reading the target's status")
	}
	for hup := false; !hup; {
		if b, _ := os.ReadFile(hupMark); len(b) > 0 {
			hup = true
			continue
		}
		select {
		case <-exited:
			t.Fatal("detachScript exited without sending SIGHUP to a target whose status reads PPid 0")
		case <-gone:
			t.Fatal("the target died before it recorded SIGHUP")
		case <-time.After(5 * time.Millisecond):
		}
	}
	serveFIFO(t, statusPath, "PPid:\t7\n", exited)
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("detachScript did not exit within 10s")
	}
	select {
	case <-gone:
		t.Fatalf("detachScript sent SIGKILL to pid %s after its status stopped reading PPid 0 — the check before SIGKILL is missing", pid)
	case <-time.After(300 * time.Millisecond):
	}
}

// TestDetachScriptExitsZeroUnderImportedErrexit (the service contract review
// round 9 advisory): endShell reads any non-zero exit of the detach exec as a
// failed detach, and the script claims to exit 0 on every path it reaches. A
// container environment that exports SHELLOPTS=errexit (bash 5.3 imports it
// into `bash --posix -c`) would break that claim at the final SIGKILL: when
// the target exits between the re-check and the kill, `kill -KILL` fails and
// errexit ends the script with rc 1. The script's `set +e` takes errexit back.
//
// No timer decides the race. The status "file" is a FIFO: the first answer
// (PPid 0) passes the first check, and the target records the SIGHUP it gets.
// The test then waits for the re-check to open the FIFO, kills and reaps the
// target while that check is blocked, and only then answers PPid 0, so the
// script's `kill -KILL` names a pid that is gone. On a bash that does not
// import SHELLOPTS the case passes trivially.
func TestDetachScriptExitsZeroUnderImportedErrexit(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	root := t.TempDir()
	hupMark := filepath.Join(root, "hup")
	target, gone := startHUPTarget(t, hupMark, false)

	pid := strconv.Itoa(target.Process.Pid)
	statusPath := filepath.Join(root, pid, "status")
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	makeFIFO(t, statusPath)
	script := strings.Replace(detachScript, `"/proc/$1/status"`, `"`+root+`/$1/status"`, 1)
	argv := posixBash(script, "deskd-detach", pid)
	cmd := exec.Command(realBash, argv[1:]...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "SHELLOPTS=errexit"}
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	var runErr error
	go func() { runErr = cmd.Wait(); close(exited) }()
	defer func() { _ = cmd.Process.Kill() }()

	if !serveFIFO(t, statusPath, "PPid:\t0\n", exited) {
		t.Fatal("detachScript exited without reading the target's status")
	}
	for hup := false; !hup; {
		if b, _ := os.ReadFile(hupMark); len(b) > 0 {
			hup = true
			continue
		}
		select {
		case <-exited:
			t.Fatalf("detachScript exited without sending SIGHUP to a target whose status reads PPid 0: %v\n%s", runErr, out.String())
		case <-gone:
			t.Fatal("the target died before it recorded SIGHUP")
		case <-time.After(5 * time.Millisecond):
		}
	}
	w := awaitFIFOReader(t, statusPath, exited)
	if w == nil {
		t.Fatalf("detachScript exited before its re-check: %v\n%s", runErr, out.String())
	}
	_ = target.Process.Kill()
	<-gone
	_, werr := w.WriteString("PPid:\t0\n")
	w.Close()
	if werr != nil {
		t.Fatal(werr)
	}
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("detachScript did not exit within 10s")
	}
	if runErr != nil {
		t.Fatalf("detachScript under an imported SHELLOPTS=errexit: %v (endShell reads this as a failed detach) — the script must take errexit back and exit 0 on every path\n%s", runErr, out.String())
	}
}

// TestDetachScriptEnvCannotSignalAnotherProcess (review advisories
// A-cor-ppid-env-builtins and A-cor-unset-kill-untested): the detach exec
// inherits the container's environment, and an exported function or a $PATH
// entry could put container code where the script calls `kill`, `command` or
// `sleep`. Each case here runs the real script on a target that passes the
// gate (PPid 0) and ignores SIGHUP, so the script runs its whole grace loop
// and then sends SIGKILL. The environment tries to kill a bystander with a
// parent (PPid 7) through one of those names. The bystander must survive, and
// the target must still be ended.
func TestDetachScriptEnvCannotSignalAnotherProcess(t *testing.T) {
	realBash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	for _, c := range []struct {
		name string
		env  func(bystander, fakeBin string) []string
	}{
		{"kill-function", func(b, _ string) []string { return []string{"BASH_FUNC_kill%%=() { builtin kill -KILL " + b + "; }"} }},
		{"command-function", func(b, _ string) []string {
			return []string{"BASH_FUNC_command%%=() { builtin kill -KILL " + b + "; }"}
		}},
		{"sleep-function", func(b, _ string) []string { return []string{"BASH_FUNC_sleep%%=() { builtin kill -KILL " + b + "; }"} }},
		{"sleep-on-path", func(b, fakeBin string) []string { return []string{"PATH=" + fakeBin + ":" + os.Getenv("PATH")} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			bystander := exec.Command("sleep", "30")
			if err := bystander.Start(); err != nil {
				t.Fatal(err)
			}
			bystanderGone := make(chan struct{})
			go func() { _ = bystander.Wait(); close(bystanderGone) }()
			defer func() { _ = bystander.Process.Kill() }()
			target, targetGone := startHUPTarget(t, "", true)

			root := t.TempDir()
			bpid, tpid := strconv.Itoa(bystander.Process.Pid), strconv.Itoa(target.Process.Pid)
			for pid, ppid := range map[string]string{bpid: "7", tpid: "0"} {
				if err := os.MkdirAll(filepath.Join(root, pid), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, pid, "status"), []byte("PPid:\t"+ppid+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			fakeBin := filepath.Join(root, "bin")
			if err := os.Mkdir(fakeBin, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fakeBin, "sleep"), []byte("#!/bin/sh\nkill -KILL "+bpid+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			script := strings.Replace(detachScript, `"/proc/$1/status"`, `"`+root+`/$1/status"`, 1)
			argv := posixBash(script, "deskd-detach", tpid)
			cmd := exec.Command(realBash, argv[1:]...)
			cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, c.env(bpid, fakeBin)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("detachScript: %v\n%s", err, out)
			}
			select {
			case <-bystanderGone:
				t.Fatalf("the container environment (%s) made detachScript's run kill pid %s, whose status reads PPid 7 — only deskd's own shell may be signalled", c.name, bpid)
			case <-time.After(300 * time.Millisecond):
			}
			select {
			case <-targetGone:
			case <-time.After(2 * time.Second):
				t.Fatalf("detachScript left the HUP-ignoring target %s running although its status reads PPid 0 — detach would leak deskd's shell", tpid)
			}
		})
	}
}
