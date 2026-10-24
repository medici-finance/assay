package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
)

// detachTimeout bounds the one `docker exec` Close makes to end deskd's shell
// inside the container, so a wedged engine cannot hold shutdown open.
const detachTimeout = 4 * time.Second

// killGrace is how long detachScript waits after SIGHUP before it sends
// SIGKILL: its loop runs killGrace / 100 ms times. It is 1 s, not 2 s, so the
// pid-line wait, this grace and the engine's exec latency all fit inside
// detachTimeout with headroom (the service contract review advisory
// A-cor-detach-budget; TestDetachBudgetLeavesHeadroom). A login `bash -l`
// normally ends on SIGHUP at once.
const killGrace = time.Second

// detachScript is what Close runs, through bash (which the attach command
// already requires), to end deskd's OWN shell inside the container: SIGHUP,
// the signal a terminal hang-up would have sent; then, if the shell ignores
// it, SIGKILL after killGrace (1 s). The pid is a positional argument ("$1"), never
// interpolated into the script. It signals that one pid and nothing else.
//
// It does not trust the pid line alone. The container's own environment can
// still forge that line (an exported function named echo; xtrace with a
// crafted PS4 on bash 5.x), so before each signal the script checks that the
// target was entered by `docker exec`: such a process has PPid 0 inside the
// container's pid namespace, while every process started inside the container
// has its parent there (the service contract security advisory
// A-sec-pidline-trust). PID 1 also has PPid 0, and splitPIDLine never accepts
// it. When /proc cannot be read (or under --pid=host, where the exec'd shell's
// parent is the runtime shim), the check fails closed and nothing is
// signalled.
//
// The exec inherits the container's environment, so the script first takes
// back the names it relies on. `unset -f read kill command` drops any function
// of those names the environment exported (`unset` is a special builtin, which
// a function cannot shadow in posix mode): an exported `read` cannot make the
// check pass for another pid, and an exported `kill` or `command` cannot
// signal some other process between the checks. `set +x` ends an xtrace the
// environment turned on (SHELLOPTS), so a crafted PS4 is not evaluated inside
// the check; `set +e` ends an imported errexit, so a `kill -KILL` on a target
// that exited after the re-check does not end the script with rc 1, which
// endShell would read as a failed detach (round-9 advisory). `read`, `kill`,
// `[`, `set`, `unset` and `command` are builtins; `sleep` is not, so it runs
// as `command -p sleep`, which skips functions and looks only on the
// system's standard path (the libc's _CS_PATH, /bin and /usr/bin in practice),
// never the container's $PATH
// (the service contract review advisories A-cor-ppid-env-builtins,
// A-cor-unset-kill-untested, A-sec-pidline-trust). On an image with no
// `sleep` on that path the grace loop does not wait, so SIGKILL follows
// SIGHUP almost at once: fail-safe (the shell still ends), only faster.
//
// What stays open: a PS4 with a command substitution runs before the first
// statement, and an exported `unset` function makes bash 5.x abort with rc 2
// before the script runs (A-cor-unset-import-abort). Both need the party that
// already sets the exec's environment, which runs code in deskd's execs
// anyway. The abort fails closed: nothing is signalled, deskd's shell keeps
// running, and endShell returns the failed exec to Close's caller, which logs
// it: the Supervisor for a configured desk, the session store for a custom
// session (review F-cor-7).
const detachScript = `unset -f read kill command; set +e +x; entered() { p=; while read -r k v; do if [ "$k" = PPid: ]; then p=$v; break; fi; done 2>/dev/null < "/proc/$1/status"; [ "$p" = 0 ]; }; entered "$1" || exit 0; kill -HUP "$1" 2>/dev/null || exit 0; i=0; while [ "$i" -lt 10 ]; do kill -0 "$1" 2>/dev/null || exit 0; command -p sleep 0.1; i=$((i+1)); done; entered "$1" || exit 0; kill -KILL "$1" 2>/dev/null; exit 0`

// maxPIDLine caps how many bytes pump holds back while it looks for the
// attach command's pid line.
const maxPIDLine = 32

