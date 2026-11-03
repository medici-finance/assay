package container

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestPIDLineParsing pins what counts as the attach command's pid line: a
// bare decimal above 1 (never PID 1, the container's own init), CR tolerated.
// Anything else is scrollback, forwarded unchanged, and leaves the pid
// unknown, so Close signals nothing.
func TestPIDLineParsing(t *testing.T) {
	for _, c := range []struct {
		in      string
		pid     int
		rest    string
		decided bool
	}{
		{"12\r\nhello", 12, "hello", true},
		{"12\n", 12, "", true},
		{"12", 0, "", false},
		{"1\r\n", 0, "1\r\n", true},
		{"0\r\n", 0, "0\r\n", true},
		{"-5\r\n", 0, "-5\r\n", true},
		{"+5\r\n", 0, "+5\r\n", true},
		{"Error response from daemon: no such container\r\n", 0, "Error response from daemon: no such container\r\n", true},
		{strings.Repeat("7", maxPIDLine+1), 0, strings.Repeat("7", maxPIDLine+1), true},
	} {
		pid, rest, decided := splitPIDLine([]byte(c.in))
		if pid != c.pid || string(rest) != c.rest || decided != c.decided {
			t.Errorf("splitPIDLine(%q) = (%d, %q, %v), want (%d, %q, %v)", c.in, pid, rest, decided, c.pid, c.rest, c.decided)
		}
	}
}

// TestDetachBudgetLeavesHeadroom (the service contract review advisory
// A-cor-detach-budget): Close's whole detach is bounded by detachTimeout, and
// the pid-line wait, the script's HUP→KILL grace and the engine's own exec
// latency all come out of that one budget. If the context fires first, the
// SIGKILL a HUP-ignoring shell needs is never sent. Measured on Docker 29.4.0
// with the amd64 fixture emulated on arm64, the exec overhead around the
// script was about 0.75 s; the headroom below doubles it.
func TestDetachBudgetLeavesHeadroom(t *testing.T) {
	const execHeadroom = 1500 * time.Millisecond
	if got := pidLineWait + killGrace + execHeadroom; got > detachTimeout {
		t.Fatalf("pidLineWait %s + killGrace %s + exec headroom %s = %s, over detachTimeout %s: a HUP-ignoring shell can outlive the detach", pidLineWait, killGrace, execHeadroom, got, detachTimeout)
	}
	loop := fmt.Sprintf(`"$i" -lt %d ]`, int(killGrace/(100*time.Millisecond)))
	if !strings.Contains(detachScript, loop) {
		t.Fatalf("detachScript's HUP→KILL loop does not match killGrace %s (want %q in the script)", killGrace, loop)
	}
}
