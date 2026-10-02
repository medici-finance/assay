//go:build unix

package cellprocess

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type unixTree struct{}

func newProcessTree(cmd *exec.Cmd) (processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return unixTree{}, nil
}

func (unixTree) started(*os.Process) error { return nil }
func (unixTree) close() error              { return nil }
func (unixTree) kill(p *os.Process) error {
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
func (unixTree) empty(p *os.Process) (bool, error) {
	err := syscall.Kill(-p.Pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	return false, err
}
