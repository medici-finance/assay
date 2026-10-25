package container

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// testContainerName returns the COCKPIT_VERIFY_CONTAINER fixture name, or
// skips. These tests need a local Docker daemon and the named fixture
// container (the container driver Verify rows 2-6, ground rules: "docker run/exec/rm of
// the NAMED throwaway fixture cockpit-verify... is the ONE sanctioned
// mutation"). A plain `go test ./...` (row 1, no env var, e.g. CI) must see a
// clean skip here, never a failure or a daemon dependency.
func testContainerName(t *testing.T) string {
	t.Helper()
	name := os.Getenv("COCKPIT_VERIFY_CONTAINER")
	if name == "" {
		t.Skip("COCKPIT_VERIFY_CONTAINER not set — this test needs a local Docker daemon and a fixture container (the container driver Verify rows 2-6)")
	}
	return name
}

// testRegistry builds a one-desk [terminal.Registry] whose only desk's target
// is the named container — the same shape the terminal core's registry_test.go uses.
func testRegistry(containerName string) (*terminal.Registry, string) {
	const deskID = "cockpit-verify/review"
	cfg := &schema.File{Cells: []schema.Cell{{
		Name: "cockpit-verify",
		Loops: []schema.Loop{
			{Role: schema.RoleReview, Target: &schema.TargetSpec{
				Kind:      schema.TargetContainer,
				Container: containerName,
			}},
		},
	}}}
	return terminal.NewRegistry(cfg), deskID
}

