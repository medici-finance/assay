//go:build darwin

package deskkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func procAlive(pid int) error {
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return errProcessGone
	} else if err != nil && !errors.Is(err, unix.EPERM) {
		return err
	}
	return nil
}

func procInfo(pid int) (*unix.KinfoProc, error) {
	if err := procAlive(pid); err != nil {
		return nil, err
	}
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if procAlive(pid) == errProcessGone {
			return nil, errProcessGone
		}
		return nil, err
	}
	if int(kp.Proc.P_pid) != pid {
		return nil, errProcessGone
	}
	if kp.Proc.P_stat == 5 { // SZOMB: a zombie holds no descriptors
		return nil, errProcessGone
	}
	return kp, nil
}

// procStart returns pid's start time from the kernel process table, which
// distinguishes a reused pid from the recorded process.
func procStart(pid int) (string, error) {
	kp, err := procInfo(pid)
	if err != nil {
		return "", err
	}
	t := kp.Proc.P_starttime
	return fmt.Sprintf("%d.%06d", t.Sec, t.Usec), nil
}

func procCommand(pid int) (string, error) {
	kp, err := procInfo(pid)
	if err != nil {
		return "", err
	}
	c := kp.Proc.P_comm[:]
	if i := bytes.IndexByte(c, 0); i >= 0 {
		c = c[:i]
	}
	return string(c), nil
}

// lsofPath is where macOS ships lsof. It is a fixed system path rather than a
// PATH lookup so a planted lsof earlier on PATH is never consulted.
const lsofPath = "/usr/sbin/lsof"

// procHoldsFile asks lsof which processes have f's path open. Only a listing
// that names pid is "yes"; lsof missing, failing or timing out is could-not-check.
func procHoldsFile(pid int, f *os.File) (bool, error) {
	if _, err := os.Stat(lsofPath); err != nil {
		return false, fmt.Errorf("%w: lsof not found at %s", errCouldNotCheck, lsofPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, lsofPath, "-n", "-P", "-w", "-F", "p", "--", lockPath(f))
	cmd.Stdout = &stdout
	runErr := cmd.Run()
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "p") {
			if n, e := strconv.Atoi(line[1:]); e == nil && n == pid {
				return true, nil
			}
		}
	}
	var exit *exec.ExitError
	if runErr == nil || errors.As(runErr, &exit) && exit.ExitCode() == 1 && ctx.Err() == nil {
		return false, nil
	}
	return false, fmt.Errorf("%w: lsof: %v", errCouldNotCheck, runErr)
}

func procTerminate(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
