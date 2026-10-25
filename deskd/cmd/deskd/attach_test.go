package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// TestDeskdLinksContainerAndLocalOnly is Verify row 2: the deskd binary links
// the container and local drivers and NOT the pod driver (drivers.go's
// imports; DR-deskd-attach-start). Because the pod driver is not in the
// binary, terminal.DriverFor(pod) misses for EVERY consumer inside deskd — the
// attach supervisor, a session open, a send, a future timer — not only for
// the supervisor, whose own kind check is a separate layer. Re-adding the pod
// import to drivers.go turns this red. It checks registration only and never
// calls a factory.
func TestDeskdLinksContainerAndLocalOnly(t *testing.T) {
	for _, kind := range []schema.TargetKind{schema.TargetContainer, schema.TargetLocal} {
		if _, ok := terminal.DriverFor(kind); !ok {
			t.Errorf("terminal.DriverFor(%q) = not registered — cmd/deskd/drivers.go must link the container and local drivers", kind)
		}
	}
	if _, ok := terminal.DriverFor(schema.TargetPod); ok {
		t.Errorf("terminal.DriverFor(%q) = registered — deskd must not link the pod driver (DR-deskd-attach-start: no deskd path may build it)", schema.TargetPod)
	}
}

// TestOnceModeNeverAttaches is Verify row 15: `--once` builds the registry
// and prints the banner, and starts no supervisor, so a cron, CI or verify run
// never execs. It runs the real binary offline (DESKD_API_UNREACHABLE,
// KUBECONFIG=/dev/null). Two independent signals would show a started
// supervisor: its transition log line (Start logs the no-target desk
// synchronously) and the local desk's spawn, which would create a marker file.
func TestOnceModeNeverAttaches(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "spawned")
	cfg := writeAttachFixture(t, dir, marker)
	bin := buildDeskd(t)
	cmd := exec.Command(bin, "--config", cfg, "--index", filepath.Join(dir, "idx.db"), "--once")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "DESKD_GITHUB_TOKEN=x", "DESKD_API_UNREACHABLE=1", "KUBECONFIG=/dev/null")
	out, _ := runWithTimeout(t, cmd, 60*time.Second)

	if !strings.Contains(out, "terminal registry") || !strings.Contains(out, "attach: never in --once") {
		t.Fatalf("the --once banner does not name the registry and the never-attach mode:\n%s", out)
	}
	if strings.Contains(out, "terminal attach: desk=") {
		t.Fatalf("--once started the attach supervisor (a transition was logged):\n%s", out)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("--once spawned the local desk (marker %s exists)", marker)
	}
}

// TestConfiguredModeAttachesInTheBinary is the paired positive control for
// TestOnceModeNeverAttaches, and the offline stand-in for Verify row 9's
// "attach works in the binary, not just in tests": the same fixture without
// --once, on a loopback port. deskd spawns the local desk by itself (the
// marker appears), logs each transition, reports the no-target desk honestly,
// and exits cleanly on SIGTERM. The only process it spawns is /usr/bin/touch.
func TestConfiguredModeAttachesInTheBinary(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "spawned")
	cfg := writeAttachFixture(t, dir, marker)
	bin := buildDeskd(t)
	var buf syncBuffer
	cmd := exec.Command(bin, "--config", cfg, "--index", filepath.Join(dir, "idx.db"), "--addr", "127.0.0.1:0")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "DESKD_GITHUB_TOKEN=x", "DESKD_API_UNREACHABLE=1", "KUBECONFIG=/dev/null")
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Kill()
	}()

	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil && strings.Contains(buf.String(), "desk=v/dispatch kind=local attached -> failed reason=exited") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("configured mode never spawned the local desk and logged its exit within 20s:\n%s", buf.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("deskd did not exit within 10s of SIGTERM (attach Stop must fit the shutdown window):\n%s", buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"attach: configured",
		"desk=v/dispatch kind=local off -> attached reason= attempt=1",
		"desk=v/review kind= off -> failed reason=no-target",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("deskd output lacks %q:\n%s", want, out)
		}
	}
	// A local desk is spawned once and never re-spawned.
	if n := strings.Count(out, "desk=v/dispatch kind=local off -> attached"); n != 1 {
		t.Errorf("local desk attached %d times, want exactly 1:\n%s", n, out)
	}
}

// TestAttachTextDisclosesTheInputPath pins the operator-facing disclosure of
// what `configured` arms (security review S-sec-attach-arms-send): with
// the service contract's send route in the tree, an attached desk accepts send
// from its cell's answerer with no session open, so neither the
// --terminal-attach help nor the startup banner may call the lifecycle
// read-only, and both must say that off disables the input path too.
func TestAttachTextDisclosesTheInputPath(t *testing.T) {
	for name, text := range map[string]string{
		"--terminal-attach help": terminalAttachUsage,
		"configured-mode banner": attachBanner(terminal.AttachModeConfigured, false),
	} {
		if strings.Contains(strings.ToLower(text), "read-only") {
			t.Errorf("%s calls the attach lifecycle read-only, but an attached desk accepts send:\n%s", name, text)
		}
		for _, want := range []string{"writes nothing", "accepts send from its cell's answerer", "deskd's own OS user", "disables both"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q:\n%s", name, want, text)
			}
		}
	}
	if off := attachBanner(terminal.AttachModeOff, false); !strings.Contains(off, "no desk accepts send") || !strings.Contains(off, "every session open is refused") {
		t.Errorf("off-mode banner does not say off disables the input path:\n%s", off)
	}
}

// writeAttachFixture writes a one-cell cells.yaml: a local desk whose command
// creates marker, and a desk with no target.
func writeAttachFixture(t *testing.T, dir, marker string) string {
	t.Helper()
	cfg := filepath.Join(dir, "cells.yaml")
	yaml := "cells:\n" +
		"- name: v\n" +
		"  repos: [example-org/console]\n" +
		"  streams_root: docs/streams\n" +
		"  silence_threshold: 48h\n" +
		"  notification_tiers: {review: push, question: badge, notify: none}\n" +
		"  loops:\n" +
		"  - {role: dispatch, schedule: \"*/15 * * * *\", models: [m], app: assay-desk-app, target: {kind: local, command: [\"/usr/bin/touch\", \"" + marker + "\"]}}\n" +
		"  - {role: review, schedule: \"*/10 * * * *\", models: [m], app: assay-reviewer-app}\n"
	if err := os.WriteFile(cfg, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// syncBuffer is a goroutine-safe output sink (the process writes while the
// test polls).
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
