//go:build darwin || linux

package cellstop

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func inventory() (map[int]process, error) {
	metadata, err := exec.Command("ps", "-axo", "uid=,pid=,ppid=,lstart=,stat=,comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot inspect process ownership: %w", err)
	}
	result := map[int]process{}
	for _, line := range strings.Split(string(metadata), "\n") {
		f := strings.Fields(line)
		if len(f) < 10 || strings.HasPrefix(f[8], "Z") {
			continue
		}
		uid, _ := strconv.Atoi(f[0])
		if uid != os.Geteuid() {
			continue
		}
		pid, e := strconv.Atoi(f[1])
		if e != nil {
			continue
		}
		parent, _ := strconv.Atoi(f[2])
		result[pid] = process{pid: pid, parent: parent, identity: strings.Join(append(f[3:8:8], f[9:]...), " "), env: map[string]string{}}
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
	if err := syscall.Kill(p.pid, sig); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("cannot signal owned process %d: %w", p.pid, err)
	}
	return nil
}

const (
	sigTerm = syscall.SIGTERM
	sigKill = syscall.SIGKILL
)
