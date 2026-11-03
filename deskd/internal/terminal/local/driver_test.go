package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// drain reads chunks off ch until it closes or the deadline passes, and
// returns everything read.
func drain(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	var out []byte
	deadline := time.After(timeout)
	for {
		select {
		case b, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, b...)
		case <-deadline:
			return out
		}
	}
}

// ── Verify row 2: round trip through a REAL PTY, dereferenced against the ──
// ── spawned child's own reported pid — not a fake reader.                  ──
//
// "Through the registry" here means through this brief's own registration
// path (terminal.RegisterDriver in this package's init / terminal.DriverFor)
// rather than a locally constructed *Driver value, so the test also proves
// the registration this brief adds (task 2) actually resolves kind: local.
// Raw scrollback-byte reads by cursor are internal/terminal's ring, reserved
// for the terminal stream's SSE stream (see internal/terminal/doc.go) — this test reads
// the session's own byte channel directly, which is the driver's contract
// regardless of which registry sits on top of it.
func TestLocalDriverRoundTrip(t *testing.T) {
	factory, ok := terminal.DriverFor(schema.TargetLocal)
	if !ok {
		t.Fatal("no driver registered for kind: local — terminal.RegisterDriver was not called")
	}
	drv, err := factory()
	if err != nil {
		t.Fatalf("factory(): %v", err)
	}

	spec := &terminal.TargetSpec{
		Kind:    schema.TargetLocal,
		Command: []string{"sh", "-c", `printf 'cockpit-%s\n' "$$"`},
	}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sess.Close()

	ls, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Attach returned %T, want *local.Session", sess)
	}
	pid := ls.Pid()
	if pid <= 0 {
		t.Fatalf("Pid() = %d, want a real positive pid", pid)
	}

	out := drain(t, sess.Read(), 2*time.Second)
	want := fmt.Sprintf("cockpit-%d", pid)
	if !strings.Contains(string(out), want) {
		t.Fatalf("session output = %q, want it to contain %q (the shell's own $$ dereferenced against Session.Pid())", out, want)
	}
}

// ── Verify row 3: deskd's own GitHub token must never reach a child's env. ──
func TestLocalDriverStripsSecretsFromChildEnv(t *testing.T) {
	t.Setenv("DESKD_GITHUB_TOKEN", "leak-canary-9f2")
	t.Setenv("SOME_OTHER_SECRET_VALUE", "also-must-not-leak")

	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"env"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sess.Close()

	out := string(drain(t, sess.Read(), 2*time.Second))
	if strings.Contains(out, "leak-canary-9f2") {
		t.Fatalf("child env leaked DESKD_GITHUB_TOKEN's value into the desk shell:\n%s", out)
	}
	if strings.Contains(out, "also-must-not-leak") {
		t.Fatalf("child env leaked a *_SECRET_* variable's value into the desk shell:\n%s", out)
	}
	if strings.Contains(out, "DESKD_GITHUB_TOKEN") {
		t.Fatalf("child env still names DESKD_GITHUB_TOKEN at all:\n%s", out)
	}
}

// ── Verify row 4: the P3 re-check — off-loopback bind, auth off, refused. ──
func TestLocalDriverRefusesOffLoopbackWithoutAuth(t *testing.T) {
	drv := New()
	drv.BindAddr = "0.0.0.0:8787"
	drv.AuthMode = identity.AuthLoopback // "auth off": not bearer, so P3 refuses a non-loopback bind
	drv.ValidatorConfigured = false

	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"true"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err == nil {
		sess.Close()
		t.Fatal("Attach succeeded against an off-loopback bind with auth off, want a refusal")
	}
	if !errors.Is(err, terminal.ErrUnreachable) {
		t.Fatalf("Attach error = %v, want it to wrap terminal.ErrUnreachable", err)
	}
	if !strings.Contains(err.Error(), "0.0.0.0:8787") {
		t.Fatalf("Attach error = %v, want it to name the offending bind address (P3's own reason)", err)
	}
}

// A loopback bind (deskd's default) needs no bearer auth and must attach.
func TestLocalDriverAttachesOnDefaultLoopbackBind(t *testing.T) {
	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"true"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach on the default (loopback) bind: %v", err)
	}
	sess.Close()
}

