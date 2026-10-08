package deskkit

// prepush.go — running a repository's pre-push hook for an IN-PROCESS push.
//
// gitcore.Push runs no hooks. Where a desk tool replaces a `git push` child with it
// (deskpr's branch push, verifyloop's durable push), the repository's
// configured pre-push hook — the deskpushguard shim wherever the guard is installed — would
// silently stop guarding that push. PrePushHook keeps it in the path: it resolves the hook
// the way git does and runs it with git's own contract before any byte is sent, and a
// non-zero exit is a refusal the caller must honour.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// PrePushHook runs one checkout's pre-push hook.
type PrePushHook struct {
	// Dir is any directory inside the checkout being pushed from.
	Dir string
	// HookPath answers `git rev-parse --path-format=absolute --git-path hooks/pre-push` for
	// Dir — the path git itself would run, core.hooksPath applied from every config scope.
	// It is a local config read (no remote), supplied by the caller through its own git seam.
	HookPath func() (string, error)
	// Command builds the hook process; nil means exec.Command. The hook is whatever the
	// repository configured, never git.
	Command func(name string, args ...string) *exec.Cmd
	// Stderr receives the hook's own output (stdout and stderr), as git relays it. Nil
	// discards it.
	Stderr io.Writer
}

// Run runs the hook, if one is configured and runnable, for one ref update: argv
// `<remoteName> <remoteURL>`, stdin `<srcRef> <srcSHA> <dstRef> <zero-id>`, cwd the worktree
// root. The remote sha is the all-zero id (git's "not known to exist"), so a hook judging
// what a push adds judges the whole branch — the conservative reading. It returns nil when no
// hook is configured (git would run none either) and a non-nil error when the hook could not
// be located or exited non-zero; the caller treats that as the push being refused.
func (h PrePushHook) Run(remoteName, remoteURL, srcRef, srcSHA, dstRef string) error {
	if h.HookPath == nil {
		return fmt.Errorf("pre-push hook: no hook-path resolver supplied")
	}
	raw, err := h.HookPath()
	if err != nil {
		return fmt.Errorf("cannot resolve the pre-push hook path: %w", err)
	}
	hook := runnableHookFile(strings.TrimSpace(raw))
	if hook == "" {
		return nil
	}
	top, err := gitcore.Toplevel(h.Dir)
	if err != nil {
		return fmt.Errorf("cannot resolve the worktree root to run the pre-push hook in: %w", err)
	}
	command := h.Command
	if command == nil {
		command = exec.Command
	}
	cmd := command(hook, remoteName, remoteURL)
	cmd.Dir = top
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s %s %s %s\n", srcRef, srcSHA, dstRef, strings.Repeat("0", len(srcSHA))))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if out.Len() > 0 && h.Stderr != nil {
		fmt.Fprint(h.Stderr, out.String())
	}
	if runErr != nil {
		return fmt.Errorf("the pre-push hook (%s) refused the push: %w", hook, runErr)
	}
	return nil
}

// runnableHookFile returns the hook file git would run for path, or "" for none. Off Windows
// that is path itself when it is an executable regular file (git ignores a non-executable
// hook). On Windows a hook is started natively only through a .cmd / .exe beside it (the push
// guard's installer writes pre-push.cmd next to the sh shim); a bare file with no such pair is
// returned as-is, so starting it fails loudly rather than the hook being skipped.
func runnableHookFile(path string) string {
	if path == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		for _, ext := range []string{".cmd", ".exe"} {
			if fi, err := os.Stat(path + ext); err == nil && fi.Mode().IsRegular() {
				return path + ext
			}
		}
		if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
			return path
		}
		return ""
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return ""
	}
	return filepath.Clean(path)
}