// pidLineWait bounds how long Close waits for the pid line when it arrives
// before pump has read it (the supervisor detaches straight after Attach
// returns when Stop races an in-flight attempt). It is spent inside
// detachTimeout, so Close's whole bound is unchanged.
const pidLineWait = time.Second

// posixBash is how this driver runs bash in a container: `bash --posix -c
// <script> [args]`. A non-interactive bash in posix mode reads no startup
// file, so a $BASH_ENV set in the container's environment runs nothing before
// the script, and cannot print a decoy pid line ahead of the real one
// (the service contract security advisory A-sec-pidline-trust). Posix mode is not
// passed on: the `bash -l` the attach script execs is an ordinary login shell.
// It adds no `-e` or `-u`: the exec keeps the container's own user and
// environment (TestDriverNeverOverridesUserOrEnv).
func posixBash(script string, args ...string) []string {
	return append([]string{"bash", "--posix", "-c", script}, args...)
}

// session is one live `docker exec` attachment under a pty — the
// [terminal.Session] this driver hands the registry. It owns the pty master
// and a reader goroutine that feeds Read()'s channel, plus a waiter goroutine
// that reaps the child and tracks liveness. The driver owns bytes only — no
// timestamp, no cursor, no ring lives here (target.go's contract: "the
// registry owns per-desk state... the Target owns only the byte pipe").
type session struct {
	cmd       *exec.Cmd
	ptmx      *os.File
	container string

	ch chan []byte

	mu     sync.Mutex
	closed bool

	exited atomic.Bool
	// eof is set once the pty read side has ended: the exec is over, so its
	// pid may already belong to another process and Close must not signal it.
	eof atomic.Bool
	// pid is the in-container pid of deskd's shell, as the attach command
	// reported it on its first line (0 = unknown). Close signals it, because
	// the container engine does not end an exec'd process when its client
	// goes away (the service contract review finding F-cor-1).
	pid atomic.Int64
	// pidDecided is closed once pump has decided the pid line (pid set, or
	// the first bytes were not one) or the pty read side has ended, whichever
	// is first. Close waits on it, bounded by pidLineWait.
	pidDecided chan struct{}
	pidOnce    sync.Once
}

func newSession(cmd *exec.Cmd, ptmx *os.File, container string) *session {
	s := &session{cmd: cmd, ptmx: ptmx, container: container, ch: make(chan []byte, 64), pidDecided: make(chan struct{})}
	go s.pump()
	go s.wait()
	return s
}

// pump reads the pty master until it errors or closes (a stopped container
// makes the docker exec CLI exit, closing its end — facts: "a stopped
// container makes Read() return EOF"), forwarding each chunk to ch, then
// closes ch. A closed channel is how the registry's own pump (terminal
// package) learns the attachment ended.
//
// The first line the attach command prints is the in-container pid of the
// shell (see attachCmd). pump records it and does not forward it: it is
// deskd's bookkeeping, not scrollback. Anything that is not such a line (a
// daemon error, say) is forwarded unchanged and leaves the pid unknown.
func (s *session) pump() {
	defer close(s.ch)
	buf := make([]byte, 32*1024)
	var head []byte
	decided := false
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			out := buf[:n]
			if !decided {
				head = append(head, out...)
				var pid int
				pid, out, decided = splitPIDLine(head)
				if decided {
					s.pid.Store(int64(pid))
					head = nil
					s.markPIDDecided()
				}
			}
			if len(out) > 0 {
				s.ch <- append([]byte(nil), out...)
			}
		}
		if err != nil {
			if !decided && len(head) > 0 {
				s.ch <- head
			}
			s.eof.Store(true)
			s.markPIDDecided()
			return
		}
	}
}

// markPIDDecided releases a Close waiting for the pid line. Idempotent.
func (s *session) markPIDDecided() {
	s.pidOnce.Do(func() { close(s.pidDecided) })
}

