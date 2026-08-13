package deskkit

// Adopter CALLOUT plumbing — the shared exec/timeout/fail-closed half of every
// "the adopter supplies the policy, the tool supplies the mechanism" seam.
//
// WHY A CALLOUT AND NOT A CONFIG LIST. A configured list of patterns is a POLICY
// the shared tool then has to interpret, and interpreting somebody else's policy is
// how a tool acquires cases it was never designed for: the list grows a syntax, the
// syntax grows an escape, and the escape is a waiver on a gate. An executable moves
// the whole interpretation to the adopter's side of the boundary. The shared tool's
// contract shrinks to four things it can actually hold: run it, bound it, read one
// answer, and refuse when it cannot.
//
// WHAT THIS PACKAGE DOES AND DOES NOT DECIDE. Run() reports what HAPPENED — the
// callout answered, or it did not. It deliberately does not decide what a failure
// MEANS, because that differs by gate: for a write GUARD a failure means BLOCK, for
// a classifier it means "treat as the highest class". Each caller states its own
// rule at its own call site, where a reader checking the fail-closed property can
// see it next to the decision rather than one package away.
//
// THE ONLY-WIDENS RULE lives at the call sites too, and structurally rather than by
// convention: a caller consults its callout AFTER its own compiled checks have
// already answered, so there is no value a callout can return that clears a
// compiled verdict. A callout adds; it never subtracts.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultCalloutTimeout bounds one callout invocation.
//
// A callout sits in the hot path of a gate that runs per tool call, so the bound is
// what keeps "the adopter's script hung" from becoming "the session hung". Five
// seconds is far more than a pattern match needs and far less than a human waits
// before killing the process themselves — which is the failure mode a generous
// timeout actually produces, and it resolves to no gate at all.
const DefaultCalloutTimeout = 5 * time.Second

// calloutWaitDelay is the grace period between killing a timed-out callout and
// cutting its output pipes. See the WaitDelay assignment in Run for why a timeout
// without it is a timeout on nothing.
const calloutWaitDelay = 250 * time.Millisecond

// maxCalloutOutput caps what Run will read back. A callout that streams is a
// callout that is not answering the question; reading it into memory unbounded
// would make the guard's own footprint the adopter's to choose.
const maxCalloutOutput = 64 << 10

// Callout is one configured adopter executable.
type Callout struct {
	// Path is the ABSOLUTE path of the executable. The configuration loader has
	// already refused a relative one (see EnvWriteguardCallout); Run re-checks
	// rather than trusting that, because a Callout can be constructed directly.
	Path string
	// Timeout bounds one invocation. Zero means DefaultCalloutTimeout.
	Timeout time.Duration
}

// CalloutResult is what a completed callout said.
type CalloutResult struct {
	// Stdout is the callout's standard output, trimmed of surrounding space.
	Stdout string
	// Stderr is its standard error, trimmed — surfaced so a caller can put the
	// adopter's own diagnostic into its refusal message instead of a generic one.
	Stderr string
}

