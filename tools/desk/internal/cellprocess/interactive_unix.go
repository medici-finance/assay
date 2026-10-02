//go:build unix

package cellprocess

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type foregroundTree struct {
	processTree
	fd, previous int
}

func newInteractiveProcessTree(cmd *exec.Cmd) (processTree, error) {
	tree, err := newProcessTree(cmd)
	if err != nil {
		return nil, err
	}
	input, ok := cmd.Stdin.(*os.File)
	if !ok || input == nil || !term.IsTerminal(int(input.Fd())) {
		return tree, nil
	}
	fd := int(input.Fd())
	previous, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		_ = tree.close()
		return nil, err
	}
	if previous != syscall.Getpgrp() {
		_ = tree.close()
		return nil, errors.New("interactive owner is not the terminal foreground process group")
	}
	// Foreground performs the handoff inside fork/exec before any child reads.
	cmd.SysProcAttr.Foreground = true
	cmd.SysProcAttr.Ctty = fd
	return foregroundTree{tree, fd, previous}, nil
}

func (t foregroundTree) close() error {
	// The parent is temporarily a background group. Restore its terminal after
	// reaping without SIGTTOU suspending the supervisor while it owns the lease.
	ignored := signal.Ignored(syscall.SIGTTOU)
	signal.Ignore(syscall.SIGTTOU)
	err := unix.IoctlSetPointerInt(t.fd, unix.TIOCSPGRP, t.previous)
	if !ignored {
		signal.Reset(syscall.SIGTTOU)
	}
	return errors.Join(err, t.processTree.close())
}