// waitForState polls the registry until the desk reaches want or timeout
// elapses, failing the test on timeout.
func waitForState(t *testing.T, r *terminal.Registry, id string, want terminal.AgentState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for {
		h, ok := r.Resolve(id)
		if !ok {
			t.Fatalf("desk %q did not resolve", id)
		}
		last = h.State
		if h.State == string(want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("state did not reach %q within %s, last seen %q", want, timeout, last)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestContainerDriverRoundTrip is Verify row 3: attach THROUGH
// terminal.Registry (not the driver directly), write a command whose output
// names the container's own hostname, and assert the ring contains it —
// where the expected value is read INDEPENDENTLY via a separate `docker exec`
// from the test, never typed by this test and grepped back.
func TestContainerDriverRoundTrip(t *testing.T) {
	name := testContainerName(t)
	drv := NewDriver()
	r, id := testRegistry(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.Attach(ctx, id, drv, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitForState(t, r, id, terminal.StateIdle, 10*time.Second)

	// The expected value, sourced independently of the seam under test.
	hostname, err := exec.Command("docker", "exec", name, "cat", "/etc/hostname").Output()
	if err != nil {
		t.Fatalf("independent `docker exec %s cat /etc/hostname`: %v", name, err)
	}
	want := "cockpit-" + strings.TrimSpace(string(hostname))

	if err := r.Write(id, []byte("printf 'cockpit-%s\\n' \"$(cat /etc/hostname)\"\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		b, err := r.Since(id, 0)
		if err != nil {
			t.Fatalf("Since: %v", err)
		}
		if bytes.Contains(b, []byte(want)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("ring never contained %q (independently sourced); last scrollback: %q", want, b)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestContainerDriverStateAfterPrompt is Verify row 4: after the login shell's
// prompt settles the handle reads idle; a command that announces itself via
// the desk-skill's OSC-title convention (state.go: "the OSC title the desk
// skill sets") reads busy for as long as that title says "working" — even
// through a silent `sleep`, since the title signal alone decides once the PTY
// signal goes quiet (deriveState's PromptUnknown branch) — never waiting
// (nothing here asks a human), and the state returns to idle once the title
// says so and the shell's prompt has actually returned.
//
// The write sets and clears the title itself rather than relying on bare PTY
// chatter: a `sleep N` alone produces no bytes, so the two-signal machine
// has no way to tell "still running silently" from "hung" and
// correctly reads could-not-check, not busy, once the ~200ms idle window
// passes with nothing to look at. That is a real, already-shipped product
// property this brief does not own; exercising it with a bare `sleep 5\n`
// would test the heuristic's known blind spot rather than what row 4 is
// actually for — that busy/idle track this driver's real bytes, not a mock.
//
// The fixture image's default bash embeds an OSC-0 title directly in its
// PS1 (measured: it fires on every prompt redraw, unrelated to
// PROMPT_COMMAND), which would otherwise stomp this test's own "desk:
// working"/"desk: idle" markers the instant the shell redraws its prompt.
// The test first swaps in a plain PS1 with no title escape of its own, so
// the only OSC titles in the stream from then on are the ones this test
// sets — a real behaviour of the actual fixture image, not a driver concern.
func TestContainerDriverStateAfterPrompt(t *testing.T) {
	name := testContainerName(t)
	drv := NewDriver()
	r, id := testRegistry(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.Attach(ctx, id, drv, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitForState(t, r, id, terminal.StateIdle, 10*time.Second)

	if err := r.Write(id, []byte("unset PROMPT_COMMAND; PS1='\\$ '\n")); err != nil {
		t.Fatalf("Write (PS1 swap): %v", err)
	}
	waitForState(t, r, id, terminal.StateIdle, 5*time.Second)

	if err := r.Write(id, []byte("printf '\\033]0;desk: working\\007'; sleep 3; printf '\\033]0;desk: idle\\007'\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	waitForState(t, r, id, terminal.StateBusy, 2*time.Second)

	// Never waiting while the sleep runs (nothing here asks a human), and the
	// state must settle back to idle once the idle title fires and the
	// shell's prompt actually returns. A momentary could-not-check right at
	// the busy-to-idle handoff is not asserted against here: the idle title
	// and the still-flowing bytes of the printf that set it are, for an
	// instant, two disagreeing signals by the shipped design (state.go's
	// disagreement rule) — a real, correct property of the terminal core, not
	// something this row is testing.
	deadline := time.Now().Add(8 * time.Second)
	for {
		h, ok := r.Resolve(id)
		if !ok {
			t.Fatalf("desk %q did not resolve", id)
		}
		if h.State == string(terminal.StateWaiting) {
			t.Fatalf("state=waiting while a plain sleep runs — no ask was made")
		}
		if h.State == string(terminal.StateIdle) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("state never returned to idle after the sleep completed; last seen %q", h.State)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

// TestContainerDriverUnreachableIsCouldNotCheck is Verify row 5: a container
// that does not exist yields [terminal.ErrUnreachable] from the driver, which
// the registry renders as a could-not-check ROW — never a 404, never idle.
func TestContainerDriverUnreachableIsCouldNotCheck(t *testing.T) {
	name := testContainerName(t) // row 5 sets COCKPIT_VERIFY_CONTAINER=does-not-exist
	drv := NewDriver()
	r, id := testRegistry(name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.Attach(ctx, id, drv, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach with a missing container returned a hard error, want nil + could-not-check: %v", err)
	}

	h, ok := r.Resolve(id)
	if !ok {
		t.Fatal("a desk whose target is missing must still resolve as a ROW — never a miss (spec §5.2)")
	}
	if h.State != string(terminal.StateCouldNotCheck) {
		t.Fatalf("state=%q, want could-not-check", h.State)
	}
	if h.Connected {
		t.Fatal("connected=true for an unreachable target")
	}
}

// TestContainerDriverStopMidSession is Verify row 6: the container is
// stopped out from under a live session; within 2s the state reads
// could-not-check and Alive() reads false.
func TestContainerDriverStopMidSession(t *testing.T) {
	name := testContainerName(t)
	drv := NewDriver()
	r, id := testRegistry(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.Attach(ctx, id, drv, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	waitForState(t, r, id, terminal.StateIdle, 10*time.Second)

	if out, err := exec.Command("docker", "stop", "-t", "1", name).CombinedOutput(); err != nil {
		t.Fatalf("docker stop %s: %v: %s", name, err, out)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		h, _ := r.Resolve(id)
		alive := r.Alive(id)
		if h.State == string(terminal.StateCouldNotCheck) && !alive {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("state/alive did not settle to could-not-check/false within 2s: state=%q alive=%v", h.State, alive)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
