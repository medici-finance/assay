package serve

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// SetTerminalStreamTimings shortens the terminal stream's timers for a test and
// returns the restore func. A zero value leaves that timer as it is.
func SetTerminalStreamTimings(heartbeat, statePoll, writeTimeout time.Duration) (restore func()) {
	h, p, w := terminalHeartbeat, terminalStatePoll, terminalWriteTimeout
	if heartbeat > 0 {
		terminalHeartbeat = heartbeat
	}
	if statePoll > 0 {
		terminalStatePoll = statePoll
	}
	if writeTimeout > 0 {
		terminalWriteTimeout = writeTimeout
	}
	return func() { terminalHeartbeat, terminalStatePoll, terminalWriteTimeout = h, p, w }
}

// TerminalFrameBytes is the per-frame byte cap, for the tests that assert it.
const TerminalFrameBytes = terminalFrameBytes

// TerminalStateStep runs the stream's state step once, as the handler's poll
// tick does, for a client that holds cursor, and returns the SSE bytes it
// wrote. It lets a test put the stream in the one position the select loop
// reaches only by scheduling luck: new bytes in the ring, their wake not yet
// served, and the poll tick firing first.
func TerminalStateStep(f *terminal.Follower, cursor uint64) []byte {
	rec := httptest.NewRecorder()
	st := &terminalStream{w: rec, rc: http.NewResponseController(rec), f: f, now: time.Now, cursor: cursor}
	st.state(true)
	return rec.Body.Bytes()
}