// Run invokes the callout with stdin on its standard input and args as its
// arguments, and returns what it said.
//
// It returns an ERROR — never a partial answer — for every way the question can go
// unanswered: the path is not absolute, the file is missing, it is not a regular
// file, it is not executable, it is writable by group or world, it cannot be
// spawned, it exits non-zero, or it exceeds the timeout. A caller's fail-closed
// rule keys on `err != nil` alone, so there is no failure mode a caller can forget
// to enumerate.
//
// THE WRITABILITY CHECK is the sshd rule applied to an executable, and it is here
// rather than at load time on purpose: a file's mode can change between the run
// that read the roster and the run that invokes it. Anything that can write the
// callout chooses what the gate decides, so a group- or world-writable callout is
// refused — which, at a fail-closed caller, is strictly safer than running it. Note
// what is NOT checked: the file's OWNER. A callout installed root-owned under
// /usr/local/bin is the normal deployment and is if anything better than one owned
// by the invoking user, so requiring same-owner (as the roster FILE does) would
// refuse the good case.
//
// NO SHELL is involved: exec.CommandContext runs the binary directly, so nothing in
// stdin or args is ever word-split, glob-expanded or substituted. The command text a
// guard passes through here is DATA all the way down.
func (c Callout) Run(stdin string, args ...string) (CalloutResult, error) {
	if strings.TrimSpace(c.Path) == "" {
		return CalloutResult{}, errors.New("callout: no path configured")
	}
	if !filepath.IsAbs(c.Path) {
		return CalloutResult{}, fmt.Errorf("callout %q is not an absolute path", c.Path)
	}
	if err := calloutExecutable(c.Path); err != nil {
		return CalloutResult{}, err
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultCalloutTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Path, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: maxCalloutOutput}
	cmd.Stderr = &limitedWriter{w: &errb, n: maxCalloutOutput}
	// WaitDelay is what makes the timeout REAL, and it is not optional here.
	//
	// Because Stdout/Stderr are writers rather than files, os/exec pipes them and Wait
	// blocks until the write end closes. Killing the callout on the deadline does NOT
	// close that end if the callout left a child holding it — a backgrounded process,
	// or simply `sleep` still running when its shell was killed. Without WaitDelay the
	// guard then blocks for as long as that grandchild lives, and the timeout it
	// advertises is a timeout on nothing. Measured: a 150ms deadline against a callout
	// whose child slept 30s returned in 30s.
	//
	// The grace is short on purpose. It starts only AFTER the process has been killed,
	// so it is not time a healthy callout can spend; it exists solely to let a normal
	// exit's final bytes drain before the pipes are cut.
	cmd.WaitDelay = calloutWaitDelay

	err := cmd.Run()
	res := CalloutResult{
		Stdout: strings.TrimSpace(out.String()),
		Stderr: strings.TrimSpace(errb.String()),
	}
	// The timeout is reported as a timeout, not as the generic "signal: killed"
	// exec surfaces when the context kills the child — a caller putting the reason
	// in a refusal message has to be able to say which of the two happened.
	if ctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("callout %q did not answer within %s", c.Path, timeout)
	}
	if err != nil {
		return res, fmt.Errorf("callout %q failed: %w", c.Path, err)
	}
	return res, nil
}

// calloutExecutable enforces what must be true of the file at INVOCATION time.
func calloutExecutable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("callout %q cannot be read: %w", path, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("callout %q is a directory", path)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("callout %q is not a regular file", path)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("callout %q is not executable (mode %04o)", path, fi.Mode().Perm())
	}
	if m := fi.Mode().Perm(); m&0o022 != 0 {
		return fmt.Errorf("callout %q is group- or world-writable (mode %04o): anything that can "+
			"write it chooses what this gate decides. Fix with `chmod 0755 %s`", path, m, path)
	}
	if err := calloutDirNotWritable(filepath.Dir(path)); err != nil {
		return err
	}
	return nil
}

// calloutDirNotWritable refuses a callout whose own DIRECTORY is group- or
// world-writable: replacing the file is then as easy as editing it, so checking the
// file's mode alone would be a check somebody can step around without touching it.
func calloutDirNotWritable(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("callout directory %q does not exist", dir)
		}
		return fmt.Errorf("callout directory %q cannot be read: %w", dir, err)
	}
	if m := fi.Mode().Perm(); m&0o022 != 0 {
		return fmt.Errorf("callout directory %q is group- or world-writable (mode %04o): the "+
			"executable in it can be REPLACED without ever changing its own mode. "+
			"Fix with `chmod 0755 %s`", dir, m, dir)
	}
	return nil
}

// limitedWriter writes at most n bytes and silently discards the rest. Discarding
// is right here: the excess is a callout that is not answering the question, and
// the answer this package reads is a single short line.
type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		l.w.Write(p[:l.n])
		l.n = 0
		return len(p), nil
	}
	l.w.Write(p)
	l.n -= len(p)
	return len(p), nil
}