// ── Verify row 5: shell exit flips the registry's handle to could-not-check,
// ── and nothing re-spawns without a new, explicit Attach.
func TestLocalDriverShellExitIsCouldNotCheck(t *testing.T) {
	cfg := &schema.File{Cells: []schema.Cell{{
		Name: "cell",
		Loops: []schema.Loop{{
			Role: schema.RoleReview,
			Target: &schema.TargetSpec{
				Kind:    schema.TargetLocal,
				Command: []string{"true"}, // exits immediately
			},
		}},
	}}}
	reg := terminal.NewRegistry(cfg)
	id := terminal.DeskID("cell", string(schema.RoleReview))

	drv := New()
	if err := reg.Attach(context.Background(), id, drv, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("registry.Attach: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		h, ok := reg.Resolve(id)
		if !ok {
			t.Fatal("Resolve reports the configured desk missing")
		}
		state = h.State
		if state == string(terminal.StateCouldNotCheck) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state != string(terminal.StateCouldNotCheck) {
		t.Fatalf("state after shell exit = %q, want %q within 2s", state, terminal.StateCouldNotCheck)
	}

	// No respawn without a new attach: the state stays could-not-check.
	time.Sleep(200 * time.Millisecond)
	h, ok := reg.Resolve(id)
	if !ok || h.State != string(terminal.StateCouldNotCheck) {
		t.Fatalf("handle changed after the exit without a new Attach: %+v (ok=%v)", h, ok)
	}
}

// ── Correctness review (PR #135): a session whose child exits on its own ──
// ── must not leak the PTY master fd — nothing else calls Close() on that ──
// ── path (the registry's own pump just drops the reference once Read()   ──
// ── closes; internal/terminal/registry.go's pump).                        ──
//
// This reads s.ptmx only AFTER draining ch to closed, never concurrently
// with pump's own goroutine: pump's close(ch) happens-after its closePTY()
// call, and the receive that observes ch closed happens-after that send, so
// by the time drain returns here, closePTY has already run and pump's
// goroutine has exited — no data race on the shared *os.File. (An earlier
// version of this test called ptmx.Fd() concurrently with pump's in-flight
// Read and was flagged by -race; probing the file directly, after the
// happens-before edge, is both race-free and a more direct proof that the
// close actually happened than a raw fcntl on a captured fd number would be.)
func TestLocalDriverClosesMasterFd_OnNaturalExit(t *testing.T) {
	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"true"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	ls, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Attach returned %T, want *local.Session", sess)
	}

	// Drain until the channel closes: the child ("true") exits immediately,
	// so pump reaps it and closes ch without anyone calling Close().
	drain(t, sess.Read(), 2*time.Second)

	if _, err := ls.ptmx.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Read on the master after natural exit = %v, want it wrapping os.ErrClosed (the master fd must already be closed, not leaked)", err)
	}
}

// ── Task 1/3: PATH resolved once, then cached — a later PATH change cannot ──
// ── redirect an already-resolved command.                                 ──
func TestLocalDriverResolvesPathOnce(t *testing.T) {
	dir1 := t.TempDir()
	writeExecutable(t, dir1, "probe-cmd", "#!/bin/sh\necho first\n")
	t.Setenv("PATH", dir1+string(os.PathListSeparator)+os.Getenv("PATH"))

	drv := New()
	p1, err := drv.resolveArgv0("probe-cmd")
	if err != nil {
		t.Fatalf("resolveArgv0 (first): %v", err)
	}
	want1 := filepath.Join(dir1, "probe-cmd")
	if p1 != want1 {
		t.Fatalf("resolveArgv0 = %q, want %q", p1, want1)
	}

	dir2 := t.TempDir()
	writeExecutable(t, dir2, "probe-cmd", "#!/bin/sh\necho second\n")
	t.Setenv("PATH", dir2)

	p2, err := drv.resolveArgv0("probe-cmd")
	if err != nil {
		t.Fatalf("resolveArgv0 (second, after PATH changed): %v", err)
	}
	if p2 != p1 {
		t.Fatalf("PATH change after first resolution redirected the argv: got %q, want the cached %q", p2, p1)
	}
}

func TestLocalDriverRefusesUnresolvableArgv0(t *testing.T) {
	drv := New()
	t.Setenv("PATH", t.TempDir()) // a PATH with nothing on it
	if _, err := drv.resolveArgv0("definitely-not-a-real-binary-xyz"); err == nil {
		t.Fatal("resolveArgv0 succeeded for a name on no PATH entry, want an error")
	}
}

func TestLocalDriverAbsoluteArgv0BypassesPathLookup(t *testing.T) {
	drv := New()
	p, err := drv.resolveArgv0("/bin/sh")
	if err != nil {
		t.Fatalf("resolveArgv0(/bin/sh): %v", err)
	}
	if p != "/bin/sh" {
		t.Fatalf("resolveArgv0(/bin/sh) = %q, want it unchanged", p)
	}
}

