package local

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/creack/pty"
)

// Session is one live local-PTY attachment. It implements terminal.Session.
// Pid is exported beyond that interface (the interface owns only the byte
// pipe, per internal/terminal/target.go's doc) so a caller that holds the
// concrete type — a test proving the round trip against the REAL spawned
// process rather than a fake reader — can dereference
// the bytes it reads back against the process that actually produced them.
type Session struct {
	cmd  *exec.Cmd
	ptmx *os.File

	ch chan []byte

	closeOnce sync.Once
	ended     atomic.Bool
}

func newSession(cmd *exec.Cmd, ptmx *os.File) *Session {
	s := &Session{cmd: cmd, ptmx: ptmx, ch: make(chan []byte, 64)}
	go s.pump()
	return s
}

// pump copies PTY output into ch until the PTY read errors (the shell
// exited, or Close tore the attachment down), then closes the PTY master,
// reaps the child, and marks the session ended. It is the ONLY writer/closer
// of ch, so whichever of "the shell exited" or "Close was called" happens
// first drives a single, clean shutdown.
//
// The natural-exit path (a shell that exits on its own, e.g. "exit" or a
// short-lived command) never has anything else call Close(): the registry's
// own pump just drains Read() until it closes and then drops the Session
// reference (internal/terminal/registry.go's pump), so if this method didn't
// close the master fd itself, nothing would — a leak under normal use, not
// just a caller bug. closePTY shares closeOnce with Close so a concurrent
// explicit Close() cannot double-close.
func (s *Session) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.ch <- chunk
		}
		if err != nil {
			break
		}
	}
	s.closePTY()
	_ = s.cmd.Wait() // reap; ignore the exit error, the registry only cares that the channel closed
	s.ended.Store(true)
	close(s.ch)
}

// closePTY closes the PTY master exactly once, whether triggered by pump's
// own natural-exit teardown or by an explicit Close call. It never touches
// the child process — Close is the only path that kills a still-running one.
func (s *Session) closePTY() {
	s.closeOnce.Do(func() {
		_ = s.ptmx.Close()
	})
}

// Read implements terminal.Session.
func (s *Session) Read() <-chan []byte { return s.ch }

// Write implements terminal.Session: it writes directly to the PTY master, so
// bytes reach the child's stdin exactly as typed (no server-side transcoding
// — the terminal spike spike ANSWER 4).
func (s *Session) Write(p []byte) error {
	_, err := s.ptmx.Write(p)
	return err
}

// Resize implements terminal.Session via TIOCSWINSZ (pty.Setsize wraps the
// ioctl; the local driver task 1).
func (s *Session) Resize(cols, rows int) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: clampWinsize(rows),
		Cols: clampWinsize(cols),
	})
}

// Alive implements terminal.Session: false once pump has reaped the child
// (shell exit or Close), true until then.
func (s *Session) Alive() bool {
	return !s.ended.Load()
}

// Close implements terminal.Session. It closes the PTY master (which makes
// the pump's next Read error out, sharing closePTY's closeOnce so a
// natural-exit teardown racing with an explicit Close cannot double-close)
// and kills the child if it is still running; pump's own goroutine performs
// the Wait/reap exactly once. Close is idempotent.
func (s *Session) Close() error {
	s.closePTY()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

// Pid returns the spawned child's process id. See the Session doc comment for
// why this is exported on the concrete type rather than the interface.
func (s *Session) Pid() int {
	if s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}