// splitPIDLine looks for the attach command's pid line at the start of head.
// decided=false means it needs more bytes. When decided, pid is the reported
// pid (0 when head does not start with one) and rest is what to forward as
// scrollback. Only a bare decimal above 1 counts: PID 1 is the container's own
// init and is never deskd's to signal.
func splitPIDLine(head []byte) (pid int, rest []byte, decided bool) {
	i := bytes.IndexByte(head, '\n')
	if i < 0 {
		if len(head) >= maxPIDLine {
			return 0, head, true
		}
		for _, b := range head {
			if (b < '0' || b > '9') && b != '\r' {
				return 0, head, true
			}
		}
		return 0, nil, false
	}
	line := bytes.TrimRight(head[:i], "\r")
	if len(line) == 0 || len(line) > 10 {
		return 0, head, true
	}
	for _, b := range line {
		if b < '0' || b > '9' {
			return 0, head, true
		}
	}
	p, err := strconv.Atoi(string(line))
	if err != nil || p <= 1 {
		return 0, head, true
	}
	return p, head[i+1:], true
}

// wait reaps the docker-exec child and records that it exited — the source
// [Alive] reads, so a dead process is never mistaken for a live one.
func (s *session) wait() {
	_ = s.cmd.Wait()
	s.exited.Store(true)
}

// Read implements [terminal.Session].
func (s *session) Read() <-chan []byte { return s.ch }

// Write implements [terminal.Session]: bytes go straight to the pty master —
// this driver's half of the byte pipe target.go documents ("Write sends
// input (the send route gates the write route, but the transport is here)").
func (s *session) Write(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("container driver: write after close")
	}
	_, err := s.ptmx.Write(p)
	return err
}

// Resize implements [terminal.Session] via the pty ioctl, which the kernel
// treats as idempotent (facts: "ContainerExecResize is idempotent" — the pty
// equivalent, TIOCSWINSZ, is likewise safe to repeat).
func (s *session) Resize(cols, rows int) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Alive implements [terminal.Session]: false once the docker-exec child has
// exited (facts: liveness), never a panic on a gone process.
func (s *session) Alive() bool {
	return !s.exited.Load()
}

// Close implements [terminal.Session]. It ends deskd's OWN shell inside the
// container, then closes the pty master, which ends the host `docker exec`
// CLI. Closing the pty alone is not enough: the container engine does not end
// an exec'd process when its client goes away, so every detach would leave
// one more `bash -l` running in the desk (the service contract review finding
// F-cor-1). The container itself keeps running. Idempotent: a second Close is
// a no-op, never a double-close panic.
func (s *session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	detachErr := s.endShell()
	return errors.Join(detachErr, s.ptmx.Close())
}

// endShell signals deskd's shell inside the container, through a second,
// non-interactive `docker exec` on the same container (detachScript). When
// Close arrives before pump has read the pid line, it first waits for that
// line, up to pidLineWait (the service contract review advisory
// A-cor-detach-pid-race); the wait and the exec together stay inside
// detachTimeout. It does nothing when the pid is still unknown after that, or
// when the exec has already ended by itself: that pid may belong to another
// process by now.
//
// A detach exec that fails (engine error, the detachTimeout bound, or bash
// aborting before the script runs, as an exported `unset` function makes bash
// 5.x do) is returned, naming the container and pid: the signal was not
// confirmed sent, so deskd's shell may still be running there. The script
// itself exits 0 on every path it reaches, including a refusal to signal and
// under an errexit the environment imported (its `set +e`).
func (s *session) endShell() error {
	if s.container == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), detachTimeout)
	defer cancel()
	wait := time.NewTimer(pidLineWait)
	defer wait.Stop()
	select {
	case <-s.pidDecided:
	case <-wait.C:
	}
	pid := s.pid.Load()
	if pid <= 1 || s.eof.Load() || s.exited.Load() {
		return nil
	}
	args := append([]string{"exec", s.container}, posixBash(detachScript, "deskd-detach", strconv.FormatInt(pid, 10))...)
	if err := exec.CommandContext(ctx, dockerBinary, args...).Run(); err != nil {
		return fmt.Errorf("container %s: the detach exec for deskd's shell (pid %d) failed, so that shell may still be running: %w", s.container, pid, err)
	}
	return nil
}