func TestLocalDriverDefaultCommandIsLoginShell(t *testing.T) {
	if len(defaultArgv) != 2 || defaultArgv[0] != "bash" || defaultArgv[1] != "-l" {
		t.Fatalf("defaultArgv = %v, want [bash -l]", defaultArgv)
	}
}

func TestLocalDriverEnvDenyList(t *testing.T) {
	cases := map[string]bool{
		"DESKD_GITHUB_TOKEN":    true,
		"DESKD_ANYTHING":        true,
		"MY_API_TOKEN":          true,
		"AWS_SECRET_ACCESS_KEY": true,
		"SSH_KEY_PATH":          true,
		"deskd_lowercase_token": true,
		"PATH":                  false,
		"HOME":                  false,
		"TERM":                  false,
		"GITHUB_REPOSITORY":     false,
	}
	for name, wantDenied := range cases {
		if got := isDeniedEnvName(name); got != wantDenied {
			t.Errorf("isDeniedEnvName(%q) = %v, want %v", name, got, wantDenied)
		}
	}
}

// the authorization contract: no human is present to act on a Claude Code prompt
// suggestion inside a desk's spawned shell, so the driver must force it off —
// even over an environment that already inherited CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true.
// A real PTY round trip, like TestLocalDriverStripsSecretsFromChildEnv above:
// the child's own `env` output is asserted, not the driver's internal state,
// so a fix that only "looks right" in cmd.Env but loses to environ ordering
// (see overrideEnv's own comment) is still caught.
func TestLocalDriverForcesPromptSuggestionOffOverInheritedTrue(t *testing.T) {
	t.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "true")

	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"env"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sess.Close()

	out := string(drain(t, sess.Read(), 2*time.Second))
	if strings.Contains(out, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true") {
		t.Fatalf("child env still carries the inherited true — the driver did not override it:\n%s", out)
	}
	if !strings.Contains(out, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false") {
		t.Fatalf("child env does not carry the forced false value:\n%s", out)
	}
}

// The unset case: a driver spawned by a process with no opinion on the
// variable still hands its child the forced-off default, not merely an
// absent variable a later `claude` invocation might default to true itself.
func TestLocalDriverForcesPromptSuggestionOffWhenUnset(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"env"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sess.Close()

	out := string(drain(t, sess.Read(), 2*time.Second))
	if !strings.Contains(out, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false") {
		t.Fatalf("child env does not carry the forced false default when nothing was inherited:\n%s", out)
	}
}

// overrideEnv is exercised directly too: the ordering property (the override
// must WIN, not merely be present) is easiest to state as a pure unit test,
// and it is what makes the PTY tests above a regression check on the real
// mechanism rather than the only proof it works.
func TestOverrideEnvRemovesInheritedValueBeforeAppending(t *testing.T) {
	in := []string{"PATH=/bin", "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true", "HOME=/nonexistent"}
	got := overrideEnv(in, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "false")

	count := 0
	sawFalseLast := false
	for i, kv := range got {
		if strings.HasPrefix(kv, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=") {
			count++
			if kv != "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false" {
				t.Fatalf("overrideEnv left a non-false entry: %q", kv)
			}
			sawFalseLast = i == len(got)-1
		}
	}
	if count != 1 {
		t.Fatalf("overrideEnv produced %d entries for the name, want exactly 1: %v", count, got)
	}
	if !sawFalseLast {
		t.Fatalf("overrideEnv did not place the override last: %v", got)
	}
}

// writeExecutable writes an executable script named name into dir.
func writeExecutable(t *testing.T, dir, name, contents string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o755); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// regression: F-security-surface — a cell's chat key_env may be any
// upper-case name (MODEL_SUBSCRIPTION), so the TOKEN/SECRET/KEY needles let
// the model subscription key through to every desk shell.
func TestLocalDriverStripsConfiguredKeyEnv(t *testing.T) {
	t.Setenv("MODEL_SUBSCRIPTION", "subscription-canary-41c")
	DenyEnvNames("model_subscription ") // case and whitespace are normalized

	drv := New()
	spec := &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"env"}}
	sess, err := drv.Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sess.Close()
	out := string(drain(t, sess.Read(), 2*time.Second))
	if strings.Contains(out, "subscription-canary-41c") || strings.Contains(out, "MODEL_SUBSCRIPTION") {
		t.Fatalf("child env carried the configured credential variable:\n%s", out)
	}
	if !strings.Contains(out, "PATH=") {
		t.Fatalf("child env lost an ordinary variable (the deny set is too wide):\n%s", out)
	}
	if isDeniedEnvName("MODEL_SUBSCRIPTION_REGION") {
		t.Errorf("an exact configured name must not deny a longer variable that merely starts with it")
	}
}
