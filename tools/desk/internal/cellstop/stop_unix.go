//go:build darwin || linux

package cellstop

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func inventory() (map[int]process, error) {
	metadata, err := exec.Command("ps", "-axo", "uid=,pid=,ppid=,pgid=,lstart=,stat=,comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot inspect process ownership: %w", err)
	}
	result := map[int]process{}
	for _, line := range strings.Split(string(metadata), "\n") {
		f := strings.Fields(line)
		if len(f) < 11 || strings.HasPrefix(f[9], "Z") {
			continue
		}
		group, _ := strconv.Atoi(f[3])
		f = append(f[:3:3], f[4:]...) // the identity layout below predates the pgid column
		uid, _ := strconv.Atoi(f[0])
		if uid != os.Geteuid() {
			continue
		}
		pid, e := strconv.Atoi(f[1])
		if e != nil {
			continue
		}
		parent, _ := strconv.Atoi(f[2])
		result[pid] = process{pid: pid, parent: parent, group: group, identity: strings.Join(append(f[3:8:8], f[9:]...), " "), env: map[string]string{}}
	}
	for pid, p := range result {
		env, err := processEnvironment(pid)
		if err != nil {
			continue
		} // exits and inaccessible unrelated processes have no attribution
		p.env = env
		result[pid] = p
	}
	return result, nil
}
func ancestors(all map[int]process) map[int]bool {
	r := map[int]bool{}
	for pid := os.Getpid(); pid > 0; {
		if r[pid] {
			break
		}
		r[pid] = true
		p, ok := all[pid]
		if !ok {
			break
		}
		pid = p.parent
	}
	return r
}
func terminate(p process, force bool) error {
	// A second inventory is intentional: an old PID is never authority to kill
	// a replacement process whose start identity differs.
	all, err := inventory()
	if err != nil {
		return err
	}
	current, ok := all[p.pid]
	if !ok || current.identity != p.identity {
		return nil
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(p.pid, sig); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("cannot stop owned process %d: %w", p.pid, err)
	}
	return nil
}

// signalMarked sends sig to p only if a fresh inventory still shows the same
// process (pid AND start identity) carrying the same cell marker. A pid alone
// is never authority: a reused pid fails the identity comparison.
func signalMarked(p process, m Marker, sig syscall.Signal) error {
	all, err := inventory()
	if err != nil {
		return err
	}
	current, ok := all[p.pid]
	if !ok || current.identity != p.identity || !m.carries(current) {
		return nil
	}
	// The ps identity has one-second resolution. Immediately before the signal, the kernel's own
	// start time (microseconds on macOS, clock ticks on Linux) must still match the one recorded
	// at listing time; a changed one is a different process, and an unreadable one is
	// could-not-check — neither is ever signalled.
	start, gone, err := deskkit.ProcessStart(p.pid)
	switch {
	case gone || err == nil && start != p.start:
		return nil
	case err != nil:
		return fmt.Errorf("could-not-check pid %d's start time before signalling: %w", p.pid, err)
	}
	if err := syscall.Kill(p.pid, sig); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("cannot signal owned process %d: %w", p.pid, err)
	}
	return nil
}

const (
	sigTerm = syscall.SIGTERM
	sigKill = syscall.SIGKILL
)
